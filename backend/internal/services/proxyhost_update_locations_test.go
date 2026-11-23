package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Characterizes how UpdateAs persists the location rows of a host, so the restore path (host_restore.go) is
// written against what the write really does.
func TestProxyHostUpdateAsLocationsCharacterization(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host := &models.ProxyHost{
		UUID:        "loc-host",
		DomainNames: "loc.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
		Locations: []models.Location{
			{UUID: "loc-a", Path: "/a", ForwardHost: "127.0.0.1", ForwardPort: 9001},
			{UUID: "loc-b", Path: "/b", ForwardHost: "127.0.0.1", ForwardPort: 9002},
		},
	}
	require.NoError(t, service.CreateAs(host, true))

	// Change /a, drop /b, add /c.
	stored, err := service.GetByID(host.ID)
	require.NoError(t, err)
	require.NoError(t, db.Preload("Locations").First(stored, host.ID).Error)
	stored.Locations = []models.Location{
		{UUID: "loc-a", Path: "/a2", ForwardHost: "127.0.0.1", ForwardPort: 9001},
		{UUID: "loc-c", Path: "/c", ForwardHost: "127.0.0.1", ForwardPort: 9003},
	}
	require.NoError(t, service.UpdateAs(stored, true))

	var rows []models.Location
	require.NoError(t, db.Where("proxy_host_id = ?", host.ID).Order("path").Find(&rows).Error)
	got := map[string]string{}
	for _, r := range rows {
		got[r.UUID] = r.Path
	}
	// Observed: UpdateAs writes no location row at all. The added row is not inserted, the changed row keeps its
	// stored path and the removed row stays. Restore code therefore treats the rows as the snapshot.
	require.Equal(t, "/a", got["loc-a"], "a changed location is not rewritten")
	require.Contains(t, got, "loc-b", "a removed location is not deleted")
	require.NotContains(t, got, "loc-c", "an added location is not inserted")
	require.Len(t, rows, 2)
}
