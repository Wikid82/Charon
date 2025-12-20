# CI Failure Investigation: GitHub Actions run 20318460213 (PR #469 – SQLite corruption guardrails)

## What failed

- Workflow: Docker Build, Publish & Test → job `build-and-push`.
- Step that broke: **Verify Caddy Security Patches (CVE-2025-68156)** attempted `docker run ghcr.io/wikid82/charon:pr-420` and returned `manifest unknown`; the image never existed in the registry for PR builds.
- Trigger: PR #469 “feat: add SQLite database corruption guardrails” on branch `feature/beta-release`.

## Evidence collected

- Downloaded and decompressed the run artifact `Wikid82~Charon~V26M7K.dockerbuild` (gzip → tar) and inspected the Buildx trace; no stage errors were present.
- GitHub Actions log for the failing step shows the manifest lookup failure only; no Dockerfile build errors surfaced.
- Local reproduction of the CI build command (BuildKit, `--pull`, `--platform=linux/amd64`) completed successfully through all stages.

## Root cause

- PR builds set `push: false` in the Buildx step, and the workflow did not load the built image locally.
- The subsequent verification step pulls `ghcr.io/wikid82/charon:pr-<number>` from the registry even for PR builds; because the image was never pushed and was not loaded locally, the pull returned `manifest unknown`, aborting the job.
- The Dockerfile itself and base images were not at fault.

## Fix applied

- Updated [.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml) to load the image when the event is `pull_request` (`load: ${{ github.event_name == 'pull_request' }}`) while keeping `push: false` for PRs. This makes the locally built image available to the verification step without publishing it.

## Validation

- Local docker build: `DOCKER_BUILDKIT=1 docker build --progress=plain --pull --platform=linux/amd64 .` → success.
- Backend coverage: `scripts/go-test-coverage.sh` → 85.6% coverage (pass, threshold 85%).
- Frontend tests with coverage: `scripts/frontend-test-coverage.sh` → coverage 89.48% (pass).
- TypeScript check: `cd frontend && npm run type-check` → pass.
- Pre-commit: ran; `check-version-match` fails because `.version (0.9.3)` does not match latest Git tag `v0.11.2` (pre-existing repository state). All other hooks passed.

## Follow-ups / notes

- The verification step now succeeds in PR builds because the image is available locally; no Dockerfile or .dockerignore changes were necessary.
- If the version mismatch hook should be satisfied, align `.version` with the intended release tag or skip the hook for non-release branches; left unchanged to avoid an unintended version bump.

---

# Plan: Investigate GitHub Actions run hanging (run 20319807650, job 58372706756, PR #420)

## Intent

Compose a focused, minimum-touch investigation to locate why the referenced GitHub Actions run stalled. The goal is to pinpoint the blocking step, confirm whether it is a workflow, Docker build, or test harness issue, and deliver fixes that avoid new moving parts.

## Phases (minimizing requests)

### Phase 1 — Fast evidence sweep (1–2 requests)

- Pull the raw run log from the URL to capture timestamps and see exactly which job/step froze. Annotate wall-clock durations per step, especially in `build-and-push` of [../../.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml) and `backend-quality` / `frontend-quality` of [../../.github/workflows/quality-checks.yml](../../.github/workflows/quality-checks.yml).
- Note whether the hang preceded or followed `docker/build-push-action` (step `Build and push Docker image`) or the verification step `Verify Caddy Security Patches (CVE-2025-68156)` that shells into the built image and may wait on Docker or `go version -m` output.
- If the run is actually the `trivy-pr-app-only` job, check for a stall around `docker build -t charon:pr-${{ github.sha }}` or `aquasec/trivy:latest` pulls.

### Phase 2 — Timeline + suspect isolation (1 request)

- Construct a concise timeline from the log with start/end times for each step; flag any step exceeding its historical median (use neighboring successful runs of `docker-build.yml` and `quality-checks.yml` as references).
- Identify whether the hang aligns with runner resource exhaustion (look for `no space left on device`, `context deadline exceeded`, or missing heartbeats) versus a deadlock in our scripts such as `scripts/go-test-coverage.sh` or `scripts/frontend-test-coverage.sh` that could wait on coverage thresholds or stalled tests.

