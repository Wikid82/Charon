# Fix: Allow deletion of expiring_soon certificates not in use

> **Status note:** The bug report refers to the status as `expiring_soon`. In this codebase the actual status string is `'expiring'` (defined in `frontend/src/api/certificates.ts` line 10 and `backend/internal/services/certificate_service.go` line 33). All references below use `'expiring'`.

---

## 1. Bug Root Cause

`frontend/src/components/CertificateList.tsx` — `isDeletable` (lines 26–34):

```ts
export function isDeletable(cert: Certificate, hosts: ProxyHost[]): boolean {
  if (!cert.id) return false
  if (isInUse(cert, hosts)) return false
  return (
    cert.provider === 'custom' ||
    cert.provider === 'letsencrypt-staging' ||
    cert.status === 'expired'
  )
}
```

A cert with `provider === 'letsencrypt'` and `status === 'expiring'` that is not attached to any proxy host evaluates to `false` because `'expiring' !== 'expired'`. No delete button is rendered, and the cert cannot be selected for bulk delete.

**Additional bug:** `isInUse` has a false-positive when `cert.id` is falsy — `undefined === undefined` would match any proxy host with an unset certificate reference, incorrectly treating the cert as in-use. Fix by adding `if (!cert.id) return false` as the first line of `isInUse`.

Three secondary locations propagate the same status blind-spot:

- `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx` — `providerLabel` falls through to `return cert.provider` (showing raw `"letsencrypt"`) for expiring certs instead of a human-readable label.
- `frontend/src/components/dialogs/DeleteCertificateDialog.tsx` — `getWarningKey` falls through to `'certificates.deleteConfirmCustom'` for expiring certs instead of a contextual message.

---

## 2. Frontend Fix

### 2a. `frontend/src/components/CertificateList.tsx` — `isDeletable`

**Before:**
```ts
return (
  cert.provider === 'custom' ||
  cert.provider === 'letsencrypt-staging' ||
  cert.status === 'expired'
)
```

**After:**
```ts
return (
  cert.provider === 'custom' ||
  cert.provider === 'letsencrypt-staging' ||
  cert.status === 'expired' ||
  cert.status === 'expiring'
)
```

### 2b. `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx` — `providerLabel`

**Before:**
```ts
function providerLabel(cert: Certificate): string {
  if (cert.provider === 'letsencrypt-staging') return 'Staging'
  if (cert.provider === 'custom') return 'Custom'
  if (cert.status === 'expired') return 'Expired LE'
  return cert.provider
}
```

**After:**
```ts
function providerLabel(cert: Certificate): string {
  if (cert.provider === 'letsencrypt-staging') return 'Staging'
  if (cert.provider === 'custom') return 'Custom'
  if (cert.status === 'expired') return 'Expired LE'
  if (cert.status === 'expiring') return 'Expiring LE'
  return cert.provider
}
```

### 2c. `frontend/src/components/dialogs/DeleteCertificateDialog.tsx` — `getWarningKey`

**Before:**
```ts
function getWarningKey(cert: Certificate): string {
  if (cert.status === 'expired') return 'certificates.deleteConfirmExpired'
  if (cert.provider === 'letsencrypt-staging') return 'certificates.deleteConfirmStaging'
  return 'certificates.deleteConfirmCustom'
}
```

**After:**
```ts
function getWarningKey(cert: Certificate): string {
  if (cert.status === 'expired') return 'certificates.deleteConfirmExpired'
  if (cert.status === 'expiring') return 'certificates.deleteConfirmExpiring'
  if (cert.provider === 'letsencrypt-staging') return 'certificates.deleteConfirmStaging'
  return 'certificates.deleteConfirmCustom'
}
```

### 2d. `frontend/src/locales/en/translation.json` — two string updates

**Add** new key after `"deleteConfirmExpired"`:
```json
"deleteConfirmExpiring": "This certificate is expiring soon. It will be permanently removed and will not be auto-renewed.",
```

**Update** `"noteText"` to reflect that expiring certs are now also deletable:

**Before:**
```json
"noteText": "You can delete custom certificates, staging certificates, and expired production certificates that are not attached to any proxy host. Active production certificates are automatically renewed by Caddy.",
```

