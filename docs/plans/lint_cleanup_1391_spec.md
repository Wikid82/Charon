# Spec: Clear Full golangci-lint Findings and Make It Blocking (Issue #1391)

| Field | Value |
| --- | --- |
| Issue | #1391 "chore(lint): resolve ~90 findings from the full golangci-lint config and make it blocking" (labels: backend, frontend, low, security, testing) |
| Branch / PR | `chore/lint-full-config-1391` off `development`, one PR into `development` (medium scale per CLAUDE.md) |
| Status | Revision 2 (supervisor feedback applied 2026-09-28) |
| Date | 2026-09-28 |
| Why this file, not `current_spec.md` | `current_spec.md` holds the approved, in-progress #1317 auth rate-limit spec; it is left untouched. |

## 1. Introduction

`backend/.golangci.yml` (full config: bodyclose, gocritic all tags minus a disabled list, gosec, govet+shadow, ineffassign, staticcheck, unused, errcheck; `tests: true`) reports **91 findings in `backend/`** on both v2.13.0 and the CI-pinned v2.14.0 (gocritic 50, gosec 40, bodyclose 1; measured on both, see 2.2), plus **5 in `agent/`** when the same config is pointed at it (measured: `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...`). CI runs the backend step with `continue-on-error: true`, so nothing blocks. Goal: 0 findings, then make the step blocking, and clear 2 `react-hooks/set-state-in-effect` warnings in `frontend/src/pages/Security.tsx`.

Principles: real fixes first; `//nolint:<linter> // reason` only for a documented genuine need. The codebase already uses `// #nosec Gxxx` (gosec honors it) in places; new suppressions use `//nolint:gosec // reason` (per the issue). `nolintlint` is not enabled, so no extra rules apply. All fixes must be behavior-preserving (Section 4).

## 2. Research Findings

### 2.1 Modules and configs

- Go modules: `backend/` and `agent/` (`go.work` uses both). `.claude/worktrees/**` contains stale copies of both; ignore, do not lint.
- Only `backend/` has `.golangci.yml`. Repo root has `.golangci-fast.yml` (staticcheck, govet, errcheck, ineffassign, unused), used by lefthook pre-commit and by CI for the agent (`quality-checks.yml` ~L393-399) and `make lint-fast` for both modules. The full config has never run against `agent/`; it finds 5 issues there (Section 3.9).
- `make lint-backend` (Makefile L192-194) runs `golangci/golangci-lint:latest` in Docker against `backend/` with `-v`. Lefthook manual stage `golangci-lint-full` runs `scripts/pre-commit-hooks/golangci-lint-full.sh` (installs `@latest` if no v2 found). Local dev host has v2.13.0.

### 2.2 Version drift and the v2.14.0 baseline

| Place | Version |
| --- | --- |
| CI backend step (`quality-checks.yml` L307-314) | `v2.14.0`, Renovate-tracked via the custom regex at `.github/renovate.json` L338-352 (matches only `quality-checks.yml`) |
| CI agent step (L393-399) | `v2.14.0` (same regex) |
| Local host | v2.13.0 |
| `make lint-backend` | `golangci/golangci-lint:latest` via Docker (floating, untracked) |
| `scripts/rebuild-go-tools.sh` L13, `.github/skills/utility-update-go-version-scripts/run.sh` L82 | `...cmd/golangci-lint@latest` (old v1 module path) |

