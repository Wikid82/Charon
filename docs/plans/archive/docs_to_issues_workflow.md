# docs/issues to GitHub Issues Workflow - Implementation Plan

**Version:** 1.0
**Date:** 2025-12-13
**Status:** 📋 PLANNING

---

## Executive Summary

This document provides a comprehensive plan for a GitHub Actions workflow that automatically converts markdown files in `docs/issues/` into GitHub Issues, applies labels, and adds them to the project board.

---

## 1. Research Findings

### 1.1 Current docs/issues File Analysis

Analyzed 8 existing files in `docs/issues/`:

| File | Structure | Has Frontmatter | Labels Present | Title Pattern |
|------|-----------|-----------------|----------------|---------------|
| `ACL-testing-tasks.md` | Free-form + metadata section | ❌ | Inline suggestion | H1 + metadata block |
| `Additional_Security.md` | Markdown lists | ❌ | ❌ | H3 title |
| `bulk-acl-subissues.md` | Multi-issue spec | ❌ | Inline per section | Inline titles |
| `bulk-acl-testing.md` | Full issue template | ❌ | Inline section | H1 title |
| `hectate.md` | Feature spec | ❌ | ❌ | H1 title |
| `orthrus.md` | Feature spec | ❌ | ❌ | H1 title |
| `plex-remote-access-helper.md` | Issue template | ❌ | Inline section | `## Issue Title` |
| `rotating-loading-animations.md` | Enhancement spec | ❌ | `**Issue Type**` line | H1 title |

**Key Finding:** No files use YAML frontmatter. Most have ad-hoc metadata embedded in markdown body.

### 1.2 Existing Workflow Patterns

From `.github/workflows/`:

| Workflow | Pattern | Relevance |
|----------|---------|-----------|
| `auto-label-issues.yml` | `actions/github-script` for issue manipulation | Label creation, issue API |
| `propagate-changes.yml` | Complex `github-script` with file analysis | File path detection |
| `auto-add-to-project.yml` | `actions/add-to-project` action | Project board integration |
| `create-labels.yml` | Label creation/update logic | Label management |

### 1.3 Project Board Configuration

- Project URL stored in `secrets.PROJECT_URL`
- PAT token: `secrets.ADD_TO_PROJECT_PAT`
- Uses `actions/add-to-project@v1.0.2` for automatic addition

---

## 2. Markdown File Format Specification

### 2.1 Required Frontmatter Format

All files in `docs/issues/` should use YAML frontmatter:

```yaml
---
title: "Issue Title Here"
labels:
  - testing
  - feature
  - backend
assignees:
  - username1
  - username2
milestone: "v0.2.0-beta.2"
priority: high          # critical, high, medium, low
type: feature           # feature, bug, enhancement, testing, documentation
parent_issue: 42        # Optional: Link as sub-issue
create_sub_issues: true # Optional: Parse ## sections as separate issues
---

# Issue Title (H1 fallback if frontmatter title missing)

Issue body content starts here...
```

### 2.2 Frontmatter Fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `title` | Yes* | string | Issue title (*or H1 fallback) |
| `labels` | No | array | Labels to apply (created if missing) |
| `assignees` | No | array | GitHub usernames |
| `milestone` | No | string | Milestone name |
| `priority` | No | enum | Maps to priority label |
| `type` | No | enum | Maps to type label |
| `parent_issue` | No | number | Parent issue number for linking |
| `create_sub_issues` | No | boolean | Parse H2 sections as sub-issues |

### 2.3 Sub-Issue Detection

When `create_sub_issues: true`, each `## Section Title` becomes a sub-issue:

```markdown
---
title: "Main Testing Issue"
labels: [testing]
create_sub_issues: true
---

# Main Testing Issue

Overview text (goes in parent issue).

## Sub-Issue #1: Unit Testing

This section becomes a separate issue titled "Unit Testing"
with body content from this section.

## Sub-Issue #2: Integration Testing

This section becomes another separate issue.
```

---

## 3. Workflow Implementation

### 3.1 Workflow File: `.github/workflows/docs-to-issues.yml`

