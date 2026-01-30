# Debug Logging in Action: How to Diagnose Test Failures

This document explains how the new comprehensive debugging infrastructure helps diagnose the E2E test failures with concrete examples.

## What Changed: Before vs. After

### BEFORE: Generic Failure Output
```
  ✗ [chromium] › tests/settings/account-settings.spec.ts › should validate certificate email format
    Timeout 30s exceeded, waiting for expect(locator).toBeDisabled()
       at account-settings.spec.ts:290
```

**Problem**: No information about:
- What page was displayed when it failed
- What network requests were in flight
- What the actual button state was
- How long the test ran before timing out

---

### AFTER: Rich Debug Logging Output

#### 1. **Test Step Logging** (From enhanced global-setup.ts)
```
✅ Global setup complete

🔍 Health Checks:
  ✅ Caddy admin API health (port 2019)        [45ms]
  ✅ Emergency tier-2 server health (port 2020) [123ms]
  ✅ Security modules status verified           [89ms]

🔓 Security Reset:
  ✅ Emergency reset via tier-2 server [134ms]
  ✅ Modules disabled (ACL, WAF, rate-limit, CrowdSec)
  ⏳ Waiting for propagation... [510ms]
```

#### 2. **Network Activity Logging** (From network.ts interceptor)
```
📡 Network Log (automatic)
────────────────────────────────────────────────────────────
Timestamp    │ Method │ URL                         │ Status │ Duration
────────────────────────────────────────────────────────────
03:48:12.456 │ GET    │ /api/auth/profile          │ 200    │ 234ms
03:48:12.690 │ GET    │ /api/settings              │ 200    │ 45ms
03:48:13.001 │ POST   │ /api/certificates          │ 200    │ 567ms
03:48:13.568 │ GET    │ /api/acl/lists             │ 200    │ 89ms
03:48:13.912 │ POST   │ /api/account/email -PEND...│ 422    │ 234ms ⚠️  ERROR
```

**Key Insight**: The 422 error on email update shows the API is rejecting the input, which explains why the button didn't disable—the form never validated successfully.

#### 3. **Locator Matching Logs** (From debug-logger.ts)
```
🎯 Locator Actions:
────────────────────────────────────────────────────────────
[03:48:14.123] ✅ getByRole('button', {name: /save certificate/i}) matched [8ms]
                  -> Elements found: 1
                  -> Action: click()

[03:48:14.234] ❌ getByRole('button', {name: /save certificate/i}) NOT matched [5000ms+]
                  -> Elements found: 0
                  -> Reason: Test timeout while waiting for element
                  -> DOM Analysis:
                     - Dialog present? YES
                     - Form visible? NO (display: none)
                     - Button HTML: <button disabled aria-label="Save...">
```

**Key Insight**: The form wasn't visible in the DOM when the test tried to click the button.

#### 4. **Assertion Logging** (From debug-logger.ts)
```
✓ Assert: "button is enabled" PASS          [15ms]
  └─ Expected: enabled=true
  └─ Actual: enabled=true
  └─ Element state: aria-disabled=false

❌ Assert: "button is disabled" FAIL        [5000ms+]
  └─ Expected: disabled=true
  └─ Actual: disabled=false
  └─ Element state: aria-disabled=false, type=submit, form=cert-form
  └─ Form status: pristine (no changes detected)
  └─ Validation errors found:
    - email: "Invalid email format" (hidden error div)
```

**Key Insight**: The validation error exists but is hidden, so the button remains enabled. The test expected it to disable.

#### 5. **Timing Analysis** (From debug reporter)
```
📊 Test Timeline:
────────────────────────────────────────────────────────────
 0ms  │ ✅ Navigate to /account
 150ms │ ✅ Fill email field with "invalid@"
 250ms │ ✅ Trigger validation (blur event)
 500ms │ ✅ Wait for API response
 700ms │ ❌ FAIL: Button should be disabled (but it's not)
       │    └─ Form validation failed on API side (422)
       │    └─ Error message not visible in DOM
       │    └─ Button has disabled=false
       │    └─ Test timeout after 5000ms of waiting
```

**Key Insight**: The timing shows validation happened (API returned 422), but the form didn't update the UI properly.

## How to Read the Debug Output in Playwright Report

### Step 1: Open the Report
```bash
npx playwright show-report
```

### Step 2: Click Failed Test
The test details page shows:

**Console Logs Section**:
```
[debug] 03:48:12.456: Step "Navigate to account settings"
[debug]   └─ URL transitioned from / to /account
[debug]   └─ Page loaded in 1234ms
[debug]
[debug] 03:48:12.690: Step "Fill certificate email with invalid value"
[debug]   └─ Input focused [12ms]
[debug]   └─ Value set: "invalid@" [23ms]
[debug]
[debug] 03:48:13.001: Step "Trigger validation"
[debug]   └─ Blur event fired [8ms]
[debug]   └─ API request sent: POST /api/account/email [timeout: 5000ms]
[debug]
[debug] 03:48:13.234: Network Response
[debug]   └─ Status: 422 (Unprocessable Entity)
[debug]   └─ Body: {"errors": {"email": "Invalid email format"}}
[debug]   └─ Duration: 234ms
[debug]
[debug] 03:48:13.500: Error context
[debug]   └─ Expected button to be disabled
[debug]   └─ Actual state: enabled
[debug]   └─ Form validation state: pristine
```

