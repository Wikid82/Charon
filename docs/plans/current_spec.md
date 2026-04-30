# Hecate Provider Integration in Remote Server Setup

**Branch**: `feature/hecate`
**Date**: 2026-04-30
**Status**: Ready for implementation

---

## 1. Summary

This spec covers two tightly related deliverables that together form the complete Hecate frontend experience:

1. **Hecate management page** (`/hecate`) — a full CRUD interface for creating and managing Hecate tunnel providers (Cloudflare, Tailscale, NetBird, ZeroTier) and Orthrus agents. This page must exist before users can select a provider in Remote Server setup.

2. **Remote Server form restructure** — the "Connection Type" flat dropdown is replaced with a two-tier UI: a radio group selecting **Agent** or **Direct** (Tier 1), then a provider/device picker (Tier 2/3) shown only when Agent is selected.

No existing pages or features are removed. The changes are purely additive and restructuring.

### What changes and why

| Area | Before | After | Reason |
|---|---|---|---|
| Remote Server form | Flat 3-option select: Direct / Orthrus Agent / Cloudflare Tunnel | Radio Agent/Direct + provider dropdown + device picker | Supports 5 providers; flat select does not scale |
| `ConnectionTypeSelector` | Single `<NativeSelect>` with 3 hard-coded options | Two-tier compound component (radio + dynamic dropdown) | New interface required |
| Hecate page | Does not exist | New `/hecate` route with tunnel CRUD + Orthrus section | Users need a place to configure providers before selecting them |
| Backend model | `connection_type`: direct/orthrus/cloudflare only; no `hecate_tunnel_uuid` field | 3 new connection types; new nullable `hecate_tunnel_uuid` column | Tailscale/NetBird/ZeroTier connections must reference their tunnel config |
| Navigation | No Hecate entry | "Hecate" nav item after Remote Servers | Users must be able to navigate to the management page |

---

## 2. Data Model Assessment

### Current state

`backend/internal/models/remote_server.go`:

```go
type ConnectionType string

const (
    ConnectionTypeDirect     ConnectionType = "direct"
    ConnectionTypeOrthrus    ConnectionType = "orthrus"
    ConnectionTypeCloudflare ConnectionType = "cloudflare"
)

type RemoteServer struct {
    ...
    ConnectionType   ConnectionType `json:"connection_type" gorm:"default:\'direct\';index"`
    OrthrusAgentUUID *string        `json:"orthrus_agent_uuid,omitempty" gorm:"index"`
    // NO hecate_tunnel_uuid field
}
```

`frontend/src/api/remoteServers.ts`:

```typescript
export interface RemoteServer {
  ...
  connection_type?: 'direct' | 'orthrus' | 'cloudflare';
  orthrus_agent_uuid?: string;
  // NO hecate_tunnel_uuid field
}
```

### Required backend changes

> **Note**: Two backend changes are required to support Tailscale, NetBird, and ZeroTier as distinct connection types.

**Why the changes are needed**:
1. There are no `ConnectionType` enum values for `tailscale`, `netbird`, or `zerotier`. Without them, the form cannot save which type of connection is configured, and the edit form cannot pre-populate the correct picker.
2. There is no column to store which `TunnelConfig` UUID a Tailscale/NetBird/ZeroTier Remote Server is using. Without it, connection status cannot be shown in the server list and the provider cannot be re-selected when editing.

Both changes are backwards-compatible. GORM `AutoMigrate` will add the new nullable column without data loss. No manual migration script is required.

**Change 1 — `backend/internal/models/remote_server.go`**:

```go
type ConnectionType string

const (
    ConnectionTypeDirect     ConnectionType = "direct"
    ConnectionTypeOrthrus    ConnectionType = "orthrus"
    ConnectionTypeCloudflare ConnectionType = "cloudflare"
    ConnectionTypeTailscale  ConnectionType = "tailscale"  // NEW
    ConnectionTypeNetBird    ConnectionType = "netbird"    // NEW
    ConnectionTypeZeroTier   ConnectionType = "zerotier"   // NEW
)

type RemoteServer struct {
    ...
    ConnectionType   ConnectionType `json:"connection_type" gorm:"default:\'direct\';index"`
    OrthrusAgentUUID *string        `json:"orthrus_agent_uuid,omitempty" gorm:"index"`
    HecateTunnelUUID *string        `json:"hecate_tunnel_uuid,omitempty" gorm:"index"` // NEW
}
```

**Change 2 — `frontend/src/api/remoteServers.ts`**:

```typescript
export interface RemoteServer {
  ...
  connection_type?: 'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier';
  orthrus_agent_uuid?: string;
  hecate_tunnel_uuid?: string; // NEW
}
```

### Semantic mapping

