# QA Report: Destination Rules for Monitors, Connection Tests and Forward Targets

- Branch: `fix/probe-destination-policy` (15 commits on `origin/development`, head `dda299d95`)
- Scope: one destination policy for Charon's own outbound checks (uptime TCP and HTTP monitors, the per-host TCP
  check, the proxy host connection test and both remote server tests), checked on the address being connected to;
  the same address rules for forward targets, location targets and advanced-configuration upstreams for every role;
  the ports Charon keeps for itself on this machine; one fixed refusal text per surface; a per-user limit on
  connection tests; stored monitors and hosts that the rules now refuse, with a passive note in the host list; one
  name lookup budget per import; bracketed upstream addresses for IPv6 forward targets; monitor creation input
  errors; remote server test latency; matching UI, locales and docs.
- Image under test: a throwaway image built from the working tree (`pr34qa-charon:qa`), run with the CI E2E settings
  (`CHARON_ENV=test`, security tests enabled, bundled CrowdSec disabled, a named data volume, emergency server on).
  All containers, networks, volumes and the image were throwaway (`pr34qa-*`) and were removed afterwards. The live
  containers were not touched.
- Toolchain: Go 1.27, CodeQL CLI 2.27.2, golangci-lint 2.14.0, Caddy 2.11.9 (in the image).
- Date: 2026-11-15. Two QA rounds; this report covers the whole branch as of round 2.

## Verdict

**Approved.** Every Definition of Done gate passes, and every container check matches the plan. The round 1
findings are fixed and verified in the container (see "Round 1 findings"). Two low items and one nit remain. None of
them blocks the PR.

## Remaining findings

| Id | Severity | Finding | Remediation |
| --- | --- | --- | --- |
| R1 | Low | When an import's name budget runs out, the hosts after that point skip the name check. A name on a reserved port is then accepted at save time. A single save refuses that case. The configuration generator still checks such a name on a reserved port when it applies the configuration, and leaves the host out when the name cannot be resolved or names this machine, so the host is never routed | Optional: keep the "lookup did not finish" verdict for names on reserved ports (refuse) instead of dropping it, in `ImportNameBudget.preflight` (`backend/internal/services/import_name_budget.go`). Add a unit test |
| R2 | Low | Import commit errors return the service error text, for example `forward target uses a port reserved by this proxy`. Proxy host saves and tests return one fixed text for every cause. Imports are administrator-only | Optional: map the destination refusals to the fixed forward target text in the three import commit paths |
| R3 | Nit | Some test titles and comments name the refused address class with a word the public wording rules avoid (the label of the link-local target constant in `authorization-rbac.spec.ts`, the refused-target comments and titles in `proxy-hosts.spec.ts` and `create-monitor.spec.ts`) | Rename to "link-local address" or "cloud instance service address" |

## Round 1 findings (all fixed, verified in round 2)

| Id | Finding | Verified |
| --- | --- | --- |
| M1 | Administrators did not receive `active_config:false` | Four hosts were stored with refused targets (literal, reserved port, a location on a legacy-form address, an advanced configuration dialing a unix socket), then an apply ran. List and get, as administrator and as standard user, carry `active_config:false` on all four. The healthy host omits the key. Administrators still get every stored field and no `has_advanced_config`, as before. Standard users get the redacted view. The new real-backend E2E test passes |
| M2 | The per-import name budget was missing | A 50-host Caddyfile import with an unreachable resolver finished in 30.0 s with all 50 hosts created. One Warn was logged, `unchecked_hosts:37`, with no host, name or address |
| L1 | HTTP monitors to a name that resolves to loopback reported refused | `http://<name>:8080/...`, with the name mapped to `127.0.0.1`, reports up (HTTP 200); `http://127.0.0.2:8080/...` reports up. The same name on 2019 or 8085 is refused on create. A stored monitor on that name and port 2019 reports `Destination is not allowed for checks` when it runs |
| O1 | IPv6 forward targets produced an invalid upstream address | `fd12:3456::1`, `::1` and a location on `fd12:3456::9` save and apply. The running configuration has `[fd12:3456::1]:8080`, `[::1]:8080` and `[fd12:3456::9]:9000`. An advanced configuration with `[fd12:3456::2]:8080` and `tcp6/[fd12:3456::3]:80` applies. `[fd00:ec2::254]:80` in an advanced configuration and `::1` on 2019 are refused. Monitor sync handles the IPv6 hosts |
| CodeQL | One new result and three stale entries | Folded into the branch. The gate passes |
| Nits | Misplaced and duplicated doc comments | Fixed |

## Gates run (all foreground or block-waited)

