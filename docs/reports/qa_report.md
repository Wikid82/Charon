# QA Report: chore/lint-sweep (GH #1743, #1716, #1820)

Date: 2026-11-05. Branch `chore/lint-sweep`, 17 commits ahead of `development`
(309 files changed, +4905 / -2266). No Go code changed. Nothing was fixed or committed by QA.

## Gate results

| # | Gate | Result | Detail |
|---|------|--------|--------|
| 1a | `frontend: npm run lint` | PASS | exit 0, 0 problems (`--max-warnings 0`) |
| 1b | `frontend: npm run lint:locales` | PASS | exit 0 (23 `i18n-json/identical-keys` warnings, 0 errors) |
| 1c | `frontend: npm run type-check` | PASS | exit 0 |
| 1d | `frontend: npm run build` | PASS | built in 4.01s |
| 2a | root `npm run lint:e2e` | PASS | exit 0, 124 warnings, 0 errors (limit 124, at the limit) |
| 2b | `npx tsc -p tests/tsconfig.json --noEmit` | PASS | exit 0 |
| 3 | `scripts/frontend-test-coverage.sh` | PASS | 327 files, 4282 passed, 2 todo, 0 failed. Statements 91.53%, Branches 86.14% (6578/7636), Functions 88.88%, Lines 92.43%. Script gate: PASS (lines vs 87%). Branches above the 85% floor |
| 4a | `make -n lint-backend lint-agent` | PASS | both show `--timeout=25m` |
| 4b | `go build ./...` | PASS | |
| 4c | `go test ./internal/api/handlers -run TestRealCommandExecutor_Execute_OutputLimit -count=1` | PASS | ok |
| 5 | markdownlint CONTRIBUTING.md, docs/plans/current_spec.md | PASS | 0 issues |
| 6 | `actionlint .github/workflows/quality-checks.yml` | PASS | exit 0 |
| 7a | Playwright emergency-token.spec.ts | PASS | 10/10 |
| 7b | Playwright security-state-restore.spec.ts | PASS | 3/3 (includes setup and shard-setup) |
| 7c | Playwright core/navigation.spec.ts | PASS | 28/28 (firefox) |

Note on 7a/7b: these two specs are excluded from the `firefox` project by `playwright.config.js` (`testIgnore`
for `security-enforcement` and `tests/security`); `--project=firefox` reports "No tests found". They only run
under the `security-tests` project, so that single project was used. All ran against the existing e2e container at
http://localhost:8080 (older build, not rebuilt). No failures, so staleness did not matter.

Not run (per instructions): `lefthook run pre-commit`, CodeQL/Trivy (no `feat:` surface), GORM scan (no model changes).

## Review

Commit prefixes are sensible: `chore` for lint config/CI/deps ordering, `refactor(frontend)` for effect and
derived-state rewrites, `fix` for the label and overlay accessibility commits, `test(frontend)` for test hygiene,
`docs` for the plan and policy. No `(security)` scope used, none needed. No `!`/BREAKING markers.

Attribution: grep over all commit bodies since `development` for `claude.ai`, `session_`, `co-authored`,
`generated with`, `claude code` returned nothing. CONTRIBUTING.md and docs/plans/current_spec.md also clean.

Production files spot-checked against `development` (behavior review):

- CrowdSecConfig.tsx: autoFocus replaced by ref + effect focus (same intent). Overlay `div role=button` became a real `button`. `Date.now()` replaced by `useNow(1000, isStarting)` for the 90s startup cap (ticks only while starting). Preset selection reconcile and reset moved from effects into adjust-during-render, keyed on catalog / selected key. The pull mutation effect still fires on `selectedPresetKey`. Equivalent.
- CertificateList.tsx: selection now derived with `useMemo` from `pickedIds` filtered by current ids (replaces prune effect). `SortIcon` hoisted to module scope, with added `data-testid`s. Equivalent.
- UsersPage.tsx: preview-url clearing moved to render, host toggle deduplicated (`createHostToggle`), `useSeedOnChange` replaces seed effects, `aria-label` added on host checkboxes. Equivalent.
- Notifications.tsx: `watch()` replaced by `useWatch` (render) and `getValues()` (handlers). Transient test/preview state reset moved to render keyed on `initialData`. Ternary refactors only. Equivalent.
- ThemeContext.tsx: missing `user:*` theme now yields derived `effectiveTheme = 'dark'`, and the `setThemeState('dark')` call was removed. Observation (low): the stored `theme` state stays `user:<id>` after fallback; consumers see `dark` via `effectiveTheme`, localStorage is still set to `dark`. This only matters if the raw state is read elsewhere; `importTheme`/`exportTheme` use the effective value. Not a blocker.
- RestoreDialog.tsx: reset-in-effect replaced by remount via `key={backup?.filename ?? ''}`. Equivalent (passphrase and mutation state reset per backup).
- ProxyHostForm.tsx: `normalizeAccessListReference` and `normalizeSecurityHeaderReference` merged into `normalizeReferenceID` (verified bodies were identical in `development`). Forward-host warning is now derived per render instead of an effect. localStorage "don't ask" preference read via lazy initializer. Docker-unavailable fallback moved to adjust-during-render. `htmlFor`/`id` label wiring added with `useId`, one `eslint-disable set-state-in-effect` with justification. Equivalent.
- Also glanced at diff stats for SystemSettings, useBackups, RemoteTargetFormDialog, SecurityHeaderProfileForm: consistent with the stated refactors (largest remaining changes). No unexpected API or behavior changes found.

### npm audit (root)

Current branch: 13 vulnerabilities (10 moderate, 3 high). `development` lock: 6 (3 moderate, 3 high), computed from
`git show development:package-lock.json` in a scratch dir.

- Pre-existing (6): `minimatch` GHSA-g3hv-75wg-m249 (DoS, via `eslint` / `@eslint/config-array`), `re2js` GHSA-gg6h-894w-4m9c (via `markdownlint` / `markdownlint-cli2`). The 3 high are the re2js chain.
- New from this PR (7, all moderate, all dev-only): `@eslint/js`, `typescript-eslint`, `@typescript-eslint/{eslint-plugin,parser,type-utils,typescript-estree,utils}`. They are flagged only transitively through the same `minimatch` advisory (via `@typescript-eslint/typescript-estree`). No new advisory, no new high.
- `npm audit --omit=dev`: 0 vulnerabilities, so nothing ships to runtime or the container.
- Lockfile delta: 19 packages added, 0 removed, 0 version changes. `package.json` adds `@eslint/js`, `eslint-plugin-playwright`, `typescript-eslint`, and the `lint:e2e` script only.

## Blockers

None. All gates PASS.

## Non-blocking notes

- `lint:e2e` is exactly at its warning ceiling (124 of 124); any new `waitForTimeout` in tests will fail the gate, which is the intended ratchet.
- Branch coverage margin over the 85% floor is 1.14 points (86.14%).
- Local e2e container is an older build; CI should confirm full-suite and cross-browser health.
