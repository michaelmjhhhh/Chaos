package desktop

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (d *Desktop) insights(c *ui.Context) {
	p := &d.pal
	s := d.snapshot.Insights
	d.page(c, "insights", "", "Insights", "The almanac of your screenshot archive.", nil, func() {
		if d.snapshot.StorageError != "" {
			d.alert(c, "storage", p.danger, icAlert, d.snapshot.StorageError, nil)
		}
		if s.TotalProcessed == 0 {
			d.emptyState(c, icChart, "No images filed yet", "Insights fill in as Chaos renames and files your screenshots.")
			return
		}
		ui.Row(c).Gap(16).Wrap().AlignItems(ui.Stretch).Children(func() {
			d.kpi(c, "Images processed", fmt.Sprint(s.TotalProcessed), archiveSince(s.Daily), func() {
				ui.Row(c).Gap(8).Children(func() {
					d.muted(c, fmt.Sprintf("%d this month · %d this week", s.ThisMonth, d.weekCount())).Grow(1).SingleLine()
					if delta := s.MonthOverMonth(); delta != nil {
						tone, ic := p.success, icTrendUp
						if *delta < 0 {
							tone, ic = p.danger, icTrendDn
						}
						ui.Row(c).Gap(4).Padding(1, 7).Radius(999).Background(tone.Alpha(.11)).TextColor(tone).Shrink(0).
							Label(fmt.Sprintf("%+.0f%% vs last month", *delta*100)).Children(func() {
							ui.Icon(c, ic).Size(11, 11)
							ui.Text(c, fmt.Sprintf("%+.0f%%", *delta*100)).FontSize(11).FontWeight(600).FontFeatures("tnum")
						})
					}
				})
			})
			d.kpi(c, "Success rate", fmt.Sprintf("%.0f%%", s.SuccessRate()*100), "Filed cleanly", func() {
				rate := float32(s.SuccessRate())
				success, danger, track := p.success, p.danger.Alpha(.55), p.fill
				bar := ui.Box(c).Height(4).Radius(2).Background(track).Label(fmt.Sprintf("%d filed, %d errors", s.Successes, s.Errors))
				shown := bar.AnimateTo("rate", rate, glide)
				bar.Draw(func(pt *ui.Painter, r ui.Rect) {
					w := r.W * shown
					if s.Errors > 0 {
						pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, danger, r.H/2)
					}
					pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(w, r.H), H: r.H}, success, r.H/2)
				})
				d.muted(c, fmt.Sprintf("%d filed · %s", s.Successes, plural(s.Errors, "error", "errors")))
			})
			d.kpi(c, "Time saved", formatTimeSaved(s.TimeSaved()), "Estimated", func() {
				d.muted(c, "≈ 30s of manual filing per image")
			})
		})
		d.activity(c, s)
		width, _ := c.Size()
		wide := width >= 980
		var panels ui.Element
		if wide {
			panels = ui.Row(c.Key("insight-panels")).Gap(16).AlignItems(ui.Stretch)
		} else {
			panels = ui.Column(c.Key("insight-panels")).Gap(16)
		}
		panels.Children(func() {
			d.panel(c, "What you capture", wide, func() {
				d.caption(c, "Top categories")
				breakdown(c, &d.pal, s.TopCategories)
				ui.Box(c).Height(1).Background(p.border).Margin(4, 0)
				d.caption(c, "File types")
				d.fileTypes(c, s.FileTypes)
			})
			d.panel(c, "Performance", wide, func() {
				ui.Row(c).Gap(24).Dividers(1, p.border).Children(func() {
					d.figure(c, duration(s.AvgDuration), "Average per image")
					d.figure(c, duration(s.P95Duration), "95th percentile")
				})
				if s.AvgDuration <= 0 {
					d.muted(c, "No timing data yet.")
				}
				d.sparkline(c)
				ui.Column(c).Gap(2).Children(func() {
					if s.Fastest != nil {
						d.record(c, "Fastest", s.Fastest)
					}
					if s.Slowest != nil {
						d.record(c, "Slowest", s.Slowest)
					}
				})
			})
		})
	})
}

func (d *Desktop) kpi(c *ui.Context, title, value, note string, more func()) {
	d.card(ui.Column(c.Key("kpi-" + title))).Grow(1).Basis(0).MinWidth(200).Padding(16).Gap(10).Children(func() {
		ui.Text(c, title).FontSize(12).FontWeight(500).TextColor(d.pal.text2)
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, value).Font(serif).FontSize(35).FontFeatures("tnum").ContentTransition(ui.ContentTransition{Numeric: true})
			d.caption(c, note)
		})
		more()
	})
}

