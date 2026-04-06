package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupNotificationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "notification_handler_test.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database")
	}
	_ = db.AutoMigrate(&models.Notification{}, &models.NotificationProvider{})
	return db
}

func TestNotificationHandler_List(t *testing.T) {
	db := setupNotificationTestDB(t)

	// Seed data
	db.Create(&models.Notification{Title: "Test 1", Message: "Msg 1", Read: false})
	db.Create(&models.Notification{Title: "Test 2", Message: "Msg 2", Read: true})

	service := services.NewNotificationService(db, nil)
	handler := handlers.NewNotificationHandler(service)
	router := gin.New()
	router.GET("/notifications", handler.List)

	// Test List All
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/notifications", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var notifications []models.Notification
	err := json.Unmarshal(w.Body.Bytes(), &notifications)
	assert.NoError(t, err)
	assert.Len(t, notifications, 2)

	// Test List Unread
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/notifications?unread=true", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &notifications)
	assert.NoError(t, err)
	assert.Len(t, notifications, 1)
	assert.False(t, notifications[0].Read)
}

func TestNotificationHandler_MarkAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)

	// Seed data
	notif := &models.Notification{Title: "Test 1", Message: "Msg 1", Read: false}
	db.Create(notif)

	service := services.NewNotificationService(db, nil)
	handler := handlers.NewNotificationHandler(service)
	router := gin.New()
	router.POST("/notifications/:id/read", handler.MarkAsRead)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/notifications/"+notif.ID+"/read", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.Notification
	db.First(&updated, "id = ?", notif.ID)
	assert.True(t, updated.Read)
}

func TestNotificationHandler_MarkAllAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)

	// Seed data
	db.Create(&models.Notification{Title: "Test 1", Message: "Msg 1", Read: false})
	db.Create(&models.Notification{Title: "Test 2", Message: "Msg 2", Read: false})

	service := services.NewNotificationService(db, nil)
	handler := handlers.NewNotificationHandler(service)
	router := gin.New()
	router.POST("/notifications/read-all", handler.MarkAllAsRead)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/notifications/read-all", http.NoBody)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var count int64
	db.Model(&models.Notification{}).Where("read = ?", false).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestNotificationHandler_MarkAllAsRead_Error(t *testing.T) {
	db := setupNotificationTestDB(t)
	service := services.NewNotificationService(db, nil)
	handler := handlers.NewNotificationHandler(service)

	r := gin.New()
	r.POST("/notifications/read-all", handler.MarkAllAsRead)

	// Close DB to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	req, _ := http.NewRequest("POST", "/notifications/read-all", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNotificationHandler_DBError(t *testing.T) {
	db := setupNotificationTestDB(t)
	service := services.NewNotificationService(db, nil)
	handler := handlers.NewNotificationHandler(service)

	r := gin.New()
	r.POST("/notifications/:id/read", handler.MarkAsRead)

	// Close DB to force error
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	req, _ := http.NewRequest("POST", "/notifications/1/read", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
