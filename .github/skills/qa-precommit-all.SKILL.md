---
# agentskills.io specification v1.0
name: "qa-precommit-all"
version: "1.0.0"
description: "Run all pre-commit hooks for comprehensive code quality validation"
author: "Charon Project"
license: "MIT"
tags:
  - "qa"
  - "quality"
  - "pre-commit"
  - "linting"
  - "validation"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "python3"
    version: ">=3.8"
    optional: false
  - name: "pre-commit"
    version: ">=2.0"
    optional: false
environment_variables:
  - name: "PRE_COMMIT_HOME"
    description: "Pre-commit cache directory"
    default: "~/.cache/pre-commit"
    required: false
  - name: "SKIP"
    description: "Comma-separated list of hook IDs to skip"
    default: ""
    required: false
parameters:
  - name: "files"
    type: "string"
    description: "Specific files to check (default: all staged files)"
    default: "--all-files"
    required: false
outputs:
  - name: "validation_report"
    type: "stdout"
    description: "Results of all pre-commit hook executions"
  - name: "exit_code"
    type: "number"
    description: "0 if all hooks pass, non-zero if any fail"
metadata:
  category: "qa"
  subcategory: "quality"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# QA Pre-commit All

## Overview

Executes all configured pre-commit hooks to validate code quality, formatting, security, and best practices across the entire codebase. This skill runs checks for Python, Go, JavaScript/TypeScript, Markdown, YAML, and more.

This skill is designed for CI/CD pipelines and local quality validation before committing code.

## Prerequisites

- Python 3.8 or higher installed and in PATH
- Python virtual environment activated (`.venv`)
- Pre-commit installed in virtual environment: `pip install pre-commit`
- Pre-commit hooks installed: `pre-commit install`
- All language-specific tools installed (Go, Node.js, etc.)

## Usage

### Basic Usage

Run all hooks on all files:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh qa-precommit-all
```

### Staged Files Only

Run hooks on staged files only (faster):

```bash
.github/skills/scripts/skill-runner.sh qa-precommit-all staged
```

### Specific Hook

Run only a specific hook by ID:

```bash
SKIP="" .github/skills/scripts/skill-runner.sh qa-precommit-all trailing-whitespace
```

### Skip Specific Hooks

Skip certain hooks during execution:

```bash
SKIP=prettier,eslint .github/skills/scripts/skill-runner.sh qa-precommit-all
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| files     | string | No     | --all-files | File selection mode (--all-files or staged) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| SKIP | No | "" | Comma-separated hook IDs to skip |
| PRE_COMMIT_HOME | No | ~/.cache/pre-commit | Pre-commit cache directory |

## Outputs

- **Success Exit Code**: 0 (all hooks passed)
- **Error Exit Codes**: Non-zero (one or more hooks failed)
- **Output**: Detailed results from each hook

## Pre-commit Hooks Included

The following hooks are configured in `.pre-commit-config.yaml`:

### General Hooks
- **trailing-whitespace**: Remove trailing whitespace
- **end-of-file-fixer**: Ensure files end with newline
- **check-yaml**: Validate YAML syntax
- **check-json**: Validate JSON syntax
- **check-merge-conflict**: Detect merge conflict markers
- **check-added-large-files**: Prevent committing large files

### Python Hooks
- **black**: Code formatting
- **isort**: Import sorting
- **flake8**: Linting
- **mypy**: Type checking

### Go Hooks
- **gofmt**: Code formatting
- **go-vet**: Static analysis
- **golangci-lint**: Comprehensive linting

### JavaScript/TypeScript Hooks
- **prettier**: Code formatting
- **eslint**: Linting and code quality

### Markdown Hooks
- **markdownlint**: Markdown linting and formatting

### Security Hooks
- **detect-private-key**: Prevent committing private keys
- **detect-aws-credentials**: Prevent committing AWS credentials

## Examples

### Example 1: Full Quality Check

```bash
# Run all hooks on all files
source .venv/bin/activate
.github/skills/scripts/skill-runner.sh qa-precommit-all
```

