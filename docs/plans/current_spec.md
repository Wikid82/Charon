---
goal: Add tasteful, always-on support/donation links to the sidebar footer and README
version: 1.0
date_created: 2026-08-26
status: 'Planned'
tags: [chore, frontend, docs, funding]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Charon is self-hosted and always free. The maintainer's Buy Me a Coffee (`Wikid82`)
and GitHub Sponsors (`Wikid82`) accounts already exist and are already declared in
`.github/FUNDING.yml` (`buy_me_a_coffee: Wikid82`, `github: Wikid82`) — GitHub
already surfaces the "Sponsor" button on the repo from that file. This change adds
*visibility* for those two existing links in two places users actually look:

1. The app's sidebar footer (`frontend/src/components/Layout.tsx`), as two small
   icon links sitting quietly next to the existing version string.
2. `README.md`, as a small new "Support This Project" section using the same
   shields.io badge convention already used elsewhere in the file.

This is a frontend + docs **chore**. It is explicitly **not** a feature: no backend
code changes, no new settings/config/env vars, no database migration, no
AutoMigrate changes, and no toggle to hide the links. The links are always
rendered, unconditionally, for every user, matching the existing version-string
footer's own always-on behavior.

## Objective

- Add two small, muted icon links (Buy Me a Coffee, GitHub Sponsors) to the
  sidebar footer, styled identically to the existing version text (no badges,
  no color, no animation, no CTA banner).
- Add a new, separate "☕ Support This Project" section to `README.md`, and
  rename the existing "💬 Support" section (which is actually the GitHub Issues
  / help-tracker link, not a funding link) to avoid the naming collision.
- Cover the new footer links with a unit test extension in the existing
  `Layout.test.tsx` file.
- Touch nothing else: no backend, no new deps, no settings, no toggles.

---

## 1. Requirements & Constraints

- **REQ-001**: Sidebar footer must render a Buy Me a Coffee link pointing to
  `https://buymeacoffee.com/Wikid82`.
- **REQ-002**: Sidebar footer must render a GitHub Sponsors link pointing to
  `https://github.com/sponsors/Wikid82`.
- **REQ-003**: Both footer links open in a new tab: `target="_blank" rel="noopener noreferrer"`.
- **REQ-004**: Both footer links carry accessible labels: `aria-label="Support Charon on Buy Me a Coffee"` and `aria-label="Sponsor Charon on GitHub"` respectively.
- **REQ-005**: Both footer links must live inside the same collapsible block as
  the existing version text (the `<div>` at Layout.tsx line 349, class
  `` `mt-2 border-t border-border pt-4 shrink-0 ${isCollapsed ? 'hidden' : ''}` ``)
  so they hide/show in lockstep with the version string when the sidebar is
  collapsed. No new collapse-state logic.
- **REQ-006**: Footer links reuse the existing muted/small footer styling
  exactly — `text-content-muted` (icon color) and `text-xs`-scale sizing.
  No new colors, no pill/badge chrome, no animation/transition beyond the
  existing `hover:text-content-primary`-style pattern already used for other
  muted icon buttons in this file (e.g. line 406).
  **NOTE**: `text-content-muted` and `text-xs` are Tailwind utility classes
  already present in this file's design tokens — no new CSS/theme tokens are
  introduced.
- **REQ-007**: Footer links are always rendered — no feature flag, no settings
  lookup, no conditional based on `health`, auth state, or anything else.
- **REQ-008**: Use `lucide-react`'s `Coffee` and `Heart` icon components
  (already a project dependency, already imported in this file for `Menu`,
  `ChevronDown`, `ChevronRight`) instead of raw emoji, for visual consistency
  with the rest of the sidebar's icon usage.
- **REQ-009**: `Layout.test.tsx` must assert both new links render with the
  correct `href`, `aria-label`, and `target="_blank"` attributes, following
  the file's existing `renderWithProviders` + Testing Library query patterns.
- **REQ-010**: `README.md`'s existing "💬 Support" section (plain-text line 196
  + badge block lines 197–199, linking to GitHub Issues) must be renamed to
  "🐛 Get Help" (text only — no link/href changes) to remove the naming
  collision with the new funding section.
- **REQ-011**: A new, separate `README.md` section titled "☕ Support This
  Project" must be added near the renamed "🐛 Get Help" section, containing
  shields.io-style badge links to both `https://buymeacoffee.com/Wikid82`
  and `https://github.com/sponsors/Wikid82`, following the exact markup
  pattern already used by the top-of-file badge row (lines 16–26) and the
  existing Support block (lines 196–199): `<p align="center">` wrapping
  `<a href="..."><img src="https://img.shields.io/..." alt="..."></a>` pairs.
