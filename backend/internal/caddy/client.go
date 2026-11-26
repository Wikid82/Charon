// Package caddy provides a client and manager for interacting with the Caddy Admin API.
package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	charonconfig "github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/redact"
	"github.com/Wikid82/charon/backend/internal/security"
)

// Test hook for json marshalling to allow simulating failures in tests
var jsonMarshalClient = json.Marshal

// Client wraps the Caddy admin API.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	initErr    error
	// listen is the admin.listen value that makes Caddy serve the endpoint this client dials.
	listen string
}

// socketRequestBaseURL is the base of request URLs sent over the admin socket.
// The transport never resolves or dials this host, and Caddy does not check the
// Host header on unix sockets.
var socketRequestBaseURL = &url.URL{Scheme: "http", Host: "localhost"}

// NewClient creates a Caddy API client for adminAPI. A "unix://" value dials the
// admin socket. Any other value is a loopback URL checked against the TCP admin port.
func NewClient(adminAPI string) *Client {
	if socketPath, ok := charonconfig.CaddyAdminSocketPath(adminAPI); ok {
		return newSocketClient(socketPath, charonconfig.CaddyAdminListen(adminAPI))
	}
	return NewClientWithExpectedPort(adminAPI, charonconfig.CaddyAdminTCPPort)
}

// newSocketClient creates a client whose every connection goes to the unix socket at socketPath.
func newSocketClient(socketPath, listen string) *Client {
	httpClient := network.NewInternalServiceHTTPClient(30 * time.Second)
	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		return &Client{initErr: errors.New("admin socket transport is not an http.Transport")}
	}
	var dialer net.Dialer
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socketPath)
	}
	return &Client{baseURL: socketRequestBaseURL, httpClient: httpClient, listen: listen}
}

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
		listen:     charonconfig.CaddyAdminTCPAddress,
	}
	if err == nil {
		client.baseURL = validatedBase
	}
	return client
}

// AdminListen returns the admin.listen value that makes Caddy serve the endpoint this client dials.
func (c *Client) AdminListen() string {
	return c.listen
}

// adminListener is implemented by clients that report the admin endpoint they dial.
type adminListener interface {
	AdminListen() string
}

// adminListenOf returns the admin.listen value for client, or "" when the client
// does not report one. An empty value makes GenerateConfig use the default socket.
func adminListenOf(client CaddyClient) string {
	if l, ok := client.(adminListener); ok {
		return l.AdminListen()
	}
	return ""
}

// Fetch returns the body of a GET request to path on the admin API. It is the
// read-only diagnostic used by the caddy-admin command. Non-200 responses are errors.
func (c *Client) Fetch(ctx context.Context, path string) ([]byte, error) {
	urlStr, err := c.endpoint(path)
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
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newAdminError(resp.StatusCode, body, nil)
	}
	return body, nil
}

// maxFetchBytes bounds the body Fetch reads. A generated Caddy config is far smaller.
const maxFetchBytes = 64 << 20

// maxAdminErrorRead bounds how much of an error answer from the admin API is read.
const maxAdminErrorRead = 64 << 10

// maxAdminErrorText bounds the message kept in an AdminError.
const maxAdminErrorText = 2 << 10

// AdminError is a non-200 answer from Caddy's admin API. Message is Caddy's own message with credential values
// replaced and cut to maxAdminErrorText. Caddy quotes the configuration in its errors, so the message is never
// the raw answer.
type AdminError struct {
	Status  int
	Message string
}

func (e *AdminError) Error() string {
	return fmt.Sprintf("caddy returned status %d: %s", e.Status, e.Message)
}

// newAdminError builds the error for a non-200 answer. answer is the part of the body already read (at most
// maxAdminErrorRead bytes). sent is the JSON config the client posted, whose credential values are removed from
// the message; it is nil for requests without a body. The message is the "error" field of a JSON object answer
// when there is one, otherwise the raw text. Redaction runs before the cut, so a value is never shown partly.
func newAdminError(status int, answer, sent []byte) *AdminError {
	msg := string(answer)
	var doc struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(answer, &doc); err == nil && doc.Error != "" {
		msg = doc.Error
	}
	msg = redact.Text(msg, configSecretValues(sent))
	return &AdminError{Status: status, Message: redact.Truncate(msg, maxAdminErrorText)}
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
	body, err := jsonMarshalClient(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return c.LoadJSON(ctx, body)
}

// LoadJSON posts an already encoded config to /load. Load marshals and calls it.
func (c *Client) LoadJSON(ctx context.Context, body []byte) error {
	urlStr, err := c.endpoint("/load")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Caddy skips reloading an identical config. must-revalidate makes every load reach the running config, so
	// data read at startup (such as the GeoIP database) is re-read even when the generated JSON did not change.
	req.Header.Set("Cache-Control", "must-revalidate")

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
		answer, _ := io.ReadAll(io.LimitReader(resp.Body, maxAdminErrorRead))
		return newAdminError(resp.StatusCode, answer, body)
	}

	return nil
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
