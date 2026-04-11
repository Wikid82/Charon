import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import DeleteCertificateDialog from '../../dialogs/DeleteCertificateDialog'

import type { Certificate } from '../../../api/certificates'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

const baseCert: Certificate = {
  uuid: 'cert-1',
  name: 'Test Cert',
  domains: 'test.example.com',
  issuer: 'Custom CA',
  expires_at: '2026-01-01T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  has_key: true,
  in_use: false,
}

describe('DeleteCertificateDialog', () => {
  it('renders warning text for custom cert', () => {
    render(
      <DeleteCertificateDialog
        certificate={baseCert}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('certificates.deleteConfirmCustom')).toBeInTheDocument()
    expect(screen.getByText('certificates.deleteTitle')).toBeInTheDocument()
  })

  it('renders warning text for staging cert', () => {
    const staging: Certificate = { ...baseCert, provider: 'letsencrypt-staging', status: 'untrusted' }
    render(
      <DeleteCertificateDialog
        certificate={staging}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('certificates.deleteConfirmStaging')).toBeInTheDocument()
  })

  it('renders warning text for expired cert', () => {
    const expired: Certificate = { ...baseCert, provider: 'letsencrypt', status: 'expired' }
    render(
      <DeleteCertificateDialog
        certificate={expired}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('certificates.deleteConfirmExpired')).toBeInTheDocument()
  })

  it('calls onCancel when Cancel is clicked', async () => {
    const onCancel = vi.fn()
    const user = userEvent.setup()
    render(
      <DeleteCertificateDialog
        certificate={baseCert}
        open={true}
        onConfirm={vi.fn()}
        onCancel={onCancel}
        isDeleting={false}
      />
    )
    await user.click(screen.getByRole('button', { name: 'common.cancel' }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('calls onConfirm when Delete is clicked', async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(
      <DeleteCertificateDialog
        certificate={baseCert}
        open={true}
        onConfirm={onConfirm}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    await user.click(screen.getByRole('button', { name: 'certificates.deleteButton' }))
    expect(onConfirm).toHaveBeenCalled()
  })

  it('renders nothing when certificate is null', () => {
    const { container } = render(
      <DeleteCertificateDialog
        certificate={null}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders expired warning for expired staging cert (priority ordering)', () => {
    const expiredStaging: Certificate = { ...baseCert, provider: 'letsencrypt-staging', status: 'expired' }
    render(
      <DeleteCertificateDialog
        certificate={expiredStaging}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('certificates.deleteConfirmExpired')).toBeInTheDocument()
    expect(screen.queryByText('certificates.deleteConfirmStaging')).not.toBeInTheDocument()
  })
})