- **CON-001**: No backend/Go changes of any kind.
- **CON-002**: No new settings, config flags, environment variables, or
  database migrations.
- **CON-003**: No toggle/setting to hide the support links.
- **CON-004**: No changes to `.github/FUNDING.yml` (already correct).
- **CON-005**: No new npm/go dependencies (lucide-react is already installed).
- **CON-006**: No new Playwright/E2E specs (see Research Finding on existing
  E2E coverage below — none currently touches the footer/version area).
- **CON-007**: Commit message(s) must use the `chore:` prefix, per
  `CLAUDE.md` CI/CD conventions (`chore:` skips Docker image builds, which is
  correct for a docs+static-link change with no runtime behavior change).
- **SEC-001**: Both external links must use `rel="noopener noreferrer"` to
  prevent reverse-tabnabbing via `window.opener`.

---

## 2. Research Findings

### 2.1 Sidebar footer — `frontend/src/components/Layout.tsx`

**Imports (top of file, lines 1–17)**:

```tsx
import { useQuery } from '@tanstack/react-query'
import { Menu, ChevronDown, ChevronRight } from 'lucide-react'
import { type ReactNode, useState, useEffect, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation } from 'react-router'

import { useMediaQuery } from '../hooks/useMediaQuery'
import WhatsNewModal from './dialogs/WhatsNewModal'
import FeedbackWidget from './FeedbackWidget'
import NotificationCenter from './NotificationCenter'
import SystemStatus from './SystemStatus'
import { ThemeToggle } from './ThemeToggle'
import { Button } from './ui/Button'
import { getFeatureFlags } from '../api/featureFlags'
import { checkHealth } from '../api/health'
import { getSettings } from '../api/settings'
import { useAuth } from '../hooks/useAuth'
```

`lucide-react` is already imported for `Menu`, `ChevronDown`, `ChevronRight`.
The installed version is `lucide-react@1.34.0` (verified via
`frontend/node_modules/lucide-react/package.json`), which ships both a
`Coffee` icon (`coffee.mjs`) and a `Heart` icon (`heart.mjs`) — confirmed
present in `frontend/node_modules/lucide-react/dist/esm/icons/`. No new
dependency is required.

**Health query (lines 68–71)**, source of the `Version {health?.version}` text:

```tsx
const { data: health } = useQuery({
  queryKey: ['health'],
  queryFn: checkHealth,
  ...
})
```

`HealthResponse` (`frontend/src/api/health.ts`) has fields `status`, `service`,
`version`, `git_commit`, `build_time`. Not touched by this change — the new
links are static, unrelated to health data.

**Exact current footer JSX (lines 349–357)**, inside the sidebar's nav
container, immediately after the `<nav>` closing tag and before the
mobile-visible logout button block:

```tsx
<div className={`mt-2 border-t border-border pt-4 shrink-0 ${isCollapsed ? 'hidden' : ''}`}>
  <div className="text-xs text-content-muted text-center mb-2 flex flex-col gap-0.5">
    <span>Version {health?.version || 'dev'}</span>
    {health?.git_commit && health.git_commit !== 'unknown' && (
      <span className="text-[10px] opacity-75 font-mono">
        ({health.git_commit.substring(0, 7)})
      </span>
    )}
  </div>
  <button
    onClick={() => {
      setMobileSidebarOpen(false)
      logout()
    }}
    className="mt-3 w-full flex items-center justify-center gap-2 px-4 py-3 rounded-lg text-sm font-medium transition-colors text-error bg-error-muted hover:bg-error/20"
  >
    <span className="text-lg">🚪</span>
    {t('auth.logout')}
  </button>
</div>
```

Key facts to preserve exactly:
- The parent `<div>` (line 349) carries the collapse-hiding class
  `` `mt-2 border-t border-border pt-4 shrink-0 ${isCollapsed ? 'hidden' : ''}` ``.
  Anything placed as a sibling inside this parent div (i.e. inside the same
  `{isCollapsed ? 'hidden' : ''}` block, alongside the existing inner
  `text-xs text-content-muted` div and the logout `<button>`) will
  automatically hide when the sidebar is collapsed, matching REQ-005.
- The inner version-text div uses `text-xs text-content-muted text-center
  mb-2 flex flex-col gap-0.5` — this is the "small/quiet" style baseline
  referenced by REQ-006.
