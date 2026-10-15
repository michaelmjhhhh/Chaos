package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

// Preferences keeps application lifecycle choices separate from provider settings.
// On macOS the first Go launch imports the Swift application's UserDefaults.
type Preferences struct {
	AutoStart          bool `json:"auto_start"`
	OnboardingComplete bool `json:"onboarding_complete"`
	SessionNumber      int  `json:"session_number"`
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func loadPreferences(path string, legacy func() Preferences) (Preferences, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if legacy != nil {
			return legacy(), nil
		}
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, err
	}
	var p Preferences
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("read preferences: %w", err)
	}
	return p, nil
}

func savePreferences(path string, p Preferences) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return core.AtomicWrite(path, append(b, '\n'), 0600)
}

func legacyPreferences(run commandRunner) Preferences {
	if runtime.GOOS != "darwin" {
		return Preferences{}
	}
	read := func(key string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		b, err := run(ctx, "/usr/bin/defaults", "read", "com.chaos.app", key)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	p := Preferences{}
	p.AutoStart = read("autoStart") == "1"
	p.OnboardingComplete = read("hasCompletedOnboarding") == "1"
	p.SessionNumber, _ = strconv.Atoi(read("chaos.sessionNumber"))
	return p
}
