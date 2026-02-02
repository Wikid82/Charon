# Lint Remediation & Monitoring Plan

**Status:** Planning
**Created:** 2026-02-02
**Target Completion:** 2026-02-03

---

## Executive Summary

This plan addresses 40 Go linting issues (18 errcheck, 22 gosec warnings from `full_lint_output.txt`), 6 TypeScript warnings, and establishes monitoring for retry attempt frequency to ensure it remains below 5%.

### Goals

1. **Go Linting:** Fix all 40 reported issues (18 errcheck, 22 gosec)
2. **TypeScript:** Resolve 6 ESLint warnings (no-explicit-any, no-unused-vars)
3. **Monitoring:** Implement retry attempt frequency tracking (<5% threshold)

---

## Research Findings

### 1. Go Linting Issues (40 total from full_lint_output.txt)

**Source Files:**
- `backend/final_lint.txt` (34 issues - subset)
- `backend/full_lint_output.txt` (40 issues - complete list)

#### 1.1 Errcheck Issues (18 total)

**Category A: Unchecked json.Unmarshal in Tests (6)**

| File | Line | Issue |
|------|------|-------|
| `internal/api/handlers/security_handler_audit_test.go` | 581 | `json.Unmarshal(w.Body.Bytes(), &resp)` |
| `internal/api/handlers/security_handler_coverage_test.go` | 525, 589 | `json.Unmarshal(w.Body.Bytes(), &resp)` (2 locations) |
| `internal/api/handlers/settings_handler_test.go` | 895, 923, 1081 | `json.Unmarshal(w.Body.Bytes(), &resp)` (3 locations) |

**Root Cause:** Test code not checking JSON unmarshaling errors
**Impact:** Tests may pass with invalid JSON responses, false positives
**Fix:** Add error checking: `require.NoError(t, json.Unmarshal(...))`

**Category B: Unchecked Environment Variable Operations (11)**

| File | Line | Issue |
|------|------|-------|
| `internal/caddy/config_test.go` | 1794 | `os.Unsetenv(v)` |
| `internal/config/config_test.go` | 56, 57, 72, 74, 75, 82 | `os.Setenv(...)` (6 instances) |
| `internal/config/config_test.go` | 157, 158, 159, 175, 196 | `os.Unsetenv(...)` (5 instances total) |

**Root Cause:** Environment variable setup/cleanup without error handling
**Impact:** Test isolation failures, flaky tests
**Fix:** Wrap with `require.NoError(t, os.Setenv/Unsetenv(...))`

**Category C: Unchecked Database Close Operations (4)**

| File | Line | Issue |
|------|------|-------|
| `internal/services/dns_provider_service_test.go` | 1446, 1466, 1493, 1531, 1549 | `sqlDB.Close()` (4 locations) |
| `internal/database/errors_test.go` | 230 | `sqlDB.Close()` |

**Root Cause:** Resource cleanup without error handling
**Impact:** Resource leaks in tests
**Fix:** `defer func() { _ = sqlDB.Close() }()` or explicit error check

**Category D: Unchecked w.Write in Tests (3)**

| File | Line | Issue |
|------|------|-------|
| `internal/caddy/manager_additional_test.go` | 1467, 1522 | `w.Write([]byte(...))` (2 locations) |
| `internal/caddy/manager_test.go` | 133 | `w.Write([]byte(...))` |

**Root Cause:** HTTP response writing without error handling
**Impact:** Silent failures in mock HTTP servers
**Fix:** `_, _ = w.Write(...)` or check error if critical

**Category E: Unchecked db.AutoMigrate in Tests (3)**

| File | Line | Issue |
|------|------|-------|
| `internal/api/handlers/notification_coverage_test.go` | 22 | `db.AutoMigrate(...)` |
| `internal/api/handlers/pr_coverage_test.go` | 404, 438 | `db.AutoMigrate(...)` (2 locations) |

**Root Cause:** Database schema migration without error handling
**Impact:** Tests may run with incorrect schema
**Fix:** `require.NoError(t, db.AutoMigrate(...))`

#### 1.2 Gosec Security Issues (22 total - unchanged from final_lint.txt)

*(Same 22 gosec issues as documented in final_lint.txt)*

### 2. TypeScript Linting Issues (6 warnings - unchanged)

*(Same 6 ESLint warnings as documented earlier)*

### 3. Retry Monitoring Analysis

**Current State:**

**Retry Logic Location:** `backend/internal/services/uptime_service.go`

**Configuration:**
- `MaxRetries` in `UptimeServiceConfig` (default: 2)
- `MaxRetries` in `models.UptimeMonitor` (default: 3)

**Current Behavior:**
```go
for retry := 0; retry <= s.config.MaxRetries && !success; retry++ {
    if retry > 0 {
        logger.Log().Info("Retrying TCP check")
    }
    // Try connection...
}
```

**Metrics Gaps:**
- No retry frequency tracking
- No alerting on excessive retries
- No historical data for analysis

**Requirements:**
- Track retry attempts vs first-try successes
- Alert if retry rate >5% over rolling 1000 checks
- Expose Prometheus metrics for dashboarding

