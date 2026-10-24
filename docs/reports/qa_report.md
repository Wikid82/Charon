# QA Report: fix/crowdsec-busy-messages

(Replaces the previous report.) Audited on 2026-10-24 at HEAD `f988aba4`.

**Scope:** 5 commits over `origin/development` (`e53ab062`), section 3g of `docs/plans/crowdsec_hardening_batch5_spec.md` (commit 5 of 3g):
`af0604bb` (busy guard test with a real install job), `06b5aeb5` (busy message for whitelist, console enrolment and hub item restore), `ccbd7387` (shared E2E install-job stubs and busy-message E2E), `fc4eea1c` (gated test release on failure, install E2E tidy), `f988aba4` (busy refusal kept out of the whitelist IP field error). 21 files, +603/-285. Frontend and tests only, plus backend test code (`crowdsec_busy_test.go`, `crowdsec_apply_jobs_handler_test.go`). No production Go change, no model, migration, or API shape change.

**Audited against:** `CLAUDE.md` Definition of Done, `SECURITY.md`, and spec section 3g (gates 1-7). The spec's J4 gate was run in throwaway containers, not on the live `charon`.

**Public changelog:** subjects are plain `fix:`/`test:`/`refactor:` with no `(security)` scope. That is correct: the change is user-facing wording and test coverage, with no vulnerability content.

## Verdict

