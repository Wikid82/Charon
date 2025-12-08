# VS Code Go Troubleshooting Guide & Automations — Current Spec

## Overview

This document defines a focused implementation plan to add a concise VS Code Go troubleshooting guide and small automations to the Charon repository to address persistent Go compiler errors caused by gopls or workspace misconfiguration. The scope is limited to developer tooling, diagnostics, and safe automation changes that live in `docs/`, `scripts/`, and `.vscode/` (no production code changes). Implementation must respect the repository's architecture rules in .github/copilot-instructions.md (backend in `backend/`, frontend in `frontend/`, no Python).

## Goals / Acceptance Criteria

- Provide reproducible steps that make `go build ./...` succeed in the repo for contributors.
- Provide easy-to-run tasks and scripts that surface common misconfigurations (missing modules, GOPATH issues, gopls misbehavior).
- Provide VS Code settings and tasks so the `Go` extension/gopls behaves reliably for this repo layout.
- Provide CI and pre-commit recommendations to prevent regressions.
- Provide QA checklist that verifies build, language-server behavior, and CI integration.

Acceptance Criteria (testable):
- Running `./scripts/check_go_build.sh` from repo root in a clean dev environment returns exit code 0 and prints "BUILD_OK".
- `cd backend && go build ./...` returns exit code 0 locally on a standard Linux environment with Go installed.
- VS Code: running the `Go: Restart Language Server` command after applying `.vscode/settings.json` and `.vscode/tasks.json` clears gopls editor errors (no stale compiler errors remain in Problems panel for valid code).
- A dedicated GH Actions job runs `cd backend && go test ./...` and `go build ./...` and returns success in CI.

## Files to Inspect & Modify (exact paths)

