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

type operationKind string

const (
	operationInsert operationKind = "insert"
	operationUpdate operationKind = "update"
	operationDelete operationKind = "delete"
)

func (k operationKind) valid() bool {
	switch k {
	case operationInsert, operationUpdate, operationDelete:
		return true
	default:
		return false
	}
}

// The journal is written and fsynced before a no-clobber move publishes its
// destination. Recovery only completes operations whose bytes match that intent.
type operation struct {
	Kind        operationKind   `json:"kind"`
	Record      core.RecentFile `json:"record"`
	Source      string          `json:"source"`
	Destination string          `json:"destination"`
	SHA256      string          `json:"sha256"`
}

type recoveryFile struct {
	digest string
	info   os.FileInfo
}

func sameRecoveryFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && a.Mode().IsRegular() && b.Mode().IsRegular() &&
		os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (f recoveryFile) verifyEntry(path string) error {
	current, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !sameRecoveryFile(f.info, current) {
		return errors.New("image changed during recovery verification")
	}
	return nil
}

func (f recoveryFile) verifyCurrent(path string) error {
	current, err := readRecoveryFile(path)
	if err != nil {
		return err
	}
	if !sameRecoveryFile(f.info, current.info) || f.digest != current.digest {
		return errors.New("image identity or content changed during recovery verification")
	}
	return nil
}

func readRecoveryFile(path string) (recoveryFile, error) {
	// Stat on the opened handle follows symlinks. Inspect the directory entry
	// first so a destination symlink cannot masquerade as a completed copy.
	info, err := os.Lstat(path)
	if err != nil {
		return recoveryFile{}, err
	}
	if !info.Mode().IsRegular() {
		return recoveryFile{}, errors.New("expected a regular image file, not a link or directory")
	}
	f, err := os.Open(path)
	if err != nil {
		return recoveryFile{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return recoveryFile{}, err
	}
	if !sameRecoveryFile(info, opened) {
		return recoveryFile{}, errors.New("image changed while opening for recovery")
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return recoveryFile{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return recoveryFile{}, err
	}
	if n != info.Size() || !sameRecoveryFile(info, after) {
		return recoveryFile{}, errors.New("image changed while reading for recovery")
	}
	result := recoveryFile{digest: hex.EncodeToString(h.Sum(nil)), info: info}
	if err := result.verifyEntry(path); err != nil {
		return recoveryFile{}, err
	}
	return result, nil
}

func digestFile(path string) (string, error) {
	f, err := readRecoveryFile(path)
	return f.digest, err
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
	if !o.Kind.valid() {
		return errors.New("unknown pending history operation")
	}
	if o.Record.ID == "" || strings.ContainsAny(o.Record.ID, "/\\") {
		return errors.New("invalid operation ID")
	}
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return core.AtomicWrite(c.journalPath(o.Record.ID), b, 0600)
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
	case operationInsert:
		err = c.store.Insert(ctx, o.Record)
	case operationUpdate:
		err = c.store.Update(ctx, o.Record.ID, o.Record.NewName, o.Record.Path)
	case operationDelete:
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
		if !o.Kind.valid() || !filepath.IsAbs(o.Source) || !filepath.IsAbs(o.Destination) || o.SHA256 == "" || o.Record.ID == "" || c.journalPath(o.Record.ID) != path {
			failures = append(failures, fmt.Errorf("invalid recovery record %s; original files preserved", filepath.Base(path)))
			continue
		}
		if filepath.Clean(o.Source) == filepath.Clean(o.Destination) {
			failures = append(failures, fmt.Errorf("invalid same-location recovery of %s; file preserved", filepath.Base(o.Source)))
			continue
		}
		destination, dstErr := readRecoveryFile(o.Destination)
		source, srcErr := readRecoveryFile(o.Source)
		if errors.Is(dstErr, os.ErrNotExist) && srcErr == nil {
			// The app stopped before publication: the source is still intact.
			if source.digest == o.SHA256 {
				_ = c.removeOperation(o.Record.ID)
			} else {
				failures = append(failures, fmt.Errorf("source changed during interrupted filing: %s", filepath.Base(o.Source)))
			}
			continue
		}
		if dstErr != nil || destination.digest != o.SHA256 {
			failures = append(failures, fmt.Errorf("cannot verify interrupted filing of %s; both locations preserved", filepath.Base(o.Source)))
			continue
		}
		if srcErr == nil {
			// Different path spellings may resolve through parent-directory
			// aliases to the same entry. Even a hard link is ambiguous here:
			// normal moves publish a new inode, so retain both paths for review.
			if os.SameFile(source.info, destination.info) {
				failures = append(failures, fmt.Errorf("recovery locations refer to the same file: %s; no files removed", filepath.Base(o.Source)))
				continue
			}
			if source.digest != o.SHA256 {
				failures = append(failures, fmt.Errorf("source was replaced after filing %s; no files removed", filepath.Base(o.Source)))
				continue
			}
			// Hashing either file can take time. Recheck source bytes and
			// identity, then the retained destination before removing the source.
			if err := source.verifyCurrent(o.Source); err != nil {
				failures = append(failures, fmt.Errorf("cannot verify interrupted source; files preserved: %w", err))
				continue
			}
			if err := destination.verifyCurrent(o.Destination); err != nil {
				failures = append(failures, fmt.Errorf("cannot verify interrupted destination; files preserved: %w", err))
				continue
			}
			// Rehashing the destination can take time too. Reject a source
			// replacement observed since its content was verified.
			if err := source.verifyEntry(o.Source); err != nil {
				failures = append(failures, fmt.Errorf("source changed before completing recovery; files preserved: %w", err))
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
		if err := destination.verifyCurrent(o.Destination); err != nil {
			failures = append(failures, fmt.Errorf("destination changed before recording recovery: %w", err))
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
