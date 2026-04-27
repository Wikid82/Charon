# Hecate: Tunnel & Pathway Manager — Full Specification

**Feature Branch:** `feature/hecate`
**GitHub Issue:** #368
**Primary PR:** #983
**Status:** PRs 1–5 Complete. PR 6 (Frontend) and PR 7 (E2E Tests) remain.
**Last Updated:** 2026-04-27

---

## 1. Introduction

### 1.1 Overview

**Hecate** is the internal subsystem within Charon responsible for managing third-party tunneling services and reverse-proxy agents. Named after the Greek goddess of pathways, it allows Charon to route traffic to remote servers—through encrypted tunnels—without requiring open inbound ports on the target host.

**Orthrus** is the companion lightweight Go agent that runs on remote servers. It connects outbound to Charon using a yamux-multiplexed WebSocket ("The Leash"), securely proxying the Docker socket and TCP ports back to Charon with an HTTP allowlist filter ("The Muzzle").

### 1.2 Goals

- Allow users to add remote servers that are behind NAT/firewalls (Orthrus Agent connection type).
- Allow users to configure managed external tunnel providers (Cloudflare, Tailscale, ZeroTier, NetBird).
- Expose real-time tunnel log streaming via WebSocket.
- Provide a polished install wizard for deploying Orthrus agents.
- Integrate tunnel status indicators into the Remote Servers page.

### 1.3 Design Principles

- Hecate has **no separate page**. All UI is embedded in the existing **Remote Servers** page and form.
- Credentials (`EncryptedCredentials`, `AuthKeyHash`) are **never** sent to the frontend.
- Auth keys are shown **exactly once** at provision time and never again.

---

## 2. Completed Work (PRs 1–5)

### PR 1 — Backend Foundation: Models & Migration

| File | Description |
|------|-------------|
| `backend/internal/models/tunnel_config.go` | `TunnelConfig` GORM model with AES-256-GCM encrypted credentials |
| `backend/internal/models/orthrus_agent.go` | `OrthrusAgent` GORM model with bcrypt-hashed auth key |
| `backend/internal/models/remote_server.go` | Extended `RemoteServer` with `connection_type` and `orthrus_agent_uuid` fields |
| DB migration | Auto-migrate creates `tunnel_configs` and `orthrus_agents` tables |

**Key types:**

```go
// TunnelProviderType values
const (
    ProviderCloudflare TunnelProviderType = "cloudflare"
    ProviderTailscale  TunnelProviderType = "tailscale"
    ProviderZeroTier   TunnelProviderType = "zerotier"
    ProviderNetBird    TunnelProviderType = "netbird"
)

// ConnectionType values on RemoteServer
const (
    ConnectionTypeDirect     ConnectionType = "direct"
    ConnectionTypeOrthrus    ConnectionType = "orthrus"
    ConnectionTypeCloudflare ConnectionType = "cloudflare"
)
```

### PR 2 — Backend Core: `internal/hecate` + `internal/orthrus`

- `internal/hecate`: `TunnelManager`, `TunnelProvider` interface, `RingBuffer` (1000-line circular log buffer), exponential backoff restart policy.
- `internal/orthrus`: `OrthrusServer` (manages incoming agent WebSocket sessions), internal CA, `GetProxyAddr()` for Caddy upstream injection.
- `internal/orthrus/muzzle`: HTTP allowlist filter for Docker socket proxying.

### PR 3 — Provider Implementations

| Provider | Package | Key Type |
|----------|---------|----------|
| Cloudflare | `internal/hecate/providers/cloudflare` | `CloudflareTunnelProvider`, `CloudflareClient` |
| Tailscale | `internal/hecate/providers/tailscale` | `TailscaleProvider`, `TailscaleClient` |
| ZeroTier | `internal/hecate/providers/zerotier` | `ZeroTierProvider`, `ZeroTierClient` |
| NetBird | `internal/hecate/providers/netbird` | `NetBirdProvider`, `NetBirdClient` |

### PR 4 — API Handlers & Routes

All endpoints wired and tested (85.3% coverage):

**Hecate REST** (management group — requires auth + `RequireManagementAccess`):

| Method | Path | Handler |
|--------|------|---------|
| GET | `/hecate/status` | `GetStatus` |
| GET | `/hecate/tunnels` | `List` |
| POST | `/hecate/tunnels` | `Create` |
| GET | `/hecate/tunnels/:uuid` | `Get` |
| PUT | `/hecate/tunnels/:uuid` | `Update` |
| DELETE | `/hecate/tunnels/:uuid` | `Delete` |
| POST | `/hecate/tunnels/:uuid/start` | `Start` |
| POST | `/hecate/tunnels/:uuid/stop` | `Stop` |
| POST | `/hecate/tunnels/:uuid/rotate-credentials` | `RotateCredentials` |
| GET | `/hecate/cloudflare/tunnels` | `ListCloudflareTunnels` |
| GET | `/hecate/tunnels/:uuid/config/cloudflared` | `GetCloudflaredConfig` |
| GET | `/hecate/tailscale/devices` | `ListTailscaleDevices` |
| POST | `/hecate/tailscale/sync` | `SyncTailscale` |
| GET | `/hecate/zerotier/networks` | `ListZeroTierNetworks` |
| GET | `/hecate/zerotier/networks/:network_id/members` | `ListZeroTierMembers` |
| GET | `/hecate/netbird/peers` | `ListNetBirdPeers` |
| POST | `/hecate/netbird/sync` | `SyncNetBird` |
| GET (WS) | `/ws/hecate/logs/:uuid` | `StreamLogs` |

