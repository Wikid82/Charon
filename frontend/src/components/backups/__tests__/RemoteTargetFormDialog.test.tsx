import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { RemoteTargetFormDialog } from '../RemoteTargetFormDialog'

import type { RemoteTarget } from '../../../hooks/useRemoteTargets'

const mockCreateMutate = vi.fn()
const mockUpdateMutate = vi.fn()
const mockTestMutate = vi.fn()
const mockTestDraftMutate = vi.fn()
const mockStartOAuthMutate = vi.fn()
let createPending = false
let updatePending = false
let testPending = false
let testDraftPending = false
let startOAuthPending = false

vi.mock('../../../hooks/useRemoteTargets', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useRemoteTargets')>('../../../hooks/useRemoteTargets')
  return {
    ...actual,
    useCreateRemoteTarget: () => ({ mutate: mockCreateMutate, isPending: createPending }),
    useUpdateRemoteTarget: () => ({ mutate: mockUpdateMutate, isPending: updatePending }),
    useTestRemoteTarget: () => ({ mutate: mockTestMutate, isPending: testPending }),
    useTestDraftRemoteTarget: () => ({ mutate: mockTestDraftMutate, isPending: testDraftPending }),
    useStartRemoteTargetOAuth: () => ({ mutate: mockStartOAuthMutate, isPending: startOAuthPending }),
  }
})

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

