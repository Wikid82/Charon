# Hecate: Tunnel & Pathway Manager

## 1. Overview

**Hecate** is the internal module within Charon responsible for managing third-party tunneling services. It serves as the "Goddess of Pathways," allowing Charon to route traffic not just to local ports, but through encrypted tunnels to remote networks without exposing ports on the public internet.

## 2. Architecture

Hecate is not a separate binary; it is a **Go package** (`internal/hecate`) running within the main Charon daemon.

### 2.1 The Provider Interface

To support multiple services (Tailscale, Cloudflare, Netbird), Hecate uses a strict Interface pattern.

```go
type TunnelProvider interface {
    // Name returns the unique ID of the provider (e.g., "tailscale-01")
    Name() string

    // Status returns the current health (Connected, Connecting, Error)
    Status() TunnelState

    // Start initiates the tunnel daemon
    Start(ctx context.Context) error

    // Stop gracefully terminates the connection
    Stop() error

    // GetAddress returns the internal IP/DNS routed through the tunnel
    GetAddress() string
}
```

### 2.2 Supported Integrations (Phase 1)

#### Cloudflare Tunnels (cloudflared)

- **Mechanism**: Charon manages the `cloudflared` binary via `os/exec`.
- **Config**: User provides the Token via the UI.
- **Outcome**: Exposes Charon directly to the edge without opening port 80/443 on the router.

#### Tailscale / Headscale

