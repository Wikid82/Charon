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

1. In the Charon sidebar, open **Hecate** → **Agent** (admins only)
2. Click **Add Agent**
3. Give it a friendly name (e.g. "HomeLab Server" or "NAS")
4. Click **Create**

### Step 2 — Save the Auth Key

> ⚠️ **Save this key now.** It starts with `ch_orthrus_` and is shown **once only**. If you lose it, delete the agent and create a new one.

Copy the key somewhere safe — a password manager, a note, anything. Once you close this screen, Charon will never show the full key again.

### Step 3 — Install the Agent on Your Remote Machine

Charon gives you a ready-made install snippet. Pick the method that fits your setup:

| Method | Best For |
| --- | --- |
| Docker Compose | Servers already running Docker |
| systemd | Bare-metal Linux servers |
| Kubernetes | K8s clusters — deploys as a DaemonSet |
| Homebrew | macOS machines |
| Tarball | Any Linux without a package manager |

1. Click the **Install** tab on the agent page
2. Choose your preferred method
3. Copy the snippet
4. On your **remote machine**, paste and run it (replace `<AUTH_KEY>` with the key you saved)
5. Check that the file includes `PUID`, `PGID`, and `TZ` lines, and that `PGID` is set to your Docker group's number. If a line is missing, add it as shown in [Set the Permissions](#set-the-permissions-puid-pgid-and-tz) below. Most setups need this step.

### Step 4 — Watch It Go Online

Back in Charon → **Hecate** → **Agent**, your agent should flip to **Online** within 10–30 seconds.

That's it. You can now use this agent when [adding a Source](../guides/remote-docker-setup.md). Running Charon and Docker on the same machine? See [Using Orthrus on the same server as Charon](#using-orthrus-on-the-same-server-as-charon).

---

## Set the Permissions (PUID, PGID, and TZ)

For safety, Orthrus runs **without admin rights**. Inside its container it works as a regular, low-privilege user. Three settings control this:

| Setting | What it means | What to put here |
| --- | --- | --- |
| `PUID` | The user number the agent runs as. Think of it as its ID badge. | Leave it at `65532` (the default). |
| `PGID` | The group number the agent belongs to. Groups decide who may open shared files, such as the Docker socket. | The number of your Docker group (see below). |
| `TZ` | Your timezone, so the log times match your clock. | For example `America/New_York`. Default: `UTC`. |

**What is the Docker socket?** It is a connection point (the file `/var/run/docker.sock`) that lets programs talk to Docker. Orthrus needs it to see your containers. Only certain groups may open that file, so the agent must belong to the right group. That group is what `PGID` sets.

**Find your PGID.** On the machine that runs Docker (Linux), open a terminal and run:

```bash
stat -c '%g' /var/run/docker.sock
```

It prints one number, such as `998` or `988`. Put that number after `PGID=`.

> **Heads up:** That number goes in `PGID` only. `DOCKER_SOCK` stays the socket's *path* (`/var/run/docker.sock`); do not build it from this number. To check the number, run `getent group <number>`. It should print the `docker` group. You do not need to add the agent to any group, and nothing needs to be created inside the container.

**Example `.env` file** (kept in the same folder as your Orthrus compose file):

```env
PUID=65532
PGID=998
TZ=America/New_York
DOCKER_SOCK=/var/run/docker.sock
ORTHRUS_SERVER_URL=wss://charon.example.com/api/v1/ws/orthrus/connect
ORTHRUS_AGENT_ID=<agent ID from Charon>
ORTHRUS_AUTH_KEY=<auth key from Charon>
```

**Example Docker Compose service** (the same shape as the install snippet, with the permission settings included):

```yaml
services:
  orthrus:
    image: wikid82/orthrus:latest
    restart: always
    container_name: orthrus
    environment:
      - PUID=${PUID}
      - PGID=${PGID}
      - TZ=${TZ}
      - ORTHRUS_SERVER_URL=${ORTHRUS_SERVER_URL}
      - ORTHRUS_AGENT_ID=${ORTHRUS_AGENT_ID}
      - ORTHRUS_AUTH_KEY=${ORTHRUS_AUTH_KEY}
    volumes:
      - ${DOCKER_SOCK}:/var/run/docker.sock:ro
```

