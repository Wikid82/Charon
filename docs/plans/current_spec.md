# Plan: Orthrus/Hecate Docs + FeedbackWidget Docs Link

**Status:** Draft — Pending Review
**Date:** 2025-06
**Scope:** Documentation authoring (ELI5 feature pages) + Frontend widget enhancement

---

## 1. Introduction

### Overview

This plan covers two independent, non-blocking deliverables:

1. **Docs Deliverable** — Write ELI5-level documentation files for two undocumented features (Orthrus and Hecate) and a remote Docker setup guide, fix a broken link, and update index/nav entries to surface these pages.
2. **Widget Deliverable** — Add a "View Documentation" third link to the floating `FeedbackWidget` React component so users can navigate to the docs site directly from anywhere in the UI.

### Objectives

- Close the broken `features/hecate.md` reference in `docs/features.md` line 226.
- Create `docs/features/orthrus.md` — dedicated ELI5 explainer for the Orthrus tunnel agent.
- Create `docs/features/hecate.md` — ELI5 explainer for the Hecate Tunnel & Pathway Manager.
- Create `docs/guides/remote-docker-setup.md` — step-by-step guide for connecting a remote HomeLab/server via Orthrus.
- Update `docs/index.md` to surface these three new pages.
- Update `docs/features.md` to add an Orthrus entry and fix the Hecate link.
- Add a third "View Docs" link to `FeedbackWidget.tsx` with full i18n and accessibility support.
- Update `frontend/src/components/__tests__/FeedbackWidget.test.tsx` to cover the new link.
- Update `frontend/src/locales/en/translation.json` with the new i18n keys.

---

## 2. Research Findings

### 2.1 Docs Site Architecture

- **Framework:** No `mkdocs.yml` was found anywhere in the repository. The docs site is authored as raw Markdown under `docs/` and served via GitHub Pages.
- **Base URL:** `https://wikid82.github.io/Charon/` (from `README.md` line 134).
- **Navigation:** Purely file-system-based relative links; there is no central nav config file to update.
- **Existing docs gaps:**
  - `docs/features/hecate.md` — **does not exist** but is linked from `docs/features.md` line 226 — this is an active broken link (bug fix).
  - `docs/features/orthrus.md` — does not exist, no link yet.
  - `docs/guides/remote-docker-setup.md` — does not exist, no link yet.

### 2.2 Orthrus System

- **Package:** `backend/internal/orthrus/`
- **What it is:** A reverse-WebSocket tunnel agent system. An `OrthrusAgent` binary runs on the remote machine, connects outbound via WebSocket to Charon's management interface, and multiplexes streams over yamux. Charon uses these multiplexed streams to talk to Docker on the remote machine.
- **Why it exists:** Remote Docker hosts behind NAT/firewalls cannot accept inbound TCP connections. Orthrus flips the direction — the remote agent dials outward to Charon.
- **Muzzle filter (`muzzle.go`):** Restricts Docker API access to a read-only allowlist (`/containers/json`, `/images/json`, `/_ping`, `/info`, `/version`, `/events`, `/volumes`, `/networks`, `/system/df`). Dynamic read-only patterns: `/containers/*/json`, `/containers/*/logs`, `/containers/*/stats`, `/containers/*/top`. All non-GET methods blocked (except HEAD `/_ping`). HTTP 403 for disallowed paths.
- **Key model fields (`models/orthrus_agent.go`):**
  - `UUID`, `Name`, `Status` — `OrthrusStatus`: "online" / "offline" / "pending"
  - `AuthKeyHash` — bcrypt hash; `json:"-"` (never exposed); plain key shown once at provisioning, prefixed `ch_orthrus_`
  - `Capabilities` — JSON array, e.g. `["docker", "tcp:5432"]`
  - `AgentCertPEM` — mTLS cert from Charon's internal CA
  - `HecateTunnelUUID` — links agent to a Hecate tunnel provider
  - `ResolvedAddress` — cached connectivity address
  - `ExternalProxyPort` — TCP port for inter-container Docker API access (0 = disabled)
  - `LastHeartbeat`, `LastSeen`
