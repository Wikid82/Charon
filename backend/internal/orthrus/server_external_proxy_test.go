package orthrus

import (
	"fmt"
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

// U-SRV-01: HandleWebSocket with ExternalProxyPort > 0 starts the external proxy;
// GetExternalProxyStatus returns Active=true with the configured port.
func TestOrthrusServer_HandleWebSocket_StartsExternalProxy(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	srv.heartbeatTimeout = 50 * time.Millisecond

	port := findFreePort(t)

	token := "ch_orthrus_ext_srv01" //nolint:gosec // G101: test credential
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	require.NoError(t, err)

	agent := &models.OrthrusAgent{
		UUID:              "ext-srv01-uuid",
		Name:              "ext-srv01-agent",
		AuthKeyHash:       string(hash),
		Status:            models.OrthrusStatusPending,
		ExternalProxyPort: port,
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

	assert.Eventually(t, func() bool {
		status, ok := srv.GetExternalProxyStatus("ext-srv01-uuid")
		return ok && status.Active
	}, 2*time.Second, 20*time.Millisecond, "external proxy should be active after WS connect")

	status, ok := srv.GetExternalProxyStatus("ext-srv01-uuid")
	require.True(t, ok)
	assert.Equal(t, port, status.ConfiguredPort)
	assert.Equal(t, port, status.ActivePort)
}

// U-SRV-02: DisconnectAgent with active external proxy closes the http.Server;
// TCP dial to the external port is refused after disconnect.
func TestOrthrusServer_DisconnectAgent_ClosesExternalProxy(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)
	t.Cleanup(srv.Stop)

	serverConn, wsCleanup := testWSPair(t)
	defer wsCleanup()

	sess, err := NewAgentSession("ext-srv02-uuid", "ext-srv02-agent", serverConn)
	require.NoError(t, err)
	require.NoError(t, sess.StartDockerProxy())

	port := findFreePort(t)
	require.NoError(t, sess.StartExternalProxy(port))

	assert.Eventually(t, func() bool {
		return sess.GetExternalProxyStatus().Active
	}, 2*time.Second, 10*time.Millisecond)

	// Verify external port is reachable before disconnect.
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	require.NoError(t, err)
	_ = c.Close()

	agent := &models.OrthrusAgent{
		UUID:   "ext-srv02-uuid",
		Name:   "ext-srv02-agent",
		Status: models.OrthrusStatusOnline,
	}
	require.NoError(t, db.Create(agent).Error)

	srv.sessions.Store("ext-srv02-uuid", sess)
	require.NoError(t, srv.DisconnectAgent("ext-srv02-uuid"))

	_, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	assert.Error(t, dialErr, "external proxy port should be closed after DisconnectAgent")
}
