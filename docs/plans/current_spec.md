---
post_title: Local CI-Parity Playwright Shard Task Set Spec
author1: "Charon Team"
post_slug: local-ci-parity-playwright-task-spec
categories:
  - testing
  - ci
  - actions
tags:
  - playwright
  - vscode-tasks
  - firefox
  - ci-parity
summary: "Concise implementation plan to add five VS Code tasks for CI-like Firefox non-security shard execution: one sequential 1/4..4/4 runner and four per-shard triage tasks, all with CI-parity environment variables and explicit test paths."
post_date: "2026-02-15"
---

## 1. Introduction

Add five minimal VS Code tasks in `.vscode/tasks.json` that reproduce the CI Firefox
non-security shard execution locally for `/projects/Charon`.

Goals:

- Match CI env parity for these runs: `CI=true`,
  `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080`,
-  `CHARON_SECURITY_TESTS_ENABLED=false`, and shard-specific
  `TEST_WORKER_INDEX` values (`1..4`).
- Add one sequential task that executes Firefox shards `1/4` through `4/4` in order.
- Add four triage tasks, one per shard (`1/4`, `2/4`, `3/4`, `4/4`).
- Use deterministic Playwright output paths:
  `playwright-output/firefox-shard-1` through
  `playwright-output/firefox-shard-4`.
- Use explicit test path arguments from the active CI workflow non-security list.
- Keep task definition style aligned with existing Playwright tasks.
- Keep scope minimal to `.vscode/tasks.json` only.

## 1.1 Prerequisites

Before task execution, the E2E runtime MUST be healthy per
`.github/instructions/testing.instructions.md`:

- Use existing healthy `charon-e2e` container, OR
- Rebuild/start it with:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

## 2. Research Findings

Source of truth for CI-like command shape and env:

- `.github/workflows/e2e-tests-split.yml` (`e2e-firefox`, non-security shard job).

CI-like non-security Firefox path list (must be explicit in every shard command):

- `tests/core`
- `tests/dns-provider-crud.spec.ts`
- `tests/dns-provider-types.spec.ts`
- `tests/integration`
- `tests/manual-dns-provider.spec.ts`
- `tests/monitoring`
- `tests/settings`
- `tests/tasks`

Current task style in `.vscode/tasks.json` for Playwright tasks uses:

- `type: "shell"`
- `group: "test"`
- `problemMatcher: []`
- `presentation` with `reveal: "always"`, `panel: "dedicated"`, `close: false`

## 3. Technical Specification

### 3.1 EARS Requirements

- WHEN the developer runs the sequential VS Code task, THE SYSTEM SHALL execute
  four `npx playwright test` shard commands with `--project=firefox` in order
  (`1/4`, `2/4`, `3/4`, `4/4`) using `&&` chaining.
- WHEN the developer runs any per-shard triage task, THE SYSTEM SHALL execute
  exactly one matching shard command with `--project=firefox` and
  `--shard=N/4`.
- WHEN any task executes, THE SYSTEM SHALL set
  `CI=true`, `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080`, and
  `CHARON_SECURITY_TESTS_ENABLED=false`, and shard-appropriate
  `TEST_WORKER_INDEX` for command execution.
- WHEN any shard command executes, THE SYSTEM SHALL pass deterministic Playwright
  output path `--output=playwright-output/firefox-shard-N` for that shard.
- WHEN any task executes, THE SYSTEM SHALL pass the explicit CI-like
  non-security test paths and SHALL NOT include security-only directories.

### 3.2 Planned Task Additions

File to modify:

- `.vscode/tasks.json`

Planned labels and exact command strings:

- Label: `Test: E2E Playwright (FireFox) - CI Parity Non-Security Shards 1-4 (Sequential)`
  Command:
  `cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=1 npx playwright test --project=firefox --shard=1/4 --output=playwright-output/firefox-shard-1 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=2 npx playwright test --project=firefox --shard=2/4 --output=playwright-output/firefox-shard-2 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=3 npx playwright test --project=firefox --shard=3/4 --output=playwright-output/firefox-shard-3 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=4 npx playwright test --project=firefox --shard=4/4 --output=playwright-output/firefox-shard-4 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`

