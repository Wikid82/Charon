---
name: "docker-start-dev"
version: "1.0.0"
description: "Starts the Charon development Docker Compose environment with all required services"
author: "Charon Project"
license: "MIT"
tags:
  - "docker"
  - "development"
  - "compose"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "docker"
    version: ">=24.0"
    optional: false
  - name: "docker-compose"
    version: ">=2.0"
    optional: false
environment_variables: []
parameters: []
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 on success, non-zero on failure"
metadata:
  category: "docker"
  subcategory: "environment"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: false
  requires_network: true
  idempotent: true
---

# Docker: Start Development Environment

## Overview

Starts the Charon development Docker Compose environment in detached mode. This brings up all required services including the application, database, CrowdSec, and any other dependencies defined in `.docker/compose/docker-compose.dev.yml`.

## Prerequisites

- Docker Engine installed and running
- Docker Compose V2 installed
- `.docker/compose/docker-compose.dev.yml` file in repository
- Network access (for pulling images)
- Sufficient system resources (CPU, memory, disk)

## Usage

### Basic Usage

```bash
.github/skills/docker-start-dev-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh docker-start-dev
```

### Via VS Code Task

Use the task: **Docker: Start Dev Environment**

## Parameters

This skill accepts no parameters. Services are configured in `.docker/compose/docker-compose.dev.yml`.

## Environment Variables

This skill uses environment variables defined in:
- `.env` (if present)
- `.docker/compose/docker-compose.dev.yml` environment section
- Shell environment

## Outputs

- **Success Exit Code**: 0 - All services started successfully
- **Error Exit Codes**: Non-zero - Service startup failed
- **Console Output**: Docker Compose logs and status

### Output Example

```
[+] Running 5/5
 ✔ Network charon-dev_default        Created
 ✔ Container charon-dev-db-1         Started
 ✔ Container charon-dev-crowdsec-1   Started
 ✔ Container charon-dev-app-1        Started
 ✔ Container charon-dev-caddy-1      Started
```

## What Gets Started

Services defined in `.docker/compose/docker-compose.dev.yml`:
1. **charon-app**: Main application container
2. **charon-db**: SQLite or PostgreSQL database
3. **crowdsec**: Security bouncer
4. **caddy**: Reverse proxy (if configured)
5. **Other Services**: As defined in compose file

## Service Startup Order

Docker Compose respects `depends_on` directives:
1. Database services start first
2. Security services (CrowdSec) start next
3. Application services start after dependencies
4. Reverse proxy starts last

## Examples

### Example 1: Start Development Environment

```bash
# Start all development services
.github/skills/docker-start-dev-scripts/run.sh

# Verify services are running
docker compose -f .docker/compose/docker-compose.dev.yml ps
```

### Example 2: Start and View Logs

```bash
# Start services in detached mode
.github/skills/docker-start-dev-scripts/run.sh

# Follow logs from all services
docker compose -f .docker/compose/docker-compose.dev.yml logs -f
```

### Example 3: Start and Test Application

```bash
# Start development environment
.github/skills/docker-start-dev-scripts/run.sh

# Wait for services to be healthy
sleep 10

# Test application endpoint
curl http://localhost:8080/health
```

## Service Health Checks

After starting, verify services are healthy:

```bash
# Check service status
docker compose -f .docker/compose/docker-compose.dev.yml ps

# Check specific service logs
docker compose -f .docker/compose/docker-compose.dev.yml logs app

# Execute command in running container
docker compose -f .docker/compose/docker-compose.dev.yml exec app /bin/sh
```

## Port Mappings

Default development ports (check `.docker/compose/docker-compose.dev.yml`):
- **8080**: Application HTTP
- **8443**: Application HTTPS (if configured)
- **9000**: Admin panel (if configured)
- **3000**: Frontend dev server (if configured)

## Detached Mode

The `-d` flag runs containers in detached mode:
- Services run in background
- Terminal is freed for other commands
- Use `docker compose logs -f` to view output

## Error Handling

Common issues and solutions:

### Port Already in Use
```
Error: bind: address already in use
```
Solution: Stop conflicting service or change port in compose file

### Image Pull Failed
```
Error: failed to pull image
```
Solution: Check network connection, authenticate to registry

### Insufficient Resources
```
Error: failed to start container
```
Solution: Free up system resources, stop other containers

### Configuration Error
```
Error: invalid compose file
```
Solution: Validate compose file with `docker compose config`

## Post-Startup Verification

After starting, verify:

1. **All Services Running**:
   ```bash
   docker compose -f .docker/compose/docker-compose.dev.yml ps
   ```

2. **Application Accessible**:
   ```bash
   curl http://localhost:8080/health
   ```

3. **No Error Logs**:
   ```bash
   docker compose -f .docker/compose/docker-compose.dev.yml logs --tail=50
   ```

## Related Skills

- [docker-stop-dev](./docker-stop-dev.SKILL.md) - Stop development environment
- [docker-prune](./docker-prune.SKILL.md) - Clean up Docker resources
- [integration-test-all](./integration-test-all.SKILL.md) - Run integration tests

## Notes

- **Idempotent**: Safe to run multiple times (recreates only if needed)
- **Resource Usage**: Development mode may use more resources than production
- **Data Persistence**: Volumes persist data across restarts
- **Network Access**: Requires internet for initial image pulls
- **Not CI/CD Safe**: Intended for local development only
- **Background Execution**: Services run in detached mode

## Troubleshooting

### Services Won't Start

1. Check Docker daemon: `docker info`
2. Validate compose file: `docker compose -f .docker/compose/docker-compose.dev.yml config`
3. Check available resources: `docker stats`
4. Review logs: `docker compose -f .docker/compose/docker-compose.dev.yml logs`

### Slow Startup

- First run pulls images (may take time)
- Subsequent runs use cached images
- Use `docker compose pull` to pre-pull images

### Service Dependency Issues

- Check `depends_on` in compose file
- Add healthchecks for critical services
- Increase startup timeout if needed

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Compose File**: `.docker/compose/docker-compose.dev.yml`
