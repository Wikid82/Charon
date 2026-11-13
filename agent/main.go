// Command orthrus-agent is the Charon Orthrus reverse-proxy agent.
//
// The agent dials out to a Charon server over a secure WebSocket connection and
// multiplexes Docker socket and TCP port-forward streams back using yamux.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/agent/leash"
	"github.com/Wikid82/charon/agent/privdrop"
)

// authKeySource records where the agent key came from. It is logged; the key is not.
type authKeySource string

const (
	authKeySourceFile authKeySource = "file"
	authKeySourceEnv  authKeySource = "env"
	authKeySourceFlag authKeySource = "flag"
)

// resolveAuthKey picks the agent key. fileSet reports whether ORTHRUS_AUTH_KEY_FILE
// was set, and fileKey holds its already-read contents. Setting both the file and
// ORTHRUS_AUTH_KEY is an error; the file otherwise wins over env and flag.
func resolveAuthKey(fileSet bool, fileKey, envKey, flagKey string) (string, authKeySource, error) {
	switch {
	case fileSet && envKey != "":
		return "", "", errors.New("orthrus: set only one of ORTHRUS_AUTH_KEY_FILE and ORTHRUS_AUTH_KEY")
	case fileSet:
		return fileKey, authKeySourceFile, nil
	case envKey != "":
		return envKey, authKeySourceEnv, nil
	case flagKey != "":
		return flagKey, authKeySourceFlag, nil
	default:
		return "", "", errors.New("orthrus: auth key required — set ORTHRUS_AUTH_KEY_FILE, ORTHRUS_AUTH_KEY env var or --auth-key flag")
	}
}

func main() {
	log := logrus.New()

	// Read the key file before dropping root: a root-owned or 0600 secret is only
	// readable at this point. Its path is an env var, so no flag parsing is needed.
	keyFileSet := os.Getenv(authKeyFileEnv) != ""
	var fileKey string
	if keyFileSet {
		k, err := readAuthKeyFile(os.Getenv(authKeyFileEnv))
		if err != nil {
			log.WithError(err).Fatal("orthrus: cannot use the auth key file")
		}
		fileKey = k
	}

	// Drop root before any flag, config, or network work. No-op when not root.
	if err := privdrop.DropPrivileges(log); err != nil {
		log.WithError(err).Fatal("orthrus: privilege drop failed")
	}

	var (
		serverURL  = flag.String("server-url", "", "WebSocket URL of the Charon Orthrus endpoint (wss://...)")
		authKey    = flag.String("auth-key", "", "Orthrus auth key; prefer ORTHRUS_AUTH_KEY_FILE or ORTHRUS_AUTH_KEY")
		agentID    = flag.String("agent-id", "", "UUID of this agent (from provisioning response)")
		dockerSock = flag.String("docker-socket", "/var/run/docker.sock", "Path to Docker socket")
		logLevel   = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orthrus: invalid log level %q\n", *logLevel)
		os.Exit(1)
	}
	log.SetLevel(lvl)

	key, keySource, err := resolveAuthKey(keyFileSet, fileKey, os.Getenv("ORTHRUS_AUTH_KEY"), *authKey)
	if err != nil {
		log.Fatal(err)
	}
	if keySource == authKeySourceFlag {
		log.Warn("orthrus: the auth key flag is visible in process listings; use ORTHRUS_AUTH_KEY_FILE")
	}

	svrURL := os.Getenv("ORTHRUS_SERVER_URL")
	if svrURL == "" {
		svrURL = *serverURL
	}
	if svrURL == "" {
		log.Fatal("orthrus: server URL required — set ORTHRUS_SERVER_URL env var or --server-url flag")
	}

	dockSock := os.Getenv("ORTHRUS_DOCKER_SOCKET")
	if dockSock == "" {
		dockSock = *dockerSock
	}

	if err := validateServerURL(svrURL); err != nil {
		log.WithError(err).Fatal("orthrus: invalid server URL")
	}
	svrURL = normalizeServerURL(svrURL)

	agentName := os.Getenv("ORTHRUS_AGENT_ID")
	if agentName == "" {
		agentName = *agentID
	}
	if agentName == "" {
		agentName = "unknown"
	}

	// NEVER log the auth key value.
	log.WithFields(logrus.Fields{
		"server_url":      svrURL,
		"agent_id":        agentName,
		"docker_socket":   dockSock,
		"uid":             os.Geteuid(),
		"gid":             os.Getegid(),
		"auth_key":        "[REDACTED]",
		"auth_key_source": string(keySource),
	}).Info("orthrus agent starting")

	if err := checkDockerSocket(dockSock); err != nil {
		log.WithError(err).Warn("orthrus: docker socket is not usable; Docker proxy requests will fail")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	l := leash.New(leash.Config{
		ServerURL:  svrURL,
		AuthKey:    key,
		AgentID:    agentName,
		DockerSock: dockSock,
		Log:        log,
	})

	if err := l.Run(ctx); err != nil && err != context.Canceled {
		log.WithError(err).Fatal("orthrus agent exited with error")
	}
	log.Info("orthrus agent stopped")
}

// checkDockerSocket dials the Docker socket once to confirm the effective
// uid/gid can reach it. The socket is mounted read-only and is owned by the
// host docker group, so the agent's PGID must equal that group's GID.
//
// The path is operator-supplied configuration. It must be absolute and must
// name an existing unix socket before the dial is attempted.
func checkDockerSocket(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("docker socket %s must be an absolute path", path)
	}
	clean := filepath.Clean(path)

	info, err := os.Stat(clean) //nolint:gosec // G703: path is operator-supplied unix socket config validated above, not request input
	if err != nil {
		switch {
		case errors.Is(err, os.ErrPermission):
			return dockerSocketPermissionError(clean, err)
		case errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("docker socket %s not found: is it mounted?: %w", clean, err)
		default:
			return fmt.Errorf("stat docker socket %s: %w", clean, err)
		}
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s is not a unix socket", clean)
	}

	conn, err := net.Dial("unix", clean) //nolint:gosec // G704: path is operator-supplied unix socket config validated above, not request input
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return dockerSocketPermissionError(clean, err)
		}
		return fmt.Errorf("dial docker socket %s: %w", clean, err)
	}
	return conn.Close()
}

