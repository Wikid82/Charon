---
title: "Manual Testing: Restore-Reliability Remediation (PR #1136 Follow-Up)"
labels:
  - testing
  - bug
  - backend
  - frontend
priority: high
milestone: "v0.4.0"
assignees: []
---

# Manual Testing: Restore-Reliability Remediation (PR #1136 Follow-Up)

## Description

Manual testing plan for the restore-reliability fixes that landed on
`feature/backuprestore` (PR #1136) after a second, adversarial pre-merge
audit. These fixes make Charon's restore flow tell the truth about failure
instead of quietly reporting success, make the underlying database swap
all-or-nothing instead of partially-applied, and prove (with real tests)
that three existing safety nets actually catch the problems they claim to
catch. See `docs/plans/current_spec.md` for the full technical background
and `docs/reports/pre_merge_audit_2026-07-14.md` for the original findings
this remediates (C1, H1, H2, M4, M5). This plan supplements, not replaces,
the automated Definition-of-Done coverage already run for these commits.

There is no user-visible feature change here — this is a reliability/bugfix
pass on the already-documented [Backup & Restore](../features/backup-restore.md)
and [Disaster Recovery](../features/disaster-recovery.md) features. The goal
of this manual pass is to confirm the fixes behave correctly against a real
running instance, not just in unit tests.

## Prerequisites

- A local or staging Charon instance you're comfortable breaking (do not
  run this against a production instance).
- Filesystem access to the instance's data directory, so you can simulate a
  disk-full or permissions failure.
- At least one backup archive already created, ideally with a couple of
  proxy hosts, a user, and (for the encryption-key test) a DNS provider
  credential, a tunnel config, or a remote storage target configured.
- A second Charon instance (or the same instance with `CHARON_ENCRYPTION_KEY`
  changed/unset) for the cross-host encryption-key test.

## Test Cases

### A. Genuine Restore Failure Shows a Real Error (C1)

- [ ] Create a manual backup of the current configuration.
- [ ] Simulate a disk-full condition on the data directory (e.g. mount a
      small tmpfs/loop device as the data directory, or fill the disk to
      within a few KB of capacity) so Charon cannot write its
      finish-on-restart marker file.
- [ ] Force the live database swap itself to fail during the same restore
      attempt (e.g. make the database file or its containing directory
      temporarily read-only, or otherwise interrupt the live swap) so both
      recovery paths fail together, not just one.
- [ ] Click **Restore** on the backup from step 1.
- [ ] Confirm the UI shows a clear **error toast** — not a "Backup restored
      successfully" message and not a "restart required" message.
- [ ] Confirm the restore dialog stays open (does not close as if the
      restore succeeded).
- [ ] Confirm the error message references that a pre-restore safety backup
      was created and can be used for manual recovery.
- [ ] Check **Tasks → Backups** — confirm no proxy hosts/settings were
      silently changed; your pre-failure configuration is still what's
      actually running.
- [ ] Free up disk space / restore normal permissions, then retry the same
      restore — confirm it now completes normally (either live or with the
      documented "restart required" flow).

### B. Pre-Restore Safety Backup Survives Aggressive Cleanup (M4)

- [ ] Under **Tasks → Backups → Backup Schedule**, set **Local Retention
      Count** to the smallest allowed value (e.g. `1` or `2`).
- [ ] Create several manual backups so you have more than the retention
      count on disk.
- [ ] Perform a normal restore (any backup) so Charon creates a pre-restore
      safety backup automatically.
- [ ] Create additional manual backups afterward so a cleanup pass is
      triggered and would, by count alone, be aggressive enough to reach the
      pre-restore backup's position in the list.
- [ ] Confirm via **Tasks → Backups** (and, if you have filesystem access,
      directly in the `backups/` folder) that the pre-restore safety backup
      is **still present** after cleanup, even though it's older than
      several backups that were pruned.
- [ ] Confirm the count of regular (non-safety) backups was pruned down to
      the configured retention count as expected — i.e. the safety backup
      wasn't just spared, it was correctly excluded from the retention math
      entirely.

### C. Encryption-Key Warning Triggers Correctly on a Different Host (M5)

- [ ] On the source instance, configure at least one of: a DNS provider
      credential, a tunnel (Orthrus/Hecate) config, or a remote storage
      target (S3/SFTP/WebDAV/Dropbox/Google Drive).
- [ ] Create a manual backup on the source instance.
- [ ] Copy the backup file to a second Charon instance (or the same
      instance after changing/removing `CHARON_ENCRYPTION_KEY`) that does
      **not** have the same `CHARON_ENCRYPTION_KEY`.
- [ ] Upload the backup via **Upload Backup** (or place it in the target
      instance's `backups/` folder) and click **Validate**.
- [ ] Confirm the **Validate** screen shows the "this backup contains data
      that needs a specific encryption key" warning.
- [ ] Proceed to **Restore** and confirm the same warning is shown again
      before you confirm the restore.
- [ ] Complete the restore and confirm proxy hosts/users/settings come back
      correctly, but the DNS provider credential, tunnel config, or remote
      storage secret from step 1 does **not** work (fails silently, as
      documented) until you re-enter it or set the matching encryption key.
- [ ] As a negative check: repeat with a backup that has **no** encrypted
      secrets configured (no DNS credentials, tunnel configs, or remote
      storage targets) and confirm the warning does **not** appear.

## Acceptance Criteria

- [ ] A genuine double-failure during restore shows an actionable error
      message, never a false "Backup restored successfully" toast.
- [ ] The restore dialog does not close and does not imply success when the
      restore actually failed.
- [ ] A pre-restore safety backup is never removed by automatic cleanup,
      regardless of how aggressive the retention count is set.
- [ ] The "encryption key required" warning appears on both Validate and
      Restore when the archive genuinely contains encrypted secrets, and
      does not appear when it doesn't.

## Related Issues

- PR #1136 (`feature/backuprestore`) — restore-reliability remediation
  (C1, H1, H2, M4, M5)
- `docs/reports/pre_merge_audit_2026-07-14.md` — original audit findings
- `docs/plans/current_spec.md` — remediation technical spec
