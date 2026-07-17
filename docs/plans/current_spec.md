# Async Backup/Restore Jobs — NS_BINDING_ABORTED Remediation (PR #1136)

**Branch:** `feature/backuprestore` (existing PR #1136 — this plan adds commits to that same PR; it does **not** open a new PR, per CLAUDE.md "one feature = one PR")
**Supersedes for planning purposes:** the previous `docs/plans/current_spec.md` (Restore-Reliability Remediation — C1/H1/H2/M4/M5, dated 2026-07-14 — already implemented; its fixes are visible in code as the `H1 defensive addition` / `C1 fix` comments cited below). This document's `spec §3.x` references continue that same numbering (the original Issue #32 Phase 2 spec) since that's what the codebase's inline comments already cite.
**Trigger:** Bug report — clicking "Create Backup" produces `NS_BINDING_ABORTED` in Firefox DevTools (client-aborted XHR, 0 bytes transferred, no server error).

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specifications](#3-technical-specifications)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)
7. [Ignore-File & Repo Hygiene Review](#7-ignore-file--repo-hygiene-review)
8. [Deferred Findings (Explicitly Out of Scope)](#8-deferred-findings-explicitly-out-of-scope)
9. [Addendum: Backup Download 401 Over Plain-HTTP/Tailscale Access (Same PR)](#9-addendum-backup-download-401-over-plain-httptailscale-access-same-pr)

---

## 1. Introduction

`POST /api/v1/backups` (and, as this plan will show, `POST /api/v1/backups/:filename/restore`) runs its entire archive-creation (or restore) pipeline **synchronously inside the HTTP request/response cycle**, while holding a global mutex. On a host with a large `data/` directory or slow disk, that pipeline legitimately exceeds the frontend's 30-second axios timeout. The browser then aborts the still-in-flight XHR client-side — Firefox reports this as `NS_BINDING_ABORTED`, 0 bytes transferred — while the Go handler keeps working, unaware the client is gone, and its eventual response is discarded.

**Goal:** make backup creation and restore return to the browser almost immediately (job accepted), moving the actual archive/restore work into a tracked background goroutine that the frontend polls for status — eliminating the client-timeout race entirely, for data directories of any size, rather than papering over it with a bigger timeout number.

This is **not** a symptom patch. Raising `client.ts`'s 30s timeout would still fail for a big-enough data directory or slow-enough disk; it would also leave the tab spinning for minutes with no feedback, and does nothing for the equally-synchronous restore path. The fix must remove the requirement that a single HTTP request stay open for the full duration of the operation.

**Revision note (post-review):** production log evidence surfaced a second, independent, source-level bug during review — Charon opens the live SQLite database file through **two different SQLite engine implementations concurrently** (§2.5), which is a plausible self-inflicted cause of the "database disk image is malformed" corruption seen in a real scheduled-backup failure. §2 below now documents **two separate confirmed root causes**, both fixed by this plan: (A) the original unbounded-synchronous-HTTP-request race (§2.1-§2.4, fixed by the async-job architecture) and (B) a dual-SQLite-driver concurrency hazard that can corrupt the database and silently fail backups (§2.5, fixed by driver standardization + a fail-fast integrity pre-check). Neither subsumes the other — both are real, both are fixed here.

---

## 2. Research Findings

### 2.1 Confirmed root cause

The bug report's own working hypothesis is **confirmed**: the client-visible `NS_BINDING_ABORTED` is caused by axios's client-side 30s timeout firing while the Go backend is still synchronously executing a **multi-step, unbounded-duration** pipeline, not by a hang, deadlock, or symlink loop.

| # | Evidence | File:Line |
|---|----------|-----------|
| 1 | Global axios timeout is `30000` ms, applied to every request via the shared instance; no per-call override exists for `POST /backups` | `frontend/src/api/client.ts:10`; `frontend/src/api/backups.ts:56-61` (`createBackup` calls `client.post('/backups', options)` with no `{timeout}` override) |
| 2 | `BackupHandler.Create` calls `h.service.CreateBackupWithOptions(...)` and blocks on its return before writing any JSON | `backend/internal/api/handlers/backup_handler.go:142-154` |
| 3 | `CreateBackupWithOptions` → `createBackupLocked` runs, **in order, on the request goroutine, holding `s.mu`**: (a) `os.Stat` + disk-space check, (b) `writeV2Archive`, which itself calls **`createSQLiteSnapshot`** — a `VACUUM INTO` snapshot of the *live* database via a **second, independent SQLite connection** (§2.5) — before walking `caddy/**` and `crowdsec/**` into the zip via `filepath.Walk`, (c) optional age/scrypt encryption of the whole archive, (d) `sha256File` — a second full sequential read of the finished archive, (e) a GORM insert. **Correction from an earlier pass of this research:** the initial read of `createBackupLocked` cited `writeV2Archive` as going straight into the directory walk; it was incomplete — `writeV2Archive` (backup_service.go:695-749) calls `createSQLiteSnapshot(dbPath)` at line 708, *before* any zip-writing begins, and that function (lines 216-245) is where the `VACUUM INTO` actually executes. | `backend/internal/services/backup_service.go:559-669` (`CreateBackupWithOptions`, `createBackupLocked`), `:695-749` (`writeV2Archive`), `:708` (call site), `:216-245` (`createSQLiteSnapshot`, `VACUUM INTO ?`), `:776-794` (`sha256File`) |
| 4 | None of steps (b)-(d) are bounded by a context, deadline, or size cap — total wall-clock time scales linearly with `data/` size and disk speed | Same as above; no `context.Context` parameter exists anywhere in `createBackupLocked`'s call chain |

### 2.2 Each hypothesis explicitly ruled on

| Hypothesis | Verdict | Evidence |
|---|---|---|
| axios's own 30s timeout elapses and aborts the request | **CONFIRMED — primary cause** | §2.1 above. |
| Component unmounts and an AbortController fires | **RULED OUT** | No `AbortController`/`signal` usage exists anywhere in `frontend/src/api/backups.ts`, `frontend/src/hooks/useBackups.ts`, or `frontend/src/api/client.ts` (grepped, none found). TanStack Query v5's default mutation behavior does **not** cancel in-flight mutations on unmount (only queries are unmount-aborted via their internal `AbortSignal`); mutations run to completion in the background regardless of component lifecycle. The Create dialog also stays mounted for the duration (`Backups.tsx` doesn't unmount `<Dialog>` while `createMutation.isPending`). |
| Tab navigates away mid-request (native form submit) | **RULED OUT** | The Create button (`frontend/src/pages/Backups.tsx:286-293`) is a plain `<Button onClick={handleCreateConfirm}>` inside a `<DialogFooter>`, not inside a `<form>` element anywhere in the component tree (`Dialog`/`DialogContent` render a `<div>`-based modal, confirmed by reading `frontend/src/pages/Backups.tsx:254-296` in full — no `<form>` tag present). No native submit/navigation is possible. |
| A genuine hang/deadlock in `s.mu` from an earlier goroutine | **RULED OUT** | `s.mu` is a plain `sync.Mutex` acquired via **`TryLock()` in both places** (fails fast with `ErrBackupInProgress` → 409, never blocks) — `CreateBackupWithOptions` (backup_service.go:564) **and** `RestoreBackupSafe` (backup_restore_safe.go:222, `if !s.mu.TryLock() { return nil, ErrBackupInProgress }`). **Correction from an earlier pass of this plan:** this row previously (incorrectly) described `RestoreBackupSafe` as using a blocking `Lock()`; re-reading the source confirms it is `TryLock()`, identical to Create — there is no asymmetry between the two functions' locking style. (That mistaken "asymmetric locking" mental model is what let the §3.3.1 double-lock design bug below slip through an earlier draft of this plan — now fixed.) Every acquisition path has a matching, unconditional `Unlock()` (`defer` or explicit) on all returns, including error returns. No path leaves `s.mu` held indefinitely. |
| Unbounded `filepath.Walk` / symlink loop | **RULED OUT as infinite-loop risk, CONFIRMED as unbounded-duration risk** | `addDirToZipTracked` (backup_service.go:854-871) uses `filepath.Walk`, which calls `os.Lstat` (not `Stat`) on each entry — symlinks are visited as symlink entries themselves, never *followed*, so a symlink loop cannot cause infinite recursion here. However, the walk **is** unbounded in *duration*: no entry-count cap, no time budget, and it runs synchronously on the request goroutine. For a large `crowdsec/` or `caddy/` tree on slow storage this alone can consume the full 30s budget. This is the same root cause as #2.1, not a distinct bug. |
| Caddy (or another reverse proxy) sits in front of Charon's own port with a shorter timeout | **RULED OUT** | Per `ARCHITECTURE.md:742-797,987,996`, the browser talks to the Gin/Go **Management Interface directly on port 8080** (`Browser -->|HTTPS :8080| React`). Caddy (ports 80/443) is a *separate* listener that only reverse-proxies the **user's configured hosts** (spec §3.5's `CaddyReloader.ApplyConfig`) — it never sits in front of Charon's own admin API. `docker-entrypoint.sh` starts both processes side by side in one container; there is no internal proxy hop for `/api/v1/*` traffic. Confirmed no `reverse_proxy`/self-proxy wiring exists in `backend/internal/caddy/*.go` for the admin UI. |
| A 409 (`ErrBackupInProgress`) was actually returned and misread as a client abort | **RULED OUT** | 409 responses are small, fast, and always produce a normal completed XHR with a body and a real HTTP status in DevTools — never `NS_BINDING_ABORTED` with `Transferred: 0 B`. The bug report's network trace is consistent only with a client-side abort of a still-pending request. |

### 2.3 Restore has the identical (and structurally worse) defect

`BackupHandler.Restore` (`backend/internal/api/handlers/backup_handler.go:238-262`) calls `h.service.RestoreBackupSafe(...)` synchronously, exactly like `Create`. `RestoreBackupSafe` (`backend/internal/services/backup_restore_safe.go:221-351`) runs the full **V→S→A→R→F** pipeline on the request goroutine, holding `s.mu.Lock()` (blocking, not `TryLock`) for the whole duration:

| Step | What it does | Cost profile |
|---|---|---|
| V1-V6 | `validateBackupArchive`: age-decrypt (if `.age`), parse manifest, verify **every** manifest entry's checksum (reads the *entire* archive), extract the DB entry | O(archive size) — same order as a full backup read |
| S1 | Calls `s.createBackupLocked(BackupOptions{Type: "pre_restore"})` — **a full backup creation**, identical cost to §2.1's Create pipeline | O(data dir size) — as expensive as a manual Create |
| A1 | `unzipWithSkipManifest` — extracts the entire archive (minus DB entries) into `DataDir` | O(archive size) |
| A2 | `RehydrateLiveDatabase` — copies the restored DB to a temp file, `ATTACH DATABASE`, per-table `DELETE`+`INSERT SELECT` inside one transaction, with up to 5 retries on `database is locked` | O(DB size), plus up to `0+150+300+450+600ms` of retry backoff |
| R1 | `caddyReloader.ApplyConfig(ctx)` with its own internal 30s timeout | Bounded, but adds to total |

**Restore's synchronous cost is create's cost (S1) plus extraction (A1) plus a DB rehydrate (A2) — strictly ≥ create's duration.** It is reachable from the UI via `RestoreDialog.tsx`'s "Restore" button, uses the same unguarded axios client, and will produce the identical `NS_BINDING_ABORTED` symptom on the same class of host.

**Decision: this plan fixes Create AND Restore together, in one PR.** Rationale:
- Both already share the same `s.mu` guard, the same `createBackupLocked` internals (S1 literally calls it), and the same handler/HTTP-client pattern — a single async-job abstraction serves both with no duplicated design.
- Shipping a create-only fix would leave a near-identical, already-confirmed-worse bug in Restore, on the very same PR/branch, for the very same feature. That fails CLAUDE.md's "one feature = one PR" spirit in the other direction (a half-fixed feature) and would immediately generate a follow-up bug report.
- No new architectural surface is introduced by including Restore — it reuses the same `BackupJob` model, the same polling endpoint, and the same frontend hook shape as Create.

### 2.4 What is explicitly NOT the fix (symptom-patch rejected)

Increasing `frontend/src/api/client.ts`'s `timeout: 30000` to a larger number (e.g. 300000) would reduce how often this triggers but:
1. Does not fix it for a data directory whose backup legitimately takes longer than any fixed timeout (unbounded by design — `crowdsec/` and `caddy/` directories grow without limit).
2. Leaves the browser tab appearing frozen/unresponsive for however long the operation takes, with the `Button isLoading` spinner as the only feedback and no way to know if it's stuck.
3. Does nothing for Restore's structurally larger cost (§2.3).
4. A raised timeout was explicitly rejected by the task brief as a valid fix for this root cause, and the research confirms the root cause is exactly the case that brief anticipated.

**Decision, per the task brief's own guidance:** implement the architectural fix — `202 Accepted` + background job + polling status endpoint. `client.ts`'s global timeout is **left at 30000ms and is not part of this fix** — the endpoints that now must complete in one round trip (`POST /backups`, `POST /backups/:filename/restore`, `GET /backups/jobs/:job_id`) all become fast, bounded operations (row insert / row read), well within 30s regardless of `data/` size.

### 2.5 New evidence: a second, independent, source-level bug — dual SQLite engines racing the live database

A production log surfaced during review:
```
{"error":"create sqlite snapshot before backup: vacuum into sqlite snapshot: database disk image is malformed","level":"error","msg":"Scheduled backup failed","time":"2026-07-15T03:00:21-04:00"}
```
Tracing the literal error strings (`grep`'d directly) confirms this comes from `createSQLiteSnapshot`'s `db.Exec("VACUUM INTO ?", tmpPath)` call at `backend/internal/services/backup_service.go:235`, wrapped at `:237` and re-wrapped by its caller `writeV2Archive` at `:710` — exactly the call chain corrected in §2.1's table above. This is a **scheduled** backup failure (`RunScheduledBackup` → `s.createBackup` → `CreateBackup()` → `CreateBackupWithOptions` → `createBackupLocked`, `backup_service.go:364-388`), but it is **the same `createBackupLocked`/`createSQLiteSnapshot` code path** the manual "Create Backup" button (§2.1) and Restore's S1 step (§2.3) both go through.

#### (a) Corrected call chain — done, see §2.1's revised row 3/citations above.

#### (b) Why is the database malformed? Traced to true origin, not just handled.

Per CLAUDE.md's Root Cause Analysis Protocol, the question is not "how do we handle a malformed DB" but "why does Charon have one." Tracing every writer/opener of the live `charon.db` file:

| Connection | Driver | File:Line | Concurrency posture |
|---|---|---|---|
| Main application connection (all API traffic, GORM) | `github.com/glebarez/sqlite` (wraps `modernc.org/sqlite`, pure-Go, no CGO) | `backend/internal/database/database.go:54` (`gorm.Open(sqlite.Open(dbPath), ...)`) | `SetMaxOpenConns(1)` (`database.go:144`) — single connection, always open, serving live traffic |
| Startup background integrity scan | `sqlite.DriverName` (same glebarez/modernc driver, via `database/sql`) | `database.go:114` (`sql.Open(sqlite.DriverName, dbPath)`) | Own dedicated connection — **correctly uses the same engine as the main connection** |
| **`createSQLiteSnapshot` (`VACUUM INTO`, runs on every backup)** | **`"sqlite3"` → `github.com/mattn/go-sqlite3` (CGO, links the real C SQLite library)** | `backend/internal/services/backup_service.go:217` (`sql.Open("sqlite3", dbPath)`), blank-imported at `:32` | **Own separate connection, different engine implementation, opened directly against the live, actively-written-to `dbPath` while the main glebarez/modernc connection is simultaneously open** |
| `checkpointSQLiteDatabase` (WAL checkpoint, runs during restore/rehydrate) | Same `"sqlite3"`/mattn | `backup_service.go:201` | Same hazard, on `restoredDBPath`/`tmpPath` (lower live-contention risk, still driver-inconsistent) |
| `sanityCheckSQLiteFile` (restore V6 check) | Same `"sqlite3"`/mattn | `backend/internal/services/backup_restore_safe.go:480` | Operates on a fresh extracted temp file — no concurrent-file risk, but same driver inconsistency |
| `sqliteIntegrityCheck` / `markPendingRestoreOutcome` | Same `"sqlite3"`/mattn | `backend/internal/database/pending_restore.go:84,108` | Runs at startup **before** `database.Connect` opens the main connection (per that file's own doc comment, `pending_restore.go:22`) — no live-concurrency risk today, but still inconsistent, and fragile if that ordering guarantee is ever violated |

**Finding:** `backend/internal/services/backup_service.go` and `backend/internal/database/pending_restore.go` blank-import `github.com/mattn/go-sqlite3` (a CGO binding to the actual C SQLite library) and use it via the literal driver name `"sqlite3"`, while every other part of the codebase — including the main live connection — uses `github.com/glebarez/sqlite` (a pure-Go build of SQLite, via `modernc.org/sqlite`, exposed as `sqlite.DriverName`). **`createSQLiteSnapshot` opens this second, different-engine connection directly against the live, in-use database file while the main connection is open and the server is actively serving traffic** (a scheduled backup runs on a live server by definition; a manual "Create Backup" click happens on a live server too). Concurrent access to the same WAL-mode SQLite file from two independently-built copies of the SQLite engine — one the real C library via CGO, one a separately-built Go port — is not a configuration either library tests or supports; SQLite's own documentation is explicit that all connections to one database should go through "the same build" of the library for WAL-mode correctness guarantees to hold (the VFS/file-locking and WAL-index shared-memory implementations must agree bit-for-bit on locking semantics, and two independently-built engines are not guaranteed to). **This dual-engine concurrent access is the most concrete, source-level, plausible explanation for the "database disk image is malformed" corruption** — it is not established here as *provably* the sole cause (that would require forensic access to the corrupted file this plan doesn't have), but it is a real, unforced, easily-eliminated architectural defect in this codebase, not merely an assumption of pre-existing host/environment disk corruption. It is fixed at the source in §3.8, not papered over with a try/catch.

**Independently discovered, related bug (same fix resolves both):** `.goreleaser.yaml:19` builds Charon's direct binary releases with `CGO_ENABLED=0`. `github.com/mattn/go-sqlite3`'s driver implementation is gated by `//go:build cgo` (confirmed by inspecting the module source) and compiles to nothing under `CGO_ENABLED=0` — verified empirically in this session via `cd backend && CGO_ENABLED=0 go build ./internal/services/... ./internal/database/... ./cmd/api/...`, which succeeds (the mattn package's driver `init()` that calls `sql.Register("sqlite3", ...)` is simply excluded, not a build error). This means **every `sql.Open("sqlite3", ...)` call in a `CGO_ENABLED=0` release binary fails at runtime** with `sql: unknown driver "sqlite3" (forgotten import?)` — i.e., **backup creation, restore, and the pending-restore integrity check are completely non-functional in goreleaser's non-Docker binary releases**, a distinct failure signature (immediate "unknown driver" error, not slow corruption) from what the production log shows. The Docker image, which is the officially-documented, primary self-hosted deployment path (`docker-entrypoint.sh` runs Caddy+Charon together) and almost certainly what the reporting user is running, explicitly overrides this with `CGO_ENABLED=1` (`Dockerfile:268,277`) specifically so `mattn/go-sqlite3` works — which is why the user saw *corruption*, not *unknown driver*. Standardizing on the glebarez/modernc driver everywhere (§3.8) fixes both bugs with one change, and lets `mattn/go-sqlite3` be dropped from `go.mod` entirely — a net simplification, more consistent with CLAUDE.md's "no external dependencies... simple binary" goal (CGO already complicates cross-compilation, which is presumably *why* the main connection deliberately avoids it).

#### (c) Does a malformed DB during `VACUUM INTO` plausibly explain the *original* NS_BINDING_ABORTED?

**Conclusion: most likely two separate bugs, not one — both real, both fixed by this plan, neither explains the other away.** Reasoning:

- `Create`'s error path **does** propagate correctly today — verified by re-reading `BackupHandler.Create` (`backup_handler.go:142-154`): `CreateBackupWithOptions`'s error return is passed straight to `respondCreateError`, which for an unclassified error (which a raw `VACUUM INTO` "malformed" error is, today — no `errors.Is` branch matches it) falls to the `default` case and returns a synchronous `500` with the error text (`backup_handler.go:169`). This is corroborated by the log itself: the **scheduled** path's error was correctly caught and logged by `RunScheduledBackup`'s `if name, err := createBackup(); err != nil { logger.Log().WithError(err).Error(...) }` (`backup_service.go:376-377`) — the error return path itself is not hung or swallowed. Management's hypothesis "the error path might not be propagating correctly" (point 3, second option) is therefore **ruled out**: a manual click that hits this exact corruption would, on today's code, most likely surface as a fast, completed `500` toast — which is a distinguishable network trace from `NS_BINDING_ABORTED` (a completed 500 has a body, a status code, and non-zero transferred bytes; the bug report's trace has none of those).
- SQLite's `VACUUM INTO` does not perform a slow, exhaustive recovery/repair scan on hitting a corrupt page during ordinary B-tree traversal — it returns `SQLITE_CORRUPT` once it detects an invalid page, without the kind of deep multi-pass recovery `PRAGMA` extensions (not used here) would do. So "a hanging recovery attempt" (Management's point 3, first option) is not the SQLite engine's documented behavior; the more defensible version of that hypothesis is narrower: on a **large** table (this app explicitly tracks `RequestLog` growth for dashboard statistics, per `ARCHITECTURE.md`/`models.RequestLog`), a `VACUUM INTO` copies pages roughly in schema/B-tree order, so corruption located late in a large table could still mean a non-trivial amount of successful copying happened before the error — i.e., *some* runs could be slow-then-fail rather than fail-fast, but this is materially different from "hangs indefinitely," and still ends in a **returned error**, not a hang.
- The original bug report's trace (`NS_BINDING_ABORTED`, 0 bytes transferred, no server error) is specifically the signature of a **still-pending** request the client gave up on — not a slow-then-failed one (which would show as a slow-but-completed error response). That is most consistent with §2.1's original finding (legitimately large `data/`/slow disk exceeding 30s while still succeeding-in-progress), not with a corruption error (which, once hit, returns promptly rather than hanging).

**Both bugs are fixed regardless of which one the original reporter hit:** the async-job architecture (§3.2-§3.7, unchanged) eliminates the *client-visible* symptom for any cause of a long-or-failing synchronous request, corruption included — a corrupted DB will now surface as a fast `status:"failed"` poll result instead of either a hang (large-data case) or an opaque 500 (corruption case). But the async-job fix **alone does not make backups succeed again** on a corrupted/dual-engine-damaged database — that requires the source-level fix in §3.8. Both are therefore in scope.

#### (d) Integrity-check gap

Charon already has real (if incomplete) corruption-detection infrastructure, discovered while tracing "wherever the DB is opened":
- `backend/internal/database/database.go:104,113-136` (`runQuickCheck`): runs `PRAGMA quick_check` on a **dedicated** connection at startup, correctly avoiding the shared single-connection pool (its own comment notes the scan "can take well over a minute on larger databases"). Result is **logged only** (`Error("SQLite database integrity check failed - database may be corrupted")`) — never surfaced to the UI, never blocks/warns startup beyond the log line.
- `backend/internal/database/errors.go:23-35` (`IsCorruptionError`) and `:58-72` (`CheckIntegrity`): a ready-made corruption-message classifier (matches "malformed", "corrupt", "disk I/O error", etc.) and a `PRAGMA quick_check` wrapper — already exist, already unit-tested, currently unused by the backup pipeline.
- `backend/internal/api/handlers/db_health_handler.go` (`DBHealthHandler.Check`) + `backend/internal/api/routes/routes.go:238-239` (`router.GET("/api/v1/health/db", ...)`, **registered on the bare, unauthenticated `router`, not the `management` group**): an already-built, already-routed endpoint that calls `database.CheckIntegrity(h.db)` and returns `200`/`{"status":"healthy",...}` or `503`/`{"status":"corrupted",...}`. **Never called by the frontend today** (grepped `frontend/src/**`; zero references to `/health/db` or `integrity_ok`). **Also a latent, separate bug**, discovered in the same trace: `CheckIntegrity` runs `PRAGMA quick_check` on the `*gorm.DB` it's given, and `DBHealthHandler` passes it the **shared, `SetMaxOpenConns(1)` main connection** — unlike `runQuickCheck`, which deliberately avoids exactly this by opening its own dedicated connection. On a large database, an unauthenticated `GET /api/v1/health/db` could block the single connection (and therefore the entire app) for the full multi-minute scan duration referenced in `runQuickCheck`'s own comment.

**Decision: close this gap, in scope for this PR**, because it's exactly what makes the corruption in §2.5(b) go from "silently failing scheduled backups per a log line nobody watches" to "clearly surfaced to the admin" — the same theme as the rest of this plan (don't leave the admin staring at a hang/silent failure with no explanation). Specifically, in scope:
1. A **fast, dedicated-connection** `PRAGMA quick_check` pre-flight inside the backup-creation job, before `createSQLiteSnapshot`/`VACUUM INTO` is attempted, so corruption fails clearly and immediately with a dedicated error code instead of via a raw `VACUUM INTO` error string (§3.8/§3.9).
2. Fix `DBHealthHandler.Check`'s shared-connection risk by giving it the same dedicated-connection pattern `runQuickCheck` already established (§3.9) — a small, low-risk, well-precedented fix directly relevant to "surfacing corruption clearly" and touching code this research already had to read in full.
3. A minimal frontend surface: a `useDbHealth()` hook calling the *existing* `/api/v1/health/db` endpoint, and a corruption warning banner **on the Backups page specifically** (§3.10) — scoped narrowly to "why did my backup fail," not a general system-health dashboard (that's a materially bigger, separate feature, deferred in §8).

Not in scope: automated corruption *repair*/recovery (e.g., SQLite's `.recover` mechanism, forensic salvage tooling) — detection and clear surfacing is the bar this plan meets; repair tooling is a substantially larger effort, deferred in §8 with justification.

---

## 3. Technical Specifications

### 3.1 New persistence: `BackupJob`

New file: `backend/internal/models/backup_job.go`

```go
package models

// BackupJob tracks a single async create-backup or restore-backup operation
// (this plan's §3.1/§3.2.3 — NS_BINDING_ABORTED remediation; the external
// Issue #32 Phase 2 spec this codebase's other backup comments cite tops out
// at §3.10 today, e.g. backup_service.go:151, so this new work is cited
// against this plan's own section numbers, not a fabricated §3.11 of that
// spec). Decoupled from BackupRecord
// (no FK) because a create job has no BackupRecord until it succeeds, and a
// restore job never produces a new BackupRecord at all.
type BackupJob struct {
	ID   uint   `json:"-" gorm:"primaryKey"`
	UUID string `json:"uuid" gorm:"uniqueIndex;size:36"`

	// Type: "create" | "restore".
	Type string `json:"type" gorm:"index;size:20"`
	// Status: "pending" | "running" | "completed" | "failed".
	Status string `json:"status" gorm:"index;size:20"`
	// Stage is a coarse, human-readable progress label updated at each
	// pipeline checkpoint (§3.4). Optional/best-effort — never blocks job
	// completion if an update fails to persist.
	Stage string `json:"stage,omitempty" gorm:"size:40"`

	// Filename: create → the archive filename once known; restore → the
	// source archive filename being restored (known at job start).
	Filename string `json:"filename,omitempty" gorm:"size:255"`
	// ResultUUID: create → the persisted BackupRecord.UUID on success.
	ResultUUID string `json:"result_uuid,omitempty" gorm:"size:36"`
	// ResultJSON: restore → the serialized *services.RestoreResult on
	// success (unmarshaled by the handler when building the poll response).
	// Unused for create (result is fully described by Filename+ResultUUID).
	ResultJSON string `json:"-" gorm:"type:text"`

	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`
	// ErrorCode mirrors the existing error_code values already used by
	// respondCreateError/respondRestoreError (backup_insufficient_space,
	// backup_passphrase_invalid, backup_validation_failed,
	// backup_restore_unrecoverable, ...) so the frontend's existing
	// error-code-driven UI copy keeps working unchanged.
	ErrorCode string `json:"error_code,omitempty" gorm:"size:60"`

	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (BackupJob) TableName() string { return "backup_jobs" }

// BeforeCreate mirrors models.BackupRecord.BeforeCreate (backup_record.go:47) — server-generated UUID.
func (b *BackupJob) BeforeCreate(_ *gorm.DB) (err error) { ... }
```

**Migration:** add `&models.BackupJob{}` to the `db.AutoMigrate(...)` call in `backend/internal/api/routes/routes.go`, immediately after `&models.BackupRemoteCopy{}` (currently line 139). No FK dependency, so ordering relative to `BackupRecord`/`BackupRemoteCopy` doesn't matter functionally, but keeping it adjacent groups the backup-feature models together per the existing file's convention (see the inline `// Issue #32: ...` comments already there).

Per CLAUDE.md §1.5 (GORM Security Scan, conditional/blocking): this change touches `backend/internal/models/**` and adds an `AutoMigrate` entry — `./scripts/scan-gorm-security.sh --check` **must** run and report zero CRITICAL/HIGH findings before this PR merges (added explicitly as a commit gate in §6).

### 3.2 API Design

#### 3.2.1 `POST /api/v1/backups` — CHANGED

Request body unchanged (`createBackupRequest{Encrypt, Passphrase}`, both optional).

| Before | After |
|---|---|
| Blocks until archive is fully written; `201 Created` with `{"filename","uuid","message"}` | Returns as soon as the job row exists; `202 Accepted` with `{"job_id","type":"create","status":"pending"}` |

Synchronous pre-checks that still return immediately (unchanged from today, no job created):
- `!requireAdmin(c)` → `403` (unchanged).
- `opts.Encrypt && strings.TrimSpace(opts.Passphrase) == ""` → `400` (unchanged; checked before `StartCreateBackupJob` acquires the lock — same validation that exists today at `backup_service.go:586`, hoisted one level up so it still short-circuits before any job row is created).
- `s.mu.TryLock()` fails → `409 {"error": "another backup or restore is in progress"}` (`ErrBackupInProgress`, unchanged HTTP mapping, still synchronous — no job row is created for a rejected request).

#### 3.2.2 `POST /api/v1/backups/:filename/restore` — CHANGED

Request body unchanged (`restoreRequest{Passphrase}`, optional).

| Before | After |
|---|---|
| Blocks for the full V→S→A→R→F pipeline; `200 OK` with the `RestoreResult` body | Returns as soon as the job row exists; `202 Accepted` with `{"job_id","type":"restore","status":"pending"}` |

Synchronous pre-checks that still return immediately (no job created):
- `!requireAdmin(c)` → `403` (unchanged).
- `s.mu.TryLock()` fails → `409` (`ErrBackupInProgress`, unchanged).
- Requested filename does not exist in `BackupDir` → `404 {"error": "Backup not found"}` (**new pre-check, added in this revision — `ErrBackupNotFound`, §3.3.1 — preserves today's synchronous 404 exactly; checked *after* the lock succeeds, matching today's `RestoreBackupSafe` precedence where an in-progress operation still wins over a not-found filename**).

**Explicit behavior change (documented, not a regression):** the passphrase-required-for-`.age` check (`ErrPassphraseRequired`, today detected synchronously inside `validateBackupArchive` before any heavy I/O) now happens **inside** the async job, because it lives at the front of the same V1-V6 pipeline that also does the expensive checksum/extraction work, and splitting V1-V2 out into its own synchronous pre-check would duplicate archive-path/open logic for marginal benefit. A wrong-or-missing passphrase now surfaces as `status: "failed"`, `error_code: "backup_passphrase_required"` / `"backup_passphrase_invalid"` on the **first poll**, typically within milliseconds (V1-V2 run before any of the expensive steps), so the UX cost is negligible — RestoreDialog's polling will see the failure on its first or second tick.

#### 3.2.3 `GET /api/v1/backups/jobs/:job_id` — NEW

Admin-gated (`requireAdmin`), registered on the `management` group alongside the existing backup routes.

**Routing precedent (already established in this codebase, not a new risk):** `backend/internal/api/routes/routes.go:347` already registers `POST /backups/remote-targets/test-draft` (a static path segment) as a sibling of `POST /backups/remote-targets/:uuid/test` (a param path) — proven, via `routes_backup_test.go`'s documented regression test, to resolve correctly under Gin's radix-tree router (static segments win over params at the same depth). `GET /backups/jobs/:job_id` follows the identical pattern relative to `GET /backups/:filename/download` etc. — `"jobs"` is a static first-segment sibling of `:filename`, and no real backup filename is ever literally `jobs` (filenames are always `backup_<timestamp>-<suffix>.zip[.age]` or `uploaded_<timestamp>.zip[.age]`, per `backup_service.go:611` / `backup_handler.go:391`). This must still get its own regression test (§3.6) extending the existing `routes_backup_test.go` file, per that file's own header comment instructing exactly this.

Response `200 OK`:

```jsonc
{
  "job_id": "…uuid…",
  "type": "create",            // "create" | "restore"
  "status": "running",         // "pending" | "running" | "completed" | "failed"
  "stage": "archiving_files",  // omitted once status is terminal, or always present — see §3.3.2
  "created_at": "2026-07-16T10:00:00Z",
  "started_at": "2026-07-16T10:00:00Z",
  "finished_at": null,
  "result": null,               // populated only when status == "completed"; shape depends on type (below)
  "error": null                 // populated only when status == "failed"
}
```

`result` shape when `type == "create"` and `status == "completed"` (mirrors today's old `201` body exactly, for frontend continuity):
```json
{ "filename": "backup_2026-07-16_100230-a1b2c3d4.zip", "uuid": "…BackupRecord.UUID…" }
```

`result` shape when `type == "restore"` and `status == "completed"` (identical to today's `RestoreResult`, unchanged struct — `backend/internal/services/backup_restore_safe.go:25-38`):
```json
{
  "message": "Backup restored successfully",
  "restart_required": false,
  "database_swap_pending": false,
  "live_rehydrate_applied": true,
  "caddy_reloaded": true,
  "pre_restore_backup": "backup_2026-07-16_095900-f00dcafe.zip",
  "legacy_format": false
}
```

`error` shape when `status == "failed"`:
```json
{ "message": "insufficient disk space for backup: need at least ... bytes, have ... available", "error_code": "backup_insufficient_space" }
```
`error_code` values reuse the existing set already produced by `respondCreateError`/`respondRestoreError` (`backup_insufficient_space`, `backup_passphrase_required`, `backup_passphrase_invalid`, `backup_validation_failed`, `backup_restore_unrecoverable`, or `""`/omitted for an unclassified error — mirrors today's `default` branches that return a plain message with no `error_code`).

Errors:
- `404 {"error": "backup job not found"}` — unknown `job_id` (`gorm.ErrRecordNotFound`).
- `403` — non-admin (`requireAdmin`, unchanged pattern).

### 3.3 Component Design — Backend

#### 3.3.1 `BackupService` additions (`backend/internal/services/backup_service.go`, `backup_restore_safe.go`)

New fields on `BackupService` (alongside the existing `uploadCtx`/`uploadCancel`/`uploadWG` trio, same pattern):
```go
jobWG sync.WaitGroup // tracks in-flight create/restore job goroutines
```
No `jobCtx`/`jobCancel` — **deliberate**: unlike remote uploads (safe to cancel mid-transfer), a create/restore job interrupted mid-`filepath.Walk`/mid-`unzipWithSkipManifest` could leave a partially-written archive or partially-restored `DataDir`. `Stop()` (§3.3.3) waits for in-flight jobs to finish rather than cancelling them — documented as an intentional risk/assumption in §7 (Risks).

New exported type, so the handler can hand the job-goroutine everything it needs to write a security-audit row without a `gin.Context` (see the "Security audit logging" callout below):
```go
// RequestAuditInfo carries the request-scoped values a Start*Job goroutine
// needs to write a permission-denied SecurityAudit row from inside the job
// (where no gin.Context is available) — captured synchronously by the
// handler, from the exact same gin.Context fields
// handlers.logPermissionAudit already reads today
// (permission_helpers.go:106-117): c.Get("userID"), c.ClientIP(),
// c.Request.UserAgent(). Admin is not carried here — Create/Restore are
// already requireAdmin-gated before a Start*Job call is reached, so it is
// always true in this context (see the "Security audit logging" callout).
type RequestAuditInfo struct {
	Actor     string
	IPAddress string
	UserAgent string
}
```

New exported methods:

| Method | Signature | Behavior |
|---|---|---|
| `StartCreateBackupJob` | `func (s *BackupService) StartCreateBackupJob(opts BackupOptions, audit RequestAuditInfo) (*models.BackupJob, error)` | Runs the existing fast synchronous pre-checks (encrypt-without-passphrase, `s.mu.TryLock()`) exactly as `CreateBackupWithOptions` does today; on success, creates a `BackupJob{Type:"create", Status:"running", StartedAt:now}` row via `s.db.Create(&job)`. **Lock-leak fix (Supervisor-caught, applied in this revision):** if `s.db.Create(&job)` itself fails (disk full, GORM error, etc.) — i.e. `TryLock()` already succeeded but no goroutine has been spawned yet to own `Unlock()` — `StartCreateBackupJob` calls `s.mu.Unlock()` synchronously right there before returning the error, since nothing else will. Only once the job row is durably created does it spawn the tracked goroutine (which runs the archive pipeline via a new progress-aware internal variant, below, and owns `s.mu.Unlock()` from that point on), and return the job **immediately** (before the goroutine's work is done). Returns `ErrBackupInProgress` synchronously, unchanged, if the lock can't be acquired — **no job row is created for a rejected request, and no extra unlock is needed since none was acquired.** Returns an error synchronously if `s.db == nil` (job tracking requires persistence; this mirrors the existing assumption that `db` is always wired via `routes.go` in production — unit tests that construct a `BackupService{}` with no `db` continue to exercise the pre-existing *synchronous* `CreateBackupWithOptions` path unchanged). |
| `StartRestoreJob` | `func (s *BackupService) StartRestoreJob(filename, passphrase string, audit RequestAuditInfo) (*models.BackupJob, error)` | **Corrected in this revision — see the "BLOCKING fix" callout immediately below the two tables; this row now matches it.** Same shape as `StartCreateBackupJob`, with one additional synchronous pre-check preserving today's 404 behavior (see the "Preserving the backup-not-found 404" callout below): (1) `s.mu.TryLock()` — `ErrBackupInProgress` if busy, no job row created, matches today's precedence exactly (a concurrent in-progress operation still wins over a not-found filename, identical to today's `RestoreBackupSafe` ordering); (2) with the lock held, resolve and stat the archive path via `s.GetBackupPath(filename)` (reuses the existing traversal-safe helper, `backup_service.go:887-897`, no new path-validation logic) — on `os.IsNotExist`, `s.mu.Unlock()` synchronously and return `ErrBackupNotFound` (no job row created); on any other stat error, fall through to today's default (unclassified) handling. (3) Create the `BackupJob{Type:"restore", Status:"running", StartedAt:now, Filename:filename}` row via `s.db.Create(&job)` — **same lock-leak fix as Create**: if this fails, `s.mu.Unlock()` synchronously before returning, since no goroutine has been spawned yet. Only then spawns a tracked goroutine that calls the new **lock-free** `restoreBackupSafeLockedWithProgress` core function and `s.mu.Unlock()`s in its own `defer` when that returns, and returns the job immediately. |
| `GetBackupJob` | `func (s *BackupService) GetBackupJob(jobUUID string) (*models.BackupJob, error)` | `s.db.Where("uuid = ?", jobUUID).First(&job)`; returns `gorm.ErrRecordNotFound` untouched for the handler to map to 404. |
| `WaitForJobs` | `func (s *BackupService) WaitForJobs()` | Thin wrapper over `s.jobWG.Wait()`. Exported **specifically for test determinism** (backend unit tests can await job completion without a sleep-poll loop, mirroring how `Stop()` already awaits `uploadWG`) — not required for production call sites. |
| `SetSecurityService` | `func (s *BackupService) SetSecurityService(svc *services.SecurityService)` | New setter mirroring the existing `SetCaddyReloader`/`SetRemoteUploadHook` wiring pattern (`backup_service.go:191-198`) — nilable, wired once from `routes.go` alongside the other `Set*` calls. Gives the job goroutine access to `securityService.LogAudit(...)` for the security-audit fix below; a `nil` `securityService` (e.g. many existing unit tests that construct a bare `BackupService{}`) makes the audit-log call a no-op, matching `logPermissionAudit`'s own existing `if securityService == nil { return }` guard (`permission_helpers.go:93-95`) exactly. |

**REQUIRED FIX (Supervisor-caught): preserving the restore "backup not found" 404.**

Traced: `validateBackupArchive` (`backup_restore_safe.go:102-104`) does `os.Stat(srcPath)` and returns the raw `*fs.PathError` on not-found; today, `respondRestoreError`'s `case os.IsNotExist(err):` branch (`backup_handler.go:274-275`) turns that into a clean, synchronous `404 {"error": "Backup not found"}`. Once that same `os.Stat` call moves inside the async job (unchanged — it's still the first thing `restoreBackupSafeLockedWithProgress` does, via `validateBackupArchive`), a request for a nonexistent/typo'd filename would, without a fix, get a `202`, spin up a phantom `BackupJob` row, and only fail on the *next poll* with an unclassified `error_code:""` and a raw path-error string — an undocumented behavior change for a case that's trivially preservable.

**Decision: preserve the exact synchronous 404, not document a behavior change (the two options Supervisor offered) — this stays fully consistent with the plan's existing "cheap, in-memory synchronous pre-check" pattern already used for `requireAdmin`/`TryLock`/encrypt-without-passphrase**, since a single `os.Stat` is exactly as cheap as those. New sentinel, alongside the block in §3.3.1 below:
```go
// ErrBackupNotFound is returned synchronously by StartRestoreJob (before any
// job row is created) when the requested filename does not exist in
// BackupDir — preserves today's synchronous 404 behavior (backup_handler.go:274-275)
// now that the equivalent os.Stat inside validateBackupArchive runs inside
// the async job for every other purpose.
var ErrBackupNotFound = errors.New("backup not found")
```
`respondStartJobError` (§3.5) gains one new branch: `case errors.Is(err, ErrBackupNotFound): 404 {"error": "Backup not found"}` — byte-for-byte the same response body Gin produces today. No change to §3.2.2's documented behavior-change list — this is explicitly *not* one, unlike the passphrase-required case just below it, precisely because it was fixed rather than left to degrade.

**BLOCKING FIX (Supervisor-caught, applied in this revision): Restore needs the same two-tier lock split Create already has, or every restore job deadlocks against itself.**

`CreateBackupWithOptions`/`createBackupLocked` are already a genuine two-tier split: the outer function does `s.mu.TryLock()`, the inner "Locked" function never touches the lock at all — that split is *why* `StartCreateBackupJob` works (external `TryLock`, spawn a goroutine that calls the lock-free inner function, unlock when it returns).

`RestoreBackupSafe` has **no such split today** — `TryLock()` and the entire V→S→A→R→F pipeline body live in the same function (`backup_restore_safe.go:221-225`: `if !s.mu.TryLock() {...}; defer s.mu.Unlock()`, then the pipeline runs inline in the rest of the same function). An earlier draft of this plan proposed `RestoreBackupSafeWithProgress` as "this exact body plus a progress param" (i.e., still doing its own internal `TryLock()`), with `StartRestoreJob` doing an *external* `TryLock()` before spawning a goroutine that calls it — which would attempt to `TryLock()` the same non-reentrant `sync.Mutex` a second time from within the same already-locked call. `TryLock()` never blocks; it just returns `false` when already held. **Result: every restore job would fail its own first attempt with `ErrBackupInProgress`, 100% of the time, with zero concurrency involved.** Caught by Supervisor review before implementation began; fixed here, not shipped.

**Fix — mirror `createBackupLocked`'s existing shape exactly:**

| Existing (unchanged signature/behavior) | New (additive) |
|---|---|
| `func (s *BackupService) createBackupLocked(opts BackupOptions) (*models.BackupRecord, error)` — still used directly by `RestoreBackupSafe`'s S1 step and by `CreateBackupWithOptions` (still used by `CreateBackup()` → `RunScheduledBackup`'s cron path, which has no HTTP client waiting and is correctly left synchronous) | `func (s *BackupService) createBackupLockedWithProgress(opts BackupOptions, progress func(stage string)) (*models.BackupRecord, error)` — `createBackupLocked` becomes a one-line wrapper calling this with `progress == nil`. Every existing call site (`CreateBackupWithOptions`, `RestoreBackupSafe`'s S1) is untouched. |
| `func (s *BackupService) RestoreBackupSafe(filename, passphrase string) (*RestoreResult, error)` — **today's entire pipeline body, including `TryLock`/`Unlock`, moves out of this function and into the new lock-free core below.** `RestoreBackupSafe` becomes a thin wrapper — `TryLock()` + `defer Unlock()` + call the lock-free core with `progress == nil` — structurally identical to `CreateBackupWithOptions`'s existing shape. This preserves 100% of `RestoreBackupSafe`'s current external signature and behavior (it has exactly one production caller today, `BackupHandler.Restore`, itself being replaced by this same PR — plus existing test callers, all of which keep working unmodified). | `func (s *BackupService) restoreBackupSafeLockedWithProgress(filename, passphrase string, progress func(stage string)) (*RestoreResult, error)` — the current V→S→A→R→F pipeline body (`backup_restore_safe.go:227-350`), **verbatim, with the `TryLock()`/`defer Unlock()` lines at the top removed** (the caller now owns lock lifetime) and `progress(...)` calls threaded in at the checkpoints in §3.3.2. This is the function `StartRestoreJob`'s goroutine calls directly — it never touches `s.mu` itself. |

`progress` is nil-safe everywhere it's invoked (`if progress != nil { progress("stage_name") }`) so passing `nil` is a true no-op, not a special case needing its own tests beyond "still behaves exactly like today" (already covered by 100% of the existing test suite for these functions, unchanged).

**Required test (§6, commit 6):** a regression test proving a restore job actually runs and completes — e.g. `TestStartRestoreJob_CompletesSuccessfully_DoesNotSelfDeadlock` — asserting the job reaches `status:"completed"` (via `WaitForJobs()` + `GetBackupJob`), not an immediate `status:"failed"`/`ErrBackupInProgress`. This is the test that would have caught the bug above; its absence in an earlier draft of this plan is why the bug wasn't caught by the plan's own acceptance criteria the first time around.

New sentinel error (alongside the existing block at `backend/internal/services/backup_service.go:38-67`):
```go
// ErrDatabaseCorrupted is returned by the new pre-flight integrity check
// (§3.9) when a dedicated-connection PRAGMA quick_check fails before a
// VACUUM INTO snapshot is attempted, and also by backupErrorCode's
// classification of a raw VACUUM INTO failure via database.IsCorruptionError
// (§2.5) for defense-in-depth (the pre-check should catch this first, but a
// TOCTOU corruption event between the check and the VACUUM INTO must still
// be classified correctly, not surfaced as an unclassified 500).
var ErrDatabaseCorrupted = errors.New("database integrity check failed; the backup was not created")
```

New unexported error→code mapping helper, used only by the job-finish paths (does **not** replace `respondCreateError`/`respondRestoreError`'s remaining logic, see §3.5 for what happens to those):
```go
// backupErrorCode maps a CreateBackupWithOptions/RestoreBackupSafe sentinel
// error to the stable error_code string the frontend already keys UI copy
// off of (spec §3.10/§3.5). Falls back, in order, to (1)
// util.MapSaveErrorCode(err) — the same permission/readonly/locked
// classifier respondPermissionError already uses (permission_helpers.go:43),
// so a permission-denied failure inside the job still gets its proper
// permissions_db_readonly/permissions_db_locked/permissions_write_denied
// code instead of "" — see the "Security audit logging" fix immediately
// below; then (2) database.IsCorruptionError(err) to classify a raw VACUUM
// INTO error as "backup_database_corrupted" even if it wasn't caught by the
// new pre-flight check (§3.9). Returns "" for a genuinely unclassified error.
func backupErrorCode(err error) string
```

**REQUIRED FIX (Supervisor-caught): security audit logging for permission-denied errors, lost by the async-job conversion unless restored explicitly.**

§3.5 already notes `respondPermissionError` becomes unreachable from `Create`/`Restore` once their bodies move into the job goroutine — but the earlier draft of this plan stopped there without accounting for what that function actually *does*. Re-reading `permission_helpers.go:42-66,92-118`: `respondPermissionError` doesn't just format an HTTP response, it calls `logPermissionAudit`, which writes a `models.SecurityAudit` row — `Actor` from `c.Get("userID")`, `IPAddress` from `c.ClientIP()`, `UserAgent` from `c.Request.UserAgent()` — classified via `util.MapSaveErrorCode` into codes like `permissions_db_readonly`/`permissions_db_locked`/`permissions_write_denied`. These are realistic failure modes for this app's self-hosted, Docker-volume-mounted audience (EACCES on a bind-mounted `data/` dir, a read-only filesystem, a locked DB file from a stray process) — silently losing that audit trail once the work moves into a goroutine (where no `gin.Context` exists) is a real security-log regression, not a cosmetic gap, and it must not be silently absorbed into `error_code:""`.

**Fix:** capture the request-scoped audit fields synchronously, while `c` still exists — via the new `RequestAuditInfo` struct (§3.3.1's methods table above), built by the handler in `Create`/`Restore` (§3.5) from the exact same `c.Get("userID")`/`c.ClientIP()`/`c.Request.UserAgent()` calls `logPermissionAudit` already makes, and passed into `StartCreateBackupJob`/`StartRestoreJob` as a parameter. The job's failure path — right where it calls `backupErrorCode(err)` to populate `BackupJob.ErrorCode` — additionally does:
```go
if code, ok := util.MapSaveErrorCode(err); ok && s.securityService != nil {
    detailsJSON, _ := json.Marshal(map[string]any{"error_code": code, "admin": true, "path": s.BackupDir})
    _ = s.securityService.LogAudit(&models.SecurityAudit{
        Actor: audit.Actor, Action: action, EventCategory: "permissions",
        Details: string(detailsJSON), IPAddress: audit.IPAddress, UserAgent: audit.UserAgent,
    })
}
```
— the same shape `logPermissionAudit` already builds (`permission_helpers.go:97-118`), just sourced from the captured `RequestAuditInfo`/`s.BackupDir` instead of a live `c`. `action` is the same literal string the handler passes into `respondPermissionError` today (`"backup_create_failed"` / `"backup_restore_failed"`, `backup_handler.go:166,284`), now hardcoded per call site inside `StartCreateBackupJob`/`StartRestoreJob` respectively so the `SecurityAudit.Action` field stays identical for continuity with any existing queries/dashboards against it. `admin` is hardcoded `true` (not threaded through `RequestAuditInfo`) because `Create`/`Restore` are already `requireAdmin`-gated before a `Start*Job` call is ever reached — it cannot be `false` in this context, so there is nothing to thread through. `path` reuses `s.BackupDir`, which the service already knows internally — no extra plumbing needed (the handler passed `h.service.BackupDir` for this today, which is the same value).

**Required tests (§6, commits 5 and 6):** a test asserting that a permission-denied failure inside a create/restore job (e.g. inject a read-only `BackupDir` and assert `os.IsPermission`-classified error) produces a `SecurityAudit` row with the expected `Actor`/`IPAddress`/`UserAgent`/`Action`/`error_code`, using a fake/spy `SecurityService` — proving the audit trail survives the move into the goroutine, not just that the job's own `ErrorCode` field is set correctly.

New reconciliation function, mirroring the existing `reconcileStuckUploadingCopies(db *gorm.DB) error` pattern exactly (`backend/internal/services/backup_remote_service.go:178-188`):
```go
// reconcileStuckBackupJobs marks any BackupJob row left "running" or
// "pending" by a previous process (crashed/killed mid-job) as "failed" with
// a fixed ErrorMessage/ErrorCode, so a stale row is never polled forever.
// Called once from NewBackupService, same place reconcileStuckUploadingCopies is called.
func reconcileStuckBackupJobs(db *gorm.DB) error
```
Wired into `NewBackupService` (`backend/internal/services/backup_service.go:302-304`) immediately after the existing `reconcileStuckUploadingCopies` call, same `if reconcileErr != nil { logger.Log().WithError(reconcileErr).Warn(...) }` best-effort pattern (never fatal to construction).

#### 3.3.2 Progress checkpoints (`Stage` values)

Threaded through the two `WithProgress` variants at the checkpoints already present in the existing code (no new logic, just an added callback invocation at each):

| Type | Stage value | Inserted at |
|---|---|---|
| create | `"checking_integrity"` | **NEW** — immediately before `s.writeV2Archive(...)`, a fast dedicated-connection `PRAGMA quick_check` pre-flight (§3.8/§3.9) — `backup_service.go:623` (new code inserted just before the existing call) |
| create | `"archiving_files"` | Immediately before `s.writeV2Archive(...)` itself — `backup_service.go:623` |
| create | `"encrypting"` | Immediately before the `if opts.Encrypt` block's `encryptArchiveWithPassphrase` call — `backup_service.go:632-634` (only reached when `opts.Encrypt`) |
| create | `"computing_checksum"` | Immediately before `sha256File(finalPath)` — `backup_service.go:645` |
| create | `"finalizing"` | Immediately before `s.db.Create(record)` — `backup_service.go:661` |
| restore | `"validating_archive"` | Immediately before `s.validateBackupArchive(...)` — `backup_restore_safe.go:228` |
| restore | `"creating_safety_backup"` | Immediately before the S1 `s.createBackupLocked(...)` call — `backup_restore_safe.go:247` |
| restore | `"applying_files"` | Immediately before `s.unzipWithSkipManifest(...)` (A1) — `backup_restore_safe.go:259` |
| restore | `"rehydrating_database"` | Immediately before the A2 rehydrate retry loop — `backup_restore_safe.go:277` |
| restore | `"reloading_proxy_config"` | Immediately before `s.caddyReloader.ApplyConfig(ctx)` (R1) — `backup_restore_safe.go:327` |

Each `progress(stage)` invocation from the job goroutine does `s.db.Model(&models.BackupJob{}).Where("id = ?", job.ID).Update("stage", stage)` — best-effort (an update failure is logged via `logger.Log().WithError(...).Warn(...)`, never aborts the job).

#### 3.3.3 `Stop()` changes (`backend/internal/services/backup_service.go:329-339`)

Add `s.jobWG.Wait()` alongside the existing `s.uploadWG.Wait()`, so graceful shutdown lets an in-flight backup/restore finish writing before the process exits (per §3.3.1's rationale — no cancellation signal is sent).

### 3.4 Component Design — Frontend

#### 3.4.1 `frontend/src/api/backups.ts` — additive + two signature changes

New types:
```ts
export type BackupJobType = 'create' | 'restore'
export type BackupJobStatus = 'pending' | 'running' | 'completed' | 'failed'

export interface BackupJobError {
  message: string
  error_code?: string
}

export interface BackupJobStartResponse {
  job_id: string
  type: BackupJobType
  status: BackupJobStatus
}

export interface BackupJob<TResult = unknown> {
  job_id: string
  type: BackupJobType
  status: BackupJobStatus
  stage?: string
  created_at: string
  started_at?: string
  finished_at?: string
  result?: TResult
  error?: BackupJobError
}
```

Changed return types (request signatures unchanged):
```ts
export const createBackup = async (options?: CreateBackupOptions): Promise<BackupJobStartResponse> => { … } // was Promise<CreateBackupResponse>; now POST returns 202
export const restoreBackup = async (filename: string, passphrase?: string): Promise<BackupJobStartResponse> => { … } // was Promise<RestoreResult>
```

New function:
```ts
export const getBackupJob = async (jobId: string): Promise<BackupJob<CreateBackupResponse | RestoreResult>> => {
  const response = await client.get<BackupJob<CreateBackupResponse | RestoreResult>>(`/backups/jobs/${jobId}`)
  return response.data
}
```
`CreateBackupResponse` and `RestoreResult` interfaces are unchanged — they now describe `BackupJob.result`'s shape instead of the POST response body's shape.

#### 3.4.2 `frontend/src/hooks/useBackups.ts` — hook shape kept call-site-compatible

Both `useCreateBackup()` and `useRestoreBackup()` are rewritten around a shared internal helper (DRY, both need identical start+poll logic):

```ts
/**
 * Polls GET /backups/jobs/:job_id every POLL_INTERVAL_MS while status is
 * "pending"/"running"; stops (refetchInterval: false) once terminal.
 * Mirrors the established polling pattern in useImport.ts's statusQuery
 * (refetchInterval as a function of the latest query data).
 */
function useBackupJobPolling(jobId: string | null) { … }  // internal, not exported

const BACKUP_JOB_POLL_INTERVAL_MS = 3000 // matches useImport.ts's statusQuery precedent (useImport.ts:30-37) exactly — see ASSUMPTION-002

function useBackupJob<TResult>(
  start: (…) => Promise<BackupJobStartResponse>
): {
  mutate: (arg: …, callbacks?: { onSuccess?: (result: TResult) => void; onError?: (error: Error) => void }) => void
  isPending: boolean
  job: BackupJob<TResult> | undefined
  reset: () => void
}
```

`useCreateBackup()` and `useRestoreBackup()` become thin instantiations of `useBackupJob(...)`. **Public surface preserved:** both still expose `.mutate(options, {onSuccess, onError})` and `.isPending` with the same semantics call sites already rely on — `onSuccess`/`onError` now fire when the **job** reaches `completed`/`failed` (not when the initial POST completes), via an effect keyed on `job?.status`, using a ref to hold the latest callbacks (avoids stale-closure bugs across polls). `isPending` is `true` from the initial `mutate()` call until the job reaches a terminal status, so `Backups.tsx`'s existing `isLoading={createMutation.isPending}` wiring (`Backups.tsx:289`) and `RestoreDialog.tsx`'s existing `isLoading={restoreMutation.isPending}` (`RestoreDialog.tsx:120`) **require no changes** to keep working correctly — the spinner now correctly spans the whole job instead of just the (now-instant) POST.

New, optional addition surfaced by the hook (not required, but recommended — see §4 Phase 3): `job?.stage`, for an optional progress label in the Create/Restore dialogs (e.g. `t('backups.stage.' + job.stage)`), consistent with CLAUDE.md's "novice user" simplicity goal — the user sees *something* is happening ("Archiving files…") instead of a bare spinner for a multi-minute operation.

`onSuccess` invalidation: `queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })` still fires on the create job's completion (was: on POST success) — same effect, later trigger point.

#### 3.4.3 `Backups.tsx` / `RestoreDialog.tsx`

No required structural changes given §3.4.2's compatible hook shape. **Recommended (Phase 3, not blocking):** render `createMutation.job?.stage` / `restoreMutation.job?.stage` as a small caption under the spinner in both dialogs, with i18n keys added under `backups.stage.*` (e.g. `backups.stage.archiving_files`), mirroring the existing `backups.types.*`/`backups.restoreWarnings.*` key-namespacing convention already used in these files.

**Locale file path (corrected — confirmed by inspection, not left as a TODO):** `frontend/src/locales/{de,en,es,fr,zh}/translation.json` — five per-language files, **not** `frontend/src/i18n/locales/*.json`. All five already carry fully-populated `backups.*` trees, including `backups.restoreWarnings.*` (verified: every one of the five files contains a `restoreWarnings` key today) — i.e. the project's established convention for this feature is to keep all five locales in sync per key, not let non-English locales fall back to an English string. **Decision: `backups.stage.*` and the new corruption-banner copy (§3.10) must be added to all five files**, not just `en`, to match that existing convention.

`client.ts`: **no change** (§2.4).

### 3.5 Handler simplification (`backend/internal/api/handlers/backup_handler.go`)

`Create` and `Restore` become thin "start a job, return 202" handlers. Each first builds a `services.RequestAuditInfo` (§3.3.1) synchronously from `c` — the exact same three fields `logPermissionAudit` reads today — and passes it into the `Start*Job` call so the security-audit fix (§3.3.1) has what it needs before `c` goes out of scope:
```go
func (h *BackupHandler) Create(c *gin.Context) {
	if !requireAdmin(c) { return }
	var req createBackupRequest
	_ = c.ShouldBindJSON(&req)
	actor := "unknown"
	if userID, ok := c.Get("userID"); ok { // identical pattern to logPermissionAudit, permission_helpers.go:106-109
		actor = fmt.Sprintf("%v", userID)
	}
	audit := services.RequestAuditInfo{
		Actor:     actor,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	job, err := h.service.StartCreateBackupJob(services.BackupOptions{Type: "manual", Encrypt: req.Encrypt, Passphrase: req.Passphrase}, audit)
	if err != nil { h.respondStartJobError(c, err); return }
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.UUID, "type": job.Type, "status": job.Status})
}
```
`Restore` mirrors this shape, building the same `audit` and calling `h.service.StartRestoreJob(filename, req.Passphrase, audit)`.

`respondCreateError`/`respondRestoreError` (today's large `switch` over sentinel errors, `backup_handler.go:157-171` and `:264-289`) are **replaced** by a single small `respondStartJobError(c, err)` handling only what `StartCreateBackupJob`/`StartRestoreJob` can now return *synchronously*: `ErrBackupInProgress` → 409 (unchanged); `ErrBackupNotFound` → `404 {"error": "Backup not found"}` (restore only — new, §3.3.1's "Preserving the backup-not-found 404" fix, byte-for-byte matches today's response); everything else → 500 with the raw error message (matches today's `default` branch). Per CLAUDE.md's "delete dead code immediately" — the removed branches (`ErrInsufficientSpace`, `ErrPassphraseRequired`, `ErrPassphraseInvalid`, `ErrNewerBackupFormat`/`ErrBackupValidationFailed`, `ErrRestoreUnrecoverable`, `respondPermissionError`) are genuinely unreachable from `Create`/`Restore` after this change (those errors can now only occur *inside* the job goroutine, where they're captured by `backupErrorCode` into the `BackupJob.ErrorCode` column — with permission-denied errors additionally routed to `securityService.LogAudit` per §3.3.1's security-audit fix — not returned synchronously to a handler) — confirmed by tracing: no other caller of the old `respondCreateError`/`respondRestoreError` exists (grepped, both are private methods called only from `Create`/`Restore` respectively).

New handler method:
```go
// GetJob handles GET /api/v1/backups/jobs/:job_id (admin, this plan's §3.2.3).
func (h *BackupHandler) GetJob(c *gin.Context) { … }
```
Builds the `backupJobResponse`/`backupJobError` response types (§3.2.3) from the stored `models.BackupJob`, unmarshaling `ResultJSON` into a `*services.RestoreResult` for `type == "restore"`, or constructing `{"filename","uuid"}` from `Filename`/`ResultUUID` for `type == "create"`. Returns `404` on `gorm.ErrRecordNotFound`.

### 3.6 Route registration (`backend/internal/api/routes/routes.go`)

```go
management.POST("/backups", backupHandler.Create)                    // unchanged line, new response contract
management.GET("/backups/jobs/:job_id", backupHandler.GetJob)        // NEW
management.POST("/backups/:filename/restore", backupHandler.Restore) // unchanged line, new response contract
```
Update the existing comment block at `routes.go:333-336` (currently documents the `:filename` wildcard vs. static-route precedent for `settings`/`remote-targets`/`upload`) to also name `jobs` as one of the static siblings requiring the same regression-test coverage.

### 3.7 Error Handling Summary

| Failure | Detected | Surfaced as |
|---|---|---|
| Not admin | Synchronously, before job starts | `403` (unchanged) |
| Another job already running | Synchronously (`s.mu.TryLock()` fails) | `409 {"error": "another backup or restore is in progress"}` (unchanged) |
| Create: encrypt requested with empty passphrase | Synchronously, before job starts | `400` (unchanged) |
| Restore: filename does not exist in `BackupDir` | Synchronously, after the lock succeeds, before job starts | `404 {"error": "Backup not found"}` (`ErrBackupNotFound` — **new pre-check added in this revision, §3.2.2/§3.3.1, explicitly preserves today's behavior, not a documented change**) |
| `s.db == nil` (job tracking unavailable) | Synchronously | `500` (new — only reachable if `BackupService` is misconfigured without a DB, which `routes.go` never does in production; existing tests that construct a DB-less `BackupService` must keep using the old synchronous `CreateBackupWithOptions`/`RestoreBackupSafe` directly, not the new `Start*Job` wrappers) |
| `s.db.Create(&job)` fails after `s.mu.TryLock()` already succeeded (disk full, GORM error) | Synchronously | `500`; `s.mu` is explicitly `Unlock()`ed before returning (**lock-leak fix added in this revision, §3.3.1** — no goroutine was spawned to own that responsibility, so the caller must) |
| Permission-denied failure inside the job (EACCES, DB read-only, DB locked) | Inside job, classified via `util.MapSaveErrorCode` (same classifier `respondPermissionError` already uses) | `status:"failed"`, `error_code:"permissions_db_readonly"` / `"permissions_db_locked"` / `"permissions_write_denied"` etc.; **also writes a `models.SecurityAudit` row** via the captured `RequestAuditInfo` (**security-audit fix added in this revision, §3.3.1** — this is not a new user-facing error class, it restores logging behavior `respondPermissionError` provides today that the async conversion would otherwise silently drop) |
| Database corrupted (pre-flight `PRAGMA quick_check` fails, or a raw `VACUUM INTO` error classifies as corruption via `database.IsCorruptionError`) | Inside job, fail-fast via §3.9's pre-flight check | `status:"failed"`, `error_code:"backup_database_corrupted"` — **new, §2.5** |
| Insufficient disk space | Inside job | `status:"failed"`, `error_code:"backup_insufficient_space"` |
| Restore: passphrase required/invalid | Inside job (was: synchronous) | `status:"failed"`, `error_code:"backup_passphrase_required"` / `"backup_passphrase_invalid"` — **documented behavior change**, §3.2.2 |
| Restore: newer/invalid backup format | Inside job | `status:"failed"`, `error_code:"backup_validation_failed"` |
| Restore: unrecoverable (C1 double-failure) | Inside job | `status:"failed"`, `error_code:"backup_restore_unrecoverable"` |
| Unclassified I/O error | Inside job | `status:"failed"`, `error_code:""` (omitted), `error.message` = raw error string (matches today's `default` branch text) |
| Unknown `job_id` | On poll | `404` |
| Server crash mid-job | On next `NewBackupService` startup | Row reconciled to `status:"failed"` by `reconcileStuckBackupJobs` before any poll can observe a stale "running" forever |

### 3.8 SQLite Driver Standardization (§2.5 corruption root-cause fix)

Replace every direct `sql.Open("sqlite3", ...)` (mattn/go-sqlite3, CGO) with `sql.Open(sqlite.DriverName, ...)` (`github.com/glebarez/sqlite`, the pure-Go driver already used for the main connection and by `runQuickCheck`), so **exactly one SQLite engine implementation ever touches any Charon-managed database file**, live or otherwise:

| File:Function | Change |
|---|---|
| `backend/internal/services/backup_service.go:201` (`checkpointSQLiteDatabase`) | `sql.Open("sqlite3", dbPath)` → `sql.Open(sqlite.DriverName, dbPath)`; add `"github.com/glebarez/sqlite"` import |
| `backend/internal/services/backup_service.go:217` (`createSQLiteSnapshot`) | Same change — this is the call site that produced the corruption log (§2.5) |
| `backend/internal/services/backup_service.go:32` | Remove `_ "github.com/mattn/go-sqlite3"` blank import (no longer referenced by this file) |
| `backend/internal/services/backup_restore_safe.go:480` (`sanityCheckSQLiteFile`) | Same driver-name change, for consistency (lower risk today since it operates on a fresh temp file, but no reason to leave a second engine wired in) |
| `backend/internal/database/pending_restore.go:84` (`sqliteIntegrityCheck`), `:108` (`markPendingRestoreOutcome`) | Same driver-name change |
| `backend/internal/database/pending_restore.go:9` | Remove `_ "github.com/mattn/go-sqlite3"` blank import |
| `backend/go.mod` | Run `go mod tidy` after the above; `github.com/mattn/go-sqlite3` (currently a **direct** dependency, `go.mod:14`) should drop out entirely — confirm via `go mod why github.com/mattn/go-sqlite3` reporting no remaining importers |

**This single change also fixes the independently-discovered goreleaser bug (§2.5b):** once no code path references the `"sqlite3"` driver name, whether `mattn/go-sqlite3` compiles under `CGO_ENABLED=0` stops mattering — there's no call site left that depends on it. `.goreleaser.yaml`'s `CGO_ENABLED=0` binary release path (previously silently non-functional for all backup/restore operations) starts working correctly with no further change needed there.

**Verification for this commit:** `sql.Open(sqlite.DriverName, ...)` must be confirmed to support `VACUUM INTO` and `PRAGMA quick_check`/`PRAGMA wal_checkpoint` identically to the mattn driver — both are standard SQL-level SQLite features implemented in the (transpiled) SQLite core itself that `modernc.org/sqlite` ships in full, not driver-specific extensions, and `runQuickCheck` (`database.go:114-136`) already proves `PRAGMA quick_check` works correctly against this exact driver in this exact codebase. `VACUUM INTO` needs its own explicit unit-test assertion (§6, commit 2) since no existing test exercises it against the glebarez driver.

### 3.9 Pre-flight Integrity Check

New unexported helper in `backend/internal/services/backup_service.go`, alongside `createSQLiteSnapshot`:
```go
// checkDatabaseIntegrity runs a fast PRAGMA quick_check on dbPath via its
// own dedicated connection (never the caller's shared pool — mirrors
// database.runQuickCheck's rationale, database.go:109-113: a corruption scan
// must not block the app's single-connection pool). Returns ErrDatabaseCorrupted
// wrapping the PRAGMA result if it isn't exactly "ok".
func checkDatabaseIntegrity(dbPath string) error
```
Called from `createBackupLockedWithProgress` immediately after the `progress("checking_integrity")` call and before `writeV2Archive` (§3.3.2's new checkpoint) — a corrupted database now fails the job in well under a second with `error_code:"backup_database_corrupted"`, instead of however long a `VACUUM INTO` against the same corruption takes to fail on its own (§2.5c).

**`DBHealthHandler.Check` fix** (`backend/internal/api/handlers/db_health_handler.go:41-73`, §2.5d): change `database.CheckIntegrity(h.db)` (which runs `PRAGMA quick_check` on the shared, `SetMaxOpenConns(1)` main connection — the same class of risk `runQuickCheck` was written to avoid) to a new `database.CheckIntegrityDedicated(dbPath string) (healthy bool, message string)` that opens its own connection via `sql.Open(sqlite.DriverName, dbPath)`, mirroring `runQuickCheck`. `DBHealthHandler` needs the raw `dbPath` threaded in (currently only holds `*gorm.DB`) — add a `dbPath string` field, set from `cfg.DatabasePath` at construction (`routes.go:238`, `NewDBHealthHandler(db, backupService)` → `NewDBHealthHandler(db, backupService, cfg.DatabasePath)`). This closes a real, independently-discovered latent risk: `GET /api/v1/health/db` is registered on the bare `router` (`routes.go:239`), unauthenticated, and today could block the entire app for the duration of a multi-minute scan on a large database if hit while the connection pool is under contention.

### 3.10 Frontend DB Health Surfacing (minimal, Backups-page-scoped)

New file `frontend/src/api/dbHealth.ts`:
```ts
export interface DBHealthResponse {
  status: 'healthy' | 'corrupted'
  integrity_ok: boolean
  integrity_result: string
  wal_mode: boolean
  journal_mode: string
  last_backup: string | null
  checked_at: string
}
export const getDbHealth = async (): Promise<DBHealthResponse> => {
  const response = await client.get<DBHealthResponse>('/health/db')
  return response.data
}
```
New hook in `frontend/src/hooks/useDbHealth.ts`:
```ts
export function useDbHealth() {
  return useQuery({ queryKey: ['db-health'], queryFn: getDbHealth, staleTime: 60_000 })
}
```
`Backups.tsx`: render an `Alert variant="error"` banner (same `Alert` component already used in `RestoreDialog.tsx:86-102`) at the top of the page when `useDbHealth().data?.status === 'corrupted'`, with copy explaining that backups will fail until the database is repaired/restored from an earlier backup, and a link to `docs/features/backup-restore.md`'s corruption-recovery guidance (added in Phase 5). This is intentionally scoped to the one page where a corruption failure is most confusing (silent scheduled-backup failures, or a newly-failed job with `error_code:"backup_database_corrupted"`) — a system-wide health dashboard is a separate, larger feature, deferred in §8.

---

## 4. Implementation Plan

### Phase 1: Playwright Tests (spec behavior, `test.fixme`)

- Update `tests/tasks/backups-create.spec.ts`: change mocked `POST /api/v1/backups` route to return `202` + `{job_id, type:"create", status:"pending"}`, add a mocked `GET /api/v1/backups/jobs/*` route that returns `"running"` once then `"completed"` with `{filename, uuid}`, mark the affected assertions `test.fixme(...)` until Phase 3/4 lands.
- Update `tests/tasks/backups-restore.spec.ts` identically for the restore flow (`RestoreResult` as `result`).
- Update `tests/integration/backup-restore-e2e.spec.ts` if it asserts the old synchronous response shape (audit during implementation; this is the real-backend integration spec, not a mocked one, so it should mostly "just work" once the real endpoints change — but must assert against `job_id`/poll now instead of an immediate `201`/`200` body).

### Phase 2: Backend Implementation

1. **(New, §2.5/§3.8) Driver standardization**: `backend/internal/services/backup_service.go`, `backend/internal/services/backup_restore_safe.go`, `backend/internal/database/pending_restore.go` — replace all `sql.Open("sqlite3", ...)` with `sql.Open(sqlite.DriverName, ...)`; remove the two `mattn/go-sqlite3` blank imports; `go mod tidy` to drop the dependency.
2. **(New, §3.9)** `backend/internal/services/backup_service.go` — `checkDatabaseIntegrity`, `ErrDatabaseCorrupted`; wire into `createBackupLockedWithProgress` before `writeV2Archive`.
3. **(New, §3.9)** `backend/internal/database/errors.go` — `CheckIntegrityDedicated(dbPath string)`; `backend/internal/api/handlers/db_health_handler.go` — thread `dbPath` through, switch `Check` to the dedicated-connection variant.
4. `backend/internal/models/backup_job.go` (+ `backup_job_test.go`) — new model, `BeforeCreate`, `TableName`.
5. `backend/internal/api/routes/routes.go` — `AutoMigrate` entry; update `NewDBHealthHandler` call site for the new `dbPath` param.
6. `backend/internal/services/backup_service.go` — `jobWG` field, `RequestAuditInfo`, `SetSecurityService`, `createBackupLockedWithProgress`, `StartCreateBackupJob` (incl. the `s.db.Create(&job)` lock-leak fix and permission-error audit logging, §3.3.1), `GetBackupJob`, `WaitForJobs`, `ErrBackupNotFound`, `backupErrorCode` (incl. its `util.MapSaveErrorCode` fallback), `reconcileStuckBackupJobs`, `Stop()` update.
7. `backend/internal/services/backup_restore_safe.go` — extract the lock-free `restoreBackupSafeLockedWithProgress` core out of today's `RestoreBackupSafe` (moving the `TryLock`/`Unlock` lines into a thin `RestoreBackupSafe` wrapper, mirroring `createBackupLocked`'s existing shape exactly — see §3.3.1's "BLOCKING FIX" callout), then add `StartRestoreJob` on top of the new lock-free core, including the `ErrBackupNotFound` pre-check, the lock-leak fix, and permission-error audit logging (co-located in this file, following existing file-splitting convention where restore-specific code lives in `backup_restore_safe.go`).
8. `backend/internal/api/handlers/backup_handler.go` — rewrite `Create`/`Restore` (incl. building `RequestAuditInfo` from `c` before calling `Start*Job`), add `GetJob`, add/replace `respondStartJobError` (incl. the new `ErrBackupNotFound`→404 branch), remove dead `respondCreateError`/`respondRestoreError` branches.
9. `backend/internal/api/routes/routes.go` — register `GET /backups/jobs/:job_id`; update the routing-precedent comment; wire `backupService.SetSecurityService(securityService)` alongside the existing `SetCaddyReloader`/`SetRemoteUploadHook` calls.
10. Run `./scripts/scan-gorm-security.sh --check` (CLAUDE.md §1.5 gate — new model + AutoMigrate).

### Phase 3: Frontend Implementation

1. `frontend/src/api/backups.ts` — new types, changed `createBackup`/`restoreBackup` return types, new `getBackupJob`.
2. `frontend/src/hooks/useBackups.ts` — `useBackupJobPolling`, `useBackupJob`, rewritten `useCreateBackup`/`useRestoreBackup`.
3. (Recommended) `frontend/src/pages/Backups.tsx` / `frontend/src/components/backups/RestoreDialog.tsx` — optional `job?.stage` caption + i18n keys.
4. **(New, §3.10)** `frontend/src/api/dbHealth.ts`, `frontend/src/hooks/useDbHealth.ts`, and a corruption banner in `frontend/src/pages/Backups.tsx`.
5. Locale files — `frontend/src/locales/{de,en,es,fr,zh}/translation.json` (confirmed path; see §3.4.3) — add `backups.stage.*` keys and the corruption-banner copy to **all five** files, matching this feature's existing all-locales-in-sync convention.

### Phase 4: Integration and Testing

- Un-`fixme` the Phase 1 Playwright specs; run `npx playwright test --project=firefox` against the real backend (per CLAUDE.md Definition of Done step 1).
- Full backend/frontend unit suites (§5, §6 gates).

### Phase 5: Documentation and Deployment

- `docs/features/backup-restore.md` — update the "Create Backup" / "Restore" flow description to mention the async job + progress indicator, and add a short "if backups are failing with a corruption error" section pointing at the new banner/health endpoint (novice-user-friendly language, per CLAUDE.md Docs Writer guidance — delegate to `docs-writer` agent, not written by this plan).
- **`ARCHITECTURE.md` — reconsidered in this revision, a short addition IS warranted (not "no change").** `ARCHITECTURE.md:383` already documents, in prose, the existing remote-upload goroutine/`sync.WaitGroup`/crash-reconciliation pattern ("Upload goroutines are tracked via `sync.WaitGroup` and canceled on `BackupService.Stop()`... `BackupRemoteCopy` rows stuck in `uploading` from a prior crash are reconciled to `failed` at the next startup"). This plan adds a second, comparably significant instance of that exact pattern (`jobWG`, `BackupJob` rows, `reconcileStuckBackupJobs`) **and** changes the public API contract of `POST /backups` and `POST /backups/:filename/restore` from synchronous `201`/`200` to `202` + polling (§3.2) — both are the kind of thing `ARCHITECTURE.md`'s existing "Components"/"API Endpoints" prose for this feature is meant to reflect. **Scope: a short addition to the existing `BackupService` bullet at `ARCHITECTURE.md:383` and the adjacent API Endpoints table (both already in the "Components" section documenting this exact feature) — one or two sentences noting the async-job/polling model and pointing at the new `GET /backups/jobs/:job_id` endpoint, not a rewrite of the section.** This is now Phase 5 / commit 11 (§6) work, delegated to `docs-writer` per this plan's existing convention for documentation tasks.

---

## 5. Acceptance Criteria

- [ ] `POST /api/v1/backups` returns `202` within well under 1s regardless of `data/` directory size, for both encrypted and unencrypted requests.
- [ ] `POST /api/v1/backups/:filename/restore` returns `202` within well under 1s.
- [ ] `GET /api/v1/backups/jobs/:job_id` reflects `pending` → `running` (with advancing `stage`) → `completed`/`failed`, matching §3.2.3/§3.7.
- [ ] A second `POST /api/v1/backups` (or `.../restore`) while one is in-flight still gets an immediate `409` with no job row created.
- [ ] Creating/restoring a backup with a multi-minute simulated archive step (test-only slow fixture) never produces a client-side timeout — the browser tab can be closed mid-job and the job completes server-side regardless (this is the core regression test for the original bug).
- [ ] All error paths in §3.7 produce the documented `error_code`.
- [ ] `reconcileStuckBackupJobs` marks any `running`/`pending` row `failed` on process restart.
- [ ] Existing synchronous callers (`CreateBackup()`, `RunScheduledBackup`, `RestoreBackupSafe`'s S1 internal call) are behaviorally unchanged — 100% of pre-existing tests for `CreateBackupWithOptions`/`createBackupLocked`/`RestoreBackupSafe` pass with zero modification to their assertions.
- [ ] `frontend/src/pages/Backups.tsx` and `RestoreDialog.tsx` continue to show a spinner for the full operation duration and a success/error toast on completion, unchanged from the user's perspective except for the (recommended) stage caption.
- [ ] `client.ts`'s global timeout remains `30000` and is untouched by this PR.
- [ ] No code path in the repository opens a Charon-managed SQLite database file via the `"sqlite3"` (mattn/go-sqlite3) driver name; `go.mod` no longer lists `github.com/mattn/go-sqlite3` as a direct dependency (`go mod why github.com/mattn/go-sqlite3` reports no importers, or the module is absent).
- [ ] `cd backend && CGO_ENABLED=0 go build ./...` succeeds and produces a binary whose backup/restore endpoints work end-to-end (proves the goreleaser `CGO_ENABLED=0` release path, §2.5b, is fixed, not just the Docker `CGO_ENABLED=1` path).
- [ ] A `VACUUM INTO`/backup attempt against a database that fails `PRAGMA quick_check` produces `status:"failed"`, `error_code:"backup_database_corrupted"` within well under a second — not a `VACUUM INTO`-derived timeout or an unclassified 500.
- [ ] `GET /api/v1/health/db` no longer runs its integrity scan on the shared, `SetMaxOpenConns(1)` main connection (verified by a test asserting the app's own DB-backed endpoints remain responsive while a `/health/db` scan is in flight against a large fixture, or by code inspection confirming `CheckIntegrityDedicated`'s separate connection).
- [ ] The Backups page shows a corruption warning banner when `GET /api/v1/health/db` reports `status:"corrupted"`.
- [ ] A restore request for a nonexistent/typo'd filename gets a synchronous `404 {"error": "Backup not found"}` — identical to today's behavior, no `BackupJob` row created, no polling required (`ErrBackupNotFound`, §3.2.2/§3.3.1).
- [ ] A permission-denied failure inside a create or restore job (simulated via a read-only `BackupDir` in a test) still writes a `models.SecurityAudit` row with the correct `Actor`/`IPAddress`/`UserAgent`/`Action`/`error_code`, matching what `respondPermissionError`/`logPermissionAudit` produce today (§3.3.1) — the security audit trail is not silently lost by the async-job conversion.
- [ ] If `s.db.Create(&job)` fails immediately after `s.mu.TryLock()` succeeds (simulated in a test), `s.mu` is not left locked — a subsequent `StartCreateBackupJob`/`StartRestoreJob` call succeeds rather than getting a spurious `ErrBackupInProgress` (§3.3.1 lock-leak fix).
- [ ] Full CLAUDE.md Definition of Done (Playwright, GORM scan, patch coverage, CodeQL/Trivy, lefthook, staticcheck, 85% coverage, type-check, builds, cleanup).

---

## 6. Commit Slicing Strategy

**Decision:** single PR (existing PR #1136 on `feature/backuprestore`), ordered logical commits, per CLAUDE.md's mandatory "one feature = one PR" / "slice commits, not PRs" rule. No new PR is opened.

| # | Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|---|
| 1 | `test: add fixme'd E2E specs for async backup/restore job polling and DB corruption handling` | Phase 1 | `tests/tasks/backups-create.spec.ts`, `tests/tasks/backups-restore.spec.ts`, `tests/integration/backup-restore-e2e.spec.ts` | — | Specs parse/compile (`npx playwright test --list`); all new assertions wrapped in `test.fixme` so the suite still passes green |
| 2 | `fix: standardize all SQLite file access on a single driver, drop mattn/go-sqlite3` | Foundation (§2.5/§3.8) | `backend/internal/services/backup_service.go`, `backend/internal/services/backup_restore_safe.go`, `backend/internal/database/pending_restore.go`, `backend/go.mod`/`go.sum`, updated `*_test.go` for each changed function, new `TestCreateSQLiteSnapshot_VacuumIntoViaGlebarezDriver`-style test proving `VACUUM INTO` still works against the standardized driver | 1 | `go build ./...`; `go test ./backend/internal/services/... ./backend/internal/database/...`; `CGO_ENABLED=0 go build ./...` (proves §2.5b's goreleaser breakage is fixed); `go mod why github.com/mattn/go-sqlite3` reports no importer, then `go mod tidy` |
| 3 | `fix: run DB integrity checks on a dedicated connection, not the shared pool` | Foundation (§2.5d/§3.9) | `backend/internal/database/errors.go` (`CheckIntegrityDedicated`), `backend/internal/api/handlers/db_health_handler.go` (thread `dbPath`), `backend/internal/api/routes/routes.go` (`NewDBHealthHandler` call site), tests for both | 2 | `go test ./backend/internal/database/... ./backend/internal/api/handlers/...`; `make lint-fast` |
| 4 | `feat: add BackupJob model and migration` | Foundation | `backend/internal/models/backup_job.go`, `backend/internal/models/backup_job_test.go`, `backend/internal/api/routes/routes.go` (AutoMigrate line only) | 3 | `go build ./...`; `go test ./backend/internal/models/...`; `./scripts/scan-gorm-security.sh --check` (zero CRITICAL/HIGH) |
| 5 | `feat: add async job-tracking to BackupService (create), with a pre-flight integrity check and security-audit logging` | Backend | `backend/internal/services/backup_service.go` — `RequestAuditInfo`, `SetSecurityService`, `StartCreateBackupJob` (incl. the `s.db.Create(&job)` lock-leak fix and the permission-error `securityService.LogAudit` call), `createBackupLockedWithProgress`, `checkDatabaseIntegrity`, `ErrDatabaseCorrupted`, `ErrBackupNotFound`, `backupErrorCode` (incl. its `util.MapSaveErrorCode` fallback), `reconcileStuckBackupJobs`, `WaitForJobs`, `Stop()` (+ new/updated `*_test.go`, including a lock-not-leaked-on-persistence-failure test and a security-audit-row-written-on-permission-error test using a spy `SecurityService`, per §3.3.1) | 4 | `go test ./backend/internal/services/...`; `make lint-fast` |
| 6 | `feat: extract lock-free restore core and add async job-tracking to BackupService (restore)` | Backend | `backend/internal/services/backup_restore_safe.go`: **extract `restoreBackupSafeLockedWithProgress` (the current pipeline body, no lock acquisition) out of `RestoreBackupSafe`, which becomes a thin `TryLock`+`defer Unlock`+call-the-core wrapper (mirrors `createBackupLocked`'s existing shape) — this is the mandatory §3.3.1 "BLOCKING FIX," not optional refactor polish** — then add `StartRestoreJob` (external `TryLock`, the new `ErrBackupNotFound` pre-check, the `s.db.Create(&job)` lock-leak fix, spawn goroutine calling the lock-free core, `Unlock` in the goroutine's own `defer`) on top (+ tests, including `TestStartRestoreJob_CompletesSuccessfully_DoesNotSelfDeadlock`, a not-found-stays-synchronous-404 test, a lock-not-leaked-on-persistence-failure test, and a security-audit-row-written-on-permission-error test, per §3.3.1) | 5 | `go test ./backend/internal/services/...` (must include the self-deadlock regression test actually passing, not just building); `make lint-fast` |
| 7 | `feat: switch backup create/restore handlers to 202+job, add GetJob` | Backend | `backend/internal/api/handlers/backup_handler.go` (`RequestAuditInfo` construction in `Create`/`Restore`, `respondStartJobError` incl. the new `ErrBackupNotFound`→404 branch) (+ `backup_handler_test.go`/new job-handler test file), `backend/internal/api/routes/routes.go` (route registration + comment update, **plus wiring `backupService.SetSecurityService(securityService)` alongside the existing `SetCaddyReloader`/`SetRemoteUploadHook` calls, §3.3.1**), `backend/internal/api/routes/routes_backup_test.go` (new `/backups/jobs/:job_id` routing regression case) | 6 | `go test ./backend/...`; `go build ./...`; `make lint-fast` |
| 8 | `feat: add job polling and DB health hooks to frontend backup API/hooks` | Frontend | `frontend/src/api/backups.ts`, `frontend/src/api/__tests__/backups.test.ts`, `frontend/src/hooks/useBackups.ts`, `frontend/src/hooks/__tests__/useBackups.test.tsx`, `frontend/src/api/dbHealth.ts`, `frontend/src/hooks/useDbHealth.ts` (+ tests) | 7 | `cd frontend && npm run type-check`; `npx vitest run src/api/__tests__/backups.test.ts src/hooks/__tests__/useBackups.test.tsx` |
| 9 | `feat: surface backup job progress and DB corruption warnings in the Backups page` | Frontend | `frontend/src/pages/Backups.tsx`, `frontend/src/components/backups/RestoreDialog.tsx`, `frontend/src/pages/__tests__/Backups.test.tsx`, `frontend/src/components/backups/__tests__/RestoreDialog.test.tsx`, `frontend/src/locales/{de,en,es,fr,zh}/translation.json` (all five, §3.4.3) | 8 | `cd frontend && npm run type-check && npm run build`; relevant Vitest files |
| 10 | `test: un-fixme async backup/restore/corruption E2E specs` | Hardening | Same files as commit 1 (remove `test.fixme`, finalize assertions against the now-real implementation) | 9 | `npx playwright test --project=firefox` (full green run, CLAUDE.md DoD step 1) |
| 11 | `docs: describe async backup/restore progress and corruption handling` | Docs | `docs/features/backup-restore.md`, `ARCHITECTURE.md` (short addition to the existing `BackupService` bullet + API Endpoints table, §4 Phase 5) | 10 | Markdown lint (`.markdownlint.json`), doc review |

**Rollback/contingency for the PR as a whole:** commits #4 onward are additive to the database schema (new table, no column changes to existing tables) and additive to the Go API surface (new methods; the two changed handler methods' *behavior* changes but their route paths/HTTP verbs do not) — reverting any of these, or the whole PR from that point, requires no data migration rollback (the `backup_jobs` table is simply left unused/empty by a revert; existing `BackupRecord`/`BackupRemoteCopy` rows are untouched throughout). Commits #2-#3 are **not purely additive** — they change which SQLite engine opens the live database file and which connection an existing endpoint uses — but are both narrowly-scoped, mechanical substitutions (swap a driver-name string, swap which connection a `PRAGMA` runs on) with no schema or wire-format changes, fully covered by the new `VACUUM INTO`-against-glebarez test (commit #2's gate) before anything downstream depends on them; reverting either is a clean, independent `git revert` of that commit alone. If a CI gate fails on a later commit, the failing commit can be amended-and-re-pushed without unwinding earlier commits, since each commit's validation gate is independently green before the next begins (per CLAUDE.md's per-commit build/test requirement).

---

## 7. Ignore-File & Repo Hygiene Review

| File | Change needed? | Reasoning |
|---|---|---|
| `.gitignore` | No | No new on-disk artifact type is introduced — `BackupJob` rows live in the existing SQLite DB file, already covered by `backend/data/*.db` etc. (`.gitignore:95-98`). No new directories or file extensions are created. |
| `.dockerignore` | No | No new build inputs/scripts added. |
| `codecov.yml` | No | No new top-level package/module needing its own ignore rule; new files land in already-covered `backend/internal/models`, `backend/internal/services`, `backend/internal/api/handlers`, `backend/internal/api/routes`, `frontend/src/api`, `frontend/src/hooks`, `frontend/src/pages`, `frontend/src/components/backups` — all already in scope for the existing 87%/90% targets. |
| `Dockerfile` | No | No new binaries, ports, or entrypoint behavior. `CGO_ENABLED=1` (`Dockerfile:268,277`) can be left as-is — dropping `mattn/go-sqlite3` doesn't require it, but there's no other CGO dependency in this PR that would justify also flipping it to `0` for the Docker image; out of scope. |
| `ARCHITECTURE.md` | **Yes (short addition, reconsidered in this revision)** | See §4 Phase 5 — `ARCHITECTURE.md:383` already documents the analogous existing upload-goroutine/WaitGroup/crash-reconciliation pattern in prose, and this PR both adds a second instance of that pattern and changes two endpoints' public response contract (sync → 202+polling); a one-to-two-sentence addition to the existing `BackupService` bullet and API Endpoints table is warranted, not a rewrite. |
| `backend/go.mod`/`go.sum` | **Yes** | `github.com/mattn/go-sqlite3` (currently a direct dependency, `go.mod:14`) is removed by §3.8's commit; run `go mod tidy` and commit the resulting `go.mod`/`go.sum` diff as part of commit #2 (§6). Not an "ignore file" in the traditional sense, but flagged here per this section's dependency-hygiene intent. |

**Risks & Assumptions:**
- **RISK-001:** `Stop()` now waits unboundedly for `jobWG` (§3.3.3) — a container `docker stop` with a short grace period could still SIGKILL mid-job if the grace period is shorter than the in-flight job's remaining duration. This is a pre-existing risk class (today's synchronous handler has the identical exposure — a SIGKILL mid-request already can corrupt state), not a regression introduced by this plan; not fixed here (would require making `writeV2Archive`/`unzipWithSkipManifest` themselves resumable/atomic-per-chunk, out of scope).
- **RISK-002:** Restore's passphrase-required check moving from synchronous to async (§3.2.2) is a minor UX regression (extra round trip before the error surfaces) accepted in exchange for not duplicating V1-V2 parsing logic into a second synchronous pre-check path.
- **RISK-003 (§2.5):** The dual-SQLite-engine hazard is presented as the most plausible, concrete, source-level explanation for the observed corruption, based on the architecture (two independently-built SQLite implementations opening the same live WAL-mode file concurrently, which neither library's documentation supports) — not as a forensically-proven cause of that specific corrupted file, which this plan has no access to. If §3.8's driver standardization ships and corruption recurs afterward, that would point to a genuine pre-existing environment/hardware/filesystem cause instead, and should be re-escalated rather than assumed fixed.
- **RISK-004 (§3.9):** The new pre-flight `PRAGMA quick_check` adds a small fixed latency (typically sub-second to low-seconds, scaling with DB size, well short of `runQuickCheck`'s "well over a minute" figure which is for a *deep* scan — `quick_check` is deliberately the fast/shallow variant) to every backup job's start. Considered acceptable: it runs inside the now-async job (§3.2), so it never risks the client-timeout race this plan otherwise eliminates.
- **ASSUMPTION-001:** `s.db` is always non-nil for any `BackupService` reachable via HTTP (true today per `routes.go:228`); the `Start*Job` methods' hard requirement on `s.db` is therefore safe in production. Unit tests exercising a DB-less `BackupService` must continue to call the pre-existing synchronous methods directly (`CreateBackupWithOptions`/`RestoreBackupSafe`), which remain fully supported.
- **ASSUMPTION-002:** A 3000ms poll interval (§3.4.2) is a reasonable balance between UI responsiveness and request volume for a single-admin-typical, single-tab usage pattern; not user-configurable. **Corrected in this revision:** an earlier draft set the constant to 2000ms while claiming it matched `useImport.ts`'s precedent — `useImport.ts:30-37`'s `statusQuery.refetchInterval` actually returns `3000`, not `2000`. The constant is now literally `3000` so the "matches precedent" claim is true, not just asserted.
- **ASSUMPTION-003 (§3.8):** `github.com/glebarez/sqlite`'s underlying `modernc.org/sqlite` (already a transitive dependency, `go.mod:113`) supports `VACUUM INTO`, `PRAGMA wal_checkpoint(TRUNCATE)`, and `PRAGMA quick_check`/`integrity_check` with equivalent semantics to `mattn/go-sqlite3` — plausible since `modernc.org/sqlite` is a mechanical transpilation of the actual SQLite C source (not a clean-room reimplementation), so these are core VDBE-level SQL features rather than driver-specific extensions; still called out as an explicit assumption requiring the new `VACUUM INTO`-against-glebarez test (§6 commit #2) to confirm empirically before anything downstream depends on it.

---

## 8. Deferred Findings (Explicitly Out of Scope)

| Finding | Why deferred |
|---|---|
| **Frontend job-tracking state is lost on dialog close / navigation / reload while a job is running** (flagged in Supervisor review, explicitly recorded here rather than left silently unaddressed) | `useBackupJob`'s `jobId` (§3.4.2) is component-local `useState`, not persisted anywhere client-side. The **backend job itself is unaffected** — it keeps running and completing regardless of the browser (that's the entire point of this fix, and is covered by this plan's own acceptance criteria in §5). But if the admin closes the Create/Restore dialog, navigates to another page, or reloads the tab while a job is `pending`/`running`, the frontend has no way to rediscover that job's ID and resume polling it — a fresh `mutate()` call on the same page would hit the still-held `s.mu` and correctly get an immediate `409`/`ErrBackupInProgress` toast (so the user isn't silently misled into thinking nothing is happening or that they can start a second one), but there is no "reconnect to my in-flight job and keep watching its progress" UX. Fixing this properly would mean either a `GET /api/v1/backups/jobs?status=running` list-active-jobs endpoint the page could query on mount, or persisting the last-started `job_id` in `localStorage`/`sessionStorage` — both are reasonable, neither is implemented here, kept out of this PR to avoid scope creep beyond what's needed to fix the reported bug. A future, separate small enhancement. |
| `POST /api/v1/backups/:filename/validate` (dry-run validate) has the same theoretical unbounded-duration profile (V1-V6 reads/checksums the whole archive) | Not reachable from the "Create Backup" button the bug report describes; not a mutating operation; lower real-world risk (typically invoked right after upload/before restore on an archive the user just picked, not routinely on the largest possible archive). Flagged for a future, separate ticket if it proves to reproduce in practice. |
| `POST /api/v1/backups/upload`'s embedded `ValidateBackup` call after an upload | Bounded by the existing 512MB upload cap (`maxBackupUploadSize`, `backup_handler.go:25`), meaningfully smaller worst case than an unbounded `data/` directory; same reasoning as above. |
| `filepath.Walk`'s lack of an entry-count/time budget in `addDirToZipTracked` | The async-job fix removes the *client-facing* symptom regardless of walk duration; a defensive cap on the walk itself is an independent hardening item (already partially covered by `maxExtractedEntryCount`/`maxTotalExtractedSize` on the *extraction* side, per spec §3.9 — no equivalent cap exists on the *creation* side's walk, but that's a resource-usage concern, not a correctness or client-timeout concern once this plan lands). |
| Making `Stop()` actually cancel/checkpoint an in-flight job cleanly (RISK-001) | Requires chunked/resumable archive writes; substantial scope increase disproportionate to this bug fix. |
| Automated SQLite corruption *repair*/recovery (e.g., driving SQLite's `.recover` mechanism or an equivalent forensic salvage pass, auto-restoring from the most recent good backup on detected corruption) | §2.5/§3.9 close the *detection and clear-surfacing* gap (fail fast, dedicated-connection checks, a visible banner) — genuinely fixing a corrupted live database requires either a manual restore-from-backup by the admin (already supported) or dedicated recovery tooling that reads and salvages what it can from a damaged file, which is a substantially larger, higher-risk effort (a bad recovery pass can destroy more data than it saves) warranting its own dedicated plan and explicit user consent flow, not folded into a reliability bug-fix PR. |
| System-wide database/service health dashboard (surfacing `/api/v1/health/db`, uptime, disk space, etc. in one place) | §3.10 intentionally scopes the new corruption banner to the Backups page only, where it's directly actionable ("this is why your backup failed"). A general health dashboard is a materially larger, separate feature (new page, new component library usage, broader design/IA decisions) outside this bug-fix PR's boundary. |
| Gating `GET /api/v1/health/db` behind authentication | §3.9 fixes the *resource-exhaustion* shape of the risk (dedicated connection, bounded cost) but leaves the endpoint itself public/unauthenticated, matching `GET /api/v1/health`'s existing precedent (both are liveness/monitoring-style endpoints conventionally left open for external monitoring tools, e.g. the Docker `HEALTHCHECK` at `Dockerfile:833` hits `/api/v1/health` unauthenticated). Changing that convention for `/health/db` specifically is an auth-model decision independent of this bug fix; not made here. |

---

## 9. Addendum: Backup Download 401 Over Plain-HTTP/Tailscale Access (Same PR)

**Status:** separate, smaller bug found in production logs on `feature/backuprestore`, unrelated to the async-job work above. Same PR #1136 per CLAUDE.md's "one feature = one PR" (this branch is already mid-flight; this is a bug fix riding along, not a new feature warranting its own PR).

**Bug report:** `GET /api/v1/backups/{filename}/download` returns `401 Unauthorized` in the browser even though the user is logged in and other API calls succeed. Instance is accessed over plain HTTP via Tailscale IP (`http://100.98.12.109:8787`), no TLS/reverse-proxy termination.

### 9.1 Root cause (verified against current source)

The bug reporter's trace is **correct in every step**, with one gap this section closes (§9.1.4):

1. **Entry point** — `frontend/src/pages/Backups.tsx:82`: `window.location.href = \`/api/v1/backups/${filename}/download\`` inside `handleDownload`. Plain browser navigation cannot attach a custom `Authorization` header; the line-81 comment ("the browser sends the auth cookie automatically") is the intended fallback design, confirmed correct.

2. **Auth flow** — `backend/internal/api/middleware/auth.go:46-76` (`extractAuthToken`) checks the `Authorization` header first, then falls back to the `auth_token` cookie via `extractAuthCookieToken` (lines 78-97, dedupes multiple `auth_token` cookies by taking the last one). Confirmed: this is the only fallback path, so the download's success depends entirely on the cookie surviving the browser's cookie jar.

3. **Root cause** — `backend/internal/api/handlers/auth_handler.go`, `setSecureCookie` (lines 128-158). `SameSite` is already scheme/network-aware (lines 136-143: `Lax` whenever `scheme != "https"`, and additionally whenever `isLocalRequest(c)`). `Secure`, however, is a **hardcoded literal `true`** at line 155, regardless of scheme. Per RFC 6265bis, browsers refuse to persist a `Secure` cookie set over a plain-HTTP response, so `auth_token` is silently dropped at login time on this deployment, breaking the cookie-fallback path for every navigation-triggered download. **3 call sites** exist in the file: `Login` (line 184), `Refresh` (line 248), and `clearSecureCookie`/`Logout` (line 162, which itself calls `setSecureCookie`) — all inherit the bug since they all funnel through the one hardcoded literal.

4. **Why this code looks the way it does (git archaeology — load-bearing for the fix design):** this exact line has flip-flopped three times:
   - `5bfead5f` — made `Secure` scheme/`isLocalRequest`-aware (the "correct" shape).
   - `c2ee2c17` (**authored by `GitHub Actions`**, commit message `fix(security): harden auth cookie to always set Secure attribute`) — deliberately reverted that, hardcoding `true`, explicitly to eliminate the dataflow path CodeQL rule **`go/cookie-secure-not-set` (CWE-614)** was flagging, and removed the `// codeql[go/cookie-secure-not-set]` suppression comment that had been covering it, reasoning "the root cause is gone, not merely silenced." It also flipped 5 existing test assertions from `assert.False` to `assert.True` to match.
   - This is the version on `main`/this branch today.

   **Implication:** simply restoring the pre-`c2ee2c17` conditional will very likely re-trigger the same CodeQL finding in this repo's CI (`.github/workflows/codeql.yml` + `security-pr.yml` genuinely run and upload SARIF — this is not a dead check). The fix in §9.2 restores the conditional **and** re-adds a suppression comment, using the codebase's existing justified-suppression convention (`// codeql[go/log-injection] Safe: ...`, already used in `backup_handler.go:284` and `crowdsec_handler.go:1121` etc.) — a bare rule-ID suppression with no reasoning is what got this reverted before; a suppression with an inline truth-table justification should not be.

5. **Gap not in the original trace — Tailscale/CGNAT (§9.1 continued):** the reported instance is accessed via `100.98.12.109`, a Tailscale-assigned address. Tailscale's default range is `100.64.0.0/10` — **RFC 6598 "Shared Address Space" (carrier-grade NAT)**, not RFC 1918. `backend/internal/api/handlers/auth_handler.go:80-90` (`isLocalOrPrivateHost`) only checks `ip.IsLoopback()` and `ip.IsPrivate()`; Go's `net.IP.IsPrivate()` (stdlib, since Go 1.17) implements RFC 1918 (`10/8`, `172.16/12`, `192.168/16`) and IPv6 ULA (`fc00::/7`) only — it does **not** cover `100.64.0.0/10`. So `isLocalRequest(c)` returns `false` for the exact IP in the bug report. **A fix that only mirrors the existing `SameSite` gating (i.e., `secure = false` whenever `isLocalRequest(c)` is true) would not actually fix the reported repro** — `100.98.12.109` would still fall through to `isLocalRequest == false` and get `Secure: true`, and the cookie would still be dropped. §9.2 closes this gap explicitly.

   **Risk acceptance (Supervisor review):** extending `isLocalRequest` coverage to `100.64.0.0/10` for `Secure` purposes is a *new* tradeoff, not simply riding on the already-accepted `SameSite` risk posture — `SameSite=Lax` and `Secure` defend against different threats (CSRF vs. passive network eavesdropping/cookie theft), and unlike RFC 1918 space, a CGNAT pool (used by Tailscale, but also by mobile carriers and some ISPs for ordinary internet-facing traffic) can be shared among mutually-untrusting tenants — there is no way to distinguish "this admin's own Tailscale mesh" from "an unrelated CGNAT neighbor" from the IP address alone. Downgrading `Secure` for `100.64.0.0/10` therefore accepts a materially different, address-family-inherent risk (not a code defect) that a RFC 1918/loopback downgrade does not carry. This is judged acceptable here because it is consistent with Charon's self-hosted/LAN/VPN-mesh threat model (CLAUDE.md's "Big Picture") and the affected asset is a session cookie for an already-authenticated admin session, not credentials in transit — but it is a deliberate, explicit risk acceptance, not a mechanical extension of the existing `SameSite` logic, and should be called out as such in the eventual commit message.

### 9.2 The fix

**`backend/internal/api/handlers/auth_handler.go`** — two changes:

**(a) Extend `isLocalOrPrivateHost` to recognize Tailscale/CGNAT:**

```go
// tailscaleCGNAT is Tailscale's default address range (RFC 6598 "Shared
// Address Space" / carrier-grade NAT) — not covered by net.IP.IsPrivate(),
// which only implements RFC 1918 + IPv6 ULA. Self-hosted access over a
// Tailscale mesh without TLS termination is a legitimate, expected Charon
// deployment mode (Big Picture, CLAUDE.md), so it must be treated the same
// as any other private-network origin for the Secure-cookie downgrade below.
//
// Risk acceptance: unlike RFC 1918 space, a CGNAT pool can in principle be
// shared with mutually-untrusting tenants (mobile carriers, some ISPs) — the
// IP alone can't distinguish "this admin's own Tailscale mesh" from "another
// CGNAT tenant." This is an inherent limitation of the address family, not a
// code defect, and is accepted here as consistent with Charon's self-hosted/
// LAN/VPN-mesh threat model (see docs/plans/current_spec.md §9.1.5).
var tailscaleCGNAT = func() *net.IPNet {
	_, block, err := net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		panic(err) // unreachable: constant, valid CIDR
	}
	return block
}()

func isLocalOrPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}

	return tailscaleCGNAT.Contains(ip)
}
```

**(b) Make `Secure` conditional again, mirroring `SameSite`'s existing shape, with a justified CodeQL suppression:**

```go
// setSecureCookie sets an auth cookie with security best practices
//   - HttpOnly: prevents JavaScript access (XSS protection)
//   - Secure: true for HTTPS, or for any request whose origin is not
//     local/private (fail-safe: a plain-HTTP request from a public host still
//     gets Secure=true, so the browser silently drops the cookie rather than
//     transmit it unencrypted — the Authorization-header/localStorage token
//     path in extractAuthToken remains available regardless). Secure is false
//     only when scheme != "https" AND isLocalRequest(c) is true (loopback,
//     RFC 1918, IPv6 ULA, or Tailscale/CGNAT 100.64.0.0/10) — this is what
//     lets the cookie-fallback auth path (used by navigation-triggered
//     downloads, §9 of docs/plans/current_spec.md) work for Charon's
//     documented self-hosted LAN/VPN-mesh deployment mode without TLS
//     termination.
//   - SameSite: Lax for any local/private-network request (regardless of scheme),
//     Strict otherwise (public HTTPS only)
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
	scheme := requestScheme(c)
	secure := true
	sameSite := http.SameSiteStrictMode
	if scheme != "https" {
		sameSite = http.SameSiteLaxMode
	}

	if isLocalRequest(c) {
		sameSite = http.SameSiteLaxMode
		if scheme != "https" {
			secure = false
		}
	}

	// Use the host without port for domain
	domain := ""

	c.SetSameSite(sameSite)
	c.SetCookie( // codeql[go/cookie-secure-not-set] Safe: secure is false only
		// when isLocalRequest(c) AND scheme != "https" (loopback/RFC1918/
		// IPv6-ULA/Tailscale-CGNAT origin over plain HTTP) — every other path
		// (HTTPS, or plain HTTP from a public host) still gets secure=true.
		// See the truth table in docs/plans/current_spec.md §9.2.
		name,   // name
		value,  // value
		maxAge, // maxAge in seconds
		"/",    // path
		domain, // domain (empty = current host)
		secure, // secure
		true,   // httpOnly (no JS access)
	)
}
```

**Truth table** (Backend Dev must implement exactly this — `scheme` is `requestScheme(c)`'s *resolved* value, which already honors `X-Forwarded-Proto` ahead of the raw connection, so "plain HTTP behind an HTTPS-terminating proxy" collapses into the `https` rows below, not a separate case):

| `scheme` | `isLocalRequest(c)` | `Secure` | `SameSite` | Example |
|---|---|---|---|---|
| `https` | `false` | `true` | `Strict` | Public domain behind Caddy/Let's Encrypt |
| `https` | `true` | `true` | `Lax` | `https://192.168.1.5` direct, or LAN reverse-proxy TLS |
| `http` | `false` (public host) | `true` (fail-safe; cookie dropped by browser) | `Lax` | Charon exposed directly on a public IP over plain HTTP (unsupported/discouraged) |
| `http` | `true` (loopback / RFC 1918 / IPv6 ULA / **Tailscale 100.64.0.0/10**) | `false` | `Lax` | **The reported bug: `http://100.98.12.109:8787`** |
| `http` + `X-Forwarded-Proto: https` | any | `true` (resolves to the `https` row — `requestScheme` reads the header first) | per row above | Behind Caddy/nginx TLS termination |

**Regression check (bug report item 5, confirmed no regression):** `requestScheme` (lines 37-50) reads `X-Forwarded-Proto` before anything else, so any reverse-proxy/TLS-termination deployment resolves `scheme == "https"` and gets `Secure: true` unconditionally — unaffected by this change. Verified by existing tests `TestSetSecureCookie_ForwardedHTTPS_LocalhostForcesInsecure` / `_ForwardedHTTPS_LoopbackForcesInsecure` (both already assert `Secure: true`, both continue to pass unmodified).

### 9.3 Blast radius (bug report items 6-7, confirmed/corrected)

**Frontend `window.location.href` navigation sites** (`grep -rn "window.location.href\s*=" frontend/src`, excluding tests):

| File:line | Target | Depends on cookie-fallback auth? |
|---|---|---|
| `frontend/src/pages/Backups.tsx:82` | `/api/v1/backups/${filename}/download` (own API, authenticated) | **Yes — this is the bug.** |
| `frontend/src/components/backups/RemoteTargetsCard.tsx:136` | `result.authorize_url` (external OAuth provider, e.g. Dropbox/Google) | No — external URL, not a Charon API call. |
| `frontend/src/components/backups/RemoteTargetFormDialog.tsx:209` | `result.authorize_url` (same OAuth flow) | No — same as above. |

**Correction to bug report item 6:** the reporter flagged `logs/:filename/download` (`routes.go:369`) as likely-affected alongside the backup download. It is **not**. `frontend/src/api/logs.ts:93-101` (`downloadLog`, used by `useLogs.ts`'s `useDownloadLog`) goes through the shared `client.get(..., { responseType: 'blob' })` — the axios client, which attaches the `Authorization` header from JS-held storage via its interceptor (the same path every other successful API call uses) — then saves the blob via `URL.createObjectURL` + a synthetic `<a>` click. It never navigates the browser to the endpoint and never depends on the cookie fallback. `Backups.tsx:82` is the **only** affected call site in `frontend/src`.

**Backend `setSecureCookie` call sites** (all 3, confirmed via grep, all fixed by the one function change): `Login` (`auth_handler.go:184`), `Refresh` (`auth_handler.go:248`), `Logout` → `clearSecureCookie` (`auth_handler.go:162, 221`).

### 9.4 Existing test coverage (bug report item 7, confirmed)

**Backend unit tests** — `backend/internal/api/handlers/auth_handler_test.go` already has 13 `TestSetSecureCookie_*` functions (lines 65-286) plus `TestClearSecureCookie` (403-415), added across the `5bfead5f`/`c2ee2c17` history in §9.1.4. Because the fix in §9.2 restores exactly the pre-`c2ee2c17` conditional (plus the new Tailscale/CGNAT branch), most of these already assert the **correct** post-fix behavior and need **no change** — only 5 currently assert the bug itself and must flip:

| Test | Host used | Current assertion | Correct post-fix assertion | Action |
|---|---|---|---|---|
| `TestSetSecureCookie_HTTP_Loopback_Insecure` | `127.0.0.1` | `Secure: true` | `Secure: false` | **Flip to `assert.False`** |
| `TestSetSecureCookie_HTTP_PrivateIP_Insecure` | `192.168.1.50` | `Secure: true` | `Secure: false` | **Flip to `assert.False`** |
| `TestSetSecureCookie_HTTP_10Network_Insecure` | `10.0.0.5` | `Secure: true` | `Secure: false` | **Flip to `assert.False`** |
| `TestSetSecureCookie_HTTP_172Network_Insecure` | `172.16.0.1` | `Secure: true` | `Secure: false` | **Flip to `assert.False`** |
| `TestSetSecureCookie_HTTP_IPv6ULA_Insecure` | `fd12::1` | `Secure: true` | `Secure: false` | **Flip to `assert.False`** |
| `TestSetSecureCookie_HTTPS_Strict`, `_HTTP_Lax` (host `192.0.2.10`, a public TEST-NET-1 address, not private), `_ForwardedHTTPS_LocalhostForcesInsecure`, `_ForwardedHTTPS_LoopbackForcesInsecure`, `_ForwardedHostLocalhostForcesInsecure`, `_OriginLoopbackForcesInsecure`, `_HTTPS_PrivateIP_Secure`, `_HTTP_PublicIP_Secure` (host `203.0.113.5`, public TEST-NET-3), `TestClearSecureCookie` | mixed | `Secure: true` | `Secure: true` | **No change** — already correct; these are the regression guard that the fix doesn't over-broaden `Secure: false` beyond the local+http case. |

**New backend tests required (TDD, write red first):**
1. `TestSetSecureCookie_HTTP_TailscaleCGNAT_Insecure` — host `100.98.12.109` (the exact bug-report IP), `X-Forwarded-Proto: http` → assert `Secure: false`, `SameSite: Lax`. This is the direct regression test for the reported bug.
2. `TestSetSecureCookie_HTTP_TailscaleCGNAT_Boundary` (or fold into `TestAuthHandler_HelperFunctions`'s existing `isLocalOrPrivateHost`/`isLocalRequest` subtest) — assert `isLocalOrPrivateHost("100.64.0.1")` and `isLocalOrPrivateHost("100.127.255.254")` are `true` (inside `100.64.0.0/10`), and `isLocalOrPrivateHost("100.63.255.255")` / `isLocalOrPrivateHost("100.128.0.1")` are `false` (just outside the block on each side) — pins the CIDR boundary so a future edit can't silently widen or shrink it.
3. `TestAuthHandler_Logout_InvalidatesSessionBeforeClearingCookie` — documents the §9.7 known-limitation mitigation: asserts `InvalidateSessions` is called (and, per the current code, completes) regardless of the request's scheme/locality, so a stale/non-cleared client cookie after a cross-scheme logout is inert rather than a live session. See §9.7 for full rationale.

**Frontend:** `frontend/src/pages/__tests__/Backups.test.tsx:236` already asserts `window.location.href` is set to the download URL on button click — this is Vitest-level (JSDOM), doesn't exercise cookies/auth, unaffected by this fix, no change needed.

**E2E (Playwright):** `tests/tasks/backups-create.spec.ts:553-598` (`should download backup file successfully`) and `tests/tasks/long-running-operations.spec.ts:332-333` both intercept the download request with `page.route(...)` (mocked) or only assert the button is enabled — neither exercises the real cookie-based navigation download. `backups-create.spec.ts:581-592` has an explicit comment: *"Since Playwright can't track navigation-based downloads directly, we verify the download button triggers the correct action"* with the real `page.waitForEvent('download')` flow left commented out. **Conclusion: no existing E2E coverage exercises the cookie-fallback-over-HTTP path this bug lives in; it would not have caught this bug and won't verify the fix.** Recommend (not mandatory — Playwright's own dev server runs on `localhost`, which was never actually broken by this bug since `isLocalOrPrivateHost("localhost")` was already `true` and the *symptom* only manifests on non-localhost origins pre-fix... but see below) that Playwright Dev un-comment and enable the real `page.waitForEvent('download')` assertion in `backups-create.spec.ts` as part of hardening — this at minimum guards against a future regression on the happy path, even though it can't reproduce the Tailscale-specific angle (E2E runs same-origin `localhost`, not a CGNAT address) — the CGNAT boundary is covered at the unit level (§9.4, test 2 above) instead, which is the appropriate layer for it.

### 9.5 GORM security scan

**Not required.** This fix touches no `backend/internal/models/**`, no GORM queries, and no migrations — it's confined to `setSecureCookie`/`isLocalOrPrivateHost` (cookie/header logic only). Per CLAUDE.md §1.5, skip `./scripts/scan-gorm-security.sh --check` for this fix.

### 9.6 Commit Slicing Strategy (this addendum only)

**Decision:** same PR (#1136, `feature/backuprestore`), appended as additional ordered commits after the async-job work in §6 — not a new PR, per CLAUDE.md. All three commits below are **required**, including A2: it is already fully spec'd (§9.4's E2E finding) and low-effort, and CLAUDE.md's Definition of Done treats Playwright E2E as mandatory (step 1, "Run First") — leaving it "optional" would risk it being dropped under time pressure precisely because it's the one commit that isn't backend-unit-test-gated.

| # | Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|---|
| A1 | `fix: make auth cookie Secure flag scheme/network-aware, close Tailscale CGNAT gap` | Backend | `backend/internal/api/handlers/auth_handler.go` (`isLocalOrPrivateHost`, `setSecureCookie`), `backend/internal/api/handlers/auth_handler_test.go` (5 flipped assertions + 2 new tests per §9.4) | §6 commit 11 (end of existing PR work), or independent if landed first | `go test ./backend/internal/api/handlers/...`; `make lint-fast`; `lefthook run pre-commit` (confirm CodeQL Go scan shows zero high/critical — specifically that `go/cookie-secure-not-set` does not fire given the suppression comment, per §9.1.4's history) |
| A2 (**required**) | `test: exercise real navigation-triggered backup download in E2E` | Hardening | `tests/tasks/backups-create.spec.ts` (un-comment the `page.waitForEvent('download')` flow at lines 588-592, remove the `page.route` mock for this one test so it hits the real backend) | A1 | `npx playwright test --project=firefox tests/tasks/backups-create.spec.ts` |
| A3 | `docs: record Secure-cookie/Tailscale fix in current_spec.md` | Docs | `docs/plans/current_spec.md` (this section — already written) | A1 | Markdown lint / doc review only |

**Rollback/contingency:** purely a logic fix to an existing function with no schema, wire-format, or route change — `git revert` of A1 alone is safe and independent of every other commit in this PR (including the unrelated async-job work in §1-8). If CI's CodeQL scan flags the suppression comment as insufficiently justified, the fallback is to keep `Secure: true` unconditionally (today's behavior) and instead document that Tailscale/plain-HTTP users must use the in-app "Authorization header" download path if one is added later — but this is a last resort, not the intended outcome; the suppression comment in §9.2 is written to preempt that review objection with an inline truth-table citation.

### 9.7 Known Limitation (Supervisor review — acknowledged, not fixed here)

Login over HTTPS followed by Logout over HTTP (e.g. an admin who logs in from a public HTTPS domain, then later visits the same instance over its local/Tailscale HTTP address) can hit RFC 6265bis's **"Leave Secure Cookies Alone"** rule: a response delivered over a non-secure connection is not permitted to overwrite/clear a cookie of the same name/domain/path that was previously set with `Secure`. Since `clearSecureCookie` (`auth_handler.go:161-163`) now computes `Secure` per-request via the same `setSecureCookie` logic (§9.2), a `Logout` call made over plain HTTP from a local/Tailscale origin sends a non-`Secure` clearing cookie — which the browser may refuse to apply on top of a previously-`Secure` cookie set during an earlier HTTPS session, leaving a stale `auth_token` cookie in the browser's cookie jar client-side.

**Severity: low.** Confirmed via `Logout` (`auth_handler.go:211-223`): `h.authService.InvalidateSessions(userID)` (line 214) runs and is awaited **before** `clearSecureCookie(c, "auth_token")` (line 221) — so even if the client-side cookie clear silently fails, the token itself is already revoked server-side first, and any subsequent request presenting the stale cookie fails `AuthenticateToken` and is rejected as unauthenticated. The stale cookie is inert, not a live session.

**Not fixed here** — genuinely solving it (e.g. detecting the scheme mismatch and returning a redirect/instruction to clear cookies via a same-scheme request, or accepting the cookie will simply expire via `maxAge`) is a separate, small-but-nontrivial UX/security question outside this bug's scope (backup downloads returning 401), and mixing an HTTPS public domain and a plain-HTTP local/Tailscale address for the *same* browser's cookie jar is itself an edge case of an edge case. Recorded here so it's an acknowledged tradeoff, not a silently-missed one.

**Test coverage:** add one documentation-style test in `auth_handler_test.go`, e.g. `TestAuthHandler_Logout_InvalidatesSessionBeforeClearingCookie`, asserting the current, already-safe ordering: given a request context with `userID` set, call `Logout`, and assert (via a spy/mock `AuthService` or by checking `InvalidateSessions` was called) that session invalidation happens even when the response's `clearSecureCookie` cookie ends up non-`Secure` (local/HTTP request) — i.e. the test documents "revocation is server-side and unconditional, so a dropped/stale client cookie is inert" rather than attempting to fix or simulate actual browser cookie-jar behavior (which is out of Go's test harness's reach). Add this as an 8th item in §9.4's "New backend tests required" list, part of commit A1.

---
