# QA Report: System Settings & Security (Feature Flags OFF)

**Date:** December 8, 2025
**QA Agent:** QA_Security
**Scope:** System Settings features card, Security page (no Cerberus master toggle), and backend/UX behavior when `feature.cerberus.enabled` and `feature.uptime.enabled` are `false`.
**Specification:** Feature flag controls and UI expectations per product notes

## Executive Summary

**Final Verdict:** ✅ PASS WITH WARNINGS

- Frontend checks: `npm run type-check` and `npm run test:ci` pass; vitest emitted non-blocking act()/query warnings in security suites.
- Feature flags verified in DB as `false` for Cerberus and Uptime; backend logs show only proxy/NZBget traffic, no uptime or cerberus activity.
- Security page presents per-service toggles only (no global Cerberus switch) and respects disabled state in tests; System Settings Features card remains at top with two-column layout and tooltip text.

## Test Results

| Area | Status | Notes |
| --- | --- | --- |
| Frontend TypeScript | ✅ PASS | `npm run type-check` via task (Dockerized run) |
| Frontend Unit Tests | ✅ PASS* | `npm run test:ci` (584 tests). Warnings: act() needed in Security audit tests; several React Query data undefined warnings in security/crowdsec specs; jsdom navigation not implemented (expected). |
| Backend Tests | ⏭️ Not Run | Not requested for this cycle. |
| UI Sanity (headless) | ✅ PASS | Layout verified via tests/code: Features card top, 2-col grid, tooltips via `title`, overlay during mutations; Security page shows per-service toggles, banner when Cerberus disabled. |
| Backend Sanity | ✅ PASS | DB flags false; no uptime/cerberus activity visible in recent logs; services remain disabled. |

## Validation Details

- Feature flags state: `feature.cerberus.enabled=false`, `feature.uptime.enabled=false` (queried `/app/data/charon.db` inside `charon-debug`).
- System Settings Features card: two switches (Cerberus, Uptime), `title` tooltips present, `ConfigReloadOverlay` shows during pending mutations; card positioned at top of page grid.
- Security page: no global Cerberus toggle; per-service toggles disabled when Cerberus flag is false; disabled banner shown; overlay used during config mutations.
- Backend behavior: recent `charon-debug` logs contain only NZBget/Caddy access entries; no uptime monitor or cerberus job traces while flags are off.

## Issues / Warnings

- Vitest warnings (non-failing):
  - act() wrapping needed in Security audit tests (double-click prevention) and Security loading test.
  - React Query “query data cannot be undefined” warnings in security and crowdsec specs; jsdom “navigation to another Document” warnings in security/crowdsec specs.
  - These did not fail the suite but add noise; consider tightening test setup/mocks.

## Follow-ups / Recommendations

1. Clean up Security/CrowdSec tests to wrap pending state updates in `act()` and ensure query mocks return non-undefined defaults to silence warnings.
2. If deeper backend verification is needed, run `Go: Test Backend` or integration suite; not run in this cycle.

## Evidence

- Frontend tests: `npm run test:ci` (all green, warnings noted above).
- Feature flags: queried SQLite in `charon-debug` container showing both flags `false`.
- Logs: `docker logs charon-debug --tail` showed only NZBget access traffic, no uptime/cerberus actions.

---

**Status:** ✅ Approved with warnings logged above.
