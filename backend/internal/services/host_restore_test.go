package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// seedRestoreHost stores a host with two locations through CreateAsSnapshot and returns the host and the snapshot
// the create left.
func seedRestoreHost(t *testing.T, service *ProxyHostService, uuid, domain string) (models.ProxyHost, ProxyHostSnapshot) {
	t.Helper()
	host := models.ProxyHost{
		UUID:        uuid,
		DomainNames: domain,
		ForwardHost: "10.0.0.1",
		ForwardPort: 8080,
		Locations: []models.Location{
			{UUID: uuid + "-a", Path: "/a", ForwardHost: "10.0.0.1", ForwardPort: 9001},
			{UUID: uuid + "-b", Path: "/b", ForwardHost: "10.0.0.1", ForwardPort: 9002},
		},
	}
	written, err := service.CreateAsSnapshot(&host, true)
	require.NoError(t, err)
	return host, written
}

func locationPaths(locations []models.Location) []string {
	paths := make([]string, 0, len(locations))
	for _, loc := range locations {
		paths = append(paths, loc.Path)
	}
	return paths
}

func TestProxyHostCreateAsSnapshotReturnsTheStoredState(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	host, written := seedRestoreHost(t, NewProxyHostService(db), "snap-create", "snap-create.example.com")

	assert.Equal(t, host.ID, written.Host.ID)
	assert.Equal(t, []string{"/a", "/b"}, locationPaths(written.Locations))
}

func TestProxyHostSnapshotOfMissingHostFails(t *testing.T) {
	t.Parallel()
	service := NewProxyHostService(setupProxyHostTestDB(t))
	_, err := service.Snapshot(9999)
	assert.Error(t, err)
}

func TestProxyHostRestoreUpdatedPutsTheRowAndLocationsBack(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "restore-up", "restore-up.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	current.ForwardPort = 9999
	written, err := service.UpdateAsSnapshot(current, true)
	require.NoError(t, err)

	require.NoError(t, service.RestoreUpdated(before, written))

	after, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, 8080, after.Host.ForwardPort)
	assert.Equal(t, []string{"/a", "/b"}, locationPaths(after.Locations))
	assert.Equal(t, before.Locations[0].ID, after.Locations[0].ID, "the original location ids are kept")
}

func TestProxyHostRestoreUpdatedKeepsANewerWrite(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "restore-newer", "restore-newer.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	current.ForwardPort = 9999
	written, err := service.UpdateAsSnapshot(current, true)
	require.NoError(t, err)

	// A newer valid write lands after the refused one.
	newer, err := service.GetByID(host.ID)
	require.NoError(t, err)
	newer.ForwardHost = "10.5.5.5"
	require.NoError(t, service.UpdateAs(newer, true))

	err = service.RestoreUpdated(before, written)
	require.ErrorIs(t, err, ErrRestoreSuperseded)

	after, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "10.5.5.5", after.ForwardHost)
	assert.Equal(t, 9999, after.ForwardPort)
}

func TestProxyHostRemoveCreatedRemovesRowAndLocations(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "remove-created", "remove-created.example.com")

	require.NoError(t, service.RemoveCreated(written))

	var hosts, locations int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Where("id = ?", host.ID).Count(&hosts).Error)
	require.NoError(t, db.Model(&models.Location{}).Where("proxy_host_id = ?", host.ID).Count(&locations).Error)
	assert.Zero(t, hosts)
	assert.Zero(t, locations)
}

func TestProxyHostRemoveCreatedKeepsAHostEditedSinceTheCreate(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "remove-edited", "remove-edited.example.com")
	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	current.Name = "edited since"
	require.NoError(t, service.UpdateAs(current, true))

	err = service.RemoveCreated(written)

	require.ErrorIs(t, err, ErrRestoreSuperseded)
	_, err = service.GetByID(host.ID)
	assert.NoError(t, err, "the edited host is kept")
}

func TestProxyHostRemoveCreatedOfAMissingRowIsSuperseded(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "remove-gone", "remove-gone.example.com")
	require.NoError(t, service.Delete(host.ID))

	assert.ErrorIs(t, service.RemoveCreated(written), ErrRestoreSuperseded)
}

func TestProxyHostReinsertDeletedRestoresIdsAndLocations(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert", "reinsert.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.Delete(host.ID))

	require.NoError(t, service.ReinsertDeleted(before))

	after, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "reinsert", after.Host.UUID)
	assert.Equal(t, []string{"/a", "/b"}, locationPaths(after.Locations))
	assert.Equal(t, before.Locations[1].ID, after.Locations[1].ID)
}

func TestProxyHostReinsertDeletedSkipsWhenTheUUIDExists(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert-uuid", "reinsert-uuid.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	err = service.ReinsertDeleted(before)

	require.ErrorIs(t, err, ErrRestoreSuperseded, "the host still exists")
}

func TestProxyHostReinsertDeletedSkipsWhenADomainIsClaimed(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert-claimed", "reinsert-claimed.example.com")
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.Delete(host.ID))
	claim := models.ProxyHost{UUID: "claimer", DomainNames: "reinsert-claimed.example.com", ForwardHost: "10.0.0.2", ForwardPort: 1}
	require.NoError(t, db.Create(&claim).Error)

	err = service.ReinsertDeleted(before)

	require.ErrorIs(t, err, ErrRestoreSuperseded)
}

func TestProxyHostRestoreColumnRestoresUnchangedRowsAndKeepsEditedOnes(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	first, _ := seedRestoreHost(t, service, "column-one", "column-one.example.com")
	second, _ := seedRestoreHost(t, service, "column-two", "column-two.example.com")

	var changes []ColumnChange
	for _, id := range []uint{first.ID, second.ID} {
		current, err := service.GetByID(id)
		require.NoError(t, err)
		next := uint(4)
		current.AccessListID = &next
		written, err := service.UpdateAsSnapshot(current, true)
		require.NoError(t, err)
		changes = append(changes, ColumnChange{ID: id, Old: nil, WrittenAt: written.Host.UpdatedAt})
	}
	// The second host is edited again after the bulk write.
	edited, err := service.GetByID(second.ID)
	require.NoError(t, err)
	other := uint(5)
	edited.AccessListID = &other
	require.NoError(t, service.UpdateAs(edited, true))

	err = service.RestoreColumn("access_list_id", changes)

	require.ErrorIs(t, err, ErrRestoreSuperseded)
	restored, err := service.GetByID(first.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.AccessListID, "the unchanged row is restored")
	kept, err := service.GetByID(second.ID)
	require.NoError(t, err)
	require.NotNil(t, kept.AccessListID)
	assert.Equal(t, uint(5), *kept.AccessListID, "the edited row keeps its newer value")
}

func TestProxyHostRestoreColumnRefusesColumnsOutsideTheAllowList(t *testing.T) {
	t.Parallel()
	service := NewProxyHostService(setupProxyHostTestDB(t))
	err := service.RestoreColumn("forward_port", nil)
	assert.Error(t, err)
}
