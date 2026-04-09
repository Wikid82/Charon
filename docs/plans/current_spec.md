# Nightly Build Vulnerability Remediation Plan

**Date**: 2026-04-09
**Status**: Draft — Awaiting Approval
**Scope**: Dependency security patches for 5 HIGH + 3 MEDIUM vulnerability groups
**Target**: Single PR — all changes ship together
**Archived**: Previous plan (CrowdSec Hub Bootstrapping) → `docs/plans/archive/crowdsec-hub-bootstrap-spec.md`

---

## 1. Problem Statement

The Charon nightly build is failing container image vulnerability scans with **5 HIGH-severity** and **multiple MEDIUM-severity** findings. These vulnerabilities exist across three compiled binaries embedded in the container image:

1. **Charon backend** (`/app/charon`) — Go binary built from `backend/go.mod`
2. **Caddy** (`/usr/bin/caddy`) — Built via xcaddy in the Dockerfile Caddy builder stage
3. **CrowdSec** (`/usr/local/bin/crowdsec`, `/usr/local/bin/cscli`) — Built from source in the Dockerfile CrowdSec builder stage

Additionally, the **nightly branch** was synced from development before the Go 1.26.2 bump landed, so the nightly image was compiled with Go 1.26.1 (confirmed in `ci_failure.log` line 55: `GO_VERSION: 1.26.1`).

---

## 2. Research Findings

### 2.1 Go Version Audit

All files on `development` / `main` already reference **Go 1.26.2**:

| File | Current Value | Status |
|------|---------------|--------|
| `backend/go.mod` | `go 1.26.2` | ✅ Current |
| `go.work` | `go 1.26.2` | ✅ Current |
| `Dockerfile` (`ARG GO_VERSION`) | `1.26.2` | ✅ Current |
| `.github/workflows/nightly-build.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/codecov-upload.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/quality-checks.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/codeql.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/benchmark.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/release-goreleaser.yml` | `'1.26.2'` | ✅ Current |
| `.github/workflows/e2e-tests-split.yml` | `'1.26.2'` | ✅ Current |
| `.github/skills/examples/gorm-scanner-ci-workflow.yml` | `'1.26.1'` | ❌ **Stale** |
| `scripts/install-go-1.26.0.sh` | `1.26.0` | ⚠️ Old install script (not used in CI/Docker builds) |

**Root Cause of Go stdlib CVEs**: The nightly branch's last sync predated the 1.26.2 bump. The next nightly sync from development will propagate 1.26.2 automatically. The only file requiring a fix is the example workflow.

### 2.2 Vulnerability Inventory

#### HIGH Severity (must fix — merge-blocking)

| # | CVE / GHSA | Package | Current | Fix | Binary | Dep Type |
|---|-----------|---------|---------|-----|--------|----------|
| 1 | CVE-2026-39883 | `go.opentelemetry.io/otel/sdk` | v1.40.0 | v1.43.0 | Caddy | Transitive (Caddy plugins → otelhttp → otel/sdk) |
| 2 | CVE-2026-34986 | `github.com/go-jose/go-jose/v3` | v3.0.4 | **v3.0.5** | Caddy | Transitive (caddy-security → JWT/JOSE stack) |
| 3 | CVE-2026-34986 | `github.com/go-jose/go-jose/v4` | v4.1.3 | **v4.1.4** | Caddy | Transitive (grpc v1.79.3 → go-jose/v4) |
| 4 | CVE-2026-32286 | `github.com/jackc/pgproto3/v2` | v2.3.3 | pgx/v4 v4.18.3 ¹ | CrowdSec | Transitive (CrowdSec → pgx/v4 v4.18.2 → pgproto3/v2) |

¹ pgproto3/v2 has **no patched release**. Fix requires upstream migration to pgx/v5 (uses pgproto3/v3). See §5 Risk Assessment.

#### MEDIUM Severity (fix in same pass)

