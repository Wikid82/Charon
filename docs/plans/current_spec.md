# Spec: Static Feedback Widget

**Status**: Draft
**Target**: Single PR — one atomic commit

---

## 1. Introduction

### Overview

Add a persistent, accessible feedback widget to every authenticated page in the Charon frontend. The widget appears as a small floating icon button anchored to the bottom-right corner of the viewport. When activated, it expands into a compact popover panel offering two GitHub Issue links:

- **Report a Bug** → `https://github.com/Wikid82/Charon/issues/new?template=bug_report.md`
- **Request a Feature** → `https://github.com/Wikid82/Charon/issues/new?template=feature_request.md`

Both links open in a new tab. The widget is rendered inside `Layout.tsx` so it appears on all authenticated routes but is absent from `/login`, `/setup`, and `/accept-invite`.

### Objectives

- Provide a low-friction path for users to report bugs or request features directly from the app
- Match existing UI design language (semantic tokens, Tailwind, Lucide icons)
- Meet WCAG 2.2 AA accessibility requirements
- Introduce zero new runtime dependencies
- Keep the widget unobtrusive — collapsed by default, non-blocking

---

## 2. Research Findings

### Architecture Summary

| Layer | Detail |
|---|---|
| Framework | React 19.2.3, TypeScript (strict), Vite 8 |
| Styling | Tailwind CSS 4.x with semantic CSS custom properties; `darkMode: 'class'` |
| Icons | `lucide-react` — used uniformly across all components |
| Accessible overlays | `@radix-ui/react-tooltip`, `@radix-ui/react-dialog` already installed |
| Classname utility | `cn()` at `frontend/src/utils/cn.ts` |
| Component variants | `class-variance-authority` (cva) |
| i18n | `react-i18next`, keys loaded from `src/locales/{locale}/translation.json` |
| Unit tests | Vitest 4 + React Testing Library; files in `src/components/__tests__/` |
| Layout entrypoint | `frontend/src/components/Layout.tsx` |

### Z-Index Hierarchy

| Element | z-index |
|---|---|
| Mobile overlay (backdrop) | `z-20` |
| Sidebar (`<aside>`) | `z-30` |
| Mobile header | `z-40` |
| Skip-to-content link (focus) | `z-50` |
| **Feedback Widget** | **`z-50`** ← must sit on top of sidebar and mobile header |

### Integration Point

`Layout.tsx` returns a single root `<div className="min-h-screen bg-light-bg dark:bg-dark-bg flex transition-colors duration-200">`. All sidebar, overlay, and `<main>` elements are children of this div. `<FeedbackWidget />` must be rendered as the **last child** of this root div. Because the widget uses `position: fixed`, its DOM position does not affect layout — it is always anchored to the viewport.

```tsx
// Layout.tsx — end of JSX return
return (
  <div className="min-h-screen bg-light-bg dark:bg-dark-bg flex transition-colors duration-200">
    {/* ... skip link, mobile header, sidebar, overlay, main ... */}
    <FeedbackWidget />   {/* ← insert here: after </main>, outside all header/sidebar branches */}
  </div>
)
```

> **Placement constraint**: `<FeedbackWidget />` must be placed after `</main>`, as the final sibling inside the root wrapper div. It must NOT be nested inside the mobile header branch, the desktop sidebar branch, or the `<main>` element itself.

### Existing Pattern: Self-Managed Popover

`NotificationCenter.tsx` is the primary pattern reference: a button toggles `isOpen` state to show/hide a floating panel (`absolute` positioned within a `relative` container). The feedback widget follows the same pattern but uses `fixed` positioning so it is viewport-anchored regardless of scroll position.

**No Radix Popover needed.** The NotificationCenter pattern is the reference implementation. NotificationCenter uses a backdrop `<div className="fixed inset-0 z-10" onClick={() => setIsOpen(false)}>` to handle click-outside dismissal — NOT a `useRef`/`useEffect` document-level event listener. The feedback widget uses the same backdrop approach for pattern consistency. Plain React `useState` is sufficient; no `@radix-ui/react-popover` needed.

