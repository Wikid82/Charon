# QA Report: Host Save Limits, Refused Changes, Import Sessions and Request Answers

- Branch: `fix/management-input-limits` (13 commits on `origin/development`, head `7e33d24ad`)
- Plan: `docs/plans/pr35_management_input_plan.md`, section 7 (Definition of Done and container gates G1-G5)
- Scope: a proxy host or redirection host change that the proxy refuses leaves the previous settings in place
  (create, update, delete and the three bulk edits); request size limits on host, settings check, Caddyfile, JSON and
  Nginx Proxy Manager import routes; a size limit on a changed advanced configuration; in-memory import previews that
  expire and are capped; Caddyfile upload files that are removed after commit, cancel or failure, and swept when
  abandoned; fixed answers for credential storage errors; the forward target text in import results; one plain
  validation answer (`invalid_request`) that names only public field names.
- Images under test: a throwaway image built from the working tree (`pr35qa:pr`) and, for G1 only, the previous
  image (`wikid82/charon:dev`, revision `1d0b57e0d`, the merge base of this branch). All containers and volumes were
  throwaway (`pr35qa-*`) and were removed afterwards. The live and shared containers were not touched.
- Toolchain: Go 1.27, CodeQL CLI 2.27.5, lefthook 2.2.2, Caddy 2.11.9 (in the image).
- Date: 2026-11-23.

## Verdict

**Approved.** Every Definition of Done gate passes and every container gate (G1-G5) matches the plan. QA added two
test-only files that raise patch coverage from 86.9% to 93.6%, and a `#nosec` comment in one test file that the
full lint flagged (see "Files changed by QA"). Two `Success Modal
Navigation` E2E cases failed on the first local run because two workers shared one import session; they passed on a
rerun with two workers and with one worker (F1). No application code was changed by QA.

## Findings

| Id | Severity | Finding | Remediation |
| --- | --- | --- | --- |
| F1 | Low (test isolation, not a product bug) | `caddy-import-gaps.spec.ts` 1.3 and 1.4 ran at the same time on two local workers. The Caddyfile import session is one global resource. Timeline from the container log: 1.3 uploaded at 19:19:01 (200, 571 ms); 1.4's `beforeEach` reset cancelled that session at 19:19:02 (`DELETE /import/cancel` 200, 2 ms); 1.3's commit then answered 404 in 2 ms, and its `waitForResponse` waits only for a 200, so it hung until the 90 s test timeout. 1.4 uploaded at 19:19:04 (200) and loaded the preview (200, 3 ms), but its page then sat on `Loading...` and never sent a commit; it also hit 90 s. Every backend answer in the window took under 600 ms. The rerun passed (5/5 with two workers in 22.1 s, and 5/5 with one worker in 21.0 s, the commit steps 3.6-5.1 s per test). CI runs one worker, so CI does not hit this | Make the `Success Modal Navigation` describe (or the whole spec) serial with `test.describe.configure({ mode: 'serial' })`, and fail fast when the commit answer is not 200 instead of waiting for a 200 only. Hand to playwright-dev; the spec was not edited here |
| F2 | Low (informational) | A Caddyfile upload with no importable site answers 400 with a `session` object (`state: "transient"`, `source_file` under `uploads/`) whose file has already been removed | Optional: drop `session` from that 400 answer, or document it as informational only. No user impact found: the UI shows the warning |
| F3 | Info (unchanged by design) | A Caddyfile that does not parse answers 400 with the adapter output, which includes the upload's relative file path. The plan keeps this text unchanged; the route is administrator-only | None for this PR |
| F4 | Info (out of scope, to be filed by the main session) | 31 handler sites outside the bind helpers still call `ShouldBind*` directly (10 in `security_handler.go`, 4 in `notification_provider_handler.go`, 3 each in `certificate_handler.go` and `backup_handler.go`, others single). The ones checked answer a fixed text and do not echo binder output; they do not use the `invalid_request` shape | Move them to `bindJSON`/`bindQuery` in a follow-up so every route answers the same way |

No critical, high or medium finding. No security-sensitive finding outside the plan's scope.

## Gates run (all foreground or block-waited)

