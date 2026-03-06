# Dockerfile Version Consolidation Plan

**Status:** Draft
**Created:** 2026-03-06
**Scope:** Single PR — consolidate all duplicated version references in `Dockerfile` into top-level ARGs

---

## 1. Problem Statement

When Go was bumped from 1.26.0 to 1.26.1, the version appeared in 4 separate `FROM golang:X.XX.X-alpine` lines. Renovate's Docker manager updated some but not all, requiring manual fixes. The same class of duplication exists for Alpine base images, CrowdSec version/SHA, and Go dependency patch versions.

**Root cause:** Version values are duplicated across build stages instead of being declared once at the top level and referenced via ARG interpolation.

**EARS Requirement:**
> WHEN a dependency version is updated (by Renovate or manually),
> THE SYSTEM SHALL require exactly one edit location per version.

---

## 2. Full Inventory of Duplicated Version References

### 2.1 Go Toolchain Version (`golang:1.26.1-alpine`)

| # | Line | Stage | Current Code |
|---|------|-------|-------------|
| 1 | 47 | `gosu-builder` | `FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS gosu-builder` |
| 2 | 98 | `backend-builder` | `FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS backend-builder` |
| 3 | 200 | `caddy-builder` | `FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS caddy-builder` |
| 4 | 297 | `crowdsec-builder` | `FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS crowdsec-builder` |

**Renovate annotation:** Each line currently has its own `# renovate: datasource=docker depName=golang` comment. With 4 copies, Renovate may match and PR-update only a subset when generating grouped PRs, leaving the others stale.

### 2.2 Alpine Base Image (`alpine:3.23.3@sha256:...`)

| # | Line | Stage/Context | Current Code |
|---|------|--------------|-------------|
| 1 | 30 | Top-level ARG | `ARG CADDY_IMAGE=alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659` |
| 2 | 358 | `crowdsec-fallback` | `FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS crowdsec-fallback` |

**Problem:** The `CADDY_IMAGE` ARG is already top-level with a Renovate annotation (line 29), but the `crowdsec-fallback` stage hardcodes the same version+digest independently with its own Renovate annotation (line 357). Both can drift.

### 2.3 CrowdSec Version + SHA256

| # | Lines | Stage | Current Code |
|---|-------|-------|-------------|
| 1 | 303–305 | `crowdsec-builder` | `ARG CROWDSEC_VERSION=1.7.6` + `ARG CROWDSEC_RELEASE_SHA256=704e37...` |
| 2 | 367–368 | `crowdsec-fallback` | `ARG CROWDSEC_VERSION=1.7.6` + `ARG CROWDSEC_RELEASE_SHA256=704e37...` |

**Problem:** Both stages independently declare the same version and SHA. The `# renovate:` annotation exists on each, so Renovate may update one and miss the other in a grouped PR.

### 2.4 Go Dependency Patch: `github.com/expr-lang/expr`

| # | Line | Stage | Current Code |
|---|------|-------|-------------|
| 1 | 255 | `caddy-builder` | `go get github.com/expr-lang/expr@v1.17.7` |
| 2 | 325 | `crowdsec-builder` | `go get github.com/expr-lang/expr@v1.17.7` |

### 2.5 Go Dependency Patch: `golang.org/x/net`

| # | Line | Stage | Current Code |
|---|------|-------|-------------|
| 1 | 259 | `caddy-builder` | `go get golang.org/x/net@v0.51.0` |
| 2 | 327 | `crowdsec-builder` | `go get golang.org/x/net@v0.51.0` |

### 2.6 Non-Duplicated References (For Completeness)

These appear only once and need no consolidation but are noted for context:

| Dependency | Line | Stage |
|-----------|------|-------|
| `golang.org/x/crypto@v0.46.0` | 326 | `crowdsec-builder` only |
| `github.com/hslatman/ipstore@v0.4.0` | 257 | `caddy-builder` only |
| `github.com/slackhq/nebula@v1.9.7` | 268 | `caddy-builder` only (conditional) |

---

## 3. Proposed Top-Level ARG Consolidation

### 3.1 New Top-Level ARGs

Add these after the existing `ARG BUILD_DEBUG=0` block (around line 9) and before the Caddy-related ARGs:

