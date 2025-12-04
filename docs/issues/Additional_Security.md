### Additional Security Threats to Consider

**1. Supply Chain Attacks**
- **Threat:** Compromised Docker images, npm packages, Go modules
- **Current Protection:** ❌ None
- **Recommendation:** Add Trivy scanning (already in CI) + SBOM generation

**2. DNS Hijacking / Cache Poisoning**
- **Threat:** Attacker redirects DNS queries to malicious servers
- **Current Protection:** ❌ None (relies on system DNS resolver)
- **Recommendation:** Document use of encrypted DNS (DoH/DoT) in deployment guide

**3. TLS Downgrade Attacks**
- **Threat:** Force clients to use weak TLS versions
- **Current Protection:** ✅ Caddy enforces TLS 1.2+ by default
- **Recommendation:** Document minimum TLS version in security.md

**4. Certificate Transparency (CT) Log Poisoning**
- **Threat:** Attacker registers fraudulent certs for your domains
- **Current Protection:** ❌ None
- **Recommendation:** Add CT log monitoring (future feature)

**5. Privilege Escalation (Container Escape)**
- **Threat:** Attacker escapes Docker container to host OS
- **Current Protection:** ⚠️ Partial (Docker security best practices)
- **Recommendation:** Document running with least-privilege, read-only root filesystem

**6. Session Hijacking / Cookie Theft**
- **Threat:** Steal user session tokens via XSS or network sniffing
- **Current Protection:** ✅ HTTPOnly cookies, Secure flag, SameSite (verify implementation)
- **Recommendation:** Add CSP (Content Security Policy) headers

**7. Timing Attacks (Cryptographic Side-Channel)**
- **Threat:** Infer secrets by measuring response times
- **Current Protection:** ❌ Unknown (need bcrypt timing audit)
- **Recommendation:** Use constant-time comparison for tokens

**Enterprise-Level Security Gaps:**
- **Missing:** Security Incident Response Plan (SIRP)
- **Missing:** Automated security update notifications
- **Missing:** Multi-factor authentication (MFA) for admin accounts (Use Authentik via built in. No extra external containers)
- **Missing:** Audit logging for compliance (GDPR, SOC 2)
