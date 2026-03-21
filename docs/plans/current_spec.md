# Security Remediation Plan — 2026-03-20 Audit

**Date**: 2026-03-20
**Scope**: All patchable CVEs and code findings from the 2026-03-20 QA security scan
**Source**: `docs/reports/qa_security_scan_report.md`
**Status**: Draft — Awaiting implementation

---

## 1. Introduction

### Overview

A full-stack security audit conducted on 2026-03-20 identified 18 findings across the Charon
container image, Go modules, source code, and Python development tooling. This plan covers all
**actionable** items that can be resolved without waiting for upstream patches.

The audit also confirmed two prior remediations are complete:

- **CHARON-2026-001** (Debian CVE cluster): The Alpine 3.23.3 migration eliminated all 7 HIGH
  Debian CVEs. `SECURITY.md` must be updated to reflect this as patched.
- **CVE-2026-25793** (nebula in Caddy): Resolved by `CADDY_PATCH_SCENARIO=B`.

### Objectives

1. Rebuild the `charon:local` Docker image so CrowdSec binaries are compiled with a patched Go
   toolchain, resolving 1 CRITICAL + 5 additional CVEs.
2. Suppress a gosec false positive in `mail_service.go` with a justification comment.
3. Fix an overly-permissive test file permission setting.
4. Upgrade Python development tooling to resolve 4 Medium/Low advisory findings.
5. Update `SECURITY.md` to accurately reflect the current vulnerability state:
   move resolved entries to Patched, expand CHARON-2025-001, and add new Known entries.
6. Confirm DS-0002 (Dockerfile root user) is a false positive.

### Out of Scope

- **CVE-2026-2673** (OpenSSL `libcrypto3`/`libssl3`): No Alpine fix available as of 2026-03-20.
  Tracked in `SECURITY.md` as Awaiting Upstream.
- **CHARON-2025-001 original cluster** (CVE-2025-58183/58186/58187/61729): Awaiting CrowdSec
  upstream release with Go 1.26.0+ binaries.

---

## 2. Research Findings

### 2.1 Container Image State

| Property | Value |
|----------|-------|
| OS | Alpine Linux 3.23.3 |
| Base image digest | `alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659` |
| Charon backend | go 1.26.1 — **clean** (govulncheck: 0 findings) |
| CrowdSec binaries (scanned) | go1.25.6 / go1.25.7 |
| CrowdSec binaries (Dockerfile intent) | go1.26.1 (see §3.1) |
| npm dependencies | **clean** (281 packages, 0 advisories) |

The contradiction between the scanned go1.25.6/go1.25.7 CrowdSec binaries and the Dockerfile's
`GO_VERSION=1.26.1` is because the `charon:local` image cached on the build host predates the
last Dockerfile update. A fresh Docker build will compile CrowdSec with go1.26.1.

### 2.2 Dockerfile — CrowdSec Build Stage

The `crowdsec-builder` stage is defined at Dockerfile line 334:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS crowdsec-builder
```

The controlling argument (Dockerfile line 13):

```dockerfile
# renovate: datasource=docker depName=golang versioning=docker
ARG GO_VERSION=1.26.1
```

**ARG name**: `GO_VERSION`
**Current value**: `1.26.1`
**Scope**: Shared — also used by `gosu-builder`, `backend-builder`, and `caddy-builder`.

**go1.26.1 vs go1.25.8**: Go follows a dual-branch patch model. CVEs patched in go1.25.7 are
simultaneously patched in the corresponding go1.26.x release. Since go1.26.1 was released after
the go1.25.7 fixes, it covers CVE-2025-68121 and CVE-2025-61732. CVE-2026-25679 and
CVE-2026-27142/CVE-2026-27139 (fixed in go1.25.8) require verification that go1.26.1 incorporates
the equivalent go1.25.8-level patches. If go1.26.2 is available at time of implementation,
prefer updating `GO_VERSION=1.26.2`.

**Action**: No Dockerfile ARG change is required if go1.26.1 covers all go1.25.8 CVEs. The fix
is a Docker image rebuild with `--no-cache`. If post-rebuild scanning still reports go stdlib
CVEs in CrowdSec binaries, increment `GO_VERSION` to the latest available stable go1.26.x patch.

### 2.3 Dockerfile — Final Stage USER Instruction (DS-0002)

The Dockerfile final stage contains (approximately line 625):

```dockerfile
# Security: Run the container as non-root by default.
USER charon
```

`charon` (uid 1000) is created earlier in the build sequence:

```dockerfile
RUN addgroup -S charon && adduser -S charon -G charon
```

The `charon` user owns `/app`, `/config`, and all runtime directories.
`SECURITY.md`'s Security Features section also states: "Charon runs as an unprivileged user
(`charon`, uid 1000) inside the container."

**Verdict: DS-0002 is a FALSE POSITIVE.** The `USER charon` instruction is present. The Trivy
repository scan flagged this against an older cached image or ran without full multi-stage build
context. No code change is required.

### 2.4 mail\_service.go — G203 Template Cast Analysis

**File**: `backend/internal/services/mail_service.go`, line 195
**Flagged code**: `data.Content = template.HTML(contentBuf.String())`

Data flow through `RenderNotificationEmail`:

1. `contentBytes` loaded from `emailTemplates.ReadFile("templates/" + templateName)` — an
   `//go:embed templates/*` embedded FS. Templates are compiled into the binary; fully trusted.
