## CI Planning Spec: PR Image Output Contract Hardening

Date: 2026-05-24
Branch context: feature/hecate
Target workflow: .github/workflows/docker-build.yml
Failing job: Security Scan PR Image
Failing step signal: Missing PR image reference from build-and-push outputs

## Introduction

This plan defines a long-term, contract-first fix for the CI failure where
scan-pr-image receives an empty value for
needs.build-and-push.outputs.pr_image_ref.

Goal:
- Ensure pr_image_ref is always emitted when scan-pr-image is eligible to run.
- Preserve correct job dependency and condition behavior for pull_request, push,
	and workflow_run triggers.
- Improve observability so producer/consumer output failures become immediately
	diagnosable.

Out of scope:
- Changing vulnerability thresholds or Trivy policy behavior.
- Refactoring unrelated Docker build or release logic.

## Research Findings

### Evidence sources reviewed

- Workflow producer and consumer chain:
	.github/workflows/docker-build.yml
- Failing execution log:
	.github/logs/ci_failure.log
- Ancillary files requested for necessity review:
	.gitignore, .dockerignore, codecov.yml, Dockerfile

### Failure evidence summary

- The failing job is Security Scan PR Image.
- In the failing run, Load PR image reference executes with:
	IMAGE_REF="" and exits with:
	Missing PR image reference from build-and-push outputs.
- Trigger context in the same log confirms PR semantics:
	TRIGGER_EVENT=pull_request,
	TRIGGER_PR_NUMBER=1035,
	TRIGGER_HEAD_REF=feature/hecate.

### Producer to consumer mapping

| Stage | Workflow location | Contract detail | Risk observed |
|---|---|---|---|
| Producer job output | docker-build.yml build-and-push.outputs.pr_image_ref | Mapped from steps.pr-image-ref.outputs.image_ref | Empty at consumer in failing run |
| Producer step | docker-build.yml step id pr-image-ref | Sets image_ref from first line of steps.meta.outputs.tags | First-line selection can be brittle |
| Consumer job gate | docker-build.yml scan-pr-image.if | Requires build success, skip_build != true, github.event_name == pull_request | Does not assert non-empty pr_image_ref |
| Consumer step | docker-build.yml Load PR image reference | Reads needs.build-and-push.outputs.pr_image_ref and fails if empty | Current hard failure point |

## Root Cause Candidates (with evidence mapping)

### Candidate A: Brittle PR tag extraction strategy

Observation:
- The producer step resolves IMAGE_REF using head -n 1 from
	steps.meta.outputs.tags.

Evidence mapping:
- Source: docker-build.yml Resolve PR image reference step.
- Failure signal: downstream receives empty output.

Why plausible:
- Metadata tags are multiline and include multiple tag classes and registries.
- Selecting only the first line is order-dependent and not contract-driven.
- Any change in tag ordering, formatting, or empty leading line can break
	producer output without a compile-time error.

### Candidate B: Output contract not validated before downstream usage

Observation:
- scan-pr-image enforces non-empty output, but build-and-push does not enforce
	a terminal output contract assertion tied to PR execution.

Evidence mapping:
- Source: docker-build.yml scan-pr-image Load PR image reference step.
- Source: docker-build.yml lacks a final PR output assertion step.

Why plausible:
- An empty producer output can reach the next job, moving failure farther from
	origin and reducing diagnosability.

### Candidate C: Job/step id and output reference fragility

Observation:
- Producer and internal references use hyphenated step id naming.

Evidence mapping:
- Source: docker-build.yml ids and output references around pr-image-ref.

Why plausible:
- Hyphenated ids are valid, but are more error-prone during future edits,
	especially when copied into complex expressions.
- Using a canonical underscore id and an explicit output emitter step reduces
	long-term maintenance risk.

## Preferred Long-Term Fix

### Decision

Adopt a contract-first producer model for pr_image_ref in build-and-push, then
consume that contract in scan-pr-image with explicit gating and diagnostics.

### Design overview

1. Replace first-line tag selection with deterministic PR-tag resolution
	 and validation against metadata tag list.
2. Emit pr_image_ref from a dedicated producer step with stable id naming.
3. Add a producer-side assertion step so PR runs fail in build-and-push if
	 pr_image_ref is empty.
4. Keep scan-pr-image as dependent consumer, but gate it with explicit
	 non-empty output check for defensive correctness.
5. Add summary telemetry for emitted pr_image_ref.

### High-level YAML shape (targeted snippets)

In build-and-push (producer):

