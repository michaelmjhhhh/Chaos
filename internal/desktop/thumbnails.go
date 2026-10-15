package desktop

import (
	"context"
	"image"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	thumbnailWorkers   = 2
	thumbnailQueueSize = 16
	thumbnailCacheSize = 64
	thumbnailRefresh   = 5 * time.Second
)

// Entries belong to the UI thread. A failed load has a nil bitmap and is
// retried after checked, rather than being confused with an in-flight request.
type thumbnailEntry struct {
	bitmap  *ui.Bitmap
	info    os.FileInfo
	checked time.Time
	used    uint64
	pending bool
}

type thumbnailRequest struct {
	path   string
	entry  *thumbnailEntry
	bitmap *ui.Bitmap
	info   os.FileInfo
}

// Only this lifecycle state is shared with workers and Close. Workers receive
// copies of cached values; they never read or mutate the UI's entries.
type thumbnailLoader struct {
	ctx      context.Context
	cancel   context.CancelFunc
	requests chan thumbnailRequest
	workers  sync.WaitGroup
}

func (d *Desktop) startThumbnailLoader() {
	load, dispatch := d.actions.LoadThumbnail, d.actions.Dispatch
	if load == nil || dispatch == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	loader := &thumbnailLoader{ctx: ctx, cancel: cancel, requests: make(chan thumbnailRequest, thumbnailQueueSize)}
	d.thumbnailLoader = loader
	for range thumbnailWorkers {
		loader.workers.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case request := <-loader.requests:
					if ctx.Err() != nil {
						return
					}
					readCtx, stop := context.WithTimeout(ctx, 30*time.Second)
					bitmap, info := loadThumbnail(readCtx, request, load)
					stop()
					if ctx.Err() != nil {
						return
					}
					dispatch(func() {
						if ctx.Err() != nil || d.thumbnails[request.path] != request.entry {
							return
						}
						entry := request.entry
						entry.bitmap, entry.info = bitmap, info
						entry.checked, entry.pending = d.now(), false
					})
				}
			}
		})
	}
}

func loadThumbnail(ctx context.Context, request thumbnailRequest, load func(context.Context, string) (image.Image, error)) (*ui.Bitmap, os.FileInfo) {
	info, err := os.Lstat(request.path)
	if err != nil || !info.Mode().IsRegular() || ctx.Err() != nil {
		return nil, nil
	}
	if request.bitmap != nil && sameThumbnailFile(request.info, info) {
		return request.bitmap, info
	}
	img, err := load(ctx, request.path)
	if err != nil || img == nil || ctx.Err() != nil {
		return nil, nil
	}
	after, err := os.Lstat(request.path)
	if err != nil || !sameThumbnailFile(info, after) || ctx.Err() != nil {
		return nil, nil
	}
	return ui.NewBitmap(img), after
}

func sameThumbnailFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (d *Desktop) thumbnail(c *ui.Context, path string) *ui.Bitmap {
	bitmap := d.requestThumbnail(path)
	if path != "" && d.thumbnailLoader != nil {
		// Visible paths can disappear, be restored, or be replaced while idle.
		c.After(thumbnailRefresh)
	}
	return bitmap
}

func (d *Desktop) requestThumbnail(path string) *ui.Bitmap {
	loader := d.thumbnailLoader
	if path == "" || loader == nil || loader.ctx.Err() != nil {
		return nil
	}
	entry := d.thumbnails[path]
	if entry == nil {
		if len(d.thumbnails) >= thumbnailCacheSize {
			var oldest string
			var leastUsed uint64 = ^uint64(0)
			for key, cached := range d.thumbnails {
				if !cached.pending && cached.used < leastUsed {
					oldest, leastUsed = key, cached.used
				}
			}
			if oldest == "" {
				return nil
			}
			delete(d.thumbnails, oldest)
		}
		entry = &thumbnailEntry{}
		d.thumbnails[path] = entry
	}
	d.thumbnailUse++
	entry.used = d.thumbnailUse
	if entry.pending || (!entry.checked.IsZero() && d.now().Sub(entry.checked) < thumbnailRefresh) {
		return entry.bitmap
	}
	request := thumbnailRequest{path: path, entry: entry, bitmap: entry.bitmap, info: entry.info}
	select {
	case loader.requests <- request:
		entry.pending = true
	default:
		// The next render retries admission after a worker publishes a result.
		if entry.checked.IsZero() {
			delete(d.thumbnails, path)
		}
	}
	return entry.bitmap
}