```yaml
name: Convert Docs to Issues

on:
  push:
    branches:
      - main
      - development
    paths:
      - 'docs/issues/**/*.md'
      - '!docs/issues/created/**'

  # Allow manual trigger
  workflow_dispatch:
    inputs:
      dry_run:
        description: 'Dry run (no issues created)'
        required: false
        default: 'false'
        type: boolean
      file_path:
        description: 'Specific file to process (optional)'
        required: false
        type: string

permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  convert-docs:
    name: Convert Markdown to Issues
    runs-on: ubuntu-latest
    if: github.actor != 'github-actions[bot]'

    steps:
      - name: Checkout repository
        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4
        with:
          fetch-depth: 2  # Need previous commit for diff

      - name: Set up Node.js
        uses: actions/setup-node@39370e3970a6d050c480ffad4ff0ed4d3fdee5af # v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm install gray-matter

      - name: Detect changed files
        id: changes
        uses: actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea # v7
        with:
          script: |
            const fs = require('fs');
            const path = require('path');

            // Manual file specification
            const manualFile = '${{ github.event.inputs.file_path }}';
            if (manualFile) {
              if (fs.existsSync(manualFile)) {
                core.setOutput('files', JSON.stringify([manualFile]));
                return;
              } else {
                core.setFailed(`File not found: ${manualFile}`);
                return;
              }
            }

            // Get changed files from commit
            const { data: commit } = await github.rest.repos.getCommit({
              owner: context.repo.owner,
              repo: context.repo.repo,
              ref: context.sha
            });

            const changedFiles = (commit.files || [])
              .filter(f => f.filename.startsWith('docs/issues/'))
              .filter(f => !f.filename.startsWith('docs/issues/created/'))
              .filter(f => f.filename.endsWith('.md'))
              .filter(f => f.status !== 'removed')
              .map(f => f.filename);

            console.log('Changed issue files:', changedFiles);
            core.setOutput('files', JSON.stringify(changedFiles));

      - name: Process issue files
        id: process
        uses: actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea # v7
        env:
          DRY_RUN: ${{ github.event.inputs.dry_run || 'false' }}
        with:
          script: |
            const fs = require('fs');
            const path = require('path');
            const matter = require('gray-matter');

            const files = JSON.parse('${{ steps.changes.outputs.files }}');
            const isDryRun = process.env.DRY_RUN === 'true';
            const createdIssues = [];
            const errors = [];

            if (files.length === 0) {
              console.log('No issue files to process');
              core.setOutput('created_count', 0);
              return;
            }

            // Helper: Ensure label exists
            async function ensureLabel(name) {
              try {
                await github.rest.issues.getLabel({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  name: name
                });
              } catch (e) {
                if (e.status === 404) {
                  // Create label with default color
                  const colors = {
                    testing: 'BFD4F2',
                    feature: 'A2EEEF',
                    enhancement: '84B6EB',
                    bug: 'D73A4A',
                    documentation: '0075CA',
                    backend: '1D76DB',
                    frontend: '5EBEFF',
                    security: 'EE0701',
                    ui: '7057FF',
                    caddy: '1F6FEB',
                    'needs-triage': 'FBCA04',
                    acl: 'C5DEF5',
                    regression: 'D93F0B',
                    'manual-testing': 'BFD4F2',
                    'bulk-acl': '006B75',
                    'error-handling': 'D93F0B',
                    'ui-ux': '7057FF',
                    integration: '0E8A16',
                    performance: 'EDEDED',
                    'cross-browser': '5319E7',
                    plus: 'FFD700',
                    beta: '0052CC',
                    alpha: '5319E7',
                    high: 'D93F0B',
                    medium: 'FBCA04',
                    low: '0E8A16',
                    critical: 'B60205'
                  };
                  await github.rest.issues.createLabel({
                    owner: context.repo.owner,
                    repo: context.repo.repo,
                    name: name,
                    color: colors[name.toLowerCase()] || '666666'
                  });
                  console.log(`Created label: ${name}`);
                }
              }
            }

            // Helper: Parse markdown file
            function parseIssueFile(filePath) {
              const content = fs.readFileSync(filePath, 'utf8');
              const { data: frontmatter, content: body } = matter(content);

              // Extract title: frontmatter > first H1 > filename
              let title = frontmatter.title;
              if (!title) {
                const h1Match = body.match(/^#\s+(.+)$/m);
                title = h1Match ? h1Match[1] : path.basename(filePath, '.md');
              }

              // Build labels array
              const labels = [...(frontmatter.labels || [])];
              if (frontmatter.priority) labels.push(frontmatter.priority);
              if (frontmatter.type) labels.push(frontmatter.type);

              return {
                title,
                body: body.trim(),
                labels: [...new Set(labels)],
                assignees: frontmatter.assignees || [],
                milestone: frontmatter.milestone,
                parent_issue: frontmatter.parent_issue,
                create_sub_issues: frontmatter.create_sub_issues || false
              };
            }

            // Helper: Extract sub-issues from H2 sections
            function extractSubIssues(body, parentLabels) {
              const sections = [];
              const regex = /^##\s+(?:Sub-Issue\s*#?\d*:?\s*)?(.+?)$([\s\S]*?)(?=^##\s|\Z)/gm;
              let match;

              while ((match = regex.exec(body)) !== null) {
                const sectionTitle = match[1].trim();
                const sectionBody = match[2].trim();

                // Extract inline labels like **Labels**: `testing`, `acl`
                const inlineLabels = [];
                const labelMatch = sectionBody.match(/\*\*Labels?\*\*:?\s*[`']?([^`'\n]+)[`']?/i);
                if (labelMatch) {
                  labelMatch[1].split(',').forEach(l => {
                    const label = l.replace(/[`']/g, '').trim();
                    if (label) inlineLabels.push(label);
                  });
                }

                sections.push({
                  title: sectionTitle,
                  body: sectionBody,
                  labels: [...parentLabels, ...inlineLabels]
                });
              }

              return sections;
            }

            // Process each file
            for (const filePath of files) {
              console.log(`\nProcessing: ${filePath}`);

              try {
                const parsed = parseIssueFile(filePath);
                console.log(`  Title: ${parsed.title}`);
                console.log(`  Labels: ${parsed.labels.join(', ')}`);

                if (isDryRun) {
                  console.log('  [DRY RUN] Would create issue');
                  createdIssues.push({ file: filePath, title: parsed.title, dryRun: true });
                  continue;
                }

                // Ensure labels exist
                for (const label of parsed.labels) {
                  await ensureLabel(label);
                }

                // Create the main issue
                const issueResponse = await github.rest.issues.create({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  title: parsed.title,
                  body: parsed.body + `\n\n---\n*Auto-created from [${path.basename(filePath)}](https://github.com/${context.repo.owner}/${context.repo.repo}/blob/${context.sha}/${filePath})*`,
                  labels: parsed.labels,
                  assignees: parsed.assignees
                });

                const issueNumber = issueResponse.data.number;
                console.log(`  Created issue #${issueNumber}`);

                // Handle sub-issues
                if (parsed.create_sub_issues) {
                  const subIssues = extractSubIssues(parsed.body, parsed.labels);
                  for (const sub of subIssues) {
                    for (const label of sub.labels) {
                      await ensureLabel(label);
                    }
                    const subResponse = await github.rest.issues.create({
                      owner: context.repo.owner,
                      repo: context.repo.repo,
                      title: `[${parsed.title}] ${sub.title}`,
                      body: sub.body + `\n\n---\n*Sub-issue of #${issueNumber}*`,
                      labels: sub.labels,
                      assignees: parsed.assignees
                    });
                    console.log(`    Created sub-issue #${subResponse.data.number}: ${sub.title}`);
                  }
                }

                // Link to parent issue if specified
                if (parsed.parent_issue) {
                  await github.rest.issues.createComment({
                    owner: context.repo.owner,
                    repo: context.repo.repo,
                    issue_number: parsed.parent_issue,
                    body: `Sub-issue created: #${issueNumber}`
                  });
                }

                createdIssues.push({
                  file: filePath,
                  title: parsed.title,
                  issueNumber
                });

              } catch (error) {
                console.error(`  Error processing ${filePath}: ${error.message}`);
                errors.push({ file: filePath, error: error.message });
              }
            }

            core.setOutput('created_count', createdIssues.length);
            core.setOutput('created_issues', JSON.stringify(createdIssues));
            core.setOutput('errors', JSON.stringify(errors));

            if (errors.length > 0) {
              core.warning(`${errors.length} file(s) had errors`);
            }

      - name: Move processed files
        if: steps.process.outputs.created_count != '0' && github.event.inputs.dry_run != 'true'
        run: |
          mkdir -p docs/issues/created
          CREATED_ISSUES='${{ steps.process.outputs.created_issues }}'
          echo "$CREATED_ISSUES" | jq -r '.[].file' | while read file; do
            if [ -f "$file" ] && [ ! -z "$file" ]; then
              filename=$(basename "$file")
              timestamp=$(date +%Y%m%d)
              mv "$file" "docs/issues/created/${timestamp}-${filename}"
              echo "Moved: $file -> docs/issues/created/${timestamp}-${filename}"
            fi
          done

      - name: Commit moved files
        if: steps.process.outputs.created_count != '0' && github.event.inputs.dry_run != 'true'
        run: |
          git config --local user.email "github-actions[bot]@users.noreply.github.com"
          git config --local user.name "github-actions[bot]"
          git add docs/issues/
          git diff --staged --quiet || git commit -m "chore: move processed issue files to created/ [skip ci]"
          git push

      - name: Add to project board
        if: steps.process.outputs.created_count != '0' && github.event.inputs.dry_run != 'true'
        continue-on-error: true
        uses: actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea # v7
        env:
          PROJECT_URL: ${{ secrets.PROJECT_URL }}
          ADD_TO_PROJECT_PAT: ${{ secrets.ADD_TO_PROJECT_PAT }}
        with:
          github-token: ${{ secrets.ADD_TO_PROJECT_PAT || secrets.GITHUB_TOKEN }}
          script: |
            const projectUrl = process.env.PROJECT_URL;
            if (!projectUrl) {
              console.log('PROJECT_URL not configured, skipping project board');
              return;
            }

            const createdIssues = JSON.parse('${{ steps.process.outputs.created_issues }}');
            console.log(`Would add ${createdIssues.length} issues to project board`);
            // Note: Actual project board integration requires GraphQL API
            // The actions/add-to-project action handles this automatically
            // for issues created via normal flow

      - name: Summary
        if: always()
        run: |
          echo "## Docs to Issues Summary" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY

          CREATED='${{ steps.process.outputs.created_issues }}'
          ERRORS='${{ steps.process.outputs.errors }}'
          DRY_RUN='${{ github.event.inputs.dry_run }}'

          if [ "$DRY_RUN" = "true" ]; then
            echo "🔍 **Dry Run Mode** - No issues were actually created" >> $GITHUB_STEP_SUMMARY
            echo "" >> $GITHUB_STEP_SUMMARY
          fi

          echo "### Created Issues" >> $GITHUB_STEP_SUMMARY
          if [ -n "$CREATED" ] && [ "$CREATED" != "[]" ]; then
            echo "$CREATED" | jq -r '.[] | "- \(.title) (#\(.issueNumber // "dry-run"))"' >> $GITHUB_STEP_SUMMARY
          else
            echo "_No issues created_" >> $GITHUB_STEP_SUMMARY
          fi

          echo "" >> $GITHUB_STEP_SUMMARY
          echo "### Errors" >> $GITHUB_STEP_SUMMARY
          if [ -n "$ERRORS" ] && [ "$ERRORS" != "[]" ]; then
            echo "$ERRORS" | jq -r '.[] | "- ❌ \(.file): \(.error)"' >> $GITHUB_STEP_SUMMARY
          else
            echo "_No errors_" >> $GITHUB_STEP_SUMMARY
          fi
