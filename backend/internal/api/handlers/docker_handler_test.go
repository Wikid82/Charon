package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

	// Setup DB for RemoteServerService
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RemoteServer{}))

	rsService := services.NewRemoteServerService(db)

	h := NewDockerHandler(svc, rsService)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.RegisterRoutes(r.Group("/"))

	req, _ := http.NewRequest("GET", "/docker/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// It might return 200 or 500 depending on if ListContainers succeeds
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.Code)
}
