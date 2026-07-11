import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { RemoteTargetsCard } from '../RemoteTargetsCard'
import type { RemoteTarget } from '../../../hooks/useRemoteTargets'

const mockDeleteMutate = vi.fn()
const mockTestMutate = vi.fn()
let targets: RemoteTarget[] | undefined
let isLoading = false

vi.mock('../../../hooks/useRemoteTargets', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useRemoteTargets')>('../../../hooks/useRemoteTargets')
  return {
    ...actual,
    useRemoteTargets: () => ({ data: targets, isLoading }),
    useDeleteRemoteTarget: () => ({ mutate: mockDeleteMutate, isPending: false }),
    useTestRemoteTarget: () => ({ mutate: mockTestMutate, isPending: false, variables: undefined }),
  }
})

vi.mock('../RemoteTargetFormDialog', () => ({
  RemoteTargetFormDialog: ({
    open,
    target,
    onClose,
  }: {
    open: boolean
    target?: RemoteTarget | null
    onClose: () => void
  }) =>
    open ? (
      <div data-testid="mock-form-dialog">
        {target ? `edit:${target.name}` : 'create'}
        <button onClick={onClose}>mock-form-dialog-close</button>
      </div>
    ) : null,
}))

vi.mock('../../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const nas: RemoteTarget = {
  uuid: 'r1',
  name: 'Home NAS',
  type: 'sftp',
  enabled: true,
  config: { host: 'nas.lan', port: 22, path: '/backups/charon', username: 'charon' },
  secrets_set: true,
  last_test_at: '2026-07-07T09:00:00Z',
  last_test_status: 'ok',
  last_error: '',
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-07T09:00:00Z',
}

const b2: RemoteTarget = {
  uuid: 'r2',
  name: 'Backblaze B2',
  type: 's3',
  enabled: true,
  config: { endpoint: 's3.example.com', region: 'us', bucket: 'b', path_prefix: 'p', use_ssl: true, force_path_style: false },
  secrets_set: true,
  last_test_at: '2026-07-06T09:00:00Z',
  last_test_status: 'failed',
  last_error: 'connection timed out',
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-06T09:00:00Z',
}

describe('RemoteTargetsCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    targets = [nas, b2]
    isLoading = false
  })

  it('lists targets with a status badge reflecting last_test_status', () => {
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    expect(within(nasRow).getByTestId('backup-remote-target-status-badge')).toHaveTextContent(/ok/i)

    const b2Row = rows.find((row) => within(row).queryByText('Backblaze B2'))!
    expect(within(b2Row).getByTestId('backup-remote-target-status-badge')).toHaveTextContent(/failed/i)
  })

  it('shows an empty state when there are no targets', () => {
    targets = []
    render(<RemoteTargetsCard />)
    expect(screen.getByTestId('backup-remote-targets-empty-state')).toBeInTheDocument()
  })

  it('opens the create dialog when "Add Remote Target" is clicked', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    await user.click(screen.getByRole('button', { name: /add remote target/i }))
    expect(screen.getByTestId('mock-form-dialog')).toHaveTextContent('create')
  })

  it('opens the edit dialog with the target when the edit button is clicked', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-edit-btn'))

    expect(screen.getByTestId('mock-form-dialog')).toHaveTextContent('edit:Home NAS')
  })

  it('tests a target connection and shows a success toast', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestMutate.mockImplementation((_uuid, { onSuccess }) => {
      onSuccess({ success: true, message: 'Connected', latency_ms: 42 })
    })
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-test-btn'))

    expect(mockTestMutate).toHaveBeenCalledWith('r1', expect.any(Object))
    expect(toast.success).toHaveBeenCalledWith('Connected')
  })

  it('shows an error toast when a target connection test fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestMutate.mockImplementation((_uuid, { onError }) => {
      onError(new Error('connection timed out'))
    })
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const b2Row = rows.find((row) => within(row).queryByText('Backblaze B2'))!
    await user.click(within(b2Row).getByTestId('backup-remote-target-test-btn'))

    expect(toast.error).toHaveBeenCalledWith('connection timed out')
  })

  it('deletes a target after confirmation', async () => {
    mockDeleteMutate.mockImplementation((_uuid, { onSuccess }) => onSuccess())
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-delete-btn'))

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /^delete$/i }))

    expect(mockDeleteMutate).toHaveBeenCalledWith('r1', expect.any(Object))
  })

  it('shows an error toast and keeps the dialog open when delete fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockDeleteMutate.mockImplementation((_uuid, { onError }) => onError(new Error('delete failed')))
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-delete-btn'))

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /^delete$/i }))

    expect(toast.error).toHaveBeenCalledWith('delete failed')
  })

  it('closes the delete dialog when Cancel is clicked without deleting', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-delete-btn'))

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /cancel/i }))

    expect(mockDeleteMutate).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows a failure toast when a test reports success: false', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestMutate.mockImplementation((_uuid, { onSuccess }) => {
      onSuccess({ success: false, message: 'host key not yet trusted' })
    })
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-test-btn'))

    expect(toast.error).toHaveBeenCalledWith('host key not yet trusted')
  })

  it('opens the create dialog from the empty-state action', async () => {
    targets = []
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const emptyState = screen.getByTestId('backup-remote-targets-empty-state')
    await user.click(within(emptyState).getByRole('button', { name: /add remote target/i }))

    expect(screen.getByTestId('mock-form-dialog')).toHaveTextContent('create')
  })

  it('resets editingTarget back to undefined when the form dialog calls onClose', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    await user.click(screen.getByRole('button', { name: /add remote target/i }))
    expect(screen.getByTestId('mock-form-dialog')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /mock-form-dialog-close/i }))
    expect(screen.queryByTestId('mock-form-dialog')).not.toBeInTheDocument()
  })

  it('closes the delete confirmation dialog via onOpenChange (e.g. Escape) without deleting', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetsCard />)

    const rows = screen.getAllByTestId('backup-remote-target-row')
    const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
    await user.click(within(nasRow).getByTestId('backup-remote-target-delete-btn'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    await user.keyboard('{Escape}')

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(mockDeleteMutate).not.toHaveBeenCalled()
  })
})
