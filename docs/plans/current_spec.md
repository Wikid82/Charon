# Certificate Management Enhancement - Execution Plan

**Issue**: The Certificates page has no actions for deleting certificates, and proxy host deletion doesn't prompt about certificate cleanup.

**Date**: December 5, 2025
**Status**: Planning Complete - Ready for Implementation

---

## Overview

This plan implements two related features:
1. **Certificate Deletion Actions**: Add delete buttons to the Certificates page actions column for expired/unused certificates
2. **Proxy Host Deletion Certificate Prompt**: When deleting a proxy host, prompt user to confirm deletion of the associated certificate (default: No)

Both features prioritize user safety with confirmation dialogs, automatic backups, and sensible defaults.

---

## Architecture Analysis

### Current State

**Backend**:
- Certificate model: `backend/internal/models/ssl_certificate.go` - SSLCertificate with ID, UUID, Name, Provider, Domains, etc.
- ProxyHost model: `backend/internal/models/proxy_host.go` - Has `CertificateID *uint` (nullable foreign key) and `Certificate *SSLCertificate` relationship
- Certificate service: `backend/internal/services/certificate_service.go`
  - Already has `DeleteCertificate(id uint) error` method
  - Already has `IsCertificateInUse(id uint) (bool, error)` - checks if cert is linked to any ProxyHost
  - Returns `ErrCertInUse` error if certificate is in use
- Certificate handler: `backend/internal/api/handlers/certificate_handler.go`
  - Already has `Delete(c *gin.Context)` endpoint at `DELETE /api/v1/certificates/:id`
  - Creates backup before deletion (if backupService available)
  - Checks if certificate is in use and returns 409 Conflict if so
  - Returns appropriate error messages

**Frontend**:
- CertificateList component: `frontend/src/components/CertificateList.tsx`
  - Already checks if certificate is in use: `hosts.some(h => h.certificate_id === cert.id)`
  - Already has delete button for custom and staging certificates
  - Already shows appropriate confirmation messages
  - Already creates backup before deletion
- ProxyHostForm: `frontend/src/components/ProxyHostForm.tsx`
  - Certificate selector with dropdown showing available certificates
  - No certificate deletion logic on proxy host deletion
- ProxyHosts page: `frontend/src/pages/ProxyHosts.tsx`
  - Delete handler calls `deleteHost(uuid, deleteUptime?)`
  - Currently prompts about uptime monitors but not certificates

**Key Relationships**:
- One certificate can be used by multiple proxy hosts (one-to-many)
- Proxy hosts can have no certificate (certificate_id is nullable)
- Backend prevents deletion of certificates in use (409 Conflict)
- Frontend already checks usage and blocks deletion

---

## Backend Requirements

### Current Implementation is COMPLETE ✅

The backend already has all required functionality:

1. ✅ **DELETE /api/v1/certificates/:id** endpoint exists
2. ✅ Certificate usage validation (`IsCertificateInUse`)
3. ✅ Backup creation before deletion
4. ✅ Proper error responses (400, 404, 409, 500)
5. ✅ Notification service integration
6. ✅ GORM relationship handling

**No backend changes required** - the API fully supports certificate deletion with proper validation.

### Proxy Host Deletion - No Changes Needed

The proxy host deletion endpoint (`DELETE /api/v1/proxy-hosts/:uuid`) already:
- Deletes the proxy host
- GORM cascade rules handle the relationship cleanup
- Does NOT delete the certificate (certificate is shared resource)

This is **correct behavior** - certificates should not be auto-deleted when a proxy host is removed, as they may be:
- Used by other proxy hosts
- Reusable for future proxy hosts
- Auto-managed by Let's Encrypt (shouldn't be manually deleted)

**Frontend will handle certificate cleanup prompting** - no backend API changes needed.

---

## Frontend Requirements

### 1. Certificate Actions Column (Already Working)

**Status**: ✅ **IMPLEMENTED** in `frontend/src/components/CertificateList.tsx`

The actions column already shows delete buttons for:
- Custom certificates (`cert.provider === 'custom'`)
- Staging certificates (`cert.issuer?.toLowerCase().includes('staging')`)

The delete logic already:
- Checks if certificate is in use by proxy hosts
- Shows appropriate confirmation messages
- Creates backup before deletion
- Handles errors properly

**Current implementation is correct and complete.**

### 2. Proxy Host Deletion Certificate Prompt (NEW FEATURE)

**File**: `frontend/src/pages/ProxyHosts.tsx`
**Location**: `handleDelete` function (lines ~119-162)

**Required Changes**:

1. **Update `handleDelete` function** to check for associated certificates:
   ```tsx
   const handleDelete = async (uuid: string) => {
     const host = hosts.find(h => h.uuid === uuid)
     if (!host) return

     if (!confirm('Are you sure you want to delete this proxy host?')) return

     try {
       // Check for uptime monitors (existing code)
       let associatedMonitors: UptimeMonitor[] = []
       // ... existing uptime monitor logic ...

       // NEW: Check for associated certificate
       let shouldDeleteCert = false
       if (host.certificate_id && host.certificate) {
         const cert = host.certificate

         // Check if this is the ONLY proxy host using this certificate
         const otherHostsUsingCert = hosts.filter(h =>
           h.uuid !== uuid && h.certificate_id === host.certificate_id
         ).length

         if (otherHostsUsingCert === 0) {
           // This is the only host using the certificate
           // Only prompt for custom/staging certs (not production Let's Encrypt)
           if (cert.provider === 'custom' || cert.issuer?.toLowerCase().includes('staging')) {
             shouldDeleteCert = confirm(
               `This proxy host uses certificate "${cert.name || cert.domain}". ` +
               `Do you want to delete the certificate as well?\n\n` +
               `Click "Cancel" to keep the certificate (default).`
             )
           }
         }
       }

       // Delete uptime monitors if confirmed (existing)
       if (associatedMonitors.length > 0) {
         const deleteUptime = confirm('...')
         await deleteHost(uuid, deleteUptime)
       } else {
         await deleteHost(uuid)
       }

       // NEW: Delete certificate if user confirmed
       if (shouldDeleteCert && host.certificate_id) {
         try {
           await deleteCertificate(host.certificate_id)
           toast.success('Proxy host and certificate deleted')
         } catch (err) {
           // Host is already deleted, just log cert deletion failure
           toast.error(`Proxy host deleted but failed to delete certificate: ${err instanceof Error ? err.message : 'Unknown error'}`)
         }
       }
     } catch (err) {
       alert(err instanceof Error ? err.message : 'Failed to delete')
     }
   }
   ```

2. **Import required API function**:
   ```tsx
   import { deleteCertificate } from '../api/certificates'
   ```

