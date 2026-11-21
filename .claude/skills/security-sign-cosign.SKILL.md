````markdown
---
# agentskills.io specification v1.0
name: "security-sign-cosign"
version: "1.0.0"
description: "Sign Docker images and artifacts with Cosign (Sigstore) for supply chain security"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "signing"
  - "cosign"
  - "supply-chain"
  - "sigstore"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "cosign"
    version: ">=2.4.0"
    optional: false
    install_url: "https://github.com/sigstore/cosign"
  - name: "docker"
    version: ">=24.0"
    optional: true
    description: "Required only for Docker image signing"
environment_variables:
  - name: "COSIGN_EXPERIMENTAL"
    description: "Enable keyless signing (OIDC)"
    default: "1"
    required: false
  - name: "COSIGN_YES"
    description: "Non-interactive mode"
    default: "true"
    required: false
  - name: "COSIGN_PRIVATE_KEY"
    description: "Base64-encoded private key for key-based signing"
    default: ""
    required: false
  - name: "COSIGN_PASSWORD"
    description: "Password for private key"
    default: ""
    required: false
parameters:
  - name: "type"
    type: "string"
    description: "Artifact type (docker, file)"
    required: false
    default: "docker"
  - name: "target"
    type: "string"
    description: "Docker image tag or file path"
    required: true
outputs:
  - name: "signature"
    type: "file"
    description: "Signature file (.sig for files, registry for images)"
  - name: "certificate"
    type: "file"
    description: "Certificate file (.pem for files)"
  - name: "exit_code"
    type: "number"
    description: "0 if signing succeeded, non-zero otherwise"
metadata:
  category: "security"
  subcategory: "supply-chain"
  execution_time: "fast"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: false
exit_codes:
  0: "Signing successful"
  1: "Signing failed"
  2: "Missing dependencies or invalid parameters"
---

# Security: Sign with Cosign

Sign Docker images and files using Cosign (Sigstore) for supply chain security and artifact integrity verification.

## Overview

This skill signs Docker images and arbitrary files using Cosign, creating cryptographic signatures that can be verified by consumers. It supports both keyless signing (using GitHub OIDC tokens in CI/CD) and key-based signing (using local private keys for development).

Signatures are stored in Rekor transparency log for public accountability and can be verified without sharing private keys.

## Features

- Sign Docker images (stored in registry)
- Sign arbitrary files (binaries, archives, etc.)
- Keyless signing with GitHub OIDC (CI/CD)
- Key-based signing with local keys (development)
- Automatic verification after signing
- Rekor transparency log integration
- Non-interactive mode for automation

## Prerequisites

- Cosign 2.4.0 or higher
- Docker (for image signing)
- GitHub account (for keyless signing with OIDC)
- Or: Local key pair (for key-based signing)

## Usage

### Sign Docker Image (Keyless - CI/CD)

In GitHub Actions or environments with OIDC:

```bash
# Keyless signing (uses GitHub OIDC token)
COSIGN_EXPERIMENTAL=1 .github/skills/scripts/skill-runner.sh \
  security-sign-cosign docker ghcr.io/user/charon:latest
```

### Sign Docker Image (Key-Based - Local Development)

For local development with generated keys:

```bash
# Generate key pair first (if you don't have one)
# cosign generate-key-pair
# Enter password when prompted

# Sign with local key
COSIGN_EXPERIMENTAL=0 COSIGN_PRIVATE_KEY="$(cat cosign.key)" \
  COSIGN_PASSWORD="your-password" \
  .github/skills/scripts/skill-runner.sh \
  security-sign-cosign docker charon:local
```

### Sign File (Binary, Archive, etc.)

```bash
# Sign a file (creates .sig and .pem files)
.github/skills/scripts/skill-runner.sh \
  security-sign-cosign file ./dist/charon-linux-amd64
```

### Verify Signature

