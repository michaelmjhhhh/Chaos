package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

// Effects is the narrow boundary between background Go services and the desktop.
// Effect failures are warnings: they must never turn a durable filing into a failure.
type Effects struct {
	CopyImage func(string) error
	Notify    func(title, message string) error
	Sound     func() error
}

type Options struct {
	DataDir, Home string
	Processor     *core.Processor
	Client        *core.Client
	Effects       Effects
	Now           func() time.Time
	WatchInterval time.Duration
	// Tests and explicit --data-dir sessions never read real user preferences.
	ImportLegacyPreferences bool
}

type job struct {
	kind, path, id, name   string
	watch                  bool
	batchIndex, batchTotal int
}

type Controller struct {
	mu                               sync.Mutex
	snapshot                         Snapshot
	onChange                         func(Snapshot)
	ctx                              context.Context
	cancel                           context.CancelFunc
	wg                               sync.WaitGroup
	closeOnce                        sync.Once
	closed                           chan struct{}
	wake, saveWake, watchWake        chan struct{}
	queue                            []job
	pending                          map[string]bool
	activeCancel                     context.CancelFunc
	activeWatch                      bool
	watchMu                          sync.Mutex
	watcher                          *core.Watcher
	watchCancel                      context.CancelFunc
	store                            *core.Store
	processor                        *core.Processor
	client                           *core.Client
	effects                          Effects
	home, dataDir                    string
	now                              func() time.Time
	watchInterval                    time.Duration
	wantWatching                     bool
	configEdited                     bool
	settingsRevision, healthRevision uint64
	closeErr                         error
	prefs                            Preferences
	refreshMu                        sync.Mutex
}

