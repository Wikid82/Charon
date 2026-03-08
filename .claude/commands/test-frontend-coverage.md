# Test: Frontend Coverage

Run frontend tests with coverage reporting. Minimum threshold: 85%.

## Command

```bash
.github/skills/scripts/skill-runner.sh test-frontend-coverage
```

## Direct Alternative

```bash
bash scripts/frontend-test-coverage.sh
```

## What It Does

1. Runs all Vitest tests with V8 coverage provider
2. Generates HTML + JSON coverage reports
3. Checks against minimum threshold (85%)
4. Fails if below threshold

## View Coverage Report

```bash
cd frontend && npx vite preview --outDir coverage
# Or open coverage/index.html in browser
```

## Fix Coverage Gaps

If coverage is below 85%:
1. Run `/codecov-patch-fix` to identify uncovered lines
2. Write targeted tests with Testing Library
3. Re-run coverage to verify

## Related

- `/test-frontend-unit` — Run tests without coverage
- `/test-backend-coverage` — Backend coverage (also 85% minimum)
- `/codecov-patch-fix` — Fix specific coverage gaps
