# CrowdSec Reconciliation Failure Root Cause Analysis

**Date:** December 15, 2025
**Status:** CRITICAL - CrowdSec NOT starting despite 7+ commits attempting fixes
**Location:** `backend/internal/services/crowdsec_startup.go`

## Executive Summary

**The CrowdSec reconciliation function starts but exits silently** because the `security_configs` table **DOES NOT EXIST** in the production database. The table was added to AutoMigrate but the container was never rebuilt/restarted with a fresh database state after the migration code was added.

## The Silent Exit Point

Looking at the container logs:

```
{"bin_path":"crowdsec","data_dir":"/app/data/crowdsec","level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"2025-12-14T20:55:39-05:00"}
```

Then... NOTHING. The function exits silently.

### Why It Exits

In `backend/internal/services/crowdsec_startup.go`, line 33-36:

```go
// Check if SecurityConfig table exists and has a record with CrowdSecMode = "local"
if !db.Migrator().HasTable(&models.SecurityConfig{}) {
    logger.Log().Debug("CrowdSec reconciliation skipped: SecurityConfig table not found")
    return
}
```

**This guard clause triggers because the table doesn't exist**, but it logs at **DEBUG** level, not INFO/WARN/ERROR. Since the container is running in production mode (not debug), this log message is never shown.

### Database Evidence

```bash
$ sqlite3 data/charon.db ".tables"
access_lists                remote_servers
caddy_configs               settings
domains                     ssl_certificates
import_sessions             uptime_heartbeats
locations                   uptime_hosts
proxy_hosts                 uptime_monitors
notification_providers      uptime_notification_events
notifications               users
```

**NO `security_configs` TABLE EXISTS.** Yet the code in `backend/internal/api/routes/routes.go` clearly calls:

```go
if err := db.AutoMigrate(
    // ... other models ...
    &models.SecurityConfig{},
    &models.SecurityDecision{},
    &models.SecurityAudit{},
    &models.SecurityRuleSet{},
    // ...
); err != nil {
    return fmt.Errorf("auto migrate: %w", err)
}
```

## Why AutoMigrate Didn't Create the Tables

### Theory 1: Database Persistence Across Rebuilds ✅ MOST LIKELY

The `charon.db` file is mounted as a volume in the Docker container:

```yaml
# docker-compose.yml
volumes:
  - ./data:/app/data
```

**What happened:**

1. SecurityConfig model was added to AutoMigrate in recent commits
2. Container was rebuilt with `docker build -t charon:local .`
3. Container started with `docker compose up -d`
4. **BUT** the existing `data/charon.db` file (from before the migration code existed) was reused
5. GORM's AutoMigrate is **non-destructive** - it only adds new tables if they don't exist
6. The tables were never created because the database predates the migration code

### Theory 2: AutoMigrate Failed Silently

Looking at the logs, there is **NO** indication that AutoMigrate failed:

```
{"level":"info","msg":"starting Charon backend on version dev","time":"2025-12-14T20:55:39-05:00"}
{"bin_path":"crowdsec","data_dir":"/app/data/crowdsec","level":"info","msg":"CrowdSec reconciliation: starting startup check","time":"2025-12-14T20:55:39-05:00"}
{"level":"info","msg":"starting Charon backend on :8080","time":"2025-12-14T20:55:39-05:00"}
```

If AutoMigrate had failed, we would see an error from `routes.Register()` because it has:

```go
if err := db.AutoMigrate(...); err != nil {
    return fmt.Errorf("auto migrate: %w", err)
}
```

Since the server started successfully, AutoMigrate either:

- Ran successfully but found the DB already in sync (no new tables to add)
- Never ran because the DB was opened but the tables already existed from a previous run

## The Cascading Failures

Because `security_configs` doesn't exist:

