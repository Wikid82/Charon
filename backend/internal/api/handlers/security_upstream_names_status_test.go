package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
)

// upstreamNamesManager is a caddy manager that reports whether the upstream name answerer runs.
type upstreamNamesManager struct {
	available bool
}

func (upstreamNamesManager) ApplyConfig(context.Context) error { return nil }

func (m upstreamNamesManager) UpstreamNameChecksAvailable() bool { return m.available }

// noReporterManager is a caddy manager that does not report the answerer.
type noReporterManager struct{}

func (noReporterManager) ApplyConfig(context.Context) error { return nil }

// upstreamNameStatus reads upstream_name_checks from the status endpoint. ok is false when the field is absent.
func upstreamNameStatus(t *testing.T, h *SecurityHandler) (map[string]any, bool) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/security/status", http.NoBody)
	h.GetStatus(c)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	status, ok := body["upstream_name_checks"].(map[string]any)
	return status, ok
}

func TestSecurityStatus_UpstreamNameChecksReflectTheAnswerer(t *testing.T) {
	db := OpenTestDB(t)

	running := NewSecurityHandler(config.SecurityConfig{}, db, upstreamNamesManager{available: true})
	status, ok := upstreamNameStatus(t, running)
	require.True(t, ok)
	assert.Equal(t, true, status["available"])

	notRunning := NewSecurityHandler(config.SecurityConfig{}, db, upstreamNamesManager{available: false})
	status, ok = upstreamNameStatus(t, notRunning)
	require.True(t, ok)
	assert.Equal(t, false, status["available"])
}

func TestSecurityStatus_UpstreamNameChecksOmittedWithoutAReporter(t *testing.T) {
	db := OpenTestDB(t)

	_, ok := upstreamNameStatus(t, NewSecurityHandler(config.SecurityConfig{}, db, noReporterManager{}))
	assert.False(t, ok, "a manager that does not report the answerer adds no field")

	_, ok = upstreamNameStatus(t, NewSecurityHandler(config.SecurityConfig{}, db, nil))
	assert.False(t, ok, "without a manager the field is absent")
}
