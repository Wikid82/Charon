# Patch Coverage Remediation Plan (Codecov) — Backend

**Created:** 2026-01-09

> Note: This plan was moved from `docs/plans/current_spec.md` to keep the current plan focused on security remediation.

## Goal

Restore **Codecov patch coverage** to green by ensuring **100% of modified lines** are executed by tests.

- **Codecov patch coverage:** 92.93866%
- **Reported missing patch lines:** ~99

Hard constraints:

- Do **not** lower Codecov thresholds.
- Fix with **targeted tests** (only add micro “test hooks” if absolutely unavoidable).

## Scope (two workstreams; both in-scope)

### Workstream A (required): Patch coverage fixes for 10 backend files

This workstream fixes Codecov **patch coverage** by adding targeted tests (and only minimal seams if unavoidable) for the following backend files:

1. backend/internal/api/handlers/plugin_handler.go
2. backend/internal/api/handlers/encryption_handler.go
3. backend/internal/api/handlers/credential_handler.go
4. backend/internal/api/handlers/settings_handler.go
5. backend/internal/api/handlers/crowdsec_handler.go
6. backend/internal/api/handlers/proxy_host_handler.go
7. backend/internal/api/handlers/security_handler.go
8. backend/internal/caddy/config.go
9. backend/internal/caddy/client.go
10. backend/internal/api/handlers/testdb.go (special: ignored in `.codecov.yml` but may still show up)

### Workstream B (required): Prevention updates (instructions + agent files)

This workstream updates the following files to ensure future production-code changes include the patch-coverage triage + tests needed to keep Codecov patch coverage green:

- .github/instructions/testing.instructions.md
- .github/instructions/copilot-instructions.md
- .github/instructions/taming-copilot.instructions.md
- .github/agents/Backend_Dev.agent.md
- .github/agents/QA_Security.agent.md
- .github/agents/Supervisor.agent.md
- .github/agents/Frontend_Dev.agent.md (only if frontend changes are involved; otherwise leave untouched)

Non-goals:

- No unrelated refactors.
- No new integration tests requiring external services.

## Missing Files Table (copy from Codecov patch report)

Codecov “Patch” view is the source of truth. Paste the **exact missing/partial line ranges** into this table.

Note: Codecov “Patch” view is the source of truth. This table is only for tracking what you copied from Codecov and what test you’ll add.

| File | Missing patch line ranges (Codecov) | Partial patch line ranges (Codecov) | Primary test strategy |
|------|-------------------------------------|-------------------------------------|-----------------------|
| backend/internal/api/handlers/plugin_handler.go | (paste) | (paste) | Gin handler tests (httptest) to hit error + warn-but-200 branches |
| backend/internal/api/handlers/encryption_handler.go | (paste) | (paste) | Handler tests for rotate/validate error + success branches |
| backend/internal/api/handlers/credential_handler.go | (paste) | (paste) | Handler tests for parse/notfound/service-error branches |
| backend/internal/api/handlers/settings_handler.go | (paste) | (paste) | Handler tests for URL test/SSRF + “reachable=false” mapping |
| backend/internal/api/handlers/crowdsec_handler.go | (paste) | (paste) | Handler tests using httptest.Server to force non-200/non-JSON branches |
| backend/internal/api/handlers/proxy_host_handler.go | (paste) | (paste) | Handler tests for JSON type coercion and invalid payload validation |
| backend/internal/api/handlers/security_handler.go | (paste) | (paste) | Handler tests for effective-status branches + validation branches |
| backend/internal/caddy/config.go | (paste) | (paste) | Unit tests for helper branches + config generation edge cases |
| backend/internal/caddy/client.go | (paste) | (paste) | Unit tests for HTTP non-200 + endpoint/parse branches |
| backend/internal/api/handlers/testdb.go | (paste) | (paste) | Prefer moving helpers into *_test.go; else fix ignore/path mismatch |

## Why patch coverage missed (common patterns)

Patch coverage misses are usually caused by newly-changed lines being in branches that existing tests don’t naturally hit:

- Error-only branches (DB errors, JSON decode failures, upstream non-200)
- “Warn but succeed” flows (HTTP 200 with a warning field)
- JSON type coercion (e.g., `map[string]any` decoding numbers vs strings)
- Env-driven branches (missing `t.Setenv` in tests)
- Codecov ignore/path normalization mismatch (repo-relative path vs module path in coverprofile)

## Handling partial (yellow) patch lines

Partial patch lines (yellow) usually mean the line executed, but **not all branches associated with that line** did.

Common causes:

- Short-circuit boolean logic (e.g., `a && b`, `a || b`) where tests only exercise one side.
- Multi-branch conditionals (`if/else if/else`, `switch`) where only one case is hit.
- Error wrapping/logging paths that run only when an upstream returns an error.

