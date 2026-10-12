package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func testController(t *testing.T, handler http.HandlerFunc, effects Effects) (*Controller, string) {
	t.Helper()
	dir := t.TempDir()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	watch, out := filepath.Join(dir, "watch"), filepath.Join(dir, "out")
	if err := os.MkdirAll(watch, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := core.Config{Provider: "openai-compatible", APIKey: "test-key", BaseURL: server.URL, WatchDir: watch, OutputDir: out, FilenameTemplate: "{slug}", CopyToClipboard: effects.CopyImage != nil}
	if err := core.SaveConfig(filepath.Join(dir, "config.json"), cfg); err != nil {
		t.Fatal(err)
	}
	c, err := New(Options{DataDir: dir, Home: dir, Client: &core.Client{HTTP: server.Client()}, Effects: effects, WatchInterval: 10 * time.Millisecond, DeviceHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c, watch
}

func successHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"terminal-git-log"}}]}`))
}

func writePNG(t *testing.T, path string, large bool) []byte {
	t.Helper()
	size := 12
	if large {
		size = 200
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	var v uint32 = 0x19831
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			v = v*1664525 + 1013904223
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(v), G: uint8(v >> 8), B: uint8(v >> 16), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func eventually(t *testing.T, why string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", why)
}

func TestFilingRenameReopenAndRevertPreserveBytes(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{})
	source := filepath.Join(watch, "original.png")
	original := writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	eventually(t, "filed", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
	r := c.Snapshot().RecentFiles[0]
	if r.IsError || r.NewName != "terminal-git-log.png" {
		t.Fatalf("result: %+v", r)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists: %v", err)
	}
	c.Rename(r.ID, "corrected name")
	eventually(t, "renamed", func() bool { return c.Snapshot().RecentFiles[0].NewName == "corrected-name.png" })
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(Options{DataDir: c.dataDir, Home: c.home, DeviceHash: c.deviceHash})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.Snapshot().RecentFiles[0]; got.ID != r.ID || got.NewName != "corrected-name.png" {
		t.Fatalf("reopened: %+v", got)
	}
	reopened.Revert(r.ID)
	eventually(t, "reverted", func() bool { return len(reopened.Snapshot().RecentFiles) == 0 })
	b, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(original, b) {
		t.Fatalf("original bytes lost: %v", err)
	}
	if reopened.Snapshot().Insights.TotalProcessed != 0 {
		t.Fatal("insights retained undone image")
	}
}

func TestClipboardFailureRemainsDurableSuccess(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{CopyImage: func(string) error { return errors.New("clipboard denied") }})
	source := filepath.Join(watch, "clip.png")
	writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	eventually(t, "clipboard warning", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy && s.Warning != "" })
	s := c.Snapshot()
	if s.RecentFiles[0].IsError || s.Successes != 1 || s.Errors != 0 {
		t.Fatalf("filed image became failure: %+v", s.RecentFiles)
	}
	if _, err := os.Stat(s.RecentFiles[0].Path); err != nil {
		t.Fatal(err)
	}
}

func TestSameNameInitialFilingHasDistinctDestinationAndUndo(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{})
	cfg := c.Snapshot().Config
	cfg.OutputDir = watch
	c.SaveConfig(cfg)
	source := filepath.Join(watch, "terminal-git-log.png")
	original := writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	eventually(t, "same-name image filed", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
	r := c.Snapshot().RecentFiles[0]
	if r.IsError || r.Path == source || r.NewName != "terminal-git-log-2.png" {
		t.Fatalf("filing needs a distinct journal destination: %+v", r)
	}
	c.Revert(r.ID)
	eventually(t, "same-name filing undone", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 0 && !s.Busy })
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("undo lost original: %v", err)
	}
}

func TestSourceReplacedDuringAnalysisIsPreserved(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c, watch := testController(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		successHandler(w, r)
	}, Effects{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	source := filepath.Join(watch, "original.png")
	writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider not called")
	}
	replacement := filepath.Join(watch, "replacement.png")
	want := writePNG(t, replacement, true)
	if err := os.Rename(replacement, source); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	eventually(t, "source change reported", func() bool { s := c.Snapshot(); return !s.Busy && len(s.RecentFiles) == 1 })
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, want) || !c.Snapshot().RecentFiles[0].IsError {
		t.Fatalf("replacement was filed or lost: %v; history=%+v", err, c.Snapshot().RecentFiles)
	}
}

func TestRecoveryRejectsSameLocationWithoutDeletingImage(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{})
	source := filepath.Join(watch, "unchanged.png")
	want := writePNG(t, source, false)
	op := operation{Kind: "insert", Source: source, Destination: source, Record: core.RecentFile{ID: newUUID()}}
	if err := op.fingerprint(); err != nil {
		t.Fatal(err)
	}
	if err := c.writeOperation(op); err != nil {
		t.Fatal(err)
	}
	if err := c.recoverOperations(); err == nil {
		t.Fatal("same-location journal was accepted")
	}
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("recovery deleted image: %v", err)
	}
}

func TestAppearanceChangeRetainsConnectionHealth(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	c.CheckHealth()
	eventually(t, "healthy connection", func() bool { return c.Snapshot().APIStatus == "OK" })
	cfg := c.Snapshot().Config
	cfg.Appearance = "dark"
	c.SaveConfig(cfg)
	if c.Snapshot().APIStatus != "OK" {
		t.Fatal("appearance change discarded connection health")
	}
	cfg.APIKey = "different-key"
	c.SaveConfig(cfg)
	if c.Snapshot().APIStatus != "N/A" {
		t.Fatal("credential change retained stale connection health")
	}
}

func TestQueueDeduplicatesConcurrentIntake(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	c, watch := testController(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		successHandler(w, r)
	}, Effects{})
	source := filepath.Join(watch, "duplicate.png")
	writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider not called")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.enqueue([]string{source, source}, false) }()
	}
	wg.Wait()
	close(release)
	eventually(t, "one durable success", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
	if calls.Load() != 1 {
		t.Fatalf("%d duplicate paid calls", calls.Load())
	}
}

func TestStopCancelsWatchedRequestWithoutMovingSource(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	c, watch := testController(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["max_tokens"] == float64(8) {
			successHandler(w, r)
			return
		}
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}, Effects{})
	c.Start()
	eventually(t, "watcher started", func() bool { return c.Snapshot().Watching })
	source := filepath.Join(watch, "Screenshot cancel.png")
	writePNG(t, source, true)
	select {
	case <-started:
	case <-time.After(4 * time.Second):
		t.Fatal("watcher did not call provider")
	}
	c.Stop()
	eventually(t, "watcher and request stopped", func() bool { s := c.Snapshot(); return !s.Watching && !s.Busy })
	if _, err := os.Stat(source); err != nil {
		t.Fatal("cancel lost source", err)
	}
	if len(c.Snapshot().RecentFiles) != 0 {
		t.Fatal("cancellation was recorded as a failed attempt")
	}
}

func TestRapidStopStartPreservesNewestIntent(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)
	c.Start()
	eventually(t, "started", func() bool { return c.Snapshot().Watching })
	for i := 0; i < 20; i++ {
		c.Stop()
		c.Start()
		time.Sleep(20 * time.Millisecond)
		if !c.Snapshot().Watching {
			t.Fatalf("latest Start lost on iteration %d", i)
		}
	}
}

func TestUndoWhileWatchingThroughSymlinkDoesNotRefile(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{})
	alias := filepath.Join(c.dataDir, "watch-alias")
	if err := os.Symlink(watch, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	cfg := c.Snapshot().Config
	cfg.WatchDir = alias
	c.SaveConfig(cfg)
	c.Start()
	eventually(t, "watching alias", func() bool { return c.Snapshot().Watching })
	source := filepath.Join(watch, "Screenshot restore.png")
	writePNG(t, source, true)
	eventually(t, "filed", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
	c.Revert(c.Snapshot().RecentFiles[0].ID)
	eventually(t, "undone", func() bool { return len(c.Snapshot().RecentFiles) == 0 })
	time.Sleep(200 * time.Millisecond)
	if s := c.Snapshot(); len(s.RecentFiles) != 0 || s.Busy {
		t.Fatal("Undo was immediately refiled")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("restored file missing", err)
	}
}

func TestCheckUsageDoesNotInvalidateNonHostedHealth(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c, _ := testController(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-release; successHandler(w, r) }, Effects{})
	c.CheckHealth()
	<-started
	c.CheckUsage()
	close(release)
	eventually(t, "health completes despite unrelated usage action", func() bool { return c.Snapshot().APIStatus == "OK" })
}

func TestCorruptConfigIsNotOverwrittenByPreferenceChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	corrupt := []byte("not-json")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := New(Options{DataDir: dir, Home: dir})
	if err != nil {
		t.Fatal(err)
	}
	if c.Snapshot().StorageError == "" {
		t.Fatal("corrupt settings silently ignored")
	}
	c.SetAutoStart(true)
	_ = c.Close()
	b, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(b, corrupt) {
		t.Fatal("corrupt config overwritten", err)
	}
}

func TestDeviceIdentityMatchesSwiftAndUsesExactKeychainFallback(t *testing.T) {
	for _, keychain := range []bool{false, true} {
		t.Run(map[bool]string{false: "hardware", true: "keychain"}[keychain], func(t *testing.T) {
			seed := "0123ABCD-5678-4000-ABCD-123456789ABC"
			run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "/usr/sbin/ioreg" {
					if keychain {
						return nil, errors.New("unavailable")
					}
					return []byte(`"IOPlatformUUID" = "` + seed + `"`), nil
				}
				if name == "/usr/bin/security" && strings.Join(args, " ") == "find-generic-password -s com.chaos.app.device -a device-seed -w" {
					return []byte(seed + "\n"), nil
				}
				t.Fatalf("unexpected native call: %s %v", name, args)
				return nil, nil
			}
			got, err := deviceHash(t.TempDir(), "darwin", run)
			if err != nil {
				t.Fatal(err)
			}
			want := sha256.Sum256([]byte("chaos.device.v1" + seed))
			if got != hex.EncodeToString(want[:]) {
				t.Fatal("existing trial identity changed")
			}
		})
	}
}

func TestRecoveryCommitsPublishedMoveIdempotently(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "filed.png")
	original := writePNG(t, source, false)
	r := core.RecentFile{ID: "RECOVERY-ID", OriginalName: "source.png", NewName: "filed.png", SourcePath: source, Path: destination, Timestamp: time.Now(), Duration: 1}
	op := operation{Kind: "insert", Record: r, Source: source, Destination: destination}
	if err := op.fingerprint(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pending"), 0700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(op)
	if err := os.WriteFile(filepath.Join(dir, "pending", r.ID+".json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	// Crash after publication but before removing the original and committing DB.
	if err := os.WriteFile(destination, original, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		c, err := New(Options{DataDir: dir, Home: dir})
		if err != nil {
			t.Fatal(err)
		}
		if s := c.Snapshot(); len(s.RecentFiles) != 1 || s.RecentFiles[0].ID != r.ID || s.StorageError != "" {
			t.Fatalf("recovery: %+v, %s", s.RecentFiles, s.StorageError)
		}
		_ = c.Close()
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("recovery did not finish move")
	}
	actual, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(actual, original) {
		t.Fatal("recovery changed image")
	}
}

func TestRecoveryPreservesReplacedSource(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source.png"), filepath.Join(dir, "filed.png")
	original := writePNG(t, source, false)
	op := operation{Kind: "insert", Record: core.RecentFile{ID: "REPLACED", Timestamp: time.Now()}, Source: source, Destination: destination}
	_ = op.fingerprint()
	_ = os.MkdirAll(filepath.Join(dir, "pending"), 0700)
	b, _ := json.Marshal(op)
	_ = os.WriteFile(filepath.Join(dir, "pending", "REPLACED.json"), b, 0600)
	_ = os.WriteFile(destination, original, 0600)
	_ = os.WriteFile(source, []byte("new unrelated file"), 0600)
	c, err := New(Options{DataDir: dir, Home: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Snapshot().StorageError == "" {
		t.Fatal("conflict not surfaced")
	}
	if b, _ := os.ReadFile(source); string(b) != "new unrelated file" {
		t.Fatal("replaced source deleted")
	}
}

func TestHealthResultCannotOverrideChangedProvider(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c, _ := testController(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-release; successHandler(w, r) }, Effects{})
	c.CheckHealth()
	<-started
	cfg := c.Snapshot().Config
	cfg.Model = "new-model"
	c.SaveConfig(cfg)
	close(release)
	time.Sleep(100 * time.Millisecond)
	if c.Snapshot().APIStatus != "N/A" {
		t.Fatal("stale health result overwrote settings revision")
	}
}

func TestSnapshotsDoNotExposeMutableControllerState(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	cfg := c.Snapshot().Config
	cfg.ProviderKeys = map[string]string{"openai": "original"}
	c.SaveConfig(cfg)
	s := c.Snapshot()
	s.Config.ProviderKeys["openai"] = "changed"
	s.Insights.Daily["2001-01-01"] = 99
	if next := c.Snapshot(); next.Config.ProviderKeys["openai"] != "original" || next.Insights.Daily["2001-01-01"] != 0 {
		t.Fatal("snapshot mutated live state")
	}
}
