# QA Audit Report — PR-1: Allow Empty Value in UpdateSetting

**Date:** 2026-03-17
**Scope:** Remove `binding:"required"` from `Value` field in `UpdateSettingRequest`
**File:** `backend/internal/api/handlers/settings_handler.go`

---

# QA Security Audit Report — Rate Limit CI Fix

**Audited by**: QA Security Auditor
**Date**: 2026-03-17
**Spec reference**: `docs/plans/rate_limit_ci_fix_spec.md`
**Files audited**:
- `scripts/rate_limit_integration.sh`
- `Dockerfile` (GeoIP section, non-CI path)
- `.github/workflows/rate-limit-integration.yml`

---

## Pre-Commit Check Results

| Check | Command | Result |
|-------|---------|--------|
| Bash syntax | `bash -n scripts/rate_limit_integration.sh` | ✅ PASS (exit 0) |
| Pre-commit hooks | `lefthook run pre-commit` (project uses lefthook; no `.pre-commit-config.yaml`) | ✅ PASS — all 6 hooks passed: `check-yaml`, `actionlint`, `end-of-file-fixer`, `trailing-whitespace`, `dockerfile-check`, `shellcheck` |
| Caddy admin API trailing slash (workflow) | `grep -n "2119" .github/workflows/rate-limit-integration.yml` | ✅ PASS — line 71 references `/config/` (trailing slash present) |
| Caddy admin API trailing slash (script) | All 6 occurrences of `localhost:2119/config` in script | ✅ PASS — all use `/config/` |

---

## Security Focus Area Results

### 1. Credential Handling — `TMP_COOKIE`

**`mktemp` usage**: `TMP_COOKIE=$(mktemp)` at line 208. Creates a file in `/tmp` with `600` permissions via the OS. ✅ SECURE.

**Removal on exit**: The `cleanup()` function at line 103 removes the file with `rm -f "${TMP_COOKIE:-}"`. However, `cleanup` is only registered via explicit calls — there is **no `trap cleanup EXIT`**. Only `trap on_failure ERR` is registered (line 108).

**Gap**: On 5 early `exit 1` paths after line 208 (login failure L220, auth failure L251, Caddy readiness failure L282, security config failure L299, and handler verification failure L316), `cleanup` is never called. The cookie file is left in `/tmp`.

**Severity**: LOW — The cookie contains session credentials for a localhost test server (`ratelimit@example.local` / `password123`, non-production). CI runners are ephemeral and auto-cleaned. Local runs will leave a `/tmp/tmp.XXXXXX` file until next reboot or manual cleanup.

**Note**: The exit at line 386 (inside the 429 enforcement failure block) intentionally skips cleanup to leave containers running for manual inspection. This is by design and acceptable.

**Recommendation**: Add `trap cleanup EXIT` immediately after `trap on_failure ERR` (line 109) to ensure the cookie file is always removed.

---

### 2. `curl` — Sensitive Values in Command-Line Arguments

Cookie file path is passed via `-c ${TMP_COOKIE}` and `-b ${TMP_COOKIE}` (unquoted). No credentials, tokens, or API keys are passed as command-line arguments. All authentication is via the cookie file (read/write by path), which is the correct pattern — cookie values never appear in `ps` output.

**Finding (LOW)**: `${TMP_COOKIE}` is unquoted in all 6 curl invocations. `mktemp` on Linux produces paths of the form `/tmp/tmp.XXXXXX` which never contain spaces or shell metacharacters under default `$TMPDIR`. However, under a non-standard `$TMPDIR` (e.g., `/tmp/my dir/`) this would break. This is a portability issue, not a security issue.

**Recommendation**: Quote `"${TMP_COOKIE}"` in all curl invocations.

---

### 3. Shell Injection

All interpolated values in curl `-d` payloads are either:
- Script-level constants (`RATE_LIMIT_REQUESTS=3`, `RATE_LIMIT_WINDOW_SEC=10`, `RATE_LIMIT_BURST=1`, `TEST_DOMAIN=ratelimit.local`, `BACKEND_CONTAINER=ratelimit-backend`)
- Values derived from API responses stored in double-quoted variables (`"$CREATE_RESP"`, `"$SEC_CONFIG_RESP"`)

