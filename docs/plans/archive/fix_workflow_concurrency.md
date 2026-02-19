# Fix Workflow Concurrency Logic

## 1. Introduction
The current GitHub Actions workflows use `concurrency` settings that often group runs solely by branch name. This causes an issue where a `push` to a branch cancels an active `pull_request` check for the same branch (or vice versa), because they resolve to the same concurrency group key.

This plan aims to decouple these contexts so that:
- **Push runs** only cancel previous **Push runs** on the same branch.
- **PR runs** only cancel previous **PR runs** on the same PR/branch.
- They **do not** cancel each other.

## 2. Technical Specification

### 2.1 Standard Workflows
For workflows triggered by `push` or `pull_request` (e.g., `docker-build.yml`), we will inject `${{ github.event_name }}` into the concurrency group key.

**Current Pattern:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.head_ref || github.ref_name }}
  cancel-in-progress: true
```

**New Pattern:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event_name }}-${{ github.head_ref || github.ref_name }}
  cancel-in-progress: true
```

### 2.2 Chained Workflows (`workflow_run`)
For workflows triggered by the completion of another workflow (e.g., `security-pr.yml` triggered by `docker-build`), we must differentiate based on what triggered the *upstream* run.

**Current Pattern:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event.workflow_run.head_branch || github.ref }}
  cancel-in-progress: true
```

**New Pattern:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event.workflow_run.event || github.event_name }}-${{ github.event.workflow_run.head_branch || github.ref }}
  cancel-in-progress: true
```
*Note: We use `|| github.event_name` and `|| github.ref` to handle cases where the workflow might be manually triggered (`workflow_dispatch`), where `workflow_run` context is missing.*

## 3. Implementation Plan

### Phase 1: Update Standard Workflows
Target Files:
- `.github/workflows/docker-build.yml`
- `.github/workflows/quality-checks.yml`
- `.github/workflows/codeql.yml`
- `.github/workflows/benchmark.yml`
- `.github/workflows/docs.yml`

### Phase 2: Update Chained Workflows
Target Files:
- `.github/workflows/security-pr.yml`
- `.github/workflows/cerberus-integration.yml`
- `.github/workflows/crowdsec-integration.yml`
- `.github/workflows/rate-limit-integration.yml`
- `.github/workflows/waf-integration.yml`
- `.github/workflows/supply-chain-pr.yml`

## 4. Acceptance Criteria
- [x] Push events triggers do not cancel visible PR checks.
- [x] PR synchronizations cancel older PR checks.
- [x] Repeated Pushes cancel older Push checks.
- [x] Manual triggers (`workflow_dispatch`) are handled gracefully without syntax errors.

## 5. Resolution Log
**Executed by Agent on 2025-02-23:**

Applied concurrency group updates to differentiate between `push` and `pull_request` events.

**Updated Standard Workflows:**
- `docker-build.yml`
- `quality-checks.yml`
- `codeql.yml`
- `benchmark.yml`
- `docs.yml`
- `docker-lint.yml` (Added)
- `codecov-upload.yml` (Added)
- `repo-health.yml` (Added)
- `auto-changelog.yml` (Added)
- `history-rewrite-tests.yml` (Added)
- `dry-run-history-rewrite.yml` (Added)

**Updated Chained Workflows (`workflow_run`):**
- `security-pr.yml`
- `cerberus-integration.yml`
- `crowdsec-integration.yml`
- `rate-limit-integration.yml`
- `waf-integration.yml`
- `supply-chain-pr.yml`

All identified workflows now include `${{ github.event_name }}` (or `${{ github.event.workflow_run.event }}`) in their concurrency group keys to prevent aggressive cancellation.
