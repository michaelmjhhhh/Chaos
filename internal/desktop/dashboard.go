package desktop

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (d *Desktop) dashboard(c *ui.Context) {
	title := "Ready when you are."
	if d.snapshot.Watching {
		title = "Watching for screenshots."
	}
	if d.snapshot.Busy {
		title = "Reading a new image."
	}
	d.page(c, "dashboard", d.now().Format("Monday, 2 January"), title, "", func() {
		b := btn{label: "Start Watching", icon: icPlay, kind: primary}
		if d.snapshot.Watching {
			b = btn{label: "Stop Watching", icon: icPause, kind: secondary}
		}
		d.button(c, b).OnClick(d.toggleWatching)
	}, func() {
		d.banners(c)
		width, _ := c.Size()
		var columns ui.Element
		wide := width >= 980
		if wide {
			columns = ui.Row(c.Key("dashboard-wide")).Gap(20).AlignItems(ui.Start)
		} else {
			columns = ui.Column(c.Key("dashboard-columns")).Gap(20)
		}
		columns.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(20).Children(func() {
				d.dropZone(c)
				d.recent(c)
			})
			side := ui.Column(c).Gap(20)
			if wide {
				side.Width(272).Shrink(0)
			}
			side.Children(func() {
				d.totals(c)
				d.folders(c)
			})
		})
	})
}

func (d *Desktop) banners(c *ui.Context) {
	p := &d.pal
	dismiss := func() {
		d.button(c, btn{label: "Dismiss", icon: icX, kind: ghost, small: true, iconOnly: true}).OnClick(func() {
			if d.controller != nil {
				d.controller.DismissError()
			}
		})
	}
	if s := d.snapshot.StorageError; s != "" {
		d.alert(c, "storage", p.danger, icAlert, s, nil)
	}
	if s := d.snapshot.Error; s != "" {
		d.alert(c, "error", p.danger, icAlert, s, func() {
			switch d.snapshot.RecoveryAction {
			case core.RecoverySettings:
				d.button(c, btn{label: "Open Settings", kind: secondary, small: true}).OnClick(func() { d.tab = "Settings" })
			case core.RecoveryRetry, core.RecoveryInternet:
				d.button(c, btn{label: "Retry failed images", icon: icRetry, kind: secondary, small: true}).OnClick(func() {
					if d.controller != nil {
						d.controller.RetryAll()
					}
				})
			}
			dismiss()
		})
	}
	if s := d.snapshot.Warning; s != "" {
		d.alert(c, "warning", p.warn, icWarn, s, dismiss)
	}
}

// dropZone is the dashboard's centerpiece: a soft card that warms to clay
// and lifts slightly while files are dragged over it, and shows the image
// being named once work begins.
func (d *Desktop) dropZone(c *ui.Context) {
	p := &d.pal
	zone := ui.Column(c.Key("image-drop-zone")).Radius(16).Padding(24).Gap(16).MinHeight(276).Justify(ui.Center).
		Background(p.card).Label("Drop PNG, JPEG, HEIC or WebP images here to organize them")
	dragging, hovered := zone.FileDragOver(), zone.Hovered()
	edge, glow := p.border, p.clay.Alpha(0)
	switch {
	case dragging:
		edge, glow = p.clay, p.clay.Alpha(.16)
	case hovered:
		edge = p.borderStrong
	}
	zone.Border(1, zone.AnimateColor("edge", edge, fade)).InsetShadow(0, 0, 32, -6, zone.AnimateColor("glow", glow, fade))
	if dragging {
		zone.InsetShadow(0, 0, 0, 1, p.clay)
	}
	if !p.dark {
		lift := float32(.035)
		if hovered || dragging {
			lift = .06
		}
		zone.Shadow(0, 4, 22, 0, ui.RGBA(0, 0, 0, zone.AnimateTo("lift", lift, fade)))
	}
	scale := float32(1)
	if dragging {
		scale = 1.008
	}
	zone.AnimatePose(ui.Pose{Scale: scale}, glide)
	if paths := zone.DroppedFiles(); len(paths) > 0 && d.controller != nil {
		d.controller.ProcessFiles(paths)
	}
	zone.Children(func() {
		if d.snapshot.CurrentFile != "" {
			d.progress(c)
		} else {
			d.idle(c, dragging)
		}
	})
}

