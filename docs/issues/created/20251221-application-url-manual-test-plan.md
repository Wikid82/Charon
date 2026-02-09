---
title: "Application URL Feature - Manual Test Plan"
labels:
  - manual-testing
  - feature
  - user-management
type: testing
priority: high
---

# Application URL Feature - Manual Test Plan

**Feature**: Application URL Configuration & User Invitation Preview
**Status**: Ready for Manual Testing

---

## Overview

This test plan covers the new Application URL configuration feature and its integration with user invitations. The feature allows administrators to configure the public URL used in invitation emails and provides a preview function to verify invite links before sending.

---

## Test Scenarios

### 1. Application URL Configuration - Valid URLs

**Objective**: Verify that valid URLs can be configured and saved correctly.

**Prerequisites**:

- Logged in as an administrator
- Access to System Settings page

**Steps**:

1. Navigate to **System Settings** (gear icon in sidebar)
2. Scroll to the **"Application URL"** section
3. Test each of the following valid URLs:

   a. **HTTPS with domain**:
      - Enter: `https://charon.example.com`
      - Click **"Validate"**
      - Verify: Shows normalized URL without errors
      - Click **"Test"**
      - Verify: New browser tab opens to the URL
      - Click **"Save Changes"**
      - Verify: Success toast appears
      - Refresh page
      - Verify: URL is still set

   b. **HTTPS with custom port**:
      - Enter: `https://charon.example.com:8443`
      - Click **"Validate"**
      - Verify: Shows normalized URL without errors
      - Click **"Save Changes"**
      - Verify: Saves successfully

   c. **HTTP with warning** (internal testing):
      - Enter: `http://192.168.1.100:8080`
      - Click **"Validate"**
      - Verify: Shows warning about using HTTP instead of HTTPS
      - Verify: URL is still marked as valid
      - Click **"Save Changes"**
      - Verify: Saves successfully

**Expected Results**:

- [ ] All valid URLs are accepted
- [ ] Normalized URLs are displayed correctly
- [ ] HTTP URLs show security warning but still save
- [ ] Test button opens URLs in new tab
- [ ] Settings persist after page refresh
- [ ] Success toast appears after saving

---

### 2. Application URL Configuration - Invalid URLs

**Objective**: Verify that invalid URLs are rejected with appropriate error messages.

**Prerequisites**:

- Logged in as an administrator
- Access to System Settings page

**Steps**:

1. Navigate to **System Settings** → **Application URL**
2. Test each of the following invalid URLs:

   a. **Missing protocol**:
      - Enter: `charon.example.com`
      - Click **"Validate"**
      - Verify: Shows error "URL must start with http:// or https://"
      - Verify: Cannot save (Save button disabled or shows error)

   b. **URL with path**:
      - Enter: `https://charon.example.com/admin`
      - Click **"Validate"**
      - Verify: Shows error "cannot include path components"
      - Verify: Cannot save

   c. **URL with trailing slash**:
      - Enter: `https://charon.example.com/`
      - Click **"Validate"**
      - Verify: Either auto-corrects to `https://charon.example.com` OR shows error

   d. **Wrong protocol**:
      - Enter: `ftp://charon.example.com`
      - Click **"Validate"**
      - Verify: Shows error about invalid protocol

   e. **Empty URL**:
      - Leave field empty
      - Click **"Validate"**
      - Verify: Shows error or disables validate button

**Expected Results**:

- [ ] All invalid URLs are rejected
- [ ] Clear error messages are displayed
- [ ] Save button is disabled for invalid URLs
- [ ] No invalid URLs can be persisted to database

---

### 3. User Invitation Preview - With Configured URL

**Objective**: Verify invite preview works correctly when Application URL is configured.

**Prerequisites**:

- Logged in as an administrator
- Application URL configured (e.g., `https://charon.example.com`)

**Steps**:

1. Navigate to **Users** page
2. Click **"Add User"** or **"Invite User"** button
3. Enter email: `testuser@example.com`
4. Click **"Preview Invite"** button
5. Observe the preview modal/section

**Expected Results**:

- [ ] Preview shows full invite URL: `https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW`
- [ ] Base URL displayed: `https://charon.example.com`
- [ ] Configuration status shows: ✅ Configured
- [ ] No warning message is displayed
- [ ] Warning indicator is not shown

