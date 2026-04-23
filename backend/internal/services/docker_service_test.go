package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerService_New(t *testing.T) {
	// NewDockerService now always returns a service (never nil)
	// If Docker is unavailable, the service will have initErr set
	svc := NewDockerService()
	assert.NotNil(t, svc, "NewDockerService should always return a non-nil service")

	// If Docker is unavailable, the service should have an initErr
	if svc.initErr != nil {
		t.Logf("Docker service initialized but Docker is unavailable: %v", svc.initErr)
	}
}

func TestDockerService_ListContainers(t *testing.T) {
	svc := NewDockerService()
	assert.NotNil(t, svc)

	// Test local listing
	containers, err := svc.ListContainers(context.Background(), "")

	// If service has initErr, it should return DockerUnavailableError
	if svc.initErr != nil {
		var unavailableErr *DockerUnavailableError
		assert.ErrorAs(t, err, &unavailableErr, "Should return DockerUnavailableError when Docker is not available")
		t.Logf("Docker unavailable (expected in some environments): %v", err)
		return
	}

	// If we can connect to docker daemon, this should succeed
	if err == nil {
		assert.IsType(t, []DockerContainer{}, containers)
	}
}

func TestDockerUnavailableError_ErrorMethods(t *testing.T) {
	// Test NewDockerUnavailableError with base error
	baseErr := errors.New("socket not found")
	err := NewDockerUnavailableError(baseErr)

	// Test Error() method
	assert.Contains(t, err.Error(), "docker unavailable")
	assert.Contains(t, err.Error(), "socket not found")

	// Test Unwrap()
	unwrapped := err.Unwrap()
	assert.Equal(t, baseErr, unwrapped)

	// Test Details()
	errWithDetails := NewDockerUnavailableError(baseErr, "socket permission mismatch")
	assert.Equal(t, "socket permission mismatch", errWithDetails.Details())

	// Test nil receiver cases
	var nilErr *DockerUnavailableError
	assert.Equal(t, "docker unavailable", nilErr.Error())
	assert.Nil(t, nilErr.Unwrap())

	// Test nil base error
	nilBaseErr := NewDockerUnavailableError(nil)
	assert.Equal(t, "docker unavailable", nilBaseErr.Error())
	assert.Nil(t, nilBaseErr.Unwrap())
	assert.Equal(t, "", nilBaseErr.Details())
}

func TestIsDockerConnectivityError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"daemon not running", errors.New("cannot connect to the docker daemon"), true},
		{"daemon running check", errors.New("is the docker daemon running"), true},
		{"error during connect", errors.New("error during connect: test"), true},
		{"connection refused", syscall.ECONNREFUSED, true},
		{"no such file", os.ErrNotExist, true},
		{"context timeout", context.DeadlineExceeded, true},
		{"permission denied - EACCES", syscall.EACCES, true},
		{"permission denied - EPERM", syscall.EPERM, true},
		{"no entry - ENOENT", syscall.ENOENT, true},
		{"random error", errors.New("random error"), false},
		{"empty error", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDockerConnectivityError(tt.err)
			assert.Equal(t, tt.expected, result, "isDockerConnectivityError(%v) = %v, want %v", tt.err, result, tt.expected)
		})
	}
}

// ============== Phase 3.1: Additional Docker Service Tests ==============

func TestIsDockerConnectivityError_URLError(t *testing.T) {
	// Test wrapped url.Error
	innerErr := errors.New("connection refused")
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://discord.com/api/webhooks/123/abc",
		Err: innerErr,
	}

	result := isDockerConnectivityError(urlErr)
	// Should unwrap and process the inner error
	assert.False(t, result, "url.Error wrapping non-connectivity error should return false")

	// Test url.Error wrapping ECONNREFUSED
	urlErrWithSyscall := &url.Error{
		Op:  "dial",
		URL: "unix:///var/run/docker.sock",
		Err: syscall.ECONNREFUSED,
	}
	result = isDockerConnectivityError(urlErrWithSyscall)
	assert.True(t, result, "url.Error wrapping ECONNREFUSED should return true")
}

