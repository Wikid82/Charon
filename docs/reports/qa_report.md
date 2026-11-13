# QA Report: Management Permissions for Standard Users

- Branch: `fix/management-permissions` (11 commits on `origin/development`, up to `74bfc4fda`)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 7
- Scope: backend (host write checks, role-aware response helpers, monitor routes, container list, forward target
  validation, administration interface hardening), frontend (proxy host form, redirection host form, Uptime,
  Notifications, DNS providers, locales), container image and entrypoint, integration scripts, Playwright specs,
  docs.
- Image under test: `charon:local` from `docker-rebuild-e2e`, rebuilt for this round.
- Date: 2026-11-13
- This is the second QA round. The first round found one high issue (H1); this round checks the fix and the other
  changes made since.

## Verdict

**Approved.** Nothing blocking is left. H1 from the first round is fixed: the proxy's administration interface is
no longer reachable over the network, and the earlier reproduction now fails. QA found one gap in that fix (the
socket directory was not private) and fixed it with a one-line change in the entrypoint and the Dockerfile (M1).
QA also added tests to close the coverage gaps, and fixed one stale script comment. Five low findings remain open;
each is suggested as a separate follow-up (see Findings).

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `docker-rebuild-e2e` | Pass, container healthy (rebuilt for this round, and again after the QA Dockerfile change) |
| Playwright `authorization-rbac.spec.ts --project=security-tests` | Pass, 116/116 (on the image rebuilt with the QA Dockerfile change) |
| Playwright `tests/security/self-proxy-requests.spec.ts --project=security-tests` (Firefox locally) | Pass, 8/8. The spec reads the per-process hop value through the new admin path and requires a non-empty 32+ character value, so a failed read fails the test instead of weakening it |
| Firefox UI pass on the throwaway container (standard user) | Pass (see below) |
| `scripts/go-test-coverage.sh` (with `-race`, 60m timeout) | Pass, 93.7% statements / 92.7% lines (gate 87%), 0 failures, no races (re-run with the QA tests) |
| `scripts/frontend-test-coverage.sh` | Pass, 355 files / 4526 tests, 91.8% statements |
| `bash scripts/local-patch-report.sh` | Before QA tests: backend 392/439 (89.3%), frontend 67/70. After: backend 434/439 (98.9%), frontend 69/70 (the two Notifications lines confirmed with a targeted coverage run). Remaining lines are listed under "Patch coverage" |
| `make lint-backend` (full golangci-lint) | Pass, 0 issues |
| `golangci-lint-fast` (backend and agent) after QA tests | Pass, 0 issues |
| `npm run lint` (max warnings 0), `npm run type-check`, `tsc6 --noEmit` (TypeScript 6.0.3) | Pass |
| `lefthook run pre-commit --all-files` | Pass, all 20 hooks (semgrep, type-check and lint included) |
| `lefthook run codeql` (Go and JS) | Pass. Go: 0 blocking (4 existing suppressed results, none in files this PR changes). JS: 0 |
| `./scripts/scan-gorm-security.sh --check` | Pass, 0 critical/high/medium |
| `shellcheck` on the entrypoint and every changed script | No new findings compared with `origin/development` (the hook gate is `--severity=error`; existing lower-severity notes are unchanged) |
| bats | No bats suite covers the changed scripts |
| actionlint | Not needed: no workflow changes |
| `docker compose -f .docker/compose/docker-compose.playwright-local.yml config` (with the `.env` values exported, as the file requires) | Valid. Publishes `127.0.0.1:2020` for the emergency server; no admin port anywhere |
| docs-site `sync-docs.sh` + `zensical build --strict` | Pass, no issues |
| markdownlint on changed Markdown | Pass (existing issues in a file outside the docs tree are untouched) |
| Trivy | Deferred to CI |

## Integration scripts (run in their real flow)

The shared E2E container was stopped while scripts that publish its ports ran, then restarted. The dev container
was not touched. Coraza ran with its port variables moved off ports 80 and 443.

