# CI/CD Workflow Modularization Plan

**Date**: 2026-01-15
**Status**: Planning
**Category**: CI/CD Infrastructure Refactoring
**Repository**: Wikid82/Charon

---

## Overview

Refactor `docker-build.yml` to separate ALL post-build testing into dedicated workflows that trigger via `workflow_run`. This creates a modular CI architecture where:

- `docker-build.yml` focuses ONLY on building and uploading artifacts
- Each test type runs independently with its own pass/fail status
- Cleaner logs for easier troubleshooting
- Parallel execution of independent tests

---

## Current Job Inventory

| Job | Line | Type | Event | Action |
|-----|------|------|-------|--------|
| `build-and-push` | 36 | **KEEP** | all | Core build job |
| `test-image` | 406 | **KEEP** | push only | Integration test (not PRs) |
| `trivy-pr-app-only` | 493 | **EXTRACT** | PR only | Trivy scan |
| `verify-supply-chain-pr` | 524 | **EXTRACT** | PR only | SBOM + vuln scan |
| `verify-supply-chain-pr-skipped` | 769 | **REMOVE** | PR only | Notification |
| `e2e-tests-pr` | 809 | **EXTRACT** | PR only | Playwright E2E |

---

## New Workflow Architecture

```
docker-build.yml (build-and-push + test-image only)
                     │
                     │ workflow_run (completed)
                     ▼
    ┌────────────────┼────────────────┐
    │                │                │
    ▼                ▼                ▼
playwright.yml  security-pr.yml  supply-chain-pr.yml
(E2E Tests)     (Trivy Scan)     (SBOM + Grype)
```

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `.github/workflows/playwright.yml` | CREATE |
| `.github/workflows/security-pr.yml` | CREATE |
| `.github/workflows/supply-chain-pr.yml` | CREATE |
| `.github/workflows/docker-build.yml` | MODIFY - Remove lines 493-909 |
| `.github/workflows/playwright.yml.disabled` | DELETE |

---

## Benefits

- **Independent failures** - Playwright failure doesn't block security scan
- **Cleaner logs** - Each workflow has focused output
- **Parallel execution** - All post-build tests run simultaneously
- **Easier debugging** - Know exactly which test type failed

---

**Status**: Ready for Implementation
