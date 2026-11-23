package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestRedirectionHostRestoreUpdatedPutsTheRowBack(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-up", DomainNames: "redir-up.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	current.TargetURL = "https://b.example.com"
	_, written, err := service.UpdateAsSnapshot(current, true)
	require.NoError(t, err)

	require.NoError(t, service.RestoreUpdated(before, written))
	after, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://a.example.com", after.TargetURL)
}

func TestRedirectionHostRestoreUpdatedKeepsANewerWrite(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-newer", DomainNames: "redir-newer.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	current, err := service.GetByID(host.ID)
	require.NoError(t, err)
	current.TargetURL = "https://b.example.com"
	_, written, err := service.UpdateAsSnapshot(current, true)
	require.NoError(t, err)
	newer, err := service.GetByID(host.ID)
	require.NoError(t, err)
	newer.TargetURL = "https://c.example.com"
	require.NoError(t, service.UpdateAs(newer, true))

	assert.ErrorIs(t, service.RestoreUpdated(before, written), ErrRestoreSuperseded)
	after, err := service.GetByID(host.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://c.example.com", after.TargetURL)
}

func TestRedirectionHostRemoveCreatedAndReinsertDeleted(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-cycle", DomainNames: "redir-cycle.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	written, err := service.CreateAsSnapshot(&host, true)
	require.NoError(t, err)

	require.NoError(t, service.RemoveCreated(written))
	_, err = service.GetByID(written.ID)
	require.Error(t, err)

	require.NoError(t, service.ReinsertDeleted(written))
	back, err := service.GetByID(written.ID)
	require.NoError(t, err)
	assert.Equal(t, "redir-cycle", back.UUID)
}

func TestRedirectionHostRemoveCreatedKeepsAHostEditedSinceTheCreate(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-edited", DomainNames: "redir-edited.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	written, err := service.CreateAsSnapshot(&host, true)
	require.NoError(t, err)
	current, err := service.GetByID(written.ID)
	require.NoError(t, err)
	current.Name = "edited"
	require.NoError(t, service.UpdateAs(current, true))

	assert.ErrorIs(t, service.RemoveCreated(written), ErrRestoreSuperseded)
	_, err = service.GetByID(written.ID)
	assert.NoError(t, err)
}

func TestRedirectionHostReinsertDeletedSkipsWhenTheIDExists(t *testing.T) {
	t.Parallel()
	db := setupDomainUniquenessTestDB(t)
	service := NewRedirectionHostService(db)
	host := models.RedirectionHost{UUID: "redir-exists", DomainNames: "redir-exists.example.com", TargetURL: "https://a.example.com", StatusCode: 301}
	require.NoError(t, service.CreateAs(&host, true))
	before, err := service.Snapshot(host.ID)
	require.NoError(t, err)

	assert.ErrorIs(t, service.ReinsertDeleted(before), ErrRestoreSuperseded)
}

func TestRedirectionHostSnapshotOfMissingHostFails(t *testing.T) {
	t.Parallel()
	service := NewRedirectionHostService(setupDomainUniquenessTestDB(t))
	_, err := service.Snapshot(404)
	assert.Error(t, err)
}
