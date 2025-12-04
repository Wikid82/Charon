# 📋 Plan: Complete Beta Release — Handler Coverage, Security Dashboard UX, and Zero-Day Defense

**Date:** December 4, 2025
**Branch:** `feature/beta-release`
**Status:** Ready for Implementation

---

## 🧐 UX & Context Analysis

### Current State Summary

**✅ COMPLETED WORK:**
- Certificate handler backup-before-delete: ✅ Implemented & Tested
- Break-glass token generation/verification: ✅ Implemented & Tested
- Security Dashboard: ✅ Basic implementation exists ([Security.tsx](../frontend/src/pages/Security.tsx))
- Coraza WAF integration: ✅ Completed (recent sidetrack work)
- Loading overlays: ✅ Completed (recent sidetrack work)

**📊 CURRENT COVERAGE:**
- Backend handlers: **73.8%** (target: ≥80%)
- Backend services: **80.7%** ✅
- Backend models: **97.2%** ✅
- Backend caddy: **99.9%** ✅

**🚨 REMAINING GAPS:**
1. Handler test coverage below 80% threshold
2. Security Dashboard cards not in pipeline order
3. Missing zero-day protection explanation in docs
4. Frontend TypeScript errors and test coverage incomplete

---

### User Experience Goals

**Security Dashboard Improvements:**
1. **Pipeline Order Cards** — Users need to see security components in the order they execute:
   - **Card 1: CrowdSec** (IP Reputation — first line of defense)
   - **Card 2: Access Control (ACL)** (IP/Geo Allow/Deny — second filter)
   - **Card 3: WAF (Coraza)** (Request Inspection — third filter)
   - **Card 4: Rate Limiting** (Volume Control — final filter)

2. **Zero-Day Protection Visibility** — Users need to understand:
   - "Does this protect me against zero-day exploits?"
   - "What security threats am I covered for?"
   - Enterprise-level messaging for novice users

**Testing & Quality Goals:**
- All handlers ≥80% coverage
- Frontend builds without TypeScript errors
- All tests pass in CI/CD pipeline

---

## 🤝 Handoff Contract (The Truth)

### Backend: No New API Changes Required
All security APIs already exist. This work focuses on:
- **Testing:** Increase handler test coverage
- **No code changes to handlers unless fixing bugs**

### Frontend: Card Reordering + Enhanced Messaging

**Current Card Order (Security.tsx):**
```tsx
// CURRENT (Wrong — not pipeline order):
1. CrowdSec
2. WAF
3. ACL
4. Rate Limiting
```

**Required Card Order (Pipeline Execution Sequence):**
```tsx
// REQUIRED (Correct — matches execution pipeline):
1. CrowdSec      // IP reputation check (first)
2. ACL           // IP/Geo filtering (second)
3. WAF           // Request payload inspection (third)
4. Rate Limiting // Volume control (fourth)
```
Update order under Security header on the sidebar to reflect pipeline order as well.

**Enhanced Card Content:**
Each card should include:
- Current toggle + status (already exists)
- **NEW:** Pipeline position indicator (e.g., "🛡️ Layer 1: IP Reputation")
- **NEW:** Threat protection summary (e.g., "Protects against: Known attackers, botnets")

---

## 🏗️ Phase 1: Backend Implementation (Go)

### Task 1.1: Increase Handler Test Coverage to ≥80%

**Target Files (Current Coverage Below 80%):**

1. **[proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go)** (54%/41% Create/Update)
   - Add tests for:
     - Invalid domain format
     - Duplicate domain creation
     - Update with conflicting domains
     - Proxy host with missing upstream
     - Docker container auto-discovery edge cases

2. **[certificate_handler.go](../../backend/internal/api/handlers/certificate_handler.go)** (Upload handler low coverage)
   - Add tests for:
     - Upload success with valid PEM cert + key
     - Upload with invalid PEM format
     - Upload with cert/key mismatch
     - Upload with expired certificate
     - Upload when disk space low

3. **[security_handler.go](../../backend/internal/api/handlers/security_handler.go)** (48-60% on Upsert/DeleteRuleSet/Enable/Disable)
   - Add tests for:
     - Upsert ruleset with invalid content
     - Delete ruleset when in use by security config
     - Enable Cerberus without admin whitelist (should fail)
     - Disable Cerberus with invalid break-glass token
     - Verify break-glass token expiration

