# QA and Security Report: fix/crowdsec-preset-apply-hardening (PR #1525)

Date: 2026-10-09
Scope: full diff `origin/development...HEAD` (HEAD 513440f5), spec `docs/plans/current_spec.md` (issues #1514-#1518, #1523, #1524).

## Verdict: PASS (no blocking findings)

No critical or high findings. Two low-severity security hardening candidates are flagged for the maintainer, plus a few low/medium non-security items. None block the PR.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | pass |
| Backend vet | `go vet ./...` | pass, no output |
| golangci-lint, backend | `golangci-lint run --config .golangci.yml ./...` | 0 issues |
| golangci-lint, agent | `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...` | 0 issues |
| Frontend type-check | `npm run type-check` | pass |
| Frontend build | `npm run build` | pass |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query files changed |
| Lefthook | `lefthook run pre-commit --all-files` (plain run skips every hook with nothing staged) | all 17 hooks pass, including semgrep (0 findings, 367 rules), shellcheck, actionlint, dockerfile-check, golangci-lint-fast, frontend-lint |
| govulncheck | skill `security-scan-go-vuln` | 0 affected. 1 module-level advisory, GO-2026-5932 (x/crypto/openpgp, no fix), not called by Charon code |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum (backend, agent), package-lock.json and Dockerfile | no HIGH/CRITICAL output. The snap-confined trivy cannot read `/projects`, so the manifests were copied under `$HOME` for the scan. No container image scan was done. CI runs the full Trivy/CodeQL. |
| CodeQL | not run locally | change is `fix:`-scoped with no new feature surface, so deferred to CI per CLAUDE.md |
| E2E | `docker-rebuild-e2e` skill, then `npx playwright test tests/security/crowdsec-hub-preset-apply.spec.ts tests/security/crowdsec-file-editor.spec.ts tests/security/crowdsec-config.spec.ts --project=security-tests` | 27 passed, 0 failed (image rebuilt first because the Dockerfile and merge commits were newer than the running container) |
| Coverage (given, not rerun) | prior runs | backend 90.9% stmt, frontend 91.4% lines, patch 93.9% |

The `go-test-coverage` and `local-patch-report` scripts were not rerun, as instructed. The working tree stayed clean until this report was written.

## Security review

Reviewed against SECURITY.md. No exploit detail is given here.

- `backend/internal/crowdsec/backup.go`: snapshots and restores copy rather than rename. Symlinks are preserved literally and never followed when copying. Clearing the config removes symlinks themselves, because `DirEntry.IsDir` is false for links. Directories are created 0700. Engine-owned state (live db and WAL/SHM at any depth, top-level `data/` and `hub_cache/`) is excluded from snapshot, restore, clear and import. Prune sorts by the timestamp in the directory name, uses `Lstat` and skips non-directories, so a symlinked "backup" is ignored. No issue.
- `backend/internal/api/handlers/crowdsec_files.go`: relative-path checks use `filepath.IsLocal`, so NUL, absolute and `..` paths are rejected. Read resolves symlinks, checks containment, re-applies the deny-list to the resolved path and requires a regular file. Write refuses any symlink along the path and any non-regular target. It also enforces the extension allowlist, the credential-file deny-list, the temp-prefix deny-list, `hub/` and `config/hub/` exclusion, a 1 MiB content cap and an 8 MiB body cap (`MaxBytesReader`). Writes go through a temp file, fsync and rename, and the original mode is preserved. A single-file backup is taken before the write. Operations serialize on `dataMu`. Logged errors go through `sanitizeForLog`. Adequate. Minor items are F2 to F4 below.
- `crowdsec_handler.go` `ImportConfig`: takes `dataMu`, then snapshots, clears, extracts and validates. It rolls back in place on failure and then prunes. The extractor blocks traversal and skips engine-owned entries, and symlink and hardlink entries are not materialized. Lock order is documented (`dataMu` before `HubService.mu`). See F3.
- `hub_sync.go` and `curated_apply.go`: apply is now snapshot, then cscli or extract, then restore on failure, with prune afterwards. The extractor rejects symlink entries, traversal and absolute paths, strips setuid and setgid bits, caps decompressed size at 100 MB and skips engine-owned paths. See F1.
- `.docker/docker-entrypoint.sh`: the new `data_dir` redirect, `hub_branch` pin and missing-data recovery are idempotent and non-fatal. `hub upgrade` is bounded by `timeout 120s`. Values read from hub YAML (`dest_file`) are only used in `[ -e ]` tests and echoes, never executed or written. All variables are quoted. shellcheck passes.
- `configs/crowdsec/install_hub_items.sh`: the `is_installed` helper greps the JSON "installed" flag. No injection surface (arguments are static).
- `Dockerfile`: the XMOD pin is parameterized and registered in `scripts/toolchain-key.sh` and the toolchain key. The x/net and x/crypto re-pin and the post-build `go version -m` guards are correct (`sort -V` floor check, fails the build). The ldflags target `go-cs-lib/version.Version`, and a new cscli `version: v${CROWDSEC_VERSION}` assertion guards against a silent no-op `-X`. The toolchain tag and digest were synced. No weakening.
- Frontend `CrowdSecConfig.tsx`: the client-side preset fallback write is removed, so the server is authoritative. Server error text is rendered via toast text (React-escaped), so there is no XSS. Mutation errors now surface.
- CI: the new `notify-codecov` job uses a SHA-pinned action, `persist-credentials: false` and a minimal `if` guard. The persistence integration step is wired with cleanup.

## Findings

No critical or high findings.

### Security-sensitive (do not file publicly; maintainer to decide on advisory or hardening commit)

**S1: low.** `backend/internal/crowdsec/hub_sync.go:951-971` (`extractTarGz` file open). The old flow emptied the target before extracting; the new documented overlay design does not. File creation uses `O_CREATE|O_TRUNC` without a symlink check on the destination, so it follows any pre-existing link in the live tree. The archive is hub-sourced and cached, and symlink entries in the archive are already rejected, so exploitation needs a hostile hub archive plus an existing link. The spec's "residual risk" note covers overlaying but not this write-through. Suggested hardening: `Lstat` the destination (and ideally its parents) and refuse symlinks, or open with `O_NOFOLLOW`.

**S2: low.** `backend/internal/api/handlers/crowdsec_handler.go:793` (`extractArchive`). Same pattern: `OpenFile` without `O_EXCL` or `O_TRUNC`, which can leave stale trailing bytes when an archive repeats a name. The tree is cleared first, so impact is limited to duplicate entries. Fix: `O_CREATE|O_WRONLY|O_TRUNC` plus a symlink check.

### Medium and low non-security (for the maintainer to file)

- **F1: low.** `backend/internal/crowdsec/hub_sync.go:954-957`. The containment check compares against `rel == ".."` and uses `HasPrefix(cleanName, "..")`, which also rejects legitimate names starting with `..` (for example `..foo`). Cosmetic and safe, since it errs on the strict side.
- **F2: low (information exposure to authenticated admin).** `crowdsec_files.go:300` and `:325` return `err.Error()` to the client on 500, which can embed absolute server paths. Other branches use fixed messages. Suggested: generic message and log the detail with `fileErrorLog`.
- **F3: low.** The `backup` field in the `WriteFile` (`crowdsec_files.go:391`) and `ImportConfig` (`crowdsec_handler.go:727`) responses returns an absolute server path. This is consistent with the existing preset-apply behavior and with UI expectations, so it is likely intentional. The extraction error response in `crowdsec_handler.go:715` also interpolates the archive entry name and `%v` error text.
- **F4: low.** `ReadFile` (`crowdsec_files.go:319`) has no read-size cap (the write cap is 1 MiB). The risk is small because the endpoint is admin-only and the tree is config-sized.
- **F5: info.** `local_api_credentials.yaml` and `online_api_credentials.yaml` are write-protected but readable and listed by the editor. This matches the design where `isWritable` is a subset of `isReadable`; confirm that exposing them to admins in the UI is intended.
- **F6: low (test quality).** Known and accepted: #1526 (tolerant Playwright specs).

## Out of scope and not verified

- Codecov manual-trigger behavior cannot be tested locally.
- Full-suite and cross-browser E2E are deferred to CI.
- CodeQL and a container-image Trivy scan are deferred to CI.
