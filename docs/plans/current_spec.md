# Certificate Bulk Delete — Spec

**Date**: 2026-03-22
**Priority**: Medium
**Type**: User Requested Feature
**Status**: Planning — Awaiting Implementation

---

## 1. Problem Statement

The Certificates page now supports individual deletion of custom, staging, and expired
production Let's Encrypt certificates. Users who accumulate many such certificates must
click through a confirmation dialog for each one. There is no mechanism to select and
destroy multiple certificates in a single operation.

This feature adds checkbox-based bulk selection and a single confirmation step to delete
N certificates at once, operating under exactly the same policy terms as the existing
individual delete affordance.

### Non-Goals

- Changes to the individual delete flow or `DeleteCertificateDialog`.
- A new backend batch endpoint (sequential per-cert calls are sufficient).
- Auto-cleanup / scheduled pruning.
- Migrating `CertificateList` from its current raw `<table>` to the `DataTable` component.

---

## 2. Existing Foundation

### 2.1 Individual Delete Policy (Preserved Verbatim)

The following logic lives in `frontend/src/components/CertificateList.tsx` and must be
preserved without modification. Bulk selection obeys it exactly:

```ts
export function isInUse(cert: Certificate, hosts: ProxyHost[]): boolean {
  return hosts.some(h => (h.certificate_id ?? h.certificate?.id) === cert.id)
}

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

| Certificate Category | `isDeletable` | `isInUse` | Individual button | Checkbox |
|---|---|---|---|---|
| custom / staging — not in use | ✅ true | ❌ false | Active delete Trash2 | ✅ Enabled checkbox |
| custom / staging / expired LE — in use | ❌ false | ✅ true | `aria-disabled` Trash2 + tooltip | ✅ Checkbox rendered but `disabled` + tooltip |
| expired LE — not in use | ✅ true | ❌ false | Active delete Trash2 | ✅ Enabled checkbox |
| valid/expiring LE — not in use | ❌ false | ❌ false | No affordance at all | No checkbox, no column cell |
| valid/expiring LE — in use | ❌ false | ✅ true | No affordance at all | No checkbox, no column cell |

### 2.2 Backend — No New Endpoint Required

`DELETE /api/v1/certificates/:id` is registered at `backend/internal/api/routes/routes.go:673`
and already:

- Guards against in-use certs (`IsCertificateInUse` → `409 Conflict`).
- Creates a server-side backup before deletion.
- Deletes the DB record and ACME files.
- Invalidates the cert cache and fires a notification.

Bulk deletion will call this endpoint N times concurrently using `Promise.allSettled`,
exactly as the ProxyHosts bulk delete does for `deleteHost`. `ids.map(id => deleteCertificate(id))`
fires all promises concurrently; `Promise.allSettled` awaits all settlements before resolving.
No batch endpoint is warranted at this scale.

### 2.3 CertificateList Rendering Architecture

`CertificateList.tsx` renders a purpose-built raw `<table>` with a manual `sortedCertificates`
`useMemo`. It does **not** use the `DataTable` UI component. This plan does not migrate it —
the selection layer will be grafted directly onto the existing table.

The `Checkbox` component at `frontend/src/components/ui/Checkbox.tsx` supports an
`indeterminate` prop backed by Radix UI `CheckboxPrimitive`. This component will be reused
for both the header "select all" checkbox and each row checkbox, matching the rendering
style in `DataTable.tsx`.

### 2.4 Bulk Selection Precedent — ProxyHosts Page

`frontend/src/pages/ProxyHosts.tsx` is the reference implementation for bulk operations:

- `selectedHosts: Set<string>` — `useState<Set<string>>(new Set())`
- The `DataTable` `selectable` prop handles the per-row checkbox column and the header
  "select all" checkbox automatically, but `DataTable.handleSelectAll` selects **every** row.
- For certificates the "select all" must only select the `isDeletable && !isInUse` subset,
  so we cannot delegate to `DataTable`'s built-in logic even if we migrated.
- The bulk action bar is a conditional `<div>` that appears only when
  `selectedHosts.size > 0`, containing the count and action buttons.

---

## 3. Technical Specification

### 3.1 State Changes — `CertificateList.tsx`

Add two pieces of state alongside the existing `certToDelete` and `sortColumn` state:

```ts
const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
const [showBulkDeleteDialog, setShowBulkDeleteDialog] = useState(false)
```

Add a memoised derived set of all cert IDs that are eligible for selection:

```ts
const selectableCertIds = useMemo<Set<number>>(() => {
  const ids = new Set<number>()
  for (const cert of sortedCertificates) {
    if (isDeletable(cert, hosts) && cert.id) {
      ids.add(cert.id)
    }
  }
  return ids
}, [sortedCertificates, hosts])
```

> Only `isDeletable` certs (not in use, correct provider/status) enter `selectableCertIds`.
> In-use-but-would-be-deletable certs (`isInUse && (custom || staging || expired)`) do NOT
> enter the selectable set — their row checkbox is rendered but is `disabled`.

Add selection handlers:

```ts
const handleSelectAll = () => {
  if (selectedIds.size === selectableCertIds.size) {
    setSelectedIds(new Set())
  } else {
    setSelectedIds(new Set(selectableCertIds))
  }
}