3. **UI/UX Considerations**:
   - Show certificate prompt AFTER proxy host deletion confirmation
   - Default is "No" (Cancel button) - safer option
   - Only prompt for custom/staging certificates (not production Let's Encrypt)
   - Only prompt if this is the ONLY host using the certificate
   - Certificate deletion happens AFTER host deletion (host must be removed first to pass backend validation)
   - Show appropriate toast messages for both actions

### 3. Bulk Proxy Host Deletion (Enhancement)

**File**: `frontend/src/pages/ProxyHosts.tsx`
**Location**: `handleBulkDelete` function (lines ~204-242)

**Required Changes** (similar pattern):

```tsx
const handleBulkDelete = async () => {
  const hostUUIDs = Array.from(selectedHosts)
  setIsCreatingBackup(true)

  try {
    // Create automatic backup (existing)
    toast.loading('Creating backup before deletion...')
    const backup = await createBackup()
    toast.dismiss()
    toast.success(`Backup created: ${backup.filename}`)

    // NEW: Collect certificates to potentially delete
    const certsToConsider: Set<number> = new Set()
    hostUUIDs.forEach(uuid => {
      const host = hosts.find(h => h.uuid === uuid)
      if (host?.certificate_id && host.certificate) {
        const cert = host.certificate
        // Only consider custom/staging certs
        if (cert.provider === 'custom' || cert.issuer?.toLowerCase().includes('staging')) {
          // Check if this cert is ONLY used by hosts being deleted
          const otherHosts = hosts.filter(h =>
            h.certificate_id === host.certificate_id &&
            !hostUUIDs.includes(h.uuid)
          )
          if (otherHosts.length === 0) {
            certsToConsider.add(host.certificate_id)
          }
        }
      }
    })

    // NEW: Prompt for certificate deletion if any are orphaned
    let shouldDeleteCerts = false
    if (certsToConsider.size > 0) {
      shouldDeleteCerts = confirm(
        `${certsToConsider.size} certificate(s) will no longer be used after deleting these hosts. ` +
        `Do you want to delete the unused certificates as well?\n\n` +
        `Click "Cancel" to keep the certificates (default).`
      )
    }

    // Delete each host (existing)
    let deleted = 0
    let failed = 0
    for (const uuid of hostUUIDs) {
      try {
        await deleteHost(uuid)
        deleted++
      } catch {
        failed++
      }
    }

    // NEW: Delete certificates if user confirmed
    if (shouldDeleteCerts && certsToConsider.size > 0) {
      let certsDeleted = 0
      let certsFailed = 0
      for (const certId of certsToConsider) {
        try {
          await deleteCertificate(certId)
          certsDeleted++
        } catch {
          certsFailed++
        }
      }

      if (certsFailed > 0) {
        toast.error(`Deleted ${deleted} host(s) and ${certsDeleted} certificate(s), ${certsFailed} certificate(s) failed`)
      } else if (certsDeleted > 0) {
        toast.success(`Deleted ${deleted} host(s) and ${certsDeleted} certificate(s)`)
      }
    } else {
      // No certs deleted (existing logic)
      if (failed > 0) {
        toast.error(`Deleted ${deleted} host(s), ${failed} failed`)
      } else {
        toast.success(`Successfully deleted ${deleted} host(s). Backup available for restore.`)
      }
    }

    setSelectedHosts(new Set())
    setShowBulkDeleteModal(false)
  } catch (err) {
    toast.dismiss()
    toast.error('Failed to create backup. Deletion cancelled.')
  } finally {
    setIsCreatingBackup(false)
  }
}
```

---

## Testing Strategy

### Backend Tests (Already Exist) ✅

Location: `backend/internal/api/handlers/certificate_handler_test.go`

Existing tests cover:
- ✅ Delete certificate in use (409 Conflict)
- ✅ Delete certificate not in use (success with backup)
- ✅ Delete invalid ID (400 Bad Request)
- ✅ Delete non-existent certificate (404 Not Found)
- ✅ Delete without backup service (still succeeds)

**No new backend tests required** - coverage is complete.

### Frontend Tests (Need Updates)

#### 1. CertificateList Component Tests ✅
Location: `frontend/src/components/__tests__/CertificateList.test.tsx`

Already has tests for:
- ✅ Delete custom certificate with confirmation
- ✅ Delete staging certificate
- ✅ Block deletion when certificate is in use
- ✅ Block deletion when certificate is active (valid/expiring)

**Current tests are sufficient.**

#### 2. ProxyHosts Component Tests (Need New Tests)
Location: `frontend/src/pages/__tests__/ProxyHosts-coverage.test.tsx`

**New tests required**:

```typescript
describe('ProxyHosts - Certificate Deletion Prompts', () => {
  it('prompts to delete certificate when deleting proxy host with unique custom cert', async () => {
    const cert = { id: 1, provider: 'custom', name: 'CustomCert', domain: 'test.com' }
    const host = baseHost({
      uuid: 'h1',
      name: 'Host1',
      certificate_id: 1,
      certificate: cert
    })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([cert])

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Confirm proxy host deletion
      .mockReturnValueOnce(true)  // Confirm certificate deletion

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    const deleteBtn = screen.getByText('Delete')
    await userEvent.click(deleteBtn)

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(certificatesApi.deleteCertificate).toHaveBeenCalledWith(1)
    })

    confirmSpy.mockRestore()
  })

  it('does NOT prompt for certificate deletion when cert is shared by multiple hosts', async () => {
    const cert = { id: 1, provider: 'custom', name: 'SharedCert' }
    const host1 = baseHost({ uuid: 'h1', certificate_id: 1, certificate: cert })
    const host2 = baseHost({ uuid: 'h2', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Only asked once (proxy host deletion)

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText(host1.name)).toBeTruthy())

    const deleteBtn = screen.getAllByText('Delete')[0]
    await userEvent.click(deleteBtn)

    await waitFor(() => expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1'))
    expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
    expect(confirmSpy).toHaveBeenCalledTimes(1)  // Only proxy host confirmation

    confirmSpy.mockRestore()
  })

  it('does NOT prompt for production Let\'s Encrypt certificates', async () => {
    const cert = { id: 1, provider: 'letsencrypt', issuer: 'Let\'s Encrypt', name: 'LE Prod' }
    const host = baseHost({ uuid: 'h1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Only proxy host deletion

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText(host.name)).toBeTruthy())

    const deleteBtn = screen.getByText('Delete')
    await userEvent.click(deleteBtn)

    expect(confirmSpy).toHaveBeenCalledTimes(1)  // No cert prompt
    expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()

    confirmSpy.mockRestore()
  })

  it('prompts for staging certificates', async () => {
    const cert = {
      id: 1,
      provider: 'letsencrypt-staging',
      issuer: 'Let\'s Encrypt Staging',
      name: 'Staging Cert'
    }
    const host = baseHost({ uuid: 'h1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Proxy host deletion
      .mockReturnValueOnce(false) // Decline certificate deletion (default)

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText(host.name)).toBeTruthy())

    const deleteBtn = screen.getByText('Delete')
    await userEvent.click(deleteBtn)

    await waitFor(() => expect(confirmSpy).toHaveBeenCalledTimes(2))
    expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()

    confirmSpy.mockRestore()
  })

  it('handles certificate deletion failure gracefully', async () => {
    const cert = { id: 1, provider: 'custom', name: 'CustomCert' }
    const host = baseHost({ uuid: 'h1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.deleteCertificate).mockRejectedValue(
      new Error('Certificate is still in use')
    )

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Proxy host
      .mockReturnValueOnce(true)  // Certificate

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText(host.name)).toBeTruthy())

    const deleteBtn = screen.getByText('Delete')
    await userEvent.click(deleteBtn)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringContaining('failed to delete certificate')
      )
    })

    confirmSpy.mockRestore()
  })

  it('bulk delete prompts for orphaned certificates', async () => {
    const cert = { id: 1, provider: 'custom', name: 'BulkCert' }
    const host1 = baseHost({ uuid: 'h1', certificate_id: 1, certificate: cert })
    const host2 = baseHost({ uuid: 'h2', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.db' })

    const confirmSpy = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)  // Confirm bulk delete modal
      .mockReturnValueOnce(true)  // Confirm certificate deletion

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText(host1.name)).toBeTruthy())

    // Select both hosts
    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])  // Select all

    // Click bulk delete
    const bulkDeleteBtn = screen.getByText('Delete')
    await userEvent.click(bulkDeleteBtn)

    // Confirm in modal
    await userEvent.click(screen.getByText('Delete Permanently'))

    await waitFor(() => {
      expect(confirmSpy).toHaveBeenCalledTimes(2)
      expect(certificatesApi.deleteCertificate).toHaveBeenCalledWith(1)
    })

    confirmSpy.mockRestore()
  })
})
```

#### 3. Integration Tests

**Manual Testing Checklist**:
- [ ] Delete custom certificate from Certificates page
- [ ] Attempt to delete certificate in use (should show error)
- [ ] Delete proxy host with unique custom certificate (should prompt)
- [ ] Delete proxy host with shared certificate (should NOT prompt)
- [ ] Delete proxy host with production Let's Encrypt cert (should NOT prompt)
- [ ] Delete proxy host with staging certificate (should prompt)
- [ ] Decline certificate deletion (default) - only host deleted
- [ ] Accept certificate deletion - both deleted
- [ ] Bulk delete hosts with orphaned certificates
- [ ] Verify backups are created before deletions
- [ ] Check certificate deletion failure doesn't block host deletion

---

## Security Considerations

### Authorization
- ✅ All certificate endpoints protected by authentication middleware
- ✅ All proxy host endpoints protected by authentication middleware
- ✅ Only authenticated users can delete resources

### Validation
- ✅ Backend validates certificate not in use before deletion (409 Conflict)
- ✅ Backend validates certificate ID is numeric and exists (400/404)
- ✅ Frontend checks certificate usage before allowing deletion
- ✅ Frontend validates proxy host UUID before deletion

### Data Protection
- ✅ Automatic backup created before all deletions
- ✅ Soft deletes NOT used (certificates are fully removed)
- ✅ File system cleanup for Let's Encrypt certificates
- ✅ Database cascade rules properly configured

### User Safety
- ✅ Confirmation dialogs required for all deletions
- ✅ Certificate deletion default is "No" (safer)
- ✅ Clear messaging about what will be deleted
- ✅ Descriptive toast messages for success/failure
- ✅ Only prompt for custom/staging certs (not production)
- ✅ Only prompt when certificate is orphaned (no other hosts)

### Error Handling
- ✅ Graceful handling of certificate deletion failures
- ✅ Host deletion succeeds even if cert deletion fails
- ✅ Appropriate error messages shown to user
- ✅ Failed deletions don't block other operations

---

## Implementation Order

### Phase 1: Certificate Actions Column ✅
**Status**: COMPLETE - Already implemented correctly

No changes needed.

### Phase 2: Single Proxy Host Deletion Certificate Prompt
**Priority**: HIGH
**Estimated Time**: 2 hours

1. Update `frontend/src/pages/ProxyHosts.tsx`:
   - Modify `handleDelete` function to check for certificates
   - Add certificate deletion prompt logic
   - Handle certificate deletion after host deletion
   - Import `deleteCertificate` API function

2. Write unit tests in `frontend/src/pages/__tests__/ProxyHosts-coverage.test.tsx`:
   - Test certificate prompt for unique custom cert
   - Test no prompt for shared certificate
   - Test no prompt for production Let's Encrypt
   - Test prompt for staging certificates
   - Test default "No" behavior
   - Test certificate deletion failure handling

3. Manual testing:
   - Test all scenarios in Testing Strategy checklist
   - Verify toast messages are clear
   - Verify backups are created
   - Test error cases

### Phase 3: Bulk Proxy Host Deletion Certificate Prompt
**Priority**: MEDIUM
**Estimated Time**: 2 hours

1. Update `frontend/src/pages/ProxyHosts.tsx`:
   - Modify `handleBulkDelete` function
   - Add logic to identify orphaned certificates
   - Add certificate deletion prompt
   - Handle bulk certificate deletion

2. Write unit tests:
   - Test bulk deletion with orphaned certificates
   - Test bulk deletion with shared certificates
   - Test mixed scenarios

3. Manual testing:
   - Bulk delete scenarios
   - Multiple certificate handling
   - Error recovery

### Phase 4: Documentation & Polish
**Priority**: LOW
**Estimated Time**: 1 hour

1. Update `docs/features.md`:
   - Document certificate deletion feature
   - Document proxy host certificate cleanup

2. Update `docs/api.md` (if needed):
   - Verify certificate deletion endpoint documented

3. Code review:
   - Review all changes
   - Ensure consistent error messages
   - Verify test coverage

---

## Risk Assessment

### Low Risk ✅
- Backend API already exists and is well-tested
- Certificate deletion already works correctly
- Backup system already in place
- Frontend certificate list already handles deletion

### Medium Risk ⚠️
- User confusion about certificate deletion prompts
  - **Mitigation**: Clear messaging, sensible defaults (No), only prompt for custom/staging
- Race conditions with shared certificates
  - **Mitigation**: Check certificate usage at deletion time (backend validation)
- Certificate deletion failure after host deleted
  - **Mitigation**: Graceful error handling, informative toast messages

### No Risk ❌
- Data loss: Backups created before all deletions
- Accidental deletion: Multiple confirmations required
- Production certs: Never prompted for deletion

---

## Success Criteria

### Must Have ✅
1. Certificate delete buttons visible in Certificates page actions column
2. Delete buttons only shown for custom and staging certificates
3. Certificate deletion blocked if in use by any proxy host
4. Automatic backup created before certificate deletion
5. Proxy host deletion prompts for certificate cleanup (default: No)
6. Certificate prompt only shown for custom/staging certs
7. Certificate prompt only shown when orphaned (no other hosts)
8. All operations have clear confirmation dialogs
9. All operations show appropriate toast messages
10. Backend validation prevents invalid deletions

### Nice to Have ✨
1. Show certificate usage count in Certificates table
2. Highlight orphaned certificates in the list
3. Batch certificate cleanup tool
4. Certificate expiry warnings before deletion

---

## Open Questions

1. ✅ Should production Let's Encrypt certificates ever be manually deletable?
   - **Answer**: No, they are auto-managed by Caddy

2. ✅ Should certificate deletion be allowed if status is "valid" or "expiring"?
   - **Answer**: Yes, if not in use (user may want to replace)

3. ✅ What happens if certificate deletion fails after host is deleted?
   - **Answer**: Show error toast, certificate remains, user can delete later

4. ✅ Should bulk host deletion prompt for each certificate individually?
   - **Answer**: No, single prompt for all orphaned certificates

---

## Notes

- Certificate deletion is a **shared resource operation** - multiple hosts can use the same certificate
- The backend correctly prevents deletion of in-use certificates (409 Conflict)
- The frontend already has all the UI components and logic needed
- Focus is on **adding prompts** to the proxy host deletion flow
- Default behavior is **conservative** (don't delete certificates) for safety
- Only custom and staging certificates are considered for cleanup
- Production Let's Encrypt certificates should never be manually deleted

---

## Definition of Done

- [ ] Certificate delete buttons visible and functional
- [ ] Proxy host deletion prompts for certificate cleanup
- [ ] All confirmation dialogs use appropriate defaults
- [ ] Unit tests written and passing
- [ ] Manual testing completed
- [ ] Documentation updated
- [ ] Code reviewed
- [ ] No console errors or warnings
- [ ] Pre-commit checks pass
- [ ] Feature tested in local Docker environment

---

**Plan Created**: December 5, 2025
**Plan Author**: Planning Agent (Architect)
**Ready for Implementation**: ✅ YES