1. ✅ Reconciliation exits at line 33-36 (HasTable check)
2. ✅ CrowdSec is never started
3. ✅ Frontend shows "CrowdSec is not running" in Console Enrollment
4. ✅ Security page toggle is stuck ON (because there's no DB record to persist the state)
5. ✅ Log viewer shows "disconnected" (CrowdSec process doesn't exist)
6. ✅ All subsequent API calls fail because they expect the table to exist

## Why This Wasn't Caught During Development

Looking at the test files, **EVERY TEST** manually calls AutoMigrate:

```go
// backend/internal/services/crowdsec_startup_test.go:75
err = db.AutoMigrate(&models.SecurityConfig{})

// backend/internal/api/handlers/security_handler_coverage_test.go:25
require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, ...))
```

So tests **always create the table fresh**, hiding the issue that would occur in production with a persistent database.

## The Fix

### Option 1: Manual Database Migration (IMMEDIATE FIX)

Run this on the production container:

```bash
# Connect to running container
docker exec -it charon /bin/sh

# Run migration command (create a new CLI command in main.go)
./backend migrate

# OR manually create tables with sqlite3
sqlite3 /app/data/charon.db << EOF
CREATE TABLE IF NOT EXISTS security_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    name TEXT,
    enabled BOOLEAN DEFAULT false,
    admin_whitelist TEXT,
    break_glass_hash TEXT,
    crowdsec_mode TEXT DEFAULT 'disabled',
    crowdsec_api_url TEXT,
    waf_mode TEXT DEFAULT 'disabled',
    waf_rules_source TEXT,
    waf_learning BOOLEAN DEFAULT false,
    waf_paranoia_level INTEGER DEFAULT 1,
    waf_exclusions TEXT,
    rate_limit_mode TEXT DEFAULT 'disabled',
    rate_limit_enable BOOLEAN DEFAULT false,
    rate_limit_burst INTEGER DEFAULT 10,
    rate_limit_requests INTEGER DEFAULT 100,
    rate_limit_window_sec INTEGER DEFAULT 60,
    rate_limit_bypass_list TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS security_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    ip TEXT NOT NULL,
    reason TEXT,
    action TEXT DEFAULT 'ban',
    duration INTEGER,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS security_audits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    event_type TEXT,
    ip_address TEXT,
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS security_rule_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    type TEXT DEFAULT 'ip_list',
    content TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS crowdsec_preset_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT false,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS crowdsec_console_enrollments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    enrollment_key TEXT,
    organization_id TEXT,
    instance_name TEXT,
    enrolled_at DATETIME,
    status TEXT DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
EOF

# Restart container
exit
docker restart charon
```

### Option 2: Add Migration CLI Command (CLEAN SOLUTION)

Add to `backend/cmd/api/main.go`:

```go
// Handle CLI commands
if len(os.Args) > 1 {
    switch os.Args[1] {
    case "migrate":
        cfg, err := config.Load()
        if err != nil {
            log.Fatalf("load config: %v", err)
        }

        db, err := database.Connect(cfg.DatabasePath)
        if err != nil {
            log.Fatalf("connect database: %v", err)
        }

        logger.Log().Info("Running database migrations...")
        if err := db.AutoMigrate(
            &models.SecurityConfig{},
            &models.SecurityDecision{},
            &models.SecurityAudit{},
            &models.SecurityRuleSet{},
            &models.CrowdsecPresetEvent{},
            &models.CrowdsecConsoleEnrollment{},
        ); err != nil {
            log.Fatalf("migration failed: %v", err)
        }

        logger.Log().Info("Migration completed successfully")
        return

    case "reset-password":
        // existing reset-password code
    }
}
```

Then run:

```bash
docker exec charon /app/backend migrate
docker restart charon
```

### Option 3: Nuclear Option - Reset Database (DESTRUCTIVE)

```bash
# BACKUP FIRST
docker exec charon cp /app/data/charon.db /app/data/backups/charon-pre-security-migration.db

# Remove database
rm data/charon.db data/charon.db-shm data/charon.db-wal

# Restart container (will recreate fresh DB with all tables)
docker restart charon
```

## Fix Verification Checklist

After applying any fix, verify:

1. ✅ Check table exists:

   ```bash
   docker exec charon sqlite3 /app/data/charon.db "SELECT name FROM sqlite_master WHERE type='table' AND name='security_configs';"
   ```

   Expected: `security_configs`

2. ✅ Check reconciliation logs:

   ```bash
   docker logs charon 2>&1 | grep -i "crowdsec reconciliation"
   ```

   Expected: "starting CrowdSec" or "already running" (NOT "skipped: SecurityConfig table not found")

3. ✅ Check CrowdSec is running:

   ```bash
   docker exec charon ps aux | grep crowdsec
   ```

   Expected: `crowdsec -c /app/data/crowdsec/config/config.yaml`

4. ✅ Check frontend Console Enrollment:
   - Navigate to `/security` page
   - Click "Console Enrollment" tab
   - Should show CrowdSec status as "Running"

5. ✅ Check toggle state persists:
   - Toggle CrowdSec OFF
   - Refresh page
   - Toggle should remain OFF

## Code Improvements Needed

### 1. Change Debug Log to Warning

**File:** `backend/internal/services/crowdsec_startup.go:35`

```go
// BEFORE (line 35)
logger.Log().Debug("CrowdSec reconciliation skipped: SecurityConfig table not found")

// AFTER
logger.Log().Warn("CrowdSec reconciliation skipped: SecurityConfig table not found - run migrations")
```

**Rationale:** This is NOT a debug-level issue. If the table doesn't exist, it's a critical setup problem that should always be logged, regardless of debug mode.

### 2. Add Startup Migration Check

**File:** `backend/cmd/api/main.go` (after database.Connect())

```go
// Verify critical tables exist before starting server
requiredTables := []interface{}{
    &models.SecurityConfig{},
    &models.SecurityDecision{},
    &models.SecurityAudit{},
    &models.SecurityRuleSet{},
}

for _, model := range requiredTables {
    if !db.Migrator().HasTable(model) {
        logger.Log().Warnf("Missing table for %T - running migration", model)
        if err := db.AutoMigrate(model); err != nil {
            log.Fatalf("failed to migrate %T: %v", model, err)
        }
    }
}
```

### 3. Add Health Check for Tables

**File:** `backend/internal/api/handlers/health.go`

```go
func HealthHandler(c *gin.Context) {
    db := c.MustGet("db").(*gorm.DB)

    health := gin.H{
        "status": "healthy",
        "database": "connected",
        "migrations": checkMigrations(db),
    }

    c.JSON(200, health)
}

func checkMigrations(db *gorm.DB) map[string]bool {
    return map[string]bool{
        "security_configs": db.Migrator().HasTable(&models.SecurityConfig{}),
        "security_decisions": db.Migrator().HasTable(&models.SecurityDecision{}),
        "security_audits": db.Migrator().HasTable(&models.SecurityAudit{}),
        "security_rule_sets": db.Migrator().HasTable(&models.SecurityRuleSet{}),
    }
}
```

## Related Issues

- Frontend toggle stuck in ON position → Database issue (no table to persist state)
- Console Enrollment says "not running" → CrowdSec never started (reconciliation exits)
- Log viewer disconnected → CrowdSec process doesn't exist
- All 7 previous commits failed because they addressed symptoms, not the root cause

## Lessons Learned

1. **Always log critical guard clauses at WARN level or higher** - Debug logs are invisible in production
2. **Verify database state matches code expectations** - AutoMigrate is non-destructive and won't fix missing tables from before the migration code existed
3. **Add database health checks** - Make missing tables visible in /api/v1/health endpoint
4. **Test with persistent databases** - All unit tests use fresh in-memory DBs, hiding this issue
5. **Add migration CLI command** - Allow operators to manually trigger migrations without container restart

## Recommended Action Plan

1. **IMMEDIATE:** Run Option 2 (Add migrate CLI command) and execute migration
2. **SHORT-TERM:** Apply Code Improvements #1 and #2
3. **LONG-TERM:** Add health check endpoint and integration tests with persistent DBs
4. **DOCUMENTATION:** Update deployment docs to mention migration requirement

## Status

- [x] Root cause identified (missing tables due to persistent DB from before migration code)
- [x] Silent exit point found (HasTable check with DEBUG logging)
- [x] Fix options documented
- [ ] Fix implemented
- [ ] Fix verified
- [ ] Code improvements applied
- [ ] Documentation updated