| Script | Result |
| --- | --- |
| `rate_limit_integration.sh` | Pass; its readiness step reads the admin socket ("Caddy admin API is ready") |
| `waf_integration.sh` | Pass |
| `cerberus_integration.sh` | Pass |
| `coraza_integration.sh` | Pass |
| `security_headers_integration.sh` | Pass |
| `debug_rate_limit.sh` | Runs; it prints the full route list read through the admin socket |
| `integration-test.sh` | The readiness command works against a running container, but the script stops earlier at setup (L4) |
| `verify_crowdsec_app_config.sh` | The printed commands use the new path and work when run by hand |

## Throwaway container gate

A fresh `charon:local` container on random loopback ports with its own volume, an administrator and a standard
user, checked with real HTTP calls and a real Firefox session. All throwaway containers, volumes and networks
were removed afterwards.

| Check | Result |
| --- | --- |
| Start-up logs | Clean: no refusal, no address-in-use. Caddy logs one generic line about host checking on the socket (L1) |
| Admin socket | `srw------- charon`. Directory: `drwxr-sr-x` on the image as built (M1); `drwx------ charon` with the QA fix |
| `caddy-admin get /config/` | JSON with `admin.listen` = `unix//app/data/run/caddy-admin.sock\|0600` |
| TCP admin listener | None: no port 2019 in the listening sockets, and a request to `127.0.0.1:2019` inside the container fails |
| Config reload | Creating and editing a proxy host is served at once; the old domain is no longer routed |
| `docker restart` (same volume) | Clean start, readiness passes |
| Hard kill, then start on the same volume | The stale socket left on the volume is cleared and Caddy starts; no address-in-use |
| Legacy setting `CHARON_CADDY_ADMIN_API=http://localhost:2019` | Starts with a warning to remove the setting; still no TCP listener |
| Setting that names another host or address | Start-up stops with a clear error |
| First-round H1 reproduction (as a standard user) | Hosts that name local-only targets on the old admin port can still be created, but every request to them gets 502 and the admin interface does not answer on any path |
| Forward hosts that look like socket paths, or contain `/`, `\`, `@`, `?`, `#` or spaces (hosts and locations) | 400 for every role |
| Ordinary forward hosts (`10.0.0.5`, service names with `_`) | Accepted |
| UI as a standard user: Notifications, DNS providers | No add, edit, delete, test or template controls; no secret values |
| UI as a standard user: proxy host form | No uptime option, no advanced editor; editing and saving works |

## Security review

- **Administration interface.** The proxy's admin API is now a unix socket inside the container. No TCP listener
  is opened, even when an older loopback URL is configured, and other addresses are refused at start-up. The
  socket is mode 0600 and owned by the charon user, so only Charon and Caddy can connect. With the QA fix (M1)
  the directory is 0700 as documented. No proxy target, user setting or published port can reach it.
- **Forward target validation.** Forward hosts are now limited to a host, an optional port and an optional
  `http://` or `https://` prefix, for proxy hosts and their locations, for every role. Values that a later step
  would shorten into a different target are refused instead. An existing host could not have used the refused
  characters, because Caddy would not load such a target, so the stricter check does not break working hosts.
- **Write checks and read redaction** (unchanged since the first round). Every user-facing write path goes through
  the service-level admin-only field check. All responses that carry the sensitive fields are redacted for
  non-admin callers. Imports are administrator-only. Roles fail closed.
- **Other users of the old port.** The image no longer exposes it. Health checks use the application port.
  Workflows, the agent and the docs-site do not reference the admin API. The integration scripts, compose
  comments, Playwright helpers and user docs use the new command.

## Findings

### High

None open. **H1 (first round): fixed** by moving the administration interface off the network (verified above).

### Medium

