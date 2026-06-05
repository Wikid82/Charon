# Fix Codecov Patch Coverage

You are a senior test engineer. Analyze the provided Codecov report and generate the minimum set of high-quality tests required to achieve **100% patch coverage** on all modified lines.

## Input

$ARGUMENTS

(Provide ONE of: a Codecov bot comment from a PR, a Codecov report link, or specific file + line references like `backend/internal/services/mail_service.go lines 45-48`.)

## Execution Protocol

### Phase 1: Parse and Identify

Extract: files with missing patch coverage, specific uncovered line numbers, current patch coverage percentage.

### Phase 2: Analyze Uncovered Code

For each file: read the source, understand what the uncovered lines do, identify what inputs or conditions trigger those lines (error paths, edge cases, conditional branches).

Find corresponding test files:
- Go: `*_test.go` in the same package
- TypeScript: `*.test.ts` or `*.spec.ts`

### Phase 3: Generate Tests

Follow project patterns from existing tests. Write targeted tests that:
- Exercise the specific uncovered lines
- Verify behavior (not just coverage)
- Are deterministic and independent
- Use descriptive test names

**Go pattern** (table-driven tests):
```go
func TestFunctionName_Scenario(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {name: "descriptive case", input: ..., want: ...},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

**TypeScript pattern** (Vitest):
```typescript
describe('Component', () => {
    it('should handle edge case at line XX', () => {
        // Arrange
        // Act
        // Assert
    });
});
```

### Phase 4: Validate

Run the new tests (`go test ./...` or `npm test`), verify they pass, confirm existing tests still pass.

## Constraints

- **Do NOT relax coverage thresholds** — always aim for 100% patch coverage
- **Do NOT write tests that only exist for coverage** — tests must verify behavior
- **Do NOT modify production code** unless a bug is discovered during testing
- **Do NOT create flaky tests** — all tests must be deterministic
