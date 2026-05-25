## Introduction

This plan targets the latest CI failure in job Security Scan PR Image and is
scoped to workflow-only changes in
[.github/workflows/docker-build.yml](.github/workflows/docker-build.yml).

Objectives:
- Explain why diagnostics still reports fallback with reason parser command
  failure.
- Verify whether `.trivyignore` is honored by both PR Trivy steps.
- Keep step 15 (Enforce PR Trivy security gate) as the authoritative blocker.
- Add deterministic diagnostics that can still list unsuppressed IDs when
  possible.
- Define minimal edits, exact step names, and validation commands.

## Research Findings

### Evidence Pointers

1. Gate failure is real and is driven by `trivy-scan` outcome:
   - log shows enforce step evaluating failure and exiting 1:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1551)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1579)
   - workflow gate logic is explicitly tied to `steps.trivy-scan.outcome`:
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L1020)
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L1023)

2. Diagnostics fallback is specifically parser-command related (not missing
   SARIF):
   - fallback reason assignment:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1505)
   - fallback summary path in workflow:
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L1005)

3. `.trivyignore` is passed and discovered in both PR scan steps:
   - table scan input and ignore discovery:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L711)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L716)
   - SARIF scan input and ignore discovery:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1172)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1177)
   - corresponding workflow step configuration:
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L852)
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L862)
   - IDs repeatedly seen in log are present in ignore file (example set):
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1208)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1218)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1256)
     [.trivyignore](.trivyignore#L39)
     [.trivyignore](.trivyignore#L49)
     [.trivyignore](.trivyignore#L88)

4. Signal mismatch is present:
   - table output reports 0 vulnerabilities and indicates suppression occurred:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L832)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L833)
   - SARIF step still exits with code 1:
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1292)
     [.github/logs/ci_failure.log](.github/logs/ci_failure.log#L1294)

5. Current diagnostics parser is brittle and hides useful failure detail:
   - large coupled jq filter with package regex extraction:
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L933)
     [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L960)
   - stderr is redirected away in the command path used for findings JSON,
     making root parser error opaque in logs.

### Root Cause Assessment

1. Why parser command failure still happens:
- The diagnostics step couples ID extraction and package text parsing in one jq
  pipeline. Any jq runtime/shape issue in optional fields can fail the whole
  command and force fallback.
- The current implementation suppresses parser stderr (`2>/dev/null`) for the
  core parser command, so CI only records generic fallback reason without
  actionable parser detail.

2. Are table and SARIF scans honoring ignore file:
- Evidence shows both steps load `.trivyignore`.
- Therefore ignore-file loading is not the primary failure.
- Most likely mismatch is due to scan surface differences when SARIF step exits
  1 while table shows 0 vulnerabilities. A minimal deterministic fix is to
  pin both PR Trivy steps to vulnerability scanner scope (`vuln`) so the gate
  aligns exactly with unsuppressed vulnerability IDs governed by
  `.trivyignore`.

## Technical Specification (Workflow-Only)

File in scope:
- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)

Exact step names in scope:
1. Run Trivy scan on PR image (table output)
2. Run Trivy scan on PR image (SARIF - blocking)
3. Diagnose unsuppressed PR Trivy blockers
4. Enforce PR Trivy security gate (no logic change)

### Minimal Edit Set

### Edit A: Align scanner surface for both PR Trivy steps

Steps:
- Run Trivy scan on PR image (table output)
- Run Trivy scan on PR image (SARIF - blocking)

Change:
- Add explicit `scanners: 'vuln'` to both steps.

Reason:
- Makes table and SARIF behavior deterministic for vulnerability gating.
- Ensures `.trivyignore` governs the same result set used by both outputs.
- Preserves intended gate semantics: fail on unsuppressed vulnerabilities or
  scan execution failure.

### Edit B: Make diagnostics deterministic and ID-first

Step:
- Diagnose unsuppressed PR Trivy blockers

Changes:
- Keep prechecks for file missing/unreadable/invalid JSON/unexpected schema.
- Split parsing into two paths:
  - authoritative path: extract unsuppressed IDs only, with robust optional
    navigation and `try` safeguards.
  - optional enrichment path (package parsing) that never controls fallback.
- On parser failure:
  - record `parser_exit_code`.
  - capture first parser stderr line in summary (sanitized, no secrets).
- Summary always includes deterministic fields:
  - Diagnostics status
  - Fallback reason (if any)
  - Count
  - Blocker IDs (or unknown when parser truly cannot produce IDs)

