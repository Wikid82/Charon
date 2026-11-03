package services

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// missingTableProcess is a CrowdsecProcessManager that is never expected to start anything in this test.
type missingTableProcess struct{}

func (missingTableProcess) Start(context.Context, string, string) (int, error) { return 0, nil }
func (missingTableProcess) Stop(context.Context, string) error                 { return nil }
func (missingTableProcess) Status(context.Context, string) (running bool, pid int, err error) {
	return false, 0, nil
}

// TestReconcileCrowdSecOnStartup_MissingSecurityConfigTableWarnsWithoutMigrationHint pins the defensive branch: when
// the table is absent the warning names the real cause (the schema did not exist when the check ran) and no longer
// tells the operator to run a command that the boot already runs.
func TestReconcileCrowdSecOnStartup_MissingSecurityConfigTableWarnsWithoutMigrationHint(t *testing.T) {
	logs := &bytes.Buffer{}
	logger.Init(false, logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ReconcileCrowdSecOnStartup(db, missingTableProcess{}, "", t.TempDir(), nil)

	assert.Contains(t, logs.String(), "CrowdSec reconciliation skipped: the SecurityConfig table is missing")
	assert.NotContains(t, logs.String(), "charon migrate")
	assert.NotContains(t, logs.String(), "table not found")
}
