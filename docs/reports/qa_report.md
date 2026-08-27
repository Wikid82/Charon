# QA Report — Sidebar Footer & README Support/Donation Links

**Branch**: `development`
**Commits reviewed**: `298b8b3d` ("chore: add support links to sidebar footer"), `7202b3d0` ("chore: add support/donation section to README")
**Reviewed by**: qa-security agent (scaled-down DoD validation pass)
**Date**: 2026-08-26
**Plan reference**: `docs/plans/current_spec.md` (§6 Phase 5, §7 Acceptance Criteria)

## Summary

This is a frontend + docs **chore**: two small, always-on external links (Buy Me a
Coffee, GitHub Sponsors) added to the app's sidebar footer, plus a matching README
section. No backend/Go changes, no new settings/config/DB, no toggle. Both commits
already passed a supervisor code review (APPROVED, no blocking issues). This report
independently RUNS the scaled-down Definition of Done gates called for by the plan
and confirms they pass, plus a security spot-check on the two new external links.

**Verdict: PASS**, for the two commits as actually committed (`298b8b3d`,
`7202b3d0`). See **important caveat** below — a live, uncommitted, unreviewed
modification to the same three files was discovered mid-session in the shared
working tree; it is *not* part of what was reviewed/approved and is called out
separately so it isn't silently folded into this sign-off.

---

## Important Caveat: Concurrent Uncommitted Drift Detected Mid-Session

While running checks, `git status` revealed the working tree had gone dirty
**after** several of my checks had already run, with fresh timestamps
(`Layout.tsx` 23:10:19, `Layout.test.tsx` 23:10:57, `README.md` 23:10:34 — all
within minutes of my QA session, and after the two commits under review had
already landed cleanly). An untracked `verify-footer.mjs` (a scratch Playwright
script driving a live dev server) also appeared at 23:15:08. This strongly
indicates another agent/process was concurrently editing these exact files while
this QA pass was running.

The uncommitted diff (confirmed via `git diff`) reorders the two footer links
(Sponsor before Coffee instead of the committed Coffee-before-Sponsor), adjusts
spacing classes (`pt-4`→`pt-6 pb-3`, restructures `mb-2`→`mt-2 mb-2`), adds a new,
**unreviewed** test (`'orders the sidebar footer as icon row, then version
block...'`), and reorders the two README badges. None of this went through the
supervisor review that approved the two commits I was asked to validate.

**Action taken**: None. Per the Git Safety Protocol, I did not stash, revert, or
otherwise touch this in-flight, uncommitted work — doing so risked destroying
another process's active edits. I instead validated the actual approved commits
directly via `git show <sha>:<path>` (bypassing the dirty working tree) wherever
a check could have been contaminated by it (see Check 7 below).

**Recommendation for whoever dispatched this QA pass**: this drift needs to be
triaged before the branch is considered final — either finish it, get it
reviewed, and land it as a proper follow-up commit (a legitimate visual-order
tweak is fine, but it needs review + a real test, not silent inclusion), or
discard it if it was exploratory. The untracked `verify-footer.mjs` at the repo
root should not be committed as-is (it's a scratch script hardcoding a
scratchpad output path and test credentials, and lives outside `tests/`).

**Impact on this report's checks**: Checks 1–3, 4, 5, 6, 8 all completed *before*
23:10 (the drift's first timestamp) and are unaffected. Check 7 (security
spot-check) was initially run against the by-then-dirty working tree and has been
re-run against the authoritative committed blobs (`git show`) below — result
unchanged (still PASS), just re-verified against the correct source.

---

## Scope Verification

```
git diff --stat 298b8b3d~1..7202b3d0
```
```
 README.md                                         | 13 ++++++++++++-
 frontend/src/components/Layout.tsx                | 22 +++++++++++++++++++++-
 frontend/src/components/__tests__/Layout.test.tsx | 22 ++++++++++++++++++++++
 3 files changed, 55 insertions(+), 2 deletions(-)
```
Exactly the three files the plan calls for. **Zero** backend/Go files, **zero**
`.github/FUNDING.yml` changes, **zero** settings/config/migration files. Matches
plan Acceptance Criteria and CON-001/CON-002/CON-004. ✅ PASS

---

## Definition of Done — Gate by Gate

