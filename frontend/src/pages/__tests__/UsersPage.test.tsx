import { screen, waitFor, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import client from '../../api/client'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as usersApi from '../../api/users'
import { useAuth } from '../../hooks/useAuth'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import UsersPage from '../UsersPage'

// Mock APIs
vi.mock('../../api/users', () => ({
  listUsers: vi.fn(),
  getUser: vi.fn(),
  createUser: vi.fn(),
  inviteUser: vi.fn(),
  updateUser: vi.fn(),
  deleteUser: vi.fn(),
  updateUserPermissions: vi.fn(),
  validateInvite: vi.fn(),
  acceptInvite: vi.fn(),
  previewInviteURL: vi.fn(),
  resendInvite: vi.fn(),
  getProfile: vi.fn(),
  updateProfile: vi.fn(),
  regenerateApiKey: vi.fn(),
}))

vi.mock('../../hooks/useAuth', () => ({
  useAuth: vi.fn().mockReturnValue({
    user: { user_id: 1, role: 'admin', name: 'Admin User', email: 'admin@example.com' },
    changePassword: vi.fn().mockResolvedValue(undefined),
    isAuthenticated: true,
    isLoading: false,
    login: vi.fn(),
    logout: vi.fn(),
  }),
}))

vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
}))

vi.mock('../../api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockUsers = [
  {
    id: 1,
    uuid: '123-456',
    email: 'admin@example.com',
    name: 'Admin User',
    role: 'admin' as const,
    enabled: true,
    permission_mode: 'allow_all' as const,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    uuid: '789-012',
    email: 'user@example.com',
    name: 'Regular User',
    role: 'user' as const,
    enabled: true,
    invite_status: 'accepted' as const,
    permission_mode: 'allow_all' as const,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 3,
    uuid: '345-678',
    email: 'pending@example.com',
    name: '',
    role: 'user' as const,
    enabled: false,
    invite_status: 'pending' as const,
    permission_mode: 'deny_all' as const,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 4,
    uuid: '999-000',
    email: 'passthrough@example.com',
    name: 'Passthrough User',
    role: 'passthrough' as const,
    enabled: true,
    invite_status: 'accepted' as const,
    permission_mode: 'allow_all' as const,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
]

const mockProxyHosts = [
  {
    uuid: '1',
    name: 'Test Host',
    domain_names: 'test.example.com',
    forward_scheme: 'http',
    forward_host: 'localhost',
    forward_port: 8080,
    ssl_forced: true,
    http2_support: true,
    hsts_enabled: true,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: false,
    application: 'none' as const,
    locations: [],
    enabled: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
]

describe('UsersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(mockProxyHosts)
    vi.mocked(toast.success).mockClear()
    vi.mocked(toast.error).mockClear()
  })

  it('renders loading state initially', () => {
    vi.mocked(usersApi.listUsers).mockReturnValue(new Promise(() => {}))

    renderWithQueryClient(<UsersPage />)

    expect(document.querySelector('.animate-spin')).toBeTruthy()
  })

  it('renders user list', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('User Management')).toBeTruthy()
    })

    expect(screen.getAllByText('Admin User').length).toBeGreaterThan(0)
    expect(screen.getAllByText('admin@example.com').length).toBeGreaterThan(0)
    expect(screen.getByText('Regular User')).toBeTruthy()
    expect(screen.getByText('user@example.com')).toBeTruthy()
  })

  it('shows pending invite status', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Pending Invite')).toBeTruthy()
    })
  })

  it('shows active status for accepted users', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getAllByText('Active').length).toBeGreaterThan(0)
    })
  })

  it('opens invite modal when clicking invite button', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Invite User')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Invite User/i }))

    await waitFor(() => {
      expect(screen.getByPlaceholderText('user@example.com')).toBeTruthy()
    })
  })

  it('shows permission mode in user list', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getAllByText('Blacklist').length).toBeGreaterThan(0)
    })

    expect(screen.getByText('Whitelist')).toBeTruthy()
  })

  it('toggles user enabled status', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.updateUser).mockResolvedValue({ message: 'Updated' })

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Regular User')).toBeTruthy()
    })

    // Find the switch for the non-admin user and toggle it
    const switches = screen.getAllByRole('checkbox')
    // The second switch should be for the regular user (admin switch is disabled)
    const userSwitch = switches.find(
      (sw) => !(sw as HTMLInputElement).disabled && (sw as HTMLInputElement).checked
    )

    expect(userSwitch).toBeDefined()
    const user = userEvent.setup()
    await user.click(userSwitch!)

    await waitFor(() => {
      expect(usersApi.updateUser).toHaveBeenCalledWith(2, { enabled: false })
    })
  })

  it('invites a new user', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.inviteUser).mockResolvedValue({
      id: 4,
      uuid: 'new-user',
      email: 'new@example.com',
      role: 'user',
      invite_token_masked: '********',
      invite_url: '[REDACTED]',
      email_sent: false,
      expires_at: '2024-01-03T00:00:00Z',
    })

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Invite User')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Invite User/i }))

    // Wait for modal to open - look for the modal's email input placeholder
    await waitFor(() => {
      expect(screen.getByPlaceholderText('user@example.com')).toBeTruthy()
    })

    await user.type(screen.getByPlaceholderText('user@example.com'), 'new@example.com')
    await user.click(screen.getByRole('button', { name: /^Send Invite$/i }))

    await waitFor(() => {
      expect(usersApi.inviteUser).toHaveBeenCalledWith({
        email: 'new@example.com',
        role: 'user',
        permission_mode: 'allow_all',
        permitted_hosts: [],
      })
    })
  })

  it('deletes a user after confirmation', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.deleteUser).mockResolvedValue({ message: 'Deleted' })

    // Mock window.confirm
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Regular User')).toBeTruthy()
    })

    // Find delete buttons (trash icons) - admin user's delete button is disabled
    const deleteButtons = screen.getAllByTitle('Delete User')
    // Find the first non-disabled delete button
    const enabledDeleteButton = deleteButtons.find((btn) => !(btn as HTMLButtonElement).disabled)

    expect(enabledDeleteButton).toBeTruthy()

    const user = userEvent.setup()
    await user.click(enabledDeleteButton!)

    await waitFor(() => {
      expect(confirmSpy).toHaveBeenCalledWith('Are you sure you want to delete this user?')
    })

    await waitFor(() => {
      expect(usersApi.deleteUser).toHaveBeenCalled()
    })

    confirmSpy.mockRestore()
  })

  it('updates user permissions from the modal', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.updateUserPermissions).mockResolvedValue({ message: 'ok' })

    renderWithQueryClient(<UsersPage />)

    expect(await screen.findByText('Regular User')).toBeInTheDocument()

    const editButtons = screen.getAllByTitle('Edit Permissions')
    const firstEditable = editButtons.find((btn) => !(btn as HTMLButtonElement).disabled)
    expect(firstEditable).toBeTruthy()

    const user = userEvent.setup()
    await user.click(firstEditable!)

    const modal = await screen.findByText(/Edit Permissions/i)
    const modalContainer = modal.closest('.bg-dark-card') as HTMLElement

    // Switch to whitelist (deny_all) and toggle first host
    const modeSelect = within(modalContainer).getByDisplayValue('Allow All (Blacklist)')
    await user.selectOptions(modeSelect, 'deny_all')
    const checkbox = within(modalContainer).getByLabelText(/Test Host/) as HTMLInputElement
    expect(checkbox.checked).toBe(false)
    await user.click(checkbox)

    await user.click(screen.getByRole('button', { name: 'Save Permissions' }))

    await waitFor(() => {
      expect(usersApi.updateUserPermissions).toHaveBeenCalledWith(2, {
        permission_mode: 'deny_all',
        permitted_hosts: expect.arrayContaining([expect.any(Number)]),
      })
      expect(toast.success).toHaveBeenCalledWith('Permissions updated')
    })
  })

  it('hides invite link when backend returns a redacted URL', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.inviteUser).mockResolvedValue({
      id: 5,
      uuid: 'invitee',
      email: 'manual@example.com',
      role: 'user',
      invite_token_masked: '********',
      invite_url: '[REDACTED]',
      email_sent: false,
      expires_at: '2025-01-01T00:00:00Z',
    })

    renderWithQueryClient(<UsersPage />)

    const user = userEvent.setup()
    expect(await screen.findByText('Invite User')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Invite User/i }))
    await user.type(screen.getByPlaceholderText('user@example.com'), 'manual@example.com')
    await user.click(screen.getByRole('button', { name: /^Send Invite$/i }))

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /copy invite link/i })).not.toBeInTheDocument()
      expect(screen.queryByDisplayValue('[REDACTED]')).not.toBeInTheDocument()
    })
  })

  it('renders passthrough role badge', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Passthrough')).toBeTruthy()
    })
  })

  it('renders My Profile card for current user', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('My Profile')).toBeTruthy()
    })
  })

  it('shows passthrough option in invite role select', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    const user = userEvent.setup()
    expect(await screen.findByText('Invite User')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: /Invite User/i }))

    await waitFor(() => {
      const roleSelect = screen.getByLabelText('Role') as HTMLSelectElement
      const options = Array.from(roleSelect.options).map(o => o.value)
      expect(options).toContain('passthrough')
    })
  })

  it('opens detail modal when edit button is clicked', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithQueryClient(<UsersPage />)

    expect(await screen.findByText('Regular User')).toBeTruthy()

    const user = userEvent.setup()
    const editButtons = screen.getAllByTitle('Edit User')
    await user.click(editButtons[1])

    await waitFor(() => {
      expect(screen.getByRole('dialog', { name: /Edit User/i })).toBeTruthy()
    })
  })

  describe('URL Preview in InviteModal', () => {
    afterEach(() => {
      vi.useRealTimers()
    })

    it('shows URL preview when valid email is entered', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'https://charon.example.com',
          is_configured: true,
          warning: false,
          warning_message: '',
        },
      })

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))

      const emailInput = screen.getByPlaceholderText('user@example.com')
      await user.type(emailInput, 'test@example.com')

      await waitFor(() => {
        expect(client.post).toHaveBeenCalledWith('/users/preview-invite-url', { email: 'test@example.com' })
      }, { timeout: 1000 })

      // Look for the preview URL content with ellipsis replacing the token
      await waitFor(() => {
        expect(screen.getByText('https://charon.example.com/accept-invite?token=...')).toBeInTheDocument()
      }, { timeout: 1000 })
    })

    it('debounces URL preview for 500ms', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'https://example.com',
          is_configured: true,
          warning: false,
          warning_message: '',
        },
      })

      renderWithQueryClient(<UsersPage />)
      const user = userEvent.setup()

      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))
      expect(await screen.findByPlaceholderText('user@example.com')).toBeInTheDocument()

      vi.useFakeTimers()

      try {
        const emailInput = screen.getByPlaceholderText('user@example.com')
        fireEvent.change(emailInput, { target: { value: 'test@example.com' } })

        // Verify not called immediately
        expect(client.post).not.toHaveBeenCalled()

        await act(async () => {
          await vi.advanceTimersByTimeAsync(550)
        })

        expect(client.post).toHaveBeenCalledTimes(1)
        expect(client.post).toHaveBeenCalledWith('/users/preview-invite-url', { email: 'test@example.com' })
      } finally {
        vi.useRealTimers()
      }
    })

    it('replaces sample token with ellipsis in preview', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'https://example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'https://example.com',
          is_configured: true,
          warning: false,
          warning_message: '',
        },
      })

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))

      const emailInput = screen.getByPlaceholderText('user@example.com')
      await user.type(emailInput, 'test@example.com')

      await waitFor(() => {
        const preview = screen.getByText('https://example.com/accept-invite?token=...')

        expect(preview.textContent).toContain('...')
        expect(preview.textContent).not.toContain('SAMPLE_TOKEN_PREVIEW')
      }, { timeout: 1000 })
    })

    it('shows warning when not configured', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(client.post).mockResolvedValue({
        data: {
          preview_url: 'http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW',
          base_url: 'http://localhost:8080',
          is_configured: false,
          warning: true,
          warning_message: 'Application URL not configured',
        },
      })

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))

      const emailInput = screen.getByPlaceholderText('user@example.com')
      await user.type(emailInput, 'test@example.com')

      await waitFor(() => {
        // Look for link to system settings
        const link = screen.getByRole('link')
        expect(link.getAttribute('href')).toContain('/settings/system')
      }, { timeout: 1000 })
    })

    it('does not show preview when email is invalid', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))

      const emailInput = screen.getByPlaceholderText('user@example.com')
      await user.type(emailInput, 'invalid')

      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 600))
      })

      // Preview should not be fetched or displayed
      expect(client.post).not.toHaveBeenCalled()
    })

    it('handles preview API error gracefully', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(client.post).mockRejectedValue(new Error('API error'))

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: /Invite User/i }))

      const emailInput = screen.getByPlaceholderText('user@example.com')
      await user.type(emailInput, 'test@example.com')

      // Wait for debounce
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 600))
      })

      await waitFor(() => {
        expect(client.post).toHaveBeenCalledWith('/users/preview-invite-url', { email: 'test@example.com' })
      }, { timeout: 1000 })

      // Verify preview is not displayed after error
      const previewQuery = screen.queryByText(/accept-invite/)
      expect(previewQuery).toBeNull()
    })
  })

  describe('InviteModal role reset on close', () => {
    it('resets role to user when modal is closed', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Invite User')).toBeInTheDocument()

      // Open invite modal
      await user.click(screen.getByRole('button', { name: /Invite User/i }))
      expect(await screen.findByLabelText(/Role/i)).toBeInTheDocument()

      // Change role to passthrough
      await user.selectOptions(screen.getByLabelText(/Role/i), 'passthrough')
      expect((screen.getByLabelText(/Role/i) as HTMLSelectElement).value).toBe('passthrough')

      // Close via Cancel button (calls handleClose which resets role)
      await user.click(screen.getByRole('button', { name: /^Cancel$/i }))

      // Reopen modal — role should be reset to 'user'
      await user.click(screen.getByRole('button', { name: /Invite User/i }))
      expect(await screen.findByLabelText(/Role/i)).toBeInTheDocument()
      expect((screen.getByLabelText(/Role/i) as HTMLSelectElement).value).toBe('user')
    })
  })

  describe('UserDetailModal', () => {
    it('shows profile update error via toast', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.updateUser).mockRejectedValue({
        response: { data: { error: 'Email already in use' } },
      })

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Regular User')).toBeInTheDocument()

      // Click Edit User for Regular User (second "Edit User" button in the table)
      const editButtons = screen.getAllByTitle('Edit User')
      await user.click(editButtons[1]) // index 1 = Regular User row

      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      // Click Save
      await user.click(screen.getByRole('button', { name: /^Save$/i }))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })

    it('toggles the password change section', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'abc-****' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      // Click Edit User in My Profile card (opens with isSelf=true) — card button is first
      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])

      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      // Password fields should not be visible until toggled
      expect(screen.queryByLabelText(/Current Password/i)).toBeNull()

      // Click the Change Password toggle
      await user.click(screen.getAllByRole('button', { name: /Change Password/i })[0])

      // Password fields should now be visible
      await waitFor(() => {
        expect(screen.getByLabelText(/Current Password/i)).toBeInTheDocument()
      })
    })

    it('submits password change successfully', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'abc-****' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      // Expand password section
      await user.click(screen.getAllByRole('button', { name: /Change Password/i })[0])
      expect(await screen.findByLabelText(/Current Password/i)).toBeInTheDocument()

      // Fill matching passwords
      await user.type(screen.getByLabelText(/Current Password/i), 'oldpass123')
      await user.type(screen.getByLabelText(/^New Password/i), 'newpass456')
      await user.type(screen.getByLabelText(/Confirm Password/i), 'newpass456')

      // Submit button (second "Change Password" button — the submit one)
      const changePasswordButtons = screen.getAllByRole('button', { name: /Change Password/i })
      const submitButton = changePasswordButtons[changePasswordButtons.length - 1]
      await user.click(submitButton)

      await waitFor(() => {
        expect(toast.success).toHaveBeenCalled()
      })
    })

    it('shows error toast on password change failure', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'abc-****' } as never)
      vi.mocked(useAuth).mockReturnValue({
        user: { user_id: 1, role: 'admin', name: 'Admin User', email: 'admin@example.com' },
        changePassword: vi.fn().mockRejectedValue(new Error('Invalid current password')),
        isAuthenticated: true,
        isLoading: false,
        login: vi.fn(),
        logout: vi.fn(),
      })

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Change Password/i })[0])
      expect(await screen.findByLabelText(/Current Password/i)).toBeInTheDocument()

      await user.type(screen.getByLabelText(/Current Password/i), 'wrongpass')
      await user.type(screen.getByLabelText(/^New Password/i), 'newpass456')
      await user.type(screen.getByLabelText(/Confirm Password/i), 'newpass456')

      const changePasswordButtons = screen.getAllByRole('button', { name: /Change Password/i })
      const submitButton = changePasswordButtons[changePasswordButtons.length - 1]
      await user.click(submitButton)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith('Invalid current password')
      })
    })

    it('regenerates API key when user confirms', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'old-****' } as never)
      vi.mocked(usersApi.regenerateApiKey).mockResolvedValue({ api_key_masked: 'new-****' } as never)
      vi.spyOn(window, 'confirm').mockReturnValue(true)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Regenerate API Key/i })).toBeInTheDocument()
      })

      await user.click(screen.getByRole('button', { name: /Regenerate API Key/i }))

      await waitFor(() => {
        expect(usersApi.regenerateApiKey).toHaveBeenCalled()
      })
    })

    it('updates self profile and shows profile updated toast', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.updateProfile).mockResolvedValue({ message: 'ok' } as never)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'abc-****' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      const dialog = screen.getByRole('dialog')
      await user.click(within(dialog).getByRole('button', { name: /^Save$/i }))

      await waitFor(() => {
        expect(usersApi.updateProfile).toHaveBeenCalled()
        expect(toast.success).toHaveBeenCalledWith('Profile updated successfully')
      })
    })

    it('updates non-self user profile and shows success toast', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.updateUser).mockResolvedValue({ message: 'ok' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('Regular User')).toBeInTheDocument()

      const editButtons = screen.getAllByTitle('Edit User')
      await user.click(editButtons[1])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      const dialog = screen.getByRole('dialog')
      await user.click(within(dialog).getByRole('button', { name: /^Save$/i }))

      await waitFor(() => {
        expect(usersApi.updateUser).toHaveBeenCalledWith(2, expect.objectContaining({
          email: 'user@example.com',
        }))
        expect(toast.success).toHaveBeenCalledWith('Profile updated successfully')
      })
    })

    it('displays masked API key text when profile query resolves', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: 'SK-****-masktest' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      await waitFor(() => {
        expect(screen.getByText('SK-****-masktest')).toBeInTheDocument()
      })
    })

    it('shows password mismatch alert when new and confirm passwords differ', async () => {
      vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
      vi.mocked(usersApi.getProfile).mockResolvedValue({ api_key_masked: '' } as never)

      renderWithQueryClient(<UsersPage />)

      const user = userEvent.setup()
      expect(await screen.findByText('My Profile')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Edit User/i })[0])
      expect(await screen.findByRole('dialog')).toBeInTheDocument()

      await user.click(screen.getAllByRole('button', { name: /Change Password/i })[0])
      expect(await screen.findByLabelText(/Current Password/i)).toBeInTheDocument()

      await user.type(screen.getByLabelText(/Current Password/i), 'current123')
      await user.type(screen.getByLabelText(/^New Password/i), 'newpass1')
      await user.type(screen.getByLabelText(/Confirm Password/i), 'different2')

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument()
        expect(screen.getByText('Passwords do not match')).toBeInTheDocument()
      })
    })
  })
})
