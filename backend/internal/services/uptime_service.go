package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// orthrusStatusChecker allows UptimeService to query Orthrus session liveness
// without a direct dependency on the orthrus package.
type orthrusStatusChecker interface {
	GetProxyAddr(agentUUID string) (string, bool)
}

type UptimeService struct {
	DB                  *gorm.DB
	NotificationService *NotificationService
	orthrusResolver     orthrusStatusChecker // nil when Orthrus feature is disabled
	// Batching: track pending notifications
	pendingNotifications map[string]*pendingHostNotification
	notificationMutex    sync.Mutex
	batchWindow          time.Duration
	// Host-specific mutexes to prevent concurrent database updates
	hostMutexes   map[string]*sync.Mutex
	hostMutexLock sync.Mutex
	// Configuration
	config UptimeConfig
	// uptimeCfg is the hot-reloading snapshot of the uptime.* Settings rows
	// (default interval, worker pool size, retention days). Shared read-only
	// with the scheduler/pruner in later commits; used here for write-time
	// interval resolution. Named to avoid colliding with the config field above.
	uptimeCfg *uptimeConfig

	// Ingester is the buffered, batched write path for check results (spec
	// §3.3). Constructed here so later commits (worker pool, /uptime/health)
	// can hold a stable reference. Its Run loop is NOT started yet — nothing
	// sends to it until the scheduler commit wires the worker pool — so it is
	// inert: DroppedCount() returns 0 and no goroutine is spawned.
	Ingester *UptimeIngester

	// Pool is the bounded worker pool that owns the authoritative in-memory
	// debounce maps (monState/hostState) and the shared keep-alive SSRF client
	// (spec §3.2). When non-nil (production, from C5 on) the check paths
	// (CheckAll, SyncAndCheckForHost/RemoteServer, the manual-check handler)
	// enqueue onto it instead of probing inline. nil in most unit tests, which
	// then take the synchronous inline path below.
	Pool *UptimeWorkerPool

	// checker is the one shared probe engine (single keep-alive SSRF client +
	// one copy of the http/tcp/orthrus switch). Used by the inline check path
	// here and reused by the worker pool (spec §3.1.5 / N3).
	checker *uptimeChecker
}

// ErrIntervalTooLow is returned by UpdateMonitor when a caller tries to set a
// check interval below the 30-second hard floor. Handlers map it to HTTP 400.
var ErrIntervalTooLow = errors.New("interval must be at least 30 seconds")

// UptimeConfig holds configurable timeouts and thresholds
type UptimeConfig struct {
	TCPTimeout       time.Duration
	MaxRetries       int
	FailureThreshold int
	CheckTimeout     time.Duration
	StaggerDelay     time.Duration
}

type pendingHostNotification struct {
	hostID       string
	hostName     string
	downMonitors []monitorDownInfo
	timer        *time.Timer
	createdAt    time.Time
}

type monitorDownInfo struct {
	ID             string
	Name           string
	URL            string
	Message        string
	PreviousUptime string
}

func NewUptimeService(db *gorm.DB, ns *NotificationService) *UptimeService {
	s := &UptimeService{
		DB:                   db,
		NotificationService:  ns,
		pendingNotifications: make(map[string]*pendingHostNotification),
		batchWindow:          30 * time.Second, // Wait 30 seconds to batch notifications
		hostMutexes:          make(map[string]*sync.Mutex),
		uptimeCfg:            newUptimeConfig(db),
		Ingester:             newUptimeIngester(db),
		config: UptimeConfig{
			TCPTimeout:       10 * time.Second,
			MaxRetries:       2,
			FailureThreshold: 2,
			CheckTimeout:     60 * time.Second,
			StaggerDelay:     100 * time.Millisecond,
		},
	}
	s.checker = newUptimeChecker(s)
	return s
}

// uptimeFeatureEnabled reports whether feature.uptime.enabled is set to "true".
// A missing row is treated as enabled (matches the pre-existing default at every
// call site that used to inline this check).
func (s *UptimeService) uptimeFeatureEnabled() bool {
	var setting models.Setting
	if err := s.DB.Where("key = ?", "feature.uptime.enabled").First(&setting).Error; err == nil {
		return setting.Value == "true"
	}
	return true
}

