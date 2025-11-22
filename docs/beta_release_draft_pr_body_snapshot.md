# Beta Release Draft Pull Request

## Overview
This draft PR merges recent beta preparation changes from `feature/beta-release` into `feature/alpha-completion` to align the alpha integration branch with the latest CI, workflow, and release process improvements.

## Changes Included (Summary)
- Workflow token migration (`GITHUB_TOKEN` → `PROJECT_TOKEN`) across release and maintenance workflows.
- Stabilized release workflow prerelease detection and artifact publication.
- Prior (already merged earlier) CI enhancements: pinned action versions, Docker multi-arch debug tooling reliability, dynamic `dlv` binary resolution.
- Documentation updates enumerating each incremental workflow/token adjustment for auditability.

## Commits Ahead of `feature/alpha-completion`
(See `docs/beta_release_draft_pr.md` for full enumerated list.) Latest unique commit: `75beff09` (updated body snapshot).

## Rationale
Ensures alpha integration branch inherits hardened CI/release pipeline and updated secret naming policy before further alpha feature consolidation.

## Risk & Mitigation
- Secret Name Change: Requires `PROJECT_TOKEN` to exist. Mitigation: Verify secret presence before merge.
- Workflow Fan-out: Reusable workflow path validated locally; CI run (draft) will confirm.

## Follow-ups (Out of Scope)
- Frontend test coverage improvements (ProxyHostForm).
- Additional beta observability and import validation tasks.

## Checklist
- [x] YAML lint (pre-commit passed)
- [x] Secret reference consistency
- [x] Release artifact list intact
- [ ] Draft PR CI run (pending after opening)

## Requested Review Focus
1. Confirm `PROJECT_TOKEN` availability.
2. Sanity-check release artifact matrix remains correct.
3. Spot any residual `GITHUB_TOKEN` references missed.

---
Generated draft to align branches; will convert to ready-for-review after validation.