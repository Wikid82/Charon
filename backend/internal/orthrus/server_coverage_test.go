package orthrus

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestOrthrusServer_GetSession_KnownUUID(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("sess-uuid", "sess-agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	srv.sessions.Store("sess-uuid", sess)

	got, ok := srv.GetSession("sess-uuid")
	assert.True(t, ok)
	assert.Equal(t, sess, got)
}

func TestOrthrusServer_GetProxyAddr_SessionExists_NoProxy(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("no-proxy-uuid", "agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	srv.sessions.Store("no-proxy-uuid", sess)

	addr, ok := srv.GetProxyAddr("no-proxy-uuid")
	assert.Equal(t, "", addr)
	assert.False(t, ok)
}

func TestOrthrusServer_GetProxyAddr_SessionExists_WithProxy(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("with-proxy-uuid", "agent", serverConn)
	require.NoError(t, err)
	defer func() { _ = sess.Close() }()

	sess.mu.Lock()
	sess.proxyPort = 9876
	sess.mu.Unlock()

	srv.sessions.Store("with-proxy-uuid", sess)

	addr, ok := srv.GetProxyAddr("with-proxy-uuid")
	assert.True(t, ok)
	assert.Equal(t, "127.0.0.1:9876", addr)
}

func TestOrthrusServer_DisconnectAgent_WithSession(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("disc-uuid", "disc-agent", serverConn)
	require.NoError(t, err)

	srv.sessions.Store("disc-uuid", sess)

	err = srv.DisconnectAgent("disc-uuid")
	assert.NoError(t, err)

	_, ok := srv.GetSession("disc-uuid")
	assert.False(t, ok)
}

func TestOrthrusServer_MarkOffline_UpdatesAgentStatus(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:   "offline-uuid",
		Name:   "offline-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	srv.markOffline("offline-uuid")

	var updated models.OrthrusAgent
	require.NoError(t, db.Where("uuid = ?", "offline-uuid").First(&updated).Error)
	assert.Equal(t, models.OrthrusStatusOffline, updated.Status)
}

func TestOrthrusServer_MarkOffline_DBError_NosPanic(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	// Should log warning but not panic when DB is unavailable.
	srv.markOffline("ghost-uuid")
}

func TestOrthrusServer_FindAgentByToken_Success(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	token := "ch_orthrus_testtoken123" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "find-uuid",
		Name:        "find-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	found, err := srv.findAgentByToken(token)
	require.NoError(t, err)
	assert.Equal(t, "find-uuid", found.UUID)
}

func TestOrthrusServer_HandleWebSocket_NoAuthHeader_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)

	srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrthrusServer_HandleWebSocket_InvalidToken_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	c.Request = req

	srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrthrusServer_WatchHeartbeat_ClosedSession_ExitsAndMarksOffline(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	t.Cleanup(srv.Stop) // drains watchHeartbeat and yamux goroutines before TempDir cleanup
	srv.heartbeatTimeout = 40 * time.Millisecond

	serverConn, done := testWSPair(t)
	defer done()

	sess, err := NewAgentSession("wh-uuid", "wh-agent", serverConn)
	require.NoError(t, err)

	// Close session immediately so IsAlive() returns false on first tick.
	require.NoError(t, sess.Close())

	agent := &models.OrthrusAgent{
		UUID:   "wh-uuid",
		Name:   "wh-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	srv.sessions.Store("wh-uuid", sess)

	finished := make(chan struct{})
	go func() {
		srv.watchHeartbeat("wh-uuid", sess)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(600 * time.Millisecond):
		t.Fatal("watchHeartbeat did not exit within deadline")
	}

	_, ok := srv.GetSession("wh-uuid")
	assert.False(t, ok, "session should be deleted after disconnect")
}

func TestOrthrusServer_HandleWebSocket_ValidToken_UpgradesConnection(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 50 * time.Millisecond

	token := "ch_orthrus_wscov789" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "wscov-uuid",
		Name:        "wscov-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", srv.HandleWebSocket)

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	t.Cleanup(srv.Stop) // must run before DB/TempDir cleanups to drain watchHeartbeat goroutine

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}

	conn, dialResp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	var ok bool
	for i := 0; i < 20; i++ {
		_, ok = srv.GetSession("wscov-uuid")
		if ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.True(t, ok)
}
func TestOrthrusServer_FindAgentByToken_DBError_ReturnsError(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	_, err = srv.findAgentByToken("anytoken")
	assert.Error(t, err)
}

// TestOrthrusServer_HandleWebSocket_UpgradeFailure covers server.go:82-85 —
// the error-log path when wsUpgrader.Upgrade rejects a non-WebSocket request.
func TestOrthrusServer_HandleWebSocket_UpgradeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	token := "ch_orthrus_upgerr01" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "upgerr-uuid",
		Name:        "upgerr-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	// Plain HTTP GET with valid auth — NOT a WebSocket upgrade.
	// wsUpgrader.Upgrade writes 400 Bad Request and returns an error.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	c.Request = req

	// Should not panic; handler logs the error and returns.
	srv.HandleWebSocket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestOrthrusServer_HandleWebSocket_ExternalProxyFails covers server.go:100-104 —
// the warning-log path when StartExternalProxy fails on an occupied port.
func TestOrthrusServer_HandleWebSocket_ExternalProxyFails(t *testing.T) {
	// Occupy a port so StartExternalProxy cannot bind it.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	blockedPort := blocker.Addr().(*net.TCPAddr).Port
	defer blocker.Close() //nolint:errcheck

	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 200 * time.Millisecond

	token := "ch_orthrus_extfail02" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:              "extfail-uuid",
		Name:              "extfail-agent",
		AuthKeyHash:       string(hash),
		Status:            models.OrthrusStatusPending,
		ExternalProxyPort: blockedPort,
	}
	require.NoError(t, db.Create(agent).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", srv.HandleWebSocket)
	ts := httptest.NewServer(router)
	t.Cleanup(srv.Stop)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	conn, dialResp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	// sessions.Store is called after StartExternalProxy, so once the session is
	// present the proxy attempt (and its failure) has already completed.
	assert.Eventually(t, func() bool {
		_, ok := srv.GetSession("extfail-uuid")
		return ok
	}, 2*time.Second, 20*time.Millisecond, "session should be stored after WS connect")

	status, ok := srv.GetExternalProxyStatus("extfail-uuid")
	require.True(t, ok)
	assert.False(t, status.Active)
	assert.NotEmpty(t, status.Error)
}

// TestHandleWebSocket_DisplacesExistingSession covers server.go:98-100 —
// the displacement block that closes the old session when a new connection
// arrives for an agent UUID that already has an active session in the map.
func TestHandleWebSocket_DisplacesExistingSession(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 200 * time.Millisecond

	token := "ch_orthrus_displace01" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "displace-uuid",
		Name:        "displace-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	// Create an "old" session and store it in the sessions map to simulate a
	// prior connection that has not yet been cleaned up.
	oldConn, oldCleanup := testWSPair(t)
	defer oldCleanup()
	oldSess, err := NewAgentSession("displace-uuid", "displace-agent", oldConn)
	require.NoError(t, err)
	srv.sessions.Store("displace-uuid", oldSess)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", srv.HandleWebSocket)
	ts := httptest.NewServer(router)
	t.Cleanup(srv.Stop)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	conn, dialResp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	if dialResp != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.Close() }()

	// Wait for the new session to be stored, which means HandleWebSocket has run
	// past the displacement block and stored the replacement session.
	assert.Eventually(t, func() bool {
		raw, ok := srv.GetSession("displace-uuid")
		return ok && raw != oldSess
	}, 2*time.Second, 20*time.Millisecond, "new session should replace old session")

	// The old session must have been closed by the displacement block (lines 98-100).
	assert.False(t, oldSess.IsAlive(), "old session must be closed by displacement")
}
