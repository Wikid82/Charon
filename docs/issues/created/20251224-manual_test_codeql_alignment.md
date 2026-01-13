# Manual Test Plan: CodeQL CI Alignment

**Test Date:** December 24, 2025
**Feature:** CodeQL CI/Local Execution Alignment
**Target Release:** Next release after implementation merge
**Testers:** Development team, QA, Security reviewers

## Test Objective

Validate that local CodeQL scans match CI execution and that developers can catch security issues before pushing code.

---

## Prerequisites

- [ ] Implementation merged to main branch
- [ ] CodeQL CLI installed (minimum v2.17.0)
  - Check version: `codeql version`
  - Upgrade if needed: `gh codeql set-version latest`
- [ ] Pre-commit installed: `pip install pre-commit`
- [ ] Pre-commit hooks installed: `pre-commit install`
- [ ] VS Code with workspace open

---

## Test Cases

### TC1: VS Code Task Execution - Go Scan

**Objective:** Verify Go CodeQL scan runs successfully with CI-aligned parameters

**Steps:**

1. Open VS Code Command Palette (`Ctrl+Shift+P`)
2. Type "Tasks: Run Task"
3. Select `Security: CodeQL Go Scan (CI-Aligned) [~60s]`
4. Wait for completion (~60 seconds)

**Expected Results:**

- [ ] Task completes successfully (no errors)
- [ ] Output shows database creation progress
- [ ] Output shows query execution progress
- [ ] SARIF file generated: `codeql-results-go.sarif`
- [ ] Database created: `codeql-db-go/`
- [ ] Terminal output includes findings count (e.g., "79 results")
- [ ] Uses `security-and-quality` suite (visible in output)

**Pass Criteria:** All items checked ✅

---

### TC2: VS Code Task Execution - JavaScript Scan

**Objective:** Verify JavaScript/TypeScript CodeQL scan runs with CI-aligned parameters

**Steps:**

1. Open VS Code Command Palette
2. Type "Tasks: Run Task"
3. Select `Security: CodeQL JS Scan (CI-Aligned) [~90s]`
4. Wait for completion (~90 seconds)

**Expected Results:**

- [ ] Task completes successfully
- [ ] Output shows database creation for frontend source
- [ ] Output shows query execution progress (202 queries)
- [ ] SARIF file generated: `codeql-results-js.sarif`
- [ ] Database created: `codeql-db-js/`
- [ ] Terminal output includes findings count (e.g., "105 results")
- [ ] Uses `security-and-quality` suite

**Pass Criteria:** All items checked ✅

---

### TC3: VS Code Combined Task

**Objective:** Verify sequential execution of both scans

**Steps:**

1. Open VS Code Command Palette
2. Type "Tasks: Run Task"
3. Select `Security: CodeQL All (CI-Aligned)`
4. Wait for completion (~3 minutes)

**Expected Results:**

- [ ] Go scan executes first
- [ ] JavaScript scan executes second (after Go completes)
- [ ] Both SARIF files generated
- [ ] Both databases created
- [ ] No errors or failures
- [ ] Terminal shows sequential progress

**Pass Criteria:** All items checked ✅

---

### TC4: Pre-Commit Hook - Quick Security Check

**Objective:** Verify govulncheck runs on commit

**Steps:**

1. Open terminal in project root
2. Make a trivial change to any `.go` file (add comment)
3. Stage file: `git add <file>`
4. Attempt commit: `git commit -m "test: manual test"`
5. Observe pre-commit execution

**Expected Results:**

- [ ] Pre-commit hook triggers automatically
- [ ] `security-scan` stage executes
- [ ] `govulncheck` runs on backend code
- [ ] Completes in < 10 seconds
- [ ] Shows "Passed" if no vulnerabilities
- [ ] Commit succeeds if all hooks pass

**Pass Criteria:** All items checked ✅

**Note:** This is a fast check. Full CodeQL scans are manual stage (next test).

---

### TC5: Pre-Commit Hook - Manual CodeQL Scan

**Objective:** Verify manual-stage CodeQL scans work via pre-commit

**Steps:**

