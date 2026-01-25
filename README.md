<p align="center">
  <img src="frontend/public/banner.png" alt="Charon" width="600">
</p>

<h1 align="center">Charon</h1>

<p align="center"><strong>Your server, your rules—without the headaches.</strong></p>

<p align="center">
Simply manage multiple websites and self-hosted applications. Click, save, done. No code, no config files, no PhD required.
</p>

<br>

<p align="center">
  <a href="https://www.repostatus.org/#active"><img src="https://www.repostatus.org/badges/latest/active.svg" alt="Project Status: Active – The project is being actively developed." /></a>
  <a href="https://www.bestpractices.dev/projects/11648"><img src="https://www.bestpractices.dev/projects/11648/badge"></a>
 <br>
 <a href="https://hub.docker.com/r/wikid82/charon"><img src="https://img.shields.io/docker/pulls/wikid82/charon.svg" alt="Docker Pulls"></a>
 <a href="https://hub.docker.com/r/wikid82/charon"><img src="https://img.shields.io/docker/v/wikid82/charon?sort=semver" alt="Docker Version"></a>
 <a href="https://codecov.io/gh/Wikid82/Charon" ><img src="https://codecov.io/gh/Wikid82/Charon/branch/main/graph/badge.svg?token=RXSINLQTGE" alt="Code Coverage"/></a>
  <a href="https://github.com/Wikid82/charon/releases"><img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Release"></a>
 <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="SECURITY.md"><img src="https://img.shields.io/badge/Security-Audited-brightgreen.svg" alt="Security: Audited"></a>
</p>

---

## Why Charon?

You want your apps accessible online. You don't want to become a networking expert first.

**The problem:** Managing reverse proxies usually means editing config files, memorizing cryptic syntax, and hoping you didn't break everything.

**Charon's answer:** A web interface where you click boxes and type domain names. That's it.

- ✅ **Your blog** gets a green lock (HTTPS) automatically
- ✅ **Your chat server** works without weird port numbers
- ✅ **Your admin panel** blocks everyone except you
- ✅ **Everything stays up** even when you make changes

---

## 🐕 Cerberus Security Suite

### 🕵️‍♂️ **CrowdSec Integration**

- Protects your applications from attacks using behavior-based detection and automated remediation.

### 🔐 **Access Control Lists (ACLs)**

- Define fine-grained access rules for your applications, controlling who can access what and under which conditions.

### 🧱 **Web Application Firewall (WAF)**

- Protects your applications from common web vulnerabilities such as SQL injection, XSS, and more using Coraza.

### ⏱️ **Rate Limiting**

- Protect your applications from abuse by limiting the number of requests a user or IP can make within a certain timeframe.

---

## ✨ Top 10 Features

### 🎯 **Point & Click Management**

No config files. No terminal commands. Just click, type your domain name, and you're live. If you can use a website, you can run Charon.

### 🔐 **Automatic HTTPS Certificates**

Free SSL certificates that request, install, and renew themselves. Your sites get the green padlock without you lifting a finger.

### 🌐 **DNS Challenge for Wildcard Certificates**

Secure all your subdomains with a single `*.example.com` certificate. Supports 15+ DNS providers including Cloudflare, Route53, DigitalOcean, and Google Cloud DNS. Credentials are encrypted and automatically rotated.

### 🛡️ **Enterprise-Grade Security Built In**

Web Application Firewall, rate limiting, geographic blocking, access control lists, and intrusion detection via CrowdSec. Protection that "just works."

### 🔐 **Supply Chain Security**

Verifiable builds with cryptographic signatures, SLSA provenance attestation, and comprehensive SBOMs. Verify what you run with transparent, tamper-proof evidence.

### 🌐 **Smart Proxy Headers**

Automatically adds standard headers (X-Real-IP, X-Forwarded-Proto, etc.) so your backend applications see real client IPs, enforce HTTPS correctly, and log accurately—with full backward compatibility for existing hosts.

### 🐳 **Instant Docker Discovery**

Already running apps in Docker? Charon finds them automatically and offers one-click proxy setup. No manual configuration required.

### 📊 **Real-Time Monitoring & Logs**

See exactly what's happening with live request logs, uptime monitoring, and instant notifications when something goes wrong.

### 📥 **Migration Made Easy**

Import your existing configurations with one click:
- **Caddyfile Import** — Migrate from other Caddy setups
- **NPM Import** — Import from Nginx Proxy Manager exports
- **JSON Import** — Restore from Charon backups or generic JSON configs

Already invested in another reverse proxy? Bring your work with you.

### ⚡ **Live Configuration Changes**

Update domains, add security rules, or modify settings instantly—no container restarts needed.* Your sites stay up while you make changes.

### 🌍 **Multi-App Management**

Run dozens of websites, APIs, or services from a single dashboard. Perfect for homelab enthusiasts and small teams managing multiple projects.

### 🚀 **Zero-Dependency Deployment**

One Docker container. No databases to install. No external services required. No complexity—just pure simplicity.

### 💯 **100% Free & Open Source**

No premium tiers. No feature paywalls. No usage limits. Everything you see is yours to use, forever, backed by the MIT license.

<sup>* Note: Initial security engine setup (CrowdSec) requires a one-time container restart to initialize the protection layer. All subsequent changes happen live.</sup>

