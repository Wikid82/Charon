# Phase 5: Tasks & Monitoring - Detailed Implementation Plan

**Status:** IN PROGRESS
**Timeline:** Week 9
**Total Estimated Tests:** 92-114 tests across 7 test files
**Last Updated:** 2025-01-XX

---

## Overview

Phase 5 covers backup management, log viewing, import wizards, and monitoring features. This document provides the complete implementation plan with exact file paths, test scenarios, UI selectors, API endpoints, and mock data requirements.

---

## Directory Structure

```
tests/
├── tasks/
│   ├── backups-create.spec.ts      # 17 tests - Backup creation, list, delete, download
│   ├── backups-restore.spec.ts     # 8 tests  - Backup restoration workflows
│   ├── logs-viewing.spec.ts        # 18 tests - Static log file viewing
│   ├── import-caddyfile.spec.ts    # 18 tests - Caddyfile import wizard
│   └── import-crowdsec.spec.ts     # 8 tests  - CrowdSec config import
└── monitoring/
    ├── uptime-monitoring.spec.ts   # 22 tests - Uptime monitor CRUD & sync
    └── real-time-logs.spec.ts      # 20 tests - WebSocket log streaming
```

---

## Implementation Order

Execute in this order based on dependencies and complexity:

| Order | File | Priority | Reason | Depends On |
|-------|------|----------|--------|------------|
| 1 | `backups-create.spec.ts` | P0 | Foundation for restore tests | auth-fixtures |
| 2 | `backups-restore.spec.ts` | P0 | Requires backup data | backups-create |
| 3 | `logs-viewing.spec.ts` | P0 | Static logs, no WebSocket | auth-fixtures |
| 4 | `import-caddyfile.spec.ts` | P1 | Multi-step wizard | auth-fixtures |
| 5 | `import-crowdsec.spec.ts` | P1 | Simpler import flow | auth-fixtures |
| 6 | `uptime-monitoring.spec.ts` | P1 | Monitor CRUD | auth-fixtures |
| 7 | `real-time-logs.spec.ts` | P2 | WebSocket complexity | logs-viewing |

---

## File 1: `tests/tasks/backups-create.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/tasks/backups` | `Backups.tsx` | `frontend/src/pages/Backups.tsx` |

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `GET` | `/api/v1/backups` | List all backups | `BackupFile[]` |
| `POST` | `/api/v1/backups` | Create new backup | `BackupFile` |
| `DELETE` | `/api/v1/backups/:filename` | Delete backup | `204 No Content` |
| `GET` | `/api/v1/backups/:filename/download` | Download backup | Binary file |

### TypeScript Interfaces

```typescript
interface BackupFile {
  filename: string;  // e.g., "backup_2024-01-15_120000.tar.gz"
  size: number;      // Bytes
  time: string;      // ISO timestamp
}
```

### UI Selectors

```typescript
// Page elements
const SELECTORS = {
  // Page shell
  pageTitle: 'h1 >> text=Backups',

  // Create button
  createBackupButton: 'button:has-text("Create Backup")',

  // Backup list (DataTable component)
  backupTable: '[role="table"]',
  backupRows: '[role="row"]',
  emptyState: '[data-testid="empty-state"]',

  // Row actions
  restoreButton: 'button:has-text("Restore")',
  deleteButton: 'button:has-text("Delete")',
  downloadButton: 'button:has([data-icon="download"])',

  // Confirmation dialogs (Dialog component)
  confirmDialog: '[role="dialog"]',
  confirmButton: 'button:has-text("Confirm")',
  cancelButton: 'button:has-text("Cancel")',

  // Settings section (optional)
  retentionInput: 'input[name="retention"]',
  intervalSelect: 'select[name="interval"]',
  saveSettingsButton: 'button:has-text("Save Settings")',

  // Loading states
  loadingSpinner: '[data-testid="loading"]',
  skeleton: '[data-testid="skeleton"]',
};
```

### Test Scenarios (17 tests)

#### Page Layout & Navigation (3 tests)

| # | Test Name | Description | Priority | Auth |
|---|-----------|-------------|----------|------|
| 1 | `should display backups page with correct heading` | Navigate to `/tasks/backups`, verify page title | P0 | admin |
| 2 | `should show Create Backup button for admin users` | Verify button visible for admin role | P0 | admin |
| 3 | `should hide Create Backup button for guest users` | Verify button hidden for guest role | P1 | guest |

```typescript
test.describe('Page Layout', () => {
  test('should display backups page with correct heading', async ({ page }) => {
    await page.goto('/tasks/backups');
    await waitForLoadingComplete(page);
    await expect(page.locator('h1')).toContainText('Backups');
  });

  test('should show Create Backup button for admin users', async ({ page }) => {
    await page.goto('/tasks/backups');
    await expect(page.locator(SELECTORS.createBackupButton)).toBeVisible();
  });
});

test.describe('Guest Access', () => {
  test.use({ ...guestUser });

  test('should hide Create Backup button for guest users', async ({ page }) => {
    await page.goto('/tasks/backups');
    await expect(page.locator(SELECTORS.createBackupButton)).not.toBeVisible();
  });
});
```

#### Backup List Display (4 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 4 | `should display empty state when no backups exist` | Show EmptyState component | P0 |
| 5 | `should display list of existing backups` | Show table with filename, size, time | P0 |
| 6 | `should sort backups by date newest first` | Verify descending order | P1 |
| 7 | `should show loading skeleton while fetching` | Skeleton during API call | P2 |

```typescript
test('should display empty state when no backups exist', async ({ page }) => {
  // Mock empty response
  await page.route('**/api/v1/backups', route => {
    route.fulfill({ status: 200, json: [] });
  });

  await page.goto('/tasks/backups');
  await expect(page.locator(SELECTORS.emptyState)).toBeVisible();
  await expect(page.getByText('No backups found')).toBeVisible();
});

test('should display list of existing backups', async ({ page }) => {
  const mockBackups: BackupFile[] = [
    { filename: 'backup_2024-01-15_120000.tar.gz', size: 1048576, time: '2024-01-15T12:00:00Z' },
    { filename: 'backup_2024-01-14_120000.tar.gz', size: 2097152, time: '2024-01-14T12:00:00Z' },
  ];

  await page.route('**/api/v1/backups', route => {
    route.fulfill({ status: 200, json: mockBackups });
  });

  await page.goto('/tasks/backups');
  await waitForTableLoad(page, page.locator(SELECTORS.backupTable));

  // Verify both backups displayed
  await expect(page.getByText('backup_2024-01-15_120000.tar.gz')).toBeVisible();
  await expect(page.getByText('backup_2024-01-14_120000.tar.gz')).toBeVisible();
});
```

#### Create Backup Flow (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 8 | `should create a new backup successfully` | Click button, verify API call | P0 |
| 9 | `should show success toast after backup creation` | Toast appears with message | P0 |
| 10 | `should update backup list with new backup` | List refreshes after creation | P0 |
| 11 | `should disable create button while in progress` | Button disabled during API call | P1 |
| 12 | `should handle backup creation failure` | Show error toast on 500 | P1 |

```typescript
test('should create a new backup successfully', async ({ page }) => {
  const newBackup = { filename: 'backup_2024-01-16_120000.tar.gz', size: 512000, time: new Date().toISOString() };

  await page.route('**/api/v1/backups', async route => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status: 201, json: newBackup });
    } else {
      await route.continue();
    }
  });

  await page.goto('/tasks/backups');
  await page.click(SELECTORS.createBackupButton);

  await waitForAPIResponse(page, '/api/v1/backups', 201);
  await waitForToast(page, /backup created|success/i);
});

test('should disable create button while in progress', async ({ page }) => {
  // Delay response to observe disabled state
  await page.route('**/api/v1/backups', async route => {
    if (route.request().method() === 'POST') {
      await new Promise(r => setTimeout(r, 500));
      await route.fulfill({ status: 201, json: { filename: 'test.tar.gz', size: 100, time: new Date().toISOString() } });
    } else {
      await route.continue();
    }
  });

  await page.goto('/tasks/backups');
  await page.click(SELECTORS.createBackupButton);

  // Button should be disabled during request
  await expect(page.locator(SELECTORS.createBackupButton)).toBeDisabled();

  // After completion, button should be enabled
  await waitForAPIResponse(page, '/api/v1/backups', 201);
  await expect(page.locator(SELECTORS.createBackupButton)).toBeEnabled();
});
```

