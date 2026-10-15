package core

import (
	"context"
	"database/sql"
	"fmt"
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
	rows, err := tx.QueryContext(ctx, `SELECT CAST(timestamp AS TEXT), originalName, newName, duration, isError FROM processed_images`)
	if err != nil {
		return Insights{}, err
	}
	defer rows.Close()
	aggregate := newInsightsAccumulator(now, location)
	for rows.Next() {
		var raw, original, name string
		var duration float64
		var isError bool
		if err := rows.Scan(&raw, &original, &name, &duration, &isError); err != nil {
			return Insights{}, err
		}
		at, err := parseTimestamp(raw)
		if err != nil {
			return Insights{}, fmt.Errorf("history analytics: %w", err)
		}
		aggregate.add(at, original, name, duration, isError)
	}
	if err := rows.Err(); err != nil {
		return Insights{}, err
	}
	if err := rows.Close(); err != nil {
		return Insights{}, err
	}
	// Release the single database connection before sorting durations and rankings.
	if err := tx.Commit(); err != nil {
		return Insights{}, err
	}
	return aggregate.finish(), nil
}

// ComputeInsights applies timezone rules in Go so historical daylight-saving
// offsets are independent of the host SQLite build's localtime implementation.
func ComputeInsights(files []RecentFile, now time.Time, location *time.Location) Insights {
	aggregate := newInsightsAccumulator(now, location)
	for _, f := range files {
		aggregate.add(f.Timestamp, f.OriginalName, f.NewName, f.Duration, f.IsError)
	}
	return aggregate.finish()
}

type insightsAccumulator struct {
	now                         time.Time
	location                    *time.Location
	today, thisMonth, lastMonth string
	result                      Insights
	durations, todayDurations   []float64
	types, nouns                map[string]int
}

func newInsightsAccumulator(now time.Time, location *time.Location) *insightsAccumulator {
	if location == nil {
		location = time.Local
	}
	now = now.In(location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	return &insightsAccumulator{
		now:       now,
		location:  location,
		today:     now.Format("2006-01-02"),
		thisMonth: now.Format("2006-01"),
		lastMonth: monthStart.AddDate(0, -1, 0).Format("2006-01"),
		result:    Insights{Daily: map[string]int{}, TopCategories: []Labeled{}, FileTypes: []Labeled{}},
		types:     map[string]int{},
		nouns:     map[string]int{},
	}
}
func (a *insightsAccumulator) add(timestamp time.Time, original, name string, duration float64, isError bool) {
	at := timestamp.In(a.location)
	day, month := at.Format("2006-01-02"), at.Format("2006-01")
	result := &a.result
	result.TotalProcessed++
	result.Daily[day]++
	result.Hourly[at.Hour()]++
	if month == a.thisMonth {
		result.ThisMonth++
	}
	if month == a.lastMonth {
		result.LastMonth++
	}
	if day == a.today {
		result.TodayProcessed++
	}
	if isError {
		result.Errors++
		if day == a.today {
			result.TodayErrors++
		}
		return
	}
	result.Successes++
	if day == a.today {
		result.TodaySuccesses++
	}
	countNouns(a.nouns, name)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "" {
		ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(original), "."))
	}
	if ext != "" {
		a.types[ext]++
	}
	if duration > 0 {
		a.durations = append(a.durations, duration)
		if day == a.today {
			a.todayDurations = append(a.todayDurations, duration)
		}
		if result.Fastest == nil || duration < result.Fastest.Duration {
			result.Fastest = &DurationRecord{name, duration}
		}
		if result.Slowest == nil || duration > result.Slowest.Duration {
			result.Slowest = &DurationRecord{name, duration}
		}
	}
}
func (a *insightsAccumulator) finish() Insights {
	result := a.result
	result.AvgDuration, result.P95Duration = ownedDurationStats(a.durations)
	result.TodayAvgDuration, result.TodayP95Duration = ownedDurationStats(a.todayDurations)
	result.TopCategories = ranked(a.nouns, 6)
	result.FileTypes = ranked(a.types, 5)
	now, location := a.now, a.location
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

// DurationStats leaves caller-owned observations in their original order.
func DurationStats(values []float64) (float64, float64) {
	return ownedDurationStats(append([]float64(nil), values...))
}
func ownedDurationStats(ordered []float64) (float64, float64) {
	if len(ordered) == 0 {
		return 0, 0
	}
	sort.Float64s(ordered)
	sum := 0.0
	for _, v := range ordered {
		sum += v
	}
	index := int(float64(len(ordered)-1)*.95 + .5)
	return sum / float64(len(ordered)), ordered[index]
}
