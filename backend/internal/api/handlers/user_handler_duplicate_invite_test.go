package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// openDuplicateInviteDB opens a file-backed WAL database with a busy timeout, so concurrent
// invites contend for the write lock the way they do in production.
func openDuplicateInviteDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/duplicate_invite.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Setting{}, &models.SecurityAudit{}))
	return db
}

func postInviteUser(r *gin.Engine, email string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(map[string]string{"email": email})
	req := httptest.NewRequest(http.MethodPost, "/users/invite", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func adminInviteRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(9999))
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)
	return r
}

// TestInviteUser_ConcurrentDuplicateAnswersConflict sends invites for one address at once. Exactly one
// creates the user; the rest answer 409 with no storage text.
func TestInviteUser_ConcurrentDuplicateAnswersConflict(t *testing.T) {
	const concurrency = 8

	db := openDuplicateInviteDB(t)
	r := adminInviteRouter(NewUserHandler(db, nil))

	start := make(chan struct{})
	results := make([]*httptest.ResponseRecorder, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = postInviteUser(r, "duplicate-invite@example.com")
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
	assert.Equal(t, 1, created, "exactly one duplicate invite may create the user")

	var count int64
	require.NoError(t, db.Model(&models.User{}).Where("email = ?", "duplicate-invite@example.com").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestInviteUser_InsertFailureHidesStorageDetail(t *testing.T) {
	db := openDuplicateInviteDB(t)
	r := adminInviteRouter(NewUserHandler(db, nil))
	require.NoError(t, db.Migrator().DropTable(&models.User{}))

	w := postInviteUser(r, "storage-invite@example.com")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, `{"error":"Failed to create user"}`, strings.TrimSpace(w.Body.String()))
}

func TestIsUniqueConstraintError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "sqlite unique index", err: errors.New("UNIQUE constraint failed: users.email"), want: true},
		{name: "postgres style duplicate key", err: errors.New("ERROR: duplicate key value violates unique constraint"), want: true},
		{name: "database locked is not a unique violation", err: errors.New("database is locked"), want: false},
		{name: "table locked is not a unique violation", err: errors.New("database table is locked: users"), want: false},
		{name: "other failure", err: errors.New("no such table: users"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUniqueConstraintError(tt.err))
		})
	}
}

func TestIsSetupConflictError_CoversUniqueAndLockErrors(t *testing.T) {
	assert.True(t, isSetupConflictError(errors.New("UNIQUE constraint failed: users.email")))
	assert.True(t, isSetupConflictError(errors.New("database is locked")))
	assert.True(t, isSetupConflictError(errors.New("database table is locked: users")))
	assert.False(t, isSetupConflictError(errors.New("no such table: users")))
	assert.False(t, isSetupConflictError(nil))
}