#### Delete Backup Flow (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 13 | `should show confirmation dialog before deleting` | Click delete, dialog appears | P0 |
| 14 | `should delete backup after confirmation` | Confirm, verify DELETE call | P0 |
| 15 | `should show success toast after deletion` | Toast after successful delete | P1 |

```typescript
test('should show confirmation dialog before deleting', async ({ page }) => {
  await setupBackupsList(page); // Helper to mock backup list
  await page.goto('/tasks/backups');

  // Click delete on first backup
  await page.locator(SELECTORS.backupRows).first().locator(SELECTORS.deleteButton).click();

  // Verify dialog appears
  await expect(page.locator(SELECTORS.confirmDialog)).toBeVisible();
  await expect(page.getByText(/confirm|are you sure/i)).toBeVisible();
});

test('should delete backup after confirmation', async ({ page }) => {
  const filename = 'backup_2024-01-15_120000.tar.gz';
  let deleteRequested = false;

  await page.route(`**/api/v1/backups/${filename}`, async route => {
    if (route.request().method() === 'DELETE') {
      deleteRequested = true;
      await route.fulfill({ status: 204 });
    }
  });

  await setupBackupsList(page, [{ filename, size: 1000, time: new Date().toISOString() }]);
  await page.goto('/tasks/backups');

  // Trigger delete flow
  await page.locator(SELECTORS.deleteButton).first().click();
  await page.locator(SELECTORS.confirmButton).click();

  await waitForAPIResponse(page, `/api/v1/backups/${filename}`, 204);
  expect(deleteRequested).toBe(true);
});
```

#### Download Backup Flow (2 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 16 | `should download backup file successfully` | Trigger download, verify request | P0 |
| 17 | `should show error toast when download fails` | Handle 404/500 errors | P1 |

```typescript
test('should download backup file successfully', async ({ page }) => {
  const filename = 'backup_2024-01-15_120000.tar.gz';

  // Track download event
  const downloadPromise = page.waitForEvent('download');

  await page.route(`**/api/v1/backups/${filename}/download`, route => {
    route.fulfill({
      status: 200,
      headers: {
        'Content-Type': 'application/gzip',
        'Content-Disposition': `attachment; filename="${filename}"`,
      },
      body: Buffer.from('mock backup content'),
    });
  });

  await setupBackupsList(page, [{ filename, size: 1000, time: new Date().toISOString() }]);
  await page.goto('/tasks/backups');

  await page.locator(SELECTORS.downloadButton).first().click();

  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe(filename);
});
```

---

## File 2: `tests/tasks/backups-restore.spec.ts`

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `POST` | `/api/v1/backups/:filename/restore` | Restore from backup | `{ message: string }` |

### UI Selectors

```typescript
const SELECTORS = {
  // Restore specific
  restoreButton: 'button:has-text("Restore")',

  // Warning dialog (AlertDialog style)
  warningDialog: '[role="alertdialog"]',
  warningMessage: '[data-testid="restore-warning"]',

  // Confirmation input (type backup name)
  confirmationInput: 'input[placeholder*="backup name"]',
  confirmRestoreButton: 'button:has-text("Restore"):not([disabled])',

  // Progress indicator
  progressBar: '[role="progressbar"]',
  restoreStatus: '[data-testid="restore-status"]',
};
```

### Test Scenarios (8 tests)

#### Restore Flow (6 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should show warning dialog before restore` | Click restore, see warning | P0 |
| 2 | `should require explicit confirmation` | Must type backup name | P0 |
| 3 | `should restore backup successfully` | Complete flow, API call | P0 |
| 4 | `should show success toast after restoration` | Toast message | P0 |
| 5 | `should show progress indicator during restore` | Progress bar visible | P1 |
| 6 | `should handle restore failure gracefully` | Error toast on 500 | P1 |

```typescript
test.describe('Restore Flow', () => {
  test.beforeEach(async ({ page }) => {
    await setupBackupsList(page);
    await page.goto('/tasks/backups');
  });

  test('should show warning dialog before restore', async ({ page }) => {
    await page.locator(SELECTORS.restoreButton).first().click();

    await expect(page.locator(SELECTORS.warningDialog)).toBeVisible();
    await expect(page.getByText(/warning|caution|data loss/i)).toBeVisible();
    await expect(page.getByText(/current configuration will be replaced/i)).toBeVisible();
  });

  test('should require explicit confirmation', async ({ page }) => {
    await page.locator(SELECTORS.restoreButton).first().click();

    // Confirm button should be disabled initially
    await expect(page.locator(SELECTORS.confirmRestoreButton)).toBeDisabled();

    // Type backup name to enable
    await page.locator(SELECTORS.confirmationInput).fill('backup_2024-01-15');
    await expect(page.locator(SELECTORS.confirmRestoreButton)).toBeEnabled();
  });

  test('should restore backup successfully', async ({ page }) => {
    const filename = 'backup_2024-01-15_120000.tar.gz';

    await page.route(`**/api/v1/backups/${filename}/restore`, route => {
      route.fulfill({ status: 200, json: { message: 'Restore completed successfully' } });
    });

    await page.locator(SELECTORS.restoreButton).first().click();
    await page.locator(SELECTORS.confirmationInput).fill('backup_2024-01-15');
    await page.locator(SELECTORS.confirmRestoreButton).click();

    await waitForAPIResponse(page, `/api/v1/backups/${filename}/restore`, 200);
    await waitForToast(page, /restore.*success|completed/i);
  });
});
```

#### Post-Restore Verification (2 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 7 | `should reload application state after restore` | App refreshes/reloads | P1 |
| 8 | `should preserve user session after restore` | Stay logged in | P2 |

```typescript
test('should reload application state after restore', async ({ page }) => {
  // Track navigation/reload
  let reloadTriggered = false;
  page.on('load', () => { reloadTriggered = true; });

  await completeRestoreFlow(page);

  // After restore, app should reload or navigate
  await page.waitForTimeout(1000); // Allow reload to trigger
  expect(reloadTriggered).toBe(true);
});
```

---

## File 3: `tests/tasks/logs-viewing.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/tasks/logs` | `Logs.tsx` | `frontend/src/pages/Logs.tsx` |
| - | `LogTable.tsx` | `frontend/src/components/LogTable.tsx` |
| - | `LogFilters.tsx` | `frontend/src/components/LogFilters.tsx` |

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `GET` | `/api/v1/logs` | List log files | `LogFile[]` |
| `GET` | `/api/v1/logs/:filename` | Read log content | `LogResponse` |
| `GET` | `/api/v1/logs/:filename/download` | Download log file | Binary |

### TypeScript Interfaces

```typescript
interface LogFile {
  name: string;
  size: number;
  modified: string;
}

interface LogResponse {
  entries: CaddyAccessLog[];
  total: number;
  page: number;
  limit: number;
}

interface CaddyAccessLog {
  level: string;
  ts: number;
  logger: string;
  msg: string;
  request: {
    remote_ip: string;
    method: string;
    host: string;
    uri: string;
    proto: string;
  };
  status: number;
  duration: number;
  size: number;
}

interface LogFilter {
  search?: string;
  level?: string;
  host?: string;
  status_min?: number;
  status_max?: number;
  sort?: 'asc' | 'desc';
  page?: number;
  limit?: number;
}
```

### UI Selectors

