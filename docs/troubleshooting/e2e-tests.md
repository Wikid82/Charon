# E2E Test Troubleshooting

Common issues and solutions for Playwright E2E tests.

---

## Recent Improvements (2026-02)

### Test Timeout Issues - RESOLVED

**Symptoms**: Tests timing out after 30 seconds, config reload overlay blocking interactions

**Resolution**:

- Extended timeout from 30s to 60s for feature flag propagation
- Added automatic detection and waiting for config reload overlay
- Improved test isolation with proper cleanup in afterEach hooks

**If you still experience timeouts**:

1. Rebuild the E2E container: `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
2. Check Docker logs for health check failures
3. Verify emergency token is set in `.env` file

### API Key Format Mismatch - RESOLVED

**Symptoms**: Feature flag tests failing with propagation timeout

**Resolution**:

- Added key normalization to handle both `feature.cerberus.enabled` and `cerberus.enabled` formats
- Tests now automatically detect and adapt to API response format

**Configuration**: No manual configuration needed, normalization is automatic.

---

## Quick Diagnostics

**Run these commands first:**

```bash
# Check emergency token is set
grep CHARON_EMERGENCY_TOKEN .env

# Verify token length
echo -n "$(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" | wc -c
# Should output: 64

# Check Docker container is running
docker ps | grep charon

# Check health endpoint
curl -f http://localhost:8080/api/v1/health || echo "Health check failed"
```

---

## Error: "CHARON_EMERGENCY_TOKEN is not set"

### Symptoms

- Tests fail immediately with environment configuration error
- Error appears in global setup before any tests run

### Cause

Emergency token not configured in `.env` file.

### Solution

1. **Generate token:**

   ```bash
   openssl rand -hex 32
   ```

2. **Add to `.env` file:**

   ```bash
   echo "CHARON_EMERGENCY_TOKEN=<paste_token_here>" >> .env
   ```

3. **Verify:**

   ```bash
   grep CHARON_EMERGENCY_TOKEN .env
   ```

4. **Run tests:**

   ```bash
   npx playwright test --project=chromium
   ```

📖 **More Info:** See [Getting Started - Emergency Token Configuration](../getting-started.md#step-18-emergency-token-configuration-development--e2e-tests)

---

## Error: "CHARON_EMERGENCY_TOKEN is too short"

### Symptoms

- Global setup fails with message about token length
- Current token length shown in error (e.g., "32 chars, minimum 64")

### Cause

Token is shorter than 64 characters (security requirement).

### Solution

1. **Regenerate token with correct length:**

   ```bash
   openssl rand -hex 32  # Generates 64-char hex string
   ```

2. **Update `.env` file:**

   ```bash
   sed -i "s/CHARON_EMERGENCY_TOKEN=.*/CHARON_EMERGENCY_TOKEN=<new_token>/" .env
   ```

3. **Verify length:**

   ```bash
   echo -n "$(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" | wc -c
   # Should output: 64
   ```

---

## Error: "Failed to reset security modules using emergency token"

### Symptoms

- Security teardown fails
- Causes 20+ cascading test failures
- Error message about emergency reset

### Possible Causes

1. **Token too short** (< 64 chars)
2. **Token doesn't match backend configuration**
3. **Backend not running or unreachable**
4. **Network/container issues**

### Solution

**Step 1: Verify token configuration**

```bash
# Check token exists and is 64 chars
echo -n "$(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" | wc -c

# Check backend env matches (if using Docker)
docker exec charon env | grep CHARON_EMERGENCY_TOKEN
```

**Step 2: Verify backend is running**

```bash
curl http://localhost:8080/api/v1/health
# Should return: {"status":"ok"}
```

**Step 3: Test emergency endpoint directly**

```bash
curl -X POST http://localhost:8080/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: $(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" \
  -H "Content-Type: application/json" \
  -d '{"reason":"manual test"}' | jq
```

**Step 4: Check backend logs**

```bash
# Docker Compose
docker compose logs charon | tail -50

# Docker Run
docker logs charon | tail -50
```

**Step 5: Regenerate token if needed**

```bash
# Generate new token
NEW_TOKEN=$(openssl rand -hex 32)

