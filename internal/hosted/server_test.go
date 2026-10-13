package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testDevice = "testdevicehash0001"

type testQuota struct {
	mu          sync.Mutex
	used, spent int64
	active      map[string]bool
	err         error
	resetGlobal bool
	lastMonth   string
}

func (q *testQuota) Reserve(_ context.Context, device, month string, limit, global int64) (Reservation, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return Reservation{}, q.err
	}
	if q.spent >= global {
		return Reservation{}, ErrGlobalLimit
	}
	if q.used >= limit {
		return Reservation{}, ErrDeviceLimit
	}
	q.used++
	q.spent++
	q.lastMonth = month
	id, _ := reservationID()
	if q.active == nil {
		q.active = map[string]bool{}
	}
	q.active[id] = true
	return Reservation{ID: id, Device: device, Month: month, Used: q.used}, nil
}
func (q *testQuota) Commit(_ context.Context, r Reservation) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.active, r.ID)
	return q.err
}
func (q *testQuota) Rollback(_ context.Context, r Reservation) error {
	return q.refund(r, true)
}
func (q *testQuota) RefundDevice(_ context.Context, r Reservation) error {
	return q.refund(r, false)
}
func (q *testQuota) refund(r Reservation, global bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return q.err
	}
	if q.active[r.ID] {
		q.used--
		if global {
			q.spent--
		}
		delete(q.active, r.ID)
	}
	return nil
}
func (q *testQuota) Usage(context.Context, string, string) (int64, int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.used, q.spent, q.err
}
func (q *testQuota) Reset(_ context.Context, _, _ string, global bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return q.err
	}
	q.used = 0
	if global {
		q.spent = 0
	}
	q.resetGlobal = global
	return nil
}

func newTestServer(t *testing.T, upstream http.HandlerFunc, quota QuotaStore) *Server {
	t.Helper()
	remote := httptest.NewServer(upstream)
	t.Cleanup(remote.Close)
	s, err := New(Config{AppToken: "app-secret", AdminToken: "admin-secret", APIKey: "provider-secret", BaseURL: remote.URL, Model: "pinned-model", FreeLimit: 20, GlobalLimit: 100}, quota, remote.Client())
	if err != nil {
		t.Fatal(err)
	}
	s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.now = func() time.Time { return time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("east", 8*3600)) }
	return s
}

func request(s *Server, method, path, body, auth, admin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", auth)
	r.Header.Set("X-Admin-Token", admin)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestAuthenticationAndMethods(t *testing.T) {
	q := &testQuota{}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unauthorized request reached upstream") }, q)
	for _, tc := range []struct {
		method, path, auth string
		status             int
	}{
		{"GET", "/api/chat/completions", "", 405},
		{"POST", "/api/chat/completions", "", 401},
		{"POST", "/api/chat/completions", "Bearer wrong:" + testDevice, 401},
		{"POST", "/api/chat/completions", "Bearer app-secret:short", 400},
		{"POST", "/api/chat/completions", "Bearer app-secret:" + strings.Repeat("x", 257), 400},
		{"POST", "/api/chat/completions", "Bearer app-secret:../../arbitrary-redis", 400},
		{"DELETE", "/api/usage", "Bearer app-secret:" + testDevice, 405},
		{"GET", "/api/usage", "", 401},
		{"GET", "/unknown", "", 404},
	} {
		t.Run(fmt.Sprintf("%s_%s_%d", tc.method, tc.path, tc.status), func(t *testing.T) {
			w := request(s, tc.method, tc.path, `{}`, tc.auth, "")
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("credential response can be cached")
			}
		})
	}
}