const handleSelectRow = (id: number) => {
  const next = new Set(selectedIds)
  next.has(id) ? next.delete(id) : next.add(id)
  setSelectedIds(next)
}
```

Header checkbox state derived from selection:

```ts
const allSelectableSelected =
  selectableCertIds.size > 0 && selectedIds.size === selectableCertIds.size
const someSelected =
  selectedIds.size > 0 && selectedIds.size < selectableCertIds.size
```

### 3.2 Table Column Changes — `CertificateList.tsx`

#### 3.2.1 New Leftmost `<th>` — Header Checkbox

Insert a new `<th>` as the **first column** in the `<thead>` row, before the existing
"Name" column:

```tsx
<th className="w-12 px-4 py-3">
  <Checkbox
    checked={allSelectableSelected}
    indeterminate={someSelected}
    onCheckedChange={handleSelectAll}
    aria-label={t('certificates.bulkSelectAll')}
    disabled={selectableCertIds.size === 0}
  />
</th>
```

Also update the empty-state `<td colSpan={6}>` to `colSpan={7}`.

#### 3.2.2 New Leftmost `<td>` — Per-Row Checkbox

For each row in the `sortedCertificates.map(...)`, insert a new `<td>` as the first cell.
Three mutually exclusive cases:

**Case A — `isDeletable && !isInUse` (enabled checkbox):**

```tsx
<td className="w-12 px-4 py-4">
  <Checkbox
    checked={selectedIds.has(cert.id!)}
    onCheckedChange={() => handleSelectRow(cert.id!)}
    aria-label={t('certificates.selectCert', { name: cert.name || cert.domain })}
  />
</td>
```

**Case B — in-use-but-deletable category (`isInUse && (custom || staging || expired)`):**

The individual delete button mirrors this with `aria-disabled`. The checkbox must match:
rendered but disabled, with the same tooltip text from `t('certificates.deleteInUse')`.

```tsx
<td className="w-12 px-4 py-4">
  <TooltipProvider>
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Checkbox
            checked={false}
            disabled
            aria-disabled="true"
            aria-label={t('certificates.selectCert', { name: cert.name || cert.domain })}
          />
        </span>
      </TooltipTrigger>
      <TooltipContent>{t('certificates.deleteInUse')}</TooltipContent>
    </Tooltip>
  </TooltipProvider>
