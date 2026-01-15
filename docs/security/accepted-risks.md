# Accepted Security Risks

This document tracks security vulnerabilities that have been assessed and accepted as low-risk, pending upstream patches.

---

## Alpine Linux Base Image Vulnerabilities

### CVE-2025-60876 (busybox, busybox-binsh, ssl_client)

**Status**: ⚠️ ACCEPTED - Pending Alpine Security Patch
**Date Accepted**: 2026-01-11
**Severity**: Medium
**CVSS**: TBD

#### Affected Components

- **busybox**: 1.37.0-r20
- **busybox-binsh**: 1.37.0-r20
- **ssl_client**: 1.37.0-r20

#### Vulnerability Description

CVE-2025-60876 affects multiple busybox utilities in Alpine Linux 3.21. As of 2026-01-11, no patch is available from Alpine Security Team.

#### Risk Assessment

**Exploitability**: Low

- Requires local shell access or specific network conditions
- Not directly exposed through application APIs
- Container isolation limits attack surface

**Impact**: Limited

- busybox provides minimal shell utilities used for healthchecks and diagnostics
- ssl_client used internally by Alpine package manager
- No direct user input processing through these utilities

**Mitigation Strategies**:

1. **Container Isolation**: Running in containerized environment limits local access
2. **Network Policies**: Ingress/egress rules restrict network-based exploitation
3. **Non-Privileged Container**: Runs as non-root user (caddy user)
4. **Read-Only Filesystem**: Application code and binaries mounted read-only where possible

#### Monitoring Plan

- **Frequency**: Daily checks of Alpine Security advisories
- **Source**: <https://security.alpinelinux.org/vuln>
- **Alert Trigger**: Patch release for CVE-2025-60876
- **Action**: Rebuild Docker image with updated Alpine base

#### Remediation Timeline

- **Expected Upstream Fix**: TBD (monitoring Alpine Security Team)
- **Automatic Remediation**: Will be included in next Docker rebuild after Alpine patch
- **Review Date**: 2026-02-11 (30 days) or upon patch release, whichever is sooner

---

### CVE-2025-10966 (curl/libcurl)

**Status**: ⚠️ ACCEPTED - Pending Alpine Security Patch
**Date Accepted**: 2026-01-11
**Severity**: Medium
**CVSS**: TBD

#### Affected Components

- **curl**: 8.14.1-r2
- **libcurl**: 8.14.1-r2 (implicit)

#### Vulnerability Description

CVE-2025-10966 affects libcurl in Alpine Linux 3.21. As of 2026-01-11, no patch is available from Alpine Security Team.

#### Risk Assessment

**Exploitability**: Medium

- Requires network access and specific request patterns
- curl used only in healthcheck scripts and manual debugging
- Not exposed directly to user input

**Impact**: Limited

- curl invoked only for internal health monitoring
- No user-controlled URLs passed to curl
- Healthcheck scripts use hardcoded localhost endpoints

**Mitigation Strategies**:

1. **Limited Usage**: curl only used for internal healthchecks (`http://localhost:8080/api/v1/health`)
2. **No User Input**: All curl invocations use hardcoded, internal URLs
3. **Container Isolation**: Network policies restrict external access
4. **Alternative Available**: Application can fall back to TCP socket checks

#### Monitoring Plan

- **Frequency**: Daily checks of Alpine Security advisories
- **Source**: <https://security.alpinelinux.org/vuln>
- **Alert Trigger**: Patch release for CVE-2025-10966
- **Action**: Rebuild Docker image with updated Alpine base

#### Remediation Timeline

- **Expected Upstream Fix**: TBD (monitoring Alpine Security Team)
- **Automatic Remediation**: Will be included in next Docker rebuild after Alpine patch
- **Review Date**: 2026-02-11 (30 days) or upon patch release, whichever is sooner

---

## Review Schedule

### Quarterly Security Review

- **Next Review**: 2026-04-11
- **Scope**: Re-assess all accepted risks, evaluate alternative base images
- **Attendees**: Security team, DevOps, Engineering Director

### Monthly Monitoring

- **Frequency**: First Monday of each month
- **Scope**: Check Alpine and upstream security advisories
- **Action**: Update this document if status changes

### Continuous Monitoring

- **Automated**: GitHub Dependabot, Renovate Bot
- **Manual**: Daily check of Alpine security feed during active incident periods

---

## Escalation Criteria

Accepted risks will be escalated to immediate remediation if:

1. **Severity Upgrade**: CVE severity upgraded to High or Critical
2. **Active Exploitation**: Evidence of active exploitation in the wild
3. **CISA KEV**: Added to CISA Known Exploited Vulnerabilities catalog
4. **Proof of Concept**: Public PoC demonstrating exploitability in containers
5. **Compliance Requirement**: Regulatory or audit requirement to remediate

---

## Alternative Mitigation Considered

### Switch to Distroless Base Image

**Status**: Under Evaluation
**Timeline**: Q1 2026

**Pros**:

- Minimal attack surface (no shell, no package manager)
- Faster security patches from Google
- Smaller image size

**Cons**:

- Debugging challenges (no shell access)
- May require custom healthcheck mechanisms
- Migration effort required

**Decision**: Continue monitoring Alpine CVEs while evaluating distroless for Q1 2026.

---

## Approval

**Approved By**: Engineering Director
**Date**: 2026-01-11
**Review Scheduled**: 2026-02-11

**Rationale**: The assessed risk from these Medium-severity Alpine CVEs is acceptable given:

1. Low exploitability in containerized environment
2. No upstream patches available
3. Effective mitigation strategies in place
4. Active monitoring for patches
5. No critical or high-severity vulnerabilities present

---

## References

- [Alpine Linux Security](https://security.alpinelinux.org/)
- [CVE-2025-60876 Details](https://nvd.nist.gov/vuln/detail/CVE-2025-60876) (pending NVD update)
- [CVE-2025-10966 Details](https://nvd.nist.gov/vuln/detail/CVE-2025-10966) (pending NVD update)
- [Supply Chain Remediation Plan](./supply-chain-no-cache-solution.md)
- [NIST SP 800-53: Security Controls](https://csrc.nist.gov/publications/detail/sp/800-53/rev-5/final)
