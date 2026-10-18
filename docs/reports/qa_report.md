# QA Report: fix/crowdsec-acquisition-and-backup-names

Date: 2026-10-18
Scope: 3 commits (atomic and serialised acquisition PUT, microsecond backup names, Top IPs ordering fix).

## Verdict

PASS with Minor findings. No Blocking issues. One Major-adjacent item is listed under Minor because the layout it affects is not a supported deployment.

## Results

| # | Check | Result |
|---|-------|--------|
| 1 | `go vet ./...` | PASS |
| 2 | `go test -race` for `internal/crowdsec/...` and `internal/api/handlers/...` | PASS |
| 3 | Backend coverage script (min 87%) | PASS, 92.1% statements, 91.1% lines |
| 4 | `scripts/local-patch-report.sh` | WARN, backend patch 56.9% (see M3); frontend 85.7% |
| 5 | Vitest for ActiveDecisionsTable (13 tests) | PASS |
| 6 | Frontend type-check | PASS |
| 7 | ESLint on touched files | PASS, 0 errors, 7 pre-existing style warnings |
| 8 | markdownlint | N/A, only this report touched docs |
| 9 | Container gate 1: PUT against real layout, mode preserved | PASS |
| 10 | Container gate 2: 10 concurrent PUTs | PASS |
| 11 | Container gate 3: same-second backups, legacy ordering and pruning | PASS |
| 12 | Single-file bind-mount layout | FAIL (see M1) |

## Real-process gate

Throwaway image built from the repo Dockerfile, container started with the default entrypoint setup. In the image `/etc/crowdsec` is a symlink to `/app/data/crowdsec/config`, and `acquis.yaml` is a regular file in that directory. The documented compose file has no single-file mount of `acquis.yaml`.

1. PUT on the real layout: HTTP 200. With the live file set to mode 0640, the new file stayed 0640 and the backup was 0640.
2. Ten simultaneous PUTs: all returned 200. The directory held exactly one valid payload (`p5.log`), no `.tmp-*` leftovers, and exactly 5 backups.
3. Backups created in the same second had distinct microsecond names (for example `...101729.746609`, `...101729.857217`, `...101729.870612`). Three planted legacy-named backups (two old, one in the same second as new backups) were pruned first, and the 5 newest backups were kept in order.

## Findings

### Minor

- M1. Single-file bind mount of `acquis.yaml` now returns 500. The rename over a bind-mounted file fails with EBUSY. The old code ignored the failed backup rename and wrote in place, so this is a behaviour change. The entrypoint already fails on a fresh volume with such a mount (`cp: can't create ... File exists`). It only works when the config directory is pre-initialised, so this is not a supported layout and I did not rate it Blocking. Suggested fix: on EBUSY or EXDEV from `os.Rename`, fall back to an in-place write of the temp content, or document that the file cannot be bind-mounted on its own.
- M2. A failed replace leaks its backup. In `replaceAcquisitionConfig` (`backend/internal/api/handlers/crowdsec_acquisition_write.go`), the `return "", fmt.Errorf(...)` statements set the named `backupPath` to `""` before the deferred cleanup runs, so `removeIfSet(backupPath)` removes nothing. Observed in the bind-mount run: HTTP 500 left `acquis.yaml.backup.20261018-101907.243126` behind. The live file and temp file were cleaned correctly, and pruning bounds the leak. Suggested fix: capture the backup path in a separate local variable for the defer. Add a unit test that forces a rename failure.
- M3. Backend patch coverage is 56.9% (target 85%). Uncovered lines are the error branches of `crowdsec_acquisition_write.go` (lines 25-26, 33, 44-62, 68-69) and the lock-cancel branch in `crowdsec_handler.go` (2329-2331). Overall backend coverage is fine. Adding tests for these error paths would also have caught M2.

### Not findings

- The mode of a newly created acquisition file changed from 0600 to 0644. This is intentional and matches the image default.
- CodeQL and Trivy were not run locally (`fix:` scope, deferred to CI).

## Cleanup

Throwaway containers, volumes, the image and temp files are removed. The live `charon` container was untouched. `changelog.json` was not polluted. Nothing was staged or committed.
