# Navigation Restructure: Hecate Section + Cerberus Rebrand

**Branch**: `feature/hecate`
**Type**: UI Navigation Restructure (frontend-only)
**PR Scope**: Two logical commits, one PR

---

## 1. Introduction

### Objectives

1. **Part 1 — Cerberus rebrand**: Rename the sidebar header "Security" → "Cerberus" to align the UI label with the internal feature flag name (`feature.cerberus.enabled`). No route or page changes — only the nav label and its downstream references.

2. **Part 2 — Hecate collapsible section**: Convert the current flat "Hecate" sidebar link into a collapsible accordion group with four distinct sub-pages: Remote Servers, Tunnels, Providers, and Agent. This wires up all existing but unrouted Hecate components.

### Current State (feature/hecate branch)

| Item | Current | Target |
|---|---|---|
| Sidebar "Security" | Flat collapsible group, label = "Security" | Label = "Cerberus" (same path/children) |
| Sidebar "Remote Servers" | Flat link at `/remote-servers` | Moves into Hecate section at `/hecate/remote-servers` |
| Sidebar "Hecate" | Flat link at `/hecate` (no route in App.tsx) | Collapsible group with 4 children |
| `Hecate.tsx` | Monolithic page: tunnels table + agent management | Split: `HecateTunnels.tsx` + `HecateAgent.tsx` |
| `HecateProviders.tsx` | Does not exist | New page |
| App.tsx `/hecate` route | Not registered | Registered with all 4 child routes |

---

## 2. Research Findings

### 2.1 Layout.tsx (feature/hecate branch — current filesystem state)

Navigation array (relevant excerpt, lines ~70–100):
```
{ name: t('navigation.remoteServers'), path: '/remote-servers', icon: '🖥️' },
{ name: t('navigation.hecate'), path: '/hecate', icon: '🔗' },           // flat, no children
...
{ name: t('navigation.security'), path: '/security', icon: '🛡️', children: [
  { name: t('navigation.dashboard'),      path: '/security', ... },
  { name: t('navigation.crowdsec'),       path: '/security/crowdsec', ... },
  { name: t('navigation.accessLists'),    path: '/security/access-lists', ... },
  { name: t('navigation.rateLimiting'),   path: '/security/rate-limiting', ... },
  { name: t('navigation.waf'),            path: '/security/waf', ... },
  { name: t('navigation.securityHeaders'),path: '/security/headers', ... },
  { name: t('navigation.encryption'),     path: '/security/encryption', ... },
]},
```

Feature flag filter (lines ~116–120):
```js
if (item.name === t('navigation.uptime'))   return featureFlags?.['feature.uptime.enabled'] !== false
if (item.name === t('navigation.security')) return featureFlags?.['feature.cerberus.enabled'] !== false
```

### 2.2 App.tsx (feature/hecate branch — current filesystem state)

- `RemoteServers` lazy import exists; route `<Route path="remote-servers" element={<RemoteServers />} />` is registered.
- **No Hecate lazy import. No `/hecate` routes registered anywhere.**
- Security routes: all registered as flat siblings under the layout Route — not nested inside a Security parent route.

### 2.3 Existing Hecate Component Inventory

| File | Purpose | Reuse Target |
|---|---|---|
| `frontend/src/pages/Hecate.tsx` | Monolith: tunnel table + agent management | Split into HecateTunnels + HecateAgent |
| `frontend/src/pages/__tests__/Hecate.test.tsx` | Tests for the monolith | Orphaned after split — leave in place |
| `frontend/src/pages/RemoteServers.tsx` | Remote server CRUD with grid/list views | Reuse directly in `/hecate/remote-servers` route |
| `frontend/src/components/hecate/HecateTunnelForm.tsx` | Create/edit form for TunnelConfig | Used by HecateTunnels page |
| `frontend/src/components/hecate/TunnelLogViewer.tsx` | Live log viewer | Used by HecateTunnels page |
| `frontend/src/components/hecate/TunnelStatusBadge.tsx` | Status chip | Used by HecateTunnels page |
| `frontend/src/components/hecate/CloudflareTunnelWizard.tsx` | Cloudflare-specific wizard | Used by HecateProviders page |
| `frontend/src/components/hecate/ConnectionTypeSelector.tsx` | Provider type picker | Used by HecateProviders page |
| `frontend/src/components/hecate/OrthrusAgentManager.tsx` | Agent CRUD table | Used by HecateAgent page |
| `frontend/src/components/hecate/OrthrusInstallWizard.tsx` | Install snippet generator | Used by HecateAgent page |
| `frontend/src/components/hecate/TailscaleDevicePicker.tsx` | Tailscale device list | Used by HecateProviders page |
| `frontend/src/components/hecate/NetBirdPeerPicker.tsx` | NetBird peer list | Used by HecateProviders page |
| `frontend/src/components/hecate/ZeroTierMemberPicker.tsx` | ZeroTier member list | Used by HecateProviders page |
| `frontend/src/hooks/useHecate.ts` | Query/mutation hooks for tunnels | Used by HecateTunnels page |
| `frontend/src/hooks/__tests__/useHecate.test.tsx` | Hook tests | No change needed |
| `frontend/src/api/hecate.ts` | API client | No change needed |
| `frontend/src/api/__tests__/hecate.test.ts` | API tests | No change needed |

