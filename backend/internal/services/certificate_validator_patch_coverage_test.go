package services

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"software.sslmate.com/src/go-pkcs12"
)

func TestDetectFormat_PasswordProtectedPFX(t *testing.T) {
	cert, key, _, _ := makeRSACertAndKey(t, "pfx-pw.example.com", time.Now().Add(24*time.Hour))

	pfxData, err := pkcs12.Modern.Encode(key, cert, nil, "custompw")
	require.NoError(t, err)

	format := DetectFormat(pfxData)
	assert.Equal(t, FormatPFX, format)
}

func TestParsePEMPrivateKey_PKCS1RSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyDER := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})

	parsed, err := parsePEMPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestParsePEMPrivateKey_ECPrivKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	parsed, err := parsePEMPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestDetectKeyType_ECDSAP384(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "p384.example.com"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	assert.Equal(t, "ECDSA-P384", detectKeyType(cert))
}

func TestDetectKeyType_ECDSAUnknownCurve(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "p224.example.com"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	assert.Equal(t, "ECDSA", detectKeyType(cert))
}

func TestConvertPEMToPFX_EmptyChain(t *testing.T) {
	_, _, certPEM, keyPEM := makeRSACertAndKey(t, "pfx-chain.example.com", time.Now().Add(24*time.Hour))

	pfxData, err := ConvertPEMToPFX(string(certPEM), string(keyPEM), "", "testpass")
	require.NoError(t, err)
	assert.NotEmpty(t, pfxData)
}

func TestConvertPEMToDER_NonCertBlock(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	_, err = ConvertPEMToDER(string(keyPEM))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid certificate PEM")
}

func TestFormatSerial_NilInput(t *testing.T) {
	assert.Equal(t, "", formatSerial(nil))
}

func TestDetectFormat_EmptyPasswordPFX(t *testing.T) {
	cert, key, _, _ := makeRSACertAndKey(t, "empty-pw.example.com", time.Now().Add(24*time.Hour))

	pfxData, err := pkcs12.Modern.Encode(key, cert, nil, "")
	require.NoError(t, err)

	format := DetectFormat(pfxData)
	assert.Equal(t, FormatPFX, format)
}

func TestParseCertificateInput_BadChainPEM(t *testing.T) {
	_, _, certPEM, _ := makeRSACertAndKey(t, "bad-chain-test.example.com", time.Now().Add(24*time.Hour))

	badChain := []byte("-----BEGIN CERTIFICATE-----\naW52YWxpZA==\n-----END CERTIFICATE-----\n")

	_, err := ParseCertificateInput(certPEM, nil, badChain, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse chain PEM")
}

func TestValidateChain_WithIntermediates(t *testing.T) {
	cert, _, _, _ := makeRSACertAndKey(t, "chain-inter.example.com", time.Now().Add(24*time.Hour))

	_ = ValidateChain(cert, []*x509.Certificate{cert})
}

func TestConvertPEMToPFX_BadCertPEM(t *testing.T) {
	badCertPEM := "-----BEGIN CERTIFICATE-----\naW52YWxpZA==\n-----END CERTIFICATE-----\n"

	_, err := ConvertPEMToPFX(badCertPEM, "somekey", "", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse cert PEM")
}

func TestConvertPEMToPFX_BadChainPEM(t *testing.T) {
	_, _, certPEM, keyPEM := makeRSACertAndKey(t, "pfx-bad-chain.example.com", time.Now().Add(24*time.Hour))

	badChain := "-----BEGIN CERTIFICATE-----\naW52YWxpZA==\n-----END CERTIFICATE-----\n"

	_, err := ConvertPEMToPFX(string(certPEM), string(keyPEM), badChain, "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse chain PEM")
}

func TestParsePEMPrivateKey_PKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	parsed, err := parsePEMPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, parsed)
}

func TestEncodeKeyToPEM_UnsupportedKeyType(t *testing.T) {
	type badKey struct{}

	_, err := encodeKeyToPEM(badKey{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal private key")
}

func TestDetectKeyType_Unknown(t *testing.T) {
	cert := &x509.Certificate{
		PublicKey: "not-a-real-key",
	}
	assert.Equal(t, "Unknown", detectKeyType(cert))
}
