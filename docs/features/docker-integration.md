---
title: Docker Auto-Discovery
description: Automatically find and proxy Docker containers with one click
category: integration
---

# Docker Auto-Discovery

Already running apps in Docker? Charon automatically finds your containers and offers one-click proxy setup. Supports both local Docker installations and remote Docker servers.

## Overview

Docker auto-discovery eliminates manual IP address hunting and port memorization. Charon queries the Docker API to list running containers, extracts their network information, and lets you create proxy configurations with a single click.

### How It Works

1. Charon connects to Docker via socket or TCP
2. Queries running containers and their exposed ports
3. Displays container list with network details
4. You select a container and assign a domain
5. Charon creates the proxy configuration automatically

## Why Use This

### Eliminate IP Address Hunting

- No more running `docker inspect` to find container IPs
- No more updating configs when containers restart with new IPs
- Container name resolution handles dynamic addressing

### Accelerate Development

- Spin up a new service, proxy it in seconds
- Test different versions by proxying multiple containers
- Remove proxies as easily as you create them

### Simplify Team Workflows

- Developers create their own proxy entries
- No central config file bottlenecks
- Self-service infrastructure access

## Configuration

### Docker Socket Mounting

For Charon to discover containers, it needs Docker API access.

**Docker Compose:**

```yaml
services:
  charon:
    image: charon:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

**Docker Run:**

```bash
docker run -v /var/run/docker.sock:/var/run/docker.sock:ro charon
```

> **Security Note**: The socket grants significant access. Use read-only mode (`:ro`) and consider Docker socket proxies for production.

### Remote Docker Server Support

Connect to Docker hosts over TCP:

1. Go to **Settings** → **Docker**
2. Click **Add Remote Host**
3. Enter connection details:
   - **Name**: Friendly identifier
   - **Host**: IP or hostname
   - **Port**: Docker API port (default: 2375/2376)
   - **TLS**: Enable for secure connections
4. Upload TLS certificates if required
5. Click **Test Connection**, then **Save**

## Container Selection Workflow

### Viewing Available Containers

1. Navigate to **Hosts** → **Add Host**
2. Click **Select from Docker**
3. Choose Docker host (local or remote)
4. Browse running containers

### Container List Display

Each container shows:

- **Name**: Container name
- **Image**: Source image and tag
- **Ports**: Exposed ports and mappings
- **Networks**: Connected Docker networks
- **Status**: Running, paused, etc.

### Creating a Proxy

1. Click a container row to select it
2. If multiple ports are exposed, choose the target port
3. Enter the domain name for this proxy
4. Configure SSL options
5. Click **Create Host**

### Automatic Updates

When containers restart:

- Charon continues proxying to the container name
- Docker's internal DNS resolves the new IP
- No manual intervention required

## Advanced Configuration

### Network Selection

If a container is on multiple networks, specify which network Charon should use for routing:

1. Edit the host after creation
2. Go to **Advanced** → **Docker**
3. Select the preferred network

### Port Override

Override the auto-detected port:

1. Edit the host
2. Change the backend URL port manually
3. Useful for containers with non-standard port configurations

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| No containers shown | Socket not mounted | Add Docker socket volume |
| Connection refused | Remote Docker not configured | Enable TCP API on Docker host |
| Container not proxied | Container not running | Start the container |
| Wrong IP resolved | Multi-network container | Specify network in advanced settings |

## Security Considerations

- **Socket Access**: Docker socket provides root-equivalent access. Mount read-only.
- **Remote Connections**: Always use TLS for remote Docker hosts.
- **Network Isolation**: Use Docker networks to segment container communication.

## Related

- [Web UI](web-ui.md) - Point & click management
- [SSL Certificates](ssl-certificates.md) - Automatic HTTPS for proxied containers
- [Back to Features](../features.md)
