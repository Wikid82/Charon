# QA Report: Browser Session Requests

- Branch: `fix/browser-session-requests` (5 commits on `origin/development`, up to `90e645514`)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 6
- Scope: backend (origin check middleware, route wiring, settings refresh, WebSocket origin check), Playwright
  security spec, docs (`ARCHITECTURE.md`, `docs/configuration/public-url.md`, docs-site nav). No frontend changes
  (`git diff --stat` confirms), so frontend coverage was not re-run.
- Image under test: `charon:local` from `docker-rebuild-e2e`.
- Date: 2026-11-13

## Verdict

**Approved.** Nothing blocking. QA added tests to bring patch coverage to 100%. Three low findings remain open
and are recommended as small follow-ups (see Findings). The full E2E matrix runs only in the PR's CI.

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `docker-rebuild-e2e` | Pass, container healthy |
| Playwright `authorization-rbac.spec.ts --project=security-tests` | Pass, 98/98 |
| Playwright `tests/core/authentication.spec.ts` + `tests/core/proxy-hosts.spec.ts --project=firefox` | Pass, 51/51 |
| Playwright `tests/settings/smtp-settings.spec.ts --project=firefox` (UI settings mutations) | Pass, 21/21 |
| `scripts/go-test-coverage.sh` (with `-race`) | Pass, 92.6% line coverage (gate 87%), 0 failures |
| `bash scripts/local-patch-report.sh` | Backend 83.0% before QA tests, **100%** after (88/88 lines); frontend and agent: no changed lines |
| `make lint-backend` (full golangci-lint) | Pass, 0 issues |
| `lefthook run pre-commit --all-files` | Pass, all 20 hooks (semgrep, type-check and lint included) |
| `./scripts/scan-gorm-security.sh --check` | Pass, 0 critical/high/medium (no model changes; run because a query constant moved) |
| `lefthook run codeql` (Go and JS) | 0 new. Go: 4 existing suppressed results, all in files this PR does not touch. JS: 0 |
| `bash docs-site/scripts/sync-docs.sh && zensical build --strict` | Pass, no issues |
| Trivy | Deferred to CI (fix-scoped change, no dependency or Dockerfile changes) |

## Throwaway container gate

A fresh `charon:local` container behind a second throwaway container running Caddy as an outer proxy. The proxy
rewrote `Host` to the inner address and removed `X-Forwarded-Host`, so the browser address was not visible to
Charon. Real Firefox (Playwright) drove the UI through the proxy. All throwaway containers and the network were
removed afterwards; the shared E2E and dev containers were not touched.

| Check | Result |
| --- | --- |
| No Application URL: sign in and change a setting from the UI | 200 |
| No Application URL: live log WebSocket from the UI | Refused (expected: the inner `Host` does not match the browser origin) |
| Application URL saved through System Settings (UI save request carried same-origin metadata) | 200 |
| After saving, without a restart: live log WebSocket from the UI | Opened (101) |
| After saving: session cookie + foreign origin + cross-site metadata | 403, fixed body `cross_origin_refused` |
| Session cookie + same-site sibling origin | 403 |
| Session cookie + Application URL host on another port, or with the other scheme | 403 |
| Session cookie + Application URL origin, no fetch metadata (older browser path) | 200 with the URL set, 403 without |
| WebSocket handshake with session cookie + foreign origin (with and without fetch metadata) | 403 |
| Refused responses | Never repeat request headers |

## Security review

- **Construction and refresh.** Each rebuild creates a new standard library check and stores it with one atomic
  pointer swap; a mutex serialises rebuilds, and each rebuild reads the current setting, so the last one wins.
  Requests only load the pointer: no database read per request (verified in code and by the concurrent test under
  `-race`).
- **Fail closed.** A stored value the standard library rejects leaves no extra origin trusted, both at startup and
  on refresh. QA added a startup test for this.
- **Exact match.** The trusted origin is the normalised scheme, host and port of the Application URL. Port and
  scheme mismatches were refused in the container gate.
