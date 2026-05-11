package cloudflare

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/hecate"
	"github.com/Wikid82/charon/backend/internal/models"
)

// cfCredentials holds the decrypted JSON credentials for the Cloudflare provider.
// Both snake_case (current) and camelCase (legacy) keys are accepted to support
// credentials stored before the frontend key-mapping fix.
type cfCredentials struct {
	APIToken       string `json:"api_token"`
	APITokenAlt    string `json:"apiToken"`
	AccountID      string `json:"account_id"`
	AccountIDAlt   string `json:"accountId"`
	TunnelToken    string `json:"tunnel_token"`
	TunnelTokenAlt string `json:"tunnelToken"`
}

func (c *cfCredentials) resolve() {
	if c.APIToken == "" && c.APITokenAlt != "" {
		c.APIToken = c.APITokenAlt
	}
	if c.AccountID == "" && c.AccountIDAlt != "" {
		c.AccountID = c.AccountIDAlt
	}
	if c.TunnelToken == "" && c.TunnelTokenAlt != "" {
		c.TunnelToken = c.TunnelTokenAlt
	}
}

// CloudflareTunnelProvider implements hecate.TunnelProvider using the cloudflared binary.
type CloudflareTunnelProvider struct {
	cfg        *models.TunnelConfig
	creds      cfCredentials
	client     *CloudflareClient
	binaryPath string
	buf        *hecate.RingBuffer

	mu      sync.RWMutex
	state   hecate.TunnelState
	cmd     *exec.Cmd
	done    chan struct{}
	address string
}

// NewCloudflareProvider constructs a CloudflareTunnelProvider from a config and
// decrypted credentials JSON. The credentials must contain api_token, account_id,
// and tunnel_token fields.
func NewCloudflareProvider(cfg *models.TunnelConfig, credentials string) (*CloudflareTunnelProvider, error) {
	var creds cfCredentials
	if err := json.Unmarshal([]byte(credentials), &creds); err != nil {
		return nil, fmt.Errorf("cloudflare: parse credentials: %w", err)
	}
	creds.resolve()
	if creds.TunnelToken == "" {
		return nil, fmt.Errorf("cloudflare: tunnel_token is required in credentials")
	}

	return &CloudflareTunnelProvider{
		cfg:        cfg,
		creds:      creds,
		client:     NewCloudflareClient(creds.APIToken, creds.AccountID),
		binaryPath: "cloudflared",
		buf:        hecate.NewRingBuffer(1000),
		state:      hecate.TunnelStateStopped,
	}, nil
}

// Factory satisfies hecate.ProviderFactory and is registered with the TunnelManager
// by HecateService at startup.
func Factory(cfg *models.TunnelConfig, credentials string) (hecate.TunnelProvider, error) {
	return NewCloudflareProvider(cfg, credentials)
}

// Name returns the provider identifier string.
func (p *CloudflareTunnelProvider) Name() string {
	return "cloudflare"
}

// Status returns the current tunnel state.
func (p *CloudflareTunnelProvider) Status() hecate.TunnelState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// GetAddress returns an empty string; Cloudflare tunnels route via the CF network,
// not a static address.
func (p *CloudflareTunnelProvider) GetAddress() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.address
}

// GetClient returns the underlying CloudflareClient for API calls.
func (p *CloudflareTunnelProvider) GetClient() *CloudflareClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client
}

// Start validates the cloudflared binary is available, then launches it as a subprocess.
// stdout and stderr are captured to the internal ring buffer.
func (p *CloudflareTunnelProvider) Start(ctx context.Context) error {
	binaryPath, err := exec.LookPath(p.binaryPath)
	if err != nil {
		return fmt.Errorf("cloudflare: cloudflared binary not found (%q): %w", p.binaryPath, err)
	}

	p.mu.Lock()
	p.state = hecate.TunnelStateConnecting
	p.done = make(chan struct{})
	p.mu.Unlock()

	cmd := exec.CommandContext(ctx, binaryPath, "tunnel", "run") //nolint:gosec
	cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+p.creds.TunnelToken)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cloudflare: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cloudflare: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateError
		close(p.done)
		p.mu.Unlock()
		return fmt.Errorf("cloudflare: start cloudflared: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.state = hecate.TunnelStateConnected
	p.mu.Unlock()

	// Stream stdout to the ring buffer.
	go func() {
		s := bufio.NewScanner(stdoutPipe)
		for s.Scan() {
			p.buf.Write(s.Text())
		}
	}()

	// Stream stderr to the ring buffer.
	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			p.buf.Write(s.Text())
		}
	}()

	// Monitor process exit and update state accordingly.
	go func() {
		defer func() {
			p.mu.Lock()
			p.cmd = nil
			if p.state != hecate.TunnelStateStopped {
				p.state = hecate.TunnelStateError
			}
			p.mu.Unlock()
			p.buf.Close()
			close(p.done)
		}()
		_ = cmd.Wait()
	}()

	return nil
}

// Stop sends SIGTERM to the cloudflared subprocess and waits up to 10 seconds
// for it to exit. If it has not exited by then, SIGKILL is sent. Stop is idempotent.
func (p *CloudflareTunnelProvider) Stop() error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	p.mu.Unlock()

	if cmd == nil || done == nil {
		p.mu.Lock()
		p.state = hecate.TunnelStateStopped
		p.mu.Unlock()
		return nil
	}

	// If the process has already exited, nothing to do.
	select {
	case <-done:
		p.mu.Lock()
		p.state = hecate.TunnelStateStopped
		p.mu.Unlock()
		return nil
	default:
	}

	// Send SIGTERM; ignore error if the process has already exited.
	_ = cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Signal(syscall.SIGKILL)
		<-done
	}

	p.mu.Lock()
	p.state = hecate.TunnelStateStopped
	p.mu.Unlock()
	return nil
}
