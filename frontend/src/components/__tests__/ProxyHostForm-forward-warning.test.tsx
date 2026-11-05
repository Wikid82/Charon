import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import ProxyHostForm from '../ProxyHostForm'

vi.mock('../../api/uptime', () => ({
  syncMonitors: vi.fn(() => Promise.resolve({})),
}))

// Minimal hook mocks used by the component
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: [],
    isLoading: false,
    error: null,
    createRemoteServer: vi.fn(),
    updateRemoteServer: vi.fn(),
    deleteRemoteServer: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({ containers: [], isLoading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({ domains: [], createDomain: vi.fn().mockResolvedValue({}), isLoading: false, error: null })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}))

// stub global fetch for health endpoint
vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ json: () => Promise.resolve({ internal_ip: '127.0.0.1' }) })))

describe('ProxyHostForm forward host IP warnings', () => {
  const renderForm = () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <ProxyHostForm onSubmit={vi.fn(() => Promise.resolve())} onCancel={vi.fn()} />
      </QueryClientProvider>
    )
  }

  it('warns when the forward host is a Docker bridge container IP', async () => {
    renderForm()
    const hostInput = screen.getByLabelText(/^Host$/)
    await userEvent.type(hostInput, '172.17.0.5')

    expect(await screen.findByText(/looks like a Docker container IP address/i)).toBeInTheDocument()
    expect(screen.queryByText(/Using a private IP address/i)).not.toBeInTheDocument()
  })

  it('warns when the forward host is a private IP address', async () => {
    renderForm()
    const hostInput = screen.getByLabelText(/^Host$/)
    await userEvent.type(hostInput, '192.168.1.10')

    expect(await screen.findByText(/Using a private IP address/i)).toBeInTheDocument()
    expect(screen.queryByText(/looks like a Docker container IP address/i)).not.toBeInTheDocument()
  })

  it('shows no IP warning for a hostname', async () => {
    renderForm()
    const hostInput = screen.getByLabelText(/^Host$/)
    await userEvent.type(hostInput, 'my-app')

    expect(screen.queryByText(/looks like a Docker container IP address/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Using a private IP address/i)).not.toBeInTheDocument()
  })

  it('clears the warning when the host changes to a public address', async () => {
    renderForm()
    const hostInput = screen.getByLabelText(/^Host$/)
    await userEvent.type(hostInput, '10.0.0.4')
    expect(await screen.findByText(/Using a private IP address/i)).toBeInTheDocument()

    await userEvent.clear(hostInput)
    await userEvent.type(hostInput, 'my-app')
    expect(screen.queryByText(/Using a private IP address/i)).not.toBeInTheDocument()
  })
})
