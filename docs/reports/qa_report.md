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

---
---

# QA/Security Audit — Notify Provider Registry (Self-Registering Factory) (Independent Verification)

**Branch**: `feature/notifications-engine-extraction`
**Commit range reviewed**: `a28f0db9..HEAD` (`e3e971ec`, `326d28e9`, `14421c06`, `567de4e7`, `7c48f009`)
**Reviewed by**: qa-security agent
**Date**: 2026-08-17
**Scope**: Backend-only. Replaces a hardcoded `switch p.Type { case "discord": ... }` provider-construction dispatch in `backend/internal/services/notify_provider_adapter.go` with a call into a new self-registering factory registry (`notify.New`) shipped by a companion module, `github.com/Wikid82/go_notify_yourself` (pinned `v0.2.0` via a local-path `replace` directive to `/projects/go_notify_yourself`, branch `feature/provider-registry`, not yet pushed to GitHub — expected/tracked, not a defect).
**Prior reviews**: `backend-dev` implementation pass + `supervisor` code review — **APPROVED WITH MINOR NOTES** (no blocking issues; email construction path and the local-path `replace` directive both flagged as known, non-blocking).
**Purpose**: Independent re-verification of all Definition of Done gates plus a dedicated security review of the new `map[string]any` config-boundary and provider registry, per `docs/plans/notify_provider_registry_spec.md` §3 and §5.

## Summary Verdict: **READY TO MERGE** — no blocking issues found. All Definition of Done gates independently re-run and passed with real, non-cached numbers where applicable.

---

## 1. Definition of Done Gates (all independently re-run from repo root / `backend/`)

| Gate | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | **PASS** — clean, zero errors |
| Static vet | `cd backend && go vet ./...` | **PASS** — zero findings |
| Staticcheck | `make lint-staticcheck-only` | **PASS** — `0 issues.` (backend), `0 issues.` (agent) |
| Full backend test suite | `cd backend && go test ./...` | **PASS** — all packages `ok`, zero failures. `internal/services` (the touched package) ran uncached, 111.308s, all green. |
| Backend coverage gate | `bash scripts/go-test-coverage.sh` (`CHARON_MIN_COVERAGE` default 87%) | **PASS** — Statement coverage **89.3%**, line coverage **89.2%** (gate: line coverage ≥ 87%). Console: `Coverage gate (line coverage): minimum required 87% / Coverage requirement met`. |
| Local patch coverage preflight | `bash scripts/local-patch-report.sh` | **PASS** — Backend: 235/235 changed lines covered = **100.0%** patch coverage (threshold 85%). Overall/Frontend/Agent all 100.0% (0 changed lines outside backend). Artifacts confirmed at `test-results/local-patch-report.md` and `test-results/local-patch-report.json`. |
| GORM security scan | `./scripts/scan-gorm-security.sh --check` | **PASS** — `CRITICAL: 0`, `HIGH: 0`, `MEDIUM: 0`, `INFO: 2` (both pre-existing, in `backend/internal/models/user.go`'s `UserPermittedHost` struct — unrelated to this change; 61 files / 3675 lines scanned). |
| Lefthook pre-commit | `lefthook run pre-commit` | **N/A / clean** — working tree is clean (no staged changes; this is an audit pass on already-committed code, `git status` confirmed clean at session start), so every hook reported `(skip) no matching staged files`. The equivalent checks (`go vet`, staticcheck) were independently re-run directly above and passed. |

Playwright E2E: **not run**, per task scope. Verified during the security review (§3 below) that provider dispatch behavior is byte-for-byte unchanged — `notify_provider_adapter_test.go`'s existing per-provider tests assert the exact same dispatch URL, headers, and JSON payload shape as the pre-registry switch (e.g. Gotify's `X-Gotify-Key` header, Pushover's hardcoded production URL + `user`/`token` payload fields, Telegram's `bot<token>/sendMessage` URL). No new HTTP payload/header/URL behavior was introduced for any provider type, so the Playwright skip condition in the task brief is satisfied.

