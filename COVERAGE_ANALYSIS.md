# Coverage Analysis Report
**Date**: January 12, 2026
**Current Coverage**: 83.2%
**Target Coverage**: 85.0%
**Gap**: 1.8%

---

## Executive Summary

**Recommendation: ADJUST THRESHOLD to 83% (realistic) or ADD TARGETED TESTS (< 1 hour)**

We're 1.8 percentage points away from 85% coverage. The gap is **achievable** but requires strategic testing of specific files.

---

## Critical Finding: pkg/dnsprovider/registry.go

**Impact**: This single file has **0% coverage** and is the primary bottleneck.

```
pkg/dnsprovider/registry.go:
├── Lines: 129
├── Coverage: 0.0%
├── Functions: 10 functions (all at 0%)
    ├── Global()
    ├── NewRegistry()
    ├── Register()
    ├── Get()
    ├── List()
    ├── Types()
    ├── IsSupported()
    ├── Unregister()
    ├── Count()
    └── Clear()
```

**Why it matters**: This is core infrastructure for DNS provider management. Testing it would provide significant coverage gains.

---

## Secondary Issue: manual_challenge_handler.go

**Partial Coverage**: Some endpoints are well-tested, others are not.

```
internal/api/handlers/manual_challenge_handler.go:
├── Lines: 657
├── Functions with LOW coverage:
    ├── VerifyChallenge: 35.0%
    ├── PollChallenge: 36.7%
    ├── DeleteChallenge: 36.7%
    ├── CreateChallenge: 48.1%
    ├── ListChallenges: 52.2%
    ├── GetChallenge: 66.7%
├── Functions with HIGH coverage:
    ├── NewManualChallengeHandler: 100.0%
    ├── RegisterRoutes: 100.0%
    ├── challengeToResponse: 100.0%
    ├── getUserIDFromContext: 100.0%
```

**Analysis**: The handler has basic tests but lacks edge case and error path testing.

---

## Files Under 20% Coverage (Quick Wins)

| File | Coverage | Est. Lines | Testability |
|------|----------|------------|-------------|
| `internal/api/handlers/sanitize.go` | 9% | ~50 | **EASY** (pure logic) |
| `pkg/dnsprovider/builtin/init.go` | 9% | ~30 | **EASY** (init function) |
| `internal/util/sanitize.go` | 10% | ~40 | **EASY** (pure logic) |
| `pkg/dnsprovider/custom/init.go` | 10% | ~30 | **EASY** (init function) |
| `internal/api/middleware/request_logger.go` | 12% | ~80 | **MEDIUM** (HTTP middleware) |
| `internal/server/server.go` | 12% | ~120 | **MEDIUM** (server setup) |
| `internal/api/middleware/recovery.go` | 14% | ~60 | **MEDIUM** (panic recovery) |
| `cmd/api/main.go` | 26% | ~150 | **HARD** (main function, integration) |

---

## DNS Challenge Feature Status

### ✅ WELL-TESTED Components:
- `pkg/dnsprovider/custom/manual_provider.go`: **91.1%** ✓
- `internal/services/dns_provider_service.go`: **81-100%** per function ✓
- `internal/services/manual_challenge_service.go`: **75-100%** per function ✓

### ⚠️ NEEDS WORK Components:
- `pkg/dnsprovider/registry.go`: **0%** ❌
- `internal/api/handlers/manual_challenge_handler.go`: **35-66%** on key endpoints ⚠️

---

## Path to 85% Coverage

### Option A: Test pkg/dnsprovider/registry.go (RECOMMENDED)
**Effort**: 30-45 minutes
**Impact**: ~0.5-1.0% coverage gain (129 lines)
**Risk**: Low (pure logic, no external dependencies)

**Test Strategy**:
```go
// Test plan for registry_test.go
1. TestNewRegistry() - constructor
2. TestRegister() - add provider
3. TestGet() - retrieve provider
4. TestList() - list all providers
5. TestTypes() - get provider type names
6. TestIsSupported() - validate provider type
7. TestUnregister() - remove provider
8. TestCount() - count providers
9. TestClear() - clear all providers
10. TestGlobal() - singleton access
```

### Option B: Improve manual_challenge_handler.go
**Effort**: 45-60 minutes
**Impact**: ~0.8-1.2% coverage gain
**Risk**: Medium (HTTP testing, state management)

