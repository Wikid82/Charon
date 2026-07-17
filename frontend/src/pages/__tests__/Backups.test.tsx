import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import Backups from '../Backups'
import { toast } from '../../utils/toast'

const mockUseAuth = vi.fn()
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}))

const mockUseBackups = vi.fn()
const mockUseDbHealth = vi.fn()
const mockUseRemoteTargets = vi.fn()
const mockCreateMutate = vi.fn()
const mockDeleteMutate = vi.fn()
let createIsPending = false
let createJob: { stage?: string } | undefined
let encryptionPassphraseSet = false

vi.mock('../../hooks/useDbHealth', () => ({
  useDbHealth: () => mockUseDbHealth(),
}))

vi.mock('../../hooks/useRemoteTargets', () => ({
  useRemoteTargets: () => mockUseRemoteTargets(),
}))

vi.mock('../../hooks/useBackups', async () => {
  const actual = await vi.importActual<typeof import('../../hooks/useBackups')>('../../hooks/useBackups')
  return {
    ...actual,
    useBackups: () => mockUseBackups(),
    useCreateBackup: () => ({ mutate: mockCreateMutate, isPending: createIsPending, job: createJob }),
    useDeleteBackup: () => ({ mutate: mockDeleteMutate, isPending: false }),
    useBackupSettingsForm: () => ({
      settings: undefined,
      isLoading: false,
      enabled: true,
      setEnabled: vi.fn(),
      frequency: 'daily',
      setFrequency: vi.fn(),
      time: '03:00',
      setTime: vi.fn(),
      dayOfWeek: '0',
      setDayOfWeek: vi.fn(),
      cronExpression: '0 3 * * *',
      setCronExpression: vi.fn(),
      cronValid: true,
      retentionCount: '7',
      setRetentionCount: vi.fn(),
      remoteRetentionCount: '7',
      setRemoteRetentionCount: vi.fn(),
      encryptionEnabled: false,
      setEncryptionEnabled: vi.fn(),
      encryptionPassphrase: '',
      setEncryptionPassphrase: vi.fn(),
      encryptionPassphraseSet,
      isSaving: false,
      saveDisabled: false,
      save: vi.fn(),
    }),
  }
})

vi.mock('../../components/backups/BackupScheduleCard', () => ({
  BackupScheduleCard: () => <div data-testid="mock-schedule-card" />,
}))
vi.mock('../../components/backups/BackupEncryptionCard', () => ({
  BackupEncryptionCard: () => <div data-testid="mock-encryption-card" />,
}))
vi.mock('../../components/backups/RemoteTargetsCard', () => ({
  RemoteTargetsCard: () => <div data-testid="mock-remote-targets-card" />,
}))
vi.mock('../../components/backups/UploadBackupButton', () => ({
  UploadBackupButton: () => <div data-testid="mock-upload-button" />,
}))
vi.mock('../../components/backups/RestoreDialog', () => ({
  RestoreDialog: ({ backup, onClose }: { backup: { filename: string } | null; onClose: () => void }) =>
    backup ? (
      <div data-testid="mock-restore-dialog">
        {backup.filename}
        <button onClick={onClose}>mock-restore-dialog-close</button>
      </div>
    ) : null,
}))

const mockBackups = [
  {
    filename: 'backup_2026-07-07_03-00-00.zip',
    size: 1048576,
    time: '2026-07-07T03:00:00Z',
    uuid: 'a1',
    type: 'scheduled' as const,
    encrypted: false,
    format_version: 2,
  },
  {
    filename: 'backup_2026-07-06_03-00-00.zip.age',
    size: 2048,
    time: '2026-07-06T03:00:00Z',
    uuid: 'a2',
    type: 'uploaded' as const,
    encrypted: true,
    format_version: 2,
    remote_copies: [{ target_uuid: 'r1', target_name: 'NAS', status: 'uploaded' }],
  },
]

