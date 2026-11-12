# QA Report: Workflow Permissions and Build Inputs

- Branch: `chore/ci-workflow-permissions` (11 commits on `origin/development`, up to `7da34cd2d`)
- Spec: `docs/plans/cerberus_hardening_batch7_spec.md`, section 3, plus two follow-ups (per-run key in
  `quality-checks.yml`; base-branch Trivy ignore list in `docker-build.yml` and `toolchain-image.yml`)
- Scope: CI workflows, CI and skill shell scripts, Playwright compose files, E2E test fixtures, editor tasks, CI docs.
  No application code changed (`git diff --stat origin/development...HEAD` touches no `backend/`, `frontend/` or
  `agent/` source), so the backend and frontend coverage suites and the patch coverage preflight were not run.
- Image under test: `charon:local` from `docker-rebuild-e2e` on the committed branch.
- Date: 2026-11-12

## Verdict

**Findings resolved in follow-up commits.** QA first blocked the branch on two defects in the per-run test value
flow of `e2e-tests-split.yml` (H1, H2) and found four low items (L1 to L4). All six were fixed in the five commits
after `7da34cd2d`; the report below describes the state that was reviewed. The remaining check is the PR's own CI
run, which is the only way to exercise the E2E jobs, the result-publishing job and the base-branch policy fetch.

## Gates run (all foreground)

| Gate | Result |
| --- | --- |
| `actionlint` (all workflows) | Pass |
| `shellcheck` on changed scripts | Pass (info-level SC1091 only, same as on `development`) |
| `bats scripts/tests/dockerfile-arg.bats` | Pass, 10/10 |
| `scripts/ci/dockerfile-arg.sh --require-digest Dockerfile ALPINE_IMAGE` | Prints the tag@digest reference |
| `markdownlint` on changed docs | Pass |
| `jq . .vscode/tasks.json` | Pass |
| `docker compose config`, CI file, all four values set | Pass; port 2020 bound to `127.0.0.1` |
| `docker compose config`, CI file, values unset or empty | Fails with the `:?` message (intended) |
| `docker compose config`, CI file, only token and key set (what the workflow sets) | **Fails**: username and password missing (see H1) |
| `docker compose config`, local file, set and unset | Pass / fails as intended |
| `lefthook run pre-commit --all-files` | Pass (all 20 hooks) |
| `docker-rebuild-e2e` skill | Pass; container healthy, 2020 on loopback |
| `npx playwright test tests/security-enforcement/emergency-reset.spec.ts --project=security-tests` | 5 passed |
| `npx playwright test tests/security-enforcement/multi-component-security-workflows.spec.ts --project=security-tests` | 5 passed |
| Same emergency spec with the emergency password unset | Fails in setup with the clear "is not set" message (intended) |
| `npx tsc --noEmit -p tests/tsconfig.json` | Pass |
| `npm run lint:e2e` | 0 errors, 123 warnings (cap 124); no new warnings on changed lines |
| Installer checksums in `.vscode/tasks.json` | Both match an independent download (see below) |

## Findings

### HIGH

**H1. CI E2E jobs do not provide the emergency username and password.**
`.docker/compose/docker-compose.playwright-ci.yml` now requires `CHARON_EMERGENCY_USERNAME` and
`CHARON_EMERGENCY_PASSWORD` (`${VAR:?}`), and the test helpers (`requireEmergencyEnv`) throw when they are unset.
The six E2E jobs in `.github/workflows/e2e-tests-split.yml` set only `CHARON_EMERGENCY_TOKEN` and
`CHARON_ENCRYPTION_KEY`. `docker compose ... up -d` will fail in every job, and Playwright setup would fail after
it. Reproduced locally with `docker compose config` using only the two values the workflow sets.
Remediation: generate a per-run username and password alongside the token and key, and set them in the same place
as the other two values in all six jobs (see H2 for where that place should be).

**H2. Per-run values passed as job outputs will arrive empty.**
The `build` job masks the generated token and key (`::add-mask::`) and exposes them as job outputs; the E2E jobs
read `needs.build.outputs.*`. The runner drops any job output that contains a masked value ("Skip output ... since
it may contain secret"), so the E2E jobs receive empty strings. The token check step then fails with "secret not
configured", and compose fails on the empty key. (GitHub documents this behaviour in "Passing information between
jobs".) Remediation: generate the values inside each E2E job instead of in `build` (each job starts its own
container, so the values do not need to be shared). Because job-level `env` is evaluated before steps run, either
set them per step from a step output, or keep one generation step that masks them and writes them for the job. The
spec's no-`GITHUB_ENV` rule can be kept by giving the compose, Playwright and log steps the values through
step-level `env` from `steps.<id>.outputs`. Do not drop the masking to make the outputs pass: the job-level `env`
block is printed in step logs before any later mask takes effect.

Only the PR's CI run can confirm the fix for H1 and H2; there is no local equivalent of the cross-job output filter.

### MEDIUM

None.

### LOW

**L1. Backend coverage job lost its Go cache key.** `.github/workflows/codecov-upload.yml`, `backend-codecov`
job, "Set up Go": the `cache-dependency-path: backend/go.sum` line was removed together with the old key step.
The job still works but restores no module cache (setup-go looks for a root `go.sum`). Remediation: restore the line.

