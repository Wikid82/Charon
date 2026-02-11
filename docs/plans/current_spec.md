---
post_title: Permissions Integrity Plan
author1: "Charon Team"
post_slug: permissions-integrity-plan-non-root
microsoft_alias: "charon"
featured_image: >-
  https://wikid82.github.io/charon/assets/images/featured/charon.png
categories:
  - security
tags:
  - permissions
  - non-root
  - diagnostics
  - settings
summary: "Plan to harden non-root permissions, add diagnostics, and align
  saves."
post_date: "2026-02-11"
---

## Permissions Integrity Plan — Non-Root Containers, Notifications, Saves,<br>
and Dropdown State

Last updated: 2026-02-11

## 1) Introduction

Running Charon as a non-root container should feel like a locked garden gate:
secure, predictable, and fully functional. Today, permission mismatches on
mounted volumes can silently corrode core features—notifications, settings
saves, and dropdown selections—because persistence depends on writing to
`/app/data`, `/config`, and related paths. This plan focuses on a full, precise
remediation: map every write path, instrument permissions, tighten error
handling, and make the UI reveal permission failures in plain terms.

**Objectives**
- Ensure all persistent paths are writable for non-root execution without
  weakening security.
- Make permission errors visible and actionable from API to UI.
- Reduce multi-request settings saves to avoid partial writes and improve
  reliability.
- Align notification settings fields between frontend and backend.
- Provide a clear path for operators to set correct volume ownership.

## Handoff Contract

Use this contract to brief implementation and QA. All paths and schemas must
match the plan.

```json
{
  "endpoints": {
    "GET /api/v1/system/permissions": {
      "response_schema": {
        "paths": [
          {
            "path": "/app/data",
            "required": "rwx",
            "writable": false,
            "owner_uid": 1000,
            "owner_gid": 1000,
            "mode": "0755",
            "error": "permission denied",
            "error_code": "permissions_write_denied"
          }
        ]
      }
    },
    "POST /api/v1/system/permissions/repair": {
      "request_schema": {
        "paths": ["/app/data", "/config"],
        "group_mode": false
      },
      "response_schema": {
        "paths": [
          {
            "path": "/app/data",
            "status": "repaired",
            "owner_uid": 1000,
            "owner_gid": 1000,
            "mode_before": "0755",
            "mode_after": "0700",
            "message": "ownership and mode updated"
          },
          {
            "path": "/config",
            "status": "error",
            "error_code": "permissions_readonly",
            "message": "read-only filesystem"
          }
        ]
      }
    }
  }
}
```

## 2) Research Findings

### 2.1 Runtime Permissions and Startup Flow

- Container entrypoint:
  [.docker/docker-entrypoint.sh](.docker/docker-entrypoint.sh)
  - `is_root()` and `run_as_charon()` drop privileges using `gosu`.
  - Warns if `/app/data` or `/config` is not writable; does not repair unless
    root.
  - Creates `/app/data/caddy`, `/app/data/crowdsec`, `/app/data/geoip` and
    `chown` only when root.
  - If the container is started with `--user` (non-root), it cannot `chown` or
    repair volume permissions.

- Docker runtime image: [Dockerfile](Dockerfile)
  - Creates `charon` user (`uid=1000`, `gid=1000`) and sets ownership of `/app`,
    `/config`, `/var/log/crowdsec`, `/var/log/caddy`.
  - Entry point starts as root, then drops privileges; this is good for dynamic
    socket group handling but still depends on host volume ownership.
  - Default environment points DB and data to `/app/data`.

- Compose volumes:
  [.docker/compose/docker-compose.yml](.docker/compose/docker-compose.yml)
  - `cpm_data:/app/data` and `caddy_config:/config` are mounted without a user
    override.
  - `plugins_data:/app/plugins:ro` is read-only, so plugin operations should
    never require writes there.

### 2.2 Persistent Writes and Vulnerable Paths

- Backend config creates directories with restrictive permissions (0700):
  - [backend/internal/config/config.go](backend/internal/config/config.go)
    - `Load()` calls `os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o700)`
    - `os.MkdirAll(cfg.CaddyConfigDir, 0o700)`
    - `os.MkdirAll(cfg.ImportDir, 0o700)`
  - If `/app/data` is owned by root and container runs as non-root, startup can
    fail or later writes can silently fail.

- Database writes:
  [backend/internal/database/database.go](backend/internal/database/database.go)
  - SQLite file at `CHARON_DB_PATH` (default `/app/data/charon.db`).
  - Read-only DB or directory permission failures block settings, notifications,
    and any save flows.

- Backups (writes to `/app/data/backups`):
  -
    [backend/internal/services/backup_service.go][backup-service-go]
  - Uses `os.MkdirAll(backupDir, 0o700)` and `os.Create()` for ZIPs.

- Import workflows write under `/app/data/imports`:
  -
    [backend/internal/api/handlers/import_handler.go][import-handler-go]
  - Writes to `imports/uploads/` using `os.MkdirAll(..., 0o755)` and
    `os.WriteFile(..., 0o644)`.

