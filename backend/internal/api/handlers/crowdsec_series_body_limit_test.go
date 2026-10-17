package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Every CrowdSec endpoint that reads a small JSON body refuses one above 64 KiB with a 413 before it does any work.
func TestCrowdsecJSONEndpointsLimitTheRequestBody(t *testing.T) {
	skipUntilLaterCommit(t)
	t.Parallel()
	const limit = 64 << 10
	endpoints := []struct {
		name, method, target, field string
	}{
		{"console enroll", http.MethodPost, "/api/v1/admin/crowdsec/console/enroll", "enrollment_key"},
		{"ban", http.MethodPost, "/api/v1/admin/crowdsec/ban", "ip"},
		{"acquisition", http.MethodPut, "/api/v1/admin/crowdsec/acquisition", "content"},
		{"whitelist", http.MethodPost, "/api/v1/admin/crowdsec/whitelist", "ip_or_cidr"},
		{"preset pull", http.MethodPost, "/api/v1/admin/crowdsec/presets/pull", "slug"},
		{"preset apply", http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", "slug"},
	}
	body := func(field string, size int) string {
		return `{"` + field + `":"` + strings.Repeat("a", size) + `"}`
	}
	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			ex := pr4Answer(t, "decisions_add_ban")
			h := pr4Handler(t, ex, t.TempDir())
			require.NoError(t, h.DB.Create(&models.Setting{Key: "feature.crowdsec.console_enrollment", Value: "true"}).Error)
			require.NoError(t, h.DB.Create(&models.Setting{Key: "feature.cerberus.enabled", Value: "true"}).Error)
			r := pr4Router(h)

			w := pr4Serve(r, ep.method, ep.target, body(ep.field, limit+1))

			require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
			resp := pr4Body(t, w)
			assert.Equal(t, "request_too_large", resp["error_code"])
			assert.NotEmpty(t, resp["error"])
			assert.Zero(t, ex.callCount(), "nothing ran")

			// A body that stays within the limit is read as before.
			small := pr4Serve(r, ep.method, ep.target, body(ep.field, 1000))
			assert.NotEqual(t, http.StatusRequestEntityTooLarge, small.Code, small.Body.String())
		})
	}
}

// A body just inside the limit is still read.
func TestCrowdsecJSONBodyJustBelowTheLimitIsRead(t *testing.T) {
	t.Parallel()
	h := pr4Handler(t, pr4Answer(t, "decisions_add_ban"), t.TempDir())

	w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, `{"ip":"`+strings.Repeat("a", 60<<10)+`"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code, "the value is read and refused as an address, not as an oversize body")
	assert.Equal(t, "invalid_ip", pr4Body(t, w)["error_code"])
}
