import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

// Initialises the global i18next instance the interceptor translates with
import '../../i18n'
import client from '../../api/client'
import { AuthProvider } from '../../context/AuthContext'
import { useAuth } from '../../hooks/useAuth'
import { isMaintenanceActive, leaveMaintenance } from '../../utils/maintenanceMode'
import MaintenanceGate from '../MaintenanceGate'

// Real client + real interceptor + real AuthProvider; only the transport is stubbed.
const originalAdapter = client.defaults.adapter

function Probe() {
  const { user, isLoading } = useAuth()
  if (isLoading) return <div>loading</div>
  return <div>{user ? `app for ${user.name}` : 'logged out'}</div>
}

describe('maintenance 503 end to end (client + AuthProvider + gate)', () => {
  let maintenanceActive = true
  const calls: string[] = []

  beforeEach(() => {
    maintenanceActive = true
    calls.length = 0
    localStorage.setItem('charon_auth_token', 'tok')
    client.defaults.adapter = (async (config: InternalAxiosRequestConfig) => {
      const url = config.url ?? ''
      calls.push(url)
      const respond = (status: number, data: unknown) => ({
        status,
        statusText: '',
        headers: {},
        config,
        data,
      })
      if (url === '/maintenance/status') {
        return respond(200, { active: maintenanceActive, phase: maintenanceActive ? 'converting' : 'done', elapsed_seconds: 1 })
      }
      if (url === '/auth/me') {
        if (maintenanceActive) {
          const response = respond(503, { error: 'Database optimization in progress', maintenance: true })
          throw new AxiosError('503', 'ERR_BAD_RESPONSE', config, null, response)
        }
        return respond(200, { user_id: 1, role: 'admin', name: 'Ada', email: 'a@b.c' })
      }
      return respond(200, {})
    }) as AxiosAdapter
  })

  afterEach(() => {
    client.defaults.adapter = originalAdapter
    leaveMaintenance()
    localStorage.clear()
  })

  it('shows the view, keeps the session token, checks the session once, then returns to the app', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <MaintenanceGate>
          <AuthProvider>
            <Probe />
          </AuthProvider>
        </MaintenanceGate>
      </QueryClientProvider>
    )

    expect(await screen.findByRole('heading', { name: /optimizing the database/i })).toBeInTheDocument()
    expect(isMaintenanceActive()).toBe(true)
    // Not a logout: the token survives and no login/logout call was made.
    expect(localStorage.getItem('charon_auth_token')).toBe('tok')
    expect(calls.filter((u) => u === '/auth/me')).toHaveLength(1)
    expect(calls).not.toContain('/auth/logout')

    // Maintenance ends: the next status poll flips the gate and the session check re-runs.
    maintenanceActive = false
    expect(await screen.findByText('app for Ada', {}, { timeout: 9000 })).toBeInTheDocument()
    expect(isMaintenanceActive()).toBe(false)
    expect(calls.filter((u) => u === '/auth/me')).toHaveLength(2)
  }, 15_000)
})
