package orthrus

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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

func TestOrthrusServer_HandleWebSocket_StartsProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	plainKey := "ch_orthrus_proxytest1234"
	hash, err := bcrypt.GenerateFromPassword([]byte(plainKey), bcrypt.MinCost)
	require.NoError(t, err)

	agentUUID := "proxy-test-uuid"
	agent := &models.OrthrusAgent{
		UUID:        agentUUID,
		Name:        "proxy-test-agent",
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}
	require.NoError(t, db.Create(agent).Error)

	r := gin.New()
	r.GET("/ws/orthrus/connect", srv.HandleWebSocket)
	httpSrv := httptest.NewServer(r)
	defer httpSrv.Close()

	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws/orthrus/connect"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+plainKey)

	clientConn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	require.NoError(t, err)
	if resp != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = clientConn.Close() }()

	// Allow server goroutine to process the connection and start the proxy.
	time.Sleep(50 * time.Millisecond)

	addr, ok := srv.GetProxyAddr(agentUUID)
	assert.True(t, ok, "proxy addr should be registered after connection")
	assert.NotEmpty(t, addr)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "addr must be on loopback: %s", addr)
}
