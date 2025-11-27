#!/bin/bash
set -e

REPO="Wikid82/cpmp"
MILESTONE="v0.2.0-beta.2"

echo "Creating Bulk ACL Testing Issues for $REPO"
echo "============================================"

# Create main issue
echo ""
echo "Creating main testing issue..."
MAIN_ISSUE=$(gh issue create \
  --repo "$REPO" \
  --title "Test: Bulk ACL Application Feature" \
  --label "beta,high,feature,frontend,backend" \
  --body "## Description

Comprehensive testing required for the newly implemented Bulk ACL (Access Control List) application feature. This feature allows users to apply or remove access lists from multiple proxy hosts simultaneously.

## Feature Overview

The bulk ACL feature introduces:
- Multi-select checkboxes in Proxy Hosts table
- Bulk Actions button with ACL selection modal
- Backend endpoint: \`PUT /api/v1/proxy-hosts/bulk-update-acl\`
- Comprehensive error handling for partial failures

## Testing Status

### Backend Testing ✅ (Completed)
- [x] Unit tests for \`BulkUpdateACL\` handler (5 tests)
- [x] Coverage: 82.2% maintained

### Frontend Testing ✅ (Completed)
- [x] Unit tests for API client and hooks (10 tests)
- [x] Coverage: 86.06% (improved from 85.57%)

### Manual Testing 🔴 (Required)
See sub-issues below for detailed test plans.

## Sub-Issues

- [ ] #TBD - Basic Functionality Testing
- [ ] #TBD - ACL Removal Testing
- [ ] #TBD - Error Handling Testing
- [ ] #TBD - UI/UX Testing
- [ ] #TBD - Integration Testing
- [ ] #TBD - Cross-Browser Testing
- [ ] #TBD - Regression Testing

## Success Criteria

- ✅ All manual test checklists completed
- ✅ No critical bugs found
- ✅ Performance acceptable with 50+ hosts
- ✅ UI/UX meets design standards
- ✅ Cross-browser compatibility confirmed
- ✅ No regressions in existing features

## Related Files

**Backend:**
- \`backend/internal/api/handlers/proxy_host_handler.go\`
- \`backend/internal/api/handlers/proxy_host_handler_test.go\`

**Frontend:**
- \`frontend/src/pages/ProxyHosts.tsx\`
- \`frontend/src/api/proxyHosts.ts\`
- \`frontend/src/hooks/useProxyHosts.ts\`

**Documentation:**
- \`BULK_ACL_FEATURE.md\`
- \`docs/issues/bulk-acl-testing.md\`
- \`docs/issues/bulk-acl-subissues.md\`

**Implementation Date**: November 27, 2025
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created main issue #$MAIN_ISSUE"

# Sub-issue 1: Basic Functionality
echo ""
echo "Creating sub-issue #1: Basic Functionality..."
SUB1=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] Basic Functionality - Selection and Application" \
  --label "beta,medium,feature,frontend" \
  --body "Part of #$MAIN_ISSUE

## Description
Test the core functionality of the bulk ACL feature - selecting hosts and applying access lists.

## Test Checklist
- [ ] Navigate to Proxy Hosts page
- [ ] Verify checkbox column appears in table
- [ ] Select individual hosts using checkboxes
- [ ] Verify \"Select All\" checkbox works correctly
- [ ] Confirm selection count displays accurately
- [ ] Click \"Bulk Actions\" button - modal should appear
- [ ] Select an ACL from dropdown - hosts should update
- [ ] Verify toast notification shows success message
- [ ] Confirm hosts table refreshes with updated ACL assignments
- [ ] Check database to verify \`access_list_id\` fields updated

## Expected Results
- All checkboxes functional
- Selection count accurate
- Modal displays correctly
- ACL applies to all selected hosts
- Database reflects changes

## Test Environment
Local development
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB1"

# Sub-issue 2: ACL Removal
echo ""
echo "Creating sub-issue #2: ACL Removal..."
SUB2=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] ACL Removal Functionality" \
  --label "beta,medium,feature,frontend" \
  --body "Part of #$MAIN_ISSUE

## Description
Test the ability to remove access lists from multiple hosts simultaneously.

## Test Checklist
- [ ] Select hosts that have ACLs assigned
- [ ] Open Bulk Actions modal
- [ ] Select \"🚫 Remove Access List\" option
- [ ] Confirm removal dialog appears
- [ ] Proceed with removal
- [ ] Verify toast shows \"Access list removed from X host(s)\"
- [ ] Confirm hosts no longer have ACL assigned in UI
- [ ] Check database to verify \`access_list_id\` is NULL

## Expected Results
- Removal option clearly visible
- Confirmation dialog prevents accidental removal
- All selected hosts have ACL removed
- Database updated correctly (NULL values)

## Test Environment
Local development
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB2"

# Sub-issue 3: Error Handling
echo ""
echo "Creating sub-issue #3: Error Handling..."
SUB3=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] Error Handling and Edge Cases" \
  --label "beta,medium,feature,backend" \
  --body "Part of #$MAIN_ISSUE

## Description
Test error scenarios and edge cases to ensure graceful degradation.

## Test Checklist
- [ ] Select multiple hosts including one that doesn't exist
- [ ] Apply ACL via bulk action
- [ ] Verify toast shows partial success: \"Updated X host(s), Y failed\"
- [ ] Confirm successful hosts were updated
- [ ] Test with no hosts selected (button should not appear)
- [ ] Test with empty ACL list (dropdown should show appropriate message)
- [ ] Disconnect backend - verify network error handling
- [ ] Test applying invalid ACL ID (edge case)

## Expected Results
- Partial failures handled gracefully
- Clear error messages displayed
- No data corruption on partial failures
- Network errors caught and reported

## Test Environment
Local development + simulated failures
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB3"

# Sub-issue 4: UI/UX
echo ""
echo "Creating sub-issue #4: UI/UX..."
SUB4=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] UI/UX and Usability" \
  --label "beta,medium,frontend" \
  --body "Part of #$MAIN_ISSUE

## Description
Test the user interface and experience aspects of the bulk ACL feature.

## Test Checklist
- [ ] Verify checkboxes align properly in table
- [ ] Test checkbox hover states
- [ ] Verify \"Bulk Actions\" button appears/disappears based on selection
- [ ] Test modal appearance and dismissal (click outside, ESC key)
- [ ] Verify dropdown styling and readability
- [ ] Test loading state (\`isBulkUpdating\`) - button should show \"Updating...\"
- [ ] Verify selection persists during table sorting
- [ ] Test selection persistence during table filtering (if applicable)
- [ ] Verify toast notifications don't overlap
- [ ] Test on mobile viewport (responsive design)

## Expected Results
- Clean, professional UI
- Intuitive user flow
- Proper loading states
- Mobile-friendly
- Accessible (keyboard navigation)

## Test Environment
Local development (multiple screen sizes)
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB4"

# Sub-issue 5: Integration
echo ""
echo "Creating sub-issue #5: Integration..."
SUB5=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] Integration and Performance" \
  --label "beta,high,feature,backend,frontend" \
  --body "Part of #$MAIN_ISSUE

## Description
Test the feature in realistic scenarios and with varying data loads.

## Test Checklist
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

## Expected Results
- Single Caddy reload per bulk operation
- Performance acceptable up to 50+ hosts
- No race conditions with rapid operations
- Graceful handling of deleted/disabled entities

## Test Environment
Docker production build
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB5"

# Sub-issue 6: Cross-Browser
echo ""
echo "Creating sub-issue #6: Cross-Browser..."
SUB6=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] Cross-Browser Compatibility" \
  --label "beta,low,frontend" \
  --body "Part of #$MAIN_ISSUE

## Description
Verify the feature works across all major browsers and devices.

## Test Checklist
- [ ] Chrome/Chromium (latest)
- [ ] Firefox (latest)
- [ ] Safari (macOS/iOS)
- [ ] Edge (latest)
- [ ] Mobile Chrome (Android)
- [ ] Mobile Safari (iOS)

## Expected Results
- Feature works identically across all browsers
- No CSS layout issues
- No JavaScript errors in console
- Touch interactions work on mobile

## Test Environment
Multiple browsers/devices
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB6"

# Sub-issue 7: Regression
echo ""
echo "Creating sub-issue #7: Regression..."
SUB7=$(gh issue create \
  --repo "$REPO" \
  --title "[Bulk ACL Testing] Regression Testing - Existing Features" \
  --label "beta,high,feature,frontend,backend" \
  --body "Part of #$MAIN_ISSUE

## Description
Ensure the new bulk ACL feature doesn't break existing functionality.

## Test Checklist
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

## Expected Results
- Zero regressions
- All existing features work as before
- No performance degradation
- No new bugs introduced

## Test Environment
Docker production build
" | grep -oP '(?<=github.com/Wikid82/cpmp/issues/)\d+')

echo "✓ Created sub-issue #$SUB7"

# Update main issue with sub-issue numbers
echo ""
echo "Updating main issue with sub-issue references..."
gh issue edit "$MAIN_ISSUE" \
  --repo "$REPO" \
  --body "## Description

Comprehensive testing required for the newly implemented Bulk ACL (Access Control List) application feature. This feature allows users to apply or remove access lists from multiple proxy hosts simultaneously.

## Feature Overview

The bulk ACL feature introduces:
- Multi-select checkboxes in Proxy Hosts table
- Bulk Actions button with ACL selection modal
- Backend endpoint: \`PUT /api/v1/proxy-hosts/bulk-update-acl\`
- Comprehensive error handling for partial failures

## Testing Status

### Backend Testing ✅ (Completed)
- [x] Unit tests for \`BulkUpdateACL\` handler (5 tests)
- [x] Coverage: 82.2% maintained

### Frontend Testing ✅ (Completed)
- [x] Unit tests for API client and hooks (10 tests)
- [x] Coverage: 86.06% (improved from 85.57%)

### Manual Testing 🔴 (Required)
See sub-issues below for detailed test plans.

## Sub-Issues

- [ ] #$SUB1 - Basic Functionality Testing
- [ ] #$SUB2 - ACL Removal Testing
- [ ] #$SUB3 - Error Handling Testing
- [ ] #$SUB4 - UI/UX Testing
- [ ] #$SUB5 - Integration Testing
- [ ] #$SUB6 - Cross-Browser Testing
- [ ] #$SUB7 - Regression Testing

## Success Criteria

- ✅ All manual test checklists completed
- ✅ No critical bugs found
- ✅ Performance acceptable with 50+ hosts
- ✅ UI/UX meets design standards
- ✅ Cross-browser compatibility confirmed
- ✅ No regressions in existing features

## Related Files

**Backend:**
- \`backend/internal/api/handlers/proxy_host_handler.go\`
- \`backend/internal/api/handlers/proxy_host_handler_test.go\`

**Frontend:**
- \`frontend/src/pages/ProxyHosts.tsx\`
- \`frontend/src/api/proxyHosts.ts\`
- \`frontend/src/hooks/useProxyHosts.ts\`

**Documentation:**
- \`BULK_ACL_FEATURE.md\`
- \`docs/issues/bulk-acl-testing.md\`
- \`docs/issues/bulk-acl-subissues.md\`

**Implementation Date**: November 27, 2025
"

echo "✓ Updated main issue"

echo ""
echo "============================================"
echo "✅ Successfully created all issues!"
echo ""
echo "Main Issue: #$MAIN_ISSUE"
echo "Sub-Issues: #$SUB1, #$SUB2, #$SUB3, #$SUB4, #$SUB5, #$SUB6, #$SUB7"
echo ""
echo "View them at: https://github.com/$REPO/issues/$MAIN_ISSUE"
