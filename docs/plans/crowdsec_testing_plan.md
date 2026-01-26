# CrowdSec Testing Plan - Issue #319

## Summary of CrowdSec Implementation

### Architecture Overview

CrowdSec in Charon is managed through a combination of:

1. **Process Management** (`crowdsec_exec.go`): CrowdSec runs as a subprocess managed by Charon
   - Uses PID file (`crowdsec.pid`) in the data directory for process tracking
   - Start/Stop/Status operations via `CrowdsecExecutor` interface
   - Binary path configurable, defaults to `crowdsec`

2. **LAPI Communication**: Charon communicates with CrowdSec Local API for decisions
   - Default LAPI URL: `http://127.0.0.1:8085` (port 8085 to avoid conflict with Charon on port 8080)
   - Configurable via `CROWDSEC_API_KEY` or similar env vars
   - Falls back to `cscli` commands when LAPI unavailable

3. **CLI Integration**: Uses `cscli` for decision management (ban/unban IPs)
   - `cscli decisions list -o json` - List current bans
   - `cscli decisions add -i <IP> -d <duration> -R <reason> -t ban` - Ban IP
   - `cscli decisions delete -i <IP>` - Unban IP

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CERBERUS_SECURITY_CROWDSEC_MODE` | Mode: `local` or `disabled` | `disabled` |
| `CERBERUS_SECURITY_CROWDSEC_API_URL` | LAPI endpoint URL | (empty) |
| `CERBERUS_SECURITY_CROWDSEC_API_KEY` | API key for LAPI | (empty) |
| `CHARON_CROWDSEC_CONFIG_DIR` | Data directory | `data/crowdsec` |
| `CROWDSEC_API_KEY` | Bouncer API key | (empty) |
| `FEATURE_CERBERUS_ENABLED` | Enable Cerberus suite | `true` |

### Data Directory Structure

```
data/crowdsec/
├── config.yaml          # CrowdSec configuration
├── crowdsec.pid         # Process ID file (when running)
├── hub_cache/           # Cached presets from CrowdSec Hub
└── *.backup.*           # Automatic backups before changes
```

---

## API Endpoints

All endpoints are under `/api/v1/admin/crowdsec/` and require authentication.

### Process Management

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/status` | GET | Get CrowdSec running state | `{"running": bool, "pid": int}` |
| `/start` | POST | Start CrowdSec process | `{"status": "started", "pid": int}` |
| `/stop` | POST | Stop CrowdSec process | `{"status": "stopped"}` |

### Decision Management (Banned IPs)

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/decisions` | GET | List all banned IPs via cscli | `{"decisions": [...], "total": int}` |
| `/decisions/lapi` | GET | List decisions via LAPI (preferred) | `{"decisions": [...], "total": int, "source": "lapi"}` |
| `/lapi/health` | GET | Check LAPI health | `{"healthy": bool, "lapi_url": str}` |
| `/ban` | POST | Ban an IP address | `{"status": "banned", "ip": str, "duration": str}` |
| `/ban/:ip` | DELETE | Unban an IP address | `{"status": "unbanned", "ip": str}` |

### Configuration Management

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/import` | POST | Import config (tar.gz/zip upload) | `{"status": "imported", "backup": str}` |
| `/export` | GET | Export config as tar.gz | Binary (application/gzip) |
| `/files` | GET | List config files | `{"files": [str]}` |
| `/file` | GET | Read config file (query: `path`) | `{"content": str}` |
| `/file` | POST | Write config file | `{"status": "written", "backup": str}` |

### Preset Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/presets` | GET | List available presets |
| `/presets/pull` | POST | Pull preset preview from Hub |
| `/presets/apply` | POST | Apply preset with backup |
| `/presets/cache/:slug` | GET | Get cached preset |

---

## Test Cases

### TC-1: Start CrowdSec

**Objective:** Verify CrowdSec can be started via the Security dashboard

**Prerequisites:**

- Charon running with `FEATURE_CERBERUS_ENABLED=true`
- CrowdSec binary available in container

**Steps:**

1. Navigate to Security Dashboard (`/security`)
2. Locate CrowdSec status card
3. Click "Start" button
4. Observe loading animation