func TestIsDockerConnectivityError_OpError(t *testing.T) {
	// Test wrapped net.OpError
	opErr := &net.OpError{
		Op:  "dial",
		Net: "unix",
		Err: syscall.ENOENT,
	}

	result := isDockerConnectivityError(opErr)
	assert.True(t, result, "net.OpError wrapping ENOENT should return true")
}

func TestIsDockerConnectivityError_SyscallError(t *testing.T) {
	// Test wrapped os.SyscallError
	syscallErr := &os.SyscallError{
		Syscall: "connect",
		Err:     syscall.ECONNREFUSED,
	}

	result := isDockerConnectivityError(syscallErr)
	assert.True(t, result, "os.SyscallError wrapping ECONNREFUSED should return true")
}

// Implement net.Error interface for timeoutError
type timeoutError struct {
	timeout   bool
	temporary bool
}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return e.timeout }
func (e *timeoutError) Temporary() bool { return e.temporary }

func TestIsDockerConnectivityError_NetErrorTimeout(t *testing.T) {
	// Create a mock net.Error with Timeout()
	err := &timeoutError{timeout: true, temporary: true}

	// Wrap it to ensure it implements net.Error
	var netErr net.Error = err

	result := isDockerConnectivityError(netErr)
	assert.True(t, result, "net.Error with Timeout() should return true")
}

func TestResolveLocalDockerHost_IgnoresRemoteTCPEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://docker-proxy:2375")

	host := resolveLocalDockerHost()

	assert.Equal(t, "unix:///var/run/docker.sock", host)
}

func TestResolveLocalDockerHost_UsesExistingUnixSocketFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	socketFile := filepath.Join(tmpDir, "docker.sock")
	require.NoError(t, os.WriteFile(socketFile, []byte(""), 0o600))

	t.Setenv("DOCKER_HOST", "unix://"+socketFile)

	host := resolveLocalDockerHost()

	assert.Equal(t, "unix://"+socketFile, host)
}

func TestBuildLocalDockerUnavailableDetails_PermissionDeniedIncludesGroupHint(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.EACCES}
	details := buildLocalDockerUnavailableDetails(err, "unix:///var/run/docker.sock")

	assert.Contains(t, details, "not accessible")
	assert.Contains(t, details, "uid=")
	assert.Contains(t, details, "gid=")
	assert.NotContains(t, strings.ToLower(details), "token")

	// When docker socket exists with a GID not in process groups, verify both
	// CLI and compose supplemental-group guidance are present.
	if strings.Contains(details, "--group-add") {
		assert.Contains(t, details, "group_add",
			"when supplemental group hint is present, it should include compose group_add syntax")
	}
}

func TestBuildLocalDockerUnavailableDetails_MissingSocket(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.ENOENT}
	host := "unix:///tmp/nonexistent-docker.sock"

	details := buildLocalDockerUnavailableDetails(err, host)

	assert.Contains(t, details, "not found")
	assert.Contains(t, details, "/tmp/nonexistent-docker.sock")
	assert.Contains(t, details, host)
	assert.Contains(t, details, "Mount", "ENOENT path should include mount guidance")
}

func TestBuildLocalDockerUnavailableDetails_PermissionDeniedSocketGIDInGroups(t *testing.T) {
	// Temp file GID = our primary GID (already in process groups) → no group hint
	tmpDir := t.TempDir()
	socketFile := filepath.Join(tmpDir, "docker.sock")
	require.NoError(t, os.WriteFile(socketFile, []byte(""), 0o600))

	host := "unix://" + socketFile
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.EACCES}
	details := buildLocalDockerUnavailableDetails(err, host)

	assert.Contains(t, details, "not accessible")
	assert.Contains(t, details, "uid=")
	assert.NotContains(t, details, "--group-add",
		"group-add hint should not appear when socket GID is already in process groups")
}

func TestBuildLocalDockerUnavailableDetails_PermissionDeniedStatFails(t *testing.T) {
	// EACCES with a socket path that doesn't exist → stat fails
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.EACCES}
	details := buildLocalDockerUnavailableDetails(err, "unix:///tmp/nonexistent-stat-fail.sock")

	assert.Contains(t, details, "not accessible")
	assert.Contains(t, details, "could not be stat")
}