| Tier 2 provider selected | `connection_type` stored | Reference field used |
|---|---|---|
| Orthrus agent | `orthrus` | `orthrus_agent_uuid` |
| Cloudflare tunnel | `cloudflare` | `hecate_tunnel_uuid` |
| Tailscale tunnel | `tailscale` | `hecate_tunnel_uuid` + `host` (device IP) |
| NetBird tunnel | `netbird` | `hecate_tunnel_uuid` + `host` (peer IP) |
| ZeroTier tunnel | `zerotier` | `hecate_tunnel_uuid` + `host` (member IP) |
| (Direct) | `direct` | `host` + `port` |

For Tailscale/NetBird/ZeroTier: after picking a device/peer/member, `host` is auto-filled with the VPN IP and Caddy routes to it directly. The `hecate_tunnel_uuid` is stored so the form can re-open the correct picker on edit, and the server list can show the tunnel live status.

For Cloudflare: `host` is left blank. The cloudflared daemon on the remote server handles routing. `hecate_tunnel_uuid` identifies which `TunnelConfig` holds the credentials.

---

## 3. Component Changes

### 3.1 `frontend/src/components/hecate/ConnectionTypeSelector.tsx`

**Current interface** (to be replaced):

```typescript
export type ConnectionType = 'direct' | 'orthrus' | 'cloudflare';

interface ConnectionTypeSelectorProps {
  value: ConnectionType;
  onChange: (value: ConnectionType) => void;
  id?: string;
  disabled?: boolean;
}
```

The component renders a single `<NativeSelect>` with 3 hard-coded `<option>` elements.

**New interface**:

```typescript
export type ConnectionMode = 'direct' | 'agent';

export type ConnectionType =
  | 'direct'
  | 'orthrus'
  | 'cloudflare'
  | 'tailscale'
  | 'netbird'
  | 'zerotier';

export type HecateProvider = Exclude<ConnectionType, 'direct'>;

export interface ConnectionTypeSelectorProps {
  mode: ConnectionMode;
  onModeChange: (mode: ConnectionMode) => void;
  selectedTunnelUUID: string | null;
  selectedAgentUUID: string | null;
  onTunnelSelect: (tunnelUUID: string, provider: HecateProvider) => void;
  onAgentSelect: (agentUUID: string) => void;
  disabled?: boolean;
}
```

**What the component renders**:

- **Tier 1**: A `<fieldset>` with `<legend>` containing two `<input type="radio">` elements for Direct and Agent.
- **Tier 2** (only when `mode === 'agent'`): A labeled `<NativeSelect>` or `<select>` populated by calling `useHecate()` and `useAgentList()` internally. Options are grouped using `<optgroup>` by provider. Orthrus agents use value prefix `orthrus:${agent.uuid}` to distinguish from tunnel UUIDs.

**Option groups** (only rendered when the group has at least one option):

```
<optgroup label="Cloudflare">  — TunnelConfig entries with provider=cloudflare
<optgroup label="Tailscale">   — TunnelConfig entries with provider=tailscale
<optgroup label="NetBird">     — TunnelConfig entries with provider=netbird
<optgroup label="ZeroTier">    — TunnelConfig entries with provider=zerotier
<optgroup label="Orthrus">     — OrthrusAgent entries
```

**Empty state**: When `tunnels.length === 0` and `agents.length === 0`, show inline note:
"No providers configured. [Go to Hecate to add one.](/hecate)" — rendered below the select, with `aria-live="polite"`.

**Option value decoding** in `onChange`:

```typescript
const handleProviderChange = (value: string) => {
  if (value.startsWith('orthrus:')) {
    onAgentSelect(value.replace('orthrus:', ''))
  } else {
    const tunnel = tunnels.find(t => t.uuid === value)
    if (tunnel) onTunnelSelect(value, tunnel.provider as HecateProvider)
  }
}
```

---

### 3.2 `frontend/src/components/RemoteServerForm.tsx`

See Section 5 for the complete restructure. Summary:

- Extend `formData` to include `connection_mode`, `hecate_tunnel_uuid`, `selected_device_name`, `selected_device_address`
- Replace old `ConnectionTypeSelector` call with new compound component
- Add Tier 3 device pickers (`TailscaleDevicePicker`, `NetBirdPeerPicker`, `ZeroTierMemberPicker`) rendered conditionally
- Remove `CloudflareTunnelWizard` from form (Cloudflare tunnels are created on the Hecate page)
- Remove inline agent provisioning ("Provision New Agent", "Manage Agents" buttons) — move to Hecate page
- "Test Connection" button: only enabled when `connection_type === 'direct'` and host is non-empty

---

### 3.3 `frontend/src/pages/RemoteServers.tsx`

**Changes** (minimal):

1. **Connection type column**: handle all 6 values including `tailscale`, `netbird`, `zerotier`.
2. **TunnelStatusBadge lookup**: change `getStatus(server.uuid)` to `getStatus(server.hecate_tunnel_uuid ?? server.uuid)` so the badge shows the correct tunnel state for Hecate-backed connections.

---

### 3.4 `frontend/src/App.tsx`

Add lazy import (in alphabetical order with other pages):

