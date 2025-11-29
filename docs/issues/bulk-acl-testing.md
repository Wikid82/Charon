# Issue: Test Bulk ACL Application Feature

**Labels**: `testing`, `enhancement`, `needs-testing`
**Milestone**: v0.2.0-beta.2
**Priority**: High

## Description

Comprehensive testing required for the newly implemented Bulk ACL (Access Control List) application feature. This feature allows users to apply or remove access lists from multiple proxy hosts simultaneously, replacing the previous manual per-host workflow.

## Feature Overview

**Implementation PR**: [Link to PR]

The bulk ACL feature introduces:
- Multi-select checkboxes in Proxy Hosts table
- Bulk Actions button with ACL selection modal
- Backend endpoint: `PUT /api/v1/proxy-hosts/bulk-update-acl`
- Comprehensive error handling for partial failures

## Testing Scope

### Backend Testing ✅ (Completed)
- [x] Unit tests for `BulkUpdateACL` handler (5 tests)
- [x] Success scenario: Apply ACL to multiple hosts
- [x] Success scenario: Remove ACL (null value)
- [x] Error handling: Partial failures (some hosts fail)
- [x] Validation: Empty UUIDs array
- [x] Validation: Invalid JSON payload
- **Coverage**: 82.2% maintained

### Frontend Testing ✅ (Completed)
- [x] Unit tests for `bulkUpdateACL` API client (5 tests)
- [x] Unit tests for `useBulkUpdateACL` hook (5 tests)
- [x] Build verification (TypeScript compilation)
- **Coverage**: 86.06% (improved from 85.57%)

### Manual Testing 🔴 (Required)

#### Sub-Issue #1: Basic Functionality Testing
**Checklist:**
- [ ] Navigate to Proxy Hosts page
- [ ] Verify checkbox column appears in table
- [ ] Select individual hosts using checkboxes
- [ ] Verify "Select All" checkbox works correctly
- [ ] Confirm selection count displays accurately
- [ ] Click "Bulk Actions" button - modal should appear
- [ ] Select an ACL from dropdown - hosts should update
- [ ] Verify toast notification shows success message
- [ ] Confirm hosts table refreshes with updated ACL assignments
- [ ] Check database to verify `access_list_id` fields updated

#### Sub-Issue #2: ACL Removal Testing
**Checklist:**
- [ ] Select hosts that have ACLs assigned
- [ ] Open Bulk Actions modal
- [ ] Select "🚫 Remove Access List" option
- [ ] Confirm removal dialog appears
- [ ] Proceed with removal
- [ ] Verify toast shows "Access list removed from X host(s)"
- [ ] Confirm hosts no longer have ACL assigned in UI
- [ ] Check database to verify `access_list_id` is NULL

#### Sub-Issue #3: Error Handling Testing
**Checklist:**
- [ ] Select multiple hosts including one that doesn't exist
- [ ] Apply ACL via bulk action
- [ ] Verify toast shows partial success: "Updated X host(s), Y failed"
- [ ] Confirm successful hosts were updated
- [ ] Test with no hosts selected (button should not appear)
- [ ] Test with empty ACL list (dropdown should show appropriate message)
- [ ] Disconnect backend - verify network error handling
- [ ] Test applying invalid ACL ID (edge case)

#### Sub-Issue #4: UI/UX Testing
**Checklist:**
- [ ] Verify checkboxes align properly in table
- [ ] Test checkbox hover states
- [ ] Verify "Bulk Actions" button appears/disappears based on selection
- [ ] Test modal appearance and dismissal (click outside, ESC key)
- [ ] Verify dropdown styling and readability
- [ ] Test loading state (`isBulkUpdating`) - button should show "Updating..."
- [ ] Verify selection persists during table sorting
- [ ] Test selection persistence during table filtering (if applicable)
- [ ] Verify toast notifications don't overlap
- [ ] Test on mobile viewport (responsive design)

