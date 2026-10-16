# QA and Security Report: fix/crowdsec-preset-handling

Date: 2026-10-16
Scope: full diff `origin/development...HEAD` (10 branch commits plus the 3 QA commits listed below) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved spec. The branch fixes hub preset apply, which reported success but installed nothing, and adds input validation around the hub preset cache and the hub download. Apply now installs the pulled item with the typed `cscli` command, checks that the installed result is the item that was pulled, and returns stable error codes for each outcome that is not a success.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Two low findings and three informational notes are listed; none blocks the merge. No model, migration or GORM query file is in the diff, so the GORM scan was not required. The real-cscli check done earlier was not repeated; the Go, handler and browser tests below show no regression.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Frontend tests and coverage | `scripts/frontend-test-coverage.sh` | 299 files, 3710 passed, 4 skipped, 2 todo, 0 failed; statements 90.39%, branches 83.9%, functions 88.36%, lines 91.53% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (run after the backend coverage run so the backend input is current) | overall 95.3% (624/655), backend 95.0% (588/619), frontend 100% (36/36); artifacts `test-results/local-patch-report.{md,json}` produced |
| Backend tests and coverage | `scripts/go-test-coverage.sh` | 0 failing tests; statements 91.9%, lines 90.9% (gate 87%) |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` | 0 issues |
| Strict lint on touched packages | `golangci-lint run --config ../.golangci-fast.yml ./internal/crowdsec/... ./internal/api/handlers/...` | 0 issues |
| Build | `cd backend && go build ./...` | pass |
| Type check and build | `cd frontend && npm run type-check && npm run build` | pass |
| Race run | `go test -race -timeout 60m ./internal/crowdsec/... ./internal/api/handlers/...` | pass (crowdsec 105.6 s, handlers 433.6 s) |
| CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CLI 2.27.2) + `codeql-check-findings.sh` | 4 results, all suppressed baseline entries in files outside the diff (`auth_handler.go`, `remote_server_handler.go`, `uptime_service.go`); 0 blocking |
| CodeQL JS | `scripts/pre-commit-hooks/codeql-js-scan.sh` + `codeql-check-findings.sh` | 0 results |
| Trivy | `trivy fs --severity HIGH,CRITICAL --scanners vuln,secret` | no findings |
| Markdown lint | `make lint-markdown` | 0 issues in 265 files |
| GORM scan | not run | no model, migration or GORM query file in the diff |
| E2E (E2E container rebuilt; the `charon` container was not touched) | `npx playwright test tests/security/crowdsec-preset-apply.spec.ts tests/security/crowdsec-hub-preset-apply.spec.ts --project=security-tests` | 16 passed (52.7 s) |

## Security review

1. Cache reads: file locations are derived from the slug, never from stored metadata. Entries are opened through `os.Root` with a non-blocking read-only open, the file type is checked on the open descriptor, and metadata and item sizes are capped. Throwaway probes (deleted afterwards) showed that a linked owner or entry directory, a linked item file, a special file in place of the metadata or item, an oversized file and unknown stored path fields all produce an invalid-entry result without reading outside the cache directory and without hanging; removal deletes a link and never its target; a store through a linked owner directory is refused.
2. Slug rule: `owner/name` with a restricted character set and a length cap. About two dozen malformed forms (parent references, extra separators, whitespace, control characters, non-ASCII, a leading dash, reserved file names, over-long names) were all refused; real mixed-case names are accepted. A name that passes cannot be read as a command option.
3. Integrity: apply requires the cache key from the pull, installs with the typed `cscli <type> install <name>` command (literal `cscli`, argument vector, no shell, no `--force` anywhere in non-test code), inspects the installed item and compares its local hash with the previewed content. Probes with an empty, truncated, padded, zero and older-version mismatching hash all ended in the "hub still updating" outcome (409) with the snapshot restored and no success status; a matching hash (either case) succeeds. A wrong, empty or stale cache key never reaches `cscli`.
4. Pull: the download URL must pass the hub host allowlist over HTTPS (the client blocks private targets and follows no redirects), the item type must be on the allowlist, and the payload must be a single YAML mapping within 1 MiB. Probes: gzip, HTML, multi-document, scalar, list, duplicate-key, alias-expansion and 100,000-deep nesting inputs were refused in under 20 ms with fixed messages.
5. Outcomes: deadline expiry gives 504 and restores the snapshot without evicting the cache entry; a mismatch evicts only when the entry still carries the key Apply loaded; `tainted_items` is limited to the preset's own item and its members and is filtered through `ValidHubItemName` (capped at 50). The hub-update step has its own 20 s sub-budget. Error text never carries command output (the executor error is an exit status), and the default branch passes through `RedactPaths`.
6. `cscli` JSON reading: the first value that starts a line and decodes completely is used, with a 32 MiB cap and 16 candidate starts. Probes with 100,000 candidate lines, 20 MB of openers and 30 MB of nesting returned the fixed error in under 60 ms. A decoy value before the real document would be read first but fails closed, because the inspect and hub-list readers then find no `installed` field or a wrong shape.
7. Deleted code: the archive extraction used for hub presets has no remaining caller; the extractor used by the configuration import is untouched. The curated preset path checks a supplied cache key against its own value.
8. Frontend: apply sends the canonical slug and cache key from the pull and is disabled for hub presets until a pull has succeeded; coded outcomes are shown inline from translations in all five languages; edited-item names and warnings are rendered as React text (no HTML injection) and the names are sanitised and capped at 10; the apply request timeout is 130 s, above the 120 s server default.
9. Defaults in the docs match the code: pull timeout 25 s, apply timeout 120 s (`HUB_APPLY_TIMEOUT_SECONDS`), hub-update sub-budget 20 s.
10. Public wording: commit subjects and bodies, docs, fixtures, code comments, test names and error texts were checked for terms that describe the earlier behaviour or an attack. The only `(security)` subject is `fix(security): harden CrowdSec preset handling`. One test name, one constant and two comments that hinted at the earlier behaviour were reworded (commit below). Fixtures are recorded public hub data. No session id or link appears in any message.

## Findings

No critical, high or medium findings.

### Low

- **L1 (fixed in this review).** The troubleshooting guide advised raising `HUB_APPLY_TIMEOUT_SECONDS` without saying that the web interface stops waiting after 130 s, so values above about 125 s only help for installs run directly with `cscli`. The text was corrected, and the intro to the message list no longer claims a rollback for the "applied with a note" case.
- **L2.** `validateMeta` compiles a regular expression on every cache load. The cost is negligible at the current cache sizes; precomputing it is a possible tidy-up.

### Informational

- **I1.** Members of a collection are installed by `cscli` from its own signed index and are not individually compared with the previewed content; only the previewed top-level item is. This is by design (the preview shows the top-level item).
- **I2.** `cscli` output is read with `CombinedOutput` without a size limit before the 32 MiB parser cap applies (existing executor behaviour).
- **I3.** Pre-existing CodeQL log-injection and cookie results (4) remain in untouched files and are suppressed in `codeql-suppressions.yml`.

## Commits added by this review

- `test: neutralise wording in CrowdSec cache tests and comments` (renames one test and one constant, reworded two comments; no behaviour change).
- `docs: clarify the CrowdSec hub preset timeout and messages` (troubleshooting text only).
- `docs: update QA report for the CrowdSec hub preset install` (this report).
