# QA & Security Report

**Date:** 2026-02-08
**Status:** 🔴 FAILED
**Evaluator:** GitHub Copilot (QA Security Mode)

## Executive Summary

QA validation ran per Definition of Done. Failures are listed below with verbatim output.

| Check | Status | Details |
| :--- | :--- | :--- |
| **Docker: Rebuild E2E Environment** | 🟢 PASS | Completed |
| **Playwright E2E (All Browsers)** | 🔴 FAIL | Container not ready after 30000ms |
| **Backend Coverage** | 🟢 PASS | Skill reported success |
| **Frontend Coverage** | 🔴 FAIL | Test failures; see output |
| **TypeScript Check** | 🟢 PASS | `tsc --noEmit` completed |
| **Pre-commit Hooks** | 🟢 PASS | Hooks passed |
| **Lint: Frontend** | 🟢 PASS | `eslint . --report-unused-disable-directives` completed |
| **Lint: Go Vet** | 🟢 PASS | No errors reported |
| **Lint: Staticcheck (Fast)** | 🟢 PASS | `0 issues.` |
| **Lint: Markdownlint** | 🟢 PASS | No errors reported |
| **Lint: Hadolint Dockerfile** | 🔴 FAIL | DL3008, DL4006, SC2015 warnings |
| **Security: Trivy Scan (filesystem)** | 🟢 PASS | No issues found |
| **Security: Docker Image Scan (Local)** | 🔴 FAIL | 0 critical, 8 high vulnerabilities |
| **Security: CodeQL Go Scan (CI-Aligned)** | 🟢 PASS | Completed (output truncated) |
| **Security: CodeQL JS Scan (CI-Aligned)** | 🟢 PASS | Completed (output truncated) |

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
## 1. Validation Results

### Playwright E2E - FAILED

**Failure Output (verbatim):**
```
> e2e:all
> PLAYWRIGHT_HTML_OPEN=never npx playwright test

[dotenv@17.2.4] injecting env (2) from .env -- tip: ⚙️  override existing env vars with { override: true }

🧹 Running global test setup...

🔐 Validating emergency token configuration...
    🔑 Token present: f51dedd6...346b
    ✓ Token length: 64 chars (valid)
    ✓ Token format: Valid hexadecimal
    ✓ Token appears to be unique (not a placeholder)
✅ Emergency token validation passed

📍 Base URL: http://127.0.0.1:8080
⏳ Waiting for container to be ready at http://127.0.0.1:8080...
    ⏳ Waiting for container... (1/15)
    ⏳ Waiting for container... (2/15)
    ⏳ Waiting for container... (3/15)
    ⏳ Waiting for container... (4/15)
    ⏳ Waiting for container... (5/15)
    ⏳ Waiting for container... (6/15)
    ⏳ Waiting for container... (7/15)
    ⏳ Waiting for container... (8/15)
    ⏳ Waiting for container... (9/15)
    ⏳ Waiting for container... (10/15)
    ⏳ Waiting for container... (11/15)
    ⏳ Waiting for container... (12/15)
    ⏳ Waiting for container... (13/15)
    ⏳ Waiting for container... (14/15)
    ⏳ Waiting for container... (15/15)
Error: Container failed to start after 30000ms

     at global-setup.ts:158

    156 |     }
    157 |   }
> 158 |   throw new Error(`Container failed to start after ${maxRetries * delayMs}ms`);
            |         ^
    159 | }
    160 |
    161 | /**
        at waitForContainer (/projects/Charon/tests/global-setup.ts:158:9)
        at globalSetup (/projects/Charon/tests/global-setup.ts:203:3)


To open last HTML report run:

    npx playwright show-report
```

### Frontend Coverage - FAILED

**Failure Output (verbatim):**
```
Terminal: Test: Frontend with Coverage (Charon)
Output:


[... PREVIOUS OUTPUT TRUNCATED ...]

An update to SelectItemText inside a test was not wrapped in act(...).

When testing, code that causes React state updates should be wrapped into act(...):

act(() => {
    /* fire events that update state */
});
/* assert on the output */

This ensures that you're testing the behavior the user would see in the browser. Learn more at https://react.dev/link/w
rap-tests-with-act
An update to SelectItem inside a test was not wrapped in act(...).

When testing, code that causes React state updates should be wrapped into act(...):

act(() => {
    /* fire events that update state */
});
/* assert on the output */

This ensures that you're testing the behavior the user would see in the browser. Learn more at https://react.dev/link/w
rap-tests-with-act

 ✓ src/pages/__tests__/AuditLogs.test.tsx (14 tests) 2443ms
         ✓ toggles filter panel  307ms
         ✓ closes detail modal  346ms

 ❯ src/components/__tests__/ProxyHostForm-dns.test.tsx 9/15
 ❯ src/pages/__tests__/AccessLists.test.tsx 1/5
 ❯ src/pages/__tests__/AuditLogs.test.tsx 14/14

 Test Files 1 failed | 33 passed | 5 skipped (153)
            Tests 1 failed | 862 passed | 84 skipped (957)
     Start at 01:04:39
     Duration 105.97s
```

