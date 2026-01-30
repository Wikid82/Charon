# PR #460: Frontend DNS Provider Coverage Plan

## Overview

Add comprehensive test coverage for DNS provider feature to achieve 85%+ coverage threshold.

## Files Requiring Tests

### 1. `frontend/src/api/dnsProviders.ts`

**Status:** No existing tests
**Target Coverage:** 85%+

**Test Cases:**

- `getDNSProviders()` - Fetch all providers list
  - Successful response with providers array
  - Empty providers list
  - Error handling (network, 500, etc.)
- `getDNSProvider(id)` - Fetch single provider
  - Valid provider ID returns provider
  - Invalid ID (404 error)
  - Error handling
- `getDNSProviderTypes()` - Fetch supported types
  - Returns types array with field definitions
  - Error handling
- `createDNSProvider(data)` - Create new provider
  - Successful creation returns provider with ID
  - Validation errors (missing fields, invalid type)
  - Duplicate name error
  - Error handling
- `updateDNSProvider(id, data)` - Update existing
  - Successful update returns updated provider
  - Not found (404)
  - Validation errors
  - Error handling
- `deleteDNSProvider(id)` - Delete provider
  - Successful deletion (204)
  - Not found (404)
  - In-use error (409 - used by proxy hosts)
  - Error handling
- `testDNSProvider(id)` - Test saved provider
  - Success result with propagation time
  - Failure result with error message
  - Not found (404)
  - Error handling
- `testDNSProviderCredentials(data)` - Test before saving
  - Valid credentials success
  - Invalid credentials failure
  - Validation errors
  - Error handling

**File:** `frontend/src/api/__tests__/dnsProviders.test.ts`

---

### 2. `frontend/src/hooks/useDNSProviders.ts`

**Status:** No existing tests
**Target Coverage:** 85%+

**Test Cases:**

- `useDNSProviders()` hook
  - Returns providers list on mount
  - Loading state during fetch
  - Error state on failure
  - Query key consistency
- `useDNSProvider(id)` hook
  - Fetches single provider when id > 0
  - Disabled when id = 0
  - Disabled when id < 0
  - Loading and error states
- `useDNSProviderTypes()` hook
  - Fetches types list
  - Applies staleTime (1 hour)
  - Loading and error states
- `useDNSProviderMutations()` hook
  - `createMutation` - Creates provider
    - Invalidates list query on success
    - Handles errors
  - `updateMutation` - Updates provider
    - Invalidates list and detail queries on success
    - Handles errors
  - `deleteMutation` - Deletes provider
    - Invalidates list query on success
    - Handles errors
  - `testMutation` - Tests provider
    - Returns test result
    - Handles errors
  - `testCredentialsMutation` - Tests credentials
    - Returns test result
    - Handles errors

**File:** `frontend/src/hooks/__tests__/useDNSProviders.test.tsx`

---

### 3. `frontend/src/components/DNSProviderSelector.tsx`

**Status:** No existing tests
**Target Coverage:** 85%+

**Test Cases:**

- Component rendering
  - Renders with label when provided
  - Renders without label
  - Shows required asterisk when required=true
  - Shows helper text when provided
  - Shows error message when provided (replaces helper text)
- Provider filtering
  - Only shows enabled providers
  - Only shows providers with credentials
  - Filters out disabled providers
  - Filters out providers without credentials
- Loading states
  - Shows loading option while fetching
  - Disables select during loading
- Empty states
  - Shows "no providers available" when list is empty
  - Shows "no providers available" when all filtered out
- Selection behavior
  - Displays selected provider by ID
  - Shows "none" option when not required
  - Hides "none" option when required=true
  - Calls onChange with provider ID on selection
  - Calls onChange with undefined when "none" selected
- Provider display
  - Shows provider name
  - Shows default star icon for default provider
  - Shows provider type in parentheses
  - Translates provider type labels
- Disabled state
  - Disables select when disabled=true
  - Disables select during loading
- Accessibility
  - Error has role="alert"
  - Label properly associates with select

**File:** `frontend/src/components/__tests__/DNSProviderSelector.test.tsx`

---

### 4. `frontend/src/components/ProxyHostForm.tsx`

**Status:** Partial tests exist, DNS provider integration NOT covered
**Target Coverage:** Add DNS-specific tests to existing suite

**Test Cases to Add:**

- Wildcard domain detection
  - Detects `*.example.com` as wildcard
  - Does not detect `sub.example.com` as wildcard
  - Detects multiple wildcards in comma-separated list
- DNS provider requirement for wildcards
  - Shows DNS provider selector when wildcard domain entered
  - Shows info alert explaining DNS-01 requirement
  - Shows validation error on submit if wildcard without provider
  - Does not show DNS provider selector without wildcard
- DNS provider selection
  - Selecting DNS provider updates form state
  - Clears DNS provider when switching to non-wildcard
  - Preserves DNS provider selection during form edits
- Form submission with DNS provider
  - Includes `dns_provider_id` in payload
  - Sends null when no provider selected
  - Sends provider ID when selected

**File:** `frontend/src/components/__tests__/ProxyHostForm-dns.test.tsx` (new file for DNS-specific tests)

---

## Implementation Order

1. **API Layer** (`dnsProviders.test.ts`) - Foundation for all other tests
2. **Hooks Layer** (`useDNSProviders.test.tsx`) - Depends on API mocks
3. **Selector Component** (`DNSProviderSelector.test.tsx`) - Depends on hooks
4. **Integration** (`ProxyHostForm-dns.test.tsx`) - Tests full flow

## Testing Strategy

- Use MSW (Mock Service Worker) for API mocking
- Follow existing patterns in `useProxyHosts.test.tsx` and `ProxyHostForm.test.tsx`
- Use React Testing Library for component tests
- Use `@tanstack/react-query` test utilities for hook tests
- Mock i18n translations
- Test both success and error paths
- Verify query invalidation on mutations

## Coverage Target

**Overall Goal:** 85%+ coverage for all four files

- Statements: ≥85%
- Branches: ≥85%
- Functions: ≥85%
- Lines: ≥85%

## Dependencies

- Existing test setup in `frontend/src/test/setup.ts`
- MSW handlers pattern from existing tests
- Mock data factories for DNS providers
- Translation mocks

## Validation

Run coverage after implementation:

```bash
npm test -- --coverage --collectCoverageFrom='src/api/dnsProviders.ts' --collectCoverageFrom='src/hooks/useDNSProviders.ts' --collectCoverageFrom='src/components/DNSProviderSelector.tsx' --collectCoverageFrom='src/components/ProxyHostForm.tsx'
```

---

**Completion Criteria:**

- [ ] All four test files created
- [ ] All test cases implemented
- [ ] Coverage report shows ≥85% for all metrics
- [ ] All tests passing
- [ ] No console errors or warnings during test execution