### 2.3 Notifications and Settings Persistence

- Notification providers and templates are stored in DB:
  -
    [backend/internal/services/notification_service.go][notification-service-go]
  -
    [backend/internal/api/handlers/<br>
    notification_handler.go][notification-handler-go]
  - UI:
    [frontend/src/pages/Notifications.tsx][notifications-page-tsx]

- Security notification settings are stored in DB:
  - Backend:
    [backend/internal/services/<br>
    security_notification_service.go][security-notification-service-go]
  - Handler:
    [backend/internal/api/handlers/<br>
    security_notifications.go][security-notifications-handler-go]
  - UI modal:
    [frontend/src/components/<br>
    SecurityNotificationSettingsModal.tsx][security-notification-modal-tsx]

**Field mismatch discovered:**
- Frontend expects `notify_rate_limit_hits` and `email_recipients` and also
  offers `min_log_level = fatal`.
- Backend model
  [backend/internal/models/notification_config.go][notification-config-go]
  only includes:
  - `Enabled`, `MinLogLevel`, `NotifyWAFBlocks`, `NotifyACLDenies`,
    `WebhookURL`.
  - Handler validation allows `debug|info|warn|error` only. This mismatch can
    cause failed saves or silent drops, and it is adjacent to permissions issues
    because a permissions error amplifies the confusion.

### 2.4 Settings and Dropdown Persistence

- System settings save path:
  - UI:
    [frontend/src/pages/SystemSettings.tsx][system-settings-tsx]
  - API client: [frontend/src/api/settings.ts](frontend/src/api/settings.ts)
  - Handler:
    [backend/internal/api/handlers/settings_handler.go][settings-handler-go]
  - Current UI saves multiple settings via multiple requests; a write failure
    mid-way can lead to partial persistence.

- SMTP settings save path:
  - UI:
    [frontend/src/pages/SMTPSettings.tsx][smtp-settings-tsx]
  - Backend:
    [backend/internal/services/mail_service.go][mail-service-go]

- Dropdowns use Radix Select:
  - Component:
    [frontend/src/components/ui/Select.tsx][select-component-tsx]
  - If API writes fail, the UI state can appear to “stick” until a reload resets
    it.

### 2.5 Initial Hygiene Review

- [.gitignore](.gitignore), [.dockerignore](.dockerignore),
  [codecov.yml](codecov.yml) currently do not require changes for permissions
  work.
- Dockerfile may require optional enhancements to accommodate PUID/PGID or a
  dedicated permissions check, but no mandatory change is confirmed yet.

## 3) Technical Specifications

### 3.1 Data Paths, Ownership, and Required Access

| Path | Purpose | Required Access | Notes |
| --- | --- | --- | --- |
| `/app/data` | Primary data root | rwx | Note A |
| `/app/data/charon.db` | SQLite DB | rw | DB and parent dir must be writable |
| `/app/data/backups` | Backup ZIPs | rwx | Created by backup service |
| `/app/data/imports` | Import uploads | rwx | Used by import handler |
| `/app/data/caddy` | Caddy state | rwx | Caddy writes certs and data |
| `/app/data/crowdsec` | CrowdSec persistent config | rwx | Note B |
| `/app/data/geoip` | GeoIP database | rwx | MaxMind GeoIP DB storage |
| `/config` | Caddy config | rwx | Managed by Caddy |
| `/var/log/caddy` | Caddy logs | rwx | Writable when file logging enabled |
| `/var/log/crowdsec` | CrowdSec logs | rwx | Local bouncer and agent logs |
| `/app/plugins` | Plugins | r-x | Should not be writable in production |

Notes:
- Note A: Must be owned by runtime user or group-writable.
- Note B: Entry point chown when root.

### 3.2 Permission Readiness Diagnostics

**Goal:** Provide definitive, machine-readable permission diagnostics for UI and
logs.

**Proposed API**

- `GET /api/v1/system/permissions`
  - Returns a list of paths, expected access, current uid/gid ownership, mode
    bits, writeability, and a stable `error_code` when a check fails.
  - Example response schema:
    ```json
    {
      "paths": [
        {
          "path": "/app/data",
          "required": "rwx",
          "writable": false,
          "owner_uid": 1000,
          "owner_gid": 1000,
          "mode": "0755",
          "error": "permission denied",
          "error_code": "permissions_write_denied"
        }
      ]
    }
    ```

**Writable determination (explicit, non-destructive):**
- For each path, perform `os.Stat` to capture owner/mode and to confirm the
  path exists.
- If the `required` access does not include `w` (for example `r-x`), skip any
  writeability probe, do not set `error_code`, and optionally set
  `status=expected_readonly` to clarify that non-writable is expected.
- If the path is a directory, attempt a non-destructive writeability probe by
  creating a temp file in the directory (`os.CreateTemp`) and then immediately
  removing it.
- If the path is a file, attempt to open it with write permissions
  (`os.OpenFile` with `os.O_WRONLY` or `os.O_RDWR`) without truncation and close
  immediately.
- Do not modify file contents or truncate; no destructive writes are allowed.
- If any step fails, set `writable=false` and return a stable `error_code`.