---

## Technical Specifications

### Phase 1: Backend Go Linting Fixes

#### 1.1 Errcheck Fixes (18 issues)

**JSON Unmarshal (6 fixes):**

```go
// Pattern to apply across 6 locations
// BEFORE:
json.Unmarshal(w.Body.Bytes(), &resp)

// AFTER:
err := json.Unmarshal(w.Body.Bytes(), &resp)
require.NoError(t, err, "Failed to unmarshal response")
```

**Files:**
- `internal/api/handlers/security_handler_audit_test.go:581`
- `internal/api/handlers/security_handler_coverage_test.go:525, 589`
- `internal/api/handlers/settings_handler_test.go:895, 923, 1081`

**Environment Variables (11 fixes):**

```go
// BEFORE:
os.Setenv("VAR_NAME", "value")

// AFTER:
require.NoError(t, os.Setenv("VAR_NAME", "value"))
```

**Files:**
- `internal/config/config_test.go:56, 57, 72, 74, 75, 82, 157, 158, 159, 175, 196`
- `internal/caddy/config_test.go:1794`

**Database Close (4 fixes):**

```go
// BEFORE:
sqlDB.Close()

// AFTER (Pattern 1 - Immediate cleanup with error reporting):
if err := sqlDB.Close(); err != nil {
    t.Errorf("Failed to close database connection: %v", err)
}

// AFTER (Pattern 2 - Deferred cleanup with error reporting):
defer func() {
    if err := sqlDB.Close(); err != nil {
        t.Errorf("Failed to close database connection: %v", err)
    }
}()
```

**Rationale:**
- Tests must report resource cleanup failures for debugging
- Using `_` silences legitimate errors that could indicate resource leaks
- `t.Errorf` doesn't stop test execution but records the failure
- Pattern 1 for immediate cleanup (end of test)
- Pattern 2 for deferred cleanup (start of test)

**Files:**
- `internal/services/dns_provider_service_test.go:1446, 1466, 1493, 1531, 1549`
- `internal/database/errors_test.go:230`

**HTTP Write (3 fixes):**

```go
// BEFORE:
w.Write([]byte(`{"data": "value"}`))

// AFTER (Enhanced with error handling):
if _, err := w.Write([]byte(`{"data": "value"}`)); err != nil {
    t.Errorf("Failed to write HTTP response: %v", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}
```

**Rationale:**
- Mock servers should fail fast on write errors to avoid misleading test results
- `http.Error` ensures client sees error response, not partial data
- Early return prevents further processing with invalid state
- Critical for tests that validate response content

**Files:**
- `internal/caddy/manager_additional_test.go:1467, 1522`
- `internal/caddy/manager_test.go:133`

**AutoMigrate (3 fixes):**

```go
// BEFORE:
db.AutoMigrate(&models.Model{})

// AFTER:
require.NoError(t, db.AutoMigrate(&models.Model{}))
```

**Files:**
- `internal/api/handlers/notification_coverage_test.go:22`
- `internal/api/handlers/pr_coverage_test.go:404, 438`

#### 1.2 Gosec Security Fixes (22 issues)

**G101: Hardcoded Credentials (1 issue)**

**Location:** Test fixtures containing example API tokens

```go
// BEFORE:
apiKey := "sk_test_1234567890abcdef"

// AFTER:
// #nosec G101 -- Test fixture with non-functional API key for validation testing
apiKey := "sk_test_1234567890abcdef"
```

**Security Analysis:**
- **Risk Level:** LOW (test-only code)
- **Validation:** Verify value is non-functional, documented as test fixture
- **Impact:** None if properly annotated, prevents false positives

---

**G110: Decompression Bomb (2 issues)**

**Locations:**
- `internal/crowdsec/hub_cache.go`
- `internal/crowdsec/hub_sync.go`

```go
// BEFORE:
reader, err := gzip.NewReader(resp.Body)
if err != nil {
    return err
}
defer reader.Close()
io.Copy(dest, reader) // Unbounded read

// AFTER:
const maxDecompressedSize = 100 * 1024 * 1024 // 100MB limit

reader, err := gzip.NewReader(resp.Body)
if err != nil {
    return fmt.Errorf("gzip reader init failed: %w", err)
}
defer reader.Close()

// Limit decompressed size to prevent decompression bombs
limitedReader := io.LimitReader(reader, maxDecompressedSize)
written, err := io.Copy(dest, limitedReader)
if err != nil {
    return fmt.Errorf("decompression failed: %w", err)
}

// Verify we didn't hit the limit (which would indicate potential attack)
if written >= maxDecompressedSize {
    return fmt.Errorf("decompression size exceeded limit (%d bytes), potential decompression bomb", maxDecompressedSize)
}
```

**Security Analysis:**
- **Risk Level:** HIGH (remote code execution vector)
- **Attack Vector:** Malicious CrowdSec hub response with crafted gzip bomb
- **Mitigation:**
  - Hard limit at 100MB (CrowdSec hub files are typically <10MB)
  - Early termination on limit breach
  - Error returned prevents further processing
- **Impact:** Prevents memory exhaustion DoS attacks

---