func TestBuildLocalDockerUnavailableDetails_ConnectionRefused(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.ECONNREFUSED}
	details := buildLocalDockerUnavailableDetails(err, "unix:///var/run/docker.sock")

	assert.Contains(t, details, "not accepting connections")
}

func TestBuildLocalDockerUnavailableDetails_GenericError(t *testing.T) {
	err := errors.New("some unknown docker error")
	details := buildLocalDockerUnavailableDetails(err, "unix:///var/run/docker.sock")

	assert.Contains(t, details, "Cannot connect")
	assert.Contains(t, details, "uid=")
	assert.Contains(t, details, "gid=")
}

// ===== Additional coverage for uncovered paths =====

func TestDockerUnavailableError_NilDetails(t *testing.T) {
	var nilErr *DockerUnavailableError
	assert.Equal(t, "", nilErr.Details())
}

func TestExtractErrno_UrlErrorWrapping(t *testing.T) {
	urlErr := &url.Error{Op: "dial", URL: "unix:///var/run/docker.sock", Err: syscall.EACCES}
	errno, ok := extractErrno(urlErr)
	assert.True(t, ok)
	assert.Equal(t, syscall.EACCES, errno)
}

func TestExtractErrno_SyscallError(t *testing.T) {
	scErr := &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}
	errno, ok := extractErrno(scErr)
	assert.True(t, ok)
	assert.Equal(t, syscall.ECONNREFUSED, errno)
}

func TestExtractErrno_NilError(t *testing.T) {
	_, ok := extractErrno(nil)
	assert.False(t, ok)
}

func TestExtractErrno_NonSyscallError(t *testing.T) {
	_, ok := extractErrno(errors.New("some generic error"))
	assert.False(t, ok)
}

func TestExtractErrno_OpErrorWrapping(t *testing.T) {
	opErr := &net.OpError{Op: "dial", Net: "unix", Err: syscall.EPERM}
	errno, ok := extractErrno(opErr)
	assert.True(t, ok)
	assert.Equal(t, syscall.EPERM, errno)
}

func TestExtractErrno_NestedUrlSyscallOpError(t *testing.T) {
	innerErr := &net.OpError{
		Op:  "dial",
		Net: "unix",
		Err: &os.SyscallError{Syscall: "connect", Err: syscall.EACCES},
	}
	urlErr := &url.Error{Op: "Get", URL: "unix:///var/run/docker.sock", Err: innerErr}
	errno, ok := extractErrno(urlErr)
	assert.True(t, ok)
	assert.Equal(t, syscall.EACCES, errno)
}

func TestSocketPathFromDockerHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{"unix socket", "unix:///var/run/docker.sock", "/var/run/docker.sock"},
		{"tcp host", "tcp://192.168.1.1:2375", ""},
		{"empty", "", ""},
		{"whitespace unix", " unix:///tmp/docker.sock ", "/tmp/docker.sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := socketPathFromDockerHost(tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildLocalDockerUnavailableDetails_OsErrNotExist(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", os.ErrNotExist)
	details := buildLocalDockerUnavailableDetails(err, "unix:///var/run/docker.sock")
	assert.Contains(t, details, "not found")
	assert.Contains(t, details, "/var/run/docker.sock")
}

func TestBuildLocalDockerUnavailableDetails_NonUnixHost(t *testing.T) {
	err := errors.New("cannot connect")
	details := buildLocalDockerUnavailableDetails(err, "tcp://192.168.1.1:2375")
	assert.Contains(t, details, "Cannot connect")
	assert.Contains(t, details, "tcp://192.168.1.1:2375")
}

// ===== Container loop coverage via mock Docker HTTP transport =====

// newContainerListClient starts a local HTTP test server that serves
// containerJSON for any request path, then builds a moby client pointing at it.
// This lets unit tests exercise the container-mapping loop without a real daemon.
func newContainerListClient(t *testing.T, containerJSON string) *client.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, containerJSON)
	}))
	t.Cleanup(server.Close)
	cli, err := client.New(
		client.WithHost("tcp://"+server.Listener.Addr().String()),
		client.WithAPIVersion("1.43"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })
	return cli
}

