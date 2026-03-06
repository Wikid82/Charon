# QA Report — PR #800 (feature/beta-release)

**Date:** 2026-03-06
**Auditor:** QA Security Agent (QA Security Mode)
**Branch:** `feature/beta-release`
**PR:** [#800](https://github.com/Wikid82/charon/pull/800)
**Scope:** Security hardening — WebSocket origin validation, CodeQL email-injection suppressions, Semgrep pipeline refactor, `security-local` Makefile target

---

## Executive Summary

All 10 QA steps pass. No blocking issues. Backend coverage is **87.9%** (threshold: 85%). Frontend coverage is **89.73% lines** (threshold: 87%). Patch coverage is **90.6%** (overall). Zero static-analysis findings. Zero security scan findings. Security changes correctly implemented and individually verified.

**Overall Verdict: ✅ PASS — Ready to merge**

---

## QA Step Results

### Step 1: Backend Build, Vet, and Tests

#### 1a — `go build ./...`

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Exit Code** | 0 |
| **Output** | Clean — no errors or warnings |
| **Command** | `cd /projects/Charon && go build ./...` |

#### 1b — `go vet ./...`

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Exit Code** | 0 |
| **Output** | Clean — no issues |
| **Command** | `cd /projects/Charon && go vet ./...` |

#### 1c — `go test ./...`

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Packages** | 31 tested, 2 with no test files (skipped) |
| **Failures** | 0 |
| **Slowest Packages** | `crowdsec` (93s), `services` (74s), `handlers` (66s) |
| **Command** | `cd /projects/Charon && go test ./...` |

---

### Step 2: Backend Coverage Report

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Statement Coverage** | **87.9%** (threshold: 85%) |
| **Line Coverage** | **88.1%** (threshold: 85%) |
| **Command** | `bash /projects/Charon/scripts/go-test-coverage.sh` |

**Packages below 85% (pre-existing, not caused by this PR):**

| Package | Coverage | Notes |
|---------|----------|-------|
| `cmd/api` | 82.8% | Pre-existing; bootstrap/init code difficult to unit-test |
| `internal/util` | 78.0% | Pre-existing; utility helpers with edge-case paths |

All other packages meet or exceed the 85% threshold.

---

### Step 3: Frontend Tests and Coverage

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Test Files** | 27 |
| **Tests Passed** | 581 |
| **Tests Skipped** | 1 |
| **Failures** | 0 |
| **Line Coverage** | **89.73%** (threshold: 87%) |
| **Statement Coverage** | 89.0% |
| **Function Coverage** | 86.26% |
| **Branch Coverage** | 81.07% (not enforced — only `lines` is configured) |
| **Command** | `cd /projects/Charon/frontend && npm run test -- --coverage --reporter=verbose` |

**Note:** Branch coverage at 81.07% is below a 85% target but is **not an enforced threshold** in the Vitest configuration. Only line coverage is enforced (configured at 87%). This is pre-existing and not caused by this PR.

---

### Step 4: TypeScript Type Check

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Errors** | 0 |
| **Exit Code** | 0 |
| **Command** | `cd /projects/Charon/frontend && npm run type-check` |

---

### Step 5: Pre-commit Hooks (Non-Semgrep)

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Hooks Passed** | 15/15 |
| **Semgrep** | Correctly absent (now at `stages: [pre-push]` — see Security Changes) |
| **Command** | `cd /projects/Charon && pre-commit run --all-files` |

**Hooks executed and their status:**

| Hook | Status |
|------|--------|
| fix end of files | Passed |
| trim trailing whitespace | Passed |
| check yaml | Passed |
| check for added large files | Passed |
| shellcheck | Passed |
| actionlint (GitHub Actions) | Passed |
| dockerfile validation | Passed |
| Go Vet | Passed |
| golangci-lint (Fast Linters - BLOCKING) | Passed |
| Check .version matches latest Git tag | Passed |
| Prevent large files not tracked by LFS | Passed |
| Prevent committing CodeQL DB artifacts | Passed |
| Prevent committing data/backups files | Passed |
| Frontend TypeScript Check | Passed |
| Frontend Lint (Fix) | Passed |

---

### Step 6: Local Patch Coverage Preflight

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Overall Patch Coverage** | **90.6%** |
| **Backend Patch Coverage** | **90.4%** |
| **Frontend Patch Coverage** | **100%** |
| **Artifacts** | `test-results/local-patch-report.md` ✅, `test-results/local-patch-report.json` ✅ |
| **Command** | `bash /projects/Charon/scripts/local-patch-report.sh` |

**Files with partially covered changed lines:**

| File | Patch Coverage | Uncovered Lines | Notes |
|------|---------------|-----------------|-------|
| `backend/internal/services/mail_service.go` | 84.2% | 334–335, 339–340, 346–347, 351–352, 540 | SMTP sink lines; CodeQL `[go/email-injection]` suppressions applied; hard to unit-test directly |
| `backend/internal/services/notification_service.go` | 97.6% | 1 line | Minor branch |

Frontend patch coverage is 100% — all changed lines in `notifications.test.ts` and `SecurityNotificationSettingsModal.test.tsx` are covered.

---

### Step 7: Static Analysis

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Issues** | 0 |
| **Linters** | golangci-lint (`--config .golangci-fast.yml`) |
| **Command** | `make -C /projects/Charon lint-fast` |

---

### Step 8: Semgrep Validation (Manual)

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **Findings** | 0 |
| **Rules Applied** | 42 (from `p/golang` ruleset) |
| **Files Scanned** | 182 |
| **Command** | `SEMGREP_CONFIG=p/golang bash /projects/Charon/scripts/pre-commit-hooks/semgrep-scan.sh` |

This step validates the new `p/golang` ruleset configuration introduced in this PR.

---

### Step 9: `make security-local`

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **govulncheck** | 0 vulnerabilities |
| **Semgrep (p/golang)** | 0 findings |
| **Exit Code** | 0 |
| **Command** | `make -C /projects/Charon security-local` |

This is the new `security-local` Makefile target introduced by this PR — both constituent checks pass.

---

### Step 10: Git Diff Summary

`git diff --name-only` reports **9 changed files** (unstaged relative to last commit):

| File | Category | Change Summary |
|------|----------|----------------|
| `.pre-commit-config.yaml` | Config | `semgrep-scan` moved from `stages: [manual]` → `stages: [pre-push]` |
| `Makefile` | Build | Added `security-local` target |
| `backend/internal/api/handlers/cerberus_logs_ws.go` | Backend | Added `# nosemgrep` annotation on `.Upgrade()` call |
| `backend/internal/api/handlers/logs_ws.go` | Backend | Replaced insecure `CheckOrigin: return true` with host-based validation |
| `backend/internal/api/handlers/settings_handler.go` | Backend | Added documentation comment above `SendEmail` call |
| `backend/internal/api/handlers/user_handler.go` | Backend | Added documentation comments above 2 `SendInvite` calls |
| `backend/internal/services/mail_service.go` | Backend | Added `// codeql[go/email-injection]` suppressions on 3 SMTP sink lines |
| `docs/plans/current_spec.md` | Docs | Spec updates |
| `scripts/pre-commit-hooks/semgrep-scan.sh` | Scripts | Default config `auto`→`p/golang`; added `--severity` flags; scope to `frontend/src` |

**HEAD commit (`ee224adc`) additionally included** (committed changes not in the diff above):

- `backend/internal/models/notification_config.go`
- `backend/internal/services/mail_service_test.go`
- `backend/internal/services/notification_service.go`
- `backend/internal/services/notification_service_test.go`
- `frontend/src/api/notifications.test.ts`
- `frontend/src/components/__tests__/SecurityNotificationSettingsModal.test.tsx`

---

### Bonus: GORM Security Scan

Triggered because `backend/internal/models/notification_config.go` changed in HEAD.

| Metric | Result |
|--------|--------|
| **Status** | ✅ PASS |
| **CRITICAL** | 0 |
| **HIGH** | 0 |
| **MEDIUM** | 0 |
| **INFO** | 2 (pre-existing: missing index suggestions on `UserPermittedHost` foreign keys in `user.go`) |
| **Files Scanned** | 41 Go model files |
| **Command** | `bash /projects/Charon/scripts/scan-gorm-security.sh --check` |

The 2 INFO findings are pre-existing and unrelated to this PR.

---

## Security Change Verification

### 1. WebSocket Origin Validation (`logs_ws.go`)

**Change:** Replaced `CheckOrigin: func(r *http.Request) bool { return true }` with proper host-based validation.

**Implementation verified:**
- Imports `"net/url"` (line 5)
- `CheckOrigin` function parses `Origin` header via `url.Parse()`
- Compares `originURL.Host` to `r.Host`, honoring `X-Forwarded-Host` for proxy scenarios
- Returns `false` on parse error or host mismatch

**Assessment:** ✅ Correct. Addresses the Semgrep `websocket-missing-origin-check` finding. Guards against cross-site WebSocket hijacking (CWE-346).

---

### 2. Nosemgrep Annotation (`cerberus_logs_ws.go`)

**Change:** Added `# nosemgrep: go.gorilla.security.audit.websocket-missing-origin-check.websocket-missing-origin-check` on the `.Upgrade()` call.

**Justification verified:** This handler uses the shared `upgrader` variable defined in `logs_ws.go`, which now has a valid `CheckOrigin` function. The annotation is correct — the rule fires on the call site but the underlying `upgrader` is already secured.

**Assessment:** ✅ Correct. Suppression is justified and scoped to a single line.

---

### 3. CodeQL Email-Injection Suppressions (`mail_service.go`)

**Change:** Added `// codeql[go/email-injection]` on lines 370, 534, 588 (SMTP `smtp.SendMail()` calls).

**Assessment:** ✅ Correct. Each suppressed sink is protected by documented 4-layer defense:
1. `sanitizeForEmail()` — strips `\r`/`\n` from user inputs
2. `rejectCRLF()` — hard-rejects strings containing CRLF sequences
3. `encodeSubject()` — RFC 2047 encodes email subject
4. `html.EscapeString()` / `sanitizeEmailBody()` — HTML-escapes body content

Suppressions are placed at the exact CodeQL sink lines per the CodeQL suppression spec.

---

### 4. Semgrep Pipeline Refactor

**Changes verified:**

| Change | File | Assessment |
|--------|------|------------|
| `stages: [pre-push]` | `.pre-commit-config.yaml` | ✅ Semgrep now runs on `git push`, not every commit. Faster commit loop. |
| Default config `auto` → `p/golang` | `semgrep-scan.sh` | ✅ Deterministic, focused ruleset. `auto` was non-deterministic. |
| `--severity ERROR --severity WARNING` flags | `semgrep-scan.sh` | ✅ Explicitly filters noise; only ERROR/WARNING findings are blocking. |
| Scope to `frontend/src` | `semgrep-scan.sh` | ✅ Focuses frontend scanning on source directory. |

---

### 5. `security-local` Makefile Target

**Target verified (Makefile line 149):**
```makefile
security-local: ## Run govulncheck + semgrep (p/golang) before push — fast local gate
    @echo "[1/2] Running govulncheck..."
    @./scripts/security-scan.sh
    @echo "[2/2] Running Semgrep (p/golang, ERROR+WARNING)..."
    @SEMGREP_CONFIG=p/golang ./scripts/pre-commit-hooks/semgrep-scan.sh
```

**Assessment:** ✅ Correct. Provides a fast, developer-friendly pre-push gate that mirrors the CI security checks.

---

## Gotify Token Review

- No Gotify tokens found in diffs, test output, or log artifacts
- No tokenized URLs (e.g., `?token=...`) exposed in any output
- ✅ Clean

---

## Issues and Observations

### Blocking Issues

**None.**

### Non-Blocking Observations

| Observation | Severity | Notes |
|-------------|----------|-------|
| `cmd/api` backend coverage at 82.8% | ⚠️ INFO | Pre-existing. Bootstrap/init code. Not caused by this PR. |
| `internal/util` backend coverage at 78.0% | ⚠️ INFO | Pre-existing. Utility helpers. Not caused by this PR. |
| Frontend branch coverage at 81.07% | ⚠️ INFO | Pre-existing. Threshold not enforced (only `lines` is). |
| `mail_service.go` patch coverage at 84.2% | ⚠️ INFO | SMTP sink lines are intentionally difficult to unit-test. CodeQL suppressions are the documented mitigation. |
| GORM INFO findings (missing FK indexes) | ⚠️ INFO | Pre-existing in `user.go`. Unrelated to this PR. |

---

## Final Determination

| Step | Status |
|------|--------|
| 1a. `go build ./...` | ✅ PASS |
| 1b. `go vet ./...` | ✅ PASS |
| 1c. `go test ./...` | ✅ PASS |
| 2. Backend coverage | ✅ PASS — 87.9% / 88.1% |
| 3. Frontend tests + coverage | ✅ PASS — 581 pass, 89.73% lines |
| 4. TypeScript type check | ✅ PASS |
| 5. Pre-commit hooks | ✅ PASS — 15/15 |
| 6. Local patch coverage preflight | ✅ PASS — 90.6% overall |
| 7. `make lint-fast` | ✅ PASS — 0 issues |
| 8. Semgrep manual validation | ✅ PASS — 0 findings |
| 9. `make security-local` | ✅ PASS |
| 10. Git diff | ✅ 9 changed files |
| GORM security scan | ✅ PASS — 0 CRITICAL/HIGH |

**✅ OVERALL: PASS — All gates met. No blocking issues. Ready to merge.**
