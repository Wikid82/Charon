# QA Report: CrowdSec Bouncer Key Robustness (PR 12)

- Branch: `fix/crowdsec-key-robustness`, HEAD `01981fbba` (10 commits ahead of `origin/development` `5381f3f52`). The audit ran first at `6e38d5984`; the branch was then rebased onto a newer `origin/development` during the run (see "Rebase during the run"). The 10 commit patches are identical before and after (`git range-diff`, all `=`).
- Date: 2026-11-02
- Spec: `docs/plans/crowdsec_hardening_batch5_spec.md` section 3t (3t.14 to 3t.16 authoritative), with `SECURITY.md` and `CLAUDE.md`.
- Scope: the three-valued LAPI key check (403 is the only rejection), the held key after a failed save, the single key path (`PathFor`, `CHARON_CROWDSEC_BOUNCER_KEY_PATH`), and log hygiene for key material.
- Working tree: nothing committed, staged, stashed or pushed by QA. This report is the only file QA wrote. Throwaway containers and volumes were removed; `charon-e2e` and the live `charon` container were not touched by the gates.
- **Verdict: BLOCKED on one CI-blocking lint failure.** `make lint-backend` (full golangci-lint v2.15.0 with gosec) exits 2 on six gosec findings in test files this PR adds (finding B1). Every other DoD gate and every container gate (a) to (l) passes. Fix B1 in a `test:` or `fix:` commit inside the PR, then re-run `make lint-backend`.

## Gate Summary

| # | Gate | Result | Numbers and notes |
| --- | --- | --- | --- |
| 1 | `cd backend && go build ./...`, `go vet` on touched packages | PASS | clean |
| 2 | `go test -race -count=1` `./internal/crowdsec/...`, `./internal/config/...`, `./internal/caddy/...`, `./internal/util/...`, `./cmd/api/...` | PASS | crowdsec ok (127 s), bouncerkey ok, config ok, caddy ok, util ok, cmd/api ok |
| 3 | `go test -race -count=1 ./internal/api/handlers/` | PASS | ok, 491 s |
| 4 | `go test -race -count=1 ./internal/services/` | PASS | ok, 607 s |
| 5 | `scripts/go-test-coverage.sh` (gate 87%) | PASS | line coverage 92.0%, statements 93.1% |
| 6 | `bash scripts/local-patch-report.sh` | PASS | overall and backend patch coverage 96.2% (235 changed lines, 226 covered); both artifacts written. 7 uncovered changed lines, see L1 |
| 7 | `npm run type-check` (frontend) | PASS | clean, run on the first tree and again on `01981fbba` |
| 8 | `scripts/frontend-test-coverage.sh` (gate 85%) | PASS | at `01981fbba`: 319 files, 4009 passed, 4 skipped, 2 todo; lines 91.83%, branches 85.05%, functions 88.3%. Branches sit at the floor (3t.8 said to watch it) |
| 9 | `make lint-backend` (full golangci-lint, gosec) | **FAIL** | exit 2, 6 issues, all gosec, all in new test files (B1) |
| 10 | `lefthook run pre-commit` | PASS, but a no-op on a clean tree | with no staged files every hook was skipped. Re-run as `lefthook run pre-commit --all-files`: exit 0 on `01981fbba`, including `golangci-lint-fast`, `go-vet`, `frontend-lint`, `frontend-type-check`, `shellcheck`, `markdownlint`, `actionlint` and `semgrep` (0 findings, 367 rules, 2313 files) |
| 11 | `./scripts/scan-gorm-security.sh --check` | NOT REQUIRED | `git diff --name-only origin/development..HEAD` has zero `backend/internal/models/` paths |
| 12 | Targeted Playwright: `tests/security/crowdsec-key-robustness.spec.ts`, `crowdsec-first-enable.spec.ts`, `crowdsec-import.spec.ts` | PASS, 64 of 64 | ran on `--project=security-tests`, see note N2 |
| 13 | Container gates (a) to (l) | PASS (a to l), see table below | the development control reproduces the bug for (b) and (d) |
| 14 | CodeQL and Trivy | DEFERRED TO CI | `fix:`, `test:`, `docs:` and `refactor:` commits, no new feature surface (CLAUDE.md scope rule) |

### Container gates (a) to (l)

