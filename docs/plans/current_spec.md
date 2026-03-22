# Certificate Deletion Feature — Spec

**Date**: 2026-03-22
**Priority**: Medium
**Type**: User Requested Feature
**Status**: Approved — Supervisor Reviewed 2026-03-22

---

## 1. Introduction

### Overview

Users accumulate expired and orphaned certificates in the Certificates UI over time. Currently,
the delete button is only shown for `custom` (manually uploaded) and `staging` certificates. Expired
production Let's Encrypt certificates that are no longer attached to any proxy host cannot be
removed, creating UI clutter and user confusion.

### Objectives

1. Allow deletion of **expired** certificates that are not attached to any proxy host.
2. Allow deletion of **custom** (manually uploaded) certificates that are not attached to any
   proxy host, regardless of expiry status (already partially implemented).
3. Allow deletion of **staging** certificates that are not attached to any proxy host (already
   partially implemented).
4. **Prevent deletion** of any certificate currently attached to a proxy host.
5. Replace the native `confirm()` dialog with an accessible, themed confirmation dialog.
6. Provide clear visual feedback on why a certificate can or cannot be deleted.

### Non-Goals

- Bulk certificate deletion (separate feature).
- Auto-cleanup / scheduled pruning of expired certificates.
- Changes to certificate auto-renewal logic.

---

## 2. Research Findings

### 2.1 Existing Backend Infrastructure

The backend already has complete delete support:

| Component | File | Status |
|-----------|------|--------|
| Model | `backend/internal/models/ssl_certificate.go` | `SSLCertificate` struct with `Provider` ("letsencrypt", "letsencrypt-staging", "custom"), `ExpiresAt` fields |
| Service | `backend/internal/services/certificate_service.go` | `DeleteCertificate(id)`, `IsCertificateInUse(id)` — fully implemented |
| Handler | `backend/internal/api/handlers/certificate_handler.go` | `Delete()` — validates in-use, creates backup, deletes, sends notification |
| Route | `backend/internal/api/routes/routes.go:673` | `DELETE /api/v1/certificates/:id` — already registered |
| Error | `backend/internal/services/certificate_service.go:23` | `ErrCertInUse` sentinel error defined |
| Tests | `backend/internal/api/handlers/certificate_handler_test.go` | Tests for in-use, backup, backup failure, auth, invalid ID, not found |

**Key finding**: The backend imposes NO provider or expiry restrictions on deletion. Any certificate
can be deleted as long as it is not referenced by a proxy host (`certificate_id` FK). The
backend is already correct for the requested feature.

### 2.2 Existing Frontend Infrastructure

| Component | File | Status |
|-----------|------|--------|
| API client | `frontend/src/api/certificates.ts` | `deleteCertificate(id)` — exists |
| Hook | `frontend/src/hooks/useCertificates.ts` | `useCertificates()` — react-query based |
| List component | `frontend/src/components/CertificateList.tsx` | Delete button and mutation — exists but **gated incorrectly** |
| Page | `frontend/src/pages/Certificates.tsx` | Upload dialog only |
| Cleanup dialog | `frontend/src/components/dialogs/CertificateCleanupDialog.tsx` | Used for proxy host deletion cleanup — not for standalone cert deletion |
| i18n | `frontend/src/locales/en/translation.json:168-185` | Certificate strings — needs new deletion strings |

### 2.3 Current Delete Button Visibility Logic (The Problem)

In `frontend/src/components/CertificateList.tsx:145`:

```tsx
{cert.id && (cert.provider === 'custom' || cert.issuer?.toLowerCase().includes('staging')) && (
```

This condition **excludes expired production Let's Encrypt certificates**, which is the core
issue. An expired LE cert not attached to any host should be deletable.

### 2.4 Certificate-to-ProxyHost Relationship

- `ProxyHost.CertificateID` (`*uint`, nullable FK) → `SSLCertificate.ID`
- Defined in `backend/internal/models/proxy_host.go:24-25`
- GORM foreign key: `gorm:"foreignKey:CertificateID"`
- **No cascade delete** on the FK — deletion is manually guarded by `IsCertificateInUse()`
- Frontend checks in-use client-side via `hosts.some(h => h.certificate_id === cert.id)`

