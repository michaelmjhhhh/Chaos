package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

type historyRow struct {
	header, day string
	file        core.RecentFile
}

var filters = []string{"All", "Errors", "Today"}

func (d *Desktop) matches(f core.RecentFile, filter, q, today string) bool {
	if filter == "Errors" && !f.IsError {
		return false
	}
	if filter == "Today" && f.Timestamp.Local().Format("2006-01-02") != today {
		return false
	}
	return q == "" || strings.Contains(f.SearchKey(), q)
}

func (d *Desktop) filteredFiles() []core.RecentFile {
	q := strings.ToLower(strings.TrimSpace(d.query))
	today := d.now().Format("2006-01-02")
	result := make([]core.RecentFile, 0, len(d.snapshot.RecentFiles))
	for _, f := range d.snapshot.RecentFiles {
		if d.matches(f, d.filter, q, today) {
			result = append(result, f)
		}
	}
	return result
}

func groupFiles(files []core.RecentFile, now time.Time) []historyRow {
	today, yesterday := now.Format("2006-01-02"), now.AddDate(0, 0, -1).Format("2006-01-02")
	var rows []historyRow
	previous := ""
	for _, f := range files {
		label, day := "Earlier", ""
		switch f.Timestamp.Local().Format("2006-01-02") {
		case today:
			label, day = "Today", now.Format("2 Jan")
		case yesterday:
			label, day = "Yesterday", now.AddDate(0, 0, -1).Format("2 Jan")
		}
		if label != previous {
			rows = append(rows, historyRow{header: label, day: day})
			previous = label
		}
		rows = append(rows, historyRow{file: f})
	}
	return rows
}

func (d *Desktop) pipeline(c *ui.Context) {
	p := &d.pal
	d.header(c, "pipeline", "", "Pipeline", "Your latest 500 attempts. Search, correct a name, or restore an original.", nil, false)
	d.toolbar(c)
	rows := groupFiles(d.filteredFiles(), d.now())
	if len(rows) == 0 {
		ui.Column(c.Key("pipeline-empty")).Grow(1).Justify(ui.Center).Children(func() {
			switch {
			case len(d.snapshot.RecentFiles) == 0:
				d.emptyState(c, icImage, "No filings yet.", "Start watching or drop images on the dashboard. Everything Chaos files will be listed here.")
			case d.query == "" && d.filter == "Errors":
				d.emptyState(c, icCheck, "No errors filed.", "Every image was named cleanly.")
			case d.query == "" && d.filter == "Today":
				d.emptyState(c, icClock, "Nothing filed today.", "Take a screenshot and it will appear here.")
			default:
				d.emptyState(c, icSearch, "No matches.", "Try a different word, or clear the search.")
			}
		})
		return
	}
	d.files.Selected = &d.selected
	d.files.Key = func(i int) any {
		if rows[i].header != "" {
			return "header-" + rows[i].header
		}
		return rows[i].file.ID
	}
	d.files.Header = func(i int) bool { return rows[i].header != "" }
	d.files.Label = func(i int) string {
		if rows[i].header != "" {
			return rows[i].header
		}
		return fileName(rows[i].file)
	}
	list := ui.List(c.Key("history-list"), &d.files, len(rows), func(i int) {
		row := rows[i]
		if row.header != "" {
			ui.Row(c).Padding(16, 8, 6, 8).Gap(8).Background(p.canvas).Children(func() {
				ui.Text(c, row.header).FontSize(12).FontWeight(600).TextColor(p.text2)
				if row.day != "" {
					d.caption(c, row.day)
				}
			})
			return
		}
		d.historyRow(c, row.file, i == d.selected)
	}).Grow(1).MaxWidth(944).FillWidth().Margin(0, ui.Auto).Padding(0, 24, 16, 24).Gap(2).Label("Filed images")
	if list.Submitted() && d.selected >= 0 && d.selected < len(rows) {
		d.open(rows[d.selected].file.Path)
	}
	if d.selected >= 0 && d.selected < len(rows) && rows[d.selected].header == "" {
		d.selectionBar(c, rows[d.selected].file)
	}
}

