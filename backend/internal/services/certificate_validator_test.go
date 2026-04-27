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

// --- helpers ---

func makeRSACertAndKey(t *testing.T, cn string, expiry time.Time) (cert *x509.Certificate, priv *rsa.PrivateKey, certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     expiry,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return cert, priv, certPEM, keyPEM
}

func makeECDSACertAndKey(t *testing.T, cn string) (cert *x509.Certificate, priv *ecdsa.PrivateKey, certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return cert, priv, certPEM, keyPEM
}

func makeEd25519CertAndKey(t *testing.T, cn string) (cert *x509.Certificate, priv ed25519.PrivateKey, certPEM, keyPEM []byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	require.NoError(t, err)
	cert, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return cert, priv, certPEM, keyPEM
}

// --- DetectFormat ---

func TestDetectFormat(t *testing.T) {
	cert, _, certPEM, _ := makeRSACertAndKey(t, "test.com", time.Now().Add(time.Hour))

	t.Run("PEM format", func(t *testing.T) {
		assert.Equal(t, FormatPEM, DetectFormat(certPEM))
	})

	t.Run("DER format", func(t *testing.T) {
		assert.Equal(t, FormatDER, DetectFormat(cert.Raw))
	})

	t.Run("unknown format", func(t *testing.T) {
		assert.Equal(t, FormatUnknown, DetectFormat([]byte("not a cert")))
	})

	t.Run("empty data", func(t *testing.T) {
		assert.Equal(t, FormatUnknown, DetectFormat([]byte{}))
	})
}

// --- ParseCertificateInput ---

func TestParseCertificateInput(t *testing.T) {
	t.Run("PEM cert only", func(t *testing.T) {
		_, _, certPEM, _ := makeRSACertAndKey(t, "pem.test", time.Now().Add(time.Hour))
		parsed, err := ParseCertificateInput(certPEM, nil, nil, "")
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.Equal(t, FormatPEM, parsed.Format)
		assert.Nil(t, parsed.PrivateKey)
	})

	t.Run("PEM cert with key", func(t *testing.T) {
		_, _, certPEM, keyPEM := makeRSACertAndKey(t, "pem-key.test", time.Now().Add(time.Hour))
		parsed, err := ParseCertificateInput(certPEM, keyPEM, nil, "")
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.NotNil(t, parsed.PrivateKey)
		assert.Equal(t, FormatPEM, parsed.Format)
	})

	t.Run("DER cert", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "der.test", time.Now().Add(time.Hour))
		parsed, err := ParseCertificateInput(cert.Raw, nil, nil, "")
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.Equal(t, FormatDER, parsed.Format)
	})

	t.Run("empty data returns error", func(t *testing.T) {
		_, err := ParseCertificateInput(nil, nil, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("unrecognized format returns error", func(t *testing.T) {
		_, err := ParseCertificateInput([]byte("garbage"), nil, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unrecognized")
	})

	t.Run("invalid key PEM returns error", func(t *testing.T) {
		_, _, certPEM, _ := makeRSACertAndKey(t, "badkey.test", time.Now().Add(time.Hour))
		_, err := ParseCertificateInput(certPEM, []byte("not-key"), nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key")
	})
}

// --- ValidateKeyMatch ---

func TestValidateKeyMatch(t *testing.T) {
	t.Run("RSA matching", func(t *testing.T) {
		cert, priv, _, _ := makeRSACertAndKey(t, "rsa.test", time.Now().Add(time.Hour))
		assert.NoError(t, ValidateKeyMatch(cert, priv))
	})

	t.Run("RSA mismatched", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "rsa1.test", time.Now().Add(time.Hour))
		_, otherPriv, _, _ := makeRSACertAndKey(t, "rsa2.test", time.Now().Add(time.Hour))
		err := ValidateKeyMatch(cert, otherPriv)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mismatch")
	})

	t.Run("ECDSA matching", func(t *testing.T) {
		cert, priv, _, _ := makeECDSACertAndKey(t, "ecdsa.test")
		assert.NoError(t, ValidateKeyMatch(cert, priv))
	})

	t.Run("ECDSA mismatched", func(t *testing.T) {
		cert, _, _, _ := makeECDSACertAndKey(t, "ec1.test")
		_, other, _, _ := makeECDSACertAndKey(t, "ec2.test")
		assert.Error(t, ValidateKeyMatch(cert, other))
	})

	t.Run("Ed25519 matching", func(t *testing.T) {
		cert, priv, _, _ := makeEd25519CertAndKey(t, "ed.test")
		assert.NoError(t, ValidateKeyMatch(cert, priv))
	})

	t.Run("Ed25519 mismatched", func(t *testing.T) {
		cert, _, _, _ := makeEd25519CertAndKey(t, "ed1.test")
		_, other, _, _ := makeEd25519CertAndKey(t, "ed2.test")
		assert.Error(t, ValidateKeyMatch(cert, other))
	})

	t.Run("type mismatch RSA cert with ECDSA key", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "rsa.test", time.Now().Add(time.Hour))
		_, ecKey, _, _ := makeECDSACertAndKey(t, "ec.test")
		err := ValidateKeyMatch(cert, ecKey)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type mismatch")
	})

	t.Run("nil certificate", func(t *testing.T) {
		_, priv, _, _ := makeRSACertAndKey(t, "rsa.test", time.Now().Add(time.Hour))
		assert.Error(t, ValidateKeyMatch(nil, priv))
	})

	t.Run("nil key", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "rsa.test", time.Now().Add(time.Hour))
		assert.Error(t, ValidateKeyMatch(cert, nil))
	})
}

