# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in Charon, please report it responsibly.

### Where to Report

**Preferred Method**: GitHub Security Advisory (Private)

1. Go to <https://github.com/Wikid82/charon/security/advisories/new>
2. Fill out the advisory form with:
   - Vulnerability description
   - Steps to reproduce
   - Proof of concept (non-destructive)
   - Impact assessment
   - Suggested fix (if applicable)

**Alternative Method**: Email

- Send to: `security@charon.dev` (if configured)
- Use PGP encryption (key available below, if applicable)
- Include same information as GitHub advisory

### What to Include

Please provide:

1. **Description**: Clear explanation of the vulnerability
2. **Reproduction Steps**: Detailed steps to reproduce the issue
3. **Impact Assessment**: What an attacker could do with this vulnerability
4. **Environment**: Charon version, deployment method, OS, etc.
5. **Proof of Concept**: Code or commands demonstrating the vulnerability (non-destructive)
6. **Suggested Fix**: If you have ideas for remediation

### What Happens Next

1. **Acknowledgment**: We'll acknowledge your report within **48 hours**
2. **Investigation**: We'll investigate and assess the severity
3. **Updates**: We'll provide regular status updates (weekly minimum)
4. **Fix Development**: We'll develop and test a fix
5. **Disclosure**: Coordinated disclosure after fix is released
6. **Credit**: We'll credit you in release notes (if desired)

### Responsible Disclosure

We ask that you:

- ✅ Give us reasonable time to fix the issue before public disclosure (90 days preferred)
- ✅ Avoid destructive testing or attacks on production systems
- ✅ Not access, modify, or delete data that doesn't belong to you
- ✅ Not perform actions that could degrade service for others

We commit to:

- ✅ Respond to your report within 48 hours
- ✅ Provide regular status updates
- ✅ Credit you in release notes (if desired)
- ✅ Not pursue legal action for good-faith security research

---

## Security Features

### Server-Side Request Forgery (SSRF) Protection

Charon implements industry-leading **5-layer defense-in-depth** SSRF protection to prevent attackers from using the application to access internal resources or cloud metadata.

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

For complete technical details, see:

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
# Recommended docker-compose.yml settings
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    restart: unless-stopped
    environment:
      - CHARON_ENV=production
      - LOG_LEVEL=info  # Don't use debug in production
    volumes:
      - ./charon-data:/app/data:rw
      - /var/run/docker.sock:/var/run/docker.sock:ro  # Read-only!
    networks:
      - charon-internal  # Isolated network
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Only if binding to ports < 1024
    security_opt:
      - no-new-privileges:true
    read_only: true  # If possible
    tmpfs:
      - /tmp:noexec,nosuid,nodev
```

### Network Security

- **Firewall Rules**: Only expose necessary ports (80, 443, 8080)
- **VPN Access**: Use VPN for admin access in production
- **Fail2Ban**: Consider fail2ban for brute-force protection
- **Intrusion Detection**: Enable CrowdSec for threat detection

---

## Supply Chain Security

Charon implements comprehensive supply chain security measures to ensure the integrity and authenticity of releases. Every release includes cryptographic signatures, SLSA provenance attestation, and Software Bill of Materials (SBOM).

### Verification Commands

#### Verify Container Image Signature

All official Charon images are signed with Sigstore Cosign:

```bash
# Install cosign (if not already installed)
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
sudo chmod +x /usr/local/bin/cosign

# Verify image signature
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:latest
```

Successful verification output confirms:

- The image was built by GitHub Actions
- The build came from the official Charon repository
- The image has not been tampered with since signing

#### Verify SLSA Provenance

SLSA (Supply-chain Levels for Software Artifacts) provenance provides tamper-proof evidence of how the software was built:

```bash
# Install slsa-verifier (if not already installed)
curl -LO https://github.com/slsa-framework/slsa-verifier/releases/latest/download/slsa-verifier-linux-amd64
sudo mv slsa-verifier-linux-amd64 /usr/local/bin/slsa-verifier
sudo chmod +x /usr/local/bin/slsa-verifier

# Download provenance from release assets
curl -LO https://github.com/Wikid82/charon/releases/latest/download/provenance.json

