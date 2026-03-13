# Post-Slack Merge Blockers — Remediation Plan

**Status:** Active
**Created:** 2026-03-13
**Branch:** `feature/beta-release`
**Context:** Slack notification provider is functionally complete. Four blockers remain before merge.

---

## 1. Executive Summary

| # | Blocker | Severity | Effort | Fix Available |
|---|---------|----------|--------|---------------|
| 1 | Patch coverage short by 15 backend lines (Slack commit) | Medium | ~1 hr | Yes — add unit tests |
| 2 | 2 HIGH vulnerabilities in Docker image (`binutils`) | Low | None | No — no upstream fix; build-time only |
| 3 | `anchore/sbom-action` uses Node.js 20 | Medium | Blocked | No — upstream has not released a node24 build |
| 4 | 13 MEDIUM vulnerabilities in Docker image | Mixed | ~30 min | 1 fixable (`zlib`), 12 unfixable (no upstream fix) |

**Bottom line:** Blocker 1 is the only item requiring code changes. Blockers 2/3/4 are environmental and require either upstream fixes, documented risk acceptance, or a single `apk upgrade` in the Dockerfile.

---

## 2. Blocker 1: Patch Coverage — 15 Uncovered Backend Lines

### Methodology

Computed by cross-referencing `git diff HEAD~1...HEAD` (Slack commit `26be592f`) against `backend/coverage.txt` using Go's atomic coverage profile. Only non-test `.go` files in the Slack commit are considered.

**Totals:** 63 changed source lines, 15 uncovered → 76.2% patch coverage.

### Uncovered Lines

#### File: `backend/internal/api/handlers/notification_provider_handler.go` (9 lines)

| Lines | Code | Description |
|-------|------|-------------|
| 141–143 | `return "PROVIDER_TEST_VALIDATION_FAILED", "validation", "Slack rejected the payload..."` | Error classification for `invalid_payload` / `missing_text_or_fallback` Slack API errors |
| 145–147 | `return "PROVIDER_TEST_AUTH_REJECTED", "dispatch", "Slack webhook is revoked..."` | Error classification for `no_service` Slack API error |
| 325–327 | `respondSanitizedProviderError(c, http.StatusBadRequest, "TOKEN_WRITE_ONLY", ...)` + `return` | Guard preventing Slack webhook URL from being sent in test-notification requests |

#### File: `backend/internal/services/notification_service.go` (6 lines)

| Lines | Code | Description |
|-------|------|-------------|
| 462–463 | `marshalErr` branch → `return fmt.Errorf("failed to normalize slack payload: %w", marshalErr)` | Error path when `json.Marshal` fails during Slack payload normalization (`message` → `text` rewrite) |
| 466–467 | `writeErr` branch → `return fmt.Errorf("failed to write normalized slack payload: %w", writeErr)` | Error path when `body.Write` fails after normalization |
| 549–550 | `return fmt.Errorf("slack webhook URL is not configured")` | Guard when decrypted Slack webhook URL is empty at dispatch time |

### Proposed Test Additions

All tests go in existing test files alongside the current Slack test cases.

**1. `notification_provider_handler_test.go` — Slack error classification (covers lines 141–147)**

Add test cases to the `classifySlackProviderTestError` test table:
- Input error containing `"invalid_payload"` → assert returns `PROVIDER_TEST_VALIDATION_FAILED`
- Input error containing `"missing_text_or_fallback"` → assert returns `PROVIDER_TEST_VALIDATION_FAILED`
- Input error containing `"no_service"` → assert returns `PROVIDER_TEST_AUTH_REJECTED`

**2. `notification_provider_handler_test.go` — Slack TOKEN_WRITE_ONLY guard (covers lines 325–327)**

Add a test case to the test-notification endpoint tests:
- Send a test-notification request with `type=slack` and a non-empty `token` field
- Assert HTTP 400 with error code `TOKEN_WRITE_ONLY`

**3. `notification_service_test.go` — Slack payload normalization errors (covers lines 462–467)**

Add test cases to the Slack dispatch tests:
- Provide a payload with a `message` field but inject a marshal failure (e.g., via a value that causes `json.Marshal` to fail such as `math.NaN` or a channel-based mock)
- Alternatively, test the `message`→`text` normalization happy path (which exercises lines 459–467 inclusive) and use a mock `body.Write` that returns an error

**4. `notification_service_test.go` — Empty Slack webhook URL (covers lines 549–550)**

