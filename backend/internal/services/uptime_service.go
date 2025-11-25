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

		if err == gorm.ErrRecordNotFound {
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
		} else if err == nil {
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
	status := "down"
	if success {
		status = "up"
	}

	// Record Heartbeat
	heartbeat := models.UptimeHeartbeat{
		MonitorID: monitor.ID,
		Status:    status,
		Latency:   latency,
		Message:   msg,
	}
	s.DB.Create(&heartbeat)

	// Update Monitor Status
	oldStatus := monitor.Status
	monitor.Status = status
	monitor.LastCheck = time.Now()
	monitor.Latency = latency
	s.DB.Save(&monitor)

	// Send Notification if status changed
	if oldStatus != "pending" && oldStatus != status {
		title := fmt.Sprintf("Monitor %s is %s", monitor.Name, status)

		nType := models.NotificationTypeInfo
		if status == "down" {
			nType = models.NotificationTypeError
		} else if status == "up" {
			nType = models.NotificationTypeSuccess
		}

		s.NotificationService.Create(
			nType,
			title,
			fmt.Sprintf("Monitor %s changed status from %s to %s. Latency: %dms. Message: %s", monitor.Name, oldStatus, status, latency, msg),
		)

		data := map[string]interface{}{
			"Name":    monitor.Name,
			"Status":  status,
			"Latency": latency,
			"Message": msg,
		}
		s.NotificationService.SendExternal("uptime", title, msg, data)
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