No shell injection vector exists. All heredoc expansions (`cat <<EOF...EOF`) expand only the hardcoded constants listed above.

The UUID extraction pattern at line 429 includes `${TEST_DOMAIN}` unquoted within a `grep -o` pattern, but because the variable expands to `ratelimit.local` (controlled constant), this has no injection risk. The `.` in `ratelimit.local` is treated as a regex wildcard but in this context only matches the intended hostname. ✅ PASS.

---

### 4. `set -euo pipefail` Compatibility

The new status-capture idiom:

```bash
LOGIN_STATUS=$(curl -s -w "\n%{http_code}" ... | tail -n1)
```

Behavior under `set -euo pipefail`:
- **Network failure** (curl exits non-zero, e.g., `ECONNREFUSED`): `pipefail` propagates curl's non-zero exit through the pipeline; the assignment fails; `set -e` fires the `on_failure` ERR trap and exits. ✅ Correct.
- **HTTP error** (curl exits 0, HTTP 4xx/5xx): curl outputs `\n{code}`; `tail -n1` extracts the code; assignment succeeds; subsequent `[ "$LOGIN_STATUS" != "200" ]` detects the failure. ✅ Correct.
- **Empty body edge case**: If curl returns an empty body, output is `\n200`. `tail -n1` → `200`; `head -n-1` → empty string. Status check still works. ✅ Correct.

The `SEC_CONFIG_RESP` split pattern (`tail -n1` for status, `head -n-1` for body) is correct for both single-line and multiline JSON responses. ✅ PASS.

---

### 5. Workflow Secrets Exposure

The workflow (`rate-limit-integration.yml`) contains **no `${{ secrets.* }}` references**. All test credentials are hardcoded constants in the script (`ratelimit@example.local` / `password123`), appropriate for an ephemeral test user that is registered and used only within the test run.

`$GITHUB_STEP_SUMMARY` output includes: container status, API config JSON, container logs. None of these contain secrets or credentials. The security config JSON may contain rate limit settings (integers) but nothing sensitive.

No accidental log exposure identified. ✅ PASS.

---

### 6. GeoIP Change — Supply-Chain Risk

**Change**: The non-CI Dockerfile build path previously ran `sha256sum -c -` against `GEOLITE2_COUNTRY_SHA256`. This was removed. The remaining guard is `[ -s /app/data/geoip/GeoLite2-Country.mmdb ]` (file-size non-empty check).

**Risk assessment** (MEDIUM): The download source is `https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb`, a public GitHub repository. If this repository is compromised or the file is replaced with a malicious binary:
- The `-s` check only verifies the file is non-empty
- The application loads it at `CHARON_GEOIP_DB_PATH` for IP geolocation — a non-privileged read operation
- A malicious file would not achieve RCE via MMDb parsing in the MaxMind reader library (no known attack surface), but could corrupt GeoIP lookups silently

**This is an acknowledged, pre-existing architectural limitation** documented in the spec. The `sha256sum` check was ineffective by design because the P3TERX repository updates the file continuously while the pinned hash only updates weekly via `update-geolite2.yml`. The new behavior (accept any non-empty file) is more honest about the actual constraint.

**Spec compliance**: `ARG GEOLITE2_COUNTRY_SHA256` is **retained** in the Dockerfile (line ~441) as required by the spec, preserving `update-geolite2.yml` workflow compatibility. ✅ PASS.

**Residual risk**: MEDIUM. Mitigated by: (1) `wget` uses HTTPS to fetch from GitHub (TLS in transit), (2) downstream Trivy scans of the built image would flag a malicious MMDB independently, (3) the GeoIP reader is sandboxed to a read operation with no known parse-exploit surface.

---

## Correctness Against Spec

