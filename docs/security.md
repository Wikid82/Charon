# Security Services

CaddyProxyManager+ (CPM+) includes optional, high-value security integrations to protect your services. These features are disabled by default to keep the application lightweight but can be easily enabled via environment variables.

## Available Services

### 1. CrowdSec (Intrusion Prevention)
[CrowdSec](https://www.crowdsec.net/) is a collaborative security automation tool that analyzes logs to detect and block malicious behavior.

**Modes:**
*   **Local**: Installs the CrowdSec agent *inside* the CPM+ container. Useful for single-container setups.
    *   *Note*: Increases container startup time and resource usage.
*   **External**: Connects to an existing CrowdSec agent running elsewhere (e.g., on the host or another container).
    *   *Recommended* for production or multi-server setups.

### 2. WAF (Web Application Firewall)
Uses [Coraza](https://coraza.io/), a Go-native WAF, with the **OWASP Core Rule Set (CRS)** to protect against common web attacks (SQL Injection, XSS, etc.).

### 3. Access Control Lists (ACL)
Allows you to define IP allow/block lists to restrict access to your services.

### 4. Rate Limiting
Protects your services from abuse by limiting the number of requests a client can make within a specific time frame.

---

## Configuration

All security services are controlled via environment variables in your `docker-compose.yml`.

### CrowdSec Configuration

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CPM_SECURITY_CROWDSEC_MODE` | `disabled` | (Default) CrowdSec is turned off. |
| | `local` | Installs and runs CrowdSec agent inside the container. |
| | `external` | Connects to an external CrowdSec agent. |
| `CPM_SECURITY_CROWDSEC_API_URL` | URL | (Required for `external`) e.g., `http://crowdsec:8080` |
| `CPM_SECURITY_CROWDSEC_API_KEY` | String | (Required for `external`) Your CrowdSec bouncer API key. |

**Example (Local Mode):**
```yaml
environment:
  - CPM_SECURITY_CROWDSEC_MODE=local
```

**Example (External Mode):**
```yaml
environment:
  - CPM_SECURITY_CROWDSEC_API_URL=http://192.168.1.50:8080
  - CPM_SECURITY_CROWDSEC_API_KEY=your-bouncer-key-here
```

### WAF Configuration

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CPM_SECURITY_WAF_MODE` | `disabled` | (Default) WAF is turned off. |
| | `enabled` | Enables Coraza WAF with OWASP CRS. |

**Example:**
```yaml
environment:
  - CPM_SECURITY_WAF_MODE=enabled
```

### Rate Limiting & ACLs

| Variable | Value | Description |
| :--- | :--- | :--- |
| `CPM_SECURITY_RATELIMIT_MODE` | `enabled` / `disabled` | Enable global rate limiting. |
| `CPM_SECURITY_ACL_MODE` | `enabled` / `disabled` | Enable IP-based Access Control Lists. |

---

## Dashboard

You can view the status of these services in the CPM+ web interface under the **Security** tab.

*   **CrowdSec**: Shows connection status and mode.
*   **WAF**: Indicates if the Core Rule Set is loaded.
*   **ACLs**: Manage your Block/Allow lists.
*   **Rate Limits**: Configure global request limits.