- **Install surfaces (`snippets.go`):** Docker Compose, systemd, tarball, Homebrew, Kubernetes DaemonSet — delivered via `GET /orthrus/agents/:uuid/snippets`.
- **REST API (`orthrus_handler.go`):**
  - `GET /management/orthrus/agents` — list agents
  - `POST /management/orthrus/agents` — provision (returns one-time auth key)
  - `GET /management/orthrus/agents/:uuid` — get one agent
  - `PATCH /management/orthrus/agents/:uuid` — update
  - `DELETE /management/orthrus/agents/:uuid` — delete
  - `POST /management/orthrus/agents/:uuid/revoke` — revoke auth key
  - `GET /management/orthrus/agents/:uuid/snippets` — install instructions
  - `GET /management/orthrus/agents/:uuid/proxy-status` — live external proxy state
- **WebSocket endpoint:** `GET /api/v1/ws/orthrus/connect` — Bearer token auth (bcrypt), HeartbeatTimeout 10 seconds.
- **`RemoteServer` linkage:** `ConnectionTypeOrthrus = "orthrus"` in `models/remote_server.go`; `OrthrusAgentUUID *string` field links a host config to its agent.

### 2.3 Hecate System

- **Package:** `backend/internal/hecate/`
- **What it is:** The Tunnel & Pathway Manager. Manages third-party tunneling providers (Cloudflare, Tailscale, ZeroTier, NetBird) and integrates the Orthrus agent protocol. `TunnelManager` supervises lifecycle of all active tunnel providers with exponential backoff restart (5s → 10s → 30s → 60s).
- **Currently registered provider:** `netbird` (`NewHecateService` in `services/hecate_service.go`). Architecture supports cloudflare, tailscale, zerotier via `RegisterFactory()`.
- **`HecateService`:** CRUD for `TunnelConfig` records; delegates start/stop to `TunnelManager`. Credentials encrypted AES-GCM before DB storage. If `IsActive=true` at creation, tunnel starts immediately.
- **Connection modes (from `docs/features.md`):**
  - **Direct** — manual hostname/IP
  - **Agent** — pick an Orthrus agent; address resolved from `OrthrusAgent.ResolvedAddress`
  - **Provider** — pick a VPN tunnel device directly (no agent required)
- **Relationship to Orthrus:** Each Orthrus agent can be assigned a `HecateTunnelUUID` pointing to a provider tunnel, giving it a `ResolvedAddress`. Remote Servers then use `ConnectionTypeOrthrus`.

### 2.4 FeedbackWidget

- **File:** `frontend/src/components/FeedbackWidget.tsx`
- **Current links:** 2 — "Report a Bug" (`GITHUB_BUG_URL`) and "Request a Feature" (`GITHUB_FEATURE_URL`)
- **Structure:** `<nav aria-label={...}>` containing `<a>` elements. First link has `ref={firstLinkRef}` for focus-on-open. Second link has `border-t` separator class.
- **Icons:** `Bug`, `Sparkles` from `lucide-react`. Trigger uses `MessageSquarePlus`.
- **i18n:** `useTranslation()` reads from `frontend/src/locales/en/translation.json` at `feedback.*` namespace.
- **Tests:** 15 tests in `frontend/src/components/__tests__/FeedbackWidget.test.tsx` — trigger, nav, link URLs, keyboard (Escape), focus management, backdrop.
- **Docs URL:** `https://wikid82.github.io/Charon/`

### 2.5 ELI5 Tone Reference

From existing docs (`getting-started.md`, `docker-integration.md`): plain English, short sentences, analogies, reassuring tone, explains *why* before *how*. Tables for comparisons. Numbered steps for procedures.

---

## 3. Technical Specifications

### 3.1 New File: `docs/features/orthrus.md`

