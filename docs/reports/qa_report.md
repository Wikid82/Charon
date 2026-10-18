# QA Report: feat/crowdsec-background-hub-install (PR 6, #1615)

Date: 2026-10-18
Scope: 12 commits (`git log --oneline development..HEAD`): background CrowdSec hub preset install jobs, `crowdsec_apply_jobs` model, job endpoints, restart recovery, busy guard on data-changing handlers, progress UI, docs.
Verdict: **Not ready to merge until the Major finding M1 (stale E2E spec, 5 failures) is fixed.** Everything else passes; the real-cscli gate found no blocking product defect.

## Summary

| Area | Result |
|---|---|
| `go vet ./...` | PASS |
| golangci-lint (`backend/.golangci.yml`, incl. gosec) | PASS, 0 issues |
| `scripts/pre-commit-hooks/golangci-lint-fast.sh` (backend + agent) | PASS, 0 issues |
| `go test -race -count=1` crowdsec, api, services, models | PASS (no failures) |
| GORM scan `--check` | PASS: 0 CRITICAL, 0 HIGH, 0 MEDIUM, 2 INFO |
| Backend coverage script | PASS 91.1% lines (gate 87%) |
| Frontend full vitest (307 files, 3838 tests, includes i18n key test) | PASS |
| Frontend coverage | PASS lines 91.79% (gate 87%) |
| `npm run type-check` | PASS |
| ESLint on touched TS/TSX | PASS: 0 errors, 19 warnings (jsx-a11y / testing-library naming, in existing and new files) |
| markdownlint (ARCHITECTURE.md, docs/features/crowdsec.md, docs/troubleshooting/crowdsec.md) | PASS |
| `scripts/local-patch-report.sh` | PASS: overall 92.9%, backend 91.9%, frontend 99.2% (artifacts written) |
| CodeQL Go (CLI 2.27.2 already current) | 0 results in changed files; 4 results in untouched files (see m3) |
| CodeQL JS | PASS, 0 results |
| Trivy fs (vuln+secret, HIGH/CRITICAL) | PASS, nothing found |
| Playwright `crowdsec-background-install` + `crowdsec-ui-states` (security-tests) | **FAIL: 22 passed, 5 failed (M1)** |
| Real-cscli gate | PASS for all 8 items with caveats noted below |

Uncovered changed lines per patch report (all minor): `models/crowdsec_apply_job.go` 67, 71-74; `handlers/testdb.go` 146; `handlers/crowdsec_files.go` 385-386; `handlers/crowdsec_hub_items.go` 143-144.

## Findings

### Blocking

None in product code.

### Major

**M1. Five E2E specs fail against the final UI (stale selector after commit 63ff60a6).**
`tests/security/crowdsec-background-install.spec.ts` lines 228, 289, 303, 343, 368 (assertions at 244, 299, 338, 364, 386).
Evidence: `progressRow()` is `getByRole('status', { name: /installing preset/i })`. Since 63ff60a6 `PresetInstallProgress` keeps one live region mounted for the whole job, with the same `aria-label` ("Installing preset"), and swaps its content to the result text once the job ends. The spec still asserts `toBeHidden()` / `toHaveCount(0)` on that locator after a terminal state, so it fails (error-context shows `status "Installing preset"` present beside the result text). The app behaviour is the intended accessibility fix; the spec was not updated.
Remediation: after a terminal state, assert on `data-testid="preset-install-progress"` (running variant only; the terminal variant uses `preset-install-result`) or on the absence of the Cancel button, instead of the status role. Ship as `test:` commit on this branch. CI will otherwise fail. (The changelog.json side effect of the e2e rebuild was reverted with `git checkout`.)

