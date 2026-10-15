package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (c *Controller) process(ctx context.Context, j job) {
	start := c.now()
	c.mu.Lock()
	cfg := c.snapshot.Config.Resolve(c.home)
	c.mu.Unlock()
	c.change(func(s *Snapshot) {
		s.Busy = true
		s.CurrentFile = filepath.Base(j.path)
		s.CurrentPath = j.path
		s.CurrentStage = "analyzing"
		s.BatchIndex = j.batchIndex
		s.BatchTotal = j.batchTotal
		s.Error = ""
		s.RecoveryAction = ""
		s.HourlyThroughput[start.Hour()]++
	})
	plan, err := c.processor.Prepare(ctx, j.path, cfg)
	if err != nil {
		c.failed(ctx, j, cfg, err)
		return
	}
	if err = ctx.Err(); err != nil {
		c.cancelled()
		return
	}
	record := core.RecentFile{
		ID:           core.NewID(),
		SourcePath:   j.path,
		OriginalName: filepath.Base(j.path),
		Timestamp:    c.now(),
	}
	op := operation{Kind: "insert", Record: record, Source: j.path, SHA256: plan.SourceSHA256}
	if err = plan.VerifySource(); err != nil {
		c.failed(ctx, j, cfg, err)
		return
	}
	c.change(func(s *Snapshot) { s.CurrentStage = "renaming" })
	destination, err := core.MoveFileWithHook(j.path, plan.Directory, plan.BaseName, func(destination string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := plan.VerifySource(); err != nil {
			return err
		}
		op.Destination = destination
		op.Record.Path = destination
		op.Record.NewName = filepath.Base(destination)
		op.Record.Duration = c.now().Sub(start).Seconds()
		return c.writeOperation(op)
	})
	if err != nil {
		// A published destination remains recoverable even when unlinking the
		// source or flushing the directory fails. Do not invent a failed row.
		if op.Destination != "" {
			if _, statErr := os.Stat(op.Destination); statErr == nil {
				c.change(func(s *Snapshot) {
					s.StorageError = "A filing needs recovery: " + err.Error()
					s.CurrentStage = "error"
				})
				return
			}
			_ = c.removeOperation(op.Record.ID)
		}
		c.failed(ctx, j, cfg, err)
		return
	}
	op.Record.Path = destination
	op.Record.NewName = filepath.Base(destination)
	op.Record.Duration = c.now().Sub(start).Seconds()
	if err = c.writeOperation(op); err == nil {
		err = c.commitOperation(op)
	}
	if err != nil {
		c.change(func(s *Snapshot) {
			s.StorageError = "The image was filed, but history needs recovery: " + err.Error()
			s.CurrentPath = destination
			s.CurrentFile = filepath.Base(destination)
			s.CurrentStage = "error"
		})
		return
	}
	c.change(func(s *Snapshot) {
		s.CurrentPath = destination
		s.CurrentFile = filepath.Base(destination)
		s.CurrentStage = "success"
		s.Error = ""
		s.Latencies = append(s.Latencies, op.Record.Duration)
		if len(s.Latencies) > 100 {
			s.Latencies = s.Latencies[len(s.Latencies)-100:]
		}
		s.SuccessWindow = append(s.SuccessWindow, 1)
		if len(s.SuccessWindow) > 100 {
			s.SuccessWindow = s.SuccessWindow[len(s.SuccessWindow)-100:]
		}
	})
	c.refresh()
	var warnings []string
	if cfg.CopyToClipboard && c.effects.CopyImage != nil {
		c.change(func(s *Snapshot) { s.CurrentStage = "clipboard" })
		if err := c.effects.CopyImage(destination); err != nil {
			warnings = append(warnings, "Image filed; clipboard copy failed: "+err.Error())
		}
	}
	if c.effects.Sound != nil {
		if err := c.effects.Sound(); err != nil {
			warnings = append(warnings, "Image filed; sound could not play.")
		}
	}
	if cfg.NotifyOnComplete && c.effects.Notify != nil {
		if err := c.effects.Notify("Screenshot filed", filepath.Base(j.path)+" → "+filepath.Base(destination)); err != nil {
			warnings = append(warnings, "Image filed; notification unavailable: "+err.Error())
		}
	}
	c.change(func(s *Snapshot) {
		s.CurrentStage = "success"
		if len(warnings) > 0 {
			s.Warning = strings.Join(warnings, "\n")
		}
	})
}

