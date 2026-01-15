# Coverage Gap Analysis Report

**Date:** December 23, 2025
**PR:** feature/beta-release
**Overall Patch Coverage:** 34.84848%
**Missing Lines:** 43

## Executive Summary

This report analyzes the coverage gaps in 4 frontend files that were modified in the feature/beta-release PR. The primary issues are:

1. **Newly added functions lack tests** - `validatePublicURL()` and `testPublicURL()` in settings API
2. **New API function untested** - `previewInviteURL()` in users API
3. **New UI features untested** - Public URL validation/testing UI in SystemSettings component
4. **Modal interactions partially covered** - URL preview and permission update flows in UsersPage

---

## File-by-File Analysis

### 1. frontend/src/pages/SystemSettings.tsx

**Patch Coverage:** 25% (23 lines missing, 7 partial)
**Existing Test File:** `/projects/Charon/frontend/src/pages/__tests__/SystemSettings.test.tsx`

#### Untested Code Paths

##### A. Public URL Validation Logic (Lines ~79-94)

```typescript
const validatePublicURL = async (url: string) => {
  if (!url) {
    setPublicURLValid(null)
    return
  }
  try {
    const response = await client.post('/settings/validate-url', { url })
    setPublicURLValid(response.data.valid)
  } catch {
    setPublicURLValid(false)
  }
}

// Debounce validation
useEffect(() => {
  const timer = setTimeout(() => {
    if (publicURL) {
      validatePublicURL(publicURL)
    }
  }, 300)
  return () => clearTimeout(timer)
}, [publicURL])
```

**Missing Coverage:**

- Empty URL handling
- Successful validation response
- Failed validation (catch block)
- Debounce timer logic
- Cleanup of debounce timer

##### B. Test Public URL Handler (Lines ~113-131)

```typescript
const testPublicURLHandler = async () => {
  if (!publicURL) {
    toast.error(t('systemSettings.applicationUrl.invalidUrl'))
    return
  }
  setPublicURLSaving(true)
  try {
    const result = await testPublicURL(publicURL)
    if (result.reachable) {
      toast.success(
        result.message || `URL reachable (${result.latency?.toFixed(0)}ms)`
      )
    } else {
      toast.error(result.error || 'URL not reachable')
    }
  } catch (error) {
    toast.error(error instanceof Error ? error.message : 'Test failed')
  } finally {
    setPublicURLSaving(false)
  }
}
```

**Missing Coverage:**

- Empty URL validation
- Successful URL test with latency
- Successful URL test without message
- Unreachable URL handling
- Error handling in catch block
- Loading state management

##### C. Application URL Card JSX (Lines ~372-425)

```tsx
<Card>
  <CardHeader>
    <CardTitle>{t('systemSettings.applicationUrl.title')}</CardTitle>
    <CardDescription>{t('systemSettings.applicationUrl.description')}</CardDescription>
  </CardHeader>
  <CardContent className="space-y-4">
    <Alert variant="info">...</Alert>
    <div className="space-y-2">
      <Label htmlFor="public-url">...</Label>
      <div className="flex gap-2">
        <Input
          id="public-url"
          type="url"
          value={publicURL}
          onChange={(e) => { setPublicURL(e.target.value) }}
          placeholder="https://charon.example.com"
          className={cn(
            publicURLValid === false && 'border-red-500',
            publicURLValid === true && 'border-green-500'
          )}
        />
        {publicURLValid !== null && (
          publicURLValid ? (
            <CheckCircle2 className="h-5 w-5 text-green-500 self-center flex-shrink-0" />
          ) : (
            <XCircle className="h-5 w-5 text-red-500 self-center flex-shrink-0" />
          )
        )}
      </div>
      {publicURLValid === false && (
        <p className="text-sm text-red-500">
          {t('systemSettings.applicationUrl.invalidUrl')}
        </p>
      )}
    </div>
    {!publicURL && (
      <Alert variant="warning">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription>
          {t('systemSettings.applicationUrl.notConfiguredWarning')}
        </AlertDescription>
      </Alert>
    )}
    <div className="flex gap-2">
      <Button
        variant="secondary"
        onClick={testPublicURLHandler}
        disabled={!publicURL || publicURLSaving}
      >
        <ExternalLink className="h-4 w-4 mr-2" />
        {t('systemSettings.applicationUrl.testButton')}
      </Button>
    </div>
  </CardContent>
  <CardFooter className="justify-end">...</CardFooter>
</Card>
```

