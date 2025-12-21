---
name: "utility-db-recovery"
version: "1.0.0"
description: "Performs SQLite database integrity checks and recovery operations for Charon database"
author: "Charon Project"
license: "MIT"
tags:
  - "utility"
  - "database"
  - "recovery"
  - "sqlite"
  - "backup"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "sqlite3"
    version: ">=3.0"
    optional: false
environment_variables: []
parameters:
  - name: "--force"
    type: "flag"
    description: "Skip confirmation prompts"
    default: "false"
    required: false
outputs:
  - name: "exit_code"
    type: "integer"
    description: "0 on success, 1 on failure"
  - name: "backup_file"
    type: "file"
    description: "Timestamped backup of database"
    path: "backend/data/backups/charon_backup_*.db"
metadata:
  category: "utility"
  subcategory: "database"
  execution_time: "medium"
  risk_level: "high"
  ci_cd_safe: false
  requires_network: false
  idempotent: false
---

# Utility: Database Recovery

## Overview

Performs comprehensive SQLite database integrity checks and recovery operations for the Charon database. This skill can detect corruption, create backups, and attempt automatic recovery using SQLite's `.dump` and rebuild strategy. Critical for maintaining database health and recovering from corruption.

## Prerequisites

- `sqlite3` command-line tool installed
- Database file exists at expected location
- Write permissions for backup directory
- Sufficient disk space for backups and recovery

## Usage

### Basic Usage (Interactive)

```bash
.github/skills/utility-db-recovery-scripts/run.sh
```

### Force Mode (Non-Interactive)

```bash
.github/skills/utility-db-recovery-scripts/run.sh --force
```

### Via Skill Runner

```bash
.github/skills/scripts/skill-runner.sh utility-db-recovery [--force]
```

### Via VS Code Task

Use the task: **Utility: Database Recovery**

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| --force   | flag | No       | false   | Skip confirmation prompts |
| -f        | flag | No       | false   | Alias for --force |

## Environment Variables

This skill requires no environment variables. It auto-detects Docker vs local environment.

## Outputs

- **Success Exit Code**: 0 - Database healthy or recovered
- **Error Exit Codes**: 1 - Recovery failed or prerequisites missing
- **Backup Files**: `backend/data/backups/charon_backup_YYYYMMDD_HHMMSS.db`
- **Dump Files**: `backend/data/backups/charon_dump_YYYYMMDD_HHMMSS.sql` (if recovery attempted)
- **Recovered DB**: `backend/data/backups/charon_recovered_YYYYMMDD_HHMMSS.db` (temporary)

### Success Output Example (Healthy Database)

```
==============================================
  Charon Database Recovery Tool
==============================================

[INFO] sqlite3 found: 3.40.1
[INFO] Running in local development environment
[INFO] Database path: backend/data/charon.db
[INFO] Created backup directory: backend/data/backups
[INFO] Creating backup: backend/data/backups/charon_backup_20251220_143022.db
[SUCCESS] Backup created successfully

==============================================
  Integrity Check Results
==============================================
[INFO] Running SQLite integrity check...
ok
[SUCCESS] Database integrity check passed!
[INFO] WAL mode already enabled
[INFO] Cleaning up old backups (keeping last 10)...

==============================================
  Summary
==============================================
[SUCCESS] Database is healthy
[INFO] Backup stored at: backend/data/backups/charon_backup_20251220_143022.db
```

### Recovery Output Example (Corrupted Database)

```
==============================================
  Integrity Check Results
==============================================
[INFO] Running SQLite integrity check...
*** in database main ***
Page 15: btreeInitPage() returns error code 11
[ERROR] Database integrity check FAILED

WARNING: Database corruption detected!
This script will attempt to recover the database.
A backup has already been created at: backend/data/backups/charon_backup_20251220_143022.db

Continue with recovery? (y/N): y

==============================================
  Recovery Process
==============================================
[INFO] Attempting database recovery...
[INFO] Exporting database via .dump command...
[SUCCESS] Database dump created: backend/data/backups/charon_dump_20251220_143022.sql
[INFO] Creating new database from dump...
[SUCCESS] Recovered database created: backend/data/backups/charon_recovered_20251220_143022.db
[INFO] Verifying recovered database integrity...
[SUCCESS] Recovered database passed integrity check
[INFO] Replacing original database with recovered version...
[SUCCESS] Database replaced successfully
[INFO] Enabling WAL (Write-Ahead Logging) mode...
[SUCCESS] WAL mode enabled

==============================================
  Summary
==============================================
[SUCCESS] Database recovery completed successfully!
[INFO] Original backup: backend/data/backups/charon_backup_20251220_143022.db
[INFO] Please restart the Charon application
```