# Update .env
sed -i "s/CHARON_EMERGENCY_TOKEN=.*/CHARON_EMERGENCY_TOKEN=${NEW_TOKEN}/" .env

# Restart backend with new token
docker restart charon

# Wait for health
sleep 5 && curl http://localhost:8080/api/v1/health
```

---

## Error: "Blocked by access control list" (403)

### Symptoms

- Most tests fail with 403 Forbidden errors
- Error message contains "Blocked by access control"

### Cause

Security teardown did not successfully disable ACL before tests ran.

### Solution

1. **Run teardown script manually:**

   ```bash
   npx playwright test tests/security-teardown.setup.ts
   ```

2. **Check teardown output for errors:**
   - Look for "Emergency reset successful" message
   - Verify no error messages about missing token

3. **Verify ACL is disabled:**

   ```bash
   curl http://localhost:8080/api/v1/security/status | jq
   # acl.enabled should be false
   ```

4. **If still blocked, manually disable via API:**

   ```bash
   # Using emergency token
   curl -X POST http://localhost:8080/api/v1/emergency/security-reset \
     -H "X-Emergency-Token: $(grep CHARON_EMERGENCY_TOKEN .env | cut -d= -f2)" \
     -H "Content-Type: application/json" \
     -d '{"reason":"manual disable before tests"}'
   ```

5. **Run tests again:**

   ```bash
   npx playwright test --project=chromium
   ```

---

## Tests Pass Locally but Fail in CI/CD

### Symptoms

- Tests work on your machine
- Same tests fail in GitHub Actions
- Error about missing emergency token in CI logs

### Cause

`CHARON_EMERGENCY_TOKEN` not configured in GitHub Secrets.

### Solution

1. **Navigate to repository settings:**
   - Go to: `https://github.com/<your-org>/<your-repo>/settings/secrets/actions`
   - Or: Repository → Settings → Secrets and Variables → Actions

2. **Create secret:**
   - Click **"New repository secret"**
   - Name: `CHARON_EMERGENCY_TOKEN`
   - Value: Generate with `openssl rand -hex 32`
   - Click **"Add secret"**

3. **Verify secret is set:**
   - Secret should appear in list (value is masked)
   - Cannot view value after creation (security)

4. **Re-run workflow:**
   - Navigate to Actions tab
   - Re-run failed workflow
   - Check "Validate Emergency Token Configuration" step passes

📖 **Detailed Instructions:** See [GitHub Setup Guide](../github-setup.md)

---

## Error: "ECONNREFUSED" or "ENOTFOUND"

### Symptoms

- Tests fail with connection refused errors
- Cannot reach `localhost:8080` or configured base URL

### Cause

Backend container not running or not accessible.

### Solution

1. **Check container status:**

   ```bash
   docker ps | grep charon
   ```

2. **If not running, start it:**

   ```bash
   # Docker Compose
   docker compose up -d

   # Docker Run
   docker start charon
   ```

3. **Wait for health:**

   ```bash
   timeout 60 bash -c 'until curl -f http://localhost:8080/api/v1/health; do sleep 2; done'
   ```

4. **Check logs if still failing:**

   ```bash
   docker logs charon | tail -50
   ```

---

## Error: Token appears to be a placeholder value

### Symptoms

- Global setup validation fails
- Error mentions "placeholder value"

### Cause

Token contains common placeholder strings like:

- `test-emergency-token`
- `your_64_character`
- `replace_this`
- `0000000000000000`

### Solution

1. **Generate a unique token:**

   ```bash
   openssl rand -hex 32
   ```

2. **Replace placeholder in `.env`:**

   ```bash
   sed -i "s/CHARON_EMERGENCY_TOKEN=.*/CHARON_EMERGENCY_TOKEN=<new_token>/" .env
   ```

3. **Verify it's not a placeholder:**

   ```bash
   grep CHARON_EMERGENCY_TOKEN .env
   # Should show a random hex string
   ```

---

## Debug Mode

Run tests with full debugging for deeper investigation:

