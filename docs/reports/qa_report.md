# QA & Security Report

**Date:** 2026-02-07
**Status:** 🔴 FAILED
**Evaluator:** GitHub Copilot (QA Security Mode)

## Executive Summary

QA validation was **stopped** after frontend coverage tests failed. Remaining checks were not executed per stop-on-failure policy.

| Check | Status | Details |
| :--- | :--- | :--- |
| **Frontend Coverage** | 🔴 FAIL | Test failures; see verbatim output below |
| **TypeScript Check** | ⚪ NOT RUN | Stopped after frontend coverage failure |
| **Pre-commit Hooks** | ⚪ NOT RUN | Stopped after frontend coverage failure |
| **Linting (Go/Frontend/Markdown/Hadolint)** | ⚪ NOT RUN | Stopped after frontend coverage failure |
| **Security Scans** | ⚪ SKIPPED | Skipped by request |

---

## 1. Security Findings

### Security Scans - SKIPPED

### Frontend Coverage - FAILED

**Failure Output (verbatim):**
```
Terminal: Test: Frontend with Coverage (Charon)
Output:


[... PREVIOUS OUTPUT TRUNCATED ...]

ity header profile to selected hosts using bulk endpoint  349ms
     ✓ removes security header profile when "None" selected  391ms
     ✓ handles partial failure with appropriate toast  303ms
     ✓ resets state on modal close  376ms
     ✓ shows profile description when profile is selected  504ms
 ✓ src/pages/__tests__/Plugins.test.tsx (30 tests) 1828ms
 ✓ src/components/__tests__/DNSProviderSelector.test.tsx (29 tests) 292ms
 ↓ src/pages/__tests__/Security.audit.test.tsx (18 tests | 18 skipped)
 ✓ src/api/__tests__/presets.test.ts (26 tests) 26ms
 ↓ src/pages/__tests__/Security.errors.test.tsx (13 tests | 13 skipped)
 ✓ src/components/__tests__/SecurityHeaderProfileForm.test.tsx (17 tests) 1928ms
     ✓ should show security score  582ms
     ✓ should calculate score after debounce  536ms
 ↓ src/pages/__tests__/Security.dashboard.test.tsx (18 tests | 18 skipped)
 ✓ src/components/__tests__/CertificateStatusCard.test.tsx (24 tests) 267ms
 ✓ src/api/__tests__/dnsProviders.test.ts (30 tests) 33ms
 ✓ src/pages/__tests__/Uptime.spec.tsx (11 tests) 1047ms
 ✓ src/components/__tests__/LoadingStates.security.test.tsx (41 tests) 417ms
 ↓ src/pages/__tests__/Security.loading.test.tsx (12 tests | 12 skipped)
 ✓ src/data/__tests__/crowdsecPresets.test.ts (38 tests) 22ms
Error: Not implemented: navigation (except hash changes)
    at module.exports (/projects/Charon/frontend/node_modules/jsdom/lib/jsdom/browser/not-implemented.js:9:17)
    at navigateFetch (/projects/Charon/frontend/node_modules/jsdom/lib/jsdom/living/window/navigation.js:77:3)
    at exports.navigate (/projects/Charon/frontend/node_modules/jsdom/lib/jsdom/living/window/navigation.js:55:3)
    at Timeout._onTimeout (/projects/Charon/frontend/node_modules/jsdom/lib/jsdom/living/nodes/HTMLHyperlinkElementUtils
-impl.js:81:7)
    at listOnTimeout (node:internal/timers:581:17)
    at processTimers (node:internal/timers:519:7) undefined
stderr | src/pages/__tests__/AuditLogs.test.tsx > <AuditLogs /> > handles export error
Export error: Error: Export failed
    at /projects/Charon/frontend/src/pages/__tests__/AuditLogs.test.tsx:324:7
    at file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:145:11
    at file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:915:26
    at file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:1243:20
    at new Promise (<anonymous>)
    at runWithTimeout (file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:1209:10)
    at file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:1653:37
    at Traces.$ (file:///projects/Charon/frontend/node_modules/vitest/dist/chunks/traces.CCmnQaNT.js:142:27)
    at trace (file:///projects/Charon/frontend/node_modules/vitest/dist/chunks/test.B8ej_ZHS.js:239:21)
    at runTest (file:///projects/Charon/frontend/node_modules/@vitest/runner/dist/index.js:1653:12)

 ✓ src/pages/__tests__/AuditLogs.test.tsx (14 tests) 1219ms
 ✓ src/hooks/__tests__/useSecurity.test.tsx (19 tests) 1107ms
 ✓ src/hooks/__tests__/useSecurityHeaders.test.tsx (15 tests) 805ms
stdout | src/api/logs.test.ts > logs api > connects to live logs websocket and handles lifecycle events
Connecting to WebSocket: ws://localhost/api/v1/logs/live?level=error&source=cerberus
WebSocket connection established
WebSocket connection closed { code: 1000, reason: '', wasClean: true }

stderr | src/api/logs.test.ts > logs api > connects to live logs websocket and handles lifecycle events
WebSocket error: Event { isTrusted: [Getter] }

stdout | src/api/logs.test.ts > connectSecurityLogs > connects to cerberus logs websocket endpoint
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?

stdout | src/api/logs.test.ts > connectSecurityLogs > passes source filter to websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?source=waf

stdout | src/api/logs.test.ts > connectSecurityLogs > passes level filter to websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?level=error

stdout | src/api/logs.test.ts > connectSecurityLogs > passes ip filter to websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?ip=192.168

stdout | src/api/logs.test.ts > connectSecurityLogs > passes host filter to websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?host=example.com

stdout | src/api/logs.test.ts > connectSecurityLogs > passes blocked_only filter to websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?blocked_only=true

stdout | src/api/logs.test.ts > connectSecurityLogs > receives and parses security log entries
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket connection established

stdout | src/api/logs.test.ts > connectSecurityLogs > receives blocked security log entries
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket connection established

stdout | src/api/logs.test.ts > connectSecurityLogs > handles onOpen callback
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket connection established

stdout | src/api/logs.test.ts > connectSecurityLogs > handles onError callback
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?

stderr | src/api/logs.test.ts > connectSecurityLogs > handles onError callback
Cerberus logs WebSocket error: Event { isTrusted: [Getter] }

stdout | src/api/logs.test.ts > connectSecurityLogs > handles onClose callback
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket closed { code: 1000, reason: '', wasClean: true }

stdout | src/api/logs.test.ts > connectSecurityLogs > returns disconnect function that closes websocket
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket connection established
Cerberus logs WebSocket closed { code: 1000, reason: '', wasClean: true }

stdout | src/api/logs.test.ts > connectSecurityLogs > handles JSON parse errors gracefully
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?
Cerberus logs WebSocket connection established

stdout | src/api/logs.test.ts > connectSecurityLogs > uses wss protocol when on https
Connecting to Cerberus logs WebSocket: wss://secure.example.com/api/v1/cerberus/logs/ws?

stdout | src/api/logs.test.ts > connectSecurityLogs > combines multiple filters in websocket url
Connecting to Cerberus logs WebSocket: ws://localhost/api/v1/cerberus/logs/ws?source=waf&level=warn&ip=10.0.0&host=examp
le.com&blocked_only=true

 ✓ src/api/logs.test.ts (19 tests) 18ms
 ✓ src/pages/__tests__/Security.spec.tsx (6 tests) 613ms
 ❯ src/pages/__tests__/AccessLists.test.tsx (5 tests | 3 failed) 4121ms
     ✓ renders empty state and opens create form 168ms
     ✓ shows CGNAT warning and allows dismiss 74ms
     × deletes access list with backup 1188ms
     × bulk deletes selected access lists 1282ms
     × tests IP against access list 1407ms
 ✓ src/components/__tests__/ProxyHostForm-dns.test.tsx (15 tests) 14889ms
       ✓ detects *.example.com as wildcard  1330ms
       ✓ does not detect sub.example.com as wildcard  632ms
       ✓ detects multiple wildcards in comma-separated list  1525ms
       ✓ detects wildcard at start of comma-separated list  1276ms
       ✓ shows DNS provider selector when wildcard domain entered  638ms
       ✓ shows info alert explaining DNS-01 requirement  645ms
       ✓ shows validation error on submit if wildcard without provider  1976ms
       ✓ does not show DNS provider selector without wildcard  711ms
       ✓ DNS provider selector is present for wildcard domains  542ms
       ✓ clears DNS provider when switching to non-wildcard  1362ms
       ✓ preserves form state during wildcard domain edits  1027ms
       ✓ includes dns_provider_id null for non-wildcard domains  1506ms
       ✓ prevents submission when wildcard present without DNS provider  1446ms
 ✓ src/hooks/__tests__/usePlugins.test.tsx (15 tests) 842ms
 ❯ src/components/__tests__/Layout.test.tsx (16 tests | 1 failed) 1129ms
     ✓ renders the application logo 125ms
     × renders all navigation items 302ms
     ✓ renders children content 26ms
     ✓ displays version information 50ms
     ✓ calls logout when logout button is clicked 140ms
     ✓ toggles sidebar on mobile 73ms
     ✓ persists collapse state to localStorage 63ms
     ✓ restores collapsed state from localStorage on load 25ms
       ✓ displays Security nav item when Cerberus is enabled 25ms
       ✓ hides Security nav item when Cerberus is disabled 49ms
       ✓ displays Uptime nav item when Uptime is enabled 38ms
       ✓ hides Uptime nav item when Uptime is disabled 46ms
       ✓ shows Security and Uptime when both features are enabled 55ms
       ✓ hides both Security and Uptime when both features are disabled 66ms
       ✓ defaults to showing Security and Uptime when feature flags are loading 24ms
       ✓ shows other nav items regardless of feature flags 19ms
 ✓ src/components/ui/__tests__/DataTable.test.tsx (19 tests) 450ms
 ✓ src/pages/__tests__/SMTPSettings.test.tsx (10 tests) 2655ms
     ✓ saves SMTP settings successfully  724ms
     ✓ sends test email  312ms
     ✓ surfaces backend validation errors on save  375ms
     ✓ disables test connection until required fields are set and shows error toast on failure  487ms
     ✓ handles test email failures and keeps input value intact  375ms
 ✓ src/components/__tests__/SecurityNotificationSettingsModal.test.tsx (13 tests) 1103ms
     ✓ submits updated settings  372ms
 ✓ src/hooks/__tests__/useCredentials.test.tsx (16 tests) 242ms
 ✓ src/pages/__tests__/Uptime.test.tsx (9 tests) 587ms
 ✓ src/hooks/__tests__/useRemoteServers.test.tsx (10 tests) 774ms
 ✓ src/api/auditLogs.test.ts (14 tests) 13ms
 ✓ src/api/__tests__/security.test.ts (16 tests) 13ms
 ✓ src/pages/__tests__/Login.overlay.audit.test.tsx (7 tests) 3011ms
     ✓ shows coin-themed overlay during login  650ms
     ✓ ATTACK: rapid fire login attempts are blocked by overlay  499ms
     ✓ ATTACK: XSS in login credentials does not break overlay  788ms
     ✓ ATTACK: network timeout does not leave overlay stuck  323ms
 ✓ src/hooks/__tests__/useNotifications.test.tsx (9 tests) 541ms
 ✓ src/pages/__tests__/CrowdSecConfig.test.tsx (7 tests) 1928ms
     ✓ allows reading and saving config files  582ms
     ✓ allows banning an IP  581ms
 ✓ src/pages/__tests__/EncryptionManagement.test.tsx (14 tests) 1237ms
 ✓ src/components/__tests__/WebSocketStatusCard.test.tsx (8 tests) 432ms
 ✓ src/components/__tests__/CSPBuilder.test.tsx (13 tests) 788ms
 ✓ src/components/__tests__/DNSProviderForm.test.tsx (7 tests) 2406ms
     ✓ handles form submission for creation  743ms
     ✓ tests connection  553ms
     ✓ handles test connection failure  402ms
 ✓ src/hooks/__tests__/useManualChallenge.test.tsx (11 tests) 236ms
 ✓ src/components/__tests__/ImportReviewTable.test.tsx (9 tests) 463ms
 ✓ src/pages/__tests__/RateLimiting.spec.tsx (9 tests) 389ms
 ✓ src/components/ui/Tabs.test.tsx (10 tests) 406ms
 ✓ src/components/import/__tests__/FileUploadSection.test.tsx (9 tests) 651ms
     ✓ rejects files over 5MB limit  415ms
 ✓ src/components/__tests__/CertificateList.test.tsx (6 tests) 365ms
 ✓ src/hooks/__tests__/useProxyHosts.test.tsx (8 tests) 492ms
 ✓ src/components/__tests__/DNSDetectionResult.test.tsx (10 tests) 210ms
 ✓ src/api/__tests__/users.test.ts (10 tests) 14ms
 ✓ src/api/__tests__/manualChallenge.test.ts (14 tests) 13ms
stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > creates WebSocket connection with corre
ct URL
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > uses wss protocol when page is https
Connecting to WebSocket: wss://example.com/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > includes filters in query parameters
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?level=error&source=waf

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > calls onMessage callback when message i
s received
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > handles JSON parse errors gracefully
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > returns a close function that closes th
e WebSocket
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > does not throw when closing already clo
sed connection
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > handles missing optional callbacks
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stderr | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > handles missing optional callbacks
WebSocket error: Event { isTrusted: [Getter] }

stdout | src/api/__tests__/logs-websocket.test.ts > logs API - connectLiveLogs > processes multiple messages in sequence
Connecting to WebSocket: ws://localhost:8080/api/v1/logs/live?

stdout | src/api/__tests__/logs-websocket.test.ts
WebSocket connection closed { code: 1000, reason: '', wasClean: true }

 ✓ src/api/__tests__/logs-websocket.test.ts (11 tests | 2 skipped) 30ms
 ✓ src/hooks/__tests__/useDNSDetection.test.tsx (10 tests) 680ms
 ✓ src/components/__tests__/CrowdSecBouncerKeyDisplay.test.tsx (13 tests | 4 skipped) 198ms
 ✓ src/pages/__tests__/ProxyHosts-coverage-isolated.test.tsx (3 tests) 1504ms
     ✓ renders SSL staging badge, websocket badge  634ms
     ✓ bulk apply merges host data and calls updateHost  628ms
 ✓ src/components/__tests__/ImportReviewTable-warnings.test.tsx (7 tests) 337ms
 ✓ src/api/__tests__/settings.test.ts (16 tests) 11ms
 ✓ src/pages/__tests__/AcceptInvite.test.tsx (8 tests) 1368ms
     ✓ shows password mismatch error  315ms
     ✓ submits form and shows success  522ms
     ✓ shows error on submit failure  336ms
 ✓ src/components/__tests__/RemoteServerForm.test.tsx (9 tests) 726ms
 ✓ src/components/ui/__tests__/Skeleton.test.tsx (18 tests) 233ms
 ✓ src/components/__tests__/CrowdSecKeyWarning.test.tsx (8 tests) 380ms
 ✓ src/pages/__tests__/ProxyHosts-progress.test.tsx (2 tests) 902ms
     ✓ shows progress when applying multiple ACLs  819ms
 ✓ src/api/notifications.test.ts (5 tests) 10ms
 ✓ src/pages/__tests__/ImportCaddy-multifile-modal.test.tsx (9 tests) 359ms
 ✓ src/pages/__tests__/ProxyHosts-bulk-apply.test.tsx (3 tests) 1241ms
     ✓ shows Bulk Apply button when hosts selected and opens modal  497ms
     ✓ applies selected settings to all selected hosts by calling updateProxyHost merged payload  429ms
     ✓ cancels bulk apply modal when Cancel clicked  313ms
 ✓ src/components/ui/__tests__/Alert.test.tsx (18 tests) 210ms
 ✓ src/data/__tests__/securityPresets.test.ts (24 tests) 11ms
 ✓ src/pages/__tests__/ImportCaddy-warnings.test.tsx (6 tests) 82ms
 ✓ src/hooks/__tests__/useAccessLists.test.tsx (6 tests) 361ms
 ✓ src/components/__tests__/PermissionsPolicyBuilder.test.tsx (8 tests) 723ms
 ✓ src/components/ui/__tests__/StatsCard.test.tsx (14 tests) 245ms


 ❯ src/components/__tests__/NotificationCenter.test.tsx 2/6
 ❯ src/components/ui/__tests__/Input.test.tsx 10/16

 Test Files 4 failed | 83 passed | 5 skipped (153)
      Tests 39 failed | 1397 passed | 90 skipped (1536)
   Start at 04:46:11
   Duration 109.78s
```
 ❯ src/pages/__tests__/ImportCrowdSec.spec.tsx 1/1

 Test Files 8 failed | 129 passed | 5 skipped (153)
    Tests 44 failed | 1669 passed | 90 skipped (1803)
   Start at 04:32:30
   Duration 119.07s
```

---

## 3. Completed Checks

No other checks were executed.

---

## 4. Deferred Checks (Not Run)

The following checks were **not executed** due to the frontend coverage failure:
- TypeScript type check
- Pre-commit hooks
- Linting (Go vet, staticcheck, frontend lint, markdownlint, hadolint)

---

## 5. Next Actions Required

1. Fix the failing frontend tests and rerun frontend coverage.
2. Resume deferred QA checks once frontend coverage passes.

---

## Accepted Risks

- Security scans skipped for this run per instruction; CVE risk accepted temporarily. Re-run when risk acceptance ends.
