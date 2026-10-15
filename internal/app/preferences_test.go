package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLegacyPreferencesImportsOnceAndPreservesSession(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("UserDefaults migration is macOS-only")
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	imported := 0
	importer := func() Preferences {
		imported++
		return legacyPreferences(func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "/usr/bin/defaults" || len(args) != 3 || args[0] != "read" || args[1] != "com.chaos.app" {
				t.Fatalf("unexpected legacy command %q %v", name, args)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("legacy read has no timeout")
			}
			switch args[2] {
			case "autoStart", "hasCompletedOnboarding":
				return []byte("1\n"), nil
			case "chaos.sessionNumber":
				return []byte("41\n"), nil
			default:
				t.Fatalf("unexpected key %q", args[2])
				return nil, errors.New("unknown key")
			}
		})
	}
	p, err := loadPreferences(path, importer)
	if err != nil || !p.AutoStart || !p.OnboardingComplete || p.SessionNumber != 41 {
		t.Fatalf("legacy values: %+v %v", p, err)
	}
	p.SessionNumber++
	if err := savePreferences(path, p); err != nil {
		t.Fatal(err)
	}
	again, err := loadPreferences(path, importer)
	if err != nil || again != p || imported != 1 {
		t.Fatalf("preferences reimported: %+v %v imports=%d", again, err, imported)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("preferences permissions: %o", info.Mode().Perm())
	}
	c, err := New(Options{DataDir: filepath.Dir(path), Home: filepath.Dir(path)})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := loadPreferences(path, nil)
	if err != nil || persisted.SessionNumber != 43 || !persisted.AutoStart || !persisted.OnboardingComplete {
		t.Fatalf("session persistence: %+v %v", persisted, err)
	}
}

func TestLegacyPreferencesMissingAndMalformedValues(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("UserDefaults migration is macOS-only")
	}
	p := legacyPreferences(func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("missing") })
	if p != (Preferences{}) {
		t.Fatalf("missing values: %+v", p)
	}
	p = legacyPreferences(func(context.Context, string, ...string) ([]byte, error) { return []byte("invalid"), nil })
	if p != (Preferences{}) {
		t.Fatalf("malformed values: %+v", p)
	}
}