### 2.4 Translation Keys (Current State)

File: `frontend/src/locales/en/translation.json`, `"navigation"` block (line 53–86):

```json
"navigation": {
  "dashboard": "Dashboard",
  "proxyHosts": "Proxy Hosts",
  "remoteServers": "Remote Servers",
  "domains": "Domains",
  "certificates": "Certificates",
  "dns": "DNS",
  "dnsProviders": "DNS Providers",
  "security": "Security",
  "accessLists": "Access Lists",
  "crowdsec": "CrowdSec",
  "rateLimiting": "Rate Limiting",
  "waf": "WAF",
  "uptime": "Uptime",
  "notifications": "Notifications",
  "users": "Users",
  "tasks": "Tasks",
  "settings": "Settings",
  "system": "System",
  "email": "Email (SMTP)",
  "import": "Import",
  "caddyfile": "Caddyfile",
  "importNPM": "Import NPM",
  "importJSON": "Import JSON",
  "backups": "Backups",
  "logs": "Logs",
  "securityHeaders": "Security Headers",
  "expandSidebar": "Expand sidebar",
  "collapseSidebar": "Collapse sidebar",
  "encryption": "Encryption",
  "admin": "Admin",
  "plugins": "Plugins",
  "hecate": "Hecate"
}
```

**Missing keys (must add):**
- `navigation.cerberus` = `"Cerberus"` (Commit 1)
- `navigation.tunnels` = `"Tunnels"` (Commit 2)
- `navigation.providers` = `"Providers"` (Commit 2)
- `navigation.agent` = `"Agent"` (Commit 2)

**Note**: `navigation.security` is kept — it is used in the `security.*` namespace for page titles inside `Security.tsx` via `t('security.title')`. That is a different key namespace.

### 2.5 Backend Routes (No Changes Required)

All Hecate and Orthrus API routes are already registered in the backend (`/api/v1/hecate/`, `/api/v1/orthrus/`). This is a pure frontend-only change.

### 2.6 E2E Test Files Affected

| File | Why affected |
|---|---|
| `tests/core/navigation.spec.ts` | Checks nav items; Remote Servers moves from top-level to Hecate submenu |
| `tests/hecate-tunnel-manager.spec.ts` | May navigate to `/remote-servers` directly; needs route update |

---

## 3. Technical Specification

### 3.1 Route Architecture (Target State)

```
/ (Layout wrapper)
├── /                              (Dashboard)
├── /proxy-hosts                   (ProxyHosts)
├── /domains                       (Domains)
├── /certificates                  (Certificates)
├── /dns
│   ├── /dns/providers
│   └── /dns/plugins
├── /hecate                        → Navigate to /hecate/tunnels  [NEW]
│   ├── /hecate/tunnels            → HecateTunnels                [NEW PAGE]
│   ├── /hecate/remote-servers     → RemoteServers                [EXISTING PAGE, new path]
│   ├── /hecate/providers          → HecateProviders              [NEW PAGE]
│   └── /hecate/agent              → HecateAgent                  [NEW PAGE]
├── /remote-servers                → Navigate to /hecate/remote-servers  [LEGACY REDIRECT]
├── /security                      (Security — path unchanged)
│   ├── /security/crowdsec
│   ├── /security/access-lists
│   ├── /security/rate-limiting
│   ├── /security/waf
│   ├── /security/headers
│   └── /security/encryption
├── /uptime
├── /settings/...
└── /tasks/...
```

### 3.2 Sidebar Navigation (Target State)

```
Dashboard              /
Proxy Hosts            /proxy-hosts
Hecate                 /hecate  [collapsible]
  Remote Servers       /hecate/remote-servers
  Tunnels              /hecate/tunnels
  Providers            /hecate/providers
  Agent                /hecate/agent
Domains                /domains
Certificates           /certificates
DNS                    /dns  [collapsible]
  DNS Providers        /dns/providers
  Plugins              /dns/plugins
Uptime                 /uptime
Cerberus               /security  [collapsible — label changed from "Security"]
  Dashboard            /security
  CrowdSec             /security/crowdsec
  Access Lists         /security/access-lists
  Rate Limiting        /security/rate-limiting
  WAF                  /security/waf
  Security Headers     /security/headers
  Encryption           /security/encryption
Settings               /settings  [collapsible]
Tasks                  /tasks  [collapsible]
```

### 3.3 New Page Components

#### `frontend/src/pages/HecateTunnels.tsx`

**Type**: Extracted (refactored from `Hecate.tsx`)
**Responsibility**: Tunnel CRUD table with start/stop/delete/rotate-credentials actions, tunnel form dialog, log viewer dialog.
**Reuses**: `HecateTunnelForm`, `TunnelLogViewer`, `TunnelStatusBadge`, `useHecate` hook
**Does NOT include**: Agent/Orthrus management (moved to HecateAgent)

