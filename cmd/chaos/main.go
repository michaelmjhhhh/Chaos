package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/desktop"
)

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	launch, err := parseLaunch(os.Args[1:], workingDir)
	if err == flag.ErrHelp {
		fmt.Println("Usage: Chaos [--data-dir PATH] [--version] [--] [IMAGES...]")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	if launch.version {
		version := mygo.App.Version()
		if version == "" {
			version = "dev"
		}
		fmt.Println("Chaos " + version + " (MyGo native)")
		return
	}
	customDir := launch.dataDir != ""
	if !customDir {
		base, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		launch.dataDir = filepath.Join(base, "chaos")
	}
	mygo.App.SetName("Chaos")
	if customDir {
		// QA profiles use a separate single-instance socket from production.
		sum := sha256.Sum256([]byte(launch.dataDir))
		mygo.App.SetName("Chaos Preview " + hex.EncodeToString(sum[:4]))
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	controller, err := app.New(app.Options{DataDir: launch.dataDir, ImportLegacyPreferences: !customDir, Effects: app.Effects{CopyImage: copyImage, Sound: playSound, Notify: notify}})
	if err != nil {
		log.Fatal(err)
	}
	runDesktop(controller, launch.files)
}

func runDesktop(controller *app.Controller, initialFiles []string) {
	var win, panel *mygo.Window
	var tray *mygo.Tray
	var view *desktop.Desktop
	var shuttingDown, drained atomic.Bool
	show := func() {
		if win != nil {
			win.Invalidate() // tray navigation can change the shared view's selected tab
			win.Show()
			win.Focus()
		}
	}
	mygo.App.OnWindowAllClosed(func() {}) // closing a window keeps the watcher alive
	mygo.App.OnActivate(func(bool) { show() })
	mygo.App.OnSecondInstance(func(args []string, wd string) {
		if err := openLaunchFiles(args, wd, controller.ProcessFiles); err != nil {
			controller.ReportError(err)
		}
		show()
	})
	mygo.App.OnOpenFile(func(path string) { controller.ProcessFiles([]string{path}); show() })
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if drained.Load() {
			return
		}
		e.PreventDefault()
		if shuttingDown.Swap(true) {
			return
		}
		closingView, closingWindow := view, win
		go func() {
			if closingView != nil {
				closingView.Close()
			}
			if err := controller.Close(); err != nil {
				log.Printf("close history: %v", err)
			}
			if closingWindow != nil {
				closingWindow.Update(func() { drained.Store(true); mygo.App.Quit() })
			} else {
				drained.Store(true)
				mygo.App.Quit()
			}
		}()
	})
	mygo.App.WhenReady(func() {
		actions := desktop.Actions{
			PickImages:          func() { go pickImages(win, controller) },
			PickWatchDirectory:  func() { go pickDirectory(win, controller, true) },
			PickOutputDirectory: func() { go pickDirectory(win, controller, false) },
			OpenPath: func(path string) {
				go func() {
					if err := mygo.Shell.OpenPath(path); err != nil {
						controller.ReportError(err)
					}
				}()
			},
			RevealPath: func(path string) { go mygo.Shell.ShowItemInFolder(path) },
			OpenURL: func(url string) {
				go func() {
					if err := mygo.Shell.OpenExternal(url); err != nil {
						controller.ReportError(err)
					}
				}()
			},
			Quit:       mygo.App.Quit,
			ShowWindow: show,
			Dispatch: func(fn func()) {
				if win != nil {
					win.Update(fn)
				}
			},
			LoadThumbnail: thumbnail,
		}
		view = desktop.New(controller, actions)
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Chaos", Width: 1000, Height: 780, MinWidth: 620, MinHeight: 560, Content: ui.View(view.View)})
		win.OnClose(func(e *mygo.CloseEvent) {
			if !drained.Load() {
				e.PreventDefault()
				win.Hide()
			}
		})
		panel = mygo.NewWindow(mygo.WindowOptions{Title: "Chaos Menu", Width: 370, Height: 540, Hidden: true, Frameless: true, DisableResize: true, DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, AlwaysOnTop: true, SkipTaskbar: true, Content: ui.View(view.TrayView)})
		panel.OnBlur(panel.Hide)
		panel.OnClose(func(e *mygo.CloseEvent) {
			if !drained.Load() {
				e.PreventDefault()
				panel.Hide()
			}
		})
		menu := applicationMenu(controller, view, win, show, func() { go pickImages(win, controller) })
		mygo.App.SetMenu(menu)
		var err error
		tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(false), IconIsTemplate: runtime.GOOS == "darwin", ToolTip: "Chaos — Paused"})
		if err != nil {
			controller.ReportWarning("The menu bar is unavailable: " + err.Error())
		} else {
			trayMenu := mygo.NewMenu([]*mygo.MenuItem{
				{Label: "Open Chaos", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
				{Label: "Start / Stop Watching", Click: func(*mygo.MenuItem, *mygo.Window) {
					if controller.Snapshot().Watching {
						controller.Stop()
					} else {
						controller.Start()
					}
				}},
				{Label: "Settings…", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(func() { view.SelectTab("Settings") }); show() }},
				mygo.Separator(), {Role: mygo.RoleQuit},
			})
			if runtime.GOOS == "linux" {
				tray.SetMenu(trayMenu)
			} else {
				tray.OnClick(func() {
					if panel.IsVisible() {
						panel.Hide()
					} else {
						placePanel(panel, tray)
						panel.Show()
					}
				})
				tray.OnRightClick(func() { panel.Hide(); tray.SetMenu(trayMenu); defer tray.SetMenu(nil); tray.PopUpMenu() })
			}
		}
		var lastRevision uint64
		var lastWatching bool
		lastAppearance := ""
		controller.SetOnChange(func(s app.Snapshot) {
			win.Update(func() {
				if s.Revision < lastRevision {
					return
				}
				lastRevision = s.Revision
				view.SetSnapshot(s)
				if s.Config.Appearance != lastAppearance {
					lastAppearance = s.Config.Appearance
					mygo.Theme.SetSource(mygo.ThemeSource(lastAppearance))
				}
				if tray != nil {
					tray.SetToolTip("Chaos — " + s.WatcherStatus)
					if s.Watching != lastWatching {
						lastWatching = s.Watching
						_ = tray.SetIcon(trayIcon(lastWatching), runtime.GOOS == "darwin")
					}
				}
				panel.Invalidate()
			})
		})
		if s := controller.Snapshot(); s.AutoStart {
			controller.Start()
		}
		if len(initialFiles) > 0 {
			controller.ProcessFiles(initialFiles)
		}
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if tray != nil {
			tray.Destroy()
		}
	})
	if err := mygo.App.Run(); err != nil {
		if view != nil {
			view.Close()
		}
		_ = controller.Close()
		log.Fatal(err)
	}
}

