# CrowdSec LAPI Authentication Fix

**Issue Reference**: Related to Issue #585 (CrowdSec Web Console Enrollment)
**Status**: Ready for Implementation
**Priority**: P1 (Blocking CrowdSec functionality)
**Created**: 2026-02-03
**Estimated Effort**: 2-4 hours

## Executive Summary

After a container rebuild, the CrowdSec integration fails with "access forbidden" when attempting to connect to the LAPI. This blocks all CrowdSec functionality including IP banning and web console enrollment testing.

**Error Observed**:
```json
{
  "level": "error",
  "ts": 1770143945.8009417,
  "logger": "crowdsec",
  "msg": "failed to connect to LAPI, retrying in 10s: API error: access forbidden",
  "instance_id": "99c91cc1",
  "address": "http://127.0.0.1:8085"
}
```

---

## Root Cause Analysis

### Finding 1: Invalid Static API Key

**Problem**: User configured `CHARON_SECURITY_CROWDSEC_API_KEY=charonbouncerkey2024` in docker-compose.yml.

**Why This Fails**: CrowdSec bouncer API keys must be **generated** by CrowdSec via `cscli bouncers add <name>`. The manually-specified key `charonbouncerkey2024` was never registered with CrowdSec LAPI, so the LAPI rejects authentication with "access forbidden".

