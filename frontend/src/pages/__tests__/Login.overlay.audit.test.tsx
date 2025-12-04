import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import Login from '../Login'
import * as authHook from '../../hooks/useAuth'
import client from '../../api/client'

// Mock modules
vi.mock('../../api/client')
vi.mock('../../hooks/useAuth')
vi.mock('../../api/setup', () => ({
  getSetupStatus: vi.fn(() => Promise.resolve({ setupRequired: false })),
}))

const mockLogin = vi.fn()
vi.mocked(authHook.useAuth).mockReturnValue({
  user: null,
  login: mockLogin,
  logout: vi.fn(),
  loading: false,
} as unknown as ReturnType<typeof authHook.useAuth>)

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>
  )
}

describe('Login - Coin Overlay Security Audit', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows coin-themed overlay during login', async () => {
    vi.mocked(client.post).mockImplementation(
      () => new Promise(resolve => setTimeout(() => resolve({ data: {} }), 100))
    )

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, 'admin@example.com')
    await userEvent.type(passwordInput, 'password123')
    await userEvent.click(submitButton)

    // Coin-themed overlay should appear
    expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()
    expect(screen.getByText('Your obol grants passage')).toBeInTheDocument()

    // Verify coin theme (gold/amber)
    const overlay = screen.getByText('Paying the ferryman...').closest('div')
    expect(overlay).toHaveClass('bg-amber-950/90')

    // Wait for completion
    await waitFor(() => {
      expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
    }, { timeout: 200 })
  })

  it('ATTACK: rapid fire login attempts are blocked by overlay', async () => {
    let resolveCount = 0
    vi.mocked(client.post).mockImplementation(
      () => new Promise(resolve => {
        setTimeout(() => {
          resolveCount++
          resolve({ data: {} })
        }, 200)
      })
    )

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, 'admin@example.com')
    await userEvent.type(passwordInput, 'password123')

    // Click multiple times rapidly
    await userEvent.click(submitButton)
    await userEvent.click(submitButton)
    await userEvent.click(submitButton)

    // Overlay should block subsequent clicks (form is disabled)
    expect(emailInput).toBeDisabled()
    expect(passwordInput).toBeDisabled()
    expect(submitButton).toBeDisabled()

    await waitFor(() => {
      expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
    }, { timeout: 300 })

    // Should only execute once
    expect(resolveCount).toBe(1)
  })

  it('clears overlay on login error', async () => {
    vi.mocked(client.post).mockRejectedValue({
      response: { data: { error: 'Invalid credentials' } }
    })

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, 'wrong@example.com')
    await userEvent.type(passwordInput, 'wrong')
    await userEvent.click(submitButton)

    // Overlay appears
    expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()

    // Overlay clears after error
    await waitFor(() => {
      expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
    }, { timeout: 200 })

    // Form should be re-enabled
    expect(emailInput).not.toBeDisabled()
    expect(passwordInput).not.toBeDisabled()
  })

  it('ATTACK: XSS in login credentials does not break overlay', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: {} })

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, '<script>alert(1)</script>@example.com')
    await userEvent.type(passwordInput, '<img src=x onerror=alert(1)>')
    await userEvent.click(submitButton)

    // Overlay should still work
    expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
    }, { timeout: 200 })
  })

  it('ATTACK: network timeout does not leave overlay stuck', async () => {
    vi.mocked(client.post).mockImplementation(
      () => new Promise((_, reject) => {
        setTimeout(() => reject(new Error('Network timeout')), 100)
      })
    )

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, 'admin@example.com')
    await userEvent.type(passwordInput, 'password123')
    await userEvent.click(submitButton)

    expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()

    // Overlay should clear after error
    await waitFor(() => {
      expect(screen.queryByText('Paying the ferryman...')).not.toBeInTheDocument()
    }, { timeout: 200 })
  })

  it('overlay has correct z-index hierarchy', () => {
    vi.mocked(client.post).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    userEvent.type(emailInput, 'admin@example.com')
    userEvent.type(passwordInput, 'password123')
    userEvent.click(submitButton)

    // Overlay should be z-50
    const overlay = document.querySelector('.z-50')
    expect(overlay).toBeInTheDocument()
  })

  it('overlay renders CharonCoinLoader component', async () => {
    vi.mocked(client.post).mockImplementation(
      () => new Promise(resolve => setTimeout(() => resolve({ data: {} }), 100))
    )

    renderWithProviders(<Login />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /sign in/i })

    await userEvent.type(emailInput, 'admin@example.com')
    await userEvent.type(passwordInput, 'password123')
    await userEvent.click(submitButton)

    // CharonCoinLoader has aria-label="Authenticating"
    expect(screen.getByLabelText('Authenticating')).toBeInTheDocument()
  })
})
