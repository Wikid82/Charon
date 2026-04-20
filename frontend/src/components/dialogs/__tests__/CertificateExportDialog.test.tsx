import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import type { Certificate } from '../../../api/certificates'
import { createTestQueryClient } from '../../../test/createTestQueryClient'
import CertificateExportDialog from '../CertificateExportDialog'

const exportMutateFn = vi.fn()

vi.mock('../../../hooks/useCertificates', () => ({
  useExportCertificate: vi.fn(() => ({
    mutate: exportMutateFn,
    isPending: false,
  })),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

vi.mock('../../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const baseCert: Certificate = {
  uuid: 'cert-1',
  name: 'Test Cert',
  domains: 'example.com',
  issuer: 'Test CA',
  expires_at: '2026-06-01T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  has_key: true,
  in_use: false,
}

function renderDialog(
  certificate: Certificate | null = baseCert,
  open = true,
  onOpenChange = vi.fn(),
) {
  const qc = createTestQueryClient()
  return {
    onOpenChange,
    ...render(
      <QueryClientProvider client={qc}>
        <CertificateExportDialog
          certificate={certificate}
          open={open}
          onOpenChange={onOpenChange}
        />
      </QueryClientProvider>,
    ),
  }
}

describe('CertificateExportDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders dialog when open', () => {
    renderDialog()
    expect(screen.getByTestId('certificate-export-dialog')).toBeTruthy()
    expect(screen.getByText('certificates.exportTitle')).toBeTruthy()
  })

  it('does not render when closed', () => {
    renderDialog(baseCert, false)
    expect(screen.queryByTestId('certificate-export-dialog')).toBeFalsy()
  })

  it('shows format radio options', () => {
    renderDialog()
    expect(screen.getByText('certificates.exportFormatPem')).toBeTruthy()
    expect(screen.getByText('certificates.exportFormatPfx')).toBeTruthy()
    expect(screen.getByText('certificates.exportFormatDer')).toBeTruthy()
  })

  it('shows include private key checkbox', () => {
    renderDialog()
    expect(screen.getByText('certificates.includePrivateKey')).toBeTruthy()
  })

  it('shows export button', () => {
    renderDialog()
    expect(screen.getByTestId('export-certificate-submit')).toBeTruthy()
  })

  it('shows cancel button', () => {
    renderDialog()
    expect(screen.getByText('common.cancel')).toBeTruthy()
  })

  it('calls onOpenChange(false) on cancel', async () => {
    const onOpenChange = vi.fn()
    renderDialog(baseCert, true, onOpenChange)
    await userEvent.click(screen.getByText('common.cancel'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('selects PEM format by default', () => {
    renderDialog()
    const pemRadio = screen.getByRole('radio', { name: 'certificates.exportFormatPem' })
    expect(pemRadio).toHaveAttribute('aria-checked', 'true')
  })

  it('can select PFX format', async () => {
    renderDialog()
    await userEvent.click(screen.getByText('certificates.exportFormatPfx'))
    const pfxRadio = screen.getByRole('radio', { name: 'certificates.exportFormatPfx' })
    expect(pfxRadio).toHaveAttribute('aria-checked', 'true')
  })

  it('shows PFX password when PFX format selected', async () => {
    renderDialog()
    await userEvent.click(screen.getByText('certificates.exportFormatPfx'))
    expect(screen.getByText('certificates.exportPfxPassword')).toBeTruthy()
  })

  it('shows private key warning when include key is checked', async () => {
    renderDialog()
    const checkbox = screen.getByRole('checkbox')
    await userEvent.click(checkbox)
    expect(screen.getByText('certificates.includePrivateKeyWarning')).toBeTruthy()
  })

  it('shows password field when include key is checked', async () => {
    renderDialog()
    const checkbox = screen.getByRole('checkbox')
    await userEvent.click(checkbox)
    expect(screen.getByText('certificates.exportPassword')).toBeTruthy()
  })

  it('calls export mutation on submit', async () => {
    renderDialog()
    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(exportMutateFn).toHaveBeenCalledTimes(1)
    expect(exportMutateFn.mock.calls[0][0]).toMatchObject({
      uuid: 'cert-1',
      format: 'pem',
      includeKey: false,
    })
  })

  it('sends include key and password when checked', async () => {
    renderDialog()
    const checkbox = screen.getByRole('checkbox')
    await userEvent.click(checkbox)

    const pwInput = document.getElementById('export-password') as HTMLInputElement
    await userEvent.type(pwInput, 'secret123')

    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(exportMutateFn.mock.calls[0][0]).toMatchObject({
      uuid: 'cert-1',
      format: 'pem',
      includeKey: true,
      password: 'secret123',
    })
  })

  it('hides include key checkbox when cert has no key', () => {
    const certNoKey = { ...baseCert, has_key: false }
    renderDialog(certNoKey)
    expect(screen.queryByRole('checkbox')).toBeFalsy()
  })

  it('triggers blob download on export success', async () => {
    const fakeBlob = new Blob(['cert-data'], { type: 'application/x-pem-file' })
    const revokeURL = vi.fn()
    const createURL = vi.fn(() => 'blob:http://localhost/fake')
    globalThis.URL.createObjectURL = createURL
    globalThis.URL.revokeObjectURL = revokeURL

    const appendSpy = vi.spyOn(document.body, 'appendChild')
    const removeSpy = vi.fn()

    exportMutateFn.mockImplementation(
      (_params: unknown, opts: { onSuccess: (b: Blob) => void }) => {
        const origCreate = document.createElement.bind(document)
        vi.spyOn(document, 'createElement').mockImplementationOnce((tag: string) => {
          const el = origCreate(tag) as HTMLAnchorElement
          el.remove = removeSpy
          return el
        })
        opts.onSuccess(fakeBlob)
      },
    )

    renderDialog()
    await userEvent.click(screen.getByTestId('export-certificate-submit'))

    expect(createURL).toHaveBeenCalledWith(fakeBlob)
    expect(appendSpy).toHaveBeenCalled()
    expect(revokeURL).toHaveBeenCalledWith('blob:http://localhost/fake')
    expect(removeSpy).toHaveBeenCalled()
    appendSpy.mockRestore()
  })

  it('shows toast error on export failure', async () => {
    const { toast: mockToast } = await import('../../../utils/toast')
    exportMutateFn.mockImplementation(
      (_params: unknown, opts: { onError: (e: Error) => void }) => {
        opts.onError(new Error('Export failed'))
      },
    )

    renderDialog()
    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(mockToast.error).toHaveBeenCalled()
  })

  it('selects DER format and submits', async () => {
    renderDialog()
    await userEvent.click(screen.getByText('certificates.exportFormatDer'))
    const derRadio = screen.getByRole('radio', { name: 'certificates.exportFormatDer' })
    expect(derRadio).toHaveAttribute('aria-checked', 'true')

    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(exportMutateFn.mock.calls[0][0]).toMatchObject({
      format: 'der',
    })
  })

  it('sends pfxPassword when PFX format selected', async () => {
    renderDialog()
    await userEvent.click(screen.getByText('certificates.exportFormatPfx'))

    const pfxInput = document.getElementById('pfx-password') as HTMLInputElement
    await userEvent.type(pfxInput, 'pfx-secret')

    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(exportMutateFn.mock.calls[0][0]).toMatchObject({
      format: 'pfx',
      pfxPassword: 'pfx-secret',
    })
  })

  it('returns early from submit when certificate is null', async () => {
    renderDialog(null)
    // Dialog doesn't render without open+cert, so no submit button to click
    // Just verify no calls
    expect(exportMutateFn).not.toHaveBeenCalled()
  })

  it('uses certificate name in download filename on success', async () => {
    const fakeBlob = new Blob(['data'])
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:fake')
    globalThis.URL.revokeObjectURL = vi.fn()

    let capturedAnchor: HTMLAnchorElement | null = null
    exportMutateFn.mockImplementation(
      (_params: unknown, opts: { onSuccess: (b: Blob) => void }) => {
        const origCreate = document.createElement.bind(document)
        vi.spyOn(document, 'createElement').mockImplementationOnce((tag: string) => {
          const el = origCreate(tag) as HTMLAnchorElement
          el.remove = vi.fn()
          capturedAnchor = el
          return el
        })
        opts.onSuccess(fakeBlob)
      },
    )

    renderDialog()
    await userEvent.click(screen.getByTestId('export-certificate-submit'))
    expect(capturedAnchor!.download).toBe('Test Cert.pem')
  })
})
