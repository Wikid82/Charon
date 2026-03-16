---
title: Features
description: Discover what makes Charon the easiest way to manage your reverse proxy. Explore automatic HTTPS, Docker integration, enterprise security, and more.
---

# Features

Charon makes managing your web applications simple. No command lines, no config files—just a clean interface that lets you focus on what matters: running your apps.

---

## 🎯 Core Features

### 🎯 Point & Click Management

Say goodbye to editing configuration files and memorizing commands. Charon gives you a beautiful web interface where you simply type your domain name, select your backend service, and click save. If you can browse the web, you can manage a reverse proxy.

Whether you're setting up your first website or managing dozens of services, everything happens through intuitive forms and buttons. No terminal required.

→ [Learn More](features/web-ui.md)

---

### 🔐 Automatic HTTPS Certificates

Every website deserves the green padlock. Charon automatically obtains free SSL certificates from Let's Encrypt or ZeroSSL, installs them, and renews them before they expire—all without you lifting a finger.

Your visitors get secure connections, search engines reward you with better rankings, and you never have to think about certificate management again.

→ [Learn More](features/ssl-certificates.md)

---

### 🌐 DNS Challenge for Wildcard Certificates

Need to secure `*.example.com` with a single certificate? Charon now supports DNS challenge authentication, letting you obtain wildcard certificates that cover all your subdomains at once.

**Supported Providers:**

- Cloudflare, AWS Route53, DigitalOcean, Google Cloud DNS
- Namecheap, GoDaddy, Hetzner, OVH, Linode
- And 10+ more DNS providers

Your credentials are stored securely with encryption and automatic key rotation. A plugin architecture means new providers can be added easily.

→ [Learn More](features/dns-challenge.md)

---

## 🐕 Cerberus Security Suite

Enterprise-grade protection that "just works." Cerberus bundles multiple security layers into one easy-to-manage system.

### 🎛️ Security Dashboard Toggles

Control your security modules with a single click. The Security Dashboard provides instant toggles for each security layer:

- **ACL Toggle** — Enable/disable Access Control Lists without editing config files
- **WAF Toggle** — Turn the Web Application Firewall on/off in real-time
- **Rate Limiting Toggle** — Activate or deactivate request rate limits instantly

**Key Features:**

- **Instant Updates** — Changes take effect immediately with automatic Caddy config reload
- **Persistent State** — Toggle settings persist across page reloads and container restarts
- **Optimistic UI** — Toggle changes reflect instantly with automatic rollback on failure
- **Performance Optimized** — 60-second cache layer minimizes database queries in middleware

→ [Learn More](features/security-dashboard.md)

---

### 🕵️ CrowdSec Integration

Protect your applications using behavior-based threat detection powered by a global community of security data. Bad actors get blocked automatically before they can cause harm.

→ [Learn More](features/crowdsec.md) • [Setup Guide](guides/crowdsec-setup.md)

---

### 🔐 Access Control Lists (ACLs)

Define exactly who can access what. Block specific countries, allow only certain IP ranges, or require authentication for sensitive applications. Fine-grained rules give you complete control.

→ [Learn More](features/access-control.md)

---

### 🧱 Web Application Firewall (WAF)

Stop common attacks like SQL injection, cross-site scripting (XSS), and path traversal before they reach your applications. Powered by Coraza, the WAF protects your apps from the OWASP Top 10 vulnerabilities.

→ [Learn More](features/waf.md)

---

### ⏱️ Rate Limiting

Prevent abuse by limiting how many requests a user or IP address can make. Stop brute-force attacks, API abuse, and resource exhaustion with simple, configurable limits.

→ [Learn More](features/rate-limiting.md)

---

## �️ Development & Security Tools

### 🔍 GORM Security Scanner

Automated static analysis that detects GORM security issues and common mistakes before they reach production. The scanner identifies ID leak vulnerabilities, exposed secrets, and enforces GORM best practices.

**Key Features:**

- **6 Detection Patterns** — ID leaks, exposed secrets, DTO embedding issues, and more
- **3 Operating Modes** — Report, check, and enforce modes for different workflows
- **Fast Performance** — Scans entire codebase in 2.1 seconds
- **Zero False Positives** — Smart GORM model detection prevents incorrect warnings
- **Lefthook Integration** — Catches issues before they're committed
- **VS Code Task** — Run security scans from the Command Palette

**Detects:**

- Numeric ID exposure in JSON (`json:"id"` on `uint`/`int` fields)
- Exposed API keys, tokens, and passwords
- Response DTOs that inherit model ID fields
- Missing primary key tags and foreign key indexes

**Usage:**

```bash
# Run via VS Code: Command Palette → "Lint: GORM Security Scan"
# Or via lefthook:
lefthook run pre-commit
```

→ [Learn More](implementation/gorm_security_scanner_complete.md)

---

### ⚡ Optimized CI Pipelines

Time is valuable. Charon's development workflows are tuned for efficiency, ensuring that security verifications only run when valid artifacts exist.

- **Smart Triggers** — Supply chain checks wait for successful builds
- **Zero Redundancy** — Eliminates wasted runs on push/PR events
- **Stable Feedback** — Reduces false negatives for contributors

→ [See Developer Guide](guides/supply-chain-security-developer-guide.md)

---

## �🛡️ Security & Headers

### 🛡️ HTTP Security Headers

