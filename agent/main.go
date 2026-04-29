// Command orthrus-agent is the Charon Orthrus reverse-proxy agent.
//
// The agent dials out to a Charon server over a secure WebSocket connection and
// multiplexes Docker socket and TCP port-forward streams back using yamux.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/agent/leash"
)

func main() {
	var (
		serverURL  = flag.String("server-url", "", "WebSocket URL of the Charon Orthrus endpoint (wss://...)")
		authKey    = flag.String("auth-key", "", "Orthrus auth key; env ORTHRUS_AUTH_KEY takes precedence")
		agentID    = flag.String("agent-id", "", "UUID of this agent (from provisioning response)")
		dockerSock = flag.String("docker-socket", "/var/run/docker.sock", "Path to Docker socket")
		logLevel   = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	)
	flag.Parse()

	log := logrus.New()
	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orthrus: invalid log level %q\n", *logLevel)
		os.Exit(1)
	}
	log.SetLevel(lvl)

	// Environment variables take precedence over flags for sensitive values.
	key := os.Getenv("ORTHRUS_AUTH_KEY")
	if key == "" {
		key = *authKey
	}
	if key == "" {
		log.Fatal("orthrus: auth key required — set ORTHRUS_AUTH_KEY env var or --auth-key flag")
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

	agentName := os.Getenv("ORTHRUS_AGENT_ID")
	if agentName == "" {
		agentName = *agentID
	}
	if agentName == "" {
		agentName = "unknown"
	}

	// NEVER log the auth key value.
	log.WithFields(logrus.Fields{
		"server_url":    svrURL,
		"agent_id":      agentName,
		"docker_socket": dockSock,
		"auth_key":      "[REDACTED]",
	}).Info("orthrus agent starting")

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

// validateServerURL rejects non-WebSocket schemes and insecure ws:// connections
// to non-localhost hosts. Allows ws://localhost and ws://127.0.0.1 for local testing.
func validateServerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())

	isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"

	switch scheme {
	case "wss":
		return nil
	case "ws":
		if isLocalhost {
			return nil
		}
		return fmt.Errorf("insecure ws:// scheme is only allowed for localhost; use wss:// for %q", host)
	case "http", "https":
		return fmt.Errorf("expected WebSocket scheme wss:// (got %q); use wss:// instead of %s://", rawURL, scheme)
	default:
		return fmt.Errorf("unsupported scheme %q: expected wss://", u.Scheme)
	}
}