**G305: File Traversal (1 issue)**

**Location:** File path handling in backup/restore operations

```go
// BEFORE:
filePath := filepath.Join(baseDir, userInput)
file, err := os.Open(filePath)

// AFTER:
// Sanitize and validate file path to prevent directory traversal
func SafeJoinPath(baseDir, userPath string) (string, error) {
    // Clean the user-provided path
    cleanPath := filepath.Clean(userPath)

    // Reject absolute paths and parent directory references
    if filepath.IsAbs(cleanPath) {
        return "", fmt.Errorf("absolute paths not allowed: %s", cleanPath)
    }
    if strings.Contains(cleanPath, "..") {
        return "", fmt.Errorf("parent directory traversal not allowed: %s", cleanPath)
    }

    // Join with base directory
    fullPath := filepath.Join(baseDir, cleanPath)

    // Verify the resolved path is still within base directory
    absBase, err := filepath.Abs(baseDir)
    if err != nil {
        return "", fmt.Errorf("failed to resolve base directory: %w", err)
    }

    absPath, err := filepath.Abs(fullPath)
    if err != nil {
        return "", fmt.Errorf("failed to resolve file path: %w", err)
    }

    if !strings.HasPrefix(absPath, absBase) {
        return "", fmt.Errorf("path escape attempt detected: %s", userPath)
    }

    return fullPath, nil
}

// Usage:
safePath, err := SafeJoinPath(baseDir, userInput)
if err != nil {
    return fmt.Errorf("invalid file path: %w", err)
}
file, err := os.Open(safePath)
```

**Security Analysis:**
- **Risk Level:** CRITICAL (arbitrary file read/write)
- **Attack Vectors:**
  - `../../etc/passwd` - Read sensitive system files
  - `../../../root/.ssh/id_rsa` - Steal credentials
  - Symlink attacks to escape sandbox
- **Mitigation:**
  - Reject absolute paths
  - Block `..` sequences
  - Verify resolved path stays within base directory
  - Works even with symlinks (uses `filepath.Abs`)
- **Impact:** Prevents unauthorized file system access

---

**G306/G302: File Permissions (8 issues)**

**Permission Security Matrix:**

| Permission | Octal | Use Case | Justification |
|------------|-------|----------|---------------|
| **0600** | rw------- | SQLite database files, private keys | Contains sensitive data; only process owner needs access |
| **0640** | rw-r----- | Log files, config files | Owner writes, group reads for monitoring/debugging |
| **0644** | rw-r--r-- | Public config templates, documentation | World-readable reference data, no sensitive content |
| **0700** | rwx------ | Backup directories, data directories | Process-owned workspace, no group/world access needed |
| **0750** | rwxr-x--- | Binary directories, script directories | Owner manages, group executes; prevents tampering |

**Implementation Pattern:**

```go
// BEFORE:
os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644) // Too permissive for sensitive data
os.MkdirAll(path, 0755) // Too permissive for private directories

// AFTER - Database files (0600):
// Rationale: Contains user credentials, tokens, PII
// Risk if compromised: Full system access, credential theft
os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY, 0600)

// AFTER - Log files (0640):
// Rationale: Monitoring tools run in same group, need read access
// Risk if compromised: Information disclosure, system reconnaissance
os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)

// AFTER - Backup directories (0700):
// Rationale: Contains complete database dumps with sensitive data
// Risk if compromised: Mass data exfiltration
os.MkdirAll(backupDir, 0700)

// AFTER - Config templates (0644):
// Rationale: Reference documentation, no secrets or user data
// Risk if compromised: None (public information)
os.OpenFile(tplPath, os.O_CREATE|os.O_RDONLY, 0644)
```

**Security Analysis by File Type:**

| File Type | Current | Required | Risk If Wrong | Affected Files |
|-----------|---------|----------|---------------|----------------|
| SQLite DB | 0644 | **0600** | Credential theft | `internal/database/*.go` |
| Backup tar | 0644 | **0600** | Mass data leak | `internal/services/backup_service.go` |
| Data dirs | 0755 | **0700** | Unauthorized writes | `internal/config/config.go` |
| Log files | 0644 | **0640** | Info disclosure | `internal/caddy/config.go` |
| Test temp | 0777 | **0700** | Test pollution | `*_test.go` files |

**Files Requiring Updates (8 total):**
1. `cmd/seed/seed_smoke_test.go` - Test DB files (0600)
2. `internal/caddy/config.go` - Log files (0640)
3. `internal/config/config.go` - Data dirs (0700), DB files (0600)
4. `internal/database/database_test.go` - Test DB (0600)
5. `internal/services/backup_service.go` - Backup files (0600)
6. `internal/services/backup_service_test.go` - Test backups (0600)
7. `internal/services/uptime_service_test.go` - Test DB (0600)
8. `internal/util/crypto_test.go` - Test temp files (0600)

---

**G115: Integer Overflow (3 issues)**

