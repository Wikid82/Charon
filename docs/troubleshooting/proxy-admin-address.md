---
title: Charon Stops at Startup with a Proxy Admin Address Message
description: Fix the startup message about CHARON_CADDY_ADMIN_API that appears after an upgrade.
---

## Charon Stops at Startup with a Proxy Admin Address Message

### What You See

Charon stops right after it starts. The container logs (run `docker logs charon`) show this message:

```text
CHARON_CADDY_ADMIN_API must point at this container (localhost); using an external proxy is not supported: set it to http://localhost:2019 or remove it
```

If your container is set to restart automatically, it will keep restarting and show this message each time.

### Why It Happens

Charon always uses the Caddy web server that is built into its own container. It no longer works with a Caddy running on another computer or under another name. The setting `CHARON_CADDY_ADMIN_API` tells Charon where that built-in Caddy is, and the only valid answer is `localhost`.

### How to Fix It

1. Open your `docker-compose.yml` file in a text editor.
2. Find the line that starts with `CHARON_CADDY_ADMIN_API` (or `CPM_CADDY_ADMIN_API`).
3. Do one of these:
   - Change the line to `- CHARON_CADDY_ADMIN_API=http://localhost:2019`
   - Or delete the line. Charon uses the correct address on its own.
4. Save the file, then restart Charon:

```bash
docker compose up -d
```

### Which Values Work

| Value | Result |
| ----- | ------ |
| `http://localhost:2019` | Works (recommended) |
| `https://localhost:2019` | Works. Charon treats it the same as the value above. |
| The line removed | Works |
| A service name, a container name, `127.1`, or any other server address | Refused. Charon stops with the message above. |

### Still Using Another Caddy?

Charon brings its own Caddy, so it cannot manage a Caddy running somewhere else. To move your existing sites into Charon, use the Caddyfile import. See the [import guide](../import-guide.md).

### Check That It Worked

Run `docker logs charon`. The message should be gone, and Charon should keep running (`docker ps` shows the container as "Up").