Images: `charon:local` rebuilt from the branch with `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e` (exit 0, healthy). Control: `charon:qa-control` built with `docker build` from `git archive 3c24d1b21` (the development base before the PR). All throwaway containers ran on the rootless daemon with unique names, named volumes, and host port 80/443 unpublished. The live `charon` container was not used.

| Gate | Branch result | Control (development) result | Evidence |
| --- | --- | --- | --- |
| (a) real-LAPI verdict | PASS | n/a | Real engine CrowdSec v1.8.2 inside the container. Bogus key `GET /v1/decisions` returned **403**. No key returned **403** too. A key from the file returned **200**. `kill -STOP` on the engine: `key-status` returned `key_check: unknown` after 5.02 s. `kill -CONT`: `accepted` in 0.02 s. `kill -9` on the engine: `unknown` after 5.03 s, key file hash unchanged, `caddy-bouncer` still listed. |
| (b) unreachable `CHARON_SECURITY_CROWDSEC_API_URL=http://127.0.0.1:9` at boot | PASS | **FAIL** (bug reproduced) | Branch: key hash `0b371f6112570859` before and after; `created_at` unchanged; boot log line 110 "CrowdSec bouncer registration: LAPI is ready" precedes the single Warn at line 111; `cscli lapi status` exit 0 at that time (gate l). Control: hash `0f4ca0249c954c31` became `ef128badbf7eb933`, `created_at` moved, and the log says "successfully registered and saved key" with a `masked_key` field |
| (c) garbage saved key | PASS | not run | Key file replaced by `printf` garbage (`chown charon`, mode 600). Restart with healthy LAPI: hash `4711337f164a535e`, new `created_at`, `key-status` `accepted`, Caddy-carried key returns **200** from LAPI |
| (d) read-only bind-mounted key file holding a rejected key | PASS | **FAIL** (stale key kept) | Branch: `-v <file>:/app/data/crowdsec/bouncer_key:ro`. `key_source: memory`, `key_check: accepted`, message says the key is used until restart. Caddy admin config carries the registered key: LAPI **200**. The error log shows `read-only file system`. Control: `key_source: file`, `valid: false`, Caddy carries the garbage key: LAPI **403** |
| (e) `CHARON_CROWDSEC_BOUNCER_KEY_PATH` | PASS | not run | `alt_bouncer_key` created, the default `bouncer_key` not created, `key_file_path` reports the alt path, Caddy-carried key LAPI **200** |
| (f) log scan for key material | PASS on the branch | control positive | Scanned 6 branch and control log files for `masked_key`, `API Key:`, and the first and last four characters of every key collected from the containers (7 distinct values). Branch: zero matches. Control: `masked_key` x1 and a 4+4 character match in the control boot log (expected for the pre-fix build) |
| (g) existing CrowdSec key specs | PASS | not run | 64 of 64 (see the Playwright row above) |
| (h) Start with a stored unreachable `CrowdSecAPIURL` | PASS | not run | `POST /security/config` set `crowdsec_api_url` to `http://127.0.0.1:9`. Then stop and Start. Response carries `warning`; key hash and bouncer `created_at` unchanged; **0** `bouncers delete`/`add` log lines |
| (i) double Start with the read-only file | PASS | not run | Two stop/start cycles. Bouncer `created_at` unchanged after each; `key_source` stays `memory`; one read-only save error logged |
| (j) restart after a hold | PASS | not run | `docker restart` with the read-only file still in place. The bouncer rotated (`created_at` changed) and the `read-only file system` error was logged. The held key is lost on restart, as the spec's "lost on restart" claim (3t.14 m5 j) states |
| (k) 401 vs 403 probe | PASS (accepted as "403 seen, no 401") | not run | 403 for a bogus key and for a missing key. No 401 seen |
| (l) LAPI wait passed before the unverified warning | PASS | not run | Proof in (b): "LAPI is ready" (line 110) before the Warn (line 111); `cscli lapi status` exit 0 |

## Findings

Severity order. Each finding has evidence and the remediation.

### B1 (High, CI blocker): six gosec findings fail `make lint-backend`

- Evidence: `make lint-backend` on `01981fbba` ends with `6 issues: * gosec: 6` and `make: *** [Makefile:199: lint-backend] Error 1` (exit 2). The findings:
  - `backend/internal/api/handlers/crowdsec_key_check_unknown_test.go:143` G304 (`os.ReadFile(keyPath)`), `:155` G304.
  - `backend/internal/api/handlers/crowdsec_key_held_test.go:66` G302 (`os.Chmod(dataDir, 0o500)`), `:68` G302 (`os.Chmod(dataDir, 0o700)`).
  - `backend/internal/services/crowdsec_startup_held_test.go:35` G302 (`os.Chmod(keyDir, 0o500)`), `:37` G302.
