import { QueryClient, QueryClientProvider, type Query } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import { enterMaintenance, isMaintenanceActive, leaveMaintenance } from '../../utils/maintenanceMode'
import MaintenanceGate from '../MaintenanceGate'

vi.mock('../../api/databaseMaintenance')

const renderGate = (queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })) =>
  render(
    <QueryClientProvider client={queryClient}>
      <MaintenanceGate>
        <div>the app</div>
      </MaintenanceGate>
    </QueryClientProvider>
  )

describe('MaintenanceGate', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => leaveMaintenance())

  it('renders the app and does not poll while maintenance is off', () => {
    renderGate()
    expect(screen.getByText('the app')).toBeInTheDocument()
    expect(api.getMaintenanceStatus).not.toHaveBeenCalled()
  })

  it('swaps the app for the Optimizing view and polls the status route', async () => {
    vi.mocked(api.getMaintenanceStatus).mockResolvedValue({ active: true, phase: 'converting', elapsed_seconds: 42 })
    renderGate()
    act(() => enterMaintenance())

    expect(screen.queryByText('the app')).toBeNull()
    expect(screen.getByRole('heading', { name: /optimizing the database/i })).toBeInTheDocument()
    expect(screen.getByText(/your proxies are still running/i)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText(/running for 42 seconds/i)).toBeInTheDocument())
    expect(api.getMaintenanceStatus).toHaveBeenCalled()
    expect(isMaintenanceActive()).toBe(true)
  })

  it('returns to the app (remounted) once the status reports active:false', async () => {
    vi.mocked(api.getMaintenanceStatus).mockResolvedValue({ active: false, phase: 'done', elapsed_seconds: 90 })
    renderGate()
    act(() => enterMaintenance())

    expect(await screen.findByText('the app')).toBeInTheDocument()
    expect(isMaintenanceActive()).toBe(false)
    expect(screen.queryByRole('heading', { name: /optimizing the database/i })).toBeNull()
  })

  it('keeps showing the view when a poll fails, without leaving maintenance', async () => {
    vi.mocked(api.getMaintenanceStatus).mockRejectedValue(new Error('network'))
    renderGate()
    act(() => enterMaintenance())

    await waitFor(() => expect(api.getMaintenanceStatus).toHaveBeenCalled())
    expect(screen.getByRole('heading', { name: /optimizing the database/i })).toBeInTheDocument()
    expect(isMaintenanceActive()).toBe(true)
  })

  it('cancels in-flight app queries on entry so nothing retries against the 503', async () => {
    vi.mocked(api.getMaintenanceStatus).mockResolvedValue({ active: true, phase: 'converting', elapsed_seconds: 1 })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const spy = vi.spyOn(queryClient, 'cancelQueries')
    renderGate(queryClient)
    act(() => enterMaintenance())

    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    const matches = (queryKey: string[]) =>
      spy.mock.calls[0][0]?.predicate?.({ queryKey } as unknown as Query)
    expect(matches(['proxy-hosts'])).toBe(true)
    expect(matches(['maintenance-status'])).toBe(false)
  })
})