2. `contentTmpl.Execute(&contentBuf, data)` renders the inner template. Go's `html/template`
   engine **auto-escapes all string fields** in `data` at this step.
3. All user-supplied fields in `EmailTemplateData` (`Title`, `Message`, etc.) are pre-sanitized
   via `sanitizeForEmail()` before the struct is populated (confirmed at `notification_service.go`
   lines 332–333).
4. `template.HTML(contentBuf.String())` wraps the **already-escaped, fully-rendered** output
   as a trusted HTML fragment so the outer `baseTmpl.Execute` does not double-escape HTML
   entities when embedding `.Content` in the base layout template.

This is the idiomatic nested-template composition pattern in Go's `html/template` package.
The cast is intentional and safe because the content it wraps was produced by `html/template`
execution (not from raw user input).

**Verdict: FALSE POSITIVE.** Fix: suppress with `// #nosec G203` and `//nolint:gosec`.

### 2.5 docker\_service\_test.go — G306 File Permission

**File**: `backend/internal/services/docker_service_test.go`, line 231

```go
// Current
require.NoError(t, os.WriteFile(socketFile, []byte(""), 0o660))
```

`0o660` (rw-rw----) grants write access to the file's group. The correct mode for a temporary
test socket placeholder is `0o600` (rw-------). No production impact; trivial fix.

### 2.6 Python Dev Tooling

Affects the development host only. None of these packages enter the production Docker image.

| Package | Installed | Target | Advisory |
|---------|-----------|--------|----------|
| `filelock` | 3.20.0 | ≥ 3.20.3 | GHSA-qmgc-5h2g-mvrw, GHSA-w853-jp5j-5j7f |
| `virtualenv` | 20.35.4 | ≥ 20.36.1 | GHSA-597g-3phw-6986 |
| `pip` | 25.3 | ≥ 26.0 | GHSA-6vgw-5pg2-w6jp |

### 2.7 CVE-2025-60876 (busybox) — Status Unconfirmed

`SECURITY.md` (written 2026-02-04) stated Alpine had patched CVE-2025-60876. The 2026-03-18
`grype` image scan reports `busybox` 1.37.0-r30 with no fixed version. This requires live
verification against a freshly built `charon:local` image before adding to SECURITY.md.

---

## 3. Technical Specifications

### P1 — Docker Image Rebuild (CrowdSec Go Toolchain)

**Resolves**: CVE-2025-68121 (CRITICAL), CVE-2026-25679 (HIGH), CVE-2025-61732 (HIGH),
CVE-2026-27142 (MEDIUM), CVE-2026-27139 (LOW), GHSA-fw7p-63qq-7hpr (LOW).

#### Dockerfile ARG Reference

| File | Line | ARG Name | Current Value | Action |
|------|------|----------|---------------|--------|
| `Dockerfile` | 13 | `GO_VERSION` | `1.26.1` | No change required if go1.26.1 covers go1.25.8-equivalent patches. Increment to latest stable go1.26.x only if post-rebuild scan confirms CVEs persist in CrowdSec binaries. |

