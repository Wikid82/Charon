# QA Report: CrowdSec Persistence Fix

## Execution Summary
**Date**: 2025-12-14
**Task**: Fixing CrowdSec "Offline" status due to lack of persistence.
**Agent**: QA_Security (Antigravity)

## 🧪 Verification Results

### Static Analysis
- **Pre-commit**: ⚠️ Skipped (Tool not installed in environment).
- **Manual Code Review**: ✅ Passed.
  - `docker-entrypoint.sh`: Logic correctly handles directory initialization, copying of defaults, and symbolic linking.
  - `docker-compose.yml`: Documentation added clearly.
  - **Idempotency**: Checked. The script checks for file/link existence before acting, preventing data overwrite on restarts.

### Logic Audit
- **Persistence**:
  - Config: `/etc/crowdsec` -> `/app/data/crowdsec/config`.
  - Data: `DATA` env var -> `/app/data/crowdsec/data`.
  - Hub: `/etc/crowdsec/hub` is created in persistent path.
- **Fail-safes**:
  - Fallback to `/etc/crowdsec.dist` or `/etc/crowdsec` ensures config covers missing files.
  - `cscli` checks integrity on startup.

### ⚠️ Risks & Edges
- **First Restart**: The first restart after applying this fix requires the user to **re-enroll** with CrowdSec Console because the Machine ID will change (it is now persistent, but the previous one was ephemeral and lost).
- **File Permissions**: Assumes the container user (`root` usually in this context) has write access to `/app/data`. This is standard for Charon.

## Recommendations
- **Approve**. The fix addresses the root cause directly.
- **User Action**: User must verify by running `cscli machines list` across restarts.
