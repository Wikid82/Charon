# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

To report a security issue, use
[GitHub Private Security Advisories](https://github.com/Wikid82/charon/security/advisories/new)
or open a [GitHub Issue](https://github.com/Wikid82/Charon/issues) for non-sensitive disclosures.

Please include a description, reproduction steps, impact assessment, and a non-destructive proof of
concept where possible.

We will acknowledge your report within **48 hours** and provide a remediation timeline within
**7 days**. Reporters are credited in release notes with their consent. We do not pursue legal
action against good-faith security researchers. Please allow **90 days** from initial report before
public disclosure.

---

## Known Vulnerabilities

Last reviewed: 2026-08-19

### [RESOLVED] GHSA-rw47-hm26-6wr7 / CVE-2026-44982 · CrowdSec AppSec Drops HTTP Request Body

| Field        | Value |
|--------------|-------|
| **ID**       | GHSA-rw47-hm26-6wr7 / CVE-2026-44982 |
| **Severity** | High |
| **Status**   | Resolved — crowdsec upgraded to v1.7.8 |

**What**
The CrowdSec AppSec component silently dropped the HTTP request body for chunked-encoded or
HTTP/2 requests, causing the Web Application Firewall rules to operate on an empty body. This
allowed malicious payloads in those request types to bypass WAF inspection.

**Who**

- Discovered by: CrowdSec security team
- Reported: 2026-05-27 (via GHSA advisory)
- Affects: Charon deployments with the AppSec/WAF security module enabled

**Where**

- Component: `github.com/crowdsecurity/crowdsec` (via `caddy-crowdsec-bouncer`)
- Versions affected: crowdsec < v1.7.8

**When**

- Discovered: 2026-05-27
- Fixed upstream: crowdsec v1.7.8
- Resolved in Charon: 2026-05-27

**How**
The body reader in the AppSec engine did not correctly buffer chunked or HTTP/2 request bodies
before passing them to the WAF rule evaluation pipeline. Requests with these transfer encodings
would present an empty body to inspection rules, meaning payload-based WAF rules had no effect.

**Resolution**
Upgraded `CROWDSEC_VERSION` to `v1.7.8` in the Dockerfile. The `caddy-crowdsec-bouncer` module
(upgraded to `v0.12.1`) now builds against crowdsec v1.7.8 which contains the body-reader fix.
Two source-level compatibility patches are applied at build time to handle breaking API changes
introduced between v1.6.x and v1.7.8 (`DecisionsListOpts` field types and
`version.DetectOS()` return signature).

---

### [RESOLVED] CVE-2026-2673 · OpenSSL TLS 1.3 Key Exchange Group Downgrade

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-2673 |
| **Severity** | High · 7.5 |
| **Status**   | Resolved — superseded by Alpine base image package upgrade |

**What**
An OpenSSL TLS 1.3 server may fail to negotiate the intended key exchange group when the
configuration includes the `DEFAULT` keyword, potentially allowing downgrade to weaker cipher
suites. Affected Alpine packages `libcrypto3` and `libssl3` at version 3.5.5-r0.

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-03-20
- Affects: Container runtime environment; Caddy reverse proxy TLS negotiation could be affected
  if default key group configuration is used

**Where**

- Component: Alpine base image (`libcrypto3`, `libssl3`)
- Versions affected: `libcrypto3`/`libssl3` 3.5.5-r0

**When**

- Discovered: 2026-03-20
- Disclosed (if public): 2026-03-13 (OpenSSL advisory)
- Resolved: 2026-08-19 (confirmed via re-scan; Alpine base image had already moved past the
  vulnerable package version by this date)

**How**
When an OpenSSL TLS 1.3 server configuration uses the `DEFAULT` keyword for key exchange groups,
the negotiation logic may select a weaker group than intended. Charon's Caddy TLS configuration
does not use the `DEFAULT` keyword, which limited practical exploitability throughout the
suppression window. The packages were present in the base image regardless of Caddy's
configuration.

**Resolution**
The pinned `ALPINE_IMAGE` base moved `libcrypto3`/`libssl3` past 3.5.5-r0 to 3.5.7-r0 via a
routine Alpine base image digest/package upgrade. Re-verified 2026-08-19: a current Grype scan no
longer reports CVE-2026-2673 at all. Per the suppression's own removal criteria, the
CVE-2026-2673 entries were removed from both `.trivyignore` and `.grype.yaml` simultaneously with
this entry moving to Resolved.

---

### [HIGH] CVE-2026-14456 · OpenSSL QUIC Server Unbounded Memory Allocation

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-14456 (affects `libcrypto3` and `libssl3`) |
| **Severity** | High |
| **Status**   | Awaiting Upstream |

**What**
OpenSSL's QUIC SERVER implementation, when processing valid QUIC Initial packets for unknown
destination connection IDs, allocates and queues new incoming channels with no limit. A remote
peer can grow memory unboundedly and make the QUIC listener unavailable, causing denial of
service. Affects OpenSSL 3.5+. Affected Alpine packages `libcrypto3` and `libssl3` at version
3.5.7-r0.

**Who**

- Discovered by: Automated scan (Grype); real-world discovery credited to Filipe Casal
  (Trail of Bits) with OpenAI
- Reported: 2026-08-19 (OpenSSL advisory published 2026-08-13; reported to OpenSSL 2026-06-25)
- Affects: Container runtime environment; does not affect Charon application code directly

**Where**

- Component: Alpine base image (`libcrypto3`, `libssl3`)
- Versions affected: `libcrypto3`/`libssl3` 3.5.7-r0 (Alpine v3.24 branch, build date 2026-06-10,
  predates the advisory)

**When**

- Discovered: 2026-08-19
- Disclosed (if public): 2026-08-13 (OpenSSL advisory)
- Target fix: When Alpine Security publishes a patched OpenSSL APK

**How**
The vulnerable code path is OpenSSL's native QUIC server accepting Initial packets for unknown
destination connection IDs. None of Charon's own binaries or bundled third-party binaries run an
OpenSSL-based QUIC server — Caddy's HTTP/3 stack uses `github.com/quic-go/quic-go` (pure Go,
userspace QUIC), and Charon's Go backend uses Go's native `crypto/tls`, not OpenSSL/cgo. The
`libcrypto3`/`libssl3` packages are present in the image only as Alpine system-library
dependencies backing TLS-client tools (wget/curl/busybox-extras); the vulnerable server path is
never invoked in this deployment.

**Planned Remediation**
Monitor <https://security.alpinelinux.org/vuln/CVE-2026-14456> for a patched Alpine APK. No
upstream fix available as of 2026-08-19 (current v3.24-branch package, build date 2026-06-10,
predates the 2026-08-13 advisory). Once available, update the pinned `ALPINE_IMAGE` digest in the
Dockerfile.

---

### [HIGH] CVE-2026-31790 · OpenSSL Vulnerability in Alpine Base Image

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-31790 (affects `libcrypto3` and `libssl3`) |
| **Severity** | High · CVSS pending |
| **Status**   | Awaiting Upstream |

**What**
An OpenSSL vulnerability in the Alpine base image system packages `libcrypto3` and `libssl3`.
This is a pre-existing issue in the Alpine base image and was not introduced by Charon.

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-04-09
- Affects: Container runtime environment; does not affect Charon application code directly

**Where**

- Component: Alpine base image (`libcrypto3`, `libssl3`)
- Versions affected: Current Alpine base image OpenSSL packages

**When**

- Discovered: 2026-04-09
- Disclosed (if public): Public
- Target fix: When Alpine Security publishes a patched OpenSSL APK

**How**
The vulnerability resides in Alpine's system OpenSSL library and affects TLS operations at
the OS level. Charon's application code does not directly invoke these libraries. Practical
exploitability depends on direct TLS usage through the system OpenSSL, which is limited to
the container runtime environment.

**Planned Remediation**
Monitor <https://security.alpinelinux.org/> for a patched Alpine APK. No upstream fix
available as of 2026-04-09. Once available, update the pinned `ALPINE_IMAGE` digest in the
Dockerfile.

---

### [MEDIUM] CVE-2026-33997 · Docker Off-by-One Plugin Privilege Validation

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-33997 (GHSA-pxq6-2prw-chj9) |
| **Severity** | Medium · 6.8 |
| **Status**   | Awaiting Upstream |

**What**
An off-by-one error in Docker Engine's plugin privilege validation could allow
a malicious plugin to escalate privileges. Charon uses the Docker client SDK
for container management and does not install or manage Docker plugins.

**Who**

- Discovered by: Automated scan (govulncheck, Grype)
- Reported: 2026-04-04
- Affects: Docker Engine plugin operators; Charon application is not directly vulnerable

**Where**

- Component: `github.com/docker/docker` v28.5.2+incompatible (Docker client SDK)
- Versions affected: Docker Engine < 29.3.1

**When**

- Discovered: 2026-04-04
- Disclosed (if public): Public
- Target fix: When moby/moby/v2 stabilizes or docker/docker import path is updated

**How**
The vulnerability is in Docker Engine's plugin privilege validation at the
daemon level. Charon does not use Docker plugins — it only manages containers
via the Docker client SDK. The attack requires a malicious Docker plugin to be
installed on the host, which is outside Charon's operational scope.

**Planned Remediation**
Monitor Moby advisory updates and verify scanner results against current modular
Moby dependency paths.

---

### [MEDIUM] CVE-2025-60876 · BusyBox wget HTTP Request Smuggling

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2025-60876 |
| **Severity** | Medium · 6.5 |
| **Status**   | Awaiting Upstream |

**What**
BusyBox wget through 1.37 accepts raw CR/LF and other C0 control bytes in the HTTP
request-target, allowing request line splitting and header injection (CWE-284).

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-03-24
- Affects: Container runtime environment; Charon does not invoke busybox wget in application logic

**Where**

- Component: Alpine 3.23.3 base image (`busybox` 1.37.0-r30)
- Versions affected: All Charon images using Alpine 3.23.3 with busybox < patched version

**When**

- Discovered: 2026-03-24
- Disclosed (if public): Not yet publicly disclosed with fix
- Target fix: When Alpine Security publishes a patched busybox APK

**How**
The vulnerable wget applet would need to be manually invoked inside the container with
attacker-controlled URLs. Charon's application logic does not use busybox wget. EPSS score is
0.00064 (0.20 percentile), indicating extremely low exploitation probability.

**Planned Remediation**
Monitor Alpine 3.23 for a patched busybox APK. No immediate action required. Practical risk to
Charon users is negligible since the vulnerable code path is not exercised.

---

### ✅ [HIGH] CVE-2026-45135 · Caddy FastCGI Unsafe Unicode Handling in splitPos

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-45135 |
| **Severity** | High · 8.1 |
| **Status**   | Fully remediated — `no-cache-filters: caddy-builder,crowdsec-builder` applied to nightly and E2E build workflows |

**What**
Caddy v2.11.2 contains unsafe Unicode handling in the FastCGI `splitPos` function. A
network-reachable attacker (no privileges required, high attack complexity) can trigger
potential confidentiality and integrity impact through malformed Unicode in FastCGI requests.

**Who**

- Discovered by: Automated scan (Trivy image scan)
- Reported: 2026-05-20
- Affects: Charon deployments using the FastCGI reverse proxy capability

**Where**

- Component: `github.com/caddyserver/caddy/v2` v2.11.2 (embedded in Charon binary via xcaddy)
- Versions affected: Caddy < v2.11.3

**When**

- Discovered: 2026-05-20
- Disclosed (if public): Public
- Fixed: 2026-05-21

**How**
Caddy is built from source via xcaddy in the Dockerfile multi-stage `caddy-builder` stage.
The `CADDY_VERSION` ARG was updated to `2.11.3` (commit `d94519d1`), but the nightly CI build
continued to produce images containing v2.11.2. Root cause: the GHA BuildKit layer cache
(`cache-from: type=gha,mode=max`) was serving the stale `caddy-builder` stage output from a
prior nightly run despite the ARG value change — a known edge case where GHA cache import
loses ARG-scoped metadata, preventing proper cache key invalidation.

**Remediation Applied**
Added `no-cache-filters: caddy-builder,crowdsec-builder` to the `build-and-push-nightly` job in
`.github/workflows/nightly-build.yml` and to the `Build Docker image` step in
`.github/workflows/e2e-tests-split.yml`. This forces both the `caddy-builder` and
`crowdsec-builder` stages to rebuild from scratch on every nightly and E2E build run, bypassing
the GHA layer cache for those stages. All other stages continue to benefit from the cache.

---

### [HIGH] CVE-2026-32286 · pgproto3 DoS via Negative DataRow Field Length

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-32286 |
| **Severity** | High · 7.5 |
| **Status**   | Awaiting Upstream |

**What**
`github.com/jackc/pgproto3/v2` v2.3.3 — `DataRow.Decode` does not validate field length
before allocation. A malicious or compromised PostgreSQL server can send a DataRow message
with a negative field length to trigger a panic or excessive memory allocation, causing denial
of service (CWE-400).

**Who**

- Discovered by: Automated scan (Trivy image scan)
- Reported: 2026-05-20
- Affects: Charon deployments connecting to an untrusted PostgreSQL server (non-default)

**Where**

- Component: `github.com/jackc/pgproto3/v2` v2.3.3 (transitive Go dependency)
- Versions affected: pgproto3/v2 < patched version

**When**

- Discovered: 2026-05-20
- Disclosed (if public): Public
- Target fix: When upstream publishes a patched release

**How**
Exploitation requires a malicious or compromised PostgreSQL server to send a crafted DataRow
message. Charon's default installation uses SQLite and does not connect to PostgreSQL. Users
deploying Charon with a PostgreSQL backend (non-standard configuration) may be exposed if the
database server is untrusted. EPSS score not yet available.

**Planned Remediation**
Monitor https://github.com/jackc/pgproto3 for a fix release. Upgrade the indirect dependency
once a patched version is available. Pre-existing; not introduced by PR #1031.

Re-verified 2026-07-29 (this entry's original `.trivyignore`/`.grype.yaml` review date had
lapsed): confirmed directly against upstream `go.mod` files that `jackc/pgproto3` is still
archived at v2.3.3 (no new tags) and that CrowdSec v1.7.8 — the current pin and latest stable
release — as well as v1.8.0-rc1 (latest including pre-releases) both still resolve
`github.com/jackc/pgx/v4 v4.18.3` → `github.com/jackc/pgproto3/v2 v2.3.3`. No migration to
pgx/v5 has landed upstream. Risk assessment unchanged. Suppression renewed in `.trivyignore` and
`.grype.yaml`; next review 2026-09-01, aligned with the two duplicate-root-cause advisories
(GHSA-jqcq-xjh3-6g23, GHSA-x6gf-mpr2-68h6) covering the same underlying bug.

---

### [LOW] CVE-2026-41889 · pgx/v4 Panic via Crafted PostgreSQL Wire Payload

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-41889 (GHSA-j88v-2chj-qfwx) |
| **Severity** | Low · 3.7 |
| **Status**   | Awaiting Upstream |

**What**
`github.com/jackc/pgx/v4` may panic when decoding a crafted PostgreSQL wire payload,
which can cause a denial-of-service condition in affected clients.

**Who**

- Discovered by: Automated scan (Trivy image scan)
- Reported: 2026-05-25
- Affects: Deployments using PostgreSQL-backed CrowdSec code paths (non-default)

**Where**

- Component: `github.com/jackc/pgx/v4` (transitive dependency in bundled CrowdSec components)
- Versions affected: pgx/v4 prior to upstream remediation

**When**

- Discovered: 2026-05-25
- Disclosed (if public): Public
- Target fix: When upstream dependencies migrate to a patched path (pgx/v5)

**How**
Exploitation requires a crafted PostgreSQL protocol payload delivered to an affected pgx/v4
decode path. Charon defaults to SQLite, so standard deployments do not expose this path.

**Planned Remediation**
Track upstream CrowdSec dependency updates and remove suppression once pgx/v4 is no longer
present in bundled components.

---

### [HIGH] GO-2026-5932 · golang.org/x/crypto/openpgp Unmaintained, Unsafe by Design

| Field        | Value |
|--------------|-------|
| **ID**       | GO-2026-5932 |
| **Severity** | Reported Unknown by Go vulndb; Trivy substitutes another vendor's rating |
| **Status**   | Awaiting Upstream (no fixed version exists) |

**What**
`golang.org/x/crypto/openpgp` v0.53.0 is flagged by the Go vulnerability database as
unmaintained and unsafe by design — this is not a specific patchable bug, it is Go's standing
recommendation to migrate off the openpgp subpackage entirely (superseded by
`github.com/ProtonMail/go-crypto`). No fixed `golang.org/x/crypto` version resolves this.

**Who**

- Discovered by: Automated scan (Trivy image scan, PR #1133)
- Reported: 2026-07-08
- Affects: `app/charon`, `usr/bin/caddy`, `usr/local/bin/crowdsec`, `usr/local/bin/cscli`

**Where**

- Component: `golang.org/x/crypto` v0.53.0 (declared dependency; embedded across all four binaries)
- Versions affected: all versions containing the `openpgp` subpackage — no fix exists

**When**

- Discovered: 2026-07-08
- Disclosed (if public): Public
- Target fix: N/A — no fixed version will be published; suppression is permanent absent a
  Trivy scanner change to symbol-level (rather than module-level) matching

**How**
Trivy's `gobinary` scanner flags based on module versions recorded in the binary's embedded
build info, not on which subpackages are actually imported and linked. Verified via
`go list -deps ./...` in `backend`: the `openpgp` subpackage is not imported anywhere in
Charon's own code, so the `app/charon` finding is a false positive. `caddy`, `crowdsec`, and
`cscli` are third-party binaries built in the Dockerfile; `golang.org/x/crypto` is a transitive
dependency of theirs that Charon does not control.

**Planned Remediation**
No remediation path exists upstream. Monitor whether caddy/crowdsec/cscli drop their dependency
on `x/crypto/openpgp`, and periodically re-run `govulncheck` to confirm Charon's own code stays
clean. Suppressed in `.trivyignore` and `.grype.yaml`; review 2026-08-08.

---

### [HIGH] GHSA-mh99-v99m-4gvg · brace-expansion DoS via `eslint-plugin-jsx-a11y` (frontend)

| Field        | Value |
|--------------|-------|
| **ID**       | GHSA-mh99-v99m-4gvg (CVSS 7.5) |
| **Severity** | High |
| **Status**   | Accepted Risk (no fixed version exists) |

**What**
`brace-expansion` DoS via unbounded expansion length causing an out-of-memory process crash.
Reachable through `frontend/`'s real `eslint-plugin-jsx-a11y` dependency on
`minimatch@3.1.5` → `brace-expansion@1.1.16`. This is a separate chain from the
`@types/eslint-plugin-jsx-a11y` regression fixed in `e95bd277` (that one was a types-only
package that briefly picked up a real `eslint@^9` dependency; this one is the actual lint
plugin's own long-standing dependency).

**Who**

- Discovered by: `npm audit` (local `scripts/dep_update.sh` runs and CI)
- Reported: 2026-07-25 (alongside the related `@types` fix)
- Affects: `frontend/` devDependencies only — never shipped to users

**Where**

- Component: `node_modules/eslint-plugin-jsx-a11y/node_modules/{minimatch,brace-expansion}`
- Versions affected: `eslint-plugin-jsx-a11y` `>=6.5.0` through the current latest, `6.10.2` —
  every version depends on `minimatch: ^3.1.2`. The only version without this dependency is
  `6.4.1`.

**When**

- Discovered: 2026-07-25
- Disclosed (if public): Public
- Target fix: N/A until `eslint-plugin-jsx-a11y` ships a release that drops or bumps its
  `minimatch` dependency

**How**
`npm audit fix --force` offers to downgrade to `6.4.1`, a multi-year regression in a11y lint
coverage — rejected as unacceptable. Forcing the patched `brace-expansion@5.0.8` via `overrides`
breaks `minimatch@3.1.5`'s callable-default `require('brace-expansion')(pattern)` usage (5.0.8
changed the CJS export to a named-export object); forcing `minimatch` itself to the 10.x line
breaks `eslint-plugin-jsx-a11y`'s compiled `_interopRequireDefault(...).default` usage instead
(minimatch@10.x's CJS build has no `.default` export). Both verified by direct `require()`
experiments, not just version inspection. Devdependency-only, never shipped, and `minimatch`
here only processes trusted local source identifiers during linting, not untrusted input.

**Planned Remediation**
Monitor for an upstream `eslint-plugin-jsx-a11y` release that resolves the `minimatch`
dependency; re-check on every dependency update pass. Until then, allowlisted precisely (exact
advisory ID + full dependency path, not by severity level) in `frontend/audit-ci.json`, run via
`npm run audit:ci` from both `scripts/dep_update.sh` and CI's `quality-checks.yml` — this does
not suppress any other high/critical finding, only this exact chain.

---

## Patched Vulnerabilities

### ✅ [LOW] GO-2026-5024 / CVE-2026-39824 · golang.org/x/sys in gosu Build Stage

| Field        | Value |
|--------------|-------|
| **ID**       | GO-2026-5024 / CVE-2026-39824 |
| **Severity** | Low · N/A (no CVSS published) |
| **Patched**  | 2026-07-16 |

**What**
Grype/GitHub code scanning flagged `golang.org/x/sys` embedded in `/usr/sbin/gosu`, at version
v0.13.0 — well below the v0.44.0 fix floor for the vulnerable `NewNTUnicodeString` string-length
overflow. This version is not a Charon-authored dependency; it is vendored by upstream
`tianon/gosu@1.17`'s own `go.mod`/`go.sum` and only surfaces because `gosu` is built from source
inside Charon's Docker image (to avoid separate CVEs in Debian's precompiled `gosu` binary).

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-07-16

**Where**

- Component: `golang.org/x/sys` (vendored via upstream `tianon/gosu@1.17`'s `go.sum`)
- Versions affected: `gosu-builder` Dockerfile stage prior to this fix (resolved v0.13.0)

**When**

- Discovered: 2026-07-16
- Patched: 2026-07-16
- Time to patch: 0 days

**How**
The vulnerable function, `NewNTUnicodeString`, lives entirely in `golang.org/x/sys/windows`, which
is excluded from compilation by Go's `GOOS` build constraints on every target this stage builds.
`gosu` is a Unix-only tool, and the `gosu-builder` stage only ever cross-compiles Linux targets
(`CGO_ENABLED=0`). The flagged code path is never compiled into the shipped `gosu` binary —
real-world exploitability is effectively zero. This is a distinct finding from the CVE-2026-39824
entry already resolved for `backend/go.mod`'s indirect dependency (see
[vulnerability-analysis-2026-06-26.md](docs/security/vulnerability-analysis-2026-06-26.md), which
carries a 2026-07-16 erratum correcting its original "scanner false positive" conclusion): that
finding lives in Charon's own application module graph and was fixed by a transitive upgrade; this
one lives in upstream `gosu`'s vendored `go.sum` inside a Docker build stage and required its own
explicit pin.

**Resolution**
Added `go get golang.org/x/sys@v0.46.0 && go mod tidy && go mod verify` to the `gosu-builder`
Dockerfile stage, matching the same `v0.46.0` pin already used by the Delve (`dlv`) debug-binary
stage for this identical advisory. `GOSU_VERSION` was intentionally not bumped — upstream tag
`1.19`'s `go.mod` regresses to an older `golang.org/x/sys v0.1.0`, which would make the finding
worse. Full analysis: [vulnerability-analysis-2026-07-16.md](docs/security/vulnerability-analysis-2026-07-16.md).

---

### ✅ [HIGH] CVE-2026-34040 · Docker AuthZ Plugin Bypass via Oversized Request Body

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-34040 (GHSA-x744-4wpc-v9h2) |
| **Severity** | High · 8.8 |
| **Patched**  | 2026-04-21 |

**What**
Docker Engine AuthZ plugins can be bypassed when an API request body exceeds a
certain size threshold. The previous Charon backend dependency path was
`github.com/docker/docker`.

**Who**

- Discovered by: Automated scan (govulncheck, Grype)
- Reported: 2026-04-04

**Where**

- Previous component: `github.com/docker/docker` v28.5.2+incompatible (Docker client SDK)
- Remediated component path: `github.com/moby/moby/client` with `github.com/moby/moby/api`

**When**

- Discovered: 2026-04-04
- Patched: 2026-04-21
- Time to patch: 17 days

**How**
The backend Docker service imports and module dependencies were migrated away from
the vulnerable monolith package path to modular Moby dependencies.

**Resolution**
Validation evidence after remediation:

- Backend: `go mod tidy`, `go test ./...`, and `go build ./cmd/api` passed.
- Trivy gate output did not include `CVE-2026-34040` or `GHSA-x744-4wpc-v9h2`.
- Docker image scan gate reported `0 Critical` and `0 High`, and did not include
  `CVE-2026-34040` or `GHSA-x744-4wpc-v9h2`.

---

### ✅ [LOW] CVE-2026-26958 · edwards25519 MultiScalarMult Invalid Results

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-26958 (GHSA-fw7p-63qq-7hpr) |
| **Severity** | Low · 1.7 |
| **Patched**  | 2026-04-04 |

**What**
`filippo.io/edwards25519` v1.1.0 `MultiScalarMult` produces invalid results or undefined
behavior if the receiver is not the identity point. Fix available at v1.1.1 but requires
CrowdSec to rebuild.

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-03-24

**Where**

- Component: CrowdSec Agent (bundled `cscli` and `crowdsec` binaries)
- Versions affected: CrowdSec builds using `filippo.io/edwards25519` < v1.1.1

**When**

- Discovered: 2026-03-24
- Patched: 2026-04-04
- Time to patch: 11 days

**How**
This is a rarely used advanced API within the edwards25519 library. CrowdSec does not directly
expose MultiScalarMult to external input. EPSS score is 0.00018 (0.04 percentile).

**Resolution**
Dependency no longer present in Charon's dependency tree. CrowdSec binaries no longer bundle
affected version.

---

### ✅ [CRITICAL] CVE-2025-68121 · Go Stdlib Critical in CrowdSec Bundled Binaries

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2025-68121 (see also CHARON-2025-001) |
| **Severity** | Critical |
| **Patched**  | 2026-03-24 |

**What**
A critical Go standard library vulnerability affects CrowdSec binaries bundled in the Charon
container image. The binaries were compiled against Go 1.25.6, which contains this flaw.
Charon's own application code, compiled with Go 1.26.1, is unaffected.

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-03-20

**Where**

- Component: CrowdSec Agent (bundled `cscli` and `crowdsec` binaries)
- Versions affected: Charon container images with CrowdSec binaries compiled against Go < 1.25.7

**When**

- Discovered: 2026-03-20
- Patched: 2026-03-24
- Time to patch: 4 days

**How**
The vulnerability resides entirely within CrowdSec's compiled binary artifacts. Exploitation
is limited to the CrowdSec agent's internal execution paths, which are not externally exposed
through Charon's API or network interface.

**Resolution**
CrowdSec binaries now compiled with Go 1.26.1 (was 1.25.6).

---

### ✅ [HIGH] CHARON-2025-001 · CrowdSec Bundled Binaries — Go Stdlib CVEs

| Field        | Value |
|--------------|-------|
| **ID**       | CHARON-2025-001 (aliases: CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729, CVE-2026-25679, CVE-2025-61732, CVE-2026-27142, CVE-2026-27139) |
| **Severity** | High · (preliminary, CVSS scores pending upstream confirmation) |
| **Patched**  | 2026-03-24 |

**What**
Multiple CVEs in Go standard library packages continue to accumulate in CrowdSec binaries bundled
with Charon. The cluster originated when CrowdSec was compiled against Go 1.25.1; subsequent
CrowdSec updates advanced the toolchain to Go 1.25.6/1.25.7, resolving earlier CVEs but
introducing new ones. The cluster now includes a Critical-severity finding (CVE-2025-68121,
tracked separately above). All issues resolve when CrowdSec is rebuilt against Go ≥ 1.26.2.
Charon's own application code is unaffected.

**Who**

- Discovered by: Automated scan (Trivy, Grype)
- Reported: 2025-12-01 (original cluster); expanded 2026-03-20

**Where**

- Component: CrowdSec Agent (bundled `cscli` and `crowdsec` binaries)
- Versions affected: All Charon versions shipping CrowdSec binaries compiled against Go < 1.26.2

**When**

- Discovered: 2025-12-01
- Patched: 2026-03-24
- Time to patch: 114 days

**How**
The CVEs reside entirely within CrowdSec's compiled binaries and cover HTTP/2, TLS, and archive
processing paths that are not invoked by Charon's core application logic. The relevant network
interfaces are not externally exposed via Charon's API surface.

**Resolution**
CrowdSec binaries now compiled with Go 1.26.1.

---

### ✅ [MEDIUM] CVE-2026-27171 · zlib CPU Exhaustion via Infinite Loop in CRC Combine Functions

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-27171 |
| **Severity** | Medium · 5.5 (NVD) / 2.9 (MITRE) |
| **Patched**  | 2026-03-24 |

**What**
zlib before 1.3.2 allows unbounded CPU consumption (denial of service) via the `crc32_combine64`
and `crc32_combine_gen64` functions. An internal helper `x2nmodp` performs right-shifts inside a
loop with no termination condition when given a specially crafted input, causing a CPU spin
(CWE-1284).

**Who**

- Discovered by: 7aSecurity audit (commissioned by OSTIF)
- Reported: 2026-02-17

**Where**

- Component: Alpine 3.23.3 base image (`zlib` package, version 1.3.1-r2)
- Versions affected: zlib < 1.3.2; all current Charon images using Alpine 3.23.3

**When**

- Discovered: 2026-02-17
- Patched: 2026-03-24
- Time to patch: 35 days

**How**
Exploitation requires local access (CVSS vector `AV:L`) and the ability to pass a crafted value
to the `crc32_combine`-family functions. This code path is not invoked by Charon's reverse proxy
or backend API. The vulnerability is non-blocking under the project's CI severity policy.

**Resolution**
Alpine now ships zlib 1.3.2-r0 (fix threshold was 1.3.2).

---

### ✅ [HIGH] CHARON-2026-001 · Debian Base Image CVE Cluster

| Field        | Value |
|--------------|-------|
| **ID**       | CHARON-2026-001 (aliases: CVE-2026-0861, CVE-2025-15281, CVE-2026-0915, CVE-2025-13151, and 2 libtiff HIGH CVEs) |
| **Severity** | High · 8.4 (highest per CVSS v3.1) |
| **Patched**  | 2026-03-20 (Alpine base image migration complete) |

**What**
Seven HIGH-severity CVEs in Debian Trixie base image system libraries (`glibc`, `libtasn1-6`,
`libtiff`). These vulnerabilities resided in the container's OS-level packages with no fixes
available from the Debian Security Team.

**Who**

- Discovered by: Automated scan (Trivy)
- Reported: 2026-02-04

**Where**

- Component: Debian Trixie base image (`libc6`, `libc-bin`, `libtasn1-6`, `libtiff`)
- Versions affected: Charon container images built on Debian Trixie base (prior to Alpine migration)

**When**

- Discovered: 2026-02-04
- Patched: 2026-03-20
- Time to patch: 44 days

**How**
The affected packages were OS-level shared libraries bundled in the Debian Trixie container base
image. Exploitation would have required local container access or a prior application-level
compromise. Caddy reverse proxy ingress filtering and container isolation significantly reduced
the effective attack surface throughout the exposure window.

**Resolution**
Reverted to Alpine Linux base image (Alpine 3.23.3). Alpine's patch of CVE-2025-60876 (busybox
heap overflow) removed the original blocker for the Alpine migration. Post-migration scan
confirmed zero HIGH/CRITICAL CVEs from this cluster.

- Spec: [docs/plans/alpine_migration_spec.md](docs/plans/alpine_migration_spec.md)
- Advisory: [docs/security/advisory_2026-02-04_debian_cves_temporary.md](docs/security/advisory_2026-02-04_debian_cves_temporary.md)

**Credit**
Internal remediation; no external reporter.

---

### ✅ [HIGH] CVE-2025-68156 · expr-lang/expr ReDoS

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2025-68156 |
| **Severity** | High · 7.5 |
| **Patched**  | 2026-01-11 |

**What**
Regular Expression Denial of Service (ReDoS) vulnerability in the `expr-lang/expr` library used
by CrowdSec for expression evaluation. Malicious regular expressions in CrowdSec scenarios or
parsers could cause CPU exhaustion and service degradation through exponential backtracking.

**Who**

- Discovered by: Automated scan (Trivy)
- Reported: 2026-01-11

**Where**

- Component: CrowdSec (via `expr-lang/expr` dependency)
- Versions affected: CrowdSec versions using `expr-lang/expr` < v1.17.7

**When**

- Discovered: 2026-01-11
- Patched: 2026-01-11
- Time to patch: 0 days

**How**
Maliciously crafted regular expressions in CrowdSec scenario or parser rules could trigger
exponential backtracking in `expr-lang/expr`'s evaluation engine, causing CPU exhaustion and
denial of service. The vulnerability is in the upstream expression evaluation library, not in
Charon's own code.

**Resolution**
Upgraded CrowdSec to build from source with the patched `expr-lang/expr` v1.17.7. Verification
confirmed via `go version -m ./cscli` showing the patched library version in compiled artifacts.
Post-patch Trivy scan reports 0 HIGH/CRITICAL vulnerabilities in application code.

- Technical details: [docs/plans/crowdsec_source_build.md](docs/plans/crowdsec_source_build.md)

**Credit**
Internal remediation; no external reporter.

---

## Security Features

### Server-Side Request Forgery (SSRF) Protection

Charon implements industry-leading **5-layer defense-in-depth** SSRF protection to prevent
attackers from using the application to access internal resources or cloud metadata.

#### Protected Against

- **Private network access** (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- **Cloud provider metadata endpoints** (AWS, Azure, GCP: 169.254.169.254)
- **Localhost and loopback addresses** (127.0.0.0/8, ::1/128)
- **Link-local addresses** (169.254.0.0/16, fe80::/10)
- **IPv6-mapped IPv4 bypass attempts** (::ffff:127.0.0.1)
- **Protocol bypass attacks** (file://, ftp://, gopher://, data:)

#### Defense Layers

1. **URL Format Validation**: Scheme, syntax, and structure checks
2. **DNS Resolution**: Hostname resolution with timeout protection
3. **IP Range Validation**: ALL resolved IPs checked against 13+ CIDR blocks
4. **Connection-Time Validation**: Re-validation at TCP dial (prevents DNS rebinding)
5. **Redirect Validation**: Each redirect target validated before following

#### Protected Features

- Security notification webhooks
- Custom webhook notifications
- CrowdSec hub synchronization
- External URL connectivity testing (admin-only)

#### Learn More

- [SSRF Protection Guide](docs/security/ssrf-protection.md)
- [Manual Test Plan](docs/issues/ssrf-manual-test-plan.md)
- [QA Audit Report](docs/reports/qa_ssrf_remediation_report.md)

---

### Authentication & Authorization

- **JWT-based authentication**: Secure token-based sessions
- **Role-based access control**: Admin vs. user permissions
- **Session management**: Automatic expiration and renewal
- **Secure cookie attributes**: HttpOnly, Secure (HTTPS), SameSite

### Data Protection

- **Database encryption**: Sensitive data encrypted at rest
- **Secure credential storage**: Hashed passwords, encrypted API keys
- **Input validation**: All user inputs sanitized and validated
- **Output encoding**: XSS protection via proper encoding

### Infrastructure Security

- **Non-root by default**: Charon runs as an unprivileged user (`charon`, uid 1000) inside the
  container. Docker socket access is granted via a minimal supplemental group matching the host
  socket's GID — never by running as root. If the socket GID is `0` (root group), Charon requires
  explicit opt-in before granting access.
- **Container isolation**: Docker-based deployment
- **Minimal attack surface**: Alpine Linux base image
- **Dependency scanning**: Regular Trivy and govulncheck scans
- **No unnecessary services**: Single-purpose container design

### Web Application Firewall (WAF)

- **Coraza WAF integration**: OWASP Core Rule Set support
- **Rate limiting**: Protection against brute-force and DoS
- **IP allowlisting/blocklisting**: Network access control
- **CrowdSec integration**: Collaborative threat intelligence

---

## Supply Chain Security

Charon implements comprehensive supply chain security measures to ensure the integrity and
authenticity of releases. Every release includes cryptographic signatures, SLSA provenance
attestation, and a Software Bill of Materials (SBOM).

### Verification Commands

#### Verify Container Image Signature

All official Charon images are signed with Sigstore Cosign:

```bash
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:latest
```

Successful verification confirms the image was built by GitHub Actions from the official
repository and has not been tampered with since signing.

#### Verify SLSA Provenance

```bash
# Download provenance from release assets
curl -LO https://github.com/Wikid82/charon/releases/latest/download/provenance.json

slsa-verifier verify-artifact \
  --provenance-path provenance.json \
  --source-uri github.com/Wikid82/charon \
  ./backend/charon-binary
```

#### Inspect the SBOM

```bash
# Download SBOM from release assets
curl -LO https://github.com/Wikid82/charon/releases/latest/download/sbom.spdx.json

# Scan for known vulnerabilities
grype sbom:sbom.spdx.json
```

### Transparency Log (Rekor)

All signatures are recorded in the public Sigstore Rekor transparency log:
<https://search.sigstore.dev/>

### Digest Pinning Policy

**Scope (Required):**

- CI workflows: `.github/workflows/*.yml`
- CI compose files: `.docker/compose/*.yml`
- CI helper actions with container refs: `.github/actions/**/*.yml`

CI workflows and CI compose files MUST use digest-pinned images for third-party services.
Tag+digest pairs are preferred for human-readable references with immutable resolution.
Self-built images MUST propagate digests to downstream jobs and tests.

**Local Development Exceptions:**

Local-only overrides (e.g., `CHARON_E2E_IMAGE`, `CHARON_IMAGE`, `CHARON_DEV_IMAGE`) MAY use tags
for developer iteration. Tag-only overrides MUST NOT be used in CI contexts.

**Documented Exceptions & Compensating Controls:**

1. **Go toolchain shim** (`golang.org/dl/goX.Y.Z@latest`) — Uses `@latest` to install the shim;
   compensated by the target toolchain version being pinned in `go.work` with Renovate tracking.
2. **Unpinnable dependencies** — Require documented justification; prefer vendor checksums or
   signed releases; keep SBOM/vulnerability scans in CI.

### Learn More

- [User Guide](docs/guides/supply-chain-security-user-guide.md)
- [Developer Guide](docs/guides/supply-chain-security-developer-guide.md)
- [Sigstore Documentation](https://docs.sigstore.dev/)
- [SLSA Framework](https://slsa.dev/)

---

## Security Audits & Scanning

### Automated Scanning

| Tool | Purpose |
|------|---------|
| Trivy | Container image vulnerability scanning |
| CodeQL | Static analysis for Go and JavaScript |
| Semgrep | Static analysis for security anti-patterns (Go, JS/TS, React, secrets, Dockerfile) |
| govulncheck | Go module vulnerability scanning |
| golangci-lint (gosec) | Go code linting |
| npm audit | Frontend dependency scanning |

### Scanning Workflows

**Docker Build & Scan** (`.github/workflows/docker-build.yml`) — runs on every commit to `main`,
`development`, and `feature/beta-release`, and on all PRs targeting those branches. Performs Trivy
scanning, generates an SBOM, creates SBOM attestations, and uploads SARIF results to the GitHub
Security tab.

**Supply Chain Verification** (`.github/workflows/supply-chain-verify.yml`) — triggers
automatically via `workflow_run` after a successful docker-build. Runs SBOM completeness checks,
Grype vulnerability scans, and (on releases) Cosign signature and SLSA provenance validation.

**Weekly Security Rebuild** (`.github/workflows/security-weekly-rebuild.yml`) — runs every Sunday
at 02:00 UTC. Performs a full no-cache rebuild, scans for all severity levels, and retains JSON
artifacts for 90 days.

**PR-Specific Scanning** — extracts and scans only the Charon application binary on each pull
request. Fails the PR if CRITICAL or HIGH vulnerabilities are found in application code.

**Semgrep SAST Scan** (`.github/workflows/semgrep.yml`) — runs on every push and pull request to
`main`, `nightly`, and `development`, on manual dispatch, and weekly on Mondays at 04:00 UTC. Scans
the full repository inside a pinned `semgrep/semgrep` container using the `p/golang`,
`p/javascript`, `p/typescript`, `p/react`, `p/secrets`, and `p/dockerfile` rulesets — the same
rule configs, exclusions, and ERROR/WARNING severity gate developers already run locally via
`scripts/pre-commit-hooks/semgrep-scan.sh`. Uploads SARIF results to the GitHub Security tab and
fails the build on any blocking finding.

### Manual Reviews

- Security code reviews for all major features
- Peer review of security-sensitive changes
- Third-party security audits (planned)

---

## Security Best Practices

### Deployment Recommendations

1. **Use HTTPS**: Always deploy behind a reverse proxy with TLS
2. **Restrict Admin Access**: Limit admin panel to trusted IPs
3. **Regular Updates**: Keep Charon and dependencies up to date
4. **Secure Webhooks**: Only use trusted webhook endpoints
5. **Strong Passwords**: Enforce password complexity policies
6. **Backup Encryption**: Encrypt backup files before storage

### Configuration Hardening

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    restart: unless-stopped
    environment:
      - CHARON_ENV=production
      - LOG_LEVEL=info
    volumes:
      - ./charon-data:/app/data:rw
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - charon-internal
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp:noexec,nosuid,nodev
```

### Gotify Token Hygiene

Gotify application tokens are secrets and must be handled with strict confidentiality.

- Never echo, print, log, or return token values in API responses or errors.
- Never expose tokenized endpoint query strings (e.g., `...?token=...`) in logs, diagnostics,
  examples, screenshots, tickets, or reports.
- Always redact query parameters in diagnostics and examples before display or storage.
- Use write-only token inputs in operator workflows and UI forms.
- Store tokens only in environment variables or a dedicated secret manager.
- Validate Gotify endpoints over HTTPS only.
- Rotate tokens immediately on suspected exposure.

### Network Security

- **Firewall Rules**: Only expose necessary ports (80, 443, 8080)
- **VPN Access**: Use VPN for admin access in production
- **Fail2Ban**: Consider fail2ban for brute-force protection
- **Intrusion Detection**: Enable CrowdSec for threat detection

---

## Security Hall of Fame

We recognize security researchers who help improve Charon:

<!-- Add contributors here -->
- *Your name could be here!*

---

**Last Updated**: 2026-05-18
