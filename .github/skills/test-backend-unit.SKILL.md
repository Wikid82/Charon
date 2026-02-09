---
# agentskills.io specification v1.0
name: "test-backend-unit"
version: "1.0.0"
description: "Run Go backend unit tests without coverage analysis (fast execution)"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "unit-tests"
  - "go"
  - "backend"
  - "fast"
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
environment_variables: []
parameters:
  - name: "verbose"
    type: "boolean"
    description: "Enable verbose test output"
    default: "false"
    required: false
  - name: "package"
    type: "string"
    description: "Specific package to test (e.g., ./internal/...)"
    default: "./..."
    required: false
outputs:
  - name: "test_results"
    type: "stdout"
    description: "Go test output showing pass/fail status"
metadata:
  category: "test"
  subcategory: "unit"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Test Backend Unit

## Overview

Executes the Go backend unit test suite without coverage analysis. This skill provides fast test execution for quick feedback during development, making it ideal for pre-commit checks and rapid iteration.

Unlike test-backend-coverage, this skill does not generate coverage reports or enforce coverage thresholds, focusing purely on test pass/fail status.

## Prerequisites

- Go 1.23 or higher installed and in PATH
- Backend dependencies installed (`cd backend && go mod download`)
- Sufficient disk space for test artifacts

## Usage

### Basic Usage

Run all backend unit tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh test-backend-unit
```

### Test Specific Package

Test only a specific package or module:

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- ./internal/handlers/...
```

### Verbose Output

Enable verbose test output for debugging:

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- -v
```

### CI/CD Integration

For use in GitHub Actions or other CI/CD pipelines:

```yaml
- name: Run Backend Unit Tests
  run: .github/skills/scripts/skill-runner.sh test-backend-unit
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose test output (-v flag) |
| package   | string | No     | ./...   | Package pattern to test |

## Environment Variables

No environment variables are required for this skill.

## Outputs

### Success Exit Code
- **0**: All tests passed

### Error Exit Codes
- **Non-zero**: One or more tests failed

### Console Output
Example output:
```
ok      github.com/Wikid82/charon/backend/internal/handlers    0.523s
ok      github.com/Wikid82/charon/backend/internal/models      0.189s
ok      github.com/Wikid82/charon/backend/internal/services    0.742s
```

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit
```

### Example 2: Test Specific Package

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- ./internal/handlers
```

### Example 3: Verbose Output

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- -v
```

### Example 4: Run with Race Detection

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- -race
```

### Example 5: Short Mode (Skip Long Tests)

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit -- -short
```

## Error Handling

### Common Errors

#### Error: package not found
**Solution**: Verify package path is correct; run `go list ./...` to see available packages

#### Error: build failed
**Solution**: Fix compilation errors; run `go build ./...` to identify issues

#### Error: test timeout
**Solution**: Increase timeout with `-timeout` flag or fix hanging tests

## Related Skills

- test-backend-coverage - Run tests with coverage analysis (slower)
- build-check-go - Verify Go builds without running tests
- security-check-govulncheck - Go vulnerability scanning

## Notes

- **Execution Time**: Fast execution (~5-10 seconds typical)
- **No Coverage**: Does not generate coverage reports
- **Race Detection**: Not enabled by default (unlike test-backend-coverage)
- **Idempotency**: Safe to run multiple times
- **Caching**: Benefits from Go test cache for unchanged packages
- **Suitable For**: Pre-commit hooks, quick feedback, TDD workflows

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: Inline task command