// toolbar holds the search field and the filter, under a hairline that
// separates them from the list scrolling below.
func (d *Desktop) toolbar(c *ui.Context) {
	p := &d.pal
	today := d.now().Format("2006-01-02")
	counts := map[string]int{}
	for _, f := range d.snapshot.RecentFiles {
		for _, name := range filters {
			if d.matches(f, name, "", today) {
				counts[name]++
			}
		}
	}
	ui.Column(c.Key("pipeline-toolbar")).Padding(0, 32, 12, 32).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Shrink(0).Children(func() {
		ui.Column(c).MaxWidth(880).FillWidth().Margin(0, ui.Auto).Gap(10).Children(func() {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				ui.Column(c).Grow(1).MinWidth(200).Children(func() {
					if d.input(c, "filing-search", &d.query, fieldOpts{label: "Search filings", placeholder: "Search filings…", icon: icSearch, handle: &d.search, clearable: true, hint: "⌘F"}).Changed() {
						d.selected = -1
					}
				})
				idx := 0
				items := make([]seg, len(filters))
				for i, name := range filters {
					if name == d.filter {
						idx = i
					}
					items[i] = seg{label: name, count: fmt.Sprint(counts[name])}
				}
				if d.segmented(c, "pipeline-filter", &idx, items).Changed() {
					d.filter, d.selected = filters[idx], -1
				}
				if d.filter == "Errors" && counts["Errors"] > 0 {
					d.button(c, btn{label: "Retry all", icon: icRetry, kind: secondary}).OnClick(func() {
						if d.controller != nil {
							d.controller.RetryAll()
						}
					})
				}
			})
			if s := d.snapshot; s.Busy {
				ui.Row(c.Key("pipeline-live")).Gap(8).Transition(ui.ElementTransition{Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).Children(func() {
					d.pulse(c, p.clay, true, "")
					ui.Text(c, fmt.Sprintf("%s · %s · %d of %d", s.CurrentStage, s.CurrentFile, s.BatchIndex, s.BatchTotal)).FontSize(12).TextColor(p.clayText).SingleLine()
				})
			}
		})
	})
}

// historyRow is one filing. The list paints its chosen row in the accent
// color; this row covers it with an opaque tint so the choice reads as a
// soft clay wash. Actions surface while the row is hovered or chosen.
func (d *Desktop) historyRow(c *ui.Context, f core.RecentFile, chosen bool) {
	p := &d.pal
	// Bleed a pixel past the list's row so its accent edge never peeks out.
	row := ui.Row(c.Key("record-" + f.ID)).Height(62).Margin(-1).PaddingX(9).Gap(12).Radius(9).TextColor(p.text)
	hovered := row.Hovered()
	bg := p.canvas
	switch {
	case chosen:
		tint := float32(.1)
		if p.dark {
			tint = .15
		}
		bg = p.canvas.Mix(p.clay, tint)
	case hovered:
		bg = p.canvas.Mix(p.ink, .045)
	}
	row.Background(row.AnimateColor("bg", bg, fade))
	row.Children(func() {
		d.thumb(c, f.Path, 40, 40, 8)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			if f.IsError {
				ui.Text(c, f.ErrorMessage).TextColor(p.danger).MaxLines(1)
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, f.OriginalName).Font(mono).FontSize(11.5).TextColor(p.text2).SingleLine().Shrink(1)
					d.caption(c, "· "+f.Timestamp.Local().Format("15:04:05")).FontFeatures("tnum")
				})
				return
			}
			ui.Text(c, fileName(f)).Font(mono).FontSize(13).SingleLine()
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, f.OriginalName).FontSize(11.5).TextColor(p.text2).SingleLine().Shrink(1)
				meta := "· " + f.Timestamp.Local().Format("15:04:05")
				if f.Duration > 0 {
					meta += " · " + duration(f.Duration)
				}
				d.caption(c, meta).FontFeatures("tnum").SingleLine()
			})
		})
		if hovered || chosen {
			ui.Row(c.Key("actions")).Gap(2).Shrink(0).Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Enter: &ui.Motion{X: 4}, Exit: &ui.Motion{}}).Children(func() {
				d.rowActions(c, f)
			})
			return
		}
		if f.IsError {
			d.pill(c, "Failed", p.danger, p.danger.Alpha(.1))
		} else {
			ui.Row(c).Size(18, 18).Radius(9).Justify(ui.Center).Background(p.success.Alpha(.12)).Shrink(0).Label("Filed").Children(func() {
				ui.Icon(c, icCheck).Size(11, 11).TextColor(p.success)
			})
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if f.IsError {
			if m.Item("Retry").Chosen() && d.controller != nil {
				d.controller.Retry(f.ID)
			}
		}
		if !f.IsError && f.Path != "" {
			if m.Item("Rename…").Chosen() {
				d.beginRename(f)
			}
			if f.SourcePath != "" && m.Item("Undo (restore original name)").Chosen() && d.controller != nil {
				d.controller.Revert(f.ID)
			}
		}
		if f.Path != "" {
			m.Separator()
			if m.Item("Open image").Chosen() {
				d.open(f.Path)
			}
			if m.Item("Reveal in Finder").Chosen() {
				d.reveal(f.Path)
			}
		}
	})
}

