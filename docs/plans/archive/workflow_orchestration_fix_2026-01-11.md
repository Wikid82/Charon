# Workflow Orchestration Fix: Supply Chain Verification Dependency

**Status**: ✅ Complete
**Date Completed**: 2026-01-11
**Issue**: Workflow Orchestration Fix for Supply Chain Verification

---

## Implementation Summary

Successfully implemented workflow orchestration dependency to ensure supply chain verification runs **after** Docker image build completes. See full documentation in [docs/implementation/WORKFLOW_ORCHESTRATION_FIX.md](../../implementation/WORKFLOW_ORCHESTRATION_FIX.md).

---

## Original Specification

### Problem Statement

The `supply-chain-verify.yml` workflow runs **concurrently** with `docker-build.yml` on PR triggers, causing it to skip verification because the Docker image doesn't exist yet:

```
PR Opened
    ├─> docker-build.yml starts     (builds image)
    └─> supply-chain-verify.yml starts  (image not found → skips)
```

**Root Cause**: Both workflows trigger independently on the same events with no orchestration dependency ensuring verification runs **after** the build completes.

**Evidence**: From the GitHub Actions run, supply-chain-verify correctly detects image doesn't exist and logs: "⚠️ Image not found - likely not built yet"

### Proposed Solution

**Architecture Decision**: Keep workflows separate with dependency orchestration via `workflow_run` trigger.

**Rationale**:

- **Modularity**: Each workflow has a distinct, cohesive purpose
- **Reusability**: Verification can run on-demand or scheduled independently
- **Maintainability**: Easier to test, debug, and understand individual workflows
- **Flexibility**: Can trigger verification separately without rebuilding images

### Implementation Plan

#### Phase 1: Add `workflow_run` Trigger

Modify `supply-chain-verify.yml` triggers:

**Current**:

```yaml
on:
  release:
    types: [published]
  pull_request:
    paths: [...]
  schedule:
    - cron: '0 0 * * 1'
  workflow_dispatch:
```

**Proposed**:

```yaml
on:
  release:
    types: [published]

  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types: [completed]
    branches:
      - main
      - development
      - feature/beta-release

  schedule:
    - cron: '0 0 * * 1'

  workflow_dispatch:
```

**Key Changes**:

1. Remove `pull_request` trigger (prevents premature execution)
2. Add `workflow_run` trigger that waits for docker-build workflow
3. Specify branches to match docker-build's branch targets
4. Preserve `workflow_dispatch` for manual verification
5. Preserve `schedule` for weekly security scans

#### Phase 2: Filter by Build Success

Add job-level conditional to ensure we only verify successfully built images:

```yaml
jobs:
  verify-sbom:
    name: Verify SBOM
    runs-on: ubuntu-latest
    if: |
      (github.event_name != 'schedule' || github.ref == 'refs/heads/main') &&
      (github.event_name != 'workflow_run' || github.event.workflow_run.conclusion == 'success')
    steps:
      # ... existing steps
```

#### Phase 3: Update Tag Determination Logic

Modify the "Determine Image Tag" step to handle `workflow_run` context:

```yaml
- name: Determine Image Tag
  id: tag
  run: |
    if [[ "${{ github.event_name }}" == "release" ]]; then
      TAG="${{ github.event.release.tag_name }}"
    elif [[ "${{ github.event_name }}" == "workflow_run" ]]; then
      # Extract tag from the workflow that triggered us
      if [[ "${{ github.event.workflow_run.head_branch }}" == "main" ]]; then
        TAG="latest"
      elif [[ "${{ github.event.workflow_run.head_branch }}" == "development" ]]; then
        TAG="dev"
      elif [[ "${{ github.event.workflow_run.head_branch }}" == "feature/beta-release" ]]; then
        TAG="beta"
      elif [[ "${{ github.event.workflow_run.event }}" == "pull_request" ]]; then
        # Extract PR number from workflow_run context
        PR_NUMBER=$(jq -r '.pull_requests[0].number' <<< '${{ toJson(github.event.workflow_run) }}')
        TAG="pr-${PR_NUMBER}"
      else
        TAG="sha-$(echo ${{ github.event.workflow_run.head_sha }} | cut -c1-7)"
      fi
    else
      TAG="latest"
    fi
    echo "tag=${TAG}" >> $GITHUB_OUTPUT
```

#### Phase 4: Update PR Comment Logic

Update the "Comment on PR" step to work with `workflow_run` context:

```yaml
- name: Comment on PR
  if: |
    github.event_name == 'pull_request' ||
    (github.event_name == 'workflow_run' && github.event.workflow_run.event == 'pull_request')
  uses: actions/github-script@v7
  with:
    script: |
      // Determine PR number from context
      let prNumber;
      if (context.eventName === 'pull_request') {
        prNumber = context.issue.number;
      } else if (context.eventName === 'workflow_run') {
        const pullRequests = context.payload.workflow_run.pull_requests;
        if (pullRequests && pullRequests.length > 0) {
          prNumber = pullRequests[0].number;
        }
      }

      if (!prNumber) {
        console.log('No PR number found, skipping comment');
        return;
      }

      // ... rest of existing comment logic
```

### Workflow Execution Flow (After Fix)

**PR Workflow**:

```
PR Opened/Updated
    └─> docker-build.yml runs
            ├─> Builds image: ghcr.io/wikid82/charon:pr-XXX
            ├─> Pushes to registry
            ├─> Runs tests
            └─> Completes successfully
                    └─> Triggers supply-chain-verify.yml
                            ├─> Image now exists
                            ├─> Generates SBOM
                            ├─> Scans with Grype
                            └─> Posts results to PR
```

**Push to Main**:

```
Push to main
    └─> docker-build.yml runs
            └─> Completes successfully
                    └─> Triggers supply-chain-verify.yml
                            └─> Verifies SBOM and signatures
```

### Implementation Checklist

**Changes to `.github/workflows/supply-chain-verify.yml`**:

- [x] Update triggers section (remove pull_request, add workflow_run)
- [x] Add job conditional (check workflow_run.conclusion)
- [x] Update tag determination (handle workflow_run context)
- [x] Update PR comment logic (extract PR number correctly)

**Testing Plan**:

- [ ] Test PR workflow (verify sequential execution and correct tagging)
- [ ] Test push to main (verify 'latest' tag usage)
- [ ] Test manual trigger (verify workflow_dispatch works)
- [ ] Test scheduled run (verify weekly scan works)
- [ ] Test failed build scenario (verify verification doesn't run)

### Benefits

- ✅ Verification always runs AFTER image exists
- ✅ No more false "image not found" skips on PRs
- ✅ Manual verification via workflow_dispatch still works
- ✅ Scheduled weekly scans remain functional
- ✅ Only verifies successfully built images
- ✅ Clear separation of concerns

### Potential Issues & Mitigations

1. **workflow_run Limitations**: Can only chain 3 levels deep
   - Mitigation: We're only chaining 2 levels (safe)

2. **Branch Context**: workflow_run runs on default branch context
   - Mitigation: Extract correct branch/PR info from workflow_run metadata

3. **Failed Build Silent Skip**: If docker-build fails, verification doesn't run
   - Mitigation: This is desired behavior; failed builds shouldn't be verified

4. **Forked PRs**: workflow_run from forks may have limited permissions
   - Mitigation: Acceptable due to security constraints; docker-build loads images locally for PRs

### Security Considerations

- `workflow_run` runs with permissions of the target branch (prevents privilege escalation)
- Existing permissions in supply-chain-verify are appropriate (read-only for packages)
- Only runs after successfully built images (trust boundary maintained)

### Success Criteria

- ✅ Supply chain verification runs **after** docker-build completes
- ✅ Verification correctly identifies the built image tag
- ✅ PR comments are posted with actual verification results (not skips)
- ✅ Manual and scheduled triggers continue to work
- ✅ Failed builds do not trigger verification
- ✅ Workflow remains maintainable and modular

---

## Implementation Results

**Status**: ✅ All phases completed successfully

**Changes Made**:

1. ✅ Added `workflow_run` trigger to supply-chain-verify.yml
2. ✅ Removed `pull_request` trigger
3. ✅ Added workflow success filter
4. ✅ Enhanced tag determination logic
5. ✅ Updated PR comment extraction
6. ✅ Added debug logging for validation

**Validation**:

- ✅ Security audit passed (see [qa_report_workflow_orchestration.md](../../reports/qa_report_workflow_orchestration.md))
- ✅ Pre-commit hooks passed
- ✅ YAML syntax validated
- ✅ No breaking changes to other workflows

**Documentation**:

- [Implementation Summary](../../implementation/WORKFLOW_ORCHESTRATION_FIX.md)
- [QA Report](../../reports/qa_report_workflow_orchestration.md)

---

**Archived**: 2026-01-11
**Implementation Time**: ~2 hours
**Next Steps**: Monitor first production workflow_run execution
