<p align="center">
  <img src="https://raw.githubusercontent.com/Wikid82/Charon/refs/heads/main/frontend/public/banner.webp" alt="Charon" width="350">
</p>

<h1 align="center">Charon</h1>

<p align="center">
  <strong>Your server, your rules—without the headaches.</strong>
</p>

<p align="center">
  Manage reverse proxies with a clean web interface.<br>
  No config files. No cryptic syntax. No networking degree required.
</p>

<p align="center">
  <a href="https://hub.docker.com/r/wikid82/charon">
    <img src="https://img.shields.io/docker/pulls/wikid82/charon.svg" alt="Docker Pulls">
  </a>
  <a href="https://github.com/Wikid82/charon/releases">
    <img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Latest Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT License">
  </a>
  <a href="https://discord.gg/Tvzg6BQx">
    <img src="https://img.shields.io/badge/Community-Discord-5865F2?logo=discord&logoColor=white">
  </a>
</p>

---

## 🚀 Why Charon?

You want your apps online.

You don’t want to edit config files or memorize reverse proxy syntax.

Charon gives you:

- ✅ Automatic HTTPS certificates
- ✅ Clean domain routing
- ✅ Built-in security protection
- ✅ One-click Docker app discovery
- ✅ Live updates without restarts
- ✅ Zero external dependencies

If you can use a website, you can run Charon.

---

## 🛡 Built-In Security

Charon includes security features that normally require multiple tools:

- Web Application Firewall (WAF)
- CrowdSec intrusion detection with analytics dashboard
- Access Control Lists (ACLs)
- Rate limiting
- Emergency recovery tools

Secure by default. No extra containers required.

📖 [Learn more about security →](https://wikid82.github.io/charon/security)

---

## ⚡ Quick Start (5 Minutes)

### 1️⃣ Create `docker-compose.yml`

```yaml
services:
  charon:
    image: wikid82/charon:latest
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
      - TZ=America/New_York
      # Generate with: openssl rand -base64 32
      - CHARON_ENCRYPTION_KEY=your-32-byte-base64-key
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:8080/api/v1/health || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

> **Docker Socket Access:** Charon runs as a non-root user. If you mount the Docker socket for container discovery, the container needs permission to read it. Find your socket's group ID and add it to the compose file:
>
> ```bash
> stat -c '%g' /var/run/docker.sock
> ```
>
> Then add `group_add: ["<gid>"]` under your service (replace `<gid>` with the number from the command above). For example, if the result is `998`:
>
> ```yaml
>     group_add:
>       - "998"
> ```

### 2️⃣ Generate encryption key

```bash
openssl rand -base64 32
```

### 3️⃣ Start Charon

```bash
docker-compose up -d
```

### 4️⃣ Access the dashboard

Open your browser and navigate to `http://localhost:8080` to access the dashboard and create your admin account.

```code
http://localhost:8080
```

### Getting Started

Full setup instructions and documentation are available at [https://wikid82.github.io/Charon/docs/getting-started.html](https://wikid82.github.io/Charon/docs/getting-started.html).

--- ## ✨ Top 10 Features

### 🎯 **Point & Click Management**

No config files. No terminal commands. Just click, type your domain name, and you're live. If you can use a website, you can run Charon.

### 🔐 **Automatic HTTPS Certificates**

Free SSL certificates that request, install, and renew themselves. Your sites get the green padlock without you lifting a finger.

### 🌐 **DNS Challenge for Wildcard Certificates**

Secure all your subdomains with a single *.example.com certificate. Supports 15+ DNS providers including Cloudflare, Route53, DigitalOcean, and Google Cloud DNS. Credentials are encrypted and automatically rotated.

### 🛡️ **Enterprise-Grade Security Built In**

Web Application Firewall, rate limiting, geographic blocking, access control lists, and intrusion detection via CrowdSec—with a built-in analytics dashboard showing attack trends, top offenders, and ban history. Protection that "just works."

### 🔐 **Supply Chain Security**

Verifiable builds with cryptographic signatures, SLSA provenance attestation, and comprehensive SBOMs. Verify what you run with transparent, tamper-proof evidence.

### 🌐 **Smart Proxy Headers**

Automatically adds standard headers (X-Real-IP, X-Forwarded-Proto, etc.) so your backend applications see real client IPs, enforce HTTPS correctly, and log accurately—with full backward compatibility for existing hosts.

### 🐳 **Instant Docker Discovery**

Already running apps in Docker? Charon finds them automatically and offers one-click proxy setup. No manual configuration required.

### 📊 **Real-Time Monitoring & Logs**

See exactly what's happening with live request logs, uptime monitoring, and instant notifications when something goes wrong.

### 📥 **Migration Made Easy**

Already invested in another reverse proxy? Bring your work with you by importing your existing configurations with one click:

- **Caddyfile** — Migrate from other Caddy setups
- **Nginx** — Import from Nginx based configurations (Coming Soon)
- **Traefik** - Import from Traefik based configurations (Coming Soon)
- **CrowdSec** - Import from CrowdSec configurations
- **JSON Import** — Restore from Charon backups or generic JSON configs

### ⚡ **Live Configuration Changes**

Update domains, add security rules, or modify settings instantly—no container restarts needed.* Your sites stay up while you make changes.

### 🌍 **Multi-App Management**

Run dozens of websites, APIs, or services from a single dashboard. Perfect for homelab enthusiasts and small teams managing multiple projects.

### 🚀 **Zero-Dependency Deployment**

One Docker container. No databases to install. No external services required. No complexity—just pure simplicity.

### 💯 **100% Free & Open Source**

No premium tiers. No feature paywalls. No usage limits. Everything you see is yours to use, forever, backed by the MIT license. <sup>* Note: Initial security engine setup (CrowdSec) requires a one-time container restart to initialize the protection layer. All subsequent changes happen live.</sup> **

[Explore All Features →](https://github.com/Wikid82/Charon/blob/main/docs/features.md)**

---
💬 Support
<p align="center">   <a href="https://github.com/Wikid82/Charon/issues">
    <img alt="GitHub issues"
         src="https://img.shields.io/github/issues/Wikid82/Charon"><a href="https://github.com/Wikid82/Charon/issues/new/choose"> <img src="https://img.shields.io/badge/Support-Open%20Issue-blue?logo=github"> </a> <a href="https://discord.gg/Tvzg6BQx"> <img src="https://img.shields.io/badge/Community-Discord-5865F2?logo=discord&logoColor=white"> </a> </p>

---

❤️ Free & Open Source

Charon is 100% free and open source under the MIT License.

No premium tiers. No locked features. No usage limits.

Built for the self-hosting community.