Output:
```
Trim Trailing Whitespace.....................................Passed
Fix End of Files.............................................Passed
Check Yaml...................................................Passed
Check JSON...................................................Passed
Check for merge conflicts....................................Passed
Check for added large files..................................Passed
black........................................................Passed
isort........................................................Passed
prettier.....................................................Passed
eslint.......................................................Passed
markdownlint.................................................Passed
```

### Example 2: Quick Staged Files Check

```bash
# Run only on staged files (faster for pre-commit)
.github/skills/scripts/skill-runner.sh qa-precommit-all staged
```

### Example 3: Skip Slow Hooks

```bash
# Skip time-consuming hooks for quick validation
SKIP=golangci-lint,mypy .github/skills/scripts/skill-runner.sh qa-precommit-all
```

### Example 4: CI/CD Pipeline Integration

```yaml
# GitHub Actions example
- name: Set up Python
  uses: actions/setup-python@v4
  with:
    python-version: '3.11'

- name: Install pre-commit
  run: pip install pre-commit

- name: Run QA Pre-commit Checks
  run: .github/skills/scripts/skill-runner.sh qa-precommit-all
```

### Example 5: Auto-fix Mode

```bash
# Some hooks can auto-fix issues
# Run twice: first to fix, second to validate
.github/skills/scripts/skill-runner.sh qa-precommit-all || \
.github/skills/scripts/skill-runner.sh qa-precommit-all
```

## Error Handling

### Common Issues

**Virtual environment not activated**:
```bash
Error: pre-commit not found
Solution: source .venv/bin/activate
```

**Pre-commit not installed**:
```bash
Error: pre-commit command not available
Solution: pip install pre-commit
```

**Hooks not installed**:
```bash
Error: Run 'pre-commit install'
Solution: pre-commit install
```

**Hook execution failed**:
```bash
Hook X failed
Solution: Review error output and fix reported issues
```

**Language tool missing**:
```bash
Error: golangci-lint not found
Solution: Install required language tools
```

## Exit Codes

- **0**: All hooks passed
- **1**: One or more hooks failed
- **Other**: Hook execution error

## Hook Fixing Strategies

### Auto-fixable Issues
These hooks automatically fix issues:
- `trailing-whitespace`
- `end-of-file-fixer`
- `black`
- `isort`
- `prettier`
- `gofmt`

**Workflow**: Run pre-commit, review changes, commit fixed files

### Manual Fixes Required
These hooks only report issues:
- `check-yaml`
- `check-json`
- `flake8`
- `eslint`
- `markdownlint`
- `go-vet`
- `golangci-lint`

**Workflow**: Review errors, manually fix code, re-run pre-commit

## Related Skills

- [test-backend-coverage](./test-backend-coverage.SKILL.md) - Backend test coverage
- [test-frontend-coverage](./test-frontend-coverage.SKILL.md) - Frontend test coverage
- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Security scanning

## Notes

- Pre-commit hooks cache their environments for faster execution
- First run may be slow while environments are set up
- Subsequent runs are much faster (seconds vs minutes)
- Hooks run in parallel where possible
- Failed hooks stop execution (fail-fast behavior)
- Use `SKIP` to bypass specific hooks temporarily
- Recommended to run before every commit
- Can be integrated into Git pre-commit hook for automatic checks
- Cache location: `~/.cache/pre-commit` (configurable)

## Performance Tips

- **Initial Setup**: First run takes longer (installing hook environments)
- **Incremental**: Run on staged files only for faster feedback
- **Parallel**: Pre-commit runs compatible hooks in parallel
- **Cache**: Hook environments are cached and reused
- **Skip**: Use `SKIP` to bypass slow hooks during development

## Integration with Git

To automatically run on every commit:

```bash
# Install Git pre-commit hook
pre-commit install

# Now pre-commit runs automatically on git commit
git commit -m "Your commit message"
```

To bypass pre-commit hook temporarily:

```bash
git commit --no-verify -m "Emergency commit"
```

## Configuration

Pre-commit configuration is in `.pre-commit-config.yaml`. To update hooks:

```bash
# Update to latest versions
pre-commit autoupdate

# Clean cache and re-install
pre-commit clean
pre-commit install --install-hooks
```

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `pre-commit run --all-files`
