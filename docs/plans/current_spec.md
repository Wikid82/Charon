# CI Supply Chain CVE Remediation Plan

**Status:** Active
**Created:** 2026-03-13
**Branch:** `feature/beta-release`
**Context:** Three HIGH vulnerabilities (CVE-2025-69650, CVE-2025-69649, CVE-2026-3805) in the Docker runtime image are blocking the CI supply-chain scan. Two Grype ignore-rule entries are also expired and require maintenance.

---

## 1. Executive Summary

| # | Action | Severity Reduction | Effort |
|---|--------|--------------------|--------|
| 1 | Remove `curl` from runtime image (replace with `wget`) | Eliminates 1 HIGH + ~7 MEDIUMs + 2 LOWs | ~30 min |
| 2 | Remove `binutils` + `libc-utils` from runtime image | Eliminates 2 HIGH + 3 MEDIUMs | ~5 min |
| 3 | Update expired Grype ignore rules | Prevents false scan failures at next run | ~10 min |

**Bottom line:** All three HIGH CVEs are eliminated at root rather than suppressed. After Phase 1 and Phase 2, `fail-on-severity: high` passes cleanly. Phase 3 is maintenance-only.

---

## 2. CVE Inventory

### Blocking HIGH CVEs

| CVE | Package | Version | CVSS | Fix State | Notes |
|-----|---------|---------|------|-----------|-------|
| CVE-2026-3805 | `curl` | 8.17.0-r1 | 7.5 | `unknown` | **New** — appeared in Grype DB 2026-03-13, published 2026-03-11. SMB protocol use-after-free. Charon uses HTTPS/HTTP only. |
| CVE-2025-69650 | `binutils` | 2.45.1-r0 | 7.5 | `` (none) | Double-free in `readelf`. Charon never invokes `readelf`. |
| CVE-2025-69649 | `binutils` | 2.45.1-r0 | 7.5 | `` (none) | Null-ptr deref in `readelf`. Charon never invokes `readelf`. |

### Associated MEDIUM/LOW CVEs eliminated as side-effects

| CVEs | Package | Count | Eliminated by |
|------|---------|-------|---------------|
| CVE-2025-14819, CVE-2025-15079, CVE-2025-14524, CVE-2025-13034, CVE-2025-14017 | `curl` | 5 × MEDIUM | Phase 1 |
| CVE-2025-69652, CVE-2025-69644, CVE-2025-69651 | `binutils` | 3 × MEDIUM | Phase 2 |

### Expired Grype Ignore Rules

| Entry | Expiry | Status | Action |
|-------|--------|--------|--------|
| `CVE-2026-22184` (zlib) | 2026-03-14 | Expires tomorrow; underlying CVE already fixed via `apk upgrade --no-cache zlib` | **Remove entirely** |
| `GHSA-69x3-g4r3-p962` (nebula) | 2026-03-05 | **Expired 8 days ago**; upstream fix still unavailable | **Extend to 2026-04-13** |

---

## 3. Phase 1 — Remove `curl` from Runtime Image

### Rationale

`curl` is present solely for:
1. GeoLite2 DB download at build time (Dockerfile, runtime stage `RUN` block)
2. HEALTHCHECK probe (Dockerfile `HEALTHCHECK` directive)
3. Caddy admin API readiness poll (`.docker/docker-entrypoint.sh`)

`busybox` (already installed on Alpine as a transitive dependency of `busybox-extras`, which is explicitly installed) provides `wget` with sufficient functionality for all three uses.

### 3.1 `wget` Translation Reference

| `curl` invocation | `wget` equivalent | Notes |
|-------------------|--------------------|-------|
| `curl -fSL -m 10 "URL" -o FILE 2>/dev/null` | `wget -qO FILE -T 10 "URL" 2>/dev/null` | `-q` = quiet; `-T` = timeout (seconds); exits nonzero on failure |
| `curl -fSL -m 30 --retry 3 "URL" -o FILE` | `wget -qO FILE -T 30 -t 4 "URL"` | `-t 4` = 4 total tries (1 initial + 3 retries); add `&& [ -s FILE ]` guard |
| `curl -f http://HOST/path \|\| exit 1` | `wget -q -O /dev/null http://HOST/path \|\| exit 1` | HEALTHCHECK; wget exits nonzero on HTTP error |
| `curl -sf http://HOST/path > /dev/null 2>&1` | `wget -qO /dev/null http://HOST/path 2>/dev/null` | Silent readiness probe |

