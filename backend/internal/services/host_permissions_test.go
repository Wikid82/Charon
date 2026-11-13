package services

import (
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const hostPermsAdvancedConfig = `{"handle":[{"handler":"headers"}]}`

func hostPermsDNSID(id uint) *uint { return &id }

func TestHostPermsProxyCreateNonAdminAdminOnlyFields(t *testing.T) {
	tests := []struct {
		name string
		host models.ProxyHost
	}{
		{
			name: "dns provider binding",
			host: models.ProxyHost{DomainNames: "dns-bind.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, DNSProviderID: hostPermsDNSID(1)},
		},
		{
			name: "dns challenge toggle",
			host: models.ProxyHost{DomainNames: "dns-toggle.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, UseDNSChallenge: true, DNSProviderID: hostPermsDNSID(1)},
		},
		{
			name: "advanced config",
			host: models.ProxyHost{DomainNames: "adv.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, AdvancedConfig: hostPermsAdvancedConfig},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupProxyHostTestDB(t)
			svc := NewProxyHostService(db)
			host := tt.host

			err := svc.CreateAs(&host, false)
			require.ErrorIs(t, err, ErrAdminOnlyHostField)

			var count int64
			require.NoError(t, db.Model(&models.ProxyHost{}).Count(&count).Error)
			assert.Zero(t, count, "a refused create must not persist anything")
		})
	}
}

func TestHostPermsProxyCreateAdminAllowed(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := models.ProxyHost{
		DomainNames:     "admin-dns.example.com",
		ForwardHost:     "10.0.0.5",
		ForwardPort:     8080,
		DNSProviderID:   hostPermsDNSID(1),
		AdvancedConfig:  hostPermsAdvancedConfig,
		UseDNSChallenge: true,
	}
	require.NoError(t, svc.CreateAs(&host, true))
	assert.NotZero(t, host.ID)
}

func TestHostPermsProxyCreateNonAdminPlainHostAllowed(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := models.ProxyHost{DomainNames: "plain.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080}
	require.NoError(t, svc.CreateAs(&host, false))
}

func TestHostPermsProxyUpdateNonAdmin(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	stored := models.ProxyHost{
		DomainNames:     "stored.example.com",
		ForwardHost:     "10.0.0.5",
		ForwardPort:     8080,
		DNSProviderID:   hostPermsDNSID(7),
		UseDNSChallenge: true,
		AdvancedConfig:  hostPermsAdvancedConfig,
	}
	require.NoError(t, svc.CreateAs(&stored, true))

	t.Run("edit leaving admin fields unchanged is allowed", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.Name = "renamed"
		host.ForwardPort = 9090
		require.NoError(t, svc.UpdateAs(host, false))
	})

	t.Run("changing dns provider is refused", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.DNSProviderID = hostPermsDNSID(8)
		require.ErrorIs(t, svc.UpdateAs(host, false), ErrAdminOnlyHostField)
	})

	t.Run("clearing dns provider is refused", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.DNSProviderID = nil
		host.UseDNSChallenge = false
		require.ErrorIs(t, svc.UpdateAs(host, false), ErrAdminOnlyHostField)
	})

	t.Run("changing advanced config is refused", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.AdvancedConfig = ""
		require.ErrorIs(t, svc.UpdateAs(host, false), ErrAdminOnlyHostField)
	})

	t.Run("reformatted but equivalent advanced config is allowed", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.AdvancedConfig = `{ "handle": [ { "handler": "headers" } ] }`
		require.NoError(t, svc.UpdateAs(host, false))
	})

	t.Run("admin may change admin fields", func(t *testing.T) {
		host, err := svc.GetByUUID(stored.UUID)
		require.NoError(t, err)
		host.DNSProviderID = hostPermsDNSID(9)
		host.AdvancedConfig = ""
		require.NoError(t, svc.UpdateAs(host, true))
	})
}

