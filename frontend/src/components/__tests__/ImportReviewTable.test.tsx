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
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Hosts to Import (1)')).toBeInTheDocument()
    expect(screen.getByText('test.example.com')).toBeInTheDocument()
  })

  it('displays conflicts with resolution dropdowns', () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={mockImportPreview.conflicts}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText(/Conflicts Detected \(1\)/)).toBeInTheDocument()
    expect(screen.getByText('app.local.dev')).toBeInTheDocument()
    expect(screen.getByText('-- Choose action --')).toBeInTheDocument()
  })

  it('displays errors', () => {
    const errors = ['Invalid Caddyfile syntax', 'Missing required field']

    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={[]}
        errors={errors}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Errors')).toBeInTheDocument()
    expect(screen.getByText('Invalid Caddyfile syntax')).toBeInTheDocument()
    expect(screen.getByText('Missing required field')).toBeInTheDocument()
  })

  it('disables commit button until all conflicts are resolved', () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={mockImportPreview.conflicts}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const commitButton = screen.getByText('Commit Import')
    expect(commitButton).toBeDisabled()
  })

  it('enables commit button when all conflicts are resolved', async () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={mockImportPreview.conflicts}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const dropdown = screen.getAllByRole('combobox')[0]
    fireEvent.change(dropdown, { target: { value: 'skip' } })

    await waitFor(() => {
      const commitButton = screen.getByText('Commit Import')
      expect(commitButton).not.toBeDisabled()
    })
  })

  it('calls onCommit with resolutions', async () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={mockImportPreview.conflicts}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const dropdown = screen.getAllByRole('combobox')[0]
    fireEvent.change(dropdown, { target: { value: 'overwrite' } })

    const commitButton = screen.getByText('Commit Import')
    fireEvent.click(commitButton)

    await waitFor(() => {
      expect(mockOnCommit).toHaveBeenCalledWith({
        'app.local.dev': 'overwrite',
      })
    })
  })

  it('calls onCancel when cancel button is clicked', () => {
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={[]}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledOnce()
  })

  it('shows conflict indicator on conflicting hosts', () => {
    render(
      <ImportReviewTable
        hosts={[
          {
            domain_names: 'app.local.dev',
            forward_scheme: 'http',
            forward_host: 'localhost',
            forward_port: 3000,
            ssl_forced: false,
            http2_support: false,
            websocket_support: false,
          },
        ]}
        conflicts={['app.local.dev']}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Conflict')).toBeInTheDocument()
  })
})
