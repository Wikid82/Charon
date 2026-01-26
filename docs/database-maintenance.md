---
title: Database Maintenance
description: SQLite database maintenance guide for Charon. Covers backups, recovery, and troubleshooting database issues.
---

## Database Maintenance

Charon uses SQLite as its embedded database. This guide explains how the database
is configured, how to maintain it, and what to do if something goes wrong.

---

## Overview

### Why SQLite?

SQLite is perfect for Charon because:

- **Zero setup** — No external database server needed
- **Portable** — One file contains everything
- **Reliable** — Used by billions of devices worldwide
- **Fast** — Local file access beats network calls

### Where Is My Data?

| Environment | Database Location |
|-------------|-------------------|
| Docker | `/app/data/charon.db` |
| Local dev | `backend/data/charon.db` |

You may also see these files next to the database:

- `charon.db-wal` — Write-Ahead Log (temporary transactions)
- `charon.db-shm` — Shared memory file (temporary)

**Don't delete the WAL or SHM files while Charon is running!**
They contain pending transactions.

---

## Database Configuration

Charon automatically configures SQLite with optimized settings:

| Setting | Value | What It Does |
|---------|-------|--------------|
| `journal_mode` | WAL | Enables concurrent reads while writing |
| `busy_timeout` | 5000ms | Waits 5 seconds before failing on lock |
| `synchronous` | NORMAL | Balanced safety and speed |
| `cache_size` | 64MB | Memory cache for faster queries |

### What Is WAL Mode?

**WAL (Write-Ahead Logging)** is a more modern journaling mode for SQLite that:

- ✅ Allows readers while writing (no blocking)
- ✅ Faster for most workloads
- ✅ Reduces disk I/O
- ✅ Safer crash recovery

Charon enables WAL mode automatically — you don't need to do anything.

---

## Backups

### Automatic Backups

Charon creates automatic backups before destructive operations (like deleting hosts).
These are stored in:

| Environment | Backup Location |
|-------------|-----------------|
| Docker | `/app/data/backups/` |
| Local dev | `backend/data/backups/` |

### Manual Backups

To create a manual backup:

```bash
# Docker
docker exec charon cp /app/data/charon.db /app/data/backups/manual_backup.db

# Local development
cp backend/data/charon.db backend/data/backups/manual_backup.db
```

**Important:** If WAL mode is active, also copy the `-wal` and `-shm` files:

```bash
cp backend/data/charon.db-wal backend/data/backups/manual_backup.db-wal
cp backend/data/charon.db-shm backend/data/backups/manual_backup.db-shm
```

Or use the recovery script which handles this automatically (see below).

---

## Database Recovery

If your database becomes corrupted (rare, but possible after power loss or
disk failure), Charon includes a recovery script.

### When to Use Recovery

Use the recovery script if you see errors like:

- "database disk image is malformed"
- "database is locked" (persists after restart)
- "SQLITE_CORRUPT"
- Application won't start due to database errors

### Running the Recovery Script

**In Docker:**

```bash
# First, stop Charon to release database locks
docker stop charon

# Run recovery (from host)
docker run --rm -v charon_data:/app/data charon:latest /app/scripts/db-recovery.sh

# Restart Charon
docker start charon
```

**Local Development:**

```bash
# Make sure Charon is not running, then:
./scripts/db-recovery.sh
```

**Force mode (skip confirmations):**

```bash
./scripts/db-recovery.sh --force
```

### What the Recovery Script Does

1. **Creates a backup** — Saves your current database before any changes
2. **Runs integrity check** — Uses SQLite's `PRAGMA integrity_check`
3. **If healthy** — Confirms database is OK, enables WAL mode
4. **If corrupted** — Attempts automatic recovery:
   - Exports data using SQLite `.dump` command
   - Creates a new database from the dump
   - Verifies the new database integrity
   - Replaces the old database with the recovered one
5. **Cleans up** — Removes old backups (keeps last 10)

### Recovery Output Example

**Healthy database:**

