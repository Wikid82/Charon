---
# agentskills.io specification v1.0
name: "docker-rebuild-e2e"
version: "1.0.0"
description: "Rebuild Docker image and restart E2E Playwright container with fresh code and clean state"
author: "Charon Project"
license: "MIT"
tags:
  - "docker"
  - "e2e"
  - "playwright"
  - "rebuild"
  - "testing"
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
environment_variables:
  - name: "DOCKER_NO_CACHE"
    description: "Set to 'true' to force a complete rebuild without cache"
    default: "false"
    required: false
  - name: "SKIP_VOLUME_CLEANUP"
    description: "Set to 'true' to preserve test data volumes"
    default: "false"
    required: false
parameters:
  - name: "no-cache"
    type: "boolean"
    description: "Force rebuild without Docker cache"
    default: "false"
    required: false
  - name: "clean"
    type: "boolean"
    description: "Remove test volumes for a completely fresh state"
    default: "false"
    required: false
  - name: "profile"
    type: "string"
    description: "Docker Compose profile to enable (security-tests, notification-tests)"
    default: ""
    required: false
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 on success, non-zero on failure"
metadata:
  category: "docker"
  subcategory: "e2e"
  execution_time: "long"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Docker: Rebuild E2E Environment

## Overview

Rebuilds the Charon Docker image and restarts the Playwright E2E testing environment with fresh code. This skill handles the complete lifecycle: stopping existing containers, optionally cleaning volumes, rebuilding the image, and starting fresh containers with health check verification.

**Use this skill when:**
- You've made code changes and need to test them in E2E tests
- E2E tests are failing due to stale container state
- You need a clean slate for debugging
- The container is in an inconsistent state

## Prerequisites

- Docker Engine installed and running
- Docker Compose V2 installed
- Dockerfile in repository root
  - `.docker/compose/docker-compose.playwright-ci.yml` file (used in CI)
- Network access for pulling base images (if needed)
- Sufficient disk space for image rebuild

## Usage

### Basic Usage

Rebuild image and restart E2E container:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

### Force Rebuild (No Cache)

Rebuild from scratch without Docker cache:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --no-cache
```

### Clean Rebuild

Remove test volumes and rebuild with fresh state:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
```

### With Security Testing Services

Enable CrowdSec for security testing:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --profile=security-tests
```

### With Notification Testing Services

Enable MailHog for email testing:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --profile=notification-tests
```

### Full Clean Rebuild with All Services

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --no-cache --clean --profile=security-tests
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| no-cache  | boolean | No | false | Force rebuild without Docker cache |
| clean     | boolean | No | false | Remove test volumes for fresh state |
| profile   | string | No | "" | Docker Compose profile to enable |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| DOCKER_NO_CACHE | No | false | Force rebuild without cache |
| SKIP_VOLUME_CLEANUP | No | false | Preserve test data volumes |

## What This Skill Does

1. **Stop Existing Containers**: Gracefully stops any running Playwright containers
2. **Clean Volumes** (if `--clean`): Removes test data volumes for fresh state
3. **Rebuild Image**: Builds `charon:local` image from Dockerfile
4. **Start Containers**: Starts the Playwright compose environment
5. **Wait for Health**: Verifies container health before returning
6. **Report Status**: Outputs container status and connection info

## Docker Compose Configuration

This skill uses `.docker/compose/docker-compose.playwright-ci.yml` which includes:

- **charon-app**: Main application container on port 8080
- **crowdsec** (profile: security-tests): Security bouncer for WAF testing
- **mailhog** (profile: notification-tests): Email testing service

### Volumes Created

| Volume | Purpose |
|--------|---------|
| playwright_data | Application data and SQLite database |
| playwright_caddy_data | Caddy server data |
| playwright_caddy_config | Caddy configuration |
| playwright_crowdsec_data | CrowdSec data (if enabled) |
| playwright_crowdsec_config | CrowdSec config (if enabled) |

## Examples

### Example 1: Quick Rebuild After Code Change

```bash
# Rebuild and restart after making backend changes
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Run E2E tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

### Example 2: Debug Failing Tests with Clean State

```bash
# Complete clean rebuild
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean --no-cache

# Run specific test in debug mode
.github/skills/scripts/skill-runner.sh test-e2e-playwright-debug --grep="failing-test"
```

### Example 3: Test Security Features

```bash
# Start with CrowdSec enabled
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --profile=security-tests

# Run security-related E2E tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright --grep="security"
```

## Health Check Verification

After starting, the skill waits for the health check to pass:

```bash
# Health endpoint checked
curl -sf http://localhost:8080/api/v1/health
```

The skill will:
- Wait up to 60 seconds for container to be healthy
- Check every 5 seconds
- Report final health status
- Exit with error if health check fails

## Error Handling

### Common Issues

#### Docker Build Failed
```
Error: docker build failed
```
**Solution**: Check Dockerfile syntax, ensure all COPY sources exist

#### Port Already in Use
```
Error: bind: address already in use
```
**Solution**: Stop conflicting services on port 8080

#### Health Check Timeout
```
Error: Container did not become healthy in 60s
```
**Solution**: Check container logs with `docker logs charon-playwright`

#### Volume Permission Issues
```
Error: permission denied
```
**Solution**: Run with `--clean` to recreate volumes with proper permissions

## Verifying the Environment

After the skill completes, verify the environment:

```bash
# Check container status
docker ps --filter "name=charon-playwright"

# Check logs
docker logs charon-playwright --tail 50

# Test health endpoint
curl http://localhost:8080/api/v1/health

# Check database state
docker exec charon-playwright sqlite3 /app/data/charon.db ".tables"
```

## Related Skills

- [test-e2e-playwright](./test-e2e-playwright.SKILL.md) - Run E2E tests
- [test-e2e-playwright-debug](./test-e2e-playwright-debug.SKILL.md) - Debug E2E tests
- [docker-start-dev](./docker-start-dev.SKILL.md) - Start development environment
- [docker-stop-dev](./docker-stop-dev.SKILL.md) - Stop development environment
- [docker-prune](./docker-prune.SKILL.md) - Clean up Docker resources

## Key File Locations

| File | Purpose |
|------|---------|
| `Dockerfile` | Main application Dockerfile |
| `.docker/compose/docker-compose.playwright-ci.yml` | CI E2E test compose config |
| `.docker/compose/docker-compose.playwright-local.yml` | Local E2E test compose config |
| `playwright.config.js` | Playwright test configuration |
| `tests/` | E2E test files |
| `playwright/.auth/user.json` | Stored authentication state |

## Notes

- **Build Time**: Full rebuild takes 2-5 minutes depending on cache
- **Disk Space**: Image is ~500MB, volumes add ~100MB
- **Network**: Base images may need to be pulled on first run
- **Idempotent**: Safe to run multiple times
- **CI/CD Safe**: Designed for use in automated pipelines

---

**Last Updated**: 2026-01-27
**Maintained by**: Charon Project Team
**Compose Files**:
- CI: `.docker/compose/docker-compose.playwright-ci.yml` (uses GitHub Secrets, no .env)
- Local: `.docker/compose/docker-compose.playwright-local.yml` (uses .env file)
