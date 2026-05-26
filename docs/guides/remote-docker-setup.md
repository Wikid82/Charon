---
title: Connecting a Remote Docker Host
description: Step-by-step guide to managing Docker on a remote machine through Charon using the Orthrus agent
category: guides
---

# Connecting a Remote Docker Host

Your HomeLab is in the basement. Charon is running somewhere else — maybe on a cloud server, maybe in your office. This guide connects them safely, without opening any ports on your HomeLab or touching your router.

By the end, Charon will list your remote machine's Docker containers just like local ones, and you can route websites to them with a single click.

---

## Before You Start

Make sure you have:

- [ ] Charon is running and you can log in
- [ ] The remote machine (HomeLab) has Docker installed
- [ ] The remote machine can reach the internet (standard outbound HTTPS on port 443)
- [ ] You can run commands on the remote machine — via SSH, a keyboard, or a terminal app

---

## Step 1 — Register an Agent in Charon

<!-- screenshot: Charon sidebar with "Remote Agents" highlighted -->

1. In the Charon sidebar, click **Remote Agents**
2. Click **Add Agent**
3. Type a name for this machine — something like "HomeLab" or "NAS Box"
4. Click **Create**

> ⚠️ **Copy the auth key before closing this screen.** It starts with `ch_orthrus_` and is shown **one time only**. Paste it into a note or your password manager immediately. If you lose it, you'll need to delete the agent and start over.

<!-- screenshot: Auth key screen with copy button visible -->

---

## Step 2 — Install the Agent on Your Remote Machine

Still in Charon, click the **Install** tab on the agent page. You'll see several ready-made snippets. **Docker Compose is the easiest if your remote machine already runs Docker.**

<!-- screenshot: Install tab showing Docker Compose snippet -->

### Using Docker Compose (recommended)

On your **remote machine**, open a terminal and:

1. Create a new folder for the agent: `mkdir orthrus-agent && cd orthrus-agent`
2. Copy the Docker Compose snippet from the Charon UI
3. Create a file called `docker-compose.yml` and paste the snippet into it
4. Replace `<AUTH_KEY>` with the key you saved in Step 1
5. Run:

```bash
docker compose up -d
```

The agent starts in the background and immediately tries to connect to Charon.

> **Other install options:** If you prefer not to use Docker, the **Install** tab also provides a systemd unit file (for Linux servers), a Homebrew formula (for macOS), a Kubernetes DaemonSet, and a plain tarball. All work the same way — paste in your auth key and run it.

---

## Step 3 — Verify the Agent Is Online

<!-- screenshot: Remote Agents list showing "Online" status badge -->

Switch back to your **Charon browser tab** and look at **Remote Agents**. Your agent should show a green **Online** badge within 10–30 seconds.

**Still showing "Pending"?** That means the agent hasn't connected yet. Double-check:
- The `docker-compose.yml` is saved correctly and `docker compose up -d` ran without errors
- The auth key in the file matches what Charon gave you (no extra spaces or missing characters)
- The remote machine has outbound internet access

---

## Step 4 — (Optional) Assign a VPN Tunnel

> **Skip this step** if your remote machine has a direct IP address that Charon can see — for example it's on the same local network, or it has a public IP. Go straight to Step 5.

If your remote machine is behind a NAT (no public IP) **and** you also use a VPN like NetBird or Tailscale, you can tell Hecate which VPN device is your remote machine. This lets Charon resolve the address automatically.

1. First, add your VPN provider credentials: **Settings → Tunnel Providers → Add Provider**
2. Then open your agent in **Remote Agents**
3. Under **Network Assignment**, pick your provider and the device that represents the remote machine
4. Click **Save**

Not sure which option applies to you? The [Hecate guide](../features/hecate.md) explains each connection mode in plain English.

---

## Step 5 — Add the Remote Machine as a Docker Host

<!-- screenshot: Settings → Docker → Add Remote Host dialog -->

1. Go to **Settings → Docker** (you may also find this under **Remote Servers**)
2. Click **Add Remote Host**
3. Set **Connection Mode** to **Agent**
4. In the **Agent** dropdown, choose the agent you just registered
5. Click **Test Connection**

If you see a green success message, Charon can reach your remote Docker. Click **Save**.

If the test fails, check that the agent still shows **Online** in Remote Agents.

---

## Step 6 — Use Your Remote Containers

<!-- screenshot: "Select from Docker" dropdown showing remote host option -->

Your remote machine now appears as a Docker source anywhere Charon asks "which container?".

1. Go to **Hosts → Add Host**
2. Click **Select from Docker**
3. In the host dropdown, select your remote machine (it'll show the name you gave it)
4. Browse the containers running there and pick one
5. Fill in your domain name and click **Save**

Charon now proxies traffic through the Orthrus tunnel to reach that container. From the visitor's point of view, it's just a normal website with HTTPS.

---

## (Optional) Add Uptime Monitoring

Want to know immediately if a remote service goes down? Enable uptime monitoring on any proxy host that points to a remote container:

1. Open the proxy host in **Hosts**
2. Click **Edit**
3. Scroll to **Uptime Monitoring** and toggle it on
4. Click **Save**

Charon will check the service regularly and alert you through your configured notification channels if it becomes unreachable.

→ Learn more: [Uptime Monitoring](../features/uptime-monitoring.md)

---

## Troubleshooting

| Problem | Likely Cause | Fix |
|---|---|---|
| Agent stays **Pending** | Install snippet not run yet | Run `docker compose up -d` on the remote machine |
| **Test Connection** fails | Agent is offline | Check the agent container is running: `docker ps` |
| No containers listed | Docker socket not mounted in agent | Add `-v /var/run/docker.sock:/var/run/docker.sock:ro` to the agent's volumes |
| Auth key rejected | Key copied incorrectly | Delete the agent, create a new one, copy the key carefully |
| Agent disconnects repeatedly | Network instability | Normal — the agent reconnects automatically, no action needed |

---

## What's Next?

- **[Orthrus guide](../features/orthrus.md)** — More detail on the agent, install methods, and the read-only safety filter
- **[Hecate guide](../features/hecate.md)** — All three connection modes explained, plus VPN provider setup
