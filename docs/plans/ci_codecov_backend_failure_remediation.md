# CI Codecov Backend Test Failures - Remediation Plan

**Date:** 2026-02-16
**Status:** Investigation Complete - Ready for Implementation
**Priority:** CRITICAL CI BLOCKER
**Workflow:** `.github/workflows/codecov-upload.yml` → `backend-codecov` job

---

## Executive Summary

**CRITICAL: Multiple CI workflows are failing** with the same root cause. Investigation reveals these failures affect 3 workflows, not just codecov-upload.

### Affected Workflows

| Workflow File | Purpose | Job Name(s) | Test Command | Status | Priority |
|---------------|---------|-------------|--------------|--------|----------|
| `codecov-upload.yml` | Coverage upload to Codecov | `backend-codecov` | `go-test-coverage.sh` | ❌ Failing | **CRITICAL** |
| `quality-checks.yml` | PR quality gates | `backend-quality` | `go-test-coverage.sh` + `go test -run TestPerf` | ❌ Failing | **CRITICAL** |
| `benchmark.yml` | Performance regression checks | `benchmark` | `go test -bench` + `go test -run TestPerf` | ⚠️ At Risk | **HIGH** |

**Other Workflows Analyzed (NOT affected):**
- ✅ `e2e-tests-split.yml` - Already has `CHARON_ENCRYPTION_KEY` configured (6+ locations)
- ✅ `cerberus-integration.yml` - Runs integration scripts, not Go unit tests
- ✅ `crowdsec-integration.yml` - Runs integration scripts, not Go unit tests
- ✅ All other workflows - Do not run backend Go tests

### Root Cause Issues

1. **RotationService Initialization Warnings** (Non-blocking but pollutes logs)
   - Multiple services print: "Warning: RotationService initialization failed, using basic encryption: CHARON_ENCRYPTION_KEY is required"
   - Root cause: Missing `CHARON_ENCRYPTION_KEY` environment variable in ALL 3 affected workflows
   - Impact: Services fall back to basic encryption (no test failures, but warnings appear)

2. **GORM "record not found" Errors** (Blocking failures)
   - Source: `backend/internal/services/proxyhost_service.go:194`
   - Root cause: Tests calling `GetByID()` without proper test data setup
   - Impact: Tests expecting proxy host records fail with `gorm.ErrRecordNotFound`

---

## Investigation Findings

### 1. Encryption Key Requirements

#### File Analysis: `.github/workflows/codecov-upload.yml`
**Path:** `/projects/Charon/.github/workflows/codecov-upload.yml`
**Lines:** 43-53 (backend-codecov job)

**Current Environment Variables:**
```yaml
env:
  CGO_ENABLED: 1
```

**Missing Variables:**
- `CHARON_ENCRYPTION_KEY` (required for RotationService)

#### File Analysis: `backend/internal/crypto/rotation_service.go`
**Path:** `/projects/Charon/backend/internal/crypto/rotation_service.go`
**Lines:** 63-75

**Error Trigger:**
```go
func NewRotationService(db *gorm.DB) (*RotationService, error) {
	// Load current key (required)
	currentKeyB64 := os.Getenv("CHARON_ENCRYPTION_KEY")
	if currentKeyB64 == "" {
		return nil, fmt.Errorf("CHARON_ENCRYPTION_KEY is required")
	}
	// ...
}
```

#### File Analysis: Service Dependencies
**Affected Services:**
- `backend/internal/services/dns_provider_service.go:145` - Calls `crypto.NewRotationService(db)`
- `backend/internal/services/credential_service.go:72` - Calls `crypto.NewRotationService(db)`

**Fallback Behavior:**
```go
rotationService, err := crypto.NewRotationService(db)
if err != nil {
	// Fallback to non-rotation mode
	fmt.Printf("Warning: RotationService initialization failed, using basic encryption: %v\n", err)
}
```

**Test Setup Comparison:**

| Test File | Sets CHARON_ENCRYPTION_KEY? | Uses RotationService? |
|-----------|----------------------------|-----------------------|
| `rotation_service_test.go` | ✅ Yes (via `setupTestKeys()`) | ✅ Yes |
| `dns_provider_service_test.go` | ❌ No (hardcoded test key) | ⚠️ Tries but falls back |
| `credential_service_test.go` | ❌ No (hardcoded test key) | ⚠️ Tries but falls back |

#### Example: How Tests Set Encryption Keys

**File:** `backend/internal/crypto/rotation_service_test.go:28-41`
```go
func setupTestKeys(t *testing.T) (currentKey, nextKey, legacyKey string) {
	currentKey, err := GenerateNewKey()
	require.NoError(t, err)

	_ = os.Setenv("CHARON_ENCRYPTION_KEY", currentKey)
	t.Cleanup(func() { _ = os.Unsetenv("CHARON_ENCRYPTION_KEY") })

	return currentKey, nextKey, legacyKey
}
```

**File:** `backend/internal/services/dns_provider_service_test.go:62`
```go
// Does NOT set CHARON_ENCRYPTION_KEY
encryptor, err := crypto.NewEncryptionService("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
```

### 2. ProxyHost "Record Not Found" Errors

