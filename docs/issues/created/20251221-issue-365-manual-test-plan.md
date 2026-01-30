---
title: "Issue #365: Additional Security Enhancements - Manual Test Plan"
labels:
  - manual-testing
  - security
  - testing
type: testing
priority: medium
parent_issue: 365
---

# Issue #365: Additional Security Enhancements - Manual Test Plan

**Issue**: <https://github.com/Wikid82/Charon/issues/365>
**PRs**: #436, #437
**Status**: Ready for Manual Testing

---

## Test Scenarios

### 1. Invite Token Security

**Objective**: Verify constant-time token comparison doesn't leak timing information.

**Steps**:

1. Create a new user invite via the admin UI
2. Copy the invite token from the generated link
3. Attempt to accept the invite with the correct token - should succeed
4. Attempt to accept with a token that differs only in the last character - should fail with same response time
5. Attempt to accept with a completely wrong token - should fail with same response time

**Expected**: Response times should be consistent regardless of where the token differs.

---

### 2. Security Headers Verification

**Objective**: Verify all security headers are present.

**Steps**:

1. Start Charon with HTTPS enabled
2. Use browser dev tools or curl to inspect response headers
3. Verify presence of:
   - `Content-Security-Policy`
   - `Strict-Transport-Security` (with preload)
   - `X-Frame-Options: DENY`
   - `X-Content-Type-Options: nosniff`
   - `Referrer-Policy`
   - `Permissions-Policy`

**curl command**:

```bash
curl -I https://your-charon-instance.com/
```

---

### 3. Container Hardening (Optional - Production)

**Objective**: Verify documented container hardening works.

**Steps**:

1. Deploy Charon using the hardened docker-compose config from docs/security.md
2. Verify container starts successfully with `read_only: true`
3. Verify all functionality works (proxy hosts, certificates, etc.)
4. Verify logs are written to tmpfs mount

---

### 4. Documentation Review

**Objective**: Verify all documentation is accurate and complete.

**Pages to Review**:

- [ ] `docs/security.md` - TLS, DNS, Container Hardening sections
- [ ] `docs/security-incident-response.md` - SIRP document
- [ ] `docs/getting-started.md` - Security Update Notifications section

**Check for**:

- Correct code examples
- Working links
- No typos or formatting issues

---

### 5. SBOM Generation (CI/CD)

**Objective**: Verify SBOM is generated on release builds.

**Steps**:

1. Push a commit to trigger a non-PR build
2. Check GitHub Actions workflow run
3. Verify "Generate SBOM" step completes successfully
4. Verify "Attest SBOM" step completes successfully
5. Verify attestation is visible in GitHub container registry

---

## Acceptance Criteria

- [ ] All test scenarios pass
- [ ] No regressions in existing functionality
- [ ] Documentation is accurate and helpful

---

**Tester**: ________________
**Date**: ________________
**Result**: [ ] PASS / [ ] FAIL
