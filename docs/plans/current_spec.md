# Agent Skills Migration Plan - COMPLETE SPECIFICATION

**Status**: Planning Phase - READY FOR IMPLEMENTATION
**Created**: 2025-12-20
**Last Updated**: 2025-12-20
**Objective**: Migrate all task-oriented scripts from `/scripts` directory to Agent Skills following the [agentskills.io specification](https://agentskills.io/specification)

---

## Executive Summary

This plan outlines the complete migration of 29 executable scripts from `/scripts` to the Agent Skills format. The migration will transform shell scripts into structured, AI-discoverable skills that follow the agentskills.io specification. Each skill will be self-documenting with YAML frontmatter and Markdown instructions.

**Location**: Skills will be stored in `.github/skills/` as per [VS Code Copilot documentation](https://code.visualstudio.com/docs/copilot/customization/agent-skills). This is the standard location for VS Code Copilot skills, while the SKILL.md format follows the [agentskills.io specification](https://agentskills.io/specification).

**Total Scripts**: 29 identified
**Skills to Create**: 24 (5 scripts excluded as internal/utility-only)
**Phases**: 6 implementation phases (including Phase 0 and Phase 5)
**Target Completion**: 2025-12-27

### Success Criteria

1. All 24 skills are functional and pass validation tests
2. All existing tasks.json entries reference skills correctly
3. CI/CD workflows reference skills instead of scripts
4. Skills are AI-discoverable via agentskills.io tooling
5. Coverage meets 85% threshold for all test-related skills
6. Documentation is complete and accurate
7. Backward compatibility warnings are in place for 1 release cycle

---

## Directory Structure - FLAT LAYOUT

The new `.github/skills` directory will be created following VS Code Copilot standards with a **FLAT structure** (no subcategories) for maximum AI discoverability:

```

**Rationale for Flat Structure:**
- Maximum AI discoverability (no directory traversal needed)
- Simpler skill references in tasks.json and CI/CD workflows
- Easier maintenance and navigation
- Aligns with VS Code Copilot standards and agentskills.io specification examples
- Clear naming convention makes categorization implicit (prefix-based)
- Compatible with GitHub Copilot's skill discovery mechanism

**Naming Convention:**
- `{category}-{feature}-{variant}.SKILL.md`
- Categories: `test`, `integration-test`, `security`, `qa`, `build`, `utility`, `docker`
- Examples: `test-backend-coverage.SKILL.md`, `integration-test-crowdsec.SKILL.md`

### Important: Location vs. Format Specification

**Directory Location**: `.github/skills/`
- This is the **VS Code Copilot standard location** for Agent Skills
- Required for VS Code's GitHub Copilot to discover and utilize skills
- Source: [VS Code Copilot Documentation](https://code.visualstudio.com/docs/copilot/customization/agent-skills)

**File Format**: SKILL.md (agentskills.io specification)
- The **format and structure** of SKILL.md files follows [agentskills.io specification](https://agentskills.io/specification)
- agentskills.io defines the YAML frontmatter schema, markdown structure, and metadata fields
- The format is platform-agnostic and can be used in any AI-assisted development environment

**Key Distinction**:
- `.github/skills/` = **WHERE** skills are stored (VS Code Copilot location)
- agentskills.io = **HOW** skills are structured (format specification)

---

## Complete SKILL.md Template with Validated Frontmatter

Each SKILL.md file follows this structure:

```markdown
---
# agentskills.io specification v1.0
name: "skill-name-kebab-case"
version: "1.0.0"
description: "Single-line description (max 120 chars)"
author: "Charon Project"
license: "MIT"
tags:
  - "tag1"
  - "tag2"
  - "tag3"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
    - "sh"
requirements:
  - name: "go"
    version: ">=1.23"
    optional: false
  - name: "docker"
    version: ">=24.0"
    optional: false
environment_variables:
  - name: "ENV_VAR_NAME"
    description: "Description of variable"
    default: "default_value"
    required: false
parameters:
  - name: "param_name"
    type: "string"
    description: "Parameter description"
    default: "default_value"
    required: false
outputs:
  - name: "output_name"
    type: "file"
    description: "Output description"
    path: "path/to/output"
metadata:
  category: "category-name"
  subcategory: "subcategory-name"
  execution_time: "short|medium|long"
  risk_level: "low|medium|high"
  ci_cd_safe: true|false
  requires_network: true|false
  idempotent: true|false
---

# Skill Name

## Overview

Brief description of what this skill does (1-2 sentences).

## Prerequisites

- List prerequisites here
- Each on its own line

## Usage

### Basic Usage

```bash
# Command example
./path/to/script.sh
```

### Advanced Usage

```bash
# Advanced example with options
ENV_VAR=value ./path/to/script.sh --option
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| param1    | string | No     | value   | Description |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| VAR_NAME | No       | value   | Description |

## Outputs

- **Success Exit Code**: 0
- **Error Exit Codes**: Non-zero
- **Output Files**: Description of any files created

## Examples

### Example 1: Basic Execution

```bash
# Description of what this example does
command example
```

### Example 2: Advanced Execution

```bash
# Description of what this example does
command example with options
```

## Error Handling

- Common errors and solutions
- Exit codes and their meanings

## Related Skills

- [related-skill-1](./related-skill-1.SKILL.md)
- [related-skill-2](./related-skill-2.SKILL.md)

## Notes

- Additional notes
- Warnings or caveats

---

**Last Updated**: YYYY-MM-DD
**Maintained by**: Charon Project
**Source**: `scripts/original-script.sh`
```

### Frontmatter Validation Rules

1. **Required Fields**: `name`, `version`, `description`, `author`, `license`, `tags`
2. **Name Format**: Must be kebab-case (lowercase, hyphens only)
3. **Version Format**: Must follow semantic versioning (x.y.z)
4. **Description**: Max 120 characters, single line, no markdown
5. **Tags**: Minimum 2, maximum 5, lowercase
6. **Custom Metadata**: All fields under `metadata` are project-specific extensions

### Progressive Disclosure Strategy

To keep SKILL.md files under 500 lines:
1. **Basic documentation**: Frontmatter + overview + basic usage (< 100 lines)
2. **Extended documentation**: Link to separate docs/skills/{name}.md for:
   - Detailed troubleshooting guides
   - Architecture diagrams
   - Historical context
   - Extended examples
3. **Inline scripts**: Keep embedded scripts under 50 lines; extract larger scripts to `.github/skills/scripts/`

---

## Complete Script Inventory

### Scripts to Migrate (24 Total)

| # | Script Name | Skill Name | Category | Priority | Est. Lines |
|---|-------------|------------|----------|----------|------------|
| 1 | `go-test-coverage.sh` | `test-backend-coverage` | test | P0 | 150 |
| 2 | `frontend-test-coverage.sh` | `test-frontend-coverage` | test | P0 | 120 |
| 3 | `integration-test.sh` | `integration-test-all` | integration | P1 | 200 |
| 4 | `coraza_integration.sh` | `integration-test-coraza` | integration | P1 | 180 |
| 5 | `crowdsec_integration.sh` | `integration-test-crowdsec` | integration | P1 | 250 |
| 6 | `crowdsec_decision_integration.sh` | `integration-test-crowdsec-decisions` | integration | P1 | 220 |
| 7 | `crowdsec_startup_test.sh` | `integration-test-crowdsec-startup` | integration | P1 | 150 |
| 8 | `cerberus_integration.sh` | `integration-test-cerberus` | integration | P2 | 180 |
| 9 | `rate_limit_integration.sh` | `integration-test-rate-limit` | integration | P2 | 150 |
| 10 | `waf_integration.sh` | `integration-test-waf` | integration | P2 | 160 |
| 11 | `trivy-scan.sh` | `security-scan-trivy` | security | P1 | 80 |
| 12 | `security-scan.sh` | `security-scan-general` | security | P1 | 100 |
| 13 | `qa-test-auth-certificates.sh` | `qa-test-auth-certificates` | qa | P2 | 200 |
| 14 | `check_go_build.sh` | `build-check-go` | build | P2 | 60 |
| 15 | `check-version-match-tag.sh` | `utility-version-check` | utility | P2 | 70 |
| 16 | `clear-go-cache.sh` | `utility-cache-clear-go` | utility | P2 | 40 |
| 17 | `bump_beta.sh` | `utility-bump-beta` | utility | P2 | 90 |
| 18 | `db-recovery.sh` | `utility-db-recovery` | utility | P2 | 120 |
| 19 | `repo_health_check.sh` | `utility-repo-health` | utility | P2 | 150 |
| 20 | `verify_crowdsec_app_config.sh` | `docker-verify-crowdsec-config` | docker | P2 | 80 |
| 21 | Unit test scripts (backend) | `test-backend-unit` | test | P0 | 50 |
| 22 | Unit test scripts (frontend) | `test-frontend-unit` | test | P0 | 50 |
| 23 | Go vulnerability check | `security-check-govulncheck` | security | P1 | 60 |
| 24 | Release script | `utility-release` | utility | P3 | 300+ |

### Scripts Excluded from Migration (5 Total)

| # | Script Name | Reason | Alternative |
|---|-------------|--------|-------------|
| 1 | `debug_db.py` | Python debugging tool, not task-oriented | Keep in scripts/ |
| 2 | `debug_rate_limit.sh` | Debugging tool, not production | Keep in scripts/ |
| 3 | `gopls_collect.sh` | IDE tooling, not CI/CD | Keep in scripts/ |
| 4 | `install-go-1.25.5.sh` | One-time setup script | Keep in scripts/ |
| 5 | `create_bulk_acl_issues.sh` | Ad-hoc script, not repeatable | Keep in scripts/ |

---

## Exact tasks.json Updates

### Current Structure (36 tasks)

All tasks that currently reference `scripts/*.sh` will be updated to reference `.github/skills/*.SKILL.md` via a skill runner script.

### New Task Structure

Each task will be updated with this pattern:

```json
{
    "label": "Test: Backend with Coverage",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh test-backend-coverage",
    "group": "test",
    "problemMatcher": []
}
```

### Skill Runner Script

Create `.github/skills/scripts/skill-runner.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SKILL_NAME="$1"
SKILL_FILE=".github/skills/${SKILL_NAME}.SKILL.md"

if [[ ! -f "$SKILL_FILE" ]]; then
    echo "Error: Skill '$SKILL_NAME' not found at $SKILL_FILE" >&2
    exit 1
fi

# Extract and execute the embedded script from the SKILL.md file
# This is a placeholder - actual implementation will parse the markdown
# and execute the appropriate code block

# For now, delegate to the original script (backward compatibility)
LEGACY_SCRIPT="scripts/$(grep -A 1 "^**Source**: " "$SKILL_FILE" | tail -n 1 | sed 's/.*`scripts\/\(.*\)`/\1/')"

if [[ -f "$LEGACY_SCRIPT" ]]; then
    exec "$LEGACY_SCRIPT" "$@"
else
    echo "Error: Legacy script not found: $LEGACY_SCRIPT" >&2
    exit 1
fi
```

### Complete tasks.json Mapping

| Current Task Label | Current Script | New Skill | Command Update |
|--------------------|----------------|-----------|----------------|
| Test: Backend with Coverage | `scripts/go-test-coverage.sh` | `test-backend-coverage` | `.github/skills/scripts/skill-runner.sh test-backend-coverage` |
| Test: Frontend with Coverage | `scripts/frontend-test-coverage.sh` | `test-frontend-coverage` | `.github/skills/scripts/skill-runner.sh test-frontend-coverage` |
| Integration: Run All | `scripts/integration-test.sh` | `integration-test-all` | `.github/skills/scripts/skill-runner.sh integration-test-all` |
| Integration: Coraza WAF | `scripts/coraza_integration.sh` | `integration-test-coraza` | `.github/skills/scripts/skill-runner.sh integration-test-coraza` |
| Integration: CrowdSec | `scripts/crowdsec_integration.sh` | `integration-test-crowdsec` | `.github/skills/scripts/skill-runner.sh integration-test-crowdsec` |
| Integration: CrowdSec Decisions | `scripts/crowdsec_decision_integration.sh` | `integration-test-crowdsec-decisions` | `.github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions` |
| Integration: CrowdSec Startup | `scripts/crowdsec_startup_test.sh` | `integration-test-crowdsec-startup` | `.github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup` |
| Security: Trivy Scan | Docker command (inline) | `security-scan-trivy` | `.github/skills/scripts/skill-runner.sh security-scan-trivy` |
| Security: Go Vulnerability Check | `go run golang.org/x/vuln/cmd/govulncheck` | `security-check-govulncheck` | `.github/skills/scripts/skill-runner.sh security-check-govulncheck` |
| Utility: Check Version Match Tag | `scripts/check-version-match-tag.sh` | `utility-version-check` | `.github/skills/scripts/skill-runner.sh utility-version-check` |
| Utility: Clear Go Cache | `scripts/clear-go-cache.sh` | `utility-cache-clear-go` | `.github/skills/scripts/skill-runner.sh utility-cache-clear-go` |
| Utility: Bump Beta Version | `scripts/bump_beta.sh` | `utility-bump-beta` | `.github/skills/scripts/skill-runner.sh utility-bump-beta` |
| Utility: Database Recovery | `scripts/db-recovery.sh` | `utility-db-recovery` | `.github/skills/scripts/skill-runner.sh utility-db-recovery` |

### Tasks NOT Modified (Build/Lint/Docker)

These tasks use inline commands or direct Go/npm commands and do NOT need skill migration:
- Build: Backend (`cd backend && go build ./...`)
- Build: Frontend (`cd frontend && npm run build`)
- Build: All (composite task)
- Test: Backend Unit Tests (`cd backend && go test ./...`)
- Test: Frontend (`cd frontend && npm run test`)
- Lint: Pre-commit, Go Vet, GolangCI-Lint, Frontend, TypeScript, Markdownlint, Hadolint
- Docker: Start/Stop Dev/Local Environment, View Logs, Prune

---

## CI/CD Workflow Updates

### Workflows to Update (8 files)

| Workflow File | Current Script References | New Skill References | Priority |
|---------------|---------------------------|---------------------|----------|
| `.github/workflows/quality-checks.yml` | `scripts/go-test-coverage.sh` | `test-backend-coverage` | P0 |
| `.github/workflows/quality-checks.yml` | `scripts/frontend-test-coverage.sh` | `test-frontend-coverage` | P0 |
| `.github/workflows/quality-checks.yml` | `scripts/trivy-scan.sh` | `security-scan-trivy` | P1 |
| `.github/workflows/waf-integration.yml` | `scripts/coraza_integration.sh` | `integration-test-coraza` | P1 |
| `.github/workflows/waf-integration.yml` | `scripts/crowdsec_integration.sh` | `integration-test-crowdsec` | P1 |
| `.github/workflows/security-weekly-rebuild.yml` | `scripts/security-scan.sh` | `security-scan-general` | P1 |
| `.github/workflows/auto-versioning.yml` | `scripts/check-version-match-tag.sh` | `utility-version-check` | P2 |
| `.github/workflows/repo-health.yml` | `scripts/repo_health_check.sh` | `utility-repo-health` | P2 |

### Workflow Update Pattern

Before:
```yaml
- name: Run Backend Tests with Coverage
  run: scripts/go-test-coverage.sh
```

After:
```yaml
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Workflows NOT Modified

These workflows do not reference scripts and are not affected:
- `docker-publish.yml`, `auto-changelog.yml`, `auto-add-to-project.yml`, `create-labels.yml`
- `docker-lint.yml`, `renovate.yml`, `auto-label-issues.yml`, `pr-checklist.yml`
- `history-rewrite-tests.yml`, `docs-to-issues.yml`, `dry-run-history-rewrite.yml`
- `caddy-major-monitor.yml`, `propagate-changes.yml`, `codecov-upload.yml`, `codeql.yml`

---

## .gitignore Patterns - CI/CD COMPATIBLE

### ⚠️ CRITICAL FOR CI/CD WORKFLOWS

**Skills MUST be committed to the repository** to work in CI/CD pipelines. GitHub Actions runners clone the repository and need access to all skill definitions and scripts.

### What Should Be COMMITTED (DO NOT IGNORE):

All skill infrastructure must be in version control:

```
✅ .github/skills/*.SKILL.md                   # Skill definitions (REQUIRED)
✅ .github/skills/scripts/*.sh                 # Execution scripts (REQUIRED)
✅ .github/skills/scripts/*.py                 # Helper scripts (REQUIRED)
✅ .github/skills/scripts/*.js                 # Helper scripts (REQUIRED)
✅ .github/skills/README.md                    # Documentation (REQUIRED)
✅ .github/skills/INDEX.json                   # AI discovery index (REQUIRED)
✅ .github/skills/examples/                    # Example files (RECOMMENDED)
✅ .github/skills/references/                  # Reference docs (RECOMMENDED)
```

### What Should Be IGNORED (Runtime Data Only):

Add the following section to `.gitignore` to exclude only runtime-generated artifacts:

```gitignore
# -----------------------------------------------------------------------------
# Agent Skills - Runtime Data Only (DO NOT ignore skill definitions)
# -----------------------------------------------------------------------------
# ⚠️ IMPORTANT: Only runtime artifacts are ignored. All .SKILL.md files and
# scripts MUST be committed for CI/CD workflows to function.

# Runtime temporary files
.github/skills/.cache/
.github/skills/temp/
.github/skills/tmp/
.github/skills/**/*.tmp

# Execution logs
.github/skills/logs/
.github/skills/**/*.log
.github/skills/**/nohup.out

# Test/coverage artifacts
.github/skills/coverage/
.github/skills/**/*.cover
.github/skills/**/*.html
.github/skills/**/test-output*.txt
.github/skills/**/*.db

# OS and editor files
.github/skills/**/.DS_Store
.github/skills/**/Thumbs.db
```

### Verification Checklist

Before committing the migration, verify:

- [ ] `.github/skills/*.SKILL.md` files are tracked by git
- [ ] `.github/skills/scripts/` directory is tracked by git
- [ ] Run `git status` and confirm no SKILL.md files are ignored
- [ ] CI/CD workflows can read skill files in test runs
- [ ] `.gitignore` only excludes runtime artifacts (logs, cache, temp)

### Why This Matters for CI/CD

GitHub Actions workflows depend on skill files being in the repository:

```yaml
# Example: CI/CD workflow REQUIRES committed skills
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage
  # ↑ This FAILS if test-backend-coverage.SKILL.md is not committed!
```

**Common Mistake**: Adding `.github/skills/` to `.gitignore` will break all CI/CD pipelines that reference skills.

**Correct Pattern**: Only ignore runtime subdirectories (`logs/`, `cache/`, `tmp/`) and specific file patterns (`*.log`, `*.tmp`).

---

## Validation and Testing Strategy

### Phase 0 Validation Tools

1. **Frontmatter Validator**: Python script to validate YAML frontmatter
   - Check required fields
   - Validate formats (name, version, tags)
   - Validate custom metadata fields
   - Report violations

   ```bash
   python3 .github/skills/scripts/validate-skills.py
   ```

2. **Skills Reference Tool Integration**:
   ```bash
   # Install skills-ref CLI tool
   npm install -g @agentskills/cli

   # Validate all skills
   skills-ref validate .github/skills/

   # Test skill discovery
   skills-ref list .github/skills/
   ```

3. **Skill Runner Tests**: Ensure each skill can be executed
   ```bash
   for skill in .github/skills/*.SKILL.md; do
       skill_name=$(basename "$skill" .SKILL.md)
       echo "Testing: $skill_name"
       .github/skills/scripts/skill-runner.sh "$skill_name" --dry-run
   done
   ```

### AI Discoverability Testing

1. **GitHub Copilot Discovery Test**:
   - Open VS Code with GitHub Copilot enabled
   - Type: "Run backend tests with coverage"
   - Verify Copilot suggests the skill

2. **Workspace Search Test**:
   ```bash
   # Search for skills by keyword
   grep -r "coverage" .github/skills/*.SKILL.md
   ```

3. **Skills Index Generation**:
   ```bash
   # Generate skills index for AI tools
   python3 .github/skills/scripts/generate-index.py > .github/skills/INDEX.json
   ```

### Coverage Validation

For all test-related skills (test-backend-coverage, test-frontend-coverage):
- Run the skill
- Capture coverage output
- Verify coverage meets 85% threshold
- Compare with legacy script output to ensure parity

```bash
# Test coverage parity
LEGACY_COV=$(scripts/go-test-coverage.sh 2>&1 | grep "total:" | awk '{print $3}')
SKILL_COV=$(.github/skills/scripts/skill-runner.sh test-backend-coverage 2>&1 | grep "total:" | awk '{print $3}')

if [[ "$LEGACY_COV" != "$SKILL_COV" ]]; then
    echo "Coverage mismatch: legacy=$LEGACY_COV skill=$SKILL_COV"
    exit 1
fi
```

### Integration Test Validation

For all integration-test-* skills:
- Execute in isolated Docker environment
- Verify exit codes match legacy scripts
- Validate output format matches expected patterns
- Check for resource cleanup (containers, volumes)

---

## Backward Compatibility and Deprecation Strategy

### Phase 1 (Release v1.0-beta.1): Dual Support

1. **Keep Legacy Scripts**: All `scripts/*.sh` files remain functional
2. **Add Deprecation Warnings**: Add to each legacy script:
   ```bash
   echo "⚠️  WARNING: This script is deprecated and will be removed in v1.1.0" >&2
   echo "    Please use: .github/skills/scripts/skill-runner.sh ${SKILL_NAME}" >&2
   echo "    For more info: docs/skills/migration-guide.md" >&2
   sleep 2
   ```
3. **Create Symlinks**: (Optional, for quick migration)
   ```bash
   ln -s ../.github/skills/scripts/skill-runner.sh scripts/test-backend-coverage.sh
   ```

### Phase 2 (Release v1.1.0): Full Migration

1. **Remove Legacy Scripts**: Delete all migrated scripts from `scripts/`
2. **Keep Excluded Scripts**: Retain debugging and one-time setup scripts
3. **Update Documentation**: Remove all references to legacy scripts

### Rollback Procedure

If critical issues are discovered post-migration:

1. **Immediate Rollback** (< 24 hours):
   ```bash
   git revert <migration-commit>
   git push origin main
   ```

2. **Partial Rollback** (specific skills):
   - Restore specific script from git history
   - Update tasks.json to point back to script
   - Document issues in GitHub issue

3. **Rollback Triggers**:
   - Coverage drops below 85%
   - Integration tests fail in CI/CD
   - Production deployment blocked
   - Skills not discoverable by AI tools

---

## Implementation Phases (6 Phases)

### Phase 0: Validation & Tooling (Days 1-2)
**Goal**: Set up validation infrastructure and test harness

**Tasks**:
1. Create `.github/skills/` directory structure
2. Implement `validate-skills.py` (frontmatter validator)
3. Implement `skill-runner.sh` (skill executor)
4. Implement `generate-index.py` (AI discovery index)
5. Create test harness for skill execution
6. Set up CI/CD job for skill validation
7. Document validation procedures

**Deliverables**:
- [ ] `.github/skills/scripts/validate-skills.py` (functional)
- [ ] `.github/skills/scripts/skill-runner.sh` (functional)
- [ ] `.github/skills/scripts/generate-index.py` (functional)
- [ ] `.github/skills/scripts/_shared_functions.sh` (utilities)
- [ ] `.github/workflows/validate-skills.yml` (CI/CD)
- [ ] `docs/skills/validation-guide.md` (documentation)

**Success Criteria**:
- All validators pass with zero errors
- Skill runner can execute proof-of-concept skill
- CI/CD pipeline validates skills on PR

---

### Phase 1: Core Testing Skills (Days 3-4)
**Goal**: Migrate critical test skills with coverage validation

**Skills (Priority P0)**:
1. `test-backend-coverage.SKILL.md` (from `go-test-coverage.sh`)
2. `test-backend-unit.SKILL.md` (from inline task)
3. `test-frontend-coverage.SKILL.md` (from `frontend-test-coverage.sh`)
4. `test-frontend-unit.SKILL.md` (from inline task)

**Tasks**:
1. Create SKILL.md files with complete frontmatter
2. Validate frontmatter with validator
3. Test each skill execution
4. Verify coverage output matches legacy
5. Update tasks.json for these 4 skills
6. Update quality-checks.yml workflow
7. Add deprecation warnings to legacy scripts

**Deliverables**:
- [ ] 4 test-related SKILL.md files (complete)
- [ ] tasks.json updated (4 tasks)
- [ ] .github/workflows/quality-checks.yml updated
- [ ] Coverage validation tests pass
- [ ] Deprecation warnings added to legacy scripts

**Success Criteria**:
- All 4 skills execute successfully
- Coverage meets 85% threshold
- CI/CD pipeline passes
- No regression in test execution time

---

### Phase 2: Integration Testing Skills (Days 5-7)
**Goal**: Migrate all integration test skills

**Skills (Priority P1)**:
1. `integration-test-all.SKILL.md`
2. `integration-test-coraza.SKILL.md`
3. `integration-test-crowdsec.SKILL.md`
4. `integration-test-crowdsec-decisions.SKILL.md`
5. `integration-test-crowdsec-startup.SKILL.md`
6. `integration-test-cerberus.SKILL.md`
7. `integration-test-rate-limit.SKILL.md`
8. `integration-test-waf.SKILL.md`

**Tasks**:
1. Create SKILL.md files for all 8 integration tests
2. Extract shared Docker helpers to `.github/skills/scripts/_docker_helpers.sh`
3. Validate each skill independently
4. Update tasks.json for integration tasks
5. Update waf-integration.yml workflow
6. Run full integration test suite

**Deliverables**:
- [ ] 8 integration-test SKILL.md files (complete)
- [ ] `.github/skills/scripts/_docker_helpers.sh` (utilities)
- [ ] tasks.json updated (8 tasks)
- [ ] .github/workflows/waf-integration.yml updated
- [ ] Integration test suite passes

**Success Criteria**:
- All 8 integration skills pass in CI/CD
- Docker cleanup verified (no orphaned containers)
- Test execution time within 10% of legacy
- All exit codes match legacy behavior

---

### Phase 3: Security & QA Skills (Days 8-9)
**Goal**: Migrate security scanning and QA testing skills

**Skills (Priority P1-P2)**:
1. `security-scan-trivy.SKILL.md`
2. `security-scan-general.SKILL.md`
3. `security-check-govulncheck.SKILL.md`
4. `qa-test-auth-certificates.SKILL.md`
5. `build-check-go.SKILL.md`

**Tasks**:
1. Create SKILL.md files for security/QA skills
2. Test security scans with known vulnerabilities
3. Update tasks.json for security tasks
4. Update security-weekly-rebuild.yml workflow
5. Validate QA test outputs

**Deliverables**:
- [ ] 5 security/QA SKILL.md files (complete)
- [ ] tasks.json updated (5 tasks)
- [ ] .github/workflows/security-weekly-rebuild.yml updated
- [ ] Security scan validation tests pass

**Success Criteria**:
- All security scans detect known issues
- QA tests pass with expected outputs
- CI/CD security checks functional

---

### Phase 4: Utility & Docker Skills (Days 10-11)
**Goal**: Migrate remaining utility and Docker skills

**Skills (Priority P2-P3)**:
1. `utility-version-check.SKILL.md`
2. `utility-cache-clear-go.SKILL.md`
3. `utility-bump-beta.SKILL.md`
4. `utility-db-recovery.SKILL.md`
5. `utility-repo-health.SKILL.md`
6. `docker-verify-crowdsec-config.SKILL.md`

**Tasks**:
1. Create SKILL.md files for utility/Docker skills
2. Test version checking logic
3. Test database recovery procedures
4. Update tasks.json for utility tasks
5. Update auto-versioning.yml and repo-health.yml workflows

**Deliverables**:
- [ ] 6 utility/Docker SKILL.md files (complete)
- [ ] tasks.json updated (6 tasks)
- [ ] .github/workflows/auto-versioning.yml updated
- [ ] .github/workflows/repo-health.yml updated

**Success Criteria**:
- All utility skills functional
- Version checking accurate
- Database recovery tested successfully

---

### Phase 5: Documentation & Cleanup (Days 12-13)
**Goal**: Complete documentation and prepare for full migration

**Tasks**:
1. Create `.github/skills/README.md` (skill index and overview)
2. Create `docs/skills/migration-guide.md` (user guide)
3. Create `docs/skills/skill-development-guide.md` (contributor guide)
4. Update main `README.md` with skills section
5. Generate skills index JSON for AI discovery
6. Run full validation suite
7. Create migration announcement (CHANGELOG.md)
8. Tag release v1.0-beta.1

**Deliverables**:
- [ ] `.github/skills/README.md` (complete)
- [ ] `docs/skills/migration-guide.md` (complete)
- [ ] `docs/skills/skill-development-guide.md` (complete)
- [ ] Main README.md updated
- [ ] `.github/skills/INDEX.json` (generated)
- [ ] CHANGELOG.md updated
- [ ] Git tag: v1.0-beta.1

**Success Criteria**:
- All documentation complete and accurate
- Skills index generated successfully
- AI tools can discover skills
- Migration guide tested by contributor

---

### Phase 6: Full Migration & Legacy Cleanup (Days 14+)
**Goal**: Remove legacy scripts and complete migration (v1.1.0)

**Tasks**:
1. Monitor v1.0-beta.1 for 1 release cycle (2 weeks minimum)
2. Address any issues discovered during beta
3. Remove deprecation warnings from skill runner
4. Delete legacy scripts from `scripts/` (keep excluded scripts)
5. Remove symlinks (if used)
6. Update all documentation to remove legacy references
7. Run full test suite (unit + integration + security)
8. Tag release v1.1.0

**Deliverables**:
- [ ] All legacy scripts removed (except excluded)
- [ ] All deprecation warnings removed
- [ ] Documentation updated (no legacy references)
- [ ] Full test suite passes
- [ ] Git tag: v1.1.0

**Success Criteria**:
- Zero references to legacy scripts in codebase
- All CI/CD workflows functional
- Coverage maintained at 85%+
- No production issues

---

## Risk Assessment and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Skills not discoverable by AI | High | Medium | Validate with skills-ref tool, test with GitHub Copilot |
| Coverage regression | High | Low | Automated coverage validation in CI/CD |
| Integration tests break | High | Low | Isolated Docker environments, rollback procedure |
| CI/CD pipeline failures | High | Low | Phase rollout, manual testing before merging |
| Frontmatter validation errors | Medium | Medium | Comprehensive validator, pre-commit hooks |
| Backward compatibility issues | Medium | Low | Dual support for 1 release cycle, deprecation warnings |
| Documentation gaps | Low | Medium | Peer review, user testing of migration guide |
| Excessive SKILL.md line counts | Low | Medium | Progressive disclosure, extract to separate docs |

---

## Proof-of-Concept: test-backend-coverage.SKILL.md

See separate file: [proof-of-concept/test-backend-coverage.SKILL.md](./proof-of-concept/test-backend-coverage.SKILL.md)

This POC demonstrates:
- Complete, validated frontmatter
- Progressive disclosure (< 500 lines)
- Embedded script with error handling
- Clear documentation structure
- AI-discoverable metadata

---

## Appendix A: Custom Metadata Fields

The `metadata` section in frontmatter includes project-specific extensions:

| Field | Type | Values | Purpose |
|-------|------|--------|---------|
| `category` | string | test, integration, security, qa, build, utility, docker | Primary categorization |
| `subcategory` | string | coverage, unit, scan, etc. | Secondary categorization |
| `execution_time` | enum | short (<1min), medium (1-5min), long (>5min) | Resource planning |
| `risk_level` | enum | low, medium, high | Impact assessment |
| `ci_cd_safe` | boolean | true, false | Can run in CI/CD without human oversight |
| `requires_network` | boolean | true, false | Network dependency flag |
| `idempotent` | boolean | true, false | Safe to run multiple times |

---

## Appendix B: Skills Index Schema

The `.github/skills/INDEX.json` file follows this schema for AI discovery:

```json
{
  "schema_version": "1.0",
  "generated_at": "2025-12-20T00:00:00Z",
  "project": "Charon",
  "skills_count": 24,
  "skills": [
    {
      "name": "test-backend-coverage",
      "file": "test-backend-coverage.SKILL.md",
      "version": "1.0.0",
      "description": "Run Go backend tests with coverage analysis and validation",
      "category": "test",
      "tags": ["testing", "coverage", "go", "backend"],
      "execution_time": "medium",
      "ci_cd_safe": true
    }
  ]
}
```

---

## Appendix C: Supervisor Concerns Addressed

### 1. Directory Structure (Flat vs Categorized)
**Decision**: Flat structure in `.github/skills/`
**Rationale**:
- Simpler AI discovery (no directory traversal)
- Easier skill references in tasks.json and workflows
- Naming convention provides implicit categorization
- Aligns with agentskills.io examples

### 2. SKILL.md Templates
**Resolution**: Complete template provided with validated frontmatter
**Details**:
- All required fields documented
- Custom metadata fields defined
- Validation rules specified
- Example provided in POC

### 3. Progressive Disclosure
**Strategy**:
- Keep SKILL.md under 500 lines
- Extract detailed docs to `docs/skills/{name}.md`
- Extract large scripts to `.github/skills/scripts/`
- Use links for extended documentation

### 4. Metadata Usage
**Custom Fields**: Defined in Appendix A
**Purpose**:
- AI discovery and filtering
- Resource planning and scheduling
- Risk assessment
- CI/CD automation

### 5. CI/CD Workflow Updates
**Complete Plan**:
- 8 workflows identified for updates
- Exact file paths provided
- Update pattern documented
- Priority assigned

### 6. Validation Strategy
**Comprehensive Plan**:
- Phase 0 dedicated to validation tooling
- Frontmatter validator (Python)
- Skill runner with dry-run mode
- Skills index generator
- AI discoverability tests
- Coverage parity validation

### 7. Testing Strategy
**AI Discoverability**:
- GitHub Copilot integration test
- Workspace search validation
- Skills index generation
- skills-ref tool validation

### 8. Backward Compatibility
**Complete Strategy**:
- Dual support for 1 release cycle
- Deprecation warnings in legacy scripts
- Optional symlinks for quick migration
- Clear rollback procedures
- Rollback triggers defined

### 9. Phase 0 and Phase 5
**Added**:
- Phase 0: Validation & Tooling (Days 1-2)
- Phase 5: Documentation & Cleanup (Days 12-13)
- Phase 6: Full Migration & Legacy Cleanup (Days 14+)

### 10. Rollback Procedure
**Detailed Plan**:
- Immediate rollback (< 24 hours): git revert
- Partial rollback: restore specific scripts
- Rollback triggers: coverage drop, CI/CD failures, production blocks
- Documented procedures and commands

---

## Next Steps

1. **Review and Approval**: Supervisor reviews this complete specification
2. **Phase 0 Kickoff**: Begin validation tooling implementation
3. **POC Review**: Review and approve proof-of-concept SKILL.md
4. **Phase 1 Start**: Migrate core testing skills (Days 3-4)

**Estimated Total Timeline**: 14 days (2 weeks)
**Target Completion**: 2025-12-27

---

**Document Status**: COMPLETE - READY FOR IMPLEMENTATION
**Last Updated**: 2025-12-20
**Author**: Charon Project Team
**Reviewed by**: [Pending Supervisor Approval]

plaintext
.github/skills/
├── README.md                                    # Overview and index
├── test-backend-coverage.SKILL.md              # Backend Go test coverage
├── test-backend-unit.SKILL.md                  # Backend unit tests (fast)
├── test-frontend-coverage.SKILL.md             # Frontend test with coverage
├── test-frontend-unit.SKILL.md                 # Frontend unit tests (fast)
├── integration-test-all.SKILL.md               # Run all integration tests
├── integration-test-coraza.SKILL.md            # Coraza WAF integration
├── integration-test-crowdsec.SKILL.md          # CrowdSec bouncer integration
├── integration-test-crowdsec-decisions.SKILL.md # CrowdSec decisions API
├── integration-test-crowdsec-startup.SKILL.md  # CrowdSec startup sequence
├── integration-test-cerberus.SKILL.md          # Cerberus integration
├── integration-test-rate-limit.SKILL.md        # Rate limiting integration
├── integration-test-waf.SKILL.md               # WAF integration tests
├── security-scan-trivy.SKILL.md                # Trivy vulnerability scan
├── security-scan-general.SKILL.md              # General security scan
├── security-check-govulncheck.SKILL.md         # Go vulnerability check
├── qa-test-auth-certificates.SKILL.md          # Auth/certificate QA test
├── build-check-go.SKILL.md                     # Verify Go build
├── utility-version-check.SKILL.md              # Check version matches tag
├── utility-cache-clear-go.SKILL.md             # Clear Go build cache
├── utility-bump-beta.SKILL.md                  # Bump beta version
├── utility-db-recovery.SKILL.md                # Database recovery
├── utility-repo-health.SKILL.md                # Repository health check
├── docker-verify-crowdsec-config.SKILL.md      # Verify CrowdSec config
└── scripts/                                     # Shared scripts (reusable)
    ├── _shared_functions.sh                    # Common bash functions
    ├── _test_helpers.sh                        # Test utility functions
    ├── _docker_helpers.sh                      # Docker utility functions
    └── _coverage_helpers.sh                    # Coverage calculation helpers
