# E2E Test Failure Analysis

**Date:** 2026-02-04
**Purpose:** Investigate 4 failing Playwright E2E tests and determine root causes and fixes.

---

## Summary

| Test | File:Line | Category | Root Cause |
|------|-----------|----------|------------|
| Invalid YAML returns 500 | crowdsec-import.spec.ts:95 | **Backend Bug** | `validateYAMLFile()` uses `json.Unmarshal` on YAML data |
| Error message mismatch | crowdsec-import.spec.ts:128 | **Test Bug** | Test regex doesn't match actual backend error message |
| Path traversal backup error | crowdsec-import.spec.ts:333 | **Test Bug** | Test archive helper may not preserve `../` paths + test regex |
| admin_whitelist undefined | zzzz-break-glass-recovery.spec.ts:177 | **Test Bug** | Test accesses `body.admin_whitelist` instead of `body.config.admin_whitelist` |

---

## Detailed Analysis

### 1. Invalid YAML Returns 500 Instead of 422

**Test Location:** [crowdsec-import.spec.ts](../../tests/security/crowdsec-import.spec.ts#L95)

**Test Purpose:** Verifies that archives containing malformed YAML in `config.yaml` are rejected with HTTP 422.

**Test Input:**
```yaml
invalid: yaml: syntax: here:
  unclosed: [bracket
  bad indentation
no proper structure
```

**Expected:** HTTP 422 with error matching `/yaml|syntax|invalid/`
**Actual:** HTTP 500 Internal Server Error

**Root Cause:** Backend bug in `validateYAMLFile()` function.

**Backend Handler:** [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go#L255-L270)

```go
func validateYAMLFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }

    // BUG: Uses json.Unmarshal on YAML data!
    var config map[string]interface{}
    if err := json.Unmarshal(data, &config); err != nil {
        // Falls through to string-contains check
        content := string(data)
        if !strings.Contains(content, "api:") && !strings.Contains(content, "server:") {
            return fmt.Errorf("invalid CrowdSec config structure")
        }
    }
    return nil
}
```

The function uses `json.Unmarshal()` to parse YAML data, which is incorrect. YAML is a superset of JSON, so valid YAML will fail JSON parsing unless it happens to also be valid JSON.

**Fix Required:** (Backend)

```go
import "gopkg.in/yaml.v3"

func validateYAMLFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }

    // Use proper YAML parsing
    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil {
        return fmt.Errorf("invalid YAML syntax: %w", err)
    }

    // Check for required CrowdSec fields
    if _, hasAPI := config["api"]; !hasAPI {
        return fmt.Errorf("missing required field: api")
    }

    return nil
}
```

**Dependency:** Requires adding `gopkg.in/yaml.v3` to imports in crowdsec_handler.go.

---

### 2. Error Message Pattern Mismatch

**Test Location:** [crowdsec-import.spec.ts](../../tests/security/crowdsec-import.spec.ts#L128)

**Test Purpose:** Verifies that archives with valid YAML but missing required CrowdSec fields (like `api.server.listen_uri`) are rejected with HTTP 422.

**Test Input:**
```yaml
other_config:
  field: value
  nested:
    key: data
```

**Expected Pattern:** `/api.server.listen_uri|required field|missing field/`
**Actual Message:** `"config validation failed: invalid CrowdSec config structure"`

**Root Cause:** The backend error message doesn't match the test's expected pattern.

The current `validateYAMLFile()` returns:
```go
return fmt.Errorf("invalid CrowdSec config structure")
```

This doesn't contain any of: `api.server.listen_uri`, `required field`, `missing field`.

**Fix Options:**

**Option A: Update Test** (Simpler)
```typescript
// THEN: Import fails with structure validation error
expect(response.status()).toBe(422);
const data = await response.json();
expect(data.error).toBeDefined();
expect(data.error.toLowerCase()).toMatch(/api.server.listen_uri|required field|missing field|invalid.*config.*structure/);
```

**Option B: Update Backend** (Better user experience)
Update `validateYAMLFile()` to return more specific errors:
```go
if _, hasAPI := config["api"]; !hasAPI {
    return fmt.Errorf("missing required field: api.server.listen_uri")
}
```

**Recommendation:** Fix the backend (Option B) as part of fixing Issue #1. This provides better error messages to users and aligns with the test expectations.

---

### 3. Path Traversal Shows Backup Error

**Test Location:** [crowdsec-import.spec.ts](../../tests/security/crowdsec-import.spec.ts#L333)

**Test Purpose:** Verifies that archives containing path traversal attempts (e.g., `../../../etc/passwd`) are rejected.

**Test Input:**
```typescript
{
  'config.yaml': `api:\n  server:\n    listen_uri: 0.0.0.0:8080\n`,
  '../../../etc/passwd': 'malicious content',
}
```

**Expected:** HTTP 422 or 500 with error matching `/path|security|invalid/`
**Actual:** Error message may contain "backup" instead of path/security/invalid

**Root Cause:** Multiple potential issues:

1. **Archive Helper Issue:** The `createTarGz()` helper in [archive-helpers.ts](../../tests/utils/archive-helpers.ts) writes files to a temp directory before archiving:
   ```typescript
   for (const [filename, content] of Object.entries(files)) {
     const filePath = path.join(tempDir, filename);
     await fs.mkdir(path.dirname(filePath), { recursive: true });
     await fs.writeFile(filePath, content, 'utf-8');
   }
   ```

   Writing to `path.join(tempDir, '../../../etc/passwd')` may cause the file to be written outside the temp directory rather than being included in the archive with that literal name. The `tar` library may then not preserve the `../` prefix.

2. **Backend Path Detection:** The path traversal is detected during extraction at [crowdsec_handler.go#L691](../../backend/internal/api/handlers/crowdsec_handler.go#L691):
   ```go
   if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
       return fmt.Errorf("invalid file path: %s", header.Name)
   }
   ```

   This returns `"extraction failed: invalid file path: ../../../etc/passwd"` which SHOULD match the regex `/path|security|invalid/` since it contains both "path" and "invalid".

3. **Environment Setup:** If the test environment doesn't have the CrowdSec data directory properly initialized, the backup step could fail:
   ```go
   if err := os.Rename(h.DataDir, backupDir); err != nil {
       c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup"})
       return
   }
   ```

**Investigation Needed:**

1. Verify the archive actually contains `../../../etc/passwd` as a filename (not as a resolved path)
2. Check if the E2E test environment has proper CrowdSec data directory setup
3. Review the actual error message returned by running the test

**Fix Options:**

**Option A: Fix Archive Helper** (Recommended)
Update `createTarGz()` to preserve path traversal filenames by using a different approach:

```typescript
// Use tar-stream library to create archives with arbitrary header names
import tar from 'tar-stream';

export async function createTarGzWithRawPaths(
  files: Record<string, string>,
  outputPath: string
): Promise<string> {
  const pack = tar.pack();

  for (const [filename, content] of Object.entries(files)) {
    pack.entry({ name: filename }, content);
  }

  pack.finalize();

  // Pipe through gzip and write to file
  // ...
}
```

**Option B: Update Test Regex** (Partial fix)
Add "backup" to the acceptable error patterns if that's truly expected:
```typescript
expect(data.error.toLowerCase()).toMatch(/path|security|invalid|backup/);
```

**Note:** This is a security test, so the actual behavior should be validated before changing the test expectations.

---

### 4. admin_whitelist Undefined

**Test Location:** [zzzz-break-glass-recovery.spec.ts](../../tests/security-enforcement/zzzz-break-glass-recovery.spec.ts#L177)

**Test Purpose:** Verifies that the admin whitelist was successfully set to `0.0.0.0/0` for universal bypass.

**Test Code:**
```typescript
await test.step('Verify admin whitelist is set to 0.0.0.0/0', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/config`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.admin_whitelist).toBe('0.0.0.0/0');  // <-- BUG
});
```

**Expected:** Test passes
**Actual:** `body.admin_whitelist` is undefined

**Root Cause:** The API response wraps the config in a `config` object.

**Backend Handler:** [security_handler.go#L205](../../backend/internal/api/handlers/security_handler.go#L205-L215)

```go
func (h *SecurityHandler) GetConfig(c *gin.Context) {
    cfg, err := h.svc.Get()
    if err != nil {
        // error handling...
    }
    c.JSON(http.StatusOK, gin.H{"config": cfg})  // Wrapped in "config"
}
```

**API Response Structure:**
```json
{
  "config": {
    "uuid": "...",
    "name": "default",
    "admin_whitelist": "0.0.0.0/0",
    ...
  }
}
```

**Fix Required:** (Test)

```typescript
await test.step('Verify admin whitelist is set to 0.0.0.0/0', async () => {
  const response = await request.get(`${BASE_URL}/api/v1/security/config`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.config?.admin_whitelist).toBe('0.0.0.0/0');  // Fixed: access nested property
});
```

---

## Fix Priority

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| 1 | #4 - admin_whitelist access | 1 line change | Unblocks break-glass recovery test |
| 2 | #1 - YAML parsing | Medium | Core functionality bug |
| 3 | #2 - Error message | 1 line change | Test alignment |
| 4 | #3 - Path traversal archive | Medium | Security test reliability |

---

## Implementation Tasks

### Task 1: Fix admin_whitelist Test Access
**File:** `tests/security-enforcement/zzzz-break-glass-recovery.spec.ts`
**Change:** Line 177 - Access `body.config.admin_whitelist` instead of `body.admin_whitelist`

### Task 2: Fix YAML Validation in Backend
**File:** `backend/internal/api/handlers/crowdsec_handler.go`
**Changes:**
1. Add import for `gopkg.in/yaml.v3`
2. Replace `json.Unmarshal` with `yaml.Unmarshal` in `validateYAMLFile()`
3. Add proper error messages for missing required fields

### Task 3: Update Error Message Pattern in Test
**File:** `tests/security/crowdsec-import.spec.ts`
**Change:** Line ~148 - Update regex to match backend error or update backend to match test expectation

### Task 4: Investigate Path Traversal Archive Creation
**File:** `tests/utils/archive-helpers.ts`
**Investigation:** Verify that archives with `../` prefixes are created correctly
**Potential Fix:** Use low-level tar creation to set raw header names

---

## Notes

- Issues #1, #2, and #3 are related to the CrowdSec import validation flow
- Issue #4 is completely unrelated (different feature area)
- All issues appear to be **pre-existing** rather than regressions from current PR
- The YAML parsing bug (#1) is the most significant as it affects core functionality
