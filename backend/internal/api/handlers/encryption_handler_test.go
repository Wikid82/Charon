package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// encryptionFixture is a file-backed database with the single-connection pool used in production.
type encryptionFixture struct {
	t       *testing.T
	db      *gorm.DB
	handler *EncryptionHandler
}

func newEncryptionFixture(t *testing.T, keys config.EncryptionKeys) *encryptionFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "encryption.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	// Cleanups run last-in first-out: the audit worker is closed (and drained)
	// before the database it writes to is closed.
	t.Cleanup(func() { _ = sqlDB.Close() })
	security := services.NewSecurityService(db)
	t.Cleanup(security.Close)
	require.NoError(t, db.AutoMigrate(
		&models.EncryptionKeyVersion{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.RemoteStorageTarget{},
		&models.SSLCertificate{},
		&models.TunnelConfig{},
		&models.Setting{},
		&models.SecurityAudit{},
	))

	rotation, err := crypto.NewRotationService(db, keys)
	require.NoError(t, err)
	return &encryptionFixture{
		t:       t,
		db:      db,
		handler: NewEncryptionHandler(rotation, security),
	}
}

// router mounts the handler; admin controls the caller role.
func (f *encryptionFixture) router(admin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if admin {
			c.Set("role", "admin")
			c.Set("userID", uint(1))
		} else {
			c.Set("role", "user")
			c.Set("userID", uint(2))
		}
		c.Next()
	})
	api := router.Group("/api/v1/admin/encryption")
	api.GET("/status", f.handler.GetStatus)
	api.POST("/rotate", f.handler.Rotate)
	api.GET("/history", f.handler.GetHistory)
	api.POST("/validate", f.handler.Validate)
	return router
}

// do sends a request through the router and returns the recorder.
func (f *encryptionFixture) do(admin bool, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, http.NoBody)
	w := httptest.NewRecorder()
	f.router(admin).ServeHTTP(w, req)
	return w
}

// seedProvider stores one DNS provider whose credentials are encrypted with svc.
func (f *encryptionFixture) seedProvider(svc *crypto.EncryptionService) {
	f.t.Helper()
	ct, err := svc.Encrypt([]byte(`{"api_key":"x"}`))
	require.NoError(f.t, err)
	provider := models.DNSProvider{UUID: uuid.NewString(), Name: "p", ProviderType: "cloudflare", CredentialsEncrypted: ct}
	require.NoError(f.t, f.db.Create(&provider).Error)
	require.NoError(f.t, f.db.Model(&provider).UpdateColumn("key_version", 1).Error)
}

// keyB64 returns a fresh random key.
func keyB64(t *testing.T) string {
	t.Helper()
	k, err := crypto.GenerateNewKey()
	require.NoError(t, err)
	return k
}

// service returns the EncryptionService for keyB64.
func service(t *testing.T, k string) *crypto.EncryptionService {
	t.Helper()
	svc, err := crypto.NewEncryptionService(k)
	require.NoError(t, err)
	return svc
}

// failingCallback makes every statement on table fail for the rest of the test.
// The callback is never removed: it lives on the fixture's own database, which
// is closed in cleanup after the audit worker has drained, so removing it could
// race with a write still in flight.
func failingCallback(t *testing.T, db *gorm.DB, table, op string) {
	t.Helper()
	fail := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(gorm.ErrInvalidDB)
		}
	}
	name := "test:fail-" + table + "-" + op
	var err error
	switch op {
	case "update":
		err = db.Callback().Update().Before("gorm:update").Register(name, fail)
	case "create":
		err = db.Callback().Create().Before("gorm:create").Register(name, fail)
	case "row":
		err = db.Callback().Row().Before("gorm:row").Register(name, fail)
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register(name, fail)
	default:
		t.Fatalf("unknown operation %q", op)
	}
	require.NoError(t, err)
}

// bodyError decodes the "error" field of a JSON response.
func bodyError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	msg, _ := body["error"].(string)
	return msg
}

func TestEncryptionHandler_NonAdminIsForbiddenOnEveryEndpoint(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t)})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/encryption/status"},
		{http.MethodPost, "/api/v1/admin/encryption/rotate"},
		{http.MethodGet, "/api/v1/admin/encryption/history"},
		{http.MethodPost, "/api/v1/admin/encryption/validate"},
	} {
		w := f.do(false, tc.method, tc.path)
		assert.Equal(t, http.StatusForbidden, w.Code, tc.path)
	}
}

