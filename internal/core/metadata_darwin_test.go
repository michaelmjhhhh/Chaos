//go:build darwin

package core

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestMoveRenameAndRevertPreserveMacACL(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestFile(t, source, []byte("image"))
	macACLCommand(t, "+a", "everyone deny write", source)
	want := macACLEntries(t, source)
	if !strings.Contains(want, "deny write") {
		t.Fatalf("test ACL was not installed: %q", want)
	}
	destination, err := MoveFile(source, filepath.Join(dir, "out"), "filed")
	if err != nil {
		t.Fatal(err)
	}
	if got := macACLEntries(t, destination); got != want {
		t.Fatalf("filed ACL changed: %q != %q", got, want)
	}
	destination, err = RenameFile(destination, "renamed")
	if err != nil {
		t.Fatal(err)
	}
	if got := macACLEntries(t, destination); got != want {
		t.Fatalf("renamed ACL changed: %q != %q", got, want)
	}
	destination, err = RevertFile(destination, source)
	if err != nil {
		t.Fatal(err)
	}
	if got := macACLEntries(t, destination); got != want {
		t.Fatalf("restored ACL changed: %q != %q", got, want)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY, 0)
	if err == nil {
		file.Close()
		t.Fatal("restored ACL no longer denies writes")
	}
}

func TestPublicationWithoutLinksPreservesMacACL(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "copy.png")
	writeTestFile(t, source, []byte("image"))
	macACLCommand(t, "+a", "everyone deny write", source)
	if err := publishWithoutLinks(source, destination); err != nil {
		t.Fatal(err)
	}
	if got, want := macACLEntries(t, destination), macACLEntries(t, source); got != want {
		t.Fatalf("copy fallback ACL changed: %q != %q", got, want)
	}
}

func TestMovePreservesEmptyACLInInheritingDirectory(t *testing.T) {
	dir := t.TempDir()
	source, out := filepath.Join(dir, "source.png"), filepath.Join(dir, "out")
	writeTestFile(t, source, []byte("image"))
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	macACLCommand(t, "+a", "everyone allow read,file_inherit", out)
	destination, err := MoveFile(source, out, "filed")
	if err != nil {
		t.Fatal(err)
	}
	if got := macACLEntries(t, destination); got != "" {
		t.Fatalf("move acquired permissions from destination folder: %s", got)
	}
}

func TestMacACLFailurePreservesOriginalAndCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestFile(t, source, []byte("original"))
	macACLCommand(t, "+a", "everyone deny readsecurity", source)
	t.Cleanup(func() { _ = exec.Command("/bin/chmod", "-N", source).Run() })
	_, err := MoveFile(source, filepath.Join(dir, "out"), "filed")
	if err == nil {
		t.Fatal("move silently ignored unreadable ACL")
	}
	data, readErr := os.ReadFile(source)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("original was modified: %q, %v", data, readErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(dir, "out"))
	if (readErr != nil && !errors.Is(readErr, os.ErrNotExist)) || len(entries) != 0 {
		t.Fatalf("failed move left destination artifacts: %v, %v", entries, readErr)
	}
}

func TestMacACLAndFlagsSurviveRejectedJournalWithoutArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestFile(t, source, []byte("image"))
	macACLCommand(t, "+a", "everyone deny write", source)
	if err := unix.Chflags(source, unix.UF_HIDDEN|unix.UF_NODUMP); err != nil {
		t.Fatal(err)
	}
	wantACL := macACLEntries(t, source)
	rejected := errors.New("journal write failed")
	_, err := MoveFileWithHook(source, filepath.Join(dir, "out"), "filed", func(string) error { return rejected })
	if !errors.Is(err, rejected) {
		t.Fatal(err)
	}
	if got := macACLEntries(t, source); got != wantACL {
		t.Fatal("rejected move modified source ACL")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "out"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected move left temporary files: %v, %v", entries, err)
	}
	destination, err := MoveFile(source, filepath.Join(dir, "out"), "filed")
	if err != nil {
		t.Fatal(err)
	}
	var stat unix.Stat_t
	if err := unix.Stat(destination, &stat); err != nil {
		t.Fatal(err)
	}
	if stat.Flags&(unix.UF_HIDDEN|unix.UF_NODUMP) != unix.UF_HIDDEN|unix.UF_NODUMP {
		t.Fatalf("ordinary user flags were lost: %#x", stat.Flags)
	}
}

func macACLCommand(t *testing.T, args ...string) {
	t.Helper()
	if output, err := exec.Command("/bin/chmod", args...).CombinedOutput(); err != nil {
		t.Fatalf("set test ACL: %v: %s", err, output)
	}
}

func macACLEntries(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command("/bin/ls", "-lde", path).CombinedOutput()
	if err != nil {
		t.Fatalf("read test ACL: %v: %s", err, output)
	}
	_, entries, _ := strings.Cut(string(output), "\n")
	return strings.TrimSpace(entries)
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
