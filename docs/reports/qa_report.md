# QA & Security Report — Proxy Host Group Selector + Grouped-View Row Layout Fix (#1367)

- **Feature branch:** `development`
- **Commits audited:** `7a97f589e`, `fd6f83d503`, `8697ac1098`, `45316b636b`, `266c0743`, `ce730d4d`, `01466e421`, `11a65fd980`, `1848de6a1` (9 commits)
- **Plan:** `docs/plans/current_spec.md`
- **Prior gate:** Supervisor pass — approved (diff fidelity, bug-fix correctness, test coverage, E2E status, acceptance criteria, preliminary security read)
- **Date:** 2026-09-21
- **Verdict:** **PASS — SAFE TO MERGE.** No blocking issues found. Zero CRITICAL/HIGH security findings. Coverage gates met. All unit, integration, and E2E tests pass with zero failures.

---

## 0. Scope confirmation

Confirmed via `git diff HEAD~9 HEAD --stat`: 17 files changed (+1072/-10) — a new frontend component (`ProxyGroupSelector.tsx`), a `ProxyHostForm.tsx` field addition, a `DataTable.tsx` refactor (`minWidth` support), a `ProxyHosts.tsx` layout fix (flat vs. grouped column sets), one backend handler bug fix (`proxy_host_handler.go`, `Create` silently dropping `proxy_group_id`), corresponding unit tests, E2E specs, and a one-line `docs/features.md` addition. No new dependencies (`go.mod`/`go.sum`/`package.json`/`package-lock.json` diffs are empty). No new API endpoints — reuses the existing `/proxy-groups` and `/proxy-hosts` routes.

---

## 1. Definition of Done — gate-by-gate

| # | Gate | Result | Notes |
|---|------|--------|-------|
| 1 | Targeted Playwright E2E (`tests/proxy-groups.spec.ts --project=firefox`) | **PASS** | 14/14 tests passed (30.1s), including all 4 new "Group Selector" tests and both new "Grouped Row Layout" tests. |
| 1.5 | GORM security scan (`scan-gorm-security.sh --check`) | **PASS** | Triggered by `ce730d4d` touching `proxy_host_handler.go`. 0 CRITICAL/HIGH. 2 pre-existing INFO suggestions in unrelated `user.go` (missing index hints), 1 suppressed. |
| 2 | Local patch coverage preflight | **PASS** | See §2 below — 100% patch coverage overall/backend/frontend after regenerating fresh coverage inputs. |
| 3 | Security scans (CodeQL Go/JS, Trivy, Semgrep) — run locally (new feature surface) | **PASS** | See §3 below. |
| 4 | `lefthook run pre-commit` | **PASS (see note)** | Staged-file gate reported all hooks "skip" (working tree is clean/committed, nothing staged). Ran every underlying scan script directly instead (CodeQL Go/JS, GORM, Semgrep) — more thorough than the staged-file gate, all clean. |
| 5 | `make lint-staticcheck-only` | **PASS** | 0 issues, backend + agent. |
| 6 | Coverage (`go-test-coverage.sh`, `frontend-test-coverage.sh`) | **PASS** | Backend: 88.6% line / 92.0% statement (gate 87%). Frontend: 90.99% lines / 89.78% statements (gate 87%). |
| 7 | `npm run type-check` | **PASS** | Clean. |
| 8 | Build verification | **PASS** | `go build ./...` clean; `npm run build` clean (3319 modules, no errors). |
| 9 | Full unit test suites, zero failures | **PASS** | Backend: `go test -race ./...` all pass. Frontend: **3416 passed, 4 skipped, 2 todo, 0 failed** (272/272 test files) — full suite, not just touched files. See §4 for the investigation into two transient failures observed mid-audit. |
| 10 | Clean-up check | **PASS** | No debug prints, no dead code, no stray `console.log`/`fmt.Println` introduced in any of the 5 touched application files. |

---

## 2. Local patch coverage (regenerated)