### Lint: Hadolint Dockerfile - FAILED

**Failure Output (verbatim):**
```
this check
-:335 DL3008 warning: Pin versions in apt get install. Instead of `apt-get install <package>` use `apt-get install <package>=<version>`
-:354 DL4006 warning: Set the SHELL option -o pipefail before RUN with a pipe in it. If you are using /bin/sh in an alpine image or if your shell is symlinked to busybox then consider explicitly setting your SHELL to /bin/ash, or disable this check
-:354 SC2015 info: Note that A && B || C is not if-then-else. C may run when A is true.
 *  The terminal process "/bin/bash '-c', 'docker run --rm -i hadolint/hadolint < Dockerfile'" terminated with exit code: 1.
```

### Security: Docker Image Scan (Local) - FAILED

**Failure Output (verbatim):**
```
[SUCCESS] Vulnerability scan complete
[ANALYSIS] Analyzing vulnerability scan results

[INFO] Vulnerability Summary:
    🔴 Critical: 0
    🟠 High: 8
    🟡 Medium: 20
    🟢 Low: 2
    ⚪ Negligible: 380
    📊 Total: 410

[WARNING] High Severity Vulnerabilities Found:

    - CVE-2025-13151 in libtasn1-6
        Package: libtasn1-6@4.20.0-2
        Fixed: No fix available
        CVSS: 7.5
        Description: Stack-based buffer overflow in libtasn1 version: v4.20.0. The function fails to validate the size of..
.

    - CVE-2025-15281 in libc-bin
        Package: libc-bin@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 7.5
        Description: Calling wordexp with WRDE_REUSE in conjunction with WRDE_APPEND in the GNU C Library version 2.0 to ..
.

    - CVE-2025-15281 in libc6
        Package: libc6@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 7.5
        Description: Calling wordexp with WRDE_REUSE in conjunction with WRDE_APPEND in the GNU C Library version 2.0 to ..
.

    - CVE-2026-0915 in libc-bin
        Package: libc-bin@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 7.5
        Description: Calling getnetbyaddr or getnetbyaddr_r with a configured nsswitch.conf that specifies the library's ..
.

    - CVE-2026-0915 in libc6
        Package: libc6@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 7.5
        Description: Calling getnetbyaddr or getnetbyaddr_r with a configured nsswitch.conf that specifies the library's ..
.

    - CVE-2026-0861 in libc-bin
        Package: libc-bin@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 8.4
        Description: Passing too large an alignment to the memalign suite of functions (memalign, posix_memalign, aligned..
.

    - CVE-2026-0861 in libc6
        Package: libc6@2.41-12+deb13u1
        Fixed: No fix available
        CVSS: 8.4
        Description: Passing too large an alignment to the memalign suite of functions (memalign, posix_memalign, aligned..
.

    - GHSA-69x3-g4r3-p962 in github.com/slackhq/nebula
        Package: github.com/slackhq/nebula@v1.9.7
        Fixed: 1.10.3
        CVSS: 7.6
        Description: Blocklist Bypass possible via ECDSA Signature Malleability...

[ERROR] Found 0 Critical and 8 High severity vulnerabilities
[ERROR] These issues must be resolved before deployment
[ERROR] Review grype-results.json for detailed remediation guidance
[ERROR] Skill execution failed: security-scan-docker-image
```

## 2. Notes

- Some tool outputs were truncated by the runner; the report includes the exact emitted text where available.

---

## 5. Next Actions Required

1. Fix the failing frontend tests and rerun frontend coverage.
2. Resume deferred QA checks once frontend coverage passes.

---

## Accepted Risks

- Security scans skipped for this run per instruction; CVE risk accepted temporarily. Re-run when risk acceptance ends.
