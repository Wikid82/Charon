package dbmaint

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Settings rows of the maintenance feature. All share the reserved
// "maintenance." prefix the settings API never exposes or accepts.
const (
	// SettingKeyFlag is the user's "reclaim space on next restart" request.
	SettingKeyFlag = "maintenance.compact_requested"

	keyAttempts   = "maintenance.attempts"
	keyInProgress = "maintenance.in_progress"
	keyLastResult = "maintenance.last_result"

	settingsCategory = "maintenance"
)

// Result is the recorded outcome of a maintenance run.
type Result string

// Run results. Converted and ConvertedPendingCheckpoint mean the database is in
// incremental mode; the latter means the file has not shrunk yet.
const (
	ResultConverted                  Result = "converted"
	ResultConvertedPendingCheckpoint Result = "converted_pending_checkpoint"
	ResultInterrupted                Result = "interrupted"
	ResultFailed                     Result = "failed"
	ResultSkipped                    Result = "skipped"
	// ResultCancelled means the application stopped before any work started;
	// it is reported to the caller but not persisted.
	ResultCancelled Result = "cancelled"
)

// LastResult is the persisted outcome of the latest run for one database file.
type LastResult struct {
	At          time.Time `json:"at"`
	Outcome     Result    `json:"outcome"`
	Reason      Reason    `json:"reason"`
	BytesBefore int64     `json:"bytes_before"`
	BytesAfter  int64     `json:"bytes_after"`
	FileID      string    `json:"file_id,omitempty"`
}

// State is the persisted maintenance state that applies to the current file.
type State struct {
	FlagRequested bool
	// Attempts counts consecutive failed conversions, including a leftover
	// in-progress marker of an earlier boot.
	Attempts   int
	LastResult *LastResult
}

type attemptsRecord struct {
	Count  int    `json:"count"`
	FileID string `json:"file_id"`
}

type markerRecord struct {
	FileID string    `json:"file_id"`
	Boot   time.Time `json:"boot"`
}

// SQLExecer is the subset of *sql.DB and *sql.Conn the store needs. Writing
// through a pinned *sql.Conn avoids waiting for the pool's only connection.
type SQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store reads and writes the maintenance rows of the settings table.
type Store struct {
	x SQLExecer
}

// NewStore returns a store that uses x.
func NewStore(x SQLExecer) *Store { return &Store{x: x} }

// FileID identifies the database file by its inode number only. The device
// number is not stable on every setup (overlayfs, btrfs subvolumes, NFS), so
// comparing it would silently discard state on every boot. An in-place VACUUM
// keeps the inode; a file renamed into place gets a new one.
func FileID(dbPath string) (string, error) {
	info, err := os.Stat(filepath.Clean(dbPath))
	if err != nil {
		return "", fmt.Errorf("stat database file: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("database file identity is not available on this platform")
	}
	return strconv.FormatUint(uint64(st.Ino), 10), nil //nolint:gosec // G115: st.Ino is unsigned on linux; conversion is lossless
}

// inodeOf returns the inode part of a stored file id, which is either "ino" or
// the legacy "dev:ino".
func inodeOf(fileID string) string {
	if i := strings.LastIndex(fileID, ":"); i >= 0 {
		return fileID[i+1:]
	}
	return fileID
}

// sameFile compares inode numbers only: a device-only difference keeps state.
func sameFile(stored, current string) bool { return inodeOf(stored) == inodeOf(current) }

const upsertSQL = `INSERT INTO settings ("key", value, type, category, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT("key") DO UPDATE SET value = excluded.value, type = excluded.type, category = excluded.category, updated_at = excluded.updated_at`

func (s *Store) put(ctx context.Context, key, value, typ string) error {
	if _, err := s.x.ExecContext(ctx, upsertSQL, key, value, typ, settingsCategory, time.Now().UTC()); err != nil {
		return fmt.Errorf("write setting %s: %w", key, err)
	}
	return nil
}

func (s *Store) putJSON(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode setting %s: %w", key, err)
	}
	return s.put(ctx, key, string(raw), "json")
}

