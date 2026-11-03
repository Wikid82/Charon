# QA Report: CrowdSec Bouncer Registration (PR 15)

- Branch: `fix/crowdsec-bouncer-registration`, HEAD `5bab022f8` (11 commits ahead of `origin/development` `bdd52d144`).
- Date: 2026-11-03
- Spec: `docs/plans/crowdsec_hardening_batch5_spec.md` section 3w (3w.13 authoritative), gates G1 to G5 from 3w.8, with `SECURITY.md` and `CLAUDE.md`.
- Scope: test and verification only. No application code, commits or pushes.

## Verdict

**Not ready to merge.** Two Definition of Done items fail on this HEAD:

1. Pinned golangci-lint v2.15.0 reports 2 issues in branch test files (required: 0).
2. Backend unit tests are not green: `internal/api/handlers` has a timing-bounded test that fails under load, and `internal/caddy` has a test that fails only because of a gitignored local compose file (expected, excluded below).

All container gates G1 to G5 passed (G4 partially, see below). The targeted Playwright specs pass (16/16).

## Blockers

| ID | Item | Evidence | Remediation |
|---|---|---|---|
| B1 | Pinned golangci-lint reports 2 issues | `docker run --rm -v $PWD:/app -w /app/backend golangci/golangci-lint:v2.15.0 golangci-lint run --timeout=25m ./...` prints `2 issues`: `internal/api/handlers/crowdsec_registrations_test.go:85:7` errcheck (`Error return value is not checked`; the `case <-drained:` receive in the drain test discards the channel's error value), and `internal/api/handlers/crowdsec_register_outcomes_test.go:84:12` gosec G304 (`os.ReadFile(path)` in `bodyText`). The `golangci-lint-fast` lefthook hook passes, so it does not catch these. | Check or deliberately discard the drain error in the test. In `bodyText`, sanitise the path with `filepath.Clean` (project rule) and/or add a justified `//nolint:gosec` with the reason. Re-run the pinned command until it prints 0 issues. |
| B2 | `internal/api/handlers` timing assertion fails under load | Full coverage run: `TestRealCommandExecutor_Execute_OutputLimit/output_beyond_the_cap_on_stderr...` asserted `"4.519515962s" is not less than "4s"`. Re-run in isolation (`-count=3`): PASS. The test predates this branch (added in `aa69da135`, reachable from `bdd52d144`). | Relax the bound or use a deadline that does not depend on wall time under load, then re-run. CLAUDE.md does not allow deferring this. |
| B3 | `-race` perf threshold tests fail under concurrent load | `TestPerf_GetStatus_AssertThreshold` (P95 8.52 ms vs 6.00 ms) and `TestPerf_GetStatus_Parallel_AssertThreshold` (21.9 ms vs 15.0 ms), in the handlers shard that ran alongside the other race shard on 4 cores. Re-run in isolation with `-race -count=2`: PASS. | Treat as load-sensitive. Either run the race handlers shard without a concurrent shard in CI, or give these asserts a race-aware budget. Not a branch regression by evidence so far. |

## Expected or excluded items

- `TestShippedFilesDoNotSetAdminAPIVariable` (`internal/caddy`) fails only on `.docker/compose/docker-compose.test.yml:29` (`CHARON_CADDY_ADMIN_API=http://localhost:2019`). That file is gitignored (`.gitignore:296`) and local-only, as the brief states. It is not tracked, so CI is unaffected. It fails in the coverage run and the race run.
- `.docker/compose/docker-compose.test.yml` still contains the `API_URL` and duplicate key lines (spec m2 cleanup). That is the same local-only file, so nothing in the branch needs it.

## Gate results

| Gate | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | PASS |
| Backend coverage | `scripts/go-test-coverage.sh` | Coverage PASS: statements 93.1%, line 92.0%, gate 87%. Script exit 1 because of B2 and the excluded caddy test. |
| Backend race | `go test -race` (sharded: handlers alone, all other 48 packages with `-p 3`) | Other shard: 43 packages ok, `internal/caddy` fails only on the excluded local-file test. Handlers shard: fails only the two load-sensitive perf asserts (B3). Both pass in isolation. |
| Frontend coverage | `scripts/frontend-test-coverage.sh` | PASS: 320 files, 4047 tests passed (2 todo). Statements 90.99%, branches 85.12% (floor 85%, margin 0.12 points), functions 88.39%, lines 92.01% (gate 87%). |
| Frontend type-check | `npm run type-check` | PASS |
| Frontend build | `npm run build` | PASS |
| Local patch preflight | `scripts/local-patch-report.sh` (default baseline `origin/development...HEAD`, strict) | PASS. Overall 234/234 (100%), backend 225/225, frontend 9/9. Artifacts written: `test-results/local-patch-report.md` and `.json`. |
| Pinned lint | golangci-lint v2.15.0 in Docker | FAIL: 2 issues (B1). |
| GORM security | `./scripts/scan-gorm-security.sh --check` | PASS: 0 issues. |
| Lefthook | `lefthook run pre-commit --all-files` | PASS (exit 0). Frontend ESLint: 0 errors, 1355 warnings. Semgrep: timeout warnings on three test files. |
| Shell | `shellcheck` on changed `.sh` files | PASS. The deleted `configs/crowdsec/register_bouncer.sh` is gone from disk. |
| Workflows | `actionlint .github/workflows/security-pr.yml` | PASS |
| Hub mirror | `bash scripts/cscli_hub_mirror_test.sh` | PASS: 140 passed, 0 failed |
| Markdown | `markdownlint` on changed `.md` files | PASS |
| Playwright | `PLAYWRIGHT_BASE_URL=http://127.0.0.1:18080 PLAYWRIGHT_CADDY_PROXY_PORT=18180 npx playwright test tests/security/crowdsec-ui-states.spec.ts tests/security/crowdsec-key-robustness.spec.ts --project=security-tests` | PASS: 16 of 16. Includes the previously `fixme` "neutral badge when registration is unknown" test, now enabled. No `fixme`/`skip` left in either spec. |
| Docs build | `bash docs-site/scripts/sync-docs.sh && cd docs-site && zensical build --strict` | NOT RUN: `zensical` is not installed on this host. |
| CodeQL and Trivy | local scan | Not run. The branch is `fix:`/`test:`/`docs:` only with no new feature surface, so CI owns it (per CLAUDE.md). |

Not run as a local gate: `docker-rebuild-e2e`. That skill rebuilds `charon:local` and injects a changelog fixture into a tracked file. The brief asked to avoid retagging `charon:local`, so the Playwright run used a separate image (see Container gates).

## Container gates (3w.8)

Throwaway containers, volumes and images used distinct names (`charon-qa-*`, `charon:qa-bouncer`, `charon:qa-prev`). They were removed at the end. The live `charon` and `charon-e2e` containers and the `charon:local` image were not modified.

| Gate | Check | Result |
|---|---|---|
| G1 (#1819 row sync) | Previous image built from `bdd52d144` (`git archive`, no worktree). Seeded the state the previous build leaves behind: setting `security.crowdsec.enabled=true`, default row `disabled`. Previous image boot: reconciliation starts CrowdSec with `mode=local` and the row stays `disabled`. New image boot on the same volume: row becomes `local`, engine starts and verifies, bouncer validated. Restart: row stays `local`, `enabled` column stays `0`, no sync errors. | PASS |
| G2 (#1808 register keeps saved key) | `POST /api/v1/admin/crowdsec/bouncer/register` with a valid saved key. Key file sha256 identical before and after, key from `GET .../bouncer/key` identical, `cscli bouncers list` still shows one validated bouncer. | PASS |
| G3 (#1810 re-registration and shutdown) | (a) `cscli bouncers delete`, then restart: boot re-registers, exactly one validated bouncer and a new saved key. (b) A `cscli` shim (throwaway container only, root) slept 8 s on `bouncers add`; SIGTERM via `docker stop -t 45` landed while `add` was running (marker file confirmed). Container exited with code 0 in about 0.9 s from the stop. The next boot found one validated bouncer and a key that validated. | PASS for the end state. Observation: at boot, the process exited about 2 s after SIGTERM, well before the shim's 8 s sleep finished. The boot-time registration is not tracked by the handler registry (the plan records this as a known limit). The shutdown wait therefore covers only handler-initiated registrations. |
| G4 (#1809 healthy path) | Healthy state reports `registration: "registered"`, `registered: true` through the API. | PARTIAL: API verified. No spec asserts the healthy badge in the UI. The unknown-state badge is covered by the passing Playwright test. |
| G5 (O1 / #1722) | Container recreated on the same volume with `CHARON_SECURITY_CROWDSEC_API_URL=http://127.0.0.1:9`. Exactly one `warning` line: `CHARON_SECURITY_CROWDSEC_API_URL is ignored for key checks; remove it from docker-compose.yml`. The log line does not contain the URL value. Saved key file hash unchanged. Bouncer still validated, and `GET .../bouncer` reports `registered`. | PASS |

Container gate notes:
- G3 signal timing: the stop used a grace period of 45 s. The exit-after-about-2-s behavior above is the one place where the observed behavior differs from the plan's "bounded drain" expectation. The plan records boot-time registration as outside the drain, so this matches the stated limit rather than a new defect.
- The QA stack's compose copy dropped the host Docker socket mount that the E2E compose file includes. That keeps the test app away from the live Docker host.

## Security-sensitive notes (hand-back only, no public issue)

- Unauthenticated `POST /api/v1/admin/crowdsec/bouncer/register` and `GET /api/v1/admin/crowdsec/bouncer/key` return 401. The admin role gate on these routes was not exercised with a non-admin account.
- `GET /api/v1/admin/crowdsec/bouncer/key` returns the full bouncer key to an authenticated caller. This predates the branch. It is an existing admin-only behavior, recorded here for awareness.
- The register endpoint now refuses with 409 `crowdsec_busy` or 503 when busy or shutting down, as the plan records (W-R9). No caller in the repository depends on the removed script's output.
- No Gotify token strings were found in the branch diff.
- Forbidden wording check (`lock-out`, `invalidates the key`, `re-register each start`): no matches in the branch diff.
- Pre-existing observation, not from this branch: after a container restart the reconciliation logs `PID exists but is not CrowdSec (PID recycled)` as a warning at boot. Not investigated further.

## Environment and process notes

- Live container change, outside this QA: the live `charon` container (`charon:local`) was recreated at `2026-11-03T22:11:23Z` by a Compose project named `containers` (`charon-e2e` was not affected). None of my commands used that project name or that container name. The source of the recreation was not determined. It is healthy now. Please confirm whether another tool (for example the Dockhand updater) triggered it.
- Seed-state caveat (G1): my first attempts to seed the pre-carry-over shape were disturbed by WAL and file-ownership issues on the copied database, and I did not complete a clean reproduction of the exact legacy `mode=local` shape. The G1 result above uses the state the previous build actually leaves (setting `true`, row `disabled`), which is the state the new sync must repair.
- `.docker/compose/docker-compose.test.yml` (gitignored, line 29) is the source of the local-only caddy test failure, as the brief states.
- Frontend branch coverage has a 0.12 point margin over the 85% floor. Keep the new three-state tests in the suite.

## Remediation order

1. B1: fix the two lint findings and re-run the pinned golangci-lint command to 0 issues.
2. B2: make the output-limit timing test independent of wall time under load (existing `development` test; fix in this PR per CLAUDE.md).
3. B3: decide the race-run policy for the perf asserts (CI-side or race-aware budget).
4. Run the docs build with `zensical` available, or confirm the docs CI job covers it.
5. Confirm the live container recreation source (owner action).
