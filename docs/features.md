# What Can Charon Do?

Here's everything Charon can do for you, explained simply.

---

## \ud83d\udd10 SSL Certificates (The Green Lock)

**What it does:** Makes browsers show a green lock next to your website address.

**Why you care:** Without it, browsers scream "NOT SECURE!" and people won't trust your site.

**What you do:** Nothing. Charon gets free certificates from Let's Encrypt and renews them automatically.

---

## \ud83d\udee1\ufe0f Security (Optional)

Charon includes **Cerberus**, a security system that blocks bad guys. It's off by default—turn it on when you're ready.

### Block Bad IPs Automatically

**What it does:** CrowdSec watches for attackers and blocks them before they can do damage.

**Why you care:** Someone tries to guess your password 100 times? Blocked automatically.

**What you do:** Add one line to your docker-compose file. See [Security Guide](security.md).

### Block Entire Countries

**What it does:** Stop all traffic from specific countries.

**Why you care:** If you only need access from the US, block everywhere else.

**What you do:** Create an access list, pick countries, assign it to your website.

### Block Bad Behavior

**What it does:** Detects common attacks like SQL injection or XSS.

**Why you care:** Protects your apps even if they have bugs.

**What you do:** Turn on "WAF" mode in security settings.
### Zero-Day Exploit Protection

**What it does:** The WAF (Web Application Firewall) can detect and block many zero-day exploits before they reach your apps.

**Why you care:** Even if a brand-new vulnerability is discovered in your software, the WAF might catch it by recognizing the attack pattern.

**How it works:**
- Attackers use predictable patterns (SQL syntax, JavaScript tags, command injection)
- The WAF inspects every request for these patterns
- If detected, the request is blocked or logged (depending on mode)

**What you do:**
1. Enable WAF in "Monitor" mode first (logs only, doesn't block)
2. Review logs for false positives
3. Switch to "Block" mode when ready

**Limitations:**
- Only protects web-based exploits (HTTP/HTTPS traffic)
- Does NOT protect against zero-days in Docker, Linux, or Charon itself
- Does NOT replace regular security updates

**Learn more:** [OWASP Core Rule Set](https://coreruleset.org/)
---

## \ud83d\udc33 Docker Integration

### Auto-Discover Containers

**What it does:** Sees all your Docker containers and shows them in a list.

**Why you care:** Instead of typing IP addresses, just click your container and Charon fills everything in.

**What you do:** Make sure Charon can access `/var/run/docker.sock` (it's in the quick start).

### Remote Docker Servers

**What it does:** Manages containers on other computers.

**Why you care:** Run Charon on one server, manage containers on five others.

**What you do:** Add remote servers in the "Docker" section.

---

## \ud83d\udce5 Import Your Old Setup

**What it does:** Reads your existing Caddyfile and creates proxy hosts for you.

**Why you care:** Don't start from scratch if you already have working configs.

**What you do:** Click "Import," paste your Caddyfile, review the results, click "Import."

**[Detailed Import Guide](import-guide.md)**

---

## \u26a1 Zero Downtime Updates

**What it does:** Apply changes without stopping traffic.

**Why you care:** Your websites stay up even while you're making changes.

**What you do:** Nothing special—every change is zero-downtime by default.

---

## \ud83c\udfa8 Beautiful Loading Animations

When you make changes, Charon shows you themed animations so you know what's happening.

### The Gold Coin (Login)

When you log in, you see a spinning gold coin. In Greek mythology, people paid Charon the ferryman with a coin to cross the river into the afterlife. So logging in = paying for passage!

### The Blue Boat (Managing Websites)

When you create or update websites, you see Charon's boat sailing across the river. He's literally "ferrying" your changes to the server.

### The Red Guardian (Security)

When you change security settings, you see Cerberus—the three-headed guard dog. He protects the gates of the underworld, just like your security settings protect your apps.

**Why these exist:** Changes can take 1-10 seconds to apply. The animations tell you what's happening so you don't think it's broken.

---

## \ud83d\udd0d Health Checks

**What it does:** Tests if your app is actually reachable before saving.

**Why you care:** Catches typos and mistakes before they break things.

**What you do:** Click the "Test" button when adding a website.

---

## \ud83d\udccb Logs & Monitoring

**What it does:** Shows you what's happening with your proxy.

**Why you care:** When something breaks, you can see exactly what went wrong.

**What you do:** Click "Logs" in the sidebar.

---

## \ud83d\udcbe Backup & Restore

**What it does:** Saves a copy of your configuration before destructive changes.

**Why you care:** If you accidentally delete something, restore it with one click.

**What you do:** Backups happen automatically. Restore from the "Backups" page.

---

## \ud83c\udf10 WebSocket Support

**What it does:** Handles real-time connections for chat apps, live updates, etc.

**Why you care:** Apps like Discord bots, live dashboards, and chat servers need this to work.

**What you do:** Nothing—WebSockets work automatically.

---

## \ud83d\udcca Uptime Monitoring (Coming Soon)

**What it does:** Checks if your websites are responding.

**Why you care:** Get notified when something goes down.

**Status:** Coming in a future update.

---

## \ud83d\udcf1 Mobile-Friendly Interface

**What it does:** Works perfectly on phones and tablets.

**Why you care:** Fix problems from anywhere, even if you're not at your desk.

**What you do:** Just open the web interface on your phone.

---

## \ud83c\udf19 Dark Mode

**What it does:** Easy-on-the-eyes dark interface.

**Why you care:** Late-night troubleshooting doesn't burn your retinas.

**What you do:** It's always dark mode. (Light mode coming if people ask for it.)

---

## \ud83d\udd0c API for Automation

**What it does:** Control everything via code instead of the web interface.

**Why you care:** Automate repetitive tasks or integrate with other tools.

**What you do:** See the [API Documentation](api.md).

---

## Missing Something?

**[Request a feature](https://github.com/Wikid82/charon/discussions)** — Tell us what you need!