func New(opts Options) (*Controller, error) {
	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}
	if opts.DataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		opts.DataDir = filepath.Join(base, "chaos")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Client == nil {
		opts.Client = &core.Client{}
	}
	if opts.Processor == nil {
		opts.Processor = &core.Processor{Client: opts.Client, Now: opts.Now}
	}
	if err := os.MkdirAll(opts.DataDir, 0700); err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(opts.DataDir, "config.json")
	legacy := ""
	if opts.ImportLegacyPreferences {
		legacy = filepath.Join(filepath.Dir(opts.DataDir), "vibe-shot", "config.json")
	}
	cfg, cfgErr := core.LoadConfig(cfgPath, legacy)
	var importPrefs func() Preferences
	if opts.ImportLegacyPreferences {
		importPrefs = func() Preferences { return legacyPreferences(runCommand) }
	}
	prefs, prefErr := loadPreferences(filepath.Join(opts.DataDir, "preferences.json"), importPrefs)
	prefs.SessionNumber++
	if prefErr == nil {
		prefErr = savePreferences(filepath.Join(opts.DataDir, "preferences.json"), prefs)
	}
	db, err := core.OpenStore(filepath.Join(opts.DataDir, "history.sqlite"), filepath.Join(opts.DataDir, "history.json"))
	if err != nil {
		return nil, fmt.Errorf("open screenshot history: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Controller{ctx: ctx, cancel: cancel, closed: make(chan struct{}), wake: make(chan struct{}, 1), saveWake: make(chan struct{}, 1), pending: make(map[string]bool), store: db, processor: opts.Processor, client: opts.Client, effects: opts.Effects, home: opts.Home, dataDir: opts.DataDir, now: opts.Now, watchInterval: opts.WatchInterval, prefs: prefs}
	r := cfg.Resolve(opts.Home)
	c.snapshot = Snapshot{Config: cfg, APIStatus: "N/A", WatcherStatus: "Paused", StartedAt: opts.Now(), SessionNumber: prefs.SessionNumber, AutoStart: prefs.AutoStart, OnboardingComplete: prefs.OnboardingComplete, DataDir: opts.DataDir, ConfigPath: cfgPath, ResolvedWatchDir: r.WatchDir, ResolvedOutputDir: r.OutputDir}
	if err := errors.Join(cfgErr, prefErr); err != nil {
		c.snapshot.StorageError = err.Error()
	}
	if err := c.recoverOperations(); err != nil {
		c.snapshot.StorageError = err.Error()
	}
	c.refresh()
	c.watchWake = make(chan struct{}, 1)
	c.wg.Add(3)
	go c.worker()
	go c.settingsWorker()
	go c.watchWorker()
	return c, nil
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cloneSnapshot(c.snapshot)
}

func (c *Controller) SetOnChange(fn func(Snapshot)) {
	c.mu.Lock()
	c.onChange = fn
	s := cloneSnapshot(c.snapshot)
	c.mu.Unlock()
	if fn != nil {
		fn(s)
	}
}

func (c *Controller) change(fn func(*Snapshot)) {
	c.mu.Lock()
	fn(&c.snapshot)
	c.snapshot.Revision++
	s := cloneSnapshot(c.snapshot)
	cb := c.onChange
	c.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

func (c *Controller) report(err error) {
	if err == nil {
		return
	}
	c.change(func(s *Snapshot) { s.Error = err.Error(); s.RecoveryAction = "none" })
}

func (c *Controller) ReportError(err error) { c.report(err) }
func (c *Controller) ReportWarning(message string) {
	c.change(func(s *Snapshot) { s.Warning = message })
}
func (c *Controller) DismissError() {
	c.change(func(s *Snapshot) { s.Error = ""; s.Warning = ""; s.RecoveryAction = "" })
}

func (c *Controller) SaveConfig(cfg core.Config) {
	if c.ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	old := c.snapshot.Config.Resolve(c.home)
	c.snapshot.Config = cloneConfig(cfg)
	c.configEdited = true
	r := cfg.Resolve(c.home)
	c.snapshot.ResolvedWatchDir = r.WatchDir
	c.snapshot.ResolvedOutputDir = r.OutputDir
	c.settingsRevision++
	if old.Provider.ID != r.Provider.ID || old.APIKey != r.APIKey || old.Model != r.Model || old.BaseURL != r.BaseURL {
		c.snapshot.APIStatus = "N/A"
		c.snapshot.HealthError = ""
		c.healthRevision++
	}
	changedWatch := c.snapshot.Watching && old.WatchDir != r.WatchDir
	c.mu.Unlock()
	c.signal(c.saveWake)
	c.change(func(*Snapshot) {})
	if changedWatch {
		c.signal(c.watchWake)
	}
}

func (c *Controller) SetAutoStart(enabled bool) {
	c.mu.Lock()
	c.prefs.AutoStart = enabled
	c.snapshot.AutoStart = enabled
	c.settingsRevision++
	c.mu.Unlock()
	c.signal(c.saveWake)
	c.change(func(*Snapshot) {})
}

func (c *Controller) CompleteOnboarding(start bool) {
	c.mu.Lock()
	c.prefs.OnboardingComplete = true
	c.snapshot.OnboardingComplete = true
	c.settingsRevision++
	c.mu.Unlock()
	c.signal(c.saveWake)
	c.change(func(*Snapshot) {})
	if start {
		c.Start()
	}
}

func (c *Controller) ReplayOnboarding() {
	c.mu.Lock()
	c.prefs.OnboardingComplete = false
	c.snapshot.OnboardingComplete = false
	c.settingsRevision++
	c.mu.Unlock()
	c.signal(c.saveWake)
	c.change(func(*Snapshot) {})
}

func (c *Controller) signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Controller) settingsWorker() {
	defer c.wg.Done()
	for {
		select {
		case <-c.saveWake:
			c.saveSettings()
		case <-c.ctx.Done():
			c.saveSettings()
			return
		}
	}
}

func (c *Controller) saveSettings() {
	c.mu.Lock()
	cfg := cloneConfig(c.snapshot.Config)
	p := c.prefs
	revision := c.settingsRevision
	configEdited := c.configEdited
	c.mu.Unlock()
	// Only explicit edits save provider configuration. Opening a corrupt file must
	// never replace it with defaults during shutdown.
	if revision == 0 {
		return
	}
	var configErr error
	if configEdited {
		configErr = core.SaveConfig(filepath.Join(c.dataDir, "config.json"), cfg)
	}
	err := errors.Join(configErr, savePreferences(filepath.Join(c.dataDir, "preferences.json"), p))
	c.change(func(s *Snapshot) {
		if err != nil {
			s.StorageError = "Save settings: " + err.Error()
		}
	})
}

func (c *Controller) Start() {
	c.mu.Lock()
	c.wantWatching = true
	c.mu.Unlock()
	if c.ctx.Err() == nil {
		c.signal(c.watchWake)
	}
}
func (c *Controller) Stop() {
	c.mu.Lock()
	c.wantWatching = false
	c.mu.Unlock()
	c.cancelWatchedJobs()
	c.signal(c.watchWake)
}

func (c *Controller) watchWorker() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			c.watchMu.Lock()
			c.stopWatcherLocked()
			c.watchMu.Unlock()
			c.cancelWatchedJobs()
			return
		case <-c.watchWake:
			c.startWatcher()
		}
	}
}

