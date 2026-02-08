import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Certificates from '../Certificates'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import type { Certificate } from '../../api/certificates'
import { uploadCertificate } from '../../api/certificates'
import { toast } from '../../utils/toast'

const translations: Record<string, string> = {
  'certificates.addCertificate': 'Add Certificate',
  'certificates.uploadCertificate': 'Upload Certificate',
  'certificates.friendlyName': 'Friendly Name',
  'certificates.certificatePem': 'Certificate (PEM)',
  'certificates.privateKeyPem': 'Private Key (PEM)',
  'certificates.uploadSuccess': 'Certificate uploaded successfully',
  'certificates.uploadFailed': 'Failed to upload certificate',
  'common.upload': 'Upload',
  'common.cancel': 'Cancel',
}

const t = (key: string, options?: Record<string, unknown>) => {
  const template = translations[key] ?? key

  if (!options) return template

  return Object.entries(options).reduce((acc, [optionKey, optionValue]) => {
    return acc.replace(`{{${optionKey}}}`, String(optionValue))
  }, template)
}

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t,
  }),
}))

vi.mock('../../components/CertificateList', () => ({
  default: () => <div>CertificateList</div>,
}))

vi.mock('../../api/certificates', () => ({
  uploadCertificate: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('Certificates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('uploads certificate and closes dialog on success', async () => {
    const certificate: Certificate = {
      domain: 'example.com',
      issuer: 'Test CA',
      expires_at: '2026-03-01T00:00:00Z',
      status: 'valid',
      provider: 'custom',
    }
    vi.mocked(uploadCertificate).mockResolvedValue(certificate)

    const user = userEvent.setup()
    const { queryClient } = renderWithQueryClient(<Certificates />)
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    await user.click(screen.getByRole('button', { name: t('certificates.addCertificate') }))

    const dialog = await screen.findByRole('dialog', { name: t('certificates.uploadCertificate') })

    const nameInput = within(dialog).getByLabelText(t('certificates.friendlyName')) as HTMLInputElement
    await user.type(nameInput, 'My Cert')
    await waitFor(() => {
      expect(nameInput.value).toBe('My Cert')
    })

    const certFile = new File(['cert'], 'cert.pem', { type: 'application/x-pem-file' })
    const keyFile = new File(['key'], 'key.pem', { type: 'application/x-pem-file' })

    const certInput = within(dialog).getByLabelText(t('certificates.certificatePem')) as HTMLInputElement
    const keyInput = within(dialog).getByLabelText(t('certificates.privateKeyPem')) as HTMLInputElement

    await user.upload(certInput, certFile)
    await user.upload(keyInput, keyFile)

    await waitFor(() => {
      expect(certInput.files?.[0]).toBe(certFile)
      expect(keyInput.files?.[0]).toBe(keyFile)
    })

    const form = dialog.querySelector('form') as HTMLFormElement
    fireEvent.submit(form)

    await waitFor(() => {
      expect(uploadCertificate).toHaveBeenCalledWith('My Cert', certFile, keyFile)
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['certificates'] })
      expect(toast.success).toHaveBeenCalledWith(t('certificates.uploadSuccess'))
    })

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: t('certificates.uploadCertificate') })).not.toBeInTheDocument()
    })
  })

  it('surfaces upload errors', async () => {
    vi.mocked(uploadCertificate).mockRejectedValue(new Error('Upload failed'))

    const user = userEvent.setup()
    renderWithQueryClient(<Certificates />)

    await user.click(screen.getByRole('button', { name: t('certificates.addCertificate') }))

    const dialog = await screen.findByRole('dialog', { name: t('certificates.uploadCertificate') })

    const nameInput = within(dialog).getByLabelText(t('certificates.friendlyName')) as HTMLInputElement
    await user.type(nameInput, 'My Cert')
    await waitFor(() => {
      expect(nameInput.value).toBe('My Cert')
    })

    const certFile = new File(['cert'], 'cert.pem', { type: 'application/x-pem-file' })
    const keyFile = new File(['key'], 'key.pem', { type: 'application/x-pem-file' })

    const certInput = within(dialog).getByLabelText(t('certificates.certificatePem')) as HTMLInputElement
    const keyInput = within(dialog).getByLabelText(t('certificates.privateKeyPem')) as HTMLInputElement

    await user.upload(certInput, certFile)
    await user.upload(keyInput, keyFile)

    await waitFor(() => {
      expect(certInput.files?.[0]).toBe(certFile)
      expect(keyInput.files?.[0]).toBe(keyFile)
    })

    const form = dialog.querySelector('form') as HTMLFormElement
    fireEvent.submit(form)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(`${t('certificates.uploadFailed')}: Upload failed`)
    })
  })
})