func (c *Controller) cancelled() {
	c.change(func(s *Snapshot) { s.CurrentStage = ""; s.CurrentFile = ""; s.CurrentPath = "" })
}

func (c *Controller) failed(ctx context.Context, j job, cfg core.ResolvedConfig, err error) {
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		c.cancelled()
		return
	}
	friendly := core.Friendly(err, cfg.Provider.ID)
	r := core.RecentFile{ID: core.NewID(), OriginalName: filepath.Base(j.path), SourcePath: j.path, Timestamp: c.now(), IsError: true, ErrorMessage: friendly.Message}
	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	writeErr := c.store.Insert(writeCtx, r)
	c.change(func(s *Snapshot) {
		s.Error = friendly.Message
		s.RecoveryAction = friendly.Action
		s.CurrentStage = "error"
		s.SuccessWindow = append(s.SuccessWindow, 0)
		if len(s.SuccessWindow) > 100 {
			s.SuccessWindow = s.SuccessWindow[len(s.SuccessWindow)-100:]
		}
		if writeErr != nil {
			s.StorageError = "Save failed attempt: " + writeErr.Error()
		}
	})
	if writeErr == nil {
		c.refresh()
	}
	if cfg.NotifyOnComplete && c.effects.Notify != nil {
		if e := c.effects.Notify("Could not file screenshot", r.OriginalName+": "+friendly.Message); e != nil {
			c.ReportWarning("Notification unavailable: " + e.Error())
		}
	}
}

func (c *Controller) mutate(ctx context.Context, j job) {
	r, ok := c.findRecord(j.id)
	if !ok {
		c.report(errors.New("that history entry is no longer available"))
		return
	}
	if r.IsError || r.Path == "" {
		return
	}
	if ctx.Err() != nil {
		return
	}
	c.change(func(s *Snapshot) { s.Busy = true })
	op := operation{Kind: "update", Record: r, Source: r.Path}
	if j.kind == "revert" {
		op.Kind = "delete"
	}
	if err := op.fingerprint(); err != nil {
		c.report(fmt.Errorf("that image is no longer where Chaos left it: %w", err))
		return
	}
	hook := func(destination string) error {
		op.Destination = destination
		op.Record.Path = destination
		op.Record.NewName = filepath.Base(destination)
		// Suppress the restored filename before publication, so Undo never
		// triggers the watcher to immediately file it again.
		if j.kind == "revert" {
			c.watchMu.Lock()
			if c.watcher != nil {
				c.watcher.Suppress(destination)
			}
			c.watchMu.Unlock()
		}
		return c.writeOperation(op)
	}
	var destination string
	var err error
	if j.kind == "revert" {
		destination, err = core.RevertFileWithHook(r.Path, r.SourcePath, hook)
	} else {
		if strings.TrimSpace(j.name) == "" {
			return
		}
		destination, err = core.RenameFileWithHook(r.Path, j.name, hook)
	}
	if err != nil {
		c.report(err)
		return
	}
	if destination == r.Path {
		if j.kind == "revert" {
			writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.store.Delete(writeCtx, r.ID)
			cancel()
			if err != nil {
				c.report(err)
			} else {
				c.refresh()
			}
		}
		return
	}
	op.Destination = destination
	op.Record.Path = destination
	op.Record.NewName = filepath.Base(destination)
	if err = c.commitOperation(op); err != nil {
		c.change(func(s *Snapshot) { s.StorageError = "History change needs recovery: " + err.Error() })
		return
	}
	c.change(func(s *Snapshot) { s.Error = ""; s.CurrentStage = ""; s.CurrentFile = ""; s.CurrentPath = "" })
	c.refresh()
}
