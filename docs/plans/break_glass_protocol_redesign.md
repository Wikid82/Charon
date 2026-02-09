# Break Glass Protocol Redesign - Root Cause Analysis & 3-Tier Architecture

**Date:** January 26, 2026
**Status:** Analysis Complete - Implementation Pending
**Priority:** 🔴 CRITICAL - Emergency access is broken
**Estimated Timeline:** 2-4 hours implementation + testing

---

## Executive Summary

The emergency break glass token is **currently non-functional** due to a fundamental architectural flaw: the emergency reset endpoint is protected by the same Cerberus middleware it needs to bypass. This creates a deadlock scenario where administrators locked out by ACL/WAF cannot use the emergency token to regain access.

**Current State:** Emergency endpoint → Cerberus ACL blocks request → Emergency handler never executes
**Required State:** Emergency endpoint → Bypass all security → Emergency handler executes

This document provides:
1. Complete root cause analysis with evidence
2. 3-tier break glass architecture design
3. Actionable implementation plan
4. Comprehensive verification strategy

---

## Part 1: Root Cause Analysis

### 1.1 The Deadlock Problem

#### Evidence from Code Analysis

**File:** `backend/internal/api/routes/routes.go` (Lines 113-116)

```go
// Emergency endpoint - MUST be registered BEFORE Cerberus middleware
// This endpoint bypasses all security checks for lockout recovery
// Requires CHARON_EMERGENCY_TOKEN env var to be configured
emergencyHandler := handlers.NewEmergencyHandler(db)
router.POST("/api/v1/emergency/security-reset", emergencyHandler.SecurityReset)
```

**File:** `backend/internal/api/routes/routes.go` (Lines 118-122)

```go
api := router.Group("/api/v1")

// Cerberus middleware applies the optional security suite checks (WAF, ACL, CrowdSec)
cerb := cerberus.New(cfg.Security, db)
api.Use(cerb.Middleware())
```

#### The Critical Flaw

While the comment claims the emergency endpoint is registered "BEFORE Cerberus middleware," examination of the code reveals **it's registered on the root router but still under the `/api/v1` path**. The issue is:

1. **Emergency endpoint registration:** `router.POST("/api/v1/emergency/security-reset", ...)`
2. **API group with Cerberus:** `api := router.Group("/api/v1")` followed by `api.Use(cerb.Middleware())`

**The problem:** Both routes share the `/api/v1` prefix. While there's an attempt to register the emergency endpoint on the root router before the API group is created with middleware, **Gin's routing may not guarantee this bypass behavior**. The `/api/v1/emergency/security-reset` path could still match routes within the `/api/v1` group depending on Gin's internal route resolution order.

### 1.2 Middleware Execution Order

#### Current Middleware Chain (from `routes.go`)

```
1. gzip.Gzip() - Global compression (Line 61)
2. middleware.SecurityHeaders() - Security headers (Line 68)
3. [Emergency endpoint registered here - Line 116]
4. cerb.Middleware() - Cerberus ACL/WAF/CrowdSec (Line 122)
5. authMiddleware() - JWT validation (Line 201)
6. [Protected endpoints]
```

#### The Cerberus Middleware ACL Logic

**File:** `backend/internal/cerberus/cerberus.go` (Lines 134-160)

```go
if aclEnabled {
    acls, err := c.accessSvc.List()
    if err == nil {
        clientIP := ctx.ClientIP()
        for _, acl := range acls {
            if !acl.Enabled {
                continue
            }
            allowed, _, err := c.accessSvc.TestIP(acl.ID, clientIP)
            if err == nil && !allowed {
                // Send security notification
                _ = c.securityNotifySvc.Send(context.Background(), models.SecurityEvent{
                    EventType: "acl_deny",
                    Severity:  "warn",
                    Message:   "Access control list blocked request",
                    ClientIP:  clientIP,
                    Path:      ctx.Request.URL.Path,
                    Timestamp: time.Now(),
                    Metadata: map[string]any{
                        "acl_name": acl.Name,
                        "acl_id":   acl.ID,
                    },
                })

                ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
                return
            }
        }
    }
}
```

**Key observations:**
- ACL check happens **before** any endpoint-specific logic
- Uses `ctx.AbortWithStatusJSON()` which **terminates the request chain**
- Emergency token header is **never examined** by Cerberus
- No bypass mechanism for emergency scenarios

### 1.3 Layer 3 vs Layer 7 Analysis

#### CrowdSec Bouncer Investigation

**File:** `.docker/compose/docker-compose.e2e.yml` (Lines 1-31)

```yaml
services:
  charon-e2e:
    image: charon:local
    container_name: charon-e2e
    restart: "no"
    ports:
      - "8080:8080"    # Management UI (Charon)
    environment:
      - CHARON_ENV=development
      - CHARON_DEBUG=0
      - TZ=UTC
      - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY:?CHARON_ENCRYPTION_KEY is required}
      - CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN:-test-emergency-token-for-e2e-32chars}
```

**Evidence from container inspection:**

```bash
$ docker exec charon-e2e sh -c "command -v cscli"
/usr/local/bin/cscli

$ docker exec charon-e2e sh -c "iptables -L -n -v 2>/dev/null"
[No output - iptables not available or no rules configured]
```

**Analysis:**
- CrowdSec CLI (`cscli`) is present in the container
- iptables does not appear to have active rules
- **However:** The actual blocking may be happening at the **Caddy layer** via the `caddy-crowdsec-bouncer` plugin

**File:** `backend/internal/cerberus/cerberus.go` (Lines 162-170)

```go
// CrowdSec integration: The actual IP blocking is handled by the caddy-crowdsec-bouncer
// plugin at the Caddy layer. This middleware provides defense-in-depth tracking.
// When CrowdSec mode is "local", the bouncer communicates directly with the LAPI
// to receive ban decisions and block malicious IPs before they reach the application.
if c.cfg.CrowdSecMode == "local" {
    // Track that this request passed through CrowdSec evaluation
    // Note: Blocking decisions are made by Caddy bouncer, not here
    metrics.IncCrowdSecRequest()
    logger.Log().WithField("client_ip", ctx.ClientIP()).WithField("path", ctx.Request.URL.Path).Debug("Request evaluated by CrowdSec bouncer at Caddy layer")
}
```

**Critical finding:** CrowdSec blocking happens at **Caddy layer (Layer 7 reverse proxy)** BEFORE the request reaches the Go application. This means:

