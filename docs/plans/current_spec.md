# Spec: Drag-and-Drop Proxy Host Group Assignment

**Status**: Draft
**Author**: Principal Architect
**Target**: Single PR — ordered logical commits

---

## 1. Introduction

### Overview

This feature adds drag-and-drop (DnD) group assignment for proxy host cards in `ProxyHosts.tsx`. Users can drag one or more host rows from any group section (including Ungrouped) and drop them onto a different group's header/container. Alphabetical ordering within each group is preserved — DnD only changes group membership.

### Objectives

1. Allow individual host rows to be dragged to a different group without opening a modal.
2. When the dragged host is part of a multi-selection, moving it moves **all** selected hosts.
3. When the dragged host is **not** selected, only that one host moves (selection is unaffected).
4. The "Ungrouped" section is always a valid drop target, setting `proxy_group_id = null`.
5. Visual feedback (highlight ring) on the target group during drag.
6. Optimistic UI: cache updated immediately, rolled back on API error.
7. Full keyboard accessibility via dnd-kit's built-in `KeyboardSensor`.
8. Feature coexists with the existing "Assign to Group" bulk-action button — they are not mutually exclusive.

---

## 2. Research Findings

### 2.1 No Existing DnD Library

`frontend/package.json` has no `@dnd-kit`, `react-dnd`, `react-beautiful-dnd`, or `@hello-pangea/dnd`. The library must be installed.

**Chosen library**: `@dnd-kit/core` + `@dnd-kit/utilities`
**Rationale**: Actively maintained, touch/pointer/keyboard sensors out-of-the-box, composable (no opinionated sortable wrapper needed — cross-group movement only, no intra-group reordering).

### 2.2 Backend: PUT /proxy-hosts/:uuid Already Supports proxy_group_id

The `Update` handler in `proxy_host_handler.go` handles `proxy_group_id` via `resolveProxyGroupReference`:

- `nil` → sets `ProxyGroupID = nil` (ungrouped)
- UUID string → looked up in `proxy_groups` table by `WHERE uuid = ?`
- Empty string → treated as null

**Conclusion**: Sending `PUT /proxy-hosts/:uuid` with `{ "proxy_group_id": "group-uuid" }` (or `null`) is already supported with no backend changes for single-host updates.

### 2.3 New Bulk Endpoint Needed

The existing `BulkUpdateACL` handler pattern: loops over `host_uuids`, updates each, then calls `caddyManager.ApplyConfig` **once**. If we fire N individual PUTs for a multi-select drag, Caddy rebuilds its config N times. A bulk endpoint applies the config once.

**Decision**: Add `PUT /proxy-hosts/bulk-update-group` following the exact `BulkUpdateACL` pattern.

### 2.4 DataTable Has No DnD Hooks

`frontend/src/components/ui/DataTable.tsx` renders standard `<table>/<tr>/<td>`. The cleanest extension is adding an optional `renderDragHandle?: (row: T) => React.ReactNode` prop that, when provided, adds a narrow leading column per row.

### 2.5 ProxyHosts.tsx Grouped Layout

When `groups.length > 0`, the grouped view renders:

```
<div className="space-y-6">
  {groups.map(group => (
    <section key={group.uuid} aria-label={group.name}>
      <div>{/* header: color dot, name, count, edit/delete buttons */}</div>
      <DataTable data={groupHosts} ... />
    </section>
  ))}
  {groupedHosts.ungrouped.length > 0 && (
    <section aria-label={t('proxyGroups.ungrouped')}>
      <DataTable data={groupedHosts.ungrouped} ... />
    </section>
  )}
</div>
```

Each `<section>` must become a droppable zone. Each DataTable row must have a drag handle.

### 2.6 Selection State

`selectedHosts: Set<string>` (UUID strings) is managed in `ProxyHosts.tsx`. The drag hook reads this to decide single vs. multi-drag.

### 2.7 Frontend API Types

`ProxyHost.proxy_group_id?: number | string | null` — accepted by the PUT endpoint.
`ProxyHost.proxy_group?: { uuid, name, color } | null` — what the frontend actually reads.

For optimistic updates, both fields must be updated in the cache snapshot.

---

## 3. Technical Specifications

### 3.1 API Design

#### 3.1.1 Existing Endpoint (no changes needed)

```
PUT /api/proxy-hosts/:uuid
Body: { "proxy_group_id": "<group-uuid>" | null }
Response: ProxyHost (200 OK)
```

#### 3.1.2 New Bulk Endpoint

