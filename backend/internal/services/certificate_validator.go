package services

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// CertFormat represents a certificate file format.
type CertFormat string

const (
	FormatPEM     CertFormat = "pem"
	FormatDER     CertFormat = "der"
	FormatPFX     CertFormat = "pfx"
	FormatUnknown CertFormat = "unknown"
)

// ParsedCertificate contains the parsed result of certificate input.
type ParsedCertificate struct {
	Leaf          *x509.Certificate
	Intermediates []*x509.Certificate
	PrivateKey    crypto.PrivateKey
	CertPEM       string
	KeyPEM        string
	ChainPEM      string
	Format        CertFormat
}

// CertificateMetadata contains extracted metadata from an x509 certificate.
type CertificateMetadata struct {
	CommonName   string
	Domains      []string
	Fingerprint  string
	SerialNumber string
	IssuerOrg    string
	KeyType      string
	NotBefore    time.Time
	NotAfter     time.Time
}

// ValidationResult contains the result of a certificate validation.
type ValidationResult struct {
	Valid      bool      `json:"valid"`
	CommonName string    `json:"common_name"`
	Domains    []string  `json:"domains"`
	IssuerOrg  string    `json:"issuer_org"`
	ExpiresAt  time.Time `json:"expires_at"`
	KeyMatch   bool      `json:"key_match"`
	ChainValid bool      `json:"chain_valid"`
	ChainDepth int       `json:"chain_depth"`
	Warnings   []string  `json:"warnings"`
	Errors     []string  `json:"errors"`
}

// DetectFormat determines the certificate format from raw file content.
// Uses trial-parse strategy: PEM → PFX → DER.
func DetectFormat(data []byte) CertFormat {
	block, _ := pem.Decode(data)
	if block != nil {
		return FormatPEM
	}

	if _, _, _, err := pkcs12.DecodeChain(data, ""); err == nil {
		return FormatPFX
	}
	// PFX with empty password failed, but it could be password-protected
	// If data starts with PKCS12 magic bytes (ASN.1 SEQUENCE), treat as PFX candidate
	if len(data) > 2 && data[0] == 0x30 {
		// Could be DER or PFX; try DER parse
		if _, err := x509.ParseCertificate(data); err == nil {
			return FormatDER
		}
		// If DER parse fails, it's likely PFX
		return FormatPFX
	}

	if _, err := x509.ParseCertificate(data); err == nil {
		return FormatDER
	}

	return FormatUnknown
}

// ParseCertificateInput handles PEM, PFX, and DER input parsing.
func ParseCertificateInput(certData []byte, keyData []byte, chainData []byte, pfxPassword string) (*ParsedCertificate, error) {
	if len(certData) == 0 {
		return nil, fmt.Errorf("certificate data is empty")
	}

	format := DetectFormat(certData)

	switch format {
	case FormatPEM:
		return parsePEMInput(certData, keyData, chainData)
	case FormatPFX:
		return parsePFXInput(certData, pfxPassword)
	case FormatDER:
		return parseDERInput(certData, keyData)
	default:
		return nil, fmt.Errorf("unrecognized certificate format")
	}
}

func parsePEMInput(certData []byte, keyData []byte, chainData []byte) (*ParsedCertificate, error) {
	result := &ParsedCertificate{Format: FormatPEM}

	// Parse leaf certificate
	certs, err := parsePEMCertificates(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate PEM: %w", err)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found in PEM data")
	}

	result.Leaf = certs[0]
	result.CertPEM = string(certData)

	// If certData contains multiple certs, treat extras as intermediates
	if len(certs) > 1 {
		result.Intermediates = certs[1:]
	}

	// Parse chain file if provided
	if len(chainData) > 0 {
		chainCerts, err := parsePEMCertificates(chainData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse chain PEM: %w", err)
		}
		result.Intermediates = append(result.Intermediates, chainCerts...)
		result.ChainPEM = string(chainData)
	}

	// Build chain PEM from intermediates if not set from chain file
	if result.ChainPEM == "" && len(result.Intermediates) > 0 {
		var chainBuilder strings.Builder
		for _, ic := range result.Intermediates {
			if err := pem.Encode(&chainBuilder, &pem.Block{Type: "CERTIFICATE", Bytes: ic.Raw}); err != nil {
				return nil, fmt.Errorf("failed to encode intermediate certificate: %w", err)
			}
		}
		result.ChainPEM = chainBuilder.String()
	}

	// Parse private key
	if len(keyData) > 0 {
		key, err := parsePEMPrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key PEM: %w", err)
		}
		result.PrivateKey = key
		result.KeyPEM = string(keyData)
	}

	return result, nil
}

