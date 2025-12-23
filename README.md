<p align="center">
  <img src="frontend/public/banner.png" alt="Charon" width="600">
</p>

<h1 align="center">Charon</h1>

<p align="center"><strong>Your websites, your rules—without the headaches.</strong></p>

<p align="center">
Turn multiple websites and apps into one simple dashboard. Click, save, done. No code, no config files, no PhD required.
</p>

<br>

<p align="center">
  <a href="https://www.repostatus.org/#active"><img src="https://www.repostatus.org/badges/latest/active.svg" alt="Project Status: Active – The project is being actively developed." /></a>
  <a href="https://www.bestpractices.dev/projects/11648"><img src="https://www.bestpractices.dev/projects/11648/badge"></a>
 <br>
 <a href="https://codecov.io/gh/Wikid82/Charon" ><img src="https://codecov.io/gh/Wikid82/Charon/branch/main/graph/badge.svg?token=RXSINLQTGE" alt="Code Coverage"/></a>
  <a href="https://github.com/Wikid82/charon/releases"><img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Release"></a>
 <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
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

## ✨ Top 10 Features

### 🎯 **Point & Click Management**

No config files. No terminal commands. Just click, type your domain name, and you're live. If you can use a website, you can run Charon.

### 🔐 **Automatic HTTPS Certificates**

Free SSL certificates that request, install, and renew themselves. Your sites get the green padlock without you lifting a finger.

### 🛡️ **Enterprise-Grade Security Built In**

Web Application Firewall, rate limiting, geographic blocking, access control lists, and intrusion detection via CrowdSec. Protection that "just works."

### 🔗 **Smart Proxy Headers**

Automatically adds standard headers (X-Real-IP, X-Forwarded-Proto, etc.) so your backend applications see real client IPs, enforce HTTPS correctly, and log accurately—with full backward compatibility for existing hosts.

### 🐳 **Instant Docker Discovery**

Already running apps in Docker? Charon finds them automatically and offers one-click proxy setup. No manual configuration required.

### 📊 **Real-Time Monitoring & Logs**

See exactly what's happening with live request logs, uptime monitoring, and instant notifications when something goes wrong.

### 📥 **Migration Made Easy**

Import your existing Caddy configurations with one click. Already invested in another reverse proxy? Bring your work with you.

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

### Docker Compose (Recommended)

Save this as `docker-compose.yml`:

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
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

```

Then run:

```bash
docker-compose up -d
```

### Docker Run (One-Liner)

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
  ghcr.io/wikid82/charon:latest
```

### What Just Happened?

1. Charon downloaded and started
2. The web interface opened on port 8080
3. Your websites will use ports 80 (HTTP) and 443 (HTTPS)

**Open <http://localhost:8080>** and start adding your websites!

### Upgrading? Run Migrations

If you're upgrading from a previous version with persistent data:

```bash
docker exec charon /app/charon migrate
docker restart charon
```

This ensures security features (especially CrowdSec) work correctly.

**Important:** If you had CrowdSec enabled before the upgrade, it will **automatically restart** after migration. You don't need to manually re-enable it via the GUI. See [Migration Guide](https://wikid82.github.io/charon/migration-guide) for details.

---

## Security Features

### SSRF Protection

Charon implements comprehensive Server-Side Request Forgery (SSRF) protection for all user-controlled URLs:

- ✅ **Private IP Blocking**: Prevents access to RFC 1918 private networks
- ✅ **Cloud Metadata Protection**: Blocks AWS, GCP, Azure metadata endpoints
- ✅ **Protocol Restrictions**: Only allows HTTP/HTTPS schemes
- ✅ **DNS Validation**: Resolves and validates all IPs before requests
- ✅ **Defense in Depth**: Multiple validation layers for maximum security

**Protected Features:**
- Security notification webhooks
- Custom webhook notifications
- CrowdSec hub synchronization
- External URL connectivity testing

**Learn More:** See [docs/security/ssrf-protection.md](docs/security/ssrf-protection.md)

---

## Getting Help