- **Header-authenticated requests.** Requests with an `Authorization` header skip the check. Charon has no CORS
  configuration, so a page on another site cannot add that header without a preflight that is never granted.
  A credential a browser attaches by itself for an outer proxy (Basic) is not accepted as a Charon session, so it
  cannot be used to skip the check. See L1 for a hardening note.
- **Requests without browser metadata.** Current browsers always send fetch metadata, and a page cannot remove it.
  Without it, the standard library falls back to comparing `Origin` with `Host`, and refuses a mismatch unless the
  origin is the Application URL (verified in the container gate). Requests with neither header are treated as
  non-browser clients, as before.
- **WebSockets.** The handshake is checked like a state-changing request by the middleware, and the handler's own
  origin check additionally accepts the Application URL. Both refused foreign origins in the gate.
- **Route coverage.** The check applies to the whole authenticated route tree (`protected` and every subgroup),
  the session routes (logout, refresh, change password) and the import routes. Login, first-run setup, invite
  acceptance, the emergency routes, the security event intake and the backup OAuth callback (a GET) stay outside
  it. No route that legitimately receives cross-site browser posts is inside the guarded groups. Certificate
  challenges are served by Caddy, not by these routes.
- **Settings refresh.** Both write paths (single setting and bulk configuration) refresh the check after the write
  is stored; a failed refresh is logged and the saved value is kept.

## Findings

### High / Medium

None.

### Low

- **L1 (open, recommend fixing in this PR or a follow-up): align the header exemption with the auth middleware.**
  The check is skipped whenever an `Authorization` header is present, but the auth middleware only uses that header
  when it has a value, and otherwise falls back to the session cookie. Browsers cannot send such a request from
  another site (the header needs a preflight that Charon never grants), so this is defence in depth, not a reachable
  issue. Suggested approach: skip the check only when the header carries a non-empty value, using the same rule as
  the auth middleware's credential lookup, and add a unit test.
- **L2 (open, recommend follow-up): restored backups do not refresh the trusted origin.** A backup restore
  normally applies the database live, without a restart, but the origin check is only rebuilt when the Application
  URL is saved or at startup. After a live restore the previous Application URL stays trusted until one of those
  happens. `ARCHITECTURE.md` and `docs/configuration/public-url.md` say the restored value applies "after the
  restart that applies the restore", which does not match the live-restore path. Suggested approach: call the
  guard's refresh after a successful restore (the guard is already exposed as `Registered.BrowserOrigin`), or
  update both docs to "after the next restart or the next save of the Application URL".
- **L3 (open, recommend follow-up): Application URL normalisation.** The stored URL keeps the host's letter case
  and an explicit default port (`:443`, `:80`). Browsers send lowercase hosts and omit default ports, so such a
  value is not trusted (fail closed). The WebSocket check ignores case but not the default port. Suggested
  approach: lowercase the host and drop a default port when normalising the configured URL, with tests.
- **L4 (informational): docs emphasis.** With current browsers, ordinary changes made in the UI are not refused
  behind a proxy that rewrites `Host`, because the browser reports them as same-origin. What needs the Application
  URL in that setup is the live views (WebSocket) and older browsers. The troubleshooting text in
  `docs/configuration/public-url.md` leads with failing saves; suggest also naming live logs and live dashboards
  that do not connect.

## Changes made by QA (uncommitted, in the working tree)

Tests only; no application code changed.

- `backend/internal/api/middleware/browser_origin_test.go`: a guard that was never built trusts no origin; an
  invalid value at startup does not stop startup and is not trusted.
- `backend/internal/api/handlers/websocket_public_origin_test.go`: the WebSocket upgrade uses the public origin
  source (accepted, refused, and no source); all four WebSocket handlers wire the source.
- `backend/internal/api/handlers/settings_public_origin_test.go`: saving the Application URL with no guard wired
  still stores the value.