func pickImages(win *mygo.Window, c *app.Controller) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Organize Existing Screenshots", Multiple: true, Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "heic", "webp"}}}})
	if err != nil {
		c.ReportError(err)
		return
	}
	if len(paths) > 0 {
		c.ProcessFiles(paths)
	}
}

func pickDirectory(win *mygo.Window, c *app.Controller, watch bool) {
	s := c.Snapshot()
	title, path := "Choose output folder", s.ResolvedOutputDir
	if watch {
		title, path = "Choose folder to watch", s.ResolvedWatchDir
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: title, DefaultPath: path, Directory: true, CreateDirectories: true})
	if err != nil {
		c.ReportError(err)
		return
	}
	if len(paths) == 0 {
		return
	}
	cfg := c.Snapshot().Config
	if watch {
		cfg.WatchDir = paths[0]
	} else {
		cfg.OutputDir = paths[0]
	}
	c.SaveConfig(cfg)
}

func applicationMenu(c *app.Controller, d *desktop.Desktop, w *mygo.Window, show, organize func()) *mygo.Menu {
	selectTab := func(tab string) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { w.Update(func() { d.SelectTab(tab) }); show() }
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Chaos", Submenu: []*mygo.MenuItem{{Role: mygo.RoleAbout}, mygo.Separator(), {Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: selectTab("Settings")}, {Label: "Check API Health", Accelerator: "CmdOrCtrl+Shift+T", Click: func(*mygo.MenuItem, *mygo.Window) { c.CheckHealth() }}, mygo.Separator(), {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit}}},
		{Label: "File", Submenu: []*mygo.MenuItem{{Label: "Organize Existing Screenshots…", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { organize() }}, {Label: "Open Chaos", Click: func(*mygo.MenuItem, *mygo.Window) { show() }}, {Role: mygo.RoleClose}}},
		{Role: mygo.RoleEditMenu},
		{Label: "View", Submenu: []*mygo.MenuItem{{Label: "Dashboard", Accelerator: "CmdOrCtrl+1", Click: selectTab("Dashboard")}, {Label: "Pipeline", Accelerator: "CmdOrCtrl+2", Click: selectTab("Pipeline")}, {Label: "Insights", Accelerator: "CmdOrCtrl+3", Click: selectTab("Insights")}, {Role: mygo.RoleToggleFullScreen}}},
		{Role: mygo.RoleWindowMenu},
		{Label: "Help", Role: mygo.RoleHelp, Submenu: []*mygo.MenuItem{{Label: "Chaos Help", Accelerator: "CmdOrCtrl+Shift+/", Click: func(*mygo.MenuItem, *mygo.Window) { w.Update(d.ShowHelp); show() }}, {Label: "Replay Welcome Guide", Click: func(*mygo.MenuItem, *mygo.Window) { w.Update(d.ShowWelcome); show() }}, {Label: "Chaos Website", Click: func(*mygo.MenuItem, *mygo.Window) {
			go mygo.Shell.OpenExternal("https://github.com/michaelmjhhhh/Chaos")
		}}}},
	})
}

func placePanel(panel *mygo.Window, tray *mygo.Tray) {
	a := tray.Bounds()
	if a.Width == 0 || a.Height == 0 {
		panel.Center()
		return
	}
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: a.X + a.Width/2, Y: a.Y + a.Height/2}).WorkArea
	b := panel.Bounds()
	x, y := a.X+(a.Width-b.Width)/2, a.Y+a.Height+6
	if y+b.Height > area.Y+area.Height {
		y = a.Y - b.Height - 6
	}
	x = max(area.X, min(x, area.X+max(0, area.Width-b.Width)))
	y = max(area.Y, min(y, area.Y+max(0, area.Height-b.Height)))
	panel.SetPosition(x, y)
}
