# QA Report: PR 19 (security settings value checks and compare-and-restore), branch `fix/security-settings-checks`

Date: 2026-11-05. Scope: 8 commits ahead of `development` (plan: `docs/plans/current_spec.md`; plan commit 3, the CrowdSec Start revert unification, was intentionally dropped). Tree clean at start and at end. CodeQL and Trivy are deferred to CI (`fix:` scope, no new endpoint or UI). The frontend is untouched.

## Result: PASS

No blocking findings. Three low observations and the residual risks are listed below.

| Gate | Result |
| --- | --- |
| `cd backend && go build ./...` | Pass |
| `./scripts/scan-gorm-security.sh --check` | Pass: 0 critical, 0 high, 0 medium (2 informational suggestions, 1 suppressed) |
| `scripts/go-test-coverage.sh` | Pass: statements 93.2%, lines 92.1% (minimum 87%); no failing package |
| `cd frontend && npm run type-check` | Pass (no frontend files changed on the branch) |
| `bash scripts/local-patch-report.sh` | Pass: overall 213/227 changed lines (93.8%); backend 213/227, frontend 0/0; artifacts in `test-results/` |
| `lefthook run pre-commit --all-files` | Pass: all 18 hooks green, including semgrep (657 s), golangci-lint fast, go vet, markdownlint, frontend lint and type-check |
| Container gates (a), (b), (c), (d) | Pass, details below |

Patch coverage detail: the uncovered changed lines are error-return branches only, `backend/internal/api/handlers/security_handler.go` 626-627, 629-630, 636-637, 647-648 (storage failures inside the disable write) and `backend/internal/models/security_module_state.go` 439-440, 468-469, 493-494 (restore-transaction failure paths). Both files are above the 85% backend floor for the patch.

## Container gates

Two images were built for the comparison: `qa19-branch` (this branch) and `qa19-base` (a `git archive` of `development`, built outside the repo tree). Throwaway containers used unique names, loopback-only ports and their own bind-mounted data directories. The live `charon` and `charon-e2e` containers were not touched. All throwaway containers, data directories and both images were removed afterwards.

### (a) Value checks on `POST /security/config` (branch image, fresh data)

Each of these returned 400 with the documented message, and the full config (including `updated_at`) read back byte-identical afterwards:

- `waf_paranoia_level` 0, 5 and 99
- `rate_limit_requests` 100001 and -1
- `rate_limit_burst` 10001 and -1
- `rate_limit_window_sec` 86401
- `rate_limit_mode` `"garbage"` with `rate_limit_enable: false`, and `"Enabled"` (case)
- `rate_limit_bypass_list` `not-an-ip`, and `1.2.3.4, 10.0.0.0/8, bad`
- `crowdsec_api_url` `ftp://x`

These returned 200: `waf_paranoia_level` 1 and 4; requests/burst/window at 100000/10000/86400 and at 0/0/0; the UI limits 1000/100/3600; a bypass list `1.2.3.4, 10.0.0.0/8,,` (blank entries skipped); an empty bypass list; `crowdsec_api_url` `https://example.com:8080` and empty; mode `enabled` and `disabled`; and a save that echoes the whole row read from `GET /security/config`. The Rate Limiting page itself was not driven in a browser (no frontend change); the 1000/100/3600 values were saved through the API.

### (b) Refused-apply restore

Forcing method: a proxy host whose `advanced_config` carries an unknown handler is refused at create time (the host is rolled back and not stored, 500), so it cannot be used to make a later apply fail. The plan's fallback was used instead: the Caddy process inside the container was frozen with `SIGSTOP`, so the admin `/load` call times out after 30 s and the rollback also fails. The save answers 500 `Failed to reload configuration`; Caddy was resumed afterwards.

- Save of `waf_mode`, `rate_limit_requests` and `rate_limit_burst` with the apply refused: the module keys (`security.waf.enabled`, and others) and the row columns returned to the prior values; settings keys kept their original `updated_at`.
- Concurrent write: while the apply was stalled, `security.waf.enabled` was changed to a different value through a direct database write (the writer's own change to it was already committed). After the 500, that key kept the concurrent value, the row columns were restored, and the log carried exactly one `security state restore skipped: a concurrent write changed this value` warning with `target=security.waf.enabled`. The warning is written to the application log files under `/app/data/logs` (it appears once per file, the same event in `charon.log` and `cpmp.log`), not to the container's stdout.

### (c) Upgrade comparison on one data volume (base image, then branch image, same volume)

- Key-only-enabled instance (`POST /security/cerberus/enable`, row `Enabled=false`, rate limiting configured at 2 requests per 60 s on a proxied host): on the base image and then on the branch image the status body was identical (cerberus on, rate limit enabled and enforced), the settings and row were identical, and a proxied request sequence returned 2 x 200 then 3 x 429 on both. No enforcement turns off at upgrade.
- Stale keys `true` with row `Enabled=false`, then `/security/disable` from localhost (authenticated, run inside the container): base image left both keys `true`, status cerberus on, and every request 429 (still enforcing). Branch image set both keys to `false`, status cerberus off and rate limit off, and every request returned 200.
- The same Disable on the branch image with the apply refused (Caddy frozen): response 500, both keys and the row `Enabled` returned to their prior values, key `updated_at` unchanged.
- Fresh install on the branch image was exercised by gate (a). Gate (e) does not apply (plan commit 3 dropped). Gate (vi), the emergency reset comparison, was not part of this run.

### (d) Blank-valued feature key

With `feature.cerberus.enabled` and `security.cerberus.enabled` set to the empty string and the row off, a `waf_mode: block` save with the apply refused showed both keys as `true` mid-flight (so the check is not vacuous), and after the 500 both read back as the empty string with their original `updated_at`. They were not left as `true`.

## Observations (not blocking)

1. Low. A refused apply restores the row's imaged column values but not the row's own `updated_at`: after a restore the row shows a newer modified time (seen in gates (b) and (c)). Values are correct. The settings keys do keep their modified time. The base behaves the same, so this is not a regression. Files: `backend/internal/models/security_module_state.go` (row restore).
2. Low, documented limit 4 in `backend/internal/models/security_module_state.go`. `waf_paranoia_level` and `crowdsec_api_url` are not imaged, so a config save that changes them and then fails the apply answers 500 but leaves the new values stored (observed with paranoia 3 to 2). Pre-existing and unchanged by this branch.
3. Low. The restore warning goes to the file logger and not to stdout, so an operator following `docker logs` will not see it. Consistent with the other application warnings.

## Residual risks

- The two restore transactions (settings, then row) are separate, so a concurrent write can win on one and lose on the other (documented limit 2, covered by a unit test). A concurrent write that stores exactly the writer's after-image value is overwritten by the restore (limit 1).
- The concurrent-write gate used a direct database write during a stalled apply rather than a second API request, because a second API write would also have been refused and restored.
- The restore does not re-apply the proxy configuration after a concurrent writer's apply (limit 3); the next apply corrects it.
- Frontend coverage and frontend tests were not re-run, because no frontend file changed.
- CodeQL and Trivy are deferred to CI as per the `fix:` scope rule.
