package hecate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ProviderFactory creates a TunnelProvider from a config and decrypted credentials.
// PR 3 registers factories for each supported provider type without modifying this file.
type ProviderFactory func(cfg *models.TunnelConfig, credentials string) (TunnelProvider, error)

// tunnelState holds the runtime state of a single managed tunnel.
type tunnelState struct {
	uuid     string
	name     string
	provider string
	instance TunnelProvider
	buffer   *RingBuffer
	cancel   context.CancelFunc
	startAt  time.Time
}

// backoffDelays defines the exponential backoff schedule for tunnel restart attempts.
var backoffDelays = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	60 * time.Second,
}

// TunnelManager supervises the lifecycle of all active tunnel providers.
type TunnelManager struct {
	db        *gorm.DB
	encSvc    *crypto.EncryptionService
	factories map[models.TunnelProviderType]ProviderFactory
	state     map[string]*tunnelState
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewTunnelManager creates a TunnelManager ready to accept factory registrations and tunnel operations.
func NewTunnelManager(db *gorm.DB, encSvc *crypto.EncryptionService) *TunnelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TunnelManager{
		db:        db,
		encSvc:    encSvc,
		factories: make(map[models.TunnelProviderType]ProviderFactory),
		state:     make(map[string]*tunnelState),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterFactory registers a provider factory for a given provider type.
// Called by each provider package (cloudflare, tailscale, zerotier) at init time.
func (m *TunnelManager) RegisterFactory(providerType models.TunnelProviderType, factory ProviderFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.factories[providerType] = factory
}

// RegisterProvider injects a TunnelProvider directly, bypassing factory lookup.
// Intended for tests and the service layer when a provider instance already exists.
func (m *TunnelManager) RegisterProvider(uuid string, p TunnelProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.state[uuid]; ok {
		existing.instance = p
		return
	}

	watchCtx, cancelFn := context.WithCancel(m.ctx) //nolint:gosec // G118: cancelFn stored in tunnelState.cancel and called during cleanup
	m.state[uuid] = &tunnelState{
		uuid:     uuid,
		name:     p.Name(),
		instance: p,
		buffer:   NewRingBuffer(1000),
		cancel:   cancelFn,
		startAt:  time.Now(),
	}
	go m.runWatcher(watchCtx, uuid)
}

// Start loads all active TunnelConfig records from the database and starts their providers.
// Returns after all available providers are started (or skipped if no factory is registered).
func (m *TunnelManager) Start(ctx context.Context) error {
	var configs []models.TunnelConfig
	if err := m.db.Where("is_active = ?", true).Find(&configs).Error; err != nil {
		return fmt.Errorf("hecate: load active tunnels: %w", err)
	}

	for i := range configs {
		if err := m.StartTunnel(configs[i].UUID); err != nil {
			logger.Log().WithFields(logrus.Fields{
				"uuid":  configs[i].UUID,
				"error": err.Error(),
			}).Warn("hecate: failed to start tunnel on startup")
		}
	}
	return nil
}

// Stop shuts down all running providers and cancels background goroutines.
func (m *TunnelManager) Stop() error {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for uuid, ps := range m.state {
		if err := ps.instance.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("tunnel %s: %w", uuid, err))
		}
		ps.buffer.Close()
	}
	m.state = make(map[string]*tunnelState)

	if len(errs) > 0 {
		return fmt.Errorf("hecate: stop errors: %v", errs)
	}
	return nil
}