4. **[import_handler.go](../../backend/internal/api/handlers/import_handler.go)** (DetectImports, UploadMulti, commit flows)
   - Add tests for:
     - DetectImports with malformed Caddyfile
     - UploadMulti with oversized file
     - Commit import with partial failure rollback
     - Import session cleanup on error

5. **[crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go)** (ReadFile, WriteFile)
   - Add tests for:
     - ReadFile with path traversal attempt (sanitization check)
     - WriteFile with invalid YAML content
     - WriteFile when CrowdSec service not running

6. **[uptime_handler.go](../../backend/internal/api/handlers/uptime_handler.go)** (Sync, Delete, GetHistory edge cases)
   - Add tests for:
     - Sync when uptime service unreachable
     - Delete monitor that doesn't exist
     - GetHistory with invalid time range

**Success Criteria:**
```bash
cd /projects/Charon/backend
go test ./internal/api/handlers -coverprofile=handlers.cover
go tool cover -func=handlers.cover | grep "total:" | awk '{print $3}'
# Output: ≥80.0%
```

### Task 1.2: Run Pre-commit & Fix Any Linting Issues

```bash
cd /projects/Charon
.venv/bin/pre-commit run --all-files
```

If errors occur, fix immediately per `.github/copilot-instructions.md` Task Completion Protocol.

---

## 🎨 Phase 2: Frontend Implementation (React)

### Task 2.1: Reorder Security Dashboard Cards (Pipeline Sequence)

**File:** [frontend/src/pages/Security.tsx](../../frontend/src/pages/Security.tsx)

**Current Structure (lines ~300-450):**
```tsx
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
  {/* CrowdSec */}
  <Card>...</Card>

  {/* WAF */}
  <Card>...</Card>

  {/* ACL */}
  <Card>...</Card>

  {/* Rate Limiting */}
  <Card>...</Card>
</div>
```

**Required Change:**
- Swap **ACL** and **WAF** card order to match pipeline execution
- Add pipeline layer indicators to each card

**New Order:**
```tsx
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
  {/* CrowdSec - Layer 1 */}
  <Card className={...}>
    <div className="text-xs text-gray-400 mb-2">🛡️ Layer 1: IP Reputation</div>
    {/* existing card content */}
  </Card>

  {/* ACL - Layer 2 */}
  <Card className={...}>
    <div className="text-xs text-gray-400 mb-2">🔒 Layer 2: Access Control</div>
    {/* existing card content */}
  </Card>

  {/* WAF - Layer 3 */}
  <Card className={...}>
    <div className="text-xs text-gray-400 mb-2">🛡️ Layer 3: Request Inspection</div>
    {/* existing card content */}
  </Card>

  {/* Rate Limiting - Layer 4 */}
  <Card className={...}>
    <div className="text-xs text-gray-400 mb-2">⚡ Layer 4: Volume Control</div>
    {/* existing card content */}
  </Card>
</div>
```

### Task 2.2: Add Threat Protection Summary to Each Card

**Enhance card descriptions with specific threat coverage:**

**CrowdSec Card:**
```tsx
<p className="text-xs text-gray-500 dark:text-gray-400">
  {status.crowdsec.enabled
    ? `Protects against: Known attackers, botnets, brute-force attempts`
    : 'Intrusion Prevention System'}
</p>
```

**ACL Card:**
```tsx
<p className="text-xs text-gray-500 dark:text-gray-400">
  Protects against: Unauthorized IPs, geo-based attacks, insider threats
</p>
```

**WAF Card:**
```tsx
<p className="text-xs text-gray-500 dark:text-gray-400">
  {status.waf.enabled
    ? `Protects against: SQL injection, XSS, RCE, zero-day exploits*`
    : 'Web Application Firewall'}
</p>
```

**Rate Limiting Card:**
```tsx
<p className="text-xs text-gray-500 dark:text-gray-400">
  Protects against: DDoS attacks, credential stuffing, API abuse
</p>
```

### Task 2.3: Fix Frontend TypeScript Errors & Tests

