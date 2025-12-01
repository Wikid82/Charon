package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wikid82/charon/backend/internal/logger"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
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
			if err := s.DB.First(&uptimeHost, "id = ?", hostID).Error; err == nil {
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

	for i := range hosts {
		s.checkHost(&hosts[i])
	}
}

// checkHost performs a basic TCP connectivity check to determine if the host is reachable
func (s *UptimeService) checkHost(host *models.UptimeHost) {
	start := time.Now()

	// Get common ports for this host from its monitors
	var monitors []models.UptimeMonitor
	s.DB.Where("uptime_host_id = ?", host.ID).Find(&monitors)

	if len(monitors) == 0 {
		return
	}

	// Try to connect to any of the monitor ports
	success := false
	var msg string

	for _, monitor := range monitors {
		port := extractPort(monitor.URL)
		if port == "" {
			continue
		}

		// Use net.JoinHostPort for IPv6 compatibility
		addr := net.JoinHostPort(host.Host, port)
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			_ = conn.Close()
			success = true
			msg = fmt.Sprintf("TCP connection to %s successful", addr)
			break
		}
		msg = err.Error()
	}

	latency := time.Since(start).Milliseconds()
	oldStatus := host.Status
	newStatus := "down"
	if success {
		newStatus = "up"
	}

	statusChanged := oldStatus != newStatus && oldStatus != "pending"

	host.Status = newStatus
	host.LastCheck = time.Now()
	host.Latency = latency

		if statusChanged {
			host.LastStatusChange = time.Now()
			logger.Log().WithFields(map[string]interface{}{
				"host_name": host.Name,
				"host_ip":   host.Host,
				"old":       oldStatus,
				"new":       newStatus,
				"message":   msg,
			}).Info("Host status changed")
		}

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
	data := map[string]interface{}{
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

func (s *UptimeService) checkMonitor(monitor models.UptimeMonitor) {
	start := time.Now()
	success := false
	var msg string

	switch monitor.Type {
	case "http", "https":
		client := http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(monitor.URL)
		if err == nil {
			defer resp.Body.Close()
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
			_ = conn.Close()
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
		if err := s.DB.First(&uptimeHost, "id = ?", hostID).Error; err == nil {
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
	data := map[string]interface{}{
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

	data := map[string]interface{}{
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

// CRUD for Monitors

func (s *UptimeService) ListMonitors() ([]models.UptimeMonitor, error) {
	var monitors []models.UptimeMonitor
	result := s.DB.Order("name ASC").Find(&monitors)
	return monitors, result.Error
}

func (s *UptimeService) GetMonitorHistory(id string, limit int) ([]models.UptimeHeartbeat, error) {
	var heartbeats []models.UptimeHeartbeat
	result := s.DB.Where("monitor_id = ?", id).Order("created_at desc").Limit(limit).Find(&heartbeats)
	return heartbeats, result.Error
}

func (s *UptimeService) UpdateMonitor(id string, updates map[string]interface{}) (*models.UptimeMonitor, error) {
	var monitor models.UptimeMonitor
	if err := s.DB.First(&monitor, "id = ?", id).Error; err != nil {
		return nil, err
	}

	// Whitelist allowed fields to update
	allowedUpdates := make(map[string]interface{})
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
	if err := s.DB.First(&monitor, "id = ?", id).Error; err != nil {
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
