# QA Report: CrowdSec Whitelist, Acquisition, Busy Guard and Retention Tidy

- Branch: `fix/crowdsec-backup-acquisition-tidy`
- HEAD: `1644f823` (base `development` `cbe2c76b`, 9 commits)
- Scope: whitelist writer safety and lock (#1718, W1), kept-snapshot cap with 7-day floor (#1707), acquisition permission error codes (#1708), shared `Hold()` busy guard (#1730, T1), frontend `crowdsecBusyMessage` and `busyOther` translations, docs.
- Working tree: clean before the run; the only change afterwards is this report (`docs/reports/qa_report.md`, uncommitted). Nothing was committed, pushed, or opened as a PR.
- Verdict: **PASS, no blockers.** Every local gate passes. Lint has 0 errors. The only residual items are minor test-coverage and wording notes (see Findings).

## Gate Summary

| # | Gate | Result |
|---|------|--------|
| 1 | `go build ./...`, `-race` on crowdsec, handlers and services (services `-count=1`), other touched packages | PASS |
| 2 | `scripts/go-test-coverage.sh` (gate 87%) | PASS, line 91.5%, statement 92.6%, no FAIL |
| 3 | `scripts/local-patch-report.sh` | PASS, backend 98.8%, frontend 100%, overall 98.9% |
| 4 | Frontend coverage (gate 87%), type-check, lint, build, i18n and CrowdSec tests | PASS (lines 91.95%; branches 84.64% informational) |
| 5 | `make lint-fast`, `make lint-backend`, markdownlint, gofmt | PASS (0 issues; gofmt flags one pre-existing fixture) |
| 6 | `scan-gorm-security.sh --check`, `govulncheck` | PASS (0 CRITICAL/HIGH; no vulnerabilities in called code) |
| 7 | Rebuild `charon-e2e`; targeted Playwright (one project per run) | PASS (security-tests 53/53; firefox whitelist 11/11) |
| 8 | Real-cscli throwaway container gate (a) to (f) | PASS, all sub-gates |
| 9 | Changelog and go.work.sum unchanged | PASS |

Security scans: CodeQL and Trivy were not run locally. This branch adds no feature surface (commits are `test:`, `fix:` and `docs:`), so CLAUDE.md defers them to CI.

## Gate 1: Build and Race Tests

- `go build ./...`: OK.
- `go test -race -count=1 ./internal/crowdsec/...`: ok (crowdsec 115.6 s, bouncerkey ok).
- `go test -race -count=1 ./internal/api/handlers/`: ok (426.7 s).
- `go test -race -count=1 ./internal/services/`: ok (572.3 s). Run with `-count=1` as instructed (issue #1742).
- Other packages: `git diff --stat cbe2c76b..HEAD` lists changes only under `backend/internal/crowdsec`, `backend/internal/api/handlers` and `backend/internal/services` (plus frontend, docs, and the E2E spec). No other Go package was touched, so no extra non-race run was needed.

## Gate 2: Backend Coverage

- `scripts/go-test-coverage.sh` finished in under 600 s on the second attempt (the first call hit the tool cap and was re-attached in place, not split). Log: `/var/tmp/qa_backend_cov.log`.
- Statement coverage 92.6%, line coverage 91.5%, gate 87%: met. Zero `FAIL` lines.

## Gate 3: Local Patch Coverage

Artifacts present: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`. Re-run after the frontend coverage gate so the frontend figure is fresh.

| Scope | Changed | Covered | Patch % | Status |
|---|---:|---:|---:|---|
| Overall | 185 | 183 | 98.9 | pass |
| Backend | 172 | 170 | 98.8 | pass |
| Frontend | 13 | 13 | 100.0 | pass |

Expected ~100% backend was not reached. Two backend lines are uncovered, both in `backend/internal/crowdsec/backup.go`, function `capRetained`:
- line 459: `continue` for a retained snapshot directory whose name does not parse as a timestamp;
- line 472: the path tie-break in the sort comparator, reached only when two kept snapshots share an identical timestamp.

Neither is a blocker (both are above the 85% threshold). They are recorded as a test gap (see Findings).

## Gate 4: Frontend

- Coverage (`scripts/frontend-test-coverage.sh`): statements 90.91%, branches 84.64% (informational), functions 88.77%, lines 91.95%. Gate met.
- `npm run type-check`: exit 0.
- `npm run lint`: **0 errors**, 1358 warnings across the whole repo (pre-existing backlog, filed as #1743). Warnings in files this branch touched:
  - `frontend/src/api/__tests__/errorBody.test.ts`: `import-x/order` (3).
  - `frontend/src/components/crowdsec/HubItemsRestorePanel.tsx`: `import-x/order` (2).
  - `frontend/src/components/crowdsec/__tests__/HubItemsRestorePanel.test.tsx:76`: `unicorn/no-useless-undefined`.
  - `frontend/src/pages/CrowdSecConfig.tsx:25-26, 113`: import order, and `Date.now` during render at line 113. Line 113 is pre-existing (unchanged from the base).
  - `frontend/src/pages/__tests__/CrowdSecConfig.i18nKeys.test.ts:20-21`: `unicorn/prefer-ternary`, `unicorn/no-useless-undefined`.
  - `frontend/src/pages/__tests__/CrowdSecConfig.spec.tsx:285,288`: `testing-library/no-node-access`.
  - `i18n-json/identical-keys` on the de, es, fr and zh `translation.json`: pre-existing (the `proxyHosts.source` block is absent in those locales on the base too). Tracked as #1741.
- `npm run build`: exit 0.
- Targeted vitest (i18n test, CrowdSec i18n-keys, errorBody, presets and preset apply-jobs, HubItemsRestorePanel, CrowdSecConfig importErrors, installProgress, spec and config): 10 files, 176 tests, all passed.

## Gate 5: Lint and Formatting

- `make lint-fast` (backend and agent): 0 issues.
- `make lint-backend` (full golangci-lint): 0 issues.
- markdownlint on `docs/features/crowdsec.md` and `docs/troubleshooting/crowdsec.md`: 0 issues.
- `gofmt -l ./internal ./cmd`: flags `internal/testutil/cscliguard/testdata/broken/broken.go`. This is a deliberately broken fixture added in `6d6f36dc`, before the base; it is not in this branch's diff and the Go toolchain ignores `testdata`.

## Gate 6: Security Scans

- `./scripts/scan-gorm-security.sh --check`: 74 Go files scanned; 0 CRITICAL, 0 HIGH, 0 MEDIUM. 2 INFO suggestions (informational).
- `govulncheck ./...` (backend): no vulnerabilities in called code; 1 vulnerability in a required module that the code does not call.
- Gotify token review: no Gotify references in the diff; no secrets added to logs. The only secret-looking matches are a test fixture payload (`[]byte("secret\n")`) in an unreadable-file test.

## Gate 7: E2E (Playwright)

- Rebuilt `charon-e2e` with the `docker-rebuild-e2e` skill (no `--clean`, no `--no-cache`). Result: container healthy, image `charon:local` built from this branch. The live `charon` container (compose project `charon`) was not touched; `charon-e2e` is in compose project `compose`.
- `--project=security-tests`, one run: `tests/security/crowdsec-background-install.spec.ts`, `tests/security/crowdsec-security-toggle-busy.spec.ts`, `tests/security/crowdsec-hub-items.spec.ts`. **53 passed.**
- `--project=firefox`, one run: `tests/crowdsec-whitelist.spec.ts`. **11 passed.**
- Full suite was not run locally (per CLAUDE.md); CI will confirm.

## Gate 8: Real-cscli Throwaway Container Gate

Containers and volumes (unique names, all removed at cleanup):
- `charon-qa-gate-6e6945` with volume `charon-qa-gate-data-6e6945`: main container. Published on 127.0.0.1 only.
- `charon-qa-ro-d46a73` with volume `charon-qa-ro-data-d46a73`: read-only bind test container.

Image: `charon:local` (CrowdSec `cscli` v1.8.2-rc2 inside). Setup used the Charon API with a throwaway admin account; no real secrets were printed. Hub access worked (presets listed, presets pulled).

### (a) Acquisition errors: PASS

Acquisition file: `/app/data/crowdsec/config/acquis.yaml` (reached through `/etc/crowdsec`).

| Case | GET | PUT | Code |
|---|---:|---:|---|
| Healthy | 200 | 200 | none |
| File mode 000 (charon user cannot read) | 500 | 500 | `acquisition_config_unreadable` |
| Config directory 555 (cannot create backup or replace) | - | 500 | `acquisition_config_not_writable` |
| Read-only bind mount over the file | 200 | 500 | `acquisition_config_not_writable` |
| Restored | 200 | 200 | none |

Observation: a file mode of 444 alone does not block saves (the write is temp-file-then-rename in a writable directory), so it still returned 200. This matches the code, which keys on directory and file access. Not a defect, but worth knowing when reading the 'check the owner and mode of the file and its folder' message.

### (b) Busy guard: PASS

- During an async preset apply (job `state: running`), `start`, `stop`, `presets/apply` and whitelist add each returned **409** with `error_code: crowdsec_busy`, text "CrowdSec is busy installing a preset; try again when it finishes", and the job id.
- Console enroll during an apply also returned 409 `crowdsec_busy` with the job id (after enabling the `feature.crowdsec.console_enrollment` flag).
- During a Start's LAPI wait window (Start raced with an async apply, six iterations): apply returned **409 `crowdsec_busy`** with "CrowdSec is finishing another change; try again in a moment", and **no job_id**. Start returned 200 each time.
- Note: the preset apply job completes in under one second, so the busy window is short. The Start race used `&` and `wait` inside one foreground command so that both calls finished within the same tool call.

### (c) Whitelist writer: PASS

- 10 parallel adds (`10.20.0.1` to `.10`): all 201.
- A client with a 50 ms timeout still got 201. A client with a 1 ms timeout aborted (curl exit 28). The DB still committed (`10.20.0.98` present in the API list and in the file), so the file matched the DB.
- `charon-whitelist.yaml` matched the API list exactly (12 entries, IPs and CIDRs); `cidr: []`.
- Mode `640`, owner `charon:charon`.
- A stale legacy `charon-whitelist.yaml.tmp` (planted before the adds) was removed.

### (d) Retention cap and floor: PASS

Method: snapshot directories were renamed to aged timestamps and retain markers were placed by hand. Failing imports were triggered by making `crowdsec/config` read-only (500 "failed to create config dir"; the rollback failed and retained the snapshot). Ages were computed with the host's GNU `date`.

- **Floor test:** four marked snapshots aged 1 to 4 days, then a failing import (6 kept, all under 7 days). All six markers kept. No directory deleted.
- **Cap test:** two marked snapshots aged 8 and 9 days, then a failing import (7 kept, five under 7 days). The 9-day and 8-day markers were removed; the five younger markers stayed (floor wins over cap). No directory deleted: 12 `crowdsec.backup.*` entries before and after the retain (7 directories plus 5 markers).
- **Removal at next prune:** the 9-day unmarked snapshot was removed by the next prune after four successful applies (unmarked count went above `DefaultBackupRetention = 5`). The 8-day unmarked snapshot stayed (it is among the newest five unmarked). See Findings for the wording note.

### (e) Observation: whitelist file editing (W2): PASS (observation recorded)

- Editing `config/parsers/s02-enrich/charon-whitelist.yaml` via the file editor (`POST /admin/crowdsec/file`) persisted. It did **not** regenerate within 5 s. File and API/DB then diverged (file had an extra `192.0.2.250` and lacked `10.20.0.98`; API did not).
- The next whitelist add (`10.20.0.97`) regenerated the file from the DB: the hand-added IP was dropped and `10.20.0.98` came back. Mode stayed `640`.
- `ImportConfig` with a fresh export (which contains the whitelist file) replaced the edited file with the archived content, so the file and API matched again.
- Behavior summary: the file is not authoritative and is rewritten only on the next whitelist write or an import. Until then, a hand-edited file is live in CrowdSec while the Charon list does not show it. See the private note below.

### (f) Basic start/stop/on/off: PASS

- Stop: 200, `stopped`. Status: `running: false`.
- Start: 200, `started`, LAPI ready.
- Disable security.crowdsec: 200 `enabled: false`. Enable: 200 `enabled: true`.
- Start again: 200 `already_running`.

### Container health

Logs for both containers: 0 panics, 0 data-race reports, 0 fatal lines. One startup warning in the main container: `Failed to create log file for LogWatcher` for `/var/log/caddy/access.log` (expected in a throwaway container with no Caddy logs; recorded, not filed).

## Gate 9: Changelog and go.work.sum

- `git diff 034d318d..HEAD -- backend/internal/changelog`: empty (0 bytes). Working tree has no change there.
- `go.work.sum`: unmodified against `HEAD` and against `034d318d`.

## Findings

Severity follows the QA rules. None is a CI blocker.

### Non-security, filed

1. **Frontend lint backlog: 1358 warnings (0 errors)**, issue #1743. Low. Repository-wide. Suggested cleanup in a chore PR, then `--max-warnings` in CI.
2. **Missing `proxyHosts.source` block in de, es, fr and zh locales**: already tracked as #1741. Not filed again (duplicate). Pre-existing, not introduced by this branch.

### CrowdSec-specific, reported for the spec (not filed)

3. **Patch-coverage gap in `capRetained`** (`backend/internal/crowdsec/backup.go:459` and `:472`). Low. Add unit tests for an unparseable retained directory name and for two kept snapshots with equal timestamps. Both lines are new code in this branch, so the test belongs in this PR per CLAUDE.md. Backend patch coverage is 98.8%, above the threshold.
4. **Retention wording vs. behavior** (`backend/internal/crowdsec/backup.go` retain marker text, and `docs/troubleshooting/crowdsec.md`). Low. The marker text says an older snapshot that loses its marker "is removed at the next cleanup". In practice, an unmarked snapshot is removed only when the unmarked count exceeds `DefaultBackupRetention` (5). Observed: an 8-day unmarked snapshot survived a prune. Suggest rewording to "removed once newer snapshots fill the retention count" or similar.
5. **Generic error message on a failed clear in import** (`crowdsec_handler.go`, import path): 500 "failed to create config dir" is returned when `ClearConfigKeeping` fails for any reason (including permission problems). Low. A coded message (similar to `acquisition_config_not_writable`) would help operators.
6. **Acquisition message vs. file mode** (informational): a 444 file mode does not block saves; only folder permissions do. The message "Check the owner and mode of the file and its folder" is accurate but may mislead. Low.

### Security-sensitive, reported privately (not filed)

7. **Whitelist file is not authoritative** (see gate 8(e)). A hand edit to `charon-whitelist.yaml` is applied by CrowdSec immediately, is not reflected in the Charon whitelist API or UI, and persists until the next whitelist write or import. This means an entry can be active that the operator's Charon list does not show. Treat as medium, and handle as a private advisory per `SECURITY.md` if you judge it security-relevant. Nothing was filed publicly. The fix direction is for the writer to be the only source of the file (or for the file to be regenerated on read or start).

## Commands and Logs

Logs under `/var/tmp`: `qa_backend_cov.log`, `qa_fe_cov.log`, `qa_fe_tc.log`, `qa_fe_lint.log`, `qa_fe_build.log`, `qa_fe_targeted.log`, `qa_lintfast.log`, `qa_lintbe.log`, `qa_mdl.log`, `qa_gorm.log`, `qa_govuln.log`, `qa_rebuild.log`, `qa_pw_security.log`, `qa_pw_wl.log`. Removed at cleanup, except this report.
