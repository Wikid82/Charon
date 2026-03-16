import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import ImportSuccessModal from '../ImportSuccessModal'

describe('ImportSuccessModal', () => {
  const defaultProps = {
    visible: true,
    onClose: vi.fn(),
    onNavigateDashboard: vi.fn(),
    onNavigateHosts: vi.fn(),
    results: {
      created: 5,
      updated: 2,
      skipped: 1,
      errors: [],
    },
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders import summary correctly', () => {
    render(<ImportSuccessModal {...defaultProps} />)

    expect(screen.getByText('Import Completed')).toBeInTheDocument()
    expect(screen.getByText('8 hosts processed')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.getByText(/hosts created/)).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText(/hosts updated/)).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText(/host skipped/)).toBeInTheDocument()
  })

  it('displays certificate provisioning guidance when hosts are created', () => {
    render(<ImportSuccessModal {...defaultProps} />)

    expect(screen.getByText('Certificate Provisioning')).toBeInTheDocument()
    expect(screen.getByText(/SSL certificates will be automatically provisioned/)).toBeInTheDocument()
    expect(screen.getByText(/1-5 minutes per domain/)).toBeInTheDocument()
  })

  it('hides certificate provisioning guidance when no hosts are created', () => {
    const props = {
      ...defaultProps,
      results: { created: 0, updated: 2, skipped: 0, errors: [] },
    }
    render(<ImportSuccessModal {...props} />)

    expect(screen.queryByText('Certificate Provisioning')).not.toBeInTheDocument()
  })

  it('shows errors when present', () => {
    const props = {
      ...defaultProps,
      results: {
        created: 0,
        updated: 0,
        skipped: 0,
        errors: ['example.com: duplicate entry', 'api.example.com: invalid config'],
      },
    }
    render(<ImportSuccessModal {...props} />)

    expect(screen.getByText('2 errors encountered')).toBeInTheDocument()
    expect(screen.getByText('example.com: duplicate entry')).toBeInTheDocument()
    expect(screen.getByText('api.example.com: invalid config')).toBeInTheDocument()
  })

  it('calls onNavigateDashboard when clicking Dashboard button', () => {
    const onNavigateDashboard = vi.fn()
    render(<ImportSuccessModal {...defaultProps} onNavigateDashboard={onNavigateDashboard} />)

    fireEvent.click(screen.getByText('Go to Dashboard'))
    expect(onNavigateDashboard).toHaveBeenCalledTimes(1)
  })

  it('calls onNavigateHosts when clicking View Proxy Hosts button', () => {
    const onNavigateHosts = vi.fn()
    render(<ImportSuccessModal {...defaultProps} onNavigateHosts={onNavigateHosts} />)

    fireEvent.click(screen.getByText('View Proxy Hosts'))
    expect(onNavigateHosts).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when clicking Close button', () => {
    const onClose = vi.fn()
    render(<ImportSuccessModal {...defaultProps} onClose={onClose} />)

    fireEvent.click(screen.getByText('Close'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when clicking backdrop', () => {
    const onClose = vi.fn()
    const { container } = render(<ImportSuccessModal {...defaultProps} onClose={onClose} />)

    // Click the backdrop (the overlay behind the modal)
    const backdrop = container.querySelector('.bg-black\\/60')
    if (backdrop) {
      fireEvent.click(backdrop)
    }
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not render when visible is false', () => {
    render(<ImportSuccessModal {...defaultProps} visible={false} />)

    expect(screen.queryByText('Import Completed')).not.toBeInTheDocument()
  })

  it('does not render when results is null', () => {
    render(<ImportSuccessModal {...defaultProps} results={null} />)

    expect(screen.queryByText('Import Completed')).not.toBeInTheDocument()
  })

  it('handles singular grammar correctly for single host', () => {
    const props = {
      ...defaultProps,
      results: { created: 1, updated: 0, skipped: 0, errors: [] },
    }
    render(<ImportSuccessModal {...props} />)

    expect(screen.getByText('1 host processed')).toBeInTheDocument()
    expect(screen.getByText(/host created/)).toBeInTheDocument()
  })

  it('handles single error with correct grammar', () => {
    const props = {
      ...defaultProps,
      results: {
        created: 0,
        updated: 0,
        skipped: 0,
        errors: ['single error'],
      },
    }
    render(<ImportSuccessModal {...props} />)

    expect(screen.getByText('1 error encountered')).toBeInTheDocument()
  })

  it('shows message when no hosts were processed', () => {
    const props = {
      ...defaultProps,
      results: { created: 0, updated: 0, skipped: 0, errors: [] },
    }
    render(<ImportSuccessModal {...props} />)

    expect(screen.getByText('No hosts were processed')).toBeInTheDocument()
  })
})
