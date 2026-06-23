import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { BannerCustomizer } from '../BannerCustomizer'

// Mock useAuth hook
vi.mock('../../../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../../../hooks/useAuth'

const mockUseAuth = vi.mocked(useAuth)

function renderAdmin(props: Partial<React.ComponentProps<typeof BannerCustomizer>> = {}) {
  mockUseAuth.mockReturnValue({
    user: { user_id: 1, role: 'admin', name: 'Admin' },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    isAuthenticated: true,
    isLoading: false,
  })
  return render(
    <BannerCustomizer
      currentBannerUrl={null}
      onUpload={vi.fn()}
      onUrlSave={vi.fn()}
      onReset={vi.fn()}
      isSaving={false}
      {...props}
    />
  )
}

function renderNonAdmin(props: Partial<React.ComponentProps<typeof BannerCustomizer>> = {}) {
  mockUseAuth.mockReturnValue({
    user: { user_id: 2, role: 'user', name: 'Regular User' },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    isAuthenticated: true,
    isLoading: false,
  })
  return render(
    <BannerCustomizer
      currentBannerUrl={null}
      onUpload={vi.fn()}
      onUrlSave={vi.fn()}
      onReset={vi.fn()}
      isSaving={false}
      {...props}
    />
  )
}

describe('BannerCustomizer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // BC-01: Upload tab renders file input with id="banner-file-input"
  it('BC-01: renders file input with id="banner-file-input" for admin', () => {
    renderAdmin()
    const fileInput = document.getElementById('banner-file-input') as HTMLInputElement
    expect(fileInput).toBeInTheDocument()
    expect(fileInput).toHaveAttribute('type', 'file')
    expect(fileInput).toHaveAttribute('accept', 'image/png,image/jpeg,image/webp')
  })

  // BC-02: File too large shows error
  it('BC-02: shows error when file exceeds 2 MB', async () => {
    renderAdmin()

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const largeFile = new File(['x'.repeat(10)], 'banner.png', { type: 'image/png' })
    Object.defineProperty(largeFile, 'size', { value: 3 * 1024 * 1024, configurable: true })
    Object.defineProperty(fileInput, 'files', { value: [largeFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(screen.getByText('PNG, JPG or WebP — max 2 MB')).toBeInTheDocument()
    })
  })

  // BC-03: Non-image file rejected
  it('BC-03: shows error when non-image file is selected', async () => {
    renderAdmin()

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const textFile = new File(['hello'], 'doc.txt', { type: 'text/plain' })
    Object.defineProperty(fileInput, 'files', { value: [textFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(screen.getByText('PNG, JPG or WebP — max 2 MB')).toBeInTheDocument()
    })
  })

  // BC-04: Valid file triggers onUpload callback
  it('BC-04: valid file triggers onUpload callback', async () => {
    const onUpload = vi.fn()
    renderAdmin({ onUpload })

    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const validFile = new File([new Uint8Array(100)], 'banner.png', { type: 'image/png' })
    Object.defineProperty(fileInput, 'files', { value: [validFile], configurable: true })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(onUpload).toHaveBeenCalledWith(validFile)
    })
  })

  // BC-05: URL tab renders
  it('BC-05: URL tab renders URL input', async () => {
    renderAdmin()

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('https://example.com/banner.png')).toBeInTheDocument()
  })

  // BC-06: http:// URL is rejected with inline error (critical security test)
  it('BC-06: rejects http:// URL with inline error and does NOT call onUrlSave', async () => {
    const onUrlSave = vi.fn()
    renderAdmin({ onUrlSave })

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    const input = screen.getByPlaceholderText('https://example.com/banner.png')
    await userEvent.type(input, 'http://example.com/banner.png')

    const saveButton = screen.getByText('Save Banner')
    await userEvent.click(saveButton)

    expect(onUrlSave).not.toHaveBeenCalled()
    expect(screen.getByText('URL must start with https://')).toBeInTheDocument()
  })

  // BC-07: https:// URL calls onUrlSave
  it('BC-07: valid https:// URL calls onUrlSave', async () => {
    const onUrlSave = vi.fn()
    renderAdmin({ onUrlSave })

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    const input = screen.getByPlaceholderText('https://example.com/banner.png')
    await userEvent.type(input, 'https://example.com/banner.png')

    const saveButton = screen.getByText('Save Banner')
    await userEvent.click(saveButton)

    expect(onUrlSave).toHaveBeenCalledWith('https://example.com/banner.png')
    expect(screen.queryByText('URL must start with https://')).not.toBeInTheDocument()
  })

  // BC-08: Reset button calls onReset
  it('BC-08: reset button triggers onReset callback', async () => {
    const onReset = vi.fn()
    renderAdmin({ onReset })

    const resetButton = screen.getByText('Reset to Default')
    await userEvent.click(resetButton)

    expect(onReset).toHaveBeenCalled()
  })

  // BC-09: Non-admin user sees read-only notice and no file/URL inputs
  it('BC-09: non-admin sees admin access notice and no upload or URL inputs', () => {
    renderNonAdmin()

    expect(screen.getByText('Banner customization requires admin access')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).not.toBeInTheDocument()
    expect(screen.queryByText('Reset to Default')).not.toBeInTheDocument()
  })

  // BC-10: Shows banner preview when currentBannerUrl is provided (admin)
  it('BC-10: shows banner preview when currentBannerUrl is provided', () => {
    renderAdmin({ currentBannerUrl: '/uploads/banner.png' })
    const img = screen.getByAltText('Banner preview') as HTMLImageElement
    expect(img.src).toContain('/uploads/banner.png')
  })

  // BC-12: Switching from URL tab back to Upload tab restores file input
  it('BC-12: switching from URL tab back to Upload tab shows file input', async () => {
    renderAdmin()

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)
    expect(screen.getByRole('textbox')).toBeInTheDocument()

    const uploadTab = screen.getByText('Upload File')
    await userEvent.click(uploadTab)

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(document.querySelector('#banner-file-input')).toBeInTheDocument()
  })

  // BC-11: javascript: URL is rejected
  it('BC-11: rejects javascript: URL', async () => {
    const onUrlSave = vi.fn()
    renderAdmin({ onUrlSave })

    const urlTab = screen.getByText('Enter URL')
    await userEvent.click(urlTab)

    const input = screen.getByPlaceholderText('https://example.com/banner.png')
    await userEvent.type(input, 'javascript:alert(1)')

    const saveButton = screen.getByText('Save Banner')
    await userEvent.click(saveButton)

    expect(onUrlSave).not.toHaveBeenCalled()
    expect(screen.getByText('URL must start with https://')).toBeInTheDocument()
  })
})