func parsePFXInput(pfxData []byte, password string) (*ParsedCertificate, error) {
	privateKey, leaf, caCerts, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PFX/PKCS12: %w", err)
	}

	result := &ParsedCertificate{
		Format:        FormatPFX,
		Leaf:          leaf,
		Intermediates: caCerts,
		PrivateKey:    privateKey,
	}

	// Convert to PEM for storage
	result.CertPEM = encodeCertToPEM(leaf)

	if len(caCerts) > 0 {
		var chainBuilder strings.Builder
		for _, ca := range caCerts {
			chainBuilder.WriteString(encodeCertToPEM(ca))
		}
		result.ChainPEM = chainBuilder.String()
	}

	keyPEM, err := encodeKeyToPEM(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key to PEM: %w", err)
	}
	result.KeyPEM = keyPEM

	return result, nil
}

func parseDERInput(certData []byte, keyData []byte) (*ParsedCertificate, error) {
	cert, err := x509.ParseCertificate(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DER certificate: %w", err)
	}

	result := &ParsedCertificate{
		Format:  FormatDER,
		Leaf:    cert,
		CertPEM: encodeCertToPEM(cert),
	}

	if len(keyData) > 0 {
		key, err := parsePEMPrivateKey(keyData)
		if err != nil {
			// Try DER key
			key, err = x509.ParsePKCS8PrivateKey(keyData)
			if err != nil {
				key2, err2 := x509.ParseECPrivateKey(keyData)
				if err2 != nil {
					return nil, fmt.Errorf("failed to parse private key: %w", err)
				}
				key = key2
			}
		}
		result.PrivateKey = key
		keyPEM, err := encodeKeyToPEM(key)
		if err != nil {
			return nil, fmt.Errorf("failed to encode private key to PEM: %w", err)
		}
		result.KeyPEM = keyPEM
	}

	return result, nil
}

// ValidateKeyMatch checks that the private key matches the certificate public key.
func ValidateKeyMatch(cert *x509.Certificate, key crypto.PrivateKey) error {
	if cert == nil {
		return fmt.Errorf("certificate is nil")
	}
	if key == nil {
		return fmt.Errorf("private key is nil")
	}

	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		privKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("key type mismatch: certificate has RSA public key but private key is not RSA")
		}
		if pub.N.Cmp(privKey.N) != 0 {
			return fmt.Errorf("RSA key mismatch: certificate and private key modulus differ")
		}
	case *ecdsa.PublicKey:
		privKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return fmt.Errorf("key type mismatch: certificate has ECDSA public key but private key is not ECDSA")
		}
		if pub.X.Cmp(privKey.X) != 0 || pub.Y.Cmp(privKey.Y) != 0 {
			return fmt.Errorf("ECDSA key mismatch: certificate and private key points differ")
		}
	case ed25519.PublicKey:
		privKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return fmt.Errorf("key type mismatch: certificate has Ed25519 public key but private key is not Ed25519")
		}
		pubFromPriv := privKey.Public().(ed25519.PublicKey)
		if !pub.Equal(pubFromPriv) {
			return fmt.Errorf("Ed25519 key mismatch: certificate and private key differ")
		}
	default:
		return fmt.Errorf("unsupported public key type: %T", cert.PublicKey)
	}

	return nil
}

// ValidateChain verifies the certificate chain from leaf to root.
func ValidateChain(leaf *x509.Certificate, intermediates []*x509.Certificate) error {
	if leaf == nil {
		return fmt.Errorf("leaf certificate is nil")
	}

	pool := x509.NewCertPool()
	for _, ic := range intermediates {
		pool.AddCert(ic)
	}

	opts := x509.VerifyOptions{
		Intermediates: pool,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := leaf.Verify(opts); err != nil {
		return fmt.Errorf("chain verification failed: %w", err)
	}

	return nil
}

// ConvertDERToPEM converts DER-encoded certificate to PEM.
func ConvertDERToPEM(derData []byte) (string, error) {
	cert, err := x509.ParseCertificate(derData)
	if err != nil {
		return "", fmt.Errorf("invalid DER data: %w", err)
	}
	return encodeCertToPEM(cert), nil
}

// ConvertPFXToPEM extracts cert, key, and chain from PFX/PKCS12.
func ConvertPFXToPEM(pfxData []byte, password string) (certPEM string, keyPEM string, chainPEM string, err error) {
	privateKey, leaf, caCerts, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decode PFX: %w", err)
	}

	certPEM = encodeCertToPEM(leaf)

	keyPEM, err = encodeKeyToPEM(privateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to encode key: %w", err)
	}

	if len(caCerts) > 0 {
		var builder strings.Builder
		for _, ca := range caCerts {
			builder.WriteString(encodeCertToPEM(ca))
		}
		chainPEM = builder.String()
	}

	return certPEM, keyPEM, chainPEM, nil
}