**Purpose:** ELI5 explanation of Orthrus — what it is, why someone needs it, how to set it up.
**Target reader:** Self-hosters who want to manage Docker containers on a machine they cannot directly reach.

**Outline:**

```
---
title: Orthrus — Remote Tunnel Agent
description: Connect to Docker on a remote machine through a secure outbound tunnel
category: features
---

# Orthrus — Remote Tunnel Agent

[Opening analogy: your HomeLab server is behind a locked door; Orthrus is
a messenger installed there that knocks on Charon's door from the inside —
no key to the front door required.]

## What Problem Does Orthrus Solve?
- Remote machine is behind NAT / firewall / no public IP
- You cannot open a port on it
- You still want Charon to discover its Docker containers

## How It Works (Plain English)
1. Orthrus agent runs on your remote machine
2. Agent dials outbound to Charon (no inbound ports needed on remote)
3. Charon multiplexes Docker API calls back through that connection
4. Result: Charon sees remote containers as if they were local

Disconnections are handled automatically — the agent reconnects on its own with no action required.

## What Orthrus Can (and Cannot) Do
- CAN: List running containers, images, networks, volumes
- CANNOT: Create, delete, restart, or modify anything
- The "Muzzle" filter enforces this at every request — it is not configurable

## Setting Up an Orthrus Agent
1. In Charon go to Remote Agents (sidebar)
2. Click Add Agent — give it a friendly name
3. Copy the one-time auth key (it starts with ch_orthrus_)
> ⚠️ **Save this key now.** It starts with `ch_orthrus_` and is shown **once only**. If you lose it, delete the agent and create a new one.
4. Choose your install method (Docker Compose is easiest)
5. Paste the snippet + auth key on your remote machine and run it
6. The agent appears as "Online" within seconds

## Install Methods
| Method           | Best For                          |
|------------------|-----------------------------------|
| Docker Compose   | Servers already running Docker    |
| systemd          | Bare-metal Linux servers          |
| Kubernetes       | K8s clusters, DaemonSet per node  |
| Homebrew         | macOS machines                    |
| Tarball          | Any Linux, no package manager     |

## Agent Status Reference
| Status  | Meaning                               | What To Do                      |
|---------|---------------------------------------|---------------------------------|
| online  | Connected and healthy                 | Nothing — you're good           |
| offline | Lost connection or not started        | Check the agent is running      |
| pending | Registered but never connected        | Run the install snippet         |

## After Setup
→ Continue to the [Remote Docker Setup Guide](../guides/remote-docker-setup.md)

## Troubleshooting
| Problem                     | Likely Cause                        | Fix                             |
|-----------------------------|-------------------------------------|---------------------------------|
| Agent stays "pending"       | Snippet not run yet                 | Run it on the remote machine    |
| Agent shows "offline"       | Agent process stopped               | Restart agent service           |
| Agent "offline" after reboot| Not configured to start on boot     | Use systemd or Docker restart:always |
| Auth key lost               | Closed the page without saving      | Delete agent, create a new one  |
```

---

### 3.2 New File: `docs/features/hecate.md`

**Purpose:** ELI5 explanation of Hecate — the UI layer that manages how Charon reaches remote servers.
**Target reader:** Users configuring Remote Servers and choosing between connectivity options.

**Outline:**