---

## 2. Commit-Range Diff Verification

- `git diff --stat a28f0db9..HEAD`: 7 files changed, 223 insertions(+), 84 deletions(-) — `ARCHITECTURE.md`, `backend/go.mod`, `backend/go.sum`, `notification_service_registry_consistency_test.go` (new), `notify_provider_adapter.go`, `notify_provider_adapter_test.go`, `notify_providers_import.go` (new).
- `git diff a28f0db9..HEAD -- backend/internal/models/notification_provider.go` → **empty**. Confirms the `Token` field's `json:"-"` GORM protection is untouched.
- `git diff a28f0db9..HEAD -- backend/internal/services/notification_service.go` → **empty**. Confirms `isSupportedNotificationProviderType`, `supportsJSONTemplates`, `isDispatchEnabled` are byte-for-byte unchanged, per the spec's hard design requirement (§3.6.2 Option A).
- `grep -rn "providers/all" backend/` → only 2 matches, both inside a comment in `notify_providers_import.go` explicitly explaining *why* `providers/all` is deliberately **not** imported. No actual import anywhere in `backend/`.

---

## 3. Security-Specific Findings

### 3.1 No token/secret leakage into logs or errors — **PASS**

Read `backend/internal/services/notify_provider_adapter.go` and `notify_providers_import.go` in full, plus every `providers/*/register.go` in `/projects/go_notify_yourself` (discord, slack, gotify, pushover, ntfy, telegram, webhook, email) and `factory.go`.

- The only error-wrapping call site in `notify_provider_adapter.go` is `fmt.Errorf("notify provider adapter: %w", err)` (line 175) — wraps, never re-formats field values.
- Every registry-level error (`notify.New`'s "no provider registered for type %q", each factory's `config["transport"] must be a non-nil *transport.Wrapper` / `config["mailer"] must be a non-nil Mailer`) references only **field/key names and the provider type discriminator** — never `provider.URL`, `provider.Token`, or any config map value.
- Log call sites that consume `buildNotifySender`'s error (`notification_service.go:312`, `:324`) log only `util.SanitizeForLog(p.Name)` plus the wrapped error — never `p.URL`/`p.Token`.
- No `fmt.Println`, `log.Print`, or debug statements were introduced anywhere in the diff.

**Conclusion**: no Gotify token, webhook URL-as-secret, or any provider credential can reach logs, error messages, or test artifacts through this code path. SECURITY.md's "Gotify Token Hygiene" requirement is satisfied.

### 3.2 `Token` field GORM protection untouched — **PASS**

Confirmed via the empty diff above (§2). `Token string \`json:"-"\`` is unchanged in `backend/internal/models/notification_provider.go`.

### 3.3 `map[string]any` config-boundary type-confusion risk — **PASS, verified independently, not just re-trusted**

Read `/projects/go_notify_yourself/factory.go` and all eight `providers/*/register.go` files directly (not assumed from the spec). Every factory:

- Type-asserts `config["transport"].(*transport.Wrapper)` (or `config["mailer"].(Mailer)` for email) with the two-value `ok` form and an explicit nil check — **never a bare/panicking assertion**.
- Returns a descriptive `fmt.Errorf` (never panics) when the assertion fails or the value is nil.
- Delegates all scalar/slice field extraction to `providers/internal/regconfig.StringField`/`StringSliceField`, both of which are deliberately lenient: a wrong-typed or missing key produces the zero value (`""`/`nil`), never a panic. Verified via `regconfig`'s own test table, which explicitly covers `wrong type int`, `wrong type nil value`, `any slice with non-string element`, and `wrong type entirely` — all resolve to zero values, not panics.
- `Register` itself panics only on programmer misuse (`nil` factory, empty name, duplicate registration) at `init()` time — never on caller-supplied runtime data, matching the `database/sql`/`image` prior-art convention the spec cites.

