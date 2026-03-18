# QA Security Scan Report

**Date**: 2026-03-18
**Scope**: Charon project — filesystem + Docker image
**Scanners**: Trivy (filesystem), Grype (Docker image via `security-scan-docker-image` skill)
**Previous scan data reviewed**: `trivy-report.json`, `trivy-image-report.json`, `grype-results.json`, `vuln-results.json`

---

## Executive Summary

The CI supply chain run flagged **2 HIGH severity vulnerabilities**. Both are the same CVE affecting two sibling OpenSSL packages in the Alpine 3.23.3 base image. **Neither has a fixed Alpine package version available as of the scan date.** This is an upstream-blocked situation requiring monitoring, not an immediately actionable code change.

No CRITICAL findings exist in any scan component (filesystem, Go modules, npm, or Docker image).

---

## Findings

### Finding 1 — CVE-2026-2673 [HIGH] in `libcrypto3`

| Field | Value |
|-------|-------|
| CVE | CVE-2026-2673 |
| Severity | HIGH (CVSS 7.5) |
| Package | `libcrypto3` |
| Installed Version | `3.5.5-r0` |
| Fixed Version | **None available** |
| Fix State | Unknown / Upstream-pending |
| Component | Docker image final stage (Alpine 3.23.3 APK) |
| Scanner | Grype `security-scan-docker-image` |
| Advisory Published | 2026-03-13 |

**Description**: An OpenSSL TLS 1.3 server may fail to negotiate the expected preferred key exchange group when its key exchange group configuration includes the `DEFAULT` keyword. This can result in weaker cipher negotiation than intended, potentially enabling downgrade attacks on TLS connections.

**References**:
- https://openssl-library.org/news/secadv/20260313.txt
- https://github.com/openssl/openssl/commit/2157c9d81f7b0bd7dfa25b960e928ec28e8dd63f
- https://github.com/openssl/openssl/commit/85977e013f32ceb96aa034c0e741adddc1a05e34
- http://www.openwall.com/lists/oss-security/2026/03/13/3

---

### Finding 2 — CVE-2026-2673 [HIGH] in `libssl3`

| Field | Value |
|-------|-------|
| CVE | CVE-2026-2673 |
| Severity | HIGH (CVSS 7.5) |
| Package | `libssl3` |
| Installed Version | `3.5.5-r0` |
| Fixed Version | **None available** |
| Fix State | Unknown / Upstream-pending |
| Component | Docker image final stage (Alpine 3.23.3 APK) |
| Scanner | Grype `security-scan-docker-image` |
| Advisory Published | 2026-03-13 |

**Description**: Same CVE as Finding 1. `libssl3` and `libcrypto3` are sibling packages that constitute Alpine's OpenSSL 3.5.5 installation. Both packages must be patched together.

---

## Classification

| CVE | Package | Classification | Reason |
|-----|---------|----------------|--------|
| CVE-2026-2673 | libcrypto3@3.5.5-r0 | **Waiting on Upstream** | No fixed Alpine APK available; advisory published 5 days ago |
| CVE-2026-2673 | libssl3@3.5.5-r0 | **Waiting on Upstream** | Same CVE, same upstream blocking condition |

---

## Historical Finding (Resolved)

### CVE-2026-25793 [HIGH] in `github.com/slackhq/nebula` — **RESOLVED**

| Field | Value |
|-------|-------|
| CVE | CVE-2026-25793 |
| Severity | HIGH |
| Package | `github.com/slackhq/nebula` |
| Vulnerable Version | v1.9.7 |
| Fixed Version | v1.10.3 |
| Component | `usr/bin/caddy` (Go binary) |
| Status | **Resolved** |

This finding appeared in the `trivy-image-report.json` scan from 2026-02-25, when the Dockerfile used `CADDY_PATCH_SCENARIO=A`, which explicitly pinned nebula to v1.9.7. The Dockerfile was updated to `CADDY_PATCH_SCENARIO=B` (see `Dockerfile:42`), which skips the explicit nebula pin and allows upstream resolution. The finding does not appear in the current (2026-03-18) Docker image scan.

