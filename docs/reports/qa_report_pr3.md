# QA Security Report — PR-3: RFC 1918 Bypass for Uptime Monitor

**Date:** 2026-03-17
**QA Agent:** Security QA
**Status at Review Start:** Implementation complete, Supervisor-approved
**Final Verdict:** ✅ APPROVED FOR COMMIT

---

## Summary

PR-3 adds RFC 1918 private-address bypass capability to the uptime monitor system, allowing users to monitor hosts on private network ranges (10.x.x.x, 172.16–31.x.x, 192.168.x.x) without disabling SSRF protections globally. The implementation spans three production files and three test files.

---

## Pre-Audit Fix

**Issue:** The Supervisor identified that `TestValidateExternalURL_WithAllowRFC1918_BlocksMetadata` in `url_validator_test.go` should include `WithAllowHTTP()` to exercise the SSRF IP-level check rather than failing at the scheme check.

**Finding:** `WithAllowHTTP()` was already present in the test at audit start. No change required.

---

## Audit Results

### 1. Build Verification

```
cd /projects/Charon/backend && go build ./...
```

**Result: ✅ PASS** — Clean build, zero errors.

---

### 2. Targeted Package Tests (PR-3 Files)

#### `internal/network` — RFC 1918 tests

| Test | Result |
|---|---|
| `TestIsRFC1918_RFC1918Addresses` (11 subtests) | ✅ PASS |
| `TestIsRFC1918_NonRFC1918Addresses` (9 subtests) | ✅ PASS |
| `TestIsRFC1918_NilIP` | ✅ PASS |
| `TestIsRFC1918_BoundaryAddresses` (5 subtests) | ✅ PASS |
| `TestIsRFC1918_IPv4MappedAddresses` (5 subtests) | ✅ PASS |
| `TestSafeDialer_AllowRFC1918_ValidationLoopSkipsRFC1918` | ✅ PASS |
| `TestSafeDialer_AllowRFC1918_BlocksLinkLocal` | ✅ PASS |
| `TestSafeDialer_AllowRFC1918_BlocksLoopbackWithoutAllowLocalhost` | ✅ PASS |
| `TestNewSafeHTTPClient_AllowRFC1918_BlocksSSRFMetadata` | ✅ PASS |
| `TestNewSafeHTTPClient_WithAllowRFC1918_OptionApplied` | ✅ PASS |

**Package result:** `ok github.com/Wikid82/charon/backend/internal/network 0.208s`

#### `internal/security` — RFC 1918 tests

| Test | Result |
|---|---|
| `TestValidateExternalURL_WithAllowRFC1918_Permits10x` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_Permits172_16x` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_Permits192_168x` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_BlocksMetadata` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_BlocksLinkLocal` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_BlocksLoopback` | ✅ PASS |
| `TestValidateExternalURL_RFC1918BlockedByDefault` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_IPv4MappedIPv6Allowed` | ✅ PASS |
| `TestValidateExternalURL_WithAllowRFC1918_IPv4MappedMetadataBlocked` | ✅ PASS |

**Package result:** `ok github.com/Wikid82/charon/backend/internal/security 0.007s`

#### `internal/services` — RFC 1918 / Private IP tests

| Test | Result |
|---|---|
| `TestCheckMonitor_HTTP_LocalhostSucceedsWithPrivateIPBypass` | ✅ PASS |
| `TestCheckMonitor_TCP_AcceptsRFC1918Address` | ✅ PASS |

**Package result:** `ok github.com/Wikid82/charon/backend/internal/services 4.256s`

**Targeted total: 21/21 tests pass.**

---

### 3. Full Backend Coverage Suite

All 30 packages pass. No regressions introduced.

| Package | Coverage | Result |
|---|---|---|
| `internal/network` | 92.1% | ✅ PASS |
| `internal/security` | 94.1% | ✅ PASS |
| `internal/services` | 86.0% | ✅ PASS |
| `internal/api/handlers` | 86.3% | ✅ PASS |
| `internal/api/middleware` | 97.2% | ✅ PASS |
| `internal/caddy` | 96.8% | ✅ PASS |
| `internal/cerberus` | 93.8% | ✅ PASS |
| `internal/crowdsec` | 86.2% | ✅ PASS |
| `internal/models` | 97.5% | ✅ PASS |
| `internal/server` | 92.0% | ✅ PASS |
| *(all other packages)* | ≥78% | ✅ PASS |

**No packages failed. No regressions.**

All three PR-3 packages are above the 85% project threshold.