func (d *Desktop) panel(c *ui.Context, title string, inRow bool, build func()) {
	e := d.card(ui.Column(c.Key("panel-" + title))).MinWidth(0).Padding(16).Gap(12)
	if inRow {
		// Share the row evenly; stacked, each keeps its own height.
		e.Grow(1).Basis(0)
	}
	e.Children(func() {
		ui.Text(c, title).FontWeight(600).Role(ui.RoleHeading).Level(2)
		build()
	})
}

func (d *Desktop) figure(c *ui.Context, value, label string) {
	ui.Column(c.Key(label)).Grow(1).Basis(0).Gap(2).Children(func() {
		ui.Text(c, value).Font(serif).FontSize(26).FontFeatures("tnum")
		d.caption(c, label)
	})
}

func (d *Desktop) record(c *ui.Context, label string, r *core.DurationRecord) {
	p := &d.pal
	ui.Row(c.Key("record-"+label)).Gap(10).Padding(6, 0).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.text2).Width(56).Shrink(0)
		ui.Text(c, r.Name).Font(mono).FontSize(11.5).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, duration(r.Duration)).FontSize(12).FontWeight(500).FontFeatures("tnum").Shrink(0)
	})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func archiveSince(daily map[string]int) string {
	var earliest time.Time
	for day, count := range daily {
		if count <= 0 {
			continue
		}
		at, err := time.Parse("2006-01-02", day)
		if err == nil && (earliest.IsZero() || at.Before(earliest)) {
			earliest = at
		}
	}
	if earliest.IsZero() {
		return "Lifetime"
	}
	return "Lifetime · since " + earliest.Format("Jan 2006")
}
func (d *Desktop) weekCount() int {
	n := 0
	now := d.now()
	for day := 0; day < 7; day++ {
		n += d.snapshot.Insights.Daily[now.AddDate(0, 0, -day).Format("2006-01-02")]
	}
	return n
}
func formatTimeSaved(v time.Duration) string {
	if v < time.Minute {
		return fmt.Sprintf("%ds", int(v.Seconds()))
	}
	if v < time.Hour {
		return fmt.Sprintf("%.0f min", v.Minutes())
	}
	return fmt.Sprintf("%.1f hrs", v.Hours())
}

// breakdown draws a ranked list with thin bars that grow in from the left.
func breakdown(c *ui.Context, p *palette, items []core.Labeled) {
	if len(items) == 0 {
		ui.Text(c, "No filed images yet.").FontSize(12).TextColor(p.text2)
		return
	}
	peak := 0
	for _, item := range items {
		peak = max(peak, item.Count)
	}
	ui.Column(c).Gap(10).Children(func() {
		for i, item := range items {
			ui.Column(c.Key(item.Label)).Gap(5).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, item.Label).SingleLine().Grow(1)
					ui.Text(c, fmt.Sprint(item.Count)).FontSize(12).TextColor(p.text2).FontFeatures("tnum")
				})
				bar := ui.Box(c).Height(4).Radius(2).Background(p.fill)
				a := glide
				a.Delay = time.Duration(i) * 40 * time.Millisecond
				ratio := bar.AnimateTo("ratio", float32(item.Count)/float32(max(1, peak)), a)
				tone := p.clay.Alpha(.45 + .55*float32(item.Count)/float32(max(1, peak)))
				bar.Draw(func(pt *ui.Painter, r ui.Rect) {
					if ratio > 0 {
						pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*ratio), H: r.H}, tone, r.H/2)
					}
				})
			})
		}
	})
}

// fileTypes is a single stacked bar with a legend, which reads shares at
// a glance better than a bar per type.
func (d *Desktop) fileTypes(c *ui.Context, items []core.Labeled) {
	p := &d.pal
	if len(items) == 0 {
		d.muted(c, "No filed images yet.")
		return
	}
	total := 0
	for _, it := range items {
		total += it.Count
	}
	shades := []ui.Color{p.clay, p.clay.Alpha(.62), p.clay.Alpha(.38), p.ink.Alpha(.22), p.ink.Alpha(.12)}
	shade := func(i int) ui.Color { return shades[min(i, len(shades)-1)] }
	ui.Box(c).Height(8).Radius(4).Clip().Label("File types").Draw(func(pt *ui.Painter, r ui.Rect) {
		x := r.X
		for i, it := range items {
			w := r.W * float32(it.Count) / float32(max(1, total))
			if i > 0 {
				x += 2
				w -= 2
			}
			pt.Fill(ui.Rect{X: x, Y: r.Y, W: max(0, w), H: r.H}, shade(i), 0)
			x += w
		}
	})
	ui.Row(c).Gap(14).Wrap().Children(func() {
		for i, it := range items {
			ui.Row(c.Key(it.Label)).Gap(6).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Background(shade(i))
				ui.Text(c, it.Label).FontSize(12)
				ui.Text(c, fmt.Sprintf("%d · %.0f%%", it.Count, float64(it.Count)/float64(max(1, total))*100)).FontSize(12).TextColor(p.text2).FontFeatures("tnum")
			})
		}
	})
}

