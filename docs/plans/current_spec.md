# PR #1018 Coverage Gap Remediation — Proxy Groups Feature

**Status**: Active
**Target**: PR #1018 (`feature/proxy_groups` → `main`)

---

## 1. Introduction

### Overview

PR #1018 introduces proxy group management: hosts can be assigned to named groups, dragged between groups via a drag-and-drop interface, and bulk-reassigned via a new `BulkUpdateGroup` API endpoint. Codecov reports **84.12% patch coverage**, failing the **90% CI gate**. This spec identifies every uncovered block and specifies the exact tests to close the gaps.

### Objectives

1. Bring Codecov patch coverage from 84.12% to ≥ 90%.
2. Identify all uncovered lines in changed files.
3. Specify tests with enough detail for direct implementation without further research.
4. Address one confirmed dead-code block via code removal rather than a contrived test.

---

## 2. Research Findings

### 2.1 Architecture

- **Backend**: Gin + GORM + SQLite (in-memory for tests). Handlers: `backend/internal/api/handlers/`. Key file: `proxy_host_handler.go`.
- **Frontend**: React + TypeScript + Vitest. API: `frontend/src/api/`, hooks: `frontend/src/hooks/`, components: `frontend/src/components/`.
- **Coverage tooling**: `scripts/go-test-coverage.sh` (backend), `scripts/frontend-test-coverage.sh` (frontend), `scripts/local-patch-report.sh` (patch delta report).

### 2.2 Confirmed Uncovered Lines — Backend

Source: fresh coverage profile `backend/internal/api/handlers/coverage_handlers.txt` (4518 lines).

File: `backend/internal/api/handlers/proxy_host_handler.go`

| Block | Lines | Description | Root Cause |
|---|---|---|---|
| `271.19,273.3 1 0` | 271–273 | `if trimmed == ""` in `resolveProxyGroupReference` | **Dead code** — `parseNullableUintField` handles blank strings internally, returning `nil, nil` (no error), so the early-return at `if parseErr == nil` fires before this check |
| `420.3,420.46 1 0` | 420 | `payload["proxy_group_id"] = resolvedGroupID` in Create handler | No test creates a proxy host with a valid ProxyGroup UUID in the DB |
| `635.3,635.38 1 0` | 635 | `host.ProxyGroupID = resolvedGroupID` in Update handler | No test updates a proxy host with a valid ProxyGroup UUID in the DB |
| `904.48,909.12 2 0` | 904–909 | `errors = append(...)` on `service.Update` failure in `BulkUpdateGroup` | Existing PartialFailure test only exercises `GetByUUID` failure; `service.Update` never fails |
| `914.42,915.73 1 0` | 914 | `if updated > 0 && h.caddyManager != nil` in `BulkUpdateGroup` | All test setups pass `nil` for `caddyManager` |
| `915.73,922.4 2 0` | 915–922 | `caddyManager.ApplyConfig(...)` error path in `BulkUpdateGroup` | Same as above |

### 2.3 Dead Code Analysis — Lines 271–273

```go
// parseNullableUintField (line ~170)
case string:
    trimmed := strings.TrimSpace(v)
    if trimmed == "" {
        return nil, nil  // ← blank strings return nil,nil — NO error raised
    }

// resolveProxyGroupReference (line 260)
func (h *ProxyHostHandler) resolveProxyGroupReference(value any) (*uint, error) {
    parsedID, parseErr := parseNullableUintField(value, "proxy_group_id")
    if parseErr == nil {
        return parsedID, nil  // ← blank strings exit here (parseErr is nil)
    }
    uuidValue, isString := value.(string)
    ...
    trimmed := strings.TrimSpace(uuidValue)
    if trimmed == "" {        // ← DEAD: blank strings already handled above
        return nil, nil       // ← lines 271-273: never reached
    }
    ...
}
```

**Conclusion**: Unreachable. Removal is the correct fix; no test can cover it.

### 2.4 Caddy Manager Pattern in Existing Tests

`TestProxyHostErrors` (line ~364 of test file) demonstrates the established pattern for testing `caddyManager` error paths. No interface extraction is needed — a real `caddy.Manager` is constructed with a failing `httptest.Server`:

```go
caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusInternalServerError)
}))
client := caddy.NewClientWithExpectedPort(caddyServer.URL, expectedPortFromURL(t, caddyServer.URL))
manager := caddy.NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})
h := NewProxyHostHandler(db, manager, ns, nil)
```

