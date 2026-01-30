# Manual Test Plan: CI Docker Build Fix Verification

**Issue**: Docker image artifact save failing with "reference does not exist" error
**Fix Date**: 2026-01-12
**Test Target**: `.github/workflows/docker-build.yml` (Save Docker Image as Artifact step)
**Test Priority**: HIGH (blocks PR builds and supply chain verification)

---

## Test Objective

Verify that the CI Docker build fix resolves the "reference does not exist" error and enables successful PR builds with artifact generation and supply chain verification.

---

## Prerequisites

- [ ] Changes merged to a feature branch or development
- [ ] Ability to create test PRs against the target branch
- [ ] Access to GitHub Actions logs for the test PR
- [ ] Understanding of expected workflow behavior

---

## Test Scenarios

### Scenario 1: Standard PR Build (Happy Path)

**Objective**: Verify normal PR build succeeds with image artifact save

**Steps**:

1. Create a test PR with a minor change (e.g., update README.md)
2. Wait for `docker-build.yml` workflow to trigger
3. Monitor the workflow execution in GitHub Actions

**Expected Results**:

- [ ] ✅ `build-and-push` job completes successfully
- [ ] ✅ "Save Docker Image as Artifact" step completes without errors
- [ ] ✅ Step output shows: "🔍 Detected image tag: ghcr.io/wikid82/charon:pr-XXX"
- [ ] ✅ Step output shows: "✅ Artifact created: /tmp/charon-pr-image.tar"
- [ ] ✅ "Upload Image Artifact" step succeeds
- [ ] ✅ Artifact `pr-image-XXX` appears in workflow artifacts
- [ ] ✅ `verify-supply-chain-pr` job starts and uses the artifact
- [ ] ✅ Supply chain verification completes successfully

**Pass Criteria**: All checks pass, no "reference does not exist" errors

---

### Scenario 2: Metadata Tag Validation

**Objective**: Verify defensive validation catches missing or invalid tags

**Steps**:

1. Review the "Save Docker Image as Artifact" step logs
2. Check for validation output

**Expected Results**:

- [ ] ✅ Step logs show: "🔍 Detected image tag: ghcr.io/wikid82/charon:pr-XXX"
- [ ] ✅ No error messages about missing tags
- [ ] ✅ Image inspection succeeds (no "not found locally" errors)

**Pass Criteria**: Validation steps execute and pass cleanly

---

### Scenario 3: Supply Chain Verification Integration

**Objective**: Verify downstream job receives and processes the artifact correctly

**Steps**:

1. Wait for `verify-supply-chain-pr` job to start
2. Check "Download Image Artifact" step
3. Check "Load Docker Image" step
4. Check "Verify Loaded Image" step

**Expected Results**:

- [ ] ✅ Artifact downloads successfully
- [ ] ✅ Image loads without errors
- [ ] ✅ Verification step confirms image exists: "✅ Image verified: ghcr.io/wikid82/charon:pr-XXX"
- [ ] ✅ SBOM generation step uses correct image reference
- [ ] ✅ Vulnerability scanning completes
- [ ] ✅ PR comment appears with supply chain verification results

**Pass Criteria**: Full supply chain verification pipeline executes end-to-end

---

### Scenario 4: Error Handling (Edge Case)

**Objective**: Verify defensive validation catches actual errors (if possible to trigger)

**Note**: This scenario is difficult to test without artificially breaking the build. Monitor for this in production if a natural failure occurs.

**Expected Behavior** (if error occurs):

- [ ] Step fails fast with clear diagnostics
- [ ] Error message shows exact issue (missing tag, image not found, etc.)
- [ ] Available images are listed for debugging
- [ ] Workflow fails with actionable error message

**Pass Criteria**: If error occurs, diagnostics are clear and actionable

---

## Regression Testing

### Check Previous Failure Cases

**Steps**:

1. Review previous failed PR builds (before fix)
2. Note the exact error messages
3. Confirm those errors no longer occur

**Expected Results**:

- [ ] ✅ No "reference does not exist" errors
- [ ] ✅ No "image not found" errors during save
- [ ] ✅ No manual tag reconstruction mismatches

**Pass Criteria**: Previous failure patterns are eliminated

---

## Performance Validation

**Objective**: Ensure fix does not introduce performance degradation

**Metrics to Monitor**:

- [ ] Build time (build-and-push job duration)
- [ ] Artifact save time
- [ ] Artifact upload time
- [ ] Total PR workflow duration

**Expected Results**:

- Build time: ~10-15 minutes (no significant change)
- Artifact save: <30 seconds
- Artifact upload: <1 minute
- Total workflow: <20 minutes for PR builds

**Pass Criteria**: No significant performance regression (±10% acceptable variance)

---

## Rollback Plan

**If Tests Fail**:

1. **Immediate Action**:
   - Revert commit fixing the artifact save step
   - Notify team of rollback
   - Create new issue with failure details

2. **Investigation**:
   - Capture full workflow logs
   - Check docker images output from failing run
   - Verify metadata action output format
   - Check for platform-specific issues (amd64 vs arm64)

3. **Recovery**:
   - Develop alternative fix approach
   - Test in isolated branch
   - Reapply fix after validation

---

## Test Log Template

**Test Execution Date**: [YYYY-MM-DD]
**Test PR Number**: #XXX
**Workflow Run**: [Link to GitHub Actions run]
**Tester**: [Name]

### Scenario 1: Standard PR Build

- Status: [ ] PASS / [ ] FAIL
- Notes:

### Scenario 2: Metadata Tag Validation

- Status: [ ] PASS / [ ] FAIL
- Notes:

### Scenario 3: Supply Chain Verification Integration

- Status: [ ] PASS / [ ] FAIL
- Notes:

### Scenario 4: Error Handling

- Status: [ ] PASS / [ ] FAIL / [ ] N/A
- Notes:

### Regression Testing

- Status: [ ] PASS / [ ] FAIL
- Notes:

### Performance Validation

- Status: [ ] PASS / [ ] FAIL
- Build time: X minutes
- Artifact save: X seconds
- Total workflow: X minutes
- Notes:

---

## Sign-Off

**Test Result**: [ ] PASS / [ ] FAIL
**Tested By**: _____________________
**Date**: _____________________
**Approved By**: _____________________
**Date**: _____________________

---

## References

- Original issue: See `current_spec.md` for root cause analysis
- Workflow file: `.github/workflows/docker-build.yml`
- Related fix: Lines 135-167 (Save Docker Image as Artifact step)
- CHANGELOG entry: See "Fixed" section under "Unreleased"
