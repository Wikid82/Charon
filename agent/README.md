# Orthrus Agent

Orthrus is a lightweight reverse-proxy agent that dials out to a [Charon](https://github.com/Wikid82/Charon) server over a secure WebSocket connection. It allows Charon to route traffic to remote services that are not directly reachable via a public IP — without requiring firewall port-forwarding.

Once connected, Charon multiplexes Docker socket and TCP port-forward streams back to the host running the agent using [yamux](https://github.com/hashicorp/yamux).

## Quick Start

### Docker

```bash
docker run wikid82/orthrus \
  --server-url wss://charon.example.com/api/v1/ws/orthrus/connect \
  --auth-key ch_orthrus_<your-key-here>
```

### Key file (recommended)

Save the key to a file, restrict it, and mount it. The agent reads it before it drops root, so the file can stay `0600`:

```bash
printf '%s' 'ch_orthrus_<your-key-here>' > orthrus_auth_key && chmod 600 orthrus_auth_key
docker run \
  -e ORTHRUS_SERVER_URL=wss://charon.example.com/api/v1/ws/orthrus/connect \
  -e ORTHRUS_AUTH_KEY_FILE=/run/secrets/orthrus_auth_key \
  -v "$PWD/orthrus_auth_key:/run/secrets/orthrus_auth_key:ro" \
  -e PUID=65532 \
  -e PGID=$(stat -c '%g' /var/run/docker.sock) \
  -e TZ=UTC \
  -e ORTHRUS_DOCKER_SOCKET=/var/run/docker.sock \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  wikid82/orthrus
```

### Environment Variables

```bash
docker run \
  -e ORTHRUS_SERVER_URL=wss://charon.example.com/api/v1/ws/orthrus/connect \
  -e ORTHRUS_AUTH_KEY=ch_orthrus_<your-key-here> \
  -e PUID=65532 \
  -e PGID=$(stat -c '%g' /var/run/docker.sock) \
  -e TZ=UTC \
  -e ORTHRUS_DOCKER_SOCKET=/var/run/docker.sock \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  wikid82/orthrus
```

The agent runs as an unprivileged user. `PGID` must be the group that owns the Docker socket on the host (the `$(stat ...)` part above looks it up for you on Linux). See [docs/features/orthrus.md](../docs/features/orthrus.md#set-the-permissions-puid-pgid-and-tz) for a plain-language explanation.

> **Heads up:** The number from `stat` goes in `PGID` only. `ORTHRUS_DOCKER_SOCKET` stays the socket's *path* (`/var/run/docker.sock`); do not build it from that number. To check the number, run `getent group <number>`; it should print the `docker` group. The agent does not need the host user added to any group.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `ORTHRUS_SERVER_URL` | — | WebSocket URL of the Charon Orthrus endpoint (`wss://...` behind HTTPS, `ws://...` for direct HTTP) |
| `ORTHRUS_AUTH_KEY_FILE` | — | Absolute path to a file holding the auth key (`ch_orthrus_...`). Preferred. Read before the agent drops root; must be a regular file of at most 4 KiB; surrounding whitespace is trimmed. Cannot be combined with `ORTHRUS_AUTH_KEY` |
| `ORTHRUS_AUTH_KEY` | — | Auth key from the Charon provisioning response (`ch_orthrus_...`). Visible in `docker inspect` and the process environment |
| `ORTHRUS_DOCKER_SOCKET` | `/var/run/docker.sock` | Path to the local Docker socket |
| `PUID` | `65532` | User ID the agent runs as. Must not be `0` (root) unless `ORTHRUS_ALLOW_ROOT=true` is set |
| `PGID` | `65532` | Group ID the agent runs as. Set it to the Docker socket's group (`stat -c '%g' /var/run/docker.sock`) so the agent can read the socket. Must not be `0` unless `ORTHRUS_ALLOW_ROOT=true` is set |
| `ORTHRUS_ALLOW_ROOT` | `false` | Set to `true` to allow `PUID` or `PGID` of `0` (root). Not recommended: a root-level agent has full control of the host if compromised. Logs a loud warning at startup |
| `TZ` | `UTC` | Timezone used for log timestamps (e.g. `America/New_York`) |

## CLI Flags

All environment variables have equivalent CLI flags:

```text
--server-url   wss://charon.example.com/api/v1/ws/orthrus/connect
--auth-key     ch_orthrus_<key>     (prefer ORTHRUS_AUTH_KEY_FILE; the flag is visible in process listings and logs a warning)
--agent-id     <uuid>               (from provisioning response)
--docker-socket /var/run/docker.sock
--log-level    info
```

Environment variables take precedence over flags. For the auth key, the order is `ORTHRUS_AUTH_KEY_FILE`, then `ORTHRUS_AUTH_KEY`, then `--auth-key`. Setting both `ORTHRUS_AUTH_KEY_FILE` and `ORTHRUS_AUTH_KEY` is an error.

## Security

**Muzzle filter**: The agent only proxies a curated allowlist of read-only Docker API endpoints to Charon. All `POST`, `DELETE`, `PUT`, and `PATCH` requests are rejected with `403 Forbidden` before they reach the Docker socket. Allowed endpoints:

- `GET /v*/containers/json`
- `GET /v*/containers/*/json`
- `GET /v*/info`
- `GET /v*/images/json`
- `GET /v*/version`
- `GET /v*/events`

**mTLS enrollment** (future): Full mutual TLS enrollment via Certificate Signing Request (CSR) submitted to Charon's internal CA is planned. Currently the agent generates a self-signed ECDSA-P256 certificate in memory for local identification.

**TLS**: Use `wss://` when Charon is behind an HTTPS reverse proxy. Charon itself serves plain HTTP, so when connecting directly to its port (e.g. over Tailscale or a Docker network) use `ws://`; the agent logs a warning for non-localhost `ws://`. Using `wss://` against a plain HTTP port fails with `tls: first record does not look like a TLS handshake`.

**Auth key**: Never logged; it appears in logs only as `[REDACTED]`. Prefer `ORTHRUS_AUTH_KEY_FILE`: a Compose secret or Kubernetes Secret keeps the key out of `docker inspect` output and the process environment. The startup log records which source was used as `auth_key_source` (`file`, `env` or `flag`).
