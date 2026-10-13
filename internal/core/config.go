// Package core implements Chaos's portable application services. It has no UI dependency.
package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const DefaultTemplate = "{slug}_{time}"

type Provider struct {
	ID, Name, BaseURL, Model, Description, SignupURL string
	RequiresKey, CustomURL                           bool
}

func Providers() []Provider {
	return []Provider{
		{"siliconrouter", "SiliconRouter", "https://api.siliconrouter.com/v1", "gemini-3-flash-preview", "SiliconRouter hosted API.", "https://siliconrouter.com", true, false},
		{"openai", "OpenAI", "https://api.openai.com/v1", "gpt-4o-mini", "OpenAI hosted API.", "https://platform.openai.com/api-keys", true, false},
		{"deepseek", "DeepSeek", "https://api.deepseek.com", "deepseek-v4-flash", "DeepSeek hosted API.", "https://platform.deepseek.com/api_keys", true, false},
		{"openrouter", "OpenRouter", "https://openrouter.ai/api/v1", "openai/gpt-4o-mini", "OpenRouter hosted API.", "https://openrouter.ai/keys", true, false},
		{"openai-compatible", "OpenAI-Compatible", "", "gpt-4o-mini", "Custom OpenAI-compatible endpoint.", "", true, true},
		{"ollama", "Ollama", "http://localhost:11434/v1", "qwen3-vl:2b", "Local vision model served by Ollama.", "", false, false},
	}
}
func ProviderByID(id string) Provider {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, p := range Providers() {
		if p.ID == id {
			return p
		}
	}
	return Providers()[0]
}

type Config struct {
	Provider         string            `json:"provider,omitempty"`
	APIKey           string            `json:"api_key,omitempty"`
	ProviderKeys     map[string]string `json:"provider_keys,omitempty"`
	Model            string            `json:"model,omitempty"`
	BaseURL          string            `json:"base_url,omitempty"`
	WatchDir         string            `json:"watch_dir,omitempty"`
	OutputDir        string            `json:"output_dir,omitempty"`
	CopyToClipboard  bool              `json:"copy_to_clipboard,omitempty"`
	Language         string            `json:"language,omitempty"`
	FilenameTemplate string            `json:"filename_template,omitempty"`
	SubfolderRule    string            `json:"subfolder_rule,omitempty"`
	NotifyOnComplete bool              `json:"notify_on_complete,omitempty"`
	Appearance       string            `json:"appearance,omitempty"`
	UseCustomPrompt  bool              `json:"use_custom_prompt,omitempty"`
	CustomPrompt     string            `json:"custom_prompt,omitempty"`
}

type ResolvedConfig struct {
	Provider                                                                                                         Provider
	APIKey, Model, BaseURL, WatchDir, OutputDir, Language, FilenameTemplate, SubfolderRule, Appearance, CustomPrompt string
	CopyToClipboard, NotifyOnComplete                                                                                bool
}

func (c Config) Resolve(home string) ResolvedConfig {
	c = c.withoutRetiredProvider()
	p := ProviderByID(c.Provider)
	r := ResolvedConfig{Provider: p, Model: strings.TrimSpace(c.Model), BaseURL: p.BaseURL, WatchDir: strings.TrimSpace(c.WatchDir), OutputDir: strings.TrimSpace(c.OutputDir), Language: strings.ToLower(strings.TrimSpace(c.Language)), FilenameTemplate: strings.TrimSpace(c.FilenameTemplate), SubfolderRule: c.SubfolderRule, Appearance: c.Appearance, CopyToClipboard: c.CopyToClipboard, NotifyOnComplete: c.NotifyOnComplete}
	if p.RequiresKey {
		r.APIKey = strings.TrimSpace(c.APIKey)
	}
	if p.CustomURL {
		r.BaseURL = strings.TrimSpace(c.BaseURL)
	}
	if r.Model == "" {
		r.Model = p.Model
	}
	if r.WatchDir == "" {
		r.WatchDir = filepath.Join(home, "Desktop")
	}
	if r.OutputDir == "" {
		r.OutputDir = filepath.Join(home, "Desktop", "chaos-output")
	}
	r.WatchDir = expandHome(r.WatchDir, home)
	r.OutputDir = expandHome(r.OutputDir, home)
	if r.Language != "zh" {
		r.Language = "en"
	}
	if r.FilenameTemplate == "" {
		r.FilenameTemplate = DefaultTemplate
	}
	if r.SubfolderRule != "day" && r.SubfolderRule != "month" {
		r.SubfolderRule = "none"
	}
	if r.Appearance != "light" && r.Appearance != "dark" {
		r.Appearance = "system"
	}
	if c.UseCustomPrompt {
		r.CustomPrompt = strings.TrimSpace(c.CustomPrompt)
	}
	return r
}
func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return filepath.Clean(path)
}
func (r ResolvedConfig) Validate() error {
	if r.Provider.RequiresKey && strings.TrimSpace(r.APIKey) == "" {
		return errors.New("API key not configured")
	}
	if _, err := endpoint(r.BaseURL, "chat/completions"); err != nil {
		return err
	}
	if !filepath.IsAbs(r.WatchDir) || !filepath.IsAbs(r.OutputDir) {
		return errors.New("watch and output folders must be absolute paths")
	}
	return nil
}
func (c *Config) SetProvider(id string) {
	*c = c.withoutRetiredProvider()
	old := ProviderByID(c.Provider).ID
	next := ProviderByID(id).ID
	if old == next {
		return
	}
	keys := make(map[string]string, len(c.ProviderKeys)+1)
	for k, v := range c.ProviderKeys {
		keys[k] = v
	}
	if c.APIKey != "" {
		keys[old] = c.APIKey
	}
	c.ProviderKeys = keys
	c.Provider = next
	c.APIKey = keys[next]
	c.Model = ""
	c.BaseURL = ""
}

// withoutRetiredProvider resets the removed trial selection without carrying its
// model, endpoint or credentials over to a different provider.
func (c Config) withoutRetiredProvider() Config {
	if strings.EqualFold(strings.TrimSpace(c.Provider), "chaos-hosted") {
		c.Provider = Providers()[0].ID
		c.APIKey, c.Model, c.BaseURL = "", "", ""
	}
	if _, ok := c.ProviderKeys["chaos-hosted"]; ok {
		keys := make(map[string]string, len(c.ProviderKeys))
		for id, key := range c.ProviderKeys {
			if id != "chaos-hosted" {
				keys[id] = key
			}
		}
		c.ProviderKeys = keys
	}
	return c
}

func LoadConfig(path, legacyPath string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && legacyPath != "" {
		data, err = os.ReadFile(legacyPath)
		if err == nil {
			var cfg Config
			if err = json.Unmarshal(data, &cfg); err != nil {
				return Config{}, fmt.Errorf("read legacy configuration: %w", err)
			}
			if err = SaveConfig(path, cfg); err != nil {
				return Config{}, err
			}
			return cfg.withoutRetiredProvider(), nil
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg.withoutRetiredProvider(), nil
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("configuration is invalid (original preserved): %w", err)
	}
	return cfg.withoutRetiredProvider(), nil
}
func SaveConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg.withoutRetiredProvider(), "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, append(data, '\n'), 0600)
}

// AtomicWrite replaces one application-owned file without exposing partial contents.
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".chaos-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func endpoint(base, suffix string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid provider base URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + suffix
	return u.String(), nil
}

// boundedRead rejects a body exceeding its explicit memory budget.
func boundedRead(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e == nil && int64(len(b)) > limit {
		e = errors.New("response exceeds size limit")
	}
	return b, e
}
