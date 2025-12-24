# CWE-918 (SSRF) Comprehensive Mitigation Plan

**Status:** Ready for Implementation
**Priority:** CRITICAL
**CWE Reference:** CWE-918 (Server-Side Request Forgery)
**Created:** December 24, 2025
**Supersedes:** `ssrf_remediation_spec.md` (Previous plan - now archived reference)

---

## Executive Summary

This plan implements a **three-layer defense-in-depth strategy** for SSRF mitigation:

1. **Input Validation** - Strictly allowlist schemes and domains
2. **Network Layer ("Safe Dialer")** - Validate IP addresses at connection time (prevents DNS Rebinding)
3. **Client Configuration** - Disable/validate redirects, enforce timeouts

### Current State Analysis

**Good News:** The codebase already has substantial SSRF protection:
- ✅ `internal/security/url_validator.go` - Comprehensive URL validation with IP blocking
- ✅ `internal/utils/url_testing.go` - SSRF-safe dialer implementation exists
- ✅ `internal/services/notification_service.go` - Uses security validation

**Gaps Identified:**
- ⚠️ Multiple `isPrivateIP` implementations exist (should be consolidated)
- ⚠️ HTTP clients not using the safe dialer consistently
- ⚠️ Some services create their own `http.Client` without SSRF protection

---

## Phase 1: Create the Safe Network Package

**Goal:** Centralize all SSRF protection into a single, reusable package.

### 1.1 File Location

**New File:** `/backend/internal/network/safeclient.go`

### 1.2 Functions to Implement

#### `isPrivateIP(ip net.IP) bool`

Checks if an IP is in private/reserved ranges. Consolidates existing implementations.

**CIDR Ranges to Block:**
```go
var privateBlocks = []string{
    // IPv4 Private Networks (RFC 1918)
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",

    // IPv4 Link-Local (RFC 3927) - includes AWS/GCP metadata
    "169.254.0.0/16",

    // IPv4 Loopback
    "127.0.0.0/8",

    // IPv4 Reserved ranges
    "0.0.0.0/8",          // "This network"
    "240.0.0.0/4",        // Reserved for future use
    "255.255.255.255/32", // Broadcast

    // IPv6 Loopback
    "::1/128",

    // IPv6 Unique Local Addresses (RFC 4193)
    "fc00::/7",

    // IPv6 Link-Local
    "fe80::/10",
}
```

#### `safeDialer(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error)`

Custom dial function that:
1. Parses host:port from address
2. Resolves DNS with context timeout
3. Validates ALL resolved IPs against `isPrivateIP`
4. Dials using the validated IP (prevents DNS rebinding)

```go
func safeDialer(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
    return func(ctx context.Context, network, addr string) (net.Conn, error) {
        host, port, err := net.SplitHostPort(addr)
        if err != nil {
            return nil, fmt.Errorf("invalid address: %w", err)
        }

        // Resolve DNS
        ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
        if err != nil {
            return nil, fmt.Errorf("DNS resolution failed: %w", err)
        }

        if len(ips) == 0 {
            return nil, fmt.Errorf("no IP addresses found")
        }

        // Validate ALL IPs
        for _, ip := range ips {
            if isPrivateIP(ip.IP) {
                return nil, fmt.Errorf("connection to private IP blocked: %s", ip.IP)
            }
        }

        // Connect to first validated IP
        dialer := &net.Dialer{Timeout: timeout}
        return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
    }
}
```

#### `NewSafeHTTPClient(opts ...Option) *http.Client`

Creates an HTTP client with:
- Safe dialer for SSRF protection
- Configurable timeout (default: 10s)
- Disabled keep-alives (prevents connection reuse attacks)
- Redirect validation (blocks redirects to private IPs)

```go
type ClientOptions struct {
    Timeout         time.Duration
    AllowRedirects  bool
    MaxRedirects    int
    AllowLocalhost  bool  // For testing only
}

func NewSafeHTTPClient(opts ...Option) *http.Client {
    cfg := defaultOptions()
    for _, opt := range opts {
        opt(&cfg)
    }

    return &http.Client{
        Timeout: cfg.Timeout,
        Transport: &http.Transport{
            DialContext:       safeDialer(cfg.Timeout),
            DisableKeepAlives: true,
            MaxIdleConns:      1,
            IdleConnTimeout:   cfg.Timeout,
            TLSHandshakeTimeout: 10 * time.Second,
        },
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if !cfg.AllowRedirects {
                return http.ErrUseLastResponse
            }
            if len(via) >= cfg.MaxRedirects {
                return fmt.Errorf("too many redirects (max %d)", cfg.MaxRedirects)
            }
            // Validate redirect destination
            return validateRedirectTarget(req.URL)
        },
    }
}
```

### 1.3 Test File

**New File:** `/backend/internal/network/safeclient_test.go`

**Test Cases:**
```go
func TestIsPrivateIP(t *testing.T) {
    tests := []struct {
        ip       string
        isPrivate bool
    }{
        // IPv4 Private (RFC 1918)
        {"10.0.0.1", true},
        {"10.255.255.255", true},
        {"172.16.0.1", true},
        {"172.31.255.255", true},
        {"192.168.0.1", true},
        {"192.168.255.255", true},

        // IPv4 Loopback
        {"127.0.0.1", true},
        {"127.255.255.255", true},

        // Cloud metadata endpoints
        {"169.254.169.254", true},  // AWS/Azure
        {"169.254.0.1", true},

        // IPv4 Reserved
        {"0.0.0.0", true},
        {"240.0.0.1", true},
        {"255.255.255.255", true},

        // IPv6 Loopback
        {"::1", true},

        // IPv6 Unique Local (fc00::/7)
        {"fc00::1", true},
        {"fd00::1", true},

        // IPv6 Link-Local
        {"fe80::1", true},

        // Public IPs (should NOT be blocked)
        {"8.8.8.8", false},
        {"1.1.1.1", false},
        {"203.0.113.1", false},
        {"2001:4860:4860::8888", false},
    }

    for _, tt := range tests {
        t.Run(tt.ip, func(t *testing.T) {
            ip := net.ParseIP(tt.ip)
            if ip == nil {
                t.Fatalf("invalid IP: %s", tt.ip)
            }
            got := isPrivateIP(ip)
            if got != tt.isPrivate {
                t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.isPrivate)
            }
        })
    }
}

func TestSafeDialer_BlocksPrivateIPs(t *testing.T) {
    // Test with mock DNS resolver
}

func TestNewSafeHTTPClient_BlocksSSRF(t *testing.T) {
    // Integration tests
}
```

---

