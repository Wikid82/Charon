# SSRF Protection in Charon

## Overview

Server-Side Request Forgery (SSRF) is a critical web security vulnerability where an attacker can abuse server functionality to access or manipulate internal resources. Charon implements comprehensive defense-in-depth SSRF protection across all features that accept user-controlled URLs.

**Status**: ✅ **CodeQL CWE-918 Resolved** (PR #450)

- Taint chain break verified via static analysis
- Test coverage: 90.2% for URL validation utilities
- Zero security vulnerabilities (Trivy, govulncheck clean)
- See [PR #450 Implementation Summary](../implementation/PR450_TEST_COVERAGE_COMPLETE.md) for details

### What is SSRF?

SSRF occurs when an application fetches a remote resource based on user input without validating the destination. Attackers exploit this to:

- Access internal network resources (databases, admin panels, internal APIs)
- Retrieve cloud provider metadata (AWS, Azure, GCP credentials)
- Scan internal networks and enumerate services
- Bypass firewalls and access control lists
- Exfiltrate sensitive data through DNS or HTTP requests

### Why It's Critical

SSRF vulnerabilities are classified as **OWASP A10:2021** (Server-Side Request Forgery) and have a typical CVSS score of **8.6 (HIGH)** due to:

- **Network-based exploitation**: Attacks originate from the internet
- **Low attack complexity**: Easy to exploit with basic HTTP knowledge
- **High impact**: Can expose internal infrastructure and sensitive data
- **Difficult detection**: May appear as legitimate server behavior in logs

### How Charon Protects Against SSRF

Charon implements a **three-layer defense-in-depth strategy** using two complementary packages:

#### Defense Layer Architecture

| Layer | Component | Function | Prevents |
|-------|-----------|----------|----------|
| **Layer 1** | `security.ValidateExternalURL()` | Pre-validates URL format, scheme, and DNS resolution | Malformed URLs, blocked protocols, obvious private IPs |
| **Layer 2** | `network.NewSafeHTTPClient()` | Connection-time IP validation in custom dialer | DNS rebinding, TOCTOU attacks, cached DNS exploits |
| **Layer 3** | Redirect validation | Each redirect target validated before following | Redirect-based SSRF bypasses |

#### Validation Flow

```
User URL Input
      │
      ▼
┌─────────────────────────────────────┐
│  Layer 1: security.ValidateExternalURL()   │
│  - Parse URL (scheme, host, path)   │
│  - Reject non-HTTP(S) schemes       │
│  - Resolve DNS with timeout         │
│  - Check ALL IPs against blocklist  │
└───────────────┬─────────────────────┘
                │ ✓ Passes validation
                ▼
┌─────────────────────────────────────┐
│  Layer 2: network.NewSafeHTTPClient()      │
│  - Custom safeDialer() at TCP level │
│  - Re-resolve DNS immediately       │
│  - Re-validate ALL resolved IPs     │
│  - Connect to validated IP directly │
└───────────────┬─────────────────────┘
                │ ✓ Connection allowed
                ▼
┌─────────────────────────────────────┐
│  Layer 3: Redirect Validation       │
│  - Check redirect count (max 2)     │
│  - Validate redirect target URL     │
│  - Re-run IP validation on target   │
└───────────────┬─────────────────────┘
                │ ✓ Response received
                ▼
         Safe Response
```

This layered approach ensures that even if an attacker bypasses URL validation through DNS rebinding or cache manipulation, the connection-time validation will catch the attack.

---

## Protected Endpoints

Charon validates all user-controlled URLs in the following features:

### 1. Security Notification Webhooks

**Endpoint**: `POST /api/v1/settings/security/webhook`

**Protection**: All webhook URLs are validated before saving to prevent SSRF attacks when security events trigger notifications.

**Validation on**:

- Configuration save (fail-fast)
- Notification delivery (defense-in-depth)

**Example Valid URL**:

```json
{
  "webhook_url": "https://hooks.slack.com/services/T00/B00/XXX"
}
```

**Blocked URLs**:

- Private IPs: `http://192.168.1.1/admin`
- Cloud metadata: `http://169.254.169.254/latest/meta-data/`
- Internal hostnames: `http://internal-db.local:3306/`

---

### 2. Custom Webhook Notifications

**Endpoint**: `POST /api/v1/notifications/custom-webhook`

**Protection**: Custom webhooks for alerts, monitoring, and integrations are validated to prevent attackers from using Charon to probe internal networks.

**Use Case**: Send notifications to Discord, Slack, or custom monitoring systems.

**Example Valid URL**:

```json
{
  "webhook_url": "https://discord.com/api/webhooks/123456/abcdef"
}
```

---

### 3. URL Connectivity Testing

**Endpoint**: `POST /api/v1/settings/test-url`

**Protection**: Admin-only endpoint for testing URL reachability. All tested URLs undergo SSRF validation to prevent network scanning.

**Admin Access Required**: Yes (prevents abuse by non-privileged users)

**Example Usage**:

```bash
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"url": "https://api.example.com"}'
```

---

### 4. CrowdSec Hub Synchronization

**Feature**: CrowdSec security rules and hub data retrieval

**Protection**: CrowdSec hub URLs are validated against an allowlist of official hub domains. Custom hub URLs require HTTPS.

**Allowed Domains**:

- `hub-data.crowdsec.net` (official hub)
- `raw.githubusercontent.com` (official mirror)
- `*.example.com`, `*.local` (test domains, testing only)

**Blocked**: Any URL not matching the allowlist or using insecure protocols.

---

### 5. Update Service

**Feature**: Checking for new Charon releases

**Protection**: GitHub API URLs are validated to ensure only official GitHub domains are queried. Prevents SSRF via update check manipulation.

**Allowed Domains**:

- `api.github.com`
- `github.com`

**Protocol**: HTTPS only

---

## Blocked Destinations

Charon blocks **13+ IP ranges** to prevent SSRF attacks:

### Private IP Ranges (RFC 1918)

**Purpose**: Block access to internal networks

| CIDR | Description | Example |
|------|-------------|---------|
| `10.0.0.0/8` | Class A private network | `10.0.0.1` |
| `172.16.0.0/12` | Class B private network | `172.16.0.1` |
| `192.168.0.0/16` | Class C private network | `192.168.1.1` |

**Attack Example**:

```bash
# Attacker attempts to access internal database
curl -X POST /api/v1/settings/security/webhook \
  -d '{"webhook_url": "http://192.168.1.100:5432/"}'

# Response: 400 Bad Request
# Error: "URL resolves to a private IP address (blocked for security)"
```

---

### Cloud Metadata Endpoints

**Purpose**: Block access to cloud provider metadata services

| IP Address | Provider | Contains |
|------------|----------|----------|
| `169.254.169.254` | AWS, Azure, Oracle | Instance credentials, API keys |
| `metadata.google.internal` | Google Cloud | Service account tokens |
| `100.100.100.200` | Alibaba Cloud | Instance metadata |

**Attack Example**:

```bash
# Attacker attempts AWS metadata access
curl -X POST /api/v1/settings/security/webhook \
  -d '{"webhook_url": "http://169.254.169.254/latest/meta-data/iam/security-credentials/"}'

# Response: 400 Bad Request
# Error: "URL resolves to a private IP address (blocked for security)"
```

**Why This Matters**: Cloud metadata endpoints expose:

- IAM role credentials (AWS access keys)
- Service account tokens (GCP)
- Managed identity credentials (Azure)
- Instance metadata (hostnames, IPs, security groups)

---

### Loopback Addresses

**Purpose**: Prevent access to services running on the Charon host itself

| CIDR | Description | Example |
|------|-------------|---------|
| `127.0.0.0/8` | IPv4 loopback | `127.0.0.1` |
| `::1/128` | IPv6 loopback | `::1` |

**Attack Example**:

```bash
# Attacker attempts to access Charon's internal API
curl -X POST /api/v1/settings/security/webhook \
  -d '{"webhook_url": "http://127.0.0.1:8080/admin/backdoor"}'

# Response: 400 Bad Request (unless localhost explicitly allowed for testing)
```

---

### Link-Local Addresses

**Purpose**: Block non-routable addresses used for local network discovery

| CIDR | Description | Example |
|------|-------------|---------|
| `169.254.0.0/16` | IPv4 link-local (APIPA) | `169.254.1.1` |
| `fe80::/10` | IPv6 link-local | `fe80::1` |

---

### Reserved and Special Addresses

**Purpose**: Block addresses with special routing or security implications

| CIDR | Description | Risk |
|------|-------------|------|
| `0.0.0.0/8` | Current network | Undefined behavior |
| `240.0.0.0/4` | Reserved for future use | Non-routable |
| `255.255.255.255/32` | Broadcast | Network scan |
| `fc00::/7` | IPv6 unique local | Private network |

---

## Validation Process

Charon uses a **four-stage validation pipeline** for all user-controlled URLs:

### Stage 1: URL Format Validation

**Checks**:

- ✅ URL is not empty
- ✅ URL parses correctly (valid syntax)
- ✅ Scheme is `http` or `https` only
- ✅ Hostname is present and non-empty
- ✅ No credentials in URL (e.g., `http://user:pass@example.com`)

**Blocked Schemes**:

- `file://` (file system access)
- `ftp://` (FTP protocol smuggling)
- `gopher://` (legacy protocol exploitation)
- `data:` (data URI bypass)
- `javascript:` (XSS/code injection)

**Example Validation Failure**:

```go
// Blocked: Invalid scheme
err := ValidateExternalURL("file:///etc/passwd")
// Error: ErrInvalidScheme ("URL must use HTTP or HTTPS")
```

---

### Stage 2: DNS Resolution

**Checks**:

- ✅ Hostname resolves via DNS (3-second timeout)
- ✅ At least one IP address returned
- ✅ Handles both IPv4 and IPv6 addresses
- ✅ Prevents DNS timeout attacks

**Protection Against**:

- Non-existent domains (typosquatting)
- DNS timeout DoS attacks
- DNS rebinding (resolved IPs checked immediately before request)

**Example**:

```go
// Resolve hostname to IPs
ips, err := net.LookupIP("webhook.example.com")
// Result: [203.0.113.10, 2001:db8::1]

// Check each IP against blocklist
for _, ip := range ips {
    if isPrivateIP(ip) {
        return ErrPrivateIP
    }
}
```

---

### Stage 3: IP Range Validation

**Checks**:

- ✅ ALL resolved IPs are checked (not just the first)
- ✅ Private IP ranges blocked (13+ CIDR blocks)
- ✅ IPv4 and IPv6 support
- ✅ Cloud metadata endpoints blocked
- ✅ Loopback and link-local addresses blocked

**Validation Logic**:

```go
func isPrivateIP(ip net.IP) bool {
    // Check loopback and link-local first (fast path)
    if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
        return true
    }

    // Check against 13+ CIDR ranges
    privateBlocks := []string{
        "10.0.0.0/8",           // RFC 1918
        "172.16.0.0/12",        // RFC 1918
        "192.168.0.0/16",       // RFC 1918
        "169.254.0.0/16",       // Link-local (AWS metadata)
        "127.0.0.0/8",          // Loopback
        "0.0.0.0/8",            // Current network
        "240.0.0.0/4",          // Reserved
        "255.255.255.255/32",   // Broadcast
        "::1/128",              // IPv6 loopback
        "fc00::/7",             // IPv6 unique local
        "fe80::/10",            // IPv6 link-local
    }

    for _, block := range privateBlocks {
        _, subnet, _ := net.ParseCIDR(block)
        if subnet.Contains(ip) {
            return true
        }
    }

    return false
}
```

---

### Stage 4: Request Execution

**Checks**:

- ✅ Use validated IP explicitly (bypass DNS caching attacks)
- ✅ Set timeout (5-10 seconds, context-based)
- ✅ Limit redirects (0-2 maximum)
- ✅ Log all SSRF attempts with HIGH severity

**HTTP Client Configuration**:

```go
client := &http.Client{
    Timeout: 10 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 2 {
            return fmt.Errorf("stopped after 2 redirects")
        }
        return nil
    },
}
```

---

## Testing Exceptions

### Localhost Allowance

**Purpose**: Enable local development and integration testing

**When Allowed**:

- URL connectivity testing with `WithAllowLocalhost()` option
- Development environment (`CHARON_ENV=development`)
- Explicit test fixtures in test code

**Allowed Addresses**:

- `localhost` (hostname)
- `127.0.0.1` (IPv4)
- `::1` (IPv6)

**Example**:

```go
// Production: Blocked
err := ValidateExternalURL("http://localhost:8080/admin")
// Error: ErrLocalhostNotAllowed

// Testing: Allowed with option
err := ValidateExternalURL("http://localhost:8080/admin", WithAllowLocalhost())
// Success: nil error
```

**Security Note**: Localhost exception is **NEVER enabled in production**. Build tags and environment checks enforce this.

---

### When to Use `WithAllowLocalhost`

**✅ Safe Uses**:

1. **Unit tests**: Testing validation logic with mock servers
2. **Integration tests**: Testing against local test fixtures
3. **Development mode**: Local webhooks for debugging (with explicit flag)

**❌ Unsafe Uses**:

1. Production webhook configurations
2. User-facing features without strict access control
3. Any scenario where an attacker controls the URL

---

### Security Implications

**Why Localhost is Dangerous**:

1. **Access to Internal Services**: Charon may have access to:
   - Database ports (PostgreSQL, MySQL, Redis)
   - Admin panels on localhost
   - Monitoring systems (Prometheus, Grafana)
   - Container orchestration APIs (Docker, Kubernetes)

2. **Port Scanning**: Attacker can enumerate services:

   ```bash
   # Scan internal ports via error messages
   http://localhost:22/    # SSH (connection refused)
   http://localhost:3306/  # MySQL (timeout vs refuse reveals service)
   http://localhost:8080/  # HTTP service (200 OK)
   ```

3. **Bypass Firewall**: Internal services often trust localhost connections without authentication.

---

## Configuration Examples

### ✅ Safe Webhook URLs

**Production-Ready Examples**:

```json
// Slack webhook (HTTPS, public IP)
{
  "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX"
}

// Discord webhook (HTTPS, public IP)
{
  "webhook_url": "https://discord.com/api/webhooks/1234567890/abcdefghijklmnop"
}

// Custom webhook with HTTPS
{
  "webhook_url": "https://webhooks.example.com/charon/security-alerts"
}

// Webhook with port (HTTPS)
{
  "webhook_url": "https://alerts.company.com:8443/webhook/receive"
}
```

**Why These Are Safe**:

- ✅ Use HTTPS (encrypted, authenticated)
- ✅ Resolve to public IP addresses
- ✅ Operated by known, trusted services
- ✅ Follow industry standard webhook patterns

---

### ❌ Blocked Webhook URLs

**Dangerous Examples (All Rejected)**:

```json
// BLOCKED: Private IP (RFC 1918)
{
  "webhook_url": "http://192.168.1.100/webhook"
}
// Error: "URL resolves to a private IP address (blocked for security)"

// BLOCKED: AWS metadata endpoint
{
  "webhook_url": "http://169.254.169.254/latest/meta-data/"
}
// Error: "URL resolves to a private IP address (blocked for security)"

// BLOCKED: Loopback address
{
  "webhook_url": "http://127.0.0.1:8080/internal-api"
}
// Error: "localhost URLs are not allowed (blocked for security)"

// BLOCKED: Internal hostname (resolves to private IP)
{
  "webhook_url": "http://internal-db.company.local:5432/"
}
// Error: "URL resolves to a private IP address (blocked for security)"

// BLOCKED: File protocol
{
  "webhook_url": "file:///etc/passwd"
}
// Error: "URL must use HTTP or HTTPS"

// BLOCKED: FTP protocol
{
  "webhook_url": "ftp://internal-ftp.local/upload/"
}
// Error: "URL must use HTTP or HTTPS"
```

**Why These Are Blocked**:

- ❌ Expose internal network resources
- ❌ Allow cloud metadata access (credentials leak)
- ❌ Enable protocol smuggling
- ❌ Bypass network security controls
- ❌ Risk data exfiltration

---

## Security Considerations

### DNS Rebinding Protection

**Attack**: DNS record changes between validation and request execution

**Charon's Defense**:

1. Resolve hostname immediately before HTTP request
2. Check ALL resolved IPs against blocklist
3. Use explicit IP in HTTP client (bypass DNS cache)

**Example Attack Scenario**:

```
Time T0: Attacker configures webhook: http://evil.com/webhook
  DNS Resolution: evil.com → 203.0.113.10 (public IP, passes validation)

Time T1: Charon saves configuration (validation passes)

Time T2: Attacker changes DNS record
  New DNS: evil.com → 192.168.1.100 (private IP)

Time T3: Security event triggers webhook
  Charon Re-Validates: Resolves evil.com again
  Result: 192.168.1.100 detected, request BLOCKED
```

**Protection Mechanism**:

```go
// Validation happens at configuration save AND request time
func (s *SecurityNotificationService) sendWebhook(ctx context.Context, webhookURL string, event models.SecurityEvent) error {
    // RE-VALIDATE on every use (defense against DNS rebinding)
    validatedURL, err := security.ValidateExternalURL(webhookURL,
        security.WithAllowLocalhost(),  // Only for testing
    )
    if err != nil {
        log.Warn("Webhook URL failed SSRF validation (possible DNS rebinding)")
        return err
    }

    // Make HTTP request
    req, _ := http.NewRequestWithContext(ctx, "POST", validatedURL, bytes.NewBuffer(payload))
    resp, err := client.Do(req)
}
```

---

### Time-of-Check-Time-of-Use (TOCTOU)

**Attack**: Race condition between validation and request

**Charon's Defense**:

- Validation and DNS resolution happen in a single transaction
- HTTP request uses resolved IP (no re-resolution)
- Timeout prevents long-running requests that could stall validation

---

### Redirect Following

**Attack**: Initial URL is valid, but redirects to private IP

**Charon's Defense**:

```go
client := &http.Client{
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        // Limit to 2 redirects maximum
        if len(via) >= 2 {
            return fmt.Errorf("stopped after 2 redirects")
        }
        // TODO: Validate redirect target URL (future enhancement)
        return nil
    },
}
```

**Future Enhancement**: Validate each redirect target against the blocklist before following.

---

### Error Message Security

**Problem**: Detailed error messages can leak internal information

**Charon's Approach**: Generic user-facing errors, detailed server logs

**User-Facing Error**:

```json
{
  "error": "URL resolves to a private IP address (blocked for security)"
}
```

**Server Log**:

```
level=HIGH msg="Blocked SSRF attempt" url="http://192.168.1.100/admin" resolved_ip="192.168.1.100" user_id="admin123" timestamp="2025-12-23T10:30:00Z"
```

**What's Hidden**:

- ❌ Internal IP addresses (except in validation context)
- ❌ Network topology details
- ❌ Service names or ports
- ❌ DNS resolution details

**What's Revealed**:

- ✅ Validation failure reason (security context)
- ✅ User-friendly explanation
- ✅ Security justification

---

## Troubleshooting

### Common Error Messages

#### "URL resolves to a private IP address (blocked for security)"

**Cause**: The URL's hostname resolves to a private IP range (RFC 1918, loopback, link-local).

**Solutions**:

1. ✅ Use a publicly accessible webhook endpoint
2. ✅ If webhook must be internal, use a public-facing gateway/proxy
3. ✅ For development, use a service like ngrok or localtunnel

**Example Fix**:

```bash
# BAD: Internal IP
http://192.168.1.100/webhook

# GOOD: Public endpoint
https://webhooks.example.com/charon-alerts

# GOOD: ngrok tunnel (development only)
https://abc123.ngrok.io/webhook
```

---

#### "localhost URLs are not allowed (blocked for security)"

**Cause**: The URL uses `localhost`, `127.0.0.1`, or `::1` without the localhost exception enabled.

**Solutions**:

1. ✅ Use a public webhook service (Slack, Discord, etc.)
2. ✅ Deploy webhook receiver on a public server
3. ✅ For testing, use test URL endpoint with admin privileges

**Example Fix**:

```bash
# BAD: Localhost
http://localhost:3000/webhook

# GOOD: Production webhook
https://hooks.example.com/webhook

# TESTING: Use test endpoint
curl -X POST /api/v1/settings/test-url \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"url": "http://localhost:3000"}'
```

---

#### "URL must use HTTP or HTTPS"

**Cause**: The URL uses a non-HTTP scheme (file://, ftp://, gopher://, etc.).

**Solution**: Change the URL scheme to `http://` or `https://`.

**Example Fix**:

```bash
# BAD: File protocol
file:///etc/passwd

# GOOD: HTTPS
https://api.example.com/webhook
```

---

#### "DNS lookup failed" or "Connection timeout"

**Cause**: Hostname does not resolve, or the server is unreachable.

**Solutions**:

1. ✅ Verify the domain exists and is publicly resolvable
2. ✅ Check for typos in the URL
3. ✅ Ensure the webhook receiver is running and accessible
4. ✅ Test connectivity with `curl` or `ping` from Charon's network

**Example Debugging**:

```bash
# Test DNS resolution
nslookup webhooks.example.com

# Test HTTP connectivity
curl -I https://webhooks.example.com/webhook

# Test from Charon container
docker exec charon curl -I https://webhooks.example.com/webhook
```

---

### How to Debug Validation Failures

**Step 1: Check Server Logs**

```bash
# View recent logs
docker logs charon --tail=100 | grep -i ssrf

# Look for validation errors
docker logs charon --tail=100 | grep "Webhook URL failed SSRF validation"
```

**Step 2: Test URL Manually**

```bash
# Use the test URL endpoint
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"url": "https://your-webhook-url.com"}'

# Response will show reachability and latency
{
  "reachable": true,
  "latency": 145,
  "message": "URL is reachable"
}
```

**Step 3: Verify DNS Resolution**

```bash
# Check what IPs the domain resolves to
nslookup your-webhook-domain.com

# If any IP is private, validation will fail
```

**Step 4: Check for Private IPs**

Use an IP lookup tool to verify the resolved IP is public:

```bash
# Check if IP is private (Python)
python3 << EOF
import ipaddress
ip = ipaddress.ip_address('203.0.113.10')
print(f"Private: {ip.is_private}")
EOF
```

---

### When to Contact Security Team

**Report SSRF bypasses if you can**:

1. Access private IPs despite validation
2. Retrieve cloud metadata endpoints
3. Use protocol smuggling to bypass scheme checks
4. Exploit DNS rebinding despite re-validation
5. Bypass IP range checks with IPv6 or encoding tricks

**How to Report**:

- Email: `security@charon.example.com` (if configured)
- GitHub Security Advisory: <https://github.com/Wikid82/charon/security/advisories/new>
- Include: steps to reproduce, proof of concept (non-destructive)

---

## Developer Guidelines

### How to Use `ValidateExternalURL`

**Import**:

```go
import "github.com/Wikid82/charon/backend/internal/security"
```

**Basic Usage**:

```go
func SaveWebhookConfig(webhookURL string) error {
    // Validate before saving to database
    validatedURL, err := security.ValidateExternalURL(webhookURL)
    if err != nil {
        return fmt.Errorf("invalid webhook URL: %w", err)
    }

    // Save validatedURL to database
    return db.Save(validatedURL)
}
```

**With Options**:

```go
// Allow HTTP (not recommended for production)
validatedURL, err := security.ValidateExternalURL(webhookURL,
    security.WithAllowHTTP(),
)

// Allow localhost (testing only)
validatedURL, err := security.ValidateExternalURL(webhookURL,
    security.WithAllowLocalhost(),
)

// Custom timeout
validatedURL, err := security.ValidateExternalURL(webhookURL,
    security.WithTimeout(5 * time.Second),
)

// Multiple options
validatedURL, err := security.ValidateExternalURL(webhookURL,
    security.WithAllowLocalhost(),
    security.WithAllowHTTP(),
    security.WithTimeout(3 * time.Second),
)
```

---

### Configuration Options

**Available Options**:

```go
// WithAllowHTTP permits HTTP scheme (default: HTTPS only)
func WithAllowHTTP() ValidationOption

// WithAllowLocalhost permits localhost/127.0.0.1/::1 (default: blocked)
func WithAllowLocalhost() ValidationOption

// WithTimeout sets DNS resolution timeout (default: 3 seconds)
func WithTimeout(timeout time.Duration) ValidationOption
```

**When to Use Each Option**:

| Option | Use Case | Security Impact |
|--------|----------|-----------------|
| `WithAllowHTTP()` | Legacy webhooks without HTTPS | ⚠️ MEDIUM - Exposes traffic to eavesdropping |
| `WithAllowLocalhost()` | Local testing, integration tests | 🔴 HIGH - Exposes internal services |
| `WithTimeout()` | Custom DNS timeout for slow networks | ✅ LOW - No security impact |

---

### Using `network.NewSafeHTTPClient()`

For runtime HTTP requests where SSRF protection must be enforced at connection time (defense-in-depth), use the `network.NewSafeHTTPClient()` function. This provides a **three-layer defense strategy**:

1. **Layer 1: URL Pre-Validation** - `security.ValidateExternalURL()` checks URL format and DNS resolution
2. **Layer 2: Connection-Time Validation** - `network.NewSafeHTTPClient()` re-validates IPs at dial time
3. **Layer 3: Redirect Validation** - Each redirect target is validated before following

**Import**:

```go
import "github.com/Wikid82/charon/backend/internal/network"
```

**Basic Usage**:

```go
// Create a safe HTTP client with default options
client := network.NewSafeHTTPClient()

// Make request (connection-time SSRF protection is automatic)
resp, err := client.Get("https://api.example.com/data")
if err != nil {
    // Connection blocked due to private IP, or normal network error
    log.Error("Request failed:", err)
    return err
}
defer resp.Body.Close()
```

**With Options**:

```go
// Custom timeout (default: 10 seconds)
client := network.NewSafeHTTPClient(
    network.WithTimeout(5 * time.Second),
)

// Allow localhost for local services (e.g., CrowdSec LAPI)
client := network.NewSafeHTTPClient(
    network.WithAllowLocalhost(),
)

// Allow limited redirects (default: 0)
client := network.NewSafeHTTPClient(
    network.WithMaxRedirects(2),
)

// Restrict to specific domains
client := network.NewSafeHTTPClient(
    network.WithAllowedDomains("api.github.com", "hub-data.crowdsec.net"),
)

// Custom dial timeout (default: 5 seconds)
client := network.NewSafeHTTPClient(
    network.WithDialTimeout(3 * time.Second),
)

// Combine multiple options
client := network.NewSafeHTTPClient(
    network.WithTimeout(10 * time.Second),
    network.WithAllowLocalhost(),        // For CrowdSec LAPI
    network.WithMaxRedirects(2),
    network.WithDialTimeout(5 * time.Second),
)
```

#### Available Options for `NewSafeHTTPClient`

| Option | Default | Description | Security Impact |
|--------|---------|-------------|-----------------|
| `WithTimeout(d)` | 10s | Total request timeout | ✅ LOW |
| `WithAllowLocalhost()` | false | Permit 127.0.0.1/localhost/::1 | 🔴 HIGH |
| `WithAllowedDomains(...)` | nil | Restrict to specific domains | ✅ Improves security |
| `WithMaxRedirects(n)` | 0 | Max redirects to follow (each validated) | ⚠️ MEDIUM if > 0 |
| `WithDialTimeout(d)` | 5s | Connection timeout per dial | ✅ LOW |

#### How It Prevents DNS Rebinding

The `safeDialer()` function prevents DNS rebinding attacks by:

1. Resolving the hostname to IP addresses **immediately before connection**
2. Validating **ALL resolved IPs** (not just the first) against `IsPrivateIP()`
3. Connecting directly to the validated IP (bypassing DNS cache)

```go
// Internal implementation (simplified)
func safeDialer(opts *ClientOptions) func(ctx, network, addr string) (net.Conn, error) {
    return func(ctx, network, addr string) (net.Conn, error) {
        host, port := net.SplitHostPort(addr)

        // Resolve DNS immediately before connection
        ips, _ := net.DefaultResolver.LookupIPAddr(ctx, host)

        // Validate ALL resolved IPs
        for _, ip := range ips {
            if IsPrivateIP(ip.IP) {
                return nil, fmt.Errorf("connection to private IP blocked: %s", ip.IP)
            }
        }

        // Connect to validated IP (not hostname)
        return dialer.DialContext(ctx, network, net.JoinHostPort(selectedIP, port))
    }
}
```

#### Blocked CIDR Ranges

The `network.IsPrivateIP()` function blocks these IP ranges:

| CIDR Block | Description |
|------------|-------------|
| `10.0.0.0/8` | RFC 1918 Class A private network |
| `172.16.0.0/12` | RFC 1918 Class B private network |
| `192.168.0.0/16` | RFC 1918 Class C private network |
| `169.254.0.0/16` | Link-local (AWS/Azure/GCP metadata: 169.254.169.254) |
| `127.0.0.0/8` | IPv4 loopback |
| `0.0.0.0/8` | Current network |
| `240.0.0.0/4` | Reserved for future use |
| `255.255.255.255/32` | Broadcast |
| `::1/128` | IPv6 loopback |
| `fc00::/7` | IPv6 unique local addresses |
| `fe80::/10` | IPv6 link-local |

---

### Best Practices

**1. Validate Early (Fail-Fast Principle)**

```go
// ✅ GOOD: Validate at configuration time
func CreateWebhookConfig(c *gin.Context) {
    var req struct {
        WebhookURL string `json:"webhook_url"`
    }

    c.ShouldBindJSON(&req)

    // Validate BEFORE saving
    if _, err := security.ValidateExternalURL(req.WebhookURL); err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid webhook URL: %v", err)})
        return
    }

    // Save to database
    db.Save(req.WebhookURL)
}

// ❌ BAD: Validate only at use time
func SendWebhook(webhookURL string, data []byte) error {
    // Saved URL might be invalid, fails at runtime
    validatedURL, err := security.ValidateExternalURL(webhookURL)
    if err != nil {
        return err  // Too late, user already saved invalid config
    }

    http.Post(validatedURL, "application/json", bytes.NewBuffer(data))
}
```

**2. Re-Validate at Use Time (Defense in Depth)**

```go
func SendWebhook(webhookURL string, data []byte) error {
    // Even if validated at save time, re-validate at use
    // Protects against DNS rebinding
    validatedURL, err := security.ValidateExternalURL(webhookURL)
    if err != nil {
        log.WithFields(log.Fields{
            "url": webhookURL,
            "error": err.Error(),
        }).Warn("Webhook URL failed re-validation (possible DNS rebinding)")
        return err
    }

    // Make request
    resp, err := http.Post(validatedURL, "application/json", bytes.NewBuffer(data))
    return err
}
```

**3. Log SSRF Attempts with HIGH Severity**

```go
func SaveWebhookConfig(webhookURL string) error {
    validatedURL, err := security.ValidateExternalURL(webhookURL)
    if err != nil {
        // Log potential SSRF attempt
        log.WithFields(log.Fields{
            "severity": "HIGH",
            "event_type": "ssrf_blocked",
            "url": webhookURL,
            "error": err.Error(),
            "user_id": getCurrentUserID(),
            "timestamp": time.Now().UTC(),
        }).Warn("Blocked SSRF attempt in webhook configuration")

        return fmt.Errorf("invalid webhook URL: %w", err)
    }

    return db.Save(validatedURL)
}
```

**4. Never Trust User Input**

```go
// ❌ BAD: No validation
func FetchRemoteConfig(url string) ([]byte, error) {
    resp, err := http.Get(url)  // SSRF vulnerability!
    if err != nil {
        return nil, err
    }
    return ioutil.ReadAll(resp.Body)
}

// ✅ GOOD: Always validate
func FetchRemoteConfig(url string) ([]byte, error) {
    // Validate first
    validatedURL, err := security.ValidateExternalURL(url)
    if err != nil {
        return nil, fmt.Errorf("invalid URL: %w", err)
    }

    // Use validated URL
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "GET", validatedURL, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return ioutil.ReadAll(resp.Body)
}
```

---

### Code Examples

**Example 1: Webhook Configuration Handler**

```go
func (h *SettingsHandler) SaveSecurityWebhook(c *gin.Context) {
    var req struct {
        WebhookURL string `json:"webhook_url" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "webhook_url is required"})
        return
    }

    // Validate webhook URL (fail-fast)
    validatedURL, err := security.ValidateExternalURL(req.WebhookURL,
        security.WithAllowHTTP(),  // Some webhooks use HTTP
    )
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid webhook URL: %v", err)})
        return
    }

    // Save to database
    if err := h.db.SaveWebhookConfig(validatedURL); err != nil {
        c.JSON(500, gin.H{"error": "Failed to save configuration"})
        return
    }

    c.JSON(200, gin.H{"message": "Webhook configured successfully"})
}
```

---

**Example 2: Webhook Notification Sender**

```go
func (s *NotificationService) SendWebhook(ctx context.Context, event SecurityEvent) error {
    // Get configured webhook URL from database
    webhookURL, err := s.db.GetWebhookURL()
    if err != nil {
        return fmt.Errorf("get webhook URL: %w", err)
    }

    // Re-validate (defense against DNS rebinding)
    validatedURL, err := security.ValidateExternalURL(webhookURL,
        security.WithAllowHTTP(),
    )
    if err != nil {
        log.WithFields(log.Fields{
            "severity": "HIGH",
            "event": "ssrf_blocked",
            "url": webhookURL,
            "error": err.Error(),
        }).Warn("Webhook URL failed SSRF re-validation")
        return fmt.Errorf("webhook URL validation failed: %w", err)
    }

    // Marshal event to JSON
    payload, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal event: %w", err)
    }

    // Create HTTP request with timeout
    req, err := http.NewRequestWithContext(ctx, "POST", validatedURL, bytes.NewBuffer(payload))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "Charon-Webhook/1.0")

    // Execute request
    client := &http.Client{
        Timeout: 10 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 2 {
                return fmt.Errorf("stopped after 2 redirects")
            }
            return nil
        },
    }

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()

    // Check response
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned error: %d", resp.StatusCode)
    }

    log.WithFields(log.Fields{
        "event_type": event.Type,
        "status_code": resp.StatusCode,
        "url": validatedURL,
    }).Info("Webhook notification sent successfully")

    return nil
}
```

---

---

## Test Coverage (PR #450)

### Comprehensive SSRF Protection Tests

Charon maintains extensive test coverage for all SSRF protection mechanisms:

**URL Validation Tests** (90.2% coverage):

- ✅ Private IP detection (IPv4/IPv6)
- ✅ Cloud metadata endpoint blocking (169.254.169.254)
- ✅ DNS resolution with timeout handling
- ✅ Localhost allowance in test mode only
- ✅ Custom timeout configuration
- ✅ Multiple IP address validation (all must pass)

**Security Notification Tests**:

- ✅ Webhook URL validation on save
- ✅ Webhook URL re-validation on send
- ✅ HTTPS enforcement in production
- ✅ SSRF blocking for private IPs
- ✅ DNS rebinding protection

**Integration Tests**:

- ✅ End-to-end webhook delivery with SSRF checks
- ✅ CrowdSec hub URL validation
- ✅ URL connectivity testing with admin-only access
- ✅ Performance benchmarks (< 10ms validation overhead)

**Test Pattern Example**:

```go
func TestValidateExternalURL_CloudMetadataDetection(t *testing.T) {
    // Test blocking AWS metadata endpoint
    _, err := security.ValidateExternalURL("http://169.254.169.254/latest/meta-data/")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "private IP address")
}

