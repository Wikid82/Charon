package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupBackupRemoteHandlerTest(t *testing.T, enc *crypto.EncryptionService) (*gin.Engine, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteStorageTarget{}, &models.BackupRecord{}, &models.BackupRemoteCopy{}, &models.Setting{}))

	svc := services.NewBackupRemoteService(db, enc, t.TempDir())
	h := NewBackupRemoteHandler(svc)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	api := router.Group("/api/v1/backups/remote-targets")
	api.GET("", h.List)
	api.POST("", h.Create)
	api.PUT("/:uuid", h.Update)
	api.DELETE("/:uuid", h.Delete)
	api.POST("/:uuid/test", h.Test)
	api.POST("/test-draft", h.TestDraft)

	return router, db
}

// findLocalRFC1918Address returns the first RFC1918 IPv4 address bound to
// any local network interface (e.g. a Docker bridge, or the private
// address most CI runners are assigned on their primary NIC), so tests can
// dial a real local TCP listener through the exact SSRF-allowed path a
// self-hosted NAS on the operator's own LAN would use (spec §3.7 — RFC1918
// is explicitly allowed by remotestorage.ValidateHostSSRF). Loopback cannot
// be used here since it is always rejected. Skips the test if no such
// address is available in the current environment.
func findLocalRFC1918Address(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		if network.IsRFC1918(ip4) {
			return ip4.String()
		}
	}
	t.Skip("no local RFC1918 address available to bind the fake SSH server fixture")
	return ""
}

// startFakeSSHServer starts a minimal local SSH server bound to bindIP,
// presenting a freshly-generated (unpinned/unknown-to-the-caller) host key,
// whose PasswordCallback and PublicKeyCallback both record whether they
// were ever invoked. Mirrors
// remotestorage.startFakeSSHServer (sftp_discovery_test.go) — duplicated
// here rather than shared so this package's tests stay independent of
// remotestorage's internal (unexported) test fixtures, and so this test
// exercises the real HTTP handler end-to-end rather than a service-layer
// fake.
func startFakeSSHServer(t *testing.T, bindIP string) (port int, authAttempted func() bool) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	var mu sync.Mutex
	attempted := false
	markAttempted := func() {
		mu.Lock()
		defer mu.Unlock()
		attempted = true
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, _ []byte) (*ssh.Permissions, error) {
			markAttempted()
			return nil, errors.New("auth rejected by test fixture")
		},
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			markAttempted()
			return nil, errors.New("auth rejected by test fixture")
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", net.JoinHostPort(bindIP, "0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				sshConn, chans, reqs, handshakeErr := ssh.NewServerConn(conn, config)
				if handshakeErr != nil {
					_ = conn.Close()
					return
				}
				defer func() { _ = sshConn.Close() }()
				go ssh.DiscardRequests(reqs)
				for newChan := range chans {
					_ = newChan.Reject(ssh.Prohibited, "test fixture accepts no channels")
				}
			}()
		}
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	return tcpAddr.Port, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return attempted
	}
}

func testEncryptionService(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	enc, err := crypto.NewEncryptionService(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
	return enc
}

func TestBackupRemoteHandler_List_Empty(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "[]", resp.Body.String())
}

func TestBackupRemoteHandler_Create_Success(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"name": "Home NAS",
		"type": "sftp",
		"config": map[string]any{
			"host":                 "10.0.0.5",
			"port":                 22,
			"path":                 "/backups",
			"host_key_fingerprint": "SHA256:abc123",
		},
		"secrets": map[string]any{"password": "hunter2"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "Home NAS", respBody["name"])
	require.Equal(t, true, respBody["secrets_set"])
	require.NotContains(t, resp.Body.String(), "hunter2")
}

