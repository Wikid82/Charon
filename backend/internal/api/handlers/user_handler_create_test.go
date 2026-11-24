package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// adminCreateRouter mounts CreateUser for an admin caller.
func adminCreateRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(9999))
		c.Next()
	})
	r.POST("/users", handler.CreateUser)
	return r
}

func postCreateUser(r *gin.Engine, email string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(map[string]string{"email": email, "name": "Created", "password": "create-password-1"})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCreateUser_ConcurrentDuplicateAnswersConflict creates one address from several requests at
// once. Exactly one succeeds; the rest answer 409 without storage text.
func TestCreateUser_ConcurrentDuplicateAnswersConflict(t *testing.T) {
	const concurrency = 8

	db := openDuplicateInviteDB(t)
	r := adminCreateRouter(NewUserHandler(db, nil))

	start := make(chan struct{})
	results := make([]*httptest.ResponseRecorder, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = postCreateUser(r, "duplicate-create@example.com")
		}(i)
	}
	close(start)
	wg.Wait()

	created := 0
	for i, w := range results {
		switch w.Code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			assert.Contains(t, w.Body.String(), "Email already in use", "request %d", i)
			assert.NotContains(t, strings.ToLower(w.Body.String()), "constraint", "request %d leaked storage text", i)
		default:
			t.Fatalf("request %d: unexpected status %d: %s", i, w.Code, w.Body.String())
		}
	}
	assert.Equal(t, 1, created, "exactly one create may succeed")

	var count int64
	require.NoError(t, db.Model(&models.User{}).Where("email = ?", "duplicate-create@example.com").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// TestCreateUser_InsertFailureHidesStorageDetail makes the insert fail after the email check passed.
func TestCreateUser_InsertFailureHidesStorageDetail(t *testing.T) {
	db := openDuplicateInviteDB(t)
	r := adminCreateRouter(NewUserHandler(db, nil))
	require.NoError(t, db.Exec(
		"CREATE TRIGGER block_user_insert BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'write blocked for test'); END",
	).Error)

	w := postCreateUser(r, "blocked-create@example.com")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, `{"error":"Failed to create user"}`, strings.TrimSpace(w.Body.String()))
	assert.NotContains(t, w.Body.String(), "write blocked")
}
