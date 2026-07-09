import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { RemoteTargetFormDialog } from '../RemoteTargetFormDialog'
import type { RemoteTarget } from '../../../hooks/useRemoteTargets'

const mockCreateMutate = vi.fn()
const mockUpdateMutate = vi.fn()
const mockTestMutate = vi.fn()
let createPending = false
let updatePending = false
let testPending = false

vi.mock('../../../hooks/useRemoteTargets', async () => {
  const actual = await vi.importActual<typeof import('../../../hooks/useRemoteTargets')>('../../../hooks/useRemoteTargets')
  return {
    ...actual,
    useCreateRemoteTarget: () => ({ mutate: mockCreateMutate, isPending: createPending }),
    useUpdateRemoteTarget: () => ({ mutate: mockUpdateMutate, isPending: updatePending }),
    useTestRemoteTarget: () => ({ mutate: mockTestMutate, isPending: testPending }),
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

describe('RemoteTargetFormDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createPending = false
    updatePending = false
    testPending = false
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

  it('submits an SFTP target with host/port/path/username/password and supports host-key discovery', async () => {
    mockTestMutate.mockImplementation((_uuid, { onSuccess }) => {
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

  it('shows an error toast when host-key discovery fails', async () => {
    const { toast } = await import('../../../utils/toast')
    mockTestMutate.mockImplementationOnce((_uuid, { onError }) => onError(new Error('dial failed')))
    const user = userEvent.setup()
    render(<RemoteTargetFormDialog open target={null} onClose={vi.fn()} />)

    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('radio', { name: /sftp/i }))
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
})
