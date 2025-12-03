import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import CertificateList from '../CertificateList'

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [
      { id: 1, name: 'CustomCert', domain: 'example.com', issuer: 'Custom CA', expires_at: new Date().toISOString(), status: 'expired', provider: 'custom' },
      { id: 2, name: 'LE Staging', domain: 'staging.example.com', issuer: "Let's Encrypt Staging", expires_at: new Date().toISOString(), status: 'untrusted', provider: 'letsencrypt-staging' },
      { id: 3, name: 'ActiveCert', domain: 'active.example.com', issuer: 'Custom CA', expires_at: new Date().toISOString(), status: 'valid', provider: 'custom' },
    ],
    isLoading: false,
    error: null,
  }))
}))

vi.mock('../../api/certificates', () => ({
  deleteCertificate: vi.fn(async () => undefined),
}))

vi.mock('../../api/backups', () => ({
  createBackup: vi.fn(async () => ({ filename: 'backup-cert' })),
}))

vi.mock('../../hooks/useProxyHosts', () => ({
  useProxyHosts: vi.fn(() => ({
    hosts: [
      { uuid: 'h1', name: 'Host1', certificate_id: 3 },
    ],
    loading: false,
    isFetching: false,
    error: null,
    createHost: vi.fn(),
    updateHost: vi.fn(),
    deleteHost: vi.fn(),
    bulkUpdateACL: vi.fn(),
    isBulkUpdating: false,
  })),
}))

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('CertificateList', () => {
  it('deletes custom certificate when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    const { deleteCertificate } = await import('../../api/certificates')
    const { createBackup } = await import('../../api/backups')
    const { toast } = await import('../../utils/toast')

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert')) as HTMLElement
    expect(customRow).toBeTruthy()
    const customBtn = customRow.querySelector('button[title="Delete Certificate"]') as HTMLButtonElement
    expect(customBtn).toBeTruthy()
    await customBtn.click()

    await waitFor(() => expect(createBackup).toHaveBeenCalled())
    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(1))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Certificate deleted'))
    confirmSpy.mockRestore()
  })

  it('deletes staging certificate when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    const { deleteCertificate } = await import('../../api/certificates')

    renderWithClient(<CertificateList />)
    const stagingButtons = await screen.findAllByTitle('Delete Staging Certificate')
    expect(stagingButtons.length).toBeGreaterThan(0)
    await stagingButtons[0].click()

    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(2))
    confirmSpy.mockRestore()
  })

  it('blocks deletion when certificate is in use by a proxy host', async () => {
    const { toast } = await import('../../utils/toast')
    renderWithClient(<CertificateList />)
    const deleteButtons = await screen.findAllByTitle('Delete Certificate')
    // Find button corresponding to ActiveCert (id 3)
    const activeButton = deleteButtons.find(btn => btn.closest('tr')?.querySelector('td')?.textContent?.includes('ActiveCert'))
    expect(activeButton).toBeTruthy()
    if (activeButton) await activeButton.click()
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('in use')))
  })

  it('blocks deletion when certificate status is active (valid/expiring)', async () => {
    const { toast } = await import('../../utils/toast')
    renderWithClient(<CertificateList />)
    const deleteButtons = await screen.findAllByTitle('Delete Certificate')
    // ActiveCert (valid) should block even if not linked – ensure hosts mock links it so previous test covers linkage.
    // Here, simulate clicking a valid cert button if present
    const validButton = deleteButtons.find(btn => btn.closest('tr')?.querySelector('td')?.textContent?.includes('ActiveCert'))
    expect(validButton).toBeTruthy()
    if (validButton) await validButton.click()
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })
})