`DOCKER_SOCK` is the path to the Docker socket on your machine. The default (`/var/run/docker.sock`) is right for most Linux systems. The `:ro` at the end mounts it **read-only**, so the agent cannot change or delete the socket file.

**Good to know:**

- **Never use `0`.** A value of `0` means the administrator (root). Orthrus refuses to start if `PUID` or `PGID` is `0`.
- **Your own `user:` setting wins.** If you start the container with a `user:` line in Compose (or `-u` with `docker run`), the agent already runs as that user and `PUID` and `PGID` are ignored.
- **Can't see any containers?** Almost always `PGID` is wrong. Run the `stat` command again, fix the number, and restart the agent.

### Running as Root (Not Recommended)

Orthrus will only run as root if you deliberately allow it. Set `ORTHRUS_ALLOW_ROOT=true` in the agent's environment, then `PUID` or `PGID` set to `0` is accepted. Use this only if you understand the risk: anyone who breaks into a root-level helper gets control of the whole machine. Orthrus prints a loud warning each time it starts this way.

### Rootless Docker

Rootless Docker runs Docker as your own everyday user instead of as the machine's administrator. If you use it, the Docker socket is **not** at `/var/run/docker.sock`. It is in your user's folder:

```text
/run/user/<your-uid>/docker.sock
```

To find your number, run `id -u` as the user who runs Docker. For example, if it prints `1000`, the path is `/run/user/1000/docker.sock`.

**Why the normal settings fail here.** Rootless Docker translates IDs between the container and your computer. "Root" inside the container becomes your own unprivileged user on the computer. Every other ID, such as `65532` or `1000`, is mapped to a reserved range of IDs that does not include your user's access to the socket. So dropping to a `PUID` and `PGID` like those loses access to the socket. Changing `PGID` cannot fix this, because group numbers are translated the same way.

**What to set instead:**

```env
PUID=0
PGID=0
ORTHRUS_ALLOW_ROOT=true
DOCKER_SOCK=/run/user/1000/docker.sock   # replace 1000 with your number from id -u
```

**Example Docker Compose service:**

```yaml
services:
  orthrus:
    image: wikid82/orthrus:latest
    restart: always
    container_name: orthrus
    environment:
      - PUID=0
      - PGID=0
      - ORTHRUS_ALLOW_ROOT=true
      - TZ=${TZ}
      - ORTHRUS_SERVER_URL=${ORTHRUS_SERVER_URL}
      - ORTHRUS_AGENT_ID=${ORTHRUS_AGENT_ID}
      - ORTHRUS_AUTH_KEY=${ORTHRUS_AUTH_KEY}
    volumes:
      - ${DOCKER_SOCK}:/var/run/docker.sock:ro
```

**Why this is safe here:** In rootless Docker, the agent's "root" is only your own everyday user account on the computer, not the machine's administrator. It has no more power over your computer than that account already has.

**Do not use this on normal (rootful) Docker.** On regular Docker, which is the default on most Linux systems, `PUID=0` means real administrator (root) on your machine. Keep the normal settings from the top of this page. Not sure which kind you have? Run `docker info --format '{{.SecurityOptions}}'`. If the output includes the word `rootless`, you have rootless Docker. If it does not, you do not.

> **Not yet verified:** This setup follows from how rootless Docker maps IDs. The Charon maintainers have not yet tested it on a live rootless Docker install. If it does not work for you, please open an issue and tell us your Docker version.

## Agent Status Reference

| Status | Meaning | What To Do |
| --- | --- | --- |
| ✅ Online | Connected and healthy | Nothing — you're good |
| ❌ Offline | Lost connection or not started | Check the agent is running on the remote machine |
| 🟡 Pending | Registered but never connected yet | Run the install snippet on the remote machine |

---

## Using Orthrus on the Same Server as Charon

Orthrus is not just for faraway machines. Running it beside Charon on the same server is the **recommended way** to see your containers, because Charon itself no longer needs the Docker socket. (Mounting the socket straight into Charon is still supported if you prefer it.)

