# CrowdSec IP Whitelist Management — Implementation Plan

**Issue**: [#939 — CrowdSec IP Whitelist Management](https://github.com/owner/Charon/issues/939)
**Date**: 2026-05-20
**Status**: Draft — Awaiting Approval
**Priority**: High
**Archived Previous Plan**: Coverage Improvement Plan (patch coverage ≥ 90%) → `docs/plans/archive/patch-coverage-improvement-plan-2026-05-02.md`

---

## 1. Introduction

### 1.1 Overview

CrowdSec enforces IP ban decisions by default. Operators need a way to permanently exempt known-good IPs (uptime monitors, internal subnets, VPN exits, partners) from ever being banned. CrowdSec handles this through its `whitelists` parser, which intercepts alert evaluation and suppresses bans for matching IPs/CIDRs before decisions are even written.

This feature gives Charon operators a first-class UI for managing those whitelist entries: add an IP or CIDR, give it a reason, and have Charon persist it in the database, render the required YAML parser file into the CrowdSec config tree, and signal CrowdSec to reload—all without manual file editing.

### 1.2 Objectives

- Allow operators to add, view, and remove CrowdSec whitelist entries (IPs and CIDRs) through the Charon management UI.
- Persist entries in SQLite so they survive container restarts.
- Generate a `crowdsecurity/whitelists`-compatible YAML parser file on every mutating operation and on startup.
- Automatically install the `crowdsecurity/whitelists` hub parser so CrowdSec can process the file.
- Show the Whitelist tab only when CrowdSec is in `local` mode, consistent with other CrowdSec-only tabs.

---

## 2. Research Findings

### 2.1 Existing CrowdSec Architecture

| Component | Location | Notes |
|---|---|---|
| Hub parser installer | `configs/crowdsec/install_hub_items.sh` | Run at container start; uses `cscli parsers install --force` |
| CrowdSec handler | `backend/internal/api/handlers/crowdsec_handler.go` | ~2750 LOC; `RegisterRoutes` at L2704 |
| Route registration | `backend/internal/api/routes/routes.go` | `crowdsecHandler.RegisterRoutes(management)` at ~L620 |
| CrowdSec startup | `backend/internal/services/crowdsec_startup.go` | `ReconcileCrowdSecOnStartup()` runs before process start |
| Security config | `backend/internal/models/security_config.go` | `CrowdSecMode`, `CrowdSecConfigDir` (via `cfg.Security.CrowdSecConfigDir`) |
| IP/CIDR helper | `backend/internal/security/whitelist.go` | `IsIPInCIDRList()` using `net.ParseIP` / `net.ParseCIDR` |
| AutoMigrate | `routes.go` ~L95–125 | `&models.ManualChallenge{}` is currently the last entry |

### 2.2 Gap Analysis

- `crowdsecurity/whitelists` hub parser is **not** installed by `install_hub_items.sh` — the YAML file would be ignored by CrowdSec without it.
- No `CrowdSecWhitelist` model exists in `backend/internal/models/`.
- No whitelist service, handler methods, or API routes exist.
- No frontend tab, API client functions, or TanStack Query hooks exist.
- No E2E test spec covers whitelist management.

### 2.3 Relevant Patterns

**Model pattern** (from `access_list.go` + `security_config.go`):
```go
type Model struct {
    ID        uint      `json:"-" gorm:"primaryKey"`
    UUID      string    `json:"uuid" gorm:"uniqueIndex;not null"`
    // domain fields
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

**Service pattern** (from `access_list_service.go`):
```go
var ErrXxxNotFound = errors.New("xxx not found")

type XxxService struct { db *gorm.DB }

func NewXxxService(db *gorm.DB) *XxxService { return &XxxService{db: db} }
```

**Handler error response pattern** (from `crowdsec_handler.go`):
```go
c.JSON(http.StatusBadRequest, gin.H{"error": "..."})
c.JSON(http.StatusNotFound, gin.H{"error": "..."})
c.JSON(http.StatusInternalServerError, gin.H{"error": "..."})
```

**Frontend API client pattern** (from `frontend/src/api/crowdsec.ts`):
```typescript
export const listXxx = async (): Promise<XxxEntry[]> => {
  const resp = await client.get<XxxEntry[]>('/admin/crowdsec/xxx')
  return resp.data
}
```

**Frontend mutation pattern** (from `CrowdSecConfig.tsx`):
```typescript
const mutation = useMutation({
  mutationFn: (data) => apiCall(data),
  onSuccess: () => {
    toast.success('...')
    queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
  },
  onError: (err) => toast.error(err instanceof Error ? err.message : '...'),
})
```

### 2.4 CrowdSec Whitelist YAML Format

CrowdSec's `crowdsecurity/whitelists` parser expects the following YAML structure at a path under the `parsers/s02-enrich/` directory:

```yaml
name: charon-whitelist
description: "Charon-managed IP/CIDR whitelist"
filter: "evt.Meta.service == 'http'"
whitelist:
  reason: "Charon managed whitelist"
  ip:
    - "1.2.3.4"
  cidr:
    - "10.0.0.0/8"
    - "192.168.0.0/16"
```

For an empty whitelist, both `ip` and `cidr` must be present as empty lists (not omitted) to produce valid YAML that CrowdSec can parse without error.

---

## 3. Technical Specifications

### 3.1 Database Schema

**New model**: `backend/internal/models/crowdsec_whitelist.go`

```go
package models

import "time"

// CrowdSecWhitelist represents a single IP or CIDR exempted from CrowdSec banning.
type CrowdSecWhitelist struct {
    ID        uint      `json:"-"          gorm:"primaryKey"`
    UUID      string    `json:"uuid"       gorm:"uniqueIndex;not null"`
    IPOrCIDR  string    `json:"ip_or_cidr" gorm:"not null;uniqueIndex"`
    Reason    string    `json:"reason"     gorm:"not null;default:''"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

**AutoMigrate registration** (`backend/internal/api/routes/routes.go`, append after `&models.ManualChallenge{}`):
```go
&models.CrowdSecWhitelist{},
```

### 3.2 API Design

All new endpoints live under the existing `/api/v1` prefix and are registered inside `CrowdsecHandler.RegisterRoutes(rg *gin.RouterGroup)`, following the same `rg.METHOD("/admin/crowdsec/...")` naming pattern as every other CrowdSec endpoint.

#### Endpoint Table

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/admin/crowdsec/whitelist` | Management | List all whitelist entries |
| `POST` | `/api/v1/admin/crowdsec/whitelist` | Management | Add a new entry |
| `DELETE` | `/api/v1/admin/crowdsec/whitelist/:uuid` | Management | Remove an entry by UUID |

#### `GET /admin/crowdsec/whitelist`

**Response 200**:
```json
{
  "whitelist": [
    {
      "uuid": "a1b2c3d4-...",
      "ip_or_cidr": "10.0.0.0/8",
      "reason": "Internal subnet",
      "created_at": "2026-05-20T12:00:00Z",
      "updated_at": "2026-05-20T12:00:00Z"
    }
  ]
}
```

#### `POST /admin/crowdsec/whitelist`

**Request body**:
```json
{ "ip_or_cidr": "10.0.0.0/8", "reason": "Internal subnet" }
```

**Response 201**:
```json
{
  "uuid": "a1b2c3d4-...",
  "ip_or_cidr": "10.0.0.0/8",
  "reason": "Internal subnet",
  "created_at": "...",
  "updated_at": "..."
}
```

**Error responses**:
- `400` — missing/invalid `ip_or_cidr` field, unparseable IP/CIDR
- `409` — duplicate entry (same `ip_or_cidr` already exists)
- `500` — database or YAML write failure

#### `DELETE /admin/crowdsec/whitelist/:uuid`

**Response 204** — no body

**Error responses**:
- `404` — entry not found
- `500` — database or YAML write failure

### 3.3 Service Design

**New file**: `backend/internal/services/crowdsec_whitelist_service.go`

```go
package services

import (
    "context"
    "errors"
    "net"
    "os"
    "path/filepath"
    "text/template"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/yourusername/charon/backend/internal/models"
    "github.com/yourusername/charon/backend/internal/logger"
)

var (
    ErrWhitelistNotFound = errors.New("whitelist entry not found")
    ErrInvalidIPOrCIDR   = errors.New("invalid IP address or CIDR notation")
    ErrDuplicateEntry    = errors.New("whitelist entry already exists")
)

type CrowdSecWhitelistService struct {
    db      *gorm.DB
    dataDir string
}

func NewCrowdSecWhitelistService(db *gorm.DB, dataDir string) *CrowdSecWhitelistService {
    return &CrowdSecWhitelistService{db: db, dataDir: dataDir}
}

// List returns all whitelist entries ordered by creation time.
func (s *CrowdSecWhitelistService) List(ctx context.Context) ([]models.CrowdSecWhitelist, error) { ... }

// Add validates, persists, and regenerates the YAML file.
func (s *CrowdSecWhitelistService) Add(ctx context.Context, ipOrCIDR, reason string) (*models.CrowdSecWhitelist, error) { ... }

// Delete removes an entry by UUID and regenerates the YAML file.
func (s *CrowdSecWhitelistService) Delete(ctx context.Context, uuid string) error { ... }

// WriteYAML renders all current entries to <dataDir>/parsers/s02-enrich/charon-whitelist.yaml
func (s *CrowdSecWhitelistService) WriteYAML(ctx context.Context) error { ... }
```

**Validation logic** in `Add()`:
1. Trim whitespace from `ipOrCIDR`.
2. Attempt `net.ParseIP(ipOrCIDR)` — if non-nil, it's a bare IP ✓
3. Attempt `net.ParseCIDR(ipOrCIDR)` — if `err == nil`, it's a valid CIDR ✓; normalize host bits immediately: `ipOrCIDR = network.String()` (e.g., `"10.0.0.1/8"` → `"10.0.0.0/8"`).
4. If both fail → return `ErrInvalidIPOrCIDR`
5. Attempt DB insert; if GORM unique constraint error → return `ErrDuplicateEntry`
6. On success → call `WriteYAML(ctx)` (non-fatal on YAML error: log + return original entry)

> **Note**: `Add()` and `Delete()` do **not** call `cscli hub reload`. Reload is the caller's responsibility (handled in `CrowdsecHandler.AddWhitelist` and `DeleteWhitelist` via `h.CmdExec`).

**CIDR normalization snippet** (step 3):
```go
if ip, network, err := net.ParseCIDR(ipOrCIDR); err == nil {
    _ = ip
    ipOrCIDR = network.String() // normalizes "10.0.0.1/8" → "10.0.0.0/8"
}
```

**YAML generation** in `WriteYAML()`:

Guard: if `s.dataDir == ""`, return `nil` immediately (no-op — used in unit tests that don't need file I/O).

```go
const whitelistTmpl = `name: charon-whitelist
description: "Charon-managed IP/CIDR whitelist"
filter: "evt.Meta.service == 'http'"
whitelist:
  reason: "Charon managed whitelist"
  ip:
{{- range .IPs}}
    - "{{.}}"
{{- end}}
{{- if not .IPs}}
    []
{{- end}}
  cidr:
{{- range .CIDRs}}
    - "{{.}}"
{{- end}}
{{- if not .CIDRs}}
    []
{{- end}}
`
```

Target file path: `<dataDir>/config/parsers/s02-enrich/charon-whitelist.yaml`

Directory created with `os.MkdirAll(..., 0o750)` if absent.

File written atomically: render to `<path>.tmp` → `os.Rename(tmp, path)`.

### 3.4 Handler Design

**Additions to `CrowdsecHandler` struct**:
```go
type CrowdsecHandler struct {
    // ... existing fields ...
    WhitelistSvc *services.CrowdSecWhitelistService  // NEW
}
```

**`NewCrowdsecHandler` constructor** — initialize `WhitelistSvc`:
```go
h := &CrowdsecHandler{
    // ... existing assignments ...
}
if db != nil {
    h.WhitelistSvc = services.NewCrowdSecWhitelistService(db, dataDir)
}
return h
```

**Three new methods on `CrowdsecHandler`**:

```go
// ListWhitelists handles GET /admin/crowdsec/whitelist
func (h *CrowdsecHandler) ListWhitelists(c *gin.Context) {
    entries, err := h.WhitelistSvc.List(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list whitelist entries"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"whitelist": entries})
}

// AddWhitelist handles POST /admin/crowdsec/whitelist
func (h *CrowdsecHandler) AddWhitelist(c *gin.Context) {
    var req struct {
        IPOrCIDR string `json:"ip_or_cidr" binding:"required"`
        Reason   string `json:"reason"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ip_or_cidr is required"})
        return
    }
    entry, err := h.WhitelistSvc.Add(c.Request.Context(), req.IPOrCIDR, req.Reason)
    if errors.Is(err, services.ErrInvalidIPOrCIDR) {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if errors.Is(err, services.ErrDuplicateEntry) {
        c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
        return
    }
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add whitelist entry"})
        return
    }
    // Reload CrowdSec so the new entry takes effect immediately (non-fatal).
    if reloadErr := h.CmdExec.Execute("cscli", "hub", "reload"); reloadErr != nil {
        logger.Log().WithError(reloadErr).Warn("failed to reload CrowdSec after whitelist add (non-fatal)")
    }
    c.JSON(http.StatusCreated, entry)
}

// DeleteWhitelist handles DELETE /admin/crowdsec/whitelist/:uuid
func (h *CrowdsecHandler) DeleteWhitelist(c *gin.Context) {
    id := strings.TrimSpace(c.Param("uuid"))
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "uuid required"})
        return
    }
    err := h.WhitelistSvc.Delete(c.Request.Context(), id)
    if errors.Is(err, services.ErrWhitelistNotFound) {
        c.JSON(http.StatusNotFound, gin.H{"error": "whitelist entry not found"})
        return
    }
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete whitelist entry"})
        return
    }
    // Reload CrowdSec so the removed entry is no longer exempt (non-fatal).
    if reloadErr := h.CmdExec.Execute("cscli", "hub", "reload"); reloadErr != nil {
        logger.Log().WithError(reloadErr).Warn("failed to reload CrowdSec after whitelist delete (non-fatal)")
    }
    c.Status(http.StatusNoContent)
}
```

**Route registration** (append inside `RegisterRoutes`, after existing decision/bouncer routes):
```go
// Whitelist management
rg.GET("/admin/crowdsec/whitelist", h.ListWhitelists)
rg.POST("/admin/crowdsec/whitelist", h.AddWhitelist)
rg.DELETE("/admin/crowdsec/whitelist/:uuid", h.DeleteWhitelist)
```

### 3.5 Startup Integration

**File**: `backend/internal/services/crowdsec_startup.go`

In `ReconcileCrowdSecOnStartup()`, before the CrowdSec process is started:

```go
// Regenerate whitelist YAML to ensure it reflects the current DB state.
whitelistSvc := NewCrowdSecWhitelistService(db, dataDir)
if err := whitelistSvc.WriteYAML(ctx); err != nil {
    logger.Log().WithError(err).Warn("failed to write CrowdSec whitelist YAML on startup (non-fatal)")
}
```

This is **non-fatal**: if the DB has no entries, WriteYAML still writes an empty whitelist file, which is valid.

### 3.6 Hub Parser Installation

**File**: `configs/crowdsec/install_hub_items.sh`

Add after the existing `cscli parsers install` lines:

```bash
cscli parsers install crowdsecurity/whitelists --force || echo "⚠️ Failed to install crowdsecurity/whitelists"
```

### 3.7 Frontend Design

#### API Client (`frontend/src/api/crowdsec.ts`)

Append the following types and functions:

```typescript
export interface CrowdSecWhitelistEntry {
  uuid: string
  ip_or_cidr: string
  reason: string
  created_at: string
  updated_at: string
}

