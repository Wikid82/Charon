package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInternalSettingKey_ReservedSecurityKeys(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"security.proxy.trusted_proxies", true},
		{"  Security.Proxy.Trusted_Proxies ", true},
		{"security.acl.management_lists", true},
		{"security.acl.enabled", false},
		{"security.proxy.trusted_proxies_extra", false},
		{"maintenance.flag", true},
		{"migration.v1", true},
		{"app.public_url", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, isInternalSettingKey(tt.key))
		})
	}
}
