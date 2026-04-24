package security

import (
	"strings"
	"testing"
)

func TestParseExactHostnameAllowlist(t *testing.T) {
	allow := ParseExactHostnameAllowlist(" crowdsec , CADDY, ,http://example.com,example.com/path,user@host,::1 ")

	if _, ok := allow["crowdsec"]; !ok {
		t.Fatalf("expected allowlist to contain crowdsec")
	}
	if _, ok := allow["caddy"]; !ok {
		t.Fatalf("expected allowlist to contain caddy")
	}
	if _, ok := allow["::1"]; !ok {
		t.Fatalf("expected allowlist to contain ::1")
	}

	if _, ok := allow["http://example.com"]; ok {
		t.Fatalf("expected scheme-containing entry to be ignored")
	}
	if _, ok := allow["example.com/path"]; ok {
		t.Fatalf("expected path-containing entry to be ignored")
	}
	if _, ok := allow["user@host"]; ok {
		t.Fatalf("expected userinfo-containing entry to be ignored")
	}
}

func TestValidateInternalServiceBaseURL(t *testing.T) {
	allowed := map[string]struct{}{"localhost": {}, "127.0.0.1": {}, "::1": {}}

	cases := []struct {
		name         string
		raw          string
		expectedPort int
		want         string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "OK http localhost explicit port",
			raw:          "http://localhost:2019",
			expectedPort: 2019,
			want:         "http://localhost:2019",
		},
		{
			name:         "OK http localhost path normalized",
			raw:          "http://localhost:2019/config/",
			expectedPort: 2019,
			want:         "http://localhost:2019",
		},
		{
			name:         "OK https localhost default port",
			raw:          "https://localhost",
			expectedPort: 443,
			want:         "https://localhost:443",
		},
		{
			name:         "OK ipv6 loopback explicit port",
			raw:          "http://[::1]:2019",
			expectedPort: 2019,
			want:         "http://[::1]:2019",
		},
		{ //nolint:gosec // test verifies rejection of embedded credentials in URL
			name:         "Reject userinfo",
			raw:          "http://user:pass@localhost:2019",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "embedded credentials",
		},
		{
			name:         "Reject unsupported scheme",
			raw:          "file://localhost:2019",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "unsupported scheme",
		},
		{
			name:         "Reject missing hostname",
			raw:          "http://:2019",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "missing hostname",
		},
		{
			name:         "Reject hostname not allowed",
			raw:          "http://evil.example:2019",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "hostname not allowed",
		},
		{
			name:         "Reject unexpected port when omitted",
			raw:          "http://localhost",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "unexpected port",
		},
		{
			name:         "Reject invalid port",
			raw:          "http://localhost:0",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "invalid port",
		},
		{
			name:         "Reject out-of-range port",
			raw:          "http://localhost:99999",
			expectedPort: 2019,
			wantErr:      true,
			errContains:  "invalid port",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := ValidateInternalServiceBaseURL(tc.raw, tc.expectedPort, allowed)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error to contain %q, got %q", tc.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.String() != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, u.String())
			}
		})
	}
}
