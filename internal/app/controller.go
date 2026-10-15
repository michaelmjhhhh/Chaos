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

type jobKind uint8

const (
	jobProcess jobKind = iota
	jobRename
	jobRevert
)

type job struct {
	kind                   jobKind
	path, id, name         string
	watch                  bool
	batchIndex, batchTotal int
}

type Controller struct {
	mu        sync.Mutex
	snapshot  Snapshot
	onChange  func(Snapshot)
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error

	wake, saveWake, watchWake chan struct{}
	queue                     []job
	pending                   map[string]bool
	activeCancel              context.CancelFunc
	activeWatch               bool

	watchMu       sync.Mutex
	watcher       *core.Watcher
	watchCancel   context.CancelFunc
	watchInterval time.Duration
	wantWatching  bool

	store         *core.Store
	processor     *core.Processor
	client        *core.Client
	effects       Effects
	home, dataDir string
	now           func() time.Time
	refreshMu     sync.Mutex

	configRevision, savedConfigRevision           uint64
	preferencesRevision, savedPreferencesRevision uint64
	healthRevision                                uint64
	storageErrors                                 storageErrors
	prefs                                         Preferences
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
	c := &Controller{
		ctx:           ctx,
		cancel:        cancel,
		closed:        make(chan struct{}),
		wake:          make(chan struct{}, 1),
		saveWake:      make(chan struct{}, 1),
		watchWake:     make(chan struct{}, 1),
		pending:       make(map[string]bool),
		store:         db,
		processor:     opts.Processor,
		client:        opts.Client,
		effects:       opts.Effects,
		home:          opts.Home,
		dataDir:       opts.DataDir,
		now:           opts.Now,
		watchInterval: opts.WatchInterval,
		prefs:         prefs,
	}
	r := cfg.Resolve(opts.Home)
	c.snapshot = Snapshot{
		Config:             cfg,
		APIStatus:          "N/A",
		WatcherStatus:      "Paused",
		AutoStart:          prefs.AutoStart,
		OnboardingComplete: prefs.OnboardingComplete,
		ConfigPath:         cfgPath,
		ResolvedWatchDir:   r.WatchDir,
		ResolvedOutputDir:  r.OutputDir,
	}
	if cfgErr != nil {
		c.storageErrors.config = cfgErr.Error()
	}
	if prefErr != nil {
		c.storageErrors.preferences = prefErr.Error()
	}
	if err := c.recoverOperations(); err != nil {
		c.storageErrors.recovery = err.Error()
	}
	c.snapshot.StorageError = c.storageErrors.message()
	c.refresh()
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

// change publishes only when the mutation changes visible state. Internal revision
// acknowledgements may still happen when the callback returns false.
func (c *Controller) change(fn func(*Snapshot) bool) bool {
	c.mu.Lock()
	if !fn(&c.snapshot) {
		c.mu.Unlock()
		return false
	}
	c.snapshot.StorageError = c.storageErrors.message()
	c.snapshot.Revision++
	cb := c.onChange
	var snapshot Snapshot
	if cb != nil {
		snapshot = cloneSnapshot(c.snapshot)
	}
	c.mu.Unlock()
	if cb != nil {
		cb(snapshot)
	}
	return true
}

func (c *Controller) ReportError(err error) {
	if err == nil {
		return
	}
	c.change(func(s *Snapshot) bool {
		message := err.Error()
		if s.Error == message && s.RecoveryAction == "" {
			return false
		}
		s.Error, s.RecoveryAction = message, ""
		return true
	})
}

func (c *Controller) ReportWarning(message string) {
	c.change(func(s *Snapshot) bool {
		if s.Warning == message {
			return false
		}
		s.Warning = message
		return true
	})
}

func (c *Controller) DismissError() {
	c.change(func(s *Snapshot) bool {
		if s.Error == "" && s.Warning == "" && s.RecoveryAction == "" {
			return false
		}
		s.Error, s.Warning, s.RecoveryAction = "", "", ""
		return true
	})
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Controller) Start() {
	c.mu.Lock()
	c.wantWatching = true
	c.mu.Unlock()
	if c.ctx.Err() == nil {
		signal(c.watchWake)
	}
}
func (c *Controller) Stop() {
	c.mu.Lock()
	c.wantWatching = false
	c.mu.Unlock()
	c.cancelWatchedJobs()
	signal(c.watchWake)
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
		c.change(func(s *Snapshot) bool {
			s.WatcherStatus = "Needs attention"
			s.Error = err.Error()
			s.RecoveryAction = core.RecoverySettings
			return true
		})
		return
	}
	c.change(func(s *Snapshot) bool {
		s.WatcherStatus = "Starting"
		return true
	})
	ctx, cancel := context.WithCancel(c.ctx)
	w := &core.Watcher{Directory: r.WatchDir, Interval: c.watchInterval}
	if err := w.Start(ctx, func(path string) { c.enqueue([]string{path}, true) }, func(err error) { c.ReportError(fmt.Errorf("watch folder: %w", err)) }); err != nil {
		cancel()
		c.change(func(s *Snapshot) bool {
			s.WatcherStatus = "Needs attention"
			s.Error = err.Error()
			s.RecoveryAction = core.RecoverySettings
			return true
		})
		return
	}
	c.watcher = w
	c.watchCancel = cancel
	c.change(func(s *Snapshot) bool {
		s.Watching = true
		s.WatcherStatus = "Watching"
		s.ActiveWatchDir = r.WatchDir
		return true
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
	c.change(func(s *Snapshot) bool {
		if !s.Watching && s.WatcherStatus == "Paused" && s.ActiveWatchDir == "" && s.QueueLength == len(c.queue) {
			return false
		}
		s.Watching = false
		s.WatcherStatus = "Paused"
		s.ActiveWatchDir = ""
		s.QueueLength = len(c.queue)
		return true
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
	if !core.AcceptsImage(path) {
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
	changed := c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil || (watch && !c.wantWatching) {
			return false
		}
		var added []job
		for _, p := range accepted {
			if !c.pending[p] {
				c.pending[p] = true
				added = append(added, job{kind: jobProcess, path: p, watch: watch})
			}
		}
		for i := range added {
			added[i].batchIndex, added[i].batchTotal = i+1, len(added)
		}
		c.queue = append(c.queue, added...)
		queueChanged := s.QueueLength != len(c.queue)
		s.QueueLength = len(c.queue)
		warningChanged := false
		if len(rejected) > 0 {
			warning := strings.Join(rejected, "\n")
			warningChanged = warning != s.Warning
			s.Warning = warning
		}
		return len(added) > 0 || queueChanged || warningChanged
	})
	if changed {
		signal(c.wake)
	}
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
	c.run(func() {
		snapshot := c.Snapshot()
		var paths []string
		for _, record := range snapshot.RecentFiles {
			if c.ctx.Err() != nil {
				return
			}
			if record.IsError && record.SourcePath != "" {
				if info, err := os.Stat(record.SourcePath); err == nil && info.Mode().IsRegular() {
					paths = append(paths, record.SourcePath)
				}
			}
		}
		c.enqueue(paths, false)
	})
}

func (c *Controller) Rename(id, name string) {
	c.enqueueMutation(job{kind: jobRename, id: id, name: name})
}
func (c *Controller) Revert(id string) { c.enqueueMutation(job{kind: jobRevert, id: id}) }
func (c *Controller) enqueueMutation(j job) {
	if c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil {
			return false
		}
		c.queue = append(c.queue, j)
		s.QueueLength = len(c.queue)
		return true
	}) {
		signal(c.wake)
	}
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
			switch j.kind {
			case jobProcess:
				c.process(ctx, j)
			case jobRename, jobRevert:
				c.mutate(ctx, j)
			default:
				c.ReportError(fmt.Errorf("unknown queued operation: %d", j.kind))
			}
			cancel()
			c.change(func(s *Snapshot) bool {
				delete(c.pending, j.path)
				c.activeCancel, c.activeWatch = nil, false
				s.Busy, s.BatchIndex, s.BatchTotal, s.QueueLength = false, 0, 0, len(c.queue)
				return true
			})
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
	go func() {
		defer c.wg.Done()
		fn()
	}()
}

func (c *Controller) CheckHealth() { c.run(c.checkHealth) }

func (c *Controller) checkHealth() {
	var cfg core.ResolvedConfig
	var revision uint64
	if !c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil {
			return false
		}
		cfg = s.Config.Resolve(c.home)
		c.healthRevision++
		revision = c.healthRevision
		s.APIStatus, s.HealthError = "Checking…", ""
		return true
	}) {
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	err := cfg.Validate()
	if err == nil {
		err = c.client.CheckHealth(ctx, cfg)
	}
	c.change(func(s *Snapshot) bool {
		if revision != c.healthRevision || c.ctx.Err() != nil {
			return false
		}
		if err == nil {
			s.APIStatus, s.HealthError = "OK", ""
		} else {
			s.APIStatus, s.HealthError = "FAIL", core.Friendly(err, cfg.Provider.ID).Message
		}
		return true
	})
}

func (c *Controller) refresh() {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	recent, err := c.store.Recent(ctx, 500)
	if err != nil {
		c.change(func(*Snapshot) bool {
			message := "Read history: " + err.Error()
			changed := c.storageErrors.historyRead != message
			c.storageErrors.historyRead = message
			return changed
		})
		return
	}
	insights, err := c.store.Insights(ctx, c.now(), time.Local)
	if err != nil {
		c.change(func(*Snapshot) bool {
			message := "Read insights: " + err.Error()
			changed := c.storageErrors.historyRead != message
			c.storageErrors.historyRead = message
			return changed
		})
		return
	}
	c.change(func(s *Snapshot) bool {
		s.RecentFiles = recent
		s.Insights = insights
		c.storageErrors.historyRead = ""
		s.Successes = insights.TodaySuccesses
		s.Errors = insights.TodayErrors
		s.AvgLatency = insights.TodayAvgDuration
		return true
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
