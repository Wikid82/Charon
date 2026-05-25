## Introduction

This plan remediates PR Security Scan diagnostics fallback for GitHub Actions
job Security Scan PR Image, scoped only to
[.github/workflows/docker-build.yml](.github/workflows/docker-build.yml).

Goals:
- Determine why diagnostics reported fallback Unable to parse SARIF results
  even though SARIF existed and uploaded.
- Apply a minimal, robust workflow-only fix.
- Preserve existing gate semantics:
  - step Run Trivy scan on PR image (SARIF - blocking) may fail when
    vulnerabilities exist.
  - step Enforce PR Trivy security gate remains authoritative blocker.
- Add explicit step-15 authority invariant:
  - Enforce PR Trivy security gate must keep `if: always()`.
  - Gate decision must depend only on trivy-scan outcome.
  - Diagnostics output must not influence gate decision.
- Improve diagnostics classification to distinguish file missing, file
  unreadable, invalid JSON, unexpected schema, and parser command failure.
- Keep blocker ID extraction reliable when SARIF is valid.

## Research Findings

### Evidence Mapping

1. SARIF path and upload path are valid in the failing run:
   - trivy-pr-results.sarif is generated.
   - Upload Trivy scan results validates and uploads SARIF.
2. Diagnostics still fell back with reason Unable to parse SARIF results.
3. Gate behavior is currently correct:
   - Run Trivy scan on PR image (SARIF - blocking) returns failure when
     vulnerabilities exist.
   - Enforce PR Trivy security gate exits 1 based on trivy-scan outcome.

### Likely Root Cause

Most probable cause is parser command failure inside step
Diagnose unsuppressed PR Trivy blockers, not missing SARIF:
- The jq program used for package extraction is complex and brittle.
- A jq program parse/runtime failure triggers fallback reason
  Unable to parse SARIF results.
- Because this fallback path currently lumps parser and schema cases together,
  diagnostics lose precision.

Secondary contributor:
- Blocker ID extraction and package parsing are coupled in one jq expression,
  so non-essential package parsing failures can suppress otherwise available
  blocker IDs.

## Targeted Workflow Edits

File in scope:
- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)

Exact steps likely needing edits by name:
1. Diagnose unsuppressed PR Trivy blockers
2. Enforce PR Trivy security gate
3. Run Trivy scan on PR image (SARIF - blocking)

### Step-Level Design

### 1) Diagnose unsuppressed PR Trivy blockers

Design intent:
- Keep step non-blocking.
- Split diagnostics into explicit states with deterministic fallback reason.
- Decouple blocker ID extraction from optional package parsing.

Required logic states:
1. SARIF file missing.
2. SARIF file unreadable.
3. SARIF JSON invalid.
4. SARIF schema unexpected.
5. Parser command failure.
6. Parsed successfully.

Design details:
- Perform ordered checks:
  - file exists
  - file readable
  - JSON validity
  - valid SARIF parse requires `.runs` to be an array
  - missing or empty `.runs[].results` is treated as parsed with count 0
    (not fallback)
  - parser execution status code
- Use one jq expression dedicated to blocker IDs
  from unsuppressed results:
  - id source order: .ruleId then .rule.id then unknown
  - unique and stable ordering for summary output.
- Package extraction remains best effort and cannot block ID extraction.
- Always append diagnostics block to GITHUB_STEP_SUMMARY.

### 2) Run Trivy scan on PR image (SARIF - blocking)

No semantic change:
- continue-on-error remains true.
- exit-code remains 1 for CRITICAL/HIGH findings.

### 3) Enforce PR Trivy security gate

No semantic change:
- This step remains the only authoritative blocking gate.
- It still fails when trivy-scan outcome is not success.
- Step-15 authority invariant is explicit and preserved:
  - `if: always()` must remain on this gate step.
  - Diagnostics status, fallback reason, and summary text are non-authoritative
    and cannot affect pass/fail.

## Implementation Phases

### Phase 1: Failure Taxonomy Refactor (Diagnostics Only)

Update only step Diagnose unsuppressed PR Trivy blockers to:
- emit explicit diagnostics_status and fallback_reason categories.
- report parser exit code when parser command failure occurs.
- keep continue-on-error true.

### Phase 2: Reliable Blocker ID Extraction

Refactor extraction flow in same step so:
- blocker IDs are extracted by a minimal jq path that tolerates SARIF
  field variability.