1. Open terminal in project root
2. Run manual stage: `pre-commit run --hook-stage manual codeql-go-scan --all-files`
3. Wait for completion (~60s)
4. Run: `pre-commit run --hook-stage manual codeql-js-scan --all-files`
5. Run: `pre-commit run --hook-stage manual codeql-check-findings --all-files`

**Expected Results:**

- [ ] `codeql-go-scan` executes successfully
- [ ] `codeql-js-scan` executes successfully
- [ ] `codeql-check-findings` checks SARIF files
- [ ] All hooks show "Passed" status
- [ ] SARIF files generated/updated
- [ ] Error-level findings reported (if any)

**Pass Criteria:** All items checked ✅

---

### TC6: Pre-Commit Hook - Severity Blocking

**Objective:** Verify that ERROR-level findings block the hook

**Steps:**

1. Temporarily introduce a known security issue (e.g., SQL injection)

   ```go
   // In any handler file, add:
   query := "SELECT * FROM users WHERE id = " + userInput
   ```

2. Run: `pre-commit run --hook-stage manual codeql-go-scan --all-files`
3. Run: `pre-commit run --hook-stage manual codeql-check-findings --all-files`
4. Observe output

**Expected Results:**

- [ ] CodeQL scan completes
- [ ] `codeql-check-findings` hook **FAILS**
- [ ] Error message shows high-severity finding
- [ ] Hook exit code is non-zero (blocks commit)
- [ ] Error includes CWE number and description

**Pass Criteria:** Hook fails as expected ✅

**Cleanup:** Remove test code before proceeding.

---

### TC7: SARIF File Validation

**Objective:** Verify SARIF files are GitHub-compatible

**Steps:**

1. Run any CodeQL scan (TC1 or TC2)
2. Open generated SARIF file in text editor
3. Validate JSON structure
4. Check for required fields

**Expected Results:**

- [ ] File is valid JSON
- [ ] Contains `$schema` property
- [ ] Contains `runs` array with results
- [ ] Each result has:
  - [ ] `ruleId` (e.g., "go/sql-injection")
  - [ ] `level` (e.g., "error", "warning")
  - [ ] `message` with description
  - [ ] `locations` with file path and line number
- [ ] Compatible with GitHub Code Scanning API

**Pass Criteria:** All items checked ✅

---

### TC8: CI Workflow Verification

**Objective:** Verify CI behavior matches local execution

**Steps:**

1. Create test branch: `git checkout -b test/codeql-alignment`
2. Make trivial change and commit
3. Push to GitHub: `git push origin test/codeql-alignment`
4. Open pull request
5. Monitor CI workflow execution
6. Review security findings in PR

**Expected Results:**

- [ ] CodeQL workflow triggers on PR
- [ ] Go and JavaScript scans execute
- [ ] Workflow uses `security-and-quality` suite
- [ ] Finding count similar to local scans
- [ ] SARIF uploaded to GitHub Security tab
- [ ] PR shows security findings (if any)
- [ ] Workflow summary shows counts and links

**Pass Criteria:** All items checked ✅

---

### TC9: Documentation Accuracy

**Objective:** Validate user-facing documentation

**Steps:**

1. Review: `docs/security/codeql-scanning.md`
2. Follow quick start instructions
3. Review: `.github/instructions/copilot-instructions.md`
4. Verify Definition of Done section

**Expected Results:**

- [ ] Quick start instructions work as documented
- [ ] Command examples are accurate
- [ ] Task names match VS Code tasks
- [ ] Pre-commit commands execute correctly
- [ ] DoD includes security scan requirements
- [ ] Links to documentation are valid

**Pass Criteria:** All items checked ✅

---

### TC10: Performance Validation

**Objective:** Verify scan execution times are reasonable

**Steps:**

1. Run Go scan via VS Code task
2. Measure execution time
3. Run JS scan via VS Code task
4. Measure execution time

**Expected Results:**

- [ ] Go scan completes in **50-70 seconds**
- [ ] JS scan completes in **80-100 seconds**
- [ ] Combined scan completes in **2.5-3.5 minutes**
- [ ] No memory exhaustion errors
- [ ] No timeout errors

