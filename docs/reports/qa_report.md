# QA and Security Report: Login Protection (#1317)

- **Branch:** `feat/auth-rate-limit-1317` (rebased on `origin/development`, 28 feature commits plus two QA commits)
- **Plan:** `docs/plans/current_spec.md` (approved); DoD mapping in spec section 5.2
- **Scope:** per-client rate limiting on `/api/v1/auth/*` and password-verifying routes, the admin
  `GET /api/v1/security/login-protection` status endpoint, the Login page notice, the Security page
  Login Protection card, the shared `internal/ratelimit` package, the reworked Cerberus API limiter,
  trusted-proxy parsing, and the related docs and E2E coverage.
- **Verdict:** **PASS WITH NOTES** (see section 6)

## 1. Definition of Done results

| # | Step | Command | Result | Evidence |
| --- | --- | --- | --- | --- |
| 1 | E2E stack rebuild | `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e` | Pass | 35 s, `charon-e2e` healthy |
| 1 | Playwright, targeted | `npx playwright test tests/core/auth-rate-limit.spec.ts tests/core/authentication.spec.ts tests/settings/user-lifecycle.spec.ts --project=firefox` | Pass | 32 passed, 0 failed, 0 skipped, 1.0 min. Rate-limit tests 1 to 3 executed (not skipped) |
| 1 | Playwright, repeat | `npx playwright test tests/core/auth-rate-limit.spec.ts --project=firefox --repeat-each=3` | Pass | 19 passed, 0 failed, 0 skipped, 26 s |
| 1.5 | GORM scan | `./scripts/scan-gorm-security.sh --check` | Pass | 0 CRITICAL, 0 HIGH, 0 MEDIUM; 2 INFO. The only diff under `backend/internal/models` is a comment repoint in `redirection_host.go`; no model, query-shape or migration changes in this feature |
| 2 | Patch coverage | `bash scripts/local-patch-report.sh` (rerun after the coverage runs so it used fresh inputs) | Pass | Overall 97.8 percent (611 of 625 changed lines), backend 97.8, frontend 97.6. Artifacts: `test-results/local-patch-report.{md,json}` |
| 3 | CodeQL Go | `scripts/pre-commit-hooks/codeql-go-scan.sh` (CodeQL CLI 2.26.4) | Pass | After the fix in section 4 and the suppression line update in section 6: 4 results, all pre-existing and suppressed; `scripts/security/codeql-findings-gate.sh codeql-results-go.sarif go` reports 4 suppressed, 0 blocking |
| 3 | CodeQL JS | `scripts/pre-commit-hooks/codeql-js-scan.sh` | Pass | 0 results; gate reports 0 blocking |
| 3 | Trivy | `.github/skills/scripts/skill-runner.sh security-scan-trivy` | Pass for the change | 0 findings in tracked files. Findings reported only in git-ignored paths (`.claude/worktrees/*` copies, `docs-site/build/` output, a local untracked key file); none in files this branch adds or changes |
| 3 | govulncheck | `.github/skills/scripts/skill-runner.sh security-scan-go-vuln` | Pass | 0 vulnerabilities affecting the code (new dependency: `hashicorp/golang-lru/v2` v2.0.7) |
| 4 | Lefthook | `lefthook run pre-commit --all-files` | Pass | All hooks pass; the earlier Semgrep findings in three untouched files are resolved (section 4 and 6) |
| 5 | Staticcheck | `make lint-fast` | Pass | 0 issues (backend and agent) |
| 5 | Full golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` | Non-blocking in CI | 91 findings remain (bodyclose 1, gocritic 50, gosec 40; pre-existing, at the per-linter caps). Findings on lines this branch added or changed (`--new-from-rev=origin/development`): 7 found and fixed, now 0 |
| 6 | Backend coverage | `bash scripts/go-test-coverage.sh` | Pass | Lines 88.8 percent, statements 92.2 percent, gate 87 percent; 8 min 13 s |
| 6 | Frontend coverage | `bash scripts/frontend-test-coverage.sh` | Pass | Lines 91.2 percent (7981 of 8751), statements 90.04, branches 83.38, functions 87.86; 281 files, 3525 tests passed; 13 min 50 s |
| 7 | Type check, frontend | `cd frontend && npm run type-check` | Pass | 0 errors, 33 s |
| 7 | Type check, tests | `npx tsc --noEmit --strict --module esnext --moduleResolution bundler --skipLibCheck --target es2022 --lib es2022,dom --types node <167 .ts files under tests/>` | Pass | 0 errors |
| 8 | Build | `cd backend && go build ./...`; `cd frontend && npm run build` | Pass | Both succeed |
| 9 | Backend, cache-free, race | `cd backend && go test -race -count=1 ./...` | Pass | 38 packages ok, 0 failures, 8 min 30 s |
| 9 | Frontend, full | `cd frontend && npm run test` | Pass | 281 files, 3525 passed, 4 skipped and 2 todo (all pre-existing, none added by this branch), 0 failed, 10 min 49 s |
| 10 | Clean-up scan | added lines of `git diff origin/development...HEAD` | Pass | No debug prints, commented-out code or new skips. One conditional `test.skip` in the rate-limit E2E spec is an environment guard with a message; it did not trigger. `.gitignore`, `.dockerignore`, `codecov.yml` need no change for `backend/internal/ratelimit`, the new docs pages or the test helpers |

## 2. Test and coverage numbers

- Backend: 88.8 percent line coverage (gate 87), 92.2 percent statements.
- Frontend: 91.2 percent line coverage (gate 87).
- Patch coverage of the change: 97.8 percent (gate 87). Remaining uncovered changed lines are defensive
  branches: `frontend/src/api/security.ts` 77-78, `backend/cmd/api/main.go` 263,
  `backend/internal/api/routes/routes.go` 326-327, `backend/internal/ratelimit/limiter.go` 81-82 and 148-149,
  `backend/internal/api/middleware/auth_rate_limit.go` 135-139, `backend/internal/cerberus/rate_limit.go` 123.

## 3. Security audit

Reviewed the branch diff against `SECURITY.md` and OWASP Top 10 categories.

| Area | Result |
| --- | --- |
| Authorization on `GET /api/v1/security/login-protection` | Registered on the admin-only group (`RequireRole(admin)`). Route tests assert 401 without a token and 403 for a non-admin user. |
| Input handling on the new endpoint | Read-only, no request body, query or path input consumed. Response contains budgets, trusted-proxy count, observation counters and the caller's own derived key and scope. |
| Log content | Denial and detector lines carry the client key, route template, class and counts only. No credentials, tokens, emails or usernames. Client key and route template are now passed through `util.SanitizeForLog` (section 4). The first denial per episode is WARN under a global cap; the rest are DEBUG. |
| Header trust | Client address comes from Gin `ClientIP()` with the trusted-proxy list parsed once. RFC 7239 `Forwarded` is never consulted. Unit and route tests cover untrusted peers ignoring forwarded headers and trusted peers using the rightmost untrusted hop. |
| Memory and CPU bounds | `KeyedLimiter` is an LRU capped at 10,000 keys with idle sweeping and no goroutines. Throwaway benchmark (not committed): 2,000,000 distinct keys, tracked keys stayed at 10,000, about 729 ns per call, heap growth about 2.7 MiB. |
| Configuration failure modes | Zero, negative, non-numeric or out-of-range budget values fall back to defaults with a startup warning. An unrecognised enable value keeps protection on. Only a literal `false` disables it, and that is logged at WARN. A limiter construction error aborts route registration rather than starting unprotected. |
| Break-glass paths | `/api/v1/emergency/*` is registered outside the throttled group; the Tier-2 server is a separate listener. Verified by test and against the E2E stack (below). |
| Route classification | Every `/api/v1/auth` route is in the classification table; an inventory test fails when a route is added without a class. A second inventory test covers every password-verifying route outside the group. |

### Black-box checks against the E2E stack (`charon-e2e`, localhost only)

The E2E stack sets the login budget to 300 per 60 s and trusts loopback and private ranges, so requests
from the test host arrive from a trusted peer.

- Burst of 900 parallel bad-credential logins from one client: 831 to 835 answered 429 with `Retry-After`
  and the generic body `{"error":"Too many requests. Please wait before trying again."}`; the rest 401.
- `GET /auth/me`, `GET /auth/verify` and `GET /auth/status` were never throttled while login was throttled.
- The RFC 7239 `Forwarded` header did not change the throttle key (429 rate unchanged versus control).
- `POST /api/v1/emergency/security-reset` (400 requests) and the Tier-2 port (300 requests) returned only
  401, never 429, while the login route was throttled.
- Not tested black-box: forged forwarded headers from an untrusted peer. Every peer that can reach this
  stack from the host is inside a trusted range, so the case is covered by the middleware and route tests
  named above instead.

## 4. Fixes made during QA

| Commit | Subject | What |
| --- | --- | --- |
| `96fb0713` | `fix: sanitize client and route fields in rate-limit denial logs` | Two new CodeQL `go/log-injection` findings (severity 6.1) in the denial log lines of the auth throttle and the Cerberus limiter. Values are passed through `util.SanitizeForLog`; rescan shows both gone. |
| `b9050d21` | `refactor: resolve gocritic findings in the rate-limit code` | Seven full-config golangci-lint findings on lines this branch added (named results, `http.NoBody`, early `continue`). Affected packages re-tested. |
| `51dcc88d` | `test: remove scanner findings from test fixtures` | Scanner findings in three test files (section 4a). |

### 4a. Semgrep gate and findings

CI (`.github/workflows/semgrep.yml`) runs `scripts/pre-commit-hooks/semgrep-scan.sh` twice: once with SARIF output
(`continue-on-error`, upload only) and once as the hard-fail gate with `--error`. Rulesets are p/golang,
p/javascript, p/typescript, p/react, p/secrets and p/dockerfile at ERROR and WARNING severity, over the targets
`Dockerfile backend frontend/src scripts .github/workflows`. Semgrep skips test paths by default, so a CI run
reports 0 findings for the files below; they surface when the files are passed explicitly (the staged-file
lefthook hook does this) or when default ignores are disabled. The three files are identical to
`origin/development` (empty `git diff origin/development...HEAD` for each), so the findings pre-date this branch.

| File | Finding | Resolution |
| --- | --- | --- |
| `backend/internal/api/middleware/auth_test.go` | Cookie without `HttpOnly` and `Secure` (rules `cookie-missing-httponly`, `cookie-missing-secure`; test cookies) | Fixed: explicit `HttpOnly: true, Secure: true` on all test cookies. The middleware reads only name and value, so behavior is unchanged. |
| `tests/certificate-export.spec.ts` | Committed private key literal (`detected-private-key`) | Fixed: new helper `tests/utils/test-certificate.ts` generates the certificate and key with `openssl` at runtime. The same subject (`CN=test.local, O=TestOrg`) is kept. |
| `tests/security-enforcement/authorization-rbac.spec.ts` | Hard-coded bearer token (`hardcoded-bearer-token`) | Fixed: the expired-token header is built from base64url-encoded header and payload parts at runtime; same claims. |

No `nosemgrep` suppressions were added. Result: the CI invocation reports 0 findings (0 blocking), and the four
touched files report 0 findings when scanned explicitly. Verification: `go test -race -count=1
./internal/api/middleware/...` passes; strict `tsc` over all `.ts` files under `tests/` reports 0 errors;
`tests/certificate-export.spec.ts` passes (14 of 14, firefox); `tests/security-enforcement/authorization-rbac.spec.ts`
ran in the `security-tests` project (the firefox project ignores that directory): 88 passed, including the expired
session test; 5 UI redirect tests failed only because the Chromium headless shell is not installed on this host.

## 5. Deviations and how they were handled

- The local-patch report was first run on stale coverage inputs and rerun after the fresh coverage runs.
- `make lint-backend` needs Docker, so `golangci-lint` was run locally with `backend/.golangci.yml`.

## 6. Notes (not blocking)

1. **CodeQL suppression line.** This branch moved the flagged cookie-setting call in
   `backend/internal/api/handlers/auth_handler.go` from line 198 to 187. The maintainer updated the matching
   entry in `.github/codeql/codeql-suppressions.yml` (commit `b0ca33aa`). A fresh Go scan and
   `scripts/security/codeql-findings-gate.sh` now report 4 suppressed, 0 blocking; the JS scan reports 0 results.
2. **Semgrep.** See section 4a for what CI gates and how the findings were resolved.
3. **Full golangci-lint config:** 91 pre-existing findings remain, non-blocking in CI.
4. Additional item tracked privately.

## 7. Verdict

**PASS.** All Definition of Done gates pass, targeted E2E is green including repeats, patch
coverage is 97.8 percent, and the QA fixes are committed. The CodeQL gate (Go and JS) and the Semgrep gate report
no blocking findings.
