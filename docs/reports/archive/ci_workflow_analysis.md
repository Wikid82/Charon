# CI Workflow Analysis - E2E Timeout Investigation

## Scope
Reviewed CI workflow configuration and the provided E2E job logs to identify timeout and shard-related risks, per sections 2, 3, 7, and 9 of the current spec.

## CI Evidence Collection (Spec Sections 2, 3, 7, 9)
The following commands capture the exact evidence sources used for this investigation.

### Run Logs Download (gh)
```bash
gh run download 21865692694 --repo Wikid82/Charon --dir artifacts-21865692694
```

### Job Logs API Call (curl)
```bash
export GITHUB_OWNER=Wikid82
export GITHUB_REPO=Charon
export JOB_ID=<JOB_ID>
curl -H "Accept: application/vnd.github+json" \
  -H "Authorization: token $GITHUB_TOKEN" \
  -L "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/jobs/$JOB_ID/logs" \
  -o job-$JOB_ID-logs.zip
unzip -d job-$JOB_ID-logs job-$JOB_ID-logs.zip
```

### Artifact List API Call (curl)
```bash
export GITHUB_OWNER=Wikid82
export GITHUB_REPO=Charon
export RUN_ID=21865692694
curl -H "Accept: application/vnd.github+json" \
  -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/runs/$RUN_ID/artifacts" | jq '.'
```

### Job JSON Inspection (Cancellation Evidence)
```bash
export GITHUB_OWNER=Wikid82
export GITHUB_REPO=Charon
export JOB_ID=<JOB_ID>
curl -H "Accept: application/vnd.github+json" \
  -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/$GITHUB_OWNER/$GITHUB_REPO/actions/jobs/$JOB_ID" | jq '.'
```