**Expected Results:**

- API returns `{"status": "started", "pid": <number>}`
- Status changes to "Running"
- PID file created at `data/crowdsec/crowdsec.pid`

**Curl Command:**

```bash
curl -X POST -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/start
```

**Expected Response:**

```json
{"status": "started", "pid": 12345}
```

---

### TC-2: Verify Status

**Objective:** Verify CrowdSec status is correctly reported

**Steps:**

1. After TC-1, check status endpoint
2. Verify UI shows "Running" badge

**Curl Command:**

```bash
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/status
```

**Expected Response (when running):**

```json
{"running": true, "pid": 12345}
```

**Expected Response (when stopped):**

```json
{"running": false, "pid": 0}
```

---

### TC-3: View Banned IPs

**Objective:** Verify banned IPs table displays correctly

**Steps:**

1. Navigate to `/security/crowdsec`
2. Scroll to "Banned IPs" section
3. Verify table columns: IP, Reason, Duration, Banned At, Source, Actions

**Curl Command (via cscli):**

```bash
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/decisions
```

**Curl Command (via LAPI - preferred):**

```bash
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/decisions/lapi
```

**Expected Response (empty):**

```json
{"decisions": [], "total": 0}
```

**Expected Response (with bans):**

```json
{
  "decisions": [
    {
      "id": 1,
      "origin": "cscli",
      "type": "ban",
      "scope": "ip",
      "value": "192.168.100.100",
      "duration": "1h",
      "scenario": "manual ban: test",
      "created_at": "2024-12-12T10:00:00Z",
      "until": "2024-12-12T11:00:00Z"
    }
  ],
  "total": 1
}
```

---

### TC-4: Manual Ban IP

**Objective:** Ban a test IP address with custom duration

**Test Data:**

- IP: `192.168.100.100`
- Duration: `1h`
- Reason: `Integration test ban`

**Steps:**

1. Navigate to `/security/crowdsec`
2. Click "Ban IP" button
3. Enter IP: `192.168.100.100`
4. Select duration: "1 hour"
5. Enter reason: "Integration test ban"
6. Click "Ban IP"

**Curl Command:**

```bash
curl -X POST -b "$COOKIE_FILE" \
  -H "Content-Type: application/json" \
  -d '{"ip": "192.168.100.100", "duration": "1h", "reason": "Integration test ban"}' \
  http://localhost:8080/api/v1/admin/crowdsec/ban
```

**Expected Response:**

```json
{"status": "banned", "ip": "192.168.100.100", "duration": "1h"}
```

**Validation:**

```bash
# Verify via decisions list
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/decisions | jq '.decisions[] | select(.value == "192.168.100.100")'
```

---

### TC-5: Verify Ban in Table

**Objective:** Confirm banned IP appears in the UI table

**Steps:**

1. After TC-4, refresh the page or observe real-time update
2. Verify table shows the new ban entry
3. Check columns display correct data

**Expected Table Row:**

| IP | Reason | Duration | Banned At | Source | Actions |
|----|--------|----------|-----------|--------|---------|
| 192.168.100.100 | manual ban: Integration test ban | 1h | (timestamp) | manual | [Unban] |

---

### TC-6: Manual Unban IP

**Objective:** Remove ban from test IP

**Steps:**

1. In Banned IPs table, find `192.168.100.100`
2. Click "Unban" button
3. Confirm in modal dialog
4. Observe IP removed from table

**Curl Command:**

```bash
curl -X DELETE -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/ban/192.168.100.100
```

**Expected Response:**

```json
{"status": "unbanned", "ip": "192.168.100.100"}
```

---

### TC-7: Verify IP Removal

**Objective:** Confirm IP no longer appears in banned list

**Steps:**

1. After TC-6, verify table no longer shows the IP
2. Query decisions endpoint to confirm

**Curl Command:**

```bash
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/decisions
```

**Expected Response:**

- IP `192.168.100.100` not present in decisions array

---

### TC-8: Export Configuration

**Objective:** Export CrowdSec configuration as tar.gz

**Steps:**

