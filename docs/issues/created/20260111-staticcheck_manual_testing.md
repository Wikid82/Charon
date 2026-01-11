# Staticcheck Pre-Commit Integration - Manual Testing Checklist

**Purpose:** Find potential bugs and edge cases in the staticcheck blocking implementation
**Date Created:** 2026-01-11
**Target:** Pre-commit hook blocking behavior and developer workflow

---

## Testing Overview

This checklist focuses on **adversarial testing** - finding ways the implementation might fail or cause developer friction.

---

## 1. Commit Blocking Scenarios

### 1.1 Basic Blocking Behavior

- [ ] **Test:** Create a `.go` file with an unused variable, attempt commit
  - **Expected:** Commit blocked, clear error message
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Create a `.go` file with unchecked error return, attempt commit
  - **Expected:** Commit blocked with errcheck error
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Create a `.go` file with shadowed variable, attempt commit
  - **Expected:** Commit blocked with govet/shadow error
  - **Actual:** _____________________
  - **Issues:** _____________________

### 1.2 Edge Case Files

- [ ] **Test:** Commit a `_test.go` file with lint issues
  - **Expected:** Commit succeeds (test files excluded)
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Commit Go file in subdirectory (e.g., `backend/internal/api/`)
  - **Expected:** Commit blocked if issues present
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Commit Go file in nested package (e.g., `backend/internal/api/handlers/proxy/`)
  - **Expected:** Recursive linting works correctly
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Commit `.go` file outside backend directory (edge case)
  - **Expected:** Hook runs correctly or gracefully handles
  - **Actual:** _____________________
  - **Issues:** _____________________

### 1.3 Multiple Files

- [ ] **Test:** Stage multiple `.go` files, some with issues, some clean
  - **Expected:** Commit blocked if any file has issues
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Stage mix of `.go`, `.js`, `.md` files with only Go issues
  - **Expected:** Commit blocked due to Go issues
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 2. Lint Error Types

### 2.1 Staticcheck Errors

- [ ] **Test:** SA1019 (deprecated API usage)
  - **Example:** `filepath.HasPrefix()`
  - **Expected:** Blocked with clear message
  - **Actual:** _____________________

- [ ] **Test:** SA4006 (value never used)
  - **Example:** `x := 1; x = 2`
  - **Expected:** Blocked with clear message
  - **Actual:** _____________________

- [ ] **Test:** SA1029 (string key for context.WithValue)
  - **Example:** `ctx = context.WithValue(ctx, "key", "value")`
  - **Expected:** Blocked with clear message
  - **Actual:** _____________________

### 2.2 Other Fast Linters

- [ ] **Test:** Unchecked error (errcheck)
  - **Example:** `file.Close()` without error check
  - **Expected:** Blocked
  - **Actual:** _____________________

- [ ] **Test:** Ineffectual assignment (ineffassign)
  - **Example:** Assign value that's never read
  - **Expected:** Blocked
  - **Actual:** _____________________

- [ ] **Test:** Unused function/variable (unused)
  - **Example:** Private function never called
  - **Expected:** Blocked
  - **Actual:** _____________________

- [ ] **Test:** Shadow variable (govet)
  - **Example:** `:=` in inner scope shadowing outer variable
  - **Expected:** Blocked
  - **Actual:** _____________________

---

## 3. Emergency Bypass Scenarios

### 3.1 --no-verify Flag

- [ ] **Test:** `git commit --no-verify -m "Emergency hotfix"` with lint issues
  - **Expected:** Commit succeeds, bypasses hook
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** `git commit --no-verify` without `-m` (opens editor)
  - **Expected:** Commit succeeds after saving message
  - **Actual:** _____________________
  - **Issues:** _____________________

### 3.2 SKIP Environment Variable

- [ ] **Test:** `SKIP=golangci-lint-fast git commit -m "Test"` with issues
  - **Expected:** Commit succeeds, skips specific hook
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** `SKIP=all git commit -m "Test"` (skip all hooks)
  - **Expected:** All hooks skipped, commit succeeds
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 4. Performance Testing

### 4.1 Small Codebase

- [ ] **Test:** Commit single Go file (~100 lines)
  - **Expected:** < 5 seconds
  - **Actual:** _____ seconds
  - **Issues:** _____________________

### 4.2 Large Commits

- [ ] **Test:** Commit 5+ Go files simultaneously
  - **Expected:** < 15 seconds (scales linearly)
  - **Actual:** _____ seconds
  - **Issues:** _____________________

- [ ] **Test:** Commit with changes to 20+ Go files
  - **Expected:** < 20 seconds (acceptable threshold)
  - **Actual:** _____ seconds
  - **Issues:** _____________________

### 4.3 Edge Case Performance

- [ ] **Test:** Commit Go file while golangci-lint is already running
  - **Expected:** Graceful handling or reasonable wait
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 5. Error Handling & Messages

### 5.1 Missing golangci-lint

- [ ] **Test:** Temporarily rename golangci-lint binary, attempt commit
  - **Expected:** Clear error message with installation instructions
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Remove `$GOPATH/bin` from PATH, attempt commit
  - **Expected:** Clear error about missing tool
  - **Actual:** _____________________
  - **Issues:** _____________________

### 5.2 Configuration Issues

- [ ] **Test:** Corrupt `.golangci-fast.yml` (invalid YAML), attempt commit
  - **Expected:** Clear error about config file
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Delete `.golangci-fast.yml`, attempt commit
  - **Expected:** Falls back to default config or clear error
  - **Actual:** _____________________
  - **Issues:** _____________________

### 5.3 Syntax Errors

