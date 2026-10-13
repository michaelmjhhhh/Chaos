//go:build darwin

package core

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// Public signatures/constants from the macOS SDK's sys/acl.h. Keeping ACLs as
// opaque handles avoids depending on Apple's private struct layouts. purego
// already supplies the native UI bridge and keeps CGO_ENABLED=0 builds working.
const aclTypeExtended int32 = 0x00000100

type darwinACLAPI struct {
	get      func(int32, int32) uintptr
	init     func(int32) uintptr
	set      func(string, int32, uintptr) int32
	free     func(uintptr) int32
	getEntry func(uintptr, int32, *uintptr) int32
	errno    func() *int32
}

var loadDarwinACL = sync.OnceValues(func() (*darwinACLAPI, error) {
	api := new(darwinACLAPI)
	for _, binding := range []struct {
		name string
		fn   any
	}{
		{"acl_get_fd_np", &api.get},
		{"acl_init", &api.init},
		{"acl_set_file", &api.set},
		{"acl_free", &api.free},
		{"acl_get_entry", &api.getEntry},
		{"__error", &api.errno},
	} {
		symbol, err := purego.Dlsym(purego.RTLD_DEFAULT, binding.name)
		if err != nil {
			return nil, fmt.Errorf("load macOS ACL API %s: %w", binding.name, err)
		}
		purego.RegisterFunc(binding.fn, symbol)
	}
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
	// errno is thread-local. Pin the caller until after each native result and
	// its errno have been inspected, including when Go reschedules a goroutine.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	acl := api.get(int32(file.Fd()), aclTypeExtended)
	if acl == 0 {
		err := syscall.Errno(*api.errno())
		// On macOS an existing file without an ACL returns ENOENT. Using a
		// valid descriptor distinguishes that result from a missing pathname.
		if err != unix.ENOENT && err != unix.ENOTSUP && err != unix.EOPNOTSUPP {
			return &os.PathError{Op: "read image ACL", Path: source, Err: err}
		}
		// An explicit empty ACL also clears ACL entries that the newly-created
		// destination may have inherited from its enclosing directory.
		acl = api.init(0)
		if acl == 0 {
			return fmt.Errorf("create empty image ACL: %w", syscall.Errno(*api.errno()))
		}
	}
	defer api.free(acl)
	if api.set(destination, aclTypeExtended, acl) == 0 {
		return nil
	}
	err = syscall.Errno(*api.errno())
	if err == unix.ENOTSUP || err == unix.EOPNOTSUPP {
		// Moving an ordinary file onto a filesystem without ACLs is safe. A
		// nonempty ACL must never be discarded just to make a move succeed.
		var entry uintptr
		if api.getEntry(acl, 0 /* ACL_FIRST_ENTRY */, &entry) == -1 && syscall.Errno(*api.errno()) == unix.EINVAL {
			return nil
		}
	}
	return &os.PathError{Op: "preserve image ACL", Path: destination, Err: err}
}
