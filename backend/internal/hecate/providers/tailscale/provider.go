package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// tsCredentials holds the decrypted JSON credentials for the Tailscale provider.
type tsCredentials struct {
	APIKey  string `json:"api_key"`
	Tailnet string `json:"tailnet"`
}

// TailscaleProvider implements hecate.TunnelProvider for Tailscale network discovery.
// This phase is discovery-only: no process is spawned. Start() validates the API key.
type TailscaleProvider struct {
	cfg    *models.TunnelConfig
	client *TailscaleClient
	buf    *hecate.RingBuffer

	mu    sync.RWMutex
	state hecate.TunnelState
}

// NewTailscaleProvider constructs a TailscaleProvider from a config and
// decrypted credentials JSON. The credentials must contain api_key and tailnet fields.
func NewTailscaleProvider(cfg *models.TunnelConfig, credentials string) (*TailscaleProvider, error) {
	var creds tsCredentials
	if err := json.Unmarshal([]byte(credentials), &creds); err != nil {
		return nil, fmt.Errorf("tailscale: parse credentials: %w", err)
	}
	if creds.APIKey == "" {
		return nil, fmt.Errorf("tailscale: api_key is required in credentials")
	}
	if creds.Tailnet == "" {
		return nil, fmt.Errorf("tailscale: tailnet is required in credentials")
	}

	return &TailscaleProvider{
		cfg:    cfg,
		client: NewTailscaleClient(creds.APIKey, creds.Tailnet),
		buf:    hecate.NewRingBuffer(1000),
		state:  hecate.TunnelStateStopped,
	}, nil
}

// Factory satisfies hecate.ProviderFactory and is registered with the TunnelManager
// by HecateService at startup.
func Factory(cfg *models.TunnelConfig, credentials string) (hecate.TunnelProvider, error) {
	return NewTailscaleProvider(cfg, credentials)
}

// Name returns the provider identifier string.
func (p *TailscaleProvider) Name() string {
	return "tailscale"
}

// Status returns the current provider state.
func (p *TailscaleProvider) Status() hecate.TunnelState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// GetAddress returns an empty string. Tailscale device addresses are discovered
// via the API rather than a single static endpoint.
func (p *TailscaleProvider) GetAddress() string {
	return ""
}

// Start validates the configured API key by calling ListDevices once.
// On success the state transitions to connected; on failure to error.
func (p *TailscaleProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	p.state = hecate.TunnelStateConnecting
	p.mu.Unlock()

	if _, err := p.client.ListDevices(ctx); err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		p.mu.Unlock()
		return fmt.Errorf("tailscale: validate api key: %w", err)
	}

	p.mu.Lock()
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()
	return nil
}

// Stop transitions the provider to stopped. No process cleanup is required.
func (p *TailscaleProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = hecate.TunnelStateStopped
	return nil
}

// GetClient exposes the underlying TailscaleClient for use by HTTP handlers
// that need to query device listings directly.
func (p *TailscaleProvider) GetClient() *TailscaleClient {
	return p.client
}