**After:**
```json
"noteText": "You can delete custom certificates, staging certificates, and expired or expiring production certificates that are not attached to any proxy host. Active production certificates are automatically renewed by Caddy.",
```

---

## 3. Unit Test Updates

### 3a. `frontend/src/components/__tests__/CertificateList.test.tsx`

**Existing test at line 152–155 — update assertion:**

The test currently documents the wrong behavior:

```ts
it('returns false for expiring LE cert not in use', () => {
  const cert: Certificate = { id: 7, name: 'Exp', domain: 'd', issuer: 'LE', expires_at: '', status: 'expiring', provider: 'letsencrypt' }
  expect(isDeletable(cert, noHosts)).toBe(false)  // wrong — must become true
})
```

**Changes:**
- Rename description to `'returns true for expiring LE cert not in use'`
- Change assertion to `toBe(true)`

**Add** a new test case to guard the in-use path:
```ts
it('returns false for expiring LE cert in use', () => {
  const cert: Certificate = { id: 8, name: 'ExpUsed', domain: 'd', issuer: 'LE', expires_at: '', status: 'expiring', provider: 'letsencrypt' }
  expect(isDeletable(cert, withHost(8))).toBe(false)
})
```

### 3b. `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`

The current `certs` fixture (lines 28–30) covers: `custom/valid`, `letsencrypt-staging/untrusted`, `letsencrypt/expired`. No `expiring` fixture exists.

**Add** a standalone test verifying `providerLabel` renders the correct label for an expiring cert:
```ts
it('renders "Expiring LE" label for expiring letsencrypt certificate', () => {
  render(
    <BulkDeleteCertificateDialog
      certificates={[makeCert({ id: 4, name: 'Cert Four', domain: 'four.example.com', provider: 'letsencrypt', status: 'expiring' })]}
      open={true}
      onConfirm={vi.fn()}
      onCancel={vi.fn()}
      isDeleting={false}
    />
  )
  expect(screen.getByText('Expiring LE')).toBeInTheDocument()
})
```

---

## 4. Backend Verification

**No backend change required.**

`DELETE /api/v1/certificates/:id` is handled by `CertificateHandler.Delete` in `backend/internal/api/handlers/certificate_handler.go` (line 140). The only guard is `IsCertificateInUse` (line 156). There is no status-based check — the handler invokes `service.DeleteCertificate(uint(id))` unconditionally once the in-use check passes.

`CertificateService.DeleteCertificate` in `backend/internal/services/certificate_service.go` (line 396) likewise only inspects `IsCertificateInUse` before proceeding. The cert's `Status` field is never read or compared during deletion.

A cert with `status = 'expiring'` that is not in use is deleted successfully by the backend today; the bug is frontend-only.

---

## 5. E2E Consideration

**File:** `tests/certificate-bulk-delete.spec.ts`

The spec currently has no fixture that places a cert into `expiring` status, because status is computed server-side at query time from the actual expiry date. Manufacturing an `expiring` cert in Playwright requires inserting a certificate whose expiry falls within the renewal window (≈30 days out).

**Recommended addition:** Add a test scenario that uploads a custom certificate with an expiry date 15 days from now and is not attached to any proxy host, then asserts:
1. Its row checkbox is enabled and its per-row delete button is present.
2. It can be selected and bulk-deleted via `BulkDeleteCertificateDialog`.

If producing a near-expiry cert at the E2E layer is not feasible, coverage from §3a and §3b is sufficient for this fix and the E2E test may be deferred to a follow-up.

---

## 6. Commit Slicing Strategy

**Single PR.** This is a self-contained bug fix with no API contract changes and no migration.

Suggested commit message:
```
fix(certificates): allow deletion of expiring certs not in use

Expiring Let's Encrypt certs not attached to any proxy host were
undeletable because isDeletable only permitted deletion when
status === 'expired'. Extends the condition to also allow
status === 'expiring', updates providerLabel and getWarningKey for
consistent UX, and corrects the existing unit test that was asserting
the wrong behavior.
```

**Files changed:**
- `frontend/src/components/CertificateList.tsx`
- `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx`
- `frontend/src/components/dialogs/DeleteCertificateDialog.tsx`
- `frontend/src/locales/en/translation.json`
- `frontend/src/components/__tests__/CertificateList.test.tsx`
- `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`