### Tailwind Token Vocabulary

The existing components (Layout, NotificationCenter, Button) use the following token pattern consistently:

```
bg:      bg-white dark:bg-dark-card
border:  border-gray-200 dark:border-gray-800
text:    text-gray-700 dark:text-gray-300
hover:   hover:bg-gray-100 dark:hover:bg-gray-800
focus:   focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2
shadow:  shadow-md  /  shadow-lg
```

These are the canonical tokens used; the widget will follow this same vocabulary.

---

## 3. Technical Specifications

### 3.1 Component: `FeedbackWidget`

**File:** `frontend/src/components/FeedbackWidget.tsx`
**Test:** `frontend/src/components/__tests__/FeedbackWidget.test.tsx`

#### Props

None. The component is fully self-contained with no configuration props.

#### State and Refs

| Name | Type | Purpose |
|---|---|---|
| `isOpen` | `boolean` (useState) | Controls popover panel visibility |
| `triggerRef` | `useRef<HTMLButtonElement>` | Focus return target on panel close |
| `firstLinkRef` | `useRef<HTMLAnchorElement>` | Focus management: receives focus when panel opens |

**Focus-on-open mechanism:**

```tsx
const firstLinkRef = useRef<HTMLAnchorElement>(null)

useEffect(() => {
  if (isOpen) firstLinkRef.current?.focus()
}, [isOpen])
```

The `firstLinkRef` is attached to the first `<a>` element (Bug report link). When `isOpen` transitions to `true`, this effect fires and moves keyboard focus to that link, fulfilling WCAG 2.4.3 and ARIA authoring guidance for disclosure widgets.

**Click-outside mechanism** (matches NotificationCenter pattern):

```tsx
{isOpen && (
  <div
    className="fixed inset-0 z-10"
    aria-hidden="true"
    onClick={() => setIsOpen(false)}
  />
)}
```