### With Playwright Inspector

```bash
npx playwright test --debug
```

Interactive UI for stepping through tests.

### With Full Traces

```bash
npx playwright test --trace=on
```

Capture execution traces for each test.

### View Trace After Test

```bash
npx playwright show-trace test-results/traces/*.zip
```

Opens trace viewer in browser.

### With Enhanced Logging

```bash
DEBUG=charon:*,charon-test:* PLAYWRIGHT_DEBUG=1 npx playwright test --project=chromium
```

Enables all debug output.

---

## Performance Issues

### Tests Running Slowly

**Symptoms:** Tests take > 5 minutes for full suite.

**Solutions:**

1. **Use sharding (parallel execution):**

   ```bash
   npx playwright test --shard=1/4 --project=chromium
   ```

2. **Run specific test files:**

   ```bash
   npx playwright test tests/manual-dns-provider.spec.ts
   ```

3. **Skip slow tests during development:**

   ```bash
   npx playwright test --grep-invert "@slow"
   ```

### Feature Flag Toggle Tests Timing Out

**Symptoms:**

- Tests in `tests/settings/system-settings.spec.ts` fail with timeout errors
- Error messages mention feature flag toggles (Cerberus, CrowdSec, Uptime, Persist)

**Cause:**

- Backend N+1 query pattern causing 300-600ms latency in CI
- Hard-coded waits insufficient for slower CI environments

**Solution (Fixed in v2.x):**

- Backend now uses batch query pattern (3-6x faster: 600ms → 200ms P99)
- Tests use condition-based polling with `waitForFeatureFlagPropagation()`
- Retry logic with exponential backoff handles transient failures

**If you still experience issues:**

1. Check backend latency: `grep "[METRICS]" docker logs charon`
2. Verify batch query is being used (should see `WHERE key IN (...)` in logs)
3. Ensure you're running latest version with the optimization

📖 **See Also:** [Feature Flags Performance Documentation](../performance/feature-flags-endpoint.md)

### Container Startup Slow

**Symptoms:** Health check timeouts, tests fail before running.

**Solutions:**

1. **Increase health check timeout:**

   ```bash
   timeout 120 bash -c 'until curl -f http://localhost:8080/api/v1/health; do sleep 2; done'
   ```

2. **Pre-pull Docker image:**

   ```bash
   docker pull wikid82/charon:latest
   ```

3. **Check Docker resource limits:**

   ```bash
   docker stats charon
   # Ensure adequate CPU/memory
   ```

---

## Getting Help

If you're still stuck after trying these solutions:

1. **Check known issues:**
   - Review [E2E Triage Report](../reports/e2e_triage_report.md)
   - Search [GitHub Issues](https://github.com/Wikid82/charon/issues)

2. **Collect diagnostic info:**

   ```bash
   # Environment
   echo "OS: $(uname -a)"
   echo "Docker: $(docker --version)"
   echo "Node: $(node --version)"

   # Configuration
   echo "Base URL: ${PLAYWRIGHT_BASE_URL:-http://localhost:8080}"
   echo "Token set: $([ -n "$CHARON_EMERGENCY_TOKEN" ] && echo "Yes" || echo "No")"

   # Logs
   docker logs charon > charon-logs.txt
   npx playwright test --project=chromium > test-output.txt 2>&1
   ```

3. **Open GitHub issue:**
   - Include diagnostic info above
   - Attach `charon-logs.txt` and `test-output.txt`
   - Describe steps to reproduce
   - Tag with `testing` and `e2e` labels

4. **Ask in community:**
   - [GitHub Discussions](https://github.com/Wikid82/charon/discussions)
   - Include relevant error messages (mask any secrets!)

---

## Related Documentation

- [Getting Started Guide](../getting-started.md)
- [GitHub Setup Guide](../github-setup.md)
- [Feature Flags Performance Documentation](../performance/feature-flags-endpoint.md)
- [E2E Triage Report](../reports/e2e_triage_report.md)
- [Playwright Documentation](https://playwright.dev/docs/intro)

---

**Last Updated:** 2026-02-02