### 2.5 Provider Values

| Provider Value | Source | Deletable? |
|---------------|--------|------------|
| `letsencrypt` | Auto-provisioned by Caddy ACME | Only when **expired** AND **not in use** |
| `letsencrypt-staging` | Staging ACME | When **not in use** (any status) |
| `custom` | User-uploaded via UI | When **not in use** (any status) |

> **Note**: The model comment in `ssl_certificate.go` lists `"self-signed"` as a possible
> provider, but no code path ever writes that value. The actual provider universe is
> `letsencrypt`, `letsencrypt-staging`, `custom`. The stale comment should be corrected as
> part of this PR.

#### Edge Case: `expiring` LE Cert Not In Use

An `expiring` Let's Encrypt certificate that is not attached to any proxy host is in limbo —
not expired yet, but no proxy host references it, so no renewal will be triggered. **Decision**:
accept this as intended behavior. The cert will eventually expire and become deletable. We do
**not** add `expiring` to the deletable set because Caddy may still auto-renew certificates
that were previously provisioned, even if no host currently references them.

### 2.6 Existing UX Issues

1. Delete uses native `confirm()` — not accessible, not themed.
2. No tooltip or visual indicator explaining why a cert cannot be deleted.
3. The in-use check is duplicated: once client-side before `confirm()`, once server-side in the handler. This is fine (defense in depth) but the server is the source of truth.

---

## 3. Technical Specifications

### 3.1 Backend Changes

**No backend code changes required.** The existing `DELETE /api/v1/certificates/:id` endpoint
already:
- Validates the certificate exists
- Checks `IsCertificateInUse()` and returns `409 Conflict` if in use
- Creates a backup before deletion
- Deletes the DB record (and ACME files for LE certs)
- Invalidates the cert cache
- Sends a notification
- Returns `200 OK` on success

The backend does not restrict by provider or expiry — all deletion policy is enforced by the
frontend's visibility of the delete button and confirmed server-side by the in-use check.

### 3.2 Frontend Changes

#### 3.2.1 Delete Button Visibility — `CertificateList.tsx`

Replace the current delete button condition with new business logic:

```
isDeletable(cert, hosts) =
  cert.id exists
  AND NOT isInUse(cert, hosts)
  AND (
    cert.provider === 'custom'
    OR cert.provider === 'letsencrypt-staging'
    OR cert.status === 'expired'
  )
```

Where `isInUse(cert, hosts)` checks:
```
hosts.some(h => (h.certificate_id ?? h.certificate?.id) === cert.id)
```

In plain terms:
- **Custom / staging** certs: deletable if not in use (any expiry status).
- **Production LE** certs: deletable **only if expired** AND not in use.
- **Any cert in use** by a proxy host: NOT deletable, regardless of status.

> **Important**: Use `cert.provider === 'letsencrypt-staging'` for staging detection — not
> `cert.issuer?.toLowerCase().includes('staging')`. The `provider` field is the canonical,
> authoritative classification. Issuer-based checks are fragile and may break if the ACME
> issuer string changes.

#### 3.2.2 Confirmation Dialog — New `DeleteCertificateDialog.tsx`

Create `frontend/src/components/dialogs/DeleteCertificateDialog.tsx`:

- Reuse the existing `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogFooter`,
  `Button` UI components from `frontend/src/components/ui`.
- Show certificate name, domain, status, and provider.
- Warning text varies by cert type:
  - Custom: "This will permanently delete this certificate. A backup will be created first."
  - Staging: "This staging certificate will be removed. It will be regenerated on next request."
  - Expired LE: "This expired certificate is no longer active and will be permanently removed."
- Two buttons: Cancel (secondary) and Delete (destructive).
- Props: `certificate: Certificate | null`, `onConfirm: () => void`, `onCancel: () => void`,
  `open: boolean`, `isDeleting: boolean`.
- Keyboard accessible: focus trap, Escape to close, Enter on Delete button.

#### 3.2.3 Disabled Delete Button with Tooltip

When a certificate is in use by a proxy host, render the delete button as `aria-disabled="true"`
(not HTML `disabled`) with a Radix Tooltip explaining why. Using `aria-disabled` keeps the
button focusable, which is required for the tooltip to appear on hover/focus.

