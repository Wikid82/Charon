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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?host=tcp://127.0.0.1:2375", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, dockerSvc.called, "docker service should not be called for invalid host")
}

func TestDockerHandler_ListContainers_DockerUnavailableMappedTo503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{err: services.NewDockerUnavailableError(errors.New("no docker socket"), "Local Docker socket is mounted but not accessible by current process")}
	remoteSvc := &fakeRemoteServerService{}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?host=local", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Docker daemon unavailable")
	// Verify the new details field is included in the response
	assert.Contains(t, w.Body.String(), "details")
	assert.Contains(t, w.Body.String(), "not accessible by current process")
}

func TestDockerHandler_ListContainers_ServerIDResolvesToTCPHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{ret: []services.DockerContainer{}}
	remoteSvc := &fakeRemoteServerService{server: &models.RemoteServer{Host: "example.internal", Port: 2375}}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=abc-123", http.NoBody)
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=missing", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, dockerSvc.called)
}

// Phase 4.1: Additional test cases for complete coverage

func TestDockerHandler_ListContainers_Local(t *testing.T) {
	// Test local/default docker connection (empty host parameter)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{
		ret: []services.DockerContainer{
			{
				ID:      "abc123456789",
				Names:   []string{"test-container"},
				Image:   "nginx:latest",
				State:   "running",
				Status:  "Up 2 hours",
				Network: "bridge",
				IP:      "172.17.0.2",
				Ports: []services.DockerPort{
					{PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
				},
			},
		},
	}
	remoteSvc := &fakeRemoteServerService{}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, dockerSvc.called)
	assert.Empty(t, dockerSvc.host, "local connection should have empty host")
	assert.Contains(t, w.Body.String(), "test-container")
	assert.Contains(t, w.Body.String(), "nginx:latest")
}

func TestDockerHandler_ListContainers_RemoteServerSuccess(t *testing.T) {
	// Test successful remote server connection via server_id
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{
		ret: []services.DockerContainer{
			{
				ID:     "remote123",
				Names:  []string{"remote-nginx"},
				Image:  "nginx:alpine",
				State:  "running",
				Status: "Up 1 day",
			},
		},
	}
	remoteSvc := &fakeRemoteServerService{
		server: &models.RemoteServer{
			UUID: "server-uuid-123",
			Name: "Production Server",
			Host: "192.168.1.100",
			Port: 2376,
		},
	}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=server-uuid-123", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, dockerSvc.called)
	assert.Equal(t, "server-uuid-123", remoteSvc.gotUUID)
	assert.Equal(t, "tcp://192.168.1.100:2376", dockerSvc.host)
	assert.Contains(t, w.Body.String(), "remote-nginx")
}

func TestDockerHandler_ListContainers_RemoteServerNotFound(t *testing.T) {
	// Test server_id that doesn't exist in database
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{}
	remoteSvc := &fakeRemoteServerService{
		err: errors.New("server not found"),
	}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?server_id=nonexistent-uuid", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, dockerSvc.called, "docker service should not be called when server not found")
	assert.Contains(t, w.Body.String(), "Remote server not found")
}

func TestDockerHandler_ListContainers_InvalidHost(t *testing.T) {
	// Test SSRF protection: reject arbitrary host values
	gin.SetMode(gin.TestMode)
	router := gin.New()

	dockerSvc := &fakeDockerService{}
	remoteSvc := &fakeRemoteServerService{}
	h := NewDockerHandler(dockerSvc, remoteSvc)

	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	tests := []struct {
		name      string
		hostParam string
	}{
		{"arbitrary IP", "host=10.0.0.1"},
		{"tcp URL", "host=tcp://evil.com:2375"},
		{"unix socket", "host=unix:///var/run/docker.sock"},
		{"http URL", "host=http://attacker.com/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?"+tt.hostParam, http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, "should reject invalid host: %s", tt.hostParam)
			assert.Contains(t, w.Body.String(), "Invalid docker host selector")
			assert.False(t, dockerSvc.called, "docker service should not be called for invalid host")
		})
	}
}

func TestDockerHandler_ListContainers_DockerUnavailable(t *testing.T) {
	// Test various Docker unavailability scenarios
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "daemon not running",
			err:      services.NewDockerUnavailableError(errors.New("cannot connect to docker daemon")),
			wantCode: http.StatusServiceUnavailable,
			wantMsg:  "Docker daemon unavailable",
		},
		{
			name:     "socket permission denied",
			err:      services.NewDockerUnavailableError(errors.New("permission denied")),
			wantCode: http.StatusServiceUnavailable,
			wantMsg:  "Docker daemon unavailable",
		},
		{
			name:     "socket not found",
			err:      services.NewDockerUnavailableError(errors.New("no such file or directory")),
			wantCode: http.StatusServiceUnavailable,
			wantMsg:  "Docker daemon unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			dockerSvc := &fakeDockerService{err: tt.err}
			remoteSvc := &fakeRemoteServerService{}
			h := NewDockerHandler(dockerSvc, remoteSvc)

			api := router.Group("/api/v1")
			h.RegisterRoutes(api)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers?host=local", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantMsg)
			assert.True(t, dockerSvc.called)
		})
	}
}

func TestDockerHandler_ListContainers_GenericError(t *testing.T) {
	// Test non-connectivity errors (should return 500)
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "API error",
			err:      errors.New("API error: invalid request"),
			wantCode: http.StatusInternalServerError,
			wantMsg:  "Failed to list containers",
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			wantCode: http.StatusInternalServerError,
			wantMsg:  "Failed to list containers",
		},
		{
			name:     "unknown error",
			err:      errors.New("unexpected error occurred"),
			wantCode: http.StatusInternalServerError,
			wantMsg:  "Failed to list containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			dockerSvc := &fakeDockerService{err: tt.err}
			remoteSvc := &fakeRemoteServerService{}
			h := NewDockerHandler(dockerSvc, remoteSvc)

			api := router.Group("/api/v1")
			h.RegisterRoutes(api)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/docker/containers", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantMsg)
			assert.True(t, dockerSvc.called)
		})
	}
}
