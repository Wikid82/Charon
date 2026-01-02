import { QueryClient, QueryClientProvider, QueryClientConfig } from '@tanstack/react-query'
import { ReactNode } from 'react'
import { MemoryRouter, MemoryRouterProps } from 'react-router-dom'
import { render } from '@testing-library/react'

const defaultConfig: QueryClientConfig = {
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false },
    mutations: { retry: false },
  },
}

export const createTestQueryClient = (config: QueryClientConfig = defaultConfig) => new QueryClient(config)

interface RenderOptions {
  client?: QueryClient
  routeEntries?: MemoryRouterProps['initialEntries']
}

export const renderWithQueryClient = (ui: ReactNode, options: RenderOptions = {}) => {
  const queryClient = options.client ?? createTestQueryClient()
  const routeEntries = options.routeEntries ?? ['/']

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={routeEntries}>{children}</MemoryRouter>
    </QueryClientProvider>
  )

  return {
    queryClient,
    ...render(<>{ui}</>, { wrapper }),
  }
}