// SetOrthrusResolver injects the Orthrus session resolver.
// Uses the typed-nil guard pattern established in DockerHandler.
func (s *UptimeService) SetOrthrusResolver(r orthrusStatusChecker) {
	if r == nil {
		s.orthrusResolver = nil
		return
	}

	rv := reflect.ValueOf(r)
	if (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && rv.IsNil() {
		s.orthrusResolver = nil
		return
	}
	s.orthrusResolver = r
}

// extractPort extracts the port from a URL or host:port string
func extractPort(urlStr string) string {
	// Try parsing as URL first
	if u, err := url.Parse(urlStr); err == nil && u.Host != "" {
		// Check if port is in the host
		port := u.Port()
		if port != "" {
			return port
		}

		// Look for :port pattern in the path (like /api/webhooks/123/abc:8080)
		// This handles webhook URLs where the token contains a port-like pattern
		if strings.Contains(u.Path, ":") {
			// Find the last : followed by digits
			parts := strings.Split(u.Path, ":")
			for i := len(parts) - 1; i >= 1; i-- {
				// Extract digits after the colon
				candidate := parts[i]
				// Take only leading digits (stop at / or other chars)
				digits := ""
				for _, r := range candidate {
					if r >= '0' && r <= '9' {
						digits += string(r)
					} else {
						break
					}
				}
				if digits != "" {
					return digits
				}
			}
		}

		// Default ports based on scheme
		if u.Scheme == "https" {
			return "443"
		}
		if u.Scheme == "http" {
			return "80"
		}
	}

	// Try as host:port
	if _, port, err := net.SplitHostPort(urlStr); err == nil {
		return port
	}

	return ""
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SyncMonitors ensures every ProxyHost and RemoteServer has a corresponding UptimeMonitor
// and that UptimeHosts are created for grouping
func (s *UptimeService) SyncMonitors() error {
	var hosts []models.ProxyHost
	if err := s.DB.Find(&hosts).Error; err != nil {
		return err
	}

	for _, host := range hosts {
		var monitor models.UptimeMonitor
		err := s.DB.Where("proxy_host_id = ?", host.ID).First(&monitor).Error

		domains := strings.Split(host.DomainNames, ",")
		firstDomain := ""
		if len(domains) > 0 {
			firstDomain = strings.TrimSpace(domains[0])
		}

		// Construct the public URL
		scheme := "http"
		if host.SSLForced {
			scheme = "https"
		}
		publicURL := fmt.Sprintf("%s://%s", scheme, firstDomain)
		internalURL := fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)

		// The upstream host for grouping is the ForwardHost
		upstreamHost := host.ForwardHost

		switch err {
		case gorm.ErrRecordNotFound:
			// Create new monitor
			name := host.Name
			if name == "" {
				name = firstDomain
			}

			// Find or create UptimeHost
			uptimeHostID := s.ensureUptimeHost(upstreamHost, name)

			monitor = models.UptimeMonitor{
				ProxyHostID:  &host.ID,
				UptimeHostID: &uptimeHostID,
				Name:         name,
				Type:         "http", // Check public access
				URL:          publicURL,
				UpstreamHost: upstreamHost,
				Interval:     clampInterval(0, s.uptimeCfg), // honour uptime.default_interval_seconds (S3)
				Enabled:      true,
				Status:       "pending",
			}
			if err := s.DB.Create(&monitor).Error; err != nil {
				logger.Log().WithError(err).WithField("host_id", host.ID).Error("Failed to create monitor")
			}
		case nil:
			// Always sync the name from proxy host
			newName := host.Name
			if newName == "" {
				newName = firstDomain
			}
			needsSave := false

			if monitor.Name != newName {
				monitor.Name = newName
				needsSave = true
			}

			// Ensure upstream host is set for grouping
			if monitor.UpstreamHost == "" || monitor.UpstreamHost != upstreamHost {
				monitor.UpstreamHost = upstreamHost
				needsSave = true
			}

			// Ensure UptimeHost link exists
			if monitor.UptimeHostID == nil {
				uptimeHostID := s.ensureUptimeHost(upstreamHost, newName)
				monitor.UptimeHostID = &uptimeHostID
				needsSave = true
			}

			// Update existing monitor if it looks like it's using the old default (TCP to internal upstream)
			if monitor.Type == "tcp" && monitor.URL == internalURL {
				monitor.Type = "http"
				monitor.URL = publicURL
				needsSave = true
				logger.Log().WithField("host_id", host.ID).Infof("Migrated monitor for host %d to check public URL: %s", host.ID, publicURL)
			}

			// Upgrade to HTTPS if SSL is forced and we are currently checking HTTP
			if host.SSLForced && strings.HasPrefix(monitor.URL, "http://") {
				monitor.URL = strings.Replace(monitor.URL, "http://", "https://", 1)
				needsSave = true
				logger.Log().WithField("host_id", host.ID).Infof("Upgraded monitor for host %d to HTTPS: %s", host.ID, monitor.URL)
			}

			if needsSave {
				s.DB.Save(&monitor)
			}
		}
	}

	// Sync Remote Servers
	var remoteServers []models.RemoteServer
	if err := s.DB.Find(&remoteServers).Error; err != nil {
		return err
	}

	for _, server := range remoteServers {
		var monitor models.UptimeMonitor
		err := s.DB.Where("remote_server_id = ?", server.ID).First(&monitor).Error

		targetType := "tcp"
		targetURL := fmt.Sprintf("%s:%d", server.Host, server.Port)

		if server.Scheme == "http" || server.Scheme == "https" {
			targetType = server.Scheme
			targetURL = fmt.Sprintf("%s://%s:%d", server.Scheme, server.Host, server.Port)
		}

		// The upstream host for grouping
		upstreamHost := server.Host

		// Orthrus-managed servers: connectivity is measured by session liveness, not TCP.
		if server.ConnectionType == models.ConnectionTypeOrthrus {
			if server.OrthrusAgentUUID == nil || *server.OrthrusAgentUUID == "" {
				continue // No agent linked — cannot create a meaningful monitor
			}
			targetType = "orthrus"
			targetURL = *server.OrthrusAgentUUID // Agent UUID as the monitor identifier
			// upstreamHost remains server.Host (Tailscale IP) — correct for grouping/display
		}

		switch err {
		case gorm.ErrRecordNotFound:
			// Find or create UptimeHost
			uptimeHostID := s.ensureUptimeHost(upstreamHost, server.Name)

			monitor = models.UptimeMonitor{
				RemoteServerID: &server.ID,
				UptimeHostID:   &uptimeHostID,
				Name:           server.Name,
				Type:           targetType,
				URL:            targetURL,
				UpstreamHost:   upstreamHost,
				Interval:       clampInterval(0, s.uptimeCfg), // honour uptime.default_interval_seconds (S3)
				Enabled:        server.Enabled,
				Status:         "pending",
			}
			if err := s.DB.Create(&monitor).Error; err != nil {
				logger.Log().WithError(err).WithField("remote_server_id", server.ID).Error("Failed to create monitor for remote server")
			}
		case nil:
			needsSave := false

			if monitor.Name != server.Name {
				monitor.Name = server.Name
				needsSave = true
			}

			// Ensure upstream host is set for grouping
			if monitor.UpstreamHost == "" || monitor.UpstreamHost != upstreamHost {
				monitor.UpstreamHost = upstreamHost
				needsSave = true
			}

			// Ensure UptimeHost link exists
			if monitor.UptimeHostID == nil {
				uptimeHostID := s.ensureUptimeHost(upstreamHost, server.Name)
				monitor.UptimeHostID = &uptimeHostID
				needsSave = true
			}

			if monitor.URL != targetURL || monitor.Type != targetType {
				monitor.URL = targetURL
				monitor.Type = targetType
				needsSave = true
			}
			if monitor.Enabled != server.Enabled {
				monitor.Enabled = server.Enabled
				needsSave = true
			}

			if needsSave {
				s.DB.Save(&monitor)
			}
		}
	}

	return nil
}

// ensureUptimeHost finds or creates an UptimeHost for the given host string and
// returns its ID (or "" on failure).
//
// Host resolution is atomic: rather than a check-then-act (SELECT, then INSERT
// if missing) that races two concurrent callers into a "UNIQUE constraint
// failed: uptime_hosts.host" on the loser (GitHub issue #1221), the create is
// an upsert on the `host` unique index with DoNothing. When the insert is a
// no-op because another caller won the race, the winning row is re-fetched so
// every caller returns the same non-empty ID.
func (s *UptimeService) ensureUptimeHost(host, defaultName string) string {
	var uptimeHost models.UptimeHost
	err := s.DB.Where("host = ?", host).First(&uptimeHost).Error
	if err == nil {
		return uptimeHost.ID
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Log().WithError(err).WithField("host", util.SanitizeForLog(host)).Error("Failed to query UptimeHost")
		return ""
	}

	uptimeHost = models.UptimeHost{
		Host:   host,
		Name:   defaultName,
		Status: "pending",
	}
	result := s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "host"}},
		DoNothing: true,
	}).Create(&uptimeHost)
	if result.Error != nil {
		logger.Log().WithError(result.Error).WithField("host", util.SanitizeForLog(host)).Error("Failed to create UptimeHost")
		return ""
	}

	if result.RowsAffected == 0 {
		// Lost the race: another caller inserted this host between our lookup
		// and our insert. Load the winning row into a fresh struct — reusing
		// uptimeHost would leak its BeforeCreate-assigned (never persisted) ID
		// into the query as an extra primary-key predicate.
		var winner models.UptimeHost
		if err := s.DB.Where("host = ?", host).First(&winner).Error; err != nil {
			logger.Log().WithError(err).WithField("host", util.SanitizeForLog(host)).Error("Failed to load UptimeHost after insert conflict")
			return ""
		}
		return winner.ID
	}

	logger.Log().WithField("host_id", uptimeHost.ID).WithField("host", util.SanitizeForLog(uptimeHost.Host)).Info("Created UptimeHost")
	return uptimeHost.ID
}

