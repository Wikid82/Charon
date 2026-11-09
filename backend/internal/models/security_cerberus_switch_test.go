package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCerberusSwitchFromSetting pins the shared reading of a stored Cerberus switch key. Blank, "1", "yes" and
// garbage are all Off here; the readers that treat some of them differently handle those values before calling it.
func TestCerberusSwitchFromSetting(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		stored bool
		want   models.CerberusSwitch
	}{
		{name: "not stored is absent", value: "", stored: false, want: models.CerberusSwitchAbsent},
		{name: "not stored ignores value", value: "true", stored: false, want: models.CerberusSwitchAbsent},
		{name: "true is on", value: "true", stored: true, want: models.CerberusSwitchOn},
		{name: "TRUE is on", value: "TRUE", stored: true, want: models.CerberusSwitchOn},
		{name: "false is off", value: "false", stored: true, want: models.CerberusSwitchOff},
		{name: "blank is off", value: "", stored: true, want: models.CerberusSwitchOff},
		{name: "1 is off here", value: "1", stored: true, want: models.CerberusSwitchOff},
		{name: "yes is off here", value: "yes", stored: true, want: models.CerberusSwitchOff},
		{name: "garbage is off", value: "garbage", stored: true, want: models.CerberusSwitchOff},
		{name: "padded true is off (no trim)", value: " true ", stored: true, want: models.CerberusSwitchOff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, models.CerberusSwitchFromSetting(tt.value, tt.stored))
		})
	}
}