- A separate, independent "Collapsed Logout" block exists at lines 371–384
  (rendered only `{isCollapsed && (...)}`) with its own logout button — this
  is NOT part of the version-footer block and must NOT be touched; the new
  support links are only relevant in the expanded state per REQ-005 (they
  hide, not relocate, when collapsed — same behavior as the version text
  itself, which has no collapsed-state equivalent either).
- Icon-only muted buttons elsewhere in this file already use a `hover:text-*`
  transition pattern, e.g. line 405–409's sidebar-collapse toggle button:
  `className="p-2 rounded-lg text-content-muted hover:bg-surface-muted
  transition-colors"`. The new links should follow this same muted-icon
  hover convention rather than inventing new styling.

### 2.2 Existing test file — `frontend/src/components/__tests__/Layout.test.tsx`

Located at `frontend/src/components/__tests__/Layout.test.tsx` (521 lines).

Conventions observed:
- Testing library: Vitest (`describe`, `it`, `expect`, `vi`, `beforeEach`,
  `afterEach` from `'vitest'`) + `@testing-library/react`
  (`render`, `screen`, `waitFor`) + `@testing-library/user-event`.
- Render helper: `renderWithProviders(children: ReactNode)` (lines 58–76)
  wraps in `QueryClientProvider` → `BrowserRouter` → `ThemeProvider`. All new
  tests must use this helper, not raw `render`.
- API mocks are set up via `vi.mock(...)` at module scope (lines 15–56),
  including a `checkHealth` mock (lines 27–32) resolving
  `{ version: '0.1.0', git_commit: 'abcdef1' }`. No changes to these mocks are
  needed for this task — the new links are static and independent of health
  data.
- Existing version-footer test, to model the new test after (lines 179–187):

  ```tsx
  it('displays version information', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(await screen.findByText('Version 0.1.0')).toBeInTheDocument()
  })
  ```

- Attribute-assertion style used elsewhere in the file (e.g. lines 163–164,
  aria-current tests at 413–518) queries by role/name then asserts on
  `.getAttribute(...)` or `toHaveAttribute(...)`:

  ```tsx
  const crowdSecLinks = await screen.findAllByRole('link', { name: 'CrowdSec' })
  expect(crowdSecLinks.some(link => link.getAttribute('href') === '/tasks/import/crowdsec')).toBe(true)
  ```

  and

  ```tsx
  const link = await screen.findByTitle('Hecate')
  expect(link).toHaveAttribute('aria-current', 'page')
  ```

  New assertions should follow this `toHaveAttribute` pattern, querying by
  `getByRole('link', { name: <aria-label text> })` since the new links'
  accessible name comes from `aria-label` (REQ-004), not visible text.

### 2.3 `README.md` structure

