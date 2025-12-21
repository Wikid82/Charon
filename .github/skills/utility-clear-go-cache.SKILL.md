---
name: "utility-clear-go-cache"
version: "1.0.0"
description: "Clears Go build, test, and module caches along with gopls cache for troubleshooting"
author: "Charon Project"
license: "MIT"
tags:
  - "utility"
  - "golang"
  - "cache"
  - "troubleshooting"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "go"
    version: ">=1.23"
    optional: false
environment_variables:
  - name: "XDG_CACHE_HOME"
    description: "XDG cache directory (defaults to $HOME/.cache)"
    default: "$HOME/.cache"
    required: false
parameters: []
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 on success, 1 on failure"
metadata:
  category: "utility"
  subcategory: "cache-management"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: false
  requires_network: true
  idempotent: true
---

# Utility: Clear Go Cache

## Overview

Clears all Go-related caches including build cache, test cache, module cache, and gopls (Go Language Server) cache. This is useful for troubleshooting build issues, resolving stale dependency problems, or cleaning up disk space.

## Prerequisites

- Go toolchain installed (go 1.23+)
- Write access to cache directories
- Internet connection (for re-downloading modules)

## Usage

### Basic Usage

```bash
.github/skills/utility-clear-go-cache-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh utility-clear-go-cache
```

### Via VS Code Task

Use the task: **Utility: Clear Go Cache**

## Parameters

This skill accepts no parameters.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| XDG_CACHE_HOME | No | $HOME/.cache | XDG cache directory location |

## Outputs

- **Success Exit Code**: 0
- **Error Exit Codes**: 1 - Cache clearing failed
- **Console Output**: Progress messages and next steps

### Output Example

```
Clearing Go build and module caches...
Clearing gopls cache...
Re-downloading modules...
Caches cleared and modules re-downloaded.
Next steps:
- Restart your editor's Go language server (gopls)
  - In VS Code: Command Palette -> 'Go: Restart Language Server'
- Verify the toolchain:
  $ go version
  $ gopls version
```

## Examples

### Example 1: Troubleshoot Build Issues

```bash
# Clear caches when experiencing build errors
.github/skills/utility-clear-go-cache-scripts/run.sh

# Restart VS Code's Go language server
# Command Palette: "Go: Restart Language Server"
```

### Example 2: Clean Development Environment

```bash
# Clear caches before major Go version upgrade
.github/skills/utility-clear-go-cache-scripts/run.sh

# Verify installation
go version
gopls version
```

## What Gets Cleared

This skill clears the following:

1. **Go Build Cache**: `go clean -cache`
   - Compiled object files
   - Build artifacts

2. **Go Test Cache**: `go clean -testcache`
   - Cached test results

3. **Go Module Cache**: `go clean -modcache`
   - Downloaded module sources
   - Module checksums

4. **gopls Cache**: Removes `$XDG_CACHE_HOME/gopls` or `$HOME/.cache/gopls`
   - Language server indexes
   - Cached analysis results

5. **Re-downloads**: `go mod download`
   - Fetches all dependencies fresh

## When to Use This Skill

Use this skill when experiencing:
- Build failures after dependency updates
- gopls crashes or incorrect diagnostics
- Module checksum mismatches
- Stale test cache results
- Disk space issues related to Go caches
- IDE reporting incorrect errors

## Error Handling

- All cache clearing operations use `|| true` to continue even if a cache doesn't exist
- Module re-download requires network access
- Exits with error if `backend/` directory not found

## Related Skills

- [build-check-go](../build-check-go.SKILL.md) - Verify Go build after cache clear
- [test-backend-unit](./test-backend-unit.SKILL.md) - Run tests after cache clear

## Notes

- **Warning**: This operation re-downloads all Go modules (may be slow on poor network)
- Not CI/CD safe due to network dependency and destructive nature
- Requires manual IDE restart after execution
- Safe to run multiple times (idempotent)
- Consider using this before major Go version upgrades

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `scripts/clear-go-cache.sh`
