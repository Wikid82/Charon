# Docs Workflow Update Plan

## 1. Introduction
The current documentation workflow only validates and deploys on pushes to `main`. This leaves other branches without validation of documentation changes, potentially leading to broken docs being merged. This plan outlines the updates to ensure documentation is built/validated on all relevant branches and PRs, while deployment remains restricted to `main`.

## 2. Research Findings
- **Current File**: `.github/workflows/docs.yml`
- **Build Method**: Uses `npm install -g marked` to convert Markdown to HTML.
- **Deploy Method**: Uses `actions/upload-pages-artifact` and `actions/deploy-pages`.
- **Triggers**: Currently limited to `push: branches: [main]`.

## 3. Technical Specifications

### Workflow Triggers (`on`)
The workflow triggers need to be expanded to cover:
- Pull Requests targeting `main` or `development`.
- Pushes to `main`, `development`, `feature/**`, and `hotfix/**`.

```yaml
on:
  push:
    branches:
      - main
      - development
      - 'feature/**'
      - 'hotfix/**'
    paths:
      - 'docs/**'
      - 'README.md'
      - '.github/workflows/docs.yml'
  pull_request:
    branches:
      - main
      - development
    paths:
      - 'docs/**'
      - 'README.md'
      - '.github/workflows/docs.yml'
  workflow_dispatch:
```

### Concurrency
Update concurrency to be scoped by branch. This allows parallel builds for different feature branches.
Use `cancel-in-progress: true` for all branches except `main` to save resources on rapid fast-forward pushes, but ensure robust deployments for `main`.

```yaml
concurrency:
  group: "pages-${{ github.ref }}"
  cancel-in-progress: ${{ github.ref != 'refs/heads/main' }}
```

### Job Constraints
- **Job `build`**: Should run on all triggers. No changes needed to conditions.
- **Job `deploy`**: Must be restricted to `main` branch pushes only.

```yaml
  deploy:
    name: Deploy to GitHub Pages
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    timeout-minutes: 5
    needs: build
    # ... steps ...
```

## 4. Implementation Tasks
1.  **Modify `.github/workflows/docs.yml`**:
    - Update `on` triggers.
    - Update `concurrency` block with `group: "pages-${{ github.ref }}"` and conditional `cancel-in-progress`.
    - Add `if` condition to `deploy` job.
    - **Fix 404 Link Error**:
        - Replace hardcoded `/charon/` paths in generated HTML navigation with dynamic repository name variable.
        - Use `${{ github.event.repository.name }}` within the workflow to construct the base path, ensuring case-sensitivity compatibility (e.g., `Charon` vs `charon`).

## 5. Acceptance Criteria
- [ ] Pushing to a feature branch triggers the `build` job but skips `deploy`.
- [ ] Multiple feature branch pushes run in parallel (checked via Actions tab).
- [ ] Rapid pushes to the same feature branch cancel previous runs.
- [ ] Opening a PR triggers the `build` job.
- [ ] Pushing to `main` triggers both `build` and `deploy`.
- [ ] Pushing to `main` does not cancel in-progress runs (safe deployment).
