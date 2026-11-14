package routes

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestWarnUnrecognisedCerberusSwitches_WarnsOncePerBadKey describes the startup check: each switch key stored with a
// value other than true or false produces one warning naming the key, and a canonical value produces none.
func TestWarnUnrecognisedCerberusSwitches_WarnsOncePerBadKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:cerberus_warn_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	require.NoError(t, db.Create(&models.Setting{Key: models.CerberusFeatureSettingKey, Value: " true", Category: "feature", Type: "bool"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: models.CerberusLegacySettingKey, Value: "TRUE", Category: "security", Type: "bool"}).Error)

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	warnUnrecognisedCerberusSwitches(db)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 1, "only the padded feature key is reported: %s", buf.String())
	assert.Contains(t, lines[0], `"level":"warning"`)
	assert.Contains(t, lines[0], models.CerberusFeatureSettingKey)
	assert.NotContains(t, lines[0], models.CerberusLegacySettingKey)
}

// TestWarnUnrecognisedCerberusSwitches_ReadFailureIsLogged describes a failed read during the check: startup continues
// and one warning records the failure.
func TestWarnUnrecognisedCerberusSwitches_ReadFailureIsLogged(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:cerberus_warn_fail_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	warnUnrecognisedCerberusSwitches(db)

	assert.Contains(t, buf.String(), "Failed to check the Cerberus switch values")
}
