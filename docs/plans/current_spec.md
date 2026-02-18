## PR #718 CodeQL Remediation Master Plan (Detailed)

### Introduction

This plan defines a full remediation program for CodeQL findings associated with PR #718, using repository evidence from:

- `docs/reports/codeql_pr718_origin_map.md`
- `codeql-results-go.sarif`
- `codeql-results-js.sarif`
- `codeql-results-javascript.sarif`
- GitHub Code Scanning API snapshot for PR #718 (`state=open`)

Objectives:

1. Close all PR #718 findings with deterministic verification.
2. Prioritize security-impacting findings first, then correctness/quality findings.
3. Minimize review overhead by slicing work into the fewest safe PRs.
4. Harden repository hygiene in `.gitignore`, `.dockerignore`, `codecov.yml`, and `.codecov.yml`.

### Research Findings

#### Evidence summary

- Origin-map report identifies **67 high alerts** mapped to PR #718 integration context:
  - `go/log-injection`: 58
  - `js/regex/missing-regexp-anchor`: 6
  - `js/insecure-temporary-file`: 3
- Current PR #718 open alert snapshot contains **100 open alerts**:
  - `js/unused-local-variable`: 95
  - `js/automatic-semicolon-insertion`: 4
  - `js/comparison-between-incompatible-types`: 1
- Current local SARIF snapshots show:
  - `codeql-results-go.sarif`: 84 results (83 `go/log-injection`, 1 `go/cookie-secure-not-set`)
  - `codeql-results-js.sarif`: 142 results (includes 6 `js/regex/missing-regexp-anchor`, 3 `js/insecure-temporary-file`)
  - `codeql-results-javascript.sarif`: 0 results (stale/alternate artifact format)

#### Architecture and hotspot mapping (files/functions/components)

Primary backend hotspots (security-sensitive log sinks):

- `backend/internal/api/handlers/crowdsec_handler.go`
  - `(*CrowdsecHandler) PullPreset`
  - `(*CrowdsecHandler) ApplyPreset`
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `(*ProxyHostHandler) Update`
- `backend/internal/api/handlers/emergency_handler.go`
  - `(*EmergencyHandler) SecurityReset`
  - `(*EmergencyHandler) performSecurityReset`
- `backend/internal/services/uptime_service.go`
  - `(*UptimeService) CreateMonitor`
- `backend/internal/crowdsec/hub_sync.go`
  - `(*HubService) Pull`
  - `(*HubService) Apply`
  - `(*HubService) fetchWithFallback`
  - `(*HubService) loadCacheMeta`
  - `(*HubService) refreshCache`

Primary frontend/test hotspots:

- `tests/fixtures/auth-fixtures.ts`
  - `acquireLock`
  - `saveTokenCache`
- `tests/tasks/import-caddyfile.spec.ts`
  - `test('should accept valid Caddyfile via file upload', ...)`
  - `test('should accept valid Caddyfile via paste', ...)`
- `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx`
  - CSP report-only URI test case
- `frontend/src/components/CredentialManager.tsx`
  - incompatible type comparison at line 274

#### Risk interpretation

- The 67 high-security findings are blocking from a security posture perspective.
- The 100 open findings are mostly non-blocking quality/test hygiene, but they increase review noise and hide true security deltas.
- The most important engineering risk is inconsistent scanning/reporting context between CI, local tasks, and artifact naming.

### Requirements (EARS)

1. **WHEN** PR #718 findings are remediated, **THE SYSTEM SHALL** produce zero high/critical CodeQL findings in Go and JavaScript scans.
2. **WHEN** log lines include user-influenced data, **THE SYSTEM SHALL** sanitize or quote those values before logging.
3. **WHEN** URL host regexes are used in assertions or validation, **THE SYSTEM SHALL** anchor expressions with explicit start/end boundaries.
4. **WHEN** temporary files are created in tests/fixtures, **THE SYSTEM SHALL** use secure creation semantics with restricted permissions and deterministic cleanup.
5. **WHEN** lint/quality-only findings are present, **THE SYSTEM SHALL** resolve them in a dedicated cleanup slice that does not change runtime behavior.
6. **IF** scan artifacts conflict (`codeql-results-javascript.sarif` vs `codeql-results-js.sarif`), **THEN THE SYSTEM SHALL** standardize to one canonical artifact path per language.
7. **WHILE** remediation is in progress, **THE SYSTEM SHALL** preserve deployability and pass DoD gates for each PR slice.

### Technical Specifications

#### API / Backend design targets

- Introduce a consistent log-sanitization pattern:
  - Use `utils.SanitizeForLog(...)` on user-controlled values.
  - Prefer structured logging with placeholders instead of string concatenation.
  - For ambiguous fields, use `%q`/quoted output where readability permits.
