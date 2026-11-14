# QA Report: Forward Target Checks (Round 2)

- Branch: `fix/orthrus-listener-targets` (4 commits on `origin/development`: `7f7f0a93d`, `164170a34`,
  `b2cddbf49`, `3100cb08f`)
- Scope: backend (forward target policy service, config generator filter and rollback guard, agent server hook,
  proxy host and agent handlers, route wiring), removal of the agent-name forward target form from the config
  generator, one section in `docs/features/orthrus.md`, and one Playwright spec fix from round 1.
- Images under test: `charon:local` from `docker-rebuild-e2e`, agent image `wikid82/orthrus:pr-1957` (no agent
  changes on this branch).
- Toolchain: Go 1.27.4.
- Date: 2026-11-14

## Round 1 outcome

Round 1 was blocked on one high finding: a forward target form that the new check skipped by design still reached
an internal listener at config generation. The developer removed that form: writes that use it are refused for
every role (host, location, import, Test Connection), stored rows that use it are left out at apply, and the
generator no longer resolves it. The developer also addressed the medium and low round 1 items (host-side
addresses, refusal when a name cannot be resolved, and a rollback guard). Round 2 re-tested each item.

## Verdict

**Approved.** Every gate passes. The round 1 blocking case is closed and was re-tested in a container for both
roles. The remaining findings are low or informational residuals that do not block this PR; they are listed below
with remediation for follow-up tracking.

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `docker-rebuild-e2e` | Pass, container healthy |
| `bash scripts/local-patch-report.sh` | Pass. Backend 318/324 changed lines (98.1%), frontend and agent 0 changed lines. QA added tests for three gap ranges (see "Changes made by QA"); the 6 remaining lines are defensive error branches (see below) |
| `./scripts/scan-gorm-security.sh --check` | Pass, 0 critical/high/medium |
| `scripts/go-test-coverage.sh` | 93.8% statements / 92.8% lines (gate 87%). The second run had one failure in an unrelated startup test under load; QA fixed the cause (see "Changes made by QA") and the package passes |
| Playwright firefox: `core/proxy-hosts*`, `proxy-host-drag-drop`, `core/proxy-host-docker-source`, `orthrus-*.spec.ts` | Pass, 121 passed, 13 skipped (existing state-dependent skips) |
| Playwright `security-enforcement/authorization-rbac` (`--project=security-tests`) | Pass, 116 passed |
| UI and specs using the removed form | None. No frontend code, Playwright spec or user doc builds or documents an agent-name forward target |
| CodeQL Go (`codeql-go-scan.sh`, CLI 2.27.2) | Pass. 4 existing results, none in files this branch changes |
| Trivy | Skipped: no dependency or image input changes |
| `lefthook run pre-commit --all-files` | Pass (all hooks; existing lint warnings only, 0 errors) |
| `make lint-backend` (full golangci-lint with gosec, run last) | Pass, 0 issues |

Gotify token review: not applicable, no notification code or artifacts in scope.

## Container gate

Throwaway stack only: network `pr-fwd2-net`, `pr-fwd2-charon` (`charon:local`), a connected agent
`pr-fwd2-orthrus`, and a test backend `pr-fwd2-web`. The agent's external proxy was on port 2375; the session
loopback port was ephemeral. The stack was removed after the gate.

| Check | Result |
| --- | --- |
| Round 1 blocking case: agent-name target on ports 80, 2375, the loopback port and 1, plus a leading-space variant, standard user and admin | 400 `forward_target_refused` for every case; nothing created, nothing served |
| Round 1 spellings on the external port and the loopback port, both roles (loopback names and literals, legacy and mapped forms, container IP/name/hostname with case and trailing-dot variants, `http://`/`https://` forms) | 400 `forward_target_refused` (104/104) |
| Host-side cases: attached network gateway address and `host.docker.internal` (any case, trailing dot) on listener ports | 400 `forward_target_refused` |
| Unresolvable name on a listener port | 400 `forward_target_refused`; the same name on another port is accepted |
| Location target, update of an existing host, Test Connection, JSON import (admin; standard users get 403) | Refused with the same error for every target above; the stored host is unchanged; Test Connection to other targets still dials |
| Normal hosts: loopback on another port, another container or remote IP on a listener port number, gateway on another port | 201 |
| Stored rows inserted directly into the database (agent-name form, loopback, gateway, unresolvable name, refused location) | Each left out at apply with a neutral warning naming the host. The unmatched host name falls through to the default page; other hosts keep serving |
| External proxy port changed to the port of an existing host | That host is left out on the re-apply; it returns when the port moves away |
| Agent reconnect | Re-apply runs without errors; skipped hosts stay skipped, other hosts keep serving |
| Self-proxy: host whose upstream is Charon's own API | Hop headers and the recovery route are still generated; the recovery path reaches the API with the real client address. Normal proxy hosts are unchanged |
| Rollback | Covered by unit tests (refused snapshot is not reinstated; a failed check keeps it out; an accepted snapshot is reinstated). Not forced in the container |

