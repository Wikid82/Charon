import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../../api/client'
import * as setupApi from '../../api/setup'
import * as authHook from '../../hooks/useAuth'
import { toast } from '../../utils/toast'
import Login from '../Login'

import type { AuthContextType } from '../../context/AuthContextValue'

// Mock react-router useNavigate at module level
const mockNavigate = vi.fn()
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

vi.mock('../../api/setup')
vi.mock('../../hooks/useAuth')

describe('<Login />', () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const renderWithProviders = (ui: React.ReactNode) => (
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{ui}</MemoryRouter>
      </QueryClientProvider>
    )
  )

  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(authHook, 'useAuth').mockReturnValue({ login: vi.fn() } as unknown as AuthContextType)
  })

  it('navigates to /setup when setup is required', async () => {
    vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: true })
    renderWithProviders(<Login />)
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/setup')
    })
  })

  it('shows error toast when login fails', async () => {
    vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
    const postSpy = vi.spyOn(client, 'post').mockRejectedValueOnce({ response: { data: { error: 'Bad creds' } } })
    const toastSpy = vi.spyOn(toast, 'error')
    renderWithProviders(<Login />)
    // Fill and submit
    const email = screen.getByPlaceholderText(/admin@example.com/i)
    const pass = screen.getByPlaceholderText(/••••••••/i)
    fireEvent.change(email, { target: { value: 'a@b.com' } })
    fireEvent.change(pass, { target: { value: 'pw' } })
    fireEvent.click(screen.getByRole('button', { name: /Sign In/i }))
    // Wait for the promise chain
    await waitFor(() => expect(postSpy).toHaveBeenCalled())
    expect(toastSpy).toHaveBeenCalledWith('Bad creds')
  })

  it('uses returned token when cookie is unavailable', async () => {
    vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
    const postSpy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { token: 'bearer-token' } })
    const loginFn = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(authHook, 'useAuth').mockReturnValue({ login: loginFn } as unknown as AuthContextType)

    renderWithProviders(<Login />)
    const email = screen.getByPlaceholderText(/admin@example.com/i)
    const pass = screen.getByPlaceholderText(/••••••••/i)
    fireEvent.change(email, { target: { value: 'a@b.com' } })
    fireEvent.change(pass, { target: { value: 'pw' } })
    fireEvent.click(screen.getByRole('button', { name: /Sign In/i }))

    await waitFor(() => expect(postSpy).toHaveBeenCalled())
    expect(loginFn).toHaveBeenCalledWith('bearer-token')
  })

  it('has proper autocomplete attributes for password managers', async () => {
    vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
    renderWithProviders(<Login />)

    await waitFor(() => screen.getByPlaceholderText(/admin@example.com/i))

    const emailInput = screen.getByPlaceholderText(/admin@example.com/i)
    const passwordInput = screen.getByPlaceholderText(/••••••••/i)

    expect(emailInput).toHaveAttribute('autocomplete', 'email')
    expect(passwordInput).toHaveAttribute('autocomplete', 'current-password')
  })

  describe('rate limiting', () => {
    const submit = async (postSpy: ReturnType<typeof vi.spyOn>) => {
      fireEvent.change(screen.getByPlaceholderText(/admin@example.com/i), { target: { value: 'a@b.com' } })
      fireEvent.change(screen.getByPlaceholderText(/••••••••/i), { target: { value: 'pw' } })
      const calls = postSpy.mock.calls.length
      fireEvent.click(screen.getByRole('button', { name: /Sign In/i }))
      await waitFor(() => expect(postSpy.mock.calls.length).toBe(calls + 1))
    }

    const throttledError = (retryAfter: string) => ({
      response: { status: 429, headers: { 'retry-after': retryAfter }, data: { error: 'Too many requests.' } },
    })

    it('shows an inline notice with the wait and an administrator docs link instead of a toast', async () => {
      vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
      const postSpy = vi.spyOn(client, 'post').mockRejectedValueOnce(throttledError('42'))
      const toastSpy = vi.spyOn(toast, 'error')
      renderWithProviders(<Login />)
      await waitFor(() => screen.getByPlaceholderText(/admin@example.com/i))

      await submit(postSpy)

      const notice = await screen.findByTestId('login-rate-limit-notice')
      expect(notice).toHaveAttribute('role', 'alert')
      expect(notice).toHaveTextContent('Please wait 42 seconds')
      expect(notice).toHaveTextContent('Are you the administrator?')
      const link = within(notice).getByRole('link', { name: /how login protection works behind a proxy/i })
      expect(link).toHaveAttribute('href', 'https://wikid82.github.io/Charon/docs/configuration/trusted-proxies')
      expect(link).toHaveAttribute('target', '_blank')
      expect(link).toHaveAttribute('rel', 'noopener noreferrer')
      expect(toastSpy).not.toHaveBeenCalled()
    })

    it('clears the notice on the next submit', async () => {
      vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
      const postSpy = vi
        .spyOn(client, 'post')
        .mockRejectedValueOnce(throttledError('5'))
        .mockRejectedValueOnce({ response: { data: { error: 'Bad creds' } } })
      renderWithProviders(<Login />)
      await waitFor(() => screen.getByPlaceholderText(/admin@example.com/i))

      await submit(postSpy)
      await screen.findByTestId('login-rate-limit-notice')

      await submit(postSpy)
      await waitFor(() => expect(screen.queryByTestId('login-rate-limit-notice')).not.toBeInTheDocument())
    })

    it('keeps the toast for other errors', async () => {
      vi.spyOn(setupApi, 'getSetupStatus').mockResolvedValue({ setupRequired: false })
      const postSpy = vi
        .spyOn(client, 'post')
        .mockRejectedValueOnce({ response: { status: 401, data: { error: 'Bad creds' } } })
      const toastSpy = vi.spyOn(toast, 'error')
      renderWithProviders(<Login />)
      await waitFor(() => screen.getByPlaceholderText(/admin@example.com/i))

      await submit(postSpy)

      await waitFor(() => expect(toastSpy).toHaveBeenCalledWith('Bad creds'))
      expect(screen.queryByTestId('login-rate-limit-notice')).not.toBeInTheDocument()
    })
  })
})
