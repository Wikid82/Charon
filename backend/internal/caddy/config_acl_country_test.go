package caddy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestBuildACLHandler_CountryCodesEmittedUpperCase(t *testing.T) {
	cases := []struct {
		name    string
		aclType string
		codes   string
		want    string
	}{
		{name: "lower-case allow list", aclType: "geo_whitelist", codes: "us,ca", want: `in [\"US\", \"CA\"]`},
		{name: "mixed-case allow list", aclType: "geo_whitelist", codes: " Us , cA ", want: `in [\"US\", \"CA\"]`},
		{name: "lower-case block list", aclType: "geo_blacklist", codes: "ru,cn", want: `in [\"RU\", \"CN\"]`},
		{name: "mixed-case block list", aclType: "geo_blacklist", codes: "Ru, Cn", want: `in [\"RU\", \"CN\"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := buildACLHandler(&models.AccessList{Type: tc.aclType, CountryCodes: tc.codes}, "")
			require.NoError(t, err)

			raw, err := json.Marshal(handler)
			require.NoError(t, err)
			require.Contains(t, string(raw), "{geoip2.country_code} "+tc.want)
		})
	}
}
