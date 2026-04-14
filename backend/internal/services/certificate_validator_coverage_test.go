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

// --- parsePFXInput ---

func TestParsePFXInput(t *testing.T) {
	cert, priv, _, _ := makeRSACertAndKey(t, "pfx.test", time.Now().Add(time.Hour))
	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, pkcs12.DefaultPassword)
	require.NoError(t, err)

	t.Run("valid PFX", func(t *testing.T) {
		parsed, err := parsePFXInput(pfxData, pkcs12.DefaultPassword)
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.NotNil(t, parsed.PrivateKey)
		assert.Equal(t, FormatPFX, parsed.Format)
		assert.Contains(t, parsed.CertPEM, "BEGIN CERTIFICATE")
		assert.Contains(t, parsed.KeyPEM, "PRIVATE KEY")
	})

	t.Run("PFX with chain", func(t *testing.T) {
		caKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		caTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(100),
			Subject:               pkix.Name{CommonName: "Test CA"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}
		caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
		require.NoError(t, err)
		caCert, err := x509.ParseCertificate(caDER)
		require.NoError(t, err)

		pfxWithChain, err := pkcs12.Modern.Encode(priv, cert, []*x509.Certificate{caCert}, pkcs12.DefaultPassword)
		require.NoError(t, err)

		parsed, err := parsePFXInput(pfxWithChain, pkcs12.DefaultPassword)
		require.NoError(t, err)
		assert.NotEmpty(t, parsed.ChainPEM)
		assert.Contains(t, parsed.ChainPEM, "BEGIN CERTIFICATE")
	})

	t.Run("invalid PFX data", func(t *testing.T) {
		_, err := parsePFXInput([]byte("not-pfx"), "password")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "PFX")
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := parsePFXInput(pfxData, "wrong-password")
		assert.Error(t, err)
	})
}

// --- parseDERInput ---

