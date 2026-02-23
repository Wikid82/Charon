## PR-1 Blocker Remediation Plan

### Introduction

This plan remediates only PR-1 failed QA/security gates identified in:

- `docs/reports/qa_report_pr1.md`
- `docs/reports/pr1_supervisor_review.md`

Scope is strictly limited to PR-1 blockers and evidence gaps. PR-2/PR-3 work is explicitly out of scope.

### Research Findings (PR-1 Blockers Only)

Confirmed PR-1 release blockers:

1. Targeted Playwright gate failing (`Authorization header required` in test bootstrap path).
2. Backend test failures (`TestSetSecureCookie_*`) preventing backend QA gate completion.
3. Docker image scan failing with one High vulnerability (`GHSA-69x3-g4r3-p962`, `github.com/slackhq/nebula`).
4. Missing/invalid local patch preflight artifacts (`test-results/local-patch-report.md` and `.json`).
5. Missing freshness-gate evidence artifact(s) required by current PR-1 spec/supervisor review.
6. Missing explicit emergency/security regression evidence and one report inconsistency in PR-1 status docs.

### Prioritized Blockers by Release Impact

| Priority | Blocker | Release Impact | Primary Owner | Supporting Owner |
|---|---|---|---|---|
| P0 | E2E auth bootstrap failure in targeted suite | Blocks proof of user-facing correctness in PR-1 path | Playwright Dev | Backend Dev |
| P0 | Backend `TestSetSecureCookie_*` failures | Blocks backend quality/security gate for PR-1 | Backend Dev | QA Security |
| P0 | High image vulnerability (`GHSA-69x3-g4r3-p962`) | Hard security release block | DevOps | Backend Dev |
| P1 | Missing local patch preflight artifacts | Blocks auditability of changed-line risk | QA Security | DevOps |
| P1 | Missing freshness-gate evidence artifact(s) | Blocks supervisor/spec compliance | QA Security | DevOps |
| P1 | Missing explicit emergency/security regression evidence + report inconsistency | Blocks supervisor approval confidence | QA Security | Playwright Dev |

### Owner Mapping (Exact Roles)

- **Backend Dev**
  - Resolve cookie behavior/test expectation mismatch for PR-1 auth/cookie logic.
  - Support Playwright bootstrap auth fix when API/auth path changes are required.
  - Support dependency remediation if backend module updates are needed.

- **DevOps**
  - Remediate image SBOM vulnerability path and rebuild/rescan image.
  - Ensure local patch/freshness artifacts are emitted, persisted, and reproducible in CI-aligned paths.

- **QA Security**
  - Own evidence completeness: patch preflight artifacts, freshness artifact(s), and explicit emergency/security regression proof.
  - Validate supervisor-facing status report accuracy and traceability.

- **Playwright Dev**
  - Fix and stabilize targeted Playwright suite bootstrap/authorization behavior.
  - Produce deterministic targeted E2E evidence for emergency/security control flows.

### Execution Order (Fix First, Verify Once)

#### Phase A — Implement all fixes (no full reruns yet)

1. **Playwright Dev + Backend Dev**: Fix auth bootstrap path causing `Authorization header required` in targeted PR-1 E2E setup.
2. **Backend Dev**: Fix `TestSetSecureCookie_*` mismatch (policy-consistent behavior for localhost/scheme/forwarded cases).
3. **DevOps + Backend Dev**: Upgrade vulnerable dependency path to a non-vulnerable version and rebuild image.
4. **QA Security + DevOps**: Correct artifact generation paths for local patch preflight and freshness snapshots.
5. **QA Security + Playwright Dev**: Ensure explicit emergency/security regression evidence is generated and report inconsistency is corrected.

#### Phase B — Single consolidated verification pass

Run once, in order, after all Phase A fixes are merged into PR-1 branch:

1. Targeted Playwright PR-1 suites (including security/emergency affected flows).
2. Backend test gate (including `TestSetSecureCookie_*`).
3. Local patch preflight artifact generation and existence checks.
4. Freshness-gate artifact generation and existence checks.
5. CodeQL check-findings (confirm target PR-1 rules remain clear).
6. Docker image security scan (confirm zero High/Critical).
7. Supervisor evidence pack update (`docs/reports/*`) and re-audit submission.

### Acceptance Criteria by Blocker

#### B1 — Targeted Playwright Gate (P0)
- Targeted PR-1 suites pass with no auth bootstrap failures.
- No `Authorization header required` error occurs in setup/fixture path.
- Emergency/security-related user flows in PR-1 scope have explicit pass evidence.

