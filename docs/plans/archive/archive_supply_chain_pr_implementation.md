# CI Docker Build Failure Analysis & Fix Plan

**Issue**: Docker Build workflow failing on PR builds during image artifact save
**Workflow**: `.github/workflows/docker-build.yml`
**Error**: `Error response from daemon: reference does not exist`
**Date**: 2026-01-12
**Status**: Analysis Complete - Ready for Implementation

---

## Executive Summary

The `docker-build.yml` workflow is failing at the "Save Docker Image as Artifact" step (line 135-142) for PR builds. The root cause is a **mismatch between the image name/tag format used by `docker/build-push-action` with `load: true` and the image reference used in the `docker save` command**.

**Impact**: All PR builds fail at the artifact save step, preventing the `verify-supply-chain-pr` job from running.

**Fix Complexity**: Low - Single line change to use correct image reference format.

---

## Root Cause Analysis

### The Failing Step (Lines 135-142)

```yaml
- name: Save Docker Image as Artifact
  if: github.event_name == 'pull_request'
  run: |
    IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
    docker save ghcr.io/${IMAGE_NAME}:pr-${{ github.event.pull_request.number }} -o /tmp/charon-pr-image.tar
    ls -lh /tmp/charon-pr-image.tar
```

**Line 140**: Normalizes the image name to lowercase (e.g., `Wikid82/charon` → `wikid82/charon`)
**Line 141**: Attempts to save the image with the full registry path: `ghcr.io/wikid82/charon:pr-123`

### The Build Step (Lines 111-123)

```yaml
- name: Build and push Docker image
  if: steps.skip.outputs.skip_build != 'true'
  id: build-and-push
  uses: docker/build-push-action@263435318d21b8e681c14492fe198d362a7d2c83 # v6
  with:
    context: .
    platforms: ${{ github.event_name == 'pull_request' && 'linux/amd64' || 'linux/amd64,linux/arm64' }}
    push: ${{ github.event_name != 'pull_request' }}
    load: ${{ github.event_name == 'pull_request' }}
    tags: ${{ steps.meta.outputs.tags }}
    labels: ${{ steps.meta.outputs.labels }}
    no-cache: true
    pull: true
    build-args: |
      VERSION=${{ steps.meta.outputs.version }}
      BUILD_DATE=${{ fromJSON(steps.meta.outputs.json).labels['org.opencontainers.image.created'] }}
      VCS_REF=${{ github.sha }}
      CADDY_IMAGE=${{ steps.caddy.outputs.image }}
```

**Key Parameters for PR Builds**:

- `push: false` (line 117)
- `load: true` (line 118) - **This loads the image into the local Docker daemon**
- `tags: ${{ steps.meta.outputs.tags }}` (line 119)

### The Metadata Step (Lines 105-113)

```yaml
- name: Extract metadata (tags, labels)
  if: steps.skip.outputs.skip_build != 'true'
  id: meta
  uses: docker/metadata-action@c299e40c65443455700f0fdfc63efafe5b349051 # v5.10.0
  with:
    images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    tags: |
      type=raw,value=latest,enable={{is_default_branch}}
      type=raw,value=dev,enable=${{ github.ref == 'refs/heads/development' }}
      type=raw,value=beta,enable=${{ github.ref == 'refs/heads/feature/beta-release' }}
      type=raw,value=pr-${{ github.event.pull_request.number }},enable=${{ github.event_name == 'pull_request' }}
      type=sha,format=short,enable=${{ github.event_name != 'pull_request' }}
```

**For PR builds**, only this tag is enabled (line 111):

- `type=raw,value=pr-${{ github.event.pull_request.number }}`

This generates the tag: `ghcr.io/${IMAGE_NAME}:pr-${PR_NUMBER}`

**Example**: For PR #123 with owner "Wikid82", the tag would be:

- Input to metadata-action: `ghcr.io/wikid82/charon` (already normalized at line 56-57)
- Generated tag: `ghcr.io/wikid82/charon:pr-123`

### The Critical Issue

**When `docker/build-push-action` uses `load: true`**, the behavior depends on the Docker Buildx backend:

1. **Expected Behavior**: Image is loaded into local Docker daemon with the tags specified in `tags:`
2. **Actual Behavior**: The image might be loaded with **tags but without guaranteed registry prefix** OR the tags might not all be applied to the local image

**Evidence from Docker Build-Push-Action Documentation**:
> When using `load: true`, the image is loaded into the local Docker daemon. However, **multi-platform builds cannot be loaded** (they require `push: true`), so only single-platform builds work with `load: true`.

**The Problem**: The `docker save` command at line 141 references:

```bash
ghcr.io/${IMAGE_NAME}:pr-${{ github.event.pull_request.number }}
```

But the image loaded locally might be tagged as:

- `ghcr.io/wikid82/charon:pr-123` ✅ (correct - what we expect)
- `wikid82/charon:pr-123` ❌ (missing registry prefix)
- Or the image might exist but with a different tag format

### Why This Matters

The `docker save` command requires an **exact match** of the image name and tag as it exists in the local Docker daemon. If the image is loaded as `wikid82/charon:pr-123` but we're trying to save `ghcr.io/wikid82/charon:pr-123`, Docker will throw:

```
Error response from daemon: reference does not exist
```

This is **exactly the error we're seeing**.

### Job Dependencies Analysis

Looking at the complete workflow structure:

```
build-and-push (lines 34-234)
├── Outputs: skip_build, digest
├── Steps include:
│   ├── Build image (load=true for PRs)
│   ├── Save image artifact (FAILS HERE) ❌
│   └── Upload artifact (never reached)
│
test-image (lines 354-463)
├── needs: build-and-push
├── if: needs.build-and-push.outputs.skip_build != 'true' && github.event_name != 'pull_request'
└── (Not relevant for PRs)
│
trivy-pr-app-only (lines 465-493)
├── if: github.event_name == 'pull_request'
└── (Independent - builds its own image)
│
verify-supply-chain-pr (lines 495-722)
├── needs: build-and-push
├── if: github.event_name == 'pull_request' && needs.build-and-push.result == 'success'
├── Steps include:
│   ├── Download artifact (NEVER RUNS - artifact doesn't exist) ❌
│   ├── Load image
│   └── Scan image
└── (Currently skipped due to build-and-push failure)
│
verify-supply-chain-pr-skipped (lines 724-754)
├── needs: build-and-push
└── if: github.event_name == 'pull_request' && needs.build-and-push.outputs.skip_build == 'true'
```

**Dependency Chain Impact**:

1. ❌ `build-and-push` fails at line 141 (docker save)
2. ❌ Artifact is never uploaded (lines 144-150)
3. ❌ `verify-supply-chain-pr` cannot download artifact (line 517) - job is marked as "skipped" or "failed"
4. ❌ Supply chain verification never runs for PRs

### Verification of the Issue

Looking at similar patterns in the file that **work correctly**:

**Line 376** (in `test-image` job):

```yaml
- name: Normalize image name
  run: |
    raw="${{ github.repository_owner }}/${{ github.event.repository.name }}"
    IMAGE_NAME=$(echo "$raw" | tr '[:upper:]' '[:lower:]')
    echo "IMAGE_NAME=${IMAGE_NAME}" >> $GITHUB_ENV
```

This job **doesn't load images locally** - it pulls from the registry (line 395):

```yaml
- name: Pull Docker image
  run: docker pull ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.tag.outputs.tag }}
```

So this pattern works because it's pulling from a pushed image, not a locally loaded one.

**Line 516** (in `verify-supply-chain-pr` job):

```yaml
- name: Normalize image name
  run: |
    IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
    echo "IMAGE_NAME=${IMAGE_NAME}" >> $GITHUB_ENV
```

This step **expects to load the image from an artifact** (lines 511-520), so it doesn't directly reference a registry image. The image is loaded from the tar file uploaded by `build-and-push`.

**Key Difference**: The `verify-supply-chain-pr` job expects the artifact to exist, but since `build-and-push` fails at the `docker save` step, the artifact is never created.

---

## Technical Design

### Workflow-Level Configuration

**Tool Versions** (extracted as environment variables):

- `SYFT_VERSION`: v1.17.0
- `GRYPE_VERSION`: v0.85.0

These will be defined at the workflow level to ensure consistency and easier updates.

