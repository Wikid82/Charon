import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import ImportCaddy from '../ImportCaddy'
import { useImport } from '../../hooks/useImport'

// Mock the hooks and API calls
vi.mock('../../hooks/useImport')
vi.mock('../../api/backups', () => ({
  createBackup: vi.fn().mockResolvedValue({}),
}))

const mockUseImport = vi.mocked(useImport)

describe('ImportCaddy - Multi-File Modal', () => {
  const defaultMockReturn = {
    session: null,
    preview: null,
    loading: false,
    error: null,
    commitSuccess: false,
    commitResult: null,
    clearCommitResult: vi.fn(),
    upload: vi.fn(),
    commit: vi.fn(),
    cancel: vi.fn(),
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseImport.mockReturnValue(defaultMockReturn)
  })

  it('renders multi-file button when no session exists', () => {
    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    const button = screen.getByTestId('multi-file-import-button')
    expect(button).toBeInTheDocument()
    expect(button).toHaveTextContent(/multi.*site.*import/i)
  })

  it('shows import banner when session exists (multi-file hidden during active session)', () => {
    mockUseImport.mockReturnValueOnce({
      ...defaultMockReturn,
      session: { id: 'test-session-id', state: 'reviewing', created_at: '', updated_at: '' },
    })

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    // When a session exists, the import banner is shown instead of the upload form
    expect(screen.getByTestId('import-banner')).toBeInTheDocument()
    // Multi-file button is part of upload form, which is hidden during active session
    expect(screen.queryByTestId('multi-file-import-button')).not.toBeInTheDocument()
  })

  it('opens modal when multi-file button is clicked', async () => {
    const user = userEvent.setup()

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    const button = screen.getByTestId('multi-file-import-button')
    await user.click(button)

    await waitFor(() => {
      const modal = screen.getByTestId('multi-site-modal')
      expect(modal).toBeInTheDocument()
    })
  })

  it('modal has correct accessibility attributes', async () => {
    const user = userEvent.setup()

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    const button = screen.getByTestId('multi-file-import-button')
    await user.click(button)

    await waitFor(() => {
      const modal = screen.getByRole('dialog')
      expect(modal).toBeInTheDocument()
      expect(modal).toHaveAttribute('aria-modal', 'true')
      expect(modal).toHaveAttribute('aria-labelledby', 'multi-site-modal-title')
      expect(modal).toHaveAttribute('data-testid', 'multi-site-modal')
    })
  })

  it('modal contains correct title for screen readers', async () => {
    const user = userEvent.setup()

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    const button = screen.getByTestId('multi-file-import-button')
    await user.click(button)

    await waitFor(() => {
      // Use heading role to specifically target the modal title, not the button
      const title = screen.getByRole('heading', { name: 'Multi-site Import' })
      expect(title).toBeInTheDocument()
      expect(title).toHaveAttribute('id', 'multi-site-modal-title')
    })
  })

  it('closes modal when clicking outside overlay', async () => {
    const user = userEvent.setup()

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    // Open modal
    const button = screen.getByTestId('multi-file-import-button')
    await user.click(button)

    // Wait for modal to appear
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    // Click overlay (the semi-transparent background)
    const overlay = screen.getByRole('dialog').querySelector('.bg-black\\/60')
    expect(overlay).toBeInTheDocument()

    if (overlay) {
      await user.click(overlay)

      // Modal should close
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      })
    }
  })

  it('opens modal and shows it correctly', async () => {
    const user = userEvent.setup()

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    const button = screen.getByTestId('multi-file-import-button')
    await user.click(button)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    // Verify modal is displayed
    const modal = screen.getByRole('dialog')
    expect(modal).toBeInTheDocument()
  })

  it('modal button text matches E2E test selector', () => {
    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    // E2E test uses: page.getByRole('button', { name: /multi.*file|multi.*site/i })
    const button = screen.getByRole('button', { name: /multi.*file|multi.*site/i })
    expect(button).toBeInTheDocument()
  })

  it('handles error state from import', async () => {
    mockUseImport.mockReturnValueOnce({
      ...defaultMockReturn,
      error: 'Import directives detected',
    })

    render(
      <BrowserRouter>
        <ImportCaddy />
      </BrowserRouter>
    )

    // Error message should display
    expect(screen.getByText(/Import directives detected/i)).toBeInTheDocument()
  })
})
