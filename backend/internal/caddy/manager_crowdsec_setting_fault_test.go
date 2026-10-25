package caddy

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// TestManager_ApplyConfig_UnreadableCrowdSecSettingFallsBackToRow: when the enabled setting cannot be read,
// the apply logs a Warn and decides from the default row, so a local row still attaches CrowdSec.
func TestManager_ApplyConfig_UnreadableCrowdSecSettingFallsBackToRow(t *testing.T) {
	withTestBouncerKey(t)
	db := newCrowdSecLookupDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-default", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)
	require.NoError(t, db.Create(&models.Setting{Key: "security.crowdsec.enabled", Value: "false", Category: "security", Type: "bool"}).Error)
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs("security.crowdsec.enabled"))

	logs := &bytes.Buffer{}
	logger.Init(false, logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })

	cfg := applyAndCaptureLoad(t, db, config.SecurityConfig{CerberusEnabled: true, CrowdSecMode: "disabled"})

	require.NotNil(t, cfg.Apps.CrowdSec, "the unreadable setting must fall back to the local default row")
	assert.Contains(t, logs.String(), "Failed to read CrowdSec setting, using the default SecurityConfig row")
}
