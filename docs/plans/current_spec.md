# Lint Sweep: Frontend Backlog (#1743) and Root E2E Linting (#1716, #1820)

Status: DRAFT for supervisor review. Base: `development` @ 77fbbb656. Date: 2026-11-05.
Branch: `chore/lint-sweep-1743` (one PR into `development`, ordered commits). No worktrees.
Scope owner agents: `frontend-dev` (src), `playwright-dev` (root `tests/` findings), `devops` (Makefile, lefthook, CI).

## 1. Introduction

### Goals

1. Take `cd frontend && npm run lint` from 1350 warnings to 0 outside the locale-sync category, fixing real
   a11y, security-linter and React-correctness findings instead of waiving them (#1743).
2. Give the root Playwright `tests/` tree a working ESLint step in lefthook and CI (#1716, #1820).
3. Fix the Makefile `lint-backend` timeout (cold-cache exit 4) (#1820).
4. Make new warnings fail CI so the backlog cannot regrow (`--max-warnings` ratchet).

### Non-goals

- No product behavior changes. The only behavior-adjacent edits are the React-compiler rule fixes
  (Commit 9 to 11), which are isolated and individually tested.
- No locale content edits (Crowdin owns `frontend/src/locales/**`).
- No Go code changes besides the Makefile.
- No new E2E specs: nothing user-visible is added, so the "E2E as `test.fixme`" step from the suggested
  commit sequence is intentionally omitted. Existing component vitest files are the safety net; targeted
  Playwright runs are only required for the a11y/hook commits that change rendered DOM (see gates).

## 2. Research Findings

Measured 2026-11-05 (`npm run lint`, JSON output): 0 errors, 1350 warnings, 219 auto-fixable, in 247 of 672 linted files (247 files carry at least one finding)
(125 `import-x/order`, 42 `unicorn/prefer-ternary`, 21 `testing-library/prefer-find-by`, 16
`unicorn/no-useless-undefined`, 12 `import-x/no-duplicates`, 1 each `sonarjs/prefer-immediate-return`,
`testing-library/no-wait-for-multiple-assertions`, `unicorn/prefer-includes`).

Where the warnings live: ~1,000 of 1,350 are in test files (`src/pages/__tests__` 457,
`src/components/__tests__` 315, `src/hooks/__tests__` 27, others small). Production files with the most:
`components/dialogs` 81 (mostly `__tests__`), `components/hecate` 49, `components/theme` 48,
`ProxyHostForm.tsx` 22, `ProxyHosts.tsx` 9, `Notifications.tsx` 8, `useProxyGroupDnD.ts` 8.

Existing config facts (`frontend/eslint.config.js`):

- Every react-hooks recommended-latest rule is forced to `warn` (the React Compiler diagnostics:
  `set-state-in-effect`, `static-components`, `immutability`, `purity`, `globals`, `incompatible-library`).
- Test override block sets six `testing-library/*` rules and `vitest/expect-expect` to `warn`.
- Locale block sets `i18n-json/identical-keys` to `warn` on purpose ("Missing keys are expected until
  Crowdin catches up"); placeholders and JSON validity are `error`.
- `npm run lint` is `eslint . --report-unused-disable-directives` (no `--max-warnings`).
- CI: `quality-checks.yml` job `frontend-quality`, step "Run frontend lint" (line ~694) has
  `continue-on-error: true`, so lint does not gate today.
- Lefthook: `frontend-lint` runs `cd frontend && npm run lint` (whole project, not staged files) for any
  `frontend/**/*.{ts,tsx,js,jsx}` change. With `--max-warnings 0` this blocks any frontend commit if the
  tree has a stray warning (intended, but see Risks).
- Vitest coverage gate: `lines` threshold 87 (`CHARON_MIN_COVERAGE`); lead reports branch coverage 85.6%
  vs the 85% floor, so test rewrites must be behavior-preserving.

Root tree: no root `eslint.config.*`, `tests/` has 192 `.ts`/`.js` files, no `tests/tsconfig.json`,
root `package.json` already lists `eslint ^10.14.0` and `@playwright/test` but not `typescript-eslint`,
`@eslint/js` or `eslint-plugin-playwright`. Root `typescript` is aliased to `@typescript/typescript6@^6`.
A scratch run (typescript-eslint `recommended` only, config in the scratchpad, nothing committed) over
`tests/**/*.{ts,js}` produced 77 findings: 45 `no-explicit-any`, 30 `no-unused-vars`, 2 `prefer-const`
(plus 1 file that yielded a null-rule parse/config message to triage). `eslint-plugin-playwright` findings
are unmeasured (plugin not installed); Commit 12 must measure them first.

Open PRs: none (`gh pr list --state open` is empty). Remote branches ahead of `development` that touch
affected paths: `origin/l10n_development` (Crowdin, 28 locale files; we do not edit locales, so no conflict)
and `origin/chore/release-please-migration` (1 file in the watched set, stale since 2026-08-17).
Every other remote branch is already merged or identical to development for these paths.

## 3. Priorities & Ordering

Findings and what happens first:

| # | Item | Class | Priority | Handling |
| --- | ------ | ------- | ---------- | ---------- |
| 1 | `security/detect-unsafe-regex` (3), `detect-non-literal-regexp` (4) | security linter | High within this PR | Fixed first among manual work (Commit 6), per #1743's own advice |
| 2 | a11y: label association 45, interactions 27, autofocus 3, heading 2, aria props 2 | real user-facing defects | High | Fixed (Commit 7 and 8), not waived |
| 3 | Makefile `--timeout=25m` | tooling bug (reproducible exit 4) | Medium, trivial | Commit 1, independent |
| 4 | Root `tests/` has no lint | tooling gap, bug-fix class (#1716/#1820 are user-filed) | Medium | Commit 12 to 13 |
| 5 | react-hooks compiler rules (53 total) | correctness, behavior-affecting | Medium, riskiest | Fixed last, one cluster per commit with tests (Commit 9 to 11) |
| 6 | Mechanical style (import order, ternary, undefined, type imports) | style | Low | Auto-fix commits first (Commit 2 to 4) |
| 7 | Test-hygiene rules (testing-library, vitest) | test quality | Low | Commit 5 plus the Decision 1 outcome |
| 8 | 23 `identical-keys` | external sync (Crowdin) | Not a defect | Config split, Commit 14, no locale edits |

Rule applied: this is our own tooling/quality work (the three issues are filed chores, not user-reported
product bugs), and there is no new `feat`. There are no critical/high out-of-scope findings. No security
advisory is implied: the regex findings are lint-level (validated input in `cron.ts`/`validation.ts`;
Commit 6 measures them with an adversarial-input test before deciding fix vs. proof-of-safety). If that
measurement shows a real ReDoS on user-controlled input, stop, file a private advisory per `SECURITY.md`
and ship the fix as its own PR with a vague `fix(security):` subject.

Out-of-scope findings discovered during the sweep: file a GitHub issue immediately (CLAUDE.md triage).
Already known and to be filed if the user picks the matching option: the `no-node-access` residual
(Decision 1, option B).

## 4. Decisions and Recommendations

### Decision 1: per-rule disposition

| Rule (count) | Disposition | Rationale and evidence |
| --- | --- | --- |
| `import-x/order` (148), `import-x/no-duplicates` (24) | Auto-fix (125 + 12 fixable), hand-fix the remaining ~35 | Pure formatting; the unfixable ones are mostly type-import grouping |
| `consistent-type-imports` (13) | Hand-fix (not auto-fixable here) | Mechanical |
| `unicorn/prefer-ternary` (48), `no-useless-undefined` (16), `prefer-includes` (1), `sonarjs/prefer-immediate-return` (1) | Auto-fix, review diff | Touches production code, so vitest `related` must run |
| `testing-library/prefer-find-by` (115) | Auto-fix 21, hand-fix ~94 | Real test hygiene: `await waitFor(() => getBy...)` to `await findBy...`; same semantics |
| `testing-library/no-unnecessary-act` (18), `render-result-naming-convention` (15), `no-manual-cleanup` (2), `no-wait-for-multiple-assertions` (1), `vitest/expect-expect` (8) | Hand-fix | Small. `expect-expect`: add the missing assertion, or register the helper via `assertFunctionNames` if it wraps `expect` |
| `testing-library/no-node-access` (734 in 84 files, with `no-container` 44) | **Recommended: option B, scope down for tests, with a tracking issue.** See below | |
| `jsx-a11y/label-has-associated-control` (45) | Fix | Real defect: e.g. `ProxyHostForm.tsx:767` is a `<label>` with no `htmlFor`/nesting, so screen readers and `getByLabelText` cannot associate it. Fix with `htmlFor` + `id` (use `useId`) |
| `no-static-element-interactions` (14), `click-events-have-key-events` (13) | Fix | e.g. `CertificateCleanupDialog.tsx:26-30`: modal backdrop `onClick={onCancel}` and an inner `stopPropagation` div. Fix by giving the overlay `role="presentation"`, the panel `role="dialog" aria-modal`, and Escape handling (check whether the repo already has a shared dialog primitive and reuse it, DRY) |
| `jsx-a11y/no-autofocus` (3) | Fix | All in `CrowdSecConfig.tsx` modals (e.g. `:1756` ban-IP input). Replace the attribute with ref-based focus on open (`useEffect` + `ref.focus()`), preserving the focus-on-open UX |
| `jsx-a11y/heading-has-content` (2), `role-has-required-aria-props` (2) | Fix | Real a11y |
| `security/detect-unsafe-regex` (3) | Keep patterns, prove linear, reasoned inline disable (the one explicit exception to "no waivers") | `utils/cron.ts:29`, `utils/validation.ts:10`, test `crowdsecPresets.test.ts:19`. Add a long-adversarial-input test; if linear time, keep the pattern with a justified inline disable (preferred removal: rewrite with split/Number parsing, for `isIPv4` and cron fields) |
| `security/detect-non-literal-regexp` (4) | Fix | All in tests/setup: escape input with a shared `escapeRegExp` helper (or use string matchers); 1 in `src/test/setup.ts` |
| `react-hooks/set-state-in-effect` (37 findings in 27 files; only 4 in `src/hooks`: `useBackups` x3, `useMediaQuery` x1) | Fix (no waivers planned; `useMediaQuery` uses `useSyncExternalStore`) | Examples: `CertificateList.tsx:41` prunes `selectedIds` in an effect when `certificates` changes (derive instead); form components reset state from props (use `key` or derive during render). Each cluster gets tests. Last-resort inline disable only for a genuine external-system sync, listed in the PR description with a reason |
| `react-hooks/static-components` (3), `immutability` (3), `purity` (2), `globals` (1, in test file `AuthContext.test.tsx:125`), `incompatible-library` (2), `exhaustive-deps` (1) | Fix | `CertificateList.tsx:185` component created in render (`SortIcon`), `CrowdSecConfig.tsx:128` and `ManualDNSChallenge.tsx:246` use `Date.now()` in render. For `incompatible-library` (TanStack/RHF style APIs), a one-line disable with reason is acceptable only if the library is the cause |
| `promise/always-return` (2), `sonarjs/no-identical-functions` (2) | Fix | Trivial |
| `i18n-json/identical-keys` (23) | See Decision 2 | |

**`no-node-access` evidence.** All 734 hits share one message. Samples:

- `AccessListForm.test.tsx:102` `getAllByRole('button').find(b => b.querySelector('.lucide-x'))`: finding an
  icon-only button by its SVG class. The correct fix is to give the button an accessible name
  (`aria-label`), which is itself an a11y improvement, then `getByRole('button', {name})`. That edit touches
  product code.
- `ProxyHosts-coverage*.test.tsx` (54 hits in one file) are "coverage" tests that assert on DOM structure
  and classes, not user behavior.
- A hand rewrite of 734 sites across 84 files is realistically a multi-day, high-churn change whose payoff
  is test style, with a real risk of weakening assertions and lowering branch coverage (85.6% vs 85%).

Options:

- A. Rewrite all 734 (plus 44 `no-container`) by hand over about 10 test-only commits. Cleanest end state,
  highest risk and review cost, may also require product `aria-label` additions.
- **B (recommended).** Turn `no-node-access` and `no-container` to `off` in the test override, with a
  comment stating the rationale, and file one tracking issue listing the 84 files and the preferred
  replacement patterns (accessible names, `within`, `getByLabelText`). Fix opportunistically any such hit
  in a file that another commit in this PR already rewrites, then re-measure. Rationale: 84 files of
  legacy structural tests are low-value to churn in a lint PR; the rule remains valuable for new tests, so
  scope it with a path-based override (`warn` for everything outside a named legacy glob list generated
  from the 84 files, `off` for those) rather than a global `off`. This keeps the signal for new test files
  and lets the legacy list only shrink.
- C. Leave at `warn` and absorb in the ratchet baseline. Rejected: it makes `--max-warnings 0` impossible.

Decision (approved): option B. File the tracking issue at the start of the PR (Commit 1 time); generate the
legacy glob list from the lint JSON rather than by hand.

### Decision 2: the 23 `i18n-json/identical-keys` findings

Investigated. They are not typos or stray keys. `en/translation.json` has 2215 leaf keys; 23 of the 27
other locales have 2143 (exactly 72 keys behind, identical gap in every one), and `de`, `es`, `fr`, `zh`
are complete. A key diff of `en` vs `af` shows the 72 are all present in `en` only (CrowdSec hub install
progress, `wafConfig.rulesSource`, `security.enforcement.*`, `crowdsec.errors.*`, etc.): new English
strings added by recent features that Crowdin has not translated yet. There are no extra keys in any
locale. i18next falls back to `en`, so this is intended behavior and the config comment already says so.

Recommendation: do not touch locale files (Crowdin would see copied English as "translated"; the open
`l10n_development` branch edits these files). Split locale linting from code linting:

- `lint` script: `eslint . --ignore-pattern 'src/locales/**' --max-warnings 0 --report-unused-disable-directives`.
- New `lint:locales`: `eslint src/locales` (JSON validity and placeholder parity stay `error` and blocking;
  `identical-keys` stays `warn` and informational, never gating).
- CI and lefthook run both; only `lint:locales` errors fail. The finding count is one per locale file, so it
  does not grow as more English keys are added, and it falls to 0 as Crowdin catches up.

### Decision 3: root `tests/` linting

- New standalone root `eslint.config.js` (ESM; root `package.json` has `"type": "module"`), scoped to
  `tests/**/*.{ts,js}`. Do not extend `frontend/eslint.config.js`: it is React, testing-library, vitest,
  CSS/JSON/markdown oriented, resolves plugins from `frontend/node_modules`, and uses
  `import.meta.dirname` paths for locales. Sharing it would couple unrelated toolchains.
- Plugins (new root devDependencies, exact versions pinned by `frontend-dev`/`devops` against what
  `frontend/package.json` already uses for `@eslint/js` and `typescript-eslint`): `@eslint/js`,
  `typescript-eslint`, `eslint-plugin-playwright` (flat `recommended`).
- Typed linting: add `tests/tsconfig.json` (noEmit, includes `tests/**/*`, Playwright types) and enable
  `projectService` so `@typescript-eslint/no-floating-promises` and `await-thenable` run. Missing `await` on
  Playwright calls is the highest-value bug class in E2E code. Typed linting is the decision (the `typescript-eslint` peer range covers TypeScript 6.0.x and
  the frontend already uses the same alias, so no peer-conflict contingency is planned). `tests/example.spec.js`
  gets `disableTypeChecked` (or `allowJs`), and root `playwright.config.js` sits outside `tests/` and is
  linted untyped or ignored explicitly.
- Severity policy: correctness rules are `error` (`missing-playwright-await`, `no-focused-test`,
  `valid-expect`, `no-floating-promises`, `no-unused-vars`, `prefer-const`); style-ish and noisy rules are
  `warn` first (`no-explicit-any`, `playwright/no-conditional-in-test`, `no-wait-for-timeout`,
  `no-force-option`, `no-skipped-test`, `expect-expect`). Baseline after the first run drives which warn
  rules get fixed vs. downgraded; target is 0 errors and a stable warning budget.
- Findings treatment: fix `unused-vars`, `prefer-const`, `any` where a real type is available (replace with
  `unknown` + narrowing or the Playwright types); anything requiring Playwright/test-semantics judgement
  goes to `playwright-dev`. Do not skip or delete tests; anything un-fixable gets a per-line disable with a
  reason. Code bugs found in specs are reported back, not fixed by `playwright-dev`.
- Wiring: root script `"lint:e2e": "eslint tests --max-warnings <N>"`; lefthook step `e2e-lint`
  (`glob: "tests/**/*.{ts,js}"`, runs `npx eslint {staged_files}` from repo root); new CI job `e2e-lint` in
  `quality-checks.yml` (path-filtered to `tests/**`, `eslint.config.js`, root `package.json`/lockfile),
  root `npm ci --ignore-scripts`, `npm run lint:e2e`. Blocking from day one (note: a path-filtered job cannot be marked a required status check without an always-run wrapper job; if branch protection should require it, add that wrapper) (the same commit fixes the
  findings), unlike frontend's historical `continue-on-error`.
- Housekeeping: `.dockerignore` already excludes `tests/`; add `eslint.config.js` at root to
  `.dockerignore` if it is not covered; verify `.gitignore` and `.codecov.yml` need no change.

### Decision 4: `--max-warnings` ratchet

- Frontend `lint` ships with `--max-warnings 0` in the last code commit, and the CI step loses
  `continue-on-error: true` in the same commit. Because everything else in the PR drives code warnings to
  0, no residual baseline number is needed. If Decision 1 option B is chosen, the path-scoped `warn` for
  new test files provides the forward signal and the legacy list is `off`, so the count still reaches 0.
- Avoiding blocking other work: there are no open PRs now; nothing else needs rebasing when this lands.
  Any branch cut before merge that adds warnings will fail lint after rebase, which is the intent; the PR
  description tells contributors to run `npm run lint -- --fix` after rebasing. If the sweep ends with a
  non-zero residual (e.g. a deferred cluster), set the ratchet to that exact count, not 0, and file the
  follow-up issue; never merge with the ratchet below the actual count.
- The two-state ratchet (frontend and root `lint:e2e`) is stored in `package.json` scripts, so tightening is
  a one-line change reviewed in a PR.

### Decision 5: Makefile timeout

`lint-backend` (Makefile line 199) uses `--timeout=5m`; change to `--timeout=25m` per #1820 evidence for the
pinned image on a cold cache. Apply the same to `lint-agent` (line 204), which has the identical failure mode,
(DRY: define a `GOLANGCI_TIMEOUT ?= 25m` variable used by both). CI already uses 10m in the action
(agent job `quality-checks.yml:587-588`, `--timeout=10m`) with a warm cache; leave it, and note in the commit body that CI is unaffected.
Validation: `make -n lint-backend lint-agent` shows the new flag; actual cold-cache run is optional and
long, run it foreground if executed.

## 5. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Frontend branch coverage 85.6% vs 85% floor; test rewrites could drop it | Test rewrites are mechanical (`findBy`, `act` removal, renames); each commit's gate runs only the touched vitest files, and `qa-security` runs the full coverage script once at the end. Any commit that deletes assertions is rejected in review. Do not remove tests or `.skip` anything |
| Sweep touches ~250 files; review fatigue | Slice by rule group and directory (Section 7), mostly mechanical commits with the exact `eslint --fix` command recorded in the commit body so a reviewer can reproduce the diff |
| `--fix` changes semantics (`prefer-ternary` on side-effecting branches, `no-useless-undefined` on explicit `undefined` args) | Review diffs; `vitest related` on changed source files; `tsc --noEmit` |
| React-compiler fixes alter render timing | Isolated commits, one component cluster each, with new/extended tests asserting the previous behavior (selection pruning, form reset on prop change, countdown/starting-cache timing) and targeted Playwright runs for `ProxyHostForm`, `CertificateList`, `UsersPage` flows |
| Lefthook `frontend-lint` lints the whole tree, so a single stray warning blocks every frontend commit once `--max-warnings 0` lands | Intended post-sweep; release note in PR; Renovate `deps:` PRs touching frontend run the same check in CI |
| `eslint --fix` import reorder fights `prettier` or editorconfig | Run `npx prettier --check` on touched files if the repo gates on it; otherwise none |
| Merge conflicts | None open. `l10n_development` only edits locales, which this PR does not touch. Keep the PR short-lived and rebase on `development` before each remaining commit |
| Autofocus fix regresses modal focus UX | Test that the input is focused after open (RTL `toHaveFocus`) and a targeted Playwright run on the CrowdSec ban modal if a spec exists |

## 6. Technical Specifications (per area)

No API, DB or schema changes. File-level contract changes:

- `frontend/eslint.config.js`: test override gains `assertFunctionNames` if needed; Decision 1 option B
  adds a legacy-glob override; locales block unchanged (`warn` for identical-keys); `react-hooks` map stays
  `warn` until Commit 11 flips the cleared rules to `error` so they cannot regress.
- `frontend/package.json`: `lint`, `lint:locales` scripts (Decision 2 and 4).
- Root: `eslint.config.js`, `tests/tsconfig.json`, `package.json` (`lint:e2e` + devDeps) and lockfile,
  `lefthook.yml` (`e2e-lint`, `frontend-lint` also runs `lint:locales`), `.github/workflows/quality-checks.yml`
  (new `e2e-lint` job; frontend lint blocking), `Makefile`.
- Possible new shared test helper `frontend/src/test/escapeRegExp.ts` (Commit 6) and, if no shared dialog
  primitive exists, a small `useDialogA11y` style helper (decided by `frontend-dev` after a code search;
  reuse before adding, per DRY).
- Docs: `CONTRIBUTING`/`docs/` lint section if one exists (search `docs/` for "npm run lint"); no
  `ARCHITECTURE.md` change expected unless the directory/CI structure statement mentions lint steps.

## 7. Implementation Plan: Commit Slicing Strategy

Decision: single PR (`chore/lint-sweep-1743` into `development`), ordered logical commits. Commit prefixes
(approved): `fix:` for Commits 7 and 8a (real a11y defects; and Commit 6 only if a pattern changes behavior);
`refactor:` for 3, 9, 10a, 10b, 11; `test(frontend):` for 4 and 5; `chore:` for the Makefile, root lint
config, CI/lefthook and docs. None qualify for `(security)`. No session ID or attribution lines in commit
messages or PR body.

Dev-agent rules for every commit: run commands foreground and blocking; run only scoped tests. Gate
pattern (unless stated): `cd frontend && npx eslint <touched paths> --max-warnings 0 --rule` check that the
targeted rule count in those paths is 0; `npm run type-check`; `npx vitest run --changed` or
`npx vitest related <touched src files>` (explicit file list); no full suite, no coverage script. The
orchestrator commits. Record the before/after count per rule in each commit body.

Phase 1 (E2E specs as fixme): intentionally empty, see Non-goals.

| # | Commit | Scope and files | Depends on | Validation gate |
| --- | -------- | ----------------- | ------------ | ----------------- |
| 1 | `chore: raise golangci-lint timeout for local lint targets` | `Makefile` only (`GOLANGCI_TIMEOUT ?= 25m` used by `lint-backend` line 199 and `lint-agent` line 204). **At this point the orchestrator also files the `no-node-access`/`no-container` legacy tracking issue** (see Decision 1) with the file list generated from the lint JSON | none | `make -n lint-backend lint-agent` shows `--timeout=25m`; no Go changes; issue URL recorded in the PR body |
| 2 | `chore(frontend): auto-fix import order and duplicates` | `eslint . --fix` limited to `import-x/order` (148; 70 in non-test source, 78 in tests), `import-x/no-duplicates` (24; 22 in non-test source); hand-fix the residual and the 13 `consistent-type-imports`. Split 2a non-test source / 2b `**/__tests__` if the diff exceeds ~80 files | 1 | those 3 rules at 0 repo-wide; `npm run type-check`; `npm run build`; `vitest related` over touched non-test sources |
| 3 | `refactor(frontend): apply unicorn and sonar auto-fixes` | `prefer-ternary` 48, `no-useless-undefined` 16, `prefer-includes`, `prefer-immediate-return`. Mostly `Notifications.tsx`, `UsersPage.tsx`, `Uptime.tsx`, `ProxyHostForm.tsx`, `CertificateList.tsx`, `useImport.test.tsx`. **Adds tests for any new branch an auto-fix introduces** (e.g. both arms of a converted ternary) | 2 | rules at 0; `npm run type-check`; `vitest related` on each touched source file (explicit list); diff reviewed for side-effecting branches |
| 4 | `test(frontend): migrate waitFor+getBy to findBy` | `prefer-find-by` 115 (21 auto-fix, rest hand). Split by directory: 4a `pages/__tests__` (Security*, SecurityHeaders ... ~65 hits), 4b `components/**/__tests__` and `hooks/__tests__` | 2 | rule at 0 in scope; edited specs green (explicit list); **`expect(` count before vs. after the diff is equal or higher** (record both in the commit body) |
| 5 | `test(frontend): fix remaining testing-library and vitest hygiene` | `no-unnecessary-act` 18, `render-result-naming-convention` 15, `no-manual-cleanup` 2, `no-wait-for-multiple-assertions` 1, `vitest/expect-expect` 8 | 4 | rules at 0; edited specs green; same `expect(` count diff check as Commit 4 |
| 6 | `chore(frontend): resolve regex lint findings` (becomes `fix:` only if a pattern's behavior actually changes) | `utils/cron.ts:29`, `utils/validation.ts:10` (`isIPv4`): measured as linear-time (no ReDoS), so **keep the patterns**, add an adversarial long-input test for each (bounded runtime assertion) and a reasoned inline `eslint-disable-next-line security/detect-unsafe-regex` comment. This is the one explicit exception to "no waivers" in this plan. `crowdsecPresets.test.ts:19`: same treatment or a literal-safe rewrite. The 4 `detect-non-literal-regexp` test/setup sites are fixed (shared `src/test/escapeRegExp.ts` or string matchers), not waived | none | rules at 0 (or only the reasoned disables); `vitest run src/utils src/data` plus edited specs; if any measurement unexpectedly shows super-linear behavior, STOP per Section 3 |
| 7 | `fix: associate form labels with their controls` | `label-has-associated-control` 45 (`AuditLogs.tsx` 15, `SecurityHeaderProfileForm.tsx` 15, `ProxyHostForm.tsx` 11, `UsersPage.tsx` 2, `PermissionsPolicyBuilder.tsx`); `htmlFor` + `useId`. Add or update `getByLabelText` assertions in their specs | none | rule at 0; those components' specs green (explicit list); targeted `npx playwright test <specs for proxy hosts / audit logs / security headers> --project=firefox` |
| 8a | `fix: make clickable overlays keyboard and screen-reader accessible` | Minimal per-overlay change: `role` + Escape + keyboard handlers for `no-static-element-interactions` 14, `click-events-have-key-events` 13, `no-autofocus` 3 (ref-based focus), `heading-has-content` 2, `role-has-required-aria-props` 2: `CertificateCleanupDialog`, `UsersPage`, `CrowdSecConfig`, `WafConfig`, `Uptime`, `ProxyHostForm`, others. **Preserve the CrowdSec modal's three-layer pointer-events structure and its existing Escape / Ctrl+Enter handling** (do not restructure). Tests cover each new branch: Escape closes, Enter/Space activates, `toHaveFocus` on open, Ctrl+Enter still submits | 7 | rules at 0; edited components' specs green; targeted Playwright (firefox) for the CrowdSec ban modal and certificate cleanup if specs exist. **`qa-security` runs `scripts/frontend-test-coverage.sh` after this commit** |
| 8b | (OUT OF THIS PR) migrate overlays to `src/components/ui/Dialog.tsx` | The shared `Dialog` primitive already exists; moving the ad-hoc overlays onto it is a refactor with wider blast radius. Default decision: not done here; **file a follow-up issue** listing the overlays migrated in 8a | n/a | n/a |
| 9 | `refactor(frontend): remove effect-driven state sync in hooks and theme context` | Re-derived from the lint JSON: `src/hooks/useBackups.ts` x3, `src/hooks/useMediaQuery.ts` x1 (4 in `src/hooks`) and `src/context/ThemeContext.tsx` x1. `useMediaQuery` moves to `useSyncExternalStore` (subscribe to `matchMedia`, server snapshot = false); a reasoned inline disable only if that is demonstrably wrong. `useBackups` and `ThemeContext`: derive or initialise lazily. New/extended hook tests for every new branch (initial value, change event, unmount cleanup) | none | rule at 0 in these files; `vitest run src/hooks src/context` plus the specs of consumers of `useMediaQuery`/`ThemeContext` (explicit list); **targeted firefox Playwright smoke** (theme toggle plus a responsive layout spec, since both are global). Only the 4 hooks files plus ThemeContext: not `useProxyGroupDnD` or `useStats*` (they have no `set-state-in-effect` findings) |
| 10a | `refactor(frontend): derive form state from props instead of effects` | The 37 `set-state-in-effect` findings span 27 files; hooks/context are in Commit 9. Forms/dialogs here: `ProxyHostForm.tsx` x4, `DNSProviderForm`, `RedirectionHostForm`, `RemoteServerForm`, `ProxyGroupForm`, `CredentialManager`, `CSPBuilder`, `PermissionsPolicyBuilder`, `CrowdSecKeyWarning`, `backups/RemoteTargetFormDialog`, `backups/RestoreDialog`, `hecate/AgentExternalProxyDialog`, `hecate/AgentWriteModeDialog`, `hecate/OrthrusInstallWizard`. Use `key` resets or derive during render. Tests for each new branch | 9 | rule at 0 in these files; each component's specs green; targeted Playwright (firefox) for proxy-host create/edit |
| 10b | `refactor(frontend): derive list and page state instead of syncing in effects` | `CertificateList.tsx` (**first add a test pinning the current `selectedIds` retain/prune behavior when `certificates` changes, then change it; prefer a derived-at-use filter over state** so the effect disappears), `crowdsec/AlertsList`, pages: `UsersPage` x4, `CrowdSecConfig` x2, `Logs` x2, `EncryptionManagement`, `RateLimiting`, `SMTPSettings`, `Setup`, `SystemSettings`. Tests for each new branch | 10a | rule at 0 repo-wide (`set-state-in-effect`); specs green; targeted Playwright (firefox) for certificate list and users flows |
| 11 | `refactor(frontend): resolve remaining React compiler diagnostics` | `static-components` 3 (`CertificateList` `SortIcon` x2, `SystemSettings`), `immutability` 3 (`CredentialManager`, `DNSProviderForm`, `ProxyHostForm`), `purity` 2 (`CrowdSecConfig.tsx:128`, `ManualDNSChallenge.tsx:246`), `incompatible-library` 2 (`Notifications.tsx`), `exhaustive-deps` 1 (`ProxyHostForm`), and `globals` 1, which is in the test file `context/__tests__/AuthContext.test.tsx:125`. Then in `eslint.config.js` flip the cleared compiler rules from `warn` to `error`. Tests for each new branch; timing specs use fake timers | 10b | all `react-hooks/*` at 0; affected specs green; `npm run type-check`; `npm run build`. **`qa-security` runs `scripts/frontend-test-coverage.sh` after this commit** (branch coverage must stay at or above the 85% floor) |
| 12 | `chore: add root ESLint config for Playwright tests` | root `eslint.config.js`, `tests/tsconfig.json`, root `package.json` + lockfile (devDeps, `lint:e2e`), measure and record the baseline (TS + Playwright rules) in the commit body; severities per Decision 3; set `--max-warnings` to the measured count. Handle `tests/example.spec.js` (apply `tseslint.configs.disableTypeChecked` to `*.js`, or enable `allowJs` in the tests tsconfig) and the root-level `playwright.config.js` (outside `tests/`: lint it with the untyped config or ignore it explicitly) | none | `npx eslint tests` exits 0 against the committed baseline from the repo root; `npx playwright test --list` still parses |
| 13 | `chore: clear root E2E lint findings` | `playwright-dev`: fix `no-unused-vars` 30, `prefer-const` 2, `any` 45 (types), Playwright-plugin findings, the one parse-message file; then lower `--max-warnings` to the new count (0 target). Anything needing logic judgement is reported back, not guessed | 12 | `npm run lint:e2e` green; a targeted run of up to three edited specs `--project=firefox` (only specs whose logic lines changed); no tests skipped or deleted |
| 14 | `chore: enforce lint in lefthook and CI` | `frontend/package.json` (`lint` with `--max-warnings 0` + `--ignore-pattern 'src/locales/**'`, `lint:locales`), `lefthook.yml` (`e2e-lint`, locales step), `quality-checks.yml` (frontend lint step blocking, locales step, new `e2e-lint` job), `.dockerignore` if needed | 2 to 13 | `cd frontend && npm run lint` = 0 warnings and `npm run lint:locales` = 0 errors; `npm run lint:e2e` green; `lefthook run pre-commit` on staged frontend and tests files (foreground) passes; `actionlint` if available |
| 15 | `docs: note lint policy` | brief note in the contributing/testing docs listing the three commands and the ratchet policy; PR description includes before/after counts. No `docs-site` edit unless a manifested page changes | 14 | markdownlint (`lefthook` runs it) |

Per-commit sizing note: Commits 2 and 4 are the largest (dozens of files, mechanical); 2a/2b and 4a/4b
split them if needed. All commits 2 to 11 are independent of each other except where "Depends on" says so,
so `frontend-dev` can parallelize 6, 7, 8 only when they touch disjoint files (7 and 8 both touch
`ProxyHostForm.tsx`, `UsersPage.tsx`, `CrowdSecConfig.tsx`: run sequentially). Do not run `qa-security` while
any dev agent is still editing the same files.

Rollback and contingency: every commit is independently revertable (`git revert <sha>`); mechanical commits
leave no schema or API traces. If Commit 10 or 11 exposes a behavior regression that cannot be fixed in two
attempts, drop that cluster from the PR, re-add the corresponding rule at `warn`, set the ratchet to the
exact residual count, and file an issue (a decision for the user, not the dev agent). The Makefile commit can ship on its own via `development` if the rest stalls (it is independent).

## 8. Final Verification (Definition of Done, run once by `qa-security` / orchestrator)

1. Targeted Playwright (firefox): specs covering ProxyHostForm, CertificateList, UsersPage, CrowdSec ban
   modal, security headers / audit logs forms. No full-suite or cross-browser runs locally.
2. GORM scan, backend coverage: N/A (no backend change). Make sure `git diff --stat` shows only
   `Makefile` as a non-frontend, non-tests, non-config change.
3. Mid-PR checkpoints: `qa-security` runs `scripts/frontend-test-coverage.sh` after Commit 8a and after
   Commit 11 (not only at the end), failing back to the dev agent with uncovered files. At the end,
   `bash scripts/local-patch-report.sh` then `scripts/frontend-test-coverage.sh` (line and branch >=85%;
   investigate any drop vs. the 85.6% branch baseline).
4. Security scans (CodeQL/Trivy): deferred to CI (no new feature surface; `chore:`/`fix:` only).
5. `lefthook run pre-commit`, `cd frontend && npm run type-check && npm run build`,
   `npm run lint` (0 warnings), `npm run lint:locales`, `npm run lint:e2e`.
6. Clean up: no stray `eslint-disable` without a reason comment; grep the diff for new `eslint-disable`,
   count must be justified in the PR body.

## 9. Acceptance Criteria

- `cd frontend && npm run lint` reports 0 errors and 0 warnings (locale sync excluded and reported
  separately by `npm run lint:locales`, 0 errors).
- `npm run lint:e2e` from the repo root lints `tests/**` and passes; lefthook runs it on staged `tests/` files.
- CI: frontend lint and the new `e2e-lint` job are blocking; `--max-warnings` is 0 (or the exact documented
  residual).
- `make lint-backend` and `make lint-agent` use a 25m timeout.
- All real a11y, security-linter and react-hooks findings are fixed; any inline disable has a reason and is
  listed in the PR body.
- Frontend branch coverage did not drop below the 85% floor; all vitest and targeted Playwright runs green.
- Issues #1743, #1716, #1820 are closed by the PR (PR body: `Closes #1743, closes #1716, closes #1820`).

## 10. Resolved Decisions (user approved the plan with these recommendations)

1. `no-node-access` / `no-container`: option B (path-scoped legacy list, rule kept for new tests). The
   tracking issue is filed at the start of the PR (Commit 1 time), with the glob list generated from the lint
   JSON (`jq` over `testing-library/no-node-access` and `no-container` file paths, unique, sorted).
2. Locales: `lint` (gating) split from `lint:locales` (informational `identical-keys`).
3. Root linting is typed (`tests/tsconfig.json` + `no-floating-promises`).
4. Root `e2e-lint` job is blocking immediately.
5. Commit prefixes as listed in Section 7.
6. Commit 8b (migrating overlays to `src/components/ui/Dialog.tsx`) is a filed follow-up issue, not in this PR.

Remaining small open item: none blocking.
