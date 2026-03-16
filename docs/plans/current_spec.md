# Fix Plan: 6 HIGH CVEs in node:24.14.0-alpine frontend-builder Stage

**Status:** Active
**Created:** 2026-03-16
**Branch:** `fix/node-alpine-cve-remediation`
**Scope:** `Dockerfile` — `frontend-builder` stage only
**Previous Plan:** Backed up to `docs/plans/current_spec.md.bak`

---

## 1. Introduction

The `frontend-builder` stage in the multi-stage `Dockerfile` is pinned to:

```dockerfile
# renovate: datasource=docker depName=node
FROM --platform=$BUILDPLATFORM node:24.14.0-alpine@sha256:7fddd9ddeae8196abf4a3ef2de34e11f7b1a722119f91f28ddf1e99dcafdf114 AS frontend-builder
```

Docker Scout (via Docker Hub) and Grype/Trivy scans report **6 HIGH-severity CVEs** in this image. Although the `frontend-builder` stage is build-time only and does not appear in the final runtime image, these CVEs are still relevant for **supply chain security**: CI scans, SBOM attestations, and SLSA provenance all inspect intermediate build stages. Failing to address them causes CI gates to fail and weakens the supply chain posture.

---

## 2. Research Findings

### 2.1 Current Image

| Field | Value |
|---|---|
| Tag | `node:24.14.0-alpine` |
| Multi-arch index digest (used in FROM) | `sha256:7fddd9ddeae8196abf4a3ef2de34e11f7b1a722119f91f28ddf1e99dcafdf114` |
| amd64 platform-specific manifest digest | `sha256:e9445c64ace1a9b5cdc60fc98dd82d1e5142985d902f41c2407e8fffe49d46a3` |
| arm64/v8 platform-specific manifest digest | `sha256:0e0d39e04fdf3dc5f450a07922573bac666d28920df2df3f3b1540b0aba7ab98` |
| Base Alpine version | Alpine 3.23 |
| Compressed size (amd64) | 53.63 MB |
| Last pushed on Docker Hub | 2026-02-26 (19 days before research date) |

### 2.2 Docker Hub Floating Tag Alignment

`docker manifest inspect node:24-alpine` confirmed on 2026-03-16:

- amd64: `sha256:e9445c64ace1a9b5cdc60fc98dd82d1e5142985d902f41c2407e8fffe49d46a3`
- arm64/v8: `sha256:0e0d39e04fdf3dc5f450a07922573bac666d28920df2df3f3b1540b0aba7ab98`
- s390x: `sha256:965b4135b1067dca4b1aff58675c9b9a1f028d57e30c2e0d39bcd9863605ad62`

The Docker Hub layers page for the amd64 manifest confirms **INDEX DIGEST: `sha256:7fddd9ddeae8196abf4a3ef2de34e11f7b1a722119f91f28ddf1e99dcafdf114`** — exactly matching the digest pinned in the Dockerfile.

**`node:24-alpine`, `node:24-alpine3.23`, and `node:24.14.0-alpine` all resolve to the identical multi-arch index digest.** There is no newer `node:24.x.y-alpine` image on Docker Hub as of 2026-03-16.

### 2.3 CVE Summary

Docker Scout scan of `node:24-alpine` amd64 manifest `sha256:e9445c64ace1...`:

| CVE ID | CVSS | Severity | Package manager | Package | Version |
|---|---|---|---|---|---|
| CVE-2026-26996 | 8.7 | **HIGH** | npm | minimatch | 10.1.2 |
| CVE-2026-29786 | 8.2 | **HIGH** | npm | tar | 7.5.7 |
| CVE-2026-31802 | 8.2 | **HIGH** | npm | tar | 7.5.7 |
| CVE-2026-27904 | 7.5 | **HIGH** | npm | minimatch | 10.1.2 |
| CVE-2026-27903 | 7.5 | **HIGH** | npm | minimatch | 10.1.2 |
| CVE-2026-26960 | 7.1 | **HIGH** | npm | tar | 7.5.7 |
| CVE-2025-60876 | 6.5 | MEDIUM | apk | alpine/busybox | 1.37.0-r30 |
| CVE-2026-22184 | 4.6 | MEDIUM | apk | alpine/zlib | 1.3.1-r2 |
| CVE-2026-27171 | 2.9 | LOW | apk | alpine/zlib | 1.3.1-r2 |

