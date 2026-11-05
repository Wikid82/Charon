# QA Report: PR 20 (fix/security-settings-leftovers)

Scope: 4 commits ahead of `development` (#1701 output-limit test, #1741 locales, #1773 e2e helpers, #1774 docs). Tests, JSON and docs only. CodeQL and Trivy are deferred to CI (fix-scoped, no new feature surface).

## Gate results

| # | Gate | Result | Detail |
| - | ---- | ------ | ------ |
| 1a | `go test ./internal/api/handlers -v -count=1` | PASS | `ok` in 207s, no failing test. The earlier one-off failure did not recur. |
| 1b | `go build ./...` | PASS | exit 0 |
| 1c | staticcheck (`make lint-staticcheck-only`) | PASS | 0 issues (backend and agent) |
| 2a | `scripts/frontend-test-coverage.sh` | PASS | 321 files, 4139 passed, 2 todo, 0 failed. Statements 91.11%, Branches 85.61% (6462/7548), Functions 88.42%, Lines 92.11%. Script gate (lines vs 87%) passed. |
| 2b | `npm run type-check` | PASS | `tsc --noEmit` clean |
| 3 | Playwright (project `security-tests`, e2e container at 8080) | PASS | waf-enforcement 6 passed, rate-limit-enforcement 5 passed, security-state-restore 3 passed (setup included). Container healthy, not rebuilt. |
| 4 | markdownlint `docs/cerberus.md`, `docs/api.md` | PASS | 0 issues |
| 4 | Docs accuracy, `docs-site/docs` edits, attribution | PASS | see below |
| 5 | Test diff review | PASS | see below |

Notes:
- Branch coverage margin is thin (85.61% against the 85% floor), but the floor is met.
- While the coverage run was in flight I ran `npm run type-check`, whose `pretype-check` hook runs `npm ci`. The coverage run still finished with all tests green, so the result stands.

## Docs review (`git diff development -- docs`)

Verified against code:
- Metrics: `backend/internal/metrics/metrics.go` defines the three WAF counters with no labels. `IncWAFBlocked` and `IncWAFMonitored` have no callers outside tests (GH #1848). `IncWAFRequest` is called only from the Cerberus middleware (`internal/cerberus/cerberus.go:185`), which wraps the management API, so "management API only, blocked/monitored always 0" is correct.
- SecRuleEngine: `internal/caddy/manager.go:~428` prepends the header only when the lowercased content does not contain `secruleengine`. The docs wording ("own `SecRuleEngine` line") is accurate in practice. The code check is a substring match, so a mention in a comment would also suppress the header. This is a minor precision gap in the docs, not a blocker.
- Burst: `StandardRateLimitBurst = 20` is a default only. The proxy limit in `internal/caddy/config.go:1455` says it has no separate burst parameter. The management API limiter (`internal/cerberus/rate_limit.go`) uses burst. Matches the docs.
- Roadmap rows (WAF complete, CrowdSec local agent complete) are consistent with the Active/Complete status elsewhere in the page.
- The `docs/api.md` anchor `#allowed-values` exists (line 154). `docs/features/rate-limiting.md` exists.
- No files under `docs-site/` changed. No session ids, `claude.ai/code` links or co-author lines in the diff.

## Test diff review

- `crowdsec_cscli_contract_test.go`: the `fx sleep 5` typo was fixed to `exec sleep 60`. The wall-clock `Less(15s)` assertion was replaced with `require.NoError(ctx.Err())`. If the executor did not kill the command, the sleeper would outlive the 30s context and fail the test deterministically, with no timing flake under load. Correct.
- `security-helpers.ts`: `captureSecurityState` is now the shared `snapshotSecurityState`, and `restoreSecurityState` is re-exported. CrowdSec capture and restore are kept separate (`captureCrowdsecEnabled`, `restoreCrowdsecEnabled`), as intended. `restoreCrowdsecEnabled` writes only when the value differs. `withSecurityEnabled` and the crowdsec-enforcement spec call it after the snapshot restore. `crowdsec-whitelist.spec.ts` uses the new capture function. The `CapturedSecurityState` to `SecurityStateSnapshot` rename is applied consistently in the acl, combined, rate-limit and waf specs.
- `security-state-restore.spec.ts`: the header documents that absent keys cannot be deleted through the API, so effective state is restored. The test asserts effective switches through `expectModuleSwitchesRestored`. Passes.
- `i18n.test.ts` adds the `proxyHosts` parity root. The de/es/fr/zh `translation.json` additions (5 keys each) pass it.

## Blockers

None.

## Deferred to CI

CodeQL Go/JS, Trivy, the full Playwright suite and cross-browser runs. The GORM scan is not applicable (no model changes).
