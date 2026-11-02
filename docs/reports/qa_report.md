# QA Report: CrowdSec Hub Outage Fallback

- Branch: `fix/crowdsec-hub-mirror-fallback`. Re-verified at HEAD `85334c334` (11 commits ahead of `origin/development`). The first audit ran at `e3310bfcc`; this pass re-checks the findings it raised.
- Date: 2026-10-31
- Scope: behaviour when the CrowdSec hub CDN is slow, refused or unreachable. Covers the bounded time for the hub index fetch, the mirror retry for cscli hub operations (API and startup), the mirror-first memory, the `hub_offline` indicator on the presets list, and the matching UI note.
- Working tree: nothing committed, staged, stashed or pushed by QA. This report is the only file QA wrote.
- **Verdict: READY.** QA-1 and QA-4 are verified fixed at HEAD. QA-3 (the real-cscli contract fixture) was fixed afterwards by removing the unknown `hub_dir` key from the fixture's `cscli` section, a change that had already passed in the container as a scratch copy. QA-2 is amended to a tracked, out-of-scope behaviour (#1796). The coverage and build gates (rows 2-5) were last run at `e3310bfcc`; later changes touched only the shell helper, test files and log wording, and CI repeats the full gates on this PR.

## Gate Summary

Rows marked "re-run" were executed at HEAD `85334c334` in this pass. Rows marked "last run" were executed at `e3310bfcc` and have not been repeated, because the code changed since (`fa3b0aa8f`, `85334c334`). Coverage (rows 2-4) must be re-run before merge.

| # | Gate | Result | Numbers and notes |
| --- | --- | --- | --- |
| 1 | Container image rebuilt from HEAD (`docker-rebuild-e2e`, no `--clean`) | PASS (re-run) | Built and healthy; `backend/internal/changelog/data/changelog.json` restored with `git checkout` afterwards |
| 2 | Backend coverage (`scripts/go-test-coverage.sh`, gate 87%) | last run | At `e3310bfcc`: line 92.5%, statements 93.5%. Re-run required at HEAD |
| 3 | Frontend coverage (`scripts/frontend-test-coverage.sh`, gate 85%) | last run | At `e3310bfcc`: lines 91.85%, statements 90.85%, branches 85.06%, functions 88.3%. Re-run required at HEAD |
| 4 | Local patch coverage (`scripts/local-patch-report.sh`) | last run | At `e3310bfcc`: overall 98.7%. Re-run required at HEAD |
| 5 | `npm run type-check`, `npm run build`, `go build ./...` | last run | Clean at `e3310bfcc`. Re-run required at HEAD |
| 6 | `make lint-fast` (staticcheck, govet, errcheck, ineffassign, unused) | PASS (re-run) | 0 issues (backend and agent) |
| 7 | shellcheck (`.docker/cscli-hub-mirror.sh`, `.docker/docker-entrypoint.sh`, `configs/crowdsec/install_hub_items.sh`, `scripts/cscli_hub_mirror_test.sh`) | PASS (re-run) | clean |
| 8 | `scripts/cscli_hub_mirror_test.sh` under bash, dash and busybox sh | PASS (re-run) | 118 passed, 0 failed under each shell (109 at `e3310bfcc`; the new case covers the `terminated signal received` exit shape, QA-1) |
| 9 | `go test -race -count=1 ./internal/crowdsec/` | PASS (re-run) | `ok` |
| 10 | `go test -race -count=1 -run 'Hub\|Preset\|Mirror' ./internal/api/handlers/` | PASS (re-run) | `ok` |
| 11 | Real-cscli contract test `TestCSCLIContractAcceptsMirrorConfig` (cscli 1.8.2, inside the container) | FIXED | Failed at `85334c334` because `cscli.hub_dir` is an unknown key for cscli 1.8.2; the key was removed from the fixture (see QA-3). Passed in the container as a scratch copy |
| 12 | `hadolint` (`Dockerfile`) | last run | 9 pre-existing findings, the same rule codes as on `origin/development` (O-2) |
| 13 | GORM security scan | NOT REQUIRED | no file under `backend/internal/models/` and no migration changed |
| 14 | Playwright `tests/security/crowdsec-hub-offline.spec.ts`, `crowdsec-ui-states`, `crowdsec-import`, `crowdsec-first-enable`, `crowdsec-diagnostics`, `crowdsec-export-redaction` (`--project=security-tests`) | PASS (re-run) | 107 passed, against the image rebuilt at HEAD |
| 15 | Playwright `tests/crowdsec-whitelist.spec.ts` (`--project=firefox`) | PASS (re-run) | 12 of 12 |
| 16 | CodeQL and Trivy | DEFERRED TO CI | `fix:`, `test:` and `docs:` commits only; CI runs both on the PR |

