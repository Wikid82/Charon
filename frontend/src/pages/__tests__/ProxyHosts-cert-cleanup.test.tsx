import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import type { ProxyHost, Certificate } from '../../api/proxyHosts'
import ProxyHosts from '../ProxyHosts'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as certificatesApi from '../../api/certificates'
import * as accessListsApi from '../../api/accessLists'
import * as settingsApi from '../../api/settings'
import * as uptimeApi from '../../api/uptime'
import * as backupsApi from '../../api/backups'
import { createMockProxyHost } from '../../testUtils/createMockProxyHost'

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn()
  }
}))

vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
  createProxyHost: vi.fn(),
  updateProxyHost: vi.fn(),
  deleteProxyHost: vi.fn(),
  bulkUpdateACL: vi.fn(),
  testProxyHostConnection: vi.fn(),
}))

vi.mock('../../api/certificates', () => ({
  getCertificates: vi.fn(),
  deleteCertificate: vi.fn(),
}))
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }))
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }))
vi.mock('../../api/backups', () => ({ createBackup: vi.fn() }))
vi.mock('../../api/uptime', () => ({ getMonitors: vi.fn() }))
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

const createQueryClient = () => new QueryClient({
  defaultOptions: {
    queries: { retry: false, gcTime: 0 },
    mutations: { retry: false }
  }
})

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

const baseHost = (overrides: Partial<ProxyHost> = {}) => createMockProxyHost(overrides)

