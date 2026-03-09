# Docker: Rebuild E2E Container

Rebuild the Charon E2E test container with the latest application code.

## When to Run

**Rebuild required** when:
- Application code changed
- Docker build inputs changed (Dockerfile, .env, dependencies)

**Skip rebuild** when:
- Only test files changed and the container is already healthy

## Command

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

## What It Does

Rebuilds the E2E container to include:
- Latest application code
- Current environment variables (emergency token, encryption key from `.env`)
- All Docker build dependencies

## Verify Healthy

After rebuild, confirm the container is ready:

```bash
docker compose -f .docker/compose/docker-compose.e2e.yml ps
curl http://localhost:8080/health
```

## Run E2E Tests After Rebuild

```bash
cd /projects/Charon && npx playwright test --project=firefox
```

## Related

- `/docker-start-dev` — Start development environment
- `/test-e2e-playwright` — Run E2E Playwright tests