**Error code coverage (explicit):**
- The `error_code` field SHALL be returned by diagnostics responses for both
  `GET /api/v1/system/permissions` and `POST /api/v1/system/permissions/repair`
  whenever a per-path check fails.
- For a `GET` diagnostics entry that is healthy, omit `error_code` and `error`.
- Diagnostics error mapping MUST distinguish read-only vs permission denied:
  - `EROFS` -> `permissions_readonly`
  - `EACCES` -> `permissions_write_denied`

- `POST /api/v1/system/permissions/repair` (optional)
  - Only enabled when process is root.
  - Attempts to `chown` and `chmod` only for known safe paths.
  - Returns a per-path remediation report.
  - **Request schema (explicit):**
    ```json
    {
      "paths": ["/app/data", "/config"],
      "group_mode": false
    }
    ```
  - **Response schema (explicit):**
    ```json
    {
      "paths": [
        {
          "path": "/app/data",
          "status": "repaired",
          "owner_uid": 1000,
          "owner_gid": 1000,
          "mode_before": "0755",
          "mode_after": "0700",
          "message": "ownership and mode updated"
        },
        {
          "path": "/config",
          "status": "error",
          "error_code": "permissions_readonly",
          "message": "read-only filesystem"
        }
      ]
    }
    ```
  - **Target ownership and mode rules (explicit):**
    - Use runtime UID/GID (effective process UID/GID at time of request).
    - Directory mode: `0700` by default; `0770` when `group_mode=true`.
    - File mode: `0600` by default; `0660` when `group_mode=true`.
    - `group_mode` applies to all provided paths; per-path overrides are not
      supported in this plan.
  - **Per-path behavior and responses (explicit):**
    - For each path in `paths`, validate and act independently.
    - If a path is missing, return `status=error` with
      `error_code=permissions_missing_path` and do not create it.
    - If a path resolves to a directory, apply directory mode rules and
      ownership updates.
    - If a path resolves to a file, apply file mode rules and ownership
      updates.
    - If a path resolves to neither a file nor directory, return
      `status=error` with `error_code=permissions_unsupported_type`.
    - If a path is already correct, return `status=skipped` with a
      `message` indicating no change.
    - If any mutation fails (read-only FS, permission denied), return
      `status=error` and include a stable `error_code`.
  - **Allowlist + Symlink Safety:**
    - **Allowlist roots (hard-coded, immutable):**
      - `/app/data`
      - `/config`
      - `/var/log/caddy`
      - `/var/log/crowdsec`
    - Only allow subpaths that remain within these roots after
      `filepath.Clean` and `filepath.EvalSymlinks` checks.
    - Resolve each requested path with `filepath.EvalSymlinks` and reject any
      that resolve outside the allowlist roots.
    - Use `os.Lstat` to detect and reject symlinks before any mutation.
    - Use no-follow semantics for any filesystem operations (reject if any path
      component is a symlink).
    - If a path is missing, return a per-path error instead of creating it.
  - **Path Normalization (explicit):**
    - Only accept absolute paths and reject relative inputs.
    - Normalize with `filepath.Clean` before validation.
    - Reject any path that resolves to `.` or contains `..` after normalization.
    - Reject any request where normalization would change the intended path
      outside the allowlist roots.

**Scope:**
- Diagnostics SHALL include all persistent write paths listed in section 3.1,
  including `/app/data/geoip`, `/var/log/caddy`, and `/var/log/crowdsec`.
- Any additional persistent write paths referenced elsewhere in this plan SHALL
  be included in diagnostics as they are added.
- Diagnostics SHALL include `/app/plugins` as a read-only check with
  `required: r-x`. A non-writable result for `/app/plugins` is expected and
  MUST NOT be treated as a failure condition; skip the write probe and do not
  include an `error_code`.

**Backend placement:**
- New handler in `backend/internal/api/handlers/system_permissions_handler.go`.
- Utility in `backend/internal/util/permissions.go` for POSIX stat + access
  checks.

### 3.3 Access Control and Path Exposure

**Goal:** Ensure diagnostics are admin-only and paths are not exposed to non-
admins.

- `GET /api/v1/system/permissions` and `POST /api/v1/system/permissions/repair`
  must be admin-only.
- Non-admin requests SHALL return `403` with a stable error code
  `permissions_admin_only`.
- Full filesystem paths SHALL only be included for admins; non-admin errors must
  omit or redact path details.

**Redaction and authorization strategy (explicit):**
- Admin enforcement happens in the handler layer using the existing admin guard
  middleware; handlers SHALL read the admin flag from request context and fail
  closed if the flag is missing.
- Redaction happens in the error response builder at the handler boundary before
  JSON serialization. Services return a structured error with optional `path`
  and `detail` fields; the handler removes `path` and sensitive filesystem hints
  for non-admins and replaces help text with a generic remediation message.
- The redaction decision SHALL not rely on client-provided hints; it must only
  use server-side auth context.