```
---
title: Hecate — Tunnel & Pathway Manager
description: Choose how each remote server connects to Charon
category: features
---

# Hecate — Tunnel & Pathway Manager

[Opening analogy: Hecate is the traffic controller at a crossroads —
it decides which road each remote server uses to reach Charon.]

## What Is Hecate?
The UI that manages all "how do I reach this server?" decisions.
When you add a Remote Server, Hecate is what lets you pick Direct / Agent / Provider.

## When Do You Need Hecate?
- **Direct Mode users:** You don't need to configure Hecate at all — just type in the IP.
- **Agent Mode users:** Register an agent in Orthrus first; Hecate fills in the address automatically.
- **Provider Mode users:** Hecate is the place to add your VPN credentials before you can use them.

## The Three Connection Modes

| Mode     | When To Use                                    | What You Provide              |
|----------|------------------------------------------------|-------------------------------|
| Direct   | You know the server's hostname or IP           | Hostname / IP + port          |
| Agent    | You installed Orthrus on the remote machine    | Pick the Orthrus agent        |
| Provider | Server is already on a VPN, no agent needed    | Pick provider + device        |

### Direct Mode
Type in the address. That's it. Use this when the machine is on your local
network or has a public IP.

### Agent Mode (Orthrus)
Pick an Orthrus agent from the list. Charon fills in the connection address
automatically from the agent's network assignment. You don't need to know
the IP.

### Provider Mode
Your server is already reachable via a VPN tunnel (NetBird, Tailscale, etc.).
Pick the provider and the device on it. No agent installation required.

## Supported Tunnel Providers

| Provider    | Notes                                      |
|-------------|---------------------------------------------|
| NetBird     | Fully integrated — add API key to activate |
| Tailscale   | Supported — requires Tailscale API key     |
| Cloudflare  | Supported — Cloudflare Tunnel credentials  |
| ZeroTier    | Supported — ZeroTier network + node ID     |

## Managing Providers
- Where to find: **Settings → Tunnel Providers**
- Click **Add Provider** → choose type → enter credentials → save
- Each provider card shows its configured tunnels; click the settings icon to edit

## Assigning a Tunnel to an Orthrus Agent
This tells Charon where on a VPN network your agent lives.

1. Go to Remote Agents and open the agent
2. Under **Network Assignment**, pick a Provider and a Device
3. Save — Charon caches the resolved address
4. When you create a Remote Server using this agent, the address fills in automatically

## Live Status Badges
Every Remote Server shows a connection health indicator:
- 🟢 Green — connected and healthy
- 🟡 Yellow — connecting or degraded
- 🔴 Red — unreachable

## Troubleshooting

| Problem                          | Likely Cause                        | Fix                                  |
|----------------------------------|-------------------------------------|--------------------------------------|
| Provider shows error state       | Bad credentials or API key          | Re-enter credentials in Settings     |
| Agent address not resolved       | No network assignment set           | Assign provider + device to the agent|
| Tunnel keeps restarting          | Provider unreachable                | Check VPN credentials; backoff is normal |
| "Provider" mode device not listed| Provider not configured             | Add provider in Settings first       |
```

---

### 3.3 New File: `docs/guides/remote-docker-setup.md`

**Purpose:** Complete step-by-step walkthrough to connect a remote HomeLab server's Docker to Charon using Orthrus + Hecate.
**Target reader:** First-time users going through the full remote Docker setup flow.

**Outline:**

