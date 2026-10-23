# QA Report: fix/crowdsec-backup-retention-and-acquis-write

(Replaces the previous report.) Audited on 2026-10-23 at HEAD `451919f1`.

**Scope:** 3 commits over `origin/development` (`88ca4ca7`): `f1d94212` (J1, keep CrowdSec backups that a failed rollback left for recovery), `60de6702` (N1 owner preservation, N2 stale temp sweep, N3 directory sync), `451919f1` (save in place whenever the owner cannot be kept, plus the retention edge-case tests). 14 files, +1154/-17. Backend only, plus one line in `docs/troubleshooting/crowdsec.md`. No model, migration, API shape or frontend change.

**Audited against:** the `fix/crowdsec-backup-retention-and-acquis-write` spec section 3f in `docs/plans/crowdsec_hardening_batch5_spec.md`, the Definition of Done in `CLAUDE.md`, and the backend security rules in `CLAUDE.md`. For `SECURITY.md`, this audit checked the applicable items (path handling, error wrapping, log sanitising, no Gotify tokens). It was not a full line-by-line review of every `SECURITY.md` section, which covers supply chain and infrastructure items outside this change.

**Public changelog:** the subjects are plain `fix:` (no `(security)` scope). That is correct for a reliability fix with no vulnerability content.

## Verdict

**PASS with findings.** Every gate that can run here passes. The container gate passes on 4 of 5 items. Item 4 (the live rollback-failure trigger) is covered by unit tests and a simulated marker, not by a live failing rollback. One spec deviation is worth a decision before merge (F1). No CRITICAL or HIGH findings.

## Gate results

| # | Gate | Result | Evidence |
|---|------|--------|----------|
| G1 | `go build ./...` (backend) | PASS | Built cleanly. |
| G2 | `go test -race -count=1 ./internal/crowdsec/... ./internal/api/handlers/...` | PASS | crowdsec 122.8s ok, handlers 478.7s ok, zero failures. |
| G3 | `scripts/go-test-coverage.sh` (gate 87%) | PASS | Line coverage 91.2%, statements 92.2%. Backend gate met. |
| G4 | `scripts/local-patch-report.sh` | PASS | Backend patch coverage 100% (160/160 changed lines). `test-results/local-patch-report.md` and `.json` both present. |
| G5 | `lefthook run pre-commit` | NOT MEANINGFUL | Exit 0, but every hook reported "skip: no matching staged files" because nothing is staged on this branch. Substituted: `golangci-lint run --config .golangci.yml ./internal/crowdsec/... ./internal/api/handlers/...` gives 0 issues (includes gosec, which lefthook defers to CI). `markdownlint` was not run on the one changed docs line. |
| G6 | `govulncheck ./...` | PASS | 0 vulnerabilities in reachable code. One module-level advisory exists in a required module that this code does not call (informational, not introduced here). |
| G7 | GORM security scan | NOT TRIGGERED | No `backend/internal/models/**` or query changes in the diff. |
| G8 | Gotify token review | PASS | No Gotify references in the diff, test artifacts or new logs. |
| G9 | CodeQL and Trivy | DEFERRED TO CI | `fix:`-only change with no new feature surface, per CLAUDE.md. |
| G10 | Frontend coverage and type-check | SKIPPED | Frontend untouched. |
| G11 | Targeted Playwright, `--project=security-tests`, Firefox only | PASS | `crowdsec-file-editor`, `crowdsec-import`, `crowdsec-diagnostics`, `crowdsec-hub-preset-apply`: 94 passed, 0 failed. Ran against a throwaway container, not the live `charon`. Full suite and cross-browser runs deferred to CI. |
| G12 | Changelog file restored after Playwright | PASS | `backend/internal/changelog/data/changelog.json` restored; working tree clean. |

## Container gate (throwaway image `charon-qa-acq:451919f1` built from this branch)

Containers were created with unique names on loopback-only ports, and each has its own unique volume. The live `charon` container, its volumes, its compose files and its `charon:local` image tag were never touched. The E2E rebuild skill was deliberately not run, because it retags `charon:local` and rewrites `.env`.

