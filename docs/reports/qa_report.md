# QA Report: First-Run Setup Code

- Branch: `fix/first-run-setup` (7 commits on `origin/development`, up to `d64af2420`)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 5
- Scope: backend (setup handler, setup code service, route wiring, config, password reset command), frontend
  (first-run page, error mapping, locales), CI workflows, integration and smoke scripts, Playwright fixtures, docs.
- Image under test: `charon:local` from `docker-rebuild-e2e`, rebuilt after the QA changes below.
- Date: 2026-11-13

## Verdict

**Approved after QA fixes.** QA found one CI-blocking defect (H1) and fixed it in the working tree, along with
the patch coverage gaps and two doc inaccuracies. Nothing blocking remains. The full E2E and integration matrix
runs only in the PR's CI.

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `docker-rebuild-e2e` (twice: before and after QA changes) | Pass, container healthy |
| Playwright `public-registration-removed.spec.ts --project=security-tests` | Pass, 7/7 (both images) |
| Playwright `tests/core/authentication.spec.ts --project=firefox` | Pass, 16/16 |
| `scripts/setup_code_integration.sh` | Pass, 22/22 (both images) |
| `scripts/security_headers_integration.sh` (after the H1 fix) | Pass |
| `scripts/go-test-coverage.sh` | Pass, 92.6% line coverage (gate 87%) |
| `scripts/frontend-test-coverage.sh` | Pass, 92.66% lines, 4485 tests |
| `go test -race -count=3` on the setup, setup code, routes and reset paths | Pass |
| `bash scripts/local-patch-report.sh` | Frontend 100%; backend 99.1% (1 line, see L3) |
| `make lint-backend` (full golangci-lint) | Pass, 0 issues |
| `lefthook run pre-commit --all-files` | Pass, all hooks (semgrep included) |
| `npm run lint` (`--max-warnings 0`) | Pass |
| `tsc6 --noEmit`, `npm run type-check` | Pass |
| `npm run build`, `go build ./...` | Pass |
| `./scripts/scan-gorm-security.sh --check` | Pass, 0 critical/high/medium |
| `lefthook run codeql` (Go and JS) | 0 new; Go has 4 existing suppressed results in untouched files; JS 0 |
| `actionlint` on the changed workflows | Pass |
| `shellcheck --severity=error` on the changed scripts | Pass |
| Trivy filesystem scan (vuln + secret, HIGH/CRITICAL) | 0 findings |
| Trivy image scan of `charon:local` (HIGH/CRITICAL) | 3 HIGH with no fixed version, all in bundled third-party binaries; unchanged by this PR (no dependency or Dockerfile changes) |

## Throwaway container gate

Run against a separate container on a random loopback port, not the shared E2E container. All containers and
volumes were removed afterwards.

| Check | Result |
| --- | --- |
| Fresh volume, no environment: exactly one `Setup code:` log line | Pass (1 line, 52-character code) |
| Status before setup | `setupRequired` and `setup_code_required` both true; no other fields |
| UI flow (Firefox, first-run page): wrong code shows the translated alert; correct code returns 201 and leaves `/setup` | Pass |
| Restart after success | No `Setup code:` line; status reports both flags false |
| Restart on an emptied database | One new code, different from the first; the old code is refused (403), the new one works (201) |
| Preset token with debug logging on: wrong and correct attempts | Token absent from stdout logs, response bodies and the data volume (including the log file) |
| Preset shorter than the minimum | Container exits at startup with a length error; the value is not in the logs |

## Security review

- **Code generation and storage.** 32 bytes from `crypto/rand`, base32 without padding. Only a SHA-256 digest is
  kept; the comparison is constant-time over fixed-size digests. A nil or discarded guard accepts nothing.
- **Ordering.** Request binding, then the per-client limiter, then the code check, all before any password
  hashing. The limiter's 429 is returned even for a correct code, as intended.
