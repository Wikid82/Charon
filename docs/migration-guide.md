---
title: CrowdSec Control Migration Guide
description: Migration guide for upgrading Charon from environment variable to GUI-controlled CrowdSec configuration.
---

## CrowdSec Control Migration Guide

### What Changed in Version 2.0

**Before (v1.x):** CrowdSec was controlled by environment variables like `CHARON_SECURITY_CROWDSEC_MODE`.

**After (v2.x):** CrowdSec is controlled via the **GUI toggle** in the Security dashboard, matching how WAF, ACL, and Rate Limiting work.

---

## Why This Changed

### The Problem with Environment Variables

In version 1.x, CrowdSec had **inconsistent control**:

- **WAF, ACL, Rate Limiting:** GUI-controlled via Settings table
- **CrowdSec:** Environment variable controlled via docker-compose.yml

This created issues:

- ❌ Users had to restart containers to enable/disable CrowdSec
- ❌ GUI toggle didn't actually control the service
- ❌ Console enrollment could fail silently when LAPI wasn't running
- ❌ Inconsistent UX compared to other security features

### The Solution: GUI-Based Control

Version 2.0 makes CrowdSec work like all other security features:

- ✅ Enable/disable via GUI toggle (no container restart)
- ✅ Real-time status visible in dashboard
- ✅ Better integration with Charon's security orchestration
- ✅ Consistent UX across all security features

---

## Migration Steps

### Step 1: Check Current Configuration

Check if you have CrowdSec environment variables set:

```bash
grep -i "CROWDSEC_MODE" docker-compose.yml
```

If you see any of these:

- `CHARON_SECURITY_CROWDSEC_MODE`
- `CERBERUS_SECURITY_CROWDSEC_MODE`
- `CPM_SECURITY_CROWDSEC_MODE`

...then you need to migrate.

### Step 2: Remove Environment Variables

**Edit your `docker-compose.yml`** and remove these lines:

```yaml
# REMOVE THESE LINES:
- CHARON_SECURITY_CROWDSEC_MODE=local
- CERBERUS_SECURITY_CROWDSEC_MODE=local
- CPM_SECURITY_CROWDSEC_MODE=local
```

Also remove (if present):

```yaml
# These are no longer used (external mode removed)
- CERBERUS_SECURITY_CROWDSEC_API_URL=
- CERBERUS_SECURITY_CROWDSEC_API_KEY=
```

**Example: Before**

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    environment:
      - CHARON_ENV=production
      - CHARON_SECURITY_CROWDSEC_MODE=local  # ← Remove this
```

**Example: After**

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    environment:
      - CHARON_ENV=production
      # CrowdSec is now GUI-controlled
```

### Step 3: Restart Container

```bash
docker compose down
docker compose up -d
```

⚠️ **Important:** After restart, CrowdSec will NOT be running by default. You must enable it via the GUI (next step).

### Step 4: Enable CrowdSec via GUI

1. Open Charon UI (default: `http://localhost:8080`)
2. Navigate to **Security** in the sidebar
3. Find the **CrowdSec** card
4. Toggle the switch to **ON**
5. Wait 10-15 seconds for LAPI to start
6. Verify status shows "Active" with a running PID

### Step 5: Verify LAPI is Running

```bash
docker exec charon cscli lapi status
```

**Expected output:**

```
✓ You can successfully interact with Local API (LAPI)
```

If you see this, migration is complete! ✅

---

---

## Database Migrations for Upgrades

### What Are Database Migrations?

Charon version 2.0 introduced new database tables to support security features like CrowdSec, WAF configurations, and security audit logs. If you're upgrading from version 1.x **with persistent data**, you need to run migrations to add these tables.

### Do I Need to Run Migrations?

**Yes, if:**

- ✅ You're upgrading from Charon 1.x to 2.x
- ✅ You're using a persistent volume for `/app/data`
- ✅ You see "CrowdSec not starting" after upgrade
- ✅ Container logs show: `WARN security tables missing`

**No, if:**

- ❌ This is a fresh installation (tables created automatically)
- ❌ You're not using persistent storage
- ❌ You've already run migrations once

### How to Run Migrations

**Step 1: Execute Migration Command**