### Step 3: Check the Trace
Click "Trace" tab:
- **Timeline**: See each action with exact timing
- **Network**: View all HTTP requests and responses
- **DOM Snapshots**: Inspect page state at each step
- **Console**: See browser console messages

### Step 4: Watch the Video
The video shows:
- What the user would have seen
- Where the UI hung or stalled
- If spinners/loading states appeared
- Exact moment of failure

## Failure Category Examples

### Category 1: Timeout Failures
**Indicator**: `Timeout 30s exceeded, waiting for...`

**Debug Output**:
```
⏱️  Operation Timeline:
  [03:48:14.000] ← Start waiting for locator
  [03:48:14.100]   Network request pending: GET /api/data [+2400ms]
  [03:48:16.500]   API response received (slow network)
  [03:48:16.600]   DOM updated with data
  [03:48:17.000]   ✅ Locator finally matched
  [03:48:17.005] → Success after 3000ms wait
```

**Diagnosis**: The network was slow (2.4s for a 50KB response). Test didn't wait long enough.

**Fix**:
```javascript
await page.waitForLoadState('networkidle'); // Wait for network before assertion
await expect(locator).toBeVisible({timeout: 10000}); // Increase timeout
```

---

### Category 2: Assertion Failures
**Indicator**: `expect(locator).toBeDisabled() failed`

**Debug Output**:
```
✋ Assertion failed: toBeDisabled()
  Expected: disabled=true
  Actual: disabled=false

  Button State:
    - type: submit
    - aria-disabled: false
    - form-attached: true
    - form-valid: false ← ISSUE!

  Form Validation:
    - Field 1: ✅ valid
    - Field 2: ✅ valid
    - Field 3: ❌ invalid (email format)

  DOM Inspection:
    - Error message exists: YES (display: none)
    - Form has error attribute: NO
    - Submit button has disabled attr: NO

  Likely Cause:
    Form validation logic doesn't disable button when form.valid=false
    OR error message display doesn't trigger button disable
```

**Diagnosis**: The component's disable logic isn't working correctly.

**Fix**:
```jsx
// In React component:
const isFormValid = !hasValidationErrors;
<button
  disabled={!isFormValid}  // ← Double-check this logic
  type="submit"
>
  Save
</button>
```

---

### Category 3: Locator Failures
**Indicator**: `getByRole('button', {name: /save/i}): multiple elements found`

**Debug Output**:
```
🚨 Strict Mode Violation: Multiple elements matched
  Selector: getByRole('button', {name: /save/i})

  Elements found: 2

  [1] ✓ <button type="submit">Save Certificate</button>
      └─ Located in: Modal dialog
      └─ Visible: YES
      └─ Class: btn-primary

  [2] ✗ <button type="button">Resave Settings</button>
      └─ Located in: Table row
      └─ Visible: YES
      └─ Class: btn-ghost

  Problem: Selector matches both buttons - test can't decide which to click

  Solution: Scope selector to dialog context
    page.getByRole('dialog').getByRole('button', {name: /save certificate/i})
```

**Diagnosis**: Locator is too broad and matches multiple elements.

**Fix**:
```javascript
// ✅ Good: Scoped to dialog
await page.getByRole('dialog').getByRole('button', {name: /save certificate/i}).click();

// ✅ Also good: Use .first() if scoping isn't possible
await page.getByRole('button', {name: /save certificate/i}).first().click();

// ❌ Bad: Too broad
await page.getByRole('button', {name: /save/i}).click();
```

---

### Category 4: Network/API Failures
**Indicator**: `API returned 422` or `POST /api/endpoint failed with 500`

**Debug Output**:
```
❌ Network Error
  Request: POST /api/account/email
  Status: 422 Unprocessable Entity
  Duration: 234ms

  Request Body:
    {
      "email": "invalid@",  ← Invalid format
      "format": "personal"
    }

  Response Body:
    {
      "code": "INVALID_EMAIL",
      "message": "Email must contain domain",
      "field": "email",
      "errors": [
        "Invalid email format: missing @domain"
      ]
    }

  What Went Wrong:
    1. Form submitted with invalid data
    2. Backend rejected it (expected behavior)
    3. Frontend didn't show error message
    4. Test expected button to disable but it didn't

  Root Cause:
    Error handling code in frontend isn't updating the form state
```

**Diagnosis**: The API is working correctly, but the frontend error handling isn't working.

