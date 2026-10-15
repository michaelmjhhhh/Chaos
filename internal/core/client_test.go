package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientVisionAndHealthRequestContracts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	sampleImage(t, path, 32, 32)
	prepared, err := PrepareImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	type request struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Stream    *bool  `json:"stream"`
		Messages  []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	for _, tc := range []struct {
		name, language, customPrompt, systemContains, user string
	}{
		{"English", "en", "", "English", "Return only the filename slug."},
		{"Chinese", "zh", "", "Simplified Chinese", "Return only the filename slug in Simplified Chinese."},
		{"custom prompt", "zh", "Use my naming rules", "Use my naming rules", "Return only the filename slug."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan request, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/chat/completions" {
					t.Errorf("wrong request destination: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer unit-key" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("request authentication or content type missing")
				}
				var body request
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requests <- body
				fmt.Fprint(w, `{"choices":[{"message":{"content":"  中文名字  "}}]}`)
			}))
			defer server.Close()
			cfg := testConfig(server.URL+"/api", filepath.Join(t.TempDir(), "out"))
			cfg.CustomPrompt, cfg.Language = tc.customPrompt, tc.language
			client := &Client{HTTP: server.Client()}
			slug, err := client.GenerateSlug(context.Background(), prepared, cfg)
			if err != nil || slug != "中文名字" {
				t.Fatalf("response slug = %q, %v", slug, err)
			}
			vision := <-requests
			if vision.Model != cfg.Model || vision.MaxTokens != 32 || vision.Stream == nil || *vision.Stream {
				t.Fatalf("wrong naming request parameters: %+v", vision)
			}
			if len(vision.Messages) != 2 || vision.Messages[0].Role != "system" || vision.Messages[1].Role != "user" {
				t.Fatalf("wrong naming conversation: %+v", vision.Messages)
			}
			var system string
			if err := json.Unmarshal(vision.Messages[0].Content, &system); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(system, tc.systemContains) || (tc.customPrompt != "" && system != tc.customPrompt) {
				t.Fatalf("wrong naming instructions: %q", system)
			}
			var parts []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			}
			if err := json.Unmarshal(vision.Messages[1].Content, &parts); err != nil {
				t.Fatal(err)
			}
			if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != tc.user || parts[1].Type != "image_url" {
				t.Fatalf("wrong vision content: %+v", parts)
			}
			if parts[1].ImageURL.URL != "data:image/jpeg;base64,"+prepared.Base64 {
				t.Fatal("request lost or changed the prepared image payload")
			}
			if err := client.CheckHealth(context.Background(), cfg); err != nil {
				t.Fatal(err)
			}
			health := <-requests
			if health.Model != cfg.Model || health.MaxTokens != 8 || health.Stream == nil || *health.Stream || len(health.Messages) != 1 || health.Messages[0].Role != "user" {
				t.Fatalf("wrong health request parameters: %+v", health)
			}
			var healthPrompt string
			if err := json.Unmarshal(health.Messages[0].Content, &healthPrompt); err != nil {
				t.Fatal(err)
			}
			if healthPrompt != "Reply with only: ok" {
				t.Fatalf("health request includes unexpected content: %q", healthPrompt)
			}
		})
	}
}

func TestParseSlugResponseContracts(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"text", `{"choices":[{"message":{"content":"  terminal-log  "}}]}`, "terminal-log"},
		{"custom endpoint compatibility", `{"candidates":[{"content":{"parts":[{"text":" "},{"text":"  terminal-log  "}]}}]}`, "terminal-log"},
		{"parts", `{"choices":[{"message":{"content":[{"type":"text","text":" "},{"type":"image_url"},{"type":"text","text":" 中文名字 "}]}}]}`, "中文名字"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSlug([]byte(tc.input))
			if err != nil || got != tc.want {
				t.Fatalf("slug = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, input := range []string{`{}`, `{"choices":[{"message":{"content":" "}}]}`, `{"choices":[{"message":{"content":null}}]}`} {
		if _, err := ParseSlug([]byte(input)); !errors.Is(err, ErrEmptyProviderResponse) {
			t.Fatalf("expected empty response error for %s: %v", input, err)
		}
	}
	for _, input := range []string{`{"error":{"message":"upstream failure"}}`, `{"error":"upstream failure"}`} {
		if _, err := ParseSlug([]byte(input)); err == nil || err.Error() != "upstream failure" {
			t.Fatalf("provider error lost: %v", err)
		}
	}
	if _, err := ParseSlug([]byte("broken")); err == nil {
		t.Fatal("malformed provider JSON accepted")
	}
}

func TestClientDoesNotForwardCredentialsThroughRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"redirected"}}]}`)
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	cfg := testConfig(server.URL, filepath.Join(t.TempDir(), "out"))
	err := (&Client{}).CheckHealth(context.Background(), cfg)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusTemporaryRedirect || redirected.Load() != 0 {
		t.Fatalf("redirect was followed or hidden: %v; requests=%d", err, redirected.Load())
	}
}

func TestFriendlyUsesErrorIdentityInsteadOfProviderProse(t *testing.T) {
	for _, tc := range []struct {
		err             error
		action          RecoveryAction
		messageContains string
	}{
		{ErrUnknownProvider, RecoverySettings, "Settings"},
		{ErrMissingAPIKey, RecoverySettings, "Settings"},
		{ErrInvalidProviderURL, RecoverySettings, "Settings"},
		{ErrImageNotStable, RecoveryRetry, "still being saved"},
		{ErrEmptyProviderResponse, RecoveryRetry, "didn't return a name"},
	} {
		err := fmt.Errorf("processing failed: %w", tc.err)
		got := Friendly(err, "openai")
		if got.Action != tc.action || !strings.Contains(got.Message, tc.messageContains) {
			t.Fatalf("wrong recovery for %v: %+v", err, got)
		}
	}
	message := "provider stability check failed; API key rotation is not required"
	got := Friendly(errors.New(message), "openai")
	if got.Action != RecoveryRetry || got.Message != message {
		t.Fatalf("provider prose was mistaken for a local error: %+v", got)
	}
}
