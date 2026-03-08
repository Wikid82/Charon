# Create GitHub Issues from Implementation Plan

Create GitHub Issues for the implementation plan at: **$ARGUMENTS**

## Process

1. **Analyse** the plan file to identify all implementation phases
2. **Check existing issues** using `gh issue list` to avoid duplicates
3. **Create one issue per phase** using `gh issue create`
4. **Use appropriate templates** from `.github/ISSUE_TEMPLATE/` (feature or general)

## Requirements

- One issue per implementation phase
- Clear, structured titles and descriptions
- Include only changes required by the plan
- Verify against existing issues before creation

## Issue Content Structure

**Title**: Phase name from the implementation plan (e.g., `feat: Phase 1 - Backend API implementation`)

**Description**:
```md
## Overview
[Phase goal from implementation plan]

## Tasks
[Task list from the plan's phase table]

## Acceptance Criteria
[Success criteria / DoD for this phase]

## Dependencies
[Any issues that must be completed first]

## Related Plan
[Link to docs/plans/current_spec.md or specific plan file]
```

**Labels**: Use appropriate labels:
- `feature` — new functionality
- `chore` — tooling, CI, maintenance
- `bug` — defect fixes
- `security` — security-related changes
- `documentation` — docs-only changes

## Commands

```bash
# List existing issues to avoid duplicates
gh issue list --state open

# Create a new issue
gh issue create \
  --title "feat: Phase 1 - [Phase Name]" \
  --body "$(cat <<'EOF'
## Overview
...
EOF
)" \
  --label "feature"

# Link issues (add parent reference in body)
```
