# Security Features

Charon includes **Cerberus**, a security system that protects your websites. It's **turned off by default** so it doesn't get in your way while you're learning.

When you're ready to turn it on, this guide explains everything.

---

## What Is Cerberus?

Think of Cerberus as a guard dog for your websites. It has three heads (in Greek mythology), and each head watches for different threats:

1. **CrowdSec** — Blocks bad IP addresses
2. **WAF (Web Application Firewall)** — Blocks bad requests
3. **Access Lists** — You decide who gets in

---

## Turn It On (The Safe Way)

**Step 1: Start in "Monitor" Mode**

This means Cerberus watches but doesn't block anyone yet.

Add this to your `docker-compose.yml`:

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=monitor
  - CERBERUS_SECURITY_CROWDSEC_MODE=local
```

Restart Charon:

```bash
docker-compose restart
```

**Step 2: Watch the Logs**

Check "Security" in the sidebar. You'll see what would have been blocked. If it looks right, move to Step 3.

**Step 3: Turn On Blocking**

Change `monitor` to `block`:

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=block
```

Restart again. Now bad guys actually get blocked.

---

## CrowdSec (Block Bad IPs)

**What it does:** Thousands of people share information about attackers. When someone tries to hack one of them, everyone else blocks that attacker too.

**Why you care:** If someone is attacking servers in France, you block them before they even get to your server in California.

### How to Enable It

**Local Mode** (Runs inside Charon):

```yaml
environment:
  - CERBERUS_SECURITY_CROWDSEC_MODE=local
```

That's it. CrowdSec starts automatically and begins blocking bad IPs.

**What you'll see:** The "Security" page shows blocked IPs and why they were blocked.

---

## WAF (Block Bad Behavior)

**What it does:** Looks at every request and checks if it's trying to do something nasty—like inject SQL code or run JavaScript attacks.

**Why you care:** Even if your app has a bug, the WAF might catch the attack first.

### How to Enable It

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=block
```

**Start with `monitor` first!** This lets you see what would be blocked without actually blocking it.

---

## Access Lists (You Decide Who Gets In)

Access lists let you block or allow specific countries, IP addresses, or networks.

### Example 1: Block a Country

**Scenario:** You only need access from the US, so block everyone else.

1. Go to **Access Lists**
2. Click **Add List**
3. Name it "US Only"
4. **Type:** Geo Whitelist
5. **Countries:** United States
6. **Assign to your proxy host**

Now only US visitors can access that website. Everyone else sees "Access Denied."

### Example 2: Private Network Only

**Scenario:** Your admin panel should only work from your home network.

1. Create an access list
2. **Type:** Local Network Only
3. Assign it to your admin panel proxy

Now only devices on `192.168.x.x` or `10.x.x.x` can access it. The public internet can't.

### Example 3: Block One Country

**Scenario:** You're getting attacked from one specific country.

1. Create a list
2. **Type:** Geo Blacklist
3. Pick the country
4. Assign to the targeted website

---

## Don't Lock Yourself Out!

**Problem:** If you turn on security and misconfigure it, you might block yourself.

**Solution:** Add your IP to the "Admin Whitelist" first.

### How to Add Your IP

1. Go to **Settings → Security**
2. Find "Admin Whitelist"
3. Add your IP address (find it at [ifconfig.me](https://ifconfig.me))
4. Save

Now you can never accidentally block yourself.

### Break-Glass Token (Emergency Exit)

If you do lock yourself out:

1. Log into your server directly (SSH)
2. Run this command:

```bash
docker exec charon charon break-glass
```

It generates a one-time token that lets you disable security and get back in.

---

## Recommended Settings by Service Type

### Internal Admin Panels (Router, Pi-hole, etc.)

```
Access List: Local Network Only
```

Blocks all public internet traffic.

### Personal Blog or Portfolio

```
No access list
WAF: Enabled
CrowdSec: Enabled
```

Keep it open for visitors, but protect against attacks.

### Password Manager (Vaultwarden, etc.)

```
Access List: IP Whitelist (your home IP)
Or: Geo Whitelist (your country only)
```

Most restrictive. Only you can access it.

### Media Server (Plex, Jellyfin)

```
Access List: Geo Blacklist (high-risk countries)
CrowdSec: Enabled
```

Allows friends to access, blocks obvious threat countries.

---

## Check If It's Working

1. Go to **Security → Decisions** in the sidebar
2. You'll see a list of recent blocks
3. If you see activity, it's working!

---

## Turn It Off

If security is causing problems:

**Option 1: Via Web UI**

1. Go to **Settings → Security**
2. Toggle "Enable Cerberus" off

**Option 2: Via Environment Variable**

Remove the security lines from `docker-compose.yml` and restart.

---

## Common Questions

### "Will this slow down my websites?"

No. The checks happen in milliseconds. Humans won't notice.

### "Can I whitelist specific paths?"

Not yet, but it's planned. For now, access lists apply to entire websites.

### "What if CrowdSec blocks a legitimate visitor?"

You can manually unblock IPs in the Security → Decisions page.

### "Do I need all three security features?"

No. Use what you need:

- **Just starting?** CrowdSec only
- **Public service?** CrowdSec + WAF
- **Private service?** Access Lists only

---

## More Technical Details

Want the nitty-gritty? See [Cerberus Technical Docs](cerberus.md).