</td>
```

> Radix `Checkbox` with `disabled` swallows pointer events. Wrapping in a `<span>` restores
> hover targeting for the tooltip — the same technique used for the existing `aria-disabled`
> delete buttons where `TooltipTrigger asChild` wraps the `<button>`.

**Case C — valid/expiring LE cert (no affordance):**

```tsx
<td className="w-12 px-4 py-4" aria-hidden="true" />
```

### 3.3 Bulk Action Toolbar

Render the toolbar **above** the `<div className="bg-dark-card ...">` table container,
inside the outer React fragment. It is conditionally visible when any cert is selected:

```tsx
{selectedIds.size > 0 && (
  <div
    role="status"
    aria-live="polite"
    className="flex items-center justify-between rounded-lg border border-brand-500/30 bg-brand-500/10 px-4 py-2 mb-3"
  >
    <span className="text-sm text-gray-300">
      {t('certificates.bulkSelectedCount', { count: selectedIds.size })}
    </span>
    <Button
      variant="danger"
      size="sm"
      leftIcon={Trash2}
      onClick={() => setShowBulkDeleteDialog(true)}
    >
      {t('certificates.bulkDeleteButton', { count: selectedIds.size })}
    </Button>
  </div>
)}
```

The `Button` component from `frontend/src/components/ui/Button.tsx` with `variant="danger"`,
`size="sm"`, and `leftIcon={Trash2}` matches the ProxyHosts bulk delete button style.

On successful bulk mutation, `setSelectedIds(new Set())` is called in `onSuccess`,
collapsing the toolbar automatically.

### 3.4 Bulk Delete Mutation — `CertificateList.tsx`

Add a second `useMutation` alongside the existing single-delete `deleteMutation`:

```ts
const bulkDeleteMutation = useMutation({
  mutationFn: async (ids: number[]) => {
    const results = await Promise.allSettled(ids.map(id => deleteCertificate(id)))
    const failed = results.filter(r => r.status === 'rejected').length
    const succeeded = results.filter(r => r.status === 'fulfilled').length
    return { succeeded, failed }
  },
  onSuccess: ({ succeeded, failed }) => {
    queryClient.invalidateQueries({ queryKey: ['certificates'] })
    queryClient.invalidateQueries({ queryKey: ['proxyHosts'] })
    setSelectedIds(new Set())
    setShowBulkDeleteDialog(false)
    if (failed > 0) {
      toast.error(t('certificates.bulkDeletePartial', { deleted: succeeded, failed }))
    } else {
      toast.success(t('certificates.bulkDeleteSuccess', { count: succeeded }))
    }
  },
  onError: () => {
    toast.error(t('certificates.bulkDeleteFailed'))
    setShowBulkDeleteDialog(false)
  },
})
```

> Clearing `selectedIds` in `onSuccess` even when `succeeded === 0` (total failure) is
> acceptable UX — the toast communicates the failure and the user can re-select if desired.

Wire the dialog at the bottom of the component's returned fragment:

```tsx
<BulkDeleteCertificateDialog
  certificates={sortedCertificates.filter(c => c.id && selectedIds.has(c.id))}
  open={showBulkDeleteDialog}
  onConfirm={() => bulkDeleteMutation.mutate(Array.from(selectedIds))}
  onCancel={() => setShowBulkDeleteDialog(false)}
  isDeleting={bulkDeleteMutation.isPending}
/>
```

Extend the existing `ConfigReloadOverlay` condition:

```tsx
{(deleteMutation.isPending || bulkDeleteMutation.isPending) && (
  <ConfigReloadOverlay ... />
)}
```

### 3.5 New Dialog — `BulkDeleteCertificateDialog.tsx`

**File**: `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx`

Do **not** modify the existing `DeleteCertificateDialog`. This is a new sibling file.

#### Interface

```ts
interface BulkDeleteCertificateDialogProps {
  certificates: Certificate[]   // the currently selected, deletable certs
  open: boolean
  onConfirm: () => void
  onCancel: () => void
  isDeleting: boolean
}
```

#### Structure

```
Dialog (open, onOpenChange → onCancel)
  DialogContent (max-w-lg)
    DialogHeader
      DialogTitle   → t('certificates.bulkDeleteTitle', { count: certificates.length })
      DialogDescription → t('certificates.bulkDeleteDescription', { count: certificates.length })

    <div className="px-6 space-y-4">
      Warning panel (red border / bg-red-900/10 / AlertTriangle icon — same as DeleteCertificateDialog)
        → t('certificates.bulkDeleteConfirm')

      Scrollable cert list
        className="max-h-48 overflow-y-auto rounded-lg border border-gray-800 divide-y divide-gray-800"
        tabIndex={0}
        aria-label="Certificates to be deleted"
        → For each cert:
             <div className="flex items-center justify-between px-4 py-2">
               <span className="text-sm text-white">{cert.name || cert.domain}</span>
               <span className="text-xs text-gray-500">{providerLabel(cert)}</span>
             </div>
    </div>

    DialogFooter
      Button variant="secondary" onClick={onCancel} disabled={isDeleting}
        → t('common.cancel')
      Button variant="danger" onClick={onConfirm} isLoading={isDeleting}
        → t('certificates.bulkDeleteButton', { count: certificates.length })