func TestEncryptionHandler_GetStatusReturnsKeysAndStores(t *testing.T) {
	current := keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current})
	f.seedProvider(service(t, current))

	w := f.do(true, http.MethodGet, "/api/v1/admin/encryption/status")
	require.Equal(t, http.StatusOK, w.Code)
	var status crypto.RotationStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	assert.Equal(t, 1, status.CurrentVersion)
	assert.Len(t, status.Keys, 1)
	assert.True(t, status.Keys[0].Loaded)
	assert.Equal(t, 1, status.ProvidersOnCurrentVersion)
}

func TestEncryptionHandler_GetStatusHidesStorageErrors(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t)})
	failingCallback(t, f.db, "dns_providers", "row")

	w := f.do(true, http.MethodGet, "/api/v1/admin/encryption/status")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Could not read the encryption status.", bodyError(t, w))
	assert.NotContains(t, w.Body.String(), "database")
}

func TestEncryptionHandler_RotateWithoutNextKeyIsConflict(t *testing.T) {
	current := keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current})
	f.seedProvider(service(t, current))

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/rotate")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "Nothing to re-encrypt. Every stored secret already uses the current key; to move to a new key, set CHARON_ENCRYPTION_KEY_NEXT and restart.", bodyError(t, w))
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "rotation_not_needed", body["error_code"])
}

func TestEncryptionHandler_RotateReencryptsToNextKey(t *testing.T) {
	current, next := keyB64(t), keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current, Next: next})
	f.seedProvider(service(t, current))

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/rotate")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result crypto.RotationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 1, result.RotatedCount)
	assert.Equal(t, 1, result.TotalRecords)
	assert.NotEmpty(t, result.Stores)
}

func TestEncryptionHandler_RotateFailureReturnsFixedTextAndChangesNothing(t *testing.T) {
	current, next := keyB64(t), keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current, Next: next})
	f.seedProvider(service(t, current))
	failingCallback(t, f.db, "dns_providers", "update")

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/rotate")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Key rotation failed. No stored secret was changed.", bodyError(t, w))
	assert.NotContains(t, w.Body.String(), "injected")
}

func TestEncryptionHandler_RotateSucceedsWhenAuditWritesFail(t *testing.T) {
	current, next := keyB64(t), keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current, Next: next})
	f.seedProvider(service(t, current))
	failingCallback(t, f.db, "security_audits", "create")

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/rotate")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEncryptionHandler_ValidateReportsFailingSlotNames(t *testing.T) {
	current := keyB64(t)
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: current, Legacy: map[int]string{2: "not-a-key"}})

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/validate")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "One or more encryption keys failed validation.", bodyError(t, w))
	var body struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, []string{"CHARON_ENCRYPTION_KEY_V2"}, body.Errors)
	assert.False(t, strings.Contains(w.Body.String(), "not-a-key"), "key values are never returned")
}

func TestEncryptionHandler_ValidateSucceedsForGoodKeys(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t), Next: keyB64(t)})
	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/validate")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "All encryption keys are valid")
}

func TestEncryptionHandler_HistoryReturnsAuditPageWithDefaults(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t)})

	// Out-of-range paging values fall back to the defaults (page 1, limit 50).
	w := f.do(true, http.MethodGet, "/api/v1/admin/encryption/history?page=bogus&limit=500")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Audits []json.RawMessage `json:"audits"`
		Page   int               `json:"page"`
		Limit  int               `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 50, body.Limit)
}

func TestEncryptionHandler_HistoryHidesReadErrors(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t)})
	failingCallback(t, f.db, "security_audits", "query")

	w := f.do(true, http.MethodGet, "/api/v1/admin/encryption/history")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Could not read the encryption history.", bodyError(t, w))
}

func TestEncryptionHandler_ValidateSucceedsWhenAuditWriteFails(t *testing.T) {
	f := newEncryptionFixture(t, config.EncryptionKeys{Current: keyB64(t)})
	failingCallback(t, f.db, "security_audits", "create")

	w := f.do(true, http.MethodPost, "/api/v1/admin/encryption/validate")
	assert.Equal(t, http.StatusOK, w.Code)
}