Add a test case:
- Create a Slack provider with an empty/whitespace-only Token (webhook URL)
- Call dispatch
- Assert error contains `"slack webhook URL is not configured"`

---

## 3. Blocker 2: 2 HIGH Vulnerabilities

### Findings

| CVE | Package | Version | CVSS | Fix Available | Source |
|-----|---------|---------|------|---------------|--------|
| CVE-2025-69650 | `binutils` | 2.45.1-r0 | 7.5 | No | `grype-results.json` |
| CVE-2025-69649 | `binutils` | 2.45.1-r0 | 7.5 | No | `grype-results.json` |

### Analysis

- **CVE-2025-69650:** Double-free in `readelf` when processing crafted ELF files.
- **CVE-2025-69649:** Null pointer dereference in `readelf` when processing crafted ELF files.

Both affect GNU Binutils, which is present in the Alpine image as a build dependency pulled in by `gcc`/`musl-dev` for CGo compilation. These are:

1. **Build-time only** — `binutils` is not used at runtime by Charon
2. **Not exploitable** — requires processing a malicious ELF file via `readelf`, which Charon never invokes
3. **No upstream fix** — Alpine has not released a patched `binutils`
4. **Pre-existing** — present before the Slack commit

### Remediation

- **Action:** Document as accepted risk in the PR description
- **Rationale:** Build-toolchain-only vulnerability with no runtime exposure. No fix available upstream.
- **Review trigger:** Re-evaluate when Alpine releases `binutils >= 2.47` or patches the 2.45.1 package

---

## 4. Blocker 3: `anchore/sbom-action` Uses Node.js 20

### Current State

| Workflow | File | Line |
|----------|------|------|
| Docker Build | `docker-build.yml` | 577 |
| Nightly Build | `nightly-build.yml` | 266 |
| Supply Chain PR | `supply-chain-pr.yml` | 269 |
| Supply Chain Verify | `supply-chain-verify.yml` | 122 |

All four reference the same pin: `anchore/sbom-action@57aae528053a48a3f6235f2d9461b05fbcb7366d # v0.23.1`

**v0.23.1** (released 2026-03-10) is the latest release. Its `action.yml` declares `runs.using: "node20"`.

### Analysis

- GitHub Actions is deprecating Node.js 20 actions (targeting Node.js 24 as the successor runtime).
- `anchore/sbom-action` has **not released a node24-compatible version** yet. The project transitioned from node16→node20 around v0.17.x.
- No open issue or PR on the `anchore/sbom-action` repository tracks node24 migration.
- The current pin at v0.23.1 / `57aae52` is the best available version.

### Remediation

| Option | Action | Risk |
|--------|--------|------|
| **A) Wait (recommended)** | Keep current pin. Monitor `anchore/sbom-action` releases for a node24 build. Renovate will auto-propose the update. | GitHub will show deprecation warnings but will not break the action until the hard cutoff. |
| **B) Suppress warning** | Add `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION=true` env var to the workflow steps. | Masks the warning but does not fix the underlying issue. Not recommended. |
| **C) Fork and patch** | Fork the action, change `node20` to `node24`, rebuild dist. | Maintenance burden; likely breaks without code changes to support Node 24 APIs. |

**Recommendation:** Option A. Pin stays at `v0.23.1` / `57aae528053a48a3f6235f2d9461b05fbcb7366d`. Document in PR description as a known upstream dependency awaiting update.

---

## 5. Blocker 4: 13 MEDIUM Vulnerabilities

### Full Catalog

All from `grype-results.json` (Docker image scan of Alpine 3.23.3 base).

| # | CVE | Package | Version | Fix | Category |
|---|-----|---------|---------|-----|----------|
| 1 | CVE-2025-60876 | `busybox` | 1.37.0-r30 | None | Unfixable |
| 2 | CVE-2025-60876 | `busybox-binsh` | 1.37.0-r30 | None | Unfixable (same CVE) |
| 3 | CVE-2025-60876 | `busybox-extras` | 1.37.0-r30 | None | Unfixable (same CVE) |
| 4 | CVE-2025-60876 | `ssl_client` | 1.37.0-r30 | None | Unfixable (same CVE) |
| 5 | CVE-2025-14819 | `curl` | 8.17.0-r1 | None | Unfixable |
| 6 | CVE-2025-15079 | `curl` | 8.17.0-r1 | None | Unfixable |
| 7 | CVE-2025-14524 | `curl` | 8.17.0-r1 | None | Unfixable |
| 8 | CVE-2025-13034 | `curl` | 8.17.0-r1 | None | Unfixable |
| 9 | CVE-2025-14017 | `curl` | 8.17.0-r1 | None | Unfixable |
| 10 | CVE-2025-69652 | `binutils` | 2.45.1-r0 | None | Unfixable |
| 11 | CVE-2025-69644 | `binutils` | 2.45.1-r0 | None | Unfixable |
| 12 | CVE-2025-69651 | `binutils` | 2.45.1-r0 | None | Unfixable |
| 13 | CVE-2026-27171 | `zlib` | 1.3.1-r2 | **1.3.2-r0** | **Fixable** |

