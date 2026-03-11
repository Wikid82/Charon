import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act, waitFor } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/jsonImport'
import { useJSONImport } from '../useJSONImport'

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

  it('sets preview and sessionId after successful upload', async () => {
    const uploadResponse = {
      session: {
        id: 'json-session-upload',
        state: 'reviewing',
        source: 'json',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    }

    vi.mocked(api.uploadJSONExport).mockResolvedValue(uploadResponse)

    const { result } = renderHook(() => useJSONImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe('json-session-upload')
      expect(result.current.preview).toEqual(uploadResponse)
    })
  })

  it('commits active session and clears preview/session state', async () => {
    const uploadResponse = {
      session: {
        id: 'json-session-commit',
        state: 'reviewing',
        source: 'json',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    }
    const commitResponse = {
      created: 1,
      updated: 0,
      skipped: 0,
      errors: [],
    }

    vi.mocked(api.uploadJSONExport).mockResolvedValue(uploadResponse)
    vi.mocked(api.commitJSONImport).mockResolvedValue(commitResponse)

    const { result } = renderHook(() => useJSONImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe('json-session-commit')
    })

    await act(async () => {
      await result.current.commit({ 'json.example.com': 'replace' }, { 'json.example.com': 'JSON Example' })
    })

    expect(api.commitJSONImport).toHaveBeenCalledWith(
      'json-session-commit',
      { 'json.example.com': 'replace' },
      { 'json.example.com': 'JSON Example' }
    )

    await waitFor(() => {
      expect(result.current.sessionId).toBeNull()
      expect(result.current.preview).toBeNull()
      expect(result.current.commitResult).toEqual(commitResponse)
    })
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
    vi.mocked(api.cancelJSONImport).mockResolvedValue()

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

  it('returns No active session and skips cancel API call when session is missing', async () => {
    const { result } = renderHook(() => useJSONImport(), { wrapper: createWrapper() })

    await expect(result.current.cancel()).rejects.toThrow('No active session')
    expect(api.cancelJSONImport).not.toHaveBeenCalled()
  })

  it('exposes commit error and preserves session on commit failure', async () => {
    const uploadResponse = {
      session: {
        id: 'json-session-error',
        state: 'reviewing',
        source: 'json',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    }
    const commitError = new Error('404 Not Found')

    vi.mocked(api.uploadJSONExport).mockResolvedValue(uploadResponse)
    vi.mocked(api.commitJSONImport).mockRejectedValue(commitError)

    const { result } = renderHook(() => useJSONImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await expect(result.current.commit({}, {})).rejects.toBe(commitError)

    await waitFor(() => {
      expect(result.current.commitError).toBe(commitError)
      expect(result.current.sessionId).toBe('json-session-error')
      expect(result.current.preview).not.toBeNull()
    })
  })
})