**Non-admin response schema (redacted, brief):**
- Diagnostics (non-admin, 403):
  ```json
  {
    "error": "admin privileges required",
    "error_code": "permissions_admin_only"
  }
  ```
- Repair (non-admin, 403):
  ```json
  {
    "error": "admin privileges required",
    "error_code": "permissions_admin_only"
  }
  ```

**Save endpoint access (admin-only):**
- Settings and configuration save endpoints SHALL remain admin-only where
  applicable (e.g., system settings, SMTP settings, notification
  providers/templates, security notification settings, imports, and backups).
- If any save endpoint is currently not admin-gated, the implementation MUST add
  admin-only checks or explicitly document the exception in this plan before
  implementation.

#### 3.3.1 Admin-Gated Save Endpoints Checklist

For each endpoint below, confirm the current state and enforce admin-only access
unless explicitly documented as public.

- System settings save
  - Current: Verify admin guard is enforced in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- SMTP settings save
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- Notification providers save/update/delete
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- Notification templates save/update/delete
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- Security notification settings save
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- Import create/upload
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.
- Backup create/restore
  - Current: Verify admin guard in handler and service.
  - Target: Admin-only with `403` and stable error code on failure.
  - Verify: API call as non-admin returns `403` without write.

### 3.4 Permission-Aware Error Mapping

**Goal:** When a save fails, the user sees “why.”

- Identify key persistence actions and wrap errors with permission hints:
  - Settings saves: `SettingsHandler.UpdateSetting()` and `PatchConfig()`.
  - SMTP saves: `MailService.SaveSMTPConfig()`.
  - Notification providers/templates: `NotificationService.CreateProvider()`,
    `UpdateProvider()`, `CreateTemplate()`, `UpdateTemplate()`.
  - Security notification settings:
    `SecurityNotificationService.UpdateSettings()`.
  - Backup creation: `BackupService.CreateBackup()`.
  - Import uploads: `ImportHandler.Upload()` and `UploadMulti()`.

**Error behavior:**
- If error is permission-related (`os.IsPermission`, SQLite read-only), return a
  500 with a standard payload:
  - `error`: short message
  - `help`: actionable guidance using runtime UID/GID (e.g.,
    `chown -R <runtime_uid>:<runtime_gid> /path/to/volume`)
  - `path`: affected path (admin-only; omit or redact for non-admins)
  - `code`: stable error code (required for permission-related save failures;
    e.g., `permissions_write_failed`)

**Audit logging:**
- Log all diagnostics reads and repair attempts as audit events, including
  requestor identity, admin flag, and outcome.
- Log permission-related save failures (settings, notifications, imports,
  backups, SMTP) as audit events with error codes and redacted path details for
  non-admin contexts.

**SQLite read-only detection (explicit):**
- Map SQLite read-only failures by driver code when available (e.g.,
  `SQLITE_READONLY` and extended codes such as `SQLITE_READONLY_DB`,
  `SQLITE_READONLY_DIRECTORY`).
- Also detect string-based error messages to cover driver variations (e.g.,
  `attempt to write a readonly database`, `readonly database`, `read-only
  database`).
- If driver codes are unavailable, fall back to message matching +
  `os.IsPermission` to produce the same standard payload.

### 3.4.1 Canonical Error-Code Catalog (Diagnostics + Repair + Save Failures)

**Goal:** Provide a single source of truth for error codes used by diagnostics,
repair, and persistence failures. All responses MUST use values from this
catalog.

**Scope:**
- Diagnostics: `GET /api/v1/system/permissions`
- Repair: `POST /api/v1/system/permissions/repair`
- Save failures: settings, SMTP, notifications, security notifications,
  imports, backups

| Error Code | Scope | Meaning |
| --- | --- | --- |
| `permissions_admin_only` | Diagnostics/Repair/Save | Note 1 |
| `permissions_non_root` | Repair | Note 2 |
| `permissions_repair_disabled` | Repair | Note 3 |
| `permissions_missing_path` | Diagnostics/Repair | Path does not exist. |
| `permissions_unsupported_type` | Diagnostics/Repair | Note 4 |
| `permissions_outside_allowlist` | Repair | Note 5 |
| `permissions_symlink_rejected` | Repair | Path or a component is a symlink. |
| `permissions_invalid_path` | Diagnostics/Repair | Note 6 |
| `permissions_readonly` | Diagnostics/Repair/Save | Filesystem is read-only. |
| `permissions_write_denied` | Diagnostics/Save | Note 7 |
| `permissions_write_failed` | Save | Note 8 |
| `permissions_db_readonly` | Save | Note 9 |
| `permissions_db_locked` | Save | Note 10 |
| `permissions_repair_failed` | Repair | Note 11 |
| `permissions_repair_skipped` | Repair | No changes required for the path. |

Notes:
- Note 1: Request requires admin privileges.
- Note 2: Repair endpoint invoked without root privileges.
- Note 3: Repair endpoint disabled because single-container mode is false.
- Note 4: Path is not a file or directory.
- Note 5: Path resolves outside allowlist roots.
- Note 6: Path is relative, normalizes to `.`/`..`, or fails validation.
- Note 7: Write probe or write operation denied.
- Note 8: Write operation failed for another permission-related reason.
- Note 9: SQLite database or directory is read-only.
- Note 10: SQLite database locked; treat as transient write failure.
- Note 11: Repair attempted but failed (non-permission errors).

