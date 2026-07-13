import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import { RemoteTargetsCard } from '../RemoteTargetsCard'

import type { RemoteTarget } from '../../../hooks/useRemoteTargets'

const mockDeleteMutate = vi.fn()
const mockTestMutate = vi.fn()
const mockStartOAuthMutate = vi.fn()
const mockDisconnectOAuthMutate = vi.fn()
const mockInvalidateQueries = vi.fn()
let targets: RemoteTarget[] | undefined
let isLoading = false

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-query')>('@tanstack/react-query')
  return {
    ...actual,
    useQueryClient: () => ({ invalidateQueries: mockInvalidateQueries }),
  }
})

vi.mock('../../../hooks/useRemoteTargets', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useRemoteTargets')>('../../../hooks/useRemoteTargets')
  return {
    ...actual,
    useRemoteTargets: () => ({ data: targets, isLoading }),
    useDeleteRemoteTarget: () => ({ mutate: mockDeleteMutate, isPending: false }),
    useTestRemoteTarget: () => ({ mutate: mockTestMutate, isPending: false, variables: undefined }),
    useStartRemoteTargetOAuth: () => ({ mutate: mockStartOAuthMutate, isPending: false, variables: undefined }),
    useDisconnectRemoteTargetOAuth: () => ({ mutate: mockDisconnectOAuthMutate, isPending: false, variables: undefined }),
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
  oauth_status: '',
  oauth_connected_at: null,
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
  oauth_status: '',
  oauth_connected_at: null,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-06T09:00:00Z',
}

const dropboxNotConnected: RemoteTarget = {
  uuid: 'r3',
  name: 'Dropbox',
  type: 'dropbox',
  enabled: true,
  config: { dropbox: { app_key: 'abc123', folder_path: '/charon-backups' } },
  secrets_set: true,
  last_test_at: null,
  last_test_status: 'never',
  last_error: '',
  oauth_status: 'not_connected',
  oauth_connected_at: null,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
}

const dropboxConnected: RemoteTarget = {
  ...dropboxNotConnected,
  uuid: 'r4',
  name: 'Dropbox Connected',
  oauth_status: 'connected',
  oauth_connected_at: '2026-07-05T00:00:00Z',
}

const gdriveRevoked: RemoteTarget = {
  uuid: 'r5',
  name: 'Google Drive',
  type: 'google_drive',
  enabled: true,
  config: { google_drive: { client_id: 'xyz', folder_path: 'Charon/Backups' } },
  secrets_set: true,
  last_test_at: null,
  last_test_status: 'never',
  last_error: '',
  oauth_status: 'revoked',
  oauth_connected_at: '2026-06-01T00:00:00Z',
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
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

  describe('OAuth status badge and connect/reconnect/disconnect buttons', () => {
    beforeEach(() => {
      targets = [nas, dropboxNotConnected, dropboxConnected, gdriveRevoked]
    })

    it('does not show an OAuth badge for non-OAuth types (s3/sftp)', () => {
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const nasRow = rows.find((row) => within(row).queryByText('Home NAS'))!
      expect(within(nasRow).queryByTestId('backup-remote-target-oauth-status-badge')).not.toBeInTheDocument()
    })

    it('shows a "Not Connected" badge and a Connect button for a not_connected Dropbox target', () => {
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox'))!
      expect(within(row).getByTestId('backup-remote-target-oauth-status-badge')).toHaveTextContent(/not connected/i)
      expect(within(row).getByTestId('backup-remote-target-connect-btn')).toHaveTextContent(/connect/i)
      expect(within(row).queryByTestId('backup-remote-target-disconnect-btn')).not.toBeInTheDocument()
    })

    it('shows a "Revoked" badge and a Reconnect button for a revoked Google Drive target', () => {
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Google Drive'))!
      expect(within(row).getByTestId('backup-remote-target-oauth-status-badge')).toHaveTextContent(/revoked/i)
      expect(within(row).getByTestId('backup-remote-target-connect-btn')).toHaveTextContent(/reconnect/i)
    })

    it('shows a "Connected" badge and a Disconnect button (no Connect button) for a connected target', () => {
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox Connected'))!
      expect(within(row).getByTestId('backup-remote-target-oauth-status-badge')).toHaveTextContent(/connected/i)
      expect(within(row).queryByTestId('backup-remote-target-connect-btn')).not.toBeInTheDocument()
      expect(within(row).getByTestId('backup-remote-target-disconnect-btn')).toBeInTheDocument()
    })

    it('starts the OAuth flow and redirects on Connect click', async () => {
      mockStartOAuthMutate.mockImplementation((_uuid, { onSuccess }) => {
        onSuccess({ authorize_url: 'https://www.dropbox.com/oauth2/authorize?client_id=abc123' })
      })
      // @ts-expect-error -- jsdom's window.location is not directly assignable; test-only override
      delete window.location
      // @ts-expect-error -- see above
      window.location = { href: '', search: '', pathname: '/backups' }

      const user = userEvent.setup()
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox'))!
      await user.click(within(row).getByTestId('backup-remote-target-connect-btn'))

      expect(mockStartOAuthMutate).toHaveBeenCalledWith('r3', expect.any(Object))
      expect(window.location.href).toBe('https://www.dropbox.com/oauth2/authorize?client_id=abc123')
    })

    it('shows an error toast when starting the OAuth flow fails', async () => {
      const { toast } = await import('../../../utils/toast')
      mockStartOAuthMutate.mockImplementation((_uuid, { onError }) => {
        onError(new Error('app.public_url is not configured'))
      })
      const user = userEvent.setup()
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox'))!
      await user.click(within(row).getByTestId('backup-remote-target-connect-btn'))

      expect(toast.error).toHaveBeenCalledWith('app.public_url is not configured')
    })

    it('disconnects OAuth and shows a success toast on Disconnect click', async () => {
      const { toast } = await import('../../../utils/toast')
      mockDisconnectOAuthMutate.mockImplementation((_uuid, { onSuccess }) => onSuccess())
      const user = userEvent.setup()
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox Connected'))!
      await user.click(within(row).getByTestId('backup-remote-target-disconnect-btn'))

      expect(mockDisconnectOAuthMutate).toHaveBeenCalledWith('r4', expect.any(Object))
      expect(toast.success).toHaveBeenCalled()
    })

    it('shows an error toast when disconnecting OAuth fails', async () => {
      const { toast } = await import('../../../utils/toast')
      mockDisconnectOAuthMutate.mockImplementation((_uuid, { onError }) => onError(new Error('disconnect failed')))
      const user = userEvent.setup()
      render(<RemoteTargetsCard />)
      const rows = screen.getAllByTestId('backup-remote-target-row')
      const row = rows.find((r) => within(r).queryByText('Dropbox Connected'))!
      await user.click(within(row).getByTestId('backup-remote-target-disconnect-btn'))

      expect(toast.error).toHaveBeenCalledWith('disconnect failed')
    })
  })

  describe('OAuth callback query-param handling on mount', () => {
    const setLocation = (search: string) => {
      // @ts-expect-error -- jsdom's window.location is not directly assignable; test-only override
      delete window.location
      // @ts-expect-error -- see above
      window.location = { href: '', search, pathname: '/backups' }
    }

    afterEach(() => {
      setLocation('')
    })

    it('shows a success toast, strips the query string, and invalidates the target list on oauth_result=success', async () => {
      const { toast } = await import('../../../utils/toast')
      setLocation('?oauth_result=success&provider=dropbox&target=r3')
      const replaceStateSpy = vi.spyOn(window.history, 'replaceState')

      render(<RemoteTargetsCard />)

      await waitFor(() => expect(toast.success).toHaveBeenCalled())
      expect(toast.success).toHaveBeenCalledWith(expect.stringMatching(/dropbox/i))
      expect(replaceStateSpy).toHaveBeenCalledWith(null, '', '/backups')
      expect(mockInvalidateQueries).toHaveBeenCalled()
    })

    it('shows a "denied" toast on oauth_result=error&message=authorization_denied', async () => {
      const { toast } = await import('../../../utils/toast')
      setLocation('?oauth_result=error&provider=dropbox&message=authorization_denied')

      render(<RemoteTargetsCard />)

      await waitFor(() => expect(toast.error).toHaveBeenCalled())
      expect(toast.error).toHaveBeenCalledWith('Authorization was denied')
    })

    it('shows a generic error toast for any other oauth_result=error message', async () => {
      const { toast } = await import('../../../utils/toast')
      setLocation('?oauth_result=error&provider=dropbox&message=token_exchange_failed')

      render(<RemoteTargetsCard />)

      await waitFor(() => expect(toast.error).toHaveBeenCalled())
      expect(toast.error).toHaveBeenCalledWith('Authorization failed. Please try again.')
    })

    it('does nothing when there is no oauth_result query param', () => {
      setLocation('')
      const replaceStateSpy = vi.spyOn(window.history, 'replaceState')

      render(<RemoteTargetsCard />)

      expect(replaceStateSpy).not.toHaveBeenCalled()
      expect(mockInvalidateQueries).not.toHaveBeenCalled()
    })
  })
})