**Total: 0 Critical, 6 High, 2 Medium, 1 Low**
**Docker Scout fixability as of 2026-03-16: 0 Fixable** (no patched versions yet available in Alpine apk repositories or npm registry)

### 2.4 CVE Location Analysis

All **6 HIGH** CVEs are in **npm's own internally-bundled packages**, not in the frontend project's `node_modules`. These packages live inside the image at:

```
/usr/local/lib/node_modules/npm/node_modules/minimatch/   ← CVE-2026-26996, CVE-2026-27904, CVE-2026-27903
/usr/local/lib/node_modules/npm/node_modules/tar/         ← CVE-2026-29786, CVE-2026-31802, CVE-2026-26960
```

`minimatch` is used by the npm CLI for glob pattern matching. `tar` is used by npm for `.tgz` tarball extraction during `npm install`/`npm ci`. These are NOT declared in `frontend/package.json`; they are shipped inside the npm CLI binary itself.

The **2 MEDIUM + 1 LOW** CVEs are in Alpine OS packages managed by apk:
- `busybox@1.37.0-r30`: CVE-2025-60876
- `zlib@1.3.1-r2`: CVE-2026-22184, CVE-2026-27171

### 2.5 `apk upgrade` Effectiveness

`apk upgrade --no-cache` operates exclusively on Alpine apk-managed packages. It has no effect on files under `/usr/local/lib/node_modules/`.

| CVE set | Fixed by `apk upgrade`? |
|---|---|
| 6 HIGH (npm/minimatch, npm/tar) | **No** — these are npm-managed, not apk-managed |
| 2 MEDIUM + 1 LOW (apk/busybox, apk/zlib) | **Yes, once Alpine maintainers publish patches** — currently 0 fixable per Docker Scout, but the `apk upgrade` step will apply patches automatically when they land |

### 2.6 Renovate Automation

The Dockerfile already carries the correct Renovate comment on the line immediately before the FROM:

```dockerfile
# renovate: datasource=docker depName=node
FROM --platform=$BUILDPLATFORM node:24.14.0-alpine@sha256:7fddd9... AS frontend-builder
```

When the Node.js project publishes `node:24.15.0-alpine` (or later) to Docker Hub, Renovate will automatically propose a PR updating the version tag (`24.14.0` → next) and the `@sha256:` digest to the new multi-arch index. That Renovate PR is the **definitive fix path** because the new release will ship npm bundling patched `minimatch` and `tar`.

### 2.7 Risk Assessment

| Risk factor | Assessment |
|---|---|
| Appears in final runtime image | **No** — only the compiled `dist/` output is `COPY`-ed to the final stage |
| Exploitable at runtime | **No** — `npm`, `minimatch`, and `tar` are not present in the final image |
| Exploitable during build | Theoretical (supply chain attack on the build worker) |
| CI scan failures | **Yes** — Grype/Trivy flag build stages; this is the main driver for the fix |
| SBOM/SLSA impact | **Yes** — SBOM includes build-stage packages; HIGH CVEs degrade attestation quality |

---

## 3. Technical Specification

### 3.1 FROM Line — No Change (No Newer Image Available)

Since `node:24-alpine` and `node:24.14.0-alpine` resolve to the **same** multi-arch index digest (`sha256:7fddd9...`), there is no newer pinned image to upgrade to. **The FROM line does not change.** Renovate handles future image bumps autonomously.

### 3.2 Changes to `frontend-builder` Stage

**Single file changed:** `Dockerfile`

**Locations:** Two changes in `Dockerfile`.

**Change A — Top-level ARG (Pinned Toolchain Versions block):**

Add after the existing `ARG XNET_VERSION` line in the `# ---- Pinned Toolchain Versions ----` section:

```diff
 # renovate: datasource=go depName=golang.org/x/net
 ARG XNET_VERSION=0.51.0
+
+# renovate: datasource=npm depName=npm
+ARG NPM_VERSION=11.11.1
```

**Change B — `frontend-builder` stage (before `RUN npm ci`):**

```diff
 # Vite 8: Rolldown native bindings auto-resolved per platform via optionalDependencies

+# Upgrade npm to replace its bundled minimatch/tar with patched versions
+# Addresses: CVE-2026-26996, CVE-2026-27903, CVE-2026-27904 (npm/minimatch)
+#            CVE-2026-26960, CVE-2026-29786, CVE-2026-31802 (npm/tar)
+# Run apk upgrade for Alpine package CVEs (busybox, zlib) once patches land
+# hadolint ignore=DL3017
+RUN apk upgrade --no-cache && \
+    npm install -g npm@${NPM_VERSION} --no-fund --no-audit && \
+    npm cache clean --force
+
 RUN npm ci
```

