package desktop

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (d *Desktop) dashboard(c *ui.Context) {
	ui.Scroll(c.Key("dashboard-scroll")).Grow(1).Padding(24).Gap(20).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				small(c, strings.ToUpper(d.now().Format("Monday, 2 January")))
				title := "Ready when you are."
				if d.snapshot.Watching {
					title = "Watching for screenshots."
				}
				if d.snapshot.Busy {
					title = "Reading a new image."
				}
				heading(c, title, "")
			})
			label := "Start Watching"
			if d.snapshot.Watching {
				label = "Stop Watching"
			}
			ui.PrimaryButton(c, label).OnClick(d.toggleWatching)
		})
		d.banners(c)
		width, _ := c.Size()
		var columns ui.Element
		if width >= 780 {
			columns = ui.Row(c.Key("dashboard-wide")).Gap(20).AlignItems(ui.Start)
		} else {
			columns = ui.Column(c.Key("dashboard-columns")).Gap(20)
		}
		columns.Children(func() {
			ui.Column(c).Grow(2).Gap(12).Children(func() { d.hero(c) })
			side := ui.Column(c).Grow(1).Gap(20)
			if width >= 780 {
				side.Width(260).Shrink(0)
			}
			side.Children(func() {
				section(c, "Today", func() {
					ui.Row(c).Gap(20).Children(func() {
						metric(c, fmt.Sprint(d.snapshot.Successes), "Filed")
						metric(c, fmt.Sprint(d.snapshot.Errors), "Errors")
					})
					small(c, "Average "+duration(d.snapshot.AvgLatency)+" per image")
					small(c, "From your complete local history.")
				})
				section(c, "Folders", func() {
					small(c, "WATCHING")
					ui.Text(c, d.shorten(d.watchDir())).Font("monospace").FontSize(12).Selectable()
					ui.Divider(c)
					small(c, "OUTPUT")
					ui.Text(c, d.shorten(d.outputDir())).Font("monospace").FontSize(12).Selectable()
					ui.Button(c, "Open Output").OnClick(func() { d.open(d.outputDir()) })
				})
			})
		})
	})
}
func (d *Desktop) banners(c *ui.Context) {
	messages := []struct{ title, text string }{{"Storage", d.snapshot.StorageError}, {"Attention", d.snapshot.Error}, {"Notice", d.snapshot.Warning}, {"Notice", d.notice}}
	for i, m := range messages {
		if m.text == "" {
			continue
		}
		ui.Column(c.Key(fmt.Sprintf("banner-%d", i))).Padding(12).Gap(8).Background(c.Theme().Danger.Alpha(.08)).Border(1, c.Theme().Danger.Alpha(.3)).Radius(8).Children(func() {
			ui.Text(c, m.text).Selectable().TextColor(c.Theme().Danger)
			if i == 1 {
				ui.Row(c).Gap(8).Children(func() {
					if d.snapshot.RecoveryAction == "settings" {
						ui.Button(c, "Open Settings").OnClick(func() { d.tab = "Settings" })
					} else if d.snapshot.RecoveryAction == "retry" || d.snapshot.RecoveryAction == "internet" {
						ui.Button(c, "Retry failed images").OnClick(func() {
							if d.controller != nil {
								d.controller.RetryAll()
							}
						})
					}
					ui.Button(c, "Dismiss").OnClick(func() {
						if d.controller != nil {
							d.controller.DismissError()
						}
					})
				})
			}
		})
	}
}
func (d *Desktop) hero(c *ui.Context) {
	zone := ui.Column(c.Key("image-drop-zone")).Padding(24).Gap(16).MinHeight(270).Background(c.Theme().Surface).Radius(10).Border(1, c.Theme().Border).Label("Drop PNG, JPEG, HEIC or WebP images here to organize them")
	if zone.FileDragOver() {
		zone.Border(2, c.Theme().Accent)
	}
	if paths := zone.DroppedFiles(); len(paths) > 0 && d.controller != nil {
		d.controller.ProcessFiles(paths)
	}
	zone.Children(func() {
		if d.snapshot.CurrentFile != "" {
			if b := d.thumbnails[d.snapshot.CurrentPath]; b != nil {
				ui.Image(c, b).Height(150).FillWidth().Fit(ui.Contain).Radius(8).Label("Current image")
			}
			ui.Text(c, d.snapshot.CurrentFile).Font("monospace").FontSize(15).Selectable()
			stage := d.snapshot.CurrentStage
			if stage == "" {
				stage = "Ready"
			}
			ui.Text(c, strings.ToUpper(stage)).FontSize(11).TextColor(c.Theme().Accent)
			ui.Row(c).Gap(14).Wrap().Children(func() {
				for _, label := range []string{"Analyzing", "Renaming", "Clipboard"} {
					if label == "Clipboard" && !d.config.CopyToClipboard {
						continue
					}
					color := c.Theme().TextMuted
					if strings.EqualFold(stage, label) {
						color = c.Theme().Accent
					}
					ui.Text(c, label).FontSize(11).TextColor(color)
				}
			})
			if d.snapshot.Busy && !d.activeSince.IsZero() {
				small(c, duration(d.now().Sub(d.activeSince).Seconds())+" elapsed")
				c.After(time.Second)
			} else {
				for _, record := range d.snapshot.RecentFiles {
					if record.Path == d.snapshot.CurrentPath && !record.IsError {
						small(c, duration(record.Duration)+" elapsed")
						break
					}
				}
			}
			if d.snapshot.BatchTotal > 0 {
				small(c, fmt.Sprintf("%d of %d · %d waiting", d.snapshot.BatchIndex, d.snapshot.BatchTotal, d.snapshot.QueueLength))
			}
		} else {
			ui.Text(c, "⌘ ⇧ 4").Font("Georgia").FontSize(38).TextColor(c.Theme().Accent)
			ui.Text(c, "A clearer name for every screenshot.").Font("Georgia").FontSize(22)
			small(c, "Start watching, then take a screenshot — or drop existing images here.")
		}
		small(c, "PNG · JPEG · HEIC · WebP")
	})
	ui.PrimaryButton(c, "Organize Existing Screenshots").OnClick(func() {
		if d.actions.PickImages != nil {
			d.actions.PickImages()
		}
	})
	if d.snapshot.Busy {
		small(c, "New selections join the queue. Your images are processed one at a time.")
	}
}