// CheckAll triggers a check of every enabled host and monitor. It is the target
// of POST /api/v1/system/uptime/check and a handful of tests.
//
// With a live worker pool (production, C5+) it enqueues one JobHostCheck per
// UptimeHost and one JobMonitorCheck per enabled monitor onto the bounded queue
// and returns (enqueued, dropped) counts (spec §3.1.5 / N5) — no goroutine
// fan-out, no direct writes. Without a pool it falls back to a synchronous
// inline sweep (host pre-checks then monitor checks, no goroutines) so legacy
// tests keep working.
func (s *UptimeService) CheckAll() (enqueued, dropped int) {
	if s.Pool != nil {
		return s.enqueueAllChecks()
	}
	return s.checkAllInline()
}

// enqueueAllChecks is the pool-backed CheckAll: every host + every enabled
// monitor onto the bounded queue, drop-on-full counted.
func (s *UptimeService) enqueueAllChecks() (enqueued, dropped int) {
	var hosts []models.UptimeHost
	if err := s.DB.Find(&hosts).Error; err != nil {
		logger.Log().WithError(err).Error("CheckAll: failed to fetch uptime hosts")
	}
	for i := range hosts {
		h := hosts[i]
		if s.Pool.TryEnqueue(UptimeJob{Kind: JobHostCheck, Host: &h}) {
			enqueued++
		} else {
			dropped++
		}
	}

	var monitors []models.UptimeMonitor
	if err := s.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		logger.Log().WithError(err).Error("CheckAll: failed to fetch monitors")
		return enqueued, dropped
	}
	for _, m := range monitors {
		if s.Pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck, Monitor: m, Manual: true}) {
			enqueued++
		} else {
			dropped++
		}
	}
	return enqueued, dropped
}

// checkAllInline is the no-pool fallback: synchronous host pre-checks, then a
// synchronous per-monitor sweep with the same host-down short-circuit the legacy
// path had, minus the unbounded goroutine fan-out.
func (s *UptimeService) checkAllInline() (checked, dropped int) {
	s.checkAllHosts()

	var monitors []models.UptimeMonitor
	if err := s.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to fetch monitors")
		return 0, 0
	}

	hostMonitors := make(map[string][]models.UptimeMonitor)
	for _, monitor := range monitors {
		hostID := ""
		if monitor.UptimeHostID != nil {
			hostID = *monitor.UptimeHostID
		}
		hostMonitors[hostID] = append(hostMonitors[hostID], monitor)
	}

	for hostID, monitors := range hostMonitors {
		if hostID != "" {
			var uptimeHost models.UptimeHost
			if err := s.DB.Where("id = ?", hostID).First(&uptimeHost).Error; err == nil && uptimeHost.Status == "down" {
				tcpMonitors := make([]models.UptimeMonitor, 0, len(monitors))
				nonTCPMonitors := make([]models.UptimeMonitor, 0, len(monitors))
				for _, monitor := range monitors {
					if strings.ToLower(strings.TrimSpace(monitor.Type)) == "tcp" {
						tcpMonitors = append(tcpMonitors, monitor)
						continue
					}
					nonTCPMonitors = append(nonTCPMonitors, monitor)
				}
				if len(tcpMonitors) > 0 {
					s.markHostMonitorsDown(tcpMonitors, &uptimeHost)
				}
				for _, monitor := range nonTCPMonitors {
					s.checkMonitor(monitor)
				}
				checked += len(monitors)
				continue
			}
		}
		for _, monitor := range monitors {
			s.checkMonitor(monitor)
		}
		checked += len(monitors)
	}
	return checked, 0
}

// checkAllHosts performs TCP connectivity check on all UptimeHosts
func (s *UptimeService) checkAllHosts() {
	var hosts []models.UptimeHost
	if err := s.DB.Find(&hosts).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to fetch uptime hosts")
		return
	}

	if len(hosts) == 0 {
		return
	}

	logger.Log().WithField("host_count", len(hosts)).Info("Starting host checks")

	// Create context with timeout for all checks
	ctx, cancel := context.WithTimeout(context.Background(), s.config.CheckTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		// Staggered startup to reduce load spikes
		if i > 0 {
			time.Sleep(s.config.StaggerDelay)
		}
		go func(host *models.UptimeHost) {
			defer wg.Done()
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				logger.Log().WithField("host_name", host.Name).Warn("Host check cancelled due to timeout")
				return
			default:
				s.checkHost(ctx, host)
			}
		}(&hosts[i])
	}
	wg.Wait() // Wait for all host checks to complete

	logger.Log().WithField("host_count", len(hosts)).Info("All host checks completed")
}

