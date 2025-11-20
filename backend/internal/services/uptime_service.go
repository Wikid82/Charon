package services

import (
	"fmt"
	"net"
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

// CheckHost checks a single host and creates a notification if it's down
func (s *UptimeService) CheckHost(host string, port int) bool {
	timeout := 5 * time.Second
	target := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		conn.Close()
		return true
	}
	return false
}

// CheckAllHosts iterates through ProxyHosts and checks their upstream targets
func (s *UptimeService) CheckAllHosts() {
	var hosts []models.ProxyHost
	if err := s.DB.Find(&hosts).Error; err != nil {
		return
	}

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}
		// Assuming ProxyHost has ForwardHost and ForwardPort
		// We need to check if the upstream is reachable
		alive := s.CheckHost(host.ForwardHost, host.ForwardPort)
		if !alive {
			// Check if we already notified recently? For now just notify.
			// In a real app, we'd want to avoid spamming.
			s.NotificationService.Create(
				models.NotificationTypeError,
				"Host Unreachable",
				fmt.Sprintf("Proxy Host %s (Upstream: %s:%d) is unreachable.", host.DomainNames, host.ForwardHost, host.ForwardPort),
			)
		}
	}
}