#### File Analysis: `backend/internal/services/proxyhost_service.go`
**Path:** `/projects/Charon/backend/internal/services/proxyhost_service.go`
**Lines:** 192-197

**Error Source:**
```go
func (s *ProxyHostService) GetByID(id uint) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.Where("id = ?", id).First(&host).Error; err != nil {
		return nil, err  // Returns gorm.ErrRecordNotFound if no record
	}
	return &host, nil
}
```

**GORM Error Type:** `gorm.ErrRecordNotFound` (not explicitly handled in ProxyHostService)

#### Test Pattern Analysis

**File:** `backend/internal/services/proxyhost_service_test.go:73-102`

**Working Test Pattern:**
```go
func TestProxyHostService_CRUD(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// Create test data BEFORE calling GetByID
	host := &models.ProxyHost{
		UUID:        "uuid-1",
		DomainNames: "test.example.com",
		ForwardHost: "127.0.0.1",
		ForwardPort: 8080,
	}
	err := service.Create(host)  // Creates record in DB
	assert.NoError(t, err)
	assert.NotZero(t, host.ID)

	// Now GetByID works because record exists
	fetched, err := service.GetByID(host.ID)
	assert.NoError(t, err)
	assert.Equal(t, host.DomainNames, fetched.DomainNames)
}
```

**File:** `backend/internal/api/handlers/proxy_host_handler_update_test.go:50-60`

**Helper Function Pattern:**
```go
func createTestProxyHost(t *testing.T, db *gorm.DB, name string) models.ProxyHost {
	host := models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          name,
		DomainNames:   name + ".test.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)
	return host
}
```

#### Likely Failure Scenario

**Hypothesis:** Some tests are calling `GetByID()` with a hardcoded ID (e.g., `GetByID(1)`) expecting a record to exist, but:
- SQLite in-memory DB is empty at test start
- Test doesn't create the record before calling `GetByID()`
- Test previously relied on global seeding that no longer runs

**To Identify Failing Tests:**
```bash
# Search for tests calling GetByID without creating the record first
grep -r "GetByID(" backend/**/*_test.go
```

---

## Root Cause Analysis

### Why Were Tests Passing Before?

**Encryption Key Warnings:**
- Tests have ALWAYS printed these warnings (not a recent regression)
- Warnings are to stderr, don't fail tests
- This is "noise" that should be cleaned up

**ProxyHost Errors:**
- **Likely Recent Change:**
  - A test was recently modified to call `GetByID()` without proper setup
  - A global test fixture/seed was removed
  - Test database setup order changed
- **Verification Needed:** Check recent commits to `*_test.go` files

### CI vs. Local Test Differences

**CI Environment (`codecov-upload.yml`):**
- No environment variables set beyond `CGO_ENABLED=1`
- Fresh test database for each test run
- No `.env` file loaded

**Local Environment:**
- May have `.env` file with `CHARON_ENCRYPTION_KEY` set
- Test setup may differ from CI
- Local runs might have different test execution order

**Key Files Checked:**
- `.env.example` - Shows `CHARON_ENCRYPTION_KEY=` (empty, requires generation)
- `scripts/go-test-coverage.sh` - Does NOT set `CHARON_ENCRYPTION_KEY`
- `scripts/setup-e2e-env.sh` - Generates key for E2E tests (NOT unit tests)

---

## Remediation Plan

### Phase 1: Environment Variable Configuration (WARNING ELIMINATION)

**Objective:** Eliminate RotationService initialization warnings in CI logs across ALL affected workflows

#### Implementation Strategy

**Single Secret for All Workflows:**
- Use one GitHub Secret: `CHARON_ENCRYPTION_KEY_TEST`
- Apply to all 3 workflows consistently
- Same security model across all test runs

#### Option A: Set in GitHub Actions (RECOMMENDED)
**Security:** Use GitHub Repository Secrets for production-like CI

**Implementation:**

1. **Generate Test Key:**
   ```bash
   # Local execution to generate key
   openssl rand -base64 32
   ```

2. **Add to GitHub Secrets:**
   - Navigate to: Repository → Settings → Secrets → Actions
   - Create new secret: `CHARON_ENCRYPTION_KEY_TEST`
   - Value: Generated base64 key from step 1