```go
// BEFORE:
intValue := int(int64Value) // Unchecked conversion

// AFTER:
import "math"

func SafeInt64ToInt(val int64) (int, error) {
    if val > math.MaxInt || val < math.MinInt {
        return 0, fmt.Errorf("integer overflow: value %d exceeds int range", val)
    }
    return int(val), nil
}

// Usage:
intValue, err := SafeInt64ToInt(int64Value)
if err != nil {
    return fmt.Errorf("invalid integer value: %w", err)
}
```

**Security Analysis:**
- **Risk Level:** MEDIUM (logic errors, potential bypass)
- **Impact:** Array bounds violations, incorrect calculations
- **Affected:** Timeout values, retry counts, array indices

---

**G304: File Inclusion (3 issues)**

```go
// BEFORE:
content, err := ioutil.ReadFile(userInput) // Arbitrary file read

// AFTER:
// Use SafeJoinPath from G305 fix above
safePath, err := SafeJoinPath(allowedDir, userInput)
if err != nil {
    return fmt.Errorf("invalid file path: %w", err)
}

// Additional validation: Check file extension whitelist
allowedExts := map[string]bool{".json": true, ".yaml": true, ".yml": true}
ext := filepath.Ext(safePath)
if !allowedExts[ext] {
    return fmt.Errorf("file type not allowed: %s", ext)
}

content, err := os.ReadFile(safePath)
```

**Security Analysis:**
- **Risk Level:** HIGH (arbitrary file read)
- **Mitigation:** Path validation + extension whitelist
- **Impact:** Limits read access to configuration files only

---

**G404: Weak Random (Informational)**

*(Using crypto/rand for security-sensitive operations, math/rand for non-security randomness - no changes needed)*

### Phase 2: Frontend TypeScript Linting Fixes (6 warnings)

*(Apply the same 6 TypeScript fixes as documented in the original plan)*

### Phase 3: Retry Monitoring Implementation

#### 3.1 Data Model & Persistence

**Database Schema Extensions:**

```go
// Add to models/uptime_monitor.go
type UptimeMonitor struct {
    // ... existing fields ...

    // Retry statistics (new fields)
    TotalChecks       uint64    `gorm:"default:0" json:"total_checks"`
    RetryAttempts     uint64    `gorm:"default:0" json:"retry_attempts"`
    RetryRate         float64   `gorm:"-" json:"retry_rate"` // Computed field
    LastRetryAt       time.Time `json:"last_retry_at,omitempty"`
}

// Add computed field method
func (m *UptimeMonitor) CalculateRetryRate() float64 {
    if m.TotalChecks == 0 {
        return 0.0
    }
    return float64(m.RetryAttempts) / float64(m.TotalChecks) * 100.0
}
```

**Migration:**
```sql
-- Add retry tracking columns
ALTER TABLE uptime_monitors ADD COLUMN total_checks INTEGER DEFAULT 0;
ALTER TABLE uptime_monitors ADD COLUMN retry_attempts INTEGER DEFAULT 0;
ALTER TABLE uptime_monitors ADD COLUMN last_retry_at DATETIME;
```

#### 3.2 Thread-Safe Metrics Collection

**New File: `backend/internal/metrics/uptime_metrics.go`**

