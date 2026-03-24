import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import BulkDeleteCertificateDialog from '../../dialogs/BulkDeleteCertificateDialog'

import type { Certificate } from '../../../api/certificates'

const makeCert = (overrides: Partial<Certificate>): Certificate => ({
  id: 1,
  name: 'Test Cert',
  domain: 'test.example.com',
  issuer: 'Custom CA',
  expires_at: '2026-01-01T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  ...overrides,
})

const certs: Certificate[] = [
  makeCert({ id: 1, name: 'Cert One', domain: 'one.example.com' }),
  makeCert({ id: 2, name: 'Cert Two', domain: 'two.example.com', provider: 'letsencrypt-staging', status: 'untrusted' }),
  makeCert({ id: 3, name: 'Cert Three', domain: 'three.example.com', provider: 'letsencrypt', status: 'expired' }),
]

describe('BulkDeleteCertificateDialog', () => {
  it('renders dialog with count in title when 3 certs supplied', () => {
    render(
      <BulkDeleteCertificateDialog
        certificates={certs}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: 'Delete 3 Certificate(s)' })).toBeInTheDocument()
  })

  it('lists each certificate name in the scrollable list', () => {
    render(
      <BulkDeleteCertificateDialog
        certificates={certs}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('Cert One')).toBeInTheDocument()
    expect(screen.getByText('Cert Two')).toBeInTheDocument()
    expect(screen.getByText('Cert Three')).toBeInTheDocument()
    expect(screen.getByText('Custom')).toBeInTheDocument()
    expect(screen.getByText('Staging')).toBeInTheDocument()
    expect(screen.getByText('Expired LE')).toBeInTheDocument()
  })

  it('calls onConfirm when the Delete button is clicked', async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(
      <BulkDeleteCertificateDialog
        certificates={certs}
        open={true}
        onConfirm={onConfirm}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete 3 Certificate(s)' }))
    expect(onConfirm).toHaveBeenCalled()
  })

  it('calls onCancel when the Cancel button is clicked', async () => {
    const onCancel = vi.fn()
    const user = userEvent.setup()
    render(
      <BulkDeleteCertificateDialog
        certificates={certs}
        open={true}
        onConfirm={vi.fn()}
        onCancel={onCancel}
        isDeleting={false}
      />
    )
    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('Delete button is loading/disabled when isDeleting is true', () => {
    render(
      <BulkDeleteCertificateDialog
        certificates={certs}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={true}
      />
    )
    const dialog = screen.getByRole('dialog')
    const deleteBtn = within(dialog).getByRole('button', { name: 'Delete 3 Certificate(s)' })
    expect(deleteBtn).toBeDisabled()
    const cancelBtn = within(dialog).getByRole('button', { name: 'Cancel' })
    expect(cancelBtn).toBeDisabled()
  })

  it('returns null when certificates array is empty', () => {
    const { container } = render(
      <BulkDeleteCertificateDialog
        certificates={[]}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders "Expiring LE" label for a letsencrypt cert with status expiring', () => {
    const expiringCert = makeCert({ id: 4, name: 'Expiring Cert', domain: 'expiring.example.com', provider: 'letsencrypt', status: 'expiring' })
    render(
      <BulkDeleteCertificateDialog
        certificates={[expiringCert]}
        open={true}
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        isDeleting={false}
      />
    )
    expect(screen.getByText('Expiring LE')).toBeInTheDocument()
  })
})
