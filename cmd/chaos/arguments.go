package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

type launchOptions struct {
	dataDir string
	version bool
	files   []string
}

// Both launches use the sender's working directory. A second instance may have
// been opened from a different directory than the running desktop process.
func parseLaunch(args []string, workingDir string) (launchOptions, error) {
	var options launchOptions
	flags := flag.NewFlagSet("Chaos", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.dataDir, "data-dir", "", "use an isolated settings and history directory")
	flags.BoolVar(&options.version, "version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	resolve := func(path string) (string, error) {
		if filepath.IsAbs(path) {
			return filepath.Clean(path), nil
		}
		if !filepath.IsAbs(workingDir) {
			return "", fmt.Errorf("cannot resolve %q without the launch working directory", path)
		}
		return filepath.Join(workingDir, path), nil
	}
	if options.dataDir != "" {
		var err error
		options.dataDir, err = resolve(options.dataDir)
		if err != nil {
			return options, err
		}
	}
	for _, path := range flags.Args() {
		absolute, err := resolve(path)
		if err != nil {
			return options, err
		}
		options.files = append(options.files, absolute)
	}
	return options, nil
}

func openLaunchFiles(args []string, workingDir string, process func([]string)) error {
	options, err := parseLaunch(args, workingDir)
	if err != nil {
		return err
	}
	if !options.version && len(options.files) > 0 {
		process(options.files)
	}
	return nil
}