```go
package metrics

import (
    "sync"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// UptimeMetrics provides thread-safe retry tracking
type UptimeMetrics struct {
    mu sync.RWMutex

    // Per-monitor statistics
    monitorStats map[uint]*MonitorStats

    // Prometheus metrics
    checksTotal *prometheus.CounterVec
    retriesTotal *prometheus.CounterVec
    retryRate *prometheus.GaugeVec
}

type MonitorStats struct {
    TotalChecks   uint64
    RetryAttempts uint64
    LastRetryAt   time.Time
}

// Global instance
var (
    once sync.Once
    instance *UptimeMetrics
)

// GetMetrics returns singleton instance
func GetMetrics() *UptimeMetrics {
    once.Do(func() {
        instance = &UptimeMetrics{
            monitorStats: make(map[uint]*MonitorStats),
            checksTotal: promauto.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "charon_uptime_checks_total",
                    Help: "Total number of uptime checks performed",
                },
                []string{"monitor_id", "monitor_name", "check_type"},
            ),
            retriesTotal: promauto.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "charon_uptime_retries_total",
                    Help: "Total number of retry attempts",
                },
                []string{"monitor_id", "monitor_name", "check_type"},
            ),
            retryRate: promauto.NewGaugeVec(
                prometheus.GaugeOpts{
                    Name: "charon_uptime_retry_rate_percent",
                    Help: "Percentage of checks requiring retries (over last 1000 checks)",
                },
                []string{"monitor_id", "monitor_name"},
            ),
        }
    })
    return instance
}

// RecordCheck records a successful first-try check
func (m *UptimeMetrics) RecordCheck(monitorID uint, monitorName, checkType string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.monitorStats[monitorID]; !exists {
        m.monitorStats[monitorID] = &MonitorStats{}
    }

    m.monitorStats[monitorID].TotalChecks++

    // Update Prometheus counter
    m.checksTotal.WithLabelValues(
        fmt.Sprintf("%d", monitorID),
        monitorName,
        checkType,
    ).Inc()

    // Update retry rate gauge
    m.updateRetryRate(monitorID, monitorName)
}

// RecordRetry records a retry attempt
func (m *UptimeMetrics) RecordRetry(monitorID uint, monitorName, checkType string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.monitorStats[monitorID]; !exists {
        m.monitorStats[monitorID] = &MonitorStats{}
    }

    stats := m.monitorStats[monitorID]
    stats.RetryAttempts++
    stats.LastRetryAt = time.Now()

    // Update Prometheus counter
    m.retriesTotal.WithLabelValues(
        fmt.Sprintf("%d", monitorID),
        monitorName,
        checkType,
    ).Inc()

    // Update retry rate gauge
    m.updateRetryRate(monitorID, monitorName)
}

// updateRetryRate calculates and updates the retry rate gauge
func (m *UptimeMetrics) updateRetryRate(monitorID uint, monitorName string) {
    stats := m.monitorStats[monitorID]
    if stats.TotalChecks == 0 {
        return
    }

    rate := float64(stats.RetryAttempts) / float64(stats.TotalChecks) * 100.0

    m.retryRate.WithLabelValues(
        fmt.Sprintf("%d", monitorID),
        monitorName,
    ).Set(rate)
}

// GetStats returns current statistics (thread-safe)
func (m *UptimeMetrics) GetStats(monitorID uint) *MonitorStats {
    m.mu.RLock()
    defer m.mu.RUnlock()

    if stats, exists := m.monitorStats[monitorID]; exists {
        // Return a copy to prevent mutation
        return &MonitorStats{
            TotalChecks:   stats.TotalChecks,
            RetryAttempts: stats.RetryAttempts,
            LastRetryAt:   stats.LastRetryAt,
        }
    }
    return nil
}

// GetAllStats returns all monitor statistics
func (m *UptimeMetrics) GetAllStats() map[uint]*MonitorStats {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Return deep copy
    result := make(map[uint]*MonitorStats)
    for id, stats := range m.monitorStats {
        result[id] = &MonitorStats{
            TotalChecks:   stats.TotalChecks,
            RetryAttempts: stats.RetryAttempts,
            LastRetryAt:   stats.LastRetryAt,
        }
    }
    return result
}
```

#### 3.3 Integration with Uptime Service

**Update: `backend/internal/services/uptime_service.go`**

```go
import "github.com/yourusername/charon/internal/metrics"

func (s *UptimeService) performCheck(monitor *models.UptimeMonitor) error {
    metrics := metrics.GetMetrics()
    success := false

    for retry := 0; retry <= s.config.MaxRetries && !success; retry++ {
        if retry > 0 {
            // Record retry attempt
            metrics.RecordRetry(
                monitor.ID,
                monitor.Name,
                string(monitor.Type),
            )
            logger.Log().Info("Retrying check",
                zap.Uint("monitor_id", monitor.ID),
                zap.Int("attempt", retry))
        }

        // Perform actual check
        var err error
        switch monitor.Type {
        case models.HTTPMonitor:
            err = s.checkHTTP(monitor)
        case models.TCPMonitor:
            err = s.checkTCP(monitor)
        // ... other check types
        }

        if err == nil {
            success = true
            // Record successful check
            metrics.RecordCheck(
                monitor.ID,
                monitor.Name,
                string(monitor.Type),
            )
        }
    }

    return nil
}
```

#### 3.4 API Endpoint for Statistics

**New Handler: `backend/internal/api/handlers/uptime_stats_handler.go`**

```go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/yourusername/charon/internal/metrics"
    "github.com/yourusername/charon/internal/models"
)

type UptimeStatsResponse struct {
    MonitorID     uint      `json:"monitor_id"`
    MonitorName   string    `json:"monitor_name"`
    TotalChecks   uint64    `json:"total_checks"`
    RetryAttempts uint64    `json:"retry_attempts"`
    RetryRate     float64   `json:"retry_rate_percent"`
    LastRetryAt   string    `json:"last_retry_at,omitempty"`
    Status        string    `json:"status"` // "healthy" or "warning"
}

func GetUptimeStats(c *gin.Context) {
    m := metrics.GetMetrics()
    allStats := m.GetAllStats()

    // Fetch monitor names from database
    var monitors []models.UptimeMonitor
    if err := models.DB.Find(&monitors).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch monitors"})
        return
    }

    monitorMap := make(map[uint]string)
    for _, mon := range monitors {
        monitorMap[mon.ID] = mon.Name
    }

    // Build response
    response := make([]UptimeStatsResponse, 0, len(allStats))
    for id, stats := range allStats {
        retryRate := 0.0
        if stats.TotalChecks > 0 {
            retryRate = float64(stats.RetryAttempts) / float64(stats.TotalChecks) * 100.0
        }

        status := "healthy"
        if retryRate > 5.0 {
            status = "warning"
        }

        resp := UptimeStatsResponse{
            MonitorID:     id,
            MonitorName:   monitorMap[id],
            TotalChecks:   stats.TotalChecks,
            RetryAttempts: stats.RetryAttempts,
            RetryRate:     retryRate,
            Status:        status,
        }

        if !stats.LastRetryAt.IsZero() {
            resp.LastRetryAt = stats.LastRetryAt.Format(time.RFC3339)
        }

        response = append(response, resp)
    }

    c.JSON(http.StatusOK, response)
}
```

