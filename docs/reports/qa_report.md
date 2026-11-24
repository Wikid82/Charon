# QA Report: Single-Use Invite Links

- Branch: `fix/invite-acceptance` (9 commits on `origin/development`, head `589c17aea`)
- Plan: `docs/plans/pr_invite_acceptance_plan.md` (rev 2), section 7 (Definition of Done and container check)
- Scope: an invite link is checked and used in one write-locked step, so it works once, including when several
  requests use it at the same time; an invite whose expiry is unset counts as expired; an account that already has a
  password cannot be claimed through an invite; any password change (administrator reset, self-service change,
  recovery command) closes a pending invite and leaves the account's enabled state alone; resending replaces the
  link in the same guarded way and is recorded in the audit log; invite acceptance draws on the sign-in allowance per
  client address and has a 64 KiB request size cap; a duplicate invite or a duplicate directly created user answers
  409 without storage text.
- Image under test: a throwaway image built from the working tree at the head above (`invqa:pr`). All containers
  were throwaway (`invqa-*`, own ports, `tmpfs` data) and were removed afterwards. The live and shared containers
  were not touched.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.5, lefthook 2.2.2, Playwright 1.64.1.
- Date: 2026-11-24.

## Verdict

**Approved.** Every Definition of Done gate passes and every container gate matches the plan. Patch coverage is
90.5% (target 87%), so QA added no tests. No application code, test or configuration was changed by QA; the only
file changed is this report. One low, out-of-scope finding (F1) is listed for the orchestrating session to file or
fold in.

Note on the head: the branch was rebased onto `origin/development` during this run (that brought in only a
dependency and container toolchain refresh, no invite code). The image was rebuilt from the rebased head, and the
container gates and targeted E2E specs were run again on it; the results below are from the rebased head.

## Findings

| Id | Severity | Finding | Remediation |
| --- | --- | --- | --- |
| F1 | Low (UX and data tidiness, out of scope) | A password change closes an invite only while its status is `pending` (`backend/internal/services/auth_service.go:220-228`, `Where("id = ? AND invite_status = ?", ..., pending)`). An invite already marked `expired` keeps its status and stored token after an administrator sets a password. Verified in a throwaway container: invite, expiry moved into the past, accept answers 410 (status becomes `expired`), admin `PUT /users/:id` with a password answers 200, the row then reads `invite_status = expired`, token still stored, password set; `GET /users/:id` reports `invite_status: expired`, so the Users page shows the "Expired" badge for an account that has a password. The old link still cannot be used (410 again; the password condition keeps it closed) | Close any invite that is not `accepted` on a password change (`invite_status IN ('pending', 'expired')`, clear token and expiry), with a unit test next to the existing pending case. Either fold into this PR as a small `fix:` commit or file a public issue ("Users page shows Expired for an account whose password was set by an administrator") |

No CRITICAL, HIGH or MEDIUM findings.

## Gate results

| # | Gate | Command | Result |
| --- | --- | --- | --- |
| 1 | Targeted E2E (firefox) | `npx playwright test tests/settings/user-management.spec.ts tests/settings/user-lifecycle.spec.ts tests/core/data-consistency.spec.ts --project=firefox` | **Pass**: 52 passed, 1 skipped (`user-management.spec.ts:1094`, an existing `test.skip`), 2.4 min |
| 1b | Targeted E2E (security project, firefox engine locally) | `npx playwright test tests/security-enforcement/public-registration-removed.spec.ts --project=security-tests` | **Pass**: 7 passed (setup, shard setup, 5 spec cases) |
| 2 | GORM security scan | `./scripts/scan-gorm-security.sh --check` | **Pass**: 0 critical, 0 high, 0 medium, 2 informational index suggestions (existing) |
| 3 | Local patch coverage | `bash scripts/local-patch-report.sh` | **Pass**: overall 90.5% (191/211 changed lines), backend 90.5%, frontend and agent no changed lines; both artifacts written |
| 4 | Backend coverage | `scripts/go-test-coverage.sh` | **Pass**: line 92.9%, statement 94.0% (gate 87%); handlers 93.3%, services 92.1%, models 97.6%, routes 97.1% |
| 4b | Frontend coverage (input for gate 3) | `scripts/frontend-test-coverage.sh` | **Pass**: lines 92.87% (gate 87%) |
| 4c | Race detector on the invite tests | `go test ./internal/services/ ./internal/models/ -run 'Invite\|HasPending\|InviteState\|ConsumeInvite' -race` and `go test ./internal/api/handlers/ ./internal/api/routes/ -run 'Invite\|Resend\|PublicRegistration\|UniqueConstraint\|CreateUser' -race` | **Pass** |
| 5 | CodeQL Go and JS | `lefthook run codeql` | **Pass**: Go 1 result, suppressed by an existing entry in `codeql-suppressions.yml` (session cookie on local plain-HTTP origins, unrelated); JS 0 results; parity check passed |
| 6 | Pre-commit, all files | `lefthook run pre-commit --all-files` | **Pass**: all 20 hooks (golangci-lint fast, go vet, frontend lint and type-check, semgrep 0 findings, markdownlint, shellcheck, actionlint, and the rest) |
| 7 | Backend build | `cd backend && go build ./...` | **Pass** |
| 8 | Frontend type-check | `cd frontend && npm run type-check` | **Pass** |
| 9 | Frontend build | `cd frontend && npm run build` | **Pass** |
| 10 | Trivy | | Deferred to CI (no new feature surface; CI runs it on every PR) |
| 11 | Full lint, run last | `make lint-backend` | **Pass**: 0 issues (agent unchanged, so `make lint-agent` not required) |