**Fix**:
```javascript
// In frontend error handler:
try {
  const response = await fetch('/api/account/email', {body});
  if (!response.ok) {
    const error = await response.json();
    setFormErrors(error.errors);  // ← Update form state with error
    setFormErrorVisible(true);     // ← Show error message
  }
} catch (error) {
  setFormError(error.message);
}
```

---

## Real-World Example: The Certificate Email Test

**Test Code** (simplified):
```javascript
test('should validate certificate email format', async ({page}) => {
  await page.goto('/account');

  // Fill with invalid email
  await page.getByLabel('Certificate Email').fill('invalid@');

  // Trigger validation
  await page.getByLabel('Certificate Email').blur();

  // Expect button to disable
  await expect(
    page.getByRole('button', {name: /save certificate/i})
  ).toBeDisabled();  // ← THIS FAILED
});
```

**Debug Output Sequence**:
```
1️⃣  Navigate to /account
    ✅ Page loaded [1234ms]

2️⃣  Fill certificate email field
    ✅ Input found and focused [45ms]
    ✅ Value set to "invalid@" [23ms]

3️⃣  Trigger validation (blur)
    ✅ Blur event fired [8ms]
    📡 API request: POST /api/account/email [payload: {email: "invalid@"}]

4️⃣  Wait for API response
    ✋ Network activity: Waiting...
    ✅ Response received: 422 Unprocessable Entity [234ms]
    └─ Error: "Email must contain @ domain"

5️⃣  Check form error state
    ✅ Form has errors: email = "Email must contain @ domain"
    ✅ Error message DOM element exists
    ❌ But error message has display: none (CSS)

6️⃣  Wait for button to disable
    ⏰ [03:48:14.000] Start waiting for button[disabled]
    ⏰ [03:48:14.500] Still waiting...
    ⏰ [03:48:15.000] Still waiting...
    ⏰ [03:48:19.000] Still waiting...
    ❌ [03:48:24.000] TIMEOUT after 10000ms

   Button Found:
     - HTML: <button type="submit" class="btn-primary">Save</button>
     - Attribute disabled: MISSING (not disabled!)
     - Aria-disabled: false
     - Computed CSS: pointer-events: auto (not disabled)

   Form State:
     - Validation errors: YES (email invalid)
     - Button should disable: YES (by test logic)
     - Button actually disabled: NO (bug!)

🔍 ROOT CAUSE:
   The form disables the button in HTML, but the CSS is hiding the error
   message and not calling setState to disable the button. This suggests:

   1. Form validation ran on backend (API returned 422)
   2. Error wasn't set in React state
   3. Button didn't re-render as disabled

   LIKELY CODE BUG:
   - Error response not processed in catch/error handler
   - setFormErrors() not called
   - Button disable logic checks form.state.errors but it's empty
```

**How to Fix**:
1. Check the `Account.tsx` form submission error handler
2. Ensure API errors update form state: `setFormErrors(response.errors)`
3. Ensure button disable logic: `disabled={Object.keys(formErrors).length > 0}`
4. Verify error message shows: `{formErrors.email && <p>{formErrors.email}</p>}`

---

## Interpreting the Report Summary

After tests complete, you'll see:

```
⏱️  Slow Tests (>5s):
────────────────────────────────────────────────────────────
1. test name [16.25s] ← Takes 16+ seconds to run/timeout
2. test name [12.61s] ← Long test setup or many operations
...

🔍 Failure Analysis by Type:
────────────────────────────────────────────────────────────
timeout      │ ████░░░░░░░░░░░░░░░░ 4/11 (36%)
             │ Action: Add waits, increase timeouts
             │
assertion    │ ███░░░░░░░░░░░░░░░░░ 3/11 (27%)
             │ Action: Check component state logic
             │
locator      │ ██░░░░░░░░░░░░░░░░░░ 2/11 (18%)
             │ Action: Make selectors more specific
             │
other        │ ██░░░░░░░░░░░░░░░░░░ 2/11 (18%)
             │ Action: Review trace for error details
```

**What this tells you**:
- **36% Timeout**: Network is slow or test expectations unrealistic
- **27% Assertion**: Component behavior wrong (disable logic, form state, etc.)
- **18% Locator**: Selector strategy needs improvement
- **18% Other**: Exceptions or edge cases (need to investigate individually)

---

## Next Steps When Tests Complete

1. **Run the tests**: Already in progress ✅
2. **Open the report**: `npx playwright show-report`
3. **For each failure**:
   - Click test name
   - Read the assertion error
   - Check the console logs (our debug output)
   - Inspect the trace timeline
   - Watch the video
4. **Categorize the failure**: Timeout? Assertion? Locator? Network?
5. **Apply the appropriate fix** based on the category
6. **Re-run just that test**: `npx playwright test --grep "test name"`
7. **Validate**: Confirm test now passes

The debugging infrastructure gives you everything you need to understand exactly why each test failed and what to fix.
