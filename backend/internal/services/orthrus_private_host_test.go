package services

import "testing"

// TestOrthrusIsPrivateAgentHost mirrors the agent's TestIsPrivateAgentHost
// table. The agent is a separate module, so both copies of the rule carry the
// same cases and must be kept in step.
func TestOrthrusIsPrivateAgentHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.1.2.3", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"100.128.0.1", false},
		{"fd7a:115c:a1e0::1", true},
		{"::ffff:10.0.0.1", true},
		{"fe80::1", false},
		{"169.254.1.1", false},
		{"8.8.8.8", false},
		{"localhost", true},
		{"LOCALHOST.", true},
		{"app.localhost", true},
		{"charon", true},
		{"charon.", true},
		{"nas.local", true},
		{"nas.LOCAL.", true},
		{"router.home.arpa", true},
		{"svc.internal", true},
		{"charon.lan", false},
		{"host.tailnet.ts.net", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			if got := isPrivateAgentHost(tc.host); got != tc.want {
				t.Errorf("isPrivateAgentHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