- package/detail enrichment runs independently and does not affect IDs.

### Phase 3: Gate Semantics Preservation Check

Confirm workflow wiring remains:
- Run Trivy scan on PR image (SARIF - blocking) may fail for vulnerabilities.
- Enforce PR Trivy security gate remains final blocking decision point.
- Step-15 authority invariant holds:
  - gate uses `if: always()`.
  - gate fails only when trivy-scan outcome is not success.
  - diagnostics output does not influence gate decision.

## Validation Commands

Run from repository root:

```bash
actionlint .github/workflows/docker-build.yml
```

Optional full workflow lint sweep:

```bash
actionlint .github/workflows/*.yml
```

Optional local parser sanity checks for regression prevention:

```bash
jq -e '.' trivy-pr-results.sarif
jq -r '.runs | type' trivy-pr-results.sarif
```

## Expected Runtime Outcomes

1. If SARIF is absent:
   - diagnostics summary shows fallback reason SARIF file missing.
   - gate decision still depends only on trivy-scan outcome.
2. If SARIF exists but cannot be read:
  - diagnostics summary shows fallback reason SARIF file unreadable.
3. If SARIF exists but JSON is malformed:
   - diagnostics summary shows fallback reason SARIF JSON invalid.
4. If SARIF JSON is valid but structure is unexpected:
   - diagnostics summary shows fallback reason SARIF schema unexpected.
5. If jq/parser command fails:
   - diagnostics summary shows fallback reason parser command failure and
     includes parser exit code.
6. If SARIF is valid and expected:
  - `.runs` is treated as authoritative parse indicator.
  - missing or empty `.runs[].results` reports parsed status with blocker
    count 0 (not fallback).
   - diagnostics summary shows parsed status and reliable blocker ID list.
7. For CRITICAL/HIGH findings:
   - step Run Trivy scan on PR image (SARIF - blocking) may report failure.
   - step Enforce PR Trivy security gate exits 1 and blocks merge.
8. Step-15 authority invariant:
  - Enforce PR Trivy security gate keeps `if: always()`.
  - only trivy-scan outcome determines gate pass/fail.
  - diagnostics output cannot change gate pass/fail.

## Acceptance Criteria (EARS)

1. WHEN step Diagnose unsuppressed PR Trivy blockers runs and SARIF file is
   missing, THE SYSTEM SHALL emit diagnostics status fallback with reason
   SARIF file missing.
2. WHEN SARIF exists but is unreadable, THE SYSTEM SHALL emit diagnostics
  status fallback with reason SARIF file unreadable.
3. WHEN SARIF exists but is not valid JSON, THE SYSTEM SHALL emit diagnostics
   status fallback with reason SARIF JSON invalid.
4. WHEN SARIF is valid JSON but not in expected SARIF shape,
   THE SYSTEM SHALL emit diagnostics status fallback with reason
   SARIF schema unexpected.
5. WHEN diagnostics parser command fails for any other reason,
   THE SYSTEM SHALL emit diagnostics status fallback with reason
   parser command failure and include parser exit code.
6. WHEN `.runs` is an array and `.runs[].results` is missing or empty,
  THE SYSTEM SHALL classify diagnostics as parsed and report blocker count 0
  instead of fallback.
7. WHEN SARIF is valid and parseable,
   THE SYSTEM SHALL extract and report unique unsuppressed blocker IDs.
8. WHEN vulnerabilities are detected,
   THE SYSTEM SHALL allow step Run Trivy scan on PR image (SARIF - blocking)
   to fail while keeping Enforce PR Trivy security gate as the sole
   authoritative merge blocker.
9. WHEN step 15 (Enforce PR Trivy security gate) executes,
  THE SYSTEM SHALL keep `if: always()` and SHALL fail only when
  trivy-scan outcome is not success.
10. WHEN diagnostics output changes (status, fallback reason, summary text),
   THE SYSTEM SHALL NOT alter step-15 gate pass/fail behavior.

## Commit Slicing Strategy

Decision:
- Single PR with one logical commit.

Commit 1:
- Scope: diagnostics hardening and blocker extraction robustness in
  [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml).
- Files: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- Dependencies: none.
- Validation gate: actionlint .github/workflows/docker-build.yml passes.

Rollback and contingency:
- Revert only the diagnostics step edits.
- Keep trivy-scan and enforce-gate step semantics unchanged.
