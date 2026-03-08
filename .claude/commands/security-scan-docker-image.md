# Security: Docker Image Scan

Run a comprehensive security scan of the built Charon Docker image using Syft/Grype.

## Command

```bash
.github/skills/scripts/skill-runner.sh security-scan-docker-image
```

## Why This Scan Is MANDATORY

This scan catches vulnerabilities that Trivy filesystem scan **misses**:
- Alpine package CVEs in the base image
- Compiled Go binary vulnerabilities
- Embedded dependencies only present post-build
- Multi-stage build artifacts with known issues

**Always run BOTH** Trivy (`/security-scan-trivy`) AND Docker image scan. Compare results — the image scan is the more comprehensive source of truth.

## CI Alignment

Uses the same Syft/Grype versions as the `supply-chain-pr.yml` CI workflow, ensuring local results match CI results.

## Prerequisites

The Docker image must be built first:
```bash
docker build -t charon:local .
```

## On Findings

All CRITICAL and HIGH findings must be addressed. Use `/supply-chain-remediation` for the full remediation workflow.

## Related

- `/security-scan-trivy` — Filesystem scan (run first, then this)
- `/security-scan-codeql` — Static analysis
- `/supply-chain-remediation` — Fix vulnerabilities