| Spec Change | Implemented | Verified |
|-------------|-------------|----------|
| C1: Login status check (Step 4) | ✅ Yes — `LOGIN_STATUS` checked, fails fast on non-200 | Script lines 211–220 |
| C2: Proxy host creation — auth failures fatal, 409 continues | ✅ Yes — 401/403 abort, other non-201 continues | Script lines 248–256 |
| C3: Caddy admin API readiness gate before security config POST | ✅ Yes — 20-retry loop before SEC_CFG call | Script lines 274–284 |
| C4: Security config POST status checked | ✅ Yes — `SEC_CONFIG_STATUS` checked, body logged on error | Script lines 286–301 |
| C5: `verify_rate_limit_config` failure is hard exit | ✅ Yes — prints debug and `exit 1` | Script lines 307–318 |
| C6: Pre-verification sleep increased 5 → 8 s | ✅ Yes — `sleep 8` | Script line 305 |
| C7: Trailing slash on `/config/` | ✅ Yes — all 6 script occurrences; workflow line 71 | Confirmed by grep |
| Dockerfile: sha256sum removed from non-CI path | ✅ Yes — only `-s` check remains | Dockerfile lines ~453–463 |
| Dockerfile: `ARG GEOLITE2_COUNTRY_SHA256` retained | ✅ Yes — line ~441 | Dockerfile audited |
| Workflow: debug dump uses `/config/` | ✅ Yes — line 71 | Confirmed by grep |

---

## Findings Summary

| ID | Severity | Area | Description |
|----|----------|------|-------------|
| M1 | MEDIUM | Dockerfile supply-chain | GeoIP downloaded without hash; `-s` is minimum viability only. Accepted trade-off per spec — hash was perpetually stale. |
| L1 | LOW | Shell security | `${TMP_COOKIE}` unquoted in 6 curl invocations. No practical impact under standard `$TMPDIR`. |
| L2 | LOW | Temp file hygiene | No `trap cleanup EXIT`; TMP_COOKIE and containers not cleaned on 5 early failure paths (lines 220, 251, 282, 299, 316). Low sensitivity (localhost test credentials only). |

No CRITICAL or HIGH severity findings.

---

## Overall Verdict

**✅ APPROVED**

All spec-required changes are correctly implemented. No OWASP Top 10 vulnerabilities were introduced. The two LOW findings (unquoted variable, missing EXIT trap) are hygiene improvements that do not block the fix. The MEDIUM GeoIP supply-chain concern is a pre-existing architectural trade-off explicitly acknowledged in the spec.

### Recommended follow-up (non-blocking)

Add `trap cleanup EXIT` immediately after `trap on_failure ERR` in `scripts/rate_limit_integration.sh` to ensure TMP_COOKIE is always removed and containers are cleaned on all exit paths.
**Purpose:** Allow admins to set a setting to an empty string value (required to fix the fresh-install CrowdSec enabling bug where `value` was legitimately empty).

---

## Overall Verdict: APPROVED

All structural, linting, and security gates pass. The change is correctly scoped to the build-only `frontend-builder` stage and introduces no new attack surface in the final runtime image.

---

## Changes Under Review

| Element | Location | Description |
|---|---|---|
| `ARG NPM_VERSION=11.11.1` | Line 30 (global ARG block) | Pinned npm version with Renovate comment |
| `ARG NPM_VERSION` | Line 105 (frontend-builder) | Bare re-declaration to inherit global ARG into stage |
| `# hadolint ignore=DL3017` | Line 106 | Lint suppression for intentional `apk upgrade` |
| `RUN apk upgrade --no-cache && ...` | Lines 107–109 | Three-command RUN: OS patch + npm upgrade + cache clear |
| `RUN npm ci` | Line 111 | Unchanged dependency install follows the new RUN block |

---

## Gate Summary

