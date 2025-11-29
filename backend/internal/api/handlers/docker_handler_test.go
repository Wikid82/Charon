package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDockerTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *services.RemoteServerService) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteServer{}))

	rsService := services.NewRemoteServerService(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	return r, db, rsService
}

func TestDockerHandler_ListContainers(t *testing.T) {
	// We can't easily mock the DockerService without an interface,
	// and the DockerService depends on the real Docker client.
	// So we'll just test that the handler is wired up correctly,
	// even if it returns an error because Docker isn't running in the test env.

	svc, _ := services.NewDockerService()
	// svc might be nil if docker is not available, but NewDockerHandler handles nil?
	// Actually NewDockerHandler just stores it.
	// If svc is nil, ListContainers will panic.
	// So we only run this if svc is not nil.

	if svc == nil {
		t.Skip("Docker not available")
	}

	r, _, rsService := setupDockerTestRouter(t)

	h := NewDockerHandler(svc, rsService)
	h.RegisterRoutes(r.Group("/"))

	req, _ := http.NewRequest("GET", "/docker/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// It might return 200 or 500 depending on if ListContainers succeeds
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}

func TestDockerHandler_ListContainers_NonExistentServerID(t *testing.T) {
	svc, _ := services.NewDockerService()
	if svc == nil {
		t.Skip("Docker not available")
	}

	r, _, rsService := setupDockerTestRouter(t)

	h := NewDockerHandler(svc, rsService)
	h.RegisterRoutes(r.Group("/"))

	// Request with non-existent server_id
	req, _ := http.NewRequest("GET", "/docker/containers?server_id=non-existent-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Remote server not found")
}

func TestDockerHandler_ListContainers_WithServerID(t *testing.T) {
	svc, _ := services.NewDockerService()
	if svc == nil {
		t.Skip("Docker not available")
	}

	r, db, rsService := setupDockerTestRouter(t)

	// Create a remote server
	server := models.RemoteServer{
		UUID:    uuid.New().String(),
		Name:    "Test Docker Server",
		Host:    "docker.example.com",
		Port:    2375,
		Scheme:  "",
		Enabled: true,
	}
	require.NoError(t, db.Create(&server).Error)

	h := NewDockerHandler(svc, rsService)
	h.RegisterRoutes(r.Group("/"))

	// Request with valid server_id (will fail to connect, but shouldn't error on lookup)
	req, _ := http.NewRequest("GET", "/docker/containers?server_id="+server.UUID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should attempt to connect and likely fail with 500 (not 404)
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
	if w.Code == http.StatusInternalServerError {
		assert.Contains(t, w.Body.String(), "Failed to list containers")
	}
}

func TestDockerHandler_ListContainers_WithHostQuery(t *testing.T) {
	svc, _ := services.NewDockerService()
	if svc == nil {
		t.Skip("Docker not available")
	}

	r, _, rsService := setupDockerTestRouter(t)

	h := NewDockerHandler(svc, rsService)
	h.RegisterRoutes(r.Group("/"))

	// Request with custom host parameter
	req, _ := http.NewRequest("GET", "/docker/containers?host=tcp://invalid-host:2375", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should attempt to connect and fail with 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to list containers")
}

func TestDockerHandler_RegisterRoutes(t *testing.T) {
	svc, _ := services.NewDockerService()
	if svc == nil {
		t.Skip("Docker not available")
	}

	r, _, rsService := setupDockerTestRouter(t)

	h := NewDockerHandler(svc, rsService)
	h.RegisterRoutes(r.Group("/"))

	// Verify route is registered
	routes := r.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/docker/containers" && route.Method == "GET" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected /docker/containers GET route to be registered")
}

func TestDockerHandler_NewDockerHandler(t *testing.T) {
	svc, _ := services.NewDockerService()
	if svc == nil {
		t.Skip("Docker not available")
	}

	_, _, rsService := setupDockerTestRouter(t)

	h := NewDockerHandler(svc, rsService)
	assert.NotNil(t, h)
	assert.NotNil(t, h.dockerService)
	assert.NotNil(t, h.remoteServerService)
}
