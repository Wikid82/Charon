package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerUnavailableError struct {
	err     error
	details string
}

func NewDockerUnavailableError(err error, details ...string) *DockerUnavailableError {
	detailMsg := ""
	if len(details) > 0 {
		detailMsg = details[0]
	}

	return &DockerUnavailableError{err: err, details: detailMsg}
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

func (e *DockerUnavailableError) Details() string {
	if e == nil {
		return ""
	}
	return e.details
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
	client    *client.Client
	initErr   error // Stores initialization error if Docker is unavailable
	localHost string
}

// NewDockerService creates a new Docker service instance.
// If Docker client initialization fails, it returns a stub service that will return
// DockerUnavailableError for all operations. This allows routes to be registered
// and provide helpful error messages to users.
func NewDockerService() *DockerService {
	envHost := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	localHost := resolveLocalDockerHost()
	if envHost != "" && !strings.HasPrefix(envHost, "unix://") {
		logger.Log().WithFields(map[string]any{"docker_host_env": envHost, "local_host": localHost}).Info("ignoring non-unix DOCKER_HOST for local docker mode")
	}

	cli, err := client.NewClientWithOpts(client.WithHost(localHost), client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to initialize Docker client - Docker features will be unavailable")
		unavailableErr := NewDockerUnavailableError(err, buildLocalDockerUnavailableDetails(err, localHost))
		return &DockerService{
			client:    nil,
			initErr:   unavailableErr,
			localHost: localHost,
		}
	}
	return &DockerService{client: cli, initErr: nil, localHost: localHost}
}

func (s *DockerService) ListContainers(ctx context.Context, host string) ([]DockerContainer, error) {
	// Check if Docker was available during initialization
	if s.initErr != nil {
		var unavailableErr *DockerUnavailableError
		if errors.As(s.initErr, &unavailableErr) {
			return nil, unavailableErr
		}
		return nil, NewDockerUnavailableError(s.initErr, buildLocalDockerUnavailableDetails(s.initErr, s.localHost))
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
			if host == "" || host == "local" {
				return nil, NewDockerUnavailableError(err, buildLocalDockerUnavailableDetails(err, s.localHost))
			}
			return nil, NewDockerUnavailableError(err)
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

func resolveLocalDockerHost() string {
	envHost := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if strings.HasPrefix(envHost, "unix://") {
		socketPath := socketPathFromDockerHost(envHost)
		if socketPath != "" {
			if _, err := os.Stat(socketPath); err == nil {
				return envHost
			}
		}
	}

	defaultSocketPath := "/var/run/docker.sock"
	if _, err := os.Stat(defaultSocketPath); err == nil {
		return "unix:///var/run/docker.sock"
	}

	rootlessSocketPath := fmt.Sprintf("/run/user/%d/docker.sock", os.Getuid())
	if _, err := os.Stat(rootlessSocketPath); err == nil {
		return "unix://" + rootlessSocketPath
	}

	return "unix:///var/run/docker.sock"
}

func socketPathFromDockerHost(host string) string {
	trimmedHost := strings.TrimSpace(host)
	if !strings.HasPrefix(trimmedHost, "unix://") {
		return ""
	}
	return strings.TrimPrefix(trimmedHost, "unix://")
}

func buildLocalDockerUnavailableDetails(err error, localHost string) string {
	socketPath := socketPathFromDockerHost(localHost)
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	uid := os.Getuid()
	gid := os.Getgid()
	groups, _ := os.Getgroups()
	groupsStr := ""
	if len(groups) > 0 {
		groupValues := make([]string, 0, len(groups))
		for _, groupID := range groups {
			groupValues = append(groupValues, strconv.Itoa(groupID))
		}
		groupsStr = strings.Join(groupValues, ",")
	}

	if errno, ok := extractErrno(err); ok {
		switch errno {
		case syscall.ENOENT:
			return fmt.Sprintf("Local Docker socket not found at %s (local host selector uses %s). Mount %s as read-only or read-write.", socketPath, localHost, socketPath)
		case syscall.ECONNREFUSED:
			return fmt.Sprintf("Docker daemon is not accepting connections at %s.", socketPath)
		case syscall.EACCES, syscall.EPERM:
			infoMsg, socketGID := localSocketStatSummary(socketPath)
			permissionHint := ""
			if socketGID >= 0 && !slices.Contains(groups, socketGID) {
				permissionHint = fmt.Sprintf(" Process groups (%s) do not include socket gid %d; run container with matching supplemental group (e.g., --group-add %d).", groupsStr, socketGID, socketGID)
			}
			return fmt.Sprintf("Local Docker socket is mounted but not accessible by current process (uid=%d gid=%d). %s%s", uid, gid, infoMsg, permissionHint)
		}
	}

	if errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("Local Docker socket not found at %s (local host selector uses %s).", socketPath, localHost)
	}

	return fmt.Sprintf("Cannot connect to local Docker via %s. Ensure Docker is running and the mounted socket permissions allow uid=%d gid=%d access.", localHost, uid, gid)
}

func extractErrno(err error) (syscall.Errno, bool) {
	if err == nil {
		return 0, false
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Unwrap()
	}

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
		return errno, true
	}

	return 0, false
}

func localSocketStatSummary(socketPath string) (string, int) {
	info, statErr := os.Stat(socketPath)
	if statErr != nil {
		return fmt.Sprintf("Socket path %s could not be stat'ed: %v.", socketPath, statErr), -1
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fmt.Sprintf("Socket path %s has mode %s.", socketPath, info.Mode().String()), -1
	}

	return fmt.Sprintf("Socket path %s has mode %s owner uid=%d gid=%d.", socketPath, info.Mode().String(), stat.Uid, stat.Gid), int(stat.Gid)
}
