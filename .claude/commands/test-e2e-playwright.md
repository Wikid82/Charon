# Test: E2E Playwright Tests

Run Charon end-to-end tests with Playwright.

## Command

```bash
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

## Direct Alternative (Recommended for local runs)

```bash
cd /projects/Charon && npx playwright test --project=firefox
```

## Prerequisites

The E2E container must be running and healthy. Rebuild if application code changed:

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

## Targeted Testing

```bash
# Specific test file
npx playwright test tests/proxy-hosts.spec.ts --project=firefox

# Specific test by name
npx playwright test --grep "user can create proxy host" --project=firefox

# All browsers (for full CI parity)
npx playwright test --project=chromium --project=firefox --project=webkit

# Debug mode (headed browser)
npx playwright test --project=firefox --headed --debug
```

## CRITICAL: Never Truncate Output

**NEVER** pipe Playwright output through `head`, `tail`, or other truncating commands. Playwright requires user input to quit when piped, causing hangs.

## View Report

```bash
npx playwright show-report
```

## On Failure

1. Capture **full** output — never truncate
2. Use EARS methodology for structured failure analysis
3. Check if a code bug needs fixing (delegate to `backend-dev` or `frontend-dev` agents)
4. Fix root cause — do NOT skip or delete the failing test

## Related

- `/docker-rebuild-e2e` — Rebuild E2E container
- `/playwright-generate-test` — Generate new Playwright tests
