---
name: "docker-prune"
version: "1.0.0"
description: "Removes unused Docker resources including stopped containers, dangling images, and unused networks"
author: "Charon Project"
license: "MIT"
tags:
  - "docker"
  - "cleanup"
  - "maintenance"
  - "disk-space"
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
environment_variables: []
parameters: []
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 on success, non-zero on failure"
  - name: "reclaimed_space"
    type: "string"
    description: "Amount of disk space freed"
metadata:
  category: "docker"
  subcategory: "maintenance"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: false
  requires_network: false
  idempotent: true
---

# Docker: Prune Unused Resources

## Overview

Removes unused Docker resources to free up disk space and clean up the Docker environment. This includes stopped containers, dangling images, unused networks, and build cache. The operation is safe and only removes resources not currently in use.

## Prerequisites

- Docker Engine installed and running
- Sufficient permissions to run Docker commands
- No critical containers running (verify first)

## Usage

### Basic Usage

```bash
.github/skills/docker-prune-scripts/run.sh
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh docker-prune
```

### Via VS Code Task

Use the task: **Docker: Prune Unused Resources**

## Parameters

This skill uses Docker's default prune behavior (safe mode). No parameters accepted.

## Environment Variables

This skill requires no environment variables.

## Outputs

- **Success Exit Code**: 0
- **Error Exit Codes**: Non-zero on failure
- **Console Output**: List of removed resources and space reclaimed

### Output Example

```
Deleted Containers:
f8d1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

Deleted Networks:
charon-test_default
old-network_default

Deleted Images:
untagged: myimage@sha256:abcdef1234567890...
deleted: sha256:1234567890abcdef...

Deleted build cache objects:
abcd1234
efgh5678

Total reclaimed space: 2.5GB
```

## What Gets Removed

The `docker system prune -f` command removes:

1. **Stopped Containers**: Containers not currently running
2. **Dangling Images**: Images with no tag (intermediate layers)
3. **Unused Networks**: Networks with no connected containers
4. **Build Cache**: Cached layers from image builds

## What Gets Preserved

This command **DOES NOT** remove:
- **Running Containers**: Active containers are untouched
- **Tagged Images**: Images with tags are preserved
- **Volumes**: Data volumes are never removed
- **Used Networks**: Networks with connected containers
- **Active Build Cache**: Cache for recent builds

## Safety Features

- **Force Flag (`-f`)**: Skips confirmation prompt (safe for automation)
- **Safe by Default**: Only removes truly unused resources
- **Volume Protection**: Volumes require separate `docker volume prune` command
- **Running Container Protection**: Cannot remove active containers

## Examples

### Example 1: Regular Cleanup

```bash
# Clean up Docker environment
.github/skills/docker-prune-scripts/run.sh
```

### Example 2: Check Disk Usage Before/After

```bash
# Check current usage
docker system df

# Run cleanup
.github/skills/docker-prune-scripts/run.sh

# Verify freed space
docker system df
```

### Example 3: Aggressive Cleanup (Manual)

```bash
# Standard prune
.github/skills/docker-prune-scripts/run.sh

# Additionally prune volumes (WARNING: data loss)
docker volume prune -f

# Remove all unused images (not just dangling)
docker image prune -a -f
```

## Disk Space Analysis

Check Docker disk usage:

```bash
# Summary view
docker system df

# Detailed view
docker system df -v
```

Output shows:
- **Images**: Total size of cached images
- **Containers**: Size of container writable layers
- **Local Volumes**: Size of data volumes
- **Build Cache**: Size of cached build layers

## When to Use This Skill

Use this skill when:
- Disk space is running low
- After development cycles (many builds)
- After running integration tests
- Before system backup/snapshot
- As part of regular maintenance
- After Docker image experiments

## Frequency Recommendations

- **Daily**: For active development machines
- **Weekly**: For CI/CD build servers
- **Monthly**: For production servers (cautiously)
- **On-Demand**: When disk space is low

## Error Handling

Common issues and solutions:

### Permission Denied
```
Error: permission denied
```
Solution: Add user to docker group or use sudo

### Daemon Not Running
```
Error: Cannot connect to Docker daemon
```
Solution: Start Docker service

### Resource in Use
```
Error: resource is in use
```
This is normal - only unused resources are removed

## Advanced Cleanup Options

For more aggressive cleanup:

### Remove All Unused Images

```bash
docker image prune -a -f
```

### Remove Unused Volumes (DANGER: Data Loss)

```bash
docker volume prune -f
```

### Complete System Prune (DANGER)

```bash
docker system prune -a --volumes -f
```

## Related Skills

- [docker-stop-dev](./docker-stop-dev.SKILL.md) - Stop containers before cleanup
- [docker-start-dev](./docker-start-dev.SKILL.md) - Restart after cleanup
- [utility-clear-go-cache](./utility-clear-go-cache.SKILL.md) - Clear Go build cache

## Notes

- **Idempotent**: Safe to run multiple times
- **Low Risk**: Only removes unused resources
- **No Data Loss**: Volumes are protected by default
- **Fast Execution**: Typically completes in seconds
- **No Network Required**: Local operation only
- **Not CI/CD Safe**: Can interfere with parallel builds
- **Build Cache**: May slow down next build if cache is cleared

## Disk Space Recovery

Typical space recovery by resource type:
- **Stopped Containers**: 10-100 MB each
- **Dangling Images**: 100 MB - 2 GB total
- **Build Cache**: 1-10 GB (if many builds)
- **Unused Networks**: Negligible space

## Troubleshooting

### No Space Freed

- Check for running containers: `docker ps`
- Verify images are untagged: `docker images -f "dangling=true"`
- Check volume usage: `docker volume ls`

### Space Still Low After Prune

- Use aggressive pruning (see Advanced Cleanup)
- Check non-Docker disk usage: `df -h`
- Consider increasing disk allocation

### Container Won't Be Removed

- Check if container is running: `docker ps`
- Stop container first: `docker stop container_name`
- Force removal: `docker rm -f container_name`

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Docker Command**: `docker system prune -f`
