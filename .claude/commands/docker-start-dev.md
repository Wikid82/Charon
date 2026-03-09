# Docker: Start Dev Environment

Start the Charon development Docker Compose environment with all required services.

## Command

```bash
.github/skills/scripts/skill-runner.sh docker-start-dev
```

## What Gets Started

Services in `.docker/compose/docker-compose.dev.yml`:
1. **charon-app** — Main application container
2. **charon-db** — SQLite/database
3. **crowdsec** — Security bouncer
4. **caddy** — Reverse proxy

## Default Ports

- `8080` — Application HTTP
- `2020` — Emergency access
- `2019` — Caddy admin API

## Verify Healthy

```bash
docker compose -f .docker/compose/docker-compose.dev.yml ps
curl http://localhost:8080/health
```

## Common Issues

| Error | Solution |
|-------|----------|
| `address already in use` | Stop conflicting service or change port |
| `failed to pull image` | Check network, authenticate to registry |
| `invalid compose file` | `docker compose -f .docker/compose/docker-compose.dev.yml config` |

## Related

- `/docker-stop-dev` — Stop the environment
- `/docker-rebuild-e2e` — Rebuild the E2E test container
- `/docker-prune` — Clean up Docker resources
