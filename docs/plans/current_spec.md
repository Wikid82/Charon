# Hecate Simplified Architecture — Feature Spec

**Branch**: `feature/hecate`
**PR**: #983
**Date**: 2026-05-03

---

## 1. Introduction

### Overview

This spec replaces the previous plan, which suffered from a "double-setup problem": users had to configure Tailscale in Hecate > Providers, and then also configure Tailscale-specific fields on each Orthrus Agent. The new design eliminates that redundancy.

### Core Architecture Principle

Providers are configured **once** in Hecate > Providers. They can then be:

1. **Assigned to an Orthrus Agent** — the agent holds a `hecate_tunnel_uuid` + `device_id` reference. When a Remote Server picks that agent, connectivity is derived automatically from `resolved_address`.
2. **Used directly on a Remote Server** — pick any provider + device directly (no agent required).

```
Providers (configured once in Hecate > Providers):
  └── Tailscale "My Network"     → TunnelConfig UUID = abc-123
  └── Cloudflare "My Tunnel"     → TunnelConfig UUID = def-456
  └── NetBird "My Network"       → TunnelConfig UUID = ghi-789

Agent "prod-server-01":
  └── hecate_tunnel_uuid: "abc-123" (Tailscale "My Network")
  └── device_id: "ts-node-abcde"
  └── resolved_address: "100.64.0.5"  ← cached at assignment time

Remote Server → 3 radio options:
  ├── Direct    → host entered manually
  ├── Agent     → pick agent → host = agent.resolved_address (auto-derived)
  └── Provider  → pick any tunnel + device directly (no agent needed)
```

### Objectives

1. **Problem 1** — Add inline tunnel list + edit button on each provider card in `HecateProviders.tsx`
2. **Problem 2** — Add generic provider assignment to Orthrus agents (backend model + frontend dialog)
3. **Problem 3** — Simplify `RemoteServers` connection UI to 3 clean radios: direct / agent / provider

---

## 2. Research Findings

### 2.1 Backend — Current State

#### `backend/internal/models/orthrus_agent.go`

The `OrthrusAgent` struct currently has **no provider-related fields**:

```go
type OrthrusAgent struct {
    ID            uint
    UUID          string
    Name          string
    AuthKeyHash   string        // json:"-", bcrypt hash
    Status        OrthrusStatus // online / offline / pending
    Capabilities  string        // JSON array
    AgentCertPEM  string
    LastHeartbeat *time.Time
    LastSeen      *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**No `hecate_tunnel_uuid`, `device_id`, or `resolved_address` fields exist.** These must be added.

#### `backend/internal/models/remote_server.go`

`RemoteServer` already has:
- `ConnectionType` — `'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'`
- `OrthrusAgentUUID *string`
- `HecateTunnelUUID *string`

**No `device_id` or `resolved_address` on RemoteServer** — that's correct; these fields live on the agent when using agent mode. In provider mode (direct tunnel selection), host is resolved at device-pick time.

#### `backend/internal/api/handlers/orthrus_handler.go`

Current routes (confirmed):
```
GET    /orthrus/agents
POST   /orthrus/agents
GET    /orthrus/agents/:uuid
PATCH  /orthrus/agents/:uuid   → Rename handler
DELETE /orthrus/agents/:uuid
POST   /orthrus/agents/:uuid/revoke
GET    /orthrus/agents/:uuid/snippets
```

The `PATCH` handler currently uses `renameRequest { Name string binding:"required" }`. This **blocks** partial updates — sending only `hecate_tunnel_uuid` would fail validation. Must replace with `patchAgentRequest` using all-optional pointer fields.

#### `backend/internal/services/orthrus_service.go`

Has `Rename(uuid, newName string)` but no general `Patch` method. Must add one that accepts all 4 optional fields and uses a GORM map-based selective update.

### 2.2 Frontend — Current State

#### `frontend/src/pages/HecateProviders.tsx`

- Shows 4 provider cards (Cloudflare, Tailscale, NetBird, ZeroTier)
- Each card shows tunnel count and a single "Add Provider" button
- **No inline tunnel list** — no way to see existing tunnels from the card
- **No edit button per tunnel** — users cannot edit an existing tunnel from this page
- `HecateTunnelForm` already has `tunnel?: TunnelConfig` prop for edit mode — it is just never passed

#### `frontend/src/components/hecate/HecateTunnelForm.tsx`

- Accepts `tunnel?: TunnelConfig` prop — when present, renders in edit mode (`isEdit = !!tunnel`)
- Initialises name, provider, isActive from the tunnel prop
- Calls `updateTunnel` on submit when `isEdit` is true
- **Already fully supports edit mode — just needs to be wired from `HecateProviders`**

#### `frontend/src/components/hecate/OrthrusAgentManager.tsx`

- Renders a table: Name (inline-editable), UUID, Status, Last Seen, Actions
- Actions column: only Delete button currently
- **No "Assign Provider" action exists**

#### `frontend/src/components/hecate/ConnectionTypeSelector.tsx`

Current type:
```typescript
export type ConnectionMode = 'direct' | 'agent'
```

Renders **2 radios**: Direct / Agent. The Agent mode combines Orthrus agents AND provider tunnels in one dropdown — these are logically separate concepts and must be split.

#### `frontend/src/components/RemoteServerForm.tsx`

Current `resolveConnectionMode()` collapses all non-direct modes into `'agent'`. The form also has an `orthrus_ip_mode` state (`'' | 'tailscale' | 'netbird' | 'zerotier' | 'manual'`) that adds complexity. The simplified design removes this in favour of the 3-radio model.

#### Existing Device Pickers (confirmed, no changes required)

- `frontend/src/components/hecate/TailscaleDevicePicker.tsx` — accepts `onSelect(device)`, `open`, `onClose`
- `frontend/src/components/hecate/NetBirdPeerPicker.tsx` — accepts `onSelect(peer)`, `selectedId`, `open`, `onClose`
- `frontend/src/components/hecate/ZeroTierMemberPicker.tsx` — two-step (network → member), accepts `onSelect(member)`, `open`, `onClose`

#### Existing i18n Keys Present

```json
"hecate.form.mode.label"
"hecate.form.mode.direct"
"hecate.form.mode.agent"
"hecate.form.mode.directDescription"
"hecate.form.mode.agentDescription"
"hecate.form.mode.provider"        ← label exists, description missing
"hecate.form.mode.selectProvider"
"hecate.form.mode.noProviders"
"hecate.form.mode.goToHecate"
"hecate.form.mode.selectedDevice"
"hecate.form.mode.changeDevice"
"hecate.providers.title"
"hecate.providers.description"
"hecate.providers.tunnelCount_one"
"hecate.providers.tunnelCount_other"
"hecate.agentManager.tableLabel"
"hecate.agentManager.colName"
"hecate.agentManager.colUUID"
"hecate.agentManager.colStatus"
"hecate.agentManager.colLastSeen"
"hecate.agentManager.colActions"
```

---

## 3. Technical Specifications

### 3.1 Problem 1 — HecateProviders: Inline Tunnel List + Edit

#### `frontend/src/pages/HecateProviders.tsx` — Changes

**New state:**

```typescript
const [editTunnel, setEditTunnel] = useState<TunnelConfig | null>(null)
const [editFormOpen, setEditFormOpen] = useState(false)
```

**Helper handlers:**

```typescript
const openEdit = (tunnel: TunnelConfig) => {
  setEditTunnel(tunnel)
  setEditFormOpen(true)
}

