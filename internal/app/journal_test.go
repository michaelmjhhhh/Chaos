package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func TestRecoveryPreservesImagesAtAmbiguousLocations(t *testing.T) {
	for _, kind := range []string{"destination symlink", "parent directory alias", "hard link", "destination directory", "source symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "filed.png")
			original := writePNG(t, source, false)
			op := operation{Kind: "insert", Record: core.RecentFile{ID: "AMBIGUOUS", SourcePath: source, Timestamp: time.Now()}, Source: source, Destination: destination}
			if err := op.fingerprint(); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "destination symlink":
				if err := os.Symlink(source, destination); err != nil {
					t.Fatal(err)
				}
			case "parent directory alias":
				alias := filepath.Join(dir, "alias")
				if err := os.Symlink(dir, alias); err != nil {
					t.Fatal(err)
				}
				op.Destination = filepath.Join(alias, filepath.Base(source))
			case "hard link":
				if err := os.Link(source, destination); err != nil {
					t.Fatal(err)
				}
			case "destination directory":
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
			case "source symlink":
				if err := os.Rename(source, destination); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(destination, source); err != nil {
					t.Fatal(err)
				}
			}
			pendingDir := filepath.Join(dir, "pending")
			if err := os.Mkdir(pendingDir, 0700); err != nil {
				t.Fatal(err)
			}
			journal, err := json.Marshal(op)
			if err != nil {
				t.Fatal(err)
			}
			journalPath := filepath.Join(pendingDir, op.Record.ID+".json")
			if err := os.WriteFile(journalPath, journal, 0600); err != nil {
				t.Fatal(err)
			}
			// Reopening must remain safe and retain the recovery intent; an
			// ambiguous first attempt cannot quietly become a success next time.
			for attempt := 0; attempt < 2; attempt++ {
				c, err := New(Options{DataDir: dir, Home: dir})
				if err != nil {
					t.Fatal(err)
				}
				snapshot := c.Snapshot()
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
				if snapshot.StorageError == "" || len(snapshot.RecentFiles) != 0 {
					t.Fatalf("ambiguous recovery recorded success: storage=%q rows=%v", snapshot.StorageError, snapshot.RecentFiles)
				}
				if data, err := os.ReadFile(source); err != nil || !bytes.Equal(data, original) {
					t.Fatalf("source image was removed or changed: %v", err)
				}
				if _, err := os.Lstat(op.Destination); err != nil {
					t.Fatalf("ambiguous destination was removed: %v", err)
				}
				if data, err := os.ReadFile(journalPath); err != nil || !bytes.Equal(data, journal) {
					t.Fatalf("pending journal was removed or changed: %v", err)
				}
			}
		})
	}
}

func TestRecoveryFingerprintRejectsReplacedDirectoryEntry(t *testing.T) {
	for _, kind := range []string{"replacement file", "replacement symlink", "same inode content replacement"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "image.png")
			original := writePNG(t, path, false)
			fingerprint, err := readRecoveryFile(path)
			if err != nil {
				t.Fatal(err)
			}
			previous := filepath.Join(dir, "previous.png")
			preservedPath, preservedBytes := previous, original
			if kind == "same inode content replacement" {
				// An editor can preserve both size and mtime while changing
				// bytes. The final verification must compare content as well.
				preservedPath = path
				preservedBytes = append([]byte(nil), original...)
				preservedBytes[0] ^= 0xff
				if err := os.WriteFile(path, preservedBytes, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, fingerprint.info.ModTime(), fingerprint.info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := fingerprint.verifyEntry(path); err != nil {
					t.Fatalf("fixture must retain original identity/size/mtime: %v", err)
				}
			} else {
				// Preserve the old inode, then reuse its pathname with identical
				// bytes, size and mtime. Identity must detect the replacement.
				if err := os.Rename(path, previous); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "replacement symlink" {
				if err := os.Symlink(previous, path); err != nil {
					t.Fatal(err)
				}
			} else if kind == "replacement file" {
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, fingerprint.info.ModTime(), fingerprint.info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			if err := fingerprint.verifyCurrent(path); err == nil {
				t.Fatal("replaced entry passed recovery verification")
			}
			if data, err := os.ReadFile(preservedPath); err != nil || !bytes.Equal(data, preservedBytes) {
				t.Fatalf("verification modified image: %v", err)
			}
		})
	}
}

func TestRecoveryRejectsUnknownKindBeforeRemovingSource(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "destination.png")
	want := writePNG(t, source, false)
	if err := os.WriteFile(destination, want, 0600); err != nil {
		t.Fatal(err)
	}
	op := operation{Kind: "unsupported", Record: core.RecentFile{ID: "UNKNOWN"}, Source: source, Destination: destination}
	if err := op.fingerprint(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pending", "UNKNOWN.json")
	if err := core.AtomicWrite(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := New(Options{DataDir: dir, Home: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Snapshot().StorageError == "" {
		t.Fatal("unknown operation was accepted")
	}
	for _, path := range []string{source, destination} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("unknown operation changed %s: %v", path, err)
		}
	}
	if len(c.Snapshot().RecentFiles) != 0 {
		t.Fatal("unknown operation changed history")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("unknown operation discarded journal", err)
	}
}
