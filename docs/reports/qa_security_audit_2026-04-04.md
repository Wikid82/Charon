# QA Security Vulnerability Audit Report

**Date:** 2026-04-04
**Previous Review:** 2026-03-24
**Reviewed by:** QA Security Engineer
**Scope:** Full security scan — filesystem, dependencies, Docker image, npm, Go vulncheck

---

## 1. Executive Summary

| Severity | Docker Image | Filesystem (Grype) | npm | govulncheck | Total Unique |
|----------|-------------|-------------------|-----|-------------|--------------|
| Critical | 0 | 3 | 0 | 0 | 3 |
| High | 3 | 15+ | 0 | 2 | ~12 unique |
| Medium | 2 | 12+ | 2 | 0 | ~8 unique |
| Low | 0 | 3 | 0 | 0 | ~2 unique |

**Key Findings:**
- **Docker Image (production):** 5 unique vulnerabilities remaining (all previously known and suppressed). No new image-level CVEs.
- **Filesystem (development tooling/stale caches):** Bulk of findings are from CrowdSec/Caddy embedded binaries, `.cache/` module cache (gopls tooling), GitHub Actions, and Python virtualenv tooling — **not from Charon application code**.
- **Charon Backend (direct deps):** All direct Go deps are at or above fix thresholds. `golang.org/x/crypto` at v0.49.0, `golang.org/x/net` at v0.52.0, `google.golang.org/grpc` at v1.79.3, `quic-go` at v0.59.0, `otel/sdk` at v1.42.0.
- **npm:** 2 moderate findings in `smol-toml` (dev dependency via `markdownlint-cli2`).
- **govulncheck:** 2 vulnerabilities from `github.com/docker/docker v28.5.2+incompatible` (no fix available for this import path).
- **No new CRITICAL vulnerabilities** affecting Charon production code since last review.

---

## 2. New Vulnerabilities (Not in SECURITY.md)

### 2.1 [HIGH] GO-2026-4887 — Docker AuthZ Plugin Bypass (Oversized Request Body)

| Field | Value |
|-------|-------|
| **ID** | GO-2026-4887 / CVE-2026-34040 / GHSA-x744-4wpc-v9h2 |
| **Package** | `github.com/docker/docker` v28.5.2+incompatible |
| **Fixed In** | moby/moby v29.3.1 (no fix for `docker/docker` import path) |
| **Severity** | High (CVSS 8.8) |
| **Status** | NEW — **already suppressed** in `.trivyignore` and `.grype.yaml` (added 2026-03-30), but **not yet documented in SECURITY.md** |
| **EPSS** | < 0.1% (1st percentile) |
| **Source** | govulncheck (symbol-level match), Grype (Docker image) |
| **Action** | **WATCH** — Add to SECURITY.md Known Vulnerabilities. No fix available for import path. |

**govulncheck confirmed** this is reachable via `services.DockerService.ListContainers` and `handlers.CrowdsecHandler.DiagnosticsConnectivity`. However, the vulnerability is server-side in the Docker daemon's AuthZ plugin handler — Charon only uses the Docker client SDK.

### 2.2 [MEDIUM] GO-2026-4883 — Moby Off-by-One Plugin Privilege Validation

| Field | Value |
|-------|-------|
| **ID** | GO-2026-4883 / CVE-2026-33997 / GHSA-pxq6-2prw-chj9 |
| **Package** | `github.com/docker/docker` v28.5.2+incompatible |
| **Fixed In** | moby/moby v29.3.1 (no fix for `docker/docker` import path) |
| **Severity** | Medium (CVSS 6.8) |
| **Status** | NEW — **already suppressed** in `.trivyignore` and `.grype.yaml` (added 2026-03-30), but **not yet documented in SECURITY.md** |
| **Source** | govulncheck (symbol-level match), Grype (Docker image) |
| **Action** | **WATCH** — Add to SECURITY.md Known Vulnerabilities. |

### 2.3 [MODERATE] GHSA-v3rj-xjv7-4jmq — smol-toml DoS via Commented Lines

| Field | Value |
|-------|-------|
| **ID** | GHSA-v3rj-xjv7-4jmq |
| **Package** | `smol-toml` < 1.6.1 (npm, via `markdownlint-cli2`) |
| **Fixed In** | smol-toml >= 1.6.1 |
| **Severity** | Moderate |
| **Status** | NEW |
| **Source** | npm audit |
| **Action** | **FIX NOW** — Run `npm audit fix --force` (will install markdownlint-cli2@0.21.0, breaking change). Or pin smol-toml override. |