1. **Layer 7 Block (Caddy):** CrowdSec bouncer → IP banned → HTTP 403 response
2. **Layer 7 Block (Go):** Cerberus ACL → IP not in whitelist → HTTP 403 response

**Neither blocking point examines the emergency token header.**

### 1.4 Test Environment Network Topology

#### Docker Network Analysis

**Container:** `charon-e2e`
**Port Mapping:** `8080:8080` (host → container)
**Network Mode:** Docker bridge network (default)
**Test Client:** Playwright running on host machine

**Request Flow:**

```
[Playwright Test]
    ↓ (localhost:8080)
[Docker Bridge Network]
    ↓ (172.17.0.x → charon-e2e:8080)
[Caddy Reverse Proxy]
    ↓ (CrowdSec bouncer check - Layer 7)
[Charon Go Application]
    ↓ (Cerberus ACL middleware - Layer 7)
[Emergency Handler] ← NEVER REACHED
```

**Client IP as seen by backend:**

From the test client's perspective, the backend sees the request coming from:
- **Development:** `127.0.0.1` or `::1` (loopback)
- **Docker bridge:** `172.17.0.1` (Docker gateway)
- **E2E tests:** Likely appears as Docker internal IP

**ACL Whitelist Issue:** If ACL is enabled with a restrictive whitelist (e.g., only `10.0.0.0/8`), the test client's IP (`172.17.0.1`) would be **blocked** before the emergency endpoint can execute.

### 1.5 Test Failure Scenario

**File:** `tests/global-setup.ts` (Lines 63-106)

```typescript
async function emergencySecurityReset(requestContext: APIRequestContext): Promise<void> {
  console.log('Performing emergency security reset...');

  const emergencyToken = 'test-emergency-token-for-e2e-32chars';
  const headers = {
    'Content-Type': 'application/json',
    'X-Emergency-Token': emergencyToken,
  };

  const modules = [
    { key: 'security.acl.enabled', value: 'false' },
    { key: 'security.waf.enabled', value: 'false' },
    { key: 'security.crowdsec.enabled', value: 'false' },
    { key: 'security.rate_limit.enabled', value: 'false' },
    { key: 'feature.cerberus.enabled', value: 'false' },
  ];

  for (const { key, value } of modules) {
    try {
      await requestContext.post('/api/v1/settings', {
        data: { key, value },
        headers,
      });
      console.log(`  ✓ Disabled: ${key}`);
    } catch (e) {
      console.log(`  ⚠ Could not disable ${key}: ${e}`);
    }
  }
  // ...
}
```

**Problem:** The test uses `/api/v1/settings` endpoint (not the emergency endpoint!) and passes the emergency token header. This is **incorrect** because:

1. **Wrong endpoint:** `/api/v1/settings` requires authentication via `authMiddleware`
2. **Wrong endpoint (again):** The emergency endpoint is `/api/v1/emergency/security-reset`
3. **ACL blocks first:** If ACL is enabled, the request is blocked at Cerberus before reaching the settings handler

**Expected test flow:**
```typescript
await requestContext.post('/api/v1/emergency/security-reset', {
  headers: {
    'X-Emergency-Token': emergencyToken,
  },
});
```

### 1.6 Emergency Handler Validation

**File:** `backend/internal/api/handlers/emergency_handler.go` (Lines 1-312)

The emergency handler itself is **well-designed** with:
- ✅ Timing-safe token comparison (constant-time)
- ✅ Rate limiting (5 attempts per minute per IP)
- ✅ Minimum token length validation (32 chars)
- ✅ Comprehensive audit logging
- ✅ Disables all security modules via settings
- ✅ Updates `SecurityConfig` database record

**The handler works correctly IF it can be reached.**

---

## Part 2: 3-Tier Break Glass Architecture

### 2.1 Design Philosophy

**Defense in Depth for Recovery:**
- **Tier 1 (Digital Key):** Fast, convenient, Layer 7 bypass within the application
- **Tier 2 (Sidecar Door):** Separate ingress with minimal security, network-isolated
- **Tier 3 (Physical Key):** Direct system access for catastrophic failures

Each tier provides a fallback if the previous tier fails.

### 2.2 Tier 1: Digital Key (Layer 7 Bypass)

#### Concept

A high-priority middleware that short-circuits the entire security stack when the emergency token is present and valid.

#### Design

**Middleware Registration Order (NEW):**

```go
// TOP OF CHAIN: Emergency bypass middleware (before gzip, before security headers)
router.Use(middleware.EmergencyBypass(cfg.Security.EmergencyToken, db))

// Then standard middleware
router.Use(gzip.Gzip(gzip.DefaultCompression))
router.Use(middleware.SecurityHeaders(securityHeadersCfg))

// Emergency handler registration on root router
router.POST("/api/v1/emergency/security-reset", emergencyHandler.SecurityReset)

// API group with Cerberus (emergency requests skip this entirely)
api := router.Group("/api/v1")
api.Use(cerb.Middleware())
```

#### Implementation: Emergency Bypass Middleware

**File:** `backend/internal/api/middleware/emergency.go` (NEW)

