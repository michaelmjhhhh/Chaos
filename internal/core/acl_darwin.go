//go:build darwin

package core

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// Public signatures/constants from the macOS SDK's sys/acl.h. Keeping ACLs as
// opaque handles avoids depending on Apple's private struct layouts. purego
// already supplies the native UI bridge and keeps CGO_ENABLED=0 builds working.
const aclTypeExtended int32 = 0x00000100

type darwinACLAPI struct {
	get      func(int32, int32) (uintptr, syscall.Errno)
	init     func(int32) (uintptr, syscall.Errno)
	set      func(string, int32, uintptr) (int32, syscall.Errno)
	free     func(uintptr) int32
	getEntry func(uintptr, int32, *uintptr) (int32, syscall.Errno)
}

var loadDarwinACL = sync.OnceValues(func() (*darwinACLAPI, error) {
	var get, init, set, free, getEntry uintptr
	for _, binding := range []struct {
		name    string
		address *uintptr
	}{
		{"acl_get_fd_np", &get},
		{"acl_init", &init},
		{"acl_set_file", &set},
		{"acl_free", &free},
		{"acl_get_entry", &getEntry},
	} {
		symbol, err := purego.Dlsym(purego.RTLD_DEFAULT, binding.name)
		if err != nil {
			return nil, fmt.Errorf("load macOS ACL API %s: %w", binding.name, err)
		}
		*binding.address = symbol
	}
	// Capture errno in the native trampoline. A later __error call can observe
	// runtime activity's errno even when the goroutine is pinned to an OS thread.
	// Darwin errno is a C int; purego v0.11.1 reads a pointer-width word, so
	// discard the neighboring bytes from its captured result.
	api := &darwinACLAPI{
		get: func(fd, kind int32) (uintptr, syscall.Errno) {
			result, _, errno := purego.SyscallN(get, uintptr(fd), uintptr(kind))
			return result, syscall.Errno(uint32(errno))
		},
		init: func(count int32) (uintptr, syscall.Errno) {
			result, _, errno := purego.SyscallN(init, uintptr(count))
			return result, syscall.Errno(uint32(errno))
		},
		set: func(path string, kind int32, acl uintptr) (int32, syscall.Errno) {
			name, err := syscall.BytePtrFromString(path)
			if err != nil {
				return -1, syscall.EINVAL
			}
			result, _, errno := purego.SyscallN(set, uintptr(unsafe.Pointer(name)), uintptr(kind), acl)
			runtime.KeepAlive(name)
			return int32(result), syscall.Errno(uint32(errno))
		},
		getEntry: func(acl uintptr, entryID int32, entry *uintptr) (int32, syscall.Errno) {
			result, _, errno := purego.SyscallN(getEntry, acl, uintptr(entryID), uintptr(unsafe.Pointer(entry)))
			runtime.KeepAlive(entry)
			return int32(result), syscall.Errno(uint32(errno))
		},
	}
	purego.RegisterFunc(&api.free, free)
	return api, nil
})

func copyACL(source, destination string) error {
	api, err := loadDarwinACL()
	if err != nil {
		return err
	}
	return api.copy(source, destination)
}

func (api *darwinACLAPI) copy(source, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	acl, nativeErr := api.get(int32(file.Fd()), aclTypeExtended)
	if acl == 0 {
		// On macOS an existing file without an ACL returns ENOENT. Using a
		// valid descriptor distinguishes that result from a missing pathname.
		if nativeErr != unix.ENOENT && nativeErr != unix.ENOTSUP && nativeErr != unix.EOPNOTSUPP {
			return &os.PathError{Op: "read image ACL", Path: source, Err: nativeErr}
		}
		// An explicit empty ACL also clears ACL entries that the newly-created
		// destination may have inherited from its enclosing directory.
		acl, nativeErr = api.init(0)
		if acl == 0 {
			return fmt.Errorf("create empty image ACL: %w", nativeErr)
		}
	}
	defer api.free(acl)
	result, nativeErr := api.set(destination, aclTypeExtended, acl)
	if result == 0 {
		return nil
	}
	if nativeErr == unix.ENOTSUP || nativeErr == unix.EOPNOTSUPP {
		// Moving an ordinary file onto a filesystem without ACLs is safe. A
		// nonempty ACL must never be discarded just to make a move succeed.
		var entry uintptr
		if result, entryErr := api.getEntry(acl, 0 /* ACL_FIRST_ENTRY */, &entry); result == -1 && entryErr == unix.EINVAL {
			return nil
		}
	}
	return &os.PathError{Op: "preserve image ACL", Path: destination, Err: nativeErr}
}
