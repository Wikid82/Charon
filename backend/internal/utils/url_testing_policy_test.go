package utils

import (
	"context"
	"testing"
)

func TestResolveAllowedIP_PolicyOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		host           string
		allowLocalhost bool
		wantErr        bool
	}{
		{"public literal", "8.8.8.8", false, false},
		{"loopback literal", "127.0.0.1", false, true},
		{"loopback literal allowed", "127.0.0.1", true, false},
		{"loopback v6 allowed", "::1", true, false},
		{"private literal", "10.0.0.1", true, true},
		{"link-local literal", "169.254.169.254", true, true},
		{"metadata alias", "100.100.100.200", false, true},
		{"shared space literal", "100.64.0.1", false, true},
		{"shared space literal allowed loopback only", "100.64.0.1", true, true},
		{"special-purpose literal", "198.18.0.1", false, true},
		{"special-purpose v4 literal", "192.0.0.1", false, true},
		{"translation literal", "64:ff9b::808:808", false, true},
		{"translation local-use literal", "64:ff9b:1::1", false, true},
		{"tunnel literal", "2002:c000:204::1", false, true},
		{"embedded v4 literal", "::1.2.3.4", false, true},
		{"tunnel v6 literal", "2001:0:4136:e378:8000:63bf:3fff:fdd2", false, true},
		{"mapped shared space literal", "::ffff:100.64.0.1", false, true},
		{"loopback name blocked", "localhost", false, true},
		{"loopback name allowed", "localhost", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveAllowedIP(context.Background(), tt.host, tt.allowLocalhost)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveAllowedIP(%q, %v) err = %v, wantErr %v", tt.host, tt.allowLocalhost, err, tt.wantErr)
			}
		})
	}
}

func TestOutboundDialer_RejectsLoopbackAlways(t *testing.T) {
	t.Parallel()
	dial := ssrfSafeDialer()
	for _, addr := range []string{"127.0.0.1:80", "[::1]:80", "10.0.0.1:80", "100.100.100.200:80"} {
		conn, err := dial(context.Background(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
			t.Errorf("dial %s succeeded", addr)
		}
	}
}