| # | CVE / GHSA | Package(s) | Current | Fix | Binary | Dep Type |
|---|-----------|------------|---------|-----|--------|----------|
| 5 | GHSA-xmrv-pmrh-hhx2 | AWS SDK v2: `eventstream` v1.7.1, `cloudwatchlogs` v1.57.2, `kinesis` v1.40.1, `s3` v1.87.3 | See left | Bump all | CrowdSec | Direct deps of CrowdSec v1.7.7 |
| 6 | CVE-2026-32281, -32288, -32289 | Go stdlib | 1.26.1 | **1.26.2** | All (nightly image) | Toolchain |
| 7 | CVE-2026-39882 | OTel HTTP exporters: `otlploghttp` v0.16.0, `otlpmetrichttp` v1.40.0, `otlptracehttp` v1.40.0 | See left | Bump all | Caddy | Transitive (Caddy plugins → OTel exporters) |

### 2.3 Dependency Chain Analysis

#### Backend (`backend/go.mod`)

```
charon/backend (direct)
  └─ docker/docker v28.5.2+incompatible (direct)
       └─ otelhttp v0.68.0 (indirect)
            └─ otel/sdk v1.43.0 (indirect) — already at latest
  └─ grpc v1.79.3 (indirect)
  └─ otlptracehttp v1.42.0 (indirect) ── CVE-2026-39882
```

Backend resolved versions (verified via `go list -m -json`):

| Package | Version | Type |
|---------|---------|------|
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.42.0 | indirect |
| `google.golang.org/grpc` | v1.79.3 | indirect |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.68.0 | indirect |

**Not present in backend**: go-jose/v3, go-jose/v4, otel/sdk, pgproto3/v2, AWS SDK, otlploghttp, otlpmetrichttp.

#### CrowdSec Binary (Dockerfile `crowdsec-builder` stage)

Source: CrowdSec v1.7.7 `go.mod` (verified via `git clone --depth 1 --branch v1.7.7`):

```
crowdsec v1.7.7
  └─ pgx/v4 v4.18.2 (direct) → pgproto3/v2 v2.3.3 (indirect) ── CVE-2026-32286
  └─ aws-sdk-go-v2/service/s3 v1.87.3 (direct) ── GHSA-xmrv-pmrh-hhx2
  └─ aws-sdk-go-v2/service/cloudwatchlogs v1.57.2 (direct) ── GHSA-xmrv-pmrh-hhx2
  └─ aws-sdk-go-v2/service/kinesis v1.40.1 (direct) ── GHSA-xmrv-pmrh-hhx2
  └─ aws-sdk-go-v2/aws/protocol/eventstream v1.7.1 (indirect) ── GHSA-xmrv-pmrh-hhx2
  └─ otel v1.39.0, otel/metric v1.39.0, otel/trace v1.39.0 (indirect)
```

Confirmed by Trivy image scan (`trivy-image-report.json`): pgproto3/v2 v2.3.3 flagged in `usr/local/bin/crowdsec` and `usr/local/bin/cscli`.

#### Caddy Binary (Dockerfile `caddy-builder` stage)

Built via xcaddy with plugins. go.mod is generated at build time. Vulnerable packages enter via:

```
xcaddy build (Caddy v2.11.2 + plugins)
  └─ caddy-security v1.1.61 → go-jose/v3 (JWT auth stack) ── CVE-2026-34986
  └─ grpc (patched to v1.79.3 in Dockerfile) → go-jose/v4 v4.1.3 ── CVE-2026-34986
  └─ Caddy/plugins → otel/sdk v1.40.0 ── CVE-2026-39883
  └─ Caddy/plugins → otlploghttp v0.16.0, otlpmetrichttp v1.40.0, otlptracehttp v1.40.0 ── CVE-2026-39882
```

---

## 3. Technical Specifications

### 3.1 Backend go.mod Changes

**File**: `backend/go.mod` (+ `backend/go.sum` auto-generated)

```bash
cd backend

# Upgrade grpc to v1.80.0 (security patches for transitive deps)
go get google.golang.org/grpc@v1.80.0

# CVE-2026-39882: OTel HTTP exporter (backend only has otlptracehttp)
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0

go mod tidy
```

