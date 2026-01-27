# E2E Shard Failures – Run 21377510901 (PR 550)

**Issue**: CI shards are failing/flaking against Docker environment (localhost:8080) while local runs pass. Need root-cause plan without re-enabling Vite/coverage.
**Status**: 🔴 ACTIVE – Planning
**Priority**: 🔴 CRITICAL – CI blocked
**Created**: 2026-01-27

---

## 🔍 CI vs Local Findings

- **Shard 1** (passed but flaky): `tests/core/access-lists-crud.spec.ts` intermittently misses toast / ACL visibility assertion.
- **Shard 2** (hard fail): `emergency-server/*.spec.ts` and `tier2-validation.spec.ts` hit `ECONNREFUSED ::1:2019/2020`; access list creation returns "Blocked by access control list".
- **Shard 3** (fail): `tests/core/account-settings.spec.ts` certificate email validation – error message not visible after retries.
- **Shard 4** (fail):
  - `tests/core/system-settings.spec.ts` success toast not observed.
  - `tests/core/user-management.spec.ts` invite/resend flows fail with strict mode locator collisions (multiple matching buttons).
- **Container logs (shard 2 artifact)**: `Emergency server disabled (CHARON_EMERGENCY_SERVER_ENABLED=false)` and emergency bypass called. Tier-2 server (port 2020) never starts → explains connection refusals. Security ACL reported as disabled post emergency reset but initial access-list calls still 401/blocked until login.
- **Environment parity**: Local likely starts emergency server (or uses 127.0.0.1), CI disables it via env; CI uses IPv6 loopback (::1) causing refusals when service is off.
- **Architecture**: Vite/coverage already removed; tests target Docker app only.

---

## 🧭 Hypotheses

1) **Emergency server/tier2 disabled in CI** → all shard-2 tests fail; local enables by default. Root cause: env var CHARON_EMERGENCY_SERVER_ENABLED is false in e2e compose or workflow.
2) **ACL bypass timing** → initial emergency reset happens, but ACL state may still block access-list creation; needs deterministic disable hook.
3) **UI assertion drift** → account-settings/system-settings/user-management expectations mismatch current UI text/roles; strict-mode locator ambiguity for invite buttons.
4) **Toast race / network latency** → success toasts not awaited with retryable locator; CI slower than local.

---

## 🎯 Action Plan (phased)

### Phase 1 – Environment parity (CI vs local)
- Enable emergency server in CI Docker stack: set `CHARON_EMERGENCY_SERVER_ENABLED=true`, expose admin port 2019 and tier-2 port 2020, and ensure services bind for both IPv4/IPv6 (CI uses ::1).
- Explicitly set emergency token for tier-2 if required; document its source (redacted) in test env.
- Add startup assertion in global-setup to poll `http://localhost:2019/config/` and `http://localhost:2020/health` (skip if disabled) with short timeout to fail fast.
- Capture env snapshot in CI logs for emergency-related vars (redact secrets) and note resolved base URL (IPv4 vs IPv6).

### Phase 2 – Deterministic security disable
- After login/setup, call emergency reset and then verify ACL/rate-limit flags via `/api/v1/security/config` before continuing tests; make this idempotent and fail fast before any data creation.
- If ACL still blocks create, call `/api/v1/access-lists/templates` to assert 200; otherwise retry emergency reset once and fail with clear error.
- Add small utility in TestDataManager to assert ACL is disabled before creating ACL-dependent resources; short-circuit with actionable error.

### Phase 3 – Shard-specific fixes
- **Shard 2**: Once emergency server enabled, rerun to confirm. Add health check for tier-2 server; fail early if down.
- **Shard 1**: Wrap ACL toast assertions with `expect.poll`/`toHaveText` on role-based toast locator; ensure list refresh after create. Add a shared toast helper (role-based with short retries) to reuse across specs.
- **Shard 3**: Update certificate email validation assertion to target the visible validation message role/text; avoid brittle `getByText` timeouts.
- **Shard 4**:
  - System settings toast: use role-based toast locator with retry; ensure the form submit awaits network idle before assert.
  - User management invite/resend: replace ambiguous button locators with role+name scoped to each row (e.g., row locator then `getByRole('button', { name: /resend invite/i })`); add a row-scoped locator helper to avoid strict-mode collisions.

### Phase 4 – Observability and flake defense
- Add Playwright trace/video for shard 1–4 in CI (already default? confirm); keep artifacts for failing shards only to save time.
- Log emergency server state (enabled/disabled), ACL status, and resolved base URL (IPv4 vs IPv6) at start of each project.
- Add short retries (max 2) for toast assertions using auto-retrying expect.

### Phase 5 – Validation loop
- Rerun shards 1–4 in CI after env toggle; compare to local.
- If shard 2 passes but others fail, prioritize locator/UX updates in phases 3–4.
- Keep Vite/coverage off until all shards green; plan separate coverage job later.

---

## 📄 Files/Areas to touch
- Workflow/compose env: ensure `CHARON_EMERGENCY_SERVER_ENABLED=true`; expose tier-2 port 2020; confirm emergency token variable passed.
- `tests/core/*`: adjust locators and toast assertions per shard notes.
- `tests/utils/TestDataManager.ts`: add ACL-disabled check before ACL creation.
- `global-setup.ts` (if needed): add emergency server health probe and state logging.

---

## ✅ Completion checklist
- [ ] CI env starts emergency server (port 2020) and admin API (2019); health probes added.
- [ ] Security disable verified before data setup; ACL create no longer blocked.
- [ ] Shard 1 toast flake mitigated with resilient locator/wait.
- [ ] Shard 2 emergency/tier2 tests pass in CI.
- [ ] Shard 3 account-settings validation assertion updated and passing.
- [ ] Shard 4 system-settings toast and user-management locators stabilized.
- [ ] Vite/coverage remain off during fixes; add a guard/checklist item in workflow to ensure coverage flags stay disabled during triage; plan coverage follow-up separately.

---

## 📎 Artifacts reviewed
- GH Actions log: `.agent_work/run-21377510901.log`
- Docker logs (shard 2): `.agent_work/run-21377510901-artifacts/docker-logs-shard-2.txt` (shows emergency server disabled, ACL reset attempts)
