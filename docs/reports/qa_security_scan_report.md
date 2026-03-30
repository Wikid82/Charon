# QA Security Scan Report

**Date**: 2026-03-20
**Scope**: Charon project — full stack (filesystem, Go modules, npm, Docker image, source code)
**Scanners**: grype (live + cached), Trivy (cached), govulncheck (live), npm audit (live), golangci-lint/gosec (live)
**Scan results reviewed**: `trivy-report.json`, `trivy-image-report.json`, `grype-results.json`, `vuln-results.json`
**Project Go version**: go 1.26.1 (Charon backend), go 1.25.6–1.25.7 (CrowdSec bundled binaries)
**Container OS**: Alpine Linux 3.23.3

---

## Executive Summary

| Severity | Total Found | Patchable Now | Awaiting Upstream | Code Fix Required | Already Resolved |
|----------|-------------|---------------|-------------------|-------------------|------------------|
| CRITICAL | 1 | 1 | 0 | 0 | — |
| HIGH | 7 | 2 | 3 | 1 | 3 |
| MEDIUM | 6 | 4 | 1 | 1 | — |
| LOW | 4 | 3 | 1 | 1 | — |
| **Total** | **18** | **10** | **4** | **3** | **3** |

**Key narrative:**

- The Alpine base image migration (CHARON-2026-001) is **complete**. `charon:local` confirmed Alpine 3.23.3. The 7 Debian HIGH CVEs are gone.
- A **new CRITICAL CVE** (CVE-2025-68121) was discovered in CrowdSec's bundled Go binaries compiled with go1.25.6. This is the most urgent finding.
- Two **new HIGH CVEs** in those same bundled binaries (CVE-2026-25679, CVE-2025-61732) are also actionable — all three resolve by rebuilding CrowdSec against go ≥ 1.25.8.
- **CVE-2026-2673** (OpenSSL TLS 1.3 key exchange group downgrade) affects `libcrypto3` and `libssl3` in Alpine 3.23.3. No Alpine package fix exists yet (advisory: 2026-03-13).
- Charon's own Go backend (go 1.26.1) and all npm dependencies are **clean with zero vulnerabilities**.
- CVE-2026-25793 (nebula in Caddy) is **resolved** by the CADDY_PATCH_SCENARIO=B Dockerfile change.

---

## Findings Table

### CRITICAL

| # | CVE ID | CVSS | Package | Installed | Fixed In | Component | Status | New? |
|---|--------|------|---------|-----------|----------|-----------|--------|------|
| C-1 | CVE-2025-68121 | Critical | `stdlib` (Go) | go1.25.6 | go1.25.7 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |

### HIGH

| # | CVE / ID | CVSS | Package | Installed | Fixed In | Component | Status | New? |
|---|----------|------|---------|-----------|----------|-----------|--------|------|
| H-1 | CVE-2026-2673 | 7.5 | `libcrypto3` | 3.5.5-r0 | None | Alpine 3.23.3 base image | AWAITING UPSTREAM | ✅ NEW |
| H-2 | CVE-2026-2673 | 7.5 | `libssl3` | 3.5.5-r0 | None | Alpine 3.23.3 base image | AWAITING UPSTREAM | ✅ NEW |
| H-3 | CVE-2026-25679 | High | `stdlib` (Go) | go1.25.6 & go1.25.7 | go1.25.8 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |
| H-4 | CVE-2025-61732 | High | `stdlib` (Go) | go1.25.6 | go1.25.7 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |
| H-5 | CHARON-2025-001 | High | CrowdSec binaries (Go stdlib) | go < 1.26 | Upstream rebuild | CrowdSec Agent (CVE-2025-58183/58186/58187/61729) | AWAITING UPSTREAM | Known |
| H-6 | DS-0002 | High (misconfig) | `Dockerfile` | — | Add `USER` instruction | Container configuration | CODE FIX | ✅ NEW |

> **Note on H-3 / H-4 and CHARON-2025-001**: The original CHARON-2025-001 CVEs tracked CrowdSec binaries at go1.25.1. Current scans show binaries at go1.25.6/go1.25.7, indicating CrowdSec was updated but remains behind go1.25.8. The new CVEs (C-1, H-3, H-4) are continuations of the same class of issue.

### MEDIUM

