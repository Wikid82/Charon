package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// challengeScopeEnv wires the challenge handlers behind the real auth
// middleware and real services, with two admin users.
type challengeScopeEnv struct {
	router   *gin.Engine
	db       *gorm.DB
	tokenA   string
	tokenB   string
	userA    uint
	userB    uint
	provider uint
}

func newChallengeScopeEnv(t *testing.T) *challengeScopeEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Setting{}, &models.ManualChallenge{}))

	authSvc := services.NewAuthService(db, config.Config{JWTSecret: "test-secret"})
	mkUser := func(email string) (uint, string) {
		u := &models.User{UUID: uuid.NewString(), APIKey: uuid.NewString(), Email: email, Name: email, Role: models.RoleAdmin, Enabled: true}
		require.NoError(t, u.SetPassword("password123"))
		require.NoError(t, db.Create(u).Error)
		tok, err := authSvc.GenerateToken(u)
		require.NoError(t, err)
		return u.ID, tok
	}
	idA, tokA := mkUser("a@example.com")
	idB, tokB := mkUser("b@example.com")

	providers := new(mockDNSProviderServiceForChallenge)
	providers.On("Get", mock.Anything, uint(1)).Return(&models.DNSProvider{ID: 1, ProviderType: "manual"}, nil)

	handler := NewManualChallengeHandler(services.NewManualChallengeService(db), providers)
	r := gin.New()
	// Mirrors the production wiring: an optional emergency flag upstream of the real auth middleware.
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Emergency") == "1" {
			c.Set(middleware.EmergencyBypassContextKey, true)
		}
		c.Next()
	})
	r.Use(middleware.AuthMiddleware(authSvc))
	handler.RegisterRoutes(r.Group("/api/v1"))

	return &challengeScopeEnv{router: r, db: db, tokenA: tokA, tokenB: tokB, userA: idA, userB: idB, provider: 1}
}

func (e *challengeScopeEnv) do(method, path, token string, body any, extra map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *challengeScopeEnv) seed(t *testing.T, owner uint, fqdn string) string {
	t.Helper()
	ch := &models.ManualChallenge{
		ID: uuid.NewString(), ProviderID: e.provider, UserID: owner, FQDN: fqdn, Value: "txt",
		Status: models.ChallengeStatusPending, ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, e.db.Create(ch).Error)
	return ch.ID
}

func (e *challengeScopeEnv) path(id, suffix string) string {
	return fmt.Sprintf("/api/v1/dns-providers/%d/manual-challenge/%s%s", e.provider, id, suffix)
}

func TestChallengeHandlers_ScopedToCaller(t *testing.T) {
	e := newChallengeScopeEnv(t)
	idA := e.seed(t, e.userA, "_acme-challenge.a.example.com")

	t.Run("owner can read, poll and list", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, e.do(http.MethodGet, e.path(idA, ""), e.tokenA, nil, nil).Code)
		assert.Equal(t, http.StatusOK, e.do(http.MethodGet, e.path(idA, "/poll"), e.tokenA, nil, nil).Code)

		w := e.do(http.MethodGet, fmt.Sprintf("/api/v1/dns-providers/%d/manual-challenges", e.provider), e.tokenA, nil, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), idA)
	})

	t.Run("other user is refused on get", func(t *testing.T) {
		w := e.do(http.MethodGet, e.path(idA, ""), e.tokenB, nil, nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "_acme-challenge.a.example.com")
	})

	t.Run("other user is refused on poll", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, e.path(idA, "/poll"), e.tokenB, nil, nil).Code)
	})

	t.Run("other user is refused on verify", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodPost, e.path(idA, "/verify"), e.tokenB, nil, nil).Code)
	})

	t.Run("other user does not see the challenge in the list", func(t *testing.T) {
		w := e.do(http.MethodGet, fmt.Sprintf("/api/v1/dns-providers/%d/manual-challenges", e.provider), e.tokenB, nil, nil)
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), idA)
	})

	t.Run("other user is refused on delete and the challenge remains", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodDelete, e.path(idA, ""), e.tokenB, nil, nil).Code)
		var n int64
		require.NoError(t, e.db.Model(&models.ManualChallenge{}).Where("id = ?", idA).Count(&n).Error)
		assert.Equal(t, int64(1), n)
	})

	t.Run("owner can delete", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, e.do(http.MethodDelete, e.path(idA, ""), e.tokenA, nil, nil).Code)
	})
}

func TestChallengeHandlers_CreateRecordsCaller(t *testing.T) {
	e := newChallengeScopeEnv(t)

	w := e.do(http.MethodPost, fmt.Sprintf("/api/v1/dns-providers/%d/manual-challenges", e.provider), e.tokenB,
		map[string]string{"fqdn": "_acme-challenge.b.example.com", "token": "tok", "value": "txt"}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var ch models.ManualChallenge
	require.NoError(t, e.db.Where("fqdn = ?", "_acme-challenge.b.example.com").First(&ch).Error)
	assert.Equal(t, e.userB, ch.UserID)

	// The creating user can read it back; the other admin cannot.
	assert.Equal(t, http.StatusOK, e.do(http.MethodGet, e.path(ch.ID, ""), e.tokenB, nil, nil).Code)
	assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, e.path(ch.ID, ""), e.tokenA, nil, nil).Code)
}

func TestChallengeHandlers_RequireSignedInUser(t *testing.T) {
	e := newChallengeScopeEnv(t)
	owned := e.seed(t, e.userA, "_acme-challenge.owned.example.com")
	legacy := e.seed(t, 0, "_acme-challenge.legacy.example.com")
	emergency := map[string]string{"X-Test-Emergency": "1"}
	listPath := fmt.Sprintf("/api/v1/dns-providers/%d/manual-challenges", e.provider)

	t.Run("no credentials", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, e.do(http.MethodGet, e.path(owned, ""), "", nil, nil).Code)
	})

	t.Run("emergency context is refused on every route", func(t *testing.T) {
		for _, id := range []string{owned, legacy} {
			assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, e.path(id, ""), "", nil, emergency).Code)
			assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, e.path(id, "/poll"), "", nil, emergency).Code)
			assert.Equal(t, http.StatusForbidden, e.do(http.MethodPost, e.path(id, "/verify"), "", nil, emergency).Code)
			assert.Equal(t, http.StatusForbidden, e.do(http.MethodDelete, e.path(id, ""), "", nil, emergency).Code)
		}
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, listPath, "", nil, emergency).Code)
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodPost, listPath, "",
			map[string]string{"fqdn": "_acme-challenge.x.example.com", "token": "t", "value": "v"}, emergency).Code)

		var n int64
		require.NoError(t, e.db.Model(&models.ManualChallenge{}).Where("id IN ?", []string{owned, legacy}).Count(&n).Error)
		assert.Equal(t, int64(2), n, "refused requests leave challenges untouched")
	})

	t.Run("signed-in users cannot reach rows without an owner", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodGet, e.path(legacy, ""), e.tokenA, nil, nil).Code)
		assert.Equal(t, http.StatusForbidden, e.do(http.MethodDelete, e.path(legacy, ""), e.tokenB, nil, nil).Code)
	})
}

func TestRequireChallengeCaller_UnexpectedValueType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	c.Set(middleware.UserIDKey, "7")

	_, ok := requireChallengeCaller(c)
	assert.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