// TrayView is a genuinely native panel, used by the shell's menu-bar window.
func (d *Desktop) TrayView(c *ui.Context) {
	d.theme(c)
	ui.Scroll(c).Fill().Padding(14).Gap(7).Background(c.Theme().Background).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, "Chaos").Font("Georgia").FontSize(22).Grow(1)
			label := "Start"
			if d.snapshot.Watching {
				label = "Stop"
			}
			ui.PrimaryButton(c, label).OnClick(d.toggleWatching)
		})
		small(c, d.status())
		ui.Divider(c)
		if len(d.snapshot.RecentFiles) == 0 {
			small(c, "No recent files")
		}
		for i, f := range d.snapshot.RecentFiles {
			if i >= 5 {
				break
			}
			name := fileName(f)
			if f.IsError {
				name = "Failed: " + name
			}
			ui.Button(c.Key("tray-"+f.ID), name).FillWidth().Height(27).Clip().FontSize(12).Label(name).Disabled(f.Path == "").OnClick(func() { d.reveal(f.Path) })
		}
		ui.Divider(c)
		ui.Button(c, "Open Output").OnClick(func() { d.open(d.outputDir()) })
		for _, tab := range []string{"Dashboard", "Settings", "Help"} {
			ui.Button(c.Key("tray-tab-"+tab), tab).OnClick(func() {
				d.SelectTab(tab)
				if d.actions.ShowWindow != nil {
					d.actions.ShowWindow()
				}
			})
		}
		ui.Divider(c)
		small(c, coreProviderLabel(d.config.Provider, d.config.Model))
		ui.Button(c, "Quit Chaos").OnClick(func() {
			if d.actions.Quit != nil {
				d.actions.Quit()
			}
		})
	})
}
