# QA Report: `fix/crowdsec-hub-item-scan`

- **Date:** 2026-10-24
- **Branch:** `fix/crowdsec-hub-item-scan` (HEAD `28575985`, 10 commits on top of `da0d1b21`)
- **Scope:** #1712 (`.yml` hub items counted), #1711 (Security page no longer pre-writes `security.crowdsec.enabled`), #1713 (whitelist add fallback and field errors), R4 (ImportCrowdSec busy text), Security toggle translations, new E2E `tests/security/crowdsec-security-toggle-busy.spec.ts`
- **Spec:** `docs/plans/crowdsec_hardening_batch5_spec.md` section 3h (read only)
- **Verdict:** PASS on every gate. One medium-low item needs a decision before merge (finding F1). The J4 live gate passes.

## Gate summary

| # | Gate | Result | Evidence |
|---|------|--------|----------|
| 1 | `go test -race` on `./internal/api/handlers/...` and `./internal/crowdsec/...` | PASS | both `ok`; handlers 397 s, crowdsec 116 s |
| 2 | `scripts/go-test-coverage.sh` (gate 87%) | PASS | line 91.2%, statements 92.2% |
| 3 | `scripts/frontend-test-coverage.sh` | PASS | lines 91.94%, statements 90.89%, branches 84.62%, functions 88.76% (gate is lines) |
| 4 | `scripts/local-patch-report.sh` | PASS | overall 96.7% (29/30), backend 100% (7/7), frontend 95.7% (22/23); both artifacts written. One uncovered changed line: F2 |
| 5a | `golangci-lint --config backend/.golangci.yml` on `internal/crowdsec/...` and `internal/api/handlers/...` | PASS | 0 issues |
| 5b | eslint on the 10 touched frontend `.ts`/`.tsx` files | PASS | 0 errors, 59 warnings (F3) |
| 5c | `npm run type-check` | PASS | exit 0 |
| 5d | `npm run build` | PASS | built in 1.5 s |
| 5e | `go build ./...` | PASS | exit 0 |
| 6 | Targeted Playwright, `--project=security-tests`, 6 specs | PASS | 65 passed, 0 failed (2.7 min). Specs: `security/crowdsec-security-toggle-busy`, `security/crowdsec-first-enable`, `crowdsec-whitelist`, `security/crowdsec-background-install`, `security/crowdsec-hub-preset-apply`, `security/crowdsec-hub-items`. `charon-e2e` rebuilt from the branch with `docker-rebuild-e2e --profile=security-tests` first. `changelog.json` not modified. |
| 7 | shellcheck on the two changed shell files | PASS | `scripts/crowdsec_data_persistence_test.sh` clean. `.docker/docker-entrypoint.sh` has only SC2012 (info, line 323), which is pre-existing in the base. |
| 8 | `govulncheck ./...` | PASS | 0 vulnerabilities in called code. 1 vulnerability in a required module that the code does not call (not a gate failure). |
| 9 | i18n parity | PASS for the maintained locales | en, de, es, fr and zh contain every key the branch adds. de/es/fr/zh miss only the three `proxyHosts.source.*` keys, which predate the branch. The other 23 locales lack the branch's keys (and 34 older keys); they are missing 43 keys each; they fall back to en. This branch did not touch them, which matches the existing practice (info, not a failure). |
| J4 | Real install gate on a throwaway container | PASS | see the J4 section |

Not run locally, by policy: CodeQL and Trivy (`fix:`/`test:`-only change with no new feature surface, so CI runs them). GORM scan not triggered (no `backend/internal/models/**` change). Gotify token review: no token-like strings in the branch diff.

## J4 live gate

Throwaway containers only. The live `charon` container was not touched. Images, volumes, containers and secrets files were removed afterwards.

- **Run 1** (`charon-qa-j4`, `127.0.0.1:18080`, own volume, cold hub cache): job `succeeded` in 8 s with `items_installed` 523. Whitelist add during the job returned 409 `crowdsec_busy`. Console enrol returned 404 "console enrollment disabled", because the feature flag was off, so this run did not test the busy path for console enrol. Whitelist add after the job returned 201.
- **Run 2** (`charon-qa-j4b`, `127.0.0.1:18081`, fresh image, fresh volume, cold cache, `feature.crowdsec.console_enrollment=true`): one-second polling of `GET /api/v1/admin/crowdsec/presets/jobs/current`:

  | t (s) | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
  |---|---|---|---|---|---|---|---|---|---|
  | items_installed | 0 | 0 | 114 | 114 | 273 | 273 | 415 | 415 | 523 (`succeeded`) |

  The counter advances in steps during the install (previously flat at 1, then 3). It ends at 523, which equals the 522 linked items (1 parser, 1 context, 520 scenarios) plus the collection itself.