**Orthrus REST** (management group):

| Method | Path | Handler |
|--------|------|---------|
| GET | `/orthrus/agents` | `List` |
| POST | `/orthrus/agents` | `Provision` → returns `{ agent, auth_key }` |
| GET | `/orthrus/agents/:uuid` | `Get` |
| DELETE | `/orthrus/agents/:uuid` | `Delete` |
| POST | `/orthrus/agents/:uuid/revoke` | `Revoke` |
| GET | `/orthrus/agents/:uuid/snippets` | `GetInstallSnippets` |
| GET (WS) | `/ws/orthrus/connect` | Agent WebSocket (`api` group, bearer token auth) |

**Known issue (Supervisor non-blocking):** `hecate_handler.go` returns HTTP 500 for not-found instead of 404. Fix in Commit 18.

### PR 5 — Orthrus Agent Binary

- `agent/` — standalone Go module (`github.com/Wikid82/charon/agent`)
- yamux over WebSocket reverse tunnel to Charon
- Docker image: `ghcr.io/wikid82/charon-orthrus-agent` (~2.4 MB, from scratch)
- Configuration via environment variables: `ORTHRUS_NAME`, `CHARON_LINK`, `AUTH_KEY`, `ORTHRUS_MODE`
- mTLS enrollment: on first connect, generates keypair, sends CSR, receives signed cert from Charon CA

---

## 3. PR 6 — Frontend Implementation

### 3.1 File Map

```
frontend/src/
├── api/
│   ├── hecate.ts              # NEW — Hecate REST API client
│   └── orthrus.ts             # NEW — Orthrus REST API client
├── hooks/
│   ├── useHecate.ts           # NEW — TanStack Query hooks for Hecate
│   └── useOrthrus.ts          # NEW — TanStack Query hooks for Orthrus
├── components/
│   └── hecate/                # NEW directory
│       ├── TunnelStatusBadge.tsx
│       ├── TunnelLogViewer.tsx
│       ├── OrthrusInstallWizard.tsx
│       ├── ConnectionTypeSelector.tsx
│       ├── CloudflareTunnelWizard.tsx
│       └── TailscaleDevicePicker.tsx
├── components/
│   └── RemoteServerForm.tsx   # MODIFIED — add connection type field
└── pages/
    └── RemoteServers.tsx      # MODIFIED — integrate TunnelStatusBadge
```

### 3.2 TypeScript Interfaces

#### `frontend/src/api/hecate.ts` — Types

```typescript
export type TunnelProvider = 'cloudflare' | 'tailscale' | 'zerotier' | 'netbird';
export type TunnelState = 'connected' | 'connecting' | 'error' | 'stopped';

export interface TunnelConfig {
  uuid: string;
  name: string;
  provider: TunnelProvider;
  configuration: string;   // provider-specific JSON blob
  is_active: boolean;
  created_at: string;
  updated_at: string;
  // NOTE: credentials are NEVER returned; only sent on create/update
}

export interface TunnelStatus {
  uuid: string;
  name: string;
  provider: TunnelProvider;
  state: TunnelState;
  uptime_seconds: number;
  last_error: string;
}

export interface CreateTunnelRequest {
  name: string;
  provider: TunnelProvider;
  credentials: string;
  configuration?: string;
  is_active?: boolean;
}

export interface UpdateTunnelRequest {
  name: string;
  provider: TunnelProvider;
  credentials?: string;      // omit to keep existing credentials
  configuration?: string;
  is_active?: boolean;
}

export interface CloudflareTunnel {
  id: string;
  name: string;
  status: string;
  created_at: string;
}

export interface TailscaleDevice {
  id: string;
  hostname: string;
  addresses: string[];
  os: string;
  last_seen: string;
  online: boolean;
}

export interface ZeroTierNetwork {
  id: string;
  name: string;
  description: string;
  private: boolean;
  total_member_count: number;
}

export interface ZeroTierMember {
  node_id: string;
  name: string;
  description: string;
  ip_assignments: string[];
  authorized: boolean;
  online: boolean;
}

export interface NetBirdPeer {
  id: string;
  name: string;
  ip: string;
  os: string;
  connection_state: string;
  last_seen: string;
  online: boolean;
}
```

#### `frontend/src/api/hecate.ts` — Functions

