package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAdminWhitelist(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		wantErr   error
	}{
		{name: "empty clears the list", whitelist: "", wantErr: nil},
		{name: "single IPv4 address", whitelist: "192.0.2.10", wantErr: nil},
		{name: "single IPv6 address", whitelist: "2001:db8::1", wantErr: nil},
		{name: "CIDR range", whitelist: "10.0.0.0/8", wantErr: nil},
		{name: "mixed entries with spaces", whitelist: " 10.0.0.0/8 , 192.0.2.1 ", wantErr: nil},
		{name: "empty entries are skipped", whitelist: "10.0.0.0/8,,", wantErr: nil},
		{name: "non-address entry is refused", whitelist: "not-an-ip", wantErr: ErrInvalidAdminCIDR},
		{name: "one bad entry refuses the list", whitelist: "10.0.0.0/8,999.1.1.1", wantErr: ErrInvalidAdminCIDR},
		{name: "malformed prefix is refused", whitelist: "10.0.0.0/99", wantErr: ErrInvalidAdminCIDR},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAdminWhitelist(tt.whitelist)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestResolveRateLimitMode(t *testing.T) {
	tests := []struct {
		name   string
		enable bool
		mode   string
		want   string
	}{
		{name: "enable true forces enabled", enable: true, mode: "", want: "enabled"},
		{name: "enable true overrides a stored disabled mode", enable: true, mode: "disabled", want: "enabled"},
		{name: "enable false with empty mode is disabled", enable: false, mode: "", want: "disabled"},
		{name: "enable false keeps a non-empty mode", enable: false, mode: "custom", want: "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ResolveRateLimitMode(tt.enable, tt.mode))
		})
	}
}