// checkHost performs a basic TCP connectivity check to determine if the host is reachable
func (s *UptimeService) checkHost(ctx context.Context, host *models.UptimeHost) {
	// Get host-specific mutex to prevent concurrent database updates
	s.hostMutexLock.Lock()
	if s.hostMutexes[host.ID] == nil {
		s.hostMutexes[host.ID] = &sync.Mutex{}
	}
	mutex := s.hostMutexes[host.ID]
	s.hostMutexLock.Unlock()

	mutex.Lock()
	defer mutex.Unlock()

	start := time.Now()

	logger.Log().WithFields(map[string]any{
		"host_name": host.Name,
		"host_ip":   host.Host,
		"host_id":   host.ID,
	}).Debug("Starting TCP check for host")

	// Get common ports for this host from its monitors
	var monitors []models.UptimeMonitor
	s.DB.Preload("ProxyHost").Where("uptime_host_id = ?", host.ID).Find(&monitors)

	logger.Log().WithField("host_name", host.Name).WithField("monitor_count", len(monitors)).Debug("Retrieved monitors for host")

	if len(monitors) == 0 {
		return
	}

	// Fast-path: if every monitor for this host is Orthrus-type, skip the
	// TCP pre-check entirely — individual checkMonitor calls determine status.
	hasDialable := false
	for _, m := range monitors {
		if strings.ToLower(m.Type) != "orthrus" {
			hasDialable = true
			break
		}
	}
	if !hasDialable {
		return
	}

	// Track whether any non-Orthrus monitor with a valid port was attempted.
	attempted := false
	success := false
	var msg string
	var lastErr error

	// Single non-blocking dial per candidate port — no in-call sleep-retry loop
	// (spec §3.2.3 de-blocking). The cross-cycle FailureThreshold/FailureCount
	// debounce below is unchanged: a host still needs FailureThreshold
	// consecutive failed check cycles to flip to "down". Removing the retry loop
	// only drops up to ~4s (2s x MaxRetries) of blocked goroutine time per down
	// host per cycle.
	select {
	case <-ctx.Done():
		logger.Log().WithField("host_name", host.Name).Warn("TCP check cancelled")
		return
	default:
	}

	// Connect timeout: 3s in production; honour a smaller test-configured
	// TCPTimeout so unit tests stay fast.
	dialTimeout := s.config.TCPTimeout
	if dialTimeout <= 0 || dialTimeout > 3*time.Second {
		dialTimeout = 3 * time.Second
	}

	for _, monitor := range monitors {
		// Orthrus liveness is checked per-monitor via session state, not TCP pre-check.
		if strings.ToLower(monitor.Type) == "orthrus" {
			continue
		}

		var port string
		if monitor.ProxyHost != nil {
			// Use actual backend port from ProxyHost if available.
			port = fmt.Sprintf("%d", monitor.ProxyHost.ForwardPort)
		} else {
			// Fallback to extracting from URL for standalone monitors.
			port = extractPort(monitor.URL)
		}
		if port == "" {
			continue
		}

		attempted = true
		addr := net.JoinHostPort(host.Host, port) // net.JoinHostPort for IPv6 compatibility
		dialer := net.Dialer{Timeout: dialTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Warn("failed to close tcp connection")
			}
			success = true
			msg = fmt.Sprintf("TCP connection to %s successful", addr)
			break
		}
		lastErr = err
		msg = fmt.Sprintf("TCP check failed: %v", err)
	}

	// If every monitor for this host is Orthrus-type, there are no dialable ports.
	// Skip the TCP pre-check; individual checkMonitor() calls determine status.
	if !attempted {
		return
	}

	latency := time.Since(start).Milliseconds()
	oldStatus := host.Status
	var newStatus string

	// Implement failure count debouncing
	if success {
		host.FailureCount = 0
		newStatus = "up"
	} else {
		host.FailureCount++
		if host.FailureCount >= s.config.FailureThreshold {
			newStatus = "down"
		} else {
			// Keep current status on first failure
			newStatus = host.Status
			logger.Log().WithFields(map[string]any{
				"host_name":     host.Name,
				"failure_count": host.FailureCount,
				"threshold":     s.config.FailureThreshold,
				"last_error":    lastErr,
			}).Warn("Host check failed, waiting for threshold")
		}
	}

	statusChanged := oldStatus != newStatus && oldStatus != "pending"

	host.Status = newStatus
	host.LastCheck = time.Now()
	host.Latency = latency

	if statusChanged {
		host.LastStatusChange = time.Now()
		logger.Log().WithFields(map[string]any{
			"host_name": host.Name,
			"host_ip":   host.Host,
			"old":       oldStatus,
			"new":       newStatus,
			"message":   msg,
		}).Info("Host status changed")
	}

	logger.Log().WithFields(map[string]any{
		"host_name":      host.Name,
		"host_ip":        host.Host,
		"success":        success,
		"failure_count":  host.FailureCount,
		"old_status":     oldStatus,
		"new_status":     newStatus,
		"elapsed_ms":     latency,
		"status_changed": statusChanged,
	}).Debug("Host TCP check completed")

	// Persist through the single ingester write path (no direct s.DB.Save here
	// any more — spec §3.1.5). The in-memory *host is still mutated above so
	// callers that inspect the passed struct keep working.
	if err := s.Ingester.FlushResults(HostCheckResult{
		HostID:          host.ID,
		Status:          host.Status,
		FailureCount:    host.FailureCount,
		Latency:         host.Latency,
		Message:         msg,
		CheckedAt:       host.LastCheck,
		StatusChanged:   statusChanged,
		StatusChangedAt: statusChangedAt(statusChanged, host.LastStatusChange),
	}); err != nil {
		logger.Log().WithError(err).WithField("host_id", host.ID).
			Error("checkHost: failed to persist host check result")
	}
}

// markHostMonitorsDown marks all monitors for a down host as down and sends a single notification
func (s *UptimeService) markHostMonitorsDown(monitors []models.UptimeMonitor, host *models.UptimeHost) {
	downMonitors := []monitorDownInfo{}

	for i := range monitors {
		monitor := &monitors[i]
		oldStatus := monitor.Status
		if oldStatus == "down" {
			continue // Already down, no need to update
		}

		// Calculate previous uptime
		var durationStr string
		if !monitor.LastStatusChange.IsZero() {
			duration := time.Since(monitor.LastStatusChange)
			durationStr = formatDuration(duration)
		}

		monitor.Status = "down"
		monitor.LastCheck = time.Now()
		monitor.FailureCount = monitor.MaxRetries // Max out failure count
		if oldStatus != "pending" {
			monitor.LastStatusChange = time.Now()
		}
		monitor.NotifiedInBatch = true
		s.DB.Save(monitor)

		// Record heartbeat
		heartbeat := models.UptimeHeartbeat{
			MonitorID: monitor.ID,
			Status:    "down",
			Latency:   0,
			Message:   "Host unreachable",
		}
		s.DB.Create(&heartbeat)

		if oldStatus != "pending" && oldStatus != "down" {
			downMonitors = append(downMonitors, monitorDownInfo{
				ID:             monitor.ID,
				Name:           monitor.Name,
				URL:            monitor.URL,
				Message:        "Host unreachable",
				PreviousUptime: durationStr,
			})
		}
	}

	// Send consolidated notification if any monitors transitioned to down
	if len(downMonitors) > 0 && time.Since(host.LastNotifiedDown) > 5*time.Minute {
		s.sendHostDownNotification(host, downMonitors)
	}
}

// sendHostDownNotification sends a single consolidated notification for a down host
func (s *UptimeService) sendHostDownNotification(host *models.UptimeHost, downMonitors []monitorDownInfo) {
	title := fmt.Sprintf("🔴 Host %s is DOWN (%d services affected)", host.Name, len(downMonitors))

	var sb strings.Builder
	fmt.Fprintf(&sb, "Host: %s (%s)\n", host.Name, host.Host)
	fmt.Fprintf(&sb, "Time: %s\n", time.Now().Format(time.RFC1123))
	fmt.Fprintf(&sb, "Services affected: %d\n\n", len(downMonitors))

	sb.WriteString("Impacted services:\n")
	for _, m := range downMonitors {
		if m.PreviousUptime != "" {
			fmt.Fprintf(&sb, "• %s (was up %s)\n", m.Name, m.PreviousUptime)
		} else {
			fmt.Fprintf(&sb, "• %s\n", m.Name)
		}
	}

	// Store notification in DB
	_, _ = s.NotificationService.Create(
		models.NotificationTypeError,
		title,
		sb.String(),
	)

	// Collect monitor IDs for tracking
	monitorIDs := make([]string, len(downMonitors))
	for i, m := range downMonitors {
		monitorIDs[i] = m.ID
	}
	monitorIDsJSON, _ := json.Marshal(monitorIDs)

	// Record notification event
	event := models.UptimeNotificationEvent{
		HostID:     host.ID,
		EventType:  "down",
		MonitorIDs: string(monitorIDsJSON),
		Message:    sb.String(),
		SentAt:     time.Now(),
	}
	s.DB.Create(&event)

	// Update host notification tracking
	host.LastNotifiedDown = time.Now()
	host.NotifiedServiceCount = len(downMonitors)
	s.DB.Save(host)

	// Send external notification
	data := map[string]any{
		"HostName":     host.Name,
		"HostIP":       host.Host,
		"Status":       "DOWN",
		"ServiceCount": len(downMonitors),
		"Services":     downMonitors,
		"Time":         time.Now().Format(time.RFC1123),
	}
	s.NotificationService.SendExternal(context.Background(), "uptime", title, sb.String(), data)

	logger.Log().WithField("host_name", host.Name).WithField("service_count", len(downMonitors)).Info("Sent consolidated DOWN notification")
}

