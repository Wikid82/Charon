import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useImport } from '../useImport'
import * as api from '../../services/api'

// Mock the API
vi.mock('../../services/api', () => ({
  importAPI: {
    status: vi.fn(),
    preview: vi.fn(),
    upload: vi.fn(),
    commit: vi.fn(),
    cancel: vi.fn(),
  },
}))

describe('useImport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.importAPI.status).mockResolvedValue({ has_pending: false })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('starts with no active session', async () => {
    const { result } = renderHook(() => useImport())

    await waitFor(() => {
      expect(result.current.session).toBeNull()
    })
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('uploads content and creates session', async () => {
    const mockSession = {
      uuid: 'session-1',
      filename: 'Caddyfile',
      state: 'reviewing',
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    const mockPreview = {
      hosts: [{ domain: 'test.com' }],
      conflicts: [],
      errors: [],
    }

    vi.mocked(api.importAPI.upload).mockResolvedValue({ session: mockSession })
    vi.mocked(api.importAPI.status).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.importAPI.preview).mockResolvedValue(mockPreview)

    const { result } = renderHook(() => useImport())

    await act(async () => {
      await result.current.upload('example.com { reverse_proxy localhost:8080 }')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    expect(api.importAPI.upload).toHaveBeenCalledWith('example.com { reverse_proxy localhost:8080 }', undefined)
    expect(result.current.loading).toBe(false)
  })

  it('handles upload errors', async () => {
    const mockError = new Error('Upload failed')
    vi.mocked(api.importAPI.upload).mockRejectedValue(mockError)

    const { result } = renderHook(() => useImport())

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
      uuid: 'session-2',
      filename: 'Caddyfile',
      state: 'reviewing',
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    vi.mocked(api.importAPI.upload).mockResolvedValue({ session: mockSession })
    // Keep session pending during initial checks so upload retains session state
    vi.mocked(api.importAPI.status).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.importAPI.preview).mockResolvedValue({ hosts: [], conflicts: [], errors: [] })
    vi.mocked(api.importAPI.commit).mockResolvedValue({})

    const { result } = renderHook(() => useImport())

    await act(async () => {
      await result.current.upload('test')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    await act(async () => {
      await result.current.commit({ 'test.com': 'skip' })
    })

    expect(api.importAPI.commit).toHaveBeenCalledWith('session-2', { 'test.com': 'skip' })

    await waitFor(() => {
      expect(result.current.session).toBeNull()
    })
  })

  it('cancels active import session', async () => {
    const mockSession = {
      uuid: 'session-3',
      filename: 'Caddyfile',
      state: 'reviewing',
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    vi.mocked(api.importAPI.upload).mockResolvedValue({ session: mockSession })
    vi.mocked(api.importAPI.status).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.importAPI.preview).mockResolvedValue({ hosts: [], conflicts: [], errors: [] })
    vi.mocked(api.importAPI.cancel).mockResolvedValue(undefined)

    const { result } = renderHook(() => useImport())

    await act(async () => {
      await result.current.upload('test')
    })

    await waitFor(() => {
      expect(result.current.session).toEqual(mockSession)
    })

    await act(async () => {
      await result.current.cancel()
    })

    expect(api.importAPI.cancel).toHaveBeenCalledWith('session-3')
    await waitFor(() => {
      expect(result.current.session).toBeNull()
    })
  })

  it('handles commit errors', async () => {
    const mockSession = {
      uuid: 'session-4',
      filename: 'Caddyfile',
      state: 'reviewing',
      created_at: '2025-01-18T10:00:00Z',
      updated_at: '2025-01-18T10:00:00Z',
    }

    vi.mocked(api.importAPI.upload).mockResolvedValue({ session: mockSession })
    vi.mocked(api.importAPI.status).mockResolvedValue({ has_pending: true, session: mockSession })
    vi.mocked(api.importAPI.preview).mockResolvedValue({ hosts: [], conflicts: [], errors: [] })

    const mockError = new Error('Commit failed')
    vi.mocked(api.importAPI.commit).mockRejectedValue(mockError)

    const { result } = renderHook(() => useImport())

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