---

### 4. User Invitation Preview - Without Configured URL

**Objective**: Verify warning message appears when Application URL is not configured.

**Prerequisites**:

- Logged in as an administrator
- Application URL NOT configured (clear the setting first)

**Steps**:

1. Go to **System Settings** → Clear Application URL setting → Save
2. Navigate to **Users** page
3. Click **"Add User"** or **"Invite User"** button
4. Enter email: `testuser@example.com`
5. Click **"Preview Invite"** button
6. Observe the preview modal/section

**Expected Results**:

- [ ] Preview shows localhost URL: `http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW`
- [ ] Warning indicator is displayed (⚠️)
- [ ] Warning message: "Application URL not configured. The invite link may not be accessible from external networks."
- [ ] Configuration status shows: ❌ Not Configured
- [ ] Helpful link or button to navigate to System Settings

---

### 5. Multi-Language Support

**Objective**: Verify feature works correctly in all supported languages.

**Prerequisites**:

- Logged in as an administrator

**Steps**:

1. Test in each language:
   - English
   - Spanish (Español)
   - French (Français)
   - German (Deutsch)
   - Chinese (中文)

2. For each language:
   - Go to **System Settings** → Change language
   - Navigate to **Application URL** section
   - Verify section title is translated
   - Verify description is translated
   - Enter invalid URL: `charon.example.com`
   - Click **"Validate"**
   - Verify error message is translated
   - Go to **Users** → Preview Invite
   - Verify warning message is translated

**Expected Results**:

- [ ] All UI text is properly translated
- [ ] No English fallbacks appear (except for technical terms)
- [ ] Error and warning messages are localized
- [ ] Button labels are translated
- [ ] Help text is translated

---

### 6. Admin-Only Access Control

**Objective**: Verify non-admin users cannot access Application URL configuration.

**Prerequisites**:

- Admin account and non-admin user account

**Steps**:

1. **As Admin**:
   - Navigate to System Settings
   - Verify Application URL section is visible
   - Verify can modify settings

2. **As Non-Admin User**:
   - Log out and log in as regular user
   - Navigate to System Settings (if accessible)
   - Verify Application URL section is either:
     - Not visible at all, OR
     - Visible but disabled/read-only

3. **API Access Test** (optional, requires curl/Postman):
   - Get non-admin user token
   - Attempt to call: `POST /api/v1/settings/validate-url`
   - Verify: Returns 403 Forbidden
   - Attempt to call: `POST /api/v1/users/preview-invite-url`
   - Verify: Returns 403 Forbidden

**Expected Results**:

- [ ] Admin users can access and modify Application URL
- [ ] Non-admin users cannot access or modify settings
- [ ] API endpoints return 403 for non-admin requests
- [ ] No privilege escalation is possible

---

### 7. Settings Persistence & Integration

**Objective**: Verify Application URL setting persists correctly and integrates with user invitation flow.

**Prerequisites**:

- Logged in as administrator
- Clean database state

**Steps**:

1. **Configure URL**:
   - Go to System Settings
   - Set Application URL: `https://test.example.com`
   - Save and verify success

2. **Restart Container** (Docker only):
   - `docker restart charon`
   - Wait for container to start
   - Log back in

3. **Verify Persistence**:
   - Go to System Settings
   - Verify Application URL is still: `https://test.example.com`

4. **Create Actual User Invitation**:
   - Go to Users page
   - Click "Add User"
   - Enter email, role, etc.
   - Submit invitation
   - Check email inbox (if SMTP configured)
   - Verify invite link uses configured URL

5. **Database Check** (optional):
   - Query database: `SELECT * FROM settings WHERE key = 'app.public_url';`
   - Verify value is `https://test.example.com`

**Expected Results**:

- [ ] Application URL persists after save
- [ ] Setting survives container restart
- [ ] Actual invite emails use configured URL
- [ ] Database stores correct value
- [ ] No corruption or data loss

---

### 8. Edge Cases & Error Handling

**Objective**: Verify robust error handling for edge cases.

**Prerequisites**:

- Logged in as administrator

**Steps**:

1. **Very Long URL**:
   - Enter URL with 500+ characters
   - Attempt to validate and save
   - Verify: Shows appropriate error or truncation