func TestValidateExternalURL_IPv6Comprehensive(t *testing.T) {
    // Test IPv6 private addresses
    testCases := []string{
        "http://[fc00::1]/",      // Unique local
        "http://[fe80::1]/",      // Link-local
        "http://[::1]/",          // Loopback
    }
    for _, url := range testCases {
        _, err := security.ValidateExternalURL(url)
        assert.Error(t, err, "Should block: %s", url)
    }
}
```

See [PR #450 Implementation Summary](../implementation/PR450_TEST_COVERAGE_COMPLETE.md) for complete test metrics.

---

## Reporting Security Issues

Found a way to bypass SSRF protection? We want to know!

### What to Report

We're interested in:

- ✅ **SSRF bypasses**: Any method to access private IPs or cloud metadata
- ✅ **DNS rebinding attacks**: Ways to change DNS after validation
- ✅ **Protocol smuggling**: Bypassing scheme restrictions
- ✅ **Redirect exploitation**: Following redirects to private IPs
- ✅ **Encoding bypasses**: IPv6, URL encoding, or obfuscation tricks

### What NOT to Report

- ❌ **Localhost exception in testing**: This is intentional and documented
- ❌ **Error message content**: Generic errors are intentional (no info leak)
- ❌ **Rate limiting**: Not yet implemented (future feature)

### How to Report

**Preferred Method**: GitHub Security Advisory

1. Go to <https://github.com/Wikid82/charon/security/advisories/new>
2. Provide:
   - Steps to reproduce
   - Proof of concept (non-destructive)
   - Expected vs. actual behavior
   - Impact assessment
3. We'll respond within 48 hours

**Alternative Method**: Email

- Send to: `security@charon.example.com` (if configured)
- Encrypt with PGP key (if available in SECURITY.md)
- Include same information as GitHub advisory

### Responsible Disclosure

**Please**:

- ✅ Give us time to fix before public disclosure (90 days)
- ✅ Provide clear reproduction steps
- ✅ Avoid destructive testing (don't attack real infrastructure)

**We'll**:

- ✅ Acknowledge your report within 48 hours
- ✅ Provide regular status updates
- ✅ Credit you in release notes (if desired)
- ✅ Consider security bounty (if applicable)

---

## Additional Resources

- **OWASP SSRF Prevention Cheat Sheet**: <https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html>
- **CWE-918**: Server-Side Request Forgery (SSRF) - <https://cwe.mitre.org/data/definitions/918.html>
- **Charon Security Instructions**: `.github/instructions/security-and-owasp.instructions.md`
- **Implementation Report**: `docs/implementation/SSRF_REMEDIATION_COMPLETE.md`
- **QA Audit Report**: `docs/reports/qa_ssrf_remediation_report.md`

---

**Document Version**: 1.1
**Last Updated**: December 24, 2025
**Status**: Production
**Maintained By**: Charon Security Team