This same pattern applies to `BulkUpdateGroup` (lines 914–922).

### 2.5 Existing PR Test Functions (lines 2381+)

Already in `proxy_host_handler_test.go`:

- `setupTestRouterWithProxyGroupTable` — router helper with `ProxyGroup` + `ProxyHost` auto-migrate
- `TestProxyHostHandler_ResolveProxyGroupReference_TargetedBranches`
- `TestProxyHostCreate_WithProxyGroupReference_BadUUID_400`
- `TestProxyHostUpdate_WithProxyGroupReference_BadUUID_400`
- `TestProxyHostHandler_BulkUpdateGroup_Success`
- `TestProxyHostHandler_BulkUpdateGroup_Ungrouped`
- `TestProxyHostHandler_BulkUpdateGroup_InvalidGroup`
- `TestProxyHostHandler_BulkUpdateGroup_PartialFailure` — exercises `GetByUUID` failure only
- `TestProxyHostHandler_BulkUpdateGroup_EmptyUUIDs`
- `TestProxyHostHandler_BulkUpdateGroup_InvalidJSON`

### 2.6 Confirmed Uncovered Lines — Frontend

| File | Gap | Root Cause |
|---|---|---|
| `frontend/src/api/proxyHosts.ts` | `bulkUpdateGroup()` function body | Added in PR; `proxyHosts.test.ts` has no tests for it |
| `frontend/src/hooks/useProxyHosts.ts` | `bulkGroupMutation` and `bulkUpdateGroup` wrapper | `useProxyHosts-bulk.test.tsx` + `useProxyHosts.test.tsx` have zero `bulkUpdateGroup` tests |
| `frontend/src/components/GroupDropZone.tsx` | `isOver` ring-style branch; `isDragActive` aria-dropeffect branch | No `GroupDropZone.test.tsx` exists |
| `frontend/src/components/ProxyHostDragHandle.tsx` | `isDragging` opacity branch; `dragCount > 1` aria-label branch | No `ProxyHostDragHandle.test.tsx` exists |
| `frontend/src/components/ui/DataTable.tsx` | `renderDragHandle` prop — header cell, row cell, colSpan (lines 103, 122, 225–231) | No DataTable test exercises `renderDragHandle` |

---

## 3. Technical Specifications

### 3.1 Backend Tests

#### 3.1.1 `TestProxyHostCreate_WithProxyGroupReference_ValidUUID_201`

**Covers**: Line 420 (`payload["proxy_group_id"] = resolvedGroupID`)

**Location**: `backend/internal/api/handlers/proxy_host_handler_test.go` — after `TestProxyHostCreate_WithProxyGroupReference_BadUUID_400`

**Setup**:
1. `router, db := setupTestRouterWithProxyGroupTable(t)`
2. Insert `models.ProxyGroup{Name: "test-group", Color: "#123456"}` directly via `db.Create(&pg)`
3. POST to `/api/v1/proxy-hosts` with all required fields plus `"proxy_group_id": "<pg.UUID>"`

**Assertions**:
- Response code: 201
- Response body `proxy_group_id` field equals `pg.UUID`

**Minimal request body**:
```json
{
  "name": "Host With Group",
  "domain_names": "with-group.test.local",
  "forward_scheme": "http",
  "forward_host": "localhost",
  "forward_port": 8080,
  "enabled": true,
  "proxy_group_id": "<pg.UUID>"
}
```

---

#### 3.1.2 `TestProxyHostUpdate_WithProxyGroupReference_ValidUUID_200`

**Covers**: Line 635 (`host.ProxyGroupID = resolvedGroupID`)

**Location**: After 3.1.1

**Setup**:
1. `router, db := setupTestRouterWithProxyGroupTable(t)`
2. Insert ProxyGroup directly in DB
3. Insert ProxyHost directly in DB (bypass handler to avoid caddy error): `db.Create(&host)`
4. PUT to `/api/v1/proxy-hosts/<host.UUID>` with `"proxy_group_id": "<pg.UUID>"`

**Assertions**:
- Response code: 200
- Response body contains `proxy_group_id` equal to `pg.UUID`

---

#### 3.1.3 `TestProxyHostHandler_BulkUpdateGroup_ServiceUpdateError`

**Covers**: Lines 904–909 (`errors = append(...)` on `service.Update` failure)

**Location**: After `TestProxyHostHandler_BulkUpdateGroup_PartialFailure`

