import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Certificates from '../Certificates'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('../../components/CertificateList', () => ({
  default: () => <div data-testid="certificate-list">CertificateList</div>,
}))

vi.mock('../../components/dialogs/CertificateUploadDialog', () => ({
  default: ({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) =>
    open ? (
      <div role="dialog" data-testid="upload-dialog">
        <button onClick={() => onOpenChange(false)}>Close</button>
      </div>
    ) : null,
}))

describe('Certificates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the page with certificate list and add button', () => {
    renderWithQueryClient(<Certificates />)
    expect(screen.getByText('certificates.addCertificate')).toBeInTheDocument()
    expect(screen.getByTestId('certificate-list')).toBeInTheDocument()
  })

  it('opens upload dialog when add button is clicked', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Certificates />)

    expect(screen.queryByTestId('upload-dialog')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'certificates.addCertificate' }))
    expect(screen.getByTestId('upload-dialog')).toBeInTheDocument()
  })

  it('closes upload dialog via onOpenChange callback', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Certificates />)

    await user.click(screen.getByRole('button', { name: 'certificates.addCertificate' }))
    expect(screen.getByTestId('upload-dialog')).toBeInTheDocument()

    await user.click(screen.getByText('Close'))
    expect(screen.queryByTestId('upload-dialog')).not.toBeInTheDocument()
  })

  it('renders info alert with note text', () => {
    renderWithQueryClient(<Certificates />)
    expect(screen.getByText('certificates.noteText')).toBeInTheDocument()
  })
})
