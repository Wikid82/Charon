package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestRegister_GeoIPDatabaseRoutesRefuseCrossOriginCookieRequests checks the two mutating GeoIP database routes: an admin
// cookie session from another origin is refused before the handler runs, while header-authenticated and same-origin
// requests reach it. The requests that reach the handler never touch the data folder, so the test cannot change a
// database on the host.
func TestRegister_GeoIPDatabaseRoutesRefuseCrossOriginCookieRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	cfg := config.Config{JWTSecret: "test-secret"}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	authSvc := services.NewAuthService(db, cfg)
	adminAcct := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: "admin-geoip-origin@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, db.Create(adminAcct).Error)
	adminToken, err := authSvc.GenerateToken(adminAcct)
	require.NoError(t, err)
	userAcct := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: "user-geoip-origin@example.com", Role: models.RoleUser, Enabled: true}
	require.NoError(t, db.Create(userAcct).Error)
	userToken, err := authSvc.GenerateToken(userAcct)
	require.NoError(t, err)

	const path = "/api/v1/security/geoip/database"
	send := func(method, token string, headers map[string]string, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: token, HttpOnly: true, Secure: true})
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	crossSite := map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}
	sameSite := map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://app.charon.test"}

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method+" from another origin is refused", func(t *testing.T) {
			w := send(method, adminToken, sameSite, "")

			assert.Equal(t, http.StatusForbidden, w.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, "cross_origin_refused", body["error_code"])
		})
	}

	t.Run("POST from the same origin reaches the handler", func(t *testing.T) {
		w := send(http.MethodPost, adminToken, map[string]string{"Sec-Fetch-Site": "same-origin"}, "not multipart")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "geoip_file_required")
	})
	t.Run("header-authenticated cross-site request is not refused by the origin check", func(t *testing.T) {
		headers := map[string]string{"Authorization": "Bearer " + adminToken, "Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}
		w := send(http.MethodPost, "", headers, "not multipart")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "geoip_file_required")
	})
	t.Run("a non-admin cookie session is still refused by the role gate", func(t *testing.T) {
		w := send(http.MethodPost, userToken, map[string]string{"Sec-Fetch-Site": "same-origin"}, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
	t.Run("a cross-site delete with an admin cookie is refused", func(t *testing.T) {
		w := send(http.MethodDelete, adminToken, crossSite, "")

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "cross_origin_refused")
	})
}