```

Returns `null` when `certificates.length === 0`.

#### Provider Label Helper (inline in the file)

```ts
function providerLabel(cert: Certificate): string {
  if (cert.provider === 'letsencrypt-staging') return 'Staging'
  if (cert.provider === 'custom') return 'Custom'
  if (cert.status === 'expired') return 'Expired LE'
  return cert.provider
}
```

### 3.6 i18n Keys

#### English (`frontend/src/locales/en/translation.json`)

Add to the `"certificates"` object (10 new keys):

```json
"bulkSelectAll": "Select all deletable certificates",
"selectCert": "Select certificate {{name}}",
"bulkSelectedCount": "{{count}} certificate(s) selected",
"bulkDeleteTitle": "Delete {{count}} Certificate(s)",
"bulkDeleteDescription": "Delete {{count}} certificate(s)",
"bulkDeleteConfirm": "The following certificates will be permanently deleted. The server creates a backup before each removal.",
"bulkDeleteButton": "Delete {{count}} Certificate(s)",
"bulkDeleteSuccess": "{{count}} certificate(s) deleted",
"bulkDeletePartial": "{{deleted}} deleted, {{failed}} failed",
"bulkDeleteFailed": "Failed to delete certificates"
```

`t('common.cancel')` already exists — no new cancel key needed.

#### German (`frontend/src/locales/de/translation.json`)

```json
"bulkSelectAll": "Alle löschbaren Zertifikate auswählen",
"selectCert": "Zertifikat {{name}} auswählen",
"bulkSelectedCount": "{{count}} Zertifikat(e) ausgewählt",
"bulkDeleteTitle": "{{count}} Zertifikat(e) löschen",
"bulkDeleteDescription": "{{count}} Zertifikat(e) löschen",
"bulkDeleteConfirm": "Die folgenden Zertifikate werden dauerhaft gelöscht. Der Server erstellt vor jeder Löschung eine Sicherung.",
"bulkDeleteButton": "{{count}} Zertifikat(e) löschen",
"bulkDeleteSuccess": "{{count}} Zertifikat(e) gelöscht",
"bulkDeletePartial": "{{deleted}} gelöscht, {{failed}} fehlgeschlagen",
"bulkDeleteFailed": "Zertifikate konnten nicht gelöscht werden"
```

#### Spanish (`frontend/src/locales/es/translation.json`)

```json
"bulkSelectAll": "Seleccionar todos los certificados eliminables",
"selectCert": "Seleccionar certificado {{name}}",
"bulkSelectedCount": "{{count}} certificado(s) seleccionado(s)",
"bulkDeleteTitle": "Eliminar {{count}} Certificado(s)",
"bulkDeleteDescription": "Eliminar {{count}} certificado(s)",
"bulkDeleteConfirm": "Los siguientes certificados se eliminarán permanentemente. El servidor crea una copia de seguridad antes de cada eliminación.",
"bulkDeleteButton": "Eliminar {{count}} Certificado(s)",
"bulkDeleteSuccess": "{{count}} certificado(s) eliminado(s)",
"bulkDeletePartial": "{{deleted}} eliminado(s), {{failed}} fallido(s)",
"bulkDeleteFailed": "No se pudieron eliminar los certificados"
```

#### French (`frontend/src/locales/fr/translation.json`)

```json
"bulkSelectAll": "Sélectionner tous les certificats supprimables",
"selectCert": "Sélectionner le certificat {{name}}",
"bulkSelectedCount": "{{count}} certificat(s) sélectionné(s)",
"bulkDeleteTitle": "Supprimer {{count}} Certificat(s)",
"bulkDeleteDescription": "Supprimer {{count}} certificat(s)",
"bulkDeleteConfirm": "Les certificats suivants seront définitivement supprimés. Le serveur crée une sauvegarde avant chaque suppression.",
"bulkDeleteButton": "Supprimer {{count}} Certificat(s)",
"bulkDeleteSuccess": "{{count}} certificat(s) supprimé(s)",
"bulkDeletePartial": "{{deleted}} supprimé(s), {{failed}} échoué(s)",
"bulkDeleteFailed": "Impossible de supprimer les certificats"
```

#### Chinese (`frontend/src/locales/zh/translation.json`)

```json
"bulkSelectAll": "选择所有可删除的证书",
"selectCert": "选择证书 {{name}}",
"bulkSelectedCount": "已选择 {{count}} 个证书",
"bulkDeleteTitle": "删除 {{count}} 个证书",
"bulkDeleteDescription": "删除 {{count}} 个证书",
"bulkDeleteConfirm": "以下证书将被永久删除。服务器在每次删除前会创建备份。",
"bulkDeleteButton": "删除 {{count}} 个证书",
"bulkDeleteSuccess": "已删除 {{count}} 个证书",
"bulkDeletePartial": "已删除 {{deleted}} 个，{{failed}} 个失败",
"bulkDeleteFailed": "证书删除失败"
```

### 3.7 Data Flow

```
User checks N certs in CertificateList
  → selectedIds: Set<number> grows
  → Bulk action toolbar appears above table

