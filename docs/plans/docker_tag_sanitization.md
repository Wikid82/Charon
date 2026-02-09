# Docker Tag Sanitization Plan

## 1. Introduction

### Overview
Harden Docker tag generation in the CI pipeline by sanitizing all tag inputs to comply with Docker tag rules.

### Objectives
- Ensure generated tags only use the allowed character set.
- Prevent invalid leading characters and enforce max length.
- Keep tag behavior deterministic for branches, PRs, and overrides.

## 2. Research Findings

### Current Tag Computation
The tag generation logic is in the build job in [.github/workflows/ci-pipeline.yml](.github/workflows/ci-pipeline.yml) under the "Compute image tags" step. It derives tags from:
- `github.ref_name` (branches and tags)
- `github.head_ref` for pull requests
- `inputs.image_tag_override`
- `SHORT_SHA` and `PR_NUMBER`

A helper `sanitize_tag()` exists, but it currently only allows lowercase `a-z0-9-`, strips other characters, and is applied to branch-derived tags. The override tag and `DEFAULT_TAG` derived from overrides/PRs are not sanitized.

### Invalid Character Sources
Potential invalid characters can appear in:
- Branch names (`github.ref_name`, `github.head_ref`) which can contain `/`, `@`, or other characters.
- Manual `inputs.image_tag_override` which is unvalidated user input.
- Any tag concatenation that uses raw `DEFAULT_TAG`.

### Current Gaps
- `inputs.image_tag_override` is not sanitized.
- Tag rules allow `[A-Za-z0-9_.-]`, but the current sanitization removes `_` and `.`.
- Leading period or dash is not explicitly disallowed for all tag paths.
- Max length is partially enforced for branch-derived tags but not for `DEFAULT_TAG`.

## 3. Technical Specifications

### 3.1 Requirements (EARS)
- WHEN the build job computes Docker tags, THE SYSTEM SHALL sanitize every tag component to allow only `[A-Za-z0-9_.-]`.
- WHEN a tag starts with `.` or `-`, THE SYSTEM SHALL remove leading invalid characters until the tag begins with `[A-Za-z0-9_]`.
- WHEN a computed tag exceeds 128 characters, THE SYSTEM SHALL truncate it to 128 characters.
- WHEN a tag becomes empty after sanitization, THE SYSTEM SHALL fall back to a safe default tag (`sha-<short-sha>`).
- WHEN an `image_tag_override` is provided, THE SYSTEM SHALL sanitize it and use the sanitized value for all downstream tags.

### 3.2 Tag Sanitization Rules
- Allowed characters: `[A-Za-z0-9_.-]`.
- Disallowed characters are replaced with `-`.
- Consecutive `-` are collapsed to a single `-`.
- No leading `.` or `-`.
- Maximum length: 128 characters (inclusive).
- Case policy: preserve case (uppercase allowed).

### 3.3 Sanitization Order
1. Replace invalid characters.
2. Collapse consecutive dashes.
3. Trim leading `.` and `-`.
4. Truncate to 128 characters.

### 3.4 Update Scope
- Update `sanitize_tag()` and its call sites in the build job.
- Apply sanitization to `DEFAULT_TAG`, `BRANCH_TAG`, `BRANCH_SHA_TAG`, and `SHORT_SHA` use in tag strings.
- Ensure any `image_tag_override` goes through sanitization.

### 3.5 Error Handling
- If a tag becomes empty after sanitization, default to `sha-<short-sha>`.
- Validate and fail fast if the final tag list is empty or contains whitespace (existing checks stay in place).

### 3.6 Data Flow (Tag Inputs to Outputs)
- Inputs: `github.ref_name`, `github.head_ref`, `inputs.image_tag_override`, `github.sha`, `github.event.pull_request.number`.
- Processing: `sanitize_tag()` transforms each input into a compliant tag component.
- Outputs: Docker tag strings used by `docker/build-push-action` and job outputs.

## 4. Implementation Plan

### Phase 1: Playwright Tests
- Not applicable (CI pipeline change only). Document as N/A.

### Phase 2: Backend Implementation
- Not applicable.

### Phase 3: Frontend Implementation
- Not applicable.

### Phase 4: Integration and Testing
- Add a shell-based self-check inside the CI step (dry validation only) or local verification script (optional) that prints and validates tags.
- Validate that tags generated for branch names with `/` and `@` sanitize correctly.
- Validate that override tags with invalid characters are sanitized and not empty.

### Phase 5: Documentation and Deployment
- Update CI workflow in [.github/workflows/ci-pipeline.yml](.github/workflows/ci-pipeline.yml) to apply the sanitization rules.
- No external documentation updates required unless release notes or changelog policy mandates it.

## 5. Acceptance Criteria

- All computed tags only contain `[A-Za-z0-9_.-]` and do not start with `.` or `-`.
- Tags never exceed 128 characters.
- Tags derived from branch names with `/` are sanitized deterministically.
- `image_tag_override` is sanitized and used consistently.
- Build job still produces a non-empty, whitespace-free tag list for all supported workflows.

## 6. Risks and Mitigations

- Risk: Lowercasing tags may change existing tag semantics.
  - Mitigation: Confirm policy in Open Questions before implementation.
- Risk: Truncation could cause collisions for very long branch names.
  - Mitigation: Add suffixing with `SHORT_SHA` when truncating branch tags.

## 7. Open Questions

1. Should we keep `feature/*` tag behavior unchanged (post-sanitization) or alter it for long names?
