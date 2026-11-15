package handlers

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

// openLoopbackPort returns a loopback port that accepts connections.
func openLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

// remoteCustomRequest runs the custom remote server test with the given body.
func remoteCustomRequest(t *testing.T, host string, port int) *httptest.ResponseRecorder {
	t.Helper()
	db := setupRemoteServerCoverageDB2(t)
	h := NewRemoteServerHandler(services.NewRemoteServerService(db), nil)
	body, err := json.Marshal(map[string]any{"host": host, "port": port})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/remote-servers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.TestConnectionCustom(c)
	return w
}

func TestRemoteServerTestCustom_RefusedDestinationIsBadRequest(t *testing.T) {
	w := remoteCustomRequest(t, "169.254.169.254", 80)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"This destination is not allowed.","error_code":"destination_refused"}`, w.Body.String())
}

func TestRemoteServerTestCustom_UnreachableUsesFixedText(t *testing.T) {
	w := remoteCustomRequest(t, "127.0.0.1", closedPort(t))
	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, false, got["reachable"])
	assert.Equal(t, "Could not connect to the target.", got["error"])
	assert.NotContains(t, got["error"], "127.0.0.1", "the error text names no address")
}

func TestRemoteServerTestCustom_ReachableReportsLatency(t *testing.T) {
	port := openLoopbackPort(t)
	w := remoteCustomRequest(t, "127.0.0.1", port)
	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, true, got["reachable"])
	assert.Contains(t, got, "latency_ms")
	assert.Equal(t, strconv.Itoa(port), got["address"].(string)[len("127.0.0.1:"):])
}

func TestRemoteServerTestCustom_DatabaseErrorIsServerError(t *testing.T) {
	db := setupRemoteServerCoverageDB2(t)
	svc := services.NewRemoteServerService(db)
	h := NewRemoteServerHandler(svc, nil)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	body := []byte(`{"host":"127.0.0.1","port":2019}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/remote-servers/test", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.TestConnectionCustom(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"Could not run the connection test."}`, w.Body.String())
}

func TestProxyHostTestConnection_FailureUsesFixedText(t *testing.T) {
	p := setupHostPermsRouter(t)
	closed := closedPort(t)

	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts/test", "user", map[string]any{
		"forward_host": "127.0.0.1",
		"forward_port": closed,
	})
	assert.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":"Could not connect to the target."}`, rec.Body.String())
}

// closedPort returns a loopback port with nothing listening on it.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}
