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

### Required Secrets (Generate Before Installing)

Two secrets must be set before starting Charon. Omitting them will cause **sessions to reset on every container restart**, locking users out.

Generate both values now and keep them somewhere safe:

```bash
# JWT secret — signs and validates login sessions
openssl rand -hex 32

# Encryption key — protects stored credentials at rest
openssl rand -base64 32
```

> **Why this matters:** If `CHARON_JWT_SECRET` is not set, Charon generates a random key on each boot. Any active login session becomes invalid the moment the container restarts, producing a "Session validation failed" error.

---

### Option A: Docker Compose (Easiest)

Create a file called `docker-compose.yml`:

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
      - "8080:8080"
    volumes:
      - ./charon-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - CHARON_ENV=production
      - CHARON_JWT_SECRET=<output of: openssl rand -hex 32>
      - CHARON_ENCRYPTION_KEY=<output of: openssl rand -base64 32>
```

Then run:

```bash
docker-compose up -d
```

### Option B: Docker Run (One Command)

**Docker Hub (recommended):**

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  -e CHARON_JWT_SECRET=<output of: openssl rand -hex 32> \
  -e CHARON_ENCRYPTION_KEY=<output of: openssl rand -base64 32> \
  wikid82/charon:latest
```

**Alternative (GitHub Container Registry):**

```bash
docker run -d \
  --name charon \
  -p 80:80 \
  -p 443:443 \
  -p 8080:8080 \
  -v ./charon-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e CHARON_ENV=production \
  -e CHARON_JWT_SECRET=<output of: openssl rand -hex 32> \
  -e CHARON_ENCRYPTION_KEY=<output of: openssl rand -base64 32> \
  ghcr.io/wikid82/charon:latest
```

### What Just Happened?

- **Port 80** and **443**: Where your websites will be accessible (like mysite.com)
- **Port 8080**: The control panel where you manage everything
- **Docker socket**: Lets Charon see your other Docker containers

**Open <http://localhost:8080>** in your browser!

### Docker Socket Access (Important)

Charon runs as a non-root user inside the container. To discover your other Docker containers, it needs permission to read the Docker socket. Without this, you'll see a "Docker Connection Failed" message in the UI.

**Step 1:** Find your Docker socket's group ID:

```bash
stat -c '%g' /var/run/docker.sock
```

This prints a number (for example, `998` or `999`).

**Step 2:** Add that number to your compose file under `group_add`:

```yaml
services:
  charon:
    image: wikid82/charon:latest
    group_add:
      - "998"   # <-- replace with your number from Step 1
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      # ... rest of your config
```

**Using `docker run` instead?** Add `--group-add <gid>` to your command:

```bash
docker run -d \
  --name charon \
  --group-add 998 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  # ... rest of your flags
  wikid82/charon:latest
```

**Why is this needed?** The Docker socket is owned by a specific group on your host machine. Adding that group lets Charon read the socket without running as root—keeping your setup secure.

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

---

## Step 1.8: Emergency Token Configuration (Development & E2E Tests)

The emergency token is a security feature that allows bypassing all security modules in emergency situations (e.g., lockout scenarios). It is **required for E2E test execution** and recommended for development environments.

### Purpose

- **Emergency Access**: Bypass ACL, WAF, or other security modules when locked out
- **E2E Testing**: Required for running Playwright E2E tests
- **Audit Logged**: All uses are logged for security accountability

### Generation

Choose your platform:

**Linux/macOS (recommended):**

```bash
openssl rand -hex 32
```

**Windows PowerShell:**

