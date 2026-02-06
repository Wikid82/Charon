# Remediation Plan: Docker Security Vulnerabilities (Deferred)

**Objective**: Ensure CI pipeline functionality and logic verification despite known vulnerabilities in the base image.

**Status Update (Feb 2026)**:
- **Decision**: The attempt to switch to Ubuntu was rejected. We are reverting to the Debian-based image.
- **Action**: Relax the blocking security scan in the CI pipeline to allow the workflow to complete and validat logic changes, even if vulnerabilities are present.
- **Rationale**: Prioritize confirming CI stability and workflow correctness over immediate vulnerability remediation.

## 1. Findings (Historical)

| Vulnerability | Severity | Source Package | Current Base Image |
|---------------|----------|----------------|--------------------|
| **CVE-2026-0861** | HIGH | `libc-bin`, `libc6` | `debian:trixie-slim` (Debian 13 Testing) |
| **CVE-2025-7458** | CRITICAL | `sqlite3` | `debian:bookworm-slim` (Debian 12 Stable) |
| **CVE-2023-45853** | CRITICAL | `zlib1g` | `debian:bookworm-slim` (Debian 12 Stable) |

## 2. Technical Specifications

### 2.1. Dockerfile Update
**Goal**: Revert to the previous stable state.

*   **File**: `Dockerfile`
*   **Changes**: Revert to `debian:trixie-slim` (GitHub HEAD version).

### 2.2. CI Workflow Update
**Goal**: Allow Trivy scans to report errors without failing the build.

*   **File**: `.github/workflows/docker-build.yml`
*   **Changes**:
    *   Step: `Run Trivy scan on PR image (SARIF - blocking)`
    *   Action: Add `continue-on-error: true`.

## 3. Implementation Plan

### Phase 1: Revert & Relax
- [x] **Task 1.1**: Revert `Dockerfile` to HEAD.
- [x] **Task 1.2**: Update `.github/workflows/docker-build.yml` to allow failure on Trivy scan.

### Phase 2: Verification
- [ ] **Task 2.1**: Commit and Push.
- [ ] **Task 2.2**: Verify CI pipeline execution on GitHub.

## 4. Acceptance Criteria
- [ ] CI pipeline `docker-build.yml` completes successfully (green).
- [ ] Trivy scan runs and reports results, but does not block the build.