- Apply changes in targeted handlers/services only (no broad refactor in same PR):
  - `backup_handler.go`, `crowdsec_handler.go`, `docker_handler.go`, `emergency_handler.go`, `proxy_host_handler.go`, `security_handler.go`, `settings_handler.go`, `uptime_handler.go`, `user_handler.go`
  - `middleware/emergency.go`
  - `cerberus/cerberus.go`, `cerberus/rate_limit.go`
  - `crowdsec/console_enroll.go`, `crowdsec/hub_cache.go`, `crowdsec/hub_sync.go`
  - `server/emergency_server.go`
  - `services/backup_service.go`, `services/emergency_token_service.go`, `services/mail_service.go`, `services/manual_challenge_service.go`, `services/uptime_service.go`

#### Frontend/test design targets

- Regex remediation:
  - Replace unanchored host patterns with anchored variants: `^https?:\/\/(allowed-host)(:\d+)?$` style.
- Insecure temp-file remediation:
  - Replace ad hoc temp writes with `fs.mkdtemp`-scoped directories, `0o600` file permissions, and cleanup in `finally`.
- Quality warning remediation:
  - Remove unused locals/imports in test utilities/specs.
  - Resolve ASI warnings with explicit semicolons / expression wrapping.
  - Resolve one incompatible comparison with explicit type normalization and guard.

#### CI/reporting hardening targets

- Standardize scan outputs:
  - Go: `codeql-results-go.sarif`
  - JS/TS: `codeql-results-js.sarif`
- Enforce single source of truth for local scans:
  - `.vscode/tasks.json` → existing `scripts/pre-commit-hooks/codeql-*.sh` wrappers.
- Keep `security-and-quality` suite explicit and consistent.

### Finding-by-Finding Remediation Matrix

#### Matrix A — High-risk units correlated to PR #718 origin commits

Scope: 75 location-level units from repository evidence (weighted counts), covering `go/log-injection`, `js/regex/missing-regexp-anchor`, and `js/insecure-temporary-file`.

