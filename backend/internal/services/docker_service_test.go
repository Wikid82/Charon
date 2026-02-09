package services

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
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

	// Test nil receiver cases
	var nilErr *DockerUnavailableError
	assert.Equal(t, "docker unavailable", nilErr.Error())
	assert.Nil(t, nilErr.Unwrap())

	// Test nil base error
	nilBaseErr := NewDockerUnavailableError(nil)
	assert.Equal(t, "docker unavailable", nilBaseErr.Error())
	assert.Nil(t, nilBaseErr.Unwrap())
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
		URL: "http://example.com",
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