---

### 4. Linting

Initial run on the three modified packages revealed **one new issue introduced by PR-3** and **17 pre-existing issues** in unrelated service files.

#### New issue in PR-3 code

| File | Line | Issue | Action |
|---|---|---|---|
| `internal/network/safeclient_test.go:1130` | `bodyclose` — response body not closed | ✅ **Fixed** |

**Fix applied:** `TestNewSafeHTTPClient_AllowRFC1918_BlocksSSRFMetadata` was updated to assign the response and conditionally close the body:

```go
resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
if resp != nil {
    _ = resp.Body.Close()
}
```

A secondary `gosec G104` (unhandled error on `Body.Close()`) was also resolved by the explicit `_ =` assignment.

#### Pre-existing issues (not introduced by PR-3)

17 issues exist in `internal/services/` files unrelated to PR-3 (`backup_service.go`, `crowdsec_startup.go`, `dns_detection_service.go`, `emergency_token_service.go`, `mail_service.go`, `plugin_loader.go`, `docker_service_test.go`, etc.). These are pre-existing and out of scope for this PR.

#### Final lint state — PR-3 packages

```
golangci-lint run ./internal/network/... ./internal/security/...
0 issues.
```

**Result: ✅ PASS** for all PR-3 code.

---

### 5. Security Manual Check — Call Site Isolation

```
grep -rn "WithAllowRFC1918" --include="*.go" .
```

**Expected:** `WithAllowRFC1918` used only in its definition files and `uptime_service.go` (2 call sites).

**Actual findings:**

| File | Context |
|---|---|
| `internal/network/safeclient.go:259` | Definition of `WithAllowRFC1918()` (network layer option) |
| `internal/security/url_validator.go:161` | Definition of `WithAllowRFC1918()` (security layer option) |
| `internal/services/uptime_service.go:748` | Call site 1 — `security.WithAllowRFC1918()` (URL pre-validation) |
| `internal/services/uptime_service.go:767` | Call site 2 — `network.WithAllowRFC1918()` (dial-time SSRF guard, mirrors line 748) |
| `internal/network/safeclient_test.go` | Test uses only |
| `internal/security/url_validator_test.go` | Test uses only |

**Security assessment:**

- `WithAllowRFC1918` is **not present** in any notification, webhook, DNS, or other service call paths.
- The two `uptime_service.go` call sites are correctly paired: the security layer pre-validates the URL hostname, and the network layer enforces the same policy at dial time. This dual-layer approach is the correct defense-in-depth pattern.
- 169.254.x.x (link-local/cloud metadata), 127.x.x.x (loopback), and IPv4-mapped IPv6 equivalents remain blocked even with `AllowRFC1918=true`. Confirmed by test coverage.

**Result: ✅ PASS — Call site isolation confirmed. No scope creep.**

---

### 6. GORM Security Scan

**Skipped** per `testing.instructions.md` gate criteria: PR-3 does not touch `backend/internal/models/**` or any database/GORM query logic. Trigger condition not met.

---

### 7. Pre-Commit Hooks (lefthook)

```
lefthook run pre-commit
```

| Hook | Result |
|---|---|
| `check-yaml` | ✅ PASS |
| `actionlint` | ✅ PASS |
| `end-of-file-fixer` | ✅ PASS |
| `trailing-whitespace` | ✅ PASS |
| `dockerfile-check` | ✅ PASS |
| `shellcheck` | ✅ PASS |
| File-specific hooks (golangci-lint-fast, go-vet, etc.) | Skipped — no staged files (expected behavior) |

**Result: ✅ PASS** — All active hooks passed in 7.45s.

---

## Issues Found and Resolved

| # | Severity | File | Issue | Resolution |
|---|---|---|---|---|
| 1 | Low | `safeclient_test.go:1130` | `bodyclose`: response body not closed in test | Fixed — added conditional `resp.Body.Close()` |
| 2 | Low | `safeclient_test.go:1132` | `gosec G104`: unhandled error on `Body.Close()` | Fixed — added `_ =` explicit ignore |

No security vulnerabilities. No logic defects. No regressions.

---

## Final Verdict

**✅ APPROVED FOR COMMIT**

All audit steps passed. The two minor lint issues introduced by the new test code have been fixed. The implementation correctly scopes `WithAllowRFC1918` to the uptime service only, maintains dual-layer SSRF protection, and does not weaken any other security boundary. All 21 new tests pass. All 30 backend packages pass with zero regressions.
