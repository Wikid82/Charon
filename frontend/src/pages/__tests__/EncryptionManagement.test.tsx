import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as encryptionApi from '../../api/encryption'
import EncryptionManagement from '../EncryptionManagement'


// Mock the API module
vi.mock('../../api/encryption')

const mockEncryptionApi = encryptionApi as {
  getEncryptionStatus: ReturnType<typeof vi.fn>
  getRotationHistory: ReturnType<typeof vi.fn>
  rotateEncryptionKey: ReturnType<typeof vi.fn>
  validateKeyConfiguration: ReturnType<typeof vi.fn>
}

describe('EncryptionManagement', () => {
  let queryClient: QueryClient

  const mockStatus = {
    current_version: 2,
    next_key_configured: true,
    legacy_key_count: 1,
    providers_on_current_version: 5,
    providers_on_older_versions: 2,
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
      expect(screen.getByText('5')).toBeInTheDocument() // providers on current version
      expect(screen.getByText('Using current key version')).toBeInTheDocument()
      expect(screen.getByText('Configured')).toBeInTheDocument() // next key status
    })
  })

  it('shows warning when providers on older versions exist', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Providers Outdated')).toBeInTheDocument()
    })
  })

  it('displays legacy key warning when legacy keys exist', async () => {
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Legacy Encryption Keys Detected')).toBeInTheDocument()
      expect(screen.getByText(/1 legacy keys are configured/)).toBeInTheDocument()
    })
  })

  it('enables rotation button when next key is configured', async () => {
    renderComponent()

    await waitFor(() => {
      const rotateButton = screen.getByText('Rotate Encryption Key')
      expect(rotateButton).toBeEnabled()
    })
  })

  it('disables rotation button when next key is not configured', async () => {
    mockEncryptionApi.getEncryptionStatus.mockResolvedValue({
      ...mockStatus,
      next_key_configured: false,
    })

    renderComponent()

    await waitFor(() => {
      const rotateButton = screen.getByText('Rotate Encryption Key')
      expect(rotateButton).toBeDisabled()
    })
  })

  it('shows confirmation dialog when rotation is triggered', async () => {
    const user = userEvent.setup()
    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    const rotateButton = screen.getByText('Rotate Encryption Key')
    await user.click(rotateButton)

    await waitFor(() => {
      expect(screen.getByText('Confirm Key Rotation')).toBeInTheDocument()
      expect(screen.getByText(/This will re-encrypt all DNS provider credentials/)).toBeInTheDocument()
    })
  })

  it('executes rotation when confirmed', async () => {
    const user = userEvent.setup()
    const mockResult = {
      total_providers: 7,
      success_count: 7,
      failure_count: 0,
      duration: '5.2s',
      new_key_version: 3,
    }

    mockEncryptionApi.rotateEncryptionKey.mockResolvedValue(mockResult)

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    // Open dialog
    const rotateButton = screen.getByText('Rotate Encryption Key')
    await user.click(rotateButton)

    // Confirm rotation
    await waitFor(() => {
      expect(screen.getByText('Start Rotation')).toBeInTheDocument()
    })

    const confirmButton = screen.getByText('Start Rotation')
    await user.click(confirmButton)

    await waitFor(() => {
      expect(mockEncryptionApi.rotateEncryptionKey).toHaveBeenCalled()
    })
  })

  it('handles rotation errors gracefully', async () => {
    const user = userEvent.setup()
    mockEncryptionApi.rotateEncryptionKey.mockRejectedValue(new Error('Rotation failed'))

    renderComponent()

    await waitFor(() => {
      expect(screen.getByText('Rotate Encryption Key')).toBeInTheDocument()
    })

    const rotateButton = screen.getByText('Rotate Encryption Key')
    await user.click(rotateButton)

    await waitFor(() => {
      expect(screen.getByText('Start Rotation')).toBeInTheDocument()
    })

    const confirmButton = screen.getByText('Start Rotation')
    await user.click(confirmButton)

    await waitFor(() => {
      expect(mockEncryptionApi.rotateEncryptionKey).toHaveBeenCalled()
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

    const validateButton = screen.getByText('Validate Configuration')
    await user.click(validateButton)

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
      expect(screen.getByText(/CHARON_ENCRYPTION_KEY_V2=/)).toBeInTheDocument()
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