| Finding Unit | Count | Rule | Severity | File | Line | Function/Test Context | Root cause hypothesis | Fix pattern | Verification | Rollback |
|---|---:|---|---|---|---:|---|---|---|---|---|
| HR-001 | 4 | go/log-injection | high | internal/crowdsec/hub_sync.go | 579 | (s *HubService) Pull(ctx context.Context, slug string) (PullResult, error) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan + grep for raw log interpolations | Revert per-file sanitization patch |
| HR-002 | 4 | go/log-injection | high | internal/api/handlers/crowdsec_handler.go | 1110 | (h *CrowdsecHandler) PullPreset(c *gin.Context) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan + grep for raw log interpolations | Revert per-file sanitization patch |
| HR-003 | 3 | go/log-injection | high | internal/crowdsec/console_enroll.go | 213 | (s *ConsoleEnrollmentService) Enroll(ctx context.Context, req ConsoleEnrollRequest) (ConsoleEnrollmentStatus, error) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan + grep for raw log interpolations | Revert per-file sanitization patch |
| HR-004 | 2 | go/log-injection | high | internal/crowdsec/hub_sync.go | 793 | (s *HubService) refreshCache(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-005 | 2 | go/log-injection | high | internal/crowdsec/hub_sync.go | 720 | (s *HubService) fetchWithFallback(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-006 | 2 | go/log-injection | high | internal/crowdsec/hub_sync.go | 641 | (s *HubService) Apply(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-007 | 2 | go/log-injection | high | internal/crowdsec/hub_sync.go | 571 | (s *HubService) Pull(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-008 | 2 | go/log-injection | high | internal/crowdsec/hub_sync.go | 567 | (s *HubService) Pull(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-009 | 2 | go/log-injection | high | internal/crowdsec/console_enroll.go | 246 | (s *ConsoleEnrollmentService) Enroll(...) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-010 | 2 | go/log-injection | high | internal/cerberus/cerberus.go | 244 | (c *Cerberus) Middleware() gin.HandlerFunc | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-011 | 2 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 496 | (h *ProxyHostHandler) Update(c *gin.Context) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-012 | 2 | go/log-injection | high | internal/api/handlers/crowdsec_handler.go | 1216 | (h *CrowdsecHandler) ApplyPreset(c *gin.Context) | Unsanitized user-controlled data interpolated into logs | Wrap tainted fields with `utils.SanitizeForLog` or `%q`; avoid raw concatenation | Go unit tests + CodeQL Go scan | Revert per-file sanitization patch |
| HR-013 | 1 | js/regex/missing-regexp-anchor | high | tests/tasks/import-caddyfile.spec.ts | 324 | import-caddyfile paste test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Playwright/Vitest + CodeQL JS scan | Revert regex patch |
| HR-014 | 1 | js/regex/missing-regexp-anchor | high | tests/tasks/import-caddyfile.spec.ts | 307 | import-caddyfile upload test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Playwright/Vitest + CodeQL JS scan | Revert regex patch |
| HR-015 | 1 | js/regex/missing-regexp-anchor | high | tests/security-enforcement/zzz-caddy-imports/caddy-import-cross-browser.spec.ts | 204 | caddy import cross-browser test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Playwright/Vitest + CodeQL JS scan | Revert regex patch |
| HR-016 | 1 | js/regex/missing-regexp-anchor | high | frontend/src/pages/__tests__/ProxyHosts-progress.test.tsx | 141 | proxy hosts progress test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Vitest + CodeQL JS scan | Revert regex patch |
| HR-017 | 1 | js/regex/missing-regexp-anchor | high | frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx | 310 | CSP report-only test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Vitest + CodeQL JS scan | Revert regex patch |
| HR-018 | 1 | js/regex/missing-regexp-anchor | high | frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx | 298 | CSP report-only test | Regex host match not anchored | Add `^...$` anchors and explicit host escape | Targeted Vitest + CodeQL JS scan | Revert regex patch |
| HR-019 | 1 | js/insecure-temporary-file | high | tests/fixtures/auth-fixtures.ts | 181 | saveTokenCache helper | Temp file created in shared OS temp dir | Use `fs.mkdtemp` + `0o600` + deterministic cleanup | Fixture tests + CodeQL JS scan | Revert temp-file patch |
| HR-020 | 1 | js/insecure-temporary-file | high | tests/fixtures/auth-fixtures.ts | 129 | acquireLock helper | Temp file created in shared OS temp dir | Use `fs.mkdtemp` + `0o600` + deterministic cleanup | Fixture tests + CodeQL JS scan | Revert temp-file patch |
| HR-021 | 1 | js/insecure-temporary-file | high | tests/fixtures/auth-fixtures.ts | 107 | acquireLock helper | Temp file created in shared OS temp dir | Use `fs.mkdtemp` + `0o600` + deterministic cleanup | Fixture tests + CodeQL JS scan | Revert temp-file patch |
| HR-022 | 1 | go/log-injection | high | internal/api/handlers/backup_handler.go | 104 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-023 | 1 | go/log-injection | high | internal/api/handlers/crowdsec_handler.go | 1102 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-024 | 1 | go/log-injection | high | internal/api/handlers/crowdsec_handler.go | 1115 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-025 | 1 | go/log-injection | high | internal/api/handlers/crowdsec_handler.go | 1119 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-026 | 1 | go/log-injection | high | internal/api/handlers/docker_handler.go | 59 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-027 | 1 | go/log-injection | high | internal/api/handlers/docker_handler.go | 74 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-028 | 1 | go/log-injection | high | internal/api/handlers/docker_handler.go | 82 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-029 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 104 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-030 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 113 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-031 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 128 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-032 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 144 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-033 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 160 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-034 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 182 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-035 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 199 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-036 | 1 | go/log-injection | high | internal/api/handlers/emergency_handler.go | 92 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-037 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 459 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-038 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 468 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-039 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 472 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-040 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 474 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-041 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 477 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-042 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 481 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-043 | 1 | go/log-injection | high | internal/api/handlers/proxy_host_handler.go | 483 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-044 | 1 | go/log-injection | high | internal/api/handlers/security_handler.go | 1219 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-045 | 1 | go/log-injection | high | internal/api/handlers/settings_handler.go | 191 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-046 | 1 | go/log-injection | high | internal/api/handlers/uptime_handler.go | 103 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-047 | 1 | go/log-injection | high | internal/api/handlers/uptime_handler.go | 115 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-048 | 1 | go/log-injection | high | internal/api/handlers/uptime_handler.go | 64 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-049 | 1 | go/log-injection | high | internal/api/handlers/uptime_handler.go | 75 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-050 | 1 | go/log-injection | high | internal/api/handlers/uptime_handler.go | 82 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-051 | 1 | go/log-injection | high | internal/api/handlers/user_handler.go | 545 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-052 | 1 | go/log-injection | high | internal/api/middleware/emergency.go | 106 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-053 | 1 | go/log-injection | high | internal/api/middleware/emergency.go | 79 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-054 | 1 | go/log-injection | high | internal/cerberus/cerberus.go | 154 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-055 | 1 | go/log-injection | high | internal/cerberus/rate_limit.go | 128 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-056 | 1 | go/log-injection | high | internal/cerberus/rate_limit.go | 205 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-057 | 1 | go/log-injection | high | internal/crowdsec/console_enroll.go | 229 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-058 | 1 | go/log-injection | high | internal/crowdsec/hub_cache.go | 110 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-059 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 575 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-060 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 629 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-061 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 715 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-062 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 771 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-063 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 774 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-064 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 777 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-065 | 1 | go/log-injection | high | internal/crowdsec/hub_sync.go | 790 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-066 | 1 | go/log-injection | high | internal/server/emergency_server.go | 111 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-067 | 1 | go/log-injection | high | internal/services/backup_service.go | 685 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-068 | 1 | go/log-injection | high | internal/services/emergency_token_service.go | 128 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-069 | 1 | go/log-injection | high | internal/services/emergency_token_service.go | 303 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-070 | 1 | go/log-injection | high | internal/services/mail_service.go | 616 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-071 | 1 | go/log-injection | high | internal/services/manual_challenge_service.go | 184 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-072 | 1 | go/log-injection | high | internal/services/manual_challenge_service.go | 211 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-073 | 1 | go/log-injection | high | internal/services/manual_challenge_service.go | 286 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-074 | 1 | go/log-injection | high | internal/services/manual_challenge_service.go | 355 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |
| HR-075 | 1 | go/log-injection | high | internal/services/uptime_service.go | 1090 | Sanitized logging at sink (context in baseline export) | Unsanitized user-influenced value reaches log sink | Apply `utils.SanitizeForLog(...)` and structured logging placeholders; avoid raw concatenation | CodeQL Go scan (CI-aligned) + targeted go test for touched package + grep check for raw interpolations | Revert file-local sanitization commit owned by backend phase lead |

#### Matrix B — Current PR #718 open findings (per-file ownership)

| Rule | Severity | Count | File | Alert IDs | Owner role | Root cause hypothesis | Fix pattern | Verification | Rollback |
|---|---|---:|---|---|---|---|---|---|---|
| js/automatic-semicolon-insertion | note | 1 | frontend/src/pages/__tests__/ProxyHosts-bulk-acl.test.tsx | 1248 | Frontend test owner | ASI-sensitive multiline statements in tests | Add explicit semicolons / wrap expressions | Targeted test files + CodeQL JS scan | Revert syntax-only commit |
| js/automatic-semicolon-insertion | note | 3 | tests/core/navigation.spec.ts | 1251,1250,1249 | E2E owner | ASI-sensitive multiline statements in tests | Add explicit semicolons / wrap expressions | Targeted test files + CodeQL JS scan | Revert syntax-only commit |
| js/comparison-between-incompatible-types | warning | 1 | frontend/src/components/CredentialManager.tsx | 1247 | Frontend owner | Incompatible operand types in `CredentialManager` | Normalize types before compare; add type guard | Unit test + `npm run type-check` + CodeQL JS scan | Revert isolated type fix |
| js/unused-local-variable | note | 1 | tests/global-setup.ts | 1156 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 4 | tests/integration/import-to-production.spec.ts | 1155,1154,1153,1152 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 5 | tests/integration/multi-feature-workflows.spec.ts | 1162,1160,1159,1158,1157 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 4 | tests/integration/proxy-certificate.spec.ts | 1170,1164,1163,1161 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 5 | tests/integration/proxy-dns-integration.spec.ts | 1169,1168,1167,1166,1165 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/modal-dropdown-triage.spec.ts | 1171 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/monitoring/uptime-monitoring.spec.ts | 1173 | E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/reporters/debug-reporter.ts | 1172 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security-enforcement/combined-enforcement.spec.ts | 1194 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/security-enforcement/emergency-server/emergency-server.spec.ts | 1196,1195 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security-enforcement/emergency-token.spec.ts | 1197 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security-enforcement/zzz-caddy-imports/caddy-import-firefox.spec.ts | 1198 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security-enforcement/zzz-caddy-imports/caddy-import-webkit.spec.ts | 1199 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 6 | tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts | 1217,1213,1205,1204,1203,1202 | Security UI owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts | 1201,1200 | Security UI owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 3 | tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts | 1215,1214,1209 | Security UI owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 7 | tests/security-enforcement/zzz-security-ui/real-time-logs.spec.ts | 1216,1212,1211,1210,1208,1207,1206 | Security UI owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts | 1219,1218 | Security UI owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security-enforcement/zzzz-break-glass-recovery.spec.ts | 1220 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 8 | tests/security/acl-integration.spec.ts | 1184,1183,1182,1181,1180,1179,1178,1177 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security/audit-logs.spec.ts | 1175 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security/crowdsec-config.spec.ts | 1174 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security/crowdsec-decisions.spec.ts | 1179 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security/rate-limiting.spec.ts | 1185 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/security/security-headers.spec.ts | 1186 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 4 | tests/security/suite-integration.spec.ts | 1190,1189,1188,1187 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 3 | tests/security/waf-config.spec.ts | 1193,1192,1191 | Security E2E owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 5 | tests/settings/account-settings.spec.ts | 1227,1226,1224,1222,1221 | Settings test owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/settings/notifications.spec.ts | 1233,1225 | Settings test owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/settings/smtp-settings.spec.ts | 1223 | Settings test owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/settings/user-management.spec.ts | 1235,1234 | Settings test owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 3 | tests/tasks/backups-create.spec.ts | 1230,1229,1228 | Task flow owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/tasks/backups-restore.spec.ts | 1232,1231 | Task flow owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 2 | tests/tasks/import-caddyfile.spec.ts | 1237,1236 | Task flow owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/tasks/logs-viewing.spec.ts | 1238 | Task flow owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 3 | tests/utils/archive-helpers.ts | 1241,1240,1239 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/utils/debug-logger.ts | 1243 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/utils/diagnostic-helpers.ts | 1242 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/utils/phase5-helpers.ts | 1244 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/utils/test-steps.ts | 1245 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |
| js/unused-local-variable | note | 1 | tests/utils/wait-helpers.spec.ts | 1246 | QA tooling owner | Test helper variables/imports retained after refactors | Remove dead locals/imports; enforce lint gate | `npm run lint`, `npm run type-check`, CodeQL JS scan | Revert individual cleanup commits |

### Baseline Freshness Gate (Mandatory before each PR slice)

1. Re-pull PR #718 open alerts immediately before opening/updating PR-1, PR-2, and PR-3.
2. Compare fresh snapshot against frozen baseline (`docs/reports/pr718_open_alerts_baseline.json`) by `alert_number`, `rule.id`, `location.path`, and `location.start_line`.
3. If drift is detected (new alert, missing alert, rule/line migration), planning fails closed and matrices must be regenerated before implementation proceeds.
4. Persist each freshness run to `docs/reports/pr718_open_alerts_freshness_<timestamp>.json` and add a delta summary in `docs/reports/`.

Drift policy:

- `No drift`: proceed with current phase.
- `Additive drift`: block and expand Matrix A/B ownership before coding.
- `Subtractive drift`: verify closure source (already fixed vs query change) and update baseline evidence.

### Disposition Workflow (false-positive / won't-fix / out-of-scope)

All non-fixed findings require an explicit disposition record, no exceptions.

Required record fields:

- Alert ID, rule ID, file, line, severity.
- Disposition (`false-positive`, `won't-fix`, `out-of-scope`).
- Technical justification (query semantics, unreachable path, accepted risk, or external ownership).
- Evidence link (code reference, scan artifact, upstream issue, or policy decision).
- Owner role, reviewer/approver, decision date, next review date.
- Audit trail entry in `docs/reports/codeql_pr718_dispositions.md`.

Disposition gating rules:

1. `false-positive`: requires reviewer approval and reproducible evidence.
2. `won't-fix`: requires explicit risk acceptance and rollback/mitigation note.
3. `out-of-scope`: requires linked issue/PR and target milestone.
4. Any undispositioned unresolved finding blocks phase closure.

### Implementation Plan (Phase ↔ PR mapped execution)

#### Phase metadata (ownership, ETA, rollback)

| Phase | PR slice | Primary owner role | ETA | Rollback owner | Merge dependency |
|---|---|---|---|---|---|
| Phase 1: Baseline freeze and freshness gate | PR-0 (no code changes) | Security lead | 0.5 day | Security lead | none |
| Phase 2: Security remediations | PR-1 | Backend security owner | 2-3 days | Backend owner | Phase 1 complete |
| Phase 3: Open alert cleanup | PR-2 | Frontend/E2E owner | 1-2 days | Frontend owner | PR-1 merged |
| Phase 4: Hygiene and scanner hardening | PR-3 | DevEx/CI owner | 1 day | DevEx owner | PR-1 and PR-2 merged |
| Phase 5: Final verification and closure | Post PR-3 | Release/security lead | 0.5 day | Release lead | PR-3 merged |

#### Phase 1 — Baseline freeze and freshness gate (PR-0)

Deliverables:

- Freeze baseline artifacts:
  - `codeql-results-go.sarif`
  - `codeql-results-js.sarif`
  - `docs/reports/pr718_open_alerts_baseline.json`
- Confirm scanner parity and canonical artifact naming.

Tasks:

1. Confirm all scan entrypoints produce canonical SARIF names.
2. Re-run CodeQL Go/JS scans locally with CI-aligned tasks.
3. Store pre-remediation summary in `docs/reports/`.
4. Run freshness gate and block if baseline drift is detected.

#### Phase 2 — Security-first remediation (PR-1)

Scope:

- `go/log-injection` units `HR-001`..`HR-075`
- `js/regex/missing-regexp-anchor` units `HR-013`..`HR-018`
- `js/insecure-temporary-file` units `HR-019`..`HR-021`

Tasks:

1. Patch backend log sinks file-by-file using consistent sanitization helper policy.
2. Patch regex patterns in affected test/component files with anchors.
3. Patch temp-file helpers in `tests/fixtures/auth-fixtures.ts`.
4. Run targeted tests after each module group to isolate regressions.
5. Re-run freshness gate before merge to ensure matrix parity.

#### Phase 3 — Quality cleanup (PR-2)

Scope:

- 100 current open findings (`js/unused-local-variable`, `js/automatic-semicolon-insertion`, `js/comparison-between-incompatible-types`) using Matrix B ownership rows.

Tasks:

1. Remove unused vars/imports by directory cluster (`tests/utils`, `tests/security*`, `tests/integration*`, `tests/settings*`, etc.).
2. Resolve ASI findings in:
  - `tests/core/navigation.spec.ts`
  - `frontend/src/pages/__tests__/ProxyHosts-bulk-acl.test.tsx`
3. Resolve type comparison warning in:
  - `frontend/src/components/CredentialManager.tsx`
4. Record dispositions for any non-fixed findings.

#### Phase 4 — Hygiene and scanner hardening (PR-3)

Tasks:

1. Normalize `.gitignore`/`.dockerignore` scan artifact handling and remove duplication.
2. Select one canonical Codecov config path and deprecate the other.
3. Normalize scan task outputs in `.vscode/tasks.json` and `scripts/pre-commit-hooks/` if required.
4. Re-run freshness gate before merge to confirm no PR #718 drift.

#### Phase 5 — Final verification and closure (post PR-3)

Tasks:

1. Run E2E-first verification path.
2. If runtime inputs changed (`backend/**`, `frontend/**`, `go.mod`, `go.sum`, `package.json`, `package-lock.json`, `Dockerfile`, `.docker/**`, compose files), rebuild E2E environment before running Playwright.
3. Run CodeQL Go/JS scans and validate zero high/critical findings.
4. Run coverage gates and type checks.
5. Confirm no SARIF/db artifacts are accidentally committed.
6. Update remediation report with before/after counts and close PR #718 checklist.

### Phase-to-PR Merge Dependency Contract

1. PR-1 cannot open until Phase 1 baseline and freshness gate pass.
2. PR-2 cannot merge until PR-1 merges and a fresh alert snapshot confirms no drift.
3. PR-3 cannot merge until PR-1 and PR-2 both merge and freshness gate passes again.
4. Phase 5 closure is blocked until all three PRs are merged and disposition log is complete.

### PR Slicing Strategy

#### Decision

Use **three PRs** (minimum safe split). Single-PR delivery is rejected due to:

- cross-domain blast radius (backend + frontend + test infra + CI hygiene),
- security-critical codepaths,
- reviewer load and rollback risk.

#### PR-1 — Security remediations only (high risk)

Scope:

- Backend `go/log-injection` hotspots (`HR-001`..`HR-075`)
- Frontend/test security hotspots (`HR-013`..`HR-021`)

Primary files:

- `backend/internal/api/handlers/*`
- `backend/internal/api/middleware/emergency.go`
- `backend/internal/cerberus/*`
- `backend/internal/crowdsec/*`
- `backend/internal/server/emergency_server.go`
- `backend/internal/services/*`
- `tests/fixtures/auth-fixtures.ts`
- `tests/tasks/import-caddyfile.spec.ts`
- `tests/security-enforcement/zzz-caddy-imports/caddy-import-cross-browser.spec.ts`
- `frontend/src/components/__tests__/SecurityHeaderProfileForm.test.tsx`
- `frontend/src/pages/__tests__/ProxyHosts-progress.test.tsx`

Dependencies:

- Phase 1 baseline freeze and freshness gate must be complete.

Acceptance criteria:

1. No remaining `go/log-injection`, `js/regex/missing-regexp-anchor`, `js/insecure-temporary-file` findings in fresh scan.
2. Targeted tests pass for modified suites.
3. No behavior regressions in emergency/security control flows.

Rollback:

- Revert by module batch (handlers, services, crowdsec, tests) to isolate regressions.

#### PR-2 — Open alert cleanup (quality/non-blocking)

Scope:

- `js/unused-local-variable` (95)
- `js/automatic-semicolon-insertion` (4)
- `js/comparison-between-incompatible-types` (1)

Dependencies:

- PR-1 merged (required).

Acceptance criteria:

1. `codeql-results-js.sarif` shows zero of the three rules above.
2. `npm run lint` and `npm run type-check` pass.
3. Playwright/Vitest suites touched by cleanup pass.

Rollback:

- Revert by directory cluster commits (`tests/utils`, `tests/security*`, etc.).

#### PR-3 — Hygiene and scanner hardening

Scope:

- `.gitignore`
- `.dockerignore`
- `codecov.yml`
- `.codecov.yml`
- Optional: normalize scan task outputs in `.vscode/tasks.json` and `scripts/pre-commit-hooks/`

Dependencies:

- PR-1 and PR-2 complete.

Acceptance criteria:

1. No duplicate/contradictory ignore patterns that mask source or commit scan artifacts unexpectedly.
2. Single canonical Codecov config path selected (either keep `codecov.yml` and deprecate `.codecov.yml`, or vice-versa).
3. Docker context excludes scan/report artifacts but preserves required runtime/build inputs.

Rollback:

- Revert config-only commit; no application runtime risk.

### PR-3 Addendum — `js/insecure-temporary-file` in auth token cache

#### Scope and intent

This addendum defines the concrete remediation plan for the CodeQL `js/insecure-temporary-file` pattern in `tests/fixtures/auth-fixtures.ts`, focused on token cache logic that currently persists refreshed auth tokens to temporary files (`token.lock`, `token.json`) under OS temp storage.

#### 1) Root cause analysis

- The fixture stores bearer tokens on disk in a temp location, which is unnecessary for test execution and increases secret exposure risk.
- Even with restrictive permissions and lock semantics, the pattern still relies on filesystem primitives in a shared temp namespace and is flagged as insecure temporary-file usage.
- The lock/cache design uses predictable filenames (`token.lock`, `token.json`) and file lifecycle management; this creates avoidable risk and complexity for what is effectively process-local test state.
- The vulnerability is in the storage approach, not only in file flags/permissions; therefore suppression is not an acceptable fix.

#### 2) Recommended proper fix (no suppression)

- Replace file-based token cache + lock with an in-memory cache guarded by an async mutex/serialization helper.
- Keep existing behavior contract intact:
  - cached token reuse while valid,
  - refresh when inside threshold,
  - safe concurrent calls to `refreshTokenIfNeeded`.
- Remove all temp-directory/file operations from the token-cache path.
- Preserve JWT expiry extraction and fallback behavior when refresh fails.

Design target:

- `TokenCache` remains as a module-level in-memory object.
- Introduce a module-level promise-queue lock helper (single-writer section) to serialize read/update operations.
- `readTokenCache` / `saveTokenCache` become in-memory helpers only.

#### 3) Exact files/functions to edit

- `tests/fixtures/auth-fixtures.ts`
  - Remove/replace file-based helpers:
    - `getTokenCacheFilePath`
    - `getTokenLockFilePath`
    - `cleanupTokenCacheDir`
    - `ensureCacheDir`
    - `acquireLock`
  - Refactor:
    - `readTokenCache` (memory-backed)
    - `saveTokenCache` (memory-backed)
    - `refreshTokenIfNeeded` (use in-memory lock path; no filesystem writes)
  - Remove unused imports/constants tied to temp files (`fs`, `path`, `os`, lock/cache file constants).

- `tests/fixtures/token-refresh-validation.spec.ts`
  - Update concurrency test intent text from file-lock semantics to in-memory serialized access semantics.
  - Keep behavioral assertions (valid token, no corruption/no throw under concurrent refresh requests).

- `docs/reports/pr718_open_alerts_freshness_<timestamp>.md` (or latest freshness report in `docs/reports/`)
  - Add a PR-3 note that the insecure temp-file finding for auth-fixtures moved to memory-backed token caching and is expected to close in next scan.

#### 4) Acceptance criteria

- CodeQL JavaScript scan reports zero `js/insecure-temporary-file` findings for `tests/fixtures/auth-fixtures.ts`.
- No auth token artifacts (`token.json`, `token.lock`, or `charon-test-token-cache-*`) are created by token refresh tests.
- `refreshTokenIfNeeded` still supports concurrent calls without token corruption or unhandled errors.
- `tests/fixtures/token-refresh-validation.spec.ts` passes in targeted execution.
- No regression to authentication fixture consumers using `refreshTokenIfNeeded`.

#### 5) Targeted verification commands (no full E2E suite)

- Targeted fixture tests:
  - `cd /projects/Charon && npx playwright test tests/fixtures/token-refresh-validation.spec.ts --project=firefox`

- Targeted static check for removed temp-file pattern:
  - `cd /projects/Charon && rg "tmpdir\(|token\.lock|token\.json|mkdtemp" tests/fixtures/auth-fixtures.ts`

- Targeted JS security scan (CI-aligned task):
  - VS Code task: `Security: CodeQL JS Scan (CI-Aligned) [~90s]`
  - or CLI equivalent: `cd /projects/Charon && pre-commit run --hook-stage manual codeql-js-scan --all-files`

- Targeted freshness evidence generation:
  - `cd /projects/Charon && ls -1t docs/reports/pr718_open_alerts_freshness_*.md | head -n 1`

#### 6) PR-3 documentation/report updates required

- Keep this addendum in `docs/plans/current_spec.md` as the planning source of truth for the token-cache remediation.
- Update the latest PR-3 freshness report in `docs/reports/` to include:
  - finding scope (`js/insecure-temporary-file`, auth fixture token cache),
  - remediation approach (memory-backed cache, no disk token persistence),
  - verification evidence references (targeted Playwright + CodeQL JS scan).
- If PR-3 has a dedicated summary report, include a short “Security Remediation Delta” subsection with before/after status for this rule.

### Configuration Review and Suggested Updates

#### `.gitignore`

Observed issues:

- Duplicated patterns (`backend/main`, `codeql-linux64.zip`, `.docker/compose/docker-compose.test.yml` repeated).
- Broad ignores (`*.sarif`) acceptable, but duplicate SARIF patterns increase maintenance noise.
- Multiple planning/docs ignore entries may hide useful artifacts accidentally.

Suggested updates:

1. Deduplicate repeated entries.
2. Keep one CodeQL artifact block with canonical patterns.
3. Keep explicit allow-list comments for intentionally tracked plan/report docs.

#### `.dockerignore`

Observed issues:

- Broad `*.md` exclusion with exceptions is valid, but easy to break when docs are needed during build metadata steps.
- Both `codecov.yml` and `.codecov.yml` ignored (good), but duplicate conceptual config handling elsewhere remains.

Suggested updates:

1. Keep current exclusions for scan artifacts (`*.sarif`, `codeql-db*`).
2. Add explicit comment that only runtime-required docs are whitelisted (`README.md`, `CONTRIBUTING.md`, `LICENSE`).
3. Validate no required frontend/backend build file is accidentally excluded when adding new tooling.

#### `codecov.yml` and `.codecov.yml`

Observed issues:

- Two active Codecov configs create ambiguity.
- `codecov.yml` is richer and appears primary; `.codecov.yml` may be legacy overlap.

Suggested updates:

1. Choose one canonical config (recommended: `codecov.yml`).
2. Remove or archive `.codecov.yml` to avoid precedence confusion.
3. Ensure ignore patterns align with real source ownership and avoid suppressing legitimate production code coverage.

#### `Dockerfile`

Observed issues relative to CodeQL remediation scope:

- Large and security-focused already; no direct blocker for PR #718 findings.
- Potentially excessive complexity for fallback build paths can hinder deterministic scanning/debugging.

Suggested updates (non-blocking, PR-3 backlog):

1. Add a short “security patch policy” comment block for dependency pin rationale consistency.
2. Add CI check to verify `CADDY_VERSION`, `CROWDSEC_VERSION`, and pinned Go/node versions are in expected policy ranges.
3. Keep build deterministic and avoid hidden side-effects in fallback branches.

### Validation Strategy

Execution order (required):

1. E2E Playwright targeted suites for touched areas.
2. Local patch coverage report generation.
3. CodeQL Go + JS scans (CI-aligned).
4. Pre-commit fast hooks.
5. Backend/frontend coverage checks.
6. TypeScript type-check.

Success gates:

- Zero high/critical security findings.
- No regression in emergency/security workflow behavior.
- Codecov thresholds remain green.

### Acceptance Criteria

1. DoD checks complete without errors.
2. PR #718 high-risk findings remediated and verified.
3. Current open PR #718 findings remediated and verified.
4. Config hardening updates approved and merged.
5. Post-remediation evidence published in `docs/reports/` with before/after counts.

### Risks and Mitigations

- Risk: over-sanitizing logs reduces operational diagnostics.
  - Mitigation: preserve key context with safe quoting/sanitization and structured fields.
- Risk: regex anchor changes break tests with dynamic URLs.
  - Mitigation: update patterns with explicit optional groups and escape strategies.
- Risk: temp-file hardening affects test parallelism.
  - Mitigation: per-test unique temp dirs and teardown guards.
- Risk: cleanup PR introduces noisy churn.
  - Mitigation: file-cluster commits + narrow CI checks per cluster.

### Handoff

After user approval of this plan:

1. Execute PR-1 (security) first.
2. Execute PR-2 (quality/open findings) second.
3. Execute PR-3 (hygiene/config hardening) third.
4. Submit final supervisor review with linked evidence and closure checklist.