**Note:** This is a **dev-only dependency** (markdownlint-cli2 for linting docs). Not present in production Docker image. Low real-world risk.

### 2.4 [HIGH] GHSA-wvj2-96wp-fq3f / GHSA-89xv-2j6f-qhc8 / GHSA-q382-vc8q-7jhj / GHSA-xw59-hvm2-8pj6 — MCP Go SDK Vulnerabilities

| Field | Value |
|-------|-------|
| **IDs** | GHSA-wvj2-96wp-fq3f, GHSA-89xv-2j6f-qhc8, GHSA-q382-vc8q-7jhj, GHSA-xw59-hvm2-8pj6 |
| **Package** | `github.com/modelcontextprotocol/go-sdk` v0.8.0 |
| **Fixed In** | v1.3.1 / v1.4.0 / v1.4.1 |
| **Severity** | High |
| **Status** | NOT APPLICABLE — **false positive** |
| **Source** | Grype filesystem scan (found in `.cache/go/pkg/mod/` — gopls tooling, not Charon code) |
| **Action** | **IGNORE** — Not a Charon dependency. Present only in Go module cache from `gopls` IDE tooling. |

### 2.5 [HIGH] GHSA-g754-hx8w-x2g6 / GHSA-47m2-4cr7-mhcw — quic-go Vulnerabilities

| Field | Value |
|-------|-------|
| **ID** | GHSA-g754-hx8w-x2g6 (fixed 0.57.0), GHSA-47m2-4cr7-mhcw (fixed 0.54.1) |
| **Package** | `github.com/quic-go/quic-go` v0.54.0, v0.55.0 |
| **Current Version** | **v0.59.0** (backend go.mod) |
| **Status** | NOT APPLICABLE — **false positive** |
| **Source** | Grype filesystem scan (old versions in go.sum/cache, not in actual dependency tree) |
| **Action** | **IGNORE** — Backend uses v0.59.0, which is above all fix thresholds. |

### 2.6 [HIGH] GHSA-9h8m-3fm2-qjrq — OpenTelemetry SDK

| Field | Value |
|-------|-------|
| **ID** | GHSA-9h8m-3fm2-qjrq |
| **Package** | `go.opentelemetry.io/otel/sdk` v1.38.0 |
| **Current Version** | **v1.42.0** (backend go.mod) |
| **Fixed In** | v1.40.0 |
| **Status** | NOT APPLICABLE — **false positive** |
| **Source** | Grype filesystem scan (old version in go.sum/cache) |
| **Action** | **IGNORE** — Backend uses v1.42.0, above the fix threshold. |

### 2.7 [CRITICAL] GHSA-p77j-4mvh-x3m3 — gRPC-Go Authorization Bypass

| Field | Value |
|-------|-------|
| **ID** | GHSA-p77j-4mvh-x3m3 / CVE-2026-33186 |
| **Package** | `google.golang.org/grpc` v1.67.0 |
| **Current Version** | **v1.79.3** (backend go.mod) |
| **Fixed In** | v1.79.3 |
| **Status** | NOT APPLICABLE — **already fixed** in Charon's direct deps |
| **Source** | Grype filesystem scan (old version from CrowdSec/Caddy embedded binaries) |
| **Action** | **IGNORE** for Charon direct deps. Already suppressed in `.trivyignore` for CrowdSec/Caddy binaries. |

### 2.8 Various Go Stdlib CVEs (CrowdSec/Caddy Embedded Binaries)

| CVE | Severity | Fixed In | Source |
|-----|----------|----------|--------|
| CVE-2025-61726 | High | go1.25.6 | CrowdSec binaries (go1.25.4/5) |
| CVE-2026-25679 | High | go1.25.8/1.26.1 | CrowdSec binaries (go1.25.4/5/6/7) |
| CVE-2025-68121 | Critical | go1.25.7 | CrowdSec binaries (go1.25.4/5/6) — **already patched in SECURITY.md** |
| CVE-2025-61729 | High | go1.25.5 | CrowdSec binaries (go1.25.4) |
| CVE-2025-68119 | High | go1.25.6 | CrowdSec binaries (go1.25.4/5) |
| CVE-2025-61731 | High | go1.25.6 | CrowdSec binaries (go1.25.4/5) |
| CVE-2025-61732 | High | go1.25.7 | CrowdSec binaries (go1.25.4/5/6) |
| CVE-2026-27142 | Medium | go1.25.8/1.26.1 | CrowdSec binaries (go1.25.4/5/6/7) |
| CVE-2025-61728 | Medium | go1.25.6 | CrowdSec binaries (go1.25.4/5) |
| CVE-2025-61730 | Medium | go1.25.6 | CrowdSec binaries (go1.25.4/5) |
| CVE-2025-61727 | Medium | go1.25.5 | CrowdSec binaries (go1.25.4) |
| CVE-2026-27139 | Low | go1.25.8/1.26.1 | CrowdSec binaries (go1.25.4/5/6/7) |

