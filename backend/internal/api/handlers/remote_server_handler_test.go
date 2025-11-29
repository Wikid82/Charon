package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

func setupRemoteServerTest_New(t *testing.T) (*gin.Engine, *handlers.RemoteServerHandler) {
	t.Helper()
	db := setupTestDB()
	// Ensure RemoteServer table exists
	db.AutoMigrate(&models.RemoteServer{})

	ns := services.NewNotificationService(db)
	handler := handlers.NewRemoteServerHandler(services.NewRemoteServerService(db), ns)

	r := gin.Default()
	api := r.Group("/api/v1")
	servers := api.Group("/remote-servers")
	servers.GET("", handler.List)
	servers.POST("", handler.Create)
	servers.GET("/:uuid", handler.Get)
	servers.PUT("/:uuid", handler.Update)
	servers.DELETE("/:uuid", handler.Delete)
	servers.POST("/test", handler.TestConnectionCustom)
	servers.POST("/:uuid/test", handler.TestConnection)

	return r, handler
}

func TestRemoteServerHandler_TestConnectionCustom(t *testing.T) {
	r, _ := setupRemoteServerTest_New(t)

	// Test with a likely closed port
	payload := map[string]interface{}{
		"host": "127.0.0.1",
		"port": 54321,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/remote-servers/test", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, false, result["reachable"])
	assert.NotEmpty(t, result["error"])
}

func TestRemoteServerHandler_FullCRUD(t *testing.T) {
	r, _ := setupRemoteServerTest_New(t)

	// Create
	rs := models.RemoteServer{
		Name:     "Test Server CRUD",
		Host:     "192.168.1.100",
		Port:     22,
		Provider: "manual",
	}
	body, _ := json.Marshal(rs)
	req, _ := http.NewRequest("POST", "/api/v1/remote-servers", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created models.RemoteServer
	err := json.Unmarshal(w.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.Equal(t, rs.Name, created.Name)
	assert.NotEmpty(t, created.UUID)

	// List
	req, _ = http.NewRequest("GET", "/api/v1/remote-servers", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get
	req, _ = http.NewRequest("GET", "/api/v1/remote-servers/"+created.UUID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Update
	created.Name = "Updated Server CRUD"
	body, _ = json.Marshal(created)
	req, _ = http.NewRequest("PUT", "/api/v1/remote-servers/"+created.UUID, bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete
	req, _ = http.NewRequest("DELETE", "/api/v1/remote-servers/"+created.UUID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Create - Invalid JSON
	req, _ = http.NewRequest("POST", "/api/v1/remote-servers", bytes.NewBuffer([]byte("invalid json")))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Update - Not Found
	req, _ = http.NewRequest("PUT", "/api/v1/remote-servers/non-existent-uuid", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Delete - Not Found
	req, _ = http.NewRequest("DELETE", "/api/v1/remote-servers/non-existent-uuid", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
