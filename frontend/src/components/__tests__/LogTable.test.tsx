import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { type CaddyAccessLog } from '../../api/logs'
import { LogTable } from '../LogTable'

const accessLog = (overrides: Partial<CaddyAccessLog> = {}): CaddyAccessLog => ({
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
  ...overrides,
})

const defaultProps = {
  isLoading: false,
  sortBy: 'ts' as const,
  sortDir: 'desc' as const,
  onSortChange: vi.fn(),
}

describe('LogTable', () => {
  it('shows the translated loading state', () => {
    render(<LogTable {...defaultProps} logs={[]} isLoading />)
    expect(screen.getByText('Loading logs...')).toBeInTheDocument()
  })

  it('shows an empty state matching the E2E regex', () => {
    render(<LogTable {...defaultProps} logs={[]} />)
    expect(screen.getByText(/no logs found|no.*matching/i)).toBeInTheDocument()
  })

  it('keeps the plain column labels as columnheader accessible names', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} />)

    // Regression guard: E2E locates headers via getByRole('columnheader', { name })
    expect(screen.getByRole('columnheader', { name: 'Time' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Level' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Method' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Host' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Path' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: /time/i })).toBeInTheDocument()
  })

  it('renders sort buttons with sort-header testids for each sortable column', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} />)

    for (const field of ['ts', 'level', 'status', 'method', 'uri']) {
      expect(screen.getByTestId(`sort-header-${field}`)).toBeInTheDocument()
    }
    expect(screen.queryByTestId('sort-header-host')).not.toBeInTheDocument()
  })

  it('sets aria-sort descending only on the active column', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} sortBy="ts" sortDir="desc" />)

    expect(screen.getByRole('columnheader', { name: 'Time' })).toHaveAttribute('aria-sort', 'descending')
    expect(screen.getByRole('columnheader', { name: 'Status' })).not.toHaveAttribute('aria-sort')
    expect(screen.getByRole('columnheader', { name: 'Level' })).not.toHaveAttribute('aria-sort')
  })

  it('sets aria-sort ascending when the active direction is asc', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} sortBy="status" sortDir="asc" />)

    expect(screen.getByRole('columnheader', { name: 'Status' })).toHaveAttribute('aria-sort', 'ascending')
    expect(screen.getByRole('columnheader', { name: 'Time' })).not.toHaveAttribute('aria-sort')
  })

  it('invokes onSortChange with the field when a header button is clicked', () => {
    const onSortChange = vi.fn()
    render(<LogTable {...defaultProps} logs={[accessLog()]} onSortChange={onSortChange} />)

    fireEvent.click(screen.getByTestId('sort-header-status'))
    expect(onSortChange).toHaveBeenCalledWith('status')

    fireEvent.click(screen.getByTestId('sort-header-uri'))
    expect(onSortChange).toHaveBeenCalledWith('uri')
  })

  it('renders level badges with level testids and colors', () => {
    render(
      <LogTable
        {...defaultProps}
        logs={[
          accessLog({ level: 'error', status: 500 }),
          accessLog({ level: 'warn', status: 429 }),
          accessLog({ level: 'info', status: 200 }),
          accessLog({ level: 'debug', status: 204 }),
        ]}
      />
    )

    expect(screen.getByTestId('level-error')).toHaveClass('bg-red-100')
    expect(screen.getByTestId('level-warn')).toHaveClass('bg-yellow-100')
    expect(screen.getByTestId('level-info')).toHaveClass('bg-blue-100')
    expect(screen.getByTestId('level-debug')).toHaveClass('bg-gray-100')
  })

  it('falls back to the gray badge for unknown levels', () => {
    render(<LogTable {...defaultProps} logs={[accessLog({ level: 'TRACE' })]} />)
    expect(screen.getByTestId('level-trace')).toHaveClass('bg-gray-100')
  })

  it('highlights 5xx status badges with red styling', () => {
    render(<LogTable {...defaultProps} logs={[accessLog({ status: 502 })]} />)

    // Protects the E2E assertion toHaveClass(/red/)
    expect(screen.getByTestId('status-502').className).toMatch(/red/)
  })

  it('renders plain-text system log rows across the full width', () => {
    const plainLog = accessLog({
      status: 0,
      msg: 'server started on :8080',
      request: undefined as unknown as CaddyAccessLog['request'],
    })
    render(<LogTable {...defaultProps} logs={[plainLog]} />)

    const wideCell = screen.getByText('server started on :8080')
    expect(wideCell).toHaveAttribute('colspan', '8')
  })

  it('dims the table body while a background refetch is in flight', () => {
    const { container, rerender } = render(
      <LogTable {...defaultProps} logs={[accessLog()]} isFetching />
    )
    expect(container.querySelector('tbody')).toHaveClass('opacity-50')

    rerender(<LogTable {...defaultProps} logs={[accessLog()]} isFetching={false} />)
    expect(container.querySelector('tbody')).not.toHaveClass('opacity-50')
  })

  it('marks the table aria-busy while a background refetch is in flight', () => {
    const { rerender } = render(
      <LogTable {...defaultProps} logs={[accessLog()]} isFetching />
    )
    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true')

    rerender(<LogTable {...defaultProps} logs={[accessLog()]} isFetching={false} />)
    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'false')
  })

  it('sets scope="col" on every column header', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} />)

    const headers = screen.getAllByRole('columnheader')
    expect(headers).toHaveLength(9)
    headers.forEach((th) => expect(th).toHaveAttribute('scope', 'col'))
  })

  it('renders request details in access log rows', () => {
    render(<LogTable {...defaultProps} logs={[accessLog()]} />)

    expect(screen.getByText('GET')).toBeInTheDocument()
    expect(screen.getByText('api.example.com')).toBeInTheDocument()
    expect(screen.getByText('/api/v1/users')).toBeInTheDocument()
    expect(screen.getByText('192.168.1.100')).toBeInTheDocument()
    expect(screen.getByText('45.00ms')).toBeInTheDocument()
  })
})
