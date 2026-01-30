# Local Key Management for Cosign Signing

## Overview

This guide provides comprehensive procedures for managing Cosign signing keys in local development environments. It covers key generation, secure storage, rotation, and air-gapped signing workflows.

**Audience**: Developers, DevOps engineers, Security team
**Last Updated**: 2026-01-10

## Table of Contents

1. [Key Generation](#key-generation)
2. [Secure Storage](#secure-storage)
3. [Key Usage](#key-usage)
4. [Key Rotation](#key-rotation)
5. [Backup and Recovery](#backup-and-recovery)
6. [Air-Gapped Signing](#air-gapped-signing)
7. [Troubleshooting](#troubleshooting)

---

## Key Generation

### Prerequisites

- Cosign 2.4.0 or higher installed
- Strong password (20+ characters, mixed case, numbers, special characters)
- Secure environment (trusted machine, no malware)

### Generate Key Pair

```bash
# Navigate to secure directory
cd ~/.cosign

# Generate key pair interactively
cosign generate-key-pair

# You will be prompted for a password
# Enter a strong password (minimum 20 characters recommended)

# This creates two files:
# - cosign.key (PRIVATE KEY - keep secure!)
# - cosign.pub (public key - share freely)
```

### Non-Interactive Generation (for automation)

```bash
# Generate with password from environment
export COSIGN_PASSWORD="your-strong-password"
cosign generate-key-pair --output-key-prefix=cosign-dev

# Cleanup environment variable
unset COSIGN_PASSWORD
```

### Key Naming Convention

Use descriptive prefixes for different environments:

```
cosign-dev.key      # Development environment
cosign-staging.key  # Staging environment
cosign-prod.key     # Production environment (use HSM if possible)
```

**⚠️ WARNING**: Never use the same key for multiple environments!

---

## Secure Storage

### File System Permissions

```bash
# Set restrictive permissions on private key
chmod 600 ~/.cosign/cosign.key

# Verify permissions
ls -l ~/.cosign/cosign.key
# Should show: -rw------- (only owner can read/write)
```

### Password Manager Integration

Store private keys in a password manager:

1. **1Password, Bitwarden, or LastPass**:
   - Create a secure note
   - Add the private key content
   - Add the password as a separate field
   - Tag as "cosign-key"

2. **Retrieve when needed**:

   ```bash
   # Example with op (1Password CLI)
   op read "op://Private/cosign-dev-key/private key" > /tmp/cosign.key
   chmod 600 /tmp/cosign.key

   # Use the key
   COSIGN_PRIVATE_KEY="$(cat /tmp/cosign.key)" \
   COSIGN_PASSWORD="$(op read 'op://Private/cosign-dev-key/password')" \
   cosign sign --key /tmp/cosign.key charon:local

   # Cleanup
   shred -u /tmp/cosign.key
   ```

### Hardware Security Module (HSM)

For production keys, use an HSM or YubiKey:

```bash
# Generate key on YubiKey
cosign generate-key-pair --key-slot 9a

# Sign with YubiKey
cosign sign --key yubikey://slot-id charon:latest
```

### Environment Variables (Development Only)

For development convenience:

```bash
# Add to ~/.bashrc or ~/.zshrc (NEVER commit this file!)
export COSIGN_PRIVATE_KEY="$(cat ~/.cosign/cosign-dev.key)"
export COSIGN_PASSWORD="your-dev-password"

# Source the file
source ~/.bashrc
```

**⚠️ WARNING**: Only use environment variables in trusted development environments!

---

## Key Usage

### Sign Docker Image

```bash
# Export private key and password
export COSIGN_PRIVATE_KEY="$(cat ~/.cosign/cosign-dev.key)"
export COSIGN_PASSWORD="your-password"

# Sign the image
cosign sign --yes --key <(echo "${COSIGN_PRIVATE_KEY}") charon:local

# Or use the Charon skill
.github/skills/scripts/skill-runner.sh security-sign-cosign docker charon:local

# Cleanup
unset COSIGN_PRIVATE_KEY
unset COSIGN_PASSWORD
```

### Sign Release Artifacts

```bash
# Sign a binary
cosign sign-blob --yes \
  --key ~/.cosign/cosign-prod.key \
  --output-signature ./dist/charon-linux-amd64.sig \
  ./dist/charon-linux-amd64

# Verify signature
cosign verify-blob ./dist/charon-linux-amd64 \
  --signature ./dist/charon-linux-amd64.sig \
  --key ~/.cosign/cosign-prod.pub
```

### Batch Signing

```bash
# Sign all artifacts in a directory
for artifact in ./dist/charon-*; do
  if [[ -f "$artifact" && ! "$artifact" == *.sig ]]; then
    echo "Signing: $(basename $artifact)"
    cosign sign-blob --yes \
      --key ~/.cosign/cosign-prod.key \
      --output-signature "${artifact}.sig" \
      "$artifact"
  fi
done
```

---

## Key Rotation

### When to Rotate

- **Every 90 days** (recommended)
- After any suspected compromise
- When team members with key access leave
- After security incidents
- Before major releases

### Rotation Procedure

1. **Generate new key pair**:

   ```bash
   cd ~/.cosign
   cosign generate-key-pair --output-key-prefix=cosign-prod-v2
   ```

2. **Test new key**:

   ```bash
   # Sign test artifact
   cosign sign-blob --yes \
     --key cosign-prod-v2.key \
     --output-signature test.sig \
     test-file

   # Verify
   cosign verify-blob test-file \
     --signature test.sig \
     --key cosign-prod-v2.pub

   # Cleanup test files
   rm test-file test.sig
   ```

3. **Update documentation**:
   - Update README with new public key
   - Update CI/CD secrets (if key-based signing)
   - Notify team members

4. **Transition period**:
   - Sign new artifacts with new key
   - Keep old key available for verification
   - Document transition date

5. **Retire old key**:
   - After 30-90 days (all old artifacts verified)
   - Archive old key securely (for historical verification)
   - Delete from active use

6. **Archive old key**:

   ```bash
   mkdir -p ~/.cosign/archive/$(date +%Y-%m)
   mv cosign-prod.key ~/.cosign/archive/$(date +%Y-%m)/
   chmod 400 ~/.cosign/archive/$(date +%Y-%m)/cosign-prod.key
   ```

---

## Backup and Recovery

### Backup Procedure

```bash
# Create encrypted backup
cd ~/.cosign
tar czf cosign-keys-backup.tar.gz cosign*.key cosign*.pub

# Encrypt with GPG
gpg --symmetric --cipher-algo AES256 cosign-keys-backup.tar.gz

# This creates: cosign-keys-backup.tar.gz.gpg

# Remove unencrypted backup
shred -u cosign-keys-backup.tar.gz

# Store encrypted backup in:
# - Password manager
# - Encrypted USB drive (stored in safe)
# - Encrypted cloud storage (e.g., Tresorit, ProtonDrive)
```

### Recovery Procedure

```bash
# Decrypt backup
gpg --decrypt cosign-keys-backup.tar.gz.gpg > cosign-keys-backup.tar.gz

# Extract keys
tar xzf cosign-keys-backup.tar.gz

# Set permissions
chmod 600 cosign*.key
chmod 644 cosign*.pub

# Verify keys work
cosign sign-blob --yes \
  --key cosign-dev.key \
  --output-signature test.sig \
  <(echo "test")

# Cleanup
rm cosign-keys-backup.tar.gz
shred -u test.sig
```

### Disaster Recovery

If private key is lost:

1. **Generate new key pair** (see Key Generation)
2. **Revoke old public key** (update documentation)
3. **Re-sign critical artifacts** with new key
4. **Notify stakeholders** of key change
5. **Update CI/CD pipelines** with new key
6. **Document incident** for compliance

---

## Air-Gapped Signing

For environments without internet access:

### Setup

1. **On internet-connected machine**:

   ```bash
   # Download Cosign binary
   curl -O -L https://github.com/sigstore/cosign/releases/download/v2.4.1/cosign-linux-amd64
   sha256sum cosign-linux-amd64

   # Transfer to air-gapped machine via USB
   ```

2. **On air-gapped machine**:

   ```bash
   # Install Cosign
   sudo install cosign-linux-amd64 /usr/local/bin/cosign

   # Verify installation
   cosign version
   ```

### Signing Without Rekor

```bash
# Sign without transparency log
COSIGN_EXPERIMENTAL=0 \
COSIGN_PRIVATE_KEY="$(cat ~/.cosign/cosign-airgap.key)" \
COSIGN_PASSWORD="your-password" \
cosign sign --yes --key ~/.cosign/cosign-airgap.key charon:local

# Note: This disables Rekor transparency log
# Suitable only for internal use or air-gapped environments
```

### Verification (Air-Gapped)

```bash
# Verify signature with public key only
cosign verify charon:local --key ~/.cosign/cosign-airgap.pub --insecure-ignore-tlog
```

**⚠️ SECURITY NOTE**: Air-gapped signing without Rekor loses public auditability. Use only when necessary and document the decision.

---

## Troubleshooting

### "cosign: error: signing: getting signer: reading key: decrypt: encrypted: no password provided"

**Cause**: Missing COSIGN_PASSWORD environment variable
**Solution**:

```bash
export COSIGN_PASSWORD="your-password"
cosign sign --key cosign.key charon:local
```

### "cosign: error: signing: getting signer: reading key: decrypt: openpgp: invalid data: private key checksum failure"

**Cause**: Incorrect password
**Solution**: Verify you're using the correct password for the key

### "Error: signing charon:local: uploading signature: PUT <https://registry/v2/.../manifests/sha256->...: UNAUTHORIZED"

**Cause**: Not authenticated with Docker registry
**Solution**:

```bash
docker login ghcr.io
# Enter credentials, then retry signing
```

### "Error: verifying charon:local: fetching signatures: getting signature manifest: GET <https://registry/>...: NOT_FOUND"

**Cause**: Image not signed yet, or signature not pushed to registry
**Solution**: Sign the image first with `cosign sign`

### Key File Corrupted

**Symptoms**: Decryption errors, unusual characters in key file
**Solution**:

1. Restore from encrypted backup (see Backup and Recovery)
2. If no backup: Generate new key pair and re-sign artifacts
3. Update documentation and notify stakeholders

### Lost Password

**Solution**:

1. **Cannot recover** - private key is permanently inaccessible
2. Generate new key pair
3. Revoke old public key from documentation
4. Re-sign all artifacts
5. Consider using password manager to prevent future loss

---

## Best Practices Summary

### DO

✅ Use strong passwords (20+ characters)
✅ Store keys in password manager or HSM
✅ Set restrictive file permissions (600 on private keys)
✅ Rotate keys every 90 days
✅ Create encrypted backups
✅ Use different keys for different environments
✅ Test keys after generation
✅ Document key rotation dates
✅ Use keyless signing in CI/CD when possible

### DON'T

❌ Commit private keys to version control
❌ Share private keys via email or chat
❌ Store keys in plaintext files
❌ Use the same key for multiple environments
❌ Hardcode passwords in scripts
❌ Skip backups
❌ Ignore rotation schedules
❌ Use weak passwords
❌ Store keys on network shares

---

## Security Contacts

If you suspect key compromise:

1. **Immediately**: Stop using the compromised key
2. **Notify**: Security team at <security@example.com>
3. **Rotate**: Generate new key pair
4. **Audit**: Review all signatures made with compromised key
5. **Document**: Create incident report

---

## References

- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview/)
- [Key Management Best Practices (NIST)](https://csrc.nist.gov/publications/detail/sp/800-57-part-1/rev-5/final)
- [OpenSSF Security Best Practices](https://best.openssf.org/)
- [SLSA Requirements](https://slsa.dev/spec/v1.0/requirements)

---

**Document Version**: 1.0
**Last Reviewed**: 2026-01-10
**Next Review**: 2026-04-10 (quarterly)