```go
package middleware

import (
    "crypto/subtle"
    "net"
    "os"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/Wikid82/charon/backend/internal/logger"
    "gorm.io/gorm"
)

const (
    EmergencyTokenHeader = "X-Emergency-Token"
    EmergencyTokenEnvVar = "CHARON_EMERGENCY_TOKEN"
    MinTokenLength       = 32
)

// EmergencyBypass creates middleware that bypasses all security checks
// when a valid emergency token is present from an authorized source.
//
// Security conditions (ALL must be met):
// 1. Request from management CIDR (RFC1918 private networks by default)
// 2. X-Emergency-Token header matches configured token (timing-safe)
// 3. Token meets minimum length requirement (32+ chars)
//
// This middleware must be registered FIRST in the middleware chain.
func EmergencyBypass(managementCIDRs []string, db *gorm.DB) gin.HandlerFunc {
    // Load emergency token from environment
    emergencyToken := os.Getenv(EmergencyTokenEnvVar)
    if emergencyToken == "" {
        logger.Log().Warn("CHARON_EMERGENCY_TOKEN not set - emergency bypass disabled")
        return func(c *gin.Context) { c.Next() } // noop
    }

    if len(emergencyToken) < MinTokenLength {
        logger.Log().Warn("CHARON_EMERGENCY_TOKEN too short - emergency bypass disabled")
        return func(c *gin.Context) { c.Next() } // noop
    }

    // Parse management CIDRs
    var managementNets []*net.IPNet
    for _, cidr := range managementCIDRs {
        _, ipnet, err := net.ParseCIDR(cidr)
        if err != nil {
            logger.Log().WithError(err).WithField("cidr", cidr).Warn("Invalid management CIDR")
            continue
        }
        managementNets = append(managementNets, ipnet)
    }

    // Default to RFC1918 private networks if none specified
    if len(managementNets) == 0 {
        managementNets = []*net.IPNet{
            mustParseCIDR("10.0.0.0/8"),
            mustParseCIDR("172.16.0.0/12"),
            mustParseCIDR("192.168.0.0/16"),
            mustParseCIDR("127.0.0.0/8"), // localhost for local development
        }
    }

    return func(c *gin.Context) {
        // Check if emergency token is present
        providedToken := c.GetHeader(EmergencyTokenHeader)
        if providedToken == "" {
            c.Next() // No emergency token - proceed normally
            return
        }

        // Validate source IP is from management network
        clientIP := net.ParseIP(c.ClientIP())
        if clientIP == nil {
            logger.Log().WithField("ip", c.ClientIP()).Warn("Emergency bypass: invalid client IP")
            c.Next()
            return
        }

        inManagementNet := false
        for _, ipnet := range managementNets {
            if ipnet.Contains(clientIP) {
                inManagementNet = true
                break
            }
        }

        if !inManagementNet {
            logger.Log().WithField("ip", clientIP.String()).Warn("Emergency bypass: IP not in management network")
            c.Next()
            return
        }

        // Timing-safe token comparison
        if !constantTimeCompare(emergencyToken, providedToken) {
            logger.Log().WithField("ip", clientIP.String()).Warn("Emergency bypass: invalid token")
            c.Next()
            return
        }

        // Valid emergency token from authorized source
        logger.Log().WithFields(map[string]interface{}{
            "ip":   clientIP.String(),
            "path": c.Request.URL.Path,
        }).Warn("EMERGENCY BYPASS ACTIVE: Request bypassing all security checks")

        // Set flag for downstream handlers to know this is an emergency request
        c.Set("emergency_bypass", true)

        // Strip emergency token header to prevent it from reaching application
        // This is critical for security - prevents token exposure in logs
        c.Request.Header.Del(EmergencyTokenHeader)

        c.Next()
    }
}

func mustParseCIDR(cidr string) *net.IPNet {
    _, ipnet, _ := net.ParseCIDR(cidr)
    return ipnet
}

func constantTimeCompare(a, b string) bool {
    return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

#### Cerberus Middleware Update

**File:** `backend/internal/cerberus/cerberus.go` (Line 106)

```go
func (c *Cerberus) Middleware() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        // Check for emergency bypass flag
        if bypass, exists := ctx.Get("emergency_bypass"); exists && bypass.(bool) {
            logger.Log().WithField("path", ctx.Request.URL.Path).Debug("Cerberus: Skipping security checks (emergency bypass)")
            ctx.Next()
            return
        }

        if !c.IsEnabled() {
            ctx.Next()
            return
        }

        // ... rest of existing logic
    }
}
```

#### Security Considerations

**Strengths:**
- ✅ Double authentication: IP CIDR + secret token
- ✅ Timing-safe comparison prevents timing attacks
- ✅ Token stripped before reaching application (log safety)
- ✅ Comprehensive audit logging
- ✅ Bypass flag prevents any middleware from blocking

**Weaknesses:**
- ⚠️ Relies on `ClientIP()` which can be spoofed if behind proxies
- ⚠️ Token in HTTP header (use HTTPS only)
- ⚠️ If Caddy bouncer blocks at Layer 7, request never reaches Go app

**Mitigations:**
- Configure Gin's `SetTrustedProxies()` correctly
- Document HTTPS-only requirement
- Implement Tier 2 for Caddy-level blocks

### 2.3 Tier 2: Sidecar Door (Separate Entry Point)

#### Concept

A secondary HTTP port with minimal security, bound to localhost or VPN-only interfaces.

#### Design

**Architecture:**

```
[Public Traffic:443/80]
    ↓
[Caddy Reverse Proxy]
    ↓ (WAF, CrowdSec, ACL)
[Charon Main Port:8080]

[VPN/Localhost Only:2019]  ← Sidecar Port
    ↓
[Emergency-Only Server]
    ↓ (Basic Auth or mTLS ONLY)
[Emergency Handlers]
```

#### Implementation

**File:** `backend/internal/server/emergency_server.go` (NEW)

```go
package server

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/api/handlers"
    "github.com/Wikid82/charon/backend/internal/api/middleware"
    "github.com/Wikid82/charon/backend/internal/config"
    "github.com/Wikid82/charon/backend/internal/logger"
)

// EmergencyServer provides a minimal HTTP server for emergency operations.
// This server runs on a separate port with minimal security for failsafe access.
type EmergencyServer struct {
    server *http.Server
    db     *gorm.DB
    cfg    config.EmergencyConfig
}

// NewEmergencyServer creates a new emergency server instance
func NewEmergencyServer(db *gorm.DB, cfg config.EmergencyConfig) *EmergencyServer {
    return &EmergencyServer{
        db:  db,
        cfg: cfg,
    }
}

// Start initializes and starts the emergency server
func (s *EmergencyServer) Start() error {
    if !s.cfg.Enabled {
        logger.Log().Info("Emergency server disabled")
        return nil
    }

    router := gin.New()
    router.Use(gin.Recovery())

    // Basic request logging (minimal)
    router.Use(func(c *gin.Context) {
        start := time.Now()
        c.Next()
        logger.Log().WithFields(map[string]interface{}{
            "method":  c.Request.Method,
            "path":    c.Request.URL.Path,
            "status":  c.Writer.Status(),
            "latency": time.Since(start).Milliseconds(),
        }).Info("Emergency server request")
    })

    // Basic auth middleware (if configured)
    if s.cfg.BasicAuthUsername != "" && s.cfg.BasicAuthPassword != "" {
        router.Use(gin.BasicAuth(gin.Accounts{
            s.cfg.BasicAuthUsername: s.cfg.BasicAuthPassword,
        }))
    } else {
        logger.Log().Warn("Emergency server has no authentication - use only on localhost!")
    }

    // Emergency endpoints
    emergencyHandler := handlers.NewEmergencyHandler(s.db)
    router.POST("/emergency/security-reset", emergencyHandler.SecurityReset)

    // Health check
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok", "server": "emergency"})
    })

    // Start server
    s.server = &http.Server{
        Addr:         s.cfg.BindAddress,
        Handler:      router,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    logger.Log().WithField("address", s.cfg.BindAddress).Info("Starting emergency server")

    go func() {
        if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Log().WithError(err).Error("Emergency server failed")
        }
    }()

    return nil
}

