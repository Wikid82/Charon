# SecurityHeaders Page Rendering Issue - Root Cause Analysis

**Date:** December 18, 2025
**Issue:** SecurityHeaders page is not rendering
**Analysis Type:** Complete Workflow Trace (NO CODE CHANGES)

---

## Executive Summary

**ROOT CAUSE IDENTIFIED:** Backend-Frontend API Response Format Mismatch

The backend returns security header profiles wrapped in an object with a `profiles` key:

```json
{ "profiles": [...] }
```

But the frontend expects a raw array:

```typescript
Promise<SecurityHeaderProfile[]>
```

This causes the frontend React Query hook to fail silently, preventing the page from rendering the data correctly.

---

## Complete File Workflow Map

### 1. Frontend Component Layer

**File:** [frontend/src/pages/SecurityHeaders.tsx](../frontend/src/pages/SecurityHeaders.tsx)

**Role:** Main page component that displays security header profiles

**Key Operations:**

- Uses `useSecurityHeaderProfiles()` hook to fetch profiles
- Expects `data: SecurityHeaderProfile[] | undefined`
- Filters profiles into `customProfiles` and `presetProfiles`
- Renders cards for each profile

**Critical Line:**

```typescript
const { data: profiles, isLoading } = useSecurityHeaderProfiles();
// Expects profiles to be SecurityHeaderProfile[] or undefined
```

---

### 2. Frontend Hooks Layer

**File:** [frontend/src/hooks/useSecurityHeaders.ts](../frontend/src/hooks/useSecurityHeaders.ts)

**Role:** React Query wrapper for security headers API

**Key Operations:**

```typescript
export function useSecurityHeaderProfiles() {
  return useQuery({
    queryKey: ['securityHeaderProfiles'],
    queryFn: securityHeadersApi.listProfiles,  // ← Expects array return
  });
}
```

**Expected Return Type:** `Promise<SecurityHeaderProfile[]>`

---

### 3. Frontend API Layer

**File:** [frontend/src/api/securityHeaders.ts](../frontend/src/api/securityHeaders.ts)

**Role:** Type-safe API client for security headers endpoints

**Key Operations:**

```typescript
async listProfiles(): Promise<SecurityHeaderProfile[]> {
  const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
  return response.data;  // ← Expects response.data to be an array
}
```

**API Endpoint:** `GET /api/v1/security/headers/profiles`

**Expected Response Format:** Direct array `SecurityHeaderProfile[]`

---

### 4. Frontend HTTP Client

**File:** [frontend/src/api/client.ts](../frontend/src/api/client.ts)

**Role:** Axios instance with base configuration

**Configuration:**

- Base URL: `/api/v1`
- With credentials: `true` (for cookies)
- Timeout: 30 seconds
- 401 interceptor for auth errors

**No issues here** - properly configured.

---

### 5. Backend Route Registration

**File:** [backend/internal/api/routes/routes.go](../backend/internal/api/routes/routes.go)

**Role:** Registers all API routes including security headers

**Lines 421-423:**

```go
// Security Headers
securityHeadersHandler := handlers.NewSecurityHeadersHandler(db, caddyManager)
securityHeadersHandler.RegisterRoutes(protected)
```

**Routes Registered:**

- `GET /api/v1/security/headers/profiles` → `ListProfiles`
- `GET /api/v1/security/headers/profiles/:id` → `GetProfile`
- `POST /api/v1/security/headers/profiles` → `CreateProfile`
- `PUT /api/v1/security/headers/profiles/:id` → `UpdateProfile`
- `DELETE /api/v1/security/headers/profiles/:id` → `DeleteProfile`
- `GET /api/v1/security/headers/presets` → `GetPresets`
- `POST /api/v1/security/headers/presets/apply` → `ApplyPreset`
- `POST /api/v1/security/headers/score` → `CalculateScore`
- `POST /api/v1/security/headers/csp/validate` → `ValidateCSP`
- `POST /api/v1/security/headers/csp/build` → `BuildCSP`

**Migration:** Line 48 includes `&models.SecurityHeaderProfile{}` in AutoMigrate list ✓

---

### 6. Backend Handler

**File:** [backend/internal/api/handlers/security_headers_handler.go](../backend/internal/api/handlers/security_headers_handler.go)

**Role:** HTTP handlers for security headers endpoints

**ListProfiles Handler (Lines 54-62):**

```go
func (h *SecurityHeadersHandler) ListProfiles(c *gin.Context) {
 var profiles []models.SecurityHeaderProfile
 if err := h.db.Order("is_preset DESC, name ASC").Find(&profiles).Error; err != nil {
  c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  return
 }
 c.JSON(http.StatusOK, gin.H{"profiles": profiles})  // ← WRAPPED IN OBJECT!
}
```

**ACTUAL Response Format:**