## Phase 2: Update Existing Code to Use Safe Client

### 2.1 Files Requiring Updates

| File | Current Pattern | Change Required |
|------|----------------|-----------------|
| `internal/services/notification_service.go:205` | `&http.Client{Timeout: 10s}` | Use `network.NewSafeHTTPClient()` |
| `internal/services/security_notification_service.go:130` | `&http.Client{Timeout: 10s}` | Use `network.NewSafeHTTPClient()` |
| `internal/services/update_service.go:112` | `&http.Client{Timeout: 5s}` | Use `network.NewSafeHTTPClient(WithTimeout(5s))` |
| `internal/crowdsec/registration.go:136,176,211` | `&http.Client{Timeout: defaultHealthTimeout}` | Use `network.NewSafeHTTPClient()` (localhost-only allowed) |
| `internal/crowdsec/hub_sync.go:185` | Custom Transport | Use `network.NewSafeHTTPClient()` with hub domain allowlist |

### 2.2 Specific Changes

#### notification_service.go (Lines 204-212)

**Current:**
```go
client := &http.Client{
    Timeout: 10 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        return http.ErrUseLastResponse
    },
}
```

**Change To:**
```go
import "github.com/Wikid82/charon/backend/internal/network"

client := network.NewSafeHTTPClient(
    network.WithTimeout(10 * time.Second),
    network.WithAllowLocalhost(), // For testing
)
```

#### security_notification_service.go (Line 130)

**Current:**
```go
client := &http.Client{Timeout: 10 * time.Second}
```

**Change To:**
```go
client := network.NewSafeHTTPClient(
    network.WithTimeout(10 * time.Second),
    network.WithAllowLocalhost(),
)
```

#### update_service.go (Line 112)

**Current:**
```go
client := &http.Client{Timeout: 5 * time.Second}
```

**Change To:**
```go
// Note: update_service.go already has domain allowlist (github.com only)
// Add safe client for defense in depth
client := network.NewSafeHTTPClient(
    network.WithTimeout(5 * time.Second),
)
```

#### crowdsec/hub_sync.go (Lines 173-190)

**Current:**
```go
func newHubHTTPClient(timeout time.Duration) *http.Client {
    transport := &http.Transport{
        Proxy: http.ProxyFromEnvironment,
        DialContext: (&net.Dialer{
            Timeout:   10 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,
        // ...
    }
    return &http.Client{...}
}
```

**Change To:**
```go
func newHubHTTPClient(timeout time.Duration) *http.Client {
    // Hub URLs are already validated by validateHubURL() which:
    // - Enforces HTTPS for production
    // - Allowlists known CrowdSec domains
    // - Allows localhost for testing
    // Add safe dialer for defense-in-depth
    return network.NewSafeHTTPClient(
        network.WithTimeout(timeout),
        network.WithAllowedDomains(
            "hub-data.crowdsec.net",
            "hub.crowdsec.net",
            "raw.githubusercontent.com",
        ),
    )
}
```

#### crowdsec/registration.go

**Current (Lines 136, 176, 211):**
```go
client := &http.Client{Timeout: defaultHealthTimeout}
```

**Change To:**
```go
// LAPI is validated to be localhost only by validateLAPIURL()
// Use safe client but allow localhost
client := network.NewSafeHTTPClient(
    network.WithTimeout(defaultHealthTimeout),
    network.WithAllowLocalhost(),
)
```

---

## Phase 3: Comprehensive Testing

### 3.1 Unit Test Files

| Test File | Purpose |
|-----------|---------|
| `internal/network/safeclient_test.go` | Unit tests for IP validation, safe dialer |
| `internal/security/url_validator_test.go` | Already exists - extend with edge cases |
| `internal/utils/url_testing_test.go` | Already has SSRF tests - verify alignment |

### 3.2 Integration Test File

**New File:** `/backend/integration/ssrf_protection_test.go`

```go
//go:build integration

package integration

import (
    "net/http/httptest"
    "testing"
)

func TestSSRFProtection_EndToEnd(t *testing.T) {
    // Test 1: Webhook to private IP is blocked
    // Test 2: Webhook to public IP works
    // Test 3: DNS rebinding attack is blocked
    // Test 4: Redirect to private IP is blocked
    // Test 5: Cloud metadata endpoint is blocked
}

func TestSSRFProtection_DNSRebinding(t *testing.T) {
    // Setup mock DNS that changes resolution
    // First: returns public IP (passes validation)
    // Second: returns private IP (should be blocked at dial time)
}
```

### 3.3 Test Coverage Targets

| Package | Current Coverage | Target |
|---------|-----------------|--------|
| `internal/network` | NEW | 95%+ |
| `internal/security` | ~85% | 95%+ |
| `internal/utils` (url_testing.go) | ~80% | 90%+ |

---

## Phase 4: Code Consolidation

### 4.1 Duplicate `isPrivateIP` Functions to Consolidate

Currently found in:
1. `internal/security/url_validator.go:isPrivateIP()` - Comprehensive
2. `internal/utils/url_testing.go:isPrivateIP()` - Comprehensive
3. `internal/services/notification_service.go:isPrivateIP()` - Partial
4. `internal/utils/ip_helpers.go:IsPrivateIP()` - IPv4 only

**Action:** Keep `internal/network/safeclient.go:IsPrivateIP()` as the canonical implementation and update all other files to import from `network` package.

### 4.2 Migration Strategy

1. Create `internal/network/safeclient.go` with `IsPrivateIP()` exported
2. Update `internal/security/url_validator.go` to use `network.IsPrivateIP()`
3. Update `internal/utils/url_testing.go` to use `network.IsPrivateIP()`
4. Update `internal/services/notification_service.go` to use `network.IsPrivateIP()`
5. Deprecate `internal/utils/ip_helpers.go:IsPrivateIP()` (keep for backward compat, wrap network package)

---

## Phase 5: Documentation Updates

### 5.1 Files to Update

| File | Change |
|------|--------|
| `docs/security/ssrf-protection.md` | Already exists - update with new package location |
| `SECURITY.md` | Add section on SSRF protection |
| Inline code docs | Add godoc comments to all new functions |

### 5.2 API Documentation

Document in `docs/api.md`:
- Webhook URL validation requirements
- Allowed/blocked URL patterns
- Error messages and their meanings

---

## Configuration Files Review

### .gitignore ✅

Already ignores:
- `codeql-db-*/`
- `*.sarif`
- Test artifacts

**No changes needed.**

### .dockerignore ✅

Already ignores:
- `codeql-db-*/`
- `*.sarif`
- Test artifacts
- `coverage/`

**No changes needed.**

