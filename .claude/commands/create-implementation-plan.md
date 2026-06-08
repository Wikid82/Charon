# Create Implementation Plan

Create a new implementation plan for: $ARGUMENTS

## Primary Directive

Your goal is to create a new implementation plan file. Your output must be machine-readable, deterministic, and structured for autonomous execution by AI agents or humans.

## Output File Specifications

- Save implementation plan files in `docs/plans/` directory
- Use naming convention: `[purpose]-[component]-[version].md`
- Purpose prefixes: `upgrade|refactor|feature|data|infrastructure|process|architecture|design`
- Example: `feature-proxy-host-crud-1.md`

## Mandatory Template Structure

```md
---
goal: [Concise Title Describing the Plan's Goal]
version: 1.0
date_created: [YYYY-MM-DD]
status: 'Planned'
tags: [feature, upgrade, chore, architecture, migration, bug]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

[A short concise introduction to the plan and the goal it is intended to achieve.]

## 1. Requirements & Constraints

- **REQ-001**: Requirement 1
- **SEC-001**: Security Requirement 1
- **CON-001**: Constraint 1

## 2. Implementation Steps

### Implementation Phase 1

- GOAL-001: [Describe the goal of this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | Description of task 1 | | |
| TASK-002 | Description of task 2 | | |

### Implementation Phase 2

- GOAL-002: [Describe the goal of this phase]

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-003 | Description of task 3 | | |

## 3. Alternatives

- **ALT-001**: Alternative approach 1 and why it was not chosen

## 4. Dependencies

- **DEP-001**: Dependency 1

## 5. Files

- **FILE-001**: Description of file 1 to be modified

## 6. Testing

- **TEST-001**: Description of test 1

## 7. Risks & Assumptions

- **RISK-001**: Risk 1
- **ASSUMPTION-001**: Assumption 1

## 8. Related Specifications / Further Reading

[Link to related spec or external documentation]
```

## Requirements

- Use explicit, unambiguous language with zero interpretation required
- Structure all content as machine-parseable formats (tables, lists)
- Include specific file paths, line numbers, and exact code references where applicable
- Define all variables, constants, and configuration values explicitly
- Include validation criteria that can be automatically verified
- No placeholder text may remain in the final output
