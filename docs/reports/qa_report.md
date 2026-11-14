# QA Report: Orthrus Agent and Connections

- Branch: `fix/orthrus-transport` (15 commits on `origin/development`, up to `4b96d4016`)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 8 (PR 33)
- Scope: backend (agent handshakes, external Docker proxy listen address, inspect responses, request filtering,
  key rotation, install instructions), agent (key file, link rule, stream limits, request filtering), frontend
  (listen address setting, rotate key dialog, install wizard, locales), compose examples, release workflow for
  agent image tags, Playwright specs, docs.
- Images under test: `charon:local` from `docker-rebuild-e2e`, and a throwaway agent image built from `agent/`.
- Toolchain: Go 1.27.4.
- Date: 2026-11-14

## Verdict

**Approved.** Nothing blocking. All gates pass, and the container gate matches the plan on every step. QA added
tests to both copies of the request filter (see "Changes made by QA"). Three low findings and four informational
notes remain. Each is suggested as a follow-up (see Findings).

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `docker-rebuild-e2e` | Pass, container healthy |
| Playwright `orthrus-agents`, `orthrus-external-proxy`, `orthrus-agent-install` (`--project=firefox`) | Pass, 58/58 |
| `scripts/go-test-coverage.sh` | Pass, 93.8% statements / 92.7% lines (gate 87%), 0 failures |
| `scripts/agent-test-coverage.sh` | Pass, 86.0% statements / 86.6% lines with the QA tests (84.8% / 85.0% before) |
| `scripts/frontend-test-coverage.sh` | Pass, 362 files / 4579 tests, 91.9% statements / 92.8% lines |
| `bash scripts/local-patch-report.sh` (fresh coverage inputs) | Pass. Overall 837/873 (95.9%); backend 666/697 (95.6%); frontend 98/100 (98.0%); agent 73/76 (96.1%) |
| `./scripts/scan-gorm-security.sh --check` (new column) | Pass, 0 critical/high/medium |
| `lefthook run codeql` (Go and JS) | Pass. Go: 0 blocking (4 existing suppressed results, none in files this PR changes). JS: 0 |
| Trivy filesystem (`security-scan-trivy`) | No findings in this branch's manifests. Hits come only from leftover local `.claude/worktrees/` copies and an untracked, git-ignored local key file that is not in the image |
| Trivy image (`charon:local`) | Alpine and the Charon binary: 0 high/critical. Bundled Caddy and CrowdSec binaries: existing items with no upstream fix, already listed in `.trivyignore` and `SECURITY.md` |
| `npm run type-check`, `npm run lint` (max warnings 0) | Pass |
| Builds: `go build ./...` (backend, agent), `npm run build` | Pass |
| `actionlint` on `orthrus-build.yml` and `docker-build.yml` | Pass |
| `docker compose config` checks (see below) | Pass |
| `lefthook run pre-commit --all-files` | Pass |
| `make lint-backend`, `make lint-agent` (full golangci-lint with gosec, run last) | Pass, 0 issues. The first backend run found one `errcheck` issue in a new test (`session_bounds_test.go`); QA fixed it and the re-run is clean |

### Compose checks

These ran in a scratch copy of `.docker/compose/` with no `orthrus_auth_key` file and a minimal `.env`. The files
already require `DOCKER_SOCK` and `CHARON_EMERGENCY_TOKEN`; that is unchanged from `origin/development`.

| Check | Result |
| --- | --- |
| `docker-compose.yml config`, `docker-compose.local.yml config` without a key file | Valid; `--services` lists only `charon` |
| Same files with `--profile orthrus` and a key file | Valid; `orthrus` listed |
| `docker-compose.remote.yml`, `docker-compose.dev.yml` with a key file | Valid |

## Throwaway container gate

A fresh `charon:local` container and agent containers on their own random-named networks (`pr33-*`), with the API
published on loopback only. Shared and live containers were not touched. All throwaway containers, networks and
images were removed afterwards.

| Check | Result |
| --- | --- |
| Agent start with `ORTHRUS_AUTH_KEY_FILE` (root-owned file, then privilege drop) | Connects; startup log shows `auth_key_source=file`, key redacted; container env holds no key |
| Default listen address | Listener on the container's network address (`172.21.0.2:23750`), not `0.0.0.0`; `bind_mode` `auto`, source `default_route` |
| Container inspect through the external proxy | Every `Config.Env` value is `[redacted]`, names kept. Same with gzip requested, with a raw `Accept-Encoding: gzip` header, with a version prefix and with an encoded path. Path variants the daemon does not serve return a redirect or 404 with no environment data. Charon's own container inspected the same way: no value shown |
| Registry lookups through the read-only proxy | 403 for `127.0.0.1:5000`, `127.1:5000`, `10.1`, `0x7f.1`, `0x7f.0x0.0x0.0x1:5000`, `localhost`, `myregistry:5000`, `[::1]:5000`, `2130706433:5000`, and the versioned `127.1` path; `nginx` is forwarded (200) |
| Second network attached to Charon | Listed by `GET /orthrus/local-networks`. A client on it cannot connect with the automatic setting; after choosing that subnet it can, and the first network no longer can |
| Listen address validation | Interface names, zoned addresses and malformed subnets: 400. A valid address that is not local: status error, no listener at all (no fallback to all interfaces) |
| `ws://` to a public name or public IP | Agent exits with the opt-in message |
| `ws://` with `ORTHRUS_ALLOW_INSECURE_WS=true`, and `ws://charon.home.arpa` | Agent proceeds to dial |
| `ORTHRUS_ALLOW_INSECURE_WS` set to an unparsable value | Agent exits with a clear message |
| Failed handshakes naming agent A from one client | 401 ×10, then 429 with `Retry-After` |
| Agent B from the same client, correct key | Upgrades (101) |
| Failures that name no known agent, same client | Separate budget, still 401 |
| Agent A from its own address | Stays connected |
| Cost of a failed handshake | About one key check for an existing agent, an unknown agent ID and an unknown name (see L2) |
| Rotate key for agent A | 200 with `Cache-Control: no-store` and a new key. A is disconnected and the old key gets 401. The audit entry `orthrus_key_rotated` holds only the agent name; the key is not in the audit log. After the key file is updated and A is restarted, A reconnects with the same UUID, listen setting and remote server link |
| Key file edge cases | Empty or whitespace-only file: "the agent key is not set yet; paste the key shown in Charon into this file". Relative path, directory and missing file: clear messages. Key file plus `ORTHRUS_AUTH_KEY` (non-empty file): "set only one of". No message contains the key |

