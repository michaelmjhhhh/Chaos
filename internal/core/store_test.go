package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "history.sqlite"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestGRDBSchemaCompatibilityAndMutation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE grdb_migrations(identifier TEXT NOT NULL PRIMARY KEY);INSERT INTO grdb_migrations VALUES('v1_createProcessedImages');CREATE TABLE processed_images(id TEXT PRIMARY KEY,originalName TEXT NOT NULL,newName TEXT NOT NULL,path TEXT NOT NULL,sourcePath TEXT NOT NULL,timestamp DATETIME NOT NULL,duration DOUBLE NOT NULL,isError BOOLEAN NOT NULL,errorMessage TEXT,ext TEXT NOT NULL);INSERT INTO processed_images VALUES('SWIFT-ID','Screenshot.png','terminal.png','/out/terminal.png','/in/Screenshot.png','2026-02-01 12:04:05.125',1.25,0,NULL,'png')`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	files, err := s.Recent(context.Background(), 500)
	if err != nil || len(files) != 1 {
		t.Fatalf("%v %v", files, err)
	}
	f := files[0]
	if f.ID != "SWIFT-ID" || f.Timestamp.UnixMilli() != time.Date(2026, 2, 1, 12, 4, 5, 125000000, time.UTC).UnixMilli() {
		t.Fatal(f)
	}
	if err = s.Insert(context.Background(), f); err != nil {
		t.Fatal("idempotent insert", err)
	}
	if err = s.Update(context.Background(), f.ID, "renamed.jpg", "/out/renamed.jpg"); err != nil {
		t.Fatal(err)
	}
	var ext, migration string
	s.db.QueryRow(`SELECT ext FROM processed_images WHERE id='SWIFT-ID'`).Scan(&ext)
	s.db.QueryRow(`SELECT identifier FROM grdb_migrations`).Scan(&migration)
	if ext != "jpg" || migration != "v1_createProcessedImages" {
		t.Fatal("compatibility metadata damaged")
	}
	if err = s.Delete(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	files, _ = s.Recent(context.Background(), 500)
	if len(files) != 0 {
		t.Fatal("delete failed")
	}
}
func TestLegacySwiftEpochImportMergesAndKeepsBackups(t *testing.T) {
	dir := t.TempDir()
	path, legacy := filepath.Join(dir, "history.sqlite"), filepath.Join(dir, "history.json")
	s, err := OpenStore(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Insert(context.Background(), RecentFile{ID: "existing", Timestamp: time.Now()}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	writeTestFile(t, legacy, []byte(`[{"id":"swift-success","originalName":"Screenshot.png","newName":"terminal.png","path":"/out/terminal.png","sourcePath":"/in/Screenshot.png","timestamp":-978307100.25,"duration":1.25,"result":{"success":{}}},{"id":"swift-failure","originalName":"Dropped.jpg","newName":"","path":"","sourcePath":"/in/Dropped.jpg","timestamp":-978307000,"duration":0,"result":{"error":{"_0":"API timeout"}}}]`))
	writeTestFile(t, legacy+".migrated", []byte("older-backup"))
	s, err = OpenStore(path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	files, err := s.Recent(context.Background(), 10)
	s.Close()
	if err != nil || len(files) != 3 {
		t.Fatalf("%v %v", files, err)
	}
	seen := map[string]RecentFile{}
	for _, f := range files {
		seen[f.ID] = f
	}
	if seen["swift-success"].Timestamp.UnixMilli() != 99750 {
		t.Fatal("Swift epoch conversion", seen["swift-success"].Timestamp)
	}
	if !seen["swift-failure"].IsError || seen["swift-failure"].ErrorMessage != "API timeout" {
		t.Fatal("enum decode lost error")
	}
	old, _ := os.ReadFile(legacy + ".migrated")
	if string(old) != "older-backup" {
		t.Fatal("older backup overwritten")
	}
	if _, err = os.Stat(legacy + ".migrated.2"); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	files, _ = s.Recent(context.Background(), 10)
	if len(files) != 3 {
		t.Fatal("migration not idempotent")
	}
}
func TestCorruptLegacyNeverArchived(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "history.json")
	writeTestFile(t, legacy, []byte("corrupt"))
	s, err := OpenStore(filepath.Join(dir, "history.sqlite"), legacy)
	if s != nil {
		s.Close()
	}
	if err == nil {
		t.Fatal("corruption ignored")
	}
	if _, err = os.Stat(legacy); err != nil {
		t.Fatal("original removed")
	}
	if _, err = os.Stat(legacy + ".migrated"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("corrupt data archived")
	}
}
func TestFullHistoryBeyondRecentLimitAndDSTAnalytics(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, loc)
	for n := 0; n < 510; n++ {
		f := RecentFile{ID: fmt.Sprint(n), OriginalName: "Screenshot.png", NewName: "terminal-log.png", Timestamp: now, Duration: float64(n%3 + 1)}
		if err = s.Insert(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	for n, day := range []int{8, 7} {
		f := RecentFile{ID: fmt.Sprintf("prior%d", n), NewName: "browser.jpg", Timestamp: time.Date(2026, 3, day, 14, 0, 0, 0, loc), Duration: 4}
		if err = s.Insert(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Insert(ctx, RecentFile{ID: "error", OriginalName: "failed.webp", IsError: true, ErrorMessage: "offline", Timestamp: now}); err != nil {
		t.Fatal(err)
	}
	files, err := s.Recent(ctx, 500)
	if err != nil || len(files) != 500 {
		t.Fatal(len(files), err)
	}
	insights, err := s.Insights(ctx, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if insights.TotalProcessed != 513 || insights.Successes != 512 || insights.Errors != 1 || insights.TodayProcessed != 511 || insights.TodaySuccesses != 510 || insights.CurrentStreak != 3 || insights.LongestStreak != 3 {
		t.Fatalf("wrong totals %+v", insights)
	}
	if insights.PeakHour() != 12 || insights.AvgDuration <= 0 || insights.TodayAvgDuration != 2 || insights.TodayP95Duration != 3 {
		t.Fatal(insights)
	}
	if insights.FileTypes[0].Label != "png" || insights.FileTypes[0].Count != 510 {
		t.Fatal(insights.FileTypes)
	}
	if insights.TimeSaved() != 512*30*time.Second {
		t.Fatal("saved time")
	}
}
func TestAnalyticsCalendarBoundsAndEmpty(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 2, 1, 1, 0, 0, 0, loc)
	files := []RecentFile{{Timestamp: time.Date(2026, 1, 31, 17, 0, 0, 0, time.UTC), Duration: 1}, {Timestamp: time.Date(2026, 1, 31, 15, 0, 0, 0, time.UTC), Duration: 2}, {Timestamp: time.Date(2026, 3, 1, 1, 0, 0, 0, loc), Duration: 3}}
	insight := ComputeInsights(files, now, loc)
	if insight.ThisMonth != 1 || insight.LastMonth != 1 || insight.CurrentStreak != 2 {
		t.Fatal(insight)
	}
	if delta := insight.MonthOverMonth(); delta == nil || *delta != 0 {
		t.Fatal(delta)
	}
	empty := ComputeInsights(nil, now, loc)
	if empty.SuccessRate() != 0 || empty.PeakHour() != -1 || empty.MonthOverMonth() != nil || empty.CurrentStreak != 0 {
		t.Fatal(empty)
	}
}

func TestLegacyDateBoundariesAndNegativeFraction(t *testing.T) {
	for _, tc := range []struct{ seconds, want string }{{"-63145526400", "0000-01-01T00:00:00Z"}, {"252423993599.75", "9999-12-31T23:59:59.75Z"}, {"-0.25", "2000-12-31T23:59:59.75Z"}, {"-978307200.25", "1969-12-31T23:59:59.75Z"}} {
		t.Run(tc.seconds, func(t *testing.T) {
			dir := t.TempDir()
			legacy := filepath.Join(dir, "history.json")
			writeTestFile(t, legacy, []byte(fmt.Sprintf(`[{"id":"valid","timestamp":%s,"result":{"success":{}}}]`, tc.seconds)))
			s, err := OpenStore(filepath.Join(dir, "history.sqlite"), legacy)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			got, err := s.Recent(context.Background(), 10)
			if err != nil || len(got) != 1 {
				t.Fatal(got, err)
			}
			if at := got[0].Timestamp.Format(time.RFC3339Nano); at != tc.want {
				t.Fatalf("got %s want %s", at, tc.want)
			}
		})
	}
}
func TestInsightsFallBackCalendarAndDurationDistribution(t *testing.T) {
	s := openTestStore(t)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for n, tc := range []struct {
		at, name string
		duration float64
		failed   bool
	}{
		{"2026-11-01T03:30:00Z", "terminal-first.png", 1, false},
		{"2026-11-01T05:30:00Z", "terminal-fast.jpg", 2, false},
		{"2026-11-01T06:30:00Z", "terminal-slow.jpg", 20, false},
		{"2026-11-01T07:30:00Z", "ignored-failure.webp", 999, true},
		{"2026-10-30T16:00:00Z", "browser.png", 0, false},
		{"2026-09-30T16:00:00Z", "browser.png", 4, false},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Insert(context.Background(), RecentFile{ID: fmt.Sprint(n), Timestamp: at, NewName: tc.name, Duration: tc.duration, IsError: tc.failed}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Insights(context.Background(), time.Date(2026, 11, 1, 12, 0, 0, 0, loc), loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalProcessed != 6 || got.Successes != 5 || got.Errors != 1 || got.TodayProcessed != 3 || got.TodaySuccesses != 2 || got.TodayErrors != 1 || got.ThisMonth != 3 || got.LastMonth != 2 {
		t.Fatalf("calendar totals: %+v", got)
	}
	if got.CurrentStreak != 3 || got.LongestStreak != 3 || got.Hourly[1] != 2 || got.Hourly[2] != 1 || got.Daily["2026-10-31"] != 1 {
		t.Fatalf("DST calendar: %+v", got)
	}
	if got.AvgDuration != 6.75 || got.P95Duration != 20 || got.TodayAvgDuration != 11 || got.TodayP95Duration != 20 || got.Fastest.Name != "terminal-first.png" || got.Slowest.Name != "terminal-slow.jpg" {
		t.Fatalf("duration stats: %+v", got)
	}
	if got.TopCategories[0] != (Labeled{"terminal", 3}) || got.TopCategories[1] != (Labeled{"browser", 2}) || got.FileTypes[0] != (Labeled{"png", 3}) || got.FileTypes[1] != (Labeled{"jpg", 2}) {
		t.Fatal("category/type aggregation", got.TopCategories, got.FileTypes)
	}
}
func TestDurationStatsPercentileAndInputOwnership(t *testing.T) {
	values := []float64{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	avg, p95 := DurationStats(values)
	if avg != 10.5 || p95 != 19 || values[0] != 20 || values[19] != 1 {
		t.Fatal(avg, p95, values)
	}
}