**L2. Editor scanner tasks report success when the checksum fails.** `.vscode/tasks.json`, Grype and Syft update
tasks: the command ends with `; rm -f "$tmp"`, so the task's exit status is that of `rm`, not of the checksum or
install. A mismatch prints `FAILED` and installs nothing (safe), but the task shows as successful. Remediation:
`... ; rc=$?; rm -f "$tmp"; exit $rc`, or a `trap`. Also, the spec asked for `sudo` only on the final copy; the
whole installer runs under `sudo`. Acceptable because the script is checksum-verified first, but an unprivileged
install to a temp dir followed by `sudo install` would match the spec.

**L3. Issue-file path check validates only the last path component.** `.github/workflows/docs-to-issues.yml`,
`rejectIssueFile`: `lstat` rejects a symlinked file, but a symlinked directory inside `docs/issues/` would still be
followed. Inputs come from pushes and manual runs by people with write access, so impact is small. Remediation:
also require `fs.realpathSync(filePath)` to start with `fs.realpathSync('docs/issues') + path.sep`. Manual paths
also skip the `created/`, template and README exclusions that commit-derived paths get; consider applying the same
filter to both.

**L4. Toolchain scan job can now fail on PRs where it was report-only.** `.github/workflows/toolchain-image.yml`,
`trivy-scan`: the base-policy step runs on every PR (including fork PRs where `scan` is false) and has no
`continue-on-error`, while the Trivy gate it feeds is report-only on PRs. A transient fetch failure fails the job.
This is fail-closed and acceptable; optionally gate the step on `steps.ref.outputs.scan == 'true'`.

### Informational

- `benchmark.yml`: the concurrency group still references `github.event.workflow_run.head_branch`, which no longer
  applies (the workflow has no `workflow_run` trigger). Harmless; can be dropped.
- `supply-chain-pr.yml`, "Fail on Critical/High vulnerabilities": still interpolates step outputs directly into
  `run`. The values are counts produced by `jq` in the same job, so this is not exploitable, but moving them to
  `env` would match the rest of the change.
- Once this merges, the repository secrets for the backend test key and the E2E emergency token are no longer read
  by any workflow and can be deleted.

## Security review notes

- **Permissions.** `supply-chain-pr.yml` workflow scope is read-only; write scopes exist only on
  `publish-results`, which never checks out code, downloads the scan artifact only, and validates every count as a
  non-negative integer before building the comment. Comment values and repository/run identifiers now come through
  `env`, not inline expressions. `benchmark.yml` runs PR and dispatch code with `contents: read` and no repository
  secrets; the `store` job holds the write scopes and runs only on `push` to `main`.
- **Registry token expression.** `(head.repo.full_name == '' || head.repo.full_name == github.repository) &&
  secrets.GITHUB_TOKEN || ''` is the exact complement of the existing `builder-src` inline condition: push and
  dispatch (no PR payload) and same-repository PRs get the token; fork PRs get `''`, and the composite action skips
  the registry login when the token is empty. The deviation from the spec wording is sound. Edge case: a PR whose
  head repository was deleted has an empty `full_name` and is treated like a push; its token is read-only.
- **Base-branch scanner policy.** `BASE_REF` is passed through `env` and quoted. `FETCH_HEAD` is the right ref to
  read after `git fetch --depth=1 origin "$BASE_REF"` (a checkout made with a single ref does not always update the
  remote-tracking ref), so this deviation from the spec is sound. Failure is closed: `set -euo pipefail` fails the
  step, and the Trivy steps that consume the output have no `always()`, so they cannot run with the PR's own policy.
  The repository is public, so the anonymous fetch works without persisted credentials.
- **Six E2E jobs.** All three security and three non-security jobs were switched, and all six old key steps
  removed. No caller passes the removed reusable-workflow secret. The coverage is complete; the flow itself is
  broken (H1, H2).
- **Secrets in logs.** The old token preview (first and last characters) is gone. No value is written to
  `GITHUB_ENV`. Test-only values in `quality-checks.yml`, `codecov-upload.yml` and `benchmark.yml` stay in step
  outputs within one job, which works.
- **Installer pins.** Independently downloaded both commit-pinned `install.sh` files: Grype `8646f06a...3556` and
  Syft `2ed88b16...4b3b` match `.vscode/tasks.json`. Both commits are ancestors of the upstream `main` branch, not
  commits that exist only in a fork. The Grype pin matches `supply-chain-pr.yml`. The installers then verify the
  release archive checksum themselves; the tasks install the latest release rather than a pinned version.
- **Compose.** Port 2020 now binds to loopback in both Playwright compose files, and no compose file carries a
  committed default credential.

## What only the PR's own CI run can confirm

- The E2E jobs start and pass after H1 and H2 are fixed.
- `publish-results` posts or updates the PR comment and uploads SARIF without a checkout.
- The base-branch policy steps fetch and feed Trivy on a real PR.
- The benchmark `store` job's push to the results branch runs only after a merge to `main` and cannot be tested on
  the PR; its condition was checked by reading it.
- The `beta` tag condition was checked by reading it.
