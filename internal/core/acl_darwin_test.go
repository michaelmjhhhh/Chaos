//go:build darwin

package core

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestACLRefusesSecurityDowngradeAndReleasesNativeAllocation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setErr    int32
		hasEntry  bool
		wantError bool
	}{
		{"permission denied", int32(unix.EACCES), true, true},
		{"nonempty unsupported", int32(unix.ENOTSUP), true, true},
		{"empty unsupported", int32(unix.ENOTSUP), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.png")
			writeTestFile(t, source, []byte("image"))
			freed := false
			api := darwinACLAPI{
				get: func(fd int32, kind int32) (uintptr, syscall.Errno) {
					if fd < 0 || kind != aclTypeExtended {
						t.Fatal("invalid ACL arguments")
					}
					return 123, 0
				},
				set: func(_ string, kind int32, acl uintptr) (int32, syscall.Errno) {
					if acl != 123 || kind != aclTypeExtended {
						t.Fatal("wrong ACL passed to destination")
					}
					return -1, syscall.Errno(tc.setErr)
				},
				getEntry: func(_ uintptr, _ int32, entry *uintptr) (int32, syscall.Errno) {
					if tc.hasEntry {
						*entry = 456
						return 0, 0
					}
					return -1, unix.EINVAL
				},
				free: func(acl uintptr) int32 { freed = acl == 123; return 0 },
			}
			err := api.copy(source, filepath.Join(dir, "destination.png"))
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected copy result: %v", err)
			}
			if !freed {
				t.Fatal("native ACL allocation leaked")
			}
			if tc.wantError && !errors.Is(err, unix.Errno(tc.setErr)) {
				t.Fatalf("destination error was replaced: %v", err)
			}
		})
	}
}

func TestACLNativeErrnoSurvivesConcurrentCalls(t *testing.T) {
	api, err := loadDarwinACL()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 500 {
				acl, err := api.get(-1, aclTypeExtended)
				if acl != 0 {
					api.free(acl)
					t.Error("invalid descriptor returned an ACL")
					return
				}
				if err != unix.EBADF {
					t.Errorf("native errno changed before observation: %v", err)
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestCopyACLReportsNativeDestinationPermissionError(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "destination.png")
	writeTestFile(t, source, []byte("image"))
	writeTestFile(t, destination, []byte("other"))
	macACLCommand(t, "+a", "everyone deny write", source)
	macACLCommand(t, "+a", "everyone allow read", destination)
	if err := unix.Chflags(destination, unix.UF_IMMUTABLE); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Chflags(destination, 0); err != nil {
			t.Error(err)
		}
	})
	err := copyACL(source, destination)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "preserve image ACL" || pathErr.Path != destination {
		t.Fatalf("destination ACL failure was hidden: %v", err)
	}
	if got := macACLEntries(t, source); got != "0: group:everyone deny write" {
		t.Fatalf("source ACL changed: %q", got)
	}
	if got := macACLEntries(t, destination); got != "0: group:everyone allow read" {
		t.Fatalf("destination ACL was replaced despite error: %q", got)
	}
}