- **Single use.** The code is discarded only after the write transaction commits. Concurrent correct submissions
  produce exactly one account (unit tests under `-race`, plus the transaction's own recheck). A failed setup
  (for example a short password) keeps the code active.
- **No disclosure.** Rejections use one fixed message for missing and wrong codes and never echo the input.
  The status endpoint adds only `setup_code_required`. A preset is never logged: the startup line only names
  its source. The integration script redacts both values from any debug output; its own output contained no
  code-shaped strings. The E2E rebuild log, E2E container log and local Playwright artifacts did not contain the
  token.
- **Preset length.** Values under 32 characters stop startup with an error that does not repeat the value.
- **Password reset command.** Now goes through the shared password change path, so it ends existing sessions
  and clears lock state; a failed save leaves the session version unchanged (new test).
- **CI.** All six E2E jobs and all five integration jobs that bootstrap an admin generate their own token in a
  step of the same job, mask it, and pass it through step-level `env`. Nothing is written to `GITHUB_ENV` and
  nothing crosses jobs. The new setup code job generates its token inside the script.

## Findings

### High

- **H1 (fixed): security headers integration job could not create its admin.**
  `scripts/security_headers_integration.sh` still posted the first-run request without a setup code, and its
  `integration-tests.yml` job did not provide one, so the login step would fail on every CI run.
  Fix applied: the script now defaults `CHARON_SETUP_TOKEN` to a per-run value, passes it to the container and
  sends it with the setup request (built with `jq`); the job gets a per-job generated, masked token through
  step-level `env`, matching the other jobs. Verified by running the script locally (pass).

### Medium

None.

### Low

- **L1 (fixed): docs.** `docs/getting-started.md` told users to `docker restart` after adding
  `CHARON_SETUP_TOKEN`, which does not apply new environment settings; it now says to recreate the container.
  It and `docs/features/user-accounts.md` now say that only the newest code works when several are logged, and
  that the log entry contains (rather than is) the shown line, since the log is JSON.
- **L2 (fixed): unreachable error branches.** `generateSetupCode` handled an error that `crypto/rand.Read`
  never returns on this Go version, and `IssueStartupSetupCode` re-validated a preset it had already validated.
  Both branches were removed (`pickSetupCode` helper); behavior is unchanged.
- **L3 (accepted): one uncovered patch line.** `backend/cmd/api/main.go:183` is a `log.Fatalf` inside `main()`
  and cannot run in-process. `backend/cmd/api/**` is in the Codecov ignore list, so CI patch coverage is not
  affected. All other changed lines are covered.
- **L4 (open, recommend follow-up issue): surrounding whitespace in the code.** Neither the first-run page nor
  the server trims the submitted code, and a preset read with a trailing newline would count it. A pasted value
  with extra spaces is refused as incorrect. Suggested approach: trim the field before submit and trim the preset
  when loading config.
- **L5 (informational): request bodies in failure traces.** Two Playwright fixtures send the per-run token in
  API calls, and CI uploads traces for retried tests. The token is generated per job, only valid until the first
  account exists, and its container is removed at job end, so this carries no practical exposure. No change made.

## Changes made by QA (uncommitted, in the working tree)

- `scripts/security_headers_integration.sh`, `.github/workflows/integration-tests.yml`: H1 fix.
- `scripts/setup_code_integration.sh`: the "preset not logged" check now also searches for the raw token value,
  not only the log line text (still 22 checks).
- `backend/internal/services/setup_code.go`: L2 cleanup.
- `backend/internal/api/handlers/user_handler.go`: corrected a duplicated step number in a comment.
- Tests: `backend/internal/api/routes/routes_setup_code_log_test.go` (short preset fails registration without
  repeating the value; nothing logged when no setup is pending) and `backend/cmd/api/reset_password_test.go`
  (unhashable password; failed save leaves sessions alone).
- Docs: `docs/getting-started.md`, `docs/features/user-accounts.md` (L1).