```
---
title: Connecting a Remote Docker Host
description: Step-by-step guide to managing Docker on a remote machine through Charon
category: guides
---

# Connecting a Remote Docker Host

[Opening: "Your HomeLab is in the basement. Charon is in the cloud.
This guide connects them safely without opening any ports."]

## Before You Start

Checklist:
- [ ] Charon is running and accessible
- [ ] Remote machine has Docker installed
- [ ] Remote machine can reach the internet (outbound HTTPS)
- [ ] You have terminal/SSH access to the remote machine

## Step 1: Register an Orthrus Agent in Charon

1. In Charon sidebar click **Remote Agents**
2. Click **Add Agent**
3. Enter a name (e.g. "HomeLab Server")
4. Click **Create**
5. **Copy the auth key now** — it starts with `ch_orthrus_` and is shown only once

> Need more detail? → [Orthrus feature guide](../features/orthrus.md)

## Step 2: Install the Agent on Your Remote Machine

The easiest method is Docker Compose. On your remote machine:

1. Copy the Docker Compose snippet from the Charon UI (Agent → Install → Docker Compose)
2. Replace `<AUTH_KEY>` with the key you copied
3. Run:
   ```bash
   docker compose up -d
   ```
4. The agent starts and dials out to Charon.

> Other methods (systemd, Kubernetes, tarball) are available in the Install tab.

## Step 3: Verify the Agent Is Online

Back in Charon → Remote Agents. Your agent should show **Online** within 10–30 seconds.

If it stays "pending", the agent hasn't connected yet — double-check the auth key and that the container is running.

## Step 4: (Optional) Assign a Tunnel Provider

Do this step if your remote machine **doesn't** have a direct public IP — for example it's on a VPN (NetBird, Tailscale, etc.) or behind a NAT.

1. Open the agent in Charon → **Network Assignment**
2. Pick your VPN provider and the device that represents your remote machine
3. Save — Charon stores the resolved address automatically

> Not on a VPN? Skip to Step 5.
> Need to set up a provider first? → [Hecate guide](../features/hecate.md)

## Step 5: Add a Remote Server

1. Go to **Settings → Docker** (or Remote Servers)
2. Click **Add Remote Host**
3. Set Connection Mode to **Agent**
4. Choose the Orthrus agent you registered
5. Click **Test Connection** — you should see a success response
6. Click **Save**

## Step 6: Use Your Remote Server

Your remote machine now appears as a Docker source in Charon:

1. Go to **Hosts → Add Host**
2. Click **Select from Docker**
3. In the host dropdown, pick your remote server
4. Browse and select any container running there

Done. Charon proxies through the Orthrus tunnel to reach it.

## Troubleshooting

| Problem                         | Likely Cause                  | Fix                                        |
|---------------------------------|-------------------------------|--------------------------------------------|
| Agent stays "pending"           | Snippet not run               | Run docker compose up -d on remote         |
| Test Connection fails           | Agent offline                 | Check agent container is running           |
| No containers listed            | Docker socket not mounted     | Add /var/run/docker.sock volume to agent   |
| Auth key expired / lost         | Page closed before saving     | Delete agent, register a new one           |
| Tunnel keeps disconnecting      | Network instability           | Normal — agent reconnects automatically    |
```

---

### 3.4 Changes to `docs/index.md`

Add a "Remote Access" section after "For Developers" and before "Need Help":

```markdown
---

## 🌐 Remote Access

**[Orthrus Agent Setup](features/orthrus.md)** — Tunnel into a remote machine's Docker without open ports
**[Hecate Connection Manager](features/hecate.md)** — Choose how each remote server reaches Charon
**[Remote Docker Guide](guides/remote-docker-setup.md)** — Step-by-step: connect a HomeLab server

---
```

Exact insertion point: after the closing `---` of the "For Developers" section (after the `database-schema.md` link line).

---

### 3.4.1 Cross-Reference: `docs/features/uptime-monitoring.md`

At the end of `docs/features/uptime-monitoring.md`, append one cross-reference line:

```markdown
*Monitoring a service on a remote Docker host? See [Connecting a Remote Docker Host](../guides/remote-docker-setup.md).*
```

---

### 3.5 Changes to `docs/features.md`

Add the following block immediately **before** the `### 🔀 Hecate Tunnel & Pathway Manager` heading (currently around line 207):

```markdown
### 🐾 Orthrus — Remote Tunnel Agent

Your HomeLab behind a firewall? Orthrus is a small agent you install on any remote machine. It dials outward to Charon over a secure WebSocket — no open inbound ports required. Once connected, Charon can discover and proxy Docker containers on that machine just like local ones.

→ [Learn More](features/orthrus.md)

---

```

The existing `→ [Learn More](features/hecate.md)` on line 226 does not need text changes — the broken destination is fixed by creating the file.

---

### 3.6 FeedbackWidget — `frontend/src/components/FeedbackWidget.tsx`

**Change 1 — Import:**
```typescript
// Before:
import { MessageSquarePlus, Bug, Sparkles } from 'lucide-react'

// After:
import { MessageSquarePlus, Bug, Sparkles, BookOpen } from 'lucide-react'
```

**Change 2 — Constant (after `GITHUB_FEATURE_URL`):**
```typescript
const DOCS_URL = 'https://wikid82.github.io/Charon/'
```