### Job Definitions

**Job 1: Image Artifact Upload** (modification to existing `build-and-push` job)
**Trigger**: Only for `pull_request` events
**Purpose**: Save and upload the built Docker image as an artifact

**Job 2: `verify-supply-chain-pr`**
**Trigger**: Only for `pull_request` events
**Dependency**: `needs: build-and-push`
**Purpose**: Download image artifact, perform SBOM generation and vulnerability scanning
**Skip Conditions**:

- If `build-and-push` output `skip_build == 'true'`
- If `build-and-push` did not succeed

**Job 3: `verify-supply-chain-pr-skipped`**
**Trigger**: Only for `pull_request` events
**Dependency**: `needs: build-and-push`
**Purpose**: Provide user feedback when build is skipped
**Run Condition**: If `build-and-push` output `skip_build == 'true'`

### Key Technical Decisions

#### Decision 1: Image Sharing Strategy

**Chosen Approach**: Save image as tar archive and share via GitHub Actions artifacts
**Why**:

- Jobs run in isolated environments; local Docker images are not shared by default
- Artifacts provide reliable cross-job data sharing
- Avoids registry push for PR builds (maintains current security model)
- 1-day retention minimizes storage costs
**Alternative Considered**: Push to registry with ephemeral tags (rejected: requires registry permissions, security concerns, cleanup complexity)

#### Decision 2: Tool Versions

**Syft**: v1.17.0 (matches existing security-verify-sbom skill)
**Grype**: v0.85.0 (matches existing security-verify-sbom skill)
**Why**: Consistent with existing workflows, tested versions

#### Decision 3: Failure Behavior

**Critical Vulnerabilities**: Fail the job (exit code 1)
**High Vulnerabilities**: Warn but don't fail
**Why**: Aligns with project standards (see security-verify-sbom.SKILL.md)

#### Decision 4: SARIF Category Strategy

**Category Format**: `supply-chain-pr-${{ github.event.pull_request.number }}-${{ github.sha }}`
**Why**: Including SHA prevents conflicts when multiple commits are pushed to the same PR concurrently
**Without SHA**: Concurrent uploads to the same category would overwrite each other

#### Decision 5: Null Safety in Outputs

**Approach**: Add explicit null checks and fallback values for all step outputs
**Why**:

- Step outputs may be undefined if steps are skipped or fail
- Prevents workflow failures in reporting steps
- Ensures graceful degradation of user feedback

#### Decision 6: Workflow Conflict Resolution

**Issue**: `supply-chain-verify.yml` currently handles PR workflow_run events, creating duplicate verification
**Solution**: Update `supply-chain-verify.yml` to exclude PR builds from workflow_run triggers
**Why**: Inline verification in docker-build.yml provides faster feedback; workflow_run is unnecessary for PRs

---

## Implementation Steps

### Step 1: Update Workflow Environment Variables

**File**: `.github/workflows/docker-build.yml`
**Location**: After line 22 (after existing `env:` section start)
**Action**: Add tool version variables

```yaml
env:
  # ... existing variables ...
  SYFT_VERSION: v1.17.0
  GRYPE_VERSION: v0.85.0
```

### Step 2: Add Artifact Upload to build-and-push Job

**File**: `.github/workflows/docker-build.yml`
**Location**: After the "Build and push Docker image" step (after line 113)
**Action**: Insert two new steps for image artifact handling

```yaml
    - name: Save Docker Image as Artifact
      if: github.event_name == 'pull_request'
      run: |
        IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
        docker save ghcr.io/${IMAGE_NAME}:pr-${{ github.event.pull_request.number }} -o /tmp/charon-pr-image.tar
        ls -lh /tmp/charon-pr-image.tar

    - name: Upload Image Artifact
      if: github.event_name == 'pull_request'
      uses: actions/upload-artifact@b4b15b8c7c6ac21ea08fcf65892d2ee8f75cf882 # v4.4.3
      with:
        name: pr-image-${{ github.event.pull_request.number }}
        path: /tmp/charon-pr-image.tar
        retention-days: 1
```

**Rationale**: These steps execute only for PRs and share the built image with downstream jobs.

### Step 3: Add verify-supply-chain-pr Job

