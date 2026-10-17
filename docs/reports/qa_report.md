# QA and Security Report: fix/crowdsec-cscli-pages-2 (PR 4)

Date: 2026-10-17
Scope: full diff `origin/development...HEAD` (9 commits, HEAD `905485bf`, 165 files) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and section 5 of the batch 5 spec. The branch validates the manual ban and unban requests (addresses, ranges, durations, reasons), routes every cscli call of the ban, unban, listing, alerts and status handlers through one helper with a deadline, reads the saved bouncer key for direct local API calls, returns a typed alert view, maps oversized cscli output to the soft error, decides console enrollment from the status JSON, shows clear ban and unban messages in five languages, and updates the cscli contract and CrowdSec documents.

## Verdict: PASS (no critical or high findings; one medium and several low findings, none blocking)

No command or flag injection, path escape, secret exposure or panic is reachable from authenticated input (adversarial probes below). The one medium finding is a cscli call outside the diff that still has no deadline. No model, migration or GORM query file is in the diff, so the GORM scan was not required. No file was changed by this review except this report.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Frontend tests and coverage | `scripts/frontend-test-coverage.sh` (18 m 58 s) | 301 test files passed; 3742 passed, 4 skipped, 2 todo; statements 90.49%, branches 84.10%, functions 88.46%, lines 91.62% (gate 87% on lines) |
| Backend tests and coverage | `scripts/go-test-coverage.sh` (12 m 34 s, system `tee` did not crash this time) | 0 failing tests; statements 92.1%, lines 91.0% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (run after both coverage runs, strict mode) | overall 245/245 = 100.0%, backend 215/215 = 100.0%, frontend 30/30 = 100.0%, agent 0 changed lines; artifacts `test-results/local-patch-report.{md,json}` exist |
| golangci-lint, full | `cd backend && golangci-lint run --config .golangci.yml ./...` | 0 issues (13.6 s) |
| Strict lint, touched packages | `golangci-lint run --config ../.golangci-fast.yml ./internal/api/handlers/... ./internal/crowdsec/...` | 0 issues |
| Build | `cd backend && go build ./...` | pass |
| Type check and frontend build | `cd frontend && npm run type-check && npm run build` | pass, pass (built in 3.50 s) |
| Race run | `go test -race -timeout 60m ./internal/crowdsec/... ./internal/api/handlers/... ./internal/services/...` (10 m 43 s) | pass: crowdsec 120.986 s, handlers 439.866 s, services 626.510 s, remotestorage 49.087 s |
| CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CLI 2.27.2, 1 m 53 s) | 4 results, all baseline and outside the diff: `go/cookie-secure-not-set` `auth_handler.go:201`, `go/log-injection` `remote_server_handler.go:148` (x2), `go/log-injection` `uptime_service.go:1551`; 0 new |
| CodeQL JS | `scripts/pre-commit-hooks/codeql-js-scan.sh` (2 m 18 s) | 0 results |
| Trivy | `trivy fs --scanners vuln --severity HIGH,CRITICAL` on copies of `go.mod`, `go.sum`, `package.json`, `package-lock.json` under `$HOME` (snap build cannot read the repo path; removed afterwards; DB freshly downloaded) | 0 findings (this PR changes no dependency) |
| Markdown lint | `make lint-markdown` | 0 issues in 266 files |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| E2E | `docker-rebuild-e2e` (24 s, all layers cached because the tree was unchanged since the last build; the `charon` container was not touched), then `npx playwright test tests/security/crowdsec-decisions.spec.ts tests/security/crowdsec-diagnostics.spec.ts --project=security-tests` | 45 passed (2.1 m) |
| E2E dashboard | `npx playwright test tests/security/crowdsec-dashboard.spec.ts --project=security-tests` (the file exists, so it was kept) | 39 passed (2.1 m) |

`backend/internal/changelog/data/changelog.json` was restored with `git checkout` after the E2E runs; `go.work.sum` is unmodified; `git status` is clean.

## Local real-cscli gate (cscli v1.8.2-rc1, image built from the working tree)

A throwaway container `qa-pr4gate-ct` (volume `qa-pr4gate-data`, image tag `qa-pr4gate:tmp`, port bound to 127.0.0.1) ran the image with a deliberately wrong value in `CHARON_SECURITY_CROWDSEC_API_KEY`. The first admin was created through the setup API and CrowdSec was started through the API (`lapi_ready` true). All addresses are documentation addresses. Container, volume and image tag were removed afterwards; the `charon` container (up 4 h, healthy) and `charon-e2e` were never touched.