| # | Gate | Status | Detail |
|---|------|--------|--------|
| 1 | `npx vitest run src/components/__tests__/Layout.test.tsx` | ✅ **PASS** | `Test Files 1 passed (1)`, `Tests 26 passed (26)`. Verbose run confirms the new test explicitly: `✓ Layout > renders support/donation links in the sidebar footer`. |
| 2 | `npm run type-check` | ✅ **PASS** | `tsc --noEmit` — zero errors. |
| 3 | `npm run build` | ✅ **PASS** | Vite production build completed (`✓ built in 1.83s`), no errors. |
| 4 | `bash scripts/local-patch-report.sh` | ✅ **PASS (artifacts + scope)** / ⚠️ pre-existing unrelated overall warning | Both `test-results/local-patch-report.md` and `.json` produced. See coverage detail below. |
| 5 | `lefthook run pre-commit` | ✅ **PASS** | See detail below. |
| 6 | Frontend coverage (`scripts/frontend-test-coverage.sh`) | ✅ **PASS** | Project-wide lines coverage 90.8% (7551/8316), gate is 87% minimum → `Coverage gate: PASS`. See detail below for `Layout.tsx`-specific note. |
| 7 | Security spot-check on new links | ✅ **PASS** | See detail below, re-verified via `git show` against the actual committed blobs. |
| 8 | Combined diff scope | ✅ **PASS** | Confirmed above — only the 3 expected files. |

### Check 4 detail — Local Patch Coverage Preflight

First run used a stale `frontend/coverage/lcov.info` (dated Aug 18, predating
these commits), which produced a misleading `Frontend patch coverage 0.0%`
warning purely because the coverage snapshot was 8 days old. After regenerating
fresh frontend coverage (Check 6) and re-running:

```
| Scope    | Changed Lines | Covered Lines | Patch Coverage (%) | Status |
|----------|--------------:|---------------:|--------------------:|--------|
| Overall  | 105           | 94              | 89.5                 | warn   |
| Backend  | 105           | 94              | 89.5                 | pass   |
| Frontend | 0             | 0               | 100.0                | pass   |
| Agent    | 0             | 0               | 100.0                | pass   |
```

- **Frontend: 0 changed instrumentable lines, 100% / PASS.** The new JSX
  attribute lines (`href`, `target`, `rel`, `aria-label`) in `Layout.tsx` carry
  no separate Istanbul instrumentation markers (they're not independently
  executable statements), so the tool correctly counts 0 coverable changed
  lines for the frontend scope of this chore — nothing uncovered.
- **Overall: 89.5%, WARN (just under the 90% default threshold).** The single
  file driving this is `backend/internal/services/notification_service.go`
  (11 uncovered lines: 275, 418-419, 430-431, 443-444, 446, 465, 469-470).
  Confirmed via `git log -- backend/internal/services/notification_service.go`
  that these changes come from commits `8bb9553b`, `e83b563a`, `c073f4b2`, etc.
  — **all predate and are unrelated to** the two chore commits under review
  (unmerged "Notification Engine Roadmap" work already on `development` before
  this task started). The report's baseline (`origin/main...HEAD`) naturally
  includes all of `development`'s divergence from `main`, not just this chore's
  two commits — this WARN is pre-existing branch drift, not something
  introduced by `298b8b3d`/`7202b3d0`. Not a blocker for this chore's sign-off.

### Check 5 detail — Lefthook

Since the two commits are already committed (nothing staged), a plain
`lefthook run pre-commit` reports all hooks as `(skip) no matching staged
files`. Re-ran targeted at the three changed files with `--force`:

```
lefthook run pre-commit --file frontend/src/components/Layout.tsx \
  --file frontend/src/components/__tests__/Layout.test.tsx --file README.md --force
```

Result: **all hooks ✔️**, including `frontend-lint`, `frontend-type-check`,
`golangci-lint-fast` (`0 issues.` for backend and agent — expected, no Go files
in scope), `semgrep`, `shellcheck`, `dockerfile-check`, `actionlint`,
`muzzle-allowlist-parity`. ESLint reported `1188 problems (0 errors, 1188
warnings)` — **zero errors**, and a targeted grep of the full lint output for
`Layout.tsx`/`Layout.test.tsx` returned **zero matches** — i.e. none of the
1188 pre-existing warnings originate from either file touched by this chore.

### Check 6 detail — Frontend coverage regression check

`bash scripts/frontend-test-coverage.sh` (deprecated wrapper, still functional)
ran the full frontend suite with coverage:

```
Statements   : 89.6% ( 8031/8963 )
Branches     : 82.88% ( 5554/6701 )
Functions    : 87.2% ( 2577/2955 )
Lines        : 90.8% ( 7551/8316 )
Coverage gate: PASS (lines 90.8% vs minimum 87%)
```

