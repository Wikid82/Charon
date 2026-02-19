# Caddy Import Debug E2E Test Specification

**Version:** 1.1
**Status:** POC Ready
**Priority:** HIGH
**Created:** 2026-01-30
**Updated:** 2026-01-30 (Critical fixes applied)
**Target:** Issue 2 from Reddit Feedback - "import from my caddy is not working"

---

## Executive Summary

This specification defines Playwright E2E tests designed to **expose failure modes** in the Caddy import functionality. These tests are intentionally written to FAIL initially, revealing the exact root causes of import issues reported by users.

**Goal:** Create diagnostic tests that surface backend errors, API response issues, and UI feedback gaps.

---

## Research Summary

### Implementation Architecture

**Flow:** Frontend → Backend Handler → Caddy Importer → caddy CLI

```
ImportCaddy.tsx (upload)
    ↓
POST /api/v1/import/upload
    ↓
import_handler.go (Upload)
    ↓
importer.go (ParseCaddyfile)
    ↓
exec caddy adapt --config <file>
    ↓
ExtractHosts
    ↓
Return preview
```

### Key Files

| File | Purpose | Lines of Interest |
|------|---------|-------------------|
| `backend/internal/api/handlers/import_handler.go` | API endpoints for import | 243-318 (Upload), 507-524 (detectImportDirectives) |
| `backend/internal/caddy/importer.go` | Caddy JSON parsing | 86-103 (ParseCaddyfile), 175-244 (ExtractHosts), 315-329 (ConvertToProxyHosts) |
| `frontend/src/pages/ImportCaddy.tsx` | UI for import wizard | 28-52 (handleUpload), 54-67 (handleFileUpload) |
| `tests/tasks/import-caddyfile.spec.ts` | Existing E2E tests | Full file - uses mocked API responses |

### Known Issues from Code Analysis

1. **Import Directives Not Resolved** - If Caddyfile contains `import ./sites.d/*`, hosts in those files are ignored unless user uses multi-file flow
2. **Silent Host Skipping** - Hosts without `reverse_proxy` are skipped with no user feedback
3. **Cryptic Parse Errors** - `caddy adapt` errors are passed through verbatim without context
4. **No Error Log Capture** - Backend logs errors but frontend doesn't display diagnostic info
5. **File Server Warnings Hidden** - `file_server` directives add warnings but may not show in UI

---

## Test Strategy

### Philosophy

**DO NOT mock the API.** Real integration tests against the running backend will expose:
- Backend parsing errors
- Caddy CLI execution issues
- Database transaction failures
- Error message formatting problems
- UI error display gaps

### Critical Pattern Corrections

**1. Authentication:**
- ❌ **WRONG:** `await loginUser(page, adminUser)` in each test
- ✅ **RIGHT:** Rely on stored auth state from `auth.setup.ts`
- **Rationale:** Existing tests in `tests/tasks/import-caddyfile.spec.ts` use the authenticated storage state automatically. Tests inherit the session.

**2. Race Condition Prevention:**
- ❌ **WRONG:** Setting up `waitForResponse()` after or simultaneously with click
- ✅ **RIGHT:** Register `waitForResponse()` BEFORE triggering the action
```typescript
// CORRECT PATTERN:
const responsePromise = page.waitForResponse(...);
await parseButton.click(); // Action happens after promise is set up
const apiResponse = await responsePromise;
```

**3. Backend Log Capture:**
- ❌ **WRONG:** Manual terminal watching in separate session
- ✅ **RIGHT:** Programmatic Docker API capture in test hooks
```typescript
import { exec } from 'child_process';
import { promisify } from 'util';
const execAsync = promisify(exec);

test.afterEach(async ({ }, testInfo) => {
  if (testInfo.status !== 'passed') {
    const { stdout } = await execAsync(
      'docker logs charon-app 2>&1 | grep -i import | tail -50'
    );
    testInfo.attach('backend-logs', {
      body: stdout,
      contentType: 'text/plain'
    });
  }
});
```