func (d *Desktop) idle(c *ui.Context, dragging bool) {
	p := &d.pal
	ui.Column(c.Key("idle")).Gap(16).AlignItems(ui.Center).Transition(ui.ElementTransition{Enter: &ui.Motion{Y: 6}}).Children(func() {
		tile := ui.Row(c).Size(52, 52).Radius(15).Justify(ui.Center).Background(p.claySoft)
		ic, y, s := icImageAdd, float32(0), float32(1)
		if dragging {
			ic, y, s = icDownload, -3, 1.06
		}
		tile.AnimatePose(ui.Pose{Y: y, Scale: s}, glide).Children(func() {
			ui.Icon(c, ic).Size(24, 24).TextColor(p.clay).ContentTransition(ui.ContentTransition{})
		})
		ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
			title := "A clearer name for every screenshot."
			if dragging {
				title = "Release to organize."
			}
			ui.Text(c, title).Font(serif).FontSize(23).TextAlign(ui.Center).ContentTransition(ui.ContentTransition{})
			ui.Text(c, "Start watching, then take a screenshot — or drop existing images here.").TextColor(p.text2).TextAlign(ui.Center).MaxWidth(380)
		})
		d.button(c, btn{label: "Organize Existing Screenshots", icon: icFolder, kind: secondary}).OnClick(d.pickImages)
		ui.Row(c).Gap(8).Children(func() {
			d.keys(c, "⌘⇧4")
			d.caption(c, "to capture")
			ui.Box(c).Size(3, 3).Radius(1.5).Background(p.text3).Opacity(.6)
			d.caption(c, "PNG · JPEG · HEIC · WebP")
		})
	})
}

func (d *Desktop) pickImages() {
	if d.actions.PickImages != nil {
		d.actions.PickImages()
	}
}

// progress shows the image being named: its preview, what stage it is in,
// a stepper through the pipeline, and the batch around it.
func (d *Desktop) progress(c *ui.Context) {
	p := &d.pal
	s := d.snapshot
	ui.Column(c.Key("progress")).Gap(18).Transition(ui.ElementTransition{Enter: &ui.Motion{Y: 6}}).Children(func() {
		ui.Row(c).Gap(20).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Radius(10).Clip().Shrink(0).Children(func() { d.thumb(c, s.CurrentPath, 152, 114, 10) }).Label("Current image")
			ui.Column(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					d.stagePill(c, s.CurrentStage)
					ui.Spacer(c)
					if s.Busy && !d.activeSince.IsZero() {
						d.caption(c, duration(d.now().Sub(d.activeSince).Seconds())+" elapsed").FontFeatures("tnum")
						c.After(100 * time.Millisecond)
					} else {
						for _, record := range s.RecentFiles {
							if record.Path == s.CurrentPath && !record.IsError {
								d.caption(c, duration(record.Duration)+" elapsed").FontFeatures("tnum")
								break
							}
						}
					}
				})
				ui.Text(c, s.CurrentFile).Font(mono).FontSize(14.5).Selectable().MaxLines(2).ContentTransition(ui.ContentTransition{})
				if s.CurrentStage == app.StageError && s.Error != "" {
					ui.Text(c, s.Error).FontSize(12).TextColor(p.danger).MaxLines(2)
				} else {
					d.stepper(c, s.CurrentStage)
				}
			})
		})
		if s.BatchTotal > 0 {
			d.batch(c)
		}
		ui.Row(c).Gap(10).Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
			msg := "Drop more images at any time."
			if s.Busy {
				msg = "New selections join the queue. Images are processed one at a time."
			}
			d.muted(c, msg).Grow(1).MinWidth(0)
			d.button(c, btn{label: "Organize Existing Screenshots", icon: icFolder, kind: secondary, small: true}).OnClick(d.pickImages)
		})
	})
}

