# QA Report - fix/harden-backend-consistency-checks

Date: 2026-10-06. Scope: 12 commits, `bb787410..HEAD` (HEAD 95d31201). Backend only, no frontend changes.
Verdict: **PASS, no blocking findings.**

> Security-sensitive detail (specific weaknesses, residual risks, and re-test considerations) has been
> intentionally redacted from this public report. It is tracked privately per `SECURITY.md`.

## Definition of Done

| # | Item | Result | Evidence |
|---|------|--------|----------|
| 1 | Targeted Playwright E2E, firefox only | PASS | E2E image rebuilt with the `docker-rebuild-e2e` skill. Final HEAD: 11 spec files covering first-run onboarding, proxy hosts, redirection hosts, certificates (delete, bulk delete, list) and the Caddyfile/NPM/JSON import flows. 178 passed, 1 skipped, 1 flaky-under-load failure (`caddy-import-gaps` 4.2, a session-resume test that depends on leftover import-session state from earlier specs in the same run). It passes in isolation and the whole `caddy-import-gaps` file passes 12/12 when re-run. A prior run on the first eight commits had 0 failures. |
| 1.5 | GORM security scan | PASS | `scan-gorm-security.sh --check`: 0 CRITICAL, 0 HIGH, 0 MEDIUM. |
| 2 | Local patch coverage | PASS | Backend/overall patch coverage 96.7% (264 of 273 changed lines; thresholds 90 overall, 85 backend). Uncovered changed lines are defensive database-error branches. |
| 3 | Backend coverage | PASS | `scripts/go-test-coverage.sh` on the first eight commits: statements 92.6%, lines 89.8% (gate 87%). Package coverage at final HEAD: handlers 90.5%, services 91.0%. Frontend coverage, type-check and build skipped: the frontend is untouched. |
| 4 | Lint / static analysis | PASS | `make lint-fast`: 0 issues (backend and agent). `lefthook run pre-commit --all-files`: all hooks passed, semgrep 0 findings. |
| 5 | Build and full test run | PASS | `go build ./...` OK (backend and agent). `go test ./... -race -count=1` passes with zero failures. |
| 6 | Vulnerability scans | PASS | `govulncheck`: 0 vulnerabilities affecting the code. CodeQL and Trivy are deferred to CI per `CLAUDE.md` for a `fix:`-only branch. |

## Review summary

- Concurrency-sensitive writes now run inside a shared write-locked transaction helper. All callers were reviewed for correct transaction-handle use. The concurrency tests use multi-connection databases, which is a stronger scenario than production's single-connection pool.
- Domain handling is consolidated into shared helpers used by the proxy host, redirection host and import code paths. No remaining non-test callers of the removed helpers were found.
- Error responses on the changed paths are generic, with detail logged server-side.
- Commit subjects for `(security)` commits are intentionally generic. No session identifiers or links appear in commit messages or the diff.
- Dead code identified in review was removed in this branch.

## Findings

### Blocking
None.

### Non-blocking
Several low-severity, non-blocking items were identified during review. The in-scope ones were fixed in this
branch (dead-code removal, input normalization, reference validation inside the write transaction, and
certificate-handler status codes). The remainder are tracked privately or as plain follow-ups and are not
listed here.

## Not verified / limits

- Per-commit buildability (bisect) was not checked. HEAD builds and passes everything.
- CodeQL and Trivy were not run locally, by policy. CI runs them.
- Concurrency behavior was judged from the code and the multi-connection tests; no live-database stress run beyond the test suite was performed.
- The security review and full QA pass were done on the first eight commits. The four later commits (cleanup, input normalization, reference validation, test fixture) were verified by build, full `-race` test run, lint, GORM scan, patch coverage and the targeted E2E re-run above, but did not get a second full manual review.