func (d *Desktop) rowActions(c *ui.Context, f core.RecentFile) {
	if f.IsError {
		d.button(c, btn{label: "Retry", icon: icRetry, kind: secondary, small: true}).OnClick(func() {
			if d.controller != nil {
				d.controller.Retry(f.ID)
			}
		})
		return
	}
	if f.Path == "" {
		return
	}
	d.button(c, btn{label: "Rename", icon: icPencil, kind: ghost, small: true, iconOnly: true}).OnClick(func() { d.beginRename(f) })
	if f.SourcePath != "" {
		d.button(c, btn{label: "Undo filing", icon: icUndo, kind: ghost, small: true, iconOnly: true}).OnClick(func() {
			if d.controller != nil {
				d.controller.Revert(f.ID)
			}
		})
	}
	d.button(c, btn{label: "Reveal in Finder", icon: icArrowUR, kind: ghost, small: true, iconOnly: true}).OnClick(func() { d.reveal(f.Path) })
}

// selectionBar floats under the list for the chosen filing, so the
// keyboard reaches every action without the pointer.
func (d *Desktop) selectionBar(c *ui.Context, f core.RecentFile) {
	p := &d.pal
	ui.Row(c.Key("selection-wrap")).Justify(ui.Center).Padding(0, 24, 14, 24).Shrink(0).Children(func() {
		bar := d.popover(ui.Row(c.Key("selection-actions"))).Padding(5, 5, 5, 12).Gap(4).Radius(12).MaxWidth(640)
		bar.Transition(ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{Y: 10, Scale: .98}, Exit: &ui.Motion{Y: 6}})
		bar.Children(func() {
			ui.Text(c, fileName(f)).Font(mono).FontSize(12).TextColor(p.text2).SingleLine().Shrink(1).MinWidth(60)
			ui.Box(c).Size(1, 18).Background(p.border).Margin(0, 6)
			if f.IsError {
				d.button(c, btn{label: "Retry selected", icon: icRetry, kind: ghost, small: true}).OnClick(func() {
					if d.controller != nil {
						d.controller.Retry(f.ID)
					}
				})
			} else if f.Path != "" {
				d.button(c, btn{label: "Open image", icon: icImage, kind: ghost, small: true}).OnClick(func() { d.open(f.Path) })
				d.button(c, btn{label: "Rename…", icon: icPencil, kind: ghost, small: true}).OnClick(func() { d.beginRename(f) })
				if f.SourcePath != "" {
					d.button(c, btn{label: "Undo filing", icon: icUndo, kind: ghost, small: true}).OnClick(func() {
						if d.controller != nil {
							d.controller.Revert(f.ID)
						}
					})
				}
			}
			if f.Path != "" {
				d.button(c, btn{label: "Reveal in Finder", icon: icArrowUR, kind: ghost, small: true}).OnClick(func() { d.reveal(f.Path) })
			}
		})
	})
}

func (d *Desktop) beginRename(f core.RecentFile) {
	d.renameID = f.ID
	name := fileName(f)
	d.renameExt = filepath.Ext(name)
	d.renameName = strings.TrimSuffix(name, d.renameExt)
	d.renameOpen = true
}
func (d *Desktop) commitRename() {
	if strings.TrimSpace(d.renameName) == "" {
		return
	}
	if d.controller != nil {
		d.controller.Rename(d.renameID, d.renameName)
	}
	d.renameOpen = false
}

func (d *Desktop) renameDialog(c *ui.Context) {
	p := &d.pal
	d.dialog(c, "rename-modal", &d.renameOpen, 420, func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Rename file").FontSize(15).FontWeight(600).Role(ui.RoleHeading).Level(1)
			ui.Text(c, "The file extension and current folder stay the same.").TextColor(p.text2)
		})
		if d.input(c, "rename-input", &d.renameName, fieldOpts{label: "New name", code: true, suffix: d.renameExt, autofocus: true}).Submitted() {
			d.commitRename()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			d.button(c, btn{label: "Cancel", kind: secondary}).OnClick(func() { d.renameOpen = false })
			d.button(c, btn{label: "Rename", kind: primary, disabled: strings.TrimSpace(d.renameName) == ""}).OnClick(d.commitRename)
		})
	})
}

// dialog is a panel over a softly dimmed window that settles in with a
// small scale and fade.
func (d *Desktop) dialog(c *ui.Context, key string, open *bool, width float32, build func()) {
	p := &d.pal
	ui.DialogBase(c.Key(key), open, func(backdrop, panel ui.Element) {
		dim := float32(.28)
		if p.dark {
			dim = .5
		}
		backdrop.Background(ui.RGBA(0, 0, 0, dim))
		d.popover(panel).Width(width).MaxWidthPercent(92).Padding(20).Gap(16).Radius(16)
		in := panel.Keyframes("in", []ui.Keyframe{{Value: 0}, {At: 200 * time.Millisecond, Value: 1, Ease: ui.CubicBezier(.16, 1, .3, 1)}})
		panel.Opacity(.2+.8*in).Scale(.97+.03*in, .97+.03*in).Translate(0, 6*(1-in))
		panel.Children(build)
	})
}
