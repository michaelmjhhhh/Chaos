package core

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// Exercise full retained archives, including history fields the UI needs but
// analytics does not. Database creation and durable inserts are outside timing.
func BenchmarkStoreInsights(b *testing.B) {
	for _, count := range []int{10_000, 100_000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			store, err := OpenStore(filepath.Join(b.TempDir(), "history.sqlite"), "")
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := store.Close(); err != nil {
					b.Error(err)
				}
			})
			tx, err := store.db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			defer tx.Rollback()
			statement, err := tx.Prepare(insertSQL)
			if err != nil {
				b.Fatal(err)
			}
			now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
			for i := 0; i < count; i++ {
				record := RecentFile{
					ID:           fmt.Sprint(i),
					OriginalName: fmt.Sprintf("Screenshot %06d.png", i),
					NewName:      "terminal-git-log.png",
					Path:         fmt.Sprintf("/Users/example/Pictures/organized/2026/terminal-git-log-%06d.png", i),
					SourcePath:   fmt.Sprintf("/Users/example/Desktop/Screenshot %06d.png", i),
					Timestamp:    now.Add(-time.Duration(i) * time.Hour),
					Duration:     float64(i%20+1) / 2,
					IsError:      i%25 == 0,
					ErrorMessage: "The naming service was temporarily unavailable.",
				}
				if _, err := statement.Exec(recordArgs(record)...); err != nil {
					b.Fatal(err)
				}
			}
			if err := statement.Close(); err != nil {
				b.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				insights, err := store.Insights(context.Background(), now, time.UTC)
				if err != nil || insights.TotalProcessed != count {
					b.Fatalf("wrong archive result: %d, %v", insights.TotalProcessed, err)
				}
			}
		})
	}
}