Use the existing Radix-based Tooltip component from `frontend/src/components/ui/Tooltip.tsx`
(`Tooltip`, `TooltipTrigger`, `TooltipContent` exports).

Tooltip text: "Cannot delete — certificate is attached to a proxy host".

When a production LE cert is valid/expiring (not expired) and not in use, do **not** show the
delete button at all. Production LE certs in active use are auto-managed.

#### 3.2.4 i18n Translation Keys

Add to `frontend/src/locales/en/translation.json` under `"certificates"`:

```json
"deleteTitle": "Delete Certificate",
"deleteConfirmCustom": "This will permanently delete this certificate. A backup will be created first.",
"deleteConfirmStaging": "This staging certificate will be removed. It will be regenerated on next request.",
"deleteConfirmExpired": "This expired certificate is no longer active and will be permanently removed.",
"deleteSuccess": "Certificate deleted",
"deleteFailed": "Failed to delete certificate",
"deleteInUse": "Cannot delete — certificate is attached to a proxy host",
"deleteButton": "Delete"
```

A shared `"common.cancel"` key already exists — use `t('common.cancel')` for the Cancel
button instead of a certificate-specific key.

The same keys should be added to all other locale files (`de`, `es`, `fr`, `pt`) with
placeholder English values (to be translated later).

#### 3.2.5 Data Flow

```
User clicks delete icon → isDeletable check (client) → open DeleteCertificateDialog
  → User confirms → deleteMutation fires:
    1. deleteCertificate(id) → DELETE /api/v1/certificates/:id
       → Handler: IsCertificateInUse check (server)
       → Handler: createBackup (server)
       → Handler: DeleteCertificate (service)
       → Handler: notification
    2. Invalidate react-query cache → UI refreshes
```

Note: Remove the duplicate client-side `createBackup()` call from the mutation — the server
already creates a backup. Keeping the client-side call creates two backups per deletion.

### 3.3 Database Considerations

- **No schema changes needed.** The `ssl_certificates` table and `proxy_hosts.certificate_id` FK
  are already correct.
- **No cascade behavior changes.** Deletion is guarded by the in-use check, not by DB cascades.
- The existing backup-before-delete behavior in the handler is sufficient for data safety.

### 3.4 Security Considerations

- **Authorization**: The `DELETE /api/v1/certificates/:id` route is under the `management` group
  which requires authentication middleware. No changes needed.
- **Server-side validation**: `IsCertificateInUse()` is checked server-side as defense-in-depth,
  preventing deletion even if the frontend check is bypassed.
- **ID parameter**: The handler uses numeric ID from URL param, validated with `strconv.ParseUint`.
  This prevents injection.
- **Backup safety**: A backup is created before every deletion. Low disk space is checked first
  (100MB minimum).

### 3.5 Accessibility Considerations

- New `DeleteCertificateDialog` must use the existing `Dialog` component which already provides
  focus trap, `role="dialog"`, and `aria-modal`.
- Disabled delete buttons must use `aria-disabled="true"` (not HTML `disabled`) to remain
  focusable. Wrap in the Radix `Tooltip` / `TooltipTrigger` / `TooltipContent` from
  `frontend/src/components/ui/Tooltip.tsx` for an accessible visible tooltip (not just
  `title` attribute).
- The delete icon button needs `aria-label` for screen readers.

> **Known inconsistency**: The existing `CertificateCleanupDialog` uses a hand-rolled overlay
> (`<div className="fixed inset-0 bg-black/50 ...">`) instead of the Radix Dialog component.
> This is a pre-existing issue — **not in scope for this PR**. Flagged as a future chore to
> migrate `CertificateCleanupDialog` to Radix Dialog for consistency.

### 3.6 Config/Build File Review

| File | Change Needed? | Notes |
|------|---------------|-------|
| `.gitignore` | No | No new artifacts to ignore |
| `codecov.yml` | No | New dialog component will be covered by existing frontend test config |
| `.dockerignore` | No | No new build inputs |
| `Dockerfile` | No | Frontend is built into `dist/` as part of existing build stage |

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests (Test-First)

