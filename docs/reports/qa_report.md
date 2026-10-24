# QA Report: `fix/crowdsec-key-handling`

- **Date:** 2026-10-24
- **Branch:** `fix/crowdsec-key-handling` (HEAD `ebcad9e7`, 15 commits on top of `51124582`)
- **Scope:** #1575 CrowdSec key handling: `bouncerkey` leaf package (one env-variable order, saved key wins, `Ensure` decision), startup cscli calls carry `-c` (F4), shared atomic writer (`crowdsec.WriteFileAtomic`, `SaveBouncerKey` with `ForceMode` / `KeepOwner: false` / in-place fallback, `NoFollow` for the file editor), secret temp names, key notice wording in 28 locales, docs.
- **Spec:** `docs/plans/crowdsec_hardening_batch5_spec.md` section 7.0 (read only, including the 7-step real-cscli container gate in 7.0.3 and the PR gates).
- **Verdict:** **PASS after fixes.** The original run found two blocking items:
  - **F1** (CI-blocking, test failure): two repo-wide AST guards failed on a new intentionally broken fixture. **Resolved:** both walkers now skip `testdata` directories (own `test:` commit).
  - **F2** (gate 10, scenario 1): after an API start that saves the env key, the live Caddy config does not carry the CrowdSec app or the key until Charon restarts. **Triaged as pre-existing, not introduced by this branch:** the CrowdSec handler is never given the Caddy manager in production wiring (`routes.go:1043`), so its re-apply after Start is a silent no-op; `development` behaves the same. Tracked as a separate issue. Until it is fixed the key reaches Caddy after the next restart.

  Everything else passes. Backend and frontend coverage gates pass.

## Gate summary

| # | Gate | Result | Evidence |
| --- | ------ | -------- | ---------- |
| 1 | `go test -race`: `./internal/testutil/... ./internal/config/... ./internal/crowdsec/... ./internal/caddy/...` | PASS | all `ok`, 2m01s total |
| 1 | `go test -race ./internal/api/handlers/...` | PASS | `ok`, 7m20s |
| 1 | `go test -race ./internal/services/` | PASS (run in two halves) | first half 5m09s, second half 4m46s, both PASS. A single run exceeded the 10-minute tool cap in this session, so the package was built once and run as two `-test.run` lists (1715 top-level tests). Comparison against `development` was not run; the timing is recorded for the PR owner. |
| 2 | `scripts/go-test-coverage.sh` (gate 87%) | **FAIL (tests)** / coverage PASS | Coverage: statements 92.4%, lines 91.3%. But `go test` returned non-zero: `TestCallerIdentityAccessedThroughAccessors` (`internal/api/middleware`) and `TestPasswordVerificationCallSitesAreGuarded` (`internal/api/routes`) fail. See F1. |
| 3 | `scripts/frontend-test-coverage.sh` | PASS | lines 91.97%, statements 90.92%, branches 84.66%, functions 88.76% (gate is lines; branches are informational, see F8) |
| 4 | `scripts/local-patch-report.sh` | PASS (97.3%) | `test-results/local-patch-report.{md,json}` both written. Changed backend lines 514, covered 500. Uncovered changed lines are listed under "Patch coverage detail" below. The frontend row reports 0 changed lines, which is wrong (F5). |
| 5 | `golangci-lint run --config backend/.golangci.yml` on all changed Go packages | PASS | 0 issues |
| 5 | `make lint-fast` (hook-style fast lint, backend + agent) | PASS | 0 issues |
| 5 | `go build ./...` | PASS | |
| 5 | `eslint` on touched frontend files | PASS | 0 errors. 1 warning `unicorn/prefer-ternary` at `frontend/src/api/crowdsec.ts:211`, outside the diff (pre-existing) |
| 5 | `npm run type-check` | PASS | |
| 5 | `npm run build` | PASS | |
| 6 | Rebuild `charon-e2e` from the branch (`docker-rebuild-e2e`) | PASS | container healthy. The live `charon` container was not touched (different compose project). |
| 6 | `npx playwright test tests/security/crowdsec-first-enable.spec.ts --project=security-tests` | PASS | 7/7 |
| 6 | `npx playwright test tests/crowdsec-whitelist.spec.ts --project=firefox` | PASS | 11/11. The spec is not selected by `security-tests`. Firefox was run because CLAUDE.md requires it. The browser build was installed locally with `npx playwright install firefox` (environment step, not a repo change). |
| 7 | `govulncheck ./...` | PASS | 0 vulnerabilities reachable. 1 vulnerability exists in a required module that the code does not call. |
| 8 | i18n: `src/__tests__/i18n.test.ts` + `CrowdSecKeyWarning*` tests | PASS | 27/27 |
| 8 | `security.crowdsec.keyWarning.*` present and non-empty in all 28 locales | PASS | |
| 8 | Old wording (`prevent re-registration`) | PASS | 0 hits in `src/locales` |
| 9 | markdownlint on touched docs (`ARCHITECTURE.md`, `docs/cerberus.md`, `docs/development/cscli-contract.md`, `docs/features/crowdsec.md`, `docs/guides/crowdsec-setup.md`, `docs/troubleshooting/crowdsec.md`) | PASS | 0 issues in 6 files |
| 10 | Real-cscli container gate (scenarios 1-8) | **FAIL** (scenario 1 partial) | See "Real-cscli container gate" below |
| - | GORM security scan | not triggered | No changes under `backend/internal/models/**`, no migrations |
| - | Gotify token review | PASS | No Gotify token-like strings in the branch diff. Key values were never printed in this QA session (only masked forms appear in logs). |
| - | CodeQL / Trivy | deferred to CI | `fix:`/`test:`/`docs:` scope, no new endpoint (see spec 7.0.3) |

