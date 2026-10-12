//go:build darwin

package core

import (
	"encoding/binary"
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

func TestMovePreservesMacCreationDate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "old.png")
	writeTestFile(t, source, []byte("image"))
	var birth [16]byte
	binary.NativeEndian.PutUint64(birth[:8], 946684800)
	attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: unix.ATTR_CMN_CRTIME}
	if err := unix.Setattrlist(source, &attrs, birth[:], unix.FSOPT_NOFOLLOW); err != nil {
		t.Fatal(err)
	}
	var before unix.Stat_t
	if err := unix.Stat(source, &before); err != nil {
		t.Fatal(err)
	}
	destination, err := MoveFile(source, filepath.Join(dir, "out"), "dated")
	if err != nil {
		t.Fatal(err)
	}
	var after unix.Stat_t
	if err = unix.Stat(destination, &after); err != nil {
		t.Fatal(err)
	}
	if before.Btim != after.Btim {
		t.Fatalf("birthtime changed: before %v after %v", before.Btim, after.Btim)
	}
}