```powershell
[Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

**Node.js (all platforms):**

```bash
node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
```

### Local Development

Add to `.env` file in project root:

```bash
CHARON_EMERGENCY_TOKEN=<paste_64_character_token_here>
```

**Example:**

```bash
CHARON_EMERGENCY_TOKEN=7b3b8a36a6fad839f1b3122131ed4b1f05453118a91b53346482415796e740e2
```

**Verify:**

```bash
# Token should be exactly 64 characters
echo -n "$(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" | wc -c
```

### CI/CD (GitHub Actions)

For continuous integration, store the token in GitHub Secrets:

1. Navigate to: **Repository Settings → Secrets and Variables → Actions**
2. Click **"New repository secret"**
3. **Name:** `CHARON_EMERGENCY_TOKEN`
4. **Value:** Generate with one of the methods above
5. Click **"Add secret"**

📖 **Detailed Instructions:** See [GitHub Setup Guide](github-setup.md)

### Rotation Schedule

- **Recommended:** Rotate quarterly (every 3 months)
- **Required:** After suspected compromise or team member departure
- **Process:**
  1. Generate new token
  2. Update `.env` (local) and GitHub Secrets (CI/CD)
  3. Restart services
  4. Verify with E2E tests

### Security Best Practices

✅ **DO:**

- Generate tokens using cryptographically secure methods
- Store in `.env` (gitignored) or secrets management
- Rotate quarterly or after security events
- Use minimum 64 characters

❌ **DON'T:**

- Commit tokens to repository (even in examples)
- Share tokens via email or chat
- Use weak or predictable values
- Reuse tokens across environments

---

1. **Settings table** for `security.crowdsec.enabled = "true"`
2. **Starts CrowdSec** if either condition is true

**How it works:**

- Reconciliation happens **before** the HTTP server starts (during container boot)
- Protected by mutex to prevent race conditions
- Validates binary and config paths before starting
- Verifies process is running after start (2-second health check)

You'll see this in the logs:

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check"}
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'"}
{"level":"info","msg":"CrowdSec reconciliation: successfully started and verified CrowdSec","pid":123}
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

**Troubleshooting:**

If CrowdSec doesn't auto-start:

1. **Check reconciliation logs:**

   ```bash
   docker logs charon 2>&1 | grep "CrowdSec reconciliation"
   ```

2. **Verify SecurityConfig mode:**

   ```bash
   docker exec charon sqlite3 /app/data/charon.db \
     "SELECT crowdsec_mode FROM security_configs LIMIT 1;"
   ```

   Expected: `local`

3. **Check directory permissions:**

   ```bash
   docker exec charon ls -la /var/lib/crowdsec/data/
   ```

   Expected: `charon:charon` ownership

4. **Manual start:**

   ```bash
   curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
   ```

**For detailed troubleshooting:** See [CrowdSec Startup Fix Documentation](implementation/crowdsec_startup_fix_COMPLETE.md)

---

## Step 2: Configure Application URL (Recommended)

Before inviting users, you should configure your Application URL. This ensures invite links work correctly from external networks.

**What it does:** Sets the public URL used in user invitation emails and links.

**When you need it:** If you plan to invite users or access Charon from external networks.

**How to configure:**

1. **Go to System Settings** (gear icon in sidebar)
2. **Scroll to "Application URL" section**
3. **Enter your public URL** (e.g., `https://charon.example.com`)
   - Must start with `http://` or `https://`
   - Should be the URL users use to access Charon
   - No path components (e.g., `/admin`)
4. **Click "Validate"** to check the format
5. **Click "Test"** to verify the URL opens in a new tab
6. **Click "Save Changes"**

**What happens if you skip this?** User invitation emails will use the server's local address (like `http://localhost:8080`), which won't work from external networks. You'll see a warning when previewing invite links.

**Examples:**

- ✅ `https://charon.example.com`
- ✅ `https://proxy.mydomain.net`
- ✅ `http://192.168.1.100:8080` (for internal networks only)
- ❌ `charon.example.com` (missing protocol)
- ❌ `https://charon.example.com/admin` (no paths allowed)

---

## Step 3: Add Your First Website

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

## Step 4: Get HTTPS (The Green Lock)

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

## Common Development Warnings

### Expected Browser Console Warnings

When developing locally, you may encounter these browser warnings. They are **normal and safe to ignore** in development mode:

#### COOP Warning on HTTP Non-Localhost IPs

```
Cross-Origin-Opener-Policy policy would block the window.closed call.
```

**When you'll see this:**

- Accessing Charon via HTTP (not HTTPS)
- Using a non-localhost IP address (e.g., `http://192.168.1.100:8080`)
- Testing from a different device on your local network

**Why it appears:**

- COOP header is disabled in development mode for convenience
- Browsers enforce stricter security checks on HTTP connections to non-localhost IPs
- This protection is enabled automatically in production HTTPS mode

**What to do:** Nothing! This is expected behavior. The warning disappears when you deploy to production with HTTPS.

**Learn more:** See [COOP Behavior](security.md#coop-cross-origin-opener-policy-behavior) in the security documentation.

#### 401 Errors During Authentication Checks

```
GET /api/auth/me → 401 Unauthorized
```

**When you'll see this:**

- Opening Charon before logging in
- Session expired or cookies cleared
- Browser making auth validation requests

**Why it appears:**

- Charon checks authentication status on page load
- 401 responses are the expected way to indicate "not authenticated"
- The frontend handles this gracefully by showing the login page

**What to do:** Nothing! This is normal application behavior. Once you log in, these errors stop appearing.

**Learn more:** See [Authentication Flow](README.md#authentication-flow) for details on how Charon validates user sessions.

### Development Mode Behavior

**Features that behave differently in development:**

- **Security Headers:** COOP, HSTS disabled on HTTP
- **Cookies:** `Secure` flag not set (allows HTTP cookies)
- **CORS:** More permissive for local testing
- **Logging:** More verbose debugging output

**Production mode automatically enables full security** when accessed over HTTPS.

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

**2. Notifications and Automatic Updates with Dockhand**

- Dockhand is a free service that monitors Docker images for updates and can send notifications or trigger auto-updates. <https://github.com/Finsys/dockhand>

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
