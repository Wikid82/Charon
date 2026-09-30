# Plan: Lower default heartbeat retention to 30 days and make the retention control clear (GH #1419)

Branch: `fix/uptime-heartbeat-retention-default` (single PR into `development`).
Type: `fix:` (not `(security)`). Issue: "charon db is huge (4.8 GB) due to uptime_heartbeats".
Status: IMPLEMENTED (commits 1-7 on the branch; pending PR/CI).
Companion: GH #1422 (automatic database maintenance; follow-up feature PR that owns all compaction / VACUUM work).

## 1. Introduction

### Problem

A reporter's Charon database reached 4.8 GB, almost all of it `uptime_heartbeats`. A read-only look at a comparable live instance (28 monitors at 60 s) showed:

- The v0.39.0 retention pruner works. It runs hourly, deletes in chunks, and reads `uptime.heartbeat_retention_days`.
- Steady state at the 90-day default is about 3.3 M rows, which is a 2.05 GB file. The table itself is about 280 MB; its four secondary indexes are about 770 MB:
  - `idx_heartbeat_lookup (monitor_id, status, created_at)`: 256 MB
  - `idx_heartbeat_monitor_created (monitor_id, created_at)`: 239 MB (built by the pruner's `ensureIndex`)
  - `idx_uptime_heartbeats_monitor_id (monitor_id)`: 139 MB (strict prefix of both composites, redundant)
  - `idx_uptime_heartbeats_created_at (created_at)`: 139 MB
- `auto_vacuum=0` and nothing ever runs `VACUUM` (the pruner comment says this is deliberate). About 35% of the file (about 690 MB) was free pages that never return to the OS.
- Seeding writes the default only when the row is missing, so existing installs keep 90 unless migrated.

New facts from the reporter (issue thread):

- They run **v0.43.0 (build f9d36f9)** with **about 60 monitors**, all settings at defaults.
- They report **no old container logs are available**, so no `UptimePruner` log evidence can be obtained. The pruner investigation (Section 6) is code-review driven only; the concrete finding is the query plan.
- They had **about 19 M heartbeat rows**. At 60 monitors x 1440 checks/day = 86 400 rows/day, the 90-day steady state is about **7.8 M** rows; 19 M is about 2.4x that (about 220 days of data). Either the pruner is not keeping up at this scale, or they upgraded recently from a pre-v0.39.0 build and the backlog was still draining. Section 6 records the review; the scan finding is fixed, the 19 M cause itself is not assumed.

### Decisions fixed by the user (not re-litigated here)

1. Default retention goes from 90 to 30 days.
2. Retention stays user-adjustable in the UI (range 1-3650). No "0 = keep forever".
3. An Info log line plus a release-note mention is enough for the one-time migration. No UI notice.
4. No online "compact" button in this PR. **All compaction / VACUUM work is out of this PR** and lives in the follow-up PR tracked in GH #1422.
5. **The strict migration ships** (Section 3.2). Not "no automatic migration".
6. **The bounded-scan pruner fix ships in this PR, unconditionally** (Section 3.6 / commit 5), not gated on a benchmark.
7. **The manual `sqlite3 VACUUM` workaround is documented**, marked advanced (Section 4, Phase 5).
8. A separate GH issue for localizing `systemSettings.uptime` into de/es/fr/zh has been filed (#1421). Not part of this PR.

### Objectives

1. Default becomes 30 everywhere it is defined.
2. Existing installs still on the untouched seeded 90 move to 30, without overriding a deliberate user choice.
3. The UI retention control explains units, range, and the disk tradeoff, and shows useful validation errors.
4. Drop the redundant index (immediate about 13% saving of table plus index bytes).
5. A corrupt stored retention value can never wipe history, on either write path (validation in both settings endpoints) or on read (clamp).
6. The hourly pruner scan uses the `created_at` index instead of scanning the whole table.

### Explicit tradeoff of removing compaction from this PR

Lowering retention (or the migration) frees pages inside the SQLite file but never returns them to the OS: `auto_vacuum=0`, and nothing runs `VACUUM`. **Reporter-type installs will not see the file shrink until the follow-up ships.** The database stops growing (the free pages are reused for new heartbeats), but the 4.8 GB file stays 4.8 GB. Therefore the helper text and docs must say "the file may not shrink until the database is compacted", and the docs give an honest advanced manual workaround (Section 4, Phase 5). This is accepted by design, not a bug.

## 2. Research Findings (verified against the code)

### 2.1 Every place the 90 default (or a 90-assuming value) lives

| Location | What | Action |
| --- | --- | --- |
| `backend/internal/services/uptime_config.go:19` | `defaultUptimeRetentionDays = 90` (fallback when row missing or unparseable) | Change to 30 (exported, see 3.1) |
| `backend/internal/api/routes/routes.go:797` | Seed `{Key: "uptime.heartbeat_retention_days", Value: "90"}` via `FirstOrCreate` with `Attrs` (writes only when missing) | Change to `"30"` |
| `backend/internal/api/handlers/settings_handler.go:444-448` | `uptimeSettingBounds` `{1, 3650}` for retention | Reference the shared exported constants (range unchanged) |
| `backend/internal/services/uptime_config_test.go:26,87` | Asserts `90` for default retention (lines 60/65 are the unrelated *interval* value 90) | Update to 30 |
| `backend/internal/services/uptime_pruner_test.go:70-71,105,109` | Comments and scenario assume "90d default" with rows seeded at -100d and -10d | Update comments; verify the "pass 1 default retention" case still deletes only the -100d rows |
| `backend/internal/api/handlers/settings_handler_test.go:1896` | `retention_in_bounds` uses `"90"`, just an in-range sample | Leave |
| `frontend/src/pages/__tests__/SystemSettings.test.tsx:754,771` | Mock settings value `'90'`, asserts field shows 90 | Fixture is a mocked stored value; update to `'30'` |
| `tests/monitoring/uptime-monitoring-scale.spec.ts:190,329-355` | Fixture stored `'90'`, then fills `30` and asserts `30` | Change fixture to a non-default (e.g. `'45'`) so the round-trip still proves a change |
| `ARCHITECTURE.md:465, 504, 790` | "default 90" (x3) | Update to 30 |
| `docs/features/uptime-monitoring.md:113, 124, 422` | "default 90" (x3) | Update to 30 |
| `docs/features/audit-logging.md` | 90 days is *audit-log* retention (`AUDIT_LOG_RETENTION_DAYS`) | Unrelated, do NOT touch |
| `docs/reports/supervisor_review.md:344` | Historic review | Historic record, do not edit |
| `frontend/src/pages/SystemSettings.tsx` | No default hard-coded; field is populated from `settings` (`?? ''`) | Add `placeholder="30"` only |

### 2.2 Retention read path and the clamp hazard

`uptimeConfig.snapshot()` (60 s TTL cache) -> `loadInt("uptime.heartbeat_retention_days", default)` -> `RetentionDays()` -> `UptimePruner.pruneOnce`, which computes `cutoff = now - days*24h` and deletes chunks (5000 rows) where `created_at < cutoff`. The write path is `POST /api/v1/settings` validated by `validateUptimeSetting` (integer, 1-3650).

`loadInt` returns whatever integer is stored, unbounded (verified in `uptime_config.go`; it only falls back on a missing row or a non-integer). A stored `0` or negative value makes `cutoff = now` and the next pass deletes the whole table.

**This is reachable through the API today, not only by bypassing it.** `PATCH /api/v1/config` (`SettingsHandler.PatchConfig`, `settings_handler.go:275`) flattens nested JSON into `key -> value` pairs and writes them in a transaction; it never calls `validateUptimeSetting` (only `UpdateSetting`, the single-row `POST/PATCH /api/v1/settings` handler, does, at `:162`). So `{"uptime":{"heartbeat_retention_days":"0"}}` stores `0` today, and the next pruner pass would delete the entire heartbeat table. This is a real existing bug, fixed in this PR (Section 3.1) in addition to the read-side clamp.

### 2.3 Index usage (what needs which index)

| Query | File | Served by |
| --- | --- | --- |
| Monitor history: `WHERE monitor_id=? [AND created_at<?] ORDER BY created_at DESC LIMIT` | `uptime_service.go:1298-1303` | `idx_heartbeat_monitor_created (monitor_id, created_at)` |
| Delete by monitor: `WHERE monitor_id=?` | `uptime_service.go:1348,1426` | Prefix of `idx_heartbeat_monitor_created` |
| `EXISTS (... monitor_id = uptime_monitors.id)` | `uptime_service.go:1680` | Prefix of `idx_heartbeat_monitor_created` |
| Summary: window `created_at >= ?`, `ROW_NUMBER() PARTITION BY monitor_id ORDER BY created_at DESC` | `uptime_summary_service.go:171-215` | `idx_uptime_heartbeats_created_at`; composite helps the window |
| Pruner: `created_at < ? ORDER BY id LIMIT` | `uptime_pruner.go:~190` | Currently a rowid scan, not the created_at index; fixed in Section 3.6 (`ORDER BY created_at, id` uses `idx_uptime_heartbeats_created_at`) |

- `idx_uptime_heartbeats_monitor_id` (bare GORM `index` tag on `MonitorID`) is a strict prefix of **both** composites, `idx_heartbeat_monitor_created (monitor_id, created_at)` and `idx_heartbeat_lookup (monitor_id, status, created_at)`; no query needs it. Dropping it saves about 139 MB per 3.3 M rows with no query-plan loss.
- **What each composite actually serves.** Delete-by-monitor (`WHERE monitor_id=?`) and the `EXISTS (... monitor_id = uptime_monitors.id)` check only need a `monitor_id` prefix, so **either** composite serves them (`idx_heartbeat_lookup` has `monitor_id` as its first column too). Only the ordered history query (`WHERE monitor_id=? ... ORDER BY created_at DESC LIMIT`) needs `idx_heartbeat_monitor_created`: in `idx_heartbeat_lookup` the `status` column sits between `monitor_id` and `created_at`, so it cannot deliver `created_at` order for a `monitor_id`-only filter.
- `idx_heartbeat_lookup` (256 MB) is a removal candidate, but it is deferred to the follow-up PR (GH #1422) as a separate audited item (its own `perf:`/`fix:` PR). Note: dropping it later makes `idx_heartbeat_monitor_created` the **only** `monitor_id`-prefix index, so it becomes load-bearing for delete-by-monitor and EXISTS as well as history; that coupling is carried into the follow-up (GH #1422).

### 2.4 Index creation surfaces (must stay consistent)

- Model tags: `backend/internal/models/uptime.go:45` (`MonitorID string gorm:"index;index:idx_heartbeat_lookup,priority:1"`) and `:49` (`CreatedAt ... gorm:"index;index:idx_heartbeat_lookup,priority:3"`).
- AutoMigrate lists: `routes.go:191` (`&models.UptimeHeartbeat{}`; the earlier draft said 172, which is wrong), the `migrate` CLI in `cmd/api/main.go` (~L120-172), and the handlers test DB (`testdb.go:53,147`).
- `idx_heartbeat_monitor_created` is created out-of-band (pruner `ensureIndex`, and unconditionally by the `migrate` CLI), deliberately not via tags.

GORM `AutoMigrate` never drops an index removed from a tag, but it WILL re-create `idx_uptime_heartbeats_monitor_id` on every boot if the bare `index` tag stays. Dropping it therefore needs BOTH the tag change (remove the bare `index` at `models/uptime.go:45`, keep `index:idx_heartbeat_lookup,priority:1`) AND an explicit `DROP INDEX IF EXISTS idx_uptime_heartbeats_monitor_id`.

### 2.5 i18n facts (verified)

- `systemSettings.uptime.*` exists **only** in `frontend/src/locales/en/translation.json` (block at ~L1373-1395; `retentionDaysHelper` at L1380). `de`, `es`, `fr`, `zh` have a `systemSettings` block but no `uptime` sub-block; those locales already fall back to English (`frontend/src/i18n.ts:24`, `fallbackLng: 'en'`).
- `frontend/src/__tests__/i18n.test.ts` (the earlier draft's `locales/__tests__` path does not exist) checks only that all five bundles load, a few common keys, and interpolation. **No key-parity enforcement** exists there, and no CI script or test compares locale key sets (grep for `translation.json` consumers found none that do).
- Decision: **English-only helper text**, matching current behaviour. Do not add roughly 20 uptime keys to the four other locales in this PR. Localizing the whole `systemSettings.uptime` block is a separate translation task (file a GH issue at PR time).

### 2.6 Settings API write and echo surface (verified; corrects revision 2)

Three handlers matter, and they behave differently:

| Handler | Route | Reads/echoes | Validates `uptime.*` | Notes |
| --- | --- | --- | --- | --- |
| `GetSettings` (`settings_handler.go:70-90`) | `GET /api/v1/settings` | Unfiltered `h.DB.Find(&settings)`: every row, as a flat map | n/a | Would expose any `migration.*` marker row. |
| `UpdateSetting` (`:127`) | `POST/PATCH /api/v1/settings` | **Echoes only the single row it just wrote** (not a full map) | Yes, `validateUptimeSetting` at `:162` | Accepts a client-supplied `Category` (`:182`), so a `migration.*` key could be written with any category label. |
| `PatchConfig` (`:275`; full-map response at `:383-400`) | `PATCH /api/v1/config` | Unfiltered `h.DB.Find(&settings)` and returns **every row** as a `map[string]string` | **No** (Section 2.2) | Flattens nested JSON via `flattenConfig`, so `{"migration":{"uptime_retention_default_30":"x"}}` becomes the key `migration.uptime_retention_default_30` and reaches the table. |

Revision 2 wrongly attributed the full-map response to `UpdateSettings`. The correct picture: the **marker leak** is via `GetSettings` and the `PatchConfig` response; the **marker write** is possible through **both** `UpdateSetting` and `PatchConfig`; the **missing bounds validation** is in `PatchConfig` only. Other `Setting` readers (`feature_flags_handler.go:66`, `mail_service.go:216`, `cerberus.go:88`, `backup_settings.go:118`) filter by key/category and are unaffected.

## 3. Technical Specifications

### 3.1 Default change, single clamp rule (retention only)

- `services/uptime_config.go`: replace `defaultUptimeRetentionDays = 90` with **exported** constants used by both the service and the handler (DRY, one source of truth):
  - `UptimeRetentionDefaultDays = 30`
  - `UptimeRetentionMinDays = 1`
  - `UptimeRetentionMaxDays = 3650`
- `settings_handler.go`: `uptimeSettingBounds["uptime.heartbeat_retention_days"] = {services.UptimeRetentionMinDays, services.UptimeRetentionMaxDays}`.
- **One clamp rule**, applied to retention only, inside `snapshot()` after `loadInt` (helper `normalizeRetentionDays(n int) int`):
  - unparseable (already handled by `loadInt` -> fallback) or `< UptimeRetentionMinDays` (0, negatives) -> `UptimeRetentionDefaultDays` (30);
  - `> UptimeRetentionMaxDays` -> capped at 3650.
  - There is no "floor of 1" anywhere (the earlier draft's contradictory wording is removed). A corrupt low value falls back to the default rather than deleting everything or silently becoming 1 day.
- `routes.go`: seed value `"30"`. Update the comment in `uptime_config.go` ("match the seeds written in routes.go").
- **Same 1-3650 validation in `PatchConfig`.** Add a shared helper (e.g. `validateUptimeUpdates(updates map[string]string) error`, wrapping `validateUptimeSetting`) and call it in `PatchConfig` after `flattenConfig` and before the transaction, for every flattened key with the `uptime.` prefix. On failure return 400 `{"error": ..., "error_code": "invalid_uptime_setting"}` (same shape as `UpdateSetting`) and write nothing (validate the whole batch before opening the transaction so a bad key cannot leave a partial write). This is an existing bug fix (Section 2.2) delivered together with the read-side clamp, which stays as defence in depth against rows written by other means.
- No API contract change for valid input; `GET/POST /api/v1/settings` shapes are unchanged. `PATCH /api/v1/config` now returns 400 for out-of-range `uptime.*` values that it previously accepted. The interval and worker-pool values are not clamped by this change (their bounds are validated on both paths).

### 3.2 Existing installs: one-time migration of the untouched default

Options considered:

| Option | Outcome |
| --- | --- |
| A. Do nothing; only new installs get 30 | Reporter-type installs keep 90 forever. Rejected. |
| B. Blindly rewrite every stored `90` to `30` | Overrides deliberate choices. Rejected. |
| C. Rewrite `90` -> `30` only when the row is provably the untouched seed, once, guarded by a marker | **Recommended (assumed).** Fixes the common case, preserves deliberate choices when uncertain. |

**Why not the earlier "within 5 s of at least one sibling" rule** (Supervisor finding, confirmed): `models.Setting` has only `UpdatedAt`. A user who moves retention 60 -> 90 in the same save as a worker-pool change gets near-identical `UpdatedAt` on both rows, so the "at least one sibling" rule wrongly lowers a deliberate 90 and permanently deletes history. Conversely, a user who later edits both siblings would look "touched" everywhere. The default assumption is therefore the **stricter** rule below.

**Real-data evidence (user's live day-1 instance, read-only):** the three `uptime.*` rows are IDs **28, 29, 30**, `UpdatedAt` within about **11 ms** of each other, values **60 / 30 / 90** (seeded 2026-08-27). The strict rule below qualifies it (contiguous IDs, within 5 s, siblings at seed values), so the rule works on a real untouched install.

**Contiguity depends on an uninterrupted seed loop.** The three rows get consecutive IDs only if the seed loop in `routes.go` created them in one uninterrupted run on a fresh DB. If the loop was interrupted (crash between rows) and finished on a later boot, or a row was deleted and re-seeded, the IDs are not contiguous and the migration **fails safe: it leaves 90**. That is the intended direction (see "Consequences").

**Strict rule** (`MigrateUptimeRetentionDefault(db *gorm.DB)`, in new `backend/internal/services/uptime_retention_migration.go`, called from `routes.go` right after the seed loop):

1. If marker `migration.uptime_retention_default_30` exists -> return.
2. Read the retention row. If missing, or value != `"90"` -> write marker, return.
3. Load the two sibling seed rows (`uptime.default_interval_seconds`, `uptime.worker_pool_size`). If **either is missing** -> leave the value (indeterminate), write marker, return.
4. The retention row is "untouched seed" only if **all** of these hold:
   - its `UpdatedAt` is within 5 s of the `UpdatedAt` of **each** sibling (ALL present siblings, not "at least one");
   - its row `ID` is adjacent to theirs: the three IDs form a contiguous run (`max(ID) - min(ID) == 2`), matching the seed loop order in `routes.go:794-800` (rows are created consecutively);
   - the siblings still hold their seed values (`"60"` and `"30"`). This third guard is an added cheap check: it means a save that rewrote the siblings to non-default values can never be confused with the untouched seed.
5. If untouched: `UPDATE settings SET value='30' WHERE key='uptime.heartbeat_retention_days' AND value='90'` (single statement, race-safe), log at Info: `uptime heartbeat retention default lowered from 90 to 30 days; adjust under System Settings -> Uptime Monitoring`. Otherwise leave the value at 90 **and log one Info line** so an operator can tell why: `uptime heartbeat retention left at 90 days: value not provably the untouched default (it may have been set deliberately); adjust under System Settings -> Uptime Monitoring`. Logged once (the marker is written in the same pass), and only on the "value is 90 but not provably untouched" path (not for a missing row, a non-90 value, or a present marker).
6. Write the marker in every path (best-effort; failures logged Warn and retried next boot since the function is idempotent).

Consequences (documented, accepted):

- **Fail-safe direction.** Any ambiguity leaves 90. Users who edited the interval or pool at any point (siblings' `UpdatedAt` moved) are *not* migrated; they stay on 90 and rely on the release note and helper text. This is the deliberate cost of the strict rule.
- **The same-save 60 -> 90 case** (retention and pool changed together, interval untouched): the interval sibling still carries the seed time, so the row is not within 5 s of ALL siblings -> left alone. Covered by a table test.
- **Residual bulk-write risk:** if one write updated all three rows together (API client posting all three keys, or a UI save with all three fields dirty) to values that leave the siblings at `60`/`30` and retention at `90`, timestamps and IDs would look like an untouched seed and retention would be lowered. This cannot be distinguished without a schema change (`created_at` / `source` column), which is disproportionate here. Probability is very low; the outcome is that history older than 30 days is removed. Documented in the risk table and release note.
- **Restored backups.** `RehydrateLiveDatabase` copies rows with `INSERT ... SELECT *`, so `UpdatedAt` and IDs are preserved and the heuristic evaluates the same way after a restore. A backup taken *before* this upgrade has no marker row, so the migration simply re-runs on the next boot (it runs at boot only, not at rehydrate) and gives the same answer for the same row; a deliberate 90 saved after the upgrade lives only in newer backups that also contain the marker. Restoring a pre-upgrade backup discards post-upgrade settings by definition. Idempotent; no special handling beyond a test that a missing marker plus a non-untouched row is a no-op.
- After the migration, the first pruner pass deletes the 30-90-day-old rows (about two-thirds of the table). It uses the existing chunked, yielding code path; no new mechanism.

**Marker storage (recommendation):** keep the marker as a settings row (`Key: "migration.uptime_retention_default_30"`, `Type: "string"`, `Category: "migration"`). A `migration.` key must be protected in **both** settings handlers, using **one shared helper set** in `settings_handler.go` (DRY; the follow-up extends the prefix list with `maintenance.`):

- `isInternalSettingKey(key string) bool` - true when the normalized key has any prefix in `internalSettingPrefixes = []string{"migration."}`. **Keyed strictly on the key prefix, never on the client-supplied `Category`** (`UpdateSetting` accepts `Category`, so a category check could be bypassed by sending a different label). Normalize before matching (`strings.TrimSpace`, lower-case) so `" Migration.x"` cannot slip through.
- `filterInternalSettings(settings []models.Setting) []models.Setting` - drops internal rows; used by `GetSettings` **and** by the `PatchConfig` full-map response (`:383-400`). `UpdateSetting` echoes only the row it wrote, so it needs the write rejection, not response filtering.
- Write rejection: `UpdateSetting` returns 400 (`error_code: "reserved_setting_key"`) when `isInternalSettingKey(req.Key)`. `PatchConfig` checks every flattened key (after `flattenConfig`, so `{"migration":{"x":1}}` is caught) and rejects the whole batch with 400 before the transaction.

Alternative considered: a dedicated `data_migrations` table/model. It is cleaner and reusable, but adds a model to three AutoMigrate lists plus the GORM security scan surface for a single flag. Recommended: settings row plus filter; revisit a table if a second data migration appears.

### 3.3 UI retention control (English only)

Current (`SystemSettings.tsx` ~L781-800): number input, `min=1 max=3650 step=1`, label "Heartbeat retention (days)", helper "Older heartbeats are permanently deleted. Applies within ~1 minute, no restart.", error "Enter a whole number between 1 and 3650." Server rejections surface per-field via the existing `clearUptimeServerError`/`uptimeFieldError` plumbing.

Findings: the helper says "Applies within ~1 minute" although the pruner runs hourly (the 60 s TTL only refreshes the cached value); it omits the default, range, disk tradeoff, and the fact that the file may not shrink.

Changes (no new components, no new API):

1. `systemSettings.uptime.retentionDaysHelper` (English, the only string that changes): "Heartbeats older than this are permanently deleted, checked hourly. Default 30 days (range 1-3650). Longer history uses more disk space. Lowering this frees space inside the database, but the file may not shrink until the database is compacted."
2. `SystemSettings.tsx`: add `placeholder="30"` to the retention input. No logic change.
3. Label and error strings unchanged, so existing label-based queries keep passing.
4. Do **not** touch `de/es/fr/zh` (Section 2.5).
5. Verify how a server 400 for `uptime.heartbeat_retention_days` renders (`uptimeFieldError`) and add a unit test that it shows the field error rather than only a toast.

The shipped helper (commit 6) also includes the approximate size ("approximately 15 MB per monitor per 30 days", labelled approximate); the docs carry the per-row derivation: about 320 bytes per row including indexes, 1440 rows per monitor per day.

**0 / "disable pruning" stays out of scope** (would need a sentinel through the pruner, summary, API, and UI, and re-opens the unbounded-growth failure).

### 3.4 Redundant index drop

- `models/uptime.go:45`: `MonitorID string json:"monitor_id" gorm:"index:idx_heartbeat_lookup,priority:1"` (bare `index` removed).
- Existing installs: `DROP INDEX IF EXISTS idx_uptime_heartbeats_monitor_id`, executed only **after** `idx_heartbeat_monitor_created` exists. Place it in the pruner's `ensureIndex` success branch (new small `dropRedundantIndex`, in-memory short-circuit like `indexCreated`) and in the `migrate` CLI after the composite create. Both idempotent. Dropping is fast and frees pages (no file shrink, see the tradeoff above).
- **Rationale (corrected).** The guard is *not* because delete-by-monitor or EXISTS would table-scan: both only need a `monitor_id` prefix, which `idx_heartbeat_lookup` also provides. The guard exists because the **ordered monitor-history query** needs `idx_heartbeat_monitor_created (monitor_id, created_at)`; without it that query would filter by `monitor_id` on `idx_heartbeat_lookup` and sort. Keeping the drop-after-`ensureIndex` guard is harmless and keeps the reasoning simple (never drop the single-column index until the ordered composite exists). The same coupling note is carried into the `idx_heartbeat_lookup` follow-up (Section 2.3, and GH #1422).
- GORM security scan applies because `backend/internal/models/**` changes.

### 3.5 Compaction: NOT in this PR

`charon compact`, `database/compact.go`, `VACUUM`, `auto_vacuum`, `incremental_vacuum`, and the in-app compact button are removed from this plan and moved entirely to the follow-up PR tracked in GH #1422. The pruner comment "VACUUM is deliberately not used" is updated to point at the follow-up work ("space is reclaimed by the database maintenance feature"), nothing more.

### 3.6 Pruner: bounded index scan (unconditional fix in this PR)

Defect (verified on a scratch DB, see Section 6): `SELECT id FROM uptime_heartbeats WHERE created_at < ? ORDER BY id LIMIT 5000` plans as `SCAN uptime_heartbeats` (rowid scan). The final, caught-up chunk of every pass (including the steady-state hourly pass) scans the whole table looking for more matches.

Fix: keep the `DELETE ... WHERE id IN (subquery)` shape (the driver does not compile `DELETE ... LIMIT`), but change the subquery to `ORDER BY created_at, id`:

```
DELETE FROM uptime_heartbeats
WHERE id IN (
    SELECT id FROM uptime_heartbeats
    WHERE created_at < ?
    ORDER BY created_at, id
    LIMIT ?
)
```

- `idx_uptime_heartbeats_created_at (created_at)` stores entries as `(created_at, rowid)`, so `ORDER BY created_at, id` is satisfied by index order with no sort, and the scan stops at the cutoff.
- **Do not use `INDEXED BY`.** It makes the statement error if the index is missing (a fresh DB before AutoMigrate, a restored file, a future index drop), and combined with `ORDER BY id` it forces a `USE TEMP B-TREE FOR ORDER BY` sort per chunk. The planner picks the index on its own for the `created_at, id` ordering.
- Test (in `uptime_pruner_test.go`): run `EXPLAIN QUERY PLAN` on the exact subquery text (export the SQL as a package constant so the test and the code cannot drift) against a populated in-memory DB and assert the plan detail contains `idx_uptime_heartbeats_created_at` and does **not** contain `TEMP B-TREE`. Also assert behaviour: rows older than the cutoff are deleted oldest-first, newer rows untouched, multi-chunk passes terminate.
- Make the inter-chunk pause context-aware (`select` on `ctx.Done()` and `time.After(pause)` instead of `time.Sleep`) so shutdown is not delayed by up to 250 ms per chunk. Unit test: cancel during the pause returns promptly with the partial total and `ctx.Err()`.
- Explicitly **not** included: a 5 M-row benchmark gate (dropped; the fix is unconditional and pinned by the plan test) and any "retry a failed chunk inside the pass" logic (dropped; a failed chunk still returns and the next hourly tick retries).

## 4. Implementation Plan

### Phase 1 - Playwright tests (spec behavior)

Update `tests/monitoring/uptime-monitoring-scale.spec.ts` Scenario 3:

- Change the fixture stored value from `'90'` to a non-default `'45'`; keep the "set to 30 and save" round-trip.
- Add a test: with the mocked settings API returning `'30'`, the field shows `30`, the placeholder is `30`, and the helper text mentions "permanently deleted" and "compacted"; entering `0` and `3651` shows the range error and blocks Save.
- New tests are `test.fixme` in commit 1 and un-fixme'd in commit 6.

### Phase 2 - Backend

- `services/uptime_config.go`: exported constants, default 30, `normalizeRetentionDays` clamp.
- `services/uptime_retention_migration.go` (new): strict migration (Section 3.2) plus marker constant.
- `settings_handler.go`: shared bounds; `isInternalSettingKey` / `filterInternalSettings` used by `GetSettings` **and the `PatchConfig` response**; reject `migration.` writes in **both** `UpdateSetting` and `PatchConfig`; add the 1-3650 uptime validation to `PatchConfig` (Section 3.1).
- `routes.go`: seed `"30"`; call the migration after the seed loop.
- `models/uptime.go`: remove bare `index` from `MonitorID`.
- `services/uptime_pruner.go`: `dropRedundantIndex` after a successful `ensureIndex`; comment update; the bounded-scan query and context-aware pause (Section 3.6, unconditional).
- `cmd/api/main.go`: `migrate` CLI drops the redundant index after creating the composite.

### Phase 3 - Frontend

- `en/translation.json`: `retentionDaysHelper` text only. `SystemSettings.tsx`: `placeholder="30"`.

### Phase 4 - Integration and testing

Go unit tests (new/updated):

- `uptime_config_test.go`: default 30 for missing row and non-integer; clamp table: `0`, `-5` -> 30; `1`, `30`, `3650` unchanged; `3651`, `9999` -> 3650; update existing 90 assertions.
- `uptime_retention_migration_test.go` (table-driven; also asserts the "left at 90, not provably untouched" Info line is logged exactly once on that path and not on others):
  1. untouched seed (90, all three within 5 s, contiguous IDs, sibling values `60`/`30`) -> 30, marker written;
  2. 90 whose `UpdatedAt` is hours after siblings (deliberate save) -> stays 90;
  3. **same-save 60 -> 90 with a worker-pool change** (retention and pool `UpdatedAt` within 5 s of each other, interval sibling at the seed time) -> stays 90 (the case that breaks the old "at least one sibling" rule);
  4. both siblings edited later, retention untouched -> stays 90 (documented miss, fail-safe);
  5. all three rows written together with siblings still at `60`/`30` -> lowered (documents the residual bulk-write risk explicitly, with a comment);
  6. siblings edited to non-default values in the same save -> stays 90;
  7. non-contiguous IDs (a sibling deleted and re-seeded) -> stays 90;
  8. a sibling row missing -> stays 90;
  9. value 45 -> unchanged; fresh install (row 30) -> marker only;
  10. marker present -> no-op even for an untouched-looking 90;
  11. idempotent on second call; "restored pre-upgrade backup" simulation (marker absent, untouched row) -> same result as first run.
- `settings_handler_test.go`, **both handlers**:
  - `GetSettings` and the `PatchConfig` response never contain a `migration.*` row (seed one directly in the DB, assert absence in both bodies).
  - `UpdateSetting`: `migration.x` rejected with 400 regardless of the client-supplied `Category` (including `Category: "general"`, and key case/whitespace variants); nothing written.
  - `PatchConfig`: `{"migration":{"x":"1"}}` rejected with 400 and nothing written (proves flatten-path coverage); a batch mixing a valid key and a `migration.` key writes nothing.
  - `PatchConfig` uptime validation: `{"uptime":{"heartbeat_retention_days":"0"}}`, `"-1"`, `"3651"`, `"abc"` -> 400 `invalid_uptime_setting` and the stored value unchanged; `"30"`, `"1"`, `"3650"` accepted; a batch with one bad `uptime.*` key writes nothing.
  - Bounds reference the shared constants; value `30` accepted via `UpdateSetting`.
- `uptime_pruner_test.go`: `EXPLAIN QUERY PLAN` pin for the chunk subquery (uses `idx_uptime_heartbeats_created_at`, no `TEMP B-TREE`, Section 3.6); context-aware pause test; redundant index dropped only after `ensureIndex` succeeds, not dropped if the composite build fails, idempotent; `EXPLAIN QUERY PLAN` asserts monitor-history, delete-by-monitor, and EXISTS queries use `idx_heartbeat_monitor_created`; scenarios updated for the 30-day default.
- Models: fresh in-memory AutoMigrate does NOT create `idx_uptime_heartbeats_monitor_id` but does create `idx_heartbeat_lookup` and `idx_uptime_heartbeats_created_at`.

Vitest (`SystemSettings.test.tsx`): fixture `'30'`; assert the new helper copy and placeholder; client error for `0` and `3651`; server 400 for the retention key shows the field error. No locale-parity test is added (none exists to extend).

Coverage gates: backend >= 85% (`scripts/go-test-coverage.sh`), frontend >= 85% (`scripts/frontend-test-coverage.sh`); `bash scripts/local-patch-report.sh` must produce `test-results/local-patch-report.{md,json}`.

### Phase 5 - Documentation and deployment

- `docs/features/uptime-monitoring.md`: default 30 (three places); "History Retention" section states the default, range, and that lowering retention frees space inside the database but **the file may not shrink until it is compacted (automatic compaction is planned)**. Add an **"Advanced: manually shrinking the database file (at your own risk)"** section for reporter-type installs who cannot wait for the automatic feature (documented workaround only; nothing shipped):
  1. The Charon image ships the `sqlite3` CLI (`Dockerfile` ~L989, runtime stage package list).
  2. **Charon must be stopped** first; never `VACUUM` a live database from a second process.
  3. Run it in a one-off container against the data volume (entrypoint overridden), not on the host.
  4. **File ownership warning.** Charon runs as the unprivileged `charon` user (the entrypoint drops privileges). A root-run `sqlite3` can leave `charon.db-wal` / `charon.db-shm` owned by root, which the `charon` user then cannot open. Run the one-off container as the same uid/gid, or `chown` the files afterwards.
  5. **Back up first, including `-wal` and `-shm`.** Copying only `charon.db` while a WAL exists loses committed data. Copy all three files while Charon is stopped, or use `sqlite3 charon.db ".backup <dest>"`.
  6. Free space required: roughly **2x the live data** (not 2x the file size; free pages are not rebuilt).
  7. Optional: run `PRAGMA auto_vacuum=INCREMENTAL;` immediately before `VACUUM;` in the same session, so the later automatic-maintenance feature finds an already-converted database.
  8. Start Charon again; if it cannot open the database, check file ownership. Upgrade note: on the first start, if retention was still at the old default and never edited, it is lowered to 30 and older history is removed; anyone who wants longer history can raise it in System Settings.
- `ARCHITECTURE.md`: three "default 90" mentions; note the dropped index under data lifecycle. `docs/features.md`: check only.
- No `docs-site/docs/` edits (git-ignored, synced).
- Release note (commit body): "Default uptime heartbeat retention lowered from 90 to 30 days; installs that never changed the setting are migrated once." Keep the migration commit subject neutral (no wording that hints at data-loss mechanics).

## 5. Commit Slicing Strategy

Decision: one PR (`fix/uptime-heartbeat-retention-default` into `development`), ordered logical commits. Prefixes `fix:`/`test:`/`docs:`; no `(security)` scope. Commit subjects stay factual and avoid wording about deleting or purging data.

| # | Commit | Scope and files | Depends on | Validation gate |
| --- | --- | --- | --- | --- |
| 1 | `test: add e2e specs for retention default and control` | `tests/monitoring/uptime-monitoring-scale.spec.ts` (new tests as `test.fixme`; fixture stays valid) | none | `npx playwright test tests/monitoring/uptime-monitoring-scale.spec.ts --project=firefox` passes (fixme skipped) |
| 2 | `fix: set default heartbeat retention to 30 days` | `uptime_config.go` (exported constants, default, clamp), `routes.go` seed, `settings_handler.go` (shared bounds), updated tests in `uptime_config_test.go`, `uptime_pruner_test.go`, `settings_handler_test.go` | 1 | `cd backend && go build ./... && go test ./internal/services/... ./internal/api/handlers/...` |
| 3 | `fix: apply the new retention default to existing installs` | `uptime_retention_migration.go` (+ table test), `routes.go` call site, `settings_handler.go` `isInternalSettingKey`/`filterInternalSettings` (GetSettings + PatchConfig response) and `migration.` write rejection in `UpdateSetting` and `PatchConfig` (+ tests for both handlers) | 2 | `go test ./internal/services/... ./internal/api/... ` with all Section 4 migration and marker-leak cases green |
| 4 | `fix: remove redundant heartbeat monitor index` | `models/uptime.go` tag, `uptime_pruner.go` drop step, `cmd/api/main.go` migrate CLI, pruner/model tests | 2 | `go test ./...` (services, models, cmd); `./scripts/scan-gorm-security.sh --check` zero CRITICAL/HIGH; `make lint-fast` |
| 5 | `fix: keep the hourly heartbeat maintenance scan bounded` | `uptime_pruner.go` (subquery `ORDER BY created_at, id`, shared SQL constant, context-aware pause), `uptime_pruner_test.go` (EXPLAIN QUERY PLAN pin, ordering/termination, cancel-during-pause). **Unconditional.** | 2 | `go test ./internal/services/...`; plan test asserts `idx_uptime_heartbeats_created_at` and no `TEMP B-TREE` |
| 6 | `fix: clarify heartbeat retention control and enable e2e` | `SystemSettings.tsx` (placeholder), `en/translation.json` helper only, `SystemSettings.test.tsx`, un-fixme E2E | 2 | `cd frontend && npm run type-check && npx vitest run src/pages/__tests__/SystemSettings.test.tsx`; targeted firefox Playwright on the spec above |
| 7 | `docs: document 30-day retention default and disk usage` | `docs/features/uptime-monitoring.md` (incl. advanced manual VACUUM section), `ARCHITECTURE.md` | 2-6 | Manual review; markdown links valid |

Commit 2 also carries the `PatchConfig` uptime validation (same "retention can never be corrupt" theme, shares the bounds constants) with its handler tests. Commit 3's handler work covers **both** `UpdateSetting` and `PatchConfig`. Keep subjects for the migration and validation commits neutral: no words like "purge", "wipe", "delete history", or "data loss".

Full-PR Definition of Done (per `CLAUDE.md`, before merge): targeted Playwright (firefox, the spec above, single project), GORM security scan (models change in commit 4; the settings handler changes in commits 2-3 touch GORM writes too), `bash scripts/local-patch-report.sh`, `lefthook run pre-commit`, **`make lint-backend` (full golangci-lint, manual stage, before PR)**, backend coverage `scripts/go-test-coverage.sh` >= 85%, frontend coverage `scripts/frontend-test-coverage.sh` >= 85%, `npm run type-check`, `go build ./...` and `npm run build`. **CodeQL/Trivy: this PR has no `feat:` commit and no new network surface, so local runs are skipped and CI covers them** (run locally only if a `feat:` commit is added). All commands run foreground/blocking.

## 6. Pruner review: can it keep up at 60 monitors? (concrete finding plus unverified notes)

Status: the concrete finding (query plan) is fixed unconditionally in commit 5 (Section 3.6). The reporter's 19 M rows (2.4x the 90-day steady state) may also come from an upgrade off a pre-v0.39.0 build; **the reporter has no old container logs, so this cannot be confirmed from evidence** and is not requested. Everything below other than the plan finding is code review, not proof.

Code review of `pruneOnce` (`uptime_pruner.go:157-210`) and `Run`/`tick` (`:96-150`):

| Area | Finding |
| --- | --- |
| **Query plan (verified on a scratch DB, sqlite3 CLI) - the concrete finding** | `SELECT id FROM uptime_heartbeats WHERE created_at < ? ORDER BY id LIMIT 5000` plans as **`SCAN uptime_heartbeats`** (rowid scan) with and without `sqlite_stat1`; the `created_at` index is not used. While old rows sit at low ids each full chunk stops after 5000 hits, which is cheap. But the **final, caught-up chunk (fewer than 5000 matches, including the steady-state hourly pass) scans the entire table**: about 8 M rows / roughly 1 GB read per pass at 60 monitors, holding the single connection. Fixed by `ORDER BY created_at, id` (Section 3.6). Not proof of the 19 M cause. |
| `ORDER BY id` and correctness | `ORDER BY id` was **never a correctness dependency**: the `WHERE created_at < ?` predicate alone decides which rows are eligible, and every eligible row is eventually deleted regardless of scan order. The ordering only chose the (bad) access path and made deletion oldest-by-id first. Revision 2's wording "correct only while ids are monotonic" overstated this and is withdrawn. |
| Throughput on the first pass | 5000-row chunks with a 250 ms pause (`firstPassChunkPause`): ceiling about 20 000 rows/s before delete cost; with four secondary indexes each chunk delete is real work; draining 11 M excess rows is on the order of tens of minutes, not a starvation problem by itself. |
| Error handling | A failed chunk returns immediately, `tick` logs Warn and the next attempt is a full hour later. Kept as is (in-pass retry deliberately dropped). |
| Single write connection | Pool capped at 1 (`database.configurePool`); ingester flushes and API writes interleave between chunks. `wal_checkpoint(TRUNCATE)` (only after `total >= 50 000`) is best-effort and can return busy. |
| Sleep | `time.Sleep(pause)` is not context-aware; fixed in Section 3.6. |

**Unverified hypotheses (kept as notes, not acted on):**

- `created_at` text formats or timezone offsets: SQLite compares `created_at < ?` as text when the column holds strings; if some rows were written with a different time format or UTC offset than the bound cutoff parameter, comparisons could misorder relative to real time and leave old rows. Unverified: check how the driver serialises `time.Time` on write versus the cutoff bind, and whether any historical writer used another format. No evidence either way.
- A pre-v0.39.0 upgrade with a large un-pruned backlog (needs no code; would drain over hours).

## 7. Risks, Rollback, and Contingency

| Risk | Impact | Mitigation |
| --- | --- | --- |
| First pruner pass after the migration deletes about two-thirds of the table on a big instance | Temporary write contention; WAL growth | Existing chunked/yielding pruner and WAL truncate; Info log; release note; users who want longer history change the setting (migration is one-shot) |
| Heuristic wrongly lowers a deliberate 90 (bulk write of all three rows with siblings at 60/30, adjacent ids) | History older than 30 days removed against user intent | Strict rule (all siblings, adjacent ids, seed values) plus tests; residual risk documented; open question 1 offers "no automatic migration" |
| Heuristic misses users who edited a sibling | They stay on 90 and keep the large table | Deliberate fail-safe direction; helper text and docs tell them to lower it |
| Users who deliberately want 90+ see no change | None | Preserved by design |
| `PatchConfig` newly rejects out-of-range `uptime.*` values | An API client that relied on writing them gets 400 | Values were unsafe (0 wipes history); same error shape as `UpdateSetting`; release note |
| Reporter-type installs do not see the file shrink | Confusion ("I lowered it and nothing happened") | Helper text and docs say the file may not shrink until compacted; manual `sqlite3 VACUUM` workaround documented; follow-up spec delivers automatic compaction |
| Dropping `idx_uptime_heartbeats_monitor_id` before the composite exists | Slow deletes/lookups until built | Drop only after `ensureIndex` succeeds; guard tested |
| AutoMigrate re-creating the dropped index | Wasted space returns | Tag removal plus a fresh-DB AutoMigrate test |
| Marker leaks through the settings API | Internal key visible/editable | `migration.` prefix filter in `GetSettings` and the `PatchConfig` response plus write rejection in `UpdateSetting` and `PatchConfig` (key-prefix based, not Category), tested for both handlers |
| Rollback | - | Revert the PR. The setting stays at 30 or the user's value; old code accepts 1-3650. AutoMigrate on the reverted code re-creates the dropped index automatically (`charon migrate` does it eagerly). Commits 5 and 6 are independently revertible. |

## 8. Acceptance Criteria

1. A fresh install seeds `uptime.heartbeat_retention_days = 30`; missing/non-integer/`< 1` values fall back to 30; `> 3650` is capped at 3650; a corrupt stored value can never cause a full-table purge; `PATCH /api/v1/config` rejects out-of-range `uptime.*` values with 400 and writes nothing.
2. An existing install whose retention row is the untouched seed 90 (all sibling seed rows within 5 s, contiguous ids, seed sibling values) becomes 30 exactly once; a deliberately saved 90, the same-save 60 -> 90 case, and any ambiguous case are never modified; the migration is idempotent and re-runs harmlessly after a pre-upgrade backup restore.
3. `migration.*` rows never appear in `GET /api/v1/settings` or the `PATCH /api/v1/config` response and cannot be written via `POST/PATCH /api/v1/settings` or `PATCH /api/v1/config` (including nested JSON and any client-supplied Category).
4. The System Settings retention control shows the default (placeholder), range, permanent-deletion warning, and "may not shrink until compacted" in English; invalid values show a readable per-field error; 0/disable is not offered; other locales are unchanged.
5. `idx_uptime_heartbeats_monitor_id` is not created on fresh databases and is dropped from existing ones after the composite index exists; monitor history, deletion, and summary endpoints behave identically.
6. No compaction / VACUUM code is added in this PR.
7. The pruner chunk query plans on `idx_uptime_heartbeats_created_at` with no `TEMP B-TREE` (pinned by test); the inter-chunk pause honours context cancellation.
8. All tests, coverage gates, GORM scan, `make lint-backend`, and type-check pass; docs (including the advanced manual VACUUM section) updated as listed.

## 9. Decisions and Open Questions

Decided by the user (no longer open): strict migration ships; bounded-scan fix rides in this PR unconditionally; manual VACUUM workaround documented (advanced); localization is a separate, already-filed issue (#1421).

Genuinely open: none blocking. Only the GH number of the localization issue needs filling in when known.
