---
# agentskills.io specification v1.0
name: "test-backend-coverage"
version: "1.0.0"
description: "Run Go backend tests with coverage analysis and threshold validation (minimum 85%)"
author: "Charon Project"
license: "MIT"
tags:
  - "testing"
  - "coverage"
  - "go"
  - "backend"
  - "validation"
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
  - name: "python3"
    version: ">=3.8"
    optional: false
environment_variables:
  - name: "CHARON_MIN_COVERAGE"
    description: "Minimum coverage percentage required (overrides default)"
    default: "85"
    required: false
  - name: "CPM_MIN_COVERAGE"
    description: "Alternative name for minimum coverage threshold (legacy)"
    default: "85"
    required: false
  - name: "PERF_MAX_MS_GETSTATUS_P95"
    description: "Maximum P95 latency for GetStatus endpoint (ms)"
    default: "25ms"
    required: false
  - name: "PERF_MAX_MS_GETSTATUS_P95_PARALLEL"
    description: "Maximum P95 latency for parallel GetStatus calls (ms)"
    default: "50ms"
    required: false
  - name: "PERF_MAX_MS_LISTDECISIONS_P95"
    description: "Maximum P95 latency for ListDecisions endpoint (ms)"
    default: "75ms"
    required: false
parameters:
  - name: "verbose"
    type: "boolean"
    description: "Enable verbose test output"
    default: "false"
    required: false
outputs:
  - name: "coverage.txt"
    type: "file"
    description: "Go coverage profile in text format"
    path: "backend/coverage.txt"
  - name: "coverage_summary"
    type: "stdout"
    description: "Summary of coverage statistics and validation result"
metadata:
  category: "test"
  subcategory: "coverage"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Test Backend Coverage

## Overview

Executes the Go backend test suite with race detection enabled, generates a coverage profile, filters excluded packages, and validates that the total coverage meets or exceeds the configured threshold (default: 85%).

This skill is designed for continuous integration and pre-commit hooks to ensure code quality standards are maintained.

## Prerequisites

- Go 1.23 or higher installed and in PATH
- Python 3.8 or higher installed and in PATH
- Backend dependencies installed (`cd backend && go mod download`)
- Write permissions in `backend/` directory (for coverage.txt)

## Usage

### Basic Usage

Run with default settings (85% minimum coverage):

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Custom Coverage Threshold

Set a custom minimum coverage percentage:

```bash
export CHARON_MIN_COVERAGE=90
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

### CI/CD Integration

For use in GitHub Actions or other CI/CD pipelines:

```yaml
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage
  env:
    CHARON_MIN_COVERAGE: 85
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose test output (-v flag) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CHARON_MIN_COVERAGE | No | 85 | Minimum coverage percentage required for success |
| CPM_MIN_COVERAGE | No | 85 | Legacy name for minimum coverage (fallback) |
| PERF_MAX_MS_GETSTATUS_P95 | No | 25ms | Max P95 latency for GetStatus endpoint |
| PERF_MAX_MS_GETSTATUS_P95_PARALLEL | No | 50ms | Max P95 latency for parallel GetStatus |
| PERF_MAX_MS_LISTDECISIONS_P95 | No | 75ms | Max P95 latency for ListDecisions endpoint |

## Outputs

### Success Exit Code
- **0**: All tests passed and coverage meets threshold

### Error Exit Codes
- **1**: Coverage below threshold or coverage file generation failed
- **Non-zero**: Tests failed or other error occurred

### Output Files
- **backend/coverage.txt**: Go coverage profile (text format)

### Console Output
Example output:
```
Filtering excluded packages from coverage report...
Coverage filtering complete
total:  (statements)    87.4%
Computed coverage: 87.4% (minimum required 85%)
Coverage requirement met
```

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Example 2: Higher Coverage Threshold

```bash
export CHARON_MIN_COVERAGE=90
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

## Excluded Packages

The following packages are excluded from coverage analysis:
- `github.com/Wikid82/charon/backend/cmd/api` - API server entrypoint
- `github.com/Wikid82/charon/backend/cmd/seed` - Database seeding tool
- `github.com/Wikid82/charon/backend/internal/logger` - Logging infrastructure
- `github.com/Wikid82/charon/backend/internal/metrics` - Metrics infrastructure
- `github.com/Wikid82/charon/backend/internal/trace` - Tracing infrastructure
- `github.com/Wikid82/charon/backend/integration` - Integration test utilities

## Error Handling

### Common Errors

#### Error: coverage file not generated by go test
**Solution**: Review test output for failures; fix failing tests

#### Error: go tool cover failed or timed out
**Solution**: Clear Go cache and re-run tests

#### Error: Coverage X% is below required Y%
**Solution**: Add tests for uncovered code paths or adjust threshold

## Related Skills

- test-backend-unit - Fast unit tests without coverage
- security-check-govulncheck - Go vulnerability scanning
- utility-cache-clear-go - Clear Go build cache

## Notes

- **Race Detection**: Always runs with `-race` flag enabled (adds ~30% overhead)
- **Coverage Filtering**: Excluded packages are defined in the script itself
- **Python Dependency**: Uses Python for decimal-precision coverage comparison
- **Timeout Protection**: Coverage generation has a 60-second timeout
- **Idempotency**: Safe to run multiple times; cleans up old coverage files

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/go-test-coverage.sh`