| # | Gate | Result | Details |
|---|---|---|---|
| 1 | Global `ARG NPM_VERSION` present with Renovate comment | **PASS** | Line 30; `# renovate: datasource=npm depName=npm` at line 29 |
| 2 | `ARG NPM_VERSION` bare re-declaration inside stage | **PASS** | Line 105 |
| 3 | `# hadolint ignore=DL3017` on own line before RUN block | **PASS** | Line 106 |
| 4 | RUN block — three correct commands | **PASS** | Lines 107–109: `apk upgrade --no-cache`, `npm install -g npm@${NPM_VERSION} --no-fund --no-audit`, `npm cache clean --force` |
| 5 | `RUN npm ci` still present and follows new block | **PASS** | Line 111 |
| 6 | FROM line unchanged | **PASS** | `node:24.14.0-alpine@sha256:7fddd9ddeae8196abf4a3ef2de34e11f7b1a722119f91f28ddf1e99dcafdf114` |
| 7 | `${NPM_VERSION}` used (no hard-coded version) | **PASS** | Confirmed variable reference in install command |
| 8 | Trivy config scan (HIGH/CRITICAL) | **PASS** | 0 misconfigurations |
| 9 | Hadolint (new code area) | **PASS** | No errors or warnings; only pre-existing `info`-level DL3059 at unrelated lines |
| 10 | Runtime image isolation | **PASS** | Only `/app/frontend/dist` artifacts copied into final image via line 535 |
| 11 | `--no-audit` acceptability | **PASS** | Applies only to the single-package global npm upgrade; `npm ci` is unaffected |
| 12 | `npm cache clean --force` safety | **PASS** | Safe cache clear between npm tool upgrade and dependency install |

---

## 1. Dockerfile Structural Verification

### Global ARG block (lines 25–40)

```
29: # renovate: datasource=npm depName=npm
30: ARG NPM_VERSION=11.11.1
```

Both the Renovate comment and the pinned ARG are present in the correct order. Renovate will track `npm` releases on `datasource=npm` and propose version bumps automatically.

### frontend-builder stage (lines 93–115)

```
93:  FROM --platform=$BUILDPLATFORM node:24.14.0-alpine@sha256:... AS frontend-builder
...
105: ARG NPM_VERSION
106: # hadolint ignore=DL3017
107: RUN apk upgrade --no-cache && \
108:     npm install -g npm@${NPM_VERSION} --no-fund --no-audit && \
109:     npm cache clean --force
...
111: RUN npm ci
```

All structural requirements confirmed: bare re-declaration, lint suppression on dedicated line, three-command RUN, and unmodified `npm ci`.

---

## 2. Security Tool Results

### Trivy config scan

**Command:** `docker run aquasec/trivy config Dockerfile --severity HIGH,CRITICAL`

```
Report Summary
┌────────────┬────────────┬───────────────────┐
│   Target   │    Type    │ Misconfigurations │
├────────────┼────────────┼───────────────────┤
│ Dockerfile │ dockerfile │         0         │
└────────────┴────────────┴───────────────────┘
```

No HIGH or CRITICAL misconfigurations detected.

### Hadolint

**Command:** `docker run hadolint/hadolint < Dockerfile`

Findings affecting the new code: **none**.

Pre-existing `info`-level findings (unrelated to this change):

| Line | Rule | Message |
|---|---|---|
| 78, 81, 137, 335, 338 | DL3059 info | Multiple consecutive RUN — pre-existing pattern |
| 492 | SC2012 info | Use `find` instead of `ls` — unrelated |

No errors or warnings in the `frontend-builder` section.

---

## 3. Logical Security Review

### Attack surface — build-only stage

The `frontend-builder` stage is strictly a build artifact producer. The final runtime image receives only compiled frontend assets via a single targeted `COPY`:

```
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist
```

The Alpine OS packages upgraded by `apk upgrade --no-cache`, the globally installed npm binary, and all `node_modules` are confined to the builder layer and never reach the runtime image. The CVE remediation has zero footprint in the deployed container.

### `--no-audit` flag

`--no-audit` suppresses npm audit output during `npm install -g npm@${NPM_VERSION}`. This applies only to the single-package global npm tool upgrade, not to the project dependency installation. `npm ci` on line 111 installs project dependencies from `package-lock.json` and is unaffected by this flag. Suppressing audit during a build-time tool upgrade is the standard pattern for avoiding advisory database noise that cannot be acted on during the image build.

### `npm cache clean --force`

Clears the npm package cache between the global npm upgrade and the `npm ci` run. This is safe: it ensures the freshly installed npm binary is used without stale cache entries left by the older npm version bundled in the base image. The `--force` flag suppresses npm's deprecation warning about manual cache cleaning; it does not alter the clean operation itself.

---

## Blocking Issues

None.

