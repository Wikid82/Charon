# Re-enable Security Playwright Tests and Run Full E2E (feature/beta-release)

**Goal**: Turn security Playwright tests back on, run the full E2E suite (including security flows) on Docker base URL, and prepare triage steps for any failures.
**Status**: 🔴 ACTIVE – Planning
**Priority**: 🔴 CRITICAL – CI/CD gating
**Created**: 2026-01-27

---

## 🎯 Scope and Constraints
- Target branch: `feature/beta-release`.
- Base URL: Docker stack (`http://localhost:8080`) unless security tests require override.
- Keep management-mode rule: no code reading here; instructions only for execution subagents.
- Coverage: run E2E coverage only if already supported via Vite flow; otherwise note as optional follow-up.

---

## 🗂️ Files to Change (for execution agents)
- [playwright.config.js](playwright.config.js): re-enable security project/shard config, ensure `testDir` includes security specs, and restore any `grep`/`grepInvert` filters previously disabling them.
- Tests security fixtures/utilities: [tests/security/**](tests/security/), [tests/fixtures/security/**](tests/fixtures/security/), and any shared helpers under [tests/utils](tests/utils) that were toggled off (e.g., skip blocks, `test.skip`, env flags).
- Workflows/toggles: [ .github/workflows/*e2e*.yml](.github/workflows) and Docker compose overrides (e.g., [.docker/compose/docker-compose.e2e.yml](.docker/compose/docker-compose.e2e.yml)) to re-enable env vars/secrets for security tests (ACL/emergency/rate-limit toggles, tokens, base URLs).
- Global setup/teardown: [tests/global-setup.ts](tests/global-setup.ts) and related teardown to ensure security setup hooks are active (if previously short-circuited).
- Playwright reports/ignore lists: verify any `.gitignore` or report pruning that might suppress security artifacts.

---

## 🛠️ Implementation Steps
0) **Prepare environment and secrets**
  - Ensure required secrets/vars are present (redact in logs): `CHARON_EMERGENCY_TOKEN`, `CHARON_ADMIN_USERNAME`/`CHARON_ADMIN_PASSWORD`, `PLAYWRIGHT_BASE_URL` (`http://localhost:8080` for Docker runs), feature toggles for security/ACL/rate-limit (e.g., `CHARON_SECURITY_TESTS_ENABLED`).
  - Source from GitHub Actions secrets for CI; `.env`/`.env.local` for local. Do not hardcode; validate presence before run. Redact values in logs (print presence only).

1) **Restore security test inclusion**
  - Revert skips/filters: remove `test.skip`, `test.describe.skip`, or project-level `grepInvert` that excluded security specs.
  - Ensure `projects` in `playwright.config.js` include security shard (or merge back into main matrix) with correct `testDir`/`testMatch`.
  - Re-enable security fixture initialization in `global-setup.ts` (e.g., emergency server bootstrap, token wiring) if it was bypassed.

2) **Re-enable env toggles and secrets**
  - In E2E workflow and Docker compose for tests, set required env vars (examples: `CHARON_EMERGENCY_SERVER_ENABLED=true`, `CHARON_SECURITY_TESTS_ENABLED=true`, tokens/ports 2019/2020) and confirm mounted secrets for security endpoints.
  - Verify base URL resolution matches Docker (avoid Vite unless running coverage skill).

3) **Bring up/refresh test stack**
  - Start or rebuild test stack before running Playwright: use task `Docker: Start Local Environment` (or `Docker: Rebuild E2E Environment` if needed).
  - Health check: verify ports 8080/2019/2020 respond (`curl http://localhost:8080`, `http://localhost:2019/config`, `http://localhost:2020/health`).

4) **Run full E2E suite (all browsers + security)**
  - Preferred tasks (from workspace tasks):
    - `Test: E2E Playwright (All Browsers)` for breadth.
    - `Test: E2E Playwright (Chromium)` for faster iteration.
    - `Test: E2E Playwright (Skill)` if automation wrapper required.
  - If security suite has its own task (e.g., `Test: E2E Playwright (Chromium) - Cerberus: Security Dashboard/Rate Limiting`), run those explicitly after re-enable.

5) **Optional coverage pass (only if Vite path)**
  - Coverage only meaningful via Vite coverage skill (port 5173). Docker/8080 runs will show 0% coverage—do not treat as failure.
  - If required: run `.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage`; target non-zero coverage and patch coverage on changed lines.

6) **Report collection and review**
  - Generate and open report: `npx playwright show-report` (or task `Test: E2E Playwright - View Report`).
  - For failures, gather traces/videos from `playwright-report/` and `test-results/`.

7) **Targeted rerun loop for failures**
  - For each failing spec: rerun with `npx playwright test --project=chromium --grep "<failing name>"` (and the corresponding security project if separate).
  - After fixes, rerun full Chromium suite; then run all-browsers suite.

6) **Triage loop**
  - Classify failures: environment/setup vs. locator/data vs. backend errors.
  - Log failing specs, error messages, and env snapshot (base URL, env flags) into triage doc or ticket.

---

## ✅ Validation Checklist (execution order)
- [ ] Lint/typecheck: run `Lint: Frontend`, `Lint: TypeScript Check`, `Lint: Frontend (Fix)` if needed.
- [ ] E2E full suite with security (Chromium): task `Test: E2E Playwright (Chromium)` plus security-specific tasks (Rate Limiting/Security Dashboard) once re-enabled.
- [ ] E2E all browsers: `Test: E2E Playwright (All Browsers)`.
- [ ] Coverage (if applicable): run coverage skill; verify non-zero coverage in `coverage/e2e/`.
- [ ] Security scans: `Security: Trivy Scan` and `Security: Go Vulnerability Check` (or CodeQL tasks if required).
- [ ] Reports reviewed: open Playwright HTML report, inspect traces/videos for any failing specs.
 - [ ] Triage log captured: record failing spec IDs, errors, env snapshot (base URL, env flags) and artifact links in shared location (e.g., `test-results/triage.md` or ticket).

---

## 🧪 Triage Strategy for Expected Failures
- **Auth/boot failures**: Check `global-setup` logs, ensure emergency/ACL toggles and tokens present. Validate endpoints 2019/2020 reachable in Docker logs.
- **Locator/strict mode issues**: Use role-based locators and scope to rows/sections; prefer `getByRole` with accessible names. Add short `expect` retries over manual waits.
- **Timing/toast flakiness**: Switch to `await expect(locator).toHaveText(...)` with retries; avoid `waitForTimeout`. Ensure network idle or response awaited on submit.
- **Backend 4xx/5xx**: Capture response bodies via `page.waitForResponse` or Playwright traces; verify env flags not disabling required features.
- **Security endpoint mismatches**: Validate test data/fixtures match current API contract; update fixtures before rerunning.
- **Next steps after failures**: Document failing spec paths, error messages, and suspected root cause; rerun focused spec with `--project` and `--grep` once fixes applied.

---

## 📌 Commands for Executors
- Re-enable/verify config: `node -e "console.log(require('./playwright.config'))"` (sanity on projects).
- Run Chromium suite: task `Test: E2E Playwright (Chromium)`.
- Run all browsers: task `Test: E2E Playwright (All Browsers)`.
- Run security-focused tasks: `Test: E2E Playwright (Chromium) - Cerberus: Security Dashboard`, `... - Cerberus: Rate Limiting`.
- Show report: `npx playwright show-report` or task `Test: E2E Playwright - View Report`.
- Coverage (optional): `.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage`.

---

## 📎 Notes
- Keep documentation of any env/secret re-introduction minimal and redacted; avoid hardcoding secrets.
- If security tests require data resets, ensure teardown does not affect subsequent suites.