**File**: `.github/workflows/docker-build.yml`
**Location**: After line 229 (end of `trivy-pr-app-only` job)
**Action**: Insert complete job definition

See complete YAML in Appendix A.

### Step 4: Add verify-supply-chain-pr-skipped Job

**File**: `.github/workflows/docker-build.yml`
**Location**: After the `verify-supply-chain-pr` job
**Action**: Insert complete job definition

See complete YAML in Appendix B.

### Step 5: Update supply-chain-verify.yml to Avoid PR Conflicts

**File**: `.github/workflows/supply-chain-verify.yml`
**Location**: Update the `verify-sbom` job condition (around line 68)
**Current**:

```yaml
if: |
  (github.event_name != 'schedule' || github.ref == 'refs/heads/main') &&
  (github.event_name != 'workflow_run' || github.event.workflow_run.conclusion == 'success')
```

**Updated**:

```yaml
if: |
  (github.event_name != 'schedule' || github.ref == 'refs/heads/main') &&
  (github.event_name != 'workflow_run' ||
    (github.event.workflow_run.conclusion == 'success' &&
     github.event.workflow_run.event != 'pull_request'))
```

**Rationale**: Prevents duplicate supply chain verification for PRs. The inline job in docker-build.yml now handles PR verification.

---
**Generate**:

- SBOM file (CycloneDX JSON)
- Vulnerability scan results (JSON)
- GitHub SARIF report (for Security tab integration)

**Upload**: All as workflow artifacts with 30-day retention

---

## Detailed Implementation

This implementation includes 3 main components:

1. **Workflow-level environment variables** for tool versions
2. **Modifications to `build-and-push` job** to upload image artifact
3. **Two new jobs**: `verify-supply-chain-pr` (main verification) and `verify-supply-chain-pr-skipped` (feedback)
4. **Update to `supply-chain-verify.yml`** to prevent duplicate verification

See complete YAML job definitions in Appendix A and B.

### Insertion Instructions

**Location in docker-build.yml**:

- Environment variables: After line 22
- Image artifact upload: After line 113 (in build-and-push job)
- New jobs: After line 229 (end of `trivy-pr-app-only` job)

**No modifications needed to other existing jobs**. The `build-and-push` job already outputs everything we need.

---

## Testing Plan

### Phase 1: Basic Validation

1. Create test PR on `feature/beta-release`
2. Verify artifact upload/download works correctly
3. Verify image loads successfully in verification job
4. Check image reference is correct (no "image not found")
5. Validate SBOM generation (component count >0)
6. Validate vulnerability scanning
7. Check PR comment is posted with status/table (including commit SHA)
8. Verify SARIF upload to Security tab with unique category
9. Verify job summary is created with all null checks working

### Phase 2: Critical Fixes Validation

1. **Image Access**: Verify artifact contains image tar, verify download succeeds, verify docker load works
2. **Conditionals**: Test that job skips when build-and-push fails or is skipped
3. **SARIF Category**: Push multiple commits to same PR, verify no SARIF conflicts in Security tab
4. **Null Checks**: Force step failure, verify job summary and PR comment still generate gracefully
5. **Workflow Conflict**: Verify supply-chain-verify.yml does NOT trigger for PR builds
6. **Skipped Feedback**: Create chore commit, verify skipped feedback job posts comment

### Phase 3: Edge Cases

