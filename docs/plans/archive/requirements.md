## Requirements - Playwright Triage and Remediation Plan

Source: [docs/plans/current_spec.md](docs/plans/current_spec.md)

## EARS Requirements

1. WHEN Tier 0 through Tier 4 test subsets are executed, THE SYSTEM SHALL complete each tier without early termination or context-closed failures.
2. WHEN the ProxyHostForm modal dropdowns are opened, THE SYSTEM SHALL allow selection changes using pointer input.
3. WHEN the ProxyHostForm modal dropdowns are opened, THE SYSTEM SHALL allow keyboard navigation with visible focus for open, navigate, and select actions.
4. WHEN the Uptime create monitor modal dropdown is opened, THE SYSTEM SHALL allow selecting a monitor type without click failures.
5. WHEN the Uptime create monitor modal dropdown is opened, THE SYSTEM SHALL allow keyboard navigation with visible focus for open, navigate, and select actions.
6. WHEN a Discord provider uses the default minimal template, THE SYSTEM SHALL send a payload that contains content or embeds as required.
7. WHEN a Slack provider uses the default minimal template, THE SYSTEM SHALL send a payload that contains text as required.
8. WHEN provider preview is requested, THE SYSTEM SHALL validate and render the same payload shape as test send for the same provider and template.
9. WHEN the Domains page is loaded, THE SYSTEM SHALL allow creating a domain via the add form and reflect it after the create API completes.
10. WHEN a domain delete action is confirmed, THE SYSTEM SHALL remove the domain after the delete API completes.
11. WHEN the DNS Providers page is loaded, THE SYSTEM SHALL render provider cards for API-seeded providers and load provider types when the add form opens.

## Execution Outcomes

- Harness stability fixes completed, resolving context closure failures.
- ProxyHostForm and Uptime validation confirmed using Radix Select.
- Notifications templates aligned with fallback defaults for provider requirements.
- Role gating bug fix applied for the Backups empty state.
- 135 Playwright tests passing across 5 tiers.

## Remaining Phases

- Phase 6: Documentation and hygiene review.
- Phase 7: Validation and coverage gates.
