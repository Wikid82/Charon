import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { UploadBackupButton } from '../UploadBackupButton'

const mockUploadMutate = vi.fn()
let uploadData: unknown
let uploadPending = false

vi.mock('../../../hooks/useBackups', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useBackups')>('../../../hooks/useBackups')
  return {
    ...actual,
    useUploadBackup: () => ({
      mutate: mockUploadMutate,
      data: uploadData,
      isPending: uploadPending,
    }),
  }
})

vi.mock('../../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

describe('UploadBackupButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    uploadData = undefined
    uploadPending = false
  })

  it('renders a file input accepting .zip, .age, and .db', () => {
    render(<UploadBackupButton />)
    expect(screen.getByTestId('backup-upload-input')).toHaveAttribute('accept', '.zip,.age,.db')
  })

  it('uploads the selected file', async () => {
    const user = userEvent.setup()
    render(<UploadBackupButton />)

    const file = new File(['content'], 'my-backup.zip', { type: 'application/zip' })
    await user.upload(screen.getByTestId('backup-upload-input'), file)

    expect(mockUploadMutate).toHaveBeenCalledWith({ file }, expect.any(Object))
  })

  it('clicking the visible button opens the hidden file picker', async () => {
    const clickSpy = vi.spyOn(HTMLInputElement.prototype, 'click').mockImplementation(() => {})
    const user = userEvent.setup()
    render(<UploadBackupButton />)

    await user.click(screen.getByRole('button', { name: /upload backup/i }))
    expect(clickSpy).toHaveBeenCalled()
    clickSpy.mockRestore()
  })

  it('shows format version feedback for a valid v2 archive with no legacy warning', () => {
    uploadData = { filename: 'uploaded.zip', uuid: 'u1', legacy_format: false, message: 'ok' }
    render(<UploadBackupButton />)

    const feedback = screen.getByTestId('backup-upload-feedback')
    expect(feedback).toHaveTextContent(/format version 2/i)
    expect(feedback).not.toHaveTextContent(/legacy/i)
  })

  it('flags a legacy_format v1 archive with a warning', () => {
    uploadData = { filename: 'uploaded.zip', uuid: 'u2', legacy_format: true, message: 'ok' }
    render(<UploadBackupButton />)

    expect(screen.getByTestId('backup-upload-feedback')).toHaveTextContent(/legacy format/i)
  })

  it('surfaces an encryption_key_required warning', () => {
    uploadData = {
      filename: 'uploaded.zip',
      uuid: 'u3',
      legacy_format: false,
      encryption_key_required: true,
      message: 'ok',
    }
    render(<UploadBackupButton />)

    expect(screen.getByTestId('backup-upload-feedback')).toHaveTextContent(/encryption key|CHARON_ENCRYPTION_KEY/i)
  })

  it('shows an error toast when the upload is rejected', async () => {
    const { toast } = await import('../../../utils/toast')
    mockUploadMutate.mockImplementation((_vars, { onError }) => {
      onError(new Error('not a recognized backup format'))
    })
    const user = userEvent.setup()
    render(<UploadBackupButton />)

    const file = new File(['garbage'], 'garbage.zip', { type: 'application/zip' })
    await user.upload(screen.getByTestId('backup-upload-input'), file)

    expect(toast.error).toHaveBeenCalledWith('not a recognized backup format')
  })
})