**File**: `tests/certificate-delete.spec.ts`

Write E2E tests that define expected behavior:

1. **Test: Delete button visible for expired cert not in use**
   - Seed an expired custom cert with no proxy host attachment.
   - Navigate to Certificates page.
   - Verify delete button is visible for the expired cert row.

2. **Test: Delete button visible for custom cert not in use**
   - Seed a custom cert not attached to any proxy host.
   - Verify delete button is visible.

3. **Test: Delete button disabled for cert in use**
   - Seed a cert attached to a proxy host.
   - Verify delete button is `aria-disabled="true"` with tooltip text.

4. **Test: Delete button NOT visible for valid production LE cert**
   - Seed a valid LE cert not in use.
   - Verify no delete button (auto-managed, not expired).

5. **Test: Confirmation dialog appears on delete click**
   - Click delete on a deletable cert.
   - Verify dialog opens with cert details and Cancel/Delete buttons.
   - Click Cancel, verify dialog closes, cert still exists.

6. **Test: Successful deletion flow**
   - Click delete on a deletable cert.
   - Confirm in dialog.
   - Verify cert disappears from list.
   - Verify success toast appears.

7. **Test: In-use cert shows disabled button with tooltip**
   - Seed a cert in use.
   - Verify delete button has `aria-disabled="true"` and tooltip is shown on hover.

#### E2E Seeding Strategy

Certificates are scanned from Caddy's certificate storage, not manually inserted. Tests
should seed data by:
1. Using the existing API to create proxy hosts with different SSL modes (which triggers
   cert provisioning by Caddy).
2. For expired/custom certs, use the certificate upload API (`POST /api/v1/certificates`)
   with pre-generated test certificates.
3. For in-use vs. not-in-use states, create/delete proxy host associations via the proxy
   host API.
4. Direct database manipulation is a last resort and should be avoided to keep tests
   realistic.

**Complexity**: Low — straightforward UI interaction tests.

### Phase 2: Frontend Implementation

**Estimated changes**: ~3 files modified, 1 file created.

#### Step 1: Create `DeleteCertificateDialog`

**File**: `frontend/src/components/dialogs/DeleteCertificateDialog.tsx`

```
Props:
  - certificate: Certificate | null (from api/certificates.ts)
  - open: boolean
  - onConfirm: () => void
  - onCancel: () => void
  - isDeleting: boolean

Structure:
  - Dialog (open, onOpenChange=onCancel)
    - DialogContent
      - DialogHeader
        - DialogTitle: t('certificates.deleteTitle')
      - Certificate info: name, domain, status badge, provider
      - Warning text (varies by provider/status)
      - DialogFooter
        - Button (secondary): t('common.cancel')
        - Button (destructive, loading=isDeleting): Delete
```

#### Step 2: Update `CertificateList.tsx`

1. Extract `isDeletable(cert, hosts)` helper function.
2. Extract `isInUse(cert, hosts)` helper function.
3. Replace the inline delete button condition with `isDeletable()`.
4. Add disabled delete button with tooltip for in-use certs.
5. Replace `confirm()` with `DeleteCertificateDialog` state management:
   - `const [certToDelete, setCertToDelete] = useState<Certificate | null>(null)`
   - Open dialog: `setCertToDelete(cert)`
   - Confirm: `deleteMutation.mutate(certToDelete.id)`
   - Cancel/success: `setCertToDelete(null)`
6. Remove the duplicate client-side `createBackup()` call from the mutation — the server
   already creates a backup. Keeping the client-side call creates two backups per deletion.

#### Step 3: Add i18n keys

**Files**: All locale files under `frontend/src/locales/*/translation.json`

Add the keys from §3.2.4.

**Complexity**: Low — mostly UI wiring, no new APIs.

### Phase 3: Backend Unit Tests (Gap Coverage)

While the backend code needs no changes, add tests for the newly-important scenarios:

**File**: `backend/internal/api/handlers/certificate_handler_test.go`

1. **Test: Delete expired LE cert not in use succeeds** — ensures the backend does not block
   expired LE certs from deletion.
2. **Test: Delete valid LE cert not in use succeeds** — confirms the backend has no
   provider-based restrictions (policy is frontend-only).