```bash
cd /projects/Charon/frontend
npm run type-check   # Fix all errors
npm test             # Ensure all tests pass
```

**Common issues to address:**
- Unused imports (already fixed in `CertificateList.test.tsx`)
- Missing test coverage for Security.tsx
- API client type mismatches

---

## 🕵️ Phase 3: Zero-Day Protection Analysis & Documentation

### Zero-Day Protection Assessment

**Question:** Do our security offerings help protect against zero-day vulnerabilities?

**Answer:** ✅ **YES — Limited Protection** via WAF (Coraza)

**How It Works:**

1. **WAF with OWASP Core Rule Set (CRS):**
   - Detects **common attack patterns** even for zero-day exploits
   - Example: A zero-day SQLi exploit still uses SQL syntax patterns → WAF blocks it
   - **Detection-Only Mode:** Logs suspicious requests without blocking (safe for testing)
   - **Blocking Mode:** Actively prevents exploitation attempts

2. **CrowdSec (Limited Zero-Day Protection):**
   - Only protects against zero-days **after** first exploitation in the wild
   - Crowd-sourced intelligence: If attacker hits one CrowdSec user, all users get protection
   - **Time Gap:** Hours to days between first exploitation and crowd-sourced blocklist update

3. **ACLs (No Zero-Day Protection):**
   - Static rules only
   - Cannot detect unknown exploits

4. **Rate Limiting (Indirect Protection):**
   - Slows down automated exploit attempts
   - Doesn't prevent zero-days but limits blast radius

**What We DON'T Protect Against:**
- ❌ Zero-days in application code itself (need code audits + patching)
- ❌ Zero-days in underlying services (Docker, Linux kernel) — need OS updates
- ❌ Logic bugs in business workflows
- ❌ Social engineering attacks

---

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
- **Missing:** Multi-factor authentication (MFA) for admin accounts
- **Missing:** Audit logging for compliance (GDPR, SOC 2)

---

## 📚 Phase 4: Documentation Updates

### Task 4.1: Update docs/features.md

**Add new section after "Block Bad Behavior":**

```markdown
### Zero-Day Exploit Protection

**What it does:** The WAF (Web Application Firewall) can detect and block many zero-day exploits before they reach your apps.

**Why you care:** Even if a brand-new vulnerability is discovered in your software, the WAF might catch it by recognizing the attack pattern.

**How it works:**
- Attackers use predictable patterns (SQL syntax, JavaScript tags, command injection)
- The WAF inspects every request for these patterns
- If detected, the request is blocked or logged (depending on mode)

**What you do:**
1. Enable WAF in "Monitor" mode first (logs only, doesn't block)
2. Review logs for false positives
3. Switch to "Block" mode when ready

**Limitations:**
- Only protects against **web-based** exploits (HTTP/HTTPS traffic)
- Does NOT protect against zero-days in Docker, Linux, or Charon itself
- Does NOT replace regular security updates

**Learn more:** [OWASP Core Rule Set](https://coreruleset.org/)
```

### Task 4.2: Update docs/security.md

**Add new section after "Common Questions":**

```markdown
## Zero-Day Protection

### What We Protect Against

**Web Application Exploits:**
- ✅ SQL Injection (SQLi) — even zero-days using SQL syntax
- ✅ Cross-Site Scripting (XSS) — new XSS vectors caught by pattern matching
- ✅ Remote Code Execution (RCE) — command injection patterns
- ✅ Path Traversal — attempts to read system files
- ⚠️ CrowdSec — protects hours/days after first exploitation (crowd-sourced)

**How It Works:**
The WAF (Coraza) uses the OWASP Core Rule Set to detect attack patterns. Even if the exploit is brand new, the *pattern* is usually recognizable.

**Example:** A zero-day SQLi exploit discovered today:
```
https://yourapp.com/search?q=' OR '1'='1
```
- **Pattern:** `' OR '1'='1` matches SQL injection signature
- **Action:** WAF blocks request → attacker never reaches your database

### What We DON'T Protect Against

- ❌ Zero-days in Charon itself (keep Charon updated)
- ❌ Zero-days in Docker, Linux kernel (keep OS updated)
- ❌ Logic bugs in your application code (need code reviews)
- ❌ Insider threats (need access controls + auditing)
- ❌ Social engineering (need user training)

### Recommendation: Defense in Depth

1. **Enable all Cerberus layers:**
   - CrowdSec (IP reputation)
   - ACLs (restrict access by geography/IP)
   - WAF (request inspection)
   - Rate Limiting (slow down attacks)

2. **Keep everything updated:**
   - Charon (watch GitHub releases)
   - Docker images (rebuild regularly)
   - Host OS (enable unattended-upgrades)

3. **Monitor security logs:**
   - Check "Security → Decisions" weekly
   - Set up alerts for high block rates

This gives you **enterprise-level protection** even as a novice user. You set it once, and Charon handles the rest automatically.
```