### codecov.yml

**Verify coverage thresholds include new package:**
```yaml
coverage:
  status:
    project:
      default:
        target: 85%
    patch:
      default:
        target: 90%
```

**No changes needed** (new package will be automatically included).

### Dockerfile ✅

The SSRF protection is runtime code - no Dockerfile changes needed.

---

## Implementation Checklist

### Week 1: Core Implementation

- [ ] Create `/backend/internal/network/` directory
- [ ] Implement `safeclient.go` with:
  - [ ] `IsPrivateIP()` function
  - [ ] `safeDialer()` function
  - [ ] `NewSafeHTTPClient()` function
  - [ ] Option pattern (WithTimeout, WithAllowLocalhost, etc.)
- [ ] Create `safeclient_test.go` with comprehensive tests
- [ ] Run tests: `go test ./internal/network/...`

### Week 2: Integration

- [ ] Update `internal/services/notification_service.go`
- [ ] Update `internal/services/security_notification_service.go`
- [ ] Update `internal/services/update_service.go`
- [ ] Update `internal/crowdsec/registration.go`
- [ ] Update `internal/crowdsec/hub_sync.go`
- [ ] Consolidate duplicate `isPrivateIP` implementations
- [ ] Run full test suite: `go test ./...`

### Week 3: Testing & Documentation

- [ ] Create integration tests
- [ ] Run CodeQL scan to verify SSRF fixes
- [ ] Update documentation
- [ ] Code review
- [ ] Merge to main

---

## Risk Mitigation

### Risk 1: Breaking Localhost Testing

**Mitigation:** `WithAllowLocalhost()` option explicitly enables localhost for testing environments.

### Risk 2: Breaking Legitimate Internal Services

**Mitigation:**
- CrowdSec LAPI: Allowed via localhost exception
- CrowdSec Hub: Domain allowlist (crowdsec.net, github.com)
- Internal services should use service discovery, not hardcoded IPs

### Risk 3: DNS Resolution Overhead

**Mitigation:** Safe dialer performs DNS resolution during dial, which is the standard pattern. No additional overhead for most use cases.

---

## Success Criteria

1. ✅ All HTTP clients use `network.NewSafeHTTPClient()`
2. ✅ No direct `&http.Client{}` construction in service code
3. ✅ CodeQL scan shows no CWE-918 findings
4. ✅ All tests pass (unit + integration)
5. ✅ Coverage > 85% for new package
6. ✅ Documentation updated

---

## File Tree Summary

```
backend/
├── internal/
│   ├── network/                      # NEW PACKAGE
│   │   ├── safeclient.go            # IsPrivateIP, safeDialer, NewSafeHTTPClient
│   │   └── safeclient_test.go       # Comprehensive unit tests
│   │
│   ├── security/
│   │   ├── url_validator.go         # UPDATE: Use network.IsPrivateIP
│   │   └── url_validator_test.go    # Existing tests
│   │
│   ├── services/
│   │   ├── notification_service.go  # UPDATE: Use NewSafeHTTPClient
│   │   ├── security_notification_service.go  # UPDATE: Use NewSafeHTTPClient
│   │   └── update_service.go        # UPDATE: Use NewSafeHTTPClient
│   │
│   ├── crowdsec/
│   │   ├── hub_sync.go              # UPDATE: Use NewSafeHTTPClient
│   │   └── registration.go          # UPDATE: Use NewSafeHTTPClient
│   │
│   └── utils/
│       ├── url_testing.go           # UPDATE: Use network.IsPrivateIP
│       └── ip_helpers.go            # DEPRECATE: Wrap network.IsPrivateIP
│
├── integration/
│   └── ssrf_protection_test.go      # NEW: Integration tests
```

---

## References

- Previous spec: `docs/plans/ssrf_remediation_spec.md`
- OWASP SSRF Prevention: https://owasp.org/www-community/vulnerabilities/SSRF
- CWE-918: https://cwe.mitre.org/data/definitions/918.html
- Go net package: https://pkg.go.dev/net

---

**Document Version:** 1.0
**Last Updated:** December 24, 2025
**Owner:** Security Team
**Status:** READY FOR IMPLEMENTATION

---

## Phase 1: CodeQL Task Alignment (PRIORITY 1)

### Problem Analysis

**Current Local Configuration** (`.vscode/tasks.json`):
```bash
# Go Task
codeql database create codeql-db-go --language=go --source-root=backend --overwrite && \
codeql database analyze codeql-db-go \
  /projects/codeql/codeql/go/ql/src/codeql-suites/go-security-extended.qls \
  --format=sarif-latest --output=codeql-results-go.sarif

# JavaScript Task
codeql database create codeql-db-js --language=javascript --source-root=frontend --overwrite && \
codeql database analyze codeql-db-js \
  /projects/codeql/codeql/javascript/ql/src/codeql-suites/javascript-security-extended.qls \
  --format=sarif-latest --output=codeql-results-js.sarif
```

**Issues:**
1. **Wrong Query Suite:** Using `security-extended` instead of `security-and-quality`
2. **Hardcoded Paths:** Using `/projects/codeql/...` which doesn't exist (causes fallback to installed packs)
3. **Missing CI Parameters:** Not using same threading, memory limits, or build flags as CI
4. **No Results Summary:** Raw SARIF output without human-readable summary

**GitHub Actions CI Configuration** (`.github/workflows/codeql.yml`):
- Uses `github/codeql-action/init@v4` which defaults to `security-and-quality` suite
- Runs autobuild step for compilation
- Uploads results to GitHub Security tab
- Matrix strategy: `['go', 'javascript-typescript']`

### Solution: Exact CI Replication

**File:** `.vscode/tasks.json`

**New Task Configuration:**

