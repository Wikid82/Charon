# Plan: Database maintenance follow-ups (back-off edge cases, temp dir ownership, naming) - GH #1427

Type: `fix:` / `refactor:` / `test:` / `docs:` follow-ups on the shipped #1422 feature (v0.44.0). **No `feat:` commit** (no new user-facing capability; no new endpoint, setting or UI), so CodeQL and Trivy are deferred to CI. **No `(security)` scope**: nothing here is a genuine vulnerability (item 4 is a local ownership annoyance, not an escalation; the existing owner check already refuses an unsafe directory).
Single PR into `development`, ordered commits.
Branch: `fix/db-maintenance-followups-1427` (already checked out, cut from `development`).
Status: IMPLEMENTED (commits 1-7 on branch fix/db-maintenance-followups-1427; pending PR/CI).
Supervisor history: rev 1 reviewed -> CHANGES REQUIRED (no blockers; 2 should-fix S1/S2, 8 nits). Rev 2 applies all of them (traceability table below). Supervisor answers recorded: Q1 no (no entrypoint chown), Q2 yes (64 MiB), Q3 yes (patch bump).
Rev 2 verified -> one should-fix (`logPlanSkip` misdescribed, test 4 wrong) and five nits; rev 3 applies them (explicit `case ReasonTooManyFailures:` Warn before `default`, test 4 rewritten around `captureLogs(t)`, stale line references, real-`main()` test also at the commit 3 gate, `currentEUID` in the tmpdir owner check, pragma wording, `consumesFlag` wording).

| Review finding | Handled in |
| --- | --- |
| S1 docs: pragma table row, wording of the limit, Q2 rationale | 3.6, 6, 8 (Q2), commit 7 in 5 |
| S2 false "later boot resets the counter" claim, comment fixes | 3.2 (Risks, decision a), file inventory in 3.2 |
| Nit 1 pre-conversion failure vs flag consumption | 3.2 (Design, Tests), acceptance 3 |
| Nit 2 `DiscardMarkerIfConverted` error handling | 3.3 (Design, Tests) |
| Nit 3 data dir `os.Stat` failure, root-CI test guard | 3.4 (Design, Tests) |
| Nit 4 `too_many_failures` skip logged at Warn | 3.1 (Design, Tests) |
| Nit 5 `isWriterBusy` driver code check | 3.2 (Design) |
| Nit 6 flag + counter >= 3 under env=off | section 1 (X3) |
| Nit 7 SIGTERM-looped conversion never counted | section 9 (follow-up, not in this PR) |
| Nit 8 real-`main()` test at commit 2 gate and at the end | section 5 (commit 2 and commit 3 gates, final DoD) |
Predecessor: the #1422 spec is archived at `docs/plans/archive/2026-10-01_database-maintenance-1422_spec.md` (status IMPLEMENTED, shipped in v0.44.0). Section and decision numbers in this plan that say "#1422 plan" refer to that file.

## 1. Introduction

GH #1427 collects six follow-ups from the review of the database maintenance feature. Each item was re-verified against the code on this branch and against the read-only live test container (`docker exec charon sqlite3 -readonly`, `ls`, `ps`). Two items are partly wrong or better solved differently than the issue suggests (1, 2); one is real but cosmetic (3); one is real but narrow (4); one is a pure rename (5); one is decided with a measurement (6). Total change is small: about 60 lines of production code, the rest tests and docs.

### Verdict table