The `IsCertificateInUse` service-level tests already exist in `certificate_service_test.go`.
Do **not** duplicate them. Keep only the handler-level tests above that verify the HTTP layer
behavior for expired LE cert deletion.

**Complexity**: Low — standard Go table-driven tests.

### Phase 4: Frontend Unit Tests

**File**: `frontend/src/components/__tests__/CertificateList.test.tsx`

1. Test `isDeletable()` helper with all provider/status/in-use combinations.
2. Test that delete button renders for deletable certs.
3. Test that delete button is disabled for in-use certs.
4. Test that delete button is hidden for valid production LE certs.

**File**: `frontend/src/components/dialogs/__tests__/DeleteCertificateDialog.test.tsx`

5. Test dialog renders with correct warning text per provider.
6. Test Cancel closes dialog.
7. Test Delete calls onConfirm.

**Complexity**: Low.

### Phase 5: Documentation

Update `frontend/src/locales/en/translation.json` key `"noteText"` to reflect the expanded
deletion policy:

> "You can delete custom certificates, staging certificates, and expired production certificates
> that are not attached to any proxy host. Active production certificates are automatically
> renewed by Caddy."

No other documentation changes needed — the feature is self-explanatory in the UI.

---

## 5. Acceptance Criteria

- [ ] Expired certificates not attached to any proxy host show a delete button.
- [ ] Custom certificates not attached to any proxy host show a delete button.
- [ ] Staging certificates not attached to any proxy host show a delete button.
- [ ] Certificates attached to a proxy host show a disabled delete button with tooltip.
- [ ] Valid production LE certificates not in use do NOT show a delete button.
- [ ] Clicking delete opens an accessible confirmation dialog (not native `confirm()`).
- [ ] Dialog shows certificate details and appropriate warning text.
- [ ] Confirming deletion removes the certificate and shows a success toast.
- [ ] Canceling the dialog does not delete anything.
- [ ] Server returns `409 Conflict` if the certificate becomes attached between client check and
  server delete (race condition safety).
- [ ] A backup is created before each deletion (server-side).
- [ ] All new UI elements are keyboard navigable and screen-reader accessible.
- [ ] All Playwright E2E tests pass on Firefox, Chromium, and WebKit.
- [ ] All new backend unit tests pass.
- [ ] All new frontend unit tests pass.
- [ ] No regressions in existing certificate or proxy host tests.

---

## 6. Commit Slicing Strategy

### Decision: Single PR

**Rationale**: The scope is small (1 new component, 2 modified files, i18n additions, and tests).
All changes are tightly coupled — the new dialog component is only meaningful together with the
updated delete button logic. Splitting this into multiple PRs would add review overhead without
reducing risk.

### PR-1: Certificate Deletion UX Enhancement

**Scope**: All phases (E2E tests, frontend implementation, backend test gaps, frontend unit tests,
docs update).

**Files**:

| File | Action |
|------|--------|
| `tests/certificate-delete.spec.ts` | Create |
| `frontend/src/components/dialogs/DeleteCertificateDialog.tsx` | Create |
| `frontend/src/components/dialogs/__tests__/DeleteCertificateDialog.test.tsx` | Create |
| `frontend/src/components/CertificateList.tsx` | Modify |
| `frontend/src/components/__tests__/CertificateList.test.tsx` | Modify |
| `frontend/src/locales/en/translation.json` | Modify |
| `frontend/src/locales/de/translation.json` | Modify |
| `frontend/src/locales/es/translation.json` | Modify |
| `frontend/src/locales/fr/translation.json` | Modify |
| `frontend/src/locales/pt/translation.json` | Modify |
| `backend/internal/api/handlers/certificate_handler_test.go` | Modify |

**Dependencies**: None — the backend API is already complete.

**Validation Gates**:
- `go test ./backend/...` — all pass
- `npx vitest run` — all pass
- Playwright E2E on Firefox, Chromium, WebKit — all pass
- `make lint-fast` — no new warnings

**Rollback**: Revert the single PR. No database migrations to undo. No backend API changes.

**Contingency**: If E2E tests are flaky due to certificate seed data timing, add explicit
`waitFor` on the certificate list load state before asserting button visibility.
