# CrowdSec Hub Bootstrapping on Container Startup

## Problem Statement

After a container rebuild, CrowdSec has **zero collections installed** and a **stale hub index**. When users (or the backend) attempt to install collections, they encounter hash mismatch errors because the hub index bundled in the image at build time is outdated by the time the container runs.

The root cause is twofold:

1. `cscli hub update` in the entrypoint only runs if `.index.json` is missing — not if it is stale.
2. `install_hub_items.sh` (which does call `cscli hub update`) is gated behind `SECURITY_CROWDSEC_MODE=local`, an env var that is **deprecated** and no longer set by default. The entrypoint checks `$SECURITY_CROWDSEC_MODE`, but the backend reads `CERBERUS_SECURITY_CROWDSEC_MODE` / `CHARON_SECURITY_CROWDSEC_MODE` — a naming mismatch that means the entrypoint gate never opens.

## Current State Analysis

### Dockerfile (build-time)

| Aspect | What Happens |
|---|---|
| CrowdSec binaries | Built from source in `crowdsec-builder` stage, copied to `/usr/local/bin/{crowdsec,cscli}` |
| Config template | Source config copied to `/etc/crowdsec.dist/` |
| Hub index | **Not pre-populated** — no `cscli hub update` at build time |
| Collections | **Not installed** at build time |
| Symlink | `/etc/crowdsec` → `/app/data/crowdsec/config` created as root before `USER charon` |
| Helper scripts | `install_hub_items.sh` and `register_bouncer.sh` copied to `/usr/local/bin/` |

### Entrypoint (`.docker/docker-entrypoint.sh`, runtime)

| Step | What Happens | Problem |
|---|---|---|
| Config init | Copies `/etc/crowdsec.dist/*` → `/app/data/crowdsec/config/` on first run | Works correctly |
| Symlink verify | Confirms `/etc/crowdsec` → `/app/data/crowdsec/config` | Works correctly |
| LAPI port fix | `sed` replaces `:8080` → `:8085` in config files | Works correctly |
| Hub update (L313-315) | `cscli hub update` runs **only if** `/etc/crowdsec/hub/.index.json` does not exist | **Bug**: stale index is never refreshed |
| Machine registration | `cscli machines add -a --force` | Works correctly |
| Hub items install (L325-328) | Calls `install_hub_items.sh` **only if** `$SECURITY_CROWDSEC_MODE = "local"` | **Bug**: env var is deprecated, never set; wrong var name vs backend |
| Ownership fix | `chown -R charon:charon` on CrowdSec dirs | Works correctly |

### `install_hub_items.sh` (when invoked)

The script itself is well-structured — it calls `cscli hub update` then installs individual parsers, scenarios, and two collections (`crowdsecurity/http-cve`, `crowdsecurity/base-http-scenarios`). It does **not** install `crowdsecurity/caddy`.

### Backend (`crowdsec_startup.go`)

`ReconcileCrowdSecOnStartup()` checks the database for CrowdSec mode and starts the process if needed. It does **not** call `cscli hub update` or install collections. The `HubService.runCSCLI()` method in `hub_sync.go` does call `cscli hub update` before individual item installs, but this is only triggered by explicit GUI actions (Pull/Apply), not at startup.

## Proposed Changes

### File: `.docker/docker-entrypoint.sh`

**Change 1: Always refresh the hub index**

Replace the conditional hub update (current L313-315):

```sh
# CURRENT (broken):
if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
    echo "Updating CrowdSec hub index..."
    timeout 60s cscli hub update 2>/dev/null || echo "⚠️ Hub update timed out or failed, continuing..."
fi
```

With an unconditional update:

```sh
# NEW: Always refresh hub index on startup (stale index causes hash mismatch errors)
echo "Updating CrowdSec hub index..."
if ! timeout 60s cscli hub update 2>&1; then
    echo "⚠️ Hub index update failed (network issue?). Collections may fail to install."
    echo "   CrowdSec will still start with whatever index is cached."
fi
```

**Change 2: Always install required collections (remove env var gate)**

Replace the conditional hub items install (current L322-328):

```sh
# CURRENT (broken — env var never set):
if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "Installing CrowdSec hub items..."
    if [ -x /usr/local/bin/install_hub_items.sh ]; then
        /usr/local/bin/install_hub_items.sh 2>/dev/null || echo "Warning: Some hub items may not have installed"
    fi
fi
```

