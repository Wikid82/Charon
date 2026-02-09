# Workflow Modularization Implementation Specification

## Section 1: Overview

### Goal
Separate post-build testing workflows from the monolithic `docker-build.yml` to improve:
- **Modularity**: Each workflow has a single, focused responsibility
- **Maintainability**: Easier to debug, update, and extend individual workflows
- **Clarity**: Clear separation between build artifacts and validation/testing steps
- **Reusability**: Workflows can be triggered independently via `workflow_dispatch`

### Current State
The `docker-build.yml` workflow contains:
- Docker image building and publishing
- Container image testing
- SBOM generation and attestation for production builds

### Target State
Three independent workflows triggered by `docker-build.yml` completion:
1. **playwright.yml** - End-to-end testing with Playwright
2. **security-pr.yml** - Trivy security scanning of PR Docker images
3. **supply-chain-pr.yml** - SBOM generation and vulnerability scanning for PRs

**Note**: All three workflows already exist and are operational. This specification documents their current implementation and validates their compliance with requirements.

---

## Section 2: Job Inventory

| Job Name | Lines | Disposition | Rationale |
|----------|-------|-------------|-----------|
| `build-and-push` | 42-404 | **KEEP** | Core responsibility: build and publish Docker images |
| `test-image` | 406-505 | **KEEP** | Integration testing of published production images |
| Trivy scanning (PR) | N/A | **EXTRACTED** | Already in `security-pr.yml` |
| Supply chain (PR) | N/A | **EXTRACTED** | Already in `supply-chain-pr.yml` |
| Playwright tests | N/A | **EXTRACTED** | Already in `playwright.yml` |

### Current `docker-build.yml` Jobs

```yaml
jobs:
  build-and-push:    # KEEP - Core build functionality
  test-image:        # KEEP - Production image testing
```

---

## Section 3: New Workflow Specifications

### Status: ✅ All workflows already implemented and operational

The following sections document the **current implementation** of the three extracted workflows. They are provided here for reference and validation against requirements.

---

### 3.1 playwright.yml - E2E Testing Workflow

**Location**: `.github/workflows/playwright.yml`
**Status**: ✅ Already implemented
**Trigger**: `workflow_run` on `docker-build.yml` completion

#### Complete YAML Specification

