# Plan: Fix 'No packages found' for Integration Tests

## Problem
VS Code reports "No packages found" for files with `//go:build integration` tags (e.g., `backend/integration/coraza_integration_test.go`). This is because `gopls` (the Go language server) does not include the `integration` build tag by default, causing it to ignore these files during analysis.

## Solution
Configure `gopls` in the workspace settings to include the `integration` build tag.

## Steps

### 1. Check `.vscode/settings.json`
- [x] Verify `.vscode/settings.json` exists. (Confirmed)

### 2. Update `gopls` settings
- [ ] Edit `.vscode/settings.json`.
- [ ] Locate the `"gopls"` configuration object.
- [ ] Add or update the `"buildFlags"` property to include `"-tags=integration"`.

**Target Configuration:**
```json
"gopls": {
    "buildFlags": [
        "-tags=integration"
    ],
    // ... existing settings ...
}
```

### 3. Verification
- [ ] Reload VS Code window (or restart the Go language server).
- [ ] Open `backend/integration/coraza_integration_test.go`.
- [ ] Verify that the "No packages found" error is resolved and IntelliSense works for the file.
