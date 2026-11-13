package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"sync"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testSetupCode is the setup code the handler tests install on their handler.
const testSetupCode = "test-setup-code-0123456789abcdefghij"

func setupBody(code string, overrides map[string]string) []byte {
	body := map[string]string{
		"name":       "Admin",
		"email":      "admin@example.com",
		"password":   "password123",
		"setup_code": code,
	}
	for k, v := range overrides {
		body[k] = v
	}
	out, _ := json.Marshal(body)
	return out
}

func postSetupFrom(r http.Handler, remoteAddr string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newSetupRouter(handler *UserHandler) *gin.Engine {
	r := gin.New()
	r.GET("/setup", handler.GetSetupStatus)
	r.POST("/setup", handler.Setup)
	return r
}

func TestUserHandler_Setup_MissingSetupCodeIsForbidden(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	w := postSetupFrom(r, "192.0.2.10:1000", setupBody("", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"error":"Invalid setup code","error_code":"setup_code_invalid"}`, w.Body.String())
	assert.Equal(t, int64(0), countUsers(t, db))
}

func TestUserHandler_Setup_WrongSetupCodeIsForbiddenWithoutEcho(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)
	wrong := "wrong-setup-code-0123456789abcdefghijk"

	w := postSetupFrom(r, "192.0.2.11:1000", setupBody(wrong, nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"error":"Invalid setup code","error_code":"setup_code_invalid"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), wrong)
	assert.NotContains(t, w.Body.String(), testSetupCode)
	assert.Equal(t, int64(0), countUsers(t, db))
}

func TestUserHandler_Setup_NoActiveGuardRejectsEverySubmission(t *testing.T) {
	handler, db := setupUserHandler(t)
	handler.SetSetupCodeGuard(nil)
	r := newSetupRouter(handler)

	w := postSetupFrom(r, "192.0.2.12:1000", setupBody(testSetupCode, nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, int64(0), countUsers(t, db))
}

func TestUserHandler_Setup_CorrectCodeCreatesAdminAndDiscardsCode(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	w := postSetupFrom(r, "192.0.2.13:1000", setupBody(testSetupCode, nil))
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, int64(1), countUsers(t, db))
	assert.False(t, handler.setupCode.Active(), "the code must be discarded after a successful setup")

	// A second attempt, with the same code, is refused by the single-shot guard.
	again := postSetupFrom(r, "192.0.2.14:1000", setupBody(testSetupCode, map[string]string{"email": "other@example.com"}))
	assert.Equal(t, http.StatusForbidden, again.Code)
	assert.Equal(t, int64(1), countUsers(t, db))
}

func TestUserHandler_Setup_FailedSetupKeepsCodeActive(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	// A short password fails binding before any account is created.
	bad := postSetupFrom(r, "192.0.2.15:1000", setupBody(testSetupCode, map[string]string{"password": "short"}))
	require.Equal(t, http.StatusBadRequest, bad.Code)
	assert.True(t, handler.setupCode.Active())
	assert.Equal(t, int64(0), countUsers(t, db))

	good := postSetupFrom(r, "192.0.2.15:1000", setupBody(testSetupCode, nil))
	assert.Equal(t, http.StatusCreated, good.Code)
}

func TestUserHandler_Setup_ThrottlesPerClientBeforeCodeCheck(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)
	wrong := setupBody("wrong-setup-code-0123456789abcdefghijk", nil)

	for i := 0; i < 5; i++ {
		w := postSetupFrom(r, "203.0.113.20:1000", wrong)
		require.Equal(t, http.StatusForbidden, w.Code, "attempt %d within the allowance", i+1)
	}

	// The sixth attempt is throttled even though its code is correct.
	throttled := postSetupFrom(r, "203.0.113.20:1000", setupBody(testSetupCode, nil))
	assert.Equal(t, http.StatusTooManyRequests, throttled.Code)
	assert.NotEmpty(t, throttled.Header().Get("Retry-After"))
	assert.Contains(t, throttled.Body.String(), ratelimit.TooManyRequestsMessage)
	assert.Equal(t, int64(0), countUsers(t, db))

	// Another client has its own allowance.
	other := postSetupFrom(r, "203.0.113.21:1000", setupBody(testSetupCode, nil))
	assert.Equal(t, http.StatusCreated, other.Code)
}

func TestUserHandler_Setup_ConcurrentCorrectCodesCreateOneAdmin(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	const concurrency = 6
	statuses := make(chan int, concurrency)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Distinct client addresses keep the per-client throttle out of this race.
			w := postSetupFrom(r, fmt.Sprintf("198.51.100.%d:1000", i+1), setupBody(testSetupCode,
				map[string]string{"email": fmt.Sprintf("admin%d@example.com", i)}))
			statuses <- w.Code
		}(i)
	}
	close(start)
	wg.Wait()
	close(statuses)

	created := 0
	for status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusForbidden:
		default:
			t.Fatalf("unexpected status %d", status)
		}
	}
	assert.Equal(t, 1, created)
	assert.Equal(t, int64(1), countUsers(t, db))
}

func TestUserHandler_GetSetupStatus_ReportsSetupCodeRequirement(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	read := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/setup", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body
	}

	status := read()
	assert.Equal(t, true, status["setupRequired"])
	assert.Equal(t, true, status["setup_code_required"])

	handler.SetSetupCodeGuard(services.NewSetupCodeGuard(""))
	status = read()
	assert.Equal(t, true, status["setupRequired"])
	assert.Equal(t, false, status["setup_code_required"])

	require.NoError(t, db.Create(&models.User{Email: "x@example.com", Name: "X", Enabled: true}).Error)
	handler.SetSetupCodeGuard(services.NewSetupCodeGuard(testSetupCode))
	status = read()
	assert.Equal(t, false, status["setupRequired"])
	assert.Equal(t, false, status["setup_code_required"])
}

func countUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.User{}).Count(&n).Error)
	return n
}

func TestUserHandler_Setup_SurroundingWhitespaceInCodeIsIgnored(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	w := postSetupFrom(r, "192.0.2.40:1000", setupBody("  "+testSetupCode+"\r\n", nil))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, int64(1), countUsers(t, db))
}

func TestUserHandler_Setup_WhitespaceOnlyCodeIsForbidden(t *testing.T) {
	handler, db := setupUserHandler(t)
	r := newSetupRouter(handler)

	w := postSetupFrom(r, "192.0.2.41:1000", setupBody(" \n\t ", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, int64(0), countUsers(t, db))
}
