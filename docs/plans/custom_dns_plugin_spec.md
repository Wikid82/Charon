# Custom DNS Provider Plugin Support - Feature Specification

**Status:** 📋 Planning (Revised)
**Priority:** P2 (Medium)
**Estimated Time:** 48-68 hours
**Author:** Planning Agent
**Date:** January 8, 2026
**Last Revised:** January 11, 2026
**Related:** [Phase 5 Custom Plugins Spec](phase5_custom_plugins_spec.md)

---

## 1. Executive Summary

### Problem Statement

Charon currently supports 10 built-in DNS providers for ACME DNS-01 challenges:

- Cloudflare, Route53, DigitalOcean, Hetzner, DNSimple, Vultr, GoDaddy, Namecheap, Google Cloud DNS, Azure

Users with DNS services not on this list cannot obtain wildcard certificates or use DNS-01 challenges. This limitation affects:

- Organizations using self-hosted DNS (BIND, PowerDNS, Knot DNS)
- Users of regional/niche DNS providers
- Enterprise environments with custom DNS APIs
- Air-gapped or on-premise deployments

### Proposed Solution

Implement multiple extensibility mechanisms that balance ease-of-use with flexibility:

| Option | Target User | Complexity | Automation Level |
|--------|-------------|------------|------------------|
| **A: Webhook Plugin** | DevOps, Integration teams | Medium | Full |
| **B: Script Plugin** | Sysadmins, Power users | Low-Medium | Full |
| **C: RFC 2136 Plugin** | Self-hosted DNS admins | Medium | Full |
| **D: Manual Plugin** | One-off certs, Testing | None | Manual |

### Success Criteria

- Users can obtain certificates using any DNS provider
- At least one plugin option is production-ready within 2 weeks
- Existing built-in providers continue to work unchanged
- 85% test coverage maintained

---

## 2. User Stories

### 2.1 Webhook Plugin (Option A)

> **As a DevOps engineer** with a custom DNS API, I want to provide webhook endpoints so Charon can automate DNS challenges without building a custom integration.

**Acceptance Criteria:**

- I can configure URLs for create/delete TXT record operations
- Charon sends JSON payloads with record details
- I can set custom headers for authentication
- Retry logic handles temporary failures

### 2.2 Script Plugin (Option B)

> **As a system administrator**, I want to run a shell script when Charon needs to create/delete TXT records so I can use my existing DNS automation tools.

**Acceptance Criteria:**

- I can specify a script path inside the container
- Script receives ACTION, DOMAIN, TOKEN, VALUE as arguments
- Script exit code determines success/failure
- Timeout prevents hung scripts

### 2.3 RFC 2136 Plugin (Option C)

> **As a network engineer** running BIND or PowerDNS, I want to use RFC 2136 Dynamic DNS Updates so Charon integrates with my existing infrastructure.

**Acceptance Criteria:**

- I can configure DNS server address and TSIG key
- Charon sends standards-compliant UPDATE messages
- Zone detection works automatically
- Works with BIND9, PowerDNS, Knot DNS

### 2.4 Manual Plugin (Option D)

> **As a user** with an unsupported provider, I want Charon to show me the required TXT record details so I can create it manually.

**Acceptance Criteria:**

- UI clearly displays the record name and value
- I can copy values with one click
- "Verify" button checks if record exists
- Progress indicator shows timeout countdown

### 2.5 General Stories

> **As an administrator**, I want to see all available DNS provider types (built-in + custom) in a unified list.

> **As a security officer**, I want custom plugin configurations to be validated and logged for audit purposes.

---

## 3. Architecture Analysis

### 3.1 Current Plugin System

Charon already has a well-designed plugin architecture in `backend/pkg/dnsprovider/`:

```
backend/pkg/dnsprovider/
├── plugin.go          # ProviderPlugin interface (13 methods)
├── registry.go        # Thread-safe registry (Global singleton)
├── errors.go          # Custom error types
└── builtin/
    ├── init.go        # Auto-registers 10 built-in providers
    ├── cloudflare.go  # Example: implements ProviderPlugin
    ├── route53.go
    └── ... (8 more providers)
```

**Key Interface Methods:**

```go
type ProviderPlugin interface {
    Type() string
    Metadata() ProviderMetadata
    Init() error
    Cleanup() error
    RequiredCredentialFields() []CredentialFieldSpec
    OptionalCredentialFields() []CredentialFieldSpec
    ValidateCredentials(creds map[string]string) error
    TestCredentials(creds map[string]string) error
    SupportsMultiCredential() bool
    BuildCaddyConfig(creds map[string]string) map[string]any
    BuildCaddyConfigForZone(baseDomain string, creds map[string]string) map[string]any
    PropagationTimeout() time.Duration
    PollingInterval() time.Duration
}
```

### 3.2 How Custom Plugins Integrate

The existing architecture supports custom plugins via the registry pattern:

```
┌────────────────────────────────────────────────────────────────────┐
│                        DNS Provider Registry                        │
│ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────────────┐│
│ │ Cloudflare │ │  Route53   │ │ ... (8)    │ │  Custom Plugins    ││
│ │ (built-in) │ │ (built-in) │ │ (built-in) │ │ ┌────────────────┐ ││
│ └────────────┘ └────────────┘ └────────────┘ │ │ Webhook Plugin │ ││
│                                               │ ├────────────────┤ ││
│                                               │ │ Script Plugin  │ ││
│                                               │ ├────────────────┤ ││
│                                               │ │ RFC2136 Plugin │ ││
│                                               │ ├────────────────┤ ││
│                                               │ │ Manual Plugin  │ ││
│                                               │ └────────────────┘ ││
│                                               └────────────────────┘│
└────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
          ┌─────────────────┐             ┌─────────────────┐
          │ DNS Provider    │             │ Caddy Config    │
          │ Service Layer   │             │ Builder         │
          │ (CRUD + Test)   │             │ (TLS Automation)│
          └─────────────────┘             └─────────────────┘
```

### 3.3 Caddy DNS Challenge Integration

Caddy's TLS automation supports custom DNS providers via its module system. For Options A, B, C, we need to either:

1. **Use Caddy's `exec` DNS provider** - Caddy calls an external command
2. **Build a custom Caddy module** - Complex, requires Caddy rebuild
3. **Use Charon as a DNS proxy** - Charon handles DNS operations, returns status to Caddy

**Recommended Approach:** Option 3 (Charon as DNS proxy) for Webhook/Script plugins, native Caddy module for RFC 2136.

#### 3.3.1 Charon DNS Proxy Architecture

For Webhook and Script plugins, Charon acts as a DNS challenge proxy between Caddy and the external DNS provider:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    DNS Challenge Flow (Webhook/Script)                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────┐  1. Certificate    ┌──────────┐  2. DNS-01 Challenge          │
│  │  Caddy   │  ──────────────▶  │   ACME   │  ◀─────────────────────        │
│  │  (TLS)   │                    │  Server  │                                │
│  └────┬─────┘                    └──────────┘                                │
│       │                                                                      │
│       │ 3. Create TXT record                                                 │
│       │    (via exec module or                                               │
│       │     internal API)                                                    │
│       ▼                                                                      │
│  ┌──────────┐  4. POST /internal/dns-challenge                               │
│  │  Charon  │  ─────────────────────────────────────────────────────────     │
│  │  (Proxy) │                                                                │
│  └────┬─────┘                                                                │
│       │                                                                      │
│       │ 5. Execute plugin (webhook/script)                                   │
│       ▼                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    External DNS Provider                              │   │
│  │  (Webhook endpoint or DNS server via script)                          │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 3.3.2 Challenge Lifecycle State Machine

```
                              ┌─────────────┐
                              │   CREATED   │
                              │  (initial)  │
                              └──────┬──────┘
                                     │
                          Plugin executes create
                                     │
                                     ▼
                              ┌─────────────┐
        ┌─────────────────────│   PENDING   │─────────────────────┐
        │                     │ (awaiting   │                     │
        │                     │ propagation)│                     │
        │                     └──────┬──────┘                     │
        │                            │                            │
   Timeout (10 min)          DNS check passes              Plugin error
        │                            │                            │
        ▼                            ▼                            ▼
 ┌─────────────┐            ┌─────────────┐              ┌─────────────┐
 │   EXPIRED   │            │  VERIFYING  │              │   FAILED    │
 │             │            │             │              │             │
 └─────────────┘            └──────┬──────┘              └─────────────┘
                                   │
                       ┌───────────┴───────────┐
                       │                       │
                  ACME success            ACME failure
                       │                       │
                       ▼                       ▼
                ┌─────────────┐         ┌─────────────┐
                │  VERIFIED   │         │   FAILED    │
                │  (success)  │         │             │
                └─────────────┘         └─────────────┘
```

**State Definitions:**

| State | Description | Next States | TTL |
|-------|-------------|-------------|-----|
| `CREATED` | Challenge record created, plugin not yet executed | PENDING, FAILED | - |
| `PENDING` | Plugin executed, waiting for DNS propagation | VERIFYING, EXPIRED, FAILED | 10 min |
| `VERIFYING` | DNS record found, ACME validation in progress | VERIFIED, FAILED | 2 min |
| `VERIFIED` | Challenge completed successfully | (terminal) | 24h cleanup |
| `EXPIRED` | Timeout waiting for DNS propagation | (terminal) | 24h cleanup |
| `FAILED` | Plugin error or ACME validation failure | (terminal) | 24h cleanup |

#### 3.3.3 Caddy Communication

Charon exposes an internal API for Caddy to delegate DNS challenge operations:

```
POST /internal/dns-challenge/create
{
  "provider_id": "uuid",
  "fqdn": "_acme-challenge.example.com",
  "value": "token-value"
}
Response: {"challenge_id": "uuid", "status": "pending"}

DELETE /internal/dns-challenge/{challenge_id}
Response: {"status": "deleted"}
```

#### 3.3.4 Error Handling When Charon is Unavailable

