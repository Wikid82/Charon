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
        - Pass the repository name as an environment variable to the run step to avoid mixing GitHub Actions syntax with shell variables inside the heredoc.
        - **Correct Heredoc Usage**: Change the heredoc delimiter from quoted (`'HEADER'`) to unquoted (`HEADER`) to allow shell variable expansion (`${REPO_NAME}`).
        - **Code Snippet**:

```yaml
      - name: 📝 Build documentation site
        # Pass the repository name explicitly as an env var to handle casing (e.g. 'Charon' vs 'charon')
        env:
          REPO_NAME: ${{ github.event.repository.name }}
        run: |
          # ... (previous setup) ...

          # Add simple styling to all HTML files
          for html_file in _site/*.html _site/docs/*.html; do
            if [ -f "$html_file" ] && [ "$html_file" != "_site/index.html" ]; then
              # Add a header with navigation to each page
              # NOTE: using unquoted HEADER to allow expansion of $REPO_NAME
              temp_file="${html_file}.tmp"
              cat > "$temp_file" << HEADER
          <!DOCTYPE html>
          <html lang="en">
          <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>Charon - Documentation</title>
            <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
            <style>
              body { background-color: #0f172a; color: #e2e8f0; }
              nav { background: #1e293b; padding: 1rem; margin-bottom: 2rem; }
              nav a { color: #60a5fa; margin-right: 1rem; text-decoration: none; }
              nav a:hover { color: #93c5fd; }
              main { max-width: 900px; margin: 0 auto; padding: 2rem; }
              a { color: #60a5fa; }
              code { background: #1e293b; color: #fbbf24; padding: 0.2rem 0.4rem; border-radius: 4px; }
              pre { background: #1e293b; padding: 1rem; border-radius: 8px; overflow-x: auto; }
              pre code { background: none; padding: 0; }
            </style>
          </head>
          <body>
            <nav>
              <!-- Use dynamic REPO_NAME for correct GitHub Pages paths -->
              <a href="/${REPO_NAME}/">🏠 Home</a>
              <a href="/${REPO_NAME}/docs/index.html">📚 Docs</a>
              <a href="/${REPO_NAME}/docs/getting-started.html">🚀 Get Started</a>
              <a href="https://github.com/Wikid82/charon">⭐ GitHub</a>
            </nav>
            <main>
          HEADER
              
              # ... (rest of the script) ...
```

## 5. Acceptance Criteria
- [ ] Pushing to a feature branch triggers the `build` job but skips `deploy`.
- [ ] Multiple feature branch pushes run in parallel (checked via Actions tab).
- [ ] Rapid pushes to the same feature branch cancel previous runs.
- [ ] Opening a PR triggers the `build` job.
- [ ] Pushing to `main` triggers both `build` and `deploy`.
- [ ] Pushing to `main` does not cancel in-progress runs (safe deployment).
- [ ] Generated HTML links typically point to `/${REPO_NAME}/...` to work correctly on GitHub Pages subpaths.