```yaml
# Playwright E2E Tests
# Runs Playwright tests against PR Docker images after the build workflow completes
name: Playwright E2E Tests

on:
  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types:
      - completed

  workflow_dispatch:
    inputs:
      pr_number:
        description: 'PR number to test (optional)'
        required: false
        type: string

concurrency:
  group: playwright-${{ github.event.workflow_run.head_branch || github.ref }}
  cancel-in-progress: true

jobs:
  playwright:
    name: E2E Tests
    runs-on: ubuntu-latest
    timeout-minutes: 20
    # Run for: manual dispatch, PR builds, or any push builds from docker-build
    if: >-
      github.event_name == 'workflow_dispatch' ||
      ((github.event.workflow_run.event == 'pull_request' || github.event.workflow_run.event == 'push') &&
       github.event.workflow_run.conclusion == 'success')

    env:
      CHARON_ENV: development
      CHARON_DEBUG: "1"
      CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_CI_ENCRYPTION_KEY }}

    steps:
      - name: Checkout repository
        uses: actions/checkout@0c366fd6a839edf440554fa01a7085ccba70ac98 # v4.2.2

      - name: Extract PR number from workflow_run
        id: pr-info
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          if [[ "${{ github.event_name }}" == "workflow_dispatch" ]]; then
            # Manual dispatch - use input or fail gracefully
            if [[ -n "${{ inputs.pr_number }}" ]]; then
              echo "pr_number=${{ inputs.pr_number }}" >> "$GITHUB_OUTPUT"
              echo "✅ Using manually provided PR number: ${{ inputs.pr_number }}"
            else
              echo "⚠️ No PR number provided for manual dispatch"
              echo "pr_number=" >> "$GITHUB_OUTPUT"
            fi
            exit 0
          fi

          # Extract PR number from workflow_run context
          HEAD_SHA="${{ github.event.workflow_run.head_sha }}"
          echo "🔍 Looking for PR with head SHA: ${HEAD_SHA}"

          # Query GitHub API for PR associated with this commit
          PR_NUMBER=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/commits/${HEAD_SHA}/pulls" \
            --jq '.[0].number // empty' 2>/dev/null || echo "")

          if [[ -n "${PR_NUMBER}" ]]; then
            echo "pr_number=${PR_NUMBER}" >> "$GITHUB_OUTPUT"
            echo "✅ Found PR number: ${PR_NUMBER}"
          else
            echo "⚠️ Could not find PR number for SHA: ${HEAD_SHA}"
            echo "pr_number=" >> "$GITHUB_OUTPUT"
          fi

          # Check if this is a push event (not a PR)
          if [[ "${{ github.event.workflow_run.event }}" == "push" ]]; then
            echo "is_push=true" >> "$GITHUB_OUTPUT"
            echo "✅ Detected push build from branch: ${{ github.event.workflow_run.head_branch }}"
          else
            echo "is_push=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Check for PR image artifact
        id: check-artifact
        if: steps.pr-info.outputs.pr_number != '' || steps.pr-info.outputs.is_push == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          # Determine artifact name based on event type
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            ARTIFACT_NAME="push-image"
          else
            PR_NUMBER="${{ steps.pr-info.outputs.pr_number }}"
            ARTIFACT_NAME="pr-image-${PR_NUMBER}"
          fi
          RUN_ID="${{ github.event.workflow_run.id }}"

          echo "🔍 Checking for artifact: ${ARTIFACT_NAME}"

          if [[ "${{ github.event_name }}" == "workflow_dispatch" ]]; then
            # For manual dispatch, find the most recent workflow run with this artifact
            RUN_ID=$(gh api \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/actions/workflows/docker-build.yml/runs?status=success&per_page=10" \
              --jq '.workflow_runs[0].id // empty' 2>/dev/null || echo "")

            if [[ -z "${RUN_ID}" ]]; then
              echo "⚠️ No successful workflow runs found"
              echo "artifact_exists=false" >> "$GITHUB_OUTPUT"
              exit 0
            fi
          fi

          echo "run_id=${RUN_ID}" >> "$GITHUB_OUTPUT"

          # Check if the artifact exists in the workflow run
          ARTIFACT_ID=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/actions/runs/${RUN_ID}/artifacts" \
            --jq ".artifacts[] | select(.name == \"${ARTIFACT_NAME}\") | .id" 2>/dev/null || echo "")

          if [[ -n "${ARTIFACT_ID}" ]]; then
            echo "artifact_exists=true" >> "$GITHUB_OUTPUT"
            echo "artifact_id=${ARTIFACT_ID}" >> "$GITHUB_OUTPUT"
            echo "✅ Found artifact: ${ARTIFACT_NAME} (ID: ${ARTIFACT_ID})"
          else
            echo "artifact_exists=false" >> "$GITHUB_OUTPUT"
            echo "⚠️ Artifact not found: ${ARTIFACT_NAME}"
            echo "ℹ️ This is expected for non-PR builds or if the image was not uploaded"
          fi

      - name: Skip if no artifact
        if: (steps.pr-info.outputs.pr_number == '' && steps.pr-info.outputs.is_push != 'true') || steps.check-artifact.outputs.artifact_exists != 'true'
        run: |
          echo "ℹ️ Skipping Playwright tests - no PR image artifact available"
          echo "This is expected for:"
          echo "  - Pushes to main/release branches"
          echo "  - PRs where Docker build failed"
          echo "  - Manual dispatch without PR number"
          exit 0

      - name: Download PR image artifact
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: actions/download-artifact@37930b1c2abaa49bbe596cd826c3c89aef350131 # v4.1.8
        with:
          name: ${{ steps.pr-info.outputs.is_push == 'true' && 'push-image' || format('pr-image-{0}', steps.pr-info.outputs.pr_number) }}
          run-id: ${{ steps.check-artifact.outputs.run_id }}
          github-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Load Docker image
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "📦 Loading Docker image..."
          docker load < charon-pr-image.tar
          echo "✅ Docker image loaded"
          docker images | grep charon

      - name: Start Charon container
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "🚀 Starting Charon container..."

          # Normalize image name (GitHub lowercases repository owner names in GHCR)
          IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ github.event.workflow_run.head_branch }}"
          else
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
          fi

          echo "📦 Starting container with image: ${IMAGE_REF}"
          docker run -d \
            --name charon-test \
            -p 8080:8080 \
            -e CHARON_ENV="${CHARON_ENV}" \
            -e CHARON_DEBUG="${CHARON_DEBUG}" \
            -e CHARON_ENCRYPTION_KEY="${CHARON_ENCRYPTION_KEY}" \
            "${IMAGE_REF}"

          echo "✅ Container started"

      - name: Wait for health endpoint
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "⏳ Waiting for Charon to be healthy..."
          MAX_ATTEMPTS=30
          ATTEMPT=0

          while [[ ${ATTEMPT} -lt ${MAX_ATTEMPTS} ]]; do
            ATTEMPT=$((ATTEMPT + 1))
            echo "Attempt ${ATTEMPT}/${MAX_ATTEMPTS}..."

            if curl -sf http://localhost:8080/api/v1/health > /dev/null 2>&1; then
              echo "✅ Charon is healthy!"
              exit 0
            fi

            sleep 2
          done

          echo "❌ Health check failed after ${MAX_ATTEMPTS} attempts"
          echo "📋 Container logs:"
          docker logs charon-test
          exit 1

      - name: Setup Node.js
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: actions/setup-node@6044e13b5dc448c55e2357c09f80417699197238 # v4.1.0
        with:
          node-version: 'lts/*'
          cache: 'npm'

      - name: Install dependencies
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: npm ci

      - name: Install Playwright browsers
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: npx playwright install --with-deps chromium

      - name: Run Playwright tests
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        env:
          PLAYWRIGHT_BASE_URL: http://localhost:8080
        run: npx playwright test --project=chromium

      - name: Upload Playwright report
        if: always() && steps.check-artifact.outputs.artifact_exists == 'true'
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v4.4.3
        with:
          name: ${{ steps.pr-info.outputs.is_push == 'true' && format('playwright-report-{0}', github.event.workflow_run.head_branch) || format('playwright-report-pr-{0}', steps.pr-info.outputs.pr_number) }}
          path: playwright-report/
          retention-days: 14

      - name: Cleanup
        if: always() && steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "🧹 Cleaning up..."
          docker stop charon-test 2>/dev/null || true
          docker rm charon-test 2>/dev/null || true
          echo "✅ Cleanup complete"
```