func (d *Desktop) stagePill(c *ui.Context, stage app.ProcessingStage) {
	p := &d.pal
	label, tone := "Ready", p.text2
	live := false
	switch stage {
	case app.StageAnalyzing:
		label, tone, live = "Analyzing", p.clay, true
	case app.StageRenaming:
		label, tone, live = "Renaming", p.clay, true
	case app.StageClipboard:
		label, tone, live = "Copying", p.clay, true
	case app.StageSuccess:
		label, tone = "Filed", p.success
	case app.StageError:
		label, tone = "Failed", p.danger
	}
	ui.Row(c.Key("stage")).Height(22).Padding(0, 9, 0, 8).Gap(6).Radius(999).Background(tone.Alpha(.11)).TextColor(tone).Children(func() {
		switch {
		case live:
			d.pulse(c, tone, true, "")
		case stage == app.StageSuccess:
			ui.Icon(c, icCheck).Size(12, 12)
		case stage == app.StageError:
			ui.Icon(c, icAlert).Size(12, 12)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(600).ContentTransition(ui.ContentTransition{})
	})
}

// stepper walks Analyzing → Renaming (→ Clipboard). Finished steps fill
// with clay, the current one breathes, and the connectors fill as work
// moves along.
func (d *Desktop) stepper(c *ui.Context, stage app.ProcessingStage) {
	p := &d.pal
	steps := []string{"Analyzing", "Renaming"}
	if d.config.CopyToClipboard {
		steps = append(steps, "Clipboard")
	}
	current := -1
	switch stage {
	case app.StageAnalyzing:
		current = 0
	case app.StageRenaming:
		current = 1
	case app.StageClipboard:
		current = 2
	case app.StageSuccess:
		current = len(steps)
	}
	ui.Row(c.Key("stepper")).Gap(8).Children(func() {
		for i, name := range steps {
			if i > 0 {
				line := ui.Box(c).Height(1.5).Grow(1).MinWidth(12).Radius(1)
				tone := p.border
				if i <= current {
					tone = p.clay.Alpha(.55)
				}
				line.Background(line.AnimateColor("tone", tone, ui.Animation{Duration: 240 * time.Millisecond}))
			}
			ui.Row(c.Key(name)).Gap(6).Shrink(0).Children(func() {
				dot := ui.Row(c).Size(16, 16).Radius(8).Justify(ui.Center)
				switch {
				case i < current:
					dot.Background(dot.AnimateColor("bg", p.clay, fade)).TextColor(white).Children(func() { ui.Icon(c, icCheck).Size(10, 10) })
				case i == current:
					dot.Background(dot.AnimateColor("bg", p.claySoft, fade)).Children(func() { d.pulse(c, p.clay, true, "") })
				default:
					dot.Background(dot.AnimateColor("bg", p.card, fade)).Border(1.5, p.borderStrong)
				}
				tone := p.text3
				if i < current {
					tone = p.text2
				} else if i == current {
					tone = p.text
				}
				ui.Text(c, name).FontSize(12).FontWeight(500).TextColor(tone)
			})
		}
	})
}

func (d *Desktop) batch(c *ui.Context) {
	p := &d.pal
	s := d.snapshot
	ui.Column(c.Key("batch")).Gap(6).Children(func() {
		ui.Row(c).Children(func() {
			d.caption(c, fmt.Sprintf("%d of %d", s.BatchIndex, s.BatchTotal)).FontFeatures("tnum").Grow(1)
			d.caption(c, fmt.Sprintf("%d waiting", s.QueueLength)).FontFeatures("tnum")
		})
		track := ui.Box(c).Height(4).Radius(2).Background(p.fill).Label(fmt.Sprintf("%d of %d · %d waiting", s.BatchIndex, s.BatchTotal, s.QueueLength))
		ratio := track.AnimateTo("ratio", float32(s.BatchIndex)/float32(max(1, s.BatchTotal)), glide)
		clay := p.clay
		track.Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*ratio), H: r.H}, clay, r.H/2)
		})
	})
}