- Why it slipped: `lefthook` `golangci-lint-fast` does not enable gosec, so the local pre-commit hook passes. CI runs `make lint-backend` and will fail.
- Remediation: these are deliberate test fixtures (read-only directories and a fixed test path). Add a per-line `//nolint:gosec // <code>: <reason>` with a reason, the same form as `backend/internal/orthrus/ca.go:84`. Put the change in a `test:` commit in this PR. Re-run `make lint-backend` and expect `0 issues`.

### L1 (Low): uncovered changed lines in the patch

- `backend/internal/services/crowdsec_startup.go:379-380` (the default `http://127.0.0.1:8085` branch when `CHARON_SECURITY_CROWDSEC_API_URL` is unset).
- `backend/internal/crowdsec/bouncerkey/verdict.go:81` (`ctx.Done()` inside the retry wait) and `:96-97` (the `NewRequest` error branch, practically unreachable).
- `backend/internal/crowdsec/bouncerkey/bouncerkey.go:341-344` (`HeldKeyInUseMessage(saved=true)`, the "saved to the key file" text).
- Patch coverage is 96.2%, above the gate. Remediation (optional): one table case each for the default URL, the `ctx.Done` return during a backoff wait, and the `saved=true` message.

### R3 (Low, carried from 3t.16 r3): delete-then-add is not atomic

- Evidence: `backend/internal/api/handlers/crowdsec_handler.go:2021-2024` and `backend/internal/services/crowdsec_startup.go:348-354` run `bouncers delete` and then `bouncers add`. The ctx guard runs before both, so a cancel between the two calls leaves no `caddy-bouncer` until the next Start. The next Start then sees the saved key rejected and registers again. No runtime test covers the window.
- Remediation: low. Record as a low issue (not a regression; the base had the same delete-then-add). Optional later fix: add the new bouncer under a temporary name and swap, or validate after add.

### O1 (Low, code-read, not run): a reachable non-local LAPI can drive a local re-register on each Start

- Evidence: the handler key check uses the stored `CrowdSecAPIURL` (`backend/internal/api/handlers/crowdsec_handler.go:1728-1731`), while registration runs `cscli` against the local engine. If the stored URL is a reachable LAPI that does not know the local `caddy-bouncer` key, its 403 is a definite Rejected, and the local bouncer is deleted and re-added on every Start.
- Status: pre-existing. The development base uses the same stored-URL check (`git show 3c24d1b21:backend/internal/api/handlers/crowdsec_handler.go`, around `:1708`). This PR does not change the behaviour, and I did not run it. Remediation: file as a low issue, scoped to the stored URL and the local registration mismatch.

### N1 (Informational): key exposure through the API

- `GET /admin/crowdsec/key-status` sets `full_key` only in the `auto-generated` branch, as before (`crowdsec_handler.go:1803-1809`). The `memory` branch sets `CurrentKeyPreview` (masked) and no `FullKey` (`:1796-1801`). Good.
- `GET /admin/crowdsec/bouncer/key` (`crowdsec_handler.go:2124-2136`) returns the full key for `source: memory`, the same as for the file and env sources. It sits in the `managementAdmin` route group (`routes.go:1063`, `RegisterRoutes` at `crowdsec_handler.go:2836`), so only admins can call it. Accepted as the existing admin reveal; it now also reveals a held in-memory key. No change requested.

### N2 (Informational): Playwright project

- The firefox project excludes `**/tests/security/**` (`playwright.config.js`, the firefox `testIgnore`). The three specs run only under `security-tests`, which uses a Desktop Chrome device. I ran that single project for them. It is one project, not a second browser. Result: 64 passed, exit 0.

### N3 (Informational, pre-existing, out of scope): Caddy admin on the container interface

- Boot log: `admin endpoint on open interface; host checking disabled`, `address: 0.0.0.0:2019`, in the branch and control logs. The Caddy config carries the bouncer API key. The E2E compose publishes 2019 on loopback only. Not exploitable from the host in the tested setup, and not part of this PR. Security-sensitive: keep any tracking in a private advisory per `SECURITY.md`, not a public issue.

