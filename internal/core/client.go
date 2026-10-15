package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported provider; choose a provider in Settings")
	ErrMissingAPIKey       = errors.New("API key not configured")
	ErrInvalidBaseURL      = errors.New("invalid provider base URL")
	ErrEmptyResponse       = errors.New("empty response from provider")
	ErrImageNotStable      = errors.New("image did not stabilize before processing")
	ErrResponseTooLarge    = errors.New("response exceeds size limit")
)

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
	Messages  []chatMessage `json:"messages"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // The API accepts a string or a list of content parts.
}
type chatPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}
type chatImageURL struct {
	URL string `json:"url"`
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type Client struct{ HTTP HTTPDoer }
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("provider HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("provider HTTP %d", e.Status)
}
func (c *Client) do(ctx context.Context, method, address, key string, payload any, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, address, &body)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := boundedRead(response.Body, 2<<20)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &HTTPError{Status: response.StatusCode, Message: providerError(data)}
	}
	return data, nil
}
func (c *Client) GenerateSlug(ctx context.Context, image PreparedImage, cfg ResolvedConfig) (string, error) {
	if err := cfg.validateProvider(); err != nil {
		return "", err
	}
	if image.Base64 == "" {
		return "", errors.New("image is empty")
	}
	address, err := endpoint(cfg.BaseURL, "chat/completions")
	if err != nil {
		return "", err
	}
	system := cfg.CustomPrompt
	if strings.TrimSpace(system) == "" {
		system = DefaultPrompt(cfg.Language)
	}
	user := "Return only the filename slug."
	if cfg.CustomPrompt == "" && cfg.Language == "zh" {
		user = "Return only the filename slug in Simplified Chinese."
	}
	payload := chatRequest{Model: cfg.Model, MaxTokens: 32, Messages: []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: []chatPart{
			{Type: "text", Text: user},
			{Type: "image_url", ImageURL: &chatImageURL{URL: "data:" + image.MIMEType + ";base64," + image.Base64}},
		}},
	}}
	data, err := c.do(ctx, http.MethodPost, address, cfg.APIKey, payload, 30*time.Second)
	if err != nil {
		return "", err
	}
	return ParseSlug(data)
}
func (c *Client) CheckHealth(ctx context.Context, cfg ResolvedConfig) error {
	if err := cfg.validateProvider(); err != nil {
		return err
	}
	address, err := endpoint(cfg.BaseURL, "chat/completions")
	if err != nil {
		return err
	}
	data, err := c.do(ctx, http.MethodPost, address, cfg.APIKey, chatRequest{Model: cfg.Model, MaxTokens: 8, Messages: []chatMessage{{Role: "user", Content: "Reply with only: ok"}}}, 15*time.Second)
	if err != nil {
		return err
	}
	_, err = ParseSlug(data)
	return err
}
func ParseSlug(data []byte) (string, error) {
	var r struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("invalid provider JSON: %w", err)
	}
	if len(r.Choices) > 0 {
		raw := r.Choices[0].Message.Content
		var text string
		if json.Unmarshal(raw, &text) == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		var parts []struct{ Type, Text string }
		if json.Unmarshal(raw, &parts) == nil {
			for _, p := range parts {
				if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
					return strings.TrimSpace(p.Text), nil
				}
			}
		}
	}
	if len(r.Candidates) > 0 {
		for _, p := range r.Candidates[0].Content.Parts {
			if strings.TrimSpace(p.Text) != "" {
				return strings.TrimSpace(p.Text), nil
			}
		}
	}
	if msg := providerError(data); msg != "" {
		return "", errors.New(msg)
	}
	return "", ErrEmptyResponse
}
func providerError(data []byte) string {
	var v struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	var text string
	if json.Unmarshal(v.Error, &text) == nil {
		return text
	}
	var detail struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(v.Error, &detail)
	return detail.Message
}

type RecoveryAction string

const (
	RecoveryNone     RecoveryAction = ""
	RecoverySettings RecoveryAction = "settings"
	RecoveryRetry    RecoveryAction = "retry"
	RecoveryInternet RecoveryAction = "internet"
)

type FriendlyError struct {
	Message string
	Action  RecoveryAction
}

func Friendly(err error, provider string) FriendlyError {
	var h *HTTPError
	if errors.As(err, &h) {
		switch h.Status {
		case 401, 403:
			return FriendlyError{"Your API key doesn't look right. Open Settings to re-enter it.", RecoverySettings}
		case 402:
			return FriendlyError{"Your provider account is out of credit. Add funds, or switch provider in Settings.", RecoverySettings}
		case 404:
			return FriendlyError{"That model wasn't found. Check the model name in Settings.", RecoverySettings}
		case 429:
			return FriendlyError{"The naming service is busy right now. Wait a moment and try again.", RecoveryRetry}
		}
		if h.Status >= 500 {
			return FriendlyError{"The naming service had a temporary problem. Try again in a moment.", RecoveryRetry}
		}
	}
	if errors.Is(err, context.Canceled) {
		return FriendlyError{"Processing was cancelled.", RecoveryRetry}
	}
	// PathError implements net.Error too, and LinkError can wrap an errno that
	// does. Keep local file failures out of the internet/Ollama recovery path.
	// Report the filesystem error itself, not an outer request wrapper that may
	// contain a provider URL or credentials.
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return FriendlyError{pathError.Error(), RecoveryRetry}
	}
	var linkError *os.LinkError
	if errors.As(err, &linkError) {
		return FriendlyError{linkError.Error(), RecoveryRetry}
	}
	var network net.Error
	if errors.As(err, &network) {
		if network.Timeout() {
			return FriendlyError{"The naming service took too long to respond. Try again.", RecoveryRetry}
		}
		if provider == "ollama" {
			return FriendlyError{"Couldn't reach Ollama on this Mac. Make sure Ollama is running.", RecoverySettings}
		}
		return FriendlyError{"Couldn't reach the naming service. Check your internet, then try again.", RecoveryInternet}
	}
	if errors.Is(err, ErrUnsupportedProvider) {
		return FriendlyError{ErrUnsupportedProvider.Error(), RecoverySettings}
	}
	if errors.Is(err, ErrInvalidBaseURL) || errors.Is(err, ErrMissingAPIKey) {
		return FriendlyError{"Check the provider address and API key in Settings.", RecoverySettings}
	}
	if errors.Is(err, ErrEmptyResponse) {
		return FriendlyError{"The AI didn't return a name for that image. Try again.", RecoveryRetry}
	}
	if errors.Is(err, ErrImageNotStable) {
		return FriendlyError{"That screenshot was still being saved. Try again in a second.", RecoveryRetry}
	}
	return FriendlyError{err.Error(), RecoveryRetry}
}