3. **Update ALL 3 Workflows:**

   **Workflow 1: codecov-upload.yml**
   **File:** `.github/workflows/codecov-upload.yml`
   **Location:** Line 53-60 (backend-codecov job, "Run Go tests with coverage" step)

   ```yaml
   - name: Run Go tests with coverage
     working-directory: ${{ github.workspace }}
     env:
       CGO_ENABLED: 1
       CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}  # ADD THIS LINE
     run: |
       bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt
       exit "${PIPESTATUS[0]}"
   ```

   **Workflow 2: quality-checks.yml (Test Coverage Step)**
   **File:** `.github/workflows/quality-checks.yml`
   **Location:** Line 37-45 (backend-quality job, "Run Go tests" step)

   ```yaml
   - name: Run Go tests
     id: go-tests
     working-directory: ${{ github.workspace }}
     env:
       CGO_ENABLED: 1
       CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}  # ADD THIS LINE
     run: |
       bash "scripts/go-test-coverage.sh" 2>&1 | tee backend/test-output.txt
       exit "${PIPESTATUS[0]}"
   ```

   **Workflow 2: quality-checks.yml (Perf Tests Step)**
   **File:** `.github/workflows/quality-checks.yml`
   **Location:** Line 115-124 (backend-quality job, "Run Perf Asserts" step)

   ```yaml
   - name: Run Perf Asserts
     working-directory: backend
     env:
       # Conservative defaults to avoid flakiness on CI; tune as necessary
       PERF_MAX_MS_GETSTATUS_P95: 500ms
       PERF_MAX_MS_GETSTATUS_P95_PARALLEL: 1500ms
       PERF_MAX_MS_LISTDECISIONS_P95: 2000ms
       CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}  # ADD THIS LINE
     run: |
       {
         echo "## 🔍 Running performance assertions (TestPerf)"
         go test -run TestPerf -v ./internal/api/handlers -count=1 | tee perf-output.txt
       } >> "$GITHUB_STEP_SUMMARY"
       exit "${PIPESTATUS[0]}"
   ```

   **Workflow 3: benchmark.yml (Benchmark Step)**
   **File:** `.github/workflows/benchmark.yml`
   **Location:** Line 44 (benchmark job, "Run Benchmark" step)

   ```yaml
   - name: Run Benchmark
     working-directory: backend
     env:
       CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}  # ADD THIS LINE
     run: go test -bench=. -benchmem -run='^$' ./... | tee output.txt
   ```

   **Workflow 3: benchmark.yml (Perf Asserts Step)**
   **File:** `.github/workflows/benchmark.yml`
   **Location:** Line 74 (benchmark job, "Run Perf Asserts" step)

   ```yaml
   - name: Run Perf Asserts
     working-directory: backend
     env:
       PERF_MAX_MS_GETSTATUS_P95: 500ms
       PERF_MAX_MS_GETSTATUS_P95_PARALLEL: 1500ms
       PERF_MAX_MS_LISTDECISIONS_P95: 2000ms
       CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}  # ADD THIS LINE
     run: |
       echo "## 🔍 Running performance assertions (TestPerf)" >> "$GITHUB_STEP_SUMMARY"
       go test -run TestPerf -v ./internal/api/handlers -count=1 | tee perf-output.txt
       exit "${PIPESTATUS[0]}"
   ```

**Summary of Changes:**
- **3 workflow files** to modify
- **5 env sections** to update (2 in quality-checks, 2 in benchmark, 1 in codecov-upload)
- **1 GitHub Secret** to create

**Pros:**
- Secrets are encrypted at rest
- Key never appears in logs
- Matches production security model
- Consistent across all workflows

**Cons:**
- Requires GitHub repository admin access
- Key rotation requires updating secret (but affects all workflows at once)

#### Option B: Generate Ephemeral Key (ALTERNATIVE)
**Security:** Generate temporary key for each CI run

**Implementation:**

Apply this pattern to all 3 workflows. Each workflow generates its own ephemeral key.

**Workflow 1: codecov-upload.yml**
**File:** `.github/workflows/codecov-upload.yml`
**Location:** Before "Run Go tests with coverage" step (after "Set up Go")

```yaml
- name: Generate test encryption key
  id: test-key
  run: |
    TEST_KEY=$(openssl rand -base64 32)
    echo "::add-mask::${TEST_KEY}"
    echo "CHARON_ENCRYPTION_KEY=${TEST_KEY}" >> $GITHUB_ENV

- name: Run Go tests with coverage
  working-directory: ${{ github.workspace }}
  env:
    CGO_ENABLED: 1
    # CHARON_ENCRYPTION_KEY inherited from $GITHUB_ENV
  run: |
    bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt
    exit "${PIPESTATUS[0]}"
```

**Workflow 2: quality-checks.yml**
**File:** `.github/workflows/quality-checks.yml`
**Location:** Before "Run Go tests" step (after "Repo health check")

```yaml
- name: Generate test encryption key
  id: test-key
  run: |
    TEST_KEY=$(openssl rand -base64 32)
    echo "::add-mask::${TEST_KEY}"
    echo "CHARON_ENCRYPTION_KEY=${TEST_KEY}" >> $GITHUB_ENV

- name: Run Go tests
  id: go-tests
  working-directory: ${{ github.workspace }}
  env:
    CGO_ENABLED: 1
    # CHARON_ENCRYPTION_KEY inherited from $GITHUB_ENV
  run: |
    bash "scripts/go-test-coverage.sh" 2>&1 | tee backend/test-output.txt
    exit "${PIPESTATUS[0]}"

# ... later in the same job ...

- name: Run Perf Asserts
  working-directory: backend
  env:
    PERF_MAX_MS_GETSTATUS_P95: 500ms
    PERF_MAX_MS_GETSTATUS_P95_PARALLEL: 1500ms
    PERF_MAX_MS_LISTDECISIONS_P95: 2000ms
    # CHARON_ENCRYPTION_KEY inherited from $GITHUB_ENV
  run: |
    {
      echo "## 🔍 Running performance assertions (TestPerf)"
      go test -run TestPerf -v ./internal/api/handlers -count=1 | tee perf-output.txt
    } >> "$GITHUB_STEP_SUMMARY"
    exit "${PIPESTATUS[0]}"
```