- docs/plans/current_spec.md (this file)
- docs/troubleshooting/go-gopls.md (new — guidance + logs collection)
- .vscode/tasks.json (new)
- .vscode/settings.json (new)
- scripts/check_go_build.sh (new)
- scripts/gopls_collect.sh (new)
- Makefile (suggested additions at root)
- .github/workflows/ci-go.yml (suggested CI job snippet — add or integrate into existing CI)
- .pre-commit-config.yaml (suggested update to add a hook calling `scripts/check_go_build.sh`)
- backend/go.mod, backend/go.work, backend/** (inspect for module path and replace directives)
- backend/cmd/api (inspect build entrypoint)
- backend/internal/server (inspect server mount and attachFrontend logic)
- backend/internal/config (inspect env handling for CHARON_* variables)
- backend/internal/models (inspect for heavy imports that may cause build issues)
- frontend/.vscode (ensure no conflicting workspace settings in frontend)
- go.work (workspace-level module directives)

If function-level inspection is needed, likely candidates:
- `/projects/Charon/backend/cmd/api/main.go` or `/projects/Charon/backend/cmd/api/*.go` (entrypoint)
- `/projects/Charon/backend/internal/api/routes/routes.go` (AutoMigrate, router mounting)

Do NOT change production code unless the cause is strictly workspace/config related and low risk. Prefer documenting and instrumenting.

## Proposed Implementation (step-by-step)

1. Create `docs/troubleshooting/go-gopls.md` describing how to reproduce, collect logs, and file upstream issues. (Minimal doc — see Templates below.)

2. Add `.vscode/settings.json` and `.vscode/tasks.json` to the repo root to standardize developer tools. These settings will scope to the workspace and will not affect CI.

3. Create `scripts/check_go_build.sh` that runs reproducible checks: `go version`, `go env`, `go list -mod=mod`, `go build ./...` in `backend/`, and prints diagnostic info if the build fails.

4. Create `scripts/gopls_collect.sh` to collect `gopls` logs with `-rpc.trace` and instruct developers how to attach those logs when filing upstream issues.

5. Add a small Makefile target for dev convenience: `make go-check` -> runs `scripts/check_go_build.sh` and `make gopls-logs` -> runs `scripts/gopls_collect.sh`.

6. Add a recommended GH Actions job snippet to run `go test` and `go build` for `backend/`. Add this to `.github/workflows/ci-go.yml` or integrate into the existing CI workflow.

7. Add a sample `pre-commit` hook entry invoking `scripts/check_go_build.sh` (optional, manual-stage hook recommended rather than blocking commit for every contributor).

8. Add `docs/troubleshooting/go-gopls.md` acceptance checklist and QA test cases.

9. Communicate rollout plan and document how to revert the automation files (simple revert PR).

## VS Code Tasks and Settings

Place these files under `.vscode/` in the repository root. They are workspace recommendations a developer can accept when opening the workspace.

1) .vscode/tasks.json

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Go: Build Backend",
      "type": "shell",
      "command": "bash",
      "args": ["-lc", "cd backend && go build ./..."],
      "group": { "kind": "build", "isDefault": true },
      "presentation": { "reveal": "always", "panel": "shared" },
      "problemMatcher": ["$go"]
    },
    {
      "label": "Go: Test Backend",
      "type": "shell",
      "command": "bash",
      "args": ["-lc", "cd backend && go test ./... -v"],
      "group": "test",
      "presentation": { "reveal": "always", "panel": "shared" }
    },
    {
      "label": "Go: Mod Tidy (Backend)",
      "type": "shell",
      "command": "bash",
      "args": ["-lc", "cd backend && go mod tidy"],
      "presentation": { "reveal": "silent", "panel": "shared" }
    },
    {
      "label": "Gather gopls logs",
      "type": "shell",
      "command": "bash",
      "args": ["-lc", "./scripts/gopls_collect.sh"],
      "presentation": { "reveal": "always", "panel": "new" }
    }
  ]
}
```

2) .vscode/settings.json

```json
{
  "go.useLanguageServer": true,
  "gopls": {
    "staticcheck": true,
    "analyses": {
      "unusedparams": true,
      "nilness": true
    },
    "completeUnimported": true,
    "matcher": "Fuzzy",
    "verboseOutput": true
  },
  "go.toolsEnvVars": {
    "GOMODCACHE": "${workspaceFolder}/.cache/go/pkg/mod"
  },
  "go.buildOnSave": "workspace",
  "go.lintOnSave": "package",
  "go.formatTool": "gofmt",
  "files.watcherExclude": {
    "**/backend/data/**": true,
    "**/frontend/dist/**": true
  }
}
```

Notes on settings:
- `gopls.verboseOutput` will allow VS Code Output panel to show richer logs for triage.
- `GOMODCACHE` is set to workspace-local to prevent unexpected GOPATH/GOMOD cache interference on some dev machines; change if undesired.

## Scripts to Add

Create `scripts/check_go_build.sh` and `scripts/gopls_collect.sh` with the contents below. Make both executable (`chmod +x scripts/*.sh`).

1) scripts/check_go_build.sh

```sh
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
echo "[charon] repo root: $ROOT_DIR"

echo "-- go version --"
go version || true

echo "-- go env --"
go env || true

echo "-- go list (backend) --"
cd "$ROOT_DIR/backend"
echo "module: $(cat go.mod | sed -n '1p')"
go list -deps ./... | wc -l || true

echo "-- go build backend ./... --"
if go build ./...; then
  echo "BUILD_OK"
  exit 0
else
  echo "BUILD_FAIL"
  echo "Run 'cd backend && go build -v ./...' for verbose output"
  exit 2
fi
```

2) scripts/gopls_collect.sh

```sh
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="/tmp/charon-gopls-logs-$(date +%s)"
mkdir -p "$OUT_DIR"
echo "Collecting gopls debug output to $OUT_DIR"

if ! command -v gopls >/dev/null 2>&1; then
  echo "gopls not found in PATH. Install with: go install golang.org/x/tools/gopls@latest"
  exit 2
fi

cd "$ROOT_DIR/backend"
echo "Running: gopls -rpc.trace -v check ./... > $OUT_DIR/gopls.log 2>&1"
gopls -rpc.trace -v check ./... > "$OUT_DIR/gopls.log" 2>&1 || true

echo "Also collecting 'go env' and 'go version'"
go version > "$OUT_DIR/go-version.txt" 2>&1 || true
go env > "$OUT_DIR/go-env.txt" 2>&1 || true

echo "Logs collected at: $OUT_DIR"
echo "Attach the $OUT_DIR contents when filing issues against golang/vscode-go or gopls."
```

Optional: `Makefile` additions (root `Makefile`):

```makefile
.PHONY: go-check gopls-logs
go-check:
	./scripts/check_go_build.sh

gopls-logs:
	./scripts/gopls_collect.sh
```

## CI or Pre-commit Hooks to Run

CI: Add or update a GitHub Actions job in `.github/workflows/ci-go.yml` (or combine with existing CI):

```yaml
name: Go CI (backend)
on: [push, pull_request]
jobs:
  go-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.20'
      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: ~/.cache/go-build
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
      - name: Build backend
        run: |
          cd backend
          go version
          go env
          go test ./... -v
          go build ./...

```

Pre-commit (optional): add this to `.pre-commit-config.yaml` under `repos:` as a local hook or add a simple script that developers can opt into. Example local hook entry:

```yaml
- repo: local
  hooks:
    - id: go-check
      name: go-check
      entry: ./scripts/check_go_build.sh
      language: system
      stages: [manual]
```

Rationale: run as a `manual` hook to avoid blocking every commit but available for maintainers to run pre-merge.

## Tests & Validation Steps

Run these commands locally and in CI as part of acceptance testing.

1) Basic local verification (developer machine):

```bash
# from repo root
./scripts/check_go_build.sh
# expected output contains: BUILD_OK and exit code 0

cd backend
go test ./... -v
# expected: all tests PASS and exit code 0

go build ./...
# expected: no errors, exit code 0

# In VS Code: open workspace, accept recommended workspace settings, then
# Run Command Palette -> "Go: Restart Language Server"
# Open Problems panel: expect no stale gopls errors for otherwise-valid code
```

2) Gather gopls logs (if developer still sees errors):

```bash
./scripts/gopls_collect.sh
# expected: prints path to logs and files: gopls.log, go-version.txt, go-env.txt
```

3) CI validation (after adding `ci-go.yml`):

Push branch, create PR. GitHub Actions should run `Go CI (backend)` job and show green check on success. Expected steps: `go test` and `go build` both pass.

4) QA Acceptance Checklist (for QA_Security):
- `./scripts/check_go_build.sh` returns `BUILD_OK` on a clean environment.
- `cd backend && go test ./... -v` yields only PASS/ok lines.
- Running `./scripts/gopls_collect.sh` produces a non-empty `gopls.log` file when gopls invoked.
- VS Code `Go: Restart Language Server` clears stale errors in Problems panel.
- CI job `Go CI (backend)` passes on PR.

## How to Collect gopls Logs and File an upstream Issue

1. Reproduce the problem in VS Code.
2. Run `./scripts/gopls_collect.sh` and attach the `$OUT_DIR` results.
3. If `gopls` is not available, install it locally: `go install golang.org/x/tools/gopls@latest`.
4. Include these items in the issue:
- A minimal reproduction steps list.
- The `gopls.log` produced by `gopls -rpc.trace -v check ./...`.
- Output of `go version` and `go env` (from `go-version.txt` and `go-env.txt`).
5. File the issue on the `golang/vscode-go` issue tracker (preferred), include logs and reproduction steps.

Suggested issue title template: "gopls: persistent compiler errors in Charon workspace — [short symptom]"

## Rollout & Backout Plan

Rollout steps:

1. Implement the changes in a feature branch `docs/gopls-troubleshoot`.
2. Open a PR describing changes and link to this plan.
3. Run CI and verify `Go CI (backend)` passes.
4. Merge after 2 approvals.
5. Notify contributors in README or developer onboarding that workspace settings and scripts are available.

Backout steps:

1. Revert the merge commit or open a PR that removes `.vscode/*`, `scripts/*`, and `docs/troubleshooting/*` files.
2. If CI changes were added to `.github/workflows/*`, revert those files as well.

Notes on safety: these files are developer-tooling only. They do not alter production code or binaries.

## Estimated Effort (time / complexity)

- Create docs and scripts: 1–2 hours (low complexity)
- Add .vscode tasks/settings and Makefile snippet: 30–60 minutes
- Add CI job and test in GH Actions: 1 hour
- QA validation and follow-ups: 1–2 hours

Total: 3–6 hours for a single engineer to implement, test, and land.

## Notes for Roles

- Backend_Dev:
  - Inspect `/projects/Charon/backend/go.mod`, `/projects/Charon/backend/go.work`, `/projects/Charon/backend/cmd/api` and `/projects/Charon/backend/internal/*` for any non-module-safe imports, cgo usage, or platform-specific build tags that might confuse `gopls` or `go build`.
  - If `go build` fails locally but passes in CI, inspect `go env` differences (GOMODCACHE, GOPATH, GOFLAGS, GO111MODULE). Use `./scripts/check_go_build.sh` to capture environment.

- Frontend_Dev:
  - Ensure there are no conflicting workspace `.vscode` files inside `frontend/` that override root workspace settings. If present, move per-project overrides to `frontend/.vscode` and keep common settings in root `.vscode`.

- QA_Security:
  - Use the acceptance checklist above. Validate that `go test` and `go build` run cleanly in CI and locally.
  - Confirm that scripts do not leak secrets (they won't — they only run `go` commands). Confirm scripts are shell-only and do not pull remote binaries without explicit developer action.

- Docs_Writer:
  - Create `docs/troubleshooting/go-gopls.md` with background, step-by-step reproduction, and minimal triage guidance. Link to scripts and how to attach logs when filing upstream issues.

## Example docs/troubleshooting/go-gopls.md (template)

Create `docs/troubleshooting/go-gopls.md` with this content (starter):

```
# Troubleshooting gopls / VS Code Go errors in Charon

This page documents how to triage and collect logs for persistent Go errors shown by gopls or VS Code in the Charon repository.

Steps:
1. Open the Charon workspace in VS Code (project root).
2. Accept the workspace settings prompt to apply `.vscode/settings.json`.
3. Run the workspace task: `Go: Build Backend` (or run `./scripts/check_go_build.sh`).
4. If errors persist, run `./scripts/gopls_collect.sh` and attach the output directory to an issue.

When filing upstream issues, include `gopls.log`, `go-version.txt`, `go-env.txt`, and a short reproduction.

```

## Checklist (QA)

- [ ] `./scripts/check_go_build.sh` exits 0 and prints `BUILD_OK`.
- [ ] `cd backend && go test ./... -v` returns all `PASS` results.
- [ ] `go build ./...` returns exit code 0.
- [ ] VS Code Problems panel shows no stale gopls errors after `Go: Restart Language Server`.
- [ ] `./scripts/gopls_collect.sh` produces `gopls.log` containing `rpc.trace` sections.
- [ ] CI job `Go CI (backend)` passes on PR.

## Final Notes

All proposed files are restricted to developer experience and documentation. Do not modify production source files unless a concrete code-level bug (not tooling) is found and approved by the backend owner.

If you'd like, I can also open a PR implementing the `.vscode/`, `scripts/`, `docs/troubleshooting/` additions and the CI job snippet. If approved, I will run the repo-level `make go-check` and iterate on any failures.
# Plan: Refactor Feature Flags to Optional Features

## Overview
Refactor the existing "Feature Flags" system into a user-friendly "Optional Features" section in System Settings. This involves renaming, consolidating toggles (Cerberus, Uptime), and enforcing behavior (hiding sidebar items, stopping background jobs) when features are disabled.

## User Requirements
1.  **Rename**: 'Feature Flags' -> 'Optional Features'.
2.  **Cerberus**: Move global toggle to 'Optional Features'.
3.  **Uptime**: Add toggle to 'Optional Features'.
4.  **Cleanup**: Remove unused flags (`feature.global.enabled`, `feature.notifications.enabled`, `feature.docker.enabled`).
5.  **Behavior**:
    -   **Default**: Cerberus and Uptime ON.
    -   **OFF State**: Hide from Sidebar, stop background jobs, block notifications.
    -   **Persistence**: Do NOT delete data when disabled.

## Implementation Details

### 1. Backend Changes

#### `backend/internal/api/handlers/feature_flags_handler.go`
-   Update `defaultFlags` list:
    -   Keep: `feature.cerberus.enabled`, `feature.uptime.enabled`
    -   Remove: `feature.global.enabled`, `feature.notifications.enabled`, `feature.docker.enabled`
-   Ensure defaults are `true` if not set in DB or Env.

#### `backend/internal/cerberus/cerberus.go`
-   Update `IsEnabled()` to check `feature.cerberus.enabled` instead of `security.cerberus.enabled`.
-   Maintain backward compatibility or migrate existing setting if necessary (or just switch to the new key).

#### `backend/internal/api/routes/routes.go`
-   **Uptime Background Job**:
    -   In the `go func()` that runs the ticker:
        -   Check `feature.uptime.enabled` before running `uptimeService.CheckAll()`.
        -   If disabled, skip the check.
-   **Cerberus Middleware**:
    -   The middleware already calls `IsEnabled()`, so updating `cerberus.go` is sufficient.

### 2. Frontend Changes

#### `frontend/src/pages/SystemSettings.tsx`
-   **Rename Card**: Change "Feature Flags" to "Optional Features".
-   **Consolidate Toggles**:
    -   Remove "Enable Cerberus Security" from "General Configuration".
    -   Render specific toggles for "Cerberus Security" and "Uptime Monitoring" in the "Optional Features" card.
    -   Use `feature.cerberus.enabled` and `feature.uptime.enabled` keys.
    -   Add user-friendly descriptions for each.
-   **Remove Generic List**: Instead of iterating over all keys, explicitly render the supported optional features to control order and presentation.

#### `frontend/src/components/Layout.tsx`
-   **Fetch Flags**: Use `getFeatureFlags` (or a new hook) to get current state.
-   **Conditional Rendering**:
    -   Hide "Uptime" nav item if `feature.uptime.enabled` is false.
    -   Hide "Security" nav group if `feature.cerberus.enabled` is false.

### 3. Migration / Data Integrity
-   Existing `security.cerberus.enabled` setting in DB should be migrated to `feature.cerberus.enabled` or the code should handle the transition.
-   **Action**: We will switch to `feature.cerberus.enabled`. The user can re-enable it if it defaults to off, but we'll try to default it to ON in the handler.

## Step-by-Step Execution

1.  **Backend**: Update `feature_flags_handler.go` to clean up flags and set defaults.
2.  **Backend**: Update `cerberus.go` to use new flag key.
3.  **Backend**: Update `routes.go` to gate Uptime background job.
4.  **Frontend**: Update `SystemSettings.tsx` UI.
5.  **Frontend**: Update `Layout.tsx` sidebar logic.
6.  **Verify**: Test toggling features and checking sidebar/background behavior.
