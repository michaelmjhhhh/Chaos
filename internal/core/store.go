package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

const historyColumns = `id, originalName, newName, path, sourcePath, CAST(timestamp AS TEXT), duration, isError, COALESCE(errorMessage, '')`

func OpenStore(path, legacyJSON string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	ok := false
	defer func() {
		if !ok {
			db.Close()
		}
	}()
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
 CREATE TABLE IF NOT EXISTS processed_images (
 id TEXT PRIMARY KEY, originalName TEXT NOT NULL, newName TEXT NOT NULL,
 path TEXT NOT NULL, sourcePath TEXT NOT NULL, timestamp DATETIME NOT NULL,
 duration DOUBLE NOT NULL, isError BOOLEAN NOT NULL, errorMessage TEXT, ext TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_processed_images_timestamp ON processed_images(timestamp);`)
	if err != nil {
		return nil, fmt.Errorf("open history: %w", err)
	}
	// GRDB's migration metadata is left intact. Existing data is read in place.
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	if legacyJSON != "" {
		if err = store.importLegacy(legacyJSON); err != nil {
			return nil, err
		}
	}
	ok = true
	return store, nil
}
func (s *Store) Close() error { return s.db.Close() }
func recordArgs(f RecentFile) []any {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.NewName), "."))
	if ext == "" {
		ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(f.OriginalName), "."))
	}
	var errorMessage any
	if f.IsError {
		errorMessage = f.ErrorMessage
	}
	return []any{f.ID, f.OriginalName, f.NewName, f.Path, f.SourcePath, f.Timestamp.UTC().Format("2006-01-02 15:04:05.000"), f.Duration, f.IsError, errorMessage, ext}
}

const insertSQL = `INSERT INTO processed_images(id,originalName,newName,path,sourcePath,timestamp,duration,isError,errorMessage,ext) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`

func (s *Store) Insert(ctx context.Context, f RecentFile) error {
	if f.ID == "" {
		return errors.New("history record needs an identifier")
	}
	if math.IsNaN(f.Duration) || math.IsInf(f.Duration, 0) {
		return errors.New("invalid processing duration")
	}
	_, err := s.db.ExecContext(ctx, insertSQL, recordArgs(f)...)
	return err
}
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM processed_images WHERE id=?`, id)
	return err
}
func (s *Store) Update(ctx context.Context, id, newName, path string) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(newName), "."))
	result, err := s.db.ExecContext(ctx, `UPDATE processed_images SET newName=?,path=?,ext=? WHERE id=?`, newName, path, ext, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return errors.New("history record no longer exists")
	}
	return err
}
func (s *Store) Recent(ctx context.Context, limit int) ([]RecentFile, error) {
	if limit <= 0 {
		return []RecentFile{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+historyColumns+` FROM processed_images ORDER BY timestamp DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readRecords(rows)
}
func readRecords(rows *sql.Rows) ([]RecentFile, error) {
	files := []RecentFile{}
	for rows.Next() {
		var f RecentFile
		var timestamp string
		if err := rows.Scan(&f.ID, &f.OriginalName, &f.NewName, &f.Path, &f.SourcePath, &timestamp, &f.Duration, &f.IsError, &f.ErrorMessage); err != nil {
			return nil, err
		}
		var err error
		f.Timestamp, err = parseTimestamp(timestamp)
		if err != nil {
			return nil, fmt.Errorf("history row %s: %w", f.ID, err)
		}
		if f.IsError && f.ErrorMessage == "" {
			f.ErrorMessage = "Unknown error"
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
func parseTimestamp(raw string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized history timestamp %q", raw)
}
func (s *Store) importLegacy(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	data, err := boundedRead(file, 100<<20)
	file.Close()
	if err != nil {
		return err
	}
	var raw []struct {
		ID, OriginalName, NewName, Path, SourcePath string
		Timestamp                                   *float64
		Duration                                    float64
		Result                                      json.RawMessage
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("legacy history is invalid; original preserved: %w", err)
	}
	if raw == nil {
		return errors.New("legacy history must be an array; original preserved")
	}
	records := make([]RecentFile, 0, len(raw))
	for _, v := range raw {
		if v.ID == "" || v.Timestamp == nil || math.IsNaN(*v.Timestamp) || math.IsInf(*v.Timestamp, 0) {
			return errors.New("legacy history has invalid identity/date; original preserved")
		}
		seconds, fraction := math.Modf(*v.Timestamp + 978307200)
		f := RecentFile{ID: v.ID, OriginalName: v.OriginalName, NewName: v.NewName, Path: v.Path, SourcePath: v.SourcePath, Timestamp: time.Unix(int64(seconds), int64(math.Round(fraction*1e9))).UTC(), Duration: v.Duration}
		var result map[string]json.RawMessage
		if err = json.Unmarshal(v.Result, &result); err != nil {
			return fmt.Errorf("invalid legacy result; original preserved: %w", err)
		}
		if len(result) != 1 {
			return errors.New("ambiguous legacy result; original preserved")
		}
		if e, ok := result["error"]; ok {
			var reason struct {
				Message string `json:"_0"`
			}
			if err = json.Unmarshal(e, &reason); err != nil {
				return err
			}
			f.IsError = true
			f.ErrorMessage = reason.Message
		} else if _, ok := result["success"]; !ok {
			return errors.New("unknown legacy result; original preserved")
		}
		records = append(records, f)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range records {
		if _, err = tx.Exec(insertSQL, recordArgs(f)...); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// Never replace an earlier migration backup. Re-opening after a crash remains
	// idempotent because imports preserve their original primary keys.
	archive := path + ".migrated"
	for n := 2; ; n++ {
		if _, err = os.Lstat(archive); errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
		archive = fmt.Sprintf("%s.migrated.%d", path, n)
	}
	if err = os.Link(path, archive); err != nil {
		return fmt.Errorf("archive imported legacy history: %w", err)
	}
	if err = syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