```json
{
    "label": "Security: CodeQL Go Scan (CI-Aligned)",
    "type": "shell",
    "command": "bash -c 'set -e && \
        echo \"🔍 Creating CodeQL database for Go...\" && \
        rm -rf codeql-db-go && \
        codeql database create codeql-db-go \
          --language=go \
          --source-root=backend \
          --overwrite \
          --threads=0 && \
        echo \"\" && \
        echo \"📊 Running CodeQL analysis (security-and-quality suite)...\" && \
        codeql database analyze codeql-db-go \
          codeql/go-queries:codeql-suites/go-security-and-quality.qls \
          --format=sarif-latest \
          --output=codeql-results-go.sarif \
          --sarif-add-baseline-file-info \
          --threads=0 && \
        echo \"\" && \
        echo \"✅ CodeQL scan complete. Results: codeql-results-go.sarif\" && \
        echo \"\" && \
        echo \"📋 Summary of findings:\" && \
        codeql database interpret-results codeql-db-go \
          --format=text \
          --output=/dev/stdout \
          codeql/go-queries:codeql-suites/go-security-and-quality.qls 2>/dev/null || \
          (echo \"⚠️  Use SARIF Viewer extension to view detailed results\" && jq -r \".runs[].results[] | \\\"\\(.level): \\(.message.text) (\\(.locations[0].physicalLocation.artifactLocation.uri):\\(.locations[0].physicalLocation.region.startLine))\\\"\" codeql-results-go.sarif 2>/dev/null | head -20 || echo \"No findings or jq not available\")'",
    "group": "test",
    "problemMatcher": [],
    "presentation": {
        "echo": true,
        "reveal": "always",
        "focus": false,
        "panel": "shared",
        "showReuseMessage": false,
        "clear": false
    }
},
{
    "label": "Security: CodeQL JS Scan (CI-Aligned)",
    "type": "shell",
    "command": "bash -c 'set -e && \
        echo \"🔍 Creating CodeQL database for JavaScript/TypeScript...\" && \
        rm -rf codeql-db-js && \
        codeql database create codeql-db-js \
          --language=javascript \
          --source-root=frontend \
          --overwrite \
          --threads=0 && \
        echo \"\" && \
        echo \"📊 Running CodeQL analysis (security-and-quality suite)...\" && \
        codeql database analyze codeql-db-js \
          codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \
          --format=sarif-latest \
          --output=codeql-results-js.sarif \
          --sarif-add-baseline-file-info \
          --threads=0 && \
        echo \"\" && \
        echo \"✅ CodeQL scan complete. Results: codeql-results-js.sarif\" && \
        echo \"\" && \
        echo \"📋 Summary of findings:\" && \
        codeql database interpret-results codeql-db-js \
          --format=text \
          --output=/dev/stdout \
          codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls 2>/dev/null || \
          (echo \"⚠️  Use SARIF Viewer extension to view detailed results\" && jq -r \".runs[].results[] | \\\"\\(.level): \\(.message.text) (\\(.locations[0].physicalLocation.artifactLocation.uri):\\(.locations[0].physicalLocation.region.startLine))\\\"\" codeql-results-js.sarif 2>/dev/null | head -20 || echo \"No findings or jq not available\")'",
    "group": "test",
    "problemMatcher": [],
    "presentation": {
        "echo": true,
        "reveal": "always",
        "focus": false,
        "panel": "shared",
        "showReuseMessage": false,
        "clear": false
    }
},
{
    "label": "Security: CodeQL All (CI-Aligned)",
    "type": "shell",
    "dependsOn": ["Security: CodeQL Go Scan (CI-Aligned)", "Security: CodeQL JS Scan (CI-Aligned)"],
    "dependsOrder": "sequence",
    "group": "test",
    "problemMatcher": []
}
```

**Key Changes:**
1. ✅ **Correct Query Suite:** `security-and-quality` (matches CI default)
2. ✅ **Proper Pack References:** `codeql/go-queries:codeql-suites/...` format
3. ✅ **Threading:** `--threads=0` (auto-detect, same as CI)
4. ✅ **Baseline Info:** `--sarif-add-baseline-file-info` flag
5. ✅ **Human-Readable Output:** Attempts text summary, falls back to jq parsing
6. ✅ **Clean Database:** Removes old DB before creating new one
7. ✅ **Combined Task:** "Security: CodeQL All" runs both sequentially

**SARIF Viewing:**
- Primary: VS Code SARIF Viewer extension (recommended: `MS-SarifVSCode.sarif-viewer`)
- Fallback: `jq` command-line parsing for quick overview
- Alternative: Upload to GitHub Security tab manually

---

## Phase 2: Pre-Commit Integration

### Problem Analysis

**Current Pre-Commit Configuration** (`.pre-commit-config.yaml`):
- ✅ Has manual-stage hook for `security-scan` (govulncheck only)
- ❌ No CodeQL integration
- ❌ No severity-based blocking

### Solution: Add CodeQL Pre-Commit Hooks

**File:** `.pre-commit-config.yaml`

**Add to `repos[local].hooks` section:**

```yaml
      - id: codeql-go-scan
        name: CodeQL Go Security Scan (Manual - Slow)
        entry: scripts/pre-commit-hooks/codeql-go-scan.sh
        language: script
        files: '\.go$'
        pass_filenames: false
        verbose: true
        stages: [manual]  # Performance: 30-60s, only run on-demand

      - id: codeql-js-scan
        name: CodeQL JavaScript/TypeScript Security Scan (Manual - Slow)
        entry: scripts/pre-commit-hooks/codeql-js-scan.sh
        language: script
        files: '^frontend/.*\.(ts|tsx|js|jsx)$'
        pass_filenames: false
        verbose: true
        stages: [manual]  # Performance: 30-60s, only run on-demand

      - id: codeql-check-findings
        name: Block HIGH/CRITICAL CodeQL Findings
        entry: scripts/pre-commit-hooks/codeql-check-findings.sh
        language: script
        pass_filenames: false
        verbose: true
        stages: [manual]  # Only runs after CodeQL scans
```

### New Script: `scripts/pre-commit-hooks/codeql-go-scan.sh`

```bash
#!/bin/bash
# Pre-commit CodeQL Go scan - CI-aligned
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running CodeQL Go scan (CI-aligned)...${NC}"
echo ""

# Clean previous database
rm -rf codeql-db-go

# Create database
echo "📦 Creating CodeQL database..."
codeql database create codeql-db-go \
  --language=go \
  --source-root=backend \
  --threads=0 \
  --overwrite

echo ""
echo "📊 Analyzing with security-and-quality suite..."
# Analyze with CI-aligned suite
codeql database analyze codeql-db-go \
  codeql/go-queries:codeql-suites/go-security-and-quality.qls \
  --format=sarif-latest \
  --output=codeql-results-go.sarif \
  --sarif-add-baseline-file-info \
  --threads=0

echo -e "${GREEN}✅ CodeQL Go scan complete${NC}"
echo "Results saved to: codeql-results-go.sarif"
echo ""
echo "Run 'pre-commit run codeql-check-findings' to validate findings"
```

### New Script: `scripts/pre-commit-hooks/codeql-js-scan.sh`