2. **Special Characters**:
   - Try URL: `https://charon.example.com?test=1&foo=bar`
   - Verify: Rejected (query params not allowed)

3. **Unicode Domain**:
   - Try URL: `https://例え.jp` (internationalized domain)
   - Verify: Either accepted or shows clear error

4. **Rapid Clicks**:
   - Enter valid URL
   - Click "Validate" multiple times rapidly
   - Verify: No duplicate requests or UI freezing
   - Click "Test" multiple times rapidly
   - Verify: Doesn't open excessive tabs

5. **Network Error Simulation** (optional):
   - Disconnect network
   - Try to save Application URL
   - Verify: Shows network error message
   - Reconnect network
   - Retry save
   - Verify: Works correctly after reconnection

**Expected Results**:

- [ ] Long URLs handled gracefully
- [ ] Special characters rejected with clear messages
- [ ] No duplicate API requests
- [ ] Network errors handled gracefully
- [ ] UI remains responsive during errors

---

### 9. UI/UX Verification

**Objective**: Verify user interface is intuitive and accessible.

**Prerequisites**:

- Logged in as administrator

**Steps**:

1. **Visual Design**:
   - Navigate to System Settings → Application URL
   - Verify:
     - Section has clear title and description
     - Input field is properly sized
     - Buttons are visually distinct
     - Error messages are color-coded (red)
     - Warnings are color-coded (yellow/orange)
     - Success states are color-coded (green)

2. **Keyboard Navigation**:
   - Tab through all elements in order
   - Verify: Focus indicators are visible
   - Press Enter on "Validate" button
   - Verify: Triggers validation
   - Press Enter on "Test" button
   - Verify: Opens URL in new tab

3. **Mobile Responsive** (if applicable):
   - Open System Settings on mobile device or narrow browser window
   - Verify: Application URL section is usable
   - Verify: Buttons don't overflow
   - Verify: Input field adapts to screen width

4. **Loading States**:
   - Enter URL and click "Validate"
   - Observe: Loading indicator appears during validation
   - Click "Save Changes"
   - Observe: Loading indicator appears during save

5. **Help Text**:
   - Verify: Helper text explains URL format requirements
   - Verify: Examples are provided
   - Verify: Link to documentation (if present)

**Expected Results**:

- [ ] UI is visually consistent with rest of application
- [ ] Keyboard navigation works correctly
- [ ] Mobile layout is usable
- [ ] Loading states are clear
- [ ] Help text is informative and accurate

---

### 10. Documentation Accuracy

**Objective**: Verify all documentation matches actual behavior.

**Prerequisites**:

- Access to documentation

**Pages to Review**:

- [ ] `docs/getting-started.md` - Application URL configuration section
- [ ] `docs/features.md` - Application URL feature description
- [ ] `docs/api.md` - API endpoint documentation

**Check for**:

- [ ] Correct endpoint URLs
- [ ] Accurate request/response examples
- [ ] No broken links
- [ ] Screenshots or references are accurate (if present)
- [ ] Examples can be copy-pasted and work
- [ ] No typos or formatting issues
- [ ] Matches actual UI labels and messages

---

## Acceptance Criteria

All test scenarios must pass with the following results:

- [ ] All valid URLs are accepted and saved
- [ ] All invalid URLs are rejected with clear errors
- [ ] Invite preview shows correct URL when configured
- [ ] Warning appears when URL is not configured
- [ ] Multi-language support works in all 5 languages
- [ ] Admin-only access is enforced
- [ ] Settings persist across restarts
- [ ] Edge cases are handled gracefully
- [ ] UI is intuitive and accessible
- [ ] Documentation is accurate and helpful

---

## Testing Notes

**Test Environment**:

- Charon Version: _________________
- Browser: _________________
- OS: _________________
- Database: SQLite / PostgreSQL (circle one)

**Special Considerations**:

- Test with both HTTP and HTTPS configured URLs
- Verify SMTP integration if configured
- Test on actual external network if possible
- Consider firewall/proxy configurations

---

**Tester**: ________________
**Date**: ________________
**Result**: [ ] PASS / [ ] FAIL

**Issues Found** (if any):

1. ___________________________________________
2. ___________________________________________
3. ___________________________________________

**Notes**:

________________________________________________________________
________________________________________________________________
________________________________________________________________
