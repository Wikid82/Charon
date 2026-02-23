# Issue #365: Additional Security Enhancements

**Status**: Planning
**Created**: 2025-12-21
**Issue**: <https://github.com/Wikid82/Charon/issues/365>

---

## Objective

Implement additional security enhancements to address identified threats and gaps in the current security posture.

## Security Threats to Address

### 1. Supply Chain Attacks ❌ → ✅

- **Threat:** Compromised Docker images, npm packages, Go modules
- **Current Protection:** Trivy scanning in CI
- **Implementation:**
  - [ ] Add SBOM (Software Bill of Materials) generation
  - [ ] Enhanced dependency scanning

### 2. DNS Hijacking / Cache Poisoning ❌ → 📖

- **Threat:** Attacker redirects DNS queries to malicious servers
- **Implementation:**
  - [ ] Document use of encrypted DNS (DoH/DoT) in deployment guide

### 3. TLS Downgrade Attacks ✅ → 📖

- **Threat:** Force clients to use weak TLS versions
- **Current Protection:** Caddy enforces TLS 1.2+ by default
- **Implementation:**
  - [ ] Document minimum TLS version in security.md

### 4. Certificate Transparency (CT) Log Poisoning ❌ → 🔮

- **Threat:** Attacker registers fraudulent certs for your domains
- **Implementation:** Future feature (separate issue)

### 5. Privilege Escalation (Container Escape) ⚠️ → 📖

- **Threat:** Attacker escapes Docker container to host OS
- **Current Protection:** Docker security best practices (partial)
- **Implementation:**
  - [ ] Document running with least-privilege
  - [ ] Document read-only root filesystem configuration

### 6. Session Hijacking / Cookie Theft ✅ → 🔒

- **Threat:** Steal user session tokens via XSS or network sniffing
- **Current Protection:** HTTPOnly cookies, Secure flag, SameSite
- **Implementation:**
  - [ ] Verify current cookie implementation
  - [ ] Add CSP (Content Security Policy) headers

### 7. Timing Attacks (Cryptographic Side-Channel) ❌ → 🔒

- **Threat:** Infer secrets by measuring response times
- **Implementation:**
  - [ ] Audit bcrypt timing
  - [ ] Use constant-time comparison for tokens

## Enterprise-Level Security Gaps

### In Scope (This Issue)

- [ ] Security Incident Response Plan (SIRP) documentation
- [ ] Automated security update notifications documentation

### Out of Scope (Future Issues)

- Multi-factor authentication (MFA) via Authentik
- SSO for Charon admin
- Audit logging for compliance (GDPR, SOC 2)
- CT log monitoring

## Implementation Phases

### Phase 1: Documentation Updates

1. Update `docs/security.md` with TLS minimum version
2. Add container hardening guide
3. Add DNS security deployment guide
4. Create Security Incident Response Plan

### Phase 2: Code Changes

1. Implement CSP headers in backend
2. Add constant-time token comparison
3. Verify cookie security flags
4. Add SBOM generation to CI

### Phase 3: Testing & Validation

1. Security audit of all changes
2. Penetration testing documentation
3. Update integration tests

---

*This document will be updated as planning progresses.*