| Gate | Result |
| --- | --- |
| E2E image | Throwaway image built from the working tree (it contains the round 2 code) and run as its own container with the CI E2E settings. Playwright ran with `CI=true` (one worker, two retries) against it via `PLAYWRIGHT_BASE_URL` and `PLAYWRIGHT_CONTAINER_NAME` |
| Playwright `--project=firefox`: `core/proxy-hosts` (with the new real-backend test), `core/remote-server-test-connection`, `monitoring/create-monitor`, `monitoring/uptime-monitoring`, `core/caddy-import/caddy-import-firefox`, `tasks/import-caddyfile` | 108 passed, no retries (round 1 also passed `orthrus-external-proxy`, `orthrus-proxy-paths` and `security/self-proxy-requests`) |
| Playwright `--project=security-tests`: `security-enforcement/authorization-rbac` | 138 passed |
| Connection test limit headroom | The limit is 10 per minute per user, shared by the three test endpoints, and is checked after the role check (a 403 does not use it). In security shard 2 only `authorization-rbac` calls the test endpoints (administrator 5 calls, standard user 3). In the non-security firefox shards, `core/proxy-hosts` (2 calls) and `core/remote-server-test-connection` (3) run as the shared administrator, 5 in total even in one shard. No spec reaches 429 |
| GORM security check (`--check` mode) | Pass: 0 critical, 0 high, 0 medium |
| `bash scripts/local-patch-report.sh` | Pass. Overall 97.0% (929/958), backend 97.0% (909/937), frontend 95.2% (20/21). The uncovered lines are error branches (response build failures, store and close errors) |
| `CHARON_TEST_TIMEOUT=60m scripts/go-test-coverage.sh` | Pass, all packages, race detector on, no data race. Statements 93.8%, lines 92.8% (gate 87%). `services` 92.0%, `handlers` 92.9%, `network` 98.5%, `caddy` 98.0%, `security` 97.1%. `TestCheckHost_Debouncing` is not flaky: in round 1 it ran 3 of 3 times at about 2 s each, and it passes in both full runs. Its package stubs every name lookup |
| `scripts/frontend-test-coverage.sh` | Pass: 378 files, 4697 tests. Lines 92.88%, statements 92.03%, branches 86.81% |
| `npm run type-check`, `npm run lint` | Pass |
| `lefthook run pre-commit --all-files` | Pass: all 20 hooks, including semgrep |
| CodeQL Go and JS (the pre-commit CodeQL scripts and the findings gate) | Pass. Go: 4 results, all on file. JS: 0 results |
| Trivy (image, HIGH and CRITICAL) | No result in Charon's own binary. 3 HIGH results in bundled third-party binaries (Caddy: go-jose v3; CrowdSec and cscli: pgproto3 v2), already on file in `.grype.yaml`. This branch changes no dependency |
| `make lint-backend` (run last) | Pass: 0 issues. `agent/` is unchanged, so `make lint-agent` was not needed |

## Container checks (throwaway resources, real HTTP)

Names were mapped through `/etc/hosts` in the container (`--add-host`, then rewritten as root inside the throwaway
container for the connection-time checks). Rows for stored-state checks were written straight into the throwaway
database. Round 1 ran the full matrix. Round 2 ran the fixed items above plus spot checks of the refusals, the limit
and the logs.

### Refused, as administrator and as standard user

Each case was tried as proxy host create, location target and update; most were also run as a connection test.
Every one returned 400 with the same body `{"error":"This forward target is not allowed.","error_code":"forward_target_refused"}`:

- addresses: `169.254.169.254`, a name mapped to it, a name mapped to it and to `127.0.0.1`, `fd00:ec2::254`,
  `fd20:ce::254`, `168.63.129.16`, `100.100.100.200`, `0.0.0.0`, `224.0.0.1`, `ff02::1`, `240.0.0.1`,
  `169.254.1.1`, `fe80::1`, `fe80::1%eth0` (connection test), `::ffff:169.254.169.254`, `0xa9.0xfe.0xa9.0xfe`;
- port 2019 on this machine in every loopback, legacy, name, interface and gateway form tried: `127.0.0.1`,
  `localhost`, `LOCALHOST`, `localhost.`, `0x7f.1`, `127.1`, `::1`, `0.0.0.0`, the container host name, the container
  address, `host.containers.internal`;
- the other reserved ports: `127.0.0.1:8085`, `127.0.0.1:2020` with the emergency server on, and `127.0.0.1` on a
  stored agent port.

Monitor create and the remote server tests (custom and stored) gave the same
`{"error":"This destination is not allowed.","error_code":"destination_refused"}` for every cause. With the emergency
server off, `127.0.0.1:2020` was accepted as a forward target and as a monitor; 8085 and 2019 stayed refused.
Standard users get 403 on the remote server tests.

