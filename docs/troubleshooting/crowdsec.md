---
title: CrowdSec Troubleshooting
description: Troubleshooting guide for CrowdSec integration issues in Charon. LAPI initialization, console enrollment, and common problems.
---

## CrowdSec Troubleshooting

Keep Cerberus terminology and the Configuration Packages flow in mind while debugging Hub presets.

### Quick checks

- Cerberus is enabled and you are signed in with admin scope.
- `cscli` is available (preferred path); HTTPS CrowdSec Hub endpoints only.
  - Docker images (v1.7.4+): cscli is pre-installed.
  - Bare-metal deployments: install cscli for Hub preset sync or use HTTP fallback with HUB_BASE_URL.
- HUB_BASE_URL points to a JSON hub endpoint (default: <https://hub-data.crowdsec.net/api/index.json>). Redirects to HTML will be rejected.
- Proxy env is set when required: HTTP(S)_PROXY and NO_PROXY are respected by the hub client.
- For slow or proxied networks, increase HUB_PULL_TIMEOUT_SECONDS (default 25) and HUB_APPLY_TIMEOUT_SECONDS (default 45) to avoid premature timeouts.
- Preset workflow: pull from Hub using cache keys/ETags → preview changes → apply with automatic backup and reload flag.
- Preset pull/apply requires either cscli or cached presets.
- Offline/curated presets remain available at all times.

## LAPI Initialization and Timing

### Understanding LAPI Startup

When you enable CrowdSec via the GUI toggle, the Local API (LAPI) needs time to initialize before it's ready to accept requests. This is normal behavior.

**Typical startup times:**

- **Initial start:** 5-10 seconds
- **First start after container restart:** 10-15 seconds
- **Maximum wait:** 30 seconds (with automatic retries)

**What happens during startup:**

1. CrowdSec process starts
2. Configuration is loaded
3. Database connections are established
4. Parsers and scenarios are loaded
5. LAPI becomes available on port 8085
6. Status changes from "Starting" to "Active"

### Expected User Experience

When you toggle CrowdSec ON in the Security dashboard:

1. **Loading overlay appears** — "Starting CrowdSec... This may take up to 30 seconds"
2. **Backend polls LAPI** — Checks every 500ms for up to 30 seconds
3. **Success toast displays** — One of two messages:
   - ✅ "CrowdSec started and LAPI is ready" — You can immediately enroll in Console
   - ⚠️ "CrowdSec started but LAPI is still initializing" — Wait before enrolling

### Verifying LAPI Status

**Check if LAPI is running:**

```bash
docker exec charon cscli lapi status
```

**Expected output when ready:**

```
✓ You can successfully interact with Local API (LAPI)
```

**If LAPI is not ready yet:**

```
ERROR: connection refused
```

**Check LAPI health endpoint directly:**

```bash
docker exec charon curl -s http://localhost:8085/health
```

**Expected response when healthy:**

```json
{"status":"up"}
```

### Troubleshooting LAPI Initialization

#### Problem: LAPI takes longer than 30 seconds

**Symptoms:**

- Warning message: "LAPI is still initializing"
- Console enrollment fails with "LAPI not available"

**Solution 1 - Wait and retry:**

```bash
# Wait 15 seconds, then check again
sleep 15
docker exec charon cscli lapi status
```

**Solution 2 - Check CrowdSec logs:**

```bash
docker logs charon | grep -i crowdsec | tail -20
```

Look for:

- ✅ "CrowdSec Local API listening" — LAPI started successfully
- ✅ "parsers loaded" — Configuration loaded
- ❌ "error" or "fatal" — Initialization problem

**Solution 3 - Restart CrowdSec:**

1. Go to Security dashboard
2. Toggle CrowdSec **OFF**
3. Wait 5 seconds
4. Toggle CrowdSec **ON**
5. Wait 15 seconds
6. Verify status shows "Active"

#### Problem: LAPI never becomes available

**Check if CrowdSec process is running:**

```bash
docker exec charon ps aux | grep crowdsec
```

**Expected output:**

```
crowdsec  203  0.5  2.3  /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml
```

**If no process is running:**

1. Check config directory exists:

   ```bash
   docker exec charon ls -la /app/data/crowdsec/config
   ```

2. If directory is missing:

   ```bash
   docker compose restart
   ```

3. Check for port conflicts:

   ```bash
   docker exec charon netstat -tulpn | grep 8085
   ```

4. Remove deprecated environment variables from docker-compose.yml (see migration section below)

#### Problem: LAPI responds but enrollment fails

**Check LAPI can process requests:**

```bash
docker exec charon cscli machines list
```

**Expected output:**

```
Name                     IP Address      Auth Type    Version
charon-local-machine     127.0.0.1      password     v1.x.x
```

**If command fails:**

- LAPI is running but database is not ready
- Wait 10 more seconds and retry
- Check logs for database errors

**If enrollment still fails:**

- Enrollment has automatic retry (3 attempts, 2 seconds apart)
- If all retries fail, toggle CrowdSec OFF/ON and try again
- See Console Enrollment section below for token troubleshooting

## Common issues

- Hub unreachable (503): retry once, then Charon falls back to cached Hub data if available; otherwise stay on curated/offline presets until connectivity returns.
- Hub returns HTML/redirect: set HUB_BASE_URL to the JSON endpoint above or install cscli so the index is fetched locally.
- Bad preset slug (400): the slug must match Hub naming; correct the slug before retrying.
- Apply failed: review the apply response and restore from the backup that was taken automatically, then retry after fixing the underlying issue.
- Apply not supported (501): use curated/offline presets; Hub apply will be re-enabled when supported in your environment.
- **Security Engine Offline**: If your dashboard says "Offline", it means CrowdSec LAPI is not running.
  - **Fix**: Ensure CrowdSec is **enabled via GUI toggle** in the Security dashboard. Do NOT use environment variables.
  - **Action**: Go to Security dashboard, toggle CrowdSec ON, wait 15 seconds, verify status shows "Active".

## CrowdSec Not Starting After Container Restart

### Problem: Toggle shows ON but CrowdSec is not running

**Symptoms:**

- Container restarted (reboot, Docker restart, etc.)
- Security dashboard toggle shows "ON"
- Status badge shows "Not Running" or "Offline"
- Manually toggling OFF then ON fixes it

**Root Cause:**

The reconciliation function couldn't determine if CrowdSec should auto-start. This happens when:

1. **SecurityConfig table is missing/corrupted** (database issue)
2. **Settings table and SecurityConfig are out of sync** (partial update)
3. **Reconciliation logs show silent exit** (no "starting based on" message)

### Diagnosis: Check Reconciliation Logs

**View container startup logs:**

```bash
docker logs charon | grep -i "crowdsec reconciliation"
```

**Expected output when working correctly:**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"..."}
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'","time":"..."}
{"level":"info","msg":"CrowdSec Local API listening on 127.0.0.1:8085","time":"..."}
```

**Problematic output (silent exit - BUG):**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"..."}
[NO FURTHER LOGS - Function exited without starting CrowdSec]
```