```bash
# Verify Docker image (keyless)
cosign verify ghcr.io/user/charon:latest \
  --certificate-identity-regexp="https://github.com/user/repo" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Verify file (key-based)
cosign verify-blob ./dist/charon-linux-amd64 \
  --signature ./dist/charon-linux-amd64.sig \
  --certificate ./dist/charon-linux-amd64.pem \
  --certificate-identity-regexp="https://github.com/user/repo" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| type      | string | No     | docker  | Artifact type (docker, file) |
| target    | string | Yes    | -       | Docker image tag or file path |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| COSIGN_EXPERIMENTAL | No | 1 | Enable keyless signing (1=keyless, 0=key-based) |
| COSIGN_YES | No | true | Non-interactive mode |
| COSIGN_PRIVATE_KEY | No | "" | Base64-encoded private key (for key-based signing) |
| COSIGN_PASSWORD | No | "" | Password for private key |

## Signing Modes

### Keyless Signing (Recommended for CI/CD)

- Uses GitHub OIDC tokens for authentication
- No long-lived keys to manage or secure
- Signatures stored in Rekor transparency log
- Certificates issued by Fulcio CA
- Requires GitHub Actions or similar OIDC provider

**Pros**:
- No key management burden
- Public transparency and auditability
- Automatic certificate rotation
- Secure by default

**Cons**:
- Requires network access
- Depends on Sigstore infrastructure
- Not suitable for air-gapped environments

### Key-Based Signing (Local Development)

- Uses local private key files
- Keys managed by developer
- Suitable for air-gapped environments
- Requires secure key storage

**Pros**:
- Works offline
- Full control over keys
- No external dependencies

**Cons**:
- Key management complexity
- Risk of key compromise
- Manual key rotation
- No public transparency log

## Outputs

### Docker Image Signing
- Signature pushed to registry (no local file)
- Rekor transparency log entry
- Certificate (ephemeral for keyless)

### File Signing
- `<filename>.sig`: Signature file
- `<filename>.pem`: Certificate file (for keyless)
- Rekor transparency log entry (for keyless)

## Examples

### Example 1: Sign Local Docker Image (Development)

```bash
$ docker build -t charon:test .
$ COSIGN_EXPERIMENTAL=0 \
  COSIGN_PRIVATE_KEY="$(cat ~/.cosign/cosign.key)" \
  COSIGN_PASSWORD="my-secure-password" \
  .github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:test

[INFO] Signing Docker image: charon:test
[COSIGN] Using key-based signing (COSIGN_EXPERIMENTAL=0)
[COSIGN] Signing image...
[SUCCESS] Image signed successfully
[INFO] Signature pushed to registry
[INFO] Verification command:
  cosign verify charon:test --key cosign.pub
```

### Example 2: Sign Release Binary (Keyless)

```bash
$ .github/skills/scripts/skill-runner.sh \
  security-sign-cosign file ./dist/charon-linux-amd64

[INFO] Signing file: ./dist/charon-linux-amd64
[COSIGN] Using keyless signing (GitHub OIDC)
[COSIGN] Generating ephemeral certificate...
[COSIGN] Signing with Fulcio certificate...
[SUCCESS] File signed successfully
[INFO] Signature: ./dist/charon-linux-amd64.sig
[INFO] Certificate: ./dist/charon-linux-amd64.pem
[INFO] Rekor entry: https://rekor.sigstore.dev/...
```

### Example 3: CI/CD Pipeline (GitHub Actions)

```yaml
- name: Install Cosign
  uses: sigstore/cosign-installer@v3.8.1
  with:
    cosign-release: 'v2.4.1'

- name: Sign Docker Image
  env:
    DIGEST: ${{ steps.build-and-push.outputs.digest }}
    IMAGE: ghcr.io/${{ github.repository }}
  run: |
    cosign sign --yes ${IMAGE}@${DIGEST}

