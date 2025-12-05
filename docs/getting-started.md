# Getting Started with Charon

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

**Open http://localhost:8080** in your browser!

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
4. **Click "Save"**

**Done!** When someone visits `myapp.example.com`, they'll see your app.

---

## Step 3: Get HTTPS (The Green Lock)

For this to work, you need:

1. **A real domain name** (like example.com) pointed at your server
2. **Ports 80 and 443 open** in your firewall

If you have both, Charon will automatically:

- Request a free SSL certificate from Let's Encrypt
- Install it
- Renew it before it expires

**You don't do anything.** It just works.

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
- **[Turn On Security](security.md)** — Block attackers (optional but recommended)

---

## Stuck?

**[Ask for help](https://github.com/Wikid82/charon/discussions)** — The community is friendly!
