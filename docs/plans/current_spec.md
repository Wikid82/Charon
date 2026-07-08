# Configuration Backup & Restore — Gap-Closing Plan (Issue #32)

**Author:** Planning Agent (Principal Architect)
**Date:** 2026-07-07
**Branch:** `feature/backuprestore`
**Issue:** #32 "Configuration Backup & Restore" (Priority: High, Milestone: Beta)
**Type:** Extension of an existing v1 backup system — NOT a greenfield design.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings — Current State & Gap Matrix](#2-research-findings--current-state--gap-matrix)
3. [Technical Specifications](#3-technical-specifications)
   - 3.1 [EARS Requirements](#31-ears-requirements)
   - 3.2 [Backup Archive Format v2](#32-backup-archive-format-v2)
   - 3.3 [API Contracts](#33-api-contracts)
   - 3.4 [GORM Models](#34-gorm-models)
   - 3.5 [Safe-Restore Strategy](#35-safe-restore-strategy)
   - 3.6 [Encryption Design](#36-encryption-design)
   - 3.7 [Remote Storage Design](#37-remote-storage-design)
   - 3.8 [Frontend Design](#38-frontend-design)
   - 3.9 [Security Considerations](#39-security-considerations)
   - 3.10 [Error Handling & Edge Cases](#310-error-handling--edge-cases)
4. [Implementation Plan (Phases)](#4-implementation-plan-phases)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)
7. [Ignore-File & Repo Hygiene Review](#7-ignore-file--repo-hygiene-review)
8. [Open Questions for the User](#8-open-questions-for-the-user)

---

## 1. Introduction

### 1.1 Overview

Charon already ships a working v1 backup system: zip archives containing a
`VACUUM INTO` SQLite snapshot plus the Caddy data directory, created manually via the
UI or by a **hardcoded** daily 03:00 cron, restorable with a live database rehydrate.
Issue #32 asks for the full Beta feature: complete backup contents, configurable
scheduling, validated restore, optional encryption, remote storage (S3/SFTP), backup
history, and a disaster recovery guide.

This plan closes the gap between the two. Every section below states explicitly what
**exists** (and is reused), what is **modified**, and what is **new**.

### 1.2 Objectives

1. Versioned archive format v2 with `manifest.json`, SHA-256 checksums, and CrowdSec
   config included; v1 archives remain restorable.
2. User-configurable schedule and retention that the backend actually honors (today the
   UI writes `backup.interval` / `backup.retention` Settings that **no backend code reads**).
3. Restore pipeline with pre-validation (manifest, checksums, `PRAGMA integrity_check`),
   pre-restore safety backup, and rollback.
4. Optional passphrase encryption (age/scrypt) for archives at rest and in transit.
5. Interface-based remote storage with S3 and SFTP implementations, retention pruning,
   and test-connection endpoint.
6. `BackupRecord` history persisted in SQLite (type, checksum, encryption flag, remote
   copy status).
7. Disaster recovery guide + correction of `docs/features/backup-restore.md`, which
   currently **overstates** capability.

### 1.3 Non-Goals

- No backup of Docker volumes / host OS state — Charon data dir only.
- No point-in-time / incremental backups — full snapshots only.
- No restore *into a running cluster* semantics — single-instance app.
- No GUI-driven off-host restore bootstrap (documented manually in the DR guide).

---

## 2. Research Findings — Current State & Gap Matrix

### 2.1 What exists today (verified in source)

**Service — `backend/internal/services/backup_service.go`:**

- `BackupService` struct: `DataDir`, `BackupDir`, `DatabaseName`, `Cron *cron.Cron`
  (`robfig/cron/v3`, already in `backend/go.mod`), `restoreDBPath`, and test seams
  `createBackup` / `cleanupOld` (lines 81–89).
- `NewBackupService(cfg *config.Config)` — derives `BackupDir` =
  `filepath.Dir(cfg.DatabasePath) + "/backups"` (0700), **hardcodes** cron
  `"0 3 * * *"` (line 162). Scheduler lifecycle via `Start()` / `Stop()` (graceful,
  waits on `cron.Stop()` context).
- `CreateBackup()` — writes `backup_<2006-01-02_15-04-05>.zip` containing:
  1. SQLite snapshot via `createSQLiteSnapshot` → `VACUUM INTO` a temp file (line 126)
     — already the correct online-backup method for a WAL-mode DB (the prompt's claim
     of "raw file copy" is **outdated**; verified against source).
  2. `caddy/` directory walk via `addDirToZip` (certificates included).
  - **Missing:** manifest, checksums, `data/crowdsec/`, encryption, history record.
- `ListBackups()` — filesystem scan of `BackupDir`, filters `.zip` only (line 275),
  returns `BackupFile{Filename, Size, Time}` (json: `filename`, `size`, `time`),
  newest first.
- `RunScheduledBackup()` — create + `CleanupOldBackups(DefaultBackupRetention)` where
  `DefaultBackupRetention = 7` (count-based, line 172).
- `DeleteBackup` / `GetBackupPath` — path-traversal guards (`filepath.Base` equality +
  prefix check). Covered by `backend/internal/api/handlers/backup_handler_sanitize_test.go`.
- `RestoreBackup(filename)` — extracts DB (+`-wal`/`-shm`, checkpointed) to a temp file
  via `extractDatabaseFromBackup`, then `unzipWithSkip` extracts everything else into
  `DataDir` using `SafeJoinPath` (zip-slip protection, lines 45–79) with a **100 MB
  per-entry decompression-bomb limit** (lines 654, 756).
- `RehydrateLiveDatabase(db *gorm.DB)` — live restore without restart:
  `ATTACH DATABASE` a temp copy, `DELETE FROM` + `INSERT INTO ... SELECT` per table
  (identifiers validated by `quoteSQLiteIdentifier`), `sqlite_sequence` sync,
  `DETACH`, `wal_checkpoint(TRUNCATE)`.
- `GetAvailableSpace()` — `syscall.Statfs` on `BackupDir`.
- Test files: `backup_service_test.go`, `_disk_test.go`, `_rehydrate_test.go`,
  `_wave3..7_test.go` — substantial existing coverage to keep green.

**Handler — `backend/internal/api/handlers/backup_handler.go`:**

- `BackupHandler{service, securityService, db}`; constructors `NewBackupHandler`,
  `NewBackupHandlerWithDeps`.
- `List`, `Create`, `Delete`, `Download`, `Restore`. `Create`/`Delete`/`Restore` call
  `requireAdmin(c)` (`permission_helpers.go:16`). **`List` and `Download` do NOT**
  (see §3.9 — Download of a full DB dump should be admin-gated).
- `Restore` retries `RehydrateLiveDatabase` up to 5× on transient SQLite lock errors
  (`isSQLiteTransientRehydrateError`), responds
  `{"message", "restart_required", "live_rehydrate_applied"}`.
- Structured errors via `gin.H{"error": ...}`; logging via
  `middleware.GetRequestLogger` + `util.SanitizeForLog`.

**Routing — `backend/internal/api/routes/routes.go`:**

- Lines 216–223: `services.NewBackupService(&cfg)` + `backupService.Start()`;
  `NewDBHealthHandler(db, backupService)` (exposes `last_backup` in
  `db_health_handler.go:26,60-61`).
- Lines 304–309: routes mounted under `management := protected.Group("/")` with
  `middleware.RequireManagementAccess()` (JWT-protected, passthrough users blocked):
  `GET/POST /api/v1/backups`, `DELETE /backups/:filename`,
  `GET /backups/:filename/download`, `POST /backups/:filename/restore`.
- Line 787: `handlers.NewCertificateHandler(certService, backupService, ...)` — the
  certificate handler depends on `BackupServiceInterface`
  (`certificate_handler.go:21-33`); interface must stay satisfied.
- AutoMigrate list at lines 100–137 (registration point for new models).

**Crypto (reuse) — `backend/internal/crypto/`:**

- `encryption.go`: `EncryptionService` — AES-256-GCM, base64 32-byte key,
  nonce-prepended, constructed from `cfg.EncryptionKey`
  (`config.go:102` ← `CHARON_ENCRYPTION_KEY`). Wired in `routes.go` at 169–171,
  413–416, 778–780.
- `rotation_service.go`: key rotation via `CHARON_ENCRYPTION_KEY_NEXT` /
  `CHARON_ENCRYPTION_KEY_V1..V10`.
- Storage pattern for secrets: `models.DNSProviderCredential.CredentialsEncrypted`
  (`json:"-"`, AES-256-GCM JSON blob) + `KeyVersion int` — reused verbatim for
  `RemoteStorageTarget` (§3.4).

**SSRF (reuse) — `backend/internal/network/safeclient.go`:**

- `privateCIDRs` blocklist (RFC1918, loopback, link-local/cloud-metadata
  `169.254.0.0/16`, reserved, IPv6 ULA/link-local) with an explicit `AllowRFC1918`
  bypass path (`rfc1918CIDRs`, `IsRFC1918`) for homelab targets.

**Config — `backend/internal/config/config.go`:**

- `DatabasePath` (line 94, `CHARON_DB_PATH`, default `data/charon.db`),
  `EncryptionKey` (line 102), `CrowdSecConfigDir` (line 167,
  `CHARON_CROWDSEC_CONFIG_DIR`, default `data/crowdsec`).

**Version:** `backend/internal/version/version.go` — `Version` (ldflags) → manifest
`app_version`.

**Caddy reload:** `backend/internal/caddy/manager.go:105` —
`(*Manager).ApplyConfig(ctx)` with built-in snapshot/rollback (`saveSnapshot`,
`rollback`). This is the post-restore reload hook.

**Frontend:**

- `frontend/src/api/backups.ts` — `BackupFile{filename,size,time}`, `getBackups`,
  `createBackup`, `restoreBackup`, `deleteBackup` (+ `__tests__/backups.test.ts`).
- `frontend/src/pages/Backups.tsx` — table, create button, restore/delete dialogs,
  download via `window.location.href`, data-testids (`backup-table`, `backup-row`,
  `backup-download-btn`, `backup-restore-btn`, `backup-delete-btn`, `empty-state`,
  `loading-skeleton`). **Bug:** writes Settings keys `backup.interval` /
  `backup.retention` via `src/api/settings.ts:updateSetting` — grep confirms **no
  backend code reads these keys**; they are dead knobs. Also `useState(() => {...})`
  at line 58 is a misuse (never re-runs on settings load).
- No `useBackups` hook exists (page uses `useQuery` inline — deviates from the
  `src/hooks/use*.ts` convention in CLAUDE.md).
- i18n: `frontend/src/locales/{de,en,es,fr,zh}/translation.json` with existing
  `backups.*` keys (28 keys in `en`).

**E2E:** `tests/tasks/backups-create.spec.ts` (17 mocked tests via
`tests/utils/phase5-helpers.ts:setupBackupsList`), `tests/tasks/backups-restore.spec.ts`
(8 tests), `tests/integration/backup-restore-e2e.spec.ts`. Note: mock fixtures use
`.tar.gz` filenames although the real format is `.zip` (harmless because mocked, but
fixtures will be corrected to `.zip`).

**Docs:** `docs/features/backup-restore.md` **overstates** reality: claims CrowdSec
config is included (it is not), claims pre-upgrade/pre-change automatic backups
(not implemented), claims day-based retention 30/90 days (actual: count-based, 7),
claims manual-backup labels (not implemented).

**Repo hygiene:** `.gitignore:141` has `/data/backups/` (root-anchored) — it does
**not** cover `backend/data/backups/`. The stray file
`backend/data/backups/charon_backup_20251217_144822.db` exists **on disk but is NOT
committed** (verified via `git ls-files`). The guard hook
`scripts/pre-commit-hooks/block-data-backups-commit.sh` only matches `data/backups/*`,
so it would not block `backend/data/backups/*` either. See §7.

### 2.2 Gap Matrix vs Issue #32

| # | Issue #32 requirement | Status | Exists / Reused | Gap (New work) |
|---|---|---|---|---|
| 1 | Backup format: DB + configs + certificates | **Partial** | zip + `VACUUM INTO` snapshot + `caddy/` dir | `manifest.json` (version, checksums, app_version), include `data/crowdsec/`, format versioning, `.zip` v1 compat |
| 2 | One-click backup button | **Done** | `POST /backups` + `Backups.tsx` button | Extend response with record UUID/type |
| 3 | Scheduled automatic backups | **Partial** | cron wired, `Start()/Stop()`, retention cleanup | Schedule/retention **configurable** (backend must read settings; runtime reschedule via `cron.Remove(EntryID)` + `AddFunc`); dead UI knobs fixed |
| 4 | Restore with validation | **Partial** | traversal/zip-slip/bomb guards, live rehydrate (`ATTACH`-based) | Manifest+checksum verification, `PRAGMA integrity_check` on temp copy, pre-restore safety backup, rollback, Caddy `ApplyConfig` after restore, upload-and-restore, **durable pending-restore file + boot-time swap consumer** (v1's "restart completes the restore" story has no working code behind it today — see §3.5) |
| 5 | Optional backup encryption | **Missing** | `crypto.EncryptionService` reused only for stored secrets | age (scrypt) passphrase encryption of archives (§3.6) |
| 6 | Remote storage (S3, SFTP) | **Missing** | `network` SSRF validation reused | `RemoteStorageTarget` model, uploader interface, minio-go + pkg/sftp impls, test-connection, remote retention pruning |
| 7 | Backup history management | **Partial** | filesystem `ListBackups` | `BackupRecord` + `BackupRemoteCopy` GORM models, reconciliation with filesystem, richer list API |
| 8 | Disaster recovery guide | **Missing** | `docs/features/backup-restore.md` (inaccurate) | New `docs/features/disaster-recovery.md`; correct existing doc |

### 2.3 External dependencies (new)

| Dependency | Purpose | Justification |
|---|---|---|
| `filippo.io/age` | Passphrase archive encryption | Audited (Trail of Bits 2024), pure Go, streaming STREAM/ChaCha20-Poly1305 construction, scrypt recipient built in; avoids hand-rolled chunked AES-GCM (§3.6) |
| `github.com/minio/minio-go/v7` | S3 client | Single module (vs aws-sdk-go-v2's multi-module tree), first-class support for S3-compatible endpoints (MinIO, Backblaze B2, Cloudflare R2) that self-hosters actually use; path-style addressing |
| `github.com/pkg/sftp` | SFTP client | De-facto standard; pairs with already-present `golang.org/x/crypto` v0.53.0 (`x/crypto/ssh`) |

Existing deps reused: `robfig/cron/v3`, `mattn/go-sqlite3`, `google/uuid`,
`golang.org/x/crypto`.

---

## 3. Technical Specifications

### 3.1 EARS Requirements

| ID | Requirement (EARS) |
|---|---|
| R1 | WHEN an admin triggers a backup, THE SYSTEM SHALL produce a format-v2 archive containing the SQLite snapshot, `caddy/`, `crowdsec/`, and a `manifest.json` with SHA-256 checksums, and SHALL persist a `BackupRecord`. |
| R2 | WHILE backup encryption is enabled, THE SYSTEM SHALL encrypt every produced archive with the configured passphrase before it touches `data/backups/` or any remote target. |
| R3 | WHEN the configured schedule fires, THE SYSTEM SHALL create a `scheduled` backup, prune local backups beyond the retention count, upload to all enabled remote targets, and prune remote copies beyond the remote retention count. |
| R4 | WHEN a restore is requested, THE SYSTEM SHALL validate the archive (manifest version, checksums, `PRAGMA integrity_check` on a temp copy) and SHALL refuse to modify live data if validation fails. |
| R5 | WHEN validation passes, THE SYSTEM SHALL create a `pre_restore` safety backup before modifying any live data, and SHALL restore from it if the restore fails partway. |
| R6 | WHEN a restore completes, THE SYSTEM SHALL rehydrate the live database (existing `RehydrateLiveDatabase`), reload Caddy via `ApplyConfig`, and report `restart_required` when rehydrate was not possible. |
| R7 | WHEN a legacy v1 archive (no manifest) is restored, THE SYSTEM SHALL restore it with a logged warning and skip checksum verification. |
| R8 | WHEN a user submits a remote target, THE SYSTEM SHALL store credentials AES-256-GCM-encrypted (existing `crypto.EncryptionService`), SHALL never echo secrets in any response, and SHALL validate the endpoint against the `network` package's SSRF rules (RFC1918 allowed, loopback/link-local/metadata blocked). |
| R9 | IF `CHARON_ENCRYPTION_KEY` is absent, THEN THE SYSTEM SHALL disable remote-target credential storage and scheduled-encryption passphrase storage, degrading gracefully with a clear API error (mirrors `routes.go:523` behavior for DNS providers). |
| R10 | WHEN an archive is uploaded for restore, THE SYSTEM SHALL enforce a request size limit, validate it as R4 before it becomes restorable, and store it under a sanitized server-generated filename. |

### 3.2 Backup Archive Format v2

**Decision: stay on zip** (`archive/zip`), not tar.gz. Rationale: all v1 hardening
(`SafeJoinPath`, per-entry bomb limits, skip-list extraction) already targets zip;
v1 archives stay restorable through one code path; zips are double-clickable for
novice users downloading them. Versioning lives in the manifest, not the container.

**Filenames** (all produced in `BackupDir`, listed by extension):

- Unencrypted: `backup_<2006-01-02_15-04-05>.zip` (unchanged from v1)
- Encrypted: `backup_<2006-01-02_15-04-05>.zip.age` (age-encrypted whole archive)
- Uploaded: `uploaded_<ts>.zip[.age]` (server-generated on upload; client name discarded)

**Archive layout (v2):**

```
charon.db                      ← VACUUM INTO snapshot (existing createSQLiteSnapshot)
caddy/**                       ← certificates + Caddy state (existing addDirToZip)
crowdsec/**                    ← cfg.CrowdSecConfigDir contents (NEW)
manifest.json                  ← MUST be the LAST entry written
```

`manifest.json` is written **last**, not first: each preceding entry's
`ManifestEntry.SHA256` is accumulated in a single pass via
`io.MultiWriter(zipEntry, sha256.New())` as that entry streams into the archive, so
the checksums cannot exist until every other entry has already been written. Zip's
central directory makes entry *position* irrelevant for reading — a reader locates
`manifest.json` by name, not offset — so writing it last has no downside and is the
only order compatible with single-pass streaming (re-reading already-written entries
to backfill checksums would double the I/O of every backup).

**`ListBackups`, `CleanupOldBackups`, and `GetLastBackupTime` scan for both `.zip`
and `.zip.age` suffixes.** Today's `ListBackups` filters
`strings.HasSuffix(entry.Name(), ".zip")` only (`backup_service.go:275`) — this is a
required **code change**, not just documentation: without it, an encrypted backup
would be invisible to listing, retention pruning, and the DB-health `last_backup`
timestamp the moment encryption is enabled.

**`manifest.json` schema** (new Go type in `backup_service.go` or new
`backup_manifest.go` in the same package):

```go
// BackupManifest describes the contents of a format-v2 backup archive.
type BackupManifest struct {
    FormatVersion int              `json:"format_version"` // 2
    CreatedAt     time.Time        `json:"created_at"`
    AppVersion    string           `json:"app_version"`    // version.Version
    BackupType    string           `json:"backup_type"`    // manual|scheduled|pre_restore|uploaded
    DatabaseName  string           `json:"database_name"`  // e.g. "charon.db"
    Contents      []ManifestEntry  `json:"contents"`
    EncryptionKeyRequired bool     `json:"encryption_key_required"` // DB rows encrypted with CHARON_ENCRYPTION_KEY exist
}

type ManifestEntry struct {
    Path   string `json:"path"`   // archive-relative, forward slashes
    Size   int64  `json:"size"`
    SHA256 string `json:"sha256"` // hex
}
```

`EncryptionKeyRequired` is computed by checking whether any rows exist in
`dns_provider_credentials`, `tunnel_configs`, or other `*Encrypted`-bearing tables —
it powers a restore-time warning that the target host needs the same
`CHARON_ENCRYPTION_KEY` (§3.6).

**Backward compatibility:**

| Format | Detection | Restore behavior |
|---|---|---|
| v2 `.zip` / `.zip.age` | `manifest.json` present (age suffix ⇒ decrypt first) | Full validation (checksums + integrity_check) |
| v1 `.zip` | zip, no `manifest.json` | Restore allowed; warning logged + `"legacy_format": true` in validate response; checksum step skipped; integrity_check still runs on the extracted DB |
| v0 raw `.db` (e.g. `charon_backup_20251217_144822.db`) | SQLite magic header `SQLite format 3\x00` | Accepted **only** via the upload endpoint; server wraps it into a v2 archive (DB-only) then restores. Never listed from disk (`ListBackups` keeps filtering by extension). See Open Question Q4. |

**Service changes (`backend/internal/services/backup_service.go`):**

- `CreateBackup()` → becomes a thin wrapper for `CreateBackupWithOptions(opts BackupOptions) (*models.BackupRecord, error)` where
  `BackupOptions{Type string, Encrypt bool, Passphrase string}`. The existing
  signature `CreateBackup() (string, error)` is **kept** (certificate handler's
  `BackupServiceInterface` at `certificate_handler.go:21` and the cron test seams
  depend on it).
- New: `writeManifest`, checksum computation while streaming entries
  (`io.MultiWriter(zipEntry, sha256.New())` — single pass, no re-read).
- New: `addDirToZip` call for `crowdsec/` (source: `cfg.CrowdSecConfigDir`, tolerate
  absence exactly like the existing caddy-dir warning path, line 334–337).
- `NewBackupService(cfg)` gains fields: `CrowdSecDir string`, `db *gorm.DB`,
  `encryption *crypto.EncryptionService` (nilable), `scheduleEntry cron.EntryID`,
  `mu sync.Mutex`. Constructor signature change to
  `NewBackupService(cfg *config.Config, db *gorm.DB, enc *crypto.EncryptionService)`
  — single call site (`routes.go:217`) plus tests.
- New: `Reschedule(cronSpec string) error` — validates via `cron.ParseStandard`,
  `s.Cron.Remove(s.scheduleEntry)`, re-adds; called on settings save and at startup
  from persisted settings (falls back to `"0 3 * * *"`).

**Complexity:** Medium (service refactor touches many existing tests).

### 3.3 API Contracts

All routes live under the existing `management` group
(`RequireManagementAccess`) in `backend/internal/api/routes/routes.go`; mutating
routes additionally call `requireAdmin` inside handlers (existing pattern). All JSON
is snake_case. UUIDs are server-generated (`BeforeCreate` hook pattern, cf.
`models/notification_provider.go:48`). Errors: `gin.H{"error": "..."}` (+
`error_code` where the existing helpers add it).

**Auth policy summary (definitive — §3.3.1/§3.3.2/§3.9 all reference this table so
the auth decision is stated exactly once):**

| Route | Auth |
|---|---|
| `GET /api/v1/backups` | management (existing; unchanged) |
| `POST /api/v1/backups` | admin |
| `DELETE /api/v1/backups/:filename` | admin |
| `GET /api/v1/backups/:filename/download` | **admin (changed from management — full-DB exfiltration risk, §3.9)** |
| `POST /api/v1/backups/:filename/restore` | admin |
| `POST /api/v1/backups/upload` | admin |
| `POST /api/v1/backups/:filename/validate` | admin |
| `GET /api/v1/backups/settings` | management |
| `PUT /api/v1/backups/settings` | admin |
| `GET /api/v1/backups/remote-targets` | **admin — reveals NAS hostnames/usernames/bucket names even with secrets omitted, so it is not management-level like plain backup listing** |
| `POST` / `PUT` / `DELETE /api/v1/backups/remote-targets[/:uuid]` | admin |
| `POST /api/v1/backups/remote-targets/:uuid/test` | admin |

#### 3.3.1 Existing routes (extended, backward-compatible)

**`GET /api/v1/backups`** — list (now DB-backed, reconciled with filesystem):

```json
[
  {
    "filename": "backup_2026-07-07_03-00-00.zip",
    "size": 1048576,
    "time": "2026-07-07T03:00:00Z",
    "uuid": "0b6f…",
    "type": "scheduled",
    "encrypted": false,
    "format_version": 2,
    "sha256": "ab12…",
    "status": "completed",
    "app_version": "1.42.0",
    "remote_copies": [
      {"target_uuid": "…", "target_name": "NAS", "status": "uploaded", "uploaded_at": "…"}
    ]
  }
]
```

`filename`/`size`/`time` keys unchanged → existing frontend and the mocked E2E
helpers keep working during the transition. Files on disk without a `BackupRecord`
(pre-upgrade backups) are listed with `"type": "manual"`, `"format_version": 1`,
null `uuid`. The underlying filesystem scan (and `CleanupOldBackups` /
`GetLastBackupTime`) matches both `.zip` and `.zip.age` files (§3.2) so encrypted
backups are listed, pruned, and reflected in DB-health status identically to
unencrypted ones.

**`POST /api/v1/backups`** — create (admin). Optional body:

```json
{ "encrypt": true, "passphrase": "…" }   // both optional; defaults from settings
```

Response `201`: `{ "filename": "…", "uuid": "…", "message": "Backup created successfully" }`
(`filename` key kept).

**`DELETE /api/v1/backups/:filename`** (admin) — also deletes the `BackupRecord`
and (best-effort, logged) remote copies. Response unchanged.

**`GET /api/v1/backups/:filename/download`** — **now admin-gated** (`requireAdmin`;
see the auth policy summary above) — backups contain the full DB including password
hashes (§3.9). Response unchanged.

**`POST /api/v1/backups/:filename/restore`** (admin). Optional body:

```json
{ "passphrase": "…" }        // required iff filename ends in .age
```

Response `200` (extends existing keys):

```json
{
  "message": "Backup restored successfully",
  "restart_required": false,
  "database_swap_pending": false,
  "live_rehydrate_applied": true,
  "caddy_reloaded": true,
  "pre_restore_backup": "backup_2026-07-07_10-00-00.zip",
  "legacy_format": false
}
```

`database_swap_pending` is `true` only when live rehydrate was exhausted and a
durable `.pending-restore` file was written for the next process boot to consume
(§3.5) — it always accompanies `restart_required: true` and is the field the UI uses
to show "restart to finish restoring" rather than a generic restart hint. Once the
next boot completes the swap (or rejects a corrupt pending file), the corresponding
`BackupRecord.status` transitions to `restore_completed` or `restore_failed` and is
visible on the next `GET /api/v1/backups`.

Errors: `400` invalid/corrupt archive or checksum mismatch (`error_code:
"backup_validation_failed"`), `400` missing/wrong passphrase
(`"backup_passphrase_required"` / `"backup_passphrase_invalid"`), `404` not found,
`500` restore failure (safety backup restored; message says so).

#### 3.3.2 New routes

**`POST /api/v1/backups/upload`** (admin, multipart) — field `file`; optional field
`passphrase` (validates an encrypted upload immediately). Enforce
`c.Request.ParseMultipartForm` limit + `http.MaxBytesReader` of **512 MB**
(constant `maxBackupUploadSize`; precedent: `image_upload_handler.go:55`).
Validates (§3.5 step V) before persisting as `uploaded_<ts>.zip[.age]` +
`BackupRecord{type: "uploaded"}`. Response `201`:

```json
{ "filename": "uploaded_2026-07-07_10-12-00.zip", "uuid": "…", "legacy_format": false, "message": "Backup uploaded and validated" }
```

Restore then goes through the normal restore endpoint (two-step keeps the dangerous
action explicit and re-uses one restore path).

**`POST /api/v1/backups/:filename/validate`** (admin) — dry-run validation, body
optional `{ "passphrase": "…" }`. Response `200`:

```json
{
  "valid": true,
  "format_version": 2,
  "legacy_format": false,
  "app_version": "1.41.0",
  "created_at": "…",
  "database_integrity": "ok",
  "encryption_key_required": true,
  "warnings": ["archive app_version (1.41.0) is older than running version (1.42.0)"]
}
```

**`GET /api/v1/backups/settings`** (management) / **`PUT /api/v1/backups/settings`** (admin) —
typed facade over `models.Setting` rows, category `"backup"` (§3.4.4):

```json
// GET response / PUT request (PUT: all fields optional, partial update)
{
  "schedule_enabled": true,
  "schedule_cron": "0 3 * * *",
  "retention_count": 7,
  "remote_retention_count": 7,
  "encryption_enabled": false,
  "encryption_passphrase_set": true      // GET only; passphrase itself is write-only
}
```

PUT additionally accepts `"encryption_passphrase": "…"` (never returned). PUT
validates `schedule_cron` with `cron.ParseStandard`, `retention_count >= 1`, and
`remote_retention_count >= 1`, and calls `BackupService.Reschedule`. `400` on
invalid cron (`"backup_invalid_cron"`) or either retention count `< 1`
(`"backup_invalid_retention_count"`).

**Remote targets:**

| Method & path | Purpose |
|---|---|
| `GET /api/v1/backups/remote-targets` | List (admin; secrets omitted — but hostnames/buckets/usernames are still sensitive, so this route is admin-gated like every other remote-target route, not management-level) |
| `POST /api/v1/backups/remote-targets` | Create (admin) |
| `PUT /api/v1/backups/remote-targets/:uuid` | Update (admin; secret fields optional — omitted ⇒ keep existing) |
| `DELETE /api/v1/backups/remote-targets/:uuid` | Delete (admin) |
| `POST /api/v1/backups/remote-targets/:uuid/test` | Test connection (admin) |

Create/Update request:

```json
{
  "name": "Home NAS",
  "type": "sftp",                        // "s3" | "sftp"
  "enabled": true,
  "config": {
    // s3: endpoint, region, bucket, path_prefix, use_ssl, force_path_style
    // sftp: host, port, path, username
    "host": "nas.lan", "port": 22, "path": "/backups/charon", "username": "charon"
  },
  "secrets": {
    // s3: access_key_id, secret_access_key
    // sftp: password OR private_key_pem (+ optional passphrase), host_key_fingerprint (SHA256:…)
    "password": "…"
  }
}
```

Response (GET/POST/PUT — **never** includes `secrets`):

```json
{
  "uuid": "…", "name": "Home NAS", "type": "sftp", "enabled": true,
  "config": { "host": "nas.lan", "port": 22, "path": "/backups/charon", "username": "charon" },
  "secrets_set": true,
  "last_test_at": "…", "last_test_status": "ok", "last_error": "",
  "created_at": "…", "updated_at": "…"
}
```

Test response `200`: `{ "success": true, "message": "…", "latency_ms": 84 }`;
`400/502` with `gin.H{"error": …}` on failure. `503` +
`"error_code": "encryption_key_missing"` when `CHARON_ENCRYPTION_KEY` unset (R9).

**Routing regression coverage (new static routes vs. the existing `:filename`
wildcard):** `GET/PUT /api/v1/backups/settings` and the
`/api/v1/backups/remote-targets*` routes are static siblings of the pre-existing
`/api/v1/backups/:filename[...]` wildcard routes. Gin's radix-tree router
(httprouter-derived) matches static segments with priority over a param segment at
the same position, so `GET /api/v1/backups/settings` resolves to the static route,
never to `:filename="settings"` — but this must be **asserted by a router-level
test, not assumed**, since a future refactor could reorder registration and silently
invert that priority. Required `httptest`-based regression test: (a) `GET`/`PUT
/api/v1/backups/settings` and every `/api/v1/backups/remote-targets*` method hit
their intended handlers, never `backupHandler.List`/`.Download`/etc.; (b) for
methods with no static sibling (e.g. `DELETE /api/v1/backups/settings`, which falls
through to `DELETE /api/v1/backups/:filename` with `filename="settings"`), the
existing sanitize/not-found logic in `DeleteBackup` handles the literal segment
`"settings"` safely — asserts `404 backup not found`, never any interaction with the
`Setting` GORM model (there is no code path from this route to it, but the test
makes that explicit rather than implicit).

**Handler placement:** extend `backup_handler.go` for backups/settings; new
`backend/internal/api/handlers/backup_remote_handler.go` for targets. New service
`backend/internal/services/backup_remote_service.go`.

**Complexity:** Medium.

### 3.4 GORM Models

New file per model in `backend/internal/models/`; all registered in the AutoMigrate
block at `routes.go:100` (append after `&models.CustomTheme{}`), per CLAUDE.md
Migrations rule.

#### 3.4.1 `backup_record.go`

```go
type BackupRecord struct {
    ID            uint       `json:"-" gorm:"primaryKey"`
    UUID          string     `json:"uuid" gorm:"uniqueIndex;size:36"`
    Filename      string     `json:"filename" gorm:"uniqueIndex;not null;size:255"`
    Size          int64      `json:"size"`
    SHA256        string     `json:"sha256" gorm:"size:64"`         // of the final on-disk file (post-encryption)
    Type          string     `json:"type" gorm:"index;size:20"`     // manual|scheduled|pre_restore|uploaded
    FormatVersion int        `json:"format_version" gorm:"default:2"`
    Encrypted     bool       `json:"encrypted" gorm:"default:false"`
    AppVersion    string     `json:"app_version" gorm:"size:50"`
    Status        string     `json:"status" gorm:"index;size:20"`   // completed|failed|deleted|restore_pending|restore_completed|restore_failed
    ErrorMessage  string     `json:"error_message,omitempty" gorm:"type:text"`
    RemoteCopies  []BackupRemoteCopy `json:"remote_copies,omitempty" gorm:"foreignKey:BackupRecordID"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}
func (BackupRecord) TableName() string { return "backup_records" }
// BeforeCreate sets UUID via google/uuid (pattern: notification_provider.go:48)
```

#### 3.4.2 `backup_remote_copy.go`

```go
type BackupRemoteCopy struct {
    ID              uint       `json:"-" gorm:"primaryKey"`
    BackupRecordID  uint       `json:"-" gorm:"index;not null"`
    RemoteTargetID  uint       `json:"-" gorm:"index;not null"`
    RemoteTarget    *RemoteStorageTarget `json:"-" gorm:"foreignKey:RemoteTargetID"`
    TargetUUID      string     `json:"target_uuid" gorm:"-"`   // populated in handler from RemoteTarget
    TargetName      string     `json:"target_name" gorm:"-"`
    RemoteKey       string     `json:"remote_key" gorm:"size:512"`   // object key / remote path
    Status          string     `json:"status" gorm:"index;size:20"`  // pending|uploading|uploaded|failed|pruned
    ErrorMessage    string     `json:"error_message,omitempty" gorm:"type:text"`
    UploadedAt      *time.Time `json:"uploaded_at,omitempty"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}
```

#### 3.4.3 `remote_storage_target.go` (mirrors `DNSProviderCredential` secret pattern)

```go
type RemoteStorageTarget struct {
    ID              uint      `json:"-" gorm:"primaryKey"`
    UUID            string    `json:"uuid" gorm:"uniqueIndex;size:36"`
    Name            string    `json:"name" gorm:"not null;size:255"`
    Type            string    `json:"type" gorm:"index;not null;size:10"` // s3|sftp
    Enabled         bool      `json:"enabled" gorm:"default:true;index"`
    ConfigJSON      string    `json:"-" gorm:"type:text"`      // non-secret config (endpoint/bucket/host/port/…)
    SecretsEncrypted string   `json:"-" gorm:"type:text"`      // AES-256-GCM via crypto.EncryptionService
    KeyVersion      int       `json:"key_version" gorm:"default:1"`
    LastTestAt      *time.Time `json:"last_test_at,omitempty"`
    LastTestStatus  string    `json:"last_test_status" gorm:"size:20"` // ok|failed|never
    LastError       string    `json:"last_error,omitempty" gorm:"type:text"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

Handlers decode `ConfigJSON` into the typed `config` response object; `secrets` never
leave the backend (only `secrets_set` boolean).

#### 3.4.4 Schedule settings — decision: reuse `models.Setting` (no new model)

The UI already writes Settings keys; a dedicated `BackupSchedule` table would be a
second source of truth for four scalars. Canonical keys (category `"backup"`),
read/written **only** through the typed `/backups/settings` endpoints and
`BackupService`:

| Key | Type | Default | Notes |
|---|---|---|---|
| `backup.schedule_enabled` | bool | `true` | |
| `backup.schedule_cron` | string | `"0 3 * * *"` | validated by `cron.ParseStandard` |
| `backup.retention_count` | int | `7` | local, count-based (matches `DefaultBackupRetention`) |
| `backup.remote_retention_count` | int | `7` | per target |
| `backup.encryption_enabled` | bool | `false` | |
| `backup.encryption_passphrase_enc` | string | `""` | AES-256-GCM-encrypted via `crypto.EncryptionService`; required for scheduled encrypted backups; write-only |

**Generic Settings API leak (fix required).**
`backend/internal/api/handlers/settings_handler.go`'s `isSensitiveSettingKey`
(line 94) matches only the fragments `password`, `secret`, `token`, `api_key`,
`apikey`, `webhook` — **`backup.encryption_passphrase_enc` matches none of them**,
so `GET /api/v1/settings` (management-level, not admin-gated) would echo the
AES-256-GCM ciphertext of the backup passphrase verbatim to any authenticated
management-role user. Fix: add `"passphrase"` to `sensitiveFragments` — this covers
the key today and any future `*.passphrase*` setting without special-casing one
key name.

**Generic Settings API bypass (fix required).** The existing generic
`PUT /api/v1/settings` → `(*SettingsHandler).UpdateSetting` upserts any key/value
pair with no awareness of `backup.*` semantics. The handler already has precedent
for key-specific validation (`req.Key == "security.admin_whitelist"` at
`settings_handler.go:132`, `validateOptionalKeepaliveSetting`), so a write to
`backup.schedule_cron` through this generic endpoint today would skip
`cron.ParseStandard` validation entirely and would never call
`BackupService.Reschedule` — the cron scheduler would silently keep running the old
schedule while the stored setting shows a different one. Fix: add a
`strings.HasPrefix(req.Key, "backup.")` guard to `UpdateSetting` that **rejects**
the write with `400 {"error": "backup settings must be updated via PUT
/api/v1/backups/settings", "error_code": "use_typed_backup_settings_endpoint"}`.
Rejecting (rather than routing generic writes through the typed
validation/`Reschedule` path from inside the generic handler) keeps the reschedule
side effect owned by exactly one code path, which is simpler to reason about and
test than duplicating it in two handlers.

**Migration of dead knobs:** on startup, if legacy `backup.interval` /
`backup.retention` Setting rows exist, translate (`interval` days →
`schedule_cron` `"0 3 */N * *"`, `retention` → `retention_count`) and delete the
legacy rows. Frontend stops writing them.

**Complexity:** Low–Medium.

### 3.5 Safe-Restore Strategy

Pipeline in `BackupService.RestoreBackupSafe(filename, passphrase string) (*RestoreResult, error)`
(new; existing `RestoreBackup` becomes the internal extraction step to preserve its
test suite):

```
 V. VALIDATE (read-only, no live data touched)
    V1. Path sanitation (existing filepath.Base + prefix checks)
    V2. If *.age: stream-decrypt to temp file (age scrypt identity); wrong passphrase fails here
    V3. Open zip; read manifest.json → format_version ∈ {1(absent),2}; reject >2
        ("backup created by a newer Charon version")
    V4. v2: verify every ManifestEntry SHA-256 while streaming, bounding each entry's
        read by that entry's own declared Size + 64 KiB slack (not a flat cap — §3.9);
        reject on first checksum mismatch or size-cap overrun
    V5. Extract DB entry to temp (existing extractDatabaseFromBackup incl. WAL checkpoint)
    V6. sql.Open (mattn/go-sqlite3) on temp copy → PRAGMA integrity_check == "ok"
        + sanity query: sqlite_master contains "users" and "proxy_hosts" tables
 S. SAFETY BACKUP
    S1. CreateBackupWithOptions(Type: "pre_restore") of the CURRENT state
        (recorded in BackupRecord; excluded from retention pruning of scheduled backups)
 A. APPLY
    A1. unzipWithSkip → DataDir (existing zip-slip/bomb-hardened path), skipping DB files
    A2. RehydrateLiveDatabase (existing, incl. the handler's 5× transient-lock retry)
 R. RECONCILE
    R1. caddyManager.ApplyConfig(ctx) (manager.go:105) — has its own snapshot/rollback;
        runs regardless of A2's outcome, since A1 already wrote the new Caddy/CrowdSec
        files to DataDir by this point
    R2. Write result to BackupRecord/audit log; respond
 F. FAILURE HANDLING
    F1. Failure in V*: nothing was touched → 400, done.
    F2. Failure in A1: re-run A1 from the pre_restore archive (its files are known-good);
        respond 500 with "restore failed, previous state restored".
    F3. Failure in A2 after all 5 retries exhausted: do NOT claim success and do NOT
        rely on the in-memory temp file alone — see "Boot-time swap" below, which
        replaces v1's non-functional restart_required story with a durable file plus
        a real consumer. Persist the already-validated (V4/V6-passed) temp DB file to
        `DataDir/<DatabaseName>.pending-restore` (0600, fsync'd) — distinct from
        `os.CreateTemp`'s OS temp dir, which is not guaranteed to survive a container
        restart. Respond `restart_required=true, database_swap_pending=true` with a
        message stating that Caddy/CrowdSec files have already been updated but the
        database will only finish restoring on the next process start. Do not delete
        the pre_restore safety backup (S1) in this branch.
    F4. Failure in R1: log + caddy_reloaded=false in response (Caddy manager already
        rolled itself back); config will re-apply on next change/boot.
```

**Boot-time swap consumer (closes the v1 gap that made `restart_required` a
no-op).** Verified in source: today's `RestoreBackup` writes the extracted DB only
to an `os.CreateTemp("", "charon-restore-db-*.sqlite")` file in the **system temp
directory**; `unzipWithSkip`'s `skipEntries` map explicitly skips the DB/WAL/SHM
entries when extracting into `DataDir` (`backup_service.go:439-446`); and
`s.restoreDBPath` is an **in-process field with no reader anywhere in `cmd/api` or
`internal/server`**. Restarting the container today does nothing: the OS temp file
is not guaranteed to survive a restart, `DataDir/<DatabaseName>` is never
overwritten by the restore path, and no startup code looks for a pending restore —
v1's "restart_required" response was never backed by working code. This plan adds
the missing consumer rather than continue documenting a fallback that doesn't work:

- New `backend/internal/database/pending_restore.go`: `ApplyPendingRestore(dbPath
  string) error` — checks for `dbPath + ".pending-restore"`; if present, re-runs
  `PRAGMA integrity_check` on it as a second, independent verification (defense in
  depth against corruption between write and reboot), and only if that passes:
  removes any stale `dbPath`, `dbPath+"-wal"`, `dbPath+"-shm"` siblings, renames the
  pending file over `dbPath`, and deletes the pending marker. If integrity_check
  fails, the pending file is renamed to `.pending-restore.failed` (kept for
  forensic inspection, never auto-retried) and the **old `dbPath` is left
  untouched** — boot proceeds on the pre-restore database rather than installing a
  known-bad one.
- Wired into `backend/cmd/api/main.go` immediately before the default startup's
  `database.Connect(cfg.DatabasePath)` call (currently line 208) — this runs before
  GORM ever opens the file, so there is no live WAL pool to corrupt and no
  coordination with `RehydrateLiveDatabase` is needed on this path. **Not** wired
  into the `migrate`/`reset-password` CLI subcommand paths (lines 113, 174) — those
  are maintenance tools, not the running-server path a `docker restart` takes, and
  adding the swap there would let an unrelated CLI invocation silently mutate the
  database.
- On successful swap (or on a rejected corrupt pending file), the code path
  transitions the corresponding `BackupRecord.Status` from `restore_pending` to
  `restore_completed` or `restore_failed` (new enum values on the existing `Status`
  field, §3.4.1) so the next `GET /api/v1/backups` reflects the real outcome instead
  of the request-time snapshot going stale.

**Atomic-swap decision — live rehydrate is primary; the durable pending-file +
boot-time swap is an honest fallback, not a redressed no-op.** The running process
holds an open WAL-mode pool (`MaxOpenConns=1`); swapping `charon.db` under it
corrupts the pool's view, so `ATTACH`-based live rehydrate (existing, battle-tested
by `backup_service_rehydrate_test.go`) remains the first attempt. What changes from
the original v1 design is that when live rehydrate is unavailable, the fallback is
no longer a claim resting on code that doesn't exist — it is a durable file plus a
real boot-time consumer that runs strictly before GORM opens the database (above),
so a restart genuinely completes the restore instead of silently doing nothing. The
DR guide (Commit 5) documents both the automatic (live rehydrate succeeds) and
restart-required (pending-swap) paths, and states plainly that between a failed
rehydrate and the next restart, Caddy/CrowdSec files are already on the new
backup's state while the database is still the old one — a known, bounded,
documented window, not a silent inconsistency.

**Restore of remote-target rows:** after rehydrate, rows in
`remote_storage_targets` / `dns_provider_credentials` decrypt only if the host has
the same `CHARON_ENCRYPTION_KEY` — surfaced by manifest `encryption_key_required`
warning in `/validate` (§3.6).

**Complexity:** High (most intricate part of the feature).

### 3.6 Encryption Design

**Choice: `filippo.io/age` with an scrypt (passphrase) recipient.**

- Battle-tested: audited, maintained by the Go crypto lead, used by SOPS et al.
- Correct-by-construction streaming AEAD (STREAM, 64 KiB ChaCha20-Poly1305 chunks) —
  a whole-file `crypto.EncryptionService.Encrypt` call would buffer the entire
  archive in RAM and single-shot AES-GCM is unsafe/impractical for multi-hundred-MB
  files; hand-rolling chunked AES-GCM + argon2 is exactly the custom crypto CLAUDE.md's
  LEVERAGE rule forbids.
- scrypt KDF is built into age's passphrase recipient (`age.NewScryptRecipient`,
  `age.NewScryptIdentity`); work factor default is adequate, no tuning surface exposed.

**What is encrypted:** the entire finished `.zip` (manifest included) →
`backup_<ts>.zip.age`. Nothing about the backup's contents is readable without the
passphrase (metadata lives in `BackupRecord` for the UI). Unencrypted backups remain
the default (Open Question Q2).

**Key handling:**

- Manual backup/restore: passphrase supplied per-request, held in memory only, never
  logged (`util.SanitizeForLog` is NOT enough — passphrases must never reach a log
  call at all).
- Scheduled encrypted backups: passphrase stored in Setting
  `backup.encryption_passphrase_enc`, encrypted with the existing
  `crypto.EncryptionService` (`CHARON_ENCRYPTION_KEY`). If the key is unset,
  enabling scheduled encryption returns `503 encryption_key_missing` (R9).
- No passphrase recovery: documented prominently (UI warning + DR guide) — a lost
  passphrase means unrecoverable backups.

**Secrets-in-backup considerations (documented in DR guide + `/validate` warning):**

- The SQLite DB inside every backup contains `users.password_hash`, and
  AES-256-GCM blobs (`dns_provider_credentials.credentials_encrypted`,
  `remote_storage_targets.secrets_encrypted`, tunnel configs) that are useless
  without `CHARON_ENCRYPTION_KEY`. **Restoring to a new host requires setting the
  same `CHARON_ENCRYPTION_KEY` (and any rotation keys `CHARON_ENCRYPTION_KEY_V1..`)
  on that host.** The manifest's `encryption_key_required` flag drives an explicit
  warning in the validate/restore UI rather than a silent post-restore failure.
- Because backups embed password hashes and cert private keys (`caddy/`), the DR
  guide instructs treating even "unencrypted" backups as secrets — and this is the
  argument for admin-gating Download (§3.9) and for encryption-by-default being an
  open question rather than dismissed.

**Complexity:** Medium.

### 3.7 Remote Storage Design

New package `backend/internal/services/remotestorage/` (keeps `services` tidy and
lets the uploader be tested without GORM):

```go
// Uploader is implemented by each remote storage backend.
type Uploader interface {
    Upload(ctx context.Context, localPath, remoteKey string) error
    Delete(ctx context.Context, remoteKey string) error
    List(ctx context.Context, prefix string) ([]RemoteObject, error) // for retention pruning
    Test(ctx context.Context) error                                  // cheap connectivity+auth+write probe
}

type RemoteObject struct {
    Key          string    `json:"key"`
    Size         int64     `json:"size"`
    LastModified time.Time `json:"last_modified"`
}

func New(target *models.RemoteStorageTarget, secrets map[string]string) (Uploader, error) // factory by target.Type
```

- **`s3.go`** — `minio-go/v7`; honors `endpoint`, `region`, `bucket`, `path_prefix`,
  `use_ssl`, `force_path_style`; `Test` = `BucketExists` + put/delete of a
  `charon-connection-test` marker object.
- **`sftp.go`** — `pkg/sftp` over `x/crypto/ssh`; auth via password or PEM private
  key; **host key verification required**: user supplies `host_key_fingerprint`
  (`SHA256:…`). Two distinct, never-merged code paths:
  - **Discovery** (`POST .../test` when no fingerprint is stored yet): dial with an
    `ssh.ClientConfig.HostKeyCallback` that records the offered key's SHA256
    fingerprint and **unconditionally returns a non-nil error**. In
    `golang.org/x/crypto/ssh`, the host-key callback runs during transport key
    exchange, which completes **before** any authentication method is attempted
    (`ssh.NewClientConn`'s handshake sequences KEX/host-key verification ahead of
    client authentication) — returning an error from the callback aborts the dial
    at that point, so **no password or private key is ever sent** to an unpinned
    host. The discovered fingerprint is returned to the caller for the user to
    confirm out of band; nothing is persisted yet.
  - **Verified test/upload** (fingerprint already stored, or just confirmed by the
    user): dial with `ssh.FixedHostKey(pinnedKey)` as the sole `HostKeyCallback` —
    the handshake fails closed if the remote key ever changes, and only on a
    matching key does authentication proceed. `ssh.InsecureIgnoreHostKey` must
    never appear in the final code.
  - `Test` (verified path) = connect + stat/create target dir + write/delete marker.
  - **Required unit test:** a fake/local SSH server whose `PasswordCallback`
    (or `PublicKeyCallback`) sets a flag if invoked; assert the flag is **never
    set** when exercising the discovery path against a server presenting an
    unpinned/mismatched key — proving no credentials reach the wire before pinning.

**Flow:** `RunScheduledBackup` (and manual create, fire-and-forget goroutine) → for
each enabled target: create `BackupRemoteCopy{status: pending→uploading→
uploaded|failed}` → upload `filename` as `<path_prefix>/<filename>` → prune remote
objects matching `backup_*.zip*` beyond `backup.remote_retention_count` (newest by
`LastModified`; only Charon-named keys are ever deleted). Upload failures never fail
the backup itself — recorded on the copy row and surfaced in the list UI.

**Goroutine lifecycle (graceful shutdown).** `BackupService` gains `uploadCtx
context.Context` / `uploadCancel context.CancelFunc` (created via
`context.WithCancel(context.Background())` in `NewBackupService`) and an
`uploadWG sync.WaitGroup`. Every remote-upload goroutine is launched with
`uploadWG.Add(1)` / `defer uploadWG.Done()` and derives its per-upload context from
`uploadCtx`, so an in-flight upload is canceled, not just orphaned, on shutdown.
`(*BackupService).Stop()` (existing, `backup_service.go:183` — currently
`s.Cron.Stop()` + wait) is extended to also call `uploadCancel()` and
`uploadWG.Wait()` before returning, so the process does not exit mid-upload.
**Startup reconciliation:** any `BackupRemoteCopy` rows left in status `uploading`
from a prior process (crash, OOM-kill, forced shutdown — no graceful `Stop()` ran)
are transitioned to `failed` once, at the next `NewBackupService` construction, so
the UI never shows a permanently-stuck "uploading" row; the existing
next-scheduled-run retry logic picks it up from there.

**SSRF for user-supplied endpoints:** validate the resolved host with the
`network` package rules *with RFC1918 allowed* (`IsRFC1918` bypass — a self-hosted
NAS is the primary use case) while still rejecting loopback, link-local/cloud
metadata (`169.254.0.0/16`), and reserved ranges, at both config-save and dial time
(dial-time check via a `net.Dialer.Control` hook for S3's HTTP transport and the SSH
dialer — consistent with `safeclient.go`'s connection-time layer). Only admins can
configure targets, which bounds the threat, but the check stays as defense in depth.

**Complexity:** Medium–High.

### 3.8 Frontend Design

All work in `frontend/` (CLAUDE.md single-frontend rule). Stack: React 18 + TS +
TanStack Query.

**API layer — extend `frontend/src/api/backups.ts`:**

- Extend `BackupFile` with optional new fields (`uuid?`, `type?`, `encrypted?`,
  `format_version?`, `status?`, `remote_copies?`) — additive, existing tests keep
  passing.
- New functions: `uploadBackup(file: File, passphrase?: string)` (FormData),
  `validateBackup(filename, passphrase?)`, `restoreBackup(filename, passphrase?)`
  (extends existing), `getBackupSettings()`, `updateBackupSettings(payload)`,
  `getRemoteTargets()`, `createRemoteTarget()`, `updateRemoteTarget()`,
  `deleteRemoteTarget()`, `testRemoteTarget()` — typed request/response interfaces
  matching §3.3 exactly (snake_case).

**Hooks — new `frontend/src/hooks/useBackups.ts` (+ `useRemoteTargets.ts`):**

- `useBackups()` (query `['backups']`), `useCreateBackup()`, `useRestoreBackup()`,
  `useDeleteBackup()`, `useUploadBackup()`, `useBackupSettings()`,
  `useUpdateBackupSettings()`; remote-target CRUD + `useTestRemoteTarget()`.
  Mutations `invalidateQueries` on success (existing convention, cf.
  `useProxyHosts.ts`). `Backups.tsx` is refactored onto these hooks (fixes the
  inline-query deviation and the broken `useState(() => …)` settings hydration —
  replaced by controlled form state seeded from `useBackupSettings` via `useEffect`).

**Components (under `frontend/src/pages/Backups.tsx` + new
`frontend/src/components/backups/`):**

| Component | Purpose |
|---|---|
| `BackupScheduleCard.tsx` | Replaces the dead-knob "Configuration" card: enable toggle, frequency picker (Daily @ time / Weekly @ day+time / Custom cron with validation feedback), retention count, remote retention count |
| `BackupEncryptionCard.tsx` | Enable toggle + passphrase set/change (write-only; shows "passphrase is set" state; explicit "cannot be recovered" warning) |
| `RemoteTargetsCard.tsx` + `RemoteTargetFormDialog.tsx` | Target list with status badges + test button; form with type switch (S3/SFTP); secret inputs are `type="password"`, blank-on-edit ("leave blank to keep current"), never populated from API |
| `RestoreDialog.tsx` | Extracted from inline dialog; adds passphrase input when `filename.endsWith('.age')`, shows validate results + `encryption_key_required` / legacy-format warnings, shows `restart_required` outcome |
| `UploadBackupButton.tsx` | File picker (`.zip,.age,.db`) → upload → validate feedback → appears in list |

Backup table gains columns/badges: type (`manual|scheduled|pre_restore|uploaded` —
replaces the current filename-sniffing `includes('auto')` hack at `Backups.tsx:150`),
encrypted lock icon, remote-copy status. All new interactive elements get
`data-testid`s following the existing `backup-*` naming.

**i18n:** every new user-facing string added to **all 5 locales**
(`frontend/src/locales/{de,en,es,fr,zh}/translation.json`) under `backups.*`
(estimated ~45 new keys: `schedule.*`, `encryption.*`, `remoteTargets.*`,
`upload.*`, `validate.*`, `restoreWarnings.*`).

**Complexity:** Medium.

### 3.9 Security Considerations

| Area | Measure |
|---|---|
| Path traversal | Keep the existing `filepath.Base`-equality + prefix checks and `SafeJoinPath`; `backup_handler_sanitize_test.go` must keep passing unmodified; uploaded files get server-generated names (client filename discarded) |
| Zip-slip / tar-slip | Existing `SafeJoinPath` extraction path reused for all restores (uploads included) |
| Symlink attacks & mode bits | New: reject zip entries whose mode has `os.ModeSymlink` set before extraction (v1 gap — `unzipWithSkip` currently writes whatever the entry claims); additionally **ignore the archive-supplied permission bits entirely** — force every extracted regular file to `0o600` and every directory to `0o700` regardless of `f.Mode()` (v1 gap — `unzipWithSkip` at `backup_service.go:740` currently calls `os.OpenFile(fpath, ..., f.Mode())`, so an uploaded/crafted archive could claim world-writable or setuid/setgid bits); add regression tests for both |
| Decompression bombs | Per-entry cap now **scales with the declared size**: for v2 archives, each entry's `LimitedReader` is bounded by that entry's own `ManifestEntry.Size` (from the checksum-verified manifest) **+ 64 KiB slack** — not a flat 100 MB — so a `charon.db` entry larger than 100 MB (plausible today given `RequestLog` growth) is no longer permanently unrestorable. For v1/legacy archives (no manifest to consult), fall back to a generous flat cap of **2 GiB per entry**. Independent of per-entry sizing, keep a hard **total extracted bytes** cap per archive (`maxTotalExtractedSize = 4 GiB`, raised from an earlier 2 GiB to comfortably clear a >100 MB DB plus caddy/crowdsec state) and an **entry count** cap (10 000), both computed independently of the manifest's own declared sizes so a crafted manifest cannot bypass them. **Required test:** a synthetic >100 MB `charon.db` backs up and restores round-trip without hitting the decompression limit; a manifest entry whose actual streamed bytes exceed `Size + slack` is rejected. |
| Upload limits | `http.MaxBytesReader` 512 MB on `/backups/upload`; multipart parsed with the same cap; reject non-zip/non-age/non-sqlite by magic bytes, not extension alone |
| AuthN/AuthZ | All routes stay inside `protected` + `RequireManagementAccess`; see the auth policy summary table in §3.3 for the definitive per-route decision — key changes from v1: `Download` moves to admin-only (full-DB exfiltration risk), and **all** `remote-targets` routes including `GET` (list) are admin-only (hostnames/usernames/buckets are sensitive even without secrets); `GET /backups` and `GET /backups/settings` remain management-level; align `Backups.tsx` `canCreateBackup` (currently shows the button to `role === 'user'` although the API is admin-only — fix frontend gating) |
| Secrets never echoed | Remote-target `secrets` and encryption passphrase are write-only; responses expose `secrets_set` / `encryption_passphrase_set` booleans only |
| Secrets never logged | Passphrases/credentials excluded from every log field; error wrapping must not embed credential values (`fmt.Errorf("context: %w", err)` on driver errors only); code review gate in Commit 5 |
| SSRF | Remote endpoints validated via `network` package (loopback/link-local/metadata/reserved blocked; RFC1918 allowed) at save + dial time (§3.7) |
| SFTP MITM | Mandatory host-key pinning (`ssh.FixedHostKey`); `InsecureIgnoreHostKey` banned |
| Rate limiting | Restore/upload/test-connection are admin actions behind existing auth middleware; test-connection additionally debounced client-side; no new public surface |
| GORM security | New models use parameterized GORM APIs only; `./scripts/scan-gorm-security.sh --check` must report zero CRITICAL/HIGH (DoD 1.5 — triggered because `backend/internal/models/**` changes) |
| Audit | Create/restore/delete/settings-change/target-change logged through the existing `securityService`/request-logger pattern already used in `backup_handler.go` |

### 3.10 Error Handling & Edge Cases

- **Disk full:** `CreateBackup` pre-checks `GetAvailableSpace()` ≥ 2× current DB size;
  respond `507`-style `500` with `error_code: "backup_insufficient_space"` (existing
  `respondPermissionError` path retained for EACCES).
- **Backup dir deleted at runtime:** `MkdirAll` before every create (idempotent).
- **Cron spec valid but pathological** (e.g. `* * * * *`): allowed but UI warns below
  hourly; retention still bounds disk use.
- **Concurrent restores / backup-during-restore:** `BackupService.mu` serializes
  create/restore; second request gets `409 {"error": "another backup or restore is in progress"}`.
- **Encrypted backup + lost passphrase:** unrecoverable by design; delete-only.
- **Remote target unreachable during scheduled run:** copy row `failed`, local backup
  unaffected; retried on next scheduled run only (no retry queue in Beta).
- **Restore of a backup from a newer app version:** blocked in V3 with explicit error;
  older-version archives restore with a warning (GORM AutoMigrate upgrades the schema
  on next boot, and immediately after rehydrate the running AutoMigrated schema is a
  superset — rehydrate copies only tables present in both, existing behavior).
- **`ListBackups` vs `BackupRecord` drift** (file deleted manually on disk): list
  endpoint reconciles — records whose file is missing are marked `status: "deleted"`
  and hidden; files without records synthesize legacy entries (§3.3.1).

---

## 4. Implementation Plan (Phases)

Phases map 1:1 onto the commits in §6.

**Phase 1 — Playwright specs (behavior first, `test.fixme`):** new specs for
schedule settings, encryption, upload-restore, remote targets; correct `.tar.gz`
fixture names to `.zip` in `tests/utils/phase5-helpers.ts` usage.

**Phase 2 — Backend:** manifest/format v2 + models + settings + safe restore +
encryption + remote storage, TDD per component (see Commits 2–3 for file lists).

**Phase 3 — Frontend:** API clients → hooks → components → page refactor + i18n.

**Phase 4 — Integration & hardening:** enable E2E (`npx playwright test
--project=firefox` from repo root), symlink/bomb regression tests, coverage
top-up to gates.

**Phase 5 — Docs & deployment:** `docs/features/disaster-recovery.md` (new),
correct `docs/features/backup-restore.md`, `docs/features.md` link,
`ARCHITECTURE.md` (backup subsystem, new deps, data-flow), ignore-file fixes (§7).

**Complexity estimates:**

| Component | Complexity |
|---|---|
| Manifest/format v2 + service refactor | M |
| Safe-restore pipeline | **H** |
| Encryption (age) | M |
| Remote storage (S3+SFTP+SSRF) | M–H |
| Models + settings + reschedule | M |
| Handlers/API | M |
| Frontend | M |
| E2E + docs | M |

---

## 5. Acceptance Criteria

Issue #32 mapping: backups contain all critical data (R1: DB + caddy certs +
crowdsec + settings-in-DB, checksummed) ✔; restore works flawlessly (R4–R6 validated
pipeline + rollback) ✔; automatic backups run on schedule (R3, user-configurable) ✔;
remote backup options available (R8, S3 + SFTP) ✔.

Definition of Done (CLAUDE.md, in order):

- [ ] Playwright E2E: `cd /projects/Charon && npx playwright test --project=firefox`
      — all backup specs pass (including newly-enabled ones); existing
      `backups-create` / `backups-restore` / `backup-restore-e2e` suites green.
- [ ] GORM security scan: `./scripts/scan-gorm-security.sh --check` — zero
      CRITICAL/HIGH (triggered: `backend/internal/models/**` + GORM queries).
- [ ] `bash scripts/local-patch-report.sh` produces both artifacts; patch coverage ≥ 90%.
- [ ] CodeQL Go + JS via `lefthook run pre-commit` — zero high/critical; Trivy
      (`make trivy`) clean incl. the three new Go deps.
- [ ] `make lint-fast` / staticcheck clean; no `--no-verify`.
- [ ] Coverage: `scripts/go-test-coverage.sh` ≥ 85%; `scripts/frontend-test-coverage.sh` ≥ 85%.
- [ ] `cd frontend && npm run type-check` clean.
- [ ] Builds: `cd backend && go build ./...`; `cd frontend && npm run build`.
- [ ] All existing backup tests (`backup_service_*_test.go`,
      `backup_handler*_test.go`, `frontend/src/api/__tests__/backups.test.ts`) pass —
      sanitize tests **unmodified**.
- [ ] Behavior checks: legacy v1 zip restores with warning; checksum-tampered v2
      archive rejected before any live mutation; failed restore restores pre_restore
      state; wrong age passphrase → 400 without side effects; secrets absent from all
      API responses and logs; schedule change takes effect without restart; a restore
      whose live rehydrate is exhausted persists a durable `.pending-restore` file and
      a subsequent process boot actually swaps it into place before GORM opens the DB
      (verified by an integration test that restarts the process in-test); a
      corrupted/tampered pending-restore file is never installed and the prior
      database remains authoritative; a >100 MB synthetic DB backs up and restores
      round-trip without hitting the decompression cap; SFTP discovery never
      authenticates to an unpinned/mismatched host.
- [ ] No debug prints / console.log / dead code.

---

## 6. Commit Slicing Strategy

**Decision: ONE feature = ONE PR** (`feature/backuprestore` → default branch),
merged only when complete. Five ordered commits per the CLAUDE.md suggested
sequence. Each commit builds and passes its validation gate; the PR passes the full
DoD before merge. No worktrees; all work on the current branch.

### Commit 1 — `test(e2e): add backup v2 specs as fixmes and fix backup fixtures`

- **Scope:** New Playwright specs encoding target behavior, all `test.fixme`;
  fixture correction.
- **Files:** `tests/tasks/backups-schedule.spec.ts` (new),
  `tests/tasks/backups-encryption.spec.ts` (new),
  `tests/tasks/backups-remote-targets.spec.ts` (new),
  `tests/tasks/backups-upload-restore.spec.ts` (new),
  `tests/tasks/backups-create.spec.ts` + `tests/tasks/backups-restore.spec.ts` +
  `tests/utils/phase5-helpers.ts` (fixture filenames `.tar.gz` → `.zip`; additive
  mock fields).
- **Dependencies:** none.
- **Gate:** `npx playwright test --project=firefox` — existing suites green, new
  specs skipped as fixme.

### Commit 2 — `refactor(backend): backup format v2 foundation and models (no behavior change)`

- **Scope:** Types/contracts/foundation. Manifest types + checksum writer; new GORM
  models + AutoMigrate registration; `NewBackupService` signature change (db +
  encryption + crowdsec dir threaded through, cron spec still default); settings-key
  constants + legacy-knob migration; `remotestorage.Uploader` interface (no impls);
  new deps in `backend/go.mod`.
- **Files:** `backend/internal/services/backup_service.go`,
  `backend/internal/services/backup_manifest.go` (new),
  `backend/internal/models/{backup_record,backup_remote_copy,remote_storage_target}.go` (new),
  `backend/internal/api/routes/routes.go` (AutoMigrate + constructor call),
  `backend/internal/services/remotestorage/remotestorage.go` (new),
  `backend/go.mod`/`go.sum`, existing `backup_service_*_test.go` updates + new
  model/manifest unit tests.
- **Dependencies:** Commit 1 (fixtures).
- **Gate:** `go build ./... && go test ./...`; GORM scan clean; staticcheck clean;
  v1 backups still create/restore byte-compatibly (regression test).

### Commit 3 — `feat(backend): v2 backups — validated restore, encryption, schedule, remote storage`

- **Scope:** Behavior. Manifest+crowdsec in `CreateBackupWithOptions`;
  `RestoreBackupSafe` pipeline (validate → pre_restore → apply → Caddy
  `ApplyConfig` → rollback); durable pending-restore file + boot-time swap consumer
  (closes the non-functional v1 `restart_required` path); age encryption;
  `Reschedule` + settings honored (incl. generic-settings-endpoint guard and
  `isSensitiveSettingKey` fix); `remotestorage/{s3,sftp}.go` + SSRF checks +
  pinned-host-key SFTP discovery; upload/validate/settings/remote-target handlers +
  routes (incl. admin-gating all `remote-targets` routes); Download admin-gating;
  symlink/mode-bit/total-size/scaled-per-entry extraction hardening; remote-upload
  goroutine lifecycle tied to `Stop()`; routing regression coverage for the new
  static routes.
- **Files:** `backend/internal/services/backup_service.go`,
  `backend/internal/services/backup_encryption.go` (new),
  `backend/internal/services/backup_remote_service.go` (new),
  `backend/internal/services/remotestorage/{s3,sftp}.go` (new),
  `backend/internal/database/pending_restore.go` (new),
  `backend/cmd/api/main.go` (wire `ApplyPendingRestore` before the default
  startup's `database.Connect` call),
  `backend/internal/api/handlers/backup_handler.go`,
  `backend/internal/api/handlers/backup_remote_handler.go` (new),
  `backend/internal/api/handlers/settings_handler.go` (`isSensitiveSettingKey`
  `"passphrase"` fragment; `backup.*` prefix guard in `UpdateSetting`),
  `backend/internal/api/routes/routes.go`, table-driven unit tests for every new
  path, specifically including: tampered-checksum rejection, wrong-passphrase
  rejection, SSRF-rejection, symlink-entry rejection, extracted-file-mode
  clamping, **>100 MB synthetic-DB backup/restore round trip against the
  scaled per-entry cap**, **pending-restore boot-swap integration test (write
  pending file → simulate restart via `ApplyPendingRestore` → assert swap; corrupt
  pending file → assert rejection and old DB retained)**, **SFTP discovery test
  asserting no `PasswordCallback`/`PublicKeyCallback` invocation against an
  unpinned host**, **`httptest` routing regression test for `/backups/settings`
  and `/backups/remote-targets*` vs. the `:filename` wildcard**, and a fake
  `Uploader` for remote-storage flow tests.
- **Dependencies:** Commit 2.
- **Gate:** `go test ./...` incl. all legacy backup suites and all tests listed
  above; sanitize tests unmodified & green; `scripts/go-test-coverage.sh` ≥ 85%;
  GORM + CodeQL Go clean.

### Commit 4 — `feat(frontend): backup schedule, encryption, upload-restore, and remote targets UI`

- **Scope:** API clients, hooks, components, page refactor, i18n ×5.
- **Files:** `frontend/src/api/backups.ts` (+ tests),
  `frontend/src/hooks/{useBackups,useRemoteTargets}.ts` (new, + tests),
  `frontend/src/components/backups/*` (new, + Vitest/MSW tests),
  `frontend/src/pages/Backups.tsx` (refactor onto hooks; remove dead knobs; fix
  role gating), `frontend/src/locales/{de,en,es,fr,zh}/translation.json`.
- **Dependencies:** Commit 3 (real API shapes).
- **Gate:** `npm run type-check`, `npm run build`,
  `scripts/frontend-test-coverage.sh` ≥ 85%; CodeQL JS clean.

### Commit 5 — `feat: enable backup E2E, hardening pass, and disaster recovery docs`

- **Scope:** Flip `test.fixme` → live; end-to-end hardening fixes surfaced by E2E;
  docs; ignore-file fixes (§7); final DoD sweep.
- **Files:** `tests/tasks/backups-*.spec.ts`,
  `tests/integration/backup-restore-e2e.spec.ts`,
  `docs/features/disaster-recovery.md` (new — cold restore, off-host restore,
  `CHARON_ENCRYPTION_KEY` migration, passphrase caveats, remote-copy retrieval,
  and the restart-required/pending-restore boot-swap behavior including the
  documented window where Caddy/CrowdSec files are already updated but the
  database swap completes only on the next restart),
  `docs/features/backup-restore.md` (corrected), `docs/features.md`,
  `ARCHITECTURE.md`, `.gitignore`,
  `scripts/pre-commit-hooks/block-data-backups-commit.sh`.
- **Dependencies:** Commits 1–4.
- **Gate:** Full DoD (§5), every checkbox.

### Rollback & Contingency (PR-level)

- Each commit is revertible in reverse order; Commit 2's foundation keeps v1
  behavior byte-compatible, so reverting 3–5 restores a working v1 system.
- The archive format is forward-versioned: if v2 ships a defect, v1 restore paths
  are untouched and every v2 archive self-identifies via manifest — a `fix:`
  follow-up can gate v2 creation behind a setting without breaking restores.
- If remote storage slips the Beta window (Open Question Q1), Commit 3 can land
  with the `remotestorage` package + interface but routes/UI feature-flagged off —
  the commit boundary is drawn so S3/SFTP files are separable.
- Emergency: the PR merges only complete; there is no partial-merge state to roll
  back in production.

---

## 7. Ignore-File & Repo Hygiene Review

| File | Finding | Action (Commit 5) |
|---|---|---|
| `.gitignore:141` | `/data/backups/` is root-anchored; `backend/data/backups/` NOT covered | Change to `data/backups/` (unanchored) or add `backend/data/backups/`; also add `*.zip.age` under data paths |
| `backend/data/backups/charon_backup_20251217_144822.db` | Exists on disk, **NOT committed** (verified `git ls-files`) — local dev artifact only | No history rewrite needed; covered once .gitignore fixed; do not commit |
| `scripts/pre-commit-hooks/block-data-backups-commit.sh` | Only matches `data/backups/*`; misses `backend/data/backups/*` | Extend case pattern to `*data/backups/*` |
| `.dockerignore` | No backup-dir entry (only `.vscode.backup*/`) | Add `**/data/backups/` so local backups never enter build context |
| `.codecov.yml` | — | Verify new `remotestorage/` package is inside coverage paths; no ignore entries needed |
| `Dockerfile` | New Go deps (`age`, `minio-go`, `pkg/sftp`) are pure Go — no CGO/system packages needed; `mattn/go-sqlite3` CGO already configured | No change expected; verify multi-stage build still passes `make trivy` |
| `docs/plans/current_spec.md` | Previous (completed) plan archived | Done: `docs/plans/archive/2026-07-01_flaky-testmigratecommand-tempdir-race-fix.md` |

---

## 8. Open Questions for the User

1. **Beta scope — remote storage:** Ship S3 + SFTP together (as specced), or S3
   first with SFTP in a fast-follow *commit* (still same PR/feature per the
   one-feature-one-PR rule — i.e., hold the PR until both land)? Recommendation:
   both; SFTP is small once the interface exists.
2. **Encryption default:** Off by default (recommended for Beta — passphrase loss is
   unrecoverable and the audience is novices), or on-by-default with forced
   passphrase setup during onboarding?
3. **Schedule granularity:** Is the proposed simplified UI (Daily/Weekly + custom
   cron escape hatch) right, or should Beta expose only Daily-at-time with cron
   deferred?
4. **Legacy raw `.db` backups (v0):** The plan supports them only via the upload
   endpoint (magic-byte detection → wrapped to v2). Is that sufficient, or must
   raw `.db` files already present in `data/backups/` also be listed/restorable
   in place? (Current `ListBackups` has never listed them — they predate v1 zips.)
5. **Retention semantics:** Keep count-based retention (current behavior, specced)
   or switch to the day-based retention the docs currently (incorrectly) promise?
6. **`pre_restore` backups:** Exempt from scheduled pruning (specced) — should they
   instead auto-expire after N days to bound disk usage?

---

**Next step:** on approval, hand off to the `supervisor` agent to review this spec,
then to `management`/implementation agents following §6.