func (d *Desktop) activity(c *ui.Context, s core.Insights) {
	p := &d.pal
	d.card(ui.Column(c.Key("activity"))).Padding(16).Gap(14).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, "Activity").FontWeight(600).Role(ui.RoleHeading).Level(2)
			streak := fmt.Sprintf("%d day streak", s.CurrentStreak)
			ui.Row(c).Gap(5).Padding(2, 8, 2, 6).Radius(999).Background(p.claySoft).TextColor(p.clayText).Label(streak).Children(func() {
				ui.Icon(c, icFlame).Size(12, 12)
				ui.Text(c, streak).FontSize(11.5).FontWeight(600)
			})
			ui.Spacer(c)
			d.caption(c, "Longest · "+plural(s.LongestStreak, "day", "days"))
		})
		d.heatmap(c, s.Daily)
		d.hours(c, s)
	})
}

const (
	cell    = float32(11)
	cellGap = float32(3)
	gutter  = float32(28)
)

// heatmap draws a year of local calendar days. Hovering a day outlines it
// and reads out its count below, without a tooltip covering the grid.
func (d *Desktop) heatmap(c *ui.Context, daily map[string]int) {
	p := &d.pal
	now := d.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startWeek := today.AddDate(0, 0, -int(today.Weekday()))
	peak, active := 0, 0
	for _, n := range daily {
		peak = max(peak, n)
		if n > 0 {
			active++
		}
	}
	level := func(n int) ui.Color {
		if n <= 0 {
			return p.fill
		}
		f := float32(.3)
		if peak > 0 {
			f = .25 + .75*float32(n)/float32(peak)
		}
		return p.clay.Alpha(f)
	}
	weeksFor := func(w float32) int { return max(1, min(53, int((w-gutter)/(cell+cellGap)))) }
	box := ui.Box(c.Key("heatmap")).Height(18 + 7*(cell+cellGap)).Label(fmt.Sprintf("Activity heatmap: %d active days, busiest day %d images", active, peak))
	hover := ""
	if x, y, over := box.PointerPosition(); over {
		weeks := weeksFor(box.Bounds().W)
		col, row := int((x-gutter)/(cell+cellGap)), int((y-18)/(cell+cellGap))
		if x >= gutter && y >= 18 && col < weeks && row < 7 {
			day := startWeek.AddDate(0, 0, -7*(weeks-1)+col*7+row)
			if !day.After(today) {
				hover = day.Format("2006-01-02")
			}
		}
	}
	d.hoverDay = hover
	text3, ink, focus := p.text3, p.text, hover
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		weeks := weeksFor(r.W)
		start := startWeek.AddDate(0, 0, -7*(weeks-1))
		previousMonth := time.Month(0)
		for col := 0; col < weeks; col++ {
			date := start.AddDate(0, 0, col*7)
			x := r.X + gutter + float32(col)*(cell+cellGap)
			if date.Month() != previousMonth && col < weeks-2 {
				pt.Text(x, r.Y, date.Format("Jan"), 10, text3)
				previousMonth = date.Month()
			}
			for row := 0; row < 7; row++ {
				day := date.AddDate(0, 0, row)
				if day.After(today) {
					continue
				}
				key := day.Format("2006-01-02")
				rect := ui.Rect{X: x, Y: r.Y + 18 + float32(row)*(cell+cellGap), W: cell, H: cell}
				pt.Fill(rect, level(daily[key]), 2.5)
				if key == focus {
					pt.Stroke(ui.Rect{X: rect.X - 1.5, Y: rect.Y - 1.5, W: rect.W + 3, H: rect.H + 3}, ink.Alpha(.7), 3.5, 1.25)
				}
			}
		}
		for row, label := range []string{"", "Mon", "", "Wed", "", "Fri", ""} {
			if label != "" {
				pt.Text(r.X, r.Y+18+float32(row)*(cell+cellGap)+1, label, 9, text3)
			}
		}
	})
	ui.Row(c).Gap(10).Children(func() {
		if hover != "" {
			at, _ := time.ParseInLocation("2006-01-02", hover, now.Location())
			n := daily[hover]
			ui.Text(c, plural(n, "image", "images")).FontSize(12).FontWeight(600).FontFeatures("tnum")
			d.caption(c, at.Format("Monday, 2 January 2006"))
		} else {
			d.caption(c, "Each square is a local calendar day")
		}
		ui.Spacer(c)
		d.caption(c, "Less")
		for _, n := range []int{0, max(1, peak/4), max(1, peak/2), max(1, peak*3/4), max(1, peak)} {
			ui.Box(c).Size(10, 10).Radius(2.5).Background(level(n))
		}
		d.caption(c, "More")
	})
}