This indicates reconciliation found conflicting state between Settings and SecurityConfig tables.

### Solution 1: Verify Database State

**Check Settings table:**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT key, value FROM settings WHERE key = 'security.crowdsec.enabled';"
```

**Expected output:**

```
security.crowdsec.enabled|true
```

**Check SecurityConfig table:**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT uuid, crowdsec_mode, enabled FROM security_configs WHERE uuid = 'default';"
```

**Expected output:**

```
default|local|1
```

**Mismatch scenarios:**

| Settings | SecurityConfig | Behavior | Fix Needed |
|----------|----------------|----------|------------|
| `true` | `local` | ✅ Auto-starts | None |
| `true` | `disabled` | ❌ Does NOT start | Run Solution 2 |
| `true` | (missing) | ⚠️ Should auto-create | Run Solution 3 |
| `false` | `local` | ⚠️ Conflicting state | Run Solution 2 |
| `false` | `disabled` | ✅ Correctly skipped | None (expected) |

### Solution 2: Manually Sync SecurityConfig to Settings

**If you want CrowdSec enabled (Settings = true, SecurityConfig = disabled):**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE security_configs SET crowdsec_mode = 'local', enabled = 1 WHERE uuid = 'default';"

docker restart charon
```

**If you want CrowdSec disabled (Settings = false, SecurityConfig = local):**

```bash
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE security_configs SET crowdsec_mode = 'disabled', enabled = 0 WHERE uuid = 'default';"