### 3.3 Step-by-Step Rationale

| Added command | Rationale |
|---|---|
| `apk upgrade --no-cache` | Applies any Alpine repo patches for busybox (CVE-2025-60876) and zlib (CVE-2026-22184, CVE-2026-27171) without changing the base image pin. Currently 0 fixable per Docker Scout, but will take effect automatically once Alpine maintainers ship packages. |
| `npm install -g npm@${NPM_VERSION} --no-fund --no-audit` | Replaces `/usr/local/lib/node_modules/npm/` (and its bundled `minimatch` + `tar`) with the pinned npm release from the npm registry. `NPM_VERSION` is declared as `11.11.1` in the top-level Pinned Toolchain Versions ARG block and tracked by Renovate's npm datasource manager. `--no-fund` and `--no-audit` suppress log noise during build. If a patched npm has been published since the node image was created, this eliminates the 6 HIGH CVEs. |
| `npm cache clean --force` | Clears npm's cache after the global upgrade to prevent stale entries interfering with the subsequent `npm ci`. |

### 3.4 Caveats

**"0 Fixable" status:** Docker Scout reports zero fixable CVEs across all 9 at research time (2026-03-16), meaning patched npm packages are not yet in the registry. The `npm install -g npm@${NPM_VERSION}` step is **defensive** — it will self-apply patches as soon as the npm team publishes a release bundling fixed dependencies. When that release is published, Renovate will propose a bump to `NPM_VERSION` which is all that is needed.

**Definitive fix:** A new `node:24.x.y-alpine` image from the Node.js release team (bundling a fixed npm version) is the complete resolution. Renovate auto-detects and proposes this update.

**`npm ci` behavior:** `npm ci` installs project dependencies from `frontend/package-lock.json` and is unaffected by upgrading the global npm executable. The frontend project's own `node_modules` are separate from npm's internal bundled packages.

**npm pinning:** `npm@latest` has been replaced with a top-level `ARG NPM_VERSION=11.11.1` tracked by a Renovate npm datasource comment. The ARG is declared in the Pinned Toolchain Versions block alongside `GO_VERSION`, `XNET_VERSION`, etc. Renovate auto-proposes version bumps when a newer npm release is published. The implemented pattern:

```dockerfile
# renovate: datasource=npm depName=npm
ARG NPM_VERSION=11.11.1
# hadolint ignore=DL3017
RUN apk upgrade --no-cache && \
    npm install -g npm@${NPM_VERSION} --no-fund --no-audit && \
    npm cache clean --force
```

---

## 4. Implementation Plan

### Phase 1: Playwright Tests

No new Playwright tests are required. The change is entirely in the Docker build process, not in application behavior. The E2E suite exercises the running application and does not validate build-stage CVEs.

### Phase 2: Dockerfile Change

1. Open `Dockerfile`.
2. In the `# ---- Pinned Toolchain Versions ----` section (approximately line 27), locate `ARG XNET_VERSION` and insert the `NPM_VERSION` ARG immediately after it, as specified in §3.2 Change A.
3. Locate the `# ---- Frontend Builder ----` comment block (approximately line 88).
4. Find the line `# Vite 8: Rolldown native bindings auto-resolved per platform via optionalDependencies`.
5. After that line, insert the new RUN block exactly as specified in §3.2 Change B.
6. Leave all other lines in the `frontend-builder` stage unchanged.

### Phase 3: Build Verification

```bash
# Build frontend-builder stage only (fast, ~2 min)
docker build --target frontend-builder -t charon-frontend-builder-test .

# Confirm npm was upgraded (version should be newer than shipped with node:24.14.0-alpine)
docker run --rm charon-frontend-builder-test npm --version

# Grype scan of the built stage
grype charon-frontend-builder-test --fail-on high

# Trivy scan
trivy image --severity HIGH,CRITICAL --exit-code 1 charon-frontend-builder-test
```

If patched npm packages are in the registry, Grype and Trivy will report 0 HIGH CVEs for npm packages. If patches are not yet published, both scanners will still report the 6 HIGH CVEs (the `npm@${NPM_VERSION}` step installs `11.11.1`; once the npm team ships a patched release, Renovate bumps `NPM_VERSION` to pick it up).

