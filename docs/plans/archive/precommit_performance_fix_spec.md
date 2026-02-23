# Pre-commit Performance Fix Specification

**Status**: Draft
**Created**: 2025-12-17
**Purpose**: Move slow coverage tests to manual stage while ensuring they remain mandatory in Definition of Done

---

## 📋 Problem Statement

The current pre-commit configuration runs slow hooks (`go-test-coverage` and `frontend-type-check`) on every commit, causing developer friction. These hooks can take 30+ seconds each, blocking rapid iteration.

However, coverage testing is critical and must remain mandatory before task completion. The solution is to:

1. Move slow hooks to manual stage for developer convenience
2. Make coverage testing an explicit requirement in Definition of Done
3. Ensure all agent modes verify coverage tests pass before completing tasks
4. Maintain CI coverage enforcement

---

## 🎯 Goals

1. **Developer Experience**: Pre-commit runs in <5 seconds for typical commits
2. **Quality Assurance**: Coverage tests remain mandatory via VS Code tasks/scripts
3. **CI Integrity**: GitHub Actions continue to enforce coverage requirements
4. **Agent Compliance**: All agent modes verify coverage before marking tasks complete

---

## 📐 Phase 1: Pre-commit Configuration Changes

### File: `.pre-commit-config.yaml`

#### Change 1.1: Move `go-test-coverage` to Manual Stage

**Current Configuration (Lines 20-26)**:

```yaml
      - id: go-test-coverage
        name: Go Test Coverage
        entry: scripts/go-test-coverage.sh
        language: script
        files: '\.go$'
        pass_filenames: false
        verbose: true
```

**New Configuration**:

```yaml
      - id: go-test-coverage
        name: Go Test Coverage (Manual)
        entry: scripts/go-test-coverage.sh
        language: script
        files: '\.go$'
        pass_filenames: false
        verbose: true
        stages: [manual]  # Only runs when explicitly called
```

**Rationale**: This hook takes 15-30 seconds. Moving to manual stage improves developer experience while maintaining availability via `pre-commit run go-test-coverage --all-files` or VS Code tasks.

---

#### Change 1.2: Move `frontend-type-check` to Manual Stage

**Current Configuration (Lines 87-91)**:

```yaml
      - id: frontend-type-check
        name: Frontend TypeScript Check
        entry: bash -c 'cd frontend && npm run type-check'
        language: system
        files: '^frontend/.*\.(ts|tsx)$'
        pass_filenames: false
```

**New Configuration**:

```yaml
      - id: frontend-type-check
        name: Frontend TypeScript Check (Manual)
        entry: bash -c 'cd frontend && npm run type-check'
        language: system
        files: '^frontend/.*\.(ts|tsx)$'
        pass_filenames: false
        stages: [manual]  # Only runs when explicitly called
```

**Rationale**: TypeScript checking can take 10-20 seconds on large codebases. The frontend linter (`frontend-lint`) still runs automatically and catches most issues.

---

#### Summary of Pre-commit Changes

**Hooks Moved to Manual**:

- `go-test-coverage` (already manual: ❌)
- `frontend-type-check` (currently auto: ✅)

**Hooks Remaining in Manual** (No changes):

- `go-test-race` (already manual)
- `golangci-lint` (already manual)
- `hadolint` (already manual)
- `frontend-test-coverage` (already manual)
- `security-scan` (already manual)
- `markdownlint` (already manual)

**Hooks Remaining Auto** (Fast execution):

- `end-of-file-fixer`
- `trailing-whitespace`
- `check-yaml`
- `check-added-large-files`
- `dockerfile-check`
- `go-vet`
- `check-version-match`
- `check-lfs-large-files`
- `block-codeql-db-commits`
- `block-data-backups-commit`
- `frontend-lint` (with `--fix`)

---

## 📝 Phase 2: Copilot Instructions Updates

### File: `.github/copilot-instructions.md`

