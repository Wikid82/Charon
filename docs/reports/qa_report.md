# QA Report: CrowdSec Start/Stop Apply (PR 9)

- Branch: `fix/crowdsec-start-stop-apply`
- Scope: Start/Stop applies the proxy config, default security settings row repair, no-key omission, K1/K2 consistency fixes, toast translations
- Commit range: `034d318d..HEAD`
- Commit types in range: 6 `fix`, 2 `fix(security)`, 2 `refactor`, 1 `test`, 2 `docs`, 2 `chore(docker)`. No `feat` commits.

## Note on HEAD

The run started at HEAD `587688da`. At 12:47:22 UTC the branch was rebased onto the same base (`034d318d`), giving `3d143f4e`. The six subjects are unchanged. The only content difference is a one-line change to the `GEOLITE2_COUNTRY_SHA256` build ARG in `Dockerfile`. The gates below were run on both HEADs; the source under test is identical.

## Verdict

**PASS, with no blocking findings.** One low-severity finding was filed as a GitHub issue. Several API-level observations are recorded below and are not filed publicly.

## Gate results

### Pre-passed gates (cited, not rerun)

| Gate | Result |
|---|---|
| `go build ./...` | PASS |
| `-race` tests: models, caddy, cerberus, testutil, routes, handlers, services | PASS |
| `scripts/go-test-coverage.sh` | PASS, exit 0. Statements 92.6%, lines 91.5% (gate 87%). `/var/tmp/gocov.log` has 0 FAIL lines. |

### Frontend

| Gate | Result |
|---|---|
| `scripts/frontend-test-coverage.sh` (vitest, v8) | PASS. Lines 91.95% (gate 87%), statements 90.9%, functions 88.76%. Branches 84.63% (informational). 311 files, 3901 passed, 4 skipped, 2 todo, 0 failed. |
| `npm run type-check` | PASS, exit 0 |
| `npm run lint` (eslint) | PASS, 0 errors. 1358 warnings (non-blocking, not introduced by this range). |
| `npm run build` | PASS |
| i18n tests (`src/__tests__/i18n.test.ts`, `CrowdSecConfig.i18nKeys.test.ts`, `CrowdSecKeyWarning.test.tsx`) | PASS, 44/44 |

The coverage script ran past the 600s tool cap. The vitest run was re-attached and completed with exit 0. The script's own `npm ci` was run separately.

### Patch coverage

`scripts/local-patch-report.sh`: PASS, strict mode. Both artifacts were written: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.

| Scope | Changed lines | Covered | Patch % |
|---|---|---|---|
| Overall | 364 | 364 | 100.0 |
| Backend | 362 | 362 | 100.0 |
| Frontend | 2 | 2 | 100.0 |
| Agent | 0 | 0 | 100.0 |

### Lint

| Gate | Result |
|---|---|
| `make lint-fast` (staticcheck, govet, errcheck, ineffassign, unused; backend and agent) | PASS, 0 issues |
| `make lint-backend` (golangci-lint v2.15.0, full config, docker) | PASS, 0 issues |
| markdownlint on changed docs (`docs/features.md`, `docs/features/crowdsec.md`, `docs/features/live-reload.md`, `docs/troubleshooting/crowdsec.md`) | PASS |

### Security

| Gate | Result |
|---|---|
| `scripts/scan-gorm-security.sh --check` (models changed: 9 files) | PASS. 0 CRITICAL, 0 HIGH. 2 INFO suggestions (index hints). |
| govulncheck (`backend/...`) | PASS. 0 vulnerabilities reachable from code. 1 vulnerability in required modules that the code does not call. |
| CodeQL and Trivy (local) | Not run locally. The range has no `feat` commits, so the policy defers both to CI. |
| Gotify token review | PASS. No Gotify token literals in the diff. No keys were printed during QA. |

### E2E (Playwright, one project per run; full suite not run)

| Run | Result |
|---|---|
| Rebuild `charon-e2e` (`docker-rebuild-e2e`) | PASS |
| `--project=security-tests`: `tests/security/crowdsec-security-toggle-busy.spec.ts`, `tests/security/crowdsec-first-enable.spec.ts`, `tests/security-enforcement/crowdsec-enforcement.spec.ts`, `tests/security-enforcement/zzzz-break-glass-recovery.spec.ts` | PASS, 16 passed |
| `--project=firefox`: `tests/crowdsec-whitelist.spec.ts` | PASS, 11 passed |

