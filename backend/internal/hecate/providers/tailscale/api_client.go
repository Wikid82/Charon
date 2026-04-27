package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const tsBaseURL = "https://api.tailscale.com/api/v2"

// TailscaleDevice represents a device registered in a Tailscale tailnet.
type TailscaleDevice struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	Addresses []string  `json:"addresses"`
	OS        string    `json:"os"`
	LastSeen  time.Time `json:"lastSeen"`
	Online    bool      `json:"online"`
}

// devicesEnvelope is the Tailscale API response for listing devices.
type devicesEnvelope struct {
	Devices []TailscaleDevice `json:"devices"`
}

// cachedDevices holds a snapshot of the device list along with its fetch time.
type cachedDevices struct {
	devices   []TailscaleDevice
	fetchedAt time.Time
}

const cacheTTL = 60 * time.Second

// TailscaleClient is an authenticated HTTP client for the Tailscale API v2.
type TailscaleClient struct {
	apiKey     string
	tailnet    string
	baseURL    string
	httpClient *http.Client

	mu    sync.RWMutex
	cache *cachedDevices
}

// NewTailscaleClient creates a TailscaleClient with a 15-second request timeout.
func NewTailscaleClient(apiKey, tailnet string) *TailscaleClient {
	return &TailscaleClient{
		apiKey:  apiKey,
		tailnet: tailnet,
		baseURL: tsBaseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ListDevices returns the devices for the configured tailnet.
// Results are cached for 60 seconds; subsequent calls within that window return
// the cached result without making an HTTP request.
func (c *TailscaleClient) ListDevices(ctx context.Context) ([]TailscaleDevice, error) {
	c.mu.RLock()
	if c.cache != nil && time.Since(c.cache.fetchedAt) < cacheTTL {
		devices := c.cache.devices
		c.mu.RUnlock()
		return devices, nil
	}
	c.mu.RUnlock()

	return c.fetchAndCache(ctx)
}

// ForceRefresh bypasses the cache and returns a fresh device list, updating the cache.
func (c *TailscaleClient) ForceRefresh(ctx context.Context) ([]TailscaleDevice, error) {
	return c.fetchAndCache(ctx)
}

func (c *TailscaleClient) fetchAndCache(ctx context.Context) ([]TailscaleDevice, error) {
	url := fmt.Sprintf("%s/tailnet/%s/devices", c.baseURL, c.tailnet)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("tailscale: build request: %w", err)
	}
	req.SetBasicAuth(c.apiKey, "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tailscale: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("tailscale: unauthorized — check your API key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tailscale: unexpected status %d", resp.StatusCode)
	}

	var env devicesEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("tailscale: decode response: %w", err)
	}

	c.mu.Lock()
	c.cache = &cachedDevices{
		devices:   env.Devices,
		fetchedAt: time.Now(),
	}
	c.mu.Unlock()

	return env.Devices, nil
}
