package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func ScreenshotCandidate(name string) bool {
	return (strings.HasPrefix(name, "Screenshot") || strings.HasPrefix(name, "屏幕快照") || strings.HasPrefix(name, "截屏")) && strings.EqualFold(filepath.Ext(name), ".png")
}
func ScreenshotEligible(path string, started time.Time) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || !ScreenshotCandidate(filepath.Base(path)) || info.Size() < 20*1024 || info.Size() > 25*1024*1024 || info.ModTime().Before(started.Add(-2*time.Second)) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 8)
	_, err = f.Read(b)
	return err == nil && bytes.Equal(b, pngSignature)
}

type watchedFile struct {
	info   os.FileInfo
	stable int
}
type suppression struct {
	until  time.Time
	info   os.FileInfo
	stable int
}
type Watcher struct {
	Directory  string
	Interval   time.Duration
	mu         sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
	seen       map[string]os.FileInfo
	pending    map[string]watchedFile
	suppressed map[string]suppression
}

func (w *Watcher) Start(ctx context.Context, onFile func(string), onError func(error)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return errors.New("watcher already running")
	}
	entries, err := os.ReadDir(w.Directory)
	if err != nil {
		return fmt.Errorf("could not watch directory: %w", err)
	}
	w.seen = map[string]os.FileInfo{}
	w.pending = map[string]watchedFile{}
	w.suppressed = map[string]suppression{}
	// Baseline is captured synchronously: even very recent existing screenshots
	// are ignored, unlike the old approximate modification-time cutoff.
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil {
			w.seen[entry.Name()] = info
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	interval := w.Interval
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	started := time.Now()
	done := w.done
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var lastError string
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				paths, err := w.scan(started)
				if err != nil {
					if err.Error() != lastError && onError != nil {
						onError(err)
					}
					lastError = err.Error()
					continue
				}
				lastError = ""
				for _, p := range paths {
					if ctx.Err() != nil {
						return
					}
					if onFile != nil {
						onFile(p)
					}
				}
			}
		}
	}()
	return nil
}
func sameFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// A restoration can be published through an exclusive copy on filesystems
// without hard links. A stable header alone must not end suppression while that
// copy is paused: wait for PNG's terminal IEND chunk as well. This deliberately
// ignores screenshot eligibility (size and capture age), since Undo can restore
// an old or small image that should still release its temporary suppression.
func completeRestoredPNG(path string, info os.FileInfo) bool {
	if info.Size() < int64(len(pngSignature)+12) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !sameFile(info, opened) {
		return false
	}
	var header [8]byte
	var end [12]byte
	if _, err = f.ReadAt(header[:], 0); err != nil || !bytes.Equal(header[:], pngSignature) {
		return false
	}
	if _, err = f.ReadAt(end[:], info.Size()-int64(len(end))); err != nil {
		return false
	}
	iend := []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
	if !bytes.Equal(end[:], iend) {
		// Go's PNG decoder accepts bytes after IEND. Walk framed chunks for
		// those images instead of retaining suppression for their trailing data.
		// The section reader bounds every read to the observed file size.
		r := bufio.NewReader(io.NewSectionReader(f, 8, info.Size()-8))
		remaining := info.Size() - 8
		complete := false
		for remaining >= 12 {
			var chunk [8]byte
			if _, err = io.ReadFull(r, chunk[:]); err != nil {
				return false
			}
			length := int64(binary.BigEndian.Uint32(chunk[:4]))
			if length > remaining-12 {
				return false
			}
			if string(chunk[4:]) == "IEND" {
				var crc [4]byte
				if length != 0 {
					return false
				}
				_, err = io.ReadFull(r, crc[:])
				complete = err == nil && bytes.Equal(crc[:], iend[8:])
				break
			}
			if _, err = io.CopyN(io.Discard, r, length+4); err != nil {
				return false
			}
			remaining -= length + 12
		}
		if !complete {
			return false
		}
	}
	final, err := f.Stat()
	return err == nil && sameFile(info, final)
}

func (w *Watcher) scan(started time.Time) ([]string, error) {
	entries, err := os.ReadDir(w.Directory)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var found []string
	present := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		present[name] = true
		if !ScreenshotCandidate(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if suppression, suppressed := w.suppressed[name]; suppressed {
			if time.Now().Before(suppression.until) && (suppression.info == nil || os.SameFile(suppression.info, info)) {
				if sameFile(suppression.info, info) {
					suppression.stable++
				} else {
					suppression.stable = 0
				}
				w.seen[name] = info
				delete(w.pending, name)
				suppression.info = info
				if suppression.stable == 2 && completeRestoredPNG(filepath.Join(w.Directory, name), info) {
					// The restored bytes are now the baseline. Do not keep matching
					// only their inode: Linux may immediately reuse it for a new
					// capture, and an in-place rewrite keeps the inode everywhere.
					delete(w.suppressed, name)
				} else {
					w.suppressed[name] = suppression
				}
				continue
			}
			delete(w.suppressed, name)
		}
		if sameFile(w.seen[name], info) {
			continue
		}
		pending := w.pending[name]
		if sameFile(pending.info, info) {
			pending.stable++
		} else {
			pending = watchedFile{info: info}
		}
		w.pending[name] = pending
		if pending.stable < 2 {
			continue
		}
		path := filepath.Join(w.Directory, name)
		if !ScreenshotEligible(path, started) {
			continue
		}
		w.seen[name] = info
		delete(w.pending, name)
		found = append(found, path)
	}
	for name := range w.seen {
		if !present[name] {
			delete(w.seen, name)
			if s, ok := w.suppressed[name]; ok && s.info != nil {
				delete(w.suppressed, name)
			}
		}
	}
	for name := range w.pending {
		if !present[name] {
			delete(w.pending, name)
		}
	}
	return found, nil
}

// Suppress excludes a restored file from this watcher's current identity set.
func (w *Watcher) Suppress(path string) {
	// macOS commonly reports /tmp through a /private/tmp alias; the controller
	// canonicalizes selected paths while the watcher retains the chosen spelling.
	// Compare the directories' identities, not their textual path prefixes.
	if filepath.Clean(filepath.Dir(path)) != filepath.Clean(w.Directory) {
		sourceDirectory, sourceErr := os.Stat(filepath.Dir(path))
		watchDirectory, watchErr := os.Stat(w.Directory)
		if sourceErr != nil || watchErr != nil || !os.SameFile(sourceDirectory, watchDirectory) {
			return
		}
	}
	info, err := os.Lstat(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seen != nil {
		if err == nil {
			w.seen[filepath.Base(path)] = info
			delete(w.suppressed, filepath.Base(path))
		} else if errors.Is(err, os.ErrNotExist) {
			w.suppressed[filepath.Base(path)] = suppression{until: time.Now().Add(time.Minute)}
		}
		delete(w.pending, filepath.Base(path))
	}
}
func (w *Watcher) Stop() {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
		w.mu.Lock()
		if w.done == done {
			w.cancel = nil
		}
		w.mu.Unlock()
	}
}