#### Change 2.1: Expand Definition of Done Section

**Current Section (Lines 108-116)**:

```markdown
## ✅ Task Completion Protocol (Definition of Done)

Before marking an implementation task as complete, perform the following:

1. **Pre-Commit Triage**: Run `pre-commit run --all-files`.
    - If errors occur, **fix them immediately**.
    - If logic errors occur, analyze and propose a fix.
    - Do not output code that violates pre-commit standards.
2. **Verify Build**: Ensure the backend compiles and the frontend builds without errors.
3. **Clean Up**: Ensure no debug print statements or commented-out blocks remain.
```

**New Section**:

```markdown
## ✅ Task Completion Protocol (Definition of Done)

Before marking an implementation task as complete, perform the following in order:

1. **Pre-Commit Triage**: Run `pre-commit run --all-files`.
    - If errors occur, **fix them immediately**.
    - If logic errors occur, analyze and propose a fix.
    - Do not output code that violates pre-commit standards.

2. **Coverage Testing** (MANDATORY - Non-negotiable):
    - **Backend Changes**: Run the VS Code task "Test: Backend with Coverage" or execute `scripts/go-test-coverage.sh`.
        - Minimum coverage: 85% (set via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`).
        - If coverage drops below threshold, write additional tests to restore coverage.
        - All tests must pass with zero failures.
    - **Frontend Changes**: Run the VS Code task "Test: Frontend with Coverage" or execute `scripts/frontend-test-coverage.sh`.
        - Minimum coverage: 85% (set via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`).
        - If coverage drops below threshold, write additional tests to restore coverage.
        - All tests must pass with zero failures.
    - **Critical**: Coverage tests are NOT run by default pre-commit hooks (they are in manual stage for performance). You MUST run them explicitly via VS Code tasks or scripts before completing any task.
    - **Why**: CI enforces coverage in GitHub Actions. Local verification prevents CI failures and maintains code quality.

3. **Type Safety** (Frontend only):
    - Run the VS Code task "Lint: TypeScript Check" or execute `cd frontend && npm run type-check`.
    - Fix all type errors immediately. This is non-negotiable.
    - This check is also in manual stage for performance but MUST be run before completion.

4. **Verify Build**: Ensure the backend compiles and the frontend builds without errors.
    - Backend: `cd backend && go build ./...`
    - Frontend: `cd frontend && npm run build`

5. **Clean Up**: Ensure no debug print statements or commented-out blocks remain.
    - Remove `console.log`, `fmt.Println`, and similar debugging statements.
    - Delete commented-out code blocks.
    - Remove unused imports.
```

**Rationale**: Makes coverage testing and type checking explicit requirements. The current Definition of Done doesn't mention coverage testing, leading to CI failures when developers skip it.

---

## 🤖 Phase 3: Agent Mode Files Updates

### Overview

All agent mode files need explicit instructions to run coverage tests before completing tasks. The current agent files have varying levels of coverage enforcement:

- **Backend_Dev**: Has coverage requirement but not explicit about manual hook
- **Frontend_Dev**: Has coverage requirement but not explicit about manual hook
- **QA_Security**: Has coverage requirement in Definition of Done
- **Management**: Has Definition of Done but delegates to other agents
- **Planning**: No coverage requirements (documentation only)
- **DevOps**: No coverage requirements (infrastructure only)

---

### File: `.github/agents/Backend_Dev.agent.md`

#### Change 3.1: Update Verification Section

**Current Section (Lines 32-36)**:

```markdown
3. **Verification (Definition of Done)**:
    - Run `go mod tidy`.
    - Run `go fmt ./...`.
    - Run `go test ./...` to ensure no regressions.
    - **Coverage**: Run the coverage script.
        - *Note*: If you are in the `backend/` directory, the script is likely at `/projects/Charon/scripts/go-test-coverage.sh`. Verify location before running.
    - Ensure coverage goals are met as well as all tests pass. Just because Tests pass does not mean you are done. Goal Coverage Needs to be met even if the tests to get us there are outside the scope of your task. At this point, your task is to maintain coverage goal and all tests pass because we cannot commit changes if they fail.
```

**New Section**:

```markdown
3. **Verification (Definition of Done)**:
    - Run `go mod tidy`.
    - Run `go fmt ./...`.
    - Run `go test ./...` to ensure no regressions.
    - **Coverage (MANDATORY)**: Run the coverage script explicitly. This is NOT run by pre-commit automatically.
        - **VS Code Task**: Use "Test: Backend with Coverage" (recommended)
        - **Manual Script**: Execute `/projects/Charon/scripts/go-test-coverage.sh` from the root directory
        - **Minimum**: 85% coverage (configured via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`)
        - **Critical**: If coverage drops below threshold, write additional tests immediately. Do not skip this step.
        - **Why**: Coverage tests are in manual stage of pre-commit for performance. You MUST run them via VS Code tasks or scripts before completing your task.
    - Ensure coverage goals are met as well as all tests pass. Just because Tests pass does not mean you are done. Goal Coverage Needs to be met even if the tests to get us there are outside the scope of your task. At this point, your task is to maintain coverage goal and all tests pass because we cannot commit changes if they fail.
    - Run `pre-commit run --all-files` as final check (this runs fast hooks only; coverage was verified above).
