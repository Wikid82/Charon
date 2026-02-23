## QA Report — PR-2 Security Patch Posture Audit

- Date: 2026-02-23
- Scope: PR-2 only (security patch posture, admin API hardening, rollback viability)
- Verdict: **READY (PASS)**

## Gate Summary

| Gate | Status | Evidence |
| --- | --- | --- |
| Targeted E2E for PR-2 | PASS | Security settings test for Caddy Admin API URL passed (2/2). |
| Local patch preflight artifacts | PASS | `test-results/local-patch-report.md` and `.json` regenerated. |
| Coverage and type-check | PASS | Backend coverage 87.7% line / 87.4% statement; frontend type-check passed; frontend coverage preflight input passed (88.99% lines). |
| Pre-commit gate | PASS | `pre-commit run --all-files` passed after resolving version and type-check hook issues. |
| Security scans | PASS | CodeQL Go/JS CI-aligned scans passed; findings gate passed with no HIGH/CRITICAL; Trivy passed at configured severities. |
| Runtime posture + rollback | PASS | Default scenario shifted `A -> B` for PR-2 posture; rollback remains explicit via `CADDY_PATCH_SCENARIO=A`; admin API URL now validated and normalized at config load. |

## Resolved Items

1. `check-version-match` mismatch fixed by syncing `.version` to `v0.19.1`.
2. `frontend-type-check` hook stabilized to `npx tsc --noEmit` for deterministic pre-commit behavior.

## PR-2 Closure Statement

All PR-2 QA/security gates required for merge are passing. No PR-3 scope is included in this report.

---

## QA Report — PR-3 Keepalive Controls Closure

- Date: 2026-02-23
- Scope: PR-3 only (keepalive controls, safe fallback/default behavior, non-exposure constraints)
- Verdict: **READY (PASS)**

## Reviewer Gate Summary (PR-3)

| Gate | Status | Reviewer evidence |
| --- | --- | --- |
| Targeted E2E rerun | PASS | Security settings targeted rerun completed: **30 passed, 0 failed**. |
| Local patch preflight | PASS | `frontend/coverage/lcov.info` present; `scripts/local-patch-report.sh` artifacts regenerated with `pass` status. |
| Coverage + type-check | PASS | Frontend coverage gate passed (89% lines vs 85% minimum); type-check passed. |
| Pre-commit + security scans | PASS | `pre-commit --all-files`, CodeQL Go/JS CI-aligned scans, findings gate, and Trivy checks passed (no HIGH/CRITICAL blockers). |
| Final readiness | PASS | All PR-3 closure gates are green. |

## Scope Guardrails Verified (PR-3)

- Keepalive controls are limited to approved PR-3 scope.
- Safe fallback behavior remains intact when keepalive values are missing or invalid.
- Non-exposure constraints remain intact (`trusted_proxies_unix` and certificate lifecycle internals are not exposed).

## Manual Verification Reference

- PR-3 manual test tracking plan: `docs/issues/manual_test_pr3_keepalive_controls_closure.md`

## PR-3 Closure Statement

PR-3 is **ready to merge** with no open QA blockers.