```json
{
  "profiles": [
    { "id": 1, "name": "...", ... },
    { "id": 2, "name": "...", ... }
  ]
}
```

**GetPresets Handler (Lines 233-236):**

```go
func (h *SecurityHeadersHandler) GetPresets(c *gin.Context) {
 presets := h.service.GetPresets()
 c.JSON(http.StatusOK, gin.H{"presets": presets})  // ← ALSO WRAPPED!
}
```

**Other Handlers:**

- `GetProfile` returns: `gin.H{"profile": profile}` (wrapped)
- `CreateProfile` returns: `gin.H{"profile": req}` (wrapped)
- `UpdateProfile` returns: `gin.H{"profile": updates}` (wrapped)
- `ApplyPreset` returns: `gin.H{"profile": profile}` (wrapped)

**ALL ENDPOINTS WRAP RESPONSES IN OBJECTS!**

---

### 7. Backend Model

**File:** [backend/internal/models/security_header_profile.go](../backend/internal/models/security_header_profile.go)

**Role:** GORM database model for security header profiles

**Struct Definition:**

```go
type SecurityHeaderProfile struct {
 ID   uint   `json:"id" gorm:"primaryKey"`
 UUID string `json:"uuid" gorm:"uniqueIndex;not null"`
 Name string `json:"name" gorm:"index;not null"`

 // HSTS Configuration
 HSTSEnabled           bool `json:"hsts_enabled" gorm:"default:true"`
 HSTSMaxAge            int  `json:"hsts_max_age" gorm:"default:31536000"`
 HSTSIncludeSubdomains bool `json:"hsts_include_subdomains" gorm:"default:true"`
 HSTSPreload           bool `json:"hsts_preload" gorm:"default:false"`

 // ... (25+ more fields)

 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
```

**JSON Tags:** ✓ All fields have proper `json:"snake_case"` tags
**GORM Tags:** ✓ All fields have proper GORM configuration
**No issues in the model layer**

---

## Data Flow Diagram (Text-Based)

