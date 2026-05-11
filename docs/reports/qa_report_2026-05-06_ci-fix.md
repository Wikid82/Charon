# QA Audit Report — Backend CI Fix (Test Files + Script)

| Field | Value |
|-------|-------|
| Date | 2026-05-06 |
| Scope | Backend-only CI fix — test files and shell script |
| HEAD | `scripts/go-test-coverage.sh`, `orthrus/*_test.go`, `orthrus_handler_test.go` |
| Auditor | QA Security Agent |
| Verdict | **PASS** (nolint annotations recommended) |

## Changed Files

| File | Type |
|------|------|
| `scripts/go-test-coverage.sh` | Shell script — grep pattern fix |
| `backend/internal/orthrus/ca_test.go` | Go test |
| `backend/internal/orthrus/server_coverage_test.go` | Go test |
| `backend/internal/api/handlers/orthrus_handler_test.go` | Go test |

No production code was changed. No frontend changes.

---

## Check Results

### 1. Backend Coverage — PASS

```
Statement coverage: 88.5%
Line coverage:      88.6%
Threshold:          87% (minimum)
GO_TEST_STATUS:     0
```

All tests passed. No race conditions detected. Coverage exceeds the 87% gate.

---

### 2. Shell Script Syntax (`bash -n`) — PASS

```
Exit: 0 — no syntax errors in scripts/go-test-coverage.sh
```

---

### 3. shellcheck — PASS

```
shellcheck --severity=error scripts/go-test-coverage.sh
Exit: 0 — no errors
```

---

### 4. Pre-commit Hooks — SKIP (Not Applicable)

Project uses **lefthook**, not pre-commit. No `.pre-commit-config.yaml` exists.
Equivalent per-file checks (shellcheck, golangci-lint) were run individually.

---

### 5. GORM Security Scanner — PASS

```
Scanned: 46 Go files (2611 lines)
CRITICAL: 0
HIGH:     0
MEDIUM:   0
INFO:     2 (pre-existing FK index suggestions in models/user.go — unrelated)
Exit:     0 ✅
```

---

### 6. Trivy Filesystem Scan — PARTIAL (Infrastructure Limitation)

The installed Trivy snap package cannot traverse the `/projects` mount point (snap sandbox restriction). Running from within `backend/` yields `num=0 language-specific files`.

**Mitigation — reviewed existing `trivy-image-report.json`:**

| Severity | CVE | Package | In `go.mod`? |
|----------|-----|---------|-------------|
| HIGH | GHSA-6g7g-w4f8-9c9x | `buger/jsonparser` | **No** |
| HIGH | GHSA-jqcq-xjh3-6g23 | `jackc/pgproto3/v2` | **No** |

Both packages are absent from `backend/go.mod` and `backend/go.sum`. These findings are from the container base image layer, not the Go module. They are pre-existing and unrelated to this change. Trivy in CI will provide the authoritative scan.

---

### 7. golangci-lint — CONDITIONAL PASS

```
golangci-lint run ./internal/orthrus/... ./internal/api/handlers/...
38 issues total: 7 in changed files, 31 in pre-existing files
```

#### Issues in Changed Files (`ca_test.go`) — All False Positives

| Lines | Rule | Assessment |
|-------|------|------------|
| 107, 120, 134 | G306: WriteFile perm > 0600 | `0o644` on `.crt` placeholder files in `t.TempDir()`. Certificate files are public by design. The corresponding `.key` files correctly use `0o600`/`0o000`. |
| 155–156 | G302: chmod to `0o555` | Intentionally read-only directory for `TestNewInternalCA_ReadOnlyDataRoot` error-path test. Restored via `t.Cleanup`. |
| 165–166 | G301/G302: MkdirAll/chmod `0o555` | Same pattern for `TestNewInternalCA_ReadOnlyKeysDir`. |

All 7 findings are contextually correct test patterns. No real security risk.

**Recommendation**: Add `//nolint:gosec // test fixture: cert files are public; permissions set intentionally for error-condition coverage` to the 7 flagged lines to suppress CI lint noise.

#### Issues in Pre-existing Files — Not Blocking

31 issues across `crowdsec_handler.go`, `audit_log_handler_test.go`, `notification_provider_handler.go`, and others. These predate this change and are not part of the scope.

---

## Summary

| Check | Result | Notes |
|-------|--------|-------|
| Backend coverage (88.5% stmt / 88.6% line) | ✅ PASS | Exceeds 87% gate |
| Shell script syntax (`bash -n`) | ✅ PASS | |
| shellcheck | ✅ PASS | |
| Pre-commit hooks | ⏭ SKIP | Project uses lefthook |
| GORM security scan | ✅ PASS | 0 CRITICAL / 0 HIGH |
| Trivy filesystem scan | ⚠ PARTIAL | Snap sandbox prevents scan; existing image report shows 0 Go-dep vulns |
| golangci-lint (changed files) | ✅ CONDITIONAL PASS | 7 false-positive gosec findings; nolint annotations recommended |
| golangci-lint (pre-existing) | ℹ NOTE | 31 issues, pre-existing, out of scope |

### Verdict: PASS

Safe to merge. No regressions in coverage, no real security issues introduced, no production code changes. The one actionable item is adding `//nolint:gosec` annotations to 7 lines in `ca_test.go` to clean up CI lint output.