**PASS with one finding for a decision before merge.** Every code gate passes. The J4 gate is a **FAIL on its "items_installed advances during the install" criterion**. The counter sits flat during a long install and only changes at the end. The root cause is a separate, pre-existing `.yaml`-only hub scan (finding F1, filed as #1712), not this branch. The busy refusals and the post-install recovery both pass.

Per spec 3g.4 step 7, the stop rule (counter never leaves 0) is not met: the counter does move, but only once early and once at the end. The decision is therefore whether to merge with the flat counter or drop the "N items so far" text first. QA recommends merging this PR and tracking F1 in #1712, since F1 is not caused by this branch and the spec says to file rather than widen.

No CRITICAL or HIGH findings in this branch. The `SECURITY.md` Known Vulnerabilities section lists open HIGH items (braces, brace-expansion, dev tooling); none are in this change's files and they are not addressed here.

## Gate results

| # | Gate | Result | Evidence |
|---|------|--------|----------|
| G1 | `go test -race -count=1 ./internal/api/handlers/...`, run 1 | PASS | `ok` in 436.4 s. No failures. |
| G2 | Same command, run 2 (repeat for the unexplained failure seen earlier) | PASS | `ok` in 409.7 s. No failures. Both runs clean. |
| G3 | `scripts/go-test-coverage.sh` (gate 87%) | PASS | Line 91.2%, statements 92.2%. Gate met. |
| G4 | `scripts/frontend-test-coverage.sh` (gate 87%, script gates on lines) | PASS | 310 files, 3867 tests passed, 4 skipped, 2 todo. Lines 91.86%, statements 90.82%, branches 84.59%, functions 88.7%. The script gates on lines only; branches are below 85% but not enforced by the script. Noted, not a blocker. |
| G5 | `bash scripts/local-patch-report.sh` | PASS | Both artifacts present (`test-results/local-patch-report.md`, `.json`). Overall patch 93.3% (14/15), frontend 93.3%, backend 100% (0 changed lines in production Go). One uncovered changed line: `frontend/src/pages/CrowdSecConfig.tsx:867`, the start-failed busy branch (see F2). |
| G6 | `npm run type-check` | PASS | `tsc --noEmit` clean. |
| G7 | `npm run build` (frontend) and `go build ./...` (backend) | PASS | Both exit 0. |
| G8 | `golangci-lint run --config .golangci.yml ./internal/api/handlers/...` | PASS | 0 issues. |
| G9 | `eslint` on the changed frontend files | PASS | 0 errors, 29 warnings. The warnings are the existing `jsx-a11y`, `react-hooks/set-state-in-effect` and `testing-library/prefer-find-by` patterns in `CrowdSecConfig.tsx` and its tests. None are new to this branch's logic. |
| G10 | `lefthook run pre-commit` | NOT MEANINGFUL | Nothing is staged on a committed branch. Substituted `golangci-lint` and `eslint` above. |
| G11 | Targeted Playwright, `--project=security-tests` (Firefox-only project), all 9 specs that touch or import `tests/utils/crowdsec-stubs.ts` plus the hub-items spec | PASS | 214 passed, 0 failed, 9.9 min. Specs: `crowdsec-background-install`, `crowdsec-config`, `crowdsec-console-enrollment`, `crowdsec-dashboard`, `crowdsec-decisions`, `crowdsec-diagnostics`, `crowdsec-hub-preset-apply`, `crowdsec-ui-states`, `crowdsec-hub-items`. Ran against a freshly rebuilt `charon-e2e` (`docker-rebuild-e2e`, image built from this branch). Full suite and cross-browser runs deferred to CI. |
| G12 | Changelog artifact restored after Playwright | PASS | `git checkout -- backend/internal/changelog/data/changelog.json`; `git status` clean. |
| G13 | `govulncheck ./...` | PASS | 0 vulnerabilities in reachable code. One module-level advisory in a required module that the code does not call (informational, not introduced here). |
| G14 | GORM security scan | NOT TRIGGERED | No `backend/internal/models/**`, GORM query or migration changes. |
| G15 | Gotify token review | PASS | No Gotify references in the diff, test fixtures, or new logs. The only token-like string is a test fixture `token-value` in a console-enrolment UI test. |
| G16 | Secrets, debug output and skipped tests in the diff | PASS | No `console.log`, `fmt.Print*`, `.only`, `.skip` or `test.fixme` added. |
| G17 | CodeQL and Trivy | DEFERRED TO CI | `fix:`/`test:`-only, no new feature surface, per CLAUDE.md. |
| G18 | Translation parity (5 locales) | PASS | `i18n.test.ts` passes. The three new `crowdsecConfig.whitelist.*` keys are present in en, de, es, fr and zh. |

## J4 gate: real long install (throwaway containers, not the live `charon`)

Setup: two throwaway containers from `charon:local` (built from this branch), each with its own named volume, bound to `127.0.0.1` on ports 18095 and 18096. Both were removed afterwards with their volumes. Preset: `sigmahq/windows_proc_creation` (collections, cold cache), the preset the earlier QA used. Hub reachable from the sandbox.

| Check | Run 1 (434 s install) | Run 2 (22 s install) | Result |
|---|---|---|---|
| Job reaches `succeeded` | Yes, after 434 s | Yes, after 22 s | PASS |
| `items_installed` **advances during** the install | Reads 0 at queue, then 1 at the first tick (15 s). **Stays at 1 for about 7 minutes**, then jumps to 3 at success. | 0 → 1 → 1 → 3 over 22 s. Advances once at the end. | **FAIL** (F1) |
| Not stuck at 0 until the end | Met (1 from the first tick) | Met | PASS (minimal) |
| Busy refusals while running: whitelist add | `POST /admin/crowdsec/whitelist` → 409 `crowdsec_busy` with `job_id` (run 1 probe used a wrong field name and still got the busy refusal first; run 2 used the correct `ip_or_cidr`) | 409 `crowdsec_busy`, same `job_id` | PASS |
| Busy refusal: console enrol | `POST /admin/crowdsec/console/enroll` → 409 `crowdsec_busy` with `job_id` | 409 `crowdsec_busy` with `job_id` | PASS |
| Busy refusal: hub-item restore | `POST /admin/crowdsec/hub-items/restore` → 409 `crowdsec_busy` with `job_id` | 409 `crowdsec_busy` with `job_id` | PASS |
| After finishing: whitelist add works | `201`, entry created | not re-run | PASS |
| After finishing: console enrol not refused as busy | Response is 502 "Local API is not running" (LAPI not started in this container). Not a 409 busy. | not re-run | PASS (the busy guard lifted; enrolment itself needs a running LAPI) |
| After finishing: hub-item restore works | `200`, item installed | not re-run | PASS |
| Live `charon` untouched | Verified: `charon` (up 5 h, healthy) and `charon-e2e` (healthy) unchanged; throwaway volumes only | Same | PASS |

**The J4 "advances" failure is a real, user-visible defect, not a branch regression.** The mechanism (finding F1):

- The progress counter (`CountDownloadedHubFiles`, `backend/internal/crowdsec/hubitems.go:154`) and the installed-item scan (`hubItemFromTarget`, `backend/internal/crowdsec/hubitems.go:133`) only accept `.yaml`.
- The sigmahq scenarios are shipped as `.yml`: 520 of the 573 scenario links in run 1 point at `…/*.yml`. Those are skipped.
- In run 1, about 560 `.yml` files were written steadily from 04:37 to 04:44, but the counter stayed at 1. Its final value of 3 is far below the roughly 590 items actually linked.

Filed as **GitHub issue #1712** (non-security, public OK).

## Findings (ranked)

| ID | Severity | Finding | Location | Action |
|----|----------|---------|----------|--------|
| F1 | **Medium** (user-visible, misleading; not security) | Install progress does not advance for `.yml` hub items, and `items_installed` undercounts (3 of ~590 in run 1). Other callers of the installed-item scan (restore, import "already installed") may also miss `.yml` items; not verified. | `backend/internal/crowdsec/hubitems.go:133`, `:154`; `apply_jobs.go:398/440/445` | **Filed as #1712.** Not fixed here: it is a pre-existing backend defect outside this PR's scope (the spec forbids widening). Decide before merge whether to drop the "N items so far" text in this PR instead. |
| F2 | Low (test gap) | Start-failed busy branch is not covered by a test: `CrowdSecConfig.tsx:867` (`crowdsecErrorMessage(err, t, t('crowdsecConfig.startFailed'))`). It is the only uncovered changed line. | `frontend/src/pages/CrowdSecConfig.tsx:867` | Add a unit test with a 409 `crowdsec_busy` from the start call. Small, can be folded into this PR as a `test:` commit before merge. |
| F3 | Low (UX, pre-existing) | Whitelist remove and add toasts show Axios text such as "Request failed with status code 409" when the error is not a busy refusal, because the `err instanceof Error ? err.message` fallback is unchanged. | `frontend/src/hooks/useCrowdSecWhitelist.ts` (remove path) | Not in scope. Consider a follow-up to use the server `error` text, as the add path already does. |
| F4 | Info | Busy check runs before request-body validation on the whitelist endpoint: a wrong-field body gets 409 `crowdsec_busy` rather than 400 while an install runs. Consistent with the console route's guard ordering, and the busy answer is the correct first refusal. | `backend/internal/api/handlers/crowdsec_handler.go` (whitelist path) | No change needed. Recorded for the record. |
| F5 | Info | Frontend branch coverage 84.59% (lines 91.86%). The gate script enforces lines only. | `scripts/frontend-test-coverage.sh` | No action for this PR. |

## Code review notes (no defects found)

- `crowdsecErrorMessage` (`frontend/src/api/errorBody.ts`) is pure, typed with `TFunction`, and unit-tested for the busy and fallback paths.
- The console-enrol catch keeps `sanitizeErrorMessage` for every non-busy path, so no secret text is newly shown.
- Restore panel checks the busy code before the 409 switch, so a running install no longer shows "a restore is already running". A 409 without the busy code still shows `alreadyRunning`.
- The new backend tests use a `sync.Once` release that is also a `t.Cleanup`, so a failing assertion does not leak the blocked job goroutine. The J3 real-job test asserts no enrolment row was recorded while busy and that enrolment is allowed again after the job ends.
- The shared E2E stub module is the only place the install-job stubs live now; the two spec-local copies are removed.

## Follow-ups

- **#1712** (F1): hub `.yml` items not counted by the progress counter or the installed-item scan. Medium. Public issue (not security).
- **F2**: add the `CrowdSecConfig.tsx:867` busy test before merge, or accept the 1-line patch gap. Low.
- **F3**: whitelist remove error text. Low, follow-up.
- Full E2E suite and cross-browser runs remain for CI on the PR.