```typescript
import client from './client';

// Tunnel CRUD
export const getTunnelStatus = (): Promise<TunnelStatus[]>
export const listTunnels = (): Promise<TunnelConfig[]>
export const createTunnel = (req: CreateTunnelRequest): Promise<TunnelConfig>
export const getTunnel = (uuid: string): Promise<TunnelConfig>
export const updateTunnel = (uuid: string, req: UpdateTunnelRequest): Promise<{ message: string }>
export const deleteTunnel = (uuid: string): Promise<void>
export const startTunnel = (uuid: string): Promise<{ message: string }>
export const stopTunnel = (uuid: string): Promise<{ message: string }>
export const rotateCredentials = (uuid: string, credentials: string): Promise<{ message: string }>

// Cloudflare
export const listCloudflareTunnels = (): Promise<CloudflareTunnel[]>
export const getCloudflaredConfig = (uuid: string): Promise<string>  // returns YAML text

// Tailscale
export const listTailscaleDevices = (): Promise<TailscaleDevice[]>
export const syncTailscale = (): Promise<TailscaleDevice[]>

// ZeroTier
export const listZeroTierNetworks = (): Promise<ZeroTierNetwork[]>
export const listZeroTierMembers = (networkId: string): Promise<ZeroTierMember[]>

// NetBird
export const listNetBirdPeers = (): Promise<NetBirdPeer[]>
export const syncNetBird = (): Promise<NetBirdPeer[]>

// WebSocket — returns WebSocket instance; caller manages lifecycle
export const connectTunnelLogs = (uuid: string, onMessage: (line: string) => void): WebSocket
```

`connectTunnelLogs` mirrors the `connectLiveLogs` pattern in `frontend/src/api/logs.ts`:
construct a `wss://` or `ws://` URL from `window.location`, open the WebSocket, call `onMessage` for each `message` event, return the WebSocket instance for caller cleanup.

#### `frontend/src/api/orthrus.ts` — Types

```typescript
export type OrthrusStatus = 'online' | 'offline' | 'pending';

export interface OrthrusAgent {
  uuid: string;
  name: string;
  status: OrthrusStatus;
  capabilities: string;         // JSON array string e.g. '["docker","tcp:5432"]'
  agent_cert_pem?: string;
  last_heartbeat?: string;
  last_seen?: string;
  created_at: string;
  updated_at: string;
  // auth_key_hash is NEVER returned
}

export interface ProvisionAgentRequest {
  name: string;
}

export interface ProvisionAgentResponse {
  agent: OrthrusAgent;
  auth_key: string;             // Shown ONCE — user must copy immediately
}

export interface InstallSnippets {
  docker_compose: string;
  systemd: string;
  tarball: string;
  homebrew: string;
  kubernetes_daemon_set: string;
}
```

#### `frontend/src/api/orthrus.ts` — Functions

```typescript
import client from './client';

export const listAgents = (): Promise<OrthrusAgent[]>
export const provisionAgent = (req: ProvisionAgentRequest): Promise<ProvisionAgentResponse>
export const getAgent = (uuid: string): Promise<OrthrusAgent>
export const deleteAgent = (uuid: string): Promise<void>
export const revokeAgent = (uuid: string): Promise<{ message: string }>
export const getInstallSnippets = (uuid: string): Promise<InstallSnippets>
```

`getInstallSnippets` must pass `X-Charon-URL: window.location.origin` header so the backend resolves the correct public URL regardless of TLS termination.

### 3.3 Hook Specifications

#### `frontend/src/hooks/useHecate.ts`

Follows the `useRemoteServers.ts` pattern — TanStack Query v5 with `useQuery` + `useMutation` + `useQueryClient`.

```typescript
export const TUNNELS_QUERY_KEY = ['hecate', 'tunnels'];
export const STATUS_QUERY_KEY = ['hecate', 'status'];

export function useHecate() {
  // tunnels: TunnelConfig[]
  // status: TunnelStatus[]
  // loading: boolean
  // error: string | null
  // createTunnel(req: CreateTunnelRequest): Promise<TunnelConfig>
  // updateTunnel(uuid, req): Promise<void>
  // deleteTunnel(uuid): Promise<void>
  // startTunnel(uuid): Promise<void>
  // stopTunnel(uuid): Promise<void>
  // rotateCredentials(uuid, credentials): Promise<void>
  // isCreating, isUpdating, isDeleting: boolean
}
```

`status` polling: `refetchInterval: 10_000` (10 s) while any tunnel `state === 'connecting'`.

#### `frontend/src/hooks/useOrthrus.ts`

```typescript
export const AGENTS_QUERY_KEY = ['orthrus', 'agents'];

export function useOrthrus() {
  // agents: OrthrusAgent[]
  // loading: boolean
  // error: string | null
  // provisionAgent(name: string): Promise<ProvisionAgentResponse>
  // deleteAgent(uuid): Promise<void>
  // revokeAgent(uuid): Promise<void>
  // getSnippets(uuid): Promise<InstallSnippets>
  // isProvisioning, isDeleting, isRevoking: boolean
}
```

### 3.4 Component Specifications

---

#### `TunnelStatusBadge.tsx`

**File:** `frontend/src/components/hecate/TunnelStatusBadge.tsx`

**Props:**

```typescript
interface TunnelStatusBadgeProps {
  state: TunnelState;
  showLabel?: boolean;   // default: true
  className?: string;
}
```

