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

## Automatic Optimization

Over time, deleting old data (for example old uptime history) leaves empty space
inside the database file. Think of a cupboard where you removed half the plates:
the cupboard is still the same size. Charon takes care of this for you.

### What Charon Does on Its Own

- **New databases** stay small by themselves. Nothing to do.
- **Existing databases** that hold a lot of reusable space are optimized
  automatically **when Charon starts** (for example after an update), and only
  when it is worthwhile: at least 100 MB can be given back, and either at least
  one fifth of the file is empty space or at least 1 GB can be given back.
- Smaller databases are left alone. Charon never optimizes for less than 100 MB.
- After that, Charon keeps the file trimmed in the background while it removes
  old uptime history. You do not need to do anything.

### What You Will See

- **Your proxies keep running.** The reverse proxy already has its settings, so
  your websites stay online while the database is optimized.
- The Charon web page shows a short **"Optimizing the database"** page for a few
  minutes (longer for very large files), with a running timer. It goes back to
  normal by itself.
- The emergency (break-glass) access is briefly unavailable during this time too.
- If you stop the container while it is optimizing, it is **safe**. Your data is
  not harmed, and Charon simply tries again at the next start, up to 3 tries.
  - The very last step (copying the optimized data back) cannot be interrupted
    instantly. If the container stops during that step, it can use up one of the
    3 tries.
  - Your database is intact either way, because the operation is all-or-nothing.
    The next start simply tries again.

### The Database Card

In **System Settings** there is a quiet **Database** card that shows the database
size and how much space could be reclaimed.

- A short note appears when Charon plans to shrink the database at the next
  start. Nothing to do.
- Press **Reclaim space on next restart** if you want it done at your next
  restart. Changed your mind? Press **Undo**.
- A warning appears only if something needs your attention:
  - **Not enough free disk space to optimize the database.** The optimization
    needs free disk space of roughly **twice your actual data** while it works.
    The warning tells you how much to free up. Free up that space (delete old
    backups or other files on the same disk), then restart Charon. It tries
    again at every start.
  - **Database optimization stopped.** After 3 failed tries Charon stops trying.
    Make sure there is enough free disk space and that the container is not
    being killed during startup, then press **Reclaim space on next restart**.
  - A note that optimization was postponed because the database was busy. It is
    retried at the next start.

### Restoring a Backup

Replacing the database file with a backup file is detected automatically, and
Charon starts with a clean slate. But if you restore by copying rows from a
backup into your existing database, you can also bring back an old "tries so
far" counter or an old "in progress" marker from that backup. If optimization
seems stuck or stopped after a restore, press **Reclaim space on next restart**
in **System Settings**. That resets the counter.

### Turning It Off

Set this environment variable if you never want automatic optimization:

```yaml
environment:
  - CHARON_DB_COMPACT_ON_START=off
```

| Value | Meaning |
|-------|---------|
| `auto` (default) | Optimize at start when worthwhile |
| `off` | Never optimize at start. Emergency access stays available during startup |

While it is `off`, the Database card shows that the setting is disabled, and any
earlier request you made is kept and applies again once you remove the setting.

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

## Advanced: Shrinking the Database by Hand (Fallback)

The automatic optimization above makes this **rarely necessary**. Only use it if
automatic optimization is turned off or keeps failing, the file is very large,
and you are comfortable with the command line.

1. **Stop Charon first.** Never do this while it is running.
2. **Back up the database.** Copy `charon.db` together with `charon.db-wal` and
   `charon.db-shm` (if they exist) while Charon is stopped, or run
   `sqlite3 charon.db ".backup charon-backup.db"`.
3. Make sure you have free disk space of about **twice your actual data** (not
   twice the file size).
4. **Find out who owns your database file.** In your data folder, run
   `ls -ln charon.db`. The two numbers after the permissions (usually
   `1000 1000`) are the user and group Charon runs as. Use those numbers
   below in place of `1000:1000` if yours are different.
5. **Run a one-off container** that opens the database directly (it skips the
   normal Charon startup, which is why `--entrypoint sqlite3` is needed).
   Replace `/path/to/your/charon/data` with your data folder, and use the same
   image you normally run:

   ```bash
   docker run --rm -it --user 1000:1000 --entrypoint sqlite3 \
     -v /path/to/your/charon/data:/app/data \
     wikid82/charon:latest /app/data/charon.db
   ```

   At the `sqlite>` prompt, type these lines one at a time:

   ```sql
   PRAGMA auto_vacuum=INCREMENTAL;
   VACUUM;
   .quit
   ```

   The `PRAGMA` line lets the file shrink more easily in the future. `VACUUM;`
   can take a while on a big file, so wait for the prompt to come back.
6. Start Charon again and check that the dashboard opens. If it cannot open the
   database, check file ownership.

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

**Fix:** This is usually handled automatically (Charon also checkpoints after optimizing). To force a checkpoint by hand:

```bash
sqlite3 /path/to/charon.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

### Stuck on "Optimizing the database"

**Cause:** A large database takes a few minutes to optimize.

**Fix:** Wait; the page shows a running timer and returns to normal on its own.
Your proxies keep working meanwhile. If you cannot wait, stopping the container is
safe and Charon retries at the next start. To skip it, set
`CHARON_DB_COMPACT_ON_START=off`.

### Not enough disk space to optimize the database

**Cause:** The Database card in **System Settings** shows this warning when the
disk is too full to optimize safely. Charon needs free space of roughly **twice
your actual data** while it works.

**Fix:**

1. Free up the amount the warning asks for (delete old backups or other files on
   the same disk).
2. Restart Charon. It tries again at every start, so nothing else is needed.

### Database optimization stopped after 3 attempts

**Cause:** Charon gave up after 3 failed tries. Usually there was not enough free
disk space, or the container was killed during startup.

**Fix:**

1. Free up disk space (see the entry above).
2. Make sure nothing is killing the container while it starts.
3. In **System Settings**, press **Reclaim space on next restart**, then restart
   Charon.

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
