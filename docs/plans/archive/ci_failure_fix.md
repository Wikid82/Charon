# CI Failure Fix Plan

## Status: RESOLVED ✅

## Problem Statement

The CI pipeline failed on the feature/beta-release branch due to a WAF Integration Test failure. The failure was in workflow run #163, NOT in the referenced run #20452768958 (which was cancelled, not failed).

## Workflow Run Information

- **Failed Run**: <https://github.com/Wikid82/Charon/actions/runs/20449607151>
- **Cancelled Run** (not the issue): <https://github.com/Wikid82/Charon/actions/runs/20452768958>
- **Branch**: feature/beta-release
- **Failed Job**: Coraza WAF Integration
- **Commit**: 0543a15 (fix(security): resolve CrowdSec startup permission failures)
- **Fixed In**: 430eb85 (fix(integration): resolve WAF test authentication order)

## Root Cause Analysis

### Actual Failure (from logs)

The WAF integration test failed with **HTTP 401 Unauthorized** when attempting to create a proxy host:

```
{"client":"172.18.0.1","latency":"433.811µs","level":"info","method":"POST",
"msg":"handled request","path":"/api/v1/proxy-hosts","request_id":"26716960-4547-496b-8271-2acdcdda9872",
"status":401}
```

### Root Cause

The `scripts/coraza_integration.sh` test script had an **authentication ordering bug**:

1. Script attempted to create proxy host **WITHOUT** authentication cookie
2. API endpoint `/api/v1/proxy-hosts` requires authentication (returns 401)
3. Script then authenticated and obtained session cookie (too late)
4. Subsequent API calls correctly used the cookie

### Why This Occurred

The proxy host creation endpoints were moved to the authenticated API group in a previous commit, but the integration test script was not updated to authenticate before creating proxy hosts.

## Fix Implementation (Already Applied)

**Commit**: 430eb85c9f020515bf4fdc5211e32c3ce5c26877

### Changes Made to `scripts/coraza_integration.sh`

1. **Moved authentication block** from line ~207 to after line 146 (after API ready check, before proxy host creation)
2. **Added `-b ${TMP_COOKIE}`** to proxy host creation curl command
3. **Added `-b ${TMP_COOKIE}`** to proxy host list curl command (for fallback logic)
4. **Added `-b ${TMP_COOKIE}`** to proxy host update curl command (for fallback logic)
5. **Removed duplicate** authentication block that was executing too late

### Fixed Flow

```
1. Build/start containers
2. Wait for API ready
3. ✅ Register user and login (create session cookie)
4. Start httpbin backend
5. ✅ Create proxy host WITH authentication
6. Create WAF ruleset with authentication
7. Enable WAF globally with authentication
8. Run WAF tests (BLOCK and MONITOR modes)
9. Cleanup
```

## Verification Steps

✅ **Completed Successfully**

1. WAF Integration Tests workflow run #164 passed after the fix
2. Proxy host creation returned HTTP 201 (Created) instead of 401
3. All subsequent WAF tests (BLOCK mode and MONITOR mode) passed
4. No regressions in other CI workflows

## Related Files

- `scripts/coraza_integration.sh` - Fixed authentication ordering
- `docs/plans/waf_integration_fix.md` - Detailed analysis document
- `.github/workflows/waf-integration.yml` - CI workflow definition

## Key Learnings

1. **Always check ACTUAL logs** - The initially referenced run was cancelled, not failed
2. **Authentication order matters** - API endpoints that require auth must have credentials passed from the start
3. **Integration tests must track API changes** - When routes move to authenticated groups, tests must be updated

## Previous Incorrect Analysis

The initial analysis incorrectly focused on Go version 1.25.5 as a potential issue. This was completely incorrect:

- Go 1.25.5 is the current correct version (released Dec 2, 2025)
- No Go version issues existed
- The actual failure was an integration test authentication bug
- Lesson: Always examine actual error messages instead of making assumptions

---

**Resolution**: Issue fixed in commit 430eb85 and verified in subsequent CI runs.
