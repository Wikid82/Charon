# Manual Validation of E2E Test Infrastructure

- Test the following scenarios manually (or verifying via CI output):
  1. Verify `crowdsec-diagnostics.spec.ts` does NOT run in standard `chromium` shards.
  2. Verify `tests/security/acl-integration.spec.ts` passes consistently (no 401s, no modal errors).
  3. Verify `waitForModal` helper works for both standard dialogs and slide-out panels.
  4. Verify Authentication setup (`auth.setup.ts`) works with `127.0.0.1` domain.

Status: To Do
Priority: Medium
Assignee: QA Automation Team