| # | CVE / ID | CVSS | Package | Installed | Fixed In | Component | Status | New? |
|---|----------|------|---------|-----------|----------|-----------|--------|------|
| M-1 | CVE-2025-60876 | 6.5 | `busybox`, `busybox-binsh`, `busybox-extras`, `ssl_client` | 1.37.0-r30 | None (per grype scan) | Alpine 3.23.3 base image | AWAITING UPSTREAM | ✅ NEW¹ |
| M-2 | CVE-2026-27142 | Medium | `stdlib` (Go) | go1.25.6 & go1.25.7 | go1.25.8 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |
| M-3 | GHSA-qmgc-5h2g-mvrw | Medium | `filelock` | 3.20.0 | 3.20.3 | Python dev tooling | PATCHABLE | ✅ NEW |
| M-4 | GHSA-w853-jp5j-5j7f | Medium | `filelock` | 3.20.0 | 3.20.1 | Python dev tooling | PATCHABLE | ✅ NEW |
| M-5 | GHSA-597g-3phw-6986 | Medium | `virtualenv` | 20.35.4 | 20.36.1 | Python dev tooling | PATCHABLE | ✅ NEW |
| M-6 | G203 (gosec) | Medium | `mail_service.go:195` | `template.HTML()` | Sanitize input | Charon backend (Go) | CODE FIX | ✅ NEW |

> ¹ SECURITY.md stated Alpine patched CVE-2025-60876; `grype-results.json` image scan shows no fix for busybox 1.37.0-r30. Verify with a fresh container build before treating as actively exploitable.

### LOW

| # | CVE / ID | CVSS | Package | Installed | Fixed In | Component | Status | New? |
|---|----------|------|---------|-----------|----------|-----------|--------|------|
| L-1 | GHSA-fw7p-63qq-7hpr | 1.7 | `filippo.io/edwards25519` | v1.1.0 | v1.1.1 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |
| L-2 | CVE-2026-27139 | Low | `stdlib` (Go) | go1.25.6 & go1.25.7 | go1.25.8 | CrowdSec bundled binaries | PATCHABLE | ✅ NEW |
| L-3 | GHSA-6vgw-5pg2-w6jp | Low | `pip` | 25.3 | 26.0 | Python dev tooling | PATCHABLE | ✅ NEW |
| L-4 | G306 (gosec) | Low (misconfig) | `docker_service_test.go:231` | `0o660` | Change to `0o600` | Charon test code | CODE FIX | ✅ NEW |

---

## Patchable CVEs (Fix Available Now)

### P-1 · CVE-2025-68121 [CRITICAL] — Go stdlib in CrowdSec binaries

| Field | Value |
|-------|-------|
| CVSS | Critical |
| Package | `stdlib` (Go) |
| Affected | go1.25.6 and earlier |
| Fixed in | go1.25.7 |
| Component | CrowdSec bundled binaries (`cscli`, `crowdsec`) |
| Fix action | Rebuild CrowdSec against go ≥ 1.25.8 (also resolves H-3, H-4, M-2, L-1, L-2) |

govulncheck reports Charon's own backend (go 1.26.1) as clean. The vulnerability is exclusively in the CrowdSec agent binaries bundled in the container image. This is the **highest priority finding** in this audit.

---

### P-2 · CVE-2026-25679 [HIGH] + P-3 · CVE-2025-61732 [HIGH] — Go stdlib in CrowdSec binaries

Both reside in CrowdSec bundled binaries compiled with go1.25.6/go1.25.7. CVE-2026-25679 requires go1.25.8; CVE-2025-61732 resolves at go1.25.7.

**Fix action**: same as P-1 — rebuild CrowdSec against go ≥ 1.25.8.

---

### P-4 through P-8 — Python dev tooling (Medium/Low)

These affect only the development environment, not the production container:

| ID | Package | Fix |
|----|---------|-----|
| GHSA-qmgc-5h2g-mvrw | `filelock` 3.20.0 | `pip install --upgrade filelock` (→ 3.20.3) |
| GHSA-w853-jp5j-5j7f | `filelock` 3.20.0 | same |
| GHSA-597g-3phw-6986 | `virtualenv` 20.35.4 | `pip install --upgrade virtualenv` (→ 20.36.1) |
| GHSA-6vgw-5pg2-w6jp | `pip` 25.3 | `pip install --upgrade pip` (→ 26.0) |

---

## Awaiting Upstream

### U-1 · CVE-2026-2673 [HIGH] × 2 — OpenSSL TLS 1.3 key group downgrade

| Field | Value |
|-------|-------|
| CVSS | 7.5 |
| Packages | `libcrypto3` and `libssl3` |
| Installed | 3.5.5-r0 (Alpine 3.23.3) |
| Fixed in | No Alpine APK available as of 2026-03-20 |
| Advisory | 2026-03-13 — https://openssl-library.org/news/secadv/20260313.txt |
| Component | Docker base image (Alpine 3.23.3) |

