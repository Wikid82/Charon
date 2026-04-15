# Coverage Improvement Plan — Patch Coverage ≥ 90%

**Date**: 2026-05-02
**Status**: Draft — Awaiting Approval
**Priority**: High
**Archived Previous Plan**: Custom Certificate Upload & Management (Issue #22) → `docs/plans/archive/custom-cert-upload-management-spec-2026-05-02.md`

---

## 1. Introduction

This plan identifies exact uncovered branches across the six highest-gap backend source files and two frontend components, and specifies new test cases to close those gaps. The target is to raise overall patch coverage from **85.61% (206 missing lines)** to **≥ 90%**.

**Constraints**:
- No source file modifications — test files only
- Go tests placed in `*_patch_coverage_test.go` (same package as source)
- Frontend tests extend existing `__tests__/*.test.tsx` files
- Use testify (Go) and Vitest + React Testing Library (frontend)

---

## 2. Research Findings

### 2.1 Coverage Gap Summary

| Package | File | Missing Lines | Current Coverage |
|---|---|---|---|
| `handlers` | `certificate_handler.go` | ~54 | 70.28% |
| `services` | `certificate_service.go` | ~54 | 82.85% |
| `services` | `certificate_validator.go` | ~18 | 88.68% |
| `handlers` | `proxy_host_handler.go` | ~12 | 55.17% |
| `config` | `config.go` | ~8 | ~92% |
| `caddy` | `manager.go` | ~10 | ~88% |
| Frontend | `CertificateList.tsx` | moderate | — |
| Frontend | `CertificateUploadDialog.tsx` | moderate | — |

### 2.2 Test Infrastructure (Confirmed)

- **In-memory DB**: `gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})`
- **Mock auth**: `mockAuthMiddleware()` from `coverage_helpers_test.go`
- **Mock backup service**: `&mockBackupService{createFunc: ..., availableSpaceFunc: ...}`
- **Manager test hooks**: package-level `generateConfigFunc`, `validateConfigFunc`, `writeFileFunc` vars with `defer` restore pattern
- **Frontend mocks**: `vi.mock('../../hooks/...', ...)` and `vi.mock('react-i18next', ...)`

### 2.3 Existing Patch Test Files

| File | Existing Tests |
|---|---|
| `certificate_handler_patch_coverage_test.go` | `TestDelete_UUID_WithBackup_Success`, `_NotFound`, `_InUse` |
| `certificate_service_patch_coverage_test.go` | `TestExportCertificate_DER`, `_PFX`, `_P12`, `_UnsupportedFormat` |
| `certificate_validator_extra_coverage_test.go` | ECDSA/Ed25519 key match, `ConvertDERToPEM` valid/invalid |
| `manager_patch_coverage_test.go` | DNS provider encryption key paths |
| `proxy_host_handler_test.go` | Full CRUD + BulkUpdateACL + BulkUpdateSecurityHeaders |
| `proxy_host_handler_update_test.go` | Update edge cases, `ParseForwardPortField`, `ParseNullableUintField` |

---

## 3. Technical Specifications — Per-File Gap Analysis

### 3.1 `certificate_handler.go` — Export Re-Auth Path (~18 lines)

The `Export` handler re-authenticates the user when `include_key=true`. All six guard branches are uncovered.

**Gap location**: Lines ~260–320 (password empty check, `user` context key extraction, `map[string]any` cast, `id` field lookup, DB user lookup, bcrypt check)

**New tests** (append to `certificate_handler_patch_coverage_test.go`):

| Test Name | Scenario | Expected |
|---|---|---|
| `TestExport_IncludeKey_MissingPassword` | POST with `include_key=true`, no `password` field | 403 |
| `TestExport_IncludeKey_NoUserContext` | No `"user"` key in gin context | 403 |
| `TestExport_IncludeKey_InvalidClaimsType` | `"user"` set to a plain string | 403 |
| `TestExport_IncludeKey_UserIDNotInClaims` | `user = map[string]any{}` with no `"id"` key | 403 |
| `TestExport_IncludeKey_UserNotFoundInDB` | Valid claims, no matching user row | 403 |
| `TestExport_IncludeKey_WrongPassword` | User in DB, wrong plaintext password submitted | 403 |

### 3.2 `certificate_handler.go` — Export Service Errors (~4 lines)

**Gap location**: After `ExportCertificate` call — ErrCertNotFound and generic error branches

| Test Name | Scenario | Expected |
|---|---|---|
| `TestExport_CertNotFound` | Unknown UUID | 404 |
| `TestExport_ServiceError` | Service returns non-not-found error | 500 |

### 3.3 `certificate_handler.go` — Delete Numeric-ID Error Paths (~12 lines)

**Gap location**: `IsCertificateInUse` error, disk space check, backup error, `DeleteCertificateByID` returning `ErrCertInUse` or generic error

| Test Name | Scenario | Expected |
|---|---|---|
| `TestDelete_NumericID_UsageCheckError` | `IsCertificateInUse` returns error | 500 |
| `TestDelete_NumericID_LowDiskSpace` | `availableSpaceFunc` returns 0 | 507 |
| `TestDelete_NumericID_BackupError` | `createFunc` returns error | 500 |
| `TestDelete_NumericID_CertInUse_FromService` | `DeleteCertificateByID` → `ErrCertInUse` | 409 |
| `TestDelete_NumericID_DeleteError` | `DeleteCertificateByID` → generic error | 500 |

### 3.4 `certificate_handler.go` — Delete UUID Additional Error Paths (~8 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestDelete_UUID_UsageCheckInternalError` | `IsCertificateInUseByUUID` returns non-ErrCertNotFound error | 500 |
| `TestDelete_UUID_LowDiskSpace` | `availableSpaceFunc` returns 0 | 507 |
| `TestDelete_UUID_BackupCreationError` | `createFunc` returns error | 500 |
| `TestDelete_UUID_CertInUse_FromService` | `DeleteCertificate` → `ErrCertInUse` | 409 |

### 3.5 `certificate_handler.go` — Upload/Validate File Open Errors (~8 lines)

**Gap location**: `file.Open()` calls on multipart key and chain form files returning errors

| Test Name | Scenario | Expected |
|---|---|---|
| `TestUpload_KeyFile_OpenError` | Valid cert file, malformed key multipart entry | 500 |
| `TestUpload_ChainFile_OpenError` | Valid cert+key, malformed chain multipart entry | 500 |
| `TestValidate_KeyFile_OpenError` | Valid cert, malformed key multipart entry | 500 |
| `TestValidate_ChainFile_OpenError` | Valid cert+key, malformed chain multipart entry | 500 |

### 3.6 `certificate_handler.go` — `sendDeleteNotification` Rate-Limit (~2 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestSendDeleteNotification_RateLimit` | Call `sendDeleteNotification` twice within 10-second window | Second call is a no-op |

---

### 3.7 `certificate_service.go` — `SyncFromDisk` Branches (~14 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestSyncFromDisk_StagingToProductionUpgrade` | DB has staging cert, disk has production cert for same domain | DB cert updated to production provider |
| `TestSyncFromDisk_ExpiryOnlyUpdate` | Disk cert content matches DB cert, only expiry changed | Only `expires_at` column updated |
| `TestSyncFromDisk_CertRootStatPermissionError` | `os.Chmod(certRoot, 0)` before sync; add skip guard `if os.Getuid() == 0 { t.Skip("chmod permission test cannot run as root") }` | No panic; logs error; function completes |

### 3.8 `certificate_service.go` — `ListCertificates` Background Goroutine (~4 lines)

**Gap location**: `initialized=true` && TTL expired path → spawns background goroutine

| Test Name | Scenario | Expected |
|---|---|---|
| `TestListCertificates_StaleCache_TriggersBackgroundSync` | `initialized=true`, `lastScan` = 10 min ago | Returns cached list without blocking; background sync completes |

*Use `require.Eventually(t, func() bool { return svc.lastScan.After(before) }, 2*time.Second, 10*time.Millisecond, "background sync did not update lastScan")` after the call — avoids flaky fixed sleeps.*

### 3.9 `certificate_service.go` — `GetDecryptedPrivateKey` Nil encSvc and Decrypt Failure (~4 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestGetDecryptedPrivateKey_NoEncSvc` | Service with `nil` encSvc, cert has non-empty `PrivateKeyEncrypted` | Returns error |
| `TestGetDecryptedPrivateKey_DecryptFails` | encSvc configured, corrupted ciphertext in DB | Returns wrapped error |

### 3.10 `certificate_service.go` — `MigratePrivateKeys` Branches (~6 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestMigratePrivateKeys_NoEncSvc` | `encSvc == nil` | Returns nil; logs warning |
| `TestMigratePrivateKeys_WithRows` | DB has cert with `private_key` populated, valid encSvc | Row migrated: `private_key` cleared, `private_key_enc` set |

### 3.11 `certificate_service.go` — `UpdateCertificate` Errors (~4 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestUpdateCertificate_NotFound` | Non-existent UUID | Returns `ErrCertNotFound` |
| `TestUpdateCertificate_DBSaveError` | Valid UUID, DB closed before Save | Returns wrapped error |

### 3.12 `certificate_service.go` — `DeleteCertificate` ACME File Cleanup (~8 lines)

**Gap location**: `cert.Provider == "letsencrypt"` branch → Walk certRoot and remove `.crt`/`.key`/`.json` files

| Test Name | Scenario | Expected |
|---|---|---|
| `TestDeleteCertificate_LetsEncryptProvider_FileCleanup` | Create temp `.crt` matching cert domain, delete cert | `.crt` removed from disk |
| `TestDeleteCertificate_StagingProvider_FileCleanup` | Provider = `"letsencrypt-staging"` | Same cleanup behavior triggered |

### 3.13 `certificate_service.go` — `CheckExpiringCertificates` (~8 lines)

**Implementation** (lines ~966–1020): queries `provider = 'custom'` certs expiring before `threshold`, iterates and sends notification for certs with `daysLeft <= warningDays`.

| Test Name | Scenario | Expected |
|---|---|---|
| `TestCheckExpiringCertificates_ExpiresInRange` | Custom cert `expires_at = now+5d`, warningDays=30 | Returns slice with 1 cert |
| `TestCheckExpiringCertificates_AlreadyExpired` | Custom cert `expires_at = yesterday` | Result contains cert with negative days |
| `TestCheckExpiringCertificates_DBError` | DB closed before query | Returns error |

---

### 3.14 `certificate_validator.go` — `DetectFormat` Password-Protected PFX (~2 lines)

**Gap location**: PFX where `pkcs12.DecodeAll("")` fails but first byte is `0x30` (ASN.1 SEQUENCE), DER parse also fails → returns `FormatPFX`

**New file**: `certificate_validator_patch_coverage_test.go`

| Test Name | Scenario | Expected |
|---|---|---|
| `TestDetectFormat_PasswordProtectedPFX` | Generate PFX with non-empty password, call `DetectFormat` | Returns `FormatPFX` |

### 3.15 `certificate_validator.go` — `parsePEMPrivateKey` Additional Block Types (~4 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestParsePEMPrivateKey_PKCS1RSA` | PEM block type `"RSA PRIVATE KEY"` (x509.MarshalPKCS1PrivateKey) | Returns RSA key |
| `TestParsePEMPrivateKey_EC` | PEM block type `"EC PRIVATE KEY"` (x509.MarshalECPrivateKey) | Returns ECDSA key |

### 3.16 `certificate_validator.go` — `detectKeyType` P-384 and Unknown Curves (~4 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestDetectKeyType_ECDSAP384` | P-384 ECDSA key | Returns `"ECDSA-P384"` |
| `TestDetectKeyType_ECDSAUnknownCurve` | ECDSA key with custom/unknown curve (e.g. P-224) | Returns `"ECDSA"` |

### 3.17 `certificate_validator.go` — `ConvertPEMToPFX` Empty Chain (~2 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestConvertPEMToPFX_EmptyChain` | Valid cert+key PEM, empty chain string | Returns PFX bytes without error |

### 3.18 `certificate_validator.go` — `ConvertPEMToDER` Non-Certificate Block (~2 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestConvertPEMToDER_NonCertBlock` | PEM block type `"PRIVATE KEY"` | Returns nil data and error |

### 3.19 `certificate_validator.go` — `formatSerial` Nil BigInt (~2 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestFormatSerial_Nil` | `formatSerial(nil)` | Returns `""` |

---

### 3.20 `proxy_host_handler.go` — `generateForwardHostWarnings` Private IP (~2 lines)

**Gap location**: `net.ParseIP(forwardHost) != nil && network.IsPrivateIP(ip)` branch (non-Docker private IP)

**New file**: `proxy_host_handler_patch_coverage_test.go`

| Test Name | Scenario | Expected |
|---|---|---|
| `TestGenerateForwardHostWarnings_PrivateIP` | forwardHost = `"192.168.1.100"` (RFC-1918, non-Docker) | Returns warning with field `"forward_host"` |

### 3.21 `proxy_host_handler.go` — `BulkUpdateSecurityHeaders` Edge Cases (~4 lines)

| Test Name | Scenario | Expected |
|---|---|---|
| `TestBulkUpdateSecurityHeaders_AllFail_Rollback` | All UUIDs not found → `updated == 0` at end | 400, transaction rolled back |
| `TestBulkUpdateSecurityHeaders_ProfileDB_NonNotFoundError` | Profile lookup returns wrapped DB error | 500 |

---

### 3.22 Frontend: `CertificateList.tsx` — Untested Branches

**File**: `frontend/src/components/__tests__/CertificateList.test.tsx`

| Gap | New Test |
|---|---|
| `bulkDeleteMutation` success | `'calls bulkDeleteMutation.mutate with selected UUIDs on confirm'` |
| `bulkDeleteMutation` error | `'shows error toast on bulk delete failure'` |
| Sort direction toggle | `'toggles sort direction when same column clicked twice'` |
| `selectedIds` reconciliation | `'reconciles selectedIds when certificate list shrinks'` |
| Export dialog open | `'opens export dialog when export button clicked'` |

### 3.23 Frontend: `CertificateUploadDialog.tsx` — Untested Branches

**File**: `frontend/src/components/dialogs/__tests__/CertificateUploadDialog.test.tsx`

| Gap | New Test |
|---|---|
| PFX hides key/chain zones | `'hides key and chain file inputs when PFX file selected'` |
| Upload success closes dialog | `'calls onOpenChange(false) on successful upload'` |
| Upload error shows toast | `'shows error toast when upload mutation fails'` |
| Validate result shown | `'displays validation result after validate clicked'` |

---

## 4. Implementation Plan

### Phase 1: Playwright Smoke Tests (Acceptance Gating)

Add smoke coverage to confirm certificate export and delete flows reach the backend.

**File**: `tests/certificate-coverage-smoke.spec.ts`

```typescript
import { test, expect } from '@playwright/test'

test.describe('Certificate Coverage Smoke', () => {
  test('export dialog opens when export button clicked', async ({ page }) => {
    await page.goto('/')
    // navigate to Certificates, click export on a cert
    // assert dialog visible
  })

  test('delete dialog opens for deletable certificate', async ({ page }) => {
    await page.goto('/')
    // assert delete confirmation dialog appears
  })
})
```

### Phase 2: Backend — Handler Tests

**File**: `backend/internal/api/handlers/certificate_handler_patch_coverage_test.go`
**Action**: Append all tests from sections 3.1–3.6.

Setup pattern for handler tests:

```go
func setupCertHandlerTest(t *testing.T) (*gin.Engine, *CertificateHandler, *gorm.DB) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.User{}, &models.ProxyHost{}))
    tmpDir := t.TempDir()
    certSvc := services.NewCertificateService(tmpDir, db, nil)
    backup := &mockBackupService{
        availableSpaceFunc: func() (int64, error) { return 1 << 30, nil },
        createFunc:         func(string) (string, error) { return "/tmp/backup.db", nil },
    }
    h := NewCertificateHandler(certSvc, backup, nil)
    h.SetDB(db)
    r := gin.New()
    r.Use(mockAuthMiddleware())
    h.RegisterRoutes(r.Group("/api"))
    return r, h, db
}
```

For `TestExport_IncludeKey_*` tests: inject user into gin context directly using a custom middleware wrapper that sets `"user"` (type `map[string]any`, field `"id"`) to the desired value.

### Phase 3: Backend — Service Tests

**File**: `backend/internal/services/certificate_service_patch_coverage_test.go`
**Action**: Append all tests from sections 3.7–3.13.

Setup pattern:

```go
func newTestSvc(t *testing.T) (*CertificateService, *gorm.DB, string) {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}))
    tmpDir := t.TempDir()
    return NewCertificateService(tmpDir, db, nil), db, tmpDir
}
```

For `TestMigratePrivateKeys_WithRows`: use `db.Exec("INSERT INTO ssl_certificates (..., private_key) VALUES (...)` raw SQL to bypass GORM's `gorm:"-"` tag.

### Phase 4: Backend — Validator Tests

**File**: `backend/internal/services/certificate_validator_patch_coverage_test.go` (new)

Key helpers needed:

```go
// generatePKCS1RSAKeyPEM returns an RSA key in PKCS#1 "RSA PRIVATE KEY" PEM format.
func generatePKCS1RSAKeyPEM(t *testing.T) []byte {
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    require.NoError(t, err)
    return pem.EncodeToMemory(&pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(key),
    })
}

// generateECKeyPEM returns an EC key in "EC PRIVATE KEY" (SEC1) PEM format.
func generateECKeyPEM(t *testing.T, curve elliptic.Curve) []byte {
    key, err := ecdsa.GenerateKey(curve, rand.Reader)
    require.NoError(t, err)
    b, err := x509.MarshalECPrivateKey(key)
    require.NoError(t, err)
    return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
}
```

### Phase 5: Backend — Proxy Host Handler Tests

**File**: `backend/internal/api/handlers/proxy_host_handler_patch_coverage_test.go` (new)

Setup pattern mirrors existing `proxy_host_handler_test.go` — use in-memory SQLite, `mockAuthMiddleware`, and `mockCaddyManager` (already available via test hook vars).

### Phase 6: Frontend Tests

**Files**:
- `frontend/src/components/__tests__/CertificateList.test.tsx`
- `frontend/src/components/dialogs/__tests__/CertificateUploadDialog.test.tsx`

Use existing mock structure; add new `it(...)` blocks inside existing `describe` blocks.

Frontend bulk delete success test pattern:

```typescript
it('calls bulkDeleteMutation.mutate with selected UUIDs on confirm', async () => {
    const bulkDeleteFn = vi.fn()
    mockUseBulkDeleteCertificates.mockReturnValue({
        mutate: bulkDeleteFn,
        isPending: false,
    })
    render(<CertificateList />)
    // select checkboxes, click bulk delete, confirm dialog
    expect(bulkDeleteFn).toHaveBeenCalledWith(['uuid-1', 'uuid-2'])
})
```

### Phase 7: Validation

1. `cd /projects/Charon && bash scripts/go-test-coverage.sh`
2. `cd /projects/Charon && bash scripts/frontend-test-coverage.sh`
3. `bash scripts/local-patch-report.sh` → verify `test-results/local-patch-report.md` shows ≥ 90%
4. `bash scripts/scan-gorm-security.sh --check` → zero CRITICAL/HIGH

---

## 5. Commit Slicing Strategy

**Decision**: One PR with 5 ordered, independently-reviewable commits.

**Rationale**: Four packages touched across two build systems (Go + Node). Atomic commits allow targeted revert if a mock approach proves brittle for a specific file, without rolling back unrelated coverage gains.

| # | Scope | Files | Dependencies | Validation Gate |
|---|---|---|---|---|
| **Commit 1** | Handler re-auth + delete + file-open errors | `certificate_handler_patch_coverage_test.go` (extend) | None | `go test ./backend/internal/api/handlers/...` |
| **Commit 2** | Service SyncFromDisk, ListCerts, GetDecryptedKey, Migrate, Update, Delete, CheckExpiring | `certificate_service_patch_coverage_test.go` (extend) | None | `go test ./backend/internal/services/...` |
| **Commit 3** | Validator DetectFormat, parsePEMPrivateKey, detectKeyType, ConvertPEMToPFX/DER, formatSerial | `certificate_validator_patch_coverage_test.go` (new) | Commit 2 not required (separate file) | `go test ./backend/internal/services/...` |
| **Commit 4** | Proxy host warnings + BulkUpdateSecurityHeaders edge cases | `proxy_host_handler_patch_coverage_test.go` (new) | None | `go test ./backend/internal/api/handlers/...` |
| **Commit 5** | Frontend CertificateList + CertificateUploadDialog | `CertificateList.test.tsx`, `CertificateUploadDialog.test.tsx` (extend) | None | `npm run test` |

**Rollback**: Any commit is safe to revert independently — all changes are additive test-only files.

**Contingency**: If the `Export` handler's re-auth tests require gin context injection that the current router wiring doesn't support cleanly, use a sub-router with a custom test middleware that pre-populates `"user"` (`map[string]any{"id": uint(1)}`) with the specific value under test, bypassing `mockAuthMiddleware` for those cases only.

---

## 6. Acceptance Criteria

- [ ] `go test -race ./backend/...` — all tests pass, no data races
- [ ] Backend patch coverage ≥ 90% for all modified Go files per `test-results/local-patch-report.md`
- [ ] `npm run test` — all Vitest tests pass
- [ ] Frontend patch coverage ≥ 90% for `CertificateList.tsx` and `CertificateUploadDialog.tsx`
- [ ] GORM security scan: zero CRITICAL/HIGH findings
- [ ] No new `//nolint` or `//nosec` directives introduced
- [ ] No source file modifications — test files only
- [ ] All new Go test names follow `TestFunctionName_Scenario` convention
- [ ] Previous spec archived to `docs/plans/archive/`

---

## 7. Estimated Coverage Impact

| File | Current | Estimated After | Lines Recovered |
|---|---|---|---|
| `certificate_handler.go` | 70.28% | ~85% | ~42 lines |
| `certificate_service.go` | 82.85% | ~92% | ~44 lines |
| `certificate_validator.go` | 88.68% | ~96% | ~18 lines |
| `proxy_host_handler.go` | 55.17% | ~60% | ~8 lines |
| `CertificateList.tsx` | moderate | high | ~15 lines |
| `CertificateUploadDialog.tsx` | moderate | high | ~12 lines |
| **Overall patch** | **85.61%** | **≥ 90%** | **~139 lines** |

> **Note**: Proxy host handler remains below 90% after this plan because the `Create`/`Update`/`Delete` handler paths require full Caddy manager mock integration. A follow-up plan should address these with a dedicated `mockCaddyManager` interface.