- **M1 (fixed by QA): the admin socket directory was not private.** The image and the entrypoint both create the
  socket directory with the default mode before the binary's 0700 setting could apply, so the directory was
  world-readable on every install. The socket's own 0600 mode still limited who could connect, but the documented
  directory mode was not in effect. Fix: `chmod 700` in the entrypoint (runs on every start, so it also corrects
  existing volumes) and in the Dockerfile (fresh volumes copy the image's mode). Verified in a container; a test
  now checks both files.

### Low

- **L1 (open, suggest follow-up): misleading Caddy log line.** On every start Caddy logs a warning that host
  checking is disabled for the admin endpoint. On a 0600 socket this has no effect, but it reads like a problem in
  operators' logs. Suggested approach: set the admin `origins` value Charon already sends, or document the line in
  the troubleshooting page.
- **L2 (open, carried from the first round): a failed proxy host save leaves an unhandled promise.**
  `ProxyHostForm` shows the error and then re-throws it from the form's submit handler, which nothing awaits.
  Suggested approach: stop re-throwing and add a test for the refused save.
- **L3 (open, existing behaviour, suggest an issue): some forward hosts pass validation but fail later.** A forward
  host with a scheme, a port or a bare IPv6 address passes the service checks, but Caddy then refuses the config,
  so the API answers 500 and nothing is saved. The UI does not send these forms. Suggested approach: either strip
  the scheme and port when building the upstream address, or refuse these forms with a 400 at validation.
- **L4 (open, existing, suggest an issue): `scripts/integration-test.sh` no longer completes.** The deprecated script
  (still called from the Makefile) sends first-run setup without the setup code that is now required, so it stops
  before its readiness step. Suggested approach: retire it in favour of the skill it points to, or pass the code.
- **L5 (open, cosmetic): template empty state for standard users.** When there are no templates, a standard user
  sees "Use the form above to create one", but the form is administrator-only.
- **Informational.** Some historical design notes under `docs/implementation/` and `docs/security/` still describe
  the old TCP admin port. They are not published to the docs site.
- **Informational.** The CodeQL JS hook deletes `frontend/coverage` and `test-results` before it scans, so running
  it at the same time as frontend coverage breaks the coverage run. Run them one after the other.

## Patch coverage

After the QA tests, the changed lines that are still uncovered are:

- `backend/internal/caddy/client.go:53-54`: a type check on the HTTP transport that cannot fail.
- `backend/internal/caddy/client.go:119-120`: logging when closing a response body fails.
- `backend/internal/services/proxyhost_service.go:170`: an error branch on re-encoding JSON that was just parsed.
- `backend/cmd/api/main.go:96-97`: the `caddy-admin` dispatch, which exits the process. A new subprocess test runs
  this path, but coverage from the child process is not collected.
- `frontend/src/components/ProxyHostForm.tsx:623`: the refused-save message; see L2.

## Changes made by QA (uncommitted, in the working tree)

- `.docker/docker-entrypoint.sh`, `Dockerfile`: make the admin socket directory 0700 (M1).
- `backend/internal/caddy/proxy_admin_loopback_test.go`: the entrypoint and Dockerfile restrict the socket
  directory before Caddy starts.
- `backend/internal/caddy/admin_socket_test.go`: `caddy-admin` usage and failure paths (extra arguments, a
  directory that cannot be created, a failed write, an invalid setting); `PrepareAdminSocket` failures
  (unreadable directory, a socket that cannot be checked, a stale socket that cannot be removed; skipped when
  running as root); `Fetch` failures (no base address, no context, missing socket, truncated body).
- `backend/internal/config/caddy_admin_test.go`: path handling when the working directory cannot be resolved.
- `backend/internal/services/host_permissions_test.go`: the forward target helper's fallbacks.
- `backend/cmd/api/main_caddy_admin_test.go`: the binary answers `caddy-admin listen` without starting the server.
- `frontend/src/pages/__tests__/Notifications.roles.test.tsx`: an administrator can open the template form and
  delete a provider after confirming.
- `scripts/integration-test.sh`: comment updated to describe the admin socket.
- `docs/reports/qa_report.md`: this report.
