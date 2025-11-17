# CaddyProxyManager+ Project Overview

## What is CaddyProxyManager+?

CaddyProxyManager+ is a modern web UI for managing Caddy Server reverse proxy configurations, specifically designed for home-lab and self-hosted environments. It combines the simplicity of Nginx Proxy Manager with the power and modern design of Caddy Server, while adding enterprise-grade security features that are accessible through an easy-to-use interface.

## Why Was This Created?

The home-lab community loves Nginx Proxy Manager (NPM) for its simplicity, but many technically-minded users prefer Caddy for:
- **Automatic HTTPS:** Zero-config SSL certificates with Let's Encrypt
- **Modern Architecture:** Built with Go, HTTP/3 support, better performance
- **Powerful TLS:** Advanced certificate management, mTLS, DNS challenges

However, Caddy lacks an easy-to-use web UI. CaddyProxyManager+ fills this gap by providing:
- Simple web interface like NPM
- Access to Caddy's advanced features
- Enterprise security features made simple
- Perfect for self-hosted services (Plex, Jellyfin, Sonarr, Radarr, etc.)

## Key Features at a Glance

### 🔐 Authentication & Access Control
- **Single Sign-On (SSO):** One-click integration with Authelia, Authentik, Pomerium
- **Basic Auth:** Simple username/password protection
- **IP Access Control:** Whitelist/blacklist specific IPs
- **Geo-blocking:** Restrict access by country
- **Local Network Only:** RFC1918 private network restriction

### 🛡️ Threat Protection
- **Web Application Firewall:** Coraza WAF with OWASP Core Rule Set
- **Rate Limiting:** Smart presets for login pages, APIs, standard web
- **Security Headers:** HSTS, CSP, X-Frame-Options, and more
- **CrowdSec Integration:** Community-powered threat intelligence

### 🚦 Traffic & TLS Management
- **Automatic HTTPS:** Let's Encrypt certificates, automatic renewal
- **DNS Challenge:** For internal servers and wildcard certificates (*.example.com)
- **mTLS:** Client certificate authentication for zero-trust setups
- **HSTS Preload:** Submit your domain to browser preload lists