Charon's own tests (`notify_provider_adapter_test.go`) independently exercise this boundary: `TestBuildNotifySenderMissingTransportErrors` and `TestBuildNotifySenderInvalidTransportInConfigMapErrors` both assert a nil/invalid transport produces an error containing "transport", not a panic. `TestBuildNotifySenderUnsupportedTypeErrors` asserts an unregistered type ("carrier-pigeon") produces a "no provider registered" error, not a panic.

**Conclusion**: no type-confusion panic path exists at the registry boundary. Supervisor's prior claim is independently confirmed, not merely re-trusted.

### 3.4 `isSupportedNotificationProviderType` / `supportsJSONTemplates` / `isDispatchEnabled` unchanged — **PASS** (see §2, empty diff)

### 3.5 `providers/all` not imported anywhere in `backend/` — **PASS** (see §2)

`notify_providers_import.go` hand-picks exactly Charon's eight supported types (`discord`, `email`, `gotify`, `ntfy`, `pushover`, `slack`, `telegram`, `webhook`), matching `isSupportedNotificationProviderType`'s allowlist. A new consistency test, `TestSupportedProviderAllowlistIsSubsetOfRegisteredTypes` (`notification_service_registry_consistency_test.go`), asserts this allowlist is a subset of `notify.RegisteredTypes()` — guarding against future drift between the hand-picked imports and the allowlist.

### 3.6 `go.mod` local-path replace directive — tracked, not a defect

```
replace github.com/Wikid82/go_notify_yourself => /projects/go_notify_yourself
```

Annotated in `go.mod` with a `TODO` explaining it must be removed once `go_notify_yourself v0.2.0` is pushed/tagged upstream. This is a real, temporary condition (confirmed: `/projects/go_notify_yourself` is on local branch `feature/provider-registry`, not yet on GitHub) and matches what the task brief and supervisor's prior review both already flagged as expected. Not a merge blocker for the Charon-side PR per the spec's own commit-slicing sequencing, but **must** be resolved before this local-path pin would work in CI or for any other contributor — flagged below as a required follow-up.

---

## 4. Non-Blocking Follow-Ups (tracked, not blocking this PR)

1. **`go.mod` local-path `replace` directive** (`backend/go.mod:9`) must be removed and re-pinned to the real published `go_notify_yourself v0.2.0` tag once `/projects/go_notify_yourself`'s `feature/provider-registry` branch is pushed and tagged on GitHub. Already tracked via the in-file `TODO` comment; CI/other contributors cannot build this branch until resolved.
2. Email's construction (`notification_service.go:350`, `dispatchEmailViaNotify`) still calls `email.New(...)` directly rather than routing through `notify.New` — consistent with pre-existing architecture (email's `Mailer`/`TemplateRenderer` DI seam predates this work) and not a regression, per supervisor's prior note. No action required by this PR.

## Blocking Issues

**None.**

## Final Overall Verdict: **READY TO MERGE**

All seven independently re-run Definition of Done gates pass with real numbers (89.3%/89.2% overall backend coverage against an 87% gate; 100% patch coverage on the 235 changed lines against an 85% gate; 0 CRITICAL/HIGH/MEDIUM GORM findings; 0 staticcheck/vet findings; full backend suite green). The security-specific review independently verified — by reading the actual factory/register code in `/projects/go_notify_yourself`, not by re-trusting the prior supervisor review — that no token/secret can reach logs or error messages, that the `map[string]any` registry boundary cannot panic on caller-controlled input, that the `Token` field's `json:"-"` protection and Charon's three provider allowlists are byte-for-byte unchanged from before this work, and that `providers/all` is never imported in `backend/`. The one open item (local-path `go.mod` replace directive) is already tracked and does not block merging this Charon-side PR per the spec's own two-repo commit-slicing sequencing.