export interface AddWhitelistPayload {
  ip_or_cidr: string
  reason: string
}

export const listWhitelists = async (): Promise<CrowdSecWhitelistEntry[]> => {
  const resp = await client.get<{ whitelist: CrowdSecWhitelistEntry[] }>('/admin/crowdsec/whitelist')
  return resp.data.whitelist
}

export const addWhitelist = async (data: AddWhitelistPayload): Promise<CrowdSecWhitelistEntry> => {
  const resp = await client.post<CrowdSecWhitelistEntry>('/admin/crowdsec/whitelist', data)
  return resp.data
}

export const deleteWhitelist = async (uuid: string): Promise<void> => {
  await client.delete(`/admin/crowdsec/whitelist/${uuid}`)
}
```

#### TanStack Query Hooks (`frontend/src/hooks/useCrowdSecWhitelist.ts`)

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listWhitelists, addWhitelist, deleteWhitelist, AddWhitelistPayload } from '../api/crowdsec'
import { toast } from 'sonner'

export const useWhitelistEntries = () =>
  useQuery({
    queryKey: ['crowdsec-whitelist'],
    queryFn: listWhitelists,
  })

export const useAddWhitelist = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: AddWhitelistPayload) => addWhitelist(data),
    onSuccess: () => {
      toast.success('Whitelist entry added')
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to add whitelist entry')
    },
  })
}

export const useDeleteWhitelist = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (uuid: string) => deleteWhitelist(uuid),
    onSuccess: () => {
      toast.success('Whitelist entry removed')
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to remove whitelist entry')
    },
  })
}
```

