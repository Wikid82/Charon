# Supply Chain Security - Developer Guide

## Overview

This guide explains how to use Charon's supply chain security tools during development, testing, and release preparation. It covers the three agent skills, when to use them, and how they integrate into your workflow.

---

## Table of Contents

1. [Quick Reference](#quick-reference)
2. [Agent Skills Overview](#agent-skills-overview)
3. [Development Workflow](#development-workflow)
4. [Testing and Validation](#testing-and-validation)
5. [Release Process](#release-process)
6. [Troubleshooting](#troubleshooting)

---

## Quick Reference

### Available VS Code Tasks

```bash
# Verify SBOM and scan for vulnerabilities
Task: "Security: Verify SBOM"

# Sign a container image with Cosign
Task: "Security: Sign with Cosign"

# Generate SLSA provenance for a binary
Task: "Security: Generate SLSA Provenance"

# Run all supply chain checks
Task: "Security: Full Supply Chain Audit"
```

### Direct Skill Invocation

```bash
# From project root
.github/skills/scripts/skill-runner.sh security-verify-sbom [image]
.github/skills/scripts/skill-runner.sh security-sign-cosign [type] [target]
.github/skills/scripts/skill-runner.sh security-slsa-provenance [action] [target]
```

---

## Agent Skills Overview

### 1. security-verify-sbom

**Purpose:** Verify SBOM contents and scan for vulnerabilities

**Usage:**

```bash
# Verify container image SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:local

# Verify directory SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom dir ./backend

# Verify file SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom file ./backend/main
```

**What it does:**

1. Generates SBOM using Syft (if not exists)
2. Validates SBOM format (SPDX JSON)
3. Scans for vulnerabilities using Grype
4. Reports findings with severity levels

**When to use:**

- Before committing dependency updates
- After building new images
- Before releases
- During security audits

**Output:**

- SBOM file (SPDX JSON format)
- Vulnerability report
- Summary of critical/high findings

### 2. security-sign-cosign

**Purpose:** Sign container images or binaries with Cosign

**Usage:**

```bash
# Sign Docker image
.github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:local

# Sign binary file
.github/skills/scripts/skill-runner.sh security-sign-cosign file ./backend/main

# Sign OCI artifact
.github/skills/scripts/skill-runner.sh security-sign-cosign oci ghcr.io/wikid82/charon:latest
```

**What it does:**

1. Verifies target exists
2. Signs with Cosign (keyless or with key)
3. Records signature in Rekor transparency log
4. Generates verification commands

**When to use:**

- After building local test images
- Before pushing to registry
- During release preparation
- For artifact attestation

**Requirements:**

- Cosign installed (`make install-cosign`)
- Docker running (for image signing)
- Network access (for Rekor)

### 3. security-slsa-provenance

**Purpose:** Generate and verify SLSA provenance attestation

**Usage:**

```bash
# Generate provenance for binary
.github/skills/scripts/skill-runner.sh security-slsa-provenance generate ./backend/main

# Verify provenance
.github/skills/scripts/skill-runner.sh security-slsa-provenance verify ./backend/main provenance.json

# Validate provenance format
.github/skills/scripts/skill-runner.sh security-slsa-provenance validate provenance.json
```

**What it does:**

1. Collects build metadata (commit, branch, timestamp)
2. Generates SLSA provenance document
3. Signs provenance with Cosign
4. Verifies provenance integrity

**When to use:**

- After building release binaries
- Before publishing releases
- For compliance requirements
- To prove build reproducibility

**Output:**

- `provenance.json` - SLSA provenance attestation
- Verification status
- Build metadata

---

## Development Workflow

### Daily Development

#### 1. Dependency Updates

When updating dependencies:

```bash
# 1. Update dependencies
cd backend && go get -u ./...
cd ../frontend && npm update

# 2. Build and test
make build-all
make test-all

# 3. Verify SBOM (check for new vulnerabilities)
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:local
```

**Review output:**

- ✅ No critical/high vulnerabilities → Proceed
- ⚠️ Vulnerabilities found → Review, patch, or document

#### 2. Local Testing

Before committing:

```bash
# 1. Build local image
docker build -t charon:dev .

# 2. Generate and verify SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:dev

# 3. Sign image (optional, for testing)
.github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:dev
```

#### 3. Pre-Commit Checks

Add to your pre-commit routine:

```bash
# .git/hooks/pre-commit (or pre-commit config)
#!/bin/bash
set -e

echo "🔍 Running supply chain checks..."

# Build
make build-all

# Verify SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom dir ./backend

# Check for critical vulnerabilities
if grep -i "critical" sbom-scan-output.txt; then
    echo "❌ Critical vulnerabilities found! Review before committing."
    exit 1
fi

echo "✅ Supply chain checks passed"
```

### Pull Request Workflow

#### As a Developer

```bash
# 1. Build and test locally
make build-all
make test-all

# 2. Run full supply chain audit
# (Uses the composite VS Code task)
# Run via VS Code: Ctrl+Shift+P → "Tasks: Run Task" → "Security: Full Supply Chain Audit"

# 3. Document findings in PR description
# - SBOM changes (new dependencies)
# - Vulnerability scan results
# - Signature verification status
```

#### As a Reviewer

Verify supply chain artifacts:

```bash
# 1. Checkout PR branch
git fetch origin pull/123/head:pr-123
git checkout pr-123

# 2. Build
make build-all

# 3. Verify SBOM
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:local

# 4. Check for regressions
# - New vulnerabilities introduced?
# - Unexpected dependency changes?
# - SBOM completeness?
```

**Review checklist:**

- [ ] SBOM includes all new dependencies
- [ ] No new critical/high vulnerabilities
- [ ] Dependency licenses compatible
- [ ] Security documentation updated

---

## Testing and Validation

### Unit Testing Supply Chain Skills

```bash
# Test SBOM generation
.github/skills/scripts/skill-runner.sh security-verify-sbom dir ./backend
test -f sbom.spdx.json || echo "❌ SBOM not generated"

# Test signing (requires Cosign)
docker build -t charon:test .
.github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:test
echo $? # Should be 0 for success

# Test provenance generation
go build -o main ./backend/cmd/charon
.github/skills/scripts/skill-runner.sh security-slsa-provenance generate ./main
test -f provenance.json || echo "❌ Provenance not generated"
```

### Integration Testing

Create a test script:

```bash
#!/bin/bash
# test-supply-chain.sh
set -e

echo "🔧 Building test image..."
docker build -t charon:integration-test .

echo "🔍 Verifying SBOM..."
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:integration-test

echo "✍️ Signing image..."
.github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:integration-test

echo "🔐 Verifying signature..."
cosign verify \
  --certificate-identity-regexp='.*' \
  --certificate-oidc-issuer='.*' \
  charon:integration-test || echo "⚠️ Verification expected to fail for local image"

echo "📄 Generating provenance..."
.github/skills/scripts/skill-runner.sh security-slsa-provenance generate ./backend/main

echo "✅ All supply chain tests passed!"
```

Run in CI/CD:

```yaml
# .github/workflows/test.yml
- name: Test Supply Chain
  run: |
    chmod +x test-supply-chain.sh
    ./test-supply-chain.sh
```

### Validation Checklist

Before marking a feature complete:

- [ ] SBOM generation works for all artifacts
- [ ] Signing works for images and binaries
- [ ] Provenance generation includes correct metadata
- [ ] Verification commands documented
- [ ] CI/CD integration tested
- [ ] Error handling validated
- [ ] Documentation updated

---

## Release Process

### Pre-Release Checklist

#### 1. Version Bump and Tag

```bash
# Update version
echo "v1.0.0" > VERSION

# Commit and tag
git add VERSION
git commit -m "chore: bump version to v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
```

#### 2. Build Release Artifacts

```bash
# Build backend binary
cd backend
go build -ldflags="-s -w -X main.Version=v1.0.0" -o charon-linux-amd64 ./cmd/charon

# Build frontend
cd ../frontend
npm run build

# Build Docker image
cd ..
docker build -t charon:v1.0.0 .
```

#### 3. Generate Supply Chain Artifacts

```bash
# Generate SBOM for image
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:v1.0.0
mv sbom.spdx.json sbom-v1.0.0.spdx.json

# Generate SBOM for binary
.github/skills/scripts/skill-runner.sh security-verify-sbom file ./backend/charon-linux-amd64
mv sbom.spdx.json sbom-binary-v1.0.0.spdx.json

# Generate provenance for binary
.github/skills/scripts/skill-runner.sh security-slsa-provenance generate ./backend/charon-linux-amd64
mv provenance.json provenance-v1.0.0.json

# Sign binary
.github/skills/scripts/skill-runner.sh security-sign-cosign file ./backend/charon-linux-amd64
```

#### 4. Push and Sign Image

```bash
# Tag image for registry
docker tag charon:v1.0.0 ghcr.io/wikid82/charon:v1.0.0
docker tag charon:v1.0.0 ghcr.io/wikid82/charon:latest

# Push to registry
docker push ghcr.io/wikid82/charon:v1.0.0
docker push ghcr.io/wikid82/charon:latest

# Sign images
.github/skills/scripts/skill-runner.sh security-sign-cosign oci ghcr.io/wikid82/charon:v1.0.0
.github/skills/scripts/skill-runner.sh security-sign-cosign oci ghcr.io/wikid82/charon:latest
```

#### 5. Verify Release Artifacts

```bash
# Verify image signature
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:v1.0.0

# Verify provenance
slsa-verifier verify-artifact \
  --provenance-path provenance-v1.0.0.json \
  --source-uri github.com/Wikid82/charon \
  ./backend/charon-linux-amd64

# Scan SBOM
grype sbom:sbom-v1.0.0.spdx.json
```

#### 6. Create GitHub Release

Upload these files as release assets:

- `charon-linux-amd64` - Binary
- `charon-linux-amd64.sig` - Binary signature
- `sbom-v1.0.0.spdx.json` - Image SBOM
- `sbom-binary-v1.0.0.spdx.json` - Binary SBOM
- `provenance-v1.0.0.json` - SLSA provenance

Release notes should include:

- Verification commands
- Link to user guide
- Known vulnerabilities (if any)

### Automated Release (GitHub Actions)

The release process is automated via GitHub Actions. The workflow:

1. Triggers on version tags (`v*`)
2. Builds artifacts
3. Generates SBOMs and provenance
4. Signs with Cosign (keyless)
5. Pushes to registry
6. Creates GitHub release with assets

See `.github/workflows/release.yml` for implementation.

---

## Troubleshooting

### Common Issues

#### "syft: command not found"

**Solution:**

```bash
make install-syft
# Or manually:
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
```

#### "cosign: command not found"

**Solution:**

```bash
make install-cosign
# Or manually:
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
sudo chmod +x /usr/local/bin/cosign
```

#### "grype: command not found"

**Solution:**

```bash
make install-grype
# Or manually:
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
```

#### SBOM Generation Fails

**Possible causes:**

- Docker image doesn't exist
- Directory/file path incorrect
- Syft version incompatible

**Debug:**

```bash
# Check image exists
docker images | grep charon

# Test Syft manually
syft docker:charon:local -o spdx-json

# Check Syft version
syft version
```

#### Signing Fails with "no ambient OIDC credentials"

**Cause:** Cosign keyless signing requires OIDC authentication (GitHub Actions, Google Cloud, etc.)

**Solutions:**

1. Use key-based signing for local development:

   ```bash
   cosign generate-key-pair
   cosign sign --key cosign.key charon:local
   ```

2. Set up OIDC provider (GitHub Actions example):

   ```yaml
   permissions:
     id-token: write
     packages: write
   ```

3. Use environment variables:

   ```bash
   export COSIGN_EXPERIMENTAL=1
   ```

#### Provenance Verification Fails

**Possible causes:**

- Provenance file doesn't match binary
- Binary was modified after provenance generation
- Wrong source URI

**Debug:**

```bash
# Check binary hash
sha256sum ./backend/charon-linux-amd64

# Check hash in provenance
cat provenance.json | jq -r '.subject[0].digest.sha256'

# Hashes should match
```

### Performance Optimization

#### SBOM Generation is Slow

**Optimization:**

```bash
# Cache SBOM between runs
SBOM_FILE="sbom-$(git rev-parse --short HEAD).spdx.json"
if [ ! -f "$SBOM_FILE" ]; then
    syft docker:charon:local -o spdx-json > "$SBOM_FILE"
fi
```

#### Large Image Scans Timeout

**Solution:**

```bash
# Increase timeout
export GRYPE_CHECK_FOR_APP_UPDATE=false
export GRYPE_DB_AUTO_UPDATE=false
grype --timeout 10m docker:charon:local
```

### Debugging

Enable verbose logging:

```bash
# For skill scripts
export SKILL_DEBUG=1
.github/skills/scripts/skill-runner.sh security-verify-sbom docker charon:local

# For Syft
export SYFT_LOG_LEVEL=debug
syft docker:charon:local

# For Cosign
export COSIGN_LOG_LEVEL=debug
cosign sign charon:local

# For Grype
export GRYPE_LOG_LEVEL=debug
grype docker:charon:local
```

---

## Best Practices

### Security

1. **Never commit private keys**: Use keyless signing or store keys securely
2. **Verify before sign**: Always verify artifacts before signing
3. **Use specific versions**: Pin tool versions in CI/CD
4. **Rotate keys regularly**: If using key-based signing
5. **Monitor transparency logs**: Check Rekor for unexpected signatures

### Development

1. **Generate SBOM early**: Run during development, not just before release
2. **Automate verification**: Add to CI/CD and pre-commit hooks
3. **Document vulnerabilities**: Track known issues in SECURITY.md
4. **Test locally**: Verify skills work on developer machines
5. **Update dependencies**: Keep tools (Syft, Cosign, Grype) current

### CI/CD

1. **Cache tools**: Cache tool installations between runs
2. **Parallel execution**: Run SBOM generation and signing in parallel
3. **Fail fast**: Exit early on critical vulnerabilities
4. **Artifact retention**: Store SBOMs and provenance as artifacts
5. **Release automation**: Fully automate release signing and verification

---

## Additional Resources

### Documentation

- [User Guide](supply-chain-security-user-guide.md) - End-user verification
- [SECURITY.md](../../SECURITY.md) - Security policy and contacts
- [Skill Implementation](../.github/skills/security-supply-chain/) - Skill source code

### External Resources

- [Sigstore Documentation](https://docs.sigstore.dev/)
- [SLSA Framework](https://slsa.dev/)
- [Syft Documentation](https://github.com/anchore/syft)
- [Grype Documentation](https://github.com/anchore/grype)
- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview/)

### Tools

- [Sigstore Rekor Search](https://search.sigstore.dev/)
- [SPDX Online Tools](https://tools.spdx.org/)
- [Supply Chain Security Best Practices](https://slsa.dev/spec/v1.0/requirements)

---

## Support

### Getting Help

- **Questions**: [GitHub Discussions](https://github.com/Wikid82/charon/discussions)
- **Bug Reports**: [GitHub Issues](https://github.com/Wikid82/charon/issues)
- **Security**: [Security Advisory](https://github.com/Wikid82/charon/security/advisories)

### Contributing

Found a bug or want to improve the supply chain security implementation?

1. Open an issue describing the problem
2. Submit a PR with fixes/improvements
3. Update tests and documentation
4. Run full supply chain audit before submitting

---

**Last Updated**: January 10, 2026
**Version**: 1.0