1. Test with intentionally vulnerable dependency
2. Test with build skip (chore commit)
3. Test concurrent PRs (verify artifacts don't collide)
4. Test rapid successive commits to same PR

### Phase 4: Performance Validation

1. Measure baseline PR build time (without feature)
2. Measure new PR build time (with feature)
3. Verify increase is within expected 50-60% range
4. Monitor artifact storage usage

### Phase 5: Rollback

If issues arise, revert the commit. No impact on main/tag builds.

---

## Success Criteria

### Functional

- ✅ Artifacts are uploaded/downloaded correctly for all PR builds
- ✅ Image loads successfully in verification job
- ✅ Job runs for all PR builds (when not skipped)
- ✅ Job correctly skips when build-and-push fails or is skipped
- ✅ Generates valid SBOM
- ✅ Performs vulnerability scan
- ✅ Uploads artifacts with appropriate retention
- ✅ Comments on PR with commit SHA and vulnerability table
- ✅ Fails on critical vulnerabilities
- ✅ Uploads SARIF with unique category (no conflicts)
- ✅ Skipped build feedback is posted when build is skipped
- ✅ No duplicate verification from supply-chain-verify.yml

### Performance

- ⏱️ Completes in <15 minutes
- 📦 Artifact size <250MB
- 📈 Total PR build time increase: 50-60% (acceptable)

### Reliability

- 🔒 All null checks in place (no undefined variable errors)
- 🔄 Handles concurrent PR commits without conflicts
- ✅ Graceful degradation if steps fail

---

## Appendix A: Complete verify-supply-chain-pr Job YAML

```yaml
  # ============================================================================
  # Supply Chain Verification for PR Builds
  # ============================================================================
  # This job performs SBOM generation and vulnerability scanning for PR builds.
  # It depends on the build-and-push job completing successfully and uses the
  # Docker image artifact uploaded by that job.
  #
  # Dependency Chain: build-and-push (builds & uploads) → verify-supply-chain-pr (downloads & scans)
  # ============================================================================
  verify-supply-chain-pr:
    name: Supply Chain Verification (PR)
    needs: build-and-push
    runs-on: ubuntu-latest
    timeout-minutes: 15
    # Critical Fix #2: Enhanced conditional with result check
    if: |
      github.event_name == 'pull_request' &&
      needs.build-and-push.outputs.skip_build != 'true' &&
      needs.build-and-push.result == 'success'
    permissions:
      contents: read
      pull-requests: write
      security-events: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@8e8c483db84b4bee98b60c0593521ed34d9990e8 # v6

      # Critical Fix #1: Download image artifact
      - name: Download Image Artifact
        uses: actions/download-artifact@fa0a91b85d4f404e444e00e005971372dc801d16 # v4.1.8
        with:
          name: pr-image-${{ github.event.pull_request.number }}

      # Critical Fix #1: Load Docker image
      - name: Load Docker Image
        run: |
          docker load -i charon-pr-image.tar
          docker images
          echo "✅ Image loaded successfully"

      - name: Normalize image name
        run: |
          IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
          echo "IMAGE_NAME=${IMAGE_NAME}" >> $GITHUB_ENV

      - name: Set PR image reference
        id: image
        run: |
          IMAGE_REF="ghcr.io/${{ env.IMAGE_NAME }}:pr-${{ github.event.pull_request.number }}"
          echo "ref=${IMAGE_REF}" >> $GITHUB_OUTPUT
          echo "📦 Will verify: ${IMAGE_REF}"

      - name: Install Verification Tools
        run: |
          # Use workflow-level environment variables for versions
          curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin ${{ env.SYFT_VERSION }}
          curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin ${{ env.GRYPE_VERSION }}
          syft version
          grype version

      - name: Generate SBOM
        id: sbom
        run: |
          echo "🔍 Generating SBOM for ${{ steps.image.outputs.ref }}..."
          if ! syft ${{ steps.image.outputs.ref }} -o cyclonedx-json > sbom-pr.cyclonedx.json; then
            echo "❌ SBOM generation failed"
            exit 1
          fi
          COMPONENT_COUNT=$(jq '.components | length' sbom-pr.cyclonedx.json 2>/dev/null || echo "0")
          echo "📦 SBOM contains ${COMPONENT_COUNT} components"
          if [[ ${COMPONENT_COUNT} -eq 0 ]]; then
            echo "⚠️ WARNING: SBOM contains no components"
            exit 1
          fi
          echo "component_count=${COMPONENT_COUNT}" >> $GITHUB_OUTPUT

      - name: Scan for Vulnerabilities
        id: scan
        run: |
          echo "🔍 Scanning for vulnerabilities..."
          grype db update
          if ! grype sbom:./sbom-pr.cyclonedx.json --output json --file vuln-scan.json; then
            echo "❌ Vulnerability scan failed"
            exit 1
          fi
          echo ""
          echo "=== Vulnerability Summary ==="
          grype sbom:./sbom-pr.cyclonedx.json --output table || true
          CRITICAL=$(jq '[.matches[] | select(.vulnerability.severity == "Critical")] | length' vuln-scan.json 2>/dev/null || echo "0")
          HIGH=$(jq '[.matches[] | select(.vulnerability.severity == "High")] | length' vuln-scan.json 2>/dev/null || echo "0")
          MEDIUM=$(jq '[.matches[] | select(.vulnerability.severity == "Medium")] | length' vuln-scan.json 2>/dev/null || echo "0")
          LOW=$(jq '[.matches[] | select(.vulnerability.severity == "Low")] | length' vuln-scan.json 2>/dev/null || echo "0")
          echo ""
          echo "📊 Vulnerability Breakdown:"
          echo "  🔴 Critical: ${CRITICAL}"
          echo "  🟠 High: ${HIGH}"
          echo "  🟡 Medium: ${MEDIUM}"
          echo "  🟢 Low: ${LOW}"
          echo "critical=${CRITICAL}" >> $GITHUB_OUTPUT
          echo "high=${HIGH}" >> $GITHUB_OUTPUT
          echo "medium=${MEDIUM}" >> $GITHUB_OUTPUT
          echo "low=${LOW}" >> $GITHUB_OUTPUT
          if [[ ${CRITICAL} -gt 0 ]]; then
            echo "::error::${CRITICAL} CRITICAL vulnerabilities found - BLOCKING"
          fi
          if [[ ${HIGH} -gt 0 ]]; then
            echo "::warning::${HIGH} HIGH vulnerabilities found"
          fi

      - name: Generate SARIF Report
        if: always()
        run: |
          echo "📋 Generating SARIF report..."
          grype sbom:./sbom-pr.cyclonedx.json --output sarif --file grype-results.sarif || true

      # Critical Fix #3: SARIF category includes SHA to prevent conflicts
      - name: Upload SARIF to GitHub Security
        if: always()
        uses: github/codeql-action/upload-sarif@5d4e8d1aca955e8d8589aabd499c5cae939e33c7 # v4.31.9
        with:
          sarif_file: grype-results.sarif
          category: supply-chain-pr-${{ github.event.pull_request.number }}-${{ github.sha }}
        continue-on-error: true

      - name: Upload Artifacts
        if: always()
        uses: actions/upload-artifact@b4b15b8c7c6ac21ea08fcf65892d2ee8f75cf882 # v4.4.3
        with:
          name: supply-chain-pr-${{ github.event.pull_request.number }}
          path: |
            sbom-pr.cyclonedx.json
            vuln-scan.json
            grype-results.sarif
          retention-days: 30

      # Critical Fix #4: Null checks in PR comment
      - name: Comment on PR
        if: always()
        uses: actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea # v7.0.1
        with:
          script: |
            const critical = '${{ steps.scan.outputs.critical }}' || '0';
            const high = '${{ steps.scan.outputs.high }}' || '0';
            const medium = '${{ steps.scan.outputs.medium }}' || '0';
            const low = '${{ steps.scan.outputs.low }}' || '0';
            const components = '${{ steps.sbom.outputs.component_count }}' || 'N/A';
            const commitSha = '${{ github.sha }}'.substring(0, 7);

            let status = '✅ **PASSED**';
            let statusEmoji = '✅';

            if (parseInt(critical) > 0) {
              status = '❌ **BLOCKED** - Critical vulnerabilities found';
              statusEmoji = '❌';
            } else if (parseInt(high) > 0) {
              status = '⚠️ **WARNING** - High vulnerabilities found';
              statusEmoji = '⚠️';
            }

            const body = `## ${statusEmoji} Supply Chain Verification (PR Build)

            **Status**: ${status}
            **Commit**: \`${commitSha}\`
            **Image**: \`${{ steps.image.outputs.ref }}\`
            **Components Scanned**: ${components}

            ### 📊 Vulnerability Summary

            | Severity | Count |
            |----------|-------|
            | 🔴 Critical | ${critical} |
            | 🟠 High | ${high} |
            | 🟡 Medium | ${medium} |
            | 🟢 Low | ${low} |

            ${parseInt(critical) > 0 ? '### ❌ Critical Vulnerabilities Detected\n\n**Action Required**: This PR cannot be merged until critical vulnerabilities are resolved.\n\n' : ''}
            ${parseInt(high) > 0 ? '### ⚠️ High Vulnerabilities Detected\n\n**Recommendation**: Review and address high-severity vulnerabilities before merging.\n\n' : ''}
            📋 [View Full Report](${context.serverUrl}/${context.repo.owner}/${context.repo.repo}/actions/runs/${context.runId})
            📦 [Download Artifacts](${context.serverUrl}/${context.repo.owner}/${context.repo.repo}/actions/runs/${context.runId}#artifacts)
            `;

            await github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: body
            });

      - name: Fail on Critical Vulnerabilities
        if: steps.scan.outputs.critical != '0'
        run: |
          echo "❌ CRITICAL: ${{ steps.scan.outputs.critical }} critical vulnerabilities found"
          echo "This PR is blocked from merging until critical vulnerabilities are resolved."
          exit 1

      # Critical Fix #4: Null checks in job summary
      - name: Create Job Summary
        if: always()
        run: |
          # Use default values if outputs are not set
          COMPONENT_COUNT="${{ steps.sbom.outputs.component_count }}"
          CRITICAL="${{ steps.scan.outputs.critical }}"
          HIGH="${{ steps.scan.outputs.high }}"
          MEDIUM="${{ steps.scan.outputs.medium }}"
          LOW="${{ steps.scan.outputs.low }}"

          # Apply defaults
          COMPONENT_COUNT="${COMPONENT_COUNT:-N/A}"
          CRITICAL="${CRITICAL:-0}"
          HIGH="${HIGH:-0}"
          MEDIUM="${MEDIUM:-0}"
          LOW="${LOW:-0}"

          echo "## 🔒 Supply Chain Verification - PR #${{ github.event.pull_request.number }}" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Image**: \`${{ steps.image.outputs.ref }}\`" >> $GITHUB_STEP_SUMMARY
          echo "**Components**: ${COMPONENT_COUNT}" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "### Vulnerability Breakdown" >> $GITHUB_STEP_SUMMARY
          echo "- 🔴 Critical: ${CRITICAL}" >> $GITHUB_STEP_SUMMARY
          echo "- 🟠 High: ${HIGH}" >> $GITHUB_STEP_SUMMARY
          echo "- 🟡 Medium: ${MEDIUM}" >> $GITHUB_STEP_SUMMARY
          echo "- 🟢 Low: ${LOW}" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY

          if [[ ${CRITICAL} -gt 0 ]]; then
            echo "❌ **BLOCKED**: Critical vulnerabilities must be resolved" >> $GITHUB_STEP_SUMMARY
          elif [[ ${HIGH} -gt 0 ]]; then
            echo "⚠️ **WARNING**: High vulnerabilities detected" >> $GITHUB_STEP_SUMMARY
          else
            echo "✅ **PASSED**: No critical or high vulnerabilities" >> $GITHUB_STEP_SUMMARY
          fi
```

---

## Appendix B: verify-supply-chain-pr-skipped Job YAML

```yaml
  # ============================================================================
  # Supply Chain Verification - Skipped Feedback
  # ============================================================================
  # This job provides user feedback when the build is skipped (e.g., chore commits).
  # Critical Fix #7: User feedback for skipped builds
  # ============================================================================
  verify-supply-chain-pr-skipped:
    name: Supply Chain Verification (Skipped)
    needs: build-and-push
    runs-on: ubuntu-latest
    if: |
      github.event_name == 'pull_request' &&
      needs.build-and-push.outputs.skip_build == 'true'
    permissions:
      pull-requests: write

    steps:
      - name: Comment on PR - Build Skipped
        uses: actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea # v7.0.1
        with:
          script: |
            const commitSha = '${{ github.sha }}'.substring(0, 7);
            const body = `## ⏭️ Supply Chain Verification (Skipped)

            **Commit**: \`${commitSha}\`
            **Reason**: Build was skipped (likely a documentation-only or chore commit)

            Supply chain verification is not performed for skipped builds. If this commit should trigger a build, ensure it includes changes to application code or dependencies.
            `;

            await github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: body
            });
```

---

**END OF IMPLEMENTATION PLAN**
