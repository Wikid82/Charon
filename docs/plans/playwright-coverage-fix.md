# Playwright Coverage Fix Plan

**Date:** January 21, 2026
**Status:** Ready for Implementation
**Priority:** Critical
**Issue:** Playwright E2E coverage is calculating as "unknown %" with empty coverage files

---

## Root Cause Analysis

### Problem Statement
The Playwright coverage reports are empty:
- `coverage/e2e/coverage.json` contains only `{}`
- `coverage/e2e/lcov.info` is empty (0 bytes)

### Root Cause
The `@bgotink/playwright-coverage` package uses **V8 coverage** to track code execution. V8 coverage works by:
1. Instrumenting JavaScript at the V8 engine level
2. Tracking which source lines are executed
3. Mapping executed code back to original source files

**The issue:** When tests run against the Docker container (`localhost:8080`), they're hitting **pre-bundled/minified code** from the production build. V8 coverage cannot map this back to source files because:
- Source maps may not be available in the Docker container
- The bundled code structure differs from the source structure
- File paths don't match the `sourceRoot` in the coverage config

**The fix:** Tests must run against the **Vite dev server** (`localhost:3000`) which:
- Serves source files directly (ESM modules)
- Provides inline source maps
- Allows V8 to map execution back to original TypeScript/TSX files

### Secondary Issue: Port Mismatch
- Vite config specifies `port: 3000`
- Coverage script uses `VITE_PORT=5173` (default)
- This causes the script to wait for a server on the wrong port

---

## Implementation Plan

### Phase 1: Fix Port Configuration

**File:** `frontend/vite.config.ts`
**Change:** Update port to 5173 (Vite default, matches coverage script)

```typescript
server: {
  port: 5173,  // Changed from 3000 to match coverage script
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true
    }
  }
}
```

### Phase 2: Update Coverage Script Output

**File:** `.github/skills/test-e2e-playwright-coverage-scripts/run.sh`
**Changes:**
1. Fix the hardcoded port 3000 reference in logging
2. Add coverage summary extraction and display
3. Add threshold enforcement (85% minimum)

### Phase 3: Add Coverage Threshold Validation

**File:** `playwright.config.js`
**Add:** Coverage thresholds in the reporter config

```javascript
const coverageReporterConfig = defineCoverageReporterConfig({
  // ... existing config ...

  // Add threshold enforcement
  check: {
    global: {
      statements: 85,
      branches: 85,
      functions: 85,
      lines: 85,
    },
  },
});
```

### Phase 4: CI Workflow Update

**File:** `.github/workflows/e2e-tests.yml` (if exists)
**Add:** Coverage upload to Codecov with e2e flag

---

## Files to Modify

| File | Change Type | Description |
|------|-------------|-------------|
| `frontend/vite.config.ts` | UPDATE | Change port from 3000 to 5173 |
| `.github/skills/test-e2e-playwright-coverage-scripts/run.sh` | UPDATE | Fix port logging, add coverage summary |
| `playwright.config.js` | UPDATE | Add coverage thresholds (85%) |
| `docs/plans/chores.md` | UPDATE | Mark task as complete |

---

## Testing the Fix

### Manual Verification Steps
1. Start Docker backend: `docker compose -f .docker/compose/docker-compose.local.yml up -d`
2. Run coverage skill: `.github/skills/scripts/skill-runner.sh test-e2e-playwright-coverage`
3. Verify output shows:
   - Vite starting on port 5173
   - Tests passing
   - Coverage percentage displayed (not "unknown")
   - `coverage/e2e/lcov.info` contains data
   - `coverage/e2e/coverage.json` contains coverage metrics

### Expected Output
```
✅ Coverage Summary:
  Statements: XX%
  Branches: XX%
  Functions: XX%
  Lines: XX%
```

---

## Definition of Done

- [ ] Vite dev server starts on port 5173
- [ ] Coverage script successfully collects V8 coverage data
- [ ] `coverage/e2e/lcov.info` contains valid LCOV data
- [ ] `coverage/e2e/coverage.json` contains coverage metrics with percentages
- [ ] Coverage summary displays actual percentages (not "unknown")
- [ ] 85% coverage threshold enforced for local and CI
- [ ] All existing E2E tests pass

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Vite port change breaks other tooling | Low | Medium | Port 5173 is Vite default, less conflict |
| Coverage collection slows tests | Low | Low | V8 coverage has minimal overhead |
| Source map resolution issues | Medium | High | Test with multiple file types |

---

## Notes

The `@bgotink/playwright-coverage` package documentation explicitly states:
> **"Coverage is empty"**: Did you perhaps use `@playwright/test`'s own `test` function? If you don't use a `test` function created using `mixinCoverage`, coverage won't be tracked.

Our tests correctly import from `@bgotink/playwright-coverage`, so the issue is definitely the source file resolution, not the test setup.
