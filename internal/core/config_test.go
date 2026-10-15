package core

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestUnknownProviderCannotReuseCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeTestFile(t, path, []byte(`{"provider":"openia","api_key":"wrong-provider-key","model":"other-model","base_url":"https://other.invalid/v1","provider_keys":{"openai":"saved-openai-key"}}`))
	cfg, err := LoadConfig(path, "")
	if err != nil {
		t.Fatal(err)
	}
	resolved := cfg.Resolve(t.TempDir())
	if err := resolved.Validate(); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("unknown provider was accepted: %+v, %v", resolved, err)
	}
	if cfg.Provider != "openia" || resolved.APIKey != "" || resolved.BaseURL != "" {
		t.Fatalf("unknown provider was silently replaced or exposed credentials: %+v", resolved)
	}
	cfg.SetProvider("openai")
	if cfg.APIKey != "saved-openai-key" || cfg.Model != "" || cfg.BaseURL != "" || cfg.ProviderKeys["siliconrouter"] != "" || cfg.ProviderKeys[""] != "" {
		t.Fatalf("repairing unknown provider reused unrelated settings: %+v", cfg)
	}
	if err := cfg.Resolve(t.TempDir()).Validate(); err != nil {
		t.Fatal("selecting a known provider did not repair configuration", err)
	}
}

func TestProviderDefaultAndSelectionPreserveCredentialOwnership(t *testing.T) {
	for _, id := range []string{"", "  ", "SILICONROUTER"} {
		cfg := Config{Provider: id, APIKey: "default-key"}
		resolved := cfg.Resolve(t.TempDir())
		if resolved.Provider.ID != "siliconrouter" || resolved.APIKey != "default-key" {
			t.Fatalf("default or normalized provider failed: %+v", resolved)
		}
	}
	cfg := Config{Provider: "openai", APIKey: "openai-key", Model: "selected-model"}
	cfg.SetProvider("unknown-selection")
	if cfg.Provider != "openai" || cfg.APIKey != "openai-key" || cfg.Model != "selected-model" {
		t.Fatalf("invalid selection changed an existing provider: %+v", cfg)
	}
}
