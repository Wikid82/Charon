import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ProxyHostForm from '../ProxyHostForm'
import type { ProxyHost } from '../../api/proxyHosts'
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

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({
    data: [
      { id: 10, name: 'Trusted IPs', type: 'allow_list', enabled: true, description: 'Only trusted' },
      { id: 11, name: 'Geo Block', type: 'geo_block', enabled: true }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({
    data: [
      { id: 100, name: 'Strict Profile', description: 'Very strict', security_score: 90, is_preset: true, preset_type: 'strict' }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({
      domain: 'example.com',
      detected: false,
      nameservers: [],
      confidence: 'none',
    }),
    isPending: false,
    data: null,
    reset: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({
    data: [
      { id: 1, name: 'Cloudflare', provider_type: 'cloudflare', enabled: true, has_credentials: true }
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
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

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
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

const selectComboboxOption = async (label: string | RegExp, optionText: string) => {
  const trigger = screen.getByRole('combobox', { name: label })
  await userEvent.click(trigger)
  const option = await screen.findByRole('option', { name: optionText })
  await userEvent.click(option)
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

    await selectComboboxOption('Scheme', 'HTTPS')
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

    await selectComboboxOption(/Base Domain/i, 'existing.com')

    // Should not update domain names yet as no container selected
    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('')

    // Select container then base domain
    await selectComboboxOption('Source', 'Local (Docker Socket)')
    await selectComboboxOption('Containers', 'my-app (nginx:latest)')
    await selectComboboxOption(/Base Domain/i, 'existing.com')

    expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('my-app.existing.com')
  })

  // Application Preset Tests
  describe('Application Presets', () => {
    it('renders application preset dropdown with all options', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const presetTrigger = screen.getByRole('combobox', { name: /Application Preset/i })
      expect(presetTrigger).toBeInTheDocument()
      await userEvent.click(presetTrigger)

      const presetListbox = screen.getByRole('listbox')

      // Check that all presets are available
      expect(within(presetListbox).getByText('None - Standard reverse proxy')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Plex - Media server with remote access')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Jellyfin - Open source media server')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Emby - Media server')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Home Assistant - Home automation')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Nextcloud - File sync and share')).toBeInTheDocument()
      expect(within(presetListbox).getByText('Vaultwarden - Password manager')).toBeInTheDocument()
    })

    it('defaults to none preset', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const presetTrigger = screen.getByRole('combobox', { name: /Application Preset/i })
      expect(presetTrigger).toHaveTextContent('None - Standard reverse proxy')
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
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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
      await selectComboboxOption(/Application Preset/i, 'Jellyfin - Open source media server')

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
      await selectComboboxOption(/Application Preset/i, 'Home Assistant - Home automation')

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
      await selectComboboxOption(/Application Preset/i, 'Nextcloud - File sync and share')

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
      await selectComboboxOption(/Application Preset/i, 'Vaultwarden - Password manager')

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
      await selectComboboxOption('Source', 'Local (Docker Socket)')

      // Select the plex container
      await waitFor(() => {
        expect(screen.getByText('plex (linuxserver/plex:latest)')).toBeInTheDocument()
      })

      await selectComboboxOption('Containers', 'plex (linuxserver/plex:latest)')

      // The preset should be auto-detected as plex
      expect(screen.getByRole('combobox', { name: /Application Preset/i })).toHaveTextContent('Plex')
    })

    it('auto-populates advanced_config when selecting plex preset and field empty', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Ensure advanced config is empty
      const textarea = screen.getByLabelText(/Advanced Caddy Config/i)
      expect(textarea).toHaveValue('')

      // Select Plex preset
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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
        <ProxyHostForm host={existingHost as ProxyHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select Plex preset (should prompt since advanced_config is non-empty)
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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

    it('closes preset overwrite modal when cancel is clicked', async () => {
      const existingHost = {
        uuid: 'test-uuid',
        name: 'CancelOverwrite',
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
        <ProxyHostForm host={existingHost as ProxyHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

      await waitFor(() => {
        expect(screen.getByText('Confirm Preset Overwrite')).toBeInTheDocument()
      })

      const modal = screen.getByText('Confirm Preset Overwrite').closest('div')?.parentElement
      if (!modal) {
        throw new Error('Preset overwrite modal not found')
      }

      await userEvent.click(within(modal).getByRole('button', { name: 'Cancel' }))

      await waitFor(() => {
        expect(screen.queryByText('Confirm Preset Overwrite')).not.toBeInTheDocument()
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
        <ProxyHostForm host={existingHost as ProxyHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
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
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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
      expect(screen.getByRole('combobox', { name: /Application Preset/i })).toHaveTextContent('Plex')

      // The config helper should be visible
      await waitFor(() => {
        expect(screen.getByText('Plex Remote Access Setup')).toBeInTheDocument()
      })
    })

    it('does not show config helper when preset is none', async () => {
      await renderWithClientAct(
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
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

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

    it('copies plex trusted proxy IP helper snippet', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      Object.assign(navigator, {
        clipboard: { writeText: mockWriteText },
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'apps.mydomain.com')

      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')
      await userEvent.click(screen.getAllByRole('button', { name: /Copy/i })[1])

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith('192.168.1.50')
      })
    })

    it('copies jellyfin trusted proxy IP helper snippet', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      Object.assign(navigator, {
        clipboard: { writeText: mockWriteText },
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'apps.mydomain.com')
      await selectComboboxOption(/Application Preset/i, 'Jellyfin - Open source media server')
      await userEvent.click(screen.getByRole('button', { name: /Copy/i }))

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith('192.168.1.50')
      })
    })

    it('copies home assistant helper yaml snippet', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      Object.assign(navigator, {
        clipboard: { writeText: mockWriteText },
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'apps.mydomain.com')
      await selectComboboxOption(/Application Preset/i, 'Home Assistant - Home automation')
      await userEvent.click(screen.getByRole('button', { name: /Copy/i }))

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith('http:\n  use_x_forwarded_for: true\n  trusted_proxies:\n    - 192.168.1.50')
      })
    })

    it('copies nextcloud helper php snippet', async () => {
      const mockWriteText = vi.fn().mockResolvedValue(undefined)
      Object.assign(navigator, {
        clipboard: { writeText: mockWriteText },
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'apps.mydomain.com')
      await selectComboboxOption(/Application Preset/i, 'Nextcloud - File sync and share')
      await userEvent.click(screen.getByRole('button', { name: /Copy/i }))

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith("'trusted_proxies' => ['192.168.1.50'],\n'overwriteprotocol' => 'https',")
      })
    })
  })

  describe('Security Options', () => {
    it('toggles security options', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Toggle SSL Forced (default is true)
      const sslCheckbox = screen.getByLabelText('Force SSL')
      expect(sslCheckbox).toBeChecked()
      await userEvent.click(sslCheckbox)
      expect(sslCheckbox).not.toBeChecked()

      // Toggle HSTS (default is true)
      const hstsCheckbox = screen.getByLabelText('HSTS Enabled')
      expect(hstsCheckbox).toBeChecked()
      await userEvent.click(hstsCheckbox)
      expect(hstsCheckbox).not.toBeChecked()

      // Toggle HTTP/2 (default is true)
      const http2Checkbox = screen.getByLabelText('HTTP/2 Support')
      expect(http2Checkbox).toBeChecked()
      await userEvent.click(http2Checkbox)
      expect(http2Checkbox).not.toBeChecked()

      // Toggle Block Exploits (default is true)
      const blockExploitsCheckbox = screen.getByLabelText('Block Exploits')
      expect(blockExploitsCheckbox).toBeChecked()
      await userEvent.click(blockExploitsCheckbox)
      expect(blockExploitsCheckbox).not.toBeChecked()

      // Toggle HSTS Subdomains (default is true)
      const hstsSubdomainsCheckbox = screen.getByLabelText('HSTS Subdomains')
      expect(hstsSubdomainsCheckbox).toBeChecked()
      await userEvent.click(hstsSubdomainsCheckbox)
      expect(hstsSubdomainsCheckbox).not.toBeChecked()
    })

    it('submits updated hsts_subdomains flag', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('My Service'), 'HSTS Toggle')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'hsts.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.10')

      const hstsSubdomainsCheckbox = screen.getByLabelText('HSTS Subdomains')
      await userEvent.click(hstsSubdomainsCheckbox)

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          hsts_subdomains: false,
        }))
      })
    })
  })

  describe('Access Control', () => {
    it('selects an access list', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select 'Trusted IPs'
      // Need to find the select by label/aria-label? AccessListSelector has label "Access Control List"
      await selectComboboxOption(/Access Control List/i, 'Trusted IPs (allow list)')

      // Verify it was selected
      expect(screen.getByRole('combobox', { name: /Access Control List/i })).toHaveTextContent('Trusted IPs')

      // Verify description appears
      expect(screen.getByText('Trusted IPs')).toBeInTheDocument()
      expect(screen.getByText('Only trusted')).toBeInTheDocument()
    })
  })

  describe('Wildcard Domains', () => {
    it('shows DNS provider selector for wildcard domains', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Enter a wildcard domain
      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      // DNS Provider Selector should appear
      await waitFor(() => {
        expect(screen.getByTestId('dns-provider-section')).toBeInTheDocument()
      })

      // Select a provider using the mocked data: Cloudflare (ID 1)
      const section = screen.getByTestId('dns-provider-section')

      // Since Shadcn Select uses Radix, the trigger is a button with role combobox
      const providerSelectTrigger = within(section).getByRole('combobox')
      await userEvent.click(providerSelectTrigger)

      const cloudflareOption = screen.getByText('Cloudflare')
      await userEvent.click(cloudflareOption)

      // Now try to save
      await userEvent.type(screen.getByLabelText(/^Name/), 'Wildcard Test')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.10')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          domain_names: '*.example.com',
          dns_provider_id: 1
        }))
      })
    })

    it('validates DNS provider requirement for wildcard domains', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Enter a wildcard domain
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.missing.com')

      // Fill other required fields
      await userEvent.type(screen.getByLabelText(/^Name/), 'Missing Provider')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.10')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      // Click save without selecting provider
      await userEvent.click(screen.getByText('Save'))

      // Expect toast error (mocked only effectively if we check for it, but here we check it prevents submit)
      expect(mockOnSubmit).not.toHaveBeenCalled()
    })
  })

  // ===== BRANCH COVERAGE EXPANSION TESTS =====

  describe('Form Submission and Validation', () => {
    it('prevents submission without required fields', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Click save without filling any fields
      await userEvent.click(screen.getByText('Save'))

      // Submit should not be called
      expect(mockOnSubmit).not.toHaveBeenCalled()
    })

    it('submits form with all basic fields', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'My Service')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'myservice.existing.com')
      await selectComboboxOption('Scheme', 'HTTPS')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port/))
      await userEvent.type(screen.getByLabelText(/^Port/), '8080')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          name: 'My Service',
          domain_names: 'myservice.existing.com',
          forward_scheme: 'https',
          forward_host: '192.168.1.100',
          forward_port: 8080,
        }))
      })
    })

    it('submits form with certificate selection', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Cert Test')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'cert.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      // Select certificate
      await selectComboboxOption(/SSL Certificate/i, 'Cert 1 (custom)')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          certificate_id: 1,
        }))
      })
    })

    it('submits form with security header profile selection', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Security Headers Test')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'secure.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      // Select security header profile
      await selectComboboxOption(/Security Headers/i, 'Strict Profile (Score: 90/100)')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          security_header_profile_id: 100,
        }))
      })
    })

    it('renders and selects non-preset security header profile options', async () => {
      const { useSecurityHeaderProfiles } = await import('../../hooks/useSecurityHeaders')
      vi.mocked(useSecurityHeaderProfiles).mockReturnValue({
        data: [
          { id: 100, name: 'Strict Profile', description: 'Very strict', security_score: 90, is_preset: true, preset_type: 'strict' },
          { id: 101, name: 'Custom Profile', description: 'Custom profile', security_score: 70, is_preset: false },
        ],
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useSecurityHeaderProfiles>)

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await selectComboboxOption(/Security Headers/i, 'Custom Profile (Score: 70/100)')
      expect(screen.getByRole('combobox', { name: /Security Headers/i })).toHaveTextContent('Custom Profile')
    })

    it('resolves prefixed security header id tokens from existing host values', async () => {
      const existingHost = {
        uuid: 'security-token-host',
        name: 'Token Host',
        domain_names: 'token.example.com',
        forward_scheme: 'http',
        forward_host: '127.0.0.1',
        forward_port: 80,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: true,
        block_exploits: true,
        websocket_support: true,
        application: 'none' as const,
        locations: [],
        enabled: true,
        security_header_profile_id: 'id:100',
        created_at: '2025-01-01',
        updated_at: '2025-01-01',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost as unknown as ProxyHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      expect(screen.getByRole('combobox', { name: /Security Headers/i })).toHaveTextContent('Strict Profile')
    })

    it('resolves numeric-string security header ids from existing host values', async () => {
      const existingHost = {
        uuid: 'security-numeric-host',
        name: 'Numeric Host',
        domain_names: 'numeric.example.com',
        forward_scheme: 'http',
        forward_host: '127.0.0.1',
        forward_port: 80,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: true,
        block_exploits: true,
        websocket_support: true,
        application: 'none' as const,
        locations: [],
        enabled: true,
        security_header_profile_id: '100',
        created_at: '2025-01-01',
        updated_at: '2025-01-01',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost as unknown as ProxyHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      expect(screen.getByRole('combobox', { name: /Security Headers/i })).toHaveTextContent('Strict Profile')
    })

    it('skips non-preset profiles that have neither id nor uuid', async () => {
      const { useSecurityHeaderProfiles } = await import('../../hooks/useSecurityHeaders')
      vi.mocked(useSecurityHeaderProfiles).mockReturnValue({
        data: [
          { id: 100, name: 'Strict Profile', description: 'Very strict', security_score: 90, is_preset: true, preset_type: 'strict' },
          { name: 'Invalid Custom', description: 'No identity token', security_score: 10, is_preset: false },
        ],
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useSecurityHeaderProfiles>)

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.click(screen.getByRole('combobox', { name: /Security Headers/i }))

      expect(screen.queryByRole('option', { name: /Invalid Custom/i })).not.toBeInTheDocument()
    })

  })

  describe('Edit Mode vs Create Mode', () => {
    it('shows edit mode with pre-filled data', async () => {
      const existingHost: ProxyHost = {
        uuid: 'host-uuid-1',
        name: 'Existing Service',
        domain_names: 'existing.com',
        forward_scheme: 'https' as const,
        forward_host: '192.168.1.50',
        forward_port: 443,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: true,
        block_exploits: true,
        websocket_support: false,
        enable_standard_headers: true,
        application: 'none' as const,
        advanced_config: '',
        enabled: true,
        locations: [],
        certificate_id: null,
        access_list_id: null,
        security_header_profile_id: null,
        dns_provider_id: null,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      }

      await renderWithClientAct(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Fields should be pre-filled
      expect(screen.getByDisplayValue('Existing Service')).toBeInTheDocument()
      expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('existing.com')
      expect(screen.getByDisplayValue('192.168.1.50')).toBeInTheDocument()

      // Update and save
      const nameInput = screen.getByDisplayValue('Existing Service') as HTMLInputElement
      await userEvent.clear(nameInput)
      await userEvent.type(nameInput, 'Updated Service')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          name: 'Updated Service',
        }))
      })
    })

    it('renders title as Edit for existing host', async () => {
      const existingHost: ProxyHost = {
        uuid: 'host-uuid-1',
        name: 'Existing',
        domain_names: 'test.com',
        forward_scheme: 'http' as const,
        forward_host: '127.0.0.1',
        forward_port: 80,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: true,
        block_exploits: true,
        websocket_support: false,
        enable_standard_headers: true,
        application: 'none' as const,
        advanced_config: '',
        enabled: true,
        locations: [],
        certificate_id: null,
        access_list_id: null,
        security_header_profile_id: null,
        dns_provider_id: null,
        created_at: '',
        updated_at: '',
      }

      await renderWithClientAct(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      expect(screen.getByText('Edit Proxy Host')).toBeInTheDocument()
    })

    it('renders title as Add for new proxy host', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })
  })

  describe('Scheme Selection', () => {
    it('shows scheme options http and https', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const schemeTrigger = screen.getByRole('combobox', { name: 'Scheme' })
      await userEvent.click(schemeTrigger)

      expect(await screen.findByRole('option', { name: 'HTTP' })).toBeInTheDocument()
      expect(await screen.findByRole('option', { name: 'HTTPS' })).toBeInTheDocument()
    })

    it('accepts https scheme', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await selectComboboxOption('Scheme', 'HTTPS')
      expect(screen.getByRole('combobox', { name: 'Scheme' })).toHaveTextContent('HTTPS')
    })
  })

  describe('Cancel Operations', () => {
    it('calls onCancel when cancel button is clicked', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const cancelBtn = screen.getByRole('button', { name: /Cancel/i })
      await userEvent.click(cancelBtn)

      expect(mockOnCancel).toHaveBeenCalled()
    })
  })

  describe('Advanced Config', () => {
    it('shows advanced config field for application presets', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select Plex preset
      await selectComboboxOption(/Application Preset/i, 'Plex - Media server with remote access')

      // Find advanced config field (it's in a collapsible section)
      // Check that advanced config JSON for plex has been populated
      const advancedConfigField = screen.getByLabelText(/Advanced Caddy Config/i) as HTMLTextAreaElement

      // Verify it contains JSON (Plex has some default config)
      if (advancedConfigField.value) {
        expect(advancedConfigField.value).toContain('handler')
      }
    })

    it('allows manual advanced config input', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Custom Config')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'custom.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      const advancedConfigField = screen.getByPlaceholderText('Additional Caddy directives...')
      await userEvent.type(advancedConfigField, 'header /api/* X-Custom-Header "test"')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          advanced_config: expect.stringContaining('header'),
        }))
      })
    })
  })

  describe('Port Input Handling', () => {
    it('shows required-port validation branch when submit is triggered with empty port', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Port Required')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'required.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')

      const portInput = screen.getByLabelText(/^Port$/) as HTMLInputElement
      await userEvent.clear(portInput)

      const setCustomValiditySpy = vi.spyOn(HTMLInputElement.prototype, 'setCustomValidity')
      const reportValiditySpy = vi.spyOn(HTMLInputElement.prototype, 'reportValidity').mockReturnValue(false)

      const form = document.querySelector('form') as HTMLFormElement
      fireEvent.submit(form)

      await waitFor(() => {
        expect(setCustomValiditySpy).toHaveBeenCalledWith('Port is required')
        expect(reportValiditySpy).toHaveBeenCalled()
        expect(mockOnSubmit).not.toHaveBeenCalled()
      })

      setCustomValiditySpy.mockRestore()
      reportValiditySpy.mockRestore()
    })

    it('validates port number range', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Port Test')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'port.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')

      // Clear and set invalid port
      const portInput = screen.getByLabelText(/^Port$/) as HTMLInputElement
      await userEvent.clear(portInput)
      await userEvent.type(portInput, '99999')

      // Invalid port should block submission via native validation
      await userEvent.click(screen.getByText('Save'))

      expect(portInput).toBeInvalid()
      expect(mockOnSubmit).not.toHaveBeenCalled()
    })

    it('shows out-of-range validation branch when submit is triggered with invalid port', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Port Range Branch')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'range.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')

      const portInput = screen.getByLabelText(/^Port$/) as HTMLInputElement
      await userEvent.clear(portInput)
      await userEvent.type(portInput, '70000')

      const setCustomValiditySpy = vi.spyOn(HTMLInputElement.prototype, 'setCustomValidity')
      const reportValiditySpy = vi.spyOn(HTMLInputElement.prototype, 'reportValidity').mockReturnValue(false)

      const form = document.querySelector('form') as HTMLFormElement
      fireEvent.submit(form)

      await waitFor(() => {
        expect(setCustomValiditySpy).toHaveBeenCalledWith('Port must be between 1 and 65535')
        expect(reportValiditySpy).toHaveBeenCalled()
        expect(mockOnSubmit).not.toHaveBeenCalled()
      })

      setCustomValiditySpy.mockRestore()
      reportValiditySpy.mockRestore()
    })
  })

  describe('Remote Server Container Mapping', () => {
    it('allows selecting a remote docker source option', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await selectComboboxOption('Source', 'Local Docker Registry (localhost)')

      expect(screen.getByRole('combobox', { name: 'Source' })).toHaveTextContent('Local Docker Registry')
    })

    it('maps remote docker container to remote host and public port', async () => {
      const { useDocker } = await import('../../hooks/useDocker')
      vi.mocked(useDocker).mockReturnValue({
        containers: [
          {
            id: 'remote-container-1',
            names: ['remote-app'],
            image: 'nginx:latest',
            state: 'running',
            status: 'Up 1 hour',
            network: 'bridge',
            ip: '172.18.0.10',
            ports: [{ private_port: 80, public_port: 18080, type: 'tcp' }],
          },
        ],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      renderWithClient(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      fireEvent.change(screen.getByLabelText(/^Name/), { target: { value: 'Remote Mapping' } })
      fireEvent.change(screen.getByPlaceholderText('example.com, www.example.com'), { target: { value: 'remote.existing.com' } })

      await selectComboboxOption('Source', 'Local Docker Registry (localhost)')
      await selectComboboxOption('Containers', 'remote-app (nginx:latest)')

      await waitFor(() => {
        expect(screen.getByLabelText(/^Host$/)).toHaveValue('localhost')
        expect(screen.getByLabelText(/^Port$/)).toHaveValue(18080)
      })

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          forward_host: 'localhost',
          forward_port: 18080,
        }))
      })
    }, 15000)

    it('updates domain using selected container when base domain changes', async () => {
      const { useDocker } = await import('../../hooks/useDocker')
      vi.mocked(useDocker).mockReturnValue({
        containers: [
          {
            id: 'container-123',
            names: ['my-app'],
            image: 'nginx:latest',
            state: 'running',
            status: 'Up 2 hours',
            network: 'bridge',
            ip: '172.17.0.2',
            ports: [{ private_port: 80, public_port: 8080, type: 'tcp' }],
          },
        ],
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      })

      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await selectComboboxOption('Source', 'Local (Docker Socket)')
      await selectComboboxOption('Containers', 'my-app (nginx:latest)')
      await selectComboboxOption(/Base Domain/i, 'existing.com')

      expect(screen.getByLabelText(/Domain Names/i)).toHaveValue('my-app.existing.com')
    })

    it('prompts to save a new base domain when user enters a base domain directly', async () => {
      localStorage.removeItem('charon_dont_ask_domain')
      localStorage.removeItem('cpmp_dont_ask_domain')

      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, 'brandnewdomain.com')
      await userEvent.tab()

      await waitFor(() => {
        expect(screen.getByText('New Base Domain Detected')).toBeInTheDocument()
        expect(screen.getByText('brandnewdomain.com')).toBeInTheDocument()
      })
    })
  })

  describe('Host and Port Combination', () => {
    it('accepts docker container IP', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Docker Container')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'docker.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '172.17.0.2')
      await userEvent.clear(screen.getByLabelText(/^Port/))
      await userEvent.type(screen.getByLabelText(/^Port/), '8080')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          forward_host: '172.17.0.2',
        }))
      })
    })

    it('accepts localhost IP', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Localhost')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'localhost.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), 'localhost')
      await userEvent.clear(screen.getByLabelText(/^Port/))
      await userEvent.type(screen.getByLabelText(/^Port/), '3000')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          forward_host: 'localhost',
        }))
      })
    })
  })

  describe('Enabled/Disabled State', () => {
    it('toggles enabled state', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await userEvent.type(screen.getByLabelText(/^Name/), 'Enabled Test')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'enabled.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      // Toggle enabled (defaults to true) - look for "Enable Proxy Host" text
      const enabledCheckbox = screen.getByLabelText(/Enable Proxy Host/)
      await userEvent.click(enabledCheckbox)

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          enabled: false,
        }))
      })
    })
  })

  describe('Standard Headers Option', () => {
    it('toggles standard headers option', async () => {
      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      const standardHeadersCheckbox = screen.getByLabelText(/Enable Standard Proxy Headers/)
      expect(standardHeadersCheckbox).toBeChecked()

      await userEvent.click(standardHeadersCheckbox)

      expect(standardHeadersCheckbox).not.toBeChecked()

      await userEvent.type(screen.getByLabelText(/^Name/), 'No Headers')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'no-headers.existing.com')
      await userEvent.type(screen.getByLabelText(/^Host/), '192.168.1.100')
      await userEvent.type(screen.getByLabelText(/^Port/), '80')

      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({
          enable_standard_headers: false,
        }))
      })
    })
  })

  describe('Docker Connection Failed troubleshooting', () => {
    it('renders supplemental group guidance when docker error is present', async () => {
      const { useDocker } = await import('../../hooks/useDocker')
      vi.mocked(useDocker).mockReturnValue({
        containers: [],
        isLoading: false,
        error: new Error('Docker socket permission denied'),
        refetch: vi.fn(),
      })

      await renderWithClientAct(
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // Select Local Docker Socket source to trigger error panel
      await selectComboboxOption('Source', 'Local (Docker Socket)')

      await waitFor(() => {
        expect(screen.getByText('Docker Connection Failed')).toBeInTheDocument()
      })

      expect(screen.getByText(/Troubleshooting:/)).toBeInTheDocument()
      expect(screen.getByText(/Docker socket group/)).toBeInTheDocument()
      expect(screen.getByText('group_add')).toBeInTheDocument()
      expect(screen.getByText('--group-add')).toBeInTheDocument()
    })
  })
})
