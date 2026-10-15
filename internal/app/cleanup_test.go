package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func TestSaveConfigDuringWatcherStartingUsesLatestDirectory(t *testing.T) {
	for _, rebind := range []bool{false, true} {
		t.Run(map[bool]string{false: "startup", true: "rebind"}[rebind], func(t *testing.T) {
			c, _ := testController(t, successHandler, Effects{})
			if rebind {
				c.Start()
				eventually(t, "initial watcher", func() bool { return c.Snapshot().Watching })
			}
			latest := filepath.Join(c.dataDir, "latest")
			if err := os.Mkdir(latest, 0700); err != nil {
				t.Fatal(err)
			}
			var edited atomic.Bool
			c.SetOnChange(func(s Snapshot) {
				if s.WatcherStatus == "Starting" && edited.CompareAndSwap(false, true) {
					cfg := s.Config
					cfg.WatchDir = latest
					c.SaveConfig(cfg)
				}
			})
			if rebind {
				cfg := c.Snapshot().Config
				cfg.WatchDir = filepath.Join(c.dataDir, "middle")
				if err := os.Mkdir(cfg.WatchDir, 0700); err != nil {
					t.Fatal(err)
				}
				c.SaveConfig(cfg)
			} else {
				c.Start()
			}
			eventually(t, "latest directory active", func() bool { return c.Snapshot().ActiveWatchDir == latest })
		})
	}
}

func TestSettingsRepairClearsOnlyOwnedError(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "settings", true: "pending-recovery"}[recovery], func(t *testing.T) {
			c, _ := testController(t, successHandler, Effects{})
			if recovery {
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(c.dataDir, "pending"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(c.dataDir, "pending", "broken.json"), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
				reopened, err := New(Options{DataDir: c.dataDir, Home: c.home})
				if err != nil {
					t.Fatal(err)
				}
				c = reopened
				t.Cleanup(func() { _ = c.Close() })
			}
			path := filepath.Join(c.dataDir, "preferences.json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			c.SetAutoStart(true)
			eventually(t, "settings failure", func() bool { return strings.Contains(c.Snapshot().StorageError, "preferences.json") })
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			c.SetAutoStart(false)
			eventually(t, "saved preferences", func() bool { p, err := loadPreferences(path, nil); return err == nil && !p.AutoStart })
			eventually(t, "settings error cleared", func() bool { return !strings.Contains(c.Snapshot().StorageError, "preferences.json") })
			if recovery && !strings.Contains(c.Snapshot().StorageError, "broken.json") {
				t.Fatal("successful save erased unresolved recovery")
			}
		})
	}
}

func TestRetryAllReportsMissingPathsThroughIntake(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	c.change(func(s *Snapshot) {
		s.RecentFiles = []core.RecentFile{{ID: "missing", IsError: true, SourcePath: filepath.Join(c.dataDir, "missing.png")}}
	})
	c.RetryAll()
	eventually(t, "missing retry warning", func() bool { return strings.Contains(c.Snapshot().Warning, "missing.png") })
}

func TestRetryPreservesFailedAttemptsAndDeduplicatesSources(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "all"}[all], func(t *testing.T) {
			var healthy atomic.Bool
			var calls atomic.Int32
			c, watch := testController(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if !healthy.Load() {
					http.Error(w, "invalid credentials", http.StatusUnauthorized)
					return
				}
				successHandler(w, r)
			}, Effects{})
			source := filepath.Join(watch, "retry.png")
			want := writePNG(t, source, false)
			c.ProcessFiles([]string{source})
			eventually(t, "first failed attempt", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
			failed := c.Snapshot().RecentFiles[0]
			if !failed.IsError {
				t.Fatal("expected provider failure")
			}
			if all {
				c.Retry(failed.ID)
				eventually(t, "second failed attempt", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 2 && !s.Busy })
			}
			healthy.Store(true)
			if all {
				c.RetryAll()
			} else {
				c.Retry(failed.ID)
			}
			wantRows := 2
			if all {
				wantRows = 3
			}
			eventually(t, "retry filed", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == wantRows && !s.Busy })
			s := c.Snapshot()
			var successes, failures int
			for _, r := range s.RecentFiles {
				if r.IsError {
					failures++
					continue
				}
				successes++
				got, err := os.ReadFile(r.Path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("retry lost image", err)
				}
				if r.ID == failed.ID {
					t.Fatal("retry overwrote prior attempt")
				}
			}
			if successes != 1 || failures != wantRows-1 || int(calls.Load()) != wantRows {
				t.Fatalf("retry counts: successes=%d failures=%d calls=%d", successes, failures, calls.Load())
			}
		})
	}
}

func TestUnknownMutationCannotRenameAnImage(t *testing.T) {
	c, watch := testController(t, successHandler, Effects{})
	source := filepath.Join(watch, "mutation.png")
	writePNG(t, source, false)
	c.ProcessFiles([]string{source})
	eventually(t, "filed", func() bool { s := c.Snapshot(); return len(s.RecentFiles) == 1 && !s.Busy })
	before := c.Snapshot().RecentFiles[0]
	c.mutate(context.Background(), job{kind: "unsupported", id: before.ID, name: "should-not-rename"})
	if got := c.Snapshot(); got.RecentFiles[0].Path != before.Path || !strings.Contains(got.Error, "unknown") {
		t.Fatalf("unknown mutation accepted: %+v", got)
	}
	if _, err := os.Stat(before.Path); err != nil {
		t.Fatal("unknown mutation moved source", err)
	}
}

func TestWarningDismissalKeepsError(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	c.ReportError(errors.New("error remains"))
	c.ReportWarning("warning")
	c.DismissWarning()
	if s := c.Snapshot(); s.Warning != "" || s.Error != "error remains" {
		t.Fatalf("dismiss warning: %+v", s)
	}
	c.ReportWarning("warning remains")
	c.DismissError()
	if s := c.Snapshot(); s.Error != "" || s.Warning != "warning remains" {
		t.Fatalf("dismiss error: %+v", s)
	}
}

func TestConfigSaveFailureThenRepairClearsError(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	path := filepath.Join(c.dataDir, "config.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := c.Snapshot().Config
	cfg.Appearance = "dark"
	c.SaveConfig(cfg)
	eventually(t, "config save failure", func() bool { return strings.Contains(c.Snapshot().StorageError, "config.json") })
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	c.SaveConfig(cfg)
	eventually(t, "config error cleared", func() bool { return c.Snapshot().StorageError == "" })
	persisted, err := core.LoadConfig(path, "")
	if err != nil || persisted.Appearance != "dark" {
		t.Fatalf("config repair did not persist: %+v %v", persisted, err)
	}
}

func TestStopDuringStartingWins(t *testing.T) {
	c, _ := testController(t, successHandler, Effects{})
	var stopped atomic.Bool
	reconciled := make(chan struct{}, 1)
	c.SetOnChange(func(s Snapshot) {
		if s.WatcherStatus == "Starting" && stopped.CompareAndSwap(false, true) {
			c.Stop()
		}
		if stopped.Load() && s.WatcherStatus == "Paused" {
			select {
			case reconciled <- struct{}{}:
			default:
			}
		}
	})
	c.Start()
	select {
	case <-reconciled:
	case <-time.After(3 * time.Second):
		t.Fatal("stop during starting was lost")
	}
	if c.Snapshot().Watching {
		t.Fatal("watcher survived Stop")
	}
}