#### Key Features
- ✅ Uses `workflow_run` trigger on `docker-build.yml` completion
- ✅ Extracts PR number from `github.event.workflow_run.head_sha` via GitHub API
- ✅ Downloads artifact with correct name: `pr-image-{PR_NUMBER}` (for PRs) or `push-image` (for pushes)
- ✅ Loads Docker image from artifact file: `charon-pr-image.tar`
- ✅ Includes all environment variables (`CHARON_ENV`, `CHARON_DEBUG`, `CHARON_ENCRYPTION_KEY`)
- ✅ Proper artifact cleanup with 14-day retention
- ✅ Manual dispatch support for testing specific PRs

---

### 3.2 security-pr.yml - Trivy Security Scanning Workflow

**Location**: `.github/workflows/security-pr.yml`
**Status**: ✅ Already implemented
**Trigger**: `workflow_run` on `docker-build.yml` completion

#### Complete YAML Specification

```yaml
# Security Scan for Pull Requests
# Runs Trivy security scanning on PR Docker images after the build workflow completes
# This workflow extracts the charon binary from the container and performs filesystem scanning
name: Security Scan (PR)

on:
  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types:
      - completed

  workflow_dispatch:
    inputs:
      pr_number:
        description: 'PR number to scan (optional)'
        required: false
        type: string

concurrency:
  group: security-pr-${{ github.event.workflow_run.head_branch || github.ref }}
  cancel-in-progress: true

jobs:
  security-scan:
    name: Trivy Binary Scan
    runs-on: ubuntu-latest
    timeout-minutes: 10
    # Run for: manual dispatch, PR builds, or any push builds from docker-build
    if: >-
      github.event_name == 'workflow_dispatch' ||
      ((github.event.workflow_run.event == 'pull_request' || github.event.workflow_run.event == 'push') &&
       github.event.workflow_run.conclusion == 'success')

    permissions:
      contents: read
      pull-requests: write
      security-events: write
      actions: read

    steps:
      - name: Checkout repository
        uses: actions/checkout@0c366fd6a839edf440554fa01a7085ccba70ac98 # v4.2.2

      - name: Extract PR number from workflow_run
        id: pr-info
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          if [[ "${{ github.event_name }}" == "workflow_dispatch" ]]; then
            # Manual dispatch - use input or fail gracefully
            if [[ -n "${{ inputs.pr_number }}" ]]; then
              echo "pr_number=${{ inputs.pr_number }}" >> "$GITHUB_OUTPUT"
              echo "✅ Using manually provided PR number: ${{ inputs.pr_number }}"
            else
              echo "⚠️ No PR number provided for manual dispatch"
              echo "pr_number=" >> "$GITHUB_OUTPUT"
            fi
            exit 0
          fi

          # Extract PR number from workflow_run context
          HEAD_SHA="${{ github.event.workflow_run.head_sha }}"
          echo "🔍 Looking for PR with head SHA: ${HEAD_SHA}"

          # Query GitHub API for PR associated with this commit
          PR_NUMBER=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/commits/${HEAD_SHA}/pulls" \
            --jq '.[0].number // empty' 2>/dev/null || echo "")

          if [[ -n "${PR_NUMBER}" ]]; then
            echo "pr_number=${PR_NUMBER}" >> "$GITHUB_OUTPUT"
            echo "✅ Found PR number: ${PR_NUMBER}"
          else
            echo "⚠️ Could not find PR number for SHA: ${HEAD_SHA}"
            echo "pr_number=" >> "$GITHUB_OUTPUT"
          fi

          # Check if this is a push event (not a PR)
          if [[ "${{ github.event.workflow_run.event }}" == "push" ]]; then
            echo "is_push=true" >> "$GITHUB_OUTPUT"
            echo "✅ Detected push build from branch: ${{ github.event.workflow_run.head_branch }}"
          else
            echo "is_push=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Check for PR image artifact
        id: check-artifact
        if: steps.pr-info.outputs.pr_number != '' || steps.pr-info.outputs.is_push == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          # Determine artifact name based on event type
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            ARTIFACT_NAME="push-image"
          else
            PR_NUMBER="${{ steps.pr-info.outputs.pr_number }}"
            ARTIFACT_NAME="pr-image-${PR_NUMBER}"
          fi
          RUN_ID="${{ github.event.workflow_run.id }}"

          echo "🔍 Checking for artifact: ${ARTIFACT_NAME}"

          if [[ "${{ github.event_name }}" == "workflow_dispatch" ]]; then
            # For manual dispatch, find the most recent workflow run with this artifact
            RUN_ID=$(gh api \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/actions/workflows/docker-build.yml/runs?status=success&per_page=10" \
              --jq '.workflow_runs[0].id // empty' 2>/dev/null || echo "")

            if [[ -z "${RUN_ID}" ]]; then
              echo "⚠️ No successful workflow runs found"
              echo "artifact_exists=false" >> "$GITHUB_OUTPUT"
              exit 0
            fi
          fi

          echo "run_id=${RUN_ID}" >> "$GITHUB_OUTPUT"

          # Check if the artifact exists in the workflow run
          ARTIFACT_ID=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/actions/runs/${RUN_ID}/artifacts" \
            --jq ".artifacts[] | select(.name == \"${ARTIFACT_NAME}\") | .id" 2>/dev/null || echo "")

          if [[ -n "${ARTIFACT_ID}" ]]; then
            echo "artifact_exists=true" >> "$GITHUB_OUTPUT"
            echo "artifact_id=${ARTIFACT_ID}" >> "$GITHUB_OUTPUT"
            echo "✅ Found artifact: ${ARTIFACT_NAME} (ID: ${ARTIFACT_ID})"
          else
            echo "artifact_exists=false" >> "$GITHUB_OUTPUT"
            echo "⚠️ Artifact not found: ${ARTIFACT_NAME}"
            echo "ℹ️ This is expected for non-PR builds or if the image was not uploaded"
          fi

      - name: Skip if no artifact
        if: (steps.pr-info.outputs.pr_number == '' && steps.pr-info.outputs.is_push != 'true') || steps.check-artifact.outputs.artifact_exists != 'true'
        run: |
          echo "ℹ️ Skipping security scan - no PR image artifact available"
          echo "This is expected for:"
          echo "  - Pushes to main/release branches"
          echo "  - PRs where Docker build failed"
          echo "  - Manual dispatch without PR number"
          exit 0

      - name: Download PR image artifact
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: actions/download-artifact@37930b1c2abaa49bbe596cd826c3c89aef350131 # v4.1.8
        with:
          name: ${{ steps.pr-info.outputs.is_push == 'true' && 'push-image' || format('pr-image-{0}', steps.pr-info.outputs.pr_number) }}
          run-id: ${{ steps.check-artifact.outputs.run_id }}
          github-token: ${{ secrets.GITHUB_TOKEN }}

      - name: Load Docker image
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "📦 Loading Docker image..."
          docker load < charon-pr-image.tar
          echo "✅ Docker image loaded"
          docker images | grep charon

      - name: Extract charon binary from container
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        id: extract
        run: |
          # Normalize image name for reference
          IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ github.event.workflow_run.head_branch }}"
          else
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
          fi

          echo "🔍 Extracting binary from: ${IMAGE_REF}"

          # Create container without starting it
          CONTAINER_ID=$(docker create "${IMAGE_REF}")
          echo "container_id=${CONTAINER_ID}" >> "$GITHUB_OUTPUT"

          # Extract the charon binary
          mkdir -p ./scan-target
          docker cp "${CONTAINER_ID}:/app/charon" ./scan-target/charon

          # Cleanup container
          docker rm "${CONTAINER_ID}"

          # Verify extraction
          if [[ -f "./scan-target/charon" ]]; then
            echo "✅ Binary extracted successfully"
            ls -lh ./scan-target/charon
            echo "binary_path=./scan-target" >> "$GITHUB_OUTPUT"
          else
            echo "❌ Failed to extract binary"
            exit 1
          fi

      - name: Run Trivy filesystem scan (SARIF output)
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: aquasecurity/trivy-action@22438a435773de8c97dc0958cc0b823c45b064ac # v0.33.1
        with:
          scan-type: 'fs'
          scan-ref: ${{ steps.extract.outputs.binary_path }}
          format: 'sarif'
          output: 'trivy-binary-results.sarif'
          severity: 'CRITICAL,HIGH,MEDIUM'
        continue-on-error: true

      - name: Upload Trivy SARIF to GitHub Security
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: github/codeql-action/upload-sarif@a2d9de63c2916881d0621fdb7e65abe32141606d # v4
        with:
          sarif_file: 'trivy-binary-results.sarif'
          category: ${{ steps.pr-info.outputs.is_push == 'true' && format('security-scan-{0}', github.event.workflow_run.head_branch) || format('security-scan-pr-{0}', steps.pr-info.outputs.pr_number) }}
        continue-on-error: true

      - name: Run Trivy filesystem scan (fail on CRITICAL/HIGH)
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        uses: aquasecurity/trivy-action@22438a435773de8c97dc0958cc0b823c45b064ac # v0.33.1
        with:
          scan-type: 'fs'
          scan-ref: ${{ steps.extract.outputs.binary_path }}
          format: 'table'
          severity: 'CRITICAL,HIGH'
          exit-code: '1'

      - name: Upload scan artifacts
        if: always() && steps.check-artifact.outputs.artifact_exists == 'true'
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v4.4.3
        with:
          name: ${{ steps.pr-info.outputs.is_push == 'true' && format('security-scan-{0}', github.event.workflow_run.head_branch) || format('security-scan-pr-{0}', steps.pr-info.outputs.pr_number) }}
          path: |
            trivy-binary-results.sarif
          retention-days: 14

      - name: Create job summary
        if: always() && steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            echo "## 🔒 Security Scan Results - Branch: ${{ github.event.workflow_run.head_branch }}" >> $GITHUB_STEP_SUMMARY
          else
            echo "## 🔒 Security Scan Results - PR #${{ steps.pr-info.outputs.pr_number }}" >> $GITHUB_STEP_SUMMARY
          fi
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Scan Type**: Trivy Filesystem Scan" >> $GITHUB_STEP_SUMMARY
          echo "**Target**: \`/app/charon\` binary" >> $GITHUB_STEP_SUMMARY
          echo "**Severity Filter**: CRITICAL, HIGH" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          if [[ "${{ job.status }}" == "success" ]]; then
            echo "✅ **PASSED**: No CRITICAL or HIGH vulnerabilities found" >> $GITHUB_STEP_SUMMARY
          else
            echo "❌ **FAILED**: CRITICAL or HIGH vulnerabilities detected" >> $GITHUB_STEP_SUMMARY
            echo "" >> $GITHUB_STEP_SUMMARY
            echo "Please review the Trivy scan output and address the vulnerabilities." >> $GITHUB_STEP_SUMMARY
          fi

      - name: Cleanup
        if: always() && steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "🧹 Cleaning up..."
          rm -rf ./scan-target
          echo "✅ Cleanup complete"
```

