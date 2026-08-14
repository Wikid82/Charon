# QA Report — Semgrep CI Security Scan (Independent Verification)

**Branch**: `development`
**Commits reviewed**: `6bf066f8`, `2fbecf07`, `7c6fb04f`
**Reviewed by**: qa-security agent
**Date**: 2026-08-14
**Scope**: CI/CD-only feature — no application code, models, or frontend/UI surface touched.
**Prior review**: Supervisor code review — approved, no blocking issues.
**Purpose**: Independent functional/security verification per Phase 6 of the management pipeline, ahead of a final "done" determination.

## Summary Verdict: **PASS** — no blocking defects found.

The Semgrep CI gate is functionally real (confirmed to fail on findings and pass when clean, via a positive-control test), the parity guard genuinely detects drift (confirmed via four separate intentional-break tests), the container image is correctly digest-pinned, and all local DoD-relevant checks scoped to a CI/shell-script-only change are clean. Two pre-existing environment/documentation gaps were identified and are explicitly **not** attributed to this feature (see §4 and §6).

---

## 1. Functional Correctness of the Scan (PASS)

Installed Semgrep 1.173.0 into a throwaway venv (`/tmp/.../scratchpad/semgrep-venv`, exact version match to the pinned CI image) and ran the actual wrapper script `scripts/pre-commit-hooks/semgrep-scan.sh` exactly as CI invokes it.