**State → Visual Mapping:**

| `state` | Icon (lucide) | Badge classes | Label |
|---------|--------------|---------------|-------|
| `connected` | `CheckCircle2` | `bg-green-900/30 text-green-400 border-green-700` | "Connected" |
| `connecting` | `Loader2` (animate-spin) | `bg-yellow-900/30 text-yellow-300 border-yellow-600` | "Starting" |
| `error` | `AlertCircle` | `bg-red-900/30 text-red-400 border-red-700` | "Error" |
| `stopped` | `Circle` | `bg-gray-800 text-gray-400 border-gray-700` | "Stopped" |

**Accessibility:**
- Must not use color alone — icon + label always present.
- Wrapper: `role="status"` and `aria-label="Tunnel status: {state}"`.
- Use `aria-live="polite"` when embedded in a live-updating context.
- Wrap the existing `Badge` UI component rather than inventing a new styled element.

**WCAG contrast:** All color combinations must meet ≥ 4.5:1 ratio on the application's dark background. Verify with a contrast checker before finalizing.

---

#### `ConnectionTypeSelector.tsx`

**File:** `frontend/src/components/hecate/ConnectionTypeSelector.tsx`

**Props:**

```typescript
type ConnectionType = 'direct' | 'orthrus' | 'cloudflare';

interface ConnectionTypeSelectorProps {
  value: ConnectionType;
  onChange: (type: ConnectionType) => void;
  disabled?: boolean;
  id?: string;
  'aria-label'?: string;
}
```

**Options:**

| Value | Label | Description |
|-------|-------|-------------|
| `direct` | Direct / Manual | Server reachable via host + port |
| `orthrus` | Orthrus Agent | Server behind NAT; agent connects outbound |
| `cloudflare` | Cloudflare Tunnel | Expose via Cloudflare edge network |

Render as `<NativeSelect>` (existing `frontend/src/components/ui/NativeSelect.tsx`) for keyboard/screen-reader compatibility.

---

#### `OrthrusInstallWizard.tsx`

**File:** `frontend/src/components/hecate/OrthrusInstallWizard.tsx`

**Props:**

```typescript
interface OrthrusInstallWizardProps {
  agent: OrthrusAgent;
  authKey: string;               // one-time plaintext key — displayed once
  snippets: InstallSnippets | null;
  loadingSnippets: boolean;
  onClose: () => void;
}
```

**Tabs** (use `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` from `frontend/src/components/ui/Tabs.tsx`):

| Tab | Content source |
|-----|---------------|
| Docker Compose | `snippets.docker_compose` |
| systemd | `snippets.systemd` |
| Binary | `snippets.tarball` |
| Homebrew | `snippets.homebrew` |
| Kubernetes | `snippets.kubernetes_daemon_set` |

**AUTH_KEY display requirements:**
- Monospace `<input readOnly>` with `aria-label="Authentication key"`.
- `aria-describedby` points to the warning message element.
- Warning banner: "This key will not be shown again. Copy it before closing."
- "Copy Key" button → `navigator.clipboard.writeText(authKey)` on user gesture.
- After copy: show "Copied!" for 3 s, then revert.

**Snippet requirements:**
- `<pre>` block with "Copy" button (top-right per tab).
- All snippets use `<AUTH_KEY>` placeholder; show info notice: "Replace `<AUTH_KEY>` with the key shown above."

**Troubleshooting:** `<details>` collapsed by default; reveals:
```
journalctl -u orthrus -f
systemctl status orthrus
docker logs orthrus-agent -f
```

**Accessibility:**
- Wraps in `Dialog` / `DialogContent` from `frontend/src/components/ui/Dialog.tsx`.
- Dialog: `role="dialog"`, `aria-modal="true"`, `aria-labelledby`.
- Tabs: roving tabindex, `ArrowLeft`/`ArrowRight` between tabs per APG tabs pattern.

---

#### `CloudflareTunnelWizard.tsx`

**File:** `frontend/src/components/hecate/CloudflareTunnelWizard.tsx`

**Props:**

```typescript
interface CloudflareTunnelWizardProps {
  onSuccess: (tunnel: TunnelConfig) => void;
  onCancel: () => void;
}
```

**Step 1 — Token Input:**
- `<input type="password">` with show/hide toggle.
- Label: "Cloudflare Tunnel Token"
- Help text: "Find your token in the Cloudflare Zero Trust dashboard under Tunnels." (associated via `aria-describedby`).

**Step 2 — Confirmation:**
- Call `POST /hecate/tunnels` with `provider: 'cloudflare'`.
- Show resolved tunnel name on success.

**Step 3 — Live Status:**
- Poll `GET /hecate/status` every 3 s via `refetchInterval`.
- Show `TunnelStatusBadge` for the new tunnel.
- "Done" button enabled when `state === 'connected'`.

---

#### `TailscaleDevicePicker.tsx`

**File:** `frontend/src/components/hecate/TailscaleDevicePicker.tsx`

**Props:**

```typescript
interface TailscaleDevicePickerProps {
  value: string;
  onChange: (deviceId: string, device: TailscaleDevice) => void;
  disabled?: boolean;
  id?: string;
}
```

