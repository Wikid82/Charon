package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDockerService struct {
	called bool
	host   string

	ret []services.DockerContainer
	err error
}

func (f *fakeDockerService) ListContainers(_ context.Context, host string) ([]services.DockerContainer, error) {
	f.called = true
	f.host = host
	return f.ret, f.err
}

type fakeRemoteServerService struct {
	gotUUID string

	server *models.RemoteServer
	err    error
}

func (f *fakeRemoteServerService) GetByUUID(uuidStr string) (*models.RemoteServer, error) {
	f.gotUUID = uuidStr
	return f.server, f.err
}

func TestDockerHandler_ListContainers_InvalidHostRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{}
	remoteSvc := &fakeRemoteServerService{}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?host=tcp://127.0.0.1:2375", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, dockerSvc.called, "docker service should not be called for invalid host")
}

func TestDockerHandler_ListContainers_DockerUnavailableMappedTo503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{err: services.NewDockerUnavailableError(errors.New("no docker socket"))}
	remoteSvc := &fakeRemoteServerService{}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?host=local", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Docker daemon unavailable")
}

func TestDockerHandler_ListContainers_ServerIDResolvesToTCPHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{ret: []services.DockerContainer{}}
	remoteSvc := &fakeRemoteServerService{server: &models.RemoteServer{Host: "example.internal", Port: 2375}}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=abc-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.True(t, dockerSvc.called)
	assert.Equal(t, "abc-123", remoteSvc.gotUUID)
	assert.Equal(t, "tcp://example.internal:2375", dockerSvc.host)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDockerHandler_ListContainers_ServerIDNotFoundReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{}
	remoteSvc := &fakeRemoteServerService{err: errors.New("not found")}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, dockerSvc.called)
}