```bash
#!/bin/bash
# Pre-commit CodeQL JavaScript/TypeScript scan - CI-aligned
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running CodeQL JavaScript/TypeScript scan (CI-aligned)...${NC}"
echo ""

# Clean previous database
rm -rf codeql-db-js

# Create database
echo "📦 Creating CodeQL database..."
codeql database create codeql-db-js \
  --language=javascript \
  --source-root=frontend \
  --threads=0 \
  --overwrite

echo ""
echo "📊 Analyzing with security-and-quality suite..."
# Analyze with CI-aligned suite
codeql database analyze codeql-db-js \
  codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \
  --format=sarif-latest \
  --output=codeql-results-js.sarif \
  --sarif-add-baseline-file-info \
  --threads=0

echo -e "${GREEN}✅ CodeQL JavaScript/TypeScript scan complete${NC}"
echo "Results saved to: codeql-results-js.sarif"
echo ""
echo "Run 'pre-commit run codeql-check-findings' to validate findings"
```

### New Script: `scripts/pre-commit-hooks/codeql-check-findings.sh`

```bash
#!/bin/bash
# Check CodeQL SARIF results for HIGH/CRITICAL findings
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0

check_sarif() {
    local sarif_file=$1
    local lang=$2

    if [ ! -f "$sarif_file" ]; then
        echo -e "${YELLOW}⚠️  No SARIF file found: $sarif_file${NC}"
        echo "Run CodeQL scan first: pre-commit run codeql-$lang-scan --all-files"
        return 0
    fi

    echo "🔍 Checking $lang findings..."

    # Check for findings using jq (if available)
    if command -v jq &> /dev/null; then
        # Count high/critical severity findings
        HIGH_COUNT=$(jq -r '.runs[].results[] | select(.level == "error" or .level == "warning") | .level' "$sarif_file" 2>/dev/null | wc -l || echo 0)

        if [ "$HIGH_COUNT" -gt 0 ]; then
            echo -e "${RED}❌ Found $HIGH_COUNT potential security issues in $lang code${NC}"
            echo ""
            echo "Summary:"
            jq -r '.runs[].results[] | "\(.level): \(.message.text) (\(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine))"' "$sarif_file" 2>/dev/null | head -10
            echo ""
            echo "View full results: code $sarif_file"
            FAILED=1
        else
            echo -e "${GREEN}✅ No security issues found in $lang code${NC}"
        fi
    else
        # Fallback: check if file has results
        if grep -q '"results"' "$sarif_file" && ! grep -q '"results": \[\]' "$sarif_file"; then
            echo -e "${YELLOW}⚠️  CodeQL findings detected in $lang (install jq for details)${NC}"
            echo "View results: code $sarif_file"
            FAILED=1
        else
            echo -e "${GREEN}✅ No security issues found in $lang code${NC}"
        fi
    fi
}

echo "🔒 Checking CodeQL findings..."
echo ""

check_sarif "codeql-results-go.sarif" "go"
check_sarif "codeql-results-js.sarif" "js"

if [ $FAILED -eq 1 ]; then
    echo ""
    echo -e "${RED}❌ CodeQL scan found security issues. Please fix before committing.${NC}"
    echo ""
    echo "To view results:"
    echo "  - VS Code: Install SARIF Viewer extension"
    echo "  - Command line: jq . codeql-results-*.sarif"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ All CodeQL checks passed${NC}"
```

**Make scripts executable:**
```bash
chmod +x scripts/pre-commit-hooks/codeql-*.sh
```

### Usage Instructions for Developers

**Quick Security Check (Fast - 5s):**
```bash
pre-commit run security-scan --all-files
```

**Full CodeQL Scan (Slow - 2-3min):**
```bash
# Scan Go code
pre-commit run codeql-go-scan --all-files

# Scan JavaScript/TypeScript code
pre-commit run codeql-js-scan --all-files

# Check for HIGH/CRITICAL findings
pre-commit run codeql-check-findings --all-files
```

**Combined Workflow:**
```bash
# Run all security checks
pre-commit run security-scan codeql-go-scan codeql-js-scan codeql-check-findings --all-files
```

---

## Phase 3: CI/CD Enhancement

### Current CI Analysis

**Strengths:**
- ✅ Runs on push/PR to main, development, feature branches
- ✅ Matrix strategy for multiple languages
- ✅ Results uploaded to GitHub Security tab
- ✅ Scheduled weekly scan (Monday 3 AM)

**Weaknesses:**
- ❌ No blocking on HIGH/CRITICAL findings
- ❌ No PR comments with findings summary
- ❌ Forked PRs skip security checks (intentional, but should be documented)

### Solution: Enhanced CI Workflow

**File:** `.github/workflows/codeql.yml`

**Add after analysis step:**

```yaml
      - name: Check CodeQL Results
        if: always()
        run: |
          echo "## 🔒 CodeQL Security Analysis Results" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Language:** ${{ matrix.language }}" >> $GITHUB_STEP_SUMMARY
          echo "**Query Suite:** security-and-quality" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY

          # Check if SARIF file exists and has results
          SARIF_FILE="${HOME}/work/_temp/codeql-action-results/codeql-action-results-${{ matrix.language }}.sarif"

          if [ -f "$SARIF_FILE" ]; then
            RESULT_COUNT=$(jq '.runs[].results | length' "$SARIF_FILE" 2>/dev/null || echo 0)
            ERROR_COUNT=$(jq '[.runs[].results[] | select(.level == "error")] | length' "$SARIF_FILE" 2>/dev/null || echo 0)
            WARNING_COUNT=$(jq '[.runs[].results[] | select(.level == "warning")] | length' "$SARIF_FILE" 2>/dev/null || echo 0)
            NOTE_COUNT=$(jq '[.runs[].results[] | select(.level == "note")] | length' "$SARIF_FILE" 2>/dev/null || echo 0)

            echo "**Findings:**" >> $GITHUB_STEP_SUMMARY
            echo "- 🔴 Errors: $ERROR_COUNT" >> $GITHUB_STEP_SUMMARY
            echo "- 🟡 Warnings: $WARNING_COUNT" >> $GITHUB_STEP_SUMMARY
            echo "- 🔵 Notes: $NOTE_COUNT" >> $GITHUB_STEP_SUMMARY
            echo "" >> $GITHUB_STEP_SUMMARY

            if [ "$ERROR_COUNT" -gt 0 ]; then
              echo "❌ **CRITICAL:** High-severity security issues found!" >> $GITHUB_STEP_SUMMARY
              echo "" >> $GITHUB_STEP_SUMMARY
              echo "### Top Issues:" >> $GITHUB_STEP_SUMMARY
              echo '```' >> $GITHUB_STEP_SUMMARY
              jq -r '.runs[].results[] | select(.level == "error") | "\(.ruleId): \(.message.text)"' "$SARIF_FILE" 2>/dev/null | head -5 >> $GITHUB_STEP_SUMMARY
              echo '```' >> $GITHUB_STEP_SUMMARY
            else
              echo "✅ No high-severity issues found" >> $GITHUB_STEP_SUMMARY
            fi
          else
            echo "⚠️ SARIF file not found - check analysis logs" >> $GITHUB_STEP_SUMMARY
          fi

          echo "" >> $GITHUB_STEP_SUMMARY
          echo "View full results in the [Security tab](https://github.com/${{ github.repository }}/security/code-scanning)" >> $GITHUB_STEP_SUMMARY

      - name: Fail on High-Severity Findings
        if: always()
        run: |
          SARIF_FILE="${HOME}/work/_temp/codeql-action-results/codeql-action-results-${{ matrix.language }}.sarif"

          if [ -f "$SARIF_FILE" ]; then
            ERROR_COUNT=$(jq '[.runs[].results[] | select(.level == "error")] | length' "$SARIF_FILE" 2>/dev/null || echo 0)

            if [ "$ERROR_COUNT" -gt 0 ]; then
              echo "::error::CodeQL found $ERROR_COUNT high-severity security issues. Fix before merging."
              exit 1
            fi
          fi
