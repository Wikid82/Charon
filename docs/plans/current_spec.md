# Access List Handler: UUID Support Plan

**Version:** 1.0
**Created:** January 29, 2026
**Status:** Ready for Implementation

---

## Problem Statement

The Access List model uses a security design that hides the numeric ID from JSON responses:

```go
// backend/internal/models/access_list.go:9-10
ID   uint   `json:"-" gorm:"primaryKey"`    // HIDDEN from JSON
UUID string `json:"uuid" gorm:"uniqueIndex"` // Exposed as "uuid"
```

However, all four handler endpoints (`Get`, `Update`, `Delete`, `TestIP`) only accept numeric IDs:

```go
// backend/internal/api/handlers/access_list_handler.go:58, 79, 107, 131
id, err := strconv.ParseUint(c.Param("id"), 10, 32)
```

This causes E2E test failures because the JSON response returns `uuid` but tests attempt to use `id`:

1. **Line 138**: `lists[0].id` is `undefined` because `id` is hidden from JSON
2. **Line 162**: `createdList.id` is `undefined` for the same reason

---

## Requirements (EARS Notation)

### R1: UUID Path Parameter Support
WHEN a request is made to `/api/v1/access-lists/:id` endpoints,
THE SYSTEM SHALL accept either a numeric ID or a UUID string in the `:id` path parameter.

### R2: Backward Compatibility
WHEN a numeric ID is provided in the `:id` path parameter,
THE SYSTEM SHALL resolve it to an access list using the existing `GetByID()` method.

### R3: UUID Resolution
WHEN a non-numeric string is provided in the `:id` path parameter,
THE SYSTEM SHALL attempt to resolve it as a UUID using the existing `GetByUUID()` method.

### R4: Error Handling
IF neither numeric ID nor UUID matches an access list,
THEN THE SYSTEM SHALL return HTTP 404 with error message "access list not found".

### R5: Invalid Identifier
IF the `:id` parameter is empty or cannot be parsed as either uint or valid UUID format,
THE SYSTEM SHALL return HTTP 400 with error message "invalid ID or UUID".

---

## Technical Design

### Approach: Helper Function in Handler

Create a helper function `resolveAccessList()` that:
1. First attempts to parse as uint (numeric ID) for backward compatibility
2. If parsing fails, treats it as UUID and calls `GetByUUID()`
3. Returns the resolved `*models.AccessList` or appropriate error

This approach:
- Minimizes code duplication across 4 handlers
- Preserves backward compatibility with numeric IDs
- Leverages existing `GetByUUID()` service method
- Requires no service layer changes

### Implementation Pattern

```go
// resolveAccessList resolves an access list by either numeric ID or UUID.
// It first attempts to parse as uint (backward compatibility), then tries UUID.
func (h *AccessListHandler) resolveAccessList(idOrUUID string) (*models.AccessList, error) {
    // Try parsing as numeric ID first (backward compatibility)
    if id, err := strconv.ParseUint(idOrUUID, 10, 32); err == nil {
        return h.service.GetByID(uint(id))
    }

    // Try as UUID
    return h.service.GetByUUID(idOrUUID)
}
```

---

## Implementation Tasks

### Phase 1: Handler Modifications (Critical)

#### Task 1.1: Add resolveAccessList Helper
**File:** [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go)
**Location:** After line 27 (after `SetGeoIPService` method)
**Priority:** Critical

```go
// resolveAccessList resolves an access list by either numeric ID or UUID.
// It first attempts to parse as uint (backward compatibility), then tries UUID.
func (h *AccessListHandler) resolveAccessList(idOrUUID string) (*models.AccessList, error) {
    // Try parsing as numeric ID first (backward compatibility)
    if id, err := strconv.ParseUint(idOrUUID, 10, 32); err == nil {
        return h.service.GetByID(uint(id))
    }

    // Empty string check
    if idOrUUID == "" {
        return nil, fmt.Errorf("invalid ID or UUID")
    }

    // Try as UUID
    return h.service.GetByUUID(idOrUUID)
}
```

**Import Required:** Add `"fmt"` to imports (line 4).

#### Task 1.2: Update Get Handler
**File:** [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go)
**Location:** Lines 54-72 (Get method)

**Current Code:**
```go
func (h *AccessListHandler) Get(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
        return
    }

    acl, err := h.service.GetByID(uint(id))
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, acl)
}
```

**New Code:**
```go
func (h *AccessListHandler) Get(c *gin.Context) {
    acl, err := h.resolveAccessList(c.Param("id"))
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID or UUID"})
        return
    }

    c.JSON(http.StatusOK, acl)
}
```

#### Task 1.3: Update Update Handler
**File:** [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go)
**Location:** Lines 75-101 (Update method)

**Current Code:**
```go
func (h *AccessListHandler) Update(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
        return
    }

    var updates models.AccessList
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := h.service.Update(uint(id), &updates); err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Fetch updated record
    acl, _ := h.service.GetByID(uint(id))
    c.JSON(http.StatusOK, acl)
}
```

