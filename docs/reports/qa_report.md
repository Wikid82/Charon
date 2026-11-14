# QA Report: Security Settings and Headers Consistency

- Branch: `fix/security-settings-and-headers-consistency` (24 commits on `origin/development` `7b4e2f936`, head
  `51da424d7`)
- Scope: security switch reading and writing (settings API, bulk config, feature flags), restore of security state
  after a refused proxy apply, CrowdSec start revert, config writers under the write lock, rate limit mode clearing,
  rule set engine line detection, "stored as sent" create for header profiles, proxy hosts and invited users,
  `X-XSS-Protection: 0`, metrics served on `/metrics`, role-based response shapes for security settings, three UI
  fixes, two test tooling fixes, and the matching docs.
- Rounds: round 1 tested the whole branch. Round 2 re-tested the follow-up changes: URL test refusals counted on
  `/metrics`, an atomic feature flag batch that carries the security suite flag, and the QA fixes from round 1.
- Images under test: the existing `charon:local` E2E image (see "E2E image") and throwaway images built from the
  working tree in each round (`pr24qa-charon:qa`, `pr24qa-charon:qa2`) for the container gates and both integration
  scripts.
- Toolchain: Go 1.27.4, CodeQL CLI 2.27.2.
- Date: 2026-11-14

## Verdict

**Approved.** Every Definition of Done gate passes. The container gates confirm switch validation, the restore
paths (including the whole feature flag batch), the "stored as sent" creates, the header value, the role-based
response shapes, the URL test metrics and the regression items end to end.

One limit is accepted and tracked in #1848: the proxied blocked counters only count blocks the access log identifies
as WAF or CrowdSec blocks, and a real Coraza block is not identified today. The docs say the counters are a lower
bound. See "Accepted limit".

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| E2E image (`docker-rebuild-e2e`) | Not rebuilt. `charon:local` was built after the last round 1 application commit, and its binary contains this branch's new metric, flag and field names. The round 2 backend changes (URL test refusal metrics, feature flag batch) are not in that image; they were verified in the round 2 throwaway container instead, and CI runs the specs on a fresh image |
| Playwright `--project=security-tests`: `security-headers`, `rate-limiting`, `security-config-field-update`, `crowdsec-decisions`, `security-headers-enforcement`, `authorization-rbac`, `emergency-reset` | 199 passed, 1 failed (flaky helper, fixed in round 1, see "QA fixes"). After the fix `crowdsec-decisions` passed 80/80 with `--repeat-each=3` |
| Playwright `--project=security-tests`: `security-dashboard`, `system-settings-feature-toggles`, `waf-config`, `security-state-restore`, `zzz-security-ui/system-security-settings`, `suite-integration`, `rate-limit-enforcement`, `waf-enforcement`, `crowdsec-preset-apply`, `crowdsec-security-toggle-busy`, `crowdsec-config` | 124 passed, 1 skipped, 2 failed while QA ran lint on the same 4-core host. Both (toast checks with a 5 s timeout) passed 6/6 when re-run on an idle host. Not related to this branch |
| Playwright round 2: `system-settings-feature-toggles`, `zzz-security-ui/system-security-settings` | 30 passed, 1 skipped |
| Playwright `--project=firefox`: `core/proxy-hosts`, `settings/user-management`, `settings/smtp-settings`, `settings/account-settings`, `settings/user-lifecycle` | 106 passed, 21 skipped (existing state-dependent skips) |
| `./scripts/scan-gorm-security.sh --check` | Pass, 0 critical/high/medium, 2 informational |
| `bash scripts/local-patch-report.sh` (baseline `origin/development`, round 2) | Pass. Overall 92.0% (531/577), backend 91.8% (516/562), frontend 100% (15/15). Uncovered lines are error branches (storage failures, logging); no file needs follow-up |
| `scripts/go-test-coverage.sh` (round 2) | Pass, 93.8% statements / 92.8% lines (gate 87%), no failures |
| `scripts/frontend-test-coverage.sh` | Pass. 4605 tests, lines 92.78%, statements 91.92%, branches 86.6% (no frontend change in round 2) |
| `cd frontend && npm run type-check` / `npm run lint` | Pass / pass (0 warnings) |
| `lefthook run pre-commit --all-files` (round 2) | See "Round 2 hooks and lint" |
| CodeQL Go (`codeql-go-scan.sh`, round 1) | 4 results, all existing, none in a file this branch changes. No new result |
| CodeQL JS | Not run: the frontend changes are labels, read-only states and keyboard handling with no new data flow; CI runs it on the PR |
| `scripts/security_headers_integration.sh` (throwaway image, own prefix and ports) | Pass, every check. The script no longer edits the database: the plain host is created with headers off through the API. No header is duplicated on HEAD, GET, a streamed response or a 502 |
| `scripts/coraza_integration.sh` (throwaway image via `CHARON_IMAGE`) | Pass (403 in block mode, 200 in monitor mode). Ports bound to 127.0.0.1 only; no container, volume or network left behind |
| `make lint-backend` (run last) | See "Round 2 hooks and lint" |

