package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type captureRequests struct{ requests []*http.Request }

func (c *captureRequests) Do(r *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, r)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
}
func TestUnknownProviderCannotSendCredentials(t *testing.T) {
	for _, saved := range []bool{false, true} {
		t.Run(fmt.Sprint("saved=", saved), func(t *testing.T) {
			dir := t.TempDir()
			cfg := Config{Provider: "openai-compatibl", APIKey: "unknown-secret", Model: "private-model", BaseURL: "https://private.invalid/v1", ProviderKeys: map[string]string{"openai": "saved-openai", "siliconrouter": "saved-router"}}
			if saved {
				path := filepath.Join(dir, "config.json")
				if err := SaveConfig(path, cfg); err != nil {
					t.Fatal(err)
				}
				var err error
				cfg, err = LoadConfig(path, "")
				if err != nil {
					t.Fatal(err)
				}
			}
			resolved := cfg.Resolve(dir)
			if err := resolved.Validate(); !errors.Is(err, ErrUnsupportedProvider) {
				t.Error("unsupported provider passed validation")
			}
			// Even a caller supplying an otherwise valid endpoint must not bypass identity validation.
			resolved.BaseURL = "https://api.example.invalid/v1"
			resolved.APIKey = "unknown-secret"
			capture := &captureRequests{}
			client := &Client{HTTP: capture}
			if err := client.CheckHealth(context.Background(), resolved); !errors.Is(err, ErrUnsupportedProvider) {
				t.Error("health accepted unknown provider")
			}
			if _, err := client.GenerateSlug(context.Background(), PreparedImage{"eA==", "image/png"}, resolved); !errors.Is(err, ErrUnsupportedProvider) {
				t.Error("generation accepted unknown provider")
			}
			if len(capture.requests) != 0 {
				t.Errorf("sent %d requests with unknown identity", len(capture.requests))
			}
			for _, next := range []string{"siliconrouter", "openai"} {
				switched := cfg
				switched.SetProvider(next)
				want := "saved-openai"
				if next == "siliconrouter" {
					want = "saved-router"
				}
				if switched.Provider != next || switched.APIKey != want || switched.Model != "" || switched.BaseURL != "" {
					t.Errorf("switch leaked prior configuration: %+v", switched)
				}
				sent := &captureRequests{}
				if err := (&Client{HTTP: sent}).CheckHealth(context.Background(), switched.Resolve(dir)); err != nil {
					t.Fatal(err)
				}
				if len(sent.requests) != 1 || sent.requests[0].Header.Get("Authorization") != "Bearer "+want {
					t.Fatal("wrong restored credential reached transport")
				}
				withoutSaved := cfg
				withoutSaved.ProviderKeys = nil
				withoutSaved.SetProvider(next)
				sent = &captureRequests{}
				if err := (&Client{HTTP: sent}).CheckHealth(context.Background(), withoutSaved.Resolve(dir)); !errors.Is(err, ErrMissingAPIKey) || len(sent.requests) != 0 {
					t.Fatal("unknown key escaped through selection without a saved key", err)
				}

			}
		})
	}
}
func TestEmptyProviderDefaultsAndUnknownSelection(t *testing.T) {
	cfg := Config{APIKey: "default-key"}
	r := cfg.Resolve(t.TempDir())
	if r.Provider.ID != "siliconrouter" || r.APIKey != "default-key" || r.Validate() != nil {
		t.Fatalf("empty provider defaults: %+v", r)
	}
	cfg.SetProvider("unsupported")
	capture := &captureRequests{}
	if err := (&Client{HTTP: capture}).CheckHealth(context.Background(), cfg.Resolve(t.TempDir())); err == nil || len(capture.requests) != 0 {
		t.Fatal("unknown selection sent request", err)
	}
	cfg.SetProvider("siliconrouter")
	if cfg.APIKey != "default-key" {
		t.Fatal("known saved key lost")
	}
}
func TestLegacyOutOfRangeDatesPreserveSourceAndHistory(t *testing.T) {
	for _, date := range []string{"1e20", "-1e20", "1e300", "-1e300", "252423993600", "-63145526401"} {
		t.Run(date, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "history.sqlite")
			legacy := filepath.Join(dir, "history.json")
			s, err := OpenStore(path, "")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Insert(context.Background(), RecentFile{ID: "existing", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
				t.Fatal(err)
			}
			s.Close()
			original := []byte(fmt.Sprintf(`[{"id":"valid","timestamp":-0.25,"result":{"success":{}}},{"id":"invalid","timestamp":%s,"result":{"success":{}}}]`, date))
			writeTestFile(t, legacy, original)
			imported, err := OpenStore(path, legacy)
			if imported != nil {
				imported.Close()
			}
			if err == nil {
				t.Error("out-of-range timestamp accepted")
			}
			b, err := os.ReadFile(legacy)
			if err != nil || string(b) != string(original) {
				t.Error("legacy source not preserved", err)
			}
			if _, err = os.Stat(legacy + ".migrated"); !errors.Is(err, os.ErrNotExist) {
				t.Error("invalid input archived", err)
			}
			s, err = OpenStore(path, "")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var count int
			if err = s.db.QueryRow(`SELECT COUNT(*) FROM processed_images`).Scan(&count); err != nil || count != 1 {
				t.Errorf("partial import committed: %d %v", count, err)
			}
		})
	}
}
func BenchmarkStoreInsights(b *testing.B) {
	for _, count := range []int{1000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s, err := OpenStore(filepath.Join(b.TempDir(), "history.sqlite"), "")
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
			tx, err := s.db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			stmt, err := tx.Prepare(insertSQL)
			if err != nil {
				b.Fatal(err)
			}
			for n := 0; n < count; n++ {
				f := RecentFile{ID: fmt.Sprint(n), OriginalName: "Screenshot.png", NewName: fmt.Sprintf("terminal-project-%d.png", n%30), Path: "/synthetic/output/terminal-project.png", SourcePath: "/synthetic/screenshots/Screenshot.png", Timestamp: now.Add(-time.Duration(n%1000) * time.Hour), Duration: float64(n%200+1) / 10, IsError: n%17 == 0, ErrorMessage: "synthetic failure"}
				if _, err = stmt.Exec(recordArgs(f)...); err != nil {
					b.Fatal(err)
				}
			}
			stmt.Close()
			if err = tx.Commit(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				got, err := s.Insights(context.Background(), now, time.UTC)
				if err != nil || got.TotalProcessed != count {
					b.Fatal(got.TotalProcessed, err)
				}
			}
		})
	}
}