## Environment Detection

The skill automatically detects whether it's running in:

1. **Docker Environment**: Database at `/app/data/charon.db`
2. **Local Development**: Database at `backend/data/charon.db`

Backup locations adjust accordingly.

## Recovery Process

When corruption is detected, the recovery process:

1. **Creates Backup**: Timestamped copy of current database (including WAL/SHM)
2. **Exports Data**: Uses `.dump` command to export SQL (works with partial corruption)
3. **Creates New DB**: Builds fresh database from dump
4. **Verifies Integrity**: Runs integrity check on recovered database
5. **Replaces Original**: Moves recovered database to original location
6. **Enables WAL Mode**: Configures Write-Ahead Logging for durability
7. **Cleanup**: Removes old backups (keeps last 10)

## When to Use This Skill

Use this skill when:
- Application fails to start with database errors
- SQLite reports "database disk image is malformed"
- Random crashes or data inconsistencies
- After unclean shutdown (power loss, kill -9)
- Before major database migrations
- As part of regular maintenance schedule

## Backup Management

- **Automatic Backups**: Created before any recovery operation
- **Retention**: Keeps last 10 backups automatically
- **Includes WAL/SHM**: Backs up Write-Ahead Log files if present
- **Timestamped**: Format `charon_backup_YYYYMMDD_HHMMSS.db`

## WAL Mode

The skill ensures Write-Ahead Logging (WAL) is enabled:
- **Benefits**: Better concurrency, atomic commits, crash resistance
- **Trade-offs**: Multiple files (db, wal, shm) instead of single file
- **Recommended**: For all production deployments

## Examples

### Example 1: Regular Health Check

```bash
# Run integrity check (creates backup even if healthy)
.github/skills/utility-db-recovery-scripts/run.sh
```

### Example 2: Force Recovery Without Prompts

```bash
# Useful for automation/scripts
.github/skills/utility-db-recovery-scripts/run.sh --force
```

### Example 3: Docker Container Recovery

```bash
# Run inside Docker container
docker exec -it charon-app bash
/app/.github/skills/utility-db-recovery-scripts/run.sh --force
```

## Error Handling

- **No sqlite3**: Exits with installation instructions
- **Database not found**: Exits with clear error message
- **Dump fails**: Recovery aborted, backup preserved
- **Recovered DB fails integrity**: Original backup preserved
- **Insufficient disk space**: Operations fail safely

## Post-Recovery Steps

After successful recovery:

1. **Restart Application**: `docker compose restart` or restart process
2. **Verify Functionality**: Test critical features
3. **Monitor Logs**: Watch for any residual issues
4. **Review Backup**: Keep the backup until stability confirmed
5. **Investigate Root Cause**: Determine what caused corruption

## Related Skills

- [docker-start-dev](./docker-start-dev.SKILL.md) - Restart containers after recovery
- [docker-stop-dev](./docker-stop-dev.SKILL.md) - Stop containers before recovery

## Notes

- **High Risk**: Destructive operation, always creates backup first
- **Not CI/CD Safe**: Requires user interaction (unless --force)
- **Not Idempotent**: Each run creates new backup
- **Manual Intervention**: Some corruption may require manual SQL fixes
- **WAL Files**: Don't delete WAL/SHM files manually during operation
- **Backup Location**: Ensure backups are stored on different disk from database

## Troubleshooting

### Recovery Fails with Empty Dump

- Database may be too corrupted
- Try `.recover` command (SQLite 3.29+)
- Restore from external backup

### "Database is Locked" Error

- Stop application first
- Check for other processes accessing database
- Use `fuser backend/data/charon.db` to find processes

### Recovery Succeeds but Data Missing

- Some corruption may result in data loss
- Review backup before deleting
- Check dump SQL file for missing tables

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `scripts/db-recovery.sh`
