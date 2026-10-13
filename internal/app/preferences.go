package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	return atomicWrite(path, append(b, '\n'))
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".chaos-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// A successful return means both the file and its directory entry were flushed.
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	if runtime.GOOS == "windows" {
		return nil
	}
	return d.Sync()
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

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6], b[8] = (b[6]&0x0f)|0x40, (b[8]&0x3f)|0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]))
}
