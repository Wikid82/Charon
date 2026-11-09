package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	dockerBridgeWarningMessage = "This looks like a Docker container IP address. Docker IPs can change when containers restart. Consider using the container name for more reliable connections."
	privateIPWarningMessage    = "Using a private IP address. Make sure Charon can reach it (for example over your LAN or VPN). If this is a Docker container, its IP may change on restart; container names are more reliable."
)

func TestGenerateForwardHostWarnings_Branches(t *testing.T) {
	tests := []struct {
		name        string
		forwardHost string
		wantMessage string // empty means no warning expected
	}{
		{name: "docker bridge IP", forwardHost: "172.17.0.1", wantMessage: dockerBridgeWarningMessage},
		{name: "RFC1918 192.168 LAN", forwardHost: "192.168.1.100", wantMessage: privateIPWarningMessage},
		{name: "RFC1918 10/8 VPN-style address", forwardHost: "10.8.0.5", wantMessage: privateIPWarningMessage},
		{name: "public IP has no warning", forwardHost: "8.8.8.8", wantMessage: ""},
		{name: "hostname has no warning", forwardHost: "my-app", wantMessage: ""},
		{name: "empty host has no warning", forwardHost: "", wantMessage: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := generateForwardHostWarnings(tt.forwardHost)
			if tt.wantMessage == "" {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, "forward_host", warnings[0].Field)
			assert.Equal(t, tt.wantMessage, warnings[0].Message)
		})
	}
}
