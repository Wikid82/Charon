 Tasks

Repository: Wikid82/Charon
Branch: feature/beta-release

Purpose
-------
Create a tracked issue and sub-tasks to validate ACL-related changes introduced on the `feature/beta-release` branch. This file records the scope, test steps, and sub-issues so we can open a GitHub issue later or link this file in the issue body.

Top-level checklist
- [ ] Open GitHub Issue "ACL: Test and validate ACL changes (feature/beta-release)" and link this file
- [ ] Assign owner and target date

Sub-tasks (suggested GitHub issue checklist items)
1) Unit & Service Tests
   - [ ] Add/verify unit tests for `internal/services/access_list_service.go` CRUD + validation
   - [ ] Add tests for `internal/api/handlers/access_list_handler.go` endpoints (create/list/get/update/delete)
   - Acceptance: all handler tests pass and coverage for `internal/api/handlers` rises by at least 3%.

2) Integration Tests
   - [ ] Test ACL interactions with proxy hosts: ensure blocked/allowed behavior when ACLs applied to hosts
   - [ ] Test ACL import via Caddy import workflow (multi-site) — ensure imported ACLs attach correctly
   - Acceptance: end-to-end requests are blocked/allowed per ACL rules in an integration harness.

3) UI & API Validation
   - [ ] Validate frontend UI toggles for ACL enable/disable reflect DB state
   - [ ] Verify API endpoints that toggle ACL mode return correct status and persist in `settings`
   - Acceptance: toggles update DB and the UI shows consistent state after refresh.

4) Security & Edge Cases
   - [ ] Test denied webhook payloads / WAF interactions when ACLs are present
   - [ ] Confirm rate-limit and CrowdSec interactions do not conflict with ACL rules
   - Acceptance: no regressions found; documented edge cases.

5) Documentation & Release Notes
   - [ ] Update `docs/features.md` with any behavior changes
   - [ ] Add a short note in release notes describing ACL test coverage and migration steps

Manual Test Steps (quick guide)
- Set up local environment:
  1. `cd backend && go run ./cmd/api` (or use docker compose)
  2. Run frontend dev server: `cd frontend && npm run dev`
- Create an ACL via API or UI; attach it to a Proxy Host; verify request behavior.
- Import Caddyfiles (single & multi-site) with ACL directives and validate mapping.

Issue metadata (suggested)
- Title: ACL: Test and validate ACL changes (feature/beta-release)
- Labels: testing, needs-triage, acl, regression
- Assignees: @<owner-placeholder>
- Milestone: to be set

Notes
- Keep this file as the canonical checklist and paste into the GitHub issue body when opening the issue.