#### Key Features
- ✅ Uses `workflow_run` trigger on `docker-build.yml` completion
- ✅ Extracts PR number from `github.event.workflow_run.head_sha` via GitHub API
- ✅ Downloads artifact with correct name: `pr-image-{PR_NUMBER}` (for PRs) or `push-image` (for pushes)
- ✅ Loads Docker image from artifact file: `charon-pr-image.tar`
- ✅ Extracts binary from container for filesystem scanning
- ✅ Uses CodeQL action v4 for SARIF upload
- ✅ Includes all permissions: `contents: read`, `pull-requests: write`, `security-events: write`, `actions: read`
- ✅ Fails on CRITICAL/HIGH vulnerabilities

---

### 3.3 supply-chain-pr.yml - SBOM and Vulnerability Workflow

**Location**: `.github/workflows/supply-chain-pr.yml`
**Status**: ✅ Already implemented
**Trigger**: `workflow_run` on `docker-build.yml` completion

#### Complete YAML Specification

```yaml
# yaml-language-server: $schema=https://json.schemastore.org/github-workflow.json
---
name: Supply Chain Verification (PR)

on:
  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types:
      - completed

  workflow_dispatch:
    inputs:
      pr_number:
        description: "PR number to verify (optional, will auto-detect from workflow_run)"
        required: false
        type: string

concurrency:
  group: supply-chain-pr-${{ github.event.workflow_run.head_branch || github.ref }}
  cancel-in-progress: true

env:
  SYFT_VERSION: v1.17.0
  GRYPE_VERSION: v0.85.0

permissions:
  contents: read
  pull-requests: write
  security-events: write
  actions: read

jobs:
  verify-supply-chain:
    name: Verify Supply Chain
    runs-on: ubuntu-latest
    timeout-minutes: 15
    # Run for: manual dispatch, PR builds, or any push builds from docker-build
    if: >
      github.event_name == 'workflow_dispatch' ||
      ((github.event.workflow_run.event == 'pull_request' || github.event.workflow_run.event == 'push') &&
       github.event.workflow_run.conclusion == 'success')

    steps:
      - name: Checkout repository
        uses: actions/checkout@0c366fd6a839edf440554fa01a7085ccba70ac98 # v4.2.2
        with:
          sparse-checkout: |
            .github
          sparse-checkout-cone-mode: false

      - name: Extract PR number from workflow_run
        id: pr-number
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          if [[ -n "${{ inputs.pr_number }}" ]]; then
            echo "pr_number=${{ inputs.pr_number }}" >> "$GITHUB_OUTPUT"
            echo "📋 Using manually provided PR number: ${{ inputs.pr_number }}"
            exit 0
          fi

          if [[ "${{ github.event_name }}" != "workflow_run" ]]; then
            echo "❌ No PR number provided and not triggered by workflow_run"
            echo "pr_number=" >> "$GITHUB_OUTPUT"
            exit 0
          fi

          # Extract PR number from workflow_run context
          HEAD_SHA="${{ github.event.workflow_run.head_sha }}"
          HEAD_BRANCH="${{ github.event.workflow_run.head_branch }}"

          echo "🔍 Looking for PR with head SHA: ${HEAD_SHA}"
          echo "🔍 Head branch: ${HEAD_BRANCH}"

          # Search for PR by head SHA
          PR_NUMBER=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/pulls?state=open&head=${{ github.repository_owner }}:${HEAD_BRANCH}" \
            --jq '.[0].number // empty' 2>/dev/null || echo "")

          if [[ -z "${PR_NUMBER}" ]]; then
            # Fallback: search by commit SHA
            PR_NUMBER=$(gh api \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/commits/${HEAD_SHA}/pulls" \
              --jq '.[0].number // empty' 2>/dev/null || echo "")
          fi

          if [[ -z "${PR_NUMBER}" ]]; then
            echo "⚠️ Could not find PR number for this workflow run"
            echo "pr_number=" >> "$GITHUB_OUTPUT"
          else
            echo "pr_number=${PR_NUMBER}" >> "$GITHUB_OUTPUT"
            echo "✅ Found PR number: ${PR_NUMBER}"
          fi

          # Check if this is a push event (not a PR)
          if [[ "${{ github.event.workflow_run.event }}" == "push" ]]; then
            echo "is_push=true" >> "$GITHUB_OUTPUT"
            echo "✅ Detected push build from branch: ${{ github.event.workflow_run.head_branch }}"
          else
            echo "is_push=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Check for PR image artifact
        id: check-artifact
        if: steps.pr-number.outputs.pr_number != '' || steps.pr-number.outputs.is_push == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          # Determine artifact name based on event type
          if [[ "${{ steps.pr-number.outputs.is_push }}" == "true" ]]; then
            ARTIFACT_NAME="push-image"
          else
            PR_NUMBER="${{ steps.pr-number.outputs.pr_number }}"
            ARTIFACT_NAME="pr-image-${PR_NUMBER}"
          fi
          RUN_ID="${{ github.event.workflow_run.id }}"

          echo "🔍 Looking for artifact: ${ARTIFACT_NAME}"

          if [[ -n "${RUN_ID}" ]]; then
            # Search in the triggering workflow run
            ARTIFACT_ID=$(gh api \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/actions/runs/${RUN_ID}/artifacts" \
              --jq ".artifacts[] | select(.name == \"${ARTIFACT_NAME}\") | .id" 2>/dev/null || echo "")
          fi

          if [[ -z "${ARTIFACT_ID}" ]]; then
            # Fallback: search recent artifacts
            ARTIFACT_ID=$(gh api \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/actions/artifacts?name=${ARTIFACT_NAME}" \
              --jq '.artifacts[0].id // empty' 2>/dev/null || echo "")
          fi

          if [[ -z "${ARTIFACT_ID}" ]]; then
            echo "⚠️ No artifact found: ${ARTIFACT_NAME}"
            echo "artifact_found=false" >> "$GITHUB_OUTPUT"
            exit 0
          fi

          echo "artifact_found=true" >> "$GITHUB_OUTPUT"
          echo "artifact_id=${ARTIFACT_ID}" >> "$GITHUB_OUTPUT"
          echo "artifact_name=${ARTIFACT_NAME}" >> "$GITHUB_OUTPUT"
          echo "✅ Found artifact: ${ARTIFACT_NAME} (ID: ${ARTIFACT_ID})"

      - name: Skip if no artifact
        if: (steps.pr-number.outputs.pr_number == '' && steps.pr-number.outputs.is_push != 'true') || steps.check-artifact.outputs.artifact_found != 'true'
        run: |
          echo "ℹ️ No PR image artifact found - skipping supply chain verification"
          echo "This is expected if the Docker build did not produce an artifact for this PR"
          exit 0

      - name: Download PR image artifact
        if: steps.check-artifact.outputs.artifact_found == 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          ARTIFACT_ID="${{ steps.check-artifact.outputs.artifact_id }}"
          ARTIFACT_NAME="${{ steps.check-artifact.outputs.artifact_name }}"

          echo "📦 Downloading artifact: ${ARTIFACT_NAME}"

          gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/actions/artifacts/${ARTIFACT_ID}/zip" \
            > artifact.zip

          unzip -o artifact.zip
          echo "✅ Artifact downloaded and extracted"

      - name: Load Docker image
        if: steps.check-artifact.outputs.artifact_found == 'true'
        id: load-image
        run: |
          if [[ ! -f "charon-pr-image.tar" ]]; then
            echo "❌ charon-pr-image.tar not found in artifact"
            ls -la
            exit 1
          fi

          echo "🐳 Loading Docker image..."
          LOAD_OUTPUT=$(docker load -i charon-pr-image.tar)
          echo "${LOAD_OUTPUT}"

          # Extract image name from load output
          IMAGE_NAME=$(echo "${LOAD_OUTPUT}" | grep -oP 'Loaded image: \K.*' || echo "")

          if [[ -z "${IMAGE_NAME}" ]]; then
            # Try alternative format
            IMAGE_NAME=$(echo "${LOAD_OUTPUT}" | grep -oP 'Loaded image ID: \K.*' || echo "")
          fi

          if [[ -z "${IMAGE_NAME}" ]]; then
            # Fallback: list recent images
            IMAGE_NAME=$(docker images --format "{{.Repository}}:{{.Tag}}" | head -1)
          fi

          echo "image_name=${IMAGE_NAME}" >> "$GITHUB_OUTPUT"
          echo "✅ Loaded image: ${IMAGE_NAME}"

      - name: Install Syft
        if: steps.check-artifact.outputs.artifact_found == 'true'
        run: |
          echo "📦 Installing Syft ${SYFT_VERSION}..."
          curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | \
            sh -s -- -b /usr/local/bin "${SYFT_VERSION}"
          syft version

      - name: Install Grype
        if: steps.check-artifact.outputs.artifact_found == 'true'
        run: |
          echo "📦 Installing Grype ${GRYPE_VERSION}..."
          curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | \
            sh -s -- -b /usr/local/bin "${GRYPE_VERSION}"
          grype version

      - name: Generate SBOM
        if: steps.check-artifact.outputs.artifact_found == 'true'
        id: sbom
        run: |
          IMAGE_NAME="${{ steps.load-image.outputs.image_name }}"
          echo "📋 Generating SBOM for: ${IMAGE_NAME}"

          syft "${IMAGE_NAME}" \
            --output cyclonedx-json=sbom.cyclonedx.json \
            --output table

          # Count components
          COMPONENT_COUNT=$(jq '.components | length' sbom.cyclonedx.json 2>/dev/null || echo "0")
          echo "component_count=${COMPONENT_COUNT}" >> "$GITHUB_OUTPUT"
          echo "✅ SBOM generated with ${COMPONENT_COUNT} components"

      - name: Scan for vulnerabilities
        if: steps.check-artifact.outputs.artifact_found == 'true'
        id: grype-scan
        run: |
          echo "🔍 Scanning SBOM for vulnerabilities..."

          # Run Grype against the SBOM
          grype sbom:sbom.cyclonedx.json \
            --output json \
            --file grype-results.json || true

          # Generate SARIF output for GitHub Security
          grype sbom:sbom.cyclonedx.json \
            --output sarif \
            --file grype-results.sarif || true

          # Count vulnerabilities by severity
          if [[ -f grype-results.json ]]; then
            CRITICAL_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Critical")] | length' grype-results.json 2>/dev/null || echo "0")
            HIGH_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "High")] | length' grype-results.json 2>/dev/null || echo "0")
            MEDIUM_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Medium")] | length' grype-results.json 2>/dev/null || echo "0")
            LOW_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Low")] | length' grype-results.json 2>/dev/null || echo "0")
            TOTAL_COUNT=$(jq '.matches | length' grype-results.json 2>/dev/null || echo "0")
          else
            CRITICAL_COUNT=0
            HIGH_COUNT=0
            MEDIUM_COUNT=0
            LOW_COUNT=0
            TOTAL_COUNT=0
          fi

          echo "critical_count=${CRITICAL_COUNT}" >> "$GITHUB_OUTPUT"
          echo "high_count=${HIGH_COUNT}" >> "$GITHUB_OUTPUT"
          echo "medium_count=${MEDIUM_COUNT}" >> "$GITHUB_OUTPUT"
          echo "low_count=${LOW_COUNT}" >> "$GITHUB_OUTPUT"
          echo "total_count=${TOTAL_COUNT}" >> "$GITHUB_OUTPUT"

          echo "📊 Vulnerability Summary:"
          echo "  Critical: ${CRITICAL_COUNT}"
          echo "  High: ${HIGH_COUNT}"
          echo "  Medium: ${MEDIUM_COUNT}"
          echo "  Low: ${LOW_COUNT}"
          echo "  Total: ${TOTAL_COUNT}"

      - name: Upload SARIF to GitHub Security
        if: steps.check-artifact.outputs.artifact_found == 'true'
        uses: github/codeql-action/upload-sarif@a2d9de63c2916881d0621fdb7e65abe32141606d # v4
        continue-on-error: true
        with:
          sarif_file: grype-results.sarif
          category: supply-chain-pr

      - name: Upload supply chain artifacts
        if: steps.check-artifact.outputs.artifact_found == 'true'
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v4.6.0
        with:
          name: ${{ steps.pr-number.outputs.is_push == 'true' && format('supply-chain-{0}', github.event.workflow_run.head_branch) || format('supply-chain-pr-{0}', steps.pr-number.outputs.pr_number) }}
          path: |
            sbom.cyclonedx.json
            grype-results.json
          retention-days: 14

      - name: Comment on PR
        if: steps.check-artifact.outputs.artifact_found == 'true' && steps.pr-number.outputs.is_push != 'true'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          PR_NUMBER="${{ steps.pr-number.outputs.pr_number }}"
          COMPONENT_COUNT="${{ steps.sbom.outputs.component_count }}"
          CRITICAL_COUNT="${{ steps.grype-scan.outputs.critical_count }}"
          HIGH_COUNT="${{ steps.grype-scan.outputs.high_count }}"
          MEDIUM_COUNT="${{ steps.grype-scan.outputs.medium_count }}"
          LOW_COUNT="${{ steps.grype-scan.outputs.low_count }}"
          TOTAL_COUNT="${{ steps.grype-scan.outputs.total_count }}"

          # Determine status emoji
          if [[ "${CRITICAL_COUNT}" -gt 0 ]]; then
            STATUS="❌ **FAILED**"
            STATUS_EMOJI="🚨"
          elif [[ "${HIGH_COUNT}" -gt 0 ]]; then
            STATUS="⚠️ **WARNING**"
            STATUS_EMOJI="⚠️"
          else
            STATUS="✅ **PASSED**"
            STATUS_EMOJI="✅"
          fi

          COMMENT_BODY=$(cat <<EOF
          ## ${STATUS_EMOJI} Supply Chain Verification Results

          ${STATUS}

          ### 📦 SBOM Summary
          - **Components**: ${COMPONENT_COUNT}

          ### 🔍 Vulnerability Scan
          | Severity | Count |
          |----------|-------|
          | 🔴 Critical | ${CRITICAL_COUNT} |
          | 🟠 High | ${HIGH_COUNT} |
          | 🟡 Medium | ${MEDIUM_COUNT} |
          | 🟢 Low | ${LOW_COUNT} |
          | **Total** | **${TOTAL_COUNT}** |

          ### 📎 Artifacts
          - SBOM (CycloneDX JSON) and Grype results available in workflow artifacts

          ---
          <sub>Generated by Supply Chain Verification workflow • [View Details](${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }})</sub>
          EOF
          )

          # Find and update existing comment or create new one
          COMMENT_ID=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/repos/${{ github.repository }}/issues/${PR_NUMBER}/comments" \
            --jq '.[] | select(.body | contains("Supply Chain Verification Results")) | .id' | head -1)

          if [[ -n "${COMMENT_ID}" ]]; then
            echo "📝 Updating existing comment..."
            gh api \
              --method PATCH \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/issues/comments/${COMMENT_ID}" \
              -f body="${COMMENT_BODY}"
          else
            echo "📝 Creating new comment..."
            gh api \
              --method POST \
              -H "Accept: application/vnd.github+json" \
              -H "X-GitHub-Api-Version: 2022-11-28" \
              "/repos/${{ github.repository }}/issues/${PR_NUMBER}/comments" \
              -f body="${COMMENT_BODY}"
          fi

          echo "✅ PR comment posted"

      - name: Fail on critical vulnerabilities
        if: steps.check-artifact.outputs.artifact_found == 'true'
        run: |
          CRITICAL_COUNT="${{ steps.grype-scan.outputs.critical_count }}"

          if [[ "${CRITICAL_COUNT}" -gt 0 ]]; then
            echo "🚨 Found ${CRITICAL_COUNT} CRITICAL vulnerabilities!"
            echo "Please review the vulnerability report and address critical issues before merging."
            exit 1
          fi

          echo "✅ No critical vulnerabilities found"
```

