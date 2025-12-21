# Bug Investigation: Security Header Profile Not Persisting

**Created:** 2025-12-18
**Status:** Investigation Complete - Root Cause Identified

---

## Bug Report

Security header profile changes are not persisting to the database when editing proxy hosts.

**Observed Behavior:**

1. User assigns "Strict" profile to a proxy host → Saves successfully ✓
2. User edits the same host, changes to "Basic" profile → Appears to save ✓
3. User reopens the host edit form → Shows "Strict" (not "Basic") ❌

**The profile change is NOT persisting to the database.**

---

## Root Cause Analysis

### Investigation Summary

I examined the complete data flow from frontend form submission to backend database save. The code analysis reveals that **the implementation SHOULD work correctly**, but there are potential issues with value handling and silent error conditions.

### Frontend Code Analysis

**File:** [frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx)

**Lines 656-661:** Security header profile dropdown and onChange handler

```tsx
<select
  value={formData.security_header_profile_id || 0}
  onChange={e => {
    const value = parseInt(e.target.value) || null
    setFormData({ ...formData, security_header_profile_id: value })
  }}
>
```

**Issue #1: Falsy Coercion Bug**

The expression `parseInt(e.target.value) || null` has a problematic behavior:

- When user selects "None" (value="0"): `parseInt("0")` = 0, then `0 || null` = `null` ✓ (Correct - we want null for "None")
- When user selects profile ID 2: `parseInt("2")` = 2, then `2 || null` = 2 ✓ (Works)
- **BUT**: If `parseInt()` fails or returns `NaN`, the expression evaluates to `null` instead of preserving the current value

**Risk:** Edge cases where parseInt fails silently convert valid profile selections to `null`.

**Lines 308-311:** Form submission payload preparation

```tsx
const payload = { ...formData }
const { addUptime: _addUptime, uptimeInterval: _uptimeInterval, uptimeMaxRetries: _uptimeMaxRetries, ...payloadWithoutUptime } = payload as ProxyHostFormState
```

**Analysis:**

- The `security_header_profile_id` field is included in the spread operation
- If `formData.security_header_profile_id` is `undefined`, it won't be in the payload keys
- If it's `null` or a number, it WILL be included

### Backend Code Analysis

**File:** [backend/internal/api/handlers/proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go)

**Lines 231-248:** Security Header Profile update handling

```go
// Security Header Profile: update only if provided
if v, ok := payload["security_header_profile_id"]; ok {
    if v == nil {
        host.SecurityHeaderProfileID = nil
    } else {
        switch t := v.(type) {
        case float64:
            if id, ok := safeFloat64ToUint(t); ok {
                host.SecurityHeaderProfileID = &id
            }
            // ❌ NO ELSE CLAUSE - silently fails if conversion fails
        case int:
            if id, ok := safeIntToUint(t); ok {
                host.SecurityHeaderProfileID = &id
            }
            // ❌ NO ELSE CLAUSE
        case string:
            if n, err := strconv.ParseUint(t, 10, 32); err == nil {
                id := uint(n)
                host.SecurityHeaderProfileID = &id
            }
            // ❌ NO ELSE CLAUSE
        }
        // ❌ NO DEFAULT CASE - fails silently if type doesn't match
    }
}
```

**Issue #2: Silent Failure in Type Conversion**

If ANY of the following occur, the field is NOT updated:

1. `safeFloat64ToUint()` returns `ok = false`
2. `safeIntToUint()` returns `ok = false`
3. `strconv.ParseUint()` returns an error
4. The value type doesn't match `float64`, `int`, or `string`

**Critical:** No error is logged, no status code returned - the old value remains in memory and gets saved to the database.

**Lines 29-34:** The `safeFloat64ToUint` conversion function

```go
func safeFloat64ToUint(f float64) (uint, bool) {
    if f < 0 || f != float64(uint(f)) {
        return 0, false
    }
    return uint(f), true
}
```

**Analysis:**

