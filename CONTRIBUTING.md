# Contributing to Charon

Thank you for your interest in contributing to CaddyProxyManager+! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)
- [Documentation](#documentation)

## Code of Conduct

This project follows a Code of Conduct that all contributors are expected to adhere to:

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on what's best for the community
- Show empathy towards other community members

## Getting Started

-### Prerequisites

- **Go 1.24+** for backend development
- **Node.js 20+** and npm for frontend development
- Git for version control
- A GitHub account

### Development Tools

Install golangci-lint for pre-commit hooks (required for Go development):

```bash
# Option 1: Homebrew (macOS/Linux)
brew install golangci-lint

# Option 2: Go install (any platform)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Option 3: Binary installation (see https://golangci-lint.run/usage/install/)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

Ensure `$GOPATH/bin` is in your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify installation:

```bash
golangci-lint --version
# Should output: golangci-lint has version 1.xx.x ...
```

**Note:** Pre-commit hooks will **BLOCK commits** if golangci-lint finds issues. This is intentional - fix the issues before committing.

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

```bash
git clone https://github.com/YOUR_USERNAME/charon.git
cd charon
```

1. Add the upstream remote:

```bash
git remote add upstream https://github.com/Wikid82/charon.git
```

### Set Up Development Environment

**Backend:**

```bash
cd backend
go mod download
go run ./cmd/seed/main.go  # Seed test data
go run ./cmd/api/main.go   # Start backend
```

**Frontend:**

```bash
cd frontend
npm install
npm run dev  # Start frontend dev server
```

## Development Workflow

### Branching Strategy

- **main** - Production-ready code (stable releases)
- **nightly** - Pre-release testing branch (automated daily builds at 02:00 UTC)
- **development** - Main development branch (default for contributions)
- **feature/** - Feature branches (e.g., `feature/add-ssl-support`)
- **bugfix/** - Bug fix branches (e.g., `bugfix/fix-import-crash`)
- **hotfix/** - Urgent production fixes

### Branch Flow

The project uses a three-tier branching model:

```
development → nightly → main
   (unstable)    (testing)  (stable)
```

**Flow details:**

1. **development → nightly**: Automated daily merge at 02:00 UTC
2. **nightly → main**: Manual PR after validation and testing
3. **Contributors always branch from `development`**

**Why nightly?**

- Provides a testing ground for features before production
- Automated daily builds catch integration issues
- Users can test pre-release features via `nightly` Docker tag
- Maintainers validate stability before merging to `main`

### Creating a Feature Branch

Always branch from `development`:

```bash
git checkout development
git pull upstream development
git checkout -b feature/your-feature-name
```

**Note:** Never branch from `nightly` or `main`. The `nightly` branch is managed by automation and receives daily merges from `development`.

### Commit Message Guidelines

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**

```
feat(proxy-hosts): add SSL certificate upload

- Implement certificate upload endpoint
- Add UI for certificate management
- Update database schema

Closes #123
```

```
fix(import): resolve conflict detection bug

When importing Caddyfiles with multiple domains, conflicts
were not being detected properly.

Fixes #456
```

### Keeping Your Fork Updated

```bash
git checkout development
git fetch upstream
git merge upstream/development
git push origin development
```

## Coding Standards

### Go Backend

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Write godoc comments for exported functions
- Keep functions small and focused
- Handle errors explicitly

**Example:**

```go
// GetProxyHost retrieves a proxy host by UUID.
// Returns an error if the host is not found.
func GetProxyHost(uuid string) (*models.ProxyHost, error) {
    var host models.ProxyHost
    if err := db.First(&host, "uuid = ?", uuid).Error; err != nil {
        return nil, fmt.Errorf("proxy host not found: %w", err)
    }
    return &host, nil
}
```

### TypeScript Frontend

- Use TypeScript for type safety
- Follow React best practices and hooks patterns
- Use functional components
- Destructure props at function signature
- Extract reusable logic into custom hooks

**Example:**

```typescript
interface ProxyHostFormProps {
  host?: ProxyHost
  onSubmit: (data: ProxyHostData) => Promise<void>
  onCancel: () => void
}

export function ProxyHostForm({ host, onSubmit, onCancel }: ProxyHostFormProps) {
  const [domain, setDomain] = useState(host?.domain ?? '')
  // ... component logic
}
```

### CSS/Styling

- Use TailwindCSS utility classes
- Follow the dark theme color palette
- Keep custom CSS minimal
- Use semantic color names from the theme

## Testing Guidelines

### Testing Against Nightly Builds

Before submitting a PR, test your changes against the latest nightly build:

**Pull latest nightly:**

```bash
docker pull ghcr.io/wikid82/charon:nightly
```

**Run your local changes against nightly:**

```bash
# Start nightly container
docker run -d --name charon-nightly \
  -p 8080:8080 \
  ghcr.io/wikid82/charon:nightly

# Test your feature/fix
curl http://localhost:8080/api/v1/health

# Clean up
docker stop charon-nightly && docker rm charon-nightly
```

**Integration testing:**

If your changes affect existing features, verify compatibility:

1. Deploy nightly build in test environment
2. Run your modified frontend/backend against it
3. Verify no regressions in existing functionality
4. Document any breaking changes in your PR

**Reporting nightly issues:**

If you find bugs in nightly builds:

1. Check if the issue exists in `development` branch
2. Open an issue tagged with `nightly` label
3. Include nightly build date or commit SHA
4. Provide reproduction steps

### Backend Tests

Write tests for all new functionality:

```go
func TestGetProxyHost(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    host := createTestHost(db)

    // Execute
    result, err := GetProxyHost(host.UUID)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, host.Domain, result.Domain)
}
```

**Run tests:**

```bash
go test ./... -v
go test -cover ./...
```

### Frontend Tests

Write component and hook tests using Vitest and React Testing Library:

```typescript
describe('ProxyHostForm', () => {
  it('renders create form with empty fields', async () => {
    render(
      <ProxyHostForm onSubmit={vi.fn()} onCancel={vi.fn()} />
    )

    await waitFor(() => {
      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })
  })
})
```

**Run tests:**

```bash
npm test              # Watch mode
npm run test:coverage # Coverage report
```

### CrowdSec Frontend Test Coverage

The CrowdSec integration has comprehensive frontend test coverage (100%) across all modules:

- **API Clients** - All CrowdSec API endpoints tested with error handling
- **React Query Hooks** - Complete hook testing with query invalidation
- **Data & Utilities** - Preset validation and export functionality
- **162 tests total** - All passing with no flaky tests

See [QA Coverage Report](docs/reports/qa_crowdsec_frontend_coverage_report.md) for details.

### Test Coverage

- Aim for 85%+ code coverage (current backend: 85.4%)
- All new features must include tests
- Bug fixes should include regression tests
- CrowdSec modules maintain 100% frontend coverage

## Adding New Skills

Charon uses [Agent Skills](https://agentskills.io) for AI-discoverable development tasks. Skills are standardized, self-documenting task definitions that can be executed by humans and AI assistants.

### What is a Skill?

A skill is a combination of:

- **YAML Frontmatter**: Metadata following the [agentskills.io specification](https://agentskills.io/specification)
- **Markdown Documentation**: Usage instructions, examples, and troubleshooting
- **Execution Script**: Shell script that performs the actual task

### When to Create a Skill

Create a new skill when you have a:

- **Repeatable task** that developers run frequently
- **Complex workflow** that benefits from documentation
- **CI/CD operation** that should be AI-discoverable
- **Development tool** that needs consistent execution

**Examples**: Running tests, building artifacts, security scans, database operations, deployment tasks

### Skill Creation Process

#### 1. Plan Your Skill

Before creating, define:

- **Name**: Use `{category}-{feature}-{variant}` format (e.g., `test-backend-coverage`)
- **Category**: test, integration-test, security, qa, build, utility, docker
- **Purpose**: One clear sentence describing what it does
- **Requirements**: Tools, environment variables, permissions needed
- **Output**: What the skill produces (exit codes, files, reports)

#### 2. Create Directory Structure

```bash
# Create skill directory
mkdir -p .github/skills/{skill-name}-scripts

