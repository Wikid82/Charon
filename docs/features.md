# ✨ Features

Caddy Proxy Manager+ (CPMP) is packed with features to make managing your web services simple and secure. Here's everything you can do:

---

## 🔒 Security

### CrowdSec Integration
Block malicious IPs automatically using community-driven threat intelligence. CrowdSec analyzes your logs and blocks attackers before they can cause harm.
→ [Learn more about CrowdSec](https://www.crowdsec.net/)

### Web Application Firewall (WAF)
Protect your applications from common web attacks like SQL injection and cross-site scripting using Coraza WAF, an enterprise-grade firewall built into Caddy.
→ [Learn more about Coraza WAF](https://coraza.io/)

### Access Control Lists (ACLs)
Control who can access your services with IP whitelists, blacklists, and geo-blocking. Block entire countries or allow only specific networks.
→ [ACL Documentation](security.md#access-control-lists)

### Rate Limiting
Prevent abuse by limiting how many requests a single IP can make. Protect against brute force attacks and API abuse.
→ [Rate Limiting Setup](security.md#rate-limiting)

### Automatic HTTPS
Every site gets a free SSL certificate automatically. No configuration needed—just add your domain and it's secure.
→ [SSL/TLS Configuration](security.md#ssltls-certificates)

---

## 📊 Monitoring

### Built-in Uptime Monitor
Know instantly when your services go down. Get notifications via Discord, Slack, email, or webhooks when something isn't responding.
→ [Uptime Monitoring Guide](uptime.md) *(coming soon)*

### Real-time Health Dashboard
See the status of all your services at a glance. View response times, uptime history, and current availability from one dashboard.

### Smart Notifications
Get notified only when it matters. Notifications are grouped by server so you don't get spammed when a whole host goes down.

---

## 🖥️ Proxy Management

### Visual Proxy Configuration
Add and manage reverse proxies without touching configuration files. Point-and-click simplicity with full power under the hood.

### Multi-Domain Support
Host unlimited domains from a single server. Each domain can point to a different backend service.

### WebSocket Support
Real-time apps like chat, gaming, and live updates work out of the box. WebSocket connections are automatically upgraded.

### Load Balancing
Distribute traffic across multiple backend servers. Keep your services fast and reliable even under heavy load.

### Custom Headers
Add, modify, or remove HTTP headers as traffic passes through. Perfect for CORS, security headers, or custom routing logic.

---

## 🐳 Docker Integration

### Container Discovery
See all Docker containers running on your servers. One click to create a proxy for any container.

### Remote Docker Support
Manage containers on other servers through secure connections. Perfect for multi-server setups with Tailscale or WireGuard VPNs.
→ [Remote Docker Setup](getting-started.md#remote-docker)

### Automatic Port Detection
CPMP reads container labels and exposed ports automatically. Less typing, fewer mistakes.

---

## 📥 Import & Migration

### Caddyfile Import
Already using Caddy? Import your existing Caddyfile and CPMP will create proxies for each site automatically.
→ [Import Guide](import-guide.md)

### NPM Migration *(coming soon)*
Migrating from Nginx Proxy Manager? We'll import your configuration so you don't start from scratch.

### Conflict Resolution
When imports find existing entries, you choose what to do—keep existing, overwrite, or merge configurations.

---

## 💾 Backup & Restore

### Automatic Backups
Your configuration is automatically backed up before destructive operations like deletes.

### One-Click Restore
Something go wrong? Restore any previous configuration with a single click.

### Export Configuration
Download your entire configuration for safekeeping or migration to another server.

---

## 🎨 User Experience

### Dark Mode Interface
Easy on the eyes during late-night troubleshooting. The modern dark interface looks great on any device.

### Mobile Responsive
Manage your proxies from your phone or tablet. The interface adapts to any screen size.

### Bulk Operations
Select multiple items and perform actions on all of them at once. Delete, enable, or disable in bulk.

### Search & Filter
Find what you're looking for quickly. Filter by status, search by name, or sort by any column.

---

## 🔌 API & Automation

### RESTful API
Automate everything through a complete REST API. Create proxies, manage certificates, and monitor uptime programmatically.
→ [API Documentation](api.md)

### Webhook Notifications
Send events to any system that accepts webhooks. Integrate with your existing monitoring and automation tools.

---

## 🛡️ Enterprise Features

### Multi-User Support *(coming soon)*
Add team members with different permission levels. Admins, editors, and viewers.

### Audit Logging *(coming soon)*
Track who changed what and when. Full history of all configuration changes.

### SSO Integration *(coming soon)*
Sign in with your existing identity provider. Support for OAuth, SAML, and OIDC.

---

## 🚀 Performance

### Caddy-Powered
Built on Caddy, one of the fastest and most memory-efficient web servers available.

### Minimal Resource Usage
Runs happily on a Raspberry Pi. Low CPU and memory footprint.

### Instant Configuration Reloads
Changes take effect immediately without downtime. Zero-downtime configuration updates.

---

## 📚 Need More Details?

Each feature has detailed documentation:

- [Getting Started](getting-started.md) - Your first proxy in 5 minutes
- [Security Features](security.md) - Deep dive into security options
- [API Reference](api.md) - Complete API documentation
- [Import Guide](import-guide.md) - Migrating from other tools

---

<p align="center">
  <em>Missing a feature? <a href="https://github.com/Wikid82/CaddyProxyManagerPlus/discussions">Let us know!</a></em>
</p>