```
PUT /api/proxy-hosts/bulk-update-group
Body:
  {
    "host_uuids": ["<uuid1>", "<uuid2>"],
    "proxy_group_id": "<group-uuid>" | null    // null = ungrouped
  }
Response 200:
  {
    "updated": 2,
    "errors": []
  }
Response 400: { "error": "host_uuids cannot be empty" }
Response 500: { "error": "...", "updated": N, "errors": [...] }
```

**Route registration** (in `RegisterRoutes`, alongside other bulk routes):

```go
router.PUT("/proxy-hosts/bulk-update-group", h.BulkUpdateGroup)
```

#### 3.1.3 Backend Handler Signature

```go
// BulkUpdateGroup applies a proxy group assignment to multiple proxy hosts.
// PUT /proxy-hosts/bulk-update-group
func (h *ProxyHostHandler) BulkUpdateGroup(c *gin.Context) {
    var req struct {
        HostUUIDs    []string `json:"host_uuids"    binding:"required"`
        ProxyGroupID *string  `json:"proxy_group_id"` // nil = ungrouped
    }
    // 1. Bind JSON → 400 on error
    // 2. Guard: len(req.HostUUIDs) == 0 → 400
    // 3. If req.ProxyGroupID != nil: resolveProxyGroupReference(*req.ProxyGroupID) → *uint
    // 4. Loop req.HostUUIDs:
    //      host, err := h.service.GetByUUID(hostUUID) → skip on err, append to errors
    //      host.ProxyGroupID = resolvedGroupID (or nil)
    //      h.service.Update(host) → append to errors on failure
    //      updated++
    // 5. If updated > 0: h.caddyManager.ApplyConfig(ctx) once
    // 6. Respond { "updated": N, "errors": [...] }
}
```

### 3.2 Frontend API Layer

**File**: `frontend/src/api/proxyHosts.ts` — append after `bulkUpdateSecurityHeaders`:

```typescript
export interface BulkUpdateGroupRequest {
  host_uuids: string[];
  proxy_group_id: string | null; // group UUID or null for ungrouped
}

export interface BulkUpdateGroupResponse {
  updated: number;
  errors: { uuid: string; error: string }[];
}

export const bulkUpdateGroup = async (
  hostUUIDs: string[],
  proxyGroupId: string | null
): Promise<BulkUpdateGroupResponse> => {
  const { data } = await client.put<BulkUpdateGroupResponse>(
    '/proxy-hosts/bulk-update-group',
    { host_uuids: hostUUIDs, proxy_group_id: proxyGroupId }
  );
  return data;
};
```

### 3.3 Frontend Hook Layer

**File**: `frontend/src/hooks/useProxyHosts.ts` — add inside `useProxyHosts()` alongside existing mutations:

```typescript
const bulkGroupMutation = useMutation({
  mutationFn: ({ hostUUIDs, proxyGroupId }: {
    hostUUIDs: string[];
    proxyGroupId: string | null;
  }) => bulkUpdateGroup(hostUUIDs, proxyGroupId),
});

// Expose:
bulkUpdateGroup: (hostUUIDs: string[], proxyGroupId: string | null) =>
  bulkGroupMutation.mutateAsync({ hostUUIDs, proxyGroupId }),
isBulkUpdatingGroup: bulkGroupMutation.isPending,
```

### 3.4 DnD State Hook

**New file**: `frontend/src/hooks/useProxyGroupDnD.ts`

This hook encapsulates all drag logic and is the only file that imports from `@dnd-kit/core`.

#### Interface

```typescript
import { useQueryClient } from '@tanstack/react-query';
import { QUERY_KEY } from './useProxyHosts';
import type { DragStartEvent, DragEndEvent, DragOverEvent } from '@dnd-kit/core';
import type { ProxyHost } from '../api/proxyHosts';
import type { ProxyGroup } from '../api/proxyGroups';

interface UseProxyGroupDnDOptions {
  hosts: ProxyHost[];
  groups: ProxyGroup[];
  selectedHosts: Set<string>;
  setSelectedHosts: (hosts: Set<string>) => void;
  bulkUpdateGroup: (uuids: string[], groupId: string | null) => Promise<BulkUpdateGroupResponse>;
}

interface UseProxyGroupDnDReturn {
  activeDragId:     string | null;   // UUID of host being dragged
  overGroupId:      string | null;   // UUID of hover target (or 'ungrouped')
  hostsBeingDragged: string[];       // UUIDs (1 or N depending on selection)
  handleDragStart:  (event: DragStartEvent) => void;
  handleDragOver:   (event: DragOverEvent)  => void;
  handleDragEnd:    (event: DragEndEvent)   => void;
  handleDragCancel: () => void;
}
```