Full file is 209 lines. Real markdown headings (confirmed via `grep -n "^#"`):
`##` at lines 30, 50, 66; `###` at lines 68, 109, 115, 121, 129, 135, 139,
143, 147, 151, 155, 159, 163, 167, 177, 181, 185, 189. **Lines 196 ("💬
Support") and 203 ("❤️ Free & Open Source") are NOT markdown headings** —
they are plain text lines with no leading `#`, confirmed via raw byte
inspection (`cat -A`). This is existing (pre-change) inconsistency in the
file, not something this task should silently "fix" beyond the rename — the
plan preserves the exact plain-text (non-heading) style for both the renamed
section and the new section, to keep this a minimal, low-risk chore diff
rather than a drive-by markdown-heading refactor.

**Top-of-file badge row (lines 16–26)** — the established badge pattern to
replicate:

```markdown
<p align="center">
  <a href="https://hub.docker.com/r/wikid82/charon">
    <img src="https://img.shields.io/docker/pulls/wikid82/charon.svg" alt="Docker Pulls">
  </a>
  <a href="https://github.com/Wikid82/charon/releases">
    <img src="https://img.shields.io/github/v/release/Wikid82/charon?include_prereleases" alt="Latest Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT License">
  </a>
</p>
```

**Existing "Support" block, exact current text (lines 195–200)**:

```markdown
---
💬 Support
<p align="center">   <a href="https://github.com/Wikid82/Charon/issues">
    <img alt="GitHub issues"
         src="https://img.shields.io/github/issues/Wikid82/Charon"><a href="https://github.com/Wikid82/Charon/issues/new/choose"> <img src="https://img.shields.io/badge/Support-Open%20Issue-blue?logo=github"></a> </p>

---
```

This block already uses the `img.shields.io/badge/...` custom-badge pattern
(`https://img.shields.io/badge/Support-Open%20Issue-blue?logo=github`) in
addition to the dynamic `img.shields.io/github/issues/...` badge — both
styles are established precedent in this file, so either is acceptable for
the new funding badges; the plan below uses the static
`img.shields.io/badge/...?logo=...` style since shields.io has no dynamic
"buy me a coffee" or "GitHub sponsors count" endpoint in use elsewhere in
this repo.

**End of file (lines 201–209)**, unchanged, for placement context:

```markdown
---

❤️ Free & Open Source

Charon is 100% free and open source under the MIT License.

No premium tiers. No locked features. No usage limits.

Built for the self-hosting community.
```

**Discord**: `grep -n "Discord\|discord" README.md` returns zero matches.
Confirmed no Discord references exist in `README.md` — no Discord-related
edit is in scope, per task instructions.

**`.github/FUNDING.yml`** (unchanged by this task, shown for reference only):

```yaml
github: Wikid82
buy_me_a_coffee: Wikid82
```

Both usernames match what this plan links to (`Wikid82`), confirming no
typos/mismatches between the two.

### 2.4 Existing E2E (Playwright) coverage

Searched `tests/` for any spec referencing the sidebar footer, version
string, or `sidebarCollapsed` behavior:

```
grep -rln "Version\|sidebar" tests/     → 15 files, all navigation/feature specs
                                            (e.g. tests/core/navigation.spec.ts),
                                            none assert on the version-footer
                                            block or its contents
grep -n "Version\|footer\|sidebarCollapsed" tests/core/navigation.spec.ts
                                         → no matches
grep -n "buymeacoffee\|sponsor" tests/  → no matches anywhere
```

**Conclusion**: No existing Playwright spec touches the footer/version area.
Per task scope, **no new or modified E2E test is planned**. This is
consistent with CLAUDE.md's Definition of Done, scaled down here since this
change adds no new interactive flow, only two static anchor tags.

### 2.5 Ignore-file / build-file check (per standard process)

Checked `.gitignore`, `.dockerignore`, `codecov.yml`, `Dockerfile` — none of
the four exist for `.codecov.yml` (repo uses `codecov.yml`, no leading dot).
**Confirmed no changes required to any of these files**: this task adds no
new files, no new directories, no new build artifacts, and no new
dependencies — it only edits two existing text files
(`frontend/src/components/Layout.tsx`, `README.md`) and extends one existing
test file (`frontend/src/components/__tests__/Layout.test.tsx`). There is
nothing for an ignore file, coverage config, or Dockerfile to account for.

---

## 3. Technical Specifications

### 3.1 Component design — `Layout.tsx` footer links

Add `Coffee` and `Heart` to the existing `lucide-react` import (line 2):

```tsx
import { Menu, ChevronDown, ChevronRight, Coffee, Heart } from 'lucide-react'
```

Insert a new sibling `<div>` directly after the existing version-text `<div>`
(after line 357's closing `</div>`) and before the logout `<button>` (line
358), still inside the same collapse-hiding parent `<div>` (line 349):

```tsx
<div className={`mt-2 border-t border-border pt-4 shrink-0 ${isCollapsed ? 'hidden' : ''}`}>
  <div className="text-xs text-content-muted text-center mb-2 flex flex-col gap-0.5">
    <span>Version {health?.version || 'dev'}</span>
    {health?.git_commit && health.git_commit !== 'unknown' && (
      <span className="text-[10px] opacity-75 font-mono">
        ({health.git_commit.substring(0, 7)})
      </span>
    )}
  </div>
  <div className="flex items-center justify-center gap-3 text-content-muted">
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
  </div>
  <button
    onClick={() => {
      setMobileSidebarOpen(false)
      logout()
    }}
    className="mt-3 w-full flex items-center justify-center gap-2 px-4 py-3 rounded-lg text-sm font-medium transition-colors text-error bg-error-muted hover:bg-error/20"
  >
    <span className="text-lg">🚪</span>
    {t('auth.logout')}
  </button>
</div>
```

Design notes:
- `text-content-muted` on the wrapping `<div>` sets the icon color to match
  the existing muted footer text (REQ-006); `hover:text-content-primary
  transition-colors` on each `<a>` mirrors the existing hover convention
  used elsewhere in this file (e.g. the collapse-toggle button at line 406).
- `w-4 h-4` sizes the icons small/quiet, visually consistent with the
  `text-xs`/`text-[10px]` scale of the adjacent version text — no icon
  exceeds the visual weight of the text it sits beside.
- No `<span>` text labels are added next to the icons — the accessible name
  comes entirely from `aria-label` on each `<a>` (REQ-004), keeping the
  footer visually unchanged in density/height beyond one small icon row.
- Placed between the version text and the logout button (not after the
  logout button) so the block reads top-to-bottom as: version info →
  support links → logout, an ordering that keeps the "quiet, informational"
  items grouped together above the "destructive/action" logout button.
- No `i18n`/`t()` wrapping is introduced for the `aria-label` strings — the
  task requirements specify exact English `aria-label` text, and this
  matches how the sibling `Version {health?.version}` text is also plain
  English rather than translated. If the project later wants i18n coverage
  for this string, that is out of scope for this chore.

### 3.2 Test design — `Layout.test.tsx`

Add a new test (or a `describe('Support links', ...)` block) after the
existing `'displays version information'` test (after line 187), following
the file's existing `renderWithProviders` + `findByRole` patterns:

```tsx
it('renders support/donation links in the sidebar footer', async () => {
  renderWithProviders(
    <Layout>
      <div>Test Content</div>
    </Layout>
  )

  const coffeeLink = await screen.findByRole('link', {
    name: 'Support Charon on Buy Me a Coffee',
  })
  expect(coffeeLink).toHaveAttribute('href', 'https://buymeacoffee.com/Wikid82')
  expect(coffeeLink).toHaveAttribute('target', '_blank')
  expect(coffeeLink).toHaveAttribute('rel', 'noopener noreferrer')

  const sponsorLink = await screen.findByRole('link', {
    name: 'Sponsor Charon on GitHub',
  })
  expect(sponsorLink).toHaveAttribute('href', 'https://github.com/sponsors/Wikid82')
  expect(sponsorLink).toHaveAttribute('target', '_blank')
  expect(sponsorLink).toHaveAttribute('rel', 'noopener noreferrer')
})
```

No new mocks are required — the links are static and unrelated to any
mocked API. No changes to `beforeEach`/`vi.mock` blocks.

### 3.3 `README.md` edits

**Edit 1 — rename the Get Help section (was "💬 Support", lines 195–200)**:

```diff
 ---
-💬 Support
+🐛 Get Help
 <p align="center">   <a href="https://github.com/Wikid82/Charon/issues">
     <img alt="GitHub issues"
          src="https://img.shields.io/github/issues/Wikid82/Charon"><a href="https://github.com/Wikid82/Charon/issues/new/choose"> <img src="https://img.shields.io/badge/Support-Open%20Issue-blue?logo=github"></a> </p>

 ---
```

Only the heading text changes (`💬 Support` → `🐛 Get Help`); the badge
markup, hrefs, and everything else on those lines is untouched.

**Edit 2 — insert a new "☕ Support This Project" section**, placed
immediately after the renamed "🐛 Get Help" block's closing `---` (i.e.
after what is currently line 201) and before the closing "❤️ Free & Open
Source" block:

```markdown
☕ Support This Project
<p align="center">
  <a href="https://buymeacoffee.com/Wikid82">
    <img src="https://img.shields.io/badge/Buy%20Me%20A%20Coffee-donate-yellow?logo=buy-me-a-coffee&logoColor=white">
  </a>
  <a href="https://github.com/sponsors/Wikid82">
    <img src="https://img.shields.io/badge/GitHub%20Sponsors-sponsor-EA4AAA?logo=github-sponsors&logoColor=white">
  </a>
</p>

---
```

Resulting file order (lines 195 onward, after both edits):

```
---
🐛 Get Help
<p align="center"> ...GitHub Issues badges... </p>

---
☕ Support This Project
<p align="center"> ...Buy Me a Coffee + GitHub Sponsors badges... </p>

---

❤️ Free & Open Source
...
```

This mirrors the existing plain-text-heading + `<p align="center">` badge
block pattern exactly (no new heading level, no new visual pattern
introduced), keeping the diff minimal and consistent with REQ-011/CON-007's
"chore-sized" framing.

---

## 4. Data Flow

None. This is a static-content change — no data enters, transforms, or
persists anywhere. The two new anchor tags in `Layout.tsx` are plain
hyperlinks to external URLs; the two new README badges are static markdown
image/link markup rendered by GitHub's own README renderer. No API calls, no
new queries, no new state.

## 5. Error Handling

None applicable — no new runtime logic, no new failure modes. The links are
unconditional static markup; there is nothing to fail at runtime (a 404 on
`buymeacoffee.com`/`github.com/sponsors/...` if the accounts were ever
removed is an operational concern for the maintainer, not a code-handled
error case, consistent with how the existing GitHub Issues support link is
also unguarded).

---

## 6. Implementation Plan

### Phase 1: Playwright Tests (spec behavior)

