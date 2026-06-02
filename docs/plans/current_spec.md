# Fix: Add Disk Space Reclamation to Nightly Build Jobs

## 1. Introduction

### Overview

The `build-and-push-nightly` and `build-and-push-nightly-orthrus` jobs in
`.github/workflows/nightly-build.yml` crash with `System.IO.IOException: No space left on
device` during multi-platform Docker builds (`linux/amd64,linux/arm64`). The `ubuntu-latest`
GitHub Actions runner starts with approximately 14 GB of free disk space, but pre-installed
toolchains (Android SDK ~8 GB, .NET ~2 GB, Haskell ~2 GB) consume most of it before any
build step executes. When the disk fills mid-build, the runner process dies without sending
terminal step statuses, leaving GitHub's UI showing the job as simultaneously "failed" and
"in progress".

### Objectives

1. Reclaim 10–15 GB of disk space on both build jobs before any Docker-related step runs.
2. Insert a single `Free disk space` step as the **first step** in each affected job.
3. Pin the action to commit SHA per the project's existing SHA-pinning convention.
4. Preserve Docker images already present on the runner (`docker-images: false`) so Buildx
   can operate normally.

---

## 2. Research Findings

### 2.1 Affected Jobs

| Job | First step (current) | QEMU step |
|-----|----------------------|-----------|
| `build-and-push-nightly` | `Checkout nightly branch` | `Set up QEMU` (step 3) |
| `build-and-push-nightly-orthrus` | `Checkout nightly branch` | `Set up QEMU` (step 3) |

Both jobs follow an identical preamble:

```
1. Checkout nightly branch   (actions/checkout)
2. Set lowercase image name  (run: echo ...)
3. Set up QEMU               (docker/setup-qemu-action)
4. Set up Docker Buildx      (docker/setup-buildx-action)
```

### 2.2 Existing SHA-Pinning Convention

Every action in `nightly-build.yml` is pinned to a full 40-character commit SHA with a
version comment on the same line, for example:

```yaml
uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
uses: docker/setup-qemu-action@06116385d9baf250c9f4dcb4858b16962ea869c3  # v4.1.0
uses: docker/setup-buildx-action@d7f5e7f509e45cec5c76c4d5afdd7de93d0b3df5  # v4.1.0
```

The new step must follow this exact pattern.

### 2.3 Action Details

| Property | Value |
|----------|-------|
| Action | `jlumbroso/free-disk-space` |
| Version | v1.3.1 |
| Commit SHA | `54081f138730dfa15788a46383842cd2f914a1be` |
| Marketplace | https://github.com/jlumbroso/free-disk-space |

### 2.4 Configuration Rationale

| Input | Value | Reason |
|-------|-------|--------|
| `android` | `true` | Android SDK (~8 GB) — not needed by any Charon build step |
| `dotnet` | `true` | .NET SDK (~2 GB) — not needed |
| `haskell` | `true` | Haskell GHC/Stack (~2 GB) — not needed |
| `large-packages` | `true` | Additional apt packages (~3–4 GB) — not needed |
| `docker-images` | `false` | **Must stay false** — Buildx relies on pre-pulled images |
| `swap-storage` | `true` | Reclaim ~4 GB swap file space |
| `tool-cache` | `false` | Keep cached tools; not a significant source of waste |

**Expected recovered space:** 10–15 GB, leaving ~24–29 GB available before the build begins.

### 2.5 CI Failure Behaviour

When disk space is exhausted during a multi-platform `docker/build-push-action` run, the
runner OS-level write fails, which kills the runner worker process outright. Because the
process does not exit cleanly, it never sends the `complete` status event back to GitHub for
each step, resulting in:

- The job appearing as **failed** (runner reported failure on re-connect timeout)
- Individual steps remaining **in progress** in the UI (never received terminal status)
- No actionable log output past the point of failure

---

## 3. Technical Specification

### 3.1 File to Modify

```
.github/workflows/nightly-build.yml
```

### 3.2 Exact YAML Step to Insert

The following step block must be inserted as the **first step** (before `Checkout nightly
branch`) in both affected jobs:

```yaml
      - name: Free disk space
        uses: jlumbroso/free-disk-space@54081f138730dfa15788a46383842cd2f914a1be  # v1.3.1
        with:
          android: true
          dotnet: true
          haskell: true
          large-packages: true
          docker-images: false
          swap-storage: true
          tool-cache: false
```

### 3.3 Insertion Points

#### Job: `build-and-push-nightly`

Insert **before** the `Checkout nightly branch` step.

**Before (current first step):**
```yaml
    steps:
      - name: Checkout nightly branch
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
        with:
          ref: ${{ github.event_name == 'workflow_dispatch' && github.ref || 'nightly' }}
          fetch-depth: 0
```

**After (with new first step):**
```yaml
    steps:
      - name: Free disk space
        uses: jlumbroso/free-disk-space@54081f138730dfa15788a46383842cd2f914a1be  # v1.3.1
        with:
          android: true
          dotnet: true
          haskell: true
          large-packages: true
          docker-images: false
          swap-storage: true
          tool-cache: false

      - name: Checkout nightly branch
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
        with:
          ref: ${{ github.event_name == 'workflow_dispatch' && github.ref || 'nightly' }}
          fetch-depth: 0
```

#### Job: `build-and-push-nightly-orthrus`

Insert **before** the `Checkout nightly branch` step in the orthrus job.

**Before (current first step):**
```yaml
    steps:
      - name: Checkout nightly branch
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
        with:
          ref: ${{ github.event_name == 'workflow_dispatch' && github.ref || 'nightly' }}
          fetch-depth: 0
```

