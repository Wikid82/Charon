# Agent Skills Migration Guide

**Status**: Complete
**Date**: 2025-12-20
**Migration Version**: v1.0
**Target Release**: v1.0-beta.1

---

## Executive Summary

Charon has migrated from legacy shell scripts in `/scripts` to a standardized [Agent Skills](https://agentskills.io) format stored in `.github/skills/`. This migration provides AI-discoverable, self-documenting tasks that work seamlessly with GitHub Copilot and other AI assistants.

**Key Benefits:**

- ✅ **AI Discoverability**: Skills are automatically discovered by GitHub Copilot
- ✅ **Self-Documenting**: Each skill includes complete usage documentation
- ✅ **Standardized Format**: Follows agentskills.io specification
- ✅ **Better Integration**: Works with VS Code tasks, CI/CD, and command line
- ✅ **Enhanced Metadata**: Rich metadata for filtering and discovery

---

## What Changed?

### Before: Legacy Scripts

```bash
# Old way - direct script execution
scripts/go-test-coverage.sh
scripts/crowdsec_integration.sh
scripts/trivy-scan.sh
```

**Problems with legacy scripts:**

- ❌ No standardized metadata
- ❌ Not AI-discoverable
- ❌ Inconsistent documentation
- ❌ No validation tooling
- ❌ Hard to discover and understand

### After: Agent Skills

```bash
# New way - skill-based execution
.github/skills/scripts/skill-runner.sh test-backend-coverage
.github/skills/scripts/skill-runner.sh integration-test-crowdsec
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

**Benefits of Agent Skills:**

- ✅ Standardized YAML metadata (name, version, tags, requirements)
- ✅ AI-discoverable by GitHub Copilot and other tools
- ✅ Comprehensive documentation in each SKILL.md file
- ✅ Automated validation with validation tools
- ✅ Easy to browse and understand

---

## Migration Statistics

### Skills Created: 19 Total

| Category | Skills | Percentage |
|----------|--------|------------|
| Testing | 4 | 21% |
| Integration Testing | 5 | 26% |
| Security | 2 | 11% |
| QA | 1 | 5% |
| Utility | 4 | 21% |
| Docker | 3 | 16% |

### Scripts Migrated: 19 of 24

**Migrated Scripts:**

1. `go-test-coverage.sh` → `test-backend-coverage`
2. `frontend-test-coverage.sh` → `test-frontend-coverage`
3. `integration-test.sh` → `integration-test-all`
4. `coraza_integration.sh` → `integration-test-coraza`
5. `crowdsec_integration.sh` → `integration-test-crowdsec`
6. `crowdsec_decision_integration.sh` → `integration-test-crowdsec-decisions`
7. `crowdsec_startup_test.sh` → `integration-test-crowdsec-startup`
8. `trivy-scan.sh` → `security-scan-trivy`
9. `check-version-match-tag.sh` → `utility-version-check`
10. `clear-go-cache.sh` → `utility-clear-go-cache`
11. `bump_beta.sh` → `utility-bump-beta`
12. `db-recovery.sh` → `utility-db-recovery`
13. Unit test tasks → `test-backend-unit`, `test-frontend-unit`
14. Go vulnerability check → `security-scan-go-vuln`
15. Pre-commit hooks → `qa-precommit-all`
16. Docker compose dev → `docker-start-dev`, `docker-stop-dev`
17. Docker cleanup → `docker-prune`

**Scripts NOT Migrated (by design):**

- `debug_db.py` - Interactive debugging tool
- `debug_rate_limit.sh` - Interactive debugging tool
- `gopls_collect.sh` - IDE-specific tooling
- `install-go-1.25.6.sh` - One-time setup script
- `create_bulk_acl_issues.sh` - Ad-hoc administrative script

---

## Directory Structure

### New Layout

```
.github/skills/
├── README.md                                    # Skills index and documentation
├── scripts/                                     # Shared infrastructure
│   ├── skill-runner.sh                         # Universal executor
│   ├── validate-skills.py                      # Validation tool
│   ├── _logging_helpers.sh                     # Logging utilities
│   ├── _error_handling_helpers.sh              # Error handling
│   └── _environment_helpers.sh                 # Environment validation
│
├── test-backend-coverage.SKILL.md              # Testing skills
├── test-backend-unit.SKILL.md
├── test-frontend-coverage.SKILL.md
├── test-frontend-unit.SKILL.md
│
├── integration-test-all.SKILL.md               # Integration testing
├── integration-test-coraza.SKILL.md
├── integration-test-crowdsec.SKILL.md
├── integration-test-crowdsec-decisions.SKILL.md
├── integration-test-crowdsec-startup.SKILL.md
│
├── security-scan-trivy.SKILL.md                # Security skills
├── security-scan-go-vuln.SKILL.md
│
├── qa-precommit-all.SKILL.md                   # QA skills
│
├── utility-version-check.SKILL.md              # Utility skills
├── utility-clear-go-cache.SKILL.md
├── utility-bump-beta.SKILL.md
├── utility-db-recovery.SKILL.md
│
├── docker-start-dev.SKILL.md                   # Docker skills
├── docker-stop-dev.SKILL.md
└── docker-prune.SKILL.md
```

### Flat Structure Rationale

We chose a **flat directory structure** (no subcategories) for maximum AI discoverability:

- ✅ Simpler skill discovery (no directory traversal)
- ✅ Easier reference in tasks and workflows
- ✅ Category implicit in naming (`test-*`, `integration-*`, etc.)
- ✅ Aligns with VS Code Copilot standards
- ✅ Compatible with agentskills.io specification

---

## How to Use Skills

### Command Line Execution

Use the universal skill runner:

```bash
# Basic usage
.github/skills/scripts/skill-runner.sh <skill-name>

# Examples
.github/skills/scripts/skill-runner.sh test-backend-coverage
.github/skills/scripts/skill-runner.sh integration-test-crowdsec
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

### VS Code Tasks

Skills are integrated with VS Code's task system:

1. **Open Command Palette**: `Ctrl+Shift+P` (Linux/Windows) or `Cmd+Shift+P` (macOS)
2. **Select**: `Tasks: Run Task`
3. **Choose**: Your desired skill (e.g., `Test: Backend with Coverage`)

All tasks in `.vscode/tasks.json` now use the skill runner.

### GitHub Copilot

Ask GitHub Copilot naturally:

- "Run backend tests with coverage"
- "Start the development environment"
- "Run security scans on the project"
- "Check if version matches git tag"

Copilot will automatically discover and suggest the appropriate skills.

### CI/CD Workflows

Skills are integrated in GitHub Actions workflows:

```yaml
# Example from .github/workflows/quality-checks.yml
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage

- name: Run Security Scan
  run: .github/skills/scripts/skill-runner.sh security-scan-trivy
```

---

## Backward Compatibility

### Deprecation Period: v0.14.1 to v1.0.0

Legacy scripts remain functional during the deprecation period with warnings:

```bash
$ scripts/go-test-coverage.sh
⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
    Please use: .github/skills/scripts/skill-runner.sh test-backend-coverage
    For more info: docs/AGENT_SKILLS_MIGRATION.md

# Script continues to work normally...
```

### Migration Timeline

| Version | Status | Legacy Scripts | Agent Skills |
|---------|--------|----------------|--------------|
| v0.14.1 | Current | ✅ Functional with warnings | ✅ Fully operational |
| v1.0-beta.1 | Next | ✅ Functional with warnings | ✅ Fully operational |
| v1.0.0 | Stable | ✅ Functional with warnings | ✅ Fully operational |
| v2.0.0 | Future | ❌ Removed | ✅ Only supported method |

**Recommendation**: Migrate to skills now to avoid disruption when v2.0.0 is released.

---

## SKILL.md Format

Each skill follows the [agentskills.io specification](https://agentskills.io/specification):

### Structure

```markdown
---
# YAML Frontmatter (metadata)
name: "skill-name"
version: "1.0.0"
description: "Brief description"
author: "Charon Project"
license: "MIT"
tags: ["tag1", "tag2"]
compatibility:
  os: ["linux", "darwin"]
  shells: ["bash"]
requirements:
  - name: "go"
    version: ">=1.23"
metadata:
  category: "test"
  execution_time: "medium"
  ci_cd_safe: true
---

# Skill Name

## Overview
Brief description

## Prerequisites
- Required tools
- Required setup

## Usage
```bash
.github/skills/scripts/skill-runner.sh skill-name
```

## Examples

Practical examples with explanations

## Error Handling

Common errors and solutions

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project

```

### Metadata Fields

**Standard Fields** (from agentskills.io):
- `name`, `version`, `description`, `author`, `license`
- `tags`, `compatibility`, `requirements`

**Custom Fields** (Charon-specific):
- `metadata.category`: test, integration-test, security, qa, utility, docker
- `metadata.execution_time`: short (<1min), medium (1-5min), long (>5min)
- `metadata.risk_level`: low, medium, high
- `metadata.ci_cd_safe`: true/false
- `metadata.idempotent`: true/false

---

## Benefits of Agent Skills

### For Developers

**AI Discoverability:**
- GitHub Copilot automatically discovers skills
- Natural language queries work seamlessly
- No need to memorize script paths

**Self-Documentation:**
- Each skill includes complete usage documentation
- Prerequisites clearly stated
- Examples provided for common scenarios
- Error handling documented

**Consistent Interface:**
- All skills use the same execution pattern
- Standardized logging and error handling
- Predictable exit codes

### For Maintainers

**Standardization:**
- Consistent format across all tasks
- Automated validation catches errors
- Easy to review and maintain

**Metadata Rich:**
- Execution time estimates for scheduling
- Risk levels for approval workflows
- CI/CD safety flags for automation
- Dependency tracking built-in

**Extensibility:**
- Easy to add new skills using templates
- Helper scripts reduce boilerplate
- Validation ensures quality

### For CI/CD

**Integration:**
- Skills work in GitHub Actions, GitLab CI, Jenkins, etc.
- Consistent execution in all environments
- Exit codes properly propagated

**Reliability:**
- Validated before use
- Environment checks built-in
- Error handling standardized

---

## Migration Checklist

If you're using legacy scripts, here's how to migrate:

### For Individual Developers

- [ ] Read this migration guide
- [ ] Review [.github/skills/README.md](.github/skills/README.md)
- [ ] Update local scripts to use skill-runner
- [ ] Update VS Code tasks (if customized)
- [ ] Test skills in your environment

### For CI/CD Pipelines

- [ ] Update GitHub Actions workflows to use skill-runner
- [ ] Update GitLab CI, Jenkins, or other CI tools
- [ ] Test pipelines in a non-production environment
- [ ] Monitor first few runs for issues
- [ ] Update documentation with new commands

### For Documentation

- [ ] Update README references to scripts
- [ ] Update setup guides with skill commands
- [ ] Update troubleshooting guides
- [ ] Update contributor documentation

---

## Validation and Quality

All skills are validated to ensure quality:

### Validation Tool

```bash
# Validate all skills
python3 .github/skills/scripts/validate-skills.py

# Validate single skill
python3 .github/skills/scripts/validate-skills.py --single .github/skills/test-backend-coverage.SKILL.md
```

### Validation Checks

- ✅ Required YAML frontmatter fields
- ✅ Name format (kebab-case)
- ✅ Version format (semantic versioning)
- ✅ Description length (max 120 chars)
- ✅ Tag count (2-5 tags)
- ✅ Compatibility information
- ✅ Requirements format
- ✅ Metadata completeness

### Current Status

All 19 skills pass validation with **0 errors and 0 warnings** (100% pass rate).

---

## Troubleshooting

### "Skill not found" Error

```
Error: Skill not found: test-backend-coverage
Error: Skill file does not exist: .github/skills/test-backend-coverage.SKILL.md
```

**Solution**: Verify the skill name is correct and the file exists in `.github/skills/`

### "Script not executable" Error

```
Error: Skill execution script is not executable: .github/skills/test-backend-coverage-scripts/run.sh
```

**Solution**: Make the script executable:

```bash
chmod +x .github/skills/test-backend-coverage-scripts/run.sh
```

### Legacy Script Warnings

```
⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
```

**Solution**: This is informational. The script still works, but you should migrate to skills:

```bash
# Instead of:
scripts/go-test-coverage.sh

# Use:
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Validation Errors

```
[ERROR] test-backend-coverage.SKILL.md :: name: Must be kebab-case
```

**Solution**: Fix the frontmatter field according to the error message and re-validate.

---

## Resources

### Documentation

- **[Agent Skills README](.github/skills/README.md)** - Complete skill documentation
- **[agentskills.io](https://agentskills.io)** - Specification and tooling
- **[VS Code Copilot Guide](https://code.visualstudio.com/docs/copilot/customization/agent-skills)** - VS Code integration
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** - How to create new skills

### Support

- **[GitHub Discussions](https://github.com/Wikid82/charon/discussions)** - Ask questions
- **[GitHub Issues](https://github.com/Wikid82/charon/issues)** - Report problems
- **[Project README](../README.md)** - General project information

---

## Feedback and Contributions

We welcome feedback on the Agent Skills migration:

- **Found a bug?** [Open an issue](https://github.com/Wikid82/charon/issues)
- **Have a suggestion?** [Start a discussion](https://github.com/Wikid82/charon/discussions)
- **Want to contribute?** See [CONTRIBUTING.md](../CONTRIBUTING.md)

---

**Migration Status**: ✅ Complete (19/24 skills, 79%)
**Validation Status**: ✅ 100% pass rate (19/19 skills)
**Documentation**: ✅ Complete
**Backward Compatibility**: ✅ Maintained until v2.0.0

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**License**: MIT