User clicks "Delete N Certificate(s)" in toolbar
  → setShowBulkDeleteDialog(true)
  → BulkDeleteCertificateDialog renders with filtered cert list

User clicks "Delete N Certificate(s)" in dialog
  → bulkDeleteMutation.mutate(Array.from(selectedIds))
  → ConfigReloadOverlay appears (bulkDeleteMutation.isPending)
  → Promise.allSettled → N × DELETE /api/v1/certificates/:id
      Each: server IsCertificateInUse guard, createBackup, DeleteCertificate, notify
  → onSuccess:
      invalidate ['certificates'], invalidate ['proxyHosts']
      setSelectedIds(new Set())  ← toolbar collapses
      setShowBulkDeleteDialog(false)
      toast success (all) or error (partial)
```

### 3.8 Accessibility Checklist

- Header `Checkbox` has `aria-label` from `t('certificates.bulkSelectAll')`.
- Per-row enabled checkboxes have `aria-label` including the cert name.
- Per-row disabled checkboxes have `aria-disabled="true"` and tooltip via Radix
  `Tooltip`/`TooltipTrigger`/`TooltipContent` from `frontend/src/components/ui/Tooltip.tsx`.
- Empty `<td aria-hidden="true" />` cells for valid/expiring LE rows are hidden from the
  accessibility tree.
- Bulk delete toolbar button has visible text label (count + action) — not icon-only.
- Bulk action toolbar `<div>` has `role="status"` and `aria-live="polite"` so assistive
  technologies announce selection count changes.
- `BulkDeleteCertificateDialog` inherits focus trap and `role="dialog"` from Radix `Dialog`.
- Scrollable cert list inside the dialog has `tabIndex={0}` and `aria-label`.
- `colSpan` updated 6 → 7 to keep the empty-state row spanning all columns correctly.

---

## 4. Implementation Plan

### Phase 1 — New Dialog Component

**File to create**: `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx`

This phase has no state, no side effects, and no dependencies on Phase 2 or 3. It can be
reviewed in isolation.

1. Scaffold per §3.5: imports, interface, `providerLabel` helper, component body.
2. Import `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `DialogDescription`,
   `DialogFooter` from `../ui/Dialog`.
3. Import `Button` from `../ui/Button`.
4. Import `AlertTriangle` from `lucide-react`.
5. Import `useTranslation` from `react-i18next`.
6. Import `Certificate` type from `../../api/certificates`.
7. Export as default.

**Acceptance**: `npx tsc --noEmit` reports 0 errors; component renders without crashing
in a unit test harness.

### Phase 2 — i18n Keys

**Files to modify** (all under `frontend/src/locales/`):
`en/translation.json`, `de/translation.json`, `es/translation.json`,
`fr/translation.json`, `zh/translation.json`

Add the 10 keys from §3.6 to the `"certificates"` object in each file.

**Acceptance**: No missing-key warnings in the browser console or `i18next` debug output
when the Certificates page is loaded.

### Phase 3 — CertificateList Selection Layer

