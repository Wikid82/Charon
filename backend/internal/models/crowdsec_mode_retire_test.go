package models_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// retireAbsent marks a settings key that has no row in a retire case.
const retireAbsent = "<absent>"

// readSettingForRetire returns the stored value of key and whether a row exists.
func readSettingForRetire(t *testing.T, db *gorm.DB, key string) (string, bool) {
	t.Helper()
	var setting models.Setting
	res := db.Where("key = ?", key).Limit(1).Find(&setting)
	require.NoError(t, res.Error)
	return setting.Value, res.RowsAffected > 0
}

// seedRetireSettings writes the enabled and mode rows unless a value is retireAbsent.
func seedRetireSettings(t *testing.T, db *gorm.DB, enabled, mode string) {
	t.Helper()
	for key, value := range map[string]string{
		models.CrowdSecEnabledSettingKey:     enabled,
		models.CrowdSecRetiredModeSettingKey: mode,
	} {
		if value == retireAbsent {
			continue
		}
		seedCrowdSecSetting(t, db, key, value)
	}
}

// assertRetireState checks the stored enabled and mode rows after RetireCrowdSecModeSetting.
func assertRetireState(t *testing.T, db *gorm.DB, wantEnabled string, wantModeGone bool) {
	t.Helper()
	enabledValue, enabledFound := readSettingForRetire(t, db, models.CrowdSecEnabledSettingKey)
	if wantEnabled == retireAbsent {
		assert.False(t, enabledFound, "enabled key must stay absent")
	} else {
		assert.True(t, enabledFound, "enabled key must exist")
		assert.Equal(t, wantEnabled, enabledValue)
	}
	_, modeFound := readSettingForRetire(t, db, models.CrowdSecRetiredModeSettingKey)
	assert.Equal(t, !wantModeGone, modeFound, "mode key presence")
}

// oldCrowdSecDecision is the decision the reader produced before the mode key was retired: the enabled key wins
// when it has a value, the mode key decides otherwise when it has one, and the default row decides last.
// It is the oracle for the carry-over, so the decision cannot change across the upgrade.
func oldCrowdSecDecision(enabled, mode string, rowLocal bool) bool {
	if e := strings.TrimSpace(enabled); e != "" {
		return strings.EqualFold(e, "true")
	}
	if m := strings.TrimSpace(mode); m != "" {
		return m == "local"
	}
	return rowLocal
}

func TestRetireCrowdSecModeSetting_NamedCases(t *testing.T) {
	tests := []struct {
		name        string
		enabled     string
		mode        string
		wantRetired bool
		wantEnabled string
	}{
		{name: "mode absent, enabled absent", enabled: retireAbsent, mode: retireAbsent, wantRetired: false, wantEnabled: retireAbsent},
		{name: "mode absent, enabled blank", enabled: "", mode: retireAbsent, wantRetired: false, wantEnabled: ""},
		{name: "mode absent, enabled true", enabled: "true", mode: retireAbsent, wantRetired: false, wantEnabled: "true"},
		{name: "mode local, enabled absent carries true", enabled: retireAbsent, mode: "local", wantRetired: true, wantEnabled: "true"},
		{name: "mode local, enabled blank carries true", enabled: "", mode: "local", wantRetired: true, wantEnabled: "true"},
		{name: "mode local, enabled whitespace carries true", enabled: "  ", mode: "local", wantRetired: true, wantEnabled: "true"},
		{name: "mode padded local carries true", enabled: retireAbsent, mode: " local ", wantRetired: true, wantEnabled: "true"},
		{name: "mode local, enabled false stays false", enabled: "false", mode: "local", wantRetired: true, wantEnabled: "false"},
		{name: "mode disabled carries false", enabled: retireAbsent, mode: "disabled", wantRetired: true, wantEnabled: "false"},
		{name: "mode external carries false", enabled: retireAbsent, mode: "external", wantRetired: true, wantEnabled: "false"},
		{name: "mode remote carries false", enabled: "", mode: "remote", wantRetired: true, wantEnabled: "false"},
		{name: "mode disabled, enabled true stays true", enabled: "true", mode: "disabled", wantRetired: true, wantEnabled: "true"},
		{name: "mode whitespace is deleted, enabled absent stays absent", enabled: retireAbsent, mode: "   ", wantRetired: true, wantEnabled: retireAbsent},
		{name: "mode blank is deleted, enabled blank stays blank", enabled: "", mode: "", wantRetired: true, wantEnabled: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openCrowdSecSettingTestDB(t)
			seedRetireSettings(t, db, tt.enabled, tt.mode)

			retired, err := models.RetireCrowdSecModeSetting(db)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRetired, retired)
			assertRetireState(t, db, tt.wantEnabled, true)
		})
	}
}

