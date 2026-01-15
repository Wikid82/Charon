# Manual Testing Plan: Grype SBOM Remediation

**Issue Type**: Manual Testing
**Priority**: High
**Component**: CI/CD - Supply Chain Verification
**Created**: 2026-01-10
**Related PR**: #461 (DNS Challenge Support)

---

## Objective

Manually validate the Grype SBOM remediation implementation in real-world CI/CD scenarios to ensure:

- Workflow operates correctly in all expected conditions
- Error handling is robust and user-friendly
- No regressions in existing functionality

---

## Test Environment

- **Branch**: `feature/beta-release` (current)
- **Workflow File**: `.github/workflows/supply-chain-verify.yml`
- **Trigger Events**: `pull_request`, `push to main`, `workflow_dispatch`

---

## Test Scenarios

### Scenario 1: PR Without Docker Image (Skip Path)

**Objective**: Verify workflow gracefully skips when image doesn't exist (common in PR workflows before docker-build completes).

**Prerequisites**:

- Create a test PR with code changes
- Ensure docker-build workflow has NOT completed yet

**Steps**:

1. Create/update PR on feature branch
2. Navigate to Actions → Supply Chain Verification workflow
3. Wait for workflow to complete

**Expected Results**:

- ✅ Workflow completes successfully (green check)
- ✅ "Check Image Availability" step shows "Image not found" message
- ✅ "Report Skipped Scan" step shows clear skip reason
- ✅ PR comment appears with "⏭️ Status: Image not yet available" message
- ✅ PR comment explains this is normal for PR workflows
- ✅ No false failures or error messages

**Pass Criteria**:

- [ ] Workflow status: Success (not failed or warning)
- [ ] PR comment is clear and helpful
- [ ] GitHub Step Summary shows skip reason
- [ ] No confusing error messages in logs

---

### Scenario 2: Existing Docker Image (Success Path)

**Objective**: Verify full SBOM generation, validation, and vulnerability scanning when image exists.

**Prerequisites**:

- Use a branch where docker-build has completed (e.g., `main` or merged PR)
- Image exists in GHCR: `ghcr.io/wikid82/charon:latest` or `ghcr.io/wikid82/charon:pr-XXX`

**Steps**:

1. Trigger workflow manually via `workflow_dispatch` on main branch
2. OR merge a PR and wait for automatic workflow trigger
3. Monitor workflow execution

**Expected Results**:

- ✅ "Check Image Availability" step finds image
- ✅ "Verify SBOM Completeness" step generates CycloneDX SBOM
- ✅ Syft version is logged
- ✅ "Validate SBOM File" step passes all checks:
  - jq is available
  - File exists and non-empty
  - Valid JSON structure
  - CycloneDX format confirmed
  - Components found (count > 0)
- ✅ "Upload SBOM Artifact" step succeeds
- ✅ SBOM artifact available for download
- ✅ "Scan for Vulnerabilities" step:
  - Grype DB updates successfully
  - Scan completes without "format not recognized" error
  - Vulnerability counts reported
  - Results table displayed
- ✅ PR comment (if PR) shows vulnerability summary table
- ✅ No "sbom format not recognized" errors

**Pass Criteria**:

- [ ] Workflow status: Success
- [ ] SBOM artifact uploaded and downloadable
- [ ] Grype scan completes without format errors
- [ ] Vulnerability counts accurate (Critical/High/Medium/Low)
- [ ] PR comment shows detailed results (if applicable)
- [ ] No false positives

---

### Scenario 3: Invalid/Corrupted SBOM (Validation Path)

**Objective**: Verify SBOM validation catches malformed files before passing to Grype.

**Prerequisites**:

- Requires temporarily modifying workflow to introduce error (NOT for production testing)
- OR wait for natural occurrence (unlikely)

**Alternative Testing**:
This scenario is validated through code review and unit testing of validation logic. Manual testing in production environment is not recommended as it requires intentionally breaking the workflow.

**Code Review Validation** (Already Completed):

- ✅ jq availability check (lines 125-130)
- ✅ File existence check (lines 133-138)
- ✅ Non-empty check (lines 141-146)
- ✅ Valid JSON check (lines 149-156)
- ✅ CycloneDX format check (lines 159-173)

**Pass Criteria**:

- [ ] Code review confirms all validation checks present
- [ ] Error handling paths use `exit 1` for real errors
- [ ] Clear error messages at each validation point

---

### Scenario 4: Critical Vulnerabilities Detected

**Objective**: Verify workflow correctly identifies and reports critical vulnerabilities.

**Prerequisites**:

- Use an older image tag with known vulnerabilities (if available)
- OR wait for vulnerability to be discovered in current image

**Steps**:

1. Trigger workflow on image with vulnerabilities
2. Monitor vulnerability scan step
3. Check PR comment and workflow logs

**Expected Results**:

- ✅ Grype scan completes successfully
- ✅ Vulnerabilities categorized by severity
- ✅ Critical vulnerabilities trigger GitHub annotation/warning
- ✅ PR comment shows vulnerability table with non-zero counts
- ✅ PR comment includes "⚠️ Action Required" for critical vulns
- ✅ Link to full report is provided

**Pass Criteria**:

- [ ] Vulnerability counts are accurate
- [ ] Critical vulnerabilities highlighted
- [ ] Clear action guidance provided
- [ ] Links to detailed reports work

---

### Scenario 5: Workflow Performance

**Objective**: Verify workflow executes within acceptable time limits.

**Steps**:

1. Monitor workflow execution time across multiple runs
2. Check individual step durations

**Expected Results**:

- ✅ Total workflow time: < 10 minutes
- ✅ Image check: < 30 seconds
- ✅ SBOM generation: < 2 minutes
- ✅ SBOM validation: < 30 seconds
- ✅ Grype scan: < 5 minutes
- ✅ Artifact upload: < 1 minute

**Pass Criteria**:

- [ ] Average workflow time within limits
- [ ] No significant performance degradation vs. previous implementation
- [ ] No timeout failures

---

### Scenario 6: Multiple Parallel PRs

**Objective**: Verify workflow handles concurrent executions without conflicts.

**Prerequisites**:

- Create multiple PRs simultaneously
- Trigger workflows on multiple branches

**Steps**:

1. Create 3-5 PRs from different feature branches
2. Wait for workflows to run concurrently
3. Monitor all workflow executions

**Expected Results**:

- ✅ All workflows complete successfully
- ✅ No resource conflicts or race conditions
- ✅ Correct image checked for each PR (`pr-XXX` tags)
- ✅ Each PR gets its own comment
- ✅ Artifact names are unique (include tag)

**Pass Criteria**:

- [ ] All workflows succeed independently
- [ ] No cross-contamination of results
- [ ] Artifact names unique and correct

---

## Regression Testing

### Verify No Breaking Changes

**Test Areas**:

1. **Other Workflows**: Ensure docker-build.yml, codeql-analysis.yml, etc. still work
2. **Existing Releases**: Verify workflow runs successfully on existing release tags
3. **Backward Compatibility**: Old PRs can be re-run without issues

**Pass Criteria**:

- [ ] No regressions in other workflows
- [ ] Existing functionality preserved
- [ ] No unexpected failures

---

## Bug Hunting Focus Areas

Based on the implementation, pay special attention to:

1. **Conditional Logic**:
   - Verify `if: steps.image-check.outputs.exists == 'true'` works correctly
   - Check `if: steps.validate-sbom.outputs.valid == 'true'` gates scan properly

2. **Error Messages**:
   - Ensure error messages are clear and actionable
   - Verify debug output is helpful for troubleshooting

3. **Authentication**:
   - GHCR authentication succeeds for private repos
   - Token permissions are sufficient

4. **Artifact Handling**:
   - SBOM artifacts upload correctly
   - Artifact names are unique and descriptive
   - Retention period is appropriate (30 days)

5. **PR Comments**:
   - Comments appear on all PRs
   - Markdown formatting is correct
   - Links work and point to correct locations

6. **Edge Cases**:
   - Very large images (slow SBOM generation)
   - Images with many vulnerabilities (large scan output)
   - Network failures during Grype DB update
   - Rate limiting from GHCR

---

## Issue Reporting Template

If you find a bug during manual testing, create an issue with:

```markdown
**Title**: [Grype SBOM] Brief description of issue

**Scenario**: Which test scenario revealed the issue

**Expected Behavior**: What should happen

**Actual Behavior**: What actually happened

**Evidence**:
- Workflow run URL
- Relevant log excerpts
- Screenshots if applicable

**Severity**: Critical / High / Medium / Low

**Impact**: Who/what is affected

**Workaround**: If known
```

---

## Sign-Off Checklist

After completing manual testing, verify:

- [ ] Scenario 1 (Skip Path) tested and passed
- [ ] Scenario 2 (Success Path) tested and passed
- [ ] Scenario 3 (Validation) verified via code review
- [ ] Scenario 4 (Vulnerabilities) tested and passed
- [ ] Scenario 5 (Performance) verified within limits
- [ ] Scenario 6 (Parallel PRs) tested and passed
- [ ] Regression testing completed
- [ ] Bug hunting completed
- [ ] All critical issues resolved
- [ ] Documentation reviewed for accuracy

**Tester Signature**: _________________
**Date**: _________________
**Status**: ☐ PASS ☐ PASS WITH MINOR ISSUES ☐ FAIL

---

## Notes

- This manual testing plan complements automated CI/CD checks
- Focus on user experience and real-world scenarios
- Document any unexpected behavior, even if not blocking
- Update this plan based on findings for future use

---

**Status**: Ready for Manual Testing
**Last Updated**: 2026-01-10