#### B2 — Backend Cookie Test Failures (P0)
- `TestSetSecureCookie_*` tests pass consistently.
- Behavior aligns with intended security policy for secure cookie handling.
- No regression introduced to authentication/session flows in PR-1 scope.

#### B3 — Docker High Vulnerability (P0)
- Image scan reports `High=0` and `Critical=0`.
- `GHSA-69x3-g4r3-p962` no longer appears in resulting image SBOM/scan output.
- Remediation is reproducible in CI-aligned scan flow.

#### B4 — Local Patch Preflight Artifacts (P1)
- `test-results/local-patch-report.md` exists after run.
- `test-results/local-patch-report.json` exists after run.
- Artifact content reflects current PR-1 diff and is not stale.

#### B5 — Freshness-Gate Evidence (P1)
- Freshness snapshot artifact(s) required by PR-1 spec are generated in `docs/reports/`.
- Artifact filenames/timestamps are referenced in PR-1 status reporting.
- Supervisor can trace freshness evidence without manual reconstruction.

#### B6 — Emergency/Security Evidence + Report Consistency (P1)
- PR-1 status docs explicitly separate implemented vs validated vs pending (no ambiguity).
- Inconsistency in backend status report regarding cookie logic is corrected.
- Emergency/security regression evidence is linked to exact test executions.

### Technical Specifications (PR-1 Remediation Only)

#### Evidence Contracts

- Patch preflight artifacts must be present at:
  - `test-results/local-patch-report.md`
  - `test-results/local-patch-report.json`
- Freshness evidence must be present in `docs/reports/` and referenced by filename in status reports.
- PR-1 status reports must include:
  - execution timestamp,
  - exact command(s),
  - pass/fail result,
  - artifact references.

#### Scope Guardrails

- Do not add new PR-2/PR-3 features.
- Do not widen test scope beyond PR-1-impacted flows except for mandatory gate runs.
- Do not refactor unrelated subsystems.

### Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|
| Fixing one gate re-breaks another (e.g., cookie policy vs E2E bootstrap) | Medium | High | Complete all code/tooling fixes first, then single consolidated verification pass | Backend Dev + Playwright Dev |
| Security fix in dependency introduces compatibility drift | Medium | High | Pin fixed version, run image scan and targeted runtime smoke in same verification pass | DevOps |
| Artifact generation succeeds in logs but files missing on disk | Medium | Medium | Add explicit post-run file existence checks and fail-fast behavior | QA Security + DevOps |
| Supervisor rejects evidence due to formatting/traceability gaps | Low | High | Standardize report sections: implemented/validated/pending + artifact links | QA Security |

### PR Slicing Strategy

- **Decision:** Single PR-1 remediation slice (`PR-1R`) only.
- **Reason:** Scope is blocker closure and evidence completion for an already-open PR-1; splitting increases coordination overhead and rerun count.
- **Slice:** `PR-1R`
  - **Scope:** Only P0/P1 blockers listed above.
  - **Dependencies:** Existing PR-1 branch state and current QA/supervisor findings.
  - **Validation Gate:** One consolidated verification pass defined in this plan.
- **Rollback/Contingency:** Revert only remediation commits within `PR-1R`; do not pull PR-2/PR-3 changes for fallback.

### Final PR-1 Re-Audit Checklist

- [ ] Targeted Playwright PR-1 suites pass (no auth bootstrap errors).
- [ ] Backend `TestSetSecureCookie_*` and related backend gates pass.
- [ ] Docker image scan shows zero High/Critical vulnerabilities.
- [ ] `test-results/local-patch-report.md` exists and is current.
- [ ] `test-results/local-patch-report.json` exists and is current.
- [ ] Freshness-gate artifact(s) exist in `docs/reports/` and are referenced.
- [ ] Emergency/security regression evidence is explicit and linked.
- [ ] PR-1 report inconsistency (cookie logic statement) is corrected.
- [ ] CodeQL target PR-1 findings remain clear (`go/log-injection`, `go/cookie-secure-not-set`, `js/regex/missing-regexp-anchor`, `js/insecure-temporary-file`).
- [ ] Supervisor re-review package is complete with commands, timestamps, and artifact links.

### Out of Scope

- Any PR-2 or PR-3 feature scope.
- New architectural changes unrelated to PR-1 blocker closure.
- Non-blocking cleanup not required for PR-1 re-audit approval.