The backdrop renders below the panel (`z-10` vs panel's higher stacking) and captures any click outside the widget.

#### Interaction Specification

1. User presses **Tab** → focus lands on the floating trigger button.
2. User presses **Enter** or **Space** (or clicks) → panel opens; focus moves to first link.
3. User presses **Tab** / **Shift+Tab** → navigate between the two links within the panel.
4. User presses **Enter** on a link → opens GitHub in a new tab; panel stays open.
5. User presses **Escape** → panel closes; focus returns to trigger button.
6. User clicks outside the widget → panel closes.
7. User presses **Tab** past last link → focus moves to next focusable element in page (natural DOM order, no trap).

#### ARIA Attributes

| Element | Attribute | Value |
|---|---|---|
| Trigger `<button>` | `aria-label` | dynamic: `t('feedback.triggerLabel')` when closed / `t('feedback.closeTriggerLabel')` when open |
| Trigger `<button>` | `aria-expanded` | `"true"` / `"false"` |
| Trigger `<button>` | `aria-controls` | `"feedback-panel"` |
| Panel `<nav>` | `id` | `"feedback-panel"` |
| Panel `<nav>` | `aria-label` | `t('feedback.panelLabel')` |
| Bug link `<a>` | `aria-label` | `t('feedback.reportBugAriaLabel')` |
| Feature link `<a>` | `aria-label` | `t('feedback.requestFeatureAriaLabel')` |

> **No `role="menu"` / `role="menuitem"`**: These ARIA roles require a custom arrow-key keyboard handler per the ARIA spec and are semantically incorrect for navigation links that open external URLs. Use a plain `<nav aria-label="...">` containing native `<a>` elements instead. Tab navigation between the two links is provided natively by the browser — no custom keyboard handler needed.

> **No `aria-haspopup`**: The ARIA `aria-haspopup` attribute signals a menu, listbox, tree, grid, or dialog. Since the panel is a `<nav>` (not a menu), `aria-haspopup` is omitted. `aria-expanded` alone is sufficient to communicate the toggle state.

#### CSS Layout

```
Wrapper:   position: fixed; bottom: 1.5rem; right: 1.5rem; z-index: 50
Trigger:   h-10 w-10 (40×40px, matches Button size="icon"), rounded-full
Panel:     position: absolute; bottom: calc(100% + 0.5rem); right: 0; width: 12rem
```

The panel is positioned relative to the fixed wrapper, appearing above the trigger.

#### Panel Animation

CSS transition using Tailwind. The panel conditional class changes based on `isOpen`:

| State | Classes |
|---|---|
| Open | `opacity-100 scale-100 pointer-events-auto` |
| Closed | `opacity-0 scale-95 pointer-events-none` |

Combined with `transition-all duration-150 ease-out origin-bottom-right` always applied.

#### URL Constants

Defined as module-level constants (not in a config file — they are static GitHub template URLs):

```ts
const GITHUB_BUG_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=bug_report.md'
const GITHUB_FEATURE_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=feature_request.md'
```

#### Lucide Icons

| Use | Icon | Available in lucide-react |
|---|---|---|
| Trigger button | `MessageSquarePlus` | ✅ (not yet imported anywhere) |
| Bug report link | `Bug` | ✅ (not yet imported anywhere) |
| Feature request link | `Sparkles` | ✅ (not yet imported anywhere) |

Single import: `import { MessageSquarePlus, Bug, Sparkles } from 'lucide-react'`

#### WCAG 2.2 AA Compliance Map

| Criterion | Requirement | Implementation |
|---|---|---|
| 1.1.1 Non-text Content | Icon button has text alternative | `aria-label` on trigger |
| 1.3.1 Info and Relationships | Programmatic structure | `<nav>` landmark with native `<a>` links; no synthetic ARIA roles needed |
| 1.4.3 Contrast (Minimum) | 4.5:1 for normal text | Tokens inherited from existing UI (brand-500 / dark-card) |
| 1.4.11 Non-text Contrast | 3:1 for UI components | Focus ring via `ring-brand-500` matches existing Button |
| 2.1.1 Keyboard | All functionality keyboard-operable | Enter/Space open; Tab/Shift+Tab navigate natively; Escape closes |
| 2.1.2 No Keyboard Trap | User can exit any component | No focus trap; Escape always returns focus to trigger |
| 2.4.3 Focus Order | Focus follows logical order | Widget last in DOM after `</main>`; focus moves to first link on open |
| 2.4.7 Focus Visible | Focus indicator visible | `focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2` |
| 2.4.11 Focus Appearance | Focus indicator meets minimum size/contrast | `focus-visible:ring-2 ring-brand-500 ring-offset-2` — same ring token used across all interactive elements in the codebase; meets 2px minimum requirement |
| 4.1.2 Name, Role, Value | All components correctly identified | Native `<button>`, native `<a>`, `<nav>` landmark, explicit `aria-label` and `aria-expanded` |

### 3.2 i18n Keys

Add a `"feedback"` object as a new top-level key in all five locale files.

**English (`en`) — source of truth:**

```json
"feedback": {
  "triggerLabel": "Open feedback menu",
  "closeTriggerLabel": "Close feedback menu",
  "panelLabel": "Feedback options",
  "reportBug": "Report a Bug",
  "reportBugDescription": "Found an issue?",
  "reportBugAriaLabel": "Report a bug (opens GitHub Issues in new tab)",
  "requestFeature": "Request a Feature",
  "requestFeatureDescription": "Have an idea?",
  "requestFeatureAriaLabel": "Request a feature (opens GitHub Issues in new tab)"
}
```

**Per-locale values (full table):**

| Key | de | es | fr | zh |
|---|---|---|---|---|
| `triggerLabel` | Feedback-Menü öffnen | Abrir menú de comentarios | Ouvrir le menu de retour | 打开反馈菜单 |
| `closeTriggerLabel` | Feedback-Menü schließen | Cerrar menú de comentarios | Fermer le menu de retour | 关闭反馈菜单 |
| `panelLabel` | Feedback-Optionen | Opciones de comentarios | Options de retour | 反馈选项 |
| `reportBug` | Fehler melden | Reportar un error | Signaler un bug | 报告错误 |
| `reportBugDescription` | Fehler gefunden? | ¿Encontraste un problema? | Trouvé un problème ? | 发现问题了吗？ |
| `reportBugAriaLabel` | Fehler melden (öffnet GitHub Issues im neuen Tab) | Reportar un error (abre GitHub Issues en nueva pestaña) | Signaler un bug (ouvre GitHub Issues dans un nouvel onglet) | 报告错误（在新标签页打开 GitHub Issues） |
| `requestFeature` | Funktion anfragen | Solicitar una función | Demander une fonctionnalité | 请求功能 |
| `requestFeatureDescription` | Eine Idee? | ¿Tienes una idea? | Vous avez une idée ? | 有想法吗？ |
| `requestFeatureAriaLabel` | Funktion anfragen (öffnet GitHub Issues im neuen Tab) | Solicitar función (abre GitHub Issues en nueva pestaña) | Demander une fonctionnalité (ouvre GitHub Issues dans un nouvel onglet) | 请求功能（在新标签页打开 GitHub Issues） |

### 3.3 `Layout.tsx` Changes

Two surgical changes:

1. **Import** (add to existing component imports, after `NotificationCenter`):
   ```tsx
   import FeedbackWidget from './FeedbackWidget'
   ```

2. **JSX** (add as last child of root wrapper div, **after** `</main>`, outside both mobile header and desktop sidebar branches):
   ```tsx
   <FeedbackWidget />
   ```

Total: 2 lines changed, 0 lines deleted.

---

## 4. Implementation Plan

### Phase 1 — Playwright E2E Tests (TDD — Written First)

**File:** `tests/feedback-widget.spec.ts`

Write the Playwright spec before writing the component. This defines the observable contract of the feature.

```
Test suite: Feedback Widget
  ✦ Trigger button is visible on the dashboard when authenticated
  ✦ Trigger button has accessible name "Open feedback menu"
  ✦ Trigger button has aria-expanded="false" by default
  ✦ Clicking the trigger opens the panel with two links
  ✦ Focus moves to the first link ("Report a Bug") when the panel opens
  ✦ "Report a Bug" link href points to GitHub bug template URL
  ✦ "Request a Feature" link href points to GitHub feature template URL
  ✦ Both links have target="_blank" and rel="noopener noreferrer"
  ✦ Pressing Escape closes the panel
  ✦ After Escape, focus returns to the trigger button
  ✦ Clicking outside the widget closes the panel
  ✦ Widget is NOT present on the /login page
```

Run target (after Docker rebuild): `npx playwright test tests/feedback-widget.spec.ts --project=firefox`

### Phase 2 — Backend

No backend changes required.

### Phase 3 — Frontend Implementation

Execute in order:

| Step | Task | Files |
|---|---|---|
| 3.1 | Create `FeedbackWidget.tsx` | `frontend/src/components/FeedbackWidget.tsx` |
| 3.2 | Add i18n keys | `frontend/src/locales/*/translation.json` (5 files) |
| 3.3 | Integrate into `Layout.tsx` | `frontend/src/components/Layout.tsx` |
| 3.4 | Write unit tests | `frontend/src/components/__tests__/FeedbackWidget.test.tsx` |

### Phase 4 — Integration and Testing

1. Run unit tests:
   ```
   cd /projects/Charon && npx vitest run frontend/src/components/__tests__/FeedbackWidget.test.tsx
   ```
2. TypeScript check:
   ```
   cd /projects/Charon/frontend && npx tsc --noEmit
   ```
3. Rebuild E2E Docker container:
   ```
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e
   ```
4. Run Playwright spec:
   ```
   npx playwright test tests/feedback-widget.spec.ts --project=firefox
   ```
5. Smoke test: Run a subset of existing non-security shards to ensure no regressions.

### Phase 5 — Documentation

No README or CHANGELOG updates required for this internal UI component.

---

## 5. Component Data Flow

```
User Tab-focuses trigger button (bottom-right, z-50, fixed)
     │
     ▼
User presses Enter/Space or clicks
     │
     ├── isOpen = false → isOpen = true
     │   Panel transitions: opacity-0 scale-95 → opacity-100 scale-100
     │   aria-expanded: "false" → "true"
     │   Focus moves to first <a> (Bug link)
     │
     ▼
User navigates links with Tab/Shift+Tab
     │
     ├── Enter on Bug link ─────────────────────────────────────────►
     │   window opens: https://github.com/Wikid82/Charon/issues/new?template=bug_report.md
     │   Panel stays open
     │
     ├── Enter on Feature link ──────────────────────────────────────►
     │   window opens: https://github.com/Wikid82/Charon/issues/new?template=feature_request.md
     │   Panel stays open
     │
     └── Escape key or click-outside
         isOpen = true → isOpen = false
         Panel transitions: opacity-100 → opacity-0 scale-95
         aria-expanded: "true" → "false"
         Focus returns to trigger button
```

### Phase 1: Playwright Tests

## 6. Unit Tests Specification

**File:** `frontend/src/components/__tests__/FeedbackWidget.test.tsx`

Pattern: follows `NotificationCenter.test.tsx` (no ThemeProvider needed, no QueryClient needed — uses plain `render` from Testing Library).

The global `react-i18next` mock in `test/setup.ts` loads real `en/translation.json`. Once the `feedback` key is added, `t('feedback.reportBug')` will return `"Report a Bug"`.

| # | Test Case | Assertion |
|---|---|---|
| 1 | Component renders | Trigger button is in the document |
| 2 | Default aria-label | `aria-label` = "Open feedback menu" |
| 3 | Default aria-expanded | `aria-expanded` = `"false"` |
| 4 | Panel hidden by default | Panel element has class `opacity-0` or `pointer-events-none` |
| 5 | Open on click | After click: `aria-expanded` = `"true"` |
| 6 | Bug link present | `getByRole('link', { name: /report a bug/i })` exists |
| 7 | Bug link href | Bug link `href` = `GITHUB_BUG_URL` |
| 8 | Feature link href | Feature link `href` = `GITHUB_FEATURE_URL` |
| 9 | Links open new tab | Both links have `target="_blank"` |
| 10 | Links are safe | Both links have `rel="noopener noreferrer"` |
| 11 | Escape closes panel | After Escape: `aria-expanded` = `"false"` |
| 12 | Focus returns on Escape | After Escape: `document.activeElement` = trigger button |
| 13 | Second click closes panel | Click → open; click again → `aria-expanded` = `"false"` |
| 14 | aria-label reflects state | When open, `aria-label` = "Close feedback menu" |
| 15 | Focus moves to first link on open | After click: `document.activeElement` = bug report `<a>` (`firstLinkRef.current`) |

1. Run backend: `bash scripts/go-test-coverage.sh`
2. Run frontend: `bash scripts/frontend-test-coverage.sh`
3. Run patch report: `bash scripts/local-patch-report.sh`
4. Review `test-results/local-patch-report.md` — all changed files must show ≥ 90%
5. If any gap remains, identify the specific uncovered block and add a targeted test

## 7. Acceptance Criteria

| # | Criterion | Verified By |
|---|---|---|
| AC-1 | Widget trigger visible on `/dashboard` when authenticated | Playwright |
| AC-2 | Widget absent from `/login`, `/setup`, `/accept-invite` | Playwright |
| AC-3 | Clicking trigger opens panel with two links | Playwright + Unit |
| AC-4 | Bug link href = GitHub bug template URL | Playwright + Unit |
| AC-5 | Feature link href = GitHub feature template URL | Playwright + Unit |
| AC-6 | Both links open in new tab | Playwright + Unit |
| AC-7 | Keyboard navigation: Tab, Enter, Escape all work | Playwright + Unit |
| AC-8 | Focus returns to trigger after Escape | Unit test #12 |
| AC-9 | Widget renders above all other elements (z-50) | Visual review |
| AC-10 | Dark mode renders correctly | Visual review |
| AC-11 | All unit tests pass (15 tests) | `vitest run` |
| AC-12 | No new runtime dependencies introduced | `package.json` diff |
| AC-13 | TypeScript strict mode: zero errors | `tsc --noEmit` |
| AC-14 | Clicking outside the widget closes the panel | Playwright |

### Definition of Done

- [ ] `FeedbackWidget.tsx` created, lint-clean, TypeScript error-free
- [ ] i18n keys added to all 5 locale files (en, de, es, fr, zh)
- [ ] `Layout.tsx` imports and renders `<FeedbackWidget />`
- [ ] `FeedbackWidget.test.tsx` written with all 15 test cases passing
- [ ] `tests/feedback-widget.spec.ts` Playwright spec written and passing on Firefox
- [ ] `tsc --noEmit` passes
- [ ] Visual review: widget visible bottom-right on dashboard in both light and dark mode
- [ ] GORM security scan: N/A (no backend models changed)

---

## 8. Commit Slicing Strategy

### Decision

**Single PR, single commit.** This feature is entirely frontend, confined to new files and two lines changed in `Layout.tsx`. No backend changes, no API changes, no schema migrations. One atomic commit is correct for this scope.

### Commit

```
feat(ui): add feedback widget with GitHub issue links

Add a persistent floating feedback widget to all authenticated pages.
The widget provides direct links to GitHub Issues for bug reports and
feature requests, opening each in a new browser tab. Implemented as a
self-contained fixed-position component integrated into Layout.tsx.

WCAG 2.2 AA: aria-expanded on trigger, <nav> landmark panel,
native <a> links (no role="menu"/"menuitem"), keyboard navigation
(Escape closes, Tab navigates natively, focus moves to first link
on open), focus management (returns to trigger on close), visible
focus ring (2.4.7 + 2.4.11).

Zero new runtime dependencies.

Closes #<issue-number>
```

### Files Changed

| File | Change |
|---|---|
| `frontend/src/components/FeedbackWidget.tsx` | New file |
| `frontend/src/components/__tests__/FeedbackWidget.test.tsx` | New file |
| `tests/feedback-widget.spec.ts` | New file |
| `frontend/src/components/Layout.tsx` | +2 lines (import + JSX element) |
| `frontend/src/locales/en/translation.json` | +10 lines (feedback key) |
| `frontend/src/locales/de/translation.json` | +10 lines |
| `frontend/src/locales/es/translation.json` | +10 lines |
| `frontend/src/locales/fr/translation.json` | +10 lines |
| `frontend/src/locales/zh/translation.json` | +10 lines |

### Rollback

Remove the `import FeedbackWidget from './FeedbackWidget'` and `<FeedbackWidget />` from `Layout.tsx`. The remaining new files can stay in place harmlessly. No database state, no backend state to revert.

### Validation Gates

1. `npx vitest run frontend/src/components/__tests__/FeedbackWidget.test.tsx` — 15 tests pass
2. `cd frontend && npx tsc --noEmit` — zero errors
3. `npx playwright test tests/feedback-widget.spec.ts --project=firefox` — spec passes
