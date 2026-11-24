package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// storeZeroValues writes the zero value into every column that carries a default tag and is not zero by default, so
// the stored row holds what a user saved, not what the column default would supply.
func storeZeroValues(t *testing.T, db *gorm.DB, hostID uint) {
	t.Helper()
	require.NoError(t, db.Model(&models.ProxyHost{}).Where("id = ?", hostID).Updates(map[string]any{
		"enabled":                  false,
		"block_exploits":           false,
		"http2_support":            false,
		"security_headers_enabled": false,
		"forward_scheme":           "",
	}).Error)
	require.NoError(t, db.Model(&models.Location{}).Where("proxy_host_id = ?", hostID).Updates(map[string]any{
		"forward_scheme": "",
	}).Error)
}

func TestProxyHostReinsertDeletedKeepsEveryStoredValue(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "reinsert-values", "reinsert-values.example.com")
	storeZeroValues(t, db, host.ID)

	removed, err := service.DeleteSnapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.ReinsertDeleted(removed))

	after, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, storedStateOf(removed), storedStateOf(after), "every stored value comes back as it was")
	assert.False(t, after.Host.Enabled)
	assert.False(t, after.Host.BlockExploits)
	assert.False(t, after.Host.HTTP2Support)
	assert.False(t, after.Host.SecurityHeadersEnabled)
	assert.Empty(t, after.Host.ForwardScheme)
	require.Len(t, after.Locations, 2)
	assert.Empty(t, after.Locations[0].ForwardScheme, "location rows keep their stored scheme")
}

func TestRedirectionHostReinsertDeletedKeepsEveryStoredValue(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-values", DomainNames: "redir-values.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))
	require.NoError(t, db.Model(&models.RedirectionHost{}).Where("id = ?", host.ID).Updates(map[string]any{
		"enabled":       false,
		"http2_support": false,
		"ssl_forced":    false,
		"preserve_path": false,
	}).Error)

	removed, err := service.DeleteSnapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, service.ReinsertDeleted(removed))

	after, err := service.Snapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, storedRedirectionState(removed), storedRedirectionState(after), "every stored value comes back as it was")
	assert.False(t, after.Enabled)
	assert.False(t, after.HTTP2Support)
	assert.False(t, after.SSLForced)
	assert.False(t, after.PreservePath)
}

func TestProxyHostDeleteSnapshotReturnsTheStateItRemoved(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, written := seedRestoreHost(t, service, "delete-snap", "delete-snap.example.com")

	removed, err := service.DeleteSnapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, storedStateOf(written), storedStateOf(removed))

	_, err = service.DeleteSnapshot(host.ID)
	require.Error(t, err, "a host that is already gone cannot be deleted again")
}

func TestRedirectionHostDeleteSnapshotReturnsTheStateItRemoved(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-del-snap", DomainNames: "redir-del-snap.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))

	removed, err := service.DeleteSnapshot(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://a.example.com", removed.TargetURL)

	_, err = service.DeleteSnapshot(host.ID)
	require.Error(t, err)
}

// TestProxyHostTwoWritersRestoreKeepsTheAcknowledgedChange covers the interleaving where writer A loads the host before
// writer B's change lands, B's apply succeeds, and then A's write is refused. The before state A restores must be the
// state B left, so the restore keeps B's acknowledged change.
func TestProxyHostTwoWritersRestoreKeepsTheAcknowledgedChange(t *testing.T) {
	t.Parallel()
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)
	host, _ := seedRestoreHost(t, service, "two-writers", "two-writers.example.com")

	// Writer A reads the host (the original state S0) before writer B writes.
	staleA, err := service.GetByID(host.ID)
	require.NoError(t, err)

	// Writer B writes S1 and its apply succeeds, so nothing is restored.
	writerB, err := service.GetByID(host.ID)
	require.NoError(t, err)
	writerB.ForwardPort = 9001
	_, _, err = service.UpdateAsSnapshot(writerB, true)
	require.NoError(t, err)

	// Writer A writes S2 and its apply is refused.
	staleA.ForwardPort = 9002
	before, written, err := service.UpdateAsSnapshot(staleA, true)
	require.NoError(t, err)
	assert.Equal(t, 9001, before.Host.ForwardPort, "the before state is read at A's write, so it is B's state")

	require.NoError(t, service.RestoreUpdated(before, written))
	after, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, 9001, after.ForwardPort, "restore puts back S1 and B's acknowledged change survives")
}

func TestRedirectionHostTwoWritersRestoreKeepsTheAcknowledgedChange(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-two", DomainNames: "redir-two.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))

	staleA, err := service.GetByID(host.ID)
	require.NoError(t, err)
	writerB, err := service.GetByID(host.ID)
	require.NoError(t, err)
	writerB.TargetURL = "https://b.example.com"
	_, _, err = service.UpdateAsSnapshot(writerB, true)
	require.NoError(t, err)

	staleA.TargetURL = "https://a2.example.com"
	before, written, err := service.UpdateAsSnapshot(staleA, true)
	require.NoError(t, err)
	assert.Equal(t, "https://b.example.com", before.TargetURL)

	require.NoError(t, service.RestoreUpdated(before, written))
	after, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://b.example.com", after.TargetURL)
}
