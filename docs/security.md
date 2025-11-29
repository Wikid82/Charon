# Security Services

Charon includes the optional Cerberus security suite — a collection of high-value integrations (WAF, CrowdSec, ACL, Rate Limiting) designed to protect your services. These features are disabled by default to keep the application lightweight but can be easily enabled via environment variables (CHARON_ preferred; CPM_ still supported).

## Available Services

### 1. CrowdSec (Intrusion Prevention)
[CrowdSec](https://www.crowdsec.net/) is a collaborative security automation tool that analyzes logs to detect and block malicious behavior.

**Modes:**
*   **Local**: Installs the CrowdSec agent *inside* the Charon container. Useful for single-container setups.
    *   *Note*: Increases container startup time and resource usage.
*   **External**: Connects to an existing CrowdSec agent running elsewhere (e.g., on the host or another container).
    *   *Recommended* for production or multi-server setups.

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
| | `external` | Connects to an external CrowdSec agent. |
| `CERBERUS_SECURITY_CROWDSEC_API_URL` | URL | (Required for `external`) e.g., `http://crowdsec:8080` |
| `CERBERUS_SECURITY_CROWDSEC_API_KEY` | String | (Required for `external`) Your CrowdSec bouncer API key. |

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

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CERBERUS_SECURITY_WAF_MODE` | `disabled` | (Default) WAF is turned off. |
| | `enabled` | Enables Coraza WAF with OWASP CRS. |

**Example:**
```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=enabled
```

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
