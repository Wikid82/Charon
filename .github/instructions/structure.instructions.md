---
applyTo: '*'
description: 'Repository structure guidelines to maintain organized file placement'
---

# Repository Structure Guidelines

## Root Level Rules

The repository root should contain ONLY:

- Essential config files (`.gitignore`, `Makefile`, etc.)
- Standard project files (`README.md`, `CONTRIBUTING.md`, `LICENSE`, `CHANGELOG.md`)
- Go workspace files (`go.work`, `go.work.sum`)
- VS Code workspace (`Chiron.code-workspace`)
- Primary `Dockerfile` (entrypoint and compose files live in `.docker/`)

## File Placement Rules

### Implementation/Feature Documentation

- **Location**: `docs/implementation/`
- **Pattern**: `*_SUMMARY.md`, `*_IMPLEMENTATION.md`, `*_COMPLETE.md`, `*_FEATURE.md`
- **Never** place implementation docs at root

### Docker Compose Files

- **Location**: `.docker/compose/`
- **Files**: `docker-compose.yml`, `docker-compose.*.yml`
- **Override**: Local overrides go in `.docker/compose/docker-compose.override.yml` (gitignored)
- **Exception**: `docker-compose.override.yml` at root is allowed for backward compatibility

### Docker Support Files

- **Location**: `.docker/`
- **Files**: `docker-entrypoint.sh`, Docker documentation (`README.md`)

### Test Artifacts

- **Never commit**: `*.sarif`, `*_test.txt`, `*.cover` files at root
- **Location**: Test outputs should go to `test-results/` or be gitignored

### Debug/Temp Config Files

- **Never commit**: Temporary JSON configs like `caddy_*.json` at root
- **Location**: Use `configs/` for persistent configs, gitignore temp files

### Scripts

- **Location**: `scripts/` for general scripts
- **Location**: `.github/skills/scripts/` for agent skill scripts

## Before Creating New Files

Ask yourself:

1. Is this a standard project file? → Root is OK
2. Is this implementation documentation? → `docs/implementation/`
3. Is this Docker-related? → `.docker/` or `.docker/compose/`
4. Is this a test artifact? → `test-results/` or gitignore
5. Is this a script? → `scripts/`
6. Is this runtime config? → `configs/`

## Directory Structure Reference

```
/
├── .docker/                    # Docker configuration
│   ├── compose/               # All docker-compose files
│   └── docker-entrypoint.sh   # Container entrypoint
├── .github/                    # GitHub workflows, agents, instructions
├── .vscode/                    # VS Code settings and tasks
├── backend/                    # Go backend source
├── configs/                    # Runtime configurations
├── docs/                       # Documentation
│   ├── implementation/        # Implementation/feature docs archive
│   ├── plans/                 # Planning documents
│   └── ...                    # User-facing documentation
├── frontend/                   # React frontend source
├── scripts/                    # Build/test scripts
├── test-results/               # Test outputs (gitignored)
├── tools/                      # Development tools
└── [standard files]           # README, LICENSE, Makefile, etc.
```

## Enforcement

This structure is enforced by:

- `.gitignore` patterns preventing commits of artifacts at root
- Code review guidelines
- These instructions for AI assistants

When reviewing PRs or generating code, ensure new files follow these placement rules.
