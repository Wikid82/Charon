---
title: Getting Started with Charon
description: Get your first website up and running in minutes. A beginner-friendly guide to setting up Charon reverse proxy.
---

## Getting Started with Charon

**Welcome!** Let's get your first website up and running. No experience needed.

---

## What Is This?

Imagine you have several apps running on your computer. Maybe a blog, a file storage app, and a chat server.

**The problem:** Each app is stuck on a weird address like `192.168.1.50:3000`. Nobody wants to type that.

**Charon's solution:** You tell Charon "when someone visits myblog.com, send them to that app." Charon handles everything else—including the green lock icon (HTTPS) that makes browsers happy.

---

## Step 1: Install Charon

### Option A: Docker Compose (Easiest)

Create a file called `docker-compose.yml`:

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
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

### Option B: Docker Run (One Command)

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  ghcr.io/wikid82/charon:latest
```

### What Just Happened?

- **Port 80** and **443**: Where your websites will be accessible (like mysite.com)
- **Port 8080**: The control panel where you manage everything
- **Docker socket**: Lets Charon see your other Docker containers

**Open <http://localhost:8080>** in your browser!

---

## Step 1.5: Database Migrations (If Upgrading)

If you're **upgrading from a previous version** and using a persistent database, you may need to run migrations to ensure all security features work correctly.

### When to Run Migrations

Run the migration command if:

- ✅ You're upgrading from an older version of Charon
- ✅ You're using a persistent volume for `/app/data`
- ✅ CrowdSec features aren't working after upgrade

**Skip this step if:**

- ❌ This is a fresh installation (migrations run automatically)
- ❌ You're not using persistent storage

### How to Run Migrations

**Docker Compose:**

```bash
docker exec charon /app/charon migrate
```

**Docker Run:**

```bash
docker exec charon /app/charon migrate
```

**Expected Output:**

```json
{"level":"info","msg":"Running database migrations for security tables...","time":"..."}
{"level":"info","msg":"Migration completed successfully","time":"..."}
```

**What This Does:**

- Creates or updates security-related database tables
- Adds CrowdSec integration support
- Ensures all features work after upgrade
- **Safe to run multiple times** (idempotent)

**After Migration:**

If you enabled CrowdSec before the migration, restart the container:

```bash
docker restart charon
```

**Auto-Start Behavior:**

CrowdSec will automatically start if it was previously enabled. The reconciliation function runs at startup and checks:

1. **SecurityConfig table** for `crowdsec_mode = "local"`
2. **Settings table** for `security.crowdsec.enabled = "true"`
3. **Starts CrowdSec** if either condition is true

You'll see this in the logs:

```json
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'"}
```

**Verification:**

```bash
# Wait 15 seconds for LAPI to initialize
sleep 15

