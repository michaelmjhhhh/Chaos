package desktop

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func thumbnailPNG(t *testing.T, width int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, 2))
	img.Set(0, 0, color.White)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func takeUI(t *testing.T, queue <-chan func()) func() {
	t.Helper()
	select {
	case f := <-queue:
		return f
	case <-time.After(3 * time.Second):
		t.Fatal("thumbnail completion was not dispatched")
		return nil
	}
}

func TestThumbnailNewIntakeReloadsSamePath(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "successful", true: "missing"}[missing], func(t *testing.T) {
			queue := make(chan func(), 8)
			calls := make(chan string, 8)
			first := true
			data := thumbnailPNG(t, 5)
			oldData := thumbnailPNG(t, 3)
			d := New(nil, Actions{Dispatch: func(f func()) { queue <- f }, LoadThumbnail: func(_ context.Context, path string) ([]byte, error) {
				calls <- path
				if first {
					first = false
					if missing {
						return nil, os.ErrNotExist
					}
					return oldData, nil
				}
				return data, nil
			}})
			t.Cleanup(d.Close)
			d.SetSnapshot(app.Snapshot{CurrentPath: "/same.png", CurrentStage: "success"})
			takeUI(t, queue)()
			<-calls
			d.SetSnapshot(app.Snapshot{CurrentPath: "/same.png", CurrentStage: "analyzing", Busy: true})
			takeUI(t, queue)()
			if got := <-calls; got != "/same.png" {
				t.Fatal(got)
			}
			if d.thumbnail("/same.png") == nil {
				t.Fatal("new intake did not replace cached result")
			}
			if width, _ := d.thumbnail("/same.png").Size(); width != 5 {
				t.Fatalf("replacement preview width=%d, want 5", width)
			}
		})
	}
}

func TestThumbnailHistoryIdentityReloadsSamePath(t *testing.T) {
	queue := make(chan func(), 8)
	data := thumbnailPNG(t, 3)
	d := New(nil, Actions{Dispatch: func(f func()) { queue <- f }, LoadThumbnail: func(context.Context, string) ([]byte, error) { return data, nil }})
	t.Cleanup(d.Close)
	d.SetSnapshot(app.Snapshot{RecentFiles: []core.RecentFile{{ID: "old", Path: "/same.png"}}})
	takeUI(t, queue)()
	d.SetSnapshot(app.Snapshot{RecentFiles: []core.RecentFile{{ID: "new", Path: "/same.png"}}})
	takeUI(t, queue)()
}

func TestThumbnailQueueIsBounded(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	defer close(release)
	d := New(nil, Actions{Dispatch: func(func()) {}, LoadThumbnail: func(ctx context.Context, _ string) ([]byte, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, os.ErrNotExist
	}})
	t.Cleanup(d.Close)
	d.requestThumbnail("/worker-one.png")
	d.requestThumbnail("/worker-two.png")
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	for i := 0; i < 1000; i++ {
		d.requestThumbnail(string(rune(i + 1)))
	}
	if got := len(d.thumbnails.entries); got > 34 {
		t.Fatalf("retained %d pending requests; work must be bounded", got)
	}
}

func TestWarningDismissBelongsToWarning(t *testing.T) {
	d, ctl := fixture()
	d.snapshot.Warning = "Clipboard unavailable"
	tt := ui.NewTester(d.View, 1000, 800)
	click(t, tt, "Dismiss")
	expectCall(t, ctl, "dismiss-warning")
}

func TestThumbnailStaleCompletionCannotReplaceNewIntake(t *testing.T) {
	queue := make(chan func(), 8)
	data := thumbnailPNG(t, 3)
	d := New(nil, Actions{Dispatch: func(f func()) { queue <- f }, LoadThumbnail: func(context.Context, string) ([]byte, error) { return data, nil }})
	t.Cleanup(d.Close)
	d.SetSnapshot(app.Snapshot{CurrentPath: "/same.png", CurrentStage: app.StageSuccess})
	stale := takeUI(t, queue)
	d.SetSnapshot(app.Snapshot{CurrentPath: "/same.png", CurrentStage: app.StageAnalyzing, Busy: true})
	takeUI(t, queue)()
	current := d.thumbnail("/same.png")
	if current == nil {
		t.Fatal("replacement did not load")
	}
	stale()
	if d.thumbnail("/same.png") != current {
		t.Fatal("old completion replaced new intake")
	}
}

func TestThumbnailCloseCancelsWorkersAndIgnoresQueuedCompletion(t *testing.T) {
	queue := make(chan func(), 8)
	started := make(chan string, 4)
	canceled := make(chan struct{}, 4)
	data := thumbnailPNG(t, 3)
	d := New(nil, Actions{Dispatch: func(f func()) { queue <- f }, LoadThumbnail: func(ctx context.Context, path string) ([]byte, error) {
		if path == "/ready.png" {
			return data, nil
		}
		started <- path
		<-ctx.Done()
		canceled <- struct{}{}
		return nil, ctx.Err()
	}})
	t.Cleanup(d.thumbnails.cancel)
	d.requestThumbnail("/ready.png")
	late := takeUI(t, queue)
	d.requestThumbnail("/active.png")
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	d.requestThumbnail("/queued.png")
	d.Close()
	d.Close()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("loader was not canceled")
	}
	select {
	case <-d.thumbnails.done:
	case <-time.After(3 * time.Second):
		t.Fatal("workers did not exit")
	}
	late()
	d.requestThumbnail("/after-close.png")
	d.SetSnapshot(app.Snapshot{Revision: 9, CurrentPath: "/after-close.png"})
	if len(d.thumbnails.entries) != 0 || d.snapshot.Revision == 9 {
		t.Fatal("closed view accepted late work")
	}
}
