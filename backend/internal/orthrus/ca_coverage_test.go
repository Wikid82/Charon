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

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, mode))
}

func TestLoadCA_CorruptKeyBlock_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keys", "hecate-ca.key")
	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")

	// Non-PEM content causes pem.Decode to return nil block.
	writeFile(t, keyPath, []byte("not-a-pem-block"), 0o600)
	writeFile(t, certPath, []byte("not-a-pem-block"), 0o644)

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode CA key PEM")
}

func TestLoadCA_InvalidKeyDER_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keys", "hecate-ca.key")
	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")

	// Valid PEM block but garbage DER — ParseECPrivateKey will fail.
	badKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
	writeFile(t, keyPath, badKeyPEM, 0o600)
	writeFile(t, certPath, []byte("placeholder"), 0o644)

	_, err := NewInternalCA(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse CA key")
}

func TestLoadCA_CorruptCertBlock_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keys", "hecate-ca.key")
	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")

	// Write a real ECDSA key so ReadFile + Decode + Parse succeed.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	writeFile(t, keyPath, keyPEM, 0o600)

	// Non-PEM cert — certBlock will be nil.
	writeFile(t, certPath, []byte("not-a-pem-block"), 0o644)

	_, err = NewInternalCA(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode CA cert PEM")
}

func TestLoadCA_InvalidCertDER_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keys", "hecate-ca.key")
	certPath := filepath.Join(dir, "keys", "hecate-ca.crt")

	// Write a real ECDSA key.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	writeFile(t, keyPath, keyPEM, 0o600)

	// Valid PEM block but garbage DER — ParseCertificate will fail.
	badCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
	writeFile(t, certPath, badCertPEM, 0o644)

	_, err = NewInternalCA(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse CA cert")
}

func TestSignCSR_InvalidDER_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	ca, err := NewInternalCA(dir)
	require.NoError(t, err)

	// PEM block with correct type but garbage DER bytes.
	invalidCSR := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: []byte("garbage-der-bytes"),
	})

	_, err = ca.SignCSR(invalidCSR, 24*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse CSR")
}

func makeCSRWithBadSignature(t *testing.T) []byte {
	t.Helper()

	// Create a valid CSR.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test-tampered"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	require.NoError(t, err)

	// Tamper with the last few bytes of the DER to corrupt the signature.
	tampered := make([]byte, len(csrDER))
	copy(tampered, csrDER)
	for i := 1; i <= 16; i++ {
		tampered[len(tampered)-i] ^= 0xFF
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: tampered})
}

func TestSignCSR_TamperedSignature_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	ca, err := NewInternalCA(dir)
	require.NoError(t, err)

	tamperedCSR := makeCSRWithBadSignature(t)
	_, err = ca.SignCSR(tamperedCSR, 24*time.Hour)
	assert.Error(t, err)
}