**Mapping rules (explicit):**
- Diagnostics uses `permissions_missing_path`, `permissions_write_denied`,
  `permissions_readonly`, `permissions_invalid_path`,
  `permissions_unsupported_type` as appropriate.
- Repair uses `permissions_admin_only`, `permissions_non_root`, or
  `permissions_repair_disabled` when blocked, and otherwise maps to the per-path
  codes above.
- Save failures use `permissions_db_readonly` when SQLite read-only is
  detected; otherwise use `permissions_write_denied` or
  `permissions_write_failed` depending on `os.IsPermission` and error context.
- Save failures SHALL always include an error code from this catalog.

### 3.5 Notification Settings Model Alignment

**Goal:** Align UI fields with backend persistence.

- Update
  [backend/internal/models/notification_config.go][notification-config-go]
  to include:
  - `NotifyRateLimitHits bool`
  - `EmailRecipients string`
- Update handler validation in
  [backend/internal/api/handlers/<br>
  security_notifications.go][security-notifications-handler-go]:
  - Keep backend validation to `debug|info|warn|error`.
- Update UI log level options to remove `fatal` and match backend validation.
- Update `SecurityNotificationService.GetSettings()` default struct to include
  new fields.

**EmailRecipients data format (explicit):**
- Input accepts a comma-separated list of email addresses.
- Split on `,`, trim whitespace for each entry, and drop empty values.
- Validate each email using existing backend validation rules.
- Store a normalized, comma-separated string joined with `, `.
- If validation fails, return a single error listing invalid entries.

**Validation and UX notes:**
- UI helper text: "Use comma-separated emails, e.g. admin@example.com,
  ops@example.com".
- Inline error highlights the invalid address(es) and does not save.
- Empty input is treated as "no recipients" and stored as an empty string.
- The UI must preserve the normalized format returned by the API.

### 3.6 Reduce Settings Write Requests

**Goal:** Fewer requests, fewer partial failures.

- Reuse existing `PATCH /api/v1/config` in
  [backend/internal/api/handlers/settings_handler.go][settings-handler-go].
- PATCH updates MUST be transactional and all-or-nothing. If any field update
  fails (validation, DB write, or permission), the transaction must roll back
  and the API must return a single failure response.
- Update
  [frontend/src/pages/SystemSettings.tsx](frontend/src/pages/SystemSettings.tsx)
  to send one patch request for all fields.
- Add failure-mode UI message that references permission diagnostics if present.

### 3.7 UX Guidance for Non-Root Deployments

- Add a settings banner or toast when permissions fail, pointing to:
  - `docker run` or `docker compose` examples
  - `chown -R <runtime_uid>:<runtime_gid> /path/to/volume` using values from
    diagnostics or configured `--user` / `CHARON_UID` / `CHARON_GID`
  - Optionally `--user <runtime_uid>:<runtime_gid>` or PUID/PGID env if added

### 3.8 PUID/PGID and --user Behavior

- If the container is started with `--user`, the entrypoint cannot `chown`
  mounted volumes.
- When `--user` is set, `CHARON_UID`/`CHARON_GID` (and any PUID/PGID
  equivalents) SHALL be treated as no-ops and only used for logging.
- Documentation must instruct operators to pre-create and `chown` host volumes
  to the runtime UID/GID when using `--user`, based on the diagnostics-reported
  UID/GID or the configured runtime values.

**Directory permission modes (0700 vs group-writable):**
- Default directory mode remains `0700` for single-user deployments.
- When PUID/PGID or supplemental group access is used, directories MAY be
  created as `0770` (or `0750` if group write is not required).
- If group-writable directories are used, ensure the runtime user is in the
  owning group and document the expected umask behavior.

### 3.9 Risk Register and Mitigations

- **Risk:** Repair endpoint could be abused in a multi-tenant environment.
  - **Mitigation:** Only enabled in single-container mode; root-only; allowlist
    paths.
- **Risk:** Adding fields to NotificationConfig might break existing migrations.
  - **Mitigation:** Use GORM AutoMigrate and default values.
- **Risk:** UI still masks failures due to optimistic updates.
  - **Mitigation:** Ensure all mutations handle error states and show help text.

### 3.9.1 Single-Container Mode Detection and Enforcement

**Goal:** Ensure repair operations are only enabled in single-container mode
and the system can deterministically report whether this mode is active.

**Detection (explicit):**
- Environment flag: `CHARON_SINGLE_CONTAINER_MODE`.
- Accepted values: `true|false` (case-insensitive). Any other value defaults
  to `false` and logs a warning.
- Default: `true` in official Dockerfile and official compose examples.
- Non-container installs (binary on host) default to `false` unless explicitly
  set.

**Enforcement (explicit):**
- The repair endpoint is disabled when single-container mode is `false`.
- The handler MUST return `403` with `permissions_repair_disabled` when the
  mode check fails, and SHALL NOT attempt any filesystem mutations.