// StartTunnel starts the provider for the tunnel identified by uuid.
// If no factory is registered for the provider type, a warning is logged and nil is returned.
func (m *TunnelManager) StartTunnel(uuid string) error {
	var cfg models.TunnelConfig
	if err := m.db.Where("uuid = ?", uuid).First(&cfg).Error; err != nil {
		return fmt.Errorf("hecate: tunnel %s not found: %w", uuid, err)
	}

	credBytes, err := m.encSvc.Decrypt(cfg.EncryptedCredentials)
	if err != nil {
		return fmt.Errorf("hecate: decrypt credentials for %s: %w", uuid, err)
	}

	m.mu.RLock()
	ps, alreadyRunning := m.state[uuid]
	factory, hasFactory := m.factories[cfg.Provider]
	m.mu.RUnlock()

	if alreadyRunning {
		state := ps.instance.Status()
		if state == TunnelStateConnected || state == TunnelStateConnecting {
			return nil
		}
	}

	if !hasFactory {
		logger.Log().WithFields(logrus.Fields{
			"uuid":     uuid,
			"provider": string(cfg.Provider),
		}).Warn("hecate: no factory registered for provider type, skipping")
		return nil
	}

	p, err := factory(&cfg, string(credBytes))
	if err != nil {
		return fmt.Errorf("hecate: create provider for %s: %w", uuid, err)
	}

	watchCtx, cancelFn := context.WithCancel(m.ctx)
	if err := p.Start(watchCtx); err != nil {
		cancelFn()
		return fmt.Errorf("hecate: start provider for %s: %w", uuid, err)
	}

	m.mu.Lock()
	m.state[uuid] = &tunnelState{
		uuid:     cfg.UUID,
		name:     cfg.Name,
		provider: string(cfg.Provider),
		instance: p,
		buffer:   NewRingBuffer(1000),
		cancel:   cancelFn,
		startAt:  time.Now(),
	}
	m.mu.Unlock()

	go m.runWatcher(watchCtx, uuid)
	return nil
}

// StopTunnel stops the provider for the tunnel identified by uuid.
func (m *TunnelManager) StopTunnel(uuid string) error {
	m.mu.Lock()
	ps, ok := m.state[uuid]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.state, uuid)
	m.mu.Unlock()

	ps.cancel()
	ps.buffer.Close()
	if err := ps.instance.Stop(); err != nil {
		return fmt.Errorf("hecate: stop tunnel %s: %w", uuid, err)
	}
	return nil
}

// GetStatus returns a snapshot of the current state for all managed tunnels.
func (m *TunnelManager) GetStatus() []TunnelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]TunnelStatus, 0, len(m.state))
	now := time.Now()
	for uuid, ps := range m.state {
		var uptime int64
		if ps.instance.Status() == TunnelStateConnected {
			uptime = int64(now.Sub(ps.startAt).Seconds())
		}
		statuses = append(statuses, TunnelStatus{
			UUID:          uuid,
			Name:          ps.name,
			Provider:      ps.provider,
			State:         ps.instance.Status(),
			UptimeSeconds: uptime,
		})
	}
	return statuses
}

// GetLogBuffer returns the ring buffer for the tunnel identified by uuid.
func (m *TunnelManager) GetLogBuffer(uuid string) (*RingBuffer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ps, ok := m.state[uuid]
	if !ok {
		return nil, fmt.Errorf("hecate: tunnel %s not found in state", uuid)
	}
	return ps.buffer, nil
}

// runWatcher monitors a tunnel provider and applies exponential backoff restart on failure.
func (m *TunnelManager) runWatcher(ctx context.Context, uuid string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.RLock()
		ps, ok := m.state[uuid]
		m.mu.RUnlock()
		if !ok {
			return
		}

		state := ps.instance.Status()
		if state == TunnelStateConnected || state == TunnelStateConnecting {
			attempt = 0
			continue
		}

		idx := attempt
		if idx >= len(backoffDelays) {
			idx = len(backoffDelays) - 1
		}
		delay := backoffDelays[idx]
		attempt++

		logger.Log().WithFields(logrus.Fields{
			"uuid":  uuid,
			"state": string(state),
			"delay": delay.String(),
		}).Warn("hecate: tunnel in error/stopped state, scheduling restart")

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		m.mu.RLock()
		ps2, still := m.state[uuid]
		m.mu.RUnlock()
		if !still {
			return
		}

		if err := ps2.instance.Start(ctx); err != nil {
			logger.Log().WithFields(logrus.Fields{
				"uuid":  uuid,
				"error": err.Error(),
			}).Error("hecate: failed to restart tunnel")
		}
	}
}
