import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { RestoreDialog } from '../RestoreDialog'
import type { BackupFile } from '../../../hooks/useBackups'

const mockValidateMutate = vi.fn()
const mockRestoreMutate = vi.fn()
const mockValidateReset = vi.fn()
const mockRestoreReset = vi.fn()
let validateData: unknown
let validatePending = false
let restorePending = false

vi.mock('../../../hooks/useBackups', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useBackups')>('../../../hooks/useBackups')
  return {
    ...actual,
    useValidateBackup: () => ({
      mutate: mockValidateMutate,
      reset: mockValidateReset,
      data: validateData,
      isPending: validatePending,
    }),
    useRestoreBackup: () => ({
      mutate: mockRestoreMutate,
      reset: mockRestoreReset,
      isPending: restorePending,
    }),
  }
})

vi.mock('../../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const plainBackup: BackupFile = {
  filename: 'backup_2026-07-07_03-00-00.zip',
  size: 1024,
  time: '2026-07-07T03:00:00Z',
}

const encryptedBackup: BackupFile = {
  filename: 'backup_2026-07-06_03-00-00.zip.age',
  size: 2048,
  time: '2026-07-06T03:00:00Z',
  encrypted: true,
}

describe('RestoreDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    validateData = undefined
    validatePending = false
    restorePending = false
  })

  it('renders nothing when no backup is selected', () => {
    render(<RestoreDialog backup={null} onClose={vi.fn()} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('does not show a passphrase input for an unencrypted backup', () => {
    render(<RestoreDialog backup={plainBackup} onClose={vi.fn()} />)
    expect(screen.getByRole('dialog')).toBeVisible()
    expect(screen.queryByTestId('backup-restore-passphrase-input')).not.toBeInTheDocument()
  })

  it('shows a passphrase input for a .age backup', () => {
    render(<RestoreDialog backup={encryptedBackup} onClose={vi.fn()} />)
    expect(screen.getByTestId('backup-restore-passphrase-input')).toBeInTheDocument()
  })

  it('disables Validate/Restore until a passphrase is entered for encrypted backups', () => {
    render(<RestoreDialog backup={encryptedBackup} onClose={vi.fn()} />)
    expect(screen.getByRole('button', { name: /validate/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /^restore$/i })).toBeDisabled()
  })

  it('calls validate with the filename and passphrase', async () => {
    const user = userEvent.setup()
    render(<RestoreDialog backup={encryptedBackup} onClose={vi.fn()} />)

    await user.type(screen.getByTestId('backup-restore-passphrase-input'), 'correct-horse-battery-staple')
    await user.click(screen.getByRole('button', { name: /validate/i }))

    expect(mockValidateMutate).toHaveBeenCalledWith(
      { filename: encryptedBackup.filename, passphrase: 'correct-horse-battery-staple' },
      expect.any(Object)
    )
  })

  it('shows validate results including database integrity', () => {
    validateData = {
      valid: true,
      format_version: 2,
      legacy_format: false,
      database_integrity: 'ok',
      encryption_key_required: false,
    }
    render(<RestoreDialog backup={plainBackup} onClose={vi.fn()} />)

    expect(screen.getByTestId('backup-restore-validate-results')).toHaveTextContent(/database integrity: ok/i)
  })

  it('shows legacy_format and encryption_key_required warnings when present', () => {
    validateData = {
      valid: true,
      format_version: 1,
      legacy_format: true,
      database_integrity: 'ok',
      encryption_key_required: true,
      warnings: ['archive app_version is older than running version'],
    }
    render(<RestoreDialog backup={plainBackup} onClose={vi.fn()} />)

    const results = screen.getByTestId('backup-restore-validate-results')
    expect(results).toHaveTextContent(/legacy/i)
    expect(results).toHaveTextContent(/encryption key|CHARON_ENCRYPTION_KEY/i)
    expect(results).toHaveTextContent(/older than running version/i)
  })

  it('calls restore with filename/passphrase and closes on success', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    mockRestoreMutate.mockImplementation((_vars, { onSuccess }) => {
      onSuccess({
        message: 'Backup restored successfully',
        restart_required: false,
        database_swap_pending: false,
        live_rehydrate_applied: true,
        caddy_reloaded: true,
        legacy_format: false,
      })
    })

    render(<RestoreDialog backup={plainBackup} onClose={onClose} />)
    await user.click(screen.getByRole('button', { name: /^restore$/i }))

    expect(mockRestoreMutate).toHaveBeenCalledWith(
      { filename: plainBackup.filename, passphrase: undefined },
      expect.any(Object)
    )
    expect(onClose).toHaveBeenCalled()
  })

  it('keeps the dialog open and does not call onClose when restore fails', async () => {
    const { toast } = await import('../../../utils/toast')
    const user = userEvent.setup()
    const onClose = vi.fn()
    mockRestoreMutate.mockImplementation((_vars, { onError }) => {
      onError(new Error('wrong passphrase'))
    })

    render(<RestoreDialog backup={encryptedBackup} onClose={onClose} />)
    await user.type(screen.getByTestId('backup-restore-passphrase-input'), 'wrong-passphrase')
    await user.click(screen.getByRole('button', { name: /^restore$/i }))

    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeVisible()
    expect(toast.error).toHaveBeenCalledWith('Failed to restore backup: wrong passphrase')
  })

  it('shows an error toast when validate fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockValidateMutate.mockImplementation((_vars, { onError }) => onError(new Error('corrupt archive')))
    const user = userEvent.setup()

    render(<RestoreDialog backup={plainBackup} onClose={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: /validate/i }))

    expect(toast.error).toHaveBeenCalledWith('corrupt archive')
  })

  it('shows a restart-required success message when the restore result reports it', async () => {
    const { toast } = await import('../../../utils/toast')
    const onClose = vi.fn()
    mockRestoreMutate.mockImplementation((_vars, { onSuccess }) => {
      onSuccess({
        message: 'ok',
        restart_required: true,
        database_swap_pending: true,
        live_rehydrate_applied: false,
        caddy_reloaded: true,
        legacy_format: false,
      })
    })
    const user = userEvent.setup()

    render(<RestoreDialog backup={plainBackup} onClose={onClose} />)
    await user.click(screen.getByRole('button', { name: /^restore$/i }))

    expect(toast.success).toHaveBeenCalledWith(expect.stringMatching(/restart/i))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls onClose when the dialog is dismissed (e.g. Escape)', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RestoreDialog backup={plainBackup} onClose={onClose} />)

    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })
})
