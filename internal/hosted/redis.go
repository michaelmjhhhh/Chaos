package hosted

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// A single EVAL checks and increments BOTH counters atomically. Reading first
// then incrementing after success (the old proxy) allowed concurrent overspend.
// Generation stamps ensure a failed pre-reset request cannot subtract from a
// newer counter. Reservation tokens make refunds idempotent.
const reserveScript = `
local used = tonumber(redis.call('GET', KEYS[1]) or '0')
local spent = tonumber(redis.call('GET', KEYS[2]) or '0')
if not used or not spent or used < 0 or spent < 0 or used % 1 ~= 0 or spent % 1 ~= 0 then
  return redis.error_reply('invalid quota counter')
end
if spent >= tonumber(ARGV[2]) then return {0, 2, used} end
if used >= tonumber(ARGV[1]) then return {0, 1, used} end
local dg = redis.call('GET', KEYS[3]) or '0'
local bg = redis.call('GET', KEYS[4]) or '0'
redis.call('SET', KEYS[5], dg .. ':' .. bg, 'EX', 172800)
used = redis.call('INCR', KEYS[1])
spent = redis.call('INCR', KEYS[2])
if spent == 1 then redis.call('EXPIRE', KEYS[2], ARGV[3]) end
return {1, 0, used}
`

const rollbackScript = `
local stamp = redis.call('GET', KEYS[5])
if not stamp then return 0 end
local dg, bg = string.match(stamp, '^(%d+):(%d+)$')
if not dg then return redis.error_reply('invalid reservation') end
local used = tonumber(redis.call('GET', KEYS[1]) or '0')
local spent = tonumber(redis.call('GET', KEYS[2]) or '0')
if not used or not spent or used < 0 or spent < 0 then return redis.error_reply('invalid quota counter') end
if dg == (redis.call('GET', KEYS[3]) or '0') and used > 0 then redis.call('DECR', KEYS[1]) end
if bg == (redis.call('GET', KEYS[4]) or '0') and spent > 0 then redis.call('DECR', KEYS[2]) end
redis.call('DEL', KEYS[5])
return 1
`

const resetScript = `
redis.call('INCR', KEYS[3])
redis.call('DEL', KEYS[1])
if ARGV[1] == '1' then
  redis.call('INCR', KEYS[4])
  redis.call('EXPIRE', KEYS[4], 3024000)
  redis.call('DEL', KEYS[2])
end
return 1
`

// RedisStore speaks Upstash's documented JSON-array REST command protocol.
// Never retry a reservation implicitly: an uncertain result must fail closed.
type RedisStore struct {
	URL, Token string
	Client     *http.Client
}

func RedisFromEnv() (*RedisStore, error) {
	r := &RedisStore{URL: strings.TrimRight(strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_URL")), "/"), Token: strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_TOKEN"))}
	if err := validateHTTPURL(r.URL); err != nil {
		return nil, fmt.Errorf("UPSTASH_REDIS_REST_URL: %w", err)
	}
	if r.Token == "" {
		return nil, errors.New("UPSTASH_REDIS_REST_TOKEN is required")
	}
	r.Client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: rejectRedirect}
	return r, nil
}

func (s *RedisStore) Reserve(ctx context.Context, device, month string, limit, globalLimit int64) (Reservation, error) {
	id, err := reservationID()
	if err != nil {
		return Reservation{}, err
	}
	r := Reservation{ID: id, Device: device, Month: month}
	var result []int64
	if err := s.command(ctx, &result, evalArgs(reserveScript, r, limit, globalLimit, 3024000)...); err != nil {
		return r, err
	}
	if len(result) != 3 {
		return r, errors.New("invalid Redis reservation response")
	}
	if result[0] == 0 {
		if result[1] == 1 {
			return Reservation{}, ErrDeviceLimit
		}
		if result[1] == 2 {
			return Reservation{}, ErrGlobalLimit
		}
		return r, errors.New("invalid Redis quota rejection")
	}
	if result[0] != 1 || result[2] <= 0 {
		return r, errors.New("invalid Redis quota acceptance")
	}
	r.Used = result[2]
	return r, nil
}

func (s *RedisStore) Commit(ctx context.Context, r Reservation) error {
	var result int64
	return s.command(ctx, &result, "DEL", "reservation:"+r.ID)
}

func (s *RedisStore) Rollback(ctx context.Context, r Reservation) error {
	if r.ID == "" {
		return nil
	}
	var result int64
	return s.command(ctx, &result, evalArgs(rollbackScript, r)...)
}

func (s *RedisStore) Usage(ctx context.Context, device, month string) (int64, int64, error) {
	var values []json.RawMessage
	if err := s.command(ctx, &values, "MGET", "dev:"+device, "budget:"+month); err != nil {
		return 0, 0, err
	}
	if len(values) != 2 {
		return 0, 0, errors.New("invalid Redis usage response")
	}
	used, err := parseCounter(values[0])
	if err != nil {
		return 0, 0, err
	}
	spent, err := parseCounter(values[1])
	return used, spent, err
}

func (s *RedisStore) Reset(ctx context.Context, device, month string, global bool) error {
	flag := "0"
	if global {
		flag = "1"
	}
	var result int64
	return s.command(ctx, &result, "EVAL", resetScript, 4, "dev:"+device, "budget:"+month, "generation:dev:"+device, "generation:budget:"+month, flag)
}

func evalArgs(script string, r Reservation, args ...any) []any {
	return append([]any{"EVAL", script, 5, "dev:" + r.Device, "budget:" + r.Month, "generation:dev:" + r.Device, "generation:budget:" + r.Month, "reservation:" + r.ID}, args...)
}

func parseCounter(raw json.RawMessage) (int64, error) {
	if string(raw) == "null" {
		return 0, nil
	}
	text := string(raw)
	if len(text) > 0 && text[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n < 0 {
		return 0, errors.New("invalid Redis counter")
	}
	return n, nil
}

func (s *RedisStore) command(ctx context.Context, out any, args ...any) error {
	body, err := json.Marshal(args)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: rejectRedirect}
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("quota service is unreachable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("quota service returned HTTP %d", res.StatusCode)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&envelope); err != nil {
		return fmt.Errorf("invalid quota response: %w", err)
	}
	if envelope.Error != "" {
		return errors.New("quota command failed")
	}
	if len(envelope.Result) == 0 {
		return errors.New("quota response has no result")
	}
	return json.Unmarshal(envelope.Result, out)
}

func rejectRedirect(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