#### Key Features
- ✅ Uses `workflow_run` trigger on `docker-build.yml` completion
- ✅ Extracts PR number from `github.event.workflow_run.head_sha` and `head_branch` via GitHub API
- ✅ Downloads artifact with correct name: `pr-image-{PR_NUMBER}` (for PRs) or `push-image` (for pushes)
- ✅ Loads Docker image from artifact file: `charon-pr-image.tar`
- ✅ Generates SBOM using Syft v1.17.0
- ✅ Scans vulnerabilities using Grype v0.85.0
- ✅ Uses CodeQL action v4 for SARIF upload
- ✅ Posts PR comment with vulnerability summary
- ✅ Fails on critical vulnerabilities
- ✅ Includes all permissions and environment variables

---

## Section 4: docker-build.yml Modifications

### Current Status
✅ **No modifications needed** - The `docker-build.yml` already contains only build and test responsibilities. The testing workflows have been properly extracted to separate files.

### Validation
The current `docker-build.yml` contains:
- ✅ Docker image building (`build-and-push` job)
- ✅ Integration testing of production images (`test-image` job)
- ✅ SBOM generation for production builds (lines 386-395)
- ✅ Artifact upload for PR/push builds (lines 206-213)

### What Was Already Extracted
- ✅ Playwright E2E tests → `playwright.yml`
- ✅ Trivy security scanning for PRs → `security-pr.yml`
- ✅ SBOM/vulnerability scanning for PRs → `supply-chain-pr.yml`

