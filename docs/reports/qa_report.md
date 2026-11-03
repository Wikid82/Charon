# QA Report: Proxy Admin Interface on the Loopback Address

- Branch: `fix/proxy-admin-loopback`, HEAD `c0153e2d6` (6 commits ahead of `origin/development` `f87f7a533`).
- Date: 2026-11-03
- Change: the proxy admin interface now listens on the loopback address only, the Caddy Admin API setting is removed from System Settings, and an unsupported external proxy address in `CHARON_CADDY_ADMIN_API` stops startup with a clear message. Test tooling reads the proxy configuration from inside the container.
- Verdict: **BLOCKED on one CI-blocking lint finding** (B1: `make lint-backend` reports 1 gosec issue). Every other Definition of Done gate and every container gate passes. Fix B1, then re-run `make lint-backend`.

## Gate Summary

| # | Gate | Result | Numbers and notes |
| --- | --- | --- | --- |
| 1 | `cd backend && go build ./...` | PASS | exit 0 |
| 2 | `go test -race -count=1` `./internal/config/...`, `./internal/caddy/...`, `./cmd/api/...` | PASS | config ok (1.1 s), caddy ok (13.1 s), cmd/api ok (88.1 s) |
| 3 | `scripts/go-test-coverage.sh` (gate 87%) | PASS | statements 93.0%, line 91.9%; 45 packages `ok`, no `FAIL` lines. `internal/config` 95.3%, `internal/caddy` 97.9%, `cmd/api` 82.5% |
| 4 | `bash scripts/local-patch-report.sh` | PASS | strict mode. Overall patch coverage 18 of 18 changed backend lines (100%). Frontend and agent have 0 changed lines. Both artifacts written: `test-results/local-patch-report.md` and `test-results/local-patch-report.json`. The script needs `frontend/coverage/lcov.info` and `agent/coverage.txt`, so `scripts/agent-test-coverage.sh` (statements 75.8%, line 75.4%, gate 65%) and the frontend script were run first |
| 5 | `cd frontend && npm run type-check` | PASS | exit 0 |
| 6 | `scripts/frontend-test-coverage.sh` (gate 85%) | PASS | 318 files, 4007 passed, 4 skipped, 2 todo. Statements 90.84%, **branches 85.04%** (close to the floor), functions 88.36%, lines 91.84%. The touched `SystemSettings.test.tsx` passes on its own (44 of 44) |
| 7 | `make lint-backend` (full golangci-lint v2.15.0, gosec enabled) | **FAIL** | Expected 0 issues; result is 1 issue (finding B1). The first run hit the Makefile's 5-minute loader timeout on a cold cache (exit 4, no lint result). The same pinned image run directly with `--timeout=25m` reports exactly one issue, see B1. The host `golangci-lint` 2.14.0 reports the same single issue |
| 8 | `lefthook run pre-commit --all-files` | PASS | exit 0 (15 min). Includes `golangci-lint-fast`, `go-vet`, `go-vet-agent`, `frontend-type-check`, `frontend-lint`, `shellcheck`, `actionlint`, `markdownlint`, `dockerfile-check`, `check-yaml`, and `semgrep` (367 rules, 2314 targets, 0 findings). The run left the working tree unchanged |
| 9 | `shellcheck --severity=error` on the touched scripts | PASS | exit 0 for `scripts/{cerberus,coraza,crowdsec_decision,crowdsec,debug_rate_limit,integration-test,rate_limit,security_headers,verify_crowdsec_app_config,waf}_integration*.sh`, `scripts/ci/run-playwright-container.sh` and `.docker/docker-entrypoint.sh` (12 files) |
| 10 | `shellcheck` default severity and `bash -n` on the same files | PASS (informational) | `bash -n`: all pass. Default severity reports 20 style and warning notes, all in lines that predate this change (SC2086, SC2034, SC2012, SC2329). The count is 20 on this branch and 21 on `origin/development` (both measured on extracted copies) |
| 11 | `actionlint .github/workflows/quality-checks.yml` | PASS | exit 0 |
| 12 | Docs strict build (`zensical build --strict`) | NOT RUN | zensical is not installed on this host. `markdownlint` passed in gate 8. The new page `docs/troubleshooting/proxy-admin-address.md` is in a manifested directory and in the `docs-site/zensical.toml` nav |
| 13 | GORM security scan | NOT REQUIRED | no path under `backend/internal/models/` changed |
| 14 | CodeQL Go/JS and Trivy | DEFERRED TO CI | the commits are `fix:` and `docs:` scoped; CI runs both on the PR |
| 15 | Gotify token review | PASS | no Gotify token appears in the diff, tests, docs or artifacts |
| 16 | Targeted Playwright `tests/security/self-proxy-requests.spec.ts` | PASS, 8 of 8 | see G-b |
| 17 | Targeted Playwright `tests/integration/import-save-route-regression.spec.ts` on firefox | PASS, 4 of 4 | see G-b |