// ConvertPEMToPFX bundles cert, key, chain into PFX.
func ConvertPEMToPFX(certPEM string, keyPEM string, chainPEM string, password string) ([]byte, error) {
	certs, err := parsePEMCertificates([]byte(certPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse cert PEM: %w", err)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found in cert PEM")
	}

	key, err := parsePEMPrivateKey([]byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse key PEM: %w", err)
	}

	var caCerts []*x509.Certificate
	if chainPEM != "" {
		caCerts, err = parsePEMCertificates([]byte(chainPEM))
		if err != nil {
			return nil, fmt.Errorf("failed to parse chain PEM: %w", err)
		}
	}

	pfxData, err := pkcs12.Modern.Encode(key, certs[0], caCerts, password)
	if err != nil {
		return nil, fmt.Errorf("failed to encode PFX: %w", err)
	}

	return pfxData, nil
}

// ConvertPEMToDER converts PEM certificate to DER.
func ConvertPEMToDER(certPEM string) ([]byte, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}
	// Verify it's a valid certificate
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("invalid certificate PEM: %w", err)
	}
	return block.Bytes, nil
}

// ExtractCertificateMetadata extracts fingerprint, serial, issuer, key type, etc.
func ExtractCertificateMetadata(cert *x509.Certificate) *CertificateMetadata {
	if cert == nil {
		return nil
	}

	fingerprint := sha256.Sum256(cert.Raw)
	fpHex := formatFingerprint(hex.EncodeToString(fingerprint[:]))

	serial := formatSerial(cert.SerialNumber)

	issuerOrg := ""
	if len(cert.Issuer.Organization) > 0 {
		issuerOrg = cert.Issuer.Organization[0]
	}

	domains := make([]string, 0, len(cert.DNSNames)+1)
	if cert.Subject.CommonName != "" {
		domains = append(domains, cert.Subject.CommonName)
	}
	for _, san := range cert.DNSNames {
		if san != cert.Subject.CommonName {
			domains = append(domains, san)
		}
	}

	return &CertificateMetadata{
		CommonName:   cert.Subject.CommonName,
		Domains:      domains,
		Fingerprint:  fpHex,
		SerialNumber: serial,
		IssuerOrg:    issuerOrg,
		KeyType:      detectKeyType(cert),
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
	}
}

// --- helpers ---

func parsePEMCertificates(data []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func parsePEMPrivateKey(data []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM data found")
	}

	// Try PKCS8 first (handles RSA, ECDSA, Ed25519)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try PKCS1 RSA
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try EC
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, fmt.Errorf("unsupported private key format")
}

func encodeCertToPEM(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

func encodeKeyToPEM(key crypto.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

func formatFingerprint(hex string) string {
	var parts []string
	for i := 0; i < len(hex); i += 2 {
		end := i + 2
		if end > len(hex) {
			end = len(hex)
		}
		parts = append(parts, strings.ToUpper(hex[i:end]))
	}
	return strings.Join(parts, ":")
}

func formatSerial(n *big.Int) string {
	if n == nil {
		return ""
	}
	b := n.Bytes()
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return strings.Join(parts, ":")
}

func detectKeyType(cert *x509.Certificate) string {
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		bits := pub.N.BitLen()
		return fmt.Sprintf("RSA-%d", bits)
	case *ecdsa.PublicKey:
		switch pub.Curve {
		case elliptic.P256():
			return "ECDSA-P256"
		case elliptic.P384():
			return "ECDSA-P384"
		default:
			return "ECDSA"
		}
	case ed25519.PublicKey:
		return "Ed25519"
	default:
		return "Unknown"
	}
}
