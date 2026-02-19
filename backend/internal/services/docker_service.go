package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerUnavailableError struct {
	err error
}

func NewDockerUnavailableError(err error) *DockerUnavailableError {
	return &DockerUnavailableError{err: err}
}

func (e *DockerUnavailableError) Error() string {
	if e == nil || e.err == nil {
		return "docker unavailable"
	}
	return fmt.Sprintf("docker unavailable: %v", e.err)
}

func (e *DockerUnavailableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type DockerPort struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
	Type        string `json:"type"`
}

type DockerContainer struct {
	ID      string       `json:"id"`
	Names   []string     `json:"names"`
	Image   string       `json:"image"`
	State   string       `json:"state"`
	Status  string       `json:"status"`
	Network string       `json:"network"`
	IP      string       `json:"ip"`
	Ports   []DockerPort `json:"ports"`
}

type DockerService struct {
	client  *client.Client
	initErr error // Stores initialization error if Docker is unavailable
}

// NewDockerService creates a new Docker service instance.
// If Docker client initialization fails, it returns a stub service that will return
// DockerUnavailableError for all operations. This allows routes to be registered
// and provide helpful error messages to users.
func NewDockerService() *DockerService {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to initialize Docker client - Docker features will be unavailable")
		return &DockerService{
			client:  nil,
			initErr: err,
		}
	}
	return &DockerService{client: cli, initErr: nil}
}

func (s *DockerService) ListContainers(ctx context.Context, host string) ([]DockerContainer, error) {
	// Check if Docker was available during initialization
	if s.initErr != nil {
		return nil, &DockerUnavailableError{err: s.initErr}
	}

	var cli *client.Client
	var err error

	if host == "" || host == "local" {
		cli = s.client
	} else {
		cli, err = client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("failed to create remote client: %w", err)
		}
		defer func() {
			if closeErr := cli.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Warn("failed to close docker client")
			}
		}()
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		if isDockerConnectivityError(err) {
			return nil, &DockerUnavailableError{err: err}
		}
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []DockerContainer
	for _, c := range containers {
		// Get the first network's IP address if available
		networkName := ""
		ipAddress := ""
		if c.NetworkSettings != nil && len(c.NetworkSettings.Networks) > 0 {
			for name, net := range c.NetworkSettings.Networks {
				networkName = name
				ipAddress = net.IPAddress
				break // Just take the first one for now
			}
		}

		// Clean up names (remove leading slash)
		names := make([]string, len(c.Names))
		for i, name := range c.Names {
			names[i] = strings.TrimPrefix(name, "/")
		}

		// Map ports
		var ports []DockerPort
		for _, p := range c.Ports {
			ports = append(ports, DockerPort{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		result = append(result, DockerContainer{
			ID:      c.ID[:12], // Short ID
			Names:   names,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Network: networkName,
			IP:      ipAddress,
			Ports:   ports,
		})
	}

	return result, nil
}

func isDockerConnectivityError(err error) bool {
	if err == nil {
		return false
	}

	// Common high-signal strings from docker client/daemon failures.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "is the docker daemon running") ||
		strings.Contains(msg, "error during connect") {
		return true
	}

	// Context timeouts typically indicate the daemon/socket is unreachable.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Unwrap()
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	// Walk common syscall error wrappers.
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		err = syscallErr.Unwrap()
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		err = opErr.Unwrap()
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ENOENT, syscall.EACCES, syscall.EPERM, syscall.ECONNREFUSED:
			return true
		}
	}

	// os.ErrNotExist covers missing unix socket paths.
	if errors.Is(err, os.ErrNotExist) {
		return true
	}

	return false
}