**Workflow 3: benchmark.yml**
**File:** `.github/workflows/benchmark.yml`
**Location:** Before "Run Benchmark" step (after "Set up Go")

```yaml
- name: Generate test encryption key
  id: test-key
  run: |
    TEST_KEY=$(openssl rand -base64 32)
    echo "::add-mask::${TEST_KEY}"
    echo "CHARON_ENCRYPTION_KEY=${TEST_KEY}" >> $GITHUB_ENV

- name: Run Benchmark
  working-directory: backend
  env:
    # CHARON_ENCRYPTION_KEY inherited from $GITHUB_ENV
  run: go test -bench=. -benchmem -run='^$' ./... | tee output.txt

# ... later in the same job ...

- name: Run Perf Asserts
  working-directory: backend
  env:
    PERF_MAX_MS_GETSTATUS_P95: 500ms
    PERF_MAX_MS_GETSTATUS_P95_PARALLEL: 1500ms
    PERF_MAX_MS_LISTDECISIONS_P95: 2000ms
    # CHARON_ENCRYPTION_KEY inherited from $GITHUB_ENV
  run: |
    echo "## 🔍 Running performance assertions (TestPerf)" >> "$GITHUB_STEP_SUMMARY"
    go test -run TestPerf -v ./internal/api/handlers -count=1 | tee perf-output.txt
    exit "${PIPESTATUS[0]}"
```

**Pros:**
- No secrets management needed
- Key is ephemeral (discarded after run)
- Simpler to implement
- Each workflow run gets its own unique key

**Cons:**
- Generates new key on every run (minimal overhead ~0.1s)
- Doesn't test key persistence scenarios

#### Option C: Inline Test Key (NOT RECOMMENDED)
**Security:** Hardcode a test-only key in workflow

**Implementation:**

Apply same hardcoded key to all 3 workflows:

```yaml
- name: Run Go tests with coverage  # or Run Benchmark, or Run Perf Asserts
  working-directory: ${{ github.workspace }}
  env:
    CGO_ENABLED: 1
    CHARON_ENCRYPTION_KEY: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="  # Hardcoded test key
  run: |
    bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt
    exit "${PIPESTATUS[0]}"
```

**Apply to:**
- `.github/workflows/codecov-upload.yml` - Line 53 env block
- `.github/workflows/quality-checks.yml` - Lines 37 and 115 env blocks
- `.github/workflows/benchmark.yml` - Lines 44 and 74 env blocks

**Pros:**
- Simplest to implement (just add one line per env block)
- No secrets management
- No key generation overhead

**Cons:**
- ⚠️ Key visible in workflow file and logs
- ⚠️ Security audit will flag this
- ⚠️ Doesn't test real key loading from environment
- ⚠️ Not recommended for repos with security compliance requirements

**Recommendation:** Use **Option A** (GitHub Secrets) for production readiness and security compliance, or **Option B** (Ephemeral) for simplicity without security concerns. Avoid Option C unless this is a demo/test repository.

---

### Phase 2: Database Seeding/Test Setup (ERROR ELIMINATION)

**Objective:** Fix ProxyHost "record not found" failures

#### Step 1: Identify Failing Tests

**Action:** Run tests locally and capture failures

```bash
cd backend
go test -v ./... 2>&1 | tee test-output.txt
grep -i "record not found" test-output.txt
```

**Expected Output:**
```
--- FAIL: TestSomeFunction (0.00s)
    service_test.go:123: Error getting proxy host: record not found
```

#### Step 2: Classify Failures

For each failing test, determine:

1. **Test calls `GetByID()` without creating record?**
   - Fix: Add `createTestProxyHost()` call before `GetByID()`

2. **Test expects a specific ID (e.g., ID=1)?**
   - Fix: Store the returned ID from `Create()` and use it in `GetByID()`

3. **Test relies on global seed data?**
   - Fix: Add explicit test data creation in test setup

#### Step 3: Apply Fixes

**Pattern 1: Missing Test Data Creation**

**Before (Broken):**
```go
func TestSomeFunction(t *testing.T) {
	db := setupTestDB(t)
	service := NewProxyHostService(db)

	// Assumes ID=1 exists (WRONG)
	host, err := service.GetByID(1)
	require.NoError(t, err)
}
```

**After (Fixed):**
```go
func TestSomeFunction(t *testing.T) {
	db := setupTestDB(t)
	service := NewProxyHostService(db)

	// Create test data first
	testHost := &models.ProxyHost{
		UUID:        "test-uuid",
		DomainNames: "test.example.com",
		ForwardHost: "localhost",
		ForwardPort: 8080,
	}
	require.NoError(t, service.Create(testHost))

	// Now fetch it by the auto-assigned ID
	host, err := service.GetByID(testHost.ID)
	require.NoError(t, err)
	assert.Equal(t, "test.example.com", host.DomainNames)
}
```

**Pattern 2: Expecting Specific Error**