An initial `local-patch-report.sh` run early in this audit used **stale** `backend/coverage.txt`/`frontend/coverage/lcov.info` (generated hours before this session) and reported `WARN`: overall 85.7%, backend 81.8%, flagging `proxy_host_handler.go:507-508` (the `Create` fix's `host.ProxyGroupID = resolvedGroupID` line and its preceding comment) as uncovered.

After regenerating both coverage files fresh via `scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh` (which exercise the new `TestProxyHostCreate_WithProxyGroupReference_ValidUUID_201` regression test that reads back exactly those lines from the DB) and re-running `scripts/local-patch-report.sh`:

| Scope | Changed Lines | Covered Lines | Patch Coverage | Status |
|---|---:|---:|---:|---|
| Overall | 23 | 23 | **100.0%** | pass |
| Backend | 5 | 5 | **100.0%** | pass |
| Frontend | 18 | 18 | **100.0%** | pass |
| Agent | 0 | 0 | 100.0% | pass |

**Conclusion: the earlier WARN was a stale-instrumentation artifact, not a real coverage gap.** The `proxy_host_handler.go:507-508` lines are in fact covered by the new regression test; the stale `coverage.txt` simply predated that test's inclusion in the instrumented run. Artifacts: `test-results/local-patch-report.md`, `test-results/local-patch-report.json`.

---

## 3. Security scans

### 3.1 CodeQL — Go
`scripts/pre-commit-hooks/codeql-go-scan.sh` (CI-aligned `go-security-and-quality` suite, 263/263 compiled files, extraction parity confirmed against `go list` baseline): **4 results**, all pre-existing and already suppressed via `codeql-suppressions.yml`:
- `go/cookie-secure-not-set` — `auth_handler.go:198` (documented, unrelated to this feature)
- `go/log-injection` ×2 — `remote_server_handler.go:148` (documented false positive, unrelated)
- `go/log-injection` — `uptime_service.go:1551` (documented false positive, unrelated)

**None touch this PR's diff.** `codeql-check-findings.sh` → 0 blocking.

### 3.2 CodeQL — JavaScript/TypeScript
`scripts/pre-commit-hooks/codeql-js-scan.sh` (560/560 files): **0 findings.**

### 3.3 Semgrep
`scripts/pre-commit-hooks/semgrep-scan.sh` (`p/golang`, `p/javascript`, `p/typescript`, `p/react`, `p/secrets`, `p/dockerfile` — 424 rules, 1001 tracked files, ~99.9% parsed):

```
Findings: 0 (0 blocking)
Rules run: 424
Targets scanned: 1001
```

(Two earlier attempts run concurrently with the heavy backend `-race` test suite and CodeQL databases died silently under memory pressure on this shared dev host — re-run alone with a clean process table completed successfully with a definitive result above.)

### 3.4 Trivy (filesystem scan)
`security-scan-trivy` (vuln, secret, misconfig scanners across the whole working tree) surfaced findings, but every one was verified independently to be **pre-existing and out of scope for this PR**:
- `.claude/worktrees/*` — other agents' local worktrees (gitignored: `.claude/worktrees/`)
- `docs-site/build/*` — gitignored build output (gitignored: `docs-site/.gitignore` → `/build`), containing a documentation example JSON showing GCP service-account key *structure*, not a real credential
- `backend/internal/api/routes/keys/hecate-ca.key` — a local, gitignored (`*.key`) test CA artifact, not tracked by git
- `agent/Dockerfile` USER-command finding — pre-existing, untouched by this PR

Confirmed via `git check-ignore -v` and `git diff HEAD~9 HEAD` that none of these paths are part of the 9 audited commits. No new dependencies were introduced by this PR (`go.mod`/`go.sum`/`package.json`/`package-lock.json` diffs are empty), so there is no new supply-chain surface to scan.

---

## 4. Investigation: transient frontend test failures during audit

Mid-audit, a full-suite `frontend-test-coverage.sh` run (executed concurrently with the backend `-race` suite, CodeQL Go/JS scans, and a Semgrep scan all competing for the same host's resources) showed 2 failures in `ProxyHostForm.test.tsx` ("submits form with all basic fields", "submits form with certificate selection"). This was investigated rather than dismissed:

1. Re-ran `ProxyHostForm.test.tsx` in isolation (both via `-t` filter and as a full file) — **62/62 passed both times.**
2. Re-ran the full `frontend-test-coverage.sh` a second time; it stalled and then crashed with `ENOENT ... coverage/.tmp/coverage-*.json` — traced to the standalone isolation reruns in step 1 sharing the same `frontend/coverage/.tmp` directory as the concurrently-running full-suite script, corrupting its v8 coverage temp files.
3. Cleaned `frontend/coverage/`, ran `frontend-test-coverage.sh` a third time with **nothing else touching the `frontend/` directory concurrently**: **3416/3416 tests passed, 0 failures**, full clean coverage report.

**Conclusion: the observed failures were caused by this audit's own concurrent tooling (shared coverage temp-file collisions), not a regression introduced by the feature.** The `ProxyHostForm.test.tsx` diff in this PR only adds a `useProxyGroups` mock — it does not touch form-submission logic. No code changes were made as a result of this investigation; it is documented here for traceability.

---

## 5. Security-specific review (independent of Supervisor's pass)

### 5.1 Proxy group authorization scoping
`ProxyGroup` (backend/internal/models/proxy_group.go) has no owner/tenant field, and `ProxyGroupHandler.RegisterRoutes` is mounted on the non-admin `management` route group (`backend/internal/api/routes/routes.go:1025-1026`), not `managementAdmin`. This means any authenticated `user`-role account can create, update, delete, and assign proxy groups — confirmed **intentional and consistent** with `SECURITY.md`'s documented RBAC model ("`user` covers day-to-day proxy-host management"). Proxy groups are a shared/global organizational feature like the rest of proxy-host management, not a per-user resource. **No authorization regression; no new privilege-escalation surface.**

### 5.2 `ProxyGroup.Color` — CSS injection / XSS review
Traced every consumer of `group.color` across the codebase:

| File | Usage |
|---|---|
| `ProxyGroupBadge.tsx:16` | `style={{ backgroundColor: group.color }}` |
| `ProxyGroupSelector.tsx:46` (new, this PR) | `style={{ backgroundColor: group.color ?? DEFAULT_DOT_COLOR }}` |
| `ManageGroupsDialog.tsx:57` | `style={{ backgroundColor: group.color }}` |
| `ProxyHosts.tsx:812` | `style={{ backgroundColor: group.color ?? '#6b7280' }}` |

All four set `backgroundColor` as a React **style object property**, never via `dangerouslySetInnerHTML` or string-concatenated CSS/`<style>` text. React applies this through the DOM CSSOM setter (`element.style.backgroundColor = value`), which the browser itself validates and silently no-ops on any invalid value. This consumption pattern **cannot** be used to break out into arbitrary CSS (no injection point exists — there's no surrounding string to escape from) or to execute script (no HTML/JS sink is ever reached). This holds regardless of what string is stored server-side.

**Gap identified (non-blocking, pre-existing, not introduced by this PR):** `backend/internal/api/handlers/proxy_group_handler.go` `Create`/`Update` and `backend/internal/services/proxy_group_service.go` apply **zero validation** to `Color` — no hex-format regex, no length cap. Given the safe consumption pattern confirmed above, this is a **data-quality gap only**, not an exploitable XSS/CSS-injection vector (LOW severity, not blocking). Recommend a follow-up hardening ticket to add a hex-color format validator (e.g. `^#[0-9a-fA-F]{6}$`) as defense-in-depth. This gap exists on the pre-existing `ProxyGroup` CRUD handler, entirely untouched by this PR's 9 commits — it is out of scope for this feature's merge decision.

### 5.3 Create-handler fix validation ordering (`ce730d4d`)
Verified directly in the diff: `resolveProxyGroupReference` (which validates the incoming value is either `nil`/empty or a UUID matching an existing group, returning `400` otherwise) executes **before** the `json.Marshal`/`Unmarshal` round-trip that builds the `ProxyHost` struct from the request payload. The fix assigns `host.ProxyGroupID = resolvedGroupID` **after** that round-trip, using the already-resolved-and-validated `*uint` — it does not re-derive or bypass validation. This exactly mirrors `Update`'s existing (correct) pattern. The new regression test (`TestProxyHostCreate_WithProxyGroupReference_ValidUUID_201`) confirms persistence via a DB readback rather than trusting the response body (`ProxyGroupID` is `json:"-"`), which is the right test design given the bug's nature.

### 5.4 Gotify token hygiene
N/A — this feature touches no Gotify/notification code paths.

---

## 6. Acceptance criteria (from `docs/plans/current_spec.md` §6)

- [x] Creating a new proxy host allows selecting a proxy group (or none); the created host's group matches what was selected — verified by E2E ("assigns a group to a host via the create/edit form") and backend regression test.
- [x] Editing an existing host with a group pre-populates that group; changing/clearing it updates/removes the association — verified by E2E ("preselects the host's current group when editing", "clears a host's group via the create/edit form").
- [x] Backend change (Phase 3 contingency) is documented with its own commit (`ce730d4d`), tests, and rationale — present in this spec and in commit history.
- [x] At 1280px+ viewport, grouped view shows Edit/Delete fully visible and unclipped — verified by E2E ("keeps the Actions column fully visible at 1280px width when grouped") using bounding-box containment assertions.
- [x] "Group" column omitted in grouped/ungrouped sections, present in flat view — verified by E2E and by `ProxyHosts-groups.test.tsx` unit tests.
- [x] Horizontal scrollbar fallback at narrow widths — `min-w-[760px]` on `<table>`, verified via `DataTable.test.tsx` class-list assertions.
- [x] Unit test coverage; overall frontend coverage ≥85% — 90.99% lines.
- [x] `tests/proxy-groups.spec.ts` passes under `--project=firefox` — 14/14.
- [x] `npm run type-check` / `npm run build` — clean.
- [x] `lefthook run pre-commit` passes; `--no-verify` not used — confirmed (underlying scan scripts run directly, all clean; no `--no-verify` used anywhere in this audit).
- [x] Full Definition of Done satisfied — see §1.

---

## 7. Overall Verdict

**PASS.** No blocking issues. All Definition-of-Done gates satisfied with real, verified (not stale or assumed) results:

- E2E: 14/14
- Backend unit tests: all pass (`-race`), 88.6% line coverage
- Frontend unit tests: 3416/3416 pass, 0 failures, 90.99% line coverage
- Patch coverage: 100% overall/backend/frontend
- CodeQL Go/JS: 0 blocking findings
- Semgrep: 0 findings across 424 rules / 1001 files
- Trivy: all findings pre-existing and out of scope for this PR
- GORM scan: 0 CRITICAL/HIGH
- staticcheck: 0 issues
- Builds and type-check: clean
- Security review: proxy-group scoping intentional and consistent with documented RBAC; `Color` field confirmed non-exploitable for XSS/CSS-injection given React's safe style-object consumption pattern (one pre-existing, non-blocking hardening recommendation noted in §5.2); Create-handler fix correctly preserves UUID validation ordering.

**Non-blocking follow-up recommended (not required for this merge):** add server-side hex-color format validation to `ProxyGroupHandler.Create`/`Update` as defense-in-depth (§5.2).
