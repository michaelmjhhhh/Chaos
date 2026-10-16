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
	pal                  palette
	home                 string
	tab                  string
	query, filter        string
	search, helpSearch   ui.Handle
	files                ui.ListState
	scroll               map[string]*ui.ScrollState
	selected             int
	renameOpen           bool
	renameID, renameName string
	renameExt            string
	helpQuery            string
	expanded             map[int]*bool
	welcome              bool
	welcomeStep          int
	advanced             bool
	announcement         string
	announced            string
	toast                *ui.Toast
	toastAt              time.Time
	railShown            bool
	hoverDay             string
	thumbnails           map[string]*thumbnailEntry
	thumbnailUse         uint64
	thumbnailLoader      *thumbnailLoader
	now                  func() time.Time
}

func New(controller Controller, actions Actions) *Desktop {
	registerFonts()
	home, _ := os.UserHomeDir()
	d := &Desktop{
		controller: controller, actions: actions, home: home,
		tab: "Dashboard", filter: "All", selected: -1,
		expanded: make(map[int]*bool), scroll: make(map[string]*ui.ScrollState),
		thumbnails: make(map[string]*thumbnailEntry), now: time.Now,
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
			d.toast, d.toastAt = d.filedToast(s), d.now()
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

// filedToast confirms a filing in place, with Undo while the original
// location is known. One toast is reused, so a batch updates it rather
// than stacking a message per image.
func (d *Desktop) filedToast(s app.Snapshot) *ui.Toast {
	t := &ui.Toast{ID: "filed", Title: "Filed as " + s.CurrentFile, Type: "success"}
	for _, f := range s.RecentFiles {
		if f.Path == s.CurrentPath && !f.IsError {
			if f.SourcePath != "" && d.controller != nil {
				id := f.ID
				t.Action, t.OnAction = "Undo", func() { d.controller.Revert(id) }
			}
			break
		}
	}
	return t
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

const (
	sidebarWide = 216
	sidebarRail = 60
	railBelow   = 820
)

func (d *Desktop) View(c *ui.Context) {
	d.theme(c)
	if d.announcement != "" && d.announcement != d.announced {
		c.Announce(d.announcement)
		d.announced = d.announcement
	}
	// A filing confirmed while the window was hidden is old news by the
	// time it shows again.
	if d.toast != nil {
		if d.now().Sub(d.toastAt) < 5*time.Second {
			c.AddToast(*d.toast)
		}
		d.toast = nil
	}
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { d.tab = "Settings" })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyT, func() {
		if d.controller != nil {
			d.controller.CheckHealth()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		switch d.tab {
		case "Pipeline":
			d.search.Focus()
		case "Help":
			d.helpSearch.Focus()
		}
	})
	// Text controls and focused buttons consume Space first in MyGo's input routing.
	if d.tab == "Dashboard" {
		c.OnShortcut(0, ui.KeySpace, d.toggleWatching)
	}
	width, _ := c.Size()
	rail := width < railBelow
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(d.pal.canvas).Children(func() {
		d.sidebar(c, rail)
		ui.Column(c.Key("main")).Grow(1).MinWidth(0).Children(func() {
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
		})
	})
	d.renameDialog(c)
	d.welcomeDialog(c)
	d.toasts(c)
}

func (d *Desktop) sidebar(c *ui.Context, rail bool) {
	p := &d.pal
	bar := c.TitleBar()
	w := float32(sidebarWide)
	if rail {
		w = sidebarRail
	}
	side := ui.Column(c.Key("sidebar")).Width(w).Shrink(0).Background(p.sidebar).
		BorderWidth(0, 1, 0, 0).BorderColor(p.border).Padding(0, 10, 12, 10).Gap(2).ClipX()
	side.Transition(ui.ElementTransition{Size: true, Animation: &glide})
	// The current page's highlight glides between items, across sections.
	var current ui.Rect
	nav := func(tab string, ic *ui.SVG, badge string) {
		if e := d.navItem(c, tab, tab, ic, rail, badge); d.tab == tab {
			current = e.Bounds()
		}
	}
	side.Children(func() {
		// The window controls sit in the sidebar's first row; the brand
		// follows them, or drops below them in the narrow rail.
		brand := ui.Row(c).Height(max(bar.Height, 44)).Gap(8).DragWindow().Shrink(0)
		if rail {
			if bar.Height > 0 {
				brand.Height(bar.Height)
				ui.Row(c).Height(40).Justify(ui.Center).DragWindow().Children(func() {
					brandMark(c, 20, p.clay, p.clayPressed).Label("Chaos")
				})
			} else {
				brand.Justify(ui.Center).Children(func() { brandMark(c, 20, p.clay, p.clayPressed).Label("Chaos") })
			}
		} else {
			brand.Padding(0, 4, 0, max(4, bar.Left-6)).Children(func() {
				brandMark(c, 18, p.clay, p.clayPressed)
				ui.Text(c, "Chaos").Font(serif).FontSize(18.5).SingleLine()
			})
		}
		ui.Box(c).Height(8)
		ui.Column(c).Gap(2).FocusGroup(ui.Vertical).Children(func() {
			badge := ""
			if n := d.errorCount(); n > 0 {
				badge = fmt.Sprint(n)
			}
			nav("Dashboard", icHome, "")
			nav("Pipeline", icList, badge)
			nav("Insights", icChart, "")
		})
		ui.Spacer(c)
		ui.Column(c).Gap(2).FocusGroup(ui.Vertical).Children(func() {
			nav("Settings", icSliders, "")
			nav("Help", icHelp, "")
		})
		ui.Box(c).Height(8)
		d.statusCard(c, rail)
	})
	// Each layout keeps its own spring, so crossing into the rail as the
	// window resizes moves the highlight at once rather than flying. The
	// first frame of a new layout still has the old one's boxes: skip it.
	key := "nav"
	if rail {
		key = "nav-rail"
	}
	if rail != d.railShown {
		d.railShown, current = rail, ui.Rect{}
	}
	if p.dark {
		ink := p.ink.Alpha(.08)
		d.glider(c, side, key, current, func(pt *ui.Painter, r ui.Rect) { pt.Fill(r, ink, 8) })
	} else {
		d.glider(c, side, key, current, d.raisedChip(8))
	}
}

func (d *Desktop) navItem(c *ui.Context, tab, label string, ic *ui.SVG, rail bool, badge string) ui.Element {
	p := &d.pal
	active := d.tab == tab
	e := ui.ButtonBase(c.Key("nav-" + tab)).Height(32).Radius(8).Gap(10).FontWeight(500)
	if rail {
		e.Width(40).Justify(ui.Center).AlignSelf(ui.Center).Label(label).Tooltip(label)
	} else {
		e.PaddingX(10)
	}
	hovered, pressed := e.Hovered(), e.Pressed()
	fg, tint, bg := p.text2, p.text2, p.hover.Alpha(0)
	switch {
	case active:
		fg, tint = p.text, p.clay
	case pressed:
		fg, tint, bg = p.text, p.text, p.pressed
	case hovered:
		fg, tint, bg = p.text, p.text, p.hover
	}
	e.Background(e.AnimateColor("bg", bg, fade)).TextColor(e.AnimateColor("fg", fg, fade))
	e.OnClick(func() { d.tab = tab })
	e.Children(func() {
		ui.Icon(c, ic).Size(16, 16).TextColor(e.AnimateColor("tint", tint, fade))
		if !rail {
			ui.Text(c, label).SingleLine().Grow(1)
			if badge != "" {
				ui.Text(c, badge).FontSize(11).FontWeight(600).TextColor(p.danger).Padding(0, 6).Radius(999).
					Background(p.danger.Alpha(.12)).FontFeatures("tnum").Label(badge + " errors")
			}
		} else if badge != "" {
			ui.Box(c).Size(6, 6).Radius(3).Background(p.danger).Absolute().Top(7).Right(8).Label(badge + " errors")
		}
	})
	return e
}

func (d *Desktop) errorCount() int {
	n := 0
	for _, f := range d.snapshot.RecentFiles {
		if f.IsError {
			n++
		}
	}
	return n
}

// statusCard shows whether Chaos is watching and which model names images,
// with a quiet start/stop control.
func (d *Desktop) statusCard(c *ui.Context, rail bool) {
	p := &d.pal
	color, live := p.text3, false
	if d.snapshot.Watching {
		color, live = p.success, true
	}
	if d.snapshot.Busy {
		color = p.clay
	}
	toggle := btn{label: "Resume watching", icon: icPlay, kind: ghost, small: true, iconOnly: true}
	if d.snapshot.Watching {
		toggle.label, toggle.icon = "Pause watching", icPause
	}
	if rail {
		ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
			d.pulse(c, color, live, d.status()).Tooltip(d.status())
			d.button(c, toggle).OnClick(d.toggleWatching)
		})
		return
	}
	health := p.text3
	switch d.snapshot.APIStatus {
	case "OK":
		health = p.success
	case "FAIL":
		health = p.danger
	}
	ui.Column(c.Key("status-card")).Padding(10, 10, 10, 12).Gap(4).Radius(10).Background(p.ink.Alpha(.035)).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			d.pulse(c, color, live, "")
			ui.Text(c, d.status()).FontSize(12).FontWeight(600).Grow(1).SingleLine()
			d.button(c, toggle).OnClick(d.toggleWatching)
		})
		ui.Row(c).Gap(6).Children(func() {
			ui.Box(c).Size(5, 5).Radius(2.5).Background(health).Label("API status " + d.snapshot.APIStatus)
			ui.Text(c, providerLabel(d.config)).FontSize(11).TextColor(p.text3).SingleLine().Grow(1).MinWidth(0)
		})
	})
}