**Strategy**: Use a SQLite BEFORE UPDATE trigger to force `service.Update` to fail while `GetByUUID` (SELECT) succeeds.

**Setup**:
1. `router, db := setupTestRouterWithProxyGroupTable(t)` (or inline DB creation)
2. Create ProxyGroup and ProxyHost in DB
3. Install a SQLite abort trigger on the proxy_hosts table:
   ```go
   db.Exec(`CREATE TRIGGER fail_update BEFORE UPDATE ON proxy_hosts
            BEGIN SELECT RAISE(ABORT,'forced update failure'); END`)
   ```
4. PUT to `/api/v1/proxy-hosts/bulk-update-group` with `host_uuids: [host.UUID]` and `proxy_group_id: pg.UUID`

**Assertions**:
- Response code: 200
- Response body `updated` equals 0
- Response body `errors` is a non-empty array containing an entry with the host UUID

**Why this works**: The RAISE(ABORT,...) trigger fires when GORM executes the UPDATE statement, returning a database error. The SELECT used by `GetByUUID` is unaffected (trigger is BEFORE UPDATE only). This isolates lines 904–909 distinctly from the existing PartialFailure test which tests `GetByUUID` failure.

---

#### 3.1.4 `TestProxyHostHandler_BulkUpdateGroup_CaddyApplyError`

**Covers**: Lines 914–922 (`caddyManager.ApplyConfig` error path)

**Location**: After 3.1.3

**Pattern**: Same as `TestProxyHostErrors` (line ~364 of test file).

**Setup**:
1. Create `httptest.Server` returning 500 for all requests
2. Create in-memory SQLite DB and auto-migrate: `ProxyGroup`, `ProxyHost`, `Location`, `Setting`, `CaddyConfig`, `Notification`, `NotificationProvider`
3. Create `caddy.Manager` via `caddy.NewClientWithExpectedPort` + `caddy.NewManager`
4. Create handler: `NewProxyHostHandler(db, manager, ns, nil)`
5. Create ProxyGroup and ProxyHost directly in DB
6. PUT to `/api/v1/proxy-hosts/bulk-update-group` with valid `host_uuids` and `proxy_group_id`

**Assertions**:
- Response code: 500
- Response body `error` field contains `"Failed to apply"`

**Full test scaffold**:
```go
func TestProxyHostHandler_BulkUpdateGroup_CaddyApplyError(t *testing.T) {
    t.Parallel()

    caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer caddyServer.Close()

    dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
    db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(
        &models.ProxyGroup{}, &models.ProxyHost{}, &models.Location{},
        &models.Setting{}, &models.CaddyConfig{},
        &models.Notification{}, &models.NotificationProvider{},
    ))

    tmpDir := t.TempDir()
    client := caddy.NewClientWithExpectedPort(caddyServer.URL, expectedPortFromURL(t, caddyServer.URL))
    manager := caddy.NewManager(client, db, tmpDir, "", false, config.SecurityConfig{})
    ns := services.NewNotificationService(db, nil)
    h := NewProxyHostHandler(db, manager, ns, nil)
    r := gin.New()
    api := r.Group("/api/v1")
    h.RegisterRoutes(api)

    pg := models.ProxyGroup{Name: "caddy-error-group", Color: "#ff0000"}
    require.NoError(t, db.Create(&pg).Error)
    host := models.ProxyHost{
        UUID: uuid.NewString(), Name: "Caddy Error Host",
        DomainNames: "caddy-err.test.local", ForwardScheme: "http",
        ForwardHost: "localhost", ForwardPort: 8080, Enabled: true,
    }
    require.NoError(t, db.Create(&host).Error)

    body := fmt.Sprintf(`{"host_uuids":["%s"],"proxy_group_id":"%s"}`, host.UUID, pg.UUID)
    req := httptest.NewRequest(http.MethodPut, "/api/v1/proxy-hosts/bulk-update-group", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()
    r.ServeHTTP(resp, req)

    require.Equal(t, http.StatusInternalServerError, resp.Code)
    var result map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
    require.Contains(t, result["error"].(string), "Failed to apply")
}
```

---

#### 3.1.5 Dead Code Removal — Lines 271–273

**File**: `backend/internal/api/handlers/proxy_host_handler.go`

**Change**: Remove the 3-line `if trimmed == ""` block from `resolveProxyGroupReference` (lines 271–273). The variable `trimmed` is still needed for the DB lookup below, so only the `if trimmed == ""` guard is removed.

