package services

import (
	"crypto/ecdsa"
	"crypto/ed25519"
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
)

// --- ValidateKeyMatch ECDSA ---

func TestValidateKeyMatch_ECDSA_Success(t *testing.T) {
	cert, _, _, _ := makeECDSACertAndKey(t, "ecdsa-match.test")
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Use the actual key that signed the cert
	ecCert, ecKey, _, _ := makeECDSACertAndKey(t, "ecdsa-ok.test")
	err = ValidateKeyMatch(ecCert, ecKey)
	assert.NoError(t, err)

	// Mismatch: different ECDSA key
	err = ValidateKeyMatch(cert, priv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECDSA key mismatch")
}

func TestValidateKeyMatch_ECDSA_WrongKeyType(t *testing.T) {
	cert, _, _, _ := makeECDSACertAndKey(t, "ecdsa-wrong.test")
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	err = ValidateKeyMatch(cert, rsaKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key type mismatch")
}

// --- ValidateKeyMatch Ed25519 ---

func TestValidateKeyMatch_Ed25519_Success(t *testing.T) {
	cert, priv, _, _ := makeEd25519CertAndKey(t, "ed25519-ok.test")
	err := ValidateKeyMatch(cert, priv)
	assert.NoError(t, err)
}

func TestValidateKeyMatch_Ed25519_Mismatch(t *testing.T) {
	cert, _, _, _ := makeEd25519CertAndKey(t, "ed25519-mismatch.test")
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	err = ValidateKeyMatch(cert, otherPriv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Ed25519 key mismatch")
}

func TestValidateKeyMatch_Ed25519_WrongKeyType(t *testing.T) {
	cert, _, _, _ := makeEd25519CertAndKey(t, "ed25519-wrong.test")
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	err = ValidateKeyMatch(cert, rsaKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key type mismatch")
}

func TestValidateKeyMatch_UnsupportedKeyType(t *testing.T) {
	// Create a cert with a nil public key type to trigger the default branch
	cert := &x509.Certificate{PublicKey: "not-a-real-key"}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	err = ValidateKeyMatch(cert, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported public key type")
}

// --- ConvertDERToPEM ---

func TestConvertDERToPEM_Valid(t *testing.T) {
	cert, _, _, _ := makeRSACertAndKey(t, "der-to-pem.test", time.Now().Add(time.Hour))
	pemStr, err := ConvertDERToPEM(cert.Raw)
	require.NoError(t, err)
	assert.Contains(t, pemStr, "BEGIN CERTIFICATE")
}

func TestConvertDERToPEM_Invalid(t *testing.T) {
	_, err := ConvertDERToPEM([]byte("not-der-data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid DER")
}

// --- ConvertPEMToDER ---

func TestConvertPEMToDER_Valid(t *testing.T) {
	_, _, certPEM, _ := makeRSACertAndKey(t, "pem-to-der.test", time.Now().Add(time.Hour))
	derData, err := ConvertPEMToDER(string(certPEM))
	require.NoError(t, err)
	assert.NotEmpty(t, derData)

	// Verify it's valid DER
	parsed, err := x509.ParseCertificate(derData)
	require.NoError(t, err)
	assert.Equal(t, "pem-to-der.test", parsed.Subject.CommonName)
}

func TestConvertPEMToDER_NoPEMBlock(t *testing.T) {
	_, err := ConvertPEMToDER("not-pem-data")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM")
}

func TestConvertPEMToDER_InvalidCert(t *testing.T) {
	fakePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")}))
	_, err := ConvertPEMToDER(fakePEM)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid certificate PEM")
}

// --- ConvertPEMToPFX ---

func TestConvertPEMToPFX_Valid(t *testing.T) {
	_, _, certPEM, keyPEM := makeRSACertAndKey(t, "pem-to-pfx.test", time.Now().Add(time.Hour))
	pfxData, err := ConvertPEMToPFX(string(certPEM), string(keyPEM), "", "test-password")
	require.NoError(t, err)
	assert.NotEmpty(t, pfxData)
}

func TestConvertPEMToPFX_WithChain(t *testing.T) {
	_, _, certPEM, keyPEM := makeRSACertAndKey(t, "pfx-chain.test", time.Now().Add(time.Hour))
	_, _, chainPEM, _ := makeRSACertAndKey(t, "pfx-ca.test", time.Now().Add(time.Hour))
	pfxData, err := ConvertPEMToPFX(string(certPEM), string(keyPEM), string(chainPEM), "pass")
	require.NoError(t, err)
	assert.NotEmpty(t, pfxData)
}

func TestConvertPEMToPFX_BadCert(t *testing.T) {
	_, err := ConvertPEMToPFX("not-pem", "not-pem", "", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cert PEM")
}

func TestConvertPEMToPFX_BadKey(t *testing.T) {
	_, _, certPEM, _ := makeRSACertAndKey(t, "pfx-badkey.test", time.Now().Add(time.Hour))
	_, err := ConvertPEMToPFX(string(certPEM), "not-pem", "", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key PEM")
}

// --- ExtractCertificateMetadata ---

func TestExtractCertificateMetadata_Nil(t *testing.T) {
	result := ExtractCertificateMetadata(nil)
	assert.Nil(t, result)
}

func TestExtractCertificateMetadata_Valid(t *testing.T) {
	cert, _, _, _ := makeRSACertAndKey(t, "metadata.test", time.Now().Add(24*time.Hour))
	meta := ExtractCertificateMetadata(cert)
	require.NotNil(t, meta)
	assert.NotEmpty(t, meta.Fingerprint)
	assert.NotEmpty(t, meta.SerialNumber)
	assert.Contains(t, meta.KeyType, "RSA")
	assert.Contains(t, meta.Domains, "metadata.test")
}

func TestExtractCertificateMetadata_WithSANs(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "san.test", Organization: []string{"Test Org"}},
		Issuer:       pkix.Name{Organization: []string{"Test Issuer"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"san.test", "alt.test", "other.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	meta := ExtractCertificateMetadata(cert)
	require.NotNil(t, meta)
	assert.Contains(t, meta.Domains, "san.test")
	assert.Contains(t, meta.Domains, "alt.test")
	assert.Contains(t, meta.Domains, "other.test")
	assert.Equal(t, "Test Org", meta.IssuerOrg)
}

// --- detectKeyType ---

func TestDetectKeyType_Ed25519(t *testing.T) {
	cert, _, _, _ := makeEd25519CertAndKey(t, "ed25519-type.test")
	assert.Equal(t, "Ed25519", detectKeyType(cert))
}

func TestDetectKeyType_RSA(t *testing.T) {
	cert, _, _, _ := makeRSACertAndKey(t, "rsa-type.test", time.Now().Add(time.Hour))
	kt := detectKeyType(cert)
	assert.Contains(t, kt, "RSA-")
}

func TestDetectKeyType_ECDSA_P256(t *testing.T) {
	cert, _, _, _ := makeECDSACertAndKey(t, "p256-type.test")
	assert.Equal(t, "ECDSA-P256", detectKeyType(cert))
}

// --- formatSerial ---

func TestFormatSerial_Nil(t *testing.T) {
	assert.Equal(t, "", formatSerial(nil))
}

func TestFormatSerial_Value(t *testing.T) {
	result := formatSerial(big.NewInt(256))
	assert.NotEmpty(t, result)
	assert.Contains(t, result, ":")
}

// --- formatFingerprint ---

func TestFormatFingerprint_Normal(t *testing.T) {
	result := formatFingerprint("aabbccdd")
	assert.Equal(t, "AA:BB:CC:DD", result)
}

func TestFormatFingerprint_OddLength(t *testing.T) {
	result := formatFingerprint("aabbc")
	assert.Contains(t, result, "AA:BB")
}

// --- DetectFormat DER ---

func TestDetectFormat_DER(t *testing.T) {
	cert, _, _, _ := makeRSACertAndKey(t, "detect-der.test", time.Now().Add(time.Hour))
	format := DetectFormat(cert.Raw)
	assert.Equal(t, FormatDER, format)
}

func TestDetectFormat_PEM(t *testing.T) {
	_, _, certPEM, _ := makeRSACertAndKey(t, "detect-pem.test", time.Now().Add(time.Hour))
	format := DetectFormat(certPEM)
	assert.Equal(t, FormatPEM, format)
}