# Check if CrowdSec auto-started
docker exec charon cscli lapi status
```

Expected output:

```
✓ You can successfully interact with Local API (LAPI)
```

**If auto-start didn't work:** See [CrowdSec Not Starting After Restart](troubleshooting/crowdsec.md#crowdsec-not-starting-after-container-restart) for detailed troubleshooting steps.

---

## Step 2: Add Your First Website

Let's say you have an app running at `192.168.1.100:3000` and you want it available at `myapp.example.com`.

1. **Click "Proxy Hosts"** in the sidebar
2. **Click the "+ Add" button**
3. **Fill in the form:**
   - **Domain:** `myapp.example.com`
   - **Forward To:** `192.168.1.100`
   - **Port:** `3000`
   - **Scheme:** `http` (or `https` if your app already has SSL)
   - **Enable Standard Proxy Headers:** ✅ (recommended — allows your app to see the real client IP)
4. **Click "Save"**

**Done!** When someone visits `myapp.example.com`, they'll see your app.

### What Are Standard Proxy Headers?

By default (and recommended), Charon adds special headers to requests so your app knows:

- **The real client IP address** (instead of seeing Charon's IP)
- **Whether the original connection was HTTPS** (for proper security and redirects)
- **The original hostname** (for virtual host routing)

**When to disable:** Only turn this off for legacy applications that don't understand these headers.

**Learn more:** See [Standard Proxy Headers](features.md#-standard-proxy-headers) in the features guide.

---

## Step 3: Get HTTPS (The Green Lock)

For this to work, you need:

1. **A real domain name** (like example.com) pointed at your server
2. **Ports 80 and 443 open** in your firewall

If you have both, Charon will automatically:

- Request a free SSL certificate from a trusted provider
- Install it
- Renew it before it expires

**You don't do anything.** It just works.

By default, Charon uses "Auto" mode, which tries Let's Encrypt first and automatically falls back to ZeroSSL if needed. You can change this in System Settings if you want to use a specific certificate provider.

**Testing without a domain?** See [Testing SSL Certificates](acme-staging.md) for a practice mode.

---

## Common Questions

### "Where do I get a domain name?"

You buy one from places like:

- Namecheap
- Google Domains
- Cloudflare

Cost: Usually $10-15/year.

### "How do I point my domain at my server?"

In your domain provider's control panel:

1. Find "DNS Settings" or "Domain Management"
2. Create an "A Record"
3. Set it to your server's IP address

Wait 5-10 minutes for it to update.

### "Can I change which certificate provider is used?"

Yes! Go to **System Settings** and look for the **SSL Provider** dropdown. The default "Auto" mode works best for most users, but you can choose a specific provider if needed. See [Features](features.md#choose-your-ssl-provider) for details.

### "Can I use this for apps on different computers?"

Yes! Just use the other computer's IP address in the "Forward To" field.

If you're using Tailscale or another VPN, use the VPN IP.

### "Will this work with Docker containers?"

Absolutely. Charon can even detect them automatically:

1. Click "Proxy Hosts"
2. Click "Docker" tab
3. You'll see all your running containers
4. Click one to auto-fill the form

---

## What's Next?

Now that you have the basics:

- **[See All Features](features.md)** — Discover what else Charon can do
- **[Import Your Old Config](import-guide.md)** — Bring your existing Caddy setup
- **[Configure Optional Features](features.md#%EF%B8%8F-optional-features)** — Enable/disable features like security and uptime monitoring
- **[Turn On Security](security.md)** — Block attackers (enabled by default, highly recommended)

---

## Staying Updated

### Security Update Notifications

To receive notifications about security updates:

**1. GitHub Watch**

Click "Watch" → "Custom" → Select "Security advisories" on the [Charon repository](https://github.com/Wikid82/Charon)

**2. Automatic Updates with Watchtower**

```yaml
services:
  watchtower:
    image: containrrr/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - WATCHTOWER_CLEANUP=true
      - WATCHTOWER_POLL_INTERVAL=86400  # Check daily
```

**3. Diun (Docker Image Update Notifier)**

For notification-only (no auto-update), use [Diun](https://crazymax.dev/diun/). This sends alerts when new images are available without automatically updating.

**Best Practices:**

- Subscribe to GitHub security advisories for early vulnerability warnings
- Review changelogs before updating production deployments
- Test updates in a staging environment first
- Keep backups before major version upgrades

---

## Stuck?

**[Ask for help](https://github.com/Wikid82/charon/discussions)** — The community is friendly!

## Maintainers: History-rewrite Tools

If you are a repository maintainer and need to run the history-rewrite utilities, find the scripts in `scripts/history-rewrite/`.

Minimum required tools:

- `git` — install: `sudo apt-get update && sudo apt-get install -y git` (Debian/Ubuntu) or `brew install git` (macOS).
- `git-filter-repo` — recommended install via pip: `pip install --user git-filter-repo` or via your package manager if available: `sudo apt-get install git-filter-repo`.
- `pre-commit` — install via pip or package manager: `pip install --user pre-commit` and then `pre-commit install` in the repository.

Quick checks before running scripts:

```bash
# Fetch full history (non-shallow)
git fetch --unshallow || true
command -v git || (echo "install git" && exit 1)
command -v git-filter-repo || (echo "install git-filter-repo" && exit 1)
command -v pre-commit || (echo "install pre-commit" && exit 1)
```

See `docs/plans/history_rewrite.md` for the full checklist, usage examples, and recovery steps.
