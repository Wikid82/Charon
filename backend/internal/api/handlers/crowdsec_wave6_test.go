package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCrowdsecWave6_BouncerKeyPath_UsesEnvFallback(t *testing.T) {
	t.Setenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH", "/tmp/test-bouncer-key")
	h := &CrowdsecHandler{}
	require.Equal(t, "/tmp/test-bouncer-key", h.bouncerKeyPath())
}

func TestCrowdsecWave6_GetBouncerInfo_NoneSource(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "")
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "")
	t.Setenv("CERBERUS_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CPM_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH", "/tmp/non-existent-wave6-key")

	h := &CrowdsecHandler{CmdExec: &mockCmdExecutor{output: []byte(`[]`)}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/bouncer", nil)

	h.GetBouncerInfo(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "none", payload["key_source"])
}

func TestCrowdsecWave6_GetKeyStatus_NoKeyConfiguredMessage(t *testing.T) {
	t.Setenv("CROWDSEC_API_KEY", "")
	t.Setenv("CROWDSEC_BOUNCER_API_KEY", "")
	t.Setenv("CERBERUS_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CPM_SECURITY_CROWDSEC_API_KEY", "")
	t.Setenv("CHARON_CROWDSEC_BOUNCER_KEY_PATH", "/tmp/non-existent-wave6-key")

	h := &CrowdsecHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/key-status", nil)

	h.GetKeyStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "none", payload["key_source"])
	require.Equal(t, false, payload["valid"])
	require.Contains(t, payload["message"], "No CrowdSec API key configured")
}