# Skill files will be:
# .github/skills/{skill-name}.SKILL.md         # Documentation
# .github/skills/{skill-name}-scripts/run.sh    # Execution script
```

#### 3. Write the SKILL.md File

Use the template structure:

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

- List required tools
- List required permissions
- List environment setup

## Usage

```bash
.github/skills/scripts/skill-runner.sh skill-name
```

## Examples

### Example 1: Basic Usage

```bash
# Description
command example
```

## Error Handling

- Common errors and solutions
- Exit codes and meanings

## Related Skills

- [related-skill](./related-skill.SKILL.md)

---

**Last Updated**: YYYY-MM-DD
**Maintained by**: Charon Project
**Source**: Original implementation or script path

```

#### 4. Create the Execution Script

Create `.github/skills/{skill-name}-scripts/run.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../scripts" && pwd)"

source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"
check_command_exists "required-tool" "Please install required-tool"

# Execute skill logic
log_step "EXECUTION" "Running skill"
cd "${PROJECT_ROOT}"

# Your skill implementation here
if ! your-command; then
    error_exit "Skill execution failed"
fi

log_success "Skill completed successfully"
```

Make it executable:

```bash
chmod +x .github/skills/{skill-name}-scripts/run.sh
```

#### 5. Validate the Skill

Run the validation tool:

```bash
# Validate single skill
python3 .github/skills/scripts/validate-skills.py --single .github/skills/{skill-name}.SKILL.md

# Validate all skills
python3 .github/skills/scripts/validate-skills.py
```

Fix any validation errors before proceeding.

#### 6. Test the Skill

Test execution:

```bash
# Direct execution
.github/skills/scripts/skill-runner.sh {skill-name}

# Verify output
# Check exit codes
# Confirm expected behavior
```

#### 7. Add VS Code Task (Optional)

