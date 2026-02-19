---
title: Manual Checklist - Local Patch Report DoD Ordering
status: Open
priority: High
assignee: QA
labels: testing, coverage, dod
---

# Goal
Validate that local patch-report workflow is executed in Definition of Done (DoD) order and produces required artifacts for handoff.

# Preconditions
- Work from repository root: `/projects/Charon`
- Branch has local changes to evaluate
- Docker E2E environment is healthy

# Manual Checklist

## 1) E2E First (Mandatory)
- [ ] Run: `cd /projects/Charon && npx playwright test --project=firefox`
- [ ] Confirm run completes without blocking failures
- [ ] Record run timestamp for ordering evidence

## 2) Local Patch Report Preflight (Before Unit Coverage)
- [ ] Run: `cd /projects/Charon && bash scripts/local-patch-report.sh`
- [ ] Confirm artifacts exist:
  - [ ] `test-results/local-patch-report.md`
  - [ ] `test-results/local-patch-report.json`
- [ ] Confirm JSON includes:
  - [ ] `baseline = origin/development...HEAD` (or `development...HEAD` when remote ref is unavailable)
  - [ ] `mode = warn`
  - [ ] `overall`, `backend`, `frontend` coverage blocks
  - [ ] `files_needing_coverage` list

## 3) Backend Coverage Run
- [ ] Run: `cd /projects/Charon/backend && go test ./... -coverprofile=coverage.txt`
- [ ] Confirm `backend/coverage.txt` exists and is current
- [ ] Confirm run exit code is 0

## 4) Frontend Coverage Run
- [ ] Run: `cd /projects/Charon/frontend && npm run test:coverage`
- [ ] Confirm `frontend/coverage/lcov.info` exists and is current
- [ ] Confirm run exit code is 0

## 5) Refresh Local Patch Report After Coverage Updates
- [ ] Run again: `cd /projects/Charon && bash scripts/local-patch-report.sh`
- [ ] Confirm report reflects latest coverage inputs and updated file gaps

## 6) DoD Ordering Verification (Practical)
- [ ] Verify command history/logs show this order:
  1. E2E
  2. Local patch report preflight
  3. Backend/frontend coverage runs
  4. Local patch report refresh
- [ ] Verify no skipped step in the sequence

## 7) Handoff Artifact Verification
- [ ] Verify required handoff artifacts are present:
  - [ ] `test-results/local-patch-report.md`
  - [ ] `test-results/local-patch-report.json`
  - [ ] `backend/coverage.txt`
  - [ ] `frontend/coverage/lcov.info`
- [ ] Verify latest QA report includes current patch-coverage summary section

# Pass Criteria
- All checklist items complete in order.
- Local patch report artifacts are generated and current.
- Any below-threshold overall patch coverage is explicitly documented as warn-mode during rollout.
