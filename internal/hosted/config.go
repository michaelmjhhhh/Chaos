// Package hosted implements the optional hosted naming trial as a Go HTTP API.
// It deliberately has no dependency on the desktop UI or a provider SDK.
package hosted

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppToken, AdminToken, APIKey string
	BaseURL, Model               string
	FreeLimit, GlobalLimit       int64
	UpstreamTimeout              time.Duration
}

func ConfigFromEnv() (Config, error) {
	c := Config{
		AppToken:        strings.TrimSpace(os.Getenv("APP_TOKEN")),
		AdminToken:      strings.TrimSpace(os.Getenv("ADMIN_TOKEN")),
		APIKey:          strings.TrimSpace(os.Getenv("AGNES_API_KEY")),
		BaseURL:         strings.TrimRight(envDefault("AGNES_BASE_URL", "https://apihub.agnes-ai.com/v1"), "/"),
		Model:           envDefault("MODEL", "agnes-2.0-flash"),
		UpstreamTimeout: 60 * time.Second,
	}
	var err error
	if c.FreeLimit, err = limitEnv("FREE_LIMIT", 20); err != nil {
		return c, err
	}
	if c.GlobalLimit, err = limitEnv("GLOBAL_MONTHLY_LIMIT", 100_000); err != nil {
		return c, err
	}
	return c, c.validate()
}

func (c Config) validate() error {
	if c.AppToken == "" || strings.Contains(c.AppToken, ":") {
		return errors.New("APP_TOKEN must be nonempty and contain no colon")
	}
	if c.APIKey == "" {
		return errors.New("AGNES_API_KEY is required")
	}
	if c.Model == "" {
		return errors.New("MODEL is required")
	}
	if c.FreeLimit < 0 || c.GlobalLimit < 0 {
		return errors.New("quota limits must not be negative")
	}
	return validateHTTPURL(c.BaseURL)
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("service URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	return nil
}

func limitEnv(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	// Redis Lua uses doubles; bound configuration to exact, operationally useful integers.
	if err != nil || n < 0 || n > 1_000_000_000 {
		return 0, fmt.Errorf("%s must be an integer between 0 and 1000000000", name)
	}
	return n, nil
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
