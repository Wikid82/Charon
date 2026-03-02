import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useJSONImport } from '../useJSONImport'
import * as api from '../../api/jsonImport'

vi.mock('../../api/jsonImport', () => ({
  uploadJSONExport: vi.fn(),
  commitJSONImport: vi.fn(),
  cancelJSONImport: vi.fn(),
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

describe('useJSONImport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('passes active session UUID to cancelJSONImport', async () => {
    const sessionId = 'json-session-123'
    vi.mocked(api.uploadJSONExport).mockResolvedValue({
      session: {
        id: sessionId,
        state: 'reviewing',
        source: 'json',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    })
    vi.mocked(api.cancelJSONImport).mockResolvedValue(undefined)

    const { result } = renderHook(() => useJSONImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe(sessionId)
    })

    await act(async () => {
      await result.current.cancel()
    })

    expect(api.cancelJSONImport).toHaveBeenCalledWith(sessionId)

    await waitFor(() => {
      expect(result.current.sessionId).toBeNull()
    })
  })
})
