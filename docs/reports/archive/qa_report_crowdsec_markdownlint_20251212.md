# QA Security Audit Report

**Date:** December 12, 2025
**QA Agent:** QA_Security
**Scope:** CrowdSec preset apply bug fix, regression tests, Markdownlint integration
**Overall Status:** ✅ **PASS**

---

## Summary

| Check | Status | Details |
|-------|--------|---------|
| CrowdSec Tests | ✅ PASS | All 62 tests pass in `internal/crowdsec/...` |
| Backend Build | ✅ PASS | `go build ./...` compiles without errors |
| Pre-commit | ✅ PASS | All hooks pass (85.1% coverage met) |
| JSON Validation | ✅ PASS | All JSON/JSONC files valid |
| YAML Validation | ✅ PASS | `.pre-commit-config.yaml` valid |

---

## 1. CrowdSec Preset Apply Bug Fix

**File:** [hub_sync.go](../../backend/internal/crowdsec/hub_sync.go)

### Fix Description

The bug fix addresses an issue where the preset archive file handle could become invalid during the apply process. The root cause was that the backup step was moving files that were still being referenced by the cache.

### Key Fix (Lines 530-543)

```go
// Read archive into memory BEFORE backup, since cache is inside DataDir.
// If we backup first, the archive path becomes invalid (file moved).
var archive []byte
var archiveReadErr error
if metaErr == nil {
    archive, archiveReadErr = os.ReadFile(meta.ArchivePath)
    if archiveReadErr != nil {
        logger.Log().WithError(archiveReadErr).WithField("archive_path", meta.ArchivePath).
            Warn("failed to read cached archive before backup")
    }
}
```

### Verification

✅ The fix ensures the archive is read into memory before any backup operations modify the file system.

---

## 2. Regression Tests

**File:** [hub_pull_apply_test.go](../../backend/internal/crowdsec/hub_pull_apply_test.go)

### Test Coverage

The new regression tests verify the pull-then-apply workflow:

| Test Name | Purpose | Status |
|-----------|---------|--------|
| `TestPullThenApplyFlow` | End-to-end pull → cache → apply | ✅ PASS |
| `TestApplyRepullsOnCacheMissAfterCSCLIFailure` | Cache refresh on miss | ✅ PASS |
| `TestApplyRepullsOnCacheExpired` | Cache refresh on TTL expiry | ✅ PASS |
| `TestApplyReadsArchiveBeforeBackup` | Archive memory load before backup | ✅ PASS |
| `TestBackupPathOnlySetAfterSuccessfulBackup` | Backup state integrity | ✅ PASS |
| `TestApplyWithOpenFileHandles` | File handle safety | ✅ PASS |

### Test Execution Output

```
=== RUN   TestPullThenApplyFlow
    hub_pull_apply_test.go:90: Step 1: Pulling preset
    hub_pull_apply_test.go:110: Step 2: Verifying cache can be loaded
    hub_pull_apply_test.go:117: Step 3: Applying preset from cache
--- PASS: TestPullThenApplyFlow (0.00s)
```

---

## 3. Markdownlint Integration

### Files Modified

| File | Status | Notes |
|------|--------|-------|
| `.markdownlint.json` | ✅ Valid | Line length 120, code blocks 150 |
| `.pre-commit-config.yaml` | ✅ Valid | Markdownlint hook added (manual stage) |
| `.vscode/tasks.json` | ✅ Valid | JSONC format with lint tasks |
| `package.json` | ✅ Valid | `markdownlint-cli2` devDependency |

### Markdownlint Configuration

**File:** [.markdownlint.json](../../.markdownlint.json)

```json
{
  "default": true,
  "MD013": {
    "line_length": 120,
    "heading_line_length": 120,
    "code_block_line_length": 150,
    "tables": false
  },
  "MD024": { "siblings_only": true },
  "MD033": {
    "allowed_elements": ["details", "summary", "br", "sup", "sub", "kbd", "img"]
  },
  "MD041": false,
  "MD046": { "style": "fenced" }
}
```

### Pre-commit Integration

**File:** [.pre-commit-config.yaml](../../.pre-commit-config.yaml) (Lines 118-124)

```yaml
- repo: https://github.com/igorshubovych/markdownlint-cli
  rev: v0.43.0
  hooks:
    - id: markdownlint
      args: ["--fix"]
      exclude: '^(node_modules|\.venv|test-results|codeql-db|codeql-agent-results)/'
      stages: [manual]
```

### VS Code Tasks

**File:** [.vscode/tasks.json](../../.vscode/tasks.json)

Two new tasks added:

- `Lint: Markdownlint` - Check markdown files
- `Lint: Markdownlint Fix` - Auto-fix markdown issues

---

## 4. Validation Results

### JSON/YAML Syntax Validation

```
✓ .markdownlint.json is valid JSON
✓ package.json is valid JSON
✓ .vscode/tasks.json is valid JSONC (with comments stripped)
✓ .pre-commit-config.yaml is valid YAML
```

### Backend Build

```bash
$ cd /projects/Charon/backend && go build ./...
# No errors
```

### Pre-commit Hooks

```
Go Build & Test......................................................Passed
Go Vet...............................................................Passed
Check .version matches latest Git tag................................Passed
Prevent large files that are not tracked by LFS......................Passed
Prevent committing CodeQL DB artifacts...............................Passed
Prevent committing data/backups files................................Passed
Frontend TypeScript Check............................................Passed
Frontend Lint (Fix)..................................................Passed
```

Coverage: **85.1%** (minimum required: 85%) ✅

---

## 5. Security Notes

- ✅ No secrets or sensitive data exposed
- ✅ File path sanitization in place (`sanitizeSlug`, `filepath.Clean`)
- ✅ Archive size limits enforced (25 MiB max)
- ✅ Symlink rejection in tar extraction (path traversal prevention)
- ✅ Graceful error handling with rollback on failure

---

## Conclusion

All verification steps pass. The CrowdSec preset apply bug fix correctly reads the archive into memory before backup operations, preventing file handle invalidation. The new regression tests provide comprehensive coverage for the pull-apply workflow. Markdownlint integration is properly configured for manual linting.

**Status: ✅ PASS - Ready for merge**
