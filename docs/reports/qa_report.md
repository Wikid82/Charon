# QA Report: What's New Pipeline Repair and Build-Metadata Versions

- Branch: `fix/whats-new-pipeline-and-build-versions` (17 commits, `development..HEAD`, HEAD `9f513f606`)
- Spec: `docs/plans/current_spec.md`
- Date: 2026-11-07
- Docker: rootless daemon only (`DOCKER_HOST=unix:///run/user/1001/docker.sock`). No live container was
  stopped, restarted, exec'd into or reconfigured. All QA objects were named `qa-wn-*` / `smoke-wn-*`,
  bound to `127.0.0.1`, and removed afterwards (verified with `docker ps -a`, `docker volume ls`,
  `docker images`). The working tree ends clean (`git status --short` empty) apart from this report and
  `test-results/` (git-ignored).

## Verdict

**PASS, no blocking defects in the branch.** Every Definition of Done gate that can run locally passed, and
the built-image matrix rows that can run before merge (M1, M5b, M7, M8, M9, M10, plus dev and release
variants of M1) passed on a locally built image with real changelog data. One tooling hazard was found in
the E2E rebuild skill (not in this branch's code); see Blocking/Process.

## Results per step

| # | Step | Result | Evidence |
| --- | --- | --- | --- |
| 1a | E2E container rebuild via `docker-rebuild-e2e` skill | **NOT RUN (collision)** | The skill tags `charon:local`, the image the live `charon` container runs (`018975d1cf1f`). See P1 |
| 1b | `whats-new-changelog.spec.ts --project=firefox` (equivalent non-colliding container, `VERSION=dev`, fixture) | PASS, 8/8, not skipped | `/api/v1/health` `version: "dev"`; all 8 tests passed incl. the hand-edited aria snapshot (no snapshot diff, no `--update-snapshots`) |
| 1c | `system-security-settings.spec.ts` | PASS, 20/20 (`--project=security-tests`) | `--project=firefox` finds **no tests**: the firefox project ignores `**/security-enforcement/**`. The spec's gate command is wrong (O1). Ran the only project that owns the file (Chromium, single project) |
| 2 | `./scripts/scan-gorm-security.sh --check` | PASS | CRITICAL 0, HIGH 0, MEDIUM 0, INFO 2 (pre-existing index suggestions), exit 0 |
| 3 | `bash scripts/local-patch-report.sh` | PASS | Re-run with fresh coverage inputs: Overall 169/169 (100%), Backend 130/130, Frontend 39/39. Artifacts `test-results/local-patch-report.{md,json}` present. The first run read stale Nov-5 coverage files (O2) |
| 4a | `scripts/go-test-coverage.sh` | PASS | Statements 93.2%, line 92.1% (gate 87%); 0 FAIL; `internal/version` 100%, `internal/changelog` 100%, `handlers` 92.3%, `services` 90.8% |
| 4b | `scripts/frontend-test-coverage.sh` | PASS | 328 files, 4314 passed, 2 todo, 0 failed; lines 92.47%, statements 91.58%, branches 86.27% |
| 5a | `go build ./...` | PASS | exit 0 |
| 5b | `npm run type-check`, `npm run build` | PASS | both exit 0 |
| 5c | `make lint-fast` | PASS | backend 0 issues, agent 0 issues |
| 5d | lefthook pre-commit | PASS, 20/20 hooks | Run over all 126 branch-changed files (`--force --file …`), not `--all-files`: the `end-of-file-fixer`/`trailing-whitespace` hooks rewrite every file they are given, so `--all-files` would have modified unrelated files. All hooks green (actionlint, shellcheck, hadolint, semgrep, markdownlint, eslint, locales lint, gocritic parity, …) |
| 5e | bats (5 suites) | PASS | 157/157 (`generate-changelog`, `check-changelog-data`, `detect-release-commit`, `smoke-whats-new`, `resolve-build-version`) |
| 5f | docs-site strict build | NOT RUN | `zensical` not installed locally; CI only |
| 6a | CodeQL Go + JS (`lefthook run codeql`, CLI 2.27.2) | PASS | JS 0 results. Go 4 results, all pre-existing and outside the branch: `go/log-injection` (6.1) x3 at `remote_server_handler.go:148`, `uptime_service.go:1551`; `go/cookie-secure-not-set` (4.0) at `auth_handler.go:201`. 0 high/critical; `codeql-check-findings` passed |
| 6b | Trivy (host 0.52.2) on `qa-wn-local:pr` | PASS (no new findings) | 2 HIGH, both third-party, no fix, both in `.trivyignore`/`.grype.yaml`: CVE-2026-116587 (go-jose in `caddy`), CVE-2026-32286 (pgproto3 in `crowdsec`/`cscli`). The published `latest` has the identical set, so nothing is introduced by this branch. 0 CRITICAL |
| 6c | govulncheck | PASS | "Your code is affected by 0 vulnerabilities" |
| 7 | Built-image matrix | PASS (all locally runnable rows) | See below |
| 8 | SECURITY.md / spec security notes / attribution | PASS | See below |

## Step 7: built-image matrix (no E2E fixtures)

Data generation, the way CI does it:

```text
resolve-build-version.sh --channel pr --pr 9999      -> 0.46.5+pr.9999
resolve-build-version.sh --channel dev --sha <HEAD>  -> 0.46.5+dev.9f513f6
resolve-build-version.sh --channel release           -> 0.46.5
generate-changelog.sh --require-entries              -> 171 version entries
check-changelog-data.sh …/changelog.json             -> OK (171 entries, newest 0.46.5); 0 Merge subjects; 136 security items
git checkout -- backend/internal/changelog/data/changelog.json   -> placeholder `[]` restored, tree clean
```

Images built: `qa-wn-local:pr`, `qa-wn-local:dev`, `qa-wn-local:release` (all removed afterwards).

| Row | Image | Result | Evidence |
| --- | --- | --- | --- |
| Baseline | `ghcr.io/wikid82/charon:latest` (published 0.46.5) | FAIL as expected (shows the bug) | smoke: `FAIL: changelog has 0 entries, expected at least 100`; admin status `{"show_changelog":false,"versions":[]}`; `/changelog/all` count 0 |
| M1 | `qa-wn-local:pr` | PASS | smoke `--expect-version-exact 0.46.5+pr.9999 --expect-min-entries 100 --expect-newest 0.46.5 --expect-status-for-unseeded shown`: 171 entries, no duplicates or merge subjects, unseeded user sees 3 entries all ≤ 0.46.5, hidden after `dismiss_permanent` and on 2 further sign-ins. `/system/updates`: `available:false`, `current_version:"0.46.5+pr.9999"` |
| M2 (local stand-in) | `qa-wn-local:dev` | PASS | same assertions with `--expect-version-exact 0.46.5+dev.9f513f6` |
| M4/M6 (local stand-in) | `qa-wn-local:release` | PASS | same assertions with `--expect-version-exact 0.46.5` |
| M5b | latest → PR image, same volume | PASS | A (`0.46.5`): hidden. B (`0.36.1`): `["0.46.5","0.46.4","0.46.3"]`, exactly 3, not ~35. C (`''`): same 3. After "Got it", B and C hidden on 2 further logins; stored values `0.46.5` (canonical, no `+pr` metadata) |
| M7 | PR image, stored `0.46.6` | PASS | status hidden; ack returns `{"message":"No change"}`; stored stays `0.46.6` |
| M8 | PR image, passthrough user | PASS | `GET /changelog/status` 403, `GET /changelog/all` 403, `POST /changelog/ack` 403, `POST /changelog/opt-in` 403 |
| F16 | PR image | PASS | invalid action and malformed JSON both return `400 {"error":"invalid request"}` (no binder internals) |
| M9 | PR image, Firefox (Playwright script; the Playwright MCP browser could not start: "Chromium distribution 'chrome' is not found") | PASS | Status modal for an unseeded admin: 3 releases (0.46.5, 0.46.4, 0.46.3). Accessible headings are `"Fixes"` / `"Security"` (icon `aria-hidden`, 6 hidden icons). Revisit dialog: 10 versions, "Show older releases" → 20 → 30, close + reopen resets to 10. No console errors or React key warnings |
| M9 empty state | — | NOT RUN on an image | every local image carried data; covered by `WhatsNewModal.test.tsx` |
| M10 | PR image, System Settings | PASS | Card shows `Current Version 0.46.5+pr.9999`, `Latest Version v0.46.5`, "Up to Date"; "Check for Updates" issues 1 `/system/updates` request; no link rendered while up to date (link renders only when `available`, and the backend `releaseURL` keeps only `https://github.com` without userinfo). No console errors |

## Step 8: security review

- **Sha validation:** backend `commitSHAPattern = ^[0-9a-f]{7,40}$` (`backend/internal/changelog/changelog.go:30`),
  frontend `COMMIT_SHA_PATTERN` (`frontend/src/components/dialogs/WhatsNewModal.tsx:22`).
- **Release URL:** `releaseURL` (`backend/internal/services/update_service.go:172-181`) requires `https`,
  host `github.com`, no userinfo.
- **Generic bind error:** `backend/internal/api/handlers/changelog_handler.go:94-97`; verified live.
- **Workflow `env:` handling:** a YAML-level scan of the 5 changed workflows against `development` shows
  **no new `${{ }}` inside `run:`**. One pre-existing occurrence was removed (nightly
  "Run container smoke test"). All new trigger values reach shell code through `env:`. `actionlint` clean.
- **Permissions:** the new `smoke-whats-new` job declares `contents: read, packages: read`. The two new
  `quality-checks.yml` jobs inherit the workflow default (`contents: read, checks: write`), like every other
  job in that file (O4).
- **Attribution:** no session IDs or `claude.ai/code` links in commit messages or the diff; no `(security)`
  subjects (correct per the spec: no security-sensitive findings).
- **Gotify tokens:** none in changed files or artifacts.
- `IsUnversionedBuild` grep gate: only self-references in the spec.

## Blocking issues

None in the branch's code, tests or workflows.

### P1 (process hazard, outside the branch): `docker-rebuild-e2e` overwrites the live image tag

- Where: `.github/skills/docker-rebuild-e2e-scripts/run.sh:25` (`IMAGE_NAME="charon:local"`) and
  `.docker/compose/docker-compose.playwright-local.yml:18` (`image: charon:local`).
- Evidence: `docker inspect charon` → `018975d1cf1f charon:local`. The live proxy runs from `charon:local`.
- Impact: running the skill retags `charon:local` to a `VERSION=dev` image with the E2E changelog fixture.
  The running container keeps its old image ID, but the next recreate of the live `charon` (for example the
  post-reboot recreate) would deploy the E2E build. The compose file also publishes on all interfaces
  (`8080`, `8180`, `2020`) and mounts the stale root `/var/run/docker.sock`.
- Per instructions I did not run the skill. Instead I ran an equivalent non-colliding container
  (`qa-wn-e2e:dev`, same env as the compose file, ports on `127.0.0.1`, fixture injected and reverted).
- Suggested fix: give the E2E image its own tag (for example `charon:e2e`) in both files. Severity: medium
  (ops), so file an issue.

## Non-blocking observations

- **O1 (docs/spec):** spec §7 row 7 and §9 give `system-security-settings.spec.ts --project=firefox` as the
  gate, but `playwright.config.js` excludes `**/security-enforcement/**` from firefox and the command finds
  0 tests. The real owner is `--project=security-tests` (Chromium).
- **O2 (tooling):** `scripts/local-patch-report.sh` reads whatever `backend/coverage.txt` /
  `frontend/coverage/lcov.info` exist and does not warn when they are older than HEAD. The first run here
  reported 100% from Nov-5 files (91 changed lines). A fresh run gives 169 lines, also 100%.
- **O3 (generator, low):** `generate-changelog.sh` dedupes on the raw subject, but the shown text has the
  type/scope stripped. v0.32.7 and v0.24.0 each contain a duplicate fix
  (`fix:` vs `fix(tools):` "update tools list in agent markdown files for consistency").
  `check-changelog-data.sh` does not check duplicates. The backend `normalizeEntry` dedupe hides it at the
  API (smoke: "no duplicates"), so users are unaffected. Suggest deduping on the cleaned display text.
- **O4 (low):** the new `changelog-data-tests` and `build-version-resolver-tests` jobs
  (`quality-checks.yml:248`, `:274`) inherit `checks: write`, which they do not need; job-level
  `permissions: contents: read` would match the spec's "minimal permissions" rule. Pre-existing file-wide
  pattern.
- **O5 (docs):** `docs/issues/whats-new-changelog-manual-test-plan.md` step 7 (downgrade) uses
  `ghcr.io/wikid82/charon:latest` as the older image. That image predates the never-lowering Ack, so the
  check proves nothing: on the doc's flow B is already `0.46.5`, and with B at `0.46.6` the old image
  **lowered** it to `0.46.5` (observed). The meaningful check is the one in M7 above (store a higher value,
  ack on the image under test). Suggest rewording step 7.
- **O6 (pre-existing, out of scope):** CodeQL medium `go/log-injection` x3 and `go/cookie-secure-not-set`
  (files untouched by this branch). `.trivyignore` review dates for the pgproto3 entries (`exp: 2026-09-01`)
  have passed. Both should be confirmed as tracked.
- **O7 (environment):** the Playwright MCP browser cannot start on this host (no Chrome at
  `/opt/google/chrome/chrome`); the M9/M10 checks used the project's Playwright Firefox from a script.

## Verifiable only in the PR's own CI run (or after merge/release)

- The `setup` job guard on CI data; the artifact downloaded in both build jobs; `smoke-whats-new` passing
  **before** `merge-and-publish`, with the log line `Resolved build VERSION: 0.46.5+pr.<N>`.
- Version tags disabled in the metadata step on the PR.
- The docs-site strict build (`zensical`).
- Full-suite and cross-browser E2E.
- M2 and M3 (`:dev` and `:nightly` after merge, including `git_commit` = `built_sha` for #1867), M4 and M5
  (first release: `X.Y.Z`/`X.Y` tags, synthesized head entry), M6 (non-release `main`).
- Trivy/Grype with the CI scanner versions and ignore files.
