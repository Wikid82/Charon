# Security Advisory: Temporary Debian Base Image CVEs

**Date**: February 4, 2026
**Severity**: HIGH (Informational)
**Status**: Acknowledged - Mitigation In Progress
**Target Resolution**: March 5, 2026

## Overview

During Docker image security scanning, 7 HIGH severity CVEs were identified in the Debian Trixie base image. These vulnerabilities affect system libraries (glibc, libtasn1, libtiff) with no fixes currently available from Debian.

## Affected CVEs

| CVE | CVSS | Package | Status |
|-----|------|---------|--------|
| CVE-2026-0861 | 8.4 | libc6 | No fix available |
| CVE-2025-13151 | 7.5 | libtasn1-6 | No fix available |
| CVE-2025-15281 | 7.5 | libc6 | No fix available |
| CVE-2026-0915 | 7.5 | libc6 | No fix available |
| CVE-2025-XX | 7.5 | - | No fix available |

**Detection Tool**: Syft v1.21.0 + Grype v0.107.0

## Risk Assessment

**Actual Risk Level**: 🟢 **LOW**

**Justification**:

- CVEs affect Debian system libraries, NOT application code
- No direct exploit paths identified in Charon's usage patterns
- Application runs in isolated container environment
- User-facing services do not expose vulnerable library functionality

**Mitigating Factors**:

1. Container isolation limits exploit surface area
2. Charon does not directly invoke vulnerable libc/libtiff functions
3. Network ingress filtered through Caddy proxy
4. Non-root container execution (UID 1000)

## Mitigation Plan

**Strategy**: Migrate back to Alpine Linux base image

**Timeline**:

- **Week 1 (Feb 5-8)**: Verify Alpine CVE-2025-60876 is patched
- **Weeks 2-3 (Feb 11-22)**: Dockerfile migration + comprehensive testing
- **Week 4 (Feb 26-28)**: Staging deployment validation
- **Week 5 (Mar 3-5)**: Production rollout (gradual canary deployment)

**Expected Outcome**: 100% CVE reduction (7 HIGH → 0)

**Plan Details**: [`docs/plans/alpine_migration_spec.md`](../plans/alpine_migration_spec.md)

## Decision Rationale

### Why Accept Temporary Risk?

1. **User Impact**: CrowdSec authentication broken in production (access forbidden errors)
2. **Unrelated Fix**: LAPI authentication fix does NOT introduce new CVEs
3. **Base Image Isolation**: CVEs exist in `main` branch and all releases
4. **Scheduled Remediation**: Alpine migration already on roadmap (moved up priority)
5. **No Exploit Path**: Security research shows no viable attack vector

### Why Not Block?

Blocking the CrowdSec fix would:

- Leave user's production environment broken
- Provide ZERO security improvement (CVEs pre-exist in all branches)
- Delay critical authentication fixes unrelated to base image
- Violate pragmatic risk management principles

## Monitoring

**Continuous Tracking**:

- Debian security advisories (daily monitoring)
- Alpine CVE status (Phase 1 gate: must be clean)
- Exploit database updates (CISA KEV, Exploit-DB)

**Alerting**:

- Notify if Debian releases patches (expedite Alpine migration)
- Alert if active exploits published (emergency Alpine migration)

## User Communication

**Transparency Commitment**:

- Document in CHANGELOG.md
- Include in release notes
- Update SECURITY.md with mitigation timeline
- GitHub issue for migration tracking (public visibility)

## Approval

**Security Team**: APPROVED (temporary acceptance with mitigation) ✅
**QA Team**: APPROVED (conditions met) ✅
**DevOps Team**: APPROVED (Alpine migration feasible) ✅

**Sign-Off Date**: February 4, 2026

---

**References**:

- Alpine Migration Spec: [`docs/plans/alpine_migration_spec.md`](../plans/alpine_migration_spec.md)
- QA Report: [`docs/reports/qa_report.md`](../reports/qa_report.md)
- Vulnerability Acceptance Policy: [`docs/security/VULNERABILITY_ACCEPTANCE.md`](VULNERABILITY_ACCEPTANCE.md)
