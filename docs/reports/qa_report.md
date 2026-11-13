# QA Report: Redirect Target Handling

- Branch: `fix/redirect-target-handling` (8 commits on `origin/development`, up to `b649a9641`, plus the
  uncommitted QA change listed below)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 4 (fix PR)
- Scope: user-supplied values are written to proxy config as literals (redirect targets, domain matchers, location
  paths, upstream dial addresses, security header profile values); redirect targets with curly braces are refused on
  create and update with `error_code: redirect_target_invalid`; a startup warning names stored rows that the new rule
  refuses; translated form message in all locales; docs; plus a Dockerfile comment cleanup and lefthook PATH change
  carried on the branch.
- Image under test: `charon:local` from `docker-rebuild-e2e` on the committed branch.
- Date: 2026-11-12

## Verdict

**Pass.** Every Definition of Done gate for a fix PR passes. Local patch coverage is 100% (55 of 55 changed lines).
No CRITICAL or HIGH findings. The throwaway-container check confirms that stored braced targets are served as
literals and that brace-free configuration is byte-identical to `origin/development`.

## Changes made during QA

| Area | Change |
| --- | --- |
| `scripts/pre-commit-hooks/codeql-go-scan.sh`, `codeql-js-scan.sh` | The closing `echo` held backticks inside double quotes, so the shell ran a nested `lefthook run pre-commit` at the end of every CodeQL scan. The backticked text is removed; the message is otherwise unchanged. Not related to this branch; suitable as its own `chore:` commit. |

No product code was changed.

## Static checks and build

| Check | Result |
| --- | --- |
| `make lint-backend` (golangci-lint v2.15.0, full config) | 0 issues |
| `lefthook run pre-commit --all-files` | all 20 hooks pass, including semgrep (0 findings), golangci-lint-fast, frontend lint/type-check, locale lint, e2e lint (0 errors, warnings only, pre-existing) |
| `cd frontend && npm run lint` | pass, 0 warnings |
| `node_modules/.bin/tsc6 --noEmit` (TypeScript 6.0.3) | pass |
| `cd frontend && npm run build` | pass |
| `cd backend && go build ./...` | pass |
| `./scripts/scan-gorm-security.sh --check` | 0 issues (1 pre-existing suppressed) |

## Coverage

| Run | Result |
| --- | --- |
| `scripts/go-test-coverage.sh` (full) | all packages pass; statements 93.7%, lines 92.6% (gate 87%) |
| `scripts/frontend-test-coverage.sh` (full) | 346 files, 4473 tests pass (2 todo); statements 91.77%, lines 92.66% |
| `bash scripts/local-patch-report.sh` | overall 100.0% (55/55), backend 100.0% (49/49), frontend 100.0% (6/6) |

## E2E (Playwright, after the image rebuild)

| Command | Result |
| --- | --- |
| `PLAYWRIGHT_CADDY_PROXY_PORT=8180 npx playwright test tests/redirection-hosts.spec.ts --project=firefox` | 16 passed, including the four new "Redirect Target Safety" tests |
| `npx playwright test tests/security-enforcement/acl-enforcement.spec.ts tests/security-enforcement/security-headers-enforcement.spec.ts --project=security-tests` | 18 passed |

The image rebuild succeeded with the Dockerfile on this branch.

## Security scans

| Scan | Result |
| --- | --- |
| CodeQL Go + JS (`lefthook run codeql`, CLI 2.27.2) | 0 blocking. Go: 4 results, all pre-existing entries in `codeql-suppressions.yml`, none in files of this branch. JS: 0 results. Extraction and parity checks pass. |
| Semgrep (pre-commit hook) | 0 findings |
| Trivy | Not run locally (fix PR, no new feature surface); CI runs it on the PR. |

## Spot check (throwaway container)

A separate `charon:local` container with a fresh named volume, published on loopback-only ports and removed
afterwards. `charon-dev` and `charon` were not touched. The container had an extra environment variable holding a
canary value. Four redirection rows were inserted straight into the database before start: two with braced targets
(one with Preserve Path), one plain, one plain with Preserve Path.

| Step | Result |
| --- | --- |
| Startup log | One warning per braced row, with `host` and `redirection_host_uuid` only. The stored target, its placeholder names and the canary value do not appear in the log. |
| Live Caddy config (`/config/`) | Braced targets appear as `%7B...%7D`. The only raw placeholder in a redirect `Location` is the generator's own `{http.request.uri}` on Preserve Path rows. The canary value is absent. |
| Request to the braced host with a probe request header | `302`, `Location` carries `%7Benv...%7D` and `%7Bhttp.request.header...%7D` literally; neither the canary nor the probe header value appears. |
| Braced host with Preserve Path | `Location` is the encoded target followed by the requested path and query. |
| Plain host / plain host with Preserve Path | `301` to the fixed target; `308` with the requested path and query appended. Unchanged behaviour. |
| Edit the braced row (name only, or a new braced target) | `400`, `error_code: redirect_target_invalid`, fixed message, input not echoed |
| Create a braced row | Same `400` |
| Edit the row to a plain target | `200`; the next request redirects to the new target |

