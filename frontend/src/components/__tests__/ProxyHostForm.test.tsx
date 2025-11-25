import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ProxyHostForm from '../ProxyHostForm'
import { mockRemoteServers } from '../../test/mockData'

// Mock the hooks
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: mockRemoteServers,
    isLoading: false,
    error: null,
    createRemoteServer: vi.fn(),
    updateRemoteServer: vi.fn(),
    deleteRemoteServer: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({
    containers: [
      {
        id: 'container-123',
        names: ['my-app'],
        image: 'nginx:latest',
        state: 'running',
        status: 'Up 2 hours',
        network: 'bridge',
        ip: '172.17.0.2',
        ports: [{ private_port: 80, public_port: 8080, type: 'tcp' }]
      }
    ],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({
    domains: [
      { uuid: 'domain-1', name: 'existing.com' }
    ],
    createDomain: vi.fn().mockResolvedValue({}),
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [
      { id: 1, name: 'Cert 1', domain: 'example.com', provider: 'custom' }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../api/proxyHosts', () => ({
  testProxyHostConnection: vi.fn(),
}))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
})

const renderWithClient = (ui: React.ReactElement) => {
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  )
}

import { testProxyHostConnection } from '../../api/proxyHosts'

describe('ProxyHostForm', () => {
  const mockOnSubmit = vi.fn((_data: any) => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('handles scheme selection', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })

    // Find scheme select - it defaults to HTTP
    // We can find it by label "Scheme"
    const schemeSelect = screen.getByLabelText('Scheme')
    fireEvent.change(schemeSelect, { target: { value: 'https' } })

    expect(schemeSelect).toHaveValue('https')
  })

  it('prompts to save new base domain', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const domainInput = screen.getByPlaceholderText('example.com, www.example.com')

    // Enter a subdomain of a new base domain
    fireEvent.change(domainInput, { target: { value: 'sub.newdomain.com' } })
    fireEvent.blur(domainInput)

    await waitFor(() => {
      expect(screen.getByText('New Base Domain Detected')).toBeInTheDocument()
      expect(screen.getByText('newdomain.com')).toBeInTheDocument()
    })

    // Click "Yes, save it"
    fireEvent.click(screen.getByText('Yes, save it'))

    await waitFor(() => {
      expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
    })
  })

  it('respects "Dont ask me again" for new domains', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const domainInput = screen.getByPlaceholderText('example.com, www.example.com')

    // Trigger prompt
    fireEvent.change(domainInput, { target: { value: 'sub.another.com' } })
    fireEvent.blur(domainInput)

    await waitFor(() => {
      expect(screen.getByText('New Base Domain Detected')).toBeInTheDocument()
    })

    // Check "Don't ask me again"
    fireEvent.click(screen.getByLabelText("Don't ask me again"))

    // Click "No, thanks"
    fireEvent.click(screen.getByText('No, thanks'))

    await waitFor(() => {
      expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
    })

    // Try another new domain - should not prompt
    fireEvent.change(domainInput, { target: { value: 'sub.yetanother.com' } })
    fireEvent.blur(domainInput)

    // Should not see prompt
    expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
  })

  it('tests connection successfully', async () => {
    (testProxyHostConnection as any).mockResolvedValue({})

    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    // Fill required fields for test connection
    fireEvent.change(screen.getByLabelText(/^Host$/), { target: { value: '10.0.0.5' } })
    fireEvent.change(screen.getByLabelText(/^Port$/), { target: { value: '80' } })

    const testBtn = screen.getByTitle('Test connection to the forward host')
    fireEvent.click(testBtn)

    await waitFor(() => {
      expect(testProxyHostConnection).toHaveBeenCalledWith('10.0.0.5', 80)
    })
  })

  it('handles connection test failure', async () => {
    (testProxyHostConnection as any).mockRejectedValue(new Error('Connection failed'))

    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    fireEvent.change(screen.getByLabelText(/^Host$/), { target: { value: '10.0.0.5' } })
    fireEvent.change(screen.getByLabelText(/^Port$/), { target: { value: '80' } })

    const testBtn = screen.getByTitle('Test connection to the forward host')
    fireEvent.click(testBtn)

    await waitFor(() => {
      expect(testProxyHostConnection).toHaveBeenCalled()
    })

    // Should show error state (red button) - we can check class or icon
    // The button changes class to bg-red-600
    await waitFor(() => {
       expect(testBtn).toHaveClass('bg-red-600')
    })
  })

  it('handles base domain selection', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByLabelText('Base Domain (Auto-fill)')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByLabelText('Base Domain (Auto-fill)'), { target: { value: 'existing.com' } })

    // Should not update domain names yet as no container selected
    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('')

    // Select container then base domain
    fireEvent.change(screen.getByLabelText('Containers'), { target: { value: 'container-123' } })
    fireEvent.change(screen.getByLabelText('Base Domain (Auto-fill)'), { target: { value: 'existing.com' } })

    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('my-app.existing.com')
  })

  it('toggles forward auth fields', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const toggle = screen.getByLabelText('Enable Forward Auth (SSO)')
    expect(toggle).not.toBeChecked()

    // Bypass field should not be visible initially
    expect(screen.queryByLabelText('Bypass Paths (Optional)')).not.toBeInTheDocument()

    // Enable it
    fireEvent.click(toggle)
    expect(toggle).toBeChecked()

    // Bypass field should now be visible
    expect(screen.getByLabelText('Bypass Paths (Optional)')).toBeInTheDocument()
  })
})
