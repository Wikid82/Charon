import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { LogoCustomizer } from '../LogoCustomizer'

// Mock useAuth hook
vi.mock('../../../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

// Mock i18n
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'appearance.logoPreview': 'Logo Preview',
        'appearance.logoUploadTab': 'Upload File',
        'appearance.logoUrlTab': 'Enter URL',
        'appearance.logoResetButton': 'Reset to Default',
        'appearance.logoSaveButton': 'Save Logo',
        'appearance.logoUploadHint': 'PNG, JPG or WebP — max 2 MB',
        'appearance.logoUrlPlaceholder': 'https://example.com/logo.png',
        'appearance.logoUrlHttpsRequired': 'URL must start with https://',
      }
      return translations[key] ?? key
    },
  }),
}))

import { useAuth } from '../../../hooks/useAuth'

const mockUseAuth = vi.mocked(useAuth)

function renderAdmin(props: Partial<React.ComponentProps<typeof LogoCustomizer>> = {}) {
  mockUseAuth.mockReturnValue({
    user: { user_id: 1, role: 'admin', name: 'Admin' },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    isAuthenticated: true,
    isLoading: false,
  })
  return render(
    <LogoCustomizer
      currentLogoUrl={null}
      onUpload={vi.fn()}
      onUrlSave={vi.fn()}
      onReset={vi.fn()}
      isSaving={false}
      {...props}
    />
  )
}

function renderNonAdmin(props: Partial<React.ComponentProps<typeof LogoCustomizer>> = {}) {
  mockUseAuth.mockReturnValue({
    user: { user_id: 2, role: 'user', name: 'Regular User' },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    isAuthenticated: true,
    isLoading: false,
  })
  return render(
    <LogoCustomizer
      currentLogoUrl={null}
      onUpload={vi.fn()}
      onUrlSave={vi.fn()}
      onReset={vi.fn()}
      isSaving={false}
      {...props}
    />
  )
}

describe('LogoCustomizer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // LC-01: Upload tab renders file input
  it('LC-01: renders file input on Upload tab for admin', () => {
    renderAdmin()
    const fileInput = screen.getByLabelText('Upload File')
    expect(fileInput).toBeInTheDocument()
    expect(fileInput).toHaveAttribute('type', 'file')
    expect(fileInput).toHaveAttribute('accept', 'image/png,image/jpeg,image/webp')
  })

  // LC-02: File too large shows error
  it('LC-02: shows error when file exceeds 2 MB', async () => {
    renderAdmin()

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    // Create a file larger than 2MB — size property simulated via Object.defineProperty
    const largeFile = new File(['x'.repeat(10)], 'logo.png', { type: 'image/png' })
    Object.defineProperty(largeFile, 'size', { value: 3 * 1024 * 1024, configurable: true })
    Object.defineProperty(fileInput, 'files', { value: [largeFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      // Error message matches the hint text (reused as error)
      expect(screen.getByText('PNG, JPG or WebP — max 2 MB')).toBeInTheDocument()
    })
  })

  // LC-03: Non-image file rejected
  it('LC-03: shows error when non-image file is selected', async () => {
    renderAdmin()

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const textFile = new File(['hello'], 'doc.txt', { type: 'text/plain' })
    Object.defineProperty(fileInput, 'files', { value: [textFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(screen.getByText('PNG, JPG or WebP — max 2 MB')).toBeInTheDocument()
    })
  })

  // LC-04: Valid file triggers onUpload callback
  it('LC-04: valid file triggers onUpload callback', async () => {
    const onUpload = vi.fn()
    renderAdmin({ onUpload })

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const validFile = new File([new Uint8Array(100)], 'logo.png', { type: 'image/png' })
    Object.defineProperty(fileInput, 'files', { value: [validFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(onUpload).toHaveBeenCalledWith(validFile)
    })
  })

  // LC-05: URL tab renders text input
  it('LC-05: URL tab renders URL input', async () => {
    renderAdmin()

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('https://example.com/logo.png')).toBeInTheDocument()
  })

  // LC-06: Valid URL triggers onUrlSave callback
  it('LC-06: valid URL triggers onUrlSave callback', async () => {
    const onUrlSave = vi.fn()
    renderAdmin({ onUrlSave })

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    const input = screen.getByPlaceholderText('https://example.com/logo.png')
    await userEvent.type(input, 'https://example.com/mylogo.png')

    const saveButton = screen.getByText('Save Logo')
    await userEvent.click(saveButton)

    expect(onUrlSave).toHaveBeenCalledWith('https://example.com/mylogo.png')
  })

  // LC-07: Reset button triggers onReset callback
  it('LC-07: reset button triggers onReset callback', async () => {
    const onReset = vi.fn()
    renderAdmin({ onReset })

    const resetButton = screen.getByText('Reset to Default')
    await userEvent.click(resetButton)

    expect(onReset).toHaveBeenCalled()
  })

  // LC-08: Non-admin user sees read-only notice and no file/URL inputs
  it('LC-08: non-admin sees admin access notice and no upload or URL inputs', () => {
    renderNonAdmin()

    expect(screen.getByText('Logo customization requires admin access')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).not.toBeInTheDocument()
    expect(screen.queryByText('Reset to Default')).not.toBeInTheDocument()
  })

  // LC-09: Invalid URL (http://) shows error and does NOT call onUrlSave
  it('LC-09: rejects http:// URL with inline error and does NOT call onUrlSave', async () => {
    const onUrlSave = vi.fn()
    renderAdmin({ onUrlSave })

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    const input = screen.getByPlaceholderText('https://example.com/logo.png')
    await userEvent.type(input, 'http://example.com/logo.png')

    const saveButton = screen.getByText('Save Logo')
    await userEvent.click(saveButton)

    expect(onUrlSave).not.toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText('URL must start with https://')).toBeInTheDocument()
    })
  })

  // LC-10: Switching from URL tab back to Upload tab restores file input
  it('LC-10: switching from URL tab back to Upload tab shows file input', async () => {
    renderAdmin()

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)
    expect(screen.getByRole('textbox')).toBeInTheDocument()

    const uploadTab = screen.getByText('Upload File')
    await userEvent.click(uploadTab)

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).toBeInTheDocument()
  })

  // Shows current logo preview
  it('shows custom logo when currentLogoUrl is provided', () => {
    renderAdmin({ currentLogoUrl: '/uploads/logo.png' })
    const img = screen.getByAltText('Logo preview') as HTMLImageElement
    expect(img.src).toContain('/uploads/logo.png')
  })

  // Falls back to /logo.png
  it('shows default logo when currentLogoUrl is null', () => {
    renderAdmin({ currentLogoUrl: null })
    const img = screen.getByAltText('Logo preview') as HTMLImageElement
    expect(img.src).toContain('/logo.png')
  })
})