## Container gates

Both rounds used throwaway `pr24qa-*` containers on their own network and volumes, removed afterwards, with one
admin and one standard user and a busybox upstream. A refused proxy apply was produced by pausing the Caddy process
inside the throwaway container.

| Check | Result |
| --- | --- |
| Security suite switch writes accept only true/false | Pass. `maybe`, `1`, `yes`, `on`, empty and `garbage` answer 400 `"<key> must be true or false"` on `POST /settings` (both keys) and `PATCH /config` (nested form); `PUT /feature-flags` refuses non-boolean JSON with 400. Nothing is stored. `" TRUE "` and `"False"` are stored as exactly `true`/`false` on both keys. Standard users get 403 |
| Switch writes apply to the proxy | Pass. Each accepted write applies the proxy once. With the apply refused, `PUT /feature-flags` and `POST /settings` answer 500 `Failed to reload configuration` and both keys keep their previous value |
| Feature flag batch with the switch (round 2) | Pass. With the apply refused, a batch of the switch and another flag answers 500 and every key in the batch, update time included, is unchanged. When the other flag's write fails (forced with a database trigger in the throwaway database), the batch answers 500, neither key changes and no proxy apply runs. A batch without the switch and a switch-only batch both still work |
| Response shapes by role | Pass. Standard users get `/security/config` without the admin allow-list, the rate limit bypass list and the CrowdSec address, with `has_admin_whitelist` and `has_rate_limit_bypass_list`; `/security/rulesets` without rule content and source address, with `has_content` and `content_line_count`; `/settings` without the allow-list and mail server details, with `<key>.has_value`. Admin bodies keep every field |
| Bulk settings reply | Pass. `PATCH /config` answers the mail password as `********` with `.has_secret` and `.last_updated`; the raw value never appears |
| Other read paths | Pass. Marker values were stored in every admin-only field, then a standard user requested all 126 registered GET routes (62 answered 200), read and downloaded every log file, and held the live log and security event streams open while an admin saved those fields. No marker value appeared |
| Invited user | Pass. Created disabled |
| Header profile create | Pass. `hsts_enabled:false`, `xss_protection:false`, `x_frame_options:""`, `x_content_type_options:false` are stored and returned as sent; client preset markers are ignored; a profile without those keys gets the defaults |
| Proxy host create and update | Pass. `enabled`, `http2_support`, `block_exploits`, `security_headers_enabled` sent as false stay false (admin and standard user), the host keeps exactly one location, omitted fields get the defaults, and update toggles `security_headers_enabled` |
| `X-XSS-Protection` | Pass. A proxied host whose profile has it on sends `0`; a profile with it off sends no such header; Charon's API and page responses send `0` |
| WAF exclusion add/delete with the apply refused | Pass. Both answer 500 `Failed to reload configuration`; the stored exclusions and the row's update time are unchanged. Duplicate 409, missing 404 and standard user 403 unchanged |
| `/metrics` (round 2) | Pass. URL tests refused for private, loopback and metadata addresses move `charon_ssrf_blocks_total{ip_type="private"}` (3) and `{ip_type="metadata"}` (1), and `charon_url_validation_total{result="blocked"}` with reasons `private_ip` and `metadata_endpoint`. A URL test that reaches the network moves `charon_url_test_duration_seconds` and `charon_url_validation_total{result="allowed"}`. `charon_waf_monitored_total` is gone. The blocked counters move for access log entries that the log marks as WAF or CrowdSec blocks |
| CrowdSec start failure | Pass. With the CrowdSec binary made non-executable, start answers 500 and the switch keys, the enabled setting (absent before, absent after) and the row, including its update time, are back to their prior state |
| Concurrent config saves | Pass. Ten exclusion adds, four WAF mode saves, an allow-list save and a bypass list save sent at once all succeeded and every change survived |
| Clearing the rate limit mode | Pass. Saving an empty mode after `enabled` stores the enable flag off, and status reports rate limiting off |
| Rule set engine line | Pass. A rule set that mentions `SecRuleEngine` only in a comment gets Charon's `SecRuleEngine On` line in the generated file, and blocks |