---

## Scan Coverage Summary

| Scan Target | Scanner | HIGH | CRITICAL | Notes |
|-------------|---------|------|----------|-------|
| Filesystem (Go modules, npm, config) | Trivy | 0 | 0 | Clean |
| Docker image (APK packages) | Grype | 2 | 0 | CV-2026-2673 ×2 |
| Docker image (Go binaries) | Grype | 0 | 0 | Nebula CVE resolved |
| Go backend (grype-results.json) | Grype | 0 | 0 | Clean |

---

## Root Cause Analysis

The two HIGH findings share a single root cause: Alpine Linux has not yet published a patched `openssl` package for CVE-2026-2673. The advisory was disclosed on 2026-03-13 (5 days before this scan). The upstream OpenSSL commits exist, but Alpine's package maintainers have not yet issued an `openssl-3.5.x-r1` or newer release.

The Charon Dockerfile pins to `alpine:3.23.3@sha256:2510...` (see `Dockerfile:16`). The final runtime stage installs OpenSSL indirectly as a dependency of `ca-certificates` and other system libs. The existing `apk upgrade --no-cache zlib` on the final stage line 422 targets only zlib and would not pick up an OpenSSL fix even if one were available.

---

## Recommended Actions

### Immediate (No action possible yet)

No code change can resolve CVE-2026-2673 today. Both packages lack a fixed version in Alpine's package repository.

**Monitor**:
- Alpine Linux security tracker: https://security.alpinelinux.org/vuln/CVE-2026-2673
- Alpine 3.23 changelogs for an `openssl-3.5.5-r1` or later release

### When Alpine Releases a Patch

One of the following approaches will resolve both findings simultaneously:

**Option A — Update the pinned base image** (preferred for reproducibility):
```dockerfile
# In Dockerfile, update ARG ALPINE_IMAGE to the new digest when Alpine patches it
ARG ALPINE_IMAGE=alpine:3.23.4@sha256:<new-digest>
```
Renovate will detect and propose this update automatically once Alpine tags a new release.

**Option B — Add explicit runtime upgrade in the final stage**:
```dockerfile
# In Dockerfile final stage, extend the existing apk upgrade line:
RUN apk add --no-cache \
    bash ca-certificates sqlite-libs sqlite tzdata gettext libcap libcap-utils \
    c-ares busybox-extras \
    && apk upgrade --no-cache zlib libcrypto3 libssl3
```
This would pull the patched version on each image build without waiting for a new Alpine base image tag. The tradeoff is slightly reduced reproducibility.

---

## go.mod / package.json Assessment

- `backend/go.mod`: No occurrences of `openssl`, `nebula`, or `libssl`. Backend Go module tree is clean.
- `package.json` (root): Three production dependencies (`@typescript/analyze-trace`, `tldts`, `type-check`) — none flagged by any scanner.
- `frontend/package.json`: Not independently surfacing any HIGH/CRITICAL findings in the Trivy filesystem scan.

---

## Verdict

| Category | Status |
|----------|--------|
| CRITICAL vulnerabilities | ✅ None found |
| HIGH vulnerabilities — actionable now | ✅ None (0 fixable items) |
| HIGH vulnerabilities — upstream-blocked | ⚠️ 2 (CVE-2026-2673 in libcrypto3 + libssl3) |
| Historical HIGH (nebula) | ✅ Resolved via CADDY_PATCH_SCENARIO=B |

**No immediate code changes are required.** Resume monitoring Alpine's security tracker for CVE-2026-2673 patch availability. Once Alpine releases the fix, update `ALPINE_IMAGE` in the Dockerfile or add the explicit `apk upgrade` line for `libcrypto3` and `libssl3`.