#### CrowdSecConfig.tsx Changes

The `CrowdSecConfig.tsx` page uses a tab navigation pattern. The new "Whitelist" tab:

1. **Visibility**: Only render the tab when `isLocalMode === true` (same guard as Decisions tab).
2. **Tab value**: `"whitelist"` — append to the existing tab list.
3. **Tab panel content** (isolated component or inline JSX):
   - **Add entry form**: `ip_or_cidr` text input + `reason` text input + "Add" button (disabled while `addMutation.isPending`). Validation error shown inline when backend returns 400/409.
   - **Quick-add current IP**: A secondary "Add My IP" button that calls `GET /api/v1/system/my-ip` (existing endpoint) and pre-fills the `ip_or_cidr` field with the returned IP.
   - **Entries table**: Columns — IP/CIDR, Reason, Added, Actions. Each row has a delete button with a confirmation dialog (matching the ban/unban modal pattern used for Decisions).
   - **Empty state**: "No whitelist entries" message when the list is empty.
   - **Loading state**: Skeleton rows while `useWhitelistEntries` is fetching.

**Imports added to `CrowdSecConfig.tsx`**:
```typescript
import { useWhitelistEntries, useAddWhitelist, useDeleteWhitelist } from '../hooks/useCrowdSecWhitelist'
```