```typescript
const SELECTORS = {
  // Page layout
  pageTitle: 'h1 >> text=Logs',

  // Log file list (sidebar)
  logFileList: '[data-testid="log-file-list"]',
  logFileButton: 'button[data-log-file]',
  selectedLogFile: 'button[data-log-file][data-selected="true"]',

  // Log table
  logTable: '[data-testid="log-table"]',
  logTableRow: '[data-testid="log-entry"]',

  // Filters (LogFilters component)
  searchInput: 'input[placeholder*="Search"]',
  levelSelect: 'select[name="level"]',
  hostFilter: 'input[name="host"]',
  statusMinInput: 'input[name="status_min"]',
  statusMaxInput: 'input[name="status_max"]',
  clearFiltersButton: 'button:has-text("Clear")',

  // Pagination
  prevPageButton: 'button[aria-label="Previous page"]',
  nextPageButton: 'button[aria-label="Next page"]',
  pageInfo: '[data-testid="page-info"]',

  // Sort
  sortByTimestamp: 'th:has-text("Timestamp")',
  sortIndicator: '[data-sort]',

  // Empty state
  emptyLogState: '[data-testid="empty-log"]',
};
```

### Test Scenarios (18 tests)

#### Page Layout (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should display logs page with file selector` | Page loads with sidebar | P0 |
| 2 | `should show list of available log files` | Files listed in sidebar | P0 |
| 3 | `should display log filters section` | Filter inputs visible | P0 |

#### Log File Selection (4 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 4 | `should list all available log files with metadata` | Filename, size, date | P0 |
| 5 | `should load log content when file selected` | Click file, content loads | P0 |
| 6 | `should show empty state for empty log files` | EmptyState component | P1 |
| 7 | `should highlight selected log file` | Visual selection indicator | P1 |

```typescript
test('should list all available log files with metadata', async ({ page }) => {
  const mockFiles: LogFile[] = [
    { name: 'access.log', size: 1048576, modified: '2024-01-15T12:00:00Z' },
    { name: 'error.log', size: 256000, modified: '2024-01-15T11:30:00Z' },
  ];

  await page.route('**/api/v1/logs', route => {
    route.fulfill({ status: 200, json: mockFiles });
  });

  await page.goto('/tasks/logs');

  await expect(page.getByText('access.log')).toBeVisible();
  await expect(page.getByText('error.log')).toBeVisible();
  // Verify size display (formatted)
  await expect(page.getByText(/1.*MB|1048.*KB/i)).toBeVisible();
});

test('should load log content when file selected', async ({ page }) => {
  const mockEntries: CaddyAccessLog[] = [
    {
      level: 'info',
      ts: Date.now() / 1000,
      logger: 'http.log.access',
      msg: 'handled request',
      request: { remote_ip: '192.168.1.1', method: 'GET', host: 'example.com', uri: '/', proto: 'HTTP/2' },
      status: 200,
      duration: 0.05,
      size: 1234,
    },
  ];

  await page.route('**/api/v1/logs/access.log*', route => {
    route.fulfill({ status: 200, json: { entries: mockEntries, total: 1, page: 1, limit: 50 } });
  });

  await setupLogFiles(page);
  await page.goto('/tasks/logs');

  await page.click('button:has-text("access.log")');
  await waitForAPIResponse(page, '/api/v1/logs/access.log', 200);

  // Verify log entry displayed
  await expect(page.getByText('192.168.1.1')).toBeVisible();
  await expect(page.getByText('GET')).toBeVisible();
  await expect(page.getByText('200')).toBeVisible();
});
```

#### Log Content Display (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 8 | `should display log entries in table format` | Table with columns | P0 |
| 9 | `should show timestamp, level, method, uri, status` | Key columns visible | P0 |
| 10 | `should paginate large log files` | Page controls work | P1 |
| 11 | `should sort logs by timestamp` | Click header to sort | P1 |
| 12 | `should highlight error entries` | Red styling for errors | P2 |

```typescript
test('should paginate large log files', async ({ page }) => {
  // Mock paginated response
  let requestedPage = 1;
  await page.route('**/api/v1/logs/access.log*', route => {
    const url = new URL(route.request().url());
    requestedPage = parseInt(url.searchParams.get('page') || '1');

    route.fulfill({
      status: 200,
      json: {
        entries: generateMockEntries(50, requestedPage),
        total: 150,
        page: requestedPage,
        limit: 50,
      },
    });
  });

  await selectLogFile(page, 'access.log');

  // Verify initial page
  await expect(page.locator(SELECTORS.pageInfo)).toContainText('Page 1');

  // Navigate to next page
  await page.click(SELECTORS.nextPageButton);
  await waitForAPIResponse(page, '/api/v1/logs/access.log', 200);

  await expect(page.locator(SELECTORS.pageInfo)).toContainText('Page 2');
  expect(requestedPage).toBe(2);
});
```

#### Log Filtering (6 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 13 | `should filter logs by search text` | Text in message/uri | P0 |
| 14 | `should filter logs by log level` | Level dropdown | P0 |
| 15 | `should filter logs by host` | Host input | P1 |
| 16 | `should filter logs by status code range` | Min/max status | P1 |
| 17 | `should combine multiple filters` | All filters together | P1 |
| 18 | `should clear all filters` | Clear button resets | P1 |

```typescript
test('should filter logs by search text', async ({ page }) => {
  let searchQuery = '';
  await page.route('**/api/v1/logs/access.log*', route => {
    const url = new URL(route.request().url());
    searchQuery = url.searchParams.get('search') || '';
    route.fulfill({ status: 200, json: { entries: [], total: 0, page: 1, limit: 50 } });
  });

  await selectLogFile(page, 'access.log');

  await page.fill(SELECTORS.searchInput, 'api/users');
  await page.keyboard.press('Enter');

  await waitForAPIResponse(page, '/api/v1/logs/access.log', 200);
  expect(searchQuery).toBe('api/users');
});

test('should combine multiple filters', async ({ page }) => {
  let capturedParams: Record<string, string> = {};
  await page.route('**/api/v1/logs/access.log*', route => {
    const url = new URL(route.request().url());
    capturedParams = Object.fromEntries(url.searchParams);
    route.fulfill({ status: 200, json: { entries: [], total: 0, page: 1, limit: 50 } });
  });

  await selectLogFile(page, 'access.log');

  // Apply multiple filters
  await page.fill(SELECTORS.searchInput, 'error');
  await page.selectOption(SELECTORS.levelSelect, 'error');
  await page.fill(SELECTORS.statusMinInput, '400');
  await page.fill(SELECTORS.statusMaxInput, '599');
  await page.keyboard.press('Enter');

  await waitForAPIResponse(page, '/api/v1/logs/access.log', 200);

  expect(capturedParams.search).toBe('error');
  expect(capturedParams.level).toBe('error');
  expect(capturedParams.status_min).toBe('400');
  expect(capturedParams.status_max).toBe('599');
});
```

---

## File 4: `tests/tasks/import-caddyfile.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/tasks/import/caddyfile` | `ImportCaddy.tsx` | `frontend/src/pages/ImportCaddy.tsx` |
| - | `ImportReviewTable.tsx` | `frontend/src/components/ImportReviewTable.tsx` |
| - | `ImportSitesModal.tsx` | `frontend/src/components/ImportSitesModal.tsx` |

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `POST` | `/api/v1/import/upload` | Upload Caddyfile content | `ImportPreview` |
| `POST` | `/api/v1/import/upload-multi` | Upload multiple files | `ImportPreview` |
| `GET` | `/api/v1/import/status` | Get session status | `{ has_pending, session? }` |
| `GET` | `/api/v1/import/preview` | Get parsed preview | `ImportPreview` |
| `POST` | `/api/v1/import/detect-imports` | Detect import directives | `{ imports: string[] }` |
| `POST` | `/api/v1/import/commit` | Commit import | `ImportCommitResult` |
| `DELETE` | `/api/v1/import/cancel` | Cancel session | `204` |

### TypeScript Interfaces

```typescript
interface ImportSession {
  id: string;
  state: 'pending' | 'reviewing' | 'completed' | 'failed' | 'transient';
  created_at: string;
  updated_at: string;
  source_file?: string;
}

interface ImportPreview {
  session: ImportSession;
  preview: {
    hosts: Array<{ domain_names: string; [key: string]: unknown }>;
    conflicts: string[];
    errors: string[];
  };
  caddyfile_content?: string;
  conflict_details?: Record<string, ConflictDetail>;
}

interface ImportCommitResult {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}
```

