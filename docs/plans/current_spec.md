# Technical Specification: TrafficVolumeChart Invisible Line Bug Fix

**Version:** 1.0
**Date:** 2026-06-17
**Branch:** `feature/stats`
**Status:** Draft

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specification](#3-technical-specification)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)

---

## 1. Introduction

### Overview

The Traffic Volume widget on the Dashboard page renders a chart container with correct axes, grid lines, and a functional hover tooltip, but the line itself is completely invisible. This means users see a blank white rectangle where the traffic trend line should appear, even when data is present and the tooltip confirms values on hover.

### Objectives

- Identify and fix the root cause of the invisible line in `TrafficVolumeChart`.
- Add a unit test assertion that guards against regression of this specific failure mode.
- Confirm tooltip behavior is unaffected by the fix.
- Add a Playwright E2E assertion that the chart SVG contains a rendered `<path>` element when data is present.

### Scope

- **In scope**: `frontend/src/components/stats/TrafficVolumeChart.tsx`, `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx`, `tests/stats.spec.ts`.
- **Out of scope**: Backend, other chart components, CSS design token definitions.

---

## 2. Research Findings

### 2.1 Component Architecture

`TrafficVolumeChart` is a pure presentational component located at `frontend/src/components/stats/TrafficVolumeChart.tsx`. It accepts `data: TrafficBucket[] | undefined`, `isLoading: boolean`, and `bucket: StatsBucket` props. It renders via Recharts `LineChart` + `Line`. The component is consumed in `frontend/src/pages/Dashboard.tsx` at line 325.

### 2.2 Chart Library

The project uses **Recharts v3.8.1** (`"recharts": "^3.8.1"` in `frontend/package.json`). Recharts renders charts as inline SVG. Visual properties such as `stroke` and `fill` on components like `<Line>` and `<Bar>` are passed directly as SVG presentation attributes — they are **not** resolved through the browser's CSS engine.

### 2.3 Root Cause: Invalid SVG Color Value via CSS Variable

The `<Line>` component in `TrafficVolumeChart.tsx` (line 135) specifies:

```tsx
stroke="var(--color-brand-500, #6366f1)"
```

The CSS custom property `--color-brand-500` is defined in `frontend/src/index.css` (line 88) as:

```css
--color-brand-500: 59 130 246;   /* #3b82f6 - Primary */
```

This is a **space-separated raw RGB triplet** — the design token format used by Tailwind v4's `rgb(var(--color-brand-500) / <alpha-value>)` utility syntax. It is NOT a valid standalone CSS `<color>` value.

When Recharts passes `stroke="var(--color-brand-500, #6366f1)"` to the SVG DOM as a presentation attribute (not a CSS `style` property), the browser resolves `--color-brand-500` to `"59 130 246"` (the defined value), discards the `#6366f1` fallback (because the variable IS defined), and produces an invalid SVG color string `"59 130 246"`. The SVG renderer ignores invalid `stroke` values and falls back to `"none"`, making the line invisible.

The tooltip still works because it is rendered as an HTML `<div>` element outside the SVG, driven by Recharts event handlers that receive the raw data regardless of visual rendering.

### 2.4 Confirmation via Comparison

All other working charts in the codebase use **literal hex color values** for SVG attributes:

| Component | Color Approach | Works |
|---|---|---|
| `TopHostsChart` | `HOST_COLORS = ['#6366f1', ...]` literal hex in `<Cell fill={}>` | Yes |
| `TopAttackingIPsChart` | `const BAR_COLOR = '#6366f1'` literal hex in `<Bar fill={BAR_COLOR}>` | Yes |
| `BanTimelineChart` | `const BAN_COLOR = '#3b82f6'` literal hex in `<Line stroke={BAN_COLOR}>` | Yes |
| `StatusDistributionChart` | `STATUS_COLORS` map with literal hex in `<Cell fill={}>` | Yes |
| `TrafficVolumeChart` | `var(--color-brand-500, #6366f1)` CSS variable in `<Line stroke>` | **Broken** |

Note: `BanTimelineChart` uses `'#3b82f6'` which is exactly the hex equivalent of `--color-brand-500: 59 130 246`.

### 2.5 Secondary Finding: activeDot Inherits Broken Stroke

The `<Line>` also defines `activeDot={{ r: 4 }}` without an explicit `fill`. Recharts derives the active dot fill from the Line's `stroke`. Because the stroke is invalid, the active dot's fill is also unset, making the hover indicator invisible as well (though the tooltip div itself still appears).

### 2.6 Existing Test Coverage Gap

The existing test file at `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx` mocks `ResponsiveContainer`, `YAxis`, and `Tooltip` but does **not** mock the `Line` component. This means no test currently asserts the `stroke` prop value on `<Line>`, and the bug could not be caught by the test suite.

### 2.7 Existing E2E Test Coverage Gap

`tests/stats.spec.ts` includes a "Traffic Volume chart container" test (line 194) that only checks for the card heading and presence of `<svg>` elements in the page. It does not verify that the SVG contains a rendered `<path>` element (which Recharts emits for each `<Line>`), so the invisibility bug is not caught there either.

---

## 3. Technical Specification

### 3.1 Fix Specification

**File**: `frontend/src/components/stats/TrafficVolumeChart.tsx`

**Change**: Replace the invalid CSS variable reference with a literal hex color constant, following the established pattern from `BanTimelineChart` and `TopAttackingIPsChart`.

**Before** (line 135):
```tsx
stroke="var(--color-brand-500, #6366f1)"
```

**After**:
```tsx
stroke={LINE_COLOR}
```

Where `LINE_COLOR` is declared as a module-level constant above the component function:
```tsx
const LINE_COLOR = '#3b82f6'
```

The value `'#3b82f6'` is chosen because it is:
- The exact hex equivalent of `--color-brand-500: 59 130 246` (confirmed in `index.css` comment).
- Consistent with `BanTimelineChart`'s `BAN_COLOR = '#3b82f6'`.
- The project's primary brand color.

No other changes to `TrafficVolumeChart.tsx` are needed.

### 3.2 Unit Test Changes

**File**: `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx`

Extend the existing Recharts mock to also capture and expose the `<Line>` component's props for inspection:

```tsx
// Add to the vi.mock('recharts', ...) return object:
Line: ({ stroke, strokeWidth, dataKey }: { stroke?: string; strokeWidth?: number; dataKey?: string }) => (
  <g data-testid="line" data-stroke={stroke} data-stroke-width={String(strokeWidth ?? '')} data-datakey={dataKey ?? ''} />
),
```

Add a new test case:

```ts
it('passes a valid hex color to the Line stroke prop', () => {
  render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

  const line = screen.getByTestId('line')
  const stroke = line.getAttribute('data-stroke')

  // Must be a valid hex color, not a CSS variable reference
  expect(stroke).toMatch(/^#[0-9a-f]{6}$/i)
  expect(stroke).toBe('#3b82f6')
})
```

Add a second new test case to guard against regression of tooltip behavior after the fix:

```ts
it('tooltip renders bytes value correctly when line has valid stroke', () => {
  render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

  // The mocked Tooltip fires content with 1_048_576 bytes
  expect(screen.getByText(/1\.0 MB sent/i)).toBeInTheDocument()
})
```

Note: The second test covers existing tooltip behavior and uses the already-mocked `Tooltip` that invokes `content?.({ active: true, payload: [{ value: 1_048_576, ... }] })`. The text "1.0 MB sent" is produced by `formatBytes(1_048_576)` + the tooltip template `{formatBytes(bytes)} sent`. This assertion was previously missing from the test suite.

### 3.3 Playwright E2E Test Changes

**File**: `tests/stats.spec.ts`

Extend the existing "should render the Traffic Volume chart container" test to add a step that verifies a Recharts line path is rendered when data is available.

New step to add within the existing test:

```ts
await test.step('Verify SVG line path is rendered inside the chart', async () => {
  // Recharts renders <path> elements inside the svg for each Line series.
  // This will only be reachable if actual traffic data exists in the E2E environment;
  // guard with a conditional check similar to the certificate expiry test.
  const chartCard = page.locator('text=Traffic Volume').first().locator('xpath=ancestor::*[contains(@class,"card") or @data-slot="card"][1]')

  // If the chart is showing the empty state, skip the SVG assertion
  const hasEmptyState = await page.getByText(/no data available yet/i).isVisible().catch(() => false)
  if (hasEmptyState) {
    // Empty state is acceptable; just confirm the card is shown
    await expect(page.getByText(/traffic volume/i).first()).toBeVisible()
    return
  }

  // When data is present, an SVG with recharts-line-curve path must exist
  const svgLineCount = await page.locator('.recharts-line-curve').count()
  expect(svgLineCount).toBeGreaterThan(0)
})
```

The `.recharts-line-curve` CSS class is the stable Recharts class applied to the `<path>` element of each `<Line>` series.

---

## 4. Implementation Plan

### Phase 1: Component Fix (single change, 1 file)

**Task**: Edit `frontend/src/components/stats/TrafficVolumeChart.tsx`.

Steps:
1. Add `const LINE_COLOR = '#3b82f6'` as a module-level constant immediately above the `formatBytes` function (consistent with the placement of `HOST_COLORS` in `TopHostsChart.tsx` and `BAR_COLOR` in `TopAttackingIPsChart.tsx`).
2. Replace the `stroke="var(--color-brand-500, #6366f1)"` JSX attribute on `<Line>` with `stroke={LINE_COLOR}`.
3. Do not modify any other props (`strokeWidth`, `dot`, `activeDot`, `type`, `dataKey`).

**Expected result**: The line is now visible in the browser with a blue stroke matching the brand color.

### Phase 2: Unit Test Update (1 file)

**Task**: Edit `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx`.

Steps:
1. Add `Line` to the Recharts mock in `vi.mock('recharts', ...)`, rendering a `<g>` test element that exposes `stroke` via `data-stroke` attribute.
2. Add the `'passes a valid hex color to the Line stroke prop'` test case.
3. Add the `'tooltip renders bytes value correctly when line has valid stroke'` test case.
4. Run `cd frontend && npm test` to confirm all tests pass.

### Phase 3: E2E Test Update (1 file)

**Task**: Edit `tests/stats.spec.ts`.

Steps:
1. Extend the `'should render the Traffic Volume chart container'` test with the new SVG path assertion step as specified in Section 3.3.
2. The new step is guarded by an empty-state check, so it will not fail in E2E environments with no traffic data.

### Phase 4: Verification

Run the full Definition of Done checklist:

1. `cd frontend && npm run type-check` — no errors.
2. `cd frontend && npm test` — all tests pass including new assertions.
3. `npx playwright test tests/stats.spec.ts --project=firefox` — all tests pass.
4. `cd frontend && npm run build` — production build succeeds.

---

## 5. Acceptance Criteria

| # | Criterion | Verification Method |
|---|---|---|
| AC-1 | The `<Line>` stroke prop value is `'#3b82f6'` (a valid hex color) | Unit test: `data-stroke` attribute assertion |
| AC-2 | The stroke value does NOT contain `var(` or CSS variable syntax | Unit test: regex assertion `expect(stroke).toMatch(/^#[0-9a-f]{6}$/i)` |
| AC-3 | Tooltip still renders byte values correctly | Unit test: `'1.0 MB sent'` text assertion |
| AC-4 | All existing TrafficVolumeChart unit tests still pass | `npm test` — zero failures |
| AC-5 | Empty state renders correctly when data is `[]` | Existing unit test passes |
| AC-6 | Loading state renders skeleton when `isLoading=true` | Existing unit test passes |
| AC-7 | When data exists in E2E environment, `recharts-line-curve` path element is present | Playwright test step passes |
| AC-8 | `npm run type-check` passes with zero errors | TypeScript compiler |
| AC-9 | `npm run build` succeeds | Vite build |

---

## 6. Commit Slicing Strategy

**Decision**: Single PR, single commit. The fix is a two-line change (one constant, one prop swap) in one component file, with accompanying test changes in two files. Splitting this across multiple commits would add overhead without review benefit.

### Commit 1 — Fix invisible line and add regression tests

**Scope**: Bug fix + test coverage

**Files changed**:
- `frontend/src/components/stats/TrafficVolumeChart.tsx` — add `LINE_COLOR` constant, replace CSS variable with literal hex on `<Line stroke>`
- `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx` — add `Line` mock, add two new test cases
- `tests/stats.spec.ts` — extend Traffic Volume E2E test with SVG path assertion

**Validation gate**: All unit tests pass, type-check passes, Playwright test passes.

**Commit message**:
```
fix(stats): replace invalid CSS variable with literal hex on TrafficVolumeChart Line stroke

The <Line stroke="var(--color-brand-500, #6366f1)"> prop was not working
because --color-brand-500 is defined as a space-separated RGB triplet
("59 130 246") for use with Tailwind's rgb(var(...) / alpha) syntax —
not as a valid SVG color string. Recharts passes stroke as an SVG
presentation attribute (not a CSS style), so the CSS variable resolved
to "59 130 246" which SVG treated as invalid and discarded, making the
line invisible. Tooltips continued to work because they are HTML elements.

Fix: introduce LINE_COLOR = '#3b82f6' (the exact hex of brand-500,
consistent with BanTimelineChart's BAN_COLOR) and use it as the stroke.

Adds unit test asserting stroke is a valid hex value and Playwright step
asserting the recharts-line-curve path is rendered when data is present.
```

### Rollback Notes

This commit makes no API changes, no database changes, and no infrastructure changes. Rollback is simply reverting the single commit. There is no risk of data loss or state corruption. The only observable effect of the broken state vs the fixed state is the visual rendering of the line in the chart.

---

## Appendix: File Reference Table

| File | Role | Change Type |
|---|---|---|
| `frontend/src/components/stats/TrafficVolumeChart.tsx` | Presentational component | Bug fix |
| `frontend/src/components/stats/__tests__/TrafficVolumeChart.test.tsx` | Vitest unit tests | New test cases + mock extension |
| `tests/stats.spec.ts` | Playwright E2E tests | New assertion step |
| `frontend/src/index.css` | CSS design tokens | Read-only reference |
| `frontend/src/components/stats/TopHostsChart.tsx` | Comparison reference | No change |
| `frontend/src/components/crowdsec/BanTimelineChart.tsx` | Comparison reference | No change |
