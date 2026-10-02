package dbmaint

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// newSettingsDB builds a scratch database whose settings table is created by
// the real model, so the store is tested against the production schema.
func newSettingsDB(t *testing.T) (db *sql.DB, path string) {
	t.Helper()
	return newSettingsDBWith(t, scratchOpts{})
}

// newSettingsDBWith is newSettingsDB over a scratch database shaped by opts.
func newSettingsDBWith(t *testing.T, opts scratchOpts) (db *sql.DB, path string) {
	t.Helper()
	db, path = newScratchDB(t, opts)
	gdb, err := gorm.Open(sqlite.Dialector{Conn: db}, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Setting{}))
	return db, path
}

func settingFound(t *testing.T, db *sql.DB, key string) bool {
	t.Helper()
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE "key" = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func TestFileID_IsTheInodeAndSurvivesRewrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.db")
	require.NoError(t, os.WriteFile(path, []byte("a"), 0o600))
	id1, err := FileID(path)
	require.NoError(t, err)
	assert.Regexp(t, `^\d+$`, id1)

	require.NoError(t, os.WriteFile(path, []byte("bbbb"), 0o600))
	id2, err := FileID(path)
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "rewriting in place keeps the inode")

	replacement := path + ".new"
	require.NoError(t, os.WriteFile(replacement, []byte("c"), 0o600))
	require.NoError(t, os.Rename(replacement, path))
	id3, err := FileID(path)
	require.NoError(t, err)
	assert.NotEqual(t, id1, id3, "a file renamed into place gets a new inode")

	_, err = FileID(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}

func TestStore_FlagRoundTripAndSettingsShape(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()

	on, err := s.FlagRequested(ctx)
	require.NoError(t, err)
	assert.False(t, on)

	require.NoError(t, s.SetFlag(ctx))
	require.NoError(t, s.SetFlag(ctx), "setting twice is idempotent")
	on, err = s.FlagRequested(ctx)
	require.NoError(t, err)
	assert.True(t, on)

	var row models.Setting
	require.NoError(t, db.QueryRow(`SELECT "key", value, type, category FROM settings WHERE "key" = ?`, SettingKeyFlag).
		Scan(&row.Key, &row.Value, &row.Type, &row.Category))
	assert.Equal(t, "true", row.Value)
	assert.Equal(t, "bool", row.Type)
	assert.Equal(t, "maintenance", row.Category)

	require.NoError(t, s.ClearFlag(ctx))
	require.NoError(t, s.ClearFlag(ctx), "clearing twice is idempotent")
	on, err = s.FlagRequested(ctx)
	require.NoError(t, err)
	assert.False(t, on)
}

func TestStore_FlagIsFalseForAnyValueButTrue(t *testing.T) {
	db, _ := newSettingsDB(t)
	_, err := db.Exec(`INSERT INTO settings ("key", value, type, category) VALUES (?, 'nope', 'bool', 'maintenance')`, SettingKeyFlag)
	require.NoError(t, err)
	on, err := NewStore(db).FlagRequested(context.Background())
	require.NoError(t, err)
	assert.False(t, on)
}

func TestStore_RecordFailureCountsPerFile(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()

	for want := 1; want <= 3; want++ {
		require.NoError(t, s.RecordFailure(ctx, "100"))
		st, err := s.Load(ctx, "100")
		require.NoError(t, err)
		assert.Equal(t, want, st.Attempts)
	}
}

func TestStore_LoadIgnoresAndDeletesStateOfAReplacedFile(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()

	require.NoError(t, s.RecordFailure(ctx, "100"))
	require.NoError(t, s.RecordFailure(ctx, "100"))
	require.NoError(t, s.SetInProgress(ctx, "100", time.Now()))
	require.NoError(t, s.WriteLastResult(ctx, LastResult{At: time.Now(), Outcome: ResultFailed, FileID: "100"}))

	st, err := s.Load(ctx, "999") // the file was replaced: new inode
	require.NoError(t, err)
	assert.Equal(t, 0, st.Attempts)
	assert.Nil(t, st.LastResult)
	for _, key := range []string{keyAttempts, keyInProgress, keyLastResult} {
		found := settingFound(t, db, key)
		assert.False(t, found, "%s of the old file is deleted", key)
	}
}