**Behavior:**
- Fetch `GET /hecate/tailscale/devices` via `useQuery`.
- Render as `<NativeSelect>` with `${device.hostname} (${device.addresses[0]})`.
- Offline devices: `disabled` option attribute + "(offline)" suffix.
- "Refresh" icon button → `POST /hecate/tailscale/sync` then invalidate query.

---

#### `TunnelLogViewer.tsx`

**File:** `frontend/src/components/hecate/TunnelLogViewer.tsx`

**Props:**

```typescript
interface TunnelLogViewerProps {
  tunnelUuid: string;
  maxLines?: number;             // default: 500
  className?: string;
}
```

**Behavior:**
- Open WebSocket via `connectTunnelLogs(tunnelUuid, ...)` on mount.
- Buffer up to `maxLines` lines; drop oldest at capacity.
- Scrollable `<div>` with `role="log"`, `aria-live="polite"`, `aria-label="Tunnel log output"`.
- Auto-scroll to bottom unless user has scrolled up.
- Pause/Resume toggle: pauses DOM updates; buffer continues filling. `aria-pressed` state on button.
- Clear button: clears display buffer. `aria-label="Clear log output"`.
- Line coloring:
  - `error` / `ERR` in line → `text-red-400`
  - `warn` / `WARN` in line → `text-yellow-400`
  - otherwise → `text-gray-300`
- On WS `close`/`error`: show "Reconnecting…" badge; reconnect after 3 s backoff.
- `useEffect` cleanup: close WebSocket and remove event listeners on unmount.

---

### 3.5 `RemoteServerForm.tsx` Modifications

**File:** `frontend/src/components/RemoteServerForm.tsx`

**Changes:**

1. Add `connection_type: 'direct'` and `orthrus_agent_uuid: undefined` to `formData` state.
2. Render `<ConnectionTypeSelector>` between `provider` and `host` fields.
3. Conditional field rendering:
   - `direct` → show `host` + `port` (unchanged).
   - `orthrus` → hide `host`/`port`; show `<NativeSelect>` populated from `useOrthrus().agents` for agent selection + "Provision New Agent" button opening `OrthrusInstallWizard`.
   - `cloudflare` → hide `host`/`port`; show `<CloudflareTunnelWizard>` inline or in a dialog.
4. Include `connection_type` and `orthrus_agent_uuid` in `handleSubmit` payload.

**Updated `RemoteServer` interface in `frontend/src/api/remoteServers.ts`:**

```typescript
export interface RemoteServer {
  uuid: string;
  name: string;
  provider: string;
  host: string;
  port: number;
  username?: string;
  enabled: boolean;
  reachable: boolean;
  last_check?: string;
  created_at: string;
  updated_at: string;
  // Hecate fields
  connection_type?: 'direct' | 'orthrus' | 'cloudflare';
  orthrus_agent_uuid?: string;
}
```

---

### 3.6 `RemoteServers.tsx` Page Modifications

**File:** `frontend/src/pages/RemoteServers.tsx`

**Changes:**

1. Import `TunnelStatusBadge` from `../components/hecate/TunnelStatusBadge`.
2. Import `useHecate` from `../hooks/useHecate`.
3. Call `const { status } = useHecate()` at component level.
4. Helper `getTunnelState(server: RemoteServer): TunnelState | undefined` — look up server in `status` by UUID.
5. Add "Connection" column to the table:

```typescript
{
  key: 'connection_type',
  header: t('remoteServers.columnConnection'),
  cell: (server) => {
    if (!server.connection_type || server.connection_type === 'direct') {
      return <Badge variant="outline" size="sm">Direct</Badge>;
    }
    const state = getTunnelState(server);
    return state
      ? <TunnelStatusBadge state={state} />
      : <Badge variant="outline" size="sm">{server.connection_type}</Badge>;
  },
}
```

6. In card/grid view: include `TunnelStatusBadge` for non-direct servers.
7. "View Logs" icon button on each non-direct row — opens dialog containing `<TunnelLogViewer tunnelUuid={server.uuid} />`.

---

### 3.7 i18n Keys

Add to all locale files under `frontend/src/locales/*/translation.json`:

```json
{
  "hecate": {
    "tunnels": "Tunnels",
    "tunnel": "Tunnel",
    "status": {
      "connected": "Connected",
      "connecting": "Starting",
      "error": "Error",
      "stopped": "Stopped"
    },
    "provider": {
      "cloudflare": "Cloudflare",
      "tailscale": "Tailscale",
      "zerotier": "ZeroTier",
      "netbird": "NetBird"
    },
    "connectionType": {
      "direct": "Direct",
      "orthrus": "Orthrus Agent",
      "cloudflare": "Cloudflare Tunnel"
    },
    "logViewer": {
      "title": "Tunnel Logs",
      "pause": "Pause",
      "resume": "Resume",
      "clear": "Clear",
      "reconnecting": "Reconnecting…",
      "ariaLabel": "Tunnel log output"
    },
    "installWizard": {
      "title": "Install Orthrus Agent",
      "authKeyLabel": "Authentication key",
      "authKeyWarning": "This key will not be shown again. Copy it before closing.",
      "copyKey": "Copy Key",
      "copied": "Copied!",
      "snippetPlaceholder": "Replace <AUTH_KEY> with the key shown above.",
      "tabs": {
        "dockerCompose": "Docker Compose",
        "systemd": "systemd",
        "tarball": "Binary",
        "homebrew": "Homebrew",
        "kubernetes": "Kubernetes"
      },
      "troubleshootingTitle": "Troubleshooting",
      "done": "Done",
      "close": "Close"
    }
  },
  "remoteServers": {
    "columnConnection": "Connection"
  }
}
```