**Before**:
```go
trimmed := strings.TrimSpace(uuidValue)
if trimmed == "" {
    return nil, nil
}
```

**After**:
```go
trimmed := strings.TrimSpace(uuidValue)
```

**Impact**: Removes 3 lines from the coverage denominator. Codecov will no longer count them as missed. No behavioral change — `parseNullableUintField` already handles blank strings.

---

### 3.2 Frontend Tests

#### 3.2.1 `bulkUpdateGroup` API Function Tests

**File**: `frontend/src/api/__tests__/proxyHosts-bulk.test.ts`

**Test 1** — group assignment:
```typescript
it('calls PUT /proxy-hosts/bulk-update-group with host_uuids and proxy_group_id', async () => {
  const mockResponse = { updated: 2, errors: [] }
  vi.mocked(client.put).mockResolvedValueOnce({ data: mockResponse })
  const result = await bulkUpdateGroup(['uuid-1', 'uuid-2'], 'group-uuid')
  expect(client.put).toHaveBeenCalledWith(
    '/proxy-hosts/bulk-update-group',
    { host_uuids: ['uuid-1', 'uuid-2'], proxy_group_id: 'group-uuid' }
  )
  expect(result).toEqual(mockResponse)
})
```

**Test 2** — ungrouped (null):
```typescript
it('sends null proxy_group_id when ungrouping hosts', async () => {
  vi.mocked(client.put).mockResolvedValueOnce({ data: { updated: 1, errors: [] } })
  await bulkUpdateGroup(['uuid-1'], null)
  expect(client.put).toHaveBeenCalledWith(
    '/proxy-hosts/bulk-update-group',
    { host_uuids: ['uuid-1'], proxy_group_id: null }
  )
})
```

---

#### 3.2.2 `bulkGroupMutation` Hook Test

**File**: `frontend/src/hooks/__tests__/useProxyHosts-bulk.test.tsx`

Add to the existing test suite (file already exists, no bulkUpdateGroup tests):

```typescript
it('bulkUpdateGroup calls the API mutation and returns the result', async () => {
  const mockResult = { updated: 1, errors: [] }
  vi.mocked(bulkUpdateGroup).mockResolvedValueOnce(mockResult)

  const { result } = renderHook(() => useProxyHosts(), { wrapper: QueryWrapper })
  const response = await act(() =>
    result.current.bulkUpdateGroup(['uuid-1'], 'group-uuid')
  )

  expect(bulkUpdateGroup).toHaveBeenCalledWith(['uuid-1'], 'group-uuid')
  expect(response).toEqual(mockResult)
})
```

---

#### 3.2.3 `GroupDropZone.tsx` Component Tests

**File**: `frontend/src/components/__tests__/GroupDropZone.test.tsx` *(new file)*

**`@dnd-kit/core` mock** (declare at top of file):
```typescript
vi.mock('@dnd-kit/core', () => ({
  useDroppable: vi.fn(() => ({ setNodeRef: vi.fn(), isOver: false })),
}))
```

**Test cases**:

1. Renders children without ring styles when `isOver` is false:
   - Default mock (`isOver: false`)
   - Assert rendered div does NOT contain class `ring-2`

2. Applies ring styles when `isOver` is true:
   - Override mock: `vi.mocked(useDroppable).mockReturnValue({ setNodeRef: vi.fn(), isOver: true })`
   - Assert rendered div contains class `ring-2`

3. Sets `aria-dropeffect="move"` when `isDragActive` is true:
   - Render with `isDragActive={true}`
   - Assert element has attribute `aria-dropeffect="move"`

4. Omits `aria-dropeffect` when `isDragActive` is false:
   - Render with `isDragActive={false}`
   - Assert element does NOT have `aria-dropeffect` attribute

---

#### 3.2.4 `ProxyHostDragHandle.tsx` Component Tests

**File**: `frontend/src/components/__tests__/ProxyHostDragHandle.test.tsx` *(new file)*

**`@dnd-kit/core` mock**:
```typescript
vi.mock('@dnd-kit/core', () => ({
  useDraggable: vi.fn(() => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    isDragging: false,
  })),
}))
```

**Test cases**:

1. `dragCount=1` → aria-label uses single-host translation key:
   - Render `<ProxyHostDragHandle hostUuid="h1" dragCount={1} />`
   - Assert aria-label matches the single-host i18n string