func (s *Store) remove(ctx context.Context, key string) error {
	if _, err := s.x.ExecContext(ctx, `DELETE FROM settings WHERE "key" = ?`, key); err != nil {
		return fmt.Errorf("delete setting %s: %w", key, err)
	}
	return nil
}

// get returns the raw value of key and whether the row exists.
func (s *Store) get(ctx context.Context, key string) (value string, found bool, err error) {
	err = s.x.QueryRowContext(ctx, `SELECT value FROM settings WHERE "key" = ?`, key).Scan(&value)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("read setting %s: %w", key, err)
	}
	return value, true, nil
}

// getJSON decodes key into v. A row that does not decode is deleted and
// reported as not found, so corrupt state can never block the boot path.
func (s *Store) getJSON(ctx context.Context, key string, v any) (found bool, err error) {
	raw, found, err := s.get(ctx, key)
	if err != nil || !found {
		return false, err
	}
	if jsonErr := json.Unmarshal([]byte(raw), v); jsonErr != nil {
		return false, s.remove(ctx, key)
	}
	return true, nil
}

// FlagRequested reports whether the user asked to reclaim space on next restart.
func (s *Store) FlagRequested(ctx context.Context) (bool, error) {
	raw, found, err := s.get(ctx, SettingKeyFlag)
	if err != nil {
		return false, err
	}
	return found && raw == "true", nil
}

// SetFlag records the request. It is idempotent.
func (s *Store) SetFlag(ctx context.Context) error { return s.put(ctx, SettingKeyFlag, "true", "bool") }

// ClearFlag removes the request. It is idempotent.
func (s *Store) ClearFlag(ctx context.Context) error { return s.remove(ctx, SettingKeyFlag) }

// SetInProgress writes the marker that a conversion started. A marker found at
// the next boot means the process was killed mid-conversion.
func (s *Store) SetInProgress(ctx context.Context, fileID string, boot time.Time) error {
	return s.putJSON(ctx, keyInProgress, markerRecord{FileID: fileID, Boot: boot.UTC()})
}

// ClearInProgress removes the marker. It is idempotent.
func (s *Store) ClearInProgress(ctx context.Context) error { return s.remove(ctx, keyInProgress) }

// WriteLastResult records the outcome of a run.
func (s *Store) WriteLastResult(ctx context.Context, r LastResult) error {
	return s.putJSON(ctx, keyLastResult, r)
}

// RecordFailure increments the consecutive-failure counter of the file.
func (s *Store) RecordFailure(ctx context.Context, fileID string) error {
	var rec attemptsRecord
	found, err := s.getJSON(ctx, keyAttempts, &rec)
	if err != nil {
		return err
	}
	count := 1
	if found && sameFile(rec.FileID, fileID) {
		count = rec.Count + 1
	}
	return s.putJSON(ctx, keyAttempts, attemptsRecord{Count: count, FileID: fileID})
}

// Load returns the state that belongs to the file identified by fileID.
//
// Rows written for another file (a restored or replaced database arrives with
// its own rows) are deleted and ignored. A leftover in-progress marker of this
// file counts as one failed attempt and is consumed. A device-only difference
// in a legacy "dev:ino" id keeps the state.
func (s *Store) Load(ctx context.Context, fileID string) (State, error) {
	flag, err := s.FlagRequested(ctx)
	if err != nil {
		return State{}, err
	}
	attempts, err := s.loadAttempts(ctx, fileID)
	if err != nil {
		return State{}, err
	}
	last, err := s.loadLastResult(ctx, fileID)
	if err != nil {
		return State{}, err
	}
	return State{FlagRequested: flag, Attempts: attempts, LastResult: last}, nil
}

