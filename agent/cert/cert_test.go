package cert_test

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/Wikid82/charon/agent/cert"
)

// TestLoadOrGenerate exercises LoadOrGenerate's genuine logic (ECDSA key
// generation, self-signed cert templating, PEM encoding) — previously
// untested (0% coverage), unlike agent/main.go's CLI bootstrap code, which
// is excluded from the coverage gate rather than tested directly (see
// scripts/agent-test-coverage.sh's EXCLUDE_PACKAGES).
func TestLoadOrGenerate(t *testing.T) {
	const agentID = "test-agent-12345"

	cfg, err := cert.LoadOrGenerate(agentID)
	if err != nil {
		t.Fatalf("LoadOrGenerate returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadOrGenerate returned nil config")
	}

	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected exactly one certificate, got %d", len(cfg.Certificates))
	}

	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("expected MinVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, cfg.MinVersion)
	}

	tlsCert := cfg.Certificates[0]
	if len(tlsCert.Certificate) == 0 {
		t.Fatal("expected at least one DER-encoded certificate in the chain")
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse leaf certificate DER bytes: %v", err)
	}

	if !strings.Contains(leaf.Subject.CommonName, agentID) {
		t.Fatalf("expected CommonName to contain agentID %q, got %q", agentID, leaf.Subject.CommonName)
	}
}

// TestLoadOrGenerate_UniquePerCall confirms each call produces a fresh
// certificate (in-memory generation, no reuse/caching), matching the
// documented "generated in memory on every call" behavior.
func TestLoadOrGenerate_UniquePerCall(t *testing.T) {
	cfgOne, err := cert.LoadOrGenerate("agent-a")
	if err != nil {
		t.Fatalf("LoadOrGenerate returned error: %v", err)
	}
	cfgTwo, err := cert.LoadOrGenerate("agent-a")
	if err != nil {
		t.Fatalf("LoadOrGenerate returned error: %v", err)
	}

	leafOne, err := x509.ParseCertificate(cfgOne.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse first leaf certificate: %v", err)
	}
	leafTwo, err := x509.ParseCertificate(cfgTwo.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse second leaf certificate: %v", err)
	}

	if leafOne.SerialNumber.Cmp(leafTwo.SerialNumber) == 0 {
		t.Fatal("expected distinct serial numbers across independent LoadOrGenerate calls")
	}
}

// TestLoadOrGenerate_DifferentAgentIDsProduceDifferentCommonNames confirms
// the agentID parameter is actually threaded through into the certificate,
// not a hardcoded/ignored value.
func TestLoadOrGenerate_DifferentAgentIDsProduceDifferentCommonNames(t *testing.T) {
	cfgOne, err := cert.LoadOrGenerate("agent-one")
	if err != nil {
		t.Fatalf("LoadOrGenerate returned error: %v", err)
	}
	cfgTwo, err := cert.LoadOrGenerate("agent-two")
	if err != nil {
		t.Fatalf("LoadOrGenerate returned error: %v", err)
	}

	leafOne, err := x509.ParseCertificate(cfgOne.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse first leaf certificate: %v", err)
	}
	leafTwo, err := x509.ParseCertificate(cfgTwo.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse second leaf certificate: %v", err)
	}

	if leafOne.Subject.CommonName == leafTwo.Subject.CommonName {
		t.Fatalf("expected distinct CommonNames for distinct agentIDs, both were %q", leafOne.Subject.CommonName)
	}
}