// Reconcile the latest requested state in a single worker. Old asynchronous
// Stop calls cannot tear down a newer Start or a settings-triggered rebind.
func (c *Controller) startWatcher() {
	c.watchMu.Lock()
	defer c.watchMu.Unlock()
	if c.ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	r := c.snapshot.Config.Resolve(c.home)
	wanted := c.wantWatching
	activeDir := c.snapshot.ActiveWatchDir
	c.mu.Unlock()
	if !wanted {
		c.stopWatcherLocked()
		return
	}
	if c.watcher != nil && activeDir == r.WatchDir {
		return
	}
	if c.watcher != nil {
		c.stopWatcherLocked()
		c.cancelWatchedJobs()
	}
	if err := r.Validate(); err != nil {
		c.change(func(s *Snapshot) {
			s.WatcherStatus = "Needs attention"
			s.Error = err.Error()
			s.RecoveryAction = "settings"
		})
		return
	}
	c.change(func(s *Snapshot) { s.WatcherStatus = "Starting" })
	ctx, cancel := context.WithCancel(c.ctx)
	w := &core.Watcher{Directory: r.WatchDir, Interval: c.watchInterval}
	if err := w.Start(ctx, func(path string) { c.enqueue([]string{path}, true) }, func(err error) { c.report(fmt.Errorf("watch folder: %w", err)) }); err != nil {
		cancel()
		c.change(func(s *Snapshot) {
			s.WatcherStatus = "Needs attention"
			s.Error = err.Error()
			s.RecoveryAction = "settings"
		})
		return
	}
	c.watcher = w
	c.watchCancel = cancel
	c.change(func(s *Snapshot) {
		s.Watching = true
		s.WatcherStatus = "Watching"
		s.ActiveWatchDir = r.WatchDir
		s.WatcherStartedAt = c.now()
	})
	c.CheckHealth()
}

func (c *Controller) stopWatcherLocked() {
	if c.watchCancel != nil {
		c.watchCancel()
		c.watchCancel = nil
	}
	if c.watcher != nil {
		c.watcher.Stop()
		c.watcher = nil
	}
	c.change(func(s *Snapshot) {
		s.Watching = false
		s.WatcherStatus = "Paused"
		s.ActiveWatchDir = ""
		s.WatcherStartedAt = time.Time{}
		s.QueueLength = len(c.queue)
	})
}

func (c *Controller) cancelWatchedJobs() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activeWatch && c.activeCancel != nil {
		c.activeCancel()
	}
	filtered := c.queue[:0]
	for _, j := range c.queue {
		if j.watch {
			delete(c.pending, j.path)
		} else {
			filtered = append(filtered, j)
		}
	}
	c.queue = filtered
}

func canonicalInput(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("no image was selected")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("select a regular image file")
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".heic":
	default:
		return "", errors.New("choose PNG, JPEG, HEIC, or WebP images")
	}
	return path, nil
}

func (c *Controller) ProcessFiles(paths []string) {
	copyPaths := append([]string(nil), paths...)
	c.run(func() { c.enqueue(copyPaths, false) })
}

func (c *Controller) enqueue(paths []string, watch bool) {
	var accepted []string
	var rejected []string
	for _, p := range paths {
		path, err := canonicalInput(p)
		if err != nil {
			if !watch {
				rejected = append(rejected, filepath.Base(p)+": "+err.Error())
			}
			continue
		}
		accepted = append(accepted, path)
	}
	c.mu.Lock()
	if c.ctx.Err() != nil || (watch && !c.wantWatching) {
		c.mu.Unlock()
		return
	}
	var added []job
	for _, p := range accepted {
		if !c.pending[p] {
			c.pending[p] = true
			added = append(added, job{kind: "process", path: p, watch: watch})
		}
	}
	for i := range added {
		added[i].batchIndex = i + 1
		added[i].batchTotal = len(added)
	}
	c.queue = append(c.queue, added...)
	c.snapshot.QueueLength = len(c.queue)
	if len(rejected) > 0 {
		c.snapshot.Warning = strings.Join(rejected, "\n")
	}
	c.mu.Unlock()
	c.change(func(*Snapshot) {})
	c.signal(c.wake)
}

func (c *Controller) findRecord(id string) (core.RecentFile, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.snapshot.RecentFiles {
		if r.ID == id {
			return r, true
		}
	}
	return core.RecentFile{}, false
}