```dockerfile
# ---- Pinned Toolchain Versions ----
# renovate: datasource=docker depName=golang versioning=docker
ARG GO_VERSION=1.26.1

# renovate: datasource=docker depName=alpine versioning=docker
ARG ALPINE_IMAGE=alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

# ---- CrowdSec Version ----
# renovate: datasource=github-releases depName=crowdsecurity/crowdsec
ARG CROWDSEC_VERSION=1.7.6
# CrowdSec fallback tarball checksum (v${CROWDSEC_VERSION})
ARG CROWDSEC_RELEASE_SHA256=704e37121e7ac215991441cef0d8732e33fa3b1a2b2b88b53a0bfe5e38f863bd

# ---- Shared Go Security Patches ----
# renovate: datasource=go depName=github.com/expr-lang/expr
ARG EXPR_LANG_VERSION=1.17.7
# renovate: datasource=go depName=golang.org/x/net
ARG XNET_VERSION=0.51.0
```

### 3.2 ARG Naming Conventions

| ARG Name | Value | Tracks |
|----------|-------|--------|
| `GO_VERSION` | `1.26.1` | Go toolchain version (bare semver, appended to `golang:` in FROM) |
| `ALPINE_IMAGE` | `alpine:3.23.3@sha256:...` | Full image reference with digest pin |
| `CROWDSEC_VERSION` | `1.7.6` | CrowdSec release tag (moved from per-stage) |
| `CROWDSEC_RELEASE_SHA256` | `704e37...` | Tarball checksum for fallback (moved from per-stage) |
| `EXPR_LANG_VERSION` | `1.17.7` | `github.com/expr-lang/expr` Go module version |
| `XNET_VERSION` | `0.51.0` | `golang.org/x/net` Go module version |

### 3.3 Existing ARG Rename

The current `ARG CADDY_IMAGE=alpine:3.23.3@sha256:...` (line 30) should be **replaced** by the new `ALPINE_IMAGE` ARG. All references to `CADDY_IMAGE` downstream must be updated to `ALPINE_IMAGE`.

---

## 4. Line-by-Line Change Specification

### 4.1 New Top-Level ARG Block

**Location:** After line 9 (`ARG BUILD_DEBUG=0`), insert the new ARGs before Caddy version ARGs.

```
Old (lines 9–14):
  ARG BUILD_DEBUG=0
  <blank>
  # Allow pinning Caddy version...

New (lines 9–28):
  ARG BUILD_DEBUG=0

  # ---- Pinned Toolchain Versions ----
  # renovate: datasource=docker depName=golang versioning=docker
  ARG GO_VERSION=1.26.1

  # renovate: datasource=docker depName=alpine versioning=docker
  ARG ALPINE_IMAGE=alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

  # ---- CrowdSec Version ----
  # renovate: datasource=github-releases depName=crowdsecurity/crowdsec
  ARG CROWDSEC_VERSION=1.7.6
  ARG CROWDSEC_RELEASE_SHA256=704e37121e7ac215991441cef0d8732e33fa3b1a2b2b88b53a0bfe5e38f863bd

  # ---- Shared Go Security Patches ----
  # renovate: datasource=go depName=github.com/expr-lang/expr
  ARG EXPR_LANG_VERSION=1.17.7
  # renovate: datasource=go depName=golang.org/x/net
  ARG XNET_VERSION=0.51.0

  # Allow pinning Caddy version...
```

### 4.2 Remove `CADDY_IMAGE` ARG (Old Alpine Reference)

**Line 29-30** — Remove the entire `CADDY_IMAGE` ARG block:
```
Old:
  # renovate: datasource=docker depName=alpine versioning=docker
  ARG CADDY_IMAGE=alpine:3.23.3@sha256:...

New:
  (removed — replaced by top-level ALPINE_IMAGE)
```

Find all downstream references to `${CADDY_IMAGE}` and replace with `${ALPINE_IMAGE}`. Search for usage in the runtime stage (likely `FROM ${CADDY_IMAGE}`).

### 4.3 Go FROM Lines (4 Changes)

Each `FROM golang:1.26.1-alpine` line changes to `FROM golang:${GO_VERSION}-alpine`. The Renovate comment above each is **removed** (the single top-level annotation handles tracking).

#### 4.3.1 gosu-builder (line ~47)
```
Old:
  # renovate: datasource=docker depName=golang
  FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS gosu-builder

New:
  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS gosu-builder
```

#### 4.3.2 backend-builder (line ~98)
```
Old:
  # renovate: datasource=docker depName=golang
  FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS backend-builder

New:
  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS backend-builder
```

#### 4.3.3 caddy-builder (line ~200)
```
Old:
  # renovate: datasource=docker depName=golang
  FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS caddy-builder

New:
  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS caddy-builder
```