**Missing Coverage:**

- Rendering of Application URL card
- Info alert display
- Public URL input field
- Validation icon display (CheckCircle2/XCircle)
- Border color changes based on validation state
- Invalid URL error message display
- Empty URL warning alert
- Test button rendering
- Test button disabled states
- Save button in card footer

#### Recommended Test Cases

1. **URL Validation Tests:**
   - Test debounced validation triggers after 300ms
   - Test validation with valid URL shows green checkmark
   - Test validation with invalid URL shows red X
   - Test empty URL clears validation state
   - Test validation API error handling

2. **URL Test Button Tests:**
   - Test clicking test button with valid URL
   - Test button disabled when URL is empty
   - Test button disabled during test (loading state)
   - Test successful URL test shows success toast with latency
   - Test unreachable URL shows error toast
   - Test error during URL test shows error toast

3. **Visual State Tests:**
   - Test input border turns green on valid URL
   - Test input border turns red on invalid URL
   - Test CheckCircle2 icon appears on valid URL
   - Test XCircle icon appears on invalid URL
   - Test invalid URL error message appears
   - Test warning alert appears when URL is empty

4. **Integration Tests:**
   - Test saving settings includes public URL
   - Test URL validation happens on input change
   - Test debounce timer cleanup on component unmount

**Priority:** HIGH - Core new feature completely untested

---

### 2. frontend/src/pages/UsersPage.tsx

**Patch Coverage:** 50% (6 lines missing, 1 partial)
**Existing Test File:** `/projects/Charon/frontend/src/pages/__tests__/UsersPage.test.tsx`

#### Untested Code Paths

##### A. URL Preview in InviteModal (Lines ~63-78)

```typescript
// Fetch preview when email changes
useEffect(() => {
  if (email && email.includes('@')) {
    const fetchPreview = async () => {
      try {
        const response = await client.post('/users/preview-invite-url', { email })
        setUrlPreview(response.data)
      } catch {
        setUrlPreview(null)
      }
    }
    const debounce = setTimeout(fetchPreview, 500)
    return () => clearTimeout(debounce)
  } else {
    setUrlPreview(null)
  }
}, [email])
```

**Missing Coverage:**

- URL preview API call trigger
- Successful preview response handling
- Error handling in preview fetch
- Debounce cleanup
- Email validation check

##### B. URL Preview Display in Modal (Lines ~257-275)

```tsx
{/* URL Preview */}
{urlPreview && (
  <div className="space-y-2 p-4 bg-gray-900/50 rounded-lg border border-gray-700">
    <div className="flex items-center gap-2">
      <ExternalLink className="h-4 w-4 text-gray-400" />
      <Label className="text-sm font-medium text-gray-300">
        {t('users.inviteUrlPreview')}
      </Label>
    </div>
    <div className="text-sm font-mono text-gray-400 break-all bg-gray-950 p-2 rounded">
      {urlPreview.preview_url.replace('SAMPLE_TOKEN_PREVIEW', '...')}
    </div>
    {urlPreview.warning && (
      <Alert variant="warning" className="mt-2">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription className="text-xs">
          {t('users.inviteUrlWarning')}
          <Link to="/settings/system" className="ml-1 underline">
            {t('users.configureApplicationUrl')}
          </Link>
        </AlertDescription>
      </Alert>
    )}
  </div>
)}
```

**Missing Coverage:**

- URL preview section rendering
- Preview URL display with token replacement
- Warning alert display when warning is true
- Link to system settings in warning

##### C. PermissionsModal State Update (Lines ~290-295)

```typescript
// Update state when user changes
useState(() => {
  if (user) {
    setPermissionMode(user.permission_mode || 'allow_all')
    setSelectedHosts(user.permitted_hosts || [])
  }
})
```

**Missing Coverage:**

- State initialization when user prop changes
- Default values when user data is incomplete

#### Recommended Test Cases

1. **URL Preview Tests:**
   - Test URL preview appears when typing valid email
   - Test URL preview debounce (500ms delay)
   - Test URL preview displays with token replacement
   - Test URL preview warning appears when `warning: true`
   - Test URL preview warning link to system settings
   - Test URL preview error handling
   - Test URL preview clears on invalid email

2. **PermissionsModal Initialization:**
   - Test modal initializes with user's current permission_mode
   - Test modal initializes with user's current permitted_hosts
   - Test modal handles missing permitted_hosts gracefully
   - Test modal updates when different user is selected

**Priority:** MEDIUM - Partial coverage exists, new UI features untested