### 3.8 Data Flow Diagram

```
Operator adds IP in UI
        │
        ▼
POST /api/v1/admin/crowdsec/whitelist
        │
        ▼
CrowdsecHandler.AddWhitelist()
        │
        ▼
CrowdSecWhitelistService.Add()
   ├── Validate IP/CIDR (net.ParseIP / net.ParseCIDR)
   ├── Normalize CIDR host bits (network.String())
   ├── Insert into SQLite (models.CrowdSecWhitelist)
   └── WriteYAML() → <dataDir>/config/parsers/s02-enrich/charon-whitelist.yaml
        │
        ▼
h.CmdExec.Execute("cscli", "hub", "reload") [non-fatal on error]
        │
        ▼
Return 201 to frontend
        │
        ▼
invalidateQueries(['crowdsec-whitelist'])
        │
        ▼
Table re-fetches and shows new entry
```

```
Container restart
        │
        ▼
ReconcileCrowdSecOnStartup()
        │
        ▼
CrowdSecWhitelistService.WriteYAML()
   └── Reads all DB entries → renders YAML
        │
        ▼
CrowdSec process starts
        │
        ▼
CrowdSec loads parsers/s02-enrich/charon-whitelist.yaml
   └── crowdsecurity/whitelists parser activates
        │
        ▼
IPs/CIDRs in file are exempt from all ban decisions
```

