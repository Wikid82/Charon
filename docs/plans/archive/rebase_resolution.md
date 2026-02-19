# Rebase Resolution Plan

## Overview
We are resolving conflicts in 4 workflow files during an interactive rebase. The conflicts primarily involve:
1.  Updates to `workflow_dispatch` inputs (adding `latest` to description) from the rebase target.
2.  Regression/simplification of `concurrency` groups in `e2e-tests.yml` (we must keep our robust HEAD version).
3.  A massive duplication of logic ("Determine tag" -> "Pull image") in integration workflows caused by git auto-merge.
4.  A conflict between "Pull from Registry" (HEAD) vs "Download Artifact" (Incoming) in `e2e-tests.yml` (we must keep Registry pull).

## File-by-File Instructions

### 1. `.github/workflows/crowdsec-integration.yml`

*   **Conflict Area 1 (Inputs)**:
    *   **Resolution**: Accept the *Incoming* change for the description (includes `latest`).
    *   **Action**: Update description to `'Docker image tag to test (e.g., pr-123-abc1234, latest)'`.
*   **Duplication Fix (CRITICAL)**:
    *   **Issue**: The steps "Determine image tag", "Pull Docker image from registry", and "Fallback to artifact download" appear TWICE sequentially.
    *   **Resolution**: Delete the **FIRST** occurrence of this block. Keep the sequence that leads directly into "Validate image SHA".
    *   **Block to Delete**: Approximately lines 26-124.

### 2. `.github/workflows/e2e-tests.yml`

*   **Inputs Issue (No marker, but duplicated)**:
    *   **Issue**: `image_tag` input appears twice in `workflow_dispatch`.
    *   **Resolution**: Keep the second one (with `latest` in description) and delete the first one.
*   **Conflict Area 2 (Concurrency)**:
    *   **Resolution**: Keep **HEAD**. It contains the robust concurrency group key (`e2e-${{ github.workflow }}-${{ ... }}`) whereas the incoming change reverts to a simpler, less safe one.
*   **Conflict Area 3 (Pull vs Download)**:
    *   **Issue**: HEAD uses "Pull Docker image from registry" (Phase 4 strategy). Incoming uses "Download Docker image" (old strategy).
    *   **Resolution**: Keep **HEAD**.

### 3. `.github/workflows/rate-limit-integration.yml`

*   **Conflict Area 1 (Inputs)**:
    *   **Resolution**: Accept *Incoming* (with `latest`).
*   **Duplication Fix**:
    *   **Issue**: Same as CrowdSec. Duplicate logic block.
    *   **Resolution**: Delete the **FIRST** occurrence of the [Determine -> Pull -> Fallback] sequence.

### 4. `.github/workflows/waf-integration.yml`

*   **Conflict Area 1 (Inputs)**:
    *   **Resolution**: Accept *Incoming* (with `latest`).
*   **Duplication Fix**:
    *   **Issue**: Same as CrowdSec. Duplicate logic block.
    *   **Resolution**: Delete the **FIRST** occurrence of the [Determine -> Pull -> Fallback] sequence.

## Verification
After applying these fixes, we will verify:
1.  No conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) remain.
2.  No duplicate steps in the flows.
3.  `e2e-tests.yml` specifically retains "Pull Docker image from registry".