// recent lists the latest filings; a click reveals one in Finder.
func (d *Desktop) recent(c *ui.Context) {
	p := &d.pal
	d.card(ui.Column(c.Key("recent"))).Padding(6).Gap(2).Children(func() {
		ui.Row(c).Padding(8, 6, 6, 10).Children(func() {
			ui.Text(c, "Recent").FontWeight(600).Grow(1)
			d.button(c, btn{label: "View all", trailing: icArrowR, kind: ghost, small: true}).OnClick(func() { d.tab = "Pipeline" })
		})
		files := d.snapshot.RecentFiles
		if len(files) == 0 {
			d.muted(c, "Nothing filed yet. Your latest images will appear here.").Padding(4, 10, 12, 10)
			return
		}
		for i, f := range files {
			if i >= 5 {
				break
			}
			row := ui.ButtonBase(c.Key("recent-" + f.ID)).Height(48).PaddingX(8).Gap(12).Radius(8).Label(fileName(f))
			bg := p.hover.Alpha(0)
			if row.Pressed() {
				bg = p.pressed
			} else if row.Hovered() {
				bg = p.hover
			}
			row.Background(row.AnimateColor("bg", bg, fade))
			if f.IsError {
				row.OnClick(func() { d.tab, d.filter = "Pipeline", "Errors" })
			} else {
				row.Disabled(f.Path == "").OnClick(func() { d.reveal(f.Path) })
			}
			hovered := row.Hovered()
			row.Children(func() {
				d.thumb(c, f.Path, 32, 32, 7)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					if f.IsError {
						ui.Text(c, f.OriginalName).Font(mono).FontSize(12.5).SingleLine()
						ui.Text(c, f.ErrorMessage).FontSize(11.5).TextColor(p.danger).SingleLine()
					} else {
						ui.Text(c, fileName(f)).Font(mono).FontSize(12.5).SingleLine()
						d.caption(c, "from "+f.OriginalName).SingleLine()
					}
				})
				ui.Text(c, f.Timestamp.Local().Format("15:04")).FontSize(11.5).TextColor(p.text3).FontFeatures("tnum")
				ic := ui.Icon(c, icArrowUR).Size(13, 13).TextColor(p.text2)
				shown := float32(0)
				if hovered && !f.IsError {
					shown = 1
				}
				ic.Opacity(ic.AnimateTo("shown", shown, fade))
			})
		}
	})
}

func (d *Desktop) totals(c *ui.Context) {
	p := &d.pal
	s := d.snapshot
	d.card(ui.Column(c.Key("totals"))).Padding(16).Gap(14).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "Today").FontWeight(600).Grow(1)
			d.caption(c, "From your local history")
		})
		ui.Row(c).Gap(24).Dividers(1, p.border).Children(func() {
			stat := func(value, label string, tone ui.Color) {
				ui.Column(c.Key(label)).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, value).Font(serif).FontSize(29).TextColor(tone).FontFeatures("tnum").ContentTransition(ui.ContentTransition{Numeric: true})
					d.caption(c, label)
				})
			}
			stat(fmt.Sprint(s.Successes), "Filed", p.text)
			tone := p.text
			if s.Errors > 0 {
				tone = p.danger
			}
			stat(fmt.Sprint(s.Errors), "Errors", tone)
			stat(duration(s.AvgLatency), "Average", p.text)
		})
	})
}

func (d *Desktop) folders(c *ui.Context) {
	d.card(ui.Column(c.Key("folders"))).Padding(6).Gap(2).Children(func() {
		ui.Text(c, "Folders").FontWeight(600).Padding(8, 10, 4, 10)
		d.folderRow(c, "Watching", d.watchDir(), icFolder)
		d.folderRow(c, "Output", d.snapshot.ResolvedOutputDir, icFolder)
	})
}

// folderRow opens its folder on click; an arrow appears on hover to say so.
func (d *Desktop) folderRow(c *ui.Context, label, path string, ic *ui.SVG) {
	p := &d.pal
	row := ui.ButtonBase(c.Key("folder-"+label)).Padding(8, 10).Gap(12).Radius(8).Label("Open " + label + " folder").Tooltip(path)
	bg := p.hover.Alpha(0)
	if row.Pressed() {
		bg = p.pressed
	} else if row.Hovered() {
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, fade)).Disabled(path == "").OnClick(func() { d.open(path) })
	shown := float32(0)
	if row.Hovered() {
		shown = 1
	}
	row.Children(func() {
		ui.Row(c).Size(30, 30).Radius(8).Justify(ui.Center).Background(p.fill).Shrink(0).Children(func() {
			ui.Icon(c, ic).Size(15, 15).TextColor(p.text2)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			d.caption(c, label)
			ui.Text(c, d.shorten(path)).Font(mono).FontSize(12).SingleLine()
		})
		a := ui.Icon(c, icArrowUR).Size(13, 13).TextColor(p.text2)
		a.Opacity(a.AnimateTo("shown", shown, fade))
	})
}

