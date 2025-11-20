import { ReactNode } from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Layout from '../Layout'
import { ThemeProvider } from '../../context/ThemeContext'

// Mock AuthContext
vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({
    logout: vi.fn(),
  }),
}))

// Mock API
vi.mock('../../api/health', () => ({
  checkHealth: vi.fn().mockResolvedValue({
    version: '0.1.0',
    git_commit: 'abcdef1',
  }),
}))

const renderWithProviders = (children: ReactNode) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ThemeProvider>
          {children}
        </ThemeProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('Layout', () => {
  it('renders the application title', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getAllByText('CPM+')[0]).toBeInTheDocument()
  })

  it('renders all navigation items', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Certificates')).toBeInTheDocument()
    expect(screen.getByText('Import Caddyfile')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders children content', () => {
    renderWithProviders(
      <Layout>
        <div data-testid="test-content">Test Content</div>
      </Layout>
    )

    expect(screen.getByTestId('test-content')).toBeInTheDocument()
  })

  it('displays version information', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(await screen.findByText('Version 0.1.0')).toBeInTheDocument()
  })
})
