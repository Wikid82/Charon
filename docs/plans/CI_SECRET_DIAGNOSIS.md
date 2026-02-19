# CI Secret Diagnosis - CHARON_ENCRYPTION_KEY_TEST

**Date:** 2026-02-16
**Status:** ROOT CAUSE IDENTIFIED ✅
**Severity:** HIGH (Blocking CI)

---

## 🔴 CRITICAL FINDING: Wrong Key Generation Command Used

### Root Cause Analysis

The CI logs reveal **two different error patterns**:

#### Pattern 1: Key Not Set (Older/Some Tests)
```
Warning: RotationService initialization failed, using basic encryption: CHARON_ENCRYPTION_KEY is required
```

#### Pattern 2: Invalid Key Length (Most Tests) ⬅️ **THE ACTUAL PROBLEM**
```
Warning: RotationService initialization failed, using basic encryption:
failed to load current encryption key: invalid key length: expected 32 bytes, got 48 bytes
```

### The Smoking Gun 🔍

**Evidence from terminal history:**
```bash
Terminal: root@srv599055: /projects/Charon
Last Command: openssl rand -hex 32      # ❌ WRONG!
```

**What happened:**
- Command `openssl rand -hex 32` generates **64 hexadecimal characters** (32 bytes as hex)
- When base64-decoded, 64 characters = **48 bytes** of decoded data
- Application expects exactly **32 bytes** after base64 decoding

**Math:**
- `openssl rand -hex 32` → 64 hex chars → 48 bytes when base64-decoded ❌
- `openssl rand -base64 32` → 44 base64 chars → 32 bytes when decoded ✅

### Code Validation

From `backend/internal/crypto/encryption.go:32-39`:
```go
// NewEncryptionService creates a new encryption service with the provided base64-encoded key.
// The key must be exactly 32 bytes (256 bits) when decoded.
func NewEncryptionService(keyBase64 string) (*EncryptionService, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d bytes", len(key))
	}
```

**Expectation:** Base64-encoded string that decodes to exactly 32 bytes (AES-256 key)

---

## ✅ IMMEDIATE FIX

### Step 1: Generate Correct Secret

Run this command **locally** to generate a valid key:

```bash
openssl rand -base64 32
```

**Example output (44 characters):**
```
YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
```

### Step 2: Update GitHub Secret

1. **Navigate to:** `https://github.com/{OWNER}/{REPO}/settings/secrets/actions`
2. **Find:** `CHARON_ENCRYPTION_KEY_TEST`
3. **Click:** "Update"
4. **Paste:** The output from `openssl rand -base64 32`
5. **Save:** Click "Update secret"

### Step 3: Verify Secret Format

Before saving, verify the secret:
- ✅ Should be ~44 characters long (base64 encoded)
- ✅ Should end with `=` or `==` (base64 padding)
- ❌ Should NOT be 64 characters (that's hex, not base64)
- ❌ Should NOT contain only `0-9a-f` characters (that's hex)

### Step 4: Re-run Failed Workflows

After updating the secret:
1. Go to the failed PR check
2. Click "Re-run jobs"
3. Monitor for the error message:
   - ❌ `expected 32 bytes, got 48 bytes` = Still wrong
   - ✅ No warnings = Fixed!

---

## 📋 Verification Checklist

After updating the secret, the CI logs should show:

- [ ] No "CHARON_ENCRYPTION_KEY is required" errors
- [ ] No "invalid key length: expected 32 bytes, got 48 bytes" errors
- [ ] Tests pass without RotationService warnings
- [ ] Backend tests complete successfully
- [ ] Codecov upload succeeds

---

## 🔄 WHY This Happened

**User confusion:** OpenSSL has two similar commands:
- `openssl rand -base64 N` → Generates N bytes, outputs as base64 (CORRECT)
- `openssl rand -hex N` → Generates N bytes, outputs as hex (WRONG for this use case)

The hex output looks "more random" with 0-9a-f characters, which may have seemed more secure, but it's the **wrong encoding**.

---

## 🛡️ Prevention

### Documentation Updates Needed

1. **Add to `.env.example`:**
```bash
# Generate with: openssl rand -base64 32
# Must be base64-encoded 256-bit (32-byte) key
# DO NOT use -hex, must use -base64!
CHARON_ENCRYPTION_KEY=
```

2. **Add to `docs/development/secrets-management.md`:**
```markdown
## Encryption Key Generation

**CRITICAL:** Always use `-base64`, never `-hex`:

✅ **CORRECT:**
```bash
openssl rand -base64 32
```

❌ **WRONG:**
```bash
openssl rand -hex 32  # This will cause "expected 32 bytes, got 48 bytes" error
```
```

3. **Add validation script:** `scripts/validate-secrets.sh`
```bash
#!/bin/bash
# Validates CHARON_ENCRYPTION_KEY format

KEY="${CHARON_ENCRYPTION_KEY:-}"
if [ -z "$KEY" ]; then
    echo "❌ CHARON_ENCRYPTION_KEY not set"
    exit 1
fi

# Decode and check length
DECODED=$(echo "$KEY" | base64 -d 2>/dev/null | wc -c)
if [ "$DECODED" -ne 32 ]; then
    echo "❌ Invalid key length: $DECODED bytes (expected 32)"
    echo "   Generate correct key with: openssl rand -base64 32"
    exit 1
fi

echo "✅ CHARON_ENCRYPTION_KEY is valid (32 bytes)"
```

---

## 📊 Impact Analysis

**Tests Affected:** ALL backend tests that initialize services with encryption
**Workflows Affected:**
- `quality-checks.yml` (Backend Quality)
- `codecov-upload.yml` (Backend Codecov)
- Any workflow calling `scripts/go-test-coverage.sh`

**False Positives:** Some older logs show "CHARON_ENCRYPTION_KEY is required" which indicates the env var wasn't set at all. This may be from before the workflow changes were merged or from different test contexts.

---

## ⏭️ Next Steps

1. ✅ **IMMEDIATE:** Regenerate secret with correct command
2. ✅ **IMMEDIATE:** Update GitHub secret `CHARON_ENCRYPTION_KEY_TEST`
3. ✅ **IMMEDIATE:** Re-run failed CI workflows
4. 🔄 **FOLLOW-UP:** Add validation script to pre-commit hooks
5. 🔄 **FOLLOW-UP:** Update documentation with clear instructions
6. 🔄 **FOLLOW-UP:** Add CI check to validate secret format on workflow start

---

## 🎯 Expected Outcome

After fix, CI logs should show:
```
✅ Backend tests: All tests passed
✅ No RotationService warnings
✅ Codecov upload: Success
✅ Quality checks: Passed
```

---

## 📞 User Action Required

**Please execute these commands now:**

```bash
# 1. Generate correct key
NEW_KEY=$(openssl rand -base64 32)

# 2. Verify it's correct format (should output "32")
echo "$NEW_KEY" | base64 -d | wc -c

# 3. Output the key to copy (will be ~44 chars)
echo "$NEW_KEY"

# 4. Go to GitHub → Settings → Secrets → Actions
# 5. Update CHARON_ENCRYPTION_KEY_TEST with the output from step 3
# 6. Re-run the failed workflow
```

---

## 🔐 Security Note

The old (wrong) secret should be considered compromised and should not be reused. Always generate a fresh secret when correcting this issue.