- GOAL-001: Confirm no new E2E coverage is needed; make no E2E changes.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | Re-confirm (already done in Research 2.4) that no existing Playwright spec under `tests/` asserts on the sidebar footer/version block; if one is found during implementation that would break due to the new links (e.g. a strict DOM snapshot), extend that spec minimally — otherwise make no Playwright changes. | | |

This phase is intentionally a verification step, not a build step — per task
scope, no new `.spec.ts` file is created and `test.fixme` scaffolding is not
applicable here since there is no new user flow to gate.

### Phase 2: Backend Implementation

- GOAL-002: N/A — explicitly out of scope.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| N/A | No backend/Go changes of any kind (CON-001). Skip this phase entirely. | | |

### Phase 3: Frontend Implementation

- GOAL-003: Add the two support links to the sidebar footer and cover them
  with a unit test.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-002 | In `frontend/src/components/Layout.tsx` line 2, add `Coffee, Heart` to the existing `lucide-react` import. | | |
| TASK-003 | In `frontend/src/components/Layout.tsx`, insert the new icon-link `<div>` (see §3.1) between the version-text `<div>` (ends line 357) and the logout `<button>` (starts line 358), inside the existing collapse-hiding parent `<div>` (line 349). | | |
| TASK-004 | In `frontend/src/components/__tests__/Layout.test.tsx`, add the new test from §3.2 after the existing `'displays version information'` test (after line 187). | | |
| TASK-005 | Run `cd frontend && npx vitest run src/components/__tests__/Layout.test.tsx` — all tests in the file, including the new one, must pass. | | |
| TASK-006 | Run `cd frontend && npm run type-check` — zero type errors. | | |
| TASK-007 | Run `cd frontend && npm run build` — build must succeed. | | |

### Phase 4: Documentation (README)

- GOAL-004: Rename the Get Help section and add the new Support section.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-008 | In `README.md`, apply Edit 1 (§3.3): rename `💬 Support` (line 196) to `🐛 Get Help`. No other changes to that block. | | |
| TASK-009 | In `README.md`, apply Edit 2 (§3.3): insert the new `☕ Support This Project` block immediately after the renamed Get Help block. | | |
| TASK-010 | Manually re-read the full modified `README.md` section (former lines 195–209) to confirm markdown renders sanely (badge `<img>` tags inside `<a>` tags are well-formed, no stray unclosed tags) — this file already has pre-existing minor markup roughness (e.g. the nested/malformed anchor on line 199) that this task does not need to fix, but new markup added by this task must itself be well-formed. | | |
| TASK-011 | Confirm (already done in Research 2.3) that no Discord references exist in `README.md` and that no other doc files (`docs/api.md`, `docs/security.md`, `docs/live-logs-guide.md`, `docs/security-incident-response.md`, `docs/troubleshooting/dns-challenges.md`, `docs/features.md`) are touched. | | |

### Phase 5: Integration, Validation, and Cleanup

- GOAL-005: Run the scaled-down Definition of Done for a docs+frontend-only
  chore with no new feature surface.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-012 | `lefthook run pre-commit` — must pass (staticcheck N/A since no Go files change, but frontend lint/format hooks still run). | | |