```
┌─────────────────────────────────────────────────────────────────┐
│ USER OPENS /security/headers                                     │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ SecurityHeaders.tsx Component                                    │
│ - Calls: useSecurityHeaderProfiles()                            │
│ - Expects: SecurityHeaderProfile[] | undefined                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ useSecurityHeaders Hook                                          │
│ - React Query: queryFn: securityHeadersApi.listProfiles         │
│ - Returns: Promise<SecurityHeaderProfile[]>                     │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ securityHeaders.ts API Layer                                     │
│ - client.get<SecurityHeaderProfile[]>('/security/headers/...')  │
│ - Returns: response.data  ← Expects array!                      │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼ HTTP GET /api/v1/security/headers/profiles
┌─────────────────────────────────────────────────────────────────┐
│ client.ts (Axios)                                                │
│ - Base URL: /api/v1                                             │
│ - Makes request with auth cookies                               │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ Backend: routes.go                                               │
│ - Route: GET /security/headers/profiles                         │
│ - Handler: securityHeadersHandler.ListProfiles                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ security_headers_handler.go                                      │
│ - Query: db.Order(...).Find(&profiles)                          │
│ - Response: gin.H{"profiles": profiles}  ← OBJECT WRAPPER!     │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼ Returns: {"profiles": [...]}
┌─────────────────────────────────────────────────────────────────┐
│ Frontend receives:                                               │
│ {                                                                │
│   "profiles": [                                                  │
│     { id: 1, name: "...", ... }                                 │
│   ]                                                              │
│ }                                                                │
│                                                                  │
│ BUT TypeScript expects:                                          │
│ [                                                                │
│   { id: 1, name: "...", ... }                                   │
│ ]                                                                │
│                                                                  │
│ TYPE MISMATCH → React Query fails to parse → Page shows empty   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Identified Logic Gaps

### **GAP #1: Response Format Mismatch** ⚠️ **CRITICAL**

**Location:** Backend handlers vs Frontend API layer

**Backend Returns:**

```json
{
  "profiles": [...]
}
```

**Frontend Expects:**

```json
[...]
```

**Impact:**

- React Query receives an object when it expects an array
- `response.data` in `securityHeadersApi.listProfiles()` contains `{ profiles: [...] }`, not `[...]`
- This causes the component to receive `undefined` instead of an array
- Page cannot render profile data

**Affected Endpoints:**

1. `GET /security/headers/profiles` → Returns `gin.H{"profiles": profiles}`
2. `GET /security/headers/presets` → Returns `gin.H{"presets": presets}`
3. `GET /security/headers/profiles/:id` → Returns `gin.H{"profile": profile}`
4. `POST /security/headers/profiles` → Returns `gin.H{"profile": req}`
5. `PUT /security/headers/profiles/:id` → Returns `gin.H{"profile": updates}`
6. `POST /security/headers/presets/apply` → Returns `gin.H{"profile": profile}`

---

### **GAP #2: Inconsistent API Pattern**

**Issue:** This endpoint doesn't follow the project's established pattern

**Evidence from other handlers:**

Looking at the test file `security_headers_handler_test.go` line 58-62:

```go
var response map[string][]models.SecurityHeaderProfile
err := json.Unmarshal(w.Body.Bytes(), &response)
assert.NoError(t, err)
assert.Len(t, response["profiles"], 2)  // ← Test expects wrapped format
```

The backend tests are **written for the wrapped format**, which means this was intentional. However, this creates an inconsistency with the frontend expectations.

**Comparison with other endpoints:**

Most endpoints in Charon follow this pattern in handlers, but the **frontend API layer typically unwraps them**. For example, if we look at other API calls, they might do:

```typescript
const response = await client.get<{profiles: SecurityHeaderProfile[]}>('/endpoint');
return response.data.profiles;  // ← Unwrap the data
```

But `securityHeaders.ts` doesn't unwrap:

```typescript
const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
return response.data;  // ← Missing unwrap!
```

---

### **GAP #3: Missing Error Visibility**

**Issue:** Silent failure in React Query

**Current Behavior:**

- When the type mismatch occurs, React Query doesn't throw a visible error
- The component receives `data: undefined`
- `isLoading` becomes `false`
- Page shows "No custom profiles yet" empty state even if profiles exist

**Expected Behavior:**

- Should show an error message
- Should log the type mismatch to console
- Should prevent silent failures

---

## Field Name Mapping Analysis

### Frontend Type Definition (`securityHeaders.ts`)

```typescript
export interface SecurityHeaderProfile {
  id: number;
  uuid: string;
  name: string;
  hsts_enabled: boolean;
  hsts_max_age: number;
  // ... (all snake_case)
}
```

### Backend Model (`security_header_profile.go`)

```go
type SecurityHeaderProfile struct {
 ID   uint   `json:"id" gorm:"primaryKey"`
 UUID string `json:"uuid" gorm:"uniqueIndex;not null"`
 Name string `json:"name" gorm:"index;not null"`
 HSTSEnabled           bool `json:"hsts_enabled" gorm:"default:true"`
 HSTSMaxAge            int  `json:"hsts_max_age" gorm:"default:31536000"`
 // ... (all have json:"snake_case" tags)
}
```

**✓ Field names match perfectly** - All Go struct fields have proper `json` tags in snake_case
**✓ No camelCase vs snake_case issues**
**✓ Type compatibility is correct** (uint→number, string→string, bool→boolean)

---

## Root Cause Hypothesis

### Primary Cause: **API Response Wrapper Inconsistency**

The SecurityHeaders feature fails to render because:

1. **Backend Design Decision:** All handlers return responses wrapped in objects with descriptive keys:

   ```go
   c.JSON(http.StatusOK, gin.H{"profiles": profiles})
   ```

2. **Frontend Assumption:** The API layer assumes direct array responses:

   ```typescript
   async listProfiles(): Promise<SecurityHeaderProfile[]> {
     const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
     return response.data;  // data is {profiles: [...]}, not [...]
   }
   ```

3. **Type System Failure:** TypeScript's type assertion doesn't prevent the runtime mismatch:
   - TypeScript says: "This is `SecurityHeaderProfile[]`"
   - Runtime reality: "This is `{profiles: SecurityHeaderProfile[]}`"
   - React Query receives wrong type → component gets `undefined`

4. **Silent Failure:** No console errors because:
   - HTTP request succeeds (200 OK)
   - JSON parsing succeeds
   - React Query doesn't validate response shape
   - Component defensively handles `undefined` with empty state

### Secondary Contributing Factors

- **No runtime type validation:** No schema validation (e.g., Zod) to catch mismatches
- **Inconsistent patterns:** Other parts of the codebase may handle this differently
- **Test coverage gap:** Frontend tests likely mock the API, missing the real mismatch

---

## Verification Steps Performed

### ✓ Checked Component Implementation

- SecurityHeaders.tsx exists and imports correct hooks
- Component structure is sound
- No syntax errors

### ✓ Checked Hooks Layer

- useSecurityHeaders.ts properly uses React Query
- Query keys are correct
- Mutation handlers are properly structured

### ✓ Checked API Layer

- securityHeaders.ts exists with all required functions
- Endpoints match backend routes
- **ISSUE FOUND:** Response type expectations don't match backend

### ✓ Checked Backend Handlers

- security_headers_handler.go exists with all required handlers
- Routes are properly registered in routes.go
- **ISSUE FOUND:** All responses are wrapped in objects

### ✓ Checked Database Models

- SecurityHeaderProfile model is complete
- All fields have proper JSON tags
- GORM configuration is correct
- Model is included in AutoMigrate

### ✓ Checked Route Registration

- Handler is instantiated in routes.go
- RegisterRoutes is called on the protected route group
- All security header routes are mounted correctly

---

## Fix Strategy (NOT IMPLEMENTED - ANALYSIS ONLY)

### Option 1: Update Frontend API Layer (RECOMMENDED)

**Change:** Unwrap responses in `frontend/src/api/securityHeaders.ts`

**Before:**

```typescript
async listProfiles(): Promise<SecurityHeaderProfile[]> {
  const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
  return response.data;
}
```

**After:**

```typescript
async listProfiles(): Promise<SecurityHeaderProfile[]> {
  const response = await client.get<{profiles: SecurityHeaderProfile[]}>('/security/headers/profiles');
  return response.data.profiles;  // ← Unwrap
}
```

**Pros:**

- Minimal changes (only update API layer)
- Maintains backend consistency
- No breaking changes for other consumers
- Tests remain valid

**Cons:**

- Frontend needs to know about backend wrapper keys

---

### Option 2: Update Backend Handlers (NOT RECOMMENDED)

**Change:** Return direct arrays from handlers

**Before:**

```go
c.JSON(http.StatusOK, gin.H{"profiles": profiles})
```

**After:**

```go
c.JSON(http.StatusOK, profiles)
```

**Pros:**

- Simpler response format
- Matches frontend expectations

**Cons:**

- Breaking change for any existing API consumers
- Breaks all existing backend tests
- Inconsistent with other endpoints
- May violate REST best practices (responses should be objects)

---

### Option 3: Add Runtime Type Validation (FUTURE IMPROVEMENT)

**Add:** Schema validation library (e.g., Zod)

```typescript
import { z } from 'zod';

