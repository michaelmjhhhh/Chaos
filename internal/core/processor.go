package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Processor struct {
	Client       *Client
	PrepareImage ImagePreparer
	Now          func() time.Time
}
type Plan struct {
	Record              RecentFile
	Directory, BaseName string
}

func (p *Processor) Prepare(ctx context.Context, source string, cfg ResolvedConfig) (Plan, error) {
	now := p.Now
	if now == nil {
		now = time.Now
	}
	start := now()
	if !AcceptsImage(source) {
		return Plan{}, errors.New("unsupported image type")
	}
	if err := cfg.Validate(); err != nil {
		return Plan{}, err
	}
	if err := WaitUntilStable(ctx, source, 12, 100*time.Millisecond); err != nil {
		return Plan{}, err
	}
	prepare := p.PrepareImage
	if prepare == nil {
		prepare = PrepareImage
	}
	image, err := prepare(ctx, source)
	if err != nil {
		return Plan{}, err
	}
	client := p.Client
	if client == nil {
		client = &Client{}
	}
	slug, err := client.GenerateSlug(ctx, image, cfg)
	if err != nil {
		return Plan{}, err
	}
	at := now()
	return Plan{Record: RecentFile{ID: NewID(), OriginalName: filepath.Base(source), SourcePath: source, Timestamp: at, Duration: at.Sub(start).Seconds()}, Directory: OutputDirectory(cfg.OutputDir, cfg.SubfolderRule, at), BaseName: RenderName(cfg.FilenameTemplate, Sanitize(slug), at)}, nil
}
func (p *Processor) Process(ctx context.Context, source string, cfg ResolvedConfig, onStage func(string)) (RecentFile, error) {
	if onStage != nil {
		onStage("analyzing")
	}
	plan, err := p.Prepare(ctx, source, cfg)
	if err != nil {
		return RecentFile{}, err
	}
	if err = ctx.Err(); err != nil {
		return RecentFile{}, err
	}
	if onStage != nil {
		onStage("renaming")
	}
	dst, err := MoveFile(source, plan.Directory, plan.BaseName)
	if err != nil {
		return RecentFile{}, err
	}
	plan.Record.Path = dst
	plan.Record.NewName = filepath.Base(dst)
	return plan.Record, nil
}
func WaitUntilStable(ctx context.Context, path string, attempts int, interval time.Duration) error {
	var previous os.FileInfo
	for i := 0; i < attempts; i++ {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("source must be a regular image file")
		}
		if previous != nil && info.Size() > 0 && os.SameFile(previous, info) && previous.Size() == info.Size() && previous.ModTime().Equal(info.ModTime()) {
			return nil
		}
		previous = info
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("image did not stabilize before processing")
}