### 3.9 Error Handling Matrix

| Scenario | Service Error | HTTP Status | Frontend Behavior |
|---|---|---|---|
| Blank `ip_or_cidr` | — | 400 | Inline validation (required field) |
| Malformed IP/CIDR | `ErrInvalidIPOrCIDR` | 400 | Toast: "Invalid IP address or CIDR notation" |
| Duplicate entry | `ErrDuplicateEntry` | 409 | Toast: "This IP/CIDR is already whitelisted" |
| DB unavailable | generic error | 500 | Toast: "Failed to add whitelist entry" |
| UUID not found on DELETE | `ErrWhitelistNotFound` | 404 | Toast: "Whitelist entry not found" |
| YAML write failure | logged, non-fatal | 201 (Add still succeeds) | No user-facing error; log warning |
| CrowdSec reload failure | logged, non-fatal | 201/204 (operation still succeeds) | No user-facing error; log warning |

### 3.10 Security Considerations

- **Input validation**: All `ip_or_cidr` values are validated server-side with `net.ParseIP` / `net.ParseCIDR` before persisting. Arbitrary strings are rejected.
- **Path traversal**: `WriteYAML` constructs the output path via `filepath.Join(s.dataDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml")`. `dataDir` is set at startup—not user-supplied at request time.
- **Privilege**: All three endpoints require management-level access (same as all other CrowdSec endpoints).
- **YAML injection**: Values are rendered through Go's `text/template` with explicit quoting of each entry; no raw string concatenation.
- **Log safety**: IPs are logged using the same structured field pattern used in existing CrowdSec handler methods (e.g., `logger.Log().WithField("ip", entry.IPOrCIDR).Info(...)`).

---

## 4. Implementation Plan

### Phase 1 — Hub Parser Installation (Groundwork)

**Files Changed**:
- `configs/crowdsec/install_hub_items.sh`

**Task 1.1**: Add `cscli parsers install crowdsecurity/whitelists --force` after the last parser install line (currently `crowdsecurity/syslog-logs`).

**Acceptance**: File change is syntactically valid bash; `shellcheck` passes.

---

### Phase 2 — Database Model

**Files Changed**:
- `backend/internal/models/crowdsec_whitelist.go` _(new file)_
- `backend/internal/api/routes/routes.go` _(append to AutoMigrate call)_

**Task 2.1**: Create `crowdsec_whitelist.go` with the `CrowdSecWhitelist` struct per §3.1.

**Task 2.2**: Append `&models.CrowdSecWhitelist{}` to the `db.AutoMigrate(...)` call in `routes.go`.

**Validation Gate**: `go build ./backend/...` passes; GORM generates `crowdsec_whitelists` table on next startup.

---

### Phase 3 — Whitelist Service

**Files Changed**:
- `backend/internal/services/crowdsec_whitelist_service.go` _(new file)_

**Task 3.1**: Implement `CrowdSecWhitelistService` with `List`, `Add`, `Delete`, `WriteYAML` per §3.3.

**Task 3.2**: Implement IP/CIDR validation in `Add()`:
- `net.ParseIP(ipOrCIDR) != nil` → valid bare IP
- `net.ParseCIDR(ipOrCIDR)` returns no error → valid CIDR
- Both fail → `ErrInvalidIPOrCIDR`

**Task 3.3**: Implement `WriteYAML()`:
- Query all entries from DB.
- Partition into `ips` (bare IPs) and `cidrs` (CIDR notation) slices.
- Render template per §2.4.
- Atomic write: temp file → `os.Rename`.
- Create directory (`os.MkdirAll`) if not present.

**Validation Gate**: `go test ./backend/internal/services/... -run TestCrowdSecWhitelist` passes.

---

### Phase 4 — API Endpoints

**Files Changed**:
- `backend/internal/api/handlers/crowdsec_handler.go`

**Task 4.1**: Add `WhitelistSvc *services.CrowdSecWhitelistService` field to `CrowdsecHandler` struct.