const dropboxTarget: RemoteTarget = {
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

describe('RemoteTargetFormDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createPending = false
    updatePending = false
    testPending = false
    testDraftPending = false
    startOAuthPending = false
    // @ts-expect-error -- jsdom's window.location is not directly assignable; test-only override
    delete window.location
    // @ts-expect-error -- see above
    window.location = { href: '', search: '', pathname: '/backups' }
  })

  it('renders nothing visible when closed', () => {
    render(<RemoteTargetFormDialog open={false} target={null} onClose={vi.fn()} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('submits an S3 target with config + secrets', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /s3/i }))

    await user.type(within(dialog).getByLabelText(/name/i), 'My S3 Target')
    await user.type(within(dialog).getByLabelText(/endpoint/i), 's3.us-west-002.backblazeb2.com')
    await user.type(within(dialog).getByLabelText(/region/i), 'us-west-002')
    await user.type(within(dialog).getByLabelText(/bucket/i), 'charon-backups')
    await user.type(within(dialog).getByLabelText(/path prefix/i), 'prod')
    await user.click(within(dialog).getByLabelText(/use ssl/i))
    await user.type(within(dialog).getByLabelText(/access key id/i), 'AKIAEXAMPLE')
    await user.type(within(dialog).getByLabelText(/secret access key/i), 'super-secret-key')

    await user.click(within(dialog).getByRole('button', { name: /save|create/i }))

    expect(mockCreateMutate).toHaveBeenCalledTimes(1)
    const [payload] = mockCreateMutate.mock.calls[0]
    expect(payload.type).toBe('s3')
    expect(payload.config.bucket).toBe('charon-backups')
    expect(payload.secrets.access_key_id).toBe('AKIAEXAMPLE')
    expect(payload.secrets.secret_access_key).toBe('super-secret-key')
  })

  it('submits an SFTP target with host/port/path/username/password and supports host-key discovery on a draft (new) target', async () => {
    mockTestDraftMutate.mockImplementation((_payload, { onSuccess }) => {
      onSuccess({ success: false, message: 'host key not yet trusted', discovered_fingerprint: 'SHA256:abcdef1234567890' })
    })
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /sftp/i }))

    await user.type(within(dialog).getByLabelText(/^host/i), 'nas.lan')
    await user.clear(within(dialog).getByLabelText(/^port/i))
    await user.type(within(dialog).getByLabelText(/^port/i), '22')
    await user.type(within(dialog).getByLabelText(/^path/i), '/backups/charon')
    await user.type(within(dialog).getByLabelText(/username/i), 'charon')
    await user.type(within(dialog).getByLabelText(/^password/i), 'super-secret-password')

    await user.click(within(dialog).getByRole('button', { name: /discover host key/i }))

    expect(mockTestDraftMutate).toHaveBeenCalledTimes(1)
    const [draftPayload] = mockTestDraftMutate.mock.calls[0]
    expect(draftPayload).toEqual({
      type: 'sftp',
      config: { host: 'nas.lan', port: 22, path: '/backups/charon', username: 'charon' },
    })
    expect(mockTestMutate).not.toHaveBeenCalled()

    expect(within(dialog).getByTestId('backup-remote-target-host-key-fingerprint')).toHaveTextContent(
      'SHA256:abcdef1234567890'
    )

    await user.click(within(dialog).getByRole('button', { name: /confirm host key/i }))
    expect(within(dialog).getByTestId('backup-remote-target-host-key-input')).toHaveValue('SHA256:abcdef1234567890')

    await user.click(within(dialog).getByRole('button', { name: /save|create/i }))
    const [payload] = mockCreateMutate.mock.calls[0]
    expect(payload.type).toBe('sftp')
    expect(payload.config.host).toBe('nas.lan')
    expect(payload.config.port).toBe(22)
    expect(payload.config.username).toBe('charon')
    expect(payload.secrets.password).toBe('super-secret-password')
  })

  it('supports host-key discovery via the by-uuid test endpoint when editing an existing target', async () => {
    mockTestMutate.mockImplementation((_uuid, { onSuccess }) => {
      onSuccess({ success: false, message: 'host key not yet trusted', discovered_fingerprint: 'SHA256:existing1234567890' })
    })
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /discover host key/i }))

    expect(mockTestMutate).toHaveBeenCalledTimes(1)
    expect(mockTestMutate.mock.calls[0][0]).toBe('r1')
    expect(mockTestDraftMutate).not.toHaveBeenCalled()

    expect(within(dialog).getByTestId('backup-remote-target-host-key-fingerprint')).toHaveTextContent(
      'SHA256:existing1234567890'
    )
  })

  it('renders secret fields as type="password" and blank on edit, with a keep-current hint', () => {
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    const passwordField = within(dialog).getByLabelText(/^password/i)
    expect(passwordField).toHaveAttribute('type', 'password')
    expect(passwordField).toHaveValue('')
    expect(within(dialog).getByText(/leave blank to keep current/i)).toBeVisible()
  })

  it('does not show the type radio group when editing an existing target', () => {
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)
    expect(screen.queryByRole('radio', { name: /s3/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: /sftp/i })).not.toBeInTheDocument()
  })

  it('calls update (not create) with the uuid when editing', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /save/i }))

    expect(mockUpdateMutate).toHaveBeenCalledTimes(1)
    const [{ uuid }] = mockUpdateMutate.mock.calls[0]
    expect(uuid).toBe('r1')
    expect(mockCreateMutate).not.toHaveBeenCalled()
  })

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RemoteTargetFormDialog open target={null} onClose={onClose} />)

    await user.click(screen.getByRole('button', { name: /cancel/i }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls onClose when the dialog is dismissed (e.g. Escape)', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RemoteTargetFormDialog open target={null} onClose={onClose} />)

    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })

  it('calls onClose on a successful edit and shows an error toast on failure', async () => {
    const { toast } = await import('../../../utils/toast')
    const onCloseSuccess = vi.fn()
    mockUpdateMutate.mockImplementationOnce((_vars, { onSuccess }) => onSuccess())
    const user = userEvent.setup()
    const { rerender } = render(<RemoteTargetFormDialog open target={nas} onClose={onCloseSuccess} />)
    await user.click(screen.getByRole('button', { name: /save/i }))
    expect(onCloseSuccess).toHaveBeenCalled()

    mockUpdateMutate.mockImplementationOnce((_vars, { onError }) => onError(new Error('update failed')))
    rerender(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)
    await user.click(screen.getByRole('button', { name: /save/i }))
    expect(toast.error).toHaveBeenCalledWith('update failed')
  })

  it('shows an error toast when create fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockCreateMutate.mockImplementationOnce((_payload, { onError }) => onError(new Error('create failed')))
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: /save|create/i }))
    expect(toast.error).toHaveBeenCalledWith('create failed')
  })

  it('shows an error toast when draft host-key discovery fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestDraftMutate.mockImplementationOnce((_payload, { onError }) => onError(new Error('dial failed')))
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /sftp/i }))
    await user.click(within(dialog).getByRole('button', { name: /discover host key/i }))

    expect(toast.error).toHaveBeenCalledWith('dial failed')
  })

  it('shows an error toast when by-uuid host-key discovery fails on an existing target', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestMutate.mockImplementationOnce((_uuid, { onError }) => onError(new Error('dial failed')))
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: /discover host key/i }))

    expect(toast.error).toHaveBeenCalledWith('dial failed')
  })

  it('allows switching between s3 and sftp and back again', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByLabelText(/endpoint/i)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('radio', { name: /sftp/i }))
    expect(within(dialog).getByLabelText(/^host/i)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('radio', { name: /s3/i }))
    expect(within(dialog).getByLabelText(/endpoint/i)).toBeInTheDocument()
  })

  it('toggles the force-path-style checkbox', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    const checkbox = within(dialog).getByLabelText(/force path style/i)
    expect(checkbox).not.toBeChecked()
    await user.click(checkbox)
    expect(checkbox).toBeChecked()
  })

  it('shows keep-current placeholders for S3 secret fields when editing an S3 target', () => {
    render(<RemoteTargetFormDialog open target={b2} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByLabelText(/access key id/i)).toHaveAttribute(
      'placeholder',
      'Leave blank to keep current'
    )
    expect(within(dialog).getByLabelText(/secret access key/i)).toHaveAttribute(
      'placeholder',
      'Leave blank to keep current'
    )
  })

  it('allows manually typing a host key fingerprint', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={nas} onClose={vi.fn()} />)

    const input = screen.getByTestId('backup-remote-target-host-key-input')
    await user.clear(input)
    await user.type(input, 'SHA256:manual')
    expect(input).toHaveValue('SHA256:manual')
  })

  it('submits a WebDAV target with url/username/base_path/insecure-skip-verify/password', async () => {
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /webdav/i }))

    await user.type(within(dialog).getByLabelText(/webdav url/i), 'https://nas.example.com/remote.php/dav/files/charon/')
    await user.type(within(dialog).getByLabelText(/username/i), 'charon')
    await user.type(within(dialog).getByLabelText(/base path/i), '/charon-backups')
    await user.click(within(dialog).getByLabelText(/skip tls certificate verification/i))
    await user.type(within(dialog).getByLabelText(/^password/i), 'webdav-secret')

    await user.click(within(dialog).getByRole('button', { name: /create/i }))

    expect(mockCreateMutate).toHaveBeenCalledTimes(1)
    const [payload] = mockCreateMutate.mock.calls[0]
    expect(payload.type).toBe('webdav')
    expect(payload.config.webdav).toEqual({
      url: 'https://nas.example.com/remote.php/dav/files/charon/',
      username: 'charon',
      base_path: '/charon-backups',
      insecure_skip_verify: true,
    })
    expect(payload.secrets).toEqual({ password: 'webdav-secret' })
  })

  it('renders "Save & Connect" for Dropbox, creates the target, then redirects to the authorize_url', async () => {
    mockStartOAuthMutate.mockImplementation((_uuid, { onSuccess }) => {
      onSuccess({ authorize_url: 'https://www.dropbox.com/oauth2/authorize?client_id=abc123' })
    })
    mockCreateMutate.mockImplementation((_payload, { onSuccess }) => {
      onSuccess({ uuid: 'new-uuid', name: 'Dropbox' })
    })
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={onClose} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /dropbox/i }))

    expect(within(dialog).getByRole('button', { name: /save & connect/i })).toBeInTheDocument()

    await user.type(within(dialog).getByLabelText(/app key/i), 'abc123')
    await user.type(within(dialog).getByLabelText(/app secret/i), 'dropbox-app-secret')
    await user.type(within(dialog).getByLabelText(/folder path/i), '/charon-backups')

    await user.click(within(dialog).getByRole('button', { name: /save & connect/i }))

    expect(mockCreateMutate).toHaveBeenCalledTimes(1)
    const [payload] = mockCreateMutate.mock.calls[0]
    expect(payload.type).toBe('dropbox')
    expect(payload.config.dropbox).toEqual({ app_key: 'abc123', folder_path: '/charon-backups' })
    expect(payload.secrets).toEqual({ oauth_client_secret: 'dropbox-app-secret' })

    expect(mockStartOAuthMutate).toHaveBeenCalledWith('new-uuid', expect.any(Object))
    expect(window.location.href).toBe('https://www.dropbox.com/oauth2/authorize?client_id=abc123')
    // Full-page redirect — onClose is intentionally not called (spec §3.6).
    expect(onClose).not.toHaveBeenCalled()
  })

  it('submits a Google Drive target with client_id/client_secret/folder_path and starts OAuth', async () => {
    mockStartOAuthMutate.mockImplementation((_uuid, { onSuccess }) => {
      onSuccess({ authorize_url: 'https://accounts.google.com/o/oauth2/v2/auth?client_id=xyz' })
    })
    mockCreateMutate.mockImplementation((_payload, { onSuccess }) => {
      onSuccess({ uuid: 'gdrive-uuid', name: 'Google Drive' })
    })
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /google drive/i }))

    await user.type(within(dialog).getByLabelText(/client id/i), 'xyz')
    await user.type(within(dialog).getByLabelText(/client secret/i), 'gdrive-secret')
    await user.type(within(dialog).getByLabelText(/folder path/i), 'Charon/Backups')

    await user.click(within(dialog).getByRole('button', { name: /save & connect/i }))

    const [payload] = mockCreateMutate.mock.calls[0]
    expect(payload.type).toBe('google_drive')
    expect(payload.config.google_drive).toEqual({ client_id: 'xyz', folder_path: 'Charon/Backups' })
    expect(payload.secrets).toEqual({ oauth_client_secret: 'gdrive-secret' })
    expect(mockStartOAuthMutate).toHaveBeenCalledWith('gdrive-uuid', expect.any(Object))
    expect(window.location.href).toBe('https://accounts.google.com/o/oauth2/v2/auth?client_id=xyz')
  })

  it('shows a toast and closes the dialog (without redirecting) when oauth/start fails, e.g. public_url_not_configured', async () => {
    const { toast } = await import('../../../utils/toast')
    mockStartOAuthMutate.mockImplementation((_uuid, { onError }) => {
      onError(new Error('app.public_url is not configured'))
    })
    mockCreateMutate.mockImplementation((_payload, { onSuccess }) => {
      onSuccess({ uuid: 'new-uuid', name: 'Dropbox' })
    })
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={onClose} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /dropbox/i }))
    await user.type(within(dialog).getByLabelText(/app key/i), 'abc123')

    await user.click(within(dialog).getByRole('button', { name: /save & connect/i }))

    expect(toast.error).toHaveBeenCalledWith('app.public_url is not configured')
    // The target was already created server-side; the dialog just closes
    // (no redirect) so the admin can retry Connect later (spec §3.9).
    expect(onClose).toHaveBeenCalled()
  })

  it('does not trigger the OAuth flow when editing an existing Dropbox target (renders "Save", not "Save & Connect")', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RemoteTargetFormDialog open target={dropboxTarget} onClose={onClose} />)

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: /save & connect/i })).not.toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /^save$/i })).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: /^save$/i }))

    expect(mockUpdateMutate).toHaveBeenCalledTimes(1)
    expect(mockStartOAuthMutate).not.toHaveBeenCalled()
    expect(mockCreateMutate).not.toHaveBeenCalled()
  })
})
