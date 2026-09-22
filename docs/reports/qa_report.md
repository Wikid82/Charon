# QA & Security Report — Redirection Hosts Feature (#1367)

- **Feature branch:** `feat/redirection-hosts-1367`
- **Commits audited:** `cbe72b50`, `443f16c7`, `525c63b5`, `f7e72240`, `fad3d74b`, `8fdd4401`, `90bbcbea`, `c2288d6c`, `2dcdb9fc`, `d320c7ed`, `14e1d26a` (11 landed code/test/docs commits, per the Commit Slicing Strategy in `docs/plans/current_spec.md` §9)
- **Plan:** `docs/plans/current_spec.md`
- **Prior review:** Supervisor code-review pass already approved diff fidelity, the `GenerateConfig` edit-scope claim, the three inserted fixes, and a preliminary security read. This report is an independent audit, not a rubber stamp — one new bug was found and confirmed by writing and running a reproduction test (not by inspection alone).

> **Post-audit update**: Finding #1 below (`RedirectionHost.Enabled` GORM
> zero-value bug) was fixed immediately after this audit, in commit
> `5b96f9724f65fa88a026adcb9b30d9b0cb15886f` (`fix: persist explicit false
> value for redirection host enabled flag`), documented as commit 7.5 in
> `docs/plans/current_spec.md` §9. Red→green regression test confirmed;
> full validation gate re-passed. Finding #2 (patch-coverage strict-gate
> shortfall) and the certificate-badge UI-consistency gap noted in §5 were
> accepted as tracked follow-ups rather than expanding this PR further —
> both are non-blocking per this report's own §6/§7 recommendation.

## Overall Verdict: **CONDITIONAL PASS** → resolved to **PASS** (see update above)

The feature is functionally solid, passes every automated gate that measures whole-codebase health (both coverage floors, both security scanners, full test suites, all builds), and the core security properties claimed for this feature (no server-side fetch of redirect targets, self-redirect guard, cross-table uniqueness, cert-integrity on delete, no new authorization surface) are all independently verified as true. It should **not** be blocked indefinitely, but two items should be fixed (or explicitly accepted in writing by the person who owns this decision) before merge:

1. **One genuine, confirmed bug** (Medium severity): `RedirectionHost.Enabled` has the exact same GORM zero-value/default-tag collision that commit `2dcdb9fc` already fixed for three sibling fields — but the fix missed `Enabled` itself. `POST /redirection-hosts` with `"enabled": false` silently persists as `true`. Not reachable through the current UI (the create form never sends `enabled`), but it is part of the documented API contract (§4.4) and defeats the ability to create a host in a pre-disabled state via the API.
2. **Patch-coverage preflight fails its own strict gate**: `scripts/local-patch-report.sh` reports 79.0% backend / 80.7% frontend patch coverage against the required 85%/90% thresholds (this is a *diff*-coverage metric, distinct from and additional to the whole-file coverage gates in `scripts/go-test-coverage.sh`/`scripts/frontend-test-coverage.sh`, both of which pass comfortably). The uncovered lines are concentrated in real error-handling branches (see §2 below), not incidental formatting.

Neither finding is a security hole in the sense the task asked me to hunt for (SSRF, redirect-loop bypass, cross-table uniqueness bypass, cert-lifecycle gap, authorization regression) — all five of those came back clean on independent verification (§5). They are correctness/completeness gaps that a QA/security pass is supposed to catch before merge.

---

## 1. Definition of Done — Item-by-Item

