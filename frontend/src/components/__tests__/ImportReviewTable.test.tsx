import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ImportReviewTable from '../ImportReviewTable'
import { mockImportPreview } from '../../test/mockData'

describe('ImportReviewTable', () => {
  const mockOnCommit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('displays hosts to import', () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={[]}
        conflictDetails={{}}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Review Imported Hosts')).toBeInTheDocument()
    expect(screen.getByText('test.example.com')).toBeInTheDocument()
  })

  it('displays conflicts with resolution dropdowns', () => {
    const conflicts = ['test.example.com']
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={{}}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('test.example.com')).toBeInTheDocument()
    expect(screen.getByRole('combobox')).toBeInTheDocument()
  })

  it('displays errors', () => {
    const errors = ['Invalid Caddyfile syntax', 'Missing required field']

    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={[]}
        conflictDetails={{}}
        errors={errors}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Issues found during parsing')).toBeInTheDocument()
    expect(screen.getByText('Invalid Caddyfile syntax')).toBeInTheDocument()
    expect(screen.getByText('Missing required field')).toBeInTheDocument()
  })

  it('calls onCommit with resolutions', async () => {
    const conflicts = ['test.example.com']
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={{}}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const dropdown = screen.getByRole('combobox')
    fireEvent.change(dropdown, { target: { value: 'overwrite' } })

    const commitButton = screen.getByText('Commit Import')
    fireEvent.click(commitButton)

    await waitFor(() => {
      expect(mockOnCommit).toHaveBeenCalledWith({
        'test.example.com': 'overwrite',
      })
    })
  })

  it('calls onCancel when cancel button is clicked', () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={[]}
        conflictDetails={{}}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    fireEvent.click(screen.getByText('Back'))
    expect(mockOnCancel).toHaveBeenCalledOnce()
  })

  it('shows conflict indicator on conflicting hosts', () => {
    const conflicts = ['test.example.com']
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={{}}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByRole('combobox')).toBeInTheDocument()
    expect(screen.queryByText('No conflict')).not.toBeInTheDocument()
  })

  it('expands and collapses conflict details', () => {
    const conflicts = ['test.example.com']
    const conflictDetails = {
      'test.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: '192.168.1.1',
          forward_port: 8080,
          ssl_forced: true,
          websocket: true,
          enabled: true,
        },
        imported: {
          forward_scheme: 'http',
          forward_host: '192.168.1.2',
          forward_port: 9090,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    // Initially collapsed
    expect(screen.queryByText('Current Configuration')).not.toBeInTheDocument()

    // Find and click expand button (it's the ▶ button)
    const expandButton = screen.getByText('▶')
    fireEvent.click(expandButton)

    // Now should show details
    expect(screen.getByText('Current Configuration')).toBeInTheDocument()
    expect(screen.getByText('Imported Configuration')).toBeInTheDocument()
    expect(screen.getByText('http://192.168.1.1:8080')).toBeInTheDocument()
    expect(screen.getByText('http://192.168.1.2:9090')).toBeInTheDocument()

    // Click collapse button
    const collapseButton = screen.getByText('▼')
    fireEvent.click(collapseButton)

    // Details should be hidden again
    expect(screen.queryByText('Current Configuration')).not.toBeInTheDocument()
  })

  it('shows recommendation based on configuration differences', () => {
    const conflicts = ['test.example.com']
    const conflictDetails = {
      'test.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: '192.168.1.1',
          forward_port: 8080,
          ssl_forced: true,
          websocket: false,
          enabled: true,
        },
        imported: {
          forward_scheme: 'http',
          forward_host: '192.168.1.1',
          forward_port: 8080,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    // Expand to see recommendation
    const expandButton = screen.getByText('▶')
    fireEvent.click(expandButton)

    // Should show recommendation about config changes (SSL differs)
    expect(screen.getByText(/different SSL or WebSocket settings/i)).toBeInTheDocument()
  })

  it('highlights configuration differences', () => {
    const conflicts = ['test.example.com']
    const conflictDetails = {
      'test.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: '192.168.1.1',
          forward_port: 8080,
          ssl_forced: true,
          websocket: true,
          enabled: true,
        },
        imported: {
          forward_scheme: 'https',
          forward_host: '192.168.1.2',
          forward_port: 9090,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const expandButton = screen.getByText('▶')
    fireEvent.click(expandButton)

    // Check for differences being displayed
    expect(screen.getByText('https://192.168.1.2:9090')).toBeInTheDocument()
    expect(screen.getByText('http://192.168.1.1:8080')).toBeInTheDocument()
  })
})