The socket is still mounted somewhere: into the small Orthrus agent instead of into Charon. The agent is read-only by default, and write access is an explicit per-agent opt-in (see [What Orthrus Can (and Cannot) Do](#what-orthrus-can-and-cannot-do)).

1. **Register an agent.** In Charon, open **Hecate** → **Agent**, click **Add Agent**, and copy the one-time auth key.
2. **Run the agent next to Charon.** Use the Docker Compose snippet from the **Install** tab and set `ORTHRUS_SERVER_URL` to **any address the agent can reach Charon on** (see the list below).
3. **Remove the socket from Charon.** Delete the `/var/run/docker.sock` line (and any `group_add` entry) from Charon's compose file. Keep the socket only on the agent.
4. **Wait for Online.** When the agent shows **Online** under **Hecate** → **Agent**, open **Hecate** → **Sources** and add a Source with the **Docker** provider and the **Orthrus** connection, pointing at your agent.
5. **Use it.** In the Proxy Host form, your new Source now appears in the **Source** dropdown.

### Which Server URL Should I Use?

The agent only needs an address it can use to reach Charon. The path (`/api/v1/ws/orthrus/connect`) is added for you if you leave it off. Pick whichever of these fits your setup:

- **Your server's VPN (Tailscale) address.** For example `ws://100.x.y.z:8080`, where `100.x.y.z` is the machine's Tailscale IP. This works in practice: the agent dials that Tailscale address to reach Charon.
- **A shared Docker network.** Put Charon and the agent on the same Compose network and use `ws://charon:8080` (use your Charon container's name). Do not use `localhost` here: inside the agent's container, `localhost` means the agent itself, not Charon.
- **Host networking.** If the agent runs with `network_mode: host`, use `ws://localhost:8080`.
- **Your public address.** If Charon is behind HTTPS, use your normal `wss://` address, for example `wss://charon.example.com`.

> **Is plain `ws://` safe?** Over a private, encrypted link such as Tailscale or WireGuard, yes. The link itself is encrypted. Over a network you do not trust, no: use `wss://` instead. The agent logs a warning when it connects over `ws://` to any address other than `localhost`, so seeing that warning on a private link is expected.

### Local Option Greyed Out?

If no Docker socket is mounted into Charon, the **Local (Docker Socket)** choice in the Proxy Host form's **Source** dropdown is greyed out, with a short note pointing you to Orthrus. That is expected once you follow the steps above: choose your Orthrus Source instead.

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

1. Go to **Hecate** → **Agent**
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

1. Go to **Hecate** → **Agent**
2. Click the **gear icon** next to the agent you want
3. Set a port (any number from 1024 to 65535)
4. Click **Save**

**Connecting your tool:** point it at:

```text
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
| --- | --- | --- |
| Agent stays **Pending** | Snippet not run yet | Run it on the remote machine |
| Agent shows **Offline** | Agent process stopped | Restart the agent service or container |
| Agent goes **Offline** after reboot | Not set to start automatically | Use the systemd snippet, or add `restart: always` to Docker Compose |
| Auth key lost | Page closed before saving | Delete the agent and create a new one — the key cannot be recovered |
| Agent connects but no containers appear | Docker socket not mounted | Add `/var/run/docker.sock:/var/run/docker.sock:ro` to the agent's volume list |
| Agent connects but no containers appear, and it cannot open the Docker socket | `PGID` does not match the Docker group | Run `stat -c '%g' /var/run/docker.sock`, set `PGID` to that number, then restart the agent |
| Agent refuses to start and mentions `PUID` or `PGID` being `0` | Root IDs are blocked by default | Set `PUID` and `PGID` to non-zero numbers, or see [Running as Root](#running-as-root-not-recommended) |
| Permission denied on the socket, and the socket path starts with `/run/user` | Rootless Docker needs different settings | See [Rootless Docker](#rootless-docker) |
| Third-party tool can't reach my agent's Docker API | Wrong port configured in the tool | Check the tool is using the External Proxy port shown in the gear-icon dialog — not Charon's main web port |

---

*Ready to connect your first Docker source? Follow the [Remote Docker Setup Guide](../guides/remote-docker-setup.md).*
