---
title: Charon Stops at Startup with a Proxy Admin Address Message
description: Fix the startup message about CHARON_CADDY_ADMIN_API that appears after an upgrade.
---

## Charon Stops at Startup with a Proxy Admin Address Message

### What You See

Charon stops right after it starts. The container logs (run `docker logs charon`) show this message:

```text
CHARON_CADDY_ADMIN_API must be a unix:// socket path or point at this container (localhost); using an external proxy is not supported: remove the setting to use the default socket
```

If your container is set to restart automatically, it will keep restarting and show this message each time.

### Why It Happens

Charon always uses the Caddy web server that is built into its own container. It no longer works with a Caddy running on another computer or under another name. The setting `CHARON_CADDY_ADMIN_API` tells Charon where that built-in Caddy's admin API is. The API is a unix socket inside the container, so the only valid setting is the default, which is to leave the setting out.

### How to Fix It

1. Open your `docker-compose.yml` file in a text editor.
2. Find the line that starts with `CHARON_CADDY_ADMIN_API` (or `CPM_CADDY_ADMIN_API`).
3. Delete that line. Charon uses the correct socket on its own.
4. Save the file, then restart Charon:

```bash
docker compose up -d
```

### Which Values Work

| Value | Result |
| ----- | ------ |
| The line removed | Works (recommended) |
| `unix:///app/data/run/caddy-admin.sock` | Works. This is the default socket. |
| `http://localhost:2019` or `https://localhost:2019` | Accepted for older configurations. Charon ignores it, uses the socket instead, and logs a warning. Remove the line. |
| A service name, a container name, `127.1`, or any other server address | Refused. Charon stops with the message above. |

### Still Using Another Caddy?

Charon brings its own Caddy, so it cannot manage a Caddy running somewhere else. To move your existing sites into Charon, use the Caddyfile import. See the [import guide](../import-guide.md).

### Check That It Worked

Run `docker logs charon`. The message should be gone, and Charon should keep running (`docker ps` shows the container as "Up").
