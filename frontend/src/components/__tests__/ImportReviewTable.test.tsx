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

    expect(screen.getByText('Review Imported Hosts')).toBeInTheDocument()
    expect(screen.getByText('test.example.com')).toBeInTheDocument()
  })

  it('displays conflicts with resolution dropdowns', () => {
    const conflicts = ['test.example.com']
    render(
      <ImportReviewTable
        hosts={mockImportPreview.hosts}
        conflicts={conflicts}
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
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByRole('combobox')).toBeInTheDocument()
    expect(screen.queryByText('No conflict')).not.toBeInTheDocument()
  })
})