**Baseline run (done during planning, no Docker; the host's root dockerd is stale):** `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`, then `cd backend && golangci-lint run ./...` gives **91 findings, identical rule mix** (gocritic 50, gosec 40, bodyclose 1). Differences vs v2.13.0 are only positional: `middleware/auth_test.go` G124 now reports at L127, L322, L424 (was L127, L176, L202); `caddy/importer.go:427` G703 is **no longer reported** on v2.14.0; `crowdsec/hub_sync.go` G703 reports at L1004, L1010, L1013 (was L1004, L1013). The agent module under the full config gives the same 5 findings on v2.14.0. Implementers must re-run on v2.14.0 (per-package counts below use it). Rechecked by diffing the two runs: the only position shifts are the `auth_test.go` G124 lines and the hub_sync/importer G703 change; every other finding is at the same file:line on both versions, so the positions quoted in this spec are v2.14.0 positions (the `importer.go:427` row is v2.13.0-only, as labelled). Per-package counts on v2.14.0: handlers 29, middleware 3, api/tests 4, cmd/api 2, cmd/localpatchreport 1, routes 2, caddy 4, crowdsec 5, crypto 4, database 2, models 1, network 2, orthrus 5, server 2, services 14, services/remotestorage 7, utils 1, pkg/dnsprovider 3.

### 2.3 Frontend

`npx eslint src/pages/Security.tsx` gives 2 warnings, both `react-hooks/set-state-in-effect`: L93 (`setAdminWhitelist(...)` in an effect keyed on `securityConfig`) and L168 (`useEffect(() => { fetchCrowdsecStatus() }, [])` where `fetchCrowdsecStatus` calls `setCrowdsecStatus`).

## 3. Findings Inventory and Resolutions

Counts are from the 2.13.0 JSON dump. Paths relative to `backend/`. "T" = test file.

### 3.1 gocritic (50) by class

| Class | Locations | Resolution |
| --- | --- | --- |
| paramTypeCombine (12) | handlers/crowdsec_handler.go:2058; crowdsec_handler_test.go:76 (T); handlers_blackbox_test.go:1553 (T); notification_provider_handler.go:97; security_handler.go:1343; routes/routes.go:45; server/server.go:18; services/docker_service.go:275; notify_provider_adapter.go:66; remotestorage/dropbox.go:392; googledrive.go:402; remotestorage/sftp_upload_test.go:34 (T); utils/url_testing.go:417 | Combine adjacent same-type params `(a, b string)`. Pure signature spelling; no caller changes. |
| octalLiteral (9) | handlers/crowdsec_archive_test.go:319,347 (T); crowdsec_handler.go:2065,2071; crowdsec_handler_test.go:3810,4100,4110 (T); system_permissions_handler_test.go:164 (T) | `0644` -> `0o644` etc. Same value. |
| importShadow (4) | crowdsec_handler.go:2589; proxy_host_handler.go:897,966 (local var `errors` shadows package); caddy/manager_multicred_integration_test.go:415 (T, `config`) | Rename the local (`errList`/`validationErrs`, `cfg`). Rename all uses within scope. |
| unnamedResult (3) | logs_handler_test.go:18 (T); security_handler.go:1330; system_permissions_handler.go:379 | Name the results; do not introduce naked returns (keep explicit `return a, b`). |
| equalFold (3 + 1 pair) | services/uptime_service.go:652,689; pkg/dnsprovider/custom/webhook_provider.go:286,335 | Use `strings.EqualFold`. Careful: original was likely `strings.ToLower(x) == "..."`; EqualFold is equivalent for ASCII literals (both "orthrus" and "true"). |
| stringXbytes (4) | crypto/rotation_service.go:307,321,336; crypto/encryption_test.go:495 (T) | `string(a) != string(b)` -> `!bytes.Equal(a, b)`. Semantically identical; `bytes.Equal` is not constant-time either, and these are self-test round-trips (not secret comparison against attacker input), so no crypto-behavior change. Add `bytes` import. |
| httpNoBody (3) | handlers/audit_log_handler_test.go:267,352,383 (T) | `nil` -> `http.NoBody`. |
| emptyStringTest (2) | handlers/custom_theme_handler.go:105 (`len(*req.Name) == 0`); server/emergency_server.go:72 | `== ""`. Same semantics. |
| deferInLoop (2) | handlers/crowdsec_handler.go:1766; services/backup_restore_safe.go:547 | **Real fix**: defer inside `for` holds every file/zip entry open until function return. Extract the loop body into a helper func (or close explicitly) so the close runs per iteration. Needs care (Section 4). |
| unnecessaryBlock (2) | dns_provider_handler_test.go:119; encryption_handler_test.go:55 (T) | Remove the bare `{}` block (verify no variable-scope reliance, i.e. shadowing of a name reused later; compiler will flag). |
| filepathJoin (2) | system_permissions_handler_test.go:598 (T, `"\x00invalid"`); caddy/config_test.go:590 (T, joins an absolute path segment) | 598: the NUL byte is the point of the test (Lstat invalid-argument). Build the path with string concatenation `allowRoot + string(os.PathSeparator) + "\x00invalid"` rather than `//nolint`; if gocritic still flags, `//nolint:gocritic // NUL byte is the input under test`. 590: pass separate segments to `filepath.Join`. |
| builtinShadow (1) | services/uptime_scheduler_test.go:149 (T, `var min, max`) | Rename to `minDue, maxDue` (also fix uses at L153-173). |
| preferDecodeRune (1) | caddy/importer_test.go:429 (T) | `utf8.DecodeRuneInString(output)`. Check empty-string behavior: `[]rune(s)[0]` panics on empty, DecodeRuneInString returns RuneError; the test asserts on a non-empty output so behavior is preserved for passing runs. |
| typeDefFirst (1) | orthrus/muzzle.go:173 | Move `type Muzzle` definition above its methods (pure reorder, no behavior). |

### 3.2 bodyclose (1) and test-only agent/ items

- `services/notify_client_adapter_test.go:94` (T): close `resp.Body` (`defer resp.Body.Close()` / `t.Cleanup`). If the response is a stub with `Body == nil` use `http.NoBody`; do not nolint.
- Agent items in Section 3.9.

### 3.3 gosec, non-test (10): judgement

| Location | Rule | Verdict | Resolution |
| --- | --- | --- | --- |
| services/uptime_scheduler.go:479 | G115 uint64->int64 | False positive: `Uint64(b) % uint64(maxD)` is `< maxD` and `maxD > 0` is guarded above, so the result fits in `time.Duration`. | One-line `//nolint:gosec // G115: result of % uint64(maxD) is < maxD (int64), maxD > 0 guarded above` is acceptable and simpler than a `crand.Int` rewrite. If instead the function is rewritten, update its doc comment ("jitterDuration ... security scanners stay happy about randomness sources"). Not `(security)`. Existing jitter tests (`uptime_scheduler_test.go` ~L133-173) cover range/spread. |
| cmd/api/main.go:178 | G706 log injection (taint) | The flagged line is `log.Fatalf("Usage: %s ...", os.Args[0])`. Program name from argv logged to stderr on a CLI usage error; no log-forging risk to a persisted log. Real-but-trivial hardening: use `filepath.Base(os.Args[0])` or `util.SanitizeForLog` (already used in repo). | Sanitize via `util.SanitizeForLog(os.Args[0])`. Not `(security)`. |
| internal/caddy/importer.go:427 (v2.13.0 only) | G703 | `os.WriteFile(backupPath, ...)` in `BackupCaddyfile(originalPath, backupDir)`; `backupPath` derives from the **caller-supplied `backupDir`**, not from the validated `originalPath` (only the read side is validated). `grep -rn BackupCaddyfile` shows **no non-test callers** (only `importer_test.go` L284-303, L612-659 and `importer_extra_test.go` L92). | **Delete `BackupCaddyfile` as dead code** (CLAUDE.md "CLEAN") along with **all** its tests: in `importer_test.go` `TestBackupCaddyfile` (L284), `TestBackupCaddyfile_PathTraversal` (L613), `TestBackupCaddyfile_SourceNotReadable` (L654); in `importer_extra_test.go` `TestBackupCaddyfile_ReadFailure` (L92), `_Success` (L134), `_WriteFailure` (L195), `_WriteErrorDeterministic` (L357), `_InvalidOriginalPath` (L386); plus any now-unused imports/helpers. Gate: `grep -rn BackupCaddyfile backend/` must return nothing (ignore untracked `backend/test-output.txt` if present) and `go vet ./internal/caddy/...` must pass. Re-verify with grep at implementation time; if a non-test caller has appeared, instead validate that `backupDir` is absolute/cleaned and add `//nolint:gosec // G703: backupDir is operator-configured`. v2.14.0 does not flag it, but deletion is still right. |
| internal/crowdsec/hub_sync.go:1004,1010,1013 | G703 | **Redundant defense-in-depth, not an exploitable escape.** `cleanName := filepath.Clean(hdr.Name)` is already rejected if it has prefix `..`, contains `../`, or is absolute (L~989-991), symlinks are rejected, and `destPath = filepath.Join(targetDir, cleanName)`; a sibling-prefix escape (`/data/x-evil`) is therefore not reachable, and the taint tracker flags the file operations anyway. Still worth tightening because the current `strings.HasPrefix(destPath, filepath.Clean(targetDir))` check is weaker than it looks and `hdr.FileInfo().Mode()` is used verbatim for `MkdirAll`/`OpenFile`. | (a) Replace the prefix test with `rel, err := filepath.Rel(cleanTarget, destPath)`; reject if `err != nil`, `rel == ".."` or `strings.HasPrefix(rel, ".."+sep)`; (b) mask modes: `mode := hdr.FileInfo().Mode().Perm()` (drops setuid/setgid/sticky) and apply `& 0o777`-equivalent for both `MkdirAll` and `OpenFile`; (c) delete the duplicated `// #nosec G304 ... // #nosec G304 ...` comment on the `OpenFile` line and keep one, or a single `//nolint:gosec // G703/G304: destPath contained under targetDir by filepath.Rel check`. Commit prefix `chore:` (or `fix:` if reviewers prefer); **not** `fix(security)`. Existing tests in `hub_sync_test.go`: `TestExtractTarGzRejectsSymlink` (L461), `TestExtractTarGzRejectsAbsolutePath` (L471), `TestExtractTarGz` (L1404), `_NestedPathTraversal` (L2013), `_AbsolutePathWithDots` (L2035), `_EmptyArchive`, `_InvalidTarAfterGzip`, `_LargeNestedStructure`, `_SpecialCharactersInFilenames`, `_DirectoriesWithoutFiles`, `_SkipsSpecialFileTypes` (L2056-2164). **Add only the missing ones:** a bare `..` entry, a `..foo` file name (documents that the existing `HasPrefix(cleanName, "..")` rejects it; if the new Rel-based check would accept `..foo`, that is a behavior change: keep the existing name check as is), and a directory entry with setuid/sticky bits asserting the extracted mode is masked. |
| internal/crowdsec/registration.go:323 | G204 | `registerBouncer(ctx, name)` has exactly one caller (registration.go:120) which passes the constant `defaultRegistrationName`. No user input reaches it. | Plain scoped `//nolint:gosec // G204: fixed binary "cscli"; sole caller passes the constant defaultRegistrationName`. Optionally insert `--` before `name` only if cscli accepts it (verify by running `cscli bouncers add -h`; skip if unsure). No new validation, no new tests. |
| internal/crowdsec/console_enroll.go:51 | G204 | `SecureCommandExecutor.ExecuteWithEnv` is the production implementation of the injectable `CommandExecutor` interface. Verified by grep: all non-test callers pass the literal `"cscli"` (console_enroll.go L215, L268, L307; heartbeat_poller.go L162). | `//nolint:gosec // G204: executor interface; all callers pass the literal "cscli" (verified)`. |
| internal/api/handlers/crowdsec_handler.go:855 | G122 race-prone path in Walk callback | `ExportConfig` walks `h.DataDir` with `filepath.Walk` (Lstat semantics) then `os.Open(path)`. **Current behavior with a symlink in DataDir:** a symlink to a file is followed by `os.Open` and the target content is archived under the link's name (silently exfiltrating anything the process can read if a symlink is planted); a symlink to a directory reports `IsDir() == false`, `os.Open` succeeds, `io.Copy` fails and the handler returns 500. So archive contents are *not* identical after any symlink-related change. | **Explicit choice:** first add a symlink skip in the callback (`if info.Mode()&os.ModeSymlink != 0 { logger.Log().Warn(...util.SanitizeForLog(path)); return nil }`), matching the existing "skip and warn" pattern for inaccessible paths in the same callback. Then either (preferred, minimal) `//nolint:gosec // G122: symlinks skipped above; DataDir is Charon-owned` or, if the maintainer wants the stronger guarantee, `os.OpenRoot(h.DataDir)` + `root.Open(rel)`, in which case a raced symlink yields an error (500) instead of a silent follow. Recommendation: skip+Warn+nolint. Add a test in `crowdsec_archive_test.go`: DataDir containing a regular file, a file symlink and a dir symlink; assert 200, only the regular file in the tar, and no 500. The behavior change (symlinks now omitted rather than followed/500) is intentional and noted in the commit body. |
| cmd/localpatchreport/main.go:251 | G204 | Dev tool run by maintainers; `git -C repoRoot diff baseline`. `baseline` could start with `-` (option injection) only from operator's own CLI args. | `//nolint:gosec // G204: local developer tool; inputs come from the operator's own CLI flags`; optionally insert `--` after baseline handling. |
| internal/api/routes/routes.go:1075 | G118 goroutine uses context.Background | **Definite fix.** The goroutine is inside `RegisterWithDeps(ctx context.Context, ...)` (L116) and an inner `ctx := context.Background()` shadows the outer lifecycle ctx (also a govet shadow risk). | Delete the inner `ctx := context.Background()`, use the outer `ctx` for `caddyManager.Ping(ctx)` and `ApplyConfig(ctx)`, and add `case <-ctx.Done(): return` to the wait `select` (before/next to `timeout` and `ticker.C`); also return early before `Apply` if `ctx.Err() != nil`. Behavior change: bootstrap stops on shutdown (desired per CLAUDE.md "respect server.Run(ctx)"). Verify `go test ./internal/api/routes/...` (callers of `Register`, L103, pass a ctx; confirm it is not an already-cancelled or `context.Background()` value that changes timing in tests). |
| services/backup_remote_service.go:224,300,390 | G117 marshaled "Password" field | **Verified now:** each `json.Marshal(secrets)` is immediately followed by `s.encryption.Encrypt(secretsJSON)` and stored only as `SecretsEncrypted` (L224-233 Create; L300-309 Update, guarded by `s.encryption == nil -> ErrEncryptionKeyMissing`; L390-395 OAuth token refresh via `t.svc.encryption.Encrypt`). The plaintext JSON never leaves the function. False positive by design. | Three scoped `//nolint:gosec // G117: secrets are encrypted (Encrypt) before persistence and never returned or logged`. Do not rename JSON tags. |

Non-test gosec items that change behavior: hub_sync containment/mode masking (internal tightening, no reachable-input change), the ExportConfig symlink skip, and the routes.go ctx fix. Each is called out in its commit body so a revert is clean. Nothing in this PR qualifies for `(security)`.

### 3.4 gosec in test files (30): resolutions

| Location | Rule | Resolution |
| --- | --- | --- |
| api/middleware/auth_test.go:127,322,424 (v2.14.0 positions; v2.13.0 reported 127,176,202) | G124 cookie flags | Set `HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode` on the test cookies **if** the middleware under test does not depend on those flags (it reads only the cookie value; Secure cookie over `httptest` (http) still gets attached by explicit `AddCookie`). Caveat: `Secure: true` on a cookie sent through plain-http `httptest` is fine when added via `req.AddCookie` (the jar is not involved), but confirm with the tests; if a test asserts the exact `Set-Cookie`/cookie string this breaks. Fallback `//nolint:gosec // G124: test cookie, flags irrelevant to the middleware under test`. |
| api/tests/user_smtp_audit_test.go:297,309 | G101 hardcoded creds | Test fixtures for redaction assertions. `//nolint:gosec // G101: dummy secret used to assert redaction`. Genuine need (the literal is the test input). |
| api/tests/user_smtp_audit_test.go:559,602; caddy/validator_emergency_test.go:152; network/safeclient_test.go:756; pkg/dnsprovider/registry_test.go:449 | G115 int/uint -> rune | Loop generating chars: iterate over a `rune` counter (`for r := 'a'; ...`) or use `string(rune(...))` from a bounded value with explicit range check; alternatively `strings.Repeat("a", n)` when only length matters. Prefer restructuring (iterate a `rune` variable, or `strings.Repeat`/`bytes.Repeat` when only length matters); nolint only if the conversion is the test subject. |
| database/pending_restore_process_test.go:50,287; orthrus/ca_test.go:165 | G301 dir perms | `0o750` (or `0o700`); `t.TempDir` unaffected. Check the test does not assert on wide mode. |
| orthrus/ca_test.go:107,120,134 | G306 WriteFile perms | `0o600`. If a test verifies the CA loader's permission handling, keep the specific mode and nolint with reason. |
| services/remotestorage/sftp_upload_test.go:213,214,277 | G302 file perms | `0o600`; if the test is about SFTP stat modes (mode bits preserved end-to-end) use nolint with reason. Implementer decides per site by reading assertions. |
| services/backup_encryption_test.go:27,32,160 | G304 file inclusion | Paths are from `t.TempDir()`. `//nolint:gosec // G304: path is a test-controlled temp file`. Alternative: `os.ReadFile(filepath.Clean(p))` (gosec accepts Clean). Prefer `filepath.Clean`, no nolint. |
| models/remote_storage_target_test.go:34 | G101 | Dummy secret: `//nolint:gosec // G101: dummy credential fixture`. |
| network/safeclient_test.go:1192 | G112 Slowloris | Add `ReadHeaderTimeout: 5 * time.Second` to `&http.Server{...}`. Real fix, no test impact. |
| services/remotestorage/googledrive_test.go:112 | G705 XSS | Test httptest handler writes `fmt.Fprintf(w, ..., %q)`. Set `Content-Type: application/json` before writing and marshal with `json.NewEncoder(w).Encode(...)` (removes hand-built JSON too). If flagged still: nolint with reason (test server, JSON response). |
| cmd/api/main_test.go:434 | G704 SSRF | Test requests localhost httptest server URL. `//nolint:gosec // G704: URL points at a local httptest server`. |
| handlers/hecate_handler_test.go:1227 | G104 unhandled error | `conn.SetReadDeadline(...) //nolint:errcheck` exists but gosec G104 still fires. Assign `_ = conn.SetReadDeadline(...)` (and drop the errcheck nolint). |

### 3.5 Summary counts to track

Backend 91 = gocritic 50 + gosec 40 + bodyclose 1; agent 5. Re-count on v2.14.0 in Commit 1 and record in the PR description.

### 3.6 Suppression budget

Expected `//nolint` additions: about 12-15 (G101 x3, G204 x3, G117 x3, G115 jitter, G704, G122, plus G304/G302/G124/G115-rune fallbacks in tests). G118 and G703(importer) are fixed/deleted, not suppressed. Each carries a specific reason after the linter name. No blanket file-level `//nolint`, no config-level `exclusions` to hide categories (rejected: hides new true positives in tests).

### 3.7 Rejected alternative

Enabling gocritic exclusions or `gosec: excludes: [G101, ...]` in `.golangci.yml` would zero the count fast but silently exempts future real findings. Rejected except where a whole class is provably meaningless for the repo (none identified).

### 3.8 Config note

Do not enable additional linters or the disabled gocritic checks in this PR (scope creep and drift risk).

### 3.9 agent/ module (5 findings under the full config)

`agent/leash/leash.go:126` bodyclose (real; close body or verify it is hijack/upgrade case, else nolint with reason); `agent/leash/leash_test.go:104` bodyclose (T); `agent/muzzle/muzzle_test.go:62` unnamedResult (T); `agent/muzzle/muzzle_test.go:366` preferFprint (T; `fmt.Fprintf(conn, ...)`); `agent/leash/leash_test.go:249` G115 int->byte (T; range-check or mask `byte(i & 0xff)`; masking satisfies gosec only if it recognizes it, else nolint with reason).

Decision: **include the agent in the full-config lint**. Fix the 5 findings (Commit 7); the agent CI step switches to `--config ../backend/.golangci.yml` only in Commit 9 (so no commit before it changes CI). The full config lives in `backend/` and is intentionally shared rather than duplicated as `agent/.golangci.yml` or moved to the repo root (both rejected as churn); a comment in the workflow and Makefile explains the cross-module path. `make lint-backend` mounts only `backend/`, so add a `make lint-agent` target (or have `lint-backend` run both), using the same pinned version.

## 4. Behavior-Preservation Risks

Disabled in config already: hugeParam, rangeValCopy, ifElseChain, appendCombine, appendAssign, commentedOutCode, sprintfQuotedString, wrapperFunc, whyNoLint. So those risky refactors are not in scope; do not re-enable.

| Change | Risk | Care / covering tests |
| --- | --- | --- |
| deferInLoop x2 (crowdsec_handler.go:1766; backup_restore_safe.go:547) | Low. Verified: in both loops every path after the `defer` returns (backup_restore_safe.go L547-565 returns on decode error or `&parsed`; crowdsec_handler.go validateKey retry loop returns on every status branch after `defer resp.Body.Close()`; the `continue` paths sit before the defer). So the deferred close already runs once, at the first return: the finding is lint-only and the fix is control-flow identical. | Concrete fixes: (a) backup_restore_safe.go: extract `readManifest(f *zip.File) (*BackupManifest, error)` that opens, defers close, decodes; caller returns `nil, false, err` on error else `m, false, nil`. (b) crowdsec_handler.go: extract `validateKeyOnce(ctx, client, req...) (ok bool, retry bool)` or simpler, close `resp.Body` explicitly before each of the 3 returns via a small `closeBody(resp)` helper. Tests: `crowdsec_handler_test.go` (key validation), `backup_restore_safe_test.go`. Run whole packages. |
| ExportConfig symlink skip (crowdsec_handler.go:855) | **Intentional behavior change:** symlinks in DataDir were followed (file symlink archived as target content; dir symlink -> 500); they are now skipped with a Warn. Regular-file archive contents and tar header names/modes are unchanged. | `crowdsec_archive_test.go` and handler export tests; new symlink-in-datadir test (Section 3.3). |
| hub_sync extraction containment/mode masking | Redundant containment (no reachable input changes); modes are now `Perm()`-masked (setuid/setgid/sticky stripped): a fixture depending on such bits would change. | Existing `TestExtractTarGz*` (L461-2164) plus the 3 new tests; run the full `internal/crowdsec` package. |
| importShadow renames | Low: compile errors surface mistakes; but shadowed `errors` then `errors.New` uses are inside the same scope, watch for a later `errors.Is` that previously failed to compile. | Package tests (`proxy_host_handler_test.go`, `crowdsec_handler_test.go`). |
| stringXbytes -> bytes.Equal in rotation_service.go | Low; identical results. | `internal/crypto` tests (`rotation_service_test.go`). Coverage stays. |
| equalFold | Low; confirm original was case-insensitive intentionally (if it was `x == "true"` exactly, gocritic would not fire; it fires only on `ToLower(x) == "true"`). | uptime_service tests; webhook_provider tests. |
| jitterDuration nolint | None (comment only); if rewritten, the fallback must remain `maxD/2` and the doc comment be updated. | uptime_scheduler tests including spread assertion (`> 20s`). |
| uptime_service equalFold pair | Low | `uptime_service*_test.go` |
| paramTypeCombine / octal / emptyStringTest / typeDefFirst / httpNoBody / unnamedResult / unnecessaryBlock | Compile-time equivalent. Named results must not be shadowed inside the function (govet shadow is enabled). | `go build ./... && go vet` + package tests. |
| Test-only perm changes (0o600 etc.) | A test may rely on a wider mode (e.g. reading via another user, or asserting mode). | Run the package; per-site judgement. |
| routes.go outer-ctx fix | Low-medium: the bootstrap goroutine now stops when the ctx passed to `RegisterWithDeps` is cancelled; tests that pass a short-lived ctx could stop the initial sync early. | `go test ./internal/api/routes/...`; check ctx passed by tests and `cmd/api`. |

Global rule: no commit may change exported APIs or JSON tags. Run `go build ./...` and `go vet ./...` after each commit.

## 5. CI Change

File: `.github/workflows/quality-checks.yml`. All CI edits land in Commit 9 (the version-pinning/Renovate pieces in Commit 1).

1. **Backend step** (L307-314): delete `continue-on-error: true`; keep `working-directory: backend`, `args: --timeout=5m`.
2. **Step ordering (job structure).** In `backend-quality` the tests (L268) run before lint, but the lint step (L307) precedes "GORM Security Scanner", its summary/annotate steps and "Run Perf Asserts" (L316-352), so a blocking lint failure would skip them. In `agent-quality` the lint step (L393) precedes "Run Go tests with coverage gate" (L401), so a lint failure would hide test results. Decision: **move the lint step to be the last step of each job** and give it `if: ${{ !cancelled() }}` (under the default `if: success()` a failing test step would skip lint, so `!cancelled()` makes both results surface; the job still fails if either fails. No new job, so the required check names "Backend (Go)"/"Agent (Go)" are unchanged and no branch-protection edit is needed; the cost is a second setup-go/cache-free run in the same job, which is what exists today). (Note: with `!cancelled()` the lint step also runs after a failing GORM scan, which is desired.) Alternative (separate `backend-lint`/`agent-lint` jobs) rejected because it renames/adds required checks.
3. **Agent step** (L393-399): switch to `--config ../backend/.golangci.yml`, drop `continue-on-error` (none today), and **rename** the step from "Run golangci-lint (fast config, root-shared)" to "Run golangci-lint (full config)"; update the comment above it. The fast config stays for lefthook and `make lint-fast`.
4. **Version single-sourcing (REQUIRED in this PR).** The Makefile pin would be a second copy that Renovate does not track. In Commit 1: pin `Makefile` `lint-backend` (and new `lint-agent`) to `golangci/golangci-lint:v2.14.0` **and** extend the Renovate regex manager (`renovate.json` L338-352) so its `managerFilePatterns` also matches `^Makefile$` with an additional `matchStrings` entry for `# renovate: datasource=github-releases depName=golangci/golangci-lint` followed by `golangci/golangci-lint:v(?<currentValue>[^\s]+)`. Alternative that avoids a second copy: have the Makefile read the version from the workflow (`grep`/`sed` on `quality-checks.yml`), also acceptable; pick one and document it. **Regex constraint:** the Makefile `# renovate: datasource=github-releases depName=golangci/golangci-lint` comment must sit on the line **immediately above** the tab-indented `docker run ... golangci/golangci-lint:v2.14.0 ...` recipe line (the regex is `# renovate: ...\n\t.*golangci/golangci-lint:v(?<currentValue>[^\s]+)`, i.e. the newline must be followed by a tab, so the comment cannot be moved above the target name or separated by blank lines); the Commit 1 gate includes a `grep -B1 'golangci-lint:v' Makefile` check for both recipes (each needs its own comment line). The `forced-back-to-chore:` packageRule for CI tooling (`renovate.json` L858) matches `custom.regex` trackers under `.github/workflows`, `scripts`, `.github/skills`; add `Makefile` there so a Makefile-tracked bump does not become a releasing `deps:` commit.
5. **Local install/docs drift (Commit 1).** `CONTRIBUTING.md` L24 `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` (v1 path) -> `.../v2/cmd/golangci-lint@v2.14.0`; L53 same snippet; L69 "version 1.xx.x" -> "2.14.x"; the L123-127 rebuild example that shows `golangci-lint: 1.25.6` -> a v2 version. Also update `scripts/rebuild-go-tools.sh` L13 and `.github/skills/utility-update-go-version-scripts/run.sh` L82 to the `/v2/` module path (keep `@latest` there only if the script's purpose is "rebuild whatever is installed"; note that the CI-pinned version is authoritative).
6. **Drift risk.** A Renovate bump of golangci-lint can add findings and turn the bump PR red; that is intended (fixes ride with the bump). Never use `latest` in CI.
7. Verify no job-level `if:`/`needs:` depends on the lint step outcome (`grep -n "steps.golangci"` returns nothing today).

## 6. Frontend: Security.tsx

No new dependencies; behavior identical.

1. **L93 whitelist sync.** Replace the effect with a draft: `const [adminWhitelistDraft, setAdminWhitelistDraft] = useState<string | null>(null)` and `const adminWhitelist = adminWhitelistDraft ?? securityConfig?.config?.admin_whitelist ?? ''`. Readers to update: the input `value` (L390), its `onChange`, and the **Save button (L397) `mutate({ name: 'default', admin_whitelist: adminWhitelist })`**, which must read the derived value. **Stale flash:** `useUpdateSecurityConfig` (hooks/useSecurity.ts L40-53) only calls `invalidateQueries(['securityConfig'])` in `onSuccess` and does not `setQueryData`, so resetting the draft to `null` immediately would briefly show the old server value until the refetch lands. Fix: call `mutate(payload, { onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['securityConfig'] }); setAdminWhitelistDraft(null) } })` (per-call `onSuccess` runs after the hook's own), or leave the draft in place after save (it then equals the saved value). Use the await-then-reset variant.
2. **L168 CrowdSec status.** Replace `crowdsecStatus` state + `fetchCrowdsecStatus` + mount effect with `useQuery({ queryKey: ['crowdsec-status'], queryFn: async () => { try { return await statusCrowdsec() } catch { return null } } })`. Notes: `data` is `undefined` while loading and `null` on error; both fall through `??` identically at L283-284 and L374, and `crowdsecStatus &&` at L455 is falsy for both, so no UI difference. Manual call sites: L217 (`onError`, fire-and-forget) -> `queryClient.invalidateQueries({ queryKey: ['crowdsec-status'] })`; L225 sits inside `Promise.all` in `onSuccess`, so use the same `invalidateQueries` (returns a promise that resolves after refetch) to preserve await ordering. Because the `queryFn` never rejects, retry does not apply; check default `staleTime` (0) and `refetchOnWindowFocus` against tests, and that test QueryClients in `Security*.test.tsx` mock `statusCrowdsec` per test. **Shared hook decision:** `statusCrowdsec` is also used in `pages/CrowdSecConfig.tsx` L94-100, but as `useQuery({ queryKey: ['crowdsec-lapi-status'], queryFn: statusCrowdsec, enabled: consoleEnrollmentEnabled && initialCheckComplete, refetchInterval: 5000, retry: false })`. That query needs different semantics (conditional `enabled`, 5s polling, errors surfaced rather than mapped to `null`), and one query key cannot carry two option sets safely, so it is **not** merged into `['crowdsec-status']`. The two keys stay distinct: CrowdSecConfig's key and tests are unaffected by this PR. Per CLAUDE.md, wrap the new Security query in `src/hooks/useCrowdsecStatus.ts` (`useCrowdsecStatus()` returning the `useQuery` result with the error-to-`null` mapping and exporting the key constant `CROWDSEC_STATUS_QUERY_KEY`) so the key is defined once; CrowdSecConfig keeps its own query and is left alone (optionally reuse the exported key-namespace convention only). Add a small hook test (`src/hooks/__tests__/useCrowdsecStatus.test.tsx`). The two imperative `statusCrowdsec()` calls in the power mutation (Security.tsx L182, L195, verifying start/stop) stay as direct calls: they need a fresh, uncached read.
3. **Tests:** `frontend/src/pages/__tests__/Security.test.tsx`, `Security.crowdsec.test.tsx`, `Security.functional.test.tsx`, `Security.dashboard.test.tsx` (existing); add cases: whitelist initializes from config, edits persist until save then reflect server value without flashing, CrowdSec status running/stopped/error. Run `npx vitest run src/pages/__tests__/Security` (foreground). `npx eslint src/pages/Security.tsx` = 0 warnings; `npm run type-check`.
4. **E2E (concrete):** the changed UI is the Security dashboard whitelist card and CrowdSec toggle/status. Run `npx playwright test tests/security/security-dashboard.spec.ts tests/security/crowdsec-first-enable.spec.ts --project=firefox` (both exist; found via `grep -rl "admin_whitelist\|crowdsec" tests/`). `tests/security-enforcement/zzz-admin-whitelist-blocking.spec.ts` exercises whitelist blocking (backend enforcement) and needs the security-suite setup; run it only if the implementer changes how the whitelist is submitted (it should not).

## 7. Implementation Plan (Phases)

Phase 1 E2E specs: none new (pure lint). Phase 2 backend/agent fixes. Phase 3 frontend. Phase 4 CI + verify. Phase 5 docs.

## 8. Commit Slicing Strategy (single PR `chore/lint-full-config-1391` -> `development`)

Baseline: v2.14.0, 91 backend findings + 5 agent. Gate command: `cd backend && golangci-lint run ./... 2>&1 | tail -3` (use the v2.14.0 binary). Lint steps in both jobs carry `if: ${{ !cancelled() }}` (Commit 9). Every commit also: `go build ./... && go vet ./...` and `go test` for touched packages, foreground. Expected remaining backend count after each commit is in the table.

| # | Commit | Scope / files | Depends | Gate (remaining backend findings) |
| --- | --- | --- | --- | --- |
| 1 | `chore(lint): pin lint tooling version and add Renovate tracking` | `Makefile` (pin `lint-backend`, add `lint-agent`), `.github/renovate.json` (regex + Makefile in packageRule), CONTRIBUTING.md L24/L53/L69/L123-127, `scripts/rebuild-go-tools.sh`, `.github/skills/utility-update-go-version-scripts/run.sh`. No Go code. | none | 91 (unchanged); JSON valid (`jq . .github/renovate.json`); `make lint-backend` runs the pinned version |
| 2 | `chore(lint): fix gocritic findings in handlers` | handlers non-test files and these test files only: audit_log_handler_test, crowdsec_archive_test, crowdsec_handler_test, dns_provider_handler_test, encryption_handler_test, handlers_blackbox_test, logs_handler_test, system_permissions_handler_test (paramTypeCombine, octal, importShadow, unnamedResult, emptyStringTest, unnecessaryBlock, httpNoBody, filepathJoin, deferInLoop at crowdsec_handler.go:1766) | 1 | 64; `go test ./internal/api/handlers/...` |
| 3 | `chore(lint): resolve gosec findings in API and middleware tests` | `hecate_handler_test.go` (the only handlers file, disjoint from Commit 2's list), `middleware/auth_test.go`, `api/tests/user_smtp_audit_test.go`, `cmd/api/main_test.go` | 1 (files disjoint from 2, so order-independent) | 55; `go test ./internal/api/... ./cmd/api/...` |
| 4 | `chore(lint): harden CrowdSec archive handling and annotate command execution` | `crowdsec/hub_sync.go` (Rel containment, mode mask, comment cleanup) + 3 new tests in `hub_sync_test.go`; `registration.go` and `console_enroll.go` scoped nolints; `handlers/crowdsec_handler.go` (G122 symlink skip + nolint) + symlink test in `crowdsec_archive_test.go` | 2 (same handlers file) | 49; `go test ./internal/crowdsec/... ./internal/api/handlers/...` |
| 5 | `chore(lint): resolve findings in services and remotestorage` | services/* incl. deferInLoop `backup_restore_safe.go` (`readManifest`), uptime_service equalFold, uptime_scheduler G115 nolint, backup_remote_service.go (G117 x3 nolint), remotestorage/*, notify_*, docker_service, utils/url_testing.go, related tests (bodyclose, G304, G302, G705, builtinShadow) | 1 | 27; `go test ./internal/services/... ./internal/utils/...` |
| 6 | `chore(lint): resolve remaining backend findings` | crypto (rotation_service + encryption_test), caddy (**delete dead `BackupCaddyfile`** + all 8 of its tests across `importer_test.go` and `importer_extra_test.go` (gate: `grep -rn BackupCaddyfile backend/` empty); config_test, manager_multicred_integration_test, validator_emergency_test, importer_test preferDecodeRune), orthrus (muzzle.go typeDefFirst, ca_test), network/safeclient_test, database/pending_restore_process_test, models/remote_storage_target_test, server (server.go, emergency_server.go), routes/routes.go (paramTypeCombine + outer-ctx G118 fix), cmd/api/main.go, cmd/localpatchreport/main.go, pkg/dnsprovider/* | 2-5 (its gate is backend = 0) | **0**; full `go test ./...` in backend |
| 7 | `chore(lint): resolve full-config findings in agent module` | agent/leash/leash.go(+test), agent/muzzle/muzzle_test.go | 1 | agent: 5 -> 0 with `--config ../backend/.golangci.yml`; `go test ./...` in agent |
| 8 | `chore(lint): clear react-hooks warnings in Security page` | frontend/src/pages/Security.tsx + Vitest updates | none | eslint 0 warnings; type-check; vitest; targeted Playwright (firefox) |
| 9 | `chore(ci): make full golangci-lint blocking` | quality-checks.yml: remove `continue-on-error`; move lint step to last in `backend-quality` and `agent-quality`; agent step -> full config and renamed. **Only this commit changes CI behavior for the agent.** | 2-7 | backend 0, agent 0 on v2.14.0; workflow YAML valid (`actionlint` if available); PR CI green |

Commit 6 depends on 2-5 because its gate is the whole-backend zero count. Commits 2/3 overlap the handlers package but not files (listed above).

Rollback/contingency: each commit is revertable alone. If v2.14.0 shows new findings late, add them to the relevant commit rather than re-adding `continue-on-error`. If Commit 4's hub_sync tightening or symlink skip breaks a legitimate fixture in CI, revert that commit and fall back to documented nolints while filing a follow-up issue. Do not leave the CI step non-blocking to unblock the PR.

## 9. Acceptance Criteria

1. `cd backend && golangci-lint run ./...` = 0 findings on the CI-pinned version; same for `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...`.
2. `continue-on-error` removed from the full-config backend step; agent step uses the full config; Makefile pinned to same version.
3. Every new `//nolint` names the linter and a specific reason; none file-level.
4. `npx eslint src/pages/Security.tsx` reports 0 warnings; `npx eslint .` count of `set-state-in-effect` in that file is 0.
5. All existing tests pass; no exported API or JSON tag changed.
6. New tests: hub_sync (`..` entry, `..foo` name, masked dir mode) and the ExportConfig symlink case; `BackupCaddyfile` and its tests deleted.

## 10. Definition of Done Mapping (CLAUDE.md)

| Step | Applies? |
| --- | --- |
| 1 Playwright | Only for Commit 8: targeted Security spec(s), firefox, foreground. Skip for Go-only commits. |
| 1.5 GORM scan | Only if `backend/internal/models/**` non-test code or GORM queries change. Commit 6 touches only `remote_storage_target_test.go`; run `./scripts/scan-gorm-security.sh --check` only if a non-test model file is edited (expected: no). |
| 2 Local patch preflight | Yes: `bash scripts/local-patch-report.sh`. |
| 3 Security scans (CodeQL/Trivy) | Defer to CI (chore/fix scoped, no new feature surface), no local CodeQL run needed (no new feature surface; Commit 4 is defense-in-depth). |
| 4-5 Lefthook, staticcheck | Yes: `lefthook run pre-commit`; `make lint-fast`. |
| 6 Coverage | Yes: `scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh`, >= 85%. |
| 7 Type check | Yes for frontend: `npm run type-check`. |
| 8 Build | `cd backend && go build ./...`, `cd agent && go build ./...`, `cd frontend && npm run build`. |
| 9 Fixed/new code testing | Yes; failing tests in touched packages are fixed in-PR. |
| 10 Clean up | Yes. |

All commands foreground and blocking with generous timeouts, per CLAUDE.md.

## 11. Documentation Impact

- `ARCHITECTURE.md`: no change (no architecture/structure change).
- `CONTRIBUTING.md`: correct golangci-lint version note (v2) and state that the CI-pinned version is authoritative; mention `make lint-backend` runs backend and agent.
- `docs/features.md`: none. No user-facing docs.
- `.gitignore`/`.dockerignore`/`.codecov.yml`: no new files expected.
- Commit-subject policy: no commit uses `(security)` (nothing here is a genuine vulnerability fix); all are `chore:`. Every subject is published verbatim in the changelog (merge commit), so keep them accurate and unremarkable.

## 12. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| v2.14.0 finds more/different issues than 2.13.0 | Re-baseline in Commit 1; fix within package commits |
| deferInLoop refactors alter control flow | Dedicated helpers, package tests, careful review |
| Blocking lint slows future feature PRs | Intended; fast config already gates commits, keeping the full config clean is cheap |
| Renovate bump of golangci-lint turns bump PR red | Expected; fixes ride with the bump; Makefile and workflow pins move together via the extended regex |
| Concurrent work on the #1317 branch touching same files (handlers, routes) | Rebase before Commit 2/6; keep changes mechanical to minimize conflicts |

## Revision 3: uncapped baseline

golangci-lint's default caps (50 issues per linter, 3 identical messages) hid most findings, so the counts in Sections 3 and 8 understate the real backlog. From this revision on, **every gate and measurement uses the uncapped flags** with the pinned v2.14.0 binary:

```
cd backend && golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 ./...
```

Uncapped baseline (backend): about 490 findings at v2.14.0 before Commit 2, 173 after Commit 2, 108 after Commit 3. The per-commit "remaining" numbers in the Section 8 table are superseded by the table below. Commit 3 (as implemented) covers every `_test.go` finding under `internal/api/{routes,middleware,tests,handlers}` and `cmd/api`, not only the four files originally listed (it fixed 65 findings: httpNoBody, G124 cookie SameSite, G101/G302/G301/G704 scoped nolints, G306 mode, G115 rune conversions via `strconv.FormatUint`, G104 hecate, unnamedResult).

Uncapped inventory after Commit 3 (108 total), with owning commit:

| Package | Findings | Owner |
| --- | --- | --- |
| internal/api/handlers (crowdsec_handler.go G204, G122) | 2 | Commit 4 |
| internal/crowdsec (hub_sync 3, registration 1, console_enroll 1) | 5 | Commit 4 |
| internal/services (non-test and tests) | 36 | Commit 5 |
| internal/services/remotestorage | 8 | Commit 5 |
| internal/utils | 1 | Commit 5 |
| internal/api/routes (routes.go: paramTypeCombine, G703 x4, G118) | 6 | Commit 6 |
| internal/caddy | 7 | Commit 6 |
| internal/cerberus | 4 | Commit 6 |
| internal/crypto | 5 | Commit 6 |
| internal/database | 7 | Commit 6 |
| internal/models | 1 | Commit 6 |
| internal/network | 2 | Commit 6 |
| internal/orthrus | 8 | Commit 6 |
| internal/patchreport | 2 | Commit 6 |
| internal/server | 8 | Commit 6 |
| cmd/api, cmd/localpatchreport | 2 | Commit 6 |
| pkg/dnsprovider, pkg/dnsprovider/custom | 4 | Commit 6 |

Revised uncapped gates (remaining backend findings after each commit): Commit 3 = 108 (done); Commit 4 = 101; Commit 5 = 56; Commit 6 = 0. New findings surfaced by the uncapped run that the spec's original tables do not list (e.g. `routes.go` G703, `cerberus`, `internal/patchreport`, extra `services` and `orthrus` test rows, `crowdsec_handler.go:52` G204) are resolved in the owning commit using the same resolution rules as Sections 3.3 and 3.4. Commit 7 (agent) must re-measure the agent module uncapped before starting and record its own baseline.

Commit 9 scope addition: set `max-issues-per-linter: 0` and `max-same-issues: 0` in the `issues:` block of `backend/.golangci.yml` (golangci-lint v2 syntax) so CI reports the honest count and the blocking gate cannot be hidden by the default caps. This is the only config edit permitted by this plan, and it belongs to Commit 9 (Commit 3 does not touch the config).
