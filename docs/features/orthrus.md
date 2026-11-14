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

> **Note:** Orthrus is **read-only by default**. It can list containers, images, and networks — but out of the box it cannot start, stop, delete, or modify anything on your remote machine. If you want a specific agent to be able to do more — for example, letting an update-checker tool apply an update it's found — you can explicitly opt that one agent into a narrow, audited set of write operations. See [What Orthrus Can (and Cannot) Do](#what-orthrus-can-and-cannot-do) below.

---

## Setting Up an Orthrus Agent

### Step 1 — Register the Agent in Charon

1. In the Charon sidebar, open **Hecate** → **Agent** (admins only)
2. Click **Add Agent**
3. Give it a friendly name (e.g. "HomeLab Server" or "NAS")
4. Click **Create**

### Step 2 — Save the Auth Key

> ⚠️ **Save this key now.** It starts with `ch_orthrus_` and is shown **once only**. If you lose it, you can get a new one with [Rotate Key](#rotate-an-agents-key).

Copy the key somewhere safe — a password manager, a note, anything. You will store it in a small key file in Step 3. Once you close this screen, Charon will never show the full key again.

### Step 3 — Install the Agent on Your Remote Machine

Charon gives you a ready-made install snippet for each method. Pick the one that fits your setup:

| Method | Best For |
| --- | --- |
| Docker Compose | Servers already running Docker |
| Kubernetes | K8s clusters — runs one agent on each machine in the cluster |

1. Click the **Install** tab on the agent page
2. Choose your preferred method
3. Copy the snippet
4. On your **remote machine**, run the save command at the top of the snippet. It stores your key in a file. If `<AUTH_KEY>` is still in the snippet, replace it with the key you saved in Step 2. The [key file section](#keep-the-auth-key-in-a-file) explains this in more detail.
5. Paste the rest of the snippet into a file named `docker-compose.yml`, then run `docker compose up -d`. For Kubernetes, run the `kubectl` command at the top of the snippet first, then apply the rest as its comments describe.
6. **Docker Compose only:** check that the file includes `PUID`, `PGID`, and `TZ` lines, and that `PGID` is set to your Docker group's number. If a line is missing, add it as shown in [Set the Permissions](#set-the-permissions-puid-pgid-and-tz) below. Most setups need this step.

> **Which agent version?** The snippet names the agent image that matches your Charon release, so the two stay in step. Development and nightly builds of Charon use the `dev` or `nightly` agent tag instead, and the snippet notes this.

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

**Example `.env` file** (kept in the same folder as your Orthrus compose file). The auth key is not listed here; it lives in its own file, described in [Keep the Auth Key in a File](#keep-the-auth-key-in-a-file):

```env
PUID=65532
PGID=998
TZ=America/New_York
DOCKER_SOCK=/var/run/docker.sock
ORTHRUS_SERVER_URL=wss://charon.example.com/api/v1/ws/orthrus/connect
ORTHRUS_AGENT_ID=<agent ID from Charon>
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
      - ORTHRUS_AUTH_KEY_FILE=/run/secrets/orthrus_auth_key
    secrets:
      - orthrus_auth_key
    volumes:
      - ${DOCKER_SOCK}:/var/run/docker.sock:ro

secrets:
  orthrus_auth_key:
    file: ./orthrus_auth_key
```

`DOCKER_SOCK` is the path to the Docker socket on your machine. The default (`/var/run/docker.sock`) is right for most Linux systems. The `:ro` at the end mounts it **read-only**. That protects the socket file itself. It does not limit which Docker commands the agent can send; the agent's built-in allow-list decides that.

**Good to know:**

- **Never use `0`.** A value of `0` means the administrator (root). Orthrus refuses to start if `PUID` or `PGID` is `0`.
- **Your own `user:` setting wins.** If you start the container with a `user:` line in Compose (or `-u` with `docker run`), the agent already runs as that user and `PUID` and `PGID` are ignored.
- **Can't see any containers?** Almost always `PGID` is wrong. Run the `stat` command again, fix the number, and restart the agent.

### Running as Root (Not Recommended)

Orthrus will only run as root if you deliberately allow it. Set `ORTHRUS_ALLOW_ROOT=true` in the agent's environment, then `PUID` or `PGID` set to `0` is accepted. Use this only if you understand what it means: the agent then runs with full administrator rights on your machine. Orthrus prints a warning each time it starts this way.

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
      - ORTHRUS_AUTH_KEY_FILE=/run/secrets/orthrus_auth_key
    secrets:
      - orthrus_auth_key
    volumes:
      - ${DOCKER_SOCK}:/var/run/docker.sock:ro

secrets:
  orthrus_auth_key:
    file: ./orthrus_auth_key
```

**Why this is safe here:** In rootless Docker, the agent's "root" is only your own everyday user account on the computer, not the machine's administrator. It has no more power over your computer than that account already has.

**Do not use this on normal (rootful) Docker.** On regular Docker, which is the default on most Linux systems, `PUID=0` means real administrator (root) on your machine. Keep the normal settings from the top of this page. Not sure which kind you have? Run `docker info --format '{{.SecurityOptions}}'`. If the output includes the word `rootless`, you have rootless Docker. If it does not, you do not.

> **Not yet verified:** This setup follows from how rootless Docker maps IDs. The Charon maintainers have not yet tested it on a live rootless Docker install. If it does not work for you, please open an issue and tell us your Docker version.

---

## Keep the Auth Key in a File

The agent reads its key from a small file, so the key is not stored in the compose file or in the agent's settings.

**Docker Compose:**

1. In the folder with your `docker-compose.yml`, run this command. Replace the key with yours:

   ```bash
   printf '%s' 'ch_orthrus_<your-key>' > orthrus_auth_key && chmod 600 orthrus_auth_key
   ```

   `chmod 600` makes the file readable only by your own user account.
2. Keep the `secrets:` block from the example above. It passes the file into the agent at `/run/secrets/orthrus_auth_key`.
3. Keep `ORTHRUS_AUTH_KEY_FILE=/run/secrets/orthrus_auth_key` in the agent's environment. The agent reads the file before it switches to its low-privilege user.

**Kubernetes:** The snippet first creates a Secret named `orthrus-auth-key` that holds your key. The agent reads it from a mounted file.

**Good to know:**

- The file should hold only the key. Spaces and line breaks around the key are ignored.
- If the file is empty, the agent stops and says the key is not set yet. Paste your key into the file and restart the agent.
- Set **either** `ORTHRUS_AUTH_KEY_FILE` **or** `ORTHRUS_AUTH_KEY`, not both. If both are set, the agent stops with an error.
- The older `ORTHRUS_AUTH_KEY` environment variable still works for existing setups. It shows the key in `docker inspect` output and the process environment, so the key file is the recommended choice.
- The `--auth-key` command-line flag also works, but the agent logs a warning because the flag can appear in process lists.

---

## Rotate an Agent's Key

Use this when you want a new key, for example if the old one was saved somewhere you no longer use.

1. Go to **Hecate** → **Agent**
2. Click the **key icon** next to the agent
3. Read the note, then click **Rotate key**
4. Copy the new key. Charon shows it **once only**.
5. On the remote machine, replace the key in `orthrus_auth_key` with the new key (use the save command from the key file section again), then restart the agent. With Docker Compose, run `docker compose restart`.

The old key stops working right away, and the agent disconnects until it has the new key. Your agent keeps its name, port, and settings.

---

## Using Orthrus on the Same Server as Charon

Orthrus is not just for faraway machines. Running it beside Charon on the same server is the **recommended way** to see your containers, because Charon itself no longer needs the Docker socket. (Mounting the socket straight into Charon is still supported if you prefer it.)

The socket is still mounted somewhere: into the small Orthrus agent instead of into Charon. The agent is read-only by default, and write access is an explicit per-agent opt-in (see [What Orthrus Can (and Cannot) Do](#what-orthrus-can-and-cannot-do)).

1. **Register an agent.** In Charon, open **Hecate** → **Agent**, click **Add Agent**, and copy the one-time auth key.
2. **Run the agent next to Charon.** Use the Docker Compose snippet from the **Install** tab and set `ORTHRUS_SERVER_URL` to **any address the agent can reach Charon on** (see the list below). If you add the agent to Charon's own compose file instead, start it with `docker compose --profile orthrus up -d` after saving the key file.
3. **Remove the socket from Charon.** Delete the `/var/run/docker.sock` line (and any `group_add` entry) from Charon's compose file. Keep the socket only on the agent.
4. **Wait for Online.** When the agent shows **Online** under **Hecate** → **Agent**, open **Hecate** → **Sources** and add a Source with the **Docker** provider and the **Orthrus** connection, pointing at your agent.
5. **Use it.** In the Proxy Host form, your new Source now appears in the **Source** dropdown.

### Which Server URL Should I Use?

The agent only needs an address it can use to reach Charon. The path (`/api/v1/ws/orthrus/connect`) is added for you if you leave it off. Pick whichever of these fits your setup:

- **Your server's VPN (Tailscale) address.** For example `ws://100.x.y.z:8080`, where `100.x.y.z` is the machine's Tailscale IP. This works in practice: the agent dials that Tailscale address to reach Charon.
- **A shared Docker network.** Put Charon and the agent on the same Compose network and use `ws://charon:8080` (use your Charon container's name). Do not use `localhost` here: inside the agent's container, `localhost` means the agent itself, not Charon.
- **Host networking.** If the agent runs with `network_mode: host`, use `ws://localhost:8080`.
- **Your public address.** If Charon is behind HTTPS, use your normal `wss://` address, for example `wss://charon.example.com`.

If agents reach Charon through another reverse proxy, add that proxy to `CHARON_TRUSTED_PROXIES` so Charon counts each agent's connections by its own address. See [Trusted Proxies](../configuration/trusted-proxies.md).

### When Is Plain `ws://` Allowed?

Plain `ws://` does not encrypt the connection. The agent accepts it without any extra setting when the address is one of these:

- **Your own network or VPN:** loopback (`127.x.x.x`, `::1`), private IP addresses (`10.x`, `172.16.x` to `172.31.x`, `192.168.x`), VPN addresses from Tailscale (`100.64.x` to `100.127.x`), and IPv6 private addresses (`fc00::/7`).
- **`localhost`**, and names ending in `.localhost`.
- **A single-word name**, such as `charon`. This is the name of a Docker Compose service.
- **A name ending in `.local`, `.home.arpa`, or `.internal`.**

For any other address, use `wss://`. A VPN such as Tailscale or WireGuard already encrypts its traffic, so plain `ws://` over one of those is the usual setup. If you need plain `ws://` for another address, set `ORTHRUS_ALLOW_INSECURE_WS=true` in the agent's environment.

The agent logs a warning whenever it uses plain `ws://` to an address other than `localhost`. The warning is a reminder. It does not stop the agent.

### Local Option Greyed Out?

If no Docker socket is mounted into Charon, the **Local (Docker Socket)** choice in the Proxy Host form's **Source** dropdown is greyed out, with a short note pointing you to Orthrus. That is expected once you follow the steps above: choose your Orthrus Source instead.

---

## What Orthrus Can (and Cannot) Do

**By default, every agent is strictly read-only.** There is no setting that changes this for every agent at once. Orthrus only lets Charon **read** information from your remote Docker unless you take the deliberate, per-agent step described below.

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

These rules are checked twice, once by Charon and once by the agent itself, for every request. Write access is only available for the fixed list above, and only for an agent you have turned it on for.

---

## External Docker Proxy (Advanced)

Some tools like to talk to Docker directly instead of going through Charon's screens — for example, an update-checker like **Dockhand** or **Diun** that watches your containers for new versions, or a monitoring dashboard. Normally you'd run a separate `docker-socket-proxy` container just to give those tools safe, read-only access. Orthrus can do that job for you.

**What it is:** an optional door through the same secure tunnel your agent already uses. Turn it on, and a third-party tool on the network you choose can talk to that agent's Docker API — read-only, same as everything else in this guide.

**How to turn it on:**

1. Go to **Hecate** → **Agent**
2. Click the **gear icon** next to the agent you want
3. Set a port (any number from 1024 to 65535). Enter `0` to turn the proxy off.
4. Choose a **Listen on** network (see below)
5. Click **Save**

### Listen On: Choosing the Network

**Listen on** decides which network Charon's proxy answers on. The choices are:

| Choice | What it does |
| --- | --- |
| **Automatic** (default) | Uses the main network Charon is connected to. The dialog shows the address it picked. |
| A network from the list | Answers only on that network, for example `172.18.0.0/16`. Pick this when your tool is on a different Docker network. |
| **All networks (0.0.0.0)** | Answers on every network Charon has. |
| **Custom address or subnet** | Enter an IP address, such as `192.168.1.10`, or a subnet, such as `192.168.1.0/24`. |

- **If Automatic cannot find a network,** the proxy answers only on the loopback address, and the dialog says: "Automatic found no network to listen on, so the proxy listens on loopback. Choose a network to change this." Loopback addresses are shown as "Not reachable from other containers or hosts."
- **If no local network matches** the subnet you entered, the proxy does not start and the status shows an error. Choose another network.
- **Changing Listen on** takes effect when the agent reconnects. The dialog tells you when a reconnect is needed.
- **Keep the proxy for tools only.** Do not create a Charon proxy host that points at this port. The proxy has no login, so only give its address to tools you trust. Choose **All networks** only when tools on every network need it.

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
- Look up a container's details. Environment variable values appear as `[redacted]`; their names stay visible.
- Check the registry for a newer version of an image (registry digest check)

The registry check needs a note. Read-only means the proxy can't change anything on your Docker host by itself — it can't start, stop, or modify a single thing unless you've explicitly turned on write access (below). The registry check does cause your agent's Docker to reach out to the image's registry (e.g. Docker Hub) to look for updates. That outbound check is expected and is what makes update-checker tools work. In read-only mode, the proxy refuses registry names that point at this machine or a local address, such as `127.0.0.1:5000` or `localhost`. Registries named by ordinary internet names are contacted as normal.

**Want the tool to be able to apply an update it finds, not just detect one?** See [Opting an agent into write access](#opting-an-agent-into-write-access) above — it's the same idea, just for a fixed, audited set of write operations layered on top of this same proxy.

### Forward Targets

Charon keeps its own internal Orthrus ports off the list of valid forward targets. A proxy host or location that points at this machine on an agent's external proxy port, or on one of the internal ports Charon opens for a connected agent, is refused when it is saved. That includes the host-side address of the Docker network Charon is attached to, the name `host.docker.internal`, and any name that cannot be resolved. The agent-name form (`orthrus:<agent>`) is not accepted as a forward target at all. A host saved before this rule, or one whose agent port changes later, is left out of the generated config until its target is corrected, and Charon logs a warning that names the host.

---

## Agent Status Reference

| Status | Meaning | What To Do |
| --- | --- | --- |
| ✅ Online | Connected and healthy | Nothing — you're good |
| ❌ Offline | Lost connection or not started | Check the agent is running on the remote machine |
| 🟡 Pending | Registered but never connected yet | Run the install snippet on the remote machine |

---

## Troubleshooting

| Problem | Likely Cause | Fix |
| --- | --- | --- |
| Agent stays **Pending** | Snippet not run yet | Run it on the remote machine |
| Agent shows **Offline** | Agent process stopped | Restart the agent service or container |
| Agent goes **Offline** after reboot | Not set to start automatically | Make sure the Compose file has `restart: always` (the install snippet includes it) |
| Agent stops and says the key is not set yet | The key file is empty | Paste your key into `orthrus_auth_key`, then restart the agent |
| Agent stops and mentions the key file | The file is missing, is a folder, or is much larger than a key | Check that `orthrus_auth_key` sits next to `docker-compose.yml` and holds only the key. See [Keep the Auth Key in a File](#keep-the-auth-key-in-a-file) |
| Agent stops and says to set only one key setting | Both `ORTHRUS_AUTH_KEY_FILE` and `ORTHRUS_AUTH_KEY` are set | Remove the one you don't use |
| Agent stops and mentions `ws://` | The address is not private and does not use `wss://` | Use a `wss://` address, or set `ORTHRUS_ALLOW_INSECURE_WS=true`. See [When Is Plain ws:// Allowed?](#when-is-plain-ws-allowed) |
| Agent takes a while to reconnect after a restart, and its logs show the agent as `unknown` | `ORTHRUS_AGENT_ID` is not set | Set `ORTHRUS_AGENT_ID` to the agent ID shown in Charon |
| Agent is refused after several failed attempts | Charon pauses connection attempts for a short time after repeated failures, such as with an old key | Wait a minute. The agent retries on its own |
| Auth key lost | Page closed before saving | Use [Rotate Key](#rotate-an-agents-key) next to the agent to get a new key |
| Agent connects but no containers appear | Docker socket not mounted | Add `/var/run/docker.sock:/var/run/docker.sock:ro` to the agent's volume list |
| Agent connects but no containers appear, and it cannot open the Docker socket | `PGID` does not match the Docker group | Run `stat -c '%g' /var/run/docker.sock`, set `PGID` to that number, then restart the agent |
| Agent refuses to start and mentions `PUID` or `PGID` being `0` | Root IDs are blocked by default | Set `PUID` and `PGID` to non-zero numbers, or see [Running as Root](#running-as-root-not-recommended) |
| Permission denied on the socket, and the socket path starts with `/run/user` | Rootless Docker needs different settings | See [Rootless Docker](#rootless-docker) |
| Tool on another Docker network cannot reach the proxy | **Listen on** is set to a network the tool cannot reach | Choose the tool's network under **Listen on**, then save |
| Third-party tool can't reach my agent's Docker API | Wrong port configured in the tool | Check the tool is using the External Proxy port shown in the gear-icon dialog — not Charon's main web port |

---

*Ready to connect your first Docker source? Follow the [Remote Docker Setup Guide](../guides/remote-docker-setup.md).*
