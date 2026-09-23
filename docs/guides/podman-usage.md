---
title: Using Charon with Podman
description: Run Charon's container behind Podman instead of Docker, and connect Docker Auto-Discovery to a Podman socket
category: guides
---

# Using Charon with Podman

Charon ships as a container image and runs fine under Podman — for the reverse proxy itself, Podman is a true drop-in replacement for Docker. The only place the two engines aren't interchangeable out of the box is [Docker Auto-Discovery](../features/docker-integration.md), the optional feature that lists containers on the host so you can proxy them with one click. That feature talks to a container engine's socket, and Podman puts its socket somewhere different from Docker by default.

This guide covers what to change to get Auto-Discovery working under Podman. If you don't use Auto-Discovery, you can skip straight to running the image — nothing else in Charon depends on Docker or Podman.

## Why It Isn't Fully Automatic

Charon's Docker client looks for a socket in exactly two places, in order:

1. Whatever `DOCKER_HOST` is set to, **if** it's a `unix://` path that exists (non-`unix://` values, like a bare TCP URL, are ignored for local discovery)
2. `/var/run/docker.sock`
3. `/run/user/<uid>/docker.sock` (rootless Docker's default path)

Podman's default socket paths don't match any of these:

- Rootful: `/run/podman/podman.sock`
- Rootless: `/run/user/<uid>/podman/podman.sock`

Podman's socket speaks the same Docker-compatible REST API that Charon's client uses (`containers/json`, `images/json`, `info`, `version`, `events`, `volumes`, `networks`, container `json`/`logs`/`stats`/`top`), so once Charon can find the socket, discovery works exactly as it does with Docker.

## Setup

### 1. Enable the Podman socket

Rootless (recommended — runs as your own user, no elevated privileges):

```bash
systemctl --user enable --now podman.socket
```

Rootful, if you specifically need it:

```bash
sudo systemctl enable --now podman.socket
```

### 2. Point Charon at the socket

Pick one of the two approaches below.

**Option A — bind-mount it at the path Charon already expects.** This is the least invasive change if you're reusing the repo's `.docker/compose/docker-compose.yml`, since it only changes the volume *source*, not the container's environment:

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    volumes:
      - /run/user/1000/podman/podman.sock:/var/run/docker.sock:ro # swap 1000 for your uid
```

**Option B — mount the socket wherever you like and tell Charon explicitly:**

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    environment:
      - DOCKER_HOST=unix:///var/run/podman.sock
    volumes:
      - /run/user/1000/podman/podman.sock:/var/run/podman.sock:ro
```

Either way, run the stack with `podman-compose` (or generate a systemd unit with `podman generate systemd`) instead of `docker compose` — the image and compose file are unchanged.

### 3. SELinux hosts (Fedora, RHEL, CentOS)

If SELinux is enforcing, add the `:z` volume label so the container is allowed to read the socket:

```yaml
    volumes:
      - /run/user/1000/podman/podman.sock:/var/run/docker.sock:ro,z
```

### 4. Confirm it's working

Open **Hosts → Add Host → Select from Docker** in Charon. If containers show up, discovery is wired correctly. If not, see [Docker Auto-Discovery's troubleshooting table](../features/docker-integration.md#troubleshooting) — the same causes (socket not mounted, wrong permissions) apply under Podman.

## Remote Podman Hosts

The socket-mount approach above is for a Podman engine on the same host as Charon. To manage containers on a *separate* Podman machine, use [Remote Agents](remote-docker-setup.md) instead — the Orthrus agent runs on the remote box and talks to whatever local engine is there (Docker or Podman), so the same socket-path considerations from this guide apply when installing the agent itself.

## A Note on Third-Party Install Scripts

You may come across community scripts that automate a Podman-based Charon install. Treat them the same as any third-party script — read them before running, and check what image and registries they pull from. Charon's own official image and compose files (this repo's `Dockerfile` and `.docker/compose/docker-compose.yml`) work under Podman with the changes above; you don't need a separate unofficial packaging to get there.

## Related

- [Docker Auto-Discovery](../features/docker-integration.md) — the feature this guide configures
- [Connecting a Remote Docker Host](remote-docker-setup.md) — for engines on a different machine
