# Manual Test Plan: Governance Slice (GORM DoD Gate + Gotify Token Hygiene)

Date: 2026-02-20
Scope: Documentation-only validation

## Goal
Verify that governance wording is present, consistent, and enforceable across canonical instructions, agent docs, and operator docs.

## Manual Verification Checklist

### 1) GORM conditional gate wording + trigger matrix in canonical docs
- [ ] Open `.github/instructions/testing.instructions.md`.
- [ ] Confirm section `## 4. GORM Security Validation (Manual Stage)` exists.
- [ ] Confirm `When to Run (Conditional Trigger Matrix)` exists.
- [ ] Confirm Include triggers mention:
  - `backend/internal/models/**`
  - backend services/repositories with GORM query logic
  - migrations/seeding affecting persistence behavior
- [ ] Confirm Explicit Exclusions mention docs-only and frontend-only changes.
- [ ] Confirm Gate Decision Rule uses IF/THEN semantics for include vs exclude cases.

### 2) Check-mode blocking semantics presence
- [ ] In `.github/instructions/testing.instructions.md`, confirm wording states policy is process-blocking even in manual stage.
- [ ] Confirm gate decisions must use check semantics (`--check` or equivalent task wiring).
- [ ] In `.github/instructions/copilot-instructions.md`, confirm `1.5. GORM Security Scan (CONDITIONAL, BLOCKING)` exists.
- [ ] Confirm it requires check-mode pass/fail semantics and blocks completion on unresolved CRITICAL/HIGH findings.

### 3) Precedence hierarchy consistency across instructions/agents/operator docs
- [ ] In `.github/instructions/copilot-instructions.md`, confirm precedence order is:
  1. `.github/instructions/**`
  2. `.github/agents/**`
  3. `SECURITY.md`, `docs/security.md`, `docs/features/notifications.md`
- [ ] In `.github/instructions/testing.instructions.md`, confirm governance note references the same precedence concept.
- [ ] In `.github/agents/Management.agent.md` and `.github/agents/QA_Security.agent.md`, confirm they defer to canonical `.github/instructions/**` on conflicts.

### 4) Gotify token no-exposure + query redaction rules in operator docs
- [ ] In `SECURITY.md`, confirm `Gotify Token Hygiene` section exists.
- [ ] Confirm it includes no echo/print/log/response exposure language.
- [ ] Confirm it explicitly forbids exposing tokenized query URLs (for example `...?token=...`).
- [ ] Confirm it requires query-parameter redaction in diagnostics/examples.
- [ ] In `docs/security.md`, confirm `Gotify Token Hygiene (Required)` includes no-exposure + redaction rules.
- [ ] In `docs/features/notifications.md`, confirm `Gotify Token Hygiene (Required)` includes no-exposure + redaction rules.

## Pass Criteria
- All checkboxes are completed.
- No contradictory wording found between canonical instructions, agent docs, and operator docs.
- Any mismatch is logged as a documentation follow-up issue before release sign-off.
