# Security Fix: Remove Hardcoded Encryption Keys from Docker Compose Files

**Plan ID**: SEC-2026-001
**Status**: ✅ IMPLEMENTED
**Priority**: Critical (Security)
**Created**: 2026-01-25
**Implemented By**: Management Agent

---

## Summary

Removed hardcoded encryption keys from Docker Compose test files and implemented ephemeral key generation in CI workflows.

## Changes Applied

| File | Change |
|------|--------|
| `.docker/compose/docker-compose.playwright.yml` | Replaced hardcoded key with `${CHARON_ENCRYPTION_KEY:?...}` |
| `.docker/compose/docker-compose.e2e.yml` | Replaced hardcoded key with `${CHARON_ENCRYPTION_KEY:?...}` |
| `.github/workflows/e2e-tests.yml` | Added ephemeral key generation step |
| `.env.test.example` | Added prominent documentation |

## Security Notes

- The old key `ucDWy5ScLubd3QwCHhQa2SY7wL2OF48p/c9nZhyW1mA=` exists in git history
- This key should **NEVER** be used in any production environment
- Each CI run now generates a unique ephemeral key

## Testing

```bash
# Verify compose fails without key
unset CHARON_ENCRYPTION_KEY
docker compose -f .docker/compose/docker-compose.playwright.yml config 2>&1
# Expected: "CHARON_ENCRYPTION_KEY is required"

# Verify compose succeeds with key
export CHARON_ENCRYPTION_KEY=$(openssl rand -base64 32)
docker compose -f .docker/compose/docker-compose.playwright.yml config
# Expected: Valid YAML output
```

## References

- **OWASP**: [A02:2021 – Cryptographic Failures](https://owasp.org/Top10/A02_2021-Cryptographic_Failures/)
