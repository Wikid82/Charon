# GORM ID Leak Security Vulnerability Fix

**Status**: ACTIVE
**Priority**: CRITICAL 🔴
**Created**: 2026-01-28
**Security Issue**: SQL Injection Vulnerability via GORM `.First()` ID Leaking

---

## Executive Summary

**Critical Security Bug**: GORM's `.First(&model, id)` method with string ID fields causes SQL injection-like behavior by directly inserting the ID value into the WHERE clause without proper parameterization. This affects **12 production service methods** and **multiple test files**, causing widespread test failures.

**Impact**:
- 🔴 SQL syntax errors in production code
- 🔴 Test suite failures (uptime_service_race_test.go)
- 🔴 Potential SQL injection vector
- 🔴 String-based UUIDs are leaking into raw SQL

**Root Cause**: When calling `.First(&model, id)` where `model` has a string ID field, GORM directly inserts the ID as a raw SQL fragment instead of using parameterized queries.

---

## SQL Injection Evidence

### Actual Error from Tests

```
/uptime_service_race_test.go:100 unrecognized token: "639fb2d8"
SELECT * FROM \`uptime_hosts\` WHERE 639fb2d8-f3a7-4c37-9a8e-22f2e73f8d87 AND \`uptime_hosts\`.\`id\` = "639fb2d8-f3a7-4c37-9a8e-22f2e73f8d87"
```

### What Should Happen vs What Actually Happens

