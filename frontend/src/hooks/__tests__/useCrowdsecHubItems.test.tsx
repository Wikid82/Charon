import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/crowdsec'
import { useRestoreHubItems } from '../useCrowdsecHubItems'

vi.mock('../../api/crowdsec')

const result = { installed: [], already_installed: [], failed: [], remaining: 0, skipped_invalid: 0, reloaded: false }

const setup = () => {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return { invalidate, ...renderHook(() => useRestoreHubItems(), { wrapper }) }
}

describe('useRestoreHubItems', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls the API and refreshes the file list on success', async () => {
    vi.mocked(api.restoreHubItems).mockResolvedValue(result)
    const { result: hook, invalidate } = setup()
    const items = [{ type: 'collections', name: 'crowdsecurity/caddy' }]

    await act(async () => {
      await hook.current.mutateAsync(items)
    })

    expect(api.restoreHubItems).toHaveBeenCalledWith(items)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['crowdsec-files'] })
  })

  it('exposes the error without invalidating', async () => {
    vi.mocked(api.restoreHubItems).mockRejectedValue(new Error('boom'))
    const { result: hook, invalidate } = setup()

    act(() => hook.current.mutate([]))

    await waitFor(() => expect(hook.current.isError).toBe(true))
    expect(invalidate).not.toHaveBeenCalled()
  })
})
