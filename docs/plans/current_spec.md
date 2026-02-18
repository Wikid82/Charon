## CodeQL Go Coverage RCA (2026-02-18)

### 1) Observed Evidence (exact commands/workflow paths/config knobs that control scope)

- Local CI-aligned command in VS Code task `Security: CodeQL Go Scan (CI-Aligned) [~60s]`:
   - `codeql database create codeql-db-go --language=go --source-root=backend --codescanning-config=.github/codeql/codeql-config.yml --overwrite --threads=0`
   - `codeql database analyze codeql-db-go --additional-packs=codeql-custom-queries-go --format=sarif-latest --output=codeql-results-go.sarif --sarif-add-baseline-file-info --threads=0`
- Local pre-commit CodeQL Go scan command (`scripts/pre-commit-hooks/codeql-go-scan.sh`):
   - `codeql database analyze codeql-db-go codeql/go-queries:codeql-suites/go-security-and-quality.qls --format=sarif-latest --output=codeql-results-go.sarif --sarif-add-baseline-file-info --threads=0`
- Reproduced analyzer output from local run:
   - `CodeQL scanned 175 out of 436 Go files in this invocation.`
   - `Path filters have no effect for Go... 'paths' and 'paths-ignore' ... have no effect for this language.`
- Workflow controlling CI scan: `.github/workflows/codeql.yml`
   - `on.pull_request.branches: [main, nightly]`
   - `on.push.branches: [main, nightly, development]`
   - Uses `github/codeql-action/init` + `autobuild` + `analyze`.
   - `init` currently does not set `queries`, so suite selection is implicit.
   - Uses config file `./.github/codeql/codeql-config.yml`.
- Config file: `.github/codeql/codeql-config.yml`
   - Only `paths-ignore` entries for coverage/build artifacts; no Go-specific exclusions.
- Ground-truth file counts:
   - `find backend -type f -name '*.go' | wc -l` => `436`
   - `find backend -type f -name '*.go' ! -name '*_test.go' | wc -l` => `177`
   - `go list -json ./... | jq -s 'map((.GoFiles|length)+(.CgoFiles|length))|add'` => `175`
- Target file verification:
   - Local scan output includes extraction of `backend/internal/api/handlers/system_permissions_handler.go`.
   - SARIF contains `go/path-injection` findings in that file.

### 2) Why 175/436 happens (expected vs misconfiguration)

- **Expected behavior (primary):**
   - `436` is a raw repository count including `*_test.go` and non-build files.
   - Go CodeQL analyzes build-resolved files (roughly Go compiler view), not all raw `.go` files.
   - Build-resolved count is `175`, which exactly matches `go list` compiled files.
- **Denominator inflation details:**
   - `259` files are `*_test.go` and are not part of normal build-resolved extraction.
   - Two non-test files are also excluded from compiled set:
      - `backend/internal/api/handlers/security_handler_test_fixed.go` (`//go:build ignore`)
      - `backend/.venv/.../empty_template_main.go` (not in module package graph)
- **Conclusion:** `175/436` is mostly expected Go extractor semantics, not a direct scope misconfiguration by itself.

### 3) How this could miss findings

- **Build tags / ignored files:**
   - Files behind build constraints (for example `//go:build ignore`) are excluded from compiled extraction; findings there are missed.
- **Path filters:**
   - For Go, `paths` / `paths-ignore` do not reduce extraction scope (confirmed by CodeQL diagnostic).
   - Therefore `.github/codeql/codeql-config.yml` is not the cause of reduced Go coverage.
- **Generated or non-module files:**
   - Files outside the module/package graph (for example under `.venv`) can appear in raw counts but are not analyzed.
- **Uncompiled packages/files:**
   - Any code not reachable in package resolution/build context will not be analyzed.
- **Trigger gaps (CI event coverage):**
   - `pull_request` only targets `main` and `nightly`; PRs to `development` are not scanned by CodeQL workflow.
   - `push` only scans `main/nightly/development`; feature-branch pushes are not scanned.
- **Baseline behavior:**
   - `--sarif-add-baseline-file-info` adds baseline metadata; it does not itself suppress extraction.
   - Alert visibility can still appear delayed based on when a qualifying workflow run uploads SARIF.
- **Local/CI suite drift (explicit evidence):**
   - CI workflow (`.github/workflows/codeql.yml`) and VS Code CI-aligned task (`.vscode/tasks.json`) use implicit/default suite selection.
   - Pre-commit Go scan (`scripts/pre-commit-hooks/codeql-go-scan.sh`) pins explicit `go-security-and-quality.qls`.