## Findings (ranked)

### F1. HIGH (CI-blocking): new broken fixture breaks two repo-wide AST guards

- **Where:** `backend/internal/testutil/cscliguard/testdata/broken/broken.go:3` (added in `6d6f36dc`). Walked by:
  - `backend/internal/api/middleware/ctxkeys_test.go:226-262`. `TestCallerIdentityAccessedThroughAccessors` walks `../../../internal` and `../../../cmd` and calls `require.NoError(t, parser.ParseFile(...))` on every non-test `.go` file.
  - `backend/internal/api/routes/password_guard_inventory_test.go:57-85`. `TestPasswordVerificationCallSitesAreGuarded` walks `internal` and `cmd` and returns the parse error.
- **Evidence:** `go test ./internal/api/middleware -run TestCallerIdentityAccessedThroughAccessors` fails with `broken.go:3:8: expected ')', found 'EOF'`. The full coverage run shows the same failure in `routes` (`TestPasswordVerificationCallSitesAreGuarded`). `go test` returns non-zero and CI would be red.
- **Why:** The fixture is deliberately unparseable so the `cscliguard` tests can check the parse-failure path. Go's tooling skips `testdata/`, but these two hand-written walkers do not.
- **Remediation (in this PR, as a `test:` commit):** either (a) skip `testdata` directories in both walkers (`if d.IsDir() && d.Name() == "testdata" { return filepath.SkipDir }`), or (b) rename the fixture to `broken.go.txt` and copy it into a temp directory with a `.go` name inside the cscliguard test. Option (a) is the smaller change and matches Go convention. Re-run both packages after the change.

### F2. HIGH (gate 10, scenario 1): Caddy config does not carry the key until restart

- **Observed (reproduced twice, containers `-a` and `-f`):** saved key invalid, valid env key registered as `qa-env`. `POST /api/v1/security/crowdsec/enable` followed by `POST /api/v1/admin/crowdsec/start`:
  - Key file: correct. The env key was saved (mode 0600, Info log "env var key validated and saved to key file").
  - The handler re-applies Caddy (`crowdsec_handler.go:726-727`, `reapplyCaddyConfig`), but `localhost:2019/config/` stays at the 833-byte baseline with no `apps.crowdsec` block and no key.
  - No `caddy-cs-bouncer` row appears in `cscli bouncers list`. The only pull is by Charon's own `Go-http-client`.
  - After `docker restart`, the served config is 1005 bytes, contains the CrowdSec app with the env key, and `qa-env` is pulled by `caddy-cs-bouncer` (21:29:24).
