# Semgrep CI Security Scan — Implementation Plan

Status: Planning complete, revised per Supervisor review (round 2).
Owner for implementation: **devops** agent (CI/CD-only change; no backend-dev or frontend-dev involvement — no application code, no models, no UI).
Branch: current working branch (`development`) per `CLAUDE.md` — no worktree.
PR base branch: `development` (standard feature PR convention observed in this repo; `main` only receives weekly `nightly` promotion merges).

---

## 1. Introduction

### 1.1 Objective

Add an independent Semgrep SAST scan to GitHub Actions CI that reproduces, byte-for-byte, the same scan behavior developers already run locally via `scripts/pre-commit-hooks/semgrep-scan.sh` (wired through `lefthook.yml`'s `pre-commit`/`pre-push`/`security-full` targets and `make security-local`). Today, Semgrep coverage exists **only** on the developer's machine — CI has zero Semgrep footprint (confirmed: no match in `.github/workflows/`, `.github/renovate.json`, or any Dockerfile/compose file). This means:

- A developer who bypasses lefthook (`--no-verify`, an emergency hotfix, a machine without semgrep installed) ships code with no Semgrep signal at all.
- Nobody re-verifies the "clean" local Semgrep run against a controlled, versioned environment — the local binary's version, ruleset revision, and installed registry rules can silently drift per-developer with no CI backstop.

This plan adds CI-side Semgrep coverage that is authoritative (independent of the developer's local environment) while staying faithful to the existing local invocation.

### 1.2 Goals

1. A new CI job runs the **exact same** rule configs, exclusions, and severity/error-gating behavior as `scripts/pre-commit-hooks/semgrep-scan.sh`'s default (no-override) path, scanning the full repo.
2. Semgrep's version is pinned in CI (image tag + digest) — today there is no version pin anywhere in the repo for Semgrep, local or CI.
3. Findings are visible in the GitHub Security tab (SARIF upload), consistent with how CodeQL and Trivy results are surfaced today.
4. A hard-fail gate blocks the PR/branch on ERROR/WARNING findings, mirroring the local script's `--error` behavior — CI is a gate, not just an informational report.
5. `scripts/pre-commit-hooks/semgrep-scan.sh`'s binary/version resolution logic (the `command -v semgrep` check, §2.1) is **not touched** — that stays developer-local tooling, per the original brief's explicit scope boundary. The script's rule-config/exclude/severity logic, by contrast, **is** extended with one small, additive, backward-compatible hook (§2.7/§3.0) so CI can reuse it directly instead of duplicating it — see §2.7 for why this is a different constraint than "freeze the whole file," and why the narrower reading is the right one.
6. Documentation (`SECURITY.md` and `ARCHITECTURE.md`) is updated to reflect the new CI coverage.

### 1.3 Non-goals

- No change to how the local pre-commit/pre-push semgrep **binary** is discovered, installed, or versioned (the `command -v semgrep` / exit-127 block in `scripts/pre-commit-hooks/semgrep-scan.sh` is untouched).
- No new GitHub Action marketplace dependency requiring npm/JS runtime — Semgrep ships as a self-contained CLI in an official container image, which is used directly.
- No change to `.gitignore`, `.dockerignore`, `.codecov.yml`, or any `Dockerfile` (see §2.9 — reviewed explicitly, no changes needed).
- No attempt to unify Trivy's/CodeQL's SARIF-upload plumbing into a shared reusable workflow — out of scope for this feature; each scanner's workflow remains independent, consistent with current repo structure (`codeql.yml`, `security-pr.yml`, `security-weekly-rebuild.yml` are all separate files today).

---

## 2. Research Findings

### 2.1 Local Semgrep invocation (`scripts/pre-commit-hooks/semgrep-scan.sh`)

Full script behavior (verified by reading the file):

- Requires `semgrep` on `PATH`; exits 127 if missing (this resolution logic is untouched by this plan — see §1.3).
- Default rule configs (used unless `SEMGREP_CONFIG` env override is set):
  ```
  --config p/golang
  --config p/javascript
  --config p/typescript
  --config p/react
  --config p/secrets
  --config p/dockerfile
  ```
- Targets: staged files if passed as args (lefthook `pre-commit`), else full-repo default `Dockerfile backend frontend/src scripts .github/workflows` (lefthook `security-full` / manual run).
- Exact scan flags (current, pre-change):
  ```
  semgrep scan \
    "${SEMGREP_CONFIGS[@]}" \
    --severity ERROR \
    --severity WARNING \
    --error \
    --exclude "frontend/node_modules" \
    --exclude "frontend/coverage" \
    --exclude "frontend/dist" \
    --exclude-rule "go.secrets.gorm.gorm-empty-password.gorm-empty-password" \
    "${TARGETS[@]}"
  ```
- `--error` makes semgrep exit non-zero if any ERROR/WARNING-severity finding exists — this is the local "hard fail" behavior CI must reproduce.

Wiring confirmed in `lefthook.yml`:
- `pre-commit.semgrep` (line ~113-116): glob-scoped, staged-files-only, blocking.
- `security-full.semgrep` (line ~137-140, manual stage, `lefthook run security-full`): full-repo, no args → this is the invocation CI should mirror most closely (full-repo, not staged-file-scoped).
- `Makefile:security-local` additionally runs `SEMGREP_CONFIG=p/golang` as a fast pre-push subset — this is a narrower override path, not the target for CI parity (CI should mirror the **full** default ruleset, matching `security-full`).

### 2.2 Confirmed: zero Semgrep footprint in CI today

`grep -rn "semgrep" .github/workflows/ .github/renovate.json` (and Dockerfiles/compose) returns no matches. Semgrep is 100% local-only today. (Note: the repo's Renovate config lives at `.github/renovate.json`, not a root-level `renovate.json` — corrected throughout this plan.)

### 2.3 Existing CI patterns to mirror

**`.github/workflows/codeql.yml`** (closest pattern for a source-level SAST tool):
- Triggers: `pull_request`/`push` on `[main, nightly, development]`, `workflow_dispatch`, weekly `schedule` cron (`0 3 * * 1`, Mondays 03:00 UTC).
- `concurrency` group keyed on workflow/event/ref, `cancel-in-progress: true`.
- `permissions:` declared at **both** the workflow (top) level and again, identically, at job level (`contents: read`, `security-events: write`, `actions: read`, `pull-requests: read`).
- All third-party actions pinned by commit SHA with a `# vX.Y.Z` trailing comment, e.g. `github/codeql-action/init@ff2f1c621b7f889edc0d3c761ac2e6a3f8cdb0dd # v4`.
- Has a **parity guard** step ("Verify CodeQL parity guard" → `scripts/ci/check-codeql-parity.sh`) that runs *before* the scan, structurally checking that local pre-commit scripts, `.vscode/tasks.json`, and the CI workflow all agree on query-suite pinning and trigger branches — added specifically because CodeQL's local/CI ruleset previously drifted silently (see `check-codeql-parity.sh` comment referencing a real incident: a suppressed finding rode through PR #1216 unnoticed because local and CI independently duplicated blocking logic).
- Emits results to `$GITHUB_STEP_SUMMARY`, then a **separate, later step** does the actual hard-fail (`Fail on High-Severity Findings`) — reporting and gating are deliberately split into two steps so the summary always renders even on failure.

**`.github/workflows/security-pr.yml`** (closest pattern for "pinned scanner → SARIF upload → hard-fail gate"):
- Runs Trivy via `aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25` (SHA-pinned, `# aquasecurity/trivy-action 0.36.0` comment), with an explicit `version: 'v0.73.0'` input additionally pinning the *scanner* version, not just the action wrapper.
- Runs the scan **twice**: once with `format: 'sarif'` (`continue-on-error: true`, purely for the Security tab), then again with `format: 'table'` + `exit-code: '1'` (no continue-on-error) as the actual blocking gate. It also has an explicit "Check Trivy SARIF output exists" gating step between the SARIF-producing run and the upload step. This two-pass "report, then gate" split, plus the existence check, is the direct template for Semgrep's SARIF-vs-hard-fail split (§3.3).
- SARIF uploaded via `github/codeql-action/upload-sarif@ff2f1c621b7f889edc0d3c761ac2e6a3f8cdb0dd # v4.37.7` (same SHA-pinned action already used elsewhere in this repo for SARIF ingestion — no new third-party dependency needed for the upload step).
- Trigger shape is materially more complex than needed here (`workflow_run` chaining off `docker-build.yml`, PR-number resolution, artifact download) because Trivy scans a **built container image**. Semgrep scans **source**, so it needs none of that — it can trigger directly on `push`/`pull_request` like CodeQL, with no dependency on a prior Docker build.

### 2.4 Repo-wide pinning convention

Every third-party action in this repo is pinned to an exact commit SHA with a trailing `# vX.Y.Z` comment — never a floating tag, never `@latest`. This is enforced by convention/review, not currently by a lint rule for actions specifically. Any new job must follow this exactly.

### 2.5 Semgrep version/mechanism research

Options considered:

| Option | Assessment |
|---|---|
| `pip install semgrep==<version>` on `ubuntu-latest` | Works, but reintroduces a Python toolchain dependency into a Go+TS repo purely for CI plumbing (`CLAUDE.md`: "No Python — do not introduce Python scripts or requirements"). While this is arguably a tooling install rather than an authored script, it still pulls in `pip`/Python resolution behavior (version solving, transitive dependency drift) that the repo's own conventions steer away from. Rejected. |
| `semgrep/semgrep-action` (formerly `returntocorp/semgrep-action`) marketplace GitHub Action | Semgrep's own current CI docs no longer lead with this as the primary GitHub Actions pattern; it's a thin wrapper around the same official Docker image. Using it would add an extra layer of indirection (an Action wrapping an image) for no behavioral benefit over using the image directly, and re-pinning *that* action's SHA doesn't pin Semgrep's own version any more precisely than pinning the image does. Rejected in favor of the image directly. |
| Official `semgrep/semgrep` Docker image, used as a job-level `container:`, pinned by exact tag **and** digest | Matches this repo's SHA-pinning strictness (a digest is the container-image equivalent of an action's commit SHA — both are content-addressed, immutable references). Gives the CLI directly, with the identical `semgrep scan ...` invocation used locally — maximizes behavioral parity with `semgrep-scan.sh`. **Selected.** |

Confirmed via the Semgrep GitHub releases API (`api.github.com/repos/semgrep/semgrep/releases/latest`) and PyPI, current stable version at plan time is **`1.173.0`**. Resolved the corresponding Docker Hub manifest digest for `semgrep/semgrep:1.173.0`:

```
sha256:67319956da3dcb58baf5b322899c15458e3963e7018a86aeeb5cd224e69cb77a
```

Pinned reference to use in the workflow:

```
semgrep/semgrep:1.173.0@sha256:67319956da3dcb58baf5b322899c15458e3963e7018a86aeeb5cd224e69cb77a
```

**Note for the implementer (devops agent):** re-resolve this digest at implementation time (`docker buildx imagetools inspect semgrep/semgrep:1.173.0` or the registry API) rather than trusting the value transcribed into this plan verbatim, in case the tag's digest has moved between planning and implementation (Docker Hub does not guarantee a tag's digest is immutable the way a Git SHA is — pinning to the *tag+digest pair as observed at merge time* is the achievable guarantee here, and Renovate, already active in this repo via `.github/renovate.json`, will pick up future digest/tag bumps the same way it tracks other pinned SHAs if configured to watch this image — see §3.7 edge case).

**Known gotcha (Semgrep's own docs, `semgrep.dev/docs/kb/semgrep-ci/using-nonroot-docker-image-with-gha`):** running `semgrep/semgrep` as a job-level `container:` against a `actions/checkout`-produced workspace can hit git's "dubious ownership" safety check because the container user doesn't match the checkout's file ownership. Mitigate with an explicit `git config --global --add safe.directory "$GITHUB_WORKSPACE"` step before invoking `semgrep-scan.sh` (§3.3).

**Correction (Supervisor round 2, required change 1):** the container image reference **cannot** be centralized in a workflow-level `env:` var and referenced as `container.image: ${{ env.SEMGREP_IMAGE }}`. GitHub Actions' documented context-availability rules do not expose the `env` context to `jobs.<job_id>.container` — this is a known, currently-true limitation (not something that needs "verification at implementation time"; treating it as an open question in the prior draft was itself the error). The plan now specifies the pinned string **inlined directly** in `container.image` as the only correct form (§3.2) — no `env` indirection.

### 2.6 Placement decision: new file vs. an existing workflow

*(Per mid-task correction from Management: decide the best location and justify it, rather than defaulting to a new file. Approved as-is by Supervisor round 2 — no changes in this revision.)*

Three placements were evaluated:

| Placement | Verdict |
|---|---|
| **New job added to `codeql.yml`** | Rejected. `codeql.yml`'s entire structure is a `strategy.matrix` over CodeQL *languages* (`go`, `javascript-typescript`), with per-language conditional steps (`if: matrix.language == 'go'`) for Go toolchain setup/build and the CodeQL parity guard. Semgrep is not a CodeQL language variant — it's a different tool with a different container, different config format, and a different (single, non-matrixed) invocation. Bolting it in as a third matrix leg would force awkward `if: matrix.language == 'semgrep'` conditionals across steps that don't apply to it (Autobuild, `codeql-action/init`, Go build verification), degrading the readability of a file whose entire premise is "one job, matrixed by CodeQL language." Also couples Semgrep's schedule/trigger lifecycle to CodeQL's, when they are independent tools that should be able to fail, be disabled, or be re-scheduled independently. |
| **New job added to `security-pr.yml`** | Rejected. That workflow's trigger shape and majority of its steps exist *solely* to solve "how do I scan a Docker image that was already built by a separate upstream workflow" — PR-number resolution from `workflow_run` payloads, artifact download/load fallback logic, container extraction of the `charon` binary, a trust-boundary validation step for the `workflow_run` event. None of that applies to Semgrep, which scans source text directly on `push`/`pull_request` with no dependency on `docker-build.yml` having run first. Adding a source-scanning job to an image-scanning workflow would mean either (a) it inherits triggers/conditions built for image scanning that don't fit it (e.g. `workflow_dispatch` inputs are `pr_number`-shaped, meaningless for a source scan), or (b) it needs its own parallel `if:` conditions bolted onto an already condition-heavy file, adding complexity for no shared benefit — the two jobs would share a file but no actual logic. |
| **New file: `.github/workflows/semgrep.yml`** | **Selected.** Semgrep is source-level SAST, triggered directly on `push`/`pull_request`/`schedule`/`workflow_dispatch` — structurally identical in trigger shape to `codeql.yml`, but a distinct tool with its own container, config, and failure/gating semantics. This also matches the repo's existing convention of **one file per scanner**: `codeql.yml` (CodeQL), `security-pr.yml` (Trivy on PR images), `security-weekly-rebuild.yml` (Trivy weekly full scan) are already separate files rather than merged into one "security" workflow, even though they're conceptually related. A dedicated `semgrep.yml` continues that pattern: each scanner is independently triggerable, independently disable-able, and independently readable, at the cost of one more file — a cost the repo has already accepted three times over for its other scanners. |

### 2.7 "Freeze the whole script" reconsidered — design revision (Supervisor round 2, required change 3)

**The original brief's non-goal, re-read precisely:** *"Do NOT touch `scripts/pre-commit-hooks/semgrep-scan.sh`'s binary/version resolution logic itself — that's explicitly out of scope, reserved for the user's own local tooling."* This is a constraint about **binary/version discovery** (the `command -v semgrep` / exit-127 block, §2.1) — not a blanket freeze on every line of the file. The first draft of this plan over-read it into "never touch this file at all," which forced:

- A second, hand-written `semgrep scan ...` invocation inline in the workflow YAML, duplicating all six `--config` flags, all three `--exclude` flags, and the `--exclude-rule` value.
- A `check-semgrep-parity.sh` script whose primary job was detecting drift between that duplicated invocation and the real script.
- Pressure to extract shared assertion helpers out of `check-codeql-parity.sh` mainly to support that parity script's config-matching checks.

That is real, avoidable complexity, not an inherent requirement. **Revised design (adopted — option (a) from Supervisor's feedback):** add one small, additive, backward-compatible hook to `semgrep-scan.sh` itself, leaving the binary/version-resolution logic (the actual thing the non-goal protects) completely untouched:

```bash
# Existing lines (SEMGREP_CONFIGS / TARGETS construction) unchanged above this point.

if [ -n "${SEMGREP_SARIF_OUTPUT:-}" ]; then
  OUTPUT_FLAGS=(--sarif --output "${SEMGREP_SARIF_OUTPUT}")
else
  OUTPUT_FLAGS=(--error)
fi

semgrep scan \
  "${SEMGREP_CONFIGS[@]}" \
  --severity ERROR \
  --severity WARNING \
  "${OUTPUT_FLAGS[@]}" \
  --exclude "frontend/node_modules" \
  --exclude "frontend/coverage" \
  --exclude "frontend/dist" \
  --exclude-rule "go.secrets.gorm.gorm-empty-password.gorm-empty-password" \
  "${TARGETS[@]}"
```

Behavior:
- **`SEMGREP_SARIF_OUTPUT` unset (every existing call site — `pre-commit`, `pre-push`/`security-full`, `make security-local`):** `OUTPUT_FLAGS=(--error)` — byte-identical to today's behavior. Zero change for any existing developer workflow.
- **`SEMGREP_SARIF_OUTPUT=<path>` set (new — CI only):** swaps `--error` for `--sarif --output <path>`, while every `--config`, `--exclude`, and `--exclude-rule` argument stays exactly as-is, sourced from exactly one place.

This lets CI invoke the **same script** for both the SARIF-producing pass and the hard-fail gate pass (§3.3 steps 5 and 8), varying only an env var. Consequences:

- The duplicated `--config`/`--exclude` list in the workflow YAML is **eliminated entirely** — there is now exactly one place (`semgrep-scan.sh`) that defines what gets scanned, for both local and CI, for both the reporting pass and the gating pass.
- `check-semgrep-parity.sh` shrinks correspondingly (§3.4) — it no longer needs to compare two independent config lists (nothing to compare; there's only one). It still has a real, narrower job: confirming the additive hook isn't silently removed, confirming the workflow actually delegates to the script for both passes (rather than a future edit reintroducing an inline duplicate), and confirming the image pin and trigger branches stay correct. This is a smaller, more clearly justified guard than the original draft's.
- The pressure to extract `scripts/ci/lib/workflow-yaml-asserts.sh` out of `check-codeql-parity.sh` is now a plain, optional DRY nicety (the branch-check helper is still needed by both scripts) rather than something load-bearing for the config-parity story — see §3.5.

**Why not stop here and also drop the parity guard entirely?** Because two failure-independent invariants remain worth checking even with zero config duplication: (1) that the additive `SEMGREP_SARIF_OUTPUT` hook stays present in the script (a future refactor of `semgrep-scan.sh` could drop it without realizing CI depends on it), and (2) that the workflow keeps *delegating* to the script for both passes rather than a future edit reintroducing an inline `semgrep scan` call (e.g. someone "simplifying" the SARIF step by hand and accidentally dropping an `--exclude`). Both are cheap, structural, grep-level checks — proportionate, not over-engineering, and much smaller than the original draft's guard (§3.4).

This section supersedes the original §2.7 ("Parity guard: warranted, and why") from the first draft.

### 2.8 Documentation review

- **`SECURITY.md`** (`## Security Audits & Scanning` → `### Automated Scanning` table, lines 992-1020): lists Trivy, CodeQL, govulncheck, golangci-lint (gosec), npm audit, and a `### Scanning Workflows` subsection describing each workflow file's purpose (`docker-build.yml`, `supply-chain-verify.yml`, `security-weekly-rebuild.yml`, PR-specific scanning). **This is the primary file to update** — add a `Semgrep` row to the table and a new `**Semgrep SAST Scan**` paragraph under `### Scanning Workflows` describing `.github/workflows/semgrep.yml`.
- **`docs/security.md`**: verified by full-text search (`codeql|trivy|scan|pipeline`, no matches) — this file is entirely about the Cerberus runtime security feature (CrowdSec/WAF/access lists), unrelated to the CI/SAST scanning pipeline. **No change needed here.**
- **`ARCHITECTURE.md` (Supervisor round 2, required change 2 — added to scope):** `CLAUDE.md` requires `ARCHITECTURE.md` updates for changes touching security architecture, and this file already documents the CI security-scanning stack in three places that must be kept current:
  - Line 166, tech-stack table: `| **Security Scanning** | Trivy + Grype | Latest | Vulnerability detection |` — append Semgrep, e.g. `Trivy + Grype + Semgrep`.
  - Line 1376, CI Jobs list: `3. **Security:** Trivy, CodeQL, Grype, Govulncheck` — append `, Semgrep`.
  - Lines 1498-1501, "Container Scanning" components list (`Trivy: ...`, `Grype: ...`, `CodeQL: ...`) — add a fourth line, `Semgrep: Static analysis for security anti-patterns (Go, JS/TS, React, secrets, Dockerfile)`, consistent with the existing one-line-per-tool style.
  - This is now part of Commit 3's scope (§6) and Acceptance Criteria (§5), alongside `SECURITY.md`.

### 2.9 Ignore-file / build-file review (explicit confirmation per `CLAUDE.md`)

- **`.gitignore`**: already contains a blanket `*.sarif` ignore (line 189) with a narrow `!scripts/security/testdata/*.sarif` carve-out (line 190). A new `semgrep-results.sarif` file in the repo root matches the existing wildcard — **no change needed**.
- **`.dockerignore`**: already excludes `*.sarif` (line 179) — irrelevant anyway, since this is a CI-only workflow change with no Docker image content change — **no change needed**.
- **`.codecov.yml`**: workflow-only YAML change, produces no coverage-relevant files — **no change needed**.
- **Any `Dockerfile`**: not touched; Semgrep runs in its own CI container, never inside the Charon application image — **no change needed**.

(Approved as-is by Supervisor round 2 — no changes in this revision.)

### 2.10 Commit scope: `feat:` vs `feat(security):`

Per `CLAUDE.md`, `feat:`/`fix:`/`perf:` trigger Docker builds; `chore:` skips them, and `feat(security):`/`fix(security):` is reserved for "genuinely security-relevant... real vulnerability fixes, new protective mechanisms." **Decision:** the workflow-adding commit (Commit 2, §6) qualifies as a **new protective mechanism** — it is, definitionally, new automated vulnerability/anti-pattern detection gating merges — so it uses `feat(security):`, not plain `feat:`. Commit 1 (the additive `semgrep-scan.sh` hook + parity guard) also touches genuine security tooling directly and is scoped `feat(security):` for the same reason. Commit 3 (docs) stays `docs:`, matching repo convention for documentation-only changes regardless of what they document. Per `CLAUDE.md`'s vagueness requirement for `(security)` subjects, none of these commit subjects name a vulnerability class or attack vector — they describe the category ("add CI security scanning coverage") only, which is appropriate here since this isn't a vulnerability fix in the first place, just extra coverage.

**Nuance retained from the original draft:** this change touches zero Docker-build-relevant paths (no `Dockerfile`, no backend/frontend source), so the triggered Docker build (a side effect of `feat`/`feat(security)` prefixes repo-wide) is harmless but expected — not a sign something is wrong with a "just workflow files + one shell script" PR.

---

## 3. Technical Specifications

### 3.0 Change to `scripts/pre-commit-hooks/semgrep-scan.sh` (additive, in scope per §2.7)

**File:** `scripts/pre-commit-hooks/semgrep-scan.sh`
**Change:** insert the `OUTPUT_FLAGS` branch (§2.7) immediately before the existing `semgrep scan \` invocation, and replace the invocation's `--error` line with `"${OUTPUT_FLAGS[@]}"`. No other line in the file changes — the `command -v semgrep` check, the `SEMGREP_CONFIG` override branch, and the `TARGETS` construction are byte-identical to today.
**Backward compatibility:** every existing call site (`lefthook.yml`'s `pre-commit.semgrep`, `security-full.semgrep`, `Makefile`'s `security-local`) never sets `SEMGREP_SARIF_OUTPUT`, so `OUTPUT_FLAGS=(--error)` unconditionally for all of them — identical exit-code and output behavior to the pre-change script.
**New behavior (CI-only):** `SEMGREP_SARIF_OUTPUT=<path> bash scripts/pre-commit-hooks/semgrep-scan.sh [targets...]` scans with the same configs/exclusions but emits SARIF to `<path>` instead of hard-failing on findings.

### 3.1 New file: `.github/workflows/semgrep.yml`

No API/DB/frontend surface — this is CI/YAML only. Full structural spec below (devops agent should treat this as the authoritative shape; exact YAML syntax is implementer's to finalize, but every element listed must be present).

**Workflow name:** `Semgrep - SAST Scan`

**Triggers:**
```yaml
on:
  pull_request:
    branches: [main, nightly, development]
  push:
    branches: [main, nightly, development]
  workflow_dispatch:
  schedule:
    - cron: '0 4 * * 1'  # Mondays 04:00 UTC — offset 1h after CodeQL's 03:00 to avoid runner contention
```

**Concurrency:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event_name }}-${{ github.head_ref || github.ref_name }}
  cancel-in-progress: true
```
(Identical pattern to `codeql.yml`.)

**Permissions — declared at both workflow (top) level and job level, identically, matching `codeql.yml`'s style (Supervisor round 2, non-blocking fix):**
```yaml
permissions:
  contents: read
  security-events: write
  actions: read
  pull-requests: read
```
This exact block appears twice: once at the workflow top level (sibling of `on:`/`concurrency:`), and again inside `jobs.semgrep-scan.permissions` (§3.2).

### 3.2 Job: `semgrep-scan`

**Correction (Supervisor round 2, required change 1):** the pinned image is inlined directly as a literal string in `container.image` — `jobs.<job_id>.container` does not have access to the `env` context per GitHub Actions' documented context-availability rules, so a workflow-level `env:` indirection (as drafted originally) would not resolve at all. Inlining is the only correct form, not a fallback.

```yaml
jobs:
  semgrep-scan:
    name: Semgrep SAST Scan
    runs-on: ubuntu-latest
    timeout-minutes: 15
    permissions:
      contents: read
      security-events: write
      actions: read
      pull-requests: read
    container:
      image: semgrep/semgrep:1.173.0@sha256:67319956da3dcb58baf5b322899c15458e3963e7018a86aeeb5cd224e69cb77a  # semgrep/semgrep 1.173.0
```

### 3.3 Steps

1. **Checkout repository**
   ```yaml
   - name: Checkout repository
     uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7
     with:
       ref: ${{ github.ref }}
   ```
   (Same SHA already pinned and in active use in `codeql.yml` — reuse, don't re-pin a different version.)

2. **Fix git safe.directory for container user** (mitigates the "dubious ownership" issue documented in Semgrep's own GHA KB article, §2.5):
   ```yaml
   - name: Configure git safe.directory
     run: git config --global --add safe.directory "$GITHUB_WORKSPACE"
   ```

3. **Verify Semgrep parity guard** (§3.4):
   ```yaml
   - name: Verify Semgrep parity guard
     run: bash scripts/ci/check-semgrep-parity.sh
   ```

4. **Print Semgrep version** (cheap sanity check that the pinned image actually resolves to the expected CLI version — catches a bad digest pin immediately and legibly, rather than surfacing as a confusing downstream scan failure):
   ```yaml
   - name: Verify Semgrep version
     run: semgrep --version
   ```

5. **Run Semgrep (SARIF output, non-blocking)** — calls the real script (§2.7/§3.0) with the new opt-in var; no duplicated config list.
   ```yaml
   - name: Run Semgrep (SARIF output)
     id: semgrep_sarif
     continue-on-error: true
     env:
       SEMGREP_SARIF_OUTPUT: semgrep-results.sarif
     run: bash scripts/pre-commit-hooks/semgrep-scan.sh
   ```
   `continue-on-error: true` because this pass must not block the job even if semgrep itself errors — the SARIF file's presence is checked explicitly next (step 6), and the actual gate is step 8, not this step.

6. **Check Semgrep SARIF output exists** (mirrors `security-pr.yml`'s `Check Trivy SARIF output exists` step — this was an orphaned reference in the first draft of this plan; it is now a real, numbered step):
   ```yaml
   - name: Check Semgrep SARIF output exists
     id: semgrep_sarif_check
     if: always()
     run: |
       if [ -f semgrep-results.sarif ]; then
         echo "exists=true" >> "$GITHUB_OUTPUT"
       else
         echo "exists=false" >> "$GITHUB_OUTPUT"
         echo "No Semgrep SARIF output found; skipping SARIF upload"
       fi
   ```

7. **Upload Semgrep SARIF to GitHub Security** (gated on step 6's output rather than blindly attempting the upload):
   ```yaml
   - name: Upload Semgrep SARIF to GitHub Security
     if: always() && steps.semgrep_sarif_check.outputs.exists == 'true'
     uses: github/codeql-action/upload-sarif@ff2f1c621b7f889edc0d3c761ac2e6a3f8cdb0dd # v4.37.7
     with:
       sarif_file: semgrep-results.sarif
       category: semgrep
     continue-on-error: true
   ```
   (Reuses the exact SHA already pinned for this purpose in `security-pr.yml` — no new pin to introduce or maintain.)

8. **Run Semgrep (hard-fail gate)** — calls the same script, this time with the default (unset `SEMGREP_SARIF_OUTPUT`) path, i.e. its normal `--error` behavior:
   ```yaml
   - name: Run Semgrep (hard-fail gate)
     run: bash scripts/pre-commit-hooks/semgrep-scan.sh
   ```
   This is the literal `security-full` invocation (§2.1) — same script, same default full-repo targets, same `--error` flag, run a second time (this time without the SARIF env var) so failure here genuinely gates the job. If this step fails, the job fails, blocking the PR/branch — this is the CI-independent reproduction of the local "green" signal the feature exists to deliver. SARIF upload (step 7) has already completed by this point, so a gate failure here does not suppress the informational upload — order matters and is intentional.

9. **Upload SARIF artifact** (retention, matches `security-pr.yml`'s `Upload scan artifacts` step):
   ```yaml
   - name: Upload SARIF artifact
     if: always() && steps.semgrep_sarif_check.outputs.exists == 'true'
     uses: actions/upload-artifact@bbbca2ddaa5d8feaa63e36b76fdaad77386f024f # v4.4.3
     with:
       name: semgrep-sarif-${{ github.run_id }}
       path: semgrep-results.sarif
       retention-days: 14
     continue-on-error: true
   ```

10. **Job summary**
    ```yaml
    - name: Create job summary
      if: always()
      run: |
        {
          echo "## Semgrep SAST Scan Results"
          echo ""
          echo "**Rulesets**: p/golang, p/javascript, p/typescript, p/react, p/secrets, p/dockerfile"
          echo "**Severity Gate**: ERROR, WARNING (--error)"
          if [ "${{ job.status }}" == "success" ]; then
            echo "PASSED: no blocking Semgrep findings"
          else
            echo "FAILED: Semgrep reported blocking findings — see step logs and the Security tab"
          fi
        } >> "$GITHUB_STEP_SUMMARY"
    ```

### 3.4 New file: `scripts/ci/check-semgrep-parity.sh`

**Revised, smaller scope (per §2.7's design change)** — modeled on `scripts/ci/check-codeql-parity.sh`'s approach (grep/structural assertions, not full YAML parsing), but no longer needs to compare two independent config lists, because §3.0's design means there's only one config list in the whole repo (in `semgrep-scan.sh`) and the workflow only ever delegates to it. Exits non-zero with an `::error title=Semgrep parity drift::` annotation on any mismatch.

**Checks performed:**

1. Required files exist: `.github/workflows/semgrep.yml`, `scripts/pre-commit-hooks/semgrep-scan.sh`.
2. Assert `scripts/pre-commit-hooks/semgrep-scan.sh` still contains the string `SEMGREP_SARIF_OUTPUT` — the additive CI hook (§3.0) must not be silently removed by a future edit to the script that forgets CI depends on it.
3. Assert `.github/workflows/semgrep.yml` contains **two** distinct delegating calls to the real script, not a reimplemented/inlined `semgrep scan ...` invocation:
   - a call with `SEMGREP_SARIF_OUTPUT` set (the reporting pass, step 5) — e.g. assert both the literal strings `SEMGREP_SARIF_OUTPUT` and `scripts/pre-commit-hooks/semgrep-scan.sh` appear within the same step block;
   - a bare `bash scripts/pre-commit-hooks/semgrep-scan.sh` call with no env override (the gate pass, step 8).
   - This is the direct analogue of `check-codeql-parity.sh`'s "shared blocking logic must live in exactly one place" check (that script's lines enforcing `SHARED_GATE_SCRIPT` usage) — applied here to prevent a future edit from "simplifying" either step by inlining `semgrep scan` directly, which would silently reintroduce the duplicated-config problem §2.7 eliminated.
4. Assert `.github/workflows/semgrep.yml`'s pinned image reference matches the pattern `semgrep/semgrep:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}` (tag + digest both present — catches an accidental un-pin, e.g. someone changing it to `semgrep/semgrep:latest` during a quick edit).
5. Assert `pull_request`/`push` trigger branches in `semgrep.yml` are `[main, nightly, development]`, reusing `check-codeql-parity.sh`'s existing `ensure_event_branches_semantic` helper pattern (§3.5).

Note what this script **no longer does**, relative to the first draft: it does not enumerate or compare `--config`/`--exclude`/`--exclude-rule` values between two files, because after §3.0's change there is only one file that defines them.

**Where it's invoked:**
- `.github/workflows/semgrep.yml` step 3 (§3.3), analogous to `codeql.yml`'s "Verify CodeQL parity guard" step.
- Not wired into `lefthook.yml` in this PR — consistent with existing precedent: `check-codeql-parity.sh` is also CI-only today, invoked directly from `codeql.yml` and not from any lefthook stage. Noted as a possible follow-up, not a gap introduced by this plan.

### 3.5 Shared helper extraction (optional, DRY nicety — from §3.4 item 5)

`check-semgrep-parity.sh` is now the second script needing the branch-list assertion logic (`ensure_event_branches` / `ensure_event_branches_with_yq` / `ensure_event_branches_semantic`) that currently lives only in `check-codeql-parity.sh`. Recommend extracting it into `scripts/ci/lib/workflow-yaml-asserts.sh`, sourced by both scripts via `source "$(dirname "${BASH_SOURCE[0]}")/lib/workflow-yaml-asserts.sh"`, per `CLAUDE.md`'s "consolidate after second occurrence" DRY guideline. This is a pure refactor of existing, already-tested logic — low risk. Unlike the first draft, this extraction is no longer load-bearing for anything (the config-parity story doesn't depend on it, since there's no config duplication left to compare) — it is a legitimate but strictly optional cleanup. If time-boxed out of this PR, note it as a follow-up rather than skipping silently.

### 3.6 SARIF category naming

`category: semgrep` for the `upload-sarif` step (§3.3 step 7) — single, flat category since (unlike CodeQL's per-language matrix) there is only one Semgrep job/run per commit, no need for a parameterized category string.

### 3.7 Error handling / edge cases

| Scenario | Behavior |
|---|---|
| Pinned image digest becomes invalid/removed from registry (rare, but Docker Hub retention policies exist) | Job fails at container-pull time with a clear GitHub Actions infra error, not a silent skip. Remediation: re-resolve digest, bump the pin — a normal dependency-bump PR, same as any other pinned SHA bump in this repo. |
| SARIF step (step 5) itself crashes (e.g. semgrep internal error, not a rule finding) | `continue-on-error: true` on step 5 means the job continues; step 6 explicitly checks for the SARIF file's existence and sets an output consumed by steps 7 and 9, so a missing file cleanly skips upload rather than `upload-sarif` failing opaquely on a missing path. |
| Hard-fail gate step (step 8) fails legitimately (real findings) | Job fails, PR shows a red check, `$GITHUB_STEP_SUMMARY` still renders (step 10 runs on `if: always()`), SARIF is still uploaded to the Security tab (step 7 already ran before step 8 — order is intentional: SARIF upload must happen *before* the blocking step so a gate failure doesn't skip the informational upload). |
| Renovate later proposes bumping the pinned `semgrep/semgrep` image tag/digest | Handled like any other Renovate-tracked pin, via `.github/renovate.json` — devops agent should confirm at implementation time whether Renovate's Docker-image datasource already picks up `container: image:` refs in workflow YAML by default, or needs an explicit entry added to `.github/renovate.json`; note as a follow-up if configuration is needed, not a blocker for this PR. |
| A future edit to `semgrep-scan.sh` removes the `SEMGREP_SARIF_OUTPUT` hook, or an edit to `semgrep.yml` reintroduces an inline `semgrep scan` call instead of delegating | `check-semgrep-parity.sh` fails CI on the very next PR that makes either change, per §3.4 items 2-3. |

---

## 4. Implementation Plan

This is a CI/DevOps-only change plus one small, additive shell-script change. There is no Playwright/E2E surface (no user-facing behavior changes), no backend implementation, no frontend implementation. The phase structure below is adapted accordingly — **the `devops` agent implements this directly; no handoff to backend-dev, frontend-dev, or playwright-dev is needed.**

### Phase 1 — Foundation (script hook + parity guard + optional shared lib)
- Apply the additive `SEMGREP_SARIF_OUTPUT` change to `scripts/pre-commit-hooks/semgrep-scan.sh` (§3.0).
- Write `scripts/ci/check-semgrep-parity.sh` (§3.4).
- Optionally extract `scripts/ci/lib/workflow-yaml-asserts.sh` from `check-codeql-parity.sh` (§3.5); if done, refactor `check-codeql-parity.sh` to source it and verify it still passes unchanged.
- Validation gate: run the existing `semgrep` lefthook hooks locally (`lefthook run pre-commit` touching a Go/JS file, or `lefthook run security-full`) to confirm the script's default (`SEMGREP_SARIF_OUTPUT` unset) behavior is byte-identical to pre-change — this is the regression check for §3.0's edit. `shellcheck scripts/pre-commit-hooks/semgrep-scan.sh scripts/ci/check-semgrep-parity.sh` (+ `scripts/ci/lib/workflow-yaml-asserts.sh` if extracted). If the shared lib was extracted, `bash scripts/ci/check-codeql-parity.sh` still exits 0 (regression check on that refactor).

### Phase 2 — Workflow file
- Write `.github/workflows/semgrep.yml` per §3.1-§3.3 (10 steps, including the SARIF-existence-check as a first-class step, not an orphaned reference).
- Validation gate: `actionlint .github/workflows/semgrep.yml` (tool already required per `lefthook.yml`'s `actionlint` hook, §2 header comment listing required tools). `bash scripts/ci/check-semgrep-parity.sh` now passes against the real files. YAML syntax sanity via `yq eval '.' .github/workflows/semgrep.yml >/dev/null` or equivalent.
- **Live GitHub Actions execution cannot be validated locally** — the actual scan run (image pull, semgrep execution against the real repo, SARIF upload, gate pass/fail) is confirmed only once the PR opens and the workflow triggers on `pull_request`. Note this explicitly in the PR description as a manual verification step, not a local DoD gate.

### Phase 3 — Documentation
- Update `SECURITY.md` per §2.8: add Semgrep row to the `### Automated Scanning` table, add a `**Semgrep SAST Scan**` paragraph to `### Scanning Workflows` describing `.github/workflows/semgrep.yml`'s trigger shape and what it covers.
- Update `ARCHITECTURE.md` per §2.8: the three call-outs at lines 166, 1376, and 1498-1501.
- Validation gate: `markdownlint SECURITY.md ARCHITECTURE.md` (tool already required per `lefthook.yml` header comment).

### Phase 4 — Integration validation
- `lefthook run pre-commit` (full local hook suite, including the existing `actionlint` and `semgrep` hooks, to confirm nothing in this change breaks existing local gates).
- `bash scripts/ci/check-codeql-parity.sh` (if refactored) and `bash scripts/ci/check-semgrep-parity.sh` both green.
- Manual review of the rendered `semgrep.yml` against `codeql.yml`/`security-pr.yml` for pinning-comment consistency (every third-party `uses:` has a SHA + version comment; the container image has tag + digest inlined, not via `env`).

### Phase 5 — PR & CI confirmation
- Open PR; confirm `semgrep.yml` actually triggers on the PR event, pulls the pinned image successfully, produces a SARIF upload visible under the repo's Security → Code scanning alerts (filtered by tool "Semgrep"), and that the hard-fail gate step correctly reflects the current repo's Semgrep cleanliness (expected: green, since this is the same ruleset the repo already passes locally today).
- If CI surfaces findings the local run didn't (e.g. a stale local semgrep binary/ruleset that had drifted below the pinned CI version), that is itself the feature working as intended — resolve findings on their merits, not by weakening the pin.

---

## 5. Acceptance Criteria

1. `.github/workflows/semgrep.yml` exists, triggers on `pull_request`/`push` to `[main, nightly, development]`, `workflow_dispatch`, and a weekly `schedule`.
2. Semgrep runs inside a `container:` whose `image:` is the pinned string `semgrep/semgrep:<version>@sha256:<digest>` inlined directly (no `env:` indirection) — no floating tag, no `@latest`.
3. `permissions:` is declared identically at both the workflow (top) level and the job level.
4. `scripts/pre-commit-hooks/semgrep-scan.sh` carries exactly the additive `SEMGREP_SARIF_OUTPUT` change described in §3.0 — its binary/version-resolution logic and its `--config`/`--exclude`/`--exclude-rule` values are otherwise unchanged, and every existing call site's behavior (`SEMGREP_SARIF_OUTPUT` unset) is byte-identical to pre-change.
5. `.github/workflows/semgrep.yml` invokes `scripts/pre-commit-hooks/semgrep-scan.sh` directly for **both** the SARIF-producing pass (with `SEMGREP_SARIF_OUTPUT` set) and the hard-fail gate pass (unset) — no independent/duplicated `semgrep scan ...` invocation exists anywhere in the workflow YAML.
6. `scripts/ci/check-semgrep-parity.sh` exists, passes against the merged state, and is invoked as a CI step in `semgrep.yml` before the scan runs.
7. SARIF results upload to the GitHub Security tab under category `semgrep`, using the same `github/codeql-action/upload-sarif` SHA pin already used in `security-pr.yml`, gated on an explicit SARIF-existence check step.
8. `SECURITY.md`'s `### Automated Scanning` table and `### Scanning Workflows` section mention Semgrep and `semgrep.yml`.
9. `ARCHITECTURE.md` mentions Semgrep at all three existing security-scanning call-out locations (tech-stack table, CI Jobs list, Container Scanning components list).
10. `.gitignore`, `.dockerignore`, `.codecov.yml`, and all Dockerfiles are confirmed unchanged (per §2.9 — no diff expected in this PR).
11. `actionlint`, `markdownlint`, `shellcheck`, and `lefthook run pre-commit` all pass locally on the final diff.
12. `docs/plans/current_spec.md` (this file) reflects the implemented state — no divergence between plan and shipped workflow at PR time (devops agent should update this file if implementation deviates from any spec section above, per standard plan-fidelity practice).

---

## 6. Commit Slicing Strategy

**Decision:** single PR, one feature ("Semgrep CI coverage"), ordered logical commits. No cross-PR splitting per `CLAUDE.md`'s "One Feature = One PR" rule — this is a small, cohesive, CI-only change (plus one additive shell-script hook); splitting it further would violate that rule for no benefit (there's no independently-shippable sub-feature here — a workflow with no parity guard, or a parity guard with no workflow, are both incomplete on their own). Approved as-is by Supervisor round 2 — shape unchanged, contents updated below for §2.7's design revision and the ARCHITECTURE.md addition.

### Commit 1 — Script hook + parity guard foundation
- **Scope:** Additive-only, no behavior change for any existing call site. Adds the `SEMGREP_SARIF_OUTPUT` hook to `semgrep-scan.sh`, the new (as-yet-unused-by-CI) parity script, and optionally the shared helper extraction.
- **Files:** `scripts/pre-commit-hooks/semgrep-scan.sh` (modified — additive only, per §3.0), `scripts/ci/check-semgrep-parity.sh` (new — will fail if run now, since `semgrep.yml` doesn't exist yet; not wired into any workflow in this commit), `scripts/ci/lib/workflow-yaml-asserts.sh` (new, optional) and `scripts/ci/check-codeql-parity.sh` (refactored to source it, optional, no behavioral change) if the extraction from §3.5 is included.
- **Dependencies:** none.
- **Validation gate:** `shellcheck` on all touched/new scripts; local `lefthook run pre-commit` / `lefthook run security-full` on a sample file confirms `semgrep-scan.sh`'s default behavior is unchanged; `bash scripts/ci/check-codeql-parity.sh` passes unchanged if the refactor is included (regression check).
- **Commit message:** `feat(security): add opt-in SARIF output mode to local Semgrep script and add CI parity guard`

### Commit 2 — Semgrep CI workflow
- **Scope:** Adds the new workflow file, delegating both its SARIF and gate passes to the script from Commit 1, and wires the parity guard from Commit 1 into it.
- **Files:** `.github/workflows/semgrep.yml` (new).
- **Dependencies:** Commit 1 (the `SEMGREP_SARIF_OUTPUT` hook and the parity guard must exist for this workflow to reference real, working behavior).
- **Validation gate:** `actionlint .github/workflows/semgrep.yml`; `bash scripts/ci/check-semgrep-parity.sh` now passes (workflow file exists, delegates correctly, image pin format valid); `lefthook run pre-commit` clean on the diff.
- **Commit message:** `feat(security): add pinned Semgrep SAST scan to CI, mirroring local pre-commit/pre-push scan`

### Commit 3 — Documentation
- **Scope:** `SECURITY.md` and `ARCHITECTURE.md` updates only, per §4 Phase 3.
- **Files:** `SECURITY.md`, `ARCHITECTURE.md`.
- **Dependencies:** Commit 2 (documents the workflow file that now exists).
- **Validation gate:** `markdownlint SECURITY.md ARCHITECTURE.md`.
- **Commit message:** `docs: document Semgrep CI scan in SECURITY.md and ARCHITECTURE.md`

### Commit 4 — Hardening / fixups (conditional)
- **Scope:** Only if Phase 5 (opening the PR and observing the first real workflow run) surfaces something unfixable purely by inspection — e.g. the digest needs re-resolution, `actionlint`/a GitHub Actions schema quirk requires a syntax adjustment not visible from local linting alone, or the container's default shell needs an explicit `shell: bash` on a step.
- **Files:** `.github/workflows/semgrep.yml` and/or `scripts/ci/check-semgrep-parity.sh`, as needed.
- **Dependencies:** Commits 1-3, plus one observed CI run on the PR.
- **Validation gate:** the actual GitHub Actions run on the PR going green.
- **Commit message:** `fix: address Semgrep CI workflow issues found in first live run` (only created if needed — do not pre-author an empty placeholder commit).

### Rollback / contingency

- **Rollback:** revert the PR's merge commit. The change is additive-only (new files + one additive, backward-compatible shell-script hook + documentation sections); reverting it removes Semgrep CI coverage cleanly with no residual state — no DB migration, no data written, no schema changed. `git revert -m 1 <merge-sha>` is sufficient.
- **Contingency — pinned image becomes unpullable mid-development-cycle (e.g. registry outage, Docker Hub rate limiting on `ubuntu-latest` runners):** the job fails visibly (container pull failure is unambiguous in the Actions log, distinct from a scan failure), does not block other workflows (independent job, independent file), and does not gate merges any more strictly than any other required-check outage would — same failure mode and same operational response as a transient CodeQL or Trivy Action outage today.
- **Contingency — CI Semgrep surfaces findings that don't reproduce locally:** expected and desired (§4 Phase 5) — indicates local environment drift, not a CI bug. Do not suppress via `--exclude-rule` additions without documenting rationale (matching the existing precedent set by the one documented `gorm-empty-password` exclusion already in the script).
- **Contingency — parity guard is judged too strict/noisy after landing** (e.g. flags legitimate divergence that's actually fine): tune the specific assertion in `check-semgrep-parity.sh`, don't delete the guard wholesale — same operating principle already established for `check-codeql-parity.sh`, which has been iterated on rather than removed.
