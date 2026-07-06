import { act, render, screen, waitFor } from '@testing-library/react'
import { toast } from 'react-hot-toast'
import { describe, it, expect, beforeEach, vi } from 'vitest'

import client, { setAuthToken, setAuthErrorHandler } from '../../api/client'
import { AuthProvider } from '../AuthContext'
import { useAuth } from '../../hooks/useAuth'

const TOKEN_KEY = 'charon_auth_token'

vi.mock('react-hot-toast', () => ({
  toast: {
    error: vi.fn(),
    dismiss: vi.fn(),
  },
}))

vi.mock('../../api/client', () => ({
  default: {
    post: vi.fn().mockResolvedValue({}),
    get: vi.fn(),
  },
  setAuthToken: vi.fn(),
  setAuthErrorHandler: vi.fn(),
}))

const mockClient = vi.mocked(client)
const mockSetAuthToken = vi.mocked(setAuthToken)
const mockSetAuthErrorHandler = vi.mocked(setAuthErrorHandler)
const mockToast = vi.mocked(toast)

const AuthStateProbe = () => {
  const { user, isAuthenticated, isLoading } = useAuth()
  return (
    <div>
      <span data-testid="loading">{String(isLoading)}</span>
      <span data-testid="authenticated">{String(isAuthenticated)}</span>
      <span data-testid="user">{user?.email ?? 'none'}</span>
    </div>
  )
}

const renderProvider = () =>
  render(
    <AuthProvider>
      <AuthStateProbe />
    </AuthProvider>
  )

const sessionUser = { user_id: 1, role: 'admin', name: 'Test', email: 't@example.com' }

describe('<AuthProvider /> session validation on mount (page reload)', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('clears auth state without calling /auth/me when no token is stored', async () => {
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(mockClient.get).not.toHaveBeenCalled()
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(screen.getByTestId('user').textContent).toBe('none')
    expect(mockSetAuthToken).toHaveBeenCalledWith(null)
  })

  it('clears auth state when the stored token fails /auth/me validation (expired session)', async () => {
    localStorage.setItem(TOKEN_KEY, 'expired-token')
    mockClient.get.mockRejectedValue(Object.assign(new Error('Request failed with status code 401'), { response: { status: 401 } }))

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(mockClient.get).toHaveBeenCalledWith('/auth/me')
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(mockSetAuthToken).toHaveBeenLastCalledWith(null)
  })

  it('restores the session when the stored token passes /auth/me validation', async () => {
    localStorage.setItem(TOKEN_KEY, 'valid-token')
    mockClient.get.mockResolvedValue({ data: sessionUser, status: 200 })

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(screen.getByTestId('authenticated').textContent).toBe('true')
    expect(screen.getByTestId('user').textContent).toBe('t@example.com')
    expect(mockSetAuthToken).toHaveBeenCalledWith('valid-token')
  })

  it('registers an auth-error handler that clears the session, shows a toast, and unregisters on unmount', async () => {
    localStorage.setItem(TOKEN_KEY, 'valid-token')
    mockClient.get.mockResolvedValue({ data: sessionUser, status: 200 })

    const { unmount } = renderProvider()
    await waitFor(() => expect(screen.getByTestId('authenticated').textContent).toBe('true'))

    const registered = mockSetAuthErrorHandler.mock.calls.at(-1)?.[0]
    expect(registered).toBeTypeOf('function')

    act(() => registered?.())

    await waitFor(() => expect(screen.getByTestId('authenticated').textContent).toBe('false'))
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(mockSetAuthToken).toHaveBeenLastCalledWith(null)
    expect(mockToast.error).toHaveBeenCalledWith(
      'Session expired. Please log in again.',
      expect.objectContaining({ id: 'auth-session-expired' })
    )

    unmount()
    expect(mockSetAuthErrorHandler).toHaveBeenLastCalledWith(null)
  })

  it('dismisses the session-expired toast when login succeeds', async () => {
    mockClient.get.mockResolvedValue({ data: sessionUser, status: 200 })

    let loginFn: ((token?: string) => Promise<void>) | undefined
    const LoginProbe = () => {
      const { login } = useAuth()
      loginFn = login
      return null
    }

    render(
      <AuthProvider>
        <LoginProbe />
        <AuthStateProbe />
      </AuthProvider>
    )

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    await act(async () => {
      await loginFn?.('new-token')
    })

    expect(mockToast.dismiss).toHaveBeenCalledWith('auth-session-expired')
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
  })
})