Guidance:

- Treat yellow lines like “missing branch coverage”, not “missing statement coverage”.
- Write the smallest additional test that triggers the unhit branch (invalid input, not-found, service error, upstream non-200, etc.).
- Prefer deterministic seams: `httptest.Server` for upstream failures; fake services/mocks for DB/service errors; explicit `t.Setenv` for env-driven branches.

## How to locate exact missing lines in Codecov (triage)

1. Open the PR in GitHub.
2. Open the Codecov check details (“Details” / “View report”).
3. Switch to **Patch** (not Project).
4. Filter to the 10 scoped files.
5. For each file, copy the exact missing (red) and partial (yellow) line ranges.
6. Map each range to a minimal test that executes the branch.

Local assist (for understanding branches; Codecov still authoritative):

- Run VS Code task: **Test: Backend with Coverage**.
- View coverage HTML: `go tool cover -html=backend/coverage.txt`.

## Remediation Plan (tight, test-first)

1. Extract missing patch line ranges from Codecov and fill the table above.
2. For each missing range:
   - Identify the branch (if/switch/early return) causing it.
   - Add the shortest test that executes it.
3. Re-run VS Code task **Test: Backend with Coverage**.
4. Confirm Codecov patch view is green (100% patch coverage).

## Per-File Testing Strategy (scoped)

Only add tests that hit the lines Codecov marks missing.

### 1) backend/internal/api/handlers/plugin_handler.go

- Prioritize handler tests that cover:
  - invalid `:id` parsing → 400
  - not found → 404
  - DB update failures → 500
  - loader reload/load failures (often warn-but-200) → assert response body, not just status

### 2) backend/internal/api/handlers/encryption_handler.go

- Add/extend tests to cover:
  - request bind/validation errors → 400
  - service-layer failures → 500
  - success path + any new audit/log branches

### 3) backend/internal/api/handlers/credential_handler.go

- Add/extend tests to cover:
  - invalid ID parse → 400
  - not found → 404
  - create/update invalid provider / credential validation failures (where applicable)
  - “test credentials” endpoint: service error mapping

### 4) backend/internal/api/handlers/settings_handler.go

- Add/extend tests to cover:
  - invalid URL input → 400
  - SSRF-blocked / unreachable cases that return 200 but set `reachable=false`
  - network failures and how they’re represented in the response

### 5) backend/internal/api/handlers/crowdsec_handler.go

- Use `httptest.Server` to deterministically cover:
  - upstream non-200 responses
  - upstream non-JSON responses
  - query param forwarding (capture server request URL)

### 6) backend/internal/api/handlers/proxy_host_handler.go

- Add/extend tests that submit JSON payloads exercising type coercion:
  - wrong types (string vs number) → 400
  - boundary values (negative ports, missing required fields)
  - normalization branches (if Codecov points to them)

### 7) backend/internal/api/handlers/security_handler.go

- Add/extend tests to cover:
  - effective-status branch selection (settings vs DB vs defaults)
  - validation branches (IP/CIDR parsing, enum validation, etc.)

### 8) backend/internal/caddy/config.go

- Prefer helper-level unit tests (cheaper and more targeted) to cover:
  - header normalization helpers
  - CSP/security header composition
  - CIDR parsing and bypass list validation
  - skip/empty branches (e.g., “missing provider config → skip policy”)

### 9) backend/internal/caddy/client.go

- Add/extend tests for:
  - endpoint validation failures
  - HTTP non-200 response branches
  - parse/marshal error branches (only if Codecov points to them)

### 10) backend/internal/api/handlers/testdb.go (special)

If Codecov patch view includes this file:

What’s happening:

- `.codecov.yml` currently ignores `backend/internal/api/handlers/testdb.go`, but Go coverprofiles can report paths as module-import paths (example from `backend/coverage.txt`): `github.com/Wikid82/charon/backend/internal/.../testdb.go`.
- If Codecov is matching against the coverprofile path form, the repo-relative ignore may not apply.

Make it actionable:

1. Confirm what path form the coverprofile is using:

- `grep -n "testdb.go" backend/coverage.txt`

1. If the result is a module/import-path form (example: `github.com/Wikid82/charon/backend/internal/api/handlers/testdb.go`), add one additional ignore entry to `.codecov.yml` so it matches what Codecov is actually seeing.

- Minimal, explicit (exact repo/module path): `"**/github.com/Wikid82/charon/backend/internal/api/handlers/testdb.go"`
- More resilient (still narrow): `"**/github.com/**/backend/internal/api/handlers/testdb.go"`

Note:

- Do not add `"**/backend/internal/api/handlers/testdb.go"` unless it’s missing; the repo-relative ignore is already present.

