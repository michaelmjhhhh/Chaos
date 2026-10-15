// Package desktop renders Chaos with MyGo's native Go UI. Mutable view state
// belongs to the UI thread. Controller methods enqueue work; they never block a frame.
package desktop

import (
	"context"
	"fmt"
	"image"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

type Controller interface {
	Snapshot() app.Snapshot
	Start()
	Stop()
	ProcessFiles([]string)
	Retry(string)
	RetryAll()
	Rename(string, string)
	Revert(string)
	CheckHealth()
	CompleteOnboarding(bool)
	SaveConfig(core.Config)
	DismissError()
	ReplayOnboarding()
	SetAutoStart(bool)
}

// Actions supplies the native shell. Blocking actions such as file dialogs must
// start their own worker. Dispatch schedules thumbnail results on the UI thread.
type Actions struct {
	PickImages          func()
	PickWatchDirectory  func()
	PickOutputDirectory func()
	OpenPath            func(string)
	RevealPath          func(string)
	OpenURL             func(string)
	Quit                func()
	ShowWindow          func()
	Dispatch            func(func())
	LoadThumbnail       func(context.Context, string) (image.Image, error)
}

type Desktop struct {
	activeSince          time.Time
	controller           Controller
	actions              Actions
	snapshot             app.Snapshot
	config               core.Config
	home                 string
	tab                  string
	query, filter        string
	search               ui.Handle
	files                ui.ListState
	selected             int
	renameOpen           bool
	renameID, renameName string
	helpQuery            string
	expanded             map[int]bool
	welcome              bool
	welcomeStep          int
	advanced             bool
	announcement         string
	announced            string
	thumbnails           map[string]*thumbnailEntry
	thumbnailUse         uint64
	thumbnailLoader      *thumbnailLoader
	now                  func() time.Time
}

func New(controller Controller, actions Actions) *Desktop {
	home, _ := os.UserHomeDir()
	d := &Desktop{
		controller: controller, actions: actions, home: home,
		tab: "Dashboard", filter: "All", selected: -1,
		expanded: make(map[int]bool), thumbnails: make(map[string]*thumbnailEntry), now: time.Now,
	}
	if controller != nil {
		d.snapshot = controller.Snapshot()
		d.config = d.snapshot.Config
		d.welcome = !d.snapshot.OnboardingComplete
		d.advanced = core.ProviderByID(d.config.Provider).CustomURL
	}
	d.startThumbnailLoader()
	return d
}

// Close cancels thumbnail work and waits for its workers. Call it off the UI
// thread; queued UI publications become no-ops after cancellation.
func (d *Desktop) Close() {
	if loader := d.thumbnailLoader; loader != nil {
		loader.cancel()
		loader.workers.Wait()
	}
}

// SetSnapshot is called by the shell inside Window.Update, never a worker.
func (d *Desktop) SetSnapshot(s app.Snapshot) {
	if s.Revision < d.snapshot.Revision {
		return
	}
	if s.Busy && (!d.snapshot.Busy || (s.CurrentPath != "" && s.CurrentPath != d.snapshot.CurrentPath && s.CurrentStage == app.StageAnalyzing)) {
		d.activeSince = d.now()
		// A later intake can reuse a source path that was already previewed.
		delete(d.thumbnails, s.CurrentPath)
	}
	if !reflect.DeepEqual(s.Config, d.snapshot.Config) {
		if s.Config.Provider != d.snapshot.Config.Provider {
			d.advanced = core.ProviderByID(s.Config.Provider).CustomURL
		}
		d.config = s.Config
	}
	if s.OnboardingComplete {
		d.welcome = false
	} else if d.snapshot.OnboardingComplete {
		d.welcome = true
		d.welcomeStep = 0
	}
	if s.CurrentStage != d.snapshot.CurrentStage || s.CurrentFile != d.snapshot.CurrentFile {
		if s.CurrentStage == app.StageSuccess {
			d.announcement = "Filed as " + s.CurrentFile
		}
		if s.CurrentStage == app.StageError {
			d.announcement = "Couldn't file " + s.CurrentFile + ". " + s.Error
		}
	}
	if s.APIStatus != d.snapshot.APIStatus && (s.APIStatus == "OK" || s.APIStatus == "FAIL") {
		d.announcement = "Connection " + s.APIStatus
	}
	d.snapshot = s
	d.requestThumbnail(s.CurrentPath)
	for i, f := range s.RecentFiles {
		if i >= 16 {
			break
		}
		d.requestThumbnail(f.Path)
	}
}

func (d *Desktop) SelectTab(tab string) {
	switch tab {
	case "Dashboard", "Pipeline", "Insights", "Settings", "Help":
		d.tab = tab
	}
}
func (d *Desktop) ShowHelp() { d.tab = "Help" }
func (d *Desktop) ShowWelcome() {
	d.welcome = true
	d.welcomeStep = 0
	if d.controller != nil {
		d.controller.ReplayOnboarding()
	}
}
func (d *Desktop) View(c *ui.Context) {
	d.theme(c)
	if d.announcement != "" && d.announcement != d.announced {
		c.Announce(d.announcement)
		d.announced = d.announcement
	}
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { d.tab = "Settings" })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyT, func() {
		if d.controller != nil {
			d.controller.CheckHealth()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if d.tab == "Pipeline" {
			d.search.Focus()
		}
	})
	// Text controls and focused buttons consume Space first in MyGo's input routing.
	if d.tab == "Dashboard" {
		c.OnShortcut(0, ui.KeySpace, d.toggleWatching)
	}
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		d.navigation(c)
		switch d.tab {
		case "Pipeline":
			d.pipeline(c)
		case "Insights":
			d.insights(c)
		case "Settings":
			d.settings(c)
		case "Help":
			d.help(c)
		default:
			d.dashboard(c)
		}
		d.footer(c)
	})
	d.renameDialog(c)
	d.welcomeDialog(c)
}