### C1 (Low, commit text): `(security)` subject wording

- Commit `0340e880b` subject: `fix(security): harden log output in the CrowdSec integration`. It meets the CLAUDE.md rule: general category, no vulnerability class, no vector, no code path. It is the most specific of the approved options, and "log output" names the surface. If the orchestrator wants the vaguer reading, use `fix(security): harden the CrowdSec integration` (own commit, not yet pushed).
- The 10 commit bodies are empty. No session ID or session link in commits, code comments, test names or docs. The diff additions contain no `leak`, `exposure`, `fragment`, `secret`, `credential`, `vulnerab`, `CVE`, `GHSA` or `attacker` wording in code or test names. `TestCrowdSecLogsCarryNoKeyText` is neutral.
- Informational: historical records still show `masked_key` lines (`docs/reviews/crowdsec_auth_fix_code_review.md:45,158,161,163`, `docs/plans/bouncer_key_logging_spec.md:63`). They are review and plan records, not user docs, and they describe the old output as an example, not as a flaw. Leave them.

### Review minors from 3t.16: status

| Item | Status | Evidence |
| --- | --- | --- |
| r1 wording (`util/mask.go`, helper names, docs bullet) | RESOLVED | grep for "Never pass its output", "leaked fragment", "do not include any part", `assertNoKeyFragments` returns no hits in backend, frontend, docs or tests. `MaskAPIKey` says "for UI previews and API display fields only" (`backend/internal/util/mask.go:8-10`) |
| r2 TOCTOU in read-side release | RESOLVED | `releaseIf` is used in `Resolve` (`bouncerkey.go:146`) and `Ensure` (`:238`, `:248`), and in `settleNewKey` and `heldKeyValid` (`:303`, `:314`). `release_if_test.go` covers it and runs under `-race` |
| r3 delete-then-add not atomic | OPEN, low | see R3 |
| r4 env-only wording in docs | RESOLVED | `docs/features/crowdsec.md:194` now says "If there is no saved key yet, Charon keeps using the key it already has" |
| n6 stacked doc comments in `frontend/src/api/crowdsec.ts` | RESOLVED | the `CrowdSecKeyCheck` type and the `CrowdSecKeyStatus` doc now sit together |
| n7 `key_source` values comment | RESOLVED | `crowdsec_handler.go:1671` lists `memory` |
| n8 stale plan citations in the E2E spec header | RESOLVED | header lines 1-8 carry no planning-doc references |
| n9 QA report gates | RESOLVED by this report | |

## Security Review

### Destructive paths (can Unknown, an error or a cancel delete or re-register the bouncer?)

- Only two destructive sites exist: `bouncers delete` followed by `bouncers add` in `registerBouncer` (`crowdsec_handler.go:2021-2024`) and in the startup `register` closure (`crowdsec_startup.go:348-354`). Both are reached only through `bouncerkey.Ensure`.
- `Ensure` reaches `register` only after every present candidate (held, saved, env) returned `VerdictRejected` (`bouncerkey.go:285`). `Unknown` returns `OutcomeUnverified` with no register, delete or save (`:243-263`). A cancelled context returns `OutcomeUnverified` before register (`:280-282`).
- `Check` maps only 403 to Rejected. 401, 3xx, 404, 429, 5xx, transport errors, per-attempt timeouts and cancellation all map to Unknown (`verdict.go`). Runtime evidence: gates (a), (b), (h) and (i) show no delete or add on Unknown (LAPI killed, stopped, stalled, or an unreachable stored URL).
- Residual, not destructive to the key: R3 (a cancel between delete and add) and O1 (a stored URL on another LAPI).
- Result: no path found where Unknown, an error or a cancel deletes or re-registers the bouncer, other than the R3 window, which recovers at the next Start.

### Key exposure

- API: see N1. The masked preview is used everywhere except the admin reveal route.
- Logs: gate (f) found no key material in the branch logs. No new log call carries the key or a held key. The debug verdict line carries only a verdict word.

### Commit and text rule

- See C1. Subject is at the category level. No cause text in commit bodies, code comments, test names, docs or the PR text (none was opened by QA).

## Commands run (exact)

