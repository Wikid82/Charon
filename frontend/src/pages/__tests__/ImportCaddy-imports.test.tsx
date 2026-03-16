import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi } from 'vitest'

import ImportCaddy from '../ImportCaddy'

// Create a simple mock for useImport that returns the error state
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

describe('ImportCaddy - Import Detection Error Display', () => {
  it('displays error message when import directives detected', () => {
    // Mock the hook to return error state with imports
    mockUseImport.mockReturnValue({
      session: null,
      preview: null,
      loading: false,
      error: 'This Caddyfile contains import directives. Please use the multi-file import flow to upload all referenced files together.',
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Check main error message is displayed
    expect(screen.getByText(/this caddyfile contains import directives/i)).toBeInTheDocument()

    // Check multi-site import button is available as alternative
    const multiSiteButton = screen.getByTestId('multi-file-import-button')
    expect(multiSiteButton).toBeInTheDocument()
  })

  it('displays plain error when no imports detected', () => {
    // Mock the hook to return error without imports
    mockUseImport.mockReturnValue({
      session: null,
      preview: null,
      loading: false,
      error: 'no sites found in uploaded Caddyfile',
      commitSuccess: false,
      commitResult: null,
      clearCommitResult: vi.fn(),
      upload: vi.fn(),
      commit: vi.fn(),
      cancel: vi.fn(),
    })

    render(<ImportCaddy />, { wrapper: createWrapper() })

    // Should show error message
    expect(screen.getByText('no sites found in uploaded Caddyfile')).toBeInTheDocument()
  })
})