// CheckMonitor runs one monitor check. When a live worker pool is wired
// (production, C5 onward) the check is enqueued onto the bounded pool; otherwise
// it runs inline via checkMonitor. Callers use it as a fire-and-forget trigger.
func (s *UptimeService) CheckMonitor(monitor models.UptimeMonitor) {
	if s.Pool != nil {
		s.Pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck, Monitor: monitor, Manual: true})
		return
	}
	s.checkMonitor(monitor)
}

// checkMonitor is the synchronous inline probe path used when no worker pool is
// running (unit tests, legacy fallback). It runs the shared probe, resolves the
// transition against this monitor's persisted debounce columns, and persists the
// outcome through the single ingester write path — there is deliberately no
// direct s.DB.Create(&heartbeat) / s.DB.Save(&monitor) here any more (spec
// §3.1.5, N3 collapsed: the probe switch lives only in uptime_check.go).
func (s *UptimeService) checkMonitor(monitor models.UptimeMonitor) {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(clampInterval(monitor.Interval, s.uptimeCfg))*time.Second)
	defer cancel()

	raw := s.checker.probe(ctx, monitor)
	now := time.Now()

	maxRetries := monitor.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // legacy rows
	}

	prev := monDebounce{
		status:           monitor.Status,
		failureCount:     monitor.FailureCount,
		lastStatusChange: monitor.LastStatusChange,
		lastNotifiedDown: monitor.LastNotifiedDown,
	}
	if prev.status == "" {
		prev.status = "pending"
	}
	next, changed, durationStr := applyMonitorDebounce(prev, raw, maxRetries, now)

	result := CheckResult{
		MonitorID:        monitor.ID,
		HostID:           derefString(monitor.UptimeHostID),
		HeartbeatStatus:  raw.heartbeatStatus(),
		Latency:          raw.Latency,
		Message:          raw.Message,
		CheckedAt:        now,
		NewMonitorStatus: next.status,
		FailureCount:     next.failureCount,
		StatusChanged:    changed,
		StatusChangedAt:  statusChangedAt(changed, now),
	}
	if err := s.Ingester.FlushResults(result); err != nil {
		logger.Log().WithError(err).WithField("monitor_id", monitor.ID).
			Error("checkMonitor: failed to persist check result")
	}

	if changed {
		switch next.status {
		case "down":
			s.queueDownNotification(monitor, raw.Message, durationStr)
		case "up":
			s.sendRecoveryNotification(monitor, durationStr)
		}
	}
}

// queueDownNotification adds a down monitor to the batch queue
func (s *UptimeService) queueDownNotification(monitor models.UptimeMonitor, reason, previousUptime string) {
	s.notificationMutex.Lock()
	defer s.notificationMutex.Unlock()

	hostID := ""
	if monitor.UptimeHostID != nil {
		hostID = *monitor.UptimeHostID
	}

	// Get host info
	var uptimeHost models.UptimeHost
	hostName := monitor.UpstreamHost
	if hostID != "" {
		if err := s.DB.Where("id = ?", hostID).First(&uptimeHost).Error; err == nil {
			hostName = uptimeHost.Name
		}
	}

	info := monitorDownInfo{
		ID:             monitor.ID,
		Name:           monitor.Name,
		URL:            monitor.URL,
		Message:        reason,
		PreviousUptime: previousUptime,
	}

	if pending, exists := s.pendingNotifications[hostID]; exists {
		// Add to existing batch
		pending.downMonitors = append(pending.downMonitors, info)
		logger.Log().WithField("monitor", util.SanitizeForLog(monitor.Name)).WithField("host", util.SanitizeForLog(hostName)).WithField("count", len(pending.downMonitors)).Info("Added to pending notification batch")
	} else {
		// Create new batch with timer
		pending := &pendingHostNotification{
			hostID:       hostID,
			hostName:     hostName,
			downMonitors: []monitorDownInfo{info},
			createdAt:    time.Now(),
		}

		pending.timer = time.AfterFunc(s.batchWindow, func() {
			s.flushPendingNotification(hostID)
		})

		s.pendingNotifications[hostID] = pending
		logger.Log().WithField("host", util.SanitizeForLog(hostName)).WithField("monitor", util.SanitizeForLog(monitor.Name)).Info("Created pending notification batch")
	}
}

// flushPendingNotification sends the batched notification
func (s *UptimeService) flushPendingNotification(hostID string) {
	s.notificationMutex.Lock()
	pending, exists := s.pendingNotifications[hostID]
	if !exists {
		s.notificationMutex.Unlock()
		return
	}
	delete(s.pendingNotifications, hostID)
	s.notificationMutex.Unlock()

	if pending.timer != nil {
		pending.timer.Stop()
	}

	if len(pending.downMonitors) == 0 {
		return
	}

	// Build and send notification
	var title string
	var sb strings.Builder

	if len(pending.downMonitors) == 1 {
		// Single service down
		m := pending.downMonitors[0]
		title = fmt.Sprintf("🔴 %s is DOWN", m.Name)
		fmt.Fprintf(&sb, "Service: %s\n", m.Name)
		sb.WriteString("Status: DOWN\n")
		fmt.Fprintf(&sb, "Time: %s\n", time.Now().Format(time.RFC1123))
		if m.PreviousUptime != "" {
			fmt.Fprintf(&sb, "Previous Uptime: %s\n", m.PreviousUptime)
		}
		fmt.Fprintf(&sb, "Reason: %s\n", m.Message)
	} else {
		// Multiple services down
		title = fmt.Sprintf("🔴 %d Services DOWN on %s", len(pending.downMonitors), pending.hostName)
		fmt.Fprintf(&sb, "Host: %s\n", pending.hostName)
		fmt.Fprintf(&sb, "Time: %s\n", time.Now().Format(time.RFC1123))
		fmt.Fprintf(&sb, "Services affected: %d\n\n", len(pending.downMonitors))

		sb.WriteString("Impacted services:\n")
		for _, m := range pending.downMonitors {
			if m.PreviousUptime != "" {
				fmt.Fprintf(&sb, "• %s - %s (was up %s)\n", m.Name, m.Message, m.PreviousUptime)
			} else {
				fmt.Fprintf(&sb, "• %s - %s\n", m.Name, m.Message)
			}
		}
	}

	// Store in DB
	_, _ = s.NotificationService.Create(
		models.NotificationTypeError,
		title,
		sb.String(),
	)

	// Send external
	data := map[string]any{
		"HostName":     pending.hostName,
		"Status":       "DOWN",
		"ServiceCount": len(pending.downMonitors),
		"Services":     pending.downMonitors,
		"Time":         time.Now().Format(time.RFC1123),
	}
	s.NotificationService.SendExternal(context.Background(), "uptime", title, sb.String(), data)

	logger.Log().WithField("count", len(pending.downMonitors)).WithField("host", util.SanitizeForLog(pending.hostName)).Info("Sent batched DOWN notification")
}

// NotifyMonitorDown queues a batched "service down" alert for a monitor whose
// status transition was detected synchronously by the worker pool (spec
// §3.3.3). It is fast and non-blocking (map insert + AfterFunc timer); ctx is
// accepted only for interface symmetry with NotifyMonitorUp.
func (s *UptimeService) NotifyMonitorDown(_ context.Context, monitor models.UptimeMonitor, reason, previousUptime string) {
	s.queueDownNotification(monitor, reason, previousUptime)
}

