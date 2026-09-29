package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

const exportTestPassword = "correct-password"

// exportHarness wires the real AuthMiddleware, RequireRole and AuthService in
// front of CertificateHandler.Export; no auth context is hand-injected.
type exportHarness struct {
	t        *testing.T
	db       *gorm.DB
	auth     *services.AuthService
	handler  *CertificateHandler
	router   *gin.Engine
	certUUID string
}

func newExportHarness(t *testing.T, guard PasswordAttemptGuard, preMiddleware ...gin.HandlerFunc) *exportHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}, &models.Setting{}))

	certSvc := services.NewCertificateService(t.TempDir(), db, newTestEncSvc(t))
	certPEM, keyPEM, err := generateSelfSignedCertPEM()
	require.NoError(t, err)
	info, err := certSvc.UploadCertificate("export-cert", certPEM, keyPEM, "")
	require.NoError(t, err)

	h := NewCertificateHandler(certSvc, nil, nil)
	h.SetDB(db)
	if guard != nil {
		h.SetPasswordAttemptGuard(guard)
	}

	authSvc := services.NewAuthService(db, config.Config{JWTSecret: "test-secret"})
	r := gin.New()
	for _, m := range preMiddleware {
		r.Use(m)
	}
	r.Use(middleware.AuthMiddleware(authSvc))
	r.POST("/api/certificates/:uuid/export", middleware.RequireRole(models.RoleAdmin), h.Export)

	return &exportHarness{t: t, db: db, auth: authSvc, handler: h, router: r, certUUID: info.UUID}
}

func (e *exportHarness) createUser(role models.UserRole) (*models.User, string) {
	e.t.Helper()
	u := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: uuid.NewString() + "@example.com", Role: role, Enabled: true}
	require.NoError(e.t, u.SetPassword(exportTestPassword))
	require.NoError(e.t, e.db.Create(u).Error)
	token, err := e.auth.GenerateToken(u)
	require.NoError(e.t, err)
	return u, token
}

func (e *exportHarness) export(token string, body map[string]any) *httptest.ResponseRecorder {
	e.t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(e.t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+e.certUUID+"/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func keyExport(password string) map[string]any {
	body := map[string]any{"format": "pem", "include_key": true}
	if password != "" {
		body["password"] = password
	}
	return body
}

func TestCertificateHandler_Export_RealAuth_AdminCorrectPassword(t *testing.T) {
	e := newExportHarness(t, nil)
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, keyExport(exportTestPassword))

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Disposition"), "export-cert.pem")
	assert.Contains(t, w.Body.String(), "PRIVATE KEY")
}

func TestCertificateHandler_Export_RealAuth_WrongPassword(t *testing.T) {
	e := newExportHarness(t, nil)
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, keyExport("wrong-password"))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "incorrect password")
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}

func TestCertificateHandler_Export_RealAuth_MissingPassword(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	e := newExportHarness(t, guard)
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, keyExport(""))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "password required")
	assert.Zero(t, guard.calls, "a request with no password must not consume login budget")
}

func TestCertificateHandler_Export_RealAuth_CertOnlyNeedsNoPassword(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	e := newExportHarness(t, guard)
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, map[string]any{"format": "pem", "include_key": false})

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
	assert.Zero(t, guard.calls)
}

func TestCertificateHandler_Export_RealAuth_NonAdminRejected(t *testing.T) {
	for _, role := range []models.UserRole{models.RoleUser, models.RolePassthrough} {
		t.Run(string(role), func(t *testing.T) {
			guard := &fakePasswordGuard{allow: true}
			e := newExportHarness(t, guard)
			_, token := e.createUser(role)

			w := e.export(token, keyExport(exportTestPassword))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
			assert.Zero(t, guard.calls)
		})
	}
}

func TestCertificateHandler_Export_RealAuth_NoOrInvalidToken(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	e := newExportHarness(t, guard)

	assert.Equal(t, http.StatusUnauthorized, e.export("", keyExport(exportTestPassword)).Code)
	assert.Equal(t, http.StatusUnauthorized, e.export("not-a-valid-token", keyExport(exportTestPassword)).Code)
	assert.Zero(t, guard.calls)
}

