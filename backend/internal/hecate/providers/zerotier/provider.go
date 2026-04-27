package zerotier

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// ztCredentials holds the decrypted JSON credentials for the ZeroTier provider.
type ztCredentials struct {
	APIToken      string `json:"api_token"`
	ControllerURL string `json:"controller_url"`
}

// ZeroTierProvider implements hecate.TunnelProvider for ZeroTier network discovery.
// This phase is discovery-only: no process is spawned. Start() validates the token
// by calling ListNetworks and creates the API client with SSRF validation.
type ZeroTierProvider struct {
	cfg         *models.TunnelConfig
	creds       ztCredentials
	client      *ZeroTierClient
	buf         *hecate.RingBuffer
	newClientFn func(ctx context.Context, apiToken, controllerURL string) (*ZeroTierClient, error)

	mu    sync.RWMutex
	state hecate.TunnelState
}

// NewZeroTierProvider constructs a ZeroTierProvider from a config and
// decrypted credentials JSON. The credentials must contain api_token;
// controller_url is optional and defaults to https://api.zerotier.com.
func NewZeroTierProvider(cfg *models.TunnelConfig, credentials string) (*ZeroTierProvider, error) {
	var creds ztCredentials
	if err := json.Unmarshal([]byte(credentials), &creds); err != nil {
		return nil, fmt.Errorf("zerotier: parse credentials: %w", err)
	}
	if creds.APIToken == "" {
		return nil, fmt.Errorf("zerotier: api_token is required in credentials")
	}

	return &ZeroTierProvider{
		cfg:         cfg,
		creds:       creds,
		buf:         hecate.NewRingBuffer(1000),
		state:       hecate.TunnelStateStopped,
		newClientFn: NewZeroTierClient,
	}, nil
}

// Factory satisfies hecate.ProviderFactory and is registered with the TunnelManager
// by HecateService at startup.
func Factory(cfg *models.TunnelConfig, credentials string) (hecate.TunnelProvider, error) {
	return NewZeroTierProvider(cfg, credentials)
}

// Name returns the provider identifier string.
func (p *ZeroTierProvider) Name() string {
	return "zerotier"
}

// Status returns the current provider state.
func (p *ZeroTierProvider) Status() hecate.TunnelState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// GetAddress returns an empty string. ZeroTier addresses are discovered via the API.
func (p *ZeroTierProvider) GetAddress() string {
	return ""
}

// Start creates the ZeroTier API client (with SSRF validation on controller_url)
// and validates the token by calling ListNetworks. On success state transitions to
// connected; on failure to error.
func (p *ZeroTierProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	p.state = hecate.TunnelStateConnecting
	p.mu.Unlock()

	client, err := p.newClientFn(ctx, p.creds.APIToken, p.creds.ControllerURL)
	if err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		p.mu.Unlock()
		return fmt.Errorf("zerotier: create client: %w", err)
	}

	p.mu.Lock()
	p.client = client
	p.mu.Unlock()

	if _, err := client.ListNetworks(ctx); err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		p.mu.Unlock()
		return fmt.Errorf("zerotier: validate api token: %w", err)
	}

	p.mu.Lock()
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()
	return nil
}

// Stop transitions the provider to stopped. No process cleanup is required.
func (p *ZeroTierProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = hecate.TunnelStateStopped
	return nil
}

// GetClient returns the underlying ZeroTierClient. Returns nil if Start has not
// been called successfully.
func (p *ZeroTierProvider) GetClient() *ZeroTierClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}
