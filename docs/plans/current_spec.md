# Spec: Proxy Host Group Selector + Grouped-View Row Layout Fix (Issue #1367)

## 1. Introduction

### 1.1 Overview

GitHub issue #1367 reports two problems on the Proxy Hosts page, both on the
proxy-host-group UI surface:

1. **No group selector in the per-host create/edit form.** Groups are fully
   implemented end-to-end (backend CRUD, bulk group assignment, list-page
   drag-and-drop between group sections) except in the single-host dialog
   (`ProxyHostForm.tsx`), which has zero group-related fields. Users cannot
   see or set a host's group while creating/editing it there — they must
   leave the dialog and use drag-and-drop or the bulk "Assign to group"
   flow on the list page.
2. **Row layout breaks in grouped view.** When the Proxy Hosts list is
   rendered grouped (one or more proxy groups exist), rows are visually
   cramped at the right edge — the Actions column (Edit/Delete) sits
   uncomfortably close to the row edge, with no horizontal scrollbar
   appearing as a fallback.

Both problems live on the same page (`frontend/src/pages/ProxyHosts.tsx`)
and the same UI surface (proxy-host group presentation), so per
`CLAUDE.md`'s "One Feature = One PR" rule, they are fixed together in a
single PR with commits sliced by concern.

### 1.2 Objectives

- Add a group selector to `ProxyHostForm.tsx` so a host's group can be set
  or changed from the create/edit dialog, using the existing
  `proxy_group_id` field and existing `/proxy-groups` API — no new backend
  endpoints.
- Fix the grouped-view row layout so all columns (through Actions) fit
  comfortably at common desktop viewport widths, with a horizontal-scroll
  fallback as a secondary safety net at narrow widths, not the primary fix.
- Ship both with unit tests (Vitest) and Playwright E2E coverage, per the
  Definition of Done.

### 1.3 Non-Goals

- No backend/schema changes are anticipated (confirmed during research —
  see §2.3). If implementation discovers a real gap, it must be called out
  explicitly and added as its own commit before the frontend commits.
- No redesign of the drag-and-drop group-assignment UX, the bulk "Assign to
  group" dialog, or `ManageGroupsDialog.tsx` — those already work and are
  out of scope.
- No change to how groups are created/edited/deleted.

## 2. Research Findings

### 2.1 Existing architecture (groups end-to-end)

- **Model** — `backend/internal/models/proxy_host.go:61-63`:
  ```go
  // Proxy Group assignment
  ProxyGroupID *uint       `json:"-" gorm:"index"`
  ProxyGroup   *ProxyGroup `json:"proxy_group,omitempty" gorm:"foreignKey:ProxyGroupID"`
  ```
  `backend/internal/models/proxy_group.go:11-19` defines `ProxyGroup{UUID,
  Name, Description, Color}`.
- **Handler contract already accepts `proxy_group_id`** —
  `backend/internal/api/handlers/proxy_host_handler.go`:
  - `Create` (lines 459-466) resolves `payload["proxy_group_id"]` via
    `resolveProxyGroupReference` before unmarshalling into the model.
  - `Update` (lines 683-690) resolves `payload["proxy_group_id"]` the same
    way and assigns `host.ProxyGroupID` directly (partial-update pattern
    matching every other nullable FK on this handler).
  - `resolveProxyGroupReference` (lines 256-283) is **stricter than the
    other FK resolvers**: it only accepts a UUID string (or `nil`/empty
    string to clear); unlike `resolveAccessListReference` /
    `resolveCertificateReference` / `resolveDNSProviderReference`, it does
    **not** fall back to accepting a legacy numeric ID. Rationale in the
    inline comment: `ProxyGroup.ID` is `json:"-"`, so the API never exposes
    the numeric PK to clients — the field is UUID-only by design.
  - Both routes already exist:
    `POST /proxy-hosts`, `PUT /proxy-hosts/:uuid` (`RegisterRoutes`, lines
    410-420).
  - **Conclusion: no backend change is needed.** The full round trip
    (accept UUID → resolve to internal ID → persist → return
    `proxy_group` object in the response) already works; it is simply
    never exercised because no frontend form sends the field except the
    bulk endpoint.
