---
title: Docker Auto-Discovery
description: Automatically find and proxy Docker containers with one click
category: integration
---

# Docker Auto-Discovery

Already running apps in Docker? Charon automatically finds your containers and offers one-click proxy setup. Works with Docker on the same machine or on other machines. The recommended way to connect is [Orthrus](orthrus.md); mounting the Docker socket directly is still supported.

## Overview

Docker auto-discovery eliminates manual IP address hunting and port memorization. Charon queries the Docker API to list running containers, extracts their network information, and lets you create proxy configurations with a single click.

### How It Works

1. Charon connects to Docker through an Orthrus agent, a mounted socket, or a socket proxy
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

Charon needs a way to ask Docker "what is running?". There are three ways to give it that, listed from most to least recommended for security.

### Recommended: Orthrus

Orthrus is a tiny helper that sits next to Docker and answers Charon's questions. Because the helper holds the Docker socket, **Charon itself never touches it**. If someone ever broke into Charon, they would still not have a direct line to your whole Docker setup.

Orthrus works on a different machine and on the same machine as Charon.

1. In Charon, open **Hecate** → **Agent** (admins only) and add an agent. Copy the one-time key.
2. Run the ready-made install snippet next to your containers.
3. When the agent shows **Online**, add a Source for it and pick it in the **Source** dropdown of the Proxy Host form.

New to this? Follow the [Orthrus guide](orthrus.md), including [Using Orthrus on the same server as Charon](orthrus.md#using-orthrus-on-the-same-server-as-charon).

### Simple: Mount the Docker Socket Directly

Mounting the socket straight into Charon is still fully supported. It is the quickest way to get started because there is no extra container to run.

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

> **Security Note**: The socket grants significant access, and the `:ro` flag does not change that. If you prefer this method, consider the socket proxy below. If you would rather not mount the socket at all, use Orthrus.

### Hardened Middle Ground: Socket Proxy

A socket proxy lets Charon see your containers while blocking everything else. It is a good fit if you want to keep the direct approach but with fewer risks. See [Limiting Socket Access with a Proxy](#limiting-socket-access-with-a-proxy) below.

### When the Local Option Is Greyed Out

In the Proxy Host form, the **Source** dropdown has an option called **Local (Docker Socket)**. If Charon cannot use the Docker socket (for example, none is mounted), that option is greyed out and a short message explains why. Nothing is broken: pick an Orthrus Source instead, or mount the socket if you prefer the simple method.

### Connecting to Docker on Another Machine

For a Docker machine other than the one Charon runs on, the easiest and safest choice is Orthrus (see [Remote Docker Setup](../guides/remote-docker-setup.md)). You can also add a Source that points straight at the machine's Docker address:

1. In the sidebar, open **Hecate** → **Sources**
2. Click **Add Source**
3. Enter a **Name**, set **Provider** to **Docker**, and fill in the host and port
4. Click **Save**

> **Heads up**: Docker's network port is unencrypted unless you set up TLS on that machine. Keep it on a private network only.

## Container Selection Workflow

### Viewing Available Containers

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Open the **Source** dropdown and choose where your containers live (**Local (Docker Socket)** or one of your Sources)
3. Pick a container from the list
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
| ------- | ------- | ---------- |
| No containers shown | No Docker connection set up | Connect Docker with [Orthrus](orthrus.md) (recommended), or mount the Docker socket |
| "Local (Docker Socket)" is greyed out in the Source dropdown | No Docker socket is mounted, or Charon cannot read it | Use an Orthrus Source, or mount the socket (and see [Getting Started](../getting-started.md#docker-socket-access-important) if Charon cannot read it) |
| Connection refused | Remote Docker not configured | Enable TCP API on Docker host |
| Container not proxied | Container not running | Start the container |
| Wrong IP resolved | Multi-network container | Specify network in advanced settings |
| Socket proxy not reachable | DOCKER_HOST misconfigured | Verify socket-proxy container is on the same network and DOCKER_HOST matches the service name |

## Security Considerations

- **Socket Access**: The Docker socket grants broad system access. The `:ro` flag stops the socket file from being deleted, but does **not** limit which Docker actions can be taken. The safest choice is [Orthrus](orthrus.md), so Charon never holds the socket. If you mount it directly, that is still supported; a socket proxy (below) narrows what is allowed.
- **Remote Connections**: Prefer Orthrus for other machines. If you connect straight to a remote Docker address, keep it on a private network and use TLS.
- **Network Isolation**: Use Docker networks to segment container communication.

### Limiting Socket Access with a Proxy

A Docker socket proxy sits between Charon and the Docker daemon, filtering API calls so only the endpoints Charon actually needs are reachable. Charon only uses `GET /containers/*` to list containers, so you can lock everything else down.

**Recommended proxy:** `lscr.io/linuxserver/socket-proxy`

```yaml
services:
  socket-proxy:
    image: lscr.io/linuxserver/socket-proxy:latest
    container_name: socket-proxy
    restart: unless-stopped
    environment:
      - CONTAINERS=1   # Allow container listing (required by Charon)
      - POST=0         # Deny all write operations
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - proxy-internal

  charon:
    image: ghcr.io/wikid82/charon:latest
    environment:
      - DOCKER_HOST=tcp://socket-proxy:2375
    # No docker.sock volume needed!
    networks:
      - proxy-internal
      - ...

networks:
  proxy-internal:
    internal: true
```

With this setup, Charon talks to the proxy instead of the raw Docker socket. The proxy only answers container listing requests — everything else is blocked.

## Related

- [Web UI](web-ui.md) - Point & click management
- [SSL Certificates](ssl-certificates.md) - Automatic HTTPS for proxied containers
- [Using Charon with Podman](../guides/podman-usage.md) - Connect discovery to a Podman socket instead of Docker's
- [Back to Features](../features.md)