- Diagnostics remain available regardless of mode.

**Repair gating and precedence (explicit):**
1) Admin-only check first. If not admin, return `403` with
  `permissions_admin_only`.
2) Single-container mode check second. If disabled, return `403` with
  `permissions_repair_disabled`.
3) Root check third. If not root, return `403` with `permissions_non_root`.
4) Only after all gating checks pass, proceed to path validation and mutation.

**Placement:**
- Mode detection lives in `backend/internal/config` as a boolean flag on the
  runtime config object.
- Enforcement happens in the permissions repair handler before any path
  validation or mutation.
- Log the evaluated mode and source (explicit env vs default) once at startup.

### 3.10 Spec-Driven Workflow Artifacts (Pre-Implementation Gate)

Before Phase 1 begins, update the following artifacts and confirm sign-off:

- `requirements.md` with new or refined EARS statements for permissions
  diagnostics, admin-gated saves, and error mapping.
- `design.md` with new endpoints, data flow, error payloads, and non-root
  permission remediation design.
- `tasks.md` with the phase plan, test order, verification tasks, and the
  deterministic read-only DB simulation approach.

## 4) Implementation Plan (Phased, Minimal Requests)

### Pre-Implementation Gate (Required)

1) Update `requirements.md`, `design.md`, and `tasks.md` per section 3.10.
2) Confirm the deterministic read-only DB simulation approach and exact
  invocation are documented in `tasks.md`.
3) Proceed only after the spec artifacts are updated and reviewed.

### Phase 1 — Playwright & Diagnostic Ground Truth

**Goal:** Define the expected UX and capture the failure state before changes.

1) Add E2E coverage for permissions and save failures:
  - Tests in `tests/settings/settings-permissions.spec.ts`:
    - Simulate DB read-only deterministically (compose override only).
    - Verify toast/error text on save failure.
   - Tests for dropdown persistence in System Settings and SMTP:
     - Ensure selections persist after reload when writes succeed.
     - Ensure UI reverts with visible error on write failure.
   - Security notification log level options:
     - Ensure `fatal` is not present in the dropdown options.


1a) Deterministic failure simulation setup (aligned with E2E workflow):
  - Use a docker-compose override to bind-mount a read-only DB file for E2E.
    This is the single supported approach for deterministic DB read-only
    simulation.
  - Override file (example name):
    `.docker/compose/docker-compose.e2e-readonly-db.override.yml`.
  - Read-only DB setup sequence (explicit):
    1. Run the E2E rebuild skill first to ensure the base container and
       baseline volumes are fresh and healthy.
    2. Start a one-off container (or job) with a writable volume.
    3. Run migrations and seed data to create the SQLite DB file in that
       writable location.
    4. Stop the one-off container and bind-mount the DB file into the E2E
       container as read-only using the override.
  - Exact invocation (Docker E2E mode):
    ```bash
    # Step 1: rebuild E2E container
    .github/skills/scripts/skill-runner.sh docker-rebuild-e2e

    # Step 2: start with override
    docker compose -f .docker/compose/docker-compose.yml \
      -f .docker/compose/docker-compose.e2e-readonly-db.override.yml up -d
    ```
  - The override SHALL mount the DB file read-only and MUST NOT require any
    application code changes or test-only flags.
  - Teardown/cleanup after the test run (explicit):
    1. Stop and remove the override services/containers started for the
       read-only run.
    2. Remove any override-specific volumes used for the read-only DB file
       to avoid cross-test contamination.
    3. Re-run the E2E rebuild skill before the next E2E session to restore
       the standard writable DB state.
  - Add a planned VS Code task or skill-runner entry to make this workflow
    one-command and discoverable (example task label: "Test: E2E Readonly
    DB", command invoking the docker compose override sequence above).

2) Add a health check step in tests for permissions endpoint once available.

**Outputs:** New E2E baseline expectations for save behavior.

### Phase 2 — Backend Permissions Diagnostics & Errors

**Goal:** Make permission issues undeniable and actionable.

1) Add system permissions handler and util:
   - `backend/internal/api/handlers/system_permissions_handler.go`
   - `backend/internal/util/permissions.go`

2) Add standardized permission error mapping:
   - Wrap DB and filesystem errors in settings, notifications, imports, backups.

3) Extend security notifications model and defaults:
   - Update `NotificationConfig` fields.
   - Update handler validation for min log level or adjust UI.

**Outputs:** A diagnostics API and consistent error payloads across persistence
paths.

### Phase 3 — Frontend Save Flows and UI Messaging

**Goal:** Reduce request count and surface errors clearly.

1) System Settings:
   - Switch to `PATCH /api/v1/config` for multi-field save.
   - On error, show permission hint if provided.

2) Security Notification Settings modal:
   - Align log level options with backend.
   - Ensure new fields are saved and displayed.

3) Notifications providers:
   - Surface permission errors on save/update/delete.

**Outputs:** Fewer save calls, better error clarity, stable dropdown
persistence.

### Phase 4 — Integration and Testing

