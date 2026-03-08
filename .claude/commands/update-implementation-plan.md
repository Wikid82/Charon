# Update Implementation Plan

Update the implementation plan file at: **$ARGUMENTS**

Based on new or updated requirements, revise the plan to reflect the current state. Your output must be machine-readable, deterministic, and structured for autonomous execution.

## Core Requirements

- Preserve the existing plan structure and template format
- Update only sections affected by the new requirements
- Use deterministic language with zero ambiguity
- Maintain all required front matter fields

## Update Process

1. **Read the current plan** to understand existing structure, goals, and tasks
2. **Identify changes** — what requirements are new or changed?
3. **Update affected sections**:
   - Front matter: `last_updated`, `status`
   - Requirements section: add new REQ/SEC/CON identifiers
   - Implementation steps: add/modify phases and tasks
   - Files, Testing, Risks sections as needed
4. **Preserve completed tasks** — do not remove or reorder TASK items that are already checked
5. **Validate template compliance** before finalising

## Template Validation Rules

- All front matter fields must be present and properly formatted
- All section headers must match exactly (case-sensitive)
- All identifier prefixes must follow the specified format (REQ-, TASK-, SEC-, etc.)
- Tables must include all required columns
- No placeholder text may remain in the final output

## Status Values

`Completed` | `In progress` | `Planned` | `Deprecated` | `On Hold`

Update `status` in both the front matter AND the badge in the Introduction section to reflect the current state.
