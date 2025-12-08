# Plan: Refactor Feature Flags to Optional Features

## Overview
Refactor the existing "Feature Flags" system into a user-friendly "Optional Features" section in System Settings. This involves renaming, consolidating toggles (Cerberus, Uptime), and enforcing behavior (hiding sidebar items, stopping background jobs) when features are disabled.

## User Requirements
1.  **Rename**: 'Feature Flags' -> 'Optional Features'.
2.  **Cerberus**: Move global toggle to 'Optional Features'.
3.  **Uptime**: Add toggle to 'Optional Features'.
4.  **Cleanup**: Remove unused flags (`feature.global.enabled`, `feature.notifications.enabled`, `feature.docker.enabled`).
5.  **Behavior**:
    -   **Default**: Cerberus and Uptime ON.
    -   **OFF State**: Hide from Sidebar, stop background jobs, block notifications.
    -   **Persistence**: Do NOT delete data when disabled.

## Implementation Details

### 1. Backend Changes

#### `backend/internal/api/handlers/feature_flags_handler.go`
-   Update `defaultFlags` list:
    -   Keep: `feature.cerberus.enabled`, `feature.uptime.enabled`
    -   Remove: `feature.global.enabled`, `feature.notifications.enabled`, `feature.docker.enabled`
-   Ensure defaults are `true` if not set in DB or Env.

#### `backend/internal/cerberus/cerberus.go`
-   Update `IsEnabled()` to check `feature.cerberus.enabled` instead of `security.cerberus.enabled`.
-   Maintain backward compatibility or migrate existing setting if necessary (or just switch to the new key).

#### `backend/internal/api/routes/routes.go`
-   **Uptime Background Job**:
    -   In the `go func()` that runs the ticker:
        -   Check `feature.uptime.enabled` before running `uptimeService.CheckAll()`.
        -   If disabled, skip the check.
-   **Cerberus Middleware**:
    -   The middleware already calls `IsEnabled()`, so updating `cerberus.go` is sufficient.

### 2. Frontend Changes

#### `frontend/src/pages/SystemSettings.tsx`
-   **Rename Card**: Change "Feature Flags" to "Optional Features".
-   **Consolidate Toggles**:
    -   Remove "Enable Cerberus Security" from "General Configuration".
    -   Render specific toggles for "Cerberus Security" and "Uptime Monitoring" in the "Optional Features" card.
    -   Use `feature.cerberus.enabled` and `feature.uptime.enabled` keys.
    -   Add user-friendly descriptions for each.
-   **Remove Generic List**: Instead of iterating over all keys, explicitly render the supported optional features to control order and presentation.

#### `frontend/src/components/Layout.tsx`
-   **Fetch Flags**: Use `getFeatureFlags` (or a new hook) to get current state.
-   **Conditional Rendering**:
    -   Hide "Uptime" nav item if `feature.uptime.enabled` is false.
    -   Hide "Security" nav group if `feature.cerberus.enabled` is false.

### 3. Migration / Data Integrity
-   Existing `security.cerberus.enabled` setting in DB should be migrated to `feature.cerberus.enabled` or the code should handle the transition.
-   **Action**: We will switch to `feature.cerberus.enabled`. The user can re-enable it if it defaults to off, but we'll try to default it to ON in the handler.

## Step-by-Step Execution

1.  **Backend**: Update `feature_flags_handler.go` to clean up flags and set defaults.
2.  **Backend**: Update `cerberus.go` to use new flag key.
3.  **Backend**: Update `routes.go` to gate Uptime background job.
4.  **Frontend**: Update `SystemSettings.tsx` UI.
5.  **Frontend**: Update `Layout.tsx` sidebar logic.
6.  **Verify**: Test toggling features and checking sidebar/background behavior.
