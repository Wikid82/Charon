<p align="center">
  <img src="frontend/public/banner.png" alt="Charon" width="600">
</p>

<h1 align="center">Charon</h1>

<p align="center"> <strong>The Gateway to Effortless Connectivity.</strong>


Charon bridges the gap between the complex internet and your private services. Enjoy a simplified, visual management experience built specifically for the home server enthusiast. No code required—just safe passage. </p>

<h2 align="center">Cerberus</h2>

<p align="center"> <strong>The Guardian at the Gate.</strong>


Ensure nothing passes without permission. Cerberus is a robust security suite featuring the Coraza WAF, deep CrowdSec integration, and granular rate-limiting. Always watching, always protecting. </p>
<br><br>
<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://github.com/Wikid82/charon/releases"><img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Release"></a>
  <a href="https://github.com/Wikid82/charon/actions"><img src="https://img.shields.io/github/actions/workflow/status/Wikid82/charon/docker-publish.yml" alt="Build Status"></a>
</p>

---

## ✨ Top Features

| Feature | Description |
|---------|-------------|
| 🔐 **Automatic HTTPS** | Free SSL certificates from Let's Encrypt, auto-renewed |
| 🛡️ **Built-in Security** | CrowdSec integration, geo-blocking, IP access lists (optional, powered by Cerberus) |
| ⚡ **Zero Downtime** | Hot-reload configuration without restarts |
| 🐳 **Docker Discovery** | Auto-detect containers on local and remote Docker hosts |
| 📊 **Uptime Monitoring** | Know when your services go down with smart notifications |
| 🔍 **Health Checks** | Test connections before saving |
| 📥 **Easy Import** | Bring your existing Caddy configs with one click |
| 💾 **Backup & Restore** | Never lose your settings, export anytime |
| 🌐 **WebSocket Support** | Perfect for real-time apps and chat services |
| 🎨 **Beautiful Dark UI** | Modern interface that's easy on the eyes, works on any device |

**[See all features →](https://wikid82.github.io/charon/features)**

---

## 🚀 Quick Start

```bash
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"        # HTTP (Caddy proxy)
      - "443:443"      # HTTPS (Caddy proxy)
      - "443:443/udp"  # HTTP/3 (Caddy proxy)
      - "8080:8080"    # Management UI (Charon)
    environment:
      - CHARON_ENV=production # New env var prefix (CHARON_). CPM_ values still supported.
      - TZ=UTC # Set timezone (e.g., America/New_York)
      - CHARON_HTTP_PORT=8080
      - CHARON_DB_PATH=/app/data/charon.db
      - CHARON_FRONTEND_DIR=/app/frontend/dist
      - CHARON_CADDY_ADMIN_API=http://localhost:2019
      - CHARON_CADDY_CONFIG_DIR=/app/data/caddy
      - CHARON_CADDY_BINARY=caddy
      - CHARON_IMPORT_CADDYFILE=/import/Caddyfile
      - CHARON_IMPORT_DIR=/app/data/imports
      # Security Services (Optional)
      #- CERBERUS_SECURITY_CROWDSEC_MODE=disabled # disabled, local, external
      #- CERBERUS_SECURITY_CROWDSEC_API_URL= # Required if mode is external
      #- CERBERUS_SECURITY_CROWDSEC_API_KEY= # Required if mode is external
      #- CERBERUS_SECURITY_WAF_MODE=disabled # disabled, enabled
      #- CERBERUS_SECURITY_RATELIMIT_ENABLED=false
      #- CERBERUS_SECURITY_ACL_ENABLED=false
    extra_hosts:
      - "host.docker.internal:host-gateway"
    volumes:
      - <path_to_charon_data>:/app/data
      - <path_to_caddy_data>:/data
      - <path_to_caddy_config>:/config
      - /var/run/docker.sock:/var/run/docker.sock:ro # For local container discovery
      # Mount your existing Caddyfile for automatic import (optional)
      # - ./my-existing-Caddyfile:/import/Caddyfile:ro
      # - ./sites:/import/sites:ro # If your Caddyfile imports other files
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

Open **http://localhost:8080** — that's it! 🎉

**[Full documentation →](https://wikid82.github.io/charon/)**

---

## 💬 Community

- 🐛 **Found a bug?** [Open an issue](https://github.com/Wikid82/charon/issues)
- 💡 **Have an idea?** [Start a discussion](https://github.com/Wikid82/charon/discussions)
- 📋 **Roadmap** [View the project board](https://github.com/users/Wikid82/projects/7)

## 🤝 Contributing

We welcome contributions! See our [Contributing Guide](CONTRIBUTING.md) to get started.

---

<p align="center">
  <a href="LICENSE"><strong>MIT License</strong></a> ·
  <a href="https://wikid82.github.io/charon/"><strong>Documentation</strong></a> ·
  <a href="https://github.com/Wikid82/charon/releases"><strong>Releases</strong></a>
</p>

<p align="center">
  <em>Built with ❤️ by <a href="https://github.com/Wikid82">@Wikid82</a></em><br>
  <sub>Powered by <a href="https://caddyserver.com/">Caddy Server</a> · Inspired by <a href="https://nginxproxymanager.com/">Nginx Proxy Manager</a> & <a href="https://pangolin.net/">Pangolin</a></sub>
</p>
