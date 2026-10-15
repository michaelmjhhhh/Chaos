package desktop

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
)

func thumbnailFixture(t *testing.T, load func(context.Context, string) (image.Image, error)) (*Desktop, <-chan func(), *time.Time) {
	t.Helper()
	updates := make(chan func(), thumbnailCacheSize)
	d := New(nil, Actions{LoadThumbnail: load, Dispatch: func(fn func()) { updates <- fn }})
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	d.now = func() time.Time { return now }
	t.Cleanup(d.Close)
	return d, updates, &now
}

func applyThumbnailUpdate(t *testing.T, updates <-chan func()) {
	t.Helper()
	select {
	case update := <-updates:
		update()
	case <-time.After(3 * time.Second):
		t.Fatal("thumbnail worker did not publish")
	}
}

func writeThumbnailSource(t *testing.T, path string, width byte) {
	t.Helper()
	if err := os.WriteFile(path, []byte{width}, 0600); err != nil {
		t.Fatal(err)
	}
}

func thumbnailWidth(t *testing.T, bitmap *ui.Bitmap) int {
	t.Helper()
	if bitmap == nil {
		t.Fatal("thumbnail is missing")
	}
	w, _ := bitmap.Size()
	return w
}

func TestThumbnailsRefreshReplacedFilesAndReuseUnchangedImages(t *testing.T) {
	var calls atomic.Int32
	d, updates, now := thumbnailFixture(t, func(_ context.Context, path string) (image.Image, error) {
		calls.Add(1)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return image.NewRGBA(image.Rect(0, 0, int(data[0]), 1)), nil
	})
	path := filepath.Join(t.TempDir(), "source.png")
	writeThumbnailSource(t, path, 2)
	d.requestThumbnail(path)
	applyThumbnailUpdate(t, updates)
	first := d.requestThumbnail(path)
	if got := thumbnailWidth(t, first); got != 2 {
		t.Fatalf("initial width = %d", got)
	}
	*now = now.Add(thumbnailRefresh)
	d.requestThumbnail(path)
	applyThumbnailUpdate(t, updates)
	if d.requestThumbnail(path) != first || calls.Load() != 1 {
		t.Fatal("unchanged image was decoded again")
	}
	// Preserve size and mtime: file identity must still detect a replacement.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".replacement"
	writeThumbnailSource(t, replacement, 7)
	if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(thumbnailRefresh)
	d.requestThumbnail(path)
	applyThumbnailUpdate(t, updates)
	if got := thumbnailWidth(t, d.requestThumbnail(path)); got != 7 || calls.Load() != 2 {
		t.Fatalf("replacement width=%d loads=%d", got, calls.Load())
	}
}

func TestThumbnailFailuresRetryWithoutSpinning(t *testing.T) {
	var calls atomic.Int32
	d, updates, now := thumbnailFixture(t, func(context.Context, string) (image.Image, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("temporary decoding failure")
		}
		return image.NewRGBA(image.Rect(0, 0, 3, 1)), nil
	})
	path := filepath.Join(t.TempDir(), "restored.png")
	d.requestThumbnail(path) // missing file
	applyThumbnailUpdate(t, updates)
	writeThumbnailSource(t, path, 3)
	for range 10 {
		d.requestThumbnail(path)
	}
	if calls.Load() != 0 {
		t.Fatal("failed path was retried before its retry interval")
	}
	*now = now.Add(thumbnailRefresh)
	d.requestThumbnail(path)
	applyThumbnailUpdate(t, updates) // transient loader error
	if d.requestThumbnail(path) != nil || calls.Load() != 1 {
		t.Fatal("failed decode was cached as a usable image or retried immediately")
	}
	*now = now.Add(thumbnailRefresh)
	d.requestThumbnail(path)
	applyThumbnailUpdate(t, updates)
	if got := thumbnailWidth(t, d.requestThumbnail(path)); got != 3 || calls.Load() != 2 {
		t.Fatalf("restored image width=%d loads=%d", got, calls.Load())
	}
}

