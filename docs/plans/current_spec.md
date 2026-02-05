# Remediation Plan: Stability & E2E Regressions

**Objective**: Restore system stability by fixing pre-commit failures, resolving E2E regressions in the frontend, and correcting CI workflow configurations.

## 1. Findings (Current State)

| Issue | Location | Description | Severity |
|-------|----------|-------------|----------|
| **Syntax Error** | `frontend/src/pages/CrowdSecConfig.tsx` | Missing fragment closing tag (`</>`) at the end of the `showBanModal` conditional block. | **Critical** (Build Failure) |
| **UX/E2E Regression** | `frontend/src/components/ProxyHostForm.tsx` | Manual `fixed z-50` overlay causes stacking context issues, preventing interaction with nested modals (e.g., "Add Proxy Host"). | **High** (E2E Failure) |
| **CI Misconfiguration** | `.github/workflows/crowdsec-integration.yml` | Duplicate logic block for tag determination and mismatched step identifiers (`id: image` vs `steps.determine-tag`). | **Medium** (CI Failure) |
| **Version Mismatch** | `.version` | File contains `v0.17.0`, but git tag is `v0.17.1`. | **Low** (Inconsistency) |

## 2. Technical Specifications

### 2.1. Frontend: Proxy Host Form Refactor
**Goal**: Replace manual overlay implementation with standardized Shadcn UI components to resolve stacking context issues.

- **Component**: `frontend/src/components/ProxyHostForm.tsx`
- **Change**:
  - Remove manual overlay logic:
    ```tsx
    <div className="fixed inset-0 bg-black/50 z-40" onClick={onCancel} />
    <div className="fixed inset-0 flex items-center justify-center ... z-50">...</div>
    ```
  - Implement `Dialog` component (Shadcn UI):
    ```tsx
    <Dialog open={true} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto bg-dark-card border-gray-800 p-0 gap-0">
        <DialogHeader className="p-6 border-b border-gray-800">
          <DialogTitle className="text-2xl font-bold text-white">
            {host ? 'Edit Proxy Host' : 'Add Proxy Host'}
          </DialogTitle>
        </DialogHeader>
        {/* Form Content */}
      </DialogContent>
    </Dialog>
    ```
  - Ensure all form logic remains intact within the Dialog content.

### 2.2. Frontend: CrowdSec Config Fix
**Goal**: Fix JSX syntax error.

- **Component**: `frontend/src/pages/CrowdSecConfig.tsx`
- **Change**: Add missing `</>` tag to close the Fragment wrapping the Ban IP Modal.
    ```tsx
    {showBanModal && (
        <>
            {/* ... Modal Content ... */}
        </> // <-- Add this
    )}
    ```

### 2.3. CI Workflow Cleanup
**Goal**: Remove redundancy and fix references.

- **File**: `.github/workflows/crowdsec-integration.yml`
- **Changes**:
  - Rename step `id: image` to `id: determine-tag`.
  - Update all references from `steps.image.outputs...` to `steps.determine-tag.outputs...`.
  - Review file for duplicate "Determine image tag" logic blocks and remove the redundant one.

### 2.4. Versioning
**Goal**: Sync version file.

- **File**: `.version`
- **Change**: Update content to `v0.17.1`.

## 3. Implementation Plan

### Phase 1: Quick Fixes (Ops)
- [ ] **Task 1.1**: Update `.version` to `v0.17.1`.
- [ ] **Task 1.2**: Fix `.github/workflows/crowdsec-integration.yml` (Rename ID, remove duplicates).

### Phase 2: Frontend Syntax Repair
- [ ] **Task 2.1**: Add missing `</>` to `frontend/src/pages/CrowdSecConfig.tsx`.
- [ ] **Task 2.2**: Verify frontend build (`npm run build` in frontend) to ensure no other syntax errors.

### Phase 3: Frontend Component Refactor
- [ ] **Task 3.1**: Verify `Dialog` components are available in codebase (`components/ui/dialog`).
- [ ] **Task 3.2**: Refactor `ProxyHostForm.tsx` to use `Dialog`.
- [ ] **Task 3.3**: Verify "Add Proxy Host" modal interactions manually or via E2E test.

### Phase 4: Verification
- [ ] **Task 4.1**: Run Playwright E2E tests for Dashboard/Proxy Hosts.
- [ ] **Task 4.2**: Run Lint/Pre-commit checks.

## 4. Acceptance Criteria
- [ ] `npm run lint` passes in `frontend/`.
- [ ] `.github/workflows/crowdsec-integration.yml` parses correctly (no YAML errors).
- [ ] E2E tests for Proxy Host management pass.
- [ ] `.version` matches git tag.