```

---

## 4. File Conversion Templates

### 4.1 Updated ACL-testing-tasks.md (Example)

```yaml
---
title: "ACL: Test and validate ACL changes (feature/beta-release)"
labels:
  - testing
  - needs-triage
  - acl
  - regression
priority: high
milestone: "v0.2.0-beta.2"
create_sub_issues: false
---

# ACL: Test and validate ACL changes (feature/beta-release)

**Repository:** Wikid82/Charon
**Branch:** feature/beta-release

## Purpose

Create a tracked issue and sub-tasks to validate ACL-related changes...

[rest of content unchanged]
```

### 4.2 Updated bulk-acl-subissues.md (Example with Sub-Issues)

```yaml
---
title: "Bulk ACL Testing - Sub-Issues"
labels:
  - testing
  - manual-testing
  - bulk-acl
priority: medium
milestone: "v0.2.0-beta.2"
create_sub_issues: true
---

# Bulk ACL Testing - Sub-Issues

## Basic Functionality Testing

**Labels**: `testing`, `manual-testing`, `bulk-acl`

Test the core functionality of the bulk ACL feature...

## ACL Removal Testing

**Labels**: `testing`, `manual-testing`, `bulk-acl`

Test the ability to remove access lists...

[each ## section becomes a sub-issue]
```

### 4.3 Template for New Issue Files

Create `docs/issues/_TEMPLATE.md`:

```yaml
---
# REQUIRED: Issue title
title: "Your Issue Title"

# OPTIONAL: Labels to apply (will be created if missing)
labels:
  - feature       # feature, bug, enhancement, testing, documentation
  - backend       # backend, frontend, ui, security, caddy, database

# OPTIONAL: Priority (creates matching label)
priority: medium  # critical, high, medium, low

# OPTIONAL: Milestone name
milestone: "v0.2.0-beta.2"

# OPTIONAL: GitHub usernames to assign
assignees: []

# OPTIONAL: Parent issue number for linking
# parent_issue: 42

# OPTIONAL: Parse ## sections as separate sub-issues
# create_sub_issues: true
---

# Issue Title

## Description

Clear description of the issue or feature request.

## Tasks

- [ ] Task 1
- [ ] Task 2
- [ ] Task 3

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Related Issues

- #XX - Related issue description

---

*Issue specification created: YYYY-MM-DD*
```

---

## 5. Files to Update

### 5.1 Existing docs/issues Files Needing Frontmatter

| File | Action Needed |
|------|---------------|
| `ACL-testing-tasks.md` | Add frontmatter with extracted metadata |
| `Additional_Security.md` | Add frontmatter, convert to issue format |
| `bulk-acl-subissues.md` | Add frontmatter with `create_sub_issues: true` |
| `bulk-acl-testing.md` | Add frontmatter with extracted metadata |
| `hectate.md` | Add frontmatter (feature spec) |
| `orthrus.md` | Add frontmatter (feature spec) |
| `plex-remote-access-helper.md` | Add frontmatter, already has metadata section |
| `rotating-loading-animations.md` | Add frontmatter, extract inline metadata |

### 5.2 New Files to Create

| File | Purpose |
|------|---------|
| `.github/workflows/docs-to-issues.yml` | Main workflow |
| `docs/issues/_TEMPLATE.md` | Template for new issues |
| `docs/issues/created/.gitkeep` | Directory for processed files |
| `docs/issues/README.md` | Documentation for contributors |

---

## 6. Directory Structure

```
docs/
├── issues/
│   ├── README.md                    # How to create issue files
│   ├── _TEMPLATE.md                 # Template for new issues
│   ├── created/                     # Processed files moved here
│   │   └── .gitkeep
│   ├── ACL-testing-tasks.md         # Pending (needs frontmatter)
│   ├── Additional_Security.md       # Pending
│   ├── bulk-acl-subissues.md        # Pending
│   └── ...
```

---

## 7. Duplicate Prevention Strategy

### 7.1 File-Based Tracking

- **Processed files move to `docs/issues/created/`** with timestamp prefix
- **Workflow excludes `created/` directory** from processing
- **Git history provides audit trail** of when files were converted

### 7.2 Issue Tracking

Each created issue includes footer:

```markdown
---
*Auto-created from [filename.md](link-to-source-commit)*
```

### 7.3 Manual Override

- Use `workflow_dispatch` with specific file path to reprocess
- Files in `created/` can be moved back for reprocessing

---

## 8. Project Board Integration

### 8.1 Automatic Addition

The workflow leverages the existing `auto-add-to-project.yml` which triggers on `issues: [opened]`.

When the workflow creates an issue, the auto-add workflow automatically adds it to the project board (if `PROJECT_URL` secret is configured).

### 8.2 Manual Configuration

If not using `auto-add-to-project.yml`, configure in the docs-to-issues workflow:

```yaml
- name: Add to project
  uses: actions/add-to-project@244f685bbc3b7adfa8466e08b698b5577571133e # v1.0.2
  with:
    project-url: ${{ secrets.PROJECT_URL }}
    github-token: ${{ secrets.ADD_TO_PROJECT_PAT }}
```

---

## 9. Testing Strategy

### 9.1 Dry Run Testing

```bash
# Manually trigger with dry_run=true
gh workflow run docs-to-issues.yml -f dry_run=true

# Test specific file
gh workflow run docs-to-issues.yml -f dry_run=true -f file_path=docs/issues/ACL-testing-tasks.md
```

### 9.2 Local Testing

```bash
# Parse frontmatter locally
node -e "
const matter = require('gray-matter');
const fs = require('fs');
const result = matter(fs.readFileSync('docs/issues/ACL-testing-tasks.md', 'utf8'));
console.log(JSON.stringify(result.data, null, 2));
"
```

### 9.3 Validation Checklist

- [ ] Frontmatter parses correctly for all files
- [ ] Labels are created when missing
- [ ] Issue titles are correct
- [ ] Issue bodies include full content
- [ ] Sub-issues are created correctly (if `create_sub_issues: true`)
- [ ] Files move to `created/` after processing
- [ ] Auto-add-to-project triggers for new issues
- [ ] Commit message includes `[skip ci]` to prevent loops

---

## 10. Implementation Phases

### Phase 1: Setup (15 min)

1. Create `.github/workflows/docs-to-issues.yml`
2. Create `docs/issues/created/.gitkeep`
3. Create `docs/issues/_TEMPLATE.md`
4. Create `docs/issues/README.md`

### Phase 2: File Migration (30 min)

1. Add frontmatter to existing files (in order of priority)
2. Test with dry_run mode
3. Create one test issue to verify

### Phase 3: Validation (15 min)

1. Verify issue creation
2. Verify label creation
3. Verify project board integration
4. Verify file move to `created/`

---

## 11. Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Duplicate issues | Low | File move + exclude pattern |
| Label spam | Low | Predefined color map |
| Rate limiting | Medium | Process files sequentially |
| Malformed frontmatter | Medium | Try-catch with error logging |
| Project board auth failure | Low | `continue-on-error: true` |

---

## 12. Definition of Done

- [ ] Workflow file created and committed
- [ ] All existing docs/issues files have valid frontmatter
- [ ] Dry run succeeds with no errors
- [ ] At least one test issue created successfully
- [ ] File moved to `created/` after processing
- [ ] Labels created automatically
- [ ] Project board integration verified (if configured)
- [ ] Documentation in `docs/issues/README.md`

---

## Appendix A: Label Color Reference

```javascript
const labelColors = {
  // Priority
  critical: 'B60205',
  high: 'D93F0B',
  medium: 'FBCA04',
  low: '0E8A16',

  // Milestone
  alpha: '5319E7',
  beta: '0052CC',
  'post-beta': '006B75',

  // Type
  feature: 'A2EEEF',
  bug: 'D73A4A',
  enhancement: '84B6EB',
  documentation: '0075CA',
  testing: 'BFD4F2',

  // Component
  backend: '1D76DB',
  frontend: '5EBEFF',
  ui: '7057FF',
  security: 'EE0701',
  caddy: '1F6FEB',
  database: '006B75',

  // Custom
  'needs-triage': 'FBCA04',
  acl: 'C5DEF5',
  'bulk-acl': '006B75',
  'manual-testing': 'BFD4F2',
  regression: 'D93F0B',
  plus: 'FFD700'
};
```

---

## Appendix B: Frontmatter Conversion Examples

### B.1 hectate.md (Feature Spec)

```yaml
---
title: "Hecate: Tunnel & Pathway Manager"
labels:
  - feature
  - backend
  - architecture
priority: medium
milestone: "post-beta"
type: feature
---
```

### B.2 orthrus.md (Feature Spec)

```yaml
---
title: "Orthrus: Remote Socket Proxy Agent"
labels:
  - feature
  - backend
  - architecture
priority: medium
milestone: "post-beta"
type: feature
---
```

### B.3 plex-remote-access-helper.md

```yaml
---
title: "Plex Remote Access Helper & CGNAT Solver"
labels:
  - beta
  - feature
  - plus
  - ui
  - caddy
priority: medium
milestone: "beta"
type: feature
parent_issue: 44
---
```

### B.4 rotating-loading-animations.md

```yaml
---
title: "Enhancement: Rotating Thematic Loading Animations"
labels:
  - enhancement
  - frontend
  - ui
priority: low
type: enhancement
---
```

### B.5 Additional_Security.md

```yaml
---
title: "Additional Security Threats to Consider"
labels:
  - security
  - documentation
  - architecture
priority: medium
type: documentation
---
```
