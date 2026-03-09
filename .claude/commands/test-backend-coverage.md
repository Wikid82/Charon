# Test: Backend Coverage

Run backend Go tests with coverage reporting. Minimum threshold: 85%.

## Command

```bash
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

## Direct Alternative

```bash
bash scripts/go-test-coverage.sh
```

## What It Does

1. Runs all Go tests with `-coverprofile`
2. Generates HTML coverage report
3. Checks against minimum threshold (`CHARON_MIN_COVERAGE=85`)
4. Fails if below threshold

## View Coverage Report

```bash
cd backend && go tool cover -html=coverage.out
```

## Fix Coverage Gaps

If coverage is below 85%:
1. Run `/codecov-patch-fix` to identify uncovered lines
2. Write targeted tests for error paths and edge cases
3. Re-run coverage to verify

## Related

- `/test-backend-unit` — Run tests without coverage
- `/test-frontend-coverage` — Frontend coverage (also 85% minimum)
- `/codecov-patch-fix` — Fix specific coverage gaps