```

---

### File: `.github/agents/Frontend_Dev.agent.md`

#### Change 3.2: Update Verification Section

**Current Section (Lines 28-36)**:

```markdown
3. **Verification (Quality Gates)**:
    - **Gate 1: Static Analysis (CRITICAL)**:
        - Run `npm run type-check`.
        - Run `npm run lint`.
        - **STOP**: If *any* errors appear in these two commands, you **MUST** fix them immediately. Do not say "I'll leave this for later." **Fix the type errors, then re-run the check.**
    - **Gate 2: Logic**:
        - Run `npm run test:ci`.
    - **Gate 3: Coverage**:
        - Run `npm run check-coverage`.
        - Ensure the script executes successfully and coverage goals are met.
        - Ensure coverage goals are met as well as all tests pass. Just because Tests pass does not mean you are done. Goal Coverage Needs to be met even if the tests to get us there are outside the scope of your task. At this point, your task is to maintain coverage goal and all tests pass because we cannot commit changes if they fail.
```

**New Section**:

```markdown
3. **Verification (Quality Gates)**:
    - **Gate 1: Static Analysis (CRITICAL)**:
        - **Type Check (MANDATORY)**: Run the VS Code task "Lint: TypeScript Check" or execute `npm run type-check`.
            - **Why**: This check is in manual stage of pre-commit for performance. You MUST run it explicitly before completing your task.
            - **STOP**: If *any* errors appear, you **MUST** fix them immediately. Do not say "I'll leave this for later."
        - **Lint**: Run `npm run lint`.
            - This runs automatically in pre-commit, but verify locally before final submission.
    - **Gate 2: Logic**:
        - Run `npm run test:ci`.
    - **Gate 3: Coverage (MANDATORY)**:
        - **VS Code Task**: Use "Test: Frontend with Coverage" (recommended)
        - **Manual Script**: Execute `/projects/Charon/scripts/frontend-test-coverage.sh` from the root directory
        - **Minimum**: 85% coverage (configured via `CHARON_MIN_COVERAGE` or `CPM_MIN_COVERAGE`)
        - **Critical**: If coverage drops below threshold, write additional tests immediately. Do not skip this step.
        - **Why**: Coverage tests are in manual stage of pre-commit for performance. You MUST run them via VS Code tasks or scripts before completing your task.
        - Ensure coverage goals are met as well as all tests pass. Just because Tests pass does not mean you are done. Goal Coverage Needs to be met even if the tests to get us there are outside the scope of your task. At this point, your task is to maintain coverage goal and all tests pass because we cannot commit changes if they fail.
    - **Gate 4: Pre-commit**:
        - Run `pre-commit run --all-files` as final check (this runs fast hooks only; coverage and type-check were verified above).