- **Mechanism**: Uses `tsnet` (Tailscale's Go library) to embed the node directly into Charon, OR manages the `tailscaled` socket.
- **Outcome**: Charon becomes a node on the Mesh VPN.

## 3. Dashboard Implementation (Unified UI)

**Hecate does NOT have a separate "Tunnels" tab.**
Instead, it is fully integrated into the **Remote Servers** dashboard to provide a unified experience for managing connectivity.

### 3.1 "Add Server" Workflow

When a user clicks "Add Server" in the dashboard, they are presented with a **Connection Type** dropdown that determines how Charon reaches the target.

#### Connection Types

1. **Direct / Manual (Existing)**
    - **Use Case**: The server is on the same LAN or reachable via a static IP/DNS.
    - **Fields**: `Host`, `Port`, `TLS Toggle`.
    - **Backend**: Standard TCP dialer.

2. **Orthrus Agent (New)**
    - **Use Case**: The server is behind a NAT/Firewall and cannot accept inbound connections.
    - **Workflow**:
        - User selects "Orthrus Agent".
        - Charon generates a unique `AUTH_KEY`.
        - UI displays a `docker-compose.yml` snippet pre-filled with the key and `CHARON_LINK`.
        - User deploys the agent on the remote host.
        - Hecate waits for the incoming WebSocket connection.

3. **Cloudflare Tunnel (Future)**
    - **Use Case**: Exposing a service via Cloudflare's edge network.
    - **Fields**: `Tunnel Token`.
    - **Backend**: Hecate spawns/manages the `cloudflared` process.

### 3.2 Hecate's Role

Hecate acts as the invisible backend engine for these non-direct connection types. It manages the lifecycle of the tunnels and agents, while the UI simply shows the status (Online/Offline) of the "Server".

### 3.3 Install Options & UX Snippets

When a user selects `Orthrus Agent` or chooses a `Managed Tunnel` flow, the UI should offer multiple installation options so both containerized and non-containerized environments are supported.

Provide these install options as tabs/snippets in the `Add Server` flow:

- **Docker Compose**: A one-file snippet the user can copy/paste (already covered in `orthrus` docs).
- **Standalone Binary + systemd**: Download URL, SHA256, install+`systemd` unit snippet for Linux hosts.
- **Tarball + Installer**: For offline installs with checksum verification.
- **Deb / RPM**: `apt`/`yum` install commands (when packages are available).
- **Homebrew**: `brew tap` + `brew install` for macOS / Linuxbrew users.
- **Kubernetes DaemonSet**: YAML for fleet or cluster-based deployments.

UI Requirements:

- Show the generated `AUTH_KEY` prominently and a single-copy button.
- Provide checksum and GPG signature links for any downloadable artifact.
- Offer a small troubleshooting panel with commands like `journalctl -u orthrus -f` and `systemctl status orthrus`.
- Allow the user to copy a recommended sidecar snippet that runs a VPN client (e.g., Tailscale) next to Orthrus when desired.

## 4. API Endpoints

- `GET /api/hecate/status` - Returns health of all tunnels.
- `POST /api/hecate/configure` - Accepts auth tokens and provider types.
- `POST /api/hecate/logs` - Streams logs from the underlying tunnel binary (e.g., cloudflared logs) for debugging.

## 5. Security (Cerberus Integration)

Traffic entering through Hecate must still pass through Cerberus.

- Tunnels terminate **before** the middleware chain.
- Requests from a Cloudflare Tunnel are tagged `source:tunnel` and subjected to the same WAF rules as standard traffic.

## 6. Implementation Details

### 6.1 Process Supervision

Hecate will act as a process supervisor for external binaries like `cloudflared`.

- **Supervisor Pattern**: A `TunnelManager` struct will maintain a map of active `TunnelProvider` instances.
- **Lifecycle**:
  - On startup, `TunnelManager` loads enabled configs from the DB.
  - It launches the binary using `os/exec`.
  - It monitors the process state. If the process exits unexpectedly, it triggers a **Restart Policy** (Exponential Backoff: 5s, 10s, 30s, 1m).
- **Graceful Shutdown**: When Charon shuts down, Hecate must send `SIGTERM` to all child processes and wait (with timeout) for them to exit.

### 6.2 Secrets Management

API tokens and sensitive credentials must not be stored in plaintext.

- **Encryption**: Sensitive fields (like Cloudflare Tokens) will be encrypted at rest in the SQLite database using AES-GCM.
- **Key Management**: An encryption key will be generated on first run and stored in `data/keys/hecate.key` (secured with 600 permissions), or provided via `CHARON_SECRET_KEY` env var.

### 6.3 Logging & Observability

- **Capture**: The `TunnelProvider` implementation will attach to the `Stdout` and `Stderr` pipes of the child process.
- **Storage**:
  - **Hot Logs**: A circular buffer (Ring Buffer) in memory (last 1000 lines) for real-time dashboard viewing.
  - **Cold Logs**: Rotated log files stored in `data/logs/tunnels/<provider>.log`.
- **Streaming**: The frontend will consume logs via a WebSocket endpoint (`/api/ws/hecate/logs/:id`) or Server-Sent Events (SSE) to display real-time output.

### 6.4 Frontend Components

- **TunnelStatusBadge**: Visual indicator (Green=Connected, Yellow=Starting, Red=Error/Stopped).
- **LogViewer**: A terminal-like component (using `xterm.js` or a virtualized list) to display the log stream.
- **ConfigForm**: A dynamic form that renders fields based on the selected provider (e.g., "Token" for Cloudflare, "Auth Key" for Tailscale).

## 7. Database Schema

We will introduce a new GORM model `TunnelConfig` in `internal/models`.

```go
package models

import (
 "time"
 "github.com/google/uuid"
 "gorm.io/datatypes"
)

type TunnelProviderType string

const (
 ProviderCloudflare TunnelProviderType = "cloudflare"
 ProviderTailscale  TunnelProviderType = "tailscale"
)

type TunnelConfig struct {
 ID        uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
 Name      string             `gorm:"not null" json:"name"` // User-friendly name (e.g., "Home Lab Tunnel")
 Provider  TunnelProviderType `gorm:"not null" json:"provider"`

 // EncryptedCredentials stores the API token or Auth Key.
 // It is encrypted at rest and decrypted only when starting the process.
 EncryptedCredentials []byte `gorm:"not null" json:"-"`

 // Configuration stores provider-specific settings (JSON).
 // e.g., Cloudflare specific flags, region settings, etc.
 Configuration datatypes.JSON `json:"configuration"`

 IsActive  bool      `gorm:"default:false" json:"is_active"` // User's desired state
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
```
