import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useNPMImport } from '../useNPMImport'
import * as api from '../../api/npmImport'

vi.mock('../../api/npmImport', () => ({
  uploadNPMExport: vi.fn(),
  commitNPMImport: vi.fn(),
  cancelNPMImport: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useNPMImport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('passes active session UUID to cancelNPMImport', async () => {
    const sessionId = 'npm-session-123'
    vi.mocked(api.uploadNPMExport).mockResolvedValue({
      session: {
        id: sessionId,
        state: 'reviewing',
        source: 'npm',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    })
    vi.mocked(api.cancelNPMImport).mockResolvedValue(undefined)

    const { result } = renderHook(() => useNPMImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe(sessionId)
    })

    await act(async () => {
      await result.current.cancel()
    })

    expect(api.cancelNPMImport).toHaveBeenCalledWith(sessionId)

    await waitFor(() => {
      expect(result.current.sessionId).toBeNull()
    })
  })
})
