# QA and Security Report: fix/crowdsec-hub-url-and-editor-hardening

Date: 2026-10-10
Scope: full diff `origin/development...HEAD` (HEAD 5bb1889d, 24 commits, 57 files), spec `docs/plans/current_spec.md` and `docs/plans/crowdsec_preset_apply_hardening_1525_spec.md`. Parts: (A) CrowdSec editor access hardening, backup names instead of paths, archive-name fix, removal of the unenforced `WithAllowedDomains` option, strict hub URL allowlist; (B) #1526 Playwright rewrite of six CrowdSec specs; (C) fixes found by it (i18n keys, `sanitizeSecret`, enrollment form, export gzip).

## Verdict: PASS (no blocking findings)

No critical or high findings. One medium-low gap against the spec's own acceptance criterion ("no absolute path in any CrowdSec API response") is flagged as F1; it is admin-authenticated information exposure, so it is a maintainer decision whether to close it in this PR or file it. All Definition of Done gates pass.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | pass |
| Backend vet | `go vet ./...` | pass, no output |
| golangci-lint, backend | `golangci-lint run --config .golangci.yml ./...` | 0 issues |
| golangci-lint, agent | `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...` | 0 issues |
| Frontend type-check | `npm run type-check` | pass |
| Frontend build | `npm run build` | pass |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query files changed (0 matches in the diff) |
| Lefthook | `lefthook run pre-commit --all-files` | all 17 hooks pass, including semgrep (0 findings, 367 rules; it logged timeout warnings on three large test files, which are scan-coverage notes, not findings), shellcheck, actionlint, golangci-lint-fast, frontend-lint. Working tree stayed clean. |
| govulncheck | skill `security-scan-go-vuln` | 0 affected; 1 module-level advisory in a required module that Charon code does not call |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum (backend, agent), package-lock.json and Dockerfile | no HIGH/CRITICAL. The snap trivy cannot read `/projects`, so the manifests were copied under `$HOME` and removed afterwards. No container-image scan locally. |
| CodeQL | not run locally | `fix:`/`test:` scope, no new feature surface, deferred to CI per CLAUDE.md |
| E2E | `npx playwright test <nine crowdsec specs> --project=security-tests` | 179 passed, 0 failed (8.4 min). The E2E container (built at 841955af) is valid for HEAD: `git diff 841955af..HEAD` touches only `_test.go`, vitest and Playwright files. |
| Coverage (given, not rerun) | prior runs | backend 91.0% stmt / 90.1% line, frontend 91.46% lines, patch 98.9% |

## Check-by-check results

1. **No absolute path in responses.** All nine leak sites from spec section 2.3 are closed: `backup` fields now go through `crowdsec.BackupID` (file write, import, preset apply success and failure, curated apply, acquisition update); `rollbackFailure`, extractor, `CreateFileNoSymlink` and import/export errors no longer embed paths; preset pull/apply, import validation errors pass through `RedactPaths`. Backed by `crowdsec_response_paths_test.go` and the live E2E checks. Residual gaps are listed as F1 and F2.
2. **`sanitizeSecret`.** Still masks anything alphanumeric 10-64 chars that mixes letters and digits, and letters-only runs of 32+. A letters-only token of 10-31 chars, or one split by `-`/`_`, is not masked client-side (previously every 10-64 char run was masked). Real enrollment keys are random alphanumeric, so the chance of a digit-free 25-char key is negligible, and the backend already redacts the exact token (`redactSecret`, `console_enroll.go:421`) before any message reaches the client. Acceptable as defense in depth; see F3.
3. **Hub URL check.** `validateHubURL` now accepts only https plus the three allowlisted hosts. Previously `localhost`, `127.0.0.1`, `::1`, `*.example.com`, `*.example`, `*.local` and `test.hub` passed this layer, but the dial layer (`NewSafeHTTPClient`) already blocked loopback and private targets in production, and unknown public hosts were already rejected by the allowlist. So no production behavior changes for `HUB_BASE_URL` or `HUB_MIRROR_BASE_URL`; the docs note (`docs/troubleshooting/crowdsec.md`) is accurate. Redirects: the hub client uses default `MaxRedirects` 0 (`CheckRedirect` returns `ErrUseLastResponse`), so a redirect cannot reach an unvalidated host. `WithAllowedDomains` was never read anywhere (`AllowedDomains` had no consumers), so its removal is behavior-neutral and the SSRF protection actually relied on the dial layer plus `validateHubURL`. The test seam `HubService.validateURL` is an unexported field set in exactly one test (`hub_sync_test.go:117`, a local httptest server); production never sets it and nothing outside the package can. The other tests use allowlisted hostnames with mock transports, or unreachable `127.0.0.1:1` URLs that only assert failure (`backup_test.go:598-663`), so none depends on a real non-production host.
4. **gzip exclusion.** gin-contrib/gzip v1.2.8 matches `strings.HasPrefix(requestURI, path)`. The only registered route with the prefix `/api/v1/admin/crowdsec/export` is `GET /admin/crowdsec/export`; `/admin/crowdsec/decisions/export` does not start with it. No other route is excluded. Caveat in F5.
5. **E2E tolerance.** The Bz command from the spec over the six files returns nothing (exit 1). An extra scan of the six specs and both helpers for `console.log`, `annotations.push`, `test.fixme` and `.skip(` also returns nothing. `tests/` is in `.dockerignore`, not copied by the Dockerfile, and `crowdsec-stubs.ts` / `archive-helpers.ts` are not imported from `frontend/` or `backend/`, so no test helper ships in a production bundle.
6. **Commit subjects.** Reviewed with `git log origin/development..HEAD --format=%s`. The only `(security)` subject, `fix(security): harden CrowdSec editor access`, is vague. See F4 for two non-scoped subjects that are slightly specific.
7. **Security-sensitive items.** See F1 and F6; no exploit detail is given.

