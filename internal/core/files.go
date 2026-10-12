package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// MoveError records a published destination if removing the original or syncing
// its directory fails. Callers can reconcile their durable journal on restart.
type MoveError struct {
	Source, Destination string
	Err                 error
}

func (e *MoveError) Error() string {
	return fmt.Sprintf("file was published at %s, but move did not finish: %v", e.Destination, e.Err)
}
func (e *MoveError) Unwrap() error { return e.Err }
func MoveFile(source, dir, base string) (string, error) {
	return MoveFileWithHook(source, dir, base, nil)
}
func MoveFileWithHook(source, dir, base string, before func(string) error) (string, error) {
	ext := strings.ToLower(filepath.Ext(source))
	if ext == "" {
		ext = ".png"
	}
	return moveCandidates(source, dir, Sanitize(base), ext, before)
}
func RenameFile(path, base string) (string, error) { return RenameFileWithHook(path, base, nil) }
func RenameFileWithHook(path, base string, before func(string) error) (string, error) {
	return moveCandidates(path, filepath.Dir(path), Sanitize(base), filepath.Ext(path), before)
}
func MoveToWithHook(source, target string, before func(string) error) (string, error) {
	return moveCandidates(source, filepath.Dir(target), strings.TrimSuffix(filepath.Base(target), filepath.Ext(target)), filepath.Ext(target), before)
}
func RevertFile(path, original string) (string, error) {
	return RevertFileWithHook(path, original, nil)
}
func RevertFileWithHook(path, original string, before func(string) error) (string, error) {
	if _, err := os.Lstat(original); errors.Is(err, os.ErrNotExist) {
		dst, err := moveExact(path, original, before)
		if !errors.Is(err, os.ErrExist) {
			return dst, err
		}
	}
	ext := filepath.Ext(original)
	return moveCandidates(path, filepath.Dir(original), strings.TrimSuffix(filepath.Base(original), ext)+"-restored", ext, before)
}
func moveCandidates(source, dir, base, ext string, before func(string) error) (string, error) {
	if base == "" || filepath.Base(base) != base || strings.ContainsAny(base, "/\\\x00") {
		return "", errors.New("invalid filename")
	}
	for i := 0; i < 100; i++ {
		name := base + ext
		if i > 0 {
			name = fmt.Sprintf("%s-%d%s", base, i+1, ext)
		}
		dst := filepath.Join(dir, name)
		if filepath.Clean(source) == filepath.Clean(dst) {
			return source, nil
		}
		if _, err := os.Lstat(dst); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		result, err := moveExact(source, dst, before)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return result, err
	}
	return "", errors.New("couldn't find a free filename after 100 attempts")
}

// moveExact supports cross-volume moves and never overwrites another file. A
// fully-written temporary inode is hard-linked into place atomically; unlike
// os.Rename, Link refuses to replace an existing destination on Unix.
func moveExact(source, destination string, before func(string) error) (string, error) {
	if filepath.Clean(source) == filepath.Clean(destination) {
		return source, nil
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("source must be a regular image file")
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, opened) {
		return "", errors.New("source changed while opening")
	}
	output, err := os.CreateTemp(filepath.Dir(destination), ".chaos-moving-*")
	if err != nil {
		return "", err
	}
	temp := output.Name()
	defer os.Remove(temp)
	count, copyErr := io.Copy(output, input)
	if copyErr == nil && count != info.Size() {
		copyErr = errors.New("source changed while copying")
	}
	if copyErr == nil {
		copyErr = output.Chmod(info.Mode().Perm())
	}
	if copyErr == nil {
		copyErr = output.Sync()
	}
	closeErr := output.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = os.Chtimes(temp, info.ModTime(), info.ModTime()); err != nil {
		return "", err
	}
	if err = copyMetadata(source, temp); err != nil {
		return "", err
	}
	current, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, current) || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return "", errors.New("source changed while copying")
	}
	if before != nil {
		if err = before(destination); err != nil {
			return "", err
		}
	}
	if err = publishFile(temp, destination); err != nil {
		return "", err
	}
	if err = syncDirectory(filepath.Dir(destination)); err != nil {
		return destination, &MoveError{source, destination, err}
	}
	// A user or journal callback can replace the original while copying. Never
	// unlink a different file that now happens to occupy the same pathname.
	current, err = os.Lstat(source)
	if err != nil || !sameFile(info, current) {
		if err == nil {
			err = errors.New("source changed before completing move; both files were preserved")
		}
		return destination, &MoveError{source, destination, err}
	}
	if err = os.Remove(source); err != nil {
		return destination, &MoveError{source, destination, err}
	}
	if err = syncDirectory(filepath.Dir(source)); err != nil {
		return destination, &MoveError{source, destination, err}
	}
	return destination, nil
}

func publishFile(temp, destination string) error {
	err := os.Link(temp, destination)
	if err == nil || (!errors.Is(err, syscall.EPERM) && !errors.Is(err, syscall.ENOTSUP) && !errors.Is(err, syscall.EXDEV)) {
		return err
	}
	return publishWithoutLinks(temp, destination)
}

// exFAT and some network shares cannot hard-link. O_EXCL still prevents any
// overwrite. The source stays intact until this copy is synced; a crash leaves
// both paths for the application's journal to reconcile by content hash.
func publishWithoutLinks(temp, destination string) error {
	input, err := os.Open(temp)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	reserved, err := output.Stat()
	if err != nil {
		output.Close()
		return err
	}
	success := false
	defer func() {
		if !success {
			if current, e := os.Lstat(destination); e == nil && os.SameFile(reserved, current) {
				_ = os.Remove(destination)
			}
		}
	}()
	_, err = io.Copy(output, input)
	if err == nil {
		err = output.Sync()
	}
	closeErr := output.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Chtimes(destination, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	if err = copyMetadata(temp, destination); err != nil {
		return err
	}
	success = true
	return nil
}