### UI Selectors

```typescript
const SELECTORS = {
  // Upload section
  fileDropzone: '[data-testid="file-dropzone"]',
  fileInput: 'input[type="file"]',
  pasteTextarea: 'textarea[placeholder*="Paste"]',
  uploadButton: 'button:has-text("Upload")',

  // Import banner (active session)
  importBanner: '[data-testid="import-banner"]',
  continueButton: 'button:has-text("Continue")',
  cancelButton: 'button:has-text("Cancel")',

  // Preview/Review table
  reviewTable: '[data-testid="import-review-table"]',
  hostRow: '[data-testid="import-host-row"]',
  hostCheckbox: 'input[type="checkbox"][name="selected"]',
  conflictBadge: '[data-testid="conflict-badge"]',
  errorBadge: '[data-testid="error-badge"]',

  // Actions
  commitButton: 'button:has-text("Commit")',
  selectAllCheckbox: 'input[type="checkbox"][name="select-all"]',

  // Success modal
  successModal: '[data-testid="import-success-modal"]',
  viewHostsButton: 'button:has-text("View Hosts")',

  // Session expiry warning
  expiryWarning: '[data-testid="session-expiry-warning"]',
};
```

### Test Scenarios (18 tests)

#### Upload Interface (6 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should display file upload dropzone` | Dropzone visible | P0 |
| 2 | `should accept valid Caddyfile via file upload` | File input works | P0 |
| 3 | `should accept valid Caddyfile via paste` | Textarea paste | P0 |
| 4 | `should reject invalid file types` | Error for .exe, etc. | P0 |
| 5 | `should show upload progress indicator` | Progress during upload | P1 |
| 6 | `should detect import directives in Caddyfile` | Parse import statements | P1 |

```typescript
test('should accept valid Caddyfile via file upload', async ({ page }) => {
  const caddyfileContent = `
example.com {
  reverse_proxy localhost:3000
}
  `;

  await page.route('**/api/v1/import/upload', route => {
    route.fulfill({
      status: 200,
      json: {
        session: { id: 'test-session', state: 'reviewing', created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
        preview: {
          hosts: [{ domain_names: 'example.com', forward_host: 'localhost', forward_port: 3000 }],
          conflicts: [],
          errors: [],
        },
      },
    });
  });

  await page.goto('/tasks/import/caddyfile');

  // Upload file
  const fileInput = page.locator(SELECTORS.fileInput);
  await fileInput.setInputFiles({
    name: 'Caddyfile',
    mimeType: 'text/plain',
    buffer: Buffer.from(caddyfileContent),
  });

  await waitForAPIResponse(page, '/api/v1/import/upload', 200);

  // Should show review table
  await expect(page.locator(SELECTORS.reviewTable)).toBeVisible();
});

test('should accept valid Caddyfile via paste', async ({ page }) => {
  await mockImportAPI(page);
  await page.goto('/tasks/import/caddyfile');

  await page.fill(SELECTORS.pasteTextarea, 'example.com {\n  reverse_proxy localhost:8080\n}');
  await page.click(SELECTORS.uploadButton);

  await waitForAPIResponse(page, '/api/v1/import/upload', 200);
  await expect(page.locator(SELECTORS.reviewTable)).toBeVisible();
});
```

#### Preview & Review (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 7 | `should show parsed hosts from Caddyfile` | Hosts in review table | P0 |
| 8 | `should display host configuration details` | Domain, upstream, etc. | P0 |
| 9 | `should allow selection/deselection of hosts` | Checkboxes work | P0 |
| 10 | `should show validation warnings for invalid configs` | Warning badges | P1 |
| 11 | `should highlight conflicts with existing hosts` | Conflict indicator | P1 |

```typescript
test('should show parsed hosts from Caddyfile', async ({ page }) => {
  const mockPreview: ImportPreview = {
    session: { id: 'test', state: 'reviewing', created_at: '', updated_at: '' },
    preview: {
      hosts: [
        { domain_names: 'api.example.com', forward_host: 'api-server', forward_port: 8080 },
        { domain_names: 'web.example.com', forward_host: 'web-server', forward_port: 3000 },
      ],
      conflicts: [],
      errors: [],
    },
  };

  await mockImportPreview(page, mockPreview);
  await uploadCaddyfile(page, 'test content');

  // Verify both hosts shown
  await expect(page.getByText('api.example.com')).toBeVisible();
  await expect(page.getByText('web.example.com')).toBeVisible();

  // Verify upstream details
  await expect(page.getByText('api-server:8080')).toBeVisible();
  await expect(page.getByText('web-server:3000')).toBeVisible();
});

test('should highlight conflicts with existing hosts', async ({ page }) => {
  const mockPreview: ImportPreview = {
    session: { id: 'test', state: 'reviewing', created_at: '', updated_at: '' },
    preview: {
      hosts: [{ domain_names: 'existing.com' }],
      conflicts: ['existing.com'],
      errors: [],
    },
    conflict_details: {
      'existing.com': {
        existing: { forward_scheme: 'http', forward_host: 'old-server', forward_port: 80, ssl_forced: false, websocket: false, enabled: true },
        imported: { forward_scheme: 'https', forward_host: 'new-server', forward_port: 443, ssl_forced: true, websocket: true },
      },
    },
  };

  await mockImportPreview(page, mockPreview);
  await uploadCaddyfile(page, 'test');

  // Verify conflict badge visible
  await expect(page.locator(SELECTORS.conflictBadge)).toBeVisible();
  await expect(page.getByText(/conflict|already exists/i)).toBeVisible();
});
```

#### Commit Import (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 12 | `should commit selected hosts` | POST with resolutions | P0 |
| 13 | `should skip deselected hosts` | Uncheck excludes host | P1 |
| 14 | `should show success toast after import` | Toast message | P0 |
| 15 | `should navigate to proxy hosts after import` | Redirect to list | P1 |
| 16 | `should handle partial import failures` | Some succeed, some fail | P1 |

```typescript
test('should commit selected hosts', async ({ page }) => {
  let commitPayload: any = null;
  await page.route('**/api/v1/import/commit', async route => {
    commitPayload = await route.request().postDataJSON();
    route.fulfill({
      status: 200,
      json: { created: 2, updated: 0, skipped: 0, errors: [] },
    });
  });

  await setupImportReview(page, 2);

  await page.click(SELECTORS.commitButton);
  await waitForAPIResponse(page, '/api/v1/import/commit', 200);

  expect(commitPayload).toBeTruthy();
  expect(commitPayload.session_uuid).toBeTruthy();
});

test('should skip deselected hosts', async ({ page }) => {
  let commitPayload: any = null;
  await page.route('**/api/v1/import/commit', async route => {
    commitPayload = await route.request().postDataJSON();
    route.fulfill({ status: 200, json: { created: 1, updated: 0, skipped: 1, errors: [] } });
  });

  await setupImportReview(page, 2);

  // Deselect first host
  await page.locator(SELECTORS.hostCheckbox).first().uncheck();

  await page.click(SELECTORS.commitButton);
  await waitForAPIResponse(page, '/api/v1/import/commit', 200);

  // Verify skipped host in payload
  expect(commitPayload.resolutions).toBeTruthy();
});
```

#### Session Management (2 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 17 | `should handle import session timeout` | Graceful expiry handling | P2 |
| 18 | `should show warning when session expiring` | Expiry warning banner | P2 |

```typescript
test('should handle import session timeout', async ({ page }) => {
  // Mock session expired error
  await page.route('**/api/v1/import/preview', route => {
    route.fulfill({ status: 410, json: { error: 'Import session expired' } });
  });

  await page.goto('/tasks/import/caddyfile');

  // Try to continue expired session
  await page.locator(SELECTORS.continueButton).click();

  await waitForToast(page, /session expired|try again/i);

  // Should return to upload state
  await expect(page.locator(SELECTORS.fileDropzone)).toBeVisible();
});
```

