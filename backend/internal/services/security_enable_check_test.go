package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCheckCerberusEnable describes the enable check on the stored row: a row that does not exist has nothing to check,
// a break-glass token must match the stored hash, and without a token the address must be on the allow-list.
func TestCheckCerberusEnable(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("open-sesame"), bcrypt.MinCost)
	require.NoError(t, err)
	stored := models.SecurityConfig{ID: 1, AdminWhitelist: "192.0.2.0/24", BreakGlassHash: string(hash)}

	tests := []struct {
		name   string
		row    models.SecurityConfig
		ip     string
		token  string
		expect error
	}{
		{name: "row not yet stored", row: models.SecurityConfig{}, ip: "198.51.100.1"},
		{name: "empty allow-list and no token", row: models.SecurityConfig{ID: 1}, ip: "192.0.2.10", expect: ErrAdminWhitelistMissing},
		{name: "address in the range", row: stored, ip: "192.0.2.10"},
		{name: "address outside the range", row: stored, ip: "198.51.100.1", expect: ErrAdminIPNotAllowed},
		{name: "matching token from any address", row: stored, ip: "198.51.100.1", token: "open-sesame"},
		{name: "wrong token", row: stored, ip: "192.0.2.10", token: "nope", expect: ErrBreakGlassInvalid},
		{name: "token against a row with no hash", row: models.SecurityConfig{ID: 1, AdminWhitelist: "192.0.2.0/24"}, ip: "192.0.2.10", token: "open-sesame", expect: ErrBreakGlassInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckCerberusEnable(tt.row, tt.ip, tt.token)
			if tt.expect == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.expect)
		})
	}
}

// TestAdminWhitelistContains describes the allow-list match: an exact address, a CIDR range, and blank entries that are
// skipped.
func TestAdminWhitelistContains(t *testing.T) {
	tests := []struct {
		name      string
		whitelist string
		ip        string
		want      bool
	}{
		{name: "exact address", whitelist: "192.0.2.10", ip: "192.0.2.10", want: true},
		{name: "address in a range", whitelist: "10.0.0.0/8, 192.0.2.0/24", ip: "192.0.2.77", want: true},
		{name: "blank entries are skipped", whitelist: ",, 192.0.2.10 ,", ip: "192.0.2.10", want: true},
		{name: "address outside every entry", whitelist: "192.0.2.0/24", ip: "198.51.100.1", want: false},
		{name: "unparseable caller address", whitelist: "192.0.2.0/24", ip: "not-an-ip", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, adminWhitelistContains(tt.whitelist, tt.ip))
		})
	}
}

// TestBreakGlassTokenMatches describes the token comparison used inside a write transaction: an empty hash never matches
// and a stored bcrypt hash matches only its own token.
func TestBreakGlassTokenMatches(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("open-sesame"), bcrypt.MinCost)
	require.NoError(t, err)

	assert.True(t, BreakGlassTokenMatches(string(hash), "open-sesame"))
	assert.False(t, BreakGlassTokenMatches(string(hash), "other"))
	assert.False(t, BreakGlassTokenMatches("", ""), "an empty hash never matches")
}
