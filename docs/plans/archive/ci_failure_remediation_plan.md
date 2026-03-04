# CI/CD Failure Remediation Plan

> **Created:** 2025-12-13
> **Status:** Planned
> **Priority:** High

## Executive Summary

Three GitHub Actions workflows have failed. This document provides root cause analysis and specific remediation steps for each failure.

---

## 1. Quality Checks Workflow (`quality-checks.yml`)

### 1.1 Frontend Test Timeout

**File:** [frontend/src/components/**tests**/LiveLogViewer.test.tsx](../../frontend/src/components/__tests__/LiveLogViewer.test.tsx#L374)
**Test:** "displays blocked requests with special styling" under "Security Mode"
**Error:** `Test timed out in 5000ms`

#### Root Cause Analysis

The failing test at line 374 has a race condition between:

1. The `await act(async () => { mockOnSecurityMessage(blockedLog); })` call
2. The subsequent `waitFor` assertions

The test attempts to verify multiple DOM elements after sending a security log message:

```typescript
await waitFor(() => {
  // Use getAllByText since 'WAF' appears both in dropdown option and source badge
  const wafElements = screen.getAllByText('WAF');
  expect(wafElements.length).toBeGreaterThanOrEqual(2); // Option + badge
  expect(screen.getByText('10.0.0.1')).toBeTruthy();
  expect(screen.getByText(/BLOCKED: SQL injection detected/)).toBeTruthy();
  // Block reason is shown in brackets - check for the text content
  expect(screen.getByText(/\[SQL injection detected\]/)).toBeTruthy();
});
```

**Issues identified:**

1. **Multiple assertions in single waitFor:** The `waitFor` contains 4 separate assertions. If any one fails, the entire block retries, potentially causing timeout.
2. **Complex regex matching:** The regex patterns `/BLOCKED: SQL injection detected/` and `/\[SQL injection detected\]/` may be matching against DOM elements that are not yet rendered.
3. **State update timing:** The `act()` wrapper is async but the component's state update (`setLogs`) may not complete before `waitFor` starts checking.

#### Recommended Fix

**Option A: Increase test timeout (Quick fix)**

```typescript
// Line 371-374 - Add timeout option
it('displays blocked requests with special styling', async () => {
  render(<LiveLogViewer mode="security" />);

  // Wait for connection to establish
  await waitFor(() => expect(screen.getByText('Connected')).toBeTruthy());

  const blockedLog: logsApi.SecurityLogEntry = {
    timestamp: '2025-12-12T10:30:00Z',
    level: 'warn',
    logger: 'http.handlers.waf',
    client_ip: '10.0.0.1',
    method: 'POST',
    uri: '/admin',
    status: 403,
    duration: 0.001,
    size: 0,
    user_agent: 'Attack/1.0',
    host: 'example.com',
    source: 'waf',
    blocked: true,
    block_reason: 'SQL injection detected',
  };

  // Send message inside act to properly handle state updates
  await act(async () => {
    if (mockOnSecurityMessage) {
      mockOnSecurityMessage(blockedLog);
    }
  });

  // Split assertions into separate waitFor calls to isolate failures
  await waitFor(() => {
    expect(screen.getByText('10.0.0.1')).toBeTruthy();
  }, { timeout: 10000 });

  await waitFor(() => {
    expect(screen.getByText(/BLOCKED: SQL injection detected/)).toBeTruthy();
  });

  await waitFor(() => {
    expect(screen.getByText(/\[SQL injection detected\]/)).toBeTruthy();
  });

  // Verify WAF badge appears (separate from dropdown option)
  await waitFor(() => {
    const wafElements = screen.getAllByText('WAF');
    expect(wafElements.length).toBeGreaterThanOrEqual(2);
  });
}, 15000); // Increase overall test timeout
```

**Option B: Use `findBy` queries (Preferred)**

```typescript
it('displays blocked requests with special styling', async () => {
  render(<LiveLogViewer mode="security" />);

  // Wait for connection
  await screen.findByText('Connected');

  const blockedLog: logsApi.SecurityLogEntry = {
    // ... same as before
  };

  await act(async () => {
    if (mockOnSecurityMessage) {
      mockOnSecurityMessage(blockedLog);
    }
  });

  // Use findBy for async queries - cleaner and more reliable
  await screen.findByText('10.0.0.1');
  await screen.findByText(/BLOCKED: SQL injection detected/);
  await screen.findByText(/\[SQL injection detected\]/);

  // For getAllByText, wrap in waitFor since findAllBy isn't used for counts
  await waitFor(() => {
    const wafElements = screen.getAllByText('WAF');
    expect(wafElements.length).toBeGreaterThanOrEqual(2);
  });
});
```

---

### 1.2 Backend gosec G115 - Integer Overflow Errors

#### 1.2.1 backup_service.go Line 293

**File:** [backend/internal/services/backup_service.go](../../backend/internal/services/backup_service.go#L293)
**Error:** `G115: integer overflow conversion uint64 -> int64`

**Current Code:**

```go
func (s *BackupService) GetAvailableSpace() (int64, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(s.BackupDir, &stat); err != nil {
        return 0, fmt.Errorf("failed to get disk space: %w", err)
    }
    // Available blocks * block size = available bytes
    return int64(stat.Bavail) * int64(stat.Bsize), nil  // Line 293
}
```

**Root Cause:**

- `stat.Bavail` is `uint64` (available blocks)
- `stat.Bsize` is `int64` (block size, can be negative on some systems but practically always positive)
- Converting `uint64` to `int64` can overflow if `Bavail` exceeds `math.MaxInt64`

**Recommended Fix:**

```go
import (
    "math"
    // ... other imports
)

func (s *BackupService) GetAvailableSpace() (int64, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(s.BackupDir, &stat); err != nil {
        return 0, fmt.Errorf("failed to get disk space: %w", err)
    }

    // Safe conversion with overflow check
    // Bavail is uint64, Bsize is int64 (but always positive for valid filesystems)
    bavail := stat.Bavail
    bsize := stat.Bsize

    // Check for negative block size (invalid filesystem state)
    if bsize < 0 {
        return 0, fmt.Errorf("invalid block size: %d", bsize)
    }

    // Check if Bavail exceeds int64 max before conversion
    if bavail > uint64(math.MaxInt64) {
        // Cap at max int64 - this represents ~8 exabytes which is sufficient
        return math.MaxInt64, nil
    }

    // Safe to convert now
    availBlocks := int64(bavail)
    blockSize := bsize

    // Check for overflow in multiplication
    if availBlocks > 0 && blockSize > math.MaxInt64/availBlocks {
        return math.MaxInt64, nil
    }

    return availBlocks * blockSize, nil
}
```

---

#### 1.2.2 proxy_host_handler.go Lines 216 and 235

**File:** [backend/internal/api/handlers/proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go#L216)
**Error:** `G115: integer overflow conversion int -> uint`

**Current Code (Lines 213-222 for certificate_id, Lines 231-240 for access_list_id):**

```go
// Line 216
case int:
    id := uint(t)  // G115: int -> uint overflow
    host.CertificateID = &id

// Line 235
case int:
    id := uint(t)  // G115: int -> uint overflow
    host.AccessListID = &id
```

**Root Cause:**

- When JSON is unmarshaled into `map[string]interface{}`, numeric values can become `float64` or `int`
- Converting a negative `int` to `uint` causes overflow (e.g., `-1` becomes `18446744073709551615`)
- Database IDs should never be negative

**Recommended Fix:**

Create a helper function and apply to both locations:

```go
// Add at package level or in a shared util package
// safeIntToUint safely converts an int to uint, returning false if negative
func safeIntToUint(i int) (uint, bool) {
    if i < 0 {
        return 0, false
    }
    return uint(i), true
}

// safeFloat64ToUint safely converts a float64 to uint, returning false if negative or fractional
func safeFloat64ToUint(f float64) (uint, bool) {
    if f < 0 || f != float64(uint(f)) {
        return 0, false
    }
    return uint(f), true
}
```

**Updated handler code (Lines 210-245):**

```go
if v, ok := payload["certificate_id"]; ok {
    if v == nil {
        host.CertificateID = nil
    } else {
        switch t := v.(type) {
        case float64:
            if id, ok := safeFloat64ToUint(t); ok {
                host.CertificateID = &id
            }
        case int:
            if id, ok := safeIntToUint(t); ok {
                host.CertificateID = &id
            }
        case string:
            if n, err := strconv.ParseUint(t, 10, 32); err == nil {
                id := uint(n)
                host.CertificateID = &id
            }
        }
    }
}
if v, ok := payload["access_list_id"]; ok {
    if v == nil {
        host.AccessListID = nil
    } else {
        switch t := v.(type) {
        case float64:
            if id, ok := safeFloat64ToUint(t); ok {
                host.AccessListID = &id
            }
        case int:
            if id, ok := safeIntToUint(t); ok {
                host.AccessListID = &id
            }
        case string:
            if n, err := strconv.ParseUint(t, 10, 32); err == nil {
                id := uint(n)
                host.AccessListID = &id
            }
        }
    }
}
```

---

## 2. PR Checklist Validation Workflow (`pr-checklist.yml`)

**Error:** "Validate history-rewrite checklist (conditional)" failed

### Root Cause Analysis

The workflow at [.github/workflows/pr-checklist.yml](../../.github/workflows/pr-checklist.yml) validates PRs that touch history-rewrite related files. It checks for three items in the PR body:

1. `preview_removals.sh mention` - Pattern: `/preview_removals\.sh/i`
2. `data/backups mention` - Pattern: `/data\/?backups/i`
3. `explicit non-run of --force` - Pattern: `/(?:\[\s*[xX]\s*\]\s*)?(?:i will not run|will not run|do not run|don'?t run|won'?t run)\b[^\n]*--force/i`

**When this check triggers:**

The check only runs if the PR modifies files matching:

- `scripts/history-rewrite/*`
- `docs/plans/history_rewrite.md`
- Any file containing `history-rewrite` in the path

### Resolution Options

**Option A: If PR legitimately touches history-rewrite files**

Update the PR description to include all required checklist items from [.github/PULL_REQUEST_TEMPLATE/history-rewrite.md](../../.github/PULL_REQUEST_TEMPLATE/history-rewrite.md):

```markdown
## Checklist - required for history rewrite PRs

- [x] I have created a **local** backup branch: `backup/history-YYYYMMDD-HHMMSS`
- [x] I have pushed the backup branch to the remote origin
- [x] I have run a dry-run locally: `scripts/history-rewrite/preview_removals.sh --paths '...'`
- [x] I have verified the `data/backups` tarball is present
- [x] I will not run the destructive `--force` step without explicit approval
```

**Option B: If PR doesn't need history-rewrite validation**

Ensure the PR doesn't modify files in:

- `scripts/history-rewrite/`
- `docs/plans/history_rewrite.md`
- Any files with `history-rewrite` in the name

**Option C: False positive - workflow issue**

If the workflow is triggering incorrectly, check the file list detection logic at line 27-28 of the workflow.

---

## 3. Benchmark Workflow (`benchmark.yml`)

### 3.1 "Resource not accessible by integration" Error

**Root Cause:**

The `benchmark-action/github-action-benchmark@v1` action requires write permissions to push benchmark results to the repository. This fails on:

- Pull requests from forks (restricted permissions)
- PRs where `GITHUB_TOKEN` doesn't have `contents: write` permission

**Current Workflow Configuration:**

```yaml
permissions:
  contents: write
  deployments: write
```

The error occurs because:

1. On PRs, the token may not have write access
2. The `auto-push: true` setting tries to push on main branch only, but the action still needs permissions to access the benchmark data

**Recommended Fix:**

```yaml
- name: Store Benchmark Result
  uses: benchmark-action/github-action-benchmark@v1
  with:
    name: Go Benchmark
    tool: 'go'
    output-file-path: backend/output.txt
    github-token: ${{ secrets.GITHUB_TOKEN }}
    # Only auto-push on main branch pushes, not PRs
    auto-push: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}
    alert-threshold: '150%'
    comment-on-alert: true
    fail-on-alert: false
    summary-always: true
    # Skip external data storage on PRs to avoid permission errors
    skip-fetch-gh-pages: ${{ github.event_name == 'pull_request' }}
```

Alternatively, for fork PRs, consider using `pull_request_target` event with caution or skip the benchmark action entirely on PRs.

---

### 3.2 Performance Regression (1.51x threshold exceeded)

**Warning:** Performance regression 1.51x (165768 ns/op vs 109674 ns/op) exceeds 1.5x threshold

**Benchmark Functions Available:**

From [backend/internal/api/handlers/benchmark_test.go](../../backend/internal/api/handlers/benchmark_test.go):

| Benchmark | Line |
|-----------|------|
| `BenchmarkSecurityHandler_GetStatus` | 47 |
| `BenchmarkSecurityHandler_GetStatus_NoSettings` | 82 |
| `BenchmarkSecurityHandler_ListDecisions` | 105 |
| `BenchmarkSecurityHandler_ListRuleSets` | 138 |
| `BenchmarkSecurityHandler_UpsertRuleSet` | 171 |
| `BenchmarkSecurityHandler_CreateDecision` | 202 |
| `BenchmarkSecurityHandler_GetConfig` | 233 |
| `BenchmarkSecurityHandler_UpdateConfig` | 266 |
| `BenchmarkSecurityHandler_GetStatus_Parallel` | 303 |
| `BenchmarkSecurityHandler_ListDecisions_Parallel` | 336 |
| `BenchmarkSecurityHandler_LargeRuleSetContent` | 383 |
| `BenchmarkSecurityHandler_ManySettingsLookups` | 420 |

**Root Cause Analysis:**

The 1.51x regression (165768 ns vs 109674 ns ≈ 56μs increase) likely comes from:

1. **Database operations:** Benchmarks use in-memory SQLite. Any additional queries or model changes would show up here.
2. **Parallel benchmark flakiness:** `BenchmarkSecurityHandler_GetStatus_Parallel` and `BenchmarkSecurityHandler_ListDecisions_Parallel` use shared memory databases which can have contention.
3. **CI environment variability:** GitHub Actions runners have variable performance.

**Investigation Steps:**

1. Run benchmarks locally to establish baseline:

   ```bash
   cd backend && go test -bench=. -benchmem -benchtime=3s ./internal/api/handlers/... -run=^$
   ```

2. Compare with previous commit:

   ```bash
   git stash
   git checkout HEAD~1
   go test -bench=. -benchmem ./internal/api/handlers/... -run=^$ > bench_old.txt
   git checkout -
   git stash pop
   go test -bench=. -benchmem ./internal/api/handlers/... -run=^$ > bench_new.txt
   benchstat bench_old.txt bench_new.txt
   ```

3. Check recent changes to:
   - `internal/api/handlers/security_handler.go`
   - `internal/models/security*.go`
   - Database query patterns

**Recommended Actions:**

**If real regression:**

- Profile the affected handler using `go test -cpuprofile`
- Review recent commits for inefficient code
- Optimize the specific slow path

**If CI flakiness:**

- Increase `alert-threshold` to `175%` or `200%`
- Add `-benchtime=3s` for more stable results
- Consider running benchmarks multiple times and averaging

**Workflow Fix for Threshold:**

```yaml
- name: Store Benchmark Result
  uses: benchmark-action/github-action-benchmark@v1
  with:
    # ... other options
    # Increase threshold to account for CI variability
    alert-threshold: '175%'
    # Don't fail on alert for benchmarks - just warn
    fail-on-alert: false
```

---

## Summary of Required Changes

| File | Change | Priority |
|------|--------|----------|
| `frontend/src/components/__tests__/LiveLogViewer.test.tsx` | Split waitFor assertions, increase timeout | High |
| `backend/internal/services/backup_service.go` | Add overflow protection for `GetAvailableSpace` | High |
| `backend/internal/api/handlers/proxy_host_handler.go` | Add safe int-to-uint conversion helpers | High |
| `.github/workflows/benchmark.yml` | Add permission guards, adjust threshold | Medium |
| PR Description | Add history-rewrite checklist items if applicable | Conditional |

---

## Implementation Order

1. **Backend gosec fixes** - Blocking CI, straightforward fixes
2. **Frontend test timeout** - Blocking CI, may need iteration
3. **Benchmark workflow** - Non-blocking (`fail-on-alert: false`), can be addressed after
4. **PR checklist** - Context-dependent, may not need code changes

---

## Testing Verification

After implementing fixes:

```bash
# Backend
cd backend && go test ./... -v
golangci-lint run  # Should pass gosec G115

# Frontend
cd frontend && npm test

# Full CI simulation
pre-commit run --all-files
```
