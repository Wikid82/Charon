---
post_title: "Current Spec: Governance Update — DoD GORM Scan + Gotify Token Hygiene"
categories:
  - security
  - governance
tags:
  - dod
  - gorm
  - gotify
  - documentation
  - planning
summary: "Governance-only plan to explicitly enforce conditional GORM scanner execution in DoD and add policy/operational controls that prevent Gotify token exposure while preserving token validation workflows."
post_date: 2026-02-20
---

## Active Plan: Governance Update — DoD GORM Scan + Gotify Token Hygiene

Date: 2026-02-20
Status: Active and authoritative
Scope Type: Documentation/Governance only (no product feature refactor)

### 1) Introduction

This plan defines a documentation-only governance update for Charon. It focuses on two outcomes:

1. Confirm and enforce that local Definition of Done (DoD) explicitly requires GORM security scan execution when backend model/database-related changes occur.
2. Establish explicit policy and operational guidance to prevent Gotify token exposure while preserving secure token validation.

This plan intentionally excludes application feature refactors, endpoint behavior changes, and data model changes.

### 2) Scope and Assumptions

#### In Scope

- Update governance markdown under `.github/instructions/**`.
- Update operating-agent markdown under `.github/agents/**` where DoD/security verification behavior is defined.
- Update security governance docs outside `.github` only where required to keep policy alignment and operator guidance clear.
- Provide explicit wording for:
  - Conditional GORM scan gate in DoD.
  - Gotify token secrecy controls (no echo, no logs, no API response exposure).
  - Practical hardening recommendations.

#### Out of Scope

- Backend/frontend feature implementation.
- API contract/code changes.
- DB schema/migration changes.
- Test-suite refactors beyond documentation references.

#### Assumptions

- Existing GORM scanner exists and is runnable (`Lint: GORM Security Scan`, `pre-commit ... gorm-security-scan`, `scripts/scan-gorm-security.sh`).
- DoD governance is currently distributed across instruction and agent files and must remain consistent.
- Gotify continues to be supported in notification docs and needs explicit secret-handling rules documented.

### 3) Research Findings

#### Current DoD/GORM Position

- `testing.instructions.md` already includes a GORM Security Validation section and states scanner pass as DoD requirement.
- `copilot-instructions.md` DoD section is comprehensive but does not currently elevate the GORM scan as an explicit conditional DoD step tied to backend model/database touchpoints.
- Agent-level DoD ownership exists in `Management.agent.md`; however, explicit conditional GORM gate language is incomplete and should be made unambiguous.

#### Current Gotify/Token Guidance Position

- Notification/security documentation includes Gotify payload examples but does not centrally enforce a strict token non-exposure policy with explicit “never echo/log/return” phrasing.
- Security instruction files contain generic secret guidance but lack Gotify-specific operational controls and validation-safe masking patterns.

#### Governance Surfaces Identified for Update

##### Primary (`.github/instructions/**`)

1. `.github/instructions/copilot-instructions.md`
2. `.github/instructions/testing.instructions.md`
3. `.github/instructions/security-and-owasp.instructions.md`

##### Primary (`.github/agents/**`)

4. `.github/agents/Management.agent.md`
5. `.github/agents/Backend_Dev.agent.md`
6. `.github/agents/QA_Security.agent.md`

##### Additional governance docs required for policy coherence

7. `SECURITY.md`
8. `docs/security.md`
9. `docs/features/notifications.md`

### 4) Requirements (EARS)

- R1: WHEN a change touches `backend/internal/models/**` or backend database interaction paths, THE SYSTEM SHALL require execution of the GORM security scanner as a local DoD gate before task completion.
- R2: IF the GORM scanner reports CRITICAL or HIGH findings, THEN THE SYSTEM SHALL require remediation before DoD can be considered complete.
- R3: WHEN documenting or operating Gotify provider configuration, THE SYSTEM SHALL prohibit token echoing, token logging, and token inclusion in API responses.
- R4: IF token validation is needed for connectivity or health checks, THEN THE SYSTEM SHALL validate tokens without revealing raw token values and SHALL use masking/redaction in all human-visible outputs.
- R5: WHILE governance documentation is updated, THE SYSTEM SHALL keep language consistent across instruction files, agent files, and operator-facing security docs.

