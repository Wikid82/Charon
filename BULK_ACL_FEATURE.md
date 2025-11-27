# Bulk ACL Application Feature

## Overview
Implemented a bulk ACL (Access Control List) application feature that allows users to quickly apply or remove access lists from multiple proxy hosts at once, eliminating the need to edit each host individually.

## User Workflow Improvements

### Previous Workflow (Manual)
1. Create proxy hosts
2. Create access list
3. **Edit each host individually** to apply the ACL (tedious for many hosts)

### New Workflow (Bulk)
1. Create proxy hosts
2. Create access list
3. **Select multiple hosts** → Bulk Actions → Apply/Remove ACL (one operation)

## Implementation Details

### Backend (`backend/internal/api/handlers/proxy_host_handler.go`)

**New Endpoint**: `PUT /api/v1/proxy-hosts/bulk-update-acl`

**Request Body**:
```json
{
  "host_uuids": ["uuid-1", "uuid-2", "uuid-3"],
  "access_list_id": 42  // or null to remove ACL
}
```

**Response**:
```json
{
  "updated": 2,
  "errors": [
    {"uuid": "uuid-3", "error": "proxy host not found"}
  ]
}
```

**Features**:
- Updates multiple hosts in a single database transaction
- Applies Caddy config once for all updates (efficient)
- Partial failure handling (returns both successes and errors)
- Validates host existence before applying ACL
- Supports both applying and removing ACLs (null = remove)

### Frontend

#### API Client (`frontend/src/api/proxyHosts.ts`)
```typescript
export const bulkUpdateACL = async (
  hostUUIDs: string[],
  accessListID: number | null
): Promise<BulkUpdateACLResponse>
```

#### React Query Hook (`frontend/src/hooks/useProxyHosts.ts`)
```typescript
const { bulkUpdateACL, isBulkUpdating } = useProxyHosts()

// Usage
await bulkUpdateACL(['uuid-1', 'uuid-2'], 42)  // Apply ACL 42
await bulkUpdateACL(['uuid-1', 'uuid-2'], null) // Remove ACL
```

#### UI Components (`frontend/src/pages/ProxyHosts.tsx`)

**Multi-Select Checkboxes**:
- Checkbox column added to proxy hosts table
- "Select All" checkbox in table header
- Individual checkboxes per row

**Bulk Actions UI**:
- "Bulk Actions" button appears when hosts are selected
- Shows count of selected hosts
- Opens modal with ACL selection dropdown

**Modal Features**:
- Lists all enabled access lists
- "Remove Access List" option (sets null)
- Real-time feedback on success/failure
- Toast notifications for user feedback

## Testing

### Backend Tests (`proxy_host_handler_test.go`)
- ✅ `TestProxyHostHandler_BulkUpdateACL_Success` - Apply ACL to multiple hosts
- ✅ `TestProxyHostHandler_BulkUpdateACL_RemoveACL` - Remove ACL (null value)
- ✅ `TestProxyHostHandler_BulkUpdateACL_PartialFailure` - Mixed success/failure
- ✅ `TestProxyHostHandler_BulkUpdateACL_EmptyUUIDs` - Validation error
- ✅ `TestProxyHostHandler_BulkUpdateACL_InvalidJSON` - Malformed request

### Frontend Tests
**API Tests** (`proxyHosts-bulk.test.ts`):
- ✅ Apply ACL to multiple hosts
- ✅ Remove ACL with null value
- ✅ Handle partial failures
- ✅ Handle empty host list
- ✅ Propagate API errors

**Hook Tests** (`useProxyHosts-bulk.test.tsx`):
- ✅ Apply ACL via mutation
- ✅ Remove ACL via mutation
- ✅ Query invalidation after success
- ✅ Error handling
- ✅ Loading state tracking

**Test Results**:
- Backend: All tests passing (106+ tests)
- Frontend: All tests passing (132 tests)

## Usage Examples

### Example 1: Apply ACL to Multiple Hosts
```typescript
// Select hosts in UI
setSelectedHosts(new Set(['host-1-uuid', 'host-2-uuid', 'host-3-uuid']))

// User clicks "Bulk Actions" → Selects ACL from dropdown
await bulkUpdateACL(['host-1-uuid', 'host-2-uuid', 'host-3-uuid'], 5)

// Result: "Access list applied to 3 host(s)"
```

### Example 2: Remove ACL from Hosts
```typescript
// User selects "Remove Access List" from dropdown
await bulkUpdateACL(['host-1-uuid', 'host-2-uuid'], null)

// Result: "Access list removed from 2 host(s)"
```

### Example 3: Partial Failure Handling
```typescript
const result = await bulkUpdateACL(['valid-uuid', 'invalid-uuid'], 10)

// result = {
//   updated: 1,
//   errors: [{ uuid: 'invalid-uuid', error: 'proxy host not found' }]
// }

// Toast: "Updated 1 host(s), 1 failed"
```

## Benefits

1. **Time Savings**: Apply ACLs to dozens of hosts in one click vs. editing each individually
2. **User-Friendly**: Clear visual feedback with checkboxes and selection count
3. **Error Resilient**: Partial failures don't block the entire operation
4. **Efficient**: Single Caddy config reload for all updates
5. **Flexible**: Supports both applying and removing ACLs
6. **Well-Tested**: Comprehensive test coverage for all scenarios

## Future Enhancements (Optional)

- Add bulk ACL application from Access Lists page (when creating/editing ACL)
- Bulk enable/disable hosts
- Bulk delete hosts
- Bulk certificate assignment
- Filter hosts before selection (e.g., "Select all hosts without ACL")

## Related Files Modified

### Backend
- `backend/internal/api/handlers/proxy_host_handler.go` (+73 lines)
- `backend/internal/api/handlers/proxy_host_handler_test.go` (+140 lines)

### Frontend
- `frontend/src/api/proxyHosts.ts` (+19 lines)
- `frontend/src/hooks/useProxyHosts.ts` (+11 lines)
- `frontend/src/pages/ProxyHosts.tsx` (+95 lines)
- `frontend/src/api/__tests__/proxyHosts-bulk.test.ts` (+93 lines, new file)
- `frontend/src/hooks/__tests__/useProxyHosts-bulk.test.tsx` (+149 lines, new file)

**Total**: ~580 lines added (including tests)
