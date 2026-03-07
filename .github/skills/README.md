# Agent Skills - Charon Project

This directory contains [Agent Skills](https://agentskills.io) following the agentskills.io specification for AI-discoverable, executable tasks.

## Overview

Agent Skills are self-documenting, AI-discoverable task definitions that combine YAML frontmatter (metadata) with Markdown documentation. Each skill represents a specific task or workflow that can be executed by both humans and AI assistants.

**Location**: `.github/skills/` is the [VS Code Copilot standard location](https://code.visualstudio.com/docs/copilot/customization/agent-skills) for Agent Skills
**Format**: Skills follow the [agentskills.io specification](https://agentskills.io/specification) for structure and metadata

## Directory Structure

```
.github/skills/
├── README.md                           # This file
├── scripts/                            # Shared infrastructure scripts
│   ├── skill-runner.sh                # Universal skill executor
│   ├── validate-skills.py             # Frontmatter validation tool
│   ├── _logging_helpers.sh            # Logging utilities
│   ├── _error_handling_helpers.sh     # Error handling utilities
│   └── _environment_helpers.sh        # Environment validation
├── examples/                           # Example skill templates
└── {skill-name}/                      # Individual skill directories
    ├── SKILL.md                       # Skill definition and documentation
    └── scripts/
        └── run.sh                     # Skill execution script
```

## Available Skills

### Testing Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [test-backend-coverage](./test-backend-coverage.SKILL.md) | test | Run Go backend tests with coverage analysis | ✅ Active |
| [test-backend-unit](./test-backend-unit.SKILL.md) | test | Run fast Go unit tests without coverage | ✅ Active |
| [test-frontend-coverage](./test-frontend-coverage.SKILL.md) | test | Run frontend tests with coverage reporting | ✅ Active |
| [test-frontend-unit](./test-frontend-unit.SKILL.md) | test | Run fast frontend unit tests without coverage | ✅ Active |
| [test-e2e-playwright](./test-e2e-playwright.SKILL.md) | test | Run Playwright E2E tests with browser selection | ✅ Active |
| [test-e2e-playwright-debug](./test-e2e-playwright-debug.SKILL.md) | test | Run E2E tests in headed/debug mode for troubleshooting | ✅ Active |
| [test-e2e-playwright-coverage](./test-e2e-playwright-coverage.SKILL.md) | test | Run E2E tests with coverage collection | ✅ Active |

### Integration Testing Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [integration-test-all](./integration-test-all.SKILL.md) | integration | Run all integration tests in sequence | ✅ Active |
| [integration-test-coraza](./integration-test-coraza.SKILL.md) | integration | Test Coraza WAF integration | ✅ Active |
| [integration-test-crowdsec](./integration-test-crowdsec.SKILL.md) | integration | Test CrowdSec bouncer integration | ✅ Active |
| [integration-test-crowdsec-decisions](./integration-test-crowdsec-decisions.SKILL.md) | integration | Test CrowdSec decisions API | ✅ Active |
| [integration-test-crowdsec-startup](./integration-test-crowdsec-startup.SKILL.md) | integration | Test CrowdSec startup sequence | ✅ Active |

### Security Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [security-scan-gorm](./security-scan-gorm.SKILL.md) | security | Detect GORM ID leaks, exposed secrets, and misconfigurations | ✅ Active |
| [security-scan-trivy](./security-scan-trivy.SKILL.md) | security | Run Trivy vulnerability scanner | ✅ Active |
| [security-scan-go-vuln](./security-scan-go-vuln.SKILL.md) | security | Run Go vulnerability check | ✅ Active |

### QA Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [qa-lefthook-all](./qa-lefthook-all.SKILL.md) | qa | Run all lefthook pre-commit‑phase hooks on entire codebase | ✅ Active |

### Utility Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [utility-version-check](./utility-version-check.SKILL.md) | utility | Validate version matches git tag | ✅ Active |
| [utility-clear-go-cache](./utility-clear-go-cache.SKILL.md) | utility | Clear Go build and module caches | ✅ Active |
| [utility-bump-beta](./utility-bump-beta.SKILL.md) | utility | Increment beta version number | ✅ Active |
| [utility-db-recovery](./utility-db-recovery.SKILL.md) | utility | Database integrity check and recovery | ✅ Active |

### Docker Skills

| Skill Name | Category | Description | Status |
|------------|----------|-------------|--------|
| [docker-start-dev](./docker-start-dev.SKILL.md) | docker | Start development Docker Compose environment | ✅ Active |
| [docker-stop-dev](./docker-stop-dev.SKILL.md) | docker | Stop development Docker Compose environment | ✅ Active |
| [docker-rebuild-e2e](./docker-rebuild-e2e.SKILL.md) | docker | Rebuild Docker image and restart E2E Playwright container | ✅ Active |
| [docker-prune](./docker-prune.SKILL.md) | docker | Clean up unused Docker resources | ✅ Active |

## Usage

### Running Skills

Use the universal skill runner to execute any skill:

```bash
# From project root
.github/skills/scripts/skill-runner.sh <skill-name> [args...]

# Example: Run backend coverage tests
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

### From VS Code Tasks

Skills are integrated with VS Code tasks (`.vscode/tasks.json`):

1. Open Command Palette (`Ctrl+Shift+P` or `Cmd+Shift+P`)
2. Select `Tasks: Run Task`
3. Choose the task (e.g., `Test: Backend with Coverage`)

### In CI/CD Workflows

Reference skills in GitHub Actions:

```yaml
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage
```

## Validation

### Validate a Single Skill

```bash
python3 .github/skills/scripts/validate-skills.py --single .github/skills/test-backend-coverage/SKILL.md
```

### Validate All Skills

```bash
python3 .github/skills/scripts/validate-skills.py
```

### Validation Checks

The validator ensures:
- ✅ Required frontmatter fields are present
- ✅ Field formats are correct (name, version, description)
- ✅ Tags meet minimum/maximum requirements
- ✅ Compatibility information is valid
- ✅ Custom metadata follows project conventions

## Creating New Skills

### 1. Create Skill Directory Structure

```bash
mkdir -p .github/skills/{skill-name}/scripts
```

### 2. Create SKILL.md

Start with the template structure:

```markdown
---
# agentskills.io specification v1.0
name: "skill-name"
version: "1.0.0"
description: "Brief description (max 120 chars)"
author: "Charon Project"
license: "MIT"
tags:
  - "tag1"
  - "tag2"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "tool"
    version: ">=1.0"
    optional: false
metadata:
  category: "category-name"
  execution_time: "short|medium|long"
  risk_level: "low|medium|high"
  ci_cd_safe: true|false
---

# Skill Name

## Overview

Brief description of what this skill does.

## Prerequisites

- List prerequisites

## Usage

```bash
.github/skills/scripts/skill-runner.sh skill-name
```

## Examples

### Example 1: Basic Usage

```bash
# Example command
```

---

**Last Updated**: YYYY-MM-DD
**Maintained by**: Charon Project
```

### 3. Create Execution Script

Create `scripts/run.sh` with proper structure:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../../scripts" && pwd)"

source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"
# Add validation calls here

# Execute skill logic
log_step "EXECUTION" "Running skill"
cd "${PROJECT_ROOT}"

# Your skill logic here

log_success "Skill completed successfully"
```

### 4. Set Permissions

```bash
chmod +x .github/skills/{skill-name}/scripts/run.sh
```

### 5. Validate

```bash
python3 .github/skills/scripts/validate-skills.py --single .github/skills/{skill-name}/SKILL.md
```

### 6. Test

```bash
.github/skills/scripts/skill-runner.sh {skill-name}
```

## Naming Conventions

- **Skill Names**: `{category}-{feature}-{variant}` (kebab-case)
- **Categories**: `test`, `integration-test`, `security`, `qa`, `build`, `utility`, `docker`
- **Examples**:
  - `test-backend-coverage`
  - `integration-test-crowdsec`
  - `security-scan-trivy`
  - `utility-version-check`

## Best Practices

### Documentation
- Keep SKILL.md under 500 lines
- Use progressive disclosure (link to extended docs for complex topics)
- Include practical examples
- Document all prerequisites and environment variables

### Scripts
- Always source helper scripts for consistent logging and error handling
- Validate environment before execution
- Use `set -euo pipefail` for robust error handling
- Make scripts idempotent when possible
- Clean up resources on exit

### Metadata
- Use accurate `execution_time` values for scheduling
- Set `ci_cd_safe: false` for skills requiring human oversight
- Mark `idempotent: true` only if truly safe to run multiple times
- Include all required dependencies in `requirements`

### Error Handling
- Use helper functions (`log_error`, `error_exit`, `check_command_exists`)
- Provide clear error messages with remediation steps
- Return appropriate exit codes (0 = success, non-zero = failure)

## Helper Scripts Reference

### Logging Helpers (`_logging_helpers.sh`)

```bash
log_info "message"      # Informational message
log_success "message"   # Success message (green)
log_warning "message"   # Warning message (yellow)
log_error "message"     # Error message (red)
log_debug "message"     # Debug message (only if DEBUG=1)
log_step "STEP" "msg"   # Step header
log_command "cmd"       # Log command before executing
```

### Error Handling Helpers (`_error_handling_helpers.sh`)

```bash
error_exit "message" [exit_code]              # Print error and exit
check_command_exists "cmd" ["message"]        # Verify command exists
check_file_exists "file" ["message"]          # Verify file exists
check_dir_exists "dir" ["message"]            # Verify directory exists
run_with_retry max_attempts delay cmd...      # Retry command with backoff
trap_error [script_name]                      # Set up error trapping
cleanup_on_exit cleanup_func                  # Register cleanup function
```

### Environment Helpers (`_environment_helpers.sh`)

```bash
validate_go_environment ["min_version"]       # Check Go installation
validate_python_environment ["min_version"]   # Check Python installation
validate_node_environment ["min_version"]     # Check Node.js installation
validate_docker_environment                   # Check Docker installation
set_default_env "VAR" "default_value"        # Set env var with default
validate_project_structure file1 file2...     # Check required files exist
get_project_root ["marker_file"]             # Find project root directory
```

## Troubleshooting

### Skill not found
```
Error: Skill not found: skill-name
```
**Solution**: Verify the skill directory exists in `.github/skills/` and contains a `SKILL.md` file

### Skill script not executable
```
Error: Skill execution script is not executable
```
**Solution**: Run `chmod +x .github/skills/{skill-name}/scripts/run.sh`

### Validation errors
```
[ERROR] skill.SKILL.md :: description: Must be 120 characters or less
```
**Solution**: Fix the frontmatter field according to the error message and re-validate

### Command not found in skill
```
Error: go is not installed or not in PATH
```
**Solution**: Install the required dependency or ensure it's in your PATH

## Integration Points

### VS Code Tasks
Skills are integrated in `.vscode/tasks.json`:
```json
{
    "label": "Test: Backend with Coverage",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh test-backend-coverage",
    "group": "test"
}
```

### GitHub Actions
Skills are referenced in `.github/workflows/`:
```yaml
- name: Run Backend Tests with Coverage
  run: .github/skills/scripts/skill-runner.sh test-backend-coverage
```

### Pre-commit Hooks
Skills can be used in `.pre-commit-config.yaml`:
```yaml
repos:
  - repo: local
    hooks:
      - id: backend-coverage
        name: Backend Coverage Check
        entry: .github/skills/scripts/skill-runner.sh test-backend-coverage
        language: system
```

## Resources

- [agentskills.io Specification](https://agentskills.io/specification)
- [VS Code Copilot Agent Skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills)
- [Project Documentation](../../docs/)
- [Contributing Guide](../../CONTRIBUTING.md)

## Support

For issues, questions, or contributions:
1. Check existing [GitHub Issues](https://github.com/Wikid82/charon/issues)
2. Review [CONTRIBUTING.md](../../CONTRIBUTING.md)
3. Create a new issue if needed

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**License**: MIT
