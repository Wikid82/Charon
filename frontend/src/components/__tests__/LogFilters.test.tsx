import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { LogFilters } from '../LogFilters'

const defaultProps = {
  search: '',
  onSearchChange: vi.fn(),
  status: '',
  onStatusChange: vi.fn(),
  level: '',
  onLevelChange: vi.fn(),
  host: '',
  onHostChange: vi.fn(),
  onRefresh: vi.fn(),
  onDownload: vi.fn(),
  isLoading: false,
}

describe('LogFilters', () => {
  it('does not render the removed sort-select dropdown', () => {
    render(<LogFilters {...defaultProps} />)
    expect(screen.queryByTestId('sort-select')).not.toBeInTheDocument()
  })

  it('renders all remaining filter controls with their testids', () => {
    render(<LogFilters {...defaultProps} />)

    expect(screen.getByTestId('search-input')).toBeInTheDocument()
    expect(screen.getByTestId('host-input')).toBeInTheDocument()
    expect(screen.getByTestId('level-select')).toBeInTheDocument()
    expect(screen.getByTestId('status-select')).toBeInTheDocument()
    expect(screen.getByTestId('refresh-button')).toBeInTheDocument()
    expect(screen.getByTestId('download-button')).toBeInTheDocument()
  })

  it('fires onSearchChange when typing in the search input', () => {
    const onSearchChange = vi.fn()
    render(<LogFilters {...defaultProps} onSearchChange={onSearchChange} />)

    fireEvent.change(screen.getByTestId('search-input'), { target: { value: 'error' } })
    expect(onSearchChange).toHaveBeenCalledWith('error')
  })

  it('fires onHostChange when typing in the host input', () => {
    const onHostChange = vi.fn()
    render(<LogFilters {...defaultProps} onHostChange={onHostChange} />)

    fireEvent.change(screen.getByTestId('host-input'), { target: { value: 'example.com' } })
    expect(onHostChange).toHaveBeenCalledWith('example.com')
  })

  it('fires onLevelChange when selecting a level', () => {
    const onLevelChange = vi.fn()
    render(<LogFilters {...defaultProps} onLevelChange={onLevelChange} />)

    fireEvent.change(screen.getByTestId('level-select'), { target: { value: 'ERROR' } })
    expect(onLevelChange).toHaveBeenCalledWith('ERROR')
  })

  it('fires onStatusChange when selecting a status class', () => {
    const onStatusChange = vi.fn()
    render(<LogFilters {...defaultProps} onStatusChange={onStatusChange} />)

    fireEvent.change(screen.getByTestId('status-select'), { target: { value: '5xx' } })
    expect(onStatusChange).toHaveBeenCalledWith('5xx')
  })

  it('fires onRefresh and onDownload from the action buttons', () => {
    const onRefresh = vi.fn()
    const onDownload = vi.fn()
    render(<LogFilters {...defaultProps} onRefresh={onRefresh} onDownload={onDownload} />)

    fireEvent.click(screen.getByTestId('refresh-button'))
    expect(onRefresh).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByTestId('download-button'))
    expect(onDownload).toHaveBeenCalledTimes(1)
  })
})
