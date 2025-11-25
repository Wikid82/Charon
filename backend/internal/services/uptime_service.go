package services

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"gorm.io/gorm"
)

type UptimeService struct {
	DB                  *gorm.DB
	NotificationService *NotificationService
}

func NewUptimeService(db *gorm.DB, ns *NotificationService) *UptimeService {
	return &UptimeService{
		DB:                  db,
		NotificationService: ns,
	}
}

// SyncMonitors ensures every ProxyHost has a corresponding UptimeMonitor
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

		switch err {
		case gorm.ErrRecordNotFound:
			// Create new monitor
			name := host.Name
			if name == "" {
				name = firstDomain
			}

			monitor = models.UptimeMonitor{
				ProxyHostID: &host.ID,
				Name:        name,
				Type:        "http", // Check public access
				URL:         publicURL,
				Interval:    60,
				Enabled:     true,
				Status:      "pending",
			}
			if err := s.DB.Create(&monitor).Error; err != nil {
				log.Printf("Failed to create monitor for host %d: %v", host.ID, err)
			}
		case nil:
			// Always sync the name from proxy host
			newName := host.Name
			if newName == "" {
				newName = firstDomain
			}
			if monitor.Name != newName {
				monitor.Name = newName
				s.DB.Save(&monitor)
				log.Printf("Updated monitor name for host %d to: %s", host.ID, newName)
			}

			// Update existing monitor if it looks like it's using the old default (TCP to internal upstream)
			// We check if it matches the internal upstream URL to avoid overwriting custom user settings
			if monitor.Type == "tcp" && monitor.URL == internalURL {
				monitor.Type = "http"
				monitor.URL = publicURL
				s.DB.Save(&monitor)
				log.Printf("Migrated monitor for host %d to check public URL: %s", host.ID, publicURL)
			}

			// Upgrade to HTTPS if SSL is forced and we are currently checking HTTP
			if host.SSLForced && strings.HasPrefix(monitor.URL, "http://") {
				monitor.URL = strings.Replace(monitor.URL, "http://", "https://", 1)
				s.DB.Save(&monitor)
				log.Printf("Upgraded monitor for host %d to HTTPS: %s", host.ID, monitor.URL)
			}
		}
	}
	return nil
}

// CheckAll runs checks for all enabled monitors
func (s *UptimeService) CheckAll() {
	var monitors []models.UptimeMonitor
	if err := s.DB.Where("enabled = ?", true).Find(&monitors).Error; err != nil {
		log.Printf("Failed to fetch monitors: %v", err)
		return
	}

	for _, monitor := range monitors {
		go s.checkMonitor(monitor)
	}
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
			conn.Close()
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
	s.DB.Create(&heartbeat)

	// Update Monitor Status
	oldStatus := monitor.Status
	statusChanged := oldStatus != newStatus && oldStatus != "pending"

	// Calculate duration if status changed
	var durationStr string
	if statusChanged && !monitor.LastStatusChange.IsZero() {
		duration := time.Since(monitor.LastStatusChange)
		durationStr = duration.Round(time.Second).String()
	}

	monitor.Status = newStatus
	monitor.LastCheck = time.Now()
	monitor.Latency = latency

	if statusChanged {
		monitor.LastStatusChange = time.Now()
	}

	s.DB.Save(&monitor)

	// Send Notification if status changed
	if statusChanged {
		title := fmt.Sprintf("Monitor %s is %s", monitor.Name, strings.ToUpper(newStatus))

		nType := models.NotificationTypeInfo
		switch newStatus {
		case "down":
			nType = models.NotificationTypeError
		case "up":
			nType = models.NotificationTypeSuccess
		}

		// Construct rich message
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Service: %s\n", monitor.Name))
		sb.WriteString(fmt.Sprintf("Status: %s\n", strings.ToUpper(newStatus)))
		sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC1123)))

		if durationStr != "" {
			sb.WriteString(fmt.Sprintf("Duration: %s\n", durationStr))
		}

		sb.WriteString(fmt.Sprintf("Reason: %s\n", msg))

		s.NotificationService.Create(
			nType,
			title,
			sb.String(),
		)

		data := map[string]interface{}{
			"Name":     monitor.Name,
			"Status":   strings.ToUpper(newStatus),
			"Latency":  latency,
			"Message":  msg,
			"Duration": durationStr,
			"Time":     time.Now().Format(time.RFC1123),
			"URL":      monitor.URL,
		}
		s.NotificationService.SendExternal("uptime", title, sb.String(), data)
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
	// Add other fields as needed, but be careful not to overwrite SyncMonitors logic

	if err := s.DB.Model(&monitor).Updates(allowedUpdates).Error; err != nil {
		return nil, err
	}

	return &monitor, nil
}
