package orthrus

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupServerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "orthrus_test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}))
	return db
}

func setupTestCA(t *testing.T) *InternalCA {
	t.Helper()
	ca, err := NewInternalCA(t.TempDir())
	require.NoError(t, err)
	return ca
}

func TestNewOrthrusServer_Initialises(t *testing.T) {
	db := setupServerTestDB(t)
	ca := setupTestCA(t)

	srv, err := NewOrthrusServer(db, ca)
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestOrthrusServer_GetProxyAddr_UnknownUUID(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	addr, ok := srv.GetProxyAddr("nonexistent-uuid")
	assert.Equal(t, "", addr)
	assert.False(t, ok)
}

func TestOrthrusServer_GetSession_UnknownUUID(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	sess, ok := srv.GetSession("nonexistent-uuid")
	assert.Nil(t, sess)
	assert.False(t, ok)
}

func TestOrthrusServer_DisconnectAgent_UnknownUUID_Noop(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	err = srv.DisconnectAgent("ghost-uuid")
	assert.NoError(t, err)
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		header   string
		expected string
	}{
		{"Bearer abc123", "abc123"},
		{"bearer abc123", ""},
		{"Token abc123", ""},
		{"", ""},
		{"Bearer ", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, extractBearer(tc.header), "header: %q", tc.header)
	}
}

func TestOrthrusServer_FindAgentByToken_NoAgents(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	_, err = srv.findAgentByToken("ch_orthrus_sometoken")
	assert.Error(t, err)
}

func TestOrthrusServer_FindAgentByToken_DBError(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	_, err = srv.findAgentByToken("ch_orthrus_anytoken")
	assert.Error(t, err)
}

func TestOrthrusServer_FindAgentByToken_LongToken(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	longToken := "ch_orthrus_" + string(make([]byte, 72))
	hash, err := bcrypt.GenerateFromPassword([]byte("some-other-token"), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "uuid-long",
		Name:        "long-token-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	_, err = srv.findAgentByToken(longToken)
	assert.Error(t, err)
}

func TestOrthrusServer_FindAgentByToken_MatchingToken(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	plainKey := "ch_orthrus_testtoken"
	hash, err := bcrypt.GenerateFromPassword([]byte(plainKey), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:        "uuid-match",
		Name:        "match-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	found, err := srv.findAgentByToken(plainKey)
	require.NoError(t, err)
	assert.Equal(t, "uuid-match", found.UUID)
}

func TestOrthrusServer_HandleWebSocket_EmptyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orthrus/ws", http.NoBody)
	// No Authorization header → extractBearer returns "" → 401
	srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrthrusServer_HandleWebSocket_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orthrus/ws", http.NoBody)
	c.Request.Header.Set("Authorization", "Bearer no-matching-agent-token")
	// findAgentByToken returns error (no agents) → 401
	srv.HandleWebSocket(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestWatchHeartbeat_StaleGoroutine_DoesNotEvictNewSession is a regression test
// for the session race condition: a stale watchHeartbeat goroutine (holding a
// reference to an old, dead session) must not evict or mark offline a newer
// session that has already been stored for the same agent UUID.
func TestWatchHeartbeat_StaleGoroutine_DoesNotEvictNewSession(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	// Short timeout so the ticker fires immediately in the test.
	srv.heartbeatTimeout = time.Millisecond

	const agentUUID = "race-regression-uuid"

	// Insert the agent in the DB with status online so we can verify markOffline
	// is not called.
	agent := &models.OrthrusAgent{
		UUID:   agentUUID,
		Name:   "race-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	// sess1: already closed — represents a stale session whose watchHeartbeat
	// goroutine is still running after a newer session has replaced it.
	conn1, done1 := testWSPair(t)
	defer done1()
	sess1, err := NewAgentSession(agentUUID, "race-agent", conn1)
	require.NoError(t, err)
	require.NoError(t, sess1.Close())
	require.False(t, sess1.IsAlive())

	// sess2: alive — represents the current (newer) reconnect stored in the map.
	conn2, done2 := testWSPair(t)
	defer done2()
	sess2, err := NewAgentSession(agentUUID, "race-agent", conn2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess2.Close() })
	srv.sessions.Store(agentUUID, sess2)

	// Run the stale watchHeartbeat (for sess1) and wait for it to exit.
	// With CompareAndDelete, it finds sess1 ≠ sess2 in the map, so it returns
	// false and skips markOffline — sess2 stays in the map.
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.watchHeartbeat(agentUUID, sess1)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchHeartbeat did not exit within deadline")
	}

	// sess2 must still be present in the map.
	raw, ok := srv.sessions.Load(agentUUID)
	require.True(t, ok, "sess2 should still be in the sessions map")
	assert.Same(t, sess2, raw.(*AgentSession), "sess2 pointer must be unchanged")

	// The agent must NOT have been marked offline.
	var stored models.OrthrusAgent
	require.NoError(t, db.Where("uuid = ?", agentUUID).First(&stored).Error)
	assert.Equal(t, models.OrthrusStatusOnline, stored.Status,
		"stale goroutine must not flip agent status to offline")
}

// TestWatchHeartbeat_CurrentSession_MarksOfflineAndEvictsFromMap exercises the
// CompareAndDelete true-branch: when the session pointer in the map matches the
// goroutine's pointer, the agent is marked offline and the map entry is removed.
func TestWatchHeartbeat_CurrentSession_MarksOfflineAndEvictsFromMap(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = time.Millisecond

	const agentUUID = "current-session-uuid"
	agent := &models.OrthrusAgent{
		UUID:   agentUUID,
		Name:   "current-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	conn, wsCleanup := testWSPair(t)
	defer wsCleanup()

	sess, err := NewAgentSession(agentUUID, "current-agent", conn)
	require.NoError(t, err)
	require.NoError(t, sess.Close())
	require.False(t, sess.IsAlive())

	// Store the SAME pointer so CompareAndDelete returns true.
	srv.sessions.Store(agentUUID, sess)

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		srv.watchHeartbeat(agentUUID, sess)
	}()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("watchHeartbeat did not exit within deadline")
	}

	_, ok := srv.sessions.Load(agentUUID)
	assert.False(t, ok, "session must be evicted when CompareAndDelete succeeds")

	var stored models.OrthrusAgent
	require.NoError(t, db.Where("uuid = ?", agentUUID).First(&stored).Error)
	assert.Equal(t, models.OrthrusStatusOffline, stored.Status,
		"agent must be marked offline when CompareAndDelete succeeds")
}