---

## File 5: `tests/tasks/import-crowdsec.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/tasks/import/crowdsec` | `ImportCrowdSec.tsx` | `frontend/src/pages/ImportCrowdSec.tsx` |

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `POST` | `/api/v1/backups` | Create backup before import | `BackupFile` |
| `POST` | `/api/v1/crowdsec/import` | Import CrowdSec config | `{ message: string }` |

### UI Selectors

```typescript
const SELECTORS = {
  // File input
  fileInput: 'input[data-testid="crowdsec-import-file"]',
  uploadButton: 'button:has-text("Import")',

  // File type indicator
  acceptedFormats: '[data-testid="accepted-formats"]',

  // Progress/status
  importProgress: '[data-testid="import-progress"]',
  backupIndicator: '[data-testid="backup-created"]',

  // Validation
  invalidFileError: '[data-testid="invalid-file-error"]',
};
```

### Test Scenarios (8 tests)

#### Upload Interface (4 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should display file upload interface` | Page loads correctly | P0 |
| 2 | `should accept .tar.gz configuration files` | Valid file accepted | P0 |
| 3 | `should accept .zip configuration files` | Valid file accepted | P0 |
| 4 | `should reject invalid file types` | Error for .txt, .exe | P0 |

```typescript
test('should display file upload interface', async ({ page }) => {
  await page.goto('/tasks/import/crowdsec');

  await expect(page.locator(SELECTORS.fileInput)).toBeVisible();
  await expect(page.locator(SELECTORS.uploadButton)).toBeVisible();
  await expect(page.getByText(/\.tar\.gz|\.zip/i)).toBeVisible();
});

test('should accept .tar.gz configuration files', async ({ page }) => {
  await mockCrowdSecImportAPI(page);
  await page.goto('/tasks/import/crowdsec');

  await page.locator(SELECTORS.fileInput).setInputFiles({
    name: 'crowdsec-config.tar.gz',
    mimeType: 'application/gzip',
    buffer: Buffer.from('mock tar content'),
  });

  await page.click(SELECTORS.uploadButton);
  await waitForAPIResponse(page, '/api/v1/crowdsec/import', 200);
  await waitForToast(page, /success|imported/i);
});
```

#### Import Flow (4 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 5 | `should create backup before import` | Backup API called first | P0 |
| 6 | `should import CrowdSec configuration` | Import API called | P0 |
| 7 | `should validate configuration format` | Parse errors shown | P1 |
| 8 | `should handle import errors gracefully` | Error toast | P1 |

```typescript
test('should create backup before import', async ({ page }) => {
  let backupCalled = false;
  let importCalled = false;
  let callOrder: string[] = [];

  await page.route('**/api/v1/backups', async route => {
    if (route.request().method() === 'POST') {
      backupCalled = true;
      callOrder.push('backup');
      await route.fulfill({ status: 201, json: { filename: 'pre-import-backup.tar.gz', size: 1000, time: new Date().toISOString() } });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/crowdsec/import', async route => {
    importCalled = true;
    callOrder.push('import');
    await route.fulfill({ status: 200, json: { message: 'Import successful' } });
  });

  await page.goto('/tasks/import/crowdsec');
  await uploadCrowdSecConfig(page);

  expect(backupCalled).toBe(true);
  expect(importCalled).toBe(true);
  expect(callOrder).toEqual(['backup', 'import']); // Backup MUST come first
});
```

---

## File 6: `tests/monitoring/uptime-monitoring.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/uptime` | `Uptime.tsx` | `frontend/src/pages/Uptime.tsx` |
| - | `MonitorCard` | (inline in Uptime.tsx) |
| - | `EditMonitorModal` | (inline in Uptime.tsx) |

### API Endpoints

| Method | Endpoint | Purpose | Response |
|--------|----------|---------|----------|
| `GET` | `/api/v1/uptime/monitors` | List all monitors | `UptimeMonitor[]` |
| `POST` | `/api/v1/uptime/monitors` | Create monitor | `UptimeMonitor` |
| `PUT` | `/api/v1/uptime/monitors/:id` | Update monitor | `UptimeMonitor` |
| `DELETE` | `/api/v1/uptime/monitors/:id` | Delete monitor | `204` |
| `GET` | `/api/v1/uptime/monitors/:id/history` | Get heartbeat history | `UptimeHeartbeat[]` |
| `POST` | `/api/v1/uptime/monitors/:id/check` | Trigger immediate check | `{ message }` |
| `POST` | `/api/v1/uptime/sync` | Sync with proxy hosts | `{ synced: number }` |

### TypeScript Interfaces

```typescript
interface UptimeMonitor {
  id: string;
  upstream_host?: string;
  proxy_host_id?: number;
  remote_server_id?: number;
  name: string;
  type: string;           // 'http', 'tcp'
  url: string;
  interval: number;       // seconds
  enabled: boolean;
  status: string;         // 'up', 'down', 'unknown', 'paused'
  last_check?: string | null;
  latency: number;        // ms
  max_retries: number;
}

interface UptimeHeartbeat {
  id: number;
  monitor_id: string;
  status: string;
  latency: number;
  message: string;
  created_at: string;
}
```

### UI Selectors

```typescript
const SELECTORS = {
  // Page layout
  pageTitle: 'h1 >> text=Uptime',
  summaryCard: '[data-testid="uptime-summary"]',

  // Monitor cards
  monitorCard: '[data-testid="monitor-card"]',
  statusBadge: '[data-testid="status-badge"]',
  uptimePercentage: '[data-testid="uptime-percentage"]',
  lastCheck: '[data-testid="last-check"]',
  heartbeatBar: '[data-testid="heartbeat-bar"]',

  // Card actions
  refreshButton: 'button[aria-label="Check now"]',
  settingsDropdown: 'button[aria-label="Settings"]',
  editOption: '[role="menuitem"]:has-text("Edit")',
  deleteOption: '[role="menuitem"]:has-text("Delete")',
  toggleOption: '[role="menuitem"]:has-text("Pause")',

  // Edit modal
  editModal: '[role="dialog"]',
  nameInput: 'input[name="name"]',
  urlInput: 'input[name="url"]',
  intervalSelect: 'select[name="interval"]',
  saveButton: 'button:has-text("Save")',

  // Create button
  createButton: 'button:has-text("Add Monitor")',

  // Sync button
  syncButton: 'button:has-text("Sync")',

  // Empty state
  emptyState: '[data-testid="empty-state"]',

  // Confirmation dialog
  confirmDialog: '[role="alertdialog"]',
  confirmDelete: 'button:has-text("Delete")',
};
```

### Test Scenarios (22 tests)

#### Page Layout (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should display uptime monitoring page` | Page loads correctly | P0 |
| 2 | `should show monitor list or empty state` | Conditional display | P0 |
| 3 | `should display overall uptime summary` | Summary card | P1 |

#### Monitor List Display (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 4 | `should display all monitors with status indicators` | Status badges | P0 |
| 5 | `should show uptime percentage for each monitor` | Percentage display | P0 |
| 6 | `should show last check timestamp` | Timestamp format | P1 |
| 7 | `should differentiate up/down/unknown states` | Color-coded badges | P0 |
| 8 | `should show heartbeat history bar` | Last 60 checks visual | P1 |