#### Logic

**Hook initialization (inside `useProxyGroupDnD` body)**
```
const queryClient = useQueryClient()
```

**`handleDragStart(event)`**
```
activeDragId = event.active.id as string
hostsBeingDragged = selectedHosts.has(activeDragId)
  ? Array.from(selectedHosts)
  : [activeDragId]
```

**`handleDragOver(event)`**
```
overGroupId = event.over?.id as string ?? null
```

**`handleDragEnd(event)`**
```
if (!event.over) → handleDragCancel(); return

targetGroupId = event.over.id as string    // group UUID | 'ungrouped'
targetGroup   = groups.find(g => g.uuid === targetGroupId) ?? null

// Skip no-op: every dragged host is already in the target
alreadyInTarget = hostsBeingDragged.every(uuid => {
  const host = hosts.find(h => h.uuid === uuid)
  return (host?.proxy_group?.uuid ?? null) === (targetGroup?.uuid ?? null)
})
if (alreadyInTarget) → handleDragCancel(); return

// 1. Snapshot current cache
snapshot = queryClient.getQueryData<ProxyHost[]>(QUERY_KEY)

// 2. Optimistic update
queryClient.setQueryData<ProxyHost[]>(QUERY_KEY, (old = []) =>
  old.map(h => {
    if (!hostsBeingDragged.includes(h.uuid)) return h
    return {
      ...h,
      proxy_group_id: targetGroup?.uuid ?? null,
      proxy_group: targetGroup
        ? { uuid: targetGroup.uuid, name: targetGroup.name, color: targetGroup.color }
        : null,
    }
  })
)

const hostsBeingDragged_snapshot = [...hostsBeingDragged]

// 3. Reset drag state (DragOverlay disappears before API returns)
activeDragId = null; overGroupId = null; hostsBeingDragged = []

// 4. API call
try {
  result = await bulkUpdateGroup(hostsBeingDragged_snapshot, targetGroup?.uuid ?? null)
  if (result.errors.length > 0)
    toast.error(t('proxyGroups.dnd.partialError', { count: result.errors.length }))
  else
    toast.success(t('proxyGroups.dnd.moveSuccess', { count: hostsBeingDragged_snapshot.length }))
  queryClient.invalidateQueries({ queryKey: QUERY_KEY })
  setSelectedHosts(new Set())   // clear selection after successful move
} catch {
  // 5. Rollback
  queryClient.setQueryData(QUERY_KEY, snapshot)
  toast.error(t('proxyGroups.dnd.moveFailed'))
}
```

**`handleDragCancel()`**
```
activeDragId = null; overGroupId = null; hostsBeingDragged = []
```

### 3.5 Component: ProxyHostDragHandle

**New file**: `frontend/src/components/ProxyHostDragHandle.tsx`

```typescript
import { useDraggable } from '@dnd-kit/core';
import { GripVertical } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface ProxyHostDragHandleProps {
  hostUuid: string;
  /** Number of hosts that will move (≥1 when host is part of selection) */
  dragCount: number;
}

export function ProxyHostDragHandle({ hostUuid, dragCount }: ProxyHostDragHandleProps) {
  const { t } = useTranslation();
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: hostUuid,
    data: { type: 'proxy-host', hostUuid },
  });

  return (
    <span
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      className={[
        'inline-flex items-center justify-center w-6 h-6 rounded',
        'cursor-grab active:cursor-grabbing',
        'text-content-muted hover:text-content-secondary',
        'focus-visible:outline-none focus-visible:ring-2',
        'focus-visible:ring-brand-500 focus-visible:ring-offset-1',
        isDragging ? 'opacity-30' : '',
      ].join(' ')}
      aria-label={
        dragCount > 1
          ? t('proxyGroups.dnd.dragHandleMultiple', { count: dragCount })
          : t('proxyGroups.dnd.dragHandleSingle')
      }
      aria-roledescription={t('proxyGroups.dnd.roleDescription')}
    >
      <GripVertical size={16} aria-hidden="true" />
    </span>
  );
}
```

### 3.6 Component: GroupDropZone

**New file**: `frontend/src/components/GroupDropZone.tsx`

