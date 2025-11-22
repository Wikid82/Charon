import { describe, it, expect, vi } from 'vitest'
import { render, waitFor } from '@testing-library/react'
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
  it('calls checkUpdates on mount', async () => {
    vi.mocked(systemApi.checkUpdates).mockResolvedValue({
      available: false,
      latest_version: '1.0.0',
      changelog_url: '',
    })

    renderWithClient(<SystemStatus />)

    await waitFor(() => {
      expect(systemApi.checkUpdates).toHaveBeenCalled()
    })
  })
})
