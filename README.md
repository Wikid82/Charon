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
  <a href="https://www.repostatus.org/#active"><img src="https://www.repostatus.org/badges/latest/active.svg" alt="Project Status: Active – The project is being actively developed." /></a><a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://github.com/Wikid82/charon/releases"><img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Release"></a>
  <a href="https://github.com/Wikid82/charon/actions"><img src="https://img.shields.io/github/actions/workflow/status/Wikid82/charon/docker-publish.yml" alt="Build Status"></a>
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

## What Can It Do?

🔐 **Automatic HTTPS** — Free certificates that renew themselves
🛡️ **Optional Security** — Block bad guys, bad countries, or bad behavior
🐳 **Finds Docker Apps** — Sees your containers and sets them up instantly
📥 **Imports Old Configs** — Bring your Caddy setup with you
⚡ **No Downtime** — Changes happen instantly, no restarts needed
🎨 **Dark Mode UI** — Easy on the eyes, works on phones

**[See everything it can do →](https://wikid82.github.io/charon/features)**

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

**Open http://localhost:8080** and start adding your websites!

---

## Optional: Turn On Security

Charon includes **Cerberus**, a security guard for your apps. It's turned off by default so it doesn't get in your way.

When you're ready, add these lines to enable protection:

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=monitor        # Watch for attacks
  - CERBERUS_SECURITY_CROWDSEC_MODE=local     # Block bad IPs automatically
```

**Start with "monitor" mode** — it watches but doesn't block. Once you're comfortable, change `monitor` to `block`.

**[Learn about security features →](https://wikid82.github.io/charon/security)**

---

## Getting Help

**[📖 Full Documentation](https://wikid82.github.io/charon/)** — Everything explained simply
**[🚀 5-Minute Guide](https://wikid82.github.io/charon/getting-started)** — Your first website up and running
**[💬 Ask Questions](https://github.com/Wikid82/charon/discussions)** — Friendly community help
**[🐛 Report Problems](https://github.com/Wikid82/charon/issues)** — Something broken? Let us know

---

## Contributing

Want to help make Charon better? Check out [CONTRIBUTING.md](CONTRIBUTING.md)

---

## ✨ Top Features



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
