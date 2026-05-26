---
title: Hecate — Tunnel & Pathway Manager
description: Choose how each remote server connects to Charon — direct, via agent, or through a VPN
category: features
---

# Hecate — Tunnel & Pathway Manager

Think of Hecate as a traffic controller standing at a crossroads. When Charon needs to reach a remote server, Hecate decides which road to take: type in the address directly, send a message through the Orthrus agent you installed, or route through your VPN.

You interact with Hecate every time you add a Remote Server — it's the part that asks "how do you want to connect?"

---

## When Do You Need Hecate?

**Direct Mode users:** You don't need to configure Hecate at all. Just type in the IP address or hostname and you're done.

**Agent Mode users (Orthrus):** Register an Orthrus agent first (see the [Orthrus guide](orthrus.md)). Hecate then fills in the connection address automatically — no IP address hunting required.

**Provider Mode users:** Your server is already on a VPN (like NetBird or Tailscale). Go to **Settings → Tunnel Providers**, add your VPN credentials there first, then Hecate can find your device on that network.

---

## The Three Connection Modes

When you add a Remote Server, you choose one of these modes:

| Mode | When To Use | What You Provide |
|---|---|---|
| **Direct** | The machine is on your local network or has a public IP | Hostname or IP address + port |
| **Agent** | You installed Orthrus on the remote machine | Just pick the agent from the list |
| **Provider** | The machine is already on a VPN, no agent needed | Pick the VPN provider + the device |

### Direct Mode

You know where the server is — type in its address. This is the simplest option. Use it for machines on your home network or servers with a static public IP.

### Agent Mode

You installed the [Orthrus agent](orthrus.md) on the remote machine. Select it from the dropdown, and Charon fills in the connection details automatically from the agent's network information. You never have to know the IP address.

### Provider Mode

Your remote machine is already connected to a VPN service like NetBird, Tailscale, Cloudflare Tunnel, or ZeroTier. Add your VPN credentials once in **Settings → Tunnel Providers**, then pick that provider and the specific device when configuring the Remote Server. No agent installation needed.

---

## Viewing Your Agents

Go to **Remote Agents** in the sidebar to see all your Orthrus agents at a glance. Each one shows a live status badge:

| Badge | Meaning |
|---|---|
| 🟢 Green | Connected and healthy |
| 🟡 Yellow | Connecting or experiencing delays |
| 🔴 Red | Unreachable |

Click any agent to see its details, change its name, view install instructions, or assign it to a tunnel provider.

---

## Supported Tunnel Providers

These VPN and tunnel services work with Provider Mode:

| Provider | What You Need |
|---|---|
| NetBird | NetBird API key |
| Tailscale | Tailscale API key |
| Cloudflare | Cloudflare Tunnel credentials |
| ZeroTier | ZeroTier network ID + node details |

To add a provider: **Settings → Tunnel Providers → Add Provider** → choose your type → enter credentials → save.

---

## Assigning a Tunnel to an Orthrus Agent

If your agent is on a VPN, you can tell Charon exactly where on that network it lives. After you do this, Charon remembers the address — so the next time you add a Remote Server using this agent, everything fills in automatically.

1. Go to **Remote Agents** and open the agent
2. Under **Network Assignment**, pick your Provider and the Device that represents your remote machine
3. Click **Save**

---

## Uptime Monitoring Integration

Remote servers managed through Orthrus agents work with [Uptime Monitoring](uptime-monitoring.md). Enable monitoring on any proxy host that routes to a remote container, and Charon will alert you if that service goes down — even if it's on a machine behind a firewall on the other side of the world.

---

## Troubleshooting

| Problem | Likely Cause | Fix |
|---|---|---|
| Provider shows an error state | Bad credentials or expired API key | Re-enter credentials in **Settings → Tunnel Providers** |
| Agent Mode address not filling in | No network assignment set on the agent | Open the agent → assign a Provider + Device → save |
| Tunnel keeps restarting | VPN provider is temporarily unreachable | This is normal — Hecate retries automatically with increasing delays |
| Device not listed in Provider Mode | Provider not yet configured | Add the provider in Settings first |

---

*Need to set up an Orthrus agent first? See the [Orthrus guide](orthrus.md).*
*Ready to connect a remote Docker host? Follow the [Remote Docker Setup Guide](../guides/remote-docker-setup.md).*
