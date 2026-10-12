package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeScreenshot() []byte { b := make([]byte, 21*1024); copy(b, pngSignature); return b }
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
	events := make(chan string, 5)
	w := &Watcher{Directory: alias, Interval: 10 * time.Millisecond}
	if err := w.Start(context.Background(), func(p string) { events <- p }, nil); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	path := filepath.Join(real, "Screenshot restored.png")
	w.Suppress(path)
	writeTestFile(t, path, fakeScreenshot())
	noPath(t, events)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, fakeScreenshot())
	receivePath(t, events, filepath.Join(alias, filepath.Base(path)))
}