## Regression review of removed or rewritten code

- **Security state restore:** every column a config save can write is imaged, and a full restore puts back the
  row's update time. A concurrent write keeps its columns and stamps the restore time.
- **CrowdSec start revert:** the start writes inside one write-locked transaction with the shared snapshot, and the
  revert uses the shared restore. A row that start created is deleted again. The removed per-handler snapshot
  helpers have no remaining callers.
- **Concurrent saves:** `Enable`, both WAF exclusion writers and the allow-list sync read the row inside the write
  lock.
- **Rate limit mode clearing:** a mode-only save also writes the enable flag from the mode.
- **Rule set directive scanner:** `HasRuleLines` and `HasEngineDirective` share one line scanner (byte order mark,
  comments, continuation lines); only the first word of a directive line counts.
- **Feature flag batch:** the switch and the other flags of one request are written in one transaction with the
  switch snapshot, so a refused apply or a failed write leaves the whole batch as it was.
- **URL test refusals:** the handler and the connectivity check share one refusal recorder, so the labels are the
  same on both paths.

## Accepted limit

**Proxied blocked counters are a lower bound (tracked in #1848).** `charon_waf_blocked_total` and
`charon_crowdsec_blocked_total` count access log entries that identify the block as a WAF or CrowdSec block. In the
round 1 container, three requests blocked by Coraza left the WAF counter at 0: the access log entry of a real block
has no WAF marker, so it is counted as a generic refusal. Entries that carry the markers move both counters, so the
counting itself works. CrowdSec bouncer blocks could not be produced in the container. `docs/cerberus.md` and
`docs/api.md` describe the counters as a lower bound. Follow-up: give the access log a reliable marker for WAF and
bouncer refusals and match it in the log watcher, then re-test with a real block.

## Findings

No open finding in scope. The round 1 low findings (URL test refusals not counted, feature flag batch not atomic)
were fixed and re-tested in round 2.

## QA fixes (round 1, folded into the branch)

- `tests/security/crowdsec-decisions.spec.ts`: the Banned IPs helper stubs the preset catalog with the shared
  one-entry stub and waits for it. Root cause of the flake: the real hub catalog renders above the Banned IPs card
  after the card appears, so the card moved between a click's press and release and the dialog never opened.
- `backend/internal/api/tests/integration_test.go`: the test required the removed monitored metric. It now requires
  the CrowdSec blocked counter and checks that the monitored metric is absent.
- `scripts/coraza_integration.sh` and `.github/skills/integration-test-coraza.SKILL.md`: `CHARON_IMAGE` selects the
  image (default `charon:local`), so a run can use a throwaway image.

## Round 2 hooks and lint

- `lefthook run pre-commit --all-files`: pass, every hook (existing E2E lint warnings only, 0 errors).
- `make lint-backend` (full `backend/.golangci.yml`, gosec included, golangci-lint v2.15.0) ran last, on the
  round 2 head with no code change after it: 0 issues. `agent/` is unchanged, so `make lint-agent` was not needed.
