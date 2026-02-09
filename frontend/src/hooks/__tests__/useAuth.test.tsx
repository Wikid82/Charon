import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { AuthContext } from '../../context/AuthContextValue'
import { useAuth } from '../useAuth'

const TestComponent = () => {
  const auth = useAuth()
  return <div>{auth.isAuthenticated ? 'auth' : 'no-auth'}</div>
}

describe('useAuth hook', () => {
  it('throws if used outside provider', () => {
    const renderOutside = () => render(<TestComponent />)
    expect(renderOutside).toThrowError('useAuth must be used within an AuthProvider')
  })

  it('returns context inside provider', () => {
    const fakeCtx = { user: { user_id: 1, role: 'admin', name: 'Test', email: 't@example.com' }, login: async () => {}, logout: () => {}, changePassword: async () => {}, isAuthenticated: true, isLoading: false }
    render(
      <AuthContext.Provider value={fakeCtx}>
        <TestComponent />
      </AuthContext.Provider>
    )
    expect(screen.getByText('auth')).toBeTruthy()
  })
})