| Step | Observation | Result |
|---|---|---|
| Ban IPv4 `192.0.2.10`, 4h, reason | 200 `banned`, 0.14 s | pass |
| Ban IPv6 `2001:db8::10`, 1d | 200 `banned` | pass |
| Ban range with host bits `198.51.100.77/24` | 200, answered and stored as `198.51.100.0/24` (masked form) | pass |
| Ban IPv6 range `2001:db8:1::5/48` | 200, stored as `2001:db8:1::/48` | pass |
| Decisions list | all ids, origin `cscli`, type `ban`, scope `Ip` or `Range`, value, remaining duration, scenario (`manual ban: gate range`), created_at populated; no blank field | pass |
| Invalid inputs | `not-an-ip`, empty, missing, `-r`, `--help`: 400 `invalid_ip`; duration `1w`, `-d`, `permanent`: 400 `invalid_duration`; reason of 201 characters: 400 `invalid_reason`; each in 3 to 4 ms, nothing reached cscli | pass |
| Hostile unban paths | `-r`, `--help`, `..%2F..%2Fetc`, `%252F` (double encoding), `192.0.2.22%2F..%2F`, `%00`, `%0A-r`, `0.0.0.0%2F0%2F0`, `%2F`, trailing slash: all 400 `invalid_ip`; empty value 400 `ip is required`; `%2e%2e` 404 from the router; the other decisions were still listed afterwards | pass |
| Unban range, encoded (`198.51.100.0%2F24`) | 200 `{"deleted":1}` | pass |
| Unban range, plain path (`198.51.100.0/24`) | 200 `{"deleted":1}` | pass |
| Unban IPv6 range with encoded colons and slash | 200 `{"deleted":1}` | pass |
| Unban IPv6 address | 200 `{"deleted":1}` | pass |
| Unban missing address and an already-removed range | 404 `decision_not_found` | pass |
| Unban one address of a banned pair (192.0.2.21 / .22) | 200; the other address stays listed | pass |
| Unban a covering range `192.0.2.0/24` while single addresses inside it exist | 404 `decision_not_found`, the single addresses stay (cscli matches the exact scope) | observation |
| Duration forms accepted by the validator and by cscli | `01d`, `.5h`, `1.5h`, `1h30m`, `2.5h`, `90m`, `1m`, `365d`, `8760h` all 200; `0.5d` 400 | pass |
| Saved bouncer key file present, wrong key in the environment | `GET /dashboard/summary` `active_decisions` 3 (not -1) | pass |
| Alerts endpoint | row keys exactly `created_at, duration, events_count, id, ip, message, origin, scenario, start_at, stop_at, type`; no `machine_id`, no uuid; a scenario filter of `--help` returned an empty list, no flag reached cscli | pass |
| Console status, real cscli JSON | `cscli console status -o json` prints `"enrolled": false` for the unregistered engine (decided not enrolled by the new JSON branch); the HTTP console route answered `console enrollment disabled` (feature flag off by default) | pass |
| LAPI killed (process 464 killed inside the container) | ban 504 `cscli_timeout` after 15.02 s; unban 504 `cscli_timeout` after 15.01 s; alerts 200 with an empty list after 30.01 s; decisions list 200 with the soft timeout text after 30.01 s; dashboard `active_decisions` -1 in 4 ms | pass (deadlines hold) |
| Leftover cscli processes after the outage | none (`ps` inside the container) | pass |
| Restart through the API | LAPI came back, ban worked again | pass |
| Oversize path | the cap is 32 MiB plus the executor margin; the normal decisions listing is capped at 100,000 rows and the manual ban flow produces rows of a few hundred bytes, so normal use cannot reach the cap; the over-cap behavior is covered by the unit tests | not triggerable by normal use |
| Secrets | the bouncer key, the wrong environment key and the session token appear nowhere in the container log or in `/app/data/logs`; responses carry no key; the one `password` match in the log is the Gin route table | pass |

## Security review (adversarial)

Probes were Go tests placed temporarily in the packages and deleted afterwards, plus the live calls above.

