package netbird

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// credentials holds the decrypted JSON credentials for the NetBird provider.
// Both snake_case (current) and camelCase (legacy) keys are accepted to support
// credentials stored before the frontend key-mapping fix.
type credentials struct {
	AccessToken      string `json:"access_token"`
	AccessTokenAlt   string `json:"accessToken"`
	ManagementURL    string `json:"management_url"`
	ManagementURLAlt string `json:"managementUrl"`
}

func (c *credentials) resolve() {
	if c.AccessToken == "" && c.AccessTokenAlt != "" {
		c.AccessToken = c.AccessTokenAlt
	}
	if c.ManagementURL == "" && c.ManagementURLAlt != "" {
		c.ManagementURL = c.ManagementURLAlt
	}
}

// NetBirdProvider implements hecate.TunnelProvider for NetBird network discovery.
// This phase is discovery-only: no process is spawned. Start() validates the token
// by calling ListPeers and creates the API client with SSRF validation.
type NetBirdProvider struct {
	cfg         *models.TunnelConfig
	creds       credentials
	client      *NetBirdClient
	buf         *hecate.RingBuffer
	newClientFn func(ctx context.Context, token, url string) (*NetBirdClient, error)

	mu    sync.RWMutex
	state hecate.TunnelState
}

// NewNetBirdProvider constructs a NetBirdProvider from a config and
// decrypted credentials JSON. The credentials must contain access_token;
// management_url is optional and defaults to https://api.netbird.io.
func NewNetBirdProvider(cfg *models.TunnelConfig, credentialsJSON string) (*NetBirdProvider, error) {
	var creds credentials
	if err := json.Unmarshal([]byte(credentialsJSON), &creds); err != nil {
		return nil, fmt.Errorf("netbird: parse credentials: %w", err)
	}
	creds.resolve()
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("netbird: access_token is required in credentials")
	}

	return &NetBirdProvider{
		cfg:         cfg,
		creds:       creds,
		buf:         hecate.NewRingBuffer(1000),
		state:       hecate.TunnelStateStopped,
		newClientFn: NewNetBirdClient,
	}, nil
}

// Factory satisfies hecate.ProviderFactory and is registered with the TunnelManager
// by HecateService at startup.
func Factory(cfg *models.TunnelConfig, credentialsJSON string) (hecate.TunnelProvider, error) {
	return NewNetBirdProvider(cfg, credentialsJSON)
}

// Name returns the provider identifier string.
func (p *NetBirdProvider) Name() string {
	return "netbird"
}

// Status returns the current provider state.
func (p *NetBirdProvider) Status() hecate.TunnelState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// GetAddress returns an empty string. NetBird peer addresses are discovered
// via the API rather than a single static endpoint.
func (p *NetBirdProvider) GetAddress() string {
	return ""
}

// Start creates the NetBird API client (with SSRF validation on management_url)
// and validates the token by calling ListPeers. On success state transitions to
// connected; on failure to error.
func (p *NetBirdProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	p.state = hecate.TunnelStateConnecting
	p.mu.Unlock()

	client, err := p.newClientFn(ctx, p.creds.AccessToken, p.creds.ManagementURL)
	if err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		p.mu.Unlock()
		return fmt.Errorf("netbird: create client: %w", err)
	}

	peers, err := client.ListPeers(ctx)
	if err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		p.mu.Unlock()
		return fmt.Errorf("netbird: validate access token: %w", err)
	}

	p.mu.Lock()
	p.client = client
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	p.buf.Write(fmt.Sprintf("NetBird provider started, %d peers discovered", len(peers)))
	return nil
}

// Stop transitions the provider to stopped. No process cleanup is required.
func (p *NetBirdProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = hecate.TunnelStateStopped
	return nil
}

// GetClient returns the underlying NetBirdClient. Returns nil if Start has not
// been called successfully.
func (p *NetBirdProvider) GetClient() *NetBirdClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}
