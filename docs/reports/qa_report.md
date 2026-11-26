# QA Report: Proxy Apply Error Messages

- Branch: `fix/apply-error-redaction` (5 commits on `origin/development`, head `1cbf8bbd6`)
- Plan: `docs/plans/pr_f3_apply_error_redaction_plan.md`, sections 4.7 (edge cases), 6 (commit gates) and 7 (test
  plan, container gates, Definition of Done)
- Scope: errors from Caddy's admin API carry Caddy's message, cut to at most 2 KiB, with credential values from the
  sent configuration replaced by `[redacted]`; the stored apply message is cut to at most 4 KiB; the security status
  returns `config_apply.error_msg` to administrators only; the message text of earlier stored apply results is
  cleared once at start-up and again after a live restore of a backup without the clean-up marker; Caddy's
  `admin.api` logger is kept out of Caddy's default log; the credential key list is shared between the redaction
  helpers; the secure-delete wrapper is shared in `internal/database`.
- Image under test: built from the head above (`prf3qa-image`). All gate containers and volumes were throwaway
  (`prf3qa-*`, published on `127.0.0.1` high ports, rootless daemon) and were removed afterwards. The live
  containers `charon-dev` and `orthrus-dev` were not touched.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.5, lefthook 2.2.3.
- Date: 2026-11-26.

## Verdict

**Approved.** No blocking items. One test-only change was made during QA (see "Changes made during QA"). Every test
suite, build, lint, scan and container gate that was run passes. The 200-host timing gate (plan G4) was not run.

## Definition of Done

| Gate | Result |
| --- | --- |
| Playwright, firefox only: `tests/core/proxy-hosts.spec.ts` | 45 passed |
| GORM security scan (`./scripts/scan-gorm-security.sh --check`) | 0 critical, 0 high, 0 medium (2 informational) |
| Local patch coverage (`bash scripts/local-patch-report.sh`) | backend 92.5% of 252 changed lines; frontend and agent unchanged |
| Backend coverage (`scripts/go-test-coverage.sh`) | 94.2% statements, 93.2% lines, no failures |
| Frontend coverage (`scripts/frontend-test-coverage.sh`) | 92.06% statements, 92.91% lines; 380 files, 4734 tests passed |
| `go test -race` on `redact`, `crowdsec`, `caddy`, `database`, `api/routes`, scoped `services` and `api/handlers` | all pass |
| CodeQL Go and JS (`lefthook run codeql`) | no results in changed files |
| `lefthook run pre-commit --all-files` | all hooks pass after the QA change below |
| `go build ./...`, `go vet ./...` | pass |
| Frontend type-check and build | not needed (no frontend change); type-check passed in the pre-commit run |
| `make lint-backend` (run last) | 0 issues |
| Trivy | left to CI (no new feature surface) |

## Patch coverage detail

| File | Patch coverage | Uncovered changed lines |
| --- | ---: | --- |
| `backend/internal/database/secure_delete.go` | 65.5% | 26-27, 29-30, 33-34, 41-42, 52-53 (database error returns) |
| `backend/internal/services/caddy_apply_history.go` | 82.9% | 34-35, 44-45, 62-63 (database error returns) |
| `backend/internal/redact/text.go` | 95.1% | 64, 89-90 (sort tie-break and encoder error return) |

All uncovered lines are error returns or tie-breaks. The totals are above the 85% gate and the 87% target.

## Container gates

| Gate | Check | Result |
| --- | --- | --- |
| G1 | Production-mode container, Cloudflare DNS provider with a fake malformed token, wildcard proxy host using it | `500 proxy_apply_failed`; the message keeps Caddy's reason ("API token '[redacted]' appears invalid; ensure it's correctly entered ...") |
| G1 | The fake value in the container log, the 500 answer, `caddy_configs.error_msg`, `GET /security/status` (admin and `user` role), the whole data directory (database, WAL, log files) and an unzipped backup | 0 occurrences everywhere |
| G1 | `GET /security/status` as administrator / as `user` role | `error_msg` with the redacted message / no `error_msg` key |
| G1 | Control run on release `0.51.6` with the same steps | recorded for comparison |
| G2 | Caddy `admin.api` lines after Charon's first apply | none; the only `admin.api` lines are the boot-config requests at start-up; no "request error" line |
| G3 | Branch image started on a data volume from `0.51.6` holding a stored failure message | one Info line "Cleared the message text of earlier proxy apply results" with count 1; `error_msg` empty on every row; marker `migration.caddy_apply_error_text_v1` set; the value absent from the database file and WAL |
| G3 | Live restore of a backup taken on `0.51.6` | restore completed with live rehydrate; the fix-up cleared the restored row (count 1) and wrote the marker; the pre-restore backup it took holds no value |
| G3 | Restart after the restore | no further clearing line |

## Commit and text checks

- Exactly one `(security)` subject; every commit body is empty.
- No advisory or scanner identifiers in commit messages.
- Added lines contain none of the reserved words listed in the plan.
- Added docs describe behaviour only.

## Changes made during QA

- `backend/internal/caddy/client_admin_error_test.go`: a `nosemgrep` annotation with a justification on the fake admin
  server's write (test-only `httptest.Server`, same convention as the existing fake S3 servers). Semgrep blocked the
  pre-commit run on this line; after the annotation the file scans clean and its tests pass.
- Same file: a `#nosec G101` annotation with a justification on the fixture constant in
  `TestLoadJSONRedactsJSONEscapedAndGoQuotedValues` (same convention as the DNS provider tests). The full backend
  lint (gosec on test files, as in CI) reported it; the fast pre-commit lint does not check test files.

## Observations

- `docs/troubleshooting/dns-challenges.md` says the message "never shows" passwords and API tokens in full. The
  replacement covers values in the sent configuration plus common patterns, so "never" is stronger than the
  mechanism. Suggested wording: "Passwords and API tokens are replaced with `[redacted]`."
- The same page says "Caddy's request lines are not written there"; only Caddy's admin request lines after the first
  apply are left out (the boot-config requests at start-up still appear). Suggested wording: "Caddy's admin request
  lines are not written there after Charon applies its settings."
