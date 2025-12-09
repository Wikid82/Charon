Overview
--------
This document outlines an investigation and remediation plan for the error message: "Could not build remote workspace index. Could not check the remote index status for this repo." It contains diagnoses, exact checks, reproductions, fixes, CI/security checks, and acceptance criteria.

Summary: Background & likely root causes
--------------------------------------
- Background: Remote workspace index builds (e.g., on GitHub or other code hosting platforms) analyze repository contents to provide search, code navigation, and other services. Index processes can fail due to repository content, metadata, workflows, or platform limitations.
- Likely root causes:
  - Permission issues: index build needs read access or specific tokens (missing PAT or actions secrets, restricted branch protection blocking re-run).
  - Repository size limits: excessive repo size (large binaries, codeql databases, caddy caches) triggers failures.
  - Git LFS misconfiguration: large files stored in Git rather than LFS; remote indexer times out scanning big objects.
  - Cyclic symlinks or malformed symlinks: indexers can hit infinite loops or unsupported references.
  - Large or unsupported binary files: vendor/binaries and artifacts committed directly instead of using artifacts or releases.
  - Missing or stale code scanning artifacts (e.g., codeql-db) present in repo root that confuse the indexer.
  - Malformed .git/config or broken submodules: submodule references or malformed remote URL prevents indexer from checking status.
  - Network issues: temporary outages between indexer service and repo host.
  - GitHub Actions failures: workflows that set up the environment fail due to missing CI keys or secrets.
  - Branch protection / repo policies: preventing GitHub from performing necessary indexing operations.

Files and Locations To Inspect (exact)
-------------------------------------
1. Repository configuration
   - .git/config ([.git/config](.git/config)) — inspect remotes, submodules, and alternate refs
   - .gitattributes ([.gitattributes](.gitattributes)) — look for LFS vs tracked files
   - .gitignore ([.gitignore](.gitignore)) — ensure generated artifacts and codeql-db are ignored
2. CI and workflows
   - .github/workflows/**/* ([.github/workflows](.github/workflows)) — workflows and secrets usage
   - .github/ (branch protection or other settings logged in repo web UI)
3. Code scanning artifacts
   - codeql-db/ (codeql-db-go/, codeql-db-js/) — ensure not committed as code
   - backend/codeql-db/ or backend/data/ — artifacts that increase repo size and confuse indexer
4. Build & deploy config
   - Dockerfile (Dockerfile) — check for COPY of large files
   - docker-compose*.yml (docker-compose.yml, docker-compose.local.yml ) — services and volumes
