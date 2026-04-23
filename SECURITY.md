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

Last reviewed: 2026-04-21

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

### [HIGH] CVE-2026-2673 · OpenSSL TLS 1.3 Key Exchange Group Downgrade

| Field        | Value |
|--------------|-------|
| **ID**       | CVE-2026-2673 (affects `libcrypto3` and `libssl3`) |
| **Severity** | High · 7.5 |
| **Status**   | Awaiting Upstream |

**What**
An OpenSSL TLS 1.3 server may fail to negotiate the intended key exchange group when the
configuration includes the `DEFAULT` keyword, potentially allowing downgrade to weaker cipher
suites. Affects Alpine 3.23.3 packages `libcrypto3` and `libssl3` at version 3.5.5-r0.

**Who**

- Discovered by: Automated scan (Grype)
- Reported: 2026-03-20
- Affects: Container runtime environment; Caddy reverse proxy TLS negotiation could be affected
  if default key group configuration is used

**Where**

- Component: Alpine 3.23.3 base image (`libcrypto3` 3.5.5-r0, `libssl3` 3.5.5-r0)
- Versions affected: Alpine 3.23.3 prior to a patched `openssl` APK release

**When**

- Discovered: 2026-03-20
- Disclosed (if public): 2026-03-13 (OpenSSL advisory)
- Target fix: When Alpine Security publishes a patched `openssl` APK

**How**
When an OpenSSL TLS 1.3 server configuration uses the `DEFAULT` keyword for key exchange groups,
the negotiation logic may select a weaker group than intended. Charon's Caddy TLS configuration
does not use the `DEFAULT` keyword, which limits practical exploitability. The packages are
present in the base image regardless of Caddy's configuration.

**Planned Remediation**
Monitor <https://security.alpinelinux.org/vuln/CVE-2026-2673> for a patched Alpine APK. Once
available, update the pinned `ALPINE_IMAGE` digest in the Dockerfile, or add an explicit
`RUN apk upgrade --no-cache libcrypto3 libssl3` to the runtime stage.

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

## Patched Vulnerabilities

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

**Last Updated**: 2026-03-24
