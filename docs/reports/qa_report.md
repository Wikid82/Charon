# QA Report: PR 17 (CrowdSec leftovers), branch `fix/crowdsec-leftovers`

Date: 2026-11-04. Scope: 10 commits ahead of `development`, tree clean at start. Code-scanning (CodeQL/Trivy) is deferred to CI for this fix-scoped change. GORM scan skipped (models unchanged). The docs-site build was not run locally (`zensical` not installed); it runs in CI.

## Result: PASS after one fix (see Finding 1)

| Gate | Result |
| --- | --- |
| `scripts/go-test-coverage.sh` | Pass after the fix in Finding 1 (first run: 4 failures in `internal/api/handlers`). Re-run: Statement coverage 93.2%, line coverage 92.1% (min 87%) |
| `scripts/local-patch-report.sh` | Pass: backend patch coverage 57/57 lines (100%); artifacts in `test-results/` |
| Pinned golangci-lint v2.15.0 | Pass: 0 issues |
| `scripts/frontend-test-coverage.sh` | Pass: 321 files, 4131 tests; statements 91.09%, branches 85.6%, lines 92.09% |
| `npm run type-check` | Pass |
| `scripts/cscli_hub_mirror_test.sh` | Pass: 144 passed, 0 failed |
| `scripts/verify-toolchain-pin.sh` | Pass (caddy-crowdsec pin verified) |

## Finding 1 (high, FIXED in the same PR): tests still point the stored engine address at a test server

Failing, deterministically (reproduced in a scoped re-run, port 8085 free):
- `TestLAPIKey_SavedKeyFileFirst`, `TestLAPIKey_EnvironmentInRuntimeOrder` (`crowdsec_ban_alerts_contract_test.go`, helper `pr4LAPIHandler` at about line 799)
- `TestCSCLIContract_GetLAPIDecisions_RecordedResponse`, `TestCSCLIContract_DashboardSummary_ActiveDecisionsFromRecordedResponse` (`crowdsec_cscli_contract_test.go`)

Cause: these tests store a `CrowdSecAPIURL` that names an httptest server and expect the decisions and dashboard readers to call it. After commit 1 the readers always use `crowdsec.LocalLAPIURL`, so the calls miss the test server (500, `-1` count, no header recorded). The behavior is as designed; the tests were not adapted. Fix: have the tests serve the fake engine on the local address, or inject the engine address through a test seam in the handler, then re-run `go test ./internal/api/handlers/`.

## Container gates (throwaway image and containers; live `charon` and `charon:local` untouched, all throwaway resources removed)

- Step 0 (pre-change behavior of applies during a restore): not run. It needs an image of the base commit, which was not built.
- Caddy modules: crowdsec bouncer (`crowdsec`, `http.handlers.crowdsec`, `admin.api.crowdsec`, `layer4.matchers.crowdsec`), `http.handlers.waf`, `http.handlers.appsec`, `security`, `rate_limit`, `geoip2`, and 28 `dns.providers.*` are present. The generated proxy config carries `api_url` `http://127.0.0.1:8085`; the bouncer log shows key auth against that address and `started`.
- G1 pass: stored address `http://203.0.113.9:9999` accepted; decisions answer 200 `source: lapi`, health answers `healthy` at `http://127.0.0.1:8085`, status reports the local address, and the proxy config has only the local address.
- G2 pass (hub blackholed): with a restore in flight, curated sync, second curated sync, non-curated sync, non-curated async and curated async applies each answered 409 `crowdsec_busy` in under 10 ms; a second restore answered 409 with `error_code: crowdsec_busy`; the first restore ended 502 after 60 s.
- G3 not run: blocking only the item path on the CDN needs a TLS-terminating proxy with a trusted CA, which this host setup cannot provide quickly. Unit tests are the authoritative check.
- G4 pass: with the module enabled (`local`) and the engine port occupied, Start answered 500 `crowdsec_engine_exited` in 1.3 s; module state before and after were identical (enabled, `local`); Start succeeded once the port was freed.
- G5 pass: bouncer reports `registered: true`, `registration: registered`.
- Playwright `tests/security/crowdsec-ui-states.spec.ts -g "Registered badge"`, project `security-tests`: passed.

## Re-check after the fix

The four tests now serve the fake engine at the local address (the same seam the new address tests use), and a grep found no other handler test that stores a test-server address. Re-run results: `go test ./internal/api/handlers/ -count=1` pass; `scripts/go-test-coverage.sh` pass (statements 93.2%, lines 92.1%, minimum 87%); pinned golangci-lint v2.15.0: 0 issues; `go test ./internal/crowdsec/ -run 'Pull|Mirror'` pass.

Review items folded in: the mirror URL builder returns no mirror when the mirror base is empty; two docs sentences corrected; comment placement in the config struct. Note for the PR: a download body that breaks mid-transfer is now classed as a transport failure, so it also triggers the one mirror retry.
