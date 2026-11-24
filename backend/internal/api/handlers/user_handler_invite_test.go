package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// openInviteHTTPDB opens a file-backed WAL database with a busy timeout, as production runs against
// SQLite with concurrent writers.
func openInviteHTTPDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/invite_http.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.SecurityAudit{}))
	return db
}

// newInviteHTTPRouter mounts the public invite routes on a gin engine.
func newInviteHTTPRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewUserHandler(db, nil)
	r := gin.New()
	r.GET("/invite/validate", h.ValidateInvite)
	r.POST("/invite/accept", h.AcceptInvite)
	return r
}

// seedInviteHTTPUser stores an invitee with the given token and expiry (nil expiry allowed).
func seedInviteHTTPUser(t *testing.T, db *gorm.DB, email, token string, expires *time.Time) *models.User {
	t.Helper()
	user := models.User{
		UUID:          uuid.New().String(),
		APIKey:        uuid.New().String(),
		Email:         email,
		Role:          models.RoleUser,
		InviteToken:   token,
		InviteExpires: expires,
		InviteStatus:  models.InviteStatusPending,
	}
	require.NoError(t, services.CreateKeepingSent(db, &user, []string{"enabled"}))
	return &user
}

func futureExpiry() *time.Time {
	t := time.Now().Add(time.Hour)
	return &t
}

func pastExpiry() *time.Time {
	t := time.Now().Add(-time.Minute)
	return &t
}

func postInviteAccept(t *testing.T, r *gin.Engine, body map[string]string, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/invite/accept", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	msg, _ := body["error"].(string)
	return msg
}

// TestAcceptInvite_ConcurrentRequestsOneSucceeds sends several accepts with one token at once and
// asserts exactly one succeeds, the stored password is the winner's, and one audit entry exists.
func TestAcceptInvite_ConcurrentRequestsOneSucceeds(t *testing.T) {
	const (
		concurrency = 8
		rounds      = 3
	)

	for round := 0; round < rounds; round++ {
		db := openInviteHTTPDB(t)
		r := newInviteHTTPRouter(db)
		token := fmt.Sprintf("handler-race-token-%d", round)
		invitee := seedInviteHTTPUser(t, db, fmt.Sprintf("handler-race-%d@example.com", round), token, futureExpiry())

		type result struct {
			index int
			code  int
			body  string
		}
		results := make([]result, concurrency)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				w := postInviteAccept(t, r, map[string]string{
					"token":    token,
					"name":     fmt.Sprintf("Racer %d", i),
					"password": fmt.Sprintf("handler-password-%02d", i),
				}, fmt.Sprintf("198.51.100.%d:1000", i+1))
				results[i] = result{index: i, code: w.Code, body: w.Body.String()}
			}(i)
		}
		close(start)
		wg.Wait()

		var winners []int
		for _, res := range results {
			switch res.code {
			case http.StatusOK:
				winners = append(winners, res.index)
			case http.StatusNotFound, http.StatusConflict:
			default:
				t.Fatalf("round %d: unexpected status %d for racer %d: %s", round, res.code, res.index, res.body)
			}
		}
		require.Len(t, winners, 1, "round %d: exactly one accept must succeed", round)

		var stored models.User
		require.NoError(t, db.First(&stored, invitee.ID).Error)
		assert.Equal(t, fmt.Sprintf("Racer %d", winners[0]), stored.Name)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(fmt.Sprintf("handler-password-%02d", winners[0]))))
		assert.Empty(t, stored.InviteToken)
		assert.Equal(t, models.InviteStatusAccepted, stored.InviteStatus)

		var audits int64
		require.NoError(t, db.Model(&models.SecurityAudit{}).Where("action = ?", "user_invite_accept").Count(&audits).Error)
		assert.Equal(t, int64(1), audits, "round %d: one accept audit entry for the winner", round)
	}
}

func TestAcceptInvite_ReplayIsNotFound(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	seedInviteHTTPUser(t, db, "replay@example.com", "handler-replay", futureExpiry())

	first := postInviteAccept(t, r, map[string]string{"token": "handler-replay", "name": "First", "password": "first-password"}, "192.0.2.10:1")
	require.Equal(t, http.StatusOK, first.Code)

	second := postInviteAccept(t, r, map[string]string{"token": "handler-replay", "name": "Second", "password": "second-password"}, "192.0.2.10:1")
	assert.Equal(t, http.StatusNotFound, second.Code)
	assert.Equal(t, "Invalid or expired invite token", errorBody(t, second))
}