### Allowed

- Forward targets, for both roles (create, location, update): `127.0.0.1:8080`, `192.168.1.50`, `100.64.1.1`,
  `198.18.0.5`, `host.containers.internal:8080` (mapped to `169.254.1.2`), `host.docker.internal:8080`, a name mapped
  to a private address, the container's own address on 8080, and the IPv6 literals and unique-local addresses
  listed under O1.
- Monitors: the same classes, plus `[fd12:3456::1]:8080`, `tcp://192.168.1.1:5432`, and names that resolve to
  loopback (L1).

### Connection-time check

A name allowed at save time was then remapped:

- To `0.0.0.0` only (which connects locally if not refused) or to `169.254.169.254` only: TCP and HTTP monitors
  reported `Destination is not allowed for checks`. While the save-time verdict was still cached (up to 30 s), the
  proxy host test answered 502 `connection_test_failed` and the remote server test `reachable:false`. After that,
  both answered 400.
- To a refused and an allowed address, in either order: the TCP monitor connected to the allowed address and
  reported up. The HTTP client and the save-time checks refuse a name with any refused address.

An HTTP monitor to a host answering 302 to a refused address recorded `HTTP 302` and did not follow the redirect.

### Stored monitors and hosts

- Monitors rewritten to refused destinations were kept, and each check reported down with the fixed text. About 30 s
  after start (the bootstrap delay), one Warn per literal or this-machine monitor was logged with the monitor id,
  name and type and no address.
- Hosts with refused stored targets were left out with a Warn that names only the host UUID. Their domains were
  absent from the running configuration, and the rest of the configuration applied and served. Both roles see
  `active_config:false` (M1).
- A stored host whose name maps to a refused address stays in the configuration. This is the documented limit of
  this PR: names are checked when saved.

### Advanced configuration (administrator)

Refused with the fixed body:

- networks and forms: `unix//...`, `unixgram//...`, `fd/3`, `udp/...`, a placeholder dial, and a dial that does not
  parse;
- reserved ports: `127.0.0.1:2000-2100` (a range over a reserved port), `tcp/127.0.0.1:2019`, `127.0.0.1:8085`;
- refused addresses: `tcp4/169.254.169.254:80`, a name mapped to a refused address, a `subroute` to a refused address,
  and IPv6 instance service addresses in bracketed form;
- `dynamic_upstreams` and a transport `resolver`.

Subroutes to private, unique-local and bracketed IPv6 upstreams were accepted and applied. A top-level
`reverse_proxy` in an advanced configuration fails the configuration check whatever its dial. This is known and
already scheduled for a public issue.

### Name lookups and the write lock

With an unreachable resolver, a proxy host save with two names that could not be resolved took 2.08 s (the lookups
run in parallel) and was accepted. A proxy host save with a literal target and a monitor create sent during it
finished in 0.07 s and 0.01 s, so the lookups run before the write lock. A name that cannot be resolved is refused on
a reserved port in a single save. Imports share one 30 s budget (M2, R1).

### Connection test limit

Ten tests were allowed; the eleventh answered `429` with `Retry-After: 6`. Five proxy host tests plus five remote
server tests used up the same allowance. A standard user was unaffected while the administrator was limited, and
in round 2 the standard user's own eleventh test was limited.

### Logs

After the checks, the container log, the log files, and `GET /logs/charon.log` read as a standard user contained none
of the addresses, names or ports used in connection tests. With debug off, connection tests write nothing. The debug
line carries only the caller id and the outcome.

### Regression review

- The removed admin-port helper and its error are covered by the reserved-port rule: every form of port 2019 above is
  refused, for every role, with the same text.
- Monitor creation: bare `host:port` and `tcp://host:port` both create TCP monitors. Invalid input returns 400 with a
  message instead of 500.
- Remote server test latency is the measured dial time (0 ms on loopback, below one millisecond).
- Proxy host responses for administrators keep the same fields and JSON types as before, plus `active_config` when a
  host is left out.

## Out-of-scope findings (not filed by QA)

| Id | Severity | Finding | Where | Suggested approach |
| --- | --- | --- | --- | --- |
| O2 | Medium, non-security, pre-existing | A proxy host with two or more locations fails the configuration check (`duplicate host with paths`) and the save returns 500 | `backend/internal/caddy/validator.go:93-119`: each location route of the same host counts as a duplicate. The existing tests use one location only | Refuse only identical path sets for one host; add validator and generator tests with two locations. Needs a public issue |

No security-sensitive out-of-scope finding was found.
