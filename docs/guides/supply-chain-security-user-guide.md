# Supply Chain Security - User Guide

## Overview

Charon implements comprehensive supply chain security measures to ensure you can verify the authenticity and integrity of every release. This guide shows you how to verify signatures, check build provenance, and inspect Software Bill of Materials (SBOM).

## Why Supply Chain Security Matters

When you download and run software, you're trusting that:

- The software came from the legitimate source
- It hasn't been tampered with during distribution
- The build process was secure and reproducible
- You know exactly what dependencies are included

Supply chain attacks are increasingly common. Charon's verification tools help you confirm what you're running is exactly what the developers built.

---

## Quick Start: Verify a Release

### Prerequisites

Install verification tools (one-time setup):

```bash
# Install Cosign (for signature verification)
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
sudo chmod +x /usr/local/bin/cosign

# Install slsa-verifier (for provenance verification)
curl -LO https://github.com/slsa-framework/slsa-verifier/releases/latest/download/slsa-verifier-linux-amd64
sudo mv slsa-verifier-linux-amd64 /usr/local/bin/slsa-verifier
sudo chmod +x /usr/local/bin/slsa-verifier

# Install Grype (optional, for SBOM vulnerability scanning)
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
```

### Verify Container Image (Recommended)

Verify the Charon container image before running it:

```bash
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:latest
```

**Expected Output:**

```
Verification for ghcr.io/wikid82/charon:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The code-signing certificate was verified using trusted certificate authority certificates
```

---

## Detailed Verification Steps

### 1. Verify Image Signature with Cosign

**What it does:** Confirms the image was signed by the Charon project and hasn't been modified.

**Command:**

```bash
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:v1.0.0
```

**What to check:**

- ✅ "Verification for ... --" message appears
- ✅ Certificate identity matches `https://github.com/Wikid82/charon`
- ✅ OIDC issuer is `https://token.actions.githubusercontent.com`
- ✅ No errors or warnings

**Troubleshooting:**

- **Error: "no matching signatures"** → The image may not be signed, or you have the wrong tag
- **Error: "certificate identity doesn't match"** → The image may be compromised or unofficial
- **Error: "OIDC issuer doesn't match"** → The signing process didn't use GitHub Actions

### 2. Verify SLSA Provenance

**What it does:** Proves the Docker images were built by the official GitHub Actions workflow from the official repository.

**Note:** Charon uses a Docker-only deployment model. SLSA provenance is attached to container images, not standalone binaries.

**For Docker images, provenance is automatically embedded.** You can inspect it using Cosign:

```bash
# View attestations attached to the image
cosign verify-attestation \
  --type slsaprovenance \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:v1.0.0 | jq -r '.payload' | base64 -d | jq
```

**Expected Output:**

```json
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://slsa.dev/provenance/v0.2",
  "subject": [...],
  "predicate": {
    "builder": {
      "id": "https://github.com/slsa-framework/slsa-github-generator/..."
    },
    "buildType": "https://github.com/slsa-framework/slsa-github-generator@v1",
    "invocation": {
      "configSource": {
        "uri": "git+https://github.com/Wikid82/charon@refs/tags/v1.0.0"
      }
    }
  }
}
```

**What to check:**

- ✅ `predicateType` is SLSA provenance
- ✅ `builder.id` references the official SLSA generator
- ✅ `configSource.uri` matches `github.com/Wikid82/charon`
- ✅ No errors during verification

**Troubleshooting:**

- **Error: "no matching attestations"** → The image may not have provenance attached
- **Error: "certificate identity doesn't match"** → The attestation came from an unofficial source
- **Error: "invalid provenance"** → The provenance may be corrupted

### 3. Inspect Software Bill of Materials (SBOM)

**What it does:** Shows all dependencies included in Charon, allowing you to check for known vulnerabilities.

**Step 1: Download SBOM**

```bash
curl -LO https://github.com/Wikid82/charon/releases/download/v1.0.0/sbom.spdx.json
```

**Step 2: View SBOM contents**

```bash
# Pretty-print the SBOM
cat sbom.spdx.json | jq .

# List all packages
cat sbom.spdx.json | jq -r '.packages[].name' | sort
```

**Step 3: Check for vulnerabilities**

```bash
# Requires Grype (see prerequisites)
grype sbom:sbom.spdx.json
```

**Expected Output:**

```
NAME                 INSTALLED           VULNERABILITY   SEVERITY
github.com/caddyserver/caddy/v2  v2.11.0    (no vulnerabilities found)
...
```

**What to check:**

- ✅ SBOM contains expected packages (Go modules, npm packages)
- ✅ Package versions match release notes
- ✅ No critical or high-severity vulnerabilities
- ⚠️ Known acceptable vulnerabilities are documented in SECURITY.md