**M2. `docker stop` never delivers SIGTERM to Charon, so graceful drain/rollback cannot happen (pre-existing, but it defeats the PR's graceful-stop path).**
Item 7 evidence: `docker stop` mid-install returned in 0.64 s with no "Received signal" or rollback log; the job row stayed `running`/`verifying` and was only repaired on the next boot (recovery restored the snapshot and marked it `failed/interrupted`). Cause: `.docker/docker-entrypoint.sh` starts the app with `run_as_charon "$bin_path" &` (a shell function, so `$!` is the subshell, not `/app/charon`); `shutdown()` sends TERM to that subshell, the entrypoint exits, and PID 1 exit SIGKILLs everything. When SIGTERM is sent straight to the Charon process, the PR behaves well (see item 7). Because of this the 10 s docker grace vs 25 s drain question is moot in practice: the process is killed immediately.
Impact: functionally safe thanks to boot-time recovery, but every container stop mid-install is an unclean kill and shows as "interrupted" instead of "cancelled". Same applies to the uptime drain mentioned in main.go.
Remediation: use `exec`-style launch or track the real PID (for example `pgrep -x charon`/`gosu` exec) and forward TERM to it; `wait` it with `--time` aligned to the drain window; set `stop_grace_period` (compose) above the drain. Out of this PR's scope per triage rules; file a GitHub issue (not security-sensitive) and schedule after this PR.

### Minor

**m1. Progress counters barely move.** Item 1: during a 437 s install `items_installed` stayed 0 and `items_total` stayed null until the final value of 3 at `succeeded/reload`; the only visible progress was elapsed time and the phase (`install` then `reload`). The UI's "N items so far" never shows during a real large install. Suggest counting installed hub items from the filesystem or per-item cscli calls, or hide the claim in docs.
**m2. Async stale digest is reported as a failed job, not an HTTP 409.** Item 6: with `async:true` and a wrong `item_sha256` the API returns 202 and the job fails immediately with `preset_cache_stale` (history row written). The synchronous path correctly returns 409 `preset_cache_stale`. Behaviour is consistent but differs from the expected "409"; confirm it is intentional and documented. Side effect: the one-off bad request created two history rows (async and sync) as `failed`.
**m3. CodeQL Go reports 4 results in files not touched by this PR** (`go/cookie-secure-not-set` in `auth_handler.go`; `go/log-injection` in `remote_server_handler.go` x2 and `uptime_service.go`). Pre-existing; none in the changed files. Not filed as new issues here.
**m4. Console enrollment busy-guard not exercised live.** Enrollment endpoints return 404 "console enrollment disabled" in the throwaway container (feature flag off), so only the unit tests cover the 409 there.
**m5. Backup directories are retained after cancel/timeout/recovery** (`/app/data/crowdsec.backup.*`); expected (history references them) but confirm the retention cap covers these.

## Real-process gate (throwaway container `qa1615-gate` from `charon:local` rebuilt from this branch, cscli v1.8.2-rc1, own volume)

| # | Item | Result | Evidence |
|---|---|---|---|
| 1 | Large background install | PASS (with m1) | `sigmahq/windows_proc_creation`: pull then apply `{async:true}` returned 202 + job id; phases `install` then `reload`; `jobs/current` resumed across polls; final `succeeded`, 437 s, `items_installed` 3; `crowdsec_preset_events` row `applied` with `backup_path`; collection and about 520 scenario/parser/context entries installed (cscli lists diff shows it). Note: the same install took only 6 s on later runs, so duration is network dependent. |
| 2 | Cancel mid-install | PASS | cscli frozen with SIGSTOP in the verify phase, DELETE job -> `cancelled`/`rollback`; md5 of the config tree names and of collections/parsers/scenarios/contexts/hub contents identical to pre-install; `cscli collections/parsers/scenarios list` diff empty; engine running. |
| 3 | kill -9 mid-install + restart | PASS | Job `failed`/`interrupted`; log shows "restored the configuration snapshot of an interrupted crowdsec install" before "CrowdSec reconciliation: starting CrowdSec"; tree and cscli lists identical to pre-install; engine healthy (LAPI ready). |
| 3b | Empty snapshot dir / junk snapshot dir | PASS | Job `failed/interrupted` with warning "the snapshot of the interrupted install is incomplete, so nothing was restored"; config tree unchanged from the mid-install state (1200 / 897 files), not wiped; engine started healthy. |
| 4 | Busy guard | PASS (m4) | While a job ran: second async apply, sync apply, start, stop, import, file write, whitelist add, hub-items/restore all 409 `crowdsec_busy` with `job_id`; start and stop returned 409 without touching the engine (pid unchanged). GET status, files, acquisition, jobs, jobs/current, export (200) stayed open; acquisition PUT 200; ban and bouncer register 200 (not hub-modifying). Console enroll 404 (feature off). |
| 5 | Lowered job limit / sync path | PARTIAL | Job limit (`ApplyJobMaxDuration` = 30 min) is not configurable by env or setting, so the `timed_out` async path was SKIPPED (covered by unit tests only). Sync apply: `HUB_APPLY_TIMEOUT_SECONDS=2` -> 504 `preset_apply_timeout` after 2.16 s with tree restored (md5 identical); small preset `crowdsecurity/iptables` sync apply without the override -> 200 `applied` in 1.4 s, collection installed. |
| 6 | item_sha256 mismatch | PASS (m2) | Sync: 409 `preset_cache_stale`. Async: 202 then job `failed` `preset_cache_stale`. |
| 7 | Graceful stop mid-install | PASS via direct SIGTERM, FAIL via `docker stop` (M2) | Direct `kill -TERM` of the Charon process: "Received signal terminated", rollback ("preset apply failed; restored previous configuration"), job `cancelled`, server stopped in about 1 s, tree identical to pre-install. `docker stop`: SIGTERM never reaches Charon (0.64 s, no drain, job repaired by boot recovery as `interrupted`, tree restored). |
| 8 | First boot after upgrade | PASS (simulated) | No previous-version image was available locally, so the table was dropped from an existing DB (`DROP TABLE crowdsec_apply_jobs`) and the container restarted: table recreated by AutoMigrate, no recovery or startup error in logs, `jobs/current` returns `{"job":null}`, engine running. A true old-image to new-image upgrade was not run. |

## Cleanup and environment notes

- Throwaway container `qa1615-gate` and volume `qa1615-data` deleted; no other qa1615 resources remain. The live `charon` container, its data, compose and volumes were not touched (still running healthy).
- `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e` rebuilt and retagged `charon:local` and (re)created the standard `charon-e2e` container, as the skill does; the running live `charon` container keeps its older image.
- `backend/internal/changelog/data/changelog.json` restored with `git checkout`. Nothing staged, committed, pushed or stashed. Git-ignored artifacts from the scans remain (`codeql-db-*`, `codeql-results-*.sarif`, `test-results/`, `frontend/coverage/`).
- Required follow-ups: fix M1 on this branch; file an issue for M2 (and optionally m1, m2).