const openCreate = (provider: TunnelProvider) => {
  setEditTunnel(null)
  setSelectedProvider(provider)
  setFormOpen(true)
}
```

**Updated provider card — inline tunnel list section:**

```tsx
{/* Inline tunnel list */}
<ul aria-label={t('hecate.providers.tunnelCount_other', { count: providerTunnels.length })}>
  {providerTunnels.map(tun => (
    <li
      key={tun.uuid}
      className="flex items-center justify-between text-sm py-1 px-2 rounded bg-surface-subtle"
    >
      <div className="flex items-center gap-2 min-w-0">
        <span className="font-medium text-content-primary truncate">{tun.name}</span>
        <TunnelStatusBadge state={getStatus(tun.uuid)?.state ?? 'stopped'} />
      </div>
      <button
        type="button"
        className="p-1 rounded text-content-tertiary hover:text-brand-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
        aria-label={t('hecate.providers.editTunnel', { name: tun.name })}
        onClick={() => openEdit(tun)}
      >
        <Settings className="w-4 h-4" aria-hidden="true" />
      </button>
    </li>
  ))}
  {providerTunnels.length === 0 && (
    <li className="text-xs text-content-muted py-1" aria-live="polite">
      {t('hecate.providers.noTunnels')}
    </li>
  )}
</ul>
```

**Second `HecateTunnelForm` instance (edit mode):**

```tsx
{editFormOpen && (
  <HecateTunnelForm
    tunnel={editTunnel ?? undefined}
    onCancel={() => { setEditFormOpen(false); setEditTunnel(null) }}
  />
)}
```

**No backend changes required for Problem 1** — `HecateTunnelForm` calls `updateTunnel` → `PUT /hecate/tunnels/:uuid`, already implemented.

---

### 3.2 Problem 2 — Agent Generic Provider Assignment

#### 3.2.1 Backend Model — `backend/internal/models/orthrus_agent.go`

Add 3 nullable pointer fields. GORM AutoMigrate adds the columns on next startup:

```go
// HecateTunnelUUID is the TunnelConfig assigned to THIS AGENT for its own outbound
// connectivity. Distinct from RemoteServer.HecateTunnelUUID which governs how a
// RemoteServer is reached by Charon.
HecateTunnelUUID *string `json:"hecate_tunnel_uuid,omitempty" gorm:"index"`

// DeviceID is the provider-specific peer/device/member identifier within the
// assigned tunnel (e.g. Tailscale node ID, NetBird peer ID). Empty for Cloudflare.
DeviceID *string `json:"device_id,omitempty"`

// ResolvedAddress is the cached connectivity address for this agent,
// set by Charon at assignment time, used as upstream host in Remote Servers.
ResolvedAddress *string `json:"resolved_address,omitempty"`
```

#### 3.2.2 Backend Service — `backend/internal/services/orthrus_service.go`

Replace `Rename` with `Patch`. Use GORM map-based updates to avoid zero-value overwrites:

```go
// Patch applies a partial update to an OrthrusAgent.
// Only non-nil pointer fields are written; omitted fields are left unchanged.
func (s *OrthrusService) Patch(
    uuid string,
    name, hecateTunnelUUID, deviceID, resolvedAddress *string,
) (*models.OrthrusAgent, error) {
    updates := map[string]interface{}{}

    if name != nil {
        trimmed := strings.TrimSpace(*name)
        if trimmed == "" {
            return nil, fmt.Errorf("orthrus: agent name cannot be blank")
        }
        updates["name"] = trimmed
    }
    if hecateTunnelUUID != nil {
        updates["hecate_tunnel_uuid"] = *hecateTunnelUUID
    }
    if deviceID != nil {
        updates["device_id"] = *deviceID
    }
    if resolvedAddress != nil {
        updates["resolved_address"] = *resolvedAddress
    }

    if len(updates) == 0 {
        return s.Get(uuid)
    }

    if err := s.db.Model(&models.OrthrusAgent{}).
        Where("uuid = ?", uuid).
        Updates(updates).Error; err != nil {
        return nil, fmt.Errorf("orthrus: patch agent %s: %w", uuid, err)
    }
    return s.Get(uuid)
}
```

Remove the old `Rename` method (subsumed by `Patch`).

#### 3.2.3 Backend Handler — `backend/internal/api/handlers/orthrus_handler.go`

Replace `renameRequest` struct with `patchAgentRequest` (all-optional pointers):

```go
type patchAgentRequest struct {
    Name             *string `json:"name"`
    HecateTunnelUUID *string `json:"hecate_tunnel_uuid"`
    DeviceID         *string `json:"device_id"`
    ResolvedAddress  *string `json:"resolved_address"`
}
```

Replace `Rename` handler with `Patch`:

```go
func (h *OrthrusHandler) Patch(c *gin.Context) {
    uuid := c.Param("uuid")
    var req patchAgentRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    agent, err := h.svc.Patch(uuid, req.Name, req.HecateTunnelUUID, req.DeviceID, req.ResolvedAddress)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, agent)
}
```

Route registration change:
```go
// Before:
rg.PATCH("/orthrus/agents/:uuid", h.Rename)
// After:
rg.PATCH("/orthrus/agents/:uuid", h.Patch)
```

#### 3.2.4 API Contract

```
PATCH /orthrus/agents/:uuid