# Verify provenance
slsa-verifier verify-artifact \
  --provenance-path provenance.json \
  --source-uri github.com/Wikid82/charon \
  ./backend/charon-binary
```

#### Inspect Software Bill of Materials (SBOM)

Every release includes a comprehensive SBOM in SPDX format:

```bash
# Download SBOM from release assets
curl -LO https://github.com/Wikid82/charon/releases/latest/download/sbom.spdx.json

# View SBOM contents
cat sbom.spdx.json | jq .

# Check for known vulnerabilities (requires Grype)
grype sbom:sbom.spdx.json
```

### Transparency Log (Rekor)

All signatures are recorded in the public Sigstore Rekor transparency log, providing an immutable audit trail:

- **Search the log**: <https://search.sigstore.dev/>
- **Query by image**: Search for `ghcr.io/wikid82/charon`
- **View entry details**: Each entry includes commit SHA, workflow run, and signing timestamp

### Automated Verification in CI/CD

Integrate supply chain verification into your deployment pipeline:

```yaml
# Example GitHub Actions workflow
- name: Verify Charon Image
  run: |
    cosign verify \
      --certificate-identity-regexp='https://github.com/Wikid82/charon' \
      --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
      ghcr.io/wikid82/charon:${{ env.VERSION }}
```

### What's Protected

- **Container Images**: All `ghcr.io/wikid82/charon:*` images are signed
- **Release Binaries**: Backend binaries include provenance attestation
- **Build Process**: SLSA Level 3 compliant build provenance
- **Dependencies**: Complete SBOM including all direct and transitive dependencies

### Digest Pinning Policy

Charon uses digest pinning to reduce supply chain risk and ensure CI runs against immutable artifacts.

**Scope (Required):**

- **CI workflows**: `.github/workflows/*.yml`, `.github/workflows/*.yaml`
- **CI compose files**: `.docker/compose/*.yml`, `.docker/compose/*.yaml`, `.docker/compose/docker-compose*.yml`, `.docker/compose/docker-compose*.yaml`
- **CI helper actions with container refs**: `.github/actions/**/*.yml`, `.github/actions/**/*.yaml`
- CI workflows and CI compose files MUST use digest-pinned images for third-party services.
- Tag+digest pairs are preferred for human-readable references with immutable resolution.
- Self-built images MUST propagate digests to downstream jobs and tests.

**Rationale:**

- Prevent tag drift and supply chain substitution in automated runs.
- Ensure deterministic builds, reproducible scans, and stable SBOM generation.
- Reduce rollback risk by guaranteeing CI uses immutable artifacts.

**Local Development Exceptions:**

- Local-only overrides (e.g., `CHARON_E2E_IMAGE`, `CHARON_IMAGE`, `CHARON_DEV_IMAGE`) MAY use tags for developer iteration.
- Tag-only overrides MUST NOT be used in CI contexts.

**Documented Exceptions & Compensating Controls:**

1. **Go toolchain shim** (`golang.org/dl/goX.Y.Z@latest`)
  - **Exception:** Uses `@latest` to install the shim.
  - **Compensating controls:** The target toolchain version is pinned in `go.work`, and Renovate tracks the required version for updates.
2. **Unpinnable dependencies** (no stable digest or checksum source)
  - **Exception:** Dependency cannot be pinned by digest.
  - **Compensating controls:** Require documented justification, prefer vendor-provided checksums or signed releases when available, and keep SBOM/vulnerability scans in CI.

### Learn More

- **[User Guide](docs/guides/supply-chain-security-user-guide.md)**: Step-by-step verification instructions
- **[Developer Guide](docs/guides/supply-chain-security-developer-guide.md)**: Integration into development workflow
- **[Sigstore Documentation](https://docs.sigstore.dev/)**: Technical details on signing and verification
- **[SLSA Framework](https://slsa.dev/)**: Supply chain security framework overview

---

## Security Audits & Scanning

### Automated Scanning

We use the following tools:

- **Trivy**: Container image vulnerability scanning
- **CodeQL**: Static code analysis for Go and JavaScript
- **govulncheck**: Go module vulnerability scanning
- **golangci-lint**: Go code linting (including gosec)
- **npm audit**: Frontend dependency vulnerability scanning

### Security Scanning Workflows

Charon implements multiple layers of automated security scanning:

#### Docker Build & Scan (Per-Commit)

**Workflow**: `.github/workflows/docker-build.yml`

- Runs on every commit to `main`, `development`, and `feature/beta-release` branches
- Runs on all pull requests targeting these branches
- Performs Trivy vulnerability scanning on built images
- Generates SBOM (Software Bill of Materials) for supply chain transparency
- Creates SBOM attestations for verifiable build provenance
- Verifies Caddy security patches (CVE-2025-68156)
- Uploads SARIF results to GitHub Security tab

**Note**: This workflow replaced the previous `docker-publish.yml` (deleted Dec 21, 2025) with enhanced security features.

#### Supply Chain Verification

**Workflow**: `.github/workflows/supply-chain-verify.yml`

**Trigger Timing**: Runs automatically after `docker-build.yml` completes successfully via `workflow_run` trigger.

**Branch Coverage**: Triggers on **ALL branches** where docker-build completes, including:

- `main` (default branch)
- `development`
- `feature/*` branches (including `feature/beta-release`)
- Pull request branches

**Why No Branch Filter**: GitHub Actions has a platform limitation where `branches` filters in `workflow_run` triggers only match the default branch. To ensure comprehensive supply chain verification across all branches and PRs, we intentionally omit the branch filter. The workflow file must exist on the branch to execute, preventing untrusted code execution.

**Verification Steps**:

1. SBOM completeness verification
2. Vulnerability scanning with Grype
3. Results uploaded as workflow artifacts
4. PR comments with vulnerability summary (when applicable)
5. For releases: Cosign signature verification and SLSA provenance validation

**Additional Triggers**:

- Runs on all published releases
- Scheduled weekly on Mondays at 00:00 UTC
- Can be triggered manually via `workflow_dispatch`

#### Weekly Security Rebuild

**Workflow**: `.github/workflows/security-weekly-rebuild.yml`

- Runs every Sunday at 02:00 UTC
- Performs full rebuild with no cache to ensure latest base images
- Scans with Trivy for CRITICAL, HIGH, MEDIUM, and LOW vulnerabilities
- Uploads results to GitHub Security tab
- Stores JSON artifacts for 90-day retention
- Checks Alpine package versions for security updates

#### PR-Specific Scanning

**Workflow**: `.github/workflows/docker-build.yml` (trivy-pr-app-only job)

- Runs on all pull requests
- Extracts and scans only the Charon application binary
- Fails PR if CRITICAL or HIGH vulnerabilities found in application code
- Faster feedback loop for developers during code review

### Workflow Orchestration

The security scanning workflows use a coordinated orchestration pattern:

1. **Build Phase**: `docker-build.yml` builds the image and performs initial Trivy scan
2. **Verification Phase**: `supply-chain-verify.yml` triggers automatically via `workflow_run` after successful build
3. **Verification Timing**:
   - On feature branches: Runs after docker-build completes on push events
   - On pull requests: Runs after docker-build completes on PR synchronize events
   - No delay or gaps: verification starts immediately after build success
4. **Weekly Maintenance**: `security-weekly-rebuild.yml` provides ongoing monitoring

This pattern ensures:

- Images are built before verification attempts to scan them
- No race conditions between build and verification
- Comprehensive coverage across all branches and PRs
- Efficient resource usage (verification only runs after successful builds)

### Manual Reviews

- Security code reviews for all major features
- Peer review of security-sensitive changes
- Third-party security audits (planned)

### Continuous Monitoring

- GitHub Dependabot alerts
- Weekly security scans in CI/CD
- Community vulnerability reports
- Automated supply chain verification on every build

---

## Recently Resolved Vulnerabilities

Charon maintains transparency about security issues and their resolution. Below is a comprehensive record of recently patched vulnerabilities.

### CVE-2025-68156 (expr-lang/expr ReDoS)

- **Severity**: HIGH (CVSS 7.5)
- **Component**: expr-lang/expr (used by CrowdSec for expression evaluation)
- **Vulnerability**: Regular Expression Denial of Service (ReDoS)
- **Description**: Malicious regular expressions in CrowdSec scenarios or parsers could cause CPU exhaustion and service degradation through exponential backtracking in vulnerable regex patterns.
- **Fixed Version**: expr-lang/expr v1.17.7
- **Resolution Date**: January 11, 2026
- **Remediation**: Upgraded CrowdSec to build from source with patched expr-lang/expr v1.17.7
- **Verification**:
  - Binary inspection: `go version -m ./cscli` confirms v1.17.7 in compiled artifacts
  - Container scan: Trivy reports 0 HIGH/CRITICAL vulnerabilities in application code
  - Runtime testing: CrowdSec scenarios and parsers load successfully with patched library
- **Impact**: No known exploits in Charon deployments; preventive upgrade completed
- **Status**: ✅ **PATCHED** — Verified in all release artifacts
- **Technical Details**: See [CrowdSec Source Build Documentation](docs/plans/crowdsec_source_build.md)

---

## Known Security Considerations

### Debian Base Image CVEs (2026-02-04) — TEMPORARY

**Status**: ⚠️ 7 HIGH severity CVEs in Debian Trixie base image. **Alpine migration in progress.**

**Background**: Migrated from Alpine → Debian due to CVE-2025-60876 (busybox heap overflow). Debian now has worse CVE posture with no fixes available. Reverting to Alpine as Alpine CVE-2025-60876 is now patched.

**Affected Packages**:
- **libc6/libc-bin** (glibc): CVE-2026-0861 (CVSS 8.4), CVE-2025-15281, CVE-2026-0915
- **libtasn1-6**: CVE-2025-13151 (CVSS 7.5)
- **libtiff**: 2 additional HIGH CVEs

**Fix Status**: ❌ No fixes available from Debian Security Team

**Risk Assessment**: 🟢 **LOW actual risk**
- CVEs affect system libraries, NOT Charon application code
- Container isolation limits exploit surface area
- No direct exploit paths identified in Charon's usage patterns
- Network ingress filtered through Caddy proxy

**Mitigation**: Alpine base image migration
- **Spec**: [`docs/plans/alpine_migration_spec.md`](docs/plans/alpine_migration_spec.md)
- **Security Advisory**: [`docs/security/advisory_2026-02-04_debian_cves_temporary.md`](docs/security/advisory_2026-02-04_debian_cves_temporary.md)
- **Timeline**: 2-3 weeks (target completion: March 5, 2026)
- **Expected Outcome**: 100% CVE reduction (7 HIGH → 0)

**Review Date**: 2026-02-11 (Phase 1 Alpine CVE verification)

**Details**: See [VULNERABILITY_ACCEPTANCE.md](docs/security/VULNERABILITY_ACCEPTANCE.md) for complete risk assessment and monitoring plan.

### Third-Party Dependencies

**CrowdSec Binaries**: As of December 2025, CrowdSec binaries shipped with Charon contain 4 HIGH-severity CVEs in Go stdlib (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729). These are upstream issues in Go 1.25.1 and will be resolved when CrowdSec releases binaries built with go 1.26.0+.

**Impact**: Low. These vulnerabilities are in CrowdSec's third-party binaries, not in Charon's application code. They affect HTTP/2, TLS certificate handling, and archive parsing—areas not directly exposed to attackers through Charon's interface.

**Mitigation**: Monitor CrowdSec releases for updated binaries. Charon's own application code has zero vulnerabilities.

---

## Security Hall of Fame

We recognize security researchers who help improve Charon:

<!-- Add contributors here -->
- *Your name could be here!*

---

## Security Contact

- **GitHub Security Advisories**: <https://github.com/Wikid82/charon/security/advisories>
- **GitHub Discussions**: <https://github.com/Wikid82/charon/discussions>
- **GitHub Issues** (non-security): <https://github.com/Wikid82/charon/issues>

---

## License

This security policy is part of the Charon project, licensed under the MIT License.

---

**Last Updated**: January 30, 2026
**Version**: 1.2