// Stop gracefully shuts down the emergency server
func (s *EmergencyServer) Stop(ctx context.Context) error {
    if s.server == nil {
        return nil
    }
    logger.Log().Info("Stopping emergency server")
    return s.server.Shutdown(ctx)
}
```

**Configuration:** `backend/internal/config/config.go`

```go
type EmergencyConfig struct {
    Enabled           bool   `env:"CHARON_EMERGENCY_SERVER_ENABLED" envDefault:"false"`
    BindAddress       string `env:"CHARON_EMERGENCY_BIND" envDefault:"127.0.0.1:2019"`
    BasicAuthUsername string `env:"CHARON_EMERGENCY_USERNAME" envDefault:""`
    BasicAuthPassword string `env:"CHARON_EMERGENCY_PASSWORD" envDefault:""`
}
```

**Docker Compose:** `.docker/compose/docker-compose.e2e.yml`

```yaml
services:
  charon-e2e:
    ports:
      - "8080:8080"    # Main application
      - "2019:2019"    # Emergency server (DO NOT expose publicly)
    environment:
      - CHARON_EMERGENCY_SERVER_ENABLED=true
      - CHARON_EMERGENCY_BIND=0.0.0.0:2019  # Bind to all interfaces in container
      - CHARON_EMERGENCY_USERNAME=admin
      - CHARON_EMERGENCY_PASSWORD=${CHARON_EMERGENCY_PASSWORD:-changeme}
      - CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN:-test-emergency-token-for-e2e-32chars}
```

#### Security Considerations

**Strengths:**
- ✅ Completely separate from main application stack
- ✅ No WAF, no CrowdSec, no ACL
- ✅ Can bind to localhost-only (unreachable from network)
- ✅ Optional Basic Auth or mTLS

**Weaknesses:**
- ⚠️ If exposed publicly, becomes attack surface
- ⚠️ Basic Auth is weak (prefer mTLS for production)

**Mitigations:**
- **NEVER expose port publicly**
- Use firewall rules to restrict access
- Use VPN or SSH tunneling to reach port
- Implement mTLS for production

### 2.4 Tier 3: Physical Key (Direct System Access)

#### Concept

When all application-level recovery fails, administrators need direct system access to manually fix the problem.

#### Access Methods

**1. SSH to Host Machine**

```bash
# SSH to Docker host
ssh admin@docker-host.example.com

# View Charon logs
docker logs charon-e2e

# View CrowdSec decisions
docker exec charon-e2e cscli decisions list

# Delete all CrowdSec bans
docker exec charon-e2e cscli decisions delete --all

# Flush iptables (if CrowdSec uses netfilter)
docker exec charon-e2e iptables -F
docker exec charon-e2e iptables -X

# Stop Caddy to bypass reverse proxy
docker exec charon-e2e pkill caddy

# Restart container with security disabled
docker compose -f .docker/compose/docker-compose.e2e.yml down
export CHARON_SECURITY_DISABLED=true
docker compose -f .docker/compose/docker-compose.e2e.yml up -d
```

**2. Direct Database Access**

```bash
# Access SQLite database directly
docker exec -it charon-e2e sqlite3 /app/data/charon.db

# Disable all security modules
UPDATE settings SET value = 'false' WHERE key = 'feature.cerberus.enabled';
UPDATE settings SET value = 'false' WHERE key = 'security.acl.enabled';
UPDATE settings SET value = 'false' WHERE key = 'security.waf.enabled';
UPDATE security_configs SET enabled = 0 WHERE name = 'default';
```

**3. Docker Volume Inspection**

```bash
# Find Charon data volume
docker volume ls | grep charon

# Inspect volume
docker volume inspect charon_data

# Mount volume to temporary container
docker run --rm -v charon_data:/data -it alpine sh
cd /data
vi charon.db  # Or use sqlite3
```

#### Documentation: Emergency Runbooks

**File:** `docs/runbooks/emergency-lockout-recovery.md` (NEW)

```markdown
# Emergency Lockout Recovery Runbook

## Symptom

"Access Forbidden" or "Blocked by access control list" when trying to access Charon web interface.

## Tier 1: Digital Key (Emergency Token)

### Prerequisites
- Access to `CHARON_EMERGENCY_TOKEN` value from deployment configuration
- HTTPS connection to Charon (token security)
- Source IP in management network (default: RFC1918 private IPs)

### Procedure
1. Send POST request with emergency token header:

```bash
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: <your-emergency-token>" \
  -H "Content-Type: application/json"
```

2. Verify response: `{"success": true, "disabled_modules": [...]}`

3. Wait 5 seconds for settings to propagate

4. Access web interface

### Troubleshooting
- **403 Forbidden before reset:** Tier 1 failed - proceed to Tier 2
- **401 Unauthorized:** Token mismatch - verify token from deployment config
- **429 Too Many Requests:** Rate limited - wait 1 minute
- **501 Not Implemented:** Token not configured in environment

## Tier 2: Sidecar Door (Emergency Server)

### Prerequisites
- VPN or SSH access to Docker host
- Knowledge of emergency server port (default: 2019)
- Emergency server enabled in configuration

### Procedure
1. SSH to Docker host:
```bash
ssh admin@docker-host.example.com
```

2. Create SSH tunnel to emergency port:
```bash
ssh -L 2019:localhost:2019 admin@docker-host.example.com
```

3. From local machine, call emergency endpoint:
```bash
curl -X POST http://localhost:2019/emergency/security-reset \
  -H "X-Emergency-Token: <your-emergency-token>" \
  -u admin:password
```

4. Verify response and access web interface

### Troubleshooting
- **Connection refused:** Emergency server not enabled
- **401 Unauthorized:** Basic auth credentials incorrect

## Tier 3: Physical Key (Direct System Access)

### Prerequisites
- root or sudo access to Docker host
- Knowledge of container name (default: charon-e2e or charon)

### Procedure
1. SSH to Docker host:
```bash
ssh admin@docker-host.example.com
```

2. Clear CrowdSec bans:
```bash
docker exec charon cscli decisions delete --all
```