**Task 4.2**: Initialize `WhitelistSvc` in `NewCrowdsecHandler()` when `db != nil`.

**Task 4.3**: Implement `ListWhitelists`, `AddWhitelist`, `DeleteWhitelist` methods per §3.4.

**Task 4.4**: Register three routes in `RegisterRoutes()` per §3.4.

**Task 4.5**: In `AddWhitelist` and `DeleteWhitelist`, after the service call returns without error, call `h.CmdExec.Execute("cscli", "hub", "reload")`. Log a warning on failure; do not change the HTTP response status (reload failure is non-fatal).

**Validation Gate**: `go test ./backend/internal/api/handlers/... -run TestWhitelist` passes; `make lint-fast` clean.

---

### Phase 5 — Startup Integration

**Files Changed**:
- `backend/internal/services/crowdsec_startup.go`

**Task 5.1**: In `ReconcileCrowdSecOnStartup()`, after the DB and config are loaded but before calling `h.Executor.Start()`, instantiate `CrowdSecWhitelistService` and call `WriteYAML(ctx)`. Log warning on error; do not abort startup.

**Validation Gate**: `go test ./backend/internal/services/... -run TestReconcile` passes; existing reconcile tests still pass.

---

### Phase 6 — Frontend API + Hooks

**Files Changed**:
- `frontend/src/api/crowdsec.ts`
- `frontend/src/hooks/useCrowdSecWhitelist.ts` _(new file)_

**Task 6.1**: Add `CrowdSecWhitelistEntry`, `AddWhitelistPayload` types and `listWhitelists`, `addWhitelist`, `deleteWhitelist` functions to `crowdsec.ts` per §3.7.

**Task 6.2**: Create `useCrowdSecWhitelist.ts` with `useWhitelistEntries`, `useAddWhitelist`, `useDeleteWhitelist` hooks per §3.7.

**Validation Gate**: `pnpm test` (Vitest) passes; TypeScript compilation clean.

---

### Phase 7 — Frontend UI

**Files Changed**:
- `frontend/src/pages/CrowdSecConfig.tsx`

**Task 7.1**: Import the three hooks from `useCrowdSecWhitelist.ts`.

**Task 7.2**: Add `"whitelist"` to the tab list (visible only when `isLocalMode === true`).

**Task 7.3**: Implement the Whitelist tab panel:
- Add-entry form with IP/CIDR + Reason inputs.
- "Add My IP" button: `GET /api/v1/system/my-ip` → pre-fill `ip_or_cidr`.
- Entries table with UUID key, IP/CIDR, Reason, created date, delete button.
- Delete confirmation dialog (reuse existing modal pattern).

**Task 7.4**: Wire mutation errors to inline form validation messages (400/409 responses).

**Validation Gate**: `pnpm test` passes; TypeScript clean; `make lint-fast` clean.

---

### Phase 8 — Tests

**Files Changed**:
- `backend/internal/services/crowdsec_whitelist_service_test.go` _(new file)_
- `backend/internal/api/handlers/crowdsec_whitelist_handler_test.go` _(new file)_
- `tests/crowdsec-whitelist.spec.ts` _(new file)_

**Task 8.1 — Service unit tests**:

| Test | Scenario |
|---|---|
| `TestAdd_ValidIP_Success` | Bare IPv4 inserted; YAML file created |
| `TestAdd_ValidIPv6_Success` | Bare IPv6 inserted |
| `TestAdd_ValidCIDR_Success` | CIDR range inserted |
| `TestAdd_CIDRNormalization` | `"10.0.0.1/8"` stored as `"10.0.0.0/8"` |
| `TestAdd_InvalidIPOrCIDR_Error` | Returns `ErrInvalidIPOrCIDR` |
| `TestAdd_DuplicateEntry_Error` | Second identical insert returns `ErrDuplicateEntry` |
| `TestDelete_Success` | Entry removed; YAML regenerated |
| `TestDelete_NotFound_Error` | Returns `ErrWhitelistNotFound` |
| `TestList_Empty` | Returns empty slice |
| `TestList_Populated` | Returns all entries ordered by `created_at` |
| `TestWriteYAML_EmptyList` | Writes valid YAML with empty `ip: []` and `cidr: []` |
| `TestWriteYAML_MixedEntries` | IPs in `ip:` block; CIDRs in `cidr:` block |
| `TestWriteYAML_EmptyDataDir_NoOp` | `dataDir == ""` → returns `nil`, no file written |

**Task 8.2 — Handler unit tests** (using in-memory SQLite + `mockAuthMiddleware`):

| Test | Scenario |
|---|---|
| `TestListWhitelists_200` | Returns 200 with entries array |
| `TestAddWhitelist_201` | Valid payload → 201 |
| `TestAddWhitelist_400_MissingField` | Empty body → 400 |
| `TestAddWhitelist_400_InvalidIP` | Malformed IP → 400 |
| `TestAddWhitelist_409_Duplicate` | Duplicate → 409 |
| `TestDeleteWhitelist_204` | Valid UUID → 204 |
| `TestDeleteWhitelist_404` | Unknown UUID → 404 |

