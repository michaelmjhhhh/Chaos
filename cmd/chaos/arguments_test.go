package main

import (
	"flag"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSecondInstanceIntakeUsesSenderDirectoryAndFlags(t *testing.T) {
	wd := t.TempDir()
	abs := filepath.Join(t.TempDir(), "absolute.png")
	args := []string{"--data-dir", "preview", "relative.png", abs}
	want := []string{filepath.Join(wd, "relative.png"), abs}
	var got []string
	if err := openLaunchFiles(args, wd, func(paths []string) { got = paths }); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("second launch enqueued %q, want %q", got, want)
	}
	startup, err := parseLaunch(args, wd)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(startup.files, got) || startup.dataDir != filepath.Join(wd, "preview") {
		t.Fatalf("startup and second launch disagree: %#v; enqueued %q", startup, got)
	}
}

func TestLaunchArgumentErrorsDoNotEnqueueFiles(t *testing.T) {
	for _, args := range [][]string{{"--unknown", "image.png"}, {"--data-dir"}} {
		called := false
		if err := openLaunchFiles(args, t.TempDir(), func([]string) { called = true }); err == nil || called {
			t.Fatalf("invalid args %q: err=%v dispatched=%v", args, err, called)
		}
	}
	for _, args := range [][]string{nil, {"--version"}} {
		if err := openLaunchFiles(args, t.TempDir(), func([]string) { t.Fatal("empty/version launch enqueued files") }); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := parseLaunch([]string{"--help"}, t.TempDir()); err != flag.ErrHelp {
		t.Fatalf("help error = %v", err)
	}
}

func TestLaunchExplicitEndOfFlagsAndMissingWorkingDirectory(t *testing.T) {
	wd := t.TempDir()
	options, err := parseLaunch([]string{"--", "-screenshot.png"}, wd)
	if err != nil || len(options.files) != 1 || options.files[0] != filepath.Join(wd, "-screenshot.png") {
		t.Fatalf("end of flags: %#v, %v", options, err)
	}
	if _, err := parseLaunch([]string{"relative.png"}, ""); err == nil {
		t.Fatal("relative path silently used the running process's working directory")
	}
}