// TestRetireCrowdSecModeSetting_CombinationMatrix runs every (enabled, mode, default row) combination. The decision
// of the oracle and of the reader after the call must agree.
func TestRetireCrowdSecModeSetting_CombinationMatrix(t *testing.T) {
	enabledValues := []string{retireAbsent, "", "  ", "true", "false"}
	modeValues := []string{retireAbsent, "", " ", "local", " local ", "disabled", "remote"}
	for _, enabled := range enabledValues {
		for _, mode := range modeValues {
			for _, row := range []string{"local", "disabled"} {
				name := fmt.Sprintf("enabled=%q/mode=%q/row=%s", enabled, mode, row)
				t.Run(name, func(t *testing.T) {
					db := openCrowdSecSettingTestDB(t)
					seedRetireSettings(t, db, enabled, mode)
					cfg := models.SecurityConfig{CrowdSecMode: row}
					rowLocal := row == "local"

					// The oracle is the decision the removed reader gave before the call; the reader left after the call
					// must reproduce it.
					want := oldCrowdSecDecision(enabledOrEmpty(enabled), enabledOrEmpty(mode), rowLocal)

					_, err := models.RetireCrowdSecModeSetting(db)
					require.NoError(t, err)

					after, afterErr := models.CrowdSecEnabled(db, cfg)
					require.NoError(t, afterErr)
					assert.Equal(t, want, after, "carry-over must keep the decision")

					// Expected stored state: the enabled key only changes when it had no value and the mode key had one.
					// The mode row is gone afterwards whether or not it had a value.
					wantEnabled := enabled
					modeHasValue := mode != retireAbsent && strings.TrimSpace(mode) != ""
					enabledHasValue := enabled != retireAbsent && strings.TrimSpace(enabled) != ""
					if modeHasValue && !enabledHasValue {
						wantEnabled = fmt.Sprintf("%t", strings.TrimSpace(mode) == "local")
					}
					assertRetireState(t, db, wantEnabled, true)
				})
			}
		}
	}
}

// enabledOrEmpty maps the absent marker to the empty value, which the oracle treats the same way.
func enabledOrEmpty(v string) string {
	if v == retireAbsent {
		return ""
	}
	return v
}

func TestRetireCrowdSecModeSetting_IsIdempotent(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedRetireSettings(t, db, retireAbsent, "local")

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.NoError(t, err)
	require.True(t, retired)
	assertRetireState(t, db, "true", true)

	retired, err = models.RetireCrowdSecModeSetting(db)
	require.NoError(t, err)
	assert.False(t, retired, "a second run has nothing to retire")
	assertRetireState(t, db, "true", true)
}

func TestRetireCrowdSecModeSetting_NeverOverwritesNonBlankEnabled(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedRetireSettings(t, db, "false", "local")

	_, err := models.RetireCrowdSecModeSetting(db)
	require.NoError(t, err)
	assertRetireState(t, db, "false", true)
}

func TestRetireCrowdSecModeSetting_NilDBIsNoop(t *testing.T) {
	retired, err := models.RetireCrowdSecModeSetting(nil)
	require.NoError(t, err)
	assert.False(t, retired)
}

func TestRetireCrowdSecModeSetting_MissingSettingsTableIsNoop(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.NoError(t, err)
	assert.False(t, retired)
	assert.False(t, db.Migrator().HasTable(&models.Setting{}), "the retire step must not create the settings table")
}

// TestRetireCrowdSecModeSetting_DeleteFaultRollsBack proves the carry-over is all-or-nothing: the enabled row written
// before the failing mode delete must not survive the rollback.
func TestRetireCrowdSecModeSetting_DeleteFaultRollsBack(t *testing.T) {
	db := openCrowdSecSettingTestDB(t)
	seedRetireSettings(t, db, retireAbsent, "local")
	gormfault.Inject(t, db, gormfault.Delete, "settings", gormfault.VarIs(models.CrowdSecRetiredModeSettingKey))

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.Error(t, err)
	assert.False(t, retired)
	assert.True(t, gormfault.IsInjected(err))
	assertRetireState(t, db, retireAbsent, false)
}

// TestRetireCrowdSecModeSetting_ModeReadFaultIsReturned covers a failed read of the mode key on a settings table
// that exists: the error is returned and nothing is written.
func TestRetireCrowdSecModeSetting_ModeReadFaultIsReturned(t *testing.T) {
	db, path := openRetireFileDB(t)
	seedRetireSettings(t, db, retireAbsent, "local")
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs(models.CrowdSecRetiredModeSettingKey))

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.Error(t, err)
	assert.False(t, retired)
	assert.True(t, gormfault.IsInjected(err))

	// A second handle on the same file reads the rows without the injected fault.
	reader, openErr := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, openErr)
	assertRetireState(t, reader, retireAbsent, false)
}

// openRetireFileDB opens a file database with the settings table and returns its path, so a test can read the
// rows through a second handle after a fault has been injected on the first one.
func openRetireFileDB(t *testing.T) (db *gorm.DB, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "charon.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db, path
}

// TestRetireCrowdSecModeSetting_EnabledReadFaultIsReturned covers a failed read of the enabled setting: the error is
// returned and the mode row stays for the next start.
func TestRetireCrowdSecModeSetting_EnabledReadFaultIsReturned(t *testing.T) {
	db, path := openRetireFileDB(t)
	seedRetireSettings(t, db, retireAbsent, "local")
	gormfault.Inject(t, db, gormfault.Query, "settings", gormfault.VarIs(models.CrowdSecEnabledSettingKey))

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.Error(t, err)
	assert.False(t, retired)
	assert.True(t, gormfault.IsInjected(err))

	reader, openErr := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, openErr)
	assertRetireState(t, reader, retireAbsent, false)
}

// TestRetireCrowdSecModeSetting_EnabledWriteFaultRollsBack covers a failed carry-over write: a trigger rejects the
// enabled row, the transaction rolls back, and both rows keep their values.
func TestRetireCrowdSecModeSetting_EnabledWriteFaultRollsBack(t *testing.T) {
	db, path := openRetireFileDB(t)
	seedRetireSettings(t, db, retireAbsent, "local")
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_enabled_write BEFORE INSERT ON settings
		WHEN NEW.key = 'security.crowdsec.enabled'
		BEGIN SELECT RAISE(ABORT, 'forced enabled write failure'); END;`).Error)

	retired, err := models.RetireCrowdSecModeSetting(db)
	require.Error(t, err)
	assert.False(t, retired)

	reader, openErr := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, openErr)
	assertRetireState(t, reader, retireAbsent, false)
}
