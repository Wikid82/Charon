import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
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
    createDomain: vi.fn().mockResolvedValue({ uuid: 'domain-1', name: 'existing.com' }),
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [
      { id: 1, name: 'Cert 1', domain: 'example.com', provider: 'custom', issuer: 'Custom', expires_at: '2026-01-01' }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useSecurity', () => ({
  useAuthPolicies: vi.fn(() => ({
    policies: [
      { id: 1, name: 'Admin Only', description: 'Requires admin role' }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../api/proxyHosts', () => ({
  testProxyHostConnection: vi.fn(),
}))

// Mock global fetch for health API
const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

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

const renderWithClientAct = async (ui: React.ReactElement) => {
  await act(async () => {
    renderWithClient(ui)
  })
}

import { testProxyHostConnection } from '../../api/proxyHosts'

describe('ProxyHostForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  beforeEach(() => {
    // Default fetch mock for health endpoint
    mockFetch.mockResolvedValue({
      json: () => Promise.resolve({ internal_ip: '192.168.1.50' }),
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('handles scheme selection', async () => {
    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })

    // Find scheme select - it defaults to HTTP
    // We can find it by label "Scheme"
    const schemeSelect = screen.getByLabelText('Scheme') as HTMLSelectElement
    await userEvent.selectOptions(schemeSelect, 'https')

    expect(schemeSelect).toHaveValue('https')
  })

  it('prompts to save new base domain', async () => {
    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const domainInput = screen.getByPlaceholderText('example.com, www.example.com')

    // Enter a subdomain of a new base domain
    await userEvent.type(domainInput, 'sub.newdomain.com')
    await userEvent.tab()

    await waitFor(() => {
      expect(screen.getByText('New Base Domain Detected')).toBeInTheDocument()
      expect(screen.getByText('newdomain.com')).toBeInTheDocument()
    })

    // Click "Yes, save it"
    await userEvent.click(screen.getByText('Yes, save it'))

    await waitFor(() => {
      expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
    })
  })

  it('respects "Dont ask me again" for new domains', async () => {
    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const domainInput = screen.getByPlaceholderText('example.com, www.example.com')

    // Trigger prompt
    await userEvent.type(domainInput, 'sub.another.com')
    await userEvent.tab()

    await waitFor(() => {
      expect(screen.getByText('New Base Domain Detected')).toBeInTheDocument()
    })

    // Check "Don't ask me again"
    await userEvent.click(screen.getByLabelText("Don't ask me again"))

    // Click "No, thanks"
    await userEvent.click(screen.getByText('No, thanks'))

    await waitFor(() => {
      expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
    })

    // Try another new domain - should not prompt
    await userEvent.type(domainInput, 'sub.yetanother.com')
    await userEvent.tab()

    // Should not see prompt
    expect(screen.queryByText('New Base Domain Detected')).not.toBeInTheDocument()
  })

  it('tests connection successfully', async () => {
    vi.mocked(testProxyHostConnection).mockResolvedValue(undefined)

    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    // Fill required fields for test connection
    await userEvent.type(screen.getByLabelText(/^Host$/), '10.0.0.5')
    await userEvent.clear(screen.getByLabelText(/^Port$/))
    await userEvent.type(screen.getByLabelText(/^Port$/), '80')

    const testBtn = screen.getByTitle('Test connection to the forward host')
    await userEvent.click(testBtn)

    await waitFor(() => {
      expect(testProxyHostConnection).toHaveBeenCalledWith('10.0.0.5', 80)
    })
  })

  it('handles connection test failure', async () => {
    vi.mocked(testProxyHostConnection).mockRejectedValue(new Error('Connection failed'))

    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await userEvent.type(screen.getByLabelText(/^Host$/), '10.0.0.5')
    await userEvent.clear(screen.getByLabelText(/^Port$/))
    await userEvent.type(screen.getByLabelText(/^Port$/), '80')

    const testBtn = screen.getByTitle('Test connection to the forward host')
    await userEvent.click(testBtn)

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
    await renderWithClientAct(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByLabelText('Base Domain (Auto-fill)')).toBeInTheDocument()
    })

    await userEvent.selectOptions(screen.getByLabelText('Base Domain (Auto-fill)') as HTMLSelectElement, 'existing.com')

    // Should not update domain names yet as no container selected
    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('')

    // Select container then base domain
    await userEvent.selectOptions(screen.getByLabelText('Source') as HTMLSelectElement, 'local')
    await userEvent.selectOptions(screen.getByLabelText('Containers') as HTMLSelectElement, 'container-123')
    await userEvent.selectOptions(screen.getByLabelText('Base Domain (Auto-fill)') as HTMLSelectElement, 'existing.com')

    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('my-app.existing.com')
  })

  // Application Preset Tests
  describe('Application Presets', () => {
    it('renders application preset dropdown with all options', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const presetSelect = screen.getByLabelText(/Application Preset/i)
      expect(presetSelect).toBeInTheDocument()

      // Check that all presets are available
      expect(screen.getByText('None - Standard reverse proxy')).toBeInTheDocument()
      expect(screen.getByText('Plex - Media server with remote access')).toBeInTheDocument()
      expect(screen.getByText('Jellyfin - Open source media server')).toBeInTheDocument()
      expect(screen.getByText('Emby - Media server')).toBeInTheDocument()
      expect(screen.getByText('Home Assistant - Home automation')).toBeInTheDocument()
      expect(screen.getByText('Nextcloud - File sync and share')).toBeInTheDocument()
      expect(screen.getByText('Vaultwarden - Password manager')).toBeInTheDocument()
    })

    it('defaults to none preset', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const presetSelect = screen.getByLabelText(/Application Preset/i)
      expect(presetSelect).toHaveValue('none')
    })

    it('enables websockets when selecting plex preset', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // First uncheck websockets
      const websocketCheckbox = screen.getByLabelText(/Websockets Support/i)
      if (websocketCheckbox.getAttribute('checked') !== null) {
        await userEvent.click(websocketCheckbox)
      }

      // Select Plex preset
      await act(async () => {
        await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')
      })

      // Websockets should be enabled
      expect(screen.getByLabelText(/Websockets Support/i)).toBeChecked()
    })

    it('shows plex config helper with external URL when preset is selected', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'plex.mydomain.com')

      // Select Plex preset
      await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')

      // Should show the helper with external URL
      await waitFor(() => {
        expect(screen.getByText('Plex Remote Access Setup')).toBeInTheDocument()
        expect(screen.getByText('https://plex.mydomain.com:443')).toBeInTheDocument()
      })
    })

    it('shows jellyfin config helper with internal IP', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'jellyfin.mydomain.com')

      // Select Jellyfin preset
      await act(async () => {
        await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'jellyfin')
      })

      // Wait for health API fetch and show helper
      await waitFor(() => {
        expect(screen.getByText('Jellyfin Proxy Setup')).toBeInTheDocument()
        expect(screen.getByText('192.168.1.50')).toBeInTheDocument()
      })
    })

    it('shows home assistant config helper with yaml snippet', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'ha.mydomain.com')

      // Select Home Assistant preset
      await act(async () => {
        await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'homeassistant')
      })

      // Wait for health API fetch and show helper
      await waitFor(() => {
        expect(screen.getByText('Home Assistant Proxy Setup')).toBeInTheDocument()
        expect(screen.getByText(/use_x_forwarded_for/)).toBeInTheDocument()
        expect(screen.getByText(/192\.168\.1\.50/)).toBeInTheDocument()
      })
    })

    it('shows nextcloud config helper with php snippet', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'nextcloud.mydomain.com')

      // Select Nextcloud preset
      await act(async () => {
        await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'nextcloud')
      })

      // Wait for health API fetch and show helper
      await waitFor(() => {
        expect(screen.getByText('Nextcloud Proxy Setup')).toBeInTheDocument()
        expect(screen.getByText(/trusted_proxies/)).toBeInTheDocument()
        expect(screen.getByText(/overwriteprotocol/)).toBeInTheDocument()
      })
    })

    it('shows vaultwarden helper text', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'vault.mydomain.com')

      // Select Vaultwarden preset
      await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'vaultwarden')

      // Wait for helper text
      await waitFor(() => {
        expect(screen.getByText('Vaultwarden Setup')).toBeInTheDocument()
        expect(screen.getByText(/WebSocket support is enabled automatically/)).toBeInTheDocument()
        expect(screen.getByText('vault.mydomain.com')).toBeInTheDocument()
      })
    })

    it('auto-detects plex preset from container image', async () => {
      // Mock useDocker to return a Plex container
      const { useDocker } = await import('../../hooks/useDocker')
      vi.mocked(useDocker).mockReturnValue({
        containers: [
          {
            id: 'plex-container',
            names: ['plex'],
            image: 'linuxserver/plex:latest',
            state: 'running',
            status: 'Up 1 hour',
            network: 'bridge',
            ip: '172.17.0.3',
            ports: [{ private_port: 32400, public_port: 32400, type: 'tcp' }]
          }
        ],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select local source
      await userEvent.selectOptions(screen.getByLabelText('Source') as HTMLSelectElement, 'local')

      // Select the plex container
      await waitFor(() => {
        expect(screen.getByText('plex (linuxserver/plex:latest)')).toBeInTheDocument()
      })

      await userEvent.selectOptions(screen.getByLabelText('Containers') as HTMLSelectElement, 'plex-container')

      // The preset should be auto-detected as plex
      expect(screen.getByLabelText(/Application Preset/i)).toHaveValue('plex')
    })

    it('auto-populates advanced_config when selecting plex preset and field empty', async () => {
      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Ensure advanced config is empty
      const textarea = screen.getByLabelText(/Advanced Caddy Config/i)
      expect(textarea).toHaveValue('')

      // Select Plex preset
      await act(async () => {
        await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')
      })

      // Value should now be populated with a snippet containing X-Real-IP which is used by the preset
      await waitFor(() => {
        expect((screen.getByLabelText(/Advanced Caddy Config/i) as HTMLTextAreaElement).value).toContain('X-Real-IP')
      })
    })

    it('prompts to confirm overwrite when selecting preset and advanced_config is non-empty', async () => {
      const existingHost = {
        uuid: 'test-uuid',
        name: 'ConfTest',
        domain_names: 'test.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.2',
        forward_port: 8080,
        advanced_config: '{"handler":"headers","request":{"set":{"X-Test":"value"}}}',
        advanced_config_backup: '',
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: true,
        application: 'none' as const,
        locations: [],
        enabled: true,
        created_at: '2025-01-01',
        updated_at: '2025-01-01',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost as any} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select Plex preset (should prompt since advanced_config is non-empty)
      await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')

      await waitFor(() => {
        expect(screen.getByText('Confirm Preset Overwrite')).toBeInTheDocument()
      })

      // Click Overwrite
      await userEvent.click(screen.getByText('Overwrite'))

      // After overwrite, the textarea should contain the preset 'X-Real-IP' snippet
      await waitFor(() => {
        expect((screen.getByLabelText(/Advanced Caddy Config/i) as HTMLTextAreaElement).value).toContain('X-Real-IP')
      })
    })

    it('restores previous advanced_config from backup when clicking restore', async () => {
      const existingHost = {
        uuid: 'test-uuid',
        name: 'RestoreTest',
        domain_names: 'test.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.2',
        forward_port: 8080,
        advanced_config: '{"handler":"headers","request":{"set":{"X-Test":"value"}}}',
        advanced_config_backup: '{"handler":"headers","request":{"set":{"X-Prev":"backup"}}}',
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: true,
        application: 'none' as const,
        locations: [],
        enabled: true,
        created_at: '2025-01-01',
        updated_at: '2025-01-01',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost as any} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // The restore button should be visible
      const restoreBtn = await screen.findByText('Restore previous config')
      expect(restoreBtn).toBeInTheDocument()

      // Click restore and expect the textarea to have backup value
      await userEvent.click(restoreBtn)

      await waitFor(() => {
        expect((screen.getByLabelText(/Advanced Caddy Config/i) as HTMLTextAreaElement).value).toContain('X-Prev')
      })
    })

    it('includes application field in form submission', async () => {
      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill required fields
      await userEvent.type(screen.getByPlaceholderText('My Service'), 'My Plex Server')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'plex.test.com')
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port$/))
      await userEvent.type(screen.getByLabelText(/^Port$/), '32400')

      // Select Plex preset
      await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')

      // Submit form
      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(
          expect.objectContaining({
            application: 'plex',
            websocket_support: true,
          })
        )
      })
    })

    it('loads existing host application preset', async () => {
      const existingHost = {
        uuid: 'test-uuid',
        name: 'Existing Plex',
        domain_names: 'plex.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.100',
        forward_port: 32400,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: true,
        application: 'plex' as const,
        locations: [],
        enabled: true,
        created_at: '2025-01-01',
        updated_at: '2025-01-01',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // The preset should be pre-selected
      expect(screen.getByLabelText(/Application Preset/i)).toHaveValue('plex')

      // The config helper should be visible
      await waitFor(() => {
        expect(screen.getByText('Plex Remote Access Setup')).toBeInTheDocument()
      })
    })

    it('does not show config helper when preset is none', async () => {
      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await act(async () => {
        await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'test.mydomain.com')
      })

      // Preset defaults to none, so no helper should be shown
      expect(screen.queryByText('Plex Remote Access Setup')).not.toBeInTheDocument()
      expect(screen.queryByText('Jellyfin Proxy Setup')).not.toBeInTheDocument()
      expect(screen.queryByText('Home Assistant Proxy Setup')).not.toBeInTheDocument()
    })

    it('copies external URL to clipboard for plex', async () => {
      // Mock clipboard API
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      Object.assign(navigator, {
        clipboard: { writeText: mockWriteText },
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fill in domain names
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'plex.mydomain.com')

      // Select Plex preset
      await userEvent.selectOptions(screen.getByLabelText(/Application Preset/i) as HTMLSelectElement, 'plex')

      // Wait for helper to appear
      await waitFor(() => {
        expect(screen.getByText('Plex Remote Access Setup')).toBeInTheDocument()
      })

      // Click the copy button
      const copyButtons = screen.getAllByText('Copy')
      await userEvent.click(copyButtons[0])

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith('https://plex.mydomain.com:443')
        expect(screen.getByText('Copied!')).toBeInTheDocument()
      })
    })
  })
})