---

## 4. Backend Polish (Commit 18)

### 4.1 Fix Not-Found Response in `hecate_handler.go`

**File:** `backend/internal/api/handlers/hecate_handler.go`

Apply to `Get`, `Delete`, `Update`, `Start`, `Stop`, `RotateCredentials`:

```go
import (
    "errors"
    "gorm.io/gorm"
)

if err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
        return
    }
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

### 4.2 Add TLS Proxy Comment to `orthrus_handler.go`

**File:** `backend/internal/api/handlers/orthrus_handler.go`

Add above the scheme-detection block in `GetInstallSnippets`:

```go
// TLS detection via c.Request.TLS is unreliable when Charon runs behind a
// reverse proxy (e.g., Caddy) that terminates TLS. The X-Charon-URL header
// allows callers to pass the correct public URL explicitly; if absent, we
// fall back to heuristic detection. Users deploying behind a proxy should
// set the X-Charon-URL header from the frontend (window.location.origin).
```

---

## 5. PR 7 — E2E Playwright Tests

### 5.1 Test Files

| File | Coverage |
|------|---------|
| `tests/hecate-tunnel-manager.spec.ts` | Tunnel CRUD, status display, log viewer |
| `tests/orthrus-agent-install.spec.ts` | Install wizard tabs, AUTH_KEY copy, snippets |

Both files:
- Import `test`, `expect` from `./fixtures/test`
- Call `await waitForAPIHealth(request)` in `beforeEach`
- Use `getByRole` locators
- Group interactions with `test.step()`

### 5.2 `tests/hecate-tunnel-manager.spec.ts`

```typescript
test.describe('Hecate Tunnel Manager', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Remote Servers page — connection column', () => {
    test('shows Direct badge for connection_type=direct', async ({ page }) => { ... });
    test('shows TunnelStatusBadge for connection_type=orthrus', async ({ page }) => { ... });
    test('tunnel status badge uses text + icon, not color alone', async ({ page }) => { ... });
  });

  test.describe('Add Server — Connection Type selection', () => {
    test('shows host/port fields when Direct is selected', async ({ page }) => { ... });
    test('shows agent selector and Provision button when Orthrus is selected', async ({ page }) => { ... });
    test('hides host/port when Orthrus is selected', async ({ page }) => { ... });
    test('shows CloudflareTunnelWizard when Cloudflare is selected', async ({ page }) => { ... });
  });

  test.describe('Tunnel CRUD', () => {
    test('creates a Cloudflare tunnel and shows it in status list', async ({ page }) => { ... });
    test('deletes a tunnel and removes it from status list', async ({ page }) => { ... });
    test('start/stop changes badge state', async ({ page }) => { ... });
  });

  test.describe('TunnelLogViewer', () => {
    test('opens log viewer from View Logs button', async ({ page }) => { ... });
    test('log viewer has role=log and aria-live=polite', async ({ page }) => { ... });
    test('pause/resume button has aria-pressed state', async ({ page }) => { ... });
    test('clear button clears displayed log lines', async ({ page }) => { ... });
  });
});
```

**Detailed scenario — "TunnelStatusBadge for orthrus servers":**

1. Navigate to `/remote-servers`.
2. Create/mock a server with `connection_type: 'orthrus'` via API.
3. Expect ARIA snapshot match on the Connection cell:
   ```
   - status "Tunnel status: offline"
   ```
4. Verify badge has both icon (`svg`) and visible text content.

**Detailed scenario — "badge uses text + icon not color alone":**

1. For every visible `TunnelStatusBadge`:
   - `expect(badge).not.toBeEmpty()` — has text content.
   - `expect(badge).toHaveAccessibleName()` — has accessible name.

### 5.3 `tests/orthrus-agent-install.spec.ts`

```typescript
test.describe('Orthrus Agent Install Wizard', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Wizard tabs', () => {
    test('wizard shows all five install tabs', async ({ page }) => {
      // Open Add Server, select Orthrus, click Provision New Agent
      // Expected ARIA snapshot:
      // - dialog "Install Orthrus Agent":
      //   - tablist:
      //     - tab "Docker Compose"
      //     - tab "systemd"
      //     - tab "Binary"
      //     - tab "Homebrew"
      //     - tab "Kubernetes"
    });
    test('switching tabs shows different content', async ({ page }) => { ... });
    test('tabs are keyboard navigable with arrow keys', async ({ page }) => {
      // Focus tablist, press ArrowRight, expect next tab focused
      // Press Enter to activate, expect panel content to change
    });
  });

  test.describe('AUTH_KEY display', () => {
    test('displays AUTH_KEY in a read-only input', async ({ page }) => { ... });
    test('AUTH_KEY has aria-label="Authentication key"', async ({ page }) => { ... });
    test('one-time warning message is visible', async ({ page }) => { ... });
    test('copy button copies key to clipboard', async ({ page, context }) => {
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);
      // Click copy, read clipboard, assert matches displayed key
    });
    test('copy button shows "Copied!" feedback for 3s', async ({ page }) => { ... });
  });

  test.describe('Snippet validation', () => {
    test('Docker Compose snippet contains ORTHRUS_NAME placeholder', async ({ page }) => { ... });
    test('systemd snippet contains AUTH_KEY placeholder', async ({ page }) => { ... });
    test('<AUTH_KEY> placeholder info notice is visible', async ({ page }) => { ... });
    test('each tab has a functional copy button', async ({ page, context }) => { ... });
  });

  test.describe('Troubleshooting panel', () => {
    test('collapsed by default', async ({ page }) => {
      await expect(page.getByRole('group', { name: /troubleshooting/i })).toHaveAttribute('open', { visible: false });
    });
    test('expanding shows journalctl command', async ({ page }) => { ... });
  });

  test.describe('Accessibility', () => {
    test('dialog has role=dialog and aria-modal=true', async ({ page }) => { ... });
    test('dialog has accessible label', async ({ page }) => { ... });
    test('first interactive element receives focus on open', async ({ page }) => { ... });
    test('focus returns to trigger on close', async ({ page }) => { ... });
    test('Escape key closes the wizard', async ({ page }) => { ... });
  });
});
```

---

## 6. Commit Slicing Strategy

### Decision

All remaining work ships in **PR #983** via 8 ordered logical commits.

### Commit Table

| # | Scope | Commit Message | Files | Validation Gate |
|---|-------|---------------|-------|----------------|
| 18 | Backend polish | `fix(hecate): return 404 for not-found tunnels and document TLS proxy caveat` | `hecate_handler.go`, `orthrus_handler.go` | `go test ./internal/api/handlers/...` green |
| 19 | API clients + hooks | `feat(frontend): add Hecate and Orthrus API clients and TanStack Query hooks` | `api/hecate.ts`, `api/orthrus.ts`, `hooks/useHecate.ts`, `hooks/useOrthrus.ts` | `npm run build` TypeScript zero errors; vitest passes |
| 20 | Status components | `feat(hecate): add TunnelStatusBadge and ConnectionTypeSelector` | `components/hecate/TunnelStatusBadge.tsx`, `components/hecate/ConnectionTypeSelector.tsx`, i18n keys | Vitest; WCAG contrast verified |
| 21 | Install wizard | `feat(hecate): add OrthrusInstallWizard, CloudflareTunnelWizard, TailscaleDevicePicker` | `components/hecate/OrthrusInstallWizard.tsx`, `components/hecate/CloudflareTunnelWizard.tsx`, `components/hecate/TailscaleDevicePicker.tsx`, i18n keys | Vitest; wizard renders all 5 tabs |
| 22 | Form + page integration | `feat(hecate): integrate connection type into RemoteServerForm and RemoteServers page` | `components/RemoteServerForm.tsx`, `pages/RemoteServers.tsx`, `api/remoteServers.ts`, i18n keys | Vitest; form fields conditional on connection type; Connection column visible |
| 23 | Log viewer | `feat(hecate): add TunnelLogViewer real-time WebSocket streaming component` | `components/hecate/TunnelLogViewer.tsx`, i18n keys | Vitest (mock WebSocket); `role="log"` present; pause/resume works |
| 24 | E2E — tunnel manager | `test(e2e): add Playwright tests for hecate-tunnel-manager` | `tests/hecate-tunnel-manager.spec.ts` | `npx playwright test tests/hecate-tunnel-manager.spec.ts --project=firefox` green |
| 25 | E2E — install wizard | `test(e2e): add Playwright tests for orthrus-agent-install wizard` | `tests/orthrus-agent-install.spec.ts` | `npx playwright test tests/orthrus-agent-install.spec.ts --project=firefox` green |

### Dependency Order

```
Commit 18 (backend)
    └── Commit 19 (API clients + hooks)
            ├── Commit 20 (TunnelStatusBadge, ConnectionTypeSelector)
            │       └── Commit 21 (OrthrusInstallWizard, CloudflareTunnelWizard, TailscaleDevicePicker)
            │               └── Commit 22 (RemoteServerForm + RemoteServers integration)
            │                       └── Commit 23 (TunnelLogViewer)
            │                               ├── Commit 24 (E2E: tunnel manager)
            │                               └── Commit 25 (E2E: install wizard)