If Charon is unavailable during a DNS challenge:

1. **Caddy retry**: Caddy's built-in retry mechanism (3 attempts, exponential backoff)
2. **Graceful degradation**: If Charon remains unavailable, Caddy logs error and fails certificate issuance
3. **Health check**: Caddy pre-checks Charon availability via `/health` before initiating challenges
4. **Circuit breaker**: After 5 consecutive failures, Caddy disables the custom provider for 5 minutes

#### 3.3.5 Concurrent Challenge Handling

To prevent race conditions when multiple certificate requests target the same FQDN simultaneously:

**Database Locking Strategy:**

```sql
-- Acquire exclusive lock when creating challenge for FQDN
BEGIN;
SELECT * FROM dns_challenges
  WHERE fqdn = '_acme-challenge.example.com'
  AND status IN ('created', 'pending', 'verifying')
  FOR UPDATE NOWAIT;
-- If lock acquired and no active challenge exists, create new challenge
-- Otherwise, return CHALLENGE_IN_PROGRESS error
COMMIT;
```

**Queueing Behavior:**

| Scenario | Behavior |
|----------|----------|
| No active challenge for FQDN | Create new challenge immediately |
| Active challenge exists (same user) | Return existing challenge ID |
| Active challenge exists (different user) | Return `CHALLENGE_IN_PROGRESS` (409) |
| Active challenge expired/failed | Allow new challenge creation |

**Implementation Requirements:**

```go
func (s *ChallengeService) CreateChallenge(ctx context.Context, fqdn string, userID uint) (*Challenge, error) {
    tx := s.db.Begin()
    defer tx.Rollback()

    // Attempt to acquire lock on existing active challenges
    var existing Challenge
    err := tx.Set("gorm:query_option", "FOR UPDATE NOWAIT").
        Where("fqdn = ? AND status IN (?)", fqdn, []string{"created", "pending", "verifying"}).
        First(&existing).Error

    if err == nil {
        // Active challenge exists
        if existing.UserID == userID {
            return &existing, nil // Return existing challenge to same user
        }
        return nil, ErrChallengeInProgress // Different user, reject
    }

    if !errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, fmt.Errorf("lock acquisition failed: %w", err)
    }

    // No active challenge, create new one
    challenge := &Challenge{FQDN: fqdn, UserID: userID, Status: "created"}
    if err := tx.Create(challenge).Error; err != nil {
        return nil, err
    }

    tx.Commit()
    return challenge, nil
}
```

**Timeout Handling:**

- Challenges automatically transition to `expired` after 10 minutes
- Expired challenges release the "lock" on the FQDN
- Subsequent requests can then create new challenges

### 3.4 Database Model Impact

Current `dns_providers` table schema:

```sql
CREATE TABLE dns_providers (
    id INTEGER PRIMARY KEY,
    uuid VARCHAR(36) UNIQUE,
    name VARCHAR(255) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,     -- 'cloudflare', 'webhook', 'script', etc.
    enabled BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    credentials_encrypted TEXT,              -- Encrypted JSON blob
    key_version INTEGER DEFAULT 1,
    propagation_timeout INTEGER DEFAULT 120,
    polling_interval INTEGER DEFAULT 5,
    -- ... statistics fields
);
```

Custom plugins will use the same table with different `provider_type` values and plugin-specific credentials.

---

## 4. Proposed Solutions

### 4.1 Option A: Generic Webhook Plugin

#### Overview

User provides webhook URLs for create/delete TXT records. Charon POSTs JSON payloads with record details.

#### Configuration

```json
{
  "name": "My Webhook DNS",
  "provider_type": "webhook",
  "credentials": {
    "create_url": "https://api.example.com/dns/txt/create",
    "delete_url": "https://api.example.com/dns/txt/delete",
    "auth_header": "X-API-Key",
    "auth_value": "secret-token-here",
    "timeout_seconds": "30",
    "retry_count": "3"
  }
}
```

#### Request Payload (Sent to Webhook)

```json
{
  "action": "create",
  "fqdn": "_acme-challenge.example.com",
  "domain": "example.com",
  "subdomain": "_acme-challenge",
  "value": "gZrH7wL9t3kM2nP4...",
  "ttl": 300,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-08T15:30:00Z"
}
```

#### Expected Response

```json
{
  "success": true,
  "message": "TXT record created",
  "record_id": "optional-id-for-deletion"
}
```

#### Security Hardening

**DNS Rebinding Protection:**
Webhook URLs MUST be validated at both configuration time AND request execution time to prevent DNS rebinding attacks:

```go
// Configuration-time validation
func (w *WebhookProvider) ValidateCredentials(creds map[string]string) error {
    if err := security.ValidateExternalURL(creds["create_url"]); err != nil {
        return fmt.Errorf("create_url validation failed: %w", err)
    }
    // ... validate delete_url
}

// Execution-time validation (re-validate before each request)
func (w *WebhookProvider) executeWebhook(ctx context.Context, url string, payload []byte) error {
    // Re-validate URL to prevent DNS rebinding
    if err := security.ValidateExternalURL(url); err != nil {
        return fmt.Errorf("webhook URL failed re-validation: %w", err)
    }
    // ... execute request
}
```

**Response Size Limit:**

```go
const MaxWebhookResponseSize = 1 * 1024 * 1024 // 1MB

// Enforce response size limit
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()

limitedReader := io.LimitReader(resp.Body, MaxWebhookResponseSize+1)
body, err := io.ReadAll(limitedReader)
if len(body) > MaxWebhookResponseSize {
    return ErrWebhookResponseTooLarge
}
```

**TLS Validation:**

```json
{
  "credentials": {
    "insecure_skip_verify": false
  }
}
```

> ⚠️ **WARNING:** Setting `insecure_skip_verify: true` disables TLS certificate validation. This should ONLY be used in development/testing environments with self-signed certificates. NEVER enable in production.

**Idempotency Requirement:**
Webhook endpoints MUST support the `request_id` field for request deduplication. Charon will include a unique `request_id` (UUIDv4) in every webhook payload. Webhook implementations SHOULD:

1. Store processed `request_id` values with a TTL of at least 24 hours
2. Return cached response for duplicate `request_id` values
3. Use `request_id` for audit logging correlation

#### Rate Limiting and Circuit Breaker

To prevent abuse and ensure reliability, webhook plugins enforce:

| Limit | Value | Behavior |
|-------|-------|----------|
| Max calls per minute | 10 | Requests beyond limit return 429 Too Many Requests |
| Circuit breaker threshold | 5 consecutive failures | Provider disabled for 5 minutes |
| Circuit breaker reset | Automatic after 5 minutes | First successful call fully resets counter |
| Max response size | 1MB | Responses exceeding limit return 413 error |

**Implementation Requirements:**

```go
type WebhookRateLimiter struct {
    callsPerMinute    int           // Max 10
    consecutiveFails  int           // Track failures
    disabledUntil     time.Time     // Circuit breaker timestamp
}

func (w *WebhookProvider) executeWithRateLimit(ctx context.Context, req *WebhookRequest) error {
    if time.Now().Before(w.rateLimiter.disabledUntil) {
        return ErrProviderCircuitOpen
    }
    // ... execute webhook with rate limiting
}
```

#### Pros

- Works with any HTTP-capable system
- No code changes required on user side (just API endpoint)
- Supports complex authentication (headers, query params)
- Can integrate with existing automation (Terraform, Ansible AWX, etc.)

#### Cons

- User must implement and host webhook endpoint
- Network latency adds to propagation time
- Debugging requires access to both Charon and webhook logs
- Security: webhook credentials stored in Charon

#### Implementation Complexity

- Backend: ~200 lines (WebhookProvider implementation)
- Frontend: ~100 lines (form fields)
- Tests: ~150 lines

---

### 4.2 Option B: Custom Script Plugin

#### Overview

User provides path to shell script inside container. Script receives ACTION, DOMAIN, TOKEN, VALUE as arguments.

#### Configuration

```json
{
  "name": "My Script DNS",
  "provider_type": "script",
  "credentials": {
    "script_path": "/scripts/dns-update.sh",
    "timeout_seconds": "60",
    "env_vars": "DNS_SERVER=ns1.example.com,API_KEY=${API_KEY}"
  }
}
```

#### Script Interface

```bash
#!/bin/bash
# Called by Charon for DNS-01 challenge
# Arguments:
#   $1 = ACTION: "create" or "delete"
#   $2 = FQDN: "_acme-challenge.example.com"
#   $3 = TOKEN: Challenge token (for identification)
#   $4 = VALUE: TXT record value to set

ACTION="$1"
FQDN="$2"
TOKEN="$3"
VALUE="$4"

case "$ACTION" in
  create)
    # Create TXT record
    nsupdate <<EOF
server ${DNS_SERVER}
update add ${FQDN} 300 TXT "${VALUE}"
send
EOF
    ;;
  delete)
    # Delete TXT record
    nsupdate <<EOF
server ${DNS_SERVER}
update delete ${FQDN} TXT
send
EOF
    ;;
esac

# Exit code: 0 = success, non-zero = failure
```

#### Pros

- Maximum flexibility - any tool/language can be used
- Direct access to host system (if volume-mounted)
- Familiar paradigm for sysadmins
- Can leverage existing scripts/tooling

#### Cons

- **Security Risk:** Script execution in container context
- Harder to debug than API calls
- Script must be mounted into container
- No automatic retries (must implement in script)
- Sandboxing limits capability

#### Security Mitigations

1. Script must be in allowlisted directory (`/scripts/`)
2. Scripts run with restricted permissions (no network by default)
3. Timeout prevents resource exhaustion
4. All executions are audit-logged

#### Security Requirements (Mandatory)

**Argument Sanitization:**
All script arguments MUST be validated against a strict allowlist pattern:

```go
var validArgumentPattern = regexp.MustCompile(`^[a-zA-Z0-9._=-]+$`)

func sanitizeArgument(arg string) (string, error) {
    if !validArgumentPattern.MatchString(arg) {
        return "", ErrInvalidScriptArgument
    }
    if len(arg) > 1024 {
        return "", ErrArgumentTooLong
    }
    return arg, nil
}

// Usage
for i, arg := range args {
    sanitized, err := sanitizeArgument(arg)
    if err != nil {
        return fmt.Errorf("argument %d contains invalid characters: %w", i, err)
    }
    args[i] = sanitized
}
```

**Symlink Resolution:**
Path validation MUST use `filepath.EvalSymlinks()` BEFORE checking the allowed directory prefix to prevent symlink escape attacks:

```go
func validateScriptPath(scriptPath string) error {
    // CRITICAL: Resolve symlinks FIRST
    resolvedPath, err := filepath.EvalSymlinks(scriptPath)
    if err != nil {
        return fmt.Errorf("failed to resolve script path: %w", err)
    }

    // Then validate resolved path is within allowed directory
    absPath, err := filepath.Abs(resolvedPath)
    if err != nil {
        return fmt.Errorf("failed to resolve absolute path: %w", err)
    }

    allowedDir := "/scripts/"
    if !strings.HasPrefix(absPath, allowedDir) {
        return ErrScriptPathInvalid
    }

    return nil
}
```

**Resource Limits (MANDATORY):**
The following rlimits MUST be enforced for all script executions:

| Resource | Limit | Purpose |
|----------|-------|------|
| `RLIMIT_NOFILE` | 256 | Prevent file descriptor exhaustion |
| `RLIMIT_NPROC` | 64 | Prevent fork bombs |
| `RLIMIT_AS` | 256MB | Prevent memory exhaustion |
| `RLIMIT_CPU` | 60s | Prevent CPU exhaustion |
| `RLIMIT_FSIZE` | 10MB | Prevent disk filling |

```go
// MANDATORY: Apply rlimits before script execution
func setMandatoryResourceLimits() error {
    limits := []struct {
        resource int
        limit    uint64
    }{
        {syscall.RLIMIT_NOFILE, 256},
        {syscall.RLIMIT_NPROC, 64},
        {syscall.RLIMIT_AS, 256 * 1024 * 1024},
        {syscall.RLIMIT_CPU, 60},
        {syscall.RLIMIT_FSIZE, 10 * 1024 * 1024},
    }

    for _, l := range limits {
        if err := syscall.Setrlimit(l.resource, &syscall.Rlimit{Cur: l.limit, Max: l.limit}); err != nil {
            return fmt.Errorf("failed to set rlimit %d: %w", l.resource, err)
        }
    }
    return nil
}
```

**Environment Variable Clearing:**
Inherited environment variables MUST be explicitly cleared before setting script environment:

```go
func executeScript(scriptPath string, args []string, userEnv map[string]string) error {
    cmd := exec.CommandContext(ctx, scriptPath, args...)

    // CRITICAL: Start with empty environment (clear inherited vars)
    cmd.Env = []string{}

    // Add only essential system variables
    cmd.Env = append(cmd.Env,
        "PATH=/usr/local/bin:/usr/bin:/bin",
        "HOME=/tmp",
        "LANG=C.UTF-8",
        "TZ=UTC",
    )

    // Add user-provided environment variables (after validation)
    for key, value := range userEnv {
        if err := validateEnvVar(key, value); err != nil {
            return fmt.Errorf("invalid env var %s: %w", key, err)
        }
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
    }

    // Execute with cleared environment
    return cmd.Run()
}
```

#### Implementation Complexity

- Backend: ~250 lines (ScriptProvider + executor)
- Frontend: ~80 lines (form fields)
- Tests: ~200 lines (including security tests)

---

### 4.3 Option C: RFC 2136 (Dynamic DNS Update) Plugin

#### Overview

RFC 2136 defines a standard protocol for dynamic DNS updates. Supported by BIND, PowerDNS, Knot DNS, and many self-hosted DNS servers.

#### Configuration

```json
{
  "name": "My BIND Server",
  "provider_type": "rfc2136",
  "credentials": {
    "nameserver": "ns1.example.com",
    "port": "53",
    "tsig_key_name": "acme-update-key",
    "tsig_key_secret": "base64-encoded-secret",
    "tsig_algorithm": "hmac-sha256",
    "zone": "example.com"
  }
}
```

#### TSIG Algorithms Supported

| Algorithm | Status | Notes |
|-----------|--------|-------|
| `hmac-md5` | ⚠️ **DEPRECATED** | Cryptographically weak; will be removed in v2.0 |
| `hmac-sha1` | Legacy | Avoid for new deployments |
| `hmac-sha256` | ✅ Recommended | Default for new configurations |
| `hmac-sha384` | Supported | Higher security, slightly more overhead |
| `hmac-sha512` | Supported | Highest security |

> ⚠️ **DEPRECATION WARNING:** `hmac-md5` is cryptographically weak and should not be used for new deployments. Support for `hmac-md5` will be removed in Charon v2.0. Migrate to `hmac-sha256` or stronger.

**Secure Memory Handling for TSIG Secrets:**

TSIG secrets MUST be handled securely in memory:

```go
import "github.com/awnumar/memguard"

type RFC2136Provider struct {
    tsigSecret *memguard.Enclave // Encrypted in memory
}

func (r *RFC2136Provider) SetTSIGSecret(secret []byte) error {
    // Store secret in encrypted memory enclave
    enclave := memguard.NewEnclave(secret)

    // Immediately wipe the source buffer
    memguard.WipeBytes(secret)

    r.tsigSecret = enclave
    return nil
}

func (r *RFC2136Provider) Cleanup() error {
    if r.tsigSecret != nil {
        r.tsigSecret.Destroy()
    }
    return nil
}
```

**Requirements:**

1. TSIG secrets MUST be stored in encrypted memory enclaves when in use
2. Source buffers containing secrets MUST be wiped immediately after copying
3. Secrets MUST NOT appear in debug output, stack traces, or core dumps
4. Provider `Cleanup()` MUST securely destroy all secret material

#### DNS UPDATE Message Flow

```
┌──────────┐                    ┌──────────────┐
│  Charon  │                    │  DNS Server  │
│          │  DNS UPDATE        │  (BIND, etc) │
│          │  ─────────────────▶│              │
│          │  TSIG-signed       │              │
│          │                    │              │
│          │  RESPONSE          │              │
│          │  ◀─────────────────│              │
│          │  NOERROR/REFUSED   │              │
└──────────┘                    └──────────────┘
```

#### Caddy Integration

