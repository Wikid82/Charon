# QA and Security Report: fix/crowdsec-hardening-batch5

Date: 2026-10-16
Scope: full diff `origin/development...HEAD` (9 branch commits plus the QA commits listed below) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved spec. The branch keeps private key files out of CrowdSec exports and puts the current ones back on a same-host import, bounds the cost of reading YAML, uses the shared mask for the bouncer registration key preview, makes the cached preset preview route reachable for `owner/name` slugs, and gives import refusals stable error codes with translated messages.

## Verdict: PASS (no blocking findings)

No critical or high findings. Three low findings and four informational notes are listed; none blocks the merge. No model, migration or GORM query file is in the diff, so the GORM scan was not required.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Frontend tests and coverage | `scripts/frontend-test-coverage.sh` | 300 files, 3724 passed, 4 skipped, 2 todo, 0 failed; statements 90.41%, branches 83.94%, functions 88.38%, lines 91.54% (gate 87%) |
| Backend tests and coverage | `scripts/go-test-coverage.sh` | 0 failing tests; statements 92.0%, lines 91.0% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (run after both coverage runs) | overall 90.5% (315/348), backend 89.8% (292/325), frontend 100% (23/23); artifacts `test-results/local-patch-report.{md,json}` produced. Lowest file: `keyfiles_restore.go` 82.4%, the uncovered lines being defensive error returns |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` | 0 issues |
| Strict lint on touched packages | `golangci-lint run --config ../.golangci-fast.yml ./internal/crowdsec/... ./internal/api/handlers/...` | 0 issues |
| Build | `cd backend && go build ./...` | pass |
| Type check and build | `cd frontend && npm run type-check && npm run build` | pass |
| Race run | `go test -race -timeout 60m ./internal/crowdsec/... ./internal/api/handlers/...` | pass (crowdsec 118.5 s, handlers 447.2 s) |
| CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CLI 2.27.2) | 4 results, all the known baseline entries in files outside the diff (`auth_handler.go`, `remote_server_handler.go` x2, `uptime_service.go`); 0 new |
| CodeQL JS | `scripts/pre-commit-hooks/codeql-js-scan.sh` | 0 results |
| Trivy | `trivy fs --scanners vuln --severity HIGH,CRITICAL` on the Go and npm manifests (the snap build cannot read the repo path, so the manifests were copied to a home directory and removed afterwards) | 0 findings |
| Markdown lint | `make lint-markdown` | 0 issues in 265 files |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| E2E (E2E container rebuilt; the `charon` container was not touched) | `npx playwright test tests/security/crowdsec-export-redaction.spec.ts tests/security/crowdsec-import.spec.ts tests/security/crowdsec-hub-preset-apply.spec.ts tests/security/crowdsec-preset-apply.spec.ts --project=security-tests` | 82 passed (1.1 min) |

## Security review

Throwaway probes (Go tests, deleted afterwards) were run against the exported functions and the router.

1. Export key-file exclusion. Files named by an absolute `key_path` or `*_key_path`, names ending in `.key`, `.p12` or `.pfx` (any case), and files whose first 64 KiB hold private key armor (RSA, EC, OPENSSH, ENCRYPTED, PKCS#8, PGP block, CRLF line ends, leading whitespace, armor inside a YAML block or an escaped YAML string) were all left out. Symlinks and linked directories are skipped, a hard link to a key file is caught by the content rule, and the content check runs on the descriptor that is copied. Certificates and public keys are still exported. The omitted paths are returned in the `X-Charon-Omitted-Files` header (at most 50).
2. Restoring key files on import (`RestoreKeyFiles`). Only files that match the export's rules are put back, only at paths that do not exist in the new tree (the archive's file, a link or a non-directory in the way always wins), parents are checked with `Lstat` and never followed, files are created with `O_EXCL|O_NOFOLLOW`, the mode is capped at 0600, and the limits (20000 scanned entries, 64 files, 1 MiB each) fail the import and roll back. A probe with a linked parent directory, a link at the destination and a FIFO in the way wrote nothing outside the data folder and restored nothing at those paths; modes 0777 and 0400 came back as 0600 and 0400. Log lines carry file base names only, never content. The snapshot is the server's own pre-import copy, so archive content cannot choose what is restored or where.
3. YAML cost bound. The reader cap (2 MiB) and the memoised weight pass (1,000,000 nodes, loop guard) refuse alias-expansion ladders, self-referencing anchors, over-long streams and wide flat lists. Worst measured parse time on a 2 MiB input was 1.65 s (refused as too complex) and 1.4 s for an accepted 2 MiB list of small maps; deeply nested input (5,000 levels) took 10 ms. All callers treat a parse error as a refusal: redaction exports the file empty, the editor says "invalid YAML", the import validation refuses, look-alike detection treats the file as not a notification file.
4. Route `*slug`. Probes for a trailing slash, `..`, encoded and double-encoded separators, a backslash, a NUL byte, a doubled leading slash, upper-case slugs and an empty slug all returned 404 or 400 without touching the cache directory (slug validation is unchanged); `owner/name` and `owner%2Fname` return 200. The sibling `pull` and `apply` routes are unaffected.
5. Error codes. The six `error_code` values are fixed constants; `names` goes through the existing control-character stripping and length cut and is capped at 50 entries; the English `error` text is unchanged. The frontend renders names as plain text in a toast and ignores unknown codes.
6. Docs checked against the code: the omitted-files header name and its 50-file cap, the 64 file and 1 MiB restore limits, the 2 MiB YAML cap, and the message texts. Four statements were wrong or too loose and were corrected (commit below).

## Findings

No critical or high findings.

### Low

- **L1.** The key-armor content rule only looks at the first 64 KiB. A text-format key that starts after 64 KiB of other content, or whose header line straddles that boundary, is not detected by content (it is still caught by a `.key`/`.p12`/`.pfx` name or by an absolute `key_path`). This is the specified behaviour; the docs now say "within its first 64 KiB". Remediation if wanted: scan overlapping windows or the whole file up to the 1 MiB inspect cap.
- **L2.** A binary key container without a matching name (for example DER bytes in `.pem`) that is also not named by an absolute `key_path` is exported. A `key_path` that is relative, contains `..`, or is set only through a YAML merge key is not resolved to a target (relative paths cannot be resolved statically; `..` is rejected rather than cleaned). Remediation: clean `..` inside an absolute path instead of rejecting it, and read merge keys in `keyPathsOf`.
- **L3.** The server log records only how many key files were left out, not their names, so the header is the only place the names appear (the UI does not show it yet). Not a defect; noted so the docs match.

### Informational

- **I1.** The restore scan counts every regular, non-excluded file of the snapshot against the 20000 limit; an install with more than that many configuration files fails the import closed with the "could not be kept" message.
- **I2.** A parse error in a look-alike notification file outside the notification folders keeps the earlier behaviour (the file is not treated as a notification file); the new cost limits only add more inputs that take this path.
- **I3.** The Playwright and unit tests already cover the main branches of the new code; the probes above were not added as tests because they duplicate existing ones.
- **I4.** Pre-existing CodeQL log-injection and cookie results (4) remain in untouched files.

## Commits added by this review

- `docs: correct the CrowdSec export key file notes` (omitted-files header name and cap, restore limits, the 64 KiB wording, removal of a "too complex" log claim that the code does not make).
- `docs: update QA report for the CrowdSec hardening batch`.