Request (all fields optional):
{
  "name": "string",                  // omit to keep current name
  "hecate_tunnel_uuid": "string",    // omit to keep current tunnel
  "device_id": "string",             // omit to keep current device
  "resolved_address": "string"       // omit to keep current address
}

Response 200: OrthrusAgent (full object)
Response 400: { "error": "..." }     // invalid JSON or blank name
Response 500: { "error": "..." }     // DB error or agent not found
```

#### 3.2.5 Frontend API Types — `frontend/src/api/orthrus.ts`

Add new fields to `OrthrusAgent` interface:

```typescript
export interface OrthrusAgent {
  uuid: string;
  name: string;
  status: OrthrusStatus;
  capabilities: string;
  agent_cert_pem?: string;
  last_heartbeat?: string;
  last_seen?: string;
  created_at: string;
  updated_at: string;
  // Provider assignment — set via Assign Provider dialog
  hecate_tunnel_uuid?: string;  // Agent's own outbound tunnel (≠ RemoteServer.hecate_tunnel_uuid)
  device_id?: string;           // Provider-specific peer ID; empty string for Cloudflare
  resolved_address?: string;    // Cached host: IP for Tailscale/NetBird, hostname for Cloudflare
}

export interface PatchAgentRequest {
  name?: string;
  hecate_tunnel_uuid?: string;
  device_id?: string;
  resolved_address?: string;
}

export const patchAgent = async (uuid: string, req: PatchAgentRequest): Promise<OrthrusAgent> => {
  const { data } = await client.patch<OrthrusAgent>(`/orthrus/agents/${uuid}`, req);
  return data;
};

// Backward-compatible wrapper — existing callers of renameAgent are unaffected
export const renameAgent = async (uuid: string, name: string): Promise<OrthrusAgent> =>
  patchAgent(uuid, { name });
```

#### 3.2.6 Frontend Hook — `frontend/src/hooks/useOrthrus.ts`

```typescript
export const usePatchAgent = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ uuid, req }: { uuid: string; req: PatchAgentRequest }) =>
      patchAgent(uuid, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: AGENTS_QUERY_KEY });
    },
  });
};
```

`useRenameAgent` stays unchanged — it still calls `renameAgent`.

#### 3.2.7 `OrthrusAgentManager.tsx` — Assign Provider Dialog

**New "Provider" column header** (between Status and Last Seen):

```tsx
<th scope="col" className="py-3 px-4 text-left text-xs font-semibold text-content-secondary uppercase tracking-wide">
  {t('hecate.agentManager.colProvider')}
</th>
```

**New "Provider" cell per row** — shows `resolved_address` or "No provider assigned":

```tsx
<td className="py-3 px-4 text-xs text-content-secondary">
  {agent.hecate_tunnel_uuid
    ? <span className="text-content-primary font-mono">
        {agent.resolved_address ?? agent.device_id ?? '—'}
      </span>
    : <span className="text-content-muted italic">
        {t('hecate.agentManager.noProviderAssigned')}
      </span>
  }
</td>
```

**New action button in Actions column** — "Assign Provider":

```tsx
<button
  type="button"
  className="p-1 rounded text-content-tertiary hover:text-brand-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
  aria-label={t('hecate.agentManager.assignProvider', { name: agent.name })}
  onClick={() => setProviderOpen(true)}
>
  <Link2 className="h-3.5 w-3.5" aria-hidden="true" />
</button>
```

**`AssignProviderDialog` component** (in same file or extracted to `AssignProviderDialog.tsx`):

```tsx
interface AssignProviderDialogProps {
  agent: OrthrusAgent
  open: boolean
  onClose: () => void
}