Expected `go.mod` diff:
- `google.golang.org/grpc` v1.79.3 → v1.80.0
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` v1.42.0 → v1.43.0

### 3.2 Dockerfile — Caddy Builder Stage Patches

**File**: `Dockerfile`, within the caddy-builder `RUN bash -c '...'` block, in the **Stage 2: Apply security patches** section.

Add after the existing `go get golang.org/x/net@v${XNET_VERSION};` line:

```bash
# CVE-2026-34986: go-jose JOSE/JWT validation bypass
# Fix in v3.0.5 and v4.1.4. Pin here until caddy-security ships fix.
# renovate: datasource=go depName=github.com/go-jose/go-jose/v3
go get github.com/go-jose/go-jose/v3@v3.0.5; \
# renovate: datasource=go depName=github.com/go-jose/go-jose/v4
go get github.com/go-jose/go-jose/v4@v4.1.4; \
# CVE-2026-39883: OTel SDK resource leak
# Fix in v1.43.0. Pin here until Caddy ships with updated OTel.
# renovate: datasource=go depName=go.opentelemetry.io/otel/sdk
go get go.opentelemetry.io/otel/sdk@v1.43.0; \
# CVE-2026-39882: OTel HTTP exporter request smuggling
# renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp@v0.19.0; \
# renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@v1.43.0; \
# renovate: datasource=go depName=go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0; \
```

Update existing grpc patch line from `v1.79.3` → `v1.80.0`:

```bash
# Before:
go get google.golang.org/grpc@v1.79.3; \
# After:
# CVE-2026-33186: gRPC-Go auth bypass (fixed in v1.79.3)
# CVE-2026-34986: go-jose/v4 transitive fix (requires grpc >= v1.80.0)
# renovate: datasource=go depName=google.golang.org/grpc
go get google.golang.org/grpc@v1.80.0; \
```

### 3.3 Dockerfile — CrowdSec Builder Stage Patches

**File**: `Dockerfile`, within the crowdsec-builder `RUN` block that patches dependencies.

Add after the existing `go get golang.org/x/net@v${XNET_VERSION}` line:

```bash
# CVE-2026-32286: pgproto3/v2 buffer overflow (no v2 fix exists; bump pgx/v4 to latest patch)
# renovate: datasource=go depName=github.com/jackc/pgx/v4
go get github.com/jackc/pgx/v4@v4.18.3 && \
# GHSA-xmrv-pmrh-hhx2: AWS SDK v2 event stream injection
# renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream
go get github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream@v1.7.8 && \
# renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs
go get github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs@v1.68.0 && \
# renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/service/kinesis
go get github.com/aws/aws-sdk-go-v2/service/kinesis@v1.43.5 && \
# renovate: datasource=go depName=github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.99.0 && \
```

CrowdSec grpc already at v1.80.0 — no change needed.

### 3.4 Example Workflow Fix

**File**: `.github/skills/examples/gorm-scanner-ci-workflow.yml` (line 28)

```yaml
# Before:
          go-version: "1.26.1"
# After:
          go-version: "1.26.2"
```

### 3.5 Go Stdlib CVEs (nightly branch — no code change needed)

The nightly workflow syncs `development → nightly` via `git merge --ff-only`. Since `development` already has Go 1.26.2 everywhere:
- Dockerfile `ARG GO_VERSION=1.26.2` ✓
- All CI workflows `GO_VERSION: '1.26.2'` ✓
- `backend/go.mod` `go 1.26.2` ✓

The next nightly run at 09:00 UTC will automatically propagate Go 1.26.2 to the nightly branch and rebuild the image.

---

## 4. Implementation Plan

### Phase 1: Playwright Tests (N/A)

No UI/UX changes — this is a dependency-only update. Existing E2E tests validate runtime behavior.

### Phase 2: Backend Implementation

| Task | File(s) | Action |
|------|---------|--------|
| 2.1 | `backend/go.mod`, `backend/go.sum` | Run `go get` commands from §3.1 |
| 2.2 | Verify build | `cd backend && go build ./cmd/api` |
| 2.3 | Verify vet | `cd backend && go vet ./...` |
| 2.4 | Verify tests | `cd backend && go test ./...` |
| 2.5 | Verify vulns | `cd backend && govulncheck ./...` |

### Phase 3: Dockerfile Implementation

| Task | File(s) | Action |
|------|---------|--------|
| 3.1 | `Dockerfile` (caddy-builder, ~L258-280) | Add go-jose v3/v4, OTel SDK, OTel exporter patches per §3.2 |
| 3.2 | `Dockerfile` (caddy-builder, ~L270) | Update grpc patch v1.79.3 → v1.80.0 |
| 3.3 | `Dockerfile` (crowdsec-builder, ~L360-370) | Add pgx, AWS SDK patches per §3.3 |
| 3.3a | CrowdSec binaries | After patching deps, run `go build` on CrowdSec binaries before full Docker build for faster compilation feedback |
| 3.4 | `Dockerfile` | Verify `docker build .` completes successfully (amd64) |

### Phase 4: CI / Misc Fixes

| Task | File(s) | Action |
|------|---------|--------|
| 4.1 | `.github/skills/examples/gorm-scanner-ci-workflow.yml` | Bump Go version 1.26.1 → 1.26.2 |

### Phase 5: Validation

| Task | Validation |
|------|------------|
| 5.1 | `cd backend && go build ./cmd/api` — compiles cleanly |
| 5.2 | `cd backend && go test ./...` — all tests pass |
| 5.3 | `cd backend && go vet ./...` — no issues |
| 5.4 | `cd backend && govulncheck ./...` — 0 findings |
| 5.5 | `docker build -t charon:vuln-fix .` — image builds for amd64 |
| 5.6 | Trivy scan on built image: `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --severity CRITICAL,HIGH charon:vuln-fix` — 0 HIGH (pgproto3/v2 excepted) |
| 5.7 | Container health: `docker run -d -p 8080:8080 charon:vuln-fix && curl -f http://localhost:8080/health` |
| 5.8 | E2E Playwright tests pass against rebuilt container |