```yaml
# after metadata generation
- name: Resolve PR image reference (contract)
	if: steps.skip.outputs.skip_build != 'true' && env.TRIGGER_EVENT == 'pull_request'
	id: resolve_pr_image_ref
	run: |
		# 1) read docker/metadata-action multiline tags output
		# 2) select tags matching anchored PR pattern
		#    ^ghcr\.io/[^[:space:]]+:pr-[0-9]+-[0-9a-f]{7,}$
		# 3) assert exactly one match (fail on zero or multiple)
		# 4) emit that single match as image_ref (source of truth)
		# 5) reconstructed expected tag is diagnostics only (not selection input)

- name: Assert PR output contract
	if: steps.skip.outputs.skip_build != 'true' && env.TRIGGER_EVENT == 'pull_request'
	run: |
		test -n "${{ steps.resolve_pr_image_ref.outputs.image_ref }}"
```

Update build job outputs mapping:

```yaml
outputs:
	pr_image_ref: ${{ steps.resolve_pr_image_ref.outputs.image_ref }}
```

In scan-pr-image (consumer gate hardening):

```yaml
if: >-
	needs.build-and-push.outputs.skip_build != 'true' &&
	needs.build-and-push.result == 'success' &&
	github.event_name == 'pull_request' &&
	needs.build-and-push.outputs.pr_image_ref != ''
```

Consumer load step remains, but now should only fail for unexpected contract
breaches.

### Why this is preferred

- Removes dependence on implicit metadata ordering.
- Uses docker/metadata-action output as the sole source of truth for PR image
	reference selection.
- Enforces cardinality contract (exactly one anchored PR tag), preventing
	ambiguous or missing producer outputs.
- Fails fast in the producer job where root cause exists.
- Preserves existing event model while adding stronger invariants.
- Scales better as additional tag types are introduced.

## Rejected Alternative(s)

### Rejected A: Fallback compose image ref directly inside scan-pr-image

Example concept:
- If needs.build-and-push.outputs.pr_image_ref is empty, recompute expected ref
	in scan-pr-image and continue.

Reason rejected:
- Masks producer contract failures.
- Duplicates tag-construction logic across jobs.
- Increases drift risk between build and scan behavior.
- Converts a deterministic producer bug into silent consumer complexity.

### Rejected B: Remove separate scan job and perform PR scan only in build-and-push

Reason rejected:
- Reduces orchestration clarity and separation of concerns.
- Increases blast radius and complexity of build job retries/timeouts.
- Not necessary when producer contract is made explicit.

## Exact Files and Sections to Change

### Primary change file

- .github/workflows/docker-build.yml

Sections:
- build-and-push job outputs block
- Resolve PR image reference step (rename/refactor)
- New producer contract assertion step
- scan-pr-image job if condition hardening
- Optional summary line showing emitted pr_image_ref

### No code changes expected outside workflow

- No backend/frontend/runtime behavior changes planned.
- No schema/API/component code changes planned.

## Trigger and Dependency Behavior Matrix

| Trigger | build-and-push | pr_image_ref expected | scan-pr-image should run | Expected behavior |
|---|---|---|---|---|
| pull_request | yes (unless skip true) | non-empty | yes when skip false and build success | PR image scanned with Trivy |
| push (main/development) | yes | empty allowed | no | digest-based non-PR scans only |
| workflow_run (Docker Lint completed) | conditional | empty allowed unless explicitly PR-mode future change | no (current guard) | no PR image scan job |

## Implementation Plan (Phased)

### Phase 1: Playwright/UI Validation Scope

This change is CI workflow only. No product-surface files are changed.

Action:
- Record UI test waiver for this PR based on scope.

### Phase 2: Producer Contract Refactor

Tasks:
1. Refactor PR image ref resolution into a deterministic contract step.
2. Replace brittle first-line tag extraction with explicit PR-tag matching.
3. Emit stable image_ref output from canonical step id.

Complexity: Medium

### Phase 3: Consumer Gate Hardening

Tasks:
1. Add non-empty output condition to scan-pr-image job if expression.
2. Keep existing consumer-level empty check for defense in depth.

Complexity: Low

### Phase 4: Validation and Regression Matrix

