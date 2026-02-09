
# Security Remediation Plan — DoD Failures (CodeQL + Trivy)

**Created:** 2026-01-09

This plan addresses the **HIGH/CRITICAL security findings** reported in [docs/reports/qa_report.md](docs/reports/qa_report.md).

> The prior Codecov patch-coverage plan was moved to [docs/plans/patch_coverage_spec.md](docs/plans/patch_coverage_spec.md).

## Goal

Restore DoD to ✅ PASS by eliminating **all HIGH/CRITICAL** findings from:

- CodeQL (Go + JS) results produced by **Security: CodeQL All (CI-Aligned)**
- Trivy results produced by **Security: Trivy Scan**

Hard constraints:

- Do **not** weaken gates (no suppressing findings unless a false-positive is proven and documented).
- Prefer minimal, targeted changes.
- Avoid adding new runtime dependencies.

## Scope

From the QA report:

### CodeQL Go

- Rule: `go/email-injection` (**CRITICAL**)
- Location: `backend/internal/services/mail_service.go` (reported around lines ~222, ~340, ~393)

### CodeQL JS

- Rule: `js/incomplete-hostname-regexp` (**HIGH**)
- Location: `frontend/src/pages/__tests__/ProxyHosts-extra.test.tsx` (reported around line ~252)

### Trivy

QA report note: Trivy filesystem scan may be picking up **workspace caches/artifacts** (e.g., `.cache/go/pkg/mod/...` and other generated directories) in addition to repo-tracked files, while the **image scan may already be clean**.

## Step 0 — Trivy triage (required first)

Objective: Re-run the current Trivy task and determine whether HIGH/CRITICAL findings are attributable to:

- **Repo-tracked paths** (e.g., `backend/go.mod`, `backend/go.sum`, `Dockerfile`, `frontend/`, etc.), or
- **Generated/cache paths** under the workspace (e.g., `.cache/`, `**/*.cover`, `codeql-db-*`, temporary build outputs).

Steps:

1. Run **Security: Trivy Scan**.
2. For each HIGH/CRITICAL item, record the affected file path(s) reported by Trivy.
3. Classify each finding:
   - **Repo-tracked**: path is under version control (or clearly part of the shipped build artifact, e.g., the built `app/charon` binary or image layers).
   - **Scan-scope noise**: path is a workspace cache/artifact directory not intended as deliverable input.

Decision outcomes:

- If HIGH/CRITICAL are **repo-tracked / shipped** → remediate by upgrading only the affected components to Trivy’s fixed versions (see Workstreams C/D).
- If HIGH/CRITICAL are **only cache/artifact paths** → treat as scan-scope noise and align Trivy scan scope to repo contents by excluding those directories (without disabling scanners or suppressing findings).

## Workstreams (by role)

### Workstream A — Backend (Backend_Dev): Fix `go/email-injection`

Objective: Ensure no untrusted data can inject additional headers/body content into SMTP `DATA`.

Implementation direction (minimal + CodeQL-friendly):

1. **Centralize email header construction** (avoid raw `fmt.Sprintf("%s: %s\r\n", ...)` with untrusted input).
2. **Reject** header values containing `\r` or `\n` (and other control characters if feasible).
3. Ensure email addresses are created using strict parsing/formatting (`net/mail`) and avoid concatenating raw address strings.
4. Add unit tests that attempt CRLF injection in subject/from/to and assert the send/build path rejects it.

Acceptance criteria:

- CodeQL Go scan shows **0** `go/email-injection` findings.
- Backend unit tests cover the rejection paths.

### Workstream B — Frontend (Frontend_Dev): Fix `js/incomplete-hostname-regexp`

Objective: Remove an “incomplete hostname regex” pattern flagged by CodeQL.

Preferred change:

- Replace hostname regex usage with an exact string match (or an anchored + escaped regex like `^link\.example\.com$`).

Acceptance criteria:

- CodeQL JS scan shows **0** `js/incomplete-hostname-regexp` findings.

### Workstream C — Container / embedded binaries (DevOps): Fix Trivy image finding

Objective: Ensure the built image does not ship `crowdsec`/`cscli` binaries that embed vulnerable `github.com/expr-lang/expr v1.17.2`.

Implementation direction:

1. If any changes are made to `Dockerfile` (including the CrowdSec build stage), rebuild the image (**no-cache recommended**) before validating.
2. Prefer **bumping the pinned CrowdSec version** in `Dockerfile` to a release that already depends on `expr >= 1.17.7`.
3. If no suitable CrowdSec release is available, patch the build in the CrowdSec build stage similarly to the existing Caddy stage override (force `expr@1.17.7` before building).

Acceptance criteria:

- Trivy image scan reports **0 HIGH/CRITICAL**.

### Workstream D — Go module upgrades (Backend_Dev + QA_Security): Fix Trivy repo scan findings

Objective: Eliminate Trivy filesystem-scan HIGH/CRITICAL findings without over-upgrading unrelated dependencies.

Implementation direction (conditional; driven by Step 0 triage):

1. If Trivy attributes HIGH/CRITICAL to `backend/go.mod` / `backend/go.sum` **or** to the built `app/charon` binary:

- Bump **only the specific Go modules Trivy flags** to Trivy’s fixed versions.
- Run `go mod tidy` and ensure builds/tests stay green.

1. If Trivy attributes HIGH/CRITICAL **only** to workspace caches / generated artifacts (e.g., `.cache/go/pkg/mod/...`):

- Treat as scan-scope noise and align Trivy’s filesystem scan scope to repo-tracked content by excluding those directories.
- This is **not** gate weakening: scanners stay enabled and the project must still achieve **0 HIGH/CRITICAL** in Trivy outputs.

Acceptance criteria:

- Trivy scan reports **0 HIGH/CRITICAL**.

## Validation (VS Code tasks)

Run tasks in this order (only run frontend ones if Workstream B changes anything under `frontend/`):

1. **Build: Backend**
2. **Test: Backend with Coverage**
3. **Security: CodeQL All (CI-Aligned)**
4. **Security: Trivy Scan** (explicitly verify **both** filesystem-scan and image-scan outputs are **0 HIGH/CRITICAL**)
5. **Lint: Pre-commit (All Files)**

If any changes are made to `Dockerfile` / CrowdSec build stage:

1. **Build & Run: Local Docker Image No-Cache** (recommended)
2. **Security: Trivy Scan** (re-verify image scan after rebuild)

If `frontend/` changes are made:

1. **Lint: TypeScript Check**
2. **Test: Frontend with Coverage**
3. **Lint: Frontend**

## Handoff checklist

- Attach updated `codeql-results-*.sarif` and Trivy artifacts for **both filesystem and image** outputs to the QA rerun.
- Confirm the QA report’s pass/fail criteria are satisfied (no HIGH/CRITICAL findings).