func TestListContainers_ContainerMappingEdgeCases(t *testing.T) {
	// Exercises all container-loop branches introduced by the moby client migration:
	//   • ID > 12 chars  → truncated to first 12 characters
	//   • non-nil network endpoint with a valid IP → IP populated
	//   • ID exactly 12 chars → NOT truncated
	//   • nil NetworkSettings → empty network name and IP
	//   • non-nil NetworkSettings but nil endpoint value → nil-guard fires, no panic
	//   • non-nil endpoint with zero IPAddress → IPAddress.IsValid()==false, IP stays empty
	const containerJSON = `[
		{
			"Id":    "abc123def456789",
			"Names": ["/web"],
			"Image": "nginx:latest",
			"State": "running",
			"Status": "Up 1 second",
			"NetworkSettings": {
				"Networks": {"bridge": {"IPAddress": "172.17.0.2"}}
			}
		},
		{
			"Id":    "abc123def456",
			"Names": ["/exactly12"],
			"Image": "alpine:latest",
			"State": "running",
			"Status": "Up 2 seconds",
			"NetworkSettings": null
		},
		{
			"Id":    "abc",
			"Names": ["/nullendpoint"],
			"Image": "ubuntu:latest",
			"State": "exited",
			"Status": "Exited (0)",
			"NetworkSettings": {"Networks": {"bridge": null}}
		},
		{
			"Id":    "xyz098",
			"Names": ["/noip"],
			"Image": "redis:latest",
			"State": "running",
			"Status": "Up 3 seconds",
			"NetworkSettings": {"Networks": {"host": {}}}
		}
	]`

	svc := &DockerService{
		client:    newContainerListClient(t, containerJSON),
		initErr:   nil,
		localHost: "tcp://localhost:2375",
	}
	containers, err := svc.ListContainers(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, containers, 4)

	// Container 0: ID longer than 12 chars → truncated; valid IP populated.
	assert.Equal(t, "abc123def456", containers[0].ID, "ID longer than 12 chars must be truncated to 12")
	assert.Equal(t, "172.17.0.2", containers[0].IP)
	assert.Equal(t, "bridge", containers[0].Network)
	assert.Equal(t, "running", containers[0].State)
	assert.Equal(t, "web", containers[0].Names[0], "leading slash must be stripped")

	// Container 1: ID exactly 12 chars → not truncated; nil NetworkSettings.
	assert.Equal(t, "abc123def456", containers[1].ID, "12-char ID must not be modified")
	assert.Equal(t, "", containers[1].IP)
	assert.Equal(t, "", containers[1].Network)

	// Container 2: NetworkSettings present but endpoint value is nil → no panic; IP empty.
	assert.Equal(t, "abc", containers[2].ID)
	assert.Equal(t, "bridge", containers[2].Network, "network name must be populated even when endpoint is nil")
	assert.Equal(t, "", containers[2].IP, "IP must be empty when endpoint is nil")

	// Container 3: endpoint present but IPAddress is zero → IsValid()==false, IP empty.
	assert.Equal(t, "xyz098", containers[3].ID)
	assert.Equal(t, "host", containers[3].Network)
	assert.Equal(t, "", containers[3].IP, "IP must be empty when IPAddress is the zero value")
}

func TestNewDockerService_IgnoresNonUnixDockerHost(t *testing.T) {
	// When DOCKER_HOST is set to a non-unix scheme, NewDockerService must
	// emit a log entry and ignore it, using the resolved local host instead.
	t.Setenv("DOCKER_HOST", "tcp://docker-proxy:2375")
	svc := NewDockerService()
	require.NotNil(t, svc)
	assert.NotEqual(t, "tcp://docker-proxy:2375", svc.localHost,
		"localHost must be the resolved socket path, not the non-unix DOCKER_HOST value")
}

func TestBuildLocalDockerUnavailableDetails_EPERMWithStatFail(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "unix", Err: syscall.EPERM}
	details := buildLocalDockerUnavailableDetails(err, "unix:///tmp/nonexistent-eperm.sock")
	assert.Contains(t, details, "not accessible")
	assert.Contains(t, details, "could not be stat")
}

