package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

// The journal is written and fsynced before a no-clobber move publishes its
// destination. Recovery only completes operations whose bytes match that intent.
type operation struct {
	Kind        string          `json:"kind"`
	Record      core.RecentFile `json:"record"`
	Source      string          `json:"source"`
	Destination string          `json:"destination"`
	SHA256      string          `json:"sha256"`
}

func digestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("expected a regular image file")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (o *operation) fingerprint() error {
	var err error
	o.SHA256, err = digestFile(o.Source)
	return err
}
func (c *Controller) journalPath(id string) string {
	return filepath.Join(c.dataDir, "pending", id+".json")
}
func (c *Controller) writeOperation(o operation) error {
	if o.Record.ID == "" || strings.ContainsAny(o.Record.ID, "/\\") {
		return errors.New("invalid operation ID")
	}
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return atomicWrite(c.journalPath(o.Record.ID), b)
}
func (c *Controller) removeOperation(id string) error {
	err := os.Remove(c.journalPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (c *Controller) commitOperation(o operation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var err error
	switch o.Kind {
	case "insert":
		err = c.store.Insert(ctx, o.Record)
	case "update":
		err = c.store.Update(ctx, o.Record.ID, o.Record.NewName, o.Record.Path)
	case "delete":
		err = c.store.Delete(ctx, o.Record.ID)
	default:
		return errors.New("unknown pending history operation")
	}
	if err != nil {
		return err
	}
	return c.removeOperation(o.Record.ID)
}

func (c *Controller) recoverOperations() error {
	files, err := filepath.Glob(filepath.Join(c.dataDir, "pending", "*.json"))
	if err != nil {
		return err
	}
	var failures []error
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var o operation
		if err = json.Unmarshal(b, &o); err != nil {
			failures = append(failures, fmt.Errorf("recover %s: %w", filepath.Base(path), err))
			continue
		}
		if !filepath.IsAbs(o.Source) || !filepath.IsAbs(o.Destination) || o.SHA256 == "" || o.Record.ID == "" || c.journalPath(o.Record.ID) != path {
			failures = append(failures, fmt.Errorf("invalid recovery record %s; original files preserved", filepath.Base(path)))
			continue
		}
		if filepath.Clean(o.Source) == filepath.Clean(o.Destination) {
			failures = append(failures, fmt.Errorf("invalid same-location recovery of %s; file preserved", filepath.Base(o.Source)))
			continue
		}
		destinationDigest, dstErr := digestFile(o.Destination)
		sourceDigest, srcErr := digestFile(o.Source)
		if errors.Is(dstErr, os.ErrNotExist) && srcErr == nil {
			// The app stopped before publication: the source is still intact.
			if sourceDigest == o.SHA256 {
				_ = c.removeOperation(o.Record.ID)
			} else {
				failures = append(failures, fmt.Errorf("source changed during interrupted filing: %s", filepath.Base(o.Source)))
			}
			continue
		}
		if dstErr != nil || destinationDigest != o.SHA256 {
			failures = append(failures, fmt.Errorf("cannot verify interrupted filing of %s; both locations preserved", filepath.Base(o.Source)))
			continue
		}
		if srcErr == nil {
			if sourceDigest != o.SHA256 {
				failures = append(failures, fmt.Errorf("source was replaced after filing %s; no files removed", filepath.Base(o.Source)))
				continue
			}
			if err = os.Remove(o.Source); err != nil {
				failures = append(failures, err)
				continue
			}
		} else if !errors.Is(srcErr, os.ErrNotExist) {
			failures = append(failures, srcErr)
			continue
		}
		o.Record.Path = o.Destination
		o.Record.NewName = filepath.Base(o.Destination)
		if err = c.commitOperation(o); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
