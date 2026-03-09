# Security: Trivy Scan

Run Trivy filesystem scan for vulnerabilities in source code and dependencies.

## Command

```bash
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

## What It Scans

- Go module dependencies (`go.mod`)
- npm dependencies (`package.json`)
- Dockerfile configuration
- Source code files

## Important: Trivy vs Docker Image Scan

Trivy filesystem scan alone is **NOT sufficient**. Always also run the Docker image scan:

```bash
.github/skills/scripts/skill-runner.sh security-scan-docker-image
```

The Docker image scan catches additional vulnerabilities:
- Alpine package CVEs in base image
- Compiled binary vulnerabilities
- Multi-stage build artifacts
- Embedded dependencies only present post-build

## On Findings

All CRITICAL and HIGH findings must be addressed. See `/supply-chain-remediation` for the full remediation workflow.

For accepted risks, add to `.trivyignore`:
```yaml
CVE-2025-XXXXX # Accepted: [reason why it doesn't apply]
```

## Related

- `/security-scan-docker-image` — MANDATORY companion scan
- `/security-scan-codeql` — Static analysis
- `/security-scan-gorm` — GORM SQL security
- `/supply-chain-remediation` — Fix vulnerabilities