func TestProxyPinsCostsAndPreservesResponse(t *testing.T) {
	q := &testQuota{}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Error("wrong upstream route/auth")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "pinned-model" || body["stream"] != false || body["n"] != float64(1) || body["max_tokens"] != float64(512) {
			t.Errorf("unsafe forward body: %v", body)
		}
		if _, exists := body["max_completion_tokens"]; exists {
			t.Error("alternate token ceiling not stripped")
		}
		if used, spent, _ := q.Usage(r.Context(), "", ""); used != 1 || spent != 1 {
			t.Error("upstream started without reserved quota")
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"choices":[{"message":{"content":"named-image"}}]}`)
	}, q)
	w := request(s, "POST", "/api/chat/completions", `{"model":"expensive","stream":true,"n":99,"max_tokens":2,"max_completion_tokens":900000,"messages":[{"role":"user","content":"name"}]}`, "Bearer app-secret:"+testDevice, "")
	if w.Code != 200 || w.Body.String() != `{"choices":[{"message":{"content":"named-image"}}]}` || w.Header().Get("X-Chaos-Trial-Remaining") != "19" {
		t.Fatalf("response %d %s %v", w.Code, w.Body, w.Header())
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.used != 1 || q.spent != 1 || len(q.active) != 0 {
		t.Fatalf("quota not committed: %+v", q)
	}
	if q.lastMonth != "2026-09" {
		t.Fatalf("quota month must use UTC, got %s", q.lastMonth)
	}
}

func TestValidationAndTokenCeilings(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{{"", 256}, {`,"max_tokens":32`, 32}, {`,"max_tokens":"64"`, 64}, {`,"max_tokens":-1`, 256}, {`,"max_tokens":1e100`, 512}, {`,"max_tokens":12.7`, 12}, {`,"max_completion_tokens":20`, 20}} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"messages":[{"role":"user","content":"name"}]`+tc.value+`}`))
		b, err := readForwardBody(httptest.NewRecorder(), r, "model")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if m["max_tokens"] != float64(tc.want) {
			t.Errorf("%s: %s", tc.value, b)
		}
	}
	s := newTestServer(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid body reached upstream") }, &testQuota{})
	for _, body := range []string{"null", `[]`, `{`, `{}`, `{"messages":null}`, `{"messages":[]}`, `{"messages":[1]}`, `{"messages":[{}]} {}`, `{"messages":[` + strings.Repeat(`{},`, 16) + `{}]}`} {
		if w := request(s, "POST", "/api/chat/completions", body, "Bearer app-secret:"+testDevice, ""); w.Code != 400 {
			t.Errorf("body %q: %d", body, w.Code)
		}
	}
	if w := request(s, "POST", "/api/chat/completions", `{"messages":[{"content":"`+strings.Repeat("x", MaxRequestBytes)+`"}]}`, "Bearer app-secret:"+testDevice, ""); w.Code != 413 {
		t.Fatalf("oversized body: %d", w.Code)
	}
}

func TestQuotaDenialAndStoreFailure(t *testing.T) {
	for _, tc := range []struct {
		q      *testQuota
		status int
	}{{&testQuota{used: 20}, 402}, {&testQuota{spent: 100}, 402}, {&testQuota{err: errors.New("redis down")}, 503}} {
		s := newTestServer(t, func(http.ResponseWriter, *http.Request) { t.Error("request must fail closed") }, tc.q)
		w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, "")
		if w.Code != tc.status {
			t.Fatalf("got %d: %s", w.Code, w.Body)
		}
	}
}

func TestUpstreamFailuresRefund(t *testing.T) {
	for _, status := range []int{400, 429, 500, 307} {
		q := &testQuota{}
		s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "/redirect")
			w.WriteHeader(status)
			io.WriteString(w, `{"error":{"message":"upstream failure"}}`)
		}, q)
		w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, "")
		if w.Code != status || !strings.Contains(w.Body.String(), "upstream failure") {
			t.Fatalf("%d: %d %s", status, w.Code, w.Body)
		}
		if q.used != 0 || q.spent != 0 {
			t.Fatalf("failed request charged: %+v", q)
		}
	}
	q := &testQuota{}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}, q)
	s.config.UpstreamTimeout = 20 * time.Millisecond
	if w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, ""); w.Code != 502 {
		t.Fatalf("timeout: %d", w.Code)
	}
	if q.used != 0 || q.spent != 1 {
		t.Fatal("uncertain timeout must refund only the device allowance")
	}
}