# Also update Settings for consistency
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE settings SET value = 'false' WHERE key = 'security.crowdsec.enabled';"

docker restart charon
```

### Solution 3: Force Recreation of SecurityConfig

**If SecurityConfig table is missing (record not found):**

```bash
# Delete SecurityConfig (if partial record exists)
docker exec charon sqlite3 /app/data/charon.db \
  "DELETE FROM security_configs WHERE uuid = 'default';"

# Restart container - reconciliation will auto-create matching Settings state
docker restart charon

# Wait 15 seconds for startup
sleep 15

# Verify CrowdSec started
docker exec charon cscli lapi status
```

**Expected behavior:**

- Reconciliation detects missing SecurityConfig
- Checks Settings table for user preference
- Creates SecurityConfig with matching state
- Starts CrowdSec if Settings = true

**Check logs to confirm:**

```bash
docker logs charon | grep "default SecurityConfig created"
```

Expected:

```json
{"level":"info","msg":"CrowdSec reconciliation: default SecurityConfig created from Settings preference","crowdsec_mode":"local","enabled":true,"source":"settings_table"}
```

### Solution 4: Use GUI Toggle (Safest)

**The GUI toggle synchronizes both tables atomically:**

1. Go to **Security** dashboard
2. Toggle CrowdSec **OFF** (if it shows ON)
3. Wait 5 seconds
4. Toggle CrowdSec **ON**
5. Wait 15 seconds for LAPI to initialize
6. Verify status shows "Active"

**Why this works:**

- Toggle updates Settings table
- Toggle updates SecurityConfig table
- Start handler ensures both tables match
- Future restarts use reconciliation correctly

### Solution 5: Manual Reset (Nuclear Option)

**If all else fails, reset both tables:**

```bash
# Stop CrowdSec if running
docker exec charon pkill crowdsec || true

# Reset both tables
docker exec charon sqlite3 /app/data/charon.db <<EOF
UPDATE settings SET value = 'false' WHERE key = 'security.crowdsec.enabled';
DELETE FROM security_configs WHERE uuid = 'default';
EOF

# Restart container
docker restart charon

# Re-enable via GUI
# Go to Security dashboard and toggle CrowdSec ON
```

### Prevention: Verify After Manual Database Changes

**If you manually edit the database:**

```bash
# Always verify both tables match
docker exec charon sqlite3 /app/data/charon.db <<EOF
SELECT 'Settings:' as table_name, value as state
FROM settings WHERE key = 'security.crowdsec.enabled'
UNION ALL
SELECT 'SecurityConfig:', crowdsec_mode
FROM security_configs WHERE uuid = 'default';
EOF
```

**Expected output (both enabled):**

```
Settings:|true
SecurityConfig:|local
```

**Expected output (both disabled):**

```
Settings:|false
SecurityConfig:|disabled
```

### When to Contact Support

If after following all solutions:

- ❌ Reconciliation logs still show silent exit
- ❌ Both tables show correct state but CrowdSec doesn't start
- ❌ Manual `cscli lapi status` fails even after toggle

**Gather diagnostic info:**

```bash
# Collect logs
docker logs charon > charon-logs.txt 2>&1

# Collect database state
docker exec charon sqlite3 /app/data/charon.db ".dump security_configs" > db-state.sql
docker exec charon sqlite3 /app/data/charon.db ".dump settings" >> db-state.sql