```

---

### File: `.github/agents/QA_Security.agent.md`

#### Change 3.3: Update Definition of Done Section

**Current Section (Lines 45-47)**:

```markdown
## DEFENITION OF DONE ##

- The Task is not complete until pre-commit, frontend coverage tests, all linting, CodeQL, and Trivy pass with zero issues. Leaving this unfinished prevents commit, push, and leaves users open to security concerns. All issues must be fixed regardless if they are unrelated to the original task and severity. This rule must never be skipped. It is non-negotiable anytime any bit of code is added or changed.
```

**New Section**:

```markdown
## DEFINITION OF DONE ##

The task is not complete until ALL of the following pass with zero issues:

1. **Coverage Tests (MANDATORY - Run Explicitly)**:
    - **Backend**: Run VS Code task "Test: Backend with Coverage" or execute `scripts/go-test-coverage.sh`
    - **Frontend**: Run VS Code task "Test: Frontend with Coverage" or execute `scripts/frontend-test-coverage.sh`
    - **Why**: These are in manual stage of pre-commit for performance. You MUST run them via VS Code tasks or scripts.
    - Minimum coverage: 85% for both backend and frontend.
    - All tests must pass with zero failures.

2. **Type Safety (Frontend)**:
    - Run VS Code task "Lint: TypeScript Check" or execute `cd frontend && npm run type-check`
    - **Why**: This check is in manual stage of pre-commit for performance. You MUST run it explicitly.
    - Fix all type errors immediately.

3. **Pre-commit Hooks**: Run `pre-commit run --all-files` (this runs fast hooks only; coverage was verified in step 1)

4. **Security Scans**:
    - CodeQL: Run as VS Code task or via GitHub Actions
    - Trivy: Run as VS Code task or via Docker
    - Zero Critical or High severity issues allowed

5. **Linting**: All language-specific linters must pass (Go vet, ESLint, markdownlint)

**Critical Note**: Leaving this unfinished prevents commit, push, and leaves users open to security concerns. All issues must be fixed regardless of whether they are unrelated to the original task. This rule must never be skipped. It is non-negotiable anytime any bit of code is added or changed.
```

**Additional**: Fix typo "DEFENITION" → "DEFINITION"

---

### File: `.github/agents/Manegment.agent.md`

#### Change 3.4: Update Definition of Done Section

**Current Section (Lines 57-59)**:

```markdown
## DEFENITION OF DONE ##

- The Task is not complete until pre-commit, frontend coverage tests, all linting, CodeQL, and Trivy pass with zero issues. Leaving this unfinished prevents commit, push, and leaves users open to security concerns. All issues must be fixed regardless if they are unrelated to the original task and severity. This rule must never be skipped. It is non-negotiable anytime any bit of code is added or changed.
```

**New Section**:

```markdown
## DEFINITION OF DONE ##

The task is not complete until ALL of the following pass with zero issues:

1. **Coverage Tests (MANDATORY - Verify Explicitly)**:
    - **Backend**: Ensure `Backend_Dev` ran VS Code task "Test: Backend with Coverage" or `scripts/go-test-coverage.sh`
    - **Frontend**: Ensure `Frontend_Dev` ran VS Code task "Test: Frontend with Coverage" or `scripts/frontend-test-coverage.sh`
    - **Why**: These are in manual stage of pre-commit for performance. Subagents MUST run them via VS Code tasks or scripts.
    - Minimum coverage: 85% for both backend and frontend.
    - All tests must pass with zero failures.

2. **Type Safety (Frontend)**:
    - Ensure `Frontend_Dev` ran VS Code task "Lint: TypeScript Check" or `npm run type-check`
    - **Why**: This check is in manual stage of pre-commit for performance. Subagents MUST run it explicitly.