1. Navigate to `/security/crowdsec`
2. Click "Export" button
3. Verify file downloads with timestamp filename

**Curl Command:**

```bash
curl -b "$COOKIE_FILE" -o crowdsec-export.tar.gz \
  http://localhost:8080/api/v1/admin/crowdsec/export
```

**Expected Response:**

- HTTP 200 with `Content-Type: application/gzip`
- `Content-Disposition: attachment; filename=crowdsec-config-YYYYMMDD-HHMMSS.tar.gz`
- Valid tar.gz archive containing config files

**Validation:**

```bash
tar -tzf crowdsec-export.tar.gz
# Should list config files
```

---

### TC-9: Import Configuration

**Objective:** Import a CrowdSec configuration package

**Prerequisites:**

- Export file from TC-8 or test config archive

**Steps:**

1. Navigate to `/security/crowdsec`
2. Select file for import
3. Click "Import" button
4. Verify backup created and config applied

**Curl Command:**

```bash
curl -X POST -b "$COOKIE_FILE" \
  -F "file=@crowdsec-export.tar.gz" \
  http://localhost:8080/api/v1/admin/crowdsec/import
```

**Expected Response:**

```json
{"status": "imported", "backup": "data/crowdsec.backup.YYYYMMDD-HHMMSS"}
```

---

### TC-10: LAPI Health Check

**Objective:** Verify LAPI connectivity status

**Curl Command:**

```bash
curl -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/lapi/health
```

**Expected Response (healthy):**

```json
{"healthy": true, "lapi_url": "http://127.0.0.1:8085", "status": 200}
```

**Expected Response (unhealthy):**

```json
{"healthy": false, "error": "LAPI unreachable", "lapi_url": "http://127.0.0.1:8085"}
```

---

### TC-11: Stop CrowdSec

**Objective:** Verify CrowdSec can be stopped

**Steps:**

1. With CrowdSec running, click "Stop" button
2. Verify status changes to "Stopped"

**Curl Command:**

```bash
curl -X POST -b "$COOKIE_FILE" \
  http://localhost:8080/api/v1/admin/crowdsec/stop
```

**Expected Response:**

```json
{"status": "stopped"}
```

**Validation:**

- PID file removed from `data/crowdsec/`
- Status endpoint returns `{"running": false, "pid": 0}`

---

## Integration Test Script Requirements

### Script Location

`scripts/crowdsec_decision_integration.sh`

### Script Outline

