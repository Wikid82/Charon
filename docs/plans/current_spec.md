# Phase 3: Backend Routes Implementation Plan

> **Phase**: 3 of Skipped Tests Remediation
> **Status**: ✅ COMPLETE
> **Created**: 2026-01-22
> **Completed**: 2026-01-22
> **Target Tests**: 7 tests to re-enable
> **Actual Result**: 7 tests enabled and passing

---

## Executive Summary

Phase 3 addresses missing backend API routes and a data persistence issue that block 7 E2E tests:

1. **NPM Import Route** (`/tasks/import/npm`) - 4 skipped tests
2. **JSON Import Route** (`/tasks/import/json`) - 2 skipped tests
3. **SMTP Persistence Bug** - 1 skipped test at `smtp-settings.spec.ts:336`

The existing Caddyfile import infrastructure provides a solid foundation. NPM and JSON import routes will extend this pattern with format-specific parsers.

---

## Root Cause Analysis

### Issue 1: Missing NPM Import Route

**Location**: Tests at [tests/integration/import-to-production.spec.ts](../../tests/integration/import-to-production.spec.ts#L170-L237)

**Problem**: The tests navigate to `/tasks/import/npm` but this route doesn't exist in the frontend router or backend API.

**Evidence**:
```typescript
// From import-to-production.spec.ts lines 170-180
test.skip('should display NPM import page', async ({ page, adminUser }) => {
  await page.goto('/tasks/import/npm');  // Route doesn't exist
  ...
});
```

**Expected NPM Export Format** (from test file):
```json
{
  "proxy_hosts": [
    {
      "domain_names": ["test.example.com"],
      "forward_host": "192.168.1.100",
      "forward_port": 80
    }
  ],
  "access_lists": [],
  "certificates": []
}
```

### Issue 2: Missing JSON Import Route

**Location**: Tests at [tests/integration/import-to-production.spec.ts](../../tests/integration/import-to-production.spec.ts#L243-L256)

**Problem**: The `/tasks/import/json` route is not implemented. Tests navigate to this route expecting a generic JSON configuration import interface.

### Issue 3: SMTP Save Not Persisting

**Location**: Test at [tests/settings/smtp-settings.spec.ts](../../tests/settings/smtp-settings.spec.ts#L336)

**Problem**: After saving SMTP configuration and reloading the page, the updated values don't persist.

**Skip Comment**:
```typescript
// Note: Skip - SMTP save not persisting correctly (backend issue, not test issue)
```

**Analysis of Code Flow**:

1. **Frontend**: [SMTPSettings.tsx](../../frontend/src/pages/SMTPSettings.tsx#L50-L62)
   - Calls `updateSMTPConfig()` which POSTs to `/settings/smtp`
   - On success, invalidates query and shows toast

2. **Backend Handler**: [settings_handler.go](../../backend/internal/api/handlers/settings_handler.go#L109-L136)
   - `UpdateSMTPConfig()` receives the request
   - Calls `h.MailService.SaveSMTPConfig(config)`

3. **Mail Service**: [mail_service.go](../../backend/internal/services/mail_service.go#L117-L144)
   - `SaveSMTPConfig()` uses upsert pattern
   - **POTENTIAL BUG**: Uses `First()` then conditional `Create()`/`Updates()` separately

**Root Cause Hypothesis**:
The `SaveSMTPConfig` method has a problematic upsert pattern:
```go
// Current pattern in mail_service.go lines 127-143:
result := s.db.Where("key = ?", key).First(&models.Setting{})
if result.Error == gorm.ErrRecordNotFound {
    s.db.Create(&setting)  // Creates new
} else {
    s.db.Model(&models.Setting{}).Where("key = ?", key).Updates(...)  // Updates existing
}
```

**Issues identified**:
1. No transaction wrapping - partial failures possible
2. `Updates()` with map may not update all fields correctly
3. If `First()` returns error other than `ErrRecordNotFound`, the else branch runs but may not execute correctly
4. Race condition between read and write operations

---

## Implementation Plan

### Task 1: Implement NPM Import Backend Handler

**File**: `backend/internal/api/handlers/npm_import_handler.go` (NEW)

#### 1.1 Create NPM Parser Model

```go
// NPMExport represents the Nginx Proxy Manager export format
type NPMExport struct {
    ProxyHosts   []NPMProxyHost   `json:"proxy_hosts"`
    AccessLists  []NPMAccessList  `json:"access_lists"`
    Certificates []NPMCertificate `json:"certificates"`
}

type NPMProxyHost struct {
    DomainNames     []string `json:"domain_names"`
    ForwardScheme   string   `json:"forward_scheme"`
    ForwardHost     string   `json:"forward_host"`
    ForwardPort     int      `json:"forward_port"`
    CachingEnabled  bool     `json:"caching_enabled"`
    BlockExploits   bool     `json:"block_exploits"`
    AllowWebsocket  bool     `json:"allow_websocket_upgrade"`
    HTTP2Support    bool     `json:"http2_support"`
    HSTSEnabled     bool     `json:"hsts_enabled"`
    HSTSSubdomains  bool     `json:"hsts_subdomains"`
    SSLForced       bool     `json:"ssl_forced"`
    Enabled         bool     `json:"enabled"`
}

type NPMAccessList struct {
    Name   string         `json:"name"`
    Items  []NPMAccessItem `json:"items"`
}

type NPMAccessItem struct {
    Type    string `json:"type"`  // "allow" or "deny"
    Address string `json:"address"`
}

type NPMCertificate struct {
    NiceName    string   `json:"nice_name"`
    DomainNames []string `json:"domain_names"`
    Provider    string   `json:"provider"`
}
```

#### 1.2 Create NPM Import Handler

**File**: `backend/internal/api/handlers/npm_import_handler.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/caddy"
    "github.com/Wikid82/charon/backend/internal/models"
    "github.com/Wikid82/charon/backend/internal/services"
)

type NPMImportHandler struct {
    db           *gorm.DB
    proxyHostSvc *services.ProxyHostService
}

func NewNPMImportHandler(db *gorm.DB) *NPMImportHandler {
    return &NPMImportHandler{
        db:           db,
        proxyHostSvc: services.NewProxyHostService(db),
    }
}

func (h *NPMImportHandler) RegisterRoutes(router *gin.RouterGroup) {
    router.POST("/import/npm/upload", h.Upload)
    router.POST("/import/npm/commit", h.Commit)
}

// Upload handles NPM export JSON upload and returns preview
func (h *NPMImportHandler) Upload(c *gin.Context) {
    var req struct {
        Content string `json:"content" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Parse NPM export JSON
    var npmExport NPMExport
    if err := json.Unmarshal([]byte(req.Content), &npmExport); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid NPM export JSON"})
        return
    }

    // Convert to internal format
    result := h.convertNPMToImportResult(npmExport)

    // Check for conflicts with existing hosts
    existingHosts, _ := h.proxyHostSvc.List()
    existingDomainsMap := make(map[string]models.ProxyHost)
    for _, eh := range existingHosts {
        existingDomainsMap[eh.DomainNames] = eh
    }

    conflictDetails := make(map[string]gin.H)
    for _, ph := range result.Hosts {
        if existing, found := existingDomainsMap[ph.DomainNames]; found {
            result.Conflicts = append(result.Conflicts, ph.DomainNames)
            conflictDetails[ph.DomainNames] = gin.H{
                "existing": gin.H{
                    "forward_scheme": existing.ForwardScheme,
                    "forward_host":   existing.ForwardHost,
                    "forward_port":   existing.ForwardPort,
                },
                "imported": gin.H{
                    "forward_scheme": ph.ForwardScheme,
                    "forward_host":   ph.ForwardHost,
                    "forward_port":   ph.ForwardPort,
                },
            }
        }
    }

    sid := uuid.NewString()
    c.JSON(http.StatusOK, gin.H{
        "session":          gin.H{"id": sid, "state": "transient", "source": "npm"},
        "conflict_details": conflictDetails,
        "preview":          result,
    })
}

func (h *NPMImportHandler) convertNPMToImportResult(export NPMExport) *caddy.ImportResult {
    result := &caddy.ImportResult{
        Hosts:     []caddy.ParsedHost{},
        Conflicts: []string{},
        Errors:    []string{},
    }

    for _, proxy := range export.ProxyHosts {
        // Join domain names with comma for storage
        domains := strings.Join(proxy.DomainNames, ", ")

        host := caddy.ParsedHost{
            DomainNames:      domains,
            ForwardScheme:    proxy.ForwardScheme,
            ForwardHost:      proxy.ForwardHost,
            ForwardPort:      proxy.ForwardPort,
            SSLForced:        proxy.SSLForced,
            WebsocketSupport: proxy.AllowWebsocket,
        }

        if host.ForwardScheme == "" {
            host.ForwardScheme = "http"
        }
        if host.ForwardPort == 0 {
            host.ForwardPort = 80
        }

        result.Hosts = append(result.Hosts, host)
    }

    return result
}
```

#### 1.3 Register NPM Import Routes

**File**: `backend/internal/api/routes/routes.go`

Add to the `Register` function:
```go
// NPM Import Handler
npmImportHandler := handlers.NewNPMImportHandler(db)
npmImportHandler.RegisterRoutes(api)
```

### Task 2: Implement JSON Import Backend Handler

**File**: `backend/internal/api/handlers/json_import_handler.go` (NEW)

The JSON import handler will accept a generic Charon export format:

```go
package handlers

// CharonExport represents a generic Charon configuration export
type CharonExport struct {
    Version     string              `json:"version"`
    ExportedAt  string              `json:"exported_at"`
    ProxyHosts  []CharonProxyHost   `json:"proxy_hosts"`
    AccessLists []CharonAccessList  `json:"access_lists"`
    DNSRecords  []CharonDNSRecord   `json:"dns_records"`
}

type JSONImportHandler struct {
    db           *gorm.DB
    proxyHostSvc *services.ProxyHostService
}

func NewJSONImportHandler(db *gorm.DB) *JSONImportHandler {
    return &JSONImportHandler{
        db:           db,
        proxyHostSvc: services.NewProxyHostService(db),
    }
}

func (h *JSONImportHandler) RegisterRoutes(router *gin.RouterGroup) {
    router.POST("/import/json/upload", h.Upload)
    router.POST("/import/json/commit", h.Commit)
}

// Upload validates and previews JSON import
func (h *JSONImportHandler) Upload(c *gin.Context) {
    var req struct {
        Content string `json:"content" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Try to parse as Charon export format
    var charonExport CharonExport
    if err := json.Unmarshal([]byte(req.Content), &charonExport); err != nil {
        // Fallback: try NPM format
        var npmExport NPMExport
        if err := json.Unmarshal([]byte(req.Content), &npmExport); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Invalid JSON format. Expected Charon or NPM export format.",
            })
            return
        }
        // Convert NPM to import result
        // ... (similar to NPM handler)
    }

    // Convert Charon export to import result
    result := h.convertCharonToImportResult(charonExport)
    // ... (conflict checking and response)
}
```

### Task 3: Implement Frontend Routes

#### 3.1 Create ImportNPM Page

**File**: `frontend/src/pages/ImportNPM.tsx` (NEW)

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useNPMImport } from '../hooks/useNPMImport'
import ImportReviewTable from '../components/ImportReviewTable'
import ImportSuccessModal from '../components/dialogs/ImportSuccessModal'

export default function ImportNPM() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { preview, loading, error, upload, commit, commitResult, clearCommitResult } = useNPMImport()
  const [content, setContent] = useState('')
  const [showReview, setShowReview] = useState(false)
  const [showSuccessModal, setShowSuccessModal] = useState(false)

  const handleUpload = async () => {
    if (!content.trim()) {
      alert(t('importNPM.enterContent'))
      return
    }

    // Validate JSON
    try {
      JSON.parse(content)
    } catch {
      alert(t('importNPM.invalidJSON'))
      return
    }

    try {
      await upload(content)
      setShowReview(true)
    } catch {
      // Error handled by hook
    }
  }

  // ... (rest follows ImportCaddy pattern)

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">{t('importNPM.title')}</h1>
      {/* Similar UI to ImportCaddy but for JSON input */}
    </div>
  )
}
```

#### 3.2 Create ImportJSON Page

**File**: `frontend/src/pages/ImportJSON.tsx` (NEW)

Similar structure to ImportNPM, but handles generic JSON/Charon export format.

#### 3.3 Add Frontend Routes

**File**: `frontend/src/App.tsx`

Add to the Tasks routes section:
```tsx
<Route path="import">
  <Route path="caddyfile" element={<ImportCaddy />} />
  <Route path="crowdsec" element={<ImportCrowdSec />} />
  <Route path="npm" element={<ImportNPM />} />
  <Route path="json" element={<ImportJSON />} />
</Route>
```

#### 3.4 Add Navigation Items

**File**: `frontend/src/components/Layout.tsx`

Add to the import submenu:
```tsx
{ name: t('navigation.npm'), path: '/tasks/import/npm', icon: '📦' },
{ name: t('navigation.json'), path: '/tasks/import/json', icon: '📄' },
```

### Task 4: Fix SMTP Persistence Bug

**File**: `backend/internal/services/mail_service.go`

#### 4.1 Fix the Upsert Pattern

Replace the `SaveSMTPConfig` method (lines ~117-144):

```go
// SaveSMTPConfig saves SMTP settings to the database using proper upsert pattern.
func (s *MailService) SaveSMTPConfig(config *SMTPConfig) error {
    settings := map[string]string{
        "smtp_host":         config.Host,
        "smtp_port":         fmt.Sprintf("%d", config.Port),
        "smtp_username":     config.Username,
        "smtp_password":     config.Password,
        "smtp_from_address": config.FromAddress,
        "smtp_encryption":   config.Encryption,
    }

    // Use a transaction for atomic updates
    return s.db.Transaction(func(tx *gorm.DB) error {
        for key, value := range settings {
            var existing models.Setting
            result := tx.Where("key = ?", key).First(&existing)

            if result.Error == gorm.ErrRecordNotFound {
                // Create new setting
                setting := models.Setting{
                    Key:      key,
                    Value:    value,
                    Type:     "string",
                    Category: "smtp",
                }
                if err := tx.Create(&setting).Error; err != nil {
                    return fmt.Errorf("failed to create setting %s: %w", key, err)
                }
            } else if result.Error == nil {
                // Update existing setting - use Save() instead of Updates()
                existing.Value = value
                existing.Category = "smtp"
                if err := tx.Save(&existing).Error; err != nil {
                    return fmt.Errorf("failed to update setting %s: %w", key, err)
                }
            } else {
                return fmt.Errorf("failed to query setting %s: %w", key, result.Error)
            }
        }
        return nil
    })
}
```

**Key Changes**:
1. Wrapped in transaction for atomicity
2. Using `Save()` instead of `Updates()` for reliable updates
3. Proper error handling for all cases
4. Modifying the fetched struct directly before saving

---

## API Contracts

### NPM Import Upload

**Endpoint**: `POST /api/v1/import/npm/upload`

**Request**:
```json
{
  "content": "{\"proxy_hosts\": [...], \"access_lists\": [], \"certificates\": []}"
}
```

**Response** (200 OK):
```json
{
  "session": {
    "id": "uuid-string",
    "state": "transient",
    "source": "npm"
  },
  "preview": {
    "hosts": [
      {
        "domain_names": "test.example.com",
        "forward_scheme": "http",
        "forward_host": "192.168.1.100",
        "forward_port": 80,
        "ssl_forced": false,
        "websocket_support": false
      }
    ],
    "conflicts": [],
    "errors": []
  },
  "conflict_details": {}
}
```

### JSON Import Upload

**Endpoint**: `POST /api/v1/import/json/upload`

**Request**:
```json
{
  "content": "{\"version\": \"1.0\", \"proxy_hosts\": [...]}"
}
```

**Response**: Same structure as NPM import.

### Import Commit (shared)

**Endpoint**: `POST /api/v1/import/npm/commit` or `POST /api/v1/import/json/commit`

**Request**:
```json
{
  "session_uuid": "uuid-string",
  "resolutions": {
    "example.com": "overwrite",
    "test.com": "skip"
  },
  "names": {
    "example.com": "My Example Site"
  }
}
```

**Response**:
```json
{
  "created": 5,
  "updated": 2,
  "skipped": 1,
  "errors": []
}
```

---

## Files to Create/Modify

### New Files

| File | Purpose |
|------|---------|
| `backend/internal/api/handlers/npm_import_handler.go` | NPM import handler |
| `backend/internal/api/handlers/npm_import_handler_test.go` | Unit tests |
| `backend/internal/api/handlers/json_import_handler.go` | JSON import handler |
| `backend/internal/api/handlers/json_import_handler_test.go` | Unit tests |
| `frontend/src/pages/ImportNPM.tsx` | NPM import page |
| `frontend/src/pages/ImportJSON.tsx` | JSON import page |
| `frontend/src/hooks/useNPMImport.ts` | NPM import hook |
| `frontend/src/hooks/useJSONImport.ts` | JSON import hook |
| `frontend/src/api/npmImport.ts` | NPM import API client |
| `frontend/src/api/jsonImport.ts` | JSON import API client |

### Modified Files

| File | Change |
|------|--------|
| `backend/internal/api/routes/routes.go` | Register new import handlers |
| `backend/internal/services/mail_service.go` | Fix SMTP upsert pattern (lines ~117-144) |
| `frontend/src/App.tsx` | Add new routes (around line 113) |
| `frontend/src/components/Layout.tsx` | Add navigation items (around line 102) |
| `frontend/src/locales/en/translation.json` | Add i18n keys |

---

## Tests to Re-enable

After implementation, update these tests by removing `test.skip`:

### import-to-production.spec.ts

| Line | Test Name | Condition |
|------|-----------|-----------|
| 172 | `should display NPM import page` | NPM route exists |
| 188 | `should parse NPM export JSON` | NPM route exists |
| 204 | `should preview NPM import results` | NPM route exists |
| 220 | `should import NPM proxy hosts and access lists` | NPM route exists |
| 246 | `should display JSON import page` | JSON route exists |
| 262 | `should validate JSON schema before import` | JSON route exists |

### smtp-settings.spec.ts

| Line | Test Name | Condition |
|------|-----------|-----------|
| 336 | `should update existing SMTP configuration` | SMTP persistence fixed |

---

## Verification Steps

### Backend Verification

1. **Unit Tests**:
   ```bash
   go test ./backend/internal/api/handlers/... -run "NPM|JSON" -v
   go test ./backend/internal/services/... -run "SMTP" -v
   ```

2. **API Integration Tests**:
   ```bash
   # NPM Import
   curl -X POST http://localhost:8080/api/v1/import/npm/upload \
     -H "Content-Type: application/json" \
     -H "Cookie: <auth-cookie>" \
     -d '{"content": "{\"proxy_hosts\": [{\"domain_names\": [\"test.com\"], \"forward_host\": \"localhost\", \"forward_port\": 80}]}"}'

   # SMTP Persistence
   curl -X POST http://localhost:8080/api/v1/settings/smtp \
     -H "Content-Type: application/json" \
     -H "Cookie: <auth-cookie>" \
     -d '{"host": "smtp.test.local", "port": 587, "from_address": "test@test.local", "encryption": "starttls"}'

   curl http://localhost:8080/api/v1/settings/smtp -H "Cookie: <auth-cookie>"
   # Should return saved values
   ```

### Frontend Verification

1. Navigate to `/tasks/import/npm` - page should load
2. Navigate to `/tasks/import/json` - page should load
3. Paste valid NPM export JSON - should show preview
4. Save SMTP settings, reload page - values should persist

### E2E Verification

```bash
# Run the import tests
npx playwright test tests/integration/import-to-production.spec.ts --project=chromium

# Run SMTP test
npx playwright test tests/settings/smtp-settings.spec.ts -g "should update existing SMTP configuration" --project=chromium
```

---

## Implementation Checklist

### Phase 3.1: NPM Import (Backend)
- [x] Create `NPMExport` and related structs
- [x] Create `npm_import_handler.go` with Upload and Commit handlers
- [x] Create `npm_import_handler_test.go` with unit tests
- [x] Register routes in `routes.go`
- [x] Test API endpoints manually

### Phase 3.2: JSON Import (Backend)
- [x] Create `CharonExport` struct
- [x] Create `json_import_handler.go` with Upload and Commit handlers
- [x] Create `json_import_handler_test.go` with unit tests
- [x] Register routes in `routes.go`
- [x] Test API endpoints manually

### Phase 3.3: SMTP Persistence Fix
- [x] Update `SaveSMTPConfig` in `mail_service.go`
- [x] Add transaction-based upsert pattern
- [x] Update `mail_service_test.go` with persistence test
- [x] Verify fix with manual testing

### Phase 3.4: Frontend Routes
- [x] Create `ImportNPM.tsx` page component
- [x] Create `ImportJSON.tsx` page component
- [x] Create `useNPMImport.ts` hook
- [x] Create `useJSONImport.ts` hook
- [x] Create API client files
- [x] Add routes to `App.tsx`
- [x] Add navigation items to `Layout.tsx`
- [x] Add i18n translation keys

### Phase 3.5: Test Re-enablement
- [x] Remove `test.skip` from NPM import tests (4 tests)
- [x] Remove `test.skip` from JSON import tests (2 tests)
- [x] Remove `test.skip` from SMTP persistence test (1 test)
- [x] Run full E2E test suite

### Phase 3.6: Verification
- [x] All new tests pass (7 tests enabled and passing)
- [x] No regressions in existing tests
- [x] Update `skipped-tests-remediation.md` with Phase 3 completion

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| NPM export format varies by version | Medium | Medium | Support multiple format versions, validate required fields |
| SMTP fix causes other issues | Low | High | Transaction-based approach is safer, comprehensive tests |
| Frontend state management complexity | Low | Low | Follow existing ImportCaddy pattern exactly |

---

## Dependencies

- Phase 2 completion (TestDataManager auth fix) - **COMPLETE**
- Existing Caddyfile import infrastructure - **AVAILABLE**
- Frontend React component patterns - **AVAILABLE**

---

## Summary for Delegation

### For Backend_Dev Agent:

**Task 1: NPM Import Handler**
- Create file: `backend/internal/api/handlers/npm_import_handler.go`
- Implement structs: `NPMExport`, `NPMProxyHost`, `NPMAccessList`, `NPMCertificate`
- Implement handlers: `Upload()`, `Commit()`
- Register in: `backend/internal/api/routes/routes.go`

**Task 2: JSON Import Handler**
- Create file: `backend/internal/api/handlers/json_import_handler.go`
- Implement structs: `CharonExport`, `CharonProxyHost`
- Implement handlers: `Upload()`, `Commit()` (with NPM format fallback)
- Register in: `backend/internal/api/routes/routes.go`

**Task 3: SMTP Fix**
- File: `backend/internal/services/mail_service.go`
- Function: `SaveSMTPConfig()` (lines ~117-144)
- Fix: Wrap in transaction, use `Save()` instead of `Updates()`

### For Frontend_Dev Agent:

**Task 4: Frontend Routes**
- Create: `frontend/src/pages/ImportNPM.tsx`
- Create: `frontend/src/pages/ImportJSON.tsx`
- Create: `frontend/src/hooks/useNPMImport.ts`
- Create: `frontend/src/hooks/useJSONImport.ts`
- Create: `frontend/src/api/npmImport.ts`
- Create: `frontend/src/api/jsonImport.ts`
- Modify: `frontend/src/App.tsx` (add routes)
- Modify: `frontend/src/components/Layout.tsx` (add nav items)
- Modify: `frontend/src/locales/en/translation.json` (add i18n)

---

## Change Log

| Date | Author | Change |
|------|--------|--------|
| 2026-01-22 | Planning Agent (Architect) | Initial Phase 3 plan created |
| 2026-01-22 | Implementation Team | Phase 3 implementation complete - NPM/JSON import routes, SMTP fix, 7 tests enabled |