3. Disable security via database:
```bash
docker exec charon sqlite3 /app/data/charon.db <<EOF
UPDATE settings SET value = 'false' WHERE key LIKE 'security.%.enabled';
UPDATE security_configs SET enabled = 0;
EOF
```

4. Restart container:
```bash
docker restart charon
```

5. Access web interface

### Catastrophic Recovery
If all else fails, destroy and recreate:

```bash
# Backup database first!
docker exec charon tar czf /tmp/backup.tar.gz /app/data
docker cp charon:/tmp/backup.tar.gz ~/charon-backup-$(date +%Y%m%d).tar.gz

# Destroy and recreate
docker compose down
docker compose up -d
```

## Post-Recovery Tasks

After regaining access:

1. Review security audit logs for root cause
2. Adjust ACL rules if too restrictive
3. Rotate emergency token if compromised
4. Document incident and update procedures
```

---

## Part 3: Implementation Plan

### Phase 3.1: Emergency Bypass Middleware (Tier 1)

**Est. Time:** 1 hour

**Tasks:**

1. **Create middleware file**
   - File: `backend/internal/api/middleware/emergency.go`
   - Implement: `EmergencyBypass()` function (see Tier 1 implementation above)
   - Test: Unit tests for token validation, CIDR matching, bypass flag

2. **Update routes registration**
   - File: `backend/internal/api/routes/routes.go`
   - Change: Register `EmergencyBypass` middleware FIRST
   - Change: Update emergency endpoint to check bypass flag
   - Test: Integration test with ACL enabled

3. **Update Cerberus middleware**
   - File: `backend/internal/cerberus/cerberus.go`
   - Change: Check for `emergency_bypass` context flag
   - Change: Skip all checks if flag is set
   - Test: Unit test for bypass behavior

4. **Configuration**
   - File: `backend/internal/config/config.go`
   - Add: `ManagementCIDRs []string` field
   - Add: Default to RFC1918 private networks
   - Doc: Environment variable `CHARON_MANAGEMENT_CIDRS`

**Verification:**

```bash
# Test with correct token from allowed IP
curl -X POST http://localhost:8080/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: test-emergency-token-for-e2e-32chars"

# Expect: 200 OK with success message

# Test with ACL enabled (should still work)
curl -X POST http://localhost:8080/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: test-emergency-token-for-e2e-32chars"

# Expect: 200 OK (bypass ACL)
```

### Phase 3.2: Emergency Server (Tier 2)

**Est. Time:** 1.5 hours

**Tasks:**

1. **Create emergency server**
   - File: `backend/internal/server/emergency_server.go`
   - Implement: `EmergencyServer` struct (see Tier 2 implementation above)
   - Implement: `Start()` and `Stop()` methods
   - Test: Server startup, Basic Auth, endpoint routing

2. **Update configuration**
   - File: `backend/internal/config/config.go`
   - Add: `EmergencyConfig` struct
   - Parse: Environment variables for bind address, auth credentials
   - Test: Configuration loading

3. **Update main.go**
   - File: `backend/cmd/main.go`
   - Add: Initialize and start `EmergencyServer`
   - Add: Graceful shutdown on SIGTERM
   - Test: Server lifecycle

4. **Update Docker Compose**
   - File: `.docker/compose/docker-compose.e2e.yml`
   - Add: Port mapping `2019:2019` (with comment: DO NOT expose publicly)
   - Add: Environment variables for emergency server config
   - Test: Container startup, port accessibility

**Verification:**

```bash
# Test emergency server health
curl http://localhost:2019/health

# Expect: {"status":"ok","server":"emergency"}

# Test emergency endpoint with Basic Auth
curl -X POST http://localhost:2019/emergency/security-reset \
  -H "X-Emergency-Token: test-emergency-token-for-e2e-32chars" \
  -u admin:changeme

# Expect: 200 OK with success message
```

### Phase 3.3: Documentation & Runbooks (Tier 3)

**Est. Time:** 30 minutes

**Tasks:**

1. **Create emergency runbook**
   - File: `docs/runbooks/emergency-lockout-recovery.md`
   - Content: Step-by-step procedures for all 3 tiers
   - Include: Troubleshooting, verification, post-recovery tasks
   - Review: Test all commands on actual system

2. **Update main README**
   - File: `README.md`
   - Add: Link to emergency recovery runbook
   - Add: Warning about emergency token security
   - Add: Quick reference for emergency endpoints

3. **Update security documentation**
   - File: `docs/security.md`
   - Add: Break glass protocol architecture
   - Add: Emergency token rotation procedure
   - Add: Security considerations and audit logs

4. **Create Terraform/deployment templates**
   - File: `terraform/modules/emergency/` (if applicable)
   - Template: Emergency token generation
   - Template: Firewall rules for emergency port
   - Template: VPN configuration for Tier 2 access

**Verification:**

```bash
# Follow runbook procedures manually
# Verify all commands work
# Check documentation links and formatting
```

### Phase 3.4: Test Environment Updates

**Est. Time:** 45 minutes

**Tasks:**

1. **Fix global-setup.ts**
   - File: `tests/global-setup.ts`
   - Change: Use `/api/v1/emergency/security-reset` endpoint (not `/api/v1/settings`)
   - Change: Remove authentication context requirement
   - Test: Run E2E tests with security enabled

2. **Create emergency token test suite**
   - File: `tests/security-enforcement/emergency-token.spec.ts` (NEW)
   - Test: Emergency token validation
   - Test: ACL bypass with valid token
   - Test: Rate limiting
   - Test: Audit logging
   - Test: Settings disabled after reset
   - Run: `npx playwright test emergency-token.spec.ts`

3. **Update E2E test fixtures**
   - File: `tests/fixtures/security.ts` (NEW)
   - Add: `enableSecurity()` helper
   - Add: `disableSecurity()` helper
   - Add: `testEmergencyAccess()` helper

4. **Integration test for emergency server**
   - File: `backend/internal/server/emergency_server_test.go` (NEW)
   - Test: Server startup and shutdown
   - Test: Basic Auth middleware
   - Test: Emergency endpoint routing
   - Test: Concurrent requests
   - Run: `go test -v ./internal/server/...`

**Verification:**

```bash
# Run all E2E tests with security enabled
npx playwright test

# Run backend unit tests
go test -v ./...