// TrayView is the menu-bar panel: status and the main switch up top, the
// latest filings, then menu-like actions.
func (d *Desktop) TrayView(c *ui.Context) {
	d.theme(c)
	p := &d.pal
	ui.Column(c).Fill().Background(p.canvas).Children(func() {
		ui.Column(c).Padding(14, 14, 12, 14).Gap(10).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				brandMark(c, 18, p.clay, p.clayPressed)
				ui.Text(c, "Chaos").Font(serif).FontSize(18.5).Grow(1)
				b := btn{label: "Start", icon: icPlay, kind: primary, small: true}
				if d.snapshot.Watching {
					b = btn{label: "Stop", icon: icPause, kind: secondary, small: true}
				}
				d.button(c, b).OnClick(d.toggleWatching)
			})
			ui.Row(c).Gap(8).Children(func() {
				color := p.text3
				if d.snapshot.Watching {
					color = p.success
				}
				d.pulse(c, color, d.snapshot.Watching, "")
				ui.Text(c, d.status()).FontSize(12).FontWeight(500)
				ui.Text(c, providerLabel(d.config)).FontSize(11.5).TextColor(p.text3).SingleLine().Grow(1).MinWidth(0).TextAlign(ui.End)
			})
		})
		ui.Scroll(c.Key("tray-recent")).Grow(1).Padding(6).Gap(1).Children(func() {
			d.caption(c, "Recent").Padding(6, 8, 4, 8)
			if len(d.snapshot.RecentFiles) == 0 {
				d.muted(c, "No recent files").Padding(4, 8)
			}
			for i, f := range d.snapshot.RecentFiles {
				if i >= 5 {
					break
				}
				name := fileName(f)
				row := ui.ButtonBase(c.Key("tray-" + f.ID)).Height(38).PaddingX(8).Gap(10).Radius(7)
				if f.IsError {
					row.Label("Failed: " + name)
				}
				bg := p.hover.Alpha(0)
				if row.Hovered() {
					bg = p.hover
				}
				row.Background(row.AnimateColor("bg", bg, fade)).OnClick(func() {
					if f.Path != "" {
						d.reveal(f.Path)
						return
					}
					// Failed images have no filed copy: show them in Pipeline.
					d.tab, d.filter = "Pipeline", "Errors"
					if d.actions.ShowWindow != nil {
						d.actions.ShowWindow()
					}
				})
				row.Children(func() {
					d.thumb(c, f.Path, 24, 24, 5)
					ui.Text(c, name).Font(mono).FontSize(12).SingleLine().Grow(1).MinWidth(0)
					if f.IsError {
						ui.Icon(c, icAlert).Size(13, 13).TextColor(p.danger)
					} else {
						ui.Text(c, f.Timestamp.Local().Format("15:04")).FontSize(11).TextColor(p.text3).FontFeatures("tnum")
					}
				})
			}
		})
		ui.Column(c).Padding(6).Gap(1).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
			d.menuRow(c, "Open Output", icFolder, func() { d.open(d.snapshot.ResolvedOutputDir) })
			for _, item := range []struct {
				tab string
				ic  *ui.SVG
			}{{"Dashboard", icHome}, {"Settings", icSliders}, {"Help", icHelp}} {
				d.menuRow(c, item.tab, item.ic, func() {
					d.SelectTab(item.tab)
					if d.actions.ShowWindow != nil {
						d.actions.ShowWindow()
					}
				})
			}
			ui.Box(c).Height(1).Background(p.border).Margin(4, 8)
			d.menuRow(c, "Quit Chaos", icPower, func() {
				if d.actions.Quit != nil {
					d.actions.Quit()
				}
			})
		})
	})
}

func (d *Desktop) menuRow(c *ui.Context, label string, ic *ui.SVG, action func()) {
	p := &d.pal
	row := ui.ButtonBase(c.Key("menu-" + label)).Height(30).PaddingX(8).Gap(10).Radius(6).Justify(ui.Start)
	bg, fg := p.hover.Alpha(0), p.text2
	if row.Pressed() {
		bg, fg = p.pressed, p.text
	} else if row.Hovered() {
		bg, fg = p.hover, p.text
	}
	row.Background(row.AnimateColor("bg", bg, fade)).TextColor(fg).OnClick(action).Children(func() {
		ui.Icon(c, ic).Size(14, 14)
		ui.Text(c, label).FontSize(12.5)
	})
}