describe('ProxyHosts - Certificate Cleanup Prompts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([])
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.db' })
  })

  it('prompts to delete certificate when deleting proxy host with unique custom cert', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'CustomCert',
      domains: 'test.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({
      uuid: 'h1',
      name: 'Host1',
      certificate_id: 1,
      certificate: cert
    })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.deleteCertificate).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Click row delete button
    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // First dialog appears - "Delete Proxy Host?" confirmation
    await waitFor(() => {
      expect(screen.getByText('Delete Proxy Host?')).toBeTruthy()
    })

    // Click "Delete" in the confirmation dialog to proceed
    const confirmDelete = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDelete[confirmDelete.length - 1])

    // Now Certificate cleanup dialog should appear (custom modal, not Radix)
    await waitFor(() => {
      expect(screen.getByText(/orphaned certificate/i)).toBeTruthy()
      expect(screen.getByText('CustomCert')).toBeTruthy()
    })

    // Find the native checkbox by id="delete_certs"
    const checkbox = document.getElementById('delete_certs') as HTMLInputElement
    expect(checkbox).toBeTruthy()
    expect(checkbox.checked).toBe(false)

    // Check the checkbox to delete certificate
    await userEvent.click(checkbox)

    // Confirm deletion in the CertificateCleanupDialog
    const submitButton = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(submitButton[submitButton.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(certificatesApi.deleteCertificate).toHaveBeenCalledWith(1)
    })
  })

  it('does NOT prompt for certificate deletion when cert is shared by multiple hosts', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'SharedCert',
      domains: 'shared.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host1 = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })
    const host2 = baseHost({ uuid: 'h2', name: 'Host2', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    const deleteButtons = screen.getAllByRole('button', { name: /delete/i })
    await userEvent.click(deleteButtons[0])

    // Should show standard confirmation dialog (not cert cleanup)
    await waitFor(() => {
      expect(screen.getByText(/Delete Proxy Host\?/)).toBeTruthy()
    })

    // There should NOT be an orphaned certificate checkbox since cert is still used by Host2
    expect(screen.queryByText(/orphaned certificate/i)).toBeNull()

    // Click Delete to confirm
    const confirmDeleteBtn = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDeleteBtn[confirmDeleteBtn.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
    })
    expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
  })

  it('does NOT prompt for production Let\'s Encrypt certificates', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'letsencrypt',
      name: 'LE Prod',
      domains: 'prod.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // Should show standard confirmation dialog (not cert cleanup with orphan checkbox)
    await waitFor(() => {
      expect(screen.getByText(/Delete Proxy Host\?/)).toBeTruthy()
    })

    // There should NOT be an orphaned certificate option for production Let's Encrypt
    expect(screen.queryByText(/orphaned certificate/i)).toBeNull()

    // Click Delete to confirm
    const confirmDeleteBtn = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDeleteBtn[confirmDeleteBtn.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
    })
    expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
  })

  it('prompts for staging certificates', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'letsencrypt-staging',
      name: 'Staging Cert',
      domains: 'staging.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Click row delete button
    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // First dialog appears - "Delete Proxy Host?" confirmation
    await waitFor(() => {
      expect(screen.getByText('Delete Proxy Host?')).toBeTruthy()
    })

    // Click "Delete" in the confirmation dialog to proceed
    const confirmDelete = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDelete[confirmDelete.length - 1])

    // Certificate cleanup dialog should appear for staging certs
    await waitFor(() => {
      expect(screen.getByText(/orphaned certificate/i)).toBeTruthy()
    })

    // Decline certificate deletion (click Delete without checking the box)
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
    })
  })

  it('handles certificate deletion failure gracefully', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'CustomCert',
      domains: 'custom.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.deleteCertificate).mockRejectedValue(
      new Error('Certificate is still in use')
    )

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Click row delete button
    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // First dialog appears
    await waitFor(() => expect(screen.getByText('Delete Proxy Host?')).toBeTruthy())

    // Click "Delete" in the confirmation dialog
    const confirmDelete = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDelete[confirmDelete.length - 1])

    // Certificate cleanup dialog should appear
    await waitFor(() => expect(screen.getByText(/orphaned certificate/i)).toBeTruthy())

    // Check the certificate deletion checkbox
    const checkbox = document.getElementById('delete_certs') as HTMLInputElement
    await userEvent.click(checkbox)

    // Confirm deletion
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(certificatesApi.deleteCertificate).toHaveBeenCalledWith(1)
    })

    // Toast should show error about certificate but host was deleted
    const toast = await import('react-hot-toast')
    await waitFor(() => {
      expect(toast.toast.error).toHaveBeenCalledWith(
        expect.stringContaining('failed to delete certificate')
      )
    })
  })

  it('bulk delete prompts for orphaned certificates', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'BulkCert',
      domains: 'bulk.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host1 = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })
    const host2 = baseHost({ uuid: 'h2', name: 'Host2', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.deleteCertificate).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Select all hosts
    const selectAllCheckbox = screen.getByLabelText('Select all rows')
    await userEvent.click(selectAllCheckbox)

    // Click bulk delete button (the delete button in the toolbar, after Manage ACL)
    await waitFor(() => expect(screen.getByText('Manage ACL')).toBeTruthy())
    const manageACLButton = screen.getByText('Manage ACL')
    const bulkDeleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement
    await userEvent.click(bulkDeleteButton)

    // Confirm in bulk delete modal - text uses pluralized form "Proxy Host(s)"
    await waitFor(() => expect(screen.getByText(/Delete 2 Proxy Host/)).toBeTruthy())
    const deletePermBtn = screen.getByRole('button', { name: /Delete Permanently/i })
    await userEvent.click(deletePermBtn)

    // Should show certificate cleanup dialog (both hosts use same cert, deleting both = orphaned)
    await waitFor(() => {
      expect(screen.getByText(/orphaned certificate/i)).toBeTruthy()
      expect(screen.getByText('BulkCert')).toBeTruthy()
    })

    // Check the certificate deletion checkbox
    const certCheckbox = document.getElementById('delete_certs') as HTMLInputElement
    await userEvent.click(certCheckbox)

    // Confirm
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h2')
      expect(certificatesApi.deleteCertificate).toHaveBeenCalledWith(1)
    })
  })

  it('bulk delete does NOT prompt when certificate is still used by other hosts', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'SharedCert',
      domains: 'shared.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host1 = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })
    const host2 = baseHost({ uuid: 'h2', name: 'Host2', certificate_id: 1, certificate: cert })
    const host3 = baseHost({ uuid: 'h3', name: 'Host3', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2, host3])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Select only host1 and host2 (host3 still uses the cert)
    const host1Row = screen.getByText('Host1').closest('tr') as HTMLTableRowElement
    const host2Row = screen.getByText('Host2').closest('tr') as HTMLTableRowElement
    // Get the Radix Checkbox in each row (first checkbox, not the Switch which is input[type=checkbox].sr-only)
    const host1Checkbox = within(host1Row).getByLabelText(/Select row h1/)
    const host2Checkbox = within(host2Row).getByLabelText(/Select row h2/)

    await userEvent.click(host1Checkbox)
    await userEvent.click(host2Checkbox)

    // Wait for bulk operations to be available
    await waitFor(() => expect(screen.getByText('Bulk Apply')).toBeTruthy())

    // Click bulk delete - find the delete button in the toolbar (after Manage ACL)
    const manageACLButton = screen.getByText('Manage ACL')
    const bulkDeleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement
    await userEvent.click(bulkDeleteButton)

    // Confirm in modal - text uses pluralized form "Proxy Host(s)"
    await waitFor(() => expect(screen.getByText(/Delete 2 Proxy Host/)).toBeTruthy())
    const deletePermBtn = screen.getByRole('button', { name: /Delete Permanently/i })
    await userEvent.click(deletePermBtn)

    // Should NOT show certificate cleanup dialog (host3 still uses it)
    // It will directly delete without showing the orphaned cert dialog
    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h2')
      expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
    })
  })

  it('allows cancelling certificate cleanup dialog', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'CustomCert',
      domains: 'test.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // Certificate cleanup dialog appears
    await waitFor(() => expect(screen.getByText('Delete Proxy Host?')).toBeTruthy())

    // Click Cancel
    const cancelBtn = screen.getByRole('button', { name: 'Cancel' })
    await userEvent.click(cancelBtn)

    // Dialog should close, nothing deleted
    await waitFor(() => {
      expect(screen.queryByText('Delete Proxy Host?')).toBeFalsy()
      expect(proxyHostsApi.deleteProxyHost).not.toHaveBeenCalled()
      expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
    })
  })

  it('default state is unchecked for certificate deletion (conservative)', async () => {
    const cert: Certificate = {
      id: 1,
      uuid: 'cert-1',
      provider: 'custom',
      name: 'CustomCert',
      domains: 'test.com',
      expires_at: '2026-01-01T00:00:00Z'
    }
    const host = baseHost({ uuid: 'h1', name: 'Host1', certificate_id: 1, certificate: cert })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Host1')).toBeTruthy())

    // Click row delete button
    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // First dialog appears
    await waitFor(() => expect(screen.getByText('Delete Proxy Host?')).toBeTruthy())

    // Click "Delete" in the confirmation dialog
    const confirmDelete = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(confirmDelete[confirmDelete.length - 1])

    // Certificate cleanup dialog should appear
    await waitFor(() => expect(screen.getByText(/orphaned certificate/i)).toBeTruthy())

    // Checkbox should be unchecked by default
    const checkbox = document.getElementById('delete_certs') as HTMLInputElement
    expect(checkbox.checked).toBe(false)

    // Confirm deletion without checking the box
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    await userEvent.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('h1')
      expect(certificatesApi.deleteCertificate).not.toHaveBeenCalled()
    })
  })
})
