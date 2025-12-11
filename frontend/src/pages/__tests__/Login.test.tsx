import { describe, it, expect, vi, beforeEach } from 'vitest'
// Mock react-router-dom useNavigate at module level
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Login from '../Login'
import * as setupApi from '../../api/setup'
import client from '../../api/client'
import * as authHook from '../../hooks/useAuth'
import type { AuthContextType } from '../../context/AuthContextValue'
import { toast } from '../../utils/toast'
import { MemoryRouter } from 'react-router-dom'

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
})