### 5) Technical Specification (Documentation Contract)

This is a policy contract update, not an API/DB contract change.

#### 5.1 Policy Contract: Conditional GORM Scan Gate

Define a single, repeated clause across governance docs:

- Trigger: backend model/database-related changes.
- Required actions:
  - Run scanner via one approved command path.
  - Treat scanner as DoD process-blocking gate even when automation stage is manual (soft launch).
  - Use `--check` mode for gate decisions and pass/fail outcomes.
  - Block completion on unresolved CRITICAL/HIGH findings.

Approved command references remain:

- `Lint: GORM Security Scan` (must execute scanner in check/gating mode)
- `pre-commit run --hook-stage manual gorm-security-scan --all-files` (gating decision maps to check semantics)
- `./scripts/scan-gorm-security.sh --check` (canonical gate command)

#### 5.1.1 Conditional Trigger Matrix (Include/Exclude)

| Change Type | Trigger GORM Gate? | Rule |
| --- | --- | --- |
| `backend/internal/models/**` | Yes | Include |
| Backend services/repositories with GORM query logic | Yes | Include |
| DB migrations/seeding that change persistence behavior | Yes | Include |
| Docs-only changes (`**/*.md`, governance docs) | No | Exclude |
| Frontend-only changes (`frontend/**`) | No | Exclude |

Gate decision rule:

- IF any Include row matches, THEN scanner execution in check mode is mandatory DoD gate.
- IF only Exclude rows match, THEN GORM gate is not required for that change set.

#### 5.2 Policy Contract: Gotify Token Non-Exposure

Define explicit non-negotiables:

- Never print token to terminal output.
- Never write token to application logs, debug logs, test output, screenshots, or reports.
- Never include token in API response bodies, errors, or serialized DTOs.
- Never expose tokenized endpoint query strings (for example `...?token=...`) in docs, diagnostics, examples, or logs.
- Always redact URL query parameters in diagnostics/examples/log output before display or storage.
- Validation operations must be non-revealing:
  - Use token length/prefix-independent masking in UX and diagnostics.
  - Store and process token as secret only.
  - Return generic validation outcomes (`valid`/`invalid` + reason category without token value).

#### 5.3 Cross-Document Precedence and Canonical Language

To prevent policy drift, define precedence and reconciliation behavior:

1. Canonical governance source: `.github/instructions/testing.instructions.md` and `.github/instructions/security-and-owasp.instructions.md`
2. Agent execution source: `.github/agents/**`
3. Operator-facing/security guidance: `SECURITY.md`, `docs/security.md`, `docs/features/notifications.md`

Reconciliation behavior:

- IF lower-precedence text conflicts with higher-precedence canonical language, THEN lower-precedence text must be updated to match canonical wording in the same PR.
- Canonical terms must be reused verbatim for gate semantics:
  - “DoD process-blocking even in manual soft launch”
  - “check mode (`--check`) decides pass/fail gate”
  - “no tokenized URL/query exposure; redact query parameters in all diagnostics/examples/logs”

#### 5.4 Operational Hardening Recommendations

Document practical recommendations:

- Use secret storage source (env var or secret manager), not plaintext docs/snippets.
- Apply structured log redaction for keys containing `token`, `apikey`, `authorization`.
- Use one-way visibility in UI forms (write-only fields, masked placeholders).
- Rotate Gotify tokens on suspected exposure.
- Validate over HTTPS only and avoid proxy/debug middleware that logs headers/bodies with secrets.

### 6) Exact Files and Sections to Change + Precise Wording Recommendations

#### 6.1 `.github/instructions/copilot-instructions.md`

Section: `## ✅ Task Completion Protocol (Definition of Done)`

Recommended insertion (new dedicated step after security scans or adjacent to testing gates):

