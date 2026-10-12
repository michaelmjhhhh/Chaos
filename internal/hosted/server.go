package hosted

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	MaxRequestBytes     = 4 << 20
	MaxResponseBytes    = 4 << 20
	MaxMessages         = 16
	MaxOutputTokens     = 512
	DefaultOutputTokens = 256
)

type Server struct {
	config Config
	quota  QuotaStore
	client *http.Client
	now    func() time.Time
	log    *slog.Logger
}

func New(config Config, quota QuotaStore, client *http.Client) (*Server, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, errors.New("a persistent quota store is required")
	}
	if config.UpstreamTimeout <= 0 {
		config.UpstreamTimeout = 60 * time.Second
	}
	if client == nil {
		client = &http.Client{}
	}
	// Do not follow redirects with a provider credential, including same-host redirects.
	copyClient := *client
	copyClient.CheckRedirect = rejectRedirect
	return &Server{config: config, quota: quota, client: &copyClient, now: time.Now, log: slog.Default()}, nil
}

func FromEnv() (*Server, error) {
	config, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	quota, err := RedisFromEnv()
	if err != nil {
		return nil, err
	}
	return New(config, quota, nil)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch r.URL.Path {
	case "/api/chat/completions":
		s.ChatCompletions(w, r)
	case "/api/usage":
		s.Usage(w, r)
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

func (s *Server) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	device, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	body, err := readForwardBody(w, r, s.config.Model)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "Request body is too large")
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	month := s.now().UTC().Format("2006-01")
	reservation, err := s.quota.Reserve(r.Context(), device, month, s.config.FreeLimit, s.config.GlobalLimit)
	if err != nil {
		switch {
		case errors.Is(err, ErrGlobalLimit):
			w.Header().Set("X-Chaos-Trial-Remaining", "0")
			writeError(w, http.StatusPaymentRequired, "Free naming is paused for now. Add your own provider key in Settings.")
		case errors.Is(err, ErrDeviceLimit):
			w.Header().Set("X-Chaos-Trial-Remaining", "0")
			writeError(w, http.StatusPaymentRequired, "Free trial used up.")
		default:
			// A lost Redis response may have reserved quota; the unique token
			// permits a safe best-effort refund without sending a paid request.
			s.refund(reservation)
			s.log.Error("hosted quota reservation failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "Free naming is temporarily unavailable. Try again.")
		}
		return
	}
	committed := false
	defer func() {
		if !committed {
			s.refund(reservation)
		}
	}()
	ctx, cancel := context.WithTimeout(r.Context(), s.config.UpstreamTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.config.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Naming service is unavailable.")
		return
	}
	request.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Naming service is unreachable. Try again.")
		return
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil || len(payload) > MaxResponseBytes {
		writeError(w, http.StatusBadGateway, "Naming service returned an invalid response. Try again.")
		return
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		// Counters already include this request. Failure to discard the token
		// must not refund a completed paid request or allow quota overspend.
		committed = true
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.quota.Commit(cleanup, reservation); err != nil {
			s.log.Error("hosted reservation cleanup failed", "error", err)
		}
		stop()
		w.Header().Set("X-Chaos-Trial-Remaining", strconv.FormatInt(max(0, s.config.FreeLimit-reservation.Used), 10))
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(payload)
}

func (s *Server) Usage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	device, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	month := s.now().UTC().Format("2006-01")
	if r.Method == http.MethodPost {
		if !sameSecret(strings.TrimSpace(r.Header.Get("X-Admin-Token")), s.config.AdminToken) {
			writeError(w, http.StatusForbidden, "Admin token required")
			return
		}
		global := r.URL.Query().Get("global") == "1" || strings.EqualFold(r.URL.Query().Get("global"), "true")
		if err := s.quota.Reset(r.Context(), device, month, global); err != nil {
			s.log.Error("hosted quota reset failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "Usage service is temporarily unavailable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "resetDevice": true, "resetGlobal": global})
		return
	}
	used, spent, err := s.quota.Usage(r.Context(), device, month)
	if err != nil {
		s.log.Error("hosted usage lookup failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "Usage service is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, Usage{Used: used, Limit: s.config.FreeLimit, Remaining: max(0, s.config.FreeLimit-used), GlobalSpent: spent, GlobalLimit: s.config.GlobalLimit})
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "Missing credentials")
		return "", false
	}
	token, device, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), ":")
	if !sameSecret(strings.TrimSpace(token), s.config.AppToken) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return "", false
	}
	device = strings.TrimSpace(device)
	valid := len(device) >= 16 && len(device) <= 256
	for _, ch := range device {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			valid = false
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, "Missing or invalid device identifier")
		return "", false
	}
	return device, true
}

func sameSecret(got, want string) bool {
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return want != "" && subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func (s *Server) refund(reservation Reservation) {
	if reservation.ID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.quota.Rollback(ctx, reservation); err != nil {
		// Conservatively leave the counters consumed if Redis cannot confirm
		// the refund. Never overwrite a counter with an earlier snapshot.
		s.log.Error("hosted quota refund failed", "error", err)
	}
}

func readForwardBody(w http.ResponseWriter, r *http.Request, model string) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	var body map[string]json.RawMessage
	if err := decoder.Decode(&body); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return nil, err
		}
		return nil, errors.New("Invalid request body")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return nil, err
		}
		return nil, errors.New("Invalid request body")
	}
	var messages []json.RawMessage
	if body == nil || json.Unmarshal(body["messages"], &messages) != nil || len(messages) == 0 {
		return nil, errors.New("Invalid request body")
	}
	if len(messages) > MaxMessages {
		return nil, errors.New("Too many messages")
	}
	for _, raw := range messages {
		var message map[string]json.RawMessage
		if json.Unmarshal(raw, &message) != nil || message == nil {
			return nil, errors.New("Invalid message")
		}
	}
	requested := max(tokenCount(body["max_tokens"]), tokenCount(body["max_completion_tokens"]))
	if requested <= 0 {
		requested = DefaultOutputTokens
	}
	requested = min(requested, MaxOutputTokens)
	delete(body, "max_completion_tokens")
	body["model"], _ = json.Marshal(model)
	body["stream"] = json.RawMessage("false")
	body["n"] = json.RawMessage("1")
	body["max_tokens"] = json.RawMessage(strconv.Itoa(requested))
	return json.Marshal(body)
}

func tokenCount(raw json.RawMessage) int {
	text := string(raw)
	if len(text) > 0 && text[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return 0
		}
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return 0
	}
	if n >= MaxOutputTokens {
		return MaxOutputTokens
	}
	return int(n)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