## Container Gates

Throwaway names used: network `qa-adm-net`, containers `qa-adm-new`, `qa-adm-base`, `qa-adm-probe*`, `qa-adm-c-*`, E2E compose project `qa-adm-e2e` (container `qa-adm-e2e`), and `qaadm-sh` for the scripted header test. Images `charon:qa-adm-new` (built from this branch) and `charon:qa-adm-base` (built from `origin/development` through `git archive`, no worktree). All containers, the network and the E2E volumes were removed after the run. The live `charon` container was not touched.

### G-a: loopback reachability and apply

| Step | Result |
| --- | --- |
| Start `charon:qa-adm-new` on user network `qa-adm-net`, no host ports except loopback API and proxy ports | Caddy ready after 3 probes |
| Probe `http://<container-ip>:2019/config/` from a second container on the same network (stock `wget` in the image) | refused (wget exit 1, "Connection refused") |
| Inside the container, `wget -qO- http://127.0.0.1:2019/config/` | answers; `/config/admin` returns `{"listen":"127.0.0.1:2019"}` |
| Setup, login, create a proxy host through the API, then request through port 80 | 201, 201, 201; route present in `/config/`; request answers HTTP 200 |
| Container restart, repeat the probe, create a second proxy host, request through port 80 | probe refused; login 200; create 201; route present; HTTP 200 |
| `docker logs` after the apply | no apply error. Only ACME certificate retries for the test domains, caused by the throwaway environment having no outbound access and no ZeroSSL key |
| Compared with the previous build (`charon:qa-adm-base`, from `origin/development`), same probe from the second container | answered (wget exit 0, the admin config is returned) |

### G-b: test tooling and E2E compose

- The E2E compose file, rendered with `docker compose -p qa-adm-e2e -f .docker/compose/docker-compose.playwright-local.yml -f <override>`, publishes `8180->80`, `8080->8080` and `2020->2020` only. `docker port qa-adm-e2e` shows no admin port mapping. The override changed only the image tag, the container name and the env file (a generated throwaway env file, not the repository `.env`).
- `docker exec qa-adm-e2e wget -qO- http://127.0.0.1:2019/config/admin` returns `{"listen":"127.0.0.1:2019"}`.
- `tests/security/self-proxy-requests.spec.ts`, run with `PLAYWRIGHT_CONTAINER_NAME=qa-adm-e2e PLAYWRIGHT_CADDY_PROXY_PORT=8180 npx playwright test ... --project=security-tests --reporter=line`: 8 passed (20.7 s). This includes the dependency projects `setup` and `security-shard-setup`. The `security-tests` project is defined in `playwright.config.ts` with `Desktop Chrome`, so this spec ran on chromium, not firefox. No firefox project matches `tests/security`.
- `tests/integration/import-save-route-regression.spec.ts`, `--project=firefox`: 4 passed (30.8 s).

### G-c: startup matrix (`CHARON_CADDY_ADMIN_API`, `--restart=no`, exit status from `docker inspect`)

| Value | Result | Exit status | Log text |
| --- | --- | --- | --- |
| unset | starts, Caddy applied | running | none |
| `http://localhost:2019` | starts | running | none |
| `http://127.0.0.1:2019` | starts | running | none |
| `http://[::1]:2019` | starts | running | none |
| `http://caddy:2019` (service name) | stops at startup | 1 | exact message |
| `http://caddy:2019` with `CHARON_SSRF_INTERNAL_HOST_ALLOWLIST=caddy` | stops at startup | 1 | exact message |
| `http://10.0.0.5:2019` | stops at startup | 1 | exact message |
| `http://example.com:2019` | stops at startup | 1 | exact message |
| `http://127.1:2019` | stops at startup | 1 | exact message |
| `http://0.0.0.0:2019` | stops at startup | 1 | exact message |
| `http://localhost:2020` (wrong port) | stops at startup | 1 | `validate caddy admin api url: invalid caddy admin api url: unexpected port: 2020 (expected 2019)`. Not the exact message, see F2 |

The exact message is `CHARON_CADDY_ADMIN_API must point at this container (localhost); using an external proxy is not supported: set it to http://localhost:2019 or remove it`. The entrypoint's own Caddy starts before the refusal, and the message is printed before the container exits.

