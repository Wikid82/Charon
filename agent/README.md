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

### Environment Variables (recommended)

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

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `ORTHRUS_SERVER_URL` | — | WebSocket URL of the Charon Orthrus endpoint (`wss://...` behind HTTPS, `ws://...` for direct HTTP) |
| `ORTHRUS_AUTH_KEY` | — | Auth key from the Charon provisioning response (`ch_orthrus_...`) |
| `ORTHRUS_DOCKER_SOCKET` | `/var/run/docker.sock` | Path to the local Docker socket |
| `PUID` | `65532` | User ID the agent runs as. Must not be `0` (root) unless `ORTHRUS_ALLOW_ROOT=true` is set |
| `PGID` | `65532` | Group ID the agent runs as. Set it to the Docker socket's group (`stat -c '%g' /var/run/docker.sock`) so the agent can read the socket. Must not be `0` unless `ORTHRUS_ALLOW_ROOT=true` is set |
| `ORTHRUS_ALLOW_ROOT` | `false` | Set to `true` to allow `PUID` or `PGID` of `0` (root). Not recommended: a root-level agent has full control of the host if compromised. Logs a loud warning at startup |
| `TZ` | `UTC` | Timezone used for log timestamps (e.g. `America/New_York`) |

## CLI Flags

All environment variables have equivalent CLI flags:

```text
--server-url   wss://charon.example.com/api/v1/ws/orthrus/connect
--auth-key     ch_orthrus_<key>     (prefer ORTHRUS_AUTH_KEY env var)
--agent-id     <uuid>               (from provisioning response)
--docker-socket /var/run/docker.sock
--log-level    info
```

Environment variables take precedence over flags.

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

**Auth key**: Never logged. The key is read from the `ORTHRUS_AUTH_KEY` environment variable and only appears in logs as `[REDACTED]`.