```typescript
import { useDroppable } from '@dnd-kit/core';

interface GroupDropZoneProps {
  /** Group UUID or the literal string 'ungrouped' */
  groupId: string;
  /** Whether any drag is currently active (for ungrouped empty-state visibility) */
  isDragActive: boolean;
  children: React.ReactNode;
}

export function GroupDropZone({ groupId, isDragActive, children }: GroupDropZoneProps) {
  const { setNodeRef, isOver } = useDroppable({ id: groupId });

  return (
    <div
      ref={setNodeRef}
      data-drop-zone={groupId}
      className={[
        'rounded-xl transition-all duration-150',
        isOver
          ? 'ring-2 ring-brand-400 ring-offset-2 ring-offset-surface-base bg-brand-500/5'
          : '',
      ].join(' ')}
      {/* aria-dropeffect is deprecated in ARIA 1.1 but retained for backward compatibility with older assistive technologies */}
      aria-dropeffect={isDragActive ? 'move' : undefined}
    >
      {children}
    </div>
  );
}
```

### 3.7 DataTable Extension

**Modified file**: `frontend/src/components/ui/DataTable.tsx`

Add one optional prop to `DataTableProps<T>`:

```typescript
/** When provided, renders a leading drag-handle column (before checkbox). */
renderDragHandle?: (row: T) => React.ReactNode;
```

**Changes inside DataTable**:

1. In `<thead>`, when `renderDragHandle` is set, add before the checkbox `<th>`:
```tsx
<th className="w-10 px-2 py-3" aria-hidden="true" />
```

2. In each `<tbody> <tr>`, when `renderDragHandle` is set, add before checkbox `<td>` and column `<td>` cells:
```tsx
<td
  className="w-10 px-2 py-4"
  onClick={(e) => e.stopPropagation()}
  onKeyDown={(e) => e.stopPropagation()}
>
  {renderDragHandle(row)}
</td>
```

3. Update the empty-state `colSpan` to include the drag handle column:
```tsx
colSpan={
  columns.length
  + (selectable ? 1 : 0)
  + (renderDragHandle ? 1 : 0)
}
```

No other changes to DataTable. The `onClick`/`onKeyDown` stopPropagation prevents the drag handle click from toggling row selection.

### 3.8 DragOverlay

Inline inside `ProxyHosts.tsx` — no separate component file needed:

```tsx
<DragOverlay dropAnimation={{ duration: 150, easing: 'ease' }}>
  {activeDragId && (
    <div className="rounded-lg bg-surface-elevated border border-brand-400 shadow-xl px-4 py-2 text-sm font-medium text-content-primary cursor-grabbing">
      {hostsBeingDragged.length > 1
        ? t('proxyGroups.dnd.movingMultiple', { count: hostsBeingDragged.length })
        : (() => {
            const h = hosts.find(x => x.uuid === activeDragId);
            return h?.name || h?.domain_names || t('proxyGroups.dnd.movingOne');
          })()
      }
    </div>
  )}
</DragOverlay>
```

### 3.9 ProxyHosts.tsx Integration

**Modified file**: `frontend/src/pages/ProxyHosts.tsx`

#### New Imports

```typescript
import {
  DndContext, DragOverlay,
  PointerSensor, KeyboardSensor,
  useSensor, useSensors,
  pointerWithin,
} from '@dnd-kit/core';
import { GroupDropZone }       from '../components/GroupDropZone';
import { ProxyHostDragHandle } from '../components/ProxyHostDragHandle';
import { useProxyGroupDnD }    from '../hooks/useProxyGroupDnD';
```

#### New Hook Usage

```typescript
const {
  activeDragId,
  overGroupId,
  hostsBeingDragged,
  handleDragStart,
  handleDragOver,
  handleDragEnd,
  handleDragCancel,
} = useProxyGroupDnD({
  hosts,
  groups,
  selectedHosts,
  setSelectedHosts,
  bulkUpdateGroup,   // from useProxyHosts()
});

const sensors = useSensors(
  useSensor(PointerSensor, {
    activationConstraint: { distance: 8 }, // prevents accidental drags on click
  }),
  useSensor(KeyboardSensor)
);
```

#### Drag Handle Column (grouped view only)

```typescript
const dragHandleColumn = useCallback(
  (host: ProxyHost) => (
    <ProxyHostDragHandle
      hostUuid={host.uuid}
      dragCount={selectedHosts.has(host.uuid) ? selectedHosts.size : 1}
    />
  ),
  [selectedHosts]
);
```

#### Helper Utilities (add to component body)