### Real-cscli throwaway container gate

Container `charon-qa-gate9`, image `charon:local` (rebuilt from this tree), volumes `qa9_data`, `qa9_caddy_data`, `qa9_caddy_config`. The live `charon` container was not touched.

Environment deviation: the throwaway container has no outbound network. The CrowdSec hub index was missing, so LAPI could not start. The hub directory was streamed into the throwaway container from the `charon-e2e` test container with `tar`. This is test-harness seeding only; no application code was changed.

| Scenario | Result | Evidence (no key values printed) |
|---|---|---|
| Start with no bouncer key | PASS | Start with no key returned `started`. Served Caddy config had no CrowdSec app. Log: "CrowdSec is on but no bouncer key exists yet; CrowdSec is left out of the proxy configuration". |
| F2: key created by Start reaches Caddy without restart | PASS | Key file and bouncer removed, then Start. LAPI ready, `key-status` valid with `key_source: file`. Served config gained the CrowdSec app with a 43-character key. Container uptime unchanged (no restart). |
| CrowdSec on/off without restart (served config changes immediately) | PASS | Disable while running: CrowdSec app removed from served config at once. Enable: app back with key. Container not restarted. |
| 7a: emergency reset, then restart, engine and served config agree | PASS, with a finding | Reset returned 200 and the served config dropped the CrowdSec app. The engine stayed running until restart. After restart: LAPI refused, served config had no CrowdSec app. Agreement reached. Finding filed as #1736. |
| 7b: disable while running / enable while stopped (raw module API) | Observation, not filed | See "API-level observations" below. |
| Boot with a stale wrongly-named default row | PASS | Injected row named `Default Security Config` with mode `local`, default row mode `local`, setting `false`. After boot: legacy row deleted, default row mode synced to `disabled`, log "security config repair: removed legacy default security config rows". Served config and engine consistent (no CrowdSec). |
| Failed Start reverts to the exact prior settings snapshot | PASS | Engine binary forced to a missing path (`CHARON_CROWDSEC_BIN`). Start returned 500 `failed to start CrowdSec`. Settings rows identical before and after. The `security_configs` row was identical except `updated_at`, which the revert write bumps. |

### Migration and scope checks

| Check | Result |
|---|---|
| `git diff 034d318d..HEAD -- backend/internal/changelog` | PASS, empty |
| `go.work.sum` unmodified | PASS |

## Findings

### Blocking

None.

### Filed (non-security, low)

- **#1736**: Emergency security reset leaves the CrowdSec engine running until restart. Evidence and suggested fix are in the issue.

### API-level observations (not filed, handled privately)

These come from the raw module endpoints `POST /api/v1/security/crowdsec/{enable,disable}` and `PATCH /api/v1/security/crowdsec`. The Security page does not call them; its toggle uses `admin/crowdsec/start` and `admin/crowdsec/stop`, which behaved correctly in every scenario above.

- Enabling via the raw endpoint while the engine is stopped stores the module as enabled and adds the CrowdSec app to the served config, pointed at `127.0.0.1:8085`, where nothing listens. The docs say the switch, the saved setting and the engine "stay in step", which this path does not do.
- Disabling via the raw endpoint while the engine is running removes CrowdSec from the served config, but leaves the engine running. The docs say disabling stops CrowdSec, which only the Stop path does.
- Not verified: how the caddy CrowdSec bouncer behaves (fail-open or fail-closed) when LAPI is unreachable. This determines whether the first bullet is a security concern.

### Other observations (not blocking, not filed)

- The JWT secret falls back to a random per-boot value when `CHARON_JWT_SECRET` is unset, so every restart logs users out. This is existing fallback behaviour.
- The failed-Start revert bumps `security_configs.updated_at`. This is harmless.
- eslint reports 1358 warnings. None are errors and this range did not introduce them.

## Artifacts

- `/projects/Charon/docs/reports/qa_report.md` (this file, uncommitted)
- `/projects/Charon/test-results/local-patch-report.md`
- `/projects/Charon/test-results/local-patch-report.json`