### 4) Why finding appeared now (most plausible ranked causes with confidence)

1. **Trigger-path visibility gap (Plausible hypothesis, 0.60)**
   - The code likely existed before, but this remains a hypothesis unless workflow history shows explicit missing qualifying runs for the affected branch/PR path.
2. **Local/CI command drift labeled as “CI-aligned” (Medium-High, 0.70)**
   - Different entrypoints use different suite semantics (explicit in pre-commit vs implicit in workflow/task), increasing chance of inconsistent detection timing.
3. **Query/toolpack evolution over time (Medium, 0.55)**
    - Updated CodeQL packs/engines can surface dataflow paths not previously reported.
4. **Extractor file-count misunderstanding (Low, 0.25)**
    - `175/436` itself did not hide `system_permissions_handler.go`; that file is in the extracted set.

### 5) Prevention controls (local + CI): exact changes to scan commands/workflows/policies

- **CI workflow controls (`.github/workflows/codeql.yml`):**
   - Expand PR coverage to include `development`:
      - `on.pull_request.branches: [main, nightly, development]`
   - Expand push coverage to active delivery branches (or remove push branch filter if acceptable).
   - Pin query suite explicitly in `init` (avoid implicit defaults):
      - add `queries: security-and-quality`
- **Local command controls (make truly CI-aligned):**
   - Require one canonical local invocation path (single source of truth):
      - Prefer VS Code task calling `scripts/pre-commit-hooks/codeql-go-scan.sh`.
   - If task remains standalone, it must pin explicit suite:
      - `codeql database analyze codeql-db-go codeql/go-queries:codeql-suites/go-security-and-quality.qls --additional-packs=codeql-custom-queries-go ...`
- **Policy controls:**
   - Require CodeQL checks as branch-protection gates on `main`, `nightly`, and `development`.
   - Add a parity check that fails when suite selection diverges across workflow, VS Code local task, and pre-commit script.
   - Keep reporting both metrics in documentation/logs:
      - raw `.go` count
      - compiled/extracted `.go` count (`go list`-derived)
   - Add metric guardrail: fail the run when extracted compiled Go count diverges from the `go list` compiled baseline beyond approved tolerance.

### 6) Verification checklist

- [ ] Run and record raw vs compiled counts:
   - `find backend -type f -name '*.go' | wc -l`
   - `cd backend && go list -json ./... | jq -s 'map((.GoFiles|length)+(.CgoFiles|length))|add'`
- [ ] Run local CodeQL Go scan and confirm diagnostic line:
   - `CodeQL scanned X out of Y Go files...`
- [ ] Compare extraction metric to compiler baseline and fail on unexpected divergence:
   - baseline: `cd backend && go list -json ./... | jq -s 'map((.GoFiles|length)+(.CgoFiles|length))|add'`
   - extracted: parse `CodeQL scanned X out of Y Go files...` and assert `X == baseline` (or documented tolerance)
- [ ] Confirm target file is extracted:
   - local output includes `Done extracting .../system_permissions_handler.go`
- [ ] Confirm SARIF includes expected finding for file:
   - `jq` filter on `system_permissions_handler.go`
- [ ] Validate CI workflow trigger coverage includes intended PR targets/branches.
- [ ] Validate workflow and local command both use explicit `security-and-quality` suite.

### 7) PR Slicing Strategy

- **Decision:** Multiple PRs (3), to reduce rollout risk and simplify review.
- **Trigger reasons:** Cross-domain change (workflow + local tooling + policy), security-sensitive, and high review impact if combined.

- **PR-1: CI Trigger/Suite Hardening**
   - Scope: `.github/workflows/codeql.yml`
   - Changes: broaden `pull_request` branch targets, keep/expand push coverage, set explicit `queries: security-and-quality`.
   - Dependencies: none.
   - Validation gate: `actionlint` + successful CodeQL run on PR to `development`.
   - Rollback: revert workflow file only.

- **PR-2: Local Command Convergence**
   - Scope: `.vscode/tasks.json` and/or canonical script wrapper.
   - Changes: enforce explicit `go-security-and-quality.qls` in local Go task, keep custom pack additive only.
   - Dependencies: PR-1 preferred, not hard-required.
   - Validation gate: local task output shows explicit suite and reproducible SARIF.
   - Rollback: revert tasks/scripts without affecting CI.

- **PR-3: Governance/Policy Guardrails**
   - Scope: branch protection requirements + parity check job/documentation.
   - Changes: require CodeQL checks on `main/nightly/development`; add drift guard.
   - Dependencies: PR-1 and PR-2.
   - Validation gate: blocked merge when CodeQL missing/failing or parity check fails.
