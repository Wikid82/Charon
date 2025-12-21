# Issue #365: Additional Security Enhancements - Implementation Specification

**Status**: Planning Complete
**Created**: 2025-12-21
**Issue**: https://github.com/Wikid82/Charon/issues/365
**Branch**: `feature/issue-365-additional-security`
**PRs**: #436 (targets `development`), #437 (targets `main`)

---

## Executive Summary

This specification details the implementation plan for Issue #365, which addresses six security enhancement areas. After thorough codebase analysis, this document provides file-by-file implementation details, phase breakdown, and complexity estimates.

### Scope Summary

| Requirement | Status | Complexity | Phase |
|-------------|--------|------------|-------|
| Supply Chain - SBOM Generation | ❌ TODO | Medium | 2 |
| DNS Hijacking - Documentation | ❌ TODO | Low | 1 |
| TLS Downgrade - Documentation | ✅ Partial | Low | 1 |
| Privilege Escalation - Container Hardening | ⚠️ Partial | Medium | 2 |
| Session Hijacking - Cookie/CSP Audit | ✅ Implemented | Low | 1 |
| Timing Attacks - Constant-Time Comparison | ❌ TODO | Medium | 2 |
| SIRP Documentation | ❌ TODO | Low | 1 |
| Security Update Notifications Doc | ❌ TODO | Low | 1 |

---

## Codebase Research Findings

### 1. Cookie/Session Implementation

**Location**: [backend/internal/api/handlers/auth_handler.go](backend/internal/api/handlers/auth_handler.go)

**Current Implementation** (lines 52-73):
```go
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
    scheme := requestScheme(c)
    secure := isProduction() && scheme == "https"
    sameSite := http.SameSiteStrictMode
    if scheme != "https" {
        sameSite = http.SameSiteLaxMode
    }
    c.SetSameSite(sameSite)
    c.SetCookie(name, value, maxAge, "/", "", secure, true) // HttpOnly: true ✅
}
```

**Assessment**: ✅ **SECURE** - All cookie security attributes properly configured:
- `HttpOnly: true` - Prevents XSS access
- `Secure: true` (production + HTTPS)
- `SameSite: Strict` (HTTPS) / `Lax` (HTTP dev)

### 2. Security Headers Implementation

**Location**: [backend/internal/api/middleware/security.go](backend/internal/api/middleware/security.go)

**Current Headers Set**:
- ✅ Content-Security-Policy (CSP)
- ✅ Strict-Transport-Security (HSTS) with preload
- ✅ X-Frame-Options: DENY
- ✅ X-Content-Type-Options: nosniff
- ✅ X-XSS-Protection: 1; mode=block
- ✅ Referrer-Policy: strict-origin-when-cross-origin
- ✅ Permissions-Policy (restricts camera, mic, etc.)
- ✅ Cross-Origin-Opener-Policy: same-origin
- ✅ Cross-Origin-Resource-Policy: same-origin

**Assessment**: ✅ **COMPREHENSIVE** - All major security headers present.

### 3. Token Comparison Methods

**Locations Analyzed**:

| File | Function | Method | Status |
|------|----------|--------|--------|
| [models/user.go#L62](backend/internal/models/user.go#L62) | `CheckPassword` | `bcrypt.CompareHashAndPassword` | ✅ Constant-time |
| [services/security_service.go#L151](backend/internal/services/security_service.go#L151) | `VerifyBreakGlassToken` | `bcrypt.CompareHashAndPassword` | ✅ Constant-time |
| [services/auth_service.go#L128](backend/internal/services/auth_service.go#L128) | `ValidateToken` | JWT library | ✅ Handled by library |

**Potential Issue Found**: Invite token validation uses database lookup which could leak timing:
- Location: `backend/internal/api/handlers/user_handler.go` (AcceptInvite)
- Risk: Low (requires network timing analysis)
- Recommendation: Add constant-time wrapper for token comparison after DB lookup

### 4. Current Security Documentation

**Location**: [docs/security.md](docs/security.md)

**Current Coverage**:
- ✅ Cerberus security suite (CrowdSec, WAF, ACL)
- ✅ Access Lists configuration
- ✅ Certificate management
- ✅ Break-glass token
- ✅ Zero-day protection explanation
- ❌ TLS version enforcement (not documented)
- ❌ DNS security (not documented)
- ❌ SIRP (not documented)
- ❌ Container hardening (not documented)

### 5. CI/CD Pipeline Analysis

**Location**: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)

**Current State**:
- ✅ Trivy vulnerability scanning (SARIF + table output)
- ✅ Weekly security rebuilds (separate workflow)
- ✅ Caddy security patches verification
- ❌ SBOM generation (not implemented)
- ❌ SBOM attestation (not implemented)

**Best Integration Point**: After `build-and-push` step, before Trivy scan (around line 130)

### 6. Dockerfile Security Analysis

**Location**: [Dockerfile](Dockerfile)

**Current Security Features**:
- ✅ Non-root user (`charon:charon`, UID 1000)
- ✅ Healthcheck configured
- ✅ Minimal base image (Alpine)
- ✅ Multi-stage build (reduces attack surface)
- ⚠️ Root filesystem writable (can be improved)
- ⚠️ All capabilities retained (can drop unnecessary ones)

---

## Phase 1: Documentation Enhancements (Estimated: 1-2 hours)

### 1.1 TLS Downgrade Attack Documentation

**File**: `docs/security.md`

**Add Section**:
```markdown
## TLS Security

### TLS Version Enforcement

Charon (via Caddy) enforces a minimum TLS version of 1.2 by default. This prevents TLS downgrade attacks that attempt to force connections to use vulnerable TLS 1.0 or 1.1.

**What's Protected:**
- ✅ TLS 1.0/1.1 downgrade attacks
- ✅ BEAST, POODLE, and similar protocol-level attacks
- ✅ Weak cipher suite negotiation

**HSTS (HTTP Strict Transport Security):**
Charon sets HSTS headers with:
- `max-age=31536000` (1 year)
- `includeSubDomains`
- `preload` (for browser preload lists)
```

**Complexity**: Low | **Dependencies**: None

---

### 1.2 DNS Hijacking Documentation

**File**: `docs/security.md`

**Add Section**:
```markdown
## DNS Security

### Protecting Against DNS Hijacking

Configure your upstream resolver to use DNS-over-HTTPS (DoH) or DNS-over-TLS (DoT):

**Docker Host Configuration:**
```bash
# /etc/systemd/resolved.conf
[Resolve]
DNS=1.1.1.1#cloudflare-dns.com
DNSOverTLS=yes
```

**Additional Protections:**
1. **DNSSEC**: Ensure your domain registrar supports DNSSEC
2. **CAA Records**: Restrict which CAs can issue certs for your domain
```

**Complexity**: Low | **Dependencies**: None

---

### 1.3 Security Incident Response Plan

**New File**: `docs/security-incident-response.md`

**Content**: Incident classification, detection, containment, recovery procedures

**Complexity**: Low | **Dependencies**: None

---

### 1.4 Security Update Notifications

**Files**: `docs/getting-started.md`, `docs/security.md`

**Add**: GitHub Watch instructions, Watchtower/Diun configuration examples

**Complexity**: Low | **Dependencies**: None

---

## Phase 2: Code Changes (Estimated: 4-6 hours)

### 2.1 SBOM Generation

**File**: `.github/workflows/docker-build.yml`

**Changes** (add after build-and-push step):
```yaml
- name: Generate SBOM (CycloneDX)
  uses: anchore/sbom-action@v0
  with:
    image: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}
    format: cyclonedx-json
    output-file: sbom.cyclonedx.json

- name: Attest SBOM to Image
  if: github.event_name != 'pull_request'
  uses: actions/attest-sbom@v2
  with:
    subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    subject-digest: ${{ steps.build-and-push.outputs.digest }}
    sbom-path: sbom.cyclonedx.json
```

**Additional Files**:
- `.gitignore`: Add `sbom*.json`
- `.dockerignore`: Add `sbom*.json`

**Complexity**: Medium | **Dependencies**: None

---

### 2.2 Container Hardening Documentation

**File**: `docs/security.md`

**Add Example**:
```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    read_only: true
    tmpfs:
      - /tmp:size=100M
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
```

**Complexity**: Medium | **Dependencies**: Testing with read-only FS

---

### 2.3 Constant-Time Token Comparison

**New File**: `backend/internal/util/crypto.go`
```go
package util

import "crypto/subtle"

// ConstantTimeCompare compares two strings in constant time.
func ConstantTimeCompare(a, b string) bool {
    return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

**New Test File**: `backend/internal/util/crypto_test.go`

**Modify**: `backend/internal/api/handlers/user_handler.go` - Use constant-time comparison in AcceptInvite

**Complexity**: Medium | **Dependencies**: None

---

## Phase 3: Testing & Validation (Estimated: 2-3 hours)

### Required Tests

| File | Purpose |
|------|---------|
| `backend/internal/util/crypto_test.go` | Constant-time comparison tests + benchmarks |
| Integration test additions | Security header verification |

### Coverage Requirements

- All new code must achieve 85% coverage
- Run: `scripts/go-test-coverage.sh`

---

## File Change Summary

### Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/util/crypto.go` | Constant-time comparison utilities |
| `backend/internal/util/crypto_test.go` | Tests for crypto utilities |
| `docs/security-incident-response.md` | SIRP documentation |

### Files to Modify

| File | Changes |
|------|---------|
| `docs/security.md` | TLS, DNS, container hardening sections |
| `docs/getting-started.md` | Security update notification section |
| `.github/workflows/docker-build.yml` | SBOM generation steps |
| `.gitignore` | Add `sbom*.json` |
| `.dockerignore` | Add `sbom*.json` |
| `backend/internal/api/handlers/user_handler.go` | Constant-time token comparison |

### Files Verified Secure (No Changes)

| File | Reason |
|------|--------|
| `backend/internal/api/handlers/auth_handler.go` | Cookie security correct |
| `backend/internal/api/middleware/security.go` | Headers comprehensive |
| `backend/internal/models/user.go` | bcrypt is constant-time |
| `backend/internal/services/security_service.go` | bcrypt is constant-time |
| `Dockerfile` | Non-root user configured |

---

## Out of Scope (Future Issues)

Per Issue #365:
- ❌ Certificate Transparency Log Monitoring
- ❌ MFA via Authentik
- ❌ SSO for Charon admin
- ❌ Audit logging for GDPR/SOC2

---

## Acceptance Criteria

- [ ] Documentation sections added to `docs/security.md`
- [ ] SIRP document created
- [ ] SBOM generation in CI (CycloneDX format)
- [ ] Constant-time utility with tests
- [ ] Container hardening documented
- [ ] 85%+ test coverage
- [ ] Pre-commit hooks pass

---

**Analysis Date**: 2025-12-21
**Analyzed By**: GitHub Copilot
**Status**: Ready for Implementation