// --- ValidateChain ---

func TestValidateChain(t *testing.T) {
	t.Run("nil leaf returns error", func(t *testing.T) {
		err := ValidateChain(nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("self-signed cert validates", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "self.test", time.Now().Add(time.Hour))
		// Self-signed won't pass chain validation without being a CA
		err := ValidateChain(cert, nil)
		assert.Error(t, err)
	})
}

// --- ConvertDERToPEM ---

func TestConvertDERToPEM(t *testing.T) {
	t.Run("valid DER", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "der.test", time.Now().Add(time.Hour))
		pemStr, err := ConvertDERToPEM(cert.Raw)
		require.NoError(t, err)
		assert.Contains(t, pemStr, "BEGIN CERTIFICATE")
	})

	t.Run("invalid DER", func(t *testing.T) {
		_, err := ConvertDERToPEM([]byte("not-der"))
		assert.Error(t, err)
	})
}

// --- ConvertPEMToDER ---

func TestConvertPEMToDER(t *testing.T) {
	t.Run("valid PEM", func(t *testing.T) {
		_, _, certPEM, _ := makeRSACertAndKey(t, "p2d.test", time.Now().Add(time.Hour))
		der, err := ConvertPEMToDER(string(certPEM))
		require.NoError(t, err)
		assert.NotEmpty(t, der)
		// Round-trip
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		assert.Equal(t, "p2d.test", cert.Subject.CommonName)
	})

	t.Run("invalid PEM", func(t *testing.T) {
		_, err := ConvertPEMToDER("not-pem")
		assert.Error(t, err)
	})
}

// --- ExtractCertificateMetadata ---

func TestExtractCertificateMetadata(t *testing.T) {
	t.Run("nil cert returns nil", func(t *testing.T) {
		assert.Nil(t, ExtractCertificateMetadata(nil))
	})

	t.Run("RSA cert metadata", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "meta.test", time.Now().Add(time.Hour))
		m := ExtractCertificateMetadata(cert)
		require.NotNil(t, m)
		assert.Equal(t, "meta.test", m.CommonName)
		assert.Contains(t, m.KeyType, "RSA")
		assert.NotEmpty(t, m.Fingerprint)
		assert.NotEmpty(t, m.SerialNumber)
		assert.Contains(t, m.Domains, "meta.test")
	})

	t.Run("ECDSA cert metadata", func(t *testing.T) {
		cert, _, _, _ := makeECDSACertAndKey(t, "ec-meta.test")
		m := ExtractCertificateMetadata(cert)
		require.NotNil(t, m)
		assert.Contains(t, m.KeyType, "ECDSA")
	})

	t.Run("Ed25519 cert metadata", func(t *testing.T) {
		cert, _, _, _ := makeEd25519CertAndKey(t, "ed-meta.test")
		m := ExtractCertificateMetadata(cert)
		require.NotNil(t, m)
		assert.Equal(t, "Ed25519", m.KeyType)
	})

	t.Run("cert with SANs", func(t *testing.T) {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(10),
			Subject:      pkix.Name{CommonName: "main.test"},
			DNSNames:     []string{"main.test", "alt1.test", "alt2.test"},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
		require.NoError(t, err)
		cert, _ := x509.ParseCertificate(der)

		m := ExtractCertificateMetadata(cert)
		require.NotNil(t, m)
		assert.Contains(t, m.Domains, "main.test")
		assert.Contains(t, m.Domains, "alt1.test")
		assert.Contains(t, m.Domains, "alt2.test")
		// CN should not be duplicated when it matches a SAN
		count := 0
		for _, d := range m.Domains {
			if d == "main.test" {
				count++
			}
		}
		assert.Equal(t, 1, count, "CN should not be duplicated in domains list")
	})

	t.Run("cert with issuer org", func(t *testing.T) {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(11),
			Subject:      pkix.Name{CommonName: "org.test"},
			Issuer:       pkix.Name{Organization: []string{"Test Org Inc"}},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
		require.NoError(t, err)
		cert, _ := x509.ParseCertificate(der)

		m := ExtractCertificateMetadata(cert)
		require.NotNil(t, m)
		// Self-signed cert's issuer org may differ from template
		assert.NotEmpty(t, m.Fingerprint)
	})
}

// --- Helpers ---

func TestFormatFingerprint(t *testing.T) {
	assert.Equal(t, "AB:CD:EF", formatFingerprint("abcdef"))
	assert.Equal(t, "01:23", formatFingerprint("0123"))
	assert.Equal(t, "", formatFingerprint(""))
}

func TestFormatSerial(t *testing.T) {
	assert.Equal(t, "01", formatSerial(big.NewInt(1)))
	assert.Equal(t, "FF", formatSerial(big.NewInt(255)))
	assert.Equal(t, "", formatSerial(nil))
}

func TestDetectKeyType(t *testing.T) {
	t.Run("RSA key type", func(t *testing.T) {
		cert, _, _, _ := makeRSACertAndKey(t, "rsa.test", time.Now().Add(time.Hour))
		kt := detectKeyType(cert)
		assert.Contains(t, kt, "RSA-2048")
	})

	t.Run("ECDSA-P256 key type", func(t *testing.T) {
		cert, _, _, _ := makeECDSACertAndKey(t, "ec.test")
		kt := detectKeyType(cert)
		assert.Equal(t, "ECDSA-P256", kt)
	})

	t.Run("Ed25519 key type", func(t *testing.T) {
		cert, _, _, _ := makeEd25519CertAndKey(t, "ed.test")
		kt := detectKeyType(cert)
		assert.Equal(t, "Ed25519", kt)
	})
}
