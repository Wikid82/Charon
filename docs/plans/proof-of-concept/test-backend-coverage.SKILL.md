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

### VS Code Task Integration

This skill is integrated as a VS Code task:

```json
{
    "label": "Test: Backend with Coverage",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh test-backend-coverage",
    "group": "test"
}
```

Run via: `Tasks: Run Task` → `Test: Backend with Coverage`

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

**Note**: Performance thresholds are loosened when running with `-race` flag due to overhead.

## Outputs

### Success Exit Code

- **0**: All tests passed and coverage meets threshold

### Error Exit Codes

- **1**: Coverage below threshold or coverage file generation failed
- **Non-zero**: Tests failed or other error occurred

### Output Files

- **backend/coverage.txt**: Go coverage profile (text format)
  - Contains coverage data for all tested packages
  - Filtered to exclude main packages and infrastructure code
  - Used by `go tool cover` for analysis

### Console Output

The skill outputs:

1. Test execution progress (verbose mode)
2. Coverage filtering status
3. Total coverage percentage summary
4. Coverage validation result (pass/fail)

Example output:

```
Filtering excluded packages from coverage report...
Coverage filtering complete
github.com/Wikid82/charon/backend/internal/api/handlers  GetStatus       95.2%
...
total:                                                  (statements)    87.4%
Computed coverage: 87.4% (minimum required 85%)
Coverage requirement met
```

## Examples

### Example 1: Basic Execution

Run tests with default settings:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

Expected output:

```
Filtering excluded packages from coverage report...
Coverage filtering complete
total:  (statements)    87.4%
Computed coverage: 87.4% (minimum required 85%)
Coverage requirement met
```

### Example 2: Higher Coverage Threshold

Enforce stricter coverage requirement:

```bash
export CHARON_MIN_COVERAGE=90
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

If coverage is below 90%:

```
total:  (statements)    87.4%
Computed coverage: 87.4% (minimum required 90%)
Coverage 87.4% is below required 90% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)
```

### Example 3: CI/CD with Verbose Output

Run in GitHub Actions with full test output:

```yaml
- name: Run Backend Tests with Coverage
  run: |
    export VERBOSE=true
    .github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Example 4: Pre-commit Hook

Add to `.git/hooks/pre-commit`:

```bash
#!/usr/bin/env bash
echo "Running backend tests with coverage..."
if ! .github/skills/scripts/skill-runner.sh test-backend-coverage; then
    echo "❌ Coverage check failed. Commit aborted."
    exit 1
fi
echo "✅ Coverage check passed."
```

## Excluded Packages

The following packages are excluded from coverage analysis as they are entrypoints or infrastructure code that don't benefit from unit tests:

- `github.com/Wikid82/charon/backend/cmd/api` - API server entrypoint
- `github.com/Wikid82/charon/backend/cmd/seed` - Database seeding tool
- `github.com/Wikid82/charon/backend/internal/logger` - Logging infrastructure
- `github.com/Wikid82/charon/backend/internal/metrics` - Metrics infrastructure
- `github.com/Wikid82/charon/backend/internal/trace` - Tracing infrastructure
- `github.com/Wikid82/charon/backend/integration` - Integration test utilities

**Rationale**: These packages are primarily initialization code, external integrations, or test harnesses that are validated through integration tests rather than unit tests.

## Error Handling

### Common Errors and Solutions

#### Error: coverage file not generated by go test

**Cause**: Test execution failed before coverage generation
**Solution**: Review test output for failures; fix failing tests

#### Error: go tool cover failed or timed out after 60 seconds

**Cause**: Corrupted coverage data or memory issues
**Solution**:

1. Clear Go cache: `.github/skills/scripts/skill-runner.sh utility-cache-clear-go`
2. Re-run tests
3. Check available memory

#### Error: Coverage X% is below required Y%

**Cause**: Code coverage does not meet threshold
**Solution**:

1. Add tests for uncovered code paths
2. Review coverage report: `go tool cover -html=backend/coverage.txt`
3. If threshold is too strict, adjust `CHARON_MIN_COVERAGE`