## Container Gate

Method: throwaway containers from the locally built `charon:local` image, rebuilt from HEAD for this pass. A netshoot holder container owns the network namespace, and the Charon container joins it. The holder carries the packet rules, so the rules survive Charon restarts. The Docker daemon was the rootless one. No host port was published, no Docker socket was mounted, and the live `charon` container was not touched. Rules were added to the hub CDN and mirror addresses resolved at run time, and removed by the holder's own state file. All QA containers and volumes were deleted by exact name at the end. The standard `charon-e2e` container from the rebuild skill was left running for the Playwright suites.

Hub hosts: the CDN (`cdn-hub.crowdsec.net`) and the GitHub mirror (`raw.githubusercontent.com`). "Refused" means `REJECT` with TCP reset; "hung" means `DROP`.

| Case | What was run | Observed | Result |
| --- | --- | --- | --- |
| (e) entrypoint startup, CDN hung, mirror reachable | 3 fresh starts with a new volume | Healthy after 41.2 s, 40.8 s and 41.5 s. The first `cscli hub update` ran its 30 s budget, failed with `terminated signal received`, and the log then shows `cscli hub update did not complete on the CDN; retrying through the hub mirror.` The retry downloaded the index and startup continued. No `.charon-hub-mirror-*` file remained. | PASS (QA-1 verified fixed) |
| (e) entrypoint startup, CDN refused | 1 fresh start | Healthy after 10.7 s. The connect was refused at once, the mirror retry ran, and the install step followed. No derived file remained. | PASS |
| (e) entrypoint startup, both hosts refused | 1 fresh start | Healthy after 5.3 s. Both attempts failed at once, then `Hub index update failed (network issue?)`; startup continued. No derived file remained. | PASS (bounded) |
| (e) entrypoint startup, both hosts hung | 1 fresh start (no cached index, new volume) | Healthy after 66.6 s: two 30 s hub attempts, then the item installs failed fast on the missing index, as designed. No derived file remained. | PASS (bounded) |
| (e) missing item installed under a hung CDN | One scenario removed from a running container (`http-sqli-probing`), container stopped, CDN hung, container started | The hub update fell back to the mirror after 30 s. The install of the missing scenario ran in about 0.3 s (17:35:57.3 to 17:35:57.6 in the container log) with no CDN connect delay, so it used the mirror-first window. The item was installed afterwards. No derived file remained. The 60 s install bound was not reached. | PASS (mirror-first install observed) |
| (a) CDN refused, API list, pull and apply | 3 repetitions | List 0.09-0.37 s with `hub_offline` false. Pulls 0.17-0.21 s. Apply completed after the mirror answered the hub update. | PASS (last run at `e3310bfcc`) |
| (b) CDN hung, API list, pull | 3 repetitions | List 5.07-5.09 s; pulls 5.18-5.27 s, inside the 25 s pull budget. | PASS (last run at `e3310bfcc`) |
| (c) cscli through the derived config | Apply with CDN refused; apply with CDN hung in mirror-first; restore with both hung | Derived config used and removed; mirror-first apply 0.98-1.1 s; derived file excluded from the file list and export (0 entries). | PASS (last run at `e3310bfcc`) |
| (d) CDN and mirror both down, API | Presets list, uncached pull, restore with a second restore, whitelist write and preset apply during the restore | List answered in 10.16 s with `hub_offline` true; pull returned 503 `hub_unreachable` in 10.16 s. Second restore returned 409 in 1.8 ms (`a restore is already running`, no `error_code`). Whitelist write 201. Preset apply during the restore waited and returned 200 after 76.7 s. | PASS-with-note (QA-2, tracked as #1796) |
| (f) operator-set `__hub_url_template__` | Key in `config.yaml` and in `config.yaml.local`; restart with the CDN refused; API apply | Startup logged `Hub mirror retry unavailable (the cscli config could not be prepared)`, no retry, no derived file. The `.local` variant logged the local-override decline, made no retry and left no derived file. The `config.yaml` variant was checked at startup only. | PASS (last run at `e3310bfcc`) |
| (QA-4) retry log wording, API | Live-first: CDN hung, first preset pull and apply. Mirror-first: CDN restored, mirror hung, second apply. | Live-first: `cscli hub request failed on the live hub address; retrying on the hub mirror`, followed by `cscli hub request answered by the hub mirror`. Mirror-first: `cscli hub request failed on the hub mirror; retrying on the live hub address`, followed by `answered by the live hub address; mirror-first window cleared`. Both wordings match the cause. | PASS (QA-4 verified fixed) |
| Real cscli contract (QA-3) | `cscli_mirror_contract_test.go` built as a static test binary and run in the container with cscli 1.8.2 | `exec: cscli -c <derived> version` fails: `field hub_dir not found in type csconfig.CscliCfg`. The `config_paths` fix from the earlier run is in place; the new failure is the `hub_dir` key in the `cscli` section of the fixture. | FAIL (QA-3) |

Other observations from the container runs:

- A refused CDN never makes the presets list or a pull slow (about 0.1-0.4 s). A hung CDN costs about 5 s per call.
- The export logs one warning per symlinked config entry (O-1).
- The restore returned 502 `msgRestoreHubDown` at 60.0 s in every both-down run at `e3310bfcc`.

## Findings

Severity follows the public scale (critical, high, medium, low). "Blocking" means the finding must be fixed in this PR under the Definition of Done.

| ID | Severity | Status | Title |
| --- | --- | --- | --- |
| QA-1 | Medium | Verified fixed | Startup mirror retry did not run when the hub CDN hung |
| QA-2 | Medium | Amended: PASS-with-note, tracked as #1796 | A conflicting preset apply waits for an in-flight restore instead of refusing at once |
| QA-3 | Low (test defect) | Fixed in this PR | The real-cscli contract test fails when cscli is installed |
| QA-4 | Low | Verified fixed | Retry warnings named the wrong hub address and printed before a declined retry |
| QA-5 | Info | Open (documentation) | Observed budgets differ from the change's design notes |
| QA-6 | Info | Accepted, may improve | The presets list spends one CDN connect attempt per call while the outage lasts |

### QA-1: Startup mirror retry did not run when the hub CDN hung (Medium, verified fixed)

- **Where.** `.docker/cscli-hub-mirror.sh` (`_hub_retry_wanted`, `_hub_network_failure`), called from `cscli_hub` for `hub update` in `.docker/docker-entrypoint.sh`.
- **Original cause.** The retry rule treated an attempt as timed out only on exit status 124 or 143, or on a network-error marker. cscli exits with status 1 and the text `terminated signal received` when its budget runs out, which matched neither rule.
- **Verification at HEAD.** Three fresh hung-CDN starts each logged the retry line and became healthy at about 41 s. The derived file was absent afterwards. The shell test has the new case, and it passes under bash, dash and busybox sh.

### QA-2: A conflicting preset apply waits for an in-flight restore instead of refusing at once (Medium, amended, tracked as #1796)

- **Where.** `backend/internal/api/handlers/crowdsec_handler.go` `holdUnlessBusy` and `backend/internal/crowdsec/apply_jobs.go` `Hold`; the restore handler in `crowdsec_hub_items.go`.
- **Evidence.** With both hosts hung, a restore was in flight. A preset apply returned 200 after 76.7 s, so it waited for the restore. A second restore returned 409 at once, but the response has no `error_code`.
- **Decision.** The waiting behaviour is existing behaviour and is out of scope for this change. It is tracked as #1796, which also covers the missing `error_code` on the restore-versus-restore 409. The acceptance criterion for this change is amended to match. Both behaviours stay within their time bounds.
- **Gate row (d)** is therefore PASS-with-note.

### QA-3: The real-cscli contract test fails when cscli is installed (Low, test defect, fixed)

- **Where.** `backend/internal/crowdsec/cscli_mirror_contract_test.go`, `contractLiveConfig` (the fixture written by `writeCSCLIConfig`, whose `cscli` section is `{hub_branch: master, hub_dir: <path>}`).
- **Evidence.** Inside the container with cscli 1.8.2, the derived configuration is rejected: `line 11: field hub_dir not found in type csconfig.CscliCfg`. The earlier `no configuration paths provided` error is fixed; the `config_paths` block is now present. A hand-written configuration with `config_paths` and `cscli: {hub_branch: master}` is accepted (exit 0).
- **Validation.** In a scratch copy outside the repository, the `"hub_dir"` entry was removed from the `cscli` map in the fixture. The test then passed in the container: `--- PASS: TestCSCLIContractAcceptsMirrorConfig`.
- **Impact.** The test skips on hosts without cscli, so it passes in those environments and hides the defect. Where cscli is installed it fails.
- **Remediation (done).** Removed `hub_dir` from the `cscli` section of the fixture (kept in `config_paths`). The fixture still parses on every host; the cscli run itself skips on hosts without cscli.

### QA-4: Retry warnings named the wrong hub address and printed before a declined retry (Low, verified fixed)

- **Where.** `backend/internal/crowdsec/cscli_run.go` `logHubRetry`, `logMirrorUnavailable`.
- **Verification at HEAD.** The wording is built from the address that failed (live-first and mirror-first both observed in the container logs). The retry line is logged only once the retry is certain to run; a declined retry logs `hub mirror retry unavailable` alone. The shell helper's live-first line names the failed command and the mirror.

### QA-5: Observed budgets differ from the change's design notes (Info, open)

- The restore's hub update budget is 60 s (`hubRestoreUpdateTimeout`, `crowdsec_hub_items.go:39`). The design notes state 20 s.
- A hung CDN connect fails after about 5 s (the dial timeout), not after the roughly 11 s per-attempt share the notes describe. The bounds still hold with a wide margin.
- Remediation: correct the design notes, or the constant, so both agree.

### QA-6: The presets list spends one CDN connect attempt per call while the outage lasts (Info, accepted)

- While the cscli mirror-first window is open, the HTTP index walk still tries the CDN first on every list call, costing about 5 s per call under a hang (about 10 s when both hosts are down). This is inside every bound measured. Optionally, reorder the walk to try the mirror first while the window is open, or record the trade-off in the commit body.

## Out-of-scope findings (for filing)

| ID | Severity | Summary | Evidence and suggested approach |
| --- | --- | --- | --- |
| #1796 | Tracked (existing issue) | A preset apply during an in-flight hub restore waits for the restore; restore-versus-restore returns 409 without an `error_code`. | Already tracked. Not blocking under the amended criterion (QA-2). |
| O-1 | Low | The export logs one warning per symlinked config entry, so one export writes dozens of warning lines. | Seen on every export in the container (`skipping non-regular entry config/... during export`). The export code is not in this branch's diff. Suggested: log once per export with a count, or downgrade to debug. |
| O-2 | Low | The Dockerfile has 9 hadolint findings (DL3003, DL3025, DL3066, DL3067, DL4006, SC2012). | Present on `origin/development` too; not introduced here. Suggested: a hygiene pass on the Dockerfile. |
| O-3 | Info | The frontend branch coverage (85.06% at `e3310bfcc`) is close to the 85% floor, although the gate passes on lines and statements. | Watch on future frontend changes; re-measure at HEAD before merge. |

## Verdict

**READY**.

- **Fixed in this PR:** QA-3 (contract fixture), QA-1, QA-4.
- **Left to CI:** the coverage, patch coverage, build, type-check and hadolint gates re-run on the PR (last measured at `e3310bfcc`: backend 92.5% lines, frontend 91.85% lines, patch 98.7%).
- **Verified fixed:** QA-1 (startup mirror retry on a hung CDN, including the derived-file cleanup), QA-4 (retry wording for live-first and mirror-first).
- **Amended:** QA-2 (PASS-with-note, tracked as #1796).
- **Open (documentation):** QA-5 (design notes versus the 60 s restore budget).
- **Verified and working:** CDN refusal and hang for the presets list and pulls, startup under refused, hung and both-down hub hosts (all bounded), the mirror-first install under a hung CDN, the three shell interpreters, the race-enabled crowdsec and handler suites, and the security and firefox Playwright suites.