# Collect process state
docker exec charon ps aux > process-state.txt
```

**Report issue:** <https://github.com/Wikid82/charon/issues>

Include:

- Output of all diagnostic commands above
- Steps you tried from this guide
- Container restart logs showing reconciliation behavior

## Tips

- Keep the CrowdSec Hub reachable over HTTPS; HTTP is blocked.
- If you switch to offline mode, clear pending Hub pulls before retrying so cache keys/ETags refresh cleanly.
- After restoring from a backup, re-run preview before applying again to verify changes.
- **Always use the GUI toggle** for enabling/disabling CrowdSec—it ensures Settings and SecurityConfig stay synchronized.
- **Check reconciliation logs** after container restart to verify auto-start behavior.

## Database Migrations After Upgrade

### Problem: CrowdSec not starting after upgrading Charon

**Symptoms:**

- CrowdSec toggle appears enabled but status shows "Not Running"
- CrowdSec console shows "Starting..." indefinitely
- Container logs show: `WARN CrowdSec reconciliation: security tables missing`
- Console enrollment fails immediately

**Root Cause:**

Upgrading from an older version with a **persistent database** may be missing the new security tables introduced in version 2.0. The database schema needs to be migrated.

**Solution: Run Database Migration**

1. **Execute the migration command:**

   ```bash
   docker exec charon /app/charon migrate
   ```

   **Expected output:**

   ```json
   {"level":"info","msg":"Running database migrations for security tables...","time":"..."}
   {"level":"info","msg":"Migration completed successfully","time":"..."}
   ```

2. **Verify tables were created:**

   ```bash
   docker exec charon sqlite3 /app/data/charon.db ".tables"
   ```

   **Expected tables include:**
   - `security_configs`
   - `security_decisions`
   - `security_audits`
   - `security_rule_sets`
   - `crowdsec_preset_events`
   - `crowdsec_console_enrollments`

3. **Restart container to apply changes:**

   ```bash
   docker restart charon
   ```

4. **Verify CrowdSec starts automatically:**

   If you had CrowdSec enabled before the upgrade:

   ```bash
   # Wait 15 seconds after restart, then check
   docker exec charon cscli lapi status
   ```

   **Expected output:**

   ```
   ✓ You can successfully interact with Local API (LAPI)
   ```

5. **If CrowdSec doesn't auto-start:**

   Enable it manually via the GUI:
   - Go to **Security** dashboard
   - Toggle CrowdSec **ON**
   - Wait 15 seconds
   - Verify status shows "Active"

**Why This Happens:**

Charon version 2.0 moved CrowdSec configuration from environment variables to the database (see [Migration Guide](../migration-guide.md)). Persistent databases from older versions need the new security tables added via migration.

**Prevention:**

Future upgrades will run migrations automatically on startup. For now, manual migration is required for existing installations.

**Related Documentation:**

- [Getting Started - Database Migrations](../getting-started.md#step-15-database-migrations-if-upgrading)
- [Migration Guide - CrowdSec Control](../migration-guide.md)

---

## Console Enrollment

### Prerequisites

Before attempting Console enrollment, ensure:

✅ **CrowdSec is enabled** — Toggle must be ON in Security dashboard
✅ **LAPI is running** — Check with: `docker exec charon cscli lapi status`
✅ **Feature flag enabled** — `feature.crowdsec.console_enrollment` must be ON
✅ **Valid token** — Obtain from crowdsec.net

### "missing login field" or CAPI errors

Charon automatically attempts to register your instance with CrowdSec's Central API (CAPI) before enrolling. Ensure your server has internet access to `api.crowdsec.net`.

### Enrollment shows "enrolled" but not on crowdsec.net

**Root cause:** LAPI was not running when enrollment was attempted.

Charon now checks LAPI availability before enrollment and retries automatically (3 attempts with 2-second delays), but in rare cases enrollment may still fail if LAPI is initializing.

**Solution:**

1. Verify LAPI status:

   ```bash
   docker exec charon cscli lapi status
   ```

   **Expected output when ready:**

   ```
   ✓ You can successfully interact with Local API (LAPI)
   ```

   **If LAPI is not running:**

   ```
   ERROR: cannot contact local API
   ```

2. If LAPI is not running:
   - Go to Security dashboard
   - Toggle CrowdSec **OFF**
   - Wait 5 seconds
   - Toggle CrowdSec **ON**
   - **Wait 15 seconds** (important: LAPI needs time to initialize)
   - Re-check LAPI status

3. Verify LAPI health endpoint:

   ```bash
   docker exec charon curl -s http://localhost:8085/health
   ```

   **Expected response:**

   ```json
   {"status":"up"}
   ```

4. Re-submit enrollment token:
   - Go to **Cerberus → CrowdSec**
   - Click **Enroll with CrowdSec Console**
   - Paste the same enrollment token (tokens are reusable)
   - Click **Submit**
   - Wait 30-60 seconds for confirmation

5. Verify enrollment on crowdsec.net:
   - Log in to your CrowdSec Console account
   - Navigate to **Instances**
   - Your Charon instance should appear in the list

**Understanding the automatic retry:**

Charon automatically retries LAPI checks during enrollment:

- **Attempt 1:** Immediate check
- **Attempt 2:** After 2 seconds (if LAPI not ready)
- **Attempt 3:** After 4 seconds (if still not ready)
- **Total:** 3 attempts over 6 seconds

This handles most cases where LAPI is still initializing. If all 3 attempts fail, follow the solution above.

### CrowdSec won't start via GUI toggle

**Solution:**

1. Check container logs:

   ```bash
   docker logs charon | grep -i crowdsec
   ```

   Look for:
   - ✅ "Starting CrowdSec Local API"
   - ✅ "CrowdSec Local API listening on 127.0.0.1:8085"
   - ❌ "failed to start" or "error loading config"

2. Verify config directory:

   ```bash
   docker exec charon ls -la /app/data/crowdsec/config
   ```

   Expected files:
   - `config.yaml` — Main configuration
   - `local_api_credentials.yaml` — LAPI authentication
   - `acquis.yaml` — Log sources

3. Check for common startup errors:

   **Error: "config.yaml not found"**

   ```bash
   # Restart container to regenerate config
   docker compose restart
   ```

   **Error: "port 8085 already in use"**

   ```bash
   # Check for conflicting services
   docker exec charon netstat -tulpn | grep 8085
   # Stop conflicting service or change CrowdSec LAPI port
   ```

   **Error: "permission denied"**

   ```bash
   # Fix ownership (run on host)
   sudo chown -R 1000:1000 ./data/crowdsec
   docker compose restart
   ```

4. Remove any deprecated environment variables from docker-compose.yml:

   ```yaml
   # REMOVE THESE:
   - CHARON_SECURITY_CROWDSEC_MODE=local
   - CERBERUS_SECURITY_CROWDSEC_MODE=local
   - CPM_SECURITY_CROWDSEC_MODE=local
   ```

5. Restart and try GUI toggle again:

   ```bash
   docker compose restart
   # Wait 30 seconds for container to fully start
   # Then toggle CrowdSec ON in GUI
   ```

6. Verify CrowdSec is running:

   ```bash
   # Check process
   docker exec charon ps aux | grep crowdsec

   # Check LAPI health
   docker exec charon cscli lapi status

   # Check LAPI endpoint
   docker exec charon curl -s http://localhost:8085/health
   ```

### Environment Variable Migration

🚨 **DEPRECATED:** The `CHARON_SECURITY_CROWDSEC_MODE` environment variable is no longer used.

If you have this in your docker-compose.yml, remove it and use the GUI toggle instead. See [Migration Guide](../migration-guide.md) for step-by-step instructions.

### Configuration File

Charon uses the configuration located in `data/crowdsec/config.yaml`. Ensure this file exists and is readable if you are manually modifying it.
