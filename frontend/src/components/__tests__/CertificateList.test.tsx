import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useCertificates, useDeleteCertificate, useBulkDeleteCertificates } from '../../hooks/useCertificates'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import CertificateList, { isDeletable, isInUse } from '../CertificateList'

import type { Certificate } from '../../api/certificates'

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(),
  useCertificateDetail: vi.fn(() => ({ detail: null, isLoading: false })),
  useDeleteCertificate: vi.fn(),
  useBulkDeleteCertificates: vi.fn(),
  useExportCertificate: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}))

vi.mock('../../api/certificates', () => ({
  deleteCertificate: vi.fn(async () => {}),
}))

vi.mock('../../api/backups', () => ({
  createBackup: vi.fn(async () => ({ filename: 'backup-cert' })),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

function renderWithClient(ui: React.ReactNode) {
  const qc = createTestQueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const makeCert = (overrides: Partial<Certificate> = {}): Certificate => ({
  uuid: 'cert-1',
  domains: 'example.com',
  issuer: 'Custom CA',
  expires_at: '2026-03-01T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  has_key: true,
  in_use: false,
  ...overrides,
})

const createCertificatesValue = (overrides: Partial<ReturnType<typeof useCertificates>> = {}) => {
  const certificates: Certificate[] = [
    makeCert({ uuid: 'cert-1', name: 'CustomCert', domains: 'example.com', status: 'expired', in_use: false }),
    makeCert({ uuid: 'cert-2', name: 'LE Staging', domains: 'staging.example.com', issuer: "Let's Encrypt Staging", status: 'untrusted', provider: 'letsencrypt-staging', in_use: false }),
    makeCert({ uuid: 'cert-3', name: 'ActiveCert', domains: 'active.example.com', status: 'valid', in_use: true }),
    makeCert({ uuid: 'cert-4', name: 'UnusedValidCert', domains: 'unused.example.com', status: 'valid', in_use: false }),
    makeCert({ uuid: 'cert-5', name: 'ExpiredLE', domains: 'expired-le.example.com', issuer: "Let's Encrypt", expires_at: '2025-01-01T00:00:00Z', status: 'expired', provider: 'letsencrypt', in_use: false }),
    makeCert({ uuid: 'cert-6', name: 'ValidLE', domains: 'valid-le.example.com', issuer: "Let's Encrypt", expires_at: '2026-12-01T00:00:00Z', status: 'valid', provider: 'letsencrypt', in_use: false }),
  ]

  return {
    certificates,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  }
}

const getRowNames = () =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map(row => row.querySelectorAll('td')[1]?.textContent?.trim() ?? '')

let deleteMutateFn: ReturnType<typeof vi.fn>
let bulkDeleteMutateFn: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.clearAllMocks()
  deleteMutateFn = vi.fn()
  bulkDeleteMutateFn = vi.fn()
  vi.mocked(useCertificates).mockReturnValue(createCertificatesValue())
  vi.mocked(useDeleteCertificate).mockReturnValue({
    mutate: deleteMutateFn,
    isPending: false,
  } as unknown as ReturnType<typeof useDeleteCertificate>)
  vi.mocked(useBulkDeleteCertificates).mockReturnValue({
    mutate: bulkDeleteMutateFn,
    isPending: false,
  } as unknown as ReturnType<typeof useBulkDeleteCertificates>)
})

describe('CertificateList', () => {
  describe('isDeletable', () => {
    it('returns true for custom cert not in use', () => {
      expect(isDeletable(makeCert({ provider: 'custom', in_use: false }))).toBe(true)
    })

    it('returns true for staging cert not in use', () => {
      expect(isDeletable(makeCert({ provider: 'letsencrypt-staging', in_use: false }))).toBe(true)
    })

    it('returns true for expired LE cert not in use', () => {
      expect(isDeletable(makeCert({ provider: 'letsencrypt', status: 'expired', in_use: false }))).toBe(true)
    })

    it('returns false for valid LE cert not in use', () => {
      expect(isDeletable(makeCert({ provider: 'letsencrypt', status: 'valid', in_use: false }))).toBe(false)
    })

    it('returns false for cert in use', () => {
      expect(isDeletable(makeCert({ provider: 'custom', in_use: true }))).toBe(false)
    })

    it('returns true for expiring LE cert not in use', () => {
      expect(isDeletable(makeCert({ provider: 'letsencrypt', status: 'expiring', in_use: false }))).toBe(true)
    })

    it('returns false for expiring LE cert that is in use', () => {
      expect(isDeletable(makeCert({ provider: 'letsencrypt', status: 'expiring', in_use: true }))).toBe(false)
    })
  })

  describe('isInUse', () => {
    it('returns true when cert.in_use is true', () => {
      expect(isInUse(makeCert({ in_use: true }))).toBe(true)
    })

    it('returns false when cert.in_use is false', () => {
      expect(isInUse(makeCert({ in_use: false }))).toBe(false)
    })
  })

  it('renders delete button for deletable certs', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    expect(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' })).toBeInTheDocument()
  })

  it('renders delete button for expired LE cert not in use', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const expiredLeRow = rows.find(r => r.textContent?.includes('ExpiredLE'))!
    expect(within(expiredLeRow).getByRole('button', { name: 'certificates.deleteTitle' })).toBeInTheDocument()
  })

  it('renders aria-disabled delete button for in-use cert', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const activeRow = rows.find(r => r.textContent?.includes('ActiveCert'))!
    const btn = within(activeRow).getByRole('button', { name: 'certificates.deleteTitle' })
    expect(btn).toHaveAttribute('aria-disabled', 'true')
  })

  it('hides delete button for valid production LE cert', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const validLeRow = rows.find(r => r.textContent?.includes('ValidLE'))!
    expect(within(validLeRow).queryByRole('button', { name: 'certificates.deleteTitle' })).not.toBeInTheDocument()
  })

  it('opens dialog and deletes cert on confirm', async () => {
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).getByText('certificates.deleteTitle')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(deleteMutateFn).toHaveBeenCalledWith('cert-1', expect.any(Object)))
  })

  it('does not call createBackup on delete (server handles it)', async () => {
    const { createBackup } = await import('../../api/backups')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(createBackup).not.toHaveBeenCalled())
  })

  it('renders empty state when no certificates exist', async () => {
    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ certificates: [] }))
    renderWithClient(<CertificateList />)
    expect(await screen.findByText('No certificates found.')).toBeInTheDocument()
  })

  it('shows error state when certificate load fails', async () => {
    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ error: new Error('boom') }))
    renderWithClient(<CertificateList />)
    expect(await screen.findByText('Failed to load certificates')).toBeInTheDocument()
  })

  it('clicking disabled delete button for in-use cert does not open dialog', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const activeRow = rows.find(r => r.textContent?.includes('ActiveCert'))!
    const btn = within(activeRow).getByRole('button', { name: 'certificates.deleteTitle' })

    await user.click(btn)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes delete dialog when cancel is clicked', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'common.cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('renders enabled checkboxes for deletable not-in-use certs', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    for (const name of ['CustomCert', 'LE Staging', 'UnusedValidCert', 'ExpiredLE']) {
      const row = rows.find(r => r.textContent?.includes(name))!
      const checkbox = within(row).getByRole('checkbox')
      expect(checkbox).toBeEnabled()
      expect(checkbox).not.toHaveAttribute('aria-disabled', 'true')
    }
  })

  it('renders disabled checkbox for in-use cert', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const activeRow = rows.find(r => r.textContent?.includes('ActiveCert'))!
    const checkboxes = within(activeRow).getAllByRole('checkbox')
    const rowCheckbox = checkboxes[0]
    expect(rowCheckbox).toBeDisabled()
    expect(rowCheckbox).toHaveAttribute('aria-disabled', 'true')
  })

  it('renders no checkbox in valid production LE cert row', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const validLeRow = rows.find(r => r.textContent?.includes('ValidLE'))!
    expect(within(validLeRow).queryByRole('checkbox')).not.toBeInTheDocument()
  })

  it('selecting one cert makes the bulk action toolbar visible', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('checkbox'))
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('header select-all selects only ids 1, 2, 4, 5 (not in-use id 3)', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const headerRow = (await screen.findAllByRole('row'))[0]
    const headerCheckbox = within(headerRow).getByRole('checkbox')
    await user.click(headerCheckbox)
    expect(screen.getByRole('status')).toBeInTheDocument()
    const rows = screen.getAllByRole('row').slice(1)
    const activeRow = rows.find(r => r.textContent?.includes('ActiveCert'))!
    const activeCheckbox = within(activeRow).getByRole('checkbox')
    expect(activeCheckbox).toBeDisabled()
    expect(activeCheckbox).not.toBeChecked()
  })

  it('clicking the toolbar Delete button opens BulkDeleteCertificateDialog', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('confirming in the bulk dialog calls bulk delete for selected UUIDs', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    const stagingRow = rows.find(r => r.textContent?.includes('LE Staging'))!
    await user.click(within(customRow).getByRole('checkbox'))
    await user.click(within(stagingRow).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    await waitFor(() => {
      expect(bulkDeleteMutateFn).toHaveBeenCalledWith(
        expect.arrayContaining(['cert-1', 'cert-2']),
        expect.any(Object),
      )
    })
  })

  it('shows partial failure toast when some bulk deletes fail', async () => {
    const { toast } = await import('../../utils/toast')
    bulkDeleteMutateFn.mockImplementation((_uuids: string[], { onSuccess }: { onSuccess: (data: { succeeded: number; failed: number }) => void }) => {
      onSuccess({ succeeded: 1, failed: 1 })
    })
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    const stagingRow = rows.find(r => r.textContent?.includes('LE Staging'))!
    await user.click(within(customRow).getByRole('checkbox'))
    await user.click(within(stagingRow).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('certificates.bulkDeletePartial'))
  })

  it('clicking header checkbox twice deselects all and hides the bulk action toolbar', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const headerRow = (await screen.findAllByRole('row'))[0]
    const headerCheckbox = within(headerRow).getByRole('checkbox')
    await user.click(headerCheckbox)
    expect(screen.getByRole('status')).toBeInTheDocument()
    await user.click(headerCheckbox)
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument())
  })

  it('sorts certificates by name and expiry when headers are clicked', async () => {
    const certificates: Certificate[] = [
      makeCert({ uuid: 'cert-z', name: 'Zulu', domains: 'z.example.com', expires_at: '2026-03-01T00:00:00Z' }),
      makeCert({ uuid: 'cert-a', name: 'Alpha', domains: 'a.example.com', expires_at: '2026-01-01T00:00:00Z' }),
    ]

    const user = userEvent.setup()

    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ certificates }))
    renderWithClient(<CertificateList />)

    expect(getRowNames()).toEqual(['Alpha', 'Zulu'])

    await user.click(screen.getByText('Expires'))
    expect(getRowNames()).toEqual(['Alpha', 'Zulu'])

    await user.click(screen.getByText('Expires'))
    expect(getRowNames()).toEqual(['Zulu', 'Alpha'])
  })

  it('shows success toast when single delete succeeds', async () => {
    const { toast } = await import('../../utils/toast')
    deleteMutateFn.mockImplementation((_uuid: string, { onSuccess }: { onSuccess: () => void }) => {
      onSuccess()
    })
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('certificates.deleteSuccess'))
  })

  it('shows error toast when single delete fails', async () => {
    const { toast } = await import('../../utils/toast')
    deleteMutateFn.mockImplementation((_uuid: string, { onError }: { onError: (e: Error) => void }) => {
      onError(new Error('Network failure'))
    })
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('certificates.deleteFailed: Network failure'))
  })

  it('shows success toast when all bulk deletes succeed', async () => {
    const { toast } = await import('../../utils/toast')
    bulkDeleteMutateFn.mockImplementation((_uuids: string[], { onSuccess }: { onSuccess: (data: { succeeded: number; failed: number }) => void }) => {
      onSuccess({ succeeded: 2, failed: 0 })
    })
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    await user.click(within(rows.find(r => r.textContent?.includes('CustomCert'))!).getByRole('checkbox'))
    await user.click(within(rows.find(r => r.textContent?.includes('LE Staging'))!).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('certificates.bulkDeleteSuccess'))
  })

  it('shows error toast when bulk delete fails entirely', async () => {
    const { toast } = await import('../../utils/toast')
    bulkDeleteMutateFn.mockImplementation((_uuids: string[], { onError }: { onError: () => void }) => {
      onError()
    })
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    await user.click(within(rows.find(r => r.textContent?.includes('CustomCert'))!).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /certificates\.bulkDeleteButton/i }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('certificates.bulkDeleteFailed'))
  })

  it('opens detail dialog when view button is clicked', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByTestId('view-cert-cert-1'))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('opens export dialog when export button is clicked', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByTestId('export-cert-cert-1'))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('deselects a row checkbox by clicking it a second time', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    const checkbox = within(customRow).getByRole('checkbox')
    await user.click(checkbox)
    expect(screen.getByRole('status')).toBeInTheDocument()
    await user.click(checkbox)
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument())
  })

  it('closes detail dialog via the dialog close button', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByTestId('view-cert-cert-1'))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Close' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('closes export dialog via the cancel button', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByTestId('export-cert-cert-1'))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'common.cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})