const SecurityHeaderProfileSchema = z.object({
  id: z.number(),
  uuid: z.string(),
  name: z.string(),
  // ... rest of fields
});

const ListProfilesResponseSchema = z.object({
  profiles: z.array(SecurityHeaderProfileSchema),
});

async listProfiles(): Promise<SecurityHeaderProfile[]> {
  const response = await client.get('/security/headers/profiles');
  const validated = ListProfilesResponseSchema.parse(response.data);
  return validated.profiles;
}
```

**Pros:**

- Catches mismatches at runtime
- Provides clear error messages
- Documents expected shapes
- Prevents silent failures

**Cons:**

- Adds dependency
- Increases bundle size
- Requires maintenance of schemas

---

## Summary

### What Prevents the Page from Loading?

The SecurityHeaders page **does load**, but it shows **no data** because:

1. The component calls `useSecurityHeaderProfiles()`
2. The hook calls `securityHeadersApi.listProfiles()`
3. The API makes a successful HTTP request to `/api/v1/security/headers/profiles`
4. Backend returns: `{"profiles": [...]}`
5. Frontend expects: `[...]`
6. Type mismatch causes React Query to provide `undefined` to the component
7. Component renders empty state: "No custom profiles yet"

### The Fix (One-Line Change)

In `frontend/src/api/securityHeaders.ts`, change line 86-87:

```typescript
// Change from:
const response = await client.get<SecurityHeaderProfile[]>('/security/headers/profiles');
return response.data;

// To:
const response = await client.get<{profiles: SecurityHeaderProfile[]}>('/security/headers/profiles');
return response.data.profiles;
```

Apply similar changes to all other methods in the file that fetch wrapped responses.

---

## Files Requiring Updates (NOT IMPLEMENTED)

1. **`frontend/src/api/securityHeaders.ts`** - Unwrap all responses
   - `listProfiles()` - unwrap `.profiles`
   - `getProfile()` - unwrap `.profile`
   - `createProfile()` - unwrap `.profile`
   - `updateProfile()` - unwrap `.profile`
   - `getPresets()` - unwrap `.presets`
   - `applyPreset()` - unwrap `.profile`

2. **Frontend tests** - Update mocked responses to match wrapped format
   - `frontend/src/hooks/__tests__/useSecurityHeaders.test.tsx`
   - `frontend/src/api/__tests__/securityHeaders.test.ts` (if exists)

---

## Conclusion

The root cause is a **Backend-Frontend API contract mismatch**. The backend wraps all responses in objects with descriptive keys (following a common REST pattern), but the frontend API layer assumes direct array/object responses. This is easily fixed by updating the frontend to unwrap the responses at the API layer boundary.

The bug is not a logic error but an **integration contract violation** - both sides work correctly in isolation, but they don't agree on the data format at the boundary.

**Recommended Fix:** Option 1 - Update frontend API layer (5-10 lines changed)
**Estimated Time:** 5 minutes + testing
**Risk Level:** Low (isolated to API layer)

---

**Analysis completed:** December 18, 2025
**Analyst:** GitHub Copilot
**Next Steps:** Present findings to user for approval before implementing fix