- **Frontend — already wired at the list-page level**:
  - `frontend/src/api/proxyGroups.ts` — `proxyGroupsApi.list/get/create/update/delete`
    against `/proxy-groups`.
  - `frontend/src/hooks/useProxyGroups.ts` — `useProxyGroups()` (React
    Query, key `['proxy-groups']`) plus create/update/delete mutations.
  - `frontend/src/api/proxyHosts.ts:22-76` — `ProxyHost` interface already
    declares `proxy_group_id?: number | string | null` and
    `proxy_group?: { uuid; name; color } | null` (lines 52-57).
  - `frontend/src/components/ProxyGroupBadge.tsx` — small read-only
    `{dot, name}` badge, used in the list's "Group" column
    (`ProxyHosts.tsx:598-603`) — reused as-is, no changes needed.
  - `frontend/src/pages/ProxyHosts.tsx` — per-group `DataTable` sections
    with `GroupDropZone` (drag targets) and `ProxyHostDragHandle`
    (draggable handle, dnd-kit) for cross-group reassignment; a bulk
    "Assign to group" modal keyed off `selectedHosts`.
  - **Gap confirmed**: `frontend/src/components/ProxyHostForm.tsx` (1622
    lines) has fields for `certificate_id`, `access_list_id`,
    `security_header_profile_id`, `dns_provider_id` — every other nullable
    FK — but no `proxy_group_id` field, no import of `useProxyGroups`, and
    `buildInitialFormData` (lines 105-131) never reads
    `host?.proxy_group_id` / `host?.proxy_group`.

### 2.2 Reusable selector pattern

`frontend/src/components/AccessListSelector.tsx` is the closest existing
pattern: a small selector component that (a) calls its own data hook
(`useAccessLists`), (b) renders a `Select`/`SelectTrigger`/`SelectContent`
from `./ui/Select`, (c) has a `none` sentinel option, and (d) calls a
`onChange(id)` prop. However, `AccessListSelector` also carries
~60 lines of numeric-ID/UUID "token" reconciliation
(`resolveAccessListToken`/`getOptionToken`) because access lists support
legacy numeric IDs. **Proxy groups do not** (see §2.1) — `ProxyGroup.ID` is
never serialized, so the new `ProxyGroupSelector` can be materially
simpler: the value is always `string | null` (a UUID or `null`), no token
scheme needed.