```typescript
const Hecate = lazy(() => import('./pages/Hecate'))
```

Add route inside the authenticated/layout group, after `remote-servers`:

```typescript
<Route path="hecate" element={<Hecate />} />
```

---

### 3.5 `frontend/src/components/Layout.tsx`

Add nav item immediately after the `remote-servers` entry:

```typescript
{ name: t('navigation.hecate'), path: '/hecate', icon: '🔗' },
```

No feature flag is needed. The page handles the empty state gracefully.

---

### 3.6 `frontend/src/locales/en/translation.json`

**Under `navigation`** — add:

```json
"hecate": "Hecate"
```

**Under `hecate`** — add two new sub-objects `page` and `form.mode`:

```json
"page": {
  "title": "Hecate",
  "description": "Manage network tunnel providers for Agent-mode connections",
  "addProvider": "Add Provider",
  "editProvider": "Edit Provider",
  "deleteProvider": "Delete Provider",
  "start": "Start",
  "stop": "Stop",
  "rotateCredentials": "Rotate Credentials",
  "viewLogs": "View Logs",
  "provisionAgent": "Provision New Agent",
  "confirmDeleteTitle": "Delete Provider",
  "confirmDeleteDescription": "Are you sure you want to delete \"{{name}}\"? Remote Servers using this provider will lose their connection.",
  "confirmDeleteButton": "Delete",
  "rotateTitle": "Rotate Credentials",
  "rotateDescription": "Enter new credentials for {{name}}. The old credentials will be replaced immediately.",
  "rotateButton": "Rotate",
  "orthrusSection": "Orthrus Agents",
  "tunnelSection": "Tunnel Providers",
  "columns": {
    "name": "Name",
    "provider": "Provider",
    "status": "Status",
    "active": "Active",
    "created": "Created",
    "actions": "Actions"
  },
  "emptyState": {
    "title": "No providers configured",
    "description": "Add a Cloudflare, Tailscale, NetBird, or ZeroTier provider to enable Agent-mode connections for Remote Servers."
  },
  "credentials": {
    "editHint": "Leave any field blank to keep the existing credential.",
    "showField": "Show {{field}}",
    "hideField": "Hide {{field}}",
    "cloudflare": {
      "apiToken": "API Token",
      "apiTokenHelp": "Create at dash.cloudflare.com → My Profile → API Tokens",
      "accountId": "Account ID",
      "accountIdHelp": "Found in the sidebar of your Cloudflare dashboard",
      "tunnelToken": "Tunnel Token",
      "tunnelTokenHelp": "Zero Trust → Networks → Tunnels → your tunnel → Configure → Token"
    },
    "tailscale": {
      "apiKey": "API Key",
      "apiKeyHelp": "tailscale.com/admin/settings/keys → Generate access token",
      "tailnet": "Tailnet",
      "tailnetHelp": "Your organization name or email domain (e.g., example.com)"
    },
    "netbird": {
      "accessToken": "Access Token",
      "accessTokenHelp": "NetBird Management Console → Settings → Personal Access Tokens",
      "managementUrl": "Management URL (optional)",
      "managementUrlHelp": "Default: app.netbird.io — override for self-hosted NetBird"
    },
    "zerotier": {
      "apiToken": "API Token",
      "apiTokenHelp": "my.zerotier.com/account → New Token",
      "controllerUrl": "Controller URL (optional)",
      "controllerUrlHelp": "Default: api.zerotier.com — override for self-hosted ZeroTier"
    }
  }
},
"form": {
  "mode": {
    "label": "Connection mode",
    "direct": "Direct",
    "agent": "Agent",
    "directDescription": "Connect via IP or hostname",
    "agentDescription": "Route traffic through a Hecate network provider",
    "provider": "Provider",
    "selectProvider": "Select a provider...",
    "noProviders": "No providers configured.",
    "goToHecate": "Go to Hecate to add one.",
    "selectedDevice": "Routing via {{name}} ({{address}})",
    "changeDevice": "Change",
    "selectDevice": "Select device",
    "selectPeer": "Select peer",
    "selectMember": "Select member"
  }
}
```

> The existing `hecate.form.selectAgent`, `hecate.form.provisionAgent`, and related keys remain in the file for now (they are referenced by code that is being removed, so they become dead keys). Remove them in a separate cleanup commit after this PR lands.

---

## 4. New Components

### 4.1 `frontend/src/pages/Hecate.tsx` (NEW)

**Type**: Page component (no props)
**Route**: `/hecate`

**Purpose**: Standalone management page for all Hecate tunnel providers. Users configure at least one provider here before using Agent mode in Remote Server setup.

**Hooks**: `useHecate()`, `useAgentList()`, `useProvisionAgent()`

**Local state**:

```typescript
const [showForm, setShowForm] = useState(false)
const [editingTunnel, setEditingTunnel] = useState<TunnelConfig | null>(null)
const [deleteConfirm, setDeleteConfirm] = useState<TunnelConfig | null>(null)
const [isDeleting, setIsDeleting] = useState(false)
const [logsOpen, setLogsOpen] = useState(false)
const [logsTarget, setLogsTarget] = useState<TunnelConfig | null>(null)
const [rotateOpen, setRotateOpen] = useState(false)
const [rotateTarget, setRotateTarget] = useState<TunnelConfig | null>(null)
const [provisionWizardOpen, setProvisionWizardOpen] = useState(false)
```

**Page structure outline**:

```
<PageShell title="Hecate" description="..." action={<Button>Add Provider</Button>}>

  <section aria-label="Tunnel Providers">
    <h2>Tunnel Providers</h2>
    {loading ? <SkeletonTable /> : tunnels.length === 0 ? <EmptyState /> : <DataTable />}
  </section>

  <section aria-label="Orthrus Agents">
    <h2>Orthrus Agents</h2>
    <OrthrusAgentManager agents={agents} />
    <Button>Provision New Agent</Button>
  </section>

  <!-- Modals -->
  <HecateTunnelForm open={showForm} tunnel={editingTunnel ?? undefined} onClose={...} />
  <TunnelLogViewer open={logsOpen} tunnelUUID={logsTarget?.uuid} onClose={...} />
  <Dialog open={deleteConfirm !== null}> ... confirm dialog ... </Dialog>
  <OrthrusInstallWizard open={provisionWizardOpen} ... />

</PageShell>
```

**DataTable columns** for tunnel providers:

| Key | Header | Cell |
|---|---|---|
| `name` | Name | `<span className="font-medium">{tunnel.name}</span>` |
| `provider` | Provider | `<Badge variant="outline">{tunnel.provider}</Badge>` |
| `status` | Status | `<TunnelStatusBadge state={getStatus(tunnel.uuid)?.state ?? 'stopped'} />` |
| `is_active` | Active | active/inactive badge |
| `created_at` | Created | formatted date |
| `actions` | Actions | Start/Stop, Logs, Edit, Rotate Credentials, Delete |

**Action buttons per row**:
- **Start**: `startTunnel(uuid)` — shown when `state !== 'connected'`
- **Stop**: `stopTunnel(uuid)` — shown when `state === 'connected'`
- **View Logs**: opens `<TunnelLogViewer>`
- **Edit**: opens `<HecateTunnelForm>` with tunnel
- **Rotate Credentials**: opens rotate dialog
- **Delete**: opens confirm dialog → `deleteTunnel(uuid)`

---

### 4.2 `frontend/src/components/hecate/HecateTunnelForm.tsx` (NEW)

**Purpose**: Modal dialog for creating or editing a `TunnelConfig`. Renders provider-aware credential fields.

**Props**:

```typescript
interface HecateTunnelFormProps {
  tunnel?: TunnelConfig;  // undefined = create mode
  open: boolean;
  onClose: () => void;
}
```

**Local state**: `name`, `provider`, `isActive`, `creds` (per-provider credential fields), `showSecrets` (map), `loading`, `error`, `fieldErrors`

**Credential fields per provider**:

| Provider | Required fields | Optional fields |
|---|---|---|
| `cloudflare` | `api_token`, `account_id`, `tunnel_token` | — |
| `tailscale` | `api_key`, `tailnet` | — |
| `netbird` | `access_token` | `management_url` |
| `zerotier` | `api_token` | `controller_url` |

All token fields: `type="password"` with show/hide toggle button (`Eye`/`EyeOff` icon). Toggle `aria-label` uses `t('hecate.page.credentials.showField', { field: '...' })`.

**Edit mode**: Provider `<select>` is disabled (cannot switch provider on edit — that would require new credentials and is destructive). All credential fields are empty; placeholder: `"Leave blank to keep existing credential"`. On submit, credentials are only included in the payload if at least one field is non-empty.

**Submit logic**:
- Create: build `credentials` JSON from active provider fields, call `createTunnel({ name, provider, credentials, is_active })`
- Edit: conditionally include `credentials` only when non-empty, call `updateTunnel({ uuid, req: { name, provider, credentials?, is_active } })`

**`buildCredentialsJSON(provider, creds)`** — internal helper mapping flat `creds` state to the JSON string expected by the backend.

---

### 4.3 `frontend/src/components/hecate/NetBirdPeerPicker.tsx` (NEW)

Mirrors `TailscaleDevicePicker` structure exactly.

**Props**:

```typescript
interface NetBirdPeerPickerProps {
  open: boolean;
  onClose: () => void;
  onSelect: (peer: NetBirdPeer) => void;
  selectedId?: string;
}
```

**Data**: Calls `listNetBirdPeers()` from `api/hecate.ts` via `useQuery`. Uses global NetBird credentials configured in the Hecate service (no per-call tunnel UUID required).

**List item**: name, IP, OS, online/offline badge. `role="option"`, `aria-selected={peer.id === selectedId}`.