**Troubleshooting:**

- **High/Critical vulnerabilities found** → Check SECURITY.md for known issues and mitigation status
- **SBOM format error** → Download may be corrupted, try again
- **Missing packages** → SBOM may be incomplete, report as an issue

---

## Verify in Your CI/CD Pipeline

Integrate verification into your deployment workflow:

### GitHub Actions Example

```yaml
name: Deploy Charon
on:
  push:
    branches: [main]

jobs:
  verify-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Verify Charon Image
        run: |
          cosign verify \
            --certificate-identity-regexp='https://github.com/Wikid82/charon' \
            --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
            ghcr.io/wikid82/charon:latest

      - name: Deploy
        if: success()
        run: |
          docker-compose pull
          docker-compose up -d
```

### Docker Compose with Pre-Pull Verification

```bash
#!/bin/bash
set -e

IMAGE="ghcr.io/wikid82/charon:latest"

echo "🔍 Verifying image signature..."
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  "$IMAGE"

echo "✅ Signature verified!"
echo "🚀 Pulling and starting Charon..."
docker-compose pull
docker-compose up -d

echo "✅ Charon started successfully"
```

---

## Transparency and Audit Trail

### Sigstore Rekor Transparency Log

All signatures are recorded in the public Rekor transparency log:

1. **Visit**: <https://search.sigstore.dev/>
2. **Search**: Enter `ghcr.io/wikid82/charon` or a specific tag
3. **View Entry**: Click on an entry to see:
   - Signing timestamp
   - Git commit SHA
   - GitHub Actions workflow run ID
   - Certificate details

**Why this matters:** The transparency log provides an immutable, public record of all signatures. If a compromise occurs, it can be detected by comparing signatures against the log.

### GitHub Release Assets

Each Docker image release includes embedded attestations:

- **Image Signatures** - Cosign signatures (keyless signing via Sigstore)
- **SLSA Provenance** - Build attestation proving the image was built by official GitHub Actions
- **SBOM** - Software Bill of Materials attached to the image

**View releases at**: <https://github.com/Wikid82/charon/releases>

**Note:** Charon uses a Docker-only deployment model. All artifacts are embedded in container images - no standalone binaries are distributed.

---

## Security Best Practices

### Before Deploying

1. ✅ Always verify signatures before first deployment
2. ✅ Check SBOM for known vulnerabilities
3. ✅ Verify provenance for critical environments
4. ✅ Pin to specific version tags (not `latest`)

### During Operations

1. ✅ Set up automated verification in CI/CD
2. ✅ Monitor SECURITY.md for vulnerability updates
3. ✅ Subscribe to GitHub release notifications
4. ✅ Re-verify after any manual image pulls

### For Production Environments

1. ✅ Require signature verification before deployment
2. ✅ Use admission controllers (e.g., Kyverno, OPA) to enforce verification
3. ✅ Maintain audit logs of verified deployments
4. ✅ Scan SBOM against private vulnerability databases

---

## Troubleshooting

### Common Issues

#### "cosign: command not found"

**Solution:** Install Cosign (see Prerequisites section)

#### "Error: no matching signatures"

**Possible causes:**

- Image tag doesn't exist
- Image was pulled before signing implementation
- Using an unofficial image source

**Solution:** Use official images from `ghcr.io/wikid82/charon` with tags v1.0.0 or later

#### "Error: certificate identity doesn't match"

**Possible causes:**

- Image is from an unofficial source
- Image may be compromised

**Solution:** Only use images from the official repository. Report suspicious images.

#### Grype shows vulnerabilities

**Solution:**

1. Check SECURITY.md for known issues
2. Review vulnerability severity and exploitability
3. Check if patches are available in newer releases
4. Report new vulnerabilities via GitHub Security Advisory

### Getting Help

- **Documentation**: [Developer Guide](supply-chain-security-developer-guide.md)
- **Security Issues**: <https://github.com/Wikid82/charon/security/advisories>
- **Questions**: <https://github.com/Wikid82/charon/discussions>
- **Bug Reports**: <https://github.com/Wikid82/charon/issues>

---

## Additional Resources

- **[Sigstore Documentation](https://docs.sigstore.dev/)** - Learn about keyless signing
- **[SLSA Framework](https://slsa.dev/)** - Supply chain security levels
- **[SPDX Specification](https://spdx.dev/)** - SBOM format details
- **[Rekor Transparency Log](https://docs.sigstore.dev/rekor/overview/)** - Audit trail documentation

---

**Last Updated**: January 10, 2026
**Version**: 1.0
