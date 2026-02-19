# Codecov Configuration Analysis & Recommendations

**Date:** December 14, 2025
**Issue:** Local coverage (85.1%) vs Codecov dashboard (Backend 81.05%, Frontend 81.79%, Overall 81.23%)

---

## 1. Current Ignore Configuration Analysis

### Current `.codecov.yml` Ignore Patterns

The existing configuration at [.codecov.yml](../../.codecov.yml) already has a comprehensive ignore list:

| Category | Patterns | Status |
|----------|----------|--------|
| **Test files** | `**/tests/**`, `**/test/**`, `**/__tests__/**`, `**/*_test.go`, `**/*.test.ts`, `**/*.test.tsx`, `**/*.spec.ts`, `**/*.spec.tsx` | ✅ Good |
| **Vitest config** | `**/vitest.config.ts`, `**/vitest.setup.ts` | ✅ Good |
| **E2E/Integration** | `**/e2e/**`, `**/integration/**` | ✅ Good |
| **Documentation** | `docs/**`, `*.md` | ✅ Good |
| **CI/Config** | `.github/**`, `scripts/**`, `tools/**`, `*.yml`, `*.yaml`, `*.json` | ✅ Good |
| **Frontend artifacts** | `frontend/node_modules/**`, `frontend/dist/**`, `frontend/coverage/**`, `frontend/test-results/**`, `frontend/public/**` | ✅ Good |
| **Backend artifacts** | `backend/cmd/seed/**`, `backend/data/**`, `backend/coverage/**`, `backend/bin/**`, `backend/*.cover`, `backend/*.out`, `backend/*.html`, `backend/codeql-db/**` | ✅ Good |
| **Docker-only code** | `backend/internal/services/docker_service.go`, `backend/internal/api/handlers/docker_handler.go` | ✅ Good |
| **CodeQL artifacts** | `codeql-db/**`, `codeql-db-*/**`, `codeql-agent-results/**`, `codeql-custom-queries-*/**`, `*.sarif` | ✅ Good |
| **Config files** | `**/tailwind.config.js`, `**/postcss.config.js`, `**/eslint.config.js`, `**/vite.config.ts`, `**/tsconfig*.json` | ✅ Good |
| **Type definitions** | `**/*.d.ts` | ✅ Good |
| **Data directories** | `import/**`, `data/**`, `.cache/**`, `configs/crowdsec/**` | ✅ Good |

### Coverage Discrepancy Root Cause

The ~4% difference between local (85.1%) and Codecov (81.23%) is likely due to:

1. **Local script exclusions not in Codecov**: The `scripts/go-test-coverage.sh` excludes packages via `sed` filtering:
   - `github.com/Wikid82/charon/backend/cmd/api`
   - `github.com/Wikid82/charon/backend/cmd/seed`
   - `github.com/Wikid82/charon/backend/internal/logger`
   - `github.com/Wikid82/charon/backend/internal/metrics`
   - `github.com/Wikid82/charon/backend/internal/trace`
   - `github.com/Wikid82/charon/backend/integration`

2. **Frontend test utilities counted as source**: Several test utility directories/files may be included:
   - `frontend/src/test/` - Test setup files
   - `frontend/src/test-utils/` - Test helper utilities
   - `frontend/src/testUtils/` - Additional test helpers
   - `frontend/src/data/mockData.ts` (already in vitest.config.ts excludes but not in Codecov)

3. **Entry point files**: Main bootstrap files with minimal testable logic:
   - `backend/cmd/api/main.go` - App bootstrap
   - `frontend/src/main.tsx` - React entry point

---

## 2. Recommended Additions

### High Priority (Align with Local Coverage)

| Pattern | Rationale | Impact |
|---------|-----------|--------|
| `backend/cmd/api/**` | Main entry point - bootstrap code, CLI handling | ~1-2% |
| `backend/internal/logger/**` | Logging infrastructure - already excluded locally | ~0.5% |
| `backend/internal/metrics/**` | Observability infrastructure | ~0.5% |
| `backend/internal/trace/**` | Tracing infrastructure | ~0.3% |

### Medium Priority (Test Infrastructure)

| Pattern | Rationale | Impact |
|---------|-----------|--------|
| `frontend/src/test/**` | Test setup files (`setup.ts`, `setup.spec.ts`) | ~0.3% |
| `frontend/src/test-utils/**` | Query client helpers for tests | ~0.2% |
| `frontend/src/testUtils/**` | Mock proxy host creators | ~0.2% |
| `**/mockData.ts` | Test data factories | ~0.2% |
| `**/createTestQueryClient.ts` | Test-specific utilities | ~0.1% |
| `**/createMockProxyHost.ts` | Test-specific utilities | ~0.1% |
| `frontend/src/main.tsx` | React bootstrap - no logic to test | ~0.1% |