```typescript
const getHostName = useCallback((uuid: string) => {
  const h = hosts.find(x => x.uuid === uuid);
  return h?.name || h?.domain_names || uuid;
}, [hosts]);

const getGroupName = useCallback((id: string) => {
  if (id === 'ungrouped') return t('proxyGroups.ungrouped');
  return groups.find(g => g.uuid === id)?.name ?? id;
}, [groups, t]);
```

#### Grouped Render Replacement

Replace the current `<div className="space-y-6">` grouped block with:

```tsx
<DndContext
  sensors={sensors}
  // Use pointerWithin (not closestCenter): group sections are tall droppable regions;
  // closestCenter resolves to the geometrically nearest center which may be the
  // wrong group for pointers visually inside a large container.
  collisionDetection={pointerWithin}
  onDragStart={handleDragStart}
  onDragOver={handleDragOver}
  onDragEnd={handleDragEnd}
  onDragCancel={handleDragCancel}
  accessibility={{
    announcements: {
      onDragStart: ({ active }) =>
        t('proxyGroups.dnd.announcePickUp', { name: getHostName(active.id as string) }),
      onDragOver: ({ over }) =>
        over ? t('proxyGroups.dnd.announceOver', { group: getGroupName(over.id as string) }) : '',
      onDragEnd: ({ over }) =>
        over
          ? t('proxyGroups.dnd.announceDrop', { group: getGroupName(over.id as string) })
          : t('proxyGroups.dnd.announceCancel'),
      onDragCancel: () => t('proxyGroups.dnd.announceCancel'),
    },
  }}
>
  <div className="space-y-6">
    {groups.map((group) => {
      const groupHosts = groupedHosts.byGroup[group.uuid] ?? [];
      return (
        <GroupDropZone key={group.uuid} groupId={group.uuid} isDragActive={!!activeDragId}>
          <section aria-label={group.name}>
            {/* existing group header div — no changes */}
            <DataTable
              data={groupHosts}
              columns={columns}
              rowKey={(row) => row.uuid}
              selectable
              selectedKeys={selectedHosts}
              onSelectionChange={setSelectedHosts}
              renderDragHandle={dragHandleColumn}
            />
          </section>
        </GroupDropZone>
      );
    })}

    {/* Always render ungrouped zone while dragging, even if empty */}
    {(groupedHosts.ungrouped.length > 0 || !!activeDragId) && (
      <GroupDropZone groupId="ungrouped" isDragActive={!!activeDragId}>
        <section aria-label={t('proxyGroups.ungrouped')}>
          <div className="flex items-center gap-2 mb-2">
            <h2 className="text-sm font-semibold text-content-muted">
              {t('proxyGroups.ungrouped')}
            </h2>
            <span className="text-xs text-content-muted">
              {t('proxyGroups.hostCount', { count: groupedHosts.ungrouped.length })}
            </span>
          </div>
          <DataTable
            data={groupedHosts.ungrouped}
            columns={columns}
            rowKey={(row) => row.uuid}
            selectable
            selectedKeys={selectedHosts}
            onSelectionChange={setSelectedHosts}
            renderDragHandle={dragHandleColumn}
            emptyState={null}
          />
        </section>
      </GroupDropZone>
    )}
  </div>

  <DragOverlay dropAnimation={{ duration: 150, easing: 'ease' }}>
    {/* see §3.8 */}
  </DragOverlay>
</DndContext>
```

> **Note**: `DndContext` wraps ONLY the grouped `<div className="space-y-6">` block, not the flat `DataTable` rendered when `groups.length === 0`. `renderDragHandle` should NOT be passed to the flat view.

### 3.10 i18n Translation Keys

**File**: `frontend/src/locales/en/translation.json` — add inside `"proxyGroups"`:

```json
"dnd": {
  "dragHandleSingle":   "Drag to move to another group",
  "dragHandleMultiple": "Drag to move {{count}} selected hosts",
  "roleDescription":    "Draggable proxy host",
  "movingOne":          "Moving host",
  "movingMultiple":     "Moving {{count}} hosts",
  "moveSuccess_one":    "Moved 1 host",
  "moveSuccess_other":  "Moved {{count}} hosts",
  "moveFailed":         "Failed to move host(s)",
  "partialError":       "{{count}} host(s) failed to move",
  "announcePickUp":     "Picked up {{name}}. Use arrow keys to move between groups.",
  "announceOver":       "Moving over {{group}}",
  "announceDrop":       "Dropped into {{group}}",
  "announceCancel":     "Move cancelled"
}
```