## Review of the generator changes

The generator diff removes the agent-name resolution step and the per-host "remote upstream" flag that only those
rows used. With those rows now refused at write and dropped at apply, no generated route can carry an upstream
that was resolved from that form, so the flag had no remaining input. Normal proxy hosts, location routes, the
self-upstream hop headers, the recovery route and the emergency path generate the same output as before; the
existing recovery-route and self-upstream unit tests pass, and the container check above confirms it. QA fixed one
broken comment and one stale field comment left by the removal.

## Findings

### R1: Low. Other Docker host addresses can reach a published external proxy port

The check now covers the gateway and first address of each network Charon is attached to, and the Docker host
gateway name. If an admin publishes an agent's external proxy port on the Docker host, any other address of that
host (another bridge gateway, the LAN address) also reaches it, and those addresses cannot be discovered from
inside the container. In the gate, the default bridge gateway address on the external port saved with 201. The
port was not published, so nothing was reachable.

Remediation: add a sentence to the agent docs that a published external proxy port should be bound to a specific
address, and track a dial-time guard (see R2) that would close this for every address.

### R2: Low. Name resolution timing (carried from round 1)

Names that fail to resolve on a listener port are now refused. A name whose records change after the check can
still reach a listener later, because the proxy resolves names again on each connection. This requires control of
a DNS name and access to create hosts.

Remediation: track a dial-time guard as a follow-up hardening item.

### R3: Low. A failed apply keeps the running config (carried from round 1)

The rollback guard works as designed. Separately, when an apply fails (for example, Caddy rejects the new config),
Caddy keeps its running config, which can still include a host that the guard now refuses, until the next
successful apply. The short window after an agent connects, before the background re-apply finishes, is
unchanged.

Remediation: track with R2. Consider retrying the apply after a failure that follows a listener change.

### R4: Info. Agent-name check matches the exact prefix only

The write check refuses the agent-name form by exact prefix. Upper-case or URL-wrapped variants are not caught by
that check, but nothing resolves them any more: the config validation rejects them and the write is rolled back
(500, nothing stored). No exposure.

Remediation: optional. Refusing forward hosts that contain a colon after the optional scheme (see R5) would
return a 400 for these too.

### R5: Low (out of scope, existing behaviour). Some malformed forward hosts return 500

A forward host that carries a colon without a numeric port passes the form check and fails later at config
validation with a 500. The write is rolled back and the running config is unchanged. This predates the branch.

Remediation: validate the optional port in the forward host form check and return 400. Track as a separate issue.

### R6: Info. Admin-only paths not covered by the guard

Uptime monitor targets (carried from round 1) and upstreams in a host's advanced config are not checked. Both are
admin-only (standard users get 403), and admins already manage the agent listeners. An advanced config upstream is
currently rejected by config validation for an unrelated reason, so it is not usable today either.

Remediation: track with R2 so a future dial-time guard covers every path.

## Changes made by QA (uncommitted)

- `backend/internal/caddy/forward_targets_test.go`: a rollback whose target check fails does not reinstate the
  snapshot (covers the new error branch in `manager.go`).
- `backend/internal/services/forward_target_policy_test.go`: the default name resolver answers an address literal
  (covers the resolver variable without a network lookup).
- `backend/internal/api/routes/routes.go` and `reapply_proxy_config_test.go`: the agent-connect hook closure moved
  into a small named function with a test, so the wiring is covered.
- `backend/internal/caddy/self_upstream.go`, `backend/internal/caddy/manager.go`: fixed a broken sentence and a
  stale field comment left by the removal.
- `backend/cmd/api/main_test.go`: the in-process and subprocess startup tests waited at most 10 seconds for the
  server before sending SIGTERM. Under `-race` on a loaded machine startup can take longer, and a signal sent
  before the handler is installed kills the test binary. The wait is now 60 seconds, with a comment. Unrelated to
  this branch.
- `docs/reports/qa_report.md`: this report.

Remaining uncovered changed lines: JSON encode/decode error branches in the snapshot check (4 lines, not reachable
with a generated config) and an address conversion branch in the policy file (2 lines, not reachable for a parsed
IP).