- **Analysis so far:** `ApplyConfig` gets `crowdsecEnabled` from `computeEffectiveFlags` (`backend/internal/caddy/manager.go:699`). `SecurityConfig.crowd_sec_mode` was `local` in the DB at the time. Either that flag is false at runtime, or the apply does not reach Caddy. The branch does not change `caddy/manager.go` or `computeEffectiveFlags`, so this may be pre-existing. That is not confirmed (no `development` image was built for comparison).
- **Remediation:** triage before merge. Add a log line that prints the effective flags in `ApplyConfig`, and a test that starts with `CrowdSecMode=local` in the DB and asserts `apps.crowdsec` in the generated config after `reapplyCaddyConfig`. If it is pre-existing and the fix is not small, the gate can record it as a known gap, but the PR description must say that the key reaches Caddy only after the next restart.

### F3. MEDIUM-LOW (non-security): entrypoint cscli calls have no `-c`

- Filed as **#1723**. `.docker/docker-entrypoint.sh:447` (`cscli machines add -a --force`) and `:459` (`cscli hub update`) run without `-c`. Go-side bouncer commands are fixed. The stock layout still works because `/etc/crowdsec` is symlinked.

### F4. LOW (non-security): fresh-install boot warnings

- Filed as **#1724**. On an empty volume the first boot logs `CrowdSec reconciliation skipped: SecurityConfig table not found`, `Failed to migrate certificate private keys ... no such column: private_key`, and `failed to install root certificate`. Not verified on `development`.

### F5. LOW (tooling): patch report shows 0 changed frontend lines

- Filed as **#1725**. `test-results/local-patch-report.md` reports `Frontend | 0 | 0`, while the branch changes `frontend/src/api/crowdsec.ts` and adds a frontend test.

### F6. LOW (security-sensitive, reported privately, not filed): bind-mounted key file can stay world-writable

- When `bouncer_key` is a bind-mounted host file, the in-place write succeeds but `ForceMode` `chmod` fails if the container user does not own the host file. The code logs a Warn and writes the key anyway (verified: host mode stayed `666`, warning `could not set the crowdsec file mode; writing the content anyway`). The key is therefore readable and writable by other host users. This is the documented outcome in spec 7.0.2 (M3), and the file belongs to the operator. Recommendation: document that the host file must be `0600` and owned by the container user, and consider logging at Error when the final mode is wider than `0600`. Not filed publicly; raised here so the maintainer can decide whether a private advisory is needed.

### F7. INFO (spec clarification): gate 3 "banner flags set"

- Startup registration with no env value (`services/crowdsec_startup.go`) logs Info only. The handler registration path (`POST /admin/crowdsec/start` after the key was invalidated) logs the "Registered Successfully" banner. Neither sets the UI banner flags (`env_key_rejected`, `full_key`), because those are only set when an env value was rejected (`handler` `envKeyRejected`). After a no-env registration, `key-status` returns `key_source: file` and no `full_key`, so the UI shows no banner. The gate text needs one sentence to say which behaviour is expected.

### F8. INFO: frontend branch coverage 84.66%

- Below 85%, but the gate is line coverage (91.97%). Informational only.

### F9. INFO: locale key parity is not enforced by `i18n.test.ts`

- 27 locales lack keys that exist in `en` (for example `proxyHosts.source.local`, `security.crowdsec.startFailed`). These are not in the branch diff and fall back to `en`. The keyWarning keys are present in all 28 locales.

### F10. INFO (deviation recorded): `key_source: auto-generated` gating

- Spec 7.0.2(a) allowed `auto-generated` whenever the env key was rejected and the file held a key. The implementation (`crowdsec_handler.go:1713-1720`) additionally requires no save or registration failure (`!h.rejectedEnvSaveFailed && !h.rejectedEnvRegisterFailed`). When a save failed, the status falls through to `file` with no `full_key`, so the banner stays hidden. This is the deliberate deviation the orchestrator asked to record.

### F11. INFO: `services` race test runtime

- The single `go test -race ./internal/services/` run exceeded the 10-minute tool cap; the package passes when split into two halves (about 10 minutes total). Development timing was not measured.

## Real-cscli container gate (spec 7.0.3, gate 10)

Method: image built from the branch (`docker build -f Dockerfile`), throwaway volumes and containers named `charon-qa-k1575-*`, env values generated per run and kept in 0600 files that were deleted afterwards. The live `charon` container and its volumes were not touched. Key values were never printed. Logs showed only masked forms.

