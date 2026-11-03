package routes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// previousReleaseSchemaFixture is `.schema` of a database that the last tagged release (v0.46.5) created on its own
// (RegisterWithDeps, no CLI). Pass 2 of MigrateSchema must leave such a database untouched.
const previousReleaseSchemaFixture = "testdata/schema_previous_release.sql"

// statementRecorder is a GORM logger that keeps every SQL statement it traces.
type statementRecorder struct {
	mu    sync.Mutex
	stmts []string
}

func (r *statementRecorder) LogMode(gormlogger.LogLevel) gormlogger.Interface { return r }
func (r *statementRecorder) Info(context.Context, string, ...any)             {}
func (r *statementRecorder) Warn(context.Context, string, ...any)             {}
func (r *statementRecorder) Error(context.Context, string, ...any)            {}

func (r *statementRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stmts = append(r.stmts, sql)
}

// take returns the statements recorded since the last call and starts a new window.
func (r *statementRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.stmts
	r.stmts = nil
	return out
}

// openPreviousReleaseDatabase opens a file database that holds the previous release's schema.
func openPreviousReleaseDatabase(t *testing.T, rec *statementRecorder) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "charon.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: rec})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	fixture, err := os.ReadFile(previousReleaseSchemaFixture)
	require.NoError(t, err)
	// Each statement of the dump ends with ";" at the end of a line; a statement can span several lines.
	for _, stmt := range strings.Split(string(fixture), ";\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		require.NoErrorf(t, db.Exec(stmt).Error, "load fixture statement %q", stmt)
	}
	return db
}

// schemaSnapshot captures sqlite_master and the column list of every table, so any table rebuild, column change or
// index change between two snapshots fails the comparison.
func schemaSnapshot(t *testing.T, db *gorm.DB) string {
	t.Helper()
	type master struct {
		Type    string
		Name    string
		TblName string
		SQL     *string
	}
	var rows []master
	require.NoError(t, db.Raw("SELECT type, name, tbl_name, sql FROM sqlite_master ORDER BY type, name").Scan(&rows).Error)

	var b strings.Builder
	for _, r := range rows {
		sql := ""
		if r.SQL != nil {
			sql = *r.SQL
		}
		fmt.Fprintf(&b, "%s|%s|%s|%s\n", r.Type, r.Name, r.TblName, sql)
		if r.Type != "table" {
			continue
		}
		type column struct {
			CID     int
			Name    string
			Type    string
			NotNull int
			Default *string
			PK      int
		}
		var cols []column
		require.NoError(t, db.Raw(fmt.Sprintf("PRAGMA table_info(`%s`)", r.Name)).Scan(&cols).Error)
		for _, c := range cols {
			def := ""
			if c.Default != nil {
				def = *c.Default
			}
			fmt.Fprintf(&b, "  %d|%s|%s|%d|%s|%d\n", c.CID, c.Name, c.Type, c.NotNull, def, c.PK)
		}
	}
	return b.String()
}

// schemaChangingPrefixes are the statement starts that can alter a table or an index.
var schemaChangingPrefixes = []string{"CREATE TABLE", "ALTER TABLE", "DROP TABLE", "CREATE INDEX", "CREATE UNIQUE INDEX", "DROP INDEX"}

// webPushSingletonStatementPrefix is the one index statement that runs on every start. It is CREATE ... IF NOT
// EXISTS, so on an up-to-date schema it changes nothing; the snapshot comparison still proves that.
const webPushSingletonStatementPrefix = "CREATE UNIQUE INDEX IF NOT EXISTS idx_webpush_singleton"

// containsStatement reports whether any recorded statement contains the given text.
func containsStatement(stmts []string, text string) bool {
	for _, stmt := range stmts {
		if strings.Contains(stmt, text) {
			return true
		}
	}
	return false
}

// assertNoSchemaChangingStatements fails for any statement that could change a table or an index.
func assertNoSchemaChangingStatements(t *testing.T, stmts []string) {
	t.Helper()
	for _, stmt := range stmts {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		for _, prefix := range schemaChangingPrefixes {
			if strings.HasPrefix(upper, prefix) {
				assert.Truef(t, strings.HasPrefix(upper, strings.ToUpper(webPushSingletonStatementPrefix)),
					"pass 2 must not issue a schema-changing statement, got: %s", stmt)
			}
		}
	}
}

// TestMigrateSchema_SecondPassOnPreviousReleaseSchemaChangesNothing is the populated-database idempotence check:
// a database made by the previous release must not be rebuilt or altered by the second pass that RegisterWithDeps
// runs on every start.
func TestMigrateSchema_SecondPassOnPreviousReleaseSchemaChangesNothing(t *testing.T) {
	rec := &statementRecorder{}
	db := openPreviousReleaseDatabase(t, rec)

	require.NoError(t, MigrateSchema(db), "pass 1 on the previous release schema")
	// The previous release never created emergency_tokens, so pass 1 must have issued its DDL. This proves the
	// recorder is wired, so the empty pass 2 below is a real observation and not a silent logger.
	assert.Truef(t, containsStatement(rec.take(), "CREATE TABLE `emergency_tokens`"),
		"pass 1 must create emergency_tokens on a previous-release database")
	afterFirst := schemaSnapshot(t, db)
	rec.take()

	require.NoError(t, MigrateSchema(db), "pass 2 on the same database")
	afterSecond := schemaSnapshot(t, db)

	assert.Equal(t, afterFirst, afterSecond, "sqlite_master and PRAGMA table_info must be identical after pass 2")
	assertNoSchemaChangingStatements(t, rec.take())
}

// TestMigrateSchema_SecondPassOnEmptyDatabaseChangesNothing is the empty-database counterpart.
func TestMigrateSchema_SecondPassOnEmptyDatabaseChangesNothing(t *testing.T) {
	rec := &statementRecorder{}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{Logger: rec})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, MigrateSchema(db))
	afterFirst := schemaSnapshot(t, db)
	rec.take()

	require.NoError(t, MigrateSchema(db))

	assert.Equal(t, afterFirst, schemaSnapshot(t, db))
	assertNoSchemaChangingStatements(t, rec.take())
}
