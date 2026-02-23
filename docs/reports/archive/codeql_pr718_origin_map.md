# PR 718 CodeQL Origin Map

Date: 2026-02-18
Source PR: https://github.com/Wikid82/Charon/pull/718

## Scope

- Mapped all **high severity** CodeQL alerts from PR 718 (GitHub API `code-scanning/alerts?pr=718&state=open`).
- For each alert, traced `path:line` to introducing commit via `git blame`.
- Classified each introducing commit as:
  - `on_main=yes`: already reachable from `origin/main`
  - `on_main=no`: not reachable from `origin/main` (arrives via promotion PR range)

## Results

- High severity alerts mapped: **67**
- `on_main=yes`: **0**
- `on_main=no`: **67**

### Rule distribution (high only)

- `go/log-injection`: 58
- `js/regex/missing-regexp-anchor`: 6
- `js/insecure-temporary-file`: 3

### Dominant introducing commits

- `3169b051561c1a380a09ba086c81d48b4d0bf0ba` → 61 alerts
  - Subject: `fix: skip incomplete system log viewer tests`
- `a14f6ee41f4ba9718909471a99e7ea8876590954` → 3 alerts
  - Subject: `fix: add refresh token endpoint to authentication routes`
- `d0334ddd40a54262689283689bff19560458e358` → 1 alert
  - Subject: `fix: enhance backup service to support restoration from WAL files and add corresponding tests`
- `a44530a682de5ace9e1f29b9b3b4fdf296f1bed2` → 1 alert
  - Subject: `fix: change Caddy config reload from async to sync for deterministic applied state`
- `5a46ef4219d0bab6f7f951c6d690d3ad22c700c2` → 1 alert
  - Subject: `fix: include invite URL in user invitation response and update related tests`

## Representative mapped alerts

- `1119` `js/regex/missing-regexp-anchor` at `tests/tasks/import-caddyfile.spec.ts:324`
  - commit: `3169b051561c1a380a09ba086c81d48b4d0bf0ba` (`on_main=no`)
- `1112` `js/insecure-temporary-file` at `tests/fixtures/auth-fixtures.ts:181`
  - commit: `a14f6ee41f4ba9718909471a99e7ea8876590954` (`on_main=no`)
- `1109` `go/log-injection` at `backend/internal/services/uptime_service.go:1090`
  - commit: `3169b051561c1a380a09ba086c81d48b4d0bf0ba` (`on_main=no`)
- `1064` `go/log-injection` at `backend/internal/api/handlers/user_handler.go:545`
  - commit: `5a46ef4219d0bab6f7f951c6d690d3ad22c700c2` (`on_main=no`)

## Interpretation

- For high alerts, this mapping indicates they are tied to commits not yet on `main` and now being introduced together via the very large promotion range.
- This does **not** imply all were authored in PR 718; it means PR 718 is the first main-targeting integration point where these commits are entering `main` and being classified in that context.

## Important note on “CodeQL comments only on PRs to main?”

- The workflow in this branch (`.github/workflows/codeql.yml`) is configured for `pull_request` on `main`, `nightly`, and `development`.
- CodeQL itself does not rely on PR comments for enforcement; annotations/check results depend on workflow trigger execution and default-branch security baseline context.