Caddy has a native RFC 2136 module: [caddy-dns/rfc2136](https://github.com/caddy-dns/rfc2136)

**DECISION:** Charon WILL ship with the RFC 2136 Caddy module pre-built in the Docker image. Users do NOT need to rebuild Caddy.

The Charon plugin would:

1. Store TSIG credentials encrypted
2. Generate Caddy config with proper RFC 2136 settings
3. Validate credentials by attempting a test query

**Dockerfile Addition (Phase 2):**

```dockerfile
# Build Caddy with RFC 2136 module
FROM caddy:builder AS caddy-builder
RUN xcaddy build \
    --with github.com/caddy-dns/rfc2136
```

#### Pros

- Industry-standard protocol
- No custom server-side code needed
- Works with popular DNS servers (BIND9, PowerDNS, Knot)
- Secure with TSIG authentication
- Native Caddy module available

#### Cons

- Requires DNS server configuration for TSIG keys
- More complex setup than webhook
- Zone configuration required
- Firewall rules may need updating (TCP/UDP 53)

#### Implementation Complexity

- Backend: ~180 lines (RFC2136Provider)
- Frontend: ~120 lines (TSIG configuration form)
- Tests: ~150 lines
- Requires: Caddy rebuild with `caddy-dns/rfc2136` module

---

### 4.4 Option D: Manual/External Plugin

#### Overview

No automation - UI shows required TXT record details, user creates manually, clicks "Verify" when done.

#### UI Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Manual DNS Challenge                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  To obtain a certificate for *.example.com, create the following    │
│  TXT record at your DNS provider:                                   │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  Record Name:  _acme-challenge.example.com         [📋 Copy]  │ │
│  ├────────────────────────────────────────────────────────────────┤ │
│  │  Record Value: gZrH7wL9t3kM2nP4qX5yR8sT...         [📋 Copy]  │ │
│  ├────────────────────────────────────────────────────────────────┤ │
│  │  TTL: 300 (5 minutes)                                          │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                      │
│  ⏱️ Time remaining: 4:32                                             │
│  [━━━━━━━━━━━━━━━━━━━━━░░░░░░░░░░] 68%                              │
│                                                                      │
│  [Check DNS Now]  [I've Created the Record - Verify]                │
│                                                                      │
│  ℹ️ Record not yet propagated. Last check: 10 seconds ago            │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### Configuration

```json
{
  "name": "Manual DNS",
  "provider_type": "manual",
  "credentials": {
    "timeout_minutes": "10",
    "polling_interval_seconds": "30"
  }
}
```

#### Technical Implementation

- Store challenge details in session/database
- Background job periodically queries DNS
- Polling endpoint for UI updates (10-second interval)
- Timeout after configurable period

#### Session Security Requirements

**Challenge-User Binding:**
Manual challenges MUST be bound to the authenticated user's session:

```go
type Challenge struct {
    ID        string    `json:"id"`         // UUIDv4 (cryptographically random)
    UserID    uint      `json:"user_id"`    // Owner of this challenge
    SessionID string    `json:"-"`          // Session that created challenge
    // ... other fields
}

// Verify challenge ownership before any operation
func (s *ManualChallengeService) VerifyOwnership(ctx context.Context, challengeID string, userID uint) error {
    var challenge Challenge
    if err := s.db.Where("id = ?", challengeID).First(&challenge).Error; err != nil {
        return ErrChallengeNotFound
    }

    if challenge.UserID != userID {
        // Log potential unauthorized access attempt
        s.auditLog.Warn("unauthorized challenge access attempt",
            "challenge_id", challengeID,
            "owner_id", challenge.UserID,
            "requester_id", userID,
        )
        return ErrUnauthorized
    }

    return nil
}
```

**CSRF Protection:**
All state-changing operations (POST, PUT, DELETE) on manual challenges MUST validate CSRF tokens:

```go
// Middleware for manual challenge endpoints
func CSRFProtection(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
            token := r.Header.Get("X-CSRF-Token")
            sessionToken := getSessionCSRFToken(r)

            if !secureCompare(token, sessionToken) {
                http.Error(w, "CSRF token mismatch", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

**Challenge ID Generation:**
Challenge IDs MUST use cryptographically random UUIDs (UUIDv4):

```go
import "github.com/google/uuid"

func generateChallengeID() string {
    // UUIDv4 uses crypto/rand, providing 122 bits of randomness
    return uuid.New().String()
}

// DO NOT use:
// - Sequential IDs (predictable)
// - UUIDv1 (contains timestamp/MAC address)
// - Custom random without proper entropy
```

**Session Validation on Each Request:**

| Endpoint | Required Validations |
|----------|---------------------|
| `GET /manual-challenge/:id` | Valid session, challenge.user_id == session.user_id |
| `POST /manual-challenge/:id/verify` | Valid session, CSRF token, challenge ownership |
| `DELETE /manual-challenge/:id` | Valid session, CSRF token, challenge ownership |

**Note:** Although Charon has existing WebSocket infrastructure (`backend/internal/services/websocket_tracker.go`), polling is chosen for simplicity:

- Avoids additional WebSocket connection management complexity
- 10-second polling interval provides acceptable UX for manual workflows
- Reduces frontend state management burden

**Polling Endpoint:**

```
GET /api/v1/dns-providers/:id/manual-challenge/:challengeId/poll
Response (every 10s):
{
  "status": "pending|verified|expired|failed",
  "dns_propagated": false,
  "time_remaining_seconds": 432,
  "last_check_at": "2026-01-08T15:35:00Z"
}
```

#### Pros

- Works with ANY DNS provider
- No integration required
- Good for testing/development
- One-off certificate issuance

#### Cons

- User must manually intervene
- Time-sensitive (ACME challenge timeout)
- Not suitable for automated renewals
- Doesn't scale for multiple certificates

#### Implementation Complexity

- Backend: ~150 lines (ManualProvider + verification endpoint)
- Frontend: ~300 lines (interactive UI with copy/verify)
- Tests: ~100 lines

---

## 5. Recommended Approach

### Phase 1: Manual Plugin (1 week)

**Rationale:** Unblocks all users immediately. Lowest risk, highest immediate value.

Deliverables:

- ManualProvider implementation
- Interactive challenge UI
- DNS verification endpoint
- User documentation

### Phase 2: RFC 2136 Plugin (1 week)

**Rationale:** Standards-based, serves self-hosted DNS users. Caddy module already exists.

Deliverables:

- RFC2136Provider implementation
- TSIG credential storage
- Caddy module integration documentation
- BIND9/PowerDNS setup guides

### Phase 3: Webhook Plugin (1 week)

**Rationale:** Most flexible option for custom integrations. Medium complexity.

Deliverables:

- WebhookProvider implementation
- Configurable retry logic
- Request/response logging
- Example webhook implementations (Node.js, Python)

---

## Future Work

### Phase 4: Script Plugin (Conditional)

> **Go/No-Go Gate:** Phase 4 only proceeds if >20 user requests are received via GitHub issues requesting script plugin functionality. Track via label `feature:script-plugin`.

**Rationale:** Power-user feature with significant security implications. Implement only if demand warrants the additional security review and maintenance burden.

Deliverables:

- ScriptProvider implementation
- Security sandbox
- Example scripts for common scenarios

### Implementation Order Justification

```
User Value
    │
    │  ★ Manual Plugin (Phase 1)
    │    - Unblocks everyone immediately
    │    - Lowest implementation risk
    │
    │  ★ RFC 2136 Plugin (Phase 2)
    │    - Self-hosted DNS is common need
    │    - Industry standard
    │
    │  ★ Webhook Plugin (Phase 3)
    │    - Flexible for edge cases
    │    - Integration-focused teams
    │
    │  ○ Script Plugin (Phase 4)
    │    - Power users only
    │    - Security concerns
    │
    └────────────────────────────────▶ Implementation Effort
```

---

## 6. Database Schema Changes

### 6.1 No New Tables Required

The existing `dns_providers` table schema supports custom plugins. The `provider_type` column accepts new values, and `credentials_encrypted` stores plugin-specific configuration.

### 6.2 Provider Type Enumeration

Expand the allowed `provider_type` values:

```go
// backend/pkg/dnsprovider/types.go
const (
    // Built-in providers
    TypeCloudflare    = "cloudflare"
    TypeRoute53       = "route53"
    // ... existing providers

    // Custom plugins
    TypeWebhook       = "webhook"
    TypeScript        = "script"
    TypeRFC2136       = "rfc2136"
    TypeManual        = "manual"
)
```

### 6.3 Credential Schemas Per Plugin Type

#### Webhook Credentials

```json
{
  "create_url": "string (required)",
  "delete_url": "string (required)",
  "auth_header": "string (optional)",
  "auth_value": "string (optional, encrypted)",
  "content_type": "string (default: application/json)",
  "timeout_seconds": "integer (default: 30)",
  "retry_count": "integer (default: 3)",
  "custom_headers": "object (optional)"
}
```

#### Script Credentials

```json
{
  "script_path": "string (required)",
  "timeout_seconds": "integer (default: 60)",
  "working_directory": "string (optional)",
  "env_vars": "string (optional, KEY=VALUE format)"
}
```

#### RFC 2136 Credentials

```json
{
  "nameserver": "string (required)",
  "port": "integer (default: 53)",
  "tsig_key_name": "string (required)",
  "tsig_key_secret": "string (required, encrypted)",
  "tsig_algorithm": "string (default: hmac-sha256)",
  "zone": "string (optional, auto-detect)"
}
```

#### Manual Credentials

```json
{
  "timeout_minutes": "integer (default: 10)",
  "polling_interval_seconds": "integer (default: 30)"
}
```

### 6.4 Challenge Cleanup Mechanism

Challenges are cleaned up via Charon's existing scheduled task infrastructure (using `robfig/cron/v3`, same pattern as `backup_service.go`):

```go
// Cleanup job runs hourly
func (s *ManualChallengeService) scheduleCleanup() {
    _, err := s.cron.AddFunc("0 * * * *", s.cleanupExpiredChallenges)
    // ...
}

func (s *ManualChallengeService) cleanupExpiredChallenges() {
    // Mark challenges in "pending" state > 24 hours as "expired"
    // Delete challenge records > 7 days old
    cutoff := time.Now().Add(-24 * time.Hour)
    s.db.Model(&Challenge{}).
        Where("status = ? AND created_at < ?", "pending", cutoff).
        Update("status", "expired")

    // Hard delete after 7 days
    deleteCutoff := time.Now().Add(-7 * 24 * time.Hour)
    s.db.Where("created_at < ?", deleteCutoff).Delete(&Challenge{})
}
```

**Cleanup Schedule:**

| Condition | Action | Frequency |
|-----------|--------|-----------|
| `pending` status > 24 hours | Mark as `expired` | Hourly |
| Any challenge > 7 days old | Hard delete | Hourly |

---

## 7. API Design

### 7.1 Existing Endpoints (No Changes)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/dns-providers` | List all providers |
| POST | `/api/v1/dns-providers` | Create provider |
| GET | `/api/v1/dns-providers/:id` | Get provider |
| PUT | `/api/v1/dns-providers/:id` | Update provider |
| DELETE | `/api/v1/dns-providers/:id` | Delete provider |
| POST | `/api/v1/dns-providers/:id/test` | Test credentials |
| GET | `/api/v1/dns-providers/types` | List provider types |

### 7.2 New Endpoints

#### Manual Challenge Status

```
GET /api/v1/dns-providers/:id/manual-challenge/:challengeId
```

Response:

```json
{
  "id": "challenge-uuid",
  "status": "pending|verified|expired|failed",
  "fqdn": "_acme-challenge.example.com",
  "value": "gZrH7wL9t3kM2nP4...",
  "created_at": "2026-01-08T15:30:00Z",
  "expires_at": "2026-01-08T15:40:00Z",
  "last_check_at": "2026-01-08T15:35:00Z",
  "dns_propagated": false
}
```

#### Manual Challenge Verification Trigger

```
POST /api/v1/dns-providers/:id/manual-challenge/:challengeId/verify
```

Response:

```json
{
  "success": true,
  "dns_found": true,
  "message": "TXT record verified successfully"
}
```

### 7.3 Error Response Codes

All manual challenge and custom plugin endpoints use consistent error codes:

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `CHALLENGE_NOT_FOUND` | 404 | Challenge ID does not exist |
| `CHALLENGE_EXPIRED` | 410 | Challenge has timed out |
| `CHALLENGE_IN_PROGRESS` | 409 | Another challenge is currently active for this FQDN |
| `DNS_NOT_PROPAGATED` | 200 | DNS record not yet found (success: false) |
| `INVALID_PROVIDER_TYPE` | 400 | Unknown provider type |
| `INVALID_SCRIPT_ARGUMENT` | 400 | Script argument contains invalid characters (only `[a-zA-Z0-9._=-]` allowed) |
| `WEBHOOK_TIMEOUT` | 504 | Webhook did not respond in time |
| `WEBHOOK_RATE_LIMITED` | 429 | Too many webhook calls (>10/min) |
| `WEBHOOK_RESPONSE_TOO_LARGE` | 413 | Webhook response exceeded 1MB limit |
| `PROVIDER_CIRCUIT_OPEN` | 503 | Provider disabled due to consecutive failures |
| `SCRIPT_TIMEOUT` | 504 | Script execution exceeded timeout |
| `SCRIPT_PATH_INVALID` | 400 | Script path not in allowed directory |
| `TSIG_AUTH_FAILED` | 401 | RFC 2136 TSIG authentication failed |

**Error Response Format:**

```json
{
  "success": false,
  "error": {
    "code": "CHALLENGE_EXPIRED",
    "message": "Challenge timed out after 10 minutes",
    "details": {
      "challenge_id": "uuid",
      "expired_at": "2026-01-08T15:40:00Z"
    }
  }
}
```

### 7.4 Updated Types Endpoint Response

The existing `/api/v1/dns-providers/types` endpoint will include custom plugins:

```json
{
  "types": [
    {
      "type": "cloudflare",
      "name": "Cloudflare",
      "is_built_in": true,
      "fields": [...]
    },
    {
      "type": "webhook",
      "name": "Webhook (Generic)",
      "is_built_in": false,
      "category": "custom",
      "fields": [
        {"name": "create_url", "label": "Create Record URL", "type": "text", "required": true},
        {"name": "delete_url", "label": "Delete Record URL", "type": "text", "required": true},
        {"name": "auth_header", "label": "Auth Header Name", "type": "text", "required": false},
        {"name": "auth_value", "label": "Auth Header Value", "type": "password", "required": false}
      ]
    },
    {
      "type": "rfc2136",
      "name": "RFC 2136 (Dynamic DNS)",
      "is_built_in": false,
      "category": "custom",
      "fields": [
        {"name": "nameserver", "label": "DNS Server", "type": "text", "required": true},
        {"name": "tsig_key_name", "label": "TSIG Key Name", "type": "text", "required": true},
        {"name": "tsig_key_secret", "label": "TSIG Secret", "type": "password", "required": true},
        {"name": "tsig_algorithm", "label": "TSIG Algorithm", "type": "select", "options": [...]}
      ]
    },
    {
      "type": "manual",
      "name": "Manual (No Automation)",
      "is_built_in": false,
      "category": "custom",
      "fields": [
        {"name": "timeout_minutes", "label": "Challenge Timeout (minutes)", "type": "number", "default": "10"}
      ]
    }
  ]
}
```

---

## 8. Frontend UI Mockups

### 8.1 Provider Type Selection (Updated)

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Add DNS Provider                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Select Provider Type:                                               │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ BUILT-IN PROVIDERS                                              ││
│  │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐││
│  │ │ ☁️ Cloudflare│ │ 🔶 Route53  │ │ 💧 Digital  │ │ 🔷 Azure    │││
│  │ │             │ │             │ │    Ocean    │ │             │││
│  │ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘││
│  │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐││
│  │ │ 🌐 Google   │ │ 🟠 Hetzner  │ │ 📛 GoDaddy  │ │ 🔵 Namecheap│││
│  │ │   Cloud DNS │ │             │ │             │ │             │││
│  │ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ CUSTOM INTEGRATIONS                                             ││
│  │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐││
│  │ │ 🔗 Webhook  │ │ 📜 Script   │ │ 📡 RFC 2136 │ │ ✋ Manual   │││
│  │ │   (HTTP)    │ │   (Shell)   │ │   (DDNS)    │ │             │││
│  │ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│                                           [Cancel]  [Next →]         │
└─────────────────────────────────────────────────────────────────────┘
```

### 8.2 Webhook Configuration Form

```
┌─────────────────────────────────────────────────────────────────────┐
│                   Configure Webhook Provider                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Provider Name:                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ My Custom DNS Webhook                                           ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  Create Record URL: *                                                │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ https://api.example.com/dns/create                              ││
│  └─────────────────────────────────────────────────────────────────┘│
│  ℹ️ Charon will POST JSON with record details                        │
│                                                                      │
│  Delete Record URL: *                                                │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ https://api.example.com/dns/delete                              ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ── Authentication (Optional) ──────────────────────────────────────│
│                                                                      │
│  Header Name:                    Header Value:                       │
│  ┌───────────────────┐          ┌───────────────────────────────┐  │
│  │ X-API-Key         │          │ ••••••••••••••                │  │
│  └───────────────────┘          └───────────────────────────────┘  │
│                                                                      │
│  ── Advanced Settings ──────────────────────────────────────────────│
│                                                                      │
│  Timeout (seconds):  [30 ▼]    Retry Count:  [3 ▼]                  │
│                                                                      │
│                                                                      │
│         [Test Connection]        [Cancel]  [Save Provider]           │
└─────────────────────────────────────────────────────────────────────┘
```

### 8.3 RFC 2136 Configuration Form

```
┌─────────────────────────────────────────────────────────────────────┐
│                   Configure RFC 2136 Provider                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Provider Name:                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ Internal BIND Server                                            ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  DNS Server: *                          Port:                        │
│  ┌─────────────────────────────────────┐ ┌─────────────────────────┐│
│  │ ns1.internal.example.com            │ │ 53                      ││
│  └─────────────────────────────────────┘ └─────────────────────────┘│
│                                                                      │
│  ── TSIG Authentication ────────────────────────────────────────────│
│                                                                      │
│  Key Name: *                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ acme-update-key.example.com                                     ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  Key Secret: *                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ ••••••••••••••••••••••••••••••••                                ││
│  └─────────────────────────────────────────────────────────────────┘│
│  ℹ️ Base64-encoded TSIG secret                                       │
│                                                                      │
│  Algorithm:                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ HMAC-SHA256 (Recommended)                                    ▼ ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  Zone (optional - auto-detected if empty):                          │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │                                                                 ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│         [Test Connection]        [Cancel]  [Save Provider]           │
└─────────────────────────────────────────────────────────────────────┘
```

### 8.4 Manual Challenge UI

```
┌─────────────────────────────────────────────────────────────────────┐
│                   🔐 Manual DNS Challenge                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Certificate Request: *.example.com                                  │
│  Provider: Manual DNS (example-manual)                               │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  📋 CREATE THIS TXT RECORD AT YOUR DNS PROVIDER                 ││
│  │                                                                  ││
│  │  Record Name:                                                    ││
│  │  ┌──────────────────────────────────────────────────┐  ┌──────┐││
│  │  │ _acme-challenge.example.com                      │  │ Copy │││
│  │  └──────────────────────────────────────────────────┘  └──────┘││
│  │                                                                  ││
│  │  Record Type: TXT                                                ││
│  │                                                                  ││
│  │  Record Value:                                                   ││
│  │  ┌──────────────────────────────────────────────────┐  ┌──────┐││
│  │  │ gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2aB3cD4eF5gH6iJ7  │  │ Copy │││
│  │  └──────────────────────────────────────────────────┘  └──────┘││
│  │                                                                  ││
│  │  TTL: 300 seconds (5 minutes)                                   ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │  ⏱️ Time Remaining: 7:23                                         ││
│  │  [━━━━━━━━━━━━━━━━━░░░░░░░░░░░░░░░] 52%                         ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  Status: ⏳ Waiting for DNS propagation...                           │
│  Last checked: 15 seconds ago                                        │
│                                                                      │
│  ┌─────────────────────┐  ┌────────────────────────────────────────┐│
│  │  🔍 Check DNS Now   │  │  ✅ I've Created the Record - Verify   ││
│  └─────────────────────┘  └────────────────────────────────────────┘│
│                                                                      │
│                                           [Cancel Challenge]         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 9. Security Considerations

### 9.1 Threat Model

| Threat | Risk Level | Mitigation |
|--------|------------|------------|
| Credential theft from database | High | AES-256-GCM encryption at rest, key rotation |
| Webhook URL SSRF | High | URL validation, internal IP blocking |
| Script path traversal | Critical | Allowlist `/scripts/` directory only |
| Script command injection | Critical | Sanitize all arguments, no shell expansion |
| TSIG key exposure in logs | Medium | Redact secrets in all logs |
| DNS cache poisoning | Low | TSIG authentication for RFC 2136 |
| Webhook response injection | Low | Strict JSON parsing, no eval |

### 9.2 SSRF Prevention for Webhooks

Webhook URL validation MUST use Charon's existing centralized SSRF protection in `backend/internal/security/url_validator.go`:

```go
// backend/internal/services/webhook_provider.go
import "github.com/Wikid82/charon/backend/internal/security"

func (w *WebhookProvider) validateWebhookURL(urlStr string) error {
    // Use existing centralized SSRF validation
    // This validates:
    // - HTTPS scheme required (production)
    // - DNS resolution with timeout
    // - All resolved IPs checked against private/reserved ranges
    // - Cloud metadata endpoints blocked (169.254.169.254)
    // - IPv4-mapped IPv6 bypass prevention
    _, err := security.ValidateExternalURL(urlStr)
    if err != nil {
        return fmt.Errorf("webhook URL validation failed: %w", err)
    }
    return nil
}
```

**Existing `security.ValidateExternalURL()` provides:**

- RFC 1918 private network blocking (10.x, 172.16.x, 192.168.x)
- Loopback blocking (127.x.x.x, ::1) unless `WithAllowLocalhost()` option
- Link-local blocking (169.254.x.x, fe80::) including cloud metadata
- Reserved range blocking (0.x.x.x, 240.x.x.x)
- IPv6 unique local blocking (fc00::)
- IPv4-mapped IPv6 bypass prevention (::ffff:192.168.1.1)
- Hostname length validation (RFC 1035, max 253 chars)
- Suspicious pattern detection (..)
- Port range validation with privileged port blocking

**DO NOT** duplicate SSRF validation logic. Reference the existing implementation.

```

### 9.3 Script Execution Security

```go
// backend/internal/services/script_provider.go
import (
    "context"
    "os/exec"
    "syscall"
)

func executeScript(scriptPath string, args []string) error {
    // 1. Validate script path
    allowedDir := "/scripts/"
    absPath, _ := filepath.Abs(scriptPath)
    if !strings.HasPrefix(absPath, allowedDir) {
        return errors.New("script must be in /scripts/ directory")
    }

    // 2. Verify script exists and is executable
    info, err := os.Stat(absPath)
    if err != nil || info.IsDir() {
        return errors.New("invalid script path")
    }

    // 3. Create restricted command with timeout wrapper (defense-in-depth)
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Use 'timeout' command as additional safeguard against hung processes
    cmd := exec.CommandContext(ctx, "timeout", "--signal=KILL", "55s", absPath)
    cmd.Args = append(cmd.Args, args...)
    cmd.Dir = allowedDir

    // 4. Minimal but functional environment
    cmd.Env = []string{
        "PATH=/usr/local/bin:/usr/bin:/bin",
        "HOME=/tmp",
        "LANG=C.UTF-8",
    }

    // 5. Resource limits via rlimit (prevents resource exhaustion)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Credential: &syscall.Credential{
            Uid: 65534, // nobody user
            Gid: 65534,
        },
    }

    // Apply resource limits
    setResourceLimits(cmd)

    // 6. Capture output for logging
    output, err := cmd.CombinedOutput()

    // 7. Audit log
    logScriptExecution(scriptPath, args, cmd.ProcessState.ExitCode(), output)

    return err
}