**4. Pre-Test Health Check:**
```typescript
test.beforeAll(async ({ baseURL }) => {
  const healthResponse = await fetch(`${baseURL}/health`);
  if (!healthResponse.ok) {
    throw new Error('Charon container not running or unhealthy');
  }
});
```

### Test Environment

**Requirements:**
- Docker container running on `http://localhost:8080`
- Real Caddy binary available at `/usr/bin/caddy` in container
- SQLite database with real schema
- Frontend served from container (not Vite dev server for these tests)
- Authenticated storage state from global setup

**Setup:**
```bash
# Start Charon container
docker-compose up -d

# Verify container health
curl http://localhost:8080/health

# Run tests (auth state auto-loaded)
npx playwright test tests/tasks/caddy-import-debug.spec.ts --project=chromium
```

---

## Diagnostic Test Cases

### Test 1: Simple Valid Caddyfile (Baseline) ✅ POC

**Objective:** Verify the happy path works correctly and establish baseline behavior.

**Expected Result:** ✅ Should PASS (if basic import is functional)

**Status:** 🎯 **POC Implementation - Test This First**

**Caddyfile:**
```caddyfile
test-simple.example.com {
    reverse_proxy localhost:3000
}
```

**Test Implementation (WITH ALL CRITICAL FIXES APPLIED):**
```typescript
import { test, expect } from '@playwright/test';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

test.describe('Caddy Import Debug Tests @caddy-import-debug', () => {
  // CRITICAL FIX #4: Pre-test health check
  test.beforeAll(async ({ baseURL }) => {
    const healthResponse = await fetch(`${baseURL}/health`);
    if (!healthResponse.ok) {
      throw new Error('Charon container not running or unhealthy');
    }
  });

  // CRITICAL FIX #3: Programmatic backend log capture
  test.afterEach(async ({ }, testInfo) => {
    if (testInfo.status !== 'passed') {
      try {
        const { stdout } = await execAsync(
          'docker logs charon-app 2>&1 | grep -i import | tail -50'
        );
        testInfo.attach('backend-logs', {
          body: stdout,
          contentType: 'text/plain'
        });
      } catch (error) {
        console.warn('Failed to capture backend logs:', error);
      }
    }
  });

  test.describe('Baseline', () => {
    test('should successfully import a simple valid Caddyfile', async ({ page }) => {
      // CRITICAL FIX #1: No loginUser() call - auth state auto-loaded from storage

      await page.goto('/tasks/import/caddyfile');

      const caddyfile = `
test-simple.example.com {
    reverse_proxy localhost:3000
}
      `.trim();

      // Step 1: Paste content
      await page.locator('textarea').fill(caddyfile);

      // Step 2: Set up response waiter BEFORE clicking (CRITICAL FIX #2)
      const parseButton = page.getByRole('button', { name: /parse|review/i });

      // CRITICAL FIX #2: Race condition prevention - register promise FIRST
      const responsePromise = page.waitForResponse(response =>
        response.url().includes('/api/v1/import/upload') && response.status() === 200
      );

      // NOW trigger the action
      await parseButton.click();
      const apiResponse = await responsePromise;

      // Step 3: Log full API response for debugging
      const responseBody = await apiResponse.json();
      console.log('API Response:', JSON.stringify(responseBody, null, 2));

      // Step 4: Verify preview shows host
      await expect(page.getByText('test-simple.example.com')).toBeVisible({ timeout: 10000 });

      // Step 5: Verify host details are correct
      await expect(page.getByText('localhost:3000')).toBeVisible();
    });
  });
});
```

**Critical Fixes Applied:**
1. ✅ **Authentication:** Removed `loginUser()` call - uses stored auth state
2. ✅ **Race Condition:** `waitForResponse()` registered BEFORE `click()`
3. ✅ **Log Capture:** Programmatic Docker API in `afterEach()` hook
4. ✅ **Health Check:** `beforeAll()` validates container is running

