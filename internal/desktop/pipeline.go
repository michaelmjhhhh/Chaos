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
	header string
	file   core.RecentFile
}

func (d *Desktop) filteredFiles() []core.RecentFile {
	q := strings.ToLower(strings.TrimSpace(d.query))
	today := d.now().Format("2006-01-02")
	result := make([]core.RecentFile, 0, len(d.snapshot.RecentFiles))
	for _, f := range d.snapshot.RecentFiles {
		if d.filter == "Errors" && !f.IsError {
			continue
		}
		if d.filter == "Today" && f.Timestamp.Local().Format("2006-01-02") != today {
			continue
		}
		if q != "" && !strings.Contains(f.SearchKey(), q) {
			continue
		}
		result = append(result, f)
	}
	return result
}
func groupFiles(files []core.RecentFile, now time.Time) []historyRow {
	today, yesterday := now.Format("2006-01-02"), now.AddDate(0, 0, -1).Format("2006-01-02")
	var rows []historyRow
	previous := ""
	for _, f := range files {
		label := "EARLIER"
		day := f.Timestamp.Local().Format("2006-01-02")
		if day == today {
			label = "TODAY · " + now.Format("2 Jan")
		} else if day == yesterday {
			label = "YESTERDAY · " + now.AddDate(0, 0, -1).Format("2 Jan")
		}
		if label != previous {
			rows = append(rows, historyRow{header: label})
			previous = label
		}
		rows = append(rows, historyRow{file: f})
	}
	return rows
}
func (d *Desktop) pipeline(c *ui.Context) {
	ui.Column(c.Key("pipeline")).Grow(1).Padding(20).Gap(12).Children(func() {
		heading(c, "Pipeline", "Your latest 500 attempts. Search, correct a name, or restore an original.")
		if d.snapshot.Busy {
			ui.Text(c, fmt.Sprintf("%s · %s · %d of %d", d.snapshot.CurrentStage, d.snapshot.CurrentFile, d.snapshot.BatchIndex, d.snapshot.BatchTotal)).TextColor(c.Theme().Accent).SingleLine()
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.TextInput(c.Key("filing-search"), &d.query).Bind(&d.search).Label("Search filings").Placeholder("Search filings…").Grow(1).MinWidth(180).Changed() {
				d.selected = -1
			}
			for _, f := range []string{"All", "Errors", "Today"} {
				b := ui.Button(c.Key("filter-"+f), f).OnClick(func() { d.filter = f; d.selected = -1 })
				if d.filter == f {
					b.TextColor(c.Theme().Accent).Background(c.Theme().Accent.Alpha(.12))
				}
			}
			if d.filter == "Errors" {
				ui.Button(c, "Retry all").OnClick(func() {
					if d.controller != nil {
						d.controller.RetryAll()
					}
				})
			}
		})
		rows := groupFiles(d.filteredFiles(), d.now())
		if len(rows) == 0 {
			msg := "No matches."
			if len(d.snapshot.RecentFiles) == 0 {
				msg = "No filings yet."
			} else if d.query == "" && d.filter == "Errors" {
				msg = "No errors filed."
			} else if d.query == "" && d.filter == "Today" {
				msg = "Nothing filed today."
			}
			ui.Text(c, msg).Font("Georgia").FontSize(24).Padding(28).TextColor(c.Theme().TextMuted)
			ui.Spacer(c)
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
				ui.Text(c, row.header).FontSize(11).Padding(10, 8).Background(c.Theme().Background).TextColor(c.Theme().TextMuted)
				return
			}
			d.historyCard(c, row.file)
		}).Grow(1).Label("Filed images")
		if list.Submitted() && d.selected >= 0 && d.selected < len(rows) {
			d.open(rows[d.selected].file.Path)
		}
		if d.selected >= 0 && d.selected < len(rows) && rows[d.selected].header == "" {
			d.fileActions(c, rows[d.selected].file)
		}
	})
}
func (d *Desktop) historyCard(c *ui.Context, f core.RecentFile) {
	row := ui.Row(c.Key("record-"+f.ID)).Padding(10).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(c.Theme().Border)
	row.Children(func() {
		if bitmap := d.thumbnail(c, f.Path); bitmap != nil {
			ui.Image(c, bitmap).Size(44, 44).Fit(ui.Cover).Radius(6)
		} else {
			ui.Box(c).Size(44, 44).Radius(6).Background(c.Theme().Border.Alpha(.35)).Children(func() { ui.Text(c, "▧").Padding(10).TextColor(c.Theme().TextMuted) })
		}
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			if f.IsError {
				ui.Text(c, f.ErrorMessage).TextColor(c.Theme().Danger).MaxLines(2)
			} else {
				ui.Text(c, fileName(f)).Font("monospace").SingleLine()
			}
			ui.Text(c, f.OriginalName).Font("Georgia").Italic().FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
			small(c, f.Timestamp.Local().Format("15:04:05"))
		})
		if !f.IsError {
			ui.Text(c, "✓").TextColor(c.Theme().Success)
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
func (d *Desktop) fileActions(c *ui.Context, f core.RecentFile) {
	ui.Row(c.Key("selection-actions")).Gap(8).Wrap().Children(func() {
		if f.IsError {
			ui.Button(c, "Retry selected").OnClick(func() {
				if d.controller != nil {
					d.controller.Retry(f.ID)
				}
			})
		} else if f.Path != "" {
			ui.Button(c, "Open image").OnClick(func() { d.open(f.Path) })
			ui.Button(c, "Rename…").OnClick(func() { d.beginRename(f) })
			if f.SourcePath != "" {
				ui.Button(c, "Undo filing").OnClick(func() {
					if d.controller != nil {
						d.controller.Revert(f.ID)
					}
				})
			}
		}
		if f.Path != "" {
			ui.Button(c, "Reveal in Finder").OnClick(func() { d.reveal(f.Path) })
		}
	})
}
func (d *Desktop) beginRename(f core.RecentFile) {
	d.renameID = f.ID
	name := fileName(f)
	d.renameName = strings.TrimSuffix(name, filepath.Ext(name))
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
	ui.Modal(c.Key("rename-modal"), &d.renameOpen, func() {
		heading(c, "Rename file", "The file extension and current folder stay the same.")
		if ui.TextInput(c.Key("rename-input"), &d.renameName).Label("New name").AutoFocus().Submitted() {
			d.commitRename()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, "Cancel").OnClick(func() { d.renameOpen = false })
			ui.PrimaryButton(c, "Rename").Disabled(strings.TrimSpace(d.renameName) == "").OnClick(d.commitRename)
		})
	})
}