State to keep from `Hecate.tsx`:
- `formOpen`, `editingTunnel` — tunnel form open/close
- `logsTunnel` — which tunnel's logs to show
- `deleteTarget`, `deleteError`, `isConfirmDeleting` — delete confirm flow
- `rotateTarget`, `rotateValue`, `rotateError`, `isConfirmRotating` — rotate credentials flow

Handlers to keep from `Hecate.tsx`:
- `handleAddTunnel`, `handleEditTunnel`, `handleStart`, `handleStop`
- `handleDeleteConfirm`, `handleRotateConfirm`

Renders:
- `PageShell` with title `t('hecate.tunnels.title')`
- `DataTable` of tunnels with columns: name, provider, status, created date, actions
- Action buttons: start, stop, edit, view logs, rotate credentials, delete
- Dialogs: `HecateTunnelForm`, `TunnelLogViewer`, delete confirm, rotate confirm

#### `frontend/src/pages/HecateAgent.tsx`

**Type**: Extracted (refactored from `Hecate.tsx`)
**Responsibility**: Orthrus agent provisioning, management, and install wizard.
**Reuses**: `OrthrusAgentManager`, `OrthrusInstallWizard`, `useAgentList`, `useProvisionAgent`, `useOrthrus`
**Does NOT include**: Tunnel table

State to keep from `Hecate.tsx`:
- `provisionOpen`, `provisionName`, `provisionError`, `isProvisioning`
- `wizardData` (`{ agentName, agentUUID, authKey, snippets }`)

Renders:
- `PageShell` with title `t('hecate.agent.title')`
- "Provision New Agent" button
- `OrthrusAgentManager` (agent list table)
- Provision dialog (name input + submit)
- `OrthrusInstallWizard` (opens when `wizardData` is set after provision)

#### `frontend/src/pages/HecateProviders.tsx`

**Type**: New
**Responsibility**: Provider discovery overview and entry point for creating provider-specific tunnels.
**Reuses**: `CloudflareTunnelWizard`, `TailscaleDevicePicker`, `NetBirdPeerPicker`, `ZeroTierMemberPicker`, `HecateTunnelForm`

Design — 4 provider cards in a 2×2 grid:

| Provider Card | Shows | Action |
|---|---|---|
| Cloudflare | Count of existing Cloudflare tunnels | "New Tunnel" → opens `HecateTunnelForm` with `provider="cloudflare"` pre-selected |
| Tailscale | Count of existing Tailscale tunnels | "New Tunnel" → opens `HecateTunnelForm` with `provider="tailscale"` pre-selected |
| NetBird | Count of existing NetBird tunnels | "New Tunnel" → opens `HecateTunnelForm` with `provider="netbird"` pre-selected |
| ZeroTier | Count of existing ZeroTier tunnels | "New Tunnel" → opens `HecateTunnelForm` with `provider="zerotier"` pre-selected |

Counts come from `useHecate().tunnels.filter(t => t.provider === providerName).length`.

No global credential management — credentials are per-tunnel and managed inside `HecateTunnelForm`.

#### `frontend/src/pages/HecateRemoteServers.tsx` — NOT NEEDED

The existing `RemoteServers` component is registered directly on the `/hecate/remote-servers` route. No new wrapper file needed.

### 3.4 Translation Keys Delta

#### Commit 1 additions (to `"navigation"` block):
```json
"cerberus": "Cerberus"
```

#### Commit 2 additions (to `"navigation"` block):
```json
"tunnels": "Tunnels",
"providers": "Providers",
"agent": "Agent"
```

#### Commit 2 additions (new top-level namespace):
```json
"hecate": {
  "tunnels": {
    "title": "Tunnels",
    "description": "Manage tunnel connections"
  },
  "providers": {
    "title": "Providers",
    "description": "Configure provider credentials"
  },
  "agent": {
    "title": "Agent",
    "description": "Manage Orthrus agents"
  }
}
```

**Kept unchanged:**
- `"security": "Security"` — still used by `security.*` page title keys (different namespace)
- `"remoteServers": "Remote Servers"` — reused as child label in Hecate submenu
- `"hecate": "Hecate"` — used as collapsible group header

### 3.5 i18n Key Summary Table

| Key | Before | After | Commit |
|---|---|---|---|
| `navigation.security` | `"Security"` | `"Security"` (kept, page titles) | — |
| `navigation.cerberus` | absent | `"Cerberus"` | 1 |
| `navigation.hecate` | `"Hecate"` | `"Hecate"` | — |
| `navigation.remoteServers` | `"Remote Servers"` | `"Remote Servers"` | — |
| `navigation.tunnels` | absent | `"Tunnels"` | 2 |
| `navigation.providers` | absent | `"Providers"` | 2 |
| `navigation.agent` | absent | `"Agent"` | 2 |
| `hecate.tunnels.title` | absent | `"Tunnels"` | 2 |
| `hecate.providers.title` | absent | `"Providers"` | 2 |
| `hecate.agent.title` | absent | `"Agent"` | 2 |

### 3.6 Layout.tsx Filter Logic Delta

**Before (Layout.tsx line ~117):**
```js
if (item.name === t('navigation.security')) return featureFlags?.['feature.cerberus.enabled'] !== false
```

