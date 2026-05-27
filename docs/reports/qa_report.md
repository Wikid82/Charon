# QA Audit Report — Orthrus Race Fix Validation

**Date:** 2026-05-27
**Branch:** `development`
**Scope:** Validation of the orthrus race fix — single-file reorder in `backend/internal/orthrus/server.go` (`wg.Add(1)` moved before `sessions.Store` in `HandleWebSocket`)
**Auditor:** QA Security Agent

---

## Verdict: APPROVED ✅

All validation steps pass. The race condition is eliminated. No regressions introduced. Pre-existing hook warnings noted and scoped as unrelated.

---

## Fix Description

In `HandleWebSocket` (`backend/internal/orthrus/server.go`), the call to `sessions.Store` was previously placed before `wg.Add(1)`. This created a window where `Stop()` could observe a session entry in the map, call `wg.Wait()`, and return before the corresponding goroutine (`watchHeartbeat`) had registered itself with the WaitGroup — resulting in a data race.

**After fix:**

```go
s.wg.Add(1)                    // ← Add BEFORE launching goroutine
go func() {
    defer s.wg.Done()
    s.watchHeartbeat(agent.UUID, session)
}()
s.sessions.Store(agent.UUID, session)  // ← Store AFTER wg.Add
```

This ordering guarantees that any session visible to `Stop()` via `sessions.Range` has already called `wg.Add(1)`, so `wg.Wait()` cannot return while that goroutine is still running.

---

## Validation Steps

### Step 1 — Targeted race-detector stress test (20×)

**Command:**
```
cd /projects/Charon/backend && go test -race -count=20 -run TestOrthrusServer_HandleWebSocket ./internal/orthrus/
```

**Result:** PASS — 20/20 iterations completed, zero `DATA RACE` warnings

---

### Step 2 — Full orthrus package race-detector run (5×)

**Command:**
```
cd /projects/Charon/backend && go test -race -count=5 ./internal/orthrus/...
```

**Result:** PASS
```
ok  github.com/Wikid82/charon/backend/internal/orthrus  4.690s
```

---

### Step 3 — Full backend test suite (no race detector)

**Command:**
```
cd /projects/Charon/backend && go test -count=1 ./...
```

**Result:** PASS — All 36 packages `ok`, zero `FAIL` lines

| Package | Result | Time |
|---|---|---|
| `cmd/api` | ok | 0.836s |
| `cmd/localpatchreport` | ok | 2.370s |
| `cmd/seed` | ok | 0.359s |
| `internal/api` | ok | 0.007s |
| `internal/api/handlers` | ok | 72.394s |
| `internal/api/middleware` | ok | 1.011s |
| `internal/api/routes` | ok | 1.640s |
| `internal/api/tests` | ok | 0.381s |
| `internal/caddy` | ok | 6.625s |
| `internal/cerberus` | ok | 1.595s |
| `internal/config` | ok | 0.009s |
| `internal/crowdsec` | ok | 94.918s |
| `internal/crypto` | ok | 0.036s |
| `internal/database` | ok | 0.023s |
| `internal/hecate` | ok | 5.098s |
| `internal/hecate/providers/cloudflare` | ok | 0.029s |
| `internal/hecate/providers/netbird` | ok | 0.018s |
| `internal/hecate/providers/tailscale` | ok | 0.017s |
| `internal/hecate/providers/zerotier` | ok | 0.013s |
| `internal/logger` | ok | 0.007s |
| `internal/metrics` | ok | 0.011s |
| `internal/models` | ok | 0.567s |
| `internal/network` | ok | 0.724s |
| `internal/notifications` | ok | 0.220s |
| `internal/orthrus` | ok | 0.622s |
| `internal/patchreport` | ok | 0.021s |
| `internal/security` | ok | 0.073s |
| `internal/server` | ok | 0.722s |
| `internal/services` | ok | 93.611s |
| `internal/testutil` | ok | 0.021s |
| `internal/util` | ok | 0.011s |
| `internal/utils` | ok | 0.075s |
| `internal/version` | ok | 0.008s |
| `pkg/dnsprovider` | ok | 0.005s |
| `pkg/dnsprovider/builtin` | ok | 0.004s |
| `pkg/dnsprovider/custom` | ok | 0.010s |

---

### Step 4 — Pre-commit hook suite (lefthook)

> **Note:** This project uses `lefthook` v2.1.8 (not `pre-commit`). Config: `lefthook.yml`. There is no `.pre-commit-config.yaml`.

**Command:**
```
cd /projects/Charon && lefthook run pre-commit --force
```

**Results:**

| Hook | Result | Notes |
|---|---|---|
| `trailing-whitespace` | PASS | No issues |
| `block-data-backups` | PASS | No issues |
| `end-of-file-fixer` | PASS | No issues |
| `check-lfs-large-files` | PASS | No issues |
| `block-codeql-db` | PASS | No issues |
| `check-yaml` | PASS | No issues |
| `shellcheck` | SKIP | No `.sh` files matched; shellcheck exited with status 3 (no files specified — benign) |
| `check-version-match` | PRE-EXISTING WARN | `.version` file (v0.27.0) ≠ latest git tag (v0.32.0); script deprecated; **unrelated to this change** |
| `go-vet` | PASS | No issues |
| `dockerfile-check` | PASS | Dockerfile validation passed |
| `frontend-type-check` | PASS | No issues |
| `frontend-lint` | PASS | No issues |
| `golangci-lint-fast` | PASS | 0 issues |
| `actionlint` | PASS | No issues |
| `semgrep` | PASS | No issues |

**`check-version-match` warning scoped:** The `.version` / git tag mismatch is a pre-existing repository configuration issue that predates this change. The deprecated script warns it will be removed in v2.0.0. This does not indicate a defect introduced by the orthrus fix.

---

## Conclusion

The `wg.Add(1)` reorder in `HandleWebSocket` correctly eliminates the race condition between session registration and graceful shutdown. All validation steps pass:

- Race detector: zero `DATA RACE` reports across 20 targeted + 5 full-package runs
- Full backend suite: 36/36 packages pass, no regressions
- Lint and static analysis: zero new issues

The fix is approved for merge.
