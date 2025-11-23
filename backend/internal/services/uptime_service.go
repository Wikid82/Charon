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
		if err == gorm.ErrRecordNotFound {
			// Create new monitor
			domains := strings.Split(host.DomainNames, ",")
			name := host.Name
			if name == "" && len(domains) > 0 {
				name = domains[0]
			}

			monitor = models.UptimeMonitor{
				ProxyHostID: &host.ID,
				Name:        name,
				Type:        "tcp", // Default to TCP check of upstream
				URL:         fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort),
				Interval:    60,
				Enabled:     true,
				Status:      "pending",
			}
			if err := s.DB.Create(&monitor).Error; err != nil {
				log.Printf("Failed to create monitor for host %d: %v", host.ID, err)
			}
		} else if err == nil {
			// Update existing monitor if needed (e.g. if upstream changed)
			// For now, we won't overwrite user changes, but we could sync URL here
			// monitor.URL = fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)
			// s.DB.Save(&monitor)
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
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
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
		s.NotificationService.Create(
			models.NotificationTypeInfo,
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
	result := s.DB.Find(&monitors)
	return monitors, result.Error
}

func (s *UptimeService) GetMonitorHistory(id string, limit int) ([]models.UptimeHeartbeat, error) {
	var heartbeats []models.UptimeHeartbeat
	result := s.DB.Where("monitor_id = ?", id).Order("created_at desc").Limit(limit).Find(&heartbeats)
	return heartbeats, result.Error
}
