# Current Specification

**Status**: 🔧 IN PROGRESS - Staticcheck Pre-Commit Integration
**Last Updated**: 2026-01-11 (Auto-generated)
**Previous Work**: Docs-to-Issues Workflow Fix Validated (PR #461 - Archived)

---

## Active Project: Staticcheck Pre-Commit Integration

**Priority:** 🟡 MEDIUM - Code Quality Improvement
**Reported:** User experiencing staticcheck errors in VS Code Problems tab
**Objective:** Integrate staticcheck into pre-commit hooks to catch issues before commit

### Problem Statement

Staticcheck errors are appearing in VS Code's Problems tab but are not being caught by pre-commit hooks. This allows code with static analysis issues to be committed, reducing code quality and causing CI failures or technical debt accumulation.

**Current Gaps:**
- ✅ Staticcheck IS enabled in golangci-lint (`.golangci.yml` line 14)
- ✅ Staticcheck IS running in CI via golangci-lint-action (`quality-checks.yml` line 65-70)
- ❌ Staticcheck is NOT running in local pre-commit hooks
- ❌ golangci-lint pre-commit hook is in `manual` stage only (`.pre-commit-config.yaml` line 72)
- ❌ No standalone staticcheck pre-commit hook exists

**Why This Matters:**
- Developers see staticcheck warnings/errors in VS Code editor
- These issues are NOT blocked at commit time
- Creates inconsistent developer experience
- Delays feedback loop (errors found in CI instead of locally)
- Increases cognitive load (developers must remember to check manually)

---

### Current State Assessment

#### Pre-Commit Hooks Analysis

**File:** `.pre-commit-config.yaml`

**Existing Go Linting Hooks:**

1. **go-vet** (Lines 39-44) - ✅ **ACTIVE** (runs on every commit)
   - Runs on every commit for `.go` files
   - Fast (< 5 seconds)
   - Catches basic Go issues

2. **golangci-lint** (Lines 72-78) - ❌ **MANUAL ONLY**
   - **Includes staticcheck** (per `.golangci.yml`)
   - Only runs with: `pre-commit run golangci-lint --all-files`
   - Slow (30-60 seconds) - reason for manual stage
   - Runs in Docker container

#### GolangCI-Lint Configuration

**File:** `backend/.golangci.yml`

**Staticcheck Status:**
- ✅ Line 14: `- staticcheck` (enabled in linters.enable)
- ✅ Lines 68-70: Test file exclusions (staticcheck excluded from `_test.go`)

**Other Enabled Linters:**
- bodyclose, gocritic, gosec, govet, ineffassign, unused, errcheck

#### VS Code Tasks

**File:** `.vscode/tasks.json`

**Existing Lint Tasks:**
- Line 204: "Lint: Go Vet" → `cd backend && go vet ./...`
- Line 216: "Lint: GolangCI-Lint (Docker)" → Runs golangci-lint in Docker

**Missing:**
- ❌ No standalone "Lint: Staticcheck" task
- ❌ No quick lint task for just staticcheck

#### Makefile Targets

**File:** `Makefile`

**Existing Quality Targets:**
- Line 138: `lint-backend` → Runs golangci-lint in Docker
- Line 144: `test-race` → Go tests with race detection

**Missing:**
- ❌ No `staticcheck` target
- ❌ No quick `lint-quick` target

#### CI/CD Integration

**File:** `.github/workflows/quality-checks.yml`

**Lines 65-70:**
- Runs golangci-lint (includes staticcheck) in CI
- ⚠️ `continue-on-error: true` means failures don't block merges
- Uses golangci-lint-action (official GitHub Action)

#### System Environment

**Staticcheck Installation:**
```bash
$ which staticcheck
# (empty - not installed)
```

**Conclusion:** Staticcheck is NOT installed as a standalone binary on the development system.

---

### Root Cause Analysis

**Primary Cause:**
Staticcheck is integrated into the project via golangci-lint but:
1. golangci-lint pre-commit hook is in `manual` stage due to performance (30-60s)
2. No lightweight, fast-only-staticcheck pre-commit hook exists
3. Developers see staticcheck errors in VS Code but can commit without fixing them

**Contributing Factors:**
1. **Performance Trade-off:** golangci-lint is comprehensive but slow (runs 8+ linters)
2. **Manual Stage Rationale:** Documented in `.pre-commit-config.yaml` line 67
3. **Missing Standalone Tool:** Staticcheck binary not installed, only available via golangci-lint
4. **Inconsistent Enforcement:** CI runs golangci-lint with `continue-on-error: true`

---

### Recommended Solution: Standalone Staticcheck Pre-Commit Hook

**Justification:**
1. **Performance:** Fast enough for pre-commit (5-10s vs 30-60s for full golangci-lint)
2. **Developer Experience:** Aligns with VS Code feedback loop
3. **Low Risk:** Minimal changes, existing workflows unchanged
4. **Standard Practice:** Many Go projects use standalone staticcheck pre-commit

---

### Implementation Plan

#### Phase 1: Staticcheck Installation & Pre-Commit Hook

**Task 1.1: Add staticcheck installation to documentation**

**File:** `README.md`
**Location:** Development Setup section

**Addition:**
```markdown
### Go Development Tools

Install required Go development tools:

\`\`\`bash
# Required for pre-commit hooks
go install honnef.co/go/tools/cmd/staticcheck@latest
\`\`\`

Ensure `$GOPATH/bin` is in your `PATH`:
\`\`\`bash
export PATH="$PATH:$(go env GOPATH)/bin"
\`\`\`
```

**Task 1.2: Add staticcheck pre-commit hook**

**File:** `.pre-commit-config.yaml`
**Location:** After `go-vet` hook (after line 44)

```yaml
      - id: staticcheck
        name: Staticcheck (Go Static Analysis)
        entry: bash -c 'cd backend && staticcheck ./...'
        language: system
        files: '\.go$'
        pass_filenames: false
```

---

#### Phase 2: Developer Tooling

**Task 2.1: Add VS Code task**

**File:** `.vscode/tasks.json`
**Location:** After "Lint: Go Vet" task (after line 211)

```json
        {
            "label": "Lint: Staticcheck",
            "type": "shell",
            "command": "cd backend && staticcheck ./...",
            "group": "test",
            "problemMatcher": ["$go"]
        },
```

**Task 2.2: Add Makefile targets**

**File:** `Makefile`
**Location:** After `lint-backend` target (after line 141)

```makefile

staticcheck:
	@echo "Running staticcheck..."
	cd backend && staticcheck ./...

lint-quick: staticcheck
	@echo "Quick lint complete"
	cd backend && go vet ./...
```

---

#### Phase 3: Definition of Done Updates

**Task 3.1: Update checklist**

**File:** `.github/instructions/copilot-instructions.md`
**Location:** After line 91 (Pre-Commit Triage section)

```markdown
3. **Staticcheck Validation**: Run VS Code task "Lint: Staticcheck" or execute `cd backend && staticcheck ./...`.
    - Fix all staticcheck errors immediately.
    - Staticcheck warnings should be evaluated and triaged.
    - This check runs automatically in pre-commit.
    - Why: Staticcheck catches subtle bugs and style issues that go vet misses.
```

**Task 3.2: Document in Backend Workflow**

**File:** `.github/instructions/copilot-instructions.md`
**Location:** After line 36 (Backend Workflow section)

```markdown
- **Static Analysis**: Staticcheck runs automatically in pre-commit hooks.
  - Run manually: `staticcheck ./...` or VS Code task "Lint: Staticcheck"
  - Staticcheck errors MUST be fixed; warnings should be triaged.
```

---

#### Phase 4: Testing & Validation

**Task 4.1: Validate staticcheck installation**

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
which staticcheck
staticcheck --version
```

**Task 4.2: Test pre-commit hook**

```bash
pre-commit run staticcheck --all-files
```

**Task 4.3: Test VS Code task**

1. Open Command Palette
2. Run Task → "Lint: Staticcheck"
3. Verify errors in Problems tab

**Task 4.4: Test Makefile targets**

```bash
make staticcheck
make lint-quick
```

---

#### Phase 5: Documentation & Communication

**Task 5.1: Update CHANGELOG.md**

```markdown
### Added
- Pre-commit hook for staticcheck (Go static analysis)
- VS Code task "Lint: Staticcheck"
- Makefile targets: `staticcheck` and `lint-quick`

### Changed
- Dev setup now requires staticcheck installation
```

**Task 5.2: Create implementation summary**

**File:** `docs/implementation/STATICCHECK_PRE_COMMIT_INTEGRATION.md`

**Task 5.3: Archive this plan**

Move to `docs/plans/archive/staticcheck_integration_2026-01-11.md`

---

### Success Criteria

1. ✅ **Pre-Commit Hook:**
   - Staticcheck hook added to `.pre-commit-config.yaml`
   - Runs automatically on `.go` file commits
   - Fails on staticcheck errors
   - Runtime < 15 seconds

2. ✅ **Developer Tooling:**
   - VS Code task "Lint: Staticcheck" created
   - Makefile targets added

3. ✅ **Documentation:**
   - Installation instructions added
   - Definition of Done updated
   - CHANGELOG.md updated
   - Implementation summary created

4. ✅ **Validation:**
   - All tools tested and verified
   - Performance benchmarked

5. ✅ **Quality Checks:**
   - Pre-commit passes
   - CodeQL scans pass
   - Tests pass with coverage
   - Build succeeds

---

### File Reference Summary

**Files to Modify:**
1. `.pre-commit-config.yaml` (line 44: add staticcheck hook)
2. `.vscode/tasks.json` (line 211: add Staticcheck task)
3. `Makefile` (line 141: add staticcheck targets)
4. `.github/instructions/copilot-instructions.md` (lines 36, 91: add to DoD)
5. `README.md` (add installation instructions)
6. `CHANGELOG.md` (add unreleased changes)

**Files to Create:**
1. `docs/implementation/STATICCHECK_PRE_COMMIT_INTEGRATION.md`
2. `docs/plans/archive/staticcheck_integration_2026-01-11.md`

---

### Performance Benchmarks

**Target:**
- Current pre-commit: ~5-10 seconds
- After staticcheck: ~10-20 seconds
- Maximum acceptable: 25 seconds

---

### Rollback Plan

If problems occur:
1. Delete staticcheck hook from `.pre-commit-config.yaml`
2. Run `pre-commit clean`
3. Revert documentation changes
4. Rollback time: < 5 minutes

---

### Risk Assessment

**Overall Risk Level:** 🟢 LOW

1. **Performance Impact** (LOW) - Benchmark before/after
2. **Installation Friction** (LOW) - Clear docs provided
3. **False Positives** (MEDIUM) - Use lint:ignore comments if needed
4. **CI Misalignment** (LOW) - Document version pinning

---

## Phase Breakdown Timeline

| Phase | Time | Dependencies |
|-------|------|--------------|
| Phase 1 | 30 min | None |
| Phase 2 | 30 min | Phase 1 |
| Phase 3 | 30 min | Phase 1 |
| Phase 4 | 45 min | Phases 1-3 |
| Phase 5 | 30 min | Phase 4 |

**Total Estimated Time:** 2-3 hours

---

## Archive Location

Previous specifications in: [docs/plans/archive/](archive/)

---

**Note**: This specification follows [Spec-Driven Workflow v1](.github/instructions/spec-driven-workflow-v1.instructions.md) format.