```bash
docker exec charon /app/charon migrate
```

**Expected Output:**

```json
{"level":"info","msg":"Running database migrations for security tables...","time":"2025-12-15T..."}
{"level":"info","msg":"Migration completed successfully","time":"2025-12-15T..."}
```

**Step 2: Verify Tables Created**

```bash
docker exec charon sqlite3 /app/data/charon.db ".tables"
```

**You should see these tables:**

- `security_configs` — Security feature settings (replaces environment variables)
- `security_decisions` — CrowdSec blocking decisions
- `security_audits` — Security event audit log
- `security_rule_sets` — WAF and rate limiting rules
- `crowdsec_preset_events` — CrowdSec Hub preset tracking
- `crowdsec_console_enrollments` — CrowdSec Console enrollment state

**Step 3: Restart Container**

If you had CrowdSec enabled before the upgrade, restart to apply changes:

```bash
docker restart charon
```

CrowdSec will automatically start if it was previously enabled.

**Step 4: Verify CrowdSec Status**

Wait 15 seconds after restart, then check:

```bash
docker exec charon cscli lapi status
```

**Expected Output (if CrowdSec was enabled):**

```
✓ You can successfully interact with Local API (LAPI)
```

### What Gets Migrated?

The migration creates **empty tables with the correct schema**. Your existing data (proxy hosts, certificates, users, etc.) is **not modified**.

**New tables added:**

1. **SecurityConfig**: Stores security feature state (on/off)
2. **SecurityDecision**: Tracks CrowdSec blocking decisions
3. **SecurityAudit**: Logs security-related actions
4. **SecurityRuleSet**: Stores WAF rules and rate limits
5. **CrowdsecPresetEvent**: Tracks Hub preset installations
6. **CrowdsecConsoleEnrollment**: Stores Console enrollment tokens

### Migration is Safe

✅ **Idempotent**: Safe to run multiple times (no duplicates)
✅ **Non-destructive**: Only adds tables, never deletes data
✅ **Fast**: Completes in <1 second
✅ **No downtime**: Container stays running during migration

### Troubleshooting Migrations

#### "Migration command not found"

**Cause**: You're running an older version of Charon that doesn't include the migrate command.

**Solution**: Pull the latest image first:

```bash
docker compose pull
docker compose up -d
docker exec charon /app/charon migrate
```

#### "Database is locked"

**Cause**: Another process is accessing the database.

**Solution**: Retry in a few seconds:

```bash
sleep 5
docker exec charon /app/charon migrate
```

#### "Permission denied accessing database"

**Cause**: Database file has incorrect permissions.

**Solution**: Fix ownership (run on host):

```bash
sudo chown -R 1000:1000 ./charon-data
docker exec charon /app/charon migrate
```

#### "CrowdSec still not starting after migration"

