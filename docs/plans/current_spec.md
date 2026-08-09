# Issue #619 — Phase 3 Technical Debt: Test Infrastructure Cleanup

Status: Planning complete, pending supervisor review.
Branch: `test/issue-619-test-infra-debt` (tip of `development`, working tree clean at plan time).
PR base branch: **`development`** (per `gh pr list` convention — `main` only receives weekly `nightly` promotion merges via merge commit; this is a normal feature PR).
Closes: `#619` ("Phase 3 Technical Debt Issues" — bundles 5 sub-issues, verified below).

---

## 1. Introduction

### 1.1 Objective

Close out GitHub issue #619 with a single feature PR that:

1. Un-skips 5 confirmed-stale Vitest suites blocked on a long-fixed `undici`/jsdom WebSocket bug (sub-issue 1), resolves the 6th related skip with a root-cause-appropriate fix (not a blind unskip), and proves no regressions via a full frontend suite run.
2. Replaces 59 tautological (`expect(x || true).toBeTruthy()`-shaped) Playwright assertions across 11 E2E spec files with real, deterministic assertions or explicit `test.skip()` calls with accurate reasons (sub-issue 2) — the bulk of this PR's work.
3. Confirms backend coverage for `internal/services` and the relocated `backend/pkg/dnsprovider/builtin` package remains healthy with no code changes required (sub-issue 3).
4. Confirms the feature-flag async propagation flakiness was already resolved via `waitForFeatureFlagPropagation()` in the reorganized spec file, with no code changes required (sub-issue 4).
5. Confirms WebKit E2E test discovery/config is healthy and schedules the outstanding full WebKit run as a Definition-of-Done gate (sub-issue 5).

### 1.2 Why one PR

Per `CLAUDE.md` "Commit Slicing & PR Strategy" and repo memory (`feedback_one_feature_one_pr.md`): issue #619 is one feature (test-infrastructure debt), closed by one PR with ordered commits. Sub-issues 3 and 4 require **no code changes** — they contribute verification evidence to the PR's DoD run and the closing PR description, not separate commits.

### 1.3 Non-goals

- No production code changes (backend or frontend application code). This PR touches only test files, test infrastructure, and documentation.
- No changes to `.gitignore`, `.dockerignore`, `codecov.yml`, or any `Dockerfile` — reviewed explicitly in §3.6, all confirmed already correct for this change (see findings).
- `CrowdSecBouncerKeyDisplay.test.tsx` (4 `it.skip` at lines 205/209/213/219, unrelated clipboard-API mock issue) is explicitly **out of scope** and must not be touched.

---

## 2. Research Findings — Ground Truth Verification (2026-08-07)

All findings below were re-verified directly against the current working tree (branch `test/issue-619-test-infra-debt`, tip of `development`) — greps, file reads, and non-mutating test/coverage runs. Numbers in the original issue text and the prior same-day investigation summary are corrected where they drifted.

### 2.1 Sub-issue 1 — undici/WebSocket jsdom blocker: CONFIRMED STALE, ACTION REQUIRED

Dependency state confirmed via `npm ls`:
- `jsdom@30.0.1` (root + deduped under `vitest@4.1.10`)
- `undici@8.10.0` (transitive, via jsdom only)

The upstream bug this blocker cited (`nodejs/undici#1671`, WebSocket mock `InvalidArgumentError`) is long fixed at this version pair.

**Confirmed skip inventory** (exact, re-counted against source, not the prior summary):

| File | Skip marker | Test count (verified via grep) |
|---|---|---|
| `frontend/src/pages/__tests__/Security.test.tsx:35` | `describe.skip('Security', ...)`, comment `// BLOCKER 3: Temporarily skipped due to undici InvalidArgumentError in WebSocket mocks` | 22 |
| `frontend/src/pages/__tests__/Security.audit.test.tsx:52` | `describe.skip('Security Page - QA Security Audit', ...)` | 18 |
| `frontend/src/pages/__tests__/Security.errors.test.tsx:68` | `describe.skip('Security Error Handling Tests', ...)` | 13 |
| `frontend/src/pages/__tests__/Security.loading.test.tsx:59` | `describe.skip('Security Loading Overlay Tests', ...)` | 12 |
| `frontend/src/pages/__tests__/Security.dashboard.test.tsx:67` | `describe.skip('Security Dashboard - Card Status Tests', ...)` | 18 |

Subtotal: **83 tests** across 5 files. Sum matches exactly.

**The 6th file — `Security.functional.test.tsx:680`, `it.skip('should open notification settings modal when button is clicked', ...)`, comment `// Skip: Modal component uses WebSocket connections internally`:**

This comment is **inaccurate**, and unskipping as-is would produce a real (non-WebSocket) failure. Root-cause trace performed per `CLAUDE.md`'s Root Cause Analysis Protocol:

- `frontend/src/pages/Security.tsx:297-303` — the "Notifications" header button's `onClick` is `() => navigate('/settings/notifications')`. It is a **React Router navigation**, not a modal. There is no `role="dialog"` anywhere in `Security.tsx`.
- `Security.functional.test.tsx:20-27` mocks `useNavigate` (`mockNavigate = vi.hoisted(() => vi.fn())`) — the file's own test harness already expects navigation, not a modal, elsewhere.
- **The correct test already exists in the same file**, passing, uncontested: `Security.functional.test.tsx:452-464`, `it('should navigate to notifications settings when Notifications button is clicked', ...)`, which asserts `expect(mockNavigate).toHaveBeenCalledWith('/settings/notifications')`.

Conclusion: the skipped test at line ~680 is **dead, stale test code** describing UI behavior (a modal) that was replaced by a navigation at some prior refactor, and the replacement behavior already has full, correct, passing coverage elsewhere in the same file. Per `CLAUDE.md` "CLEAN: Delete dead code immediately," the correct fix is **deletion of the stale `it.skip` block** (the `describe('Notification Settings Modal', ...)` wrapper at line ~677 becomes empty and should be removed with it), not an unskip and not a comment-only edit. This is a stronger, more correct resolution than either option the investigation brief offered, and it should be called out explicitly in the PR description as the resolution for this file.

**Current full-suite baseline** (`npx vitest run --coverage=false`, non-mutating, run to completion — 635s):

```
Test Files  263 passed | 5 skipped (268)
     Tests  3247 passed | 88 skipped | 2 todo (3337)
```

88 skipped = 83 (sub-issue-1, in scope) + 1 (`Security.functional.test.tsx` notification-modal test, in scope, to be deleted not unskipped) + 4 (`CrowdSecBouncerKeyDisplay.test.tsx`, confirmed out of scope). Arithmetic reconciles exactly — no other undici/WebSocket-flavored skips exist anywhere else in `frontend/src` (verified via repo-wide grep for `undici`, `BLOCKER 3`, `WebSocket connections internally`).

### 2.2 Sub-issue 2 — Weak/tautological E2E assertions: CONFIRMED, LARGER THAN ORIGINAL ISSUE TEXT, MAJORITY REQUIRE REAL FIXES

