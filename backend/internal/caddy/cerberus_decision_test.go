package caddy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// TestCerberusEnabledForProxy_KeyMatrix checks the extracted function against the same case list the manager
// matrix uses, passing the row the way the manager does (nil when no default row exists).
func TestCerberusEnabledForProxy_KeyMatrix(t *testing.T) {
	for _, tc := range cerberusDecisionCases() {
		name := "static=" + boolWord(tc.static) + "/row=" + tc.row + "/feature=" + tc.feature + "/legacy=" + tc.legacy
		t.Run(name, func(t *testing.T) {
			db := openCerberusDecisionDB(t, tc)
			var row *models.SecurityConfig
			if tc.row != crowdSecAbsent {
				found, err := models.FindDefaultSecurityConfig(db)
				require.NoError(t, err)
				row = &found
			}

			got := CerberusEnabledForProxy(db, config.SecurityConfig{CerberusEnabled: tc.static}, row)

			require.Equal(t, tc.wantOn, got)
		})
	}
}

// TestCerberusEnabledForProxy_NilDB pins the no-database path: only the static config and the row decide.
func TestCerberusEnabledForProxy_NilDB(t *testing.T) {
	require.True(t, CerberusEnabledForProxy(nil, config.SecurityConfig{CerberusEnabled: true}, nil))
	require.False(t, CerberusEnabledForProxy(nil, config.SecurityConfig{CerberusEnabled: false}, nil))
	require.True(t, CerberusEnabledForProxy(nil, config.SecurityConfig{}, &models.SecurityConfig{Enabled: true}))
	require.False(t, CerberusEnabledForProxy(nil, config.SecurityConfig{CerberusEnabled: true}, &models.SecurityConfig{Enabled: false}))
}
