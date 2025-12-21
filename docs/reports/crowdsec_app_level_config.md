# CrowdSec App-Level Configuration Implementation Report

**Date:** December 15, 2025
**Agent:** Backend_Dev
**Status:** ✅ **COMPLETE**

---

## Executive Summary

Successfully implemented app-level CrowdSec configuration for Caddy, moving from inline handler configuration to the proper `apps.crowdsec` section as required by the caddy-crowdsec-bouncer plugin.

**Key Changes:**

- ✅ Added `CrowdSecApp` struct to `backend/internal/caddy/types.go`
- ✅ Populated `config.Apps.CrowdSec` in `GenerateConfig` when enabled
- ✅ Simplified handler to minimal `{"handler": "crowdsec"}`
- ✅ Updated all tests to reflect new structure
- ✅ All tests pass

---

## Implementation Details

### 1. App-Level Configuration Struct

**File:** `backend/internal/caddy/types.go`

Added new `CrowdSecApp` struct:

```go
// CrowdSecApp configures the CrowdSec app module.
// Reference: https://github.com/hslatman/caddy-crowdsec-bouncer
type CrowdSecApp struct {
    APIUrl          string `json:"api_url"`
    APIKey          string `json:"api_key"`
    TickerInterval  string `json:"ticker_interval,omitempty"`
    EnableStreaming *bool  `json:"enable_streaming,omitempty"`
}
```

Updated `Apps` struct to include CrowdSec:

```go
type Apps struct {
    HTTP     *HTTPApp     `json:"http,omitempty"`
    TLS      *TLSApp      `json:"tls,omitempty"`
    CrowdSec *CrowdSecApp `json:"crowdsec,omitempty"`
}
```

### 2. Config Population

**File:** `backend/internal/caddy/config.go` in `GenerateConfig` function

When CrowdSec is enabled, populate the app-level configuration:

```go
// Configure CrowdSec app if enabled
if crowdsecEnabled {
    apiURL := "http://127.0.0.1:8085"
    if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
        apiURL = secCfg.CrowdSecAPIURL
    }
    apiKey := getCrowdSecAPIKey()
    enableStreaming := true
    config.Apps.CrowdSec = &CrowdSecApp{
        APIUrl:          apiURL,
        APIKey:          apiKey,
        TickerInterval:  "60s",
        EnableStreaming: &enableStreaming,
    }
}
```

### 3. Simplified Handler

**File:** `backend/internal/caddy/config.go` in `buildCrowdSecHandler` function

Handler is now minimal - all configuration is at app-level:

```go
func buildCrowdSecHandler(_ *models.ProxyHost, _ *models.SecurityConfig, crowdsecEnabled bool) (Handler, error) {
    if !crowdsecEnabled {
        return nil, nil
    }

    // Return minimal handler - all config is at app-level
    return Handler{"handler": "crowdsec"}, nil
}
```

### 4. Test Updates

**Files Updated:**

- `backend/internal/caddy/config_crowdsec_test.go` - All handler tests updated to expect minimal structure
- `backend/internal/caddy/config_generate_additional_test.go` - Config generation test updated to check app-level config

**Key Test Changes:**

- Handlers no longer have inline `lapi_url`, `api_key` fields
- Tests verify `config.Apps.CrowdSec` is populated correctly
- Tests verify handler is minimal `{"handler": "crowdsec"}`

---

## Configuration Structure

### Before (Inline Handler Config) ❌

```json
{
  "apps": {
    "http": {
      "servers": {
        "srv0": {
          "routes": [{
            "handle": [{
              "handler": "crowdsec",
              "lapi_url": "http://127.0.0.1:8085",
              "api_key": "xxx",
              "enable_streaming": true,
              "ticker_interval": "60s"
            }]
          }]
        }
      }
    }
  }
}
```

**Problem:** Plugin rejected inline config with "json: unknown field" errors.

### After (App-Level Config) ✅

```json
{
  "apps": {
    "crowdsec": {
      "api_url": "http://127.0.0.1:8085",
      "api_key": "xxx",
      "ticker_interval": "60s",
      "enable_streaming": true
    },
    "http": {
      "servers": {
        "srv0": {
          "routes": [{
            "handle": [{
              "handler": "crowdsec"
            }]
          }]
        }
      }
    }
  }
}
```

