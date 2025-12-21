---
# agentskills.io specification v1.0
name: "test-frontend-unit"
version: "1.0.0"
description: "Run frontend unit tests without coverage analysis (fast execution)"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "unit-tests"
  - "frontend"
  - "vitest"
  - "fast"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "node"
    version: ">=18.0"
    optional: false
  - name: "npm"
    version: ">=9.0"
    optional: false
environment_variables: []
parameters:
  - name: "watch"
    type: "boolean"
    description: "Run tests in watch mode"
    default: "false"
    required: false
  - name: "filter"
    type: "string"
    description: "Filter tests by name pattern"
    default: ""
    required: false
outputs:
  - name: "test_results"
    type: "stdout"
    description: "Vitest output showing pass/fail status"
metadata:
  category: "test"
  subcategory: "unit"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Test Frontend Unit

## Overview

Executes the frontend unit test suite using Vitest without coverage analysis. This skill provides fast test execution for quick feedback during development, making it ideal for pre-commit checks and rapid iteration.

Unlike test-frontend-coverage, this skill does not generate coverage reports or enforce coverage thresholds, focusing purely on test pass/fail status.

## Prerequisites

- Node.js 18.0 or higher installed and in PATH
- npm 9.0 or higher installed and in PATH
- Frontend dependencies installed (`cd frontend && npm install`)

## Usage

### Basic Usage

Run all frontend unit tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh test-frontend-unit
```

### Watch Mode

Run tests in watch mode for continuous testing:

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- --watch
```

### Filter Tests

Run tests matching a specific pattern:

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- --grep "Button"
```

### CI/CD Integration

For use in GitHub Actions or other CI/CD pipelines:

```yaml
- name: Run Frontend Unit Tests
  run: .github/skills/scripts/skill-runner.sh test-frontend-unit
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| watch     | boolean | No    | false   | Run tests in watch mode |
| filter    | string | No     | ""      | Filter tests by name pattern |

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
✓ src/components/Button.test.tsx (3)
✓ src/utils/helpers.test.ts (5)
✓ src/hooks/useAuth.test.ts (4)

Test Files  3 passed (3)
     Tests  12 passed (12)
```

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit
```

### Example 2: Watch Mode for TDD

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- --watch
```

### Example 3: Test Specific File

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- Button.test.tsx
```

### Example 4: UI Mode (Interactive)

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- --ui
```

### Example 5: Reporter Configuration

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit -- --reporter=verbose
```

## Error Handling

### Common Errors

#### Error: Cannot find module
**Solution**: Run `npm install` to ensure all dependencies are installed

#### Error: Test timeout
**Solution**: Increase timeout in vitest.config.ts or fix hanging async tests

#### Error: Unexpected token
**Solution**: Check for syntax errors in test files

## Related Skills

- test-frontend-coverage - Run tests with coverage analysis (slower)
- test-backend-unit - Backend Go unit tests
- build-check-go - Verify builds without running tests

## Notes

- **Execution Time**: Fast execution (~3-5 seconds typical)
- **No Coverage**: Does not generate coverage reports
- **Vitest Features**: Full access to Vitest CLI options via arguments
- **Idempotency**: Safe to run multiple times
- **Caching**: Benefits from Vitest's smart caching
- **Suitable For**: Pre-commit hooks, quick feedback, TDD workflows
- **Watch Mode**: Available for interactive development

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: Inline task command
