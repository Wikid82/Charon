package orthrus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInternalCA_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	ca, err := NewInternalCA(dir)
	require.NoError(t, err)
	require.NotNil(t, ca)
	assert.NotNil(t, ca.caCert)
	assert.NotNil(t, ca.caKey)

	// PEM should be non-empty and parseable.
	pemBytes := ca.CACertPEM()
	require.NotEmpty(t, pemBytes)
	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block)
	_, err = x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
}

func TestNewInternalCA_LoadsExisting(t *testing.T) {
	dir := t.TempDir()

	ca1, err := NewInternalCA(dir)
	require.NoError(t, err)

	ca2, err := NewInternalCA(dir)
	require.NoError(t, err)

	// Both instances should represent the same CA.
	assert.Equal(t, ca1.caCert.SerialNumber, ca2.caCert.SerialNumber)
}

func makeCSR(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test-agent"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
}

func TestInternalCA_SignCSR_ProducesValidCert(t *testing.T) {
	dir := t.TempDir()
	ca, err := NewInternalCA(dir)
	require.NoError(t, err)

	csrPEM := makeCSR(t)

	certPEM, err := ca.SignCSR(csrPEM, 24*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, certPEM)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify the issued cert is signed by our CA.
	pool := x509.NewCertPool()
	pool.AddCert(ca.caCert)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	assert.NoError(t, err)
}

func TestInternalCA_SignCSR_InvalidPEM_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	ca, err := NewInternalCA(dir)
	require.NoError(t, err)

	_, err = ca.SignCSR([]byte("not-a-pem"), 24*time.Hour)
	assert.Error(t, err)
}

func TestNewInternalCA_CorruptKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "keys")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))

	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "hecate-ca.key"), []byte("not-valid-pem"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "hecate-ca.crt"), []byte("cert-placeholder"), 0o644))

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_CorruptCertFile(t *testing.T) {
	dir := t.TempDir()

	_, err := NewInternalCA(dir)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")
	require.NoError(t, os.WriteFile(certPath, []byte("not-valid-pem"), 0o644))

	_, err = NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_UnreadableKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "keys")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))

	keyPath := filepath.Join(keyDir, "hecate-ca.key")
	certPath := filepath.Join(keyDir, "hecate-ca.crt")
	require.NoError(t, os.WriteFile(keyPath, []byte("key-content"), 0o000))
	require.NoError(t, os.WriteFile(certPath, []byte("cert-content"), 0o644))

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_UnreadableCertFile(t *testing.T) {
	dir := t.TempDir()

	_, err := NewInternalCA(dir)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")
	require.NoError(t, os.Chmod(certPath, 0o000))

	_, err = NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_ReadOnlyDataRoot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_ReadOnlyKeysDir(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")
	require.NoError(t, os.MkdirAll(keysDir, 0o555))

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
}

func TestNewInternalCA_ReadOnlyCertPath(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")
	require.NoError(t, os.MkdirAll(keysDir, 0o700))

	certPath := filepath.Join(keysDir, "hecate-ca.crt")
	require.NoError(t, os.WriteFile(certPath, []byte("placeholder"), 0o000))

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
}
