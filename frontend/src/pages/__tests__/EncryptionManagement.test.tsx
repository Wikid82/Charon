import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as encryptionApi from '../../api/encryption'
import { toast } from '../../utils/toast'
import EncryptionManagement from '../EncryptionManagement'

import type { RotationStatus } from '../../api/encryption'

// Mock the API module
vi.mock('../../api/encryption')
vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() },
}))

const mockEncryptionApi = encryptionApi as {
  getEncryptionStatus: ReturnType<typeof vi.fn>
  getRotationHistory: ReturnType<typeof vi.fn>
  rotateEncryptionKey: ReturnType<typeof vi.fn>
  validateKeyConfiguration: ReturnType<typeof vi.fn>
}

describe('EncryptionManagement', () => {
  let queryClient: QueryClient

  // Current key is still used by 5 DNS providers; V1 is an older key used by 2 providers and
  // nothing else (rotation needed); NEXT is configured and in use by one secret.
  const mockStatus: RotationStatus = {
    current_version: 2,
    next_key_configured: true,
    next_version: 3,
    legacy_key_count: 1,
    legacy_key_versions: [1],
    keys: [
      { slot: 'CHARON_ENCRYPTION_KEY', version: 2, key_id: '9f2c41aa', in_use: 5, removable: false, loaded: true },
      { slot: 'CHARON_ENCRYPTION_KEY_NEXT', version: 3, key_id: '1b7e0c3d', in_use: 1, removable: false, loaded: true },
      { slot: 'CHARON_ENCRYPTION_KEY_V1', version: 1, key_id: 'c0ffee12', in_use: 0, removable: true, loaded: true },
      { slot: 'CHARON_ENCRYPTION_KEY_V2', version: 0, key_id: '', in_use: 0, removable: true, loaded: false },
    ],
    stores: [
      { store: 'dns_providers', total: 7, by_version: { '1': 2, '2': 5 }, unreadable: 0 },
      { store: 'dns_provider_credentials', total: 1, by_version: { '3': 1 }, unreadable: 0 },
    ],
    unreadable_total: 0,
    rotation_needed: true,
    providers_on_current_version: 5,
    providers_on_older_versions: 2,
    providers_by_version: { '1': 2, '2': 5 },
  }

  const mockHistory = [
    {
      id: 1,
      uuid: 'test-uuid-1',
      actor: 'admin',
      action: 'encryption_key_rotated',
      event_category: 'encryption',
      details: JSON.stringify({ new_key_version: 2, duration: '5.2s' }),
      created_at: '2026-01-03T10:00:00Z',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })

    // Setup default mocks
    mockEncryptionApi.getEncryptionStatus.mockResolvedValue(mockStatus)
    mockEncryptionApi.getRotationHistory.mockResolvedValue(mockHistory)
  })

  const renderComponent = () => {
    return render(
      <BrowserRouter>
        <QueryClientProvider client={queryClient}>
          <EncryptionManagement />
        </QueryClientProvider>
      </BrowserRouter>
    )
  }

  const rotateButton = () => screen.getByTestId('rotate-key-btn')

  const keyRow = (slot: string) => screen.getByTestId(`encryption-key-row-${slot}`)

  const storeRow = (store: string) => screen.getByTestId(`encryption-store-row-${store}`)

  it('renders page title and description', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Encryption Key Management')).toBeInTheDocument()
      expect(screen.getByText('Manage encryption keys and rotate DNS provider credentials')).toBeInTheDocument()
    })
  })

  it('displays encryption status correctly', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getAllByText(/Version 2/)[0]).toBeInTheDocument()
      expect(within(screen.getByTestId('encryption-providers-updated')).getByText('5')).toBeInTheDocument()
      expect(screen.getByText('Using current key version')).toBeInTheDocument()
      expect(screen.getByText('Configured')).toBeInTheDocument() // next key status
    })
  })

  it('shows outdated provider count from the legacy DNS provider fields', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('DNS Providers Outdated')).toBeInTheDocument()
      expect(screen.getByText('DNS providers need key rotation')).toBeInTheDocument()
      expect(within(screen.getByTestId('encryption-providers-outdated')).getByText('2')).toBeInTheDocument()
    })
  })

  it('displays legacy key warning when legacy keys exist', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Legacy Encryption Keys Detected')).toBeInTheDocument()
      expect(screen.getByText(/1 legacy keys are configured/)).toBeInTheDocument()
    })
  })

  describe('Keys table', () => {
    it('lists every configured key with its version, short id and usage', async () => {
      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-keys-table')).toBeInTheDocument()
      })

      const current = within(keyRow('CHARON_ENCRYPTION_KEY'))
      expect(current.getByText('Version 2')).toBeInTheDocument()
      expect(current.getByText('Current')).toBeInTheDocument()
      expect(current.getByText('9f2c41aa')).toBeInTheDocument()
      expect(current.getByText('5')).toBeInTheDocument()

      const next = within(keyRow('CHARON_ENCRYPTION_KEY_NEXT'))
      expect(next.getByText('1b7e0c3d')).toBeInTheDocument()
      expect(next.getByText('1')).toBeInTheDocument()
    })

    it('labels keys still in use as "Still needed" and unused older keys as "No longer needed"', async () => {
      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-keys-table')).toBeInTheDocument()
      })

      expect(within(keyRow('CHARON_ENCRYPTION_KEY')).getByText('Still needed')).toBeInTheDocument()
      expect(within(keyRow('CHARON_ENCRYPTION_KEY_NEXT')).getByText('Still needed')).toBeInTheDocument()
      expect(within(keyRow('CHARON_ENCRYPTION_KEY_V1')).getByText('No longer needed')).toBeInTheDocument()
    })

    it('labels a configured key that failed to load as "Could not be loaded" with no version', async () => {
      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-keys-table')).toBeInTheDocument()
      })

      const unloaded = within(keyRow('CHARON_ENCRYPTION_KEY_V2'))
      expect(unloaded.getByText('Could not be loaded')).toBeInTheDocument()
      expect(unloaded.queryByText('Still needed')).not.toBeInTheDocument()
      expect(unloaded.queryByText('No longer needed')).not.toBeInTheDocument()
    })
  })

  describe('Stored secrets table', () => {
    it('shows one row per store with the count per key version', async () => {
      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-stores-table')).toBeInTheDocument()
      })

      const providers = within(storeRow('dns_providers'))
      expect(providers.getByText('DNS providers')).toBeInTheDocument()
      expect(providers.getByText('7')).toBeInTheDocument()
      expect(providers.getByText('Version 1: 2, Version 2: 5')).toBeInTheDocument()

      const credentials = within(storeRow('dns_provider_credentials'))
      expect(credentials.getByText('DNS provider credentials')).toBeInTheDocument()
      expect(credentials.getByText('Version 3: 1')).toBeInTheDocument()
    })

    it('shows the raw store name when the store has no label', async () => {
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
        ...mockStatus,
        stores: [{ store: 'future_store', total: 1, by_version: {}, unreadable: 0 }],
      })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-store-row-future_store')).toBeInTheDocument()
      })

      const row = within(storeRow('future_store'))
      expect(row.getByText('future_store')).toBeInTheDocument()
      expect(row.getByText('—')).toBeInTheDocument()
    })

    it('highlights unreadable counts per store', async () => {
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
        ...mockStatus,
        stores: [{ store: 'tunnels', total: 2, by_version: { '2': 1 }, unreadable: 1 }],
        unreadable_total: 1,
      })

      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-store-row-tunnels')).toBeInTheDocument()
      })

      expect(within(storeRow('tunnels')).getByText('1')).toHaveClass('text-warning')
    })
  })

  describe('unreadable secrets note', () => {
    it('shows an inline note with the unreadable count', async () => {
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({ ...mockStatus, unreadable_total: 2 })

      renderComponent()

      await waitFor(() => {
        expect(
          screen.getByText('2 stored secrets could not be opened with any configured key. Re-enter them.')
        ).toBeInTheDocument()
      })
    })

    it('hides the note when every stored secret can be opened', async () => {
      renderComponent()

      await waitFor(() => {
        expect(screen.getByTestId('encryption-stores-table')).toBeInTheDocument()
      })

      expect(screen.queryByText(/could not be opened with any configured key/)).not.toBeInTheDocument()
    })
  })

  describe('rotate button', () => {
    it('enables rotation when some stored secret is on another key', async () => {
      renderComponent()

      await waitFor(() => {
        expect(rotateButton()).toBeEnabled()
      })
    })

    it('disables rotation when nothing needs rotating, even with a next key configured', async () => {
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
        ...mockStatus,
        rotation_needed: false,
      })

      renderComponent()

      await waitFor(() => {
        expect(rotateButton()).toBeDisabled()
      })
    })

    it('disables rotation and explains the next key when it is not configured', async () => {
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
        ...mockStatus,
        next_key_configured: false,
        rotation_needed: false,
      })

      renderComponent()

      await waitFor(() => {
        expect(rotateButton()).toBeDisabled()
        expect(
          screen.getByText('To move to a new key, set CHARON_ENCRYPTION_KEY_NEXT to the new key and restart Charon.')
        ).toBeInTheDocument()
      })
    })

    it('enables rotation without a next key and confirms it re-encrypts with the current key', async () => {
      const user = userEvent.setup()
      mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
        ...mockStatus,
        next_key_configured: false,
        rotation_needed: true,
      })

      renderComponent()

      await waitFor(() => {
        expect(rotateButton()).toBeEnabled()
      })

      await user.click(rotateButton())

      const dialog = await screen.findByRole('dialog')
      expect(
        within(dialog).getByText(/This re-encrypts every stored secret with the current key. Nothing else changes./)
      ).toBeInTheDocument()
      expect(within(dialog).getByText(/Every stored secret is re-encrypted with the current key/)).toBeInTheDocument()
      expect(within(dialog).queryByText(/CHARON_ENCRYPTION_KEY_NEXT/)).not.toBeInTheDocument()
      expect(within(dialog).queryByText(/next key/)).not.toBeInTheDocument()
    })

    it('does not show the next key hint when the next key is configured', async () => {
      renderComponent()

      await waitFor(() => {
        expect(rotateButton()).toBeEnabled()
      })

      expect(screen.queryByText(/To move to a new key, set CHARON_ENCRYPTION_KEY_NEXT/)).not.toBeInTheDocument()
    })
  })

  it('shows confirmation dialog when rotation is triggered', async () => {
    const user = userEvent.setup()
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    await user.click(rotateButton())

    await waitFor(() => {
      expect(screen.getByText('Confirm Key Rotation')).toBeInTheDocument()
      expect(screen.getByText(/This re-encrypts every stored secret with the next key/)).toBeInTheDocument()
    })
  })

  it('executes rotation when confirmed and reports the stored secret counts', async () => {
    const user = userEvent.setup()
    mockEncryptionApi.rotateEncryptionKey.mockResolvedValue({
      target_version: 3,
      stores: [{ store: 'dns_providers', total: 7, rotated: 6, already_current: 0, unreadable: 1 }],
      total_records: 14,
      rotated_count: 13,
      already_current_count: 0,
      unreadable_count: 1,
      new_key_version: 3,
      duration: '41ms',
      started_at: '2026-01-03T10:00:00Z',
      completed_at: '2026-01-03T10:00:01Z',
    })

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    await user.click(rotateButton())

    await waitFor(() => {
      expect(screen.getByText('Start Rotation')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Start Rotation'))

    await waitFor(() => {
      expect(mockEncryptionApi.rotateEncryptionKey).toHaveBeenCalled()
      expect(toast.success).toHaveBeenCalledWith('Re-encrypted 13 of 14 stored secrets in 41ms.')
    })
  })

  it('reports a rotation failure as an error toast', async () => {
    const user = userEvent.setup()
    mockEncryptionApi.rotateEncryptionKey.mockRejectedValue(new Error('Rotation failed'))

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    await user.click(rotateButton())

    await waitFor(() => {
      expect(screen.getByText('Start Rotation')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Start Rotation'))

    await waitFor(() => {
      expect(mockEncryptionApi.rotateEncryptionKey).toHaveBeenCalled()
      expect(toast.error).toHaveBeenCalledWith('Key rotation failed: Rotation failed')
    })
  })

  it('validates key configuration when validate button is clicked', async () => {
    const user = userEvent.setup()
    const mockValidation = {
      valid: true,
      warnings: ['Keep old keys for 30 days'],
    }

    mockEncryptionApi.validateKeyConfiguration.mockResolvedValue(mockValidation)

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Validate Configuration')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Validate Configuration'))

    await waitFor(() => {
      expect(mockEncryptionApi.validateKeyConfiguration).toHaveBeenCalled()
    })
  })

  it('displays rotation history', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotation History')).toBeInTheDocument()
      expect(screen.getByText('admin')).toBeInTheDocument()
      expect(screen.getByText('encryption_key_rotated')).toBeInTheDocument()
    })
  })

  it('displays environment variable guide', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Environment Variable Configuration')).toBeInTheDocument()
      expect(screen.getByText(/CHARON_ENCRYPTION_KEY=/)).toBeInTheDocument()
      expect(screen.getByText(/CHARON_ENCRYPTION_KEY_NEXT=/)).toBeInTheDocument()
    })
  })

  it('shows loading state while fetching status', () => {
    mockEncryptionApi.getEncryptionStatus.mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    renderComponent()

    expect(screen.getByText('Encryption Key Management')).toBeInTheDocument()
    // Should show skeletons
    expect(document.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('shows error state when status fetch fails', async () => {
    mockEncryptionApi.getEncryptionStatus.mockRejectedValue(new Error('Failed to fetch'))

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Failed to load encryption status. Please refresh the page.')).toBeInTheDocument()
    })
  })
})