No regression: the new lines in `Layout.tsx` are exercised by the new
`'renders support/donation links in the sidebar footer'` test (Check 1), and
project-wide coverage remains well above the 87% gate this script enforces.

### Check 7 detail — Security spot-check (re-verified against committed blobs)

```
git show 298b8b3d:frontend/src/components/Layout.tsx | sed -n '349,375p'
```
Confirmed both anchors, exactly as committed:

```tsx
<a
  href="https://buymeacoffee.com/Wikid82"
  target="_blank"
  rel="noopener noreferrer"
  aria-label="Support Charon on Buy Me a Coffee"
  className="hover:text-content-primary transition-colors"
>
  <Coffee className="w-4 h-4" />
</a>
<a
  href="https://github.com/sponsors/Wikid82"
  target="_blank"
  rel="noopener noreferrer"
  aria-label="Sponsor Charon on GitHub"
  className="hover:text-content-primary transition-colors"
>
  <Heart className="w-4 h-4" />
</a>
```

- Both `<a>` tags carry `target="_blank"` **together with**
  `rel="noopener noreferrer"` — correct reverse-tabnabbing mitigation
  (`window.opener` cannot be used by the destination page to navigate the
  opener). SEC-001 satisfied.
- URLs match `.github/FUNDING.yml` exactly: `buy_me_a_coffee: Wikid82` →
  `https://buymeacoffee.com/Wikid82`; `github: Wikid82` →
  `https://github.com/sponsors/Wikid82`. No typos, no open-redirect risk (both
  are static, hardcoded, non-user-controlled strings — no injection surface).
- `.github/FUNDING.yml` itself: confirmed unchanged by `git diff --stat`
  (Check 8) — CON-004 satisfied.
- README (`git show 7202b3d0:README.md`) confirms the matching badge links,
  same two URLs, well-formed `<p align="center"><a href="..."><img
  src="..."></a></p>` markup, consistent with the existing badge convention
  elsewhere in the file.

No CRITICAL/HIGH/MEDIUM findings. This is static, outbound-only markup with no
user input, no dynamic URL construction, and no new attack surface.

---

## Explicitly Skipped (per plan's scaled-down DoD, justified)

- CodeQL Go/JS, Trivy, GORM scans — no new feature surface, no Go changes, no
  model/GORM changes. `fix:`/`chore:`-scoped deferral rule applies; CI runs
  both unconditionally regardless.
- Backend coverage (`scripts/go-test-coverage.sh`) — no backend files changed.
- New/modified Playwright E2E — none exist for this area, plan's Research 2.4
  confirms no existing spec touches the footer/version block; none required.

---

## Acceptance Criteria Cross-Check (plan §7)

- [x] Coffee icon link → `https://buymeacoffee.com/Wikid82`, Heart icon link →
      `https://github.com/sponsors/Wikid82`, both `target="_blank" rel="noopener
      noreferrer"`, correct `aria-label`s.
- [x] Muted styling only (`text-content-muted` / hover `text-content-primary`),
      inside the same collapse-hiding parent as the version text.
- [x] Unconditionally rendered — no settings/feature-flag/auth gate (confirmed
      by reading the committed JSX — no conditional wraps the two `<a>` tags).
- [x] `Layout.test.tsx` has a new passing test asserting `href`, `aria-label`
      accessible name, and `target`.
- [x] README "🐛 Get Help" rename + new "☕ Support This Project" section,
      badges well-formed.
- [x] No Discord edits, no edits to other docs files.
- [x] Only the 3 expected files changed; `.github/FUNDING.yml` unchanged; no
      settings/config/DB/migration code anywhere.
- [x] vitest / type-check / build / lefthook all pass.
- [x] Frontend coverage confirms no regression.
- [x] Both commit messages use the `chore:` prefix.

---

## Final Verdict

**PASS** for commits `298b8b3d` and `7202b3d0` as committed. All required gates
(1–8 above) pass; the one WARN surfaced by the patch-coverage preflight (overall
89.5% vs 90%) is fully attributable to pre-existing, unrelated backend work
already on `development` before this chore, not to these two commits, whose own
frontend scope shows 0 uncovered changed lines.

**Separate, non-blocking-for-this-verdict flag**: uncommitted, unreviewed
working-tree drift on the same three files (plus an untracked scratch script)
was detected live during this QA pass — see "Important Caveat" above. This
should be resolved (committed via review, or discarded) before the branch is
treated as fully done, since it currently sits un-reviewed on top of an
otherwise-approved, otherwise-clean chore.