describe('Backups page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue({ user: { role: 'admin' } })
    mockUseBackups.mockReturnValue({ data: mockBackups, isLoading: false })
    mockUseDbHealth.mockReturnValue({ data: { status: 'healthy' } })
    mockUseRemoteTargets.mockReturnValue({ data: [] })
    createIsPending = false
    createJob = undefined
    encryptionPassphraseSet = false
  })

  it('renders the backup table with type badges, encrypted icon, and remote copy status', () => {
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    expect(rows).toHaveLength(2)

    const encryptedRow = rows.find((row) => within(row).queryByText(/\.zip\.age/) !== null)!
    expect(within(encryptedRow).getByTestId('backup-encrypted-icon')).toBeInTheDocument()

    const plainRow = rows.find((row) => row !== encryptedRow)!
    expect(within(plainRow).queryByTestId('backup-encrypted-icon')).not.toBeInTheDocument()

    expect(within(encryptedRow).getByTestId('backup-type-badge')).toHaveTextContent(/uploaded/i)
    expect(within(encryptedRow).getByText(/NAS/)).toBeInTheDocument()
  })

  it('shows admin-only settings cards and action buttons for admin users', () => {
    render(<Backups />)

    expect(screen.getByTestId('mock-schedule-card')).toBeInTheDocument()
    expect(screen.getByTestId('mock-encryption-card')).toBeInTheDocument()
    expect(screen.getByTestId('mock-remote-targets-card')).toBeInTheDocument()
    expect(screen.getAllByTestId('backup-restore-btn')).toHaveLength(2)
    expect(screen.getAllByTestId('backup-delete-btn')).toHaveLength(2)
    expect(screen.getAllByTestId('backup-download-btn')).toHaveLength(2)
  })

  it('hides admin-only settings cards and action buttons for non-admin users (role-gating fix)', () => {
    mockUseAuth.mockReturnValue({ user: { role: 'user' } })
    render(<Backups />)

    expect(screen.queryByTestId('mock-schedule-card')).not.toBeInTheDocument()
    expect(screen.queryByTestId('mock-encryption-card')).not.toBeInTheDocument()
    expect(screen.queryByTestId('mock-remote-targets-card')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /create backup/i })).not.toBeInTheDocument()
    expect(screen.queryByTestId('backup-restore-btn')).not.toBeInTheDocument()
    expect(screen.queryByTestId('backup-delete-btn')).not.toBeInTheDocument()
    expect(screen.queryByTestId('backup-download-btn')).not.toBeInTheDocument()
  })

  it('shows the loading skeleton while backups are loading', () => {
    mockUseBackups.mockReturnValue({ data: undefined, isLoading: true })
    render(<Backups />)

    expect(screen.getByTestId('loading-skeleton')).toBeInTheDocument()
  })

  it('shows an empty state with a create action when there are no backups', async () => {
    const user = userEvent.setup()
    mockUseBackups.mockReturnValue({ data: [], isLoading: false })
    render(<Backups />)

    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('opens the create backup dialog and submits an unencrypted backup by default', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).queryByTestId('backup-create-passphrase-input')).not.toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /^create$/i }))

    expect(mockCreateMutate).toHaveBeenCalledWith(undefined, expect.any(Object))
  })

  it('submits an encrypted backup with the entered passphrase when the encrypt toggle is on', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    await user.click(within(dialog).getByTestId('backup-create-encrypt-toggle'))
    await user.type(within(dialog).getByTestId('backup-create-passphrase-input'), 'correct-horse-battery-staple')
    await user.click(within(dialog).getByRole('button', { name: /^create$/i }))

    expect(mockCreateMutate).toHaveBeenCalledWith(
      { encrypt: true, passphrase: 'correct-horse-battery-staple' },
      expect.any(Object)
    )
  })

  it('defaults the encrypt toggle on and hides the passphrase input when a passphrase is already stored', async () => {
    encryptionPassphraseSet = true
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    expect(within(dialog).getByTestId('backup-create-encrypt-toggle')).toBeChecked()
    expect(within(dialog).queryByTestId('backup-create-passphrase-input')).not.toBeInTheDocument()
    expect(within(dialog).getByTestId('backup-create-passphrase-set-indicator')).toBeInTheDocument()
  })

  it('submits without a passphrase field when confirming an encrypted backup with a stored passphrase', async () => {
    encryptionPassphraseSet = true
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    await user.click(within(dialog).getByRole('button', { name: /^create$/i }))

    expect(mockCreateMutate).toHaveBeenCalledWith({ encrypt: true }, expect.any(Object))
    const payload = mockCreateMutate.mock.calls[0][0]
    expect(payload).not.toHaveProperty('passphrase')
  })

  it('shows and defaults on the upload-to-remote toggle when an enabled remote target exists', async () => {
    mockUseRemoteTargets.mockReturnValue({ data: [{ uuid: 'r1', name: 'NAS', enabled: true }] })
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    expect(within(dialog).getByTestId('backup-create-upload-toggle')).toBeChecked()
  })

  it('hides the upload-to-remote toggle when there are no enabled remote targets', async () => {
    mockUseRemoteTargets.mockReturnValue({ data: [{ uuid: 'r1', name: 'NAS', enabled: false }] })
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    expect(within(dialog).queryByTestId('backup-create-upload-toggle')).not.toBeInTheDocument()
  })

  it('omits upload_to_remote from the payload when there are no enabled remote targets', async () => {
    mockUseRemoteTargets.mockReturnValue({ data: [] })
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    const dialog = screen.getByRole('dialog')

    await user.click(within(dialog).getByRole('button', { name: /^create$/i }))

    expect(mockCreateMutate).toHaveBeenCalledWith(undefined, expect.any(Object))
    const payload = mockCreateMutate.mock.calls[0][0]
    expect(payload).toBeUndefined()
  })

  it('opens the restore dialog for the clicked row', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-restore-btn'))

    expect(screen.getByTestId('mock-restore-dialog')).toHaveTextContent(mockBackups[0].filename)
  })

  it('opens a delete confirmation dialog and calls delete on confirm', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-delete-btn'))

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent(mockBackups[0].filename)

    await user.click(within(dialog).getByRole('button', { name: /^delete$/i }))
    expect(mockDeleteMutate).toHaveBeenCalledWith(mockBackups[0].filename, expect.any(Object))
  })

  it('triggers a download via window.location navigation', async () => {
    const user = userEvent.setup()
    const originalLocation = window.location
    const stubLocation = { ...originalLocation, href: '' }
    Object.defineProperty(window, 'location', { value: stubLocation, writable: true, configurable: true })

    render(<Backups />)
    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-download-btn'))

    expect(window.location.href).toBe(`/api/v1/backups/${mockBackups[0].filename}/download`)
    Object.defineProperty(window, 'location', { value: originalLocation, writable: true, configurable: true })
  })

  it('waits for confirmation before invoking the delete mutation', async () => {
    render(<Backups />)
    await waitFor(() => expect(screen.getAllByTestId('backup-row')).toHaveLength(2))
    expect(mockDeleteMutate).not.toHaveBeenCalled()
  })

  it('closes the create dialog without mutating when Cancel is clicked', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^cancel$/i }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(mockCreateMutate).not.toHaveBeenCalled()
  })

  it('shows a success toast and closes the create dialog when backup creation succeeds', async () => {
    const toastSuccessSpy = vi.spyOn(toast, 'success')
    mockCreateMutate.mockImplementation((_options, { onSuccess }) => onSuccess())
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i }))

    expect(toastSuccessSpy).toHaveBeenCalledWith('Backup created successfully')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows an error toast with the failure reason when backup creation fails', async () => {
    const toastErrorSpy = vi.spyOn(toast, 'error')
    mockCreateMutate.mockImplementation((_options, { onError }) => onError(new Error('disk full')))
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i }))

    expect(toastErrorSpy).toHaveBeenCalledWith('Failed to create backup: disk full')
    // The dialog stays open so the admin can retry rather than losing their input.
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('shows a success toast and clears the confirmation state when delete succeeds', async () => {
    const toastSuccessSpy = vi.spyOn(toast, 'success')
    mockDeleteMutate.mockImplementation((_filename, { onSuccess }) => onSuccess())
    const user = userEvent.setup()
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-delete-btn'))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i }))

    expect(toastSuccessSpy).toHaveBeenCalledWith('Backup deleted successfully')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows an error toast with the failure reason when delete fails', async () => {
    const toastErrorSpy = vi.spyOn(toast, 'error')
    mockDeleteMutate.mockImplementation((_filename, { onError }) => onError(new Error('file locked')))
    const user = userEvent.setup()
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-delete-btn'))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i }))

    expect(toastErrorSpy).toHaveBeenCalledWith('Failed to delete backup: file locked')
    // The confirmation dialog stays open on failure since deleteConfirm is only cleared on success.
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('resets restoreTarget to null when RestoreDialog calls onClose', async () => {
    const user = userEvent.setup()
    render(<Backups />)

    const rows = screen.getAllByTestId('backup-row')
    await user.click(within(rows[0]).getByTestId('backup-restore-btn'))
    expect(screen.getByTestId('mock-restore-dialog')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /mock-restore-dialog-close/i }))
    expect(screen.queryByTestId('mock-restore-dialog')).not.toBeInTheDocument()
  })

  it('shows a DB corruption warning banner when db health reports status:"corrupted"', () => {
    mockUseDbHealth.mockReturnValue({ data: { status: 'corrupted' } })
    render(<Backups />)

    expect(screen.getByTestId('db-corruption-banner')).toBeInTheDocument()
    expect(screen.getByTestId('db-corruption-banner')).toHaveTextContent(/integrity/i)
  })

  it('does not show a DB corruption banner when db health is healthy', () => {
    mockUseDbHealth.mockReturnValue({ data: { status: 'healthy' } })
    render(<Backups />)

    expect(screen.queryByTestId('db-corruption-banner')).not.toBeInTheDocument()
  })

  it('does not show a DB corruption banner while db health is still loading', () => {
    mockUseDbHealth.mockReturnValue({ data: undefined })
    render(<Backups />)

    expect(screen.queryByTestId('db-corruption-banner')).not.toBeInTheDocument()
  })

  it('shows the job stage caption under the spinner while a create job is running', async () => {
    createIsPending = true
    createJob = { stage: 'archiving_files' }
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])

    expect(screen.getByTestId('backup-create-stage')).toHaveTextContent('Archiving files')
  })

  it('does not show a stage caption when the create job has no stage yet', async () => {
    createIsPending = true
    createJob = undefined
    const user = userEvent.setup()
    render(<Backups />)

    await user.click(screen.getAllByRole('button', { name: /create backup/i })[0])

    expect(screen.queryByTestId('backup-create-stage')).not.toBeInTheDocument()
  })
})
