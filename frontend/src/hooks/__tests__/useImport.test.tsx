import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useImport } from '../useImport'
import * as api from '../../api/import'

// Mock the API
vi.mock('../../api/import', () => ({
  uploadCaddyfile: vi.fn(),
  getImportPreview: vi.fn(),
  commitImport: vi.fn(),
  cancelImport: vi.fn(),
  getImportStatus: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useImport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('starts with no active session', async () => {
    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
      expect(result.current.session).toBeNull()
    })
    expect(result.current.error).toBeNull()
  })

  it('uploads content and creates session', async () => {
    const mockSession = {
      id: 'session-1',
      state: 'reviewing' as const,
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    const mockPreviewData = {
      hosts: [{ domain_names: 'test.com' }],
      conflicts: [],
      errors: [],
    }

    const mockResponse = {
      session: mockSession,
      preview: mockPreviewData,
    }

    vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)

    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('example.com { reverse_proxy localhost:8080 }')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    expect(api.uploadCaddyfile).toHaveBeenCalledWith('example.com { reverse_proxy localhost:8080 }')
    expect(result.current.loading).toBe(false)
  })

  it('handles upload errors', async () => {
    const mockError = new Error('Upload failed')
    vi.mocked(api.uploadCaddyfile).mockRejectedValue(mockError)

    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    let threw = false
    await act(async () => {
      try {
        await result.current.upload('invalid')
      } catch {
        threw = true
      }
    })
    expect(threw).toBe(true)

    await waitFor(() => {
      expect(result.current.error).toBe('Upload failed')
    })
  })

  it('commits import with resolutions', async () => {
    const mockSession = {
      id: 'session-2',
      state: 'reviewing' as const,
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    const mockResponse = {
      session: mockSession,
      preview: { hosts: [], conflicts: [], errors: [] },
    }

    let isCommitted = false
    vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
    vi.mocked(api.getImportStatus).mockImplementation(async () => {
      if (isCommitted) return { has_pending: false }
      return { has_pending: true, session: mockSession }
    })
    vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)
    vi.mocked(api.commitImport).mockImplementation(async () => {
      isCommitted = true
    })

    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('test')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    await act(async () => {
      await result.current.commit({ 'test.com': 'skip' })
    })

    expect(api.commitImport).toHaveBeenCalledWith({ 'test.com': 'skip' })

    await waitFor(() => {
      expect(result.current.session).toBeNull()
    })
  })

  it('cancels active import session', async () => {
    const mockSession = {
      id: 'session-3',
      state: 'reviewing' as const,
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    const mockResponse = {
      session: mockSession,
      preview: { hosts: [], conflicts: [], errors: [] },
    }

    let isCancelled = false
    vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
    vi.mocked(api.getImportStatus).mockImplementation(async () => {
      if (isCancelled) return { has_pending: false }
      return { has_pending: true, session: mockSession }
    })
    vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)
    vi.mocked(api.cancelImport).mockImplementation(async () => {
      isCancelled = true
    })

    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('test')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    await act(async () => {
      await result.current.cancel()
    })

    expect(api.cancelImport).toHaveBeenCalled()
    await waitFor(() => {
      expect(result.current.session).toBeNull()
    })
  })

  it('handles commit errors', async () => {
    const mockSession = {
      id: 'session-4',
      state: 'reviewing' as const,
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    const mockResponse = {
      session: mockSession,
      preview: { hosts: [], conflicts: [], errors: [] },
    }

    vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
    vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)

    const mockError = new Error('Commit failed')
    vi.mocked(api.commitImport).mockRejectedValue(mockError)

    const { result } = renderHook(() => useImport(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.upload('test')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    let threw = false
    await act(async () => {
      try {
        await result.current.commit({})
      } catch {
        threw = true
      }
    })
    expect(threw).toBe(true)

    await waitFor(() => {
      expect(result.current.error).toBe('Commit failed')
    })
  })
})
