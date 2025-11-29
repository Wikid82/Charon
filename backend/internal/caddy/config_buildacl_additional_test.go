package caddy

import (
	"encoding/json"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestBuildACLHandler_GeoBlacklist(t *testing.T) {
	acl := &models.AccessList{Type: "geo_blacklist", CountryCodes: "GB,FR", Enabled: true}
	h, err := buildACLHandler(acl)
	require.NoError(t, err)
	require.NotNil(t, h)
	b, _ := json.Marshal(h)
	require.Contains(t, string(b), "Access denied: Geographic restriction")
}

func TestBuildACLHandler_UnknownTypeReturnsNil(t *testing.T) {
	acl := &models.AccessList{Type: "unknown_type", Enabled: true}
	h, err := buildACLHandler(acl)
	require.NoError(t, err)
	require.Nil(t, h)
}