All other locale files must receive the same keys. Use English strings as placeholders where translations are unavailable.

---

## 4. Data Flow

```
User grabs drag handle
  PointerSensor (distance ≥ 8px) OR KeyboardSensor (Space)
        │
        ▼
handleDragStart()
  activeDragId    = host.uuid
  hostsBeingDragged = selectedHosts.has(uuid) → [...selectedHosts] | [uuid]
        │
   drag in progress
        ▼
handleDragOver()
  overGroupId = event.over?.id   (group.uuid | 'ungrouped')
  GroupDropZone: isOver=true → ring-2 ring-brand-400 highlight
        │
   user releases (pointer) or presses Space/Enter (keyboard)
        ▼
handleDragEnd()
  ├── no drop target → handleDragCancel()
  ├── no-op (same group) → handleDragCancel()
  └── valid drop
        ├── snapshot = queryClient.getQueryData(['proxy-hosts'])
        ├── optimistic cache update (proxy_group + proxy_group_id)
        ├── reset activeDragId / overGroupId (DragOverlay closes)
        └── await bulkUpdateGroup(hostsBeingDragged, targetGroupUuid | null)
              ├── success → invalidateQueries + toast.success + clear selection
              └── error   → restore snapshot + toast.error
```

---

## 5. Database Schema

No schema changes required. `proxy_group_id` FK already exists on `proxy_hosts`.

---

## 6. Implementation Phases

### Phase 1: Playwright Tests (Acceptance Criteria Scaffold)

**New file**: `tests/proxy-host-drag-drop.spec.ts`

Scaffold the full E2E spec before implementation:

```typescript
import { test, expect } from '@playwright/test';

test.describe('Proxy Host Drag-and-Drop Group Assignment', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/proxy-hosts');
  });

  test('drag handle appears in grouped view', async ({ page }) => { ... });
  test('drag handle absent in flat view', async ({ page }) => { ... });
  test('drag single unselected host to different group', async ({ page }) => { ... });
  test('drag selected host moves all selected hosts', async ({ page }) => { ... });
  test('drag host to Ungrouped section removes group', async ({ page }) => { ... });
  test('drop zone highlights while dragging over it', async ({ page }) => { ... });
  test('ungrouped zone visible during drag even when empty', async ({ page }) => { ... });
  test('keyboard: Space pick-up, arrow navigation, Space drop', async ({ page }) => { ... });
  test('Escape cancels drag with no state change', async ({ page }) => { ... });
});
```

**Validation gate**: Tests are written in Commit 8 using the acceptance criteria defined in Phase 1 as a guide.

### Phase 2: Backend — Bulk Update Group Endpoint

**Files**:
- `backend/internal/api/handlers/proxy_host_handler.go`

**Changes**:
1. Add `BulkUpdateGroup` method (modeled exactly on `BulkUpdateACL` lines 804–860).
2. Register `router.PUT("/proxy-hosts/bulk-update-group", h.BulkUpdateGroup)` in `RegisterRoutes`.

**Complexity**: Low.
**Validation gate**: `go test ./backend/...` passes. `curl -X PUT .../bulk-update-group` returns expected JSON.

### Phase 3: Frontend API + Hook

**Files**:
- `frontend/src/api/proxyHosts.ts`
- `frontend/src/hooks/useProxyHosts.ts`

**Changes**: Per §3.2 and §3.3.
**Validation gate**: Vitest passes. New unit test `useProxyHosts-bulkGroup.test.tsx` mocks the API and asserts the mutation works and invalidates the query.

### Phase 4: New Components + DataTable Extension

**Files** (created/modified):
- `frontend/src/components/ProxyHostDragHandle.tsx` (new)
- `frontend/src/components/GroupDropZone.tsx` (new)
- `frontend/src/components/ui/DataTable.tsx` (modified — `renderDragHandle` prop)

**Changes**: Per §3.5, §3.6, §3.7.
**Validation gate**: `npm run type-check --prefix frontend` passes. Vitest component tests pass.

### Phase 5: DnD Library + Hook

**Files**:
- `frontend/package.json` + lock file
- `frontend/src/hooks/useProxyGroupDnD.ts` (new)

**Install command**:
```bash
cd frontend && npm install @dnd-kit/core @dnd-kit/utilities
```

> `@dnd-kit/utilities` provides `CSS.Transform.toString()`, which is used to apply the DragOverlay transform style during pointer movement.

**Changes**: Per §3.4.
**Validation gate**: `npm run type-check --prefix frontend` passes. Hook unit tests pass.