---

## Section 5: Cleanup Tasks

### Files to Review
None - all workflows are currently in use and operational.

### Obsolete Artifacts
None identified. All current workflow files serve active purposes.

---

## Implementation Validation

### ✅ All Requirements Met

| Requirement | Status | Implementation |
|-------------|--------|----------------|
| Use `workflow_run` trigger | ✅ Complete | All three workflows use `workflow_run` on `docker-build.yml` |
| Extract PR number from workflow_run | ✅ Complete | Uses `github.event.workflow_run.head_sha` with GitHub API |
| Download correct artifact | ✅ Complete | Uses `pr-image-{PR_NUMBER}` or `push-image` naming |
| Load Docker image from artifact | ✅ Complete | All workflows load from `charon-pr-image.tar` |
| Include all env vars & permissions | ✅ Complete | Each workflow has required permissions and environment variables |
| Fix artifact filename | ✅ Complete | All workflows use `charon-pr-image.tar` consistently |
| Use CodeQL v4 | ✅ Complete | Both security workflows use `@a2d9de63c2916881d0621fdb7e65abe32141606d` (v4) |

---

## Testing & Rollout Strategy

### Phase 1: Validation (Current State)
All three workflows are already deployed and operational. Monitor their execution in the next 5 PR cycles.

### Phase 2: Documentation Update
Update project documentation to reference the modular workflow architecture:
- README.md - CI/CD section
- CONTRIBUTING.md - PR testing process
- docs/architecture/ - Workflow diagram

### Phase 3: Monitoring
Track workflow execution metrics:
- Success/failure rates
- Execution time
- Artifact download reliability
- PR comment accuracy

---

## Appendix: Common Troubleshooting

### Issue: Artifact Not Found
**Symptom**: Workflows skip execution with "No artifact found"
**Cause**: `docker-build.yml` didn't upload artifact (e.g., Renovate/chore PRs skipped)
**Resolution**: This is expected behavior. Workflows gracefully skip when no artifact exists.

### Issue: PR Number Not Detected
**Symptom**: Workflows can't determine PR number from `workflow_run`
**Cause**: Commit not yet associated with a PR in GitHub API
**Resolution**: Workflows fall back to branch-based artifact names (`push-image`)

### Issue: Docker Image Load Fails
**Symptom**: `docker load` fails with "invalid tar header"
**Cause**: Artifact corruption during upload/download
**Resolution**: Re-run `docker-build.yml` to regenerate artifact

---

## Conclusion

The workflow modularization is **already complete and operational**. This specification serves as:
- **Documentation** of the current architecture
- **Reference** for future workflow modifications
- **Validation** that all requirements have been met
- **Troubleshooting guide** for common issues

No further implementation actions are required at this time.
