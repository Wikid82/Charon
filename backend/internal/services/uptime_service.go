package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/gorm"
)

type UptimeService struct {
	DB                  *gorm.DB
	NotificationService *NotificationService
	// Batching: track pending notifications
	pendingNotifications map[string]*pendingHostNotification
	notificationMutex    sync.Mutex
	batchWindow          time.Duration
	// Host-specific mutexes to prevent concurrent database updates
	hostMutexes   map[string]*sync.Mutex
	hostMutexLock sync.Mutex
	// Configuration
	config UptimeConfig
}

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
	return &UptimeService{
		DB:                   db,
		NotificationService:  ns,
		pendingNotifications: make(map[string]*pendingHostNotification),
		batchWindow:          30 * time.Second, // Wait 30 seconds to batch notifications
		hostMutexes:          make(map[string]*sync.Mutex),
		config: UptimeConfig{
			TCPTimeout:       10 * time.Second,
			MaxRetries:       2,
			FailureThreshold: 2,
			CheckTimeout:     60 * time.Second,
			StaggerDelay:     100 * time.Millisecond,
		},
	}
}

// extractPort extracts the port from a URL or host:port string
func extractPort(urlStr string) string {
	// Try parsing as URL first
	if u, err := url.Parse(urlStr); err == nil && u.Host != "" {
		port := u.Port()
		if port != "" {
			return port
		}
		// Default ports
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
				Interval:     60,
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
				Interval:       60,
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

// ensureUptimeHost finds or creates an UptimeHost for the given host string
func (s *UptimeService) ensureUptimeHost(host, defaultName string) string {
	var uptimeHost models.UptimeHost
	err := s.DB.Where("host = ?", host).First(&uptimeHost).Error

	if err == gorm.ErrRecordNotFound {
		uptimeHost = models.UptimeHost{
			Host:   host,
			Name:   defaultName,
			Status: "pending",
		}
		if err := s.DB.Create(&uptimeHost).Error; err != nil {
			logger.Log().WithError(err).WithField("host", util.SanitizeForLog(host)).Error("Failed to create UptimeHost")
			return ""
		}
		logger.Log().WithField("host_id", uptimeHost.ID).WithField("host", util.SanitizeForLog(uptimeHost.Host)).Info("Created UptimeHost")
	}

	return uptimeHost.ID
}

// CheckAll runs checks for all enabled monitors with host-level pre-check
func (s *UptimeService) CheckAll() {
	// First, check all UptimeHosts
	s.checkAllHosts()

	var monitors []models.UptimeMonitor
	if err := s.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to fetch monitors")
		return
	}

	// Group monitors by UptimeHost
	hostMonitors := make(map[string][]models.UptimeMonitor)
	for _, monitor := range monitors {
		hostID := ""
		if monitor.UptimeHostID != nil {
			hostID = *monitor.UptimeHostID
		}
		hostMonitors[hostID] = append(hostMonitors[hostID], monitor)
	}

	// Check each host's monitors
	for hostID, monitors := range hostMonitors {
		// If host is down, mark all monitors as down without individual checks
		if hostID != "" {
			var uptimeHost models.UptimeHost
			if err := s.DB.Where("id = ?", hostID).First(&uptimeHost).Error; err == nil {
				if uptimeHost.Status == "down" {
					s.markHostMonitorsDown(monitors, &uptimeHost)
					continue
				}
			}
		}

		// Host is up, check individual monitors
		for _, monitor := range monitors {
			go s.checkMonitor(monitor)
		}
	}
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

	// Try to connect to any of the monitor ports with retry logic
	success := false
	var msg string
	var lastErr error

	for retry := 0; retry <= s.config.MaxRetries && !success; retry++ {
		if retry > 0 {
			logger.Log().WithFields(map[string]any{
				"host_name": host.Name,
				"retry":     retry,
				"max":       s.config.MaxRetries,
			}).Info("Retrying TCP check")
			time.Sleep(2 * time.Second) // Brief delay between retries
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			logger.Log().WithField("host_name", host.Name).Warn("TCP check cancelled")
			return
		default:
		}

		for _, monitor := range monitors {
			var port string

			// Use actual backend port from ProxyHost if available
			if monitor.ProxyHost != nil {
				port = fmt.Sprintf("%d", monitor.ProxyHost.ForwardPort)
			} else {
				// Fallback to extracting from URL for standalone monitors
				port = extractPort(monitor.URL)
			}

			if port == "" {
				continue
			}

			logger.Log().WithFields(map[string]any{
				"monitor":        monitor.Name,
				"extracted_port": extractPort(monitor.URL),
				"actual_port":    port,
				"host":           host.Host,
				"retry":          retry,
			}).Debug("TCP check port resolution")

			// Use net.JoinHostPort for IPv6 compatibility
			addr := net.JoinHostPort(host.Host, port)

			// Create dialer with timeout from context
			dialer := net.Dialer{Timeout: s.config.TCPTimeout}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				if closeErr := conn.Close(); closeErr != nil {
					logger.Log().WithError(closeErr).Warn("failed to close tcp connection")
				}
				success = true
				msg = fmt.Sprintf("TCP connection to %s successful (retry %d)", addr, retry)
				logger.Log().WithFields(map[string]any{
					"host_name": host.Name,
					"addr":      addr,
					"retry":     retry,
				}).Debug("TCP connection successful")
				break
			}
			lastErr = err
			msg = fmt.Sprintf("TCP check failed: %v", err)
		}
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

	s.DB.Save(host)
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
	sb.WriteString(fmt.Sprintf("Host: %s (%s)\n", host.Name, host.Host))
	sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("Services affected: %d\n\n", len(downMonitors)))

	sb.WriteString("Impacted services:\n")
	for _, m := range downMonitors {
		if m.PreviousUptime != "" {
			sb.WriteString(fmt.Sprintf("• %s (was up %s)\n", m.Name, m.PreviousUptime))
		} else {
			sb.WriteString(fmt.Sprintf("• %s\n", m.Name))
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

// CheckMonitor is the exported version for on-demand checks
func (s *UptimeService) CheckMonitor(monitor models.UptimeMonitor) {
	s.checkMonitor(monitor)
}

func (s *UptimeService) checkMonitor(monitor models.UptimeMonitor) {
	start := time.Now()
	success := false
	var msg string

	switch monitor.Type {
	case "http", "https":
		validatedURL, err := security.ValidateExternalURL(
			monitor.URL,
			// Uptime monitors are an explicit admin-configured feature and commonly
			// target loopback in local/dev setups (and in unit tests).
			security.WithAllowLocalhost(),
			security.WithAllowHTTP(),
			security.WithTimeout(3*time.Second),
		)
		if err != nil {
			msg = fmt.Sprintf("security validation failed: %s", err.Error())
			break
		}

		client := network.NewSafeHTTPClient(
			network.WithTimeout(10*time.Second),
			network.WithDialTimeout(5*time.Second),
			// Explicit redirect policy per call site: disable.
			network.WithMaxRedirects(0),
			// Uptime monitors are an explicit admin-configured feature and commonly
			// target loopback in local/dev setups (and in unit tests).
			network.WithAllowLocalhost(),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, http.NoBody)
		if err != nil {
			msg = err.Error()
			break
		}

		resp, err := client.Do(req)
		if err == nil {
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					logger.Log().WithError(closeErr).Warn("failed to close uptime service response body")
				}
			}()
			// Accept 2xx, 3xx, and 401/403 (Unauthorized/Forbidden often means the service is up but protected)
			if (resp.StatusCode >= 200 && resp.StatusCode < 400) || resp.StatusCode == 401 || resp.StatusCode == 403 {
				success = true
				msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			} else {
				msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
		} else {
			msg = err.Error()
		}
	case "tcp":
		conn, err := net.DialTimeout("tcp", monitor.URL, 10*time.Second)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				logger.Log().WithError(closeErr).Warn("failed to close tcp connection")
			}
			success = true
			msg = "Connection successful"
		} else {
			msg = err.Error()
		}
	default:
		msg = "Unknown monitor type"
	}

	latency := time.Since(start).Milliseconds()

	// Determine new status based on success and retries
	newStatus := monitor.Status

	if success {
		// If it was down or pending, it's now up immediately
		if monitor.Status != "up" {
			newStatus = "up"
		}
		// Reset failure count on success
		monitor.FailureCount = 0
	} else {
		// Increment failure count
		monitor.FailureCount++

		// Only mark as down if we exceeded max retries
		// Default MaxRetries to 3 if 0 (legacy records)
		maxRetries := monitor.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 3
		}

		if monitor.FailureCount >= maxRetries {
			newStatus = "down"
		}
	}

	// Record Heartbeat (always record the raw result)
	heartbeatStatus := "down"
	if success {
		heartbeatStatus = "up"
	}

	heartbeat := models.UptimeHeartbeat{
		MonitorID: monitor.ID,
		Status:    heartbeatStatus,
		Latency:   latency,
		Message:   msg,
	}
	_ = s.DB.Create(&heartbeat).Error

	// Update Monitor Status
	oldStatus := monitor.Status
	statusChanged := oldStatus != newStatus && oldStatus != "pending"

	// Calculate previous uptime/downtime if status changed
	var durationStr string
	if statusChanged && !monitor.LastStatusChange.IsZero() {
		duration := time.Since(monitor.LastStatusChange)
		durationStr = formatDuration(duration)
	}

	monitor.Status = newStatus
	monitor.LastCheck = time.Now()
	monitor.Latency = latency

	if statusChanged {
		monitor.LastStatusChange = time.Now()
	}

	s.DB.Save(&monitor)

	// Handle notifications based on status change
	if statusChanged {
		switch newStatus {
		case "down":
			// Queue for batched notification
			s.queueDownNotification(monitor, msg, durationStr)
		case "up":
			// Send recovery notification
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
		sb.WriteString(fmt.Sprintf("Service: %s\n", m.Name))
		sb.WriteString("Status: DOWN\n")
		sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
		if m.PreviousUptime != "" {
			sb.WriteString(fmt.Sprintf("Previous Uptime: %s\n", m.PreviousUptime))
		}
		sb.WriteString(fmt.Sprintf("Reason: %s\n", m.Message))
	} else {
		// Multiple services down
		title = fmt.Sprintf("🔴 %d Services DOWN on %s", len(pending.downMonitors), pending.hostName)
		sb.WriteString(fmt.Sprintf("Host: %s\n", pending.hostName))
		sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
		sb.WriteString(fmt.Sprintf("Services affected: %d\n\n", len(pending.downMonitors)))

		sb.WriteString("Impacted services:\n")
		for _, m := range pending.downMonitors {
			if m.PreviousUptime != "" {
				sb.WriteString(fmt.Sprintf("• %s - %s (was up %s)\n", m.Name, m.Message, m.PreviousUptime))
			} else {
				sb.WriteString(fmt.Sprintf("• %s - %s\n", m.Name, m.Message))
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

// sendRecoveryNotification sends a notification when a service recovers
func (s *UptimeService) sendRecoveryNotification(monitor models.UptimeMonitor, downtime string) {
	title := fmt.Sprintf("🟢 %s is UP", monitor.Name)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Service: %s\n", monitor.Name))
	sb.WriteString("Status: UP\n")
	sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))
	if downtime != "" {
		sb.WriteString(fmt.Sprintf("Downtime: %s\n", downtime))
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
	s.NotificationService.SendExternal(context.Background(), "uptime", title, sb.String(), data)
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

	// Set defaults
	if interval <= 0 {
		interval = 60 // Default 60 seconds
	}
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
		"monitor_name": monitor.Name,
		"monitor_type": monitor.Type,
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

func (s *UptimeService) GetMonitorHistory(id string, limit int) ([]models.UptimeHeartbeat, error) {
	var heartbeats []models.UptimeHeartbeat
	result := s.DB.Where("monitor_id = ?", id).Order("created_at desc").Limit(limit).Find(&heartbeats)
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
