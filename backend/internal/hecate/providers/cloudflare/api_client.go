package cloudflare

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const cfBaseURL = "https://api.cloudflare.com/client/v4"

// CloudflareTunnel represents a tunnel entry returned by the Cloudflare API.
type CloudflareTunnel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CloudflareAPIError is returned when the Cloudflare API responds with success==false.
type CloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *CloudflareAPIError) Error() string {
	return fmt.Sprintf("cloudflare api error %d: %s", e.Code, e.Message)
}

// cfEnvelope is the standard Cloudflare API response wrapper.
type cfEnvelope struct {
	Success bool                 `json:"success"`
	Result  json.RawMessage      `json:"result"`
	Errors  []CloudflareAPIError `json:"errors"`
}

// CloudflareClient is an authenticated HTTP client for the Cloudflare API v4.
type CloudflareClient struct {
	accountID  string
	apiToken   string
	baseURL    string
	httpClient *http.Client
}

// NewCloudflareClient creates a CloudflareClient with a 15-second request timeout.
func NewCloudflareClient(apiToken, accountID string) *CloudflareClient {
	return &CloudflareClient{
		accountID: accountID,
		apiToken:  apiToken,
		baseURL:   cfBaseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *CloudflareClient) do(ctx context.Context, method, path string, body io.Reader) (*cfEnvelope, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: http: %w", err)
	}
	defer resp.Body.Close()

	var env cfEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("cloudflare: decode response: %w", err)
	}

	if !env.Success {
		if len(env.Errors) > 0 {
			return nil, &env.Errors[0]
		}
		return nil, fmt.Errorf("cloudflare: unknown api error")
	}
	return &env, nil
}

// ListTunnels returns all managed tunnels for the configured account.
func (c *CloudflareClient) ListTunnels(ctx context.Context) ([]CloudflareTunnel, error) {
	env, err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/accounts/%s/cfd_tunnel", c.accountID), nil)
	if err != nil {
		return nil, err
	}

	var tunnels []CloudflareTunnel
	if err := json.Unmarshal(env.Result, &tunnels); err != nil {
		return nil, fmt.Errorf("cloudflare: parse tunnels: %w", err)
	}
	return tunnels, nil
}

// CreateTunnel creates a new Cloudflare tunnel with the given name.
// A cryptographically random 32-byte secret is generated for the tunnel.
func (c *CloudflareClient) CreateTunnel(ctx context.Context, name string) (*CloudflareTunnel, error) {
	secret, err := randomBase64(32)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: generate tunnel secret: %w", err)
	}

	payload, err := json.Marshal(map[string]string{
		"name":          name,
		"tunnel_secret": secret,
	})
	if err != nil {
		return nil, fmt.Errorf("cloudflare: marshal create payload: %w", err)
	}

	env, err := c.do(ctx, http.MethodPost,
		fmt.Sprintf("/accounts/%s/cfd_tunnel", c.accountID),
		strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}

	var tunnel CloudflareTunnel
	if err := json.Unmarshal(env.Result, &tunnel); err != nil {
		return nil, fmt.Errorf("cloudflare: parse tunnel: %w", err)
	}
	return &tunnel, nil
}

// DeleteTunnel deletes the tunnel identified by tunnelID.
func (c *CloudflareClient) DeleteTunnel(ctx context.Context, tunnelID string) error {
	_, err := c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", c.accountID, tunnelID), nil)
	return err
}

func randomBase64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