1) Run Playwright E2E tests first, before any unit tests.
2) If the E2E environment changed, rebuild using the E2E Docker skill.
3) Ensure E2E tests cover permission failure UX and dropdown persistence.
4) Run unit tests only after E2E passes.
5) Enforce 100% patch coverage for all modified lines.
6) Record any coverage gaps in `tasks.md` before adding tests.

### Phase 5 — Container & Volume Hardening

**Goal:** Provide a clear, secure non-root path.

1) Entrypoint improvements:
   - When running as root, ensure `/app/data` ownership is corrected (not only
     subdirs).
   - Log UID/GID at startup.

2) Optional PUID/PGID support:
   - If `CHARON_UID`/`CHARON_GID` are set and the container is not started with
     `--user`, re-map `charon` user or add supplemental group.
   - If `--user` is set, log that PUID/PGID overrides are ignored and volume
     ownership must be handled on the host.

3) Dockerfile/Compose review:
   - If PUID/PGID added, update Dockerfile and compose example.

**Outputs:** Hardening changes that remove the “silent failure” path.

### Phase 6 — Integration, Documentation, and Cleanup

1) Add troubleshooting docs for non-root volumes.
2) Update any user guides referencing permissions.
3) Update API docs for new endpoints:
   - Add `GET /api/v1/system/permissions` and
     `POST /api/v1/system/permissions/repair` to
     [docs/api.md](docs/api.md) with schemas, auth, and error codes.
4) Update documentation to reference `CHARON_SINGLE_CONTAINER_MODE`:
   - Add the env var description and default behavior to the primary
     configuration reference (include accepted values and fallback behavior).
   - Add or update a Docker Compose example showing
     `CHARON_SINGLE_CONTAINER_MODE=true` in the environment list.
5) Ensure `requirements.md`, `design.md`, and `tasks.md` are updated.
6) Finalize tests and ensure coverage targets are met.
7) Update [docs/features.md](docs/features.md) for any user-facing permissions
   diagnostics or repair UX changes.

## 5) Acceptance Criteria (EARS)

- WHEN the container runs as non-root and a mounted volume is not writable, THE
  SYSTEM SHALL expose a permissions diagnostic endpoint that reports the failing
  path and required access.
- WHEN the permissions repair endpoint is called by a non-root process, THE
  SYSTEM SHALL return `403` and SHALL NOT perform any filesystem mutation.
- WHEN the permissions repair endpoint is called by a non-admin user, THE SYSTEM
  SHALL return `403` with `permissions_admin_only` and SHALL NOT perform any
  filesystem mutation.
- WHEN the permissions repair endpoint is called while single-container mode is
  disabled, THE SYSTEM SHALL return `403` with `permissions_repair_disabled`
  and SHALL NOT perform any filesystem mutation.
- WHEN the permissions repair endpoint receives a path that is outside the
  allowlist, THE SYSTEM SHALL reject the request with a clear error and SHALL
  NOT touch the filesystem.
- WHEN the permissions repair endpoint receives a symlink or a path containing a
  symlinked component, THE SYSTEM SHALL reject the request with a clear error
  and SHALL NOT follow the link.
- WHEN the permissions repair endpoint receives a missing path, THE SYSTEM SHALL
  return a per-path error and SHALL NOT create the path.
- WHEN the permissions repair endpoint receives a relative path or a path that
  normalizes to `.` or `..`, THE SYSTEM SHALL reject the request and SHALL NOT
  perform any filesystem mutation.
- WHEN a user saves system, SMTP, or notification settings and the DB is read-
  only, THE SYSTEM SHALL return a clear error with a remediation hint.
- WHEN a user updates dropdown-based settings and persistence fails, THE SYSTEM
  SHALL display an error and SHALL NOT silently pretend the save succeeded.
- WHEN the security notification log level options are displayed, THE SYSTEM
  SHALL only present `debug`, `info`, `warn`, and `error`.
- WHEN security notification settings are saved, THE SYSTEM SHALL persist all
  fields that the UI presents.
- WHEN settings updates include multiple fields, THE SYSTEM SHALL apply them in
  a single request and a single transaction to avoid partial persistence.
- WHEN a non-admin user attempts to call a save endpoint, THE SYSTEM SHALL
  return `403` with `permissions_admin_only` and SHALL NOT perform any write.
- WHEN permissions diagnostics or repair endpoints are called, THE SYSTEM SHALL
  emit an audit log entry with outcome details.
- WHEN a permission-related save failure occurs, THE SYSTEM SHALL emit an audit
  log entry with a stable error code and redacted path details for non-admin
  contexts.
- WHEN a non-admin user receives a permission-related error, THE SYSTEM SHALL
  redact filesystem path details from the response payload.

## 6) Files and Components to Touch (Trace Map)

**Backend**
-
  [.docker/docker-entrypoint.sh][docker-entrypoint-sh] — permission checks
  and potential ownership fixes.
-
  [backend/internal/config/config.go][config-go] — data directory creation
  behavior.
-
  [backend/internal/api/handlers/settings_handler.go][settings-handler-go]
  — permission-aware errors, PATCH usage.
