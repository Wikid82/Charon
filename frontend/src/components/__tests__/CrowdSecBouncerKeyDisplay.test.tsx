/**
 * CrowdSecBouncerKeyDisplay Component Tests
 * Tests the bouncer API key display functionality for CrowdSec integration
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../../api/client'
import { toast } from '../../utils/toast'
import { CrowdSecBouncerKeyDisplay } from '../CrowdSecBouncerKeyDisplay'

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

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
        'security.crowdsec.registrationUnknown': 'Status unknown',
        'security.crowdsec.sourceEnvVar': 'Environment Variable',
        'security.crowdsec.sourceFile': 'File',
        'security.crowdsec.keyStoredAt': 'Key stored at',
        'security.crowdsec.keyHeldInMemory': 'Held in memory until the key file can be written',
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

const mockBouncerInfoMemory = {
  name: 'caddy-bouncer',
  key_preview: 'mem***key',
  key_source: 'memory' as const,
  file_path: '/app/data/crowdsec/bouncer_key',
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

const mockBouncerInfoRegistrationUnknown = {
  name: 'caddy-bouncer',
  key_preview: 'unk***key',
  key_source: 'file' as const,
  file_path: '/etc/crowdsec/bouncers/caddy.key',
  registered: false,
  registration: 'unknown' as const,
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

  describe('Registered Bouncer with Memory Key Source', () => {
    it('shows the held-in-memory source line', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoMemory })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('Held in memory until the key file can be written')).toBeInTheDocument()
      })
    })

    it('does not label a memory key as coming from a file or environment variable', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoMemory })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('mem***key')).toBeInTheDocument()
      })
      expect(screen.queryByText('File')).not.toBeInTheDocument()
      expect(screen.queryByText('Environment Variable')).not.toBeInTheDocument()
    })

    it('does not show the memory line for a file key', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByText('File')).toBeInTheDocument()
      })
      expect(screen.queryByText('Held in memory until the key file can be written')).not.toBeInTheDocument()
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

  describe('Registration state (three-way)', () => {
    it('shows a neutral status when registration is unknown, not an error', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoRegistrationUnknown })

      renderComponent()

      const badge = await screen.findByText('Status unknown')
      expect(screen.queryByText('Not Registered')).not.toBeInTheDocument()
      expect(screen.queryByText('Registered')).not.toBeInTheDocument()
      expect(badge.className).not.toMatch(/bg-error/)
      expect(badge.className).toMatch(/bg-surface-muted/)
    })

    it('shows Registered when the server reports registration as registered', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: { ...mockBouncerInfo, registration: 'registered' },
      })

      renderComponent()

      const badge = await screen.findByText('Registered')
      expect(badge.className).toMatch(/bg-success/)
    })

    it('shows Not Registered when the server reports registration as not_registered', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: { ...mockBouncerInfoNotRegistered, registration: 'not_registered' },
      })

      renderComponent()

      const badge = await screen.findByText('Not Registered')
      expect(badge.className).toMatch(/bg-error/)
    })

    it('falls back to the registered flag when an older server omits registration', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfo })

      renderComponent()

      expect(await screen.findByText('Registered')).toBeInTheDocument()
      expect(screen.queryByText('Status unknown')).not.toBeInTheDocument()
    })

    it('falls back to Not Registered on an older server with registered false', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockBouncerInfoNotRegistered })

      renderComponent()

      expect(await screen.findByText('Not Registered')).toBeInTheDocument()
      expect(screen.queryByText('Status unknown')).not.toBeInTheDocument()
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
    const writeText = vi.fn<(text: string) => Promise<void>>()

    beforeEach(() => {
      writeText.mockReset()
      Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    })

    it('copies the full key and shows the copied state', async () => {
      writeText.mockResolvedValue(undefined)
      vi.mocked(client.get).mockImplementation((url: string) =>
        Promise.resolve({
          data: url.endsWith('/key') ? { key: 'full-secret-key', source: 'file' } : mockBouncerInfo,
        })
      )

      renderComponent()
      fireEvent.click(await screen.findByRole('button', { name: 'Copy' }))

      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith('full-secret-key')
      })
      expect(toast.success).toHaveBeenCalledWith('Key copied to clipboard')
      expect(await screen.findByRole('button', { name: 'Success' })).toBeDisabled()
    })

    it('shows an error toast when the key cannot be copied', async () => {
      vi.mocked(client.get).mockImplementation((url: string) =>
        url.endsWith('/key')
          ? Promise.reject(new Error('network'))
          : Promise.resolve({ data: mockBouncerInfo })
      )

      renderComponent()
      fireEvent.click(await screen.findByRole('button', { name: 'Copy' }))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith('Failed to copy key')
      })
      expect(writeText).not.toHaveBeenCalled()
    })
  })

  describe('Error Handling', () => {
    it('renders nothing when the bouncer info request fails', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('server error'))

      const { container } = renderComponent()

      await waitFor(
        () => {
          expect(container).toBeEmptyDOMElement()
        },
        { timeout: 5000 }
      )
    })
  })
})
