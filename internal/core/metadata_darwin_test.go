//go:build darwin

package core

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMovePreservesMacExtendedAttributes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestFile(t, source, []byte("image"))
	if err := unix.Setxattr(source, "com.chaos.test-tag", []byte("original-tag"), 0); err != nil {
		t.Fatal(err)
	}
	destination, err := MoveFile(source, filepath.Join(dir, "out"), "tagged")
	if err != nil {
		t.Fatal(err)
	}
	value := make([]byte, 100)
	n, err := unix.Getxattr(destination, "com.chaos.test-tag", value)
	if err != nil {
		t.Fatal(err)
	}
	if string(value[:n]) != "original-tag" {
		t.Fatal("metadata lost")
	}
	info, _ := os.Stat(destination)
	if info.Mode().Perm() != 0600 {
		t.Fatal("mode changed")
	}
}