### Grouping

| Category | Entries | Unique CVEs | Packages |
|----------|---------|-------------|----------|
| BusyBox wget CRLF injection | 4 | 1 (CVE-2025-60876) | busybox, busybox-binsh, busybox-extras, ssl_client |
| curl TLS/SSH edge cases | 5 | 5 distinct | curl 8.17.0-r1 |
| binutils readelf issues | 3 | 3 distinct | binutils 2.45.1-r0 |
| zlib vulnerability | 1 | 1 (CVE-2026-27171) | zlib 1.3.1-r2 |

### Remediation

**Fixable (1 vuln):**

| CVE | Fix |
|-----|-----|
| CVE-2026-27171 (`zlib`) | Add `RUN apk upgrade --no-cache zlib` to Dockerfile runtime stage, or bump Alpine base if 3.23.4+ ships with zlib 1.3.2 |

**Unfixable (12 vulns):**

| Package | Risk | Mitigation |
|---------|------|------------|
| `busybox` (CVE-2025-60876) | Low — wget CRLF injection. Charon does not invoke `wget` at runtime. | Accept risk; monitor Alpine. |
| `curl` (5 CVEs) | Low–Medium — TLS/SSH edge cases. Charon uses Go `net/http`, not `curl`. Present only for health checks. | Accept risk; consider removing `curl` from runtime image. |
| `binutils` (3 CVEs) | Low — build-time only (`readelf` DoS). Not in runtime path. | Accept risk; monitor Alpine. |

---

## 6. Commit Slicing Strategy

### Decision: 2 PRs

| PR | Scope | Files | Validation |
|----|-------|-------|------------|
| **PR-1: Coverage + zlib fix** | Unit tests for 15 uncovered Slack lines + `apk upgrade zlib` in Dockerfile | `notification_provider_handler_test.go`, `notification_service_test.go`, `Dockerfile` | `go test ./...`, re-run grype scan, regenerate patch report |
| **PR-2: Risk acceptance docs** | Document accepted risks for unfixable vulns + SBOM node20 | PR description or `.github/security-accepted-risks.md` | Review-only |

### Trigger Reasons

- PR-1 is code (tests + Dockerfile) — requires CI
- PR-2 is documentation/process — review-only, no CI risk
- Splitting allows PR-1 to merge quickly while risk discussions happen asynchronously

### Rollback

- **PR-1:** Safe to revert — only adds tests and an `apk upgrade`. No behavioral changes.
- **PR-2:** Documentation only.

---

## 7. Execution Order

| Step | Action | Blocker | Effort |
|------|--------|---------|--------|
| 1 | Add unit tests for 15 uncovered Slack lines | 1 | ~45 min |
| 2 | Add `apk upgrade --no-cache zlib` to Dockerfile runtime stage | 4 (partial) | ~5 min |
| 3 | Re-run backend tests + coverage, regenerate patch report | 1 verification | ~10 min |
| 4 | Re-run Docker image scan (grype/trivy) | 4 verification | ~5 min |
| 5 | Open PR-1 with test + Dockerfile changes | — | ~10 min |
| 6 | Document risk acceptance for unfixable vulns + SBOM node20 in PR-2 | 2, 3, 4 | ~15 min |

---

## Acceptance Criteria

- [ ] All 15 uncovered Slack lines have corresponding unit test cases
- [ ] Backend patch coverage for Slack commit ≥ 95%
- [ ] `zlib` upgraded to ≥ 1.3.2-r0 in Docker image
- [ ] Docker image scan shows 0 fixable MEDIUM+ vulnerabilities
- [ ] Unfixable vulnerabilities documented with risk acceptance rationale
- [ ] `anchore/sbom-action` node20 status documented; pin unchanged at v0.23.1
