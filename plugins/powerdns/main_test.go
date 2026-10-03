package main

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

func TestProviderIdentity(t *testing.T) {
	p := &PowerDNSProvider{}

	if got := p.Type(); got != "powerdns" {
		t.Fatalf("Type() = %q, want powerdns", got)
	}

	md := p.Metadata()
	if md.Type != "powerdns" || md.Name != "PowerDNS" || md.IsBuiltIn {
		t.Fatalf("unexpected metadata: %+v", md)
	}
	if md.InterfaceVersion != dnsprovider.InterfaceVersion {
		t.Fatalf("InterfaceVersion = %q, want %q", md.InterfaceVersion, dnsprovider.InterfaceVersion)
	}

	if Plugin == nil || Plugin.Type() != "powerdns" {
		t.Fatalf("exported Plugin symbol is not the PowerDNS provider")
	}
}

func TestCredentialFieldSpecs(t *testing.T) {
	p := &PowerDNSProvider{}

	required := map[string]bool{}
	for _, f := range p.RequiredCredentialFields() {
		required[f.Name] = true
	}
	if !required["api_url"] || !required["api_key"] || len(required) != 2 {
		t.Fatalf("unexpected required fields: %v", required)
	}

	optional := p.OptionalCredentialFields()
	if len(optional) != 1 || optional[0].Name != "server_id" {
		t.Fatalf("unexpected optional fields: %+v", optional)
	}
}

func TestValidateCredentialsRequiredFields(t *testing.T) {
	p := &PowerDNSProvider{}

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
	}{
		{"missing api_url", map[string]string{"api_key": "k"}, true},
		{"missing api_key", map[string]string{"api_url": "https://pdns.example.com:8081"}, true},
		{"both present", map[string]string{"api_url": "https://pdns.example.com:8081", "api_key": "k"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.ValidateCredentials(tt.creds); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildCaddyConfigDefaultsServerID(t *testing.T) {
	p := &PowerDNSProvider{}
	creds := map[string]string{"api_url": "https://pdns.example.com:8081", "api_key": "k"}

	cfg := p.BuildCaddyConfig(creds)
	if cfg["name"] != "powerdns" || cfg["server_id"] != "localhost" || cfg["api_url"] != creds["api_url"] {
		t.Fatalf("unexpected config: %v", cfg)
	}

	creds["server_id"] = "pdns1"
	if got := p.BuildCaddyConfigForZone("example.com", creds)["server_id"]; got != "pdns1" {
		t.Fatalf("server_id = %v, want pdns1", got)
	}
}

func TestPropagationSettings(t *testing.T) {
	p := &PowerDNSProvider{}
	if p.PropagationTimeout() != 60*time.Second || p.PollingInterval() != 2*time.Second {
		t.Fatalf("unexpected propagation settings")
	}
	if p.SupportsMultiCredential() {
		t.Fatalf("SupportsMultiCredential() = true, want false")
	}
}
