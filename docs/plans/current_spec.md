# Proxy Groups for Better Organization — Implementation Specification

**Feature**: Issue #254 — Proxy Groups
**Branch**: `feature/proxy_groups`
**Status**: Planning
**Complexity**: Medium

---

## 1. Executive Summary

Proxy Groups allows users to organize their proxy hosts into named, color-coded groups (folders). A group is a lightweight container that holds zero or more proxy hosts. Groups appear in the Proxy Hosts page as collapsible sections, letting users manage large sets of hosts without losing them in a flat list.

**Goals:**
- Create, rename, recolor, and delete groups via the UI
- Assign proxy hosts to groups (one group per host, optional)
- Display hosts grouped on the Proxy Hosts page (ungrouped hosts shown last)
- Full CRUD API for groups
- Bulk-assign hosts to a group (multi-select in table → assign to group)

**Non-Goals (explicitly out of scope):**
- Nested/hierarchical groups
- Group-level enable/disable toggle (hosts retain individual control)
- Group-level certificate or access list assignment
- Sorting/reordering groups beyond alphabetical (future)
- Tags or multi-group membership

---

## 2. Research Findings

### 2.1 Backend Architecture

**Module**: `github.com/Wikid82/charon/backend`

**GORM Pattern** (confirmed from `models/domain.go`, `models/access_list.go`):
- Internal `ID uint` hidden with `json:"-"`
- External `UUID string` with `json:"uuid" gorm:"uniqueIndex;not null"`
- `BeforeCreate` hook for UUID generation
- No soft delete for this type of model (same pattern as AccessList)

**Routes Registration** (`backend/internal/api/routes/routes.go`):
- AutoMigrate list at ~line 110 — needs `&models.ProxyGroup{}` added
- Handler registration pattern at ~line 760-762 (after ProxyHostHandler):
  ```go
  proxyHostHandler := handlers.NewProxyHostHandler(db, caddyManager, notificationService, uptimeService)
  proxyHostHandler.RegisterRoutes(management)
  ```
- Complex handlers use `RegisterRoutes(management *gin.RouterGroup)` pattern

**ProxyHost FK Pattern** (confirmed from `models/uptime.go`, `models/proxy_host.go`):
```go
ProxyGroupID *uint        `json:"proxy_group_id,omitempty" gorm:"index"`
ProxyGroup   *ProxyGroup  `json:"proxy_group,omitempty" gorm:"foreignKey:ProxyGroupID"`
```

**ProxyHostService.List** (`services/proxyhost_service.go` line 237):
```go
s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Order("updated_at desc").Find(&hosts)
```
Must add `Preload("ProxyGroup")`.

**ProxyHostService.GetByUUID** (line 228):
```go
s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").Preload("SecurityHeaderProfile").Where("uuid = ?", uuidStr).First(&host)
```
Must add `Preload("ProxyGroup")`.

**No existing proxy group code** — zero references in backend or frontend confirmed via search.

### 2.2 Frontend Architecture

**React Query pattern** (`hooks/useProxyHosts.ts`):
```ts
const query = useQuery({ queryKey: QUERY_KEY, queryFn: getProxyHosts });
const mutation = useMutation({ mutationFn: ..., onSuccess: () => queryClient.invalidateQueries(...) });
```

**API client pattern** (`api/accessLists.ts`):
```ts
export const accessListsApi = {
  async list(): Promise<AccessList[]> { ... },
  async get(uuid: string): Promise<AccessList> { ... },
  async create(data: CreateRequest): Promise<AccessList> { ... },
  async update(uuid: string, data: Partial<CreateRequest>): Promise<AccessList> { ... },
  async delete(uuid: string): Promise<void> { ... },
};
```

**Routing** (`App.tsx`): No new top-level route needed — proxy groups are managed from within the Proxy Hosts page via modals.

**Navigation** (`components/Layout.tsx`): No nav change needed — groups are a sub-feature of Proxy Hosts.

**UI Components available**: `Dialog`, `Button`, `Badge`, `Input`, `Textarea`, `DataTable`, `EmptyState`, `Card`

---

## 3. Database Schema Changes

### 3.1 New Table: `proxy_groups`

| Column        | Type     | Constraints              | Notes                      |
|--------------|----------|--------------------------|----------------------------|
| `id`         | INTEGER  | PRIMARY KEY AUTOINCREMENT | Internal, never serialized |
| `uuid`       | TEXT     | UNIQUE NOT NULL           | External identifier        |
| `name`       | TEXT     | NOT NULL, INDEX           | Display name               |
| `description`| TEXT     |                           | Optional                   |
| `color`      | TEXT     | DEFAULT `#6366f1`         | Hex color for badge        |
| `created_at` | DATETIME |                           |                            |
| `updated_at` | DATETIME |                           |                            |

