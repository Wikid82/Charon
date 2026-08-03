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

> **Note:** Orthrus is **read-only by default**. It can list containers, images, and networks — but out of the box it cannot start, stop, delete, or modify anything on your remote machine, and that default can never be loosened by accident. If you want a specific agent to be able to do more — for example, letting an update-checker tool apply an update it's found — you can explicitly opt that one agent into a narrow, audited set of write operations. See [What Orthrus Can (and Cannot) Do](#what-orthrus-can-and-cannot-do) below.

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

**By default, every agent is strictly read-only**, and that default can't be loosened by accident — there's no setting that weakens it globally. Orthrus only ever lets Charon **read** information from your remote Docker unless you take the deliberate, per-agent step described below.

**Every agent, always, CAN:**
- List running containers and their details
- List images, networks, and volumes
- Stream container logs (for display in Charon)
- Report Docker system info

**A default (read-only) agent CANNOT:**
- Start, stop, restart, or delete containers
- Create or remove networks or volumes
- Pull images
- Run commands inside containers

### Opting an agent into write access

If you're using the [External Docker Proxy](#external-docker-proxy-advanced) to let a third-party tool (like an update-checker) talk to an agent's Docker API, you can optionally, explicitly grant that one agent a **narrow, fixed set** of write operations:

- Pull a new image
- Start a container
- Stop a container
- Restart a container
- Remove a container
- Create (recreate) a container

This is exactly enough for a tool to run a full "pull the new image, swap the container over" update cycle — nothing more. **Regardless of this setting, the following are never permitted, for any agent, under any configuration:**

- Running commands inside a container (exec / shell access)
- Creating or removing networks or volumes
- Deleting images
- Building images
- Anything Docker Swarm or service-related

**Turning it on:**

1. Go to **Remote Agents**
2. Click the **shield icon** next to the agent you want
3. Toggle **write access** on
4. Type the agent's exact current name to confirm — this typed confirmation is required specifically so this can't be flipped on by accident. Turning it back off never requires typing anything.
5. Click **Save**

The change takes effect the next time that agent reconnects (the same way changing the External Proxy port does) — if the agent is already connected, Charon will tell you a reconnect is needed.

**Every write attempt is logged**, whether it succeeds or gets blocked, so you always have a record of what a connected tool actually did. Find this under **Audit Logs**, filtered to that agent.

One limitation worth knowing: if a container is attached to **more than one** Docker network, this write-access flow can recreate it on its primary network but won't re-attach the extra ones automatically — you'd need to do that manually, or through Charon's own Docker management screens, after the recreate.

This restriction — read-only by default, write access only when you explicitly and knowingly turn it on for one agent at a time — is enforced independently, twice, at every single request: once by Charon and once by the agent itself. There's no way to weaken the read-only default globally, and no way to grant any operation outside the fixed list above, no matter what.

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

**Read-only by default, same as the rest of Orthrus** — and that default can't be loosened by accident. Through this port, a tool can always:

- List containers, images, networks, and volumes
- Read Docker system info, version, and live events
- Read container logs, stats, and running processes (top)
- Look up details about a specific image (image inspect)
- Check the registry for a newer version of an image (registry digest check)

One note on that last item: read-only here means the proxy can't change anything on your Docker host by itself — it can't start, stop, or modify a single thing unless you've explicitly turned on write access (below). But the registry digest check does cause your agent's Docker daemon to reach out to the image's registry (e.g. Docker Hub) to check for updates. That outbound check is expected and is exactly what makes update-checker tools work — it just isn't touching your host, which is the guarantee that matters here.

**Want the tool to be able to apply an update it finds, not just detect one?** See [Opting an agent into write access](#opting-an-agent-into-write-access) above — it's the same idea, just for a fixed, audited set of write operations layered on top of this same proxy.

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
