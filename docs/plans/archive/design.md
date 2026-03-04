## Design - Playwright Triage and Remediation Plan

Source: [docs/plans/current_spec.md](docs/plans/current_spec.md)

## Architecture Overview

This plan stabilizes the Playwright harness before addressing product defects.
It defines a failure taxonomy to separate code fixes from test fixes, structures
test subsets into Tier 0-4 suites for staged execution, and documents modal
dropdown remediation patterns and notification template resolution behavior.

## Failure Taxonomy

Classification axes:

- Source of truth
	- Code fix: product behavior is incorrect or missing.
	- Test fix: test is stale, mislocated, or asserts non-existent behavior.
- Failure mode
	- Deterministic: reproducible with the same steps.
	- Infra or harness: run ends early, context closed, baseURL or auth mismatch.

Triage rule: Address harness failures before product or test changes.

## Test Subset Architecture (Tier 0-4)

Tier 0: Harness stability
- tests/global-setup.ts
- tests/auth.setup.ts
- tests/utils/wait-helpers.spec.ts

Tier 1: ProxyHostForm focus
- tests/core/proxy-hosts.spec.ts
- tests/proxy-host-dropdown-fix.spec.ts
- tests/modal-dropdown-triage.spec.ts

Tier 2: Uptime focus
- tests/monitoring/uptime-monitoring.spec.ts

Tier 3: Notifications focus
- tests/settings/notifications.spec.ts

Tier 4: Narrow regression sampling
- tests/settings/account-settings.spec.ts
- tests/tasks/backups-create.spec.ts

Execution notes:
- Use a single browser (firefox) during triage.
- Run workers=1 for suites that share state or are ordering sensitive.

## Harness Stability Prerequisites

- Base URL alignment: auth cookie domain and browser baseURL must match.
- Setup and teardown sequencing must not close contexts still in use.
- Global setup must avoid emergency resets during active runs.
- E2E container must be rebuilt when app or Docker inputs change.

## Modal Dropdown Fix Patterns (Radix Select Portal Strategy)

Pattern goals:
- Avoid native select elements inside pointer-events-none modal layers.
- Render dropdown content via portal to escape modal stacking context.
- Ensure the trigger and content use consistent z-index and focus styles.

Radix Select strategy:
- Use Radix Select for modal dropdowns.
- Configure portal rendering for dropdown content.
- Validate pointer and keyboard interaction paths (open, navigate, select).

## Notification Template Resolution Flow

Behavioral flow:
1. UI selects provider defaults and template (minimal or detailed).
2. Backend resolves template with provider-specific requirements.
3. Preview and test send use the same resolution and validation logic.
4. Discord and Slack defaults must include required content fields.

Validation outcomes:
- Discord payload contains content or embeds.
- Slack payload contains text.
- Preview payload matches test send payload for the same inputs.

## Domain/DNS Suite Stabilization

Approach:

- Use authenticated fixtures for domain and DNS provider workflows.
- Seed domain and DNS provider data via API before UI assertions.
- Add API readiness waits for list and mutation endpoints before UI interaction.
- Use dialog waits for DNS provider form and deletion confirmation flows.

## Traceability

- Requirements: [docs/plans/requirements.md](docs/plans/requirements.md)
- Tasks: [docs/plans/tasks.md](docs/plans/tasks.md)

## Execution Outcomes

- Harness stability fixes completed, resolving context closure failures.
- ProxyHostForm and Uptime validation confirmed using Radix Select.
- Notifications templates aligned with fallback defaults for provider requirements.
- Role gating bug fix applied for the Backups empty state.
- 135 Playwright tests passing across 5 tiers.

## Remaining Phases

- Phase 6: Documentation and hygiene review.
- Phase 7: Validation and coverage gates.
