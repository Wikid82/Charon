import { render, screen, fireEvent, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemeProvider } from '../../../context/ThemeContext'
import { ThemeImportExport } from '../ThemeImportExport'

import type { ThemeExport } from '../../../context/ThemeContextValue'

// ThemeProvider calls useUserThemes internally; stub it so tests don't need QueryClientProvider
vi.mock('../../../hooks/useUserThemes', () => ({
  useUserThemes: vi.fn().mockReturnValue({
    userThemes: [],
    isLoading: false,
    error: null,
    createTheme: vi.fn(),
    updateTheme: vi.fn(),
    deleteTheme: vi.fn(),
    isCreating: false,
    isUpdating: false,
    isDeleting: false,
  }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'appearance.exportButton': 'Export Theme',
        'appearance.importButton': 'Import Theme',
        'appearance.importError': 'Invalid theme file',
      }
      return map[key] ?? key
    },
  }),
}))

// Use vi.hoisted so the mock fn is accessible inside the vi.mock factory (which is hoisted)
const { mockToastError } = vi.hoisted(() => ({ mockToastError: vi.fn() }))

vi.mock('../../../utils/toast', () => ({
  toast: {
    error: mockToastError,
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  },
}))

function renderThemeImportExport() {
  return render(
    <ThemeProvider>
      <ThemeImportExport />
    </ThemeProvider>
  )
}

const validThemeExport: ThemeExport = {
  version: 1,
  exportedAt: '2026-06-20T00:00:00.000Z',
  theme: 'dark',
}

const validCustomThemeExport: ThemeExport = {
  version: 1,
  exportedAt: '2026-06-20T00:00:00.000Z',
  theme: 'custom',
  customTheme: {
    name: 'My Theme',
    colors: {
      bgBase: '15 23 42',
      bgSubtle: '30 41 59',
      bgMuted: '51 65 85',
      bgElevated: '30 41 59',
      borderDefault: '51 65 85',
      borderStrong: '71 85 105',
      textPrimary: '248 250 252',
      textSecondary: '203 213 225',
      textMuted: '148 163 184',
      brandPrimary: '59 130 246',
      colorScheme: 'dark',
    },
  },
}

/**
 * Simulates the FileReader async read cycle by:
 * 1. Mocking window.FileReader to capture the onload callback
 * 2. Firing a change event on the input
 * 3. Synchronously invoking the captured onload with the given text
 */
function simulateFileRead(fileInput: HTMLInputElement, text: string) {
  let capturedOnload: ((e: { target: { result: string } }) => void) | null = null

  class MockFileReader {
    onload: ((e: { target: { result: string } }) => void) | null = null
    readAsText(_file: File) {
      capturedOnload = this.onload
    }
  }

  const OriginalFileReader = window.FileReader
  // The mock replaces the class constructor — this is intentional for testing

  ;(window as any).FileReader = MockFileReader

  const file = new File([text], 'theme.json', { type: 'application/json' })
  fireEvent.change(fileInput, { target: { files: [file] } })

  // Invoke the onload callback synchronously
  act(() => {
    if (capturedOnload) {
      capturedOnload({ target: { result: text } })
    }
  })


  ;(window as any).FileReader = OriginalFileReader
}

