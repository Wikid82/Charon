# QA and Security Report: feat/crowdsec-export-redaction

Date: 2026-10-11
Scope: full diff `origin/development...HEAD` (8 branch commits plus the 2 QA commits listed below) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved spec for blanking sensitive values in exported CrowdSec notification files (#1551). The branch rewrites notification files inside the export so they keep their names, types and structure but not their sensitive values, leaves out stray non-YAML files in notification folders, exports a file it cannot rewrite safely as an empty file, and makes the import fill blanked values from the files being replaced when the notification has the same type, name and target. The import card lists the files that still need values.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Two low findings (L1, L2) and three informational notes are listed; none blocks the merge. No model, migration or GORM query file is in the diff, so the GORM scan was not required.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Build | `cd backend && go build ./...` and `cd frontend && npm run build` | pass |
| Backend tests and coverage | `scripts/go-test-coverage.sh` | all packages pass, 0 failing tests; statements 91.5%, lines 90.5% (gate 87%) |
| Race run | `go test -race -timeout 60m ./internal/crowdsec/... ./internal/api/handlers/...` | pass (crowdsec 109.3 s, handlers 381.7 s) |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` (v2.14.0) | 0 issues |
| Frontend type-check | `cd frontend && npm run type-check` | pass |
| Frontend tests and coverage | `scripts/frontend-test-coverage.sh` | 298 files, 3683 passed, 4 skipped, 2 todo, 0 failed; statements 90.36%, branches 83.83%, functions 88.34%, lines 91.5% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (strict, run after the coverage runs) | overall 96.5% (737/764), backend 96.4% (731/758), frontend 100% (6/6), agent n/a; artifacts `test-results/local-patch-report.{md,json}` produced |
| Fuzzing | `go test ./internal/crowdsec/ -run '^$' -fuzz FuzzRedactNotificationFile -fuzztime=120s` | 536,037 executions, 463 corpus entries, no failure |
| CodeQL | `scripts/pre-commit-hooks/codeql-go-scan.sh`, `codeql-js-scan.sh`, then `codeql-check-findings.sh` (CLI 2.27.2) | Go: 4 results, all 4 pre-existing, in files outside this diff, covered by `codeql-suppressions.yml`; JS: 0 results; 0 blocking |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on the Go and frontend dependency files (copied under `$HOME` because the snap trivy cannot read `/projects`, removed afterwards); the diff does not touch `go.mod`, `go.sum` or `package.json` | no HIGH or CRITICAL |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| E2E | rebuilt `charon-e2e` with `docker-rebuild-e2e` (the user's `charon` container was not touched); `npx playwright test tests/security/crowdsec-export-redaction.spec.ts tests/security/crowdsec-import.spec.ts tests/security/crowdsec-diagnostics.spec.ts tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts --project=security-tests` | 93 passed, 0 failed, 0 skipped or fixme (1.3 min) |

Patch coverage detail (changed lines still uncovered): `export.go` 84.3% (16 lines: error branches on a file just opened or stat-ed, which need fault injection), `redact.go` 96.4% (8 lines: defensive encode and walk errors), `crowdsec_handler.go` 96.6% (1 line), `redact_merge.go` 99.3% (2 lines).

## Security review

Probes were run as throwaway Go tests (about 60 hostile or odd notification files, a 19-file export tree, merge round trips of 26 awkward values, size and node-count limits); they were deleted afterwards, and the permanent suite already covers the same ground with goldens, a property test and the fuzzer.

### 1. No sensitive value survives in the export

- Values: every value outside the per-type allowlist is replaced, whatever its YAML spelling (plain, quoted, block, folded, number, bool-like, null, binary, timestamp, URL with credentials, path or query parts, header values of any shape). Unknown plugin types, wrong-cased or quoted types, a type that arrives through an alias, and keys that differ only by case or by look-alike letters all get the strictest treatment (common keys only).
- Text around values: comments of every kind (head, line, foot, trailing document comments), tags (custom, verbatim, directive-defined, binary) and anchor names never reach the output; anchors are renumbered and aliases follow. Merge keys (single, list, inline mapping, into `headers`) are walked in the merging context; when the shared definition has been blanked away the output no longer parses and the file is exported empty.
- Fail closed: duplicate keys, complex keys, expanding alias chains, recursive anchors, unparsable or non-UTF-8 input, more than 50,000 nodes, a file over 1 MiB and a residual-value match all give the empty-file result; the original bytes are never a fallback. Redacting an already redacted file is byte-identical (idempotent), checked for every probe.
- Archive level: files in notification folders at any depth, with any letter case of the folder or the extension, a folder moved by the configuration inside the data folder, flat and nested layouts, look-alike folder names, files with an oversize or binary body, and links (files and folders) were exported without a planted value. Non-YAML files and editor leftovers inside a notification folder are left out; a name with a trailing space is not a YAML name and is left out.
- Documented limits confirmed by probe (unchanged from the spec): key text of unknown top-level keys, unknown children of `rotate` and header names stays readable; `format` templates and other kept plain settings are exported as written; other files in the configuration (for example `config.yaml`) are not rewritten.

### 2. Import merge

Fills only placeholders, only when type and name match exactly one current definition and every target key (URL, host, port, transport and certificate settings; every non-common key for unknown types) is absent on both sides or unchanged, with an exact comparison. Documents holding aliases or merge keys are never merged and are listed. Values are copied into nodes (26 awkward live values, including line breaks, document separators, directives, anchors and tags, round-tripped to the same value in a single document). Rewrites go to a new exclusive temp file renamed over the old one, mode capped at 0600 and at the current file's mode; links in the path are refused. The merge runs after extraction and before the companion restore; an error rolls the import back with a fixed message. The current files are located from the snapshot's own configuration, never from the archive's.

### 3. Logs, errors and responses

Rewriter errors carry fixed reasons only; log lines carry sanitised relative names or counts; `needs_values` carries sanitised file names only (capped at 50, sorted); a failed scan omits the list and does not undo the import. Tests assert that planted values never appear in captured logs.

### 4. Resource bounds

Size (1 MiB), node count (50,000), the residual scan (bounded work, otherwise the empty-file result) and per-tree file count (256) are enforced. Measured: a 400 KB file of 45,000 list items redacts in 55 ms; a 9,000-level nested file in 14 ms; a file of 20,000 distinct blanked values returns the empty-file result in 0.95 s. See L2 for the one super-linear case.

### 5. Frontend

The import card renders names as React text after `sanitizeKeptFileName` (control, format and separator characters stripped, length capped), lists at most 10 names plus a count, adds no toast, banner or button, and clears the line when another file is chosen. All five locales carry the new keys (parity test passes).

### 6. Public wording

Commit subjects and bodies, docs, code comments and test names were checked for vulnerability classes, attack scenarios and the phrases "tampered archive" and "destination binding": none found. One test variable was named after an attack pattern and was renamed (commit below). The only `(security)` commit subject is `feat(security): protect sensitive values in CrowdSec exports`, which is generic. No session id or link appears in any message.

## Findings

No critical, high or medium findings.

### Low

- **L1: low (hardening, spec-conformant).** The second-layer check that blanks a notification-shaped YAML file found outside the recognised notification folders looks only at the first document and only at a plain (unquoted, non-aliased) type value. This matters only when a notification folder cannot be located from the configuration (for example a folder setting that is built from an environment reference), because inside a located folder every YAML file is blanked regardless of its content. Recommended: treat a quoted type and every document of the file the same way as a plain first-document type (this only adds blanking). Tracked as a follow-up; the docs already say other files in the configuration are exported as written.
- **L2: low (availability, administrator only, pre-existing root cause).** `ParseYAMLDocuments` takes super-linear time on files with very many aliases: measured 1.3 s for 208 KB, 4.6 s for 430 KB and 36.7 s for 1 MiB (then rejected as excessively aliased). The export, the import merge and the import report now parse YAML files too, so this cost is paid in more places, while holding the data lock. Needs write access to the configuration (administrator). Recommended: reject a file early when its alias count is high (a cheap pre-scan), and skip the notification-shape probe for files that do not mention a `type` key.

### Informational

- **I1.** Merge and report stop examining after 256 notification files; any beyond that are left as extracted and not listed. Unreachable in practice (the image ships no notification plugins).
- **I2.** Charon's full backups (the backup archive of the database and the CrowdSec folder) and the pre-import snapshot copy the CrowdSec folder unchanged by design (host-local, full fidelity). This feature covers the CrowdSec configuration export only.
- **I3.** The unquoted-type and first-document behaviour of L1 and the key-text, `format` and `config.yaml` limits are the same documented limits as in the spec and the feature guide.

## Known limitations (not re-opened)

Database settings in the exported `config.yaml` and other private files in the configuration tree are tracked separately (spec follow-ups F1 and F2); the image ships no notification plugins.

## Commits added by this review

- `test: tidy a variable name in the CrowdSec notification rewriter tests` (renames a local test variable; no behaviour change).
- `docs: update QA report for the CrowdSec export protection` (this report).