func TestCertificateHandler_Export_RealAuth_DeletedUserRejectedByMiddleware(t *testing.T) {
	e := newExportHarness(t, nil)
	u, token := e.createUser(models.RoleAdmin)
	require.NoError(t, e.db.Unscoped().Delete(u).Error)

	w := e.export(token, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}

func bypassMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.EmergencyBypassContextKey, true)
		c.Next()
	}
}

func TestCertificateHandler_Export_RealAuth_EmergencyBypassKeyExportRejected(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	e := newExportHarness(t, guard, bypassMiddleware())

	w := e.export("", keyExport(exportTestPassword))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "authenticated user session")
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
	assert.Zero(t, guard.calls, "bypass sessions never reach password verification")
}

func TestCertificateHandler_Export_RealAuth_EmergencyBypassCertOnlyAllowed(t *testing.T) {
	e := newExportHarness(t, nil, bypassMiddleware())

	w := e.export("", map[string]any{"format": "pem", "include_key": false})

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}

func TestCertificateHandler_Export_RealAuth_GuardCallCounts(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	e := newExportHarness(t, guard)
	_, token := e.createUser(models.RoleAdmin)

	assert.Equal(t, http.StatusForbidden, e.export(token, keyExport("wrong-password")).Code)
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, http.StatusOK, e.export(token, keyExport(exportTestPassword)).Code)
	assert.Equal(t, 2, guard.calls)
}

func TestCertificateHandler_Export_RealAuth_GuardDeniesBeforePasswordCheck(t *testing.T) {
	guard := &fakePasswordGuard{allow: false}
	e := newExportHarness(t, guard)
	_, token := e.createUser(models.RoleAdmin)

	w := e.export(token, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, 1, guard.calls)
	assert.NotContains(t, w.Body.String(), "PRIVATE KEY")
}

func TestCertificateHandler_Export_NilDBReturnsInternalError(t *testing.T) {
	e := newExportHarness(t, nil)
	_, token := e.createUser(models.RoleAdmin)
	e.handler.db = nil

	w := e.export(token, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal error")
}

// Handler-only cases: the middleware chain cannot produce these contexts
// (deleted user with valid session, admin role without a user ID), so the
// handler's own defenses are exercised with real key shapes.

func exportWithContext(t *testing.T, h *CertificateHandler, set func(*gin.Context), body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		set(c)
		c.Next()
	})
	r.POST("/api/certificates/:uuid/export", h.Export)
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/certificates/"+uuid.NewString()+"/export", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCertificateHandler_Export_UnknownUserForbidden(t *testing.T) {
	e := newExportHarness(t, nil)

	w := exportWithContext(t, e.handler, func(c *gin.Context) {
		c.Set("role", string(models.RoleAdmin))
		c.Set("userID", uint(9999))
	}, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "user not found")
}

func TestCertificateHandler_Export_UserLookupDBErrorIsInternalError(t *testing.T) {
	e := newExportHarness(t, nil)
	require.NoError(t, e.db.Migrator().DropTable(&models.User{}))

	w := exportWithContext(t, e.handler, func(c *gin.Context) {
		c.Set("role", string(models.RoleAdmin))
		c.Set("userID", uint(1))
	}, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCertificateHandler_Export_MissingUserIDUnauthorized(t *testing.T) {
	e := newExportHarness(t, nil)

	w := exportWithContext(t, e.handler, func(c *gin.Context) {
		c.Set("role", string(models.RoleAdmin))
	}, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCertificateHandler_Export_NonAdminRoleForbiddenInHandler(t *testing.T) {
	e := newExportHarness(t, nil)

	w := exportWithContext(t, e.handler, func(c *gin.Context) {
		c.Set("role", string(models.RoleUser))
		c.Set("userID", uint(1))
	}, keyExport(exportTestPassword))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "admin privileges required")
}