The `crowdsec-builder` stage consumes this ARG as:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS crowdsec-builder
```

#### Build Command

```bash
docker build --no-cache -t charon:local .
```

`--no-cache` forces the CrowdSec builder to compile fresh binaries against the current toolchain
and prevents Docker from reusing a cached layer that produced the go1.25.6 binaries.

#### Post-Rebuild Validation

```bash
# Confirm CrowdSec binary toolchain version
docker run --rm charon:local cscli version

# Scan for remaining stdlib CVEs in CrowdSec binaries
grype charon:local -o table --only-fixed | grep -E "CRITICAL|HIGH"

# Expected: CVE-2025-68121, CVE-2026-25679, CVE-2025-61732 should no longer appear
```

If any of those CVEs persist post-rebuild, update the ARG:

```dockerfile
# Dockerfile line 13 — increment to latest stable go1.26.x patch
# renovate: datasource=docker depName=golang versioning=docker
ARG GO_VERSION=1.26.2   # or latest stable at time of implementation
```

### P2 — DS-0002 (Dockerfile Root User): FALSE POSITIVE

| Evidence | Location |
|----------|----------|
| `USER charon` present | `Dockerfile` line ~625 |
| `addgroup -S charon && adduser -S charon -G charon` | Earlier in final stage |
| Non-root documented | `SECURITY.md` Security Features section |

**No code change required.** Do not add DS-0002 as a real finding to `SECURITY.md`.

### P3 — G203: mail\_service.go template.HTML Cast

**File**: `backend/internal/services/mail_service.go`
**Line**: 195

Current code:
```go
data.Content = template.HTML(contentBuf.String())
```

Proposed change — add suppression comment immediately above the line, inline annotation on
the same line:

```go
// #nosec G203 -- contentBuf is the output of html/template.Execute, which auto-escapes all
// string fields in EmailTemplateData. The cast prevents double-escaping when this rendered
// fragment is embedded in the outer base layout template.
data.Content = template.HTML(contentBuf.String()) //nolint:gosec
```

### P4 — G306: docker\_service\_test.go File Permission

**File**: `backend/internal/services/docker_service_test.go`
**Line**: 231

| | Current | Proposed |
|-|---------|----------|
| Permission | `0o660` | `0o600` |

```go
// Current
require.NoError(t, os.WriteFile(socketFile, []byte(""), 0o660))

// Proposed
require.NoError(t, os.WriteFile(socketFile, []byte(""), 0o600))
```

### P5 — Python Dev Tooling Upgrade

Dev environment only; does not affect the production container.

```bash
pip install --upgrade filelock virtualenv pip
```

Post-upgrade verification:

```bash
pip list | grep -E "filelock|virtualenv|pip"
pip audit   # should report 0 MEDIUM/HIGH advisories for these packages
```

---

## 4. SECURITY.md Changes

All edits must conform to the entry format specified in
`.github/instructions/security.md.instructions.md`. The following is a field-level description
of every required SECURITY.md change.

### 4.1 Move CHARON-2026-001: Known → Patched

**Remove** the entire `### [HIGH] CHARON-2026-001 · Debian Base Image CVE Cluster` block from
`## Known Vulnerabilities`.

**Add** the following entry at the **top** of `## Patched Vulnerabilities` (newest-patched first,
positioned above the existing CVE-2025-68156 entry):

