package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// paddedBody returns a JSON object of exactly total bytes: base plus an unused "pad" member. Handlers ignore keys
// they do not bind, so the padding only changes the size of the request body.
func paddedBody(t *testing.T, base map[string]any, total int) string {
	t.Helper()
	base["pad"] = ""
	empty, err := json.Marshal(base)
	require.NoError(t, err)
	require.LessOrEqual(t, len(empty), total)
	base["pad"] = strings.Repeat("a", total-len(empty))
	raw, err := json.Marshal(base)
	require.NoError(t, err)
	require.Len(t, raw, total)
	return string(raw)
}

// hostRouteCase is one host route that reads a body; base is the JSON object sent, padded to size.
type hostRouteCase struct {
	name   string
	method string
	path   string
	base   map[string]any
}

// rowCount counts the rows of model, so a test can show that a refused request wrote nothing.
func rowCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Count(&n).Error)
	return n
}

// Every host route refuses a body one byte over 1 MiB with 413 and writes nothing, and accepts a body of exactly
// 1 MiB.
func TestHostRoutesLimitTheRequestBody(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "limit-host")
	redir := models.RedirectionHost{UUID: uuid.NewString(), DomainNames: "limit-redir.example.com", TargetURL: "https://t.example.com", StatusCode: 301}
	require.NoError(t, db.Create(&redir).Error)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	cases := []hostRouteCase{
		{"proxy create", http.MethodPost, "/api/v1/proxy-hosts",
			map[string]any{"name": "big", "domain_names": "big-create.example.com", "forward_host": "10.0.0.9", "forward_port": 8080}},
		{"proxy update", http.MethodPut, "/api/v1/proxy-hosts/" + host.UUID,
			map[string]any{"name": "big"}},
		{"proxy test", http.MethodPost, "/api/v1/proxy-hosts/test",
			map[string]any{"forward_host": "127.0.0.1", "forward_port": 1}},
		{"bulk acl", http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl",
			map[string]any{"host_uuids": []string{host.UUID}}},
		{"bulk group", http.MethodPut, "/api/v1/proxy-hosts/bulk-update-group",
			map[string]any{"host_uuids": []string{host.UUID}}},
		{"bulk security headers", http.MethodPut, "/api/v1/proxy-hosts/bulk-update-security-headers",
			map[string]any{"host_uuids": []string{host.UUID}}},
		{"redirection create", http.MethodPost, "/api/v1/redirection-hosts",
			map[string]any{"domain_names": "big-redir.example.com", "target_url": "https://t.example.com"}},
		{"redirection update", http.MethodPut, "/api/v1/redirection-hosts/" + redir.UUID,
			map[string]any{"name": "big"}},
	}
	const limit = 1 << 20
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := rowCount(t, db, &models.ProxyHost{}) + rowCount(t, db, &models.RedirectionHost{})

			over := f.doRaw(t, tc.method, tc.path, paddedBody(t, copyMap(tc.base), limit+1))
			require.Equal(t, http.StatusRequestEntityTooLarge, over.Code, over.Body.String())
			assert.Equal(t, "request_too_large", decodeBody(t, over)["error_code"])
			after := rowCount(t, db, &models.ProxyHost{}) + rowCount(t, db, &models.RedirectionHost{})
			assert.Equal(t, before, after, "nothing is written for a refused body")

			atLimit := f.doRaw(t, tc.method, tc.path, paddedBody(t, copyMap(tc.base), limit))
			assert.NotEqual(t, http.StatusRequestEntityTooLarge, atLimit.Code, atLimit.Body.String())
		})
	}
}

// copyMap returns a shallow copy of base, so each case builds its own body.
func copyMap(base map[string]any) map[string]any {
	out := make(map[string]any, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	return out
}

// doRaw sends a raw JSON body through the fixture router.
func (f hostRestoreFixture) doRaw(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

// An advanced configuration over 256 KiB is refused on create with its own code.
func TestProxyHostCreateRefusesAnOversizedAdvancedConfig(t *testing.T) {
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})
	big := `{"handler":"` + strings.Repeat("a", services.MaxAdvancedConfigBytes) + `"}`

	w := f.do(t, http.MethodPost, "/api/v1/proxy-hosts", map[string]any{
		"name":            "big-advanced",
		"domain_names":    "big-advanced.example.com",
		"forward_host":    "10.0.0.9",
		"forward_port":    8080,
		"advanced_config": big,
	})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	body := decodeBody(t, w)
	assert.Equal(t, "advanced_config_too_large", body["error_code"])
	assert.Equal(t, "The advanced configuration is too large (limit 256 KiB).", body["error"])
	assert.Zero(t, rowCount(t, db, &models.ProxyHost{}))
}

// A host whose stored advanced configuration is over the limit still takes unrelated edits.
func TestProxyHostUnrelatedEditKeepsAStoredOversizedAdvancedConfig(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "stored-big")
	stored := `{"handler":"` + strings.Repeat("b", services.MaxAdvancedConfigBytes) + `"}`
	require.NoError(t, db.Model(&models.ProxyHost{}).Where("id = ?", host.ID).Update("advanced_config", stored).Error)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, map[string]any{"name": "renamed"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "renamed", storedHost(t, db, host.ID).Name)
}

// Changing the advanced configuration of a host to an oversized value is refused.
func TestProxyHostChangingToAnOversizedAdvancedConfigIsRefused(t *testing.T) {
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "change-big")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})
	big := `{"handler":"` + strings.Repeat("c", services.MaxAdvancedConfigBytes) + `"}`

	w := f.do(t, http.MethodPut, "/api/v1/proxy-hosts/"+host.UUID, map[string]any{"advanced_config": big})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "advanced_config_too_large", decodeBody(t, w)["error_code"])
	assert.Empty(t, storedHost(t, db, host.ID).AdvancedConfig)
}

// The host handler's bind helpers answer a body over the limit with 413 and a request that merely fails to bind
// with the caller's 400 (not 413).
func TestHostBindFailureIsNotReportedAsTooLarge(t *testing.T) {
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})

	w := f.doRaw(t, http.MethodPost, "/api/v1/proxy-hosts", `{"name":`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotEqual(t, "request_too_large", decodeBody(t, w)["error_code"])
}