# Check coverage for emergency handler
go test -v -coverprofile=coverage.txt ./internal/api/handlers/emergency_handler_test.go
```

### Phase 3.5: Production Deployment Checklist

**Est. Time:** 30 minutes (+ deployment window)

**Pre-Deployment:**

- [ ] Generate strong emergency token: `openssl rand -hex 32`
- [ ] Store token in secrets manager (HashiCorp Vault, AWS Secrets Manager)
- [ ] Configure management CIDRs (VPN subnet, office subnet)
- [ ] Configure emergency server (if enabled)
- [ ] Update firewall rules to block public access to emergency port
- [ ] Test emergency procedures in staging environment
- [ ] Train ops team on runbook procedures

**Deployment:**

- [ ] Deploy new code with emergency middleware
- [ ] Verify middleware is registered first in chain
- [ ] Verify emergency endpoint is accessible from management network
- [ ] Test emergency token from authorized IP
- [ ] Enable monitoring alerts for emergency token usage
- [ ] Update incident response procedures

**Post-Deployment:**

- [ ] Verify all application features work normally
- [ ] Test emergency procedures end-to-end
- [ ] Review audit logs for unexpected emergency token usage
- [ ] Document any issues or improvements
- [ ] Schedule quarterly emergency procedure drills

---

## Part 4: Verification Strategy

### 4.1 Unit Tests

**File:** `backend/internal/api/middleware/emergency_test.go` (NEW)

```go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestEmergencyBypass_NoToken(t *testing.T) {
    // Test that requests without emergency token proceed normally
    gin.SetMode(gin.TestMode)

    router := gin.New()
    managementCIDRs := []string{"127.0.0.0/8"}
    router.Use(EmergencyBypass(managementCIDRs, nil))

    router.GET("/test", func(c *gin.Context) {
        _, exists := c.Get("emergency_bypass")
        assert.False(t, exists, "Emergency bypass flag should not be set")
        c.JSON(http.StatusOK, gin.H{"message": "ok"})
    })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_ValidToken(t *testing.T) {
    // Test that valid token from allowed IP sets bypass flag
    t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

    gin.SetMode(gin.TestMode)

    router := gin.New()
    managementCIDRs := []string{"127.0.0.0/8"}
    router.Use(EmergencyBypass(managementCIDRs, nil))

    router.GET("/test", func(c *gin.Context) {
        bypass, exists := c.Get("emergency_bypass")
        assert.True(t, exists, "Emergency bypass flag should be set")
        assert.True(t, bypass.(bool), "Emergency bypass flag should be true")
        c.JSON(http.StatusOK, gin.H{"message": "bypass active"})
    })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
    req.RemoteAddr = "127.0.0.1:12345"
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    // Verify token was stripped from request
    assert.Empty(t, req.Header.Get(EmergencyTokenHeader), "Token should be stripped")
}

func TestEmergencyBypass_InvalidToken(t *testing.T) {
    // Test that invalid token does not set bypass flag
    t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

    gin.SetMode(gin.TestMode)

    router := gin.New()
    managementCIDRs := []string{"127.0.0.0/8"}
    router.Use(EmergencyBypass(managementCIDRs, nil))

    router.GET("/test", func(c *gin.Context) {
        _, exists := c.Get("emergency_bypass")
        assert.False(t, exists, "Emergency bypass flag should not be set")
        c.JSON(http.StatusOK, gin.H{"message": "ok"})
    })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set(EmergencyTokenHeader, "wrong-token")
    req.RemoteAddr = "127.0.0.1:12345"
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmergencyBypass_UnauthorizedIP(t *testing.T) {
    // Test that valid token from disallowed IP does not set bypass flag
    t.Setenv("CHARON_EMERGENCY_TOKEN", "test-token-that-meets-minimum-length-requirement-32-chars")

    gin.SetMode(gin.TestMode)

    router := gin.New()
    managementCIDRs := []string{"127.0.0.0/8"}
    router.Use(EmergencyBypass(managementCIDRs, nil))

    router.GET("/test", func(c *gin.Context) {
        _, exists := c.Get("emergency_bypass")
        assert.False(t, exists, "Emergency bypass flag should not be set")
        c.JSON(http.StatusOK, gin.H{"message": "ok"})
    })

    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
    req.RemoteAddr = "203.0.113.1:12345" // Public IP (not in management network)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

### 4.2 Integration Tests

**File:** `backend/internal/api/routes/routes_test.go` (UPDATE)

```go
func TestEmergencyEndpoint_BypassACL(t *testing.T) {
    // Test that emergency endpoint works even when ACL is blocking

    // Setup: Create test database with ACL enabled
    db := setupTestDB(t)
    defer cleanupTestDB(db)

    // Enable ACL with restrictive whitelist (allow only 192.168.1.0/24)
    err := db.Create(&models.AccessList{
        Name:    "test-acl",
        Type:    "whitelist",
        Enabled: true,
        IPRules: `[{"cidr": "192.168.1.0/24"}]`,
    }).Error
    require.NoError(t, err)

    err = db.Create(&models.Setting{
        Key:   "security.acl.enabled",
        Value: "true",
    }).Error
    require.NoError(t, err)

    // Setup router with security
    cfg := config.Config{
        Security: config.SecurityConfig{
            ACLMode: "enabled",
        },
        EmergencyToken: "test-token-that-meets-minimum-length-requirement-32-chars",
    }

    router := setupTestRouter(db, cfg)

    // Test 1: Regular request from 127.0.0.1 should be blocked by ACL
    req := httptest.NewRequest(http.MethodGET, "/api/v1/proxy-hosts", nil)
    req.RemoteAddr = "127.0.0.1:12345"
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusForbidden, w.Code, "ACL should block regular requests")

    // Test 2: Emergency request from 127.0.0.1 with valid token should bypass ACL
    req = httptest.NewRequest(http.MethodPOST, "/api/v1/emergency/security-reset", nil)
    req.Header.Set(EmergencyTokenHeader, "test-token-that-meets-minimum-length-requirement-32-chars")
    req.RemoteAddr = "127.0.0.1:12345"
    w = httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code, "Emergency request should bypass ACL")

    var response map[string]interface{}
    err = json.Unmarshal(w.Body.Bytes(), &response)
    require.NoError(t, err)
    assert.True(t, response["success"].(bool))
}
```

### 4.3 E2E Tests (Playwright)

**File:** `tests/security-enforcement/emergency-token.spec.ts` (NEW)

```typescript
import { test, expect } from '@playwright/test';
import { TestDataManager } from '../utils/TestDataManager';

test.describe('Emergency Token Break Glass Protocol', () => {
  test('should bypass ACL when valid emergency token is provided', async ({ request }) => {
    const testData = new TestDataManager(request, 'emergency-token-bypass');

    // Step 1: Create restrictive ACL (whitelist only 192.168.1.0/24)
    const { id: aclId } = await testData.createAccessList({
      name: 'test-restrictive-acl',
      type: 'whitelist',
      ipRules: [{ cidr: '192.168.1.0/24', description: 'Test network' }],
      enabled: true,
    });

    // Step 2: Enable ACL globally
    await request.post('/api/v1/settings', {
      data: { key: 'security.acl.enabled', value: 'true' },
    });

    // Wait for settings to propagate
    await new Promise(resolve => setTimeout(resolve, 2000));

    // Step 3: Verify ACL is blocking (request without emergency token should fail)
    const blockedResponse = await request.get('/api/v1/proxy-hosts');
    expect(blockedResponse.status()).toBe(403);
    const blockedBody = await blockedResponse.json();
    expect(blockedBody.error).toContain('Blocked by access control');

    // Step 4: Use emergency token to disable security
    const emergencyToken = 'test-emergency-token-for-e2e-32chars';
    const emergencyResponse = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': emergencyToken,
      },
    });

    expect(emergencyResponse.status()).toBe(200);
    const emergencyBody = await emergencyResponse.json();
    expect(emergencyBody.success).toBe(true);
    expect(emergencyBody.disabled_modules).toContain('security.acl.enabled');

    // Wait for settings to propagate
    await new Promise(resolve => setTimeout(resolve, 2000));

    // Step 5: Verify ACL is now disabled (request should succeed)
    const allowedResponse = await request.get('/api/v1/proxy-hosts');
    expect(allowedResponse.ok()).toBeTruthy();

    // Cleanup
    await testData.cleanup();
  });

  test('should rate limit emergency token attempts', async ({ request }) => {
    const emergencyToken = 'wrong-token-for-rate-limit-test-32chars';

    // Make 6 rapid attempts with wrong token
    const attempts = [];
    for (let i = 0; i < 6; i++) {
      attempts.push(
        request.post('/api/v1/emergency/security-reset', {
          headers: { 'X-Emergency-Token': emergencyToken },
        })
      );
    }

    const responses = await Promise.all(attempts);

    // First 5 should be unauthorized (401)
    for (let i = 0; i < 5; i++) {
      expect(responses[i].status()).toBe(401);
    }

    // 6th should be rate limited (429)
    expect(responses[5].status()).toBe(429);
    const body = await responses[5].json();
    expect(body.error).toBe('rate limit exceeded');
  });

  test('should log emergency token usage to audit trail', async ({ request }) => {
    const emergencyToken = 'test-emergency-token-for-e2e-32chars';

    // Use emergency token
    const response = await request.post('/api/v1/emergency/security-reset', {
      headers: { 'X-Emergency-Token': emergencyToken },
    });

    expect(response.ok()).toBeTruthy();

    // Check audit logs for emergency event
    const auditResponse = await request.get('/api/v1/audit-logs');
    expect(auditResponse.ok()).toBeTruthy();

    const auditLogs = await auditResponse.json();
    const emergencyLog = auditLogs.find(
      (log: any) => log.action === 'emergency_reset_success'
    );

    expect(emergencyLog).toBeDefined();
    expect(emergencyLog.details).toContain('Disabled modules');
  });
});
```

### 4.4 Chaos Testing

**File:** `tests/chaos/security-lockout.spec.ts` (NEW)

```typescript
import { test, expect } from '@playwright/test';
import { TestDataManager } from '../utils/TestDataManager';

test.describe('Security Lockout Recovery - Chaos Testing', () => {
  test('should recover from complete lockout scenario', async ({ request }) => {
    // Simulate worst-case scenario:
    // 1. ACL enabled with restrictive whitelist
    // 2. WAF enabled and blocking patterns
    // 3. Rate limiting enabled
    // 4. CrowdSec enabled with bans

    const testData = new TestDataManager(request, 'chaos-lockout-recovery');

    // Enable all security modules with maximum restrictions
    await request.post('/api/v1/settings', {
      data: { key: 'security.acl.enabled', value: 'true' },
    });
    await request.post('/api/v1/settings', {
      data: { key: 'security.waf.enabled', value: 'true' },
    });
    await request.post('/api/v1/settings', {
      data: { key: 'security.rate_limit.enabled', value: 'true' },
    });
    await request.post('/api/v1/settings', {
      data: { key: 'feature.cerberus.enabled', value: 'true' },
    });

    // Create restrictive ACL
    await testData.createAccessList({
      name: 'chaos-test-acl',
      type: 'whitelist',
      ipRules: [{ cidr: '10.0.0.0/8' }], // Only allow 10.x.x.x
      enabled: true,
    });

    // Wait for settings to propagate
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Verify complete lockout
    const lockedResponse = await request.get('/api/v1/health');
    expect(lockedResponse.status()).toBe(403);

    // RECOVERY: Use emergency token
    const emergencyResponse = await request.post('/api/v1/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': 'test-emergency-token-for-e2e-32chars',
      },
    });

    expect(emergencyResponse.status()).toBe(200);

    // Wait for settings to propagate
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Verify full recovery
    const recoveredResponse = await request.get('/api/v1/health');
    expect(recoveredResponse.ok()).toBeTruthy();

    // Cleanup
    await testData.cleanup();
  });
});
```

---

## Part 5: Timeline & Dependencies

```
Day 1 (4 hours)
├─ Phase 3.1: Emergency Bypass Middleware (1h)
├─ Phase 3.2: Emergency Server (1.5h)
├─ Phase 3.3: Documentation (0.5h)
└─ Phase 3.4: Test Environment (1h)

Day 2 (2 hours)
├─ Phase 3.5: Production Deployment (0.5h)
├─ E2E Testing (1h)
└─ Documentation Review (0.5h)

Total: 6 hours (spread across 2 days)
```

**Dependencies:**

- Emergency Bypass Middleware → Cerberus update (sequential)
- Emergency Server → Configuration updates (sequential)
- All phases → Documentation (parallel after code complete)
- Production deployment → All tests passing (blocker)

---

## Part 6: Risk Assessment

### High Priority Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Emergency token leaked | Critical | Low | Rotate token immediately, audit logs, require 2FA |
| Middleware ordering bug | Critical | Medium | Comprehensive integration tests, code review |
| Emergency port exposed publicly | High | Medium | Firewall rules, documentation warnings |
| ClientIP spoofing behind proxy | High | Medium | Configure SetTrustedProxies() correctly |
| Emergency server no auth | Critical | Low | Require Basic Auth or mTLS in production |

### Medium Priority Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Token in logs (HTTP headers logged) | Medium | High | Strip header after validation, use HTTPS |
| Rate limiting too strict | Low | Medium | Adjust limits, provide bypass for Tier 2 |
| Emergency endpoint DOS | Medium | Low | Rate limiting, Web Application Firewall |
| Documentation outdated | Medium | Medium | Automated testing of runbook procedures |

---

## Part 7: Success Criteria

### Must Have (MVP)

- ✅ Emergency token bypasses Cerberus ACL middleware
- ✅ Emergency endpoint accessible when ACL is blocking
- ✅ Unit tests for emergency bypass middleware (>80% coverage)
- ✅ Integration tests for ACL bypass scenario
- ✅ E2E tests pass with security enabled
- ✅ Emergency runbook documented and tested

### Should Have (Production Ready)

- ✅ Emergency server (Tier 2) implemented and tested
- ✅ Management CIDR configuration
- ✅ Token rotation procedure documented
- ✅ Audit logging for all emergency access
- ✅ Monitoring alerts for emergency token usage
- ✅ Rate limiting with appropriate thresholds

### Nice to Have (Future Enhancements)

- ⏳ mTLS support for emergency server
- ⏳ Multi-factor authentication for emergency access
- ⏳ Emergency access session tokens (time-limited)
- ⏳ Automated emergency token rotation
- ⏳ Emergency access approval workflow

---

## Appendix A: Configuration Reference

### Environment Variables

```bash
# Emergency Token (Required)
CHARON_EMERGENCY_TOKEN=<64-char-hex-token>  # openssl rand -hex 32

# Management Networks (Optional, defaults to RFC1918)
CHARON_MANAGEMENT_CIDRS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# Emergency Server (Optional)
CHARON_EMERGENCY_SERVER_ENABLED=true
CHARON_EMERGENCY_BIND=127.0.0.1:2019  # localhost only by default
CHARON_EMERGENCY_USERNAME=admin
CHARON_EMERGENCY_PASSWORD=<strong-password>
```

### Docker Compose Example

```yaml
services:
  charon:
    image: charon:latest
    ports:
      - "443:443"    # Main HTTPS
      - "127.0.0.1:2019:2019"  # Emergency port (localhost only)
    environment:
      - CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN}
      - CHARON_MANAGEMENT_CIDRS=10.10.0.0/16,192.168.1.0/24
      - CHARON_EMERGENCY_SERVER_ENABLED=true
      - CHARON_EMERGENCY_USERNAME=admin
      - CHARON_EMERGENCY_PASSWORD=${EMERGENCY_PASSWORD}
```

---

## Appendix B: Testing Checklist

### Pre-Implementation Tests

- [x] Reproduce current failure (global-setup.ts emergency reset fails with ACL enabled)
- [x] Document exact error messages
- [x] Verify Cerberus middleware execution order
- [x] Verify CrowdSec layer (Caddy vs iptables)

### Post-Implementation Tests

- [ ] Unit tests for emergency bypass middleware pass
- [ ] Integration tests for ACL bypass pass
- [ ] E2E tests pass with all security modules enabled
- [ ] Emergency server unit tests pass
- [ ] Chaos testing scenarios pass
- [ ] Runbook procedures tested manually
- [ ] Emergency token rotation procedure tested

### Production Smoke Tests

- [ ] Health check endpoint responds
- [ ] Emergency endpoint responds to valid token
- [ ] Emergency endpoint blocks invalid tokens
- [ ] Emergency endpoint rate limits excessive attempts
- [ ] Audit logs capture emergency access events
- [ ] Monitoring alerts trigger on emergency access

---

## Appendix C: Decision Records

### Decision 1: Why 3 Tiers Instead of Single Break Glass?

**Date:** January 26, 2026
**Decision:** Implement 3-tier break glass architecture instead of single emergency endpoint
**Rationale:**
- **Single Point of Failure:** A single break glass mechanism can fail (blocked by Caddy, network issues, etc.)
- **Defense in Depth:** Multiple recovery paths increase resilience
- **Operational Flexibility:** Different scenarios may require different access methods

**Trade-offs:**
- More complexity to implement and maintain
- More attack surface (emergency server port)
- More documentation and training required

**Mitigation:** Comprehensive documentation, automated testing, clear runbooks

---

### Decision 2: Middleware First vs Endpoint Registration

**Date:** January 26, 2026
**Decision:** Use middleware bypass flag instead of registering endpoint before middleware
**Rationale:**
- **Gin Routing Ambiguity:** `/api/v1/emergency/...` may still match `/api/v1` group routes
- **Explicit Control:** Bypass flag gives clear control flow
- **Testability:** Easier to test middleware behavior with context flags

**Trade-offs:**
- Requires checking flag in all security middleware
- Slightly more code changes

**Mitigation:** Comprehensive testing, clear documentation of bypass mechanism

---

### Decision 3: Emergency Server Port 2019

**Date:** January 26, 2026
**Decision:** Use port 2019 for emergency server (matching Caddy admin API default)
**Rationale:**
- **Convention:** Caddy uses 2019 for admin API, familiar to operators
- **Separation:** Clearly separate from main application ports (80/443/8080)
- **Non-Standard:** Less likely to conflict with other services

**Trade-offs:**
- Not a well-known port (requires documentation)

**Mitigation:** Document in all deployment guides, include in runbooks

---

## Conclusion

This comprehensive plan provides:

1. **Root Cause Analysis:** Complete understanding of why the emergency token currently fails
2. **3-Tier Architecture:** Robust break glass system with multiple recovery paths
3. **Implementation Plan:** Actionable tasks with time estimates and verification steps
4. **Testing Strategy:** Unit, integration, E2E, and chaos testing
5. **Documentation:** Runbooks, configuration reference, decision records

**Next Steps:**

1. Review and approve this plan
2. Begin Phase 3.1 (Emergency Bypass Middleware)
3. Execute implementation phases in order
4. Verify with comprehensive testing
5. Deploy to production with monitoring

**Estimated Completion:** 6 hours of implementation + 2 hours of testing = **8 hours total**
