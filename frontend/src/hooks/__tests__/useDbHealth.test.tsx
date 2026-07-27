import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/dbHealth'
import { useDbHealth, DB_HEALTH_QUERY_KEY } from '../useDbHealth'

vi.mock('../../api/dbHealth')

const createWrapper = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useDbHealth', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches and returns a healthy status', async () => {
    vi.mocked(api.getDbHealth).mockResolvedValue({
      status: 'healthy',
      integrity_ok: true,
      integrity_result: 'ok',
      wal_mode: true,
      journal_mode: 'wal',
      last_backup: '2026-07-16T03:00:00Z',
      checked_at: '2026-07-16T10:00:00Z',
    })

    const { result } = renderHook(() => useDbHealth(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data?.status).toBe('healthy')
  })

  it('fetches and returns a corrupted status', async () => {
    vi.mocked(api.getDbHealth).mockResolvedValue({
      status: 'corrupted',
      integrity_ok: false,
      integrity_result: 'database disk image is malformed',
      wal_mode: true,
      journal_mode: 'wal',
      last_backup: null,
      checked_at: '2026-07-16T10:00:00Z',
    })

    const { result } = renderHook(() => useDbHealth(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.data?.status).toBe('corrupted'))
  })

  it('uses the db-health query key', () => {
    expect(DB_HEALTH_QUERY_KEY).toEqual(['db-health'])
  })
})