// hours is a 24-bar strip of when screenshots are taken, the peak in full
// clay.
func (d *Desktop) hours(c *ui.Context, s core.Insights) {
	p := &d.pal
	peakHour := s.PeakHour()
	if peakHour < 0 {
		return
	}
	peak := s.Hourly[peakHour]
	ui.Column(c.Key("hours")).Gap(6).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
		ui.Row(c).Children(func() {
			d.caption(c, "By hour of day").Grow(1)
			ui.Text(c, "Most active around "+time.Date(2026, 1, 1, peakHour, 0, 0, 0, time.Local).Format("3 PM")).FontSize(12).TextColor(p.text2)
		})
		hourly, clay, soft, text3 := s.Hourly, p.clay, p.clay.Alpha(.3), p.text3
		bars := ui.Box(c).Height(52).Label("Screenshots by hour of day")
		grow := bars.AnimateTo("grow", 1, glide)
		bars.Draw(func(pt *ui.Painter, r ui.Rect) {
			w := (r.W - 23*3) / 24
			chart := r.H - 14
			for h := 0; h < 24; h++ {
				x := r.X + float32(h)*(w+3)
				bh := max(2, chart*float32(hourly[h])/float32(max(1, peak))*grow)
				tone := soft
				if h == peakHour {
					tone = clay
				}
				pt.Fill(ui.Rect{X: x, Y: r.Y + chart - bh, W: w, H: bh}, tone, min(2, w/2))
				if h%6 == 0 {
					pt.Text(x, r.Y+chart+3, fmt.Sprintf("%02d", h), 9, text3)
				}
			}
		})
	})
}

// sparkline is twelve weeks of volume as a line over a soft clay wash,
// ending in a dot on this week.
func (d *Desktop) sparkline(c *ui.Context) {
	p := &d.pal
	values := make([]float64, 12)
	today := d.now()
	for i := 0; i < 84; i++ {
		values[11-i/7] += float64(d.snapshot.Insights.Daily[today.AddDate(0, 0, -i).Format("2006-01-02")])
	}
	peak := 1.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	clay, card := p.clay, p.card
	ui.Column(c.Key("sparkline")).Gap(6).Children(func() {
		ui.Box(c).Height(64).Label("Volume trend for the last twelve weeks").Draw(func(pt *ui.Painter, r ui.Rect) {
			x := func(i int) float32 { return r.X + 4 + (r.W-8)*float32(i)/11 }
			y := func(v float64) float32 { return r.Y + r.H - 4 - float32(v/peak)*(r.H-12) }
			line, area := &ui.Path{}, &ui.Path{}
			area.MoveTo(x(0), r.Y+r.H)
			for i, v := range values {
				if i == 0 {
					line.MoveTo(x(i), y(v))
				} else {
					// Smooth the line with midpoint curves.
					mx := (x(i-1) + x(i)) / 2
					line.CubeTo(mx, y(values[i-1]), mx, y(v), x(i), y(v))
				}
				if i == 0 {
					area.LineTo(x(i), y(v))
				} else {
					mx := (x(i-1) + x(i)) / 2
					area.CubeTo(mx, y(values[i-1]), mx, y(v), x(i), y(v))
				}
			}
			area.LineTo(x(11), r.Y+r.H).Close()
			pt.FillPathGradient(area, ui.LinearGradient{From: clay.Alpha(.2), To: clay.Alpha(0), Angle: 180, Oklab: true})
			pt.StrokePath(line, 1.75, clay)
			ex, ey := x(11), y(values[11])
			pt.FillPath((&ui.Path{}).Circle(ex, ey, 4.5), card)
			pt.FillPath((&ui.Path{}).Circle(ex, ey, 3), clay)
		})
		d.caption(c, fmt.Sprintf("Volume · last 12 weeks · %d this week", int(values[11])))
	})
}