**Change 3 — Third link element** (append inside `<nav>`, after the Feature Request `</a>`):
```tsx
<a
  href={DOCS_URL}
  target="_blank"
  rel="noopener noreferrer"
  aria-label={t('feedback.viewDocsAriaLabel')}
  className={cn(
    'flex items-center gap-3 px-4 py-3 text-sm',
    'border-t border-gray-200 dark:border-gray-800',
    'text-gray-700 dark:text-gray-300',
    'hover:bg-gray-100 dark:hover:bg-gray-800',
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
    'transition-colors'
  )}
>
  <BookOpen className="w-4 h-4 shrink-0" aria-hidden="true" />
  <div>
    <div className="font-medium">{t('feedback.viewDocs')}</div>
    <div className="text-xs text-gray-500 dark:text-gray-400">
      {t('feedback.viewDocsDescription')}
    </div>
  </div>
</a>
```

---

### 3.7 i18n — Locale Files (All Five Languages)

Add three keys inside the `"feedback"` object (after `requestFeatureAriaLabel`) to **all five locale files**:

- `frontend/src/locales/en/translation.json`
- `frontend/src/locales/de/translation.json`
- `frontend/src/locales/fr/translation.json`
- `frontend/src/locales/zh/translation.json`
- `frontend/src/locales/es/translation.json`

For `de`, `fr`, `zh`, and `es`: use the same English values as placeholders. Human translation is a separate backlog task and must not block this PR.

The three keys to add:

```json
"viewDocs": "View Documentation",
"viewDocsDescription": "Read the docs",
"viewDocsAriaLabel": "View documentation (opens docs site in new tab)"
```

Final `"feedback"` object (12 keys total):

```json
"feedback": {
  "triggerLabel": "Open feedback menu",
  "closeTriggerLabel": "Close feedback menu",
  "panelLabel": "Feedback options",
  "reportBug": "Report a Bug",
  "reportBugDescription": "Found an issue?",
  "reportBugAriaLabel": "Report a bug (opens GitHub Issues in new tab)",
  "requestFeature": "Request a Feature",
  "requestFeatureDescription": "Have an idea?",
  "requestFeatureAriaLabel": "Request a feature (opens GitHub Issues in new tab)",
  "viewDocs": "View Documentation",
  "viewDocsDescription": "Read the docs",
  "viewDocsAriaLabel": "View documentation (opens docs site in new tab)"
}
```

---

### 3.8 Tests — `frontend/src/components/__tests__/FeedbackWidget.test.tsx`

Add `DOCS_URL` constant and three new tests to the existing `describe` block:

```typescript
const DOCS_URL = 'https://wikid82.github.io/Charon/'

// 16. Panel contains docs link pointing to correct URL
it('panel contains docs link pointing to correct URL', async () => {
  render(<FeedbackWidget />)
  const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
  await userEvent.click(trigger)
  const docsLink = screen.getByRole('link', { name: /view documentation/i })
  expect(docsLink).toHaveAttribute('href', DOCS_URL)
})

// 17. Docs link has target="_blank"
it('docs link has target="_blank"', async () => {
  render(<FeedbackWidget />)
  const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
  await userEvent.click(trigger)
  const docsLink = screen.getByRole('link', { name: /view documentation/i })
  expect(docsLink).toHaveAttribute('target', '_blank')
})

// 18. Docs link has rel="noopener noreferrer"
it('docs link has rel="noopener noreferrer"', async () => {
  render(<FeedbackWidget />)
  const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
  await userEvent.click(trigger)
  const docsLink = screen.getByRole('link', { name: /view documentation/i })
  expect(docsLink).toHaveAttribute('rel', 'noopener noreferrer')
})

// 19. Docs link has aria-label from i18n key
it('docs link has aria-label from i18n key', async () => {
  render(<FeedbackWidget />)
  await userEvent.click(screen.getByRole('button', { name: 'Open feedback menu' }))
  const docsLink = screen.getByRole('link', { name: /view documentation/i })
  expect(docsLink).toHaveAttribute('aria-label', 'View documentation (opens docs site in new tab)')
})
```