**New Code:**
```go
func (h *AccessListHandler) Update(c *gin.Context) {
    // Resolve access list first to get the internal ID
    acl, err := h.resolveAccessList(c.Param("id"))
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID or UUID"})
        return
    }

    var updates models.AccessList
    if err := c.ShouldBindJSON(&updates); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := h.service.Update(acl.ID, &updates); err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Fetch updated record
    updatedAcl, _ := h.service.GetByID(acl.ID)
    c.JSON(http.StatusOK, updatedAcl)
}
```

#### Task 1.4: Update Delete Handler
**File:** [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go)
**Location:** Lines 104-125 (Delete method)

**Current Code:**
```go
func (h *AccessListHandler) Delete(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
        return
    }

    if err := h.service.Delete(uint(id)); err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        if err == services.ErrAccessListInUse {
            c.JSON(http.StatusConflict, gin.H{"error": "access list is in use by proxy hosts"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "access list deleted"})
}
```

**New Code:**
```go
func (h *AccessListHandler) Delete(c *gin.Context) {
    // Resolve access list first to get the internal ID
    acl, err := h.resolveAccessList(c.Param("id"))
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID or UUID"})
        return
    }

    if err := h.service.Delete(acl.ID); err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        if err == services.ErrAccessListInUse {
            c.JSON(http.StatusConflict, gin.H{"error": "access list is in use by proxy hosts"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "access list deleted"})
}
```

#### Task 1.5: Update TestIP Handler
**File:** [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go)
**Location:** Lines 128-157 (TestIP method)

