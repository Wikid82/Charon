package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCerberusSwitchFromSetting_OnlyTrueIsOn describes the one reading every reader uses: only "true" in any case is On,
// and a padded or unknown stored value is Off, not trimmed.
func TestCerberusSwitchFromSetting_OnlyTrueIsOn(t *testing.T) {
	tests := []struct {
		value string
		want  models.CerberusSwitch
	}{
		{"true", models.CerberusSwitchOn},
		{"TRUE", models.CerberusSwitchOn},
		{"false", models.CerberusSwitchOff},
		{"", models.CerberusSwitchOff},
		{" true", models.CerberusSwitchOff},
		{"true ", models.CerberusSwitchOff},
		{"1", models.CerberusSwitchOff},
		{"yes", models.CerberusSwitchOff},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.want, models.CerberusSwitchFromSetting(tt.value, true))
		})
	}
	assert.Equal(t, models.CerberusSwitchAbsent, models.CerberusSwitchFromSetting("true", false), "an unstored key is absent")
}

// TestResolveCerberusSwitchKeys_FeatureKeyDecidesWhenStored describes the shared resolver: a stored feature key decides
// whatever its value, and the legacy key is read only when the feature key is absent.
func TestResolveCerberusSwitchKeys_FeatureKeyDecidesWhenStored(t *testing.T) {
	tests := []struct {
		name    string
		feature *string
		legacy  *string
		want    models.CerberusSwitch
	}{
		{name: "neither key stored", want: models.CerberusSwitchAbsent},
		{name: "feature true", feature: ptrString("true"), want: models.CerberusSwitchOn},
		{name: "feature false wins over legacy true", feature: ptrString("false"), legacy: ptrString("true"), want: models.CerberusSwitchOff},
		{name: "blank feature is off, legacy not read", feature: ptrString(""), legacy: ptrString("true"), want: models.CerberusSwitchOff},
		{name: "padded feature is off", feature: ptrString(" true"), legacy: ptrString("true"), want: models.CerberusSwitchOff},
		{name: "absent feature falls to legacy true", legacy: ptrString("TRUE"), want: models.CerberusSwitchOn},
		{name: "absent feature falls to legacy false", legacy: ptrString("false"), want: models.CerberusSwitchOff},
		{name: "absent feature falls to blank legacy", legacy: ptrString(""), want: models.CerberusSwitchOff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newModuleTestDB(t)
			if tt.feature != nil {
				require.NoError(t, db.Create(&models.Setting{Key: models.CerberusFeatureSettingKey, Value: *tt.feature, Category: "feature", Type: "bool"}).Error)
			}
			if tt.legacy != nil {
				require.NoError(t, db.Create(&models.Setting{Key: models.CerberusLegacySettingKey, Value: *tt.legacy, Category: "security", Type: "bool"}).Error)
			}

			got, err := models.ResolveCerberusSwitchKeys(db)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestResolveCerberusSwitchKeys_ReadFailureIsReturned describes a failed read: the error is returned and the switch
// reads as absent, so the caller falls through to its next source.
func TestResolveCerberusSwitchKeys_ReadFailureIsReturned(t *testing.T) {
	db := newModuleTestDB(t)
	failQueriesOn(db, "settings")

	got, err := models.ResolveCerberusSwitchKeys(db)

	require.Error(t, err)
	assert.Equal(t, models.CerberusSwitchAbsent, got)
}

// TestUnrecognisedCerberusSwitchKeys_NamesOnlyValuesOtherThanTrueOrFalse describes the startup check: a stored value that
// is not exactly true or false (any case) is reported per key, and nothing is rewritten.
func TestUnrecognisedCerberusSwitchKeys_NamesOnlyValuesOtherThanTrueOrFalse(t *testing.T) {
	tests := []struct {
		name    string
		feature *string
		legacy  *string
		want    []string
	}{
		{name: "no keys", want: nil},
		{name: "canonical values", feature: ptrString("TRUE"), legacy: ptrString("false"), want: nil},
		{name: "blank feature", feature: ptrString(""), want: []string{models.CerberusFeatureSettingKey}},
		{name: "padded legacy", legacy: ptrString(" true"), want: []string{models.CerberusLegacySettingKey}},
		{name: "one and yes", feature: ptrString("1"), legacy: ptrString("yes"), want: []string{models.CerberusFeatureSettingKey, models.CerberusLegacySettingKey}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newModuleTestDB(t)
			if tt.feature != nil {
				require.NoError(t, db.Create(&models.Setting{Key: models.CerberusFeatureSettingKey, Value: *tt.feature, Category: "feature", Type: "bool"}).Error)
			}
			if tt.legacy != nil {
				require.NoError(t, db.Create(&models.Setting{Key: models.CerberusLegacySettingKey, Value: *tt.legacy, Category: "security", Type: "bool"}).Error)
			}

			got, err := models.UnrecognisedCerberusSwitchKeys(db)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestUnrecognisedCerberusSwitchKeys_ReadFailureIsReturned describes a failed read during the startup check.
func TestUnrecognisedCerberusSwitchKeys_ReadFailureIsReturned(t *testing.T) {
	db := newModuleTestDB(t)
	failQueriesOn(db, "settings")

	_, err := models.UnrecognisedCerberusSwitchKeys(db)

	require.Error(t, err)
}

func ptrString(s string) *string { return &s }