// loadAttempts returns the failure counter of the file, folding in a leftover
// in-progress marker (a kill mid-conversion) as one more failed attempt.
func (s *Store) loadAttempts(ctx context.Context, fileID string) (int, error) {
	count, err := s.storedAttempts(ctx, fileID)
	if err != nil {
		return 0, err
	}
	return s.consumeMarker(ctx, fileID, count)
}

// storedAttempts returns the counter of the file; another file's is deleted.
func (s *Store) storedAttempts(ctx context.Context, fileID string) (int, error) {
	count, stale, err := s.readAttempts(ctx, fileID)
	if err != nil || !stale {
		return count, err
	}
	return 0, s.remove(ctx, keyAttempts)
}

// readAttempts returns the counter of the file without modifying anything;
// stale reports a counter that belongs to another file.
func (s *Store) readAttempts(ctx context.Context, fileID string) (count int, stale bool, err error) {
	var rec attemptsRecord
	found, err := s.getJSON(ctx, keyAttempts, &rec)
	if err != nil || !found {
		return 0, false, err
	}
	if !sameFile(rec.FileID, fileID) {
		return 0, true, nil
	}
	return rec.Count, false, nil
}

// consumeMarker removes a leftover in-progress marker. One of this file counts
// as a failed attempt on top of count; another file's marker is just deleted.
func (s *Store) consumeMarker(ctx context.Context, fileID string, count int) (int, error) {
	var marker markerRecord
	found, err := s.getJSON(ctx, keyInProgress, &marker)
	if err != nil || !found {
		return count, err
	}
	if sameFile(marker.FileID, fileID) {
		count++
		if putErr := s.putJSON(ctx, keyAttempts, attemptsRecord{Count: count, FileID: fileID}); putErr != nil {
			return 0, putErr
		}
	}
	return count, s.ClearInProgress(ctx)
}

// loadLastResult returns the last result of the file; another file's is deleted.
func (s *Store) loadLastResult(ctx context.Context, fileID string) (*LastResult, error) {
	last, stale, err := s.readLastResult(ctx, fileID)
	if err != nil || !stale {
		return last, err
	}
	return nil, s.remove(ctx, keyLastResult)
}

// readLastResult returns the last result of the file without modifying
// anything; stale reports a result that belongs to another file.
func (s *Store) readLastResult(ctx context.Context, fileID string) (last *LastResult, stale bool, err error) {
	var rec LastResult
	found, err := s.getJSON(ctx, keyLastResult, &rec)
	if err != nil || !found {
		return nil, false, err
	}
	if !sameFile(rec.FileID, fileID) {
		return nil, true, nil
	}
	return &rec, false, nil
}

// Peek is the read-only counterpart of Load for the status endpoint: it never
// consumes the in-progress marker and never deletes another file's rows, so a
// request cannot change what the boot path will see.
func (s *Store) Peek(ctx context.Context, fileID string) (State, error) {
	flag, err := s.FlagRequested(ctx)
	if err != nil {
		return State{}, err
	}
	attempts, _, err := s.readAttempts(ctx, fileID)
	if err != nil {
		return State{}, err
	}
	last, _, err := s.readLastResult(ctx, fileID)
	if err != nil {
		return State{}, err
	}
	return State{FlagRequested: flag, Attempts: attempts, LastResult: last}, nil
}

// ResetAttempts clears the failure counter. It is idempotent.
func (s *Store) ResetAttempts(ctx context.Context) error { return s.remove(ctx, keyAttempts) }

// SuppressesPending reports whether the persisted last result is a terminal
// skip the next boot would repeat, so promising a conversion would be wrong
// (integrity check failed, too many failed attempts).
func SuppressesPending(last *LastResult) bool {
	if last == nil || last.Outcome != ResultSkipped {
		return false
	}
	return last.Reason == ReasonIntegrityCheckFailed || last.Reason == ReasonTooManyFailures
}