**Option A: Handle gorm.ErrRecordNotFound**
```go
func TestGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewProxyHostService(db)

	// Test error handling for non-existent ID
	_, err := service.GetByID(999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
```

**Option B: Wrap Error in Service (BETTER)**

Modify `ProxyHostService.GetByID()` to return a domain-specific error:

**File:** `backend/internal/services/proxyhost_service.go:192-197`

```go
var ErrProxyHostNotFound = errors.New("proxy host not found")

func (s *ProxyHostService) GetByID(id uint) (*models.ProxyHost, error) {
	var host models.ProxyHost
	if err := s.db.Where("id = ?", id).First(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProxyHostNotFound
		}
		return nil, err
	}
	return &host, nil
}
```

**Then tests become:**
```go
func TestGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewProxyHostService(db)

	_, err := service.GetByID(999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrProxyHostNotFound))
}
```

#### Step 4: Add Missing Test Utilities

**Create Shared Test Helper:**

**File:** `backend/internal/services/testutil/proxyhost_fixtures.go` (NEW FILE)

```go
package testutil

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// CreateTestProxyHost creates a proxy host with sensible defaults for testing.
func CreateTestProxyHost(t *testing.T, db *gorm.DB, overrides ...func(*models.ProxyHost)) *models.ProxyHost {
	t.Helper()

	host := &models.ProxyHost{
		UUID:          uuid.NewString(),
		Name:          "Test Proxy",
		DomainNames:   "test.example.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}

	// Apply overrides
	for _, override := range overrides {
		override(host)
	}

	require.NoError(t, db.Create(host).Error)
	return host
}
```

**Usage in Tests:**
```go
import "github.com/Wikid82/charon/backend/internal/services/testutil"

func TestSomeFunction(t *testing.T) {
	db := setupTestDB(t)
	service := NewProxyHostService(db)

	// Create test data with defaults
	host1 := testutil.CreateTestProxyHost(t, db)

	// Create test data with custom values
	host2 := testutil.CreateTestProxyHost(t, db, func(h *models.ProxyHost) {
		h.Name = "Custom Name"
		h.ForwardPort = 9000
	})

	// Now use them
	fetched, err := service.GetByID(host1.ID)
	require.NoError(t, err)
}
```

---

## Phase 3: Validation

### Consolidated Implementation Checklist

**Phase 1: Multi-Workflow Environment Variable Fix**

- [ ] **Generate or configure secret:**
  - Option A: Generate key with `openssl rand -base64 32`, add to GitHub Secrets as `CHARON_ENCRYPTION_KEY_TEST`
  - Option B: Add key generation step to each workflow (ephemeral keys)
  - Option C: Use hardcoded test key (not recommended)

- [ ] **Update Workflow 1 (Priority: CRITICAL):**
  - **File:** `.github/workflows/quality-checks.yml`
  - **Location 1:** Line 37-45 - Add `CHARON_ENCRYPTION_KEY` to "Run Go tests" step
  - **Location 2:** Line 115-124 - Add `CHARON_ENCRYPTION_KEY` to "Run Perf Asserts" step
  - **Verification:** Both test steps have the env var

- [ ] **Update Workflow 2 (Priority: HIGH):**
  - **File:** `.github/workflows/codecov-upload.yml`
  - **Location:** Line 53-60 - Add `CHARON_ENCRYPTION_KEY` to "Run Go tests with coverage" step
  - **Verification:** Test step has the env var

- [ ] **Update Workflow 3 (Priority: MEDIUM):**
  - **File:** `.github/workflows/benchmark.yml`
  - **Location 1:** Line 44 - Add `CHARON_ENCRYPTION_KEY` to "Run Benchmark" step
  - **Location 2:** Line 74 - Add `CHARON_ENCRYPTION_KEY` to "Run Perf Asserts" step
  - **Verification:** Both test steps have the env var

- [ ] **Total changes:** 3 files, 5 env blocks updated

**Phase 2: Test Data Setup Fixes**

- [ ] Identify failing tests with "record not found" errors
- [ ] Fix each test by adding proper test data creation
- [ ] Add `testutil.CreateTestProxyHost()` helper if needed
- [ ] Verify all tests pass locally

**Phase 3: Multi-Workflow Validation**

- [ ] Local validation (all tests pass with encryption key set)
- [ ] Push to feature branch
- [ ] Monitor **all 3 workflow runs** in GitHub Actions
- [ ] Verify each workflow:
  - ✅ quality-checks.yml - No warnings, tests pass
  - ✅ codecov-upload.yml - No warnings, tests pass, coverage uploaded
  - ✅ benchmark.yml - No warnings, benchmarks complete

---

## Phase 3: Validation (Detailed Procedures)

### Step 1: Local Validation

**Execute Before Pushing:**

```bash
# 1. Set encryption key locally (matches CI)
export CHARON_ENCRYPTION_KEY=$(openssl rand -base64 32)

# 2. Run backend tests
cd /projects/Charon
.github/skills/scripts/skill-runner.sh test-backend-coverage

# 3. Verify no warnings in output
# Look for: "Warning: RotationService initialization failed"
# Expected: No warnings

# 4. Verify coverage pass
# Expected: "Coverage requirement met"

# 5. Check for test failures
# Expected: All tests pass
```