func (c *Controller) Retry(id string) {
	if r, ok := c.findRecord(id); ok {
		if r.IsError {
			c.ProcessFiles([]string{r.SourcePath})
		}
	}
}

func (c *Controller) RetryAll() {
	s := c.Snapshot()
	paths := make([]string, 0)
	for _, r := range s.RecentFiles {
		if r.IsError && r.SourcePath != "" {
			if info, err := os.Stat(r.SourcePath); err == nil && info.Mode().IsRegular() {
				paths = append(paths, r.SourcePath)
			}
		}
	}
	c.ProcessFiles(paths)
}

func (c *Controller) Rename(id, name string) {
	c.enqueueMutation(job{kind: "rename", id: id, name: name})
}
func (c *Controller) Revert(id string) { c.enqueueMutation(job{kind: "revert", id: id}) }
func (c *Controller) enqueueMutation(j job) {
	c.mu.Lock()
	if c.ctx.Err() == nil {
		c.queue = append(c.queue, j)
	}
	c.mu.Unlock()
	c.signal(c.wake)
}

func (c *Controller) worker() {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	day := c.now().Format("2006-01-02 MST -0700")
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if current := c.now().Format("2006-01-02 MST -0700"); current != day {
				day = current
				c.refresh()
			}
		case <-c.wake:
		}
		for {
			c.mu.Lock()
			if c.ctx.Err() != nil || len(c.queue) == 0 {
				c.mu.Unlock()
				break
			}
			j := c.queue[0]
			c.queue = c.queue[1:]
			ctx, cancel := context.WithCancel(c.ctx)
			c.activeCancel = cancel
			c.activeWatch = j.watch
			c.snapshot.QueueLength = len(c.queue)
			c.mu.Unlock()
			if j.kind == "process" {
				c.process(ctx, j)
			} else {
				c.mutate(ctx, j)
			}
			cancel()
			c.mu.Lock()
			delete(c.pending, j.path)
			c.activeCancel = nil
			c.activeWatch = false
			c.mu.Unlock()
			c.change(func(s *Snapshot) { s.Busy = false; s.BatchIndex = 0; s.BatchTotal = 0; s.QueueLength = len(c.queue) })
		}
	}
}

func (c *Controller) run(fn func()) {
	c.mu.Lock()
	if c.ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	c.wg.Add(1)
	c.mu.Unlock()
	go func() { defer c.wg.Done(); fn() }()
}

func (c *Controller) CheckHealth() { c.run(c.checkHealth) }

func (c *Controller) checkHealth() {
	c.mu.Lock()
	cfg := c.snapshot.Config.Resolve(c.home)
	c.healthRevision++
	rev := c.healthRevision
	c.mu.Unlock()
	c.change(func(s *Snapshot) {
		if rev != c.healthRevision {
			return
		}
		s.APIStatus = "Checking…"
		s.HealthError = ""
	})
	ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	err := cfg.Validate()
	if err == nil {
		err = c.client.CheckHealth(ctx, cfg)
	}
	c.change(func(s *Snapshot) {
		if rev != c.healthRevision {
			return
		}
		if err == nil {
			s.APIStatus = "OK"
			s.HealthError = ""
		} else {
			s.APIStatus = "FAIL"
			s.HealthError = core.Friendly(err, cfg.Provider.ID).Message
		}
	})
}

func (c *Controller) refresh() {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	recent, err := c.store.Recent(ctx, 500)
	if err != nil {
		c.change(func(s *Snapshot) { s.StorageError = "Read history: " + err.Error() })
		return
	}
	insights, err := c.store.Insights(ctx, c.now(), time.Local)
	if err != nil {
		c.change(func(s *Snapshot) { s.StorageError = "Read insights: " + err.Error() })
		return
	}
	c.change(func(s *Snapshot) {
		s.RecentFiles = recent
		s.Insights = insights
		s.HistoryRevision++
		s.Processed = insights.TodayProcessed
		s.Successes = insights.TodaySuccesses
		s.Errors = insights.TodayErrors
		s.AvgLatency = insights.TodayAvgDuration
		s.P95Latency = insights.TodayP95Duration
	})
}

// Close cancels network work, drains durable writes, and then closes SQLite.
// Call off the UI thread because native effects can be awaiting its event loop.
func (c *Controller) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.cancel()
		c.mu.Unlock()
		c.wg.Wait()
		c.closeErr = c.store.Close()
		close(c.closed)
	})
	<-c.closed
	return c.closeErr
}