### 📊 User Interface
- **Modern Design:** Dark theme, responsive layout
- **Easy Configuration:** Tabbed interface for all settings
- **Visual Feedback:** Security badges show active features
- **Real-time Updates:** Changes apply immediately

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    User's Browser                        │
│              (http://localhost:8080)                     │
└───────────────────────┬─────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              CaddyProxyManager+ Web UI                   │
│                                                          │
│  ┌────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │   Login    │  │ Proxy Hosts  │  │   Security     │  │
│  │ (JWT Auth) │  │  Management  │  │   Dashboard    │  │
│  └────────────┘  └──────────────┘  └────────────────┘  │
└───────────────────────┬─────────────────────────────────┘
                        │ REST API
                        ▼
┌─────────────────────────────────────────────────────────┐
│           CaddyProxyManager+ Backend (Go)                │
│                                                          │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐   │
│  │   API        │  │   Config    │  │   Database   │   │
│  │   Handlers   │  │  Generator  │  │   (SQLite)   │   │
│  └──────────────┘  └─────────────┘  └──────────────┘   │
└───────────────────────┬─────────────────────────────────┘
                        │ Admin API
                        ▼
┌─────────────────────────────────────────────────────────┐
│                    Caddy Server                          │
│                                                          │
│  Handles:                                                │
│  • Reverse proxying                                      │
│  • SSL/TLS certificates                                  │
│  • Security features (WAF, rate limiting, etc.)          │
│  • Traffic routing                                       │
└───────────────────────┬─────────────────────────────────┘
                        │
                        ▼
              Your Backend Services
        (Plex, Jellyfin, Sonarr, etc.)
```

## Technology Stack

- **Backend:** Go 1.21+ with Gin web framework
- **Database:** SQLite with GORM ORM
- **Frontend:** HTML, CSS, JavaScript (no build step)
- **Proxy:** Caddy 2.7+
- **Authentication:** JWT tokens with bcrypt password hashing
- **Deployment:** Docker, Docker Compose, or standalone binary

## Use Cases

### Home Media Server
Protect Plex or Jellyfin with SSO authentication and geo-blocking to prevent unauthorized international access.

### Arr Suite (Sonarr, Radarr, etc.)
Add authentication and rate limiting to APIs that might not have built-in protection.

### WordPress/Blog
Enable WAF to protect against common attacks, rate limiting to prevent brute force, and HSTS for security.

### Internal Services
Use "Local Network Only" for admin panels and internal tools that should never be public.

### Public APIs
Implement mTLS for machine-to-machine authentication with client certificates.

## Comparison with Alternatives

| Feature | CaddyProxyManager+ | Nginx Proxy Manager | Traefik | Caddy (CLI) |
|---------|-------------------|---------------------|---------|-------------|
| Web UI | ✅ | ✅ | ❌ | ❌ |
| Automatic HTTPS | ✅ | ✅ | ✅ | ✅ |
| Zero Config SSL | ✅ | ❌ | ❌ | ✅ |
| DNS Challenge UI | ✅ | ✅ | ❌ | ❌ |
| Forward Auth UI | ✅ | ❌ | ⚠️ | ❌ |
| WAF Integration | ✅ | ❌ | ⚠️ | ⚠️ |
| Rate Limiting UI | ✅ | ❌ | ⚠️ | ❌ |
| Geo-blocking UI | ✅ | ❌ | ❌ | ❌ |
| mTLS UI | ✅ | ❌ | ❌ | ❌ |
| Modern Stack | ✅ Go | ❌ PHP | ✅ Go | ✅ Go |
| HTTP/3 | ✅ | ❌ | ✅ | ✅ |

## Security Features Deep Dive

### Why These Features Matter

**For Home Labs:**
- Your home IP is exposed to the internet
- Self-hosted services often lack robust authentication
- Automated bots constantly scan for vulnerabilities
- You need protection without complexity

**What CaddyProxyManager+ Provides:**

1. **Forward Auth:** Single login for all services
   - One authentication system
   - No need to manage passwords per service
   - Professional SSO setup in minutes

2. **WAF:** Protection against common attacks
   - SQL injection blocking
   - XSS prevention
   - Path traversal protection
   - 100+ OWASP rules

3. **Rate Limiting:** Prevents brute force attacks
   - Automatic IP-based limiting
   - Smart presets for different services
   - No manual iptables rules needed

4. **Geo-blocking:** Reduces attack surface
   - Block entire countries
   - 80%+ reduction in bot traffic
   - Simple country code selection

5. **CrowdSec:** Community-powered protection
   - Shared threat intelligence
   - Automatic bad actor blocking
   - Real-time updates

## Getting Started

### 5-Minute Setup

1. **Install Docker** (if not already installed)
2. **Clone and Start:**
   ```bash
   git clone https://github.com/Wikid82/CaddyProxyManagerPlus.git
   cd CaddyProxyManagerPlus
   docker-compose up -d
   ```
3. **Access:** http://localhost:8080
4. **Login:** admin / admin (change immediately!)
5. **Add your first proxy host**

### First Proxy Host

1. Click "Add Proxy Host"
2. Enter your domain and backend service details
3. Enable SSL/TLS (automatic certificate)
4. Enable security features (WAF, rate limiting)
5. Save and test!

## Documentation

The project includes comprehensive documentation:

- **[README.md](README.md):** Project overview and features
- **[QUICKSTART.md](QUICKSTART.md):** Get running in 5 minutes
- **[INSTALLATION.md](INSTALLATION.md):** Detailed installation guide
- **[SECURITY_FEATURES.md](SECURITY_FEATURES.md):** In-depth security documentation
- **[EXAMPLES.md](EXAMPLES.md):** 10+ real-world configurations
- **[CONTRIBUTING.md](CONTRIBUTING.md):** How to contribute
- **[CHANGELOG.md](CHANGELOG.md):** Version history

## Project Status

**Current Version:** 0.1.0 (Initial Release)

**Status:** Production Ready ✅

**Security:**
- Zero known vulnerabilities
- CodeQL security scanning passed
- All dependencies scanned
- Secure by default

**Testing:**
- Unit tests for core modules
- CI/CD with GitHub Actions
- Docker build tested
- Manual testing completed

## Roadmap

### Near Term (v0.2.0)
- Live log viewer in UI
- CrowdSec dashboard integration
- GoAccess analytics integration
- User management UI

### Medium Term (v0.3.0)
- Certificate management UI
- Backup/restore functionality
- Configuration import/export
- Template system

### Long Term (v0.4.0+)
- Multi-user with role-based access
- Mobile app (PWA)
- Prometheus metrics
- Plugin system

## Contributing

We welcome contributions! The project needs:
- Bug reports and fixes
- Feature requests and implementations
- Documentation improvements
- Testing and feedback
- Real-world usage examples

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Community

- **Issues:** [GitHub Issues](https://github.com/Wikid82/CaddyProxyManagerPlus/issues)
- **Discussions:** [GitHub Discussions](https://github.com/Wikid82/CaddyProxyManagerPlus/discussions)
- **Documentation:** This repository

## Acknowledgments

Built with:
- [Caddy Server](https://caddyserver.com/) - Modern web server
- [Gin](https://gin-gonic.com/) - Go web framework
- [GORM](https://gorm.io/) - Go ORM
- [CrowdSec](https://www.crowdsec.net/) - Collaborative security

Inspired by:
- [Nginx Proxy Manager](https://nginxproxymanager.com/) - UI inspiration
- The home-lab community - Use case understanding

## License

MIT License - see [LICENSE](LICENSE) file.

Free for personal and commercial use. Contributions welcome!

## Support

For help:
1. Check the documentation
2. Search existing issues
3. Open a new issue with details
4. Join community discussions

## Final Notes

CaddyProxyManager+ is built for people who:
- Want NPM's simplicity with Caddy's power
- Run home labs with Plex, Jellyfin, etc.
- Need enterprise security made simple
- Value automatic HTTPS and modern features
- Don't want to edit config files

If that's you, welcome! We hope this tool makes your self-hosting experience better and more secure.

**Happy proxying! 🚀**