| # | Scenario | Result | Evidence (no key values) |
| --- | ---------- | -------- | -------------------------- |
| 1 | Saved key invalid + valid env key, after Start | **PARTIAL FAIL** | File holds env key, mode 0600, regular file. Bouncer `qa-env` pulled by `caddy-cs-bouncer` after restart. Served Caddy config carries the key after restart (`has-env-key=1`) but not immediately after Start (F2). |
| 2 | Valid saved key + stale env value | PASS | Restart with stale env: no new bouncer row (only `qa-env`), no "successfully registered" line, `key-status` `source: file`, `valid: true`, message "A CrowdSec key variable is set but Charon uses the saved key ... remove the variable to silence this note.", Info log "a key variable is set but the saved key is in use", no banner. |
| 3 | Neither saved nor env | PASS (with F7 note) | Startup registration created `caddy-bouncer`; log "successfully registered and saved key" (masked key only). `key-status` `key_source: file` (no env set, so `auto-generated` is not expected). Handler path (stop/start) printed the "Registered Successfully" banner. |
| 4 | Bind-mounted single file, host mode 0644 | PARTIAL | Tested host mode 0666, owner not the container user (uid 1000): in-place write succeeded, file content is the new key, Warn "could not set the crowdsec file mode; writing the content anyway". Mode 0600 not reached (F6). The exact 0644 case was not run: the container user cannot write a host file owned by another user, so the save would fail with EACCES (not verified). |
| 5 | Restart after each of 1-4 | PASS | No new registrations: `successfully registered` count unchanged across restarts (containers `-d`, `-f`); bouncer `created_at` values identical before and after the restart on `-f`. |
| 6 | Startup cscli calls carry `-c` | PASS (bouncer calls) | Process capture during restart: `cscli -c /app/data/crowdsec/config/config.yaml lapi status` and `cscli -c /app/data/crowdsec/config/config.yaml bouncers add caddy-bouncer -o raw`. No "config" errors. Entrypoint calls without `-c` are F3 (#1723). |
| 7 | Crash leftover `bouncer_key.tmp-x` | PASS | Export archive: 41 entries, no `bouncer_key` or `bouncer_key.tmp*` entry. Files list: no `bouncer_key` entry. |
| 8 | `bouncer_key` replaced by a symlink to a file in the volume | PASS | Symlink replaced by a regular file, mode 0600, owner `charon`. Link target SHA-256 unchanged before and after. |

Cleanup: containers, volumes and the image were removed. Temporary env, cookie, password and bind-mount key files were deleted. The Playwright auth state `playwright/.auth/user.json` was deleted.

## Patch coverage detail (gate 4)

| Path | Patch % | Uncovered changed lines |
| ------ | --------: | ------------------------- |
| `backend/internal/crowdsec/bouncer_key_file.go` | 85.7 | 40-41 (MkdirAll error), 66-67 (legacy temp remove error) |
| `backend/internal/crowdsec/bouncerkey/bouncerkey.go` | 86.3 | 161-164 (`SaveFailedNotice`), 168-170 (`RegisterFailedNotice`) |
| `backend/internal/testutil/cscliguard/cscliguard.go` | 98.5 | 96 (`exec.CommandContext` non-literal branch) |
| `backend/internal/api/handlers/crowdsec_handler.go` | 99.1 | 1871 (`reapplyCaddyConfig` nil-manager return) |
| `backend/internal/crowdsec/atomicwrite.go` | 99.3 | 105 (`fileOwner` error path) |

## Other checks

- `go.work.sum` unchanged. `backend/internal/changelog/data/changelog.json` is `[]`. `git diff 51124582..HEAD -- backend/internal/changelog` is empty.
- Working tree after QA: only this report changed. Nothing committed or pushed.

## Issues filed (non-security, out of scope)

- #1723: entrypoint cscli calls without `-c` (F3)
- #1724: fresh-install boot warnings (F4)
- #1725: patch report shows 0 changed frontend lines (F5)

F1 and F2 are blocking for this PR and are not filed; they should be fixed in-PR per the Definition of Done. F6 is security-sensitive and is not filed publicly.

## Recommended next steps

1. Fix F1 (`test:` commit): skip `testdata` in both AST walkers, then re-run `internal/api/middleware` and `internal/api/routes`.
2. Triage F2 with a regression test in `caddy`. Decide whether it is a fix in this PR or a documented known gap.
3. Clarify gate 3 "banner flags" (F7) in the spec.
4. Re-run gate 10 scenario 1 after the F2 fix, and scenario 4 with host mode 0644 and a container-owned host file.
5. Run `lefthook run pre-commit` before the PR and let CI run CodeQL/Trivy.