```
==============================================
  Charon Database Recovery Tool
==============================================

[INFO] sqlite3 found: 3.40.1
[INFO] Running in Docker environment
[INFO] Database path: /app/data/charon.db
[INFO] Creating backup: /app/data/backups/charon_backup_20250101_120000.db
[SUCCESS] Backup created successfully

==============================================
  Integrity Check Results
==============================================
ok
[SUCCESS] Database integrity check passed!
[INFO] WAL mode already enabled

==============================================
  Summary
==============================================
[SUCCESS] Database is healthy
[INFO] Backup stored at: /app/data/backups/charon_backup_20250101_120000.db
```

**Corrupted database (with successful recovery):**

```
==============================================
  Integrity Check Results
==============================================
*** in database main ***
Page 42: btree page count invalid
[ERROR] Database integrity check FAILED

WARNING: Database corruption detected!
This script will attempt to recover the database.
A backup has already been created.

Continue with recovery? (y/N): y

==============================================
  Recovery Process
==============================================
[INFO] Attempting database recovery...
[INFO] Exporting database via .dump command...
[SUCCESS] Database dump created
[INFO] Creating new database from dump...
[SUCCESS] Recovered database created
[SUCCESS] Recovered database passed integrity check
[INFO] Replacing original database with recovered version...
[SUCCESS] Database replaced successfully

==============================================
  Summary
==============================================
[SUCCESS] Database recovery completed successfully!
[INFO] Please restart the Charon application
```

---

## Preventive Measures

### Do

- ✅ **Keep regular backups** — Use the backup page in Charon or manual copies
- ✅ **Use proper shutdown** — Stop Charon gracefully (`docker stop charon`)
- ✅ **Monitor disk space** — SQLite needs space for temporary files
- ✅ **Use reliable storage** — SSDs are more reliable than HDDs

### Don't

- ❌ **Don't kill Charon** — Avoid `docker kill` or `kill -9` (use `stop` instead)
- ❌ **Don't edit the database manually** — Unless you know SQLite well
- ❌ **Don't delete WAL files** — While Charon is running
- ❌ **Don't run out of disk space** — Can cause corruption

---

## Troubleshooting

### "Database is locked"

**Cause:** Another process has the database open.

**Fix:**

1. Stop all Charon instances
2. Check for zombie processes: `ps aux | grep charon`
3. Kill any remaining processes
4. Restart Charon

### "Database disk image is malformed"

**Cause:** Database corruption (power loss, disk failure, etc.)

**Fix:**

1. Stop Charon
2. Run the recovery script: `./scripts/db-recovery.sh`
3. Restart Charon

### "SQLITE_BUSY"

**Cause:** Long-running transaction blocking others.

**Fix:** Usually resolves itself (5-second timeout). If persistent:

1. Restart Charon
2. If still occurring, check for stuck processes

### WAL File Is Very Large

**Cause:** Many writes without checkpointing.

**Fix:** This is usually handled automatically. To force a checkpoint:

```bash
sqlite3 /path/to/charon.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

### Lost Data After Recovery

**What happened:** The `.dump` command recovers readable data, but severely
corrupted records may be lost.

**What to do:**

1. Check your automatic backups in `data/backups/`
2. Restore from the most recent pre-corruption backup
3. Re-create any missing configuration manually

---

## Advanced: Manual Recovery

If the automatic script fails, you can try manual recovery:

```bash
# 1. Create a SQL dump of whatever is readable
sqlite3 charon.db ".dump" > backup.sql

# 2. Check what was exported
head -100 backup.sql

# 3. Create a new database
sqlite3 charon_new.db < backup.sql

# 4. Verify the new database
sqlite3 charon_new.db "PRAGMA integrity_check;"

# 5. If OK, replace the old database
mv charon.db charon_corrupted.db
mv charon_new.db charon.db

# 6. Enable WAL mode on the new database
sqlite3 charon.db "PRAGMA journal_mode=WAL;"
```

---

## Need Help?

If recovery fails or you're unsure what to do:

1. **Don't panic** — Your backup was created before recovery attempts
2. **Check backups** — Look in `data/backups/` for recent copies
3. **Ask for help** — Open an issue on [GitHub](https://github.com/Wikid82/charon/issues)
   with your error messages
