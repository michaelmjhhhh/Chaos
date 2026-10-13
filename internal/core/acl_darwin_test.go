//go:build darwin

package core

import (
	"errors"
	"os"
	"path/filepath"
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
			var errno int32
			freed := false
			api := darwinACLAPI{
				get: func(fd int32, kind int32) uintptr {
					if fd < 0 || kind != aclTypeExtended {
						t.Fatal("invalid ACL arguments")
					}
					return 123
				},
				set: func(_ string, kind int32, acl uintptr) int32 {
					if acl != 123 || kind != aclTypeExtended {
						t.Fatal("wrong ACL passed to destination")
					}
					errno = tc.setErr
					return -1
				},
				getEntry: func(_ uintptr, _ int32, entry *uintptr) int32 {
					if tc.hasEntry {
						*entry = 456
						return 0
					}
					errno = int32(unix.EINVAL)
					return -1
				},
				free:  func(acl uintptr) int32 { freed = acl == 123; return 0 },
				errno: func() *int32 { return &errno },
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
