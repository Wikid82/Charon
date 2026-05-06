package netbird

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const defaultManagementURL = "https://api.netbird.io"

// privateRanges defines IP ranges that must not be contacted to prevent SSRF.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fe80::/10",
		"fc00::/7",
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("netbird: invalid private CIDR %q: %v", cidr, err))
		}
		privateRanges = append(privateRanges, network)
	}
}

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

const cacheTTL = 60 * time.Second

// NetBirdClient is an authenticated HTTP client for the NetBird Management API.
type NetBirdClient struct {
	baseURL     string
	httpClient  *http.Client
	accessToken string

	mu        sync.RWMutex
	cache     []NetBirdPeer
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewNetBirdClient creates a NetBirdClient with SSRF validation on the management URL.
// Returns an error if managementURL is not a valid, reachable HTTPS address or if it
// resolves to a loopback, link-local, or RFC-1918 address.
func NewNetBirdClient(ctx context.Context, accessToken, managementURL string) (*NetBirdClient, error) {
	return newNetBirdClientWithURL(ctx, accessToken, managementURL, false)
}

// newNetBirdClientWithURL is the internal constructor. When skipSSRF is true the
// DNS resolution check is skipped; use this only in tests that use httptest.Server.
func newNetBirdClientWithURL(ctx context.Context, accessToken, managementURL string, skipSSRF bool) (*NetBirdClient, error) {
	if managementURL == "" {
		managementURL = defaultManagementURL
	}

	parsed, err := url.Parse(managementURL)
	if err != nil {
		return nil, fmt.Errorf("netbird: invalid management_url: %w", err)
	}

	if !skipSSRF {
		if parsed.Scheme != "https" {
			return nil, fmt.Errorf("netbird: management_url must use https scheme, got %q", parsed.Scheme)
		}
		host := parsed.Hostname()
		addrs, resolveErr := net.DefaultResolver.LookupIPAddr(ctx, host)
		if resolveErr != nil {
			return nil, fmt.Errorf("netbird: resolve management host %q: %w", host, resolveErr)
		}
		for _, addr := range addrs {
			if isPrivateIP(addr.IP) {
				return nil, fmt.Errorf("netbird: management_url resolves to a private/loopback address — SSRF protection")
			}
		}
	}

	return &NetBirdClient{
		baseURL:     managementURL,
		accessToken: accessToken,
		cacheTTL:    cacheTTL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// isPrivateIP returns true if ip falls within any of the restricted private ranges.
func isPrivateIP(ip net.IP) bool {
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/peers", http.NoBody)
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