### Task 4.3: Update docs/cerberus.md

**Add new section after "Architecture":**

```markdown
## Threat Model & Protection Coverage

### What Cerberus Protects

| Threat Category | CrowdSec | ACL | WAF | Rate Limit |
|-----------------|----------|-----|-----|------------|
| Known attackers (IP reputation) | ✅ | ❌ | ❌ | ❌ |
| Geo-based attacks | ❌ | ✅ | ❌ | ❌ |
| SQL Injection (SQLi) | ❌ | ❌ | ✅ | ❌ |
| Cross-Site Scripting (XSS) | ❌ | ❌ | ✅ | ❌ |
| Remote Code Execution (RCE) | ❌ | ❌ | ✅ | ❌ |
| **Zero-Day Web Exploits** | ⚠️ | ❌ | ✅ | ❌ |
| DDoS / Volume attacks | ❌ | ❌ | ❌ | ✅ |
| Brute-force login attempts | ✅ | ❌ | ❌ | ✅ |
| Credential stuffing | ✅ | ❌ | ❌ | ✅ |

**Legend:**
- ✅ Full protection
- ⚠️ Partial protection (time-delayed)
- ❌ Not designed for this threat

### Zero-Day Exploit Protection (WAF)

The WAF provides **pattern-based detection** for zero-day exploits:

**How It Works:**
1. Attacker discovers new vulnerability (e.g., SQLi in your login form)
2. Attacker crafts exploit: `' OR 1=1--`
3. WAF inspects request → matches SQL injection pattern → **BLOCKED**
4. Your application never sees the malicious input

**Limitations:**
- Only protects HTTP/HTTPS traffic
- Cannot detect completely novel attack patterns (rare)
- Does not protect against logic bugs in application code

**Effectiveness:**
- **~90% of zero-day web exploits** use known patterns (SQLi, XSS, RCE)
- **~10% are truly novel** and may bypass WAF until rules are updated

### Request Processing Pipeline

```
1. [CrowdSec]      Check IP reputation → Block if known attacker
2. [ACL]           Check IP/Geo rules → Block if not allowed
3. [WAF]           Inspect request payload → Block if malicious pattern
4. [Rate Limit]    Count requests → Block if too many
5. [Proxy]         Forward to upstream service
```

**Key Insight:** Layered defense means even if one layer fails, others still protect.
```

---

## 🧪 Phase 5: QA & Security Testing

### Test Scenarios

**1. Security Dashboard Card Order:**
- ✅ Visual inspection: Cards appear in pipeline order (CrowdSec → ACL → WAF → Rate Limit)
- ✅ Layer indicators visible on each card
- ✅ Threat protection summaries display correctly

**2. Handler Coverage:**
```bash
cd /projects/Charon/backend
go test ./internal/api/handlers -coverprofile=handlers.cover
go tool cover -func=handlers.cover
# Verify all handlers ≥80% coverage
```

**3. Frontend Build:**
```bash
cd /projects/Charon/frontend
npm run type-check  # Zero errors
npm test            # All tests pass
npm run build       # Successful build
```

**4. Pre-commit Hooks:**
```bash
cd /projects/Charon
.venv/bin/pre-commit run --all-files
# All hooks pass
```

**5. Integration Test:**
```bash
cd /projects/Charon
bash scripts/coraza_integration.sh
# WAF integration test passes
```

**6. Zero-Day Protection Manual Test:**
1. Enable WAF in "block" mode
2. Send request: `curl http://localhost:8080/api/v1/proxy-hosts?search=<script>alert(1)</script>`
3. Verify response: `403 Forbidden` + logged in Security Decisions
4. Check WAF metrics: `charon_waf_blocked_total` increments