```

### New Workflow: Security Issue Creation

**File:** `.github/workflows/codeql-issue-reporter.yml`

```yaml
name: CodeQL - Create Issues for Findings

on:
  workflow_run:
    workflows: ["CodeQL - Analyze"]
    types:
      - completed
    branches: [main, development]

permissions:
  contents: read
  security-events: read
  issues: write

jobs:
  create-issues:
    name: Create GitHub Issues for CodeQL Findings
    runs-on: ubuntu-latest
    if: ${{ github.event.workflow_run.conclusion == 'failure' }}
    steps:
      - uses: actions/checkout@v4

      - name: Get CodeQL Alerts
        id: get-alerts
        uses: actions/github-script@v7
        with:
          script: |
            const alerts = await github.rest.codeScanning.listAlertsForRepo({
              owner: context.repo.owner,
              repo: context.repo.repo,
              state: 'open',
              severity: 'high,critical'
            });

            console.log(`Found ${alerts.data.length} high/critical alerts`);

            for (const alert of alerts.data.slice(0, 5)) { // Limit to 5 issues
              const title = `[Security] ${alert.rule.security_severity_level}: ${alert.rule.description}`;
              const body = `
## Security Alert from CodeQL

**Severity:** ${alert.rule.security_severity_level}
**Rule:** ${alert.rule.id}
**Location:** ${alert.most_recent_instance.location.path}:${alert.most_recent_instance.location.start_line}

### Description
${alert.rule.description}

### Message
${alert.most_recent_instance.message.text}

### View in CodeQL
${alert.html_url}

---
*This issue was automatically created from a CodeQL security scan.*
*Fix this issue and the corresponding CodeQL alert will automatically close.*
`;

              // Check if issue already exists
              const existingIssues = await github.rest.issues.listForRepo({
                owner: context.repo.owner,
                repo: context.repo.repo,
                labels: 'security,codeql',
                state: 'open'
              });

              const exists = existingIssues.data.some(issue =>
                issue.title.includes(alert.rule.id)
              );

              if (!exists) {
                await github.rest.issues.create({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  title: title,
                  body: body,
                  labels: ['security', 'codeql', 'automated']
                });
                console.log(`Created issue for alert ${alert.number}`);
              }
            }
```

---

## Phase 4: Documentation & Training

### New Documentation: `docs/security/codeql-scanning.md`

**File:** `docs/security/codeql-scanning.md`

```markdown
# CodeQL Security Scanning Guide

## Overview

Charon uses GitHub's CodeQL for static application security testing (SAST). CodeQL analyzes code to find security vulnerabilities and coding errors.

## Quick Start

### Run CodeQL Locally (CI-Aligned)

**Via VS Code Tasks:**
1. Open Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`)
2. Type "Tasks: Run Task"
3. Select:
   - `Security: CodeQL Go Scan (CI-Aligned)` - Scan backend
   - `Security: CodeQL JS Scan (CI-Aligned)` - Scan frontend
   - `Security: CodeQL All (CI-Aligned)` - Scan both

**Via Pre-Commit:**
```bash
# Quick security check (govulncheck - 5s)
pre-commit run security-scan --all-files

# Full CodeQL scan (2-3 minutes)
pre-commit run codeql-go-scan --all-files
pre-commit run codeql-js-scan --all-files
pre-commit run codeql-check-findings --all-files
```

**Via Command Line:**
```bash
# Go scan
codeql database create codeql-db-go --language=go --source-root=backend --overwrite
codeql database analyze codeql-db-go \
  codeql/go-queries:codeql-suites/go-security-and-quality.qls \
  --format=sarif-latest --output=codeql-results-go.sarif

# JavaScript/TypeScript scan
codeql database create codeql-db-js --language=javascript --source-root=frontend --overwrite
codeql database analyze codeql-db-js \
  codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \
  --format=sarif-latest --output=codeql-results-js.sarif
```

### View Results

**Method 1: VS Code SARIF Viewer (Recommended)**
1. Install extension: `MS-SarifVSCode.sarif-viewer`
2. Open `codeql-results-go.sarif` or `codeql-results-js.sarif`
3. Navigate findings with inline annotations

**Method 2: Command Line (jq)**
```bash
# Summary
jq '.runs[].results | length' codeql-results-go.sarif

# Details
jq -r '.runs[].results[] | "\(.level): \(.message.text) (\(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine))"' codeql-results-go.sarif
```

**Method 3: GitHub Security Tab**
- CI automatically uploads results to: `https://github.com/YourOrg/Charon/security/code-scanning`

## Understanding Query Suites

Charon uses the **security-and-quality** suite (GitHub Actions default):

| Suite | Go Queries | JS Queries | Use Case |
|-------|-----------|-----------|----------|
| `security-extended` | 39 | 106 | Security-only, faster |
| `security-and-quality` | 61 | 204 | Security + quality, comprehensive (CI default) |

⚠️ **Important:** Local scans MUST use `security-and-quality` to match CI behavior.

## Severity Levels

- 🔴 **Error (High/Critical):** Must fix before merge - CI will fail
- 🟡 **Warning (Medium):** Should fix - CI continues
- 🔵 **Note (Low/Info):** Consider fixing - CI continues

## Common Issues & Fixes

### Issue: "CWE-918: Server-Side Request Forgery (SSRF)"

**Location:** `backend/internal/api/handlers/url_validator.go`