```typescript
test('should display all monitors with status indicators', async ({ page }) => {
  const mockMonitors: UptimeMonitor[] = [
    { id: '1', name: 'API Server', type: 'http', url: 'https://api.example.com', interval: 60, enabled: true, status: 'up', latency: 45, max_retries: 3 },
    { id: '2', name: 'Database', type: 'tcp', url: 'tcp://db.example.com:5432', interval: 30, enabled: true, status: 'down', latency: 0, max_retries: 3 },
    { id: '3', name: 'Cache', type: 'tcp', url: 'tcp://redis.example.com:6379', interval: 60, enabled: false, status: 'paused', latency: 0, max_retries: 3 },
  ];

  await page.route('**/api/v1/uptime/monitors', route => {
    route.fulfill({ status: 200, json: mockMonitors });
  });

  await page.goto('/uptime');
  await waitForLoadingComplete(page);

  // Verify all monitors displayed
  await expect(page.getByText('API Server')).toBeVisible();
  await expect(page.getByText('Database')).toBeVisible();
  await expect(page.getByText('Cache')).toBeVisible();

  // Verify status badges
  const upBadge = page.locator('[data-testid="status-badge"][data-status="up"]');
  const downBadge = page.locator('[data-testid="status-badge"][data-status="down"]');
  const pausedBadge = page.locator('[data-testid="status-badge"][data-status="paused"]');

  await expect(upBadge).toBeVisible();
  await expect(downBadge).toBeVisible();
  await expect(pausedBadge).toBeVisible();
});

test('should show heartbeat history bar', async ({ page }) => {
  const mockHistory: UptimeHeartbeat[] = Array.from({ length: 60 }, (_, i) => ({
    id: i,
    monitor_id: '1',
    status: i % 5 === 0 ? 'down' : 'up', // Every 5th is down
    latency: Math.random() * 100,
    message: 'OK',
    created_at: new Date(Date.now() - i * 60000).toISOString(),
  }));

  await mockMonitorsWithHistory(page, mockHistory);
  await page.goto('/uptime');

  // Verify heartbeat bar rendered
  const heartbeatBar = page.locator(SELECTORS.heartbeatBar).first();
  await expect(heartbeatBar).toBeVisible();

  // Verify bar has colored segments
  await expect(heartbeatBar.locator('[data-status="up"]')).toHaveCount(48);  // 60 - 12 down
  await expect(heartbeatBar.locator('[data-status="down"]')).toHaveCount(12); // Every 5th
});
```

#### Monitor CRUD (6 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 9 | `should create new HTTP monitor` | Full create flow | P0 |
| 10 | `should create new TCP monitor` | TCP type | P1 |
| 11 | `should update existing monitor` | Edit and save | P0 |
| 12 | `should delete monitor with confirmation` | Delete flow | P0 |
| 13 | `should validate monitor URL format` | URL validation | P0 |
| 14 | `should validate check interval` | Interval range | P1 |

```typescript
test('should create new HTTP monitor', async ({ page }) => {
  let createPayload: any = null;
  await page.route('**/api/v1/uptime/monitors', async route => {
    if (route.request().method() === 'POST') {
      createPayload = await route.request().postDataJSON();
      route.fulfill({
        status: 201,
        json: { id: 'new-id', ...createPayload, status: 'unknown', latency: 0 },
      });
    } else {
      route.fulfill({ status: 200, json: [] });
    }
  });

  await page.goto('/uptime');
  await page.click(SELECTORS.createButton);

  // Fill form
  await page.fill(SELECTORS.nameInput, 'New API Monitor');
  await page.fill(SELECTORS.urlInput, 'https://api.newservice.com/health');
  await page.selectOption(SELECTORS.intervalSelect, '60');

  await page.click(SELECTORS.saveButton);
  await waitForAPIResponse(page, '/api/v1/uptime/monitors', 201);

  expect(createPayload.name).toBe('New API Monitor');
  expect(createPayload.url).toBe('https://api.newservice.com/health');
  expect(createPayload.interval).toBe(60);
});

test('should delete monitor with confirmation', async ({ page }) => {
  let deleteRequested = false;
  await page.route('**/api/v1/uptime/monitors/1', async route => {
    if (route.request().method() === 'DELETE') {
      deleteRequested = true;
      route.fulfill({ status: 204 });
    }
  });

  await setupMonitorsList(page);
  await page.goto('/uptime');

  // Open settings dropdown on first monitor
  await page.locator(SELECTORS.settingsDropdown).first().click();
  await page.click(SELECTORS.deleteOption);

  // Confirmation dialog should appear
  await expect(page.locator(SELECTORS.confirmDialog)).toBeVisible();

  // Confirm deletion
  await page.click(SELECTORS.confirmDelete);
  await waitForAPIResponse(page, '/api/v1/uptime/monitors/1', 204);

  expect(deleteRequested).toBe(true);
});
```

#### Manual Check (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 15 | `should trigger manual health check` | Check button click | P0 |
| 16 | `should update status after manual check` | Status refreshes | P0 |
| 17 | `should show check in progress indicator` | Loading state | P1 |

```typescript
test('should trigger manual health check', async ({ page }) => {
  let checkRequested = false;
  await page.route('**/api/v1/uptime/monitors/1/check', async route => {
    checkRequested = true;
    await new Promise(r => setTimeout(r, 300)); // Simulate check delay
    route.fulfill({ status: 200, json: { message: 'Check completed: UP' } });
  });

  await setupMonitorsList(page);
  await page.goto('/uptime');

  // Click refresh button on first monitor
  await page.locator(SELECTORS.refreshButton).first().click();

  // Should show loading indicator
  await expect(page.locator('[data-testid="check-loading"]')).toBeVisible();

  await waitForAPIResponse(page, '/api/v1/uptime/monitors/1/check', 200);
  expect(checkRequested).toBe(true);

  await waitForToast(page, /check.*completed|up/i);
});
```

#### Monitor History (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 18 | `should display uptime history chart` | History visualization | P1 |
| 19 | `should show incident timeline` | Down events listed | P2 |
| 20 | `should filter history by date range` | Date picker | P2 |

#### Sync with Proxy Hosts (2 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 21 | `should sync monitors from proxy hosts` | Sync button | P1 |
| 22 | `should preserve manually added monitors` | Sync doesn't delete | P1 |

```typescript
test('should sync monitors from proxy hosts', async ({ page }) => {
  let syncRequested = false;
  await page.route('**/api/v1/uptime/sync', async route => {
    syncRequested = true;
    route.fulfill({ status: 200, json: { synced: 3, message: '3 monitors synced from proxy hosts' } });
  });

  await setupMonitorsList(page);
  await page.goto('/uptime');

  await page.click(SELECTORS.syncButton);
  await waitForAPIResponse(page, '/api/v1/uptime/sync', 200);

  expect(syncRequested).toBe(true);
  await waitForToast(page, /3.*monitors.*synced/i);
});
```

---

## File 7: `tests/monitoring/real-time-logs.spec.ts`

### Route & Component Mapping

| Route | Component | Source File |
|-------|-----------|-------------|
| `/tasks/logs` (Live tab) | `LiveLogViewer.tsx` | `frontend/src/components/LiveLogViewer.tsx` |

### WebSocket Endpoints

| Endpoint | Purpose | Message Type |
|----------|---------|--------------|
| `WS /api/v1/logs/live` | Application logs stream | `LiveLogEntry` |
| `WS /api/v1/cerberus/logs/ws` | Security logs stream | `SecurityLogEntry` |

### TypeScript Interfaces

```typescript
type LogMode = 'application' | 'security';

interface LiveLogEntry {
  level: string;       // 'debug', 'info', 'warn', 'error', 'fatal'
  timestamp: string;   // ISO format
  message: string;
  source?: string;     // 'app', 'caddy', etc.
  data?: Record<string, unknown>;
}

interface SecurityLogEntry {
  timestamp: string;
  level: string;
  logger: string;
  client_ip: string;
  method: string;      // 'GET', 'POST', etc.
  uri: string;
  status: number;
  duration: number;    // seconds
  size: number;        // bytes
  user_agent: string;
  host: string;
  source: 'waf' | 'crowdsec' | 'ratelimit' | 'acl' | 'normal';
  blocked: boolean;
  block_reason?: string;
  details?: Record<string, unknown>;
}
```

### UI Selectors

