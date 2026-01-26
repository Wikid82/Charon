# Supply Chain Security PR Comments Update

## Overview

Modified the supply chain security workflow to update or create PR comments that always reflect the current security state, replacing stale scan results with fresh data.

**Date**: 2026-01-11
**Workflow**: `.github/workflows/supply-chain-verify.yml`
**Status**: ✅ Complete

---

## Problem Statement

Previously, the workflow posted a new comment on each scan run, which meant:

- Old comments with vulnerabilities remained visible even after fixes
- Multiple comments accumulated, causing confusion
- No way to track when the scan was last run
- Difficult to see the current security state at a glance

## Solution

Replaced the `actions/github-script` comment creation with the `peter-evans/create-or-update-comment` action, which:

1. **Finds existing comments** from the same workflow using a unique HTML comment identifier
2. **Updates in place** instead of creating new comments
3. **Includes timestamps** showing when the scan last ran
4. **Provides clear status indicators** with emojis and formatted tables

---

## Changes Made

### 1. Split PR Comment Logic into Multiple Steps

**Step 1: Determine PR Number**

- Extracts PR number from context (handles both `pull_request` and `workflow_run` events)
- Returns empty string if no PR found
- Uses `actions/github-script` with `result-encoding: string` for clean output

**Step 2: Build PR Comment Body**

- Generates timestamp with `date -u +"%Y-%m-%d %H:%M:%S UTC"`
- Calculates total vulnerabilities
- Creates formatted Markdown comment with:
  - Status header with appropriate emoji
  - Timestamp and workflow run link
  - Vulnerability table with severity counts
  - Color-coded emojis (🔴 Critical, 🟠 High, 🟡 Medium, 🔵 Low)
  - Links to detailed reports
  - Hidden HTML comment for identification: `<!-- supply-chain-security-comment -->`
- Saves to `/tmp/comment-body.txt` for next step

**Step 3: Update or Create PR Comment**

- Uses `peter-evans/create-or-update-comment@v4.0.0`
- Searches for existing comments containing `<!-- supply-chain-security-comment -->`
- Updates existing comment or creates new one
- Uses `edit-mode: replace` to fully replace old content

### 2. Comment Formatting Improvements

#### Status Indicators

**Waiting for Image**

```markdown
### ⏳ Status: Waiting for Image

The Docker image has not been built yet...
```

**No Vulnerabilities**

```markdown
### ✅ Status: No Vulnerabilities Detected

🎉 Great news! No security vulnerabilities were found in this image.
```

**Vulnerabilities Found**

```markdown
### 🚨 Status: Critical Vulnerabilities Detected

⚠️ **Action Required**: X critical vulnerabilities require immediate attention!
```

#### Vulnerability Table

| Severity | Count |
|----------|-------|
| 🔴 Critical | 2 |
| 🟠 High | 5 |
| 🟡 Medium | 3 |
| 🔵 Low | 1 |
| **Total** | **11** |

### 3. Technical Implementation Details

**Unique Identifier**

- Hidden HTML comment: `<!-- supply-chain-security-comment -->`
- Allows `create-or-update-comment` to find previous comments from this workflow
- Invisible to users but searchable by the action

**Multi-line Handling**

- Comment body saved to file instead of environment variable
- Prevents issues with special characters and newlines
- More reliable than shell heredocs or environment variables

**Conditional Execution**

- All three steps check for valid PR number
- Steps skip gracefully if not in PR context
- No errors on scheduled runs or release events

---

## Benefits

### 1. **Always Current**

- Comment reflects the latest scan results
- No confusion from multiple stale comments
- Clear "Last Updated" timestamp

### 2. **Easy to Understand**

- Color-coded severity levels with emojis
- Clear status headers (✅, ⚠️, 🚨)
- Formatted tables for quick scanning
- Links to detailed workflow logs

### 3. **Actionable**

- Immediate visibility of critical issues
- Direct links to full reports
- Clear indication of when action is required

### 4. **Reliable**

- Handles both `pull_request` and `workflow_run` triggers
- Graceful fallback if PR context not available
- No duplicate comments

