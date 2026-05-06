# Orthrus Agent

Orthrus is a lightweight reverse-proxy agent that dials out to a [Charon](https://github.com/Wikid82/Charon) server over a secure WebSocket connection. It allows Charon to route traffic to remote services that are not directly reachable via a public IP — without requiring firewall port-forwarding.

Once connected, Charon multiplexes Docker socket and TCP port-forward streams back to the host running the agent using [yamux](https://github.com/hashicorp/yamux).

## Quick Start

### Docker

```bash
docker run ghcr.io/wikid82/charon-orthrus-agent \
  --server-url wss://charon.example.com/api/v1/ws/orthrus/connect \
  --auth-key ch_orthrus_<your-key-here>
```

### Environment Variables (recommended)

```bash
docker run \
  -e ORTHRUS_SERVER_URL=wss://charon.example.com/api/v1/ws/orthrus/connect \
  -e ORTHRUS_AUTH_KEY=ch_orthrus_<your-key-here> \
  -e ORTHRUS_DOCKER_SOCKET=/var/run/docker.sock \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/wikid82/charon-orthrus-agent
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `ORTHRUS_SERVER_URL` | — | WebSocket URL of the Charon Orthrus endpoint (`wss://...`) |
| `ORTHRUS_AUTH_KEY` | — | Auth key from the Charon provisioning response (`ch_orthrus_...`) |
| `ORTHRUS_DOCKER_SOCKET` | `/var/run/docker.sock` | Path to the local Docker socket |

## CLI Flags

All environment variables have equivalent CLI flags:

```
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

**TLS**: All WebSocket connections must use `wss://`. Only `ws://localhost` is permitted for local development.

**Auth key**: Never logged. The key is read from the `ORTHRUS_AUTH_KEY` environment variable and only appears in logs as `[REDACTED]`.