### 3.2 Modified Table: `proxy_hosts`

Add one new nullable column:

| Column           | Type    | Constraints               | Notes               |
|-----------------|---------|---------------------------|---------------------|
| `proxy_group_id` | INTEGER | INDEX, FK → proxy_groups(id) | NULL = ungrouped |

GORM `foreignKey:ProxyGroupID` wires the association. SQLite `ON DELETE SET NULL` is **not** used via GORM tag — the `ProxyGroupService.Delete` method explicitly clears host associations before deleting the group for consistent cross-database behavior.

---

## 4. Backend Implementation Plan

### 4.1 New File: `backend/internal/models/proxy_group.go`

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProxyGroup struct {
	ID          uint      `json:"-" gorm:"primaryKey"`
	UUID        string    `json:"uuid" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"not null;index"`
	Description string    `json:"description" gorm:"type:text"`
	Color       string    `json:"color" gorm:"default:#6366f1"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (g *ProxyGroup) BeforeCreate(tx *gorm.DB) (err error) {
	if g.UUID == "" {
		g.UUID = uuid.New().String()
	}
	return
}
```

**Notes:**
- No `gorm.DeletedAt` — groups are hard-deleted (same as AccessList model)
- Color defaults to indigo (`#6366f1`) matching the project's design system
- No `ProxyHosts []ProxyHost` back-reference to avoid N+1 on group list

### 4.2 Modify: `backend/internal/models/proxy_host.go`

Add after the `DNSProvider *DNSProvider` field, before the next section:

```go
ProxyGroupID *uint        `json:"proxy_group_id,omitempty" gorm:"index"`
ProxyGroup   *ProxyGroup  `json:"proxy_group,omitempty" gorm:"foreignKey:ProxyGroupID"`
```

### 4.3 New File: `backend/internal/services/proxy_group_service.go`

```go
package services

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

var (
	ErrProxyGroupNotFound  = errors.New("proxy group not found")
	ErrProxyGroupNameEmpty = errors.New("proxy group name cannot be empty")
)

type ProxyGroupService struct {
	db *gorm.DB
}

func NewProxyGroupService(db *gorm.DB) *ProxyGroupService {
	return &ProxyGroupService{db: db}
}

func (s *ProxyGroupService) Create(group *models.ProxyGroup) error {
	if group.Name == "" {
		return ErrProxyGroupNameEmpty
	}
	return s.db.Create(group).Error
}

func (s *ProxyGroupService) List() ([]models.ProxyGroup, error) {
	var groups []models.ProxyGroup
	if err := s.db.Order("name asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *ProxyGroupService) GetByUUID(uuidStr string) (*models.ProxyGroup, error) {
	var group models.ProxyGroup
	result := s.db.Where("uuid = ?", uuidStr).Limit(1).Find(&group)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrProxyGroupNotFound
	}
	return &group, nil
}

func (s *ProxyGroupService) Update(group *models.ProxyGroup) error {
	return s.db.Save(group).Error
}

// Delete removes a group and unassigns all its proxy hosts (sets proxy_group_id = NULL).
// Hosts are not deleted. Both operations are wrapped in a transaction for atomicity.
func (s *ProxyGroupService) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProxyHost{}).
			Where("proxy_group_id = ?", id).
			Update("proxy_group_id", nil).Error; err != nil {
			return fmt.Errorf("failed to unassign proxy hosts: %w", err)
		}
		return tx.Delete(&models.ProxyGroup{}, id).Error
	})
}

// GetHostCount returns the number of proxy hosts assigned to a group.
func (s *ProxyGroupService) GetHostCount(id uint) (int64, error) {
	var count int64
	err := s.db.Model(&models.ProxyHost{}).Where("proxy_group_id = ?", id).Count(&count).Error
	return count, err
}
```

### 4.4 New File: `backend/internal/api/handlers/proxy_group_handler.go`

```go
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

type ProxyGroupHandler struct {
	service *services.ProxyGroupService
	db      *gorm.DB
}

func NewProxyGroupHandler(db *gorm.DB) *ProxyGroupHandler {
	return &ProxyGroupHandler{
		service: services.NewProxyGroupService(db),
		db:      db,
	}
}

func (h *ProxyGroupHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/proxy-groups", h.List)
	router.POST("/proxy-groups", h.Create)
	router.GET("/proxy-groups/:uuid", h.Get)
	router.PUT("/proxy-groups/:uuid", h.Update)
	router.DELETE("/proxy-groups/:uuid", h.Delete)
}

func (h *ProxyGroupHandler) List(c *gin.Context) {
	groups, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (h *ProxyGroupHandler) Create(c *gin.Context) {
	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	group := models.ProxyGroup{
		Name:        strings.TrimSpace(payload.Name),
		Description: payload.Description,
		Color:       payload.Color,
	}
	if group.Color == "" {
		group.Color = "#6366f1"
	}
	if err := h.service.Create(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

func (h *ProxyGroupHandler) Get(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}
	count, _ := h.service.GetHostCount(group.ID)
	c.JSON(http.StatusOK, gin.H{
		"uuid":        group.UUID,
		"name":        group.Name,
		"description": group.Description,
		"color":       group.Color,
		"host_count":  count,
		"created_at":  group.CreatedAt,
		"updated_at":  group.UpdatedAt,
	})
}

func (h *ProxyGroupHandler) Update(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}
	var payload struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.Name != nil {
		trimmed := strings.TrimSpace(*payload.Name)
		if trimmed == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		group.Name = trimmed
	}
	if payload.Description != nil {
		group.Description = *payload.Description
	}
	if payload.Color != nil {
		group.Color = *payload.Color
	}
	if err := h.service.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

func (h *ProxyGroupHandler) Delete(c *gin.Context) {
	group, err := h.service.GetByUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proxy group not found"})
		return
	}
	if err := h.service.Delete(group.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

### 4.5 Modify: `backend/internal/api/handlers/proxy_host_handler.go`

**`ProxyHostResponse` struct** — add after `SecurityHeaderProfile` fields:
```go
ProxyGroup   *models.ProxyGroup  `json:"proxy_group,omitempty"`
```

> **Note**: Internal numeric IDs are never exposed in response bodies per GORM security scanner policy.

**`NewProxyHostResponse` function** — add to the return literal:
```go
ProxyGroup:   host.ProxyGroup,
```

**New private method** `resolveProxyGroupReference`:
```go
func (h *ProxyHostHandler) resolveProxyGroupReference(value any) (*uint, error) {
	if value == nil {
		return nil, nil
	}
	parsedID, parseErr := parseNullableUintField(value, "proxy_group_id")
	if parseErr == nil {
		return parsedID, nil
	}
	uuidValue, isString := value.(string)
	if !isString {
		return nil, parseErr
	}
	trimmed := strings.TrimSpace(uuidValue)
	if trimmed == "" {
		return nil, nil
	}
	var pg models.ProxyGroup
	if err := h.db.Select("id").Where("uuid = ?", trimmed).First(&pg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("proxy group not found")
		}
		return nil, fmt.Errorf("failed to resolve proxy group")
	}
	id := pg.ID
	return &id, nil
}
```

**`Create` handler** — add FK resolution before the `json.Unmarshal` block (after existing `resolveAccessListReference`, etc.):
```go
if rawGroupRef, ok := payload["proxy_group_id"]; ok {
	resolvedGroupID, resolveErr := h.resolveProxyGroupReference(rawGroupRef)
	if resolveErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
		return
	}
	payload["proxy_group_id"] = resolvedGroupID
}
```

**`Update` handler** — add to the payload field processing block (after the existing access_list_id block):
```go
if rawGroupRef, ok := payload["proxy_group_id"]; ok {
	resolvedGroupID, resolveErr := h.resolveProxyGroupReference(rawGroupRef)
	if resolveErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": resolveErr.Error()})
		return
	}
	host.ProxyGroupID = resolvedGroupID
}
```

### 4.6 Modify: `backend/internal/services/proxyhost_service.go`

**`List` method** (line ~239) — add `Preload("ProxyGroup")`:
```go
if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").
	Preload("SecurityHeaderProfile").Preload("ProxyGroup").
	Order("updated_at desc").Find(&hosts).Error; err != nil {
```

**`GetByUUID` method** (line ~229) — add `Preload("ProxyGroup")`:
```go
if err := s.db.Preload("Locations").Preload("Certificate").Preload("AccessList").
	Preload("SecurityHeaderProfile").Preload("ProxyGroup").
	Where("uuid = ?", uuidStr).First(&host).Error; err != nil {
```

### 4.7 Modify: `backend/internal/api/routes/routes.go`

**AutoMigrate list** — insert `&models.ProxyGroup{}` before `&models.ProxyHost{}` so the FK constraint resolves correctly:
```go
// Within the db.AutoMigrate(...) call, add:
&models.ProxyGroup{},
// ProxyHost must come after ProxyGroup due to FK dependency
&models.ProxyHost{},  // (already exists)
```

**Handler registration** — add after `proxyHostHandler.RegisterRoutes(management)`:
```go
proxyGroupHandler := handlers.NewProxyGroupHandler(db)
proxyGroupHandler.RegisterRoutes(management)
```

---

## 5. API Contract

All endpoints are under the authenticated `management` router group → `/api/v1/`.

### `GET /api/v1/proxy-groups`

**Description**: List all proxy groups, ordered alphabetically by name.

**Response `200 OK`**:
```json
[
  {
    "uuid": "3f2a1b4c-...",
    "name": "Production",
    "description": "Production services",
    "color": "#6366f1",
    "created_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
  }
]
```

> **`host_count` is NOT returned by the list endpoint.** Frontend computes host counts client-side from the proxies list using the `groupedHosts.byGroupUUID[uuid].length` memo pattern. The detail endpoint (`GET /proxy-groups/:uuid`) optionally includes it via `GetHostCount`.

> **Optional enhancement**: compute counts server-side via subquery:
> ```go
> // Optional enhancement: compute counts server-side
> s.db.Select("proxy_groups.*, COUNT(ph.id) as host_count").
>     Joins("LEFT JOIN proxy_hosts ph ON ph.proxy_group_id = proxy_groups.id").
>     Group("proxy_groups.id").
>     Find(&groups)
> ```

### `POST /api/v1/proxy-groups`

**Request Body**:
```json
{
  "name": "Production",
  "description": "Production-facing services",
  "color": "#ef4444"
}
```

| Field         | Type   | Required | Notes                  |
|--------------|--------|----------|------------------------|
| `name`       | string | Yes      | Non-empty after trim   |
| `description`| string | No       | Defaults to `""`       |
| `color`      | string | No       | Defaults to `#6366f1`  |

**Response `201 Created`**: Full ProxyGroup object.
**Response `400 Bad Request`**: `{ "error": "proxy group name cannot be empty" }`

### `GET /api/v1/proxy-groups/:uuid`

**Response `200 OK`**:
```json
{
  "uuid": "3f2a1b4c-...",
  "name": "Production",
  "description": "Production services",
  "color": "#6366f1",
  "host_count": 5,
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

**Response `404 Not Found`**: `{ "error": "proxy group not found" }`

### `PUT /api/v1/proxy-groups/:uuid`

Partial update — only provided fields are mutated.

**Request Body** (all optional):
```json
{ "name": "Staging", "description": "Staging environment", "color": "#f59e0b" }
```

**Response `200 OK`**: Updated ProxyGroup object.

### `DELETE /api/v1/proxy-groups/:uuid`

**Description**: Delete a group. Assigned proxy hosts have `proxy_group_id` set to `NULL` (hosts are not deleted).

**Response `204 No Content`**: Empty.

### Proxy Host Group Assignment

Hosts are assigned/unassigned via the existing `PUT /api/v1/proxy-hosts/:uuid` endpoint:

```json
{ "proxy_group_id": "3f2a1b4c-..." }
```

Accepts `null` (unassign), a numeric ID, or a UUID string.

The proxy host response now includes:
```json
{
  "proxy_group": {
    "uuid": "3f2a1b4c-...",
    "name": "Production",
    "color": "#6366f1"
  }
}
```

---

## 6. Frontend Implementation Plan

### 6.1 New File: `frontend/src/api/proxyGroups.ts`

```typescript
import client from './client';

export interface ProxyGroup {
  uuid: string;
  name: string;
  description: string;
  color: string;
  host_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProxyGroupRequest {
  name: string;
  description?: string;
  color?: string;
}

export const proxyGroupsApi = {
  async list(): Promise<ProxyGroup[]> {
    const response = await client.get<ProxyGroup[]>('/proxy-groups');
    return response.data;
  },

  async get(uuid: string): Promise<ProxyGroup> {
    const response = await client.get<ProxyGroup>(`/proxy-groups/${uuid}`);
    return response.data;
  },

  async create(data: CreateProxyGroupRequest): Promise<ProxyGroup> {
    const response = await client.post<ProxyGroup>('/proxy-groups', data);
    return response.data;
  },

  async update(uuid: string, data: Partial<CreateProxyGroupRequest>): Promise<ProxyGroup> {
    const response = await client.put<ProxyGroup>(`/proxy-groups/${uuid}`, data);
    return response.data;
  },

  async delete(uuid: string): Promise<void> {
    await client.delete(`/proxy-groups/${uuid}`);
  },
};
```

### 6.2 New File: `frontend/src/hooks/useProxyGroups.ts`

Export the following named hooks (multi-hook pattern, same as `useAccessLists.ts`):

```typescript
export const PROXY_GROUPS_QUERY_KEY = ['proxy-groups'];

export function useProxyGroups(): UseQueryResult<ProxyGroup[]>
export function useCreateProxyGroup(): UseMutationResult<...>
export function useUpdateProxyGroup(): UseMutationResult<...>
export function useDeleteProxyGroup(): UseMutationResult<...>
```

- `useCreateProxyGroup`: on success → `toast.success('Group created')` + invalidate `PROXY_GROUPS_QUERY_KEY`; on error → `toast.error(\`Failed to create group: ${error.message}\`)`
- `useUpdateProxyGroup`: mutation fn takes `{ uuid: string; data: Partial<CreateProxyGroupRequest> }` → `toast.success('Group updated')`; on error → `toast.error(\`Failed to update group: ${error.message}\`)`
- `useDeleteProxyGroup`: on success → `toast.success('Group deleted — hosts moved to Ungrouped')` + invalidate both `PROXY_GROUPS_QUERY_KEY` and `['proxy-hosts']`; on error → `toast.error(\`Failed to delete group: ${error.message}\`)`

### 6.3 Modify: `frontend/src/api/proxyHosts.ts`

Add to the `ProxyHost` interface:
```typescript
proxy_group_id?: number | string | null;
proxy_group?: {
  uuid: string;
  name: string;
  color: string;
} | null;
```

### 6.4 New File: `frontend/src/components/ProxyGroupBadge.tsx`

```typescript
interface ProxyGroupBadgeProps {
  group: { name: string; color: string };
  className?: string;
}
```

Renders a colored circle dot followed by the group name. Uses inline `style={{ backgroundColor: group.color }}` for the dot. Falls back to `—` when group is undefined (caller decides).

### 6.5 New File: `frontend/src/components/ProxyGroupForm.tsx`

Create/edit dialog for a proxy group.

**Props**:
```typescript
interface ProxyGroupFormProps {
  open: boolean;
  onClose: () => void;
  group?: ProxyGroup; // undefined = create mode
}
```

**Fields**:
- `name` — `Input` (required, validates non-empty)
- `description` — `Textarea` (optional)
- `color` — 8 preset color swatches (clickable circles) + hex text input

**Color presets**: `['#6366f1', '#ef4444', '#f59e0b', '#10b981', '#3b82f6', '#8b5cf6', '#ec4899', '#6b7280']`

**Behavior**:
- Create mode: calls `useCreateProxyGroup().mutateAsync(data)` then `onClose()`
- Edit mode: calls `useUpdateProxyGroup().mutateAsync({ uuid: group.uuid, data })` then `onClose()`
- Submit button disabled when `name.trim() === ''`
- Uses `Dialog`, `Input`, `Textarea`, `Button` from `components/ui`

### 6.6 Modify: `frontend/src/pages/ProxyHosts.tsx`

**New imports**:
```typescript
import { useProxyGroups, useDeleteProxyGroup, useUpdateProxyGroup } from '../hooks/useProxyGroups';
import ProxyGroupForm from '../components/ProxyGroupForm';
import ProxyGroupBadge from '../components/ProxyGroupBadge';
```

**New state**:
```typescript
const [showGroupForm, setShowGroupForm] = useState(false);
const [editingGroup, setEditingGroup] = useState<ProxyGroup | undefined>();
const [groupToDelete, setGroupToDelete] = useState<ProxyGroup | null>(null);
const [showAssignGroupModal, setShowAssignGroupModal] = useState(false);
const [assignTargetGroupUUID, setAssignTargetGroupUUID] = useState<string | null>(null);
```

**New data hooks**:
```typescript
const { data: groups = [] } = useProxyGroups();
const deleteGroupMutation = useDeleteProxyGroup();
const updateProxyHostMutation = /* existing update from useProxyHosts */;
```

**Grouped hosts memo** (replaces or supplements `sortedHosts`):
```typescript
const groupedHosts = useMemo(() => {
  const byGroupUUID: Record<string, ProxyHost[]> = {};
  const ungrouped: ProxyHost[] = [];
  for (const host of sortedHosts) {
    if (host.proxy_group?.uuid) {
      if (!byGroupUUID[host.proxy_group.uuid]) byGroupUUID[host.proxy_group.uuid] = [];
      byGroupUUID[host.proxy_group.uuid].push(host);
    } else {
      ungrouped.push(host);
    }
  }
  return { byGroupUUID, ungrouped };
}, [sortedHosts]);
```

**Page header addition** — "Manage Groups" button (next to "Add Proxy Host" button):
```tsx
<Button variant="outline" onClick={() => { setEditingGroup(undefined); setShowGroupForm(true); }}>
  {t('proxyGroups.manageGroups')}
</Button>
```

**Table column addition** — `Group` column in the DataTable column definitions:
```typescript
{
  key: 'proxy_group',
  header: t('proxyGroups.group'),
  render: (host: ProxyHost) =>
    host.proxy_group
      ? <ProxyGroupBadge group={host.proxy_group} />
      : <span className="text-muted-foreground text-sm">—</span>,
}
```

**Grouped rendering** — replace the flat `sortedHosts` table with grouped sections:
- For each group in `groups`: render a section header (`<Card>` with group color swatch, name, host count, edit/delete icon buttons) followed by a `DataTable` scoped to `groupedHosts.byGroupUUID[group.uuid] ?? []`
- After all named groups: render an "Ungrouped" section with `groupedHosts.ungrouped`
- If `groups.length === 0`: render the existing flat table (no regression for users with no groups)

**Bulk assign** — when `selectedHosts.length > 0`, show "Assign to Group" in bulk actions bar:
- Opens a `Dialog` with a radio list of groups + "None (Ungrouped)"
- On confirm: loops over `selectedHosts`, calls `updateHost(uuid, { proxy_group_id: assignTargetGroupUUID ?? null })`
- Uses existing `applyProgress` / loading pattern if available, otherwise sequential mutations

**Dialogs to add**:
1. `ProxyGroupForm` (create/edit) — controlled by `showGroupForm` + `editingGroup`
2. Delete group confirmation — `groupToDelete` drives a `Dialog` that calls `deleteGroupMutation.mutate(groupToDelete.uuid)`
3. Assign to group modal — `showAssignGroupModal`

### 6.7 i18n Keys

Add to all locale files (`frontend/src/locales/en/translation.json`, `frontend/src/locales/es/translation.json`, `frontend/src/locales/fr/translation.json`, `frontend/src/locales/de/translation.json`, `frontend/src/locales/zh/translation.json`):

```json
{
  "proxyGroups": {
    "group": "Group",
    "manageGroups": "Manage Groups",
    "createGroup": "Create Group",
    "editGroup": "Edit Group",
    "deleteGroup": "Delete Group",
    "deleteGroupConfirm": "Delete this group? All hosts will be moved to Ungrouped.",
    "groupName": "Group Name",
    "groupDescription": "Description",
    "groupColor": "Color",
    "noGroups": "No groups yet",
    "ungrouped": "Ungrouped",
    "assignToGroup": "Assign to Group",
    "hostCount_one": "{{count}} host",
    "hostCount_other": "{{count}} hosts"
  }
}
```

---

## 7. Testing Strategy

### Phase 1 — Playwright E2E Tests (write before implementation)

**File**: `tests/proxy-groups.spec.ts`

Imports:
```typescript
import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { getToastLocator } from './utils/ui-helpers';
import { waitForAPIResponse, waitForDialog, waitForLoadingComplete } from './utils/wait-helpers';
```

Test scenarios:
```
test.describe('Proxy Groups', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page);
    await page.goto('/proxy-hosts');
    await waitForLoadingComplete(page);
  });

  test.describe('Group Management', () => {
    test('should create a proxy group with name and color')
    test('should display created group as a section header on Proxy Hosts page')
    test('should edit a group name and color')
    test('should delete a group — hosts become ungrouped')
    test('should prevent creating a group with empty name — submit disabled')
  })

  test.describe('Host Assignment', () => {
    test('should assign a proxy host to a group via host edit form')
    test('should unassign a proxy host from a group')
    test('should bulk-assign selected hosts to a group')
    test('should display host under correct group section')
  })

  test.describe('Grouped Display', () => {
    test('should show group header with correct name, color, and host count')
    test('should show ungrouped section for unassigned hosts')
    test('should show flat list when no groups exist')
  })
})
```

Use role-based locators: `getByRole('button', { name: /manage groups/i })`, `getByRole('heading', { name: groupName })`.

### Phase 2 — Backend Unit Tests

**`backend/internal/services/proxy_group_service_test.go`**:
- `TestProxyGroupService_Create` — happy path, UUID assigned
- `TestProxyGroupService_Create_EmptyName` — returns `ErrProxyGroupNameEmpty`
- `TestProxyGroupService_List` — returns sorted by name
- `TestProxyGroupService_GetByUUID` — found and not found
- `TestProxyGroupService_Update` — patches Name/Description/Color
- `TestProxyGroupService_Delete_ClearsHostAssignments` — host `proxy_group_id` set to NULL, group deleted
- `TestProxyGroupService_GetHostCount` — correct count

**`backend/internal/api/handlers/proxy_group_handler_test.go`**:
- `TestProxyGroupHandler_List_Empty`
- `TestProxyGroupHandler_List_WithGroups`
- `TestProxyGroupHandler_Create_Valid`
- `TestProxyGroupHandler_Create_EmptyName_400`
- `TestProxyGroupHandler_Get_Found`
- `TestProxyGroupHandler_Get_NotFound_404`
- `TestProxyGroupHandler_Update_PartialFields`
- `TestProxyGroupHandler_Update_EmptyName_400`
- `TestProxyGroupHandler_Delete_204`
- `TestProxyGroupHandler_Delete_NotFound_404`

**`backend/internal/services/proxyhost_service_group_test.go`**:
- `TestProxyHostService_List_PreloadsGroup`
- `TestProxyHostService_GetByUUID_PreloadsGroup`

Follow `access_list_service_test.go` pattern: in-memory SQLite (`gorm.Open(sqlite.Open(":memory:"))`), table-driven cases.

### Phase 3 — Frontend Unit Tests

**`frontend/src/components/__tests__/ProxyGroupForm.test.tsx`**:
- Renders create form when no `group` prop
- Renders edit form with pre-filled values when `group` prop provided
- Disables submit when name is empty
- Calls create mutation on submit in create mode
- Calls update mutation on submit in edit mode

**`frontend/src/components/__tests__/ProxyGroupBadge.test.tsx`**:
- Renders group name
- Renders color dot with correct background color

### GORM Security Scanner (mandatory gate)

After implementing models, run:
```bash
./scripts/scan-gorm-security.sh --check
```

Expected: `proxy_group.go` passes all checks (no `json:"id"` exposure, no DTO embedding in response structs).

---

## 8. Commit Slicing Strategy

**Decision**: Single PR with 6 ordered commits. One feature = one PR.

**Rationale**: Commits are strongly ordered by dependency (model → service → handler → routes → frontend types → frontend UI). Each commit is individually testable. The backend/frontend split allows parallel code review.

---

### Commit 1 — `feat(models): add ProxyGroup model and ProxyHost FK`

**Scope**: Backend model layer
**Files**:
- `backend/internal/models/proxy_group.go` ← **new**
- `backend/internal/models/proxy_host.go` ← add `ProxyGroupID *uint` + `ProxyGroup *ProxyGroup`

**Dependencies**: None
**Validation gate**: `go build ./...` passes; GORM scanner passes with zero CRITICAL/HIGH
**Rollback**: Revert 2 files

---

### Commit 2 — `feat(services): add ProxyGroupService and update ProxyHostService preloads`

**Scope**: Service layer
**Files**:
- `backend/internal/services/proxy_group_service.go` ← **new**
- `backend/internal/services/proxyhost_service.go` ← add `Preload("ProxyGroup")` in `List` and `GetByUUID`

**Dependencies**: Commit 1
**Validation gate**: `go test ./backend/internal/services/...` (all existing tests pass)
**Rollback**: Delete `proxy_group_service.go`; revert preload lines in `proxyhost_service.go`

---

### Commit 3 — `feat(handlers): add ProxyGroupHandler and update ProxyHostHandler`

**Scope**: Handler layer + routes registration
**Files**:
- `backend/internal/api/handlers/proxy_group_handler.go` ← **new**
- `backend/internal/api/handlers/proxy_host_handler.go` ← add response fields + `resolveProxyGroupReference` + Update/Create FK handling
- `backend/internal/api/routes/routes.go` ← add `ProxyGroup` to AutoMigrate + register handler

**AutoMigrate change** (exact lines in `backend/internal/api/routes/routes.go`):
```go
// backend/internal/api/routes/routes.go — AutoMigrate block
db.AutoMigrate(
    &models.ProxyGroup{},  // NEW — must precede ProxyHost (FK dependency)
    &models.ProxyHost{},
    // ... existing models
)
```

**Dependencies**: Commits 1 + 2
**Validation gate**: `go build ./...`; Docker E2E container rebuilds and health check passes; existing E2E proxy-host tests still pass
**Rollback**: Revert 3 files

---

### Commit 4 — `test(backend): proxy group service and handler unit tests`

**Scope**: Backend test files only
**Files**:
- `backend/internal/services/proxy_group_service_test.go` ← **new**
- `backend/internal/api/handlers/proxy_group_handler_test.go` ← **new**
- `backend/internal/services/proxyhost_service_group_test.go` ← **new**

**Dependencies**: Commit 3
**Validation gate**: `go test ./backend/internal/...` all pass; line coverage ≥ 85% for new service and handler files
**Rollback**: Delete test files (no functional regression)

---

### Commit 5 — `feat(frontend): proxy group API client, hooks, and type updates`

**Scope**: Frontend new files + ProxyHost type extension
**Files**:
- `frontend/src/api/proxyGroups.ts` ← **new**
- `frontend/src/hooks/useProxyGroups.ts` ← **new**
- `frontend/src/components/ProxyGroupForm.tsx` ← **new**
- `frontend/src/components/ProxyGroupBadge.tsx` ← **new**
- `frontend/src/api/proxyHosts.ts` ← add `proxy_group_id` + `proxy_group` to `ProxyHost` interface

**Dependencies**: Commit 3 (API endpoints must exist)
**Validation gate**: `npm run build` passes; `npm run lint` clean
**Rollback**: Delete new files; revert 2 lines in `proxyHosts.ts`

---

### Commit 6 — `feat(ui): integrate proxy groups into ProxyHosts page`

**Scope**: Page integration + i18n
**Files**:
- `frontend/src/pages/ProxyHosts.tsx` ← grouped display, manage groups button, assign modal, Group column
- `frontend/src/locales/en/translation.json` (and `es`, `fr`, `de`, `zh` equivalents) ← add `proxyGroups.*` keys

**Dependencies**: Commit 5
**Validation gate**: All Playwright E2E tests pass (Firefox, Chromium, WebKit); existing proxy-hosts tests pass; new `proxy-groups.spec.ts` passes
**Rollback**: Revert `ProxyHosts.tsx` and i18n changes

---

### Contingency Notes

- **SQLite FK constraint during AutoMigrate**: `ProxyGroup` is registered before `ProxyHost` to ensure the FK resolves. If ordering causes a regression, SQLite's deferred FK enforcement makes this a non-issue in practice, but the ordering is correct.
- **Bulk assign performance**: If >20 hosts selected, consider adding a `PUT /api/v1/proxy-hosts/bulk-update-group` endpoint (same shape as existing `bulk-update-acl`) in a follow-up PR. For v1, sequential mutations are acceptable.
- **Color validation**: No server-side hex validation for v1. The frontend color picker enforces valid hex. If invalid values appear in the database, the UI renders them as-is (no crash risk).
- **Groups with no hosts**: Rendered as empty section with a "No hosts in this group" placeholder. Not hidden to allow users to manage the group structure independently.

---

## 9. Acceptance Criteria / Definition of Done

### Functional

- [ ] User can create a proxy group with name, optional description, and optional color
- [ ] User can edit a group's name, description, or color
- [ ] User can delete a group; all affected proxy hosts become ungrouped (not deleted)
- [ ] User can assign a single proxy host to a group via the host edit form
- [ ] User can unassign a proxy host from a group (set to ungrouped)
- [ ] User can bulk-assign multiple selected hosts to a group
- [ ] Proxy Hosts page displays hosts in grouped sections with group header (name + color + host count)
- [ ] Ungrouped hosts appear in an "Ungrouped" section
- [ ] All existing proxy host functionality is unaffected (Caddy config, SSL, enable/disable)

### Technical

- [ ] `proxy_groups` table created by GORM AutoMigrate on server start
- [ ] `proxy_hosts.proxy_group_id` column created by GORM AutoMigrate on server start
- [ ] All API endpoints return correct HTTP status codes
- [ ] GORM security scanner: zero CRITICAL/HIGH findings for `proxy_group.go`
- [ ] Go unit test coverage ≥ 85% for new files (via local patch report)
- [ ] TypeScript builds without errors (`npm run build`)
- [ ] No ESLint errors (`npm run lint`)
- [ ] No existing Playwright E2E tests regress
- [ ] New Playwright E2E tests in `proxy-groups.spec.ts` pass in Firefox, Chromium, WebKit
- [ ] `go build ./...` passes on clean checkout
- [ ] `golangci-lint run` passes

### Documentation

- [ ] i18n keys added for all new UI text (English locale minimum)
- [ ] Plan archived to `docs/implementation/proxy_groups_COMPLETE.md` on merge

---

## 10. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| GORM AutoMigrate FK order causes constraint error | Low | Medium | Register `ProxyGroup` before `ProxyHost` in AutoMigrate list (specified above) |
| SQLite `ON DELETE` not triggered through GORM | Medium | High | Explicitly clear `proxy_group_id` in `Delete()` before deleting group (implemented in spec) |
| Frontend `DataTable` doesn't support grouped/sectioned rows | Medium | Medium | Use separate `DataTable` instance per group instead of one grouped table |
| Existing Playwright tests depend on flat host list ordering | Low | Medium | Ungrouped section preserves behavior for hosts with no group |
| Bulk assign with >20 hosts creates noticeable UI lag | Low | Low | Acceptable for v1; `bulk-update-group` endpoint is a documented follow-up |
| Color value persisted as invalid hex breaks UI rendering | Low | Low | Frontend enforces valid hex; worst case is an unstyled badge |