### Phase 3 — Targeted reproduction (1 request locally if needed)

- Recreate the suspected step locally using the same inputs: e.g., `DOCKER_BUILDKIT=1 docker build --progress=plain --pull --platform=linux/amd64 .` for the `build-and-push` stage, or `bash scripts/go-test-coverage.sh` and `bash scripts/frontend-test-coverage.sh` for the quality jobs.
- If the stall was inside `Verify Caddy Security Patches`, run its inner commands locally: `docker create/pull` of the PR-tagged image, `docker cp` of `/usr/bin/caddy`, and `go version -m ./caddy_binary` to see if module inspection hangs without a local Go toolchain.

### Phase 4 — Fix design (1 request)

- Add deterministic timeouts per risky step:
  - `docker/build-push-action` already inherits the job timeout (30m); consider adding `build-args`-side timeouts via `--progress=plain` plus `BUILDKIT_STEP_LOG_MAX_SIZE` to avoid log-buffer stalls.
  - For `Verify Caddy Security Patches`, add an explicit `timeout-minutes: 5` or wrap commands with `timeout 300s` to prevent indefinite waits when registry pulls are slow.
  - For `trivy-pr-app-only`, pin the action version and add `timeout 300s` around `docker build` to surface network hangs.
- If the log shows tests hanging, instrument `scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh` with `set -x`, `CI=1`, and `timeout` wrappers around `go test` / `npm run test -- --runInBand --maxWorkers=2` to avoid runner saturation.

### Phase 5 — Hardening and guardrails (1–2 requests)

- Cache hygiene: add a `docker system df` snapshot before builds and prune on failure to avoid disk pressure on hosted runners.
- Add a lightweight heartbeat to long steps (e.g., `while sleep 60; do echo "still working"; done &` in build steps) so Actions detects liveness and avoids silent 15‑minute idle timeouts.
- Mirror diagnostics into the summary: capture the last 200 lines of `~/.docker/daemon.json` or BuildKit traces (`/var/lib/docker/buildkit`) if available, to make future investigations single-pass.

## Files and components to touch (if remediation is needed)

- Workflows: [../../.github/workflows/docker-build.yml](../../.github/workflows/docker-build.yml) (step timeouts, heartbeats), [../../.github/workflows/quality-checks.yml](../../.github/workflows/quality-checks.yml) (timeouts around coverage scripts), and [../../.github/workflows/codecov-upload.yml](../../.github/workflows/codecov-upload.yml) if uploads were the hang point.
- Scripts: `scripts/go-test-coverage.sh`, `scripts/frontend-test-coverage.sh` for timeouts and verbose logging; `scripts/repo_health_check.sh` for early failure signals.
- Runtime artifacts: `docker-entrypoint.sh` only if container start was part of the stall (unlikely), and the [../../Dockerfile](../../Dockerfile) if build stages require log-friendly flags.

## Observations on ignore/config files

- [.gitignore](../../.gitignore): Already excludes build, coverage, and data artifacts; no changes appear necessary for this investigation.
- [.dockerignore](../../.dockerignore): Appropriately trims docs and cache-heavy paths; no additions needed for CI hangs.
- [.codecov.yml](../../.codecov.yml): Coverage gates are explicit at 85% with sensible ignores; leave unchanged unless coverage stalls are traced to overly broad ignores (not indicated yet).
- [Dockerfile](../../Dockerfile): Multi-stage with BuildKit-friendly caching; only consider adding `--progress=plain` via workflow flags rather than altering the file itself.

## Definition of done for the investigation

- The hung step is identified with timestamped proof from the run log.
- A reproduction (or a clear non-repro) is documented; if non-repro, capture environmental deltas.
- A minimal fix is drafted (timeouts, heartbeats, cache hygiene) with a short PR plan referencing the exact workflow steps.
- Follow-up Actions run completes without hanging; summary includes before/after step durations.