**[📖 Full Documentation](https://wikid82.github.io/charon/)** — Everything explained simply
**[🚀 5-Minute Guide](https://wikid82.github.io/charon/getting-started)** — Your first website up and running
**[💬 Ask Questions](https://github.com/Wikid82/charon/discussions)** — Friendly community help
**[🐛 Report Problems](https://github.com/Wikid82/charon/issues)** — Something broken? Let us know

---

## Authentication Flow

### How Authentication Works

Charon uses a **three-tier authentication system** to validate user sessions:

1. **Authorization Header** (`Authorization: Bearer <token>`) — Checked first
2. **HTTP Cookie** (`authToken`) — Checked if no header present
3. **Query Parameter** (`?token=<token>`) — Fallback for WebSocket connections

### Expected 401 Responses

When you first access Charon or your session expires, you'll see this in the browser console:

```
GET /api/auth/me → 401 Unauthorized
```

**This is normal and expected behavior.** Here's why:

- The frontend checks authentication status on page load
- If you're not logged in, the API returns 401
- The frontend receives this response and shows the login page
- Once you log in, the 401 errors disappear

**Development tip:** These 401 responses are not errors—they're the API's way of saying "authentication required." Modern SPAs (Single Page Applications) expect and handle these responses gracefully.

### Authentication Verification

After logging in, Charon validates your session on every API request:

```
GET /api/auth/me → 200 OK
```

**Response includes:**

- User ID and username
- Role and permissions
- Session expiration time

**Token refresh:** Sessions automatically extend on activity. The default session timeout is 24 hours.

### Security Considerations

- ✅ All authentication tokens use cryptographically secure random generation
- ✅ Tokens are hashed before storage in the database
- ✅ Sessions expire after inactivity
- ✅ HTTPS enforces `Secure` cookie attributes in production
- ✅ `HttpOnly` flag prevents JavaScript access to auth cookies

**Learn more:** See [Security Features](https://wikid82.github.io/charon/security) for complete authentication and authorization documentation.

---

## Agent Skills

Charon uses [Agent Skills](https://agentskills.io) for AI-discoverable, executable development tasks. Skills are self-documenting task definitions that can be executed by both humans and AI assistants like GitHub Copilot.

### What are Agent Skills?

Agent Skills combine YAML metadata with Markdown documentation to create standardized, AI-discoverable task definitions. Each skill represents a specific development task (testing, building, security scanning, etc.) that can be:

- ✅ **Executed directly** via command line
- ✅ **Discovered by AI** assistants (GitHub Copilot, etc.)
- ✅ **Run from VS Code** tasks menu
- ✅ **Integrated in CI/CD** pipelines

### Available Skills

Charon provides 19 operational skills across multiple categories:

- **Testing** (4 skills): Backend/frontend unit tests and coverage analysis
- **Integration** (5 skills): CrowdSec, Coraza, and full integration test suites
- **Security** (2 skills): Trivy vulnerability scanning and Go security checks
- **QA** (1 skill): Pre-commit hooks and code quality checks
- **Utility** (4 skills): Version management, cache clearing, database recovery
- **Docker** (3 skills): Development environment management

### Using Skills

**Command Line:**
```bash
# Run backend tests with coverage
.github/skills/scripts/skill-runner.sh test-backend-coverage

# Run security scan
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

**VS Code Tasks:**
1. Open Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`)
2. Select `Tasks: Run Task`
3. Choose your skill (e.g., `Test: Backend with Coverage`)

**GitHub Copilot:**
Simply ask Copilot to run tasks naturally:
- "Run backend tests with coverage"
- "Start the development environment"
- "Run security scans"

### Learning More

- **[Agent Skills Documentation](.github/skills/README.md)** — Complete skill reference
- **[agentskills.io Specification](https://agentskills.io/specification)** — Standard format details
- **[Migration Guide](docs/AGENT_SKILLS_MIGRATION.md)** — Transition from legacy scripts

---

## Contributing

Want to help make Charon better? Check out [CONTRIBUTING.md](CONTRIBUTING.md)

---

<p align="center">
  <a href="LICENSE"><strong>MIT License</strong></a> ·
  <a href="https://wikid82.github.io/charon/"><strong>Documentation</strong></a> ·
  <a href="https://github.com/Wikid82/charon/releases"><strong>Releases</strong></a>
</p>

<p align="center">
  <em>Built with ❤️ by <a href="https://github.com/Wikid82">@Wikid82</a></em><br>
  <sub>Powered by <a href="https://caddyserver.com/">Caddy Server</a></sub>
</p>