**Diagnostic Value:** If this fails, the entire import pipeline is broken. With all fixes applied, this test should reliably detect actual backend/frontend issues, not test infrastructure problems.

---

### Test 2: Caddyfile with Import Directives

**Objective:** Expose the import directive handling - should show appropriate error/guidance.

**Expected Result:** ⚠️ May FAIL if error message is unclear or missing

**Caddyfile (Main):**
```caddyfile
import sites.d/*.caddy

admin.example.com {
    reverse_proxy localhost:9090
}
```

**Test Implementation:**
```typescript
test('should detect import directives and provide actionable error', async ({ page }) => {
  // Auth state loaded from storage - no login needed
  await page.goto('/tasks/import/caddyfile');

  const caddyfileWithImports = `
import sites.d/*.caddy

admin.example.com {
    reverse_proxy localhost:9090
}
  `.trim();

  // Paste content with import directive
  await page.locator('textarea').fill(caddyfileWithImports);

  // Click parse and capture response (FIX: waitForResponse BEFORE click)
  const parseButton = page.getByRole('button', { name: /parse|review/i });

  // Register response waiter FIRST
  const responsePromise = page.waitForResponse(response =>
    response.url().includes('/api/v1/import/upload')
  );
  // THEN trigger action
  await parseButton.click();
  const apiResponse = await responsePromise;

  // Log status and response body
  const status = apiResponse.status();
  const responseBody = await apiResponse.json();
  console.log('API Status:', status);
  console.log('API Response:', JSON.stringify(responseBody, null, 2));

  // Check if backend detected import directives
  if (responseBody.imports && responseBody.imports.length > 0) {
    console.log('✅ Backend detected imports:', responseBody.imports);
  } else {
    console.warn('❌ Backend did NOT detect import directives');
  }

  // Verify user-facing error message
  const errorMessage = page.locator('.bg-red-900, .bg-red-900\\/20');
  await expect(errorMessage).toBeVisible({ timeout: 5000 });

  // Check error text is actionable
  const errorText = await errorMessage.textContent();
  console.log('Error message displayed to user:', errorText);

  // Should mention "import" and guide to multi-file flow
  expect(errorText?.toLowerCase()).toContain('import');
  expect(errorText?.toLowerCase()).toMatch(/multi.*file|upload.*files|include.*files/);
});
```

**Diagnostic Value:**
- Confirms `detectImportDirectives()` function works
- Verifies error response includes `imports` field
- Checks if UI displays actionable guidance

---

### Test 3: Caddyfile with No Reverse Proxy (File Server Only)

**Objective:** Expose silent host skipping - should inform user which hosts were ignored.

**Expected Result:** ⚠️ May FAIL if no feedback about skipped hosts

**Caddyfile:**
```caddyfile
static.example.com {
    file_server
    root * /var/www/html
}

docs.example.com {
    file_server browse
    root * /var/www/docs
}
```

**Test Implementation:**
```typescript
test('should provide feedback when all hosts are file servers (not reverse proxies)', async ({ page }) => {
  // Auth state loaded from storage
  await page.goto('/tasks/import/caddyfile');

  const fileServerCaddyfile = `
static.example.com {
    file_server
    root * /var/www/html
}

docs.example.com {
    file_server browse
    root * /var/www/docs
}
  `.trim();

  // Paste file server config
  await page.locator('textarea').fill(fileServerCaddyfile);

  // Parse and capture API response (FIX: register waiter first)
  const responsePromise = page.waitForResponse(response =>
    response.url().includes('/api/v1/import/upload')
  );

  await page.getByRole('button', { name: /parse|review/i }).click();
  const apiResponse = await responsePromise;

  const status = apiResponse.status();
  const responseBody = await apiResponse.json();
  console.log('API Status:', status);
  console.log('API Response:', JSON.stringify(responseBody, null, 2));

  // Check if preview.hosts is empty
  if (responseBody.preview?.hosts?.length === 0) {
    console.log('✅ Backend correctly parsed 0 hosts');
  } else {
    console.warn('❌ Backend unexpectedly returned hosts:', responseBody.preview?.hosts);
  }

  // Check if warnings exist for unsupported features
  if (responseBody.preview?.hosts?.some((h: any) => h.warnings?.length > 0)) {
    console.log('✅ Backend included warnings:', responseBody.preview.hosts[0].warnings);
  } else {
    console.warn('❌ Backend did NOT include warnings about file_server');
  }

  // Verify user-facing error/warning
  const warningMessage = page.locator('.bg-yellow-900, .bg-yellow-900\\/20, .bg-red-900');
  await expect(warningMessage).toBeVisible({ timeout: 5000 });

  const warningText = await warningMessage.textContent();
  console.log('Warning/Error displayed:', warningText);

  // Should mention "file server" or "not supported" or "no sites found"
  expect(warningText?.toLowerCase()).toMatch(/file.?server|not supported|no (sites|hosts|domains) found/);
});
```

**Diagnostic Value:**
- Confirms hosts with no `reverse_proxy` are correctly skipped
- Checks if warnings are surfaced in API response
- Verifies UI displays meaningful feedback (not just "no hosts found")

---

### Test 4: Caddyfile with Invalid Syntax

**Objective:** Expose how parse errors from `caddy adapt` are surfaced to the user.

**Expected Result:** ⚠️ May FAIL if error message is cryptic

**Caddyfile:**
```caddyfile
broken.example.com {
    reverse_proxy localhost:3000
    this is invalid syntax
    another broken line
}
```

**Test Implementation:**
```typescript
test('should provide clear error message for invalid Caddyfile syntax', async ({ page }) => {
  // Auth state loaded from storage
  await page.goto('/tasks/import/caddyfile');

  const invalidCaddyfile = `
broken.example.com {
    reverse_proxy localhost:3000
    this is invalid syntax
    another broken line
}
  `.trim();

  // Paste invalid content
  await page.locator('textarea').fill(invalidCaddyfile);

  // Parse and capture response (FIX: waiter before click)
  const responsePromise = page.waitForResponse(response =>
    response.url().includes('/api/v1/import/upload')
  );

  await page.getByRole('button', { name: /parse|review/i }).click();
  const apiResponse = await responsePromise;

  const status = apiResponse.status();
  const responseBody = await apiResponse.json();
  console.log('API Status:', status);
  console.log('API Error Response:', JSON.stringify(responseBody, null, 2));

  // Should be 400 Bad Request
  expect(status).toBe(400);

  // Check error message structure
  if (responseBody.error) {
    console.log('✅ Backend returned error:', responseBody.error);

    // Check if error mentions "caddy adapt" output
    if (responseBody.error.includes('caddy adapt failed')) {
      console.log('✅ Error includes caddy adapt context');
    } else {
      console.warn('⚠️ Error does NOT mention caddy adapt failure');
    }

    // Check if error includes line number hint
    if (/line \d+/i.test(responseBody.error)) {
      console.log('✅ Error includes line number reference');
    } else {
      console.warn('⚠️ Error does NOT include line number');
    }
  } else {
    console.error('❌ No error field in response body');
  }

  // Verify UI displays error
  const errorMessage = page.locator('.bg-red-900, .bg-red-900\\/20');
  await expect(errorMessage).toBeVisible({ timeout: 5000 });

  const errorText = await errorMessage.textContent();
  console.log('User-facing error:', errorText);

  // Error should be actionable
  expect(errorText?.length).toBeGreaterThan(10); // Not just "error"
});
```

**Diagnostic Value:**
- Verifies `caddy adapt` error output is captured
- Checks if backend enhances error with line numbers or suggestions
- Confirms UI displays full error context (not truncated)

---

### Test 5: Caddyfile with Mixed Content (Valid + Unsupported)

**Objective:** Test partial import scenario - some hosts valid, some skipped/warned.

**Expected Result:** ⚠️ May FAIL if skipped hosts not communicated

**Caddyfile:**
```caddyfile
# Valid reverse proxy
api.example.com {
    reverse_proxy localhost:8080
}

# File server (should be skipped)
static.example.com {
    file_server
    root * /var/www
}

# Valid reverse proxy with WebSocket
ws.example.com {
    reverse_proxy localhost:9000 {
        header_up Upgrade websocket
    }
}

# Redirect (should be warned/skipped)
redirect.example.com {
    redir https://other.example.com{uri}
}
```

**Test Implementation:**
```typescript
test('should handle mixed valid/invalid hosts and provide detailed feedback', async ({ page }) => {
  // Auth state loaded from storage
  await page.goto('/tasks/import/caddyfile');

  const mixedCaddyfile = `
# Valid reverse proxy
api.example.com {
    reverse_proxy localhost:8080
}

# File server (should be skipped)
static.example.com {
    file_server
    root * /var/www
}

# Valid reverse proxy with WebSocket
ws.example.com {
    reverse_proxy localhost:9000 {
        header_up Upgrade websocket
    }
}

# Redirect (should be warned)
redirect.example.com {
    redir https://other.example.com{uri}
}
  `.trim();

  // Paste mixed content
  await page.locator('textarea').fill(mixedCaddyfile);

  // Parse and capture response (FIX: waiter registered first)
  const responsePromise = page.waitForResponse(response =>
    response.url().includes('/api/v1/import/upload')
  );

  await page.getByRole('button', { name: /parse|review/i }).click();
  const apiResponse = await responsePromise;

  const responseBody = await apiResponse.json();
  console.log('API Response:', JSON.stringify(responseBody, null, 2));

  // Analyze what was parsed
  const hosts = responseBody.preview?.hosts || [];
  console.log(`Parsed ${hosts.length} hosts:`, hosts.map((h: any) => h.domain_names));

  // Should find 2 valid reverse proxies (api + ws)
  expect(hosts.length).toBeGreaterThanOrEqual(2);

  // Check if static.example.com is in list (should NOT be, or should have warning)
  const staticHost = hosts.find((h: any) => h.domain_names === 'static.example.com');
  if (staticHost) {
    console.warn('⚠️ static.example.com was included:', staticHost);
    expect(staticHost.warnings).toBeDefined();
    expect(staticHost.warnings.length).toBeGreaterThan(0);
  } else {
    console.log('✅ static.example.com correctly excluded');
  }

  // Check if redirect host has warnings
  const redirectHost = hosts.find((h: any) => h.domain_names === 'redirect.example.com');
  if (redirectHost) {
    console.log('ℹ️ redirect.example.com included:', redirectHost);
  }

  // Verify UI shows all importable hosts
  await expect(page.getByText('api.example.com')).toBeVisible();
  await expect(page.getByText('ws.example.com')).toBeVisible();

  // Check if warnings are displayed
  const warningElements = page.locator('.text-yellow-400, .bg-yellow-900');
  const warningCount = await warningElements.count();
  console.log(`UI displays ${warningCount} warning indicators`);
});
```

**Diagnostic Value:**
- Tests parser's ability to handle heterogeneous configs
- Verifies warnings for unsupported directives are included
- Confirms UI distinguishes between importable and skipped hosts

---

### Test 6: Import Directive with Multi-File Upload

**Objective:** Test the multi-file upload flow that SHOULD work for imports.

**Expected Result:** ✅ Should PASS if multi-file implementation is correct

**Files:**
- Main Caddyfile: `import sites.d/*.caddy`
- Site file: `sites.d/app.caddy` with reverse_proxy

**Test Implementation:**
```typescript
test('should successfully import Caddyfile with imports using multi-file upload', async ({ page }) => {
  // Auth state loaded from storage
  await page.goto('/tasks/import/caddyfile');

  // Main Caddyfile
  const mainCaddyfile = `
import sites.d/app.caddy

admin.example.com {
    reverse_proxy localhost:9090
}
  `.trim();

  // Site file
  const siteCaddyfile = `
app.example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    reverse_proxy localhost:8080
}
  `.trim();

  // Click multi-file import button
  await page.getByRole('button', { name: /multi.*file|multi.*site/i }).click();

  // Wait for modal to open
  const modal = page.locator('[role="dialog"], .modal, [data-testid="multi-site-modal"]');
  await expect(modal).toBeVisible({ timeout: 5000 });

  // Prepare files for upload
  const files = [
    {
      filename: 'Caddyfile',
      content: mainCaddyfile,
    },
    {
      filename: 'sites.d/app.caddy',
      content: siteCaddyfile,
    },
  ];

  // Find the file input within modal
  const fileInput = modal.locator('input[type="file"]');

  // Upload both files (may need to upload separately)
  for (const file of files) {
    await fileInput.setInputFiles({
      name: file.filename,
      mimeType: 'text/plain',
      buffer: Buffer.from(file.content),
    });
  }

  // Click upload/parse button in modal (FIX: waiter first)
  const uploadButton = modal.getByRole('button', { name: /upload|parse|submit/i });

  // Register response waiter BEFORE clicking
  const responsePromise = page.waitForResponse(response =>
    response.url().includes('/api/v1/import/upload-multi')
  );

  await uploadButton.click();
  const apiResponse = await responsePromise;

  const responseBody = await apiResponse.json();
  console.log('Multi-file API Response:', JSON.stringify(responseBody, null, 2));

  // Should parse all 3 hosts (admin.example.com, app.example.com, api.example.com)
  const hosts = responseBody.preview?.hosts || [];
  console.log(`Parsed ${hosts.length} hosts from multi-file import`);
  expect(hosts.length).toBe(3);

  // Verify review table shows all 3
  await expect(page.getByText('admin.example.com')).toBeVisible({ timeout: 10000 });
  await expect(page.getByText('app.example.com')).toBeVisible();
  await expect(page.getByText('api.example.com')).toBeVisible();
});
```

**Diagnostic Value:**
- Tests the proper solution for import directives
- Verifies backend correctly resolves imports when files are uploaded together
- Confirms UI workflow for multi-file uploads

---

## POC Implementation Plan 🎯

### Phase 1: Test 1 Only (POC Validation)

**Objective:** Validate that the corrected test patterns work against the live Docker container.

**Steps:**
1. Create `tests/tasks/caddy-import-debug.spec.ts` with ONLY Test 1
2. Implement all 4 critical fixes (auth, race condition, logs, health check)
3. Run against Docker container: `npx playwright test tests/tasks/caddy-import-debug.spec.ts`
4. Verify:
   - Test authenticates correctly without `loginUser()`
   - No race conditions in API response capture
   - Backend logs attached on failure
   - Health check passes before tests run

**Success Criteria:**
- ✅ Test 1 PASSES if import pipeline works
- ✅ Test 1 FAILS with clear diagnostics if import is broken
- ✅ Backend logs captured automatically on failure
- ✅ No test infrastructure issues (auth, timing, etc.)

### Phase 2: Expand to All Tests (If POC Succeeds)

Once Test 1 validates the pattern:
1. Copy the test structure (hooks, imports, patterns)
2. Implement Tests 2-6 using the same corrected patterns
3. Run full suite
4. Analyze failures to identify backend/frontend issues

**If POC Fails:**
- Review Playwright trace
- Check backend logs attachment
- Verify Docker container health
- Debug Test 1 until it works reliably
- DO NOT proceed to other tests until POC is stable

---

## Running the Tests

### Execute POC (Test 1 Only)

```bash
# Verify container is running
curl http://localhost:8080/health

# Run Test 1 only
npx playwright test tests/tasks/caddy-import-debug.spec.ts -g "should successfully import a simple valid Caddyfile" --project=chromium

# With headed browser to watch
npx playwright test tests/tasks/caddy-import-debug.spec.ts -g "simple valid" --headed --project=chromium

# With trace for debugging
npx playwright test tests/tasks/caddy-import-debug.spec.ts -g "simple valid" --trace on --project=chromium
```

### Execute All Debug Tests (After POC Success)

```bash
# Run full suite with full output (no truncation!)
npx playwright test tests/tasks/caddy-import-debug.spec.ts --project=chromium

# With headed browser to watch failures
npx playwright test tests/tasks/caddy-import-debug.spec.ts --headed --project=chromium

# With trace for detailed debugging
npx playwright test tests/tasks/caddy-import-debug.spec.ts --trace on --project=chromium
```

### Backend Log Capture

**Automatic (Recommended):**
Backend logs are automatically captured by the `test.afterEach()` hook on test failure and attached to the Playwright report.

**Manual (For Live Debugging):**
```bash
# Watch logs in separate terminal while tests run
docker logs -f charon-app 2>&1 | grep -i "import"
```

### Debugging Individual Tests

```bash
# Run single test with debug UI
npx playwright test --debug -g "should detect import directives"

# Run with verbose API logging
DEBUG=pw:api npx playwright test -g "should provide clear error"

# Check test report for backend logs
npx playwright show-report
```

---

## Expected Test Results

### Initial Run (Before Fixes)

| Test | Expected Status | Diagnostic Goal |
|------|----------------|-----------------|
| Test 1: Simple Valid | ✅ PASS | Confirm baseline works |
| Test 2: Import Directives | ⚠️ MAY FAIL | Check error message clarity |
| Test 3: File Servers Only | ⚠️ MAY FAIL | Check user feedback about skipped hosts |
| Test 4: Invalid Syntax | ⚠️ MAY FAIL | Check error message usefulness |
| Test 5: Mixed Content | ⚠️ MAY FAIL | Check partial import feedback |
| Test 6: Multi-File | ✅ SHOULD PASS | Verify proper solution works |

### After Fixes (Target)

All tests should PASS with clear, actionable error messages logged in console output.

---

## Error Pattern Analysis

### Backend Error Capture Points

**File:** `backend/internal/api/handlers/import_handler.go`

Add enhanced logging at these points:

**Line 243 (Upload start):**
```go
middleware.GetRequestLogger(c).WithField("content_len", len(req.Content)).Info("Import Upload: received upload")
```

**Line 292 (Parse failure):**
```go
middleware.GetRequestLogger(c).WithError(err).WithField("content_preview", util.SanitizeForLog(preview)).Error("Import Upload: import failed")
```

**Line 297 (Import detection):**
```go
middleware.GetRequestLogger(c).WithField("imports", sanitizedImports).Warn("Import Upload: no hosts parsed but imports detected")
```

### Frontend Error Display

**File:** `frontend/src/pages/ImportCaddy.tsx`

Error display component (lines 54-58):
```tsx
{error && (
  <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
    {error}
  </div>
)}
```

**Improvement needed:** Display structured error data (imports array, warnings, skipped hosts).

---

## Test File Location

**New File:** `tests/tasks/caddy-import-debug.spec.ts`

**Rationale:**
- Located in `tests/tasks/` to match existing test pattern (see `import-caddyfile.spec.ts`)
- Uses same authentication patterns as other task tests
- Can be run independently with `--grep @caddy-import-debug`
- Tagged for selective execution but part of main suite
- Will be integrated into CI once baseline (Test 1) is proven stable

**Test File Structure (POC Version):**
```typescript
import { test, expect } from '@playwright/test';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

test.describe('Caddy Import Debug Tests @caddy-import-debug', () => {
  // Pre-flight health check
  test.beforeAll(async ({ baseURL }) => {
    const healthResponse = await fetch(`${baseURL}/health`);
    if (!healthResponse.ok) {
      throw new Error('Charon container not running or unhealthy');
    }
  });

  // Automatic backend log capture on failure
  test.afterEach(async ({ }, testInfo) => {
    if (testInfo.status !== 'passed') {
      try {
        const { stdout } = await execAsync(
          'docker logs charon-app 2>&1 | grep -i import | tail -50'
        );
        testInfo.attach('backend-logs', {
          body: stdout,
          contentType: 'text/plain'
        });
      } catch (error) {
        console.warn('Failed to capture backend logs:', error);
      }
    }
  });

  test.describe('Baseline', () => {
    // Test 1 - POC implementation with all critical fixes
  });

  // Remaining test groups implemented after POC succeeds
  test.describe('Import Directives', () => {
    // Test 2
  });

  test.describe('Unsupported Features', () => {
    // Test 3, 5
  });

  test.describe('Parse Errors', () => {
    // Test 4
  });

  test.describe('Multi-File Flow', () => {
    // Test 6
  });
});
```

---

## Success Criteria

### Phase 1: POC Validation (This Spec - Test 1 Only)

- [ ] Test 1 implemented with all 4 critical fixes applied
- [ ] Test runs successfully against Docker container without auth errors
- [ ] No race conditions in API response capture
- [ ] Backend logs automatically attached on failure
- [ ] Health check validates container state before tests
- [ ] Test location matches existing pattern (`tests/tasks/`)
- [ ] POC either:
  - ✅ PASSES - confirming baseline import works, OR
  - ❌ FAILS - with clear diagnostics exposing the actual bug

### Phase 2: Full Diagnostic Suite (After POC Success)

- [ ] All 6 debug tests implemented using POC patterns
- [ ] Tests capture and log full API request/response
- [ ] Tests capture backend logs automatically via hooks
- [ ] Failure points clearly identified with console output
- [ ] Root cause documented for each failure

### Phase 3: Implementation Fixes (Follow-Up Spec)

- [ ] Backend returns structured error responses with:
  - `imports` array when detected
  - `skipped_hosts` array with reasons
  - `warnings` array for unsupported features
  - Enhanced error messages for parse failures
- [ ] Frontend displays all error details meaningfully
- [ ] All 6 debug tests PASS
- [ ] User documentation updated with troubleshooting guide

---

## Next Steps

### Immediate: POC Implementation

1. **Create test file** at `tests/tasks/caddy-import-debug.spec.ts`
2. **Implement Test 1 ONLY** with all 4 critical fixes:
   - ✅ No `loginUser()` - use stored auth state
   - ✅ `waitForResponse()` BEFORE `click()`
   - ✅ Programmatic Docker log capture in `afterEach()`
   - ✅ Health check in `beforeAll()`
3. **Run POC test** against Docker container:
   ```bash
   npx playwright test tests/tasks/caddy-import-debug.spec.ts -g "simple valid" --project=chromium
   ```
4. **Validate POC**:
   - If PASS → Import pipeline works, proceed to Phase 2
   - If FAIL → Analyze diagnostics, fix root cause, repeat

### After POC Success: Full Suite

5. **Implement Tests 2-6** using same patterns from Test 1
6. **Run full suite** against running Docker container
7. **Analyze failures** - capture all console output, backend logs, API responses
8. **Document findings** in GitHub issue or follow-up spec
9. **Implement fixes** based on test failures
10. **Re-run tests** until all PASS
11. **Add to CI** once stable (enabled by default, use `@caddy-import-debug` tag for selective runs)

---

## References

- **Related Spec:** [docs/plans/reddit_feedback_spec.md](./reddit_feedback_spec.md) - Issue 2 requirements
- **Backend Handler:** [backend/internal/api/handlers/import_handler.go](../../backend/internal/api/handlers/import_handler.go)
- **Caddy Importer:** [backend/internal/caddy/importer.go](../../backend/internal/caddy/importer.go)
- **Frontend UI:** [frontend/src/pages/ImportCaddy.tsx](../../frontend/src/pages/ImportCaddy.tsx)
- **Existing Tests:** [tests/tasks/import-caddyfile.spec.ts](../../tests/tasks/import-caddyfile.spec.ts)
- **Import Guide:** [docs/import-guide.md](../import-guide.md)

---

**END OF SPECIFICATION**