- `cd /projects/Charon/backend && go build ./...` and `go vet ./internal/crowdsec/... ./internal/config/... ./internal/caddy/... ./internal/util/... ./internal/services/ ./internal/api/handlers/ ./cmd/api/`: exit 0.
- `go test -race -count=1 -timeout 25m ./internal/crowdsec/... ./internal/config/... ./internal/caddy/... ./internal/util/... ./cmd/api/...`: all ok.
- `go test -race -count=1 -timeout 25m ./internal/api/handlers/`: `ok ... 491.256s`.
- `go test -race -count=1 -timeout 25m ./internal/services/`: `ok ... 606.896s`.
- `bash scripts/go-test-coverage.sh`: `total: 93.1% statements`, `Line coverage: 92.0%`, `Coverage requirement met`, exit 0.
- `cd frontend && npm run type-check`: exit 0.
- `bash scripts/local-patch-report.sh`: `Overall 96.2 pass`, artifacts `test-results/local-patch-report.{md,json}` written.
- `bash scripts/frontend-test-coverage.sh`: `Coverage gate: PASS (lines 91.83% vs minimum 87%)`, exit 0. The script prints a deprecation notice pointing at the skill runner; it still runs the gate.
- `make lint-backend`: `6 issues`, exit 2 (B1).
- `lefthook run pre-commit`: exit 0, all hooks skipped (no staged files).
- `lefthook run pre-commit --all-files`: exit 0, all hooks passed.
- `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`: exit 0, `charon-e2e` healthy on `charon:local` (rootless daemon, the default context).
- `docker build -t charon:qa-control .` from `git archive 3c24d1b21`: exit 0.
- `npx playwright test tests/security/crowdsec-key-robustness.spec.ts tests/security/crowdsec-first-enable.spec.ts tests/security/crowdsec-import.spec.ts --project=security-tests`: `64 passed`, exit 0.
- Container gates: `docker run` with `--env-file /projects/Charon/.env`, `CHARON_ENV=e2e`, named volumes, `-p 127.0.0.1::8080`. In-container probes with `wget` (the image has no `curl`) against `http://127.0.0.1:8085/v1/decisions`. Management API calls with a per-container cookie jar. Key values were never printed; only SHA-256 prefixes were compared.

## Gaps and deviations (not run or not complete)

- `go test -race -count=3` for the four touched package groups (3t.8) was not run; the brief asked for `-count=1`. Estimated extra time about 1 hour. CI runs the full suite on the PR.
- CodeQL (Go and JS) and Trivy were not run locally (see gate 14). CI runs them on every PR.
- The Playwright run used `--project=security-tests` rather than `--project=firefox`, because firefox excludes `tests/security/**` (N2).
- The gate (f) scan covered the keys present in the containers at scan time. Keys that were replaced during the runs were not retained, so they are not in the scan set.
- Gate (d): my first hash comparison line in the shell was broken. The host file hash (`a68cc43f20ea4832`) matches the content written before the run, and the read-only mount rejected every write, so the file was not changed. The check was re-done by hash, not by a broken comparison.

## Rebase during the run

- The reflog shows `rebase (finish): returning to refs/heads/fix/crowdsec-key-robustness` at 22:06:19 UTC. QA did not run the rebase. The branch moved from `3c24d1b21`-based history to `origin/development` `5381f3f52`.
- `git range-diff 37fd1ddb2^..6e38d5984 origin/development..HEAD`: all 10 commit pairs show `=` (identical patches).
- Base changes since `3c24d1b21` (`git diff --stat 3c24d1b21 origin/development`): frontend dependencies (`package-lock.json`, `package.json`, tailwindcss 4.3.4, react-query 5.105.3), CI workflows and `scripts/`. No Go or model changes.
- Therefore the Go race, coverage and container results are for the same PR content. The frontend coverage and type-check were re-run on `01981fbba`, and the lint and lefthook runs were on `01981fbba`.

## Action list (in order)

1. B1: add the `//nolint:gosec` reasons to the six test lines (or restructure the fixtures), commit as `test:`, re-run `make lint-backend` (must print `0 issues`).
2. Optional before merge: L1 table cases for the three uncovered branches.
3. File R3 and O1 as low GitHub issues (non-security, neutral wording). N3 goes to a private advisory only if the orchestrator judges it reportable.
4. Decide C1 (subject wording) before the PR is opened; only the orchestrator commits.
5. Run `go test -race -count=3` on the four package groups, or leave it to CI, and say which in the PR.