**Solution:** Configuration at app-level, handler references module only.

---

## Verification

### Unit Tests

All CrowdSec-related tests pass:

```bash
cd backend && go test ./internal/caddy/... -run "CrowdSec" -v
```

**Results:**

- ✅ `TestBuildCrowdSecHandler_Disabled`
- ✅ `TestBuildCrowdSecHandler_EnabledWithoutConfig`
- ✅ `TestBuildCrowdSecHandler_EnabledWithEmptyAPIURL`
- ✅ `TestBuildCrowdSecHandler_EnabledWithCustomAPIURL`
- ✅ `TestBuildCrowdSecHandler_JSONFormat`
- ✅ `TestBuildCrowdSecHandler_WithHost`
- ✅ `TestGenerateConfig_WithCrowdSec`
- ✅ `TestGenerateConfig_CrowdSecDisabled`
- ✅ `TestGenerateConfig_CrowdSecHandlerFromSecCfg`

### Build Verification

Backend compiles successfully:

```bash
cd backend && go build ./...
```

Docker image builds successfully:

```bash
docker build -t charon:local .
```

---

## Runtime Verification Steps

To verify in a running container:

### 1. Enable CrowdSec

Via Security dashboard UI:

1. Navigate to <http://localhost:8080/security>
2. Toggle "CrowdSec" ON
3. Click "Save"

### 2. Check App-Level Config

```bash
docker exec charon curl -s http://localhost:2019/config/ | jq '.apps.crowdsec'
```

**Expected Output:**

```json
{
  "api_url": "http://127.0.0.1:8085",
  "api_key": "<generated-key>",
  "ticker_interval": "60s",
  "enable_streaming": true
}
```

### 3. Check Handler is Minimal

```bash
docker exec charon curl -s http://localhost:2019/config/ | \
  jq '.apps.http.servers[].routes[].handle[] | select(.handler == "crowdsec")'
```

**Expected Output:**

```json
{
  "handler": "crowdsec"
}
```

### 4. Verify Bouncer Registration

```bash
docker exec charon cscli bouncers list
```

**Expected:** Bouncer registered with name containing "caddy"

### 5. Test Blocking

Add test ban:

```bash
docker exec charon cscli decisions add --ip 10.255.255.250 --duration 5m --reason "app-level test"
```

Test request:

```bash
curl -H "X-Forwarded-For: 10.255.255.250" http://localhost/ -v
```

**Expected:** 403 Forbidden with `X-Crowdsec-Decision` header

Cleanup:

```bash
docker exec charon cscli decisions delete --ip 10.255.255.250
```

### 6. Check Security Logs

Navigate to <http://localhost:8080/security/logs>

**Expected:** Blocked entry with:

- `source: "crowdsec"`
- `blocked: true`
- `X-Crowdsec-Decision: "ban"`

---

## Configuration Details

### API URL

Default: `http://127.0.0.1:8085`

Can be overridden via `SecurityConfig.CrowdSecAPIURL` in database.

### API Key

Read from environment variables in order:

1. `CROWDSEC_API_KEY`
2. `CROWDSEC_BOUNCER_API_KEY`
3. `CERBERUS_SECURITY_CROWDSEC_API_KEY`
4. `CHARON_SECURITY_CROWDSEC_API_KEY`
5. `CPM_SECURITY_CROWDSEC_API_KEY`

Generated automatically during CrowdSec startup via `register_bouncer.sh`.

### Ticker Interval

Default: `60s`

How often to poll for decisions when streaming is disabled.

### Enable Streaming

Default: `true`

Maintains persistent connection to LAPI for real-time decision updates (no polling delay).

---

## Architecture Benefits

### 1. Proper Plugin Integration

App-level configuration is the correct way to configure Caddy plugins that need global state. The bouncer plugin can now:

- Maintain a single LAPI connection across all routes
- Share decision cache across all virtual hosts
- Properly initialize streaming mode

### 2. Performance