| Gate | Result |
| --- | --- |
| E2E image | Throwaway image `pr35qa:pr` from the working tree, run as `pr35qa-e2e` with the local E2E settings on its own ports. Playwright used `PLAYWRIGHT_BASE_URL`, `EMERGENCY_SERVER_PORT`, `PLAYWRIGHT_CADDY_PROXY_PORT` and `PLAYWRIGHT_CONTAINER_NAME`; default local workers (2) |
| Playwright `--project=firefox`: `core/proxy-hosts`, `core/caddy-import/caddy-import-gaps`, `core/authentication` | 71 passed, 2 failed (F1) in 4.7 min. Rerun of `Success Modal Navigation`: 5 passed with two workers, 5 passed with one worker. The new cases all passed (refused advanced configuration keeps the previous host and a later create works, advanced configuration over 256 KiB, host body over 1 MiB, Caddyfile upload over 4 MiB, `forward_port: "abc"`, login with `{}`) |
| GORM security check (`--check`) | Pass: 0 critical, 0 high, 0 medium (2 informational index suggestions) |
| `scripts/go-test-coverage.sh` | Pass, all packages, race detector on, no failure. Statements 94.0%, lines 92.9% (gate 87%). `handlers` 93.3%, `services` 92.1%, `routes` 97.1%, `caddy` 98.0% |
| `scripts/frontend-test-coverage.sh` (input for the patch report; frontend unchanged) | Pass: lines 92.88%, statements 92.03%, branches 86.81% |
| `bash scripts/local-patch-report.sh` | Pass. Before QA's tests: overall 86.9% (799/919), below the strict 90% local threshold and under Codecov's 87% patch target. After: overall and backend 93.6% (860/919). The two later redirection tests cover four more lines (checked with a targeted coverage run) |
| CodeQL Go and JS (`lefthook run codeql`) | Pass. Go: 1 result (`go/cookie-secure-not-set`, `auth_handler.go:181`), already on file and suppressed. JS: 0 results. The first attempt failed with the old local CLI (2.27.2) against the pinned query packs; `scripts/install-codeql.sh` installed 2.27.5 and the rerun wrote fresh SARIF files |
| Trivy | Deferred to CI (no `feat` commit, per the plan) |
| `lefthook run pre-commit --all-files` | Pass: all 20 hooks, including semgrep and `golangci-lint-fast` |
| `go build ./...` | Pass |
| `npm run type-check`, `npm run build` | Pass |
| `make lint-backend` (run last, after QA's last edit) | First run: 1 issue, gosec G304 on the source scan in `internal/api/routes/request_errors_routes_test.go:92` (opens the handlers package's own files from a fixed glob). The fast pre-commit linter skips gosec in test files, so only the full lint saw it, and CI would have failed. QA added the repo's `#nosec G304` comment with a reason. Rerun: 0 issues. `agent/` is unchanged, so `make lint-agent` was not needed |

## Container gates (throwaway containers, real HTTP)

### G1: upgrade from a stuck instance

On the previous image, an administrator saved `[{"handler":"charon_unregistered_module"}]` as host B's advanced
configuration. The save answered 500 but the value stayed stored. A later save of host A (port 81) also answered 500
and was also stored; a create answered 500. The same volume was then started on the PR image:

- saving host A answered 500 "Failed to apply configuration (previous settings kept)" and A kept port 81;
- clearing B's advanced configuration answered 200, and the next save of A answered 200 (port 82);
- with the bad value written back into B, deleting B answered 200 and saving A answered 200 again.

### G2: refused advanced configuration on update

Host A had a working advanced configuration (a header handler). An update that changed the name, the port and the
advanced configuration to the unknown module answered 500 `proxy_apply_failed` with the proxy detail for the
administrator, in 0.1 s. `GET` then showed A's previous name, port and advanced configuration. Host B saved (200), A
saved an unrelated change (200, previous advanced configuration kept), and a new host was created (201). The running
proxy configuration held all three domains and A's working header, and not the unknown module.

### Refused delete of a disabled host

A disabled host with one location was deleted while another stored row made every apply fail (the unknown module
written straight into the throwaway database). The delete answered 500 "previous settings kept". The host came back
with the same id, UUID and name, still disabled, and its location came back with the same id. After the bad row was
fixed, the delete answered 200 and `GET` answered 404.

### Size limits

