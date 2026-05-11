# Manual Test Plan — Hecate Tunnel & Pathway Manager

**Feature**: Hecate Tunnel & Pathway Manager
**PR**: #983
**Branch**: feature/hecate
**Date**: 2026-04-28

---

## Scope

Functional validation of the tunnel manager UI and Orthrus agent provisioning flow. Automated E2E tests cover the happy paths. This plan targets edge cases, real provider integration, and accessibility.

---

## Prerequisites

- Charon running with a valid encryption key configured
- At minimum one remote server accessible over the network
- (Optional) Active Cloudflare account, Tailscale network, ZeroTier network, or NetBird workspace for provider-specific tests

---

## Test Cases

### TC-01: Add Direct Connection Server

1. Navigate to Remote Servers
2. Click **Add Server**
3. Select **Direct** as Connection Type
4. Enter a valid hostname and port
5. Save

**Expected**: Server appears in the list with a "Direct" badge in the Connection column.

---

### TC-02: Add Orthrus Agent Server (Provisioning)

1. Navigate to Remote Servers → Add Server
2. Select **Orthrus Agent** as Connection Type
3. Click **Provision New Agent**

**Expected**:
- Install wizard opens
- AUTH_KEY is displayed in a read-only input
- Warning "This key will only be shown once" is visible
- All five tabs are present (Docker Compose, Systemd, Tarball, Homebrew, Kubernetes)
- Each tab shows a snippet with `ch_orthrus_` prefixed key substituted
- Copy button for each snippet works and shows "Copied!" for 3 seconds
- Clicking **Done** closes the wizard

---

### TC-03: AUTH_KEY Not Re-Shown After Wizard Close

1. Complete TC-02
2. Navigate away and return to Remote Servers
3. Click on the provisioned agent

**Expected**: AUTH_KEY is not visible anywhere in the UI. There is no re-display of the one-time key.

---

### TC-04: Orthrus Agent Connection Status Badge

1. With a provisioned agent server in the list
2. Observe the Connection column when the agent is offline

**Expected**: Badge shows a stopped/disconnected state (not a "Direct" badge).

3. Start the Orthrus agent binary on the target server
4. Return to Remote Servers after a few seconds

**Expected**: Badge updates to show connected state.

---

### TC-05: Cloudflare Tunnel Creation

1. Add a new server with **Cloudflare Tunnel** connection type
2. Enter a Cloudflare API token with Tunnel permissions
3. Complete the wizard and save

**Expected**: Tunnel appears in the provider with a pending state, eventually moving to active.

---

### TC-06: Start/Stop Tunnel

1. Select an existing tunnel server
2. Use the Start/Stop controls

**Expected**: Badge state updates in the list after the operation completes. No page reload required.

---

### TC-07: Delete Tunnel

1. Stop an active tunnel
2. Delete it

**Expected**: Tunnel and server entry removed from the list. No orphaned resources visible.

---

### TC-08: Tunnel Log Viewer

1. Select an Orthrus agent server
2. Click **View Tunnel Logs**

**Expected**:
- Log viewer dialog opens
- `role="log"` and `aria-live="polite"` present (screen reader accessible)
- Pause button shows `aria-pressed="false"` initially
- Clicking Pause changes to `aria-pressed="true"` and logs stop scrolling
- Clear button clears the log display
- Closing the dialog returns focus to the trigger button

---

### TC-09: Keyboard Navigation — Connection Type Selector

1. Open **Add Server** form
2. Tab to the Connection Type selector
3. Use arrow keys to change selection

**Expected**: All connection types reachable by keyboard. No mouse required.

---

### TC-10: Orthrus Install Wizard — Keyboard Navigation

1. Open the Orthrus install wizard
2. Tab to the tab list
3. Use **ArrowRight** to navigate between tabs

**Expected**: Each tab activates and shows the correct snippet without mouse.

---

### TC-11: Escape Key Closes Dialogs

1. Open the Orthrus install wizard
2. Press **Escape**

**Expected**: Dialog closes and focus returns to the trigger element.

---

### TC-12: Encryption Key Not Set — Feature Disabled

1. Start Charon without an encryption key configured
2. Navigate to Remote Servers

**Expected**: The Hecate feature section is not available or shows an appropriate "encryption required" message. No data exposed.

---

## Regression Checks

- [ ] Existing Remote Hosts (proxy hosts) still list and work correctly
- [ ] Existing DNS providers unaffected
- [ ] Certificate management unaffected
- [ ] Navigation structure unchanged
- [ ] Dark mode renders tunnel badges correctly with sufficient contrast

---

## Out of Scope

- Tailscale, ZeroTier, NetBird provider flows (require live accounts)
- WebSocket log streaming under high-volume conditions
- Multi-instance Charon deployments
