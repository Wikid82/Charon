package zerotier

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const defaultControllerURL = "https://api.zerotier.com"

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
			panic(fmt.Sprintf("zerotier: invalid private CIDR %q: %v", cidr, err))
		}
		privateRanges = append(privateRanges, network)
	}
}

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
	controllerURL string
	httpClient    *http.Client
}

// NewZeroTierClient creates a ZeroTierClient with SSRF validation on the controller URL.
// Returns an error if controllerURL is not a valid, reachable HTTPS address or if it
// resolves to a loopback, link-local, or RFC-1918 address.
func NewZeroTierClient(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error) {
	return newZeroTierClientWithURL(ctx, apiToken, controllerURL, false)
}

// newZeroTierClientWithURL is the internal constructor. When skipSSRF is true the
// DNS resolution check is skipped; use this only in tests that use httptest.Server.
func newZeroTierClientWithURL(ctx context.Context, apiToken, controllerURL string, skipSSRF bool) (*ZeroTierClient, error) {
	if controllerURL == "" {
		controllerURL = defaultControllerURL
	}

	parsed, err := url.Parse(controllerURL)
	if err != nil {
		return nil, fmt.Errorf("zerotier: invalid controller_url: %w", err)
	}

	if !skipSSRF {
		if parsed.Scheme != "https" {
			return nil, fmt.Errorf("zerotier: controller_url must use https scheme, got %q", parsed.Scheme)
		}
		host := parsed.Hostname()
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("zerotier: resolve controller host %q: %w", host, err)
		}
		for _, addr := range addrs {
			if isPrivateIP(addr.IP) {
				return nil, fmt.Errorf("zerotier: controller_url resolves to a private/loopback address — SSRF protection")
			}
		}
	}

	return &ZeroTierClient{
		apiToken:      apiToken,
		controllerURL: controllerURL,
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

// ListNetworks returns all ZeroTier networks accessible via the configured API token.
func (c *ZeroTierClient) ListNetworks(ctx context.Context) ([]ZeroTierNetwork, error) {
	var networks []ZeroTierNetwork
	if err := c.get(ctx, "/api/v1/network", &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

// ListMembers returns all members of the given ZeroTier network.
func (c *ZeroTierClient) ListMembers(ctx context.Context, networkID string) ([]ZeroTierMember, error) {
	var members []ZeroTierMember
	path := fmt.Sprintf("/api/v1/network/%s/member", networkID)
	if err := c.get(ctx, path, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (c *ZeroTierClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.controllerURL+path, http.NoBody)
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