| Expected SQL | Actual Broken SQL |
|--------------|-------------------|
| \`WHERE \\\`uptime_hosts\\\`.\\\`id\\\` = "639fb2d8-..."\` | \`WHERE 639fb2d8-... AND \\\`uptime_hosts\\\`.\\\`id\\\` = "639fb2d8-..."\` |

**The Problem**: The UUID string \`639fb2d8-f3a7-4c37-9a8e-22f2e73f8d87\` is being leaked as a raw SQL fragment. GORM interprets it as a SQL expression and injects it directly into the WHERE clause.

---

## Technical Root Cause

### GORM Behavior with String IDs

When calling \`.First(&model, id)\`:
1. If the model's ID field is \`uint\`, GORM correctly parameterizes: \`WHERE id = ?\`
2. If the model's ID field is \`string\`, GORM **leaks the ID as raw SQL**: \`WHERE <id_value> AND id = ?\`

### Models Affected by String IDs

From codebase search, these models have string ID fields:

| Model | ID Field | Location |
|-------|----------|----------|
| \`ManualChallenge\` | \`ID string\` | \`internal/models/manual_challenge.go:30\` |
| \`UptimeMonitor\` | \`ID string\` | (inferred from error logs) |
| \`UptimeHost\` | \`ID string\` | (inferred from error logs) |

**Note**: Even models with \`uint\` IDs are vulnerable if the parameter passed is a string!

---

## Vulnerability Inventory

### CRITICAL: Production Service Files (12 instances)

1. **access_list_service.go:105** - \`s.db.First(&acl, id)\`
2. **remoteserver_service.go:68** - \`s.db.First(&server, id)\`
3. **notification_service.go:458** - \`s.DB.First(&t, "id = ?", id)\`
4. **uptime_service.go:353** - \`s.DB.First(&uptimeHost, "id = ?", hostID)\`
5. **uptime_service.go:845** - \`s.DB.First(&uptimeHost, "id = ?", hostID)\`
6. **uptime_service.go:999** - \`s.DB.First(&host, hostID)\`
7. **uptime_service.go:1101** - \`s.DB.First(&monitor, "id = ?", id)\`
8. **uptime_service.go:1115** - \`s.DB.First(&monitor, "id = ?", id)\`
9. **uptime_service.go:1143** - \`s.DB.First(&monitor, "id = ?", id)\`
10. **certificate_service.go:412** - \`s.db.First(&cert, id)\`
11. **dns_provider_service.go:118** - \`s.db.WithContext(ctx).First(&provider, id)\`
12. **proxyhost_service.go:108** - \`s.db.First(&host, id)\`

### HIGH PRIORITY: Test Files (8 instances causing failures)

#### uptime_service_race_test.go (5 instances) ⚠️ CAUSING TEST FAILURES

1. **Line 100** 🔥 - \`db.First(&host, host.ID)\`
2. **Line 106** 🔥 - \`db.First(&host, host.ID)\`
3. **Line 152** - \`db.First(&host, host.ID)\`
4. **Line 255** - \`db.First(&updatedHost, "id = ?", host.ID)\`
5. **Line 398** - \`db.First(&updatedHost, "id = ?", host.ID)\`

#### Other Test Files

6. **credential_service_test.go:426** - \`db.First(&updatedProvider, provider.ID)\`
7. **dns_provider_service_test.go:550** - \`db.First(&dbProvider, provider.ID)\`
8. **dns_provider_service_test.go:617** - \`db.First(&retrieved, provider.ID)\`
9. **security_headers_service_test.go:184** - \`db.First(&saved, profile.ID)\`

---

## Fix Pattern: The Standard Safe Query

### ❌ VULNERABLE Pattern
\`\`\`go
// NEVER DO THIS with string IDs or when ID type is uncertain:
db.First(&model, id)
db.First(&model, "id = ?", id)  // Also wrong! Second param is value, not condition
\`\`\`

### ✅ SAFE Pattern
\`\`\`go
// ALWAYS DO THIS:
db.Where("id = ?", id).First(&model)
\`\`\`

### Why This Pattern is Safe

1. **Explicit Parameterization**: \`id\` is treated as a parameter value, not SQL
2. **No Type Confusion**: Works for both \`uint\` and \`string\` IDs
3. **SQL Injection Safe**: GORM uses prepared statements with \`?\` placeholders
4. **Clear Intent**: Readable and obvious what the query does

---

## Implementation Plan

### Phase 1: Fix Production Services (CRITICAL)

**Duration**: 20 minutes
**Priority**: 🔴 CRITICAL

**Files to Fix** (12 instances):
1. access_list_service.go (Line 105)
2. remoteserver_service.go (Line 68)
3. notification_service.go (Line 458)
4. uptime_service.go (Lines 353, 845, 999, 1101, 1115, 1143)
5. certificate_service.go (Line 412)
6. dns_provider_service.go (Line 118)
7. proxyhost_service.go (Line 108)

### Phase 2: Fix Test Files

**Duration**: 15 minutes
**Priority**: 🟡 HIGH

**Files to Fix** (8+ instances):
1. uptime_service_race_test.go (Lines 100, 106, 152, 255, 398)
2. credential_service_test.go (Line 426)
3. dns_provider_service_test.go (Lines 550, 617)
4. security_headers_service_test.go (Line 184)

### Phase 3: Fix DNS Provider Test Expectation

**Duration**: 5 minutes
**Priority**: 🟢 MEDIUM

**File**: dns_provider_service_test.go
**Lines**: 1382 (comment), 1385 (assertion)

**Change**:
- Line 1382: Update comment to describe validation failure instead of decryption failure
- Line 1385: Change \`"DECRYPTION_ERROR"\` to \`"VALIDATION_ERROR"\`

### Phase 4: Verification

**Duration**: 10 minutes

**Test Commands**:
\`\`\`bash
cd /projects/Charon/backend

# Run race tests (primary failure source)
go test -v -race -run TestCheckHost ./internal/services/

# Run all uptime tests
go test -v -run Uptime ./internal/services/

# Run all service tests
go test ./internal/services/...

# Verify no vulnerable patterns remain
grep -rn "\\.First(&.*," internal/services/ | grep -v "Where"
\`\`\`

**Success Criteria**:
- ✅ \`uptime_service_race_test.go\` tests pass
- ✅ No SQL syntax errors in logs
- ✅ All service tests pass
- ✅ DNS provider test expectation matches actual behavior
- ✅ Zero vulnerable \`.First(&model, id)\` patterns remain

---

## Timeline Summary

| Phase | Duration | Priority | Tasks |
|-------|----------|----------|-------|
| **Phase 1** | 20 min | 🔴 CRITICAL | Fix 12 production service methods |
| **Phase 2** | 15 min | 🟡 HIGH | Fix 8+ test file instances |
| **Phase 3** | 5 min | 🟢 MEDIUM | Fix DNS provider test expectation |
| **Phase 4** | 10 min | 🟡 HIGH | Run full test suite verification |
| **TOTAL** | **50 min** | | Complete security fix |

---

## Risk Assessment

### Security Impact

| Risk | Before Fix | After Fix |
|------|-----------|-----------|
| **SQL Injection** | 🔴 HIGH | ✅ None |
| **Test Failures** | 🔴 Blocking | ✅ All pass |
| **Production Bugs** | 🔴 Likely | ✅ None |
| **Data Corruption** | 🟡 Possible | ✅ None |

### Deployment Risk

**Risk Level**: LOW

**Justification**:
- Changes are surgical and well-defined
- Pattern is mechanical (\`.First(&model, id)\` → \`.Where("id = ?", id).First(&model)\`)
- No business logic changes
- Comprehensive test coverage validates correctness

---

## Code Change Summary

### Production Services: 12 Changes
- access_list_service.go: 1 fix
- remoteserver_service.go: 1 fix
- notification_service.go: 1 fix
- uptime_service.go: 6 fixes
- certificate_service.go: 1 fix
- dns_provider_service.go: 1 fix
- proxyhost_service.go: 1 fix

### Test Files: 8+ Changes
- uptime_service_race_test.go: 5 fixes
- credential_service_test.go: 1 fix
- dns_provider_service_test.go: 2 fixes (1 GORM, 1 expectation)
- security_headers_service_test.go: 1 fix

### Total Lines Changed: ~25 lines
### Total Files Modified: 9 files

---

## Success Criteria

- [x] Root cause identified for SQL injection vulnerability
- [x] All vulnerable instances catalogued (20+ total)
- [x] Comprehensive fix plan documented
- [ ] All production service methods fixed (12 instances)
- [ ] All test files fixed (8+ instances)
- [ ] Test expectation corrected (DNS provider test)
- [ ] Race tests pass without SQL errors
- [ ] Full test suite passes

---

## Related Documentation

- **GORM Documentation**: https://gorm.io/docs/query.html
- **SQL Injection Prevention**: OWASP Top 10 (A03:2021 – Injection)
- **Parameterized Queries**: https://cheatsheetseries.owasp.org/cheatsheets/SQL_Injection_Prevention_Cheat_Sheet.html

---

## Notes

### Why This Bug is Insidious

1. **Silent Failure**: Works fine with \`uint\` IDs, fails with \`string\` IDs
2. **Type-Dependent**: Behavior changes based on model ID field type
3. **Misleading API**: \`.First(&model, id)\` looks safe but isn't
4. **Widespread**: Affects 20+ instances across codebase
5. **Hard to Debug**: SQL errors appear cryptic and non-obvious

### Best Practices Going Forward

✅ **DO**:
- Always use \`.Where("id = ?", id).First(&model)\`
- Use parameterized queries for all dynamic values
- Test with both \`uint\` and \`string\` ID models

❌ **DON'T**:
- Never use \`.First(&model, id)\` with string IDs
- Never trust GORM to handle ID types correctly
- Never pass SQL fragments as \`.First()\` parameters

---

**Plan Status**: ✅ COMPLETE AND READY FOR IMPLEMENTATION

**Next Action**: Begin Phase 1 - Fix Production Services