If the skill should be available in VS Code's task menu, add to `.vscode/tasks.json`:

```json
{
    "label": "Category: Skill Name",
    "type": "shell",
    "command": ".github/skills/scripts/skill-runner.sh skill-name",
    "group": "test"
}
```

#### 8. Update Documentation

Add your skill to `.github/skills/README.md`:

```markdown
| [skill-name](./skill-name.SKILL.md) | category | Description | ✅ Active |
```

### Validation Requirements

All skills must pass validation:

- ✅ **Required fields**: name, version, description, author, license, tags
- ✅ **Name format**: kebab-case (lowercase, hyphens)
- ✅ **Version format**: Semantic versioning (x.y.z)
- ✅ **Description**: Max 120 characters
- ✅ **Tags**: Minimum 2, maximum 5
- ✅ **Executable script**: Must exist and be executable

### Best Practices

**Documentation:**

- Keep SKILL.md under 500 lines
- Include real-world examples
- Document all prerequisites clearly
- Add troubleshooting section for common issues

**Scripts:**

- Always use helper functions for logging and error handling
- Validate environment before execution
- Make scripts idempotent when possible
- Clean up resources on exit (use trap)

**Testing:**

- Test skill in clean environment
- Verify all exit codes
- Check output format consistency
- Test error scenarios

**Metadata:**

- Set accurate `execution_time` (short < 1min, medium 1-5min, long > 5min)
- Use `ci_cd_safe: false` for interactive or risky operations
- Mark `idempotent: true` only if truly safe to run repeatedly
- Include all dependencies in `requirements`

### Helper Scripts Reference

Charon provides helper scripts for common operations:

**Logging** (`_logging_helpers.sh`):

- `log_info`, `log_success`, `log_warning`, `log_error`, `log_debug`
- `log_step` for section headers
- `log_command` to log before executing

**Error Handling** (`_error_handling_helpers.sh`):

- `error_exit` to print error and exit
- `check_command_exists`, `check_file_exists`, `check_dir_exists`
- `run_with_retry` for network operations
- `trap_error` for automatic error trapping

**Environment** (`_environment_helpers.sh`):

- `validate_go_environment`, `validate_python_environment`, `validate_node_environment`
- `validate_docker_environment`
- `set_default_env` for environment variables
- `get_project_root` to find repository root

### Resources

- **[Agent Skills README](.github/skills/README.md)** — Complete skills documentation
- **[agentskills.io Specification](https://agentskills.io/specification)** — Standard format
- **[Existing Skills](.github/skills/)** — Reference implementations
- **[Migration Guide](docs/AGENT_SKILLS_MIGRATION.md)** — Background and benefits

## Pull Request Process

### Before Submitting

1. **Ensure tests pass:**

```bash
# Backend
go test ./...

# Frontend
npm test -- --run
```

1. **Check code quality:**

```bash
# Go formatting
go fmt ./...

# Frontend linting
npm run lint
```

1. **Update documentation** if needed
2. **Add tests** for new functionality
3. **Rebase on latest development** branch

### Submitting a Pull Request

1. Push your branch to your fork:

```bash
git push origin feature/your-feature-name
```

1. Open a Pull Request on GitHub
2. Fill out the PR template completely
3. Link related issues using "Closes #123" or "Fixes #456"
4. Request review from maintainers

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Manual testing performed
- [ ] All tests passing

## Screenshots (if applicable)
Add screenshots of UI changes

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review performed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No new warnings generated
```

### Review Process

- Maintainers will review within 2-3 business days
- Address review feedback promptly
- Keep discussions focused and professional
- Be open to suggestions and alternative approaches

## Issue Guidelines

### Reporting Bugs

Use the bug report template and include:

- Clear, descriptive title
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, browser, Go version, etc.)
- Screenshots or error logs
- Potential solutions (if known)

### Feature Requests

Use the feature request template and include:

- Clear description of the feature
- Use case and motivation
- Potential implementation approach
- Mockups or examples (if applicable)

### Issue Labels

- `bug` - Something isn't working
- `enhancement` - New feature or request
- `documentation` - Documentation improvements
- `good first issue` - Good for newcomers
- `help wanted` - Extra attention needed
- `priority: high` - Urgent issue
- `wontfix` - Will not be fixed

## Documentation

### Code Documentation

- Add docstrings to all exported functions
- Include examples in complex functions
- Document return types and error conditions
- Keep comments up-to-date with code changes

### Project Documentation

When adding features, update:

- `README.md` - User-facing information
- `docs/api.md` - API changes
- `docs/import-guide.md` - Import feature updates
- `docs/database-schema.md` - Schema changes

## Recognition

Contributors will be recognized in:

- CONTRIBUTORS.md file
- Release notes for significant contributions
- GitHub contributors page

## Questions?

- Open a [Discussion](https://github.com/Wikid82/charon/discussions) for general questions
- Join our community chat (coming soon)
- Tag maintainers in issues for urgent matters

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

---

Thank you for contributing to CaddyProxyManager+! 🎉