```bash
#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Configuration
BASE_URL="http://localhost:8080/api/v1"
TEST_IP="192.168.100.100"
TEST_DURATION="1h"
TEST_REASON="Integration test ban"

# Error handler
trap 'log_error "Error occurred at line $LINENO"; cleanup' ERR

cleanup() {
    log_info "Cleaning up..."
    docker rm -f charon-crowdsec-test >/dev/null 2>&1 || true
    rm -f "$COOKIE_FILE" 2>/dev/null || true
}

# Build and start container
build_container() {
    log_info "Building charon:local image..."
    docker build -t charon:local .

    docker rm -f charon-crowdsec-test >/dev/null 2>&1 || true

    log_info "Starting container..."
    docker run -d --name charon-crowdsec-test \
        -p 8080:8080 \
        -e CHARON_ENV=development \
        -e FEATURE_CERBERUS_ENABLED=true \
        charon:local
}

# Wait for API
wait_for_api() {
    log_info "Waiting for API..."
    for i in {1..30}; do
        if curl -sf "$BASE_URL/" >/dev/null 2>&1; then
            log_info "API ready"
            return 0
        fi
        sleep 1
    done
    log_error "API failed to start"
    exit 1
}

# Authenticate
authenticate() {
    COOKIE_FILE=$(mktemp)
    log_info "Registering and logging in..."

    curl -sf -X POST -H "Content-Type: application/json" \
        -d '{"email":"test@example.local","password":"password123","name":"Test User"}' \
        "$BASE_URL/auth/register" >/dev/null || true

    curl -sf -X POST -H "Content-Type: application/json" \
        -d '{"email":"test@example.local","password":"password123"}' \
        -c "$COOKIE_FILE" "$BASE_URL/auth/login" >/dev/null
}

# Test: Get Status
test_status() {
    log_info "TC-2: Testing status endpoint..."
    RESP=$(curl -sf -b "$COOKIE_FILE" "$BASE_URL/admin/crowdsec/status")

    if echo "$RESP" | jq -e '.running != null' >/dev/null; then
        log_info "  Status: $(echo $RESP | jq -c)"
        return 0
    fi
    log_error "Status check failed"
    return 1
}

# Test: List Decisions (empty)
test_list_decisions_empty() {
    log_info "TC-3: Testing decisions list (expect empty)..."
    RESP=$(curl -sf -b "$COOKIE_FILE" "$BASE_URL/admin/crowdsec/decisions")

    TOTAL=$(echo "$RESP" | jq -r '.total // 0')
    if [ "$TOTAL" -eq 0 ]; then
        log_info "  Decisions list empty as expected"
        return 0
    fi
    log_warn "  Found $TOTAL existing decisions"
    return 0
}

# Test: Ban IP
test_ban_ip() {
    log_info "TC-4: Testing ban IP..."
    RESP=$(curl -sf -X POST -b "$COOKIE_FILE" \
        -H "Content-Type: application/json" \
        -d "{\"ip\": \"$TEST_IP\", \"duration\": \"$TEST_DURATION\", \"reason\": \"$TEST_REASON\"}" \
        "$BASE_URL/admin/crowdsec/ban")

    STATUS=$(echo "$RESP" | jq -r '.status')
    if [ "$STATUS" = "banned" ]; then
        log_info "  Ban successful: $(echo $RESP | jq -c)"
        return 0
    fi
    log_error "Ban failed: $RESP"
    return 1
}

# Test: Verify Ban
test_verify_ban() {
    log_info "TC-5: Verifying ban in decisions..."
    RESP=$(curl -sf -b "$COOKIE_FILE" "$BASE_URL/admin/crowdsec/decisions")

    FOUND=$(echo "$RESP" | jq -r ".decisions[] | select(.value == \"$TEST_IP\") | .value")
    if [ "$FOUND" = "$TEST_IP" ]; then
        log_info "  Ban verified in decisions list"
        return 0
    fi
    log_error "Ban not found in decisions"
    return 1
}

# Test: Unban IP
test_unban_ip() {
    log_info "TC-6: Testing unban IP..."
    RESP=$(curl -sf -X DELETE -b "$COOKIE_FILE" \
        "$BASE_URL/admin/crowdsec/ban/$TEST_IP")

    STATUS=$(echo "$RESP" | jq -r '.status')
    if [ "$STATUS" = "unbanned" ]; then
        log_info "  Unban successful: $(echo $RESP | jq -c)"
        return 0
    fi
    log_error "Unban failed: $RESP"
    return 1
}

# Test: Verify Removal
test_verify_removal() {
    log_info "TC-7: Verifying IP removal..."
    RESP=$(curl -sf -b "$COOKIE_FILE" "$BASE_URL/admin/crowdsec/decisions")

    FOUND=$(echo "$RESP" | jq -r ".decisions[] | select(.value == \"$TEST_IP\") | .value")
    if [ -z "$FOUND" ]; then
        log_info "  IP successfully removed from decisions"
        return 0
    fi
    log_error "IP still present in decisions"
    return 1
}

# Test: Export Config
test_export() {
    log_info "TC-8: Testing export..."
    EXPORT_FILE=$(mktemp --suffix=.tar.gz)

    HTTP_CODE=$(curl -sf -b "$COOKIE_FILE" -o "$EXPORT_FILE" -w "%{http_code}" \
        "$BASE_URL/admin/crowdsec/export")

    if [ "$HTTP_CODE" = "200" ] && [ -s "$EXPORT_FILE" ]; then
        log_info "  Export successful: $(ls -lh $EXPORT_FILE | awk '{print $5}')"
        rm -f "$EXPORT_FILE"
        return 0
    fi
    log_error "Export failed (HTTP $HTTP_CODE)"
    rm -f "$EXPORT_FILE"
    return 1
}

# Test: LAPI Health
test_lapi_health() {
    log_info "TC-10: Testing LAPI health..."
    RESP=$(curl -sf -b "$COOKIE_FILE" "$BASE_URL/admin/crowdsec/lapi/health" || echo '{"healthy":false}')

    log_info "  LAPI Health: $(echo $RESP | jq -c)"
    return 0
}

# Main
main() {
    log_info "=== CrowdSec Decision Management Integration Tests ==="

    build_container
    wait_for_api
    authenticate

    PASSED=0
    FAILED=0

    for test in test_status test_list_decisions_empty test_ban_ip test_verify_ban \
                test_unban_ip test_verify_removal test_export test_lapi_health; do
        if $test; then
            ((PASSED++))
        else
            ((FAILED++))
        fi
    done

    cleanup

    echo ""
    log_info "=== Results ==="
    log_info "Passed: $PASSED"
    log_info "Failed: $FAILED"

    [ $FAILED -eq 0 ]
}

main "$@"
```

