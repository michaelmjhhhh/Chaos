//go:build darwin

package core

import (
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

// Preserve Finder tags, image metadata and resource forks across the temporary
// copy. Filesystems without extended attributes retain the image bytes/mtime.
func copyMetadata(source, destination string) error {
	size, err := unix.Listxattr(source, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return nil
	}
	if err != nil {
		return err
	}
	if size == 0 {
		return nil
	}
	if size > 1<<20 {
		return errors.New("image extended attributes exceed metadata limit")
	}
	names := make([]byte, size)
	size, err = unix.Listxattr(source, names)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(string(names[:size]), "\x00") {
		if name == "" {
			continue
		}
		size, err := unix.Getxattr(source, name, nil)
		if errors.Is(err, unix.ENOATTR) {
			continue
		}
		if err != nil {
			return err
		}
		if size > 16<<20 {
			return errors.New("image extended attribute exceeds metadata limit")
		}
		value := make([]byte, size)
		size, err = unix.Getxattr(source, name, value)
		if errors.Is(err, unix.ENOATTR) {
			continue
		}
		if err != nil {
			return err
		}
		if err = unix.Setxattr(destination, name, value[:size], 0); errors.Is(err, unix.ENOTSUP) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}