3. **Pre-commit Hooks**: Ensure `QA_Security` ran `pre-commit run --all-files` (fast hooks only; coverage was verified in step 1)

4. **Security Scans**: Ensure `QA_Security` ran CodeQL and Trivy with zero Critical or High severity issues

5. **Linting**: All language-specific linters must pass

**Your Role**: You delegate implementation to subagents, but YOU are responsible for verifying they completed the Definition of Done. Do not accept "DONE" from a subagent until you have confirmed they ran coverage tests and type checks explicitly.

**Critical Note**: Leaving this unfinished prevents commit, push, and leaves users open to security concerns. All issues must be fixed regardless of whether they are unrelated to the original task. This rule must never be skipped. It is non-negotiable anytime any bit of code is added or changed.
```

**Additional**: Fix typo "DEFENITION" → "DEFINITION" and filename typo "Manegment" → "Management" (requires file rename)

---

### File: `.github/agents/DevOps.agent.md`

#### Change 3.5: Add Coverage Awareness Section

**Location**: After the `<workflow>` section, before `<output_format>` (around line 35)

**New Section**:

```markdown

<coverage_and_ci>
**Coverage Tests in CI**: GitHub Actions workflows run coverage tests automatically:
- `.github/workflows/codecov-upload.yml`: Uploads coverage to Codecov
- `.github/workflows/quality-checks.yml`: Enforces coverage thresholds

