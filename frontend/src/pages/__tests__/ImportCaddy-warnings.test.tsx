import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi } from 'vitest'

import ImportCaddy from '../ImportCaddy'

// Create a simple mock for useImport that returns the preview state
const mockUseImport = vi.fn()

// Mock the hooks
vi.mock('../../hooks/useImport', () => ({
  useImport: () => mockUseImport(),
}))

// Mock translation
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  return ({ children }: { children: React.ReactNode }) => (
    <BrowserRouter>
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    </BrowserRouter>
  )
}

describe('ImportCaddy - Warning Display', () => {
  it('displays empty file warning when session exists but no hosts found', () => {
    // Mock the hook to return session with empty hosts
    mockUseImport.mockReturnValue({
      session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
      preview: {
        session: { id: 'test-session', state: 'reviewing' },
        preview: {
          hosts: [],
          conflicts: [],
          errors: [],
        },
      },
      loading: false,
      error: null,
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check empty file warning is displayed
    expect(screen.getByText('importCaddy.noDomainsFound')).toBeInTheDocument()
    expect(screen.getByText('importCaddy.emptyFileWarning')).toBeInTheDocument()
  })

  it('displays import banner when session exists', () => {
    // Mock the hook to return session with hosts
    mockUseImport.mockReturnValue({
      session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
      preview: {
        session: { id: 'test-session', state: 'reviewing' },
        preview: {
          hosts: [{ domain_names: 'example.com' }],
          conflicts: [],
          errors: [],
        },
      },
      loading: false,
      error: null,
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check import banner is visible
    expect(screen.getByTestId('import-banner')).toBeInTheDocument()
  })

  it('does not display empty file warning when hosts exist', () => {
    // Mock the hook to return session with hosts
    mockUseImport.mockReturnValue({
      session: { id: 'test-session', state: 'reviewing', created_at: '', updated_at: '' },
      preview: {
        session: { id: 'test-session', state: 'reviewing' },
        preview: {
          hosts: [{ domain_names: 'example.com' }],
          conflicts: [],
          errors: [],
        },
      },
      loading: false,
      error: null,
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check empty file warning is NOT visible
    expect(screen.queryByText('importCaddy.noDomainsFound')).not.toBeInTheDocument()
  })

  it('does not display import banner when no session exists', () => {
    // Mock the hook to return null session
    mockUseImport.mockReturnValue({
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
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check import banner is NOT visible
    expect(screen.queryByTestId('import-banner')).not.toBeInTheDocument()
  })

  it('displays error message when error exists', () => {
    // Mock the hook to return error state
    mockUseImport.mockReturnValue({
      session: null,
      preview: null,
      loading: false,
      error: 'Failed to parse Caddyfile',
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check error message is displayed
    expect(screen.getByText('Failed to parse Caddyfile')).toBeInTheDocument()
  })

  it('shows upload form when no session exists', () => {
    // Mock the hook to return null session
    mockUseImport.mockReturnValue({
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
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check upload form elements are visible
    expect(screen.getByTestId('import-dropzone')).toBeInTheDocument()
    expect(screen.getByTestId('multi-file-import-button')).toBeInTheDocument()
  })
})
