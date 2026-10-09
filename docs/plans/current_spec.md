# Plan: CrowdSec preset-apply and data/hub persistence hardening (follow-ups of PR #1513)

Type: ONE branch, ONE PR into `development`: `fix/crowdsec-preset-apply-hardening`. Medium/large fix (touches Go backend, React frontend, Dockerfile, entrypoint, E2E, docs), so per CLAUDE.md it gets a dedicated branch and PR, never a direct commit to `development`.
Closes: #1514, #1515, #1516, #1517, #1518, #1523, #1524.
Status: rev 2 (addresses Supervisor CHANGES REQUESTED: 7 blocking items), open questions Q1-Q5 resolved by the user.
Predecessor: the #1513 plan (curated preset apply) that previously lived in this file; it merged as `3a76804b`.

## 1. Introduction

PR #1513 made curated CrowdSec presets server-authoritative. While verifying it in a rebuilt container, seven follow-ups were filed. They share one theme: **the CrowdSec preset/hub apply path and the persistent CrowdSec directory (`/app/data/crowdsec`) are not safe or durable**. Backups are taken by renaming the live directory, a generic file-write endpoint also renames the live directory, hub data files live on an ephemeral path, the bundled binary does not know its own version, and the tests that should guard this are tolerant skips.

Goal: after this PR, applying/editing CrowdSec config can never destroy the live config, hub data files survive container recreation, `cscli version` is truthful, backups are bounded, and E2E/unit tests fail when any of this regresses.

## 2. Research Findings (root cause per issue, CLAUDE.md "Context First")

Key facts established from code (all paths verified in this repo):

- `crowdsecDataDir = cfg.Security.CrowdSecConfigDir` (`backend/internal/api/routes/routes.go:1030`), set to `/app/data/crowdsec` by `Dockerfile:1142` (`CHARON_CROWDSEC_CONFIG_DIR`). The same directory is `CS_PERSIST_DIR` in `.docker/docker-entrypoint.sh`. It therefore contains the **whole persistent CrowdSec tree**: `config/` (the target of the runtime symlink `/etc/crowdsec -> /app/data/crowdsec/config`), `data/` (LAPI db), `hub_cache/`, `bouncer_key`.
- That same path is `HubService.DataDir` and `CrowdsecHandler.DataDir` (`crowdsec_handler.go:374-397`). Every "backup the DataDir" in the code is a backup directory next to it, `/app/data/crowdsec.backup.<ts>`.

### 2.1 #1514 applyPresetLocally writes unvalidated client content (root cause is wider than the issue says)

- Entry: `frontend/src/pages/CrowdSecConfig.tsx:429` `applyPresetLocally`; reached only when apply returns HTTP 501 for a non-curated (hub) preset (line ~508). It writes `presetPreview` (text from the pull preview, held in client state) to `selectedPath ?? files[0]` through `writeCrowdsecFile` (`frontend/src/api/crowdsec.ts`).
- Confirmed: the backend **never returns 501** for apply any more (grep of `crowdsec_handler.go` shows no `StatusNotImplemented`). The fallback is dead code in practice and can only be triggered by a proxy/misconfig, which makes removing it risk-free.
- Transformation/persistence: `POST /admin/crowdsec/file` -> `CrowdsecHandler.WriteFile` (`crowdsec_handler.go:950`, registered under `managementAdmin`, so admin-only). Verified defects in the handler, independent of the frontend:
  1. **Destructive backup.** It `os.Rename(h.DataDir, backupDir)` (moves the *entire* persistent CrowdSec tree away, config symlink target included), then `MkdirAll(filepath.Dir(p))` and writes just the one file. Everything else (config.yaml, hub, bouncer key, data) is left only in the backup dir. The live tree becomes a single file. There is no restore. This is the same defect class as #1515 and is a medium-severity data-loss bug, not just "unvalidated content".
  2. **Path validation and input validation are weaker than they should be** in `WriteFile` and `ReadFile`, and there is no content or size validation. Specifics are not spelled out in this document; the fix is described at policy level in 3.2.
- Exit: `{status:"written", backup}`; the UI toasts "File saved" / "Preset applied locally".
- Severity/impact: low-medium, authenticated admin only (not an unauthenticated vulnerability). See section 4.

### 2.2 #1515 HubService.Apply renames the whole data dir (CONFIRMED in code; end-to-end not executed)

- Entry: `ApplyPreset` -> `Hub.Apply` for non-curated presets (`hub_sync.go:602`).
- Transformation: `backupExisting` first tries `os.Rename(DataDir, <DataDir>.backup.<ts>)`; only if rename fails (EBUSY on a mount point, cross-device) does it fall back to `copyDir`. Then `runCSCLI` (`cscli hub update` + `cscli hub install <slug>`).
- Why it is wrong, confirmed by reading the code and by the behavior seen in a throwaway container (below): after the rename `/etc/crowdsec -> /app/data/crowdsec/config` dangles, so `cscli` cannot read `config.yaml` (observed error: `open /etc/crowdsec/config.yaml: no such file or directory`). The cscli path therefore fails, the code falls to `extractTarGz` into a freshly created `DataDir`, and on success the live dir holds only the extracted archive while config, hub, `bouncer_key`, LAPI db and `hub_cache` sit in the backup. CrowdSec is running while its db directory is moved. Rollback (`rollback`) does `RemoveAll(DataDir)` then renames the backup back, so it is also only valid for that directory and cannot undo anything `cscli` wrote outside it.
- Not confirmed: that a *real* hub-preset apply on a running container hits exactly this sequence (needs a hub preset pull + a running agent). The verification step is part of the gate in Commit 3. Whether the extract fallback ever "succeeds" with a usable tree is irrelevant; the rename itself is the defect.
- Fix direction (matches the issue): reuse the copy-based, symlink-preserving, db-excluding `backupCopy`/`restoreCopy` added for `ApplyCurated`.

### 2.3 #1516 backup directories accumulate (CONFIRMED)

Creators of `<DataDir>.backup.*`: `HubService.Apply` (rename, second-resolution timestamp), `ApplyCurated` (copy, microsecond timestamp), `CrowdsecHandler.ImportConfig` (`crowdsec_handler.go:696`, rename), `WriteFile` (rename). Nothing ever deletes any of them. grep for prune/retention in `backend/internal/crowdsec` returns nothing. Because the curated backup is a full copy of the persistent tree (minus the db) and the legacy ones are the whole tree, growth is unbounded on a volume the user sizes for a home server.

