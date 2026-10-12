package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Insights struct {
	TotalProcessed, Successes, Errors, ThisMonth, LastMonth int
	TodayProcessed, TodaySuccesses, TodayErrors             int
	TodayAvgDuration, TodayP95Duration                      float64
	Daily                                                   map[string]int
	Hourly                                                  [24]int
	TopCategories, FileTypes                                []Labeled
	AvgDuration, P95Duration                                float64
	Fastest, Slowest                                        *DurationRecord
	CurrentStreak, LongestStreak                            int
}

func (i Insights) SuccessRate() float64 {
	if i.TotalProcessed == 0 {
		return 0
	}
	return float64(i.Successes) / float64(i.TotalProcessed)
}
func (i Insights) MonthOverMonth() *float64 {
	if i.LastMonth == 0 {
		return nil
	}
	v := float64(i.ThisMonth-i.LastMonth) / float64(i.LastMonth)
	return &v
}
func (i Insights) PeakHour() int {
	peak, hour := 0, -1
	for h, n := range i.Hourly {
		if n > peak {
			peak, hour = n, h
		}
	}
	return hour
}
func (i Insights) TimeSaved() time.Duration { return time.Duration(i.Successes) * 30 * time.Second }
func (s *Store) Insights(ctx context.Context, now time.Time, location *time.Location) (Insights, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Insights{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+historyColumns+` FROM processed_images`)
	if err != nil {
		return Insights{}, err
	}
	records, err := readRecords(rows)
	closeErr := rows.Close()
	if err != nil {
		return Insights{}, err
	}
	if closeErr != nil {
		return Insights{}, closeErr
	}
	result := ComputeInsights(records, now, location)
	if err = tx.Commit(); err != nil {
		return Insights{}, err
	}
	return result, nil
}

// ComputeInsights applies timezone rules in Go so historical daylight-saving
// offsets are independent of the host SQLite build's localtime implementation.
func ComputeInsights(files []RecentFile, now time.Time, location *time.Location) Insights {
	if location == nil {
		location = time.Local
	}
	now = now.In(location)
	today := now.Format("2006-01-02")
	thisMonth := now.Format("2006-01")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	lastMonth := monthStart.AddDate(0, -1, 0).Format("2006-01")
	result := Insights{Daily: map[string]int{}, TopCategories: []Labeled{}, FileTypes: []Labeled{}}
	var durations, todayDurations []float64
	var names []string
	types := map[string]int{}
	for _, f := range files {
		at := f.Timestamp.In(location)
		day := at.Format("2006-01-02")
		month := at.Format("2006-01")
		result.TotalProcessed++
		result.Daily[day]++
		result.Hourly[at.Hour()]++
		if month == thisMonth {
			result.ThisMonth++
		}
		if month == lastMonth {
			result.LastMonth++
		}
		if day == today {
			result.TodayProcessed++
		}
		if f.IsError {
			result.Errors++
			if day == today {
				result.TodayErrors++
			}
			continue
		}
		result.Successes++
		if day == today {
			result.TodaySuccesses++
		}
		names = append(names, f.NewName)
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.NewName), "."))
		if ext == "" {
			ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(f.OriginalName), "."))
		}
		if ext != "" {
			types[ext]++
		}
		if f.Duration > 0 {
			durations = append(durations, f.Duration)
			if day == today {
				todayDurations = append(todayDurations, f.Duration)
			}
			if result.Fastest == nil || f.Duration < result.Fastest.Duration {
				result.Fastest = &DurationRecord{f.NewName, f.Duration}
			}
			if result.Slowest == nil || f.Duration > result.Slowest.Duration {
				result.Slowest = &DurationRecord{f.NewName, f.Duration}
			}
		}
	}
	result.AvgDuration, result.P95Duration = DurationStats(durations)
	result.TodayAvgDuration, result.TodayP95Duration = DurationStats(todayDurations)
	result.TopCategories = TopNouns(names, 6)
	result.FileTypes = ranked(types, 5)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if result.Daily[day.Format("2006-01-02")] == 0 {
		day = day.AddDate(0, 0, -1)
	}
	for result.Daily[day.Format("2006-01-02")] > 0 {
		result.CurrentStreak++
		day = day.AddDate(0, 0, -1)
	}
	days := make([]string, 0, len(result.Daily))
	for d := range result.Daily {
		days = append(days, d)
	}
	sort.Strings(days)
	run := 0
	var previous time.Time
	for _, d := range days {
		at, err := time.ParseInLocation("2006-01-02", d, location)
		if err != nil {
			continue
		}
		if !previous.IsZero() && previous.AddDate(0, 0, 1).Equal(at) {
			run++
		} else {
			run = 1
		}
		if run > result.LongestStreak {
			result.LongestStreak = run
		}
		previous = at
	}
	return result
}
func DurationStats(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	sum := 0.0
	for _, v := range ordered {
		sum += v
	}
	index := int(float64(len(ordered)-1)*.95 + .5)
	return sum / float64(len(ordered)), ordered[index]
}