`ProxyHostForm.tsx` already has all the pieces this selector needs close
by: `resolveSelectToken`/`resolveTokenToFormValue`/`getEntityToken`
(lines 191-249) are used for `certificate_id` and
`security_header_profile_id`, which DO need the numeric/UUID token scheme.
`proxy_group_id` does not need this machinery — it is simpler to write a
small dedicated `ProxyGroupSelector` component (mirroring
`AccessListSelector`'s file-level shape, not its token complexity) than to
bolt another token variant onto the giant inline `Select` blocks already
in `ProxyHostForm.tsx`.

### 2.3 Grouped-view row layout (Problem 2) — root cause

Table rendering lives in `frontend/src/components/ui/DataTable.tsx`
(generic, reused across the app) and is driven by the `columns: Column<ProxyHost>[]`
array built in `frontend/src/pages/ProxyHosts.tsx:502-649`.

**Column widths today** (`ProxyHosts.tsx`, `width` prop per column):

| Column | width |
|---|---|
| name | 14% |
| domain | 18% |
| forward | 14% |
| ssl | 7% |
| features | 9% |
| **group** | **10%** |
| status | 7% |
| actions | 9% |
| **declared total** | **88%** |

Plus, from `DataTable.tsx`:
- checkbox column: always rendered when `selectable` (every call site
  passes `selectable`), fixed `w-12` (48px), **no `width` accounted for in
  the 88%**.
- drag-handle column: rendered only when `renderDragHandle` is passed —
  which happens **only** in the two per-group-section `DataTable` calls
  (`ProxyHosts.tsx:838` and `:870`, gated by `showDragHandles`/"Organize"
  toggle) — fixed `w-10` (40px), also unaccounted for in the 88%.
- `<table>` has no `min-width` and no `table-layout: fixed`
  (`DataTable.tsx:117`, just `className="w-full"`); the scroll container
  is `<div className="overflow-x-auto">` (`DataTable.tsx:116`).

**Root cause**: because the table has no `min-width` floor and uses the
browser's default `table-layout: auto`, the declared `width` percentages
are only *hints* the browser can override once it must fit `width: 100%`.
When a proxy group exists, the SAME `columns` array (including the
10%-wide **"Group" column**) is reused for the **per-group section
tables** (`ProxyHosts.tsx:831-847`, `:863-872`), where it is **entirely
redundant** — every row inside a named group's `DataTable` already carries
that exact group's color/name in the section header
(`ProxyHosts.tsx:796-807`), and inside the "Ungrouped" section it always
renders `—`. That 10% is dead weight in grouped view specifically (it is
*not* dead weight in the flat, single-table view used when
`groups.length === 0`, where "Group" is the only place a host's group
membership is visible). Toggling "Organize" (`showDragHandles`) adds a
further fixed 40px drag-handle column on top, with nothing given back.
With no `min-width` on the table to force real overflow, the browser
instead compresses the flexible columns — Actions (9%, holding two text
buttons) is the first to visibly suffer, ending up pinned to the row's
right edge with too little room, and because the table never actually
exceeds `100%` of its container, `overflow-x-auto` never triggers a
scrollbar. This matches the reported symptom exactly: "not all the info in
the row fits... no scroll bar... right at the edge of the delete column."

**Fix approach** (real layout fix first, scroll fallback as secondary net):

1. **Drop the redundant "Group" column when rendering inside a group
   context.** Build two column lists in `ProxyHosts.tsx`: the existing
   `columns` (used only for the flat `groups.length === 0` table, keeps
   "Group"), and a derived `groupedColumns` (same array filtered to
   exclude the `key === 'group'` entry) used by the two per-group-context
   `DataTable` calls (named-group sections and the "Ungrouped" section).
   This reclaims 10% of width in exactly the view where the column added
   no information, with zero loss of information (group membership is
   already shown by the enclosing section).
2. **Redistribute the reclaimed width**: bump `domain` from `18%` → `22%`
   (it holds the widest content — one or more clickable domain links) and
   `actions` from `9%` → `13%` (it holds two text buttons, "Edit"/"Delete",
   that must never wrap) in `groupedColumns` only; `flatColumns` (the
   `groups.length === 0` case) is unchanged. New grouped-view total: name
   14 + domain 22 + forward 14 + ssl 7 + features 9 + status 7 + actions 13
   = 86%, plus the fixed 48px checkbox column (and 40px drag-handle column
   only while "Organize" is active) — comfortably under 100% at common
   desktop widths (≥1024px) after accounting for those fixed columns.
3. **Give the Actions column a hard floor** so its two buttons can never
   be compressed below usable size regardless of viewport: add an optional
   `minWidth?: string` to the `Column<T>` interface in `DataTable.tsx`,
   applied via `style={{ width: col.width, minWidth: col.minWidth }}` on
   both the `<th>` (line 146) and propagate `whitespace-nowrap` on the
   `<td>` for that column so "Edit"/"Delete" never wrap onto two lines.
   Set `minWidth: '140px'` on the `actions` column definition (both
   `flatColumns` and `groupedColumns`).
4. **Scroll fallback (secondary safety net, not primary fix)**: give the
   `<table>` element in `DataTable.tsx` a `min-w-[760px]` class alongside
   its existing `w-full`. This does not change rendering at normal desktop
   widths (the table is already wider than 760px there), but at narrow
   viewports it stops the browser from continuing to compress columns
   past a usable minimum — instead the existing `overflow-x-auto` wrapper
   (already present, `DataTable.tsx:116`) starts actually scrolling, which
   today it structurally cannot do because nothing forces the table past
   `100%` width. This is the "no scrollbar" half of the bug fixed as a
   fallback, while (1)-(3) are the real fix for the common desktop case
   the issue describes.

No changes are needed to `GroupDropZone.tsx`, `ProxyHostDragHandle.tsx`, or
`useProxyGroupDnD.ts` — none of them affect column widths; they only
affect drag-and-drop behavior and the section wrapper around each
`DataTable`.

### 2.4 External dependencies

None new. Uses existing `@tanstack/react-query`, existing `./ui/Select`
primitives, existing Tailwind utility classes.

## 3. Technical Specifications

### 3.1 API contract (already available — no changes)

`POST /proxy-hosts` and `PUT /proxy-hosts/:uuid` already accept:

```jsonc
{
  // ... other fields ...
  "proxy_group_id": "5c1f7e3a-....-uuid"  // string UUID, or null/"" to clear
}
```

Response body (`ProxyHost` / `ProxyHostResponse`) already includes:

```jsonc
{
  "proxy_group": { "uuid": "...", "name": "Media", "color": "#6366f1" } // or null
}
```

Validation behavior (unchanged, already implemented in
`resolveProxyGroupReference`):
- `null` or `""` → clears the group (`ProxyGroupID = nil`).
- Non-empty string not matching an existing group's `uuid` → `400 {"error": "proxy group not found"}`.
- Any non-string, non-nil value (e.g. a number) → `400 {"error": "invalid proxy_group_id: must be a UUID string"}`.

### 3.2 Frontend component: `ProxyGroupSelector`

New file: `frontend/src/components/ProxyGroupSelector.tsx`

```ts
interface ProxyGroupSelectorProps {
  value: string | null | undefined
  onChange: (uuid: string | null) => void
}

export default function ProxyGroupSelector({ value, onChange }: ProxyGroupSelectorProps) {
  const { data: groups } = useProxyGroups()
  // Select / SelectTrigger / SelectContent from ./ui/Select, mirroring
  // AccessListSelector's structure but WITHOUT the numeric/UUID token
  // machinery (proxy_group_id is UUID-only, see §2.1/§2.3).
  // Sentinel "none" option -> onChange(null).
  // Each group option value === group.uuid; renders a colored dot (like
  // ProxyGroupBadge) + group.name in the SelectItem.
  // Optional: link to "Manage groups" mirroring AccessListSelector's
  // "Manage lists" link, pointing wherever ManageGroupsDialog is triggered
  // from (informational text only; opening the dialog from inside the
  // host form is out of scope — see Non-Goals).
}
```

Unit tests: `frontend/src/components/__tests__/ProxyGroupSelector.test.tsx`
- renders "No Group" (or equivalent sentinel) plus one `SelectItem` per
  group from `useProxyGroups()`.
- selecting a group calls `onChange(group.uuid)`.
- selecting the sentinel calls `onChange(null)`.
- pre-selects the option matching a passed-in `value`.
- renders correctly when `useProxyGroups()` returns an empty/loading list
  (no crash, sentinel-only dropdown).

### 3.3 `ProxyHostForm.tsx` changes

File: `frontend/src/components/ProxyHostForm.tsx`

1. Import `useProxyGroups` is not needed directly in the form — the new
   `ProxyGroupSelector` owns its own data fetching, matching how
   `AccessListSelector` is used at line 996-999. Add:
   ```ts
   import ProxyGroupSelector from './ProxyGroupSelector'
   ```
2. `ProxyHostFormState` type (line 252-259): add
   `proxy_group_id?: string | null` to the `Omit<...>` extension list (it's
   already `string | null` shaped on `ProxyHost`, no numeric variant, so no
   `Omit` needed on the base type — just add it to the intersected extra
   fields, matching how `access_list_id` etc. are typed).
3. `buildInitialFormData` (lines 105-131): add
   ```ts
   proxy_group_id: host?.proxy_group?.uuid ?? (typeof host?.proxy_group_id === 'string' ? host.proxy_group_id : null),
   ```
   (mirrors the existing `access_list_id`/`certificate_id` pattern of
   preferring the nested object's `uuid` over the raw ID field.)
4. Render the selector in the form body — placed directly after the
   **Access Control List** block (after line 999, before the **Security
   Headers Profile** block at line 1001), as both are "optional
   host-classification" selectors of similar weight:
   ```tsx
   {/* Proxy Group */}
   <div>
     <label className="block text-sm font-medium text-gray-300 mb-2">
       Proxy Group
       <span className="text-gray-500 font-normal ml-2">(Optional)</span>
     </label>
     <ProxyGroupSelector
       value={formData.proxy_group_id ?? null}
       onChange={(uuid) => setFormData(prev => ({ ...prev, proxy_group_id: uuid }))}
     />
     <p className="text-xs text-gray-500 mt-1">
       Organize this host under a group shown on the Proxy Hosts list.
     </p>
   </div>
   ```
5. `handleSubmit` (lines 538-598): `proxy_group_id` needs no special
   normalization function (unlike `access_list_id`/`certificate_id`, which
   go through `normalizeAccessListReference` because they support the
   legacy numeric-ID shape) — it is already a plain `string | null` in
   `formData`, so it passes through `...payloadWithoutUptime` unchanged
   into `submitPayload`. No new normalizer needed.

Unit tests: extend `frontend/src/components/__tests__/ProxyHostForm.test.tsx`
(and/or a new `ProxyHostForm-group.test.tsx` if the existing file is
already large/segmented, matching the repo's pattern of splitting
`ProxyHostForm-*.test.tsx` by concern — see existing
`ProxyHostForm-dns.test.tsx`, `ProxyHostForm-uptime.test.tsx`):
- new host: no group preselected by default (`null` / sentinel).
- editing a host with `proxy_group: {uuid, name, color}`: selector shows
  that group preselected.
- selecting a different group and submitting calls `onSubmit` with
  `proxy_group_id: '<selected-uuid>'`.
- clearing the group (sentinel) and submitting calls `onSubmit` with
  `proxy_group_id: null`.
- submitting a host with no group touched at all does not regress
  existing fields (snapshot-style assertion on the rest of the payload).

### 3.4 `ProxyHosts.tsx` + `DataTable.tsx` layout changes

File: `frontend/src/components/ui/DataTable.tsx`
- Add `minWidth?: string` to the `Column<T>` interface (next to `width`,
  line 12).
- Header `<th>` (line 146): `style={{ width: col.width, minWidth: col.minWidth }}`.
- Body `<td>` (line 251-256): apply `col.minWidth ? 'whitespace-nowrap' : undefined`
  to the `className` via `cn(...)`, and the same inline `minWidth` style,
  so a column that declares a floor never wraps its content.
- `<table className="w-full">` (line 117) → `<table className="w-full min-w-[760px]">`.

Unit tests: extend `frontend/src/components/ui/__tests__/DataTable.test.tsx`
- a column with `minWidth` renders with the expected inline style and
  `whitespace-nowrap` class on its cells.
- table root retains `min-w-[760px]` alongside `w-full` (class-list
  assertion) — regression guard against the fallback being dropped later.

File: `frontend/src/pages/ProxyHosts.tsx`
- Rename the existing `columns` to `flatColumns` at the definition site
  (line 502) — used only where `groups.length === 0` (line 748).
- Add `groupedColumns = flatColumns.filter(c => c.key !== 'group').map(c => ...)`
  overriding `domain`'s `width` to `22%`, `actions`'s `width` to `13%` and
  `minWidth` to `140px`, and setting `minWidth: '140px'` on `flatColumns`'s
  `actions` entry too (both views get the floor; only grouped view gets
  the reclaimed-width redistribution). Implementation detail left to
  `frontend-dev`: either build two independent literal arrays (simplest,
  most explicit, avoids a generic "column patcher" abstraction for two
  fields) or derive `groupedColumns` from `flatColumns` with a small
  `remap` helper — either is acceptable as long as both column sets stay
  in sync via a single shared `cell` renderer per column key (no
  duplicated JSX between the two).
- Use `groupedColumns` at the two per-group-context `DataTable` call sites
  (currently lines 831-847 "named group" and 863-872 "Ungrouped section"),
  replacing `columns`.
- Flat-view `DataTable` (line 748) keeps `flatColumns`.

Unit tests: extend `frontend/src/pages/__tests__/ProxyHosts-groups.test.tsx`
- when `groups` is non-empty, the "Group" column header is **absent**
  from each rendered per-group `DataTable` section (query by column
  header text within `within(section)`).
- when `groups` is empty (flat view), the "Group" column header **is**
  present.
- Actions column ("Edit"/"Delete" buttons) is present and not wrapped
  (smoke-level: buttons render with accessible names) in both grouped and
  flat views, at a representative width (jsdom doesn't lay out CSS, so
  this test asserts markup/class presence — e.g. the `minWidth`-driven
  `whitespace-nowrap` class — rather than pixel measurements; the
  Playwright E2E spec is the real layout check, see §4.1).

## 4. Implementation Plan

### Phase 1: Playwright E2E specs (written first, `test.fixme` until implemented)

New/extended spec: `tests/proxy-groups.spec.ts` (extend existing describe
blocks) — add:

```ts
test.describe('Proxy Host Form — Group Selector', () => {
  test.fixme('shows a group selector when creating a new host', async ({ page }) => { /* ... */ });
  test.fixme('preselects the host\'s current group when editing', async ({ page }) => { /* ... */ });
  test.fixme('assigns a group to a host via the create/edit form', async ({ page }) => { /* ... */ });
  test.fixme('clears a host\'s group via the create/edit form', async ({ page }) => { /* ... */ });
});

test.describe('Proxy Hosts — Grouped Row Layout', () => {
  test.fixme('keeps the Actions column fully visible at 1280px width when grouped', async ({ page }) => { /* ... */ });
  test.fixme('does not render a redundant Group column inside a named group section', async ({ page }) => { /* ... */ });
});
```

Delegate authoring of the real Playwright steps (locators, fixtures,
assertions) to `playwright-dev` once the plan is approved — per team
convention (`playwright-dev` writes tests only, does not implement
product code). Use `tests/fixtures/proxy-hosts.ts` and existing
`proxy-groups.spec.ts` helpers (`waitForAPIHealth`, `waitForDialog`,
`waitForAPIResponse`) as the base, consistent with the existing file's
style (see §2, research read of that file's top).

Layout assertions should use `page.setViewportSize({width: 1280, height: 800})`
(a realistic laptop width) and assert the Edit/Delete buttons'
bounding boxes are fully inside the table's bounding box (no clipping),
rather than asserting exact pixel widths (brittle).

### Phase 2: Foundation

- `DataTable.tsx`: add `minWidth` to `Column<T>`, wire it into `<th>`/`<td>`,
  add `min-w-[760px]` to `<table>`. No behavior change for existing callers
  that don't pass `minWidth` (backward compatible — optional field).
- Add `frontend/src/components/ProxyGroupSelector.tsx` (net-new component,
  no wiring into `ProxyHostForm` yet in this commit) with its unit tests.

Validation gate: `cd frontend && npx vitest run src/components/ui/__tests__/DataTable.test.tsx src/components/__tests__/ProxyGroupSelector.test.tsx` passes; `npm run type-check` clean.

### Phase 3: Backend

**Gap found during Commit 6 E2E enablement (contingency triggered).**
`Create` (`backend/internal/api/handlers/proxy_host_handler.go`, ~lines
434-524) resolves `proxy_group_id` into a local variable, but then builds
the model via `json.Marshal(payload)` → `json.Unmarshal(payloadBytes, &host)`.
Because `ProxyHost.ProxyGroupID` is `json:"-"` (by design — the numeric PK
is never exposed to clients), that round-trip silently discards the
resolved value before `h.service.Create(&host)` is called, so a host
created with a `proxy_group_id` is persisted with no group at all. `Update`
does not have this bug — it assigns `host.ProxyGroupID = resolvedGroupID`
directly on the already-loaded struct, bypassing JSON entirely. This
contradicts this spec's original §2.1 conclusion ("the full round trip
already works... simply never exercised") — it only works for Update.

**Fix (Commit 4.5, inserted between Commit 4 and Commit 6)**: in `Create`,
after `json.Unmarshal(payloadBytes, &host)`, add
`host.ProxyGroupID = resolvedGroupID` (mirroring `Update`), keeping the
resolved `*uint` in scope from the earlier resolution block. Add a Go
regression test asserting a `POST /proxy-hosts` with `proxy_group_id` set
actually persists the group (assert via a follow-up `GET`, not just the
create response body, since the response's own `proxy_group` serialization
is a separate, non-blocking minor gap noted below). Scope: this is a
one-line-class fix isolated to the `Create` handler; no model or migration
change.

### Phase 4: Frontend integration

- Wire `ProxyGroupSelector` into `ProxyHostForm.tsx` per §3.3.
- Split `columns` into `flatColumns`/`groupedColumns` in `ProxyHosts.tsx`
  per §3.4, update the three `DataTable` call sites.
- Extend `ProxyHostForm.test.tsx` (or new `ProxyHostForm-group.test.tsx`)
  and `ProxyHosts-groups.test.tsx` per §3.3/§3.4.

Validation gate: `cd frontend && npx vitest run src/components/__tests__/ProxyHostForm.test.tsx src/components/__tests__/ProxyHostForm-group.test.tsx src/pages/__tests__/ProxyHosts-groups.test.tsx` passes; `npm run type-check` clean; `npm run build` clean.

### Phase 5: Hardening, enable E2E, docs

- Flip the `test.fixme` specs from Phase 1 to real assertions (or confirm
  `playwright-dev`'s authored versions pass) and run:
  `npx playwright test tests/proxy-groups.spec.ts --project=firefox`
  from repo root (targeted, single browser, per Definition of Done — never
  the full suite or multiple `--project` flags locally).
- Update `docs/features.md` if it documents the proxy-host form fields
  (brief one-line addition noting group assignment is available in the
  form, not just drag-and-drop/bulk-assign) — delegate to `docs-writer`.
- Run full Definition of Done: `scripts/local-patch-report.sh`,
  `scripts/frontend-test-coverage.sh` (≥85%), `lefthook run pre-commit`,
  `npm run type-check`, `npm run build`. No `backend/internal/models/**`
  or GORM changes are expected in this feature, so the GORM Security Scan
  gate (§1.5 of the Definition of Done) does not apply unless Phase 3
  ends up non-empty.

## 5. Commit Slicing Strategy

**Decision**: Single PR, one feature (proxy-host group UI gap +
grouped-view row layout), ordered commits within that PR. Both problems
are fixed together because they are the same UI surface (the
proxy-host/group presentation) and reviewing them separately would leave
the PR in a half-fixed state relative to issue #1367, which reports both
as one user-facing complaint.

| # | Commit | Scope / Files | Depends on | Validation gate |
|---|--------|---------------|------------|------------------|
| 1 | `test: add E2E specs for proxy host group selector and grouped-view layout (fixme)` | `tests/proxy-groups.spec.ts` (new `test.fixme` blocks only) | — | Spec file parses: `npx playwright test tests/proxy-groups.spec.ts --project=firefox --list` |
| 2 | `refactor: add column minWidth support to DataTable` | `frontend/src/components/ui/DataTable.tsx`, `frontend/src/components/ui/__tests__/DataTable.test.tsx` | — | `npx vitest run src/components/ui/__tests__/DataTable.test.tsx`; `npm run type-check` |
| 3 | `feat: add ProxyGroupSelector component` | `frontend/src/components/ProxyGroupSelector.tsx`, `frontend/src/components/__tests__/ProxyGroupSelector.test.tsx` | — | `npx vitest run src/components/__tests__/ProxyGroupSelector.test.tsx`; `npm run type-check` |
| 4 | `feat: add group selector to proxy host create/edit form` | `frontend/src/components/ProxyHostForm.tsx`, `frontend/src/components/__tests__/ProxyHostForm.test.tsx` (or new `ProxyHostForm-group.test.tsx`) | 3 | `npx vitest run src/components/__tests__/ProxyHostForm*.test.tsx`; `npm run type-check`; `npm run build` |
| 5 | `fix: correct row layout in grouped Proxy Hosts view` | `frontend/src/pages/ProxyHosts.tsx`, `frontend/src/pages/__tests__/ProxyHosts-groups.test.tsx` | 2 | `npx vitest run src/pages/__tests__/ProxyHosts-groups.test.tsx`; `npm run type-check`; `npm run build` |
| 6 | `test: enable E2E specs for group selector and grouped-view layout` | flip `test.fixme` → real assertions in `tests/proxy-groups.spec.ts` | 1, 4, 5 | `npx playwright test tests/proxy-groups.spec.ts --project=firefox` |
| 7 | `docs: note group assignment in proxy host form` | `docs/features.md` (if applicable) | 4 | doc review only; no test gate |

Commits 2 and 3 are independent of each other (no shared files) and can be
authored/reviewed in either order, but both must land before commit 4
(which depends on 3) and commit 5 (which depends on 2). Commit 1 can be
authored any time before commit 6 but is listed first per the team's
suggested sequence (E2E specs as `test.fixme` before implementation).
Before merge, the full Definition of Done (coverage, lint, build,
type-check) must pass regardless of per-commit gates above.

### Rollback / contingency

- The whole PR is one deployable unit; if QA or the user rejects the
  layout fix (§3.4) post-merge, commits 5 and 6 must be reverted together
  (commit 6's enabled E2E assertions depend on commit 5's layout fix, so
  reverting 5 alone would leave those assertions failing). Commits 2-4 —
  the form selector — are functionally and file-wise independent of 5/6,
  so a targeted `git revert` of 5+6 does not touch the form feature.
- If Phase 3 (backend) turns out non-empty after all (see §4, Phase 3
  contingency), insert it as its own commit between commits 3 and 4, with
  its own Go tests and, per the Definition of Done, a GORM security scan
  run (`./scripts/scan-gorm-security.sh --check`) before commit 4 proceeds.
- No feature flag is introduced — both changes are additive UI (a new
  optional field, a layout correction) with no migration or data
  implications, so a flag would add complexity without a corresponding
  safety benefit. If the layout fix needs to be de-risked further at
  review time, `groupedColumns`/`flatColumns` are separate arrays by
  design (§3.4), making a partial revert (keep the form selector, drop
  the layout change) mechanically trivial even without a flag.

## 6. Acceptance Criteria

- [ ] Creating a new proxy host allows selecting a proxy group (or none);
      the created host's group matches what was selected (verified via
      the list page's "Group" column badge or grouped section placement).
- [ ] Editing an existing host with a group pre-populates that group in
      the form; changing it and saving updates the host's group; clearing
      it (selecting "No Group") removes the association.
- [ ] No backend changes were required, OR any backend change made is
      documented with its own commit, tests, and rationale in this spec's
      §4 Phase 3 section before merge.
- [ ] At a 1280px (and wider) viewport, the grouped Proxy Hosts view shows
      the Edit and Delete buttons fully visible and unclipped in every
      row, in every group section, with no text overlapping the row edge.
- [ ] The "Group" column no longer renders inside per-group (or
      "Ungrouped") sections; it still renders in the flat/ungrouped-mode
      table when no groups exist.
- [ ] At narrow viewports where columns genuinely cannot all fit, the
      table area shows a horizontal scrollbar instead of visually
      clipping/overlapping content.
- [ ] All new/changed code has Vitest unit test coverage; overall frontend
      coverage remains ≥85% (`scripts/frontend-test-coverage.sh`).
- [ ] Playwright specs in `tests/proxy-groups.spec.ts` covering both
      problems pass under `--project=firefox` (targeted run, per
      Definition of Done).
- [ ] `npm run type-check` and `npm run build` (frontend) pass with zero
      errors.
- [ ] `lefthook run pre-commit` passes with zero errors; `--no-verify` was
      not used.
- [ ] Full Definition of Done from `CLAUDE.md` is satisfied before the PR
      is marked ready for merge.