### 2.4 #1517 handler file size (CONFIRMED)

`crowdsec_handler.go` is 2,906 lines. Preset handlers live at lines ~990-1325 (`ListPresets`, `PullPreset`, `ApplyPreset`, `applyCuratedPreset`, `curatedPresetPreview`, `recordPresetEvent`, `applyFailureBody`, `respondApplySuccess`, `GetCachedPreset` at ~1426). Pure move, same package.

### 2.5 #1518 tolerant Playwright tests (CONFIRMED)

`tests/security/crowdsec-config.spec.ts` "Preset Management" block (lines ~73-145) and "Configuration Files" block are `isVisible().catch(() => false)` guarded and annotate "feature may not be implemented"; they cannot fail on a regression. `tests/security/crowdsec-preset-apply.spec.ts` (from #1513) is the model: route-stubs `**/api/v1/admin/crowdsec/presets/apply` and the file-write route and asserts toasts and write counts. Missing deterministic coverage: hub (non-curated) preset apply success/failure surfaced, no client-side write on any apply failure, file editor save/failure, file-list error cases.

### 2.6 #1523 hub data files missing after container recreation (ROOT CAUSE CONFIRMED by experiment)

- Config: CrowdSec's default `config_paths.data_dir` is `/var/lib/crowdsec/data/` and `hub_dir` is `/etc/crowdsec/hub/` (upstream `config/config.yaml`). The entrypoint (`.docker/docker-entrypoint.sh:310-319`) redirects **only** `db_path` to the volume. `Dockerfile:1089` creates `/var/lib/crowdsec/data` inside the image, so on a recreated container it is an empty directory (confirmed: the issue shows only `trace/`). `/etc/crowdsec` is a symlink into the volume so the **hub items persist but their data files do not**.
- `configs/crowdsec/install_hub_items.sh` skips items for which `cscli <type> inspect <name>` exits 0 ("already installed, skipping").
- Experiment (throwaway `charon:local` container run with `--entrypoint sh`, no volumes, no live host touched):
  - Install `crowdsecurity/http-backdoors-attempts` -> `backdoors.txt` appears in `data_dir`.
  - Wipe `data_dir` (simulating recreate), keep the hub: `cscli scenarios install <name>` -> "Nothing to install or remove", data dir stays empty. `cscli scenarios install --force <name>` -> same, still empty. **`cscli hub upgrade` repopulates `backdoors.txt`, and so does `cscli hub upgrade --force`.** The issue's suggestion "`install --force` or `hub upgrade`" is therefore half right: only `hub upgrade` works.
- **Additional defect found (CONFIRMED, not in the issue):** on a hub with an index but nothing installed, `cscli scenarios inspect crowdsecurity/http-backdoors-attempts` exits **0** and prints `installed:false` metadata. `install_if_missing` uses the exit code, so on a fresh volume it may print "already installed, skipping" and install nothing. (In the experiment inspect exited 1 when the item was installed but its data file was missing, so the exit code is not a reliable installed/absent signal in either direction.) #1513's `ApplyCurated` already uses the correct signal: `inspect -o json` -> top-level `installed`.
- Not confirmed: whether the missing-install path is masked in real startups by something else (e.g. the collections installing their children). Needs the recreate test in Commit 6 on a clean volume.

### 2.7 #1524 empty version (ROOT CAUSE CONFIRMED; the fix has a gotcha)

- Confirmed: the injected symbol is wrong. In CrowdSec v1.8.1, `pkg/cwversion/version.go` declares only `Codename` and `Libre2` (and no `Version`); the version comes from `github.com/crowdsecurity/go-cs-lib/version` (`Version`, `BuildDate`, `Tag`, `System`), pinned at `go-cs-lib v0.0.25` by CrowdSec v1.8.1's `go.mod`. Upstream's own Makefile uses `-X 'github.com/crowdsecurity/go-cs-lib/version.Version=$(BUILD_VERSION)'` (plus `BuildDate`, `Tag`, and `cwversion.Codename`). `-X` on a nonexistent symbol is silently ignored, so both `Dockerfile:915` and `:923` are no-ops. Observed in the built image: `cscli version` prints empty `version:`, `Codename:`, `BuildDate:`, and `cscli` logs `Crowdsec version is not set, using hub branch 'master'`.
- The existing app-stage sanity check (`Dockerfile` ~L1054) says "CrowdSec 1.8.x prints an empty `version:` field here regardless of the -X ldflag". That comment is wrong and hides the bug; it must be corrected.
- **Gotcha, verified in `cmd/crowdsec-cli/core/require/branch.go` (v1.8.1):** with a real version cscli calls `https://version.crowdsec.net/latest` on every hub command and returns a hard error if it cannot reach it; if the running version equals latest it uses `master` (identical to today), if it is older than latest it uses the branch named after the version. `https://hub-cdn.crowdsec.net/v1.8.1/.index.json` returns **404** today (v1.8.0, v1.7.8 and master return 200). So a correct version could (a) make `cscli hub *` fail offline/air-gapped and (b) break hub updates once 1.8.1 is no longer the latest and the CDN has no `v1.8.1` branch. The issue's "hub items may be newer than the engine supports" claim is also overstated: while 1.8.1 is the latest release the behavior is already `master`. The fix therefore needs an explicit hub-branch decision (see D1) rather than a bare `-X` correction.
- The toolchain delivery path: the default Dockerfile build consumes the prebuilt `ghcr.io/wikid82/charon-toolchain` image (content-hash tag `CHARON_TOOLCHAIN_TAG`, `Dockerfile:22`). Editing the `crowdsec-inline` stage changes the key, `.github/workflows/toolchain-image.yml` builds the new image and its bot commits the new TAG/DIGEST onto the PR head branch (workflow lines ~365-420) so `verify-toolchain-pin` goes green. Until that happens the app image still contains the old binary, so the app-side assertion cannot be added in the same commit as the ldflags change.

### 2.8 Reproduction/evidence commands run for this plan (all foreground, repo-local or throwaway container)

`gh issue view` for each issue, `gh pr view 1513`; code reads listed above; `curl` of upstream v1.8.1 source files and `hub-cdn.crowdsec.net/<branch>/.index.json`; shallow clone of upstream v1.8.1 into the session scratchpad; throwaway `docker run --rm --entrypoint sh charon:local` experiments (data-dir wipe, inspect exit codes, `hub upgrade`). Nothing on live hosts or volumes was touched.

## 3. Technical Specifications

### 3.1 Shared backup helper, Apply flow, ImportConfig (#1515, #1516)

New file `backend/internal/crowdsec/backup.go` (move `backupCopy`, `restoreCopy`, `isLiveDBFile`, `emptyDirExcept` out of `curated_apply.go`; the move itself changes no behavior):

- **Two namespaces, two caps, so they cannot evict each other (blocking item 3).**
  - Full-tree snapshots: backup dirs next to `DataDir`: `<DataDir>.backup.<ts>`, `const DefaultBackupRetention = 5`.
  - Single-file edit backups (from `WriteFile`, section 3.2): backup dirs next to `DataDir`: `<DataDir>.filebackup.<ts>/<rel>`, `const DefaultFileBackupRetention = 10`. The `<base>.backup.*` glob never matches `<base>.filebackup.*`.
- **Snapshot exclusions (blocking item 4).** `snapshot` copies `DataDir` (symlinks preserved) but **excludes**: the live `crowdsec.db`, `-wal`, `-shm` (anywhere), and the top-level `data/` directory (LAPI db plus hub data files and the GeoLite2 mmdb, which after #1523 live there; hub data files are recoverable with the gated `cscli hub upgrade`, the db is owned by the running engine) and the top-level `hub_cache/` (regenerable download cache). `restore` empties `DataDir` except those same excluded entries and copies the snapshot back, so it never touches `data/` or `hub_cache/`. Only the *top-level* `data/` and `hub_cache/` are excluded; a nested directory with the same name inside `config/` is still backed up. Result: a snapshot is essentially `config/` plus `bouncer_key` (small), x5 retained.
- `func (s *HubService) snapshot() (path string, err error)`: microsecond timestamp name, created `0o700` with exclusive `os.Mkdir`.
- `func (s *HubService) restore(path string) error`.
- `func PruneBackups(dataDir, kind string, keep int) (removed []string, err error)` where `kind` is `backup` or `filebackup`. Lists siblings matching `<base>.<kind>.*`, `Lstat`-checks each is a real directory (never follows or deletes symlinks or plain files), **parses the timestamp from the directory name** (both the legacy `20060102-150405` and the microsecond `20060102-150405.000000` layouts; names that do not parse are ignored, never deleted), sorts newest first, deletes everything past `keep`. `keep < 1` is clamped to 1 so the newest is always kept. Never touches `dataDir`. Single-removal errors are logged and aggregated, never failing the caller.
- **Prune runs after every attempt, successful or failed**, always preserving the newest `keep` snapshots. A failed apply's own snapshot is the newest, so it is always retained for manual recovery.

**`HubService.Apply` new control flow (blocking item 2).** Hold `HubService.mu` and the handler-level lock (section 6).
1. Validate slug; load cache meta (non-fatal).
2. `snapshot()`; any error -> return failure, nothing changed. `BackupPath` is set only after success. The old "read the archive into memory before the backup because the cache lives in `DataDir`" workaround is **removed**: nothing is moved any more, and `hub_cache/` is excluded from snapshots and from restore, so the archive stays valid for the whole call.
3. If `cscli` is available: `cscli hub update` (non-fatal), `cscli hub install <slug>`. Success -> status `applied`, `UsedCSCLI=true`, reload, prune, return.
4. **A cscli failure still falls back to the cached-archive extract** (this keeps the offline/hub-unreachable behavior that exists today). Before extracting, `restore(snapshot)` so the extract starts from a clean pre-apply tree and any partial files `cscli` wrote are removed (this closes the partial-overlay risk of overlaying an archive on a half-installed state).
5. Extract `extractTarGz` onto `DataDir` (cache refresh from the hub if the cache is missing, as today). **Residual risk, documented:** extraction overlays archive files on top of the live tree rather than replacing it, so files in the archive replace same-named live files; mitigation is the snapshot. Any extract/refresh failure -> `restore(snapshot)` and return the error with `BackupPath`.
6. Success -> status `applied`, `UsedCSCLI=false`, reload, prune.
`backupExisting`, `rollback`, `emptyDir`, and `copyDir` (if now unused) are deleted.

**`CrowdsecHandler.ImportConfig` (blocking item 6): converted to snapshot/restore semantics, never renaming `DataDir`.** Flow: handler lock -> `snapshot()` -> empty `DataDir` except the snapshot-excluded entries (so the live db, `data/`, `hub_cache/` survive the import) -> extract the uploaded archive, **skipping any archive entry that is a live db file (`isLiveDBFile`) or lies under the top-level `data/` or `hub_cache/`** so an upload can never overwrite engine-owned state -> on any failure `restore(snapshot)` and return the error -> prune. Net behavior difference: the live db and `data/` are no longer moved or lost by an import; everything else is replaced exactly as before. Existing `ImportConfig` tests are updated, plus the new tests listed in Commit 3.

### 3.2 File endpoints hardening (#1514)

Details of the validation defects and their fixes are intentionally not written here (see section 4). Public-safe design:

- New file `backend/internal/api/handlers/crowdsec_files.go` with a single shared path-resolution helper used by `ReadFile`, `WriteFile` and `ListFiles`, so all three apply one policy ("harden file-endpoint path validation").
- Write policy: bounded body size (1 MiB); allowed file types (Q3); YAML files must parse; protected files and trees (secrets, databases, caches, engine data) are never writable; `.yaml/.yml` parsing uses `gopkg.in/yaml.v3` if already a dependency (verify).
- **Symlink policy (blocking item 7):** the path that is written is the lexical path inside `DataDir`, and `WriteFile` refuses (HTTP 400) to write through or replace a symlink: if the target or any existing component under `DataDir` is a symlink, the request is rejected. Symlinked hub items (the `scenarios`/`parsers`/... links into `hub/`) are therefore read-only from the editor. **`ReadFile` still enforces containment after symlink resolution**: symlinked hub items that resolve inside `DataDir` stay readable, a link resolving outside `DataDir` is refused. Test in Commit 4.
- **Two predicates, not one.** The viewer today lists and reads any file under `DataDir` (hub items, `config.yaml`, etc.), and filtering the list by the write predicate would hide files users can currently read. So: `isReadable(rel)` governs `ListFiles` and `ReadFile` (secrets and databases such as `bouncer_key`, `*.db*`, and the `hub_cache/` and `data/` trees are neither listed nor readable; everything else stays visible, including symlinked hub items), and the stricter `isWritable(rel)` governs `WriteFile`. A file that is readable but not writable (symlinked hub items, non-allowlisted types) stays viewable and its save returns the server's 400 message, which the existing `writeMutation` error path already shows; no frontend contract change. `ExportConfig` is unaffected. Commit 4 tests: `ReadFile` refuses `bouncer_key` and db files, reads a symlinked hub item that resolves inside `DataDir`, and refuses a symlink resolving outside it; `ListFiles` omits them but still lists a symlinked hub item and `config.yaml`; every file `WriteFile` accepts is also listed.
- `WriteFile` algorithm: lock -> validate -> if the target exists, copy just that file to `<DataDir>.filebackup.<ts>/<rel>` -> atomic write (temp file in the same directory, `fsync`, `rename`, preserve mode, default `0o600`) -> `PruneBackups(kind=filebackup, 10)` -> 200 `{status:"written", backup}`. `DataDir` is never renamed.
- Error contract (JSON `{"error": ...}`): 400 invalid request / path / type / YAML / symlink, 413 too large, 500 write failure.
- No new endpoints, no schema changes, no frontend contract changes.

### 3.3 Frontend (#1514)

`frontend/src/pages/CrowdSecConfig.tsx`: delete `applyPresetLocally`, the `501` branch in `handleApplyPreset`, the `applied-locally` apply-info status, and anything left unused (`backupMutation` if only used there, imports, `presetPreview` write path). Non-2xx apply for hub presets is surfaced like curated: server error text, backup path if present, no write. The generic file editor (`writeMutation`) is unchanged but now shows server 400/413 messages. Update Vitest files: `CrowdSecConfig.test.tsx`, `.coverage.test.tsx`, `.spec.tsx`, `.curatedApply.test.tsx` (remove tests that assert the local fallback; add "501 for a hub preset shows an error and performs no file write").

### 3.4 Persistent CrowdSec data dir and hub data (#1523)

`.docker/docker-entrypoint.sh` (next to the existing `db_path` redirect, ~L310-320):

- `sed` `data_dir: /var/lib/crowdsec/data/` -> `data_dir: ${CS_DATA_DIR}/` in `$CS_CONFIG_DIR/config.yaml` (idempotent; works for both fresh and already-initialized volumes), with the same verify/echo pattern as the db redirect. `CS_DATA_DIR` is `/app/data/crowdsec/data` and is already `mkdir`ed and chowned.
- **Gated, never unconditional (blocking item 5).** After `install_hub_items.sh`, the entrypoint runs `cscli hub upgrade` **only if** at least one installed item's data files are missing. Detection: for each installed item (`cscli <type> list -o json`), check that every file the item declares is present under `data_dir`; the exact JSON field for declared data files is confirmed by the implementer against `cscli <type> inspect <name> -o json` before coding (open implementation detail, verified in the Commit 6 gate). Fallback if the field is not exposed: treat an empty `data_dir` (ignoring `trace/` and the db) with at least one installed item as "missing". The call is bounded (`timeout 120s`), failure is a logged warning and never fatal, and it never uses `--force`. A normal restart with intact data files performs no upgrade, so items are never silently upgraded on a healthy start. Note `hub upgrade` upgrades every installed item; this is acceptable only in the recovery case it is gated on, and the log line must say so.
- It is skipped in the same condition as the rest of hub init (`CHARON_SECURITY_TESTS_ENABLED=false`).

`configs/crowdsec/install_hub_items.sh`: change `install_if_missing` to decide on `cscli <type> inspect <name> -o json` reporting `"installed": true`, matching `ApplyCurated.verifyItem`. **Verified:** `jq` is not in the `charon:local` image, so use `grep -q '"installed": *true'` (not jq). **Verified:** `inspect -o json` still prints full JSON and exits 0 when an item is installed but its data file is missing, so the JSON signal is reliable in exactly the recreate scenario (the plain-text `inspect` exit code was not). Keep the "failed to install" warnings non-fatal.

`Dockerfile`: keep creating `/var/lib/crowdsec/data` (a stale `data_dir` on an un-migrated config must still exist); no other change.
`.docker/compose/docker-compose.playwright-ci.yml:137` currently mounts a volume at `/var/lib/crowdsec/data`; repoint or remove it so E2E exercises the same persistent path as production. `docs/security.md` hardened-deployment text that tmpfs-mounts `/var/lib/crowdsec` stays valid (the db and data now live under `/app/data/crowdsec`) but its description must be updated.
`scripts/diagnose-crowdsec.sh` default `DATA_DIR` updated to the new location.

Optional passive health signal (Q4 resolved: log line only, no UI): log a startup summary line listing installed-but-missing data files. No banner or badge (Passive UI, no nudges).

### 3.5 Version injection (#1524)

`Dockerfile` `crowdsec-inline` stage (both `xx-go build` commands, ~L915 and ~L923): replace the ldflags with `-s -w -X github.com/crowdsecurity/go-cs-lib/version.Version=v${CROWDSEC_VERSION} -X github.com/crowdsecurity/go-cs-lib/version.BuildDate=<reproducible date or empty>`. Do not set `Tag` (empty `Tag` leaves `version.String()` equal to `Version`). Do not add `BuildDate` from the wall clock: that would break the content-addressed, reproducible toolchain key; prefer the release date derived from the pinned tag or omit it.

App-side assertion (second commit, after the toolchain bot has pushed the new TAG/DIGEST to the PR branch): in the existing N5 `RUN` block (~L1054) replace the wrong comment and add `grep -q "^version: v${CROWDSEC_VERSION}\$"` against `cscli version`. The final stage must redeclare `ARG CROWDSEC_VERSION` (currently declared only globally at L41 and in the caddy/crowdsec-inline stages; a global ARG is not visible inside a later stage without redeclaring). The build fails if the symbol ever silently no-ops again.

Hub branch (decision D1, recommended): set `cscli.hub_branch` explicitly in the entrypoint-managed `config.yaml` so behavior is deterministic and does not depend on `version.crowdsec.net` reachability or on `hub-cdn` having a `v1.8.1` branch. Verified in upstream v1.8.1 `chooseBranch`: a non-empty `cscli.hub_branch` is returned immediately, before the `version.crowdsec.net/latest` lookup, so setting it removes both the network dependency and the CDN-404 risk. The online/offline test result is recorded in the PR description (Commit 8 gate).

### 3.6 Handler split (#1517)

Move to `backend/internal/api/handlers/crowdsec_preset_handler.go` (same package, no behavior change, no export changes): `ListPresets`, `PullPreset`, `ApplyPreset`, `applyCuratedPreset`, `curatedPresetPreview`, `recordPresetEvent`, `applyFailureBody`, `respondApplySuccess`, `GetCachedPreset`, and preset-only helpers. Move the matching unit tests into `crowdsec_preset_handler_test.go` files where they are clearly preset-only (leave shared fixtures where they are). `RegisterRoutes` stays in `crowdsec_handler.go`. The new file must be < 700 lines.

## 4. Priorities & Ordering (CLAUDE.md "Findings Triage")

None of the seven issues is user-submitted (all filed by the maintainer from PR #1513). Rules applied: bug fixes before new implementation (there is no `feat` here); critical/high ships as its own PR first; medium/low is scheduled after.

Severity after root-causing (some differ from the issue labels):

| Rank | Issue | Severity | Why |
|------|-------|----------|-----|
| 1 | #1523 data files missing after recreate | **medium** (issue says medium) | Silent degraded detection and no GeoIP on every recreated container; install guard defect makes fresh volumes suspect. Highest user impact. |
| 2 | #1514 + `WriteFile` destructive rename | **medium** (raised from low-medium) | Admin action can wipe the live CrowdSec tree; weak input validation. Security-sensitive (below). |
| 3 | #1515 `HubService.Apply` rename | **low-medium, confirmed** | Same defect class and same helper as #1514/#1516; fixed together. |
| 4 | #1524 empty version / hub branch | **low-medium** | Fix is easy but changes runtime hub behavior (offline failure, CDN 404 risk); needs the D1 decision and the toolchain bot. Scheduled after the fixes it must not destabilize. |
| 5 | #1516 backup accumulation | low | Piggybacks on the shared helper. |
| 6 | #1518 tolerant E2E | low | Test hardening; first in commit order (as `fixme` specs) and enabled last. |
| 7 | #1517 handler split | low | `refactor`, no behavior change; sequenced first in the *commit* order (mechanical, makes later diffs reviewable) but lowest in *priority*. |

No finding is critical or high, so none must ship as its own PR ahead of this work. Priority above governs attention and what to cut first under contingency; commit order in section 5 is dependency-driven.

**Security handling (#1514).** It is admin-only and already public as an issue. Assessment: low severity (authenticated admin only; the endpoint grants nothing beyond what an admin already has). **Q2 resolved by the user: #1514 is already public, so no private advisory is opened unless root-causing shows real impact beyond what an admin already has; it does not, so the default is NO advisory.** Handling:
- The existing public issue #1514 stays as is. Keep PR text, commit bodies and code comments free of exploit specifics anyway (no value in handing detail to un-upgraded installs).
- Exactly one commit in the PR carries the `(security)` scope, and its subject must stay in the vague category-only form: `fix(security): harden CrowdSec configuration file handling`. It must not mention path handling, symlinks, prefixes, rename, or the file endpoint, and Commit 4's body and the PR description stay equally vague (for example "tighten validation in the CrowdSec configuration file endpoints"). Every commit subject in a feature PR merge-commit appears verbatim in the public changelog, so the other commits use plain scopes (`fix:`, `refactor:`, `test:`, `docs:`, `chore:`) and describe user-visible behavior only. The destructive-rename data-loss fix is a plain `fix:` (it is a reliability bug, not an exploit).

**Out-of-scope findings discovered while planning:** none left open. Items found beyond the issue text are folded in (install guard, wrong N5 comment, `ImportConfig` conversion, playwright-ci compose mount). If implementation finds anything else, file it per the triage rule at that time.

## 5. Branch/PR Decision and Commit Slicing Strategy

### 5.1 Branch/PR decision: ONE branch, ONE PR

Single branch `fix/crowdsec-preset-apply-hardening` -> PR into `development`, titled e.g. `fix: harden CrowdSec preset apply and keep hub data across container recreation`.

Why one PR (cohesion): all seven issues are one defect family, "the CrowdSec persistent tree is mutated unsafely and its hub/data state is not durable", and they share code: #1514, #1515 and #1516 all consume the same new `backup.go` helper (a single fix for rename-based backups, the file endpoint, and pruning); #1517 is the file those changes land in; #1518 is the test net for #1514/#1515; #1523 and #1524 are the same persistent-dir/hub-state story at the container layer and are both verified by the same rebuilt-container gate. Splitting would leave the helper without its callers, the E2E net without the behavior, or a half-fixed persistence story.

Considered splits and why rejected:
- Docker/entrypoint (#1523, #1524) vs Go/frontend: they can build independently, but they do not conflict, share a single verification environment (rebuilt `charon:local`), and splitting would double the merge-soak the maintainer applies before `main`. The #1524 toolchain bot is explicitly designed to push the pin bump onto the PR head branch, so it works inside this PR.
- `(security)` fix alone: not required; severity is low (authenticated admin), and the rule "own PR" applies to critical/high only.

**Contingency without splitting:** if Commits 7-8 (#1524) cannot be validated (hub-branch decision unresolved, toolchain build blocked), drop those commits from the PR and leave #1524 open rather than opening a second PR for it; the remaining six issues still form a complete, coherent PR. Rollback of the PR as a whole is a revert of the merge commit; a revert leaves a valid `data_dir` in `config.yaml` (the old image still creates the default directory), so nothing breaks.

### 5.2 Ordered commits (all on the one branch)

Every command run by implementers is foreground/blocking per CLAUDE.md. Each commit must build and pass its gate; the PR as a whole passes the full Definition of Done (section 8).

**Commit 1 - `test: add deterministic E2E specs for CrowdSec preset and file flows (fixme)`** (#1518, step 1 of the suggested sequence)
- Files: new `tests/security/crowdsec-hub-preset-apply.spec.ts`, new `tests/security/crowdsec-file-editor.spec.ts`.
- Content, all stubbed with `page.route` like `crowdsec-preset-apply.spec.ts`, marked `test.fixme` until their feature commits: hub preset apply success shows success only when body `status==='applied'`; hub preset apply 500/503/504 shows an error and **zero** `POST /admin/crowdsec/file` calls; hub preset apply 501 shows an error and zero writes; file editor save success posts exactly one write for exactly the selected path; file editor shows the server message for 400 (disallowed type / invalid YAML) and 413.
- Depends on: none. Gate: `npx playwright test tests/security/crowdsec-hub-preset-apply.spec.ts tests/security/crowdsec-file-editor.spec.ts --project=security-tests` runs and reports the tests as fixme/skipped with no failures; lint/type-check clean.

**Commit 2 - `refactor: move CrowdSec preset handlers into their own file`** (#1517)
- Files: new `backend/internal/api/handlers/crowdsec_preset_handler.go`; edit `crowdsec_handler.go`; move/split preset-only tests into `crowdsec_preset_handler_test.go`.
- Pure move: no signature, route, or behavior changes. Depends on: none.
- Gate: `cd backend && go build ./... && go test ./internal/api/handlers/... -race` all green with the same test count; `git diff --stat -M` shows moves, not rewrites; `make lint-fast`.

**Commit 3 - `fix: keep CrowdSec backups bounded and copy-based for preset apply`** (#1515, #1516)
- Files: new `backend/internal/crowdsec/backup.go` + `backup_test.go`; edit `hub_sync.go` (`Apply` per the flow in 3.1; remove `backupExisting`/`rollback`/`emptyDir`/dead `copyDir`), `curated_apply.go` (move helpers, add prune call), `crowdsec_handler.go` `ImportConfig` (snapshot/restore conversion, handler lock), `hub_pull_apply_test.go`, `device_busy_test.go` (the rename/EBUSY tests are obsolete and are replaced by copy-path tests).
- Depends on: Commit 2 (preset code already in its own file; `ImportConfig` stays in the main handler).
- Unit tests (new, all with `t.TempDir()`): snapshot excludes live db files, top-level `data/` and `hub_cache/`, preserves symlinks, and still copies a nested directory named `data` under `config/`; **size test:** a large file under `data/` and under `hub_cache/` is not copied while a small `config/` file is (assert snapshot size); restore leaves live db, `data/` and `hub_cache/` untouched; `PruneBackups` orders by the timestamp parsed from the name (not mtime; touching an old dir does not save it), handles both name layouts, ignores unparsable names, non-matching names, symlinks and plain files, never deletes `dataDir`, clamps `keep<1` to 1, always keeps the newest, handles a missing parent, and `backup` and `filebackup` namespaces never evict each other; `HubService.Apply` flows: (a) cscli success leaves `DataDir` in place (inode unchanged) and prunes; (b) **cscli failure -> snapshot restored -> cached-archive extract succeeds -> `applied`, `UsedCSCLI=false`, no cscli leftovers**; (c) cscli failure and extract failure -> snapshot restored, error returned with `BackupPath`; (d) cscli failure with cache miss and hub refresh failure -> restored; (e) prune also runs after a failed apply and keeps the failed apply's snapshot; `ImportConfig`: success replaces config but preserves live db/`data/`/`hub_cache/` and never renames `DataDir`; extract failure restores the previous config; prune runs; concurrent `ImportConfig` + apply serialize on the handler lock; an uploaded archive containing `crowdsec.db`, `data/x` and `hub_cache/y` entries extracts none of them.
- Gate: `go test ./internal/crowdsec/... ./internal/api/handlers/... -race`; package coverage not below the current level; **rebuilt-container verification of #1515**: pull + apply one hub preset against a running agent, confirm `/app/data/crowdsec` is never renamed (inode unchanged), the apply result is correct, and at most 5 full-snapshot dirs remain after 7 applies (this also settles the "not confirmed" item in 2.2).

**Commit 4 - `fix(security): harden CrowdSec configuration file handling`** (#1514 backend; subject intentionally vague, see section 4)
- Files: new `backend/internal/api/handlers/crowdsec_files.go` + `crowdsec_files_test.go`; edit `crowdsec_handler.go` (`ListFiles`, `ReadFile`, `WriteFile` delegate to the shared helper); update existing `TestCrowdsec_WriteFile_*` tests in `crowdsec_handler_coverage_test.go`.
- Depends on: Commit 3 (per-file backups use the `filebackup` namespace and handler lock). Commit body and PR text stay vague (section 4).
- Unit tests (categories only here): path validation rejection cases, symlink policy (write through or replacement of a symlink is refused, a regular file under `DataDir` is written), type/content/size validation (disallowed type, invalid YAML, oversize -> 413, protected files), atomic write leaves no temp file on failure, `DataDir` and unrelated files untouched after a write (regression test for the destructive rename), `filebackup` contains only the previous version of the target and is capped independently of full snapshots, the read/list predicate and the stricter write predicate behave as in 3.2, `ReadFile` shares the validation.
- Gate: `go test ./internal/api/handlers/... -race`, `make lint-fast`; GORM scan not required (no models); the Playwright file-editor spec enabled in Commit 9 depends on this behavior.

**Commit 5 - `fix: remove the dead local-write fallback from CrowdSec preset apply`** (#1514 frontend)
- Files: `frontend/src/pages/CrowdSecConfig.tsx`, its four test files under `frontend/src/pages/__tests__/`, `frontend/src/api/crowdsec.ts` only if a now-unused export remains.
- Depends on: none technically (the backend never returns 501); ordered after Commit 4 so the whole #1514 story is reviewable in sequence.
- Gate: `cd frontend && npm run type-check && npx vitest run src/pages/__tests__/CrowdSecConfig*` green; no unused-export/lint findings; `npm run build`.

**Commit 6 - `fix: persist CrowdSec hub data files across container recreation`** (#1523)
- Files: `.docker/docker-entrypoint.sh`, `configs/crowdsec/install_hub_items.sh`, `.docker/compose/docker-compose.playwright-ci.yml`, `scripts/diagnose-crowdsec.sh`, `docs/security.md` (wording), new integration test `scripts/crowdsec_data_persistence_test.sh` (follows the pattern of `scripts/crowdsec_startup_test.sh`).
- Depends on: none (independent of the Go commits; ordered after so a failing container gate does not block the Go work from being reviewed).
- Tests: shell-level test (start container with a named volume, wait for hub init, assert data files and `GeoLite2-City.mmdb` exist under `/app/data/crowdsec/data`, `docker rm` and recreate with the same volume, assert files are present and `crowdsec.log` has no `unable to init data for file` / `unable to initialize GeoIP` lines after enabling CrowdSec; also run it on a volume initialized by the old image to prove the sed migration). Add `shellcheck` cleanliness for changed scripts.
- Gate: rebuild `charon:local` (skill `docker-rebuild-e2e` / `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`), run `scripts/crowdsec_data_persistence_test.sh` and `scripts/crowdsec_startup_test.sh`; the fresh-volume case must show the install guard now really installing.
- Additional tests: the gate must prove `hub upgrade` is **not** run on a restart with intact data files (log assertion) and **is** run (once, bounded) when data files are missing; `shellcheck` is a required gate for every changed or new shell script (`.docker/docker-entrypoint.sh`, `install_hub_items.sh`, the new test script).
- CI wiring and ignore files: wire `scripts/crowdsec_data_persistence_test.sh` into the existing CrowdSec integration job via a skill wrapper like `.github/skills/integration-test-crowdsec-startup-scripts` and the `integration-tests.yml` workflow (rather than a new workflow). Check `.dockerignore` (it already excludes `scripts/tests/` and the toolchain scripts; the new script must be excluded from the image the same way if it lives under `scripts/`), `.gitignore`, and `.codecov.yml` (shell scripts are not coverage-measured; confirm no new exclusion needed).
- Snapshot interaction (item 4 sequencing): Commit 3's snapshot already excludes top-level `data/` and `hub_cache/`, so Commit 6's relocation of `data_dir` into `/app/data/crowdsec/data` does not enlarge any backup; the Commit 3 size test is the regression guard.

**Commit 7 - `fix: report the real CrowdSec version in the bundled binaries`** (#1524, step A: ldflags)
- Files: `Dockerfile` (`crowdsec-inline`, both `xx-go build` lines). Bot-authored follow-up commit on the branch updates `CHARON_TOOLCHAIN_TAG` / `CHARON_TOOLCHAIN_DIGEST` (do not hand-edit those lines; the workflow owns them).
- Depends on: none. **Wait here until the toolchain-image workflow has built and pushed the pin bump to the PR branch** (this is the only asynchronous step in the plan).
- Gate: `toolchain-image.yml` green on the PR (build + Trivy gate), `verify-toolchain-pin` green, and a local `make build-offline`-style build (`--build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline`) shows `cscli version` -> `version: v1.8.1`.
- Expected CI state: after this commit `verify-toolchain-pin` (and any check that compares TAG/DIGEST to the recipe key) is **red until the bot pushes the new pin**. Implementers must `git pull` the bot's commit before starting Commit 8 and must not hand-edit the two ARG lines.

**Commit 8 - `fix: fail the image build if CrowdSec reports no version and make the hub branch deterministic`** (#1524, step B)
- Files: `Dockerfile` (N5 block: correct the comment, add the version assertion, add `ARG CROWDSEC_VERSION` to the final stage), `.docker/docker-entrypoint.sh` (hub branch setting per D1).
- Depends on: Commit 7 and the bot pin bump.
- Tests / build-time assertion: the Dockerfile `RUN` fails with a clear message if `cscli version` does not print `version: v${CROWDSEC_VERSION}`. Prove it fails by temporarily building with a mismatched `--build-arg CROWDSEC_VERSION` against the pinned toolchain (expected hard failure), then with the correct value (pass). Runtime check in the rebuilt container: `cscli version` truthful; `cscli hub update` succeeds online; with the network blocked (`--network none`) hub commands behave as the D1 decision specifies (no hard failure when `hub_branch` is set).
- Gate: image builds; `scripts/crowdsec_startup_test.sh` green; both stderr lines `Crowdsec version is not set` and `unable to retrieve latest crowdsec version` absent from `docker logs` in the online and offline runs.
- Record in the PR description: `cscli.hub_branch` is honoured by v1.8.1 and skips the `version.crowdsec.net` lookup (source-verified in `chooseBranch`), plus the measured online and offline (`--network none`) results.

**Commit 9 - `test: enable CrowdSec preset and file-editor E2E specs and drop tolerant ones`** (#1518, final step)
- Files: the two specs from Commit 1 (remove `fixme`), `tests/security/crowdsec-config.spec.ts` (delete the tolerant "Preset Management" assertions that are now covered by deterministic specs, or convert each remaining case to a deterministic stub; no `isVisible().catch(() => false)` guards, no "feature may not be implemented" annotations remain in `tests/security/crowdsec-*.spec.ts`).
- Depends on: Commits 4 and 5.
- Gate: `npx playwright test tests/security/crowdsec-hub-preset-apply.spec.ts tests/security/crowdsec-file-editor.spec.ts tests/security/crowdsec-preset-apply.spec.ts tests/security/crowdsec-config.spec.ts --project=security-tests` green. Mutation check: temporarily re-add the local-write fallback (or revert the success-status check) and confirm at least one of the new specs fails, then undo. Full-suite and cross-browser runs are CI-only.

**Commit 10 - `docs: document CrowdSec preset apply safety and data persistence`**
- Files: `ARCHITECTURE.md`, `docs/features/crowdsec.md`, `docs/troubleshooting/` entry if appropriate (see 7).
- Depends on: all. Gate: markdown lint/pre-commit; `docs-site` manifest unchanged (new content goes into already-manifested `docs/features/` / `docs/troubleshooting/`).

## 6. Decisions and Edge Cases

- **D1 hub branch (Q1 resolved: option A).** Options: (A, recommended) set `cscli.hub_branch: master` explicitly: identical to today's effective behavior while 1.8.1 is the latest, no dependency on `version.crowdsec.net`, no 404 risk, truthful `cscli version`. (B) Leave auto-selection with the fixed version: matches upstream's intent but can hard-fail hub commands offline and break hub updates when 1.8.1 stops being latest and the CDN has no `v1.8.1` branch (404 today). (C) Pin to the version branch: broken today (404). Commit 8 gate must run the empirical online/offline test either way.
- **Locking (one mechanism).** A single handler-level `sync.Mutex` on `CrowdsecHandler` (e.g. `dataMu`) is held for the whole of `ApplyPreset` (both curated and hub paths), `ImportConfig` and `WriteFile`. `HubService.mu` remains as an inner guard for direct service callers; lock order is always handler mutex first, then `HubService.mu`, so no deadlock. No other lock is introduced.
- Edge cases to cover in tests: snapshot dir name collisions (microsecond timestamp plus exclusive `os.Mkdir`), parent dir not writable, `DataDir` being a mount point (the copy path must work where rename used to fall back), symlinks inside `DataDir` never followed by restore/prune/write, concurrent apply/import/file write serialized by the handler mutex (held for the full `ApplyTimeout`, so file saves block during a long apply; `ReadFile` and `ListFiles` stay lock-free), CrowdSec stopped, existing volumes whose `config.yaml` still points at `/var/lib/crowdsec/data` (sed migration, then the gated `hub upgrade` repopulates), tmpfs `/var/lib/crowdsec` deployments (no longer relevant for data after migration).
- Reload behavior from #1513 (SIGHUP via `reloadCrowdSec`) is unchanged and should also be triggered, best-effort, after the entrypoint-independent hub changes that happen at runtime only (not applicable to startup).

## 7. Docs Impact

- `ARCHITECTURE.md`: update the Layer 2 CrowdSec section (~L868-890): CrowdSec `data_dir` and LAPI db live under `/app/data/crowdsec/data` (persistent), hub data files are re-synced when missing via a gated `cscli hub upgrade`, preset applies and config edits are copy-based/atomic with bounded backup retention, `cscli version` reports the pinned version and the hub branch is explicit. The file layout table around L288-294 gets the data subdirectory note.
- `docs/features/crowdsec.md`: extend "Curated Presets" (L57-59) / Hub Presets with the backup retention (last N kept), that hub presets are also applied on the server with a rollback, and that the config editor only accepts specific file types with validation. Add a short troubleshooting entry for the `unable to init data for file` log line (what it meant, that it self-heals on restart after this release). Keep it novice-friendly.
- `docs/features.md`: no new capability; no change expected (confirm during Commit 10).
- `docs/security.md`: tmpfs `/var/lib/crowdsec` guidance wording (Commit 6).
- Changelog is generated from commit subjects; subjects in section 5.2 are written for that.

## 8. Test Plan Summary and Definition of Done

- **Backend unit:** new tests in Commits 3 and 4 (list above). Coverage: package and patch coverage must not drop; overall >= 85% (`scripts/go-test-coverage.sh`); `bash scripts/local-patch-report.sh` produces `test-results/local-patch-report.md/.json`.
- **Frontend unit:** updated Vitest in Commit 5; `scripts/frontend-test-coverage.sh` >= 85%; `npm run type-check`; `npm run build`.
- **Playwright (#1518):** deterministic stubbed specs per Commit 1/9, role-based locators, no `waitForTimeout`, no visibility-guard skips. Locally only the targeted spec files with `--project=security-tests` (security specs are excluded from the browser projects; other specs use `--project=firefox`); CI runs the full matrix.
- **Build-time assertion (#1524):** Dockerfile `RUN` check in Commit 8 with the negative (mismatched version) and positive proof recorded in the PR description.
- **Container integration:** `scripts/crowdsec_data_persistence_test.sh` and `scripts/crowdsec_startup_test.sh` (Commit 6/8).
- **Security scans:** this is a `fix:` with no new endpoints; CodeQL/Trivy are deferred to CI per CLAUDE.md, except that Commit 7's toolchain Trivy gate runs in CI by design. GORM scan not required (no model changes); run it only if a model is touched.
- **Full DoD checklist before merge (in order):** targeted Playwright (firefox) -> patch coverage preflight -> `lefthook run pre-commit` -> staticcheck (`make lint-fast`) -> backend/frontend coverage -> type-check -> `go build ./...` and `npm run build` -> clean-up (no debug prints, dead code, unused imports). Any failing test, even pre-existing, is fixed in this PR.
- **Pipeline after the plan:** Supervisor plan review -> user approval -> implement commit by commit (`backend-dev` for 2-4, `frontend-dev` for 5, `devops` for 6-8, `playwright-dev` for 1 and 9, `docs-writer` for 10) -> Supervisor implementation review -> `qa-security` last -> `docs-writer` pass.
- **Dispatch instructions (every subagent prompt must include these, verbatim in spirit):** run all tests, builds, coverage scripts, Playwright, linters, Docker builds and `lefthook` as **foreground, blocking** calls with the maximum timeout; never use `run_in_background`, `&`, `nohup` or end the turn waiting for a notification; if a call auto-backgrounds, immediately re-attach and poll until a real result; no worktrees; stay inside the repo (no live hosts); do not commit unless the orchestrator says so; Commit 4's text (body, comments, PR text) must stay vague per section 4; and the Commit 7 to Commit 8 hand-off requires pulling the toolchain bot's commit first.

## 9. Acceptance Criteria

1. Applying a hub preset or editing a CrowdSec file never renames, empties or removes `/app/data/crowdsec`; the live tree is intact after success and after any failure (tests prove it).
2. Full-tree snapshot dirs (`<DataDir>.backup.*`) are bounded to 5 and single-file edit backups (`<DataDir>.filebackup.*`) to 10, independently, after any apply/import/write attempt; snapshots exclude `data/` and `hub_cache/` so they stay small.
3. `POST /admin/crowdsec/file` validates paths, file types, YAML and size, refuses symlink writes, never renames `DataDir`, and `ReadFile`/`ListFiles` share the same validation; the frontend has no local-write fallback.
4. After `docker rm` + recreate with the same volume, the 12 listed hub data files and `GeoLite2-City.mmdb` exist and `crowdsec.log` has no `unable to init data` / GeoIP lines; fresh volumes actually install the items in `install_hub_items.sh`.
5. `cscli version` and `crowdsec -version` print `v1.8.1`; the image build fails if they do not; hub commands work online and offline per D1.
6. `crowdsec_handler.go` no longer contains the preset handlers; no behavior change from the move.
7. No tolerant "may not be implemented" Playwright tests remain for CrowdSec presets, and the mutation check in Commit 9 shows the new specs fail on regression.
8. Only one commit uses the `(security)` scope and its subject is the vague form above.

## 10. Resolved Questions (all decided by the user)

- Q1 (D1): RESOLVED: explicit `cscli.hub_branch: master`.
- Q2: RESOLVED by the user: no private advisory (issue is public; severity low, admin-only). Revisit only if implementation finds impact beyond admin privileges.
- Q3: RESOLVED: write allowlist, protected-file list, 1 MiB cap and the refuse-symlinks rule as specced in 3.2, with a separate looser read/list predicate.
- Q4: RESOLVED: startup log line only (no diagnostics field, no UI).
- Q5: RESOLVED: constants (5 full snapshots, 10 file backups), not configurable.