Tasks:
1. Validate pull_request path (PR #1035-like scenario).
2. Validate push path (main/development).
3. Validate workflow_run path (Docker Lint completed).

Complexity: Medium

### Phase 5: Documentation and Handoff

Tasks:
1. Update plan and implementation notes in this file.
2. Prepare supervisor review packet with evidence and validation outcomes.

Complexity: Low

## Validation Strategy

### PR validation

Required checks:
1. build-and-push emits non-empty pr_image_ref.
2. scan-pr-image runs and logs resolved IMAGE_REF value.
3. Trivy PR scan uses same image reference.
4. Step summary includes emitted pr_image_ref and trigger SHA.

Producer contract assertion validation case:
1. Provide producer metadata tags input with either:
	- zero anchored PR matches, or
	- more than one anchored PR match.
2. Verify build-and-push fails at producer contract assertion with diagnostics
	that include:
	- anchored pattern used,
	- total PR-tag match count,
	- matched candidate list,
	- reconstructed expected tag marked as diagnostics-only.
3. Verify scan-pr-image does not run because build-and-push result is failed.
4. Verify the failure message explicitly points to producer output generation
	(resolve/emitter step), not consumer loading.

Failure diagnostics to capture:
- metadata tags output used for resolution
- computed expected PR tag
- final emitted pr_image_ref

### Push validation

Required checks:
1. build-and-push succeeds for main/development.
2. scan-pr-image does not run.
3. Existing digest-based scan path remains unchanged.

### workflow_run validation

Required checks:
1. build-and-push honors existing workflow_run guard conditions.
2. scan-pr-image remains skipped under current event guard.
3. No false failure due to absent pr_image_ref in non-PR context.

### Regression checks

- Confirm skip_build=true still prevents scan-pr-image execution.
- Confirm forced refresh logic remains unaffected.

## EARS Requirements

- WHEN the workflow trigger is pull_request AND build-and-push is eligible,
	THE SYSTEM SHALL select exactly one GHCR tag from docker/metadata-action
	outputs matching the anchored PR pattern shown below and emit it as
	pr_image_ref.

	```regex
	^ghcr\.io/[^[:space:]]+:pr-[0-9]+-[0-9a-f]{7,}$
	```
- WHEN scan-pr-image is evaluated for pull_request,
	THE SYSTEM SHALL only run if pr_image_ref is non-empty.
- IF zero OR more than one anchored PR tag matches are found in pull_request,
	THEN THE SYSTEM SHALL fail build-and-push with clear producer-side
	diagnostics and SHALL NOT run scan-pr-image.
- IF pr_image_ref cannot be emitted from producer output generation in
	build-and-push for pull_request,
	THEN THE SYSTEM SHALL fail build-and-push with clear producer-side
	diagnostics that identify producer output generation as the failure origin.
- WHEN trigger is push or workflow_run,
	THE SYSTEM SHALL NOT require pr_image_ref for downstream non-PR scanning logic.
- THE SYSTEM SHALL preserve existing skip_build and dependency semantics.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Anchored PR pattern mismatch against metadata output | Producer emits no valid scan image ref | Validate exact-one match contract and fail in producer with actionable diagnostics |
| Overly strict consumer condition hides producer issues | Missed scans | Keep producer assertion and explicit summary logging |
| Future metadata format change | Output regression | Contract step validates shape and fails with actionable diagnostics |

## Commit Slicing Strategy

Decision:
- Single PR with ordered logical commits.
- Reason: change is tightly scoped to one workflow contract and should be
	reviewed atomically.

Trigger reasons:
- Medium risk due cross-job output contract.
- Cross-domain CI logic (producer + consumer job conditions).
- Review size remains manageable in one PR.

Ordered commits:

1. Commit 1: Producer contract hardening
- Scope: Refactor PR image reference resolution and output emission.
- Files: .github/workflows/docker-build.yml
- Dependencies: none
- Validation gate: PR dry-run ensures non-empty output and clear logs.

2. Commit 2: Consumer condition hardening
- Scope: Update scan-pr-image job if condition and retain load guard.
- Files: .github/workflows/docker-build.yml
- Dependencies: Commit 1
- Validation gate: PR run executes scan-pr-image only with non-empty output.

3. Commit 3: Validation telemetry and final polish
- Scope: Add/adjust summaries for traceability and finalize comments.
- Files: .github/workflows/docker-build.yml
- Dependencies: Commits 1-2
- Validation gate: PR, push, workflow_run behavior matrix verified.

Rollback and contingency:
- If PR scan stops running unexpectedly, revert Commit 2 first to restore
	current consumer scheduling while preserving producer diagnostics.
- If producer logic mis-resolves tags, revert Commit 1 and retain previous
	behavior temporarily, then patch with metadata-pattern validation fix.

## Necessity Review: .gitignore, codecov.yml, .dockerignore, Dockerfile

### .gitignore

No update required.

Reason:
- Failure is workflow output contract related, not artifact tracking.

### codecov.yml

No update required.

Reason:
- Coverage policy is unrelated to PR image ref propagation.

### .dockerignore

No update required.

Reason:
- Build context exclusions do not influence workflow output expressions.

### Dockerfile

No update required.

Reason:
- Failure occurs before image scanning due missing reference propagation,
	not image build definition.

## Acceptance Criteria

1. In pull_request runs, build-and-push emits pr_image_ref only after selecting
	exactly one anchored PR tag from docker/metadata-action output.
2. scan-pr-image only runs when pr_image_ref is non-empty and build succeeded.
3. For push and workflow_run, scan-pr-image remains correctly skipped.
4. Producer contract failure path is validated: build-and-push fails with
	producer diagnostics, scan-pr-image does not run, and the failure message
	points to producer output generation.
5. Workflow behavior remains backward compatible for non-PR digest scans.
6. DoD checks for this CI-only change pass without introducing unrelated file
	 modifications.

## Handoff Note

This plan is implementation-ready and scoped for supervisor review.
