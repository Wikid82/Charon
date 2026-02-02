# Codecov Patch Coverage Gap Analysis - PR #583

## Executive Summary

PR #583 introduced Caddyfile normalization functionality. Codecov reports two files with missing PATCH coverage:

| File | Patch Coverage | Missing Lines | Partial Lines |
|------|---------------|---------------|---------------|
| `backend/internal/caddy/importer.go` | 56.52% | 5 | 5 |
| `backend/internal/api/handlers/import_handler.go` | 0% | 6 | 0 |

**Goal:** Achieve 100% patch coverage on the 16 changed lines.

---

## Detailed Line Analysis

### 1. `importer.go` - Lines Needing Coverage

**Changed Code Block (Lines 27-40): `DefaultExecutor.Execute()` with Timeout**

```go
func (e *DefaultExecutor) Execute(name string, args ...string) ([]byte, error) {
    // Set a reasonable timeout for Caddy commands (5 seconds should be plenty for fmt/adapt)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, name, args...)
    output, err := cmd.CombinedOutput()

    // If context timed out, return a clear error message
    if ctx.Err() == context.DeadlineExceeded {  // ❌ PARTIAL (lines 36-38)
        return output, fmt.Errorf("command timed out after 5 seconds (caddy binary may be unavailable or misconfigured): %s %v", name, args)
    }

    return output, err
}
```

**Missing/Partial Coverage:**
- Line 36-38: Timeout error branch (`DeadlineExceeded`) - never exercised

**Changed Code Block (Lines 135-140): `NormalizeCaddyfile()` error paths**

```go
if err := tmpFile.Close(); err != nil {           // ❌ MISSING
    return "", fmt.Errorf("failed to close temp file: %w", err)
}
```

And:
```go
formatted, err := os.ReadFile(tmpFile.Name())
if err != nil {                                   // ❌ MISSING (line ~152)
    return "", fmt.Errorf("failed to read formatted file: %w", err)
}
```

**Missing Coverage:**
- `tmpFile.Close()` error path
- `os.ReadFile()` error path after `caddy fmt`

---

### 2. `import_handler.go` - Lines Needing Coverage

**Changed Code Block (Lines 264-275): Normalization in `Upload()`**

```go
// Normalize Caddyfile format before saving (handles single-line format)
normalizedContent := req.Content                              // Line 265
if normalized, err := h.importerservice.NormalizeCaddyfile(req.Content); err != nil {
    // If normalization fails, log warning but continue with original content
    middleware.GetRequestLogger(c).WithError(err).Warn("Import Upload: Caddyfile normalization failed, using original content")  // ❌ MISSING (line 269)
} else {
    normalizedContent = normalized                            // ❌ MISSING (line 271)
}
```

**Missing Coverage:**
- Line 269: Normalization error path (logs warning, uses original content)
- Line 271: Normalization success path (uses normalized content)
- Line 289: `normalizedContent` used in `WriteFile` - implicitly tested if either path above is covered

---

## Test Plan

### Test File: `backend/internal/caddy/importer_test.go`

#### Test Case 1: Timeout Branch in DefaultExecutor

**Purpose:** Cover lines 36-38 (timeout error path)

```go
func TestDefaultExecutor_Execute_Timeout(t *testing.T) {
    executor := &DefaultExecutor{}

    // Use "sleep 10" to trigger 5s timeout
    output, err := executor.Execute("sleep", "10")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "command timed out after 5 seconds")
    assert.Contains(t, err.Error(), "sleep")
}
```

**Mock Requirements:** None (uses real exec)
**Expected Assertions:**
- Error is returned
- Error message contains "command timed out"
- Output may be empty or partial

#### Test Case 2: NormalizeCaddyfile - File Read Error After Format

**Purpose:** Cover line ~152 (ReadFile error after caddy fmt)

```go
func TestImporter_NormalizeCaddyfile_ReadError(t *testing.T) {
    importer := NewImporter("caddy")

    // Mock executor that succeeds but deletes the temp file
    mockExecutor := &MockExecutor{
        ExecuteFunc: func(name string, args ...string) ([]byte, error) {
            // Delete the temp file before returning
            // This simulates a race or permission issue
            if len(args) > 2 {
                os.Remove(args[2]) // Remove the temp file path
            }
            return []byte{}, nil
        },
    }
    importer.executor = mockExecutor

    _, err := importer.NormalizeCaddyfile("test.local { reverse_proxy localhost:8080 }")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to read formatted file")
}
```

**Mock Requirements:** Custom MockExecutor with ExecuteFunc field
**Expected Assertions:**
- Error returned when file can't be read after formatting
- Error message contains "failed to read formatted file"