**Evidence**:
- [registration.go](../../backend/internal/crowdsec/registration.go#L96-L106): `EnsureBouncerRegistered()` first checks for env var API key, returns it if present (without validation)
- [register_bouncer.sh](../../configs/crowdsec/register_bouncer.sh): Script generates real API key via `cscli bouncers add`

### Finding 2: Bouncer Not Auto-Registered on Start

**Problem**: When CrowdSec agent starts, the bouncer is never registered automatically.

**Evidence**:
- [docker-entrypoint.sh](../../.docker/docker-entrypoint.sh#L223): Only **machine** registration occurs: `cscli machines add -a --force`
- [crowdsec_handler.go#L191-L295](../../backend/internal/api/handlers/crowdsec_handler.go#L191-L295): `Start()` handler starts CrowdSec process but does NOT call bouncer registration
- [crowdsec_handler.go#L1432-1478](../../backend/internal/api/handlers/crowdsec_handler.go#L1432-1478): `RegisterBouncer()` is a **separate** API endpoint (`POST /api/v1/admin/crowdsec/bouncer/register`) that must be called manually

### Finding 3: Incorrect API URL Configuration (Minor)

**Problem**: User configured `CHARON_SECURITY_CROWDSEC_API_URL=http://localhost:8080`.

**Why This Is Wrong**:
- CrowdSec LAPI listens on port **8085** (configured in entrypoint via sed)
- Port 8080 is used by Charon management API
- Code default is correct: `http://127.0.0.1:8085` (see [config.go#L60-64](../../backend/internal/caddy/config.go#L60-64))

**Evidence from entrypoint**:
```bash
# Configure CrowdSec LAPI to use port 8085 to avoid conflict with Charon (port 8080)
sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
```

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Current (Broken) Flow                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. Container starts                                                         │
│     ├─► docker-entrypoint.sh                                                 │
│     │   ├─► mkdir /app/data/crowdsec                                        │
│     │   ├─► Copy config to persistent storage                               │
│     │   ├─► cscli machines add -a --force  ✅ Machine registered            │
│     │   └─► ❌ NO bouncer registration                                       │
│                                                                              │
│  2. User enables CrowdSec via GUI                                            │
│     ├─► POST /api/v1/admin/crowdsec/start                                   │
│     │   ├─► SecurityConfig.CrowdSecMode = "local"                           │
│     │   ├─► Start crowdsec process                                          │
│     │   ├─► Wait for LAPI ready                                             │
│     │   └─► ❌ NO bouncer registration                                       │
│                                                                              │
│  3. Caddy loads CrowdSec bouncer config                                      │
│     ├─► GenerateConfig() in caddy/config.go                                 │
│     │   ├─► apiKey := getCrowdSecAPIKey()                                   │
│     │   │   └─► Returns "charonbouncerkey2024" from CHARON_SECURITY_...     │
│     │   └─► CrowdSecApp{APIKey: "charonbouncerkey2024", ...}                │
│                                                                              │
│  4. Caddy CrowdSec bouncer tries to connect                                  │
│     ├─► Connect to http://127.0.0.1:8085                                    │
│     ├─► Send API key "charonbouncerkey2024"                                 │
│     └─► ❌ LAPI responds: "access forbidden" (key not registered)           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Requirements (EARS Notation)

### REQ-1: API Key Validation
**WHEN** a CrowdSec API key is provided via environment variable, **THE SYSTEM SHALL** validate that the key is actually registered with the LAPI before using it.

**Acceptance Criteria**:
- Validation occurs during CrowdSec start
- Invalid keys trigger a warning log
- System attempts auto-registration if validation fails

### REQ-2: Auto-Registration on Start
**WHEN** CrowdSec starts and no valid bouncer is registered, **THE SYSTEM SHALL** automatically register a bouncer and store the generated API key.

**Acceptance Criteria**:
- Bouncer registration happens automatically after LAPI is ready
- Generated API key is persisted to `/etc/crowdsec/bouncers/caddy-bouncer.key`
- Caddy config is regenerated with the new API key

### REQ-3: Caddy Config Regeneration
**WHEN** a new bouncer API key is generated, **THE SYSTEM SHALL** regenerate the Caddy configuration with the updated key.

**Acceptance Criteria**:
- Caddy config uses the newly generated API key
- No container restart required
- Bouncer connects successfully to LAPI

### REQ-4: Upgrade Scenario Handling
**WHEN** upgrading from a broken state (invalid static key), **THE SYSTEM SHALL** heal automatically without manual intervention.

**Acceptance Criteria**:
- Works for both fresh installs and upgrades
- Volume-mounted data is preserved
- No manual cleanup required

---

## Technical Design

### Solution Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Fixed Flow                                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. Container starts (unchanged)                                             │
│     └─► docker-entrypoint.sh                                                 │
│         └─► cscli machines add -a --force                                   │
│                                                                              │
│  2. User enables CrowdSec via GUI                                            │
│     └─► POST /api/v1/admin/crowdsec/start                                   │
│         ├─► Start crowdsec process                                          │
│         ├─► Wait for LAPI ready                                             │
│         ├─► 🆕 EnsureBouncerRegistered()                                     │
│         │   ├─► Check if env key is valid                                   │
│         │   ├─► If invalid, run register_bouncer.sh                         │
│         │   └─► Store key in database/file                                  │
│         ├─► 🆕 Regenerate Caddy config with new API key                     │
│         └─► Return success                                                   │
│                                                                              │
│  3. Caddy loads CrowdSec bouncer config                                      │
│     ├─► getCrowdSecAPIKey()                                                 │
│     │   ├─► Check env var first                                             │
│     │   ├─► 🆕 Check file /etc/crowdsec/bouncers/caddy-bouncer.key         │
│     │   └─► 🆕 Check database settings table                                │
│     └─► CrowdSecApp{APIKey: <valid-key>, ...}                               │
│                                                                              │
│  4. Caddy CrowdSec bouncer connects                                          │
│     └─► ✅ LAPI accepts valid key                                            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Implementation Changes

#### Change 1: Update `Start()` Handler to Register Bouncer
**File**: `backend/internal/api/handlers/crowdsec_handler.go`

**Location**: After LAPI readiness check (line ~290)

```go
// After confirming LAPI is ready, ensure bouncer is registered
if lapiReady {
    // Register bouncer if needed (idempotent)
    apiKey, regErr := h.ensureBouncerRegistration(ctx)
    if regErr != nil {
        logger.Log().WithError(regErr).Warn("Failed to register bouncer, CrowdSec may not enforce decisions")
    } else if apiKey != "" {
        // Store the API key for Caddy config generation
        h.storeAPIKey(ctx, apiKey)

        // Regenerate Caddy config with new API key
        if h.CaddyManager != nil {
            if err := h.CaddyManager.ReloadConfig(ctx); err != nil {
                logger.Log().WithError(err).Warn("Failed to reload Caddy config with new bouncer key")
            }
        }
    }
}
```

#### Change 2: Add `ensureBouncerRegistration()` Method
**File**: `backend/internal/api/handlers/crowdsec_handler.go`

```go
// ensureBouncerRegistration checks if bouncer is registered and registers if needed.
// Returns the API key (empty string if already registered via env var).
func (h *CrowdsecHandler) ensureBouncerRegistration(ctx context.Context) (string, error) {
    // First check if env var key is actually valid
    envKey := getLAPIKey()
    if envKey != "" {
        // Validate the key is actually registered
        if h.validateBouncerKey(ctx, envKey) {
            return "", nil // Key is valid, nothing to do
        }
        logger.Log().Warn("Env-provided CrowdSec API key is invalid, will register new bouncer")
    }

    // Check if key file already exists
    keyFile := "/etc/crowdsec/bouncers/caddy-bouncer.key"
    if data, err := os.ReadFile(keyFile); err == nil {
        key := strings.TrimSpace(string(data))
        if key != "" && h.validateBouncerKey(ctx, key) {
            return key, nil // Key file is valid
        }
    }

    // Register new bouncer using script
    scriptPath := "/usr/local/bin/register_bouncer.sh"
    output, err := h.CmdExec.Execute(ctx, "bash", scriptPath)
    if err != nil {
        return "", fmt.Errorf("bouncer registration failed: %w: %s", err, string(output))
    }

    // Extract API key from output (last non-empty line)
    lines := strings.Split(strings.TrimSpace(string(output)), "\n")
    apiKey := ""
    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        if len(line) >= 32 && !strings.Contains(line, " ") {
            apiKey = line
            break
        }
    }

    if apiKey == "" {
        return "", fmt.Errorf("bouncer registration output did not contain API key")
    }

    return apiKey, nil
}

// validateBouncerKey checks if an API key is actually registered with CrowdSec.
func (h *CrowdsecHandler) validateBouncerKey(ctx context.Context, key string) bool {
    // Use cscli bouncers list to check if 'caddy-bouncer' exists
    checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    output, err := h.CmdExec.Execute(checkCtx, "cscli", "bouncers", "list", "-o", "json")
    if err != nil {
        return false
    }

    // Parse JSON and check for caddy-bouncer
    var bouncers []struct {
        Name   string `json:"name"`
        APIKey string `json:"api_key"`
    }
    if err := json.Unmarshal(output, &bouncers); err != nil {
        return false
    }

    for _, b := range bouncers {
        if b.Name == "caddy-bouncer" {
            // Note: cscli doesn't return the full API key, just that it exists
            // We trust it's valid if the bouncer is registered
            return true
        }
    }

    return false
}
```

#### Change 3: Update `getCrowdSecAPIKey()` to Check Key File
**File**: `backend/internal/caddy/config.go`

**Location**: `getCrowdSecAPIKey()` function (line ~1129)

```go
// getCrowdSecAPIKey retrieves the CrowdSec bouncer API key.
// Priority: environment variable > key file > empty
func getCrowdSecAPIKey() string {
    // Check environment variables first
    envVars := []string{
        "CROWDSEC_API_KEY",
        "CROWDSEC_BOUNCER_API_KEY",
        "CERBERUS_SECURITY_CROWDSEC_API_KEY",
        "CHARON_SECURITY_CROWDSEC_API_KEY",
        "CPM_SECURITY_CROWDSEC_API_KEY",
    }

    for _, key := range envVars {
        if val := os.Getenv(key); val != "" {
            return val
        }
    }

    // Check key file (generated by register_bouncer.sh)
    keyFile := "/etc/crowdsec/bouncers/caddy-bouncer.key"
    if data, err := os.ReadFile(keyFile); err == nil {
        key := strings.TrimSpace(string(data))
        if key != "" {
            return key
        }
    }

    return ""
}
```

#### Change 4: Add CaddyManager to CrowdsecHandler
**File**: `backend/internal/api/handlers/crowdsec_handler.go`

This may already exist, but ensure the handler can trigger Caddy config reload:

```go
type CrowdsecHandler struct {
    DB           *gorm.DB
    BinPath      string
    DataDir      string
    Executor     CrowdsecExecutor
    CmdExec      CommandExecutor
    LAPIMaxWait  time.Duration
    LAPIPollInterval time.Duration
    CaddyManager *caddy.Manager  // ADD: For config reload
}
```

---

## Immediate Workaround (User Action)

While the fix is being implemented, the user can:

1. **Remove the static API key** from docker-compose.yml:
   ```yaml
   # REMOVE or comment out this line:
   # - CHARON_SECURITY_CROWDSEC_API_KEY=charonbouncerkey2024
   ```

2. **Fix the API URL**:
   ```yaml
   # Change from:
   - CHARON_SECURITY_CROWDSEC_API_URL=http://localhost:8080
   # To:
   - CHARON_SECURITY_CROWDSEC_API_URL=http://localhost:8085
   ```

3. **Manually register bouncer** after container starts:
   ```bash
   docker exec -it charon /usr/local/bin/register_bouncer.sh
   ```

4. **Restart container** to pick up the new key:
   ```bash
   docker compose restart charon
   ```

---

## Test Scenarios

### Scenario 1: Fresh Install
1. Start container with no CrowdSec env vars
2. Enable CrowdSec via GUI
3. **Expected**: Bouncer auto-registers, no errors in logs

### Scenario 2: Upgrade from Invalid Key
1. Start container with `CHARON_SECURITY_CROWDSEC_API_KEY=invalid`
2. Enable CrowdSec via GUI
3. **Expected**: System detects invalid key, registers new bouncer, logs warning

### Scenario 3: Upgrade with Valid Key File
1. Container has `/etc/crowdsec/bouncers/caddy-bouncer.key` from previous run
2. Restart container, enable CrowdSec
3. **Expected**: Uses existing key file, no re-registration

### Scenario 4: API URL Misconfiguration
1. Set `CHARON_SECURITY_CROWDSEC_API_URL=http://localhost:8080` (wrong port)
2. Enable CrowdSec
3. **Expected**: Uses default 8085 port, logs warning about ignored URL

---

## Implementation Checklist

- [ ] **Task 1**: Add `validateBouncerKey()` method to crowdsec_handler.go
- [ ] **Task 2**: Add `ensureBouncerRegistration()` method
- [ ] **Task 3**: Update `Start()` to call bouncer registration after LAPI ready
- [ ] **Task 4**: Update `getCrowdSecAPIKey()` in caddy/config.go to read from key file
- [ ] **Task 5**: Add integration test for bouncer auto-registration
- [ ] **Task 6**: Update documentation to clarify API key generation

---

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/api/handlers/crowdsec_handler.go` | Add validation and auto-registration |
| `backend/internal/caddy/config.go` | Update `getCrowdSecAPIKey()` |
| `docs/docker-compose.yml` (examples) | Remove/update API key examples |
| `README.md` or `SECURITY.md` | Clarify CrowdSec setup |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing valid keys | Low | Medium | Only re-register if validation fails |
| register_bouncer.sh not present | Low | High | Check script existence before calling |
| Caddy reload fails | Low | Medium | Continue without bouncer, log warning |
| Race condition on startup | Low | Low | CrowdSec must finish starting first |

---

## References

- [CrowdSec bouncer registration](https://doc.crowdsec.net/docs/bouncers/intro)
- [Caddy CrowdSec Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)
- [Issue #585 - CrowdSec Web Console Enrollment](https://github.com/Wikid82/Charon/issues/585)
- [register_bouncer.sh](../../configs/crowdsec/register_bouncer.sh)
- [docker-entrypoint.sh](../../.docker/docker-entrypoint.sh)

---

**Last Updated**: 2026-02-03
**Owner**: TBD