1. Argument construction (`BanIP`, `UnbanIP`). Values are separate argv entries to `CmdExec.Execute("cscli", args...)`; no shell is involved. `parseBanTarget` trims, then rejects empty, longer than 64 characters, a leading `-`, and any control or space character, then requires `netip.ParseAddr` (no zone) or `netip.ParsePrefix`, and passes the re-rendered canonical form (`addr.String()` or `prefix.Masked().String()`), never the typed text. Probe results: `-192.0.2.1`, `--help`, `-r`, `fe80::1%eth0`, NUL, `192.0.2.1\n-r 0.0.0.0/0`, fullwidth digits `２.０.０.１`, the fullwidth slash `192.0.2.1／24`, a right-to-left override prefix, a zero-width space suffix, `192.0.2.01`, `0x7f.0.0.1`, `/24`, `192.0.2.1/`, `2001:db8::/129`, a trailing slash, 65 and 5,000 characters: all rejected. Accepted and normalized: surrounding whitespace and NBSP are trimmed, `192.0.2.5/24` becomes `192.0.2.0/24`, IPv4-mapped IPv6 `::ffff:c000:201` becomes `::ffff:192.0.2.1` (see L3). The duration text is passed as typed, but only after it matched `^\d+d$` or `time.ParseDuration` with a leading digit or dot and a range of 1 minute to 365 days: `0`, `59s`, `-1h`, `+1h`, `1e3h`, `1d2h`, `4H`, `1w`, `4h\n`, `1d\n`, `4 h`, `1_0h`, `permanent`, a non-ASCII digit, `9999999h` (overflow), `99999999999999999999d`, `4294967296d`, `366d` and `365d1s` are rejected; `01d` and `.5h` are accepted and real cscli accepts them. The reason is always prefixed with `manual ban`, so it can never start a flag; control characters are stripped and 200 characters is the limit (a 205-character string with NUL, newline and RLO characters is refused after stripping).
2. Whole-internet and own-address bans (design question, behavior not changed). `0.0.0.0/0`, `::/0`, `0.0.0.0`, `::`, `127.0.0.1`, `::1` and `255.255.255.255` are all valid and accepted. Live: `POST /ban {"ip":"0.0.0.0/0","duration":"1m"}` answered 200 and the decision `Range 0.0.0.0/0` was listed (removed again with the unban route). Whether the Caddy bouncer would then block the admin UI itself was not tested (observe-only, no enforcement path in the throwaway). A guard is warranted: reject prefix lengths shorter than a floor (for example /8 for IPv4 and /16 for IPv6, or at minimum `/0`), reject the unspecified and loopback addresses, and refuse (or require a confirmation flag) when the range or address contains the requesting client (`c.ClientIP()`). Recommend a follow-up issue; not a vulnerability because the caller is an authenticated admin.
3. Catch-all unban route `DELETE /admin/crowdsec/ban/*value`. The handler removes one leading slash and validates the rest; the live hostile paths above all answer 400 and change nothing. Double encoding stays encoded (`%252F` becomes `%2F`, which is not an address). `..` is rejected by the address parser; `%2e%2e` is normalized by the router and gives 404. There is no sibling `DELETE /ban/...` route, and `POST /ban` is a different method, so the route does not conflict. No path reaches the filesystem.
4. Deleted-count parsing. `cscliDeletedCount` is unanchored and the first match wins (a probe with `"99 decision(s) deleted"` before `"0 decision(s) deleted"` reads 99), but the only text in the delete output that Charon influences is the validated address; the reason is not part of the delete command and the ban output is not parsed. A 20-digit count fails `Atoi` and falls back to success. Not spoofable from request input.
5. Audit and logs. The security-decision row is written after the cscli success with the validated target and the cleaned reason; a write failure is logged, no longer ignored. The unban writes no audit row (the spec did not ask for one; see I3). Logs carry the validated value plus the first cscli line passed through `util.SanitizeForLog` and cut at 200 characters.
6. `runCSCLI` and deadlines. Every cscli call in `crowdsec_handler.go` (lapi status, decisions, config show, capi status), `crowdsec_dashboard.go` (alerts) goes through the helper; the helper adds the config prefix, applies `context.WithTimeout` and marks a deadline so `errors.Is(err, context.DeadlineExceeded)` holds whether the process was killed or reported the context error. `grep CmdExec.Execute` shows the remaining direct callers: bouncer list, delete and add (5, 5, 10 s deadlines), hub update and install (own limits), and two calls in `RegisterBouncer` (M1). The 504 mapping exists for ban and unban; the listings answer a soft message.
7. `lapiKey()`. Saved file first (`readKeyFromFile`, trimmed; an unreadable or empty file falls through), then `CHARON_SECURITY_CROWDSEC_API_KEY`, `CROWDSEC_API_KEY`, `CROWDSEC_BOUNCER_API_KEY`, `CERBERUS_...`, `CPM_...` (the runtime order, as the spec asks). The key is only set as the `X-Api-Key` header; no log line includes it (grep of the container log and data logs found neither key). `getLAPIKey` and `fetchLAPIAlerts` are deleted and have no references. The alerts typed view decodes only the fields it uses, so `machine_id` and the raw source and decisions never reach the response; the list is bounded by the requested limit (the handler applies its limit validation before the call) and the 32 MiB parser limit.
8. Over-cap sentinel. `ErrCSCLIOutputTooLarge` is returned for output above the parser limit; `ListDecisions` maps it to the same soft message as the executor limit through `respondDecisionsUnavailable`. The alerts path logs and returns an empty list (see L4).
9. Console JSON branch. `isEnrolled` trusts `console.enrolled` when it is a JSON boolean. Probes: `{"console":{"enrolled":false}}` false; `true` true; `{"console":{}}` false; a text prefix before the document is skipped; `[]`, empty and `null` false; two documents use the first. Hostile or partial JSON where the field is `null` or a string (`{"console":{"enrolled":null,...}}`, `{"console":{"enrolled":"yes"}}`) falls back to the text matcher and is judged enrolled because the text contains `enrolled` and `console` (see L2). Real cscli always prints a boolean; the input is local cscli output, not request input.
10. Frontend. Server codes map to fixed translated strings; the fallback is `err.message` (Axios text), rendered as React text, never as HTML. Unban encodes each path component. The five locales hold the same keys (the parity test is part of the passing frontend run). Ban errors are shown beside the field with `role="alert"`.
11. Docs. `docs/features/crowdsec.md` and `docs/troubleshooting/crowdsec.md` match the behavior verified live (ranges, 1 minute to 365 days, 200 characters, `decision_not_found`, `cscli_timeout`, no `1w`/`4H`); they do not mention that the dialog's "Permanent" option is rejected (L1).