---

### 3.9 Accessibility Notes

- New `<a>` element follows the identical pattern as existing links: explicit `aria-label`, `target="_blank"`, `rel="noopener noreferrer"`, `focus-visible` ring classes.
- `BookOpen` carries `aria-hidden="true"` — accessible name from `aria-label` on the anchor.
- `firstLinkRef` stays on the Bug Report link; Tab order flows naturally: Bug → Feature → Docs.
- WCAG 2.2 AA satisfied: sufficient contrast (shared classes), keyboard operability, screen-reader label.

---

## 4. Implementation Plan

### Phase 1: Playwright Tests (Spec Validation)

Write failing tests first to define expected widget behaviour.

| # | File | Action |
|---|------|--------|
| 1 | `frontend/src/components/__tests__/FeedbackWidget.test.tsx` | Add tests 16–19 (docs link URL, target, rel, aria-label) |

**Validation gate:** Tests run and fail (implementation not yet present — expected).

---

### Phase 2: Documentation Files

All docs work; zero backend/frontend code changes.

| # | File | Action |
|---|------|--------|
| 2 | `docs/features/orthrus.md` | Create — full ELI5 feature page per outline in §3.1 |
| 3 | `docs/features/hecate.md` | Create — full ELI5 feature page per outline in §3.2 (fixes broken link) |
| 4 | `docs/guides/remote-docker-setup.md` | Create — step-by-step guide per outline in §3.3 |
| 5 | `docs/features.md` | Insert Orthrus entry before Hecate section (§3.5) |
| 6 | `docs/index.md` | Add "Remote Access" section (§3.4) |
| 6a | `docs/features/uptime-monitoring.md` | Append cross-reference line per §3.4.1 |

**Validation gate:** All relative links in new files resolve to real files. `features/hecate.md` link in `features.md` is no longer broken.

---

### Phase 3: Frontend Widget

| # | File | Action |
|---|------|--------|
| 7 | `frontend/src/locales/en/translation.json` | Add three `viewDocs*` keys per §3.7 |
| 7a | `frontend/src/locales/de/translation.json` | Add three `viewDocs*` keys (English placeholders) per §3.7 |
| 7b | `frontend/src/locales/fr/translation.json` | Add three `viewDocs*` keys (English placeholders) per §3.7 |
| 7c | `frontend/src/locales/zh/translation.json` | Add three `viewDocs*` keys (English placeholders) per §3.7 |
| 7d | `frontend/src/locales/es/translation.json` | Add three `viewDocs*` keys (English placeholders) per §3.7 |
| 8 | `frontend/src/components/FeedbackWidget.tsx` | Import `BookOpen`, add `DOCS_URL`, add third link per §3.6 |

**Validation gate:** Tests 16–19 pass; existing 15 tests still pass.

---

### Phase 4: Integration Check

| # | Action |
|---|--------|
| 9 | Run `npm run test -- --reporter=verbose` — all 19 FeedbackWidget tests green, zero regressions |
| 10 | Manual browser verify: FeedbackWidget renders 3 links when open |
| 11 | Spot-check: `orthrus.md`, `hecate.md`, `remote-docker-setup.md` render correctly in browser |

---

## 5. Acceptance Criteria

### Docs

- [ ] `docs/features/orthrus.md` exists; ELI5 tone with analogy, setup steps, troubleshooting table
- [ ] `docs/features/hecate.md` exists; fixes the broken reference at `docs/features.md` line 226
- [ ] `docs/guides/remote-docker-setup.md` exists; numbered steps, auth-key warning prominent
- [ ] `docs/features.md` has a standalone Orthrus entry linking to `features/orthrus.md`
- [ ] `docs/index.md` has "Remote Access" section with all three new page links
- [ ] No broken internal relative links introduced or left unfixed
- [ ] All new docs: plain English, analogy at start of new concepts, jargon immediately explained