**Success Criteria:**
- ✅ No "RotationService initialization failed" warnings
- ✅ No "record not found" errors
- ✅ Coverage >= 85%
- ✅ All tests pass

### Step 2: CI Validation

**Push to Branch and Monitor:**

```bash
git checkout -b fix/ci-backend-test-failures
git add .github/workflows/codecov-upload.yml
git add .github/workflows/quality-checks.yml
git add .github/workflows/benchmark.yml
git add backend/internal/services/proxyhost_service.go  # If modified
git add backend/internal/services/*_test.go  # Any test fixes
git commit -m "fix(ci): resolve backend test failures across all workflows

- Add CHARON_ENCRYPTION_KEY to quality-checks, codecov-upload, and benchmark workflows
- Fix ProxyHost test data setup in service tests
- Eliminate RotationService initialization warnings

Affected workflows:
- quality-checks.yml (CRITICAL: PR blocker)
- codecov-upload.yml (HIGH: coverage tracking)
- benchmark.yml (MEDIUM: performance regression)

Resolves: backend test job failures across 3 CI workflows"
git push origin fix/ci-backend-test-failures
```

**Monitor All 3 CI Workflows:**

1. Navigate to GitHub Actions → Your PR
2. Verify these workflow runs appear:
   - ✅ **Quality Checks** (most critical)
   - ✅ **Upload Coverage to Codecov**
   - ✅ **Go Benchmark** (may run later via workflow_run trigger)

3. **For each workflow, verify:**
   - No stderr warnings in test execution steps
   - Test output shows all tests passing
   - No "RotationService initialization failed" messages
   - No "record not found" errors

4. **Quality Checks specific checks:**
   - "Run Go tests" step succeeds
   - "Run Perf Asserts" step succeeds
   - GORM Security Scanner passes
   - Frontend tests pass (unrelated but monitored)

5. **Codecov Upload specific checks:**
   - Backend tests pass
   - Coverage upload succeeds
   - Coverage report appears on PR

6. **Benchmark specific checks:**
   - Benchmarks complete without errors
   - Performance assertions pass
   - (Note: Results may only store on main branch pushes)

**Expected Duration:**
- quality-checks.yml: ~3-5 minutes
- codecov-upload.yml: ~3-5 minutes
- benchmark.yml: ~4-6 minutes

**Success Criteria - ALL workflows must:**
- ✅ Complete without failures
- ✅ Show no encryption key warnings
- ✅ Show no database record errors
- ✅ Maintain or improve coverage/performance baselines

---

## Dependencies & Risks

### Dependencies

**Internal:**
- GitHub repository secrets access (for Option A)
- Ability to modify 3 workflow files: `.github/workflows/{codecov-upload,quality-checks,benchmark}.yml`
- Go test environment (local and CI)

**External:**
- Codecov service (for coverage upload)
- GitHub Actions runner availability

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Tests fail after adding encryption key | Low | Medium | Test locally first with same env var |
| New test failures introduced by fixes | Medium | Medium | Validate each test fix individually |
| Coverage drops below 85% | Low | High | Add tests alongside fixes, not after |
| Codecov upload still fails | Low | High | Verify Codecov token is valid |
| Breaking other tests by modifying ProxyHostService | Low | High | Only add error wrapping, don't change logic |
| **Missing affected workflows (incomplete fix)** | **Low** | **Critical** | Verified all workflows via grep search; only 3 run Go tests |
| **Workflow fixes out of sync** | **Medium** | **High** | Use same env var name (`CHARON_ENCRYPTION_KEY`) across all workflows |
| **Quality checks workflow more critical than codecov** | **N/A** | **Critical** | Prioritize quality-checks.yml - it blocks PR merges |
| **Benchmark workflow fails silently** | **Low** | **Medium** | Add same fix proactively even if not currently failing |

### Multi-Workflow Coordination

**Critical Insight:** The `quality-checks.yml` workflow is MORE important than `codecov-upload.yml` because:
- Quality checks run on every PR and block merges
- Codecov upload is informational and doesn't block merges
- Quality checks includes multiple test types (unit tests + perf tests)

**Implementation Priority:**
1. **FIRST:** Fix `quality-checks.yml` (most critical - PR blocker)
2. **SECOND:** Fix `codecov-upload.yml` (high priority - coverage tracking)
3. **THIRD:** Fix `benchmark.yml` (proactive - prevent future issues)

**Consistency Requirements:**
- All workflows MUST use the same environment variable name: `CHARON_ENCRYPTION_KEY`
- If using Option A (GitHub Secrets), all workflows MUST reference the same secret: `CHARON_ENCRYPTION_KEY_TEST`
- If using Option B (Ephemeral), all workflows MUST generate keys the same way for consistency

### Technical Debt Created

1. **Test Helper Utilities:**
   - New `testutil` package should be documented
   - Consider creating similar helpers for other models

2. **Error Handling Consistency:**
   - If wrapping `gorm.ErrRecordNotFound`, apply same pattern to all services
   - Document error handling conventions

3. **Environment Variable Documentation:**
   - Update `docs/development.md` with required CI env vars
   - Document test key generation process

---

