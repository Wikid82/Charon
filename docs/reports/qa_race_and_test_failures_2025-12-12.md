# QA Report: Race + Test Failures (2025-12-12)

## Repro Commands

```sh
cd backend

go test -race ./internal/api/handlers -count=1

go test -race ./...
```

## Findings

### 1) Data race in global logger initialization/hook

- Race detector reports concurrent access between:
  - `backend/internal/logger.Init()` writing global `_broadcastHook`
  - `backend/internal/logger.GetBroadcastHook()` reading/initializing `_broadcastHook`
- Observed during WebSocket handler tests:
  - `backend/internal/api/handlers/logs_ws_test.go` (`TestLogsWebSocketHandler_SourceFilter`)
  - `backend/internal/api/handlers/logs_ws_test_utils.go` (`resetLogger` calls `logger.Init`)

Impact:

- `go test -race` fails with `WARNING: DATA RACE`.

### 2) WebSocket tests flake under -race (timeout)

- `TestLogsWebSocketHandler_LevelFilter` timed out waiting for a message:
  - `read tcp ... i/o timeout`

Likely contributing factor:

- Tests send log entries immediately after dialing without waiting for the server-side subscription/listener to be registered.

### 3) CrowdSec registration tests fail in environments without `bash`

- Failures:
  - `TestEnsureBouncerRegistered_ReturnsExistingBouncerKey`
  - `TestEnsureBouncerRegistered_RegistersNewWhenNoneExists`
- Error:
  - `register bouncer: exit status 127`

Likely root cause:

- Fake `cscli` uses `#!/usr/bin/env bash` + bashisms (`[[ ... ]]`, `pipefail`); systems without `bash` cause `/usr/bin/env` to exit `127`.

### 4) Security status contract mismatch

- `TestSecurityHandler_GetStatus_Fixed/All_Enabled` failed:
  - Expected `cerberus.enabled=true` and `acl.enabled=true`
  - Actual response returned `false` for both

Potential causes:

- Handler may not use `config.SecurityConfig` fields the way the test expects, or additional feature flags are required.

### 5) Missing-table errors in handler/service tests under -race

- Multiple `no such table: ...` errors observed (e.g., `uptime_monitors`, `uptime_heartbeats`, `settings`, etc.)

Hypothesis:

- Some tests drop tables or use DB instances without running migrations; under `-race` timing, later tests hit missing tables.
