---
name: "docker-stop-dev"
version: "1.0.0"
description: "Stops and removes the Charon development Docker Compose environment and containers"
author: "Charon Project"
license: "MIT"
tags:
  - "docker"
  - "development"
  - "compose"
  - "cleanup"
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
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: false
  requires_network: false
  idempotent: true
---

# Docker: Stop Development Environment

## Overview

Stops and removes all containers defined in the Charon development Docker Compose environment. This gracefully shuts down services, removes containers, and cleans up the default network while preserving volumes and data.

## Prerequisites

- Docker Engine installed and running
- Docker Compose V2 installed
- Development environment previously started
- `.docker/compose/docker-compose.dev.yml` file in repository

## Usage

### Basic Usage

```bash
.github/skills/docker-stop-dev-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh docker-stop-dev
```

### Via VS Code Task

Use the task: **Docker: Stop Dev Environment**

## Parameters

This skill accepts no parameters.

## Environment Variables

This skill requires no environment variables.

## Outputs

- **Success Exit Code**: 0 - All services stopped successfully
- **Error Exit Codes**: Non-zero - Service shutdown failed
- **Console Output**: Docker Compose shutdown status

### Output Example

```
[+] Running 5/5
 ✔ Container charon-dev-caddy-1      Removed
 ✔ Container charon-dev-app-1        Removed
 ✔ Container charon-dev-crowdsec-1   Removed
 ✔ Container charon-dev-db-1         Removed
 ✔ Network charon-dev_default        Removed
```

## What Gets Stopped

Services defined in `.docker/compose/docker-compose.dev.yml`:
1. **Application Containers**: Charon main app
2. **Database Containers**: SQLite/PostgreSQL services
3. **Security Services**: CrowdSec bouncer
4. **Reverse Proxy**: Caddy server
5. **Network**: Default Docker Compose network

## What Gets Preserved

The `down` command preserves:
- **Volumes**: Database data persists
- **Images**: Docker images remain cached
- **Configs**: Configuration files unchanged

To remove volumes, use `docker compose -f .docker/compose/docker-compose.dev.yml down -v`

## Shutdown Order

Docker Compose stops services in reverse dependency order:
1. Reverse proxy stops first
2. Application services stop next
3. Security services stop
4. Database services stop last

## Examples

### Example 1: Stop Development Environment

```bash
# Stop all development services
.github/skills/docker-stop-dev-scripts/run.sh

# Verify services are stopped
docker compose -f .docker/compose/docker-compose.dev.yml ps
```

### Example 2: Stop and Remove Volumes

```bash
# Stop services and remove data volumes
docker compose -f .docker/compose/docker-compose.dev.yml down -v
```

### Example 3: Stop and Verify Cleanup

```bash
# Stop development environment
.github/skills/docker-stop-dev-scripts/run.sh

# Verify no containers running
docker ps --filter "name=charon-dev"

# Verify network removed
docker network ls | grep charon-dev
```

## Graceful Shutdown

The `down` command:
1. Sends `SIGTERM` to each container
2. Waits for graceful shutdown (default: 10 seconds)
3. Sends `SIGKILL` if timeout exceeded
4. Removes stopped containers
5. Removes default network

## When to Use This Skill

Use this skill when:
- Switching between development and production modes
- Freeing system resources (CPU, memory)
- Preparing for system shutdown/restart
- Resetting environment for troubleshooting
- Applying Docker Compose configuration changes
- Before database recovery operations

## Error Handling

Common issues and solutions:

### Container Already Stopped
```
Warning: Container already stopped
```
No action needed - idempotent operation

### Volume in Use
```
Error: volume is in use
```
Solution: Check for other containers using the volume

### Permission Denied
```
Error: permission denied
```
Solution: Add user to docker group or use sudo

## Post-Shutdown Verification

After stopping, verify:

1. **No Running Containers**:
   ```bash
   docker ps --filter "name=charon-dev"
   ```

2. **Network Removed**:
   ```bash
   docker network ls | grep charon-dev
   ```

3. **Volumes Still Exist** (if data preservation intended):
   ```bash
   docker volume ls | grep charon
   ```

## Related Skills

- [docker-start-dev](./docker-start-dev.SKILL.md) - Start development environment
- [docker-prune](./docker-prune.SKILL.md) - Clean up Docker resources
- [utility-db-recovery](./utility-db-recovery.SKILL.md) - Database recovery

## Notes

- **Idempotent**: Safe to run multiple times (no error if already stopped)
- **Data Preservation**: Volumes are NOT removed by default
- **Fast Execution**: Typically completes in seconds
- **No Network Required**: Local operation only
- **Not CI/CD Safe**: Intended for local development only
- **Graceful Shutdown**: Allows containers to clean up resources

## Complete Cleanup

For complete environment reset:

```bash
# Stop and remove containers, networks
.github/skills/docker-stop-dev-scripts/run.sh

# Remove volumes (WARNING: deletes data)
docker volume rm $(docker volume ls -q --filter "name=charon")

# Remove images (optional)
docker rmi $(docker images -q "*charon*")

# Clean up dangling resources
.github/skills/docker-prune-scripts/run.sh
```

## Troubleshooting

### Container Won't Stop

1. Check container logs: `docker compose logs app`
2. Force removal: `docker compose kill`
3. Manual cleanup: `docker rm -f container_name`

### Volume Still in Use

1. List processes: `docker ps -a`
2. Check volume usage: `docker volume inspect volume_name`
3. Force volume removal: `docker volume rm -f volume_name`

### Network Can't Be Removed

1. Check connected containers: `docker network inspect network_name`
2. Disconnect containers: `docker network disconnect network_name container_name`
3. Retry removal: `docker network rm network_name`

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Compose File**: `.docker/compose/docker-compose.dev.yml`
