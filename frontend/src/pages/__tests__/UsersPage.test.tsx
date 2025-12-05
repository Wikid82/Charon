import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import UsersPage from '../UsersPage'
import * as usersApi from '../../api/users'
import * as proxyHostsApi from '../../api/proxyHosts'

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
}))

vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
}))

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

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
]

const mockProxyHosts = [
  {
    uuid: 'host-1',
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
  })

  it('renders loading state initially', () => {
    vi.mocked(usersApi.listUsers).mockReturnValue(new Promise(() => {}))

    renderWithProviders(<UsersPage />)

    expect(document.querySelector('.animate-spin')).toBeTruthy()
  })

  it('renders user list', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithProviders(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('User Management')).toBeTruthy()
    })

    expect(screen.getByText('Admin User')).toBeTruthy()
    expect(screen.getByText('admin@example.com')).toBeTruthy()
    expect(screen.getByText('Regular User')).toBeTruthy()
    expect(screen.getByText('user@example.com')).toBeTruthy()
  })

  it('shows pending invite status', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithProviders(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Pending Invite')).toBeTruthy()
    })
  })

  it('shows active status for accepted users', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithProviders(<UsersPage />)

    await waitFor(() => {
      expect(screen.getAllByText('Active').length).toBeGreaterThan(0)
    })
  })

  it('opens invite modal when clicking invite button', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)

    renderWithProviders(<UsersPage />)

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

    renderWithProviders(<UsersPage />)

    await waitFor(() => {
      expect(screen.getAllByText('Blacklist').length).toBeGreaterThan(0)
    })

    expect(screen.getByText('Whitelist')).toBeTruthy()
  })

  it('toggles user enabled status', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.updateUser).mockResolvedValue({ message: 'Updated' })

    renderWithProviders(<UsersPage />)

    await waitFor(() => {
      expect(screen.getByText('Regular User')).toBeTruthy()
    })

    // Find the switch for the non-admin user and toggle it
    const switches = screen.getAllByRole('checkbox')
    // The second switch should be for the regular user (admin switch is disabled)
    const userSwitch = switches.find(
      (sw) => !(sw as HTMLInputElement).disabled && (sw as HTMLInputElement).checked
    )

    if (userSwitch) {
      const user = userEvent.setup()
      await user.click(userSwitch)

      await waitFor(() => {
        expect(usersApi.updateUser).toHaveBeenCalledWith(2, { enabled: false })
      })
    }
  })

  it('invites a new user', async () => {
    vi.mocked(usersApi.listUsers).mockResolvedValue(mockUsers)
    vi.mocked(usersApi.inviteUser).mockResolvedValue({
      id: 4,
      uuid: 'new-user',
      email: 'new@example.com',
      role: 'user',
      invite_token: 'test-token-123',
      email_sent: false,
      expires_at: '2024-01-03T00:00:00Z',
    })

    renderWithProviders(<UsersPage />)

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
    await user.click(screen.getByRole('button', { name: /Send Invite/i }))

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

    renderWithProviders(<UsersPage />)

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
})
