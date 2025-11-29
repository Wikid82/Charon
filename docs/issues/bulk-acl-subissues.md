# Sub-Issues for Bulk ACL Testing

## Parent Issue
[Link to main testing issue]

---

## Sub-Issue #1: Basic Functionality Testing

**Title**: `[Bulk ACL Testing] Basic Functionality - Selection and Application`

**Labels**: `testing`, `manual-testing`, `bulk-acl`

**Description**:
Test the core functionality of the bulk ACL feature - selecting hosts and applying access lists.

**Test Checklist:**
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

**Expected Results:**
- All checkboxes functional
- Selection count accurate
- Modal displays correctly
- ACL applies to all selected hosts
- Database reflects changes

**Test Environment:** Local development

---

## Sub-Issue #2: ACL Removal Testing

**Title**: `[Bulk ACL Testing] ACL Removal Functionality`

**Labels**: `testing`, `manual-testing`, `bulk-acl`

**Description**:
Test the ability to remove access lists from multiple hosts simultaneously.

**Test Checklist:**
- [ ] Select hosts that have ACLs assigned
- [ ] Open Bulk Actions modal
- [ ] Select "🚫 Remove Access List" option
- [ ] Confirm removal dialog appears
- [ ] Proceed with removal
- [ ] Verify toast shows "Access list removed from X host(s)"
- [ ] Confirm hosts no longer have ACL assigned in UI
- [ ] Check database to verify `access_list_id` is NULL

**Expected Results:**
- Removal option clearly visible
- Confirmation dialog prevents accidental removal
- All selected hosts have ACL removed
- Database updated correctly (NULL values)

**Test Environment:** Local development

---

## Sub-Issue #3: Error Handling Testing

**Title**: `[Bulk ACL Testing] Error Handling and Edge Cases`

**Labels**: `testing`, `manual-testing`, `bulk-acl`, `error-handling`

**Description**:
Test error scenarios and edge cases to ensure graceful degradation.

**Test Checklist:**
- [ ] Select multiple hosts including one that doesn't exist
- [ ] Apply ACL via bulk action
- [ ] Verify toast shows partial success: "Updated X host(s), Y failed"
- [ ] Confirm successful hosts were updated
- [ ] Test with no hosts selected (button should not appear)
- [ ] Test with empty ACL list (dropdown should show appropriate message)
- [ ] Disconnect backend - verify network error handling
- [ ] Test applying invalid ACL ID (edge case)

**Expected Results:**
- Partial failures handled gracefully
- Clear error messages displayed
- No data corruption on partial failures
- Network errors caught and reported

**Test Environment:** Local development + simulated failures

---

## Sub-Issue #4: UI/UX Testing

**Title**: `[Bulk ACL Testing] UI/UX and Usability`

**Labels**: `testing`, `manual-testing`, `bulk-acl`, `ui-ux`

**Description**:
Test the user interface and experience aspects of the bulk ACL feature.

**Test Checklist:**
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

**Expected Results:**
- Clean, professional UI
- Intuitive user flow
- Proper loading states
- Mobile-friendly
- Accessible (keyboard navigation)

**Test Environment:** Local development (multiple screen sizes)

---

## Sub-Issue #5: Integration Testing

**Title**: `[Bulk ACL Testing] Integration and Performance`

**Labels**: `testing`, `manual-testing`, `bulk-acl`, `integration`, `performance`

**Description**:
Test the feature in realistic scenarios and with varying data loads.

**Test Checklist:**
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

**Expected Results:**
- Single Caddy reload per bulk operation
- Performance acceptable up to 50+ hosts
- No race conditions with rapid operations
- Graceful handling of deleted/disabled entities

**Test Environment:** Docker production build

---

## Sub-Issue #6: Cross-Browser Testing

**Title**: `[Bulk ACL Testing] Cross-Browser Compatibility`

**Labels**: `testing`, `manual-testing`, `bulk-acl`, `cross-browser`

**Description**:
Verify the feature works across all major browsers and devices.

**Test Checklist:**
- [ ] Chrome/Chromium (latest)
- [ ] Firefox (latest)
- [ ] Safari (macOS/iOS)
- [ ] Edge (latest)
- [ ] Mobile Chrome (Android)
- [ ] Mobile Safari (iOS)

**Expected Results:**
- Feature works identically across all browsers
- No CSS layout issues
- No JavaScript errors in console
- Touch interactions work on mobile

**Test Environment:** Multiple browsers/devices

---

## Sub-Issue #7: Regression Testing

**Title**: `[Bulk ACL Testing] Regression Testing - Existing Features`

**Labels**: `testing`, `manual-testing`, `bulk-acl`, `regression`

**Description**:
Ensure the new bulk ACL feature doesn't break existing functionality.

**Test Checklist:**
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

**Expected Results:**
- Zero regressions
- All existing features work as before
- No performance degradation
- No new bugs introduced

**Test Environment:** Docker production build

---

## Creating Sub-Issues on GitHub

For each sub-issue above:

1. Go to the repository's Issues tab
2. Click "New Issue"
3. Copy the content from the relevant section
4. Add to the parent issue description: "Part of #[parent-issue-number]"
5. Assign appropriate labels
6. Set milestone to `v0.2.0-beta.2`
7. Assign to tester if known

## Testing Progress Tracking

Update the parent issue with:
```markdown
## Sub-Issues Progress

- [ ] #XXX - Basic Functionality Testing
- [ ] #XXX - ACL Removal Testing
- [ ] #XXX - Error Handling Testing
- [ ] #XXX - UI/UX Testing
- [ ] #XXX - Integration Testing
- [ ] #XXX - Cross-Browser Testing
- [ ] #XXX - Regression Testing
```