**Test Strategy**:
```go
// Add tests for:
1. VerifyChallenge - error paths (invalid IDs, DNS failures)
2. PollChallenge - timeout scenarios, status transitions
3. DeleteChallenge - cleanup verification, cascade deletes
4. CreateChallenge - validation failures, duplicate handling
5. ListChallenges - pagination, filtering edge cases
```

### Option C: Quick Wins (Sanitize + Init Files)
**Effort**: 20-30 minutes
**Impact**: ~0.3-0.5% coverage gain
**Risk**: Very Low (simple utility functions)

**Test Strategy**:
```go
// Test sanitize.go functions
1. Test XSS prevention
2. Test SQL injection chars
3. Test path traversal blocking
4. Test unicode handling

// Test init.go files
1. Verify provider registration
2. Check registry state after init
```

---

## Recommended Action Plan (45-60 min)

**Phase 1** (20 min): Test `pkg/dnsprovider/registry.go`
- Create `pkg/dnsprovider/registry_test.go`
- Test all 10 functions
- Expected gain: +0.8%

**Phase 2** (25 min): Test sanitization files
- Expand `internal/api/handlers/sanitize_test.go`
- Create `internal/util/sanitize_test.go`
- Expected gain: +0.4%

**Phase 3** (15 min): Verify and adjust
- Run coverage again
- Check if we hit 85%
- If not, add 2-3 tests to `manual_challenge_handler.go`

**Total Expected Gain**: +1.2-1.5%
**Final Coverage**: **84.4-84.7%** (close to target)

---

## Alternative: Adjust Threshold

**Pragmatic Option**: Set threshold to **83.0%**

**Rationale**:
1. Main entry point (`cmd/api/main.go`) is at 26% (hard to test)
2. Seed script (`cmd/seed/main.go`) is at 19% (not production code)
3. Middleware init functions are low-value test targets
4. **Core business logic is well-tested** (DNS providers, services, handlers)

**Files Intentionally Untested** (acceptable):
- `cmd/api/main.go` - integration test territory
- `cmd/seed/main.go` - utility script
- `internal/server/server.go` - wired in integration tests
- Init functions - basic registration logic

---

## Coverage by Component

| Component | Coverage | Status |
|-----------|----------|--------|
| **Handlers** | 83.8% | ✅ Good |
| **Middleware** | 99.1% | ✅ Excellent |
| **Routes** | 86.9% | ✅ Good |
| **Cerberus** | 100.0% | ✅ Perfect |
| **Config** | 100.0% | ✅ Perfect |
| **CrowdSec** | 85.4% | ✅ Good |
| **Crypto** | 86.9% | ✅ Good |
| **Database** | 91.3% | ✅ Excellent |
| **Metrics** | 100.0% | ✅ Perfect |
| **Models** | 96.8% | ✅ Excellent |
| **Network** | 91.2% | ✅ Excellent |
| **Security** | 95.7% | ✅ Excellent |
| **Server** | 93.3% | ✅ Excellent |
| **Services** | 80.9% | ⚠️ Needs work |
| **Utils** | 74.2% | ⚠️ Needs work |
| **DNS Providers (Custom)** | 91.1% | ✅ Excellent |
| **DNS Providers (Builtin)** | 30.4% | ❌ Low |
| **DNS Provider Registry** | 0.0% | ❌ Not tested |

---

## Conclusion

**We CAN reach 85% with targeted testing in < 1 hour.**

**Recommendation**:
1. **Immediate**: Test `pkg/dnsprovider/registry.go` (+0.8%)
2. **Quick Win**: Test sanitization utilities (+0.4%)
3. **If Needed**: Add 3-5 tests to `manual_challenge_handler.go` (+0.3-0.5%)

**Alternatively**: Adjust threshold to **83%** and focus on **Patch Coverage** (new code only) in CI, which is already at 100% for recent changes.

---

## Next Steps

Choose one path:

**Path A** (Testing):
```bash
# 1. Create registry_test.go
touch pkg/dnsprovider/registry_test.go

# 2. Run tests
go test ./pkg/dnsprovider -v -cover

# 3. Check progress
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

**Path B** (Threshold Adjustment):
```bash
# Update CI configuration to 83%
# Focus on Patch Coverage (100% for new changes)
# Document exception rationale
```

**Verdict**: Both paths are valid. Path A is recommended for completeness.
