package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCreateAutomaticBackup_EncryptionOn_ProducesEncryptedArchive checks that
// an automatic backup follows the stored "Enable backup encryption" setting
// and that the record keeps the requested kind.
func TestCreateAutomaticBackup_EncryptionOn_ProducesEncryptedArchive(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newOptionsTestService(t, enc)

	passphrase := "correct-horse-battery-staple"
	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{
		EncryptionEnabled:    boolPtr(true),
		EncryptionPassphrase: &passphrase,
	}))

	record, err := svc.CreateAutomaticBackup("pre_delete")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.True(t, record.Encrypted)
	assert.True(t, strings.HasSuffix(record.Filename, ".age"), "encrypted automatic backup must end in .age, got %q", record.Filename)
	assert.Equal(t, "pre_delete", record.Type)
}

// TestCreateAutomaticBackup_EncryptionOff_ProducesPlainArchive checks the
// default setting still yields an unencrypted archive, with the kind stored.
func TestCreateAutomaticBackup_EncryptionOff_ProducesPlainArchive(t *testing.T) {
	svc := newOptionsTestService(t, nil)

	record, err := svc.CreateAutomaticBackup("pre_delete")
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.False(t, record.Encrypted)
	assert.False(t, strings.HasSuffix(record.Filename, ".age"), "plain automatic backup must not be .age, got %q", record.Filename)
	assert.Equal(t, "pre_delete", record.Type)

	var stored []models.BackupRecord
	require.NoError(t, svc.db.Find(&stored).Error)
	require.Len(t, stored, 1)
	assert.Equal(t, "pre_delete", stored[0].Type)
}

// TestCreateAutomaticBackup_EncryptionOnNoPassphrase_ReturnsError checks that
// a missing stored passphrase fails the backup instead of falling back to a
// plain archive.
func TestCreateAutomaticBackup_EncryptionOnNoPassphrase_ReturnsError(t *testing.T) {
	enc, err := crypto.NewEncryptionService(testEncryptionKeyBase64(t))
	require.NoError(t, err)
	svc := newOptionsTestService(t, enc)

	require.NoError(t, svc.UpdateSettings(BackupSettingsUpdate{
		EncryptionEnabled: boolPtr(true),
	}))

	record, err := svc.CreateAutomaticBackup("pre_delete")
	require.Error(t, err)
	assert.Nil(t, record)

	var stored []models.BackupRecord
	require.NoError(t, svc.db.Find(&stored).Error)
	assert.Empty(t, stored, "no record may be stored when the automatic backup fails")
}