- **GORM Security Scan (Conditional, BLOCKING)**:
  - Trigger this step when changes include backend models or database interaction logic (for example: `backend/internal/models/**`, GORM query/service layers, migrations).
  - Run one of:
    - `Lint: GORM Security Scan`
    - `pre-commit run --hook-stage manual gorm-security-scan --all-files`
    - `./scripts/scan-gorm-security.sh --check`
  - DoD is blocked until scanner reports zero CRITICAL/HIGH findings.

#### 6.2 `.github/instructions/testing.instructions.md`

Section: `## 4. GORM Security Validation (Manual Stage)`

Recommended wording tighten (replace opening requirement sentence):

- **Requirement:** For any change that touches backend models or database-related logic, the GORM Security Scanner is a mandatory local DoD gate and must pass with zero CRITICAL/HIGH findings.

Policy-vs-automation reconciliation clause (explicitly add under manual stage text):

- “Manual stage” describes execution mechanism only; policy enforcement remains process-blocking for DoD. Gate decisions must use check semantics (`./scripts/scan-gorm-security.sh --check` or equivalent task wiring).

Recommended addition under “When to Run”:

- **Mandatory Trigger Paths:**
  - `backend/internal/models/**`
  - Database interaction/services using GORM queries
  - Migration/seeding logic affecting model persistence behavior
- **Explicit Exclusions:**
  - Docs-only changes
  - Frontend-only changes

#### 6.3 `.github/instructions/security-and-owasp.instructions.md`

Section: add new subsection under “General Guidelines”:

- **Gotify Token Protection (Explicit):**
  - Treat Gotify application tokens as secrets.
  - Do not echo tokens in CLI output.
  - Do not log tokens (application logs, debug logs, test logs).
  - Do not return tokens in API responses or error payloads.
  - For token validation, return only non-sensitive status and redact/mask any token-derived diagnostics.

#### 6.4 `.github/agents/Management.agent.md`

Section: `## DEFINITION OF DONE ##`

Recommended insertion as explicit checklist item:

- **GORM Security Scan (Conditional Gate):**
  - If implementation touched backend models or database-interaction paths, confirm `QA_Security` (or responsible subagent) ran the GORM scanner and resolved all CRITICAL/HIGH findings before accepting completion.

#### 6.5 `.github/agents/Backend_Dev.agent.md`

Section: `3. **Verification (Definition of Done)**`

Recommended insertion:

- **Conditional GORM Gate:** If task changes include model/database-related files or GORM query logic, run `pre-commit run --hook-stage manual gorm-security-scan --all-files` (or `./scripts/scan-gorm-security.sh --check`) and treat CRITICAL/HIGH findings as blocking.

#### 6.6 `.github/agents/QA_Security.agent.md`

Sections: `<workflow>` Step 4 (Security Scanning) and reporting expectations

Recommended insertion:

- Add explicit conditional check:
  - When backend model/database-related changes are in scope, run GORM scanner and report pass/fail as DoD gate.
- Add Gotify token review check:
  - Verify no token appears in logs, test artifacts, screenshots, API examples, report output, or tokenized URL query strings.
  - Verify URL query parameters are redacted in diagnostics/examples/log artifacts.

#### 6.7 `SECURITY.md`

Section candidate: under “Security Best Practices” or “Notification/Webhook Security” area

Recommended addition:

- **Gotify Token Hygiene:**
  - Gotify tokens are secrets and must never be echoed, logged, or returned by API responses.
  - Validation endpoints and troubleshooting guidance must use masked outputs only.
  - Rotate token immediately if exposure is suspected.

#### 6.8 `docs/security.md`

Section candidate: existing “Security Notifications” / “Gotify JSON Payload Example” area

Recommended addition:

- Add operational note block:
  - “Use write-only token input. Do not paste raw tokens into logs, screenshots, tickets, or chat.”
  - “Validation should confirm connectivity without exposing token value.”

#### 6.9 `docs/features/notifications.md`

Section candidate: “Gotify Webhooks” / setup + troubleshooting sections

Recommended addition:

- Add concise “Token Safety” subsection:
  - Never expose token in templates, payload examples, or troubleshooting snippets.
  - Use masked token representation in examples (e.g., `gtf_********`).
  - Prefer token rotation and re-test when compromise is suspected.

