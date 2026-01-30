// Package caddy provides a client and manager for interacting with the Caddy Admin API.
package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
)

// Test hook for json marshalling to allow simulating failures in tests
var jsonMarshalClient = json.Marshal

// Client wraps the Caddy admin API.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	initErr    error
}

// NewClient creates a Caddy API client.
func NewClient(adminAPIURL string) *Client {
	return NewClientWithExpectedPort(adminAPIURL, defaultCaddyAdminPort)
}

const (
	defaultCaddyAdminPort = 2019
)

// NewClientWithExpectedPort creates a Caddy API client with an explicit expected port.
//
// This enforces a deny-by-default SSRF policy for internal service calls:
// - hostname must be in the internal-service allowlist (exact matches)
// - port must match expectedPort
// - proxy env vars ignored, redirects disabled
func NewClientWithExpectedPort(adminAPIURL string, expectedPort int) *Client {
	validatedBase, err := security.ValidateInternalServiceBaseURL(adminAPIURL, expectedPort, security.InternalServiceHostAllowlist())
	client := &Client{
		httpClient: network.NewInternalServiceHTTPClient(30 * time.Second),
		initErr:    err,
	}
	if err == nil {
		client.baseURL = validatedBase
	}
	return client
}

func (c *Client) endpoint(path string) (string, error) {
	if c.initErr != nil {
		return "", fmt.Errorf("caddy client init failed: %w", c.initErr)
	}
	if c.baseURL == nil {
		return "", fmt.Errorf("caddy client base URL is not configured")
	}
	u := c.baseURL.ResolveReference(&url.URL{Path: path})
	return u.String(), nil
}

// Load atomically replaces Caddy's entire configuration.
// This is the primary method for applying configuration changes.
func (c *Client) Load(ctx context.Context, config *Config) error {
	urlStr, err := c.endpoint("/load")
	if err != nil {
		return err
	}

	body, err := jsonMarshalClient(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetConfig retrieves the current running configuration from Caddy.
func (c *Client) GetConfig(ctx context.Context) (*Config, error) {
	urlStr, err := c.endpoint("/config/")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("caddy returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var config Config
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &config, nil
}

// Ping checks if Caddy admin API is reachable.
func (c *Client) Ping(ctx context.Context) error {
	urlStr, err := c.endpoint("/config/")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy unreachable: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("caddy returned status %d", resp.StatusCode)
	}

	return nil
}