---

### 3. frontend/src/api/settings.ts

**Patch Coverage:** 33.33% (4 lines missing)
**Existing Test File:** `/projects/Charon/frontend/src/api/__tests__/settings.test.ts`

#### Untested Functions

##### A. validatePublicURL (Lines ~26-35)

```typescript
export const validatePublicURL = async (url: string): Promise<{
  valid: boolean
  normalized?: string
  error?: string
}> => {
  const response = await client.post('/settings/validate-url', { url })
  return response.data
}
```

**Missing Coverage:**

- Function not tested at all
- POST request to `/settings/validate-url`
- Request payload with url parameter
- Response data structure

##### B. testPublicURL (Lines ~37-48)

```typescript
export const testPublicURL = async (url: string): Promise<{
  reachable: boolean
  latency?: number
  message?: string
  error?: string
}> => {
  const response = await client.post('/settings/test-url', { url })
  return response.data
}
```

**Missing Coverage:**

- Function not tested at all
- POST request to `/settings/test-url`
- Request payload with url parameter
- Response data structure with reachable/latency/message/error

#### Recommended Test Cases

1. **validatePublicURL Tests:**
   - Test calls POST /settings/validate-url with correct URL
   - Test returns valid: true for valid URL
   - Test returns valid: false for invalid URL
   - Test returns normalized URL when provided
   - Test returns error message when validation fails

2. **testPublicURL Tests:**
   - Test calls POST /settings/test-url with correct URL
   - Test returns reachable: true with latency for successful test
   - Test returns reachable: false with error for failed test
   - Test returns message field when provided
   - Test error handling when request fails

**Priority:** HIGH - New public API functions completely untested

---

### 4. frontend/src/api/users.ts

**Patch Coverage:** 33.33% (2 lines missing)
**Existing Test File:** `/projects/Charon/frontend/src/api/__tests__/users.test.ts`

#### Untested Function

##### previewInviteURL (Lines ~115-128)

```typescript
export interface PreviewInviteURLResponse {
  preview_url: string
  base_url: string
  is_configured: boolean
  email: string
  warning: boolean
  warning_message: string
}

export const previewInviteURL = async (email: string): Promise<PreviewInviteURLResponse> => {
  const response = await client.post<PreviewInviteURLResponse>('/users/preview-invite-url', { email })
  return response.data
}
```

**Missing Coverage:**

- Function not tested at all
- POST request to `/users/preview-invite-url`
- Request payload with email parameter
- Response data structure validation

#### Recommended Test Cases

1. **previewInviteURL Tests:**
   - Test calls POST /users/preview-invite-url with email
   - Test returns complete PreviewInviteURLResponse structure
   - Test response includes preview_url with sample token
   - Test response includes base_url
   - Test response includes is_configured flag
   - Test response includes email
   - Test response includes warning flag
   - Test response includes warning_message
   - Test error handling when request fails

**Priority:** MEDIUM - Simple API function, but part of new feature

---

## Test Type Recommendations

### Unit Tests (Immediate Priority)

1. **API Layer Tests** (High Priority):
   - `frontend/src/api/__tests__/settings.test.ts` - Add tests for `validatePublicURL()` and `testPublicURL()`
   - `frontend/src/api/__tests__/users.test.ts` - Add test for `previewInviteURL()`

   **Rationale:** These are pure API functions with no UI dependencies, easy to test, and foundational for other features.

### Component Tests (High Priority)

1. **SystemSettings Component Tests**:
   - File: `frontend/src/pages/__tests__/SystemSettings.test.tsx`
   - Add comprehensive tests for Application URL card
   - Test URL validation UI states
   - Test URL test button behavior
   - Test debounced validation
   - Mock `validatePublicURL()` and `testPublicURL()` API calls

2. **UsersPage Component Tests**:
   - File: `frontend/src/pages/__tests__/UsersPage.test.tsx`
   - Add tests for URL preview in InviteModal
   - Test preview debouncing
   - Test warning display logic
   - Mock `previewInviteURL()` API call

### Integration Tests (Lower Priority)

1. **End-to-End Scenarios** (if E2E framework exists):
   - Full user invite flow with URL preview
   - Public URL configuration and testing workflow
   - Permission updates with modal interactions

---

## Priority Order for Addressing Gaps

### Phase 1: Critical API Tests (Estimated: 1-2 hours)

1. Add `validatePublicURL()` tests in `settings.test.ts`
2. Add `testPublicURL()` tests in `settings.test.ts`
3. Add `previewInviteURL()` tests in `users.test.ts`