```markdown
### ✅ [HIGH] CHARON-2026-001 · Debian Base Image CVE Cluster

| Field        | Value |
|--------------|-------|
| **ID**       | CHARON-2026-001 (aliases: CVE-2026-0861, CVE-2025-15281, CVE-2026-0915, CVE-2025-13151, and 2 libtiff HIGH CVEs) |
| **Severity** | High · 8.4 (highest per CVSS v3.1) |
| **Patched**  | 2026-03-20 (Alpine base image migration complete) |

**What**
Seven HIGH-severity CVEs in Debian Trixie base image system libraries (`glibc`, `libtasn1-6`,
`libtiff`). These vulnerabilities resided in the container's OS-level packages with no available
fixes from the Debian Security Team.

**Who**
- Discovered by: Automated scan (Trivy)
- Reported: 2026-02-04

**Where**
- Component: Debian Trixie base image (`libc6`, `libc-bin`, `libtasn1-6`, `libtiff`)
- Versions affected: All Charon container images built on Debian Trixie base

**When**
- Discovered: 2026-02-04
- Patched: 2026-03-20
- Time to patch: 45 days

**How**
OS-level shared libraries bundled in the Debian Trixie container base image. Exploitation
required local container access or a prior application-level compromise to reach the vulnerable
library code. Caddy reverse proxy ingress filtering and container isolation limited the
effective attack surface.

**Resolution**
Migrated the container base image from Debian Trixie to Alpine Linux 3.23.3. Confirmed via
`docker inspect charon:local` showing Alpine 3.23.3. All 7 Debian HIGH CVEs are eliminated.
Post-migration Trivy scan reports 0 HIGH/CRITICAL vulnerabilities in the base OS layer.

- Spec: [docs/plans/alpine_migration_spec.md](docs/plans/alpine_migration_spec.md)
- Advisory: [docs/security/advisory_2026-02-04_debian_cves_temporary.md](docs/security/advisory_2026-02-04_debian_cves_temporary.md)
```

### 4.2 Update CHARON-2025-001 in Known Vulnerabilities

Apply the following field-level changes to the existing entry:

**Field: `**ID**`**

| | Current | Proposed |
|-|---------|----------|
| Aliases | `CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729` | `CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729, CVE-2025-68121, CVE-2026-25679, CVE-2025-61732` |

**Field: `**Status**`**

| | Current | Proposed |
|-|---------|----------|
| Status | `Awaiting Upstream` | `Fix In Progress` |

**Field: `**What**` — replace paragraph with:**

> Multiple Go standard library CVEs (HTTP/2 handling, TLS certificate validation, archive
> parsing, and net/http) present in CrowdSec binaries bundled with Charon. The original cluster
> (compiled against go1.25.1) was partially addressed as CrowdSec updated to go1.25.6/go1.25.7,
> but new CVEs — including CVE-2025-68121 (CRITICAL) — continue to accumulate against those
> versions. All CVEs in this cluster resolve when CrowdSec binaries are rebuilt against
> go ≥ 1.25.8 (or the equivalent go1.26.x patch). Charon's own application code is unaffected.

**Field: `Versions affected` (in `**Where**`)**

| | Current | Proposed |
|-|---------|----------|
| Versions affected | `All Charon versions shipping CrowdSec binaries compiled against Go < 1.26.0` | `All Charon versions shipping CrowdSec binaries compiled against Go < 1.25.8 (or equivalent go1.26.x patch)` |

**Field: `**Planned Remediation**` — replace paragraph with:**

> Rebuild the `charon:local` Docker image using the current Dockerfile. The `crowdsec-builder`
> stage at Dockerfile line 334 compiles CrowdSec from source against
> `golang:${GO_VERSION}-alpine` (currently go1.26.1), which incorporates the equivalent of the
> go1.25.7 and go1.25.8 patch series. Use `docker build --no-cache` to force recompilation of
> CrowdSec binaries. See: [docs/plans/current_spec.md](docs/plans/current_spec.md)

### 4.3 Add New Known Entries

Insert the following entries into `## Known Vulnerabilities`. Sort order: CRITICAL entries first
(currently none), then HIGH, MEDIUM, LOW. Place CVE-2025-68121 before CVE-2026-2673.

#### New Entry 1: CVE-2025-68121 (CRITICAL)