func TestAcceptInvite_ExpiredReturnsGoneAndMarks(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	invitee := seedInviteHTTPUser(t, db, "gone@example.com", "handler-gone", pastExpiry())

	w := postInviteAccept(t, r, map[string]string{"token": "handler-gone", "name": "Gone", "password": "gone-password"}, "192.0.2.11:1")
	assert.Equal(t, http.StatusGone, w.Code)
	assert.Equal(t, "Invite token has expired", errorBody(t, w))

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.Equal(t, models.InviteStatusExpired, stored.InviteStatus)
}

func TestAcceptInvite_NilExpiryReturnsGone(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	seedInviteHTTPUser(t, db, "unset@example.com", "handler-unset", nil)

	w := postInviteAccept(t, r, map[string]string{"token": "handler-unset", "name": "Unset", "password": "unset-password"}, "192.0.2.12:1")
	assert.Equal(t, http.StatusGone, w.Code)
}

func TestAcceptInvite_AccountWithPasswordReturnsConflict(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	invitee := seedInviteHTTPUser(t, db, "claimed@example.com", "handler-claimed", futureExpiry())
	require.NoError(t, db.Model(invitee).Update("password_hash", "admin-set-hash").Error)

	w := postInviteAccept(t, r, map[string]string{"token": "handler-claimed", "name": "Claimed", "password": "claimed-password"}, "192.0.2.13:1")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "Invite has already been accepted", errorBody(t, w))

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.Equal(t, "admin-set-hash", stored.PasswordHash)
}

func TestAcceptInvite_LegacyEnabledInviteAccepts(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	invitee := seedInviteHTTPUser(t, db, "legacyhttp@example.com", "handler-legacy", futureExpiry())
	require.NoError(t, db.Model(invitee).Update("enabled", true).Error)

	w := postInviteAccept(t, r, map[string]string{"token": "handler-legacy", "name": "Legacy", "password": "legacy-password"}, "192.0.2.14:1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAcceptInvite_PasswordOverLimitReportsSetPasswordFailure(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	seedInviteHTTPUser(t, db, "longpw@example.com", "handler-longpw", futureExpiry())

	// bcrypt refuses more than 72 bytes; the response text is unchanged from before the service split.
	w := postInviteAccept(t, r, map[string]string{"token": "handler-longpw", "name": "Long", "password": strings.Repeat("p", 73)}, "192.0.2.15:1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Failed to set password", errorBody(t, w))
}

func TestAcceptInvite_StorageFailureHidesDetail(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	require.NoError(t, db.Migrator().DropTable(&models.User{}))

	w := postInviteAccept(t, r, map[string]string{"token": "handler-storage", "name": "Storage", "password": "storage-password"}, "192.0.2.16:1")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Failed to accept invite", errorBody(t, w))
	assert.NotContains(t, w.Body.String(), "no such table")
}

func TestValidateInvite_ReportsStateParity(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)

	seedInviteHTTPUser(t, db, "valid@example.com", "validate-open", futureExpiry())
	seedInviteHTTPUser(t, db, "lapsed@example.com", "validate-lapsed", pastExpiry())
	seedInviteHTTPUser(t, db, "unset@example.com", "validate-unset", nil)
	claimed := seedInviteHTTPUser(t, db, "claimed-v@example.com", "validate-claimed", futureExpiry())
	require.NoError(t, db.Model(claimed).Update("password_hash", "hash").Error)

	tests := []struct {
		name   string
		token  string
		code   int
		errMsg string
	}{
		{name: "unknown token", token: "validate-none", code: http.StatusNotFound, errMsg: "Invalid or expired invite token"},
		{name: "lapsed link", token: "validate-lapsed", code: http.StatusGone, errMsg: "Invite token has expired"},
		{name: "unset expiry", token: "validate-unset", code: http.StatusGone, errMsg: "Invite token has expired"},
		{name: "password already set", token: "validate-claimed", code: http.StatusConflict, errMsg: "Invite has already been accepted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/invite/validate?token="+tt.token, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.code, w.Code)
			assert.Equal(t, tt.errMsg, errorBody(t, w))
		})
	}

	t.Run("open link returns the e-mail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/invite/validate?token=validate-open", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"valid":true`)
		assert.Contains(t, w.Body.String(), `"email":"valid@example.com"`)
	})
}

func TestValidateInvite_StorageFailureReturnsServerError(t *testing.T) {
	db := openInviteHTTPDB(t)
	r := newInviteHTTPRouter(db)
	require.NoError(t, db.Migrator().DropTable(&models.User{}))

	req := httptest.NewRequest(http.MethodGet, "/invite/validate?token=anything", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Failed to validate invite token", errorBody(t, w))
}