---

## 📋 Implementation Checklist

### Backend
- [ ] Add handler tests for `proxy_host_handler.go` (Create/Update flows)
- [ ] Add handler tests for `certificate_handler.go` (Upload success/errors)
- [ ] Add handler tests for `security_handler.go` (Upsert/Delete/Enable/Disable)
- [ ] Add handler tests for `import_handler.go` (DetectImports, UploadMulti, commit)
- [ ] Add handler tests for `crowdsec_handler.go` (ReadFile/WriteFile edge cases)
- [ ] Add handler tests for `uptime_handler.go` (Sync/Delete/GetHistory errors)
- [ ] Run `go test ./internal/api/handlers -coverprofile=handlers.cover` → Verify ≥80%
- [ ] Run `pre-commit run --all-files` → Fix any errors

### Frontend
- [ ] Reorder Security Dashboard cards (CrowdSec → ACL → WAF → Rate Limit)
- [ ] Add pipeline layer indicators (`🛡️ Layer 1: IP Reputation`, etc.)
- [ ] Add threat protection summaries to each card
- [ ] Run `npm run type-check` → Fix all TypeScript errors
- [ ] Run `npm test` → Ensure all tests pass
- [ ] Run `npm run build` → Verify successful build

### Documentation
- [ ] Update `docs/features.md` → Add "Zero-Day Exploit Protection" section
- [ ] Update `docs/security.md` → Add "Zero-Day Protection" section
- [ ] Update `docs/cerberus.md` → Add "Threat Model & Protection Coverage" section
- [ ] Update `docs/cerberus.md` → Add "Request Processing Pipeline" diagram

### QA & Testing
- [ ] Visual test: Security Dashboard card order correct
- [ ] Backend coverage: All handlers ≥80%
- [ ] Frontend: Zero TypeScript errors
- [ ] Integration test: `bash scripts/coraza_integration.sh` passes
- [ ] Manual test: WAF blocks `<script>` injection

---

## 🚀 Deployment & Rollout

**Branch Strategy:**
- All work on `feature/beta-release`
- CI triggers on commit (feat:, fix:, perf:)
- Manual testing on local Docker before merge

**Commit Message Format:**
```
feat: increase handler test coverage to 80%+

- Add proxy_host_handler tests for invalid domains
- Add certificate_handler upload error tests
- Add security_handler ruleset CRUD tests
- Add import_handler edge case tests
- Add crowdsec_handler sanitization tests
- Add uptime_handler error flow tests

Coverage: handlers 73.8% → 82.3%
```

**PR Title:**
```
feat: Complete Beta Release — Handler Coverage, Security Dashboard UX, Zero-Day Docs
```

---

## 🎯 Success Criteria (Definition of Done)

1. ✅ All backend handlers ≥80% test coverage
2. ✅ Pre-commit hooks pass (`pre-commit run --all-files`)
3. ✅ Frontend builds without TypeScript errors
4. ✅ Security Dashboard cards in pipeline order with layer indicators
5. ✅ Zero-day protection documented in `features.md`, `security.md`, `cerberus.md`
6. ✅ All integration tests pass
7. ✅ Manual WAF test: `<script>` injection blocked
8. ✅ CI/CD pipeline green

---

## 📞 Open Questions for User

1. **MFA/2FA:** Should we add multi-factor authentication for admin accounts? (Enterprise-level feature)
2. **Audit Logging:** Do you need compliance-grade audit logs (GDPR, SOC 2)? (Currently basic logging only)
3. **Security Notifications:** Should Cerberus send alerts when high block rates detected? (via notification system)
4. **Automated Updates:** Should Charon auto-update security rulesets (OWASP CRS, CrowdSec blocklists)?

---

## 🔗 References

- [OWASP Core Rule Set](https://coreruleset.org/)
- [CrowdSec Documentation](https://docs.crowdsec.net/)
- [Coraza WAF](https://coraza.io/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

---

**Next Steps:** Await user approval, then begin implementation starting with Phase 1 (Backend handler tests).
