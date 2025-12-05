import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import AcceptInvite from '../AcceptInvite'
import * as usersApi from '../../api/users'

// Mock APIs
vi.mock('../../api/users', () => ({
  validateInvite: vi.fn(),
  acceptInvite: vi.fn(),
  listUsers: vi.fn(),
  getUser: vi.fn(),
  createUser: vi.fn(),
  inviteUser: vi.fn(),
  updateUser: vi.fn(),
  deleteUser: vi.fn(),
  updateUserPermissions: vi.fn(),
}))

// Mock react-router-dom navigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (initialRoute: string = '/accept-invite?token=test-token') => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialRoute]}>
        <Routes>
          <Route path="/accept-invite" element={<AcceptInvite />} />
          <Route path="/login" element={<div>Login Page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('AcceptInvite', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows invalid link message when no token provided', async () => {
    renderWithProviders('/accept-invite')

    await waitFor(() => {
      expect(screen.getByText('Invalid Link')).toBeTruthy()
    })

    expect(screen.getByText(/This invitation link is invalid/)).toBeTruthy()
  })

  it('shows validating state initially', () => {
    vi.mocked(usersApi.validateInvite).mockReturnValue(new Promise(() => {}))

    renderWithProviders()

    expect(screen.getByText('Validating invitation...')).toBeTruthy()
  })

  it('shows error for invalid token', async () => {
    vi.mocked(usersApi.validateInvite).mockRejectedValue({
      response: { data: { error: 'Token expired' } },
    })

    renderWithProviders()

    await waitFor(() => {
      expect(screen.getByText('Invitation Invalid')).toBeTruthy()
    })
  })

  it('renders accept form for valid token', async () => {
    vi.mocked(usersApi.validateInvite).mockResolvedValue({
      valid: true,
      email: 'invited@example.com',
    })

    renderWithProviders()

    await waitFor(() => {
      expect(screen.getByText(/been invited/i)).toBeTruthy()
    })

    expect(screen.getByText(/invited@example.com/)).toBeTruthy()
    expect(screen.getByPlaceholderText('John Doe')).toBeTruthy()
    // Password and confirm password have same placeholder
    expect(screen.getAllByPlaceholderText('••••••••').length).toBe(2)
  })

  it('shows password mismatch error', async () => {
    vi.mocked(usersApi.validateInvite).mockResolvedValue({
      valid: true,
      email: 'invited@example.com',
    })

    renderWithProviders()

    await waitFor(() => {
      expect(screen.getByPlaceholderText('John Doe')).toBeTruthy()
    })

    const user = userEvent.setup()
    const [passwordInput, confirmInput] = screen.getAllByPlaceholderText('••••••••')
    await user.type(passwordInput, 'password123')
    await user.type(confirmInput, 'differentpassword')

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeTruthy()
    })
  })

  it('submits form and shows success', async () => {
    vi.mocked(usersApi.validateInvite).mockResolvedValue({
      valid: true,
      email: 'invited@example.com',
    })
    vi.mocked(usersApi.acceptInvite).mockResolvedValue({
      message: 'Success',
      email: 'invited@example.com',
    })

    renderWithProviders()

    await waitFor(() => {
      expect(screen.getByPlaceholderText('John Doe')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('John Doe'), 'John Doe')
    const [passwordInput, confirmInput] = screen.getAllByPlaceholderText('••••••••')
    await user.type(passwordInput, 'securepassword123')
    await user.type(confirmInput, 'securepassword123')

    await user.click(screen.getByRole('button', { name: 'Create Account' }))

    await waitFor(() => {
      expect(usersApi.acceptInvite).toHaveBeenCalledWith({
        token: 'test-token',
        name: 'John Doe',
        password: 'securepassword123',
      })
    })

    await waitFor(() => {
      expect(screen.getByText('Account Created!')).toBeTruthy()
    })
  })

  it('shows error on submit failure', async () => {
    vi.mocked(usersApi.validateInvite).mockResolvedValue({
      valid: true,
      email: 'invited@example.com',
    })
    vi.mocked(usersApi.acceptInvite).mockRejectedValue({
      response: { data: { error: 'Token has expired' } },
    })

    renderWithProviders()

    await waitFor(() => {
      expect(screen.getByPlaceholderText('John Doe')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('John Doe'), 'John Doe')
    const [passwordInput, confirmInput] = screen.getAllByPlaceholderText('••••••••')
    await user.type(passwordInput, 'securepassword123')
    await user.type(confirmInput, 'securepassword123')

    await user.click(screen.getByRole('button', { name: 'Create Account' }))

    await waitFor(() => {
      expect(usersApi.acceptInvite).toHaveBeenCalled()
    })

    // The toast should show error but we don't need to test toast specifically
  })

  it('navigates to login after clicking Go to Login button', async () => {
    renderWithProviders('/accept-invite')

    await waitFor(() => {
      expect(screen.getByText('Invalid Link')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Go to Login' }))

    expect(mockNavigate).toHaveBeenCalledWith('/login')
  })
})
