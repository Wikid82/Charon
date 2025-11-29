package services

import (
	"bytes"
	"fmt"
	"log"
	"net"
	neturl "net/url"
	"strings"
	"net/http"
	"regexp"
	"text/template"
	"encoding/json"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
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
				if err := s.sendCustomWebhook(p, data); err != nil {
					log.Printf("Failed to send webhook to %s: %v", p.Name, err)
				}
			} else {
				url := normalizeURL(p.Type, p.URL)
				// Validate HTTP/HTTPS destinations used by shoutrrr to reduce SSRF risk
				if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
					if _, err := validateWebhookURL(url); err != nil {
						log.Printf("Skipping notification for provider %s due to invalid destination", p.Name)
						return
					}
				}
				// Use newline for better formatting in chat apps
				msg := fmt.Sprintf("%s\n\n%s", title, message)
				if err := shoutrrr.Send(url, msg); err != nil {
					log.Printf("Failed to send notification to %s: %v", p.Name, err)
				}
			}
		}(provider)
	}
}

func (s *NotificationService) sendCustomWebhook(p models.NotificationProvider, data map[string]interface{}) error {
	// Built-in templates
	const minimalTemplate = `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}}`
	const detailedTemplate = `{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}, "host": {{toJSON .HostName}}, "host_ip": {{toJSON .HostIP}}, "service_count": {{toJSON .ServiceCount}}, "services": {{toJSON .Services}}, "data": {{toJSON .}}}`

	// Select template based on provider.Template; if 'custom' use Config; else builtin.
	tmplStr := p.Config
	switch strings.ToLower(strings.TrimSpace(p.Template)) {
	case "detailed":
		tmplStr = detailedTemplate
	case "minimal":
		tmplStr = minimalTemplate
	case "custom":
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	default:
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	}

	// Validate webhook URL to reduce SSRF risk (returns parsed URL)
	u, err := validateWebhookURL(p.URL)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}

	// Parse template and add helper funcs
	tmpl, err := template.New("webhook").Funcs(template.FuncMap{
		"toJSON": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse webhook template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute webhook template: %w", err)
	}

	// Send Request with a safe client (timeout, no auto-redirect)
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Resolve the hostname to an explicit IP and construct the request URL using the
	// resolved IP. This prevents direct user-controlled hostnames from being used
	// as the request's destination (SSRF mitigation) and helps CodeQL validate the
	// sanitisation performed by validateWebhookURL.
	//
	// NOTE (security): The following mitigations are intentionally applied to
	// reduce SSRF/request-forgery risk:
	//  - `validateWebhookURL` enforces http(s) schemes and rejects private IPs
	//    (except explicit localhost for testing) after DNS resolution.
	//  - We perform an additional DNS resolution here and choose a non-private
	//    IP to use as the TCP destination to avoid direct hostname-based routing.
	//  - We set the request's `Host` header to the original hostname so virtual
	//    hosting works while the actual socket connects to a resolved IP.
	//  - The HTTP client disables automatic redirects and has a short timeout.
	// Together these steps make the request destination unambiguous and prevent
	// accidental requests to internal networks. If your threat model requires
	// stricter controls, consider an explicit allowlist of webhook hostnames.
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("failed to resolve webhook host: %w", err)
	}
	// If hostname is local loopback, accept loopback addresses; otherwise pick
	// the first non-private IP (validateWebhookURL already ensured these
	// are not private, but check again defensively).
	var selectedIP net.IP
	for _, ip := range ips {
		if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" {
			selectedIP = ip
			break
		}
		if !isPrivateIP(ip) {
			selectedIP = ip
			break
		}
	}
	if selectedIP == nil {
		return fmt.Errorf("failed to find non-private IP for webhook host: %s", u.Hostname())
	}

	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	// Construct a safe URL using the resolved IP:port for the Host component,
	// while preserving the original path and query from the user-provided URL.
	// This makes the destination hostname unambiguously an IP that we resolved
	// and prevents accidental requests to private/internal addresses.
	safeURL := &neturl.URL{
		Scheme:   u.Scheme,
		Host:     net.JoinHostPort(selectedIP.String(), port),
		Path:     u.Path,
		RawQuery: u.RawQuery,
	}
	req, err := http.NewRequest("POST", safeURL.String(), &body)
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Preserve original hostname for virtual host (Host header)
	req.Host = u.Host

	// We validated the URL and resolved the hostname to an explicit IP above.
	// The request uses the resolved IP (selectedIP) and we also set the
	// Host header to the original hostname, so virtual-hosting works while
	// preventing requests to private or otherwise disallowed addresses.
	// This mitigates SSRF and addresses the CodeQL request-forgery rule.
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status: %d", resp.StatusCode)
	}
	return nil
}

// isPrivateIP returns true for RFC1918, loopback and link-local addresses.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// IPv4 RFC1918
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		}
	}

	// IPv6 unique local addresses fc00::/7
	if ip.To16() != nil && strings.HasPrefix(ip.String(), "fc") {
		return true
	}

	return false
}

