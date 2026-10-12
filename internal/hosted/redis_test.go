package hosted

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRedisRESTProtocolAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
		wantError  bool
	}{
		{"strings", `{"result":["4","9"]}`, 200, false},
		{"numbers", `{"result":[4,9]}`, 200, false},
		{"empty", `{"result":[null,null]}`, 200, false},
		{"counter corruption", `{"result":["oops","9"]}`, 200, true},
		{"negative counter", `{"result":[-1,9]}`, 200, true},
		{"redis error", `{"error":"denied"}`, 200, true},
		{"no result", `{}`, 200, true},
		{"wrong result", `{"result":[1]}`, 200, true},
		{"bad JSON", `<html>oops</html>`, 200, true},
		{"HTTP error", `{"error":"offline"}`, 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("incorrect Redis protocol")
				}
				var cmd []string
				if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil || len(cmd) != 3 || cmd[0] != "MGET" || cmd[1] != "dev:device" || cmd[2] != "budget:2026-10" {
					t.Errorf("command %v %v", cmd, err)
				}
				w.WriteHeader(tc.code)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			s := &RedisStore{URL: server.URL, Token: "test-token", Client: server.Client()}
			_, _, err := s.Usage(context.Background(), "device", "2026-10")
			if (err != nil) != tc.wantError {
				t.Fatalf("error %v, want error %v", err, tc.wantError)
			}
		})
	}
}