## Stop/Go Rules

### Stop Conditions

**Phase 1 (Environment Variables):**
- STOP if: Local tests fail after setting `CHARON_ENCRYPTION_KEY`
  - **Action:** Investigate why encryption key breaks tests
  - **Escalate to:** Backend service owners

**Phase 2 (Test Fixes):**
- STOP if: More than 5 test files need modifications
  - **Action:** Consider global test fixture/seed instead
  - **Escalate to:** Test infrastructure team
- STOP if: Fixing tests requires production code changes beyond error wrapping
  - **Action:** Escalate as potential design issue

**Phase 3 (Validation):**
- STOP if: CI still fails after local validation passes
  - **Action:** Compare CI environment vs. local (Go version, SQLite version, etc.)
  - **Escalate to:** DevOps/CI team

### Go Conditions

**Phase 1 → Phase 2:**
- GO if: Tests run with no RotationService warnings
- GO if: Coverage remains >= 85%

**Phase 2 → Phase 3:**
- GO if: All identified test failures are fixed
- GO if: No new test failures introduced

**Phase 3 → Complete:**
- GO if: CI run passes with all checks green
- GO if: Codecov upload succeeds

---

## Success Metrics

### Quantitative

1. **RotationService Warnings:** 0 occurrences in CI logs
2. **Test Failures:** 0 "record not found" errors
3. **Coverage:** Maintain >= 85% backend coverage
4. **CI Duration:** No increase in test execution time
5. **Test Pass Rate:** 100% (all tests pass)

### Qualitative

1. **Code Quality:** Test fixes follow established patterns
2. **Documentation:** Changes are self-explanatory or documented
3. **Maintainability:** Future tests can easily create test data
4. **Security:** Encryption key handling follows best practices

---

## Timeline Estimate

| Phase | Estimated Duration | Confidence |
|-------|-------------------|-----------|
| Phase 1: Environment Variable (3 workflows) | 45 minutes | High |
| Phase 2: Test Fixes | 1-3 hours | Medium |
| Phase 3: Validation (3 workflows) | 45 minutes | High |
| **Total** | **2.5-4.5 hours** | **Medium** |

**Assumptions:**
- Fewer than 5 tests need fixing
- No production code changes required (beyond error wrapping)
- CI environment is stable
- All 3 workflows can be tested in parallel

**Phase 1 Breakdown:**
- Generate/configure secret: 5 minutes
- Update quality-checks.yml (2 env blocks): 15 minutes
- Update codecov-upload.yml (1 env block): 10 minutes
- Update benchmark.yml (2 env blocks): 10 minutes
- Document changes and verify: 5 minutes

**Contingency:**
- If more than 5 tests fail: +2 hours
- If production code needs refactoring: +4 hours
- If CI environment has additional issues: +1 hour
- If workflows have unexpected dependencies: +1 hour

---

## Follow-Up Actions

### Immediate (This PR)

1. ✅ Add `CHARON_ENCRYPTION_KEY` to CI workflow
2. ✅ Fix all identified test failures
3. ✅ Verify CI passes

### Short-Term (Next Sprint)

1. **Test Infrastructure Audit:**
   - Document all required environment variables for tests
   - Create standardized test setup utilities (`testutil` package)
   - Add linting rule to catch missing test data setup

2. **Error Handling Standardization:**
   - Define domain-specific errors for all services (not just ProxyHost)
   - Document error handling conventions
   - Apply pattern to all `*Service.GetByID()` methods

3. **CI Environment Documentation:**
   - Document all GitHub Secrets required for workflows
   - Create key rotation procedure
   - Add CI environment variable checklist

### Long-Term (Future)

1. **Test Fixture Framework:**
   - Evaluate using `testfixtures` or similar library
   - Create declarative test data setup
   - Reduce boilerplate in test files

2. **Integration Testing:**
   - Separate unit tests (fast, mocked) from integration tests (real DB)
   - Use build tags: `//go:build integration`
   - Run integration tests separately in CI

3. **Service Constructor Refactoring:**
   - Make `RotationService` initialization explicit
   - Allow tests to inject mock `RotationService`
   - Reduce warning messages in test output

---

## References

### Files Analyzed

**CI Configuration:**
- `.github/workflows/codecov-upload.yml` (workflow definition)

**Backend Services:**
- `backend/internal/crypto/rotation_service.go` (encryption key loading)
- `backend/internal/services/dns_provider_service.go` (RotationService usage)
- `backend/internal/services/credential_service.go` (RotationService usage)
- `backend/internal/services/proxyhost_service.go` (GetByID implementation)

**Tests:**
- `backend/internal/crypto/rotation_service_test.go` (key setup pattern)
- `backend/internal/services/dns_provider_service_test.go` (test setup)
- `backend/internal/services/credential_service_test.go` (test setup)
- `backend/internal/services/proxyhost_service_test.go` (CRUD test pattern)
- `backend/internal/api/handlers/proxy_host_handler_update_test.go` (test helper)

**Documentation:**
- `.env.example` (environment variable reference)
- `ARCHITECTURE.md` (encryption key documentation)
- `docs/guides/dns-providers.md` (encryption key usage guide)

### External Resources