```typescript
const SELECTORS = {
  // Connection status
  connectionStatus: '[data-testid="connection-status"]',
  connectedIndicator: '.bg-green-900',
  disconnectedIndicator: '.bg-red-900',
  connectionError: '[data-testid="connection-error"]',

  // Mode toggle
  modeToggle: '[data-testid="mode-toggle"]',
  applicationModeButton: 'button:has-text("App")',
  securityModeButton: 'button:has-text("Security")',

  // Controls
  pauseButton: 'button[title="Pause"]',
  playButton: 'button[title="Resume"]',
  clearButton: 'button[title="Clear logs"]',

  // Filters
  textFilter: 'input[placeholder*="Filter by text"]',
  levelSelect: 'select >> text=All Levels',
  sourceSelect: 'select >> text=All Sources',
  blockedOnlyCheckbox: 'input[type="checkbox"] >> text=Blocked only',

  // Log display
  logContainer: '.font-mono.text-xs',
  logEntry: '[data-testid="log-entry"]',
  blockedEntry: '.bg-red-900\\/30',

  // Footer
  logCount: '[data-testid="log-count"]',
  pausedIndicator: '.text-yellow-400 >> text=Paused',
};
```

### Test Scenarios (20 tests)

#### WebSocket Connection (6 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 1 | `should establish WebSocket connection` | WS connects on load | P0 |
| 2 | `should show connected status indicator` | Green badge | P0 |
| 3 | `should handle connection failure gracefully` | Error message | P0 |
| 4 | `should auto-reconnect on connection loss` | Reconnect logic | P1 |
| 5 | `should authenticate via cookies` | Cookie-based auth | P1 |
| 6 | `should recover from network interruption` | Network resume | P1 |

```typescript
test('should establish WebSocket connection', async ({ page }) => {
  let wsConnected = false;

  page.on('websocket', ws => {
    if (ws.url().includes('/api/v1/cerberus/logs/ws')) {
      ws.on('open', () => { wsConnected = true; });
    }
  });

  await page.goto('/tasks/logs');

  // Switch to live logs tab if needed
  await page.click('[data-testid="live-logs-tab"]');

  await waitForWebSocketConnection(page);
  expect(wsConnected).toBe(true);

  // Verify connected indicator
  await expect(page.locator(SELECTORS.connectionStatus)).toContainText('Connected');
});

test('should show connected status indicator', async ({ page }) => {
  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  await waitForWebSocketConnection(page);

  await expect(page.locator(SELECTORS.connectedIndicator)).toBeVisible();
  await expect(page.locator(SELECTORS.connectionStatus)).toContainText('Connected');
});

test('should handle connection failure gracefully', async ({ page }) => {
  // Block WebSocket endpoint
  await page.route('**/api/v1/cerberus/logs/ws', route => {
    route.abort('connectionrefused');
  });
  await page.route('**/api/v1/logs/live', route => {
    route.abort('connectionrefused');
  });

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // Should show disconnected/error state
  await expect(page.locator(SELECTORS.disconnectedIndicator)).toBeVisible();
  await expect(page.locator(SELECTORS.connectionError)).toBeVisible();
});
```

#### Log Streaming (5 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 7 | `should display incoming log entries in real-time` | Live updates | P0 |
| 8 | `should auto-scroll to latest logs` | Scroll behavior | P1 |
| 9 | `should respect max log limit of 500 entries` | Memory limit | P1 |
| 10 | `should format timestamps correctly` | Time display | P1 |
| 11 | `should colorize log levels appropriately` | Level colors | P2 |

```typescript
test('should display incoming log entries in real-time', async ({ page }) => {
  const testEntries: SecurityLogEntry[] = [
    {
      timestamp: new Date().toISOString(),
      level: 'info',
      logger: 'http',
      client_ip: '192.168.1.100',
      method: 'GET',
      uri: '/api/users',
      status: 200,
      duration: 0.045,
      size: 1234,
      user_agent: 'Mozilla/5.0',
      host: 'api.example.com',
      source: 'normal',
      blocked: false,
    },
  ];

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // Wait for WebSocket, then send mock message
  await page.evaluate((entries) => {
    // Simulate WebSocket message
    const event = new CustomEvent('mock-ws-message', { detail: entries[0] });
    window.dispatchEvent(event);
  }, testEntries);

  // Alternative: Use Playwright's WebSocket interception
  page.on('websocket', ws => {
    ws.on('framereceived', () => {
      // Log received
    });
  });

  // Verify entry displayed
  await expect(page.getByText('192.168.1.100')).toBeVisible();
  await expect(page.getByText('GET /api/users')).toBeVisible();
});

test('should respect max log limit of 500 entries', async ({ page }) => {
  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');
  await waitForWebSocketConnection(page);

  // Send 550 mock log entries
  await page.evaluate(() => {
    for (let i = 0; i < 550; i++) {
      window.dispatchEvent(new CustomEvent('mock-ws-message', {
        detail: { timestamp: new Date().toISOString(), message: `Log ${i}`, level: 'info' }
      }));
    }
  });

  // Wait for rendering
  await page.waitForTimeout(500);

  // Should only have ~500 entries
  const logCount = await page.locator(SELECTORS.logEntry).count();
  expect(logCount).toBeLessThanOrEqual(500);

  // Footer should show limit info
  await expect(page.locator(SELECTORS.logCount)).toContainText(/500/);
});
```

#### Mode Switching (3 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 12 | `should toggle between Application and Security modes` | Mode switch | P0 |
| 13 | `should clear logs when switching modes` | Reset on switch | P1 |
| 14 | `should reconnect to correct WebSocket endpoint` | Different WS | P0 |

```typescript
test('should toggle between Application and Security modes', async ({ page }) => {
  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // Default is security mode
  await expect(page.locator(SELECTORS.securityModeButton)).toHaveAttribute('data-active', 'true');

  // Switch to application mode
  await page.click(SELECTORS.applicationModeButton);
  await expect(page.locator(SELECTORS.applicationModeButton)).toHaveAttribute('data-active', 'true');

  // Source filter should be hidden in app mode
  await expect(page.locator(SELECTORS.sourceSelect)).not.toBeVisible();

  // Switch back to security
  await page.click(SELECTORS.securityModeButton);
  await expect(page.locator(SELECTORS.sourceSelect)).toBeVisible();
});

test('should reconnect to correct WebSocket endpoint', async ({ page }) => {
  const connectedEndpoints: string[] = [];

  page.on('websocket', ws => {
    connectedEndpoints.push(ws.url());
  });

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');
  await waitForWebSocketConnection(page);

  // Should connect to security endpoint first
  expect(connectedEndpoints.some(url => url.includes('/cerberus/logs/ws'))).toBe(true);

  // Switch to application mode
  await page.click(SELECTORS.applicationModeButton);
  await waitForWebSocketConnection(page);

  // Should connect to live logs endpoint
  expect(connectedEndpoints.some(url => url.includes('/logs/live'))).toBe(true);
});
```

#### Live Filters (4 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 15 | `should filter by text search` | Client-side filter | P0 |
| 16 | `should filter by log level` | Level dropdown | P0 |
| 17 | `should filter by source in security mode` | Source dropdown | P1 |
| 18 | `should filter blocked requests only` | Checkbox filter | P1 |

```typescript
test('should filter by text search', async ({ page }) => {
  await setupLiveLogsWithMockData(page, [
    { message: 'User login successful', client_ip: '10.0.0.1' },
    { message: 'API request to /users', client_ip: '10.0.0.2' },
    { message: 'Database connection', client_ip: '10.0.0.3' },
  ]);

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // All 3 entries visible initially
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(3);

  // Filter by "login"
  await page.fill(SELECTORS.textFilter, 'login');

  // Only 1 entry should be visible
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(1);
  await expect(page.getByText('User login successful')).toBeVisible();
});

test('should filter blocked requests only', async ({ page }) => {
  await setupLiveLogsWithMockData(page, [
    { blocked: false, message: 'Normal request' },
    { blocked: true, block_reason: 'WAF rule', message: 'Blocked by WAF' },
    { blocked: false, message: 'Another normal' },
  ]);

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // All 3 entries visible
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(3);

  // Check "Blocked only"
  await page.check(SELECTORS.blockedOnlyCheckbox);

  // Only 1 blocked entry visible
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(1);
  await expect(page.locator(SELECTORS.blockedEntry)).toBeVisible();
});
```