### Phase 6: Wire ProxyHosts.tsx

**Files**:
- `frontend/src/pages/ProxyHosts.tsx`

**Changes**: Per §3.9.
**Validation gate**: `npm run type-check` passes. Manual smoke test: drag host between groups in browser.

### Phase 7: i18n + Tests

**Files**:
- `frontend/src/locales/en/translation.json` (and all other locale files)
- `tests/proxy-host-drag-drop.spec.ts` (complete the spec from Phase 1)
- `frontend/src/hooks/__tests__/useProxyGroupDnD.test.ts` (new)
- `frontend/src/components/__tests__/ProxyHostDragHandle.test.tsx` (new)
- `frontend/src/components/__tests__/GroupDropZone.test.tsx` (new)

**`useProxyGroupDnD.test.ts` — minimum test cases**:

| Test | Description |
|------|-------------|
| Happy path | Drag single host to new group → API called once, cache updated optimistically |
| No-op | Drag host to same group → API NOT called, state unchanged |
| API error | Mutation fails → cache rolled back to snapshot |
| Partial success | `result.errors.length > 0` → no rollback (partial success is accepted) |
| Rapid second drag | Second drag while first is pending → first completes before second begins (sequential, not parallel) |

**Validation gate**: All Vitest + Playwright tests pass. `scripts/scan-gorm-security.sh --check` passes.

---

## 7. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| 1 | GripVertical drag handle on each row in grouped view | Playwright |
| 2 | No drag handle in flat (no-groups) view | Playwright |
| 3 | Single host drags to different group | Playwright + API |
| 4 | Selected host drag moves all selected hosts | Playwright |
| 5 | Unselected host drag moves only that host | Playwright |
| 6 | Drop on "Ungrouped" sets `proxy_group_id = null` | Playwright |
| 7 | Drop zone ring highlights on hover | Playwright visual |
| 8 | Ungrouped zone visible during drag when empty | Playwright |
| 9 | Optimistic update before API returns | Manual |
| 10 | API failure rolls back optimistic update | Vitest (mocked failure) |
| 11 | Alphabetical order preserved within groups | Playwright |
| 12 | Keyboard DnD: Space/Arrow/Space flow | Playwright a11y |
| 13 | Screen reader announcements fire correctly | ARIA code review |
| 14 | Bulk endpoint calls Caddy config once | Backend unit test |
| 15 | `npm run type-check` passes | CI |
| 16 | Vitest ≥ 85% overall coverage | CI |
| 17 | GORM security scan: 0 CRITICAL/HIGH | `scripts/scan-gorm-security.sh --check` |

---

## 8. Accessibility

### Keyboard Interaction

| Key | Effect |
|-----|--------|
| `Tab` | Focus next drag handle |
| `Space` | Pick up item (start drag) |
| `Arrow keys` | Move focus between drop zones |
| `Space` / `Enter` | Drop into focused group |
| `Escape` | Cancel drag, item stays in original group |

### ARIA Attributes

- Drag handle: `role="button"` (from dnd-kit `attributes`), `aria-roledescription`, `aria-label` (describes count if multi-select)
- Drop zones: `aria-dropeffect="move"` while drag is active
- Live region: `aria-live="assertive"` — managed automatically by `DndContext`'s `accessibility.announcements`

### Visual Requirements

- Drag handle focus indicator: `focus-visible:ring-2 focus-visible:ring-brand-500` (≥ 3:1 contrast on UI boundary)
- Drop zone highlight: `ring-2 ring-brand-400` on `surface-base` (≥ 3:1 contrast for UI component boundary)
- `DragOverlay` card: `shadow-xl` for clear visual layering
- Handle cursor: `cursor-grab` (default), `cursor-grabbing` (active)

---

## 9. Edge Cases

| Scenario | Behavior |
|----------|----------|
| Drop on same group | No-op detected before optimistic update, returns early |
| All dragged hosts fail | Full rollback, `toast.error` |
| Some dragged hosts fail | Partial toast, `invalidateQueries` reconciles |
| Group deleted while drag in progress | 404 from API, rollback + toast error |
| `groups.length === 0` | Flat `DataTable` rendered, `DndContext` not mounted, no drag handles |
| Click on drag handle (no drag) | `activationConstraint: { distance: 8 }` prevents; `onClick` stopPropagation prevents row toggle |
| Touch device | `PointerSensor` handles native Pointer Events (touch included) |
| Rapid consecutive drags | Optimistic cache is source-of-truth until `invalidateQueries` reconciles |

