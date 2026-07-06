import { screen, waitFor, fireEvent } from '@testing-library/react'
import toast from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { getLogs, getLogContent, downloadLog, type LogFilter, type LogResponse } from '../../api/logs'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Logs from '../Logs'

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

const mockFiles = [
  { name: 'access.log', size: 1048576, mod_time: '2026-07-04T00:00:00Z' },
  { name: 'error.log', size: 256000, mod_time: '2026-07-04T00:00:00Z' },
]

const entry = {
  level: 'info',
  ts: 1751630400,
  logger: 'http.log.access',
  msg: 'handled request',
  request: {
    remote_ip: '192.168.1.100',
    method: 'GET',
    host: 'api.example.com',
    uri: '/api/v1/users',
    proto: 'HTTP/2',
  },
  status: 200,
  duration: 0.045,
  size: 1234,
}

function mockContent(overrides: Partial<LogResponse> = {}) {
  vi.mocked(getLogContent).mockImplementation((filename: string, filter: LogFilter = {}) =>
    Promise.resolve({
      filename,
      logs: [entry],
      total: 150,
      limit: filter.limit ?? 50,
      offset: filter.offset ?? 0,
      skipped_lines: 0,
      ...overrides,
    })
  )
}

function lastFilter(): LogFilter {
  const calls = vi.mocked(getLogContent).mock.calls
  return calls[calls.length - 1][1] as LogFilter
}

async function renderPage() {
  renderWithQueryClient(<Logs />)
  await waitFor(() => expect(getLogContent).toHaveBeenCalled())
  await screen.findByTestId('log-table')
}

describe('Logs page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getLogs).mockResolvedValue(mockFiles)
    vi.mocked(downloadLog).mockResolvedValue(undefined)
    mockContent()
  })

  it('auto-selects the first log file and queries with default sort ts/desc', async () => {
    await renderPage()

    expect(vi.mocked(getLogContent).mock.calls[0][0]).toBe('access.log')
    const filter = lastFilter()
    expect(filter.sortBy).toBe('ts')
    expect(filter.sort).toBe('desc')
    expect(filter.offset).toBe(0)
    expect(filter.limit).toBe(50)
  })

  it('toggles direction when the active Time column is clicked', async () => {
    await renderPage()

    fireEvent.click(screen.getByTestId('sort-header-ts'))
    await waitFor(() => expect(lastFilter().sort).toBe('asc'))
    expect(lastFilter().sortBy).toBe('ts')

    fireEvent.click(screen.getByTestId('sort-header-ts'))
    await waitFor(() => expect(lastFilter().sort).toBe('desc'))
    expect(lastFilter().sortBy).toBe('ts')
  })

  it('activates a new column with descending direction', async () => {
    await renderPage()

    fireEvent.click(screen.getByTestId('sort-header-status'))

    await waitFor(() => expect(lastFilter().sortBy).toBe('status'))
    expect(lastFilter().sort).toBe('desc')
  })

  it('resets to the first page when the sort changes', async () => {
    await renderPage()

    fireEvent.click(screen.getByTestId('next-page-button'))
    await waitFor(() => expect(lastFilter().offset).toBe(50))

    fireEvent.click(screen.getByTestId('sort-header-uri'))
    await waitFor(() => expect(lastFilter().sortBy).toBe('uri'))
    expect(lastFilter().offset).toBe(0)
  })

  it('resets to the first page when switching log files', async () => {
    await renderPage()

    fireEvent.click(screen.getByTestId('next-page-button'))
    await waitFor(() => expect(lastFilter().offset).toBe(50))

    fireEvent.click(screen.getByTestId('log-file-error.log'))
    await waitFor(() => expect(vi.mocked(getLogContent).mock.calls.at(-1)?.[0]).toBe('error.log'))
    expect(lastFilter().offset).toBe(0)
  })

  it('debounces search input before querying and resets the page', async () => {
    await renderPage()
    const callsBefore = vi.mocked(getLogContent).mock.calls.length

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'users' } })

    // No immediate query with the search term (debounced 300ms)
    expect(vi.mocked(getLogContent).mock.calls.length).toBe(callsBefore)

    await waitFor(() => expect(lastFilter().search).toBe('users'), { timeout: 2000 })
    expect(lastFilter().offset).toBe(0)
  })

  it('downloads the selected log via the download button', async () => {
    await renderPage()

    fireEvent.click(screen.getByTestId('download-button'))

    await waitFor(() => expect(downloadLog).toHaveBeenCalledWith('access.log'))
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('shows an error toast when the download fails and stays on the page', async () => {
    vi.mocked(downloadLog).mockRejectedValue(new Error('Log file not found'))
    await renderPage()

    fireEvent.click(screen.getByTestId('download-button'))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to download log file'))
    expect(screen.getByTestId('log-table')).toBeInTheDocument()
  })

  it('shows the skipped-lines warning when the backend reports skipped lines', async () => {
    mockContent({ skipped_lines: 3 })
    await renderPage()

    expect(await screen.findByTestId('skipped-lines-warning')).toBeInTheDocument()
  })

  it('hides the skipped-lines warning when no lines were skipped', async () => {
    await renderPage()

    expect(screen.queryByTestId('skipped-lines-warning')).not.toBeInTheDocument()
  })

  it('shows the empty state when no logs match the filter', async () => {
    mockContent({ logs: [], total: 0 })
    await renderPage()

    expect(await screen.findByText(/no logs found|no.*matching/i)).toBeInTheDocument()
    expect(screen.queryByTestId('page-info')).not.toBeInTheDocument()
  })

  it('renders pagination info for the current page', async () => {
    await renderPage()

    expect(screen.getByTestId('page-info')).toHaveTextContent('Showing 1 to 50 of 150 entries')
    expect(screen.getByTestId('prev-page-button')).toBeDisabled()
    expect(screen.getByTestId('next-page-button')).toBeEnabled()
  })
})
