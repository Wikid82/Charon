---
title: Disaster Recovery
description: How to restore Charon after a total failure, on a brand-new machine, or from an old backup file
---

# Disaster Recovery

This guide is for the day something goes badly wrong — the host died, a disk failed,
or you're moving Charon to new hardware and need to bring your configuration back
from a backup file. If you just want the everyday "create/restore a backup" workflow,
see [Backup & Restore](backup-restore.md); this page covers the harder cases.

## Table of Contents

- [Before You Start](#before-you-start)
- [Cold Restore (same machine, app is down)](#cold-restore-same-machine-app-is-down)
- [Off-Host Restore (new machine or container)](#off-host-restore-new-machine-or-container)
- [Getting a Backup Down From Remote Storage](#getting-a-backup-down-from-remote-storage)
- [The Encryption Key Problem](#the-encryption-key-problem)
- [Lost Your Archive Passphrase? Read This First](#lost-your-archive-passphrase-read-this-first)
- [Restart-Required Restores (Boot-Swap Behavior)](#restart-required-restores-boot-swap-behavior)
- [If a Restore Fails Completely](#if-a-restore-fails-completely)
- [Legacy and Upload-Only Backups](#legacy-and-upload-only-backups)
- [Quick Reference](#quick-reference)

---

## Before You Start

Every Charon backup archive is a `.zip` (or `.zip.age` if encrypted) that contains:

- Your full database snapshot — proxy hosts, users, settings, DNS provider
  credentials, remote storage target secrets, and any Orthrus/Hecate tunnel
  configuration.
- Your Caddy data (TLS certificates and Caddy's own state).
- Your CrowdSec configuration.
- A `manifest.json` describing exactly what's inside and a checksum for every file,
  so Charon can tell if the archive is corrupt before touching anything.

Because the database snapshot includes password hashes and certificate private
keys, **treat every backup file — encrypted or not — as a secret.** Store it
somewhere only you can reach.

## Cold Restore (same machine, app is down)

Use this when Charon itself won't start, but the machine and its disk are fine.

1. Make sure the Charon container/binary is stopped.
2. Locate a good backup file. If it's still sitting in your data directory's
   `backups/` folder, you're set. If you only have a copy you downloaded earlier,
   copy it back into that `backups/` folder.
3. Start Charon normally.
4. Once it's up, log in as an admin and go to **Tasks → Backups**.
5. Find the backup in the list, click **Restore**, click **Validate** first to
   confirm the archive isn't corrupt, then click **Restore**.
6. If the restore response says a restart is required (see
   [Restart-Required Restores](#restart-required-restores-boot-swap-behavior)
   below), restart the container/process one more time to finish the job.

If Charon won't even start well enough to log in, you likely need the
[off-host procedure](#off-host-restore-new-machine-or-container) below — restoring
onto a fresh data directory and then swapping it in.

## Off-Host Restore (new machine or container)

Use this when you're standing up Charon on hardware (or a container) that has never
run it before — a full migration, or rebuilding after the original disk is gone.

1. **Install Charon fresh** on the new machine (same Docker image / binary you were
   running before, or newer — restoring a backup made by an older version onto a
   newer Charon works; restoring a backup made by a *newer* version onto an *older*
   Charon is refused).
2. **Set the same `CHARON_ENCRYPTION_KEY`** (and any `_V1`–`_V10` rotation keys) that
   was configured on the original host, *before* you restore. See
   [The Encryption Key Problem](#the-encryption-key-problem) — skipping this step
   doesn't break the restore, but it silently leaves some of your data unusable.
3. Start Charon so it creates its default data directory and admin account, then log
   in as an admin.
4. Copy your backup file onto the new machine, then either:
   - Place it directly in the new instance's `data/backups/` folder if you have
     filesystem access, and it will appear in **Tasks → Backups**, or
   - Use the **Upload Backup** button on the **Tasks → Backups** page to hand it to
     Charon directly — this is the only path for files you don't have direct
     filesystem access to place, and it's also how you restore old (pre-format-v2 or
     raw `.db`) files (see [Legacy and Upload-Only Backups](#legacy-and-upload-only-backups)).
5. Click **Validate**, review any warnings (legacy format, encryption key required),
   then click **Restore**.
6. If a restart is required to finish, restart the container/process once more.

Your proxy hosts, certificates, users, and settings from the original host now
belong to the new one.

## Getting a Backup Down From Remote Storage

If you've configured an S3 or SFTP remote storage target (**Tasks → Backups →
Remote Storage Targets**), Charon automatically uploads a copy of every scheduled
backup there. In this Beta release there is **no in-app "restore straight from
remote" button** — you have to fetch the file yourself first, then feed it to
Charon through the normal restore or upload flow described above. There is nothing
Charon-specific about this step; use whatever tool you'd normally use to talk to
that storage:

**From an S3-compatible bucket (AWS S3, MinIO, Backblaze B2, Cloudflare R2, etc.):**

```bash
# Using the AWS CLI (works against any S3-compatible endpoint with --endpoint-url)
aws s3 cp s3://your-bucket/path-prefix/backup_2026-07-05_03-00-00.zip ./backup_2026-07-05_03-00-00.zip

# Or the MinIO client
mc cp myminio/your-bucket/path-prefix/backup_2026-07-05_03-00-00.zip .
```

**From an SFTP remote target:**

```bash
sftp charon@nas.lan:/backups/charon/backup_2026-07-05_03-00-00.zip .
# or
scp charon@nas.lan:/backups/charon/backup_2026-07-05_03-00-00.zip .
```

Once the file is local, follow [Off-Host Restore](#off-host-restore-new-machine-or-container)
step 4 onward (place it in `data/backups/` or use **Upload Backup**).

## The Encryption Key Problem

This is the single most common way an off-host restore "half-works."

Charon encrypts a handful of sensitive database fields at rest — DNS provider API
credentials, remote storage target secrets, and tunnel (Orthrus/Hecate) configs —
using `CHARON_ENCRYPTION_KEY`. That key **lives in your environment variables, not
in the backup**. It is deliberately never bundled into the archive, because doing
so would make the encryption pointless.

What this means in practice:

- If you restore a backup onto a host that has the **same** `CHARON_ENCRYPTION_KEY`
  (and any rotation keys, `CHARON_ENCRYPTION_KEY_V1` through `_V10`, that were ever
  used to encrypt data on the original host) — everything decrypts normally. Nothing
  to do.
- If you restore onto a host with a **different or missing** key, the restore itself
  still succeeds — your proxy hosts, certificates, and users all come back fine —
  but those specific encrypted rows (DNS provider credentials, remote storage
  secrets, tunnel configs) **silently fail to decrypt**. They won't error loudly;
  they just won't work, and you'll need to re-enter them by hand.
- Charon tries to warn you about this rather than let you find out the hard way.
  Every backup's manifest carries an `encryption_key_required` flag, and when it's
  set, both the **Validate** and **Restore** screens show a warning that the archive
  contains data that needs a specific encryption key to come back to life.

**Bottom line:** before restoring onto a new host, copy over the exact
`CHARON_ENCRYPTION_KEY` (and rotation keys) from the original host's environment.
See [Encryption Key Rotation](key-rotation.md) for the full key-versioning model.

## Lost Your Archive Passphrase? Read This First

Separately from the database-level encryption key above, you can optionally
password-protect an entire backup **archive** with a passphrase (Tasks → Backups →
Backup Encryption, or per-backup when creating one manually). This is off by
default.

**There is no recovery mechanism for a lost archive passphrase.** Charon uses
industry-standard passphrase-based encryption (age/scrypt) for these archives, the
same kind of encryption that protects your bank's TLS traffic — which also means
there is no back door, no "reset my passphrase" button, and no support ticket that
gets your data back. If you forget the passphrase, that specific backup file is
gone, permanently, exactly as if you'd deleted it.

Practical takeaways:

- Store the passphrase in a password manager, not a sticky note or a file next to
  the backup itself.
- If you lose it, don't waste time — start over from your next-oldest unencrypted
  (or passphrase-you-still-remember) backup instead.
- This is exactly why encryption is off by default: it protects your backups from
  someone who steals the file, at the cost of making the file worthless to you too
  if you lose the key.

## Restart-Required Restores (Boot-Swap Behavior)

Most restores finish live — you click Restore, Charon swaps your data in while
still running, reloads Caddy and CrowdSec, and you keep working. No restart needed.

Occasionally (the database was briefly busy, or under heavy load) that live swap
can't complete safely, and Charon falls back to a documented, two-part process
instead of just guessing:

1. **Right away:** Charon writes your restored configuration to Caddy and CrowdSec
   immediately, so your reverse proxy is already running with the *new* (restored)
   routing, certificates, and security rules.
2. **On the next restart:** the actual database file finishes swapping in. Charon
   checks the restored database's integrity one more time before installing it —
   if for any reason it looks corrupt, Charon leaves your previous database in place
   and keeps the pending file for a look, rather than installing something broken.

Between those two points, there's a real (but bounded and expected) window where
Caddy/CrowdSec are already running the restored configuration while the database
Charon's UI reads from is still the pre-restore one. **This is normal, documented
behavior for this fallback path, not a bug** — the restore screen tells you plainly
when this has happened (`restart_required` and `database_swap_pending` in the
response, surfaced as a "restart to finish restoring" message in the UI). Just
restart the container/process once, and the database catches up.

You'll be able to see the outcome of a pending swap by checking **Tasks → Backups**
again after the restart — the backup entry's status updates to reflect whether the
swap completed successfully.

## If a Restore Fails Completely

Restores are designed to always land in one of two good outcomes: finished live, or
queued to finish on your next restart (see above). In a genuine double-failure —
for example, the disk is full or a permissions problem stops Charon from even
writing the "finish this on restart" marker — Charon shows you a clear error
message instead of telling you the restore succeeded. Nothing was restored in
that case, and you'll know it.

If you see this error:

1. Check what stopped it — usually free disk space or file permissions on your
   data directory.
2. Fix the underlying problem.
3. Try the restore again from the same backup file.
4. If you'd rather back out entirely, Charon already made an automatic safety
   backup of your previous state right before the restore attempt (see
   [Backup Retention](backup-restore.md#backup-retention)) — restore that to get
   back to exactly where you were before you started.

That safety backup is never deleted by Charon's automatic cleanup, no matter how
aggressive your retention settings are, so it will still be there if you need it.

## Legacy and Upload-Only Backups

Charon has gone through a few backup formats over time. All of them can still be
restored, but only through specific paths:

| What you have | How to restore it |
|---|---|
| A current backup (`.zip` with a manifest, or `.zip.age` if encrypted) | Normal restore flow, from the list or via Upload |
| An older `.zip` backup with no manifest inside ("legacy format") | Still restorable — Charon logs a warning and skips the checksum step, since there's nothing to check against, but still verifies the database itself is intact |
| A raw `.db` file from a very old Charon version | **Upload only** — use the **Upload Backup** button on **Tasks → Backups**; Charon detects it by its SQLite file signature, wraps it into a proper archive, and restores it from there. These files are never picked up automatically from disk — they have to be uploaded |

## Quick Reference

| Situation | What to do |
|---|---|
| App down, same machine | [Cold restore](#cold-restore-same-machine-app-is-down) |
| New machine / container | [Off-host restore](#off-host-restore-new-machine-or-container) — set `CHARON_ENCRYPTION_KEY` first |
| Backup only exists on S3/SFTP | [Fetch it manually](#getting-a-backup-down-from-remote-storage), then restore locally |
| DNS/remote-storage credentials didn't come back after restore | [Encryption key mismatch](#the-encryption-key-problem) — re-enter them, or set the matching key and restore again |
| Forgot the archive passphrase | [Unrecoverable](#lost-your-archive-passphrase-read-this-first) — use an older backup instead |
| Restore says "restart required" | [Restart once](#restart-required-restores-boot-swap-behavior) — the database finishes swapping in on boot |
| Restore shows an error instead of success | [Real failure](#if-a-restore-fails-completely) — fix the underlying problem (often disk space or permissions) and retry, or restore the automatic safety backup |
| Only have an old `.db` file | [Use Upload Backup](#legacy-and-upload-only-backups) |

## Related

- [Backup & Restore](backup-restore.md) — everyday backup creation, scheduling, and remote storage setup
- [Encryption Key Rotation](key-rotation.md) — how `CHARON_ENCRYPTION_KEY` and its rotation keys work
- [Back to Features](../features.md)
