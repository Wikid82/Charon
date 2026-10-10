# QA and Security Report: fix/crowdsec-export-import-followups

Date: 2026-10-10
Scope: full diff `origin/development...HEAD` (HEAD 6dcc71de, 5 commits, 27 files) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the local plan for the CrowdSec export and import follow-ups (#1532, #1533, #1534). Parts: (A) the export omits engine-owned data and stored account details, and import keeps them (including on rollback); (B) the import locates `config.yaml` at the top level or in `config/` (a real-layout export now re-imports) and validates it by parsing; (C) a single `.tar.gz`-only message and UI hint; (D) the translated "Check Now" buttons.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Three low observations (F1-F3) and two informational items (F4-F5) are listed for the maintainer; none blocks merge. All Definition of Done gates pass.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | pass |
| Backend vet | `go vet ./...` | pass, no output |
| golangci-lint, backend | `golangci-lint run --config .golangci.yml ./...` | 0 issues |
| golangci-lint, agent | `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...` | 0 issues |
| Frontend type-check | `npm run type-check` | pass |
| Frontend build | `npm run build` | pass |
| Frontend unit tests (touched) | `npx vitest run src/pages/__tests__/CrowdSecConfig* src/pages/__tests__/ImportCrowdSec* src/__tests__/i18n.test.ts` | 10 files, 113 tests passed |
| Race tests, crowdsec package | `go test -race ./internal/crowdsec/... -count=1` | ok (106 s) |
| Race tests, handlers | `go test -race -run 'Import\|Export\|Rollback' ./internal/api/handlers/... -count=1` | ok |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query files in the diff (0 matches) |
| Lefthook | `lefthook run pre-commit --all-files` | all 17 hooks pass, including semgrep (0 findings, 367 rules; it logged one scan-timeout note on a large test file, which is a coverage note, not a finding). Working tree stayed clean. |
| govulncheck | skill `security-scan-go-vuln` | 0 affected; 1 module-level advisory in a required module that Charon code does not call |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum (backend, agent), package-lock.json and Dockerfile | no HIGH/CRITICAL. The snap trivy cannot read `/projects`, so the manifests were copied under `$HOME` and removed afterwards. No container-image scan locally. |
| CodeQL | not run locally | `fix:`/`test:`/`docs:` scope, no new endpoint or feature surface, deferred to CI per CLAUDE.md |
| E2E | `npx playwright test <ten crowdsec specs> --project=security-tests` against a container rebuilt from HEAD (`docker-rebuild-e2e`) | 194 passed, 0 failed (9.2 min) |
| Coverage (given, not rerun) | prior runs | backend 91.2% stmt / 90.2% line, frontend 91.46% lines, patch coverage 93.8% |

## Check-by-check results

1. **Export contents.** `ExportConfig` now uses `filepath.WalkDir` and skips an entry when `crowdsec.IsPreservedPath(rel)` is true (engine-owned state or a stored secret). `fs.SkipDir` is returned only for excluded directories; excluded files return `nil`, so siblings are never dropped (`TestExportConfigSkippedSecretKeepsSiblings`, golden list in `TestExportConfigRealLayoutEntryList`). Top-level `data/` and `hub_cache/` are not descended into; live db files and the three account/connection files are excluded at any depth and in any letter case (`TestIsSecretPath` covers case, depth, `./` and `//` forms, a directory named like a secret, and near-miss names that stay non-secret). Symlinks are still skipped. The gzip exclusion is unchanged (`routes.go:1260`, not in the diff).
2. **Import.** Uploaded archives cannot write protected entries: `extractArchive` skips any entry whose cleaned relative path is preserved, which covers case variants, nested directories named like a secret, and `..` forms that clean onto a secret. Tar symlink and hardlink entries are ignored by the extractor (regular files and directories only), and file creation still goes through `CreateFileNoSymlink`. Live files survive success: `ClearConfigKeeping(DataDir, IsPreservedPath)` evaluates `keep` before the entry type, so a kept name survives whether it is a file, directory or symlink (`TestClearConfigKeepingKeepsSymlinkedSecret`). Rollback uses `RestoreKeeping`, which clears with the same predicate and `copyTree` skips kept entries, so a live protected file is never deleted or re-copied; verified by tests for "secret changed since snapshot keeps the newer value", "restore copy failure does not delete live secrets" and `TestImportConfigFailureKeepsSecretsAndRestoresConfig`. A legacy archive that contains the protected files imports cleanly without replacing the live ones (`TestImportConfigNeverTakesProtectedEntriesFromUpload`). The real layout round trip (`config/config.yaml`) succeeds (`TestExportThenImportRealLayoutRoundTrip`, and live in E2E: "an export from this instance re-imports").
3. **`FindConfigFile` and callers.** The helper prefers `config/config.yaml`, then the root file, else an empty string. `console_enroll.go` and `heartbeat_poller.go` are one-line delegations to it and are behavior-equivalent. In `DiagnosticsConnectivity` the `lapi status` site is equivalent; the `capi status` site gains the root fallback (benign, decided in the plan). In `DiagnosticsConfig` the traversal check still runs whenever a config is found, and the empty result keeps the `config.yaml not found` path unchanged (`TestDiagnosticsConfigLocatesConfigFile`, `TestDiagnosticsConnectivityPassesLocatedConfigToCscli`). `ImportConfig` returns the fixed `config.yaml was not found at the top level or in config/` when none is found (rollback runs first).
4. **YAML validator.** Verified: 1 MiB cap (`readCapped`, fixed message), mapping root required, at least one of the 8 `csconfig.Config` top-level keys, duplicate keys and non-string keys rejected (key types are checked on the nodes), first document only, empty file reported as empty, and all client messages are fixed strings (parser detail goes to the log only, through `sanitizeForLog`). The real default `config.yaml` fixture passes. **Alias and nesting abuse (probed with a temporary test, since removed):** a 420-byte document with 10 levels of 9-way aliases (about 9^10 expanded nodes) is rejected in under 1 ms with `config.yaml is not valid YAML` (yaml.v3: "document contains excessive aliasing", about 0 MB allocated); a 400 KB flow-sequence nesting bomb is rejected in 15 ms (6 MB allocated, "exceeded max depth of 10000"); 5000-deep nesting is accepted in 10 ms (3 MB). Together with the 1 MiB read cap, the import path cannot be exhausted this way. Note the probe went through `validateYAMLFile`; there is no existing unit test for alias input (see F3).
5. **ZIP/.tgz.** `requireTarGz` is the single format gate; every other extension (`.zip`, `.tgz`, `.txt`, none) returns `only .tar.gz archives are supported`. No zip code remains in the Go handler (the remaining "zip bomb" strings are comments and the compression-ratio message). Both file inputs use `accept=".tar.gz"`, and no ZIP wording remains in any locale.
6. **Preset apply.** `hub_sync.go` `extractTarGz` now skips `IsEngineOwnedPath || IsSecretPath` entries, with `TestExtractTarGzSkipsProtectedNamesAndKeepsSiblings` (siblings still extracted).
7. **Frontend.** `crowdsecConfig.checkNow` exists in all five locales (en exactly `Check Now`, de `Jetzt prüfen`, es `Comprobar ahora`, fr `Vérifier maintenant`, zh `立即检查`), both buttons use `t('crowdsecConfig.checkNow')` (lines 650 and 693), no hardcoded literal remains, and the i18n key test passes.
8. **E2E tolerance.** A grep over `tests/security/crowdsec-import.spec.ts`, `tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts` and `tests/security/crowdsec-diagnostics.spec.ts` for conditional assertions, `.catch(`, `expect([...])` status lists, alternation patterns, `toBeOneOf`, `expect.soft`, `test.fixme`, `.skip(`, `console.log` and `annotations` returned nothing. The diff adds one helper (`listTarGzEntries`) in `tests/utils/archive-helpers.ts`, which is in a directory excluded from the Docker build context.
9. **Commit subjects.** `git log origin/development..HEAD --format=%s`: `test(e2e): add fixme specs for the CrowdSec export and import changes`, `fix: slim down the CrowdSec configuration export`, `fix: validate CrowdSec configuration files by parsing them`, `fix: offer only .tar.gz in the CrowdSec import and translate Check Now`, `docs: describe the CrowdSec export and import behavior`. All generic, none uses the `(security)` scope, and a search of the full commit messages for secret, credential, key, overwrite, vulnerability and session terms found nothing. The docs sentence says "account and connection details are not included" without file names.
10. **Security-sensitive items.** None found that needs a private advisory. The change is a protective default plus an import correctness fix and is admin-authenticated throughout. See F1 for a related low observation.

## Findings

No critical, high or medium findings.

### Low

- **F1: low (docs wording, maintainer decision).** `docs/features/crowdsec.md:69` says an export "contains configuration only". The export still includes other configuration that can hold connection details of its own (notification configs and a database section of `config.yaml`), so the sentence could give slightly more assurance than is true. The plan lists this as a known follow-up. Suggested: soften to "does not include CrowdSec's own account and connection details" or similar, and file the tracked low issue for redacting those parts. Keep the public wording generic.
- **F2: low (pre-existing, out of scope).** `DiagnosticsConfig` and similar sites check containment with `strings.HasPrefix(cleanPath, cleanDataDir)` without a path separator (`backend/internal/api/handlers/crowdsec_handler.go`, the "Path traversal protection" blocks around lines 2195 and 2240). A sibling directory sharing the prefix would pass. It is unreachable here because the path is always built from `DataDir` plus fixed names. Suggested: use `filepath.Rel` or compare with a trailing separator when next touched.
- **F3: low (test gap).** No unit test feeds an alias-expansion or deeply nested document to `validateYAMLFile` (probed manually, behavior is safe, see check 4). Suggested: add a small regression test for an alias bomb in `crowdsec_config_validation_test.go` so a yaml.v3 upgrade that changes the limits is caught.

### Informational (for the maintainer to file or ignore)

- **F4: info.** Preset apply and file-edit flows still use plain `Restore`, which copies snapshot copies of the protected files back on rollback (the plan keeps this unchanged on purpose). A rollback in those flows could therefore restore an older copy of a file that changed since the snapshot. Same files as before this change; no regression.
- **F5: info.** The root-only `-c` argument builders (`crowdsec_handler.go`, the Start/Stop/status helpers around lines 542, 635, 1774, 1858, 1899 on the base branch) were not switched to `FindConfigFile`; `cscli` falls back to its default location, so nothing breaks. Optional DRY follow-up.

Planned follow-up issues from the plan (hub-item symlinks not restored by import, medium; export not serialized with import and truncated 200 on a mid-walk error, low) are unchanged by this PR and still need filing.

## Out of scope and not verified

- Full-suite and cross-browser E2E are deferred to CI; the targeted run used the single `security-tests` project.
- CodeQL and a container-image Trivy scan are deferred to CI.
- Coverage and local patch scripts were not rerun, as instructed (figures above are the earlier results).
