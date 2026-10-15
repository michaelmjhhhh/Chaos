package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultClientRejectsRedirects(t *testing.T) {
	var received atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"leaked"}}]}`)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	cfg := testConfig(redirect.URL, filepath.Join(t.TempDir(), "out"))
	for _, generate := range []bool{false, true} {
		var err error
		if generate {
			_, err = (&Client{}).GenerateSlug(context.Background(), PreparedImage{"eA==", "image/png"}, cfg)
		} else {
			err = (&Client{}).CheckHealth(context.Background(), cfg)
		}
		var status *HTTPError
		if !errors.As(err, &status) || status.Status != 307 {
			t.Fatalf("redirect not rejected: %v", err)
		}
	}
	if received.Load() != 0 {
		t.Fatal("redirect destination received credentials")
	}
}
func TestDefaultClientRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"`+strings.Repeat("x", 2<<20)+`"}}]}`)
	}))
	defer server.Close()
	_, err := (&Client{}).GenerateSlug(context.Background(), PreparedImage{"eA==", "image/png"}, testConfig(server.URL, filepath.Join(t.TempDir(), "out")))
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized response accepted: %v", err)
	}
}
func TestDefaultClientCancelsInflightRequest(t *testing.T) {
	for _, generate := range []bool{false, true} {
		t.Run(fmt.Sprint("generate=", generate), func(t *testing.T) {
			entered, disconnected := make(chan struct{}), make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(entered)
				select {
				case <-r.Context().Done():
					close(disconnected)
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			cfg := testConfig(server.URL, filepath.Join(t.TempDir(), "out"))
			go func() {
				var err error
				if generate {
					_, err = (&Client{}).GenerateSlug(ctx, PreparedImage{"eA==", "image/png"}, cfg)
				} else {
					err = (&Client{}).CheckHealth(ctx, cfg)
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("request never started")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation lost", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("request did not return")
			}
			select {
			case <-disconnected:
			case <-time.After(5 * time.Second):
				t.Fatal("server connection not cancelled")
			}
		})
	}
}
func TestNetworkEntryPointsValidateRequiredKey(t *testing.T) {
	cfg := testConfig("https://unit.invalid", filepath.Join(t.TempDir(), "out"))
	cfg.APIKey = ""
	cfg.Provider.RequiresKey = false
	capture := &captureRequests{}
	client := &Client{HTTP: capture}
	if err := client.CheckHealth(context.Background(), cfg); !errors.Is(err, ErrMissingAPIKey) {
		t.Fatal(err)
	}
	if _, err := client.GenerateSlug(context.Background(), PreparedImage{"eA==", "image/png"}, cfg); !errors.Is(err, ErrMissingAPIKey) {
		t.Fatal(err)
	}
	if len(capture.requests) != 0 {
		t.Fatal("missing key reached transport")
	}
}
func TestFriendlyClassifiesWrappedDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		action RecoveryAction
		detail string
	}{{ErrUnsupportedProvider, RecoverySettings, "provider"}, {ErrMissingAPIKey, RecoverySettings, "Settings"}, {ErrInvalidBaseURL, RecoverySettings, "Settings"}, {ErrEmptyResponse, RecoveryRetry, "name"}, {ErrImageNotStable, RecoveryRetry, "saved"}} {
		got := Friendly(fmt.Errorf("context: %w", tc.err), "openai")
		if got.Action != tc.action || !strings.Contains(got.Message, tc.detail) {
			t.Fatalf("wrong classification: %+v", got)
		}
	}
	// External arbitrary text must not accidentally dispatch an owned domain recovery.
	got := Friendly(errors.New("unstable remote api key bookkeeping"), "openai")
	if got.Action != RecoveryRetry || got.Message != "unstable remote api key bookkeeping" {
		t.Fatalf("substring classification: %+v", got)
	}
}
