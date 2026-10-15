package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
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
	SourceSHA256        string
	sourceInfo          os.FileInfo
}

// VerifySource binds the pending move to the image actually analyzed, rather
// than whichever file occupies its pathname after a remote model responds.
func (p Plan) VerifySource() error {
	info, hash, err := fingerprintSource(p.Record.SourcePath)
	if err != nil {
		return err
	}
	if !sameFile(p.sourceInfo, info) || p.SourceSHA256 == "" || hash != p.SourceSHA256 {
		return errors.New("source image changed during analysis; the replacement was preserved")
	}
	return nil
}

func fingerprintSource(path string) (os.FileInfo, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", errors.New("source must be a regular image file")
	}
	if info.Size() > MaxImageBytes {
		return nil, "", errors.New("image exceeds the 100 MiB limit")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	if !sameFile(info, opened) {
		return nil, "", errors.New("source image changed while opening")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, MaxImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if n != info.Size() || !sameFile(info, after) {
		return nil, "", errors.New("source image changed while reading")
	}
	return info, hex.EncodeToString(hash.Sum(nil)), nil
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
	info, fingerprint, err := fingerprintSource(source)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Record: RecentFile{SourcePath: source}, SourceSHA256: fingerprint, sourceInfo: info}
	prepare := p.PrepareImage
	if prepare == nil {
		prepare = PrepareImage
	}
	image, err := prepare(ctx, source)
	if err != nil {
		return Plan{}, err
	}
	if err = plan.VerifySource(); err != nil {
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
	if err = plan.VerifySource(); err != nil {
		return Plan{}, err
	}
	at := now()
	plan.Record = RecentFile{ID: NewID(), OriginalName: filepath.Base(source), SourcePath: source, Timestamp: at, Duration: at.Sub(start).Seconds()}
	plan.Directory = OutputDirectory(cfg.OutputDir, cfg.SubfolderRule, at)
	plan.BaseName = RenderName(cfg.FilenameTemplate, Sanitize(slug), at)
	return plan, nil
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
	if err = plan.VerifySource(); err != nil {
		return RecentFile{}, err
	}
	dst, err := MoveFileWithHook(source, plan.Directory, plan.BaseName, func(string) error { return plan.VerifySource() })
	if err != nil {
		return RecentFile{}, err
	}
	plan.Record.Path = dst
	plan.Record.NewName = filepath.Base(dst)
	return plan.Record, nil
}

var ErrImageNotStable = errors.New("image did not stabilize before processing")

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
	return ErrImageNotStable
}
