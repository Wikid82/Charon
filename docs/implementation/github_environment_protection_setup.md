# GitHub Environment Protection Setup

**Status**: Manual Configuration Required
**Priority**: HIGH
**Estimated Time**: 30 minutes

## Overview

This document provides instructions for setting up GitHub environment protection rules for the `release` job in the GoReleaser workflow. This adds an additional security layer to prevent unauthorized or accidental releases.

## Why This Is Important

Currently, the `release-goreleaser.yml` workflow has broad permissions (`contents: write`, `packages: write`) without environment protection. This means:

- Anyone with write access can trigger a release
- No approval gate exists before publishing to production
- No audit trail for release decisions

Environment protection adds:
- ✅ Required reviewers before release
- ✅ Restricted to specific branches/tags
- ✅ Audit log of approvals
- ✅ Prevention of accidental releases

## Setup Instructions

### Step 1: Access Repository Settings

1. Navigate to: https://github.com/Wikid82/Charon/settings/environments
2. Click **"New environment"**

### Step 2: Create "release" Environment

1. **Environment name**: `release`
2. Click **"Configure environment"**

### Step 3: Configure Protection Rules

#### Required Reviewers

1. Under **"Environment protection rules"**, enable **"Required reviewers"**
2. Add at least 1-2 trusted maintainers who must approve releases
3. Recommended reviewers:
   - Repository owner (@Wikid82)
   - Senior maintainers with release authority

#### Deployment Branches and Tags

1. Under **"Deployment branches and tags"**, select **"Protected branches and tags only"**
2. This ensures releases can only be triggered from tags matching `v*` pattern
3. Click **"Add deployment branch or tag rule"**
4. Pattern: `v*` (matches v1.0.0, v2.1.3-beta, etc.)

#### Wait Timer (Optional)

1. **"Wait timer"**: Consider adding a 5-minute wait timer for additional safety
2. This provides a brief window to cancel accidental releases

### Step 4: Update Workflow File

The workflow file already references the environment in the correct location. No code changes needed:

```yaml
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    environment:
      name: release
      url: https://github.com/${{ github.repository }}/releases
    permissions:
      contents: write
      packages: write
```

### Step 5: Test the Setup

1. Create a test tag: `git tag v0.0.1-test && git push origin v0.0.1-test`
2. Verify the workflow run pauses for approval
3. Check that the approval request appears in GitHub UI
4. Approve the deployment to complete the test
5. Delete the test tag: `git tag -d v0.0.1-test && git push origin :refs/tags/v0.0.1-test`

## Verification Checklist

After setup, verify:

- [ ] Environment "release" exists in repository settings
- [ ] Required reviewers are configured (at least 1)
- [ ] Deployment is restricted to `v*` tags
- [ ] Test release workflow shows approval gate
- [ ] Approval notifications are sent to reviewers
- [ ] Audit log shows approval history

## Troubleshooting

### Workflow Fails with "Environment not found"

**Cause**: Environment name mismatch between workflow file and GitHub settings
**Fix**: Ensure environment name is exactly `release` (case-sensitive)

### No Approval Request Shown

**Cause**: User might be self-approving or environment protection not saved
**Fix**:
1. Verify protection rules are enabled
2. Ensure reviewer is not the same as the person who triggered the workflow
3. Check GitHub notifications settings

### Can't Add Reviewers

**Cause**: Insufficient repository permissions
**Fix**: You must be a repository admin to configure environments

## Additional Security Recommendations

Consider also implementing:

1. **Branch Protection**: Require PR reviews before merging to `main`
2. **CODEOWNERS**: Define release approval owners in `.github/CODEOWNERS`
3. **Signed Commits**: Require GPG-signed commits for release tags
4. **2FA**: Enforce 2FA for all users with write access

## Related Documentation

- [GitHub Environments Documentation](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment)
- [Release Workflow](/.github/workflows/release-goreleaser.yml)
- [CI/CD Audit Report](/docs/plans/current_spec.md)

## Status

- [x] Documentation created
- [ ] Environment created in GitHub UI
- [ ] Required reviewers added
- [ ] Deployment branch rules configured
- [ ] Test release approval flow validated

**Next Action**: Repository admin must complete Steps 1-5 in GitHub UI.
