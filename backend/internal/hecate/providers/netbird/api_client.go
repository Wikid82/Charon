package netbird

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/pkg/safehttp"
)

const defaultManagementURL = "https://api.netbird.io"

// NetBirdPeer represents a peer registered in a NetBird network.
type NetBirdPeer struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IP          string    `json:"ip"`
	OS          string    `json:"os"`
	Connected   bool      `json:"connected"`
	LastSeen    time.Time `json:"last_seen"`
	Hostname    string    `json:"hostname"`
	GroupsCount int       `json:"groups_count,omitempty"`
}

const (
	cacheTTL       = 60 * time.Second
	requestTimeout = 15 * time.Second
)

// NetBirdClient is an authenticated HTTP client for the NetBird Management API.
type NetBirdClient struct {
	baseURL     *url.URL
	httpClient  *http.Client
	accessToken string

	mu        sync.RWMutex
	cache     []NetBirdPeer
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewNetBirdClient creates a NetBirdClient for the given management URL.
// The URL must be https and must not point at a loopback, link-local, private,
// carrier-grade NAT or otherwise restricted address. The URL is checked up front
// for an early, readable error; the HTTP client then re-validates the destination
// on every connection and never follows redirects.
func NewNetBirdClient(ctx context.Context, accessToken, managementURL string) (*NetBirdClient, error) {
	if managementURL == "" {
		managementURL = defaultManagementURL
	}
	if _, err := safehttp.ValidateURL(managementURL, safehttp.PublicHTTPSOnly()); err != nil {
		return nil, fmt.Errorf("netbird: invalid management_url: %w", err)
	}
	return newNetBirdClientWithHTTP(ctx, accessToken, managementURL, safehttp.NewClient(safehttp.PublicHTTPSOnly(), requestTimeout))
}

// newNetBirdClientWithHTTP builds a client around an already-constructed
// http.Client without validating the URL against the address policy. Production
// code reaches it only through NewNetBirdClient; tests use it with an httptest
// server's client.
func newNetBirdClientWithHTTP(_ context.Context, accessToken, managementURL string, hc *http.Client) (*NetBirdClient, error) {
	if managementURL == "" {
		managementURL = defaultManagementURL
	}
	parsed, err := url.Parse(managementURL)
	if err != nil {
		return nil, fmt.Errorf("netbird: invalid management_url: %w", err)
	}

	return &NetBirdClient{
		baseURL:     parsed,
		accessToken: accessToken,
		cacheTTL:    cacheTTL,
		httpClient:  hc,
	}, nil
}

// ListPeers returns all peers visible to the configured access token.
// Results are cached for 60 seconds; subsequent calls within that window return
// the cached result without making an HTTP request.
func (c *NetBirdClient) ListPeers(ctx context.Context) ([]NetBirdPeer, error) {
	c.mu.RLock()
	if c.cache != nil && time.Since(c.cacheTime) < c.cacheTTL {
		peers := c.cache
		c.mu.RUnlock()
		return peers, nil
	}
	c.mu.RUnlock()

	return c.fetchAndCache(ctx)
}

// ForceRefresh bypasses the cache and returns a fresh peer list, updating the cache.
func (c *NetBirdClient) ForceRefresh(ctx context.Context) ([]NetBirdPeer, error) {
	return c.fetchAndCache(ctx)
}

func (c *NetBirdClient) fetchAndCache(ctx context.Context) ([]NetBirdPeer, error) {
	target, err := safehttp.JoinPath(c.baseURL, "api", "peers")
	if err != nil {
		return nil, fmt.Errorf("netbird: build request url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("netbird: build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("netbird: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("netbird: unauthorized — check your access token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netbird: unexpected status %d", resp.StatusCode)
	}

	var peers []NetBirdPeer
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, fmt.Errorf("netbird: decode response: %w", err)
	}

	c.mu.Lock()
	c.cache = peers
	c.cacheTime = time.Now()
	c.mu.Unlock()

	return peers, nil
}
