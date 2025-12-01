package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))
	return db
}

func TestNotificationTemplateCRUD(t *testing.T) {
	db := setupDB(t)
	svc := services.NewNotificationService(db)
	h := NewNotificationTemplateHandler(svc)

	// Create
	payload := `{"name":"Simple","config":"{\"title\": \"{{.Title}}\"}","template":"custom"}`
	req := httptest.NewRequest("POST", "/", nil)
	req.Body = io.NopCloser(strings.NewReader(payload))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.Create(c)
	require.Equal(t, http.StatusCreated, w.Code)

	// List
	req2 := httptest.NewRequest("GET", "/", nil)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = req2
	h.List(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	var list []models.NotificationTemplate
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &list))
	require.Len(t, list, 1)
}