**After Commit 1:**
```js
if (item.name === t('navigation.cerberus')) return featureFlags?.['feature.cerberus.enabled'] !== false
```

The Hecate section has no feature flag gate (consistent with current flat Hecate item behavior).

### 3.7 App.tsx Lazy Import Delta

**Add (Commit 2):**
```tsx
const HecateTunnels  = lazy(() => import('./pages/HecateTunnels'))
const HecateAgent    = lazy(() => import('./pages/HecateAgent'))
const HecateProviders = lazy(() => import('./pages/HecateProviders'))
```

**Keep:**
```tsx
const RemoteServers = lazy(() => import('./pages/RemoteServers'))
```

**Note**: There is no `Hecate` lazy import to remove — it was never added to App.tsx on the feature/hecate branch.

### 3.8 App.tsx Route Delta (Commit 2)

**Remove:**
```tsx
<Route path="remote-servers" element={<RemoteServers />} />
```

**Add (within the layout children block, after `/domains`):**
```tsx
{/* Hecate Routes */}
<Route path="hecate">
  <Route index element={<Navigate to="/hecate/tunnels" replace />} />
  <Route path="tunnels"        element={<HecateTunnels />} />
  <Route path="remote-servers" element={<RemoteServers />} />
  <Route path="providers"      element={<HecateProviders />} />
  <Route path="agent"          element={<HecateAgent />} />
</Route>

{/* Legacy redirect for bookmarks and E2E tests */}
<Route path="remote-servers" element={<Navigate to="/hecate/remote-servers" replace />} />
```

---

## 4. Implementation Plan

### Phase 1: Playwright Tests (Write First — Red State Expected)

#### Task 1.1 — New file: `tests/core/cerberus-navigation.spec.ts`

```typescript
test.describe('Cerberus Navigation', () => {
  test('should display sidebar group label "Cerberus" not "Security"')
  test('should still navigate to /security when Cerberus group clicked')
  test('should show all security sub-pages under Cerberus group')
  test('should hide Cerberus when feature.cerberus.enabled is false')
})
```

#### Task 1.2 — New file: `tests/core/hecate-navigation.spec.ts`

```typescript
test.describe('Hecate Navigation', () => {
  test('should display Hecate as collapsible sidebar section')
  test('should expand Hecate to reveal 4 items: Remote Servers, Tunnels, Providers, Agent')
  test('should navigate to /hecate/tunnels when Tunnels clicked')
  test('should navigate to /hecate/remote-servers when Remote Servers clicked')
  test('should navigate to /hecate/providers when Providers clicked')
  test('should navigate to /hecate/agent when Agent clicked')
  test('should redirect /hecate to /hecate/tunnels')
  test('should redirect legacy /remote-servers to /hecate/remote-servers')
  test('should NOT show Remote Servers as a top-level sidebar link')
})
```

#### Task 1.3 — Update file: `tests/core/navigation.spec.ts`

In `'should display all main navigation items'` test step `'Verify common navigation items exist'`:
- Remove any `page.getByRole('link', { name: /remote.*servers/i })` top-level assertion
- After clicking Hecate accordion, assert Remote Servers link visible as submenu item

#### Task 1.4 — Update files: `tests/hecate-tunnel-manager.spec.ts` and `tests/modal-dropdown-triage.spec.ts`

- Replace any `page.goto('/remote-servers')` calls with `page.goto('/hecate/remote-servers')` in both files
- The legacy redirect will handle the runtime path, but both files should be updated to use canonical routes and avoid confusion in future CI failures

---

### Phase 2: Backend Implementation

No backend changes. Skip.

---

### Phase 3: Frontend Implementation

#### Commit 1 — Cerberus Rebrand (4 files)

**3.1** `frontend/src/locales/en/translation.json`

In the `"navigation"` block, after `"security": "Security"`:
```json
"cerberus": "Cerberus",
```

**3.2** `frontend/src/components/Layout.tsx`

Change 1 — nav array item name (the Security collapsible group `name` field):
```diff
- name: t('navigation.security'), path: '/security', icon: '🛡️', children: [
+ name: t('navigation.cerberus'), path: '/security', icon: '🛡️', children: [
```

Change 2 — feature flag filter condition:
```diff
- if (item.name === t('navigation.security')) return featureFlags?.['feature.cerberus.enabled'] !== false
+ if (item.name === t('navigation.cerberus')) return featureFlags?.['feature.cerberus.enabled'] !== false
```

**3.3** `frontend/src/pages/CrowdSecConfig.tsx`

Find the inline link that reads `t('navigation.security')` and update to `t('navigation.cerberus')`:
```diff
- <Link to="/security">{t('navigation.security')}</Link>
+ <Link to="/security">{t('navigation.cerberus')}</Link>
```
This prevents a visual inconsistency where the sidebar says "Cerberus" but an in-page link back to the section reads "Security".

**3.4** `frontend/src/components/__tests__/Layout.test.tsx`

7 locations to update (see exact test names in §5.1):