Why this is minimal:

- `.codecov.yml` already ignores the repo-relative path, but ignore matching can fail if Codecov consumes coverprofile paths that include the module/import prefix.
- Only add the extra ignore if the grep confirms the import-path form is present.

Preferred approach (in order):

1. Move test-only helpers into a `_test.go` file.

- Caution: this is only safe if **no other packages’ tests import those helpers**. Anything in `*_test.go` cannot be imported by other packages.

1. Otherwise, keep the helper in non-test code but rely on `.codecov.yml` ignores that match the coverprofile path form.

## Prevention: required instruction/agent updates (diff-style)

These are the minimal guardrails to prevent future patch-coverage regressions.

### A) .github/instructions/testing.instructions.md

```diff
@@
 ## 3. Coverage & Completion
 * **Coverage Gate:** A task is not "Complete" until a coverage report is generated.
 * **Threshold Compliance:** You must compare the final coverage percentage against the project's threshold (Default: 85% unless specified otherwise). If coverage drops, you must identify the "uncovered lines" and add targeted tests.
+* **Patch Coverage Gate (Codecov):** If production code is modified, Codecov **patch coverage must be 100%** for the modified lines. Do not relax thresholds; add targeted tests.
+* **Patch Triage Requirement:** Plans must include the exact missing/partial patch line ranges copied from Codecov’s **Patch** view.
```

### B) .github/instructions/taming-copilot.instructions.md

```diff
@@
 ## Surgical Code Modification
-    **Focus on the Core Request**: Generate code that directly addresses the user's request, without adding extra features or handling edge cases that were not mentioned.
+    **Focus on the Core Request**: Generate code that directly addresses the user's request, without adding extra features or handling edge cases that were not mentioned.
+    **Spec Hygiene**: When asked to update a plan/spec file, do not append unrelated/archived plans; keep it strictly scoped to the current task.
```

### C) .github/instructions/copilot-instructions.md

```diff
@@
 3. **Coverage Testing** (MANDATORY - Non-negotiable):
-    - **MANDATORY**: Patch coverage must cover 100% of new/modified code. This prevents CodeCov Report failing CI.
+    - **MANDATORY**: Patch coverage must cover 100% of modified lines (Codecov Patch view must be green). If patch coverage fails, add targeted tests for the missing patch line ranges.
```

### D) .github/agents/Backend_Dev.agent.md

```diff
@@
 3. **Verification (Definition of Done)**:
@@
-    - **Coverage (MANDATORY)**: Run the coverage script explicitly. This is NOT run by pre-commit automatically.
+    - **Coverage (MANDATORY)**: Run the coverage task/script explicitly and confirm Codecov patch view is green for modified lines.
```

### E) .github/agents/Frontend_Dev.agent.md (only if frontend changes are involved)

```diff
@@
 3. **Verification (Quality Gates)**:
@@
     - **Gate 3: Coverage (MANDATORY)**:
@@
         - **MANDATORY**: Patch coverage must cover 100% of new/modified code. This prevents CodeCov Report failing CI.
+        - If patch coverage fails, identify missing patch line ranges in Codecov Patch view and add targeted tests.
```

### F) .github/agents/QA_Security.agent.md

```diff
@@
-        - When creating tests, if there are folders that don't require testing make sure to update `.codecov.yml` to exclude them from coverage reports or this throws off the difference between local and CI coverage.
+        - Prefer fixing patch coverage with tests. Only adjust `.codecov.yml` ignores when code is truly non-production (e.g., test-only helpers), and document why.
```

### G) .github/agents/Supervisor.agent.md

```diff
@@
-      -   **Plan Completeness**: Does the plan cover all edge cases? Are there any missing components or unclear requirements?
+      -   **Plan Completeness**: Does the plan cover all edge cases? Are there any missing components or unclear requirements?
+      -   **Patch Coverage Completeness**: If coverage is in scope, does the plan include Codecov Patch missing/partial line ranges and the exact tests needed to execute them?
```

## Validation Checklist (patch-coverage scope)

Required vs optional alignment:

- Required for this plan to be considered complete: Workstream A + Workstream B changes landed.
- This validation checklist is intentionally focused on Workstream A (patch coverage remediation). For additional repo-wide Definition of Done items, follow `.github/instructions/copilot-instructions.md`.
- Workstream B validation is “diff applied + reviewer confirmation” (it doesn’t impact Go patch coverage directly).

Run these tasks in order:

1. **Test: Backend with Coverage**

- Pass criteria: task succeeds; `backend/coverage.txt` generated; zero failing tests.
- Outcome criteria: Codecov patch status becomes green (100% patch coverage).

1. **Lint: Pre-commit (All Files)** (optional; general DoD)

- Pass criteria: all hooks pass.