func TestParseDERInput(t *testing.T) {
	cert, priv, _, keyPEM := makeRSACertAndKey(t, "der.test", time.Now().Add(time.Hour))

	t.Run("DER cert only", func(t *testing.T) {
		parsed, err := parseDERInput(cert.Raw, nil)
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.Equal(t, FormatDER, parsed.Format)
		assert.Contains(t, parsed.CertPEM, "BEGIN CERTIFICATE")
		assert.Nil(t, parsed.PrivateKey)
	})

	t.Run("DER cert with PEM key", func(t *testing.T) {
		parsed, err := parseDERInput(cert.Raw, keyPEM)
		require.NoError(t, err)
		assert.NotNil(t, parsed.PrivateKey)
		assert.Contains(t, parsed.KeyPEM, "PRIVATE KEY")
	})

	t.Run("DER cert with DER PKCS8 key", func(t *testing.T) {
		derKey, err := x509.MarshalPKCS8PrivateKey(priv)
		require.NoError(t, err)
		parsed, err := parseDERInput(cert.Raw, derKey)
		require.NoError(t, err)
		assert.NotNil(t, parsed.PrivateKey)
	})

	t.Run("DER cert with DER EC key", func(t *testing.T) {
		ecCert, ecPriv, _, _ := makeECDSACertAndKey(t, "ec-der.test")
		ecDERKey, err := x509.MarshalECPrivateKey(ecPriv)
		require.NoError(t, err)
		parsed, err := parseDERInput(ecCert.Raw, ecDERKey)
		require.NoError(t, err)
		assert.NotNil(t, parsed.PrivateKey)
	})

	t.Run("DER cert with invalid key", func(t *testing.T) {
		_, err := parseDERInput(cert.Raw, []byte("bad-key-data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "private key")
	})

	t.Run("invalid DER cert data", func(t *testing.T) {
		_, err := parseDERInput([]byte("not-der"), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "DER certificate")
	})
}

// --- parsePEMInput chain building ---

func TestParsePEMInput_ChainBuilding(t *testing.T) {
	t.Run("cert with intermediates in cert data", func(t *testing.T) {
		_, _, certPEM1, _ := makeRSACertAndKey(t, "leaf.test", time.Now().Add(time.Hour))
		_, _, certPEM2, _ := makeRSACertAndKey(t, "intermediate.test", time.Now().Add(time.Hour))
		combined := append(certPEM1, certPEM2...)

		parsed, err := parsePEMInput(combined, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.Len(t, parsed.Intermediates, 1)
		assert.NotEmpty(t, parsed.ChainPEM)
		assert.Contains(t, parsed.ChainPEM, "BEGIN CERTIFICATE")
	})

	t.Run("cert with chain file", func(t *testing.T) {
		_, _, certPEM, keyPEM := makeRSACertAndKey(t, "leaf.test", time.Now().Add(time.Hour))
		_, _, chainPEM, _ := makeRSACertAndKey(t, "chain.test", time.Now().Add(time.Hour))

		parsed, err := parsePEMInput(certPEM, keyPEM, chainPEM)
		require.NoError(t, err)
		assert.NotNil(t, parsed.PrivateKey)
		assert.Len(t, parsed.Intermediates, 1)
		assert.Equal(t, string(chainPEM), parsed.ChainPEM)
	})

	t.Run("invalid chain data ignored", func(t *testing.T) {
		_, _, certPEM, _ := makeRSACertAndKey(t, "leaf.test", time.Now().Add(time.Hour))
		parsed, err := parsePEMInput(certPEM, nil, []byte("not-pem"))
		require.NoError(t, err)
		assert.Empty(t, parsed.Intermediates, "invalid PEM chain should be silently ignored")
	})

	t.Run("invalid cert data", func(t *testing.T) {
		_, err := parsePEMInput([]byte("not-pem"), nil, nil)
		assert.Error(t, err)
	})

	t.Run("empty PEM block", func(t *testing.T) {
		emptyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage")})
		_, err := parsePEMInput(emptyPEM, nil, nil)
		assert.Error(t, err)
	})
}

// --- ConvertPFXToPEM ---

func TestConvertPFXToPEM(t *testing.T) {
	cert, priv, _, _ := makeRSACertAndKey(t, "pfx-convert.test", time.Now().Add(time.Hour))
	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, pkcs12.DefaultPassword)
	require.NoError(t, err)

	t.Run("valid PFX", func(t *testing.T) {
		certPEM, keyPEM, chainPEM, err := ConvertPFXToPEM(pfxData, pkcs12.DefaultPassword)
		require.NoError(t, err)
		assert.Contains(t, certPEM, "BEGIN CERTIFICATE")
		assert.Contains(t, keyPEM, "PRIVATE KEY")
		assert.Empty(t, chainPEM)
	})

	t.Run("PFX with chain", func(t *testing.T) {
		caKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		caTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(200),
			Subject:               pkix.Name{CommonName: "PFX Test CA"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}
		caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
		require.NoError(t, err)
		caCert, err := x509.ParseCertificate(caDER)
		require.NoError(t, err)

		pfxWithChain, err := pkcs12.Modern.Encode(priv, cert, []*x509.Certificate{caCert}, pkcs12.DefaultPassword)
		require.NoError(t, err)

		certPEM, keyPEM, chainPEM, err := ConvertPFXToPEM(pfxWithChain, pkcs12.DefaultPassword)
		require.NoError(t, err)
		assert.Contains(t, certPEM, "BEGIN CERTIFICATE")
		assert.Contains(t, keyPEM, "PRIVATE KEY")
		assert.Contains(t, chainPEM, "BEGIN CERTIFICATE")
	})

	t.Run("invalid PFX", func(t *testing.T) {
		_, _, _, err := ConvertPFXToPEM([]byte("bad"), "password")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "PFX")
	})
}

// --- encodeKeyToPEM ---

func TestEncodeKeyToPEM(t *testing.T) {
	t.Run("RSA key", func(t *testing.T) {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		pemStr, err := encodeKeyToPEM(priv)
		require.NoError(t, err)
		assert.Contains(t, pemStr, "PRIVATE KEY")
	})

	t.Run("ECDSA key", func(t *testing.T) {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		pemStr, err := encodeKeyToPEM(priv)
		require.NoError(t, err)
		assert.Contains(t, pemStr, "PRIVATE KEY")
	})
}

// --- ParseCertificateInput for PFX ---

func TestParseCertificateInput_PFX(t *testing.T) {
	cert, priv, _, _ := makeRSACertAndKey(t, "pfx-parse.test", time.Now().Add(time.Hour))
	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, pkcs12.DefaultPassword)
	require.NoError(t, err)

	t.Run("PFX format detected and parsed", func(t *testing.T) {
		parsed, err := ParseCertificateInput(pfxData, nil, nil, pkcs12.DefaultPassword)
		require.NoError(t, err)
		assert.NotNil(t, parsed.Leaf)
		assert.NotNil(t, parsed.PrivateKey)
		assert.Equal(t, FormatPFX, parsed.Format)
	})
}

// --- detectKeyType additional branches ---

func TestDetectKeyType_P384(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: "p384.test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	assert.Equal(t, "ECDSA-P384", detectKeyType(cert))
}

// --- parsePEMPrivateKey additional formats ---

func TestParsePEMPrivateKey_PKCS1(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	key, err := parsePEMPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, key)
}

func TestParsePEMPrivateKey_EC(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER})

	key, err := parsePEMPrivateKey(keyPEM)
	require.NoError(t, err)
	assert.NotNil(t, key)
}

func TestParsePEMPrivateKey_Invalid(t *testing.T) {
	t.Run("no PEM data", func(t *testing.T) {
		_, err := parsePEMPrivateKey([]byte("not pem"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no PEM data")
	})

	t.Run("unsupported key format", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN KEY", Bytes: []byte("junk")})
		_, err := parsePEMPrivateKey(badPEM)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})
}

// --- DetectFormat for PFX ---

func TestDetectFormat_PFX(t *testing.T) {
	cert, priv, _, _ := makeRSACertAndKey(t, "detect-pfx.test", time.Now().Add(time.Hour))
	pfxData, err := pkcs12.Modern.Encode(priv, cert, nil, pkcs12.DefaultPassword)
	require.NoError(t, err)

	format := DetectFormat(pfxData)
	assert.Equal(t, FormatPFX, format, "PFX data should be detected as FormatPFX")
}