**File to modify**: `frontend/src/components/CertificateList.tsx`

Step-by-step surgical additions:

1. Add imports: `Checkbox` from `./ui/Checkbox`; `BulkDeleteCertificateDialog` from
   `./dialogs/BulkDeleteCertificateDialog`.
2. Add state: `selectedIds`, `showBulkDeleteDialog` (§3.1).
3. Add memos: `selectableCertIds`, `allSelectableSelected`, `someSelected` (§3.1).
4. Add handlers: `handleSelectAll`, `handleSelectRow` (§3.1).
5. Add `bulkDeleteMutation` (§3.4).
6. Extend `ConfigReloadOverlay` condition to `|| bulkDeleteMutation.isPending` (§3.4).
7. Insert bulk action toolbar above the `<div className="bg-dark-card ...">` (§3.3).
8. Insert leftmost `<th>` (§3.2.1); update empty-state `colSpan` 6 → 7.
9. Insert leftmost `<td>` for each row with the three cases A/B/C (§3.2.2).
10. Mount `<BulkDeleteCertificateDialog ...>` at the end of the fragment (§3.4).

**Invariant**: `isDeletable` and `isInUse` exported function signatures must not change.
All pre-existing assertions in `CertificateList.test.tsx` must continue to pass.

### Phase 4 — Unit Tests

#### 4.1 New file: `BulkDeleteCertificateDialog.test.tsx`

**File**: `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx`

Mock `react-i18next` using the `t: (key, opts?) => opts ? JSON.stringify(opts) : key`
pattern used throughout the test suite.

| # | Test description |
|---|---|
| 1 | renders dialog with count in title when 3 certs supplied |
| 2 | lists each certificate name in the scrollable list |
| 3 | calls `onConfirm` when the Delete button is clicked |
| 4 | calls `onCancel` when the Cancel button is clicked |
| 5 | Delete button is loading/disabled when `isDeleting={true}` |
| 6 | returns null when `certificates` array is empty |

#### 4.2 Additions to `CertificateList.test.tsx`

Extend the existing `describe('CertificateList', ...)` in
`frontend/src/components/__tests__/CertificateList.test.tsx`.

The existing fixture (`createCertificatesValue`) already supplies:
- `id: 1` custom expired, not in use → `isDeletable = true` → enabled checkbox
- `id: 2` letsencrypt-staging, not in use → `isDeletable = true` → enabled checkbox
- `id: 4` custom valid, not in use → `isDeletable = true` → enabled checkbox
- `id: 5` expired LE, not in use → `isDeletable = true` → enabled checkbox
- `id: 3` custom valid, in use (host has `certificate_id: 3`) → disabled checkbox
- `id: 6` valid LE, not in use → `isDeletable = false` → no checkbox

| # | Test description |
|---|---|
| 1 | renders enabled checkboxes for ids 1, 2, 4, 5 (deletable, not in use) |
| 2 | renders disabled checkbox (with `aria-disabled`) for id 3 (in-use) |
| 3 | renders no checkbox in id 6's row (valid production LE) |
| 4 | selecting one cert makes the bulk action toolbar visible |
| 5 | header select-all selects only ids 1, 2, 4, 5 — not id 3 (in-use) |
| 6 | clicking the toolbar Delete button opens `BulkDeleteCertificateDialog` |
| 7 | confirming in the bulk dialog calls `deleteCertificate` for each selected ID |

### Phase 5 — Playwright E2E Tests

**File to create**: `tests/certificate-bulk-delete.spec.ts`

Reuse `createCustomCertViaAPI` from `tests/certificate-delete.spec.ts`. Import shared
test helpers from:
- `tests/fixtures/auth-fixtures` — `test`, `expect`, `loginUser`
- `tests/utils/wait-helpers` — `waitForLoadingComplete`, `waitForDialog`,
  `waitForAPIResponse`
- `tests/fixtures/test-data` — `generateUniqueId`
- `tests/constants` — `STORAGE_STATE`

Seed three custom certs via `beforeAll`, clean up with `afterAll`. Each `test.beforeEach`
navigates to `/certificates` and calls `waitForLoadingComplete`.

