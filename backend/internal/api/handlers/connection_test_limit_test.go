package handlers

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/services"
)

// gateContext returns a request context for a caller. A zero user leaves the
// caller unknown, so the limit falls back to the client address.
func gateContext(userID uint, remoteAddr string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	c.Request.RemoteAddr = remoteAddr
	if userID > 0 {
		c.Set(middleware.UserIDKey, userID)
	}
	return c, w
}

// smallLimiter allows burst connection tests in a minute, with no refill inside the test.
func smallLimiter(t *testing.T, burst int) *ratelimit.KeyedLimiter {
	t.Helper()
	rate, b := ratelimit.PerWindow(burst, time.Minute)
	return ratelimit.MustNewKeyedLimiter(ratelimit.Config{Rate: rate, Burst: b})
}

// closedTestPort returns a loopback port with nothing listening on it.
func closedTestPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

func TestConnectionGate_EleventhCallIsTooManyRequests(t *testing.T) {
	var gate connectionTestGate
	const user = 7001
	for i := 0; i < connectionTestsPerWindow; i++ {
		c, _ := gateContext(user, "10.0.0.1:1000")
		require.True(t, gate.allowConnectionTest(c), "call %d", i+1)
	}
	c, w := gateContext(user, "10.0.0.1:1000")
	assert.False(t, gate.allowConnectionTest(c))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"error":"`+ratelimit.TooManyRequestsMessage+`"}`, w.Body.String())
}

func TestConnectionGate_OtherUserUnaffected(t *testing.T) {
	var gate connectionTestGate
	for i := 0; i < connectionTestsPerWindow; i++ {
		c, _ := gateContext(7101, "10.0.0.2:1000")
		gate.allowConnectionTest(c)
	}
	c, _ := gateContext(7101, "10.0.0.2:1000")
	require.False(t, gate.allowConnectionTest(c))

	other, _ := gateContext(7102, "10.0.0.2:1000")
	assert.True(t, gate.allowConnectionTest(other), "another user has a full allowance")
}

func TestConnectionGate_SetterReplacesLimiter(t *testing.T) {
	var gate connectionTestGate
	gate.SetConnectionTestLimiter(smallLimiter(t, 1))
	first, _ := gateContext(7201, "10.0.0.3:1000")
	second, w := gateContext(7201, "10.0.0.3:1000")
	assert.True(t, gate.allowConnectionTest(first))
	assert.False(t, gate.allowConnectionTest(second))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestConnectionGate_UnknownCallerIsKeyedByAddress(t *testing.T) {
	var gate connectionTestGate
	gate.SetConnectionTestLimiter(smallLimiter(t, 1))
	c1, _ := gateContext(0, "10.77.7.7:1000")
	c2, _ := gateContext(0, "10.77.7.7:2000")
	other, _ := gateContext(0, "10.77.7.8:1000")
	assert.True(t, gate.allowConnectionTest(c1))
	assert.False(t, gate.allowConnectionTest(c2), "same client address shares the allowance")
	assert.True(t, gate.allowConnectionTest(other))
}

func TestConnectionGate_DefaultLimiterAppliesWithoutSetter(t *testing.T) {
	var gate connectionTestGate
	const user = 7301
	for i := 0; i < connectionTestsPerWindow; i++ {
		c, _ := gateContext(user, "10.0.0.4:1000")
		gate.allowConnectionTest(c)
	}
	c, _ := gateContext(user, "10.0.0.4:1000")
	assert.False(t, gate.allowConnectionTest(c), "the package limiter is used when no setter ran")
}

func TestProxyHostTestConnection_LimitedAndFixedText(t *testing.T) {
	db := OpenTestDB(t)
	h := &ProxyHostHandler{service: services.NewProxyHostService(db)}
	h.SetConnectionTestLimiter(smallLimiter(t, 2))
	port := closedTestPort(t)

	for i := 0; i < 2; i++ {
		c, w := gateContext(7401, "10.0.0.5:1000")
		c.Request.Body = nopBody(`{"forward_host":"127.0.0.1","forward_port":` + strconv.Itoa(port) + `}`)
		c.Request.Header.Set("Content-Type", "application/json")
		h.TestConnection(c)
		assert.Equal(t, http.StatusBadGateway, w.Code)
		assert.JSONEq(t, `{"error":"Could not connect to the target.","error_code":"connection_test_failed"}`, w.Body.String())
	}

	c, w := gateContext(7401, "10.0.0.5:1000")
	c.Request.Body = nopBody(`{"forward_host":"127.0.0.1","forward_port":` + strconv.Itoa(port) + `}`)
	c.Request.Header.Set("Content-Type", "application/json")
	h.TestConnection(c)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotContains(t, w.Body.String(), "connection_test_failed", "no dial result is returned once limited")
}

func TestRemoteServerTestCustom_LimitedBeforeDial(t *testing.T) {
	db := setupRemoteServerCoverageDB2(t)
	h := &RemoteServerHandler{service: services.NewRemoteServerService(db)}
	h.SetConnectionTestLimiter(smallLimiter(t, 1))
	port := closedTestPort(t)

	first, w1 := gateContext(7501, "10.0.0.6:1000")
	first.Request.Body = nopBody(`{"host":"127.0.0.1","port":` + strconv.Itoa(port) + `}`)
	first.Request.Header.Set("Content-Type", "application/json")
	h.TestConnectionCustom(first)
	assert.Equal(t, http.StatusOK, w1.Code)

	second, w2 := gateContext(7501, "10.0.0.6:1000")
	second.Request.Body = nopBody(`{"host":"127.0.0.1","port":` + strconv.Itoa(port) + `}`)
	second.Request.Header.Set("Content-Type", "application/json")
	h.TestConnectionCustom(second)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestConnectionTestLog_NamesNoAddressPortOrCause(t *testing.T) {
	buf, _ := captureDebugLogs(t)
	db := OpenTestDB(t)
	h := &ProxyHostHandler{service: services.NewProxyHostService(db)}
	port := closedTestPort(t)

	c, _ := gateContext(7601, "10.0.0.7:1000")
	c.Request.Body = nopBody(`{"forward_host":"127.0.0.1","forward_port":` + strconv.Itoa(port) + `}`)
	c.Request.Header.Set("Content-Type", "application/json")
	h.TestConnection(c)

	logged := buf.String()
	assert.Contains(t, logged, "connection test finished")
	assert.Contains(t, logged, "outcome=failed")
	assert.Contains(t, logged, "caller_id=7601")
	assert.NotContains(t, logged, "127.0.0.1")
	assert.NotContains(t, logged, strconv.Itoa(port))
	assert.NotContains(t, logged, "refused")
}

// nopBody returns a request body holding s.
func nopBody(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}