### Go Integration Test

Location: `backend/integration/crowdsec_decisions_integration_test.go`

```go
//go:build integration
// +build integration

package integration

import (
    "context"
    "os/exec"
    "strings"
    "testing"
    "time"
)

func TestCrowdsecDecisionsIntegration(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    cmd := exec.CommandContext(ctx, "bash", "./scripts/crowdsec_decision_integration.sh")
    cmd.Dir = "../../"

    out, err := cmd.CombinedOutput()
    t.Logf("crowdsec decisions integration output:\n%s", string(out))

    if err != nil {
        t.Fatalf("crowdsec decisions integration failed: %v", err)
    }

    if !strings.Contains(string(out), "Passed:") {
        t.Fatalf("unexpected script output")
    }
}
```

---

## Error Scenarios

### Invalid IP Format

```bash
curl -X POST -b "$COOKIE_FILE" \
  -H "Content-Type: application/json" \
  -d '{"ip": "invalid-ip"}' \
  http://localhost:8080/api/v1/admin/crowdsec/ban
```

**Expected:** HTTP 400 or underlying cscli error

### Missing IP Parameter

```bash
curl -X POST -b "$COOKIE_FILE" \
  -H "Content-Type: application/json" \
  -d '{"duration": "1h"}' \
  http://localhost:8080/api/v1/admin/crowdsec/ban
```

**Expected:** HTTP 400 `{"error": "ip is required"}`

### Empty IP String

```bash
curl -X POST -b "$COOKIE_FILE" \
  -H "Content-Type: application/json" \
  -d '{"ip": "   "}' \
  http://localhost:8080/api/v1/admin/crowdsec/ban
```

**Expected:** HTTP 400 `{"error": "ip cannot be empty"}`

### CrowdSec Not Available

When `cscli` is not in PATH:
**Expected:** HTTP 200 with `{"decisions": [], "error": "cscli not available or failed"}`

### Export When No Config

```bash
# When data/crowdsec doesn't exist
curl -b "$COOKIE_FILE" http://localhost:8080/api/v1/admin/crowdsec/export
```

**Expected:** HTTP 404 `{"error": "crowdsec config not found"}`

---

## Frontend Test IDs

The following `data-testid` attributes are available for E2E testing:

| Element | Test ID |
|---------|---------|
| Mode Toggle | `crowdsec-mode-toggle` |
| Import File Input | `import-file` |
| Import Button | `import-btn` |
| Apply Preset Button | `apply-preset-btn` |
| File Select Dropdown | `crowdsec-file-select` |

---

## Success Criteria

- [ ] All 11 test cases pass
- [ ] Integration script completes without errors
- [ ] Ban/Unban cycle completes in < 5 seconds
- [ ] Export produces valid tar.gz archive
- [ ] Import creates backup before overwriting
- [ ] UI reflects state changes within 2 seconds
- [ ] Error messages are user-friendly

---

## References

- [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) - Main handler implementation
- [crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go) - Process management
- [crowdsec.ts](../../frontend/src/api/crowdsec.ts) - Frontend API client
- [CrowdSecConfig.tsx](../../frontend/src/pages/CrowdSecConfig.tsx) - UI component
- [features.md](../features.md) - User-facing feature documentation