### Widget

- [ ] `FeedbackWidget.tsx` renders exactly 3 links when panel is open
- [ ] Third link `href` = `https://wikid82.github.io/Charon/`
- [ ] Third link has `target="_blank"` and `rel="noopener noreferrer"`
- [ ] Third link `aria-label` uses resolved value of `feedback.viewDocsAriaLabel`
- [ ] `BookOpen` icon has `aria-hidden="true"`
- [ ] `translation.json` contains all three new `feedback.viewDocs*` keys
- [ ] All 19 FeedbackWidget unit tests pass; zero regressions
- [ ] `npm run test` exits 0

---

## 6. Commit Slicing Strategy

**Decision:** Single PR with two ordered logical commits. The deliverables are independent; separate commits make each one reviewable and rollback-safe in isolation.

### Commit 1 — `docs: add Orthrus, Hecate, and remote Docker documentation`

**Scope:** Docs-only. No frontend or backend changes.

**Files:**
- `docs/features/orthrus.md` (new)
- `docs/features/hecate.md` (new — fixes broken link)
- `docs/guides/remote-docker-setup.md` (new)
- `docs/features.md` (modified — add Orthrus entry)
- `docs/index.md` (modified — add Remote Access section)

**Dependencies:** None — purely additive Markdown.

**Validation gate:** All internal links in new files resolve; `features.md` Hecate link target exists.

**Rollback:** Revert this commit with zero code impact.

---

### Commit 2 — `feat(feedback-widget): add View Documentation link`

**Scope:** Frontend only. No backend or docs changes.

**Files:**
- `frontend/src/locales/en/translation.json` (modified — 3 new keys)
- `frontend/src/locales/de/translation.json` (modified — 3 new keys, English placeholders)
- `frontend/src/locales/fr/translation.json` (modified — 3 new keys, English placeholders)
- `frontend/src/locales/zh/translation.json` (modified — 3 new keys, English placeholders)
- `frontend/src/locales/es/translation.json` (modified — 3 new keys, English placeholders)
- `frontend/src/components/FeedbackWidget.tsx` (modified — import, constant, link element)
- `frontend/src/components/__tests__/FeedbackWidget.test.tsx` (modified — 4 new tests)

**Dependencies:** Commit 1 should land first (so the target URL exists on the published site), but technically independent — URL is hardcoded.

**Validation gate:** 19 FeedbackWidget tests pass; no regressions.

**Rollback:** Revert this commit alone with zero docs or backend impact.

---

### Contingency

If `BookOpen` is unavailable in the installed lucide-react version, substitute `ExternalLink` or `FileText` — the same structural pattern applies.

---

## 7. Notes for Doc Writer Agent

### ELI5 Writing Guidelines

1. **Lead with why, not what.** "Your HomeLab is behind a firewall — Orthrus solves that" before "Orthrus is a reverse WebSocket tunnel."
2. **Open each new concept with an analogy.** Examples:
   - Orthrus: "a messenger that knocks from the inside"
   - Hecate: "a traffic controller at a crossroads"
   - Muzzle filter: "Orthrus can look but not touch"
3. **Short sentences.** One idea per sentence.
4. **No jargon without an immediate plain-English follow-up.** If "WebSocket" must appear: "(a type of live internet connection)".
5. **Procedures are numbered**, not bulleted.
6. **Use tables for comparisons** — install methods, status values, provider types.
7. **Reassuring tone.** "Don't worry if this looks complex — you only need to do it once."
8. **Make the one-time auth key warning impossible to miss.** Bold it, use a callout block or ⚠️ emoji.
9. **Security note on Muzzle is a trust-builder** — mention it explicitly. Users worry about granting Docker access to a third-party tool.
10. **Section order convention** (matches existing feature pages):
    1. Front-matter
    2. `# Title`
    3. Overview / analogy paragraph
    4. How It Works
    5. Setup steps (numbered)
    6. Reference tables
    7. Troubleshooting table
