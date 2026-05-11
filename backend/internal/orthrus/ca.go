package orthrus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// InternalCA is Charon's lightweight certificate authority used to issue
// mTLS certificates to registered Orthrus agents.
type InternalCA struct {
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
}

// NewInternalCA loads an existing CA from disk or generates a new one.
// Key and certificate are stored under {dataRoot}/keys/.
func NewInternalCA(dataRoot string) (*InternalCA, error) {
	keyPath := filepath.Join(dataRoot, "keys", "hecate-ca.key")
	certPath := filepath.Join(dataRoot, "keys", "hecate-ca.crt")

	_, keyErr := os.Stat(keyPath)
	_, certErr := os.Stat(certPath)

	if keyErr == nil && certErr == nil {
		return loadCA(keyPath, certPath)
	}
	return generateCA(keyPath, certPath)
}

// SignCSR parses the PEM-encoded CSR, signs it with the CA key, and returns
// the PEM-encoded certificate valid for the given duration.
func (ca *InternalCA) SignCSR(csrPEM []byte, validity time.Duration) ([]byte, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("orthrus: invalid CSR PEM block")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("orthrus: parse CSR: %w", err)
	}

	if err = csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("orthrus: CSR signature invalid: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("orthrus: generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-1 * time.Minute),
		NotAfter:     time.Now().Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, csr.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("orthrus: sign certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), nil
}

// CACertPEM returns the PEM-encoded CA certificate.
func (ca *InternalCA) CACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.caCert.Raw})
}

func loadCA(keyPath, certPath string) (*InternalCA, error) {
	keyPEM, err := os.ReadFile(keyPath) //nolint:gosec // G304: path is derived from application config
	if err != nil {
		return nil, fmt.Errorf("orthrus: read CA key: %w", err)
	}

	certPEM, err := os.ReadFile(certPath) //nolint:gosec // G304: path is derived from application config
	if err != nil {
		return nil, fmt.Errorf("orthrus: read CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("orthrus: decode CA key PEM")
	}

	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("orthrus: parse CA key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("orthrus: decode CA cert PEM")
	}

	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("orthrus: parse CA cert: %w", err)
	}

	return &InternalCA{caCert: caCert, caKey: caKey}, nil
}

func generateCA(keyPath, certPath string) (*InternalCA, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("orthrus: generate CA key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("orthrus: generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Charon Internal CA",
			Organization: []string{"Charon"},
		},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("orthrus: create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("orthrus: parse generated CA cert: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil { //nolint:govet // shadow: if-init scoping is intentional
		return nil, fmt.Errorf("orthrus: create keys dir: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, fmt.Errorf("orthrus: marshal CA key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("orthrus: write CA key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil { //nolint:gosec // G306: CA cert files are intentionally world-readable (standard practice)
		return nil, fmt.Errorf("orthrus: write CA cert: %w", err)
	}

	return &InternalCA{caCert: caCert, caKey: caKey}, nil
}
