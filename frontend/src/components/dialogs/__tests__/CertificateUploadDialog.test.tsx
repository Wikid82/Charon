import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { createTestQueryClient } from '../../../test/createTestQueryClient'
import CertificateUploadDialog from '../CertificateUploadDialog'
import { toast } from '../../../utils/toast'

const uploadMutateFn = vi.fn()
const validateMutateFn = vi.fn()

vi.mock('../../../hooks/useCertificates', () => ({
  useUploadCertificate: vi.fn(() => ({
    mutate: uploadMutateFn,
    isPending: false,
  })),
  useValidateCertificate: vi.fn(() => ({
    mutate: validateMutateFn,
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

function renderDialog(open = true, onOpenChange = vi.fn()) {
  const qc = createTestQueryClient()
  return {
    onOpenChange,
    ...render(
      <QueryClientProvider client={qc}>
        <CertificateUploadDialog open={open} onOpenChange={onOpenChange} />
      </QueryClientProvider>,
    ),
  }
}

function createFile(name = 'test.pem'): File {
  return new File(['cert-content'], name, { type: 'application/x-pem-file' })
}

describe('CertificateUploadDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders dialog when open', () => {
    renderDialog()
    expect(screen.getByTestId('certificate-upload-dialog')).toBeTruthy()
    expect(screen.getByText('certificates.uploadCertificate')).toBeTruthy()
  })

  it('does not render when closed', () => {
    renderDialog(false)
    expect(screen.queryByTestId('certificate-upload-dialog')).toBeFalsy()
  })

  it('shows certificate file drop zone', () => {
    renderDialog()
    expect(screen.getByText('certificates.certificateFile')).toBeTruthy()
  })

  it('shows private key and chain file zones for non-PFX', () => {
    renderDialog()
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
    expect(screen.getByText('certificates.chainFile')).toBeTruthy()
  })

  it('shows name input', () => {
    renderDialog()
    expect(screen.getByText('certificates.friendlyName')).toBeTruthy()
  })

  it('has cancel and submit buttons', () => {
    renderDialog()
    expect(screen.getByText('common.cancel')).toBeTruthy()
    expect(screen.getByText('certificates.uploadAndSave')).toBeTruthy()
  })

  it('shows validate button after cert file is selected', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = createFile()
    await userEvent.upload(certInput, file)
    expect(await screen.findByTestId('validate-certificate-btn')).toBeTruthy()
  })

  it('calls validate mutation on validate click', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = createFile()
    await userEvent.upload(certInput, file)

    const validateBtn = await screen.findByTestId('validate-certificate-btn')
    await userEvent.click(validateBtn)

    expect(validateMutateFn).toHaveBeenCalledTimes(1)
    expect(validateMutateFn.mock.calls[0][0]).toMatchObject({
      certFile: file,
    })
  })

  it('calls upload mutation on form submit with name and cert', async () => {
    renderDialog()

    const nameInput = screen.getByPlaceholderText('e.g. My Custom Cert')
    await userEvent.type(nameInput, 'My Cert')

    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = createFile()
    await userEvent.upload(certInput, file)
    // jsdom constraint validation doesn't recognise programmatic file uploads
    certInput.required = false

    const keyInput = document.getElementById('key-file') as HTMLInputElement
    await userEvent.upload(keyInput, new File(['key'], 'key.pem', { type: 'application/x-pem-file' }))
    keyInput.required = false

    const submitBtn = screen.getByTestId('upload-certificate-submit')
    await userEvent.click(submitBtn)

    expect(uploadMutateFn).toHaveBeenCalledTimes(1)
    expect(uploadMutateFn.mock.calls[0][0]).toMatchObject({
      name: 'My Cert',
      certFile: file,
    })
  })

  it('calls onOpenChange(false) on cancel click', async () => {
    const onOpenChange = vi.fn()
    renderDialog(true, onOpenChange)
    const cancelBtn = screen.getByText('common.cancel')
    await userEvent.click(cancelBtn)
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('shows PFX message when PFX file is selected', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['pfx-content'], 'cert.pfx', { type: 'application/x-pkcs12' })
    await userEvent.upload(certInput, file)
    expect(await screen.findByText('certificates.pfxDetected')).toBeTruthy()
  })

  it('hides key and chain drop zones for PFX files', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['pfx-content'], 'cert.pfx', { type: 'application/x-pkcs12' })
    await userEvent.upload(certInput, file)

    await waitFor(() => {
      expect(screen.queryByText('certificates.privateKeyFile')).toBeFalsy()
      expect(screen.queryByText('certificates.chainFile')).toBeFalsy()
    })
  })

  it('shows toast on upload success', async () => {
    uploadMutateFn.mockImplementation((_params: unknown, opts: { onSuccess: () => void }) => {
      opts.onSuccess()
    })
    renderDialog()

    const nameInput = screen.getByPlaceholderText('e.g. My Custom Cert')
    await userEvent.type(nameInput, 'Cert')

    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())
    certInput.required = false

    const keyInput = document.getElementById('key-file') as HTMLInputElement
    await userEvent.upload(keyInput, new File(['key'], 'key.pem', { type: 'application/x-pem-file' }))
    keyInput.required = false

    await userEvent.click(screen.getByTestId('upload-certificate-submit'))
    expect(toast.success).toHaveBeenCalledWith('certificates.uploadSuccess')
  })

  it('shows toast on upload error', async () => {
    uploadMutateFn.mockImplementation((_params: unknown, opts: { onError: (e: Error) => void }) => {
      opts.onError(new Error('Upload failed'))
    })
    renderDialog()

    const nameInput = screen.getByPlaceholderText('e.g. My Custom Cert')
    await userEvent.type(nameInput, 'Cert')

    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())
    certInput.required = false

    const keyInput = document.getElementById('key-file') as HTMLInputElement
    await userEvent.upload(keyInput, new File(['key'], 'key.pem', { type: 'application/x-pem-file' }))
    keyInput.required = false

    await userEvent.click(screen.getByTestId('upload-certificate-submit'))
    expect(toast.error).toHaveBeenCalled()
  })

  it('shows validation preview after successful validation', async () => {
    const mockResult = {
      valid: true,
      common_name: 'test.com',
      domains: ['test.com'],
      issuer_org: 'CA',
      expires_at: '2026-01-01',
      key_match: false,
      chain_valid: false,
      chain_depth: 0,
      warnings: [],
      errors: [],
    }
    validateMutateFn.mockImplementation((_params: unknown, opts: { onSuccess: (r: typeof mockResult) => void }) => {
      opts.onSuccess(mockResult)
    })

    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())

    await userEvent.click(await screen.findByTestId('validate-certificate-btn'))
    expect(await screen.findByTestId('certificate-validation-preview')).toBeTruthy()
  })

  it('shows toast on validate error', async () => {
    validateMutateFn.mockImplementation((_params: unknown, opts: { onError: (e: Error) => void }) => {
      opts.onError(new Error('Validation failed'))
    })
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())

    await userEvent.click(await screen.findByTestId('validate-certificate-btn'))
    expect(toast.error).toHaveBeenCalledWith('Validation failed')
  })

  it('detects .p12 as PFX format', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['pkcs12'], 'bundle.p12', { type: 'application/x-pkcs12' })
    await userEvent.upload(certInput, file)
    expect(await screen.findByText('certificates.pfxDetected')).toBeTruthy()
  })

  it('detects .crt as PEM format', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['cert'], 'my.crt', { type: 'application/x-x509' })
    await userEvent.upload(certInput, file)
    // PEM does not hide key/chain zones
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
  })

  it('detects .cer as PEM format', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['cert'], 'my.cer', { type: 'application/x-x509' })
    await userEvent.upload(certInput, file)
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
  })

  it('detects .der format', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['der'], 'cert.der', { type: 'application/x-x509' })
    await userEvent.upload(certInput, file)
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
  })

  it('detects .key format', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['key'], 'private.key', { type: 'application/x-pem-file' })
    await userEvent.upload(certInput, file)
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
  })

  it('handles unknown file extension gracefully', async () => {
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['data'], 'cert.xyz', { type: 'application/octet-stream' })
    await userEvent.upload(certInput, file)
    // Should still show key/chain zones (not PFX)
    expect(screen.getByText('certificates.privateKeyFile')).toBeTruthy()
  })

  it('resets validation when cert file changes', async () => {
    const mockResult = {
      valid: true,
      common_name: 'test.com',
      domains: ['test.com'],
      issuer_org: 'CA',
      expires_at: '2026-01-01',
      key_match: false,
      chain_valid: false,
      chain_depth: 0,
      warnings: [],
      errors: [],
    }
    validateMutateFn.mockImplementation((_params: unknown, opts: { onSuccess: (r: typeof mockResult) => void }) => {
      opts.onSuccess(mockResult)
    })

    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())
    await userEvent.click(await screen.findByTestId('validate-certificate-btn'))
    expect(await screen.findByTestId('certificate-validation-preview')).toBeTruthy()

    // Change cert file — validation result should disappear
    const newFile = new File(['new-cert'], 'new.pem', { type: 'application/x-pem-file' })
    await userEvent.upload(certInput, newFile)
    await waitFor(() => {
      expect(screen.queryByTestId('certificate-validation-preview')).toBeFalsy()
    })
  })

  it('resets validation when key file changes', async () => {
    const mockResult = {
      valid: true,
      common_name: 'test.com',
      domains: ['test.com'],
      issuer_org: 'CA',
      expires_at: '2026-01-01',
      key_match: false,
      chain_valid: false,
      chain_depth: 0,
      warnings: [],
      errors: [],
    }
    validateMutateFn.mockImplementation((_params: unknown, opts: { onSuccess: (r: typeof mockResult) => void }) => {
      opts.onSuccess(mockResult)
    })

    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())
    await userEvent.click(await screen.findByTestId('validate-certificate-btn'))
    expect(await screen.findByTestId('certificate-validation-preview')).toBeTruthy()

    const keyInput = document.getElementById('key-file') as HTMLInputElement
    const keyFile = new File(['key-data'], 'private.key', { type: 'application/x-pem-file' })
    await userEvent.upload(keyInput, keyFile)
    await waitFor(() => {
      expect(screen.queryByTestId('certificate-validation-preview')).toBeFalsy()
    })
  })

  it('resets validation when chain file changes', async () => {
    const mockResult = {
      valid: true,
      common_name: 'test.com',
      domains: ['test.com'],
      issuer_org: 'CA',
      expires_at: '2026-01-01',
      key_match: false,
      chain_valid: false,
      chain_depth: 0,
      warnings: [],
      errors: [],
    }
    validateMutateFn.mockImplementation((_params: unknown, opts: { onSuccess: (r: typeof mockResult) => void }) => {
      opts.onSuccess(mockResult)
    })

    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    await userEvent.upload(certInput, createFile())
    await userEvent.click(await screen.findByTestId('validate-certificate-btn'))
    expect(await screen.findByTestId('certificate-validation-preview')).toBeTruthy()

    const chainInput = document.getElementById('chain-file') as HTMLInputElement
    const chainFile = new File(['chain-data'], 'chain.pem', { type: 'application/x-pem-file' })
    await userEvent.upload(chainInput, chainFile)
    await waitFor(() => {
      expect(screen.queryByTestId('certificate-validation-preview')).toBeFalsy()
    })
  })

  it('shows KEY format badge when .key file is uploaded', async () => {
    const user = userEvent.setup({ applyAccept: false })
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['key-data'], 'server.key', { type: 'application/x-pem-file' })
    await user.upload(certInput, file)
    expect(await screen.findByText('KEY')).toBeTruthy()
  })

  it('shows no format badge for unknown file extension', async () => {
    const user = userEvent.setup({ applyAccept: false })
    renderDialog()
    const certInput = document.getElementById('cert-file') as HTMLInputElement
    const file = new File(['data'], 'cert.bin', { type: 'application/octet-stream' })
    await user.upload(certInput, file)
    await screen.findByText('cert.bin')
    expect(screen.queryByText('KEY')).toBeNull()
    expect(screen.queryByText('DER')).toBeNull()
    expect(screen.queryByText('PFX/PKCS#12')).toBeNull()
    expect(screen.queryByText('PEM')).toBeNull()
  })
})