---

## 5. Risk Assessment

### Low Risk

| Change | Risk | Rationale |
|--------|------|-----------|
| `go-jose/v3` v3.0.4 → v3.0.5 | Low | Security patch release only |
| `go-jose/v4` v4.1.3 → v4.1.4 | Low | Security patch release only |
| `otel/sdk` v1.40.0 → v1.43.0 (Caddy) | Low | Minor bumps, backwards compatible |
| `otlptracehttp` v1.42.0 → v1.43.0 (backend) | Low | Minor bump |
| OTel exporters (Caddy) | Low | Minor/patch bumps |
| Go version example fix | None | Non-runtime file |

### Medium Risk

| Change | Risk | Mitigation |
|--------|------|------------|
| `grpc` v1.79.3 → v1.80.0 | Medium | Minor version bump. gRPC is indirect — Charon doesn't use gRPC directly. Run full test suite. Verify Caddy and CrowdSec still compile. |
| AWS SDK major bumps (s3 v1.87→v1.99, cloudwatchlogs v1.57→v1.68, kinesis v1.40→v1.43) | Medium | CrowdSec build may fail if internal APIs changed between versions. Mitigate: run `go mod tidy` after patches and verify CrowdSec binaries compile. **Note:** AWS SDK Go v2 packages use independent semver within the `v1.x.x` line — these are minor version bumps, not major API breaks. |
| `pgx/v4` v4.18.2 → v4.18.3 | Medium | Patch release should be safe. May not fully resolve pgproto3/v2 since no patched v2 exists. |

### Known Limitation: pgproto3/v2 (CVE-2026-32286)

The `pgproto3/v2` module has **no patched release** — the fix exists only in `pgproto3/v3` (used by `pgx/v5`). CrowdSec v1.7.7 uses `pgx/v4` which depends on `pgproto3/v2`. Remediation:

1. Bump `pgx/v4` to v4.18.3 (latest v4 patch) — may transitively resolve the issue
2. If scanner still flags pgproto3/v2 after the bump: document as **accepted risk with upstream tracking**
3. Monitor CrowdSec releases for `pgx/v5` migration
4. Consider upgrading `CROWDSEC_VERSION` ARG if a newer CrowdSec release ships with pgx/v5

---

## 6. Acceptance Criteria

- [ ] `cd backend && go build ./cmd/api` succeeds with zero warnings
- [ ] `cd backend && go test ./...` passes with zero failures
- [ ] `cd backend && go vet ./...` reports zero issues
- [ ] `cd backend && govulncheck ./...` reports zero findings
- [ ] Docker image builds successfully for amd64
- [ ] Trivy/Grype scan of built image shows 0 new HIGH findings (pgproto3/v2 excepted if upstream unpatched)
- [ ] Container starts, health check passes on port 8080
- [ ] Existing E2E Playwright tests pass against rebuilt container
- [ ] No new compile errors in Caddy or CrowdSec builder stages
- [ ] `backend/go.mod` shows updated versions for grpc, otlptracehttp

---