**Status:** These are all from CrowdSec/Caddy embedded binaries compiled with older Go versions — **not from Charon's own code** (compiled with Go 1.26.1). These are stale `go.sum` entries or binary artifacts scanned by Grype.

**Action:** **WATCH** — Awaiting CrowdSec upstream rebuild with newer Go. Charon's own binaries are compiled with Go 1.26.1 and are unaffected.

### 2.9 GitHub Actions Vulnerabilities

| ID | Package | Severity | Fixed In | Action |
|----|---------|----------|----------|--------|
| GHSA-69fq-xp46-6x23 | `aquasecurity/trivy-action` 0.33.1 | Critical | 0.35.0 | **FIX NOW** |
| GHSA-9p44-j4g5-cfx5 | `aquasecurity/trivy-action` 0.33.1 | Medium | 0.34.0 | **FIX NOW** |
| GHSA-qmg3-hpqr-gqvc | `reviewdog/action-setup` v1 | High | — | **WATCH** |
| GHSA-cxww-7g56-2vh6 | `actions/download-artifact` v4 | High | 4.1.3 | **FIX NOW** |

**Action:** Update GitHub Actions workflow files to use latest versions.

### 2.10 Python Tooling Vulnerabilities (Development Only)

| ID | Package | Severity | Fixed In | Action |
|----|---------|----------|----------|--------|
| GHSA-58pv-8j8x-9vj2 | `jaraco-context` 5.3.0 | High | 6.1.0 | WATCH (dev tooling) |
| GHSA-4xh5-x5gv-qwph | `pip` 24.0 | Medium | 25.3 | WATCH (dev tooling) |
| GHSA-6vgw-5pg2-w6jp | `pip` 24.0/25.3 | Low | 26.0 | WATCH (dev tooling) |
| GHSA-8rrh-rw8j-w5fx | `wheel` 0.45.1 | High | 0.46.2 | WATCH (dev tooling) |
| GHSA-qmgc-5h2g-mvrw | `filelock` 3.20.0 | Medium | 3.20.3 | WATCH (dev tooling) |
| GHSA-w853-jp5j-5j7f | `filelock` 3.20.0 | Medium | 3.20.1 | WATCH (dev tooling) |
| GHSA-597g-3phw-6986 | `virtualenv` 20.35.4 | Medium | 20.36.1 | WATCH (dev tooling) |

**Note:** These are all from Python virtualenv/pip tooling in the development environment cache, **not from Charon production code**.

---

## 3. Resolved Vulnerabilities

### 3.1 CVE-2025-68121 — Go Stdlib Critical in CrowdSec Binaries

**Status:** RESOLVED (patched 2026-03-24, already in SECURITY.md Patched section)

Grype still detects older CrowdSec binary versions (go1.25.4/5/6) in the filesystem scan cache, but the **Docker image** no longer shows this CVE. The production image has CrowdSec rebuilt with Go 1.26.1.

### 3.2 CVE-2026-26958 — edwards25519 MultiScalarMult

**Status:** RESOLVED — `filippo.io/edwards25519` is **no longer present** in Charon's backend dependency tree (`go.mod`/`go.sum`). The original finding was from CrowdSec binaries.

**Recommendation:** Move CVE-2026-26958 from Known to Patched in SECURITY.md.

### 3.3 GHSA-p77j-4mvh-x3m3 / CVE-2026-33186 — gRPC-Go Authorization Bypass

**Status:** RESOLVED for Charon direct deps — `google.golang.org/grpc` in backend is now at v1.79.3 (the fix version). The `.trivyignore` entry for this CVE (expiry 2026-04-02) was tracking CrowdSec/Caddy embedded binaries. **The suppression expiry has passed** — needs review.

---

## 4. Existing Vulnerabilities Status Update

### 4.1 CVE-2026-2673 — OpenSSL TLS 1.3 Key Exchange Group Downgrade