#### Playback Controls (2 tests)

| # | Test Name | Description | Priority |
|---|-----------|-------------|----------|
| 19 | `should pause and resume log streaming` | Pause/play toggle | P0 |
| 20 | `should clear all logs` | Clear button | P1 |

```typescript
test('should pause and resume log streaming', async ({ page }) => {
  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');
  await waitForWebSocketConnection(page);

  // Click pause
  await page.click(SELECTORS.pauseButton);

  // Should show paused indicator
  await expect(page.locator(SELECTORS.pausedIndicator)).toBeVisible();

  // Pause button should become play button
  await expect(page.locator(SELECTORS.playButton)).toBeVisible();

  // Send new log (should be ignored while paused)
  const countBefore = await page.locator(SELECTORS.logEntry).count();
  await sendMockLogEntry(page);
  const countAfter = await page.locator(SELECTORS.logEntry).count();
  expect(countAfter).toBe(countBefore);

  // Resume
  await page.click(SELECTORS.playButton);
  await expect(page.locator(SELECTORS.pausedIndicator)).not.toBeVisible();

  // New logs should now appear
  await sendMockLogEntry(page);
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(countBefore + 1);
});

test('should clear all logs', async ({ page }) => {
  await setupLiveLogsWithMockData(page, [
    { message: 'Log 1' },
    { message: 'Log 2' },
    { message: 'Log 3' },
  ]);

  await page.goto('/tasks/logs');
  await page.click('[data-testid="live-logs-tab"]');

  // Logs visible
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(3);

  // Clear logs
  await page.click(SELECTORS.clearButton);

  // All logs cleared
  await expect(page.locator(SELECTORS.logEntry)).toHaveCount(0);
  await expect(page.getByText('No logs yet')).toBeVisible();
});
```

---

## Helper Functions

Create these helper functions in `tests/utils/phase5-helpers.ts`:

```typescript
import { Page } from '@playwright/test';
import { waitForAPIResponse, waitForWebSocketConnection } from './wait-helpers';

/**
 * Sets up mock backup list for testing
 */
export async function setupBackupsList(page: Page, backups?: BackupFile[]) {
  const defaultBackups: BackupFile[] = backups || [
    { filename: 'backup_2024-01-15_120000.tar.gz', size: 1048576, time: '2024-01-15T12:00:00Z' },
    { filename: 'backup_2024-01-14_120000.tar.gz', size: 2097152, time: '2024-01-14T12:00:00Z' },
  ];

  await page.route('**/api/v1/backups', route => {
    if (route.request().method() === 'GET') {
      route.fulfill({ status: 200, json: defaultBackups });
    } else {
      route.continue();
    }
  });
}

/**
 * Sets up mock log files for testing
 */
export async function setupLogFiles(page: Page, files?: LogFile[]) {
  const defaultFiles: LogFile[] = files || [
    { name: 'access.log', size: 1048576, modified: '2024-01-15T12:00:00Z' },
    { name: 'error.log', size: 256000, modified: '2024-01-15T11:30:00Z' },
  ];

  await page.route('**/api/v1/logs', route => {
    route.fulfill({ status: 200, json: defaultFiles });
  });
}

/**
 * Selects a log file and waits for content to load
 */
export async function selectLogFile(page: Page, filename: string) {
  await page.click(`button:has-text("${filename}")`);
  await waitForAPIResponse(page, `/api/v1/logs/${filename}`, 200);
}

/**
 * Sets up mock monitors list for testing
 */
export async function setupMonitorsList(page: Page, monitors?: UptimeMonitor[]) {
  const defaultMonitors: UptimeMonitor[] = monitors || [
    { id: '1', name: 'API Server', type: 'http', url: 'https://api.example.com', interval: 60, enabled: true, status: 'up', latency: 45, max_retries: 3 },
    { id: '2', name: 'Database', type: 'tcp', url: 'tcp://db:5432', interval: 30, enabled: true, status: 'down', latency: 0, max_retries: 3 },
  ];

  await page.route('**/api/v1/uptime/monitors', route => {
    if (route.request().method() === 'GET') {
      route.fulfill({ status: 200, json: defaultMonitors });
    } else {
      route.continue();
    }
  });
}

/**
 * Mock import API for Caddyfile testing
 */
export async function mockImportPreview(page: Page, preview: ImportPreview) {
  await page.route('**/api/v1/import/upload', route => {
    route.fulfill({ status: 200, json: preview });
  });
  await page.route('**/api/v1/import/preview', route => {
    route.fulfill({ status: 200, json: preview });
  });
}

/**
 * Generates mock log entries for pagination testing
 */
export function generateMockEntries(count: number, page: number): CaddyAccessLog[] {
  return Array.from({ length: count }, (_, i) => ({
    level: 'info',
    ts: Date.now() / 1000 - (page * count + i) * 60,
    logger: 'http.log.access',
    msg: 'handled request',
    request: {
      remote_ip: `192.168.1.${i % 255}`,
      method: 'GET',
      host: 'example.com',
      uri: `/page/${page * count + i}`,
      proto: 'HTTP/2',
    },
    status: 200,
    duration: 0.05,
    size: 1234,
  }));
}

/**
 * Simulates WebSocket network interruption for reconnection testing
 */
export async function simulateNetworkInterruption(page: Page, durationMs: number = 1000) {
  // Block WebSocket endpoints
  await page.route('**/api/v1/logs/live', route => route.abort());
  await page.route('**/api/v1/cerberus/logs/ws', route => route.abort());

  await page.waitForTimeout(durationMs);

  // Restore WebSocket endpoints
  await page.unroute('**/api/v1/logs/live');
  await page.unroute('**/api/v1/cerberus/logs/ws');
}
```

---

## Acceptance Criteria

### Backups (25 tests total)
- [ ] All CRUD operations covered (create, list, delete, download)
- [ ] Restore workflow with explicit confirmation
- [ ] Error handling for failures
- [ ] Role-based access (admin vs guest)

### Logs (38 tests total)
- [ ] Static log file viewing with filtering
- [ ] WebSocket real-time streaming
- [ ] Mode switching (Application/Security)
- [ ] Pause/Resume/Clear controls
- [ ] Client-side filtering

### Imports (26 tests total)
- [ ] Caddyfile upload and paste
- [ ] Preview and review workflow
- [ ] Conflict detection and resolution
- [ ] CrowdSec import with backup

### Uptime (22 tests total)
- [ ] Monitor CRUD operations
- [ ] Status indicator display
- [ ] Manual health check
- [ ] Heartbeat history visualization
- [ ] Sync with proxy hosts

### Overall Phase 5
- [ ] 111+ tests total (target: 92-114)
- [ ] <5% flaky test rate
- [ ] All P0 tests complete
- [ ] 90%+ P1 tests complete
- [ ] No hardcoded waits
- [ ] All tests use TestDataManager for cleanup
- [ ] WebSocket tests properly mock connections

---

## Test Execution Commands

```bash
# Run all Phase 5 tests
npx playwright test tests/tasks tests/monitoring --project=chromium

# Run specific test file
npx playwright test tests/tasks/backups-create.spec.ts --project=chromium

# Run with debug mode
npx playwright test tests/monitoring/real-time-logs.spec.ts --debug

# Run with coverage
npm run test:e2e:coverage -- tests/tasks tests/monitoring

# Generate report
npx playwright show-report
```

---

## Notes

1. **WebSocket Testing**: Use Playwright's `page.on('websocket', ...)` for real WebSocket testing. For complex scenarios, consider mocking at the API level.

2. **Session Timeouts**: Import session tests require understanding server-side TTL. Mock 410 responses for expiry scenarios.

3. **Backup Download**: Browser download events must be captured with `page.waitForEvent('download')`.

4. **Real-time Updates**: Use `retryAction` from wait-helpers for assertions that depend on WebSocket messages.

5. **Test Data Cleanup**: All tests creating backups or monitors should use TestDataManager for cleanup in `afterEach`.