| # | Item | Result |
|---|---|---|
| 1 | Targeted Playwright E2E: `npx playwright test tests/redirection-hosts.spec.ts --project=firefox` | **PASS** — run twice, foreground, blocking. **12/12 passed both times**, ~24-27s each run, zero flakiness observed. No third run needed. |
| 1.5 | GORM security scan: `./scripts/scan-gorm-security.sh --check` | **PASS** — 0 CRITICAL, 0 HIGH, 0 MEDIUM. 2 pre-existing INFO-level suggestions on an unrelated model (`UserPermittedHost`), not this feature. |
| 2 | `bash scripts/local-patch-report.sh` | **WARN (below strict threshold)** — see §2. Artifacts produced at `test-results/local-patch-report.md` / `.json`. |
| 3 | Security scans (feat-scoped, run locally): CodeQL Go/JS (`lefthook run codeql`), Trivy | **PASS** — see §3. |
| 4 | `lefthook run pre-commit` full triage | **PASS (trivially)** — nothing staged (all feature commits already landed on the branch), so all hooks report "no matching staged files." The substantive checks this would run (staticcheck, go vet, frontend type-check) were run explicitly and directly instead (items 5, 7) rather than relying on this no-op. |
| 5 | `make lint-staticcheck-only` / `make lint-fast` | staticcheck: **PASS**, 0 issues. `lint-fast` (broader govet/errcheck/ineffassign/unused): 2 findings, **both pre-existing and outside this PR's changed files** (`backend/cmd/api/main.go:261`, `backend/internal/api/handlers/docker_handler.go:47` — blamed to commits from March/May 2026, confirmed via `git diff origin/main...HEAD` touching neither file). Not a regression from this feature. |
| 6 | `scripts/go-test-coverage.sh` / `scripts/frontend-test-coverage.sh`, run alone | **PASS**, both comfortably above the 87% internal gate. Backend: 91.9% statement / **88.5% line**. Frontend: 89.51% statement / **90.82% line**. Run sequentially, not concurrently with any other heavy job, per the resource-contention lesson from the prior feature's QA pass. |
| 7 | `cd frontend && npm run type-check` | **PASS**, zero errors. |
| 8 | `go build ./...` / `npm run build` | **PASS**, both clean. |
| 9 | Full `go test ./...` and full `npx vitest run` (not just touched files) | **PASS** — backend: **37/37 packages, 0 failures**. Frontend: **276/276 test files, 3450 tests passed, 4 skipped, 2 todo, 0 failures**. No regressions anywhere else in the app from the Caddy/certificate changes. |
| 10 | Clean-up check across all ~41 touched files (`git diff --name-only origin/main...HEAD`) | **PASS** — no `console.log`, `fmt.Println`, `debugger;`, or commented-out dead code found. Two grep hits on `// ...` lines were verified false positives (a doc comment and a legitimate inline comment above a real `return` in a test stub). |

## 2. Patch/Diff Coverage — Detail

Regenerated with fresh coverage data (backend `coverage.txt` and frontend `lcov.info` were stale on first run — the E2E-era files predated today's coverage runs; re-ran `local-patch-report.sh` after regenerating both via the coverage scripts, per the task's own instruction to distinguish real gaps from stale-artifact noise).

| Scope | Changed Lines | Covered | Patch % | Threshold | Status |
|---|---:|---:|---:|---:|---|
| Overall | 601 | 477 | 79.4% | 90.0% | WARN |
| Backend | 461 | 364 | 79.0% | 85.0% | WARN |
| Frontend | 140 | 113 | 80.7% | 85.0% | WARN |

Files below their own patch threshold, with what's actually missing (verified by reading the source, not just the line numbers):

- **`backend/internal/api/handlers/redirection_host_handler.go` (61.8%, 83 uncovered lines)** — the biggest gap, and the most substantive. Untested branches include: the numeric-ID success path and the "value is neither a valid ID nor a string" type-error path in both `resolveCertificateReference`/`resolveDNSProviderReference`; the DB-failure-other-than-not-found branches in both resolvers; `parseStatusCodeField`'s `int` and `string` JSON-type branches (only the `float64` path — what `encoding/json` normally produces — is exercised); and the `ApplyConfig` failure/rollback branch in `Create` and the failure branch in `Update`/`Delete` (the handler tests appear to pass a nil `caddyManager`, so `if h.caddyManager != nil` is never entered — worth checking whether `ProxyHostHandler`'s own tests exercise this branch for comparison, since if they do, this is a real gap relative to precedent).
- **`frontend/src/components/RedirectionHostForm.tsx` (63.1%, 24 uncovered lines)** — form-state edge cases in the ~75-103 range (initial-state derivation branches) and a handful of scattered single lines.
- **`backend/internal/caddy/manager.go`, `certificate_service.go`, `routes.go`, `redirectionhost_service.go`** — small (2-8 line) gaps, lower risk given the surrounding logic is otherwise well covered (redirectionhost_service.go is at 90.6%).