```

### Rollback Notes

- Commits 24–25 are test-only; reverting either does not affect functionality.
- Commit 22 is the integration point. If it needs rework, Commits 20, 21, 23 remain independently usable.
- Commit 18 is backend-only and can be cherry-picked to `main` independently if needed.

---

## 7. Acceptance Criteria (Definition of Done)

### Commit 18 — Backend Polish

- [ ] `GET /api/v1/hecate/tunnels/:uuid` returns `404 {"error":"tunnel not found"}` for missing UUID.
- [ ] `DELETE /api/v1/hecate/tunnels/:uuid` returns `404` for missing UUID.
- [ ] `Start`, `Stop`, `Update`, `RotateCredentials` return `404` for missing UUID.
- [ ] TLS proxy caveat comment present in `GetInstallSnippets`.
- [ ] `go test ./internal/api/handlers/...` passes at ≥ 85% coverage.

### Commit 19 — API Clients + Hooks

- [ ] All API functions use shared `client` (Axios) instance from `./client`.
- [ ] `connectTunnelLogs` constructs WebSocket URL from `window.location` (handles `wss://` and `ws://`).
- [ ] `getInstallSnippets` sends `X-Charon-URL: window.location.origin`.
- [ ] TypeScript compiles with zero errors (`npm run build`).
- [ ] Vitest coverage ≥ 85% for each new file.