- **Busy refusals during the job (run 2):** whitelist add, console enrol, hub-item restore and `POST /admin/crowdsec/start` all returned 409 `crowdsec_busy`.
- **Setting (run 2):** `security.crowdsec.enabled` was unset before, during the refused start, and after the job. The refused start wrote nothing. Caveat: the throwaway had no prior value, so this shows "no write", not "value preserved".
- **After the job:** whitelist add returned 201.
- **Limit:** this host's hub finished the install in 8 s, not the several minutes of the earlier run. The slow, throttled case was not reproduced, so the "steady advance over minutes" part of the gate is shown only over an 8 s window.

## Findings (ranked)

| Rank | Severity | Finding | Where | Status / action |
|------|----------|---------|-------|-----------------|
| F1 | medium-low (to verify) | With the frontend pre-write and revert removed, a start that the backend accepts but that the post-start status check then reports as not running throws, and the backend has already persisted `security.crowdsec.enabled=true`. Nothing reverts it. Before this branch the frontend reverted it. | `frontend/src/pages/Security.tsx:177-178` (throw), `backend/internal/api/handlers/crowdsec_handler.go:661-662` (persist before start), `:686-687` (revert only when `Executor.Start` itself fails) | Not reproduced. The window is narrow (the process exits between the backend's 200 and the status poll). Suggested in-PR fix: on the verify failure, have the backend confirm the process before it persists `true`, or restore `false` through the same settings route. Add a unit test for the verify-failure path. |
| F2 | low | One uncovered changed line: the fallback success toast for a start response that is neither `lapi_ready` nor `enabled`. | `frontend/src/pages/Security.tsx:225` | Add a test case for that result shape, or accept (patch coverage 92.3%). |
| F3 | low | 59 eslint warnings in the touched frontend files: `import-x/order` in `useCrowdSecWhitelist.ts`, `testing-library/prefer-find-by` and `no-unnecessary-act` in the new tests. No errors. | `frontend/src/hooks/useCrowdSecWhitelist.ts`, `frontend/src/pages/__tests__/Security.crowdsec.test.tsx` and other touched tests | Style only. Fix in-PR or in a chore. |
| F4 | low | The root E2E TypeScript (`tests/**`) has no working ESLint config. Running eslint on those files fails for the base files too. | repo root (no `eslint.config.*`), `lefthook.yml` | Filed as **#1716** (non-security). |
| F5 | info | Frontend branch coverage is 84.62%, below 85%. The gate checks lines (91.94%), so it passes. | `scripts/frontend-test-coverage.sh` output | Watch in CI. |
| F6 | info | The J4 install took 8 s on this host, so the multi-minute steady-advance case is not covered by this run. | J4 section | Re-run with throttling if the user wants it. |
| F7 | info | Shellcheck SC2012 (use `find`) at `.docker/docker-entrypoint.sh:323`. Pre-existing. | `.docker/docker-entrypoint.sh:323` | No action. |

Existing issues this branch addresses, to close when the PR merges: **#1711** (toggle desync; covered by the Security toggle unit tests and `crowdsec-security-toggle-busy.spec.ts`), **#1712** (`.yml` items; J4 gate passes, `hubitems_test.go` and handler tests cover it), **#1713** (whitelist add fallback; covered by `useCrowdSecWhitelist.test.ts`).

## Security

No new security-sensitive finding. F1 is a correctness regression risk, not a vulnerability. Nothing was filed publicly that would need a private advisory.

## Issues filed

- **#1716** `chore: lint the root E2E TypeScript (tests/) with ESLint` (F4, low, non-security).

## Notes on the working tree

- No commits, pushes or staging were made. The only tracked file this QA changed is `docs/reports/qa_report.md`.
- `go.work.sum` is unchanged. The `charon-e2e` container was rebuilt from this branch as part of gate 6. The `charon` container was not touched.