- [ ] **Test:** Commit `.go` file with syntax error (won't compile)
  - **Expected:** Blocked with compilation error
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 6. Developer Workflow Integration

### 6.1 First-Time Setup

- [ ] **Test:** Fresh clone, `pre-commit install`, attempt commit with issues
  - **Expected:** Hook runs correctly on first commit
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Developer without golangci-lint installed
  - **Expected:** Clear pre-flight error with install link
  - **Actual:** _____________________
  - **Issues:** _____________________

### 6.2 Manual Testing Tools

- [ ] **Test:** `make lint-fast` command
  - **Expected:** Runs and reports same issues as pre-commit
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** `make lint-staticcheck-only` command
  - **Expected:** Runs only staticcheck, reports subset of issues
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** VS Code task "Lint: Staticcheck (Fast)"
  - **Expected:** Runs in VS Code terminal, displays issues
  - **Actual:** _____________________
  - **Issues:** _____________________

### 6.3 Iterative Development

- [ ] **Test:** Fix lint issue, save, immediately commit again
  - **Expected:** Second commit faster due to caching
  - **Actual:** _____ seconds (first), _____ seconds (second)
  - **Issues:** _____________________

- [ ] **Test:** Partial fix (fix some issues, leave others), attempt commit
  - **Expected:** Still blocked with remaining issues
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 7. Multi-Developer Scenarios

### 7.1 Git Operations

- [ ] **Test:** Pull changes with new lint issues, attempt commit unrelated file
  - **Expected:** Pre-commit only checks staged files
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Rebase interactive with lint issues in commits
  - **Expected:** Each commit checked during rebase
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Cherry-pick commit with lint issues
  - **Expected:** Cherry-pick completes, hook runs on final commit
  - **Actual:** _____________________
  - **Issues:** _____________________

### 7.2 Branch Workflows

- [ ] **Test:** Switch branches, attempt commit with different lint issues
  - **Expected:** Hook checks current branch's code
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Merge branch with lint issues, resolve conflicts, commit
  - **Expected:** Hook runs on merge commit
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 8. False Positive Handling

### 8.1 Legitimate Patterns

- [ ] **Test:** Use `//lint:ignore` comment for legitimate pattern
  - **Expected:** Staticcheck respects ignore comment
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Code that staticcheck flags but is correct
  - **Expected:** Developer can use ignore directive
  - **Actual:** _____________________
  - **Issues:** _____________________

### 8.2 Generated Code

- [ ] **Test:** Commit generated Go code (e.g., protobuf)
  - **Expected:** Excluded via `.golangci-fast.yml` or passes
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 9. Integration with Other Tools

### 9.1 Other Pre-Commit Hooks

- [ ] **Test:** Ensure trailing-whitespace hook still works
  - **Expected:** Both hooks run, both can block independently
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Ensure end-of-file-fixer hook still works
  - **Expected:** Hooks run in order, all function
  - **Actual:** _____________________
  - **Issues:** _____________________

### 9.2 VS Code Integration

- [ ] **Test:** VS Code Problems tab updates after running lint
  - **Expected:** Problems tab shows same issues as pre-commit
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** VS Code auto-format on save with lint issues
  - **Expected:** Format succeeds, lint still blocks commit
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 10. Documentation Accuracy

### 10.1 README.md

- [ ] **Test:** Follow installation instructions exactly as written
  - **Expected:** golangci-lint installs correctly
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Verify troubleshooting section accuracy
  - **Expected:** Solutions work as documented
  - **Actual:** _____________________
  - **Issues:** _____________________

### 10.2 copilot-instructions.md

- [ ] **Test:** Follow "Troubleshooting Pre-Commit Staticcheck Failures" guide
  - **Expected:** Each troubleshooting step resolves stated issue
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 11. Regression Testing

### 11.1 Existing Functionality

- [ ] **Test:** Commit non-Go files (JS, MD, etc.)
  - **Expected:** No impact from Go linter hook
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Backend build still succeeds
  - **Expected:** `go build ./...` exits 0
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Backend tests still pass
  - **Expected:** All tests pass with coverage > 85%
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## 12. CI/CD Alignment

### 12.1 Local vs CI Consistency

- [ ] **Test:** Code that passes local pre-commit
  - **Expected:** Should pass CI golangci-lint (if continue-on-error removed)
  - **Actual:** _____________________
  - **Issues:** _____________________

- [ ] **Test:** Code that fails local pre-commit
  - **Expected:** CI may still pass (continue-on-error: true)
  - **Actual:** _____________________
  - **Issues:** _____________________

---

## Summary Template

### Bugs Found

1. **Bug:** [Description]
   - **Severity:** [HIGH/MEDIUM/LOW]
   - **Impact:** [Developer workflow/correctness/performance]
   - **Reproduction:** [Steps]

### Friction Points

1. **Issue:** [Description]
   - **Impact:** [How it affects developers]
   - **Suggested Fix:** [Improvement idea]

### Documentation Gaps

1. **Gap:** [What's missing or unclear]
   - **Location:** [Which file/section]
   - **Suggested Addition:** [Content needed]

### Performance Issues

1. **Issue:** [Description]
   - **Measured:** [Actual timing]
   - **Expected:** [Target timing]
   - **Threshold Exceeded:** [YES/NO]

---

## Testing Execution Log

**Tester:** _____________________
**Date:** 2026-01-__
**Environment:** [OS, Go version, golangci-lint version]
**Duration:** _____ hours

**Overall Assessment:** [PASS/FAIL with blockers/FAIL with minor issues]

**Recommendation:** [Approve/Request changes/Block merge]

---

**End of Manual Testing Checklist**
