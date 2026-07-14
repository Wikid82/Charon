# Pre-Merge Adversarial Audit — Remote Storage Backup Providers (WebDAV/Dropbox/Google Drive)

**Date:** 2026-07-14
**Branch:** `feature/backuprestore` (PR #1136)
**Auditor:** Supervisor (Code Review Lead) — independent adversarial pass
**Status of this document:** **Supplemental**, not a replacement. `docs/reports/qa_report.md` (2026-07-13) already gave this feature a full Definition-of-Done pass and a **READY TO MERGE** verdict; every gate in that report was independently re-verified by QA and is not repeated here. The user has deliberately chosen **not** to merge this cycle in order to soak the branch in dev/nightly a while longer, and asked for a fresh, adversarial look specifically for defects that pass today's zero-tolerance CI gates (CodeQL, Trivy, GORM scan, staticcheck, patch coverage thresholds) but would only surface later — in production, and especially during a real restore under stress. Nothing here contradicts the prior QA verdict; this is what a second, differently-motivated pass over the same diff turned up.

Methodology: read every file named in the audit brief in full, cross-referenced against `git log`/`git blame` to distinguish this-PR code from pre-existing code, and generated real coverage profiles (`go test -coverprofile`) rather than trusting the line numbers in the brief verbatim — several had drifted slightly from the current HEAD; the numbers below are from the actual profiles generated during this audit. Ran the **full** `golangci-lint` (`.golangci.yml`, not the fast pre-commit subset) against the touched packages, which the existing DoD process does not run as a hard gate.

---

## Findings, ranked by severity

### Critical

#### C1. A double-failure in `RestoreBackupSafe` reports "success" to the user and returns `nil` error, while the database was never actually restored

**File:** `backend/internal/services/backup_restore_safe.go`, lines 271–331 (the untested branch is lines 295–297, coverage count **0** in the profile generated during this audit).

**What/why:**

```go
rehydrated := false
if s.db != nil {
    ...retry loop for RehydrateLiveDatabase...
}
result.RestartRequired = !rehydrated

if !rehydrated {
    if pendingErr := s.writePendingRestoreFile(validated.restoreDBPath); pendingErr != nil {
        logger.Log().WithError(pendingErr).Error("failed to persist pending-restore file")
        // <-- no further action. result.DatabaseSwapPending stays false.
    } else {
        result.DatabaseSwapPending = true
        ...
    }
}
...
if result.DatabaseSwapPending {
    result.Message = "Backup restored; the database will finish restoring on the next process restart"
} else {
    result.Message = "Backup restored successfully"   // <-- reached even when nothing was restored
}
return result, nil   // <-- nil error, always
```

If live rehydrate fails (exhausts its 5 retries — e.g. the DB stays locked/busy) **and** `writePendingRestoreFile` also fails (disk full, permission error, `DataDir` on a read-only remount, etc.), the function still returns `(result, nil)` with `RestartRequired: true` but `DatabaseSwapPending: false`, and `Message: "Backup restored successfully"`. The frontend (`RestoreDialog.tsx`) branches only on `restart_required`, so the user sees the toast text from `src/locales/en/translation.json:1033`:

> *"Backup restored. A restart is required to finish applying the database."*

This is actively wrong in this scenario: there is no pending-restore file on disk, so `database.ApplyPendingRestore` (`backend/internal/database/pending_restore.go:39-79`) will find nothing to do on the next boot and silently no-op (by design — "No pending file present: no-op"). The operator restarts the server expecting the restore to finish, and it never does — the live database (and the running application in the meantime) is left however `RehydrateLiveDatabase` left it, with **no error surfaced anywhere in the API response**, only a server log line (`logger.Log().WithError(pendingErr).Error(...)`) that no one is watching during a routine restore.

This exact branch (line 295-297, the `pendingErr != nil` path) has **zero test coverage** — nobody has exercised "rehydrate failed AND the pending-restore fallback also failed" and asserted what the caller sees.

**Suggested fix:** When `pendingErr != nil` here, this must not be a "successful restore" outcome. Return a non-nil error from `RestoreBackupSafe` (e.g. wrap `pendingErr` together with `rehydrateErr`) so the handler surfaces a 5xx and the frontend shows a real failure state, not a success toast. Add a test that forces both `RehydrateLiveDatabase` and `writePendingRestoreFile` to fail (e.g. point `DataDir` at a directory removed mid-call, or make it read-only) and assert the function returns an error and/or a result whose `Message` unambiguously says the restore did not complete.

---

### High

#### H1. `RehydrateLiveDatabase`'s per-table swap is not transactional — a mid-loop failure leaves the **live, currently-serving** database in a mixed old/new/empty state

**File:** `backend/internal/services/backup_service.go`, lines 1010–1027 (pre-existing function, first added in `38600d44 feat: add SQLite database corruption guardrails`, well before this PR — flagged here because `RestoreBackupSafe`'s new retry/fallback logic in C1 directly depends on this function's failure behavior, and the compounding is new).

```go
for _, tableName := range currentTables {
    ...
    if err := db.Exec("DELETE FROM " + quotedTable).Error; err != nil {
        return fmt.Errorf("clear table %s: %w", tableName, err)
    }
    if _, exists := restoredTableSet[tableName]; !exists {
        continue
    }
    if err := db.Exec("INSERT INTO " + quotedTable + " SELECT * FROM restore_src." + quotedTable).Error; err != nil {
        return fmt.Errorf("copy table %s: %w", tableName, err)
    }
}
```

Every table is `DELETE`d and then re-`INSERT`ed as **separate autocommit statements** — there is no `db.Transaction(...)` wrapping this loop. If the `INSERT` for table *N* fails (a constraint violation, a disk-full mid-copy, an interrupted connection), tables `1..N-1` already reflect the **restored** backup, table `N` is now **empty** (its `DELETE` succeeded, its `INSERT` did not), and tables after `N` still hold the **pre-restore** data. The function returns an error, `RestoreBackupSafe` marks `rehydrated = false` and falls through to the F3 pending-restore-file path (or, per C1, silently reports success) — but the **live, currently-open `*gorm.DB` handle that the running application is serving requests against right now** is left in this half-truncated state until the process is actually restarted. If the table that fails to repopulate is e.g. `users` or `sessions`, this could mean login stops working, or worse, silently-empty tables that the app treats as "no rows configured" (e.g. an empty `proxy_hosts` table momentarily serving no routes) for the entire window until an operator notices and restarts.

**Suggested fix:** Wrap the whole per-table loop (and the `sqlite_sequence` handling immediately after it) in a single SQLite transaction (`BEGIN IMMEDIATE` / `COMMIT`, or GORM's `db.Transaction(func(tx *gorm.DB) error {...})`) so a mid-loop failure rolls back to the pre-rehydrate state instead of leaving a partially-swapped database live. At minimum, add a test that fails the `INSERT` for a table in the middle of the table list and assert the **live** database afterward still has 100% of its pre-restore data (not a mix).

#### H2. `sanityCheckSQLiteFile`'s actual corruption-detection branch has never been exercised by a test

**File:** `backend/internal/services/backup_restore_safe.go`, lines 469–475. Confirmed via coverage profile: the block `if !strings.EqualFold(strings.TrimSpace(integrity), "ok") { return fmt.Errorf(...) }` has count **0** — every existing test that reaches this function supplies either a healthy database (branch not taken) or a file that fails earlier (bad zip, wrong tables), never a file where `PRAGMA integrity_check` itself reports a problem.

**Why it matters:** This is the single check whose entire purpose is "reject a restore if the extracted database is actually corrupt" (spec V6). It has never been proven to work against a genuinely corrupt SQLite file — e.g. one that got bit-flipped in transit to/from a remote storage target, or truncated by an interrupted upload. `PRAGMA integrity_check` can also return **multiple rows** on a badly corrupt file; this code uses `db.QueryRow(...).Scan(&integrity)`, which only reads the first row — that's the documented, correct way to detect "not ok", but since this exact path has 0% coverage, a subtle SQLite-version-specific formatting quirk in that first row would not be caught before shipping.

**Suggested fix:** Add a test that builds a intentionally-corrupted SQLite file (e.g. take a valid one, flip several bytes in a data page, or truncate mid-file) and assert `sanityCheckSQLiteFile` (and end-to-end, `ValidateBackup`/`RestoreBackupSafe`) rejects it with a clear error rather than silently accepting it.

#### H3. No concurrency control around the OAuth-secrets read-modify-write cycle — concurrent token refreshes can lose a token and produce a false "reconnect required"

**Files:** `backend/internal/services/backup_remote_service.go`, `SaveToken` (lines 377–406) and `CompleteOAuth` (lines 705–773).

**Why it matters:** `uploaderFor` (line 350) constructs a brand-new `dropboxUploader`/`googleDriveUploader` — and therefore a brand-new `persistingTokenSource` — on **every** call (`Test`, every scheduled `Upload`, every `pruneRemoteRetention` `List()` call). There is no mutex, no optimistic-concurrency version column, and no `SELECT ... FOR UPDATE`-equivalent guarding the read‑decrypt‑modify‑encrypt‑write of `RemoteStorageTarget.SecretsEncrypted`. If two operations against the *same* target overlap — e.g. a scheduled backup upload is still running when an admin clicks "Test Connection", or a retention-prune `List()` overlaps with an upload — and both happen to trigger a transparent OAuth token refresh (access tokens are typically only good for 1-4 hours, so this is a real if narrow window), both read the same pre-refresh `SecretsEncrypted` blob, both get a fresh access+refresh token pair from the provider, and whichever `SaveToken`/`CompleteOAuth` write lands **last** wins — silently discarding the other's refreshed refresh-token. If the provider rotates refresh tokens on use (single-use rotation is common for OAuth2 providers), the *discarded* refresh token is now invalid at the provider, so the **next** refresh attempt (using the stale, DB-persisted-but-now-dead token) fails with `invalid_grant`, which `persistingTokenSource.Token()` (`remotestorage/oauthtoken.go:84-108`) correctly translates to `ErrOAuthRevoked` — but from the operator's perspective this looks exactly like "the user revoked access at Dropbox/Google," forcing a confusing, unnecessary full reconnect, when the actual cause was an internal race with no user action involved.

`CompleteOAuth` additionally does `s.db.Save(target)` (whole-row save, line 769) rather than a targeted column update — if any other field of the same target row (name, enabled, non-OAuth config) was changed by a concurrent request between this function's `Get()` and `Save()`, that concurrent change is silently clobbered back to its stale value too.

**Suggested fix:** Serialize secret mutations per target (a per-target mutex, or a DB-level `UPDATE ... WHERE secrets_encrypted = <expected-old-value>` / row-version check that fails loudly on conflict rather than overwriting). At minimum, add a regression test that runs two concurrent `SaveToken` calls against the same target and asserts the final persisted state matches one of the two writes deterministically (not silent data loss), and change `CompleteOAuth` to update the specific columns it changed instead of the whole row.

---

### Medium

#### M1. Google Drive's resumable-upload flow trusts a server-supplied URL as the destination of the actual file PUT, on an HTTP client with **no** SSRF/dial-time protection

**File:** `backend/internal/services/remotestorage/googledrive.go`, `startResumableUpload` (lines 240–275) and `Upload` (lines 192–234).

`newGoogleDriveUploader` (and `newDropboxUploader`) build their `*http.Client` via `NewClient` → `oauth2.NewClient`, which uses the Go **default** `http.Transport` — unlike `s3.go` and `webdav.go`, which both explicitly wire `DialContext` through this package's `dialContext`/`safeDialer` (the SSRF re-validation-at-dial-time layer, `ssrf.go:60-84`). The `qa_report.md` reasoning for why this is fine ("Dropbox/Google Drive correctly have no SSRF check by design — only fixed vendor hostnames are ever dialed, never a user-supplied value") is not quite accurate for Google Drive: `startResumableUpload` takes the `Location` response header verbatim from the Drive API response and then, in `Upload`, issues a `PUT` with the full backup archive body straight to that URL (`http.NewRequestWithContext(ctx, http.MethodPut, sessionURI, f)`), with **no validation that `sessionURI`'s host is still `googleapis.com`** and no dial-time re-check on the client used to send it. In real operation the value always is a `googleapis.com` URL, so this is a defense-in-depth gap rather than an exploitable-today bug — the practical prerequisite is that Google's API response (or DNS/TLS trust to it) is already compromised, e.g. by a corporate TLS-inspecting proxy with a trusted root CA already installed on the host. But it does mean this codepath's actual invariant ("only ever dials fixed vendor hosts") is enforced by nothing in code, only by the real-world behavior of a third party's API — worth closing given the surface here is uploading the customer's entire backup archive (containing the whole database, potentially including still-encrypted secrets) to whatever URL is returned.

**Suggested fix:** Either (a) validate `sessionURI`'s host against an explicit allow-list (`www.googleapis.com` / `*.googleapis.com`) before issuing the PUT, or (b) give the Dropbox/Google Drive `http.Client`s the same `dialContext`-wired `Transport` the other two providers use, so this is enforced structurally rather than by convention. Add a test that points `startResumableUpload`'s mocked response at a `Location` header pointing to `127.0.0.1`/an RFC1918 address and asserts `Upload` rejects it.

#### M2. OAuth token-refresh HTTP calls are not bounded by the caller's context or by the client's `Timeout`

**Files:** `backend/internal/services/remotestorage/oauthtoken.go` (`NewClient`, lines 114–126) and `dropbox.go`/`googledrive.go` (`newDropboxUploader`/`newGoogleDriveUploader`).

`NewClient(ctx, conf, tok, saver)` is called with `context.Background()` (see `dropbox.go:115`, `googledrive.go:109`) and that fixed context is what `conf.TokenSource(ctx, tok)` captures for any future transparent refresh. `golang.org/x/oauth2`'s `Transport.RoundTrip` calls `Source.Token()` to obtain/refresh the token **before** dispatching the actual request, using the `TokenSource`'s own captured context — not the per-call `ctx` passed via `http.NewRequestWithContext(ctx, ...)` for the outer `Upload`/`Test`/`List`/`Delete` call, and not gated by the outer `client.Timeout` (60s) either, since that token-fetch is a distinct internal HTTP round trip. Net effect: if a provider's OAuth token endpoint hangs (network partition, provider incident, or a hostile actor who can influence routing to it), a transparent refresh triggered mid-`Upload`/`Test` can block indefinitely, with neither the caller's own request deadline nor `client.Timeout` able to interrupt it.

**Suggested fix:** Thread the actual per-call `ctx` into the `TokenSource` at call time rather than binding `context.Background()` once at uploader-construction time — e.g. construct the client (or at least re-derive the `TokenSource`) per request using the request's `ctx`, or set `oauth2.HTTPClient` in the context to a client with an explicit `Timeout`. Add a test with a `httptest.Server` token endpoint that sleeps past a short caller-supplied `ctx` deadline and confirm the `Upload`/`Test` call actually returns promptly rather than hanging.

#### M3. Raw third-party API error bodies (up to 4 KB) flow into `gin.H{"error": ...}` and are persisted indefinitely in `RemoteStorageTarget.LastError`

**Files:** `backend/internal/services/remotestorage/dropbox.go` (`dropboxAPIError`, lines 427–430), `googledrive.go` (`googleDriveAPIError`, lines 437–440), `backend/internal/services/backup_remote_service.go` (`recordTestOutcome`, lines 427–441), `backend/internal/api/handlers/backup_remote_handler.go` (`respondRemoteTargetError`'s default case, line 248).

The team clearly thought carefully about *not* leaking raw provider errors in one specific place — the OAuth callback redirect, hardened in commit `63e1bd4a` specifically to strip everything except a fixed sentinel message (`qa_report.md`'s "OAuth callback error redirect" section documents this deliberately). But the parallel "Test Connection" endpoint (`POST .../test`) has no equivalent hardening: `dropboxAPIError`/`googleDriveAPIError` read up to 4096 bytes of the provider's raw response body and fold it straight into the returned `error`; `Test()` → `recordTestOutcome` persists that string verbatim to `target.LastError` in the database, and `respondRemoteTargetError`'s default branch (used for anything that isn't one of the four specific sentinel errors) returns `err.Error()` directly to the frontend via `gin.H{"error": ...}`. `LastError` is then re-served on every subsequent `List`/`Get`/`Create`/`Update` response via `toRemoteTargetResponse` (line 50) and rendered as plain React text in the frontend (no `dangerouslySetInnerHTML`, so no XSS — but it is a long-lived, unredacted, admin-visible copy of whatever a third party's API chose to put in a 4xx/5xx body).

This is admin-gated (not a cross-tenant leak) and the comment in both provider files explicitly documents the "admin-facing only" reasoning, so this is not rated higher — but it's an inconsistency worth resolving deliberately rather than by omission, and it's exactly the class of thing that's easy to forget is a problem until a provider's error body includes something unexpected (some APIs echo back partial request parameters in validation-error bodies).

**Suggested fix:** Either explicitly document (in a spec addendum) that Test-connection errors are intentionally verbatim-forwarded to admins and keep it, or route them through the same "safe sentinel + full detail only in server logs" pattern already used for the OAuth callback. If keeping verbatim forwarding, at least lower the 4 KB cap and confirm no known provider ever echoes request bodies (including the `Dropbox-API-Arg` JSON, which contains the configured folder path — low sensitivity, but worth a conscious decision).

#### M4. `CleanupOldBackups`'s pre-restore-backup exclusion — the one thing standing between scheduled retention and deleting the crash-recovery safety net — has 0% test coverage on its only code path

**File:** `backend/internal/services/backup_service.go`, lines 400–409 (confirmed via coverage profile: the entire `if s.db != nil { ... }` block, lines 401–407, has count **0**).

```go
prunable := backups
if s.db != nil {
    prunable = make([]BackupFile, 0, len(backups))
    for _, b := range backups {
        if s.isPreRestoreBackup(b.Filename) {
            continue
        }
        prunable = append(prunable, b)
    }
}
```

This filter exists specifically so scheduled retention pruning never deletes a `pre_restore`-type safety backup — the same file `RestoreBackupSafe`'s F2 rollback path (`backup_restore_safe.go:259-266`) re-applies if a restore's apply step fails partway. Every test that exercises `CleanupOldBackups` in the current suite apparently does so with `s.db == nil` (the no-op passthrough branch, which is covered) — meaning the actual safety behavior this block exists for has never been proven. A bug here (wrong `Type` string, wrong query, a typo introduced in a future refactor) would not be caught by any existing test, and its failure mode is exactly the kind of thing that "passes today, bites during an actual incident later": a `pre_restore` backup silently expires under normal retention pressure, and the next time a restore's A1 apply step fails partway, F2's `unzipWithSkip(preRestorePath, ...)` fails because the file is simply gone, and `RestoreBackupSafe` logs `"failed to restore previous state after apply failure"` (`backup_restore_safe.go:263`) with the system left in the partially-applied intermediate state — no automatic recovery.

**Suggested fix:** Add a test that seeds `s.db` with a mix of `pre_restore` and normal `BackupRecord`s exceeding `DefaultBackupRetention`, calls `CleanupOldBackups`, and asserts the `pre_restore` entries are never among the deleted set regardless of age/count.

#### M5. `computeEncryptionKeyRequired`'s positive-detection branch (the actual "yes, a key is needed" answer) has never been tested to return `true`

**File:** `backend/internal/services/backup_service.go`, lines 731–748 (confirmed via coverage profile: `if count > 0 { return true }`, lines 743–745, has count **0** across the whole test suite).

This function is the sole source of `BackupManifest.EncryptionKeyRequired`, which is meant to warn an operator restoring a backup onto a different host that DNS-provider/tunnel/remote-storage-target credentials in the archive are ciphertext keyed to the *original* host's `CHARON_ENCRYPTION_KEY` and won't decrypt without it. Every existing test apparently only exercises the "no encrypted rows exist" (`false`) path or the "table doesn't exist yet" (`continue`, also treated as not-required) path. If this positive branch has a latent bug (e.g. a renamed table not reflected here, or the `Count` query itself silently returning 0 due to a soft-delete filter GORM applies by default), an operator moving a backup to a new host with a fresh encryption key would get **no warning** that their remote-storage-target credentials, DNS provider credentials, or tunnel configs came back as undecryptable garbage after restore — a quiet, hard-to-diagnose degradation exactly in the spirit of this feature (remote storage targets are one of the three tables checked here).

**Suggested fix:** Add a test that seeds at least one row in `remote_storage_targets` (or `dns_provider_credentials`/`tunnel_configs`) and asserts `computeEncryptionKeyRequired()` returns `true`, and that `ValidateBackup`'s `EncryptionKeyRequired` field reflects it end-to-end.

---

### Low

#### L1. `WithPermissiveSSRFForTesting` is exported, production-shipped code that globally disables SSRF protection for the whole process

**File:** `backend/internal/services/remotestorage/ssrf.go`, lines 99–107.

```go
func WithPermissiveSSRFForTesting() (restore func()) {
    origHost, origDial := ssrfValidateHost, ssrfValidateDialAddress
    ssrfValidateHost = func(string) error { return nil }
    ssrfValidateDialAddress = func(net.IP) error { return nil }
    return func() {
        ssrfValidateHost = origHost
        ssrfValidateDialAddress = origDial
    }
}
```

This lives in `ssrf.go` (not a `_test.go` file), so it compiles into the production binary and is reachable by any code — present or future — that imports `remotestorage`, not just the two cross-package tests that currently call it (`backup_remote_service_regression_test.go:146,276`). The swap of `ssrfValidateHost`/`ssrfValidateDialAddress` is also **unsynchronized** (plain package-level variable assignment, no mutex) — if it were ever called concurrently with real request handling (or by two tests running in parallel without proper serialization), it would race and could leave SSRF protection disabled process-wide for an unpredictable window. Today's usage is safe (sequential test-only call sites, each restored via its own `defer`), but the blast radius if this is ever miscalled is "every remote-storage SSRF check in the running process is silently a no-op."

**Suggested fix:** Move this behind a build tag (`//go:build testhelpers` or similar) so it cannot ship in a production build, or gate it with an explicit `if !testing.Testing() { panic(...) }` guard (Go 1.21+). At minimum, protect the swap with a mutex so a concurrent call can't race with in-flight requests.

#### L2. `ValidateHostSSRF`'s "host resolved to zero addresses" branch is untested

**File:** `backend/internal/services/remotestorage/ssrf.go`, lines 37–39 (count 0 in the profile). Low practical risk — `net.LookupIP` almost always returns an error rather than an empty slice for a genuinely bad host — but it's one of the specific "which branches are untested" answers the audit brief asked for.

#### L3. Defensive `sql.Open`/`os.Open` error branches throughout the restore path are untested (low risk, consistent pattern)

Several near-unreachable defensive branches share the same shape and risk profile — worth fixing as a batch rather than individually:
- `backend/internal/database/pending_restore.go:85-87` (`sqliteIntegrityCheck`'s `sql.Open` error) and `:109-111` (`markPendingRestoreOutcome`'s `sql.Open` error) — `sql.Open("sqlite3", ...)` essentially never fails synchronously for a well-formed DSN.
- `backend/internal/services/backup_restore_safe.go:462-464` (`sanityCheckSQLiteFile`'s `sql.Open` error), `:425-427` and `:434-439` (`verifyManifestChecksums`'s entry-open/read/close error branches), `:382-384` (`readBackupManifest`'s manifest-entry-open error).
- `backend/internal/services/backup_service.go:671-673`, `:675-677`, `:706-708`, `:711-713`, `:714-716`, `:718-720` (`writeV2Archive`'s file-create, manifest-marshal, manifest-entry-create/write, and `zip.Writer.Close()` error branches), `:793-795`, `:799-801`, `:806-808`, `:813-815` (`addToZipTracked`'s file-close, zip-entry-create, and both `io.Copy` error branches).

None of these look like bugs on inspection, and `createBackupLocked` does correctly clean up a partial zip file if `writeV2Archive` fails (`_ = os.Remove(zipPath)` at line 598) — but "correct-looking and never actually exercised" is precisely the category the user asked this audit to hunt for. Recommend picking the two or three most plausible-to-actually-happen ones (disk-full mid-write during `writeV2Archive`/`addToZipTracked`, and a truncated/corrupt archive entry during `verifyManifestChecksums`) and adding targeted tests; the `sql.Open`/`os.Open`-on-a-clearly-missing-path ones are lower value.

#### L4. `readBackupManifest`'s `defer rc.Close()` inside a `for` loop (gocritic `deferInLoop`)

**File:** `backend/internal/services/backup_restore_safe.go:385` — flagged by full `golangci-lint` (`gocritic`'s `deferInLoop`). Not an actual leak today: the loop returns immediately after the one iteration where the defer is registered (it only reaches the `defer` after finding `manifest.json` and opening it, then returns unconditionally a few lines later), so at most one `defer` is ever queued. Still worth restructuring (move the close out of the loop, or `break` before the defer-heavy code) so a future edit that adds more processing after the loop doesn't accidentally turn this into a real per-iteration leak, and so the linter stops flagging it.

---

## Lint Debt (bucket 2) — full `golangci-lint` run

The Definition-of-Done process only runs `.golangci-fast.yml` (staticcheck + govet + errcheck + ineffassign + unused) as a hard pre-commit gate; `make lint-backend` (the full config: `bodyclose`, `gocritic`, `gosec`, `govet`, `ineffassign`, `staticcheck`, `unused`, `errcheck`) is a "manual, before PR" step per `CLAUDE.md` and was not run against this feature's files as part of the QA pass (the QA report only mentions running the fast subset). Running it now (`golangci-lint run --config .golangci.yml` against `internal/services/...`, `internal/api/handlers/...`, `internal/database/...`) surfaces **56 issues total**; the ones that land inside this feature's new/modified files:

- **`gosec` G117 (marshaled secret-shaped struct), 3 findings, all new to this PR and all unsuppressed:**
  - `backend/internal/services/backup_remote_service.go:224` — `json.Marshal(secrets)` in `Create`
  - `backend/internal/services/backup_remote_service.go:300` — `json.Marshal(*secrets)` in `Update`
  - `backend/internal/services/backup_remote_service.go:390` — `json.Marshal(secrets)` in `SaveToken`
  All three are legitimate-by-design (marshal-then-encrypt, the struct is about to be encrypted before it ever touches disk), but every other genuinely-safe secret-adjacent line elsewhere in these same new files carries an explicit `#nosec G3xx -- reason` comment (e.g. `dropbox.go:144`, `webdav.go:99`, `googledrive.go:198`) — these three are the only marshal-a-secrets-struct call sites in the whole feature with **no** matching annotation, i.e. no auditable record that this was a conscious decision rather than an oversight. Recommend adding `#nosec G117 -- marshaled immediately before Encrypt(), never written to disk in plaintext` to match the codebase's existing convention.
- **`gocritic` `deferInLoop`:** `backend/internal/services/backup_restore_safe.go:385` (see L4 above).
- **`gocritic` `paramTypeCombine` (style only, zero risk):** `dropbox.go:392` (`apiPost(ctx, endpoint string, body any, out any)` → `body, out any`), `googledrive.go:402` (`apiPostJSON` same pattern).

No other linter categories in the full run touch this feature's files; the rest of the 56 findings are pre-existing, in files this PR does not modify (`crowdsec_handler.go`, `proxy_host_handler.go`, `uptime_service.go`, various test files), and out of scope for this review.

`//nolint` annotations: **none exist** in any of the new production files (`oauthtoken.go`, `oauth_state_store.go`, `ssrf.go`, `dropbox.go`, `googledrive.go`, `webdav.go`, `backup_remote_service.go`, `backup_remote_handler.go`) — so there is nothing to evaluate for staleness in that category; the `#nosec` annotations that do exist are all specific and currently justified.

---

## DRY / Refactoring (bucket 3)

Comparing `s3.go`, `sftp.go` (existing), `webdav.go`, `dropbox.go`, `googledrive.go` (new):

1. **Near-identical JSON-request helpers, implemented three times.** `dropboxUploader.apiPost` (`dropbox.go:392-420`) and `googleDriveUploader.apiGet`/`apiPostJSON` (`googledrive.go:378-430`) all do the same five steps — marshal body (if any) → `http.NewRequestWithContext` → set headers → `client.Do` → check status → decode JSON (if `out != nil`) — with only the HTTP method, host, and content-type header differing. Suggest a single shared helper in `remotestorage` (e.g. `doProviderJSONRequest(ctx, client *http.Client, method, url string, body, out any, extraHeaders map[string]string) error`) that both providers call, cutting roughly 60 lines of duplication and centralizing the one place a future bug (e.g. forgetting to check `resp.StatusCode`) would need fixing.
2. **`dropboxAPIError` (`dropbox.go:427-430`) and `googleDriveAPIError` (`googledrive.go:437-440`) are structurally identical** — same signature, same `io.LimitReader(resp.Body, 4096)` cap, same message format, differing only in the `"dropbox"`/`"google_drive"` literal prefix already baked into each call site's `op` string. Trivial to consolidate into one `providerAPIError(op string, resp *http.Response) error` in a shared file (e.g. `oauthtoken.go`, which both already import from).
3. **OAuth-uploader construction boilerplate duplicated between `newDropboxUploader` (`dropbox.go:101-119`) and `newGoogleDriveUploader` (`googledrive.go:95-113`).** Both do: validate a non-secret client-ID-shaped field → check `OAuthAccessToken != ""` → build an `*oauth2.Token` from `RemoteTargetSecrets` → build the provider's `oauth2.Config` → call `NewClient` → set `client.Timeout`. Only the config-builder function and the timeout constant differ. A shared `newOAuthUploaderClient(ctx context.Context, conf oauth2.Config, secrets RemoteTargetSecrets, tokenSaver TokenSaver, timeout time.Duration) *http.Client` in `oauthtoken.go` would remove ~10 duplicated lines per provider and become the natural place to fix M2 (thread the real `ctx` through) once, instead of twice.
4. **Minor, not worth forcing:** `RemoteObject{Key, Name, Size, LastModified}` construction is repeated once per provider (`s3.go:134`, `webdav.go:150-155`, `dropboxEntriesToObjects` in `dropbox.go:366-380`, `googleDriveFilesToObjects` in `googledrive.go:356-369`) but each maps a genuinely different source shape, so consolidating the mapping itself isn't valuable — only the target struct literal is shared, which is already about as DRY as it can be.

---

## Restore Reliability (bucket 4) — the user's top concern

Silent-vs-loud characterization for every uncovered region reviewed in `backup_restore_safe.go` and `pending_restore.go`:

| Region | Silent or loud today? | Concern |
|---|---|---|
| `backup_restore_safe.go:295-297` (pending-restore-file write fails after rehydrate already failed) | **Silent** (logged server-side only; API returns `nil` error and a "success" message) | **C1 — the headline finding of this audit.** Restore can completely fail to apply while the user is told it succeeded. |
| `backup_service.go:1010-1027` (`RehydrateLiveDatabase` per-table loop, no transaction) | **Loud on error** (returns an error, which `RestoreBackupSafe` does log/handle) but the **live database is left in a broken half-state** for the window before restart, regardless of how loudly the error is reported | **H1** — the error itself is reported; the *consequence* (mixed-state live DB) is not, and isn't rolled back. |
| `backup_restore_safe.go:469-475` (`sanityCheckSQLiteFile`'s "integrity check failed" branch) | **Loud by design** (returns an error, blocks the restore before anything is mutated — this is exactly F1's intent) but **completely unverified by any test** | **H2** — correct-looking, untested; a latent bug here would only surface the day an operator's backup actually gets corrupted, i.e. the worst possible time to discover it. |
| `backup_service.go:400-409` (`CleanupOldBackups`'s pre_restore exclusion) | **Silent by nature** — if the filter has a bug, the pre-restore safety backup is deleted with no error, no log, nothing; you find out only when you later need it and F2 fails | **M4** |
| `backup_service.go:731-748` (`computeEncryptionKeyRequired`) | **Silent** — a missed "you'll need the same encryption key" warning has no error, just an absent warning banner | **M5** |
| `pending_restore.go:85-87`, `:109-111` (`sql.Open` defensive branches) | **Loud** where it matters (`sqliteIntegrityCheck`'s failure correctly blocks the swap); the `markPendingRestoreOutcome` one is explicitly documented as best-effort/swallowed by design, which is the correct call for a UX-only status field | **L3** — low risk, both branches are essentially unreachable in practice. |
| `backup_restore_safe.go:68-69` (`cleanup()`'s empty-path skip), `:37-39` in `ssrf.go` | **N/A** — defensive guards against states that can't currently arise from this code's own call sites | **L2/L3** — housekeeping only. |

Overall: the *validation* side of the restore pipeline (V1–V6, `validateBackupArchive`) is conservative and fails loudly almost everywhere it matters — that part earns the prior QA report's confidence. The **gap is specifically in the "what happens after validation, while actually swapping the live database" step** (A2/F2/F3 in the spec's own naming) — that's where C1 and H1 live, and it's the part most likely to only be exercised for real during an actual production incident, which is exactly the scenario the user asked this audit to stress-test for.