Modern browsers expect specific security headers to protect your users. Charon automatically adds industry-standard headers including:

- **Content-Security-Policy (CSP)** — Prevents code injection attacks
- **Strict-Transport-Security (HSTS)** — Enforces HTTPS connections
- **X-Frame-Options** — Stops clickjacking attacks
- **X-Content-Type-Options** — Prevents MIME-type sniffing

One toggle gives your application the same security posture as major websites.

→ [Learn More](features/security-headers.md)

---

### 🔗 Smart Proxy Headers

Your backend applications need to know the real client IP address, not Charon's. Standard headers like `X-Real-IP`, `X-Forwarded-For`, and `X-Forwarded-Proto` are added automatically, ensuring accurate logging and proper HTTPS enforcement.

→ [Learn More](features/proxy-headers.md)

---

## 🐳 Docker & Integration

### 🐳 Docker Auto-Discovery

Already running apps in Docker? Charon automatically finds your containers and offers one-click proxy setup. No manual configuration, no port hunting—just select a container and go.

Supports both local Docker installations and remote Docker servers, perfect for managing multiple machines from a single dashboard.

→ [Learn More](features/docker-integration.md)

---

### 📥 Caddyfile Import

Migrating from another Caddy setup? Import your existing Caddyfile configurations with one click. Your existing work transfers seamlessly—no need to start from scratch.

→ [Learn More](features/caddyfile-import.md)

---

### � Nginx Proxy Manager Import

Migrating from Nginx Proxy Manager? Import your proxy host configurations directly from NPM export files. Charon parses your domains, upstream servers, SSL settings, and access lists, giving you a preview before committing.

→ [Learn More](features/npm-import.md)

---

### 📄 JSON Configuration Import

Import configurations from generic JSON exports or Charon backup files. Supports both Charon's native export format and Nginx Proxy Manager format with automatic detection. Perfect for restoring backups or migrating between Charon instances.

→ [Learn More](features/json-import.md)

---

### �🔌 WebSocket Support

Real-time applications like chat servers, live dashboards, and collaborative tools work out of the box. Charon handles WebSocket connections automatically with no special configuration needed.

→ [Learn More](features/websocket.md)

---

## 📊 Monitoring & Observability

### 📊 Uptime Monitoring

Know immediately when something goes wrong. Charon continuously monitors your applications and alerts you when a service becomes unavailable. View uptime history, response times, and availability statistics at a glance.

→ [Learn More](features/uptime-monitoring.md)

---

### 📋 Real-Time Logs

Watch requests flow through your proxy in real-time. Filter by domain, status code, or time range to troubleshoot issues quickly. All the visibility you need without diving into container logs.

→ [Learn More](features/logs.md)

---

### 🔔 Notifications

Get alerted when it matters. Charon notifications now run through the Notify HTTP wrapper with support for Discord, Gotify, and Custom Webhook providers. Payload-focused test coverage is included to help catch formatting and delivery regressions before release.

→ [Learn More](features/notifications.md)

---

## 🛠️ Administration

### 💾 Backup & Restore

Your configuration is valuable. Charon makes it easy to backup your entire setup and restore it when needed—whether you're migrating to new hardware or recovering from a problem.

→ [Learn More](features/backup-restore.md)

---

### ⚡ Zero-Downtime Updates

Make changes without interrupting your users. Update domains, modify security rules, or add new services instantly. Your sites stay up while you work—no container restarts needed.*

<sup>*Initial CrowdSec security engine setup requires a one-time restart.</sup>

→ [Learn More](features/live-reload.md)

---

### 🌍 Multi-Language Support

Charon speaks your language. The interface is available in English, Spanish, French, German, and Chinese. Switch languages instantly in settings—no reload required.

→ [Learn More](features/localization.md)

---

### 🎨 Dark Mode & Modern UI

Easy on the eyes, day or night. Toggle between light and dark themes to match your preference. The clean, modern interface makes managing complex setups feel simple.

→ [Learn More](features/ui-themes.md)

---

## 🤖 Automation & API

### 🤖 REST API

Automate everything. Charon's comprehensive REST API lets you manage hosts, certificates, security rules, and settings programmatically. Perfect for CI/CD pipelines, Infrastructure as Code, or custom integrations.

→ [Learn More](features/api.md)

---

## 🔒 Supply Chain Security

### 🔒 Verified Builds

Know exactly what you're running. Every Charon release includes:

- **Cryptographic signatures** — Verify the image hasn't been tampered with
- **SLSA provenance attestation** — Transparent build process documentation
- **Software Bill of Materials (SBOM)** — Complete list of included components

Enterprise-grade supply chain security for everyone.

→ [Learn More](features/supply-chain-security.md)

---

## 🚀 Deployment

### 🚀 Zero-Dependency Deployment

One container. No external databases. No extra services. Just pull the image and run. Charon includes everything it needs, making deployment as simple as it gets.

→ [Learn More](../README.md#quick-start)

---

### 💯 100% Free & Open Source

No premium tiers. No feature paywalls. No usage limits. Everything you see here is yours to use forever, backed by the MIT license.

→ [View on GitHub](https://github.com/Wikid82/Charon)

---

## What's Next?

Ready to get started? Check out our [Quick Start Guide](../README.md#quick-start) to have Charon running in minutes.

Have questions? Visit our [Documentation](index.md) or [open an issue](https://github.com/Wikid82/Charon/issues) on GitHub.
