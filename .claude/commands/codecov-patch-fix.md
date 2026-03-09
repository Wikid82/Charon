# Codecov Patch Coverage Fix

Analyze Codecov coverage gaps and generate the minimum set of high-quality tests to achieve 100% patch coverage on all modified lines.

**Input**: $ARGUMENTS — provide ONE of:
1. Codecov bot comment (copy/paste from PR)
2. File path + uncovered line ranges (e.g., `backend/internal/services/mail_service.go lines 45-48`)

## Execution Protocol

### Phase 1: Parse and Identify

Extract from the input:
- Files with missing patch coverage
- Specific line numbers/ranges that are uncovered
- Current patch coverage percentage

Document as:
```
UNCOVERED FILES:
- FILE-001: [path/to/file.go] - Lines: [45-48, 62]
- FILE-002: [path/to/other.ts] - Lines: [23, 67-70]
```

### Phase 2: Analyze Uncovered Code

For each file:
1. Read the source file — understand what the uncovered lines do
2. Identify what condition/input/state would execute those lines (error paths, edge cases, branches)
3. Find the corresponding test file(s)

### Phase 3: Generate Tests

Follow **existing project patterns** — analyze the test file before writing:
- Go: table-driven tests with `t.Run`
- TypeScript: Vitest `describe`/`it` with `vi.spyOn` for mocks
- Arrange-Act-Assert structure
- Descriptive test names that explain the scenario

**Go pattern**:
```go
func TestFunctionName_EdgeCase(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        wantErr bool
    }{
        {name: "handles nil input", input: nil, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
            }
        })
    }
}
```

**TypeScript pattern**:
```typescript
it('should handle error condition at line XX', async () => {
    vi.spyOn(dependency, 'method').mockRejectedValue(new Error('test error'));
    await expect(functionUnderTest()).rejects.toThrow('expected error message');
});
```

### Phase 4: Validate

1. Run the new tests: `go test ./...` or `npm test`
2. Run coverage: `scripts/go-test-coverage.sh` or `scripts/frontend-test-coverage.sh`
3. Confirm no regressions

## Constraints

- **DO NOT** relax coverage thresholds — always target 100% patch coverage
- **DO NOT** write tests just for coverage — tests must verify behaviour
- **DO NOT** modify production code unless a bug is discovered
- **DO NOT** create flaky tests — all tests must be deterministic
- **DO NOT** skip error handling paths — these are the most common coverage gaps
