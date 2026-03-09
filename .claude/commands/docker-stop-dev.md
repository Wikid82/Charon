# Docker: Stop Dev Environment

Stop the Charon development Docker Compose environment.

## Command

```bash
.github/skills/scripts/skill-runner.sh docker-stop-dev
```

## What It Does

Stops all services defined in `.docker/compose/docker-compose.dev.yml` gracefully.

**Data persistence**: Volumes are preserved — your data is safe.

## Verify Stopped

```bash
docker compose -f .docker/compose/docker-compose.dev.yml ps
```

## Related

- `/docker-start-dev` — Start the environment
- `/docker-prune` — Clean up Docker resources (removes volumes too — use with caution)