```markdown
### [CRITICAL] CVE-2025-68121 · Go stdlib — CrowdSec Bundled Binaries

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2025-68121 (see also CHARON-2025-001) |
| **Severity** | Critical |
| **Status**   | Fix In Progress |

**What**
A critical vulnerability in the Go standard library present in CrowdSec binaries bundled with
Charon. The binaries in the current `charon:local` image were compiled with go1.25.6, which is
affected. Fixed in go1.25.7 (and the equivalent go1.26.x patch). All CVEs in this component
resolve upon Docker image rebuild using the current Dockerfile (go1.26.1 toolchain).

**Who**
- Discovered by: Automated scan (grype, 2026-03-20)
- Reported: 2026-03-20
- Affects: CrowdSec Agent component within the container

**Where**
- Component: CrowdSec Agent (`cscli`, `crowdsec` binaries)
- Versions affected: Charon images with CrowdSec binaries compiled against go1.25.6 or earlier

**When**
- Discovered: 2026-03-20
- Disclosed (if public): 2026-03-20
- Target fix: Docker image rebuild (see CHARON-2025-001)

**How**
The vulnerability exists in the Go standard library compiled into CrowdSec's distributed
binaries. Exploitation targets CrowdSec's internal processing paths; the agent's network
interfaces are not directly exposed through Charon's primary API surface.

**Planned Remediation**
Rebuild the Docker image with `docker build --no-cache`. The `crowdsec-builder` stage compiles
CrowdSec from source against go1.26.1 (Dockerfile `ARG GO_VERSION=1.26.1`, line 13), which
incorporates the equivalent of the go1.25.7 patch. See CHARON-2025-001 and
[docs/plans/current_spec.md](docs/plans/current_spec.md).
```

#### New Entry 2: CVE-2026-2673 (HIGH ×2 — OpenSSL)

```markdown
### [HIGH] CVE-2026-2673 · OpenSSL TLS 1.3 Key Exchange Downgrade — Alpine 3.23.3

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-2673 |
| **Severity** | High · 7.5 |
| **Status**   | Awaiting Upstream |

**What**
An OpenSSL TLS 1.3 key exchange group downgrade vulnerability affecting `libcrypto3` and
`libssl3` in Alpine 3.23.3. A server configured with the `DEFAULT` keyword in its key group
list may negotiate a weaker cipher suite than intended. Charon's Caddy TLS configuration does
not use `DEFAULT` key groups explicitly, materially limiting practical impact. No Alpine APK
fix is available as of 2026-03-20.

**Who**
- Discovered by: Automated scan (grype, image scan 2026-03-18)
- Reported: 2026-03-20 (OpenSSL advisory: 2026-03-13)
- Affects: Container TLS stack

**Where**
- Component: Alpine 3.23.3 base image (`libcrypto3` 3.5.5-r0, `libssl3` 3.5.5-r0)
- Versions affected: All Charon images built on Alpine 3.23.3 with these package versions

**When**
- Discovered: 2026-03-13 (OpenSSL advisory)
- Disclosed (if public): 2026-03-13
- Target fix: Awaiting Alpine security tracker patch

**How**
The OpenSSL TLS 1.3 server may fail to negotiate the configured key exchange group when the
configuration includes the `DEFAULT` keyword, potentially allowing a downgrade to a weaker
cipher suite. Exploitation requires a man-in-the-middle attacker capable of intercepting and
influencing TLS handshake negotiation.

**Planned Remediation**
Monitor https://security.alpinelinux.org/vuln/CVE-2026-2673. Once Alpine releases a patched
APK for `libcrypto3`/`libssl3`, either update the pinned `ALPINE_IMAGE` SHA256 digest in the
Dockerfile or apply an explicit upgrade in the final stage:

```dockerfile
RUN apk upgrade --no-cache libcrypto3 libssl3
```
```

### 4.4 CVE-2025-60876 (busybox) — Conditional Entry

**Do not add until the post-rebuild scan verification in Phase 3 is complete.**

Verification command (run after rebuilding `charon:local`):

```bash
grype charon:local -o table | grep -i busybox
```

- **If busybox shows CVE-2025-60876 with no fixed version** → add the entry below to `SECURITY.md`.
- **If busybox is clean** → do not add; the previous SECURITY.md note was correct.

Conditional entry (add only if scan confirms vulnerability):

```markdown
### [MEDIUM] CVE-2025-60876 · busybox Heap Overflow — Alpine 3.23.3

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2025-60876 |
| **Severity** | Medium · 6.5 |
| **Status**   | Awaiting Upstream |

**What**
A heap overflow vulnerability in busybox affecting `busybox`, `busybox-binsh`, `busybox-extras`,
and `ssl_client` in Alpine 3.23.3. The live scanner reports no fix version for 1.37.0-r30,
contradicting an earlier internal note that stated Alpine had patched this CVE.

**Who**
- Discovered by: Automated scan (grype, image scan 2026-03-18)
- Reported: 2026-03-20
- Affects: Container OS-level utility binaries

**Where**
- Component: Alpine 3.23.3 base image (`busybox` 1.37.0-r30, `busybox-binsh`, `busybox-extras`, `ssl_client`)
- Versions affected: Charon images with busybox 1.37.0-r30

**When**
- Discovered: 2026-03-18 (scan)
- Disclosed (if public): Not confirmed
- Target fix: Awaiting Alpine upstream patch

**How**
Heap overflow in busybox utility programs. Requires shell or CLI access to the container;
not reachable through Charon's application interface.

**Planned Remediation**
Monitor Alpine security tracker for a patched busybox release. Rebuild the Docker image once
a fixed APK is available.
```

