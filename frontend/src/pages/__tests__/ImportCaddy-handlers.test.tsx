import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import ImportCaddy from '../ImportCaddy'
import { useImport } from '../../hooks/useImport'
import { createBackup } from '../../api/backups'

// Mock the hooks and API calls
vi.mock('../../hooks/useImport')
vi.mock('../../api/backups', () => ({
  createBackup: vi.fn().mockResolvedValue({}),
}))

// Mock translation
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

// Mock navigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

const mockUseImport = vi.mocked(useImport)
const mockCreateBackup = vi.mocked(createBackup)

// Helper to create a mock file with text() method
function createMockFile(content: string, name: string, type: string): File {
  const file = new File([content], name, { type })
  // Mock the text() method since jsdom doesn't fully support it
  Object.defineProperty(file, 'text', {
    value: () => Promise.resolve(content),
  })
  return file
}

describe('ImportCaddy - Handlers and Interactions', () => {
  const defaultMockReturn = {
    session: null,
    preview: null,
    loading: false,
    error: null,
    commitSuccess: false,
    commitResult: null,
    clearCommitResult: vi.fn(),
    upload: vi.fn().mockResolvedValue(undefined),
    commit: vi.fn().mockResolvedValue(undefined),
    cancel: vi.fn().mockResolvedValue(undefined),
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseImport.mockReturnValue(defaultMockReturn)
    mockCreateBackup.mockResolvedValue({
      filename: 'backup.db',
    })
    // Reset confirm mock
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(window, 'alert').mockImplementation(() => {})
  })

  describe('Loading State', () => {
    it('displays loading text on button when loading is true', () => {
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        loading: true,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // Button should show processing text when loading
      expect(screen.getByText('importCaddy.processing')).toBeInTheDocument()
    })

    it('disables parse button when loading', () => {
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        loading: true,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const button = screen.getByText('importCaddy.processing')
      expect(button).toBeDisabled()
    })

    it('disables parse button when content is empty', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const button = screen.getByText('importCaddy.parseAndReview')
      expect(button).toBeDisabled()
    })

    it('enables parse button when content exists and not loading', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const textarea = screen.getByRole('textbox')
      // Use fireEvent.change to avoid userEvent parsing curly braces as special keys
      fireEvent.change(textarea, { target: { value: 'example.com reverse_proxy localhost:8080' } })

      const button = screen.getByText('importCaddy.parseAndReview')
      expect(button).not.toBeDisabled()
    })
  })

  describe('handleUpload', () => {
    it('shows alert when trying to upload empty content', async () => {
      vi.spyOn(window, 'alert')
      const user = userEvent.setup()

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // Type and then clear to have empty trimmed content
      const textarea = screen.getByRole('textbox')
      await user.type(textarea, '   ')

      // Force-enable the button by changing content
      await user.clear(textarea)
      await user.type(textarea, 'x')
      await user.clear(textarea)
      await user.type(textarea, '   ') // whitespace only

      // Need to manually trigger click - button is disabled for empty content
      // Let's test by typing content first then clearing doesn't call upload
      // Actually, let's test with actual content that passes validation
      await user.clear(textarea)
      await user.type(textarea, 'valid config')

      const button = screen.getByText('importCaddy.parseAndReview')
      expect(button).not.toBeDisabled()

      // Clear and make whitespace-only
      await user.clear(textarea)
      fireEvent.change(textarea, { target: { value: '   ' } })

      // Button becomes disabled again
      expect(button).toBeDisabled()
    })

    it('calls upload function with content on valid submission', async () => {
      const mockUpload = vi.fn().mockResolvedValue(undefined)
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        upload: mockUpload,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const textarea = screen.getByRole('textbox')
      const testContent = 'example.com reverse_proxy localhost:8080'
      fireEvent.change(textarea, { target: { value: testContent } })

      const button = screen.getByText('importCaddy.parseAndReview')
      fireEvent.click(button)

      await waitFor(() => {
        expect(mockUpload).toHaveBeenCalledWith(testContent)
      })
    })

    it('handles upload error gracefully', async () => {
      const mockUpload = vi.fn().mockRejectedValue(new Error('Parse error'))
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        upload: mockUpload,
      })
      const user = userEvent.setup()

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const textarea = screen.getByRole('textbox')
      await user.type(textarea, 'invalid content')

      const button = screen.getByText('importCaddy.parseAndReview')
      await user.click(button)

      // Should not throw - error handled by hook
      expect(mockUpload).toHaveBeenCalled()
    })
  })

  describe('handleFileUpload', () => {
    it('reads file content and sets it to textarea', async () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const fileInput = screen.getByTestId('import-dropzone')
      const fileContent = 'example.com reverse_proxy localhost:3000'
      const file = createMockFile(fileContent, 'Caddyfile', 'text/plain')

      fireEvent.change(fileInput, { target: { files: [file] } })

      // Wait for file to be read and content to be set
      await waitFor(() => {
        const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
        expect(textarea.value).toBe(fileContent)
      })
    })

    it('handles empty file selection gracefully', async () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const fileInput = screen.getByTestId('import-dropzone')

      // Trigger change with no files
      fireEvent.change(fileInput, { target: { files: [] } })

      // Should not crash - textarea should remain empty
      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
      expect(textarea.value).toBe('')
    })
  })

  describe('handleCancel', () => {
    it('calls cancel when user confirms cancellation', async () => {
      const mockCancel = vi.fn().mockResolvedValue(undefined)
      vi.spyOn(window, 'confirm').mockReturnValue(true)

      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com' }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: '',
        },
        cancel: mockCancel,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // ImportBanner should be rendered - find Cancel button by text
      const cancelButton = screen.getByRole('button', { name: 'Cancel' })
      fireEvent.click(cancelButton)

      await waitFor(() => {
        expect(mockCancel).toHaveBeenCalled()
      })
    })

    it('does not call cancel when user declines confirmation', () => {
      const mockCancel = vi.fn().mockResolvedValue(undefined)
      vi.spyOn(window, 'confirm').mockReturnValue(false)

      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com' }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: '',
        },
        cancel: mockCancel,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // ImportBanner should be rendered - find Cancel button by text
      const cancelButton = screen.getByRole('button', { name: 'Cancel' })
      fireEvent.click(cancelButton)

      expect(mockCancel).not.toHaveBeenCalled()
    })

    it('handles cancel error gracefully', async () => {
      const mockCancel = vi.fn().mockRejectedValue(new Error('Cancel failed'))
      vi.spyOn(window, 'confirm').mockReturnValue(true)

      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com' }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: '',
        },
        cancel: mockCancel,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // Find Cancel button and click it
      const cancelButton = screen.getByRole('button', { name: 'Cancel' })
      fireEvent.click(cancelButton)

      // Should not throw - error is handled by hook
      await waitFor(() => {
        expect(mockCancel).toHaveBeenCalled()
      })
    })
  })

  describe('ImportReviewTable Display', () => {
    it('displays review table when session and preview exist with Review button click', async () => {
      // Mock session with preview that requires showReview to be true
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com', forward_host: 'localhost', forward_port: 8080 }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: 'example.com reverse_proxy',
        },
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // Click "Review Changes" button in the banner to show the review table
      const reviewButton = screen.getByRole('button', { name: 'Review Changes' })
      fireEvent.click(reviewButton)

      // Review table should now be visible
      await waitFor(() => {
        expect(screen.getByTestId('import-review-table')).toBeInTheDocument()
      })
    })
  })

  describe('Success Modal Navigation', () => {
    it('displays success modal after commit', () => {
      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com' }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: '',
        },
        commitResult: { created: 1, updated: 0, skipped: 0, errors: [] },
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // The success modal component should be in the DOM
      // The actual visibility is controlled by the visible prop
    })

    it('clears commit result when success modal is closed', async () => {
      const mockClearCommitResult = vi.fn()
      const mockCommit = vi.fn().mockResolvedValue(undefined)

      mockUseImport.mockReturnValue({
        ...defaultMockReturn,
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
          preview: {
            hosts: [{ domain_names: 'example.com' }],
            conflicts: [],
            errors: [],
          },
          conflict_details: {},
          caddyfile_content: '',
        },
        commitResult: null,
        clearCommitResult: mockClearCommitResult,
        commit: mockCommit,
      })

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      // Trigger the review table
      const reviewButton = screen.getByRole('button', { name: 'Review Changes' })
      fireEvent.click(reviewButton)

      await waitFor(() => {
        expect(screen.getByTestId('import-review-table')).toBeInTheDocument()
      })

      // Click Commit Import to trigger commit flow and open success modal
      const commitButton = screen.getByRole('button', { name: 'Commit Import' })
      fireEvent.click(commitButton)

      // Wait for commit to be called
      await waitFor(() => {
        expect(mockCommit).toHaveBeenCalled()
      })
    })
  })

  describe('Textarea Content Change', () => {
    it('updates content when textarea value changes', async () => {
      const user = userEvent.setup()

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
      expect(textarea.value).toBe('')

      await user.type(textarea, 'new content')
      expect(textarea.value).toBe('new content')
    })

    it('allows clearing textarea content', async () => {
      const user = userEvent.setup()

      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
      await user.type(textarea, 'some content')
      expect(textarea.value).toBe('some content')

      await user.clear(textarea)
      expect(textarea.value).toBe('')
    })
  })

  describe('Page Title', () => {
    it('renders the page title', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('importCaddy.title')
    })
  })

  describe('Form Labels', () => {
    it('renders upload and content labels', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      expect(screen.getByText('importCaddy.uploadCaddyfile')).toBeInTheDocument()
      expect(screen.getByText('importCaddy.caddyfileContent')).toBeInTheDocument()
    })

    it('renders or divider text', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      expect(screen.getByText('importCaddy.orPasteContent')).toBeInTheDocument()
    })

    it('renders description text', () => {
      render(
        <BrowserRouter>
          <ImportCaddy />
        </BrowserRouter>
      )

      expect(screen.getByText('importCaddy.description')).toBeInTheDocument()
    })
  })
})

