package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// A geo list stored with no country codes (legacy rows) still produces a matcher with an empty set.
func TestBuildACLHandler_EmptyCountryCodesGenerateEmptySet(t *testing.T) {
	t.Run("whitelist blocks every request", func(t *testing.T) {
		handler, err := buildACLHandler(&models.AccessList{Type: "geo_whitelist", CountryCodes: ""}, "")
		require.NoError(t, err)

		raw, err := json.Marshal(handler)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"not"`)
		require.Contains(t, string(raw), "{geoip2.country_code} in []")
	})

	t.Run("blacklist blocks nothing", func(t *testing.T) {
		handler, err := buildACLHandler(&models.AccessList{Type: "geo_blacklist", CountryCodes: " , "}, "")
		require.NoError(t, err)

		raw, err := json.Marshal(handler)
		require.NoError(t, err)
		require.NotContains(t, string(raw), `"not"`)
		require.Contains(t, string(raw), "{geoip2.country_code} in []")
	})
}
