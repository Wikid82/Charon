---
title: Orthrus — Remote Tunnel Agent
description: Connect to Docker on a remote machine through a secure outbound tunnel — no open ports required
category: features
---

# Orthrus — Remote Tunnel Agent

Imagine your HomeLab server is locked in a basement room with no way in from the outside. Orthrus is a small messenger you install *inside* that room. It reaches out to Charon and says "hey, I'm here — talk to me." Charon can then see what's running on that machine, even though it can never knock on the door itself.

No port-forwarding. No firewall rules. No public IP address needed on the remote machine.

---

## What Problem Does Orthrus Solve?

Most home servers sit behind a router (a NAT firewall). From the internet's point of view, the server is invisible — nobody outside can start a conversation with it.

Charon normally needs to reach your server directly, so this is a problem.

**Orthrus flips the conversation.** Instead of Charon trying to reach your server, your server reaches out to Charon first. Once that outbound connection is open, Charon can talk back through it — seeing your Docker containers as if they were right next door.

---

## How It Works

1. **You install the Orthrus agent** on your remote machine (one command).
2. **The agent dials outward** to your Charon instance over a secure, encrypted connection — just like your browser visits a website.
3. **Charon keeps that connection open** and uses it to ask "what containers are running?"
4. **You see those containers in Charon** and can route websites to them, just like local ones.

**Disconnections are handled automatically** — if the network hiccups, the agent reconnects on its own with no action required from you.

> **Note:** Orthrus is read-only. It can list containers, images, and networks — but it cannot start, stop, delete, or modify anything on your remote machine. This is by design and cannot be changed.

---

## Setting Up an Orthrus Agent

### Step 1 — Register the Agent in Charon

1. In the Charon sidebar, click **Remote Agents**
2. Click **Add Agent**
3. Give it a friendly name (e.g. "HomeLab Server" or "NAS")
4. Click **Create**

### Step 2 — Save the Auth Key

> ⚠️ **Save this key now.** It starts with `ch_orthrus_` and is shown **once only**. If you lose it, delete the agent and create a new one.

Copy the key somewhere safe — a password manager, a note, anything. Once you close this screen, Charon will never show the full key again.

### Step 3 — Install the Agent on Your Remote Machine

Charon gives you a ready-made install snippet. Pick the method that fits your setup:

| Method | Best For |
|---|---|
| Docker Compose | Servers already running Docker |
| systemd | Bare-metal Linux servers |
| Kubernetes | K8s clusters — deploys as a DaemonSet |
| Homebrew | macOS machines |
| Tarball | Any Linux without a package manager |

1. Click the **Install** tab on the agent page
2. Choose your preferred method
3. Copy the snippet
4. On your **remote machine**, paste and run it (replace `<AUTH_KEY>` with the key you saved)

### Step 4 — Watch It Go Online

Back in Charon → **Remote Agents**, your agent should flip to **Online** within 10–30 seconds.

That's it. You can now use this agent when [adding a Remote Server](../guides/remote-docker-setup.md).

---

## Agent Status Reference

| Status | Meaning | What To Do |
|---|---|---|
| ✅ Online | Connected and healthy | Nothing — you're good |
| ❌ Offline | Lost connection or not started | Check the agent is running on the remote machine |
| 🟡 Pending | Registered but never connected yet | Run the install snippet on the remote machine |

---

## What Orthrus Can (and Cannot) Do

Orthrus only ever lets Charon **read** information from your remote Docker. It cannot touch anything.

**It CAN:**
- List running containers and their details
- List images, networks, and volumes
- Stream container logs (for display in Charon)
- Report Docker system info

**It CANNOT:**
- Start, stop, restart, or delete containers
- Create or remove networks or volumes
- Pull images
- Run commands inside containers

This restriction is enforced at every single request — there is no way to turn it off.

---

## External Docker Proxy (Advanced)

Some tools like to talk to Docker directly instead of going through Charon's screens — for example, an update-checker like **Dockhand** or **Diun** that watches your containers for new versions, or a monitoring dashboard. Normally you'd run a separate `docker-socket-proxy` container just to give those tools safe, read-only access. Orthrus can do that job for you.

**What it is:** an optional door through the same secure tunnel your agent already uses. Turn it on, and a third-party tool anywhere on your network can talk to that agent's Docker API — read-only, same as everything else in this guide.

**How to turn it on:**

1. Go to **Remote Agents**
2. Click the **gear icon** next to the agent you want
3. Set a port (any number from 1024 to 65535)
4. Click **Save**

**Connecting your tool:** point it at:

```
tcp://<host>:<port>
```

`<host>` is your Charon instance's own address, as reachable from wherever the third-party tool runs — Charon fills this in for you automatically, so you don't need to look it up or type it yourself. `<port>` is the number you chose in Step 3 above.

**Still strictly read-only.** Just like the rest of Orthrus, there is no way to turn this restriction off. Through this port, a tool can:

- List containers, images, networks, and volumes
- Read Docker system info, version, and live events
- Read container logs, stats, and running processes (top)
- Look up details about a specific image (image inspect)
- Check the registry for a newer version of an image (registry digest check)

One note on that last item: "read-only" means the proxy can't change anything on your Docker host — it can't start, stop, or modify a single thing. But the registry digest check does cause your agent's Docker daemon to reach out to the image's registry (e.g. Docker Hub) to check for updates. That outbound check is expected and is exactly what makes update-checker tools work — it just isn't touching your host, which is the guarantee that matters here.

---

## Troubleshooting

| Problem | Likely Cause | Fix |
|---|---|---|
| Agent stays **Pending** | Snippet not run yet | Run it on the remote machine |
| Agent shows **Offline** | Agent process stopped | Restart the agent service or container |
| Agent goes **Offline** after reboot | Not set to start automatically | Use the systemd snippet, or add `restart: always` to Docker Compose |
| Auth key lost | Page closed before saving | Delete the agent and create a new one — the key cannot be recovered |
| Agent connects but no containers appear | Docker socket not mounted | Add `/var/run/docker.sock:/var/run/docker.sock:ro` to the agent's volume list |
| Third-party tool can't reach my agent's Docker API | Wrong port configured in the tool | Check the tool is using the External Proxy port shown in the gear-icon dialog — not Charon's main web port |

---

*Ready to connect your first remote server? Follow the [Remote Docker Setup Guide](../guides/remote-docker-setup.md).*