- Label: `Test: E2E Playwright (FireFox) - CI Parity Non-Security Shard 1/4`
  Command:
  `cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=1 npx playwright test --project=firefox --shard=1/4 --output=playwright-output/firefox-shard-1 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`

- Label: `Test: E2E Playwright (FireFox) - CI Parity Non-Security Shard 2/4`
  Command:
  `cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=2 npx playwright test --project=firefox --shard=2/4 --output=playwright-output/firefox-shard-2 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`

- Label: `Test: E2E Playwright (FireFox) - CI Parity Non-Security Shard 3/4`
  Command:
  `cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=3 npx playwright test --project=firefox --shard=3/4 --output=playwright-output/firefox-shard-3 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`

- Label: `Test: E2E Playwright (FireFox) - CI Parity Non-Security Shard 4/4`
  Command:
  `cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=4 npx playwright test --project=firefox --shard=4/4 --output=playwright-output/firefox-shard-4 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks`

Planned task structure:

- Follow existing Playwright task JSON keys and ordering pattern already used in
  `.vscode/tasks.json`.
- Keep change scope strictly to task objects in `.vscode/tasks.json`.

## 4. Implementation Plan

1. Add one sequential shard task object in `.vscode/tasks.json` adjacent to
  existing Playwright test tasks.
2. Add four per-shard triage task objects (`1/4`..`4/4`) in the same section.
3. Copy existing Playwright task structure (`group`, `problemMatcher`,
  `presentation`) to maintain consistency for all five tasks.
4. Insert CI-parity env vars and explicit non-security path list in each task
  `command`.
5. Keep all other files unchanged.

## 5. Validation

Validation commands (direct shell equivalents):

```bash
cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=1 npx playwright test --project=firefox --shard=1/4 --output=playwright-output/firefox-shard-1 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks

cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=2 npx playwright test --project=firefox --shard=2/4 --output=playwright-output/firefox-shard-2 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks

cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=3 npx playwright test --project=firefox --shard=3/4 --output=playwright-output/firefox-shard-3 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks

cd /projects/Charon && CI=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 CHARON_SECURITY_TESTS_ENABLED=false TEST_WORKER_INDEX=4 npx playwright test --project=firefox --shard=4/4 --output=playwright-output/firefox-shard-4 tests/core tests/dns-provider-crud.spec.ts tests/dns-provider-types.spec.ts tests/integration tests/manual-dns-provider.spec.ts tests/monitoring tests/settings tests/tasks
```

Expected behavior:

- Sequential task runs Firefox shards `1/4` to `4/4` in order, stopping on first
  failing shard due to `&&` chaining.
- Per-shard tasks run only their respective shard for triage speed.
- Only the listed non-security paths are included in the run.
- Execution targets `http://127.0.0.1:8080`.
- Security toggle is disabled for all runs
  (`CHARON_SECURITY_TESTS_ENABLED=false`).
- Worker index is explicitly set per shard (`TEST_WORKER_INDEX=1..4`).
- Playwright artifacts are written to deterministic per-shard paths
  (`playwright-output/firefox-shard-1` through
  `playwright-output/firefox-shard-4`).
- Process exits `0` when invoked shard(s) pass; non-zero on test failure.

## 6. Acceptance Criteria

- [ ] `.vscode/tasks.json` includes exactly five new CI-parity Firefox
      non-security tasks: one sequential runner and four per-shard triage tasks.
- [ ] VS Code task execution succeeds (`exit code 0`) in a healthy E2E runtime,
  with prerequisites satisfied (healthy container or `docker-rebuild-e2e`).
- [ ] Every task command includes all explicit parity fields:
  `CI=true`, `PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080`,
  `CHARON_SECURITY_TESTS_ENABLED=false`, matching `--shard=N/4`,
  matching `--output=playwright-output/firefox-shard-N`, matching
  `TEST_WORKER_INDEX=N`, and explicit non-security test paths.
- [ ] Task format and style match existing Playwright tasks in the repository.
- [ ] Plan scope remains minimal: implementation changes are limited to
  `.vscode/tasks.json`.
- [ ] Manual run (task or equivalent command) behaves as expected.