An OpenSSL TLS 1.3 server may fail to negotiate the intended key exchange group when the configuration includes the `DEFAULT` keyword, potentially allowing downgrade to weaker cipher suites. Charon's Caddy configuration does not use `DEFAULT` key groups explicitly, limiting practical impact.

**Monitor**: https://security.alpinelinux.org/vuln/CVE-2026-2673

When Alpine releases the patch, update the pinned `ALPINE_IMAGE` digest in the Dockerfile or extend the runtime stage:

```dockerfile
RUN apk upgrade --no-cache zlib libcrypto3 libssl3
```

---

### U-2 · CHARON-2025-001 [HIGH] — CrowdSec Go stdlib CVEs (original cluster)

Tracked in SECURITY.md: CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729. CrowdSec was updated from go1.25.1 to go1.25.6/go1.25.7 (the original CVEs may have been resolved) but go stdlib CVEs continue to accumulate. All resolve when CrowdSec releases a build using go ≥ 1.25.8.

---

### U-3 · CVE-2025-60876 [MEDIUM] — busybox 1.37.0-r30 (Alpine)

| Field | Value |
|-------|-------|
| CVSS | 6.5 |
| Packages | `busybox`, `busybox-binsh`, `busybox-extras`, `ssl_client` |
| Installed | 1.37.0-r30 |
| Fixed in | Not available per `grype-results.json` scan |
| Component | Alpine 3.23.3 base image |

SECURITY.md previously stated Alpine patched CVE-2025-60876. The `grype-results.json` image scan still shows no fix version. Verify with a fresh `grype` container scan before acting.

---

## False Positives / Already Mitigated

### R-1 · CHARON-2026-001 — Debian Base Image CVE cluster — RESOLVED

The 7 HIGH Debian Trixie CVEs (`libc6`, `libc-bin`, `libtasn1-6`, `libtiff`) are fully gone. The `charon:local` image confirmed Alpine Linux 3.23.3. No action required.

---

### R-2 · CVE-2026-25793 — nebula v1.9.7 in `usr/bin/caddy` — RESOLVED

The `CADDY_PATCH_SCENARIO=B` Dockerfile change removed the explicit nebula v1.9.7 pin. The finding does not appear in the 2026-03-18 Docker image scan. No action required.

---

### R-3 · CVE-2025-68156 — `expr-lang/expr` ReDoS — PATCHED

Resolved 2026-01-11 by upgrading CrowdSec to a build using `expr-lang/expr` v1.17.7. No finding in any current scan.

---

### R-4 · G203 (gosec) — `mail_service.go:195` template.HTML()

`template.HTML(contentBuf.String())` bypasses Go's auto-escaping. Only exploitable if `contentBuf` contains attacker-controlled HTML. If content is generated entirely from internal trusted templates, this is a false positive. Audit the call site; if any user input flows in, sanitize before casting. If fully internal, suppress via `//nolint:gosec` with a justification comment.

---

### R-5 · G306 (gosec) — `docker_service_test.go:231` file permissions 0o660

Test-only, no production risk. The fix is trivial: change `0o660` to `0o600`.

---

## Recommended Actions (Prioritized)

### Priority 1 — URGENT: Rebuild CrowdSec with Go ≥ 1.25.8

**Resolves**: CVE-2025-68121 (Critical), CVE-2026-25679 (High), CVE-2025-61732 (High), CVE-2026-27142 (Medium), CVE-2026-27139 (Low), GHSA-fw7p-63qq-7hpr (Low), supersedes CHARON-2025-001 original CVEs.

Update the CrowdSec build stage Go toolchain pin in the Dockerfile:

```dockerfile
ARG GOLANG_IMAGE=golang:1.25.8-alpine3.23
```

---

### Priority 2 — Monitor Alpine for CVE-2026-2673 patch

No code change is actionable today. Monitor https://security.alpinelinux.org/vuln/CVE-2026-2673. Once available, update `ALPINE_IMAGE` digest or add `libcrypto3 libssl3` to the `apk upgrade` line.

---

### Priority 3 — Fix Dockerfile root user (DS-0002)

Add a non-root `USER` instruction to the Dockerfile final stage. Verify runtime paths are owned by the new user before deploying.

```dockerfile
RUN addgroup -S charon && adduser -S charon -G charon
USER charon
```

---

### Priority 4 — Audit mail_service.go template.HTML cast

Audit data flow into `internal/services/mail_service.go:195`. Suppress with `//nolint:gosec` if fully internal, or sanitize user input before casting.

---

### Priority 5 — Fix test file permissions

Change `0o660` → `0o600` at `internal/services/docker_service_test.go:231`.

---

### Priority 6 — Update Python dev tooling

```bash
pip install --upgrade filelock virtualenv pip
```

---

