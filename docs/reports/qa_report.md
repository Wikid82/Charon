# QA and Security Report: fix/crowdsec-cscli-contract

Date: 2026-10-17
Scope: full diff `origin/development...HEAD` (7 commits, HEAD `0b35d700`) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved spec (PR 3). The branch shows the decisions the real `cscli` prints (alert-shaped output), checks the CrowdSec configuration with `cscli -c <config> config show` instead of the nonexistent `config check`, reads every cscli listing through one JSON reader that skips progress lines, bounds the output read from any cscli command, removes the unused bouncer registration code, and records real cscli output as fixtures with a contract document.

## Verdict: PASS (no critical or high findings; one medium defense-in-depth finding)

No critical or high findings. One medium, five low and four informational notes are listed; none blocks the merge. The one defect found during the run (the diagnostics E2E test expected "no cscli" although the E2E image bundles cscli) was fixed in the test file, which is left uncommitted for the orchestrator. No model, migration or GORM query file is in the diff, so the GORM scan was not required.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Backend tests and coverage | `scripts/go-test-coverage.sh` (system `tee` crashed the first run with a broken pipe; rerun with a `tee` shim on PATH) | 0 failing tests; statements 92.1%, lines 91.0% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (frontend lcov placeholder created for the run, removed afterwards) | overall 97.4% (147/151), backend 97.4%, frontend and agent 100% (0 changed lines); artifacts `test-results/local-patch-report.{md,json}` produced. Only file below 100%: `cscli_exec_unix.go` 60% (the `killProcessTree` fallbacks: no process, `kill` failure) |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` | 0 issues |
| Strict lint on touched packages | `golangci-lint run --config ../.golangci-fast.yml ./internal/crowdsec/... ./internal/api/handlers/... ./internal/services/...` | 0 issues |
| Build | `cd backend && go build ./...` | pass |
| Race run | `go test -race -timeout 60m ./internal/crowdsec/... ./internal/api/handlers/... ./internal/services/...` | pass (crowdsec 121.8 s, handlers 427.8 s, services 616.9 s, remotestorage 46.0 s) |
| CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CLI 2.27.2, 345 of 345 compiled Go files extracted) | 4 results, all the known baseline entries in files outside the diff (`auth_handler.go:201`, `remote_server_handler.go:148` x2, `uptime_service.go:1551`); 0 new |
| CodeQL JS | not run | the only TypeScript change is one Playwright spec (test file) |
| Trivy | `trivy fs --scanners vuln --severity HIGH,CRITICAL` on `go.mod` and `go.sum` (copied to a home directory because the snap build cannot read the repo path; removed afterwards) | 0 findings |
| Markdown lint | `make lint-markdown` | 0 issues in 266 files |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| Frontend tests, type check, build | not run | no frontend file changed |
| E2E (E2E container rebuilt in 67 s; the `charon` container was not touched) | `npx playwright test tests/security/crowdsec-decisions.spec.ts tests/security/crowdsec-diagnostics.spec.ts --project=security-tests` | first run 40 passed, 1 failed (see F1); after the test fix the diagnostics spec passes 20 of 20 (decisions spec was 21 of 21 in the first run) |

`backend/internal/changelog/data/changelog.json` was restored with `git checkout` after each E2E run; `go.work.sum` is unmodified.

## Real-cscli gate (image built from the working tree, cscli v1.8.2-rc1)

A throwaway container (own name, own volume, port bound to 127.0.0.1) ran the image; the first admin was created through the setup API and CrowdSec was started through the API. All addresses are documentation addresses. The container, volume and image tag were removed afterwards; the `charon` container (up 29 h, healthy) was never touched.

| Step | Observation | Result |
|---|---|---|
| (a) empty list | `{"decisions":[],"total":0}` | pass |
| (a) ban 192.0.2.10 and 192.0.2.11 through `POST /ban` | both `banned`; list shows id, origin `cscli`, type `ban`, scope `Ip`, value, duration `1h59m59s`, scenario `manual ban: qa gate`, created_at, no blank field | pass |
| (a) range through the API | `POST /ban` with `198.51.100.0/24` answers `{"error":"failed to ban IP"}` (the handler builds `cscli decisions add -i <value>`, which accepts addresses only). The range was added with `cscli decisions add -r` inside the throwaway; the list shows scope `Range`, value `198.51.100.0/24`, scenario `qa range`, no blank field | pass for listing; see I2 |
| (a) dashboard summary | `total_decisions` 2, `unique_ips` 2, `top_scenario` `manual` match the two API bans (they are counted from Charon's own table); `active_decisions` -1 | see I3 |
| (a) unban | `DELETE /ban/192.0.2.11` then list: only 192.0.2.10 remains (total 1) | pass |
| (b) `GET /decisions/lapi` | one decision, all fields populated (served through the cscli fallback because the direct local API call answered 403, see I3) | pass |
| (c) default config | `config_valid` true, no errors, `lapi_port` 8085 | pass |
| (c) malformed YAML | `config_valid` false, `config.yaml validation failed: <path>: [2:14] sequence end token ']' not found` (path redacted, first cscli line only) | pass |
| (c) invalid `log_level` | `config_valid` false, `config.yaml validation failed: <path>: not a valid logrus Level: "bogus"` | pass |
| (c) config restored | `config_valid` true | pass |
| (c) cscli binary renamed | `config_valid` false, `config.yaml could not be checked: cscli unavailable or not responding`; the decisions list answers `{"decisions":[],"error":"cscli not available or failed"}` | pass |
| (d) local API stopped | decisions list answered after 30.02 s with HTTP 200 `{"decisions":[],"error":"cscli decisions list timed out; the CrowdSec local API may be stopped"}` | pass |
| (d) client disconnect | request cut at 3 s (curl exit 28); no `cscli` process was left in the container 1 s later | pass |
| (e) alerts, bouncer, key status, bouncer register, lapi health, connectivity, console status | all answered with expected data (console status: `console enrollment disabled` by default; connectivity: `console_status` `not_enrolled`; bouncer register answered with a masked preview) | pass |
| (e) list after restart | `total` 2 (192.0.2.10 and the range), no error | pass |
| (f) secrets | no bouncer key, token or password in the responses or the container log (only masked previews such as `ozpB...Bjhs`); the one `password` match in the log is a route name in the Gin route table | pass |

## Security review

Throwaway probes (Go tests under the package directories, deleted afterwards; `git status` shows only the test fix) were run against the exported functions.

1. Output cap (`cscli_exec*.go`). The cap is 33,554,432 + 1,048,576 = 34,603,008 bytes. A command printing 200 MB stopped after 34,603,008 bytes in 0.85 s, returned `ErrOutputLimit` ("command output exceeded the size limit", no output in the text), and its background child (same process group) was dead. A hit on stderr only (`OutputLimited`) returned the same error in 0.46 s. A command printing exactly the cap succeeded with all bytes; cap + 1 failed with the bytes up to the cap kept; cap - 1 succeeded. A context timeout (0.5 s) ended the command in 0.504 s with `signal: killed` and killed its child. A cancel at 0.3 s ended in 0.301 s. A missing binary returns the `fork/exec ... no such file or directory` error unchanged (callers match on `*exec.Error`). Thirty consecutive 100 MB floods left the goroutine count at its start value (2 before, 2 after), so no pipe or goroutine leak. `go test -race` is clean. The `limitedBuffer` writes are mutex-protected; the kill runs once (`exceeded` latches).
2. Executors. The three production executors (`RealCommandExecutor`, `SecureCommandExecutor.ExecuteWithEnv`, services `simpleCommandExecutor`) all go through `CombinedOutputLimited`. A search of the backend finds no other `CombinedOutput()`/`Output()` on a cscli or crowdsec command; the two remaining are `caddy/importer.go` (caddy adapt, not cscli) and the developer tool `cmd/localpatchreport`. The crowdsec daemon start (`exec.Command(binPath, "-c", ...)`) does not read output.
3. `parseCSCLIDecisions`. Results of the hostile inputs: JSON starting with a log object line (`{"level":...}`) fails closed with the fixed error (the first document that decodes is the log object, which is not an array); a `level=...` text line or `[INFO]` text line before the array is skipped and the array is read; a number beyond int64 in `id` or `1e999` in `duration` fails closed; a duplicate key keeps the last value; invalid UTF-8 in a value is replaced (no error); a row without a value is skipped and counted; `[null, {...}]` skips the null; 20,000 nested arrays and 9,000 levels inside `decisions` fail closed; a second document after the first is ignored; a top-level object fails closed. No panic in any case. The row cap (100,000) and the 32 MiB parser limit hold, but see M1 for memory use before the cap applies.
4. Config check. The config path is `crowdsec.FindConfigFile(DataDir)` checked with `pathWithin`; it is passed as a separate argument (no shell). Messages go through `RedactPaths` (verified live: `<path>`), one line, 200 characters at most. A missing binary or a timeout reports "could not be checked", never "invalid". See L4 for the one remaining echo of cscli text.
5. Deleted registration code (`registration.go` and its test): `EnsureBouncerRegistered`, `BouncerRegistration`, `getBouncerAPIKey`, `hasCSCLI`, `getExistingBouncer`, `registerBouncer` have no references in code (only archived plan documents mention the old names; the names `validateLAPIURL` and `TestEnsureBouncerRegistration_*` that still exist belong to the handler package and compile against its own code).
6. Fixtures. A search for tokens of 32 or more characters, `password`, `secret`, `token` and `api_key` in the new fixture folders finds only the placeholder `FAKEKEY0fakekey1FAKEKEY2fakekey3FAKEKEY4fk` and the 64-character sha256 digests in the older hub fixtures (not secrets). Every IP is `192.0.2.x`, `198.51.100.x` or the loopback `127.0.0.1`. The machine login name `testlocal` is a throwaway name. The recording metadata has no hostnames.
7. Docs (`docs/development/cscli-contract.md`). The command table matches the commands the code runs (`decisions add -i -d -R -t ban`, `decisions delete -i`, `config show`, `capi status`, `console status`, `lapi status`, bouncers and alerts listings); the cap values and the `UnmarshalCSCLIOutput` rule match the code. One inconsistency: see L5.

## Findings

No critical or high findings.

### Medium

- **M1. Memory use of the decisions parser before the row cap applies.** `parseCSCLIDecisions` decodes the whole document (up to the 32 MiB limit) into a slice of entries and only then applies the 100,000-row cap. Measured peak heap: 31 MiB of `{},` rows (11.2 million entries) used 6,153 MiB and took 41 s; 31 MiB of `{"decisions":[{}]},` rows used 1,026 MiB (3.1 s); 31 MiB of `{"value":"a"},` rows used 1,134 MiB (1.9 s). A realistic 31 MiB document of real cscli alert rows (40,032 rows of 812 bytes) used 133 MiB and took 0.5 s. Real cscli always prints every field of each row, so the degenerate shapes cannot be produced by banning addresses through Charon and need a different cscli or local API; I therefore rate it medium (defense in depth) and not high. Remediation: decode the array with `json.Decoder` token by token and stop at the row cap, or reject documents whose element count exceeds a bound before decoding (for example count top-level `{` with a cheap scan); lower `maxCSCLIOutput` for the decisions listing.

### Low

- **L1.** Output between 32 MiB and 33 MiB (the parser limit and the capture cap differ by 1 MiB) is captured but rejected by the parser, so the decisions list answers 500 "failed to parse decisions", while larger output gives the soft "cscli not available or failed" error. Either way the list is not shown; making both cases the same soft error would be consistent.
- **L2.** When more than 100,000 decisions exist, the extra rows are dropped without any log line or flag, and `total` is the number returned. Log the truncation or return a `truncated` flag.
- **L3.** `OutputLimited` is not used by any production code (only by its tests). Delete it or use it; CLAUDE.md asks for dead code to be removed.
- **L4.** The first non-log line of a failing `cscli config show` is echoed to the admin (200 characters, paths redacted). A YAML error can quote a fragment of the value that failed (for example "cannot unmarshal ... into int"). The CrowdSec `config.yaml` holds no credentials by default (they live in `local_api_credentials.yaml`), and the endpoint is admin only, so exposure is minimal; noted for completeness.
- **L5.** `cscli-contract.md` checklist step 5 says to run "hub install" against the real cscli, while the same page says `cscli hub install` does not exist. Change it to "install a hub item with its type command".
- **L6.** After a command that exits normally, a child it left running is not killed (the process group is only killed on the cap, the context or a timeout). With a child holding the pipe the call returns the output after the 2 s `WaitDelay` together with `exec: WaitDelay expired before I/O complete` (probe: output `done`, 2.01 s). cscli does not start children, so this is theoretical.

### Informational

- **I1. F1 (fixed in the working tree, not committed).** `tests/security/crowdsec-diagnostics.spec.ts` assumed the E2E image has no cscli and expected "could not be checked: cscli unavailable". The image bundles cscli v1.8.2-rc1, which rejects the test's minimal `config.yaml` ("no configuration paths provided"), so the real message is `config.yaml validation failed: no configuration paths provided`. The test title, comment and expectation were updated; the unavailable-binary wording is covered by the handler unit tests and was confirmed live in the gate.
- **I2. Pre-existing, not in the diff.** `POST /admin/crowdsec/ban` accepts any non-empty `ip` without validation and builds `decisions add -i <value>`; a range or an invalid address returns the generic 500 "failed to ban IP". Validate with `net.ParseIP`/`netip` and return 400, or support ranges with `-r`. Not a security issue (arguments are passed without a shell and as the value of `-i`).
- **I3. Pre-existing, not in the diff.** The dashboard summary's `active_decisions` is -1 and `GET /decisions/lapi` logs a 403 when the process environment has no CrowdSec API key, because the direct local API calls read the key from the environment and not from the saved `bouncer_key` file that the bouncer pages use. The throwaway container did not set the key variable (the live deployment does).
- **I4.** `GET /admin/crowdsec/alerts` returns the raw cscli alert objects, including `machine_id`. Not a credential, pre-existing, admin only.

Recommended follow-up issues (out of scope for this PR): M1 if the maintainer wants the parser hardened further, L2, I2 and I3.

## Files changed by this review (uncommitted)

- `docs/reports/qa_report.md` (this report)
- `tests/security/crowdsec-diagnostics.spec.ts` (expectation for the config check on the E2E image; see I1)

Temporary files under `/var/tmp/qa3` (logs and a `tee` shim) and the probe test files were removed or are outside the repository; `codeql-db-go` and `codeql-results-go.sarif` are git-ignored.
