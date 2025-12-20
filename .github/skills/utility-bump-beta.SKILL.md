---
name: "utility-bump-beta"
version: "1.0.0"
description: "Increments beta version number across all project files for pre-release versioning"
author: "Charon Project"
license: "MIT"
tags:
  - "utility"
  - "versioning"
  - "release"
  - "automation"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "git"
    version: ">=2.0"
    optional: false
  - name: "sed"
    version: ">=4.0"
    optional: false
environment_variables: []
parameters: []
outputs:
  - name: "new_version"
    type: "string"
    description: "The new beta version number"
    path: ".version"
metadata:
  category: "utility"
  subcategory: "versioning"
  execution_time: "short"
  risk_level: "medium"
  ci_cd_safe: false
  requires_network: false
  idempotent: false
---

# Utility: Bump Beta Version

## Overview

Automates beta version bumping across all project files. This skill intelligently increments version numbers following semantic versioning conventions for beta releases, updating multiple files in sync to maintain consistency.

## Prerequisites

- Git repository initialized
- Write access to project files
- Clean working directory (recommended)

## Usage

### Basic Usage

```bash
.github/skills/utility-bump-beta-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh utility-bump-beta
```

### Via VS Code Task

Use the task: **Utility: Bump Beta Version**

## Parameters

This skill accepts no parameters. Version bumping logic is automatic based on current version format.

## Environment Variables

This skill requires no environment variables.

## Outputs

- **Success Exit Code**: 0
- **Error Exit Codes**: Non-zero on failure
- **Modified Files**:
  - `.version`
  - `backend/internal/version/version.go`
  - `frontend/package.json`
  - `backend/package.json` (if exists)
- **Git Tag**: `v{NEW_VERSION}` (if user confirms)

### Output Example

```
Starting Beta Version Bump...
Current Version: 0.3.0-beta.2
New Version: 0.3.0-beta.3
Updated .version
Updated backend/internal/version/version.go
Updated frontend/package.json
Updated backend/package.json
Do you want to commit and tag this version? (y/n) y
Committed and tagged v0.3.0-beta.3
Remember to push: git push origin feature/beta-release --tags
```

## Version Bumping Logic

### Current Version is Beta (x.y.z-beta.N)

Increments the beta number:
- `0.3.0-beta.2` → `0.3.0-beta.3`
- `1.0.0-beta.5` → `1.0.0-beta.6`

### Current Version is Plain Semver (x.y.z)

Bumps minor version and starts beta.1:
- `0.3.0` → `0.4.0-beta.1`
- `1.2.0` → `1.3.0-beta.1`

### Current Version is Alpha or Unrecognized

Defaults to safe fallback:
- `0.3.0-alpha` → `0.3.0-beta.1`
- `invalid-version` → `0.3.0-beta.1`

## Files Updated

1. **`.version`**: Project root version file
2. **`backend/internal/version/version.go`**: Go version constant
3. **`frontend/package.json`**: Frontend package version
4. **`backend/package.json`**: Backend package version (if exists)

All files are updated with consistent version strings using `sed` regex replacement.

## Examples

### Example 1: Bump Beta Before Release

```bash
# Bump version for next beta iteration
.github/skills/utility-bump-beta-scripts/run.sh

# Confirm when prompted to commit and tag
# Then push to remote
git push origin feature/beta-release --tags
```

### Example 2: Bump Without Committing

```bash
# Make version changes but skip git operations
.github/skills/utility-bump-beta-scripts/run.sh
# Answer 'n' when prompted about committing
```

## Interactive Confirmation

After updating files, the script prompts:

```
Do you want to commit and tag this version? (y/n)
```

- **Yes (y)**: Creates git commit and tag automatically
- **No (n)**: Leaves changes staged for manual review

## Error Handling

- Validates `.version` file exists and is readable
- Uses safe defaults for unrecognized version formats
- Does not modify VERSION.md guide content (manual update recommended)
- Skips `backend/package.json` if file doesn't exist

## Post-Execution Steps

After running this skill:

1. **Review Changes**: `git diff`
2. **Run Tests**: Ensure version change doesn't break builds
3. **Push Tags**: `git push origin <branch> --tags`
4. **Update CHANGELOG.md**: Manually document changes for this version
5. **Verify CI/CD**: Check that automated builds use new version

## Related Skills

- [utility-version-check](./utility-version-check.SKILL.md) - Validate version matches tags
- [build-check-go](../build-check-go.SKILL.md) - Verify build after version bump

## Notes

- **Not Idempotent**: Running multiple times increments version each time
- **Risk Level: Medium**: Modifies multiple critical files
- **Git State**: Recommended to have clean working directory before running
- **Manual Review**: Always review version changes before pushing
- **VERSION.md**: Update manually as it contains documentation, not just version

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `scripts/bump_beta.sh`