### Phase 4: Full Image Build

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t charon:test .
```

Confirm the final runtime image does not inherit the build-stage CVEs:

```bash
docker scout cves charon:test
```

### Phase 5: Monitor Renovate

No action required. Renovate monitors `node` on Docker Hub via the existing `# renovate: datasource=docker depName=node` comment. When `node:24.15.0-alpine` lands, Renovate opens a PR.

---

## 5. Commit Slicing Strategy

**Decision: Single PR.**

The entire change is one file (`Dockerfile`), one stage, three lines added. There are no application code changes, no schema changes, no test changes. A single commit and single PR is appropriate.

### PR-1 — `fix: upgrade npm and apk in frontend-builder to mitigate CVEs`

| Field | Value |
|---|---|
| Branch | `fix/node-alpine-cve-remediation` |
| Files changed | `Dockerfile` (1 file, ~4 lines added) |
| Dependencies | None |
| Rollback | `git revert HEAD` on the merge commit |

**Suggested commit message:**

```
fix: upgrade npm and apk in frontend-builder to mitigate node CVEs

The node:24.14.0-alpine image used in the frontend-builder stage
carries 6 HIGH-severity CVEs in npm's internally-bundled packages:

  minimatch@10.1.2: CVE-2026-26996 (8.7), CVE-2026-27904 (7.5),
                    CVE-2026-27903 (7.5)
  tar@7.5.7:        CVE-2026-29786 (8.2), CVE-2026-31802 (8.2),
                    CVE-2026-26960 (7.1)

Plus 2 medium and 1 low Alpine CVEs in busybox and zlib.

No newer node:24.x-alpine image exists on Docker Hub as of 2026-03-16.
node:24-alpine resolves to the same multi-arch index digest as the
pinned 24.14.0-alpine tag. Renovate will auto-update the FROM line
when node:24.15.0-alpine is published.

Add a pre-npm-ci RUN step in frontend-builder to:
- Run `apk upgrade --no-cache` to pick up Alpine package patches for
  busybox/zlib as soon as they land in the Alpine repos
- Run `npm install -g npm@${NPM_VERSION}` (pinned to `11.11.1`,
  Renovate-tracked via npm datasource) to replace npm's bundled
  minimatch and tar with patched versions once npm publishes a fix;
  Renovate auto-proposes NPM_VERSION bumps when newer releases land

The frontend-builder stage does not appear in the final runtime image
so runtime risk is zero; this change targets supply chain security.
```

**Validation gate:** Docker build exits 0; Grype/Trivy scans of the `frontend-builder` target report 0 HIGH CVEs for npm packages (contingent on npm publishing patched releases).

---

## 6. Acceptance Criteria

| # | Criterion | How to verify |
|---|---|---|
| 1 | Docker build succeeds for `linux/amd64` and `linux/arm64` | `docker buildx build --platform linux/amd64,linux/arm64 --target frontend-builder .` exits 0 |
| 2 | No new CVEs introduced | Grype scan of the new build shows no CVEs not already present in the baseline |
| 3 | `apk upgrade` runs without error | Build log shows apk output without error exit |
| 4 | npm version is upgraded | `docker run --rm charon-frontend-builder-test npm --version` shows a version newer than what shipped with node:24.14.0-alpine |
| 5 | `npm ci` still succeeds | Build log shows successful `npm ci` after the upgrade step |
| 6 | Final runtime image is unaffected | `docker scout cves charon:latest` shows no increase in CVE count vs pre-change baseline |
| 7 | Renovate comment preserved | `# renovate: datasource=docker depName=node` remains on the line immediately before the `FROM` |
| 8 | Diagnostic shows 0 HIGH npm CVEs | Grype/Trivy scan of `frontend-builder` target exits 0 with `--fail-on high` once npm publishes patched minimatch/tar |

---

## 7. Open Questions / Future Work

1. **When will `node:24.15.0-alpine` be released?** Node.js 24.x follows a roughly bi-weekly release cadence. Monitor https://github.com/nodejs/node/releases. Renovate handles the FROM update automatically once the image is on Docker Hub.

2. ~~**Pin npm version?**~~ Resolved. `npm@latest` has been replaced with a pinned `ARG NPM_VERSION=11.11.1` in the Pinned Toolchain Versions block, tracked by Renovate's npm datasource manager. No follow-up PR is required.

3. **Should `node:24-alpine3.22` be evaluated?** Switching Alpine base versions to 3.22 would produce a different CVE profile but is inconsistent with the final runtime stage already using `alpine:3.23.3`. Not recommended.
