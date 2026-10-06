package safehttp

import (
	"errors"
	"testing"
	"time"
)

// TestClientPolicyOutcomes pins which literal destinations each policy lets the
// client dial. A refused destination fails with ErrBlockedAddress; an allowed
// one fails later (nothing listens there), never with the sentinel.
func TestClientPolicyOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		policy      Policy
		host        string
		wantBlocked bool
	}{
		{"shared space under strict policy", PublicHTTPSOnly(), "100.64.0.1", true},
		{"shared space under private policy", PrivateNetworkOK(), "100.64.0.1", false},
		{"metadata alias under private policy", PrivateNetworkOK(), "100.100.100.200", true},
		{"benchmark range under strict policy", PublicHTTPSOnly(), "198.18.0.1", true},
		{"benchmark range under private policy", PrivateNetworkOK(), "198.18.0.1", true},
		{"tunnel range under private policy", PrivateNetworkOK(), "[2002::1]", true},
		{"private range under strict policy", PublicHTTPSOnly(), "10.0.0.1", true},
		{"private range under private policy", PrivateNetworkOK(), "10.255.255.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.policy, 300*time.Millisecond)
			resp, err := client.Get("http://" + tt.host + ":9/")
			if err == nil {
				_ = resp.Body.Close()
				t.Fatal("unexpected success")
			}
			if got := errors.Is(err, ErrBlockedAddress); got != tt.wantBlocked {
				t.Errorf("blocked = %v, want %v (err: %v)", got, tt.wantBlocked, err)
			}
		})
	}
}

func TestValidateURL_LiteralOutcomesPerPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		policy  Policy
		raw     string
		wantErr bool
	}{
		{PublicHTTPSOnly(), "https://100.64.0.1/", true},
		{PrivateNetworkOK(), "http://100.64.0.1/", false},
		{PrivateNetworkOK(), "http://100.100.100.200/", true},
		{PrivateNetworkOK(), "http://198.18.0.1/", true},
		{PrivateNetworkOK(), "http://[2002::1]/", true},
	}
	for _, tt := range tests {
		_, err := ValidateURL(tt.raw, tt.policy)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURL(%q) err = %v, wantErr %v", tt.raw, err, tt.wantErr)
		}
	}
}