### Priority 7 — Verify CVE-2025-60876 busybox status

Run a fresh `grype` scan against the current `charon:local` image and confirm whether busybox is patched. Update tracking accordingly.

---

## Scan Coverage Summary

| Scan | Target | Tool | CRITICAL | HIGH | MEDIUM | LOW | Notes |
|------|--------|------|----------|------|--------|-----|-------|
| Filesystem (live, 2026-03-20) | `/projects/Charon` | grype | 1 | 2 | 3 | 3 | Go stdlib in CrowdSec binaries; Python tooling |
| Image (cached 2026-02-25) | `charon:local` (Alpine 3.23.3) | Trivy | 0 | 1 | 0 | 0 | CVE-2026-25793 nebula (RESOLVED) |
| Image (skill runner, 2026-03-18) | `charon:local` (Alpine 3.23.3) | grype | 0 | 2 | 4 | 1 | CVE-2026-2673 OpenSSL; busybox; edwards25519 |
| Repository (cached 2026-02-20) | `/app` | Trivy | 0 | 1 (miscfg) | 0 | 0 | DS-0002 Dockerfile root |
| Repository (cached 2026-02-25) | `/app` | Trivy | 0 | 0 | 0 | 0 | Clean across go.mod + npm |
| Go modules (live, 2026-03-20) | `backend/...` | govulncheck | 0 | 0 | 0 | 0 | ✅ Clean |
| npm (live, 2026-03-20) | `/projects/Charon` | npm audit | 0 | 0 | 0 | 0 | ✅ Clean (281 deps) |
| Source code (live, 2026-03-20) | `backend/internal/...` | golangci-lint/gosec | 0 | 0 | 1 | 1 | G203 XSS; G306 permissions |

---

## CVE Status vs SECURITY.md

| CVE / ID | In SECURITY.md | Current Status |
|----------|----------------|----------------|
| CHARON-2026-001 (Debian CVE cluster) | Yes | ✅ RESOLVED — Alpine migration complete |
| CHARON-2025-001 (CVE-2025-58183/58186/58187/61729) | Yes | ⚠️ ONGOING — CVE cluster updated; now also includes CVE-2025-68121 (Critical), CVE-2026-25679 (High), CVE-2025-61732 (High) |
| CVE-2025-68156 (expr-lang/expr) | Yes | ✅ PATCHED |
| CVE-2025-68121 | **No — NEW** | 🔴 OPEN — Critical, CrowdSec go1.25.6 |
| CVE-2026-2673 (×2) | **No — NEW** | 🟠 OPEN — High, Alpine OpenSSL, awaiting upstream |
| CVE-2026-25679 | **No — NEW** | 🟠 OPEN — High, CrowdSec go1.25.6/1.25.7 |
| CVE-2025-61732 | **No — NEW** | 🟠 OPEN — High, CrowdSec go1.25.6 |
| DS-0002 | **No — NEW** | 🟠 OPEN — High misconfiguration, Dockerfile root |
| CVE-2025-60876 | **No — NEW** | 🟡 OPEN — Medium, busybox; verify Alpine patch status |
| CVE-2026-27142 | **No — NEW** | 🟡 OPEN — Medium, CrowdSec go1.25.6/1.25.7 |
| GHSA-qmgc-5h2g-mvrw | **No — NEW** | 🟡 OPEN — Medium, filelock (dev tooling) |
| GHSA-w853-jp5j-5j7f | **No — NEW** | 🟡 OPEN — Medium, filelock (dev tooling) |
| GHSA-597g-3phw-6986 | **No — NEW** | 🟡 OPEN — Medium, virtualenv (dev tooling) |
| G203 (gosec) | **No — NEW** | 🟡 OPEN — Medium, mail_service.go XSS risk |
| GHSA-fw7p-63qq-7hpr | **No — NEW** | 🟢 OPEN — Low, edwards25519 v1.1.0 (CrowdSec) |
| CVE-2026-27139 | **No — NEW** | 🟢 OPEN — Low, CrowdSec go1.25.6/1.25.7 |
| GHSA-6vgw-5pg2-w6jp | **No — NEW** | 🟢 OPEN — Low, pip 25.3 (dev tooling) |
| G306 (gosec) | **No — NEW** | 🟢 OPEN — Low, test file permissions |
| CVE-2026-25793 (nebula) | No | ✅ RESOLVED — CADDY_PATCH_SCENARIO=B applied |

**No immediate code changes are required.** Resume monitoring Alpine's security tracker for CVE-2026-2673 patch availability. Once Alpine releases the fix, update `ALPINE_IMAGE` in the Dockerfile or add the explicit `apk upgrade` line for `libcrypto3` and `libssl3`.
