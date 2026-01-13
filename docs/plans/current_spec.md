# Nightly Branch Automation & Package Creation Plan

This document details the implementation plan for adding a new `nightly` branch between `development` and `main`, with automated merging and package creation.

**Date Created:** 2026-01-13
**Status:** Planning Phase
**Priority:** High

---

## Quick Reference

**See full detailed specification in:** [Nightly Branch Implementation Specification](./nightly_branch_implementation.md)

This file contains only the executive summary. The complete 2800+ line specification includes:

- Current workflow analysis
- Branch hierarchy design
- 7-phase implementation plan
- Complete workflow files
- Testing strategies
- Rollback procedures
- Troubleshooting guides

---

## Executive Summary

**Objective:** Add a `nightly` branch between `development` and `main` to create a stabilization layer with automated builds.

**Key Changes Required:**

1. Update `.github/workflows/propagate-changes.yml` (fix line 149, enable line 151-152)
2. Create `.github/workflows/nightly-build.yml` (new workflow for nightly packages)
3. Update `.github/workflows/docker-build.yml` (add nightly branch support)
4. Update `.github/workflows/supply-chain-verify.yml` (add nightly tag handling)
5. Configure branch protection for nightly branch
6. Update documentation (README.md, VERSION.md, CONTRIBUTING.md)

**Branch Flow:**

```
feature/* → development → nightly → main (tagged releases)
```

**Automation:**

- `development` → `nightly`: Auto-merge via workflow
- `nightly` → `main`: Manual PR with full review
- `nightly`: Daily builds + packages at 02:00 UTC

**Package Artifacts:**

- Docker images: `nightly`, `nightly-{date}`, `nightly-{sha}`
- Cross-compiled binaries (Linux, Windows, macOS)
- Linux packages (deb, rpm)
- SBOM and vulnerability reports

---

## Implementation Phases

### Phase 1: Update Propagate Workflow ⚡ URGENT

**File:** `.github/workflows/propagate-changes.yml`

- Fix line 149: Remove third parameter from `createPR` call
- Enable line 151-152: Uncomment `development` → `nightly` propagation

### Phase 2: Create Nightly Build Workflow

**File:** `.github/workflows/nightly-build.yml` (NEW)

- Triggers: Push to nightly, scheduled daily at 02:00 UTC
- Jobs: build-and-push, test-image, build-release, verify-supply-chain

### Phase 3: Update Docker Build

**File:** `.github/workflows/docker-build.yml`

- Add `nightly` to trigger branches
- Add `nightly` tag to metadata action
- Update test-image tag determination

### Phase 4: Update Supply Chain Verification

**File:** `.github/workflows/supply-chain-verify.yml`

- Add `nightly` branch handling in tag determination

### Phase 5: Configuration Files

- Review `.gitignore`, `.dockerignore`, `Dockerfile` (no changes needed)
- Optionally create `codecov.yml`
- Update `.github/propagate-config.yml`

### Phase 6: Branch Protection

- Create nightly branch from development
- Configure protection rules (allow force pushes, require status checks)

### Phase 7: Documentation

- Update `README.md` with nightly info
- Update `VERSION.md` with nightly section
- Update `CONTRIBUTING.md` with workflow

---

## Files to Modify

| File | Action | Priority |
|------|--------|----------|
| `.github/workflows/propagate-changes.yml` | Edit (2 lines) | P0 |
| `.github/workflows/nightly-build.yml` | Create (new) | P1 |
| `.github/workflows/docker-build.yml` | Edit (3 locations) | P1 |
| `.github/workflows/supply-chain-verify.yml` | Edit (1 location) | P2 |
| `.github/propagate-config.yml` | Edit (optional) | P3 |
| `README.md` | Edit | P3 |
| `VERSION.md` | Edit | P3 |
| `CONTRIBUTING.md` | Edit | P3 |

---

## Success Criteria

1. ✅ Development → nightly auto-merge completes in <5 minutes
2. ✅ Nightly Docker builds complete in <25 minutes
3. ✅ Build success rate >95% over 30 days
4. ✅ Zero critical vulnerabilities in nightly builds
5. ✅ SBOM generation success rate 100%

---

## Next Steps

1. Read the full specification in `./nightly_branch_implementation.md`
2. Review current workflows to understand integration points
3. Create implementation branch: `feature/nightly-branch-automation`
4. Implement Phase 1 (propagate workflow fix)
5. Test locally with workflow triggers
6. Deploy remaining phases incrementally

---

## Timeline Estimate

| Phase | Effort | Duration |
|-------|--------|----------|
| Phase 1 | 30 min | Day 1 |
| Phase 2 | 2 hours | Day 1-2 |
| Phase 3 | 30 min | Day 2 |
| Phase 4 | 30 min | Day 2 |
| Phase 5 | 1 hour | Day 2 |
| Phase 6 | 30 min | Day 3 |
| Phase 7 | 1 hour | Day 3 |
| Testing | 4 hours | Day 3-4 |
| **Total** | **~10 hours** | **3-4 days** |

---

**For complete details, workflows, scripts, and troubleshooting guides, see:**
**[nightly_branch_implementation.md](./nightly_branch_implementation.md)**
