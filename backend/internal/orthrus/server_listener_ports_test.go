package orthrus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestOrthrusServer_LiveListenerPorts_NilServer(t *testing.T) {
	var srv *OrthrusServer
	assert.Nil(t, srv.LiveListenerPorts())
}

func TestOrthrusServer_LiveListenerPorts_ReportsBoundPortsOnly(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	assert.Empty(t, srv.LiveListenerPorts())

	serverConn, done := testWSPair(t)
	defer done()
	sess, err := NewAgentSession("ports-uuid", "agent", false, nil, serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	sess.mu.Lock()
	sess.proxyPort = 41001
	sess.mu.Unlock()
	srv.sessions.Store("ports-uuid", sess)

	assert.Equal(t, []int{41001}, srv.LiveListenerPorts())
}

func TestOrthrusServer_SetListenerStartedHook_NilSafeSetter(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.SetListenerStartedHook(nil)
	called := false
	srv.SetListenerStartedHook(func() { called = true })
	srv.onListenerStarted()
	assert.True(t, called)
}

func TestAgentSession_ListenerPorts_IncludesActiveExternalProxy(t *testing.T) {
	sess, _, cleanup := sessionWithLoopback(t)
	defer cleanup()

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port, "127.0.0.1"))

	ports := sess.listenerPorts()
	assert.Len(t, ports, 2)
	assert.Contains(t, ports, port)
}

func TestOrthrusServer_HandleWebSocket_RunsListenerStartedHookWithLivePorts(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 50 * time.Millisecond

	token := "ch_orthrus_hook_test" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.OrthrusAgent{
		UUID:        "hook-uuid",
		Name:        "hook-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}).Error)

	// The hook must see the new session's loopback port, so a re-apply it
	// triggers evaluates the ports the agent now owns.
	portsAtHook := make(chan []int, 1)
	srv.SetListenerStartedHook(func() { portsAtHook <- srv.LiveListenerPorts() })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", srv.HandleWebSocket)
	ts := httptest.NewServer(router)
	t.Cleanup(srv.Stop)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	conn, _, err := websocket.Dial(context.Background(), wsURL, &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket manages the handshake response body
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	select {
	case ports := <-portsAtHook:
		require.Len(t, ports, 1)
		assert.Positive(t, ports[0])
	case <-time.After(2 * time.Second):
		t.Fatal("listener started hook did not run after the agent connected")
	}
}