// NotifyMonitorUp sends a recovery alert. The external webhook send is bounded
// by ctx so a hung notification provider cannot wedge the calling worker — and,
// at shutdown, cannot block the teardown chain past the pool's dispatch
// deadline (supervisor change #1 / spec §3.1.4).
func (s *UptimeService) NotifyMonitorUp(ctx context.Context, monitor models.UptimeMonitor, downtime string) {
	s.sendRecoveryNotificationCtx(ctx, monitor, downtime)
}

// sendRecoveryNotification sends a notification when a service recovers. Legacy
// callers (checkMonitor, this commit) use the background-context wrapper; the
// worker pool calls sendRecoveryNotificationCtx directly with a bounded ctx.
func (s *UptimeService) sendRecoveryNotification(monitor models.UptimeMonitor, downtime string) {
	s.sendRecoveryNotificationCtx(context.Background(), monitor, downtime)
}

func (s *UptimeService) sendRecoveryNotificationCtx(ctx context.Context, monitor models.UptimeMonitor, downtime string) {
	title := fmt.Sprintf("🟢 %s is UP", monitor.Name)

	var sb strings.Builder
	fmt.Fprintf(&sb, "Service: %s\n", monitor.Name)
	sb.WriteString("Status: UP\n")
	fmt.Fprintf(&sb, "Time: %s\n", time.Now().Format(time.RFC1123))
	if downtime != "" {
		fmt.Fprintf(&sb, "Downtime: %s\n", downtime)
	}

	_, _ = s.NotificationService.Create(
		models.NotificationTypeSuccess,
		title,
		sb.String(),
	)

	data := map[string]any{
		"Name":     monitor.Name,
		"Status":   "UP",
		"Downtime": downtime,
		"Time":     time.Now().Format(time.RFC1123),
		"URL":      monitor.URL,
	}
	s.NotificationService.SendExternal(ctx, "uptime", title, sb.String(), data)
}

// FlushPendingNotifications flushes all pending batched notifications immediately.
// This is useful for testing and graceful shutdown.
func (s *UptimeService) FlushPendingNotifications() {
	s.notificationMutex.Lock()
	pendingHostIDs := make([]string, 0, len(s.pendingNotifications))
	for hostID := range s.pendingNotifications {
		pendingHostIDs = append(pendingHostIDs, hostID)
	}
	s.notificationMutex.Unlock()

	for _, hostID := range pendingHostIDs {
		s.flushPendingNotification(hostID)
	}
}

// SyncMonitorForHost updates the uptime monitor linked to a specific proxy host.
// This should be called when a proxy host is edited to keep the monitor in sync.
// Returns nil if no monitor exists for the host (does not create one).
func (s *UptimeService) SyncMonitorForHost(hostID uint) error {
	var host models.ProxyHost
	if err := s.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		return err
	}

	var monitor models.UptimeMonitor
	if err := s.DB.Where("proxy_host_id = ?", hostID).First(&monitor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // No monitor to sync
		}
		return err
	}

	// Update monitor fields based on current proxy host values
	domains := strings.Split(host.DomainNames, ",")
	firstDomain := ""
	if len(domains) > 0 {
		firstDomain = strings.TrimSpace(domains[0])
	}

	scheme := "http"
	if host.SSLForced {
		scheme = "https"
	}

	newName := host.Name
	if newName == "" {
		newName = firstDomain
	}

	monitor.Name = newName
	monitor.URL = fmt.Sprintf("%s://%s", scheme, firstDomain)
	monitor.UpstreamHost = host.ForwardHost

	return s.DB.Save(&monitor).Error
}

// CRUD for Monitors

func (s *UptimeService) ListMonitors() ([]models.UptimeMonitor, error) {
	var monitors []models.UptimeMonitor
	result := s.DB.Order("name ASC").Find(&monitors)
	return monitors, result.Error
}

// CreateMonitor creates a new uptime monitor with the given parameters
func (s *UptimeService) CreateMonitor(name, urlStr, monitorType string, interval, maxRetries int) (*models.UptimeMonitor, error) {
	// Validate URL format
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// For HTTP/HTTPS, ensure the scheme is present
	if monitorType == "http" || monitorType == "https" {
		if parsedURL.Scheme == "" {
			return nil, errors.New("URL must include scheme (http:// or https://)")
		}
		if parsedURL.Host == "" {
			return nil, errors.New("URL must include host")
		}
	}

	// For TCP, validate host:port format
	if monitorType == "tcp" {
		if _, _, err := net.SplitHostPort(urlStr); err != nil {
			return nil, fmt.Errorf("TCP URL must be in host:port format: %w", err)
		}
	}

	// Resolve the interval at write time so the stored value is always
	// concrete and >= the 30s floor: <=0 becomes uptime.default_interval_seconds,
	// then anything below 30 is raised to 30 (spec §3.6.3 / S3).
	interval = clampInterval(interval, s.uptimeCfg)
	if maxRetries <= 0 {
		maxRetries = 3 // Default 3 retries
	}

	monitor := &models.UptimeMonitor{
		Name:       name,
		URL:        urlStr,
		Type:       monitorType,
		Interval:   interval,
		MaxRetries: maxRetries,
		Enabled:    true,
		Status:     "pending",
	}

	if err := s.DB.Create(monitor).Error; err != nil {
		return nil, fmt.Errorf("failed to create monitor: %w", err)
	}

	logger.Log().WithFields(map[string]any{
		"monitor_id":   monitor.ID,
		"monitor_name": util.SanitizeForLog(monitor.Name),
		"monitor_type": util.SanitizeForLog(monitor.Type),
	}).Info("Created new uptime monitor")

	return monitor, nil
}

func (s *UptimeService) GetMonitorByID(id string) (*models.UptimeMonitor, error) {
	var monitor models.UptimeMonitor
	if err := s.DB.Where("id = ?", id).First(&monitor).Error; err != nil {
		return nil, err
	}
	return &monitor, nil
}

// uptimeHistoryDefaultLimit / uptimeHistoryMaxLimit bound the detail-view
// history query. A non-positive limit falls back to the default; anything above
// the cap is clamped (spec §3.5.4).
const (
	uptimeHistoryDefaultLimit = 60
	uptimeHistoryMaxLimit     = 500
)

// GetMonitorHistory returns a monitor's heartbeats newest-first. limit is
// clamped to (0, uptimeHistoryMaxLimit]; a non-positive limit uses
// uptimeHistoryDefaultLimit. A non-zero before acts as a "load older" cursor:
// only heartbeats with created_at < before are returned.
func (s *UptimeService) GetMonitorHistory(id string, limit int, before time.Time) ([]models.UptimeHeartbeat, error) {
	switch {
	case limit <= 0:
		limit = uptimeHistoryDefaultLimit
	case limit > uptimeHistoryMaxLimit:
		limit = uptimeHistoryMaxLimit
	}

	query := s.DB.Where("monitor_id = ?", id)
	if !before.IsZero() {
		query = query.Where("created_at < ?", before)
	}

	var heartbeats []models.UptimeHeartbeat
	result := query.Order("created_at desc").Limit(limit).Find(&heartbeats)
	return heartbeats, result.Error
}