func TestHostPermsProxyUpdateMissingRowNonAdmin(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := &models.ProxyHost{ID: 4242, DomainNames: "ghost.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080}
	// No stored row: the stored admin fields are treated as empty, so an
	// unchanged (empty) admin field set is accepted and the write matches nothing.
	require.NoError(t, svc.UpdateAs(host, false))
}

func TestHostPermsProxyStoredAdminFieldsLoadError(t *testing.T) {
	db := setupProxyHostTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	_, err := storedProxyAdminFields(db, 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAdminOnlyHostField)
}

func TestHostPermsRedirectionNonAdmin(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	svc := NewRedirectionHostService(db)

	createNonAdmin := models.RedirectionHost{
		DomainNames:     "redir-dns.example.com",
		TargetURL:       "https://target.example.net",
		StatusCode:      301,
		DNSProviderID:   hostPermsDNSID(3),
		UseDNSChallenge: true,
	}
	require.ErrorIs(t, svc.CreateAs(&createNonAdmin, false), ErrAdminOnlyHostField)

	plain := models.RedirectionHost{
		DomainNames: "redir-plain.example.com",
		TargetURL:   "https://target.example.net",
		StatusCode:  301,
	}
	require.NoError(t, svc.CreateAs(&plain, false))

	stored := models.RedirectionHost{
		DomainNames:     "redir-stored.example.com",
		TargetURL:       "https://target.example.net",
		StatusCode:      301,
		DNSProviderID:   hostPermsDNSID(3),
		UseDNSChallenge: true,
	}
	require.NoError(t, svc.CreateAs(&stored, true))

	host, err := svc.GetByUUID(stored.UUID)
	require.NoError(t, err)
	host.Name = "renamed"
	require.NoError(t, svc.UpdateAs(host, false), "unchanged DNS fields must be accepted")

	host, err = svc.GetByUUID(stored.UUID)
	require.NoError(t, err)
	host.DNSProviderID = hostPermsDNSID(4)
	require.ErrorIs(t, svc.UpdateAs(host, false), ErrAdminOnlyHostField)

	host, err = svc.GetByUUID(stored.UUID)
	require.NoError(t, err)
	host.UseDNSChallenge = false
	host.DNSProviderID = nil
	require.ErrorIs(t, svc.UpdateAs(host, false), ErrAdminOnlyHostField)

	host.DNSProviderID = hostPermsDNSID(5)
	host.UseDNSChallenge = true
	require.NoError(t, svc.UpdateAs(host, true))
}

func TestHostPermsRedirectionStoredAdminFieldsLoadError(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.RedirectionHost{}))

	_, err := storedRedirectionAdminFields(db, 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAdminOnlyHostField)
}

func TestHostPermsAdminListenerTargetRefusedForEveryRole(t *testing.T) {
	hostname, _ := os.Hostname()
	targets := []string{"localhost", "LOCALHOST.", "127.0.0.1", "127.0.0.2", "::1", "0.0.0.0", "::"}
	if hostname != "" {
		targets = append(targets, hostname)
	}

	for _, admin := range []bool{true, false} {
		db := setupProxyHostTestDB(t)
		svc := NewProxyHostService(db)
		for _, target := range targets {
			host := &models.ProxyHost{DomainNames: "loop-" + target + ".example.com", ForwardHost: target, ForwardPort: 2019}
			require.ErrorIs(t, svc.CreateAs(host, admin), ErrAdminListenerTarget, "target %q admin=%v", target, admin)
		}
	}
}

func TestHostPermsAdminListenerTargetAllowedOnOtherPorts(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := &models.ProxyHost{DomainNames: "other-port.example.com", ForwardHost: "localhost", ForwardPort: 8080}
	require.NoError(t, svc.CreateAs(host, false))
}

func TestHostPermsAdminListenerTargetAllowedForRemoteHost(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := &models.ProxyHost{DomainNames: "remote-2019.example.com", ForwardHost: "192.0.2.10", ForwardPort: 2019}
	require.NoError(t, svc.CreateAs(host, false))
}

func TestHostPermsAdminListenerTargetStripsSchemeAndPath(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)
	host := &models.ProxyHost{DomainNames: "scheme-loop.example.com", ForwardHost: "http://localhost/admin", ForwardPort: 2019}
	require.ErrorIs(t, svc.CreateAs(host, false), ErrAdminListenerTarget)
}

func TestHostPermsAdminListenerRefusedInLocations(t *testing.T) {
	db := setupProxyHostTestDB(t)
	svc := NewProxyHostService(db)

	for _, admin := range []bool{true, false} {
		host := &models.ProxyHost{
			DomainNames: "loc-loop.example.com",
			ForwardHost: "10.0.0.5",
			ForwardPort: 8080,
			Locations: []models.Location{
				{Path: "/api", ForwardHost: "127.0.0.1", ForwardPort: 2019},
			},
		}
		require.ErrorIs(t, svc.CreateAs(host, admin), ErrAdminListenerTarget, "admin=%v", admin)
	}

	host := &models.ProxyHost{
		DomainNames: "loc-ok.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Locations:   []models.Location{{Path: "/api", ForwardHost: "10.0.0.6", ForwardPort: 8081}},
	}
	require.NoError(t, svc.CreateAs(host, false))
}

func TestHostPermsCanonicalAdvancedConfigFallsBackOnInvalidStoredJSON(t *testing.T) {
	assert.Equal(t, "not json", canonicalAdvancedConfig("not json"))
	assert.Empty(t, canonicalAdvancedConfig(""))
}