func TestThumbnailQueueIsBoundedAndCloseCancelsWork(t *testing.T) {
	started := make(chan struct{}, thumbnailWorkers)
	var calls atomic.Int32
	d, _, _ := thumbnailFixture(t, func(ctx context.Context, _ string) (image.Image, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	dir := t.TempDir()
	for i := range thumbnailWorkers {
		path := filepath.Join(dir, fmt.Sprintf("active-%d.png", i))
		writeThumbnailSource(t, path, 1)
		d.requestThumbnail(path)
	}
	for range thumbnailWorkers {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("thumbnail worker did not start")
		}
	}
	for i := range 100 {
		path := filepath.Join(dir, fmt.Sprintf("queued-%d.png", i))
		writeThumbnailSource(t, path, 1)
		d.requestThumbnail(path)
	}
	if len(d.thumbnailLoader.requests) != thumbnailQueueSize || len(d.thumbnails) > thumbnailQueueSize+thumbnailWorkers {
		t.Fatalf("unbounded admission: queue=%d entries=%d", len(d.thumbnailLoader.requests), len(d.thumbnails))
	}
	closed := make(chan struct{})
	go func() { d.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not cancel thumbnail workers")
	}
	if calls.Load() != thumbnailWorkers {
		t.Fatal("queued work started after cancellation")
	}
}

func TestThumbnailPublicationAfterCloseIsIgnored(t *testing.T) {
	d, updates, _ := thumbnailFixture(t, func(context.Context, string) (image.Image, error) {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	})
	path := filepath.Join(t.TempDir(), "source.png")
	writeThumbnailSource(t, path, 1)
	d.requestThumbnail(path)
	var update func()
	select {
	case update = <-updates:
	case <-time.After(3 * time.Second):
		t.Fatal("thumbnail worker did not publish")
	}
	d.Close()
	update()
	if d.thumbnails[path].bitmap != nil {
		t.Fatal("late callback published an image after Close")
	}
}

func TestOldThumbnailCannotOverwriteNewIntakeAtTheSamePath(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var calls atomic.Int32
	d, updates, _ := thumbnailFixture(t, func(ctx context.Context, _ string) (image.Image, error) {
		width := 7
		if calls.Add(1) == 1 {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			width = 2
		}
		return image.NewRGBA(image.Rect(0, 0, width, 1)), nil
	})
	t.Cleanup(unblock)
	path := filepath.Join(t.TempDir(), "reused.png")
	writeThumbnailSource(t, path, 1)
	d.requestThumbnail(path)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first thumbnail did not start")
	}
	d.SetSnapshot(app.Snapshot{Revision: 1, Busy: true, CurrentPath: path, CurrentStage: app.StageAnalyzing})
	applyThumbnailUpdate(t, updates)
	if got := thumbnailWidth(t, d.requestThumbnail(path)); got != 7 {
		t.Fatalf("new intake thumbnail width = %d", got)
	}
	unblock()
	applyThumbnailUpdate(t, updates)
	if got := thumbnailWidth(t, d.requestThumbnail(path)); got != 7 {
		t.Fatalf("old completion replaced the new thumbnail with width %d", got)
	}
}

func TestThumbnailCacheKeepsRecentlyUsedImagesWithinLimit(t *testing.T) {
	d, updates, _ := thumbnailFixture(t, func(context.Context, string) (image.Image, error) {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	})
	dir := t.TempDir()
	var paths []string
	for i := range thumbnailCacheSize + 1 {
		path := filepath.Join(dir, fmt.Sprintf("image-%d.png", i))
		paths = append(paths, path)
		writeThumbnailSource(t, path, 1)
		d.requestThumbnail(path)
		applyThumbnailUpdate(t, updates)
		d.requestThumbnail(paths[0])
	}
	if len(d.thumbnails) != thumbnailCacheSize || d.thumbnails[paths[0]] == nil || d.thumbnails[paths[1]] != nil {
		t.Fatal("cache did not evict the least recently used completed image")
	}
}