---

## 10. Commit Slicing Strategy

**Decision**: Single PR with 8 ordered logical commits. Each commit is independently valid and does not break the build.

| # | Commit | Scope | Files | Dependencies | Validation Gate |
|---|--------|-------|-------|-------------|----------------|
| 1 | `chore(deps): add @dnd-kit/core and @dnd-kit/utilities` | deps | `frontend/package.json`, `frontend/package-lock.json` | none | `npm run type-check` |
| 2 | `feat(backend): add BulkUpdateGroup handler and route` | backend API | `proxy_host_handler.go` | none | `go test ./backend/...` |
| 3 | `feat(api): add bulkUpdateGroup to frontend API and hook` | frontend API | `proxyHosts.ts`, `useProxyHosts.ts` | Commit 2 | Vitest |
| 4 | `feat(components): add DnD components and extend DataTable` | components | `ProxyHostDragHandle.tsx`, `GroupDropZone.tsx`, `DataTable.tsx` | Commit 1 | `type-check` + Vitest |
| 5 | `feat(hooks): add useProxyGroupDnD with optimistic update` | hook | `useProxyGroupDnD.ts` | Commits 1, 3 | Vitest |
| 6 | `feat(pages): wire DnD into ProxyHosts grouped view` | page | `ProxyHosts.tsx` | Commits 4, 5 | `type-check` + manual smoke |
| 7 | `feat(i18n): add DnD translation keys` | i18n | locale files | Commit 6 | `npm run type-check` |
| 8 | `test(dnd): add DnD unit + E2E test suite` | tests | spec files, `__tests__/*.tsx` | Commit 7 | Full test suite + Playwright |

**Rollback note**: Commits 1–5 have zero visible UI impact. Commit 6 is the feature toggle. If Commit 6 must be reverted, prior commits remain in place harmlessly and can be re-applied.

---

## 11. File Inventory

### New Files

| File | Purpose |
|------|---------|
| `frontend/src/components/ProxyHostDragHandle.tsx` | Drag handle using `useDraggable` |
| `frontend/src/components/GroupDropZone.tsx` | Drop zone wrapper using `useDroppable` |
| `frontend/src/hooks/useProxyGroupDnD.ts` | All DnD state logic, optimistic updates, rollback |
| `tests/proxy-host-drag-drop.spec.ts` | Playwright E2E spec |
| `frontend/src/hooks/__tests__/useProxyGroupDnD.test.ts` | Unit tests for hook |
| `frontend/src/components/__tests__/ProxyHostDragHandle.test.tsx` | Unit tests for drag handle |
| `frontend/src/components/__tests__/GroupDropZone.test.tsx` | Unit tests for drop zone |

### Modified Files

| File | Change |
|------|--------|
| `frontend/package.json` | Add `@dnd-kit/core`, `@dnd-kit/utilities` |
| `frontend/src/api/proxyHosts.ts` | Add `BulkUpdateGroupRequest`, `BulkUpdateGroupResponse`, `bulkUpdateGroup` |
| `frontend/src/hooks/useProxyHosts.ts` | Add `bulkUpdateGroup` mutation + expose from hook |
| `frontend/src/components/ui/DataTable.tsx` | Add `renderDragHandle?: (row: T) => React.ReactNode` prop |
| `frontend/src/pages/ProxyHosts.tsx` | Wrap grouped view in `DndContext`, use `GroupDropZone` and `ProxyHostDragHandle` |
| `frontend/src/locales/en/translation.json` | Add `proxyGroups.dnd.*` keys |
| `backend/internal/api/handlers/proxy_host_handler.go` | Add `BulkUpdateGroup` handler, register route |

---

## 12. Risks and Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|-----------|
| `@dnd-kit` bundle size | Low | Core + utilities only (~12 KB gzipped). No `@dnd-kit/sortable`. |
| Accidental drag on row click | Medium | `activationConstraint: { distance: 8 }` + `onClick` stopPropagation on handle |
| Touch device drag sensitivity | Medium | `PointerSensor` inherently handles touch; `distance: 8` prevents accidental drags |
| N Caddy rebuilds for multi-select | Mitigated | Bulk endpoint calls `ApplyConfig` once per drag action |
| ESLint `exhaustive-deps` | Medium | Use `useCallback` with correct deps; CI lint will catch |
| Race: rapid consecutive drags | Low | Optimistic cache is authoritative; `invalidateQueries` reconciles after each API call |