**Empty state**: "No NetBird peers found. Ensure your access token is configured."

---

### 4.4 `frontend/src/components/hecate/ZeroTierMemberPicker.tsx` (NEW)

Two-step dialog: network → member.

**Props**:

```typescript
interface ZeroTierMemberPickerProps {
  open: boolean;
  onClose: () => void;
  onSelect: (member: ZeroTierMember, networkId: string) => void;
  selectedNodeId?: string;
}
```

**Internal state**: `step: 'network' | 'member'`, `selectedNetwork: ZeroTierNetwork | null`

**Step 1 — Network picker**: Query `listZeroTierNetworks()`. Renders list of networks (name, ID, member count). Clicking a network advances to step 2.

**Step 2 — Member picker**: Query `listZeroTierMembers(selectedNetwork.id)` (enabled only when `step === 'member'`). Back button returns to step 1. Clicking a member calls `onSelect(member, selectedNetwork.id)` and `onClose()`.

**Dialog title**: "Select ZeroTier Network" (step 1) / "Select ZeroTier Member — {network.name}" (step 2).

---

## 5. RemoteServerForm Restructure

### Current UI flow

```
[Name]
[Provider]
[ConnectionTypeSelector] — flat select: Direct / Orthrus Agent / Cloudflare Tunnel

→ if 'orthrus': agent dropdown + Provision button + Manage Agents + OrthrusAgentManager
→ if 'cloudflare': CloudflareTunnelWizard (inline)
→ if 'direct': [Host] [Port] [Username]

[Enabled]
[Test Connection | Cancel | Save]
```

### New UI flow

```
[Name]
[Provider]

┌─ Connection mode (fieldset) ──────────────────────────────────┐
│ ○ Direct   Connect via IP or hostname                         │
│ ● Agent    Route traffic through a Hecate network provider    │
└───────────────────────────────────────────────────────────────┘

  (if Agent)
  [Provider select — optgroups: Cloudflare / Tailscale / NetBird / ZeroTier / Orthrus]
  [if no providers: "No providers. Go to Hecate →"]

  (if Tailscale selected)    [Select Tailscale Device] → TailscaleDevicePicker
                             OR "Routing via server.local (100.64.1.2) [Change]"
  (if NetBird selected)      [Select NetBird Peer] → NetBirdPeerPicker
  (if ZeroTier selected)     [Select ZeroTier Member] → ZeroTierMemberPicker
  (if Cloudflare selected)   ℹ️ "Traffic routed through Cloudflare Tunnel: {name}"
  (if Orthrus selected)      Agent: {name} [status badge]
                             "Manage agents →" link to /hecate

  (if Direct)
  [Host]
  [Port] [Username]

[Enabled]
[Test Connection (Direct + host filled only) | Cancel | Save]
```

### Updated `formData` shape

```typescript
interface FormData {
  name: string;
  provider: string;
  host: string;
  port: number;
  username: string;
  enabled: boolean;
  connection_mode: 'direct' | 'agent';    // UI state only; not submitted
  connection_type: ConnectionType;         // what is stored in DB
  orthrus_agent_uuid: string;
  hecate_tunnel_uuid: string;
  selected_device_name: string;           // display only
  selected_device_address: string;        // display only
}
```

**Initialisation helper**:

```typescript
function deriveConnectionMode(ct?: string): 'direct' | 'agent' {
  return ct && ct !== 'direct' ? 'agent' : 'direct'
}
```

### Submit payload construction

```typescript
const payload: Partial<RemoteServer> = {
  name: formData.name,
  provider: formData.provider,
  enabled: formData.enabled,
  connection_type: formData.connection_type,
}

if (formData.connection_type === 'direct') {
  payload.host = formData.host
  payload.port = formData.port
  payload.username = formData.username
} else if (formData.connection_type === 'orthrus') {
  payload.orthrus_agent_uuid = formData.orthrus_agent_uuid
} else {
  // cloudflare, tailscale, netbird, zerotier
  payload.hecate_tunnel_uuid = formData.hecate_tunnel_uuid
  if (formData.host) {
    payload.host = formData.host
    payload.port = formData.port
  }
}
```

### Validation on submit

- Agent mode: `connection_type` must not be `'direct'`; provider dropdown must have a selection
- Tailscale/NetBird/ZeroTier: `host` must be non-empty (device must be picked)
- Orthrus: `orthrus_agent_uuid` must be non-empty
- Direct: `host` must be non-empty

### Inline Orthrus section (simplified)

Replace the current inline agent provisioning block with:

```tsx
{formData.connection_type === 'orthrus' && (
  <div>
    <label htmlFor="orthrus-agent-select">{t('hecate.form.selectAgent')}</label>
    <select id="orthrus-agent-select"
      value={formData.orthrus_agent_uuid}
      onChange={e => setFormData({ ...formData, orthrus_agent_uuid: e.target.value })}>
      <option value="">{t('hecate.form.selectAgent')}</option>
      {agents.map(a => <option key={a.uuid} value={a.uuid}>{a.name}</option>)}
    </select>
    <p className="text-sm text-content-muted">
      Need a new agent?{' '}
      <Link to="/hecate">Go to the Hecate page</Link>
    </p>
  </div>
)}
```