**After (with new first step):**
```yaml
    steps:
      - name: Free disk space
        uses: jlumbroso/free-disk-space@54081f138730dfa15788a46383842cd2f914a1be  # v1.3.1
        with:
          android: true
          dotnet: true
          haskell: true
          large-packages: true
          docker-images: false
          swap-storage: true
          tool-cache: false

      - name: Checkout nightly branch
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
        with:
          ref: ${{ github.event_name == 'workflow_dispatch' && github.ref || 'nightly' }}
          fetch-depth: 0
```

### 3.4 Step Ordering (Both Jobs, Post-Change)

| # | Step name | Notes |
|---|-----------|-------|
| 1 | **Free disk space** | NEW — reclaims 10–15 GB |
| 2 | Checkout nightly branch | unchanged |
| 3 | Set lowercase image name | unchanged |
| 4 | Set up QEMU | unchanged |
| 5 | Set up Docker Buildx | unchanged |
| 6+ | … remaining steps unchanged | |

---

## 4. Implementation Plan

### Phase 1: Playwright Tests

No UI changes are introduced. This fix is CI-infrastructure only; no Playwright tests are
required or applicable.

### Phase 2: Backend Implementation

Not applicable. This fix is GitHub Actions workflow YAML only.

### Phase 3: Frontend Implementation

Not applicable.

### Phase 4: Workflow Change

**File:** `.github/workflows/nightly-build.yml`

**Edit 1 — `build-and-push-nightly` job**

In the `steps:` block of `build-and-push-nightly`, insert the `Free disk space` step block
immediately before the `- name: Checkout nightly branch` step so it becomes the first step
in the job.

**Edit 2 — `build-and-push-nightly-orthrus` job**

In the `steps:` block of `build-and-push-nightly-orthrus`, insert the identical `Free disk
space` step block immediately before the `- name: Checkout nightly branch` step so it
becomes the first step in the job.

No other jobs, steps, or keys in the file are touched.

### Phase 5: Integration and Testing

1. After merging the PR, trigger `nightly-build.yml` manually via `workflow_dispatch`.
2. Confirm both `build-and-push-nightly` and `build-and-push-nightly-orthrus` complete
   without `No space left on device` errors.
3. Confirm the `Free disk space` step is listed first in the GitHub Actions UI for both jobs
   and reports recovered space in its output log.
4. Confirm the subsequent `Set up QEMU` and `Set up Docker Buildx` steps succeed, validating
   that `docker-images: false` preserved the Docker daemon state.

### Phase 6: Documentation and Deployment

No documentation changes required beyond this plan. The commit message is sufficient.

---

## 5. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|--------------|
| 1 | `Free disk space` step is the first step in `build-and-push-nightly` | Inspect YAML and GitHub Actions UI |
| 2 | `Free disk space` step is the first step in `build-and-push-nightly-orthrus` | Inspect YAML and GitHub Actions UI |
| 3 | Action is pinned to SHA `54081f138730dfa15788a46383842cd2f914a1be` with comment `# v1.3.1` | Code review / grep |
| 4 | `docker-images: false` is set (Docker daemon state preserved) | Code review |
| 5 | Both jobs complete without `No space left on device` error on next nightly run | CI run log |
| 6 | Multi-platform push (`linux/amd64,linux/arm64`) succeeds for both images | CI run log |
| 7 | No other steps, jobs, or keys in the workflow file are modified | Diff review |

---

## 6. Commit Slicing Strategy

### Decision

**Single PR · Single Commit.** This is a two-hunk edit to one YAML file with zero
functional ambiguity. There is no benefit to splitting further.

### Trigger Reasons for Single Commit

- Scope is contained: one file, two identical insertions.
- No risk of partial deployment — both jobs must be fixed simultaneously or the nightly
  workflow remains broken regardless.
- Rollback is a single `git revert`.

### Commit 1 (the only commit)

| Property | Value |
|----------|-------|
| **Scope** | `.github/workflows/nightly-build.yml` |
| **Type** | `fix` |
| **Message** | `fix(ci): free disk space before nightly multi-platform Docker builds` |
| **Files changed** | `.github/workflows/nightly-build.yml` |
| **Dependencies** | None |
| **Validation gate** | Manual `workflow_dispatch` of `nightly-build.yml` completes without disk-full error |

**Commit body:**

```
The ubuntu-latest runner (~14 GB free) is exhausted by pre-installed
toolchains (Android SDK, .NET, Haskell) before the multi-platform
build-push-action executes. The runner dies mid-build without sending
terminal step statuses, leaving jobs in a failed+in-progress limbo.

Add jlumbroso/free-disk-space@v1.3.1 as the first step in both
build-and-push-nightly and build-and-push-nightly-orthrus. Configured
to remove Android, .NET, Haskell, large-packages, and swap storage
(~10–15 GB recovered). docker-images is explicitly false to preserve
the Docker daemon state required by Buildx.
```

### Rollback

```bash
git revert <commit-sha>
```

No data loss, no migration, no downstream impact.

---

## 7. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `free-disk-space` action itself fails (network, apt lock) | Low | Medium | Step is not `continue-on-error`; if it fails the job fails fast before wasting build time. |
| Docker daemon loses needed images if `docker-images` is accidentally set `true` | Low | High | `docker-images: false` is explicit in the YAML; verified in AC-4. |
| Disk space still insufficient after reclamation | Very low | High | ~10–15 GB recovered is well above the ~4–6 GB needed for a two-platform Go+Alpine build. |
| SHA drift (action updated, SHA stale) | Low | Low | SHA is pinned; Dependabot will create a PR to update when a new release is published. |



