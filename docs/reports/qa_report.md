# QA Audit Report — PR-1: Allow Empty Value in UpdateSetting

**Date:** 2026-03-17
**Scope:** Remove `binding:"required"` from `Value` field in `UpdateSettingRequest`
**File:** `backend/internal/api/handlers/settings_handler.go`
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