See [CrowdSec Troubleshooting](troubleshooting/crowdsec.md#database-migrations-after-upgrade) for detailed diagnostics.

### When Will This Be Automatic?

Future versions will detect missing tables on startup and run migrations automatically. For now, manual migration is required when upgrading from version 1.x.

---

## Console Enrollment (If Applicable)

If you were enrolled in CrowdSec Console **before migration**:

### Your Enrollment is Preserved ✅

The enrollment data is stored in the database, not in environment variables. Your Console connection should still work after migration.

### Verify Console Status

1. Go to **Cerberus → CrowdSec** in the sidebar
2. Check the Console enrollment status
3. If it shows "Enrolled" → you're good! ✅
4. If it shows "Not Enrolled" but you were enrolled before → see troubleshooting below

### Re-Enroll (If Needed)

If enrollment was incomplete in v1.x (common issue), re-enroll now:

1. Ensure CrowdSec is **enabled** via GUI toggle (see Step 4 above)
2. Verify LAPI is running: `docker exec charon cscli lapi status`
3. Go to **Cerberus → CrowdSec**
4. Click **Enroll with CrowdSec Console**
5. Paste your enrollment token from crowdsec.net
6. Submit

⚠️ **Note:** Enrollment tokens are **reusable** — you can use the same token multiple times.

---

## Benefits of GUI Control

### Before (Environment Variables)

```
1. Edit docker-compose.yml
2. docker compose down
3. docker compose up -d
4. Wait for container to restart (30-60 seconds)
5. Hope CrowdSec started correctly
6. Check logs to verify
```

### After (GUI Toggle)

```
1. Toggle switch in Security dashboard
2. Wait 10 seconds
3. See "Active" status immediately
```

### Feature Comparison

| Aspect | Environment Variable (Old) | GUI Toggle (New) |
|--------|---------------------------|------------------|
| **Enable/Disable** | Edit file + restart container | Click toggle |
| **Time to apply** | 30-60 seconds | 10-15 seconds |
| **Status visibility** | Check logs | Real-time dashboard |
| **Downtime during change** | ❌ Yes (container restart) | ✅ No (zero downtime) |
| **Consistency with other features** | ❌ Different from WAF/ACL | ✅ Same as WAF/ACL |
| **Console enrollment requirement** | ⚠️ Easy to forget LAPI check | ✅ UI warns if LAPI not running |

---

## Troubleshooting

### "CrowdSec won't start after toggling"

**Solution:**

1. Check container logs:

   ```bash
   docker logs charon | grep crowdsec
   ```

2. Verify config directory exists:

   ```bash
   docker exec charon ls -la /app/data/crowdsec/config
   ```

3. If missing, restart container:

   ```bash
   docker compose restart
   ```

4. Try toggling again in GUI

### "Console enrollment still shows 'Not Enrolled'"

**Solution:**

1. Verify LAPI is running:

   ```bash
   docker exec charon cscli lapi status
   ```

2. If LAPI is not running:
   - Toggle CrowdSec OFF in GUI
   - Wait 5 seconds
   - Toggle CrowdSec ON in GUI
   - Wait 15 seconds
   - Re-check LAPI status

3. Re-submit enrollment token (same token works)

### "I want to keep using environment variables"

**Not recommended.** Environment variable control is deprecated and will be removed in a future version.

**If you must:**

The legacy environment variables still work in version 2.0 (for backward compatibility), but:

- ⚠️ They will be removed in version 3.0
- ⚠️ GUI toggle may not reflect actual state
- ⚠️ You'll encounter issues with Console enrollment
- ⚠️ You'll miss out on improved UX and features

**Please migrate to GUI control.**

### "Can I automate CrowdSec control via API?"

**Yes!** Use the Charon API:

**Enable CrowdSec:**

```bash
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
```

**Disable CrowdSec:**

```bash
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/stop
```

**Check status:**

```bash
curl http://localhost:8080/api/v1/admin/crowdsec/status
```

See [API Documentation](api.md) for more details.

---

## Rollback (Emergency)

If you encounter critical issues after migration, you can temporarily roll back to environment variable control:

1. **Add back the environment variable:**

   ```yaml
   environment:
     - CHARON_SECURITY_CROWDSEC_MODE=local
   ```

2. **Restart container:**

   ```bash
   docker compose down
   docker compose up -d
   ```

3. **Report the issue:**
   - [GitHub Issues](https://github.com/Wikid82/charon/issues)
   - Describe what went wrong
   - Attach relevant logs

⚠️ **This is a temporary workaround.** Please report issues so we can fix them.

---

## Support

**Need help?**

- 📖 [Full Documentation](https://wikid82.github.io/charon/)
- 🛡️ [Security Features Guide](security.md)
- 🐛 [CrowdSec Troubleshooting](troubleshooting/crowdsec.md)
- 💬 [Community Discussions](https://github.com/Wikid82/charon/discussions)
- 🐛 [Report Issues](https://github.com/Wikid82/charon/issues)

---

## Summary

✅ **Remove** environment variables from docker-compose.yml
✅ **Restart** container
✅ **Enable** CrowdSec via GUI toggle in Security dashboard
✅ **Verify** LAPI is running
✅ **Re-enroll** in Console if needed (same token works)

**Benefits:**

- ⚡ Faster enable/disable (no container restart)
- 👀 Real-time status visibility
- 🎯 Consistent with other security features
- 🛡️ Better Console enrollment reliability

**Timeline:** Environment variable support will be removed in version 3.0 (estimated 6-12 months).
