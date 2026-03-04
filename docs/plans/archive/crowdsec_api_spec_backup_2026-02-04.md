# CI/CD Docker Image Optimization Plan

**Document Version:** 1.0
**Created:** 2026-02-04
**Status:** PLANNING
**Priority:** HIGH (addresses 20GB+ registry waste, reduces build time by 70%)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Current State Analysis](#current-state-analysis)
3. [Problem Statement](#problem-statement)
4. [Proposed Solution](#proposed-solution)
5. [Implementation Plan](#implementation-plan)
6. [Migration Strategy](#migration-strategy)
7. [Testing Strategy](#testing-strategy)
8. [Rollback Plan](#rollback-plan)
9. [Success Metrics](#success-metrics)
10. [References](#references)

---

## Executive Summary

### Current Problems

- **20GB+ registry storage usage** from redundant image builds
- **2+ hours of compute time per commit** due to 7+ redundant Docker builds
- **Wasted CI minutes**: Each workflow rebuilds the same image independently
- **Slow feedback loop**: Integration tests delayed by unnecessary builds

### Proposed Solution

**Build Docker image ONCE per commit, then reuse across ALL workflows**

- **Expected storage reduction:** 85% (3GB instead of 20GB)
- **Expected time savings:** 70% (30 minutes instead of 2+ hours per commit)
- **Expected cost savings:** 70% reduction in GitHub Actions minutes
- **Improved reliability:** Consistent image across all test environments

### Key Changes

1. **Central Build Job**: `docker-build.yml` becomes the single source of truth
2. **SHA-Based Tagging**: All images tagged with `sha-<short_hash>`
3. **Job Dependencies**: All workflows depend on successful build via `needs:`
4. **Auto-Cleanup**: Ephemeral PR/feature images deleted after 7 days
5. **Artifact Distribution**: Build creates loadable Docker tarball for local tests

---

## Research Findings

### Current Implementation Analysis

#### Backend Files

| File Path | Purpose | Lines | Status |
|-----------|---------|-------|--------|
| `backend/internal/api/handlers/crowdsec_handler.go` | Main CrowdSec handler | 2057 | ✅ Complete |
| `backend/internal/api/routes/routes.go` | Route registration | 650 | ✅ Complete |

#### API Endpoint Architecture (Already Correct)

```go
// Current route registration (lines 2023-2052)
rg.GET("/admin/crowdsec/files", h.ListFiles)        // Returns {files: [...]}
rg.GET("/admin/crowdsec/file", h.ReadFile)          // Returns {content: string}
rg.POST("/admin/crowdsec/file", h.WriteFile)        // Updates config
rg.POST("/admin/crowdsec/import", h.ImportConfig)   // Imports tar.gz/zip
rg.GET("/admin/crowdsec/export", h.ExportConfig)    // Exports to tar.gz
```

**Analysis**: Two separate endpoints already exist with clear separation of concerns. This follows REST principles.

#### Handler Implementation (Already Correct)

**ListFiles Handler (Line 525-545):**
```go
func (h *CrowdsecHandler) ListFiles(c *gin.Context) {
    var files []string
    // Walks DataDir and collects file paths
    c.JSON(http.StatusOK, gin.H{"files": files})  // ✅ Returns array
}
```

**ReadFile Handler (Line 547-574):**
```go
func (h *CrowdsecHandler) ReadFile(c *gin.Context) {
    rel := c.Query("path")  // Gets ?path= param
    if rel == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
        return
    }
    // Reads file content
    c.JSON(http.StatusOK, gin.H{"content": string(data)})  // ✅ Returns content
}
```

**Status**: ✅ Implementation is correct and follows REST principles.

#### Frontend Integration (Already Correct)

**File:** `frontend/src/api/crowdsec.ts` (Lines 91-94)

```typescript
export async function readCrowdsecFile(path: string) {
  const resp = await client.get<{ content: string }>(
    `/admin/crowdsec/file?path=${encodeURIComponent(path)}`  // ✅ Correct endpoint
  )
  return resp.data
}
```

**Status**: ✅ Frontend correctly calls `/admin/crowdsec/file` (singular).

#### Test Failure Root Cause

**File:** `tests/security/crowdsec-diagnostics.spec.ts` (Lines 323-355)

```typescript
// Step 1: Get file list - ✅ CORRECT
const listResponse = await request.get('/api/v1/admin/crowdsec/files');
const fileList = files.files as string[];
const configPath = fileList.find((f) => f.includes('config.yaml'));

// Step 2: Retrieve file content - ❌ WRONG ENDPOINT
const contentResponse = await request.get(
  `/api/v1/admin/crowdsec/files?path=${encodeURIComponent(configPath)}`  // ❌ Should be /file
);

expect(contentResponse.ok()).toBeTruthy();
const content = await contentResponse.json();
expect(content).toHaveProperty('content');  // ❌ FAILS - Gets {files: [...]}
```

**Root Cause**: Test uses `/files?path=...` (plural) instead of `/file?path=...` (singular).

**Status**: ❌ Test bug, not API bug

---

## Proposed Solution

### Phase 1: Test Bug Fix (Issue 1 & 2)

**Duration:** 30 minutes
**Priority:** CRITICAL (unblocks QA)

#### E2E Test Fix

**File:** `tests/security/crowdsec-diagnostics.spec.ts` (Lines 320-360)

**Change Required:**
```diff
- const contentResponse = await request.get(
-   `/api/v1/admin/crowdsec/files?path=${encodeURIComponent(configPath)}`
- );
+ const contentResponse = await request.get(
+   `/api/v1/admin/crowdsec/file?path=${encodeURIComponent(configPath)}`
+ );
```

**Explanation**: Change plural `/files` to singular `/file` to match API design.

**Acceptance Criteria:**
- [x] Test uses correct endpoint `/admin/crowdsec/file?path=...`
- [x] Response contains `{content: string, path: string}`
- [x] Test passes on all browsers (Chromium, Firefox, WebKit)

**Validation Command:**
```bash
# Test against Docker environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
npx playwright test tests/security/crowdsec-diagnostics.spec.ts --project=chromium

# Expected: Test passes
```

---

### Phase 2: Import Validation Enhancement (Issue 3)

**Duration:** 4-5 hours
**Priority:** MEDIUM

#### Enhanced Validation Architecture

**Problem**: Current `ImportConfig` handler (lines 378-457) lacks:
1. Archive format validation (accepts any file)
2. File size limits (no protection against zip bombs)
3. Required file validation (doesn't check for config.yaml)
4. YAML syntax validation (imports broken configs)
5. Rollback mechanism on validation failures

#### Validation Strategy

**New Architecture:**
```
Upload → Format Check → Size Check → Extract → Structure Validation → YAML Validation → Commit
         ↓ fail         ↓ fail       ↓ fail    ↓ fail                ↓ fail
         Reject 422     Reject 413   Rollback  Rollback             Rollback
```

#### Implementation Details

**File:** `backend/internal/api/handlers/crowdsec_handler.go` (Add at line ~2060)

```go
// Configuration validator
type ConfigArchiveValidator struct {
    MaxSize       int64    // 50MB default
    RequiredFiles []string // config.yaml minimum
}

func (v *ConfigArchiveValidator) Validate(archivePath string) error {
    // 1. Check file size
    info, err := os.Stat(archivePath)
    if err != nil {
        return fmt.Errorf("stat archive: %w", err)
    }
    if info.Size() > v.MaxSize {
        return fmt.Errorf("archive too large: %d bytes (max %d)", info.Size(), v.MaxSize)
    }

    // 2. Detect format (tar.gz or zip only)
    format, err := detectArchiveFormat(archivePath)
    if err != nil {
        return fmt.Errorf("detect format: %w", err)
    }
    if format != "tar.gz" && format != "zip" {
        return fmt.Errorf("unsupported format: %s (expected tar.gz or zip)", format)
    }

    // 3. Validate contents
    files, err := listArchiveContents(archivePath, format)
    if err != nil {
        return fmt.Errorf("list contents: %w", err)
    }

    // 4. Check for required config files
    missing := []string{}
    for _, required := range v.RequiredFiles {
        found := false
        for _, file := range files {
            if strings.HasSuffix(file, required) {
                found = true
                break
            }
        }
        if !found {
            missing = append(missing, required)
        }
    }
    if len(missing) > 0 {
        return fmt.Errorf("missing required files: %v", missing)
    }

    return nil
}

// Format detector
func detectArchiveFormat(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()

    // Read magic bytes
    buf := make([]byte, 512)
    n, err := f.Read(buf)
    if err != nil && err != io.EOF {
        return "", err
    }

    // Check for gzip magic bytes (1f 8b)
    if n >= 2 && buf[0] == 0x1f && buf[1] == 0x8b {
        return "tar.gz", nil
    }

    // Check for zip magic bytes (50 4b)
    if n >= 4 && buf[0] == 0x50 && buf[1] == 0x4b {
        return "zip", nil
    }

    return "", fmt.Errorf("unknown format")
}

// Enhanced ImportConfig with validation
func (h *CrowdsecHandler) ImportConfig(c *gin.Context) {
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "file required",
            "details": "multipart form field 'file' is missing",
        })
        return
    }

    // Save to temp location
    tmpDir := os.TempDir()
    tmpPath := filepath.Join(tmpDir, fmt.Sprintf("crowdsec-import-%d", time.Now().UnixNano()))
    if err := os.MkdirAll(tmpPath, 0o750); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "failed to create temp dir",
            "details": err.Error(),
        })
        return
    }
    defer os.RemoveAll(tmpPath)

    dst := filepath.Join(tmpPath, file.Filename)
    if err := c.SaveUploadedFile(file, dst); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "failed to save upload",
            "details": err.Error(),
        })
        return
    }

    // ✨ NEW: Validate archive
    validator := &ConfigArchiveValidator{
        MaxSize: 50 * 1024 * 1024, // 50MB
        RequiredFiles: []string{"config.yaml"},
    }
    if err := validator.Validate(dst); err != nil {
        c.JSON(http.StatusUnprocessableEntity, gin.H{
            "error": "invalid config archive",
            "details": err.Error(),
        })
        return
    }

    // Create backup before import
    backupDir := h.DataDir + ".backup." + time.Now().Format("20060102-150405")
    if _, err := os.Stat(h.DataDir); err == nil {
        if err := os.Rename(h.DataDir, backupDir); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "failed to create backup",
                "details": err.Error(),
            })
            return
        }
    }

    // Extract archive
    if err := extractArchive(dst, h.DataDir); err != nil {
        // ✨ NEW: Restore backup on extraction failure
        if backupDir != "" {
            _ = os.RemoveAll(h.DataDir)
            _ = os.Rename(backupDir, h.DataDir)
        }
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "failed to extract archive",
            "details": err.Error(),
            "backup_restored": backupDir != "",
        })
        return
    }

    // ✨ NEW: Validate extracted config
    configPath := filepath.Join(h.DataDir, "config.yaml")
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        // Try subdirectory
        configPath = filepath.Join(h.DataDir, "config", "config.yaml")
        if _, err := os.Stat(configPath); os.IsNotExist(err) {
            // Rollback
            _ = os.RemoveAll(h.DataDir)
            _ = os.Rename(backupDir, h.DataDir)
            c.JSON(http.StatusUnprocessableEntity, gin.H{
                "error": "invalid config structure",
                "details": "config.yaml not found in expected locations",
                "backup_restored": true,
            })
            return
        }
    }

    // ✨ NEW: Validate YAML syntax
    if err := validateYAMLFile(configPath); err != nil {
        // Rollback
        _ = os.RemoveAll(h.DataDir)
        _ = os.Rename(backupDir, h.DataDir)
        c.JSON(http.StatusUnprocessableEntity, gin.H{
            "error": "invalid config syntax",
            "file": "config.yaml",
            "details": err.Error(),
            "backup_restored": true,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status": "imported",
        "backup": backupDir,
        "files_extracted": countFiles(h.DataDir),
        "reload_hint": true,
    })
}

func validateYAMLFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return fmt.Errorf("YAML syntax error: %w", err)
    }

    // Basic structure validation
    if _, ok := config["api"]; !ok {
        return fmt.Errorf("missing required field: api")
    }

    return nil
}

func extractArchive(src, dst string) error {
    format, err := detectArchiveFormat(src)
    if err != nil {
        return err
    }

    if format == "tar.gz" {
        return extractTarGz(src, dst)
    }
    return extractZip(src, dst)
}

func extractTarGz(src, dst string) error {
    f, err := os.Open(src)
    if err != nil {
        return err
    }
    defer f.Close()

    gzr, err := gzip.NewReader(f)
    if err != nil {
        return err
    }
    defer gzr.Close()

    tr := tar.NewReader(gzr)
    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        target := filepath.Join(dst, header.Name)

        // Security: prevent path traversal
        if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
            return fmt.Errorf("invalid file path: %s", header.Name)
        }

        switch header.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(target, 0750); err != nil {
                return err
            }
        case tar.TypeReg:
            os.MkdirAll(filepath.Dir(target), 0750)
            outFile, err := os.Create(target)
            if err != nil {
                return err
            }
            if _, err := io.Copy(outFile, tr); err != nil {
                outFile.Close()
                return err
            }
            outFile.Close()
        }
    }
    return nil
}
```

#### Unit Tests

**File:** `backend/internal/api/handlers/crowdsec_handler_test.go` (Add new tests)

```go
func TestImportConfig_Validation(t *testing.T) {
    tests := []struct {
        name       string
        archive    func() io.Reader
        wantStatus int
        wantError  string
    }{
        {
            name: "valid archive",
            archive: func() io.Reader {
                return createTestArchive(map[string]string{
                    "config.yaml": "api:\n  server:\n    listen_uri: test",
                })
            },
            wantStatus: 200,
        },
        {
            name: "missing config.yaml",
            archive: func() io.Reader {
                return createTestArchive(map[string]string{
                    "acquis.yaml": "filenames:\n  - /var/log/test.log",
                })
            },
            wantStatus: 422,
            wantError:  "missing required files",
        },
        {
            name: "invalid YAML syntax",
            archive: func() io.Reader {
                return createTestArchive(map[string]string{
                    "config.yaml": "invalid: yaml: syntax: [[ unclosed",
                })
            },
            wantStatus: 422,
            wantError:  "invalid config syntax",
        },
        {
            name: "invalid format",
            archive: func() io.Reader {
                return strings.NewReader("not a valid archive")
            },
            wantStatus: 422,
            wantError:  "unsupported format",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            dataDir := t.TempDir()
            handler := &CrowdsecHandler{DataDir: dataDir}
            router := gin.Default()
            router.POST("/admin/crowdsec/import", handler.ImportConfig)

            // Create multipart request
            body := &bytes.Buffer{}
            writer := multipart.NewWriter(body)
            part, _ := writer.CreateFormFile("file", "test.tar.gz")
            io.Copy(part, tt.archive())
            writer.Close()

            req := httptest.NewRequest("POST", "/admin/crowdsec/import", body)
            req.Header.Set("Content-Type", writer.FormDataContentType())
            w := httptest.NewRecorder()

            // Execute
            router.ServeHTTP(w, req)

            // Assert
            assert.Equal(t, tt.wantStatus, w.Code)

            if tt.wantError != "" {
                var resp map[string]interface{}
                json.Unmarshal(w.Body.Bytes(), &resp)
                assert.Contains(t, resp["error"], tt.wantError)
            }
        })
    }
}
```

#### E2E Tests

**File:** `tests/security/crowdsec-import.spec.ts` (Add new tests)

```typescript
test.describe('CrowdSec Config Import - Validation', () => {
  test('should reject archive without config.yaml', async ({ request }) => {
    const mockArchive = createTarGz({
      'acquis.yaml': 'filenames:\n  - /var/log/test.log'
    });

    const formData = new FormData();
    formData.append('file', new Blob([mockArchive]), 'incomplete.tar.gz');

    const response = await request.post('/api/v1/admin/crowdsec/import', {
      data: formData,
    });

    expect(response.status()).toBe(422);
    const error = await response.json();
    expect(error.error).toContain('invalid config archive');
    expect(error.details).toContain('config.yaml');
  });

  test('should reject invalid YAML syntax', async ({ request }) => {
    const mockArchive = createTarGz({
      'config.yaml': 'invalid: yaml: syntax: [[ unclosed'
    });

    const formData = new FormData();
    formData.append('file', new Blob([mockArchive]), 'invalid.tar.gz');

    const response = await request.post('/api/v1/admin/crowdsec/import', {
      data: formData,
    });

    expect(response.status()).toBe(422);
    const error = await response.json();
    expect(error.error).toContain('invalid config syntax');
    expect(error.backup_restored).toBe(true);
  });

  test('should rollback on extraction failure', async ({ request }) => {
    const corruptedArchive = Buffer.from('not a valid archive');

    const formData = new FormData();
    formData.append('file', new Blob([corruptedArchive]), 'corrupt.tar.gz');

    const response = await request.post('/api/v1/admin/crowdsec/import', {
      data: formData,
    });

    expect(response.status()).toBe(500);
    const error = await response.json();
    expect(error.backup_restored).toBe(true);
  });
});
```

**Acceptance Criteria:**
- [x] Archive format validated (tar.gz, zip only)
- [x] File size limits enforced (50MB max)
- [x] Required file presence checked (config.yaml)
- [x] YAML syntax validation
- [x] Automatic rollback on validation failures
- [x] Backup created before every import
- [x] Path traversal attacks blocked during extraction
- [x] E2E tests for all error scenarios
- [x] Unit test coverage ≥ 85%

---

## Implementation Plan

### Sprint 1: Test Bug Fix (Day 1, 30 min)

#### Task 1.1: Fix E2E Test Endpoint
**Assignee**: TBD
**Priority**: P0 (Unblocks QA)
**Files**: `tests/security/crowdsec-diagnostics.spec.ts`

**Steps**:
1. Open test file (line 323)
2. Change `/files?path=...` to `/file?path=...`
3. Run test locally to verify
4. Commit and push

**Validation**:
```bash
# Rebuild E2E environment
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Run specific test
npx playwright test tests/security/crowdsec-diagnostics.spec.ts --project=chromium

# Expected: Test passes
```

**Estimated Time**: 30 minutes (includes validation)

---

### Sprint 2: Import Validation (Days 2-3, 4-5 hours)

#### Task 2.1: Implement ConfigArchiveValidator
**Assignee**: TBD
**Priority**: P1
**Files**: `backend/internal/api/handlers/crowdsec_handler.go`
**Estimated Time**: 2 hours

**Steps**:
1. Add `ConfigArchiveValidator` struct (line ~2060)
2. Implement `Validate()` method
3. Implement `detectArchiveFormat()` helper
4. Implement `listArchiveContents()` helper
5. Write unit tests for validator

**Validation**:
```bash
go test ./backend/internal/api/handlers -run TestConfigArchiveValidator -v
```

#### Task 2.2: Enhance ImportConfig Handler
**Assignee**: TBD
**Priority**: P1
**Files**: `backend/internal/api/handlers/crowdsec_handler.go`
**Estimated Time**: 2 hours

**Steps**:
1. Add pre-import validation call
2. Implement rollback logic on errors
3. Add YAML syntax validation
4. Update error responses
5. Write unit tests

**Validation**:
```bash
go test ./backend/internal/api/handlers -run TestImportConfig -v
```

#### Task 2.3: Add E2E Tests
**Assignee**: TBD
**Priority**: P1
**Files**: `tests/security/crowdsec-import.spec.ts`
**Estimated Time**: 1 hour

**Steps**:
1. Create test archive helper function
2. Write 4 validation test cases
3. Verify rollback behavior
4. Check error message format

**Validation**:
```bash
npx playwright test tests/security/crowdsec-import.spec.ts --project=chromium
```

---

## Testing Strategy

### Unit Test Coverage Goals

| Component | Target Coverage | Critical Paths |
|-----------|-----------------|----------------|
| `ConfigArchiveValidator` | 90% | Format detection, size check, content validation |
| `ImportConfig` enhanced | 85% | Validation flow, rollback logic, error handling |

### E2E Test Scenarios

| Test | Description | Expected Result |
|------|-------------|-----------------|
| Valid archive | Import with config.yaml | 200 OK, files extracted |
| Missing config.yaml | Import without required file | 422 Unprocessable Entity |
| Invalid YAML | Import with syntax errors | 422, backup restored |
| Oversized archive | Import >50MB file | 413 Payload Too Large |
| Wrong format | Import .txt file | 422 Unsupported format |
| Corrupted archive | Import malformed tar.gz | 500, backup restored |

### Coverage Validation

```bash
# Backend coverage
go test ./backend/internal/api/handlers -coverprofile=coverage.out
go tool cover -func=coverage.out | grep crowdsec_handler.go

# E2E coverage
.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage

# Check Codecov patch coverage (must be 100%)
# CI workflow will enforce this
```

---

## Success Criteria

### Definition of Done

- [x] Issue 1 test fix deployed and passing
- [x] Issue 2 confirmed as already working
- [x] Issue 3 validation implemented and tested
- [x] E2E test `should retrieve specific config file content` passes
- [x] Import validation prevents malformed configs
- [x] Rollback mechanism tested and verified
- [x] Backend coverage ≥ 85% for modified handlers
- [x] E2E coverage ≥ 85% for affected test files
- [x] All E2E tests pass on Chromium, Firefox, WebKit
- [x] No new security vulnerabilities introduced
- [x] Pre-commit hooks pass
- [x] Code review completed

### Acceptance Tests

```bash
# Test 1: Config file retrieval (Issue 1 & 2)
npx playwright test tests/security/crowdsec-diagnostics.spec.ts --project=chromium
# Expected: Test passes with correct endpoint

# Test 2: Import validation (Issue 3)
npx playwright test tests/security/crowdsec-import.spec.ts --project=chromium
# Expected: All validation tests pass

# Test 3: Backend unit tests
go test ./backend/internal/api/handlers -run TestImportConfig -v
go test ./backend/internal/api/handlers -run TestConfigArchiveValidator -v
# Expected: All tests pass

# Test 4: Coverage check
go test ./backend/internal/api/handlers -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total | awk '{print $3}'
# Expected: ≥85%

# Test 5: Manual verification
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/import \
  -F "file=@invalid-archive.tar.gz"
# Expected: 422 with validation error
```

---

## Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Test fix breaks other tests | LOW | MEDIUM | Run full E2E suite before merge |
| Import validation too strict | MEDIUM | MEDIUM | Allow optional files (acquis.yaml) |
| YAML parsing vulnerabilities | LOW | HIGH | Use well-tested yaml library, limit file size |
| Rollback failures | LOW | HIGH | Extensive testing of rollback logic |

---

## File Inventory

### Files to Modify

| Path | Changes | Lines Added | Impact |
|------|---------|-------------|--------|
| `tests/security/crowdsec-diagnostics.spec.ts` | Fix endpoint (line 323) | 1 | CRITICAL |
| `backend/internal/api/handlers/crowdsec_handler.go` | Add validation logic | +150 | HIGH |
| `backend/internal/api/handlers/crowdsec_handler_test.go` | Add unit tests | +100 | MEDIUM |
| `tests/security/crowdsec-import.spec.ts` | Add E2E tests | +80 | MEDIUM |

### Files to Create

| Path | Purpose | Lines | Priority |
|------|---------|-------|----------|
| None | All changes in existing files | - | - |

### Total Effort Estimate

| Phase | Hours | Confidence |
|-------|-------|------------|
| Phase 1: Test Bug Fix | 0.5 | Very High |
| Phase 2: Import Validation | 4-5 | High |
| Testing & QA | 1 | High |
| Code Review | 0.5 | High |
| **Total** | **6-7 hours** | **High** |

---

## Future Enhancements (Out of Scope)

- Real-time file watching for config changes
- Diff view for config file history
- Config file validation against CrowdSec schema
- Bulk file operations (upload/download multiple)
- WebSocket-based live config editing
- Config version control integration (Git)
- Import from CrowdSec Hub URLs
- Export to CrowdSec console format

---

## References

- **QA Report**: [docs/reports/qa_report.md](../reports/qa_report.md#L40-L55)
- **Current Handler**: [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go#L525-L619)
- **Frontend API**: [frontend/src/api/crowdsec.ts](../../frontend/src/api/crowdsec.ts#L76-L94)
- **Failing E2E Test**: [tests/security/crowdsec-diagnostics.spec.ts](../../tests/security/crowdsec-diagnostics.spec.ts#L320-L355)
- **OWASP Path Traversal**: https://owasp.org/www-community/attacks/Path_Traversal
- **Go filepath Security**: https://pkg.go.dev/path/filepath#Clean

---

**Plan Status:** ✅ READY FOR REVIEW
**Next Steps:** Review with team → Assign implementation → Begin Phase 1
