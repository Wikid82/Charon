package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// A disabled remote server is not a valid container source: the request is
// refused before any Docker connection is attempted.
func TestDockerHandler_ListContainers_DisabledRemoteServerRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{}
	remoteSvc := &fakeRemoteServerService{
		server: &models.RemoteServer{
			UUID:    "disabled-server",
			Name:    "Disabled Server",
			Host:    "192.168.1.100",
			Port:    2376,
			Enabled: false,
		},
	}
	h := NewDockerHandler(dockerSvc, remoteSvc)
	h.RegisterRoutes(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=disabled-server", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "disabled")
	assert.False(t, dockerSvc.called, "no Docker connection may be attempted for a disabled server")
}