2. `dragCount=3` → aria-label uses multi-host translation key with count:
   - Render with `dragCount={3}`
   - Assert aria-label contains `3` or the multi-host key

3. `isDragging=true` → applies opacity-30 class:
   - Override mock: `vi.mocked(useDraggable).mockReturnValue({ ..., isDragging: true })`
   - Assert rendered element has class `opacity-30`

4. `isDragging=false` → no opacity class:
   - Default mock
   - Assert rendered element does NOT have class `opacity-30`

---

#### 3.2.5 `DataTable.tsx` — `renderDragHandle` Prop Tests

**File**: `frontend/src/components/ui/__tests__/DataTable.test.tsx` *(extend existing file)*

**Test cases**:

1. No drag column when `renderDragHandle` is not provided:
   - Render `<DataTable columns={cols} data={rows} />` without `renderDragHandle`
   - Assert no `data-testid="drag-handle-header"` (or check that drag-handle column header is absent)

2. Drag column appears when `renderDragHandle` is provided:
   - Render with `renderDragHandle={(row) => <span data-testid={`drag-${row.id}`} />}`
   - Assert `getByTestId('drag-1')` (or similar) exists per row

3. `colSpan` increases when `renderDragHandle` and `selectable` are both provided:
   - Render empty-state table (0 rows) with `selectable=true` and `renderDragHandle` provided
   - Assert the empty-state row's `colSpan` equals `columns.length + 2`

---

### 3.3 API Contract Reference (No Change)

```
PUT /api/v1/proxy-hosts/bulk-update-group
Content-Type: application/json

Request:
  { "host_uuids": ["uuid-1", ...], "proxy_group_id": "group-uuid" | null }

Response 200 (success):
  { "updated": N, "errors": [] }

Response 200 (partial):
  { "updated": N, "errors": [{"uuid": "...", "error": "..."}] }

Response 500 (caddy failure):
  { "error": "Failed to apply configuration: ..." }
```

---

## 4. Implementation Plan

### Phase 1: Playwright Tests

Not applicable. This is a coverage-gap remediation task. The DnD feature behavior is already implemented and tested at the unit level. Playwright E2E tests for proxy group DnD are deferred to a follow-up.

### Phase 2: Backend Implementation

| # | Task | File | Complexity | Covers |
|---|---|---|---|---|
| B1 | Remove dead code (lines 271–273) | `proxy_host_handler.go` | Trivial | Removes uncoverable lines from denominator |
| B2 | Add `TestProxyHostCreate_WithProxyGroupReference_ValidUUID_201` | `proxy_host_handler_test.go` | Low | Line 420 |
| B3 | Add `TestProxyHostUpdate_WithProxyGroupReference_ValidUUID_200` | `proxy_host_handler_test.go` | Low | Line 635 |
| B4 | Add `TestProxyHostHandler_BulkUpdateGroup_ServiceUpdateError` | `proxy_host_handler_test.go` | Medium | Lines 904–909 |
| B5 | Add `TestProxyHostHandler_BulkUpdateGroup_CaddyApplyError` | `proxy_host_handler_test.go` | Medium | Lines 914–922 |

**Execution order**: B1 first (reduces coverage denominator), then B2–B5 in any order.

### Phase 3: Frontend Implementation

| # | Task | File | Complexity | Covers |
|---|---|---|---|---|
| F1 | Add `bulkUpdateGroup` API tests | `api/__tests__/proxyHosts-bulk.test.ts` | Low | `bulkUpdateGroup` function |
| F2 | Add `bulkGroupMutation` hook test | `hooks/__tests__/useProxyHosts-bulk.test.tsx` | Low | Hook mutation wrapper |
| F3 | Create `GroupDropZone.test.tsx` | `components/__tests__/GroupDropZone.test.tsx` | Low-medium | All branches |
| F4 | Create `ProxyHostDragHandle.test.tsx` | `components/__tests__/ProxyHostDragHandle.test.tsx` | Low-medium | All branches |
| F5 | Extend `DataTable.test.tsx` | `components/ui/__tests__/DataTable.test.tsx` | Low | `renderDragHandle` paths |

### Phase 4: Integration and Verification

1. Run backend: `bash scripts/go-test-coverage.sh`
2. Run frontend: `bash scripts/frontend-test-coverage.sh`
3. Run patch report: `bash scripts/local-patch-report.sh`
4. Review `test-results/local-patch-report.md` — all changed files must show ≥ 90%
5. If any gap remains, identify the specific uncovered block and add a targeted test