// page frames a screen: a header that drags the window and gains a hairline
// once the body scrolls beneath it, and a centered, scrolling body.
func (d *Desktop) page(c *ui.Context, key, eyebrow, title, subtitle string, actions func(), body func()) {
	st := d.scroll[key]
	if st == nil {
		st = &ui.ScrollState{}
		d.scroll[key] = st
	}
	scrolled := st.Y > 2
	d.header(c, key, eyebrow, title, subtitle, actions, scrolled)
	ui.Scroll(c.Key(key + "-scroll")).Grow(1).TrackScroll(st).Children(func() {
		ui.Column(c).MaxWidth(944).FillWidth().Margin(0, ui.Auto).Padding(4, 32, 40, 32).Gap(20).Children(body)
	})
}

func (d *Desktop) header(c *ui.Context, key, eyebrow, title, subtitle string, actions func(), divided bool) {
	p := &d.pal
	bar := c.TitleBar()
	edge := p.border.Alpha(0)
	if divided {
		edge = p.border
	}
	h := ui.Column(c.Key(key+"-header")).Shrink(0).DragWindow().Padding(max(bar.Height, 0)+22, 32, 16, 32).BorderWidth(0, 0, 1, 0)
	h.BorderColor(h.AnimateColor("edge", edge, fade))
	width, _ := c.Size()
	side := float32(sidebarWide)
	if width < railBelow {
		side = sidebarRail
	}
	h.Children(func() {
		line := ui.Row(c).MaxWidth(880).FillWidth().Margin(0, ui.Auto).Gap(16).AlignItems(ui.End)
		if actions != nil && width-side-64 < 600 {
			line = ui.Column(c).MaxWidth(880).FillWidth().Margin(0, ui.Auto).Gap(14).AlignItems(ui.Start)
		}
		line.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				if eyebrow != "" {
					d.caption(c, eyebrow)
				}
				ui.Text(c, title).Font(serif).FontSize(30).Role(ui.RoleHeading).Level(1).SingleLine().ContentTransition(ui.ContentTransition{})
				if subtitle != "" {
					ui.Text(c, subtitle).TextColor(p.text2)
				}
			})
			if actions != nil {
				ui.Row(c).Gap(8).Shrink(0).Children(actions)
			}
		})
	})
}