| Field | Current Status |
|-------|---------------|
| **Severity** | HIGH (7.5) |
| **Package** | `libcrypto3` 3.5.5-r0, `libssl3` 3.5.5-r0 |
| **Alpine Version** | 3.23.3 (latest) |
| **Fix Available** | No — Alpine 3.23.3 still ships 3.5.5-r0 |
| **Suppression Expiry** | 2026-04-18 |
| **SECURITY.md Status** | Awaiting Upstream |
| **Change since last review** | None. Still awaiting Alpine upstream fix. |
| **Action** | **WATCH** — Extend suppression expiry to 2026-05-04 at next review. |

### 4.2 CVE-2025-60876 — BusyBox wget HTTP Request Smuggling

| Field | Current Status |
|-------|---------------|
| **Severity** | Medium (6.5) |
| **Package** | `busybox` 1.37.0-r30 |
| **Fix Available** | No — Alpine 3.23.3 still ships 1.37.0-r30 |
| **SECURITY.md Status** | Awaiting Upstream |
| **Change since last review** | None. Still present in Docker image scan. |
| **Action** | **WATCH** — No urgency. Charon does not use busybox wget. |

### 4.3 CVE-2026-26958 — edwards25519 MultiScalarMult

| Field | Current Status |
|-------|---------------|
| **Severity** | Low (1.7) |
| **Package** | `filippo.io/edwards25519` v1.1.0 |
| **Fix Available** | v1.1.1 |
| **SECURITY.md Status** | Awaiting Upstream |
| **Change since last review** | **RESOLVED** — No longer in Charon's dependency tree. Not detected in Docker image scan. |
| **Action** | **Move to Patched section in SECURITY.md.** |

---

## 5. Ignore/Watch File Recommendations

### 5.1 Expired Suppressions (Require Immediate Action)

| ID | File | Expiry | Action |
|----|------|--------|--------|
| CVE-2026-33186 | `.trivyignore` | 2026-04-02 | **REVIEW** — Fixed in Charon direct deps (grpc v1.79.3). Check if CrowdSec binaries still need suppression. |
| GHSA-479m-364c-43vc | `.trivyignore` | 2026-04-02 | **REVIEW** — Check if Caddy has updated goxmldsig. |

### 5.2 Suppressions Expiring Soon (Review Required)

| ID | File | Expiry | Action |
|----|------|--------|--------|
| CVE-2026-2673 | `.trivyignore`, `.grype.yaml` | 2026-04-18 | Extend to 2026-05-18 (no upstream fix) |
| GHSA-6g7g-w4f8-9c9x | `.trivyignore`, `.grype.yaml` | 2026-04-19 | Extend to 2026-05-19 (no upstream fix) |
| GHSA-jqcq-xjh3-6g23 | `.trivyignore`, `.grype.yaml` | 2026-04-19 | Extend to 2026-05-19 (no upstream fix) |
| CVE-2026-27171 | `.trivyignore` | 2026-04-21 | Extend to 2026-05-21 (no upstream fix) |
| GHSA-x6gf-mpr2-68h6 | `.trivyignore`, `.grype.yaml` | 2026-04-21 | Extend to 2026-05-21 (no upstream fix) |

### 5.3 New Suppressions to Add

| ID | Recommendation | Justification |
|----|----------------|---------------|
| CVE-2026-34040 / GHSA-x744-4wpc-v9h2 | Already in `.trivyignore`/`.grype.yaml` | Docker client-only usage; server-side vuln |
| CVE-2026-33997 / GHSA-pxq6-2prw-chj9 | Already in `.trivyignore`/`.grype.yaml` | Docker client-only usage; server-side vuln |
| MCP Go SDK findings | No suppression needed | False positive (dev tooling in `.cache/`) |
| GitHub Actions findings | No suppression needed | Fix by updating workflow files |

### 5.4 codecov.yml

No changes recommended. Current configuration is appropriate.

---

## 6. Dependency Update Recommendations

### 6.1 Immediate (FIX NOW)

| Package | Current | Target | CVE/GHSA | Impact |
|---------|---------|--------|----------|--------|
| `aquasecurity/trivy-action` | 0.33.1 | 0.35.0+ | GHSA-69fq-xp46-6x23 (Critical) | GitHub Actions workflow |
| `actions/download-artifact` | v4 | v4.1.3+ | GHSA-cxww-7g56-2vh6 (High) | GitHub Actions workflow |
| `smol-toml` (via markdownlint-cli2) | < 1.6.1 | >= 1.6.1 | GHSA-v3rj-xjv7-4jmq (Moderate) | Dev dependency only |

### 6.2 Recommended (When Feasible)

| Package | Current | Target | Reason |
|---------|---------|--------|--------|
| `reviewdog/action-setup` | v1 | Latest pinned SHA | GHSA-qmg3-hpqr-gqvc (High) |
| `github.com/docker/docker` | v28.5.2+incompatible | moby/moby/v2 (when stable) | GO-2026-4887, GO-2026-4883 |