| Item | Result | Evidence |
|------|--------|----------|
| 1. Save as default non-root user, root-owned `acquis.yaml` | PASS | (a) Root-owned 0644, process `charon` (uid 1000): save returned 200. Mode kept 644. Content correct. Owner changed root to charon via the rename fallback, with the expected Warn log (see F2). No stray temp files. (b) Root-owned `root:charon` 0660: save returned 200. Owner kept as root, mode kept 660, inode unchanged (542975 before and after), content correct, backup written, no temp files. |
| 2. Same save with the container started as root (`--user 0:0`) | PASS, with limitation | Root-owned 0644 file saved with 200, same behaviour as 1(a). The Charon process still runs as uid 1000 because the entrypoint drops privileges, so the `processOwner == root` branch is only covered by unit tests (see F4). |
| 3. Stale temp sweep | PASS | Seeded `acquis.yaml.tmp-STALE` (regular, 30 min old) was removed. `acquis.yaml.tmp-FRESH` (regular, new) kept. `acquis.yaml.tmp-LINK` (symlink) kept. `acquis.yaml.tmp-DIR` (stale directory with the same prefix) kept. Unrelated `target` file kept. |
| 4. Failed hub apply rollback: snapshot and `.keep` survive pruning and restart | PARTIAL | Live rollback failure was not reproduced (see F5). Marker written in the exact format `RetainSnapshot` uses, beside a snapshot directory. Then 6 successful applies, a container restart, and 2 more applies: the marked snapshot and its `.keep` survived every prune. Contents intact. |
| 5. Normal successful apply still prunes old snapshots | PASS | Two unmarked planted snapshots were pruned during the 6 applies. Afterwards exactly 5 unmarked snapshots remained, and the marked snapshot was not counted against the limit. |

## Findings (ranked)

### F1. MEDIUM. Spec deviation: any chown failure falls back to a non-atomic in-place write

- **Where:** `backend/internal/api/handlers/crowdsec_acquisition_write.go`, `keepOwner` (any error from `tmp.Chown` is returned as `ownerErr`) and `replaceAcquisitionConfig` (any `ownerErr` leads to `writeInPlace`). The handler test at `crowdsec_acquisition_write_test.go:273-276` asserts that EPERM, EINVAL, ENOTSUP, EROFS and EIO all take the in-place path.
- **Spec:** section 3f.2 (N1) requires EPERM only to take the in-place path. Other errors must fail the save with the live file untouched and the backup removed. Test 3f.5(C3e) asks for the same.
- **Impact:** an EIO or EINVAL from `chown` now overwrites the live file in place. That is not atomic. A crash or power loss mid-write can leave a truncated CrowdSec acquisition file. The backup is kept only when the in-place write itself fails, so on success it is the only fallback.
- **Remediation:** choose one, then align code, tests and spec. Either (a) narrow the in-place condition to `fs.ErrPermission`/EPERM as the spec says, and make other chown errors fail the save, or (b) keep the broader behaviour, update spec 3f.2 and C3(e), and document the non-atomic window in the function comment. Option (a) is the closer match to the approved design.
- **Not a vulnerability.** The file is operator config and the write stays inside the validated path. This is reliability only.

### F2. LOW. Owner change when the live file is root-owned and not writable by charon

- **Observed:** a root-owned `root:root 0644` file. The in-place open was denied, so the rename fallback ran and the owner changed to `charon:charon`. The Warn log line was written. CrowdSec still reads the file (mode 644).
- **Spec:** section 3f.2 accepts this fallback. The acceptance wording "owner preserved or in-place fallback" is met only for the `root:charon 0660` variant.
- **Security note:** no privilege change beyond what the process already had. Charon could replace the file through the directory anyway, because the rename needs only directory write permission.
- **Remediation:** document the case in the troubleshooting doc, or have the save refuse and explain when the owner cannot be kept, if operators need strict ownership.

### F3. LOW. Pre-existing, not introduced by this branch: unreadable live file gives an opaque 500

