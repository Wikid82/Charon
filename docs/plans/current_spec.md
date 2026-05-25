## Introduction

This plan targets remediation for GitHub Actions run 26376706408, job
77639044115, scoped only to
[.github/workflows/docker-build.yml](.github/workflows/docker-build.yml).

Objectives:
- Fix PR Trivy traceability to report only Caddy version output.
- Make blocker diagnostics resilient and non-blocking while still emitting
  blocker IDs when available.
- Add a mandatory pre-change dependency discovery gate for
   needs.build-and-push.outputs.pr_image_ref and pr_image_ref contract
   references, requiring zero remaining consumers before output removal.
- Remove/deprecate build-and-push job output pr_image_ref if no longer needed
  by digest-based PR scanning flow.
- Remove stale workflow summary/error wording that references pr_image_ref so
   operator-visible messaging uses digest/image_ref language only.
- Keep scope minimal to workflow YAML edits only.

## Research Findings

### Current Workflow Behavior

1. PR scan already uses digest-only image resolution in
   [scan-pr-image Load PR image reference](.github/workflows/docker-build.yml#L787)
   and does not consume needs.build-and-push.outputs.pr_image_ref.
2. Traceability currently runs
   [Record PR Trivy scan traceability](.github/workflows/docker-build.yml#L884)
   with docker run IMAGE caddy version, which can invoke image entrypoint
   behavior and print startup logs.
3. Diagnostics currently runs
   [Diagnose unsuppressed PR Trivy blockers](.github/workflows/docker-build.yml#L906)
   with strict shell mode and jq parsing assumptions, which can fail the step
   with non-security errors (observed exit code 3 pattern).
4. build-and-push still publishes a workflow job output at
   [outputs.pr_image_ref](.github/workflows/docker-build.yml#L69), even though
   PR scan path consumes digest output only.

### Root Causes

1. Traceability ambiguity:
   docker run IMAGE caddy version may route through image startup semantics,
   causing noisy logs in addition to or instead of pure caddy version output.
2. Diagnostics fragility:
   strict shell plus jq parse assumptions can fail on malformed/missing SARIF,
   unexpected JSON shape, or command subtleties.
3. Output confusion:
   exposed job output pr_image_ref appears in workflow contract but is no
   longer required by scan-pr-image, creating warning/confusion surface.

## Technical Specification

### Files In Scope

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)

No code, docs, Dockerfile, or other workflow changes are in scope.

### Remediation 1: Traceability Should Execute Caddy Binary Directly

Target step:
- [Record PR Trivy scan traceability](.github/workflows/docker-build.yml#L884)

Required change:
- Replace traceability version command with explicit binary invocation using
  entrypoint override.
- Expected command pattern:
  - docker run --rm --pull=never --entrypoint /usr/bin/caddy IMAGE_REF version

Rationale:
- Forces direct Caddy binary execution and avoids startup/entrypoint logs in
  traceability summary.

### Remediation 2: Diagnostics Step Must Be Non-Blocking and Robust

Target step:
- [Diagnose unsuppressed PR Trivy blockers](.github/workflows/docker-build.yml#L906)

Required changes:
1. Mark step non-blocking:
   - add continue-on-error: true on this step.
2. Harden script behavior:
   - do not fail the job on jq failures or unexpected SARIF shape.
   - keep best-effort extraction of unsuppressed findings.
3. Preserve useful output:
   - when parsing succeeds, print findings and unique blocker IDs.
   - when parsing fails, print an explicit fallback message and continue.
4. Keep security gate authoritative:
   - [Enforce PR Trivy security gate](.github/workflows/docker-build.yml#L964)
     remains the only blocking gate.

Rationale:
- Prevents false negatives from diagnostic tooling failures while preserving
  blocker visibility for triage.

### Remediation 3: Deprecate/Remove pr_image_ref Job Output Path

Target section:
- [build-and-push outputs](.github/workflows/docker-build.yml#L67)

Required changes:
1. Run mandatory dependency discovery gate repo-wide for:
   - needs.build-and-push.outputs.pr_image_ref
   - pr_image_ref
   - expected gate result: zero remaining consumers of the
     needs.build-and-push.outputs.pr_image_ref contract.
2. Remove job-level output mapping for pr_image_ref only after confirming zero
   remaining consumers of needs.build-and-push.outputs.pr_image_ref.
3. Remove/deprecate stale workflow summary/error contract wording that still
   references pr_image_ref, ensuring operator-visible messaging uses
   digest/image_ref language only.
4. Keep internal step output usage inside build-and-push untouched where still
   needed for tagging and local save logic.

Rationale:
- scan-pr-image already consumes digest-only contract; removing unused job
  output reduces secret-redaction warnings and operator confusion.

## Implementation Plan

### Phase 1: Workflow Edits (YAML Only)

1. Update traceability command in
   [Record PR Trivy scan traceability](.github/workflows/docker-build.yml#L884)
   to direct binary execution.
2. Update
   [Diagnose unsuppressed PR Trivy blockers](.github/workflows/docker-build.yml#L906)
   with continue-on-error and robust best-effort parsing.
3. Remove build-and-push job output pr_image_ref from
   [outputs](.github/workflows/docker-build.yml#L67).
4. Remove/deprecate any summary/contract wording that still advertises
   pr_image_ref as downstream output.

### Phase 2: Validation

Run from repository root:

```bash
actionlint .github/workflows/docker-build.yml
```

Optional strict pass for all workflows after targeted pass:

```bash
actionlint .github/workflows/*.yml
```

### Phase 3: Runtime Confidence Check (Manual)

On next PR workflow execution:
- Confirm traceability summary shows a clean Caddy version value.
- Confirm diagnostics step never blocks the job even if parsing degrades.
- Confirm blocker IDs still appear when SARIF parsing succeeds.
- Confirm scan gating behavior remains unchanged and blocking only via
  trivy-scan outcome in enforce step.

## Acceptance Criteria

1. WHEN Record PR Trivy scan traceability runs, THE SYSTEM SHALL execute
   /usr/bin/caddy directly and report version output without entrypoint startup
   log noise.
2. WHEN diagnostics parsing encounters malformed or unexpected SARIF, THE SYSTEM
   SHALL continue execution and print a fallback diagnostic message.
3. WHEN diagnostics parsing succeeds, THE SYSTEM SHALL print blocker IDs and
   findings summary in step output and job summary.
4. WHEN PR Trivy scan finds CRITICAL/HIGH blockers, THE SYSTEM SHALL fail only
   via the enforce security gate step.
5. WHEN build-and-push publishes outputs, THE SYSTEM SHALL no longer expose
   pr_image_ref as a job output contract if digest-only flow remains active.
6. WHEN diagnostics parsing fails for any reason, THE SYSTEM SHALL always
   append a fallback diagnostics section to GITHUB_STEP_SUMMARY.

## Risks And Mitigations

- Risk: diagnostics no longer fail fast on script errors.
  - Mitigation: enforce step remains blocking and authoritative.
- Risk: hidden dependency on pr_image_ref output in external tooling.
   - Mitigation: mandatory pre-change repo-wide dependency discovery gate for
      both needs.build-and-push.outputs.pr_image_ref and pr_image_ref; remove
      output only when zero consumers remain for the former contract.

## Commit Slicing Strategy

Decision:
- Single PR with one logical commit is preferred because scope is one workflow
  file and behavior change is tightly related.

Commit 1:
- Scope: traceability invocation, diagnostics hardening, output contract
  cleanup.
- Files: [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- Dependencies: none beyond existing digest output contract.
- Validation gate:
  - actionlint .github/workflows/docker-build.yml passes.

Rollback and contingency:
- Revert only the specific workflow hunks if regression appears.
- Keep digest-based PR scan flow unchanged during rollback.