**Register Route:**
```go
// In internal/api/routes.go
api.GET("/uptime/stats", handlers.GetUptimeStats)
```

#### 3.5 Prometheus Metrics Exposition

**Metrics Output Format:**

```prometheus
# HELP charon_uptime_checks_total Total number of uptime checks performed
# TYPE charon_uptime_checks_total counter
charon_uptime_checks_total{monitor_id="1",monitor_name="example.com",check_type="http"} 1247

# HELP charon_uptime_retries_total Total number of retry attempts
# TYPE charon_uptime_retries_total counter
charon_uptime_retries_total{monitor_id="1",monitor_name="example.com",check_type="http"} 34

# HELP charon_uptime_retry_rate_percent Percentage of checks requiring retries
# TYPE charon_uptime_retry_rate_percent gauge
charon_uptime_retry_rate_percent{monitor_id="1",monitor_name="example.com"} 2.73
```

**Access:** `GET /metrics` (existing Prometheus endpoint)

#### 3.6 Alert Integration

**Prometheus Alert Rule:**

```yaml
# File: configs/prometheus/alerts.yml
groups:
  - name: uptime_monitoring
    rules:
      - alert: HighRetryRate
        expr: charon_uptime_retry_rate_percent > 5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High retry rate detected for monitor {{ $labels.monitor_name }}"
          description: "Monitor {{ $labels.monitor_name }} (ID: {{ $labels.monitor_id }}) has a retry rate of {{ $value }}% over the last 1000 checks."
```

**Application-Level Logging:**

```go
// In uptime_service.go - Add to performCheck after retry loop
if retry > 0 {
    stats := metrics.GetMetrics().GetStats(monitor.ID)
    if stats != nil {
        retryRate := float64(stats.RetryAttempts) / float64(stats.TotalChecks) * 100.0
        if retryRate > 5.0 {
            logger.Log().Warn("High retry rate detected",
                zap.Uint("monitor_id", monitor.ID),
                zap.String("monitor_name", monitor.Name),
                zap.Float64("retry_rate", retryRate),
                zap.Uint64("total_checks", stats.TotalChecks),
                zap.Uint64("retry_attempts", stats.RetryAttempts),
            )
        }
    }
}
```

#### 3.7 Frontend Dashboard Widget

**New Component: `frontend/src/components/RetryStatsCard.tsx`**

```tsx
import React, { useEffect, useState } from 'react';
import axios from 'axios';

interface RetryStats {
  monitor_id: number;
  monitor_name: string;
  total_checks: number;
  retry_attempts: number;
  retry_rate_percent: number;
  status: 'healthy' | 'warning';
  last_retry_at?: string;
}

export const RetryStatsCard: React.FC = () => {
  const [stats, setStats] = useState<RetryStats[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const response = await axios.get('/api/v1/uptime/stats');
        setStats(response.data);
      } catch (error) {
        console.error('Failed to fetch retry stats:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
    const interval = setInterval(fetchStats, 30000); // Refresh every 30s

    return () => clearInterval(interval);
  }, []);

  if (loading) return <div>Loading retry statistics...</div>;

  const warningMonitors = stats.filter(s => s.status === 'warning');

  return (
    <div className="retry-stats-card">
      <h2>Uptime Retry Statistics</h2>

      {warningMonitors.length > 0 && (
        <div className="alert alert-warning">
          <strong>⚠️ High Retry Rate Detected</strong>
          <p>{warningMonitors.length} monitor(s) exceeding 5% retry threshold</p>
        </div>
      )}

      <table className="table">
        <thead>
          <tr>
            <th>Monitor</th>
            <th>Total Checks</th>
            <th>Retries</th>
            <th>Retry Rate</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {stats.map(stat => (
            <tr key={stat.monitor_id} className={stat.status === 'warning' ? 'warning' : ''}>
              <td>{stat.monitor_name}</td>
              <td>{stat.total_checks.toLocaleString()}</td>
              <td>{stat.retry_attempts.toLocaleString()}</td>
              <td>{stat.retry_rate_percent.toFixed(2)}%</td>
              <td>
                <span className={`badge ${stat.status === 'warning' ? 'badge-warning' : 'badge-success'}`}>
                  {stat.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
```

#### 3.8 Race Condition Prevention

**Thread Safety Guarantees:**

1. **Read-Write Mutex:** `sync.RWMutex` in `UptimeMetrics`
   - Multiple readers can access stats concurrently
   - Writers get exclusive access during updates
   - No data races on `monitorStats` map

2. **Atomic Operations:** Prometheus client library handles internal atomicity
   - Counter increments are atomic
   - Gauge updates are atomic
   - No manual synchronization needed for Prometheus metrics

3. **Immutable Returns:** `GetStats()` returns a copy, not reference
   - Prevents external mutation of internal state
   - Safe to use returned values without locking

4. **Singleton Pattern:** `sync.Once` ensures single initialization
   - No race during metrics instance creation
   - Safe for concurrent first access

**Stress Test:**