### G-d: scripts

- `scripts/security_headers_integration.sh` with `CHARON_IMAGE=charon:qa-adm-new SH_PREFIX=qaadm-sh` ran end to end: exit 0, "ALL SECURITY HEADERS TESTS PASSED" in 28.9 s. The script reads the admin config through `docker exec ... wget`, so this covers the converted read path.
- `scripts/cerberus_integration.sh` and `scripts/waf_integration.sh` were not run end to end. Both bind host ports that the E2E stack uses on this host, and both are covered by the repository scan and `shellcheck`.

## Findings

### B1: gosec G304 in a new test file (blocking, CI lint)

- Location: `backend/internal/caddy/proxy_admin_loopback_test.go:45:21`, `os.ReadFile(filepath.Join(root, ".docker", "docker-entrypoint.sh"))`.
- Output (pinned `golangci/golangci-lint:v2.15.0`, `golangci-lint run -v --timeout=25m ./...` in `backend/`): `G304: Potential file inclusion via variable (gosec)`, `1 issues`.
- Remediation: the sibling helper `scanFileLines` in `test_tooling_admin_access_test.go` already passes its path through `filepath.Clean` and is not flagged. Apply the same to the entrypoint read, or add a justified `//nolint:gosec` on that line. Re-run `make lint-backend` and confirm `0 issues`.

### F2: wrong-port refusal uses the existing port error (low, decision)

- `CHARON_CADDY_ADMIN_API=http://localhost:2020` stops startup with `unexpected port: 2020 (expected 2019)`, not the refusal message. The plan's section 3v.6 specifies the existing port error for this input, and the unit test `TestLoad_CaddyAdminAPIRejectsWrongPort` asserts it.
- If one message for every non-loopback input is wanted, route the port error through the same message. Otherwise no action is needed.

### F3: stale references to removed settings (low, docs)

- `.github/skills/integration-test-security-headers.SKILL.md:108` documents `SH_ADMIN_PORT`, which the script no longer reads.
- `docs/reports/caddy-security-posture.md:24,34` describes the earlier variable validation in a dated report. Update it when that report is next edited.
- `docs/testing/FAILURE_DIAGNOSIS_GUIDE.md:314` and `docs/testing/DEBUGGING_IMPLEMENTATION.md:69,385` show old `localhost:2019` commands run on the host.
- `docs/reports/archive/*` and `docs/implementation/*` are historical records and were not changed.

### F4: test-only exported variable (info)

- `backend/internal/config/config.go` exports `CaddyAdminAPIForTests`, which `Load` applies after validation. Only test helpers set it (`cmd/api/maintenance_sigterm_test.go`, `internal/config/config_test.go`). No environment variable, flag or setting reaches it.

### F5: the Makefile lint target has a 5-minute loader timeout (info)

- On a cold cache, `make lint-backend` exits 4 with `context deadline exceeded` during package loading (`Makefile:199`). The pinned image completes within 7.5 minutes with `--timeout=25m`. Consider raising the target's timeout.

### F6: E2E security project is chromium-only (info)

- `playwright.config.ts` defines `security-tests` with `Desktop Chrome`. A firefox run of `tests/security` is not possible under the current config. The security spec was run on that project as requested.

## Review Notes

- **Admin port publication.** No compose file, script, workflow or Dockerfile `ENV` publishes the admin port or sets the admin URL variable. The `Dockerfile` `EXPOSE 2019` line is kept and publishes nothing. The repository scan `TestProxyAdminAddressIsLoopbackOnly` and `TestTestEnvironmentsUseContainerLocalAdminAccess` guard both points, with one exemption: `scripts/caddy-compat-matrix.sh` runs its own stock Caddy containers.
- **Consumers.** Every host-side reader found by search was converted to `docker exec ... wget -qO- http://127.0.0.1:2019/config/` or removed. No consumer outside the container remains in the repository. The `agent/` module and the Orthrus-related code do not refer to the admin address.
- **Upgrade path.** An operator who sets `CHARON_CADDY_ADMIN_API` to a service name, a container name, `127.1`, or any other non-loopback host gets the exact message and an exit status of 1. Setting `http://localhost:2019` or removing the line works. The troubleshooting page `docs/troubleshooting/proxy-admin-address.md` documents both steps.
- **Wording.** The commit subjects, the error text, the docs and the test names describe the change only in the neutral terms listed in the change description above.
- **Test results on the touched packages** are listed under gates 1 to 6.