// setResourceLimits applies rlimits to prevent resource exhaustion
// Note: These are set via prlimit(2) or container security context
func setResourceLimits(cmd *exec.Cmd) {
    // RLIMIT_NOFILE: Max open file descriptors (prevent fd exhaustion)
    // RLIMIT_NPROC: Max processes (prevent fork bombs)
    // RLIMIT_AS: Max address space (prevent memory exhaustion)
    //
    // Recommended values:
    // - NOFILE: 256
    // - NPROC: 64
    // - AS: 256MB
    //
    // Implementation note: In containerized deployments, these limits
    // should be enforced via container security context (securityContext
    // in Kubernetes, --ulimit in Docker) for stronger isolation.
}
```

**Security Layers (Defense-in-Depth):**

| Layer | Protection | Implementation |
|-------|------------|----------------|
| 1. Path validation | Restrict to `/scripts/` | `filepath.Abs()` + prefix check |
| 2. Timeout | Prevent hung scripts | `context.WithTimeout` + `timeout` command |
| 3. Resource limits | Prevent resource exhaustion | `rlimit` (NOFILE=256, NPROC=64, AS=256MB) |
| 4. Minimal environment | Reduce attack surface | Explicit `PATH`, no secrets |
| 5. Non-root execution | Limit privilege | `nobody` user (UID 65534) |
| 6. Container isolation | Strongest isolation | seccomp profile (see below) |
| 7. Audit logging | Forensics | All executions logged |

**Container Security (seccomp profile):**

For production deployments, scripts run within Charon's container which should have a restrictive seccomp profile. Document this requirement:

```yaml
# docker-compose.yml (recommended)
services:
  charon:
    security_opt:
      - seccomp:seccomp-profile.json  # Or use default Docker profile
    # Alternative: Use --cap-drop=ALL --cap-add=<minimal>