func (s *UptimeService) UpdateMonitor(id string, updates map[string]any) (*models.UptimeMonitor, error) {
	var monitor models.UptimeMonitor
	if err := s.DB.Where("id = ?", id).First(&monitor).Error; err != nil {
		return nil, err
	}

	// Whitelist allowed fields to update
	allowedUpdates := make(map[string]any)
	if val, ok := updates["max_retries"]; ok {
		allowedUpdates["max_retries"] = val
	}
	if val, ok := updates["interval"]; ok {
		// Enforce the 30s hard floor. A positive but sub-floor value is a
		// client error (rejected); a non-positive value is left for the
		// scheduler's clampInterval to resolve to the configured default.
		if secs, parsed := coerceIntervalSeconds(val); parsed && secs > 0 && secs < minUptimeIntervalSeconds {
			return nil, ErrIntervalTooLow
		}
		allowedUpdates["interval"] = val
	}
	if val, ok := updates["enabled"]; ok {
		allowedUpdates["enabled"] = val
	}
	// Add other fields as needed, but be careful not to overwrite SyncMonitors logic

	if err := s.DB.Model(&monitor).Updates(allowedUpdates).Error; err != nil {
		return nil, err
	}

	return &monitor, nil
}

// DeleteMonitor removes a monitor and its heartbeats, and optionally cleans up the parent UptimeHost.
func (s *UptimeService) DeleteMonitor(id string) error {
	// Find monitor
	var monitor models.UptimeMonitor
	if err := s.DB.Where("id = ?", id).First(&monitor).Error; err != nil {
		return err
	}

	// Delete heartbeats
	if err := s.DB.Where("monitor_id = ?", id).Delete(&models.UptimeHeartbeat{}).Error; err != nil {
		return err
	}

	// Delete the monitor
	if err := s.DB.Delete(&monitor).Error; err != nil {
		return err
	}

	// If no other monitors reference the uptime host, we don't automatically delete the host.
	// Leave host cleanup to a manual process or separate endpoint.

	return nil
}

// SyncAndCheckForHost creates a monitor for the given proxy host (if one
// doesn't already exist) and immediately triggers a health check in a
// background goroutine. It is safe to call from any goroutine.
//
// Designed to be called as `go svc.SyncAndCheckForHost(hostID)` so it
// does not block the API response.
func (s *UptimeService) SyncAndCheckForHost(hostID uint) {
	// Check feature flag — bail if uptime is disabled
	if !s.uptimeFeatureEnabled() {
		return
	}

	// Per-host lock prevents duplicate monitors when multiple goroutines
	// call SyncAndCheckForHost for the same hostID concurrently.
	hostKey := fmt.Sprintf("proxy-%d", hostID)
	s.hostMutexLock.Lock()
	if s.hostMutexes[hostKey] == nil {
		s.hostMutexes[hostKey] = &sync.Mutex{}
	}
	mu := s.hostMutexes[hostKey]
	s.hostMutexLock.Unlock()

	mu.Lock()
	defer mu.Unlock()

	// Look up the proxy host; it may have been deleted between the API
	// response and this goroutine executing.
	var host models.ProxyHost
	if err := s.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		hostIDStr := strconv.FormatUint(uint64(hostID), 10)
		logger.Log().WithField("host_id", hostIDStr).Debug("SyncAndCheckForHost: proxy host not found (may have been deleted)")
		return
	}

	// Ensure a monitor exists for this host
	var monitor models.UptimeMonitor
	err := s.DB.Where("proxy_host_id = ?", host.ID).First(&monitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		domains := strings.Split(host.DomainNames, ",")
		firstDomain := ""
		if len(domains) > 0 {
			firstDomain = strings.TrimSpace(domains[0])
		}

		scheme := "http"
		if host.SSLForced {
			scheme = "https"
		}
		publicURL := fmt.Sprintf("%s://%s", scheme, firstDomain)
		upstreamHost := host.ForwardHost

		name := host.Name
		if name == "" {
			name = firstDomain
		}

		uptimeHostID := s.ensureUptimeHost(upstreamHost, name)

		monitor = models.UptimeMonitor{
			ProxyHostID:  &host.ID,
			UptimeHostID: &uptimeHostID,
			Name:         name,
			Type:         "http",
			URL:          publicURL,
			UpstreamHost: upstreamHost,
			Interval:     clampInterval(0, s.uptimeCfg), // honour uptime.default_interval_seconds (S3)
			Enabled:      true,
			Status:       "pending",
		}
		if createErr := s.createMonitorWithRetry(&monitor); createErr != nil {
			logger.Log().WithError(createErr).WithField("host_id", host.ID).Error("SyncAndCheckForHost: failed to create monitor")
			return
		}
	} else if err != nil {
		logger.Log().WithError(err).WithField("host_id", host.ID).Error("SyncAndCheckForHost: failed to query monitor")
		return
	}

	// Run an immediate check: enqueue host + monitor jobs onto the pool when it
	// is live, otherwise probe inline.
	s.checkOrEnqueue(monitor)
}

// checkOrEnqueue runs a monitor check the right way for the current wiring:
// enqueue onto the live worker pool (also enqueueing its parent host so the
// connectivity state is fresh), or probe inline when no pool is running.
func (s *UptimeService) checkOrEnqueue(monitor models.UptimeMonitor) {
	if s.Pool == nil {
		s.checkMonitor(monitor)
		return
	}
	if monitor.UptimeHostID != nil && *monitor.UptimeHostID != "" {
		var host models.UptimeHost
		if err := s.DB.Where("id = ?", *monitor.UptimeHostID).First(&host).Error; err == nil {
			s.Pool.TryEnqueue(UptimeJob{Kind: JobHostCheck, Host: &host})
		}
	}
	s.Pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck, Monitor: monitor, Manual: true})
}