#### 4.3.4 crowdsec-builder (line ~297)
```
Old:
  # renovate: datasource=docker depName=golang versioning=docker
  FROM --platform=$BUILDPLATFORM golang:1.26.1-alpine AS crowdsec-builder

New:
  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS crowdsec-builder
```

### 4.4 Alpine FROM Line (crowdsec-fallback)

**Line ~357-358:**
```
Old:
  # renovate: datasource=docker depName=alpine versioning=docker
  FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS crowdsec-fallback

New:
  FROM ${ALPINE_IMAGE} AS crowdsec-fallback
```

### 4.5 CrowdSec Version ARGs (Remove Per-Stage Duplicates)

#### 4.5.1 crowdsec-builder stage (lines ~303-305)

```
Old:
  # CrowdSec version - Renovate can update this
  # renovate: datasource=github-releases depName=crowdsecurity/crowdsec
  ARG CROWDSEC_VERSION=1.7.6
  # CrowdSec fallback tarball checksum (v${CROWDSEC_VERSION})
  ARG CROWDSEC_RELEASE_SHA256=704e37121e7ac215991441cef0d8732e33fa3b1a2b2b88b53a0bfe5e38f863bd

New:
  ARG CROWDSEC_VERSION
  ARG CROWDSEC_RELEASE_SHA256
```

The bare `ARG` re-declarations (without defaults) inherit the top-level values. Remove the Renovate comments — tracking is on the top-level ARG.

#### 4.5.2 crowdsec-fallback stage (lines ~367-368)

```
Old:
  # CrowdSec version - Renovate can update this
  # renovate: datasource=github-releases depName=crowdsecurity/crowdsec
  ARG CROWDSEC_VERSION=1.7.6
  ARG CROWDSEC_RELEASE_SHA256=704e37121e7ac215991441cef0d8732e33fa3b1a2b2b88b53a0bfe5e38f863bd

New:
  ARG CROWDSEC_VERSION
  ARG CROWDSEC_RELEASE_SHA256
```

### 4.6 Go Dependency Patches

#### 4.6.1 Caddy builder — expr-lang (line ~254-255)

```
Old:
  # renovate: datasource=go depName=github.com/expr-lang/expr
  go get github.com/expr-lang/expr@v1.17.7; \

New:
  go get github.com/expr-lang/expr@v${EXPR_LANG_VERSION}; \
```

Requires adding `ARG EXPR_LANG_VERSION` re-declaration inside the `caddy-builder` stage (after the existing `ARG` block). Remove the per-line Renovate comment.

#### 4.6.2 Caddy builder — golang.org/x/net (line ~258-259)

```
Old:
  # renovate: datasource=go depName=golang.org/x/net
  go get golang.org/x/net@v0.51.0; \

New:
  go get golang.org/x/net@v${XNET_VERSION}; \
```

Requires adding `ARG XNET_VERSION` re-declaration inside the `caddy-builder` stage. Remove the per-line Renovate comment.

#### 4.6.3 CrowdSec builder — expr-lang + net (lines ~322-327)

```
Old:
  # renovate: datasource=go depName=github.com/expr-lang/expr
  # renovate: datasource=go depName=golang.org/x/crypto
  # renovate: datasource=go depName=golang.org/x/net
  RUN go get github.com/expr-lang/expr@v1.17.7 && \
      go get golang.org/x/crypto@v0.46.0 && \
      go get golang.org/x/net@v0.51.0 && \
      go mod tidy

New:
  # renovate: datasource=go depName=golang.org/x/crypto
  RUN go get github.com/expr-lang/expr@v${EXPR_LANG_VERSION} && \
      go get golang.org/x/crypto@v0.46.0 && \
      go get golang.org/x/net@v${XNET_VERSION} && \
      go mod tidy
```

The `golang.org/x/crypto` Renovate comment stays because that dependency only appears here (not duplicated). The `expr-lang` and `x/net` Renovate comments are removed — they're tracked on the top-level ARGs. Requires `ARG EXPR_LANG_VERSION` and `ARG XNET_VERSION` re-declarations in the `crowdsec-builder` stage.

### 4.7 ARG Re-declarations Per Stage

Docker's scoping rules require that top-level ARGs be re-declared (without value) inside each stage that uses them. The following `ARG` lines must be added to each stage:

| Stage | Required ARG Re-declarations |
|-------|------------------------------|
| `gosu-builder` | (none — `GO_VERSION` used in FROM, automatically available) |
| `backend-builder` | (none — same) |
| `caddy-builder` | `ARG EXPR_LANG_VERSION` and `ARG XNET_VERSION` (used in RUN) |
| `crowdsec-builder` | `ARG CROWDSEC_VERSION` ¹, `ARG CROWDSEC_RELEASE_SHA256` ¹, `ARG EXPR_LANG_VERSION`, `ARG XNET_VERSION` |
| `crowdsec-fallback` | `ARG CROWDSEC_VERSION` ¹, `ARG CROWDSEC_RELEASE_SHA256` ¹ |

¹ Already re-declared, just need default values removed and Renovate comments removed.

**Important Docker ARG scoping note:**
- ARGs used in `FROM` interpolation (`GO_VERSION`, `ALPINE_IMAGE`) are evaluated at the global scope — they do NOT need re-declaration inside the stage.
- ARGs used in `RUN`, `ENV`, or other instructions inside a stage MUST be re-declared with a bare `ARG NAME` (no default) inside that stage to inherit the top-level value.

---

## 5. Renovate Annotation Strategy

### 5.1 Annotations to Keep (Top-Level Only)

| Location | Annotation | Tracks |
|----------|------------|--------|
| Top-level `ARG GO_VERSION` | `# renovate: datasource=docker depName=golang versioning=docker` | Go Docker image tag |
| Top-level `ARG ALPINE_IMAGE` | `# renovate: datasource=docker depName=alpine versioning=docker` | Alpine Docker image + digest |
| Top-level `ARG CROWDSEC_VERSION` | `# renovate: datasource=github-releases depName=crowdsecurity/crowdsec` | CrowdSec GitHub releases |
| Top-level `ARG EXPR_LANG_VERSION` | `# renovate: datasource=go depName=github.com/expr-lang/expr` | expr-lang Go module |
| Top-level `ARG XNET_VERSION` | `# renovate: datasource=go depName=golang.org/x/net` | x/net Go module |

### 5.2 Annotations to Remove

| Location | Reason |
|----------|--------|
| Line ~46 `# renovate: datasource=docker depName=golang` (gosu-builder) | Tracked by top-level `GO_VERSION` |
| Line ~97 `# renovate: datasource=docker depName=golang` (backend-builder) | Same |
| Line ~199 `# renovate: datasource=docker depName=golang` (caddy-builder) | Same |
| Line ~296 `# renovate: datasource=docker depName=golang versioning=docker` (crowdsec-builder) | Same |
| Line ~29 `# renovate: datasource=docker depName=alpine versioning=docker` (CADDY_IMAGE) | Replaced by `ALPINE_IMAGE` |
| Line ~357 `# renovate: datasource=docker depName=alpine versioning=docker` (crowdsec-fallback FROM) | Tracked by top-level `ALPINE_IMAGE` |
| Lines ~302-303 `# renovate:` annotations on crowdsec-builder CROWDSEC_VERSION | Tracked by top-level |
| Lines ~366-367 `# renovate:` annotations on crowdsec-fallback CROWDSEC_VERSION | Tracked by top-level |
| Line ~254 `# renovate: datasource=go depName=github.com/expr-lang/expr` (caddy-builder) | Tracked by top-level `EXPR_LANG_VERSION` |
| Line ~258 `# renovate: datasource=go depName=golang.org/x/net` (caddy-builder) | Tracked by top-level `XNET_VERSION` |
| Lines ~322-324 `# renovate:` for expr-lang and x/net (crowdsec-builder) | Tracked by top-level |

### 5.3 Renovate Configuration Update

The existing regex custom manager in `.github/renovate.json` for Go dependency patches currently matches the pattern:

```
#\s*renovate:\s*datasource=go\s+depName=(?<depName>[^\s]+)\s*\n\s*go get (?<depName2>[^@]+)@v(?<currentValue>[^\s|]+)
```

After this change, the `go get` lines will use `${EXPR_LANG_VERSION}` and `${XNET_VERSION}` instead of hardcoded versions. The regex manager will **no longer match** the `go get` lines for these two deps.

**Required renovate.json changes:**

Add new custom manager entries for the new top-level ARGs:

```json
{
  "customType": "regex",
  "description": "Track expr-lang version ARG in Dockerfile",
  "managerFilePatterns": ["/^Dockerfile$/"],
  "matchStrings": [
    "ARG EXPR_LANG_VERSION=(?<currentValue>[^\\s]+)"
  ],
  "depNameTemplate": "github.com/expr-lang/expr",
  "datasourceTemplate": "go",
  "versioningTemplate": "semver"
},
{
  "customType": "regex",
  "description": "Track golang.org/x/net version ARG in Dockerfile",
  "managerFilePatterns": ["/^Dockerfile$/"],
  "matchStrings": [
    "ARG XNET_VERSION=(?<currentValue>[^\\s]+)"
  ],
  "depNameTemplate": "golang.org/x/net",
  "datasourceTemplate": "go",
  "versioningTemplate": "semver"
}
```

**Also add for `GO_VERSION`** — Renovate's built-in Docker manager matches `FROM golang:X.X.X-alpine`, but after this change the FROM uses `${GO_VERSION}` interpolation. Renovate's Docker manager cannot parse variable-interpolated FROM lines. A custom regex manager is needed:

```json
{
  "customType": "regex",
  "description": "Track Go toolchain version ARG in Dockerfile",
  "managerFilePatterns": ["/^Dockerfile$/"],
  "matchStrings": [
    "#\\s*renovate:\\s*datasource=docker\\s+depName=golang.*\\nARG GO_VERSION=(?<currentValue>[^\\s]+)"
  ],
  "depNameTemplate": "golang",
  "datasourceTemplate": "docker",
  "versioningTemplate": "docker"
}
```

The existing Alpine regex manager already matches `ARG CADDY_IMAGE=...`. Update the regex to match the new ARG name `ALPINE_IMAGE`:

```
Old matchString:
  "ARG CADDY_IMAGE=alpine:(?<currentValue>[^\\s@]+@sha256:[a-f0-9]+)"

New matchString:
  "ARG ALPINE_IMAGE=alpine:(?<currentValue>[^\\s@]+@sha256:[a-f0-9]+)"
```

**CrowdSec version:** Already tracked by the existing regex manager pattern that matches `ARG CROWDSEC_VERSION=...`. Since we keep that ARG name unchanged, no renovate.json change is needed for CrowdSec — but verify only one match occurs (the top-level one) after removing the per-stage defaults.

---

## 6. Implementation Plan

### Phase 1: Playwright Tests (N/A)

This change is a build-system refactor with no UI/UX changes. No Playwright tests are needed. The Docker build itself is the validation artifact.

### Phase 2: Dockerfile Changes

| Step | Action | Files |
|------|--------|-------|
| 2.1 | Add new top-level ARG block | `Dockerfile` |
| 2.2 | Remove `CADDY_IMAGE` ARG, replace references with `ALPINE_IMAGE` | `Dockerfile` |
| 2.3 | Replace 4 `FROM golang:1.26.1-alpine` lines with `FROM golang:${GO_VERSION}-alpine`; remove per-line Renovate comments | `Dockerfile` |
| 2.4 | Replace `FROM alpine:3.23.3@sha256:...` in crowdsec-fallback with `FROM ${ALPINE_IMAGE}`; remove Renovate comment | `Dockerfile` |
| 2.5 | Convert per-stage `CROWDSEC_VERSION`/`SHA` to bare `ARG` re-declarations | `Dockerfile` |
| 2.6 | Add `ARG EXPR_LANG_VERSION` and `ARG XNET_VERSION` re-declarations in `caddy-builder` and `crowdsec-builder` stages | `Dockerfile` |
| 2.7 | Replace hardcoded `@v1.17.7` and `@v0.51.0` in `go get` with `@v${EXPR_LANG_VERSION}` and `@v${XNET_VERSION}` | `Dockerfile` |

### Phase 3: Renovate Configuration

| Step | Action | Files |
|------|--------|-------|
| 3.1 | Add `GO_VERSION` regex custom manager | `.github/renovate.json` |
| 3.2 | Add `EXPR_LANG_VERSION` regex custom manager | `.github/renovate.json` |
| 3.3 | Add `XNET_VERSION` regex custom manager | `.github/renovate.json` |
| 3.4 | Update Alpine regex manager to use `ALPINE_IMAGE` | `.github/renovate.json` |
| 3.5 | Verify the existing `go get` regex manager no longer double-matches consolidated deps | `.github/renovate.json` |

### Phase 4: Validation