```

**Note:** Full seccomp profile customization is out of scope for this feature. Users relying on script plugins in high-security environments should review container security configuration.

```

### 9.4 Log Redaction Patterns

Sensitive data MUST be redacted from all logs, including debug logs, error messages, and audit trails.

**Required Redaction Patterns:**

| Field Pattern | Redaction | Example |
|---------------|-----------|--------|
| `api_token` | `[REDACTED:api_token]` | `Bearer abc123` → `Bearer [REDACTED:api_token]` |
| `api_key` | `[REDACTED:api_key]` | `X-API-Key: secret` → `X-API-Key: [REDACTED:api_key]` |
| `secret` | `[REDACTED:secret]` | `client_secret=xyz` → `client_secret=[REDACTED:secret]` |
| `password` | `[REDACTED:password]` | `password=abc` → `password=[REDACTED:password]` |
| `tsig_key_secret` | `[REDACTED:tsig_secret]` | TSIG key value → `[REDACTED:tsig_secret]` |
| `authorization` | `[REDACTED:auth]` | `Authorization: Bearer ...` → `Authorization: [REDACTED:auth]` |
| `bearer` | `[REDACTED:bearer]` | Bearer token values → `[REDACTED:bearer]` |

**Implementation:**

```go
import "regexp"

var sensitivePatterns = []struct {
    pattern *regexp.Regexp
    replace string
}{
    {regexp.MustCompile(`(?i)(api_token["']?\s*[:=]\s*["']?)[^"'\s,}]+`), `$1[REDACTED:api_token]`},
    {regexp.MustCompile(`(?i)(api_key["']?\s*[:=]\s*["']?)[^"'\s,}]+`), `$1[REDACTED:api_key]`},
    {regexp.MustCompile(`(?i)(secret["']?\s*[:=]\s*["']?)[^"'\s,}]+`), `$1[REDACTED:secret]`},
    {regexp.MustCompile(`(?i)(password["']?\s*[:=]\s*["']?)[^"'\s,}]+`), `$1[REDACTED:password]`},
    {regexp.MustCompile(`(?i)(tsig_key_secret["']?\s*[:=]\s*["']?)[^"'\s,}]+`), `$1[REDACTED:tsig_secret]`},
    {regexp.MustCompile(`(?i)(authorization["']?\s*[:=]\s*["']?)(Bearer\s+)?[^"'\s,}]+`), `$1[REDACTED:auth]`},
    {regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-_=]+\.?[A-Za-z0-9\-_=]*\.?[A-Za-z0-9\-_=]*`), `Bearer [REDACTED:bearer]`},
}

func RedactSensitiveData(input string) string {
    result := input
    for _, sp := range sensitivePatterns {
        result = sp.pattern.ReplaceAllString(result, sp.replace)
    }
    return result
}