// SyncAndCheckForRemoteServer ensures an uptime monitor exists for the given
// remote server (creating it with the admin-configured default interval if
// missing) and triggers an immediate check. Mirrors SyncAndCheckForHost.
// Designed to be called as `go svc.SyncAndCheckForRemoteServer(id)`.
//
// Orthrus remote servers whose agent UUID has not yet bound return silently
// with no monitor row (mirrors SyncMonitors' existing `continue`); the
// 5-minute UptimeSyncLoop creates the monitor on a later pass once the UUID
// is persisted.
func (s *UptimeService) SyncAndCheckForRemoteServer(remoteServerID uint) {
	if !s.uptimeFeatureEnabled() {
		return
	}

	key := fmt.Sprintf("remote-%d", remoteServerID)
	s.hostMutexLock.Lock()
	if s.hostMutexes[key] == nil {
		s.hostMutexes[key] = &sync.Mutex{}
	}
	mu := s.hostMutexes[key]
	s.hostMutexLock.Unlock()

	mu.Lock()
	defer mu.Unlock()

	var server models.RemoteServer
	if err := s.DB.Where("id = ?", remoteServerID).First(&server).Error; err != nil {
		// Safe: remote_server_id is a uint (DB/route numeric ID), not an injectable string.
		// codeql[go/log-injection]
		logger.Log().WithField("remote_server_id", remoteServerID).
			Debug("SyncAndCheckForRemoteServer: remote server not found (may have been deleted)")
		return
	}

	targetType, targetURL, ok := remoteServerMonitorTarget(server)
	if !ok {
		return // Orthrus agent UUID not yet bound — nothing to check yet.
	}

	var monitor models.UptimeMonitor
	err := s.DB.Where("remote_server_id = ?", server.ID).First(&monitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		uptimeHostID := s.ensureUptimeHost(server.Host, server.Name)
		monitor = models.UptimeMonitor{
			RemoteServerID: &server.ID,
			UptimeHostID:   &uptimeHostID,
			Name:           server.Name,
			Type:           targetType,
			URL:            targetURL,
			UpstreamHost:   server.Host,
			Interval:       clampInterval(0, s.uptimeCfg), // honour uptime.default_interval_seconds (S3)
			Enabled:        server.Enabled,
			Status:         "pending",
		}
		if createErr := s.createMonitorWithRetry(&monitor); createErr != nil {
			logger.Log().WithError(createErr).WithField("remote_server_id", server.ID).
				Error("SyncAndCheckForRemoteServer: failed to create monitor")
			return
		}
	} else if err != nil {
		logger.Log().WithError(err).WithField("remote_server_id", server.ID).
			Error("SyncAndCheckForRemoteServer: failed to query monitor")
		return
	}

	s.checkOrEnqueue(monitor)
}

// SyncMonitorForRemoteServer updates the uptime monitor linked to a remote
// server from its current fields. No-op (nil) when no monitor exists.
func (s *UptimeService) SyncMonitorForRemoteServer(remoteServerID uint) error {
	var server models.RemoteServer
	if err := s.DB.Where("id = ?", remoteServerID).First(&server).Error; err != nil {
		return fmt.Errorf("load remote server %d: %w", remoteServerID, err)
	}

	var monitor models.UptimeMonitor
	if err := s.DB.Where("remote_server_id = ?", remoteServerID).First(&monitor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("load monitor for remote server %d: %w", remoteServerID, err)
	}

	targetType, targetURL, ok := remoteServerMonitorTarget(server)
	if !ok {
		return nil // Orthrus agent UUID not yet bound.
	}

	monitor.Name = server.Name
	monitor.Type = targetType
	monitor.URL = targetURL
	monitor.UpstreamHost = server.Host
	monitor.Enabled = server.Enabled

	if err := s.DB.Save(&monitor).Error; err != nil {
		return fmt.Errorf("save monitor for remote server %d: %w", remoteServerID, err)
	}
	return nil
}

// remoteServerMonitorTarget derives the (type, url) an uptime monitor should
// use for a remote server — the same rules SyncMonitors applies. ok is false
// only for an Orthrus server with no bound agent UUID.
func remoteServerMonitorTarget(server models.RemoteServer) (monitorType, monitorURL string, ok bool) {
	if server.ConnectionType == models.ConnectionTypeOrthrus {
		if server.OrthrusAgentUUID == nil || *server.OrthrusAgentUUID == "" {
			return "", "", false
		}
		return "orthrus", *server.OrthrusAgentUUID, true
	}
	if server.Scheme == "http" || server.Scheme == "https" {
		return server.Scheme, fmt.Sprintf("%s://%s:%d", server.Scheme, server.Host, server.Port), true
	}
	return "tcp", fmt.Sprintf("%s:%d", server.Host, server.Port), true
}

// createMonitorWithRetry creates an UptimeMonitor row, retrying with backoff
// on transient SQLite lock errors rather than treating any lock conflict as
// permanent and silently dropping the monitor. This mirrors the established
// retry convention used elsewhere in this codebase for the same class of
// error -- see credential_service.go's Delete and security_service.go's
// persistAuditWithRetry -- rather than introducing a new pattern.
//
// Production's single-connection pool (SetMaxOpenConns(1), see
// internal/database/database.go's configurePool) makes this error unlikely
// there, but this still defends against transient contention within a
// single connection's checkout window.
func (s *UptimeService) createMonitorWithRetry(monitor *models.UptimeMonitor) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = s.DB.Create(monitor).Error
		if lastErr == nil {
			return nil
		}

		errMsg := strings.ToLower(lastErr.Error())
		isTransientLock := strings.Contains(errMsg, "database is locked") || strings.Contains(errMsg, "database table is locked") || strings.Contains(errMsg, "busy")
		if !isTransientLock || attempt == maxAttempts {
			return lastErr
		}

		time.Sleep(time.Duration(attempt) * 10 * time.Millisecond)
	}

	return lastErr
}

// CleanupStaleFailureCounts resets monitors that are stuck in "down" status
// with elevated failure counts from historical bugs (e.g., port mismatch era).
// Only resets monitors with no recent successful heartbeat in the last 24 hours.
func (s *UptimeService) CleanupStaleFailureCounts() error {
	result := s.DB.Exec(`
		UPDATE uptime_monitors SET failure_count = 0, status = 'pending'
		WHERE status = 'down'
		  AND failure_count > 5
		  AND NOT EXISTS (
		    SELECT 1 FROM uptime_heartbeats
		    WHERE uptime_heartbeats.monitor_id = uptime_monitors.id
		      AND status = 'up'
		      AND created_at > datetime('now', '-24 hours')
		  )
	`)
	if result.Error != nil {
		return fmt.Errorf("cleanup stale failure counts: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Log().WithField("reset_count", result.RowsAffected).Info("Reset stale monitor failure counts")
	}

	return nil
}

// uptimeSyncLoopInterval is how often the off-hot-path monitor reconcile runs.
// SyncMonitors is a backstop for any mutation that missed its targeted hook
// (direct DB edits, Orthrus agent-UUID late-binding); the per-monitor scheduler
// drives the actual checks.
const uptimeSyncLoopInterval = 5 * time.Minute

// UptimeSyncLoop periodically reconciles UptimeMonitor rows against ProxyHost /
// RemoteServer rows. It replaces the SyncMonitors call that used to ride on the
// global 60s check ticker (spec §3.1.3).
type UptimeSyncLoop struct {
	svc  *UptimeService
	tick time.Duration
}

// NewUptimeSyncLoop builds the loop bound to svc.
func NewUptimeSyncLoop(svc *UptimeService) *UptimeSyncLoop {
	return &UptimeSyncLoop{svc: svc, tick: uptimeSyncLoopInterval}
}

// Run ticks every 5 minutes, calling SyncMonitors while the feature is enabled,
// and returns on ctx cancellation. The boot-time SyncMonitors +
// CleanupStaleFailureCounts still run once via runInitialUptimeBootstrap.
func (l *UptimeSyncLoop) Run(ctx context.Context) {
	ticker := time.NewTicker(l.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !l.svc.uptimeFeatureEnabled() {
				continue
			}
			if err := l.svc.SyncMonitors(); err != nil {
				logger.Log().WithError(err).Warn("UptimeSyncLoop: SyncMonitors failed")
			}
		}
	}
}