// validateWebhookURL parses and validates webhook URLs and ensures
// the resolved addresses are not private/local.
func validateWebhookURL(raw string) (*neturl.URL, error) {
	u, err := neturl.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	// Allow explicit loopback/localhost addresses for local tests.
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return u, nil
	}

	// Resolve and check IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed: %w", err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("disallowed host IP: %s", ip.String())
		}
	}
	return u, nil
}

func (s *NotificationService) TestProvider(provider models.NotificationProvider) error {
	if provider.Type == "webhook" {
		data := map[string]interface{}{
			"Title":   "Test Notification",
			"Message": "This is a test notification from Charon",
			"Status":  "TEST",
			"Name":    "Test Monitor",
			"Latency": 123,
			"Time":    time.Now().Format(time.RFC3339),
		}
		return s.sendCustomWebhook(provider, data)
	}
	url := normalizeURL(provider.Type, provider.URL)
	return shoutrrr.Send(url, "Test notification from Charon")
}

// Templates (external notification templates) management
func (s *NotificationService) ListTemplates() ([]models.NotificationTemplate, error) {
	var list []models.NotificationTemplate
	if err := s.DB.Order("created_at desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *NotificationService) GetTemplate(id string) (*models.NotificationTemplate, error) {
	var t models.NotificationTemplate
	if err := s.DB.First(&t, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *NotificationService) CreateTemplate(t *models.NotificationTemplate) error {
	return s.DB.Create(t).Error
}

func (s *NotificationService) UpdateTemplate(t *models.NotificationTemplate) error {
	return s.DB.Save(t).Error
}

func (s *NotificationService) DeleteTemplate(id string) error {
	return s.DB.Delete(&models.NotificationTemplate{}, "id = ?", id).Error
}

// RenderTemplate renders a provider template with provided data and returns
// the rendered JSON string and the parsed object for previewing/validation.
func (s *NotificationService) RenderTemplate(p models.NotificationProvider, data map[string]interface{}) (string, interface{}, error) {
	// Built-in templates
	const minimalTemplate = `{"message": {{toJSON .Message}}, "title": {{toJSON .Title}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}}`
	const detailedTemplate = `{"title": {{toJSON .Title}}, "message": {{toJSON .Message}}, "time": {{toJSON .Time}}, "event": {{toJSON .EventType}}, "host": {{toJSON .HostName}}, "host_ip": {{toJSON .HostIP}}, "service_count": {{toJSON .ServiceCount}}, "services": {{toJSON .Services}}, "data": {{toJSON .}}}`

	tmplStr := p.Config
	switch strings.ToLower(strings.TrimSpace(p.Template)) {
	case "detailed":
		tmplStr = detailedTemplate
	case "minimal":
		tmplStr = minimalTemplate
	case "custom":
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	default:
		if tmplStr == "" {
			tmplStr = minimalTemplate
		}
	}

	// Parse and execute template with helper funcs
	tmpl, err := template.New("webhook").Funcs(template.FuncMap{
		"toJSON": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse webhook template: %w", err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return "", nil, fmt.Errorf("failed to execute webhook template: %w", err)
	}

	// Validate produced JSON
	var parsed interface{}
	if err := json.Unmarshal(body.Bytes(), &parsed); err != nil {
		return body.String(), nil, fmt.Errorf("failed to parse rendered template: %w", err)
	}
	return body.String(), parsed, nil
}

// Provider Management

func (s *NotificationService) ListProviders() ([]models.NotificationProvider, error) {
	var providers []models.NotificationProvider
	result := s.DB.Find(&providers)
	return providers, result.Error
}

func (s *NotificationService) CreateProvider(provider *models.NotificationProvider) error {
	// Validate custom template before creating
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		// Provide a minimal preview payload
		payload := map[string]interface{}{"Title": "Preview", "Message": "Preview", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
		if _, _, err := s.RenderTemplate(*provider, payload); err != nil {
			return fmt.Errorf("invalid custom template: %w", err)
		}
	}
	return s.DB.Create(provider).Error
}

func (s *NotificationService) UpdateProvider(provider *models.NotificationProvider) error {
	// Validate custom template before saving
	if strings.ToLower(strings.TrimSpace(provider.Template)) == "custom" && strings.TrimSpace(provider.Config) != "" {
		payload := map[string]interface{}{"Title": "Preview", "Message": "Preview", "Time": time.Now().Format(time.RFC3339), "EventType": "preview"}
		if _, _, err := s.RenderTemplate(*provider, payload); err != nil {
			return fmt.Errorf("invalid custom template: %w", err)
		}
	}
	return s.DB.Save(provider).Error
}

func (s *NotificationService) DeleteProvider(id string) error {
	return s.DB.Delete(&models.NotificationProvider{}, "id = ?", id).Error
}