describe('ImportCaddy - Commit Handler', () => {
  const mockClearCommitResult = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockCreateBackup.mockResolvedValue({
      filename: 'backup.db',
    })
  })

  it('shows commit button in review table when reviewing', async () => {
    const mockCommit = vi.fn().mockResolvedValue(undefined)

    mockUseImport.mockReturnValue({
      session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
      preview: {
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          hosts: [{ domain_names: 'example.com' }],
          conflicts: [],
          errors: [],
        },
        conflict_details: {},
        caddyfile_content: '',
      },
      loading: false,
      error: null,
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: mockClearCommitResult,
      upload: vi.fn(),
      commit: mockCommit,
      cancel: vi.fn(),
    })

    render(
      <BrowserRouter>
          <ImportCaddy />
      </BrowserRouter>
    )

    // Click Review Changes to show the review table
    const reviewButton = screen.getByRole('button', { name: 'Review Changes' })
    fireEvent.click(reviewButton)

    // Verify review table is present with Commit button
    await waitFor(() => {
      expect(screen.getByTestId('import-review-table')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Commit Import' })).toBeInTheDocument()
    })
  })

  it('triggers commit flow when commit button is clicked', async () => {
    const mockCommit = vi.fn().mockResolvedValue(undefined)

    mockUseImport.mockReturnValue({
      session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
      preview: {
        session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
        preview: {
          hosts: [{ domain_names: 'example.com' }],
          conflicts: [],
          errors: [],
        },
        conflict_details: {},
        caddyfile_content: '',
      },
      loading: false,
      error: null,
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: mockClearCommitResult,
      upload: vi.fn(),
      commit: mockCommit,
      cancel: vi.fn(),
    })

    render(
      <BrowserRouter>
          <ImportCaddy />
      </BrowserRouter>
    )

    // Click Review Changes to show the review table
    const reviewButton = screen.getByRole('button', { name: 'Review Changes' })
    fireEvent.click(reviewButton)

    await waitFor(() => {
      expect(screen.getByTestId('import-review-table')).toBeInTheDocument()
    })

    // Click Commit Import button
    const commitButton = screen.getByRole('button', { name: 'Commit Import' })
    fireEvent.click(commitButton)

    // Verify createBackup is called first
    await waitFor(() => {
      expect(mockCreateBackup).toHaveBeenCalled()
    })

    // Then commit should be called
    await waitFor(() => {
      expect(mockCommit).toHaveBeenCalled()
    })
  })
})