---

## Testing Recommendations

### Manual Testing

1. **Create a test PR**

   ```bash
   git checkout -b test/supply-chain-comments
   git commit --allow-empty -m "test: supply chain comment updates"
   git push origin test/supply-chain-comments
   ```

2. **Trigger the workflow**
   - Wait for docker-build to complete
   - Verify supply-chain-verify runs and comments

3. **Re-trigger the workflow**
   - Manually re-run the workflow from Actions UI
   - Verify comment is updated, not duplicated

4. **Fix vulnerabilities and re-scan**
   - Update base image or dependencies
   - Rebuild and re-scan
   - Verify comment shows new status

### Automated Testing

Monitor the workflow on:

- Next scheduled run (Monday 00:00 UTC)
- Next PR that triggers docker-build
- Next release

---

## Action Versions Used

| Action | Version | SHA | Notes |
|--------|---------|-----|-------|
| `actions/github-script` | v7.0.1 | `60a0d83039c74a4aee543508d2ffcb1c3799cdea` | For PR number extraction |
| `peter-evans/create-or-update-comment` | v4.0.0 | `71345be0265236311c031f5c7866368bd1eff043` | For comment updates |

---

## Example Comment Output

### When No Vulnerabilities Found

```markdown
## 🔒 Supply Chain Security Scan

**Last Updated**: 2026-01-11 15:30:45 UTC
**Workflow Run**: [#123](https://github.com/owner/repo/actions/runs/123456)

---

### ✅ Status: No Vulnerabilities Detected

🎉 Great news! No security vulnerabilities were found in this image.

| Severity | Count |
|----------|-------|
| 🔴 Critical | 0 |
| 🟠 High | 0 |
| 🟡 Medium | 0 |
| 🔵 Low | 0 |

---

<!-- supply-chain-security-comment -->
```

### When Vulnerabilities Found

```markdown
## 🔒 Supply Chain Security Scan

**Last Updated**: 2026-01-11 15:30:45 UTC
**Workflow Run**: [#123](https://github.com/owner/repo/actions/runs/123456)

---

### 🚨 Status: Critical Vulnerabilities Detected

⚠️ **Action Required**: 2 critical vulnerabilities require immediate attention!

| Severity | Count |
|----------|-------|
| 🔴 Critical | 2 |
| 🟠 High | 5 |
| 🟡 Medium | 3 |
| 🔵 Low | 1 |
| **Total** | **11** |

📋 [View detailed vulnerability report](https://github.com/owner/repo/actions/runs/123456)

---

<!-- supply-chain-security-comment -->
```

---

## Troubleshooting

### Comment Not Updating

**Symptom**: New comments created instead of updating existing one

**Cause**: The hidden HTML identifier might not match

**Solution**: Check for the exact string `<!-- supply-chain-security-comment -->` in existing comments

### PR Number Not Found

**Symptom**: Steps skip with "No PR number found"

**Cause**: Workflow triggered outside PR context (scheduled, release, manual)

**Solution**: This is expected behavior; comment steps only run for PRs

### Timestamp Format Issues

**Symptom**: Timestamp shows incorrect time or format

**Cause**: System timezone or date command issues

**Solution**: Using `date -u` ensures consistent UTC timestamps

---

## Future Enhancements

1. **Trend Analysis**: Track vulnerability counts over time
2. **Comparison**: Show delta from previous scan
3. **Priority Recommendations**: Link to remediation guides
4. **Dismiss Button**: Allow developers to acknowledge and hide resolved issues
5. **Integration**: Link to JIRA/GitHub issues for tracking

---

## Related Files

- `.github/workflows/supply-chain-verify.yml` - Main workflow file
- `.github/workflows/docker-build.yml` - Triggers this workflow

---

## References

- [peter-evans/create-or-update-comment](https://github.com/peter-evans/create-or-update-comment)
- [GitHub Actions: workflow_run event](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#workflow_run)
- [Grype vulnerability scanner](https://github.com/anchore/grype)