| Check | Result |
|---|---|
| `SEMGREP_SARIF_OUTPUT=<path> bash scripts/pre-commit-hooks/semgrep-scan.sh` (full repo, no targets) | Exit 0. Produced a valid SARIF file (`version`, `runs`, `results`, `$schema` present; parsed cleanly as JSON). |
| `bash scripts/pre-commit-hooks/semgrep-scan.sh` (no env var, full repo) | Exit 0. `--error` semantics confirmed live (see §1.1). |
| Repo clean under full scan | Reproduced: 974 files tracked by git, 160 rules run, **0 findings** — matches both prior QA/DevOps reports exactly. Two suppressed (`nosemgrep`-annotated) `websocket-missing-origin-check` findings appear in the SARIF's `results` array with `suppressions: [{kind: inSource}]` — this is correct SARIF behavior (audit trail for suppressed findings) and does not affect the "0 findings / 0 blocking" scan summary or exit code. |
| Runtime | ~45–48s per full-repo pass locally (single-threaded venv on this sandbox; CI's dedicated `semgrep/semgrep` container should be comparable or faster). |

### 1.1 Positive-control test: does the gate actually gate? (Most important check — PASS)

Constructed a minimal Go file containing an unguarded `websocket.Upgrader{}.Upgrade()` call (the same rule ID, `go.gorilla.security.audit.websocket-missing-origin-check`, that appears — suppressed — in the real codebase), and ran it through the **actual, unmodified** wrapper script with a single-file target:

```
SEMGREP_SARIF_OUTPUT=out.sarif bash scripts/pre-commit-hooks/semgrep-scan.sh <vuln-file>
  → Findings: 1 (1 blocking)   → exit 0   (SARIF mode does not hard-fail)

bash scripts/pre-commit-hooks/semgrep-scan.sh <vuln-file>
  → Findings: 1 (1 blocking)   → exit 1   (--error mode hard-fails)
```

This is the critical distinction the task flagged as the top risk: a gate that always exits 0 regardless of findings would be a silent no-op. **Confirmed not the case.** The `SEMGREP_SARIF_OUTPUT` toggle in `scripts/pre-commit-hooks/semgrep-scan.sh:42-46` genuinely swaps `--error` for `--sarif --output <path>`, and only the `--error` invocation (the CI workflow's "hard-fail gate" step, `semgrep.yml:75-76`) enforces blocking. The SARIF-producing pass (`semgrep.yml:49-54`) is additionally wrapped in `continue-on-error: true` at the workflow level, which is defense-in-depth on top of the script's own non-blocking `--sarif` exit code — belt and suspenders, not a substitute for the real gate.

---

## 2. Workflow YAML Structural Validity (PASS)

- `actionlint .github/workflows/semgrep.yml` (installed via `go install github.com/rhysd/actionlint@latest` into a throwaway `GOBIN`): **0 findings, exit 0.**
- Container image resolution: `docker buildx imagetools inspect semgrep/semgrep:1.173.0@sha256:67319956da3dcb58baf5b322899c15458e3963e7018a86aeeb5cd224e69cb77a` (the exact digest read fresh from the committed file, `semgrep.yml:33`) resolved successfully against the registry, returning a multi-platform manifest list whose index digest matches the pinned digest exactly. The pin is real and correct, not a stale/copy-pasted digest.

---

## 3. Parity Guards (PASS, and confirmed non-trivial)

- `bash scripts/ci/check-semgrep-parity.sh` — exit 0.
- `bash scripts/ci/check-codeql-parity.sh` — exit 0 (unaffected by the refactor that extracted `scripts/ci/lib/workflow-yaml-asserts.sh`; this file was verified as a correct behavior-preserving extraction, not a modification of the CodeQL guard's assertions).

**Adversarial drift tests** (performed against throwaway copies in `/tmp/.../scratchpad/parity-break-test/`, never against the real repo files; all discarded after each test, working tree confirmed clean of these edits afterward):

| Simulated drift | Guard result |
|---|---|
| Inline `semgrep scan --config p/golang` reintroduced in place of `bash scripts/pre-commit-hooks/semgrep-scan.sh` delegation | **Caught.** `must delegate to scripts/pre-commit-hooks/semgrep-scan.sh instead of reimplementing the semgrep scan invocation inline` — exit 1. |
| `SEMGREP_SARIF_OUTPUT` hook deleted from the local script | **Caught.** `must retain the SEMGREP_SARIF_OUTPUT hook so CI can produce SARIF via the same script` — exit 1. |
| Image pin degraded from `semgrep/semgrep:1.173.0@sha256:...` to `semgrep/semgrep:latest` | **Caught.** `must pin the semgrep/semgrep image with both an exact tag and a sha256 digest` — exit 1. |
| `pull_request` branch list narrowed from `[main, nightly, development]` to `[main, nightly]` | **Caught.** `pull_request branches must be [main, nightly, development]` — exit 1. |

All four drift classes are detected. The guard is a real structural check, not a no-op that always passes.

---

## 4. Local DoD-Relevant Checks (PASS, with one noted pre-existing environment gap)

Scoped per the task's guidance: no Playwright E2E (no user-facing behavior), no GORM scan (confirmed zero files under `backend/internal/models/**` or any `.go` files touched — `git diff --name-only c510085f 7c6fb04f` shows only workflow/doc/shell files), no frontend type-check/build (zero `frontend/` files touched).

- **shellcheck** (installed a static v0.10.0 binary into a throwaway location, no sudo/apt available) on all touched/new shell scripts (`semgrep-scan.sh`, `check-semgrep-parity.sh`, `workflow-yaml-asserts.sh`, `check-codeql-parity.sh`), using the project's actual severity threshold from `lefthook.yml` (`shellcheck --severity=error`): **0 findings.** (Default-severity mode surfaces two SC1091 "info"-level "not following sourced file" notices caused by the scripts' dynamic `SCRIPT_DIR` resolution pattern — expected and filtered out by the repo's own configured threshold, not a defect.)
- **`lefthook run pre-commit`**, targeted against this feature's exact changed-file set (`--file <7 files>`, since nothing was staged in this session): all hooks pass — `trailing-whitespace`, `end-of-file-fixer`, `actionlint`, `check-lfs-large-files`, `block-codeql-db`, `block-data-backups`, `semgrep` (0 findings, `semgrep` installed into PATH via the throwaway venv for this run only). One hook, **`check-yaml`, failed** with `ModuleNotFoundError: No module named 'yaml'` — **confirmed pre-existing/environmental, not introduced by this feature**: the hook shells out to system `python3 -c "import yaml..."` (`lefthook.yml:54`), and this sandbox's system Python lacks PyYAML. Verified by installing PyYAML into the throwaway venv and re-running the identical parse command directly against `.github/workflows/semgrep.yml` — it parsed cleanly (exit 0), proving the YAML itself is valid and the failure is purely a missing sandbox dependency, structurally identical to the already-documented `gitleaks`-unavailable gap below.
- **`gitleaks`**: confirmed absent from PATH (`which gitleaks` → exit 1) in this sandbox, matching the DevOps report. This predates the Semgrep feature entirely (a secrets-scanning tool unrelated to Semgrep) and is not something this feature could plausibly mask — the feature adds zero new secret-bearing surface (see §5).

---

## 5. Security-Specific Checks (PASS)

- **`permissions:`** — both the workflow-level and job-level blocks are exactly `contents: read`, `security-events: write`, `actions: read`, `pull-requests: read` — least-privilege, no broader scope (no `write` on `contents`, no `id-token`, no `packages`, etc.).
- **Triggers** — `pull_request` (not `pull_request_target`) confirmed at `semgrep.yml:4`. No fork-PR privilege-escalation risk.
- **Secrets/tokens** — `grep -in "secrets\.\|token\|GITHUB_TOKEN"` across `semgrep.yml`, `semgrep-scan.sh`, `check-semgrep-parity.sh`, `workflow-yaml-asserts.sh` returns no matches (the only "token" hits were unrelated word fragments in comments, none present). The SARIF-upload step uses `github/codeql-action/upload-sarif`, which relies on the workflow's implicit default `GITHUB_TOKEN` scoped by the `permissions:` block above — no custom secret is declared or required anywhere in this feature.
- **Container image pin** — digest-pinned (`semgrep/semgrep:1.173.0@sha256:67319956...`), confirmed resolvable and matching the registry (§2). Not a floating tag.

---

## 6. Documentation Lint (PASS — no regression introduced)

Ran `markdownlint-cli2@0.23.2` (the exact tool/version pinned in `package.json`, not the unrelated `markdownlint-cli` package) against `SECURITY.md` and `ARCHITECTURE.md`.

- **SECURITY.md**: 114 findings (MD036 emphasis-as-heading, MD060 table-column-style, one MD034 bare-URL) — **identical count before and after** commit `7c6fb04f` (verified via `git show 7c6fb04f~1:SECURITY.md` piped through the same linter: 114 findings on the pre-commit version too). All findings are pre-existing formatting debt scattered across unrelated CVE-entry sections (lines 407–813); zero findings land in the new Semgrep paragraph or table row this PR added (~line 1000–1032).
- **ARCHITECTURE.md**: 93 findings, likewise identical before/after `7c6fb04f`.

This PR's doc changes introduce zero new lint findings. Pre-existing lint debt is a separate, out-of-scope cleanup item and not this feature's responsibility.

---

## Blocking Issues

**None.**

## Non-Blocking Observations (informational only, no action required for this PR)

1. `check-yaml` and `gitleaks` are unavailable in this local sandbox due to missing system dependencies (PyYAML, gitleaks binary). Both are pre-existing environment gaps unrelated to this feature; CI's environment has these tools installed and is authoritative. No masking risk identified — the underlying YAML was independently verified valid, and this feature introduces no new secret-bearing surface for `gitleaks` to have caught.
2. SECURITY.md/ARCHITECTURE.md carry substantial pre-existing markdownlint debt (207 combined findings) unrelated to this PR. Worth a future standalone cleanup pass, but explicitly out of scope here per this feature's CI/shell-script-only mandate.

## Final Overall Verdict: **PASS — ready to be marked done.**
