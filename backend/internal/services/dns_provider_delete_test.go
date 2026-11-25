package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/crypto/cryptotest"
	"github.com/Wikid82/charon/backend/internal/models"
)

func newDNSDeleteTestService(t *testing.T) (DNSProviderService, *testDNSFixture) {
	t.Helper()
	db := openReferenceEnforcedDB(t)
	encryptor, err := crypto.NewEncryptionService("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	require.NoError(t, err)
	svc := NewDNSProviderService(db, cryptotest.SingleKey(encryptor))
	return svc, &testDNSFixture{db: db}
}

type testDNSFixture struct{ db *gorm.DB }

func (f *testDNSFixture) provider(t *testing.T) *models.DNSProvider {
	t.Helper()
	p := &models.DNSProvider{UUID: uuid.NewString(), Name: "cloudflare", ProviderType: "cloudflare", Enabled: true}
	require.NoError(t, f.db.Create(p).Error)
	return p
}

// TestDNSProviderDelete_RemovesCredentials covers a provider with stored credentials and no hosts.
func TestDNSProviderDelete_RemovesCredentials(t *testing.T) {
	svc, f := newDNSDeleteTestService(t)
	p := f.provider(t)
	require.NoError(t, f.db.Create(&models.DNSProviderCredential{
		UUID: uuid.NewString(), DNSProviderID: p.ID, Label: "main", Enabled: true,
	}).Error)

	require.NoError(t, svc.Delete(context.Background(), p.ID))

	var providers, credentials int64
	require.NoError(t, f.db.Model(&models.DNSProvider{}).Count(&providers).Error)
	require.NoError(t, f.db.Model(&models.DNSProviderCredential{}).Count(&credentials).Error)
	assert.Zero(t, providers)
	assert.Zero(t, credentials)
}

// TestDNSProviderDelete_RefusedWhileHostsUseIt covers proxy and redirection hosts that still reference the provider.
func TestDNSProviderDelete_RefusedWhileHostsUseIt(t *testing.T) {
	svc, f := newDNSDeleteTestService(t)
	p := f.provider(t)
	providerID := p.ID
	require.NoError(t, f.db.Create(&models.ProxyHost{
		UUID: uuid.NewString(), DomainNames: "a.example.com", ForwardHost: "10.0.0.1", ForwardPort: 80,
		DNSProviderID: &providerID, UseDNSChallenge: true,
	}).Error)
	require.NoError(t, f.db.Create(&models.RedirectionHost{
		UUID: uuid.NewString(), DomainNames: "b.example.com", TargetURL: "https://c.example.com",
		DNSProviderID: &providerID,
	}).Error)
	require.NoError(t, f.db.Create(&models.DNSProviderCredential{
		UUID: uuid.NewString(), DNSProviderID: p.ID, Label: "main", Enabled: true,
	}).Error)

	err := svc.Delete(context.Background(), p.ID)

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrDNSProviderInUse))
	var inUse *DNSProviderInUseError
	require.True(t, errors.As(err, &inUse))
	assert.Equal(t, int64(2), inUse.HostCount)

	var providers, credentials int64
	require.NoError(t, f.db.Model(&models.DNSProvider{}).Count(&providers).Error)
	require.NoError(t, f.db.Model(&models.DNSProviderCredential{}).Count(&credentials).Error)
	assert.Equal(t, int64(1), providers, "the provider must survive a refused delete")
	assert.Equal(t, int64(1), credentials, "credentials must survive a refused delete")
}

// TestDNSProviderDelete_MissingProvider keeps the not-found result for a provider that does not exist.
func TestDNSProviderDelete_MissingProvider(t *testing.T) {
	svc, f := newDNSDeleteTestService(t)
	p := f.provider(t)
	require.NoError(t, f.db.Delete(&models.DNSProvider{}, p.ID).Error)

	err := svc.Delete(context.Background(), p.ID)
	assert.ErrorIs(t, err, ErrDNSProviderNotFound)
}