**Your Role as DevOps**:
- You do NOT write coverage tests (that's `Backend_Dev` and `Frontend_Dev`).
- You DO ensure CI workflows run coverage scripts correctly.
- You DO verify that coverage thresholds match local requirements (85% by default).
- If CI coverage fails but local tests pass, check for:
    1. Different `CHARON_MIN_COVERAGE` values between local and CI
    2. Missing test files in CI (check `.gitignore`, `.dockerignore`)
    3. Race condition timeouts (check `PERF_MAX_MS_*` environment variables)
</coverage_and_ci>
```

**Rationale**: DevOps agent needs context about coverage testing in CI to debug workflow failures effectively.

---

### File: `.github/agents/Planning.agent.md`

#### Change 3.6: Add Coverage Requirements to Output Format

**Current Output Format (Lines 36-67)** - Add coverage requirements to Phase 3 checklist.

**Modified Section (Phase 3 in output format)**:

```markdown
### 🕵️ Phase 3: QA & Security

  1. Edge Cases: {List specific scenarios to test}
  2. **Coverage Tests (MANDATORY)**:
     - Backend: Run VS Code task "Test: Backend with Coverage" or execute `scripts/go-test-coverage.sh`
     - Frontend: Run VS Code task "Test: Frontend with Coverage" or execute `scripts/frontend-test-coverage.sh`
     - Minimum coverage: 85% for both backend and frontend
     - **Critical**: These are in manual stage of pre-commit for performance. Agents MUST run them via VS Code tasks or scripts before marking tasks complete.
  3. Security: Run CodeQL and Trivy scans. Triage and fix any new errors or warnings.
  4. **Type Safety (Frontend)**: Run VS Code task "Lint: TypeScript Check" or execute `cd frontend && npm run type-check`
  5. Linting: Run `pre-commit` hooks on all files and triage anything not auto-fixed.
```

**Rationale**: Planning agent creates task specifications. Including coverage requirements ensures downstream agents have clear expectations.

---

## 🧪 Phase 4: Testing & Verification

### 4.1 Local Testing

**Step 1: Verify Pre-commit Performance**

```bash
# Time the pre-commit run (should be <5 seconds)
time pre-commit run --all-files

# Expected: Only fast hooks run (go-vet, frontend-lint, trailing-whitespace, etc.)
# NOT expected: go-test-coverage, frontend-type-check (these are manual)
```

**Step 2: Verify Manual Hooks Still Work**

```bash
# Test manual hook invocation
pre-commit run go-test-coverage --all-files
pre-commit run frontend-type-check --all-files

# Expected: Both hooks execute successfully
```

**Step 3: Verify VS Code Tasks**

```bash
# Open VS Code Command Palette (Ctrl+Shift+P)
# Run: "Tasks: Run Task"
# Select: "Test: Backend with Coverage"
# Expected: Coverage tests run and pass (85%+)

# Run: "Tasks: Run Task"
# Select: "Test: Frontend with Coverage"
# Expected: Coverage tests run and pass (85%+)

# Run: "Tasks: Run Task"
# Select: "Lint: TypeScript Check"
# Expected: Type checking completes with zero errors
```

**Step 4: Verify Coverage Script Directly**

```bash
# From project root
bash scripts/go-test-coverage.sh
# Expected: Coverage ≥85%, all tests pass

bash scripts/frontend-test-coverage.sh
# Expected: Coverage ≥85%, all tests pass
```

---

### 4.2 CI Testing

**Step 1: Verify GitHub Actions Workflows**

Check that coverage tests still run in CI:

```yaml
# .github/workflows/codecov-upload.yml (Lines 29-34, 65-68)
# Verify these lines still call coverage scripts:
- name: Run Go tests with coverage
  run: |
    bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt

- name: Run frontend tests and coverage
  run: |
    bash scripts/frontend-test-coverage.sh 2>&1 | tee frontend/test-output.txt
```

```yaml
# .github/workflows/quality-checks.yml (Lines 32, 134-139)
# Verify these lines still call coverage scripts:
- name: Run Go tests with coverage
  run: |
    bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt

- name: Run frontend tests and coverage
  run: |
    bash scripts/frontend-test-coverage.sh 2>&1 | tee frontend/test-output.txt
```

**Step 2: Push Test Commit**

```bash
# Make a trivial change to trigger CI
echo "# Test commit for coverage CI verification" >> README.md
git add README.md
git commit -m "chore: test coverage CI verification"
git push
```

**Step 3: Verify CI Runs**

- Navigate to GitHub Actions
- Verify workflows `codecov-upload` and `quality-checks` run successfully
- Verify coverage tests execute and pass
- Verify coverage reports upload to Codecov (if configured)

---

### 4.3 Agent Mode Testing

**Step 1: Test Backend_Dev Agent**

```
# In Copilot chat, invoke:
@Backend_Dev Implement a simple test function that adds two numbers in internal/utils

# Expected behavior:
1. Agent writes the function
2. Agent writes unit tests
3. Agent runs go test ./...
4. Agent explicitly runs VS Code task "Test: Backend with Coverage" or scripts/go-test-coverage.sh
5. Agent confirms coverage ≥85%
6. Agent marks task complete
```

**Step 2: Test Frontend_Dev Agent**

```
# In Copilot chat, invoke:
@Frontend_Dev Create a simple Button component in src/components/TestButton.tsx

# Expected behavior:
1. Agent writes the component
2. Agent writes unit tests
3. Agent runs npm run test:ci
4. Agent explicitly runs VS Code task "Test: Frontend with Coverage" or scripts/frontend-test-coverage.sh
5. Agent explicitly runs VS Code task "Lint: TypeScript Check" or npm run type-check
6. Agent confirms coverage ≥85% and zero type errors
7. Agent marks task complete
```

**Step 3: Test QA_Security Agent**

```
# In Copilot chat, invoke:
@QA_Security Audit the current codebase for Definition of Done compliance

# Expected behavior:
1. Agent runs pre-commit run --all-files
2. Agent explicitly runs coverage tests for backend and frontend
3. Agent explicitly runs TypeScript type check
4. Agent runs CodeQL and Trivy scans
5. Agent reports any issues found
6. Agent confirms all checks pass before marking complete
```

**Step 4: Test Management Agent**

```
# In Copilot chat, invoke:
@Management Implement a simple feature: Add a /health endpoint to the backend

# Expected behavior:
1. Agent delegates to Planning for spec
2. Agent delegates to Backend_Dev for implementation
3. Agent delegates to QA_Security for verification
4. Agent verifies QA_Security ran coverage tests explicitly
5. Agent confirms Definition of Done met before marking complete
```

---

## 📊 Phase 5: Rollback Plan

If issues arise after implementing these changes, follow this rollback procedure:

### Rollback Step 1: Revert Pre-commit Changes

```bash
# Restore original .pre-commit-config.yaml from git history
git checkout HEAD~1 -- .pre-commit-config.yaml

# Or manually remove "stages: [manual]" from:
# - go-test-coverage
# - frontend-type-check
```

### Rollback Step 2: Revert Copilot Instructions

```bash
# Restore original .github/copilot-instructions.md
git checkout HEAD~1 -- .github/copilot-instructions.md
```

### Rollback Step 3: Revert Agent Mode Files

```bash
# Restore all agent mode files
git checkout HEAD~1 -- .github/agents/Backend_Dev.agent.md
git checkout HEAD~1 -- .github/agents/Frontend_Dev.agent.md
git checkout HEAD~1 -- .github/agents/QA_Security.agent.md
git checkout HEAD~1 -- .github/agents/Manegment.agent.md
git checkout HEAD~1 -- .github/agents/DevOps.agent.md
git checkout HEAD~1 -- .github/agents/Planning.agent.md
```

### Rollback Step 4: Verify Rollback

```bash
# Verify pre-commit runs slow hooks again
pre-commit run --all-files
# Expected: go-test-coverage and frontend-type-check run automatically

# Verify CI still works
git add .
git commit -m "chore: rollback pre-commit performance changes"
git push
# Check GitHub Actions for successful workflow runs
```

---

## 📋 Implementation Checklist

Use this checklist to track implementation progress:

- [ ] **Phase 1: Pre-commit Configuration**
  - [ ] Add `stages: [manual]` to `go-test-coverage` hook
  - [ ] Change name to "Go Test Coverage (Manual)"
  - [ ] Add `stages: [manual]` to `frontend-type-check` hook
  - [ ] Change name to "Frontend TypeScript Check (Manual)"
  - [ ] Test: Run `pre-commit run --all-files` (should be fast)
  - [ ] Test: Run `pre-commit run go-test-coverage --all-files` (should execute)
  - [ ] Test: Run `pre-commit run frontend-type-check --all-files` (should execute)

- [ ] **Phase 2: Copilot Instructions**
  - [ ] Update Definition of Done section in `.github/copilot-instructions.md`
  - [ ] Add explicit coverage testing requirements (Step 2)
  - [ ] Add explicit type checking requirements (Step 3)
  - [ ] Add rationale for manual hooks
  - [ ] Test: Read through updated instructions for clarity

- [ ] **Phase 3: Agent Mode Files**
  - [ ] Update `Backend_Dev.agent.md` verification section
  - [ ] Update `Frontend_Dev.agent.md` verification section
  - [ ] Update `QA_Security.agent.md` Definition of Done
  - [ ] Fix typo: "DEFENITION" → "DEFINITION" in `QA_Security.agent.md`
  - [ ] Update `Manegment.agent.md` Definition of Done
  - [ ] Fix typo: "DEFENITION" → "DEFINITION" in `Manegment.agent.md`
  - [ ] Consider renaming `Manegment.agent.md` → `Management.agent.md`
  - [ ] Add coverage awareness section to `DevOps.agent.md`
  - [ ] Update `Planning.agent.md` output format (Phase 3 checklist)
  - [ ] Test: Review all agent mode files for consistency

- [ ] **Phase 4: Testing & Verification**
  - [ ] Test pre-commit performance (should be <5 seconds)
  - [ ] Test manual hook invocation (should work)
  - [ ] Test VS Code tasks for coverage (should work)
  - [ ] Test coverage scripts directly (should work)
  - [ ] Verify CI workflows still run coverage tests
  - [ ] Push test commit to verify CI passes
  - [ ] Test Backend_Dev agent behavior
  - [ ] Test Frontend_Dev agent behavior
  - [ ] Test QA_Security agent behavior
  - [ ] Test Management agent behavior

- [ ] **Phase 5: Documentation**
  - [ ] Update `CONTRIBUTING.md` with new workflow (if exists)
  - [ ] Add note about manual hooks to developer documentation
  - [ ] Update onboarding docs to mention VS Code tasks for coverage

---

## 🚨 Critical Success Factors

1. **CI Must Pass**: GitHub Actions must continue to enforce coverage requirements
2. **Agents Must Comply**: All agent modes must explicitly run coverage tests before completion
3. **Developer Experience**: Pre-commit must run in <5 seconds for typical commits
4. **No Quality Regression**: Coverage requirements remain mandatory (85%)
5. **Clear Documentation**: Definition of Done must be explicit and unambiguous

---

## 📚 References

- **Pre-commit Documentation**: <https://pre-commit.com/#confining-hooks-to-run-at-certain-stages>
- **VS Code Tasks**: <https://code.visualstudio.com/docs/editor/tasks>
- **Current Coverage Scripts**:
  - Backend: `scripts/go-test-coverage.sh`
  - Frontend: `scripts/frontend-test-coverage.sh`
- **CI Workflows**:
  - `.github/workflows/codecov-upload.yml`
  - `.github/workflows/quality-checks.yml`

---

## 🔍 Potential Issues & Solutions

### Issue 1: Developers Forget to Run Coverage Tests

**Symptom**: CI fails with coverage errors but pre-commit passed locally

**Solution**:

- Add reminder in commit message template
- Add VS Code task to run all manual checks before push
- Update CONTRIBUTING.md with explicit workflow

**Prevention**: Clear Definition of Done in agent instructions

---

### Issue 2: VS Code Tasks Not Available

**Symptom**: Agents cannot find VS Code tasks to run

**Solution**:

- Verify `.vscode/tasks.json` exists and has correct task names
- Provide fallback to direct script execution
- Document both methods in agent instructions

**Prevention**: Test both VS Code tasks and direct script execution

---

### Issue 3: Coverage Scripts Fail in Agent Context

**Symptom**: Coverage scripts work manually but fail when invoked by agents

**Solution**:

- Ensure agents execute scripts from project root directory
- Verify environment variables are set correctly
- Add explicit directory navigation in agent instructions

**Prevention**: Test agent execution paths during verification phase

---

### Issue 4: Manual Hooks Not Running in CI

**Symptom**: CI doesn't run coverage tests after moving to manual stage

**Solution**:

- Verify CI workflows call coverage scripts directly (not via pre-commit)
- Do NOT rely on pre-commit in CI for coverage tests
- CI workflows already use direct script calls (verified in Phase 4.2)

**Prevention**: CI workflows bypass pre-commit and call scripts directly

---

## ✅ Definition of Done for This Spec

This specification is complete when:

1. [ ] All phases are documented with exact code snippets
2. [ ] All file paths and line numbers are specified
3. [ ] Testing procedures are comprehensive
4. [ ] Rollback plan is clear and actionable
5. [ ] Implementation checklist covers all changes
6. [ ] Potential issues are documented with solutions
7. [ ] Critical success factors are identified
8. [ ] References are provided for further reading

---

## 📝 Next Steps

After this spec is approved:

1. Create a branch: `fix/precommit-performance`
2. Implement Phase 1 (pre-commit config)
3. Test locally to verify performance improvement
4. Implement Phase 2 (copilot instructions)
5. Implement Phase 3 (agent mode files)
6. Execute Phase 4 testing procedures
7. Create pull request with this spec as documentation
8. Verify CI passes on PR
9. Merge after approval

---

**End of Specification**
