---
name: "utility-version-check"
version: "1.0.0"
description: "Validates that VERSION.md/version file matches the latest git tag for release consistency"
author: "Charon Project"
license: "MIT"
tags:
  - "utility"
  - "versioning"
  - "validation"
  - "git"
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
environment_variables: []
parameters: []
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 if version matches, 1 if mismatch or error"
metadata:
  category: "utility"
  subcategory: "versioning"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Utility: Version Check

## Overview

Validates that the version specified in `.version` file matches the latest git tag. This ensures version consistency across the codebase and prevents version drift during releases. The check is used in CI/CD to enforce version tagging discipline.

## Prerequisites

- Git repository with tags
- `.version` file in repository root (optional)

## Usage

### Basic Usage

```bash
.github/skills/utility-version-check-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh utility-version-check
```

### Via VS Code Task

Use the task: **Utility: Check Version Match Tag**

## Parameters

This skill accepts no parameters.

## Environment Variables

This skill requires no environment variables.

## Outputs

- **Success Exit Code**: 0 - Version matches latest tag or no tags exist
- **Error Exit Codes**: 1 - Version mismatch detected
- **Console Output**: Validation result message

### Success Output Example

```
OK: .version matches latest Git tag v0.3.0-beta.2
```

### Error Output Example

```
ERROR: .version (0.3.0-beta.3) does not match latest Git tag (v0.3.0-beta.2)
To sync, either update .version or tag with 'v0.3.0-beta.3'
```

## Examples

### Example 1: Check Version During Release

```bash
# Before tagging a new release
.github/skills/utility-version-check-scripts/run.sh
```

### Example 2: CI/CD Integration

```yaml
- name: Validate Version
  run: .github/skills/scripts/skill-runner.sh utility-version-check
```

## Version Normalization

The skill normalizes both the `.version` file content and git tag by:
- Stripping leading `v` prefix (e.g., `v1.0.0` → `1.0.0`)
- Removing newline and carriage return characters
- Comparing normalized versions

This allows flexibility in tagging conventions while ensuring consistency.

## Error Handling

- **No .version file**: Exits with 0 (skip check)
- **No git tags**: Exits with 0 (skip check, allows commits before first tag)
- **Version mismatch**: Exits with 1 and provides guidance
- **Git errors**: Script fails with appropriate error message

## Related Skills

- [utility-bump-beta](./utility-bump-beta.SKILL.md) - Increment beta version
- [build-check-go](../build-check-go.SKILL.md) - Verify Go build integrity

## Notes

- This check is **non-blocking** when no tags exist (allows initial development)
- Version format is flexible (supports semver, beta, alpha suffixes)
- Used in CI/CD to prevent merging PRs with version mismatches
- Part of the release automation workflow

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `scripts/check-version-match-tag.sh`