- For negative numbers: Returns `false` ✓
- For integers (0, 1, 2, etc.): Returns `true` ✓
- For floats with decimals (2.5): Returns `false` (correct - can't convert to uint)

**This function should work fine for valid profile IDs.**

**Lines 256-258:** Calling the service to save

```go
if err := h.service.Update(host); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}
```

This calls the service `Update()` method which uses GORM's `Save()`.

### Service Layer Analysis

**File:** [backend/internal/services/proxyhost_service.go](../../backend/internal/services/proxyhost_service.go)

**Line 92:** The actual database save operation

```go
return s.db.Save(host).Error
```

**GORM's `Save()` method:**

- Updates ALL fields in the struct, including zero values
- Handles nullable pointers correctly (`*uint`)
- Should persist changes to `SecurityHeaderProfileID`

**No issues found here.**

### Model Analysis

**File:** [backend/internal/models/proxy_host.go](../../backend/internal/models/proxy_host.go)

**Lines 40-42:** Security Header Profile field definition

```go
SecurityHeaderProfileID *uint                  `json:"security_header_profile_id"`
SecurityHeaderProfile   *SecurityHeaderProfile `json:"security_header_profile" gorm:"foreignKey:SecurityHeaderProfileID"`
```

**Analysis:**

- Field is nullable pointer `*uint` ✓
- JSON tag is snake_case ✓
- GORM relationship configured ✓

**No issues found here.**

---

## Identified Root Causes

After comprehensive code review, I've identified **TWO potential root causes**:

### Root Cause #1: Frontend - Potential NaN Edge Case ⚠️

**Location:** [frontend/src/components/ProxyHostForm.tsx:658-661](../../frontend/src/components/ProxyHostForm.tsx)

**Problem:**

```tsx
const value = parseInt(e.target.value) || null
```

While this works for most cases, it has edge case vulnerabilities:

- If `e.target.value` is `""` (empty string): `parseInt("")` = `NaN`, then `NaN || null` = `null`
- If `parseInt()` somehow returns `0`: `0 || null` = `null` (converts valid 0 to null)

**Impact:** Low likelihood but would cause profile selection to silently become `null`.

### Root Cause #2: Backend - Silent Failure with No Logging ⚠️⚠️⚠️

**Location:** [backend/internal/api/handlers/proxy_host_handler.go:231-248](../../backend/internal/api/handlers/proxy_host_handler.go)

**Problem:**
No error handling or logging when type conversion fails:

```go
case float64:
    if id, ok := safeFloat64ToUint(t); ok {
        host.SecurityHeaderProfileID = &id
    }
    // If ok==false, nothing happens! Old value remains!
```

**Impact:** HIGH - If the payload value can't be converted (for any reason), the update is silently skipped.

**Why This Is The Likely Culprit:**

For the reported bug (changing from Strict to Basic), the frontend should send:

```json
{"security_header_profile_id": 2}
```

JSON numbers unmarshal as `float64` in Go. So `v` would be `float64(2.0)`.

The `safeFloat64ToUint(2.0)` call should return `(2, true)` and set the field correctly.

**UNLESS:**

1. The JSON payload is malformed
2. The value comes as a string `"2"` instead of number `2`
3. There's middleware modifying the payload
4. The `ok` check is somehow failing despite valid input

**The lack of logging makes this impossible to debug!**

---

## Why the Bug Occurs (Hypothesis)

Based on the code analysis, here's my hypothesis:

**Most Likely Scenario:**

1. Frontend sends `{"security_header_profile_id": 2}` as JSON number ✓
2. Backend receives it as `float64(2.0)` ✓
3. `safeFloat64ToUint(2.0)` returns `(2, true)` ✓
4. Sets `host.SecurityHeaderProfileID = &2` ✓
5. Calls `h.service.Update(host)` which runs `db.Save(host)` ✓
6. **BUT**: GORM's `Save()` might not be updating nullable pointer fields properly? ❌

**Alternative Scenario (Less Likely):**

The JSON payload is somehow getting stringified or modified before reaching the handler, causing the type assertion to fail.

**Evidence Needed:**

Without logs, we can't know which scenario is happening. The fix MUST include logging.

---

## Proposed Fix Plan

### Fix 1: Frontend - Explicit Value Handling (Safety Improvement)

**File:** `frontend/src/components/ProxyHostForm.tsx`
**Lines:** 658-661

**Change:**

```tsx
// BEFORE (risky):
onChange={e => {
  const value = parseInt(e.target.value) || null
  setFormData({ ...formData, security_header_profile_id: value })
}}

// AFTER (safe):
onChange={e => {
  const rawValue = e.target.value
  let value: number | null = null

  if (rawValue !== "0" && rawValue !== "") {
    const parsed = parseInt(rawValue, 10)
    value = isNaN(parsed) ? null : parsed
  }

  setFormData({ ...formData, security_header_profile_id: value })
}}
```

**Why:**

- Explicitly handles "0" case (None/null)
- Explicitly handles empty string
- Checks for NaN before assigning
- No reliance on falsy coercion

### Fix 2: Backend - Add Logging and Error Handling (CRITICAL)

**File:** `backend/internal/api/handlers/proxy_host_handler.go`
**Lines:** 231-248

**Change:**

```go
// Security Header Profile: update only if provided
if v, ok := payload["security_header_profile_id"]; ok {
    logger := middleware.GetRequestLogger(c)
    logger.WithField("security_header_profile_id_raw", v).Debug("Received security header profile ID")

    if v == nil {
        host.SecurityHeaderProfileID = nil
        logger.Debug("Cleared security header profile ID (set to nil)")
    } else {
        updated := false
        var finalID uint

        switch t := v.(type) {
        case float64:
            if id, ok := safeFloat64ToUint(t); ok {
                finalID = id
                updated = true
            } else {
                logger.WithField("value", t).Warn("Failed to convert float64 to uint (out of range or negative)")
            }
        case int:
            if id, ok := safeIntToUint(t); ok {
                finalID = id
                updated = true
            } else {
                logger.WithField("value", t).Warn("Failed to convert int to uint (negative)")
            }
        case string:
            if n, err := strconv.ParseUint(t, 10, 32); err == nil {
                finalID = uint(n)
                updated = true
            } else {
                logger.WithField("value", t).WithError(err).Warn("Failed to parse string to uint")
            }
        default:
            logger.WithField("type", fmt.Sprintf("%T", v)).Error("Unexpected type for security_header_profile_id")
        }

        if !updated {
            logger.Error("Failed to update security_header_profile_id - conversion failed")
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("invalid security_header_profile_id: cannot convert %v (%T) to uint", v, v),
            })
            return
        }

        host.SecurityHeaderProfileID = &finalID
        logger.WithField("security_header_profile_id", finalID).Debug("Set security header profile ID")
    }
}
```

**Why:**

- **Logs incoming raw value** - We can see exactly what the frontend sent
- **Logs conversion attempts** - We can see if type assertions match
- **Logs success/failure** - We know if the field was updated
- **Returns explicit error** - No more silent failures
- **Includes type information** - Helps diagnose payload issues

### Fix 3: Backend - Verify GORM Update Behavior (Investigation)

**File:** `backend/internal/services/proxyhost_service.go`
**Line:** 92

**Current:**

```go
return s.db.Save(host).Error
```

**Investigation needed:**

- Does `Save()` properly update nullable pointer fields?
- Should we use `Updates()` instead?
- Should we use `Select()` to explicitly update specific fields?

**Possible alternative:**

```go
// Option A: Use Updates with Select (explicit fields)
return s.db.Model(host).Select("SecurityHeaderProfileID").Updates(host).Error

// Option B: Use Updates (auto-detects changed fields)
return s.db.Model(host).Updates(host).Error
```

**Note:** `Updates()` skips zero values but handles nil pointers correctly. Need to test.

---

## Testing Plan

### Phase 1: Add Logging (Diagnostic)

1. Implement Fix 2 (backend logging)
2. Deploy to test environment
3. Reproduce the bug:
   - Create host with Strict profile
   - Edit host, change to Basic profile
   - Check logs to see:
     - What value was received
     - What type it was
     - If conversion succeeded
     - What value was set
4. Check database directly:

   ```sql
   SELECT id, name, security_header_profile_id FROM proxy_hosts WHERE name = 'Test Host';
   ```

### Phase 2: Fix Issues (Implementation)

Based on log findings:

- If conversion is failing → Fix conversion logic
- If GORM isn't saving → Change to `Updates()` or `Select()`
- If payload is wrong type → Investigate middleware/JSON unmarshaling

### Phase 3: Frontend Safety (Prevention)

1. Implement Fix 1 (explicit value handling)
2. Test all scenarios:
   - Select "None" → Should send `null`
   - Select "Basic" → Should send profile ID
   - Select invalid option → Should handle gracefully

### Phase 4: Verification (End-to-End)

1. Create proxy host
2. Assign Strict profile (ID 2)
3. Save → Verify in DB: `security_header_profile_id = 2`
4. Edit host
5. Change to Basic profile (ID 1)
6. Save → Verify in DB: `security_header_profile_id = 1` ✓
7. Edit host
8. Change to None
9. Save → Verify in DB: `security_header_profile_id = NULL` ✓

---

## Edge Cases to Test

1. **Changing between non-zero profiles:** Strict (2) → Basic (1)
2. **Setting to None:** Basic (1) → None (null)
3. **Setting from None:** None (null) → Strict (2)
4. **Rapid changes:** Strict → Basic → Paranoid → None
5. **Invalid profile ID:** Send ID 999 (non-existent)
6. **Zero profile ID:** Send ID 0 (should become null)
7. **Negative ID:** Send ID -1 (should reject)
8. **String ID:** Send "2" as string (should convert)
9. **Float ID:** Send 2.5 (should reject - not a valid uint)

---

## Success Criteria

✅ **Bug is fixed when:**

1. User can change security header profile from one to another, and it persists after save
2. User can set profile to "None" (null), and it persists
3. User can set profile from "None" to any preset, and it persists
4. Backend logs show clear diagnostic information when profile changes
5. Invalid profile IDs return explicit errors (not silent failures)
6. All edge cases pass testing
7. No regressions in other proxy host fields

---

## Files to Modify

### High Priority (Fixes)

1. **backend/internal/api/handlers/proxy_host_handler.go** (Lines 231-248)
   - Add logging
   - Add error handling
   - Remove silent failures

2. **frontend/src/components/ProxyHostForm.tsx** (Lines 658-661)
   - Explicit value handling
   - Remove falsy coercion

### Medium Priority (Investigation)

1. **backend/internal/services/proxyhost_service.go** (Line 92)
   - Verify GORM Save() vs Updates()
   - May need to change update method

### Low Priority (Testing)

1. **backend/internal/api/handlers/proxy_host_handler_test.go**
   - Add test for security header profile updates
   - Test edge cases

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| GORM doesn't save nullable pointers | Medium | High | Test with Updates() method |
| Frontend sends wrong type | Low | High | Add explicit type checking |
| Middleware modifies payload | Low | High | Add early logging to check |
| Conversion logic has bugs | Low | Medium | Add comprehensive unit tests |
| Breaking other fields | Very Low | High | Test full proxy host CRUD |

---

## Implementation Priority

1. **CRITICAL:** Add backend logging (Fix 2) - Enables diagnosis
2. **HIGH:** Add error handling (Fix 2) - Prevents silent failures
3. **MEDIUM:** Frontend safety (Fix 1) - Prevents edge case bugs
4. **LOW:** GORM investigation (Fix 3) - Only if Save() proves problematic

---

## Next Steps

1. **Implement Fix 2** (backend logging) - ~15 minutes
2. **Deploy to test environment** - ~5 minutes
3. **Reproduce bug with logging enabled** - ~10 minutes
4. **Analyze logs** - ~10 minutes
5. **Implement remaining fixes based on findings** - ~20 minutes
6. **Test all edge cases** - ~30 minutes
7. **Document findings** - ~10 minutes

**Total estimated time:** ~1.5 hours

---

## Conclusion

The root cause of the security header profile persistence bug is likely a **silent failure in the backend handler's type conversion logic**. The lack of logging makes it impossible to diagnose the exact failure point.

The immediate fix is to:

1. Add comprehensive logging to track the value through its lifecycle
2. Add explicit error handling to prevent silent failures
3. Improve frontend value handling to prevent edge cases

Once logging is in place, we can identify the exact failure point and implement a targeted fix.

---

**Investigation Date:** 2025-12-18
**Status:** Root cause hypothesized, fix plan documented
**Next Action:** Implement backend logging to confirm hypothesis

---

## Appendix: Expected vs. Actual Data Flow

### Expected Data Flow (Should Work)

```
Frontend
  User changes dropdown to "Basic" (ID 1)
    ↓
  onChange fires: parseInt("1") = 1
    ↓
  setFormData({ security_header_profile_id: 1 })
    ↓
  User clicks Save
    ↓
  handleSubmit sends: {"security_header_profile_id": 1}
    ↓
Backend
  Gin unmarshals JSON: payload["security_header_profile_id"] = float64(1.0)
    ↓
  Handler checks: if v, ok := payload["security_header_profile_id"]; ok { ✓
    ↓
  v == nil? No ✓
    ↓
  switch t := v.(type) { case float64: ✓
    ↓
  safeFloat64ToUint(1.0) returns (1, true) ✓
    ↓
  host.SecurityHeaderProfileID = &1 ✓
    ↓
  h.service.Update(host) ✓
    ↓
  s.db.Save(host) ✓
    ↓
Database
  UPDATE proxy_hosts SET security_header_profile_id = 1 WHERE id = X ✓
```

**This flow SHOULD work! So why doesn't it?**

### Actual Data Flow (Hypothesis - Needs Logging to Confirm)

**Scenario A: Payload Type Mismatch**

```
Frontend sends: {"security_header_profile_id": "1"}  ← String instead of number!
Backend receives: v = "1" (string)
Type switch enters: case string:
strconv.ParseUint("1", 10, 32) succeeds
Sets: host.SecurityHeaderProfileID = &1
BUT: Something downstream fails or overwrites it
```

**Scenario B: GORM Save Issue**

```
Everything up to Save() works correctly
host.SecurityHeaderProfileID = &1 ✓
db.Save(host) is called
BUT: GORM doesn't detect the field as "changed"
OR: GORM skips nullable pointer updates
Result: Old value remains in database
```

**Scenario C: Concurrent Request**

```
Request A: Sets profile to Basic (ID 1)
Request B: Reloads host data (has Strict, ID 2)
Request A saves: profile_id = 1
Request B saves: profile_id = 2  ← Overwrites A's change
Result: Profile reverts to Strict
```

**Only logging will tell us which scenario is happening!**

---

**End of Investigation Report**
