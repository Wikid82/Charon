package zerotier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/Wikid82/charon/backend/pkg/safehttp"
)

const (
	defaultControllerURL = "https://api.zerotier.com"
	requestTimeout       = 15 * time.Second
)

// ErrInvalidNetworkID is returned when a ZeroTier network ID is not exactly 16
// lowercase hexadecimal characters.
var ErrInvalidNetworkID = errors.New("zerotier: invalid network id")

// networkIDPattern matches a ZeroTier network ID.
var networkIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ZeroTierNetwork represents a ZeroTier network entry.
type ZeroTierNetwork struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

// ZeroTierMember represents a member of a ZeroTier network.
type ZeroTierMember struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	IPAssignments []string `json:"ipAssignments"`
	Online        bool     `json:"authorized"`
}

// ZeroTierClient is an authenticated HTTP client for the ZeroTier Central API.
type ZeroTierClient struct {
	apiToken      string
	controllerURL *url.URL
	httpClient    *http.Client
}

// NewZeroTierClient creates a ZeroTierClient for the given controller URL.
// The URL must be https and must not point at a loopback, link-local, private,
// carrier-grade NAT or otherwise restricted address. The URL is checked up front
// for an early, readable error; the HTTP client then re-validates the destination
// on every connection and never follows redirects.
func NewZeroTierClient(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error) {
	if controllerURL == "" {
		controllerURL = defaultControllerURL
	}
	if _, err := safehttp.ValidateURL(controllerURL, safehttp.PublicHTTPSOnly()); err != nil {
		return nil, fmt.Errorf("zerotier: invalid controller_url: %w", err)
	}
	return newZeroTierClientWithHTTP(ctx, apiToken, controllerURL, safehttp.NewClient(safehttp.PublicHTTPSOnly(), requestTimeout))
}

// newZeroTierClientWithHTTP builds a client around an already-constructed
// http.Client without validating the URL against the address policy. Production
// code reaches it only through NewZeroTierClient; tests use it with an httptest
// server's client.
func newZeroTierClientWithHTTP(_ context.Context, apiToken, controllerURL string, hc *http.Client) (*ZeroTierClient, error) {
	if controllerURL == "" {
		controllerURL = defaultControllerURL
	}
	parsed, err := url.Parse(controllerURL)
	if err != nil {
		return nil, fmt.Errorf("zerotier: invalid controller_url: %w", err)
	}

	return &ZeroTierClient{
		apiToken:      apiToken,
		controllerURL: parsed,
		httpClient:    hc,
	}, nil
}

// ListNetworks returns all ZeroTier networks accessible via the configured API token.
func (c *ZeroTierClient) ListNetworks(ctx context.Context) ([]ZeroTierNetwork, error) {
	var networks []ZeroTierNetwork
	if err := c.get(ctx, &networks, "api", "v1", "network"); err != nil {
		return nil, err
	}
	return networks, nil
}

// ListMembers returns all members of the given ZeroTier network.
func (c *ZeroTierClient) ListMembers(ctx context.Context, networkID string) ([]ZeroTierMember, error) {
	if !networkIDPattern.MatchString(networkID) {
		return nil, ErrInvalidNetworkID
	}
	var members []ZeroTierMember
	if err := c.get(ctx, &members, "api", "v1", "network", networkID, "member"); err != nil {
		return nil, err
	}
	return members, nil
}

func (c *ZeroTierClient) get(ctx context.Context, out any, segments ...string) error {
	target, err := safehttp.JoinPath(c.controllerURL, segments...)
	if err != nil {
		return fmt.Errorf("zerotier: build request url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return fmt.Errorf("zerotier: build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zerotier: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("zerotier: unauthorized — check your API token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("zerotier: unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("zerotier: decode response: %w", err)
	}
	return nil
}