**Fix:**
```go
// BAD: Unrestricted URL
resp, err := http.Get(userProvidedURL)

// GOOD: Validate against allowlist
if !isAllowedHost(userProvidedURL) {
    return ErrSSRFAttempt
}
resp, err := http.Get(userProvidedURL)
```

**Reference:** [docs/security/ssrf-protection.md](ssrf-protection.md)

### Issue: "CWE-079: Cross-Site Scripting (XSS)"

**Location:** `frontend/src/components/...`

**Fix:**
```typescript
// BAD: Unsafe HTML rendering
element.innerHTML = userInput;

// GOOD: Safe text content
element.textContent = userInput;

// GOOD: Sanitized HTML (if HTML is required)
import DOMPurify from 'dompurify';
element.innerHTML = DOMPurify.sanitize(userInput);
```

### Issue: "CWE-089: SQL Injection"

**Fix:** Use parameterized queries (GORM handles this automatically)
```go
// BAD: String concatenation
db.Raw("SELECT * FROM users WHERE name = '" + userName + "'")

// GOOD: Parameterized query
db.Where("name = ?", userName).Find(&users)
```

## CI/CD Integration

### When CodeQL Runs

- **Push:** Every commit to `main`, `development`, `feature/*`
- **Pull Request:** Every PR to `main`, `development`
- **Schedule:** Weekly scan on Monday at 3 AM UTC

### CI Behavior

✅ **Allowed to merge:**
- No findings
- Only warnings/notes
- Forked PRs (security scanning skipped for permission reasons)

❌ **Blocked from merge:**
- Any error-level (high/critical) findings
- CodeQL analysis failure

### Viewing CI Results

1. **PR Checks:** See "CodeQL analysis (go)" and "CodeQL analysis (javascript-typescript)" checks
2. **Security Tab:** Navigate to repo → Security → Code scanning alerts
3. **Workflow Summary:** Click on failed check → View job summary

## Troubleshooting

### "CodeQL passes locally but fails in CI"

**Cause:** Using wrong query suite locally

**Fix:** Ensure tasks use `security-and-quality`:
```bash
codeql database analyze DB_PATH \
  codeql/LANGUAGE-queries:codeql-suites/LANGUAGE-security-and-quality.qls \
  ...
```

### "SARIF file not found"

**Cause:** Database creation or analysis failed

**Fix:**
1. Check terminal output for errors
2. Ensure CodeQL is installed: `codeql version`
3. Verify source-root exists: `ls backend/` or `ls frontend/`

### "Too many findings to fix"

**Strategy:**
1. Fix all **error** level first (CI blockers)
2. Create issues for **warning** level (non-blocking)
3. Document **note** level for future consideration

**Suppress false positives:**
```go
// codeql[go/sql-injection] - Safe: input is validated by ACL
db.Raw(query).Scan(&results)
```

## Performance Tips

- **Incremental Scans:** CodeQL caches databases, second run is faster
- **Parallel Execution:** Use `--threads=0` for auto-detection
- **CI Only:** Run full scans in CI, quick checks locally

## References