## 7. Commit Slicing Strategy

### Decision: Single PR

**Rationale**: All changes are dependency version bumps with no feature or behavioral changes. They address a single concern (security vulnerability remediation) and should be reviewed and merged atomically to avoid partial-fix states.

**Trigger reasons for single PR**:
- All changes are security patches — cannot ship partial fixes
- Changes span backend + Dockerfile + CI config — logically coupled
- No risk of one slice breaking another
- Total diff is small (go.mod/go.sum + Dockerfile patch lines + 1 YAML fix)

### PR-1: Nightly Build Vulnerability Remediation

**Scope**: All changes in §3.1–§3.4

**Files modified**:

| File | Change Type |
|------|-------------|
| `backend/go.mod` | Dependency version bumps (grpc, otlptracehttp) |
| `backend/go.sum` | Auto-generated checksum updates |
| `Dockerfile` | Add `go get` patches in caddy-builder and crowdsec-builder stages |
| `.github/skills/examples/gorm-scanner-ci-workflow.yml` | Go version 1.26.1 → 1.26.2 |

**Dependencies**: None (standalone)

**Validation gates**:
1. `go build` / `go test` / `go vet` / `govulncheck` pass
2. Docker image builds for amd64
3. Trivy/Grype scan passes (0 new HIGH)
4. E2E tests pass

**Rollback**: Revert PR. All changes are version pins — reverting restores previous state with no data migration needed.

### Post-merge Actions

1. Nightly build will automatically sync development → nightly and rebuild the image with all patches
2. Monitor next nightly scan for zero HIGH findings
3. If pgproto3/v2 still flagged: open tracking issue for CrowdSec pgx/v5 upstream migration
4. If any AWS SDK bump breaks CrowdSec compilation: pin to intermediate version and document

---

## 8. CI Failure Amendment: pgx/v4 Module Path Mismatch

**Date**: 2026-04-09
**Failure**: PR #921 `build-and-push` job, step `crowdsec-builder 7/11`
**Error**: `go: github.com/jackc/pgx/v4@v5.9.1: invalid version: go.mod has non-.../v4 module path "github.com/jackc/pgx/v5" (and .../v4/go.mod does not exist) at revision v5.9.1`

### Root Cause

Dockerfile line 386 specifies `go get github.com/jackc/pgx/v4@v5.9.1`. This mixes the v4 module path with a v5 version tag. Go's semantic import versioning rejects this because tag `v5.9.1` declares module path `github.com/jackc/pgx/v5` in its go.mod.

### Fix

**Dockerfile line 386** — change:
```dockerfile
go get github.com/jackc/pgx/v4@v5.9.1 && \
```
to:
```dockerfile
go get github.com/jackc/pgx/v4@v4.18.3 && \
```

No changes needed to the Renovate annotation (line 385) or the CVE comment (line 384) — both are already correct.

### Why v4.18.3

- CrowdSec v1.7.7 uses `github.com/jackc/pgx/v4 v4.18.2` (direct dependency)
- v4.18.3 is the latest and likely final v4 release
- pgproto3/v2 is archived at v2.3.3 (July 2025) — no fix will be released in the v2 line
- The CVE (pgproto3/v2 buffer overflow) can only be fully resolved by CrowdSec migrating to pgx/v5 upstream
- Bumping pgx/v4 to v4.18.3 gets the latest v4 maintenance patch; the CVE remains an accepted risk per §5

### Validation

The same `docker build` that previously failed at step 7/11 should now pass through the CrowdSec dependency patching stage and proceed to compilation (steps 8-11).

---

## 9. Commands Reference

```bash
# === Backend dependency upgrades ===
cd /projects/Charon/backend

go get google.golang.org/grpc@v1.80.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.43.0
go mod tidy

# === Validate backend ===
go build ./cmd/api
go test ./...
go vet ./...
govulncheck ./...

# === Docker build (after Dockerfile edits) ===
cd /projects/Charon
docker build -t charon:vuln-fix .

# === Scan built image ===
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy:latest image \
  --severity CRITICAL,HIGH \
  charon:vuln-fix

# === Quick container health check ===
docker run -d --name charon-vuln-test -p 8080:8080 charon:vuln-fix
sleep 10
curl -f http://localhost:8080/health
docker stop charon-vuln-test && docker rm charon-vuln-test
```
