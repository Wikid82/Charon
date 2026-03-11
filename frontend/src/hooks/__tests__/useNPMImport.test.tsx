import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act, waitFor } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/npmImport'
import { useNPMImport } from '../useNPMImport'

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

  it('sets preview and sessionId after successful upload', async () => {
    const uploadResponse = {
      session: {
        id: 'npm-session-upload',
        state: 'reviewing',
        source: 'npm',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    }

    vi.mocked(api.uploadNPMExport).mockResolvedValue(uploadResponse)

    const { result } = renderHook(() => useNPMImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe('npm-session-upload')
      expect(result.current.preview).toEqual(uploadResponse)
    })
  })

  it('commits active session and clears preview/session state', async () => {
    const uploadResponse = {
      session: {
        id: 'npm-session-commit',
        state: 'reviewing',
        source: 'npm',
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

    vi.mocked(api.uploadNPMExport).mockResolvedValue(uploadResponse)
    vi.mocked(api.commitNPMImport).mockResolvedValue(commitResponse)

    const { result } = renderHook(() => useNPMImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await waitFor(() => {
      expect(result.current.sessionId).toBe('npm-session-commit')
    })

    await act(async () => {
      await result.current.commit({ 'npm.example.com': 'replace' }, { 'npm.example.com': 'NPM Example' })
    })

    expect(api.commitNPMImport).toHaveBeenCalledWith(
      'npm-session-commit',
      { 'npm.example.com': 'replace' },
      { 'npm.example.com': 'NPM Example' }
    )

    await waitFor(() => {
      expect(result.current.sessionId).toBeNull()
      expect(result.current.preview).toBeNull()
      expect(result.current.commitResult).toEqual(commitResponse)
    })
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
    vi.mocked(api.cancelNPMImport).mockResolvedValue()

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

  it('returns No active session and skips cancel API call when session is missing', async () => {
    const { result } = renderHook(() => useNPMImport(), { wrapper: createWrapper() })

    await expect(result.current.cancel()).rejects.toThrow('No active session')
    expect(api.cancelNPMImport).not.toHaveBeenCalled()
  })

  it('exposes commit error and preserves session on commit failure', async () => {
    const uploadResponse = {
      session: {
        id: 'npm-session-error',
        state: 'reviewing',
        source: 'npm',
      },
      preview: {
        hosts: [],
        conflicts: [],
        errors: [],
      },
      conflict_details: {},
    }
    const commitError = new Error('404 Not Found')

    vi.mocked(api.uploadNPMExport).mockResolvedValue(uploadResponse)
    vi.mocked(api.commitNPMImport).mockRejectedValue(commitError)

    const { result } = renderHook(() => useNPMImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('{"proxy_hosts":[]}')
    })

    await expect(result.current.commit({}, {})).rejects.toBe(commitError)

    await waitFor(() => {
      expect(result.current.commitError).toBe(commitError)
      expect(result.current.sessionId).toBe('npm-session-error')
      expect(result.current.preview).not.toBeNull()
    })
  })
})
