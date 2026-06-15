import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, it, expect, beforeEach, vi } from 'vitest'

import RequireAuth from '../RequireAuth'
import { AuthContext, type AuthContextType, type User } from '../../context/AuthContextValue'

const TOKEN_KEY = 'charon_auth_token'

const testUser: User = { user_id: 1, role: 'admin', name: 'Test', email: 't@example.com' }

const buildAuthValue = (overrides: Partial<AuthContextType> = {}): AuthContextType => ({
  user: testUser,
  login: vi.fn(),
  logout: vi.fn(),
  changePassword: vi.fn(),
  isAuthenticated: true,
  isLoading: false,
  ...overrides,
})

const renderGuard = (authValue: AuthContextType) =>
  render(
    <AuthContext.Provider value={authValue}>
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route
            path="/"
            element={
              <RequireAuth>
                <div>protected content</div>
              </RequireAuth>
            }
          />
          <Route path="/login" element={<div>login page</div>} />
        </Routes>
      </MemoryRouter>
    </AuthContext.Provider>
  )

describe('<RequireAuth />', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('shows the loading overlay while auth state is resolving (no premature redirect)', () => {
    localStorage.setItem(TOKEN_KEY, 'token')
    renderGuard(buildAuthValue({ user: null, isAuthenticated: false, isLoading: true }))

    expect(screen.getByText(/authenticating/i)).toBeInTheDocument()
    expect(screen.queryByText('protected content')).not.toBeInTheDocument()
    expect(screen.queryByText('login page')).not.toBeInTheDocument()
  })

  it('redirects to /login when the user is not authenticated', () => {
    renderGuard(buildAuthValue({ user: null, isAuthenticated: false }))

    expect(screen.getByText('login page')).toBeInTheDocument()
    expect(screen.queryByText('protected content')).not.toBeInTheDocument()
  })

  it('redirects to /login when context is authenticated but the localStorage token is missing', () => {
    // Simulates session expiry where storage was cleared (issue #579 scenario)
    renderGuard(buildAuthValue())

    expect(screen.getByText('login page')).toBeInTheDocument()
    expect(screen.queryByText('protected content')).not.toBeInTheDocument()
  })

  it('renders protected children when authenticated with a stored token', () => {
    localStorage.setItem(TOKEN_KEY, 'token')
    renderGuard(buildAuthValue())

    expect(screen.getByText('protected content')).toBeInTheDocument()
    expect(screen.queryByText('login page')).not.toBeInTheDocument()
  })
})
