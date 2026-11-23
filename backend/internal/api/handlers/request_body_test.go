package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Every CrowdSec endpoint that reads a small JSON body refuses one above 64 KiB with a 413 before it does any work.
func TestCrowdsecJSONEndpointsLimitTheRequestBody(t *testing.T) {
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

// bodyLimitRouter answers 200 when the body binds as a JSON object, and 400 when the bind fails for any other
// reason. The route is guarded by limitRequestBody(n).
func bodyLimitRouter(n int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/limited", limitRequestBody(n), func(c *gin.Context) {
		var payload map[string]any
		if bindSmallJSON(c, &payload, func() { c.Status(http.StatusBadRequest) }) {
			c.Status(http.StatusOK)
		}
	})
	return r
}

func postBody(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/limited", strings.NewReader(body)))
	return w
}

// A body over the route limit is refused with 413 before the handler binds it, and the body at the limit is read.
func TestLimitRequestBodyRefusesOversizedBodyWith413(t *testing.T) {
	t.Parallel()
	const limit = 32
	r := bodyLimitRouter(limit)

	atLimit := `{"a":"` + strings.Repeat("x", limit-len(`{"a":""}`)) + `"}`
	require.Len(t, atLimit, limit)
	assert.Equal(t, http.StatusOK, postBody(r, atLimit).Code, "a body exactly at the limit is read")

	tooLarge := postBody(r, `{"a":"`+strings.Repeat("x", limit-len(`{"a":""}`)+1)+`"}`)
	require.Equal(t, http.StatusRequestEntityTooLarge, tooLarge.Code, tooLarge.Body.String())
	assert.Equal(t, codeRequestTooLarge, pr4Body(t, tooLarge)["error_code"])
}

// The limit set by the route stays in effect when the bind helper wraps the body again with its own, larger cap.
func TestLimitRequestBodyStaysInEffectUnderBindHelpers(t *testing.T) {
	t.Parallel()
	r := bodyLimitRouter(16)

	w := postBody(r, `{"a":"`+strings.Repeat("x", 64)+`"}`)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
}

// A body that is within the limit but not valid JSON is not reported as too large; the caller's 400 answer runs.
func TestBindSmallJSONSendsTheCallersAnswerForInvalidJSON(t *testing.T) {
	t.Parallel()
	r := bodyLimitRouter(1 << 10)

	w := postBody(r, `{"a":`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, w.Body.String(), "the helper must not write a body; the caller answers")
}

// The limit constants keep their documented sizes (see docs/features/api.md and the import guide).
func TestRequestBodyLimitConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(64<<10), int64(maxSmallJSONBodyBytes))
	assert.Equal(t, int64(1<<20), int64(maxHostRequestBodyBytes))
	assert.Equal(t, int64(4<<20), int64(maxImportBodyBytes))
	assert.Equal(t, int64(8<<20), int64(maxJSONImportBodyBytes))
}