## Security review

- **Agent handshakes.** Failed attempts are throttled per client address and per named agent before any key is
  checked, and successful connections are never charged. Key checks are limited to two at a time; a re-check after
  a slot is taken counts failures recorded while waiting. Each failure path costs one key check, except the bounded
  legacy scan for agents that send no agent ID. That scan has its own global budget and costs one check per
  registered agent, as designed. Follow-ups: L1, L2.
- **Inspect responses.** The flag that asks for redaction is set from the same normalized path the allow-list
  uses, in read-only and write-enabled sessions. Compressed, oversized or undecodable bodies are refused (502), so
  an unredacted body is never forwarded. Verified on a real daemon above.
- **Request filtering.** The registry rule is identical in the backend and agent copies, and both mirrored test
  tables now include the hex-label forms. The allow-list parity script passes. Write-enabled sessions are
  unchanged by design.
- **Agent key.** The key file is read before the privilege drop, must be an absolute path to a regular file of at
  most 4 KiB, and is never logged. Compose examples and generated snippets use the file; the main and local
  compose files keep the agent behind a profile, so a fresh install without a key still starts.
- **Commit subjects.** Exactly three `(security)` subjects, all with empty bodies and category-level wording. No
  other subject or body names a cause or path. Shipped text (code comments, tests, docs, locales) contains no
  tracker identifiers.

## Findings

### High / Medium

None.

### Low

- **L1 (open, suggest follow-up, track privately): handshake budget keying for legacy agent lookups.** The budget
  for handshakes that identify an agent the legacy way can be tightened. Suggested approach: give legacy lookups a
  budget key separate from the agent-ID path, and keep recommending `CHARON_TRUSTED_PROXIES` when agents connect
  through another proxy. Details go to the maintainer, not to a public issue.
- **L2 (open, suggest follow-up): the comparison hash for unknown agents is created on first use.** The plan
  creates it when the handshake guard is built. Today the first failure for an unknown agent costs one extra key
  check, once per process. Suggested approach: create it in `newHandshakeGuard`.
- **L3 (open, existing behaviour, suggest a public issue): the proxy connection string ignores the listen
  address.** `connection_string` in proxy-status is built from the host the browser used. With the loopback
  fallback or a non-primary subnet, it can name an address that cannot reach the proxy. The dialog also shows the
  real listening address beside it. Suggested approach: hide or adjust the connection string when the listener is
  on loopback, or build it from the bound address when that is not the primary network.

### Informational

- A key rotation that lands while a handshake for the same agent is being verified can let that one connection
  open after the disconnect. It closes at the agent's next reconnect. Revoke behaves the same way. Suggested
  approach: re-check the stored hash after the session is registered.
- The `ws://` rule treats any single-label host as private, including an all-digit one. That matters only with a
  resolver that reads such names as addresses; the agent is built with the pure Go resolver.
- The provision response, which also returns a key once, does not set `Cache-Control: no-store` like the rotate
  response does. This is existing behaviour; suggested as a small follow-up.
- `agent/leash/leash.go:226-229`: the error branch for setting the stream type deadline cannot run, because yamux
  never returns an error there. The comment a few lines below says the same thing about clearing it.
- Leftover local `.claude/worktrees/` copies (from older sessions) hold old dependency manifests that Trivy
  filesystem scans report. They are not part of the repository; removing them keeps local scans clean.

## Patch coverage

After the QA tests, the changed lines that are still uncovered are error branches that cannot be reached in
tests:

- `backend/internal/orthrus/handshake.go`, `server.go`: failures of the system random source, and limiter
  construction errors for fixed, valid settings.
- `backend/internal/orthrus/redact.go:86-97`: re-encoding errors for values that were just decoded.
- `backend/internal/orthrus/bind.go`, `backend/internal/services/orthrus_service.go`: unreachable branches of
  address parsing and key generation.
- `agent/leash/leash.go:227-229`: see the informational note above.
- `frontend/src/components/hecate/RotateAgentKeyDialog.tsx:44`, `AgentExternalProxyDialog.tsx:56`: one fallback
  branch each.

The backend `muzzle.go` lines listed in the report (hex labels and the empty-label check) are covered by the QA
tests below. The report predates them because the backend coverage run was not repeated for these tests.

## Changes made by QA (uncommitted, in the working tree)

- `backend/internal/orthrus/muzzle_registry_test.go`: two hex-label registry cases, and a test for the empty-label
  check.
- `backend/internal/orthrus/session_bounds_test.go`: check the value received from the accept channel (full lint
  `errcheck`); the failure message now includes it.
- `agent/muzzle/registry_test.go`: the same two registry cases (keeps the mirrored tables identical).
- `agent/muzzle/registry_internal_test.go` (new): the empty-label check, and a test that a stream that cannot take a
  header deadline is refused.
- `docs/reports/qa_report.md`: this report.