Reason:
- Guarantees best-effort blocker-ID reporting even when enrichment fails.
- Removes ambiguity from current generic parser fallback.

### Edit C: Preserve authoritative gate semantics

Step:
- Enforce PR Trivy security gate

Change:
- No behavioral change.

Invariant:
- Keep `if: always()`.
- Keep pass/fail decision tied only to `steps.trivy-scan.outcome`.
- Diagnostics output remains informational and non-authoritative.

## Implementation Plan

### Phase 1: Scanner Alignment
- Add `scanners: 'vuln'` to both PR Trivy scan steps.
- Do not change `severity`, `trivyignores`, `continue-on-error`, or
  `exit-code` behavior.

### Phase 2: Diagnostics Refactor
- Refactor diagnostics step to ID-first parsing and deterministic fallback
  taxonomy.
- Add parser exit-code and first-error-line reporting when parsing fails.

### Phase 3: Gate Invariant Verification
- Confirm enforce step remains unchanged in semantics and remains step-15
  authority.

## Validation Commands

Run from repository root:

```bash
actionlint .github/workflows/docker-build.yml
```

```bash
rg -n "Run Trivy scan on PR image \(table output\)|Run Trivy scan on PR image \(SARIF - blocking\)|Diagnose unsuppressed PR Trivy blockers|Enforce PR Trivy security gate|scanners:" .github/workflows/docker-build.yml
```

If local image and Trivy are available:

```bash
trivy image --format table --severity CRITICAL,HIGH --scanners vuln --ignorefile .trivyignore <image-ref>
```

```bash
trivy image --format sarif --severity CRITICAL,HIGH --scanners vuln --ignorefile .trivyignore -o trivy-pr-results.sarif <image-ref>
```

```bash
jq -e '.' trivy-pr-results.sarif && jq -e '(.runs | type) == "array"' trivy-pr-results.sarif
```

## Expected Outcomes

1. Diagnostics parser failure path becomes explicit and actionable:
   - fallback includes parser exit code and parser error hint.

2. Ignore behavior is deterministic across PR table and SARIF scans:
   - both use the same scanner surface (`vuln`) and same ignore file.

3. Step 15 remains authoritative:
   - Enforce PR Trivy security gate fails only when `trivy-scan` outcome is not
     success.
   - diagnostics content cannot change gate decision.

4. When SARIF is parseable:
   - summary lists unsuppressed blocker IDs (or `none`), not generic unknown.

## Acceptance Criteria (EARS)

1. WHEN Diagnose unsuppressed PR Trivy blockers runs and SARIF is missing,
   THE SYSTEM SHALL emit diagnostics status fallback with reason file missing.
2. WHEN SARIF is unreadable, THE SYSTEM SHALL emit fallback with reason file
   unreadable.
3. WHEN SARIF is invalid JSON, THE SYSTEM SHALL emit fallback with reason
   invalid JSON.
4. WHEN SARIF schema is unexpected, THE SYSTEM SHALL emit fallback with reason
   unexpected schema.
5. WHEN parser execution fails, THE SYSTEM SHALL emit fallback with reason
   parser command failure and include parser exit code.
6. WHEN parser enrichment fails but ID extraction succeeds,
   THE SYSTEM SHALL still report blocker IDs and parsed status.
7. WHEN both PR Trivy steps execute,
   THE SYSTEM SHALL use `.trivyignore` and scanner scope `vuln` consistently.
8. WHEN Enforce PR Trivy security gate executes,
   THE SYSTEM SHALL keep `if: always()` and SHALL fail only when
   `steps.trivy-scan.outcome` is not success.

## Commit Slicing Strategy

Decision:
- Single PR with ordered logical commits.

Commit 1:
- Scope: scanner alignment on PR Trivy steps (`scanners: 'vuln'`).
- Files: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- Dependencies: none.
- Validation gate: actionlint and step-name/field presence verification.

Commit 2:
- Scope: diagnostics parser hardening and deterministic fallback reporting.
- Files: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- Dependencies: Commit 1 preferred (for stable scanner surface).
- Validation gate: actionlint and SARIF parse smoke checks.

Rollback and contingency:
- Revert Commit 2 first if diagnostics formatting regresses.
- Revert Commit 1 only if scanner-scope change is not desired by policy.
- Keep enforce-gate semantics unchanged in all rollback paths.
