# Security Services Implementation Plan

## Overview
This document outlines the plan to implement a modular Security Dashboard in CaddyProxyManager+ (CPM+). The goal is to provide optional, high-value security integrations (CrowdSec, WAF, ACLs, Rate Limiting) while keeping the core Docker image lightweight.

## Core Philosophy
1.  **Optionality**: All security services are disabled by default.
2.  **Environment Driven**: Activation is controlled via `CPM_SECURITY_*` environment variables.
3.  **Minimal Footprint**:
    *   Lightweight Caddy modules (WAF, Bouncers) are compiled into the binary (negligible size impact).
    *   Heavy standalone agents (e.g., CrowdSec Agent) are only installed at runtime if explicitly enabled in "Local" mode.
4.  **Unified Dashboard**: A single pane of glass in the UI to view status and configuration.

---

## 1. Environment Variables
We will introduce a new set of environment variables to control these services.

| Variable | Values | Description |
| :--- | :--- | :--- |
| `CPM_SECURITY_CROWDSEC_MODE` | `disabled` (default), `local`, `external` | `local` installs agent inside container; `external` uses remote agent. |
| `CPM_SECURITY_CROWDSEC_API_URL` | URL (e.g., `http://crowdsec:8080`) | Required if mode is `external`. |
| `CPM_SECURITY_CROWDSEC_API_KEY` | String | Required if mode is `external`. |
| `CPM_SECURITY_WAF_MODE` | `disabled` (default), `enabled` | Enables Coraza WAF with OWASP Core Rule Set (CRS). |
| `CPM_SECURITY_RATELIMIT_ENABLED` | `true`, `false` (default) | Enables global rate limiting controls. |
| `CPM_SECURITY_ACL_ENABLED` | `true`, `false` (default) | Enables IP-based Access Control Lists. |

---

## 2. Backend Implementation

### A. Dockerfile Updates
We need to compile the necessary Caddy modules into our binary. This adds minimal size overhead but enables the features natively.
*   **Action**: Update `Dockerfile` `caddy-builder` stage to include:
    *   `github.com/corazawaf/coraza-caddy/v2` (WAF)
    *   `github.com/hslatman/caddy-crowdsec-bouncer` (CrowdSec Bouncer)

### B. Configuration Management (`internal/config`)
*   **Action**: Update `Config` struct to parse `CPM_SECURITY_*` variables.
*   **Action**: Create `SecurityConfig` struct to hold these values.

### C. Runtime Installation (`docker-entrypoint.sh`)
To satisfy the "install locally" requirement for CrowdSec without bloating the image:
*   **Action**: Modify `docker-entrypoint.sh` to check `CPM_SECURITY_CROWDSEC_MODE`.
*   **Logic**: If `local`, execute `apk add --no-cache crowdsec` (and dependencies) before starting the app. This keeps the base image small for users who don't use it.

### D. API Endpoints (`internal/api`)
*   **New Endpoint**: `GET /api/v1/security/status`
    *   Returns the enabled/disabled state of each service.
    *   Returns basic metrics if available (e.g., "WAF: Active", "CrowdSec: Connected").

---

## 3. Frontend Implementation

### A. Navigation
*   **Action**: Add "Security" item to the Sidebar in `Layout.tsx`.

### B. Security Dashboard (`src/pages/Security.tsx`)
*   **Layout**: Grid of cards representing each service.
*   **Empty State**: If all services are disabled, show a clean "Security Not Enabled" state with a link to the GitHub Pages documentation on how to enable them.

### C. Service Cards
1.  **CrowdSec Card**:
    *   **Status**: Active (Local/External) / Disabled.
    *   **Content**: If Local, show basic stats (last push, alerts). If External, show connection status.
    *   **Action**: Link to CrowdSec Console or Dashboard.
2.  **WAF Card**:
    *   **Status**: Active / Disabled.
    *   **Content**: "OWASP CRS Loaded".
3.  **Access Control Lists (ACL)**:
    *   **Status**: Active / Disabled.
    *   **Action**: "Manage Blocklists" (opens modal/page to edit IP lists).
4.  **Rate Limiting**:
    *   **Status**: Active / Disabled.
    *   **Action**: "Configure Limits" (opens modal to set global requests/second).

---

## 4. Service-Specific Logic

### CrowdSec
*   **Local**:
    *   Installs CrowdSec agent via `apk`.
    *   Generates `acquis.yaml` to read Caddy logs.
    *   Configures Caddy bouncer to talk to `localhost:8080`.
*   **External**:
    *   Configures Caddy bouncer to talk to `CPM_SECURITY_CROWDSEC_API_URL`.

### WAF (Coraza)
*   **Implementation**:
    *   When enabled, inject `coraza_waf` directive into the global Caddyfile or per-host.
    *   Use default OWASP Core Rule Set (CRS).

### IP ACLs
*   **Implementation**:
    *   Create a snippet `(ip_filter)` in Caddyfile.
    *   Use `@matcher` with `remote_ip` to block/allow IPs.
    *   UI allows adding CIDR ranges to this list.

### Rate Limiting
*   **Implementation**:
    *   Use `rate_limit` directive.
    *   Allow user to define "zones" (e.g., API, Static) in the UI.

---

## 5. Documentation
*   **New Doc**: `docs/security.md`
*   **Content**:
    *   Explanation of each service.
    *   How to configure Env Vars.
    *   Trade-offs of "Local" CrowdSec (startup time vs convenience).
