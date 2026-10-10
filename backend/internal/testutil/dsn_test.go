package testutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryDSN_UniqueWithinTest(t *testing.T) {
	a, b := MemoryDSN(t), MemoryDSN(t)
	require.NotEqual(t, a, b)
	require.True(t, strings.HasPrefix(a, "file:TestMemoryDSN_UniqueWithinTest_"))
	require.Contains(t, a, "?mode=memory&cache=shared")
}

func TestMemoryDSN_Subtests(t *testing.T) {
	seen := map[string]bool{MemoryDSN(t): true}
	for _, name := range []string{"a/b", "a b", "same", "same"} {
		t.Run(name, func(t *testing.T) {
			dsn := MemoryDSN(t)
			require.False(t, seen[dsn])
			seen[dsn] = true
			// Only the characters of the sanitized name may appear before the query.
			base := strings.SplitN(strings.TrimPrefix(dsn, "file:"), "?", 2)[0]
			require.NotContains(t, base, "/")
			require.NotContains(t, base, " ")
		})
	}
}

func TestMemoryDSN_AppendsParams(t *testing.T) {
	dsn := MemoryDSN(t, "_journal_mode=WAL", "_busy_timeout=5000")
	require.True(t, strings.HasSuffix(dsn, "&_journal_mode=WAL&_busy_timeout=5000"))
}

func TestOpenMemoryDB_IsolatedAndClosedOnCleanup(t *testing.T) {
	var closedProbe func() error
	t.Run("inner", func(t *testing.T) {
		db := OpenMemoryDB(t)
		require.NoError(t, db.Exec("CREATE TABLE x (id INTEGER)").Error)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		closedProbe = sqlDB.Ping
		require.NoError(t, closedProbe())

		other := OpenMemoryDB(t)
		require.Error(t, other.Exec("SELECT * FROM x").Error, "databases must not share state")
	})
	require.Error(t, closedProbe(), "pool must be closed after the test")
}
