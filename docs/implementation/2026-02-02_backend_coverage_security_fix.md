# Backend Coverage, Security & E2E Fixes

**Date**: 2026-02-02
**Context**: Remediation of critical security vulnerabilities, backend test coverage improvements, and cross-browser E2E stability.

## 1. Architectural Constraint: Concrete Types vs Interfaces

### Problem
Initial attempts to increase test coverage for `ConfigLoader` and `ConfigManager` relied on mocking interfaces (`IConfigLoader`, `IConfigManager`). This approach proved problematic:
1.  **Brittleness**: Mocks required constant updates whenever internal implementation details changed.
2.  **False Confidence**: Mocks masked actual integration issues, particularly with file system interactions.
3.  **Complexity**: The setup for mocks became more complex than the code being tested.

### Solution: Real Dependency Pattern
We shifted strategy to test **concrete types** instead of mocks for these specific components.
- **Why**: `ConfigLoader` and `ConfigManager` are "leaf" nodes in the dependency graph responsible for IO. Testing them with real (temporary) files system operations provides higher value.
- **Implementation**:
  - Tests now create temporary directories using `t.TempDir()`.
  - Concrete `NewConfigLoader` and `NewConfigManager` are instantiated.
  - Assertions verify actual file creation and content on disk.

## 2. Security Fix: SafeJoin Remediation

### Vulnerability
Three critical vulnerabilities were identified where `filepath.Join` was used with user-controlled input, creating a risk of Path Traversal attacks.

**Locations:**
1.  `backend/internal/caddy/config_loader.go`
2.  `backend/internal/caddy/config_manager.go`
3.  `backend/internal/caddy/import_handler.go`

### Fix
Replaced all risky `filepath.Join` calls with `utils.SafeJoin`.

**Mechanism**:
`utils.SafeJoin(base, path)` performs the following checks:
1.  Joins the paths.
2.  Cleans the resulting path.
3.  Verifies that the resulting path still has the `base` path as a prefix.
4.  Returns an error if the path attempts to traverse outside the base.

## 3. E2E Fix: WebKit/Firefox Switch Interaction

### Issue
E2E tests involving the `Switch` component (shadcn/ui) were reliably passing in Chromium but failing in WebKit (Safari) and Firefox.
- **Symptoms**: Timeouts, `click intercepted` errors, or assertions failing because the switch state didn't change.
- **Root Cause**: The underlying `<input type="checkbox">` is often visually hidden or covered by the styled toggle element. Chromium's event dispatching is slightly more forgiving, while WebKit/Firefox adhere strictly to visibility and hit-testing rules.

### Fix
Refactored `tests/utils/ui-helpers.ts` to improve interaction reliability.

1.  **Semantic Clicks**: Instead of trying to force-click the input or specific coordinates, we now locate the accessible label or the wrapper element that handles the click event.
2.  **Explicit State Verification**: Replaced arbitrary `waitForTimeout` calls with smart polling assertions:
    ```typescript
    // Before
    await toggle.click();
    await page.waitForTimeout(500);

    // After
    await toggle.click();
    await expect(toggle).toBeChecked({ timeout: 5000 });
    ```
3.  **Result**: 100% pass rate across all three browser engines for System Settings and User Management tests.
