# QA Report: CrowdSec Start, Reload and Import Readiness

- Branch: `fix/crowdsec-wait-for-start`, audited at HEAD `2bba3480` (9 CrowdSec commits, patch-identical to the earlier `c2d660d2` tip; the branch was rebased onto the current `origin/development` during the run, which added 6 unrelated base commits: CI pins, i18n languages, agent file names).
- Date: 2026-10-31
- Scope: starting the CrowdSec engine, waiting for it to be ready before reloads, keeping the whitelist file present, and checking an imported configuration before it is used.
- Working tree: nothing committed, staged, stashed or pushed by QA. This report is the only file QA wrote.
- **Verdict: READY, with three conditions** (see the end of this report).

## Gate Summary

| # | Gate | Result | Numbers |
| --- | --- | --- | --- |
| 1 | `cd backend && go build ./...` | PASS | clean |
| 2 | Backend coverage (`scripts/go-test-coverage.sh`, gate 87%) | PASS (see Notes) | line coverage 91.85% (42258 of 46007); statements 93.0% |
| 3 | Frontend coverage (`scripts/frontend-test-coverage.sh`, gate 85%) | PASS (see Notes) | lines 91.10% (8421 of 9243); statements 90.09%; branches 84.19%; functions 87.16% |
| 4 | Local patch coverage (`scripts/local-patch-report.sh`) | PASS | overall 94.0% (313 of 333 changed lines); backend 94.0% (311 of 331); frontend 100% (2 of 2) |
| 5 | `npm run type-check` | PASS | clean |
| 6 | `npm run build` | PASS | clean |
| 7 | `npm run lint` | PASS | 0 errors, 1359 warnings (existing backlog) |
| 8 | `make lint-fast` (staticcheck, govet, errcheck, ineffassign, unused) | PASS | 0 issues (backend and agent) |
| 9 | `lefthook run pre-commit` | PASS (no-op) | no files were staged, so every hook skipped; the fast linters were run directly in row 8 |
| 10 | GORM security scan | NOT REQUIRED | no file under `backend/internal/models/` or any migration changed |
| 11 | Playwright `tests/security/crowdsec-import.spec.ts` and `tests/security/crowdsec-first-enable.spec.ts` (`--project=security-tests`) | PASS | 62 of 62 on the final build |
| 12 | Playwright `tests/crowdsec-whitelist.spec.ts` (`--project=firefox`) | PASS | 12 of 12 on the final build |
| 13 | CodeQL and Trivy | DEFERRED TO CI | the commits are `fix:`, `test:` and `docs:`; no new feature commit; CI runs both on the PR |

## Container Gate

Throwaway containers built from the locally built `charon:local` image, with their own volumes, published on high loopback ports only, and deleted afterwards. The live development container was not touched.

| Gate | Scenario | Repetitions | Result |
| --- | --- | --- | --- |
| (a) | Whitelist add at 0.1, 0.5, 1.0, 1.5, 2.0, 2.5 and 3.0 s after Start | 3 each (21 runs) | 21 of 21: engine alive, the add was accepted, the reload was delivered after readiness (`Reload is finished` after the add), the entry was written, no fatal line |
| (b) | Port 8085 occupied inside the container, then Start | 3 | 3 of 3: HTTP 500 `crowdsec_engine_exited` in about 1.2 s; the engine logged a bind failure on the occupied port |
| (b2) | Two concurrent Starts | 3 | 3 of 3: one engine process; one request answered 200, the other 409 `crowdsec_busy` |
| (b2) | Start then Stop 50 ms later | 3 | 3 of 3: Stop answered 200; no engine process left |
| (c) | Placeholder whitelist file (events parsed, real entry matches, sibling file ignored) | 1 pass | Placeholder present and listed; `cscli explain` on a public test address showed `parser success` with the managed whitelist unchanged; a real entry matched after add, delete (empty) and re-add; the temporary sibling file was not loaded |
| (c) | Managed-file guard | 1 | the canonical managed path answered 400 `file_managed_by_charon` on write and was readable |
| (c) | 45 s stress: 8 adds and 8 deletes (16 mutations) | 1 | 41 s elapsed; 9 engine reloads delivered (the reloader coalesced the rest); engine alive at the end; placeholder present; no fatal line |
| (d) | Import of an archive exported by a real Charon container | 1 | HTTP 200 `imported`; engine still running |
| (d) | Stock CrowdSec layout (absolute `/etc/crowdsec` paths, missing nginx log file, journald source) | 1 | direct `crowdsec -t` exit 0; import HTTP 200 (engine logged the missing source as an error but the configuration check passed) |
| (d) | Archive without `config_paths` | 1 | HTTP 422 `config validation failed: config.yaml has no config_paths section`; live configuration byte-identical (checksum) |
| (d) | Archive with a broken `config_dir` | 1 | HTTP 422 `import_config_invalid`; live configuration byte-identical (checksum) |
| (d) | `crowdsec -t` while an engine runs | 1 | exit 0 on the live configuration |
| (e) | Decision ban against a created proxy host | 1 | baseline 200; after `ban` the proxy answered 403 in about 54 s (bouncer poll interval); after `unban` the proxy answered 200 within the next poll cycle (about 60 s) |
| (e) | T2 refuse-at-once (async non-curated preset apply while a hub restore holds the data lock) | 0 | NOT EXECUTED; see Findings F6 |
| (f) | Two SIGHUPs 0.2 s apart | 3 | 3 of 3: both signals received and each reload finished (roughly 3 s each, run one after the other); engine alive; no fatal line |

