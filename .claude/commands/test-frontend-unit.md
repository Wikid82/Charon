# Test: Frontend Unit Tests

Run frontend TypeScript/React unit tests with Vitest.

## Command

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit
```

## Direct Alternative

```bash
cd frontend && npm test
```

## Targeted Testing

```bash
# Single file
cd frontend && npm test -- src/components/MyComponent.test.tsx

# Watch mode (re-runs on file changes)
cd frontend && npm test -- --watch

# With verbose output
cd frontend && npm test -- --reporter=verbose
```

## Related

- `/test-frontend-coverage` — Run with coverage report (minimum 85%)
- `/test-backend-unit` — Backend unit tests