---

## 5. Implementation Plan

### Phase 1 — Pre-Implementation Verification

| Task | Command | Decision Gate |
|------|---------|---------------|
| Verify go1.26.1 covers go1.25.8 CVEs | Review Go 1.26.1 release notes / security advisories for CVE-2026-25679 equivalent | If not covered → update `GO_VERSION` to go1.26.2+ in Dockerfile |
| Confirm busybox CVE-2025-60876 status | Run post-rebuild grype scan (see Phase 3) | Determines §4.4 SECURITY.md addition |

### Phase 2 — Code Changes

| Task | File | Line | Change |
|------|------|------|--------|
| Suppress G203 false positive | `backend/internal/services/mail_service.go` | 195 | Add `// #nosec G203 --` comment block above; `//nolint:gosec` inline |
| Fix file permission G306 | `backend/internal/services/docker_service_test.go` | 231 | `0o660` → `0o600` |

### Phase 3 — Docker Rebuild + Scan

| Task | Command | Expected Outcome |
|------|---------|-----------------|
| Rebuild image | `docker build --no-cache -t charon:local .` | Fresh CrowdSec binaries compiled with go1.26.1 |
| Verify CrowdSec toolchain | `docker run --rm charon:local cscli version` | Reports go1.26.1 in version string |
| Confirm CVE cluster resolved | `grype charon:local -o table --only-fixed \| grep -E "CVE-2025-68121\|CVE-2026-25679\|CVE-2025-61732"` | No rows returned |
| Check busybox | `grype charon:local -o table \| grep busybox` | Determines §4.4 addition |
| Verify no USER regression | `docker inspect charon:local \| jq '.[0].Config.User'` | Returns `"charon"` |

### Phase 4 — Python Dev Tooling

| Task | Command |
|------|---------|
| Upgrade packages | `pip install --upgrade filelock virtualenv pip` |
| Verify | `pip audit` (expect 0 MEDIUM/HIGH for upgraded packages) |

### Phase 5 — SECURITY.md Updates

Execute in order:

1. Move CHARON-2026-001: Known → Patched (§4.1)
2. Update CHARON-2025-001 aliases, status, What, Versions affected, Planned Remediation (§4.2)
3. Add CVE-2025-68121 CRITICAL Known entry (§4.3, Entry 1)
4. Add CVE-2026-2673 HIGH Known entry (§4.3, Entry 2)
5. Add CVE-2025-60876 MEDIUM Known entry only if Phase 3 scan confirms it (§4.4)

---

## 6. Acceptance Criteria

| ID | Criterion | Evidence |
|----|-----------|----------|
| AC-1 | CrowdSec binaries compiled with go ≥ 1.25.8 equivalent | `cscli version` shows go1.26.x; grype reports 0 stdlib CVEs for CrowdSec |
| AC-2 | G203 suppressed with justification | `golangci-lint run ./...` reports 0 G203 findings |
| AC-3 | Test file permission corrected | Source shows `0o600`; gosec reports 0 G306 findings |
| AC-4 | Python dev tooling upgraded | `pip audit` reports 0 MEDIUM/HIGH for filelock, virtualenv, pip |
| AC-5 | SECURITY.md matches current state | CHARON-2026-001 in Patched; CHARON-2025-001 updated with new aliases; CVE-2025-68121 and CVE-2026-2673 in Known |
| AC-6 | DS-0002 confirmed false positive | `docker inspect charon:local \| jq '.[0].Config.User'` returns `"charon"` |
| AC-7 | Backend linting clean | `make lint-backend` exits 0 |
| AC-8 | All backend tests pass | `cd backend && go test ./...` exits 0 |