func TestUsageAndAdminReset(t *testing.T) {
	q := &testQuota{used: 3, spent: 9}
	s := newTestServer(t, func(http.ResponseWriter, *http.Request) { t.Error("usage reached provider") }, q)
	w := request(s, "GET", "/api/usage", "", "Bearer app-secret:"+testDevice, "")
	var usage Usage
	_ = json.Unmarshal(w.Body.Bytes(), &usage)
	if w.Code != 200 || usage.Used != 3 || usage.Remaining != 17 || usage.GlobalSpent != 9 || usage.GlobalLimit != 100 {
		t.Fatalf("usage: %s", w.Body)
	}
	for _, admin := range []string{"", "wrong"} {
		if w := request(s, "POST", "/api/usage?global=1", "", "Bearer app-secret:"+testDevice, admin); w.Code != 403 {
			t.Fatalf("admin check %d", w.Code)
		}
	}
	w = request(s, "POST", "/api/usage?global=TrUe", "", "Bearer app-secret:"+testDevice, "admin-secret")
	if w.Code != 200 || q.used != 0 || q.spent != 0 || !q.resetGlobal {
		t.Fatalf("reset: %d %+v", w.Code, q)
	}
	q.err = errors.New("Redis offline")
	if w := request(s, "GET", "/api/usage", "", "Bearer app-secret:"+testDevice, ""); w.Code != 503 {
		t.Fatalf("usage store failure: %d", w.Code)
	}
}

func TestOversizedUpstreamRefundsOnlyDeviceQuota(t *testing.T) {
	q := &testQuota{}
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", MaxResponseBytes+1))
	}, q)
	w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, "")
	if w.Code != 502 || q.used != 0 || q.spent != 1 {
		t.Fatalf("oversized response refunded billable budget: status=%d usage=%d,%d", w.Code, q.used, q.spent)
	}
}

func TestPredispatchFailureRefundsBothCounters(t *testing.T) {
	q := &testQuota{}
	s := newTestServer(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid request reached upstream") }, q)
	s.config.BaseURL = "http://%"
	w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, "")
	if w.Code != 502 || q.used != 0 || q.spent != 0 {
		t.Fatalf("predispatch failure consumed quota: status=%d usage=%d,%d", w.Code, q.used, q.spent)
	}
}

func TestConcurrentHTTPRequestsCannotExceedQuota(t *testing.T) {
	q := &testQuota{}
	var calls atomic.Int64
	s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, `{}`) }, q)
	s.config.FreeLimit = 7
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(s, "POST", "/api/chat/completions", `{"messages":[{}]}`, "Bearer app-secret:"+testDevice, "")
			if w.Code != 200 && w.Code != 402 {
				t.Errorf("unexpected %d", w.Code)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 7 || q.used != 7 || q.spent != 7 {
		t.Fatalf("calls=%d used=%d spent=%d", calls.Load(), q.used, q.spent)
	}
}

func TestConfigFailsClosed(t *testing.T) {
	t.Setenv("APP_TOKEN", " app ")
	t.Setenv("AGNES_API_KEY", " key ")
	t.Setenv("FREE_LIMIT", "20")
	t.Setenv("GLOBAL_MONTHLY_LIMIT", "100")
	t.Setenv("AGNES_BASE_URL", "https://example.com/v1/")
	c, err := ConfigFromEnv()
	if err != nil || c.AppToken != "app" || c.APIKey != "key" || c.BaseURL != "https://example.com/v1" {
		t.Fatalf("config %#v %v", c, err)
	}
	for _, bad := range []string{"-1", "nope", "20junk", "1.5", "1000000001"} {
		t.Setenv("FREE_LIMIT", bad)
		if _, err := ConfigFromEnv(); err == nil {
			t.Errorf("accepted invalid limit %q", bad)
		}
	}
	t.Setenv("FREE_LIMIT", "0")
	if c, err := ConfigFromEnv(); err != nil || c.FreeLimit != 0 {
		t.Fatal("zero must disable free trial")
	}
	t.Setenv("AGNES_API_KEY", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("missing provider secret accepted")
	}
}