| # | Issue claim | Verified? | Verdict | Fix (one line) |
| --- | --- | --- | --- | --- |
| 1 | Flag plus crash loop never backs off | Yes (`plan.go:93`, `state.go:273`, `plan_test.go` case "flag with counter at max runs" pins the bypass) | **Fix, differently** | Make the request bypass the *threshold*, not the *back-off*: `Decide` applies `MaxConvertAttempts` to a flagged run too and clears the flag when it stops (instead of clearing the flag when a marker is consumed). |
| 2 | Pre-conversion failures are not counted | Partly. Counting is right, but for a *read-only / unwritable* database the counter write fails too, so no back-off can be persisted; and the issue misses the third pre-conversion exit (non-busy probe error, `runner.go:257`). | **Fix (narrower than stated)** | Count Inspect failure, non-busy probe failure and non-busy marker-write failure; classify a `SQLITE_BUSY` marker write as `database_busy` (not counted). Document the unwritable-DB limit. |
| 3 | Stale marker is counted although the conversion completed | Yes, cosmetic: a mode-2 file never converts again, so the lingering `1` has no effect on any decision | **Fix (small)** | At startup, when the file is already incremental, discard this file's leftover marker and reset the counter instead of counting. |
| 4 | Root-run CLI can leave a root-owned `<data>/.tmp` | Yes in principle, rare in practice (see 3.4); the issue also misses the mirror case (existing charon-owned `.tmp` makes the root CLI log a bogus "unsafe" Warn) | **Fix, as suggested** | `PrepareTempDir` returns a new `ErrTempDirSkipped` when euid is 0 and the data directory is not owned by root; `ApplyTempDir` logs that at Debug. |
| 5 | `spike_test.go` is misnamed | Yes (451 lines, permanent regression tests; 3 topics) | **Fix** | `git mv` and split by topic; `TestSpike_` becomes `TestDriver_`; fix two code comments. |
| 6 | `journal_size_limit` evaluated but not applied | Yes (only mention left is the #1422 plan) | **Apply: 64 MiB** | One pragma in `database.Connect`, a named constant, two tests, docs sentence. Evidence in 3.6. |

### Other small defects found (kept in scope, listed separately)

| ID | Defect | Where | Handling |
| --- | --- | --- | --- |
| X1 | The `Failed` log line says "it will be retried on the next start" even when the failure was the third (the next start will not retry). | `runner.go` `logOutcome` (~L490) | Folded into commit 3: reword to "...it is retried on the next start unless it has failed 3 times". No behaviour change. |
| X2 | `TestRun_UninspectableDatabaseFailsWithoutConvertingOrCounting` and `TestRun_MissingConvertFailsWithoutCounting` encode the old "pre-conversion failures never count" decision. | `runner_test.go:555`, `:794` | Updated in commit 3 (the first now asserts a counted failure; the second stays: a missing `Convert` is a wiring bug, deliberately not counted). |
| X3 | `RequestOptimize` returns 200 *before* resetting the counter when the flag is already set, so pressing the button a second time never clears a stuck counter. | `database_maintenance_handler.go:~235` | **Not changed.** With the item 1 fix `Decide` clears the flag whenever it stops on the back-off, so the state "flag set and counter >= 3" no longer persists across a boot. Noted, no action. One residual: flag + counter >= 3 can still persist while `CHARON_DB_COMPACT_ON_START=off` (`Decide` returns `disabled_by_env` first, so the flag is not cleared). Harmless: `RequestOptimize` answers the already-set case with 200 first, and a fresh request while env=off gets 409, so the state cannot be reached or exploited through the UI, and it clears itself once env is re-enabled and a boot stops on the back-off. |

Explicitly **not** included (would be scope creep): early marker clear right after `VACUUM` returns (see 3.3 alternatives), entrypoint self-heal of a root-owned `.tmp` (open question Q1), a Plan-time writability probe for read-only databases, `chown` of root-run SQLite side files.

## 2. Research findings

### 2.1 How the pieces fit (verified)

- `Decide` (`plan.go:81-116`): the `case in.FlagRequested:` branch (L93) only checks the 100 MiB floor; the `Attempts >= MaxConvertAttempts` check (L105) lives in the `default:` branch, so a flagged boot never backs off.
- `Store.Load` (`state.go:~225`) -> `loadAttempts` (L240) -> `consumeMarker` (L273): a leftover `maintenance.in_progress` marker of the same inode increments and persists `maintenance.attempts` and is deleted. It runs *before* anything knows the auto-vacuum mode.
- Flag consumption: `Outcome.consumesFlag` (`runner.go:468`) is `Converted() || countFailure`. A kill leaves neither, so the flag stays (by design: "a run that merely skipped or was interrupted leaves the request set").
- The UI button (`RequestOptimize`) does `ResetAttempts` then `SetFlag`, so a press always starts from a clean counter; `notice()` judges with the same `Decide` the boot uses (`database_maintenance_handler.go:~187`). A `Decide` change is therefore reflected consistently in boot, notice and `Advise` (Advise zeroes flag and attempts, unaffected).
- Runner exits before the conversion (`runner.go`): `acquireConn` non-busy probe error (L257) -> `Failed/conversion_failed`, no count; `convert()` `Inspect` error (L286-288) and `SetInProgress` error (L296-298) -> `abortBeforeConversion` (L321) -> `Failed/conversion_failed`, no count. `database_busy` (acquire timeout or `ErrWriterBusy` after 3 probe retries) is `Skipped` and is never counted.
- All settings writes in `persist()` (attempts, last_result, marker clear, flag clear) go through the same `settings` upsert as `SetInProgress`.
- The #1422 real-`main()` test (`cmd/api/maintenance_sigterm_test.go`, `TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts`) never sets the flag, so it is unaffected by item 1; it asserts "attempts == 0 right after the stop" and "converges within `MaxConvertAttempts` boots", both preserved.

### 2.2 Live container observations (read-only, 2026-10-02)

- `charon` runs with `Config.User = 0:0`; PID 1 is the entrypoint as root, `/app/charon` and `caddy` run as `charon` (entrypoint drops privileges with `gosu`). `docker exec` therefore runs as **root**.
- `/app/data` is `charon:charon 0750`; `/app/data/.tmp` is `charon:charon 0700` (created by the service at first boot). `.docker/docker-entrypoint.sh` creates/chowns `caddy`, `crowdsec`, `geoip` but **not** `.tmp`.
- `maintenance.last_result` = `converted` (2.28 GB -> 1.27 GB); no `maintenance.in_progress`, `attempts` or `compact_requested` rows; `auto_vacuum=2`, `journal_size_limit=-1` (default), live WAL 6.1 MB (steady state is small).
- `charon.log` contains the expected optimization lines and **no** "temp directory not prepared" warning, i.e. item 4 has not happened here.

### 2.3 Measurement for item 6 (sqlite3 3.x CLI, scratch files under `/var/tmp`, deleted)

One long-lived connection (like Charon's pool: `MaxOpenConns(1)`, `ConnMaxLifetime(0)`), WAL mode, one 200 000-row insert transaction, then 1 500 small autocommit inserts:

| Setting | WAL size after the burst | WAL size after 1 500 small writes |
| --- | --- | --- |
| default (no limit) | 43 494 872 B | 43 494 872 B (stays at the high-water mark for the life of the connection) |
| `PRAGMA journal_size_limit=4194304` | 43 494 872 B | **4 194 304 B** (truncated to the limit at the first WAL reset) |

A WAL file is only shrunk by `wal_checkpoint(TRUNCATE)` or when the last connection closes; Charon holds its connection for the whole process lifetime.

## 3. Technical specification, per item

### 3.1 Item 1 - flag plus crash loop never backs off

**Reproduction** (reasoning, covered by the new test): button -> counter 0, flag set. Boot A: converts, process hard-killed mid-`VACUUM` (marker remains). Boot B: `Load` counts 1, `Decide` takes the flag branch, converts, killed again ... `Attempts` climbs 2, 3, 4 ... and the flag is never consumed, so every boot repeats the conversion with a 503 window. Unflagged runs stop after 3.

**Alternatives evaluated**

| Option | Behaviour | Verdict |
| --- | --- | --- |
| A. (issue) `StartupPlan` clears the flag when a leftover marker was consumed | Bounded, but the request is dropped after **one** observed kill. For a database that is only eligible via the request (reclaimable >= 100 MiB but < 20% free and < 1 GiB, the "below threshold" band) the next boot is `below_threshold`: one power cut silently cancels an explicit user request. Needs a new `State.MarkerConsumed` plumbed from `consumeMarker` through `Load` and `StartupPlan`; `Decide` stays unbounded for any other path that leaves flag+counter set. | Rejected |
| B. **`Decide` applies the back-off to a flagged run too** | The request still bypasses the *threshold* (that is its purpose) but not the *failure limit*. A flagged run converts up to `MaxConvertAttempts` times (kills/failures), then stops with `too_many_failures` and **clears the flag**. Pure function change, one test table; boot, `notice()` and `Advise` stay consistent because they share `Decide`. | **Chosen** |
| C. Count the attempt and let the flag bypass only once | Same bound as A with the same drop-on-first-failure problem, plus new state. | Rejected |

**Design (B)** - `backend/internal/dbmaint/plan.go`, `Decide`:

- Keep order: env off -> already incremental -> threshold step. Split the current `switch`: the flag branch keeps its floor check (`nothing_to_reclaim` + `ClearFlag`), the default branch keeps `worthwhile` (`below_threshold`) but **loses** its `Attempts` check.
- After the `switch`, one shared check: `if in.Attempts >= MaxConvertAttempts { d := skip(ReasonTooManyFailures); d.ClearFlag = in.FlagRequested; return d }`.
- `logPlanSkip` (`startup.go:109`): log `ReasonTooManyFailures` at **Warn**, so the boot-D stop that ends a crash loop is easy to find in the log. Today `logPlanSkip` is a `switch`: `""` and `ReasonBelowThreshold` log nothing, `ReasonInsufficientDisk` logs Warn, and only the `default` branch logs Info. Add an explicit `case ReasonTooManyFailures:` that logs Warn **before** `default`; every other reason keeps its current level (below_threshold stays silent).
- Update the doc comment of `Decide` ("the user's flag (the floor still applies)" -> "the user's flag replaces the threshold, never the failure back-off").

**Interactions checked**

- Button semantics: unchanged. The button resets the counter, so a user who presses it gets 3 fresh tries (`MaxConvertAttempts`). Total conversions after a press that always gets killed: boots A, B, C convert (counter 0, 1, 2), boot D sees counter 3 and stops. Same as the unflagged path.
- `too_many_failures` notice wording ("a plain restart never retries; only the button does"): now true for flagged and unflagged alike; the flag is cleared at plan time by the existing `settlePlanSkip` (`startup.go`: it already handles `ClearFlag` and `ReasonTooManyFailures` together, and writes the `too_many_failures` last_result so `SuppressesPending` silences `Advise`). No handler or frontend change.
- `file_id` inode validation: unchanged; a replaced file drops its counter in `storedAttempts` and starts from 0, while the flag (which lives in the DB and travels with it) is evaluated afresh.
- Runner cancel/SIGTERM: an orderly stop is `interrupted` (not counted), a stop in the copy-back tail is `converted` or a leftover marker (counted once at the next boot). Counting is unchanged; only the consequence of reaching 3 changes.
- Counted *failed* conversions still consume the flag via `consumesFlag` (existing semantics, `TestRun_FlagIsConsumedByAConversionAndByACountedFailure`); B only closes the kill path, where the flag stays. Item 1 does not change `consumesFlag`; item 2 adds `keepFlag` (see 3.2).
- X3 (second button press while flag set): not needed, see section 1.

**Tests (TDD, write failing first)**

1. `plan_test.go`: change the table row "flag with counter at max runs" to `wantReason: ReasonTooManyFailures, wantClear: true`; add rows: flag + `Attempts: Max-1` runs; flag + `Attempts: Max` + below-floor stays `nothing_to_reclaim`; flag + `Attempts: Max` + `Stats` incremental stays `already_optimized` (order unchanged); no flag + `Attempts: Max` + below threshold stays `below_threshold` (unchanged).
2. `startup_test.go`: `TestStartupPlan_FlaggedCrashLoopStopsAfterMaxConvertAttempts` using the real settings DB helper (`newSettingsDB`) and a real legacy scratch database (`newScratchDB` with free pages above the floor): loop `MaxConvertAttempts` times { `SetFlag` once at the start; write a marker for the current `FileID`; run `StartupPlan`; assert `Decision.Run` } then one more iteration asserts `Reason == ReasonTooManyFailures`, `ClearFlag == true`; then call `Start` with the real planner and assert the flag row is gone and `last_result` is `skipped/too_many_failures`. This reproduces the loop without a 240 MB real-`main()` run.
3. `database_maintenance_handler_test.go`: one notice case: flag set + counter at max -> `noticeTooManyFailures` (documents that the UI and boot agree).
4. `startup_test.go`: `TestLogPlanSkip_TooManyFailuresIsWarn` captures the logger output with the existing hook `captureLogs(t)` (`backend/internal/dbmaint/drain_test.go:38`: calls `logger.Init(false, buf)` and returns a `*lockedBuffer`; Info and Warn are both captured; `advise_test.go` and `reclaim_test.go` already use it) and asserts: Warn for `too_many_failures`, Info for another default-branch reason such as `already_optimized`, and NO output at all for `below_threshold`.
5. Existing `TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts` must stay green unchanged (regression of the SIGTERM/SIGKILL paths). It is slow (`-short` skips it); run it explicitly in the gate.

**Risks / rollback**: behaviour change only for the already-broken loop and for "flag + 3 counted attempts", which previously ran a fourth time. Rollback: revert the commit (pure function + tests).

### 3.2 Item 2 - failures before the conversion are not counted

**What actually happens today**: three pre-conversion exits end `Failed/conversion_failed` without `countFailure`: non-busy probe error (`runner.go:257`), `Inspect` error (L286), `SetInProgress` error (L296). Each reaches `settle` -> `persist` -> `Gate.Finish(PhaseFailed)`; the 503 window already started at `BeginChecking`. Nothing is counted and the flag is not consumed, so every boot repeats it.

**Evidence that the issue's example is only partly right**: for a read-only or full database the marker write fails and so does `RecordFailure` (same `settings` upsert, via the pool): `persist` logs "could not record the result" and no counter exists to back off from. The existing test `TestRun_MarkerWriteFailureFailsWithoutConverting` (`DROP TABLE settings`) demonstrates exactly that: the count is unobservable. Counting therefore helps where the failure is *selective*: `Inspect` (a read: pragmas and stat of the file; the DB stays writable, so the counter persists), a non-busy probe error that is not "everything is unwritable", and a marker write that fails for its own reason (the marker goes through the *pinned* connection, `NewStore(conn)`, while the counter goes through the pool, so they can diverge).

**Design** - `backend/internal/dbmaint/runner.go`:

- `abortBeforeConversion`: set `out.countFailure = true` in the non-cancelled branch (a cancelled ctx stays `Cancelled/shutting_down`, never counted - existing behaviour kept).
- Non-busy probe error branch in `acquireConn` (L257): add `Outcome{..., countFailure: true}`.
- `SetInProgress` error: if `isWriterBusy(err)` (a writer slipped in between the probe's `ROLLBACK` and the marker write; the pool-wide `busy_timeout=5000` makes this rare but possible), settle as `Skipped/database_busy` and do **not** count (retry-friendly, flag kept). Any other error counts.
- Flag handling (nit 1 decision): a counted *pre-conversion* failure does **not** consume the flag. Add an explicit `keepFlag bool` on `Outcome` set by `abortBeforeConversion` and the probe branch, and make `consumesFlag` = `Converted() || (countFailure && !keepFlag)`. Rationale: item 1 rejected dropping an explicit request after one observed kill; dropping it after one transient `Inspect`/probe error would be the same mistake (a database in the below-threshold band is only eligible through the request). The bound is the shared back-off from item 1: after `MaxConvertAttempts` counted failures `Decide` stops with `too_many_failures` and clears the flag. A failed `VACUUM` keeps today's semantics (consumes the flag), since that failure happened inside the conversion.
- `isWriterBusy` hardening (nit 5): **in**, small. Add `errors.As` against the driver's error interface exposing `Code() int` and treat `code & 0xff == 5` (`SQLITE_BUSY`) as busy, next to the existing string match (kept as fallback). Reason: the string match depends on driver message wording, and this classification now decides whether a failure is counted. Test: table over a fake error type with `Code()` 5, 261 (BUSY_RECOVERY, low byte 5), 6 (LOCKED, not busy), and the existing strings.
- `Inspect` error: counted (read failures are real).
- `ErrNoConvert` and the `ErrConfigNotApplied`/timeout/quick_check/acquire-timeout skips stay uncounted (wiring bug, environment-readiness, not failures).
- X1: reword the `ResultFailed` log in `logOutcome` to "database optimization failed; it is retried on the next start unless it has failed 3 times".

**What the user sees**: after three counted failures (not necessarily consecutive, see Risks), `too_many_failures` (existing notice and docs, "container killed during startup / no disk space"); before that the existing failed state and Error log. A flagged request survives counted pre-conversion failures (`keepFlag`) and is cleared only by a conversion, a failed `VACUUM`, or the back-off stop.

**File inventory addition**: fix the comments `State.Attempts` (`state.go:64`) and `Inputs.Attempts` (`plan.go:58`): "consecutive failed conversions" -> "failed conversion attempts recorded for this file since the last successful conversion or manual reset (not necessarily consecutive)".

**Tests**

1. `runner_test.go`: `TestRun_PreConversionFailuresAreCounted` table: (a) `Inspect` failure via `h.deps.DBPath = h.path + ".missing"` (rewrite of `TestRun_UninspectableDatabaseFailsWithoutConvertingOrCounting`, X2) -> `attempts()==1`, `convertCalls==0`; (b) non-busy probe error via `h.deps.Probe` returning `errors.New("disk I/O error")` -> counted; (c) marker write failure that leaves the counter writable: create a SQLite trigger on `settings` (`CREATE TRIGGER no_marker BEFORE INSERT ON settings WHEN NEW."key"='maintenance.in_progress' BEGIN SELECT RAISE(ABORT,'readonly'); END`) -> `Failed`, `attempts()==1`, `convertCalls==0`; (d) marker write failing with a busy error (trigger raising `database is locked`... or a fake via the pinned-conn seam if the trigger cannot produce the string; fallback: unit test the classification helper `isWriterBusy` + a runner test using a `SQLExecer` seam) -> `Skipped/database_busy`, not counted, flag kept; (e) cancelled ctx during Inspect/marker stays `Cancelled`, not counted (existing `TestRun_CancelJustBeforeTheConversionStartsIsNotAFailure` keeps passing).
2. Keep `TestRun_MarkerWriteFailureFailsWithoutConverting` (DROP TABLE) and extend its comment: the count is unobservable by design when the table is gone; assert no panic, one Warn, `PhaseFailed`.
3. Flag interplay: `TestRun_FlagIsConsumedByAConversionAndByACountedFailure` keeps its cases (conversion, failed `VACUUM`) and gains a "counted pre-conversion failure keeps the flag" subtest (flag row still set, `attempts()==1`), plus the `isWriterBusy` code table from the design.
4. Unwritable-database behaviour (documented limit): a test with the real DB opened `?mode=ro` is not required; the DROP TABLE test above covers "state cannot be recorded".

**Risks**: a transient non-busy error (a one-off I/O error, a probe/`Inspect` hiccup) now costs one of three tries instead of none. The counter is **not** a consecutive-failure counter: only a conversion resets it (`persist`, the `Result.Converted()` branch). A boot that merely skips (`below_threshold`, `database_busy`, `already_optimized`) leaves it alone, so one transient error followed by non-converting good boots leaves `attempts=1` dangling, and two more unrelated failures months later lock the user out of the automatic path until they press the button (which resets it). `database_busy` stays uncounted.

**Decision (S2): option (a), correct and document, no decay in this patch.** Justification: a decay needs either a new persisted timestamp (state and migration surface, not a patch-size change) or a reset on a "healthy skip", which is subtly unsafe (a skip such as `database_busy` is decided after the conversion attempt began, and an `already_optimized` reset is exactly item 3's job and already covered). The blast radius is small: the user-visible stop is the existing `too_many_failures` notice with a one-click remedy, and the automatic path only matters for databases that still need conversion. Document the limit in `docs/database-maintenance.md` ("failed starts are counted until the next successful optimization or until you press the button"). A time-based decay is a candidate follow-up if a report appears.

### 3.3 Item 3 - stale marker counted although nothing failed

**Cause**: `settleConverted` runs `checkpointWithRetry` (up to about 90 s: 6 retries, 2 s doubling, 30 s cap) *before* `persist` clears the marker. A SIGKILL/power cut in that window leaves marker + a database already in mode 2. Next boot `consumeMarker` counts 1 and nothing resets it (`ResetAttempts` runs only on a conversion). Effect: a lingering `maintenance.attempts = 1`; harmless to decisions (a mode-2 file never converts again, `Decide` returns `already_optimized` first) but a false failure signal in the state.

**Alternatives**: (a) clear the marker immediately after `VACUUM` returns nil (root-cause shrink of the window) - rejected: reorders `settleConverted` against tests that pin the M1 ordering of the cancelled path, and still leaves the millisecond window plus existing v0.44.0 stale markers; (b) detect at load - **chosen**, covers every cause and installs already carrying a stale marker.

**Design**

- `backend/internal/dbmaint/state.go`: new `func (s *Store) DiscardMarkerIfConverted(ctx, fileID string) (bool, error)`: if a marker for the same file exists, delete it and `ResetAttempts` (nothing failed and nothing will ever convert this file again), return true; another file's marker is left to `consumeMarker`'s existing deletion. Idempotent.
- `backend/internal/dbmaint/startup.go` `StartupPlan`: before `Load`, read the mode with the existing `readPragma(ctx, db, "auto_vacuum")`; if it equals `AutoVacuumIncremental`, call `DiscardMarkerIfConverted` and log one Info line ("database optimization had completed before an earlier shutdown; clearing the leftover marker"). If `DiscardMarkerIfConverted` returns an error, log a **Warn** and continue (do not fail the plan: aborting would skip optimization for that boot over a cosmetic cleanup; `Load` then behaves exactly as before, counting the marker once). `Load` itself keeps its signature (7 call sites in tests/handler unchanged) and then finds no marker.
- `Peek` (status endpoint) untouched (read-only by contract).
- Do **not** write a `last_result` (bytes unknown; the UI would show 0 -> 0). A `converted` history entry is not needed: the status page derives "optimized" from the live `auto_vacuum` value.

**Tests**: `state_test.go`: marker + counter 2 for the same file -> `DiscardMarkerIfConverted` returns true, marker gone, counter 0; other file's marker -> false, untouched; no marker -> false; closed DB -> error. `startup_test.go`: `StartupPlan` with `DiscardMarkerIfConverted` failing (a trigger on `settings` that aborts the marker delete, or a closed pinned store) on a mode-2 DB still returns a plan (no error, `already_optimized`) and logs one Warn. Also: `StartupPlan` on a mode-2 scratch DB with marker + counter 1 -> counter 0, marker gone, `Decision.Reason == already_optimized`; on a mode-0 DB the marker still counts (regression of `TestStore_LeftoverMarkerForTheSameFileCountsAsAFailedAttempt`, which stays green unchanged).

### 3.4 Item 4 - root-run CLI creates a root-owned `<data>/.tmp`

**Who can create it** (verified): the entrypoint (root) does not; the service (euid `charon`) does at first boot; only a **root** process that reaches `loadConfigForDatabase` first can create it root-owned: `docker exec charon /app/charon reset-password|migrate` (root in this compose) or `docker run --rm -v data:/app/data charon /app/charon migrate` against an upgraded-but-never-started volume, or an exec in the first seconds of a boot. Since `.tmp` exists after the first boot of v0.44.0+, the window is small. The mirror case is real and today's behaviour: with a charon-owned `.tmp` present, a root CLI fails the owner check (`tmpdir.go:56`, `Uid != Geteuid`) and logs the Warn "database temp directory not prepared" on every CLI call (noise only).

**Alternatives**: (1) skip when euid 0 and the data dir is not root-owned - **chosen** (the issue's suggestion); (2) `chown` to the data dir owner after `Mkdir` - rejected: more code, a symlink-swap surface under a root process, and the owner check would have to be loosened for root; (3) do not prepare the temp dir for CLI subcommands - rejected: `migrate` builds/drops indexes on possibly multi-GB tables and benefits from the data volume instead of a small `/tmp` (the original #1422 reason, `main.go:75-86`); (4) lazy creation only in the server path - rejected: same effect as (3), plus restructuring `main`.

A root process that skips simply leaves SQLite on its default search order (`/var/tmp`, `/usr/tmp`, `/tmp`), which is what v0.43 did for the CLI.

**Design** - `backend/internal/dbmaint/tmpdir.go`:

- The existing owner check (`tmpdir.go:56`, `int64(st.Uid) != int64(os.Geteuid())`) must also call `currentEUID()` instead of `os.Geteuid()`, so the seam is consistent and a test that stubs the euid sees the same value in both places.
- Add `var ErrTempDirSkipped = errors.New("temp directory left to the data directory owner")` and a package-level seam `var currentEUID = os.Geteuid` (tests only).
- In `PrepareTempDir`, before the `Lstat`: `os.Stat` the cleaned data dir; if `currentEUID() == 0` and the owner uid != 0, return `ErrTempDirSkipped` (no create, no chmod). If that `os.Stat` fails (missing or unreadable data dir), do not skip: fall through to the existing behaviour (the `Lstat`/`Mkdir` path and its own errors, logged by `ApplyTempDir` as a Warn as today). Test: nonexistent data dir with `currentEUID` stubbed to 0 returns the existing error, not `ErrTempDirSkipped`. All existing rules stay: `Lstat` first, refuse symlinks/non-directories, owner must equal euid, mode tightened to 0700. A server running as root against a root-owned data dir (rootless Docker maps uid 0) behaves exactly as before.
- `ApplyTempDir`: `errors.Is(err, ErrTempDirSkipped)` -> `logger.Log().Debug(...)` and return (no Warn); other errors keep the Warn.
- `main.go` and the comment on `loadConfigForDatabase` need no code change (add one sentence to the doc comment).

**Tests** (`tmpdir_test.go`): table-test a pure helper `rootShouldSkip(euid int, ownerUID uint32) bool`; `PrepareTempDir` with `currentEUID = func() int { return 0 }` on a t.TempDir owned by the (non-root) test user -> `ErrTempDirSkipped` and **no `.tmp` created**; same with a pre-existing `.tmp` (no Warn path, nothing modified, mode untouched); with `currentEUID` returning the real uid the existing tests behave as before; `ApplyTempDir` with the seam -> env unset, no directory. A root-owned data dir cannot be built in CI without root: covered by the pure helper. Guard: the seam tests that assume a non-root test user (data dir owned by the test user, stubbed euid 0) start with `if os.Geteuid() == 0 { t.Skip("owner of t.TempDir would be root; covered by rootShouldSkip") }`, so they do not misbehave when CI runs the tests as real root (the pure-helper table has no such dependency). The existing symlink/regular-file/other-owner refusal tests stay green.

**Existing installs already affected** (root-owned `.tmp`): the service logs the Warn each boot and falls back, harmless. Manual fix goes in the troubleshooting docs: `docker exec charon chown charon:charon /app/data/.tmp` (see Q1 for an automatic fix).

### 3.5 Item 5 - naming

`backend/internal/dbmaint/spike_test.go` (451 lines) holds permanent regression tests pinning `glebarez/go-sqlite` behaviours. Split by topic with `git mv` so history follows:

| New file | Content moved from `spike_test.go` | Rename |
| --- | --- | --- |
| `backend/internal/dbmaint/driver_behavior_test.go` (`git mv` of the original) | everything that exercises SQLite/driver semantics: pragma order, VACUUM on a pinned conn, BEGIN EXCLUSIVE, `incremental_vacuum` Query vs Exec, VACUUM INTO, open rows block the pool, checkpoint busy column, prepared statements after VACUUM, VACUUM cancel early/late, plus the `buildCancelDB`/`vacuumWithCancel`/`openTempFDs`/`queryDrainStep` helpers | `TestSpike_*` -> `TestDriver_*`; header comment already says "Regression tests that pin the behaviour of the driver" - keep, drop the word "spike" |
| `backend/internal/dbmaint/tmpdir_test.go` (append) | `TestHelperTmpdirSpike`, `runTmpdirSpike`, `TestSpike_SQLiteTmpDirOnlyHonouredBeforeFirstOpen` (topic: `SQLITE_TMPDIR`; sits next to the tmpdir tests) | `TestHelperTmpdirSpike` -> `TestHelperTmpdirProbe`; `runTmpdirSpike` -> `runTmpdirProbe`; env vars `CHARON_TMPDIR_SPIKE*` -> `CHARON_TMPDIR_PROBE*`; the `-test.run` regexp string in the helper launcher must change with the function name; test -> `TestDriver_SQLiteTmpDirOnlyHonouredBeforeFirstOpen` |
| `backend/cmd/api/gin_listener_test.go` | `TestSpike_GinRunListenerServesOnABoundListener` (it pins the gin API `main` relies on; it does not belong in `dbmaint`) | `TestGin_RunListenerServesOnABoundListener`; moves its `gin`, `net`, `io`, `http` imports with it |

Also change the two code comments: `drain.go:63` ("(spike_test.go)" -> "(driver_behavior_test.go)") and `tmpdir.go:16` ("spike-verified, see spike_test.go" -> "verified by TestDriver_SQLiteTmpDirOnlyHonouredBeforeFirstOpen"). Archived plan text keeps its historical wording (no edit to `docs/plans/archive/`). A repo-wide `grep -rni "spike_test\|TestSpike"` must be empty afterwards (outside the archive). No Codecov/Sonar path rules reference the old name (checked: `grep -rn spike_test .codecov.yml sonar* .github` is empty).

Test count and names must be identical modulo the rename; the gate compares `go test -list '.*' ./internal/dbmaint ./cmd/api` before/after (same count, new names).

### 3.6 Item 6 - `PRAGMA journal_size_limit`

**Evidence** (section 2.3): with a permanent single connection the WAL file keeps the size of its largest burst until a `TRUNCATE` checkpoint; with the limit it is truncated at the first WAL reset after the burst. Existing mitigations: pruner `wal_checkpoint(TRUNCATE)` only when a pass deleted >= 50 000 rows; `Drain` and the conversion checkpoint truncate when they run. What the limit actually does (corrected, verified by re-running the experiment): it only trims a WAL that is **larger than the limit**, at the next WAL reset after a checkpoint. A 43.5 MB WAL stays 43.5 MB with a 64 MiB limit; with a 4 MiB limit it trims to about 4.19 MB. So at 64 MiB it caps leftover growth at about 64 MB; it does not "keep the log small". Gaps it closes: (a) the pruner's `wal_checkpoint(TRUNCATE)` is threshold-gated (>= 50 000 deleted rows), so a burst that is not a big prune (boot-time index builds, restore/import, `AutoMigrate`, the bulk `DELETE` of many hosts) leaves the high-water mark in place for the life of the connection; (b) `converted_pending_checkpoint` after the `VACUUM` copy-back (the WAL can be as large as the database when a reader pinned the checkpoint): once the reader is gone the next auto-checkpoint + WAL reset trims it to the limit. Steady-state WALs are far below 64 MiB (the live one is 6 MB), so the limit never touches them. It does not replace the explicit TRUNCATE checkpoints (it is a trim, not a forced checkpoint, and does nothing while a reader pins the WAL).

Concerns checked: it is a per-connection setting, and `configurePool` allows exactly one connection with `ConnMaxLifetime(0)`, so `sqlDB.Exec` in `Connect` configures the connection the whole process uses (the same assumption `busy_timeout`/`synchronous`/`cache_size` already rely on; a driver-recycled connection would lose all of them equally). `dbmaint` pins that same connection for `VACUUM`, so the setting is active during the conversion. `VACUUM INTO` (backups) writes a separate file, not this WAL. The boot `quick_check` uses its own connection (default behaviour, read-only, unaffected). Cost: one `ftruncate` at a WAL reset when the file exceeds the limit; a WAL that stays under 64 MiB (the live one is 6 MB) is never touched.

**Verdict: apply, 64 MiB.** Large enough to never act at steady state, small enough to cap leftover growth from bursts the pruner threshold misses.

**Design**: `backend/internal/database/database.go`: add `const journalSizeLimitBytes = 64 << 20` and the pragma `fmt.Sprintf("PRAGMA journal_size_limit=%d", journalSizeLimitBytes)` to the pragma list after `journal_mode=WAL`. `gofmt` alignment of the trailing comments in that slice is preserved.

**Tests**: `database_test.go`: `Connect` on a temp file -> `PRAGMA journal_size_limit` returns `67108864`. `driver_behavior_test.go`: `TestDriver_JournalSizeLimitTruncatesTheWALAfterAReset` on a scratch DB with a 1 MiB limit: one large insert transaction (> 1 MiB WAL), then a few hundred small autocommit writes on the same single connection; assert the `-wal` file size ends <= limit, and (control) the same sequence without the limit leaves it larger. Deterministic (single connection, no timers). Docs: a `journal_size_limit` row in the "Database Configuration" table plus one sentence in "WAL File Is Very Large" (section 6).

**Rollback**: remove the one pragma line and the tests; no persisted state (the pragma is not stored in the file).

## 4. Implementation plan

### Phase 1 - Playwright
No user-visible behaviour changes (no UI, API or notice wording change), so **no new or changed E2E specs**. The existing Tasks -> Database spec is untouched; CI runs the suite.

### Phase 2 - Backend
Commits 1-6 below. No models, no migrations, no new routes.

### Phase 3 - Frontend
None.

### Phase 4 - Integration and testing
Real-`main()` regression: `go test ./cmd/api -run TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts -count=1` (slow; builds a 240 MB scratch DB) at the commit 2 and commit 3 gates and once more at the very end (after commit 7), and the item 1 loop test with the real settings DB (unit-level, fast).

### Phase 5 - Documentation
`docs/database-maintenance.md` (details in 6). `ARCHITECTURE.md`: no change (the pragma set is not listed there; no new component, directory or security-architecture change). `docs/features.md`: no change.

## 5. Commit Slicing Strategy

Decision: **one PR** into `development` (`fix/db-maintenance-followups-1427`), ordered logical commits, each green on its own gate. Test-first inside each commit (red then green; the tests ship in the same commit as the fix so every commit builds and passes). No E2E commit. ENV for every gate: `export TMPDIR=/var/tmp GOTMPDIR=/var/tmp` (the 64 MB `/tmp` tmpfs is full of leaked files, GH #1426), run foreground and blocking.

Common gate for every code commit (from `/projects/Charon/backend`): `go build ./... && go vet ./internal/dbmaint/... ./internal/database/... ./cmd/api/...` and `go test -race -count=1 ./internal/dbmaint/... ./cmd/api/...` (add `./internal/database/...` in commit 6 and `./internal/api/handlers/...` in commit 3), plus `make lint-fast` (lefthook pre-commit runs staticcheck and is blocking).

| # | Commit (conventional) | Scope / files | Depends on | Validation gate (beyond the common gate) |
| --- | --- | --- | --- | --- |
| 1 | `refactor: rename and split the dbmaint driver regression tests` | `git mv backend/internal/dbmaint/spike_test.go driver_behavior_test.go`; move tmpdir helper+test into `tmpdir_test.go`; move gin test to `backend/cmd/api/gin_listener_test.go`; rename `TestSpike_`/`TestHelperTmpdirSpike`/env vars; comments in `drain.go:63`, `tmpdir.go:16`. No behaviour change. | - | `go test -list '.*'` of both packages: same number of tests, only names differ; `grep -rniE "spike_test|TestSpike|TMPDIR_SPIKE" backend` (excluding none) is empty; `go test -race` passes incl. the re-exec helper. |
| 2 | `fix: stop a reclaim request from bypassing the failure back-off` | `plan.go` (`Decide`), `plan_test.go`, `startup_test.go` (loop test), `database_maintenance_handler_test.go` (notice case) | 1 (new test files land in final locations) | Common gate with handlers; targeted `go test ./internal/dbmaint -run 'TestDecide|TestStartupPlan|TestStart'`; real-`main()` test `TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts` (SIGTERM and SIGKILL, slow, run explicitly with `-count=1`, not `-short`) passes; also covers the `logPlanSkip` Warn test. |
| 3 | `fix: count failures that happen before the conversion starts` | `runner.go` (`abortBeforeConversion`, `acquireConn` probe branch, `keepFlag`/`consumesFlag`, busy classification of the marker write, `isWriterBusy` in `probe.go`, X1 log text, `Attempts` comment fixes in `state.go`/`plan.go`), `runner_test.go` (new table, X2 rewrite) | 2 | Common gate; `go test ./internal/dbmaint -run 'TestRun_'`; real-`main()` test `TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts` (SIGTERM and SIGKILL, slow, `-count=1`, not `-short`) passes again, because this commit changes the runner; no assertion in the suite still depends on "pre-conversion failures never count". |
| 4 | `fix: ignore a leftover marker once the database is already optimized` | `state.go` (`DiscardMarkerIfConverted`), `startup.go` (`StartupPlan`), `state_test.go`, `startup_test.go` | 2 | Common gate; `TestStore_LeftoverMarkerForTheSameFileCountsAsAFailedAttempt` unchanged and green. |
| 5 | `fix: leave the temp directory alone when a root-run command targets another user's data directory` | `tmpdir.go` (`ErrTempDirSkipped`, `currentEUID`, `ApplyTempDir`), `tmpdir_test.go`, one sentence in the `loadConfigForDatabase` comment (`cmd/api/main.go:75`) | 1 | Common gate; existing refusal tests (symlink, file, other owner) green. |
| 6 | `fix: cap the leftover size of the write-ahead log after large bursts` | `backend/internal/database/database.go`, `database_test.go`, `driver_behavior_test.go` (behavioural test) | 1 | Common gate plus `go test -race ./internal/database/...`; the existing `Connect` tests and the pruner tests green; `TestDriver_JournalSizeLimit...` shows the control (no limit) larger than the limited run. |
| 7 | `docs: document the follow-up behaviour of database maintenance` | `docs/database-maintenance.md` (6 below: pragma table row, WAL sentence, back-off notes, `.tmp` entry); `ARCHITECTURE.md` untouched | 2-6 | `markdownlint` via lefthook; links intact; `docs-site` manifest unchanged (no new file). |

Final Definition of Done for the PR (once, after commit 7):

1. Playwright: not applicable (no UI/API change); CI covers the suite. If the reviewer wants a smoke: `npx playwright test tests/<tasks-database spec> --project=firefox` only.
2. GORM scan: not triggered (no models/GORM queries; `Store` uses `database/sql`). Cheap to run anyway: `./scripts/scan-gorm-security.sh --check`.
3. `bash scripts/local-patch-report.sh` -> `test-results/local-patch-report.{md,json}`.
4. `lefthook run pre-commit`; `make lint-fast`; `make lint-backend` before the PR.
5. Coverage: `scripts/go-test-coverage.sh` >= the gate (85%, `CHARON_MIN_COVERAGE`); new code is fully covered by the tests above.
6. Real-`main()` SIGTERM/SIGKILL test, once more on the final tree: `go test ./cmd/api -run TestMaintenance_StopDuringConversionIsSafeAndTheNextBootConverts -count=1`.
6a. `cd backend && go build ./... && go vet ./... && go test -race -count=1 ./internal/dbmaint/... ./internal/database/... ./cmd/api/...` and the unraced remainder via the coverage script.
7. CodeQL and Trivy: deferred to CI (no `feat:` commit, no new code path with external input; item 4 only narrows a filesystem operation).
8. Frontend type-check/build: untouched, not required.

**Rollback and contingency**: every commit is independently revertable (1 is a rename, 6 is one pragma, 2-5 are small and covered by their own tests). If the real-`main()` test fails at the commit 2 or commit 3 gate, revert that commit and re-plan; do not weaken the test. If `TestDriver_JournalSizeLimit...` proves timing-sensitive on CI filesystems, keep only the `PRAGMA` read-back test in `database_test.go` and document the measurement (section 2.3) instead of the behavioural test. If the supervisor prefers, item 6 can drop to "documented as not needed" by omitting commit 6 and adding the evidence to docs; nothing else depends on it.

## 6. Documentation to update (commit 7)

`docs/database-maintenance.md`:

- "Automatic cleanup has stopped" (~L140) and troubleshooting entry (~L455): state that this also applies to a **Reclaim space on next restart** request: after 3 failed or interrupted starts Charon stops and the request is cleared; press the button again to retry. Keep the plain-language tone.
- "Database Configuration" table (L44-50, which already lists `journal_mode`, `busy_timeout`, `synchronous`, `cache_size`): add the row `| \`journal_size_limit\` | 64MB | Caps leftover write-ahead log growth at about 64 MB |`.
- "WAL File Is Very Large" (~L424): add that Charon caps leftover write-ahead log growth at about 64 MB on its own (the limit trims a log larger than 64 MB at the next checkpoint; it does not shrink smaller ones), and that the manual checkpoint is only needed for older versions or an unusually long-running reader. Never describe it as keeping the log small.
- Back-off note: failed starts are counted until the next successful optimization or until you press the button; they are not required to be consecutive.
- New short troubleshooting entry (data directory section, near the other `docker exec` advice): "Warning: database temp directory not prepared" after running `charon reset-password`/`migrate` as root: `docker exec charon chown charon:charon /app/data/.tmp` (or delete the folder while Charon is stopped) and restart. Mention that newer versions no longer create it as root.
- No change to the `too_many_failures` notice text in the UI (no frontend/i18n edit).

`ARCHITECTURE.md`: no change (verified: `grep cache_size ARCHITECTURE.md` has no hit, so the pragma set is not listed there).

## 7. Acceptance criteria

1. A flagged start that is hard-killed on every boot stops after `MaxConvertAttempts` conversions with `too_many_failures`, the flag cleared, no further 503 windows; pressing the button again starts a fresh 3-try cycle. Test: `StartupPlan` loop test.
2. `Decide(flag, attempts >= Max)` returns `too_many_failures` with `ClearFlag`; the floor (`nothing_to_reclaim`), `already_optimized` and `disabled_by_env` outcomes are unchanged.
3. An `Inspect` failure, a non-busy probe error and a non-busy marker-write failure each record one failed attempt; a `SQLITE_BUSY` marker write is `database_busy` and uncounted; shutdown-time failures are never counted; a missing `Convert` is still uncounted; counted pre-conversion failures keep a pending reclaim request (the flag), which only a conversion, a failed `VACUUM` or the back-off stop clears.
4. A marker left for a file that is already incremental is deleted at startup, the counter is reset, nothing is counted; a marker on a mode-0 file still counts one attempt.
5. A process with euid 0 and a non-root-owned data directory neither creates, modifies nor warns about `<data>/.tmp`; a non-root service or a root service on a root-owned data directory behaves as before; symlink/file/other-owner refusals unchanged.
6. No file, function or test name in `backend/` contains `spike`; the test count is unchanged by commit 1.
7. `Connect` sets `journal_size_limit` to 64 MiB; the behavioural test shows the WAL truncated to the limit after a burst.
8. Docs updated as in section 6. All DoD gates in section 5 pass; no test skipped, deleted or weakened (only the three intentionally flipped assertions listed in X2/3.1).

## 8. Open questions (genuine, with recommendations)

- **Q1. Should the entrypoint repair an already root-owned `/app/data/.tmp` at container start?** It is the only place that runs as root before the service and could `chown -h charon:charon` it (guarded by `[ -d ] && [ ! -L ]`). **Supervisor answer: no (no entrypoint chown).** Item 4's fix prevents new cases, the live system is not affected, a symlink-following `chown` as root is exactly the class of bug worth avoiding in a shell script, and the docs entry gives the one-line manual fix. Revisit only if an actual report appears.
- **Q2. `journal_size_limit` value and whether to apply it at all.** **Supervisor answer: yes, apply 64 MiB.** Rationale: it closes the gap left by the pruner's threshold-gated checkpoint and the boot-time index builds, while steady-state WALs are far below 64 MiB so it never touches them (evidence 2.3 and 3.6). If the maintainers prefer zero default-behaviour change in a follow-up release, the alternative is "documented as not needed" with the measurement in the docs; commit 6 is isolated for that.
- **Q3. Released as?** All commits are `fix:`/`refactor:` so the release is a patch bump (0.44.1) through release-please. **Supervisor answer: yes, patch bump** (the item 1 loop is a real, if rare, user-facing fault worth a patch release).

## 9. Follow-ups NOT in this PR (for the orchestrator to file)

- **SIGTERM-interrupted conversions are never counted.** An orderly stop mid-conversion ends `interrupted` and no marker remains, so a run that is repeatedly SIGTERMed (for example by an orchestrator healthcheck or restart policy firing during the 503 window of a multi-GB `VACUUM`) restarts the conversion on every boot forever, with no back-off. Pre-existing, independent of #1427, and outside this patch's scope. **Recommendation: file it as a GH issue** (low priority, `bug`, area database), proposing either counting an interrupted conversion that made no progress or a wall-clock budget; it needs a design decision, so it should not ride on this PR.
- Time-based decay of the failed-attempt counter (S2 option b), only if a report appears.