## Current Timeout Configurations (Workflow Search)
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L216) - E2E Chromium Security timeout set to 60.
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L417) - E2E Firefox Security timeout set to 60.
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L626) - E2E WebKit Security timeout set to 60.
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L842) - E2E Chromium Shards timeout set to 60.
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1046) - E2E Firefox Shards timeout set to 60.
- [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1258) - E2E WebKit Shards timeout set to 60.
- [ .github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L52) - Docker build phase timeout set to 20 (job-level).
- [ .github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L352) - Docker build phase timeout set to 2 (step-level).
- [ .github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L637) - Docker build phase timeout set to 10 (job-level).
- [ .github/workflows/docs.yml](.github/workflows/docs.yml#L27) - Docs workflow timeout set to 10.
- [ .github/workflows/docs.yml](.github/workflows/docs.yml#L368) - Docs workflow timeout set to 5.
- [ .github/workflows/codecov-upload.yml](.github/workflows/codecov-upload.yml#L38) - Codecov upload timeout set to 15.
- [ .github/workflows/codecov-upload.yml](.github/workflows/codecov-upload.yml#L72) - Codecov upload timeout set to 15.
- [ .github/workflows/security-pr.yml](.github/workflows/security-pr.yml#L23) - Security PR workflow timeout set to 10.
- [ .github/workflows/supply-chain-pr.yml](.github/workflows/supply-chain-pr.yml#L28) - Supply chain PR timeout set to 15.
- [ .github/workflows/renovate.yml](.github/workflows/renovate.yml#L20) - Renovate timeout set to 30.
- [ .github/workflows/security-weekly-rebuild.yml](.github/workflows/security-weekly-rebuild.yml#L30) - Security weekly rebuild timeout set to 60.
- [ .github/workflows/cerberus-integration.yml](.github/workflows/cerberus-integration.yml#L24) - Cerberus integration timeout set to 20.
- [ .github/workflows/crowdsec-integration.yml](.github/workflows/crowdsec-integration.yml#L24) - CrowdSec integration timeout set to 15.
- [ .github/workflows/waf-integration.yml](.github/workflows/waf-integration.yml#L24) - WAF integration timeout set to 15.
- [ .github/workflows/rate-limit-integration.yml](.github/workflows/rate-limit-integration.yml#L24) - Rate limit integration timeout set to 15.

## E2E Playwright Invocation and Shard Strategy
- Playwright is invoked in the E2E workflow for security and non-security runs. See [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L331), [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L540), [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L749), [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L945), [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1157), and [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1369).
- Shard matrix configuration for non-security runs is set to 4 shards per browser. See [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L851-L852), [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1055-L1056), and [ .github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L1267-L1268).

## Reproduction Command Coverage (Spec Sections 3, 8)
The steps below mirror the CI flow with the same compose file, env variables, and Playwright CLI flags.

### Image Rebuild Steps (CI Parity)
```bash
# CI build job produces a local image and saves it as a tar.
# To match CI locally, rebuild the E2E image using the project skill:
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

### Environment Start Commands (CI Compose)
```bash
# CI uses the Playwright CI compose file.
docker compose -f .docker/compose/docker-compose.playwright-ci.yml up -d

# Health check to match CI wait loop behavior.
curl -sf http://127.0.0.1:8080/api/v1/health > /dev/null 2>&1
```

### Exact Playwright CLI Invocation (Non-Security Shards)
```bash
export PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080
export CI=true
export TEST_WORKER_INDEX=<SHARD_INDEX>
export CHARON_EMERGENCY_TOKEN=<SECRET>
export CHARON_EMERGENCY_SERVER_ENABLED=true
export CHARON_SECURITY_TESTS_ENABLED=false
export CHARON_E2E_IMAGE_TAG=<IMAGE_TAG>

npx playwright test \
  --project=chromium \
  --shard=<SHARD_INDEX>/<TOTAL_SHARDS> \
  --output=playwright-output/chromium-shard-<SHARD_INDEX> \
  tests/core \
  tests/dns-provider-crud.spec.ts \
  tests/dns-provider-types.spec.ts \
  tests/integration \
  tests/manual-dns-provider.spec.ts \
  tests/monitoring \
  tests/settings \
  tests/tasks
```

### Post-Failure Diagnostic Collection (CI Always-Run)
```bash
mkdir -p diagnostics
uptime > diagnostics/uptime.txt
free -m > diagnostics/free-m.txt
df -h > diagnostics/df-h.txt
ps aux > diagnostics/ps-aux.txt
docker ps -a > diagnostics/docker-ps.txt || true
docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true
docker compose -f .docker/compose/docker-compose.playwright-ci.yml logs > docker-logs-shard.txt 2>&1
```

## Emergency Server Port (2020) Configuration
- No explicit references to port 2020 were found in workflow YAMLs. The E2E workflow sets `CHARON_EMERGENCY_SERVER_ENABLED=true` but does not validate port 2020 availability.

## Job Log Evidence (Shard 3)
- No runner cancellation, runner lost, or OOM strings were present in the reviewed job log text.
- The job log shows Playwright test-level timeouts (10s and 60s expectations), not a job-level timeout.
- The job log shows the shard command executed with `--shard=3/4` and standard suite list, indicating the job did run sharded Playwright as expected.

Excerpt:
```
2026-02-10T12:58:19.5379132Z npx playwright test \
2026-02-10T12:58:19.5379658Z   --shard=3/4 \
2026-02-10T13:06:49.1304667Z     Test timeout of 60000ms exceeded.
```

## Proposed Workflow YAML Changes (Section 9)
The following changes were applied to the E2E workflow to align with the spec:

```yaml
# Timeout increase (temporary)
  e2e-chromium:
    timeout-minutes: 60

# Per-shard output + artifact upload
      - name: Run Chromium Non-Security Tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})
        run: |
          npx playwright test \
            --project=chromium \
            --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
            --output=playwright-output/chromium-shard-${{ matrix.shard }} \
            ...

      - name: Upload Playwright output (Chromium shard ${{ matrix.shard }})
        if: always()
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
        with:
          name: playwright-output-chromium-shard-${{ matrix.shard }}
          path: playwright-output/chromium-shard-${{ matrix.shard }}/

# Diagnostics (always)
      - name: Collect diagnostics
        if: always()
        run: |
          mkdir -p diagnostics
          uptime > diagnostics/uptime.txt
          free -m > diagnostics/free-m.txt
          df -h > diagnostics/df-h.txt
          ps aux > diagnostics/ps-aux.txt
          docker ps -a > diagnostics/docker-ps.txt || true
          docker logs --tail 500 charon-e2e > diagnostics/docker-charon-e2e.log 2>&1 || true

      - name: Upload diagnostics
        if: always()
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6.0.0
        with:
          name: e2e-diagnostics-chromium-shard-${{ matrix.shard }}
          path: diagnostics/
```

## Quick Mitigation Checklist (P0)
- Increase E2E job timeouts to 60 minutes in the E2E workflow to eliminate premature job cancellation risk.
- Collect diagnostics on every shard with `if: always()` and upload artifacts.
- Enforce per-shard `--output` paths and upload them as artifacts so traces and JSON are preserved even on failure.
- Re-run the failing shard locally with the exact shard flags and diagnostics enabled to capture a trace.

## CI Remediation Priority Labels (Spec Section 5)
### P0 (Immediate - already applied)
- Timeout increase to 60 minutes for E2E shard jobs.
- Always-run diagnostics collection and artifact upload.

### P1 (Same-day)
- Add a lightweight CI smoke check step before shard execution (health check + minimal Playwright smoke).
- Add basic resource monitoring output (CPU/memory/disk) to the diagnostics bundle.

### P2 (Next sprint)
- Implement shard balancing based on historical test durations.
- Stand up a test-duration/flake telemetry dashboard for CI trends.

## Explicit Confirmation Checklist
- [x] Workflow timeout-minutes locations identified
  ✓ Found timeout-minutes entries in .github/workflows (e.g., [.github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L216), [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml#L52), [.github/workflows/docs.yml](.github/workflows/docs.yml#L27), [.github/workflows/security-weekly-rebuild.yml](.github/workflows/security-weekly-rebuild.yml#L30)).
- [x] Job cancellation evidence searched
  ✓ Searched /tmp/job-63106399789-logs.zip for "Job canceled", "cancelled", and "runner lost"; no matches found.
- [x] OOM/kill signals searched
  ✓ Searched /tmp/job-63106399789-logs.zip for "Killed", "OOM", "oom_reaper", and "Out of memory"; no matches found.
- [x] Runner type confirmed (hosted vs self-hosted)
  ✓ E2E workflow runs on GitHub-hosted runners via runs-on: ubuntu-latest (see [.github/workflows/e2e-tests-split.yml](.github/workflows/e2e-tests-split.yml#L108)).
- [x] Emergency server port config validated
  ✓ Port 2020 is configured in Playwright CI compose with host mapping and bind (see [.docker/compose/docker-compose.playwright-ci.yml](.docker/compose/docker-compose.playwright-ci.yml#L42) and [.docker/compose/docker-compose.playwright-ci.yml](.docker/compose/docker-compose.playwright-ci.yml#L61)).