---

### Test File: `backend/internal/api/handlers/import_handler_test.go`

#### Test Case 3: Upload - Normalization Success Path

**Purpose:** Cover line 271 (success path where `normalizedContent` is assigned)

```go
func TestUpload_NormalizationSuccess(t *testing.T) {
    db := setupImportTestDB(t)
    handler := setupImportHandler(t, db)

    // Use single-line Caddyfile format (triggers normalization)
    singleLineCaddyfile := `test.local { reverse_proxy localhost:3000 }`

    req := ImportUploadRequest{
        Content:  singleLineCaddyfile,
        Filename: "Caddyfile",
    }
    body, _ := json.Marshal(req)

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest("POST", "/api/v1/import/upload", bytes.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    handler.Upload(c)

    // Should succeed with 200 (normalization worked)
    assert.Equal(t, http.StatusOK, w.Code)

    // Verify response contains hosts (parsing succeeded)
    var response ImportPreview
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.NotNil(t, response.Preview)
}
```

**Mock Requirements:** None (uses real Caddy binary if available, else mock executor)
**Expected Assertions:**
- HTTP 200 response
- Preview contains parsed hosts

#### Test Case 4: Upload - Normalization Failure Path (Falls Back to Original)

**Purpose:** Cover line 269 (error path where normalization fails but upload continues)

```go
func TestUpload_NormalizationFallback(t *testing.T) {
    db := setupImportTestDB(t)

    // Create handler with mock importer that fails normalization
    mockImporter := &MockImporterService{
        NormalizeCaddyfileFunc: func(content string) (string, error) {
            return "", errors.New("caddy fmt failed")
        },
        // ... other methods return normal values
    }
    handler := NewImportHandler(db, mockImporter, "/tmp/import")

    // Valid Caddyfile that would parse successfully
    caddyfile := `test.local {
    reverse_proxy localhost:3000
}`

    req := ImportUploadRequest{
        Content:  caddyfile,
        Filename: "Caddyfile",
    }
    body, _ := json.Marshal(req)

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest("POST", "/api/v1/import/upload", bytes.NewReader(body))
    c.Request.Header.Set("Content-Type", "application/json")

    handler.Upload(c)

    // Should still succeed (falls back to original content)
    assert.Equal(t, http.StatusOK, w.Code)

    // Verify hosts were parsed from original content
    var response ImportPreview
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Greater(t, len(response.Preview.Hosts), 0)
}
```

**Mock Requirements:**
- `MockImporterService` interface with controllable `NormalizeCaddyfile` behavior
- Alternatively, inject a mock executor that returns error for `caddy fmt`

**Expected Assertions:**
- HTTP 200 response (graceful fallback)
- Hosts parsed from original (non-normalized) content

---

## Implementation Checklist

- [ ] Add `TestDefaultExecutor_Execute_Timeout` to `importer_test.go`
- [ ] Add `TestImporter_NormalizeCaddyfile_ReadError` to `importer_test.go` (requires MockExecutor enhancement)
- [ ] Add `TestUpload_NormalizationSuccess` to `import_handler_test.go`
- [ ] Add `TestUpload_NormalizationFallback` to `import_handler_test.go` (requires mock importer interface)

## Mock Interface Requirements

### Option A: Enhance MockExecutor with Function Field

```go
type MockExecutor struct {
    Output      []byte
    Err         error
    ExecuteFunc func(name string, args ...string) ([]byte, error) // NEW
}

func (m *MockExecutor) Execute(name string, args ...string) ([]byte, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(name, args...)
    }
    return m.Output, m.Err
}
```

### Option B: Create MockImporterService for Handler Tests

```go
type MockImporterService struct {
    NormalizeCaddyfileFunc func(content string) (string, error)
    ParseCaddyfileFunc     func(path string) ([]byte, error)
    // ... other methods
}

func (m *MockImporterService) NormalizeCaddyfile(content string) (string, error) {
    return m.NormalizeCaddyfileFunc(content)
}
```

---

## Complexity Estimate

| Test Case | Complexity | Notes |
|-----------|------------|-------|
| Timeout test | Low | Uses real `sleep` command |
| ReadError test | Medium | Requires MockExecutor enhancement |
| Normalization success | Low | May already be covered by existing tests |
| Normalization fallback | Medium | Requires mock importer interface |

**Total Effort:** ~2 hours

---

## Acceptance Criteria

1. All 4 test cases pass locally
2. Codecov patch coverage for both files reaches 100%
3. No new regressions in existing test suite
4. Tests are deterministic (no flaky timeouts)
