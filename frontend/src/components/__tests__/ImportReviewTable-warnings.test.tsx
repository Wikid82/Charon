import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ImportReviewTable from '../ImportReviewTable'

describe('ImportReviewTable - Status Display', () => {
  const mockOnCommit = vi.fn()
  const mockOnCancel = vi.fn()

  it('displays New badge for hosts without conflicts', () => {
    const hosts = [
      {
        domain_names: 'app.example.com',
        forward_host: 'localhost',
        forward_port: 8080,
      },
    ]

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={[]}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('New')).toBeInTheDocument()
  })

  it('displays Conflict badge for hosts in conflicts array', () => {
    const hosts = [
      {
        domain_names: 'conflict.example.com',
        forward_host: 'localhost',
        forward_port: 80,
      },
    ]

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={['conflict.example.com']}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Conflict')).toBeInTheDocument()
    expect(screen.queryByText('New')).not.toBeInTheDocument()
  })

  it('shows expand button only for hosts with conflicts', () => {
    const hosts = [
      {
        domain_names: 'conflict.example.com',
        forward_host: 'localhost',
        forward_port: 80,
      },
      {
        domain_names: 'new.example.com',
        forward_host: 'localhost',
        forward_port: 8080,
      },
    ]

    const conflictDetails = {
      'conflict.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 80,
          ssl_forced: false,
          websocket: false,
          enabled: true,
        },
        imported: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 8080,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={['conflict.example.com']}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    // Expand button shows as triangle character
    const expandButtons = screen.getAllByRole('button', { name: /▶/ })
    expect(expandButtons).toHaveLength(1)
  })

  it('expands to show conflict details when clicked', async () => {
    const hosts = [
      {
        domain_names: 'conflict.example.com',
        forward_host: 'localhost',
        forward_port: 80,
      },
    ]

    const conflictDetails = {
      'conflict.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 80,
          ssl_forced: false,
          websocket: false,
          enabled: true,
        },
        imported: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 8080,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={['conflict.example.com']}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const expandButton = screen.getByRole('button', { name: /▶/ })
    fireEvent.click(expandButton)

    expect(screen.getByText('Current Configuration')).toBeInTheDocument()
    expect(screen.getByText('Imported Configuration')).toBeInTheDocument()
  })

  it('collapses conflict details when clicked again', () => {
    const hosts = [
      {
        domain_names: 'conflict.example.com',
        forward_host: 'localhost',
        forward_port: 80,
      },
    ]

    const conflictDetails = {
      'conflict.example.com': {
        existing: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 80,
          ssl_forced: false,
          websocket: false,
          enabled: true,
        },
        imported: {
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 8080,
          ssl_forced: false,
          websocket: false,
        },
      },
    }

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={['conflict.example.com']}
        conflictDetails={conflictDetails}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const expandButton = screen.getByRole('button', { name: /▶/ })

    // Expand
    fireEvent.click(expandButton)
    expect(screen.getByText('Current Configuration')).toBeInTheDocument()

    // Collapse (now button shows ▼)
    const collapseButton = screen.getByRole('button', { name: /▼/ })
    fireEvent.click(collapseButton)
    expect(screen.queryByText('Current Configuration')).not.toBeInTheDocument()
  })

  it('shows conflict resolution dropdown for conflicting hosts', () => {
    const hosts = [
      {
        domain_names: 'conflict.example.com',
        forward_host: 'localhost',
        forward_port: 80,
      },
    ]

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={['conflict.example.com']}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
    expect(screen.getByText('Keep Existing (Skip Import)')).toBeInTheDocument()
    expect(screen.getByText('Replace with Imported')).toBeInTheDocument()
  })

  it('shows "Will be imported" text for non-conflicting hosts', () => {
    const hosts = [
      {
        domain_names: 'new.example.com',
        forward_host: 'localhost',
        forward_port: 8080,
      },
    ]

    render(
      <ImportReviewTable
        hosts={hosts}
        conflicts={[]}
        errors={[]}
        onCommit={mockOnCommit}
        onCancel={mockOnCancel}
      />
    )

    expect(screen.getByText('Will be imported')).toBeInTheDocument()
  })
})