func (d *Desktop) navigation(c *ui.Context) {
	ui.Row(c).Padding(12, 20).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border).Children(func() {
		ui.Text(c, "Chaos").Font("Georgia").FontSize(22).Margin(0, 14, 0, 0)
		ui.Row(c).Gap(4).Grow(1).FocusGroup(ui.Horizontal).Children(func() {
			for _, name := range []string{"Dashboard", "Pipeline", "Insights"} {
				d.tabButton(c, name)
			}
		})
		d.tabButton(c, "Settings")
		ui.Button(c.Key("help-nav"), "?").Label("Help & Getting Started").OnClick(d.ShowHelp)
	})
}
func (d *Desktop) tabButton(c *ui.Context, name string) {
	b := ui.Button(c.Key("tab-"+name), name).Padding(7, 10).OnClick(func() { d.tab = name })
	if d.tab == name {
		b.Background(c.Theme().Accent.Alpha(.13)).TextColor(c.Theme().Accent)
	}
}
func (d *Desktop) footer(c *ui.Context) {
	ui.Row(c).Padding(9, 20).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Children(func() {
		color := c.Theme().TextMuted
		if d.snapshot.APIStatus == "OK" {
			color = c.Theme().Success
		}
		if d.snapshot.APIStatus == "FAIL" {
			color = c.Theme().Danger
		}
		ui.Text(c, "●").TextColor(color).Label("API status " + d.snapshot.APIStatus)
		ui.Text(c, providerLabel(d.config)).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1).SingleLine()
		ui.Text(c, d.status()).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}
func (d *Desktop) status() string {
	if d.snapshot.WatcherStatus != "" {
		return d.snapshot.WatcherStatus
	}
	if d.snapshot.Watching {
		return "Watching"
	}
	return "Paused"
}
func (d *Desktop) toggleWatching() {
	if d.controller == nil {
		return
	}
	if d.snapshot.Watching {
		d.controller.Stop()
	} else {
		d.controller.Start()
	}
}
func (d *Desktop) save() {
	if d.controller != nil {
		d.controller.SaveConfig(d.config)
	}
}
func (d *Desktop) watchDir() string {
	if d.snapshot.Watching && d.snapshot.ActiveWatchDir != "" {
		return d.snapshot.ActiveWatchDir
	}
	return d.snapshot.ResolvedWatchDir
}
func (d *Desktop) open(path string) {
	if path != "" && d.actions.OpenPath != nil {
		d.actions.OpenPath(path)
	}
}
func (d *Desktop) reveal(path string) {
	if path != "" && d.actions.RevealPath != nil {
		d.actions.RevealPath(path)
	}
}
func (d *Desktop) link(url string) {
	if d.actions.OpenURL != nil {
		d.actions.OpenURL(url)
	}
}
func duration(v float64) string {
	if v <= 0 {
		return "—"
	}
	if v < 1 {
		return fmt.Sprintf("%.0fms", v*1000)
	}
	return fmt.Sprintf("%.1fs", v)
}
func (d *Desktop) shorten(path string) string {
	if d.home != "" && (path == d.home || strings.HasPrefix(path, d.home+string(os.PathSeparator))) {
		return "~" + strings.TrimPrefix(path, d.home)
	}
	return path
}
func fileName(f core.RecentFile) string {
	if f.NewName != "" {
		return f.NewName
	}
	return f.OriginalName
}

func providerLabel(cfg core.Config) string {
	p := core.ProviderByID(cfg.Provider)
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = p.Model
	}
	return p.Name + " · " + model
}