### Picker dialog state

```typescript
const [tailscalePickerOpen, setTailscalePickerOpen] = useState(false)
const [netbirdPickerOpen, setNetbirdPickerOpen] = useState(false)
const [zerotierPickerOpen, setZerotierPickerOpen] = useState(false)
```

Tailscale device data is fetched with:

```typescript
const { data: tailscaleDevices = [] } = useQuery({
  queryKey: ['hecate', 'tailscale', 'devices'],
  queryFn: listTailscaleDevices,
  enabled: formData.connection_type === 'tailscale',
  staleTime: 60_000,
})
```

NetBirdPeerPicker and ZeroTierMemberPicker fetch their own data internally.

### Removed from RemoteServerForm

- `CloudflareTunnelWizard` (moved to Hecate page)
- `OrthrusAgentManager` inline display
- `OrthrusInstallWizard` inline trigger
- "Provision New Agent" button
- "Manage Agents" / "Hide agent manager" button

---

## 6. Accessibility

### Radio group (Tier 1)

Use native `<input type="radio">` elements inside a `<fieldset>` with `<legend>`. This provides correct radiogroup semantics across all assistive technologies without ARIA overrides.

```html
<fieldset>
  <legend class="text-sm font-medium">Connection mode</legend>
  <label>
    <input type="radio" name="connection-mode" value="direct" />
    <span><strong>Direct</strong></span>
    <span class="text-sm text-content-muted">Connect via IP or hostname</span>
  </label>
  <label>
    <input type="radio" name="connection-mode" value="agent" />
    <span><strong>Agent</strong></span>
    <span class="text-sm text-content-muted">Route traffic through a Hecate network provider</span>
  </label>
</fieldset>
```

**Keyboard**: Tab enters the radio group. Arrow keys switch between options. Standard browser behaviour — no custom key handling needed.

### Provider dropdown (Tier 2)

```html
<label for="hecate-provider-select">Provider</label>
<select id="hecate-provider-select"
        aria-required="true"
        aria-describedby="hecate-provider-hint">
  ...
</select>
<p id="hecate-provider-hint" aria-live="polite" class="text-sm text-content-muted">
  {noProviders && "No providers configured. Go to Hecate to add one."}
</p>
```

`aria-live="polite"` announces the "no providers" note when Agent mode is selected.

### Device pickers (Tier 3)

Apply the `TailscaleDevicePicker` pattern: `role="listbox"` on the list container, `role="option"` and `aria-selected` on each item. `NetBirdPeerPicker` and `ZeroTierMemberPicker` must follow this pattern.

### `HecateTunnelForm` dialog

- `<Dialog>` provides `role="dialog"`, `aria-modal="true"`, focus trap, Escape-to-close
- Required fields: `aria-required="true"`; `aria-invalid="true"` when in error state
- Error messages: unique `id="field-name-error"` + `aria-describedby="field-name-error"` on the associated input
- Password toggle: `aria-label` uses `showField`/`hideField` i18n keys; updates dynamically

### Error focus management

On form submit with validation errors, focus the first field with an error:

```typescript
document.getElementById(firstErrorFieldId)?.focus()
```

This matches the pattern used in `ProxyHostForm.tsx`.

### Colour contrast

All new text uses existing design tokens (`text-content-primary`, `text-content-secondary`, `text-content-muted`) which already meet WCAG AA contrast requirements.

---

## 7. Testing

### 7.1 Tests to fully rewrite

**`frontend/src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx`**

The component's prop interface changes completely. Full rewrite:

```
ConnectionTypeSelector — two-tier
  Tier 1 radio group
    renders "Direct" and "Agent" radio buttons
    "Direct" radio is checked when mode="direct"
    "Agent" radio is checked when mode="agent"
    clicking Agent radio calls onModeChange('agent')
    clicking Direct radio calls onModeChange('direct')
    Arrow key navigation between radio options (userEvent)

  Tier 2 provider dropdown
    not rendered when mode="direct"
    rendered when mode="agent"
    renders optgroup for each provider with available tunnels
    renders Orthrus optgroup with agents
    does not render empty optgroups
    selecting a Cloudflare tunnel calls onTunnelSelect(uuid, 'cloudflare')
    selecting a Tailscale tunnel calls onTunnelSelect(uuid, 'tailscale')
    selecting an Orthrus agent calls onAgentSelect(agentUUID)
    shows "No providers" hint when tunnels=[] and agents=[]
    hint contains link with href="/hecate"
    hint not shown when tunnels or agents exist

  Disabled state
    both radios disabled when disabled=true
    provider dropdown disabled when disabled=true

Mock: vi.mock('hooks/useHecate') and vi.mock('hooks/useOrthrus') with canned data
```