Pattern searched: literal `|| true` immediately preceding `.toBeTruthy()` in `tests/**/*.spec.ts` (the actual pattern in this repo — confirmed not a generic `x` placeholder). Exact current count: **59 occurrences across 11 files**, matching the prior investigation's file list and the prior day's rough counts almost exactly (one file's estimate, `system-settings-feature-toggles.spec.ts`, is 1, not the previously-noted range — reconfirmed by direct grep):

| File | Count | Lines |
|---|---|---|
| `tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts` | 13 | 156, 200, 339, 465, 545, 623, 825, 845, 862, 949, 985, 1018, 1061 |
| `tests/core/certificates.spec.ts` | 9 | 204, 229, 718, 759, 1037, 1052, 1118, 1146, 1158 |
| `tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts` | 8 | 188, 314, 393, 498, 596, 601, 684, 708 |
| `tests/core/proxy-hosts.spec.ts` | 8 | 209, 255, 461, 523, 547, 643, 969, 1014 |
| `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts` | 7 | 290, 325, 352, 452, 555, 680, 736 |
| `tests/core/navigation.spec.ts` | 4 | 238, 559, 733, 758 |
| `tests/settings/smtp-settings.spec.ts` | 4 | 121, 165, 231, 906 |
| `tests/core/dashboard.spec.ts` | 3 | 232, 370, 491 |
| `tests/settings/account-settings.spec.ts` | 1 | 875 |
| `tests/security/system-settings-feature-toggles.spec.ts` | 1 | 317 |
| `tests/manual-dns-provider.spec.ts` | 1 | 311 |
| **Total** | **59** | |

**Decision framework applied to every occurrence** (per task instructions):

- **(a) Real conditional assertion** — used when the test's own name or an adjacent comment already states a definite, deterministic expectation ("should show X", "X should appear") that the app can be made to satisfy reliably. Mechanical sub-case: when the expression already has a *real* multi-condition OR (e.g. `hasX || hasY || true`), the fix is simply dropping the trailing `|| true` — the meaningful disjunction underneath is preserved.
- **(b) Explicit `test.skip()` / early return with accurate comment** — used only where the underlying condition is genuinely environment- or timing-dependent (cross-browser keyboard focus order, network-dependent external reachability *content* — as opposed to "some feedback appeared," which is still deterministic, race conditions in animation/skeleton timing). This repo already has an established, correct convention for this — `tests/proxy-host-drag-drop.spec.ts` (19 call sites) and `tests/certificate-delete.spec.ts` / `tests/certificate-bulk-delete.spec.ts` (1 each) all use `test.skip(true, '<reason>')` mid-test when a precondition isn't met. **Reuse this exact convention** — do not invent a new pattern.
- **(dead code) Delete** — used when a hard `expect(...).toBeVisible()` (or equivalent) already precedes the tautological line for the *same* condition, making the soft check unreachable/redundant.

Classification results by file (full per-line detail for implementers; "(a)", "(b)", "(dead)" tags below are the required fix per line):

#### `tests/core/certificates.spec.ts` (9 `|| true` occurrences, plus 1 additional vacuous test with no tautology to grep for) — includes the 3 originally-named tests plus 2 more sharing the same defect

- **L204** `hasSortIcon || true` — comment above states "Sort icon should appear" as a definite requirement → **(a)**: `expect(hasSortIcon).toBe(true)`.
- **L229** `hasAlert || true`, test `'should show SSL info alert'` → **(a)**: test name itself is the requirement.
- **L718** `hasDelete || true`, test `'should show delete button for staging certificates'` → **(a)**.
- **L759** `hasToast || true`, test `'should warn if certificate is in use by proxy host'` → **(a), root-cause fix required, see below.**
- **L1037** `hasSslColumn || true` — comment: "SSL column *may* show certificate info" → checked against the source (`frontend/src/pages/ProxyHosts.tsx:556-557`): the SSL column (`key: 'ssl', header: t('proxyHosts.columnSSL')`) is a **static column definition**, not conditional per-row/per-feature-flag — unlike proxy-hosts.spec.ts L643's `hasWs`/`hasAcl` (which genuinely vary per host's configuration and need a seeded host to be deterministic), this column header renders unconditionally whenever the table itself renders. → **(a), and simpler than L643**: no seeding needed — the `hasTable` check already above this line guarantees the table is rendered, so `expect(hasSslColumn).toBe(true)` is deterministic as-is; verify at implementation time that no feature flag gates the column before finalizing.
- **L1052** `hasHeading || true` — a **hard** `await expect(heading).toBeVisible({ timeout: 10000 })` already executes immediately above this line for the identical locator → **(dead)**: delete the redundant soft-check (3 lines).
- **L1118** `hasError || true`, test `'should show error message on API failure'` — **root-cause issue**: the test never injects a failure (no `page.route(...)` interception forcing a 4xx/5xx). It cannot show an error message because no error is ever induced. → **(a) with expanded scope**: add a `page.route('**/api/v1/certificates', route => route.fulfill({ status: 500, ... }))` (or equivalent, matching the mocking convention used elsewhere in this same file's "Error Handling" section if one exists — verify at implementation time) before navigation, then assert the error message is real and visible. This is not a one-line fix; note it in the commit as a slightly larger item.
- **L1146** `hasDescription || true`, test `'should have PageShell with title and description'` → **(a)**.
- **L1158** `hasIcon || true` — comment: "Button should have Plus icon" → **(a)**.

**The 3 named tests, plus 2 more sharing the identical defect, in detail** (backend root-cause traced via `backend/internal/api/handlers/certificate_handler.go:387-470`, `CertificateHandler.Delete`):

Note: a 5th test in the same `describe` block, `'should show config reload overlay during deletion'` (~L806-821), was not caught by the initial `|| true` grep sweep because it doesn't end in a tautology — it uses the identical broken `page.once('dialog', dialog => dialog.accept())` pattern against the same non-existent native dialog, then only does `await waitForDebounce(page)` with **no assertion at all** afterward. It is just as vacuous as the other four and requires the identical interaction-model fix, so it is grouped with them below (item 5) and included in the same commit.

Critical finding: **the delete UI does not use a native `window.confirm()` dialog.** `frontend/src/components/dialogs/DeleteCertificateDialog.tsx` is a fully custom React modal (uses the shared `Dialog`/`DialogContent`/`DialogFooter` primitives, `Button` components with `onClick={onCancel}` / `onClick={onConfirm}`, translated via i18n keys `certificates.deleteTitle`/`deleteConfirmCustom`/`deleteButton`/`common.cancel`). It contains **no `confirm()` call and no "backup" text is guaranteed** — the backup-mentioning copy (`certificates.deleteConfirmCustom`: *"This will permanently delete this certificate. A backup will be created first."*, `frontend/src/locales/en/translation.json:234`) is used **only** when `getWarningKey()` falls through to the default case (i.e. the certificate is not `expired`, not `expiring`, and not `letsencrypt-staging`); the other 3 status-specific messages (`deleteConfirmStaging`, `deleteConfirmExpired`, `deleteConfirmExpiring`) never mention backups at all.

All five existing tests (`'should show delete confirmation dialog'` L723, `'should warn if certificate is in use by proxy host'` L741, `'should cancel delete when confirmation dismissed'` L764, `'should create backup before deletion'` L788, `'should show config reload overlay during deletion'` L806) currently drive the flow via `page.once('dialog', ...)` — Playwright's **native browser dialog** handler. Since the app never opens a native dialog for this flow, **that handler callback never fires**; the tests currently click the delete button (opening the *custom* modal, which is left dangling/unclosed) and then either do nothing further or check a `hasX || true` that trivially passes. These tests currently exercise almost none of the real deletion flow. This is a larger, root-cause-level fix, not a one-line assertion swap:

1. **`'should show delete confirmation dialog'` (L723, not currently `|| true` but must be fixed alongside the others for the block to work at all)**: replace `page.once('dialog', ...)` with locating the actual custom modal (`page.getByRole('dialog')` from the shared `Dialog` primitive — verify exact role/testid in `frontend/src/components/ui/Dialog.tsx` at implementation time) and asserting its title (`t('certificates.deleteTitle')` → "Delete Certificate") and Cancel/Delete buttons are visible.
2. **`'should warn if certificate is in use by proxy host'` (L741/L759)**: backend `Delete()` returns `409 {"error": "certificate is in use by one or more proxy hosts"}` **before** any backup is attempted, when `IsCertificateInUse`/`IsCertificateInUseByUUID` is true. Real fix: select (or seed via API, matching this file's existing seeding convention) a certificate that is actually attached to a proxy host, click delete, click the custom modal's Confirm button, and assert a real error toast/message appears (matching the app's toast convention — `sonner`/`[role="alert"]`, consistent with other files in this PR) rather than the current always-true check. Do not rely on "whichever cert happens to be first in the table."
3. **`'should cancel delete when confirmation dismissed'` (L764)**: replace `page.once('dialog', dialog => dialog.dismiss())` with clicking the custom modal's **Cancel** button. The existing row-count check (`rowsBefore === rowsAfter`) is real and should be **kept**, but per the task's explicit instruction, **add a backend-state assertion**: `GET /api/v1/certificates/{id}` (via `getCertificateViaAPI` from `tests/utils/api-helpers.ts`, the file's already-established API-verification helper — reuse it, do not invent a new one) returns `200` and the certificate is still present, proving cancellation didn't merely hide a row client-side.
4. **`'should create backup before deletion'` (L788, currently checks `dialog.message()` contains "backup" via a handler that never fires — the current implementation is not even a tautology, it is dead/vacuous)**: backend confirms `CreateBackup()` (via `BackupServiceInterface`) is called synchronously in the `Delete` handler, for a certificate that is **not** in use, before the delete completes. Correct fix: capture the backup list via `GET /api/v1/backups` (same endpoint mocked/used in `tests/tasks/backups-create.spec.ts`; no existing typed helper for it in `tests/utils/api-helpers.ts` — add one, `getBackupsViaAPI`, following the exact pattern of the file's other `get*ViaAPI` functions) **before** the delete, click Confirm on the custom modal for a certificate guaranteed not in use, wait for the delete to complete, then `GET /api/v1/backups` again and assert a new backup entry exists (by count increase and/or a `created_at`/filename close to "now"). Do **not** assert on dialog text — the text does not reliably mention "backup" depending on certificate status, as shown above.
5. **`'should show config reload overlay during deletion'` (~L806-821, currently `page.once('dialog', dialog => dialog.accept())` then only `await waitForDebounce(page)` — no assertion at all, silently vacuous)**: replace with clicking the custom modal's **Confirm/Delete** button for a certificate guaranteed not in use, then assert the actual loading/config-reload overlay is real: locate it the same way `tests/security/system-settings-feature-toggles.spec.ts:317`'s `overlayVisible` check does (`.fixed.inset-0.z-50` / `[data-testid="config-reload-overlay"]` — reuse that locator convention rather than inventing a new one) and assert it becomes visible during the delete request and then resolves/disappears once the request completes, rather than the current no-op.

#### `tests/core/proxy-hosts.spec.ts` (8 occurrences)

- **L209** `hasBulkBar || true` — comment: "Should show bulk action bar" → **(a)**.
- **L255** `isInvalid || true` — comment: "Browser validation or custom validation should prevent submission" → **(a)**; consider also asserting no `POST /api/v1/proxy-hosts` was sent (stronger, matches sub-issue 2's "verify backend state" spirit) if feasible without large rework.
- **L461** `hostCreated || true` — this is the **creation-verification step of a core CRUD test**. → **(a), strengthen**: assert UI text visible **and** verify via `getProxyHostsViaAPI`/`getProxyHostViaAPI` (already in `tests/utils/api-helpers.ts`) that the host exists server-side with the expected `domain`/`forward_host`/`forward_port` — mirrors the existing convention in this same helper file.
- **L523** `exists || true` (loop over expected security-option checkboxes: force SSL, HTTP/2, HSTS, block exploits, websocket) → **(a)**: these are static, always-rendered form fields; assert each `expect(exists).toBe(true)`.
- **L547** `exists || true` (loop over preset dropdown options: plex, jellyfin, homeassistant, nextcloud) → **(a)**, pending a quick implementation-time check that these presets are indeed static/guaranteed (grep the preset source, e.g. `frontend/src/**/presets*`) rather than feature-flagged.
- **L643** `hasWs || hasAcl || true`, test `'should show feature badges (WebSocket, ACL)'` — comment: "May or may not exist depending on host configuration" → **(a) via test-setup fix**: rather than leaving this permanently unverifiable, seed/select a host in the test's own setup with `websocket_support: true` (via `createProxyHostViaAPI`) so `hasWs` is deterministic; drop `|| true`.
- **L969** `hasApply || hasRemove || true` — comment: "Should have apply/remove tabs or buttons" (definite) → **(a)**: drop `|| true`.
- **L1014** `hasFocus || true` (keyboard nav: 3 Tabs inside an open modal, expect something focused) → **(a)** preferred (modals should trap/receive focus deterministically); fall back to **(b)** only if empirically flaky per-browser during implementation (cross-reference with sub-issue 5's WebKit focus-order risk).

#### `tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts` (13 occurrences)

- **L156** `hasBadge || true` (allow/deny type badge in first seeded row) → **(a)**, contingent on the `beforeEach` seed fixture guaranteeing a row with a known type — verify at implementation time.
- **L200** `isInvalid || true` — comment: "HTML5 validation should prevent submission" (definite) → **(a)**.
- **L339** `hasInfo || true` — "Blacklist should show 'Recommended' info box" (definite, immediately after `selectOption('blacklist')`) → **(a)**.
- **L465** `hasPresets || true` (preset options after clicking "Show presets") → **(a)**.
- **L545** `hasUpdated || true` (rename ACL, verify) → **(a)**, strengthen with `getAccessListViaAPI` name check (same convention as certificates §2.2 backup verification).
- **L623** `hasSuccess || true` (save success toast) → **(a)**.
- **L825** `hasBulkBar || true` → **(a)** (same pattern as proxy-hosts L209).
- **L845** `hasBulkDelete || true`, test `'should show bulk delete button when items selected'` → **(a)**.
- **L862** `hasHeading || true`, test `'should navigate between Access Lists and Proxy Hosts'` — **no** preceding hard assert here (unlike the near-identical certificates.spec.ts:1052 case, which is dead code) → **(a)**: promote to a hard `await expect(heading).toBeVisible({ timeout: 5000 })`.
- **L949** `hasWarning || true`, test `'should show CGNAT warning when ACLs exist'` → **(a)**, contingent on seeded ACL data guaranteeing the CGNAT condition — verify seed fixture at implementation time.
- **L985** `hasExternalIcon || true` (external-link icon on "best practices" link) → **(a)**.
- **L1018** `hasFocus || true` (same keyboard-tab pattern as proxy-hosts L1014) → **(a)** preferred, **(b)** fallback if flaky.
- **L1061** `isHidden || true` (IP input hidden when "local network only" toggle enabled) → **(a)**: deterministic conditional-field-visibility behavior.

#### `tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts` (8 occurrences)

- **L188** `hasWarning || true` (rotate-key confirm dialog warning content) → **(a)**: dialog title/confirm/cancel are already hard-asserted immediately above; the warning text check should be promoted to match.
- **L314** `hasProgress || true` — comment: "Progress may appear briefly - capture if visible" → **(b)**: genuinely a timing race (progress indicator can legitimately complete before the 5s poll ever samples it); use `test.skip()`-with-reason or remove if no stronger signal (e.g., a `waitForResponse` on the rotation request) can be substituted.
- **L393** `hasWarning || true` — inside `if (isDisabled)` guard, checking rotation-disabled warning text → **(a)**: once inside the guard the condition is deterministic (button is confirmed disabled).
- **L498** `hasWarning || true` — comment: "Warnings may or may not be present - just verify we can detect them" → **(b)**: explicitly optional per comment; convert to non-blocking annotation or remove — it currently asserts nothing meaningful either way.
- **L596** `hasBadge || true` (action-type badge in first audit-log row) → **(a)**, contingent on seeded rotation-history data.
- **L601** `hasVersionInfo || true` (version/duration info in same row) → **(a)**, same seeding caveat as L596.
- **L684** `hasToast || true` (keyboard-activated validate button should trigger a result toast) → **(a)**: deterministic feedback requirement.
- **L708** `accessibleName || true` (every visible button should have an accessible name) → **(a)**: real, valuable a11y assertion; drop `|| true`.

#### `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts` (7 occurrences)

- **L290** `hasValidation || true` — comment: "May not have inline validation" (explicit hedge) → **(b)**: convert to `test.skip()`/annotation.
- **L325** `toastVisible || true` — the immediately preceding step already hard-asserts `expect(saveResponse.ok()).toBeTruthy()` (a real successful save) → **(a)**: success feedback must follow a confirmed-successful save; drop `|| true`.
- **L352** `hasSuccess || true` (green-checkmark validation indicator for a valid URL) → **(a)**, pending a quick check that this indicator is unconditionally rendered for the field (verify at implementation time).
- **L452** `toastVisible || true` — comment: "URL reachability depends on network - just verify test button works" → **(a) for the meta-assertion**: regardless of network outcome (reachable/unreachable), *some* toast must always appear after clicking Test — that part is deterministic. Drop `|| true`; do not assert on toast *content*.
- **L555** `hasVersion || true` (version string format) → **(a)**: app always renders a build version (semver or `dev`).
- **L680** `newState !== initialState || true` — this directly overlaps sub-issue 4's feature-flag propagation fix; the test already awaits both the `PUT` and `GET` feature-flags responses via `Promise.all` before reading `newState` → **(a)**: the toggle state change is deterministic once both responses have resolved; drop `|| true`.
- **L736** `accessibleName || true` — same pattern as encryption-management L708 → **(a)**.

#### `tests/core/navigation.spec.ts` (4 occurrences)

- **L238** `hasActiveCurrent || hasActiveClass || true` → **(a)**, mechanical: drop `|| true`, keep the two-way OR (an active nav item must signal state via `aria-current` or an active class — at least one is a real requirement).
- **L559** `foundNavLink || true` — comment: "May not find nav link depending on focus order - this is acceptable" → **(b)**: convert to `test.skip(true, 'no focusable nav link found via keyboard tab order in this run')` per the established `test.skip(true, reason)` convention, rather than a fake pass.
- **L733** `hasAriaCurrent || true` — comment: "aria-current is recommended but not always implemented" → **(a), converge with L238's convention**: check `aria-current` **or** active class (matching the pattern already used at L238 in the same file) instead of `aria-current` alone with a fake fallback — this makes it a real, DRY assertion instead of leaving it permanently soft.
- **L758** `outline || true` (focus-visible indicator style) → **(a)** preferred: assert `outline` is non-empty/not `'none'`; fall back to **(b)** only if empirically flaky across Chromium/Firefox/WebKit during implementation.

#### `tests/core/dashboard.spec.ts` (3 occurrences)

- **L232** `foundButton || true` (quick-action button reachable via keyboard tab loop) → **(b)**: tab-order flakiness, same pattern as navigation.spec.ts:559; convert to `test.skip()`-with-reason when not found within the loop bound, rather than fake pass. The real assertion (`expect(focused).toBeFocused()`) already fires correctly when found.
- **L370** `hasEmptyState || hasActualContent || true` — comment: "Dashboard should show either empty state or content, not crash" (a genuine, environment-independent invariant) → **(a)**, mechanical: drop `|| true`.
- **L491** `reachedCard || focusableElementsFound > 0 || true` — comment: "verify we at least found some focusable elements" → **(a)**, mechanical: drop `|| true`, keep the two-way OR.

#### `tests/settings/smtp-settings.spec.ts` (4 occurrences)

- **L121** `skeletonVisible || true` — comment: "Either skeleton is shown or page loads very fast" → **(b)**: genuine loading-timing race; convert to `test.skip()`-with-reason or remove — a 500ms artificial delay plus a 1000ms visibility timeout should make the skeleton reliably visible, so first try tightening the mock/timeout to make this **(a)** before falling back to **(b)**.
- **L165** `hasValidation || true` — comment: "Either inline validation or form submission is blocked" (definite requirement, required-field case) → **(a)**.
- **L231** `hasValidation || true` — comment: "Validation should occur (inline or via toast)" (definite, invalid-email-format case) → **(a)**.
- **L906** `hasAccessibleError || true` — comment: "Some form of accessible error feedback should exist" (definite a11y requirement) → **(a)**.

#### `tests/settings/account-settings.spec.ts` (1 occurrence)

- **L875** `foundApiButton || true` — comment: "Non-blocking assertion" (explicit hedge, keyboard tab-order search for API key buttons) → **(b)**: convert to `test.skip()`-with-reason, consistent with the tab-order-flakiness cases above.

#### `tests/security/system-settings-feature-toggles.spec.ts` (1 occurrence)

- **L317** `overlayVisible || true` — comment: "Overlay may appear briefly - either is acceptable" → **(b)**: genuine timing race (config-reload overlay can complete before the 1s poll samples it); the `responsePromise` for the `PUT /feature-flags` call is already captured above but never awaited/used to gate this check — first try awaiting that promise before sampling the overlay (would make this **(a)**); fall back to **(b)** if still flaky.

#### `tests/manual-dns-provider.spec.ts` (1 occurrence)

- **L311** `hasVisibleIcon || true` (status icon inside an already-hard-asserted status indicator) → **(a)**: the indicator itself is already hard-asserted visible immediately above; the icon inside it should be deterministic too.

**Summary**: of 59 `|| true` occurrences, **~47 become real assertions (a)**, **~4 are dead code to delete**, and **~8 are genuinely environment/timing-dependent and become explicit `test.skip()` calls (b)** using the repo's existing convention — plus 1 additional vacuous test (certificates.spec.ts's `'should show config reload overlay during deletion'`) that has no `|| true` to count here but requires the identical interaction-model fix (see the certificates.spec.ts breakdown above). Exact per-line final disposition is confirmed during implementation per the guidance above; the DoD gate in §5 enforces that zero bare `|| true`-before-`toBeTruthy()` patterns remain regardless of which bucket each line lands in.

### 2.3 Sub-issue 3 — Backend coverage gaps: CONFIRMED STALE / ALREADY RESOLVED, NO CODE CHANGES

Re-ran directly (non-mutating `go test -cover`):

```
ok  internal/services                        coverage: 88.4% of statements   (target 85%)
ok  internal/services/remotestorage           coverage: 90.3% of statements
ok  backend/pkg/dnsprovider/builtin           coverage: 91.8% of statements   (target 50% incremental)
```

Confirms the prior investigation exactly. `backend/pkg/dnsprovider/builtin` is the correct current location (relocated from `internal/dnsprovider/builtin` as the original issue text said) and is excluded from `codecov.yml` reporting (`ignore:` list, line 136 — "tested via integration tests, not unit tests") but not from `go-test-coverage.sh`'s enforcement; either way, actual coverage is far above both the codecov project target (87%) and the issue's original incremental target (50%). **No regression, no code changes required.** This PR's only obligation here is to capture a coverage run as DoD evidence (§5) and state this explicitly in the PR description (§6).

### 2.4 Sub-issue 4 — Feature flag async propagation tests: CONFIRMED STALE / ALREADY RESOLVED, NO CODE CHANGES

`tests/settings/system-settings.spec.ts` no longer exists (confirmed via `find`); the feature-flag tests were reorganized into `tests/security/system-settings-feature-toggles.spec.ts`, which:
- Imports and calls `waitForFeatureFlagPropagation` **9 times** (exact count via `grep -c`, correcting the prior investigation's "11" estimate) across all 9 tests in the file.
- Has **zero** `.skip`/`.fixme` markers (aside from the one tautological assertion at L317, covered under sub-issue 2 above — a different problem, not the async-propagation flakiness this sub-issue was about).

**No regression, no code changes required.** This file is already fully in-scope for the mandatory full E2E run in §5 (it was already going to run; no special inclusion action needed).

### 2.5 Sub-issue 5 — WebKit E2E tests not executing: CONFIG CONFIRMED HEALTHY, ONE REAL RUN STILL OUTSTANDING

- WebKit `26.5` installed; `npx playwright test --list --project=webkit` discovers **963 tests across 86 files** (re-verified, matches prior investigation exactly).
- `playwright.config.js` (repo root — the config actually governing `tests/`, distinct from the unrelated minimal `frontend/e2e/playwright.config.ts`) reviewed line-by-line: the `webkit` project (L299-314) has **identical** `dependencies`, `testMatch`, and `testIgnore` patterns to `chromium`/`firefox` — no webkit-specific exclusion, no `browserName`-conditioned `test.skip()` anywhere in `tests/**` (repo-wide grep confirmed zero matches).
- Note (informational, not a defect): the `webkit` project's `testIgnore` excludes `**/security-enforcement/**` and `**/tests/security/**`, same as chromium/firefox — those specs only run under the dedicated `security-tests` project, which is **Chromium-only by design** (L237-254, "SEQUENTIAL, Chromium only"). This means 29 of this PR's 59 sub-issue-2 fixes (all of `access-lists-crud.spec.ts`, `encryption-management.spec.ts`, `system-security-settings.spec.ts`, `system-settings-feature-toggles.spec.ts`) are **out of WebKit's run scope entirely, by existing design** — not something this PR changes or needs to change.
- A dedicated `tests/core/caddy-import/caddy-import-webkit.spec.ts` (`@webkit-only` tag) already exists for known WebKit-specific quirks in the Caddyfile-import flow, and `caddy-import-cross-browser.spec.ts` already parameterizes assertions per `browserName` — evidence the team has previously handled real WebKit differences correctly elsewhere; no similar per-browser branching is missing here.
- **Risk flagged**: none identified in config. The keyboard-focus-order tautologies converted to real assertions in §2.2 (proxy-hosts.spec.ts:1014, access-lists-crud.spec.ts:1018, navigation.spec.ts:758) are the most plausible source of **new** WebKit-specific flakiness once they stop being unconditionally true — this is exactly why §2.2 marks them "(a) preferred, (b) fallback if empirically flaky" rather than a hard mandate, and why the full WebKit run (§5) must happen **after** the sub-issue-2 commits land, not before.
- **Not run in this planning pass** (explicitly deferred to execution/QA phase per task instructions): the actual full `npx playwright test --project=webkit` execution. This is a mandatory, explicit Definition-of-Done gate (§5) for this PR.

### 2.6 `.gitignore` / `.dockerignore` / `codecov.yml` / `Dockerfile` review

All reviewed; **no changes required** for this PR:

- `.gitignore`: `frontend/coverage/`, `frontend/test-results/`, `/test-results/`, `/playwright-report/` already cover all artifacts this PR's test runs will produce.
- `.dockerignore`: `tests/`, `test-results/`, `test-data/` already excluded from the Docker build context; no new test directories are being introduced by this PR (only edits to existing spec/test files).
- `codecov.yml`: `**/e2e/**`, `**/*.spec.ts`, `**/__tests__/**` already excluded from coverage accounting; `backend/pkg/dnsprovider/builtin/**` already excluded (consistent with §2.3's finding that this package is verified via integration tests). No new source paths are introduced.
- No `Dockerfile` changes — this PR ships no runtime code.

---

## 3. Technical Specifications

This PR is test-infrastructure-only. There is no new API surface, no database schema change, and no new component. The "component design" for this PR is the test-file structure itself.

### 3.1 Affected files (exhaustive)

**Frontend unit tests (sub-issue 1):**
- `frontend/src/pages/__tests__/Security.test.tsx`
- `frontend/src/pages/__tests__/Security.audit.test.tsx`
- `frontend/src/pages/__tests__/Security.errors.test.tsx`
- `frontend/src/pages/__tests__/Security.loading.test.tsx`
- `frontend/src/pages/__tests__/Security.dashboard.test.tsx`
- `frontend/src/pages/__tests__/Security.functional.test.tsx`

**E2E specs (sub-issue 2):**
- `tests/core/certificates.spec.ts` (includes the 5-test "Certificate Deletion" block rewrite — see §2.2)
- `tests/core/proxy-hosts.spec.ts`
- `tests/core/navigation.spec.ts`
- `tests/core/dashboard.spec.ts`
- `tests/settings/smtp-settings.spec.ts`
- `tests/settings/account-settings.spec.ts`
- `tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts`
- `tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts`
- `tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts`
- `tests/security/system-settings-feature-toggles.spec.ts`
- `tests/manual-dns-provider.spec.ts`

**Test utility additions (sub-issue 2):**
- `tests/utils/api-helpers.ts` — add `getBackupsViaAPI(request, token?)`, following the exact signature/error-handling pattern of the file's existing `get*ViaAPI` functions (e.g. `getCertificatesViaAPI`), targeting `GET /api/v1/backups`.

**No changes**: any `backend/**` file, any `frontend/src` file outside `__tests__/`, `.gitignore`, `.dockerignore`, `codecov.yml`, any `Dockerfile`.

### 3.2 API contracts referenced (read-only, no changes)

These existing endpoints are what the strengthened assertions in §2.2 verify against — documented here for implementer reference, not as new contracts:

| Endpoint | Method | Used by (test) | Purpose in this PR |
|---|---|---|---|
| `/api/v1/certificates/:uuid` | `GET` | `certificates.spec.ts` cancel-delete test | Verify certificate still exists after a dismissed delete |
| `/api/v1/certificates/:uuid` | `DELETE` | `certificates.spec.ts` in-use/backup tests | Existing delete flow (`certificate_handler.go:387-470`) — unchanged |
| `/api/v1/backups` | `GET` | `certificates.spec.ts` backup-creation test | Verify a new backup entry appears after a successful cert delete |
| `/api/v1/access-lists/:id` | `GET` | `access-lists-crud.spec.ts` rename test | Verify renamed ACL persisted server-side |
| `/api/v1/proxy-hosts` / `/api/v1/proxy-hosts/:id` | `GET` | `proxy-hosts.spec.ts` creation test | Verify created host persisted server-side |

### 3.3 Error handling / edge cases to cover in the new assertions

- Certificate delete "in use" path: assert the **specific** 409 error surface (toast/message), not merely "a toast of some kind."
- Certificate delete "backup" path: must select/seed a certificate guaranteed **not** in use (backend returns 409 before attempting backup if in use — asserting backup creation against an in-use cert would be a false test).
- ACL/proxy-host rename/creation: API-level verification must tolerate eventual consistency the same way existing passing tests in these files already do (reuse existing `waitFor`/polling helpers, do not add new ad hoc `setTimeout`s).
- WebKit-sensitive keyboard-focus assertions (§2.2, "(a) preferred, (b) fallback"): implementers must actually run the affected spec under `--project=webkit` (not just chromium/firefox) before finalizing as (a); if flaky, fall back to (b) with an accurate WebKit-specific skip reason, not a silent revert to `|| true`.

### 3.4 Data flow notes

No data flow changes. The certificate-deletion backup verification (§2.2) exercises an **existing** synchronous flow: `DELETE /api/v1/certificates/:uuid` → `IsCertificateInUse` check → (if not in use) `backupService.GetAvailableSpace()` → `backupService.CreateBackup()` → `service.DeleteCertificateByID()` → response. All calls are synchronous within the single request; no polling/async job is involved for this specific path (unlike the general `POST /api/v1/backups` flow used elsewhere, which does return `202` + a job id — do not conflate the two; the cert-delete backup call is a direct, blocking `CreateBackup()`).

---

## 4. Implementation Plan

### Phase 1 — E2E specs for new/changed behavior

No net-new user-facing behavior is being introduced (this is a test-quality fix, not a feature), so there is no `test.fixme()` scaffolding phase in the usual sense. Instead, Phase 1 is: write the `getBackupsViaAPI` helper addition to `tests/utils/api-helpers.ts` (foundation for Commit 2's certificate tests) and confirm it compiles/type-checks against the existing `parseResponse<T>`/`getAuthHeaders` pattern.

### Phase 2 — Foundation (no behavior change)

- Add `getBackupsViaAPI` to `tests/utils/api-helpers.ts`.
- No other foundation work required — this PR doesn't touch shared fixtures, `global-setup.ts`, or `playwright.config.js`.

### Phase 3 — Backend

N/A — confirmed no backend code changes required (§2.3).

### Phase 4 — Frontend / test changes (the bulk of the work)

- Sub-issue 1: unskip 5 files, delete the stale test in the 6th (§2.1).
- Sub-issue 2: fix all 59 tautologies per the file-by-file disposition in §2.2, including the certificate-deletion flow's larger rewrite (native-dialog → custom-modal interaction).

### Phase 5 — Hardening, full-suite validation, docs

- Full Vitest suite run (not just the touched files) to catch regressions from unskipping.
- Full Playwright run across chromium/firefox/webkit, including the security-tests shard.
- Coverage checks (frontend + backend) at/above enforced thresholds.
- PR description scaffolding (§6).

---

## 5. Commit Slicing Strategy

Single PR, `test/issue-619-test-infra-debt` → `development`, ordered commits. Each commit is independently buildable/testable; later commits depend on earlier ones as noted.

### Commit 1 — `test: add getBackupsViaAPI helper for E2E backup verification`
- **Scope**: Foundation. Add `getBackupsViaAPI(request, token?)` to `tests/utils/api-helpers.ts`, matching the existing `get*ViaAPI` pattern exactly (JSDoc block, `parseResponse<T>`, `getAuthHeaders`).
- **Files**: `tests/utils/api-helpers.ts`.
- **Dependencies**: none.
- **Validation gate**: `cd frontend && npm run type-check` passes (the helper file is TS, checked as part of the frontend project); no test run needed yet (unused until Commit 3).

### Commit 2 — `fix: unskip Security.* Vitest suites now that undici/jsdom WebSocket bug is fixed`
- **Scope**: Sub-issue 1. Remove `describe.skip` → `describe` in the 5 files; delete the stale `it.skip('should open notification settings modal...')` block (and its now-empty `describe('Notification Settings Modal', ...)` wrapper) from `Security.functional.test.tsx`.
- **Files**: the 6 files listed in §2.1.
- **Dependencies**: none (independent of Commits 1/3+).
- **Validation gate**: `npx vitest run src/pages/__tests__/Security.test.tsx src/pages/__tests__/Security.audit.test.tsx src/pages/__tests__/Security.errors.test.tsx src/pages/__tests__/Security.loading.test.tsx src/pages/__tests__/Security.dashboard.test.tsx src/pages/__tests__/Security.functional.test.tsx` — zero failures, zero unexpected skips. Then a **full** `npx vitest run` (not just these files) — zero regressions vs. the §2.1 baseline (263→268 passed test files, 3247→3330 passed tests, 88→4 skipped [only the out-of-scope CrowdSec ones remain]). Frontend coverage (`scripts/frontend-test-coverage.sh`) at/above 85%.

### Commit 3 — `fix: replace tautological assertions in certificates.spec.ts with real backend-verified checks`
- **Scope**: Sub-issue 2, certificates file only (highest-complexity file — isolated to its own commit given the custom-modal rewrite). All 9 `|| true` lines in §2.2's certificates.spec.ts breakdown, plus the native-dialog → custom-modal rewrite for all 5 deletion tests in the "Certificate Deletion" block (`should show delete confirmation dialog`, `should warn if certificate is in use by proxy host`, `should cancel delete when confirmation dismissed`, `should create backup before deletion`, and `should show config reload overlay during deletion` — the last of which has no `|| true` to grep for but shares the identical broken `page.once('dialog', ...)` interaction model and currently asserts nothing).
- **Files**: `tests/core/certificates.spec.ts` (uses `getBackupsViaAPI` from Commit 1, `getCertificateViaAPI` already present).
- **Dependencies**: Commit 1.
- **Validation gate**: `npx playwright test tests/core/certificates.spec.ts --project=chromium` and `--project=firefox` both pass, zero flaky retries. Manual grep confirms zero `|| true).toBeTruthy()` remaining in this file.

### Commit 4 — `fix: replace tautological assertions in proxy-hosts and access-lists E2E specs`
- **Scope**: Sub-issue 2, the two largest remaining CRUD-flow files. §2.2's `proxy-hosts.spec.ts` (8 lines) and `access-lists-crud.spec.ts` (13 lines) breakdowns.
- **Files**: `tests/core/proxy-hosts.spec.ts`, `tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts`.
- **Dependencies**: none (uses `getProxyHostsViaAPI`/`getProxyHostViaAPI`/`getAccessListViaAPI`, all already present in `tests/utils/api-helpers.ts` prior to this PR — does not depend on Commit 1's `getBackupsViaAPI` addition).
- **Validation gate**: `npx playwright test tests/core/proxy-hosts.spec.ts --project=chromium --project=firefox` and the access-lists spec via the `security-tests` project (`npx playwright test tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts --project=chromium` per the config's security-shard routing) both pass. Zero `|| true).toBeTruthy()` remaining in either file.

### Commit 4b — `fix: correct Access List UUID usage and CGNAT warning i18n keys` (SCOPE ADDITION — real app bugs found during Commit 4)

**Why this exists**: Commit 4's strengthened `access-lists-crud.spec.ts` assertions (no longer tautological) surfaced two genuine, previously-invisible production defects, confirmed via curl repro + Playwright network trace + source grep (not test artifacts):

1. **Access List edit/update/delete/test-IP all 404 in production.** `frontend/src/pages/AccessLists.tsx`, `frontend/src/hooks/useAccessLists.ts`, `frontend/src/api/accessLists.ts`, and the ACL selector in `frontend/src/components/.../ProxyHostForm.tsx` all key mutations off `acl.id`. `backend/internal/models/access_list.go`'s `ID uint` has `json:"-"` — never serialized; only `uuid` is sent. Every edit/delete/rename request currently resolves to `PUT/DELETE /api/v1/access-lists/undefined` → `404`. `rowKey={(acl) => String(acl.id)}` also collides to `"undefined"` for every table row. `ProxyHosts.tsx` already uses the correct `.uuid` pattern — mirror it.
2. **CGNAT warning banner renders raw i18n keys to every user.** `AccessLists.tsx` calls `t('accessLists.cgnatWarningTitle')` etc. (flat keys) but `frontend/src/locales/en/translation.json` only defines the nested `accessLists.cgnatWarning.title/.message/.solutionsTitle/.solution1-5`. The rendered DOM literally shows concatenated raw key strings to users. Fix: correct the key paths to match the nested structure.

This is a deliberate, narrow deviation from this plan's original §1.3 non-goal ("no production code changes") — made because leaving the 3 tests these bugs broke permanently `test.skip()`-ed would directly contradict sub-issue 2's entire purpose (replacing fake-always-pass checks with real ones that actually catch defects). Both fixes are small, isolated, high-confidence, and directly required for `access-lists-crud.spec.ts`'s already-committed real assertions to pass. This addition must be called out explicitly in the PR description as a scope note, separate from the planned test-infra-only work, so reviewers can evaluate it on its own merits.

- **Scope**: `.id` → `.uuid` swap across the Access List frontend mutation path (param types `number` → `string` to match); i18n key path correction in the CGNAT warning block.
- **Files**: `frontend/src/pages/AccessLists.tsx`, `frontend/src/hooks/useAccessLists.ts`, `frontend/src/api/accessLists.ts`, and the ACL selector in the proxy-host form component (exact file to be confirmed at implementation time — grep for `.id` usage against access-list objects). No backend changes (backend already correctly omits `ID` from JSON; frontend must conform to the existing contract, not the other way around).
- **Dependencies**: Commit 4 (the tests that currently fail because of these bugs must already exist).
- **Validation gate**: `npx playwright test tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts --project=security-tests` — full pass, all 45 tests, zero flaky retries (up from 42/45 after Commit 4). `cd frontend && npm run type-check` passes. `cd frontend && npx vitest run` — zero regressions (no unit tests should reference the old `.id` access-list field, but confirm). Manual smoke check: rename an ACL via the UI, confirm no `undefined` appears in any network request URL.

### Commit 5 — `fix: replace tautological assertions in remaining security-UI and settings E2E specs`
- **Scope**: Sub-issue 2, remainder. §2.2's `encryption-management.spec.ts` (8), `system-security-settings.spec.ts` (7), `navigation.spec.ts` (4), `smtp-settings.spec.ts` (4), `dashboard.spec.ts` (3), `system-settings-feature-toggles.spec.ts` (1), `account-settings.spec.ts` (1), `manual-dns-provider.spec.ts` (1).
- **Files**: the 8 files above.
- **Dependencies**: none (none of these 8 files use `getBackupsViaAPI`).
- **Validation gate**: each file passes under its correct project (security-shard files via `security-tests`/chromium; the rest via chromium + firefox). Zero `|| true).toBeTruthy()` remaining anywhere under `tests/`, verified via `grep -rn "|| true" tests/ --include=*.spec.ts` returning empty.

### Commit 5b — `fix: add aria-current to active navigation links` (SCOPE ADDITION — real app bug found during Commit 5)

**Why this exists**: same pattern as Commit 4b. Commit 5's strengthened `navigation.spec.ts` assertions (`hasActiveCurrent || hasActiveClass`, `hasAriaCurrent || <active class>`, both converged onto "must signal active state via aria-current OR a discoverable active class") surfaced that neither exists: `frontend/src/components/Layout.tsx`'s primary sidebar nav `<Link>`s never set `aria-current`, and the active-state Tailwind classes (`bg-brand-700 text-content-primary`, `text-brand-500`, `bg-brand-500/10 text-brand-500`) contain no `"active"`/`"current"` substring an assistive-tech-oriented check (or a screen reader) could key off. Confirmed reproducible 100% across 3 runs, both chromium and firefox, by the implementing agent — not flakiness. This is a genuine, previously-hidden accessibility gap: there is no programmatic way for assistive tech to identify the current page in the primary nav today.

- **Scope**: Add `aria-current="page"` to the active nav `<Link>` in `Layout.tsx`, conditioned on the existing active-route check already used to apply the active Tailwind classes (do not introduce a new route-matching mechanism — reuse whatever comparison already decides which link gets the active classes).
- **Files**: `frontend/src/components/Layout.tsx`. No backend changes.
- **Dependencies**: Commit 5 (the two navigation tests that currently fail because of this gap must already exist).
- **Validation gate**: `npx playwright test tests/core/navigation.spec.ts --project=chromium --project=firefox` — full pass, including `'should highlight active navigation item'` and `'should indicate current page with aria-current'`, zero flaky retries. `cd frontend && npm run type-check` passes. `cd frontend && npx vitest run` — zero regressions.

### Commit 6 — `docs: close out issue #619 sub-issues 3-5 with coverage/config verification evidence`
- **Scope**: Hardening + docs. No source changes beyond capturing verification evidence. Update `docs/features.md` only if any test-visible behavior description changed (unlikely — confirm at implementation time; if nothing user-facing changed, skip the `docs/features.md` edit and note that explicitly in the PR description instead of forcing an edit for its own sake).
- **Files**: none required; optionally `docs/features.md` if applicable.
- **Dependencies**: Commits 2-5 (needs the final, real test suite to attach real evidence to).
- **Validation gate**: this commit's job *is* the Definition of Done run — see §5.1 below. All gates must be green before this commit closes the PR.

### 5.1 Full DoD validation (runs once, after Commit 5, evidence captured in Commit 6 / PR description)

Per `CLAUDE.md`'s Task Completion Protocol, in order:

1. `npx playwright test --project=firefox` (full suite) — must pass.
2. `npx playwright test --project=chromium` (full suite, includes the `security-tests` shard) — must pass.
3. `npx playwright test --project=webkit` (full suite) — **this is sub-issue 5's outstanding confirming run.** If it fails in a way traceable to one of this PR's newly-real assertions (most likely candidate: the keyboard-focus-order ones flagged "(a) preferred, (b) fallback" in §2.2/§2.5), fix by falling back to the (b) disposition for that specific line with an accurate WebKit-specific skip reason — do not weaken back to `|| true`. If it fails for an unrelated, pre-existing reason, that is a **new finding** outside this PR's original scope and must be flagged back to the user/issue tracker rather than silently patched.
4. `bash scripts/local-patch-report.sh` — patch coverage evidence.
5. `lefthook run pre-commit` (CodeQL Go + JS, staticcheck, etc.) — zero high/critical findings. (No GORM-touching changes in this PR, so §1.5's conditional GORM scan is skipped — confirmed no `backend/internal/models/**` or migration changes.)
6. `make trivy` (or equivalent Trivy container/dependency scan) — zero Critical/High findings. Per `CLAUDE.md`'s Task Completion Protocol step 3, this is **mandatory, zero-tolerance, with no conditional exception** (unlike the GORM scan above, which is explicitly conditional on model/migration changes). This PR touches no dependencies, `go.mod`/`package.json`, or any `Dockerfile`, so no new findings are expected — run and capture as evidence rather than skipping it.
7. `scripts/go-test-coverage.sh` — confirm ≥85%, capturing the §2.3 numbers as evidence (no regressions expected since no backend files changed).
8. `scripts/frontend-test-coverage.sh` — confirm ≥85%, now including the ~84 newly-unskipped tests.
9. `cd frontend && npm run type-check`.
10. `cd backend && go build ./...` and `cd frontend && npm run build`.
11. Full `npx vitest run` — zero failures, zero unexpected skips (only the 4 out-of-scope CrowdSec skips remain).

### 5.2 Rollback / contingency

- Each commit is independently revertable without breaking `development` — none introduce cross-file coupling beyond Commit 1's helper (used only by Commit 3+).
- If the WebKit run (§5.1 step 3) surfaces a **pre-existing, unrelated** failure (not caused by this PR's changes), the contingency is: do not block this PR on it — capture the failure, note it explicitly in the PR description as a newly-discovered, out-of-scope finding, and open a follow-up issue (matching the precedent set by this same investigation's sibling fix, which filed `#1221` for an out-of-scope race condition rather than scope-creeping the original fix).
- If any single sub-issue-2 file proves substantially harder than estimated during implementation (most likely: `certificates.spec.ts`'s custom-modal rewrite), it is already isolated to its own commit (Commit 3) specifically so it can be iterated on without blocking Commits 4-5.
- If full-suite Vitest coverage drops below 85% after unskipping (unlikely, since unskipping only adds passing tests, never removes coverage), do not merge — investigate whether any of the newly-active tests are masking a real component defect (per Root Cause Analysis Protocol) rather than adjusting the threshold.

---

## 6. PR Description Scaffolding

```markdown
## Summary

Closes #619 (Phase 3 Technical Debt Issues). Verifies and resolves all 5 bundled sub-issues:

- **Sub-issue 1 (undici/WebSocket jsdom blocker)** — FIXED. Confirmed stale on jsdom@30.0.1/undici@8.10.0
  (upstream nodejs/undici#1671 long resolved). Unskipped 83 tests across 5 Security.*.test.tsx suites.
  The 6th related skip (Security.functional.test.tsx notification-modal test) was not a WebSocket issue at
  all — root-caused to stale test code describing a modal that was replaced by a router navigation; deleted
  as dead code since equivalent, correct, passing coverage already exists in the same file.
- **Sub-issue 2 (weak/tautological E2E assertions)** — FIXED. 59 `expect(x || true).toBeTruthy()` occurrences
  across 11 spec files replaced with real deterministic assertions, backend-state-verified checks (certificate
  deletion in-use/backup/cancel flows, ACL rename, proxy-host creation), or explicit `test.skip()` calls with
  accurate reasons where genuinely environment-dependent — reusing this repo's existing skip convention.
  certificates.spec.ts's certificate-deletion tests additionally required a root-cause interaction-model fix:
  they drove a native `window.confirm()` that the app no longer uses (replaced by a custom React modal),
  meaning they were exercising almost none of the real delete flow.
- **Sub-issue 3 (backend coverage gaps)** — STALE, already resolved, no code changes. Re-verified:
  internal/services 88.4% (target 85%), remotestorage 90.3%, backend/pkg/dnsprovider/builtin 91.8%
  (target 50% incremental).
- **Sub-issue 4 (feature flag async propagation tests)** — STALE, already resolved, no code changes.
  Re-verified: tests/security/system-settings-feature-toggles.spec.ts already uses
  waitForFeatureFlagPropagation() at 9 call sites, zero .skip/.fixme.
- **Sub-issue 5 (WebKit E2E not executing)** — Config confirmed healthy (963 tests / 86 files discovered,
  no webkit-specific exclusions or browserName-conditioned skips). Full passing run captured as this PR's
  DoD evidence (see Test Plan).

## Test Plan
- [ ] Full `npx vitest run` — zero failures, only the 4 out-of-scope CrowdSecBouncerKeyDisplay skips remain
- [ ] `npx playwright test --project=chromium` (incl. security-tests shard) — full pass
- [ ] `npx playwright test --project=firefox` — full pass
- [ ] `npx playwright test --project=webkit` — full pass (sub-issue 5 confirming run)
- [ ] `scripts/go-test-coverage.sh` ≥ 85%
- [ ] `scripts/frontend-test-coverage.sh` ≥ 85%
- [ ] `lefthook run pre-commit` — zero high/critical CodeQL findings
- [ ] `make trivy` (or equivalent) — zero Critical/High findings
- [ ] `grep -rn "|| true" tests/ --include=*.spec.ts` returns empty
```

---

## 7. Acceptance Criteria

1. Zero `describe.skip`/`it.skip` remain in the 6 sub-issue-1 files except the intentional deletion (not skip) of the stale notification-modal test.
2. `grep -rn "|| true).toBeTruthy()" tests/ --include=*.spec.ts` (or equivalent pattern check) returns **zero** matches.
3. Every occurrence converted to `test.skip()` includes a specific, accurate reason string (no generic "may not apply" left over from the tautology comments).
4. `certificates.spec.ts`'s 5 deletion tests interact with the real custom `DeleteCertificateDialog` modal, not a native `confirm()`.
5. `tests/utils/api-helpers.ts` gains exactly one new function (`getBackupsViaAPI`), matching existing conventions.
6. Full Vitest suite: 0 failures, coverage ≥ 85%.
7. Full Playwright suite on chromium, firefox, **and** webkit: 0 failures (or any webkit-specific failures are explicitly triaged per §5.2's contingency, not silently skipped).
8. Backend coverage unchanged and re-confirmed ≥ targets (no backend files touched).
9. `lefthook run pre-commit` clean.
10. PR description matches the §6 scaffolding, giving issue #619 a complete, accurate paper trail per sub-issue.
11. No changes to `.gitignore`, `.dockerignore`, `codecov.yml`, or any `Dockerfile`.