5. Platform / metadata
   - .vscode/* (workspace settings) — local workspace mapping
   - go.work (go.work) — ensures workspace go settings are correct
6. Frontend & package manifests
   - frontend/package.json (frontend/package.json) — scripts and postinstall tasks that may run CI or build steps
   - package.json (root/package.json) — library or workspace settings affecting index size
7. Binary caches & artifacts
   - data/ (data/), backend/data/ — DB files, caddy files, and caches
   - tools/ (tools/) — large binaries or vendor files committed for local dev
8. Git LFS and hooks
   - .git/hooks/* — check for LFS pre-commit/pre-push hooks
9. GitHub admin side
   - Branch protection settings in GitHub UI (branches) — check protected branches and required status checks
10. Logs & scan results
   - .github/workflows action logs (in GitHub Actions run view or `gh` CLI)
   - backend/test-output.txt (backend/test-output.txt) and other preexisting logs

Commands & Logs To Collect (reproduction & evidence gathering)
--------------------------------------------------------------
- Local repo inspection
  1. `git status --porcelain --ignored` — show ignored & modified files
  2. `git fsck --full` — scan object store for corruption
  3. `git count-objects -vH` — get repo size (packed/unpacked).
  4. `du -sh .` and `du -sh $(git rev-parse --show-toplevel)/*` — check workspace size from file system
  5. `git rev-list --objects --all | sort -k 2 > allfiles.txt` then `git rev-list --objects --all | sort -k 2 | tail -n 50` — find large commits/objects
  6. `git verify-pack -v .git/objects/pack/pack-*.idx | sort -k 3 -n | tail -n 50` — inspect object packs
  7. `git lfs ls-files` and `git lfs env` — check LFS tracked files and env
  8. `find . -type f -size +100M -exec ls -lh {} \;` — detect large files in working tree
  9. `rg -S "codeql-db|codeql database|codeql" -n` — ripgrep for CodeQL references
 10. `git submodule status --recursive` — check submodules
  11. `git config --list --show-origin` — confirm config and values (e.g., LFS config)

- Reproducing remote run locally / in GH Actions
  12. Use `gh` CLI: `gh run list --repo owner/repo` then `gh run view RUN_ID --log --repo owner/repo` to collect logs
  13. Rerun the failing index action: `gh run rerun RUN_ID --repo owner/repo` (requires permissions)
  14. Recreate CodeQL database locally to test: `codeql database create codeql-db --language=go --command='cd backend && go test ./... -c'` then `codeql database analyze codeql-db codeql-custom-queries-go` to check for broken DBs
  15. `docker build --no-cache -t charon:local .` and `docker run --rm charon:local` to catch missing `git` or LFS artifacts copied into images

- Network & permissions
  16. Check PAT and ACTIONS tokens: `gh auth status` and `gh secret list --repo owner/repo` and verify repository secrets usage in workflows
  17. Verify branch protection policies via `gh api repos/:owner/:repo/branches/:branch` or through the GitHub UI
  18. Use `curl -I https://api.github.com/repos/:owner/:repo` to confirm GitHub's API reachability

Steps To Gather Evidence (sequence)
---------------------------------
1. Triage: Run the local repo inspections above (commands 1–11). Save outputs to `scripts/diagnostics/repo_health.OUTPUT`.
2. Confirm large files or code scan DBs are present: `find . -type d -name "codeql-db*" -o -name "db*" -o -name "node_modules" -o -name "vendor"`.
3. Inspect `git lfs ls-files` and `git lfs env` for LFS misconfig.
4. Collect CI history and logs for failing runs using `gh run view --log` for the action(s) that produce the index error.
5. Verify permission and branch protections: `gh api` queries & `gh secret list --repo`.
6. Verify Dockerfile and workflow steps that might include copying large artifacts and causing indexer timeouts.

Short-Term Mitigations (quick steps)
-----------------------------------
1. Disable or re-run failing index builds with proper permissions: `gh run rerun RUN_ID --repo owner/repo`.
2. Move large artifacts out of the repo: create a new docs/ or data/ to store metadata and put `codeql-db*`, `tools/`, `data/`, and `backend/data/` into `.gitignore` and `.gitattributes` to exclude them from the index.
3. Add `codeql-db` to `.gitignore` and create `.gitattributes` entries to ensure no binary files are tracked without LFS:
   - .gitattributes additions: `codeql-db/** !binary
   - Update `.gitignore`: add `/codeql-db`, `/data`, `/backend/data`, `/.vscode/*/`.
4. Re-run the index build once large files are removed.

Medium-Term Fixes
-----------------
1. Git filter-repo/BFG: If large files are committed, remove them from history using `git filter-repo` or BFG and force-push a cleaned branch: `git filter-repo --strip-blobs-bigger-than 50M`.
2. Convert large tracks to Git LFS for binary files: `git lfs track "*.iso"` and `git lfs track "*.db"` and `git add .gitattributes && git commit -m "Track binaries in LFS" && git push`.
3. Prevent accidental artifacts in the future with a pre-commit hook (add to `scripts/pre-commit-hooks` and reference in `.git/hooks` or pre-commit framework): run `pre-commit` rule to enforce `max-file-size` and LFS checks.
4. Add GH actions health-check workflow (e.g., `.github/workflows/repo-health.yml`) that runs a small script to check for large files, LFS config, and codeql-db folders and opens an issue if thresholds are exceeded.
5. Document LFS / large file policy in `docs/` (e.g., `docs/getting-started.md` and `docs/features.md`) and add codeowner references.

Longer-Term Hardening
---------------------
1. Automate periodic repo health checks (monthly) via GH Actions to run `git count-objects -v`, `find -size +` and warn maintainers.
2. Add a repository dashboard for `codeql-db` and other artifacts using `scripts/` and a GitHub Action to report stats to PRs or issues.
3. Harden the remote indexing process by requesting GitHub support if the issue is intermittent and caused by GitHub's indexer failure.
4. Add a `scripts/ci_pre_check.sh` script to run on PR open that checks `git fsck`, LFS, and ensures `codeql-db` is not committed.
5. Add a `scripts/repo_health_check.sh` file and include it in `.vscode/tasks.json` and as a GH Action to be optionally invoked by maintainers.

Recommended Fixes (exact files & edits, tests to run, CI checks)
---------------------------------------------------------------
1. Add `.gitignore` and `.gitattributes` updates
   - Files to edit: `.gitignore`, `.gitattributes`.
   - Steps:
     - Add `/codeql-db`, `/codeql-db-*`, `/.vscode/`, `/backend/data`, `/data`, `/node_modules` to `.gitignore`.
     - Add `*.db filter=lfs diff=lfs merge=lfs -text` to `.gitattributes` for DB files and `codeql-db/** binary` to mark codeql files as binary.
     - Commit and push.
   - Tests & checks:
     - Locally run `git status --ignored` to ensure these are now ignored.
     - Run `git lfs ls-files` to ensure large files are LFS tracked if intended.
     - CI: Ensure `pre-commit` checks pass, run `gh run view` to verify indexing.

2. Add small repo-health GH Action
   - Files to add: `.github/workflows/repo-health.yml`, `scripts/repo_health_check.sh`.
   - Steps:
     - Implement `scripts/repo_health_check.sh` that runs `git count-objects -vH`, `find . -size +100M` and `git fsck` and prints a short JSON summary.
     - Add `repo-health.yml` with a scheduled trigger and PR check to run the script.
   - Tests & checks:
     - Run `bash scripts/repo_health_check.sh` locally. Ensure it exits 0 when checks are clean.
     - CI: Ensure the workflow runs and reports results in the Actions tab.

3. Build-time protections for codeql artifacts
   - Files to edit: `.github/workflows/ci.yml` (or equivalent CI) and `.gitattributes` / `.gitignore`.
   - Steps:
     - Remove `codeql-db` directories from CI cache and artifact paths; don't commit them.
     - Ensure CodeQL analysis workflow uses the `actions/cache` and `actions/upload-artifact` correctly, not storing DBs in the repo.
   - Tests & checks:
     - Re-run `gh actions` CodeQL workflow: `gh run rerun RUN_ID --repo owner/repo` and verify action no longer stores DBs as code.

4. Pre-commit hook for large files & LFS enforcement
   - Files to add: `scripts/pre-commit-hooks/check-large-file.sh`, and enable via `.pre-commit-config.yaml` or `scripts/pre-commit-install.sh`.
   - Steps:
     - Implement a hook to fail commits larger than 50MB unless tracked in LFS.
     - Add to `pre-commit` config and install in the repo.
   - Tests & checks:
     - Attempt to commit a test large file > 50MB to verify the commit is rejected unless LFS tracked.
     - CI: Add a PR check running `pre-commit` to ensure commits follow policy.

5. CI policy verification for branches
   - Files / settings to revise: .github/workflows for runner permissions, branch protection settings via GitHub UI
   - Steps:
     - Confirm user-level or organization-level `actions` and `workflows` permissions allow required actions to run indexers.
     - Modify workflow triggers: ensure that `pull_request` and `push` do not include large artifacts or directories.
   - Tests & checks:
     - Open PR with `scripts/` changes to trigger the updated workflows; verify that they run and pass.

6. Automated Monitoring & Alerts
   - Files to add: `.github/workflows/monitor-repo.yml`, `scripts/repo-monitor.sh`.
   - Steps:
     - Implement periodic monitoring workflow to run repo health checks and open an issue or send a slack message when thresholds crossed.
   - Tests & checks:
     - Local run and scheduled run for the workflow to prove the alert state.

7. Documentation updates
   - Files to update: `docs/getting-started.md`, `docs/features.md`, `docs/security.md`, CONtributing.md
   - Steps:
     - Add guidelines for how to store large artifacts, a policy to use Git LFS, instructions on running `scripts/repo_health_check.sh`.
   - Tests & checks:
     - Verify that docs references builds and CI pass with updated instructions.

8. CI Integration Tests to validate fixes
   - Files to add/edit: `.github/workflows/ci.yml`, `backend` build scripts, `frontend` script checks
   - Steps:
     - Add a `ci.yml` step to run `bash scripts/repo_health_check.sh`, `go test ./...` and `npm run build` (frontend) as a gating check.
   - Tests & checks:
     - Ensure `go build`, `go test`, and `npm run build` pass in CI after changes.

9. Forced cleanup and migration of large objects (if necessary)
   - Files to change: none—these are history-edit operations
   - Steps:
     - If large files are present and the repo will not admit LFS for them, use `git filter-repo --strip-blobs-bigger-than 50M` or BFG and push to a cleaned branch.
     - Recreate workflows or branch references as needed after forced push.
   - Tests & checks:
     - Run `git count-objects -vH` before/after and verify the pack size decreases significantly.

10. Validate GH Actions & secrets
   - Files to inspect: `.github/workflows/*` and GitHub repo settings (secrets)
   - Steps:
     - Ensure that required secrets, PATs, or `GITHUB_TOKEN` usage is correct; verify `actions/checkout` uses LFS fetch: `actions/checkout@v2` with fetch-depth: 0 and proper `lfs` enabled.
   - Tests & checks:
     - Rerun an action to ensure `git lfs ls-files` lists expected 4-5 known files and that the `gh run` does not fail with read errors.

Phased Work Plan
-----------------
Phase 1 — Triage & evidence collection (2-3 days)
- Tasks:
  1. Run all diagnostic commands and collect output (`scripts/diagnostics/repo_health.OUTPUT`).
  2. Collect failing GH Action run logs and CodeQL run logs via `gh run view`.
  3. Determine whether the failure is reproducible or intermittent.
- Acceptance criteria:
  - Have a full diagnostic report (PAIR) that identifies a top-2 likely causes.
  - Have the failing Action ID or workflow file referenced.

Phase 2 — Short-term fixes & re-run (1-2 days)
- Tasks:
  1. Add immediate `.gitignore` and `.gitattributes` protections for known artifacts.
  2. Add `scripts/repo_health_check.sh`, a `.github/workflows/repo-health.yml` workflow, and pre-commit LFS checks.
  3. Re-run GH Actions and index builds to check for improvement.
- Acceptance criteria:
  - Repo health workflow runs on PRs and schedule; the health check succeeds or reports actionable items.
  - Index build does not fail due to size or missing LFS objects on re-run.
  - No unexpected artifacts are present in commits.
  - Pre-commit hooks block large files that are not tracked by Git LFS.

**Status:** Short-term fixes implemented (gitattributes, pre-commit LFS check, health script, scheduled workflow).

Phase 3 — Medium-term fixes (2-5 days)
- Tasks:
  1. Add `pre-commit` hooks and a# CrowdSec Hub Presets Sync & Apply Plan (feature/beta-release)

## 🚨 CI/CD Incident Report - Run 20046135423-29 (2025-12-08 23:20 UTC)

**Status:** ALL BUILDS FAILING on feature/beta-release
**Trigger:** Push of commit 571a61a (CrowdSec cscli installation)
**Impact:** Docker publish blocked, codecov upload failed, all integration tests skipped

### Root Causes Identified

#### 1. **CRITICAL: Missing Frontend File** `frontend/src/data/crowdsecPresets.ts`
- **Evidence:** TypeScript compilation fails in Docker build, frontend tests, and WAF integration
- **Error:** `Cannot find module '../data/crowdsecPresets' or its corresponding type declarations`
- **Affected Jobs:**
  - Run 20046135429 (Docker Build) - exit code 2 at Dockerfile:47
  - Run 20046135423 (Frontend Codecov) - 2 test suites failed
  - Run 20046135424 (WAF Integration) - Docker build failed
  - Run 20046135426 (Quality Checks - Frontend) - test failures
- **Files Importing Missing Module:**
  - `frontend/src/pages/CrowdSecConfig.tsx:17`
  - `frontend/src/pages/__tests__/CrowdSecConfig.spec.tsx:13`
  - `frontend/src/pages/__tests__/CrowdSecConfig.test.tsx` (indirect via CrowdSecConfig.tsx)
- **Type Errors Cascade:**
  - `CrowdSecConfig.tsx(86,52): error TS7006: Parameter 'preset' implicitly has an 'any' type`
  - `CrowdSecConfig.tsx(92-96): error TS2339: Property 'title'|'description'|'content'|'tags'|'warning' does not exist on type '{}'`
  - 10 TypeScript errors total prevent npm build completion
- **Git History:** File never existed in repository; referenced in current_spec.md line 6 but never committed
- **Remediation:** Create `frontend/src/data/crowdsecPresets.ts` with `CROWDSEC_PRESETS` constant and `CrowdsecPreset` type export

#### 2. **Backend Coverage Below Threshold** (84.8% < 85.0% required)
- **Evidence:** Go test suite passes all tests but coverage check fails
- **Error:** `Coverage 84.8% is below required 85% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)`
- **Affected Job:** Run 20046135423 (Backend Codecov Upload) - exit code 1
- **Impact:** Codecov upload skipped, quality gate not met
- **Analysis:** Recent commits added CrowdSec hub sync code without corresponding unit tests
- **Likely Contributors:**
  - Commit be2900b: "add HUB_BASE_URL configuration and enhance CrowdSec hub sync"
  - Commit 571a61a: "install CrowdSec CLI (cscli) in Docker runtime"
  - New code in `backend/internal/crowdsec/` lacks test coverage
- **Remediation Options:**
  1. Add unit tests for new CrowdSec hub sync functions to reach 85%+ coverage
  2. Temporarily lower threshold via `CHARON_MIN_COVERAGE=84` (not recommended for merge)
  3. Exclude untested experimental code from coverage calculation until implementation complete

#### 3. **Frontend Test Failures** (2 test suites failed, 587 tests passed)
- **Evidence:** Vitest reports 2 failed suites due to missing module
- **Affected Job:** Run 20046135423 (Frontend Codecov Upload) & Run 20046135426 (Quality Checks)
- **Failed Suites:**
  - `src/pages/__tests__/CrowdSecConfig.spec.tsx`
  - `src/pages/__tests__/CrowdSecConfig.test.tsx`
- **Root Cause:** Same as #1 - missing `crowdsecPresets.ts` file
- **Consequence:** Frontend coverage calculation incomplete, 587 other tests pass

#### 4. **Docker Multi-Arch Build Failure** (linux/amd64, linux/arm64)
- **Evidence:** Build canceled at frontend stage with TypeScript errors
- **Affected Job:** Run 20046135429 (Docker Build, Publish & Test)
- **Build Stages:**
  - Stage `frontend-builder 6/6` failed during `npm run build`
  - Stages `backend-builder` canceled due to frontend failure
  - No images pushed to ghcr.io, Trivy scan skipped
- **Root Cause:** Same as #1 - TypeScript compilation blocked by missing module
- **Downstream Impact:** Test Docker Image job skipped (no image available)

#### 5. **WAF Integration Tests Skipped**
- **Evidence:** Docker build failed before tests could run
- **Affected Job:** Run 20046135424 (WAF Integration Tests)
- **Build Step Failure:** Same TypeScript errors at Dockerfile:47
- **Container Status:** `charon-debug` container never created
- **Root Cause:** Same as #1 - build precondition not met

### Are These Fixed by Recent Commits?

**NO** - Analysis of commits since 571a61a (the triggering commit at 2025-12-08 23:19:38Z):
- Commit 8f48e03: Merge development → feature/beta-release (no fixes)
- Commit 32ed8bc: Merge PR #332 development → feature/beta-release (no fixes)
- **Latest commit on feature/beta-release:** 32ed8bc (2025-12-09 00:26:07Z)
- **Missing file still not present** in workspace or git history
- **Coverage issue unaddressed** - no new tests added

### Required Remediation Steps

#### **IMMEDIATE (blocks all CI):**
1. **Create Missing Frontend File**
   ```bash
   # Create frontend/src/data/crowdsecPresets.ts with structure:
   export interface CrowdsecPreset {
     slug: string;
     title: string;
     description: string;
     content: string;
     tags: string[];
     warning?: string;
   }
   export const CROWDSEC_PRESETS: CrowdsecPreset[] = [
     // Populate from backend/internal/crowdsec/presets.go or empty array
   ];
   ```
2. **Verify TypeScript Compilation**
   ```bash
   cd frontend && npm run build
   cd frontend && npm run test:ci
   ```

#### **REQUIRED (for merge):**
3. **Add Backend Unit Tests for CrowdSec Hub Sync**
   - Target files: `backend/internal/crowdsec/hub_sync.go`, `hub_cache.go`
   - Create: `backend/internal/crowdsec/hub_sync_test.go`, `hub_cache_test.go`
   - Achieve: ≥85% coverage threshold
4. **Run Full CI Validation**
   ```bash
   # Backend
   cd backend && go test ./... -v -coverprofile=coverage.txt
   # Frontend
   cd frontend && npm run test:ci
   # Docker
   docker build --platform linux/amd64 -t charon:test .
   ```

#### **OPTIONAL (technical debt):**
5. **Update Documentation**
   - Fix docs/plans/current_spec.md line 6 reference to non-existent file
   - Add troubleshooting entry for missing preset file scenario
6. **Add Pre-commit Hook**
   - Validate TypeScript imports resolve before commit
   - Block commits with missing module references

### Prevention Measures
- **Pre-commit validation:** TypeScript type checking must pass (`tsc --noEmit`)
- **Coverage enforcement:** CI should fail immediately when coverage drops below threshold
- **Integration test gating:** Block merge if Docker build fails on any platform
- **Module existence checks:** Lint for import statements referencing non-existent files
- **Test coverage for new features:** Require tests in same commit as feature code

---

## Current State (what exists today)
- Backend: [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) exposes `ListPresets` (returns curated list from [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go)) and a stubbed `PullAndApplyPreset` that only validates slug and returns preview or HTTP 501 when `apply=true`. No real hub sync or apply.
- Backend uses `CommandExecutor` for `cscli decisions` only; no hub pull/install logic and no cache/backups beyond file write backups in `WriteFile` and import flow.
- Frontend: [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) calls `pullAndApplyCrowdsecPreset` then falls back to local `writeCrowdsecFile` apply. Preset catalog merges backend list with [frontend/src/data/crowdsecPresets.ts](frontend/src/data/crowdsecPresets.ts). Errors 501/404 are surfaced as info to keep local apply working. Overview toggle/start/stop already wired to `startCrowdsec`/`stopCrowdsec`.
- Docs: [docs/cerberus.md](docs/cerberus.md) still notes CrowdSec integration is a placeholder; no hub sync described.

## Incident Triage: CrowdSec preset pull/apply 502/500 (feature/beta-release)
- Logs to pull first: backend app/GIN logs under `/app/data/logs/charon.log` (or `data/logs/charon.log` in dev) via [backend/cmd/api/main.go](backend/cmd/api/main.go); look for warnings "crowdsec preset pull failed" / "crowdsec preset apply failed" emitted in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go). Access logs will also show 502/500 for POST `/api/v1/admin/crowdsec/presets/pull` and `/apply`.
- Routes and code paths: handlers `PullPreset` and `ApplyPreset` live in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) and delegate to `HubService.Pull/Apply` in [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go) with cache helpers in [backend/internal/crowdsec/hub_cache.go](backend/internal/crowdsec/hub_cache.go). Data dir used is `data/crowdsec` with cache under `data/crowdsec/hub_cache` from [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go).
- Quick checks before repro: (1) Cerberus enabled (`feature.cerberus.enabled` setting or `FEATURE_CERBERUS_ENABLED`/`CERBERUS_ENABLED` env) or handler returns 404 early; (2) `cscli` on PATH and executable (`HubService` uses real executor and calls `cscli version`/`cscli hub install`); (3) outbound HTTPS to https://hub.crowdsec.net reachable (fallback after `cscli hub list`); (4) cache dir writable `data/crowdsec/hub_cache` and contains per-slug `metadata.json`, `bundle.tgz`, `preview.yaml`; (5) backup path writable (apply renames `data/crowdsec` to `data/crowdsec.backup.<ts>`).
- Likely 502 on pull: hub cache unavailable or init failed (cache dir permission), invalid slug, hub index fetch errors (`cscli hub list -o json` or direct GET `/api/index.json`), download blocked/size >25MiB, preview/download HTTP non-200, or cache write errors. Handler logs warning and returns 502 with error string.
- Likely 500 on apply: backup rename fails, `cscli` install fails with no cache fallback (if pull never succeeded or cache expired/missing), cache read errors (`metadata.json`/`bundle.tgz` unreadable), tar extraction rejects symlinks/unsafe paths, or rollback after extract failure. Handler writes `CrowdsecPresetEvent` (if DB reachable) with backup path and returns 500 with `backup` hint.
- Validation steps during triage: verify cache entry freshness (TTL 24h) via `metadata.json` timestamps; confirm `cscli hub install <slug>` succeeds manually; if cscli missing, ensure prior pull populated cache; test hub egress with curl to hub index and archive URLs; check file ownership/permissions on `data/crowdsec` and `data/crowdsec/hub_cache`; confirm log lines around warnings for exact error message; inspect backup directory to restore if partial apply.

### Current incident: preset apply returning "Network Error" (feature/beta-release)
- What we see: frontend reports axios "Network Error" while applying a preset. Backend logs do not yet show the apply warning, suggesting the client drops before an HTTP response arrives. Apply path runs `HubService.Apply` in [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go) with a 15s context; pull uses a 10s HTTP client timeout and does not follow redirects. Axios flags a network error when the TCP connection is reset/timeout rather than when a 4xx/5xx is returned.
- Probable roots to verify quickly:
	- Hub index/preview/archives now redirect to another host; our HTTP client forbids redirects, so FetchIndex/Pull return an error and the handler responds 502 only after the hub timeout. Long hub connect attempts can hit the 10s client timeout, causing the upstream (Caddy) or browser to drop the socket and surface a network error.
	- Runtime image may be missing `cscli` if the release archive layout changed; Dockerfile only moves the binaries when expected paths exist. Without cscli, Apply falls back to cache, but if Pull already failed, Apply exits with an error and no response body. Validate `cscli version` inside the running container built from feature/beta-release.
	- Outbound egress/proxy: container must reach https://hub-data.crowdsec.net (default) from within the Docker network. Missing `HTTP(S)_PROXY`/`NO_PROXY` or a transparent MITM can cause TLS handshake or connection timeouts that the client reports as network errors.
	- TLS/HTML responses: hub returning HTML (maintenance/Cloudflare) or a 3xx/302 to http is treated as an error (`hub index responded with HTML`), which becomes 502. If the redirect/HTML arrives after ~10s the browser may already have given up.
	- Timeout budget: 10s pull / 15s apply may be too tight for hub downloads + cscli install. When the context cancels mid-stream, gin closes the connection and axios logs network error instead of an HTTP code.
- Remediation plan (no code yet):
	- Confirm cscli exists in the runtime image from [Dockerfile](Dockerfile) by running `cscli version` inside the failing container; if missing, adjust build or add a startup preflight that logs absence and forces HTTP hub path.
	- Override HUB_BASE_URL to a known JSON endpoint (e.g., `https://hub-data.crowdsec.net/api/index.json`) when redirects occur, or point to an internal mirror reachable from the Docker network; document this in env examples.
	- Ensure outbound 443 to hub-data is allowed or set `HTTP(S)_PROXY`/`NO_PROXY` on the container; retry pull/apply after validating `curl -v https://hub-data.crowdsec.net/api/index.json` inside the runtime.
	- Consider raising pull/apply timeouts (and matching frontend request timeout) and log when contexts cancel so we return a 504/timeout JSON instead of a dropped socket.
	- Capture docker logs for `charon-debug` during repro; look for `crowdsec preset pull/apply failed` warnings and any TLS/redirect messages from [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go).

## Goal
Implement real CrowdSec Hub preset sync + apply on backend (using cscli or direct hub index) with caching, validation, backups, rollback, and wire the UI to new endpoints so operators can preview/apply hub items with clear status/errors.

## Backend Plan (handlers, helpers, storage)
1) Route adjustments (gin group under `/admin/crowdsec` in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)):
	- Replace stub endpoint with `POST /admin/crowdsec/presets/pull` → fetch hub item and cache; returns metadata + preview + cache key/etag.
	- Add `POST /admin/crowdsec/presets/apply` → apply previously pulled item by cache key/slug; performs backup + cscli install + optional restart.
	- Keep `GET /admin/crowdsec/presets` but include hub/etag info and whether cached locally.
	- Optional: `GET /admin/crowdsec/presets/cache/:slug` → raw preview/download for UI.
2) Hub sync helper (new [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go)):
	- Provide `type HubClient interface { FetchIndex(ctx) (HubIndex, error); FetchPreset(ctx, slug) (PresetBundle, error) }` with real impl using either:
	  a) `cscli hub list -o json` and `cscli hub update` + `cscli hub install <item>` (preferred if cscli present), or
	  b) direct fetch of https://hub.crowdsec.net/ or GitHub raw `.index.json` + tarball download.
	- Validate downloads: size limits, tarball path traversal guard, checksum/etag compare, basic YAML validation.
3) Caching (new [backend/internal/crowdsec/hub_cache.go](backend/internal/crowdsec/hub_cache.go)):
	- Cache pulled bundles under `${DataDir}/hub_cache/<slug>/` with index metadata (etag, fetched_at, source URL) and preview YAML.
	- Expose `LoadCachedPreset(slug)` and `StorePreset(slug, bundle)`; evict stale on TTL (configurable, default 24h) or when etag changes.
4) Apply flow (extend handler):
	- `Pull`: fetch index, resolve slug, download bundle to cache, return preview + warnings (missing cscli, requires restart, etc.).
	- `Apply`: before modify, run `backupDir := DataDir + ".backup." + timestamp` (mirror current write/import backups). Then:
	  a) If cscli available: `cscli hub update`, `cscli hub install <slug>` (or collection path), maybe `cscli decisions list` sanity check. Use `CommandExecutor` with context timeout.
	  b) If cscli absent: extract bundle into DataDir with sanitized paths; preserve permissions.
	  c) Write audit record to DB table `crowdsec_preset_events` (new model in [backend/internal/models](backend/internal/models)).
	- On failure: restore backup (rename back), surface error + backup path.
5) Status and restart:
	- After apply, optionally call `h.Executor.Stop/Start` if running to reload config; or `cscli service reload` when available. Return `reload_performed` flag.
6) Validation & security hardening:
	- Enforce `Cerberus` enablement check (`isCerberusEnabled`) on all new routes.
	- Path sanitization with `filepath.Clean`, limit tar extraction to DataDir, reject symlinks/abs paths.
	- Timeouts on all external calls; default 10s pull, 15s apply.
	- Log with context: slug, etag, source, backup path; redact secrets.
7) Migration of curated list:
	- Keep curated presets in [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go) but add `Source: "hub"` for hub-backed items and include `RequiresHub` true when not bundled.
	- `ListPresets` should merge curated + live hub index when available, mark availability per slug (cached, remote-only, local-bundled).

## Frontend Plan (API wiring + UX)
1) API client updates in [frontend/src/api/presets.ts](frontend/src/api/presets.ts):
	- Replace `pullAndApplyCrowdsecPreset` with `pullCrowdsecPreset({ slug })` and `applyCrowdsecPreset({ slug, cache_key })`; include response typing for preview/status/errors.
	- Add `getCrowdsecPresetCache(slug)` if backend exposes cache preview.
2) CrowdSec config page [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx):
	- Use new mutations: `pull` to show preview + metadata (etag, fetched_at, source); disable local fallback unless backend says `apply_supported=false`.
	- Show status strip (success/error) and backup path from apply response; surface reload flag and errors inline.
	- Gate preset actions when Cerberus disabled; show tooltip if hub unreachable.
	- Keep local backup + manual file apply as last-resort only when backend explicitly returns 501/NotImplemented.
3) Overview page [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx):
	- No UI change except error surfacing when start/stop fails due to hub apply requiring reload; show toast from handler message.
4) Import page [frontend/src/pages/ImportCrowdSec.tsx](frontend/src/pages/ImportCrowdSec.tsx):
	- Add note linking to presets apply so users prefer presets over raw package imports.

## Hub Fetch/Validate/Apply Flow (detailed)
1) Pull
	- Handler: `CrowdsecHandler.PullPreset(ctx)` (new) calls `HubClient.FetchPreset` → `HubCache.StorePreset` → returns `{preset, preview_yaml, etag, cache_key, fetched_at}`.
	- If hub unavailable, return 503 with message; UI shows retry/cached copy option.
2) Apply
	- Handler: `CrowdsecHandler.ApplyPreset(ctx)` loads cache by slug/cache_key → `backupCurrentConfig()` → `InstallPreset()` (cscli or manual) → optional restart → returns `{status:"applied", backup, reloaded:true/false}`.
	- On error: restore backup, include `{status:"failed", backup, error}`.
3) Caching & rollback
	- Cache directory per slug with checksum file; TTL enforced on pull; apply uses cached bundle unless `force_refetch` flag.
	- Backups stored with timestamp; keep last N (configurable). Provide restoration note in response for UI.
4) Validation
	- Tarball extraction guard: reject absolute paths, `..`, symlinks; limit total size.
	- YAML sanity: parse key scenario/collection files to ensure readable; log warning not blocker unless parse fails.
	- Require explicit `apply=true` separate from pull; no implicit apply on pull.

## Security Considerations
- Only allow these endpoints when Cerberus enabled and user authenticated to admin scope.
- Use `CommandExecutor` to shell out to cscli; restrict PATH and working dir; do not pass user-controlled args without whitelist.
- Network egress: if hub URL configurable, validate scheme is https and host is allowlisted (crowdsec official or configured mirror).
- Rate limit pull/apply (simple in-memory token bucket) to avoid abuse.
- Logging: include slug and etag, omit file contents; redact download URLs if they contain tokens (unlikely).

## Required Tests
- Backend unit/integration:
  - `backend/internal/api/handlers/crowdsec_handler_test.go`: success and error cases for `PullPreset` (hub reachable/unreachable, invalid slug), `ApplyPreset` (cscli success, cscli missing fallback, apply fails and restores backup), `ListPresets` merging cached hub entries.
  - `backend/internal/crowdsec/hub_sync_test.go`: parse index JSON, validate tar extraction guards, TTL eviction.
  - `backend/internal/crowdsec/hub_cache_test.go`: store/load/evict logic and checksum verification.
  - `backend/internal/api/handlers/crowdsec_exec_test.go`: ensure executor timeouts/commands constructed for cscli hub calls.
- Frontend unit/UI:
  - [frontend/src/pages/__tests__/CrowdSecConfig.test.tsx](frontend/src/pages/__tests__/CrowdSecConfig.test.tsx): pull shows preview, apply success shows backup path/reload flag, hub failure falls back to cached/local message, Cerberus disabled disables actions.
  - [frontend/src/api/__tests__/presets.test.ts](frontend/src/api/__tests__/presets.test.ts): client hits new endpoints and maps response.
  - [frontend/src/pages/__tests__/Security.test.tsx](frontend/src/pages/__tests__/Security.test.tsx): start/stop toasts remain correct when apply errors bubble.

## Docs Updates
- Update [docs/cerberus.md](docs/cerberus.md) CrowdSec section with new hub preset flow, backup/rollback notes, and requirement for cscli availability when using hub.
- Update [docs/features.md](docs/features.md) to list “CrowdSec Hub presets sync/apply (admin)” and mention offline curated fallback.
- Add short troubleshooting entry in [docs/troubleshooting/crowdsec.md](docs/troubleshooting/crowdsec.md) (new) for hub unreachable, checksum mismatch, or cscli missing.

## Migration Notes
- Existing curated presets remain but are marked as bundled; UI should continue to show them even if hub unreachable.
- Stub endpoint `POST /admin/crowdsec/presets/pull/apply` is replaced by separate `pull` and `apply`; frontend must switch to new API paths before backend removal to avoid 404.
- Backward compatibility: keep returning 501 from old endpoint until frontend merged; remove once new routes live and tested.