**[Explore All Features →](https://wikid82.github.io/charon/features)**

---

## Quick Start

### Container Registries

Charon is available from two container registries:

**Docker Hub (Recommended):**

```bash
docker pull wikid82/charon:latest
```

**GitHub Container Registry:**

```bash
docker pull ghcr.io/wikid82/charon:latest
```

### Docker Compose (Recommended)

Save this as `docker-compose.yml`:

```yaml
services:
  charon:
    # Docker Hub (recommended)
    image: wikid82/charon:latest
    # Alternative: GitHub Container Registry
    # image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"
    volumes:
      - ./charon-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - CHARON_ENV=production
      # Generate with: openssl rand -base64 32
      - CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here

```

**Using Nightly Builds:**

To test the latest nightly build (automated daily at 02:00 UTC):

```yaml
services:
  charon:
    # Docker Hub
    image: wikid82/charon:nightly
    # Alternative: GitHub Container Registry
    # image: ghcr.io/wikid82/charon:nightly
    # ... rest of configuration
```

> **Note:** Nightly builds are for testing and may contain experimental features. Use `latest` for production.

Then run:

```bash
docker-compose up -d
```

### Docker Run (One-Liner)

**Stable Release (Docker Hub):**

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 443:443/udp \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  -e CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here \
  wikid82/charon:latest
```

**Stable Release (GitHub Container Registry):**

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 443:443/udp \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  -e CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here \
  ghcr.io/wikid82/charon:latest
```

**Nightly Build (Testing - Docker Hub):**

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 443:443/udp \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  -e CHARON_ENCRYPTION_KEY=your-32-byte-base64-key-here \
  wikid82/charon:nightly
```

> **Note:** Nightly builds include the latest development features and are rebuilt daily at 02:00 UTC. Use for testing only. Also available via GHCR: `ghcr.io/wikid82/charon:nightly`

### What Just Happened?

1. Charon downloaded and started
2. The web interface opened on port 8080
3. Your websites will use ports 80 (HTTP) and 443 (HTTPS)

**Open <http://localhost:8080>** and start adding your websites!

### Requirements

**Server:**

- Docker 20.10+ or Docker Compose V2
- Linux, macOS, or Windows with WSL2

**Browser:**

- Tested with React 19.2.3
- Compatible with modern browsers:
  - Chrome/Edge 90+
  - Firefox 88+
  - Safari 14+
  - Opera 76+

> **Note:** If you encounter errors after upgrading, try a hard refresh (`Ctrl+Shift+R`) or clearing your browser cache. See [Troubleshooting Guide](docs/troubleshooting/react-production-errors.md) for details.

### Development Setup

**Install golangci-lint** (for contributors): `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
See [CONTRIBUTING.md](CONTRIBUTING.md) for complete development environment setup.

### Upgrading? Run Migrations

If you're upgrading from a previous version with persistent data:

```bash
docker exec charon /app/charon migrate
docker restart charon
```

This ensures security features (especially CrowdSec) work correctly.

**Important:** If you had CrowdSec enabled before the upgrade, it will **automatically restart** after migration. You don't need to manually re-enable it via the GUI. See [Migration Guide](https://wikid82.github.io/charon/migration-guide) for details.

---

## 🔔 Smart Notifications

Stay informed about your infrastructure with flexible notification support.

### Supported Services

Charon integrates with popular notification platforms using JSON templates for rich formatting:

- **Discord** — Rich embeds with colors, fields, and custom formatting
- **Slack** — Block Kit messages with interactive elements
- **Gotify** — Self-hosted push notifications with priority levels
- **Telegram** — Instant messaging with Markdown support
- **Generic Webhooks** — Connect to any service with custom JSON payloads

### JSON Template Examples

**Discord Rich Embed:**

```json
{
  "embeds": [{
    "title": "🚨 {{.Title}}",
    "description": "{{.Message}}",
    "color": 15158332,
    "timestamp": "{{.Timestamp}}",
    "fields": [
      {"name": "Host", "value": "{{.HostName}}", "inline": true},
      {"name": "Event", "value": "{{.EventType}}", "inline": true}
    ]
  }]
}
```

**Slack Block Kit:**

```json
{
  "blocks": [
    {
      "type": "header",
      "text": {"type": "plain_text", "text": "🔔 {{.Title}}"}
    },
    {
      "type": "section",
      "text": {"type": "mrkdwn", "text": "*Event:* {{.EventType}}\n*Message:* {{.Message}}"}
    }
  ]
}
```

### Available Template Variables

All JSON templates support these variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Title}}` | Event title | "SSL Certificate Renewed" |
| `{{.Message}}` | Event details | "Certificate for example.com renewed" |
| `{{.EventType}}` | Type of event | "ssl_renewal", "uptime_down" |
| `{{.Severity}}` | Severity level | "info", "warning", "error" |
| `{{.HostName}}` | Affected host | "example.com" |
| `{{.Timestamp}}` | ISO 8601 timestamp | "2025-12-24T10:30:00Z" |

**[📖 Complete Notification Guide →](docs/features/notifications.md)**

---

## Getting Help

**[📖 Full Documentation](https://wikid82.github.io/charon/)** — Everything explained simply
**[🚀 5-Minute Guide](https://wikid82.github.io/charon/getting-started)** — Your first website up and running
**[🔐 Supply Chain Security](docs/guides/supply-chain-security-user-guide.md)** — Verify signatures and build provenance
**[🛠️ Troubleshooting](docs/troubleshooting/)** — Common issues and solutions
**[💬 Ask Questions](https://github.com/Wikid82/charon/discussions)** — Friendly community help
**[🐛 Report Problems](https://github.com/Wikid82/charon/issues)** — Something broken? Let us know

---