**Expected Coverage Gain:** ~10-12 lines

### Phase 2: Core Component Tests (Estimated: 3-4 hours)

1. Add Application URL card tests in `SystemSettings.test.tsx`
   - URL validation UI tests (8-10 test cases)
   - URL test button tests (5-6 test cases)
2. Add URL preview tests in `UsersPage.test.tsx`
   - Preview display tests (4-5 test cases)
   - Debouncing tests (2-3 test cases)

**Expected Coverage Gain:** ~20-25 lines

### Phase 3: Edge Cases and Integration (Estimated: 2-3 hours)

1. Add edge case tests for error handling
2. Add integration tests for debouncing behavior
3. Add visual state tests for validation icons
4. Add PermissionsModal initialization tests

**Expected Coverage Gain:** ~8-10 lines

---

## Test Implementation Notes

### Mocking Requirements

1. **For SystemSettings tests:**

   ```typescript
   vi.mock('../../api/settings', () => ({
     getSettings: vi.fn(),
     updateSetting: vi.fn(),
     validatePublicURL: vi.fn(), // NEW
     testPublicURL: vi.fn(), // NEW
   }))
   ```

2. **For UsersPage tests:**

   ```typescript
   vi.mock('../../api/users', () => ({
     // ... existing mocks
     previewInviteURL: vi.fn(), // NEW
   }))
   ```

3. **For API tests:**

   ```typescript
   vi.mock('../client', () => ({
     default: {
       get: vi.fn(),
       post: vi.fn(),
       put: vi.fn(),
       delete: vi.fn(),
     },
   }))
   ```

### Test Data Fixtures

1. **Valid URL validation response:**

   ```typescript
   const mockValidationSuccess = {
     valid: true,
     normalized: 'https://example.com'
   }
   ```

2. **URL test response:**

   ```typescript
   const mockTestSuccess = {
     reachable: true,
     latency: 42,
     message: 'URL is reachable'
   }
   ```

3. **URL preview response:**

   ```typescript
   const mockPreview = {
     preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
     base_url: 'https://example.com',
     is_configured: true,
     email: 'test@example.com',
     warning: false,
     warning_message: ''
   }
   ```

### Debouncing Test Pattern

```typescript
it('debounces URL validation for 300ms', async () => {
  vi.useFakeTimers()
  renderWithProviders(<SystemSettings />)

  const input = screen.getByPlaceholderText('https://charon.example.com')
  await userEvent.type(input, 'https://example.com')

  // Should not call immediately
  expect(settingsApi.validatePublicURL).not.toHaveBeenCalled()

  // Advance timers by 300ms
  vi.advanceTimersByTime(300)

  await waitFor(() => {
    expect(settingsApi.validatePublicURL).toHaveBeenCalledWith('https://example.com')
  })

  vi.useRealTimers()
})
```

---

## Coverage Target

**Current Patch Coverage:** 34.84848%
**Target Patch Coverage:** 80%+
**Lines to Cover:** ~34 additional lines (out of 43 missing)

**Estimated Total Effort:** 6-9 hours of test writing

---

## Existing Test Infrastructure

### Test Utilities Available

1. **Query Client Provider:** `renderWithQueryClient()` utility exists
2. **User Event:** `@testing-library/user-event` configured
3. **Toast Mocking:** Toast utility already mocked in tests
4. **i18n Mocking:** Global mock for react-i18next in `src/test/setup.ts`
5. **API Client Mocking:** Pattern established in existing tests

### Test Files Structure

```
frontend/src/
├── api/
│   └── __tests__/
│       ├── settings.test.ts (EXISTS - needs expansion)
│       └── users.test.ts (EXISTS - needs expansion)
└── pages/
    └── __tests__/
        ├── SystemSettings.test.tsx (EXISTS - needs expansion)
        └── UsersPage.test.tsx (EXISTS - needs expansion)
```

---

## Conclusion

The coverage gaps are concentrated in newly added features:

1. **Public URL validation/testing** - Brand new feature, no tests
2. **Invite URL preview** - New enhancement, minimal tests
3. **API functions** - New exports, completely untested

All gaps can be addressed by expanding existing test files. No new test infrastructure is needed. The recommended phased approach will systematically bring patch coverage from 34.85% to 80%+ while ensuring critical API functions are tested first.

**Next Step:** Proceed with Phase 1 (API tests) to establish foundational coverage before tackling component tests.