function AssignProviderDialog({ agent, open, onClose }: AssignProviderDialogProps) {
  const { t } = useTranslation()
  const { tunnels } = useHecate()
  const { mutate: patch, isPending } = usePatchAgent()

  const [selectedTunnelUUID, setSelectedTunnelUUID] = useState(agent.hecate_tunnel_uuid ?? '')
  const [deviceId, setDeviceId] = useState(agent.device_id ?? '')
  const [resolvedAddress, setResolvedAddress] = useState(agent.resolved_address ?? '')
  const [pickerOpen, setPickerOpen] = useState(false)

  const selectedTunnel = tunnels.find(tn => tn.uuid === selectedTunnelUUID)
  const provider = selectedTunnel?.provider as TunnelProvider | undefined

  const handleSave = () => {
    patch(
      {
        uuid: agent.uuid,
        req: {
          hecate_tunnel_uuid: selectedTunnelUUID || undefined,
          device_id: deviceId || undefined,
          resolved_address: resolvedAddress || undefined,
        },
      },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent aria-labelledby="assign-provider-title">
        <DialogHeader>
          <DialogTitle id="assign-provider-title">
            {t('hecate.agentManager.assignProviderTitle', { name: agent.name })}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Tunnel selector (grouped by provider type) */}
          <div>
            <label htmlFor="assign-tunnel" className="block text-sm font-medium mb-1">
              {t('hecate.agentManager.providerTunnel')}
            </label>
            <select
              id="assign-tunnel"
              value={selectedTunnelUUID}
              onChange={e => {
                setSelectedTunnelUUID(e.target.value)
                setDeviceId('')
                setResolvedAddress('')
              }}
              className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">{t('hecate.form.mode.selectProvider')}</option>
              {(['cloudflare', 'tailscale', 'netbird', 'zerotier'] as const).map(p => {
                const pts = tunnels.filter(tn => tn.provider === p)
                if (!pts.length) return null
                return (
                  <optgroup key={p} label={p.charAt(0).toUpperCase() + p.slice(1)}>
                    {pts.map(tn => (
                      <option key={tn.uuid} value={tn.uuid}>{tn.name}</option>
                    ))}
                  </optgroup>
                )
              })}
            </select>
          </div>

          {/* Device picker trigger (non-Cloudflare only) */}
          {selectedTunnel && provider !== 'cloudflare' && (
            <div>
              <p className="text-sm font-medium mb-1">{t('hecate.agentManager.deviceId')}</p>
              {deviceId && (
                <p className="text-xs font-mono text-content-secondary mb-1">{deviceId}</p>
              )}
              <button
                type="button"
                className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                onClick={() => setPickerOpen(true)}
              >
                {t('hecate.form.mode.selectDevice')}
              </button>
            </div>
          )}

          {/* Cloudflare hostname input — replaces device picker for Cloudflare tunnels */}
          {selectedTunnel && provider === 'cloudflare' && (
            <div className="space-y-1">
              <label htmlFor="assign-cf-hostname" className="block text-sm font-medium">
                {t('hecate.form.provider.cloudflareTunnelHostname')}
              </label>
              <input
                id="assign-cf-hostname"
                type="text"
                placeholder="app.example.com"
                value={resolvedAddress}
                onChange={e => {
                  setResolvedAddress(e.target.value)
                  setDeviceId('')
                }}
                className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-describedby="assign-cf-hostname-hint"
              />
              <p id="assign-cf-hostname-hint" className="text-xs text-content-muted">
                {t('hecate.form.provider.cloudflareTunnelHint')}
              </p>
            </div>
          )}

          {/* Resolved address (auto-filled from device pick, editable — non-Cloudflare only) */}
          {selectedTunnel && provider !== 'cloudflare' && (
            <div>
              <label htmlFor="assign-resolved" className="block text-sm font-medium mb-1">
                {t('hecate.agentManager.resolvedAddress')}
              </label>
              <input
                id="assign-resolved"
                type="text"
                value={resolvedAddress}
                onChange={e => setResolvedAddress(e.target.value)}
                placeholder="100.x.x.x or hostname"
                className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          )}
        </div>

        <DialogFooter>
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="px-4 py-2 rounded text-sm text-content-secondary hover:text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {t('common.cancel')}
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isPending || !selectedTunnelUUID}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
          >
            {t('hecate.agentManager.saveProviderAssignment')}
          </button>
        </DialogFooter>

        {/* Provider-specific device pickers */}
        {pickerOpen && provider === 'tailscale' && (
          <TailscaleDevicePicker
            open
            onClose={() => setPickerOpen(false)}
            onSelect={device => {
              setDeviceId(device.id)
              setResolvedAddress(device.addresses[0] ?? '')
              setPickerOpen(false)
            }}
          />
        )}
        {pickerOpen && provider === 'netbird' && (
          <NetBirdPeerPicker
            open
            onClose={() => setPickerOpen(false)}
            onSelect={peer => {
              setDeviceId(peer.id)
              setResolvedAddress(peer.ip)
              setPickerOpen(false)
            }}
            selectedId={deviceId}
          />
        )}
        {pickerOpen && provider === 'zerotier' && (
          <ZeroTierMemberPicker
            open
            onClose={() => setPickerOpen(false)}
            onSelect={member => {
              setDeviceId(member.node_id)
              setResolvedAddress(member.ip_assignments[0] ?? '')
              setPickerOpen(false)
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
```

---

### 3.3 Problem 3 — RemoteServers: 3-Radio Connection Model

#### 3.3.1 `ConnectionTypeSelector.tsx` — Updated Types and Props

```typescript
export type ConnectionMode = 'direct' | 'agent' | 'provider'
export type ConnectionType = 'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'
export type HecateProvider = 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier'

export interface ConnectionTypeSelectorProps {
  mode: ConnectionMode
  onModeChange: (mode: ConnectionMode) => void
  selectedTunnelUUID: string | null
  selectedDeviceId: string | null
  selectedAgentUUID: string | null
  onTunnelSelect: (tunnelUUID: string, provider: HecateProvider) => void
  onDeviceSelect: (deviceId: string, resolvedAddress: string) => void
  onAgentSelect: (agentUUID: string) => void
  disabled?: boolean
}
```

**3-radio render + tier-2 pickers:**

```tsx
{/* 3 radio buttons */}
<fieldset>
  <legend className="sr-only">{t('hecate.form.mode.label')}</legend>
  <div role="radiogroup" aria-label={t('hecate.form.mode.label')} className="flex flex-col gap-3">

    {/* Direct */}
    <label className="flex items-start gap-2 cursor-pointer">
      <input type="radio" name="connection-mode" value="direct"
        checked={mode === 'direct'} onChange={() => onModeChange('direct')}
        className="mt-0.5 w-4 h-4" />
      <div>
        <span className="text-sm font-medium text-content-primary">{t('hecate.form.mode.direct')}</span>
        <p className="text-xs text-content-muted">{t('hecate.form.mode.directDescription')}</p>
      </div>
    </label>

    {/* Agent */}
    <label className="flex items-start gap-2 cursor-pointer">
      <input type="radio" name="connection-mode" value="agent"
        checked={mode === 'agent'} onChange={() => onModeChange('agent')}
        className="mt-0.5 w-4 h-4" />
      <div>
        <span className="text-sm font-medium text-content-primary">{t('hecate.form.mode.agent')}</span>
        <p className="text-xs text-content-muted">{t('hecate.form.mode.agentDescription')}</p>
      </div>
    </label>

    {/* Provider */}
    <label className="flex items-start gap-2 cursor-pointer">
      <input type="radio" name="connection-mode" value="provider"
        checked={mode === 'provider'} onChange={() => onModeChange('provider')}
        className="mt-0.5 w-4 h-4" />
      <div>
        <span className="text-sm font-medium text-content-primary">{t('hecate.form.mode.providerLabel')}</span>
        <p className="text-xs text-content-muted">{t('hecate.form.mode.providerDescription')}</p>
      </div>
    </label>
  </div>
</fieldset>

{/* Tier 2a: Agent picker */}
{mode === 'agent' && (
  <div>
    <label htmlFor="cts-agent" className="block text-sm font-medium mb-1">
      {t('hecate.form.mode.agent')}
    </label>
    <select id="cts-agent" value={selectedAgentUUID ?? ''}
      onChange={e => onAgentSelect(e.target.value)}
      disabled={disabled}
      aria-describedby={noProviderWarningId}
      className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-60"
    >
      <option value="">{t('hecate.form.mode.selectAgent')}</option>
      {agents.map(agent => (
        <option key={agent.uuid} value={agent.uuid}>
          {agent.name}
          {!agent.resolved_address ? ` (${t('hecate.agentManager.noProviderAssigned')})` : ''}
        </option>
      ))}
    </select>
    {/* Warning when selected agent has no provider */}
    {selectedAgentUUID && !selectedAgent?.resolved_address && (
      <p id={noProviderWarningId} className="text-xs text-amber-400 mt-1" role="alert">
        {t('hecate.form.mode.agent.noProviderWarning')}{' '}
        <a href="/hecate/agent" className="underline hover:text-amber-300 focus:outline-none focus:ring-1 focus:ring-amber-400 rounded">
          {t('hecate.form.mode.agent.noProviderLink')}
        </a>
      </p>
    )}
  </div>
)}

{/* Tier 2b: Provider tunnel picker */}
{mode === 'provider' && (
  <ProviderDevicePicker
    selectedTunnelUUID={selectedTunnelUUID}
    selectedDeviceId={selectedDeviceId}
    tunnels={tunnels}
    onTunnelSelect={onTunnelSelect}
    onDeviceSelect={onDeviceSelect}
    disabled={disabled}
  />
)}
```

#### 3.3.2 New `ProviderDevicePicker` Component

File: `frontend/src/components/hecate/ProviderDevicePicker.tsx`

```typescript
interface ProviderDevicePickerProps {
  selectedTunnelUUID: string | null
  selectedDeviceId: string | null
  tunnels: TunnelConfig[]
  onTunnelSelect: (tunnelUUID: string, provider: HecateProvider) => void
  onDeviceSelect: (deviceId: string, resolvedAddress: string) => void
  disabled?: boolean
}
```

Renders tunnel dropdown + conditionally shows either a freeform hostname text input (for `cloudflare`) or a device picker trigger button (for `tailscale`, `netbird`, `zerotier`) based on the selected tunnel's provider. Manages internal picker dialog state. `onDeviceSelect(deviceId, resolvedAddress)` propagates the final pair up to the form.

**Cloudflare branch** — when `provider === 'cloudflare'`, renders a hostname input instead of the device picker trigger. Adds `cloudflareHost` local state that resets when the tunnel selection changes:

```tsx
const [cloudflareHost, setCloudflareHost] = useState('')

{provider === 'cloudflare' && (
  <div className="space-y-1">
    <label htmlFor="cf-hostname" className="text-sm text-content-secondary">
      {t('hecate.form.provider.cloudflareTunnelHostname')}
    </label>
    <input
      id="cf-hostname"
      type="text"
      placeholder="app.example.com"
      value={cloudflareHost}
      onChange={e => {
        setCloudflareHost(e.target.value)
        onDeviceSelect('', e.target.value)
      }}
      className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
      aria-describedby="cf-hostname-hint"
    />
    <p id="cf-hostname-hint" className="text-xs text-content-muted">
      {t('hecate.form.provider.cloudflareTunnelHint')}
    </p>
  </div>
)}
```

When the tunnel dropdown changes, reset `cloudflareHost` to `''` to clear any previously entered hostname.

**Non-Cloudflare path** — show the existing device picker trigger button for Tailscale / NetBird / ZeroTier.

#### 3.3.3 `RemoteServerForm.tsx` — Simplified

**Updated `resolveConnectionMode()`:**

```typescript
const resolveConnectionMode = (): ConnectionMode => {
  if (!server?.connection_type || server.connection_type === 'direct') return 'direct'
  if (server.connection_type === 'orthrus') return 'agent'
  return 'provider' // cloudflare | tailscale | netbird | zerotier
}
```

**Simplified form state** — remove `orthrus_ip_mode`:

```typescript
const [formData, setFormData] = useState({
  name: server?.name || '',
  provider: server?.provider || 'generic',
  host: server?.host || '',
  port: server?.port ?? 22,
  username: server?.username || '',
  enabled: server?.enabled ?? true,
  connection_mode: resolveConnectionMode() as ConnectionMode,
  connection_type: (server?.connection_type ?? 'direct') as ConnectionType,
  orthrus_agent_uuid: server?.orthrus_agent_uuid ?? '',
  hecate_tunnel_uuid: server?.hecate_tunnel_uuid ?? '',
  device_id: '',
  resolved_address: '',
})
```

**Updated submit payload logic:**

```typescript
if (formData.connection_mode === 'direct') {
  payload.host = formData.host
  payload.port = formData.port
  payload.username = formData.username
  payload.orthrus_agent_uuid = undefined
  payload.hecate_tunnel_uuid = undefined
} else if (formData.connection_mode === 'agent') {
  const agent = agents.find(a => a.uuid === formData.orthrus_agent_uuid)
  payload.host = agent?.resolved_address ?? formData.host
  payload.port = formData.port
  payload.orthrus_agent_uuid = formData.orthrus_agent_uuid || undefined
  payload.hecate_tunnel_uuid = undefined
} else if (formData.connection_mode === 'provider') {
  payload.host = formData.resolved_address || formData.host
  payload.port = formData.port
  payload.hecate_tunnel_uuid = formData.hecate_tunnel_uuid || undefined
  payload.orthrus_agent_uuid = undefined
}
```

**Updated `ConnectionTypeSelector` props wiring:**

```tsx
<ConnectionTypeSelector
  mode={formData.connection_mode}
  onModeChange={mode => {
    setFormData(prev => ({
      ...prev,
      connection_mode: mode,
      connection_type: mode === 'direct' ? 'direct' : prev.connection_type,
      hecate_tunnel_uuid: mode === 'provider' ? prev.hecate_tunnel_uuid : '',
      orthrus_agent_uuid: mode === 'agent' ? prev.orthrus_agent_uuid : '',
      device_id: '',
      resolved_address: '',
    }))
  }}
  selectedTunnelUUID={formData.hecate_tunnel_uuid || null}
  selectedDeviceId={formData.device_id || null}
  selectedAgentUUID={formData.orthrus_agent_uuid || null}
  onTunnelSelect={(uuid, p) =>
    setFormData(prev => ({
      ...prev,
      connection_type: p as ConnectionType,
      hecate_tunnel_uuid: uuid,
      device_id: '',
      resolved_address: '',
    }))
  }
  onDeviceSelect={(deviceId, resolvedAddress) =>
    setFormData(prev => ({ ...prev, device_id: deviceId, resolved_address: resolvedAddress }))
  }
  onAgentSelect={uuid =>
    setFormData(prev => ({
      ...prev,
      connection_type: 'orthrus',
      orthrus_agent_uuid: uuid,
      hecate_tunnel_uuid: '',
      device_id: '',
      resolved_address: '',
    }))
  }
  disabled={loading}
/>
```

**Remove from `RemoteServerForm`:** All `showTailscalePicker`, `showNetBirdPicker`, `showZeroTierPicker` state variables and their corresponding JSX (device picking is now internal to `ProviderDevicePicker`).

**Remove from `RemoteServerForm`:** `orthrus_ip_mode` state and all conditional rendering based on it.

#### 3.3.4 Data Flow Diagram

```
User selects radio →
  'direct'   → host/port/username fields visible
               → payload: host=formData.host

  'agent'    → Agent picker dropdown
               → pick agent UUID
               → if agent.resolved_address: host derived automatically
               → if not: warning shown with link to Agent page
               → payload: host=agent.resolved_address, connection_type='orthrus'

  'provider' → Tunnel dropdown (grouped by provider type)
               → pick tunnel UUID
               → if non-Cloudflare: device picker opens
               → onDeviceSelect(deviceId, resolvedAddress)
               → payload: host=resolvedAddress, connection_type=provider,
                           hecate_tunnel_uuid=tunnelUUID
```

---

## 4. i18n Keys to Add

All additions go in `frontend/src/locales/en/translation.json`:

**Under `"hecate.providers"`:**
```json
"editTunnel": "Edit {{name}}",
"noTunnels": "No tunnels configured."
```

**Under `"hecate.agentManager"`:**
```json
"colProvider": "Provider",
"assignProvider": "Assign provider to {{name}}",
"assignProviderTitle": "Assign Provider — {{name}}",
"providerTunnel": "Provider Tunnel",
"deviceId": "Device ID",
"resolvedAddress": "Resolved Address",
"noProviderAssigned": "No provider assigned",
"saveProviderAssignment": "Save Assignment"
```

**Under `"hecate.form.mode"`:**
```json
"providerLabel": "Provider",
"providerDescription": "Route via a configured network provider (Tailscale, NetBird, etc.)",
"selectAgent": "Select an agent",
"selectDevice": "Select device",
"agent": {
  "noProviderWarning": "This agent has no provider assigned — connectivity address unavailable.",
  "noProviderLink": "Assign one on the Agent page."
}
```

**Under `"hecate.form.provider"`:**
```json
"cloudflareTunnelHostname": "Tunnel Hostname",
"cloudflareTunnelHint": "Enter the public hostname configured for this Cloudflare tunnel (e.g. app.example.com)"
```

**Under `"remoteServers"`:**
```json
"connectionTypeProvider": "Provider"
```

---

## 5. Implementation Plan

### Phase 1 — Playwright Tests (Expected Behaviour First)

**New test files** (should fail until implementation is complete):

- `tests/hecate-providers-edit.spec.ts`
  - Provider card shows inline tunnel list
  - Edit button per tunnel opens form with tunnel name pre-filled
  - Save calls update endpoint and refreshes list

- `tests/hecate-agent-provider.spec.ts`
  - Agent row has "Assign Provider" button
  - Dialog shows tunnel dropdown grouped by provider
  - Selecting non-Cloudflare tunnel shows device picker trigger
  - After selection, resolved address field is populated
  - Save updates agent and shows resolved address in table

- `tests/remote-server-3modes.spec.ts`
  - Connection mode has exactly 3 radios: Direct, Agent, Provider
  - Selecting Agent shows only agent picker
  - Selecting Provider shows only tunnel + device picker
  - Agent with no resolved_address shows warning
  - Tailscale is not a top-level radio option
  - `ProviderDevicePicker` shows text input instead of device picker when Cloudflare provider is selected
  - Cloudflare case: user selects Cloudflare tunnel → enters hostname → submits → `host` field equals the typed hostname

### Phase 2 — Backend (Problem 2)

**Files and changes:**

| File | Change |
|---|---|
| `backend/internal/models/orthrus_agent.go` | Add `HecateTunnelUUID`, `DeviceID`, `ResolvedAddress` fields |
| `backend/internal/services/orthrus_service.go` | Replace `Rename` with `Patch` (map-based partial update) |
| `backend/internal/api/handlers/orthrus_handler.go` | Replace `renameRequest`/`Rename` with `patchAgentRequest`/`Patch` |

**Validation:**
```bash
cd /projects/Charon/backend && go build ./... && go vet ./...
cd /projects/Charon && go test ./backend/...
```

### Phase 3 — Frontend (Problems 1, 2, 3)

**File order (dependencies flow downward):**

1. `frontend/src/api/orthrus.ts` — extend `OrthrusAgent`, add `patchAgent`
2. `frontend/src/hooks/useOrthrus.ts` — add `usePatchAgent`
3. `frontend/src/pages/HecateProviders.tsx` — inline list + edit wiring (Problem 1)
4. `frontend/src/components/hecate/OrthrusAgentManager.tsx` — Provider column + AssignProviderDialog (Problem 2)
5. `frontend/src/components/hecate/ProviderDevicePicker.tsx` — **new file** (Problem 3 dependency)
6. `frontend/src/components/hecate/ConnectionTypeSelector.tsx` — 3-radio mode (Problem 3)
7. `frontend/src/components/RemoteServerForm.tsx` — simplified form (Problem 3)
8. `frontend/src/locales/en/translation.json` — all new i18n keys

**Validation:**
```bash
cd /projects/Charon/frontend && node node_modules/.bin/vitest run --reporter=verbose
```

### Phase 4 — Integration and Testing

**Update existing tests:**
- `frontend/src/pages/__tests__/HecateProviders.test.tsx` — inline list + edit flow tests
- `frontend/src/pages/__tests__/HecateAgent.test.tsx` — Assign Provider dialog tests
- Any test that imports `ConnectionMode` type must add `'provider'` to its type assertions

**New test files:**
- `frontend/src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx`
- `frontend/src/components/hecate/__tests__/ProviderDevicePicker.test.tsx`

**Run E2E:**
```bash
cd /projects/Charon && npx playwright test tests/hecate-providers-edit.spec.ts tests/hecate-agent-provider.spec.ts tests/remote-server-3modes.spec.ts --project=firefox
```

### Phase 5 — GORM Security Scan + DoD

```bash
./scripts/scan-gorm-security.sh --check
```

New `OrthrusAgent` fields (`HecateTunnelUUID`, `DeviceID`, `ResolvedAddress`) are not credentials — scan should pass cleanly.

Run local patch coverage report after backend and frontend coverage tests.

---

## 6. Acceptance Criteria

### Problem 1

- [ ] Each provider card lists its tunnels inline with name + status badge
- [ ] Each tunnel row has an accessible Edit button (`aria-label` includes tunnel name)
- [ ] Clicking Edit opens `HecateTunnelForm` in edit mode with tunnel name pre-filled
- [ ] Editing and saving calls `PUT /hecate/tunnels/:uuid` and refreshes the list
- [ ] Provider cards with no tunnels show "No tunnels configured" text
- [ ] "Add Provider" still opens form in create mode

### Problem 2

- [ ] `orthrus_agents` DB table gains `hecate_tunnel_uuid`, `device_id`, `resolved_address` columns after AutoMigrate
- [ ] `PATCH /orthrus/agents/:uuid` with only `{ "name": "foo" }` renames successfully
- [ ] `PATCH /orthrus/agents/:uuid` with only `{ "hecate_tunnel_uuid": "...", "device_id": "...", "resolved_address": "..." }` assigns provider without changing name
- [ ] Agent table has "Provider" column showing resolved address or "No provider assigned"
- [ ] Assign Provider button per row opens dialog
- [ ] Dialog tunnel dropdown groups tunnels by provider type (Cloudflare / Tailscale / NetBird / ZeroTier)
- [ ] Selecting Tailscale tunnel shows TailscaleDevicePicker trigger
- [ ] Selecting NetBird tunnel shows NetBirdPeerPicker trigger
- [ ] Selecting ZeroTier tunnel shows ZeroTierMemberPicker trigger
- [ ] Selecting Cloudflare tunnel shows no device picker (cloudflare = tunnel IS the endpoint)
- [ ] After picking a device, Resolved Address is auto-filled
- [ ] Saving updates the agent and table refreshes with new resolved address
- [ ] `useRenameAgent` inline rename in the table still works unchanged

### Problem 3

- [ ] Connection mode selector shows exactly 3 radios: Direct, Agent, Provider
- [ ] Selecting Direct shows host/port/username fields; hides agent and tunnel pickers
- [ ] Selecting Agent shows agent picker dropdown; hides tunnel picker; hides host field
- [ ] Selecting Provider shows tunnel picker grouped by type + device picker; hides agent picker; hides host field
- [ ] Agent picker option labels indicate "(No provider assigned)" when `resolved_address` is empty
- [ ] Warning + link shown when selected agent has no `resolved_address`
- [ ] Saving in Agent mode sends `host = agent.resolved_address`, `connection_type = 'orthrus'`
- [ ] Saving in Provider mode sends `host = device.resolvedAddress`, `connection_type = provider`, `hecate_tunnel_uuid`
- [ ] Tailscale does NOT appear as a top-level radio — only under Provider > Tailscale group

---

## 7. Commit Slicing Strategy

### Decision

Single PR #983 with 3 ordered commits. Commits 1 and 2 are independently compilable and have standalone test gates. Commit 3 has a TypeScript type dependency on Commit 2 (`OrthrusAgent.resolved_address` must be present in `api/orthrus.ts`) and must be developed and reviewed after Commit 2 is merged or cherry-picked.

### Commit 1 — Frontend only (Problem 1)

**Subject**: `feat(hecate/providers): add inline tunnel list with edit support`

**Files:**
- `frontend/src/pages/HecateProviders.tsx`
- `frontend/src/locales/en/translation.json` (`hecate.providers.editTunnel`, `hecate.providers.noTunnels`)
- `frontend/src/pages/__tests__/HecateProviders.test.tsx`

**Dependencies**: None — HecateTunnelForm edit mode already works
**Validation gate**: `node node_modules/.bin/vitest run src/pages/__tests__/HecateProviders.test.tsx`
**Risk**: Low — purely additive

### Commit 2 — Backend + Frontend (Problem 2)

**Subject**: `feat(orthrus): add generic provider assignment to agents`

**Files:**
- `backend/internal/models/orthrus_agent.go`
- `backend/internal/services/orthrus_service.go`
- `backend/internal/api/handlers/orthrus_handler.go`
- `backend/internal/api/handlers/orthrus_handler_test.go` *(new test cases — see below)*
- `frontend/src/api/orthrus.ts`
- `frontend/src/api/__tests__/orthrus.test.ts` *(new test cases — see below)*
- `frontend/src/hooks/useOrthrus.ts`
- `frontend/src/components/hecate/OrthrusAgentManager.tsx`
- `frontend/src/locales/en/translation.json` (agentManager keys)
- `frontend/src/pages/__tests__/HecateAgent.test.tsx`

**New backend test cases** (`orthrus_handler_test.go`):
- `TestOrthrusHandler_PatchAgent_NameOnly` — PATCH with only `{ "name": "new-name" }` → 200, name updated, tunnel fields unchanged
- `TestOrthrusHandler_PatchAgent_TunnelFields` — PATCH with only tunnel fields → 200, name unchanged, tunnel fields updated
- `TestOrthrusHandler_PatchAgent_EmptyBody` — PATCH with `{}` → 200, returns current agent unchanged
- `TestOrthrusHandler_PatchAgent_UnknownUUID` — PATCH with valid body but unknown UUID → 404

**New frontend API test cases** (`orthrus.test.ts`):
- `renameAgent delegates to patchAgent with only name field` — verify `axios.patch` called with `{ name: 'foo' }` and no other fields
- `patchAgent with provider fields sends correct partial payload` — verify only the provided fields appear in the PATCH body

**Dependencies**: None (fully independent of Commit 1)
**Validation gate**:
```bash
cd /projects/Charon/backend && go build ./... && go vet ./... && go test ./backend/internal/api/handlers/...
cd /projects/Charon/frontend && node node_modules/.bin/vitest run src/pages/__tests__/HecateAgent.test.tsx src/api/__tests__/orthrus.test.ts
```
**Risk**: Medium — AutoMigrate adds nullable columns (no data loss)
**Rollback note**: Orphaned DB columns are harmless if reverted

### Commit 3 — Frontend only (Problem 3, gates on Commit 2 type)

**Subject**: `feat(remote-servers): replace 2-radio connection mode with 3-radio model`

**Files:**
- `frontend/src/components/hecate/ConnectionTypeSelector.tsx`
- `frontend/src/components/hecate/ProviderDevicePicker.tsx` (new)
- `frontend/src/components/RemoteServerForm.tsx`
- `frontend/src/locales/en/translation.json` (form.mode keys)
- `frontend/src/components/hecate/__tests__/ConnectionTypeSelector.test.tsx` (new)

**Dependencies**: Commit 2 (`OrthrusAgent.resolved_address` must be in the type definition for the warning logic)
**Validation gate**:
```bash
cd /projects/Charon/frontend && node node_modules/.bin/vitest run --reporter=verbose
```
**Risk**: Medium — breaks existing 2-radio pattern; existing tests for RemoteServerForm must be updated

---

## 8. Edge Cases and Error Handling

| Scenario | Handling |
|---|---|
| Agent selected but later deleted | `agents.find()` returns undefined → show "Agent not found" in picker; do not submit |
| Agent has no `resolved_address` | Show amber warning with link; allow save; host will be empty string (backend returns 400) |
| Cloudflare tunnel selected as provider | `ProviderDevicePicker` renders a freeform hostname text input (`cf-hostname`) in place of the device picker. `onDeviceSelect('', enteredHost)` is called on `onChange`. `resolved_address` stores the typed public hostname (e.g. `app.example.com`). Same pattern in `AssignProviderDialog`: Cloudflare block writes directly to `resolvedAddress` state and clears `deviceId`; the generic resolved address field is hidden for Cloudflare. |
| ZeroTier network has no members | ZeroTierMemberPicker renders empty state message |
| `PATCH /orthrus/agents/:uuid` with empty body `{}` | No DB write; returns current agent state |
| Dangling `hecate_tunnel_uuid` (tunnel deleted after assignment) | UI shows tunnel UUID with fallback "Tunnel not found" label; still submittable |
| `name` field sent as empty string | Service rejects with "name cannot be blank" → 500 → form shows error |
| Multiple tunnels of same provider type | `<optgroup>` shows all; user selects the correct one |

---

## 9. Non-Goals

- Automatic `resolved_address` refresh on agent heartbeat (future work)
- Cloudflare sub-device/hostname picker (Cloudflare tunnels route by hostname, not peer IP)
- Removing deprecated `connection_type` enum values from the database
- Changes to Caddy config generation (existing `hecate_tunnel_uuid` handling stays as-is)
- Support for assigning multiple providers to a single agent