// ===== Patch coverage: docker_service.go error-path tests =====

func TestNewDockerServiceFromLocalHost_ClientInitError(t *testing.T) {
	// A host string without "://" is rejected by moby's ParseHostURL, causing
	// client.New to return an error. This exercises the previously-untested
	// error path in newDockerServiceFromLocalHost.
	svc := newDockerServiceFromLocalHost("no-colon-scheme")
	require.NotNil(t, svc)
	assert.Nil(t, svc.client)
	assert.NotNil(t, svc.initErr)
	var unavailErr *DockerUnavailableError
	assert.ErrorAs(t, svc.initErr, &unavailErr)
}

func TestDockerService_ListContainers_InitErrIsDockerUnavailableError(t *testing.T) {
	// When initErr is already a *DockerUnavailableError, ListContainers must
	// return it directly (without re-wrapping) so the original details are preserved.
	unavailErr := NewDockerUnavailableError(errors.New("socket not found"), "mount the socket")
	svc := &DockerService{
		client:    nil,
		initErr:   unavailErr,
		localHost: "unix:///var/run/docker.sock",
	}
	_, err := svc.ListContainers(context.Background(), "")
	require.Error(t, err)
	var gotUnavail *DockerUnavailableError
	require.ErrorAs(t, err, &gotUnavail)
	assert.Equal(t, unavailErr, gotUnavail, "original DockerUnavailableError must be returned as-is")
}

func TestDockerService_ListContainers_InitErrIsGenericError(t *testing.T) {
	// When initErr is some other error type, ListContainers must wrap it in a
	// DockerUnavailableError before returning.
	genericErr := errors.New("unexpected init failure")
	svc := &DockerService{
		client:    nil,
		initErr:   genericErr,
		localHost: "unix:///var/run/docker.sock",
	}
	_, err := svc.ListContainers(context.Background(), "")
	require.Error(t, err)
	var gotUnavail *DockerUnavailableError
	assert.ErrorAs(t, err, &gotUnavail, "generic initErr must be wrapped in DockerUnavailableError")
}

func TestDockerService_ListContainers_RemoteClientCreationError(t *testing.T) {
	// When the caller provides a remote host string that cannot be parsed by
	// the moby client (no "://" separator), ListContainers must return an error
	// containing "failed to create remote client".
	svc := &DockerService{
		client:    nil,
		initErr:   nil,
		localHost: "unix:///var/run/docker.sock",
	}
	_, err := svc.ListContainers(context.Background(), "no-scheme-host")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create remote client")
}

func TestDockerService_ListContainers_LocalConnectivityError(t *testing.T) {
	// A client pointing to a non-existent unix socket initialises without error
	// (moby client is lazy) but fails at ContainerList time with ENOENT.
	// For an empty/local host, ListContainers must return a DockerUnavailableError.
	cli, err := client.New(client.WithHost("unix:///tmp/charon-unit-test-no-docker.sock"))
	require.NoError(t, err, "client.New must succeed for a non-existent socket path (lazy init)")
	defer func() { _ = cli.Close() }()

	svc := &DockerService{
		client:    cli,
		initErr:   nil,
		localHost: "unix:///tmp/charon-unit-test-no-docker.sock",
	}
	_, err = svc.ListContainers(context.Background(), "")
	require.Error(t, err)
	var unavailErr *DockerUnavailableError
	assert.ErrorAs(t, err, &unavailErr)
}

func TestDockerService_ListContainers_RemoteConnectivityError(t *testing.T) {
	// When a valid unix scheme but non-existent socket is provided as a remote
	// host, ContainerList fails with a connectivity error. For remote hosts,
	// ListContainers must return a DockerUnavailableError (no socket-specific details).
	svc := &DockerService{
		client:    nil,
		initErr:   nil,
		localHost: "unix:///var/run/docker.sock",
	}
	_, err := svc.ListContainers(context.Background(), "unix:///tmp/charon-unit-test-remote-no-docker.sock")
	require.Error(t, err)
	var unavailErr *DockerUnavailableError
	assert.ErrorAs(t, err, &unavailErr)
}