### 6.3 Awaiting Upstream

| Package | Blocked By | Tracking |
|---------|-----------|----------|
| `libcrypto3`/`libssl3` 3.5.5-r0 | Alpine 3.23 patch | CVE-2026-2673 |
| `busybox` 1.37.0-r30 | Alpine 3.23 patch | CVE-2025-60876 |
| `buger/jsonparser` v1.1.1 | Upstream fix + CrowdSec rebuild | GHSA-6g7g-w4f8-9c9x |
| `jackc/pgproto3/v2` v2.3.3 | CrowdSec migration to pgx/v5 | GHSA-jqcq-xjh3-6g23 |

---

## 7. Alpine Base Image Status

| Field | Value |
|-------|-------|
| **Current** | Alpine 3.23.3 (sha256:25109184c71bdad...) |
| **Latest Available** | Alpine 3.23.3 |
| **Status** | **Up to date** — `alpine:latest` resolves to 3.23.3 |
| **Known Unpatched CVEs in Alpine 3.23.3** | CVE-2026-2673 (OpenSSL), CVE-2025-60876 (busybox), CVE-2026-27171 (zlib) |
| **Recommendation** | No Alpine upgrade available. Monitor for 3.23.4 or 3.24.0. |

---

## 8. Scanner Summary

### Trivy Filesystem Scan
- **Result:** 0 vulnerabilities found in source code and dependencies
- **Note:** Trivy only scanned language-specific files. Go modules resolved correctly with no findings.

### Grype Filesystem Scan
- **Result:** ~75 findings (many duplicates across versions)
- **Unique Vulnerabilities:** ~25
- **False Positives:** ~15 (stale go.sum entries, `.cache/` module cache, development tooling)
- **Actionable for Charon Production:** ~5 (all previously known and suppressed)
- **Actionable for CI/CD:** 3 (GitHub Actions version updates)

### Grype Docker Image Scan
- **Result:** 5 unique vulnerabilities
- **All previously known** and documented in `.trivyignore`/`.grype.yaml`
- **No new production vulnerabilities**

### npm audit
- **Result:** 2 moderate vulnerabilities in dev dependency (`smol-toml` via `markdownlint-cli2`)
- **Action:** Low priority — dev tooling only

### govulncheck
- **Result:** 2 vulnerabilities, both in `github.com/docker/docker` v28.5.2+incompatible
- **Symbol traces confirmed:** Code paths exist but vulnerability is server-side (Docker daemon), not client-side
- **Action:** Already suppressed; awaiting upstream fix

---

## 9. SECURITY.md Update Checklist

- [ ] **Move CVE-2026-26958 (edwards25519) from Known to Patched** — no longer in dependency tree
- [ ] **Add CVE-2026-34040 / GHSA-x744-4wpc-v9h2 (Docker AuthZ bypass) to Known** — already suppressed but not documented in SECURITY.md
- [ ] **Add CVE-2026-33997 / GHSA-pxq6-2prw-chj9 (Docker plugin privilege) to Known** — already suppressed but not documented in SECURITY.md
- [ ] **Review expired suppression CVE-2026-33186** — expiry was 2026-04-02; grpc v1.79.3 fixes it for Charon direct deps. Check if CrowdSec/Caddy still need it.
- [ ] **Review expired suppression GHSA-479m-364c-43vc** — expiry was 2026-04-02
- [ ] **Update "Last reviewed" date** to 2026-04-04
- [ ] **Extend suppression expiry dates** for CVEs still awaiting upstream (see Section 5.2)

---

## 10. Recommended Priority Actions

### P0 — Immediate
1. Update GitHub Actions: `aquasecurity/trivy-action` to 0.35.0+, `actions/download-artifact` to v4.1.3+
2. Review and extend/remove expired suppressions (CVE-2026-33186, GHSA-479m-364c-43vc)

### P1 — This Sprint
3. Update SECURITY.md: move CVE-2026-26958 to Patched, add Docker CVEs to Known
4. Fix `smol-toml` npm dev dependency vulnerability
5. Extend suppression expiry dates for upcoming expirations (Section 5.2)

### P2 — Monitor
6. Track Alpine 3.23.4/3.24.0 for OpenSSL, busybox, zlib patches
7. Track CrowdSec releases for dependency updates (jsonparser, pgproto3/v2, grpc)
8. Track `moby/moby/v2` stabilization for Docker SDK migration