- [CodeQL Documentation](https://codeql.github.com/docs/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Database](https://cwe.mitre.org/)
- [Charon Security Policy](../SECURITY.md)
```

### Update Definition of Done

**File:** `.github/instructions/copilot-instructions.md`

**Section: "✅ Task Completion Protocol (Definition of Done)"**

**Replace Step 1 with:**

```markdown
1. **Security Scans** (MANDATORY - Zero Tolerance):
    - **CodeQL Go Scan**: Run VS Code task "Security: CodeQL Go Scan (CI-Aligned)" OR `pre-commit run codeql-go-scan --all-files`
        - Must use `security-and-quality` suite (CI-aligned)
        - **Zero high/critical (error-level) findings allowed**
        - Medium/low findings should be documented and triaged
    - **CodeQL JS Scan**: Run VS Code task "Security: CodeQL JS Scan (CI-Aligned)" OR `pre-commit run codeql-js-scan --all-files`
        - Must use `security-and-quality` suite (CI-aligned)
        - **Zero high/critical (error-level) findings allowed**
        - Medium/low findings should be documented and triaged
    - **Validate Findings**: Run `pre-commit run codeql-check-findings --all-files` to check for HIGH/CRITICAL issues
    - **Trivy Container Scan**: Run VS Code task "Security: Trivy Scan" for container/dependency vulnerabilities
    - **Results Viewing**:
        - Primary: VS Code SARIF Viewer extension (`MS-SarifVSCode.sarif-viewer`)
        - Alternative: `jq` command-line parsing: `jq '.runs[].results' codeql-results-*.sarif`
        - CI: GitHub Security tab for automated uploads
    - **⚠️ CRITICAL:** CodeQL scans are NOT run by default pre-commit hooks (manual stage for performance). You MUST run them explicitly via VS Code tasks or pre-commit manual commands before completing any task.
    - **Why:** CI enforces security-and-quality suite and blocks HIGH/CRITICAL findings. Local verification prevents CI failures and ensures security compliance.
    - **CI Alignment:** Local scans now use identical parameters to CI:
        - Query suite: `security-and-quality` (61 Go queries, 204 JS queries)
        - Database creation: `--threads=0 --overwrite`
        - Analysis: `--sarif-add-baseline-file-info`
```

---

## Implementation Checklist

### Phase 1: CodeQL Alignment
- [ ] Update `.vscode/tasks.json` with new CI-aligned tasks
- [ ] Remove old tasks: "Security: CodeQL Go Scan", "Security: CodeQL JS Scan"
- [ ] Add new tasks: "Security: CodeQL Go Scan (CI-Aligned)", "Security: CodeQL JS Scan (CI-Aligned)", "Security: CodeQL All (CI-Aligned)"
- [ ] Test Go scan: Run task and verify it uses `security-and-quality` suite
- [ ] Test JS scan: Run task and verify it uses `security-and-quality` suite
- [ ] Install VS Code SARIF Viewer extension for result viewing
- [ ] Verify SARIF files are generated correctly

### Phase 2: Pre-Commit Integration
- [ ] Create `scripts/pre-commit-hooks/codeql-go-scan.sh`
- [ ] Create `scripts/pre-commit-hooks/codeql-js-scan.sh`
- [ ] Create `scripts/pre-commit-hooks/codeql-check-findings.sh`
- [ ] Make scripts executable: `chmod +x scripts/pre-commit-hooks/codeql-*.sh`
- [ ] Update `.pre-commit-config.yaml` with new hooks
- [ ] Test hooks: `pre-commit run codeql-go-scan --all-files`
- [ ] Test findings check: `pre-commit run codeql-check-findings --all-files`
- [ ] Update `.gitignore` (already has `codeql-db-*/`, `*.sarif` - verify)

### Phase 3: CI/CD Enhancement
- [ ] Update `.github/workflows/codeql.yml` with result checking steps
- [ ] Create `.github/workflows/codeql-issue-reporter.yml` (optional)
- [ ] Test CI workflow on a test branch
- [ ] Verify step summary shows findings count
- [ ] Verify CI fails on high-severity findings
- [ ] Document CI behavior in workflow comments

### Phase 4: Documentation
- [ ] Create `docs/security/codeql-scanning.md`
- [ ] Update `.github/instructions/copilot-instructions.md` Definition of Done
- [ ] Update `docs/security.md` with CodeQL section (if needed)
- [ ] Add CodeQL badge to `README.md` (optional)
- [ ] Create troubleshooting guide section
- [ ] Document CI-local alignment in CONTRIBUTING.md

### Phase 5: Verification
- [ ] Run full security scan locally: `pre-commit run codeql-go-scan codeql-js-scan codeql-check-findings --all-files`
- [ ] Push to test branch and verify CI matches local results
- [ ] Verify no false positives between local and CI
- [ ] Test SARIF viewer integration in VS Code
- [ ] Confirm all documentation links work

---

## Success Metrics

### Before Implementation
- ❌ Local CodeQL uses different query suite than CI (security-extended vs security-and-quality)
- ❌ Security issues pass locally but fail in CI
- ❌ No pre-commit integration for CodeQL
- ❌ No clear developer workflow for security scans
- ❌ No automated issue creation for findings

### After Implementation
- ✅ Local CodeQL uses identical parameters to CI
- ✅ Local scan results match CI 100%
- ✅ Pre-commit hooks catch HIGH/CRITICAL issues before push
- ✅ Clear documentation and workflow for developers
- ✅ CI blocks merge on high-severity findings
- ✅ Automated GitHub Issues for critical vulnerabilities (optional)

---

## Timeline Estimate

- **Phase 1 (CodeQL Alignment):** 1-2 hours
  - Update tasks.json: 30 min
  - Testing and verification: 1 hour
  - SARIF viewer setup: 30 min

- **Phase 2 (Pre-Commit Integration):** 2-3 hours
  - Create scripts: 1 hour
  - Update pre-commit config: 30 min
  - Testing: 1 hour
  - Troubleshooting: 30 min

- **Phase 3 (CI/CD Enhancement):** 1-2 hours
  - Update codeql.yml: 30 min
  - Create issue reporter (optional): 1 hour
  - Testing: 30 min

- **Phase 4 (Documentation):** 2-3 hours
  - Write security scanning guide: 1.5 hours
  - Update copilot instructions: 30 min
  - Update other docs: 1 hour

**Total Estimate:** 6-10 hours

---

## Risks & Mitigations

### Risk 1: Performance Impact on Pre-Commit
**Impact:** CodeQL scans take 2-3 minutes, slowing down commits
**Mitigation:** Use `stages: [manual]` - developers run scans on-demand, not on every commit
**Alternative:** CI catches issues, but slower feedback loop

### Risk 2: Breaking Changes to Existing Workflows
**Impact:** Developers accustomed to old tasks
**Mitigation:**
- Keep old task names with deprecation notice for 1 week
- Send announcement with migration guide
- Update all documentation immediately

### Risk 3: CI May Fail on Existing Code
**Impact:** Blocking all PRs if existing code has high-severity findings
**Mitigation:**
- Run full scan on main branch FIRST
- Fix or suppress existing findings before enforcing CI blocking
- Grandfather existing issues, block only new findings (use baseline)

### Risk 4: False Positives
**Impact:** Developers frustrated by incorrect findings
**Mitigation:**
- Document suppression syntax: `// codeql[rule-id] - Reason`
- Create triage process for false positives
- Contribute fixes to CodeQL queries if needed

---

## Rollout Plan

### Week 1: Development & Testing
- Implement Phase 1 (Tasks)
- Implement Phase 2 (Pre-Commit)
- Test on development branch

### Week 2: CI & Documentation
- Implement Phase 3 (CI Enhancement)
- Implement Phase 4 (Documentation)
- Run full scan on main branch, triage findings

### Week 3: Team Training
- Send announcement email with guide
- Hold team meeting to demo new workflow
- Create FAQ based on questions

### Week 4: Enforcement
- Enable CI blocking on HIGH/CRITICAL findings
- Monitor for issues
- Iterate on documentation

---

## References

- [CodeQL CLI Manual](https://codeql.github.com/docs/codeql-cli/)
- [CodeQL Query Suites](https://codeql.github.com/docs/codeql-cli/creating-codeql-query-suites/)
- [GitHub Actions CodeQL Action](https://github.com/github/codeql-action)
- [SARIF Format](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
- [Pre-Commit Manual Stages](https://pre-commit.com/#passing-arguments-to-hooks)

---

## Appendix A: Query Suite Comparison

### Go Queries

**security-extended (39 queries):**
- Focus: Pure security vulnerabilities
- CWE coverage: SSRF, XSS, SQL Injection, Command Injection, Path Traversal

**security-and-quality (61 queries):**
- All of security-extended PLUS:
- Code quality issues that may lead to security bugs
- Error handling problems
- Resource leaks
- Concurrency issues

**Recommendation:** Use `security-and-quality` (CI default) for comprehensive coverage

### JavaScript/TypeScript Queries

**security-extended (106 queries):**
- Focus: Web security vulnerabilities
- Covers: XSS, Prototype Pollution, CORS misconfig, Cookie security

**security-and-quality (204 queries):**
- All of security-extended PLUS:
- React/Angular/Vue specific patterns
- Async/await error handling
- Type confusion bugs
- DOM manipulation issues

**Recommendation:** Use `security-and-quality` (CI default) for comprehensive coverage

---

## Appendix B: Example SARIF Output

```json
{
  "version": "2.1.0",
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "CodeQL",
          "version": "2.16.0"
        }
      },
      "results": [
        {
          "ruleId": "go/ssrf",
          "level": "error",
          "message": {
            "text": "Untrusted URL in HTTP request"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "backend/internal/api/handlers/url_validator.go"
                },
                "region": {
                  "startLine": 45,
                  "startColumn": 10
                }
              }
            }
          ]
        }
      ]
    }
  ]
}
```

---

**END OF PLAN**

**Status:** Ready for implementation
**Next Steps:** Begin Phase 1 implementation - update `.vscode/tasks.json`
**Owner:** Development Team
**Approval Required:** Tech Lead review of CI changes (Phase 3)
