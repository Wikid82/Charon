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
- `cerberus-integration.yml`
- `crowdsec-integration.yml`
- `waf-integration.yml`
- `rate-limit-integration.yml`
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
- [ ] Verify the following workflows start immediately (building locally):
  - [ ] `Cerberus Integration`
  - [ ] `CrowdSec Integration`
  - [ ] `Coraza WAF Integration`
  - [ ] `Rate Limiting Integration`
- [ ] Inspect the logs of one of them.
- [ ] Confirm it executes the "Build Docker image (Local)" step and *skips* the "Pull Docker image from registry" step.

## 3. Supply Chain (Split Verification)
- [ ] Verify `Supply Chain Security (PR)` starts on the feature branch push.
- [ ] Verify `Supply Chain Verify (Release)` does **NOT** start (it should wait for `docker-build` on main/release).

## 4. E2E Tests
- [ ] Verify `E2E Tests` workflow starts immediately and builds its own image.

# Success Criteria
- All "Validation" workflows trigger on `push` to `feature/*`.
- Integration tests build locally instead of failing/waiting for registry.
- No "Resource not accessible" errors for secrets on the feature branch.
