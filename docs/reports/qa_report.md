# QA Report: Integration Script Port Fix & curl→wget Remediation

**Date:** 2026-03-14
**Branch:** `feature/beta-release`
**Scope:** 6 shell scripts in `scripts/` — one-line changes each
**Reviewer:** QA Security Agent

---

## Overall Verdict: PASS

All 6 modified scripts pass syntax validation, ShellCheck, pre-commit hooks, verification greps, security review, and Trivy scanning. No new issues were introduced. The changes are minimal, correct, and safe for merge.

---

## Change Summary

| File | Change | Line |
|------|--------|------|
| `scripts/cerberus_integration.sh` | Add `-e PORT=80` to `docker run ... mccutchen/go-httpbin` | L174 |
| `scripts/waf_integration.sh` | Add `-e PORT=80` to `docker run ... mccutchen/go-httpbin` | L167 |
| `scripts/rate_limit_integration.sh` | Add `-e PORT=80` to `docker run ... mccutchen/go-httpbin` | L187 |
| `scripts/coraza_integration.sh` | Add `-e PORT=80` to `docker run ... mccutchen/go-httpbin` | L158 |
| `scripts/crowdsec_startup_test.sh` | Replace `curl -sf` with `wget -qO -` in `docker exec` | L179 |
| `scripts/diagnose-test-env.sh` | Replace `curl -sf` with `wget -qO /dev/null` in `docker exec` | L104 |

---

## Gate Summary

| # | Gate | Result | Details |
|---|------|--------|---------|
| 1 | Syntax Validation (`bash -n`) | **PASS** | All 6 scripts parse cleanly |
| 2 | ShellCheck (error severity) | **PASS** | 0 errors; matches lefthook `--severity=error` |
| 3 | ShellCheck (all severities) | **PASS** | No findings on any modified line; all findings pre-existing |
| 4 | Pre-commit Hooks (lefthook) | **PASS** | All 6 hooks passed (shellcheck, actionlint, yaml, whitespace, eof, dockerfile) |
| 5 | Verification: go-httpbin PORT | **PASS** | 4/4 `docker run` lines have `-e PORT=80` |
| 6 | Verification: docker exec curl | **PASS** | 0 executed curl calls; 2 echo-only references (hints) |
| 7 | Security Review | **PASS** | No secrets, credentials, injection vectors, or Gotify tokens |
| 8 | Trivy Filesystem Scan | **PASS** | 0 secrets, 0 misconfigurations |

---

## 1. Syntax Validation (`bash -n`)

| Script | Result |
|--------|--------|
| `scripts/cerberus_integration.sh` | PASS |
| `scripts/waf_integration.sh` | PASS |
| `scripts/rate_limit_integration.sh` | PASS |
| `scripts/coraza_integration.sh` | PASS |
| `scripts/crowdsec_startup_test.sh` | PASS |
| `scripts/diagnose-test-env.sh` | PASS |

---

## 2. ShellCheck

### At error severity (`--severity=error`, matching lefthook pre-commit)

**Result: PASS** — Zero errors across all 6 scripts. Exit code 0.

### At default severity (full informational audit)

Exit code 1 (findings present, all `note` or `warning` severity).

| Script | Findings | Severity | On Modified Lines? |
|--------|----------|----------|--------------------|
| `cerberus_integration.sh` | 2× SC2086 (unquoted variable) | note | No (L204, L219) |
| `waf_integration.sh` | ~30× SC2317 (unreachable code in trap), 3× SC2086 | note | No |
| `rate_limit_integration.sh` | 9× SC2086 | note | No |
| `coraza_integration.sh` | 10× SC2086, 2× SC2034 (unused variable) | note/warning | No |
| `crowdsec_startup_test.sh` | ~10× SC2317, 1× SC2086 | note | No |
| `diagnose-test-env.sh` | 1× SC2034 (unused variable) | warning | No |

**No ShellCheck findings on any of the 6 modified lines.** All findings are pre-existing.

---

## 3. Pre-commit Hooks (lefthook)

Ran `lefthook run pre-commit`:

| Hook | Result | Duration |
|------|--------|----------|
| check-yaml | PASS | 1.93s |
| actionlint | PASS | 4.36s |
| end-of-file-fixer | PASS | 9.23s |
| trailing-whitespace | PASS | 9.49s |
| dockerfile-check | PASS | 10.41s |
| shellcheck | PASS | 11.24s |

Hooks for Go, TypeScript, and Semgrep correctly skipped (no matching files).

---

## 4. Verification Greps

### 4a. All `mccutchen/go-httpbin` `docker run` instances have `-e PORT=80`

```
scripts/cerberus_integration.sh:174:  docker run ... -e PORT=80 mccutchen/go-httpbin
scripts/waf_integration.sh:167:       docker run ... -e PORT=80 mccutchen/go-httpbin
scripts/rate_limit_integration.sh:187:docker run ... -e PORT=80 mccutchen/go-httpbin
scripts/coraza_integration.sh:158:    docker run ... -e PORT=80 mccutchen/go-httpbin
```

Remaining `mccutchen/go-httpbin` matches are `docker pull` lines (no `-e PORT` needed).

**Result: PASS** — 4/4 confirmed.

### 4b. Zero executed `docker exec ... curl` calls

Only 2 matches found in `scripts/verify_crowdsec_app_config.sh` (L94–95) — both inside `echo` statements (user hint text, not executed). Confirmed by manual review.

**Result: PASS** — 0 executed `docker exec ... curl` calls.

---

## 5. Security Review

| Check | Result | Notes |
|-------|--------|-------|
| Secrets/credentials in diff | PASS | `git diff | grep -iE "password\|secret\|key\|token\|credential\|auth"` — no matches |
| Gotify tokens | PASS | `grep -rn "Gotify\|gotify\|token="` across all 6 scripts — no matches |
| Injection vectors | PASS | `-e PORT=80` is a static literal; no user-controlled input flows into new code |
| Command injection | PASS | `wget -qO` flags are hardcoded; no interpolated user input |
| SSRF | N/A | URLs are internal container addresses (127.0.0.1, localhost) in CI-only scripts |
| Sensitive data in logs | PASS | No new log/echo statements added |
| URL query parameters | PASS | No tokenized URLs (e.g., `?token=...`) in changed or adjacent code |

---

## 6. Trivy Filesystem Scan

Scanners: `secret,misconfig`. Severity filter: `CRITICAL,HIGH,MEDIUM`.

| Target | Type | Secrets | Misconfigurations |
|--------|------|---------|-------------------|
| `backend/go.mod` | gomod | — | — |
| `frontend/package-lock.json` | npm | — | — |
| `package-lock.json` | npm | — | — |
| `Dockerfile` | dockerfile | — | 0 |
| `playwright/.auth/user.json` | text | 0 | — |

**Result: 0 findings. Exit code 0.**

---

## 7. Scope Exclusions

| Check | Excluded? | Justification |
|-------|-----------|---------------|
| E2E Playwright tests | Yes | Scripts are CI-only; no UI changes |
| Backend unit coverage | Yes | No Go code changes |
| Frontend unit coverage | Yes | No TypeScript/React changes |
| Docker image scan | Yes | No Dockerfile or image changes |
| CodeQL | Yes | No Go or JavaScript changes |
| GORM security scan | Yes | No model/database changes |
| Local patch coverage report | Yes | No application code; scripts not coverage-tracked |

---

## 8. Pre-existing Issues (Not Introduced by This Change)

| Category | Count | Scripts Affected | Risk |
|----------|-------|-----------------|------|
| SC2086 (unquoted variables) | ~25 | All 6 | Low — CI-controlled variables |
| SC2317 (unreachable code) | ~40 | waf, crowdsec | None — trap cleanup functions (ShellCheck false positive) |
| SC2034 (unused variables) | 3 | coraza, diagnose | Low — may be planned for future use |

---

## Remaining Validation (CI)

The integration scripts cannot be executed locally without a built `charon:local` image and Docker network. Full end-to-end validation will occur when the PR triggers CI:

- `.github/workflows/cerberus-integration.yml`
- `.github/workflows/waf-integration.yml`
- `.github/workflows/rate-limit-integration.yml`
- `.github/workflows/crowdsec-integration.yml`