Uncovered changed lines (all defensive database error branches; no behaviour path is uncovered):
`backend/internal/services/invite_service.go` 51-52, 105, 114-115, 132-133, 158-159, 161-162, 219;
`backend/internal/services/auth_service.go` 227-228; `backend/internal/api/handlers/user_handler.go` 674-676,
996-997, 1117.

## Container gates

Run with a scripted sequence against `invqa-gate` (`CHARON_AUTH_RATELIMIT_LOGIN_REQUESTS=100`) and `invqa-rl`
(default limits). Invite tokens were read with read-only `sqlite3` inside the throwaway container. 42 of 42
checks passed on the rebased head (the same sequence also passed on the pre-rebase image).

| Gate | Expected | Observed | Result |
| --- | --- | --- | --- |
| 20 parallel accepts of one token, distinct passwords | exactly one 200, the rest 404/409/410 | 1 x 200, 19 x 404 | Pass |
| Sign in with each of the 20 passwords | only the winner's password works | exactly one sign-in succeeded, with the password of the request that got 200 | Pass |
| Stored row after the race | accepted, token cleared, enabled, winner's name | `accepted`, empty token, `enabled = 1`, winner's name | Pass |
| Audit | one acceptance entry for the raced invite | 1 `user_invite_accept` row for that address | Pass |
| Control: single accept then sign in | 200, then 200 | 200, 200 | Pass |
| Replay after accept | 404 | accept 404, validate 404 | Pass |
| Admin password reset on a pending invitee | 200; invite closed; enabled unchanged | 200; status, token and expiry cleared; `enabled = 0` before and after | Pass |
| Old link after that reset | refused | validate 404, accept 404 | Pass |
| Resend on a pending invite | 200, new token and later expiry, audited | 200, token replaced, expiry moved forward, 1 `user_invite_resend` row | Pass |
| Old link after resend | refused | validate 404, accept 404 | Pass |
| New link after resend | works | validate 200, accept 200 | Pass |
| Resend after the invite is accepted | refused | 400 `User does not have a pending invite` | Pass |
| 65 KiB accept body | 413 | 413 `{"error":"the request body is too large","error_code":"request_too_large"}` | Pass |
| Rapid accepts with the default limit | reach 429 | attempts 1-10 answered 404, attempts 11-15 answered 429 with `Retry-After: 60` and the generic "Too many requests" text | Pass |
| 8 parallel invites for one e-mail | one 201, the rest 409, no storage text | 1 x 201, 7 x 409 `{"error":"Email already in use"}`; one row for that address; no storage text in any body | Pass |
| Create user with an address already in use | 409, no storage text | 409 `{"error":"Email already in use"}` (both for a created and for an invited-then-accepted address) | Pass |
| Raw token exposure | no raw token in any response; invite answers carry only the masked token and the redacted URL | no raw token in any saved response body (setup, sign-in, invite, resend, accept, validate, create, update) or in the container log; invite answers carry `invite_token_masked: "********"` and `invite_url` empty without a public URL (redacted otherwise) | Pass |

## Security review notes

- Token handling: tokens are 32 random bytes from `crypto/rand`, hex encoded; lookups refuse an empty token; the
  request logger records paths only; invite answers mask the token and redact the URL.
- Lock discipline: password hashing, token generation, audit writes and invite e-mail all run outside the
  write-locked callbacks; the callbacks only read and write through the transaction.
- The invite e-mail goroutine takes its logger before the handler returns.
- Error answers: unexpected storage errors answer fixed texts (`Failed to accept invite`, `Failed to create user`,
  `Failed to update invite token`) with detail in the server log only.
- No notification tokens appear in logs, artifacts or examples touched by this PR.

## Files changed by QA

- `docs/reports/qa_report.md` (this report, uncommitted)

No application code, tests or configuration were changed.