- name: Verify Signature
  run: |
    cosign verify ghcr.io/${{ github.repository }}@${DIGEST} \
      --certificate-identity-regexp="https://github.com/${{ github.repository }}" \
      --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

### Example 4: Batch Sign Release Artifacts

```bash
# Sign all binaries in dist/ directory
for artifact in ./dist/charon-*; do
  if [[ -f "$artifact" && ! "$artifact" == *.sig && ! "$artifact" == *.pem ]]; then
    echo "Signing: $(basename $artifact)"
    .github/skills/scripts/skill-runner.sh security-sign-cosign file "$artifact"
  fi
done
```

## Key Management Best Practices

### Generating Keys

```bash
# Generate a new key pair
cosign generate-key-pair

# This creates:
# - cosign.key (private key - keep secure!)
# - cosign.pub (public key - share freely)
```

### Storing Keys Securely

**DO**:
- Store private keys in password manager or HSM
- Encrypt private keys with strong passwords
- Rotate keys periodically (every 90 days)
- Use different keys for different environments
- Backup keys securely (encrypted backups)

**DON'T**:
- Commit private keys to version control
- Store keys in plaintext files
- Share private keys via email or chat
- Use the same key for CI/CD and local development
- Hardcode passwords in scripts

### Key Rotation

```bash
# Generate new key pair
cosign generate-key-pair --output-key-prefix cosign-new

# Sign new artifacts with new key
COSIGN_PRIVATE_KEY="$(cat cosign-new.key)" ...

# Update public key in documentation
# Revoke old key after transition period
```

## Error Handling

### Common Issues

**Cosign not installed**:
```bash
Error: cosign command not found
Solution: Install Cosign from https://github.com/sigstore/cosign
Quick install: go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

**Missing OIDC token (keyless)**:
```bash
Error: OIDC token not available
Solution: Run in GitHub Actions or use key-based signing (COSIGN_EXPERIMENTAL=0)
```

**Invalid private key**:
```bash
Error: Failed to decrypt private key
Solution: Verify COSIGN_PASSWORD is correct and key file is valid
```

**Docker image not found**:
```bash
Error: Image not found: charon:test
Solution: Build or pull the image first
```

**Registry authentication failed**:
```bash
Error: Failed to push signature to registry
Solution: Authenticate with: docker login <registry>
```

### Rekor Outages

If Rekor is unavailable, signing will fail. Fallback options:

1. **Wait and retry**: Rekor usually recovers quickly
2. **Use key-based signing**: Doesn't require Rekor
3. **Sign without Rekor**: `cosign sign --insecure-ignore-tlog` (not recommended)

## Exit Codes

- **0**: Signing successful
- **1**: Signing failed
- **2**: Missing dependencies or invalid parameters

## Related Skills

- [security-verify-sbom](./security-verify-sbom.SKILL.md) - Verify SBOM and scan vulnerabilities
- [security-slsa-provenance](./security-slsa-provenance.SKILL.md) - Generate SLSA provenance

## Notes

- Keyless signing is recommended for CI/CD pipelines
- Key-based signing is suitable for local development and air-gapped environments
- All signatures are public and verifiable
- Rekor transparency log provides audit trail
- Docker image signatures are stored in the registry, not locally
- File signatures are stored as `.sig` files alongside the original
- Certificates for keyless signing are ephemeral and stored with the signature

## Security Considerations

- **Never commit private keys to version control**
- Use strong passwords for private keys (20+ characters)
- Rotate keys regularly (every 90 days recommended)
- Verify signatures before trusting artifacts
- Monitor Rekor logs for unauthorized signatures
- Use different keys for different trust levels
- Consider using HSM for production keys
- Enable MFA on accounts with signing privileges

---

**Last Updated**: 2026-01-10
**Maintained by**: Charon Project
**Source**: Cosign (Sigstore)
**Documentation**: https://docs.sigstore.dev/cosign/overview/

````