## Findings

No critical or high findings.

### Medium

- **M1. `RegisterBouncer` still runs without a deadline and outside the helper.** `backend/internal/api/handlers/crowdsec_handler.go` `RegisterBouncer` (`POST /admin/crowdsec/bouncer/register`, lines about 2190 and 2210): `h.CmdExec.Execute(ctx, "bash", scriptPath)` and `h.CmdExec.Execute(ctx, "cscli", "bouncers", "list", "-o", "json")` use only the request context, and the cscli call has no `-c` config prefix. With the local API down cscli retries for minutes, which holds the request (the same class as the issue this PR fixes for the listings). Not introduced by this PR, but the spec's acceptance line "every cscli call in the handlers runs under a deadline" is not met for this endpoint. Remediation: run the check through `h.runCSCLI(ctx, cscliListTimeout, "bouncers", "list", "-o", "json")`, wrap the script in `context.WithTimeout` (for example 30 s) and map a timeout to `respondCSCLITimeout`. Suggest fixing in this PR (small) or filing an issue.

### Low

- **L1. The ban dialog offers "Permanent" and the server always refuses it.** `frontend/src/pages/CrowdSecConfig.tsx` has `<option value="permanent">`; the value is rejected with 400 `invalid_duration` ("Choose a duration between 1 minute and 365 days"). Real cscli also rejects it (`time: invalid duration "permanent"`), so before this PR it was a 500 "failed to ban IP"; it never worked. Remediation: remove the option or send `365d`/a documented long duration; add a test.
- **L2. Text fallback in `isEnrolled` can answer enrolled for JSON that lacks a boolean.** See security review item 9. Remediation: when the output decoded as a JSON document, do not fall back to the text matcher (treat a missing or non-boolean `enrolled` as not enrolled), and keep the fallback only for non-JSON text.
- **L3. IPv4-mapped IPv6 is stored as typed.** `::ffff:192.0.2.1` is banned as an `Ip` decision with the IPv6 value, which may not match real IPv4 clients. Remediation: apply `addr.Unmap()` (and unmap prefixes in the IPv4-mapped range) before building the argument; the same for unban so that both forms find the decision.
- **L4. Alerts timeout is invisible.** When the alerts command times out or fails, `ListAlerts` answers 200 with an empty list (30.01 s measured with the LAPI down), the same body as "no alerts". Remediation: add an `error`/`unavailable` field (as the decisions list does) so the UI can show the soft message.

### Informational

- **I1.** `0.0.0.0/0`, `::/0`, loopback and the unspecified addresses can be banned (see security review item 2); design decision for the orchestrator.
- **I2.** Unbanning a covering range does not remove contained single addresses (404 `decision_not_found`); this is cscli's exact-scope behavior and the message is accurate.
- **I3.** The unban writes no audit row, while the ban writes one; the spec only asks for the ban row.
- **I4.** The request body of `POST /ban` has no explicit size limit before binding (pre-existing, global middleware applies).

Recommended follow-up issues: M1 (if not fixed in the PR), L1 to L4, and the whole-internet guard (I1).

## Files changed by this review (uncommitted)

- `docs/reports/qa_report.md` (this report)

No test or source file was changed. Temporary probe files (`/var/tmp/qa_*`, the two `zz_probe_test.go` files, `$HOME/qa_trivy_tmp`) were deleted; `codeql-db-*` and `codeql-results-*.sarif` are git-ignored.
