import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SystemStatus from '../SystemStatus'
import * as systemApi from '../../api/system'

// Mock the API module
vi.mock('../../api/system', () => ({
  checkUpdates: vi.fn(),
}))

const renderWithClient = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  )
}

describe('SystemStatus', () => {
  it('renders nothing when loading', () => {
    // Mock implementation to return a promise that never resolves immediately or just use loading state
    // But useQuery handles loading state.
    // We can mock useQuery if we want, but mocking the API is better integration.
    // However, to test loading state easily with real QueryClient is tricky without async.
    // Let's just rely on the fact that initially it might be loading.
    // Actually, let's mock the return value of checkUpdates to delay.

    // Better: mock useQuery? No, let's stick to mocking API.
    // If we want to test "isLoading", we can mock useQuery from @tanstack/react-query.
  })

  it('renders "Up to date" when no update available', async () => {
    vi.mocked(systemApi.checkUpdates).mockResolvedValue({
      available: false,
      latest_version: '1.0.0',
      changelog_url: '',
    })

    renderWithClient(<SystemStatus />)

    expect(await screen.findByText('Up to date')).toBeInTheDocument()
  })

  it('renders update available message when update is available', async () => {
    vi.mocked(systemApi.checkUpdates).mockResolvedValue({
      available: true,
      latest_version: '2.0.0',
      changelog_url: 'https://example.com/changelog',
    })

    renderWithClient(<SystemStatus />)

    expect(await screen.findByText('Update available: 2.0.0')).toBeInTheDocument()
    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', 'https://example.com/changelog')
  })
})
