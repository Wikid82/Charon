---
title: Backup & Restore
description: Easy configuration backup and restoration
---

# Backup & Restore

Your configuration is valuable. Charon makes it easy to backup your entire setup and restore it when needed—whether you're migrating to new hardware or recovering from a problem.

## Overview

Charon provides configuration backups and validated restore functionality. Your proxy hosts, SSL certificates, access lists, users, settings, and CrowdSec configuration are all preserved, so you can recover quickly from a problem or move to new hardware.

Backups are `.zip` archives (optionally encrypted, see [Encrypting Backups](#encrypting-backups) below) stored in the Charon data directory. Every archive includes a manifest with checksums for its contents, so Charon can tell — before touching any live data — whether a file is complete and intact. You can download any backup for off-site storage, or have Charon copy it to remote storage automatically (see [Remote Storage](#remote-storage) below).

## Why Use This

- **Disaster Recovery**: Restore your entire configuration after a failure
- **Migration Made Easy**: Move to new hardware without reconfiguring
- **Change Confidence**: Make changes knowing you can roll back
- **Off-Site Copies**: Push backups to S3 or SFTP storage automatically

## What Gets Backed Up

| Component | Included |
|-----------|----------|
| **Database** | All proxy hosts, redirects, streams, 404 hosts, DNS provider credentials, remote storage targets, and tunnel configs |
| **SSL Certificates** | Let's Encrypt certificates and custom certificates |
| **Access Lists** | All access control configurations |
| **Users** | User accounts and permissions |
| **Settings** | Application preferences and configurations |
| **CrowdSec Config** | Security settings and custom rules |

## Creating Backups

### Automatic Backups

Charon creates a **scheduled** backup on the interval you configure (daily or weekly presets, or a custom cron expression) — see [Scheduling Backups](#scheduling-backups). It also creates an automatic **pre-restore safety backup** of your current state immediately before every restore, so a bad restore can always be undone; this pre-restore backup is never counted toward your regular retention limit.

Charon does **not** currently create an automatic backup before every general configuration change or before version upgrades — only before a restore. If you're about to make a risky change, create a manual backup first.

### Manual Backups

To create a manual backup:

1. Navigate to **Tasks** → **Backups**
2. Click **Create Backup**
3. Optionally check **Encrypt this backup** and set a passphrase (see [Encrypting Backups](#encrypting-backups))
4. Optionally download the backup file for off-site storage

## Restoring from Backup

To restore a previous configuration:

1. Navigate to **Tasks** → **Backups**
2. Select the backup to restore from the list
3. Click **Restore**, enter the archive passphrase if it's encrypted
4. Click **Validate** to check the archive is intact before committing (recommended, not required)
5. Click **Restore** to confirm

Charon validates the archive, takes a safety backup of your current state, restores the new data, and reloads Caddy and CrowdSec. Most restores finish without any downtime. Occasionally a restore can't finish live and reports that a restart is required to complete it — see [Disaster Recovery: Restart-Required Restores](disaster-recovery.md#restart-required-restores-boot-swap-behavior) for exactly what that means and why it's expected behavior, not a failure.

Have a backup file that isn't in the list — downloaded from remote storage, or from another machine? Use **Upload Backup** on the same page instead of steps 1–2; see [Disaster Recovery](disaster-recovery.md) for the full off-host and legacy-file walkthroughs.

> **Note**: Restoring a backup overwrites your current configuration. Charon takes an automatic safety backup first, but if you're unsure, create your own manual backup before restoring too.

## Backup Retention

Charon prunes old backups by **count**, not by age:

- **Local backups**: the most recent N are kept (default 7), configurable as **Local Retention Count**
- **Remote copies**: the most recent N uploaded to each remote target are kept (default 7), configurable as **Remote Retention Count**
- **Pre-restore safety backups**: never auto-pruned by the regular retention count — delete them manually once you're confident you no longer need them

Configure retention counts in **Tasks** → **Backups** → **Backup Schedule**.

## Scheduling Backups

Under **Tasks** → **Backups** → **Backup Schedule**, you can:

- Turn automatic backups on or off
- Choose **Daily** or **Weekly**, or drop into a **Custom** cron expression for anything else
- Set the local and remote retention counts described above

## Encrypting Backups

Backup encryption is **off by default**. When enabled, Charon encrypts the entire archive with a passphrase before it's written to disk or uploaded anywhere, using audited, industry-standard passphrase encryption (age/scrypt) — the same category of encryption used to protect files people can't afford to have read by anyone else.

**There is no way to recover an encrypted backup if you lose the passphrase.** Store it in a password manager. See [Disaster Recovery: Lost Your Archive Passphrase?](disaster-recovery.md#lost-your-archive-passphrase-read-this-first) for the full explanation.

Separately from archive encryption, sensitive database fields (DNS provider credentials, remote storage secrets, tunnel configs) are always encrypted at rest using your `CHARON_ENCRYPTION_KEY`, whether or not you turn on archive encryption. Restoring a backup on a different host requires that host to have the same key — see [Disaster Recovery: The Encryption Key Problem](disaster-recovery.md#the-encryption-key-problem).

## Remote Storage

Charon can copy every scheduled (and manual) backup to an S3-compatible bucket or an SFTP server automatically. Configure this under **Tasks** → **Backups** → **Remote Storage Targets**:

- **S3**: works with AWS S3 and S3-compatible services (MinIO, Backblaze B2, Cloudflare R2, etc.)
- **SFTP**: connects to any SSH server; the server's host key must be confirmed once before Charon will send any credentials to it, so a spoofed or swapped server can't intercept your login

Use **Test Connection** after saving a target to confirm Charon can reach it before relying on it. Upload failures never block or fail the underlying backup — they're recorded against that backup's remote-copy status so you can see at a glance whether your off-site copy actually landed.

This Beta release does not yet have an in-app "restore straight from a remote target" button — see [Disaster Recovery: Getting a Backup Down From Remote Storage](disaster-recovery.md#getting-a-backup-down-from-remote-storage) for how to fetch a file down manually and restore it.

## Best Practices

1. **Configure a remote storage target** so a local disk failure can't take your backups with it
2. **Test restores** periodically to ensure backups are valid
3. **Backup before changes** when modifying critical configurations
4. **Store your archive passphrase** (if used) in a password manager, not next to the backup file

## Related

- [Disaster Recovery](disaster-recovery.md) — cold restores, moving to new hardware, and recovering from remote storage
- [Encryption Key Rotation](key-rotation.md)
- [Zero-Downtime Updates](live-reload.md)
- [Back to Features](../features.md)