-
  [backend/internal/api/handlers/<br>
  security_notifications.go][security-notifications-handler-go]
  — validation alignment.
-
  [backend/internal/services/<br>
  security_notification_service.go][security-notification-service-go]
  — defaults, persistence.
-
  [backend/internal/models/notification_config.go][notification-config-go]
  — new fields.
-
  [backend/internal/services/mail_service.go][mail-service-go] — permission-
  aware errors.
-
  [backend/internal/services/notification_service.go][notification-service-go]
  — permission-aware errors.
-
  [backend/internal/services/backup_service.go][backup-service-go] —
  permission-aware errors.
-
  [backend/internal/util/permissions.go][permissions-util-go] — permission
  diagnostics utility.
-
  [backend/internal/api/handlers/import_handler.go][import-handler-go] —
  permission-aware errors for uploads.

**Frontend**
-
  [frontend/src/pages/SystemSettings.tsx][system-settings-tsx] — batch save
  via PATCH and better error UI.
-
  [frontend/src/pages/SMTPSettings.tsx][smtp-settings-tsx] — permission error
  messaging.
-
  [frontend/src/pages/Notifications.tsx][notifications-page-tsx] — save error
  handling.
-
  [frontend/src/components/<br>
  SecurityNotificationSettingsModal.tsx][security-notification-modal-tsx]
  — align fields.
-
  [frontend/src/components/ui/Select.tsx][select-component-tsx] — no
  functional change expected; verify for state persistence.

**Infra**
- [Dockerfile](Dockerfile)
- [.docker/compose/docker-compose.yml](.docker/compose/docker-compose.yml)

## 7) Repo Hygiene Review (Requested)

- **.gitignore:** No change required unless we add new diagnostics artifacts
  (e.g., `permissions-report.json`). If added, ignore them under root or `test-
  results/`.
- **.dockerignore:** No change required. If we add new documentation files or
  test artifacts, keep them excluded from the image.
- **codecov.yml:** No change required unless new diagnostics packages warrant
  exclusions.
- **Dockerfile:** Potential update if PUID/PGID support is added; otherwise, no
  change required.

## 8) Unit Test Plan

Backend unit tests (Go):
- Permissions diagnostics utility: validate stat parsing, writable checks, and
  error mapping for missing paths and permission denied.
- Permissions endpoints: admin-only access (403 + `permissions_admin_only`) and
  successful admin responses.
- Permissions repair endpoint:
  - Rejects non-root execution with `403` and no filesystem changes.
  - Rejects non-admin requests with `permissions_admin_only`.
  - Rejects paths outside the allowlist safe roots.
  - Rejects relative paths, `.` and `..` after normalization, and any request
    where `filepath.Clean` produces an out-of-allowlist path.
  - Rejects symlinks and symlinked path components via `Lstat` and
    `EvalSymlinks` checks.
  - Returns per-path errors for missing paths without creating them.
- Permission-aware error mapping: ensure DB read-only and `os.IsPermission`
  errors map to the standard payload fields and redact path details for non-
  admins.
- Audit logging: verify diagnostics/repair calls and permission-related save
  failures emit audit entries with redacted path details for non-admin contexts.
- Settings PATCH behavior: multi-field patch applies atomically in the
  handler/service and returns a single failure when any persistence step fails.

Frontend unit tests (Vitest):
- Diagnostics fetch handling: verify non-admin error messaging without path
  details.
- Settings save errors: ensure error toast displays remediation text and UI
  state does not silently persist on failure.

## 9) Confidence Score

Confidence: 80%

Rationale: The permissions write paths are well mapped, and the root cause (non-
root + volume ownership mismatch) is a common pattern. The only uncertainty is
the exact user environment for the failure, which will be clarified once
diagnostics are in place.

[backup-service-go]:
  backend/internal/services/backup_service.go
[import-handler-go]:
  backend/internal/api/handlers/import_handler.go
[notification-service-go]:
  backend/internal/services/notification_service.go
[notification-handler-go]:
  backend/internal/api/handlers/notification_handler.go
[notifications-page-tsx]:
  frontend/src/pages/Notifications.tsx
[security-notification-service-go]:
  backend/internal/services/security_notification_service.go
[security-notifications-handler-go]:
  backend/internal/api/handlers/security_notifications.go
[security-notification-modal-tsx]:
  frontend/src/components/SecurityNotificationSettingsModal.tsx
[notification-config-go]:
  backend/internal/models/notification_config.go
[system-settings-tsx]:
  frontend/src/pages/SystemSettings.tsx
[settings-handler-go]:
  backend/internal/api/handlers/settings_handler.go
[smtp-settings-tsx]:
  frontend/src/pages/SMTPSettings.tsx
[mail-service-go]:
  backend/internal/services/mail_service.go
[select-component-tsx]:
  frontend/src/components/ui/Select.tsx
[docker-entrypoint-sh]:
  .docker/docker-entrypoint.sh
[config-go]:
  backend/internal/config/config.go
[permissions-util-go]:
  backend/internal/util/permissions.go