#### Sub-Issue #5: Integration Testing
**Checklist:**
- [ ] Create new ACL, immediately apply to multiple hosts
- [ ] Verify Caddy config reloads once (not per host)
- [ ] Test with 1 host selected
- [ ] Test with 10+ hosts selected (performance)
- [ ] Test with 50+ hosts selected (edge case)
- [ ] Apply ACL, then immediately remove it (rapid operations)
- [ ] Apply different ACLs sequentially to same host group
- [ ] Delete a host that's selected, then bulk apply ACL
- [ ] Disable an ACL, verify it doesn't appear in dropdown
- [ ] Test concurrent user scenarios (multi-tab if possible)

#### Sub-Issue #6: Cross-Browser Testing
**Checklist:**
- [ ] Chrome/Chromium (latest)
- [ ] Firefox (latest)
- [ ] Safari (macOS/iOS)
- [ ] Edge (latest)
- [ ] Mobile Chrome (Android)
- [ ] Mobile Safari (iOS)

#### Sub-Issue #7: Regression Testing
**Checklist:**
- [ ] Verify individual proxy host edit still works
- [ ] Confirm single-host ACL assignment unchanged
- [ ] Test proxy host creation with ACL pre-selected
- [ ] Verify ACL deletion prevents assignment
- [ ] Confirm existing ACL features unaffected:
  - [ ] IP-based rules
  - [ ] Geo-blocking rules
  - [ ] Local network only rules
  - [ ] Test IP functionality
- [ ] Verify certificate assignment still works
- [ ] Test proxy host enable/disable toggle

## Test Environments

1. **Local Development**
   - Docker: `docker-compose.local.yml`
   - Backend: `http://localhost:8080`
   - Frontend: `http://localhost:5173`

2. **Docker Production Build**
   - Docker: `docker-compose.yml`
   - Full stack: `http://localhost:80`

3. **VPS/Staging** (if available)
   - Remote environment testing
   - Real SSL certificates
   - Multiple concurrent users

## Success Criteria

- ✅ All manual test checklists completed
- ✅ No critical bugs found
- ✅ Performance acceptable with 50+ hosts
- ✅ UI/UX meets design standards
- ✅ Cross-browser compatibility confirmed
- ✅ No regressions in existing features
- ✅ Documentation updated (if needed)

## Known Limitations

1. Selection state resets on page navigation
2. No "Select hosts without ACL" filter (potential enhancement)
3. No bulk operations from Access Lists page (future feature)
4. Maximum practical limit untested (100+ hosts)

## Related Files

**Backend:**
- `backend/internal/api/handlers/proxy_host_handler.go`
- `backend/internal/api/handlers/proxy_host_handler_test.go`

**Frontend:**
- `frontend/src/pages/ProxyHosts.tsx`
- `frontend/src/api/proxyHosts.ts`
- `frontend/src/hooks/useProxyHosts.ts`
- `frontend/src/api/__tests__/proxyHosts-bulk.test.ts`
- `frontend/src/hooks/__tests__/useProxyHosts-bulk.test.tsx`

**Documentation:**
- `BULK_ACL_FEATURE.md`

## Testing Timeline

**Suggested Schedule:**
- Day 1: Sub-issues #1-3 (Basic + Error Handling)
- Day 2: Sub-issues #4-5 (UI/UX + Integration)
- Day 3: Sub-issues #6-7 (Cross-browser + Regression)

## Reporting Issues

When bugs are found:
1. Create a new bug report with `[Bulk ACL]` prefix
2. Reference this testing issue
3. Include screenshots/videos
4. Provide reproduction steps
5. Tag with `bug`, `bulk-acl` labels

## Notes

- Feature has 100% backend test coverage for new code
- Feature has 100% frontend test coverage for new code
- Performance testing with large datasets (100+ hosts) recommended
- Consider adding E2E tests with Playwright/Cypress in future

---

**Implementation Date**: November 27, 2025
**Developer**: @copilot
**Reviewer**: TBD
**Tester**: TBD