**busybox wget notes:**
- `-T N` is per-connection timeout in seconds (equivalent to `curl --max-time`).
- `-t N` is total number of tries, not retries; `-t 4` = 3 retries.
- On download failure, busybox wget may leave a zero-byte or partial file at the output path. The `[ -s FILE ]` guard (`-s` = non-empty) prevents a corrupted placeholder from passing the sha256 check.

### 3.2 Dockerfile Changes

**File:** `Dockerfile`

**Change A — Remove `curl`, `binutils`, `libc-utils` from `apk add` (runtime stage, line ~413):**

Current:
```dockerfile
RUN apk add --no-cache \
    bash ca-certificates sqlite-libs sqlite tzdata curl gettext libcap libcap-utils \
    c-ares binutils libc-utils busybox-extras \
    && apk upgrade --no-cache zlib
```

New:
```dockerfile
RUN apk add --no-cache \
    bash ca-certificates sqlite-libs sqlite tzdata gettext libcap libcap-utils \
    c-ares busybox-extras \
    && apk upgrade --no-cache zlib
```

*(This single edit covers both Phase 1 and Phase 2 removals.)*

**Change B — GeoLite2 download block, CI path (line ~437):**

Current:
```dockerfile
if curl -fSL -m 10 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
    -o /app/data/geoip/GeoLite2-Country.mmdb 2>/dev/null; then
```

New:
```dockerfile
if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
    -T 10 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" 2>/dev/null; then
```

**Change C — GeoLite2 download block, non-CI path (line ~445):**

Current:
```dockerfile
if curl -fSL -m 30 --retry 3 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
    -o /app/data/geoip/GeoLite2-Country.mmdb; then
    if echo "${GEOLITE2_COUNTRY_SHA256}  /app/data/geoip/GeoLite2-Country.mmdb" | sha256sum -c -; then
```

New:
```dockerfile
if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
    -T 30 -t 4 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"; then
    if [ -s /app/data/geoip/GeoLite2-Country.mmdb ] && \
       echo "${GEOLITE2_COUNTRY_SHA256}  /app/data/geoip/GeoLite2-Country.mmdb" | sha256sum -c -; then
```

The `[ -s FILE ]` check is added before `sha256sum` to guard against wget leaving an empty file on partial failure.

**Change D — HEALTHCHECK directive (line ~581):**

Current:
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1
```

New:
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
    CMD wget -q -O /dev/null http://localhost:8080/api/v1/health || exit 1
```

### 3.3 Entrypoint Changes

**File:** `.docker/docker-entrypoint.sh`

**Change E — Caddy readiness poll (line ~368):**

Current:
```sh
if curl -sf http://127.0.0.1:2019/config/ > /dev/null 2>&1; then
```

New:
```sh
if wget -qO /dev/null http://127.0.0.1:2019/config/ 2>/dev/null; then
```

---

## 4. Phase 2 — Remove `binutils` and `libc-utils` from Runtime Image

### Rationale

`binutils` is installed solely for `objdump`, used in `.docker/docker-entrypoint.sh` to detect DWARF debug symbols when `CHARON_DEBUG=1`. The entrypoint already has a graceful fallback (lines ~401–404):

```sh
else
    # objdump not available, try to run Delve anyway with a warning
    echo "Note: Cannot verify debug symbols (objdump not found). Attempting Delve..."
    run_as_charon /usr/local/bin/dlv exec "$bin_path" ...
fi
```

When `objdump` is absent the container functions correctly for all standard and debug-mode runs. The check is advisory.

`libc-utils` appears **only once** across the entire codebase (confirmed by grep across `*.sh`, `Dockerfile`, `*.yml`): as a sibling entry on the same `apk add` line as `binutils`. It provides glibc-compatible headers for musl-based Alpine and has no independent consumer in this image. It is safe to remove together with `binutils`.

### 4.1 Dockerfile Change

Already incorporated in Phase 1 Change A — the `apk add` line removes both `binutils` and `libc-utils` in a single edit. No additional changes are required.

### 4.2 Why Not Suppress Instead?

Suppressing in Grype requires two new ignore entries with expiry maintenance every 30 days indefinitely (no upstream Alpine fix exists). Removing the packages eliminates the CVEs permanently. There is no functional regression given the working fallback.

---

## 5. Phase 3 — Update Expired Grype Ignore Rules

**File:** `.grype.yaml`

### 5.1 Remove `CVE-2026-22184` (zlib) Block