### 7.2 New test files

**`frontend/src/pages/__tests__/Hecate.test.tsx`**

```
Hecate page
  Layout
    renders "Hecate" page title
    renders "Add Provider" button
    renders "Orthrus Agents" section heading
    page has exactly one h1 element

  Empty state
    shows EmptyState when tunnels=[]
    EmptyState has "Add Provider" action

  Tunnel table
    renders DataTable when tunnels exist
    shows name, provider badge, status badge per row
    Start button calls startTunnel(uuid) when state !== 'connected'
    Stop button calls stopTunnel(uuid) when state === 'connected'
    Edit button opens HecateTunnelForm with tunnel data
    View Logs button opens TunnelLogViewer
    Delete button opens confirm dialog
    Confirm delete calls deleteTunnel(uuid) and closes dialog
    Cancel delete closes dialog without calling deleteTunnel

  Orthrus section
    renders OrthrusAgentManager
    Provision button triggers provision flow

  Accessibility
    Delete confirm dialog has role="dialog" and aria-modal="true"
```

Mock: `useHecate`, `useAgentList`, `useProvisionAgent`.

**`frontend/src/components/hecate/__tests__/HecateTunnelForm.test.tsx`**

```
HecateTunnelForm
  Create mode
    renders name, provider select, credentials section
    cloudflare: shows api_token, account_id, tunnel_token
    tailscale: shows api_key, tailnet
    netbird: shows access_token; optional management_url
    zerotier: shows api_token; optional controller_url
    changing provider select switches credential fields
    show/hide toggle changes input type password ↔ text
    show/hide toggle aria-label updates correctly
    submitting with empty name shows name error
    submitting with empty required credential shows error
    successful submit calls createTunnel with correct JSON credentials
    Escape key calls onClose

  Edit mode (tunnel prop provided)
    name field pre-filled with tunnel.name
    provider select is disabled
    credential fields are empty (not pre-filled for security)
    fields show "Leave blank to keep existing credential" placeholder
    submitting without filling credentials calls updateTunnel without credentials key
    submitting with a credential field filled includes credentials in payload
    successful update calls onClose
```

**`frontend/src/components/hecate/__tests__/NetBirdPeerPicker.test.tsx`**

```
NetBirdPeerPicker
  renders dialog with correct title
  shows skeleton/loading while fetching
  renders peer list with name, IP, OS, online badge
  online peer shows success badge
  offline peer shows secondary badge
  clicking a peer calls onSelect with peer data and calls onClose
  shows empty state when no peers returned
  selected peer has aria-selected="true"
  not rendered when open=false
```

**`frontend/src/components/hecate/__tests__/ZeroTierMemberPicker.test.tsx`**

```
ZeroTierMemberPicker
  Step 1 — network selection
    renders "Select ZeroTier Network" in dialog title
    lists networks with name, ID, member count
    clicking a network advances to step 2
    shows empty state when no networks found

  Step 2 — member selection
    shows network name in dialog title
    lists members with name, node ID, IP, authorized/online badges
    clicking a member calls onSelect(member, networkId) and onClose
    Back button returns to step 1
    shows empty state when no members in network
```

### 7.3 Tests unchanged (must still pass)

- `CloudflareTunnelWizard.test.tsx` — component interface unchanged
- `TunnelStatusBadge.test.tsx` — no changes
- `TunnelLogViewer.test.tsx` — no changes
- `OrthrusInstallWizard.test.tsx` — no changes

If any `RemoteServerForm` tests exist, update them to reflect the new `formData` shape and removed inline provisioning.

### 7.4 Playwright E2E tests (new)

**`tests/hecate.spec.ts`**:

```
Hecate page — add provider
  Navigate to /hecate
  See "No providers configured" empty state
  Click "Add Provider" → form dialog opens
  Select Tailscale from provider select
  Tailscale credential fields visible
  Fill name, api_key, tailnet → submit
  Dialog closes, table row appears with "tailscale" badge

Hecate page — delete provider
  Click Delete on a row → confirm dialog with warning
  Click "Delete" → row removed

Remote Server form — Agent mode
  Open Remote Server "Add" form
  "Direct" radio selected by default; Host/Port visible
  Select "Agent" radio → Host/Port hidden; Provider dropdown visible
  Select a Tailscale provider → "Select Tailscale Device" button visible
```

---

## 8. Commit Slicing Strategy

**Decision**: Single PR on `feature/hecate` with 3 ordered, independently buildable commits.

**Rationale**:
- Commit 0 (backend model) is a prerequisite for Commit 2 but not Commit 1, isolating backend risk.
- Commit 1 (Hecate page) delivers independent value — users can manage providers before the form restructure ships.
- Commit 2 (form restructure) is reviewable in isolation.
- All three commits compile, build, and pass tests individually.

---

### Commit 0 — Backend model extension + frontend type update