// dockerSocketPermissionError explains how to fix a uid/gid mismatch with the
// Docker socket's owning group.
func dockerSocketPermissionError(path string, err error) error {
	return fmt.Errorf("docker socket %s not accessible to uid=%d gid=%d: set PGID to the host docker group GID (`getent group docker`): %w",
		path, os.Geteuid(), os.Getegid(), err)
}

const orthrusConnectPath = "/api/v1/ws/orthrus/connect"

// normalizeServerURL ensures the URL has the canonical Orthrus WebSocket path.
// Users often provide just the base URL (e.g. ws://host:port); this function
// appends the connect path when the URL has no path set.
func normalizeServerURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = orthrusConnectPath
		logrus.StandardLogger().WithField("normalized_url", u.String()).
			Warn("orthrus: server URL had no path; appended canonical connect path")
	}
	return u.String()
}

// validateServerURL rejects non-WebSocket schemes. ws:// is permitted for all
// hosts to support deployments where Charon sits behind a TLS-terminating proxy
// and the agent connects over a trusted local/overlay network (e.g. Tailscale).
func validateServerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())

	switch scheme {
	case "wss":
		return nil
	case "ws":
		isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if !isLocalhost {
			logrus.StandardLogger().Warn("orthrus: ws:// connection is unencrypted; consider deploying Charon behind a TLS-terminating proxy")
		}
		return nil
	case "http", "https":
		return fmt.Errorf("expected WebSocket scheme wss:// (got %q); use wss:// instead of %s://", rawURL, scheme)
	default:
		return fmt.Errorf("unsupported scheme %q: expected wss://", u.Scheme)
	}
}