## Findings

No critical or high findings.

### Security-sensitive (do not file publicly; maintainer to decide)

**F1: low-medium (information exposure, admin-authenticated).** Absolute server paths can still reach CrowdSec API responses at sites that the spec did not enumerate, contradicting its acceptance criterion "no absolute path in any CrowdSec API response, including error text":
- `backend/internal/api/handlers/crowdsec_handler.go:1977` and `:1986`: `GET /admin/crowdsec/acquisition` returns `path` (the acquisition file location) on both 404 and 200. The frontend does not read it.
- `crowdsec_handler.go` diagnostics config (`~2136-2230`): `validation["config_path"]` and `validation["acquis_path"]` return absolute paths. The frontend does not read them.
- `crowdsec_handler.go:522` (Start), `:596` (Stop), `:624` (Status), `:978` (ClearEnrollment), `:2250` (console status), and `crowdsec_preset_handler.go:395` (preview) return raw `err.Error()`. The executor errors wrap filesystem errors (`crowdsec_exec.go:71` "failed to write pid file: %w", `:92` "pid file read: %w"), which embed the pid-file path.
Suggested remediation: drop the `path` fields (or return a fixed label), and either pass those errors through `crowdsec.RedactPaths` or return fixed messages and log the detail. Add the sites to `crowdsec_response_paths_test.go`.

**F6: low (documented limitation).** `ExportConfig` still archives the credentials files and `bouncer_key` that the editor now hides (the spec records this as a known limitation and defers the decision). `docs/features/crowdsec.md` and `docs/features.md` say the editor no longer lists the credential files but do not mention that the config export still includes them, which could give false assurance. Suggested: one sentence in the docs and a tracked issue for the export decision.

### Non-security (for the maintainer to file)

- **F2: low.** `RedactPaths` (`crowdsec/backup.go:351-356`) only redacts absolute paths preceded by whitespace, a quote, `(` or `=` and ends them at whitespace, `:`, `;`, `)` or a quote. Paths preceded by `[`, `{`, `,`, `<`, `@` or `:`, and the tail of a path containing a space or colon, would pass through. Today every redacted source (cscli output, validators, extractor) uses the "open /path: reason" form, so nothing leaks in tested cases; the function is a safety net, not a guarantee. It also rewrites non-path text such as " /api/v1/x" to `<path>` (cosmetic).
- **F3: low.** `frontend/src/utils/sanitizeSecret.ts:10-18` no longer masks letters-only runs of 10-31 characters (see check 2). Backend redaction is the primary control. Optional: keep masking any 20+ char run regardless of content.
- **F4: low (changelog wording).** `fix: stop returning server paths in CrowdSec responses` (66a4aced) and `refactor: remove the unenforced allowed-domains client option` (1725feb8) are not `(security)`-scoped but name the issue category fairly directly and will appear verbatim in the public changelog. Consider rewording before merge, for example `fix: show CrowdSec backups by name` and `refactor: remove unused HTTP client option`. No hash is pushed yet, so rewording is cheap.
- **F5: info.** `routes.go:1260` excludes by URI prefix, so any future route that begins with `/api/v1/admin/crowdsec/export` (for example `export-foo`, or a query string) would also skip compression. Harmless today; an exact-path or regex exclusion would be stricter.
- **F7: info.** The prior report's S1/S2 (extraction opens destination files without a symlink check in `extractTarGz` and `ImportConfig.extractArchive`) were not re-audited in this pass beyond the changed lines; `CreateFileNoSymlink` is unchanged in behavior.

## Out of scope and not verified

- Full-suite and cross-browser E2E are deferred to CI.
- CodeQL and a container-image Trivy scan are deferred to CI.
- Coverage and local patch scripts were not rerun, as instructed.

## Resolution (F1, F2 and the remaining findings)

- **F1: fixed.** `GET /admin/crowdsec/acquisition` no longer returns `path`; `GET /admin/crowdsec/diagnostics/config` no longer returns `config_path` or `acquis_path` (nothing in `frontend/src` or `tests/` read them). Start, Stop, Status, ClearEnrollment, console heartbeat and the preset cache preview/metadata handlers now return fixed messages and log the detail server-side; console enrollment errors, which are user-facing validation text, pass through `RedactPaths`. Covered by `crowdsec_response_paths_test.go`.
- **F2: fixed.** `RedactPaths` now matches absolute paths at the start of the string or after whitespace, a quote, `(`, `[`, `{`, `<`, `=` or `,`, ends them at whitespace, a quote or `: ; , ) ] } > <`, and leaves text under `/api/` untouched. Remaining limits (paths containing spaces or `:`, Windows or relative paths) are documented on the function; handlers prefer fixed messages. Covered by `TestRedactPaths`.
- **F3, F5: reviewed and left** as low or informational, as recommended (backend redaction is the primary control; the gzip exclusion is exact for every registered route).
- **F4: handled by commit rewording** before merge.