// This exercises the actual Lua scripts in a local Redis process through an
// HTTP bridge implementing the documented Upstash wire protocol. No remote
// account, network Redis database, SDK, or production credentials are involved.
// CI installs redis-server; developers can set REDIS_SERVER_BIN to a local build.
func TestRedisQuotaIntegration(t *testing.T) {
	s := localRedis(t)
	ctx := context.Background()
	resetDB := func() {
		t.Helper()
		var out string
		if err := s.command(ctx, &out, "FLUSHDB"); err != nil {
			t.Fatal(err)
		}
	}
	usage := func(device string) (int64, int64) {
		t.Helper()
		u, g, err := s.Usage(ctx, device, "2026-10")
		if err != nil {
			t.Fatal(err)
		}
		return u, g
	}
	t.Run("device cap under concurrency", func(t *testing.T) {
		resetDB()
		var successes atomic.Int64
		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, err := s.Reserve(ctx, "device", "2026-10", 7, 100)
				if err == nil {
					successes.Add(1)
					if err := s.Commit(ctx, r); err != nil {
						t.Error(err)
					}
				} else if !errors.Is(err, ErrDeviceLimit) {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		u, g := usage("device")
		if successes.Load() != 7 || u != 7 || g != 7 {
			t.Fatalf("successes=%d usage=%d,%d", successes.Load(), u, g)
		}
	})
	t.Run("global cap across devices", func(t *testing.T) {
		resetDB()
		var successes atomic.Int64
		var wg sync.WaitGroup
		for i := range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.Reserve(ctx, fmt.Sprint(i), "2026-10", 20, 9)
				if err == nil {
					successes.Add(1)
				} else if !errors.Is(err, ErrGlobalLimit) {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		_, g := usage("0")
		if successes.Load() != 9 || g != 9 {
			t.Fatalf("successes=%d global=%d", successes.Load(), g)
		}
	})
	t.Run("refund preserves concurrent success and is idempotent", func(t *testing.T) {
		resetDB()
		failed, err := s.Reserve(ctx, "device", "2026-10", 20, 100)
		if err != nil {
			t.Fatal(err)
		}
		success, err := s.Reserve(ctx, "device", "2026-10", 20, 100)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(ctx, success); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := s.Rollback(ctx, failed); err != nil {
				t.Fatal(err)
			}
		}
		u, g := usage("device")
		if u != 1 || g != 1 {
			t.Fatalf("lost concurrent usage: %d,%d", u, g)
		}
	})
	t.Run("refund across administrative reset", func(t *testing.T) {
		for _, global := range []bool{false, true} {
			resetDB()
			old, err := s.Reserve(ctx, "device", "2026-10", 20, 100)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Reset(ctx, "device", "2026-10", global); err != nil {
				t.Fatal(err)
			}
			fresh, err := s.Reserve(ctx, "device", "2026-10", 20, 100)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Commit(ctx, fresh); err != nil {
				t.Fatal(err)
			}
			if err := s.Rollback(ctx, old); err != nil {
				t.Fatal(err)
			}
			u, g := usage("device")
			if u != 1 || g != 1 {
				t.Fatalf("reset global=%v erased newer usage: %d,%d", global, u, g)
			}
		}
	})
	t.Run("corruption fails closed", func(t *testing.T) {
		resetDB()
		var out string
		if err := s.command(ctx, &out, "SET", "dev:device", "invalid"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Reserve(ctx, "device", "2026-10", 20, 100); err == nil {
			t.Fatal("corrupt quota accepted")
		}
	})
	t.Run("month TTL and rollover", func(t *testing.T) {
		resetDB()
		r, err := s.Reserve(ctx, "device", "2026-10", 20, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(ctx, r); err != nil {
			t.Fatal(err)
		}
		var ttl int64
		if err := s.command(ctx, &ttl, "TTL", "budget:2026-10"); err != nil || ttl < 3023900 {
			t.Fatalf("month TTL %d %v", ttl, err)
		}
		if _, err := s.Reserve(ctx, "device", "2026-11", 20, 1); err != nil {
			t.Fatalf("new month blocked: %v", err)
		}
	})
}

func localRedis(t *testing.T) *RedisStore {
	t.Helper()
	binary := os.Getenv("REDIS_SERVER_BIN")
	if binary == "" {
		var err error
		binary, err = exec.LookPath("redis-server")
		if err != nil {
			t.Skip("redis-server not installed; set REDIS_SERVER_BIN to run actual Lua integration tests")
		}
	}
	dir, err := os.MkdirTemp("", "chaos-redis-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "redis.sock")
	process := exec.Command(binary, "--port", "0", "--unixsocket", socket, "--save", "", "--appendonly", "no", "--dir", dir)
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Process.Kill(); _ = process.Wait() })
	ready := false
	for range 100 {
		conn, err := net.DialTimeout("unix", socket, 20*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("local Redis did not start")
	}
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-test" {
			http.Error(w, "unauthorized", 401)
			return
		}
		var args []any
		d := json.NewDecoder(r.Body)
		d.UseNumber()
		if err := d.Decode(&args); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		conn, err := net.DialTimeout("unix", socket, time.Second)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprintf(conn, "*%d\r\n", len(args))
		for _, arg := range args {
			v := fmt.Sprint(arg)
			fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(v), v)
		}
		value, err := readRESP(bufio.NewReader(conn))
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": value})
	}))
	t.Cleanup(bridge.Close)
	return &RedisStore{URL: bridge.URL, Token: "local-test", Client: bridge.Client()}
}

func readRESP(r *bufio.Reader) (any, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, errors.New("short RESP")
	}
	value := strings.TrimSuffix(line[1:], "\r\n")
	switch line[0] {
	case '+':
		return value, nil
	case '-':
		return nil, errors.New(value)
	case ':':
		return strconv.ParseInt(value, 10, 64)
	case '$':
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil
		}
		if n < 0 || n > 1<<20 {
			return nil, errors.New("invalid RESP bulk")
		}
		b := make([]byte, n+2)
		_, err = io.ReadFull(r, b)
		return string(b[:n]), err
	case '*':
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 1024 {
			return nil, errors.New("invalid RESP array")
		}
		values := make([]any, n)
		for i := range values {
			values[i], err = readRESP(r)
			if err != nil {
				return nil, err
			}
		}
		return values, nil
	default:
		return nil, errors.New("unknown RESP type")
	}
}