**Task 8.3 — E2E Playwright tests** (`tests/crowdsec-whitelist.spec.ts`):

```typescript
import { test, expect } from '@playwright/test'

test.describe('CrowdSec Whitelist Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:8080')
    await page.getByRole('link', { name: 'Security' }).click()
    await page.getByRole('tab', { name: 'CrowdSec' }).click()
    await page.getByRole('tab', { name: 'Whitelist' }).click()
  })

  test('Whitelist tab only visible in local mode', async ({ page }) => {
    await page.goto('http://localhost:8080')
    await page.getByRole('link', { name: 'Security' }).click()
    await page.getByRole('tab', { name: 'CrowdSec' }).click()
    // When CrowdSec is not in local mode, the Whitelist tab must not exist
    await expect(page.getByRole('tab', { name: 'Whitelist' })).toBeHidden()
  })

  test('displays empty state when no entries exist', async ({ page }) => {
    await expect(page.getByText('No whitelist entries')).toBeVisible()
  })

  test('adds a valid IP address', async ({ page }) => {
    await page.getByRole('textbox', { name: 'IP or CIDR' }).fill('203.0.113.5')
    await page.getByRole('textbox', { name: 'Reason' }).fill('Uptime monitor')
    await page.getByRole('button', { name: 'Add' }).click()
    await expect(page.getByText('Whitelist entry added')).toBeVisible()
    await expect(page.getByRole('cell', { name: '203.0.113.5' })).toBeVisible()
  })

  test('adds a valid CIDR range', async ({ page }) => {
    await page.getByRole('textbox', { name: 'IP or CIDR' }).fill('10.0.0.0/8')
    await page.getByRole('textbox', { name: 'Reason' }).fill('Internal subnet')
    await page.getByRole('button', { name: 'Add' }).click()
    await expect(page.getByText('Whitelist entry added')).toBeVisible()
    await expect(page.getByRole('cell', { name: '10.0.0.0/8' })).toBeVisible()
  })

  test('"Add My IP" button pre-fills the detected client IP', async ({ page }) => {
    await page.getByRole('button', { name: 'Add My IP' }).click()
    const ipField = page.getByRole('textbox', { name: 'IP or CIDR' })
    const value = await ipField.inputValue()
    // Value must be a non-empty valid IP
    expect(value).toMatch(/^[\d.]+$|^[0-9a-fA-F:]+$/)
  })

  test('shows validation error for invalid input', async ({ page }) => {
    await page.getByRole('textbox', { name: 'IP or CIDR' }).fill('not-an-ip')
    await page.getByRole('button', { name: 'Add' }).click()
    await expect(page.getByText('Invalid IP address or CIDR notation')).toBeVisible()
  })

  test('removes an entry via delete confirmation', async ({ page }) => {
    // Seed an entry first
    await page.getByRole('textbox', { name: 'IP or CIDR' }).fill('198.51.100.1')
    await page.getByRole('button', { name: 'Add' }).click()
    await expect(page.getByRole('cell', { name: '198.51.100.1' })).toBeVisible()

    // Delete it
    await page.getByRole('row', { name: /198\.51\.100\.1/ }).getByRole('button', { name: 'Delete' }).click()
    await page.getByRole('button', { name: 'Confirm' }).click()
    await expect(page.getByText('Whitelist entry removed')).toBeVisible()
    await expect(page.getByRole('cell', { name: '198.51.100.1' })).toBeHidden()
  })
})
```

---

### Phase 9 — Documentation

**Files Changed**:
- `ARCHITECTURE.md`
- `docs/features/crowdsec-whitelist.md` _(new file, optional for this PR)_

**Task 9.1**: Update the CrowdSec row in the Cerberus security components table in `ARCHITECTURE.md` to mention whitelist management.

---

## 5. Acceptance Criteria

### Functional

- [ ] Operator can add a bare IPv4 address (e.g., `203.0.113.5`) to the whitelist.
- [ ] Operator can add a bare IPv6 address (e.g., `2001:db8::1`) to the whitelist.
- [ ] Operator can add a CIDR range (e.g., `10.0.0.0/8`) to the whitelist.
- [ ] Adding an invalid IP/CIDR (e.g., `not-an-ip`) returns a 400 error with a clear message.
- [ ] Adding a duplicate entry returns a 409 conflict error.
- [ ] Operator can delete an entry; it disappears from the list.
- [ ] The Whitelist tab is only visible when CrowdSec is in `local` mode.
- [ ] After adding or deleting an entry, the whitelist YAML file is regenerated in `<dataDir>/config/parsers/s02-enrich/charon-whitelist.yaml`.
- [ ] Adding or removing a whitelist entry triggers `cscli hub reload` via `h.CmdExec` so changes take effect immediately without a container restart.
- [ ] On container restart, the YAML file is regenerated from DB entries before CrowdSec starts.
- [ ] **Admin IP protection**: The "Add My IP" button pre-fills the operator's current IP in the `ip_or_cidr` field; a Playwright E2E test verifies the button correctly pre-fills the detected client IP.