| Request | Size (bytes) | Answer |
| --- | ---: | --- |
| Host update body | 1,048,576 | 200 |
| Host update body | 1,048,577 | 413 `request_too_large` |
| Host create body | 1,048,706 | 413; no host row written |
| `advanced_config` after normalisation | 262,144 | 200 (applied) |
| `advanced_config` after normalisation | 262,145 | 400 `advanced_config_too_large` |
| Unrelated edit of a host storing a 256 KiB advanced configuration | 21 | 200 |
| Caddyfile upload body | 4,194,304 | 200 |
| Caddyfile upload body | 4,194,305 | 413 |
| JSON (Charon) import body | 8,388,608 | 200 |
| JSON (Charon) import body | 8,388,609 | 413 |
| Nginx Proxy Manager import body | 8,388,608 | 200 |
| Nginx Proxy Manager import body | 8,388,609 | 413 |
| Multi-file Caddyfile upload, 65 files | small | 400 `import_too_many_files`; nothing written |

The plan's G3 line mentions a 4 MiB JSON body; the decided limit for JSON and Nginx Proxy Manager imports is 8 MiB
(D-35-5), and that is what was tested.

### G3: JSON import previews

Five small JSON previews in a row: committing the first answered 404 "session not found or expired"; the second and
the fifth committed (200). Expiry after one hour is covered by the unit tests with a fake clock.

### G4: Caddyfile upload files

The uploads directory in the image is `/app/data/imports/uploads/`.

- A pending 4 MiB upload: cancel answered 200 and its file was removed.
- An upload that does not parse (400) and one with no sites (400) left nothing in `uploads/`.
- An upload then a commit (200, one host created): the file was removed; the session row is `committed`.
- A pending upload whose row and file were made 25 hours old, an orphan file 25 hours old and a fresh orphan file:
  after a container restart the pending session was `expired`, its file and the old orphan were removed, and the
  fresh orphan was kept. Older `rejected` and `committed` rows were left as they were.

### G5: validation answers and logs

| Request | Answer |
| --- | --- |
| `POST /setup` with a 5-character password | 400 `{"error":"The request is not valid. Check: password.","error_code":"invalid_request","fields":["password"]}` |
| `POST /setup` with `{}` | 400 "Check: email, name, password." |
| `POST /setup` with malformed JSON | 400 "The request is not valid.", no `fields` |
| `POST /setup` with `"name": 1` | 400 "Check: name." |
| `POST /auth/login` with `{}` | 400 "Check: email, password." |
| `POST /auth/login` with no body | 400 "The request is not valid.", no `fields` |
| `POST /auth/login` with an invalid email | 400 "Check: email." |
| `POST /proxy-hosts` with `"forward_port": "<text>"` | 400 "Check: forward_port." |
| `POST /import/upload` with `"content": 123`, `POST /import/npm/upload` with an array | 400 "Check: content." |

No answer contained `Key:`, `Error:Field`, a struct or Go type name, or a byte offset. For comparison, the previous
image answered login `{}` with the binder text naming `LoginRequest.Email` and the `required` tag.

A marker string was sent in failed Caddyfile, JSON and Nginx Proxy Manager imports and in a failed host create. It
did not appear in the container log, in the log files, or in `GET /logs/charon.log` and `/logs/cpmp.log` read as a
standard user. The domains of earlier successful and failed imports did not appear there either, and no
`raw_body` or Caddyfile preview field was logged.

## Review notes

- The bind-helper scan matches the developer notes: 79 `bindJSON`/`bindQuery`/`bindSmallJSON` call sites outside
  tests, and 31 direct `ShouldBind*` calls elsewhere (F4).
- A standard user's refused save answers the fixed text without the proxy detail (unit tests); the administrator
  gets the detail (G2).
- Deleting a host whose apply is refused gives it a new uptime monitor; its earlier check history is not kept, as the
  plan documents (D-35-12).

## Files changed by QA

| File | What |
| --- | --- |
| `backend/internal/services/host_restore_failure_paths_test.go` (new) | Storage-failure and edge branches of the proxy and redirection host restores: failed reads, refused inserts, updates and deletes (SQLite triggers), a domain claimed by the other host type, a deleted row in a column restore, and a column restore with no newer edit |
| `backend/internal/api/handlers/import_upload_retention_failures_test.go` (new) | Upload retention: a stale session outside `uploads/` is left alone, a failed expiry is reported (sweep and supersede), failed session lookups are reported |
| `backend/internal/api/routes/request_errors_routes_test.go` | `#nosec G304` comment, with a reason, on the source-scan `os.Open` (full lint finding) |
| `docs/reports/qa_report.md` | This report |
