# QA and Security Report: fix/diagnostics-sensitive-values

Date: 2026-10-12
Scope: full diff `origin/development...HEAD` (3 branch commits plus the 2 QA commits listed below) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved spec. The branch hardens how sensitive values are handled in diagnostic output (logs, returned errors and API responses) for the CrowdSec bouncer registration and enrollment paths, consolidates the display-masking helper into `util.MaskAPIKey`, adds `util.DescribeCommandOutput`, and adds a short operator guide.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Two low findings and two informational notes are listed; none blocks the merge. No model, migration or GORM query file is in the diff, so the GORM scan was not required.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Build | `cd backend && go build ./...` | pass (no frontend files changed) |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` | 0 issues |
| Race run | `go test -race -timeout 60m ./internal/services/... ./internal/util/... ./internal/api/handlers/... ./internal/crowdsec/... ./internal/caddy/...` | pass (services 578 s, crowdsec 109 s) |
| Backend tests and coverage | `scripts/go-test-coverage.sh` | 0 failing tests; statements 91.7%, lines 90.7% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (strict; empty frontend lcov placeholder because no frontend file changed, removed afterwards) | overall 98.2% (55/56), backend 98.2%, frontend n/a (0 changed lines); artifacts `test-results/local-patch-report.{md,json}` produced |
| CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CLI 2.27.2) | 4 results, all in files outside this diff (`remote_server_handler.go`, `uptime_service.go`, `auth_handler.go`); no clear-text-logging or sensitive-data result |
| Trivy | `trivy fs --severity HIGH,CRITICAL --scanners vuln,secret` | no findings |
| Markdown lint | `make lint-markdown` | 0 issues |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| E2E | not run | no UI or E2E spec depends on the changed `details` field or key previews (searched `tests/` and `frontend/src`) |

## Security review

1. Raw key and command output: `registerAndSaveBouncer`, the startup registration, `RegisterBouncer` and the enrollment registration step now report only a byte count of command output (`util.DescribeCommandOutput`); the `RegisterBouncer` error response carries a fixed `details` string. All key log fields and API previews go through `util.MaskAPIKey`. Tests capture log output, the broadcast hook entries (message, data, error and raw fields) and the returned errors, and assert that planted values never appear.
2. Sibling sweep (`backend/`, `agent/`, `scripts/`, `configs/`): remaining `Warn/Info/Error/WithField/WithError` calls near key, token, secret, password or output either log masked values, counts or names, or concern non-credential output. No remaining path found that logs a credential or credential-producing output.
3. `util.MaskAPIKey`: empty gives `[empty]`; below 24 bytes gives `[REDACTED]`; 24 or more shows 4 + 4 characters. Boundary tests at 15, 16, 23 and 24 pass. The length is in bytes, so a multi-byte key reveals at most four bytes at each end, and a cut inside a multi-byte character only affects display. `util.DescribeCommandOutput` reports empty or length only; tests cover nil, empty, short, multi-line and 100 kB input.
4. Admin gating: the bouncer key, key-status and registration routes are mounted under the admin-only group in `routes.go`; the new test registers the real routes behind `RequireRole(admin)` and asserts a user-role request gets 403.
5. Tests are not vacuous: they use distinct planted values, assert both the replacement text and absence of the planted value across every output channel, and check the saved key file keeps the full value with mode 0600.
6. Public wording: commit subjects and bodies, docs, code comments and test names were checked; the only `(security)` subject is `fix(security): tighten handling of sensitive values in diagnostics`, which is generic. Three test names and one code comment that hinted at the previous behaviour were reworded (commit below). No session id or link appears in any message.

## Findings

No critical, high or medium findings.

### Low

- **L1 (pre-existing, unchanged scope).** `RegisterBouncer` still returns an 8-character preview of the registered key in its success response (admin only). Consider aligning it with `util.MaskAPIKey`.
- **L2 (pre-existing wording).** Operator-facing text still advises removing or updating the compose environment variable although the saved key file takes precedence at runtime:
  - `backend/internal/api/handlers/crowdsec_handler.go` line 1579 (key-status message), line 1641 (log message), lines 1771-1772 (registration banner tip)
  - `frontend/src/locales/*/translation.json` key `keyWarning` (all locales)
  Recommended: one generic low issue after merge.

### Informational

- **I1.** Pre-existing CodeQL log-injection results (4) remain in untouched files.
- **I2.** `GetBouncerKey` intentionally returns the full key to administrators.

## Commits added by this review

- `chore: neutralize wording in diagnostics tests` (renames three tests and trims one comment; no behaviour change).
- `docs: update QA report for the diagnostics hardening` (this report).