**Pass Criteria:** All times within acceptable range ✅

---

## Regression Tests

### RT1: Existing Workflows Unaffected

**Objective:** Ensure other CI workflows still pass

**Steps:**

1. Run full CI suite on test branch
2. Check all workflow statuses

**Expected Results:**

- [ ] Build workflows pass
- [ ] Test workflows pass
- [ ] Lint workflows pass
- [ ] Other security scans pass (Trivy, gosec)
- [ ] Coverage requirements met

**Pass Criteria:** No regressions ✅

---

### RT2: Developer Workflow Unchanged

**Objective:** Verify normal development isn't disrupted

**Steps:**

1. Make code changes (normal development)
2. Run existing VS Code tasks (Build, Test, Lint)
3. Commit changes with pre-commit hooks
4. Push to branch

**Expected Results:**

- [ ] Existing tasks work normally
- [ ] Fast pre-commit hooks run automatically
- [ ] Manual CodeQL scans are opt-in
- [ ] No unexpected delays or errors
- [ ] Developer experience is smooth

**Pass Criteria:** No disruptions ✅

---

## Known Issues / Expected Findings

### Expected CodeQL Findings (as of test date)

Based on QA report, these findings are expected:

**Go (79 findings):**

- Email injection (CWE-640): 3 findings
- SSRF (CWE-918): 2 findings
- Log injection (CWE-117): 10 findings
- Quality issues: 64 findings (redundant code, missing checks)

**JavaScript (105 findings):**

- DOM-based XSS (CWE-079): 1 finding
- Incomplete validation (CWE-020): 4 findings
- Quality issues: 100 findings (mostly in minified dist/ bundles)

**Note:** These are not test failures. They are real findings that should be triaged and addressed in future work.

---

## Test Summary Template

**Tester Name:** _________________
**Test Date:** _________________
**Branch Tested:** _________________
**CodeQL Version:** _________________

| Test Case | Status | Notes |
|-----------|--------|-------|
| TC1: Go Scan | ☐ Pass ☐ Fail | |
| TC2: JS Scan | ☐ Pass ☐ Fail | |
| TC3: Combined | ☐ Pass ☐ Fail | |
| TC4: Quick Check | ☐ Pass ☐ Fail | |
| TC5: Manual Scan | ☐ Pass ☐ Fail | |
| TC6: Severity Block | ☐ Pass ☐ Fail | |
| TC7: SARIF Valid | ☐ Pass ☐ Fail | |
| TC8: CI Match | ☐ Pass ☐ Fail | |
| TC9: Docs Accurate | ☐ Pass ☐ Fail | |
| TC10: Performance | ☐ Pass ☐ Fail | |
| RT1: No Regressions | ☐ Pass ☐ Fail | |
| RT2: Dev Workflow | ☐ Pass ☐ Fail | |

**Overall Result:** ☐ **PASS** ☐ **FAIL**

**Blockers Found:**

- None / List blockers here

**Recommendations:**

- None / List improvements here

**Sign-Off:**

- [ ] All critical tests passed
- [ ] Documentation is accurate
- [ ] No major issues found
- [ ] Ready for production use

**Tester Signature:** _________________
**Date:** _________________

---

## Appendix: Troubleshooting

### Issue: CodeQL not found

**Solution:**

```bash
# Install/upgrade CodeQL
gh codeql set-version latest
codeql version  # Verify installation
```

### Issue: Predicate compatibility error

**Symptom:** Error about missing predicates or incompatible query packs

**Solution:**

```bash
# Upgrade CodeQL to v2.17.0 or newer
gh codeql set-version latest

# Clear cache
rm -rf ~/.codeql/

# Re-run scan
```

### Issue: Pre-commit hooks not running

**Solution:**

```bash
# Reinstall hooks
pre-commit uninstall
pre-commit install

# Verify
pre-commit run --all-files
```

### Issue: SARIF file not generated

**Solution:**

```bash
# Check permissions
ls -la codeql-*.sarif

# Check disk space
df -h

# Re-run with verbose output
codeql database analyze --verbose ...
```

---

**End of Manual Test Plan**
