package desktop

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (d *Desktop) insights(c *ui.Context) {
	s := d.snapshot.Insights
	ui.Scroll(c.Key("insights-scroll")).Grow(1).Padding(24).Gap(20).Children(func() {
		heading(c, "Insights", "The almanac of your screenshot archive.")
		if d.snapshot.StorageError != "" {
			ui.Text(c, d.snapshot.StorageError).TextColor(c.Theme().Danger)
		}
		if s.TotalProcessed == 0 {
			ui.Text(c, "No images filed yet").Font("Georgia").FontSize(24).Padding(20)
			small(c, "Insights fills in as Chaos renames and files your screenshots.")
			return
		}
		ui.Row(c).Gap(16).Wrap().Children(func() {
			ui.Column(c).Grow(1).MinWidth(170).Children(func() {
				section(c, "Images processed", func() {
					metric(c, fmt.Sprint(s.TotalProcessed), archiveSince(s.Daily))
					small(c, fmt.Sprintf("%d this month · %d this week", s.ThisMonth, d.weekCount()))
					if delta := s.MonthOverMonth(); delta != nil {
						small(c, fmt.Sprintf("%+.0f%% vs last month", *delta*100))
					}
				})
			})
			ui.Column(c).Grow(1).MinWidth(170).Children(func() {
				section(c, "Success rate", func() {
					metric(c, fmt.Sprintf("%.0f%%", s.SuccessRate()*100), "FILED CLEANLY")
					small(c, fmt.Sprintf("%d filed · %d errors", s.Successes, s.Errors))
				})
			})
			ui.Column(c).Grow(1).MinWidth(170).Children(func() {
				section(c, "Time saved", func() {
					metric(c, formatTimeSaved(s.TimeSaved()), "ESTIMATED")
					small(c, "≈ 30s of manual filing per image")
				})
			})
		})
		section(c, "Activity", func() {
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Text(c, fmt.Sprintf("%d day streak", s.CurrentStreak)).Font("Georgia").FontSize(20).Grow(1)
				small(c, fmt.Sprintf("Longest · %d days", s.LongestStreak))
			})
			d.heatmap(c, s.Daily)
			if hour := s.PeakHour(); hour >= 0 {
				small(c, fmt.Sprintf("Most active around %s", time.Date(2026, 1, 1, hour, 0, 0, 0, time.Local).Format("3 PM")))
			}
		})
		width, _ := c.Size()
		var panels ui.Element
		if width >= 780 {
			panels = ui.Row(c).Gap(20).AlignItems(ui.Start)
		} else {
			panels = ui.Column(c).Gap(20)
		}
		panels.Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				section(c, "What you capture", func() {
					small(c, "TOP CATEGORIES")
					breakdown(c, s.TopCategories, false)
					ui.Divider(c)
					small(c, "FILE TYPES")
					breakdown(c, s.FileTypes, true)
				})
			})
			ui.Column(c).Grow(1).Children(func() {
				section(c, "Performance", func() {
					ui.Row(c).Gap(12).Children(func() {
						metric(c, duration(s.AvgDuration), "Average per image")
						metric(c, duration(s.P95Duration), "95th percentile")
					})
					ui.Divider(c)
					if s.Fastest != nil {
						small(c, "Fastest · "+duration(s.Fastest.Duration))
						ui.Text(c, s.Fastest.Name).Font("monospace").FontSize(11).SingleLine()
					}
					if s.Slowest != nil {
						small(c, "Slowest · "+duration(s.Slowest.Duration))
						ui.Text(c, s.Slowest.Name).Font("monospace").FontSize(11).SingleLine()
					}
					if s.AvgDuration <= 0 {
						small(c, "No timing data yet.")
					}
					d.sparkline(c)
				})
			})
		})
	})
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
		return "LIFETIME"
	}
	return "LIFETIME · SINCE " + strings.ToUpper(earliest.Format("Jan 2006"))
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
func breakdown(c *ui.Context, items []core.Labeled, percent bool) {
	if len(items) == 0 {
		small(c, "No filed images yet.")
		return
	}
	maxCount, total := 0, 0
	for _, item := range items {
		maxCount = max(maxCount, item.Count)
		total += item.Count
	}
	for _, item := range items {
		ui.Column(c.Key(item.Label)).Gap(4).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, item.Label).Grow(1).SingleLine()
				label := fmt.Sprint(item.Count)
				if percent && total > 0 {
					label = fmt.Sprintf("%d · %.0f%%", item.Count, float64(item.Count)/float64(total)*100)
				}
				small(c, label)
			})
			ratio := float32(item.Count) / float32(max(1, maxCount))
			color := c.Theme().Accent
			ui.Box(c).Height(5).Background(c.Theme().Border.Alpha(.4)).Radius(3).Draw(func(p *ui.Painter, r ui.Rect) { p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * ratio, H: r.H}, color, 3) })
		})
	}
}
func (d *Desktop) heatmap(c *ui.Context, daily map[string]int) {
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
	accent, border, muted := c.Theme().Accent, c.Theme().Border, c.Theme().TextMuted
	ui.Box(c).Height(146).Label(fmt.Sprintf("Activity heatmap: %d active days, busiest day %d images", active, peak)).Draw(func(p *ui.Painter, r ui.Rect) {
		const cell = float32(11)
		const gap = float32(3)
		weeks := max(1, min(53, int((r.W-30)/(cell+gap))))
		start := startWeek.AddDate(0, 0, -7*(weeks-1))
		previousMonth := time.Month(0)
		for col := 0; col < weeks; col++ {
			date := start.AddDate(0, 0, col*7)
			x := r.X + 30 + float32(col)*(cell+gap)
			if date.Month() != previousMonth {
				p.Text(x, r.Y, date.Format("Jan"), 10, muted)
				previousMonth = date.Month()
			}
			for row := 0; row < 7; row++ {
				day := date.AddDate(0, 0, row)
				if day.After(today) {
					continue
				}
				n := daily[day.Format("2006-01-02")]
				color := border.Alpha(.4)
				if n > 0 {
					level := float32(.3)
					if peak > 0 {
						level = .25 + .75*float32(n)/float32(peak)
					}
					color = accent.Alpha(level)
				}
				p.Fill(ui.Rect{X: x, Y: r.Y + 20 + float32(row)*(cell+gap), W: cell, H: cell}, color, 2)
			}
		}
		for row, label := range []string{"", "Mon", "", "Wed", "", "Fri", ""} {
			if label != "" {
				p.Text(r.X, r.Y+20+float32(row)*(cell+gap), label, 9, muted)
			}
		}
	})
	small(c, "Less  ░ ▒ ▓ ▉  More · each square is a local calendar day")
}
func (d *Desktop) sparkline(c *ui.Context) {
	values := make([]float64, 12)
	today := d.now()
	for i := 0; i < 84; i++ {
		values[11-i/7] += float64(d.snapshot.Insights.Daily[today.AddDate(0, 0, -i).Format("2006-01-02")])
	}
	peak := 1.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	accent := c.Theme().Accent
	ui.Box(c).Height(70).Label("Volume trend for the last twelve weeks").Draw(func(p *ui.Painter, r ui.Rect) {
		path := &ui.Path{}
		for i, v := range values {
			x := r.X + r.W*float32(i)/11
			y := r.Y + r.H - 6 - float32(v/peak)*(r.H-12)
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		p.StrokePath(path, 2, accent)
	})
	small(c, fmt.Sprintf("Volume · last 12 weeks · %d this week", int(values[11])))
}