### Phase 5: Documentation

No documentation changes required beyond this spec. The dead code removal (B1) is self-explanatory from context.

---

## 5. Acceptance Criteria

| # | Criterion | Verification Method |
|---|---|---|
| AC1 | All backend tests pass | `go test ./internal/api/handlers/... -count=1` exits 0 |
| AC2 | Line 420 covered (Create + valid group UUID) | `coverage_handlers.txt`: `420.3,420.46 1 1+` |
| AC3 | Line 635 covered (Update + valid group UUID) | `coverage_handlers.txt`: `635.3,635.38 1 1+` |
| AC4 | Lines 904–909 covered (BulkUpdateGroup Update failure) | `coverage_handlers.txt`: `904.48,909.12 2 1+` |
| AC5 | Lines 914–922 covered (BulkUpdateGroup Caddy error) | `coverage_handlers.txt`: `914.42,915.73 1 1+` and `915.73,922.4 2 1+` |
| AC6 | Dead code at 271–273 removed | No `271.19,273.3 1 0` entry in coverage output |
| AC7 | `bulkUpdateGroup` API function tested | `proxyHosts-bulk.test.ts` covers PUT call with group and null |
| AC8 | `bulkGroupMutation` hook tested | `useProxyHosts-bulk.test.tsx` covers mutation path |
| AC9 | `GroupDropZone` branches covered | `GroupDropZone.test.tsx` covers `isOver` and `isDragActive` |
| AC10 | `ProxyHostDragHandle` branches covered | `ProxyHostDragHandle.test.tsx` covers `isDragging` and `dragCount > 1` |
| AC11 | `DataTable.renderDragHandle` covered | `DataTable.test.tsx` covers column header, row cells, colSpan |
| AC12 | Local patch report ≥ 90% for all changed files | `test-results/local-patch-report.md` passes threshold |
| AC13 | Codecov CI gate passes (≥ 90% patch coverage) | PR CI: `patch/target` check is green |
| AC14 | GORM security scanner passes (0 CRITICAL/HIGH) | `./scripts/scan-gorm-security.sh --check` exits 0 |

---

## 6. Commit Slicing Strategy

**Decision**: Single PR (#1018), three ordered logical commits.

**Rationale**: All changes are scoped to one feature. Three commits provide clean review checkpoints and allow bisect on regressions.

### Commit 1: `refactor(backend): remove unreachable branch in resolveProxyGroupReference`

| Attribute | Value |
|---|---|
| Scope | `backend/internal/api/handlers/proxy_host_handler.go` |
| Files | `proxy_host_handler.go` — 3-line deletion |
| Dependencies | None |
| Validation gate | `go test ./internal/api/handlers/... -count=1` passes; lines 271–273 absent from coverage denominator |

### Commit 2: `test(backend): cover proxy group create, update, and bulk update paths`

| Attribute | Value |
|---|---|
| Scope | `backend/internal/api/handlers/` |
| Files | `proxy_host_handler_test.go` — 4 new test functions (Tasks B2–B5) |
| Dependencies | Commit 1 (dead code removed) |
| Validation gate | Lines 420, 635, 904–909, 914–922 each show hit count ≥ 1 in fresh coverage profile |

### Commit 3: `test(frontend): cover bulkUpdateGroup API, hook, and new DnD components`

| Attribute | Value |
|---|---|
| Scope | `frontend/src/` |
| Files | `api/__tests__/proxyHosts-bulk.test.ts` (F1), `hooks/__tests__/useProxyHosts-bulk.test.tsx` (F2), `components/__tests__/GroupDropZone.test.tsx` (F3, new), `components/__tests__/ProxyHostDragHandle.test.tsx` (F4, new), `components/ui/__tests__/DataTable.test.tsx` (F5, extended) |
| Dependencies | None — frontend tests are independent |
| Validation gate | `npm test` passes; `scripts/local-patch-report.sh` shows ≥ 90% for all frontend changed files |

**Rollback**: Each commit is independently revertable. If Task B4 (SQLite trigger approach) proves flaky, revert Commit 2 and replace with a table-rename approach or accept partial coverage with documented rationale.

**Contingency**: If Codecov patch coverage remains below 90% after all six tasks, run the patch report to identify any remaining gap, then add a targeted test. The `ProxyHosts.tsx` DragOverlay ternary branches (lines 861, 871) are the most likely residual gap; they require simulating an active `@dnd-kit/core` drag state and are deferred as high-complexity.