func TestStore_LoadKeepsStateWhenOnlyTheDeviceDiffers(t *testing.T) {
	db, _ := newSettingsDB(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO settings ("key", value, type, category) VALUES
		(?, '{"count":2,"file_id":"77:100"}', 'json', 'maintenance'),
		(?, '{"at":"2026-10-01T00:00:00Z","outcome":"failed","file_id":"77:100"}', 'json', 'maintenance')`,
		keyAttempts, keyLastResult)
	require.NoError(t, err)

	st, err := NewStore(db).Load(ctx, "100")
	require.NoError(t, err)
	assert.Equal(t, 2, st.Attempts, "a legacy dev:ino id with the same inode keeps the state")
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ResultFailed, st.LastResult.Outcome)
}

func TestStore_LeftoverMarkerForTheSameFileCountsAsAFailedAttempt(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()

	require.NoError(t, s.RecordFailure(ctx, "100"))
	require.NoError(t, s.SetInProgress(ctx, "100", time.Now()))

	st, err := s.Load(ctx, "100")
	require.NoError(t, err)
	assert.Equal(t, 2, st.Attempts)
	found := settingFound(t, db, keyInProgress)
	assert.False(t, found, "the marker is consumed")

	st, err = s.Load(ctx, "100")
	require.NoError(t, err)
	assert.Equal(t, 2, st.Attempts, "a second load does not count it again")
}

func TestStore_ClearInProgressIsIdempotent(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()
	require.NoError(t, s.SetInProgress(ctx, "1", time.Now()))
	require.NoError(t, s.ClearInProgress(ctx))
	require.NoError(t, s.ClearInProgress(ctx))
}

func TestStore_LastResultRoundTrip(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()
	at := time.Date(2026, 10, 2, 4, 11, 0, 0, time.UTC)
	want := LastResult{At: at, Outcome: ResultConverted, Reason: "", BytesBefore: 4800, BytesAfter: 2900, FileID: "100"}
	require.NoError(t, s.WriteLastResult(ctx, want))
	require.NoError(t, s.WriteLastResult(ctx, want), "a rewrite replaces the row")

	st, err := s.Load(ctx, "100")
	require.NoError(t, err)
	require.NotNil(t, st.LastResult)
	assert.Equal(t, want.Outcome, st.LastResult.Outcome)
	assert.True(t, at.Equal(st.LastResult.At))
	assert.Equal(t, int64(4800), st.LastResult.BytesBefore)
	assert.Equal(t, int64(2900), st.LastResult.BytesAfter)
}

func TestStore_LoadWithCorruptRowsDropsThem(t *testing.T) {
	db, _ := newSettingsDB(t)
	_, err := db.Exec(`INSERT INTO settings ("key", value, type, category) VALUES
		(?, 'not json', 'json', 'maintenance'), (?, '{', 'json', 'maintenance'), (?, '[]x', 'json', 'maintenance')`,
		keyAttempts, keyInProgress, keyLastResult)
	require.NoError(t, err)

	st, err := NewStore(db).Load(context.Background(), "100")
	require.NoError(t, err)
	assert.Zero(t, st.Attempts)
	assert.Nil(t, st.LastResult)
	for _, key := range []string{keyAttempts, keyInProgress, keyLastResult} {
		found := settingFound(t, db, key)
		assert.False(t, found, key)
	}
}

func TestStore_ErrorsSurfaceFromAClosedDatabase(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	require.NoError(t, db.Close())
	ctx := context.Background()

	assert.Error(t, s.SetFlag(ctx))
	assert.Error(t, s.ClearFlag(ctx))
	_, err := s.FlagRequested(ctx)
	assert.Error(t, err)
	assert.Error(t, s.RecordFailure(ctx, "1"))
	assert.Error(t, s.SetInProgress(ctx, "1", time.Now()))
	assert.Error(t, s.ClearInProgress(ctx))
	assert.Error(t, s.WriteLastResult(ctx, LastResult{}))
	_, err = s.Load(ctx, "1")
	assert.Error(t, err)
}

func TestStore_SettingsHandlerCannotSeeTheRows(t *testing.T) {
	// The rows use the reserved "maintenance." prefix; this guards the naming
	// contract the settings handler filters on.
	for _, key := range []string{SettingKeyFlag, keyAttempts, keyInProgress, keyLastResult} {
		assert.Regexp(t, `^maintenance\.`, key)
	}
}

func TestStore_PeekReadsWithoutConsumingOrDeleting(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()
	require.NoError(t, s.SetFlag(ctx))
	require.NoError(t, s.RecordFailure(ctx, "100"))
	require.NoError(t, s.SetInProgress(ctx, "100", time.Now()))
	require.NoError(t, s.WriteLastResult(ctx, LastResult{At: time.Now(), Outcome: ResultSkipped, Reason: ReasonDatabaseBusy, FileID: "100"}))

	st, err := s.Peek(ctx, "100")
	require.NoError(t, err)
	assert.True(t, st.FlagRequested)
	assert.Equal(t, 1, st.Attempts, "a leftover marker is not folded in by a read")
	require.NotNil(t, st.LastResult)
	assert.Equal(t, ReasonDatabaseBusy, st.LastResult.Reason)
	assert.True(t, settingFound(t, db, keyInProgress), "the marker is left for the boot path")

	other, err := s.Peek(ctx, "999")
	require.NoError(t, err)
	assert.Zero(t, other.Attempts, "another file's counter does not apply")
	assert.Nil(t, other.LastResult)
	assert.True(t, settingFound(t, db, keyAttempts), "a read never deletes rows of another file")
	assert.True(t, settingFound(t, db, keyLastResult))
}

func TestStore_PeekErrorsSurfaceFromAClosedDatabase(t *testing.T) {
	db, _ := newSettingsDB(t)
	require.NoError(t, db.Close())
	_, err := NewStore(db).Peek(context.Background(), "100")
	assert.Error(t, err)
}

func TestStore_ResetAttemptsClearsTheCounterAndIsIdempotent(t *testing.T) {
	db, _ := newSettingsDB(t)
	s := NewStore(db)
	ctx := context.Background()
	require.NoError(t, s.RecordFailure(ctx, "100"))
	require.NoError(t, s.RecordFailure(ctx, "100"))

	require.NoError(t, s.ResetAttempts(ctx))
	require.NoError(t, s.ResetAttempts(ctx))

	st, err := s.Peek(ctx, "100")
	require.NoError(t, err)
	assert.Zero(t, st.Attempts)
}

func TestSuppressesPending(t *testing.T) {
	cases := []struct {
		name string
		last *LastResult
		want bool
	}{
		{"no result", nil, false},
		{"integrity check failed", &LastResult{Outcome: ResultSkipped, Reason: ReasonIntegrityCheckFailed}, true},
		{"too many failures", &LastResult{Outcome: ResultSkipped, Reason: ReasonTooManyFailures}, true},
		{"busy is transient", &LastResult{Outcome: ResultSkipped, Reason: ReasonDatabaseBusy}, false},
		{"insufficient disk is shown as its own notice", &LastResult{Outcome: ResultSkipped, Reason: ReasonInsufficientDisk}, false},
		{"converted", &LastResult{Outcome: ResultConverted}, false},
		{"same reason but not a skip", &LastResult{Outcome: ResultFailed, Reason: ReasonIntegrityCheckFailed}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.want, SuppressesPending(tc.last)) })
	}
}

// failNthRead makes the nth read of a Store fail by sending an invalid
// statement in its place.
type failNthRead struct {
	SQLExecer
	n, calls int
}

func (f *failNthRead) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	f.calls++
	if f.calls == f.n {
		query = "SELECT value FROM no_such_table WHERE 1 = ?"
		args = []any{1}
	}
	return f.SQLExecer.QueryRowContext(ctx, query, args...)
}

func TestStore_PeekSurfacesEachReadFailure(t *testing.T) {
	db, _ := newSettingsDB(t)
	for nth, what := range map[int]string{1: "flag", 2: "attempts", 3: "last result"} {
		t.Run(what, func(t *testing.T) {
			_, err := NewStore(&failNthRead{SQLExecer: db, n: nth}).Peek(context.Background(), "100")
			assert.Error(t, err)
		})
	}
}

func TestStore_DiscardMarkerIfConverted(t *testing.T) {
	ctx := context.Background()

	t.Run("a marker of the same file is dropped and the counter reset", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		s := NewStore(db)
		require.NoError(t, s.RecordFailure(ctx, "100"))
		require.NoError(t, s.RecordFailure(ctx, "100"))
		require.NoError(t, s.SetInProgress(ctx, "100", time.Now()))

		discarded, err := s.DiscardMarkerIfConverted(ctx, "100")
		require.NoError(t, err)
		assert.True(t, discarded)
		assert.False(t, settingFound(t, db, keyInProgress))
		assert.False(t, settingFound(t, db, keyAttempts))
	})

	t.Run("a legacy dev:ino marker of the same inode counts as the same file", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		s := NewStore(db)
		require.NoError(t, s.SetInProgress(ctx, "77:100", time.Now()))

		discarded, err := s.DiscardMarkerIfConverted(ctx, "100")
		require.NoError(t, err)
		assert.True(t, discarded)
	})

	t.Run("another file's marker and counter are left alone", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		s := NewStore(db)
		require.NoError(t, s.RecordFailure(ctx, "200"))
		require.NoError(t, s.SetInProgress(ctx, "200", time.Now()))

		discarded, err := s.DiscardMarkerIfConverted(ctx, "100")
		require.NoError(t, err)
		assert.False(t, discarded)
		assert.True(t, settingFound(t, db, keyInProgress))
		assert.True(t, settingFound(t, db, keyAttempts))
	})

	t.Run("no marker means nothing to do, the counter is kept", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		s := NewStore(db)
		require.NoError(t, s.RecordFailure(ctx, "100"))

		discarded, err := s.DiscardMarkerIfConverted(ctx, "100")
		require.NoError(t, err)
		assert.False(t, discarded)
		assert.True(t, settingFound(t, db, keyAttempts))
	})

	t.Run("a marker that cannot be deleted is an error and keeps the counter", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		s := NewStore(db)
		require.NoError(t, s.RecordFailure(ctx, "100"))
		require.NoError(t, s.SetInProgress(ctx, "100", time.Now()))
		_, err := db.Exec(`CREATE TRIGGER keep_marker BEFORE DELETE ON settings
			WHEN OLD."key" = 'maintenance.in_progress' BEGIN SELECT RAISE(ABORT, 'readonly'); END`)
		require.NoError(t, err)

		discarded, err := s.DiscardMarkerIfConverted(ctx, "100")
		assert.Error(t, err)
		assert.False(t, discarded)
		assert.True(t, settingFound(t, db, keyAttempts))
	})

	t.Run("a database error is returned", func(t *testing.T) {
		db, _ := newSettingsDB(t)
		require.NoError(t, db.Close())

		_, err := NewStore(db).DiscardMarkerIfConverted(ctx, "100")
		assert.Error(t, err)
	})
}
