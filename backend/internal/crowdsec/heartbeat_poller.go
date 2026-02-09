package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

const (
	defaultHeartbeatInterval = 5 * time.Minute
	heartbeatCheckTimeout    = 10 * time.Second
	stopTimeout              = 5 * time.Second
)

// HeartbeatPoller periodically checks console enrollment status and updates the last heartbeat timestamp.
// It automatically transitions enrollment from pending_acceptance to enrolled when the console confirms enrollment.
type HeartbeatPoller struct {
	db       *gorm.DB
	exec     EnvCommandExecutor
	dataDir  string
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  atomic.Bool
	stopOnce sync.Once
	mu       sync.Mutex // Protects concurrent access to enrollment record
}

// NewHeartbeatPoller creates a new HeartbeatPoller with the default 5-minute interval.
func NewHeartbeatPoller(db *gorm.DB, exec EnvCommandExecutor, dataDir string) *HeartbeatPoller {
	return &HeartbeatPoller{
		db:       db,
		exec:     exec,
		dataDir:  dataDir,
		interval: defaultHeartbeatInterval,
		stopCh:   make(chan struct{}),
	}
}

// SetInterval sets the polling interval. Should be called before Start().
func (p *HeartbeatPoller) SetInterval(d time.Duration) {
	p.interval = d
}

// IsRunning returns true if the poller is currently running.
func (p *HeartbeatPoller) IsRunning() bool {
	return p.running.Load()
}

// Start begins the background polling loop.
// It is safe to call multiple times; subsequent calls are no-ops if already running.
func (p *HeartbeatPoller) Start() {
	if !p.running.CompareAndSwap(false, true) {
		// Already running, skip
		return
	}

	p.wg.Add(1)
	go p.poll()

	logger.Log().WithField("interval", p.interval.String()).Info("heartbeat poller started")
}

// Stop signals the poller to stop and waits for graceful shutdown.
// It is safe to call multiple times; subsequent calls are no-ops.
func (p *HeartbeatPoller) Stop() {
	if !p.running.Load() {
		return
	}

	p.stopOnce.Do(func() {
		close(p.stopCh)
	})

	// Wait for the goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-time.After(stopTimeout):
		logger.Log().Warn("heartbeat poller stop timed out")
	}

	p.running.Store(false)
	logger.Log().Info("heartbeat poller stopped")
}

// poll runs the main polling loop using a ticker.
func (p *HeartbeatPoller) poll() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run an initial check immediately
	p.checkHeartbeat(context.Background())

	for {
		select {
		case <-ticker.C:
			p.checkHeartbeat(context.Background())
		case <-p.stopCh:
			return
		}
	}
}

// checkHeartbeat checks the console enrollment status and updates the database.
// It runs with a timeout and handles errors gracefully without crashing.
func (p *HeartbeatPoller) checkHeartbeat(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create context with timeout for command execution
	checkCtx, cancel := context.WithTimeout(ctx, heartbeatCheckTimeout)
	defer cancel()

	// Check if console is enrolled
	var enrollment models.CrowdsecConsoleEnrollment
	if err := p.db.WithContext(checkCtx).First(&enrollment).Error; err != nil {
		// No enrollment record, skip check
		return
	}

	// Skip if not enrolled or pending acceptance
	if enrollment.Status != consoleStatusEnrolled && enrollment.Status != consoleStatusPendingAcceptance {
		return
	}

	// Run `cscli console status` to check connectivity
	args := []string{"console", "status"}
	configPath := p.findConfigPath()
	if configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}

	out, err := p.exec.ExecuteWithEnv(checkCtx, "cscli", args, nil)
	if err != nil {
		logger.Log().WithError(err).WithField("output", string(out)).Debug("heartbeat check failed")
		return
	}

	output := string(out)
	now := time.Now().UTC()

	// Check if the output indicates successful enrollment/connection
	// CrowdSec console status output typically contains "enrolled" and "connected" when healthy
	if p.isEnrolledOutput(output) {
		// Update heartbeat timestamp
		enrollment.LastHeartbeatAt = &now

		// Transition from pending_acceptance to enrolled if console shows enrolled
		if enrollment.Status == consoleStatusPendingAcceptance {
			enrollment.Status = consoleStatusEnrolled
			enrollment.EnrolledAt = &now
			logger.Log().WithField("agent_name", enrollment.AgentName).Info("enrollment status transitioned from pending_acceptance to enrolled")
		}

		if err := p.db.WithContext(checkCtx).Save(&enrollment).Error; err != nil {
			logger.Log().WithError(err).Warn("failed to update heartbeat timestamp")
		} else {
			logger.Log().Debug("console heartbeat updated")
		}
	}
}

// isEnrolledOutput checks if the cscli console status output indicates successful enrollment.
// It detects positive enrollment indicators while excluding negative statements like "not enrolled".
func (p *HeartbeatPoller) isEnrolledOutput(output string) bool {
	lower := strings.ToLower(output)

	// Check for negative indicators first - if present, we're not enrolled
	negativeIndicators := []string{
		"not enrolled",
		"not connected",
		"you are not",
		"is not enrolled",
	}
	for _, neg := range negativeIndicators {
		if strings.Contains(lower, neg) {
			return false
		}
	}

	// CrowdSec console status shows "enrolled" and "connected" when healthy
	// Example: "Your engine is enrolled and connected to console"
	hasEnrolled := strings.Contains(lower, "enrolled")
	hasConnected := strings.Contains(lower, "connected")
	hasConsole := strings.Contains(lower, "console")

	return hasEnrolled && (hasConnected || hasConsole)
}

// findConfigPath returns the path to the CrowdSec config file.
func (p *HeartbeatPoller) findConfigPath() string {
	configPath := filepath.Join(p.dataDir, "config", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	configPath = filepath.Join(p.dataDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	return ""
}
