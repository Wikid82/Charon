import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import toast from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { getLogs, getLogContent, downloadLog, type LogResponse } from '../../api/logs'
import { logKeys, useLogFiles, useLogContent, useDownloadLog } from '../useLogs'

vi.mock('../../api/logs', () => ({
  getLogs: vi.fn(),
  getLogContent: vi.fn(),
  downloadLog: vi.fn(),
}))

vi.mock('react-hot-toast', () => ({
  default: {
    error: vi.fn(),
    success: vi.fn(),
  },
}))

const mockFiles = [{ name: 'access.log', size: 1024, mod_time: '2026-07-04T00:00:00Z' }]

const mockResponse: LogResponse = {
  filename: 'access.log',
  logs: [],
  total: 0,
  limit: 50,
  offset: 0,
  skipped_lines: 0,
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('logKeys', () => {
  it('builds stable query keys', () => {
    expect(logKeys.all).toEqual(['logs'])
    expect(logKeys.content('access.log', { limit: 50 })).toEqual(['logs', 'access.log', { limit: 50 }])
  })
})

describe('useLogFiles', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches the log file list', async () => {
    vi.mocked(getLogs).mockResolvedValue(mockFiles)

    const { result } = renderHook(() => useLogFiles(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(getLogs).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual(mockFiles)
  })
})

describe('useLogContent', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches log content for a selected file', async () => {
    vi.mocked(getLogContent).mockResolvedValue(mockResponse)

    const { result } = renderHook(
      () => useLogContent('access.log', { limit: 50, offset: 0, sort: 'desc', sortBy: 'ts' }),
      { wrapper: createWrapper() }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(getLogContent).toHaveBeenCalledWith('access.log', { limit: 50, offset: 0, sort: 'desc', sortBy: 'ts' })
    expect(result.current.data).toEqual(mockResponse)
  })

  it('is disabled when no filename is selected', async () => {
    const { result } = renderHook(() => useLogContent(null, {}), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.fetchStatus).toBe('idle'))
    expect(getLogContent).not.toHaveBeenCalled()
    expect(result.current.data).toBeUndefined()
  })

  it('keeps previous data while a new page is fetched', async () => {
    vi.mocked(getLogContent).mockResolvedValue(mockResponse)

    const wrapper = createWrapper()
    const { result, rerender } = renderHook(
      ({ offset }: { offset: number }) => useLogContent('access.log', { limit: 50, offset }),
      { wrapper, initialProps: { offset: 0 } }
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    let resolveSecond: (value: LogResponse) => void = () => {}
    vi.mocked(getLogContent).mockImplementation(
      () => new Promise((resolve) => { resolveSecond = resolve })
    )

    rerender({ offset: 50 })

    // Previous page stays rendered as placeholder data while fetching
    expect(result.current.data).toEqual(mockResponse)
    expect(result.current.isPlaceholderData).toBe(true)
    expect(result.current.isFetching).toBe(true)

    resolveSecond({ ...mockResponse, offset: 50 })
    await waitFor(() => expect(result.current.isPlaceholderData).toBe(false))
    expect(result.current.data?.offset).toBe(50)
  })
})

describe('useDownloadLog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('invokes downloadLog with the filename on mutate', async () => {
    vi.mocked(downloadLog).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDownloadLog(), { wrapper: createWrapper() })

    result.current.mutate('access.log')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(downloadLog).toHaveBeenCalledWith('access.log')
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('shows a translated error toast when the download fails', async () => {
    vi.mocked(downloadLog).mockRejectedValue(new Error('boom'))

    const { result } = renderHook(() => useDownloadLog(), { wrapper: createWrapper() })

    result.current.mutate('access.log')

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(toast.error).toHaveBeenCalledWith('Failed to download log file')
  })
})