**Current Code:**
```go
func (h *AccessListHandler) TestIP(c *gin.Context) {
    id, err := strconv.ParseUint(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
        return
    }

    var req struct {
        IPAddress string `json:"ip_address" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    allowed, reason, err := h.service.TestIP(uint(id), req.IPAddress)
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        if err == services.ErrInvalidIPAddress {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "allowed": allowed,
        "reason":  reason,
    })
}
```

**New Code:**
```go
func (h *AccessListHandler) TestIP(c *gin.Context) {
    // Resolve access list first to get the internal ID
    acl, err := h.resolveAccessList(c.Param("id"))
    if err != nil {
        if err == services.ErrAccessListNotFound {
            c.JSON(http.StatusNotFound, gin.H{"error": "access list not found"})
            return
        }
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID or UUID"})
        return
    }

    var req struct {
        IPAddress string `json:"ip_address" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    allowed, reason, err := h.service.TestIP(acl.ID, req.IPAddress)
    if err != nil {
        if err == services.ErrInvalidIPAddress {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "allowed": allowed,
        "reason":  reason,
    })
}
```

---

### Phase 2: Unit Test Updates (High)

#### Task 2.1: Add UUID Test Cases
**File:** [access_list_handler_test.go](../../backend/internal/api/handlers/access_list_handler_test.go)

Add test cases for UUID-based lookups to existing test functions.

**TestAccessListHandler_Get** (add to tests slice at line 166):
```go
{
    name:       "get by UUID",
    id:         "test-uuid",
    wantStatus: http.StatusOK,
},
{
    name:       "get by invalid format",
    id:         "",
    wantStatus: http.StatusBadRequest,
},
```

**TestAccessListHandler_Update** (add to tests slice at line 216):
```go
{
    name: "update by UUID successfully",
    id:   "test-uuid",
    payload: map[string]any{
        "name":     "Updated via UUID",
        "type":     "whitelist",
        "ip_rules": `[]`,
    },
    wantStatus: http.StatusOK,
},
```

**TestAccessListHandler_Delete** - Create new ACL with known UUID for delete-by-UUID test.

**TestAccessListHandler_TestIP** (add to tests slice at line 359):
```go
{
    name:       "test IP by UUID",
    id:         "test-uuid",
    payload:    map[string]string{"ip_address": "192.168.1.100"},
    wantStatus: http.StatusOK,
},
```

#### Task 2.2: Add Dedicated resolveAccessList Tests
**File:** [access_list_handler_test.go](../../backend/internal/api/handlers/access_list_handler_test.go)
**Location:** After `TestAccessListHandler_GetTemplates` (after line 410)

```go
func TestAccessListHandler_resolveAccessList(t *testing.T) {
    router, db := setupAccessListTestRouter(t)
    _ = router // Needed for handler creation

    handler := NewAccessListHandler(db)

    // Create test ACL with known UUID
    acl := models.AccessList{
        UUID:    "resolve-test-uuid",
        Name:    "Resolve Test ACL",
        Type:    "whitelist",
        Enabled: true,
    }
    db.Create(&acl)

    tests := []struct {
        name      string
        idOrUUID  string
        wantErr   bool
        wantName  string
    }{
        {
            name:     "resolve by numeric ID",
            idOrUUID: "1",
            wantErr:  false,
            wantName: "Resolve Test ACL",
        },
        {
            name:     "resolve by UUID",
            idOrUUID: "resolve-test-uuid",
            wantErr:  false,
            wantName: "Resolve Test ACL",
        },
        {
            name:     "fail with non-existent numeric ID",
            idOrUUID: "9999",
            wantErr:  true,
        },
        {
            name:     "fail with non-existent UUID",
            idOrUUID: "non-existent-uuid",
            wantErr:  true,
        },
        {
            name:     "fail with empty string",
            idOrUUID: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := handler.resolveAccessList(tt.idOrUUID)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, result)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
                assert.Equal(t, tt.wantName, result.Name)
            }
        })
    }
}
```

---

### Phase 3: E2E Test Fixes (High)

#### Task 3.1: Update acl-enforcement.spec.ts
**File:** [acl-enforcement.spec.ts](../../tests/security-enforcement/acl-enforcement.spec.ts)

**Line 138** - Change from `lists[0].id` to `lists[0].uuid`:
```typescript
// Before:
const testResponse = await requestContext.post(
  `/api/v1/access-lists/${lists[0].id}/test`,
  { data: { ip_address: testIp } }
);

// After:
const testResponse = await requestContext.post(
  `/api/v1/access-lists/${lists[0].uuid}/test`,
  { data: { ip_address: testIp } }
);
```

**Line 162 and beyond** - Change from `createdList.id` to `createdList.uuid`:
```typescript
// Before:
expect(createdList.id).toBeDefined();
// ...
const testResponse = await requestContext.post(
  `/api/v1/access-lists/${createdList.id}/test`,
  { data: { ip_address: '10.255.255.255' } }
);
// ...
const deleteResponse = await requestContext.delete(
  `/api/v1/access-lists/${createdList.id}`
);

// After:
expect(createdList.uuid).toBeDefined();
// ...
const testResponse = await requestContext.post(
  `/api/v1/access-lists/${createdList.uuid}/test`,
  { data: { ip_address: '10.255.255.255' } }
);
// ...
const deleteResponse = await requestContext.delete(
  `/api/v1/access-lists/${createdList.uuid}`
);
```

---

## Validation Checklist

### Unit Tests
- [ ] All existing tests pass with numeric IDs (backward compatibility)
- [ ] New UUID-based test cases pass
- [ ] `resolveAccessList` helper tests pass
- [ ] Coverage remains above 85%

### E2E Tests
- [ ] `should test IP against access list` passes (line 138)
- [ ] `should show correct error response format` passes (line 162)
- [ ] All other ACL enforcement tests remain green

### Manual Verification
```bash
# Create access list
curl -X POST http://localhost:8080/api/v1/access-lists \
  -H "Content-Type: application/json" \
  -d '{"name":"Test ACL","type":"whitelist","enabled":true}'
# Response: {"uuid":"abc-123-def", "name":"Test ACL", ...}

# Get by UUID (new)
curl http://localhost:8080/api/v1/access-lists/abc-123-def
# Should return the access list

# Get by numeric ID (backward compat - requires direct DB access)
curl http://localhost:8080/api/v1/access-lists/1
# Should return the access list

# Test IP by UUID
curl -X POST http://localhost:8080/api/v1/access-lists/abc-123-def/test \
  -H "Content-Type: application/json" \
  -d '{"ip_address":"192.168.1.1"}'
# Should return {"allowed": true/false, "reason": "..."}
```

---

## Dependencies

| Component | Dependency | Risk |
|-----------|------------|------|
| Handler | Service `GetByUUID()` | Already exists (line 115) |
| Handler | Service `GetByID()` | Already exists (line 103) |
| Handler | `strconv.ParseUint` | Standard library |
| Tests | Test fixtures | UUID must be set explicitly |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Performance (UUID lookup) | Low | Low | UUID is indexed, same performance as ID |
| Numeric string as UUID | Low | Medium | Check numeric first, then UUID |
| Breaking changes | Low | High | Backward compatible - numeric IDs still work |
| Test fixture setup | Medium | Low | Ensure test ACLs have known UUIDs |

---

## Estimated Effort

| Task | Complexity | Duration |
|------|------------|----------|
| Add resolveAccessList helper | Simple | 10 min |
| Update 4 handlers | Simple | 20 min |
| Update unit tests | Medium | 30 min |
| Update E2E tests | Simple | 10 min |
| Validation & QA | Medium | 20 min |
| **Total** | | **~90 min** |

---

## Acceptance Criteria

1. ✅ All 4 access list handlers accept both numeric ID and UUID
2. ✅ Numeric ID lookup is attempted first (backward compatibility)
3. ✅ E2E tests `acl-enforcement.spec.ts` pass without modifications to test logic (only changing `.id` to `.uuid`)
4. ✅ Unit test coverage remains above 85%
5. ✅ No breaking changes to existing API consumers using numeric IDs