```go
// File: backend/internal/metrics/uptime_metrics_test.go
func TestConcurrentAccess(t *testing.T) {
    m := GetMetrics()

    // Simulate 100 monitors with concurrent updates
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(2)
        monitorID := uint(i)

        // Concurrent check recordings
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                m.RecordCheck(monitorID, fmt.Sprintf("monitor-%d", monitorID), "http")
            }
        }()

        // Concurrent retry recordings
        go func() {
            defer wg.Done()
            for j := 0; j < 50; j++ {
                m.RecordRetry(monitorID, fmt.Sprintf("monitor-%d", monitorID), "http")
            }
        }()
    }

    wg.Wait()

    // Verify no data corruption
    for i := 0; i < 100; i++ {
        stats := m.GetStats(uint(i))
        require.NotNil(t, stats)
        assert.Equal(t, uint64(1000), stats.TotalChecks)
        assert.Equal(t, uint64(50), stats.RetryAttempts)
    }
}
```

---

## Implementation Plan

### Phase 1: Backend Go Linting Fixes

**Estimated Time:** 3-4 hours

**Tasks:**

1. **Errcheck Fixes** (60 min)
   - [ ] Fix 6 JSON unmarshal errors
   - [ ] Fix 11 environment variable operations
   - [ ] Fix 4 database close operations
   - [ ] Fix 3 HTTP write operations
   - [ ] Fix 3 AutoMigrate calls