This is a real, actionable gap, not a false alarm from stale baselines — I regenerated the coverage inputs before drawing this conclusion. It doesn't block on the "85% minimum" language in CLAUDE.md (that language is about whole-codebase coverage, which passes), but the patch-coverage preflight is listed as **MANDATORY** in CLAUDE.md's workflow, and the script itself exits non-zero in strict mode specifically to signal this shouldn't be merged silently. Recommend a small follow-up commit adding tests for: `resolveCertificateReference`/`resolveDNSProviderReference`'s type-error and DB-failure branches, `parseStatusCodeField`'s `int`/`string` cases, and (if not already precedented in `ProxyHostHandler`) an `ApplyConfig`-failure/rollback test for `RedirectionHostHandler.Create`.

## 3. Security Scans — Detail

- **CodeQL Go** (`lefthook run codeql`, `codeql/go-queries:codeql-suites/go-security-and-quality.qls`, matching CI's suite exactly): 4 results, **all 4 suppressed via `codeql-suppressions.yml`, 0 blocking**. All four are pre-existing, previously-reviewed suppressions in `auth_handler.go`, `remote_server_handler.go`, and `uptime_service.go` — none in any file this feature touches. Extraction-count parity check passed (CodeQL compiled the same file count as `go list`).
- **CodeQL JS/TS**: 568/568 files scanned, **0 findings**.
- **CodeQL parity check**: passed (workflow triggers, suite pinning, local/CI alignment all consistent).
- **Trivy** (`aquasec/trivy image --severity CRITICAL,HIGH` against `charon:local`, confirmed built at a commit including all of this feature's application-code changes — `2dcdb9fc` is the latest app-code commit and predates the image build timestamp; `d320c7ed`/`14e1d26a` are test/docs-only): **`app/charon` (Charon's own binary, where 100% of this feature's Go code compiles to): 0 vulnerabilities.** The only HIGH finding anywhere in the image is `CVE-2026-32286` in `usr/local/bin/crowdsec`/`cscli` (bundled third-party binary, `jackc/pgproto3/v2`) — already tracked in `SECURITY.md` as a pre-existing, awaiting-upstream, non-default-config-path finding, unrelated to this feature. Alpine base: 0 findings.
- No new dependencies were introduced by this PR (`git diff origin/main...HEAD` on `go.mod`/`go.sum`/`package.json`/`package-lock.json` is empty), so this is exactly the expected result — nothing to remediate.

## 4. GORM Security Scan — Detail

0 CRITICAL/HIGH/MEDIUM across 65 scanned Go files (4171 lines). The only output is 2 pre-existing INFO-level "missing index" suggestions on `UserPermittedHost`, unrelated to this feature. 1 suppressed issue (not shown without `--verbose`; not investigated further since it is below the INFO tier and pre-existing).

## 5. Independent Security-Specific Review

Each of the five items the task asked me to verify independently (not by trusting the prior review) — done by reading the actual code, not by re-reading the plan:

**(a) Redirect targets are never fetched/dialed server-side.** Confirmed by reading `backend/internal/caddy/redirect_routes.go` and `backend/internal/caddy/types.go`'s `RedirectHandler`. The target URL flows: `RedirectionHost.TargetURL` (DB) → `BuildRedirectRoutes` (string concatenation only, appends `{http.request.uri}` as a literal Caddy placeholder token, never resolved/dialed by Go code) → `RedirectHandler(location, statusCode)` → a `Handler{"handler": "static_response", "status_code": ..., "headers": {"Location": [...]}}` map that is JSON-marshaled into Caddy's config and POSTed to Caddy's admin API. There is no `net/http` client call, no `net.Dial`, no DNS resolution of the target anywhere in this path — Caddy itself only ever *emits* the string in a response header; it does not fetch it either (this is exactly what `static_response` means, as opposed to `reverse_proxy`). Confirmed clean.

**(b) The self-redirect guard actually prevents a redirect loop.** Read `validateRedirectionHost` in `backend/internal/services/redirectionhost_service.go` directly (not just the spec's description of it). Logic: `parsed.Hostname()` (lowercased) is compared against each lowercased, trimmed entry in `host.DomainNames` split on `,`. This correctly catches the direct case (source domain == target host, case-insensitively, and correctly ignores the target's port/path/scheme when comparing since `Hostname()` strips the port). It does **not** catch indirect loops (A→B, B→A across two different `RedirectionHost` rows) — but that is explicitly out of scope per the spec's Non-Goals (§1.3: "detecting `A→B→A` chains across multiple hosts is not attempted in v1"), so this is working as designed, not a gap. E2E test `redirection-hosts.spec.ts:389` independently confirms the guard fires through the real API, not just a unit-test mock.

**(c) Cross-table domain uniqueness genuinely can't be bypassed in either direction.** Read `domain_uniqueness.go`'s `CheckDomainConflict` and both call sites. `RedirectionHostService.Create`/`Update` call `CheckCrossTableDomainConflict` (→ `ProxyHost` table) in addition to their own same-table `ValidateUniqueDomain`; `ProxyHostService.Create`/`Update` gained the mirrored call in the other direction (confirmed via `git diff` — additive only, the existing `ValidateUniqueDomain` call is byte-for-byte unchanged). Both directions are independently unit-tested: `domain_uniqueness_test.go` (7 tests covering conflict-found, case-insensitivity, multi-domain, no-conflict, empty input, missing-other-table, DB-error) and `proxyhost_redirection_conflict_test.go` (3 tests specifically proving a `ProxyHost` create/update is rejected when the domain is already a `RedirectionHost`'s, and that a non-conflicting create succeeds). E2E test `redirection-hosts.spec.ts:407` proves it end-to-end through the real API for the RedirectionHost→ProxyHost direction. Comparison is per-individual-domain (both sides split on comma, lowercase, trim) rather than whole-string, so it correctly catches partial overlaps between a multi-domain `ProxyHost` and a single-domain `RedirectionHost` that the pre-existing same-table `ProxyHost` check would miss (that gap is explicitly and correctly left alone for `ProxyHost`-vs-`ProxyHost`, per the spec's stated Non-Goal). Confirmed sound in both directions.

**(d) Certificate lifecycle fixes (commits 5/6) are complete — but two adjacent gaps were found that those commits didn't cover.** The two safeguards those commits actually added are correct and verified: the startup sweep (`routes.go:202`, `cleanInvalidLetsEncryptCertAssignments(db, "redirection_hosts", ...)`) and the delete-time block (`certificate_service.go:636-657`, `IsCertificateInUse` now checks both `ProxyHost` and `RedirectionHost`, guarded by `HasTable` so ProxyHost-only test DBs are unaffected). I then searched for every other `certificate_id`-querying call site in `certificate_service.go` to check for siblings that should have gotten the same treatment, and found two that were missed:
  - `refreshCacheFromDB` (line ~284-291, feeds `ListCertificates()`'s `in_use` field shown on the certificates list page) builds its `certInUse` map from `ProxyHost` only. A certificate used *only* by a `RedirectionHost` will show `in_use: false` on the certificate list, even though `DeleteCertificate` will correctly still block its deletion. This is a UI-consistency bug (confusing, not unsafe — the block still holds), not a security hole.
  - `GetCertificate` (line ~517-536, feeds the certificate detail page's `assigned_hosts` list) also queries `ProxyHost` only — a certificate detail page will not list a `RedirectionHost` that's using it.

  Neither of these bypasses the actual delete-time protection (that check correctly covers both tables), so there is no data-loss or dangling-TLS-config risk — but the "certificate lifecycle fixes are complete" claim in the task brief is not quite accurate as stated. Recommend a small follow-up (not necessarily blocking this PR) extending both to also union in `RedirectionHost`, mirroring the `IsCertificateInUse` pattern already established.

**(e) `RedirectionHost` authorization scoping — confirmed by-design, no regression, no new surface.** Read `routes.go` directly: `redirectionHostHandler.RegisterRoutes(management)` (line 1046) registers on the exact same `management` router group as `proxyHostHandler.RegisterRoutes(management)` (line 1037) — the `RequireManagementAccess` tier, not the stricter `RequireRole(admin)` tier used for e.g. certificates/DNS providers/access lists elsewhere in the same file. This matches `SECURITY.md`'s documented model ("`user` covers day-to-day proxy-host management") and is consistent, deliberate parity with `ProxyHost` — not a new authorization surface, not a privilege-escalation path, and not a regression.

## 6. New Finding: `RedirectionHost.Enabled` GORM Zero-Value Bug (Medium)

**Confirmed by reproduction, not just code-reading.** `RedirectionHost.Enabled` (model field, `gorm:"default:true;index"`, plain non-pointer `bool`) was not included in the `booleanFieldsDefaultingTrueOnCreate` fix applied in commit `2dcdb9fc` for `PreservePath`/`SSLForced`/`HTTP2Support`. Those three fields had their `gorm:"default:true"` tags *removed* specifically because a plain `bool` at its Go zero value (`false`) is indistinguishable to GORM from "unset," so GORM omits the column from the `INSERT` and lets the DB's `default:true` clobber an explicit `false`. `Enabled` still carries `gorm:"default:true"` and is still a plain `bool` — same bug class, same field type, just missed.

I wrote and ran a temporary, throwaway test (not committed, removed after use) directly against `RedirectionHostService.Create`:

```go
host := &models.RedirectionHost{
    DomainNames: "qa-disabled-test.example.com",
    TargetURL:   "https://target.example.com",
    StatusCode:  301,
    Enabled:     false,
}
service.Create(host)
// reloaded from DB:
```
Result: `Enabled` reloads as `true`. Bug confirmed live, not just by inspection.

**Real-world impact is currently limited** — I checked `frontend/src/components/RedirectionHostForm.tsx` and it never sends an `enabled` key on create (there's no "enabled" toggle in the create form at all; disabling happens later via the list page's toggle, which goes through `Update`, and `Update` uses `.Select("*").Updates(host)`, which correctly forces all columns including zero values — so the *Update*/disable-after-creation path is unaffected). So no current UI flow can trigger this. However, `docs/plans/current_spec.md` §4.4 documents `"enabled": true` as part of the create request body contract, implying an API consumer (a script, a future UI feature, an integration) could reasonably send `"enabled": false` on create and have it silently ignored — a correctness bug in the documented contract.

**Recommendation**: add `"enabled"` to `booleanFieldsDefaultingTrueOnCreate` in `redirection_host_handler.go`'s `Create` — but note the fix pattern differs slightly, since that list currently assumes "default missing → true" for all its members; `Enabled`'s desired default really is `true` (matches its intended semantics and NPM parity), so the existing pattern applies directly: add `"enabled"` to the list. This is a small, low-risk, well-precedented fix (the exact same pattern was just established one commit ago in this same PR).

## 7. Recommendation

**CONDITIONAL PASS.** I'd recommend one of two paths, at the orchestrating session's discretion:

1. **Fix-then-merge**: a small follow-up commit adding `"enabled"` to `booleanFieldsDefaultingTrueOnCreate` (§6) plus a regression test, and optionally the patch-coverage gaps in §2 (handler error branches) and the two certificate-lifecycle display gaps in §5(d) — then re-run the patch-coverage preflight to confirm it clears strict mode. This is proportionate given how small and precedented the fix is.
2. **Merge-with-tracked-followups**: if the team wants to ship now, explicitly accept (in writing, e.g. a tracked issue) the `Enabled` bug as a known, low-impact (UI-unreachable) defect and the patch-coverage shortfall as an accepted gap, rather than silently letting them slide.

Either way, nothing found here rises to a CRITICAL/HIGH security blocker — the five specific security properties this feature depends on (SSRF non-issue, self-redirect guard, cross-table uniqueness, cert-delete protection, authorization scoping) are all sound.
