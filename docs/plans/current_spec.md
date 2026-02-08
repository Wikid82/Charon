
# Plan: Conditional E2E Rebuild Rules + Navigation Test Continuation

## 1. Introduction

This plan updates E2E testing instructions so the Docker rebuild runs only when application code changes, explicitly skips rebuilds for test-only changes, and then continues navigation E2E testing using the existing task. The intent is to reduce unnecessary rebuild time while keeping the environment reliable and consistent.

Objectives:

- Define clear, repeatable criteria for when an E2E container rebuild is required vs optional.
- Update instruction and agent documents to use the same conditional rebuild guidance.
- Preserve current E2E execution behavior and task surface, then proceed with navigation testing.

## 2. Research Findings

- The current testing protocol mandates rebuilding the E2E container before Playwright runs in [testing.instructions.md](.github/instructions/testing.instructions.md).
- The Management and Playwright agent definitions require rebuilding the E2E container before each test run in [Management.agent.md](.github/agents/Management.agent.md) and [Playwright_Dev.agent.md](.github/agents/Playwright_Dev.agent.md).
- QA Security also mandates rebuilds on every code change in [QA_Security.agent.md](.github/agents/QA_Security.agent.md).
- The main E2E skill doc encourages rebuilds before testing in [test-e2e-playwright.SKILL.md](.github/skills/test-e2e-playwright.SKILL.md).
- The rebuild skill itself is stable and already describes when it should be used in [docker-rebuild-e2e.SKILL.md](.github/skills/docker-rebuild-e2e.SKILL.md).
- Navigation test tasks already exist in [tasks.json](.vscode/tasks.json), including “Test: E2E Playwright (FireFox) - Core: Navigation”.
- CI E2E jobs rebuild via Docker image creation in [e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml); no CI changes are required for this instruction-only update.

## 3. Technical Specifications

### 3.1 Rebuild Decision Rules

Define explicit change categories to decide when to rebuild:

- **Rebuild Required (application/runtime changes)**
	- Application code or dependencies: backend/**, frontend/**, backend/go.mod, backend/go.sum, package.json, package-lock.json.
	- Container build/runtime configuration: Dockerfile, .docker/**, .docker/compose/docker-compose.playwright-*.yml, .docker/docker-entrypoint.sh.
	- Runtime behavior changes that affect container startup (e.g., config files baked into the image).

- **Rebuild Optional (test-only changes)**
	- Playwright tests and fixtures: tests/**.
	- Playwright config and test runners: playwright.config.js, playwright.caddy-debug.config.js.
	- Documentation or planning files: docs/**, requirements.md, design.md, tasks.md.
	- CI/workflow changes that do not affect runtime images: .github/workflows/**.

Decision guidance:

- If only test files or documentation change, reuse the existing E2E container if it is already healthy.
- If the container is not running, start it with docker-rebuild-e2e even for test-only changes.
- If there is uncertainty about whether a change affects the runtime image, default to rebuilding.

### 3.2 Instruction Targets and Proposed Wording

Update the following instruction and agent files to align with the conditional rebuild policy:

- [testing.instructions.md](.github/instructions/testing.instructions.md)
	- Replace the current “Always rebuild the E2E container before running Playwright tests” statement with:
		- “Rebuild the E2E container when application or Docker build inputs change (backend, frontend, dependencies, Dockerfile, .docker/compose). If changes are test-only, reuse the existing container when it is already healthy; rebuild only if the container is not running or state is suspect.”
	- Add a short file-scope checklist defining “rebuild required” vs “test-only.”

- [Management.agent.md](.github/agents/Management.agent.md)
	- Update the “PREREQUISITE: Rebuild E2E container before each test run” bullet to:
		- “PREREQUISITE: Rebuild the E2E container only when application or Docker build inputs change; skip rebuild for test-only changes if the container is already healthy.”

- [Playwright_Dev.agent.md](.github/agents/Playwright_Dev.agent.md)
	- Update “ALWAYS rebuild the E2E container before running tests” to:
		- “Rebuild the E2E container when application or Docker build inputs change. For test-only changes, reuse the running container if healthy; rebuild only when the container is not running or state is suspect.”

- [QA_Security.agent.md](.github/agents/QA_Security.agent.md)
	- Update workflow step 1 to:
		- “Rebuild the E2E image and container when application or Docker build inputs change. Skip rebuild for test-only changes if the container is already healthy.”

- [test-e2e-playwright.SKILL.md](.github/skills/test-e2e-playwright.SKILL.md)
	- Adjust “Quick Start” language to:
		- “Run docker-rebuild-e2e when application or Docker build inputs change. If only tests changed and the container is already healthy, skip rebuild and run the tests.”

- Optional alignment (if desired for consistency):
	- [test-e2e-playwright-debug.SKILL.md](.github/skills/test-e2e-playwright-debug.SKILL.md)
	- [test-e2e-playwright-coverage.SKILL.md](.github/skills/test-e2e-playwright-coverage.SKILL.md)
	- Update prerequisite language in the same conditional format when referencing docker-rebuild-e2e.

### 3.3 Data Flow and Component Impact

- No API, database, or runtime component changes are introduced.
- The change is documentation-only: it modifies decision guidance for when to rebuild the E2E container.
- The E2E execution flow remains: optionally rebuild → run navigation test task → review Playwright report.

### 3.4 Error Handling and Edge Cases

- If the container is running but tests fail due to stale state, rebuild with docker-rebuild-e2e and re-run the navigation test.
- If only tests changed but the container is stopped, rebuild to create a known-good environment.
- If Dockerfile or .docker/compose changes occurred, rebuild is required even if tests are the only edited files in the last commit.

## 4. Implementation Plan

### Phase 1: Instruction Updates (Documentation-only)

- Update conditional rebuild guidance in the instruction files listed in section 3.2.
- Ensure the rebuild decision criteria are consistent and use the same file-scope examples across documents.

### Phase 2: Supporting Artifacts

- Update requirements.md with EARS requirements for conditional rebuild behavior.
- Update design.md to document the decision rules and file-scope criteria.
- Update tasks.md with a checklist that explicitly separates rebuild-required vs test-only scenarios.

### Phase 3: Navigation Test Continuation

- Determine change scope:
	- If application/runtime files changed, run the Docker rebuild step first.
	- If only tests or docs changed and the E2E container is already healthy, skip rebuild.
- Run the existing navigation task: “Test: E2E Playwright (FireFox) - Core: Navigation” from [tasks.json](.vscode/tasks.json).
- If the navigation test fails due to environment issues, rebuild and re-run.

## 5. Acceptance Criteria

- Instruction and agent files reflect the same conditional rebuild policy.
- Rebuild-required vs test-only criteria are explicitly defined with file path examples.
- Navigation tests can be run without a rebuild when only tests change and the container is healthy.
- The navigation test task remains unchanged and is used for validation.
- requirements.md, design.md, and tasks.md are updated to reflect the new rebuild rules.

## 6. Testing Steps

- If application/runtime files changed, run the E2E rebuild using docker-rebuild-e2e before testing.
- If only tests changed and the container is healthy, skip rebuild.
- Run the navigation test task: “Test: E2E Playwright (FireFox) - Core: Navigation”.
- Review Playwright report and logs if failures occur; rebuild and re-run if the failure is environment-related.

## 7. Config Hygiene Review (Requested Files)

- .gitignore: No change required for this instruction update.
- codecov.yml: No change required; E2E outputs are already ignored.
- .dockerignore: No change required; tests/ and Playwright artifacts remain excluded from image build context.
- Dockerfile: No change required.

## 8. Risks and Mitigations

- Risk: Tests may run against stale containers when changes are misclassified as test-only. Mitigation: Provide explicit file-scope criteria and default to rebuild when unsure.
- Risk: Contributors interpret “test-only” too narrowly. Mitigation: include dependency files and Docker build inputs in rebuild-required list.

## 9. Confidence Score

Confidence: 88 percent

Rationale: This is a documentation-only change with no runtime or CI impact, but it relies on consistent interpretation of file-scope criteria.
