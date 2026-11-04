# QA Report: PR 18 (a WAF rule set needs rules), branch `fix/waf-rule-sets-need-rules`

Date: 2026-11-04. Scope: 7 commits ahead of `development` (plan: `docs/plans/current_spec.md`), tree clean at start and at end. CodeQL and Trivy are deferred to CI (fix-scoped change, no new endpoint or component). The docs-site build was not run locally; it runs in CI.

## Result: PASS

No blocking findings. One low observation and some residual risks are listed below.

| Gate | Result |
| --- | --- |
| `cd backend && go build ./...` | Pass |
| `./scripts/scan-gorm-security.sh --check` | Pass: 0 critical, 0 high, 0 medium (2 informational suggestions, 1 suppressed) |
| `scripts/go-test-coverage.sh` | Pass: statements 93.2%, lines 92.1% (minimum 87%); all packages ok |
| `scripts/frontend-test-coverage.sh` | Pass: 321 files, 4138 tests passed, 2 todo; statements 91.11%, branches 85.61%, functions 88.42%, lines 92.11% |
| `cd frontend && npm run type-check` | Pass |
| `bash scripts/local-patch-report.sh` | Pass: overall 48/48 changed lines (100%); backend 45/45, frontend 3/3; artifacts in `test-results/` |
| `lefthook run pre-commit --all-files` | Pass: all 18 hooks green (go vet, golangci-lint fast 0 issues, markdownlint 0 issues, actionlint, shellcheck, frontend type-check and lint with 0 errors, semgrep 367 rules on 2367 files with 0 findings) |
| Playwright `tests/security/waf-config.spec.ts`, project `security-tests` | Pass: 20 of 20 (including both "Rule set needs rules" cases) |

The branch margin on frontend branches is thin (85.61% against a floor of 85%); it did not drop from before the change.

## Container gates

Run against the rebuilt E2E image (`docker-rebuild-e2e`) and a throwaway pair of containers (a second `charon:local` instance as the upstream, a bind-mounted data directory, a private network, host ports 28080 and 28180). The live `charon` container was not touched. The throwaway containers and network were removed.

- URL-only create (`source_url` set, empty content) returned 400 with `error_code: ruleset_empty`.
- Comment-only create (`# just a comment` and a commented `SecRule`) returned 400 with `error_code: ruleset_empty`.
- Updating an existing set with whitespace-only content returned 400 `ruleset_empty`; the stored set kept its 76-byte content.
- A real `SecRule` set saved (200) and was written to a ruleset file.
- A legacy empty row (seeded directly in the database, because the API now refuses it) selected as the WAF source:
  - `/security/status` reported `enforced: false`, `reason: ruleset_empty`, `unprotected_hosts: 1`.
  - No ruleset file was written for it and the generated proxy config had no WAF handler for the host.
  - One apply produced exactly one Warn, `skipping WAF rule set that has no rules`, plus the existing summary Warn that names the reason. The log counter went from 3 to 4 across a single config save.
  - An attack request through the proxy returned 502 (not blocked, since nothing was enforcing; the upstream path does not exist).
- Selecting the real set as the source: `/security/status` reported `enforced: true`, `unprotected_hosts: 0`, and the same attack request returned 403.

## Notes on test coverage claims

- Locale placeholders: `wafConfig.sourceUrlHelper` was reworded in en, de, es, fr and zh. The other 23 locales carry the English sentence as a placeholder (verified: 23 locale files are string-identical to `en`). No claim is made about translation quality for those 23.
- E2E case (c), a legacy empty row shown as the passive line: the Playwright harness cannot seed such a row, because the API now refuses to store one. That behavior is covered by the Vitest row in `SecurityEnforcementNote.test.tsx` and by the Go enforcement-report test (and was confirmed by hand in the container check above).

## Observations (not blocking)

1. Low: when a host carries its own `advanced_config` WAF handler that names a valid rule set, and the global source is an empty legacy row, the proxy config does contain the host's handler and an attack request is blocked (403), yet `/security/status` still counts that host in `unprotected_hosts` (1). Seen on the first pass before the host's own handler was removed. The status counts hosts that lack the global handler and does not look at per-host handlers. This was not changed by this PR and is conservative (it under-reports protection, never over-reports). It can be filed as a separate follow-up.

## Residual risks

- A user who relied on an empty rule set row (including the removed placeholder preset) sees the WAF go from "applied" to "not applied" after upgrading. The row is kept, only the activation stops, and the Security page shows the passive reason line. This is the intended behavior.
- An `Include` of a missing file is not detected by the rule-line check (accepted in the plan).
- CodeQL and Trivy have not run locally; CI runs both on the PR.
- Full-suite and cross-browser Playwright runs are CI-only and were not run.