| Location | Before | After |
|---|---|---|
| `renders all navigation items` — button click | `findByRole('button', { name: /security/i })` | `findByRole('button', { name: /cerberus/i })` |
| `displays Security nav item when Cerberus is enabled` — assertion | `getByText('Security')` | `getByText('Cerberus')` |
| `hides Security nav item when Cerberus is disabled` — assertion | `queryByText('Security')` | `queryByText('Cerberus')` |
| `shows Security and Uptime when both features are enabled` — assertion | `getByText('Security')` | `getByText('Cerberus')` |
| `hides both Security and Uptime when both features are disabled` — assertion | `queryByText('Security')` | `queryByText('Cerberus')` |
| `defaults to showing Security and Uptime when feature flags are loading` — assertion | `getByText('Security')` | `getByText('Cerberus')` |
| `shows other nav items regardless of feature flags` — (no Security assertion) | no change | — |

---

#### Commit 2 — Hecate Navigation Restructure (10 files)

**3.4** `frontend/src/locales/en/translation.json`

In `"navigation"` block, after `"hecate": "Hecate"`:
```json
"tunnels": "Tunnels",
"providers": "Providers",
"agent": "Agent",
```

**Extend the existing `hecate` namespace** in `translation.json` (it already exists at line ~1557 with `hecate.page.*`, `hecate.form.*`, `hecate.wizard.*` sub-keys). Add alongside those existing keys:
```json
"tunnels": { "title": "Tunnels", "description": "Manage tunnel connections" },
"providers": { "title": "Providers", "description": "Configure provider credentials" },
"agent": { "title": "Agent", "description": "Manage Orthrus agents" }
```
Do NOT create a new top-level `hecate` key — it already exists. Merge into the existing object.

**3.5** `frontend/src/App.tsx`

Add imports after existing lazy imports:
```tsx
const HecateTunnels   = lazy(() => import('./pages/HecateTunnels'))
const HecateAgent     = lazy(() => import('./pages/HecateAgent'))
const HecateProviders = lazy(() => import('./pages/HecateProviders'))
```

Remove route:
```tsx
<Route path="remote-servers" element={<RemoteServers />} />
```

Add routes block (after `/domains` route, before `/certificates`):
```tsx
{/* Hecate Routes */}
<Route path="hecate">
  <Route index element={<Navigate to="/hecate/tunnels" replace />} />
  <Route path="tunnels"        element={<HecateTunnels />} />
  <Route path="remote-servers" element={<RemoteServers />} />
  <Route path="providers"      element={<HecateProviders />} />
  <Route path="agent"          element={<HecateAgent />} />
</Route>

{/* Legacy redirect for old Remote Servers bookmarks */}
<Route path="remote-servers" element={<Navigate to="/hecate/remote-servers" replace />} />
```

**3.6** `frontend/src/components/Layout.tsx`

Remove (flat top-level link):
```js
{ name: t('navigation.remoteServers'), path: '/remote-servers', icon: '🖥️' },
```

Remove (flat Hecate link):
```js
{ name: t('navigation.hecate'), path: '/hecate', icon: '🔗' },
```

Add in their place (collapsible Hecate group, insert at same position as removed flat Hecate item):
```js
{
  name: t('navigation.hecate'),
  path: '/hecate',
  icon: '🔗',
  children: [
    { name: t('navigation.remoteServers'), path: '/hecate/remote-servers', icon: '🖥️' },
    { name: t('navigation.tunnels'),       path: '/hecate/tunnels',         icon: '🌐' },
    { name: t('navigation.providers'),     path: '/hecate/providers',       icon: '🔑' },
    { name: t('navigation.agent'),         path: '/hecate/agent',           icon: '🤖' },
  ]
},
```

**3.7** `frontend/src/components/__tests__/Layout.test.tsx`

Update `'renders all navigation items'` test:
- The test currently expects `screen.findByText('Remote Servers')` to be visible without expanding anything.
- After the change, Remote Servers is under the Hecate accordion. Update test:
  ```ts
  // Expand Hecate to see nested items
  await user.click(await screen.findByRole('button', { name: /hecate/i }))
  expect(await screen.findByText('Remote Servers')).toBeInTheDocument()
  expect(await screen.findByText('Tunnels')).toBeInTheDocument()
  expect(await screen.findByText('Providers')).toBeInTheDocument()
  expect(await screen.findByText('Agent')).toBeInTheDocument()
  ```
- Remove the top-level `screen.findByText('Remote Servers')` assertion that appears before the accordion expansion block.

**3.8** `frontend/src/pages/HecateTunnels.tsx` — **New file**

Extract from `Hecate.tsx`:
- All `useState` declarations for tunnel workflow (formOpen, editingTunnel, logsTunnel, deleteTarget, deleteError, isConfirmDeleting, rotateTarget, rotateValue, rotateError, isConfirmRotating)
- `useHecate()` hook invocation
- All tunnel handlers (handleAddTunnel, handleEditTunnel, handleStart, handleStop, handleDeleteConfirm, handleRotateConfirm)
- `columns` definition for the DataTable
- The JSX for the tunnel table section + all 4 dialogs (form, logs, delete confirm, rotate confirm)
- Change `PageShell` title from `t('hecate.title')` to `t('hecate.tunnels.title')`
- Do NOT copy any agent-related imports, state, or JSX

