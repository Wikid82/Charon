# docs/issues - Issue Specification Files

This directory contains markdown files that are automatically converted to GitHub Issues when merged to `main` or `development`.

## How It Works

1. **Create a markdown file** in this directory using the template format
2. **Add YAML frontmatter** with issue metadata (title, labels, priority, etc.)
3. **Merge to main/development** - the `docs-to-issues.yml` workflow runs
4. **GitHub Issue is created** with your specified metadata
5. **File is moved** to `docs/issues/created/` to prevent duplicates

## Quick Start

Copy `_TEMPLATE.md` and fill in your issue details:

```yaml
---
title: "My New Issue"
labels:
  - feature
  - backend
priority: medium
---

# My New Issue

Description of the issue...
```

## Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `title` | Yes* | Issue title (*or uses first H1 as fallback) |
| `labels` | No | Array of labels to apply |
| `priority` | No | `critical`, `high`, `medium`, `low` |
| `milestone` | No | Milestone name |
| `assignees` | No | Array of GitHub usernames |
| `parent_issue` | No | Parent issue number for linking |
| `create_sub_issues` | No | If `true`, each `## Section` becomes a sub-issue |

## Sub-Issues

To create multiple related issues from one file, set `create_sub_issues: true`:

```yaml
---
title: "Main Testing Issue"
labels: [testing]
create_sub_issues: true
---

# Main Testing Issue

Overview content for the parent issue.

## Unit Testing

This section becomes a separate issue.

## Integration Testing

This section becomes another separate issue.
```

## Manual Trigger

You can manually run the workflow with:

```bash
# Dry run (no issues created)
gh workflow run docs-to-issues.yml -f dry_run=true

# Process specific file
gh workflow run docs-to-issues.yml -f file_path=docs/issues/my-issue.md
```

## Labels

Labels are automatically created if they don't exist. Common labels:

- **Priority**: `critical`, `high`, `medium`, `low`
- **Type**: `feature`, `bug`, `enhancement`, `testing`, `documentation`
- **Component**: `backend`, `frontend`, `ui`, `security`, `caddy`, `database`
