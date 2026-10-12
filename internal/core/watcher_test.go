package core

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeScreenshot() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	var random uint32 = 0x19831
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			random = random*1664525 + 1013904223
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(random), G: uint8(random >> 8), B: uint8(random >> 16), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}
func receivePath(t *testing.T, ch <-chan string, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher missed image")
	}
}
func noPath(t *testing.T, ch <-chan string) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("unexpected image %s", got)
	case <-time.After(100 * time.Millisecond):
	}
}
func TestWatcherBaselineDelayedWriteReuseAndSuppress(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "Screenshot existing.png")
	writeTestFile(t, existing, fakeScreenshot())
	events := make(chan string, 10)
	w := &Watcher{Directory: dir, Interval: 10 * time.Millisecond}
	if err := w.Start(context.Background(), func(path string) { events <- path }, func(err error) { t.Error(err) }); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	noPath(t, events)
	path := filepath.Join(dir, "Screenshot delayed.png")
	writeTestFile(t, path, pngSignature)
	noPath(t, events)
	writeTestFile(t, path, fakeScreenshot())
	receivePath(t, events, path)
	noPath(t, events)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, fakeScreenshot())
	receivePath(t, events, path)
	restored := filepath.Join(dir, "Screenshot restored.png")
	writeTestFile(t, restored, fakeScreenshot())
	w.Suppress(restored)
	noPath(t, events)
	writeTestFile(t, filepath.Join(dir, "unrelated.png"), fakeScreenshot())
	writeTestFile(t, filepath.Join(dir, "Screenshot fake.png"), make([]byte, 21*1024))
	noPath(t, events)
}
func TestWatcherUnavailableAndCancellation(t *testing.T) {
	w := &Watcher{Directory: filepath.Join(t.TempDir(), "missing")}
	if err := w.Start(context.Background(), nil, nil); err == nil {
		t.Fatal("missing directory accepted")
	}
	w.Stop()
	w.Directory = t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
	cancel()
	w.Stop()
	if err := w.Start(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	w.Stop()
}

func TestWatcherSuppressesBeforePublication(t *testing.T) {
	dir := t.TempDir()
	events := make(chan string, 5)
	w := &Watcher{Directory: dir, Interval: 10 * time.Millisecond}
	if err := w.Start(context.Background(), func(p string) { events <- p }, nil); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	path := filepath.Join(dir, "Screenshot restored.png")
	w.Suppress(path)
	noPath(t, events)
	writeTestFile(t, path, pngSignature)
	noPath(t, events)
	writeTestFile(t, path, fakeScreenshot())
	noPath(t, events)
}

func TestWatcherSuppressesRestorationThroughDirectoryAlias(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-watch")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "watch-alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	w, scan := scanningWatcher(t, alias)
	path := filepath.Join(real, "Screenshot restored.png")
	w.Suppress(path)
	writeTestFile(t, path, fakeScreenshot())
	scan(3, "")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, fakeScreenshot())
	scan(3, filepath.Join(alias, filepath.Base(path)))
}

// Drive complete scans directly, so settlement/replacement assertions do not
// depend on the CI host scheduling a ticker within an arbitrary sleep.
func scanningWatcher(t *testing.T, dir string) (*Watcher, func(int, string)) {
	t.Helper()
	w := &Watcher{Directory: dir, seen: map[string]os.FileInfo{}, pending: map[string]watchedFile{}, suppressed: map[string]suppression{}}
	started := time.Now()
	return w, func(count int, want string) {
		t.Helper()
		var found []string
		for range count {
			paths, err := w.scan(started)
			if err != nil {
				t.Fatal(err)
			}
			found = append(found, paths...)
		}
		if want == "" && len(found) != 0 || want != "" && (len(found) != 1 || found[0] != want) {
			t.Fatalf("scan reported %v, want %q", found, want)
		}
	}
}

func TestWatcherSettledSuppressionAllowsSameInodeChanges(t *testing.T) {
	for _, tc := range []struct {
		name            string
		small, trailing bool
	}{{name: "screenshot"}, {name: "small old image", small: true}, {name: "trailing data", trailing: true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w, scan := scanningWatcher(t, dir)
			path := filepath.Join(dir, "Screenshot reused.png")
			w.Suppress(path)
			// A paused partial copy must remain suppressed across many scans.
			writeTestFile(t, path, pngSignature)
			scan(10, "")
			complete := fakeScreenshot()
			if tc.small {
				var b bytes.Buffer
				if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
					t.Fatal(err)
				}
				complete = b.Bytes()
			}
			// Even a large, otherwise valid image remains an incomplete copy
			// until its IEND chunk has been published.
			writeTestFile(t, path, complete[:len(complete)-12])
			scan(10, "")
			if tc.trailing {
				complete = append(complete, []byte("trailing bytes accepted by the PNG decoder")...)
			}
			writeTestFile(t, path, complete)
			oldTime := time.Now().Add(-time.Hour)
			if err := os.Chtimes(path, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
			scan(3, "")
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			// An in-place rewrite deterministically recreates Linux inode reuse.
			// The larger-image case keeps its size and changes only bytes/mtime.
			changed := fakeScreenshot()
			changed[len(changed)/2] ^= 1
			writeTestFile(t, path, changed)
			now := time.Now()
			if err := os.Chtimes(path, now, now); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("test must retain source inode: %v", err)
			}
			scan(3, path)
			scan(3, "")
		})
	}
}