| # | Test scenario |
|---|---|
| 1 | **Checkbox column present**: checkboxes appear for each deletable cert |
| 2 | **No checkbox for valid LE**: valid production LE cert row has no checkbox |
| 3 | **Select one → toolbar appears**: checking one cert shows the count and Delete button |
| 4 | **Select-all**: header checkbox selects all three seeded certs; toolbar shows count 3 |
| 5 | **Dialog shows correct count**: opening bulk dialog shows "Delete 3 Certificate(s)" |
| 6 | **Cancel preserves certs**: cancelling the dialog leaves all three certs in the list |
| 7 | **Confirm deletes all selected**: confirming removes all selected certs from the table |

The "confirm deletes" test awaits the success or failure toast appearance (toast appearance
confirms all requests have settled via `Promise.allSettled`) before asserting the cert
names are no longer visible in the table.

---

## 5. Backend Considerations

### 5.1 No New Endpoint

The existing `DELETE /api/v1/certificates/:id` route at
`backend/internal/api/routes/routes.go:673` is the only backend touch point. Bulk deletion
is orchestrated entirely in the frontend using `Promise.allSettled`. This is intentional:

- The volume of certificates eligible for bulk deletion is small in practice.
- Each deletion independently creates a server-side backup. A batch endpoint would need
  N individual backups anyway, yielding no efficiency gain.
- Concurrent `Promise.allSettled` provides natural per-item error isolation — a 409 on
  one cert (race: cert becomes in-use between checkbox selection and confirmation) surfaces
  as a failed count in the toast rather than an unhandled rejection.

### 5.2 No Backend Tests Required

The handler tests added in the single-delete PR already cover: success, in-use 409, auth
guard, invalid ID, not-found, and backup-failure paths. Bulk deletion calls the same handler
N times with no new code paths. Nothing new at the backend layer warrants new tests.

---

## 6. Security Considerations

- Bulk delete inherits all security properties of the individual delete endpoint:
  authentication required, in-use guard server-side, numeric ID validation in the handler.
- The client-side `isDeletable` check is a UX gate, not a security gate; the server is the
  authoritative enforcer.
- `Promise.allSettled` does not short-circuit — a 409 on one cert becomes a failed count,
  not an unhandled exception, preserving the remaining deletions.

---

## 7. Commit Slicing Strategy

**Single PR.** The entire feature — `BulkDeleteCertificateDialog`, i18n keys in 5 locales,
selection layer in `CertificateList.tsx`, unit tests, and E2E tests — is one cohesive
change. The diff is small (< 400 lines of production code across ~6 files) and all parts
are interdependent. Splitting would temporarily ship a broken feature mid-PR.

Suggested commit title:
```
feat(certificates): add bulk deletion with checkbox selection
```

---

## 8. File Change Summary

| File | Change | Description |
|------|--------|-------------|
| `frontend/src/components/CertificateList.tsx` | Modified | Selection state, checkbox column, bulk toolbar, bulk mutation |
| `frontend/src/components/dialogs/BulkDeleteCertificateDialog.tsx` | Created | New dialog for bulk confirmation |
| `frontend/src/locales/en/translation.json` | Modified | 10 new i18n keys under `certificates` |
| `frontend/src/locales/de/translation.json` | Modified | 10 new i18n keys (DE) |
| `frontend/src/locales/es/translation.json` | Modified | 10 new i18n keys (ES) |
| `frontend/src/locales/fr/translation.json` | Modified | 10 new i18n keys (FR) |
| `frontend/src/locales/zh/translation.json` | Modified | 10 new i18n keys (ZH) |
| `frontend/src/components/__tests__/CertificateList.test.tsx` | Modified | 7 new unit test cases |
| `frontend/src/components/dialogs/__tests__/BulkDeleteCertificateDialog.test.tsx` | Created | 6 unit test cases for the new dialog |
| `tests/certificate-bulk-delete.spec.ts` | Created | E2E test suite (7 scenarios) |

**Dependencies**: None — the backend API is already complete. No database migrations.

**Validation Gates**:
- `npx vitest run` — all existing and new unit tests pass
- Playwright E2E on Firefox — all 7 new scenarios pass
- `npx tsc --noEmit` — 0 errors
- `make lint-fast` — 0 new warnings