With unconditional execution:

```sh
# NEW: Always ensure required collections are present.
# This is idempotent — already-installed items are skipped by cscli.
# Collections are needed regardless of whether CrowdSec is GUI-enabled,
# because the user can enable CrowdSec at any time via the dashboard
# and expects it to work immediately.
echo "Ensuring CrowdSec hub items are installed..."
if [ -x /usr/local/bin/install_hub_items.sh ]; then
    /usr/local/bin/install_hub_items.sh || echo "⚠️ Some hub items may not have installed. CrowdSec can still start."
fi
```

### File: `configs/crowdsec/install_hub_items.sh`

**Change 3: Add `crowdsecurity/caddy` collection**

Add after the existing `crowdsecurity/base-http-scenarios` install:

```sh
# Install Caddy collection (parser + scenarios for Caddy access logs)
echo "Installing Caddy collection..."
cscli collections install crowdsecurity/caddy --force 2>/dev/null || true
```

**Change 4: Remove redundant individual parser installs that are included in collections**

The `crowdsecurity/base-http-scenarios` collection already includes `crowdsecurity/http-logs` and several of the individually installed scenarios. The `crowdsecurity/caddy` collection includes `crowdsecurity/caddy-logs`. Keep the individual installs as fallbacks (they are idempotent), but add a comment noting the overlap. No lines need deletion — the `--force` flag and idempotency make this safe.

### Summary of File Changes

| File | Change | Lines |
|---|---|---|
| `.docker/docker-entrypoint.sh` | Unconditional `cscli hub update` | ~L313-315 |
| `.docker/docker-entrypoint.sh` | Remove `SECURITY_CROWDSEC_MODE` gate on hub items install | ~L322-328 |
| `configs/crowdsec/install_hub_items.sh` | Add `crowdsecurity/caddy` collection install | After L60 |

## Edge Cases

| Scenario | Handling |
|---|---|
| **No network at startup** | `cscli hub update` fails with timeout. The `install_hub_items.sh` also fails. Entrypoint continues — CrowdSec starts with whatever is cached (or no collections). User can retry via GUI. |
| **Hub CDN returns 5xx** | Same as no network — timeout + fallback. |
| **Collections already installed** | `--force` flag makes `cscli collections install` idempotent. It updates to latest if newer version available. |
| **First boot (no prior data volume)** | Config gets copied from `.dist`, hub update runs, collections install. Clean bootstrap path. |
| **Existing data volume (upgrade)** | Config already exists (skips copy), hub update refreshes stale index, collections install/upgrade. |
| **`install_hub_items.sh` missing/not executable** | `-x` check in entrypoint skips it with a log message. CrowdSec starts without collections. |
| **CrowdSec disabled in GUI** | Collections are still installed (they are just config files). No process runs until user enables via GUI. Zero runtime cost. |

## Startup Time Impact

- `cscli hub update`: ~2-5s (single HTTPS request to hub CDN)
- `install_hub_items.sh`: ~10-15s (multiple `cscli` invocations, each checking/installing)
- Total additional startup time: **~12-20s** (first boot) / **~5-10s** (subsequent boots, items cached)

This is acceptable for a container that runs long-lived.

## Acceptance Criteria

1. After `docker compose up` with a fresh data volume, `cscli collections list` shows `crowdsecurity/caddy`, `crowdsecurity/base-http-scenarios`, and `crowdsecurity/http-cve` installed.
2. After `docker compose up` with an existing data volume (stale index), hub index is refreshed and collections remain installed.
3. If the container starts with no network, CrowdSec initialization logs warnings but does not crash or block startup.
4. No env var (`SECURITY_CROWDSEC_MODE`) is required for collections to be installed.
5. Startup time increase is < 30 seconds.

## Commit Slicing Strategy

**Decision**: Single PR. Scope is small (3 files, ~15 lines changed), low risk, and all changes are tightly coupled.

**PR-1**: CrowdSec hub bootstrapping fix
- **Scope**: `.docker/docker-entrypoint.sh`, `configs/crowdsec/install_hub_items.sh`
- **Validation**: Manual docker rebuild + verify collections with `cscli collections list`
- **Rollback**: Revert PR; behavior returns to current (broken) state — no data loss risk