#### Error: Coverage filtering failed or timed out

**Cause**: Large coverage file or sed performance issue
**Solution**: The skill automatically falls back to unfiltered coverage; investigate if this occurs frequently

### Exit Codes Reference

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Tests passed, coverage met |
| 1 | Coverage failure | Add tests or adjust threshold |
| Non-zero | Test failure | Fix failing tests |

## Performance Considerations

### Execution Time

- **Fast machines**: ~30-60 seconds
- **CI/CD environments**: ~60-120 seconds
- **With -race flag**: +30% overhead

### Resource Usage

- **CPU**: High during test execution (parallel tests)
- **Memory**: ~500MB peak (race detector overhead)
- **Disk**: ~10MB for coverage.txt

### Optimization Tips

1. Run without `-race` for faster local testing (not recommended for CI/CD)
2. Use `go test -short` to skip long-running tests during development
3. Increase `GOMAXPROCS` for faster parallel test execution

## Related Skills

- [test-backend-unit](./test-backend-unit.SKILL.md) - Fast unit tests without coverage
- [security-check-govulncheck](./security-check-govulncheck.SKILL.md) - Go vulnerability scanning
- [build-check-go](./build-check-go.SKILL.md) - Verify Go build succeeds
- [utility-cache-clear-go](./utility-cache-clear-go.SKILL.md) - Clear Go build cache

## Integration with VS Code Tasks

This skill is integrated as a VS Code task defined in `.vscode/tasks.json`:

```json
{
    "label": "Test: Backend with Coverage",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh test-backend-coverage",
    "group": "test",
    "problemMatcher": []
}
```

**To run**:

1. Open Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`)
2. Select `Tasks: Run Task`
3. Choose `Test: Backend with Coverage`

## Integration with CI/CD

### GitHub Actions

Reference in `.github/workflows/quality-checks.yml`:

```yaml
jobs:
  backend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - name: Run Backend Tests with Coverage
        run: .github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Pre-commit Hook

Integrated via `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: backend-coverage
        name: Backend Coverage Check
        entry: .github/skills/scripts/skill-runner.sh test-backend-coverage
        language: system
        pass_filenames: false
```

## Notes

- **Race Detection**: This skill always runs with `-race` flag enabled to detect data races. This adds ~30% overhead but is critical for catching concurrency issues.
- **Coverage Filtering**: Packages excluded from coverage are defined in the script itself (not externally configurable) to maintain consistency across environments.
- **Python Dependency**: The skill uses Python for decimal-precision coverage comparison to avoid floating-point rounding issues in bash.
- **Timeout Protection**: Coverage generation has a 60-second timeout to prevent infinite hangs in CI/CD.
- **Idempotency**: This skill is safe to run multiple times; it cleans up old coverage files automatically.

## Troubleshooting

### Coverage Report Empty or Missing

1. Check that tests exist in `backend/` directory
2. Verify Go modules are downloaded: `cd backend && go mod download`
3. Check file permissions in `backend/` directory

### Tests Hang or Timeout

1. Identify slow tests: `go test -v -timeout 5m ./...`
2. Check for deadlocks in concurrent code
3. Disable race detector temporarily for debugging: `go test -timeout 5m ./...`

### Coverage Threshold Too Strict

If legitimate code cannot reach threshold:

1. Review uncovered lines: `go tool cover -html=backend/coverage.txt`
2. Add test cases for uncovered branches
3. If code is truly untestable (e.g., panic handlers), consider adjusting threshold

## Maintenance

### Updating Excluded Packages

To modify the list of excluded packages:

1. Edit the `EXCLUDE_PACKAGES` array in the script
2. Document the reason for exclusion
3. Test coverage calculation after changes

### Updating Performance Thresholds

To adjust performance assertion thresholds:

1. Update environment variable defaults in frontmatter
2. Document the reason for change in commit message
3. Verify CI/CD passes with new thresholds

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/go-test-coverage.sh`
**Migration Status**: Proof of Concept
**Lines of Code**: ~400 lines (under 500-line target)
