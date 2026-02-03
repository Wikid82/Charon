# Phase 1 Lint Fixes - Implementation Tracker

## Status: IN PROGRESS

### Completed:
✅ JSON.Unmarshal fixes:
- security_handler_audit_test.go:581
- security_handler_coverage_test.go (2 locations: line 525 initially reported, now 590)
- settings_handler_test.go (3 locations: lines 1290, 1337, 1396)
- user_handler_test.go (3 locations: lines 120, 153, 443)

### Remaining Errcheck Issues (23):

#### Environment Variables (11):
- internal/config/config_test.go:56, 57, 72 (

os.Setenv)
- internal/config/config_test.go:157, 158, 159 (os.Unsetenv)
- internal/server/emergency_server_test.go:97, 98, 142, 143, 279, 280

#### Database Close (4):
- internal/services/certificate_service_test.go:1104
- internal/services/security_service_test.go:26
- internal/services/uptime_service_unit_test.go:25
- Also needed: dns_provider_service_test.go, database/errors_test.go

#### Other (8):
- handlers_blackbox_test.go:1501, 1503 (db.Callback().Register, tx.AddError)
- security_handler_waf_test.go:526, 527, 528 (os.Remove)
- emergency_server_test.go: 67, 79, 108, 125, 155, 171 (server.Stop, resp.Body.Close)
- backup_service_test.go: Multiple Close() operations

### Remaining Gosec Issues (24):

#### G115 - Integer Overflow (3):
- internal/api/handlers/manual_challenge_handler.go:649, 651
- internal/api/handlers/security_handler_rules_decisions_test.go:162

#### G110 - Decompression Bomb (2):
- internal/crowdsec/hub_sync.go:1016
- internal/services/backup_service.go:345

#### G305 - Path Traversal (1):
- internal/services/backup_service.go:316

#### G306/G302 - File Permissions (10+):
- server_test.go:19
- backup_service.go:36, 324, 328
- backup_service_test.go:28, 35, 469, 470, 538

#### G304 - File Inclusion (4):
- config_test.go:67, 148
- backup_service.go:178, 218, 332

#### G112 - Slowloris (2):
- uptime_service_test.go:80, 855

#### G101 - Hardcoded Credentials (3):
- rfc2136_provider_test.go:171, 381, 414

#### G602 - Slice Bounds (1):
- caddy/config.go:463

## Implementation Strategy

Given the scope (55+ issues), I'll implement fixes in priority order:

1. **HIGH PRIORITY**: Gosec security issues (decompression bomb, path traversal, permissions)
2. **MEDIUM PRIORITY**: Errcheck resource cleanup (database close, file close)
3. **LOW PRIORITY**: Test environment setup (os.Setenv/Unsetenv)

## Notes

- The original `full_lint_output.txt` was outdated
- Current lint run shows 61 issues total (31 errcheck + 24 gosec + 6 other)
- Some issues (bodyclose, staticcheck) are outside original spec scope
- Will focus on errcheck and gosec as specified in the plan