- [GORM Error Handling](https://gorm.io/docs/error_handling.html)
- [GitHub Actions Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- [Go Testing Best Practices](https://go.dev/doc/effective_go#testing)
- [SQLite In-Memory Databases](https://www.sqlite.org/inmemorydb.html)

---

## Appendix A: Workflow Analysis Details

### Analysis Methodology

**Search Commands Used:**
```bash
# Find all workflow files
find .github/workflows -name "*.yml"

# Find workflows running Go tests
grep -r "go test\|go-test-coverage\.sh" .github/workflows/*.yml

# Find workflows with encryption key
grep -r "CHARON_ENCRYPTION_KEY" .github/workflows/*.yml
```

**Results:**
- **39 total workflow files** in `.github/workflows/`
- **3 workflows run Go unit tests** (affected by missing encryption key)
- **1 workflow (e2e-tests-split.yml)** already has encryption key configured
- **2 workflows (cerberus, crowdsec)** run integration tests (not affected)
- **33 workflows** don't run backend tests (not affected)

### Workflow-by-Workflow Breakdown

#### 1. quality-checks.yml (CRITICAL)
**Purpose:** PR quality gates that block merges
**Trigger:** On every pull_request to main/development
**Impact:** Most critical - blocks PR approvals
**Test Commands:**
- Line 43: `bash "scripts/go-test-coverage.sh"`
- Line 123: `go test -run TestPerf -v ./internal/api/handlers`

**Current Status:** ❌ Failing
**Fix Required:** Add `CHARON_ENCRYPTION_KEY` to both test steps
**Expected Result:** PR checks turn green, allowing merges

#### 2. codecov-upload.yml (HIGH PRIORITY)
**Purpose:** Upload test coverage to Codecov service
**Trigger:** On pull_request to main/development + workflow_dispatch
**Impact:** High - coverage tracking and reporting
**Test Commands:**
- Line 58: `bash scripts/go-test-coverage.sh`

**Current Status:** ❌ Failing
**Fix Required:** Add `CHARON_ENCRYPTION_KEY` to test step
**Expected Result:** Coverage reports appear on PRs

#### 3. benchmark.yml (MEDIUM PRIORITY)
**Purpose:** Performance regression detection
**Trigger:** After docker-build.yml completes + workflow_dispatch
**Impact:** Medium - catches performance regressions
**Test Commands:**
- Line 44: `go test -bench=. -benchmem -run='^$' ./...`
- Line 74: `go test -run TestPerf -v ./internal/api/handlers`

**Current Status:** ⚠️ At risk (may not have failed yet)
**Fix Required:** Add `CHARON_ENCRYPTION_KEY` to both test steps (proactive)
**Expected Result:** Benchmarks run cleanly without warnings

#### 4. e2e-tests-split.yml (ALREADY FIXED)
**Purpose:** End-to-end Playwright tests
**Trigger:** Multiple triggers, runs E2E test shards
**Status:** ✅ Already configured correctly

**Evidence of correct configuration:**
```yaml
# Lines 280, 481, 690, 894, 1098, 1310 - All identical:
- name: Generate test encryption key
  run: echo "CHARON_ENCRYPTION_KEY=$(openssl rand -base64 32)" >> "$GITHUB_ENV"
```

**Why it's correct:** Each shard generates its own ephemeral key before running tests. This is the pattern Option B recommends.

#### 5. cerberus-integration.yml (NOT AFFECTED)
**Purpose:** Cerberus security stack integration tests
**Test Type:** Docker compose with integration scripts
**Why not affected:** Doesn't run `go test` - runs `scripts/cerberus_integration.sh`
**Status:** ✅ No changes needed

#### 6. crowdsec-integration.yml (NOT AFFECTED)
**Purpose:** CrowdSec bouncer integration tests
**Test Type:** Docker compose with integration scripts
**Why not affected:** Doesn't run `go test` - runs skill-based integration scripts
**Status:** ✅ No changes needed

### Why Other Workflows Aren't Affected

**Workflows without backend tests:**
- `docker-build.yml` - Builds images, no test execution
- `codeql.yml` - Security scanning only
- `supply-chain-*.yml` - SBOM and provenance only
- `release-goreleaser.yml` - Release automation
- `docs.yml` - Documentation deployment
- `repo-health.yml` - Repository maintenance
- `renovate_prune.yml` - Dependency management
- `auto-versioning.yml` - Version bumping
- `caddy-major-monitor.yml` - Upstream monitoring
- `update-geolite2.yml` - GeoIP updates
- `nightly-build.yml` - Scheduled builds
- `propagate-changes.yml` - Branch sync
- `weekly-nightly-promotion.yml` - Release promotion
- `gh_cache_cleanup.yml` - Cache maintenance

**Key Insight:** The CI failures only affect workflows that run `go test` commands, and specifically those that instantiate services requiring `RotationService`. Integration test workflows use Docker compose and don't instantiate Go services directly in the CI runner.

---

## Sign-Off

**Prepared by:** Investigation Agent
**Reviewed by:** Pending (Awaiting supervisor approval)
**Approved by:** Pending

**Next Action:** Await approval to proceed with Phase 1 implementation.
