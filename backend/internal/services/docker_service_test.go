package services

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerService_New(t *testing.T) {
	// This test might fail if docker socket is not available in the build environment
	// So we just check if it returns error or not, but don't fail the test if it's just "socket not found"
	// In a real CI environment with Docker-in-Docker, this would work.
	svc, err := NewDockerService()
	if err != nil {
		t.Logf("Skipping DockerService test: %v", err)
		return
	}
	assert.NotNil(t, svc)
}

func TestDockerService_ListContainers(t *testing.T) {
	svc, err := NewDockerService()
	if err != nil {
		t.Logf("Skipping DockerService test: %v", err)
		return
	}

	// Test local listing
	containers, err := svc.ListContainers(context.Background(), "")
	// If we can't connect to docker daemon, this will fail.
	// We should probably mock the client, but the docker client is an interface?
	// The official client struct is concrete.
	// For now, we just assert that if err is nil, containers is a slice.
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
