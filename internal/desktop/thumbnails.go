package desktop

import (
	"context"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

const thumbnailCacheLimit = 64
const thumbnailQueueLimit = 32
const thumbnailWorkers = 2

type thumbnailEntry struct {
	generation uint64
	bitmap     *ui.Bitmap
}
type thumbnailJob struct {
	path       string
	generation uint64
}

// Workers only own immutable jobs and decoded results. Entries and generations
// belong to the UI thread, including the dispatched completion below.
type thumbnailCache struct {
	entries    map[string]thumbnailEntry
	order      []string
	generation uint64
	jobs       chan thumbnailJob
	cancel     context.CancelFunc
	done       chan struct{}
	closed     bool
}

func coreProviderLabel(provider, model string) string {
	p := core.ProviderByID(provider)
	if model == "" {
		model = p.Model
	}
	return p.Name + " · " + model
}

func (d *Desktop) startThumbnails() {
	d.thumbnails.entries = make(map[string]thumbnailEntry)
	if d.actions.Dispatch == nil {
		return
	}
	if d.actions.LoadThumbnail == nil {
		panic("desktop: Dispatch requires LoadThumbnail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.thumbnails.cancel = cancel
	d.thumbnails.jobs = make(chan thumbnailJob, thumbnailQueueLimit)
	d.thumbnails.done = make(chan struct{})
	dispatch, loader := d.actions.Dispatch, d.actions.LoadThumbnail
	var workers sync.WaitGroup
	for i := 0; i < thumbnailWorkers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-d.thumbnails.jobs:
					if ctx.Err() != nil {
						return
					}
					data, err := loader(ctx, job.path)
					var bitmap *ui.Bitmap
					if err == nil && ctx.Err() == nil {
						bitmap, err = ui.DecodeBitmap(data)
					}
					if ctx.Err() != nil {
						return
					}
					dispatch(func() {
						entry, ok := d.thumbnails.entries[job.path]
						if d.thumbnails.closed || !ok || entry.generation != job.generation {
							return
						}
						if err == nil {
							entry.bitmap = bitmap
						}
						d.thumbnails.entries[job.path] = entry
					})
				}
			}
		}()
	}
	go func() { workers.Wait(); close(d.thumbnails.done) }()
}

// Close must run on the UI thread, before the shell shuts down dispatch. It does
// not wait for workers; cancellation interrupts the injected loader.
func (d *Desktop) Close() {
	if d.thumbnails.closed {
		return
	}
	d.thumbnails.closed = true
	if d.thumbnails.cancel != nil {
		d.thumbnails.cancel()
	}
	d.thumbnails.entries = nil
	d.thumbnails.order = nil
}

func (d *Desktop) thumbnail(path string) *ui.Bitmap { return d.thumbnails.entries[path].bitmap }

func (d *Desktop) invalidateThumbnail(path string) {
	delete(d.thumbnails.entries, path)
	for i, p := range d.thumbnails.order {
		if p == path {
			d.thumbnails.order = append(d.thumbnails.order[:i], d.thumbnails.order[i+1:]...)
			break
		}
	}
}

func (d *Desktop) invalidateThumbnails(s app.Snapshot) {
	// Starting another intake at the same path must retry both cached failures and
	// successful previews. A phase transition also covers back-to-back batch items.
	if s.Busy && s.CurrentStage == app.StageAnalyzing && (!d.snapshot.Busy || d.snapshot.CurrentStage != app.StageAnalyzing || s.CurrentPath != d.snapshot.CurrentPath) {
		d.invalidateThumbnail(s.CurrentPath)
	}
	previous := make(map[string]core.RecentFile, len(d.snapshot.RecentFiles))
	for _, f := range d.snapshot.RecentFiles {
		if _, ok := previous[f.Path]; !ok {
			previous[f.Path] = f
		}
	}
	seen := make(map[string]bool)
	for _, f := range s.RecentFiles {
		if f.Path == "" || seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		old, ok := previous[f.Path]
		if !ok || old.ID != f.ID || !old.Timestamp.Equal(f.Timestamp) || old.IsError != f.IsError {
			d.invalidateThumbnail(f.Path)
		}
	}
}

func (d *Desktop) requestThumbnail(path string) {
	if path == "" || d.thumbnails.closed || d.thumbnails.jobs == nil {
		return
	}
	if _, ok := d.thumbnails.entries[path]; ok {
		return
	}
	d.thumbnails.generation++
	job := thumbnailJob{path: path, generation: d.thumbnails.generation}
	select {
	case d.thumbnails.jobs <- job:
		if len(d.thumbnails.order) >= thumbnailCacheLimit {
			d.invalidateThumbnail(d.thumbnails.order[0])
		}
		d.thumbnails.entries[path] = thumbnailEntry{generation: job.generation}
		d.thumbnails.order = append(d.thumbnails.order, path)
	default:
		// Leave it uncached: a later visible-row build can retry after work drains.
	}
}
