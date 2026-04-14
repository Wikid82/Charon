package caddy

import (
	"encoding/base64"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEncSvc(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	svc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	return svc
}

// Test: encrypted key with encryption service → decrypt success → cert loaded
func TestGenerateConfig_CustomCert_EncryptedKey(t *testing.T) {
	encSvc := newTestEncSvc(t)
	encKey, err := encSvc.Encrypt([]byte("-----BEGIN PRIVATE KEY-----\nfake-key-data\n-----END PRIVATE KEY-----"))
	require.NoError(t, err)

	certID := uint(10)
	hosts := []models.ProxyHost{
		{
			UUID: "h-enc", DomainNames: "enc.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-enc", Name: "EncCert", Provider: "custom",
				Certificate:         "-----BEGIN CERTIFICATE-----\nfake-cert\n-----END CERTIFICATE-----",
				PrivateKeyEncrypted: encKey,
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil, encSvc)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
	assert.NotEmpty(t, cfg.Apps.TLS.Certificates.LoadPEM)
}

// Test: encrypted key with no encryption service → skip
func TestGenerateConfig_CustomCert_EncryptedKeyNoEncSvc(t *testing.T) {
	certID := uint(11)
	hosts := []models.ProxyHost{
		{
			UUID: "h-noenc", DomainNames: "noenc.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-noenc", Name: "NoEncSvcCert", Provider: "custom",
				Certificate:         "-----BEGIN CERTIFICATE-----\nfake-cert\n-----END CERTIFICATE-----",
				PrivateKeyEncrypted: "encrypted-data-here",
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	// Cert should be skipped - no TLS certs loaded
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		assert.Empty(t, cfg.Apps.TLS.Certificates.LoadPEM)
	}
}

// Test: no key at all → skip
func TestGenerateConfig_CustomCert_NoKey(t *testing.T) {
	certID := uint(12)
	hosts := []models.ProxyHost{
		{
			UUID: "h-nokey", DomainNames: "nokey.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-nokey", Name: "NoKeyCert", Provider: "custom",
				Certificate: "-----BEGIN CERTIFICATE-----\nfake-cert\n-----END CERTIFICATE-----",
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		assert.Empty(t, cfg.Apps.TLS.Certificates.LoadPEM)
	}
}

// Test: missing cert PEM → skip
func TestGenerateConfig_CustomCert_NoCertPEM(t *testing.T) {
	certID := uint(13)
	hosts := []models.ProxyHost{
		{
			UUID: "h-nocert", DomainNames: "nocert.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-nocert", Name: "NoCertPEM", Provider: "custom",
				PrivateKey: "some-key",
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		assert.Empty(t, cfg.Apps.TLS.Certificates.LoadPEM)
	}
}

// Test: cert with chain → chain concatenated
func TestGenerateConfig_CustomCert_WithChain(t *testing.T) {
	certID := uint(14)
	hosts := []models.ProxyHost{
		{
			UUID: "h-chain", DomainNames: "chain.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-chain", Name: "ChainCert", Provider: "custom",
				Certificate:      "-----BEGIN CERTIFICATE-----\nleaf-cert\n-----END CERTIFICATE-----",
				PrivateKey:       "-----BEGIN PRIVATE KEY-----\nkey-data\n-----END PRIVATE KEY-----",
				CertificateChain: "-----BEGIN CERTIFICATE-----\nca-cert\n-----END CERTIFICATE-----",
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Apps.TLS)
	require.NotNil(t, cfg.Apps.TLS.Certificates)
	require.NotEmpty(t, cfg.Apps.TLS.Certificates.LoadPEM)
	assert.Contains(t, cfg.Apps.TLS.Certificates.LoadPEM[0].Certificate, "ca-cert")
}

// Test: decrypt failure → skip
func TestGenerateConfig_CustomCert_DecryptFailure(t *testing.T) {
	encSvc := newTestEncSvc(t)
	certID := uint(15)
	hosts := []models.ProxyHost{
		{
			UUID: "h-decfail", DomainNames: "decfail.test", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true,
			CertificateID: &certID,
			Certificate: &models.SSLCertificate{
				ID: certID, UUID: "c-decfail", Name: "DecryptFail", Provider: "custom",
				Certificate:         "-----BEGIN CERTIFICATE-----\nfake-cert\n-----END CERTIFICATE-----",
				PrivateKeyEncrypted: "not-valid-encrypted-data",
			},
		},
	}

	cfg, err := GenerateConfig(hosts, "/data", "admin@test.com", "/dist", "letsencrypt", true, false, false, false, false, "", nil, nil, nil, nil, nil, encSvc)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	if cfg.Apps.TLS != nil && cfg.Apps.TLS.Certificates != nil {
		assert.Empty(t, cfg.Apps.TLS.Certificates.LoadPEM)
	}
}
