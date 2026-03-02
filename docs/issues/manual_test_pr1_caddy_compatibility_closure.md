## Manual Test Tracking Plan — PR-1 Caddy Compatibility Closure

- Date: 2026-02-23
- Scope: PR-1 only
- Goal: Track potential bugs in the completed PR-1 slice and confirm safe promotion.

## In Scope Features

1. Compatibility matrix execution and pass/fail outcomes
2. Release guard behavior (promotion gate)
3. Candidate build path behavior (`CADDY_USE_CANDIDATE=1`)
4. Non-drift defaults (`CADDY_USE_CANDIDATE=0` remains default)

## Out of Scope

- PR-2 and later slices
- Unrelated frontend feature behavior
- Historical QA items not tied to PR-1

## Environment Checklist

- [ ] Local repository is up to date with PR-1 changes
- [ ] Docker build completes successfully
- [ ] Test output directory is clean or isolated for this run

## Test Cases

### TC-001 — Compatibility Matrix Completes

- Area: Compatibility matrix
- Risk: False PASS due to partial artifacts or mixed output paths
- Steps:
  1. Run the matrix script with an isolated output directory.
  2. Verify all expected rows are present for scenarios A/B/C and amd64/arm64.
  3. Confirm each row has explicit PASS/FAIL values for required checks.
- Expected:
  - Matrix completes without missing rows.
  - Row statuses are deterministic and readable.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-002 — Promotion Gate Enforces Scenario A Only

- Area: Release guard
- Risk: Incorrect gate logic blocks or allows promotion unexpectedly
- Steps:
  1. Review matrix results for scenario A on amd64 and arm64.
  2. Confirm promotion decision uses scenario A on both architectures.
  3. Confirm scenario B/C are evidence-only and do not flip the promotion verdict.
- Expected:
  - Promotion gate follows PR-1 rule exactly.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-003 — Candidate Build Path Is Opt-In

- Area: Candidate build path
- Risk: Candidate path becomes active without explicit opt-in
- Steps:
  1. Build with default arguments.
  2. Confirm runtime behavior is standard (non-candidate path).
  3. Build again with candidate opt-in enabled.
  4. Confirm candidate path is only active in the opt-in build.
- Expected:
  - Candidate behavior appears only when explicitly enabled.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-004 — Default Runtime Behavior Does Not Drift

- Area: Non-drift defaults
- Risk: Silent default drift after PR-1 merge
- Steps:
  1. Verify Docker defaults used by standard build.
  2. Run a standard deployment path.
  3. Confirm behavior matches pre-PR-1 default expectations.
- Expected:
  - Default runtime remains non-candidate.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Defect Log

Use this section for any issue found during manual testing.

| ID | Test Case | Severity | Summary | Reproducible | Status |
| --- | --- | --- | --- | --- | --- |
| | | | | | |

## Exit Criteria

- [ ] All four PR-1 test cases executed
- [ ] No unresolved critical defects
- [ ] Promotion decision is traceable to matrix evidence
- [ ] Any failures documented with clear next action
