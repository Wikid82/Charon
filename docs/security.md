# Security Services

Charon includes the optional Cerberus security suite — a collection of high-value integrations (WAF, CrowdSec, ACL, Rate Limiting) designed to protect your services. These features are disabled by default to keep the application lightweight but can be easily enabled via environment variables (CHARON_ preferred; CPM_ still supported).

## Available Services

### 1. CrowdSec (Intrusion Prevention)
[CrowdSec](https://www.crowdsec.net/) is a collaborative security automation tool that analyzes logs to detect and block malicious behavior.

**Modes:**
*   **Local**: Installs the CrowdSec agent *inside* the Charon container. Useful for single-container setups.
    *   *Note*: Increases container startup time and resource usage.
*   **External**: (Deprecated) connections to external CrowdSec agents are no longer supported.

### 2. WAF (Web Application Firewall)
Uses [Coraza](https://coraza.io/), a Go-native WAF, with the **OWASP Core Rule Set (CRS)** to protect against common web attacks (SQL Injection, XSS, etc.).

### 3. Access Control Lists (ACL)
Restrict access to your services based on IP addresses, CIDR ranges, or geographic location using MaxMind GeoIP2.

**Features:**
- **IP Whitelist**: Allow only specific IPs/ranges (blocks all others)
- **IP Blacklist**: Block specific IPs/ranges (allows all others)
- **Geo Whitelist**: Allow only specific countries (blocks all others)
- **Geo Blacklist**: Block specific countries (allows all others)
- **Local Network Only**: Restrict to RFC1918 private networks (10.x, 192.168.x, 172.16-31.x)

Each ACL can be assigned to individual proxy hosts, allowing per-service access control.

### 4. Rate Limiting
Protects your services from abuse by limiting the number of requests a client can make within a specific time frame.

---

## Configuration

All security services are controlled via environment variables in your `docker-compose.yml`.

### Enable Cerberus (Runtime Toggle)

You can enable or disable Cerberus at runtime via the web UI `System Settings` or by setting the `security.cerberus.enabled` setting. This allows you to control the suite without restarting the service when using the UI.


### CrowdSec Configuration

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CERBERUS_SECURITY_CROWDSEC_MODE` | `disabled` | (Default) CrowdSec is turned off. (CERBERUS_ preferred; CHARON_/CPM_ still supported) |
| | `local` | Installs and runs CrowdSec agent inside the container. |
| | `local` | Installs and runs CrowdSec agent inside the container. |

**Example (Local Mode):**
```yaml
environment:
  - CERBERUS_SECURITY_CROWDSEC_MODE=local # CERBERUS_ preferred; CHARON_/CPM_ still supported
```

**Example (External Mode):**
```yaml
 environment:
  - CERBERUS_SECURITY_CROWDSEC_MODE=external
  - CERBERUS_SECURITY_CROWDSEC_API_URL=http://192.168.1.50:8080
  - CERBERUS_SECURITY_CROWDSEC_API_KEY=your-bouncer-key-here
```

### WAF Configuration

| Variable | Values | Description |
| :--- | :--- | :--- |
| `CERBERUS_SECURITY_WAF_MODE` | `disabled` | (Default) WAF is turned off. |
|  | `monitor` | Evaluate requests, emit metrics & structured logs, do not block. |
|  | `block` | Evaluate & actively block suspicious payloads. |

**Example (Monitor Mode):**
```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=monitor
```

**Example (Blocking Mode):**
```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=block
```

> Migration Note: Earlier documentation referenced a value `enabled`. Use `block` going forward for enforcement.

### ACL Configuration

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CERBERUS_SECURITY_ACL_MODE` | `disabled` | (Default) ACLs are turned off. |
| | `enabled` | Enables IP and geo-blocking ACLs. |
| `CHARON_GEOIP_DB_PATH`/`CPM_GEOIP_DB_PATH` | Path | Path to MaxMind GeoLite2-Country.mmdb (auto-configured in Docker) (CHARON_ preferred; CPM_ still supported) |

**Example:**
```yaml
environment:
  - CERBERUS_SECURITY_ACL_MODE=enabled
```

### Rate Limiting Configuration

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CERBERUS_SECURITY_RATELIMIT_MODE` | `enabled` / `disabled` | Enable global rate limiting. |

---

## Self-Lockout Protection

When enabling the Cerberus suite (CrowdSec, WAF, ACLs, Rate Limiting) there is a risk of accidentally locking yourself out of the Admin UI or services you rely on. Charon provides the following safeguards to reduce this risk:

- **Admin Whitelist**: When enabling Cerberus you should enter at least one administrative IP or CIDR range (for example your VPN IP, Tailscale IP, or a trusted office IP). This whitelist is always excluded from blocking decisions.
- **Break-Glass Token**: You can generate a temporary break-glass token from the Security UI. This one-time token (returned plaintext once) can be used to disable Cerberus if you lose access.
- **Localhost Bypass**: Requests from `127.0.0.1` or `::1` may be allowed to manage the system locally without a token (helpful for local management access).
- **Manager Checks**: Config deployment will be refused if Cerberus is enabled and no admin whitelist is configured — this prevents accidental global lockouts when applying new configurations.

Follow a phased approach: deploy in `monitor` (log-only) first, validate findings, add admin whitelist entries, then switch to `block` enforcement.

## ACL Best Practices by Service Type

### Internal Services (Pi-hole, Home Assistant, Router Admin)
**Recommended**: **Local Network Only** ACL
- Blocks all public internet access
- Only allows RFC1918 private IPs (10.x, 192.168.x, 172.16-31.x)
- Perfect for: Pi-hole, Unifi Controller, Home Assistant, Proxmox, Router interfaces

### Media Servers (Plex, Jellyfin, Emby)
**Recommended**: **Geo Blacklist** for high-risk countries
- Block countries known for scraping/piracy monitoring (e.g., China, Russia, Iran)
- Allows legitimate users worldwide while reducing abuse
- Example countries to block: CN, RU, IR, KP, BY

### Personal Cloud Storage (Nextcloud, Syncthing)
**Recommended**: **Geo Whitelist** to your country/region
- Only allow access from countries where you actually travel
- Example: US, CA, GB, FR, DE (if you're North American/European)
- Dramatically reduces attack surface

### Public-Facing Services (Blogs, Portfolio Sites)
**Recommended**: **No ACL** or **Blacklist** only
- Keep publicly accessible for SEO and visitors
- Use blacklist only if experiencing targeted attacks
- Rely on WAF + CrowdSec for protection instead

### Password Managers (Vaultwarden, Bitwarden)
**Recommended**: **IP Whitelist** or **Geo Whitelist**
- Whitelist your home IP, VPN endpoint, or mobile carrier IPs
- Or geo-whitelist your home country only
- Most restrictive option for highest-value targets

### Business/Work Services (GitLab, Wiki, Internal Apps)
**Recommended**: **IP Whitelist** for office/VPN
- Whitelist office IP ranges and VPN server IPs
- Blocks all other access, even from same country
- Example: 203.0.113.0/24 (office), 198.51.100.50 (VPN)

---

## Multi-Layer Protection & When to Use ACLs

Charon follows a multi-layered security approach. The recommendation below shows which module is best suited for specific types of threats:

- **CrowdSec**: Best for dynamic, behavior-driven blocking — bots, scanners, credential stuffing, IP reputation. CrowdSec integrates with local or external agents and should be used for most bot and scanner detection/remediation.
- **WAF (Coraza)**: Best for payload and application-level attacks (XSS, SQLi, file inclusion). Protects against malicious payloads regardless of source IP.

### Coraza runtime integration test

To validate runtime Coraza WAF integration locally using Docker Compose:

1. Build the local Docker image and start services: `docker build -t charon:local . && docker compose -f docker-compose.local.yml up -d`.
2. Configure a ruleset via the API: POST to `/api/v1/security/rulesets` with a rule that would match an XSS payload.
3. Send a request that triggers the rule (e.g., POST with `<script>` payload) and verify `403` or similar WAF-blocking response.

There is a lightweight helper script `scripts/coraza_integration.sh` which performs these steps and can be used as a starting point for CI integration tests.
- **Rate Limiting**: Best for high-volume scanners and brute-force attempts; helps prevent abuse from cloud providers and scrapers.
- **ACLs (Geo/Page-Level)**: Best for static location-based or private network restrictions, e.g., geo-blocking or restricting access to RFC1918 ranges for internal services.

Because IP-based blocklists are dynamic and often incomplete, we removed the IP-based Access List presets (e.g., botnet, scanner, VPN lists) from the default UI presets. These dynamic IP blocklists are now the recommended responsibility of CrowdSec and rate limiting; they are easier to maintain, update, and automatically mitigate at scale.

Use ACLs primarily for explicit or static restrictions such as geofencing or limiting access to your home/office IP ranges.

---

## Observability & Logging

Charon exposes security observability through Prometheus metrics and structured logs:

### Prometheus Metrics
| Metric | Description |
| :--- | :--- |
| `charon_waf_requests_total` | Total requests evaluated by the WAF. |
| `charon_waf_blocked_total` | Requests blocked in `block` mode. |
| `charon_waf_monitored_total` | Requests logged in `monitor` mode. |

Scrape endpoint: `GET /metrics` (no auth). Integrate with Prometheus server or a compatible collector.

### Structured Logs
WAF decisions emit JSON-like structured fields:
```
source: "waf"
decision: "block" | "monitor"
mode: "block" | "monitor" | "disabled"
path: "/api/v1/..."
query: "raw url query string"
```
Use these fields to build dashboards and alerting (e.g., block rate spikes).

### Recommended Dashboards
- Block Rate (% blocked / evaluated)
- Monitor to Block Transition (verify stability before enforcing)
- Top Paths Triggering Blocks
- Recent Security Decisions (from `/api/v1/security/decisions`)

---

## Security API Summary

| Endpoint | Method | Purpose |
| :--- | :--- | :--- |
| `/api/v1/security/status` | GET | Current enabled state & modes. |
| `/api/v1/security/config` | GET | Retrieve persisted global security config. |
| `/api/v1/security/config` | POST | Upsert global security config. |
| `/api/v1/security/enable` | POST | Enable Cerberus (requires whitelist or break-glass token). |
| `/api/v1/security/disable` | POST | Disable Cerberus (localhost or break-glass token). |
| `/api/v1/security/breakglass/generate` | POST | Generate one-time break-glass token. |
| `/api/v1/security/decisions` | GET | List recent decisions (limit query param). |
| `/api/v1/security/decisions` | POST | Manually log a decision (override). |
| `/api/v1/security/rulesets` | GET | List uploaded rulesets. |
| `/api/v1/security/rulesets` | POST | Create/update a ruleset. |
| `/api/v1/security/rulesets/:id` | DELETE | Remove a ruleset. |

### Sample Security Config Payload
```json
{
  "name": "default",
  "enabled": true,
  "admin_whitelist": "198.51.100.10,203.0.113.0/24",
  "crowdsec_mode": "local",
  "crowdsec_api_url": "",
  "waf_mode": "monitor",
  "waf_rules_source": "owasp-crs-local",
  "waf_learning": true,
  "rate_limit_enable": false,
  "rate_limit_burst": 0,
  "rate_limit_requests": 0,
  "rate_limit_window_sec": 0
}
```

### Sample Ruleset Upsert Payload
```json
{
  "name": "owasp-crs-quick",
  "source_url": "https://example.com/owasp-crs.txt",
  "mode": "owasp",
  "content": "# raw rules or placeholder"
}
```

---

## Testing ACLs

Before applying an ACL to a production service:

1. Create the ACL in the web UI
2. Leave it **Disabled** initially
3. Use the **Test IP** button to verify your own IP would be allowed
4. Assign to a non-critical service first
5. Test access from both allowed and blocked locations
6. Enable on production services once validated

**Tip**: Always test with your own IP first! Use sites like `ifconfig.me` or `ipinfo.io/ip` to find your current public IP.

---

## Dashboard

You can view the status of these services in the Charon web interface under the **Security** tab.

*   **CrowdSec**: Shows connection status and mode.
*   **WAF**: Indicates if the Core Rule Set is loaded.
*   **ACLs**: Manage your Block/Allow lists.
*   **Rate Limits**: Configure global request limits.
