# Docker Compose Files

This directory contains all Docker Compose configuration variants for Charon.

## File Descriptions

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Main production compose configuration. Base services and production settings. |
| `docker-compose.dev.yml` | Development overrides. Enables hot-reload, debug logging, and development tools. |
| `docker-compose.local.yml` | Local development configuration. Standalone setup for local testing. |
| `docker-compose.remote.yml` | Remote deployment configuration. Settings for deploying to remote servers. |
| `docker-compose.override.yml` | Personal local overrides. **Gitignored** - use for machine-specific settings. |

## Usage Patterns

### Production Deployment

```bash
docker compose -f .docker/compose/docker-compose.yml up -d
```

### Development Mode

```bash
docker compose -f .docker/compose/docker-compose.yml \
               -f .docker/compose/docker-compose.dev.yml up -d
```

### Local Testing

```bash
docker compose -f .docker/compose/docker-compose.local.yml up -d
```

### With Personal Overrides

Create your own `docker-compose.override.yml` in this directory for personal
configurations (port mappings, volume paths, etc.). This file is gitignored.

```bash
docker compose -f .docker/compose/docker-compose.yml \
               -f .docker/compose/docker-compose.override.yml up -d
```

## Notes

- Always use the `-f` flag to specify compose file paths from the project root
- The override file is automatically ignored by git - do not commit personal settings
- See project tasks in VS Code for convenient pre-configured commands
