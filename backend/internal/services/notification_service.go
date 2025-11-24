package services

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"text/template"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/containrrr/shoutrrr"
	"gorm.io/gorm"
)

type NotificationService struct {
	DB *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{DB: db}
}

var discordWebhookRegex = regexp.MustCompile(`^https://discord(?:app)?\.com/api/webhooks/(\d+)/([a-zA-Z0-9_-]+)`)

func normalizeURL(serviceType, rawURL string) string {
	if serviceType == "discord" {
		matches := discordWebhookRegex.FindStringSubmatch(rawURL)
		if len(matches) == 3 {
			id := matches[1]
			token := matches[2]
			return fmt.Sprintf("discord://%s@%s", token, id)
		}
	}
	return rawURL
}

// Internal Notifications (DB)

func (s *NotificationService) Create(nType models.NotificationType, title, message string) (*models.Notification, error) {
	notification := &models.Notification{
		Type:    nType,
		Title:   title,
		Message: message,
		Read:    false,
	}
	result := s.DB.Create(notification)
	return notification, result.Error
}

func (s *NotificationService) List(unreadOnly bool) ([]models.Notification, error) {
	var notifications []models.Notification
	query := s.DB.Order("created_at desc")
	if unreadOnly {
		query = query.Where("read = ?", false)
	}
	result := query.Find(&notifications)
	return notifications, result.Error
}

func (s *NotificationService) MarkAsRead(id string) error {
	return s.DB.Model(&models.Notification{}).Where("id = ?", id).Update("read", true).Error
}

func (s *NotificationService) MarkAllAsRead() error {
	return s.DB.Model(&models.Notification{}).Where("read = ?", false).Update("read", true).Error
}

// External Notifications (Shoutrrr & Custom Webhooks)

func (s *NotificationService) SendExternal(eventType, title, message string, data map[string]interface{}) {
	var providers []models.NotificationProvider
	if err := s.DB.Where("enabled = ?", true).Find(&providers).Error; err != nil {
		log.Printf("Failed to fetch notification providers: %v", err)
		return
	}

	// Prepare data for templates
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Title"] = title
	data["Message"] = message
	data["Time"] = time.Now().Format(time.RFC3339)
	data["EventType"] = eventType

	for _, provider := range providers {
		// Filter based on preferences
		shouldSend := false
		switch eventType {
		case "proxy_host":
			shouldSend = provider.NotifyProxyHosts
		case "remote_server":
			shouldSend = provider.NotifyRemoteServers
		case "domain":
			shouldSend = provider.NotifyDomains
		case "cert":
			shouldSend = provider.NotifyCerts
		case "uptime":
			shouldSend = provider.NotifyUptime
		case "test":
			shouldSend = true
		default:
			// Default to true for unknown types or generic messages?
			// Or false to be safe? Let's say true for now to avoid missing things,
			// or maybe we should enforce types.
			shouldSend = true
		}

		if !shouldSend {
			continue
		}

		go func(p models.NotificationProvider) {
			if p.Type == "webhook" {
				s.sendCustomWebhook(p, data)
			} else {
				url := normalizeURL(p.Type, p.URL)
				if err := shoutrrr.Send(url, fmt.Sprintf("%s: %s", title, message)); err != nil {
					log.Printf("Failed to send notification to %s: %v", p.Name, err)
				}
			}
		}(provider)
	}
}

func (s *NotificationService) sendCustomWebhook(p models.NotificationProvider, data map[string]interface{}) {
	// Default template if empty
	tmplStr := p.Config
	if tmplStr == "" {
		tmplStr = `{"content": "{{.Title}}: {{.Message}}"}`
	}

	// Parse template
	tmpl, err := template.New("webhook").Parse(tmplStr)
	if err != nil {
		log.Printf("Failed to parse webhook template for %s: %v", p.Name, err)
		return
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		log.Printf("Failed to execute webhook template for %s: %v", p.Name, err)
		return
	}

	// Send Request
	resp, err := http.Post(p.URL, "application/json", &body)
	if err != nil {
		log.Printf("Failed to send webhook to %s: %v", p.Name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Webhook %s returned status: %d", p.Name, resp.StatusCode)
	}
}

func (s *NotificationService) TestProvider(provider models.NotificationProvider) error {
	if provider.Type == "webhook" {
		data := map[string]interface{}{
			"Title":   "Test Notification",
			"Message": "This is a test notification from CaddyProxyManager+",
			"Status":  "TEST",
			"Name":    "Test Monitor",
			"Latency": 123,
			"Time":    time.Now().Format(time.RFC3339),
		}
		s.sendCustomWebhook(provider, data)
		return nil
	}
	url := normalizeURL(provider.Type, provider.URL)
	return shoutrrr.Send(url, "Test notification from CaddyProxyManager+")
}

// Provider Management

func (s *NotificationService) ListProviders() ([]models.NotificationProvider, error) {
	var providers []models.NotificationProvider
	result := s.DB.Find(&providers)
	return providers, result.Error
}

func (s *NotificationService) CreateProvider(provider *models.NotificationProvider) error {
	return s.DB.Create(provider).Error
}

func (s *NotificationService) UpdateProvider(provider *models.NotificationProvider) error {
	return s.DB.Save(provider).Error
}

func (s *NotificationService) DeleteProvider(id string) error {
	return s.DB.Delete(&models.NotificationProvider{}, "id = ?", id).Error
}