- **Observed:** a root-owned `root:root 0640` file that the charon process cannot read returns 500 `failed to read acquisition config` on GET and 500 `failed to write acquisition config` on PUT. The failure happens at the read step, so nothing is written and there is no data loss.
- **Where:** `os.ReadFile(path)` in `replaceAcquisitionConfig`, and `readAcquisitionConfig` in the GET handler. Both exist on `development` too.
- **Remediation:** return a clear 403 or a specific message for a permission error, and file as a low-priority issue. This is not a security finding.

### F4. LOW. Coverage limitation: the root-owner branch is not exercised live

- The entrypoint drops privileges, so the Charon process is always uid 1000 in the image, even when the container starts as root. The `processOwner == root` branch of `keepOwner` is therefore covered only by unit tests.
- **Remediation:** none needed for this PR. Note it in the test plan.

### F5. LOW. Live rollback-failure trigger not reproduced

- The rollback-failure paths (`rollbackFailure`, the import rollback closure, and `recoverSnapshot`) call `RetainSnapshot` through `RetainForRecovery`. They are covered by unit tests and by the 100% patch coverage. A live failing install plus failing restore could not be forced offline in the throwaway container without a mock.
- **Remediation:** none required for merge. A future fault-injection test could cover it end to end.

### F6. INFO. Retention markers are unbounded by design

- A snapshot with a `.keep` file is never pruned automatically. Markers are written only on a double failure (failed apply plus failed restore), so growth should be rare. Orphan markers are cleaned up on the next prune. The troubleshooting line documents the manual removal step.
- **Remediation:** accept. Consider a size or count warning in the admin UI later, subject to the "Passive UI, No Nudges" rule.

### F7. INFO. Commit slicing differs from spec 3f.4

- Spec commit 2 (`refactor:`, seams only, no behaviour change) and commit 5 (`docs:`) are not separate commits. The N4 seams landed inside the N1 and N2 commits, and the docs line is inside the J1 commit. Each commit still builds and passes its own tests as far as this audit could tell, but future reviewers should expect this difference.

### F8. INFO. Operational observation, not investigated

- After the throwaway container was restarted, the first authenticated requests returned 401 until the user logged in again. This is probably expected session behaviour across a restart, but the cause was not confirmed. The CrowdSec entrypoint also logged "Fixing CrowdSec file ownership..." during the run. Neither is related to this change.

## Verified, no finding

- **Path handling:** `replaceAcquisitionConfig` runs `filepath.Clean` and the path comes from the validated `resolveAcquisitionConfigPath`. The temp file and the backup are created beside the live file, so there is no traversal.
- **Symlinks:** the stale sweep uses `Lstat` and removes only regular files. Symlinks and directories are kept (confirmed live).
- **Retain marker:** written with `O_EXCL` and mode 0600, outside the snapshot directory, so a restore cannot copy it into DataDir. It is a sibling file (`<snapshot>.keep`). A symlinked or missing snapshot is not marked.
- **Directory sync:** `SyncDir` treats EINVAL, ENOTSUP and `ErrUnsupported` as success. Other errors are logged and never fail the save.
- **Error handling and logging:** errors are wrapped with `%w`. Paths in logs go through `util.SanitizeForLog`. Responses use `gin.H{"error": ...}` and carry no absolute paths.
- **Typed-nil seams:** the default constructors return a literal nil interface on error, as the spec requires.
- **Gotify:** no tokens in the diff, artifacts or logs.

## Remediation tracking

- F1 is the only item that should block merge, and only if the team wants the spec to hold as written. Decide before the merge commit.
- F2 and F3: F3 is a pre-existing issue and should be filed as a low-priority public GitHub issue. F2 can be covered in the troubleshooting docs.
- F4 to F8: accept, or track as informational.
- No security-sensitive finding was identified, so no private advisory is needed.

## Cleanup

Throwaway containers, volumes, the `charon-qa-acq:451919f1` image and the QA-only files under `/var/tmp` were removed after this report was written. Nothing was committed, pushed or staged. The spec file `docs/plans/crowdsec_hardening_batch5_spec.md` was not staged.