---

## 7. Commit Slicing Strategy

### Decision: Single PR

All changes originate from a single audit, are security remediations, and are low-risk. A
single PR provides a coherent audit trail and does not impose review burden that would justify
splitting. No schema migrations, no cross-domain feature work, no conflicting refactoring.

**Triggers that would justify a multi-PR split (none apply here)**:
- Security fix coupled to a large feature refactor
- Database schema migration alongside code changes
- Changes spanning unrelated subsystems requiring separate review queues

### PR-1 (sole PR): `fix(security): remediate 2026-03-20 audit findings`

**Files changed**:

| File | Change |
|------|--------|
| `backend/internal/services/mail_service.go` | `// #nosec G203` comment + `//nolint:gosec` at line 195 |
| `backend/internal/services/docker_service_test.go` | `0o660` → `0o600` at line 231 |
| `SECURITY.md` | Move CHARON-2026-001 to Patched; update CHARON-2025-001; add new Known entries |
| `Dockerfile` *(conditional)* | Increment `ARG GO_VERSION` only if post-rebuild scan shows CVEs persist |

**Dependencies**: Docker image rebuild is a CI/CD pipeline step triggered by merge, not a file
change tracked in this PR. Use `docker build --no-cache` for local validation.

**Validation gates before merge**:
1. `go test ./...` passes
2. `golangci-lint run ./...` reports 0 G203 and 0 G306 findings
3. Docker image rebuilt and `grype charon:local` clean for the P1 CVE cluster

**Rollback**: All changes are trivially reversible via `git revert`. The `//nolint` comment can
be removed, the permission reverted, and SECURITY.md restored. No infrastructure or database
changes are involved.

**Suggested commit message**:

```
fix(security): remediate 2026-03-20 audit findings

Suppress G203 false positive in mail_service.go with justification comment.
The template.HTML cast is safe because contentBuf is produced by
html/template.Execute, which auto-escapes all EmailTemplateData fields
before the rendered fragment is embedded in the base layout template.

Correct test file permission from 0o660 to 0o600 in docker_service_test.go
to satisfy gosec G306. No production impact.

Update SECURITY.md: move CHARON-2026-001 (Debian CVE cluster) to Patched
following confirmed Alpine 3.23.3 migration; expand CHARON-2025-001 aliases
to include CVE-2025-68121, CVE-2026-25679, and CVE-2025-61732; add Known
entries for CVE-2025-68121 (CRITICAL) and CVE-2026-2673 (HIGH, awaiting
upstream Alpine patch).

Docker image rebuild with --no-cache resolves the CrowdSec Go stdlib CVE
cluster (CVE-2025-68121 CRITICAL + 5 others) by recompiling CrowdSec from
source against go1.26.1 via the existing crowdsec-builder Dockerfile stage.
DS-0002 (Dockerfile root user) confirmed false positive — USER charon
instruction is present.
```

---

## 8. Items Requiring No Code Change

| Item | Reason |
|------|--------|
| DS-0002 (Dockerfile `USER`) | FALSE POSITIVE — `USER charon` present in final stage (~line 625) |
| CVE-2026-2673 (OpenSSL) | No Alpine fix available; tracked in SECURITY.md as Awaiting Upstream |
| CHARON-2025-001 original cluster | Awaiting CrowdSec upstream release with go1.26.0+ binaries |

---

## 9. Scan Artifact .gitignore Coverage

The following files exist at the repository root and contain scan output. Verify each is covered
by `.gitignore` to prevent accidental commits of stale or sensitive scan data:

```
grype-results.json
grype-results.sarif
trivy-report.json
trivy-image-report.json
vuln-results.json
sbom-generated.json
codeql-results-go.sarif
codeql-results-javascript.sarif
codeql-results-js.sarif
```

Verify with:

```bash
git check-ignore -v grype-results.json trivy-report.json trivy-image-report.json vuln-results.json
```

If any are missing a `.gitignore` pattern, add under a `# Security scan artifacts` comment:

```gitignore
# Security scan artifacts
grype-results*.json
grype-results*.sarif
trivy-*.json
trivy-*.sarif
vuln-results.json
sbom-generated.json
codeql-results-*.sarif
```
