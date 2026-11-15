package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestProxyHostFlagsWith_MarksOnlyLeftOutHosts(t *testing.T) {
	t.Parallel()
	leftOut := map[string]struct{}{"gone": {}}
	flags := proxyHostFlagsWith(leftOut)

	gone := flags(models.ProxyHost{UUID: "gone"})
	assert.Equal(t, false, gone["active_config"], "a left-out host says so")

	active := flags(models.ProxyHost{UUID: "active", AdvancedConfig: `{"x":1}`})
	assert.NotContains(t, active, "active_config", "an active host omits the key")
	assert.Equal(t, true, active["has_advanced_config"])
}

func TestProxyHostFlags_NoHostLeftOut(t *testing.T) {
	t.Parallel()
	flags := proxyHostFlags(models.ProxyHost{UUID: "any"})
	assert.NotContains(t, flags, "active_config")
	assert.Equal(t, false, flags["has_advanced_config"])
}