**Action:** Delete the entire `CVE-2026-22184` ignore entry.

**Reason:** The Dockerfile runtime stage already contains `&& apk upgrade --no-cache zlib`, which upgrades zlib from 1.3.1-r2 to 1.3.2-r0, resolving CVE-2026-22184. Suppressing a resolved CVE creates false confidence and obscures scan accuracy. The entry's own removal criteria have been met: Alpine released `zlib 1.3.2-r0`.

### 5.2 Extend `GHSA-69x3-g4r3-p962` (nebula) Expiry

**Action:** Update the `expiry` field and review comment in the nebula block.

Current:
```yaml
    expiry: "2026-03-05"  # Re-evaluate in 14 days (2026-02-19 + 14 days)
```

New:
```yaml
    expiry: "2026-04-13"  # Re-evaluated 2026-03-13: smallstep/certificates stable still v0.27.5, no nebula v1.10+ requirement. Extended 30 days.
```

Update the review comment line:
```
  # - Next review: 2026-04-13.
  # - Reviewed 2026-03-13: smallstep stable still v0.27.5 (no nebula v1.10+ requirement). Extended 30 days.
  # - Remove suppression immediately once upstream fixes.
```

**Reason:** As of 2026-03-13, `smallstep/certificates` has not released a stable version requiring nebula v1.10+. The constraint analysis from 2026-02-19 remains valid. Expiry extended 30 days to 2026-04-13.

---

## 6. File Change Summary

| File | Change | Scope |
|------|--------|-------|
| `Dockerfile` | Remove `curl`, `binutils`, `libc-utils` from `apk add` | Line ~413–415 |
| `Dockerfile` | Replace `curl` with `wget` in GeoLite2 CI download path | Line ~437–441 |
| `Dockerfile` | Replace `curl` with `wget` in GeoLite2 non-CI path; add `[ -s FILE ]` guard | Line ~445–452 |
| `Dockerfile` | Replace `curl` with `wget` in HEALTHCHECK | Line ~581 |
| `.docker/docker-entrypoint.sh` | Replace `curl` with `wget` in Caddy readiness poll | Line ~368 |
| `.grype.yaml` | Delete `CVE-2026-22184` (zlib) ignore block entirely | zlib block |
| `.grype.yaml` | Extend `GHSA-69x3-g4r3-p962` expiry to 2026-04-13; update review comment | nebula block |

---

## 7. Commit Slicing Strategy

**Single PR** — all changes are security-related and tightly coupled. Splitting curl removal from binutils removal would produce an intermediate commit with partially resolved HIGHs, offering no validation benefit and complicating rollback.

Suggested commit message:
```
fix(security): remove curl and binutils from runtime image

Replace curl with busybox wget for GeoLite2 downloads, HEALTHCHECK,
and the Caddy readiness probe. Remove binutils and libc-utils from the
runtime image; the entrypoint objdump check has a documented fallback
for missing objdump. Eliminates CVE-2026-3805 (curl HIGH), CVE-2025-69650
and CVE-2025-69649 (binutils HIGH), plus 8 associated MEDIUM findings.

Remove the now-resolved CVE-2026-22184 (zlib) suppression from
.grype.yaml and extend GHSA-69x3-g4r3-p962 (nebula) expiry to
2026-04-13 pending upstream smallstep/certificates update.
```

---

## 8. Expected Scan Results After Fix

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| HIGH count | 3 | **0** | −3 |
| MEDIUM count | ~13 | ~5 | −8 |
| LOW count | ~2 | ~0 | −2 |
| `fail-on-severity: high` | ❌ FAIL | ✅ PASS | — |
| CI supply-chain scan | ❌ BLOCKED | ✅ GREEN | — |

Remaining MEDIUMs after fix (~5):
- `busybox` / `busybox-extras` / `ssl_client` — CVE-2025-60876 (CRLF injection in wget/ssl_client; no Alpine fix; Charon application code does not invoke `wget` directly at runtime)

---

## 9. Validation Steps

1. Rebuild Docker image: `docker build -t charon:test .`
2. Run Grype scan: `grype charon:test` — confirm zero HIGH findings
3. Confirm HEALTHCHECK probe passes: start container, check `docker inspect` for `healthy` status
4. Confirm Caddy readiness: inspect entrypoint logs for `"Caddy is ready!"`
5. Run E2E suite: `npx playwright test --project=firefox`
6. Push branch and confirm CI supply-chain workflow exits green
