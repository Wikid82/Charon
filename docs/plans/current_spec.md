# Fix CrowdSec Console Enroll Error

## Problem
The user reported an error when enrolling to CrowdSec Console: `console *** failed: Error: cscli console enroll: unknown flag: --tenant : exit status 1`.
This indicates that the `cscli` command does not support the `--tenant` flag, which is currently being passed by the backend.

## Solution
Remove the `--tenant` flag from the `cscli console enroll` command arguments in `backend/internal/crowdsec/console_enroll.go`.
The tenant information will still be stored in the database and logged, but it will not be passed to the CLI command.

## Implementation Steps

1.  **Modify `backend/internal/crowdsec/console_enroll.go`**:
    *   In the `Enroll` function, remove the conditional block that appends `--tenant` to the `args` slice.

## Verification
*   Run backend tests to ensure no regressions (though existing tests mock the execution and don't check args, so they should pass).
*   The fix relies on the removal of the flag that caused the error.
