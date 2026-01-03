
# Charon Feature & Remediation Tracker

**Last Updated:** January 3, 2026

This document serves as the central index for all active plans, implementation specs, and outstanding work items.

---

## 1. SSRF Remediation

**Status:** 🔴 IN PROGRESS

The authoritative, Supervisor-updated SSRF plan is:

- [docs/plans/ssrf-remediation.md](ssrf-remediation.md)

### Merge Policy (Supervisor requirement)

- The global CodeQL exclusion for `go/request-forgery` in
  [.github/codeql/codeql-config.yml](../../.github/codeql/codeql-config.yml) must be removed
  in the same PR/merge as the underlying SSRF fixes.
- Phase 0 can include local-only recon (e.g., temporary local edit of CodeQL config to
  surface findings), but must not be a mergeable intermediate state.

### SSRF Call Sites (Current Known)

| Location | Function | File |
|----------|----------|------|
| Uptime Monitor | `(*UptimeService).checkMonitor` | [uptime_service.go](../../backend/internal/services/uptime_service.go) |
| CrowdSec LAPI | `GetLAPIDecisions`, `CheckLAPIHealth` | [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) |
| Caddy Admin API | `NewClient`, `Load/GetConfig/Ping` | [client.go](../../backend/internal/caddy/client.go) |
| URL Connectivity Test | `utils.TestURLConnectivity` | [url_testing.go](../../backend/internal/utils/url_testing.go) |

---

## 2. DNS Provider Feature (Issue #21)

### Core Implementation

**Status:** ✅ COMPLETE

- **Implementation Spec:** [dns_providers_IMPLEMENTATION.md](../implementation/dns_providers_IMPLEMENTATION.md)
- **Pull Request:** [#461](https://github.com/Wikid82/Charon/pull/461)

All core components implemented:

| Layer | Component | Status |
|-------|-----------|--------|
| Backend | Encryption Service (`crypto/encryption.go`) | ✅ Complete |
| Backend | DNSProvider Model | ✅ Complete |
| Backend | DNS Provider Service | ✅ Complete |
| Backend | DNS Provider Handler | ✅ Complete |
| Backend | Routes Registered | ✅ Complete |
| Backend | Caddy DNS-01 Integration | ✅ Complete |
| Frontend | API Client & Hooks | ✅ Complete |
| Frontend | DNS Providers Page & Form | ✅ Complete |
| Frontend | ProxyHost Integration | ✅ Complete |
| Frontend | Translations | ✅ Complete |

### Acceptance Criteria Verification

| Criterion | Status |
|-----------|--------|
| Users can add, edit, delete, and test DNS provider configurations | ✅ Implemented |
| Credentials encrypted at rest using AES-256-GCM | ✅ Implemented |
| Credentials never exposed in API responses | ✅ Implemented (`json:"-"`) |
| Proxy hosts with wildcard domains can select a DNS provider | ✅ Implemented |
| Caddy successfully obtains wildcard certificates via DNS-01 | ✅ Implemented |
| Backend unit test coverage ≥ 85% | ✅ **85.2%** (verified 2026-01-03) |
| Frontend unit test coverage ≥ 85% | ✅ **87.8%** (verified 2026-01-03) |
| User documentation completed | ✅ Complete (5 provider guides) |
| All translations added | ✅ Complete |

### Verification Results (2026-01-03)

| Check | Result |
|-------|--------|
| Backend Coverage | ✅ 85.2% (threshold: 85%) |
| Frontend Coverage | ✅ 87.8% (threshold: 85%) |
| Security Scan (Trivy) | ✅ 0 Critical, 0 High |
| Security Scan (govulncheck) | ✅ 0 vulnerabilities |
| Pre-commit Hooks | ✅ All 11 hooks passed |
| CHANGELOG | ✅ Entry exists in [Unreleased] |

### Outstanding Items (Pre-Merge)

- [x] ~~Run backend coverage report~~ — **85.2%** ✅
- [x] ~~Run frontend coverage report~~ — **87.8%** ✅
- [x] ~~Complete Google Cloud DNS setup guide~~ — Created ✅
- [x] ~~Complete Azure DNS setup guide~~ — Created ✅
- [ ] Manual E2E validation: DNS provider → wildcard proxy → certificate issued
- [x] ~~CHANGELOG entry for DNS provider feature~~ — Already present ✅
- [x] ~~Security scans (Trivy, govulncheck)~~ — Passed ✅

### Future Enhancements

**Status:** 📋 PLANNING

- **Planning Doc:** [dns_challenge_future_features.md](dns_challenge_future_features.md)

| Priority | Feature | Est. Time | Status |
|----------|---------|-----------|--------|
| **P0** | Audit Logging for Credential Operations | 8-12 hrs | ❌ Not Started |
| **P1** | Key Rotation Automation | 16-20 hrs | ❌ Not Started |
| **P1** | Multi-Credential per Provider (Zone-Specific) | 12-16 hrs | ❌ Not Started |
| **P2** | DNS Provider Auto-Detection | 6-8 hrs | ❌ Not Started |
| **P3** | Custom DNS Provider Plugins | 20-24 hrs | ❌ Not Started |

**Recommended Implementation Order:**
1. Audit Logging (Security/Compliance baseline for SOC 2, GDPR, HIPAA)
2. Key Rotation (Security hardening, annual rotation support)
3. Multi-Credential (Enterprise/MSP multi-tenancy)
4. Auto-Detection (UX improvement)
5. Custom Plugins (Extensibility for power users)

---

## 3. Related Documents (Index)

| Document | Description |
|----------|-------------|
| [patch-coverage-codecov.md](patch-coverage-codecov.md) | Codecov patch coverage plan |
| [codeql-local-hygiene.md](codeql-local-hygiene.md) | CodeQL/Trivy local scan hygiene notes |
| [dns_providers_IMPLEMENTATION.md](../implementation/dns_providers_IMPLEMENTATION.md) | DNS provider full implementation spec |
| [dns_challenge_future_features.md](dns_challenge_future_features.md) | DNS challenge future enhancements plan |

---

## 4. Definition of Done (All Features)

Before any feature is considered complete:

- [ ] Backend unit test coverage ≥ 85%
- [ ] Frontend unit test coverage ≥ 85%
- [ ] TypeScript check passes (`npm run type-check`)
- [ ] Pre-commit hooks pass (`pre-commit run --all-files`)
- [ ] CodeQL scans: zero Critical/High issues
- [ ] Trivy scans: zero Critical/High vulnerabilities
- [ ] All linters pass
- [ ] Documentation updated
- [ ] CHANGELOG updated
