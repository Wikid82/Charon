package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestProxyHostRemoveCreated_RemovesMonitorsWithHost covers a periodic check that created a monitor for a host whose
// create is being refused: the monitor, its heartbeats and the host are all removed.
func TestProxyHostRemoveCreated_RemovesMonitorsWithHost(t *testing.T) {
	db := openReferenceEnforcedDB(t)
	require.NoError(t, db.AutoMigrate(&models.RemoteServer{}, &models.UptimeHost{}, &models.UptimeMonitor{}, &models.UptimeHeartbeat{}))
	svc := NewProxyHostService(db)

	host := &models.ProxyHost{UUID: uuid.NewString(), DomainNames: "monitor.example.com", ForwardHost: "10.0.0.4", ForwardPort: 80}
	written, err := svc.CreateAsSnapshot(host, true)
	require.NoError(t, err)

	hostID := host.ID
	monitor := models.UptimeMonitor{ID: uuid.NewString(), ProxyHostID: &hostID, Name: "monitor", Type: "http", Enabled: true}
	require.NoError(t, db.Create(&monitor).Error)
	require.NoError(t, db.Create(&models.UptimeHeartbeat{MonitorID: monitor.ID, Status: "up"}).Error)

	require.NoError(t, svc.RemoveCreated(written))

	var hosts, monitors, heartbeats int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Count(&hosts).Error)
	require.NoError(t, db.Model(&models.UptimeMonitor{}).Count(&monitors).Error)
	require.NoError(t, db.Model(&models.UptimeHeartbeat{}).Count(&heartbeats).Error)
	assert.Zero(t, hosts)
	assert.Zero(t, monitors)
	assert.Zero(t, heartbeats)
}

// TestProxyHostReinsertDeleted_ClearsMissingReferences covers a host whose certificate and DNS provider were deleted
// after the host was deleted: the host comes back with those references cleared and the DNS challenge off.
func TestProxyHostReinsertDeleted_ClearsMissingReferences(t *testing.T) {
	db := openReferenceEnforcedDB(t)
	svc := NewProxyHostService(db)

	cert := models.SSLCertificate{UUID: uuid.NewString(), Name: "cert", Domains: "back.example.com", Provider: "custom"}
	provider := models.DNSProvider{UUID: uuid.NewString(), Name: "dns", ProviderType: "cloudflare", Enabled: true}
	require.NoError(t, db.Create(&cert).Error)
	require.NoError(t, db.Create(&provider).Error)
	certID, providerID := cert.ID, provider.ID
	host := &models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "back.example.com", ForwardHost: "10.0.0.5", ForwardPort: 80,
		CertificateID: &certID, DNSProviderID: &providerID, UseDNSChallenge: true,
	}
	require.NoError(t, db.Create(host).Error)

	snap, err := svc.DeleteSnapshot(host.ID)
	require.NoError(t, err)
	require.NoError(t, db.Delete(&models.SSLCertificate{}, certID).Error)
	require.NoError(t, db.Delete(&models.DNSProvider{}, providerID).Error)

	require.NoError(t, svc.ReinsertDeleted(snap))

	restored, err := svc.GetByID(host.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.CertificateID)
	assert.Nil(t, restored.DNSProviderID)
	assert.False(t, restored.UseDNSChallenge)
	assert.Equal(t, "back.example.com", restored.DomainNames)
}

// TestRedirectionReinsertDeleted_ClearsMissingReferences is the redirection host counterpart of the proxy host test.
func TestRedirectionReinsertDeleted_ClearsMissingReferences(t *testing.T) {
	db := openReferenceEnforcedDB(t)
	svc := NewRedirectionHostService(db)

	provider := models.DNSProvider{UUID: uuid.NewString(), Name: "dns", ProviderType: "cloudflare", Enabled: true}
	require.NoError(t, db.Create(&provider).Error)
	providerID := provider.ID
	redirect := &models.RedirectionHost{
		UUID: uuid.NewString(), DomainNames: "go.example.com", TargetURL: "https://target.example.com",
		DNSProviderID: &providerID, UseDNSChallenge: true,
	}
	require.NoError(t, db.Create(redirect).Error)

	snap, err := svc.DeleteSnapshot(redirect.ID)
	require.NoError(t, err)
	require.NoError(t, db.Delete(&models.DNSProvider{}, providerID).Error)

	require.NoError(t, svc.ReinsertDeleted(snap))

	var restored models.RedirectionHost
	require.NoError(t, db.First(&restored, redirect.ID).Error)
	assert.Nil(t, restored.DNSProviderID)
	assert.False(t, restored.UseDNSChallenge)
}
