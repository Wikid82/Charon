import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as api from '../../api/import'
import { useImport, QUERY_KEY } from '../useImport'

// Mock the API
vi.mock('../../api/import', () => ({
  uploadCaddyfile: vi.fn(),
  getImportPreview: vi.fn(),
  commitImport: vi.fn(),
  cancelImport: vi.fn(),
  getImportStatus: vi.fn(),
}))

// Create wrapper with query client that we can inspect
const createWrapper = (queryClient?: QueryClient) => {
  const client = queryClient ?? new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return {
    queryClient: client,
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
  }
}

// Legacy wrapper for backward compatibility
const createSimpleWrapper = () => {
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

  describe('basic operations', () => {
    it('starts with no active session', async () => {
      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

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

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

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

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

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
        return { created: 0, updated: 0, skipped: 0, errors: [] }
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.commit({ 'test.com': 'skip' }, { 'test.com': 'Test' })
      })

      expect(api.commitImport).toHaveBeenCalledWith('session-2', { 'test.com': 'skip' }, { 'test.com': 'Test' })

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

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.cancel()
      })

      expect(api.cancelImport).toHaveBeenCalledWith('session-3')
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

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      let threw = false
      await act(async () => {
        try {
          await result.current.commit({}, {})
        } catch {
          threw = true
        }
      })
      expect(threw).toBe(true)

      await waitFor(() => {
        expect(result.current.error).toBe('Commit failed')
      })
    })

    it('captures and exposes commit result on success', async () => {
      const mockSession = {
        id: 'session-5',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      const mockCommitResult = {
        created: 3,
        updated: 1,
        skipped: 2,
        errors: [],
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
        return mockCommitResult
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(result.current.commitResult).toEqual(mockCommitResult)
      expect(result.current.commitSuccess).toBe(true)
    })

    it('clears commit result when clearCommitResult is called', async () => {
      const mockSession = {
        id: 'session-6',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      const mockCommitResult = {
        created: 2,
        updated: 0,
        skipped: 0,
        errors: [],
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
        return mockCommitResult
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(result.current.commitResult).toEqual(mockCommitResult)

      act(() => {
        result.current.clearCommitResult()
      })

      expect(result.current.commitResult).toBeNull()
      expect(result.current.commitSuccess).toBe(false)
    })
  })

  describe('status query polling', () => {
    it('should not poll when session is null', async () => {
      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await waitFor(() => {
        expect(result.current.loading).toBe(false)
      })

      // Initial call should happen
      expect(api.getImportStatus).toHaveBeenCalledTimes(1)

      // Wait and verify no additional polls (since there's no session)
      await new Promise(resolve => setTimeout(resolve, 100))
      expect(api.getImportStatus).toHaveBeenCalledTimes(1)
    })

    it('should poll status only when session state is reviewing', async () => {
      const mockSession = {
        id: 'session-poll',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      vi.mocked(api.getImportPreview).mockResolvedValue({
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      })

      const { queryClient, wrapper } = createWrapper(new QueryClient({
        defaultOptions: {
          queries: {
            retry: false,
            // Shorter refetch interval for testing
          },
        },
      }))

      renderHook(() => useImport(), { wrapper })

      await waitFor(() => {
        expect(api.getImportStatus).toHaveBeenCalled()
      })

      // Verify the query is configured (we test the return value logic)
      const queryState = queryClient.getQueryState(QUERY_KEY)
      expect(queryState?.data).toEqual({ has_pending: true, session: mockSession })
    })

    it('should not poll when session state is transient', async () => {
      const mockSession = {
        id: 'session-transient',
        state: 'transient' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      vi.mocked(api.getImportPreview).mockResolvedValue({
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      // Initial call + preview query should happen
      expect(api.getImportStatus).toHaveBeenCalledTimes(1)
    })
  })

  describe('preview query behavior', () => {
    it('should enable preview query when session has uuid', async () => {
      const mockSession = {
        id: 'session-preview',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockPreview = {
        session: mockSession,
        preview: { hosts: [{ domain_names: 'example.com' }], conflicts: [], errors: [] },
      }

      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      vi.mocked(api.getImportPreview).mockResolvedValue(mockPreview)

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      // Preview query should be called when session is active
      await waitFor(() => {
        expect(api.getImportPreview).toHaveBeenCalled()
      })
    })

    it('should disable preview query after commit succeeds', async () => {
      const mockSession = {
        id: 'session-disable-preview',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      let commitCount = 0
      let previewCallsAfterCommit = 0

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      vi.mocked(api.getImportStatus).mockImplementation(async () => {
        return { has_pending: commitCount === 0, session: commitCount === 0 ? mockSession : undefined }
      })
      vi.mocked(api.getImportPreview).mockImplementation(async () => {
        if (commitCount > 0) previewCallsAfterCommit++
        return mockResponse
      })
      vi.mocked(api.commitImport).mockImplementation(async () => {
        commitCount++
        return { created: 1, updated: 0, skipped: 0, errors: [] }
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(result.current.commitSuccess).toBe(true)

      // Preview query should not be called again after commit
      await new Promise(resolve => setTimeout(resolve, 100))
      expect(previewCallsAfterCommit).toBe(0)
    })
  })

  describe('upload mutation behavior', () => {
    it('should store preview from upload response immediately', async () => {
      const mockSession = {
        id: 'session-immediate',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockPreviewData = {
        hosts: [{ domain_names: 'immediate-test.com' }],
        conflicts: [],
        errors: [],
      }

      const mockResponse = {
        session: mockSession,
        preview: mockPreviewData,
      }

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      // Delay status query to simulate race condition
      vi.mocked(api.getImportStatus).mockImplementation(async () => {
        await new Promise(resolve => setTimeout(resolve, 500))
        return { has_pending: true, session: mockSession }
      })
      vi.mocked(api.getImportPreview).mockImplementation(async () => {
        await new Promise(resolve => setTimeout(resolve, 500))
        return mockResponse
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      // Preview should be available immediately from upload response, not waiting for queries
      expect(result.current.session).toEqual(mockSession)
      expect(result.current.preview).toEqual(mockResponse)
    })

    it('should invalidate status query on upload success', async () => {
      const mockSession = {
        id: 'session-invalidate',
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

      const { queryClient, wrapper } = createWrapper()
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const { result } = renderHook(() => useImport(), { wrapper })

      await act(async () => {
        await result.current.upload('test content')
      })

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: QUERY_KEY })
    })
  })

  describe('commit mutation behavior', () => {
    it('should invalidate proxy-hosts cache on commit success', async () => {
      const mockSession = {
        id: 'session-proxy-cache',
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
      vi.mocked(api.commitImport).mockResolvedValue({ created: 1, updated: 0, skipped: 0, errors: [] })

      const { queryClient, wrapper } = createWrapper()
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const { result } = renderHook(() => useImport(), { wrapper })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['proxy-hosts'] })
    })

    it('should set commitSucceeded flag on commit success', async () => {
      const mockSession = {
        id: 'session-flag',
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
      vi.mocked(api.commitImport).mockResolvedValue({ created: 1, updated: 0, skipped: 0, errors: [] })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      expect(result.current.commitSuccess).toBe(false)

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(result.current.commitSuccess).toBe(true)
    })

    it('should store commit result on success', async () => {
      const mockSession = {
        id: 'session-result',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      const expectedResult = { created: 5, updated: 2, skipped: 1, errors: [] }

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)
      vi.mocked(api.commitImport).mockResolvedValue(expectedResult)

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      expect(result.current.commitResult).toEqual(expectedResult)
    })
  })

  describe('cancel mutation behavior', () => {
    it('should remove preview query on cancel success', async () => {
      const mockSession = {
        id: 'session-cancel-preview',
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
      vi.mocked(api.cancelImport).mockResolvedValue()

      const { queryClient, wrapper } = createWrapper()
      const removeSpy = vi.spyOn(queryClient, 'removeQueries')

      const { result } = renderHook(() => useImport(), { wrapper })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.cancel()
      })

      expect(removeSpy).toHaveBeenCalledWith({ queryKey: ['import-preview'] })
    })

    it('should invalidate status query on cancel success', async () => {
      const mockSession = {
        id: 'session-cancel-status',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      let cancelled = false
      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      vi.mocked(api.getImportStatus).mockImplementation(async () => {
        if (cancelled) return { has_pending: false }
        return { has_pending: true, session: mockSession }
      })
      vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)
      vi.mocked(api.cancelImport).mockImplementation(async () => {
        cancelled = true
      })

      const { queryClient, wrapper } = createWrapper()
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const { result } = renderHook(() => useImport(), { wrapper })

      await act(async () => {
        await result.current.upload('test')
      })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      await act(async () => {
        await result.current.cancel()
      })

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: QUERY_KEY })
    })
  })

  describe('error handling', () => {
    it('should aggregate errors from mutations', async () => {
      const mockError = new Error('Status check failed')
      vi.mocked(api.getImportStatus).mockRejectedValue(mockError)

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await waitFor(() => {
        expect(result.current.error).toBe('Status check failed')
      })
    })

    it('should exclude preview error after commit succeeded', async () => {
      const mockSession = {
        id: 'session-error-exclude',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      vi.mocked(api.getImportStatus).mockImplementation(async () => {
        return { has_pending: false }
      })
      vi.mocked(api.getImportPreview).mockRejectedValue(new Error('Preview not found'))
      vi.mocked(api.commitImport).mockResolvedValue({ created: 1, updated: 0, skipped: 0, errors: [] })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      // After commit succeeds, preview error should not be shown
      expect(result.current.commitSuccess).toBe(true)
      expect(result.current.error).toBeNull()
    })

    it('should handle cancel errors', async () => {
      const mockSession = {
        id: 'session-cancel-error',
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
      vi.mocked(api.cancelImport).mockRejectedValue(new Error('Cancel failed'))

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      let threw = false
      await act(async () => {
        try {
          await result.current.cancel()
        } catch {
          threw = true
        }
      })
      expect(threw).toBe(true)

      await waitFor(() => {
        expect(result.current.error).toBe('Cancel failed')
      })
    })
  })

  describe('state management', () => {
    it('should clear commit result and reset state on clearCommitResult', async () => {
      const mockSession = {
        id: 'session-clear',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockResponse = {
        session: mockSession,
        preview: { hosts: [], conflicts: [], errors: [] },
      }

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockResponse)
      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: false })
      vi.mocked(api.getImportPreview).mockResolvedValue(mockResponse)
      vi.mocked(api.commitImport).mockResolvedValue({ created: 1, updated: 0, skipped: 0, errors: [] })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      await act(async () => {
        await result.current.commit({}, {})
      })

      // Verify state after commit
      expect(result.current.commitSuccess).toBe(true)
      expect(result.current.commitResult).not.toBeNull()

      // Clear and verify reset
      act(() => {
        result.current.clearCommitResult()
      })

      expect(result.current.commitResult).toBeNull()
      expect(result.current.commitSuccess).toBe(false)
    })

    it('should use upload preview when available', async () => {
      const mockSession = {
        id: 'session-upload-preview',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const uploadPreviewData = {
        hosts: [{ domain_names: 'upload-preview.com' }],
        conflicts: [],
        errors: [],
      }

      const mockUploadResponse = {
        session: mockSession,
        preview: uploadPreviewData,
      }

      const statusPreviewData = {
        hosts: [{ domain_names: 'status-preview.com' }],
        conflicts: [],
        errors: [],
      }

      vi.mocked(api.uploadCaddyfile).mockResolvedValue(mockUploadResponse)
      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      // Return different preview from getImportPreview to verify upload preview is preferred
      vi.mocked(api.getImportPreview).mockResolvedValue({
        session: mockSession,
        preview: statusPreviewData,
      })

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await act(async () => {
        await result.current.upload('test')
      })

      // Should use the upload preview, not the status query preview
      expect(result.current.preview).toEqual(mockUploadResponse)
    })

    it('should fallback to status query session when upload preview is null', async () => {
      const mockSession = {
        id: 'session-fallback',
        state: 'reviewing' as const,
        created_at: '2025-01-18T10:00:00Z',
        updated_at: '2025-01-18T10:00:00Z',
      }

      const mockPreview = {
        session: mockSession,
        preview: { hosts: [{ domain_names: 'fallback.com' }], conflicts: [], errors: [] },
      }

      // Don't upload, just have status return a session
      vi.mocked(api.getImportStatus).mockResolvedValue({ has_pending: true, session: mockSession })
      vi.mocked(api.getImportPreview).mockResolvedValue(mockPreview)

      const { result } = renderHook(() => useImport(), { wrapper: createSimpleWrapper() })

      await waitFor(() => {
        expect(result.current.session).toEqual(mockSession)
      })

      // Should use status query session since no upload was performed
      expect(result.current.session?.id).toBe('session-fallback')
    })
  })
})