**3.9** `frontend/src/pages/HecateAgent.tsx` — **New file**

Extract from `Hecate.tsx`:
- `useState` declarations for agent workflow (provisionOpen, provisionName, provisionError, isProvisioning, wizardData)
- `useAgentList()`, `useProvisionAgent()`, `useOrthrus()` hook invocations
- `handleProvision` and `handleWizardComplete` handlers (or equivalent)
- The JSX for the agent section: OrthrusAgentManager, provision dialog, OrthrusInstallWizard
- Change `PageShell` title to `t('hecate.agent.title')`

**3.10** `frontend/src/pages/HecateProviders.tsx` — **New file**

New implementation:
```tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PageShell } from '../components/layout/PageShell'
import { HecateTunnelForm } from '../components/hecate/HecateTunnelForm'
import { useHecate } from '../hooks/useHecate'
// Provider icon imports (use inline SVG or emoji placeholders)

const PROVIDERS = [
  { id: 'cloudflare', name: 'Cloudflare', icon: '☁️' },
  { id: 'tailscale',  name: 'Tailscale',  icon: '🔗' },
  { id: 'netbird',    name: 'NetBird',    icon: '🌐' },
  { id: 'zerotier',   name: 'ZeroTier',   icon: '🔒' },
] as const

export default function HecateProviders() {
  const { t } = useTranslation()
  const { tunnels } = useHecate()
  const [formOpen, setFormOpen] = useState(false)
  const [selectedProvider, setSelectedProvider] = useState<string | undefined>()

  return (
    <PageShell title={t('hecate.providers.title')} description={t('hecate.providers.description')}>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {PROVIDERS.map(p => (
          <ProviderCard
            key={p.id}
            provider={p}
            tunnelCount={tunnels.filter(t => t.provider === p.id).length}
            onNewTunnel={() => { setSelectedProvider(p.id); setFormOpen(true) }}
          />
        ))}
      </div>
      <HecateTunnelForm
        open={formOpen}
        initialProvider={selectedProvider}
        onClose={() => { setFormOpen(false); setSelectedProvider(undefined) }}
      />
    </PageShell>
  )
}
```

**Note**: This requires `HecateTunnelForm` to accept an optional `initialProvider` prop. If it does not currently accept this prop, it should be added as an optional `defaultValues` extension in `HecateTunnelForm.tsx`. Document this as a minor interface extension in the implementation notes.

**3.11** `frontend/src/pages/__tests__/HecateTunnels.test.tsx` — **New file**

```typescript
vi.mock('../../hooks/useHecate', ...)
vi.mock('../../components/hecate/HecateTunnelForm', ...)
vi.mock('../../components/hecate/TunnelLogViewer', ...)
vi.mock('../../components/hecate/TunnelStatusBadge', ...)

describe('HecateTunnels', () => {
  it('renders tunnel table with empty state when no tunnels')
  it('renders tunnel rows when tunnels exist')
  it('opens HecateTunnelForm when Add Tunnel clicked')
  it('calls startTunnel when Start button clicked')
  it('calls stopTunnel when Stop button clicked')
  it('shows delete confirm dialog before deleting')
  it('calls deleteTunnel after delete confirmation')
  it('opens TunnelLogViewer when view logs button clicked')
})
```

**3.12** `frontend/src/pages/__tests__/HecateAgent.test.tsx` — **New file**

```typescript
vi.mock('../../hooks/useOrthrus', ...)
vi.mock('../../components/hecate/OrthrusAgentManager', ...)
vi.mock('../../components/hecate/OrthrusInstallWizard', ...)

describe('HecateAgent', () => {
  it('renders OrthrusAgentManager')
  it('renders Provision New Agent button')
  it('opens provision dialog when Provision button clicked')
  it('calls useProvisionAgent.mutateAsync on provision submit')
  it('renders OrthrusInstallWizard after successful provision')
})
```

**3.13** `frontend/src/pages/__tests__/HecateProviders.test.tsx` — **New file**

```typescript
vi.mock('../../hooks/useHecate', ...)
vi.mock('../../components/hecate/HecateTunnelForm', ...)

describe('HecateProviders', () => {
  it('renders 4 provider cards: Cloudflare, Tailscale, NetBird, ZeroTier')
  it('displays correct tunnel count per provider')
  it('opens HecateTunnelForm with correct initialProvider when New Tunnel clicked')
  it('shows 0 count when no tunnels exist for a provider')
})
```

---

### Phase 4: Integration and Testing

#### Task 4.1 — Unit Test Validation

```bash
cd /projects/Charon
npx vitest run frontend/src/components/__tests__/Layout.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateTunnels.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateAgent.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateProviders.test.tsx
```

Existing tests that must still pass without modification:
- `frontend/src/components/hecate/__tests__/TunnelStatusBadge.test.tsx`
- `frontend/src/components/hecate/__tests__/HecateTunnelForm.test.tsx`
- `frontend/src/components/hecate/__tests__/CloudflareTunnelWizard.test.tsx`
- `frontend/src/components/hecate/__tests__/TunnelLogViewer.test.tsx`
- `frontend/src/hooks/__tests__/useHecate.test.tsx`

