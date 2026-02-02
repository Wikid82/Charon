# Issue: Sync .version file with Git tag

## Title
Sync .version file with latest Git tag

## Labels
- `housekeeping`
- `versioning`
- `good first issue`

## Priority
**Low** (Non-blocking, cosmetic)

## Description

The `.version` file is out of sync with the latest Git tag, causing pre-commit warnings during development.

### Current State
- **`.version` file:** `v0.15.3`
- **Latest Git tag:** `v0.16.8`

### Impact
- Pre-commit hook `check-version-tag` fails with warning:
  ```
  Check .version matches latest Git tag..................Failed
  ERROR: .version (v0.15.3) does not match latest Git tag (v0.16.8)
  ```
- Does NOT block builds or affect runtime behavior
- Creates noise in pre-commit output
- May confuse contributors about the actual version

### Expected Behavior
- `.version` file should match the latest Git tag
- Pre-commit hook should pass without warnings
- Version information should be consistent across all sources

## Steps to Reproduce

1. Clone the repository
2. Run pre-commit checks:
   ```bash
   pre-commit run --all-files
   ```
3. Observe warning: `.version (v0.15.3) does not match latest Git tag (v0.16.8)`

## Proposed Solution

### Option 1: Update .version to match latest tag (Quick Fix)

```bash
# Fetch latest tags
git fetch --tags

# Get latest tag
LATEST_TAG=$(git describe --tags --abbrev=0)

# Update .version file
echo "$LATEST_TAG" > .version

# Commit the change
git add .version
git commit -m "chore: sync .version file with latest Git tag ($LATEST_TAG)"
```

### Option 2: Automate version syncing (Comprehensive)

**Create a GitHub Actions workflow** to automatically sync `.version` with Git tags:

```yaml
name: Sync Version File

on:
  push:
    tags:
      - 'v*'

jobs:
  sync-version:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Update .version file
        run: |
          echo "${{ github.ref_name }}" > .version

      - name: Commit and push
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add .version
          git commit -m "chore: sync .version to ${{ github.ref_name }}"
          git push
```

### Option 3: Remove .version file (Simplest)

If `.version` is not used in the codebase:

1. Delete `.version` file
2. Remove or update pre-commit hook to not check version sync
3. Use Git tags as the single source of truth for versioning

## Investigation Required

Before implementing, verify:

1. **Where is `.version` used?**
   ```bash
   # Search codebase for references
   grep -r "\.version" --exclude-dir=node_modules --exclude-dir=.git
   ```

2. **Is `.version` read by the application?**
   - Check backend code for version file reads
   - Check build scripts
   - Check documentation generation

3. **Why is there a version discrepancy?**
   - Was `.version` manually updated?
   - Was it missed during release tagging?
   - Is there a broken sync process?

## Acceptance Criteria

- [ ] `.version` file matches latest Git tag (`v0.16.8`)
- [ ] Pre-commit hook `check-version-tag` passes without warnings
- [ ] Version consistency verified across all sources:
  - [ ] `.version` file
  - [ ] Git tags
  - [ ] `package.json` (if applicable)
  - [ ] `go.mod` (if applicable)
  - [ ] Documentation
- [ ] If automated workflow is added:
  - [ ] Workflow triggers on tag push
  - [ ] Workflow updates `.version` correctly
  - [ ] Workflow commits change to main branch

## Related Files

- `.version` — Version file (needs update)
- `.pre-commit-config.yaml` — Pre-commit hook configuration
- `CHANGELOG.md` — Version history
- `.github/workflows/` — Automation workflows (if Option 2 chosen)

## References

- **Pre-commit hook:** `check-version-tag`
- **QA Report:** `docs/reports/qa_report.md` (section 11.3)
- **Implementation Plan:** `docs/plans/current_spec.md`

## Priority Justification

**Why Low Priority:**
- Does not block builds or deployments
- Does not affect runtime behavior
- Only affects developer experience (pre-commit warnings)
- No security implications
- No user-facing impact

**When to address:**
- During next maintenance sprint
- When preparing for next release
- When cleaning up technical debt
- As a good first issue for new contributors

## Estimated Effort

- **Option 1 (Quick Fix):** 5 minutes
- **Option 2 (Automation):** 30 minutes
- **Option 3 (Remove file):** 15 minutes + investigation

---

**Created:** February 2, 2026
**Discovered During:** Docker build fix QA verification
**Reporter:** GitHub Copilot QA Agent
**Status:** Draft (not yet created in GitHub)
