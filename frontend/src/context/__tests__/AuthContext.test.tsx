import { act, render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

import { AuthProvider } from '../AuthContext'
import { setAuthToken, setAuthErrorHandler } from '../../api/client'
import { useAuth } from '../../hooks/useAuth'

const TOKEN_KEY = 'charon_auth_token'

vi.mock('../../api/client', () => ({
  default: { post: vi.fn().mockResolvedValue({}) },
  setAuthToken: vi.fn(),
  setAuthErrorHandler: vi.fn(),
}))

const mockSetAuthToken = vi.mocked(setAuthToken)
const mockSetAuthErrorHandler = vi.mocked(setAuthErrorHandler)

const fetchMock = vi.fn()

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
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('clears auth state without calling /auth/me when no token is stored', async () => {
    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(fetchMock).not.toHaveBeenCalled()
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(screen.getByTestId('user').textContent).toBe('none')
    expect(mockSetAuthToken).toHaveBeenCalledWith(null)
  })

  it('clears auth state when the stored token fails /auth/me validation (expired session)', async () => {
    localStorage.setItem(TOKEN_KEY, 'expired-token')
    fetchMock.mockResolvedValue({ ok: false, status: 401 })

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/me',
      expect.objectContaining({ credentials: 'include' })
    )
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(mockSetAuthToken).toHaveBeenLastCalledWith(null)
  })

  it('restores the session when the stored token passes /auth/me validation', async () => {
    localStorage.setItem(TOKEN_KEY, 'valid-token')
    fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(sessionUser) })

    renderProvider()

    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'))

    expect(screen.getByTestId('authenticated').textContent).toBe('true')
    expect(screen.getByTestId('user').textContent).toBe('t@example.com')
    expect(mockSetAuthToken).toHaveBeenCalledWith('valid-token')
  })

  it('registers an auth-error handler that clears the session, and unregisters it on unmount', async () => {
    localStorage.setItem(TOKEN_KEY, 'valid-token')
    fetchMock.mockResolvedValue({ ok: true, json: () => Promise.resolve(sessionUser) })
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    const { unmount } = renderProvider()
    await waitFor(() => expect(screen.getByTestId('authenticated').textContent).toBe('true'))

    const registered = mockSetAuthErrorHandler.mock.calls.at(-1)?.[0]
    expect(registered).toBeTypeOf('function')

    act(() => registered?.())

    await waitFor(() => expect(screen.getByTestId('authenticated').textContent).toBe('false'))
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
    expect(mockSetAuthToken).toHaveBeenLastCalledWith(null)

    unmount()
    expect(mockSetAuthErrorHandler).toHaveBeenLastCalledWith(null)

    warnSpy.mockRestore()
  })
})
