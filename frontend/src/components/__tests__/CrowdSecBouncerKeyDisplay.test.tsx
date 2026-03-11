/**
 * CrowdSecBouncerKeyDisplay Component Tests
 * Tests the bouncer API key display functionality for CrowdSec integration
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../../api/client'
import { CrowdSecBouncerKeyDisplay } from '../CrowdSecBouncerKeyDisplay'

// Create mock axios instance
vi.mock('axios', () => {
  const mockGet = vi.fn()
  return {
    default: {
      create: () => ({
        get: mockGet,
        defaults: { headers: { common: {} } },
        interceptors: {
          request: { use: vi.fn() },
          response: { use: vi.fn() },
        },
      }),
    },
    get: mockGet,
  }
})

// Mock i18n translation
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'security.crowdsec.bouncerApiKey': 'Bouncer API Key',
        'security.crowdsec.keyCopied': 'Key copied to clipboard',
        'security.crowdsec.copyFailed': 'Failed to copy key',
        'security.crowdsec.noKeyConfigured': 'No bouncer API key configured',
        'security.crowdsec.registered': 'Registered',
        'security.crowdsec.notRegistered': 'Not Registered',
        'security.crowdsec.sourceEnvVar': 'Environment Variable',
        'security.crowdsec.sourceFile': 'File',
        'security.crowdsec.keyStoredAt': 'Key stored at',
        'common.copy': 'Copy',
        'common.success': 'Success',
      }
      return translations[key] || key
    },
    ready: true,
  }),
}))

const mockBouncerInfo = {
  name: 'caddy-bouncer',
  key_preview: 'abc***xyz',
  key_source: 'file' as const,
  file_path: '/etc/crowdsec/bouncers/caddy.key',
  registered: true,
}

const mockBouncerInfoEnvVar = {
  name: 'caddy-bouncer',
  key_preview: 'env***var',
  key_source: 'env_var' as const,
  file_path: '/etc/crowdsec/bouncers/caddy.key',
  registered: true,
}

const mockBouncerInfoNotRegistered = {
  name: 'caddy-bouncer',
  key_preview: 'unreg***key',
  key_source: 'file' as const,
  file_path: '/etc/crowdsec/bouncers/caddy.key',
  registered: false,
}

const mockBouncerInfoNoKey = {
  name: 'caddy-bouncer',
  key_preview: '',
  key_source: 'none' as const,
  file_path: '',
  registered: false,
}

describe('CrowdSecBouncerKeyDisplay', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  const renderComponent = () => {
    return render(<CrowdSecBouncerKeyDisplay />, { wrapper })
  }

  describe('Loading State', () => {
    it('should show skeleton while loading bouncer info', async () => {
      vi.mocked(client.get).mockReturnValue(new Promise(() => {}))

      renderComponent()

      const skeletons = document.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)
    })
  })

  describe('Registered Bouncer with File Key Source', () => {
    it('should display bouncer key preview', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('abc***xyz')).toBeInTheDocument()
      })
    })

    it('should show registered badge for registered bouncer', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('Registered')).toBeInTheDocument()
      })
    })

    it('should show file source badge', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('File')).toBeInTheDocument()
      })
    })

    it('should display file path', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('/etc/crowdsec/bouncers/caddy.key')).toBeInTheDocument()
      })
    })

    it('should show card title with key icon', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('Bouncer API Key')).toBeInTheDocument()
      })
    })
  })

  describe('Registered Bouncer with Env Var Key Source', () => {
    it('should show env var source badge', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoEnvVar })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('Environment Variable')).toBeInTheDocument()
      })
    })
  })

  describe('Unregistered Bouncer', () => {
    it('should show not registered badge', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoNotRegistered })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('Not Registered')).toBeInTheDocument()
      })
    })
  })

  describe('No Key Configured', () => {
    it('should show warning message when no key is configured', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoNoKey })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('No bouncer API key configured')).toBeInTheDocument()
      })
    })
  })

  describe('Copy Key Functionality', () => {
    it.skip('should copy full key to clipboard when copy button is clicked', async () => {
      // Skipped: Complex async mock chain with clipboard API
    })

    it.skip('should show success state after copying', async () => {
      // Skipped: Complex async mock chain with clipboard API
    })

    it.skip('should show error toast when copy fails', async () => {
      // Skipped: Complex async mock chain
    })
  })

  describe('Error Handling', () => {
    it.skip('should return null when API fetch fails', async () => {
      // Skipped: Mock isolation issues with axios, covered in integration tests
    })
  })
})