### Output samples for the environmental-error pattern

Captured from the engine log when the port was occupied (matched by the `environmentalEngineOutput` pattern):

```
level=fatal msg="local API server stopped with error: listening on 127.0.0.1:8085: listen tcp 127.0.0.1:8085: bind: address already in use"
```

`crowdsec -t` never reported a port or database-lock error in these runs (it exited 0 while the engine was running). A missing notification plugin did not reproduce under `-t` either (the engine loads plugins at start, not during the check). The database-lock and plugin texts are therefore not yet pinned by a captured sample.

## Findings

| ID | Severity | Summary | Status |
| --- | --- | --- | --- |
| F1 | Low | The scope of the managed-file guard in the CrowdSec file editor is narrower than intended. The engine does not read the affected location, so runtime behaviour is unaffected. | Hardening item; tracked privately. Details withheld from this public report. |
| F2 | Info | The stock CrowdSec layout passes `crowdsec -t`. The hard refusal on the engine's own non-zero exit therefore does not block stock archives in this test. | Result, not a defect. Informs the decision to keep the hard refusal. |
| F3 | Low | Under rapid whitelist changes the reloader coalesces reloads: 16 mutations in 41 s produced 9 engine reloads. The final engine state matched the last mutation. | Expected behaviour of the coalescing design; the test expectation of one reload per mutation does not hold. Document in the troubleshooting page if it matters to operators. |
| F4 | Low | A truly concurrent second Start is answered 409 `crowdsec_busy`, not with the existing process as a 200. Only one engine is ever started. A sequential second Start still answers 200 `already_running`. | Safe. Confirm this is the intended contract; the UI should show the busy text. |
| F5 | Low | Gate (b) checks that the security module setting is unchanged after the failed Start, but the setting was already disabled before the test, so the revert was not exercised. | Test gap. Re-run with the module enabled before the occupied-port Start. |
| F6 | Medium (test gap) | T2 (refuse at once instead of waiting behind a hub update) was not executed. Reproducing it needs an in-flight hub restore that holds the data lock, and the throwaway container has no outbound hub access. | Carry to a follow-up test in an environment with hub access. Not a regression finding for this branch. |
| F7 | Info | The database-lock and missing-plugin output samples were not reproduced with `crowdsec -t`. The pattern covers them, but only the bind-failure sample is pinned. | Capture samples when a reproducer is available. |
| F8 | Info | One test file of the 312 in the frontend suite did not appear in the chunked run's file count (311 reported). The full suite was not run as one process in this session. | Re-check with the CI frontend job. |

## Notes on the Gate Runs

- The backend and frontend gate scripts run longer than the ten-minute limit of the QA session. The same measurements were produced by running the same packages and the same test files in chunks, merging the profiles with the same exclusions, and applying the script's line-coverage calculation. The numbers above are from those merged profiles. CI runs the scripts unchanged.
- The frontend profile was merged with the same istanbul libraries used by the test runner. The totals are the coverage summary totals.
- The Gotify test fixtures in the frontend notification tests use placeholder strings. No real token appeared in test artifacts, logs or screenshots.
- No Gotify token or other secret was written to this report.

## Conditions for Merge

1. Re-run gate (e) T2 in an environment with hub access (F6), or record the accepted gap in the PR.
2. Review and fix the managed-file scope item F1 through the private process; it does not block the functional change.
3. CodeQL and Trivy results from CI must be clean before merge (deferred locally, per the project rule for `fix:`, `test:` and `docs:` commits).