Single LAPI connection instead of per-route connections:

- Reduced memory footprint
- Lower LAPI load
- Faster startup time

### 3. Maintainability

Clear separation of concerns:

- App config: Global CrowdSec settings
- Handler config: Which routes use CrowdSec (minimal reference)

### 4. Consistency

Matches other Caddy apps (HTTP, TLS) structure:

```json
{
  "apps": {
    "http": { /* HTTP app config */ },
    "tls": { /* TLS app config */ },
    "crowdsec": { /* CrowdSec app config */ }
  }
}
```

---

## Troubleshooting

### App Config Not Appearing

**Cause:** CrowdSec not enabled in SecurityConfig

**Solution:**

```bash
# Check current mode
docker exec charon curl http://localhost:8080/api/v1/admin/security/config

# Enable via UI or update database
```

### Bouncer Not Registering

**Possible Causes:**

1. LAPI not running: `docker exec charon ps aux | grep crowdsec`
2. API key missing: `docker exec charon env | grep CROWDSEC`
3. Network issue: `docker exec charon curl http://127.0.0.1:8085/health`

**Debug:**

```bash
# Check Caddy logs
docker logs charon 2>&1 | grep -i "crowdsec"

# Check LAPI logs
docker exec charon tail -f /app/data/crowdsec/log/crowdsec.log
```

### Handler Still Has Inline Config

**Cause:** Using old Docker image

**Solution:**

```bash
# Rebuild
docker build -t charon:local .

# Restart
docker-compose -f docker-compose.override.yml restart
```

---

## Files Changed

| File | Lines Changed | Description |
|------|---------------|-------------|
| [backend/internal/caddy/types.go](../../backend/internal/caddy/types.go) | +14 | Added `CrowdSecApp` struct and field to `Apps` |
| [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go) | +15, -23 | App-level config population, simplified handler |
| [backend/internal/caddy/config_crowdsec_test.go](../../backend/internal/caddy/config_crowdsec_test.go) | +~80, -~40 | Updated all handler tests |
| [backend/internal/caddy/config_generate_additional_test.go](../../backend/internal/caddy/config_generate_additional_test.go) | +~20, -~10 | Updated config generation test |
| [scripts/verify_crowdsec_app_config.sh](../../scripts/verify_crowdsec_app_config.sh) | +90 | New verification script |

---

## Related Documentation

- [Current Spec: CrowdSec Configuration Research](../plans/current_spec.md)
- [CrowdSec Bouncer Field Investigation](./crowdsec_bouncer_field_investigation.md)
- [Security Implementation Plan](../../SECURITY_IMPLEMENTATION_PLAN.md)
- [Caddy CrowdSec Bouncer Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)

---

## Success Criteria

| Criterion | Status |
|-----------|--------|
| `apps.crowdsec` populated in Caddy config | ✅ Verified in tests |
| Handler is minimal `{"handler": "crowdsec"}` | ✅ Verified in tests |
| Bouncer registered in `cscli bouncers list` | ⏳ Requires runtime verification |
| Test ban results in 403 Forbidden | ⏳ Requires runtime verification |
| Security logs show `source="crowdsec"`, `blocked=true` | ⏳ Requires runtime verification |

**Note:** Runtime verification requires CrowdSec to be enabled in SecurityConfig. Use the verification steps above to complete end-to-end testing.

---

## Next Steps

1. **Runtime Verification:**
   - Enable CrowdSec via Security dashboard
   - Run verification steps above
   - Document results in follow-up report

2. **Integration Test Update:**
   - Update `scripts/crowdsec_startup_test.sh` to verify app-level config
   - Add check for `apps.crowdsec` presence
   - Add check for minimal handler structure

3. **Documentation Update:**
   - Update [Security Docs](../../docs/security.md) with app-level config details
   - Add troubleshooting section for bouncer registration

---

**Implementation Status:** ✅ **COMPLETE**
**Runtime Verification:** ⏳ **PENDING** (requires CrowdSec enabled in SecurityConfig)
**Estimated Blocking Time:** 2-5 minutes after CrowdSec enabled (bouncer registration + first decision sync)