// Apply to all log output
func (l *Logger) LogWithRedaction(level, msg string, fields map[string]any) {
    // Redact message
    msg = RedactSensitiveData(msg)

    // Redact field values
    for key, value := range fields {
        if str, ok := value.(string); ok {
            fields[key] = RedactSensitiveData(str)
        }
    }

    l.underlying.Log(level, msg, fields)
}
```

**Enforcement:**

- All plugin code MUST use the redacting logger
- Pre-commit hooks SHOULD scan for potential credential logging
- Security tests MUST verify no secrets appear in logs

### 9.5 Audit Logging

All custom plugin operations MUST be logged (with redaction applied):

```go
type PluginAuditEvent struct {
    Timestamp   time.Time
    PluginType  string // "webhook", "script", "rfc2136", "manual"
    Action      string // "create_record", "delete_record", "verify"
    ProviderID  uint
    Domain      string
    Success     bool
    Duration    time.Duration
    ErrorMsg    string           // Redacted before logging
    Details     map[string]any   // Redacted credentials
}
```

---

## 10. Implementation Phases

### Phase 1: Manual Plugin (Week 1)

| Task | Hours | Owner |
|------|-------|-------|
| ManualProvider implementation | 4 | Backend |
| Manual challenge data model | 2 | Backend |
| Challenge verification endpoint | 3 | Backend |
| Polling endpoint (10s interval) | 2 | Backend |
| Manual challenge UI component | 6 | Frontend |
| Challenge cleanup scheduled task | 2 | Backend |
| Unit tests | 4 | QA |
| Integration tests | 3 | QA |
| i18n translation keys | 2 | Frontend |
| Documentation | 2 | Docs |
| **Total** | **32** | |
| **With 20% buffer** | **32** | |

**Deliverables:**

- [ ] `backend/pkg/dnsprovider/custom/manual.go`
- [ ] `backend/internal/services/manual_challenge_service.go`
- [ ] `frontend/src/components/ManualDNSChallenge.tsx`
- [ ] API endpoints for challenge lifecycle (including `/poll`)
- [ ] Translation keys in `frontend/src/locales/*/translation.json`:
  - `dnsProvider.manual.title`
  - `dnsProvider.manual.instructions`
  - `dnsProvider.manual.recordName`
  - `dnsProvider.manual.recordValue`
  - `dnsProvider.manual.copyButton`
  - `dnsProvider.manual.verifyButton`
  - `dnsProvider.manual.checkDnsButton`
  - `dnsProvider.manual.timeRemaining`
  - `dnsProvider.manual.status.pending`
  - `dnsProvider.manual.status.verified`
  - `dnsProvider.manual.status.expired`
  - `dnsProvider.manual.status.failed`
  - `dnsProvider.manual.errors.*`
- [ ] User guide: `docs/features/manual-dns-challenge.md`

### Phase 2: RFC 2136 Plugin (Week 2)

| Task | Hours | Owner |
|------|-------|-------|
| RFC2136Provider implementation | 4 | Backend |
| TSIG credential validation | 3 | Backend |
| Caddy module integration research | 2 | Backend |
| **Dockerfile update (xcaddy + rfc2136)** | 2 | DevOps |
| RFC 2136 form UI | 4 | Frontend |
| i18n translation keys | 1 | Frontend |
| Unit tests | 3 | QA |
| Integration tests (with BIND container) | 4 | QA |
| Documentation + BIND setup guide | 3 | Docs |
| **Total** | **28** | |
| **With 20% buffer** | **28** | |

**Deliverables:**

- [ ] `backend/pkg/dnsprovider/custom/rfc2136.go`
- [ ] Caddy config generation for RFC 2136
- [ ] **Dockerfile modification:**

  ```dockerfile
  # Multi-stage build: Caddy with RFC 2136 module
  FROM caddy:2-builder AS caddy-builder
  RUN xcaddy build \
      --with github.com/caddy-dns/rfc2136

  # Copy custom Caddy binary to final image
  COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
  ```

- [ ] `frontend/src/components/RFC2136Form.tsx`
- [ ] Translation keys for RFC 2136 provider
- [ ] User guide: `docs/features/rfc2136-dns.md`
- [ ] BIND9 setup guide: `docs/guides/bind9-acme-setup.md`

### Phase 3: Webhook Plugin (Week 3)

| Task | Hours | Owner |
|------|-------|-------|
| WebhookProvider implementation | 5 | Backend |
| HTTP client with retry logic | 3 | Backend |
| Rate limiting + circuit breaker | 3 | Backend |
| SSRF validation (use existing) | 1 | Backend |
| Webhook form UI | 4 | Frontend |
| i18n translation keys | 1 | Frontend |
| Unit tests | 3 | QA |
| Integration tests (mock webhook server) | 3 | QA |
| Security tests (SSRF) | 2 | QA |
| Example webhook implementations | 2 | Docs |
| Documentation | 2 | Docs |
| **Total** | **30** | |
| **With 20% buffer** | **30** | |

**Deliverables:**

- [ ] `backend/pkg/dnsprovider/custom/webhook.go`
- [ ] `backend/internal/services/webhook_client.go`
- [ ] `frontend/src/components/WebhookForm.tsx`
- [ ] Translation keys for Webhook provider
- [ ] Example: `examples/webhook-server/nodejs/`
- [ ] Example: `examples/webhook-server/python/`
- [ ] User guide: `docs/features/webhook-dns.md`

### Phase 4: Script Plugin (Week 4, Optional)

| Task | Hours | Owner |
|------|-------|-------|
| ScriptProvider implementation | 4 | Backend |
| Secure execution sandbox | 4 | Backend |
| Security review | 3 | Security |
| Script form UI | 3 | Frontend |
| Unit tests | 3 | QA |
| Security tests | 4 | QA |
| Example scripts | 2 | Docs |
| Documentation | 2 | Docs |
| **Total** | **25** | |

**Deliverables:**

- [ ] `backend/pkg/dnsprovider/custom/script.go`
- [ ] `backend/internal/services/script_executor.go`
- [ ] `frontend/src/components/ScriptForm.tsx`
- [ ] Example: `examples/scripts/nsupdate.sh`
- [ ] Example: `examples/scripts/cloudns.sh`
- [ ] User guide: `docs/features/script-dns.md`
- [ ] Security guide: `docs/guides/script-plugin-security.md`

---

## 11. Testing Strategy

### 11.1 Unit Tests

Each provider requires tests for:

- Credential validation
- Config generation
- Error handling
- Timeout behavior

```go
// backend/pkg/dnsprovider/custom/webhook_test.go
func TestWebhookProvider_ValidateCredentials(t *testing.T) {
    tests := []struct {
        name    string
        creds   map[string]string
        wantErr bool
    }{
        {"valid with auth", map[string]string{"create_url": "https://...", "delete_url": "https://...", "auth_header": "X-Key", "auth_value": "secret"}, false},
        {"valid without auth", map[string]string{"create_url": "https://...", "delete_url": "https://..."}, false},
        {"missing create_url", map[string]string{"delete_url": "https://..."}, true},
        {"http not allowed", map[string]string{"create_url": "http://...", "delete_url": "http://..."}, true},
        {"internal IP blocked", map[string]string{"create_url": "https://192.168.1.1/dns", "delete_url": "https://192.168.1.1/dns"}, true},
    }
    // ...
}
```

### 11.2 Integration Tests

| Test Scenario | Components | Method |
|---------------|------------|--------|
| Manual challenge flow | Backend + Frontend | E2E with Playwright |
| RFC 2136 with BIND9 | Backend + BIND container | Docker Compose |
| Webhook with mock server | Backend + Mock HTTP | httptest |
| Script execution | Backend + Test scripts | Isolated container |

#### Manual Plugin E2E Scenarios (Playwright)

| Scenario | Description | Expected Result |
|----------|-------------|-----------------|
| Countdown timeout | User does not create DNS record | UI shows "Expired" after timeout, challenge marked expired |
| Copy buttons | User clicks "Copy" for record name/value | Values copied to clipboard, toast notification shown |
| DNS propagation success | User creates record, clicks "Verify" | After retries, status changes to "Verified" |
| DNS propagation failure | User creates wrong record | After max retries, shows "DNS record not found" |
| Cancel challenge | User clicks "Cancel Challenge" | Challenge marked as cancelled, UI returns to provider list |
| Refresh during challenge | User refreshes page during pending challenge | Challenge state persisted, countdown continues from correct time |

### 11.3 Additional Required Test Scenarios

#### Webhook Tests

| Scenario | Description | Expected Result |
|----------|-------------|----------------|
| Retry exhaustion | Webhook returns 500 for all 3 retry attempts | `WEBHOOK_TIMEOUT` error after final retry |
| Response too large | Webhook returns >1MB response | `WEBHOOK_RESPONSE_TOO_LARGE` error (413) |
| DNS rebinding | URL resolves to internal IP on second resolution | Request blocked, `SSRF_DETECTED` error |
| Idempotency replay | Same `request_id` sent twice | Second request returns cached response |

#### Circuit Breaker Tests

| Scenario | Description | Expected Result |
|----------|-------------|----------------|
| Open state transition | 5 consecutive failures | Circuit opens, `PROVIDER_CIRCUIT_OPEN` (503) |
| Half-open state | Wait 5 minutes after open | Next request allowed (test request) |
| Reset on success | Successful request in half-open | Circuit fully closes, counter resets |
| Stay open on failure | Failed request in half-open | Circuit remains open for another 5 minutes |

#### Script Tests

| Scenario | Description | Expected Result |
|----------|-------------|----------------|
| Timeout boundary (pass) | Script completes in 59 seconds | Success, output captured |
| Timeout boundary (fail) | Script runs for 61 seconds | `SCRIPT_TIMEOUT` error (504) |
| Invalid argument chars | Argument contains `; rm -rf /` | `INVALID_SCRIPT_ARGUMENT` error (400) |
| Symlink escape | Script path is symlink to `/etc/passwd` | `SCRIPT_PATH_INVALID` error (400) |
| Resource limit breach | Script tries to fork 100 processes | Script killed, resource limit error |

#### Manual Challenge Tests

| Scenario | Description | Expected Result |
|----------|-------------|----------------|
| Concurrent verify race | Two users verify same FQDN simultaneously | Only one succeeds, other gets `CHALLENGE_IN_PROGRESS` |
| CSRF token mismatch | POST without valid CSRF token | 403 Forbidden |
| Challenge ownership | User A tries to access User B's challenge | 403 Forbidden, audit log entry |
| Predictable ID attack | Attempt to enumerate challenge IDs | No information leakage, 404 for non-existent |

#### RFC 2136 Tests

| Scenario | Description | Expected Result |
|----------|-------------|----------------|
| Network timeout | DNS server unreachable | Timeout error with retry logic |
| Connection refused | DNS server port closed | `TSIG_AUTH_FAILED` or connection error |
| TSIG key mismatch | Wrong TSIG secret configured | `TSIG_AUTH_FAILED` (401) |
| Zone transfer denied | Server rejects update | Appropriate error message with zone info |

### 11.4 Security Tests

| Test | Tool | Target |
|------|------|--------|
| SSRF in webhook URLs | Custom test suite | WebhookProvider |
| Path traversal in scripts | Custom test suite | ScriptProvider |
| Credential leakage in logs | Log analysis | All providers |
| TSIG key handling | Memory dump analysis | RFC2136Provider |

### 11.5 Coverage Requirements

- Backend: ≥85% coverage
- Frontend: ≥85% coverage
- New provider code: ≥90% coverage

---

## 12. Documentation Requirements

### 12.1 User Documentation

| Document | Audience | Location |
|----------|----------|----------|
| Custom DNS Providers Overview | All users | `docs/features/custom-dns-providers.md` |
| Manual DNS Challenge Guide | Beginners | `docs/features/manual-dns-challenge.md` |
| RFC 2136 Setup Guide | Self-hosted DNS admins | `docs/features/rfc2136-dns.md` |
| Webhook Integration Guide | DevOps teams | `docs/features/webhook-dns.md` |
| Script Plugin Guide | Power users | `docs/features/script-dns.md` |

### 12.2 Technical Documentation

| Document | Audience | Location |
|----------|----------|----------|
| Custom Plugin Architecture | Contributors | `docs/development/custom-plugin-architecture.md` |
| Webhook API Specification | Integration devs | `docs/api/webhook-dns-api.md` |
| RFC 2136 Protocol Details | Network engineers | `docs/technical/rfc2136-implementation.md` |

### 12.3 Setup Guides

| Guide | Audience | Location |
|-------|----------|----------|
| BIND9 ACME Setup | Self-hosted users | `docs/guides/bind9-acme-setup.md` |
| PowerDNS ACME Setup | Self-hosted users | `docs/guides/powerdns-acme-setup.md` |
| Building Webhook Endpoints | Developers | `docs/guides/webhook-development.md` |

### 12.4 Operations and Security Documentation (Required)

The following documentation MUST be created as part of implementation:

| Document | Audience | Location | Priority |
|----------|----------|----------|----------|
| Custom DNS Plugin Troubleshooting | Support, Users | `docs/troubleshooting/custom-dns-plugins.md` | High |
| Custom DNS Security Hardening | Security, Admins | `docs/security/custom-dns-hardening.md` | High |
| Custom DNS Monitoring Guide | Operations | `docs/operations/custom-dns-monitoring.md` | Medium |

**Required Content for `docs/troubleshooting/custom-dns-plugins.md`:**

- Common error codes and resolutions
- Webhook debugging checklist
- Script execution troubleshooting
- RFC 2136 connection issues
- Manual challenge timeout scenarios
- Log analysis procedures

**Required Content for `docs/security/custom-dns-hardening.md`:**

- Webhook endpoint security best practices
- Script plugin security checklist
- TSIG key management procedures
- Network segmentation recommendations
- Audit logging configuration
- Incident response procedures

**Required Content for `docs/operations/custom-dns-monitoring.md`:**

- Key metrics to monitor (success rate, latency, errors)
- Alerting thresholds and recommendations
- Dashboard examples (Grafana/Prometheus)
- Capacity planning guidelines
- Runbook templates for common issues

---

## 13. Estimated Effort

### Summary by Phase

| Phase | Description | Hours | Hours (with 20% buffer) | Calendar |
|-------|-------------|-------|-------------------------|----------|
| 1 | Manual Plugin | 27 | 32 | 1 week |
| 2 | RFC 2136 Plugin | 23 | 28 | 1 week |
| 3 | Webhook Plugin | 25 | 30 | 1 week |
| **Total (Phases 1-3)** | **Core Features** | **75** | **90** | **3 weeks** |
| 4 | Script Plugin (Future) | 25 | 30 | 1 week |
| **Total (All Phases)** | **Including Future** | **100** | **120** | **4 weeks** |

**Note:** Phase 4 (Script Plugin) is conditional on community demand (>20 GitHub issues). See "Future Work" section.

### Effort by Role

| Role | Phase 1 | Phase 2 | Phase 3 | Phase 4* | Total |
|------|---------|---------|---------|----------|-------|
| Backend | 11h | 11h | 12h | 8h | 42h |
| Frontend | 8h | 5h | 5h | 3h | 21h |
| QA | 7h | 7h | 8h | 7h | 29h |
| Docs | 2h | 3h | 4h | 4h | 13h |
| DevOps | 0h | 2h | 0h | 0h | 2h |
| Security | 0h | 0h | 1h | 3h | 4h |

*Phase 4 effort is conditional

### MVP (Minimum Viable Product)

**MVP = Phase 1 (Manual Plugin)**

- Time: 32 hours / 1 week (with buffer)
- Unblocks: All users with unsupported DNS providers
- Risk: Low

---

## 14. Decisions and Open Questions

### Decisions Made

1. **Caddy Module Strategy for RFC 2136**

   **DECIDED: Option B — RFC 2136 module will be included in Charon's Caddy build.**

   Rationale: Best user experience. Users should not need to rebuild Caddy themselves. The Dockerfile will be updated in Phase 2 to use xcaddy with the `github.com/caddy-dns/rfc2136` module.

### Must Decide Before Implementation

1. **Script Plugin Security Model**
   - Should scripts run in a separate container/sandbox?
   - What environment variables should be available?
   - Should we allow network access from scripts?
   - **Recommendation:** No network by default, minimal env, document risks

2. **Manual Challenge Persistence**
   - Store challenge details in database or session?
   - How long to retain completed challenges?
   - **Recommendation:** Database with 24-hour TTL cleanup (see Section 6.4)

3. **Webhook Retry Strategy**
   - Exponential backoff vs. fixed interval?
   - Max retries before failure?
   - **Recommendation:** Exponential backoff (1s, 2s, 4s), max 3 retries

### Nice to Decide

1. **UI Location for Custom Plugins**
   - Same page as built-in providers?
   - Separate "Custom Integrations" section?
   - **Recommendation:** Same page, grouped by category

2. **Telemetry for Custom Plugins**
   - Should we track usage of custom plugin types?
   - Privacy considerations?
   - **Recommendation:** Opt-in anonymous usage stats

3. **Plugin Marketplace (Future)**
   - Community-contributed webhook templates?
   - Pre-configured RFC 2136 profiles?
   - **Recommendation:** Defer to Phase 5+

---

## 15. Appendix

### A. Related Documents

- [Phase 5 Custom Plugins Spec](phase5_custom_plugins_spec.md) - Go plugin architecture (external .so files)
- [DNS Challenge Backend Research](dns_challenge_backend_research.md) - Original DNS-01 implementation notes
- [DNS Challenge Future Features](dns_challenge_future_features.md) - Roadmap context

### B. External References

- [RFC 2136: Dynamic Updates in DNS](https://datatracker.ietf.org/doc/html/rfc2136)
- [RFC 2845: TSIG Authentication](https://datatracker.ietf.org/doc/html/rfc2845)
- [Caddy DNS Challenge Docs](https://caddyserver.com/docs/automatic-https#dns-challenge)
- [Let's Encrypt DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)

### C. Example Webhook Payload

```json
{
  "action": "create",
  "fqdn": "_acme-challenge.example.com",
  "domain": "example.com",
  "subdomain": "_acme-challenge",
  "value": "gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2aB3cD4eF5gH6iJ7kL",
  "ttl": 300,
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-08T15:30:00Z",
  "charon_version": "1.2.0",
  "certificate_domains": ["*.example.com", "example.com"]
}
```

### D. Example BIND9 TSIG Configuration

```zone
// /etc/bind/named.conf.local
key "acme-update-key" {
    algorithm hmac-sha256;
    secret "base64-encoded-secret-here==";
};

zone "example.com" {
    type master;
    file "/var/lib/bind/db.example.com";
    update-policy {
        grant acme-update-key name _acme-challenge.example.com. TXT;
    };
};
```

---

## 16. Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-08 | Planning Agent | Initial specification |
| 1.1 | 2026-01-08 | Planning Agent | Supervisor review: addressed 13 issues (see below) |
| 1.2 | 2026-01-11 | Planning Agent | Supervisor review: addressed 9 critical/high priority findings (see Section 18) |

---

## 17. Supervisor Review Summary

This specification was revised to address all 13 issues identified during Supervisor review:

### Critical Issues (Fixed)

| # | Issue | Resolution |
|---|-------|------------|
| 1 | SSRF Duplication | Section 9.2 updated to reference existing `security.ValidateExternalURL()` in `backend/internal/security/url_validator.go` |
| 2 | Script Security Insufficient | Section 9.3 enhanced with rlimit enforcement, seccomp documentation, minimal PATH, and `timeout` command |
| 3 | Missing Caddy Integration Detail | Added Section 3.3.1-3.3.4 with sequence diagram, state machine, error handling, and communication protocol |

### High Severity Issues (Fixed)

| # | Issue | Resolution |
|---|-------|------------|
| 4 | RFC 2136 Caddy Module | Section 4.3 updated with DECISION; Phase 2 includes Dockerfile deliverable |
| 5 | WebSocket vs Polling | Section 4.4 updated; chose polling (10s interval) with rationale; polling endpoint added to API |
| 6 | Webhook Rate Limiting | Section 4.1 updated with rate limits (10/min) and circuit breaker (5 failures → 5 min disable) |

### Medium Severity Issues (Fixed)

| # | Issue | Resolution |
|---|-------|------------|
| 7 | Phase 4 Scope Creep | Phase 4 moved to "Future Work" section with explicit Go/No-Go gate (>20 GitHub issues) |
| 8 | Missing Error Codes | Section 7.3 added with comprehensive error code table |
| 9 | Time Estimates Buffer | Section 13 updated: Phase 1→32h, Phase 2→28h, Phase 3→30h (all +20%) |
| 10 | Open Question #1 | Section 14 changed to "Decisions and Open Questions"; Option B confirmed as DECIDED |

### Low Severity Issues (Fixed)

| # | Issue | Resolution |
|---|-------|------------|
| 11 | i18n Keys | Phase 1 deliverables updated with translation keys for `frontend/src/locales/*/translation.json` |
| 12 | E2E Test Scenarios | Section 11.2 expanded with Manual Plugin E2E scenarios table |
| 13 | Cleanup Mechanism | Section 6.4 added with cron-based cleanup using existing `robfig/cron/v3` pattern |

---

*This document has completed Supervisor review and is ready for technical review and stakeholder approval.*

---

## 18. Supervisor Review Summary (v1.2)

This specification was revised on January 11, 2026 to address 9 critical/high priority findings:

### Security Enhancements

| # | Finding | Resolution |
|---|---------|------------|
| 1 | Missing concurrent challenge handling | Section 3.3.5 added with database locking (`SELECT ... FOR UPDATE`), queueing behavior, and `CHALLENGE_IN_PROGRESS` error |
| 2 | Webhook DNS rebinding vulnerability | Section 4.1 updated: URLs validated at both configuration AND execution time |
| 3 | Missing webhook response size limit | Section 4.1 updated: `MaxWebhookResponseSize = 1MB`, new error code added |
| 4 | Missing webhook TLS skip option | Section 4.1 updated: `insecure_skip_verify` config with prominent warning |
| 5 | Webhook idempotency missing | Section 4.1 updated: `request_id` requirement for deduplication |
| 6 | Script argument sanitization weak | Section 4.2 updated: strict `[a-zA-Z0-9._=-]` pattern, new error code |
| 7 | Symlink escape vulnerability | Section 4.2 updated: `filepath.EvalSymlinks()` MUST be called before prefix check |
| 8 | Resource limits optional | Section 4.2 updated: rlimits now MANDATORY with specific values |
| 9 | Environment variable leakage | Section 4.2 updated: explicit environment clearing before script execution |
| 10 | RFC 2136 hmac-md5 insecure | Section 4.3 updated: `hmac-md5` marked DEPRECATED with removal warning |
| 11 | TSIG secret memory exposure | Section 4.3 updated: secure memory handling with memguard pattern |
| 12 | Manual challenge session binding missing | Section 4.4 updated: challenge-user binding, CSRF validation, UUIDv4 IDs |
| 13 | Log credential exposure | Section 9.4 added: comprehensive redaction patterns for 7 sensitive fields |

### Error Codes Added (Section 7.3)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `CHALLENGE_IN_PROGRESS` | 409 | Another challenge active for FQDN |
| `WEBHOOK_RESPONSE_TOO_LARGE` | 413 | Response exceeded 1MB limit |
| `INVALID_SCRIPT_ARGUMENT` | 400 | Invalid characters in script argument |

### Testing Scenarios Added (Section 11.3)

- Webhook retry exhaustion tests
- Circuit breaker state transition tests
- Script timeout boundary tests (59s pass, 61s fail)
- Manual challenge concurrent verify race condition test
- RFC 2136 network error tests

### Documentation Requirements Added (Section 12.4)

- `docs/troubleshooting/custom-dns-plugins.md`
- `docs/security/custom-dns-hardening.md`
- `docs/operations/custom-dns-monitoring.md`

---

*This document has been updated to address all supervisor review findings from January 11, 2026.*