### Brace-free configuration is unchanged

The repo has no golden config files, so a differential check was run instead: one throwaway test (not committed)
generated the full Caddy config from the same fixtures against an export of `origin/development` and of this branch,
and the two JSON outputs were compared. Fixtures: a multi-domain host with two locations (one IPv6 upstream), HSTS,
websockets and standard headers; a Plex host with standard headers off; a wildcard host with a full security header
profile (HSTS preload, CSP, Permissions-Policy, cross-origin headers, cache control); an IP-only host with an
advanced-config header handler; two redirection hosts (one with Preserve Path); Cerberus, WAF, rate limit and ACL on.
Result: **byte-identical** (706 lines of JSON).

## Security review of the diff

| Area | Result |
| --- | --- |
| `caddy/literal.go` | `proxyLiteral` replaces only `{` and `}` with `%7B`/`%7D` through a `strings.Replacer`; brace-free input is returned unchanged. `proxyLiterals` copies, so the caller's slice is not modified. Table tests cover empty, plain, IPv6, existing escapes, nested and mixed braces. |
| `caddy/redirect_routes.go` | The user target is encoded before the generator appends `{http.request.uri}`, so only the generator's placeholder survives. Domain matchers are encoded too. |
| `caddy/config.go`, `types.go` | Host matchers, location path matchers and upstream dial addresses go through the helper. Generator-emitted placeholders (standard proxy headers, Plex headers, rate-limit key, self-upstream headers) are not routed through it and keep working. `advanced_config` is raw JSON by design and out of scope (spec). |
| Header value encoding | Applied to the security header profile values only, after the map is built. Profile values never carry generator placeholders, so this is safe; HSTS, CSP and Permissions-Policy output is unchanged for brace-free values (differential check). |
| `services/redirectionhost_service.go` | The brace check runs after trimming and before URL parsing; percent-encoded input is accepted. The error is a fixed sentinel and never contains the submitted value. |
| `services/redirectionhost_target_check.go` | Selects only `uuid`, `domain_names`, `target_url`; logs host and UUID, never the target; a read failure logs and returns 0 without blocking startup. |
| Handler error body | `respondRedirectionHostWriteError` returns the fixed message plus `error_code` for the sentinel; other errors keep the previous behaviour. Handler tests assert the body does not echo the input. |
| Frontend | The form maps `redirect_target_invalid` to a translated message and falls back to the previous text for other errors. All locale files carry the key (parity test). |
| Gotify tokens | None in the diff, test artifacts or logs reviewed. |

## Findings

No CRITICAL, HIGH or MEDIUM findings.

### Low: Dockerfile comment cleanup removed correct apostrophes

`chore(docker): correct typos in comments and documentation` and the follow-up quoting fix leave 18 comment
lines where possessives and contractions lost their apostrophe (for example "Debians pre-compiled version",
"Caddys dependency patches", "its cheap", "workflows bot PR"). Comments only; the image builds and nothing changes
at runtime. Apostrophes inside Dockerfile comment lines are safe even within a `RUN` continuation, because the
parser drops comment lines before the shell sees them. Remediation: restore the apostrophes in those comment lines
(a `git diff origin/development -- Dockerfile` shows each one), or drop the Dockerfile commits from this PR, since
they are outside section 4 of the spec.

### Low: docs wording for existing braced redirects

`docs/features/redirection-hosts.md` says an existing braced redirect "keeps working as before". It keeps
redirecting, but the braces are now sent as `%7B` and `%7D`, which is a change for anyone who relied on them. The
startup warning already says "served as a literal". Remediation: reword to "it keeps redirecting, with the braces
sent as `%7B` and `%7D`", which says what happens without describing why.

## Acceptance criteria (spec section 4)

| Criterion | Evidence |
| --- | --- |
| User target written as a literal; generator still appends `{http.request.uri}` for Preserve Path | `TestRedirectTargetIsLiteral`; spot check |
| Braced target refused with `redirect_target_invalid` (400); percent-encoded accepted | `TestRedirectTargetRejectsReservedCharacters`, handler tests, E2E "Redirect Target Safety", spot check |
| Audit: domains, location paths, dial addresses, header profile values written as literals | `literal_test.go`; config review |
| Brace-free input byte-identical | `TestProxyLiteral_BraceFreeInputUnchanged`; differential config check |
| Startup warning names the host, not the value | `TestWarnRefusedRedirectTargets_WarnsOncePerRowNamingHostNotValue`; spot check |
| Form explains the refusal in every locale | `RedirectionHostForm.test.tsx`, `errorBody.redirection.test.ts`, `redirectionHostLocales.test.ts` |
| Container gate: seeded braced row yields `%7B...%7D`, never an expanded value | spot check |
| Full DoD | this report |

## Deviations from the spec

- The branch also carries `chore: call npm and npx from PATH in the lefthook frontend hooks` and two Dockerfile
  comment commits that are not part of section 4. They do not affect the fix; see the low finding for the comments.
- Spec's commit 4 lists two E2E cases; the branch has four (adds an API refusal case and a percent-encoded target case).
- Trivy was not run locally (fix PR); CI runs it.