describe('ThemeImportExport', () => {
  let originalCreateObjectURL: typeof URL.createObjectURL
  let originalRevokeObjectURL: typeof URL.revokeObjectURL
  const mockCreateObjectURL = vi.fn(() => 'blob:mock-url')
  const mockRevokeObjectURL = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockToastError.mockClear()
    originalCreateObjectURL = URL.createObjectURL
    originalRevokeObjectURL = URL.revokeObjectURL
    URL.createObjectURL = mockCreateObjectURL as unknown as typeof URL.createObjectURL
    URL.revokeObjectURL = mockRevokeObjectURL as unknown as typeof URL.revokeObjectURL
    localStorage.clear()
  })

  afterEach(() => {
    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
    localStorage.clear()
  })

  it('renders export and import buttons', () => {
    renderThemeImportExport()
    expect(screen.getByRole('button', { name: /export theme/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /import theme/i })).toBeInTheDocument()
  })

  it('renders a hidden file input with .json accept', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement
    expect(fileInput.accept).toBe('.json')
  })

  it('clicking import button triggers file input click', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement
    const fileInputClickSpy = vi.spyOn(fileInput, 'click')

    const importBtn = screen.getByRole('button', { name: /import theme/i })
    fireEvent.click(importBtn)

    expect(fileInputClickSpy).toHaveBeenCalled()
  })

  // IE-01: Export button creates downloadable JSON
  it('IE-01: export button calls URL.createObjectURL and triggers download', () => {
    const anchorClickSpy = vi.fn()
    const originalCreateElement = document.createElement.bind(document)
    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = originalCreateElement(tag)
      if (tag === 'a') {
        Object.defineProperty(el, 'click', { value: anchorClickSpy, configurable: true })
      }
      return el
    })

    renderThemeImportExport()
    const exportBtn = screen.getByRole('button', { name: /export theme/i })
    fireEvent.click(exportBtn)

    expect(mockCreateObjectURL).toHaveBeenCalledExactlyOnceWith(expect.any(Blob))
    expect(anchorClickSpy).toHaveBeenCalledOnce()

    createElementSpy.mockRestore()
  })

  it('IE-01: exported file has charon-theme.json as the download attribute', () => {
    const createdAnchors: HTMLAnchorElement[] = []
    const anchorClickSpy = vi.fn()
    const originalCreateElement = document.createElement.bind(document)
    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = originalCreateElement(tag)
      if (tag === 'a') {
        createdAnchors.push(el as HTMLAnchorElement)
        Object.defineProperty(el, 'click', { value: anchorClickSpy, configurable: true })
      }
      return el
    })

    renderThemeImportExport()
    const exportBtn = screen.getByRole('button', { name: /export theme/i })
    fireEvent.click(exportBtn)

    expect(createdAnchors[0]?.download).toBe('charon-theme.json')
    createElementSpy.mockRestore()
  })

  // IE-02: Import with valid JSON calls importTheme
  it('IE-02: importing valid JSON succeeds (no error toast)', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    simulateFileRead(fileInput, JSON.stringify(validThemeExport))

    expect(mockToastError).not.toHaveBeenCalled()
  })

  it('IE-02: importing valid custom theme JSON succeeds (no error toast)', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    simulateFileRead(fileInput, JSON.stringify(validCustomThemeExport))

    expect(mockToastError).not.toHaveBeenCalled()
  })

  // IE-03: Import with invalid JSON shows error
  it('IE-03: importing invalid JSON (not parseable) shows error toast', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    simulateFileRead(fileInput, 'not valid json {{{')

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })

  // IE-04: Import with missing version field shows error
  it('IE-04: importing JSON with missing version shows error toast', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    const noVersion = { exportedAt: '2026-06-20T00:00:00.000Z', theme: 'dark' }
    simulateFileRead(fileInput, JSON.stringify(noVersion))

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })

  // IE-05: Import with wrong version shows error
  it('IE-05: importing JSON with version !== 1 shows error toast', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    const wrongVersion = { version: 2, exportedAt: '2026-06-20T00:00:00.000Z', theme: 'dark' }
    simulateFileRead(fileInput, JSON.stringify(wrongVersion))

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })

  // IE-06: Import with malformed color field (CSS injection attempt) is rejected
  it('IE-06: importing JSON with CSS injection in bgBase is rejected', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    const injectionAttempt = {
      version: 1,
      exportedAt: '2026-06-20T00:00:00.000Z',
      theme: 'custom',
      customTheme: {
        name: 'Evil Theme',
        colors: {
          bgBase: 'red; --injected: val',
          bgSubtle: '30 41 59',
          bgMuted: '51 65 85',
          bgElevated: '30 41 59',
          borderDefault: '51 65 85',
          borderStrong: '71 85 105',
          textPrimary: '248 250 252',
          textSecondary: '203 213 225',
          textMuted: '148 163 184',
          brandPrimary: '59 130 246',
          colorScheme: 'dark',
        },
      },
    }

    simulateFileRead(fileInput, JSON.stringify(injectionAttempt))

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })

  it('IE-06: unknown theme id in theme field is rejected', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    const unknownTheme = {
      version: 1,
      exportedAt: '2026-06-20T00:00:00.000Z',
      theme: 'evil-theme',
    }
    simulateFileRead(fileInput, JSON.stringify(unknownTheme))

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })

  it('exports valid JSON content including version and theme', () => {
    let blobContent = ''
    const originalBlob = window.Blob
    window.Blob = class MockBlob extends originalBlob {
      constructor(parts?: BlobPart[], options?: BlobPropertyBag) {
        super(parts, options)
        if (parts && parts[0]) blobContent = parts[0] as string
      }
    }

    const anchorClickSpy = vi.fn()
    const originalCreateElement = document.createElement.bind(document)
    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = originalCreateElement(tag)
      if (tag === 'a') {
        Object.defineProperty(el, 'click', { value: anchorClickSpy, configurable: true })
      }
      return el
    })

    renderThemeImportExport()
    const exportBtn = screen.getByRole('button', { name: /export theme/i })
    fireEvent.click(exportBtn)

    const parsed = JSON.parse(blobContent) as ThemeExport
    expect(parsed.version).toBe(1)
    expect(parsed.exportedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/)
    expect(typeof parsed.theme).toBe('string')

    createElementSpy.mockRestore()
    window.Blob = originalBlob
  })

  it('does not call toast.error when no file is selected', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    fireEvent.change(fileInput, { target: { files: [] } })

    expect(mockToastError).not.toHaveBeenCalled()
  })

  it('IE-06: null color field value is rejected', () => {
    renderThemeImportExport()
    const fileInput = screen.getByTestId('theme-file-input') as HTMLInputElement

    const nullColorField = {
      version: 1,
      exportedAt: '2026-06-20T00:00:00.000Z',
      theme: 'custom',
      customTheme: {
        name: 'Bad Theme',
        colors: {
          bgBase: null,
          bgSubtle: '30 41 59',
          bgMuted: '51 65 85',
          bgElevated: '30 41 59',
          borderDefault: '51 65 85',
          borderStrong: '71 85 105',
          textPrimary: '248 250 252',
          textSecondary: '203 213 225',
          textMuted: '148 163 184',
          brandPrimary: '59 130 246',
          colorScheme: 'dark',
        },
      },
    }
    simulateFileRead(fileInput, JSON.stringify(nullColorField))

    expect(mockToastError).toHaveBeenCalledWith('Invalid theme file')
  })
})