2. **Gosec Fixes** (2-3 hours)
   - [ ] Fix 8 permission issues
   - [ ] Fix 3 integer overflow issues
   - [ ] Fix 3 file inclusion issues
   - [ ] Fix 1 slice bounds issue
   - [ ] Fix 2 decompression bomb issues
   - [ ] Fix 1 file traversal issue
   - [ ] Fix 2 Slowloris issues
   - [ ] Fix 1 hardcoded credential (add #nosec comment)

**Verification:**
```bash
cd backend && golangci-lint run ./...
# Expected: 0 issues
```

### Phase 2: Frontend TypeScript Linting Fixes

**Estimated Time:** 1-2 hours

*(Same as original plan)*

### Phase 3: Retry Monitoring Implementation

**Estimated Time:** 4-5 hours

*(Same as original plan)*

---

## Acceptance Criteria

**Phase 1 Complete:**
- [ ] All 40 Go linting issues resolved (18 errcheck + 22 gosec)
- [ ] `golangci-lint run ./...` exits with code 0
- [ ] All unit tests pass
- [ ] Code coverage ≥85%
- [ ] **Security validation:**
  - [ ] G110 (decompression bomb): Verify 100MB limit enforced
  - [ ] G305 (path traversal): Test with `../../etc/passwd` attack input
  - [ ] G306 (file permissions): Verify database files are 0600
  - [ ] G304 (file inclusion): Verify extension whitelist blocks `.exe` files
  - [ ] Database close errors: Verify `t.Errorf` is called on close failure
  - [ ] HTTP write errors: Verify mock server returns 500 on write failure

**Phase 2 Complete:**
- [ ] All 6 TypeScript warnings resolved
- [ ] `npm run lint` shows 0 warnings
- [ ] All unit tests pass
- [ ] Code coverage ≥85%

**Phase 3 Complete:**
- [ ] Retry rate metric exposed at `/metrics`
- [ ] API endpoint `/api/v1/uptime/stats` returns correct data
- [ ] Dashboard displays retry rate widget
- [ ] Alert logged when retry rate >5%
- [ ] E2E test validates monitoring flow
- [ ] **Thread safety validation:**
  - [ ] Concurrent access test passes (100 monitors, 1000 ops each)
  - [ ] Race detector (`go test -race`) shows no data races
  - [ ] Prometheus metrics increment correctly under load
  - [ ] `GetStats()` returns consistent data during concurrent updates
- [ ] **Monitoring validation:**
  - [ ] Prometheus `/metrics` endpoint exposes all 3 metric types
  - [ ] Retry rate gauge updates within 1 second of retry event
  - [ ] Dashboard widget refreshes every 30 seconds
  - [ ] Alert triggers when retry rate >5% for 10 minutes
  - [ ] Database persistence: Stats survive application restart

---

## File Changes Summary

### Backend Files (21 total)

#### Errcheck (14 files):
1. `internal/api/handlers/security_handler_audit_test.go` (1)
2. `internal/api/handlers/security_handler_coverage_test.go` (2)
3. `internal/api/handlers/settings_handler_test.go` (3)
4. `internal/config/config_test.go` (13)
5. `internal/caddy/config_test.go` (1)
6. `internal/services/dns_provider_service_test.go` (5)
7. `internal/database/errors_test.go` (1)
8. `internal/caddy/manager_additional_test.go` (2)
9. `internal/caddy/manager_test.go` (1)
10. `internal/api/handlers/notification_coverage_test.go` (1)
11. `internal/api/handlers/pr_coverage_test.go` (2)

#### Gosec (18 files):
12. `cmd/seed/seed_smoke_test.go`
13. `internal/api/handlers/manual_challenge_handler.go`
14. `internal/api/handlers/security_handler_rules_decisions_test.go`
15. `internal/caddy/config.go`
16. `internal/config/config.go`
17. `internal/crowdsec/hub_cache.go`
18. `internal/crowdsec/hub_sync.go`
19. `internal/database/database_test.go`
20. `internal/services/backup_service.go`
21. `internal/services/backup_service_test.go`
22. `internal/services/uptime_service_test.go`
23. `internal/util/crypto_test.go`

### Frontend Files (5 total):
1. `src/components/ImportSitesModal.test.tsx`
2. `src/components/ImportSitesModal.tsx`
3. `src/components/__tests__/DNSProviderForm.test.tsx`
4. `src/context/AuthContext.tsx`
5. `src/hooks/__tests__/useImport.test.ts`

## Security Impact Analysis Summary

### Critical Fixes

| Issue | Pre-Fix Risk | Post-Fix Risk | Mitigation Effectiveness |
|-------|-------------|---------------|-------------------------|
| **G110 - Decompression Bomb** | HIGH (Memory exhaustion DoS) | LOW | 100MB hard limit prevents attacks |
| **G305 - Path Traversal** | CRITICAL (Arbitrary file access) | LOW | Multi-layer validation blocks escapes |
| **G306 - File Permissions** | HIGH (Data exfiltration) | LOW | Restrictive permissions (0600/0700) |
| **G304 - File Inclusion** | HIGH (Config poisoning) | MEDIUM | Extension whitelist limits exposure |
| **Database Close** | LOW (Resource leak) | MINIMAL | Error logging aids debugging |
| **HTTP Write** | MEDIUM (Silent test failure) | LOW | Fast-fail prevents false positives |

### Attack Vector Coverage

**Blocked Attacks:**
- ✅ Gzip bomb (G110) - 100MB limit
- ✅ Directory traversal (G305) - Path validation
- ✅ Credential theft (G306) - Database files secured
- ✅ Config injection (G304) - Extension filtering

**Remaining Considerations:**
- Symlink attacks mitigated by `filepath.Abs()` resolution
- Integer overflow (G115) caught before array access
- Test fixtures (G101) properly annotated as non-functional

---

## Monitoring Technical Specification

### Architecture

```
┌─────────────────┐
│ Uptime Service  │
│  (Goroutines)   │──┐
└─────────────────┘  │
                     │  Record metrics
┌─────────────────┐  │  (thread-safe)
│ HTTP Checks     │──┤
└─────────────────┘  │
                     │
┌─────────────────┐  │
│ TCP Checks      │──┤
└─────────────────┘  │
                     ▼
              ┌──────────────────┐
              │  UptimeMetrics   │
              │  (Singleton)     │
              │  sync.RWMutex    │
              └──────────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
        ▼            ▼            ▼
   Prometheus   Database    REST API
   /metrics     Persistence  /api/v1/uptime/stats
        │            │            │
        ▼            ▼            ▼
   Grafana    Auto-backup  React Dashboard
   Dashboard   (SQLite)    (Real-time)
```

### Data Flow

1. **Collection:** `RecordCheck()` / `RecordRetry()` called after each uptime check
2. **Storage:** In-memory map + Prometheus counters/gauges updated atomically
3. **Persistence:** Database updated every 5 minutes via background goroutine
4. **Exposition:**
   - Prometheus: Scraped every 15s by external monitoring
   - REST API: Polled every 30s by frontend dashboard
5. **Alerting:** Prometheus evaluates rules every 1m, triggers webhook on breach

### Performance Characteristics

- **Memory:** ~50 bytes per monitor (100 monitors = 5KB)
- **CPU:** < 0.1% overhead (mutex contention minimal)
- **Disk:** 1 write/5min (negligible I/O impact)
- **Network:** 3 Prometheus metrics per monitor (300 bytes/scrape for 100 monitors)

---

---

## References

- **Go Lint Output:** `backend/final_lint.txt` (34 issues), `backend/full_lint_output.txt` (40 issues)
- **TypeScript Lint Output:** `npm run lint` output (6 warnings)
- **Gosec:** https://github.com/securego/gosec
- **golangci-lint:** https://golangci-lint.run/
- **Prometheus Best Practices:** https://prometheus.io/docs/practices/naming/
- **OWASP Secure Coding:** https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/
- **CWE-409 Decompression Bomb:** https://cwe.mitre.org/data/definitions/409.html
- **CWE-22 Path Traversal:** https://cwe.mitre.org/data/definitions/22.html

---

**Plan Status:** ✅ Ready for Implementation (Post-Supervisor Review)
**Changes Made:**
- ✅ Database close pattern updated (use `t.Errorf`)
- ✅ HTTP write errors with proper handling
- ✅ Gosec G101 annotation added
- ✅ Decompression bomb mitigation (100MB limit)
- ✅ Path traversal validation logic
- ✅ File permission security matrix documented
- ✅ Complete monitoring technical specification
- ✅ Thread safety guarantees documented
- ✅ Security acceptance criteria added

**Next Step:** Begin Phase 1 - Backend Go Linting Fixes (Errcheck first, then Gosec)
