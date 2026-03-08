# Test: Backend Unit Tests

Run backend Go unit tests.

## Command

```bash
.github/skills/scripts/skill-runner.sh test-backend-unit
```

## Direct Alternative

```bash
cd backend && go test ./...
```

## Targeted Testing

```bash
# Single package
cd backend && go test ./internal/api/handlers/...

# Single test function
cd backend && go test ./... -run TestFunctionName -v

# With race detector
cd backend && go test -race ./...
```

## Related

- `/test-backend-coverage` — Run with coverage report (minimum 85%)
- `/test-frontend-unit` — Frontend unit tests