### Technical

- [ ] `go test ./backend/...` passes — no regressions.
- [ ] `pnpm test` (Vitest) passes.
- [ ] `make lint-fast` clean — no new lint findings.
- [ ] GORM Security Scanner returns zero CRITICAL/HIGH findings.
- [ ] Playwright E2E suite passes (Firefox, `--project=firefox`).
- [ ] `crowdsecurity/whitelists` parser is installed by `install_hub_items.sh`.

---

## 6. Commit Slicing Strategy

**Decision**: Single PR with ordered logical commits. No scope overlap between commits; each commit leaves the codebase in a compilable state.

**Trigger reasons**: Cross-domain change (infra script + model + service + handler + startup + frontend) benefits from ordered commits for surgical rollback and focused review.

| # | Type | Commit Message | Files | Depends On | Validation Gate |
|---|---|---|---|---|---|
| 1 | `chore` | `install crowdsecurity/whitelists parser by default` | `configs/crowdsec/install_hub_items.sh` | — | `shellcheck` |
| 2 | `feat` | `add CrowdSecWhitelist model and automigrate registration` | `backend/internal/models/crowdsec_whitelist.go`, `backend/internal/api/routes/routes.go` | #1 | `go build ./backend/...` |
| 3 | `feat` | `add CrowdSecWhitelistService with YAML generation` | `backend/internal/services/crowdsec_whitelist_service.go` | #2 | `go test ./backend/internal/services/...` |
| 4 | `feat` | `add whitelist API endpoints to CrowdsecHandler` | `backend/internal/api/handlers/crowdsec_handler.go` | #3 | `go test ./backend/...` + `make lint-fast` |
| 5 | `feat` | `regenerate whitelist YAML on CrowdSec startup reconcile` | `backend/internal/services/crowdsec_startup.go` | #3 | `go test ./backend/internal/services/...` |
| 6 | `feat` | `add whitelist API client functions and TanStack hooks` | `frontend/src/api/crowdsec.ts`, `frontend/src/hooks/useCrowdSecWhitelist.ts` | #4 | `pnpm test` |
| 7 | `feat` | `add Whitelist tab to CrowdSecConfig UI` | `frontend/src/pages/CrowdSecConfig.tsx` | #6 | `pnpm test` + `make lint-fast` |
| 8 | `test` | `add whitelist service and handler unit tests` | `*_test.go` files | #4 | `go test ./backend/...` |
| 9 | `test` | `add E2E tests for CrowdSec whitelist management` | `tests/crowdsec-whitelist.spec.ts` | #7 | Playwright Firefox |
| 10 | `docs` | `update architecture docs for CrowdSec whitelist feature` | `ARCHITECTURE.md` | #7 | `make lint-fast` |

**Rollback notes**:
- Commits 1–3 are pure additions (no existing code modified except the `AutoMigrate` list append in commit 2 and `install_hub_items.sh` in commit 1). Reverting them is safe.
- Commit 4 modifies `crowdsec_handler.go` by adding fields and methods without altering existing ones; reverting is mechanical.
- Commit 5 modifies `crowdsec_startup.go` — the added block is isolated in a clearly marked section; revert is a 5-line removal.
- Commits 6–7 are frontend-only; reverting has no backend impact.

---

## 7. Open Questions / Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| CrowdSec does not hot-reload parser files — requires `cscli reload` or process restart | Resolved | `cscli hub reload` is called via `h.CmdExec.Execute(...)` in `AddWhitelist` and `DeleteWhitelist` after each successful `WriteYAML()`. Failure is non-fatal; logged as a warning. |
| `crowdsecurity/whitelists` parser path may differ across CrowdSec versions | Low | Use `<dataDir>/config/parsers/s02-enrich/` which is the canonical path; add a note to verify on version upgrades |
| Large whitelist files could cause CrowdSec performance issues | Very Low | Reasonable for typical use; document a soft limit recommendation (< 500 entries) in the UI |
| `dataDir` empty string in tests | Resolved | Guard added to `WriteYAML`: `if s.dataDir == "" { return nil }` — no-op when `dataDir` is unset |
| `CROWDSEC_TRUSTED_IPS` env var seeding | — | **Follow-up / future enhancement** (not in scope for this PR): if `CROWDSEC_TRUSTED_IPS` is set at runtime, parse comma-separated IPs and include them as read-only seed entries in the generated YAML (separate from DB-managed entries). Document in a follow-up issue. |