**Scope**: Backend model + frontend API types only
**Files**:
- `backend/internal/models/remote_server.go` — add `HecateTunnelUUID *string` field + 3 new `ConnectionType` constants
- `frontend/src/api/remoteServers.ts` — extend `RemoteServer` interface + `connection_type` union

**Validation gate**:
- `cd backend && go build ./...` compiles without errors
- `cd backend && go test ./internal/models/...` passes
- `cd frontend && npx tsc --noEmit` passes

---

### Commit 1 — Hecate standalone page + route + nav item

**Scope**: New page, new form component, new route, new nav item
**Files**:
- `frontend/src/pages/Hecate.tsx` (new)
- `frontend/src/components/hecate/HecateTunnelForm.tsx` (new)
- `frontend/src/App.tsx` — add lazy import + route
- `frontend/src/components/Layout.tsx` — insert nav item
- `frontend/src/locales/en/translation.json` — `navigation.hecate` + `hecate.page.*` keys
- `frontend/src/pages/__tests__/Hecate.test.tsx` (new)
- `frontend/src/components/hecate/__tests__/HecateTunnelForm.test.tsx` (new)

**Dependencies**: Commit 0
**Validation gate**:
- `npx vitest run src/pages/__tests__/Hecate.test.tsx src/components/hecate/__tests__/HecateTunnelForm.test.tsx` — all pass
- All previously passing tests still pass
- `/hecate` route renders in dev server without errors
- `npm run build` succeeds

---

### Commit 2 — RemoteServerForm Agent/Direct restructure + new device pickers

**Scope**: Restructured form, redesigned ConnectionTypeSelector, 2 new picker components, test rewrites
**Files**:
- `frontend/src/components/RemoteServerForm.tsx` — two-tier UI, remove inline provisioning
- `frontend/src/components/hecate/ConnectionTypeSelector.tsx` — complete redesign
- `frontend/src/components/hecate/NetBirdPeerPicker.tsx` (new)
- `frontend/src/components/hecate/ZeroTierMemberPicker.tsx` (new)
- `frontend/src/locales/en/translation.json` — `hecate.form.mode.*` keys
- `frontend/src/pages/RemoteServers.tsx` — status badge lookup + new connection type display
- `frontend/src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx` — full rewrite
- `frontend/src/components/hecate/__tests__/NetBirdPeerPicker.test.tsx` (new)
- `frontend/src/components/hecate/__tests__/ZeroTierMemberPicker.test.tsx` (new)

**Dependencies**: Commit 0, Commit 1 (for the `/hecate` link in no-providers hint)
**Validation gate**:
- `npx vitest run` — all tests pass
- Remote Server form shows Agent/Direct radio group
- Selecting Agent shows provider dropdown
- Selecting Tailscale + a device auto-fills `host`
- Selecting Direct shows host/port/username fields
- All existing Hecate component tests still pass
- `npm run build` succeeds

---

### Rollback plan

**Backend**: GORM `AutoMigrate` does **not** drop columns automatically. If the PR is reverted, the `hecate_tunnel_uuid` column remains in existing databases. A manual `ALTER TABLE remote_servers DROP COLUMN hecate_tunnel_uuid` is needed if the column must be removed from production. Document this in the PR description.

**Frontend**: Revert the branch. No client-side state migration involved.

**Orthrus provisioning**: The inline "Provision New Agent" workflow returns to `RemoteServerForm` on revert. No data loss.

---

## Appendix A — Files Changed Summary

| File | Change | Commit |
|---|---|---|
| `backend/internal/models/remote_server.go` | Edit | 0 |
| `frontend/src/api/remoteServers.ts` | Edit | 0 |
| `frontend/src/pages/Hecate.tsx` | New | 1 |
| `frontend/src/components/hecate/HecateTunnelForm.tsx` | New | 1 |
| `frontend/src/App.tsx` | Edit | 1 |
| `frontend/src/components/Layout.tsx` | Edit | 1 |
| `frontend/src/locales/en/translation.json` | Edit | 1 + 2 |
| `frontend/src/pages/__tests__/Hecate.test.tsx` | New | 1 |
| `frontend/src/components/hecate/__tests__/HecateTunnelForm.test.tsx` | New | 1 |
| `frontend/src/components/RemoteServerForm.tsx` | Edit | 2 |
| `frontend/src/components/hecate/ConnectionTypeSelector.tsx` | Edit (redesign) | 2 |
| `frontend/src/components/hecate/NetBirdPeerPicker.tsx` | New | 2 |
| `frontend/src/components/hecate/ZeroTierMemberPicker.tsx` | New | 2 |
| `frontend/src/pages/RemoteServers.tsx` | Edit (minor) | 2 |
| `frontend/src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx` | Edit (rewrite) | 2 |
| `frontend/src/components/hecate/__tests__/NetBirdPeerPicker.test.tsx` | New | 2 |
| `frontend/src/components/hecate/__tests__/ZeroTierMemberPicker.test.tsx` | New | 2 |
