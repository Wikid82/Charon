# Issue #365: Additional Security Enhancements - Implementation Status

**Research Date**: December 23, 2025
**Issue**: <https://github.com/Wikid82/Charon/issues/365>
**Related PRs**: #436, #437, #438
**Main Implementation Commit**: `2dfe7ee` (merged via PR #438)

---

## Executive Summary

Issue #365 addressed multiple security enhancements across supply chain security, timing attacks, documentation, and incident response. The implementation is **mostly complete** with one notable rollback and one remaining verification task.

**Status Overview**:

- ✅ **Completed**: 5 of 7 primary objectives
- ⚠️ **Rolled Back**: 1 item (constant-time token comparison - see details below)
- 📋 **Verification Pending**: 1 item (CSP header implementation)

---

## Completed Items (With Evidence)

### 1. ✅ SBOM Generation and Attestation

**Status**: Fully implemented and operational

**Evidence**:

- **File**: `.github/workflows/docker-build.yml` (lines 236-252)
- **Implementation Details**:
  - Uses `anchore/sbom-action@61119d458adab75f756bc0b9e4bde25725f86a7a` (v0.17.2)
  - Generates CycloneDX JSON format SBOM for all Docker images
  - Creates verifiable attestations using `actions/attest-sbom@115c3be05ff3974bcbd596578934b3f9ce39bf68` (v2.2.0)
  - Pushes attestations to GitHub Container Registry
  - Only runs on non-PR builds (skips pull requests)
  - Permissions configured: `id-token: write`, `attestations: write`

**Verification**:

```bash
# Check workflow file
grep -A 20 "Generate SBOM" .github/workflows/docker-build.yml

# Verify on GitHub
# Navigate to: https://github.com/Wikid82/Charon/pkgs/container/charon
# Check for "Attestations" tab on container image
```

**Gitignore Protection**: SBOM artifacts (`.gitignore` line 233-235, `.dockerignore` lines 169-171)

---

### 2. ✅ Security Incident Response Plan (SIRP)

**Status**: Complete documentation created

**Evidence**:

- **File**: `docs/security-incident-response.md` (400 lines)
- **Created**: December 21, 2025
- **Version**: 1.0

**Contents**:

- Incident classification (P1-P4 severity levels)
- Detection methods (automated dashboard monitoring, log analysis)
- Containment procedures with executable commands
- Recovery steps with verification checkpoints
- Post-incident review templates
- Communication templates (internal, external, user-facing)
- Emergency contact framework
- Quick reference card with key commands

**Integration Points**:

- References Cerberus Dashboard for live monitoring
- Integrates with CrowdSec decision management
- Documents Docker container forensics procedures
- Links to automated security alerting systems

---

### 3. ✅ TLS Security Documentation

**Status**: Comprehensive documentation added to `docs/security.md`

**Evidence**:

- **File**: `docs/security.md` (lines ~755-788)
- **Section**: "TLS Security"

**Content**:

- TLS 1.2+ enforcement (via Caddy default configuration)
- Protection against downgrade attacks (BEAST, POODLE)
- HSTS header configuration with preload
  - `max-age=31536000` (1 year)
  - `includeSubDomains`
  - `preload` flag for browser preload lists

**Technical Implementation**:

- Caddy enforces TLS 1.2+ by default (no additional configuration needed)
- HSTS headers automatically added in HTTPS mode
- Load balancer header forwarding requirements documented

---

### 4. ✅ DNS Security Documentation

**Status**: Complete deployment guidance provided

**Evidence**:

- **File**: `docs/security.md` (lines ~790-823)
- **Section**: "DNS Security"

**Content**:

- DNS hijacking and cache poisoning protection strategies
- Docker host configuration for encrypted DNS (DoH/DoT)
- Example systemd-resolved configuration
- Alternative DNS providers (Cloudflare, Google, Quad9)
- DNSSEC enablement at domain registrar
- CAA record recommendations

**Example Configuration**:

```bash
# /etc/systemd/resolved.conf
[Resolve]
DNS=1.1.1.1#cloudflare-dns.com 1.0.0.1#cloudflare-dns.com
DNSOverTLS=yes
```

---

### 5. ✅ Container Hardening Documentation

**Status**: Production-ready Docker security configuration documented

**Evidence**:

- **File**: `docs/security.md` (lines ~825-860)
- **Section**: "Container Hardening"

**Content**:

- Read-only root filesystem configuration
- Capability dropping (cap_drop: ALL, cap_add: NET_BIND_SERVICE)
- tmpfs mounts for writable directories
- no-new-privileges security option
- Complete docker-compose.yml example

**Example**:

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    read_only: true
    tmpfs:
      - /tmp:size=100M
      - /config:size=50M
      - /data/logs:size=100M
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
```

---

### 6. ✅ Security Update Notification Documentation

**Status**: Multiple notification methods documented

**Evidence**:

- **File**: `docs/getting-started.md` (lines 399-430)
- **Section**: "Security Update Notifications"

**Content**:

- GitHub Watch configuration for security advisories
- Watchtower for automatic updates
  - Example docker-compose.yml configuration
  - Daily polling interval
  - Automatic cleanup
- Diun (Docker Image Update Notifier) for notification-only mode
- Best practices:
  - Subscribe to GitHub security advisories
  - Review changelogs before production updates
  - Test in staging environments
  - Maintain backups before upgrades

---

## Rolled Back / Modified Items

### 7. ⚠️ Constant-Time Token Comparison

**Initial Status**: Implemented in commit `2dfe7ee` (December 21, 2025)

**Implementation**:

- **Files Created**:
  - `backend/internal/util/crypto.go` (21 lines)
  - `backend/internal/util/crypto_test.go` (82 lines)
- **Functions**:
  - `util.ConstantTimeCompare(a, b string) bool`
  - `util.ConstantTimeCompareBytes(a, b []byte) bool`
  - Uses Go's `crypto/subtle.ConstantTimeCompare`

**Rollback**: Removed in commit `8a7b939` (December 22, 2025)

**Reason for Rollback**:
According to `docs/plans/codecov-acceptinvite-patch-coverage.md`:

1. **Unreachable Code**: The DB query in `AcceptInvite` already filters by `WHERE invite_token = req.Token`
2. **Defense-in-Depth Redundant**: If a user is found, `user.InviteToken` already equals `req.Token`
3. **Oracle Risk**: Having a separate 401 response for token mismatch (vs 404 for not found) could create a timing oracle
4. **Coverage Impact**: The constant-time comparison branch was unreachable, causing Codecov patch coverage to fail at 66.67%

**Current State**:

- ✅ Utility functions remain available in `backend/internal/util/crypto.go`
- ✅ Comprehensive test coverage in `backend/internal/util/crypto_test.go`
- ❌ NOT used in `backend/internal/api/handlers/user_handler.go` (removed from AcceptInvite handler)
- ⚠️ Utility is available for future use where constant-time comparison is genuinely needed

**Security Analysis**:
The rollback is **security-neutral** because:

- The DB query already provides the primary defense (token lookup)
- String comparison timing variance is negligible compared to DB query timing
- Avoiding different HTTP status codes (401 vs 404) eliminates a potential oracle
- The utility remains available for scenarios where constant-time comparison is beneficial

**Recommendation**: Keep utility functions but do NOT re-introduce to `AcceptInvite` handler. Consider using for:

- API key validation
- Webhook signature verification
- Any scenario where both values are in-memory and timing could leak information

---

## Verification Pending

### 8. 📋 CSP (Content-Security-Policy) Headers

**Status**: Implementation unclear - requires verification

**Expected Implementation**:
According to Issue #365 plan, CSP headers should be implemented in the backend to protect against XSS attacks.

**Evidence Found**:

- **Documentation**: Extensive CSP documentation exists in `docs/features.md` (lines 1167-1583)
  - Interactive CSP builder documentation
  - CSP configuration guidance
  - Report-Only mode recommendations
  - Template-based CSP (Secure, Strict, Custom modes)
- **Backend Code**: CSP infrastructure exists but usage in middleware is unclear
  - `backend/internal/models/security_header_profile.go` - CSP field defined
  - `backend/internal/services/security_headers_service*.go` - CSP service implementation
  - `backend/internal/services/security_score.go` - CSP scoring (25 points)
  - `backend/internal/caddy/types*.go` - CSP header application to proxy hosts

**What Needs Verification**:

1. ✅ **Proxy Host Level**: CSP headers ARE applied to individual proxy hosts via security header profiles (confirmed in code)
2. ❓ **Charon Admin UI**: Are CSP headers applied to Charon's own admin interface?
   - Check: `backend/internal/api/middleware/` for CSP middleware
   - Check: Response headers when accessing Charon admin UI (port 8080)
3. ❓ **Default Security Headers**: Does Charon set secure-by-default headers for its own endpoints?

**Verification Commands**:

```bash
# Check if CSP middleware exists in backend
grep -r "Content-Security-Policy" backend/internal/api/middleware/

# Test Charon admin UI headers
curl -I http://localhost:8080/ | grep -i "content-security-policy"

# Check for security header middleware application
grep -A 10 "SecurityHeaders" backend/internal/api/routes.go
```

**Expected Outcome**:

- [ ] Confirm CSP headers are applied to Charon's admin UI
- [ ] Document default CSP policy for admin interface
- [ ] Verify headers include: X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy
- [ ] Test that headers are present in both HTTP (development) and HTTPS (production) modes

---

## Items Not Started (Out of Scope)

Per the original Issue #365 plan, these were explicitly marked as **Future Issues**:

1. ❌ Multi-factor authentication (MFA) via Authentik
2. ❌ SSO for Charon admin
3. ❌ Audit logging for compliance (GDPR, SOC 2)
4. ❌ Certificate Transparency (CT) log monitoring

These remain **out of scope** and should be tracked as separate issues.

---

## Recommended Next Steps

### Immediate (High Priority)

1. **Verify CSP Implementation for Admin UI**
   - Run verification commands listed above
   - Document findings in a follow-up issue or comment on #365
   - If missing, create subtask: "Add CSP headers to Charon admin interface"

2. **Manual Testing Execution**
   - Execute manual test plan from `docs/issues/created/20251221-issue-365-manual-test-plan.md`
   - Test scenarios 1 (timing attacks - N/A after rollback), 2 (security headers), 4 (documentation review), 5 (SBOM generation)
   - Document results

### Short-Term (Medium Priority)

1. **Security Header Middleware Audit**
   - Verify all security headers are applied consistently:
     - Strict-Transport-Security (HSTS)
     - X-Frame-Options
     - X-Content-Type-Options
     - Referrer-Policy
     - Permissions-Policy
     - Content-Security-Policy
   - Check for proper HTTPS detection (X-Forwarded-Proto)

2. **Update Documentation**
   - Add note to `docs/security.md` explaining constant-time comparison utility availability
   - Document why it's not used in AcceptInvite (reference coverage plan)
   - Update Issue #365 to reflect rollback

### Long-Term (Low Priority)

1. **Consider Re-Using Constant-Time Comparison**
   - Identify endpoints where constant-time comparison would be genuinely beneficial
   - Examples: API key validation, webhook signatures, session token verification
   - Document use cases in crypto utility comments

2. **Security Hardening Testing**
   - Test container hardening configuration in production-like environment
   - Verify read-only filesystem doesn't break functionality
   - Document any tmpfs mount size adjustments needed

---

## Testing Checklist

From `docs/issues/created/20251221-issue-365-manual-test-plan.md`:

- [ ] ~~Scenario 1: Invite Token Security (timing attacks)~~ - N/A after rollback
- [ ] **Scenario 2: Security Headers Verification** - REQUIRED
  - [ ] Verify Content-Security-Policy header
  - [ ] Verify Strict-Transport-Security header
  - [ ] Verify X-Frame-Options: DENY
  - [ ] Verify X-Content-Type-Options: nosniff
  - [ ] Verify Referrer-Policy header
  - [ ] Verify Permissions-Policy header
- [ ] ~~Scenario 3: Container Hardening~~ - Optional (production deployment testing)
- [ ] **Scenario 4: Documentation Review** - REQUIRED
  - [ ] `docs/security.md` - TLS, DNS, Container Hardening sections
  - [ ] `docs/security-incident-response.md` - SIRP document
  - [ ] `docs/getting-started.md` - Security Update Notifications section
- [ ] **Scenario 5: SBOM Generation (CI/CD)** - REQUIRED
  - [ ] Verify GitHub Actions workflow includes SBOM generation
  - [ ] Check "Generate SBOM" step in workflow runs
  - [ ] Check "Attest SBOM" step in workflow runs
  - [ ] Verify attestation visible in GitHub Container Registry

---

## Files Changed (Summary)

**Original Implementation (commit `2dfe7ee`)**:

- `.dockerignore` - Added SBOM artifacts exclusion
- `.github/workflows/docker-build.yml` - Added SBOM generation steps
- `.gitignore` - Added SBOM artifacts exclusion
- `backend/internal/api/handlers/user_handler.go` - Added constant-time comparison (later removed)
- `backend/internal/util/crypto.go` - Created constant-time utility (KEPT)
- `backend/internal/util/crypto_test.go` - Created tests (KEPT)
- `docs/getting-started.md` - Added security update notifications
- `docs/issues/created/20251221-issue-365-manual-test-plan.md` - Created test plan
- `docs/security-incident-response.md` - Created SIRP document
- `docs/security.md` - Added TLS, DNS, and container hardening sections

**Rollback (commit `8a7b939`)**:

- `backend/internal/api/handlers/user_handler.go` - Removed constant-time comparison usage
- `docs/plans/codecov-acceptinvite-patch-coverage.md` - Created explanation document

**Current State**:

- ✅ 11 files remain changed (from original implementation)
- ⚠️ 1 file rolled back (user_handler.go)
- ✅ Utility functions preserved for future use

---

## Conclusion

Issue #365 achieved **71% completion** (5 of 7 objectives) with high-quality implementation:

**Strengths**:

- Comprehensive documentation (SIRP, TLS, DNS, container hardening)
- Supply chain security (SBOM + attestation)
- Security update guidance
- Reusable cryptographic utilities

**Outstanding**:

- CSP header verification for admin UI (high priority)
- Manual testing execution
- Constant-time comparison usage evaluation (find appropriate use cases)

**Recommendation**: Consider Issue #365 **substantially complete** after CSP verification. Any additional constant-time comparison usage should be tracked as a separate enhancement issue if needed.

---

**Document Version**: 1.0
**Last Updated**: December 23, 2025
**Researcher**: AI Assistant (GitHub Copilot)