### Commits 20–23 — Components

- [ ] `TunnelStatusBadge`: 4 states render icon + text. WCAG contrast ≥ 4.5:1. `role="status"` + `aria-label` present.
- [ ] `ConnectionTypeSelector`: Keyboard-navigable. WCAG compliant. Correct `aria-label`.
- [ ] `OrthrusInstallWizard`: All 5 tabs present and keyboard-navigable. AUTH_KEY read-only with `aria-label`. One-time warning visible. Copy button works. `<AUTH_KEY>` placeholder in all snippets. Dialog `role="dialog"` + `aria-modal="true"`.
- [ ] `TunnelLogViewer`: `role="log"`, `aria-live="polite"`. Pause/Resume has `aria-pressed`. WebSocket closes on unmount (no leak). Line coloring applied.
- [ ] `RemoteServerForm`: Shows agent selector for `orthrus`, hides host/port. Shows CloudflareWizard for `cloudflare`. Payload includes `connection_type` and `orthrus_agent_uuid`.
- [ ] `RemoteServers`: Connection column in table and card view. View Logs button for non-direct servers.
- [ ] Zero hardcoded English strings in JSX — all use `t('hecate.*')` keys.

### Commits 24–25 — E2E Tests

- [ ] `npx playwright test tests/hecate-tunnel-manager.spec.ts --project=firefox` passes.
- [ ] `npx playwright test tests/orthrus-agent-install.spec.ts --project=firefox` passes.
- [ ] ARIA snapshot assertions match rendered output.
- [ ] Keyboard navigation tests pass for tabs (ArrowLeft/ArrowRight, Enter).
- [ ] No `test.skip()` in any test.

---

## 8. Security Considerations

- `auth_key` is returned **exactly once** in `POST /orthrus/agents` response. No other endpoint returns it.
- `EncryptedCredentials` and `AuthKeyHash` are `json:"-"` in GORM models — they cannot appear in any API response.
- `connectTunnelLogs` reuses the session cookie path — no separate auth token needed for WS upgrade.
- Install snippet `<AUTH_KEY>` placeholder is an inert string; no real key is embedded in the snippet response.
- `navigator.clipboard.writeText` is only called on explicit user gesture (button click).

---

## 9. Research Findings & Architecture References

### API Client Pattern

All files in `frontend/src/api/` import the shared `client` from `./client` (Axios instance, `baseURL: '/api/v1'`, `withCredentials: true`, 30 s timeout). New `hecate.ts` and `orthrus.ts` must follow this pattern.

### WebSocket Pattern

`connectLiveLogs` in `frontend/src/api/logs.ts` is the template for `connectTunnelLogs`. Uses `new WebSocket(...)` directly (not Axios). Resolves URL from `window.location` to support both `wss://` and `ws://`.

### Hooks Pattern

`frontend/src/hooks/useRemoteServers.ts` is the template for `useHecate` and `useOrthrus`. TanStack Query v5 (`@tanstack/react-query`): `useQuery` + `useMutation` + `useQueryClient`. Mutations invalidate the relevant query key on success.

### Tabs Pattern

`frontend/src/components/ui/Tabs.tsx` wraps `@radix-ui/react-tabs`. Exports `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent`. Do not create custom tab implementations.

### Dialog Pattern

`frontend/src/components/ui/Dialog.tsx` wraps `@radix-ui/react-dialog`. `OrthrusInstallWizard` must render inside `Dialog`/`DialogContent` for correct ARIA semantics and focus management.

### Playwright Test Pattern

All spec files import from `./fixtures/test` (not directly from `@playwright/test`). Template: `dns-provider-crud.spec.ts`.
- `waitForAPIHealth(request)` in `beforeEach`
- `waitForDialog`, `waitForLoadingComplete`, `waitForAPIResponse` from `./utils/wait-helpers`
- `getToastLocator` from `./utils/ui-helpers`
- Prefer `page.getByRole(...)` locators
- Group with `test.step(...)`

---

*End of specification.*
