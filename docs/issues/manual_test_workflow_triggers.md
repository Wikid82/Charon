---
title: Manual Test Plan - Workflow Trigger Verification
status: Open
priority: Normal
assignee: DevOps
labels: testing, workflows, ci/cd
---

# Test Objectives
Verify that all CI/CD workflows trigger correctly on feature branches and provide immediate feedback without waiting for the `docker-build` workflow (except where intended for release verification).

# Scope
- `dry-run-history-rewrite.yml` (Modified)
- `integration-tests.yml` (consolidated: builds the Charon image once, then fans out to the `cerberus` / `waf` / `rate-limit` / `crowdsec` suite jobs)
- `e2e-tests-split.yml`

# Test Steps

## 1. Dry Run Workflow (Modified)
- [ ] Create a new branch `feature/test-workflow-triggers`.
- [ ] Make a dummy change to a file (e.g., `README.md`).
- [ ] Push the branch.
- [ ] Go to Actions tab.
- [ ] Verify `Dry Run History Rewrite` workflow starts immediately.

## 2. Integration Tests (Dual Mode Verification)
- [ ] Using the same branch `feature/test-workflow-triggers`.
- [ ] Verify the `Integration Tests` workflow starts immediately (building locally).
- [ ] Confirm its `Build Charon image` job runs the "Build Docker image (Local)" step exactly once.
- [ ] Confirm the `Cerberus Security Stack Integration`, `Coraza WAF Integration`, `Rate Limiting Integration` and `CrowdSec Bouncer Integration` jobs each `needs: build`, download the `charon-integration-image` artifact and `docker load` it instead of rebuilding.

## 3. Supply Chain (Split Verification)
- [ ] Verify `Supply Chain Security (PR)` starts on the feature branch push.
- [ ] Verify `Supply Chain Verify (Release)` does **NOT** start (it should wait for `docker-build` on main/release).

## 4. E2E Tests
- [ ] Verify `E2E Tests` workflow starts immediately and builds its own image.

# Success Criteria
- All "Validation" workflows trigger on `push` to `feature/*`.
- Integration tests build locally instead of failing/waiting for registry.
- No "Resource not accessible" errors for secrets on the feature branch.