### 7) Implementation Plan (Phased)

#### Phase 1: Governance Baseline Diff (Documentation Research Validation)

- Validate each target file and section exists.
- Map exact insertion points.
- Confirm no overlap with archived docs to avoid scope creep.

Complexity: Low

#### Phase 2: DoD Enforcement Text Updates

- Apply conditional GORM scan language to instruction + agent DoD surfaces.
- Ensure wording consistency between `.github/instructions/**` and `.github/agents/**`.

Complexity: Medium

#### Phase 3: Gotify Token Exposure Policy Updates

- Add explicit no echo/no log/no API exposure policy text.
- Add practical validation-safe and hardening guidance.

Complexity: Medium

#### Phase 4: Governance Consistency Pass

- Verify terminology consistency:
  - “conditional gate”, “CRITICAL/HIGH blocking”, “masked/redacted output”.
- Check that all cross-doc references remain valid.

Complexity: Low

#### Phase 5: Validation and Handoff

- Perform markdown lint/readability pass.
- Confirm plan scope is governance-only (no implementation drift).
- Prepare for Supervisor review.

Complexity: Low

### 8) Acceptance Criteria

1. `docs/plans/current_spec.md` is fully replaced with this governance-only plan.
2. Plan lists exact target files under `.github/instructions/**` and `.github/agents/**` plus required additional governance docs.
3. Plan includes explicit, precise wording recommendations per file/section.
4. Plan defines conditional DoD enforcement for GORM scan tied to backend model/database-related changes.
5. Plan defines explicit Gotify token non-exposure rules (no echo, no logging, no API response exposure) and validation-safe behavior.
6. Plan includes practical hardening recommendations.
7. Plan includes PR slicing strategy with explicit single-PR decision rationale.
8. Plan excludes code-feature refactors and remains tightly scoped to governance/documentation updates.
9. Plan explicitly defines policy-vs-automation behavior: manual scanner stage does not weaken DoD gate enforcement; check mode semantics are mandatory for gate decisions.
10. Plan includes include/exclude matrix for conditional GORM scanner triggering.
11. Plan includes explicit reconciliation edits for `testing.instructions` manual-soft-launch wording and `QA_Security.agent.md` Gotify token artifact checks.

### 9) PR Slicing Strategy

Decision: **Single PR**

#### Rationale

- Scope is text-only governance alignment across related markdown files.
- Changes are logically cohesive (DoD gate clarity + token secrecy policy).
- Splitting would increase review overhead and risk inconsistent policy wording.

#### Single-PR Scope

- All files in Section 6.
- No code changes.
- One validation pass for consistency.

#### Validation Gates (within the PR)

- Markdown renders cleanly.
- Wording consistency across instruction/agent/operator docs.
- No contradiction between DoD policies.

#### Rollback/Contingency

- If wording introduces confusion, rollback via single revert commit of doc-only changes.
- If partial disagreement occurs, retain DoD gate updates and isolate Gotify wording revisions in follow-up doc PR.

### 10) Risks and Mitigations

- **Risk 1: Policy drift between instruction and agent docs**
  Mitigation: Use shared wording patterns and perform explicit consistency pass in Phase 4.

- **Risk 2: Overly broad wording accidentally implies code changes**
  Mitigation: Keep language explicitly “documentation governance only” and avoid implementation directives beyond policy.

- **Risk 3: Sensitive token examples accidentally remain in docs**
  Mitigation: Add explicit masked-example rule and require post-edit grep check for token-like literals.

- **Risk 4: Reviewer ambiguity on conditional trigger**
  Mitigation: Define concrete trigger paths (`backend/internal/models/**`, GORM DB logic, migrations).

### 11) Supervisor Handoff Notes

This plan is ready for Supervisor review as a governance-only documentation update. Review focus should verify:

- Clarity of conditional DoD gate for GORM scanner.
- Sufficiency and practicality of Gotify token non-exposure policy, including token-in-URL query redaction requirements.
- Cross-file wording consistency and absence of scope creep.
