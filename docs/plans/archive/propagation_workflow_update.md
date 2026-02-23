# Plan: Refine Propagation Workflow to Enforce Strict Hierarchy (Pittsburgh Model)

## 1. Introduction
This plan outlines the update of the `.github/workflows/propagate-changes.yml` workflow. The goal is to enforce a strict hierarchical propagation strategy ("The Pittsburgh Model") where changes flow downstream from `main` to `development`, and then from `development` to leaf branches (`feature/*`, `hotfix/*`). This explicitly prevents "loop-backs" and direct updates from `main` to feature branches.

## 2. Methodology & Rules
**The Pittsburgh Model (Strict Hierarchy):**

1.  **Rule 1 (The Ohio River)**: `main` **ONLY** propagates to `development`.
    -   *Logic*: `main` is the stable release branch. Changes here (hotfixes, releases) must flow into `development` first.
    -   *Constraint*: `main` must **NEVER** propagate directly to `feature/*` or `hotfix/*`.

2.  **Rule 2 (The Point)**: `development` is the **ONLY** branch that propagates to leaf branches.
    -   *Logic*: `development` is the source of truth for active work. It aggregates `main` changes plus ongoing development.
    -   *Targets*: `feature/*` and `hotfix/*`.

3.  **Rule 3 (Loop Prevention)**: Determine the "source" PR to prevent re-propagation.
    -   *Problem*: When `feature/A` merges into `development`, we must not open a PR from `development` back to `feature/A`.
    -   *Mechanism*: Identify the source branch of the commit triggering the workflow and exclude it from targets.

## 3. Workflow Design

### 3.1. Branching Strategy Logic

| Trigger Branch | Source | Target(s) | Logic |
| :--- | :--- | :--- | :--- |
| `main` | `main` | `development` | Create PR `main` -> `development` |
| `development` | `development` | `feature/*`, `hotfix/*` | Create PR `development` -> `[leaf]` (Excluding changes source) |
| `feature/*` | - | - | No action (Triggers CI only) |
| `hotfix/*` | - | - | No action (Triggers CI only) |

### 3.2. Logic Updates Needed

**A. Strict Main Enforcement**
- Current logic likely does this, but we will explicitly verify `if (currentBranch === 'main') { propagate('development'); }` and nothing else.

**B. Development Distribution & Hotfix Inclusion**
- Update the branch listing logic to find both `feature/*` AND `hotfix/*` branches.
- Current code only looks for `feature/*`.

**C. Loop Prevention (The "Source Branch" Check)**
- **Trigger**: Script runs on push to `development`.
- **Action**:
  1. Retrieve the Pull Request associated with the commit sha using the GitHub API.
  2. If a merged PR exists for this commit, extract the source branch name (`head.ref`).
  3. Exclude this source branch from the list of propagation targets.

### 3.3. Technical Implementation Details
- **File**: `.github/workflows/propagate-changes.yml`
- **Action**: `actions/github-script`

**Pseudo-Code Update:**
```javascript
// 1. Get current branch
const branch = context.ref.replace('refs/heads/', '');

// 2. Rule 1: Main -> Development
if (branch === 'main') {
    await createPR('main', 'development');
    return;
}

// 3. Rule 2: Development -> Leafs
if (branch === 'development') {
    // 3a. Identify Source (Rule 3 Loop Prevention)
    // NOTE: This runs on push, so context.sha is the commit sha.
    let excludedBranch = null;
    try {
        const prs = await github.rest.repos.listPullRequestsAssociatedWithCommit({
            owner: context.repo.owner,
            repo: context.repo.repo,
            commit_sha: context.sha,
        });
        // Find the PR that was merged
        const mergedPr = prs.data.find(pr => pr.merged_at);
        if (mergedPr) {
             excludedBranch = mergedPr.head.ref;
             core.info(`Commit derived from merged PR #${mergedPr.number} (Source: ${excludedBranch}). Skipping back-propagation.`);
        }
    } catch (e) {
        core.info('Could not check associated PRs: ' + e.message);
    }

    // 3b. Find Targets
    const branches = await github.paginate(github.rest.repos.listBranches, {
        owner: context.repo.owner,
        repo: context.repo.repo,
    });

    const targets = branches
        .map(b => b.name)
        .filter(b => (b.startsWith('feature/') || b.startsWith('hotfix/')))
        .filter(b => b !== excludedBranch); // Exclude source

    // 3c. Propagate
    core.info(`Propagating to ${targets.length} branches: ${targets.join(', ')}`);
    for (const target of targets) {
        await createPR('development', target);
    }
}
```

## 4. Implementation Steps

1.  **Refactor `main` logic**: Ensure it returns immediately after propagating to `development` to prevent any fall-through.
2.  **Update `development` logic**:
    -   Add `hotfix/` to the filter regex.
    -   Implement the `listPullRequestsAssociatedWithCommit` call to identify the exclusion.
    -   Apply the exclusion to the target list.
3.  **Verify Hierarchy**:
    -   Confirm no path exists for `main` -> `feature/*`.

## 5. Acceptance Criteria
- [ ] Push to `main` creates a PR ONLY to `development`.
- [ ] Push to `development` creates PRs to all downstream `feature/*` AND `hotfix/*` branches.
- [ ] Push to `development` (caused by merge of `feature/A`) does **NOT** create a PR back to `feature/A`.
- [ ] A hotfix merged to `main` flows: `main` -> `development`, then `development` -> `hotfix/active-work` (if any exist).
