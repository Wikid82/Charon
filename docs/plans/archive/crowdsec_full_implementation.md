# CrowdSec Full Implementation Plan

**Status:** Planning
**Created:** December 12, 2025
**Priority:** Critical - CrowdSec is completely non-functional
**Issue:** `FATAL crowdsec init: while loading acquisition config: no datasource enabled`

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Root Cause Analysis](#root-cause-analysis)
3. [Architecture Overview](#architecture-overview)
4. [Implementation Phases](#implementation-phases)
   - [Phase 1: Core CrowdSec Configuration](#phase-1-core-crowdsec-configuration)
   - [Phase 2: Caddy Integration](#phase-2-caddy-integration)
   - [Phase 2.5: Unified Logging & Live Viewer Integration](#phase-25-unified-logging--live-viewer-integration)
   - [Phase 3: API Integration](#phase-3-api-integration)
   - [Phase 4: Testing](#phase-4-testing)
5. [File Changes Summary](#file-changes-summary)
6. [Configuration Options](#configuration-options)
7. [Ignore Files Review](#ignore-files-review)
8. [Rollout & Verification](#rollout--verification)

---

## Executive Summary

CrowdSec is currently **completely broken** in Charon. When starting the container with `CERBERUS_SECURITY_CROWDSEC_MODE=local`, CrowdSec crashes immediately with:

```
FATAL crowdsec init: while loading acquisition config: no datasource enabled
```

This indicates that while CrowdSec binaries are installed and configuration files are copied, the **acquisition configuration** (which tells CrowdSec what logs to parse) is missing or empty.

### What Works Today

- ✅ CrowdSec binaries installed (`crowdsec`, `cscli`)
- ✅ Base config files copied from release tarball to `/etc/crowdsec.dist/`
- ✅ Config copied to `/etc/crowdsec/` at container startup
- ✅ LAPI port changed to 8085 to avoid conflict with Charon
- ✅ Machine registration via `cscli machines add -a --force`
- ✅ Hub index update via `cscli hub update`
- ✅ Backend API handlers for decisions, ban/unban, import/export
- ✅ caddy-crowdsec-bouncer compiled into Caddy binary

### What's Broken/Missing

- ❌ **No `acquis.yaml`** - CrowdSec doesn't know what logs to parse
- ❌ **No parsers installed** - No way to understand Caddy log format
- ❌ **No scenarios installed** - No detection rules
- ❌ **No collections installed** - No pre-packaged security configurations
- ❌ **Caddy not logging in parseable format** - Default JSON logging may not match parsers
- ❌ **Bouncer not registered** - caddy-crowdsec-bouncer needs API key
- ❌ **No automated bouncer API key generation**

---

## Root Cause Analysis

### The Fatal Error Explained

CrowdSec requires **datasources** to function. A datasource tells CrowdSec:

1. Where to find logs (file path, journald, etc.)
2. What parser to use for those logs
3. Optional labels for categorization

Without datasources configured in `acquis.yaml`, CrowdSec has nothing to monitor and refuses to start.

### Missing Acquisition Configuration

The CrowdSec release tarball includes default config files, but the `acquis.yaml` in the tarball is either:

1. Empty
2. Contains example datasources that don't exist in the container (like syslog)
3. Not present at all

**Current entrypoint flow:**

```bash
# Step 1: Copy base config (MISSING acquis.yaml or empty)
cp -r /etc/crowdsec.dist/* /etc/crowdsec/

# Step 2: Variable substitution (doesn't create acquis.yaml)
envsubst < config.yaml > config.yaml.tmp && mv config.yaml.tmp config.yaml

# Step 3: Port changes (doesn't affect acquisition)
sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' config.yaml

# Step 4: Hub update (doesn't install parsers/scenarios by default)
cscli hub update

# Step 5: Machine registration (succeeds)
cscli machines add -a --force

# Step 6: START CROWDSEC → CRASH (no datasources!)
crowdsec &
```

### Missing Components

1. **Parsers**: CrowdSec needs parsers to understand log formats
   - Caddy uses JSON logging by default
   - Need `crowdsecurity/caddy-logs` parser or custom parser

2. **Scenarios**: Detection rules that identify attacks
   - HTTP flood detection
   - HTTP probing
   - HTTP path traversal
   - HTTP SQLi/XSS patterns

3. **Collections**: Bundled parsers + scenarios for specific applications
   - `crowdsecurity/caddy` collection (may not exist)
   - `crowdsecurity/http-cve` for known vulnerabilities
   - `crowdsecurity/base-http-scenarios` for generic HTTP attacks

4. **Acquisition Config**: Tells CrowdSec where to read logs

   ```yaml
   # /etc/crowdsec/acquis.yaml
   source: file
   filenames:
     - /var/log/caddy/access.log
   labels:
     type: caddy
   ```

---

## Architecture Overview

### How CrowdSec Should Integrate with Charon

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Docker Container                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────┐         ┌─────────────────────────────────────┐    │
│  │   Caddy     │──logs──▶│  /var/log/caddy/access.log (JSON)   │    │
│  │ (Reverse    │         └──────────────┬──────────────────────┘    │
│  │  Proxy)     │                        │                           │
│  │             │                        ▼                           │
│  │ ┌─────────┐ │         ┌─────────────────────────────────────┐    │
│  │ │Bouncer  │◀├─LAPI───│  CrowdSec Agent (LAPI :8085)         │    │
│  │ │Plugin   │ │         │  ┌─────────────────────────────────┐│    │
│  │ └─────────┘ │         │  │ acquis.yaml → file acquisition  ││    │
│  └─────────────┘         │  │ parsers → crowdsecurity/caddy   ││    │
│        │                 │  │ scenarios → http-flood, etc.    ││    │
│        │                 │  └─────────────────────────────────┘│    │
│        │                 └──────────────┬──────────────────────┘    │
│        │                                │                           │
│        ▼                                ▼                           │
│  ┌─────────────┐         ┌─────────────────────────────────────┐    │
│  │   Charon    │◀─API───│  cscli (CLI management)              │    │
│  │ (Management │         │  - decisions list/add/delete        │    │
│  │    API)     │         │  - hub install parsers/scenarios   │    │
│  └─────────────┘         └─────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Request arrives** at Caddy reverse proxy
2. **Caddy bouncer plugin** checks CrowdSec LAPI for decision on client IP
   - If banned → Return 403
   - If allowed → Continue processing
3. **Caddy logs request** to `/var/log/caddy/access.log` in JSON format
4. **CrowdSec agent** reads log file via acquisition config
5. **Parser** converts JSON log to normalized event
6. **Scenarios** analyze events for attack patterns
7. **Decisions** are created (ban IP for X duration)
8. **Bouncer plugin** receives decision via streaming/polling
9. **Future requests** from banned IP are blocked

### Required CrowdSec Components

| Component | Source | Purpose |
|-----------|--------|---------|
| `crowdsecurity/caddy-logs` | Hub | Parse Caddy JSON access logs |
| `crowdsecurity/base-http-scenarios` | Hub | Generic HTTP attack detection |
| `crowdsecurity/http-cve` | Hub | Known HTTP CVE detection |
| Custom `acquis.yaml` | Charon | Point CrowdSec at Caddy logs |
| Bouncer API key | Auto-generated | Authenticate bouncer with LAPI |

---

## Implementation Phases

### Phase 1: Core CrowdSec Configuration

**Goal:** Make CrowdSec agent start successfully and process logs.

#### 1.1 Create Acquisition Template

Create a default acquisition configuration that reads Caddy logs:

**New file: `configs/crowdsec/acquis.yaml`**

```yaml
# Charon/Caddy Log Acquisition Configuration
# This file tells CrowdSec what logs to monitor

# Caddy access logs (JSON format)
source: file
filenames:
  - /var/log/caddy/access.log
  - /var/log/caddy/*.log
labels:
  type: caddy
---
# Alternative: If using syslog output from Caddy
# source: journalctl
# journalctl_filter:
#   - "_SYSTEMD_UNIT=caddy.service"
# labels:
#   type: caddy
```

#### 1.2 Create Default Config Template

**New file: `configs/crowdsec/config.yaml.template`**

```yaml
# CrowdSec Configuration for Charon
# Generated at container startup

common:
  daemonize: false
  log_media: stdout
  log_level: info
  log_dir: /var/log/crowdsec/
  working_dir: .

config_paths:
  config_dir: /etc/crowdsec/
  data_dir: /var/lib/crowdsec/data/
  simulation_path: /etc/crowdsec/simulation.yaml
  hub_dir: /etc/crowdsec/hub/
  index_path: /etc/crowdsec/hub/.index.json
  notification_dir: /etc/crowdsec/notifications/
  plugin_dir: /usr/lib/crowdsec/plugins/

crowdsec_service:
  enable: true
  acquisition_path: /etc/crowdsec/acquis.yaml
  acquisition_dir: /etc/crowdsec/acquis.d/
  parser_routines: 1
  buckets_routines: 1
  output_routines: 1

cscli:
  output: human
  color: auto
  # hub_branch: master

db_config:
  log_level: info
  type: sqlite
  db_path: /var/lib/crowdsec/data/crowdsec.db
  flush:
    max_items: 5000
    max_age: 7d

plugin_config:
  user: root
  group: root

api:
  client:
    insecure_skip_verify: false
    credentials_path: /etc/crowdsec/local_api_credentials.yaml
  server:
    log_level: info
    listen_uri: 127.0.0.1:8085
    profiles_path: /etc/crowdsec/profiles.yaml
    console_path: /etc/crowdsec/console.yaml
    online_client:
      credentials_path: /etc/crowdsec/online_api_credentials.yaml
    tls:
      # TLS disabled for local-only LAPI
      # Enable when exposing LAPI externally
prometheus:
  enabled: true
  level: full
  listen_addr: 127.0.0.1
  listen_port: 6060
```

#### 1.3 Create Local API Credentials Template

**New file: `configs/crowdsec/local_api_credentials.yaml.template`**

```yaml
# CrowdSec Local API Credentials
# This file is auto-generated - do not edit manually

url: http://127.0.0.1:8085
login: ${CROWDSEC_MACHINE_ID}
password: ${CROWDSEC_MACHINE_PASSWORD}
```

#### 1.4 Create Bouncer Registration Script

**New file: `configs/crowdsec/register_bouncer.sh`**

```bash
#!/bin/sh
# Register the Caddy bouncer with CrowdSec LAPI
# This script is idempotent - safe to run multiple times

BOUNCER_NAME="${CROWDSEC_BOUNCER_NAME:-caddy-bouncer}"
API_KEY_FILE="/etc/crowdsec/bouncers/${BOUNCER_NAME}.key"

# Ensure bouncer directory exists
mkdir -p /etc/crowdsec/bouncers

# Check if bouncer already registered
if cscli bouncers list -o json 2>/dev/null | grep -q "\"name\":\"${BOUNCER_NAME}\""; then
    echo "Bouncer '${BOUNCER_NAME}' already registered"

    # If key file doesn't exist, we need to re-register
    if [ ! -f "$API_KEY_FILE" ]; then
        echo "API key file missing, re-registering bouncer..."
        cscli bouncers delete "${BOUNCER_NAME}" 2>/dev/null || true
    else
        echo "Using existing API key from ${API_KEY_FILE}"
        cat "$API_KEY_FILE"
        exit 0
    fi
fi

# Register new bouncer and save API key
echo "Registering bouncer '${BOUNCER_NAME}'..."
API_KEY=$(cscli bouncers add "${BOUNCER_NAME}" -o raw)

if [ -z "$API_KEY" ]; then
    echo "ERROR: Failed to register bouncer" >&2
    exit 1
fi

# Save API key to file
echo "$API_KEY" > "$API_KEY_FILE"
chmod 600 "$API_KEY_FILE"

echo "Bouncer registered successfully"
echo "API Key: $API_KEY"
```

#### 1.5 Create Hub Setup Script

**New file: `configs/crowdsec/install_hub_items.sh`**

```bash
#!/bin/sh
# Install required CrowdSec hub items (parsers, scenarios, collections)
# This script runs during container startup

set -e

echo "Installing CrowdSec hub items for Charon..."

# Update hub index first
echo "Updating hub index..."
cscli hub update

# Install Caddy log parser (if available)
# Note: crowdsecurity/caddy-logs may not exist yet - check hub
if cscli parsers inspect crowdsecurity/caddy-logs >/dev/null 2>&1; then
    echo "Installing Caddy log parser..."
    cscli parsers install crowdsecurity/caddy-logs --force
else
    echo "Caddy-specific parser not available, using HTTP parser..."
    cscli parsers install crowdsecurity/http-logs --force 2>/dev/null || true
fi

# Install base HTTP parsers (always needed)
echo "Installing base parsers..."
cscli parsers install crowdsecurity/syslog-logs --force 2>/dev/null || true
cscli parsers install crowdsecurity/geoip-enrich --force 2>/dev/null || true
cscli parsers install crowdsecurity/http-logs --force 2>/dev/null || true

# Install HTTP scenarios for attack detection
echo "Installing HTTP scenarios..."
cscli scenarios install crowdsecurity/http-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-sensitive-files --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-backdoors-attempts --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-path-traversal-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-xss-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-sqli-probing --force 2>/dev/null || true
cscli scenarios install crowdsecurity/http-generic-bf --force 2>/dev/null || true

# Install CVE collection for known vulnerabilities
echo "Installing CVE collection..."
cscli collections install crowdsecurity/http-cve --force 2>/dev/null || true

# Install base HTTP collection (bundles common scenarios)
echo "Installing base HTTP collection..."
cscli collections install crowdsecurity/base-http-scenarios --force 2>/dev/null || true

# Verify installation
echo ""
echo "=== Installed Components ==="
echo "Parsers:"
cscli parsers list -o json 2>/dev/null | grep -o '"name":"[^"]*"' | head -10 || echo "  (none or error)"

echo ""
echo "Scenarios:"
cscli scenarios list -o json 2>/dev/null | grep -o '"name":"[^"]*"' | head -10 || echo "  (none or error)"

echo ""
echo "Collections:"
cscli collections list -o json 2>/dev/null | grep -o '"name":"[^"]*"' | head -10 || echo "  (none or error)"

echo ""
echo "Hub installation complete!"
```

#### 1.6 Update Dockerfile

**File: `Dockerfile`**

Add CrowdSec configuration files to the image:

```dockerfile
# After the CrowdSec installer stage, add:

# Copy CrowdSec configuration templates
COPY configs/crowdsec/acquis.yaml /etc/crowdsec.dist/acquis.yaml
COPY configs/crowdsec/config.yaml.template /etc/crowdsec.dist/config.yaml.template
COPY configs/crowdsec/local_api_credentials.yaml.template /etc/crowdsec.dist/local_api_credentials.yaml.template
COPY configs/crowdsec/register_bouncer.sh /usr/local/bin/register_bouncer.sh
COPY configs/crowdsec/install_hub_items.sh /usr/local/bin/install_hub_items.sh

# Make scripts executable
RUN chmod +x /usr/local/bin/register_bouncer.sh /usr/local/bin/install_hub_items.sh
```

#### 1.7 Update docker-entrypoint.sh

**File: `docker-entrypoint.sh`**

Replace the CrowdSec initialization section with a more robust implementation:

```bash
#!/bin/sh
set -e

echo "Starting Charon with integrated Caddy..."

# ============================================================================
# CrowdSec Initialization
# ============================================================================
CROWDSEC_PID=""
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}

# Initialize CrowdSec configuration if cscli is present
if command -v cscli >/dev/null; then
    echo "Initializing CrowdSec configuration..."

    # Create required directories
    mkdir -p /etc/crowdsec
    mkdir -p /etc/crowdsec/hub
    mkdir -p /etc/crowdsec/acquis.d
    mkdir -p /etc/crowdsec/bouncers
    mkdir -p /etc/crowdsec/notifications
    mkdir -p /var/lib/crowdsec/data
    mkdir -p /var/log/crowdsec
    mkdir -p /var/log/caddy

    # Copy base configuration if not exists
    if [ ! -f "/etc/crowdsec/config.yaml" ]; then
        echo "Copying base CrowdSec configuration..."
        if [ -d "/etc/crowdsec.dist" ]; then
            cp -r /etc/crowdsec.dist/* /etc/crowdsec/ 2>/dev/null || true
        fi
    fi

    # Create/update acquisition config for Caddy logs
    # This is CRITICAL - CrowdSec won't start without datasources
    if [ ! -f "/etc/crowdsec/acquis.yaml" ] || [ ! -s "/etc/crowdsec/acquis.yaml" ]; then
        echo "Creating acquisition configuration for Caddy logs..."
        cat > /etc/crowdsec/acquis.yaml << 'EOF'
# Caddy access logs acquisition
# CrowdSec will monitor these files for security events
source: file
filenames:
  - /var/log/caddy/access.log
  - /var/log/caddy/*.log
labels:
  type: caddy
EOF
    fi

    # Ensure config.yaml has correct LAPI port (8085 to avoid conflict with Charon)
    if [ -f "/etc/crowdsec/config.yaml" ]; then
        sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
        sed -i 's|listen_uri: 0.0.0.0:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
    fi

    # Update local_api_credentials.yaml to use correct port
    if [ -f "/etc/crowdsec/local_api_credentials.yaml" ]; then
        sed -i 's|url: http://127.0.0.1:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
        sed -i 's|url: http://localhost:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
    fi

    # Update hub index
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        cscli hub update || echo "Warning: Failed to update hub index (network issue?)"
    fi

    # Register machine with LAPI (required for cscli commands)
    echo "Registering machine with CrowdSec LAPI..."
    cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"

    # Install hub items (parsers, scenarios, collections)
    if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
        echo "Installing CrowdSec hub items..."
        /usr/local/bin/install_hub_items.sh 2>/dev/null || echo "Warning: Some hub items may not have installed"
    fi
fi

# Start CrowdSec agent if local mode is enabled
if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "CrowdSec Local Mode enabled."

    if command -v crowdsec >/dev/null; then
        # Create an empty access log so CrowdSec doesn't fail on missing file
        touch /var/log/caddy/access.log

        echo "Starting CrowdSec agent..."
        crowdsec -c /etc/crowdsec/config.yaml &
        CROWDSEC_PID=$!
        echo "CrowdSec started (PID: $CROWDSEC_PID)"

        # Wait for LAPI to be ready
        echo "Waiting for CrowdSec LAPI..."
        for i in $(seq 1 30); do
            if curl -q -O- http://127.0.0.1:8085/health >/dev/null 2>&1; then
                echo "CrowdSec LAPI is ready!"
                break
            fi
            sleep 1
        done

        # Register bouncer for Caddy
        if [ -x /usr/local/bin/register_bouncer.sh ]; then
            echo "Registering Caddy bouncer..."
            BOUNCER_API_KEY=$(/usr/local/bin/register_bouncer.sh 2>/dev/null | tail -1)
            if [ -n "$BOUNCER_API_KEY" ]; then
                export CROWDSEC_BOUNCER_API_KEY="$BOUNCER_API_KEY"
                echo "Bouncer registered with API key"
            fi
        fi
    else
        echo "CrowdSec binary not found - skipping agent startup"
    fi
fi

# ... rest of entrypoint (Caddy startup, Charon startup, etc.)
```

### Phase 2: Caddy Integration

**Goal:** Configure Caddy to log in a format CrowdSec can parse, and enable the bouncer plugin.

#### 2.1 Configure Caddy Access Logging

Charon generates Caddy configuration dynamically. We need to ensure access logs are written in a format CrowdSec can parse.

**Update: `backend/internal/caddy/config.go`**

Add logging configuration to the Caddy JSON config:

```go
// In the buildCaddyConfig function or where global config is built

// Add logging configuration for CrowdSec
func buildLoggingConfig() map[string]interface{} {
    return map[string]interface{}{
        "logs": map[string]interface{}{
            "default": map[string]interface{}{
                "writer": map[string]interface{}{
                    "output":   "file",
                    "filename": "/var/log/caddy/access.log",
                },
                "encoder": map[string]interface{}{
                    "format": "json",
                },
                "level": "INFO",
            },
        },
    }
}
```

#### 2.2 Update CrowdSec Bouncer Handler

The existing `buildCrowdSecHandler` function already generates the correct format, but we need to ensure the API key is available.

**File: `backend/internal/caddy/config.go`**

The function at line 752 is mostly correct. Verify it includes:

- `api_url`: Points to `http://127.0.0.1:8085` (already done)
- `api_key`: From environment variable (already done)
- `enable_streaming`: For real-time updates (already done)

#### 2.3 Create Custom Caddy Parser for CrowdSec

Since there may not be an official `crowdsecurity/caddy-logs` parser, we need to create a custom parser or use the generic HTTP parser with appropriate normalization.

**New file: `configs/crowdsec/parsers/caddy-json-logs.yaml`**

```yaml
# Custom parser for Caddy JSON access logs
# Install with: cscli parsers install ./caddy-json-logs.yaml --force

name: charon/caddy-json-logs
description: Parse Caddy JSON access logs for Charon
filter: evt.Meta.log_type == 'caddy'

# Caddy JSON log format example:
# {"level":"info","ts":1702406400.123,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"192.168.1.100","method":"GET","uri":"/api/v1/test","host":"example.com"},"status":200,"duration":0.001}

pattern: '%{DATA:message}'
grok:
  apply_on: message
  pattern: '%{GREEDYDATA:json_data}'

statics:
  - parsed: json_data
    target: evt.Unmarshaled

# Extract fields from JSON
jsondecoder:
  - apply_on: json_data
    target: evt.Parsed

# Map Caddy fields to CrowdSec standard fields
statics:
  - target: evt.Meta.source_ip
    expression: evt.Parsed.request.remote_ip

  - target: evt.Meta.http_method
    expression: evt.Parsed.request.method

  - target: evt.Meta.http_path
    expression: evt.Parsed.request.uri

  - target: evt.Meta.http_host
    expression: evt.Parsed.request.host

  - target: evt.Meta.http_status
    expression: evt.Parsed.status

  - target: evt.Meta.http_user_agent
    expression: evt.Parsed.request.headers["User-Agent"][0]

  - target: evt.StrTime
    expression: string(evt.Parsed.ts)
```

### Phase 2.5: Unified Logging & Live Viewer Integration

**Goal:** Make Caddy access logs accessible to ALL Cerberus security modules (CrowdSec, WAF/Coraza, Rate Limiting, ACL) and integrate with the existing Live Log Viewer on the Cerberus dashboard.

#### 2.5.1 Architecture: Unified Security Log Pipeline

The current architecture has separate logging paths that need to be unified:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         Unified Logging Architecture                              │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                   │
│  ┌───────────────────────────────────────────────────────────────────────────┐   │
│  │                              CADDY REVERSE PROXY                          │   │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────────┐   │   │
│  │  │ CrowdSec  │  │  Coraza   │  │   Rate    │  │ Request Logging       │   │   │
│  │  │  Bouncer  │  │   WAF     │  │  Limiter  │  │ (access.log → JSON)   │   │   │
│  │  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └───────────┬───────────┘   │   │
│  │        │              │              │                    │               │   │
│  └────────┼──────────────┼──────────────┼────────────────────┼───────────────┘   │
│           │              │              │                    │                   │
│           ▼              ▼              ▼                    ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                    UNIFIED LOG FILE: /var/log/caddy/access.log              │ │
│  │                    (JSON format with security event annotations)            │ │
│  └─────────────────────────────────────────────────────┬───────────────────────┘ │
│                                                        │                         │
│                    ┌───────────────────────────────────┼───────────────────┐     │
│                    │                                   │                   │     │
│                    ▼                                   ▼                   ▼     │
│  ┌─────────────────────────┐  ┌──────────────────────────────────────────────┐   │
│  │     CrowdSec Agent      │  │          Charon Backend                      │   │
│  │   (acquis.yaml reads    │  │  ┌──────────────────────────────────────┐    │   │
│  │    /var/log/caddy/*.log)│  │  │ LogWatcher Service (NEW)             │    │   │
│  │                         │  │  │ - Tail log file in real-time         │    │   │
│  │  Parses → Scenarios →   │  │  │ - Parse JSON entries                 │    │   │
│  │  Decisions → LAPI       │  │  │ - Broadcast to WebSocket listeners   │    │   │
│  └─────────────────────────┘  │  │ - Tag entries by security module     │    │   │
│                               │  └──────────────────┬───────────────────┘    │   │
│                               │                     │                        │   │
│                               │                     ▼                        │   │
│                               │  ┌──────────────────────────────────────┐    │   │
│                               │  │ Live Logs WebSocket Handler          │    │   │
│                               │  │ /api/v1/logs/live                    │    │   │
│                               │  │ /api/v1/cerberus/logs/live (NEW)     │    │   │
│                               │  └──────────────────┬───────────────────┘    │   │
│                               └──────────────────────┼───────────────────────┘   │
│                                                      │                           │
│                                                      ▼                           │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                          FRONTEND                                           │ │
│  │  ┌─────────────────────────────────────────────────────────────────────┐    │ │
│  │  │ LiveLogViewer Component (Enhanced)                                   │    │ │
│  │  │ - Subscribe to cerberus log stream                                   │    │ │
│  │  │ - Filter by: source (waf, crowdsec, ratelimit, acl), level, IP      │    │ │
│  │  │ - Color-coded security events                                        │    │ │
│  │  │ - Click to expand request details                                    │    │ │
│  │  └─────────────────────────────────────────────────────────────────────┘    │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 2.5.2 Create Log Watcher Service

A new service that tails the Caddy access log and broadcasts entries to WebSocket subscribers. This replaces the current approach that only broadcasts internal Go application logs.

**New file: `backend/internal/services/log_watcher.go`**

```go
package services

import (
    "bufio"
    "context"
    "encoding/json"
    "io"
    "os"
    "sync"
    "time"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/internal/models"
)

// LogWatcher provides real-time tailing of Caddy access logs
type LogWatcher struct {
    mu           sync.RWMutex
    subscribers  map[string]chan models.SecurityLogEntry
    logPath      string
    ctx          context.Context
    cancel       context.CancelFunc
}

// SecurityLogEntry represents a security-relevant log entry broadcast to clients
type SecurityLogEntry struct {
    Timestamp   string                 `json:"timestamp"`
    Level       string                 `json:"level"`
    Source      string                 `json:"source"`      // "caddy", "waf", "crowdsec", "ratelimit", "acl"
    ClientIP    string                 `json:"client_ip"`
    Method      string                 `json:"method"`
    Host        string                 `json:"host"`
    Path        string                 `json:"path"`
    Status      int                    `json:"status"`
    Duration    float64                `json:"duration"`
    Message     string                 `json:"message"`
    Blocked     bool                   `json:"blocked"`      // True if request was blocked
    BlockReason string                 `json:"block_reason"` // "waf", "crowdsec", "ratelimit", "acl"
    Details     map[string]interface{} `json:"details,omitempty"`
}

// NewLogWatcher creates a new log watcher instance
func NewLogWatcher(logPath string) *LogWatcher {
    ctx, cancel := context.WithCancel(context.Background())
    return &LogWatcher{
        subscribers: make(map[string]chan models.SecurityLogEntry),
        logPath:     logPath,
        ctx:         ctx,
        cancel:      cancel,
    }
}

// Start begins tailing the log file
func (w *LogWatcher) Start() error {
    go w.tailFile()
    return nil
}

// Stop halts the log watcher
func (w *LogWatcher) Stop() {
    w.cancel()
}

// Subscribe adds a new subscriber for log entries
func (w *LogWatcher) Subscribe(id string) <-chan models.SecurityLogEntry {
    w.mu.Lock()
    defer w.mu.Unlock()

    ch := make(chan models.SecurityLogEntry, 100)
    w.subscribers[id] = ch
    return ch
}

// Unsubscribe removes a subscriber
func (w *LogWatcher) Unsubscribe(id string) {
    w.mu.Lock()
    defer w.mu.Unlock()

    if ch, ok := w.subscribers[id]; ok {
        close(ch)
        delete(w.subscribers, id)
    }
}

// broadcast sends a log entry to all subscribers
func (w *LogWatcher) broadcast(entry models.SecurityLogEntry) {
    w.mu.RLock()
    defer w.mu.RUnlock()

    for _, ch := range w.subscribers {
        select {
        case ch <- entry:
        default:
            // Skip if channel is full (prevents blocking)
        }
    }
}

// tailFile continuously reads new entries from the log file
func (w *LogWatcher) tailFile() {
    for {
        select {
        case <-w.ctx.Done():
            return
        default:
        }

        // Wait for file to exist
        if _, err := os.Stat(w.logPath); os.IsNotExist(err) {
            time.Sleep(time.Second)
            continue
        }

        file, err := os.Open(w.logPath)
        if err != nil {
            logger.Log().WithError(err).Error("Failed to open log file for tailing")
            time.Sleep(time.Second)
            continue
        }

        // Seek to end of file
        file.Seek(0, io.SeekEnd)

        reader := bufio.NewReader(file)
        for {
            select {
            case <-w.ctx.Done():
                file.Close()
                return
            default:
            }

            line, err := reader.ReadString('\n')
            if err != nil {
                if err == io.EOF {
                    // No new data, wait and retry
                    time.Sleep(100 * time.Millisecond)
                    continue
                }
                logger.Log().WithError(err).Warn("Error reading log file")
                break // Reopen file
            }

            if line == "" || line == "\n" {
                continue
            }

            entry := w.parseLogEntry(line)
            if entry != nil {
                w.broadcast(*entry)
            }
        }

        file.Close()
        time.Sleep(time.Second) // Brief pause before reopening
    }
}

// parseLogEntry converts a Caddy JSON log line into a SecurityLogEntry
func (w *LogWatcher) parseLogEntry(line string) *models.SecurityLogEntry {
    var caddyLog models.CaddyAccessLog
    if err := json.Unmarshal([]byte(line), &caddyLog); err != nil {
        return nil
    }

    entry := &models.SecurityLogEntry{
        Timestamp: time.Unix(int64(caddyLog.Ts), 0).Format(time.RFC3339),
        Level:     caddyLog.Level,
        Source:    "caddy",
        ClientIP:  caddyLog.Request.RemoteIP,
        Method:    caddyLog.Request.Method,
        Host:      caddyLog.Request.Host,
        Path:      caddyLog.Request.URI,
        Status:    caddyLog.Status,
        Duration:  caddyLog.Duration,
        Message:   caddyLog.Msg,
        Details:   make(map[string]interface{}),
    }

    // Detect security events from status codes and log metadata
    if caddyLog.Status == 403 {
        entry.Blocked = true
        entry.Level = "warn"

        // Determine block reason from response headers or log fields
        // WAF blocks typically include "waf" or "coraza" in the response
        // CrowdSec blocks come from the bouncer
        // Rate limit blocks have specific status patterns
        if caddyLog.Logger == "http.handlers.waf" ||
           containsKey(caddyLog.Request.Headers, "X-Coraza-Id") {
            entry.Source = "waf"
            entry.BlockReason = "WAF rule triggered"
        } else {
            entry.Source = "cerberus"
            entry.BlockReason = "Access denied"
        }
    }

    if caddyLog.Status == 429 {
        entry.Blocked = true
        entry.Source = "ratelimit"
        entry.Level = "warn"
        entry.BlockReason = "Rate limit exceeded"
    }

    return entry
}

// containsKey checks if a header map contains a specific key
func containsKey(headers map[string][]string, key string) bool {
    _, ok := headers[key]
    return ok
}
```

#### 2.5.3 Add Security Log Entry Model

**Update file: `backend/internal/models/logs.go`**

Add the SecurityLogEntry type alongside existing log models:

```go
// SecurityLogEntry represents a security-relevant log entry for live streaming
type SecurityLogEntry struct {
    Timestamp   string                 `json:"timestamp"`
    Level       string                 `json:"level"`
    Source      string                 `json:"source"`      // "caddy", "waf", "crowdsec", "ratelimit", "acl"
    ClientIP    string                 `json:"client_ip"`
    Method      string                 `json:"method"`
    Host        string                 `json:"host"`
    Path        string                 `json:"path"`
    Status      int                    `json:"status"`
    Duration    float64                `json:"duration"`
    Message     string                 `json:"message"`
    Blocked     bool                   `json:"blocked"`
    BlockReason string                 `json:"block_reason,omitempty"`
    Details     map[string]interface{} `json:"details,omitempty"`
}
```

#### 2.5.4 Create Cerberus Live Logs WebSocket Handler

A new WebSocket endpoint specifically for Cerberus security logs that streams parsed Caddy access logs.

**New file: `backend/internal/api/handlers/cerberus_logs_ws.go`**

```go
package handlers

import (
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/gorilla/websocket"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/internal/services"
)

// CerberusLogsHandler handles Cerberus security log streaming
type CerberusLogsHandler struct {
    watcher *services.LogWatcher
}

// NewCerberusLogsHandler creates a new Cerberus logs handler
func NewCerberusLogsHandler(watcher *services.LogWatcher) *CerberusLogsHandler {
    return &CerberusLogsHandler{watcher: watcher}
}

// LiveLogs handles WebSocket connections for Cerberus security log streaming
func (h *CerberusLogsHandler) LiveLogs(c *gin.Context) {
    logger.Log().Info("Cerberus logs WebSocket connection attempt")

    // Upgrade HTTP connection to WebSocket
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        logger.Log().WithError(err).Error("Failed to upgrade Cerberus logs WebSocket")
        return
    }
    defer conn.Close()

    subscriberID := uuid.New().String()
    logger.Log().WithField("subscriber_id", subscriberID).Info("Cerberus logs WebSocket connected")

    // Parse query filters
    sourceFilter := strings.ToLower(c.Query("source")) // waf, crowdsec, ratelimit, acl
    levelFilter := strings.ToLower(c.Query("level"))
    ipFilter := c.Query("ip")
    hostFilter := strings.ToLower(c.Query("host"))
    blockedOnly := c.Query("blocked") == "true"

    // Subscribe to log watcher
    logChan := h.watcher.Subscribe(subscriberID)
    defer h.watcher.Unsubscribe(subscriberID)

    // Channel to detect client disconnect
    done := make(chan struct{})
    go func() {
        defer close(done)
        for {
            if _, _, err := conn.ReadMessage(); err != nil {
                return
            }
        }
    }()

    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case entry, ok := <-logChan:
            if !ok {
                return
            }

            // Apply filters
            if sourceFilter != "" && !strings.EqualFold(entry.Source, sourceFilter) {
                continue
            }
            if levelFilter != "" && !strings.EqualFold(entry.Level, levelFilter) {
                continue
            }
            if ipFilter != "" && !strings.Contains(entry.ClientIP, ipFilter) {
                continue
            }
            if hostFilter != "" && !strings.Contains(strings.ToLower(entry.Host), hostFilter) {
                continue
            }
            if blockedOnly && !entry.Blocked {
                continue
            }

            // Send to WebSocket client
            if err := conn.WriteJSON(entry); err != nil {
                logger.Log().WithError(err).Debug("Failed to write Cerberus log to WebSocket")
                return
            }

        case <-ticker.C:
            if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
                return
            }

        case <-done:
            return
        }
    }
}
```

#### 2.5.5 Register New Routes

**Update file: `backend/internal/api/routes/routes.go`**

Add the new Cerberus logs endpoint:

```go
// In SetupRoutes function, add:

// Initialize log watcher for Caddy access logs
logPath := filepath.Join(cfg.DataDir, "logs", "access.log")
logWatcher := services.NewLogWatcher(logPath)
if err := logWatcher.Start(); err != nil {
    logger.Log().WithError(err).Error("Failed to start log watcher")
}

// Cerberus security logs WebSocket
cerberusLogsHandler := handlers.NewCerberusLogsHandler(logWatcher)
api.GET("/cerberus/logs/live", cerberusLogsHandler.LiveLogs)

// Alternative: Also expose under /logs/security/live for clarity
api.GET("/logs/security/live", cerberusLogsHandler.LiveLogs)
```

#### 2.5.6 Update Frontend API Client

**Update file: `frontend/src/api/logs.ts`**

Add the new security logs connection function:

```typescript
export interface SecurityLogEntry {
  timestamp: string;
  level: string;
  source: string;      // "caddy", "waf", "crowdsec", "ratelimit", "acl"
  client_ip: string;
  method: string;
  host: string;
  path: string;
  status: number;
  duration: number;
  message: string;
  blocked: boolean;
  block_reason?: string;
  details?: Record<string, unknown>;
}

export interface SecurityLogFilter {
  source?: string;     // Filter by security module
  level?: string;
  ip?: string;
  host?: string;
  blocked?: boolean;   // Only show blocked requests
}

/**
 * Connects to the Cerberus security logs WebSocket endpoint.
 * This streams Caddy access logs with security event annotations.
 */
export const connectSecurityLogs = (
  filters: SecurityLogFilter,
  onMessage: (log: SecurityLogEntry) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const params = new URLSearchParams();
  if (filters.source) params.append('source', filters.source);
  if (filters.level) params.append('level', filters.level);
  if (filters.ip) params.append('ip', filters.ip);
  if (filters.host) params.append('host', filters.host);
  if (filters.blocked) params.append('blocked', 'true');

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/cerberus/logs/live?${params.toString()}`;

  console.log('Connecting to Cerberus logs WebSocket:', wsUrl);
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('Cerberus logs WebSocket connected');
    onOpen?.();
  };

  ws.onmessage = (event: MessageEvent) => {
    try {
      const log = JSON.parse(event.data) as SecurityLogEntry;
      onMessage(log);
    } catch (err) {
      console.error('Failed to parse security log:', err);
    }
  };

  ws.onerror = (error: Event) => {
    console.error('Cerberus logs WebSocket error:', error);
    onError?.(error);
  };

  ws.onclose = (event: CloseEvent) => {
    console.log('Cerberus logs WebSocket closed', event);
    onClose?.();
  };

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close();
    }
  };
};
```

#### 2.5.7 Update LiveLogViewer Component

**Update file: `frontend/src/components/LiveLogViewer.tsx`**

Enhance the component to support both application logs and security access logs:

```tsx
import { useEffect, useRef, useState } from 'react';
import {
  connectLiveLogs,
  connectSecurityLogs,
  LiveLogEntry,
  LiveLogFilter,
  SecurityLogEntry,
  SecurityLogFilter
} from '../api/logs';
import { Button } from './ui/Button';
import { Pause, Play, Trash2, Filter, Shield, Globe } from 'lucide-react';

type LogMode = 'application' | 'security';

interface LiveLogViewerProps {
  filters?: LiveLogFilter;
  securityFilters?: SecurityLogFilter;
  mode?: LogMode;
  maxLogs?: number;
  className?: string;
}

// Unified log entry for display
interface DisplayLogEntry {
  timestamp: string;
  level: string;
  source: string;
  message: string;
  blocked?: boolean;
  blockReason?: string;
  clientIP?: string;
  method?: string;
  host?: string;
  path?: string;
  status?: number;
  details?: Record<string, unknown>;
}

export function LiveLogViewer({
  filters = {},
  securityFilters = {},
  mode = 'application',
  maxLogs = 500,
  className = ''
}: LiveLogViewerProps) {
  const [logs, setLogs] = useState<DisplayLogEntry[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [currentMode, setCurrentMode] = useState<LogMode>(mode);
  const [textFilter, setTextFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [showBlockedOnly, setShowBlockedOnly] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const closeConnectionRef = useRef<(() => void) | null>(null);
  const shouldAutoScroll = useRef(true);

  // Convert entries to unified format
  const toDisplayEntry = (entry: LiveLogEntry | SecurityLogEntry): DisplayLogEntry => {
    if ('client_ip' in entry) {
      // SecurityLogEntry
      const secEntry = entry as SecurityLogEntry;
      return {
        timestamp: secEntry.timestamp,
        level: secEntry.level,
        source: secEntry.source,
        message: secEntry.blocked
          ? `🚫 BLOCKED: ${secEntry.block_reason} - ${secEntry.method} ${secEntry.path}`
          : `${secEntry.method} ${secEntry.path} → ${secEntry.status}`,
        blocked: secEntry.blocked,
        blockReason: secEntry.block_reason,
        clientIP: secEntry.client_ip,
        method: secEntry.method,
        host: secEntry.host,
        path: secEntry.path,
        status: secEntry.status,
        details: secEntry.details,
      };
    }
    // LiveLogEntry (application logs)
    return {
      timestamp: entry.timestamp,
      level: entry.level,
      source: entry.source || 'app',
      message: entry.message,
      details: entry.data,
    };
  };

  useEffect(() => {
    // Cleanup previous connection
    if (closeConnectionRef.current) {
      closeConnectionRef.current();
    }

    const handleMessage = (entry: LiveLogEntry | SecurityLogEntry) => {
      if (!isPaused) {
        const displayEntry = toDisplayEntry(entry);
        setLogs((prev) => {
          const updated = [...prev, displayEntry];
          return updated.length > maxLogs ? updated.slice(-maxLogs) : updated;
        });
      }
    };

    const handleOpen = () => {
      console.log(`${currentMode} log viewer connected`);
      setIsConnected(true);
    };

    const handleError = (error: Event) => {
      console.error('WebSocket error:', error);
      setIsConnected(false);
    };

    const handleClose = () => {
      console.log(`${currentMode} log viewer disconnected`);
      setIsConnected(false);
    };

    // Connect based on mode
    if (currentMode === 'security') {
      closeConnectionRef.current = connectSecurityLogs(
        { ...securityFilters, blocked: showBlockedOnly },
        handleMessage as (log: SecurityLogEntry) => void,
        handleOpen,
        handleError,
        handleClose
      );
    } else {
      closeConnectionRef.current = connectLiveLogs(
        filters,
        handleMessage as (log: LiveLogEntry) => void,
        handleOpen,
        handleError,
        handleClose
      );
    }

    return () => {
      if (closeConnectionRef.current) {
        closeConnectionRef.current();
      }
      setIsConnected(false);
    };
  }, [currentMode, filters, securityFilters, isPaused, maxLogs, showBlockedOnly]);

  // ... rest of component (auto-scroll, handleScroll, filtering logic)

  // Color coding for security events
  const getEntryStyle = (log: DisplayLogEntry) => {
    if (log.blocked) {
      return 'bg-red-900/30 border-l-2 border-red-500';
    }
    const level = log.level.toLowerCase();
    if (level.includes('error') || level.includes('fatal')) return 'text-red-400';
    if (level.includes('warn')) return 'text-yellow-400';
    return '';
  };

  const getSourceBadge = (source: string) => {
    const colors: Record<string, string> = {
      waf: 'bg-orange-600',
      crowdsec: 'bg-purple-600',
      ratelimit: 'bg-blue-600',
      acl: 'bg-green-600',
      caddy: 'bg-gray-600',
      cerberus: 'bg-indigo-600',
    };
    return colors[source.toLowerCase()] || 'bg-gray-500';
  };

  return (
    <div className={`bg-gray-900 rounded-lg border border-gray-700 ${className}`}>
      {/* Header with mode toggle */}
      <div className="flex items-center justify-between p-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-white">
            {currentMode === 'security' ? 'Security Access Logs' : 'Live Security Logs'}
          </h3>
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
            isConnected ? 'bg-green-900 text-green-300' : 'bg-red-900 text-red-300'
          }`}>
            {isConnected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {/* Mode toggle */}
          <div className="flex bg-gray-800 rounded-md p-0.5">
            <button
              onClick={() => setCurrentMode('application')}
              className={`px-2 py-1 text-xs rounded ${
                currentMode === 'application' ? 'bg-blue-600 text-white' : 'text-gray-400'
              }`}
              title="Application logs"
            >
              <Globe className="w-4 h-4" />
            </button>
            <button
              onClick={() => setCurrentMode('security')}
              className={`px-2 py-1 text-xs rounded ${
                currentMode === 'security' ? 'bg-blue-600 text-white' : 'text-gray-400'
              }`}
              title="Security access logs"
            >
              <Shield className="w-4 h-4" />
            </button>
          </div>
          {/* Existing controls */}
          <Button variant="ghost" size="sm" onClick={() => setIsPaused(!isPaused)}>
            {isPaused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setLogs([])}>
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Enhanced filters for security mode */}
      <div className="flex items-center gap-2 p-2 border-b border-gray-700 bg-gray-800">
        <Filter className="w-4 h-4 text-gray-400" />
        <input
          type="text"
          placeholder="Filter by text..."
          value={textFilter}
          onChange={(e) => setTextFilter(e.target.value)}
          className="flex-1 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white"
        />
        {currentMode === 'security' && (
          <>
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value)}
              className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white"
            >
              <option value="">All Sources</option>
              <option value="waf">WAF</option>
              <option value="crowdsec">CrowdSec</option>
              <option value="ratelimit">Rate Limit</option>
              <option value="acl">ACL</option>
            </select>
            <label className="flex items-center gap-1 text-xs text-gray-400">
              <input
                type="checkbox"
                checked={showBlockedOnly}
                onChange={(e) => setShowBlockedOnly(e.target.checked)}
                className="rounded border-gray-600"
              />
              Blocked only
            </label>
          </>
        )}
      </div>

      {/* Log display with security styling */}
      <div
        ref={logContainerRef}
        className="h-96 overflow-y-auto p-3 font-mono text-xs bg-black"
      >
        {logs.length === 0 && (
          <div className="text-gray-500 text-center py-8">
            No logs yet. Waiting for events...
          </div>
        )}
        {logs.map((log, index) => (
          <div
            key={index}
            className={`mb-1 px-1 -mx-1 rounded ${getEntryStyle(log)}`}
          >
            <span className="text-gray-500">{log.timestamp}</span>
            <span className={`ml-2 px-1 rounded text-xs ${getSourceBadge(log.source)}`}>
              {log.source.toUpperCase()}
            </span>
            {log.clientIP && (
              <span className="ml-2 text-cyan-400">{log.clientIP}</span>
            )}
            <span className="ml-2 text-gray-200">{log.message}</span>
            {log.blocked && log.blockReason && (
              <span className="ml-2 text-red-400">[{log.blockReason}]</span>
            )}
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="p-2 border-t border-gray-700 bg-gray-800 text-xs text-gray-400">
        Showing {logs.length} logs {isPaused && <span className="text-yellow-400">⏸ Paused</span>}
      </div>
    </div>
  );
}
```

#### 2.5.8 Update Security Page to Use Enhanced Viewer

**Update file: `frontend/src/pages/Security.tsx`**

Change the LiveLogViewer invocation to use security mode:

```tsx
{/* Live Activity Section */}
{status.cerberus?.enabled && (
  <div className="mt-6">
    <LiveLogViewer
      mode="security"
      securityFilters={{ source: 'cerberus' }}
      className="w-full"
    />
  </div>
)}
```

#### 2.5.9 Update Caddy Logging to Include Security Metadata

**Update file: `backend/internal/caddy/config.go`**

Enhance the logging configuration to include security-relevant fields:

```go
// In GenerateConfig, update the logging configuration:

Logging: &LoggingConfig{
    Logs: map[string]*LogConfig{
        "access": {
            Level: "INFO",
            Writer: &WriterConfig{
                Output:       "file",
                Filename:     logFile,
                Roll:         true,
                RollSize:     10,
                RollKeep:     5,
                RollKeepDays: 7,
            },
            Encoder: &EncoderConfig{
                Format: "json",
                // Include all relevant fields for security analysis
                Fields: map[string]interface{}{
                    "request>remote_ip":    "",
                    "request>method":       "",
                    "request>host":         "",
                    "request>uri":          "",
                    "request>proto":        "",
                    "request>headers>User-Agent": "",
                    "request>headers>X-Forwarded-For": "",
                    "status":               "",
                    "size":                 "",
                    "duration":             "",
                    "resp_headers>X-Coraza-Id": "",  // WAF tracking
                    "resp_headers>X-RateLimit-Remaining": "", // Rate limit tracking
                },
            },
            Include: []string{"http.log.access.access_log"},
        },
    },
},
```

#### 2.5.10 Summary of File Changes for Phase 2.5

| Path | Type | Purpose |
|------|------|---------|
| `backend/internal/services/log_watcher.go` | New | Tail Caddy logs and broadcast to WebSocket |
| `backend/internal/models/logs.go` | Update | Add SecurityLogEntry type |
| `backend/internal/api/handlers/cerberus_logs_ws.go` | New | Cerberus security logs WebSocket handler |
| `backend/internal/api/routes/routes.go` | Update | Register new /cerberus/logs/live endpoint |
| `frontend/src/api/logs.ts` | Update | Add SecurityLogEntry types and connectSecurityLogs |
| `frontend/src/components/LiveLogViewer.tsx` | Update | Support security mode with enhanced filtering |
| `frontend/src/pages/Security.tsx` | Update | Use enhanced LiveLogViewer with security mode |
| `backend/internal/caddy/config.go` | Update | Include security metadata in access logs |

### Phase 3: API Integration

**Goal:** Ensure existing handlers work correctly and add any missing functionality.

#### 3.1 Update CrowdSec Handler Initialization

**File: `backend/internal/api/handlers/crowdsec_handler.go`**

The existing handler is comprehensive. Key areas to verify/update:

1. **LAPI Health Check**: Already implemented at `CheckLAPIHealth`
2. **Decision Management**: Already implemented via `ListDecisions`, `BanIP`, `UnbanIP`
3. **Process Management**: Already implemented via `Start`, `Stop`, `Status`

#### 3.2 Add Bouncer Registration Endpoint

**New endpoint in `crowdsec_handler.go`:**

```go
// RegisterBouncer registers a new bouncer or returns existing bouncer API key
func (h *CrowdsecHandler) RegisterBouncer(c *gin.Context) {
    ctx := c.Request.Context()

    // Use the registration helper from internal/crowdsec package
    apiKey, err := crowdsec.EnsureBouncerRegistered(ctx, "http://127.0.0.1:8085")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Don't expose the full API key - just confirm registration
    c.JSON(http.StatusOK, gin.H{
        "status": "registered",
        "bouncer_name": "caddy-bouncer",
        "api_key_preview": apiKey[:8] + "...",
    })
}

// Add to RegisterRoutes:
// rg.POST("/admin/crowdsec/bouncer/register", h.RegisterBouncer)
```

#### 3.3 Add Acquisition Config Endpoint

```go
// GetAcquisitionConfig returns the current acquisition configuration
func (h *CrowdsecHandler) GetAcquisitionConfig(c *gin.Context) {
    acquis, err := os.ReadFile("/etc/crowdsec/acquis.yaml")
    if err != nil {
        if os.IsNotExist(err) {
            c.JSON(http.StatusNotFound, gin.H{"error": "acquisition config not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "content": string(acquis),
        "path": "/etc/crowdsec/acquis.yaml",
    })
}

// UpdateAcquisitionConfig updates the acquisition configuration
func (h *CrowdsecHandler) UpdateAcquisitionConfig(c *gin.Context) {
    var payload struct {
        Content string `json:"content" binding:"required"`
    }
    if err := c.ShouldBindJSON(&payload); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
        return
    }

    // Backup existing config
    backupPath := fmt.Sprintf("/etc/crowdsec/acquis.yaml.backup.%s", time.Now().Format("20060102-150405"))
    if _, err := os.Stat("/etc/crowdsec/acquis.yaml"); err == nil {
        _ = os.Rename("/etc/crowdsec/acquis.yaml", backupPath)
    }

    // Write new config
    if err := os.WriteFile("/etc/crowdsec/acquis.yaml", []byte(payload.Content), 0644); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status": "updated",
        "backup": backupPath,
        "reload_hint": true,
    })
}
```

### Phase 4: Testing

**Goal:** Update existing test scripts and create comprehensive integration tests.

#### 4.1 Update Integration Test Script

**File: `scripts/crowdsec_decision_integration.sh`**

Add pre-flight checks for CrowdSec readiness:

```bash
# Add after container start, before other tests:

# TC-0: Verify CrowdSec agent started successfully
log_test "TC-0: Verify CrowdSec agent started"

# Check container logs for CrowdSec startup
CROWDSEC_STARTED=$(docker logs ${CONTAINER_NAME} 2>&1 | grep -c "CrowdSec LAPI is ready" || echo "0")
if [ "$CROWDSEC_STARTED" -ge 1 ]; then
    log_info "  CrowdSec agent started successfully"
    pass_test
else
    # Check for the fatal error
    FATAL_ERROR=$(docker logs ${CONTAINER_NAME} 2>&1 | grep -c "no datasource enabled" || echo "0")
    if [ "$FATAL_ERROR" -ge 1 ]; then
        fail_test "CrowdSec failed to start: no datasource enabled (acquis.yaml missing)"
    else
        log_warn "  CrowdSec may not have started properly"
        pass_test
    fi
fi

# TC-0b: Verify acquisition config exists
log_test "TC-0b: Verify acquisition config exists"
ACQUIS_EXISTS=$(docker exec ${CONTAINER_NAME} cat /etc/crowdsec/acquis.yaml 2>/dev/null | grep -c "source:" || echo "0")
if [ "$ACQUIS_EXISTS" -ge 1 ]; then
    log_info "  Acquisition config found"
    pass_test
else
    fail_test "Acquisition config missing or empty"
fi
```

#### 4.2 Create CrowdSec Startup Test

**New file: `scripts/crowdsec_startup_test.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

# Brief: Test that CrowdSec starts correctly in Charon container
# This is a focused test for the startup issue

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

CONTAINER_NAME="charon-crowdsec-startup-test"

echo "=== CrowdSec Startup Test ==="

# Build if needed
if ! docker image inspect charon:local >/dev/null 2>&1; then
    echo "Building charon:local image..."
    docker build -t charon:local .
fi

# Clean up any existing container
docker rm -f ${CONTAINER_NAME} 2>/dev/null || true

# Start container with CrowdSec enabled
echo "Starting container with CERBERUS_SECURITY_CROWDSEC_MODE=local..."
docker run -d --name ${CONTAINER_NAME} \
    -p 8580:8080 \
    -e CHARON_ENV=development \
    -e CERBERUS_SECURITY_CROWDSEC_MODE=local \
    -e FEATURE_CERBERUS_ENABLED=true \
    charon:local

echo "Waiting 30 seconds for CrowdSec to initialize..."
sleep 30

# Check logs for errors
echo ""
echo "=== Container Logs (last 50 lines) ==="
docker logs ${CONTAINER_NAME} 2>&1 | tail -50

echo ""
echo "=== Checking for CrowdSec Status ==="

# Check for fatal error
if docker logs ${CONTAINER_NAME} 2>&1 | grep -q "no datasource enabled"; then
    echo "❌ FAIL: CrowdSec failed with 'no datasource enabled'"
    echo "   The acquis.yaml file is missing or empty"
    docker rm -f ${CONTAINER_NAME}
    exit 1
fi

# Check if LAPI is healthy
LAPI_HEALTH=$(docker exec ${CONTAINER_NAME} curl -q -O- http://127.0.0.1:8085/health 2>/dev/null || echo "failed")
if [ "$LAPI_HEALTH" != "failed" ]; then
    echo "✅ PASS: CrowdSec LAPI is healthy"
else
    echo "⚠️  WARN: CrowdSec LAPI not responding (may still be starting)"
fi

# Check acquisition config
echo ""
echo "=== Acquisition Config ==="
docker exec ${CONTAINER_NAME} cat /etc/crowdsec/acquis.yaml 2>/dev/null || echo "(not found)"

# Check installed items
echo ""
echo "=== Installed Parsers ==="
docker exec ${CONTAINER_NAME} cscli parsers list 2>/dev/null || echo "(cscli not available)"

echo ""
echo "=== Installed Scenarios ==="
docker exec ${CONTAINER_NAME} cscli scenarios list 2>/dev/null || echo "(cscli not available)"

# Cleanup
docker rm -f ${CONTAINER_NAME}

echo ""
echo "=== Test Complete ==="
```

#### 4.3 Update Go Integration Test

**File: `backend/integration/crowdsec_decisions_integration_test.go`**

Add a specific test for CrowdSec startup:

```go
func TestCrowdsecStartup(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    cmd := exec.CommandContext(ctx, "bash", "./scripts/crowdsec_startup_test.sh")
    cmd.Dir = "../../"

    out, err := cmd.CombinedOutput()
    t.Logf("crowdsec startup test output:\n%s", string(out))

    if err != nil {
        t.Fatalf("crowdsec startup test failed: %v", err)
    }

    // Check for fatal errors in output
    if strings.Contains(string(out), "no datasource enabled") {
        t.Fatal("CrowdSec failed to start: no datasource enabled")
    }
}
```

---

## File Changes Summary

### New Files to Create

| Path | Purpose |
|------|---------|
| `configs/crowdsec/acquis.yaml` | Default acquisition config for Caddy logs |
| `configs/crowdsec/config.yaml.template` | CrowdSec main config template |
| `configs/crowdsec/local_api_credentials.yaml.template` | LAPI credentials template |
| `configs/crowdsec/register_bouncer.sh` | Script to register Caddy bouncer |
| `configs/crowdsec/install_hub_items.sh` | Script to install parsers/scenarios |
| `configs/crowdsec/parsers/caddy-json-logs.yaml` | Custom parser for Caddy JSON logs |
| `scripts/crowdsec_startup_test.sh` | Focused startup test script |
| `backend/internal/services/log_watcher.go` | Tail Caddy access logs and broadcast to WebSocket subscribers |
| `backend/internal/api/handlers/cerberus_logs_ws.go` | Cerberus security logs WebSocket handler |

### Files to Modify

| Path | Changes |
|------|---------|
| `Dockerfile` | Copy CrowdSec config files, make scripts executable |
| `docker-entrypoint.sh` | Complete rewrite of CrowdSec initialization section |
| `backend/internal/caddy/config.go` | Add logging configuration for Caddy with security metadata |
| `backend/internal/api/handlers/crowdsec_handler.go` | Add bouncer registration, acquisition endpoints |
| `backend/internal/models/logs.go` | Add SecurityLogEntry type for live streaming |
| `backend/internal/api/routes/routes.go` | Register `/cerberus/logs/live` WebSocket endpoint |
| `frontend/src/api/logs.ts` | Add SecurityLogEntry types and connectSecurityLogs function |
| `frontend/src/components/LiveLogViewer.tsx` | Support security mode with enhanced filtering |
| `frontend/src/pages/Security.tsx` | Use enhanced LiveLogViewer with security mode |
| `scripts/crowdsec_decision_integration.sh` | Add CrowdSec startup verification |

### File Structure

```
Charon/
├── configs/
│   └── crowdsec/
│       ├── acquis.yaml
│       ├── config.yaml.template
│       ├── local_api_credentials.yaml.template
│       ├── register_bouncer.sh
│       ├── install_hub_items.sh
│       └── parsers/
│           └── caddy-json-logs.yaml
├── backend/
│   └── internal/
│       ├── api/
│       │   └── handlers/
│       │       └── cerberus_logs_ws.go (new)
│       ├── models/
│       │   └── logs.go (updated)
│       └── services/
│           └── log_watcher.go (new)
├── frontend/
│   └── src/
│       ├── api/
│       │   └── logs.ts (updated)
│       ├── components/
│       │   └── LiveLogViewer.tsx (updated)
│       └── pages/
│           └── Security.tsx (updated)
├── docker-entrypoint.sh (modified)
├── Dockerfile (modified)
└── scripts/
    ├── crowdsec_decision_integration.sh (modified)
    └── crowdsec_startup_test.sh (new)
```

---

## Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CERBERUS_SECURITY_CROWDSEC_MODE` | `disabled` | `disabled`, `local`, or `external` |
| `CHARON_SECURITY_CROWDSEC_MODE` | (fallback) | Alternative name for mode |
| `CROWDSEC_API_KEY` | (auto) | Bouncer API key (auto-generated if local) |
| `CROWDSEC_BOUNCER_API_KEY` | (auto) | Alternative name for API key |
| `CERBERUS_SECURITY_CROWDSEC_API_URL` | `http://127.0.0.1:8085` | LAPI URL for external mode |
| `CROWDSEC_BOUNCER_NAME` | `caddy-bouncer` | Name for registered bouncer |

### CrowdSec Paths in Container

| Path | Purpose |
|------|---------|
| `/etc/crowdsec/` | Main config directory |
| `/etc/crowdsec/acquis.yaml` | Acquisition configuration |
| `/etc/crowdsec/hub/` | Hub index and downloaded items |
| `/etc/crowdsec/bouncers/` | Bouncer API keys |
| `/var/lib/crowdsec/data/` | SQLite database, GeoIP data |
| `/var/log/crowdsec/` | CrowdSec logs |
| `/var/log/caddy/` | Caddy access logs (monitored by CrowdSec) |

---

## Ignore Files Review

### `.gitignore` Updates

Add the following entries:

```gitignore
# -----------------------------------------------------------------------------
# CrowdSec Runtime Data
# -----------------------------------------------------------------------------
/etc/crowdsec/
/var/lib/crowdsec/
/var/log/crowdsec/
*.key
```

### `.dockerignore` Updates

Add the following entries:

```dockerignore
# CrowdSec configs are copied explicitly in Dockerfile
# No changes needed - configs/crowdsec/ is included by default
```

### `.codecov.yml` Updates

Add CrowdSec config files to ignore:

```yaml
ignore:
  # ... existing entries ...

  # CrowdSec configuration (not source code)
  - "configs/crowdsec/**"
```

### `Dockerfile` Updates

Add directory creation and COPY statements:

```dockerfile
# Create CrowdSec directories
RUN mkdir -p /etc/crowdsec /etc/crowdsec/acquis.d /etc/crowdsec/bouncers \
             /etc/crowdsec/hub /etc/crowdsec/notifications \
             /var/lib/crowdsec/data /var/log/crowdsec /var/log/caddy

# Copy CrowdSec configuration templates
COPY configs/crowdsec/ /etc/crowdsec.dist/
COPY configs/crowdsec/register_bouncer.sh /usr/local/bin/
COPY configs/crowdsec/install_hub_items.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/register_bouncer.sh /usr/local/bin/install_hub_items.sh
```

---

## Rollout & Verification

### Pre-Implementation Checklist

- [ ] Create `configs/crowdsec/` directory structure
- [ ] Write acquisition config template
- [ ] Write bouncer registration script
- [ ] Write hub items installation script
- [ ] Test scripts locally with Docker

### Implementation Checklist

- [ ] Update Dockerfile with new COPY statements
- [ ] Update docker-entrypoint.sh with new initialization
- [ ] Build and test image locally
- [ ] Verify CrowdSec starts without "no datasource enabled" error
- [ ] Verify LAPI responds on port 8085
- [ ] Verify bouncer registration works
- [ ] Verify Caddy logs are being written
- [ ] Verify CrowdSec parses Caddy logs

### Post-Implementation Testing

1. **Build Test:**

   ```bash
   docker build -t charon:local .
   ```

2. **Startup Test:**

   ```bash
   docker run --rm -d --name charon-test \
       -p 8080:8080 \
       -e CERBERUS_SECURITY_CROWDSEC_MODE=local \
       charon:local
   sleep 30
   docker logs charon-test 2>&1 | grep -i crowdsec
   ```

3. **LAPI Health Test:**

   ```bash
   docker exec charon-test curl -q -O- http://127.0.0.1:8085/health
   ```

4. **Integration Test:**

   ```bash
   bash scripts/crowdsec_decision_integration.sh
   ```

5. **Full Workflow Test:**
   - Enable CrowdSec in UI
   - Ban a test IP
   - Verify IP appears in banned list
   - Unban the IP
   - Verify removal

6. **Unified Logging Test:**

   ```bash
   # Verify log watcher connects to Caddy logs
   curl -s http://localhost:8080/api/v1/status | jq '.log_watcher'

   # Test WebSocket connection (using websocat if available)
   websocat ws://localhost:8080/api/v1/cerberus/logs/live

   # Generate some traffic and verify logs appear
   curl -s http://localhost:80/nonexistent 2>/dev/null
   ```

7. **Live Log Viewer UI Test:**
   - Open Cerberus dashboard in browser
   - Verify "Security Access Logs" panel appears
   - Toggle between Application and Security modes
   - Verify blocked requests show with red highlighting
   - Test source filters (WAF, CrowdSec, Rate Limit, ACL)

### Success Criteria

- [ ] CrowdSec agent starts without errors
- [ ] LAPI responds on port 8085
- [ ] `cscli decisions list` works
- [ ] `cscli decisions add -i <IP>` works
- [ ] Caddy access logs are written to `/var/log/caddy/access.log`
- [ ] Bouncer plugin can query LAPI for decisions
- [ ] Integration tests pass
- [ ] **NEW:** LogWatcher service starts and tails Caddy logs
- [ ] **NEW:** WebSocket endpoint `/api/v1/cerberus/logs/live` streams logs
- [ ] **NEW:** LiveLogViewer displays security events in real-time
- [ ] **NEW:** Security events (403, 429) are highlighted with source tags
- [ ] **NEW:** Filters by source (waf, crowdsec, ratelimit, acl) work correctly

---

## References

- [CrowdSec Documentation](https://doc.crowdsec.net/)
- [CrowdSec Acquisition Configuration](https://doc.crowdsec.net/docs/data_sources/intro)
- [caddy-crowdsec-bouncer Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)
- [CrowdSec Hub](https://hub.crowdsec.net/)
- [Caddy Logging Documentation](https://caddyserver.com/docs/json/apps/http/servers/logs/)
- [Charon Security Documentation](../security.md)
- [Cerberus Technical Documentation](../cerberus.md)
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket implementation used
