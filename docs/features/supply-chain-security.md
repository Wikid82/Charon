---
title: Verified Builds
description: Cryptographic signatures, SLSA provenance, and SBOM for every release
---

# Verified Builds

Know exactly what you're running. Every Charon release includes cryptographic signatures, SLSA provenance attestation, and a Software Bill of Materials (SBOM). Enterprise-grade supply chain security for everyone.

## Overview

Supply chain attacks are increasingly common. Charon protects you with multiple verification layers that prove the image you're running was built from the official source code, hasn't been tampered with, and contains no hidden dependencies.

### Security Artifacts

| Artifact | Purpose | Standard |
|----------|---------|----------|
| **Cosign Signature** | Cryptographic proof of origin | Sigstore |
| **SLSA Provenance** | Build process attestation | SLSA Level 3 |
| **SBOM** | Complete dependency inventory | SPDX/CycloneDX |

## Why Supply Chain Security Matters

| Threat | Mitigation |
|--------|------------|
| **Compromised CI/CD** | SLSA provenance verifies build source |
| **Malicious maintainer** | Signatures require private key access |
| **Dependency hijacking** | SBOM enables vulnerability scanning |
| **Registry tampering** | Signatures detect unauthorized changes |
| **Audit requirements** | Complete traceability for compliance |

## Verifying Image Signatures

### Prerequisites

```bash
# Install Cosign
# macOS
brew install cosign

# Linux
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
chmod +x cosign-linux-amd64 && sudo mv cosign-linux-amd64 /usr/local/bin/cosign
```

### Verify a Charon Image

```bash
# Verify signature (keyless - uses Sigstore public transparency log)
cosign verify ghcr.io/wikid82/charon:latest \
  --certificate-identity-regexp='https://github.com/Wikid82/charon/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com'

# Successful output shows:
# Verification for ghcr.io/wikid82/charon:latest --
# The following checks were performed on each of these signatures:
#   - The cosign claims were validated
#   - The signatures were verified against the specified public key
```

### Verify SLSA Provenance

```bash
# Install slsa-verifier
go install github.com/slsa-framework/slsa-verifier/v2/cli/slsa-verifier@latest

# Verify provenance attestation
slsa-verifier verify-image ghcr.io/wikid82/charon:latest \
  --source-uri github.com/Wikid82/charon \
  --source-tag v2.0.0
```

## Software Bill of Materials (SBOM)

### What's Included

The SBOM lists every component in the image:

- Go modules and versions
- System packages (Alpine)
- Frontend npm dependencies
- Build tools used

### Retrieving the SBOM

```bash
# Download SBOM attestation
cosign download sbom ghcr.io/wikid82/charon:latest > charon-sbom.spdx.json

# View in human-readable format
cat charon-sbom.spdx.json | jq '.packages[] | {name, version}'
```

### Vulnerability Scanning

Use the SBOM with vulnerability scanners:

```bash
# Scan with Trivy
trivy sbom charon-sbom.spdx.json

# Scan with Grype
grype sbom:charon-sbom.spdx.json
```

## SLSA Provenance Details

SLSA (Supply-chain Levels for Software Artifacts) provenance includes:

| Field | Content |
|-------|---------|
| `buildType` | GitHub Actions workflow |
| `invocation` | Commit SHA, branch, workflow run |
| `materials` | Source repository, dependencies |
| `builder` | GitHub-hosted runner details |

### Example Provenance

```json
{
  "buildType": "https://github.com/slsa-framework/slsa-github-generator",
  "invocation": {
    "configSource": {
      "uri": "git+https://github.com/Wikid82/charon@refs/tags/v2.0.0",
      "entryPoint": ".github/workflows/release.yml"
    }
  },
  "materials": [{
    "uri": "git+https://github.com/Wikid82/charon",
    "digest": {"sha1": "abc123..."}
  }]
}
```

## Enterprise Compliance

These artifacts support compliance requirements:

- **SOC 2**: Demonstrates secure build practices
- **FedRAMP**: Provides software supply chain documentation
- **PCI DSS**: Enables change management auditing
- **NIST SSDF**: Aligns with secure development framework

## Related

- [Security Hardening](security-hardening.md) - Runtime security features
- [Coraza WAF](coraza-waf.md) - Application firewall
- [Back to Features](../features.md)
