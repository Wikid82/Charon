# Beta Release Draft Pull Request

## Overview
Draft PR to merge hardened CI/release workflow changes from `feature/beta-release` into `feature/alpha-completion`.

## Highlights
- Secret token migration: all workflows now use `PROJECT_TOKEN` (GitHub blocks new secrets containing `GITHUB`).
- Release workflow refinements: stable prerelease detection (alpha/beta/rc), artifact matrix intact.
- Prior infra hardening (already partially merged earlier): pinned GitHub Action SHAs/tags, resilient Delve (`dlv`) multi-arch build handling.
- Extensive incremental documentation trail in `docs/beta_release_draft_pr.md` plus concise snapshot in `docs/beta_release_draft_pr_body_snapshot.md` for reviewers.

## Ahead Commits (Representative)
Most recent snapshot commit: `b8154456` (PR body content file before opening). Full ordered list in `docs/beta_release_draft_pr.md`.

## Review Checklist
- Secret `PROJECT_TOKEN` exists and has required scopes.
- No lingering `GITHUB_TOKEN` references beyond allowed GitHub-provided contexts.
- Artifact list (frontend dist, backend binaries, caddy binaries) still correct for release.

## Risks & Mitigations
- Secret rename: Mitigate by verifying secret presence before merge.
- Workflow call path validity: `docker-publish.yml` referenced locally; CI on draft will validate end-to-end.

## Deferred Items (Out of Scope Here)
- Frontend test coverage improvements (ProxyHostForm).
- Additional beta observability and import validation tasks.

## Actions After Approval
1. Confirm CI draft run passes.
2. Convert PR from draft to ready-for-review.
3. Merge into `feature/alpha-completion`.

## Request
Please focus review on secret usage, workflow call integrity, and artifact correctness. Comment with any missed token references.

---
Generated programmatically to aid structured review.