| TASK-013 | Re-confirm `.gitignore`, `.dockerignore`, `codecov.yml`, `Dockerfile` need no changes (already confirmed in Research 2.5 — no new files, dirs, or deps). | | |
| TASK-014 | Skip CodeQL/Trivy/GORM scans — no new feature surface, no Go changes, per task scope and CLAUDE.md DoD step 3's `fix:`/`chore:` deferral rule. CI runs these unconditionally regardless. | | |
| TASK-015 | Skip `scripts/go-test-coverage.sh` and the 85% backend coverage gate — no backend files change. Frontend coverage (`scripts/frontend-test-coverage.sh`) should still be run since `Layout.tsx`/`Layout.test.tsx` changed; confirm the new lines are covered by the new test. | | |
| TASK-015b | Run `bash scripts/local-patch-report.sh` from repo root (CLAUDE.md DoD step 2, "Local Patch Coverage Preflight") — this gate is unconditionally MANDATORY with **no** `chore:`/`fix:` exemption (unlike DoD step 3's CodeQL/Trivy, which is explicitly deferrable for chore-scoped changes). Confirm it produces `test-results/local-patch-report.md` and `test-results/local-patch-report.json`. | | |
| TASK-016 | Final visual sanity check: with the dev server running, confirm the two new icons render at the correct size/color in both expanded and collapsed sidebar states (collapsed: hidden, matching version text) and in both light/dark theme (uses theme-aware `text-content-muted`/`text-content-primary` tokens already in use, so no new theme-specific CSS is needed). | | |

---

## 7. Acceptance Criteria

- [ ] Sidebar footer (expanded state) shows a Coffee icon link
      (`href="https://buymeacoffee.com/Wikid82"`) and a Heart icon link
      (`href="https://github.com/sponsors/Wikid82"`), both `target="_blank"
      rel="noopener noreferrer"`, with the exact `aria-label`s specified in
      REQ-004.
- [ ] Both links are visually muted/small (no color other than
      `text-content-muted`/hover `text-content-primary`, no badge/pill
      chrome, no animation) and hide when the sidebar is collapsed,
      identically to the adjacent version text.
- [ ] Links are unconditionally rendered — no settings lookup, no feature
      flag, no auth-state gate.
- [ ] `Layout.test.tsx` has a new passing test asserting both links' `href`,
      `aria-label`-derived accessible name, and `target` attributes.
- [ ] `README.md`'s former "💬 Support" section reads "🐛 Get Help" with its
      GitHub Issues badges/links unchanged.
- [ ] `README.md` has a new "☕ Support This Project" section with working
      badge links to both `https://buymeacoffee.com/Wikid82` and
      `https://github.com/sponsors/Wikid82`.
- [ ] No Discord edits, no edits to `docs/api.md`, `docs/security.md`,
      `docs/live-logs-guide.md`, `docs/security-incident-response.md`,
      `docs/troubleshooting/dns-challenges.md`, or `docs/features.md`.
- [ ] No backend/Go files changed. `git diff --stat` shows only
      `frontend/src/components/Layout.tsx`,
      `frontend/src/components/__tests__/Layout.test.tsx`, and `README.md`.
- [ ] `.github/FUNDING.yml` unchanged.
- [ ] No new settings/config/database/migration code anywhere.
- [ ] `cd frontend && npx vitest run src/components/__tests__/Layout.test.tsx` passes.
- [ ] `cd frontend && npm run type-check` passes with zero errors.
- [ ] `cd frontend && npm run build` succeeds.
- [ ] `lefthook run pre-commit` passes.
- [ ] Frontend coverage script confirms new lines are covered (no coverage
      regression).
- [ ] Final commit message(s) start with `chore:`.

---

## 8. Commit Slicing Strategy

**Decision**: Single PR, one feature = one PR (per `CLAUDE.md`), with two
tightly-scoped, ordered `chore:`-prefixed commits. This is small enough that
it does not need the full 5-commit suggested sequence (E2E → foundation →
backend → frontend → hardening) from `CLAUDE.md` — there is no backend, no
foundation/contract change, and no new E2E behavior to gate — so it is
sliced by concern (app code vs. docs) instead, which keeps each commit
independently reviewable and independently revertible.

### Commit 1 — Frontend: sidebar footer support links + unit test

- **Message**: `chore: add support links to sidebar footer`
- **Scope**: `frontend/src/components/Layout.tsx`,
  `frontend/src/components/__tests__/Layout.test.tsx`
- **Contains**: TASK-002, TASK-003, TASK-004 (implementation + test),
  TASK-005–TASK-007 (validation).
- **Dependencies**: None (first commit).
- **Validation gate**:
  - `cd frontend && npx vitest run src/components/__tests__/Layout.test.tsx` → all pass
  - `cd frontend && npm run type-check` → zero errors
  - `cd frontend && npm run build` → succeeds
  - `lefthook run pre-commit` → passes

### Commit 2 — Docs: rename Get Help section + add Support section to README

- **Message**: `chore: add support/donation section to README`
- **Scope**: `README.md`
- **Contains**: TASK-008, TASK-009, TASK-010, TASK-011.
- **Dependencies**: None functionally, but ordered after Commit 1 so the PR
  reads "app behavior, then docs describing it" — reviewers see the actual
  UI change before the README claims it exists.
- **Validation gate**:
  - Manual markdown re-read (TASK-010) — no broken `<a>`/`<img>` markup
  - `grep -c "Discord" README.md` → `0` (re-confirms CON-004-adjacent scope
    boundary, i.e. still no Discord content introduced)

### Rollback / contingency notes (whole PR)

- **Rollback**: Both commits are additive/renaming only — no schema, no
  migration, no config. Reverting is a plain `git revert` of either or both
  commits with zero data-loss or state-migration risk, since nothing
  persists any new data.
- **Contingency — icon choice**: If `Coffee`/`Heart` from `lucide-react`
  are found at implementation time to look visually off at `w-4 h-4` next to
  the existing `text-lg` emoji used for the logout button (🚪), fall back to
  plain emoji glyphs (☕ and 💜) wrapped in the same `<a>` markup — this only
  changes the icon rendering inside Commit 1, no other requirement changes.
- **Contingency — README placement**: If review feedback prefers the new
  "☕ Support This Project" section elsewhere (e.g. directly under the
  top-of-file badge row instead of near "🐛 Get Help"), that is a
  same-commit markdown-only adjustment within Commit 2, not a scope change.
- **No feature flag exists or is needed** to disable this — if the
  maintainer ever wants the links gone, the correct action is a follow-up
  revert-style chore commit, not a runtime toggle (per CON-003, no toggle is
  being built).

---

## 9. Dependencies

- **DEP-001**: `lucide-react@1.34.0` — already an installed frontend
  dependency; no version bump needed; `Coffee` and `Heart` icons confirmed
  present in the installed package.
- **DEP-002**: None else. No new npm packages, no new Go modules.

## 10. Files

- **FILE-001**: `frontend/src/components/Layout.tsx` — add `Coffee, Heart`
  to the `lucide-react` import (line 2); insert new icon-link `<div>`
  between lines 357 and 358.
- **FILE-002**: `frontend/src/components/__tests__/Layout.test.tsx` — add
  one new test after line 187 asserting the two new links' `href`,
  `aria-label`, and `target` attributes.
- **FILE-003**: `README.md` — rename line 196 text; insert new
  "☕ Support This Project" block after the renamed section's trailing `---`
  (after former line 201).

## 11. Testing

- **TEST-001**: New Vitest test in `Layout.test.tsx` (§3.2) — asserts both
  links render with correct `href`, `aria-label`-derived accessible name,
  `target="_blank"`, and `rel="noopener noreferrer"`.
- **TEST-002**: Existing `Layout.test.tsx` suite (all other tests) must
  continue to pass unmodified — the new markup is additive and does not
  change any existing queried text/role/testid.
- **TEST-003**: No new Playwright/E2E test (see Research 2.4 — no existing
  spec covers this area; none is added per task scope).
- **TEST-004**: Frontend coverage script
  (`scripts/frontend-test-coverage.sh`) run to confirm the new lines in
  `Layout.tsx` are exercised by TEST-001 and no coverage regression is
  introduced.

## 12. Risks & Assumptions

- **RISK-001**: `lucide-react`'s `Coffee`/`Heart` icons might not visually
  match the "tasteful, non-intrusive" intent at small size. Mitigated by the
  Commit 1 contingency (fall back to emoji) documented in §8.
- **RISK-002**: The pre-existing malformed nested-anchor markup in the
  original "💬 Support" block (line 199: an `<a>` opened inside another
  `<a>`'s `<img>`) is left as-is by Edit 1 (only the heading text changes).
  This is pre-existing technical debt in `README.md`, not introduced or
  worsened by this task; fixing it is out of scope for this chore (flagged
  here for visibility, not for action).
- **RISK-003**: If a future Playwright spec is added that snapshots the full
  sidebar DOM (none currently does, per Research 2.4), it could break on the
  new markup. Not a risk to *this* change, but worth noting for QA
  awareness.
- **ASSUMPTION-001**: `buymeacoffee.com/Wikid82` and
  `github.com/sponsors/Wikid82` are both live, correct URLs — consistent
  with the usernames already present in `.github/FUNDING.yml`
  (`buy_me_a_coffee: Wikid82`, `github: Wikid82`). This plan does not
  independently verify external reachability of those third-party URLs
  (out of scope for a static-link chore); if either account does not exist
  the link is still valid markup, just would 404 externally — the
  maintainer's responsibility, not a code defect.
- **ASSUMPTION-002**: The project's Tailwind config already defines
  `content-muted` and `content-primary` as theme-aware color tokens (used
  extensively elsewhere in `Layout.tsx`), so no new Tailwind/theme config
  changes are needed for light/dark support of the new links.

## 13. Related Specifications / Further Reading

- `.github/FUNDING.yml` — source of truth for the two account usernames
  used throughout this plan.
- `frontend/src/components/Layout.tsx` — component being modified.
- `frontend/src/components/__tests__/Layout.test.tsx` — test file being
  extended.
- `README.md` — doc file being modified.
- `CLAUDE.md` §"Commit Slicing & PR Strategy" and §"Task Completion
  Protocol (Definition of Done)" — governs the scaled-down DoD and
  commit-slicing approach used in §8 of this plan.