| Step | Verification |
|------|--------------|
| 4.1 | `docker build --platform=linux/amd64 -t charon:test .` succeeds |
| 4.2 | `docker build --platform=linux/arm64 -t charon:test-arm .` succeeds (if cross-build infra available) |
| 4.3 | `grep -c 'golang:1.26' Dockerfile` returns `0` (no hardcoded Go versions remain in FROM lines) |
| 4.4 | `grep -c 'alpine:3.23' Dockerfile` returns `1` (only the top-level `ALPINE_IMAGE` ARG) |
| 4.5 | `grep -c 'CROWDSEC_VERSION=1.7' Dockerfile` returns `1` (only the top-level ARG) |
| 4.6 | `grep -c 'expr-lang/expr@v' Dockerfile` returns `0` (no hardcoded expr-lang versions in go get) |
| 4.7 | `grep -c 'x/net@v' Dockerfile` returns `0` (no hardcoded x/net versions in go get) |
| 4.8 | Renovate dry-run validates all new regex managers match exactly once |

### Phase 5: Documentation

Update `CHANGELOG.md` with a `chore: consolidate Dockerfile version ARGs` entry.

---

## 7. Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Docker ARG scoping**: ARG used in `FROM` requires global-scope declaration; ARG in `RUN` requires per-stage re-declaration | High | Explicitly tested in §4.1. Docker's scoping rules are well-documented. |
| **Renovate stops tracking**: Switching from inline `FROM` annotation to ARG-based regex manager may cause Renovate to lose tracking until config is deployed | Medium | Add regex managers in the same PR. Test with `renovate --dry-run` or Dependency Dashboard. |
| **Renovate double-matching**: Old `go get` regex manager might still match non-parameterized `go get` lines (crypto, ipstore) | Low | These deps keep their inline `# renovate:` comments and hardcoded versions. The regex correctly matches them. |
| **CADDY_IMAGE rename**: Any docker-compose override or CI script referencing `--build-arg CADDY_IMAGE=...` will break | Medium | Search codebase for `CADDY_IMAGE` references outside `Dockerfile`. Update CI workflows if found. |
| **Build cache invalidation**: Changing ARG values at the top level invalidates all downstream layer caches | Low | This is expected and already happens today when any version changes. No behavioral change. |
| **Cross-compilation**: `${GO_VERSION}` interpolation in multi-platform builds | Low | Docker resolves global ARGs before platform selection. Verified by BuildKit semantics. |

---

## 8. Commit Slicing Strategy

**Decision:** Single PR.

**Rationale:**
- All changes are confined to `Dockerfile` and `.github/renovate.json`
- No cross-domain changes (no backend/frontend code)
- The changes are logically atomic — partial consolidation would leave a confusing state
- Total diff is small (~30 lines changed, ~15 lines added, ~20 lines removed)
- Rollback is trivial: revert the single commit

**PR Slice:**

| Slice | Scope | Files | Validation Gate |
|-------|-------|-------|----------------|
| PR-1 (only) | Full consolidation | `Dockerfile`, `.github/renovate.json` | Docker build succeeds on amd64; `grep` checks pass; Renovate dry-run confirms tracking |

**Rollback:** `git revert <sha>` — single commit, no dependencies.

---

## 9. Acceptance Criteria

- [ ] `Dockerfile` has exactly one declaration of Go version (`ARG GO_VERSION=...`)
- [ ] `Dockerfile` has exactly one declaration of Alpine image+digest (`ARG ALPINE_IMAGE=...`)
- [ ] `Dockerfile` has exactly one declaration of CrowdSec version and SHA256 (top-level)
- [ ] `Dockerfile` has exactly one declaration of `expr-lang/expr` version (top-level ARG)
- [ ] `Dockerfile` has exactly one declaration of `golang.org/x/net` version (top-level ARG)
- [ ] All 4 Go `FROM` lines use `${GO_VERSION}` interpolation
- [ ] `crowdsec-fallback` FROM uses `${ALPINE_IMAGE}` interpolation
- [ ] Per-stage `CROWDSEC_VERSION` re-declarations are bare `ARG` (no default, no Renovate comment)
- [ ] `go get` lines use `${EXPR_LANG_VERSION}` and `${XNET_VERSION}` interpolation
- [ ] Renovate `# renovate:` annotations exist only on top-level ARGs (one per tracked dep)
- [ ] `.github/renovate.json` has regex managers for `GO_VERSION`, `EXPR_LANG_VERSION`, `XNET_VERSION`, and updated `ALPINE_IMAGE`
- [ ] `docker build` succeeds without errors
- [ ] No duplicate version values remain (verified by grep checks in §4)