#### Task 4.2 — E2E Validation

```bash
# Rebuild required (routing changes in Commit 2)
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Cerberus tests
npx playwright test tests/core/cerberus-navigation.spec.ts --project=firefox

# Hecate tests
npx playwright test tests/core/hecate-navigation.spec.ts --project=firefox
npx playwright test tests/core/navigation.spec.ts --project=firefox
npx playwright test tests/hecate-tunnel-manager.spec.ts --project=firefox
```

#### Task 4.3 — Route Smoke Tests (Manual)

| URL | Expected Behavior |
|---|---|
| `GET /hecate` | Redirects to `/hecate/tunnels`, renders HecateTunnels page |
| `GET /hecate/tunnels` | Renders HecateTunnels page |
| `GET /hecate/remote-servers` | Renders RemoteServers page |
| `GET /hecate/providers` | Renders HecateProviders page |
| `GET /hecate/agent` | Renders HecateAgent page |
| `GET /remote-servers` | Redirects to `/hecate/remote-servers` |
| `GET /security` | Still renders Security dashboard (no regression) |

---

### Phase 5: Documentation

No documentation changes required. This is a pure navigation restructure with no new user-facing features or API surface changes.

---

## 5. Acceptance Criteria

### Commit 1: Cerberus Rebrand DoD

- [ ] Sidebar shows "Cerberus" where "Security" previously appeared
- [ ] All existing security sub-pages remain accessible under `/security/*` — no path changes
- [ ] `feature.cerberus.enabled = false` still hides the section
- [ ] `feature.cerberus.enabled = true` still shows the section with all 7 children
- [ ] All Layout.test.tsx tests pass (0 failures)
- [ ] `tests/core/cerberus-navigation.spec.ts` passes
- [ ] No regression in `tests/core/navigation.spec.ts`

### Commit 2: Hecate Navigation Restructure DoD

- [ ] "Remote Servers" is NOT visible as a top-level sidebar item
- [ ] "Hecate" renders as collapsible group with exactly 4 children
- [ ] All 4 `/hecate/*` sub-routes render correct pages without errors
- [ ] `/hecate` redirects to `/hecate/tunnels`
- [ ] `/remote-servers` redirects to `/hecate/remote-servers`
- [ ] Remote server CRUD still works at `/hecate/remote-servers` (create/edit/delete)
- [ ] Tunnel management works at `/hecate/tunnels`
- [ ] Provider cards render at `/hecate/providers` with correct tunnel counts
- [ ] Agent management works at `/hecate/agent`
- [ ] All 3 new page unit tests pass
- [ ] All existing Hecate component tests still pass
- [ ] `tests/core/hecate-navigation.spec.ts` passes
- [ ] `tests/hecate-tunnel-manager.spec.ts` passes

---

## 6. Exact Test Failures — Cerberus Rebrand (Commit 1 Pre-fix State)

Tests in `frontend/src/components/__tests__/Layout.test.tsx` that fail after Layout.tsx is changed but before the test file is updated:

| Test Name | Exact Failure | Fix |
|---|---|---|
| `renders all navigation items` | `screen.findByRole('button', { name: /security/i })` → no element found | Change to `/cerberus/i` |
| `displays Security nav item when Cerberus is enabled` | `screen.getByText('Security')` → element not found | Change to `'Cerberus'` |
| `hides Security nav item when Cerberus is disabled` | `screen.queryByText('Security')` → now correctly returns null, but assertion `not.toBeInTheDocument()` passes for wrong reasons | Change to `queryByText('Cerberus')` |
| `shows Security and Uptime when both features are enabled` | `screen.getByText('Security')` → element not found | Change to `'Cerberus'` |
| `hides both Security and Uptime when both features are disabled` | `screen.queryByText('Security')` → passes spuriously | Change to `queryByText('Cerberus')` |
| `defaults to showing Security and Uptime when feature flags are loading` | `screen.getByText('Security')` → element not found | Change to `'Cerberus'` |
| `shows other nav items regardless of feature flags` | No failure (doesn't assert Security/Cerberus text) | No change needed |

**Total**: 6 tests break, 1 test continues passing (no update needed).

---

## 7. Commit Slicing Strategy

### Decision: 2 commits, 1 PR

**Rationale**: Both commits touch `Layout.tsx` and `translation.json`. Splitting allows reviewers to approve the trivial Cerberus label rename (Commit 1 — 3 files, <15 line changes) independently from the larger Hecate restructure (Commit 2 — 10 files, 3 new pages). Each commit is independently rollback-safe.

---

### Commit 1: Cerberus Rebrand

```
feat(frontend/nav): rename Security sidebar section to Cerberus

Aligns sidebar label with the feature flag name (feature.cerberus.enabled).
Route paths, sub-pages, and internal Security page titles are unchanged.
Updates Layout.tsx, translation.json, and Layout unit tests.
```

**Scope**: `feat(frontend/nav)`
**Files**: 3

| File | Change Summary |
|---|---|
| `frontend/src/locales/en/translation.json` | Add `"cerberus": "Cerberus"` to navigation block |
| `frontend/src/components/Layout.tsx` | `navigation.security` → `navigation.cerberus` (2 locations) |
| `frontend/src/components/__tests__/Layout.test.tsx` | Update 6 test assertions: Security → Cerberus |

**Dependencies**: None. Applies cleanly to `feature/hecate`.

**Validation gates** (must pass before tagging commit):
```bash
npx vitest run frontend/src/components/__tests__/Layout.test.tsx  # all pass
# No E2E rebuild needed — label-only change
npx playwright test tests/core/navigation.spec.ts --project=firefox
```

**Rollback**: `git revert <sha>` is safe — label-only change, no data or route impact.

---

### Commit 2: Hecate Navigation Restructure

```
feat(frontend/nav): split Hecate into collapsible section with 4 sub-pages

Converts flat Hecate sidebar link into collapsible group with:
- Remote Servers (/hecate/remote-servers) — reuses existing RemoteServers component
- Tunnels (/hecate/tunnels) — extracted from Hecate.tsx
- Providers (/hecate/providers) — new provider overview page
- Agent (/hecate/agent) — extracted from Hecate.tsx

Moves /remote-servers under Hecate with legacy redirect at old path.
Adds App.tsx routes, updates Layout.tsx nav array, adds 3 new page files.
```

**Scope**: `feat(frontend/nav)`
**Files**: 10

| File | Change Summary |
|---|---|
| `frontend/src/locales/en/translation.json` | Add `tunnels`, `providers`, `agent` nav keys; add `hecate.*` page title namespace |
| `frontend/src/App.tsx` | Add 3 lazy imports; add Hecate route block; legacy redirect for `/remote-servers` |
| `frontend/src/components/Layout.tsx` | Remove 2 flat items; add Hecate collapsible group with 4 children |
| `frontend/src/components/__tests__/Layout.test.tsx` | Update `renders all navigation items` for Remote Servers under Hecate accordion |
| `frontend/src/pages/HecateTunnels.tsx` | **New** — tunnel management extracted from Hecate.tsx |
| `frontend/src/pages/HecateAgent.tsx` | **New** — agent management extracted from Hecate.tsx |
| `frontend/src/pages/HecateProviders.tsx` | **New** — provider overview page |
| `frontend/src/pages/__tests__/HecateTunnels.test.tsx` | **New** — unit tests for HecateTunnels |
| `frontend/src/pages/__tests__/HecateAgent.test.tsx` | **New** — unit tests for HecateAgent |
| `frontend/src/pages/__tests__/HecateProviders.test.tsx` | **New** — unit tests for HecateProviders |

**Dependencies**: Commit 1 must be applied first (translation.json must already have `cerberus`).

**Validation gates** (must pass before tagging commit):
```bash
# Unit tests
npx vitest run frontend/src/components/__tests__/Layout.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateTunnels.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateAgent.test.tsx
npx vitest run frontend/src/pages/__tests__/HecateProviders.test.tsx
# Existing Hecate component tests (must still pass — no changes to them)
npx vitest run frontend/src/components/hecate/__tests__/

# E2E (rebuild required for routing changes)
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
npx playwright test tests/core/navigation.spec.ts tests/core/hecate-navigation.spec.ts tests/hecate-tunnel-manager.spec.ts --project=firefox
```

**Rollback**: Reverting this commit removes Hecate sub-navigation. The legacy redirect at `/remote-servers` disappears too, but `Hecate.tsx` is still present and untouched, so no runtime crashes occur. Note: after rollback, the flat `/hecate` link from Commit 0 (original feature/hecate state) would need to be manually restored.

**Contingency — `HecateTunnelForm` initialProvider prop**: If `HecateTunnelForm.tsx` does not accept an `initialProvider` prop, add it as part of this commit. The change is: add `initialProvider?: string` to the component's props interface, and use it to set the provider `defaultValue` in the form's `useForm` call. This is a non-breaking addition.

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `navigation.security` filter check becomes stale after Commit 1 | High (guaranteed failure) | Cerberus section hidden for all users | Covered by Layout.test.tsx — test will fail immediately if filter not updated |
| Legacy redirect `/remote-servers` missing | Low | Bookmarks/E2E links break | Explicit E2E test for this redirect in `hecate-navigation.spec.ts` |
| `Hecate.tsx` still imported somewhere after extraction | Low | Build error | Run `grep -rn "from.*pages/Hecate['\"]" frontend/src/` before commit |
| `HecateTunnelForm` missing `initialProvider` prop | Medium | HecateProviders can't pre-select provider | Contingency plan documented in Commit 2 section |
| Collapsed sidebar (icon-only) mode for new Hecate group | Low | Icon links to `/hecate` which redirects to `/hecate/tunnels` — works correctly | No action needed; consistent with existing DNS/Security collapsed behavior |
| `hecate-tunnel-manager.spec.ts` hardcodes `/remote-servers` | Medium | E2E failure | Update spec in Phase 1 Task 1.4 before writing other code |
| Translation key conflict: `"hecate"` used both in `navigation` block and as top-level namespace | Medium | JSON parse ambiguity | The navigation key is `navigation.hecate`; the new top-level is `hecate.tunnels.*` — no conflict; different paths in the JSON tree |