// toasts draws the window's toasts as a dark pill at the bottom center,
// rising in and folding away.
func (d *Desktop) toasts(c *ui.Context) {
	p := &d.pal
	ui.ToastViewportBase(c, func(viewport ui.Element, list []ui.Toast) {
		viewport.Padding(20).AlignItems(ui.Center).Gap(8)
		for _, t := range list {
			toast := ui.ToastBase(c, t)
			toast.Root.Row().Gap(10).Padding(8, 8, 8, 12).Radius(10).Background(p.tooltip).TextColor(p.tooltipText).
				Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .2)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, .1)).MaxWidth(520)
			toast.Root.Transition(ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{Y: 12, Scale: .96}, Exit: &ui.Motion{Y: 6, Scale: .98}})
			toast.Root.Children(func() {
				tone := p.tooltipText
				ic := icCheck
				if t.Type == "success" {
					tone = darkPalette.success
					if p.dark {
						tone = lightPalette.success
					}
				}
				if t.Type == "error" {
					ic, tone = icAlert, p.danger
				}
				ui.Icon(c, ic).Size(15, 15).TextColor(tone)
				ui.Text(c, t.Title).FontSize(12.5).FontWeight(500).SingleLine().MinWidth(0).Shrink(1)
				if t.Action != "" {
					a := toast.ActionButton().Height(24).PaddingX(9).Radius(6).FontSize(12).FontWeight(600)
					bg := p.tooltipText.Alpha(.1)
					if a.Hovered() {
						bg = p.tooltipText.Alpha(.18)
					}
					a.Background(a.AnimateColor("bg", bg, fade)).Children(func() { ui.Text(c, t.Action) })
				}
				x := toast.CloseButton().Size(24, 24).Radius(6).Justify(ui.Center).Label("Close")
				bg := p.tooltipText.Alpha(0)
				if x.Hovered() {
					bg = p.tooltipText.Alpha(.12)
				}
				x.Background(x.AnimateColor("bg", bg, fade)).TextColor(p.tooltipText.Alpha(.7)).Children(func() { ui.Icon(c, icX).Size(13, 13) })
			})
		}
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