func TestBackupRemoteHandler_Create_SSRFRejected(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"name":   "Evil",
		"type":   "sftp",
		"config": map[string]any{"host": "169.254.169.254"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestBackupRemoteHandler_Create_EncryptionKeyMissing(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, nil)

	payload := map[string]any{
		"name":   "Home NAS",
		"type":   "sftp",
		"config": map[string]any{"host": "10.0.0.5", "host_key_fingerprint": "SHA256:abc"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "encryption_key_missing", respBody["error_code"])
}

func TestBackupRemoteHandler_UpdateAndDelete(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	target := models.RemoteStorageTarget{Name: "Old Name", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`}
	require.NoError(t, db.Create(&target).Error)

	updatePayload := map[string]any{
		"name":   "New Name",
		"config": map[string]any{"host": "10.0.0.6", "host_key_fingerprint": "SHA256:xyz"},
	}
	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/remote-targets/"+target.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, "New Name", respBody["name"])

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/backups/remote-targets/"+target.UUID, http.NoBody)
	delResp := httptest.NewRecorder()
	router.ServeHTTP(delResp, delReq)
	require.Equal(t, http.StatusOK, delResp.Code)

	var count int64
	db.Model(&models.RemoteStorageTarget{}).Where("uuid = ?", target.UUID).Count(&count)
	require.Equal(t, int64(0), count)
}

func TestBackupRemoteHandler_Test_EncryptionKeyMissing(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, nil)

	// A target that already has an encrypted secrets blob (e.g. created
	// before CHARON_ENCRYPTION_KEY was removed) cannot be tested without
	// the key.
	target := models.RemoteStorageTarget{Name: "Home NAS", Type: "sftp", ConfigJSON: `{"host":"10.0.0.5"}`, SecretsEncrypted: "ciphertext"}
	require.NoError(t, db.Create(&target).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+target.UUID+"/test", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestBackupRemoteHandler_Test_NotFound(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/does-not-exist/test", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestBackupRemoteHandler_Create_InvalidJSON(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

// TestBackupRemoteHandler_TestDraft_NeverAuthenticates is the required
// handler-level (HTTP layer) counterpart of
// remotestorage.TestDiscoverSFTPHostKey_NeverAuthenticates: POSTing a draft
// SFTP config (no persisted target, no UUID) to /test-draft must discover
// the offered host key fingerprint WITHOUT ever attempting password or
// public-key authentication against the server — proving the guarantee
// holds end-to-end through the real HTTP handler, not just at the
// remotestorage service layer.
func TestBackupRemoteHandler_TestDraft_NeverAuthenticates(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	bindIP := findLocalRFC1918Address(t)
	port, authAttempted := startFakeSSHServer(t, bindIP)

	payload := map[string]any{
		"type":   "sftp",
		"config": map[string]any{"host": bindIP, "port": port, "path": "/backups", "username": "charon"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/test-draft", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	fingerprint, _ := respBody["discovered_fingerprint"].(string)
	require.NotEmpty(t, fingerprint)
	require.Contains(t, fingerprint, "SHA256:")
	require.Equal(t, true, respBody["success"])

	require.False(t, authAttempted(), "no password or public-key auth method may ever be attempted during draft host key discovery")
}

// TestBackupRemoteHandler_TestDraft_HappyPath proves the basic success
// response shape returned to the frontend when discovery succeeds against a
// valid draft config: a 200 with a well-formed discovered_fingerprint,
// again without ever authenticating.
func TestBackupRemoteHandler_TestDraft_HappyPath(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	bindIP := findLocalRFC1918Address(t)
	port, authAttempted := startFakeSSHServer(t, bindIP)

	payload := map[string]any{
		"type":   "sftp",
		"config": map[string]any{"host": bindIP, "port": port},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/test-draft", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var respBody map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &respBody))
	require.Equal(t, true, respBody["success"])
	require.NotEmpty(t, respBody["message"])
	fingerprint, _ := respBody["discovered_fingerprint"].(string)
	require.Contains(t, fingerprint, "SHA256:")
	require.False(t, authAttempted())
}

// TestBackupRemoteHandler_TestDraft_SSRFRejected proves a loopback/link-local
// target in the draft config is rejected before any dial is attempted (spec
// §3.7's SSRF policy applies to /test-draft exactly as it does to every
// other remote-target entry point).
func TestBackupRemoteHandler_TestDraft_SSRFRejected(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	for _, host := range []string{"127.0.0.1", "169.254.169.254"} {
		t.Run(host, func(t *testing.T) {
			payload := map[string]any{
				"type":   "sftp",
				"config": map[string]any{"host": host, "port": 22},
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/test-draft", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		})
	}
}

// TestBackupRemoteHandler_TestDraft_RequiresSFTPType proves the endpoint is
// scoped to SFTP only (S3 has no host-key-pinning discovery step).
func TestBackupRemoteHandler_TestDraft_RequiresSFTPType(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	payload := map[string]any{
		"type":   "s3",
		"config": map[string]any{"host": "10.0.0.5"},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/test-draft", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

// TestBackupRemoteHandler_TestDraft_InvalidJSON proves malformed bodies are
// rejected with 400, matching every other handler in this file.
func TestBackupRemoteHandler_TestDraft_InvalidJSON(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/test-draft", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}