### Low Priority (Already Partially Covered)

| Pattern | Rationale | Impact |
|---------|-----------|--------|
| `**/playwright.config.ts` | E2E configuration | Minimal |
| `backend/tools/**` | Build scripts (tools/ already ignored) | Already covered |

---

## 3. Exact YAML Changes for `.codecov.yml`

Add the following patterns to the `ignore:` section:

```yaml
# -----------------------------------------------------------------------------
# Exclude from coverage reporting
# -----------------------------------------------------------------------------
ignore:
  # Test files
  - "**/tests/**"
  - "**/test/**"
  - "**/__tests__/**"
  - "**/test_*.go"
  - "**/*_test.go"
  - "**/*.test.ts"
  - "**/*.test.tsx"
  - "**/*.spec.ts"
  - "**/*.spec.tsx"
  - "**/vitest.config.ts"
  - "**/vitest.setup.ts"

  # E2E tests
  - "**/e2e/**"
  - "**/integration/**"

  # === NEW: Frontend test utilities ===
  - "frontend/src/test/**"
  - "frontend/src/test-utils/**"
  - "frontend/src/testUtils/**"
  - "**/mockData.ts"
  - "**/createTestQueryClient.ts"
  - "**/createMockProxyHost.ts"

  # === NEW: Entry points (bootstrap code, minimal logic) ===
  - "backend/cmd/api/**"
  - "frontend/src/main.tsx"

  # === NEW: Infrastructure packages (align with local coverage script) ===
  - "backend/internal/logger/**"
  - "backend/internal/metrics/**"
  - "backend/internal/trace/**"

  # Documentation
  - "docs/**"
  - "*.md"

  # CI/CD & Config
  - ".github/**"
  - "scripts/**"
  - "tools/**"
  - "*.yml"
  - "*.yaml"
  - "*.json"

  # Frontend build artifacts & dependencies
  - "frontend/node_modules/**"
  - "frontend/dist/**"
  - "frontend/coverage/**"
  - "frontend/test-results/**"
  - "frontend/public/**"

  # Backend non-source files
  - "backend/cmd/seed/**"
  - "backend/data/**"
  - "backend/coverage/**"
  - "backend/bin/**"
  - "backend/*.cover"
  - "backend/*.out"
  - "backend/*.html"
  - "backend/codeql-db/**"

  # Docker-only code (not testable in CI)
  - "backend/internal/services/docker_service.go"
  - "backend/internal/api/handlers/docker_handler.go"

  # CodeQL artifacts
  - "codeql-db/**"
  - "codeql-db-*/**"
  - "codeql-agent-results/**"
  - "codeql-custom-queries-*/**"
  - "*.sarif"

  # Config files (no logic)
  - "**/tailwind.config.js"
  - "**/postcss.config.js"
  - "**/eslint.config.js"
  - "**/vite.config.ts"
  - "**/tsconfig*.json"
  - "**/playwright.config.ts"

  # Type definitions only
  - "**/*.d.ts"

  # Import/data directories
  - "import/**"
  - "data/**"
  - ".cache/**"

  # CrowdSec config files (no logic to test)
  - "configs/crowdsec/**"
```

---

## 4. Summary of New Patterns

### Patterns to Add (12 new entries)

```yaml
# Frontend test utilities
- "frontend/src/test/**"
- "frontend/src/test-utils/**"
- "frontend/src/testUtils/**"
- "**/mockData.ts"
- "**/createTestQueryClient.ts"
- "**/createMockProxyHost.ts"

# Entry points
- "backend/cmd/api/**"
- "frontend/src/main.tsx"

# Infrastructure packages
- "backend/internal/logger/**"
- "backend/internal/metrics/**"
- "backend/internal/trace/**"

# Additional config
- "**/playwright.config.ts"
```

### Expected Impact

After applying these changes:

- **Backend Codecov**: Should increase from 81.05% → ~84-85%
- **Frontend Codecov**: Should increase from 81.79% → ~84-85%
- **Overall Codecov**: Should increase from 81.23% → ~84-85%

This will align Codecov reporting with local coverage calculations by ensuring the same exclusions are applied in both environments.

---

## 5. Validation Steps

1. Apply the YAML changes to `.codecov.yml`
2. Push to trigger CI workflow
3. Compare new Codecov dashboard percentages with local `scripts/go-test-coverage.sh` output
4. If still misaligned, check for additional patterns in vitest.config.ts coverage.exclude not in Codecov

---

## 6. Alternative Consideration

If exact parity isn't achieved, consider that:

- Codecov may calculate coverage differently (line vs statement vs branch)
- Go coverage profiles include function coverage that may be weighted differently
- The local script uses `sed` filtering on the raw coverage file, which Codecov cannot replicate

The ignore patterns above address files that **should never be counted** regardless of methodology differences.
