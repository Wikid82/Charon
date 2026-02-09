package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupService_GetAvailableSpace(t *testing.T) {
	t.Parallel()

	t.Run("returns space for existing directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		svc := &BackupService{BackupDir: tmpDir}

		space, err := svc.GetAvailableSpace()
		require.NoError(t, err)
		assert.Greater(t, space, int64(0))
	})

	t.Run("errors for missing directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		missingDir := filepath.Join(tmpDir, "does-not-exist")
		svc := &BackupService{BackupDir: missingDir}

		_, err := svc.GetAvailableSpace()
		require.Error(t, err)
	})
}
