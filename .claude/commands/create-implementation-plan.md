# Create Implementation Plan

Create a new implementation plan file for: **$ARGUMENTS**

Your output must be machine-readable, deterministic, and structured for autonomous execution.

## Core Requirements

- Generate plans fully executable by AI agents or humans
- Use deterministic language with zero ambiguity
- Structure all content for automated parsing
- Self-contained — no external dependencies for understanding

## Output File

- Save to `docs/plans/` directory
- Naming: `[purpose]-[component]-[version].md`
- Purpose prefixes: `upgrade|refactor|feature|data|infrastructure|process|architecture|design`
- Examples: `feature-auth-module-1.md`, `upgrade-system-command-4.md`

## Mandatory Template

```md
---
goal: [Concise Title]
version: [1.0]
date_created: [YYYY-MM-DD]
last_updated: [YYYY-MM-DD]
owner: [Team/Individual]
status: 'Planned'
tags: [feature, upgrade, chore, architecture, migration, bug]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

[Short introduction to the plan and its goal.]

## 1. Requirements & Constraints

- **REQ-001**: Requirement 1
- **SEC-001**: Security Requirement 1
- **CON-001**: Constraint 1
- **GUD-001**: Guideline 1
- **PAT-001**: Pattern to follow

## 2. Implementation Steps

### Implementation Phase 1

- GOAL-001: [Goal of this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | Description of task 1 | | |
| TASK-002 | Description of task 2 | | |

### Implementation Phase 2

- GOAL-002: [Goal of this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-003 | Description of task 3 | | |

## 3. Alternatives

- **ALT-001**: Alternative approach 1 — reason not chosen

## 4. Dependencies

- **DEP-001**: Dependency 1

## 5. Files

- **FILE-001**: Description of file 1

## 6. Testing

- **TEST-001**: Description of test 1

## 7. Risks & Assumptions

- **RISK-001**: Risk 1
- **ASSUMPTION-001**: Assumption 1

## 8. Related Specifications / Further Reading

[Links to related specs or external docs]
```

## Phase Architecture

- Each phase must have measurable completion criteria
- Tasks within phases must be executable in parallel unless dependencies are specified
- All task descriptions must include specific file paths, function names, and exact implementation details
- No task should require human interpretation

## Status Badge Colors

`Completed` → bright green | `In progress` → yellow | `Planned` → blue | `Deprecated` → red | `On Hold` → orange